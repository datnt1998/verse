package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

func authReq(g *Gateway, method, path, body, cookie string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	return w
}

func TestADMembersContract(t *testing.T) {
	g, _ := newAuthGateway(t)
	admin := authLogin(t, g, "admin", "admin-pass")
	created := authReq(g, "POST", "/api/users", `{"username":"linh","name":"Linh","role":"member","projects":{"p":"collab"},"password":"temporary-pass"}`, admin)
	if created.Code != 201 {
		t.Fatalf("create member: %d %s", created.Code, created.Body)
	}
	if !strings.Contains(created.Body.String(), `"mustChangePassword":true`) {
		t.Fatal("temporary password must force first-login change")
	}
	if !strings.Contains(created.Body.String(), `"password":"temporary-pass"`) {
		t.Fatal("temporary password must be returned once on creation")
	}
	rawStored, err := os.ReadFile(g.accounts.Path())
	if err != nil || strings.Contains(string(rawStored), "temporary-pass") || !strings.Contains(string(rawStored), "$argon2id$") {
		t.Fatalf("temporary password was not stored hashed: %v", err)
	}
	cookie := authLogin(t, g, "linh", "temporary-pass")
	if rec := authReq(g, "GET", "/api/library", "", cookie); rec.Code != 403 || !strings.Contains(rec.Body.String(), `"mustChangePassword":true`) {
		t.Fatalf("temporary account bypassed password change: %d %s", rec.Code, rec.Body)
	}
	changed := authReq(g, "POST", "/api/me/password", `{"current":"temporary-pass","new":"permanent-pass"}`, cookie)
	if changed.Code != 200 {
		t.Fatalf("change password: %d %s", changed.Code, changed.Body)
	}
	if rec := authReq(g, "GET", "/api/me", "", cookie); rec.Code != 401 {
		t.Fatalf("password change must revoke the current session: %d %s", rec.Code, rec.Body)
	}
	cookie = authLogin(t, g, "linh", "permanent-pass")
	list := authReq(g, "GET", "/api/users", "", admin)
	if strings.Contains(list.Body.String(), "temporary-pass") || strings.Contains(list.Body.String(), "passwordHash") || strings.Contains(list.Body.String(), `"books"`) {
		t.Fatalf("member list leaked legacy or secret data: %s", list.Body)
	}
	update := authReq(g, "POST", "/api/users/linh", `{"projects":{"p":"view"},"reason":"Đổi nhiệm vụ"}`, admin)
	if update.Code != 200 {
		t.Fatalf("update projects: %d %s", update.Code, update.Body)
	}
	me := authReq(g, "GET", "/api/me", "", cookie)
	if me.Code != 200 || !strings.Contains(me.Body.String(), `"p":"view"`) {
		t.Fatalf("existing session must see current permissions: %d %s", me.Code, me.Body)
	}
	audit := authReq(g, "GET", "/api/admin/audit?type=perm", "", admin)
	if audit.Code != 200 || !strings.Contains(audit.Body.String(), "Đổi nhiệm vụ") {
		t.Fatalf("permission audit: %d %s", audit.Code, audit.Body)
	}
	revoke := authReq(g, "POST", "/api/users/linh/sessions/revoke-all", `{}`, admin)
	if revoke.Code != 200 {
		t.Fatalf("revoke all: %d %s", revoke.Code, revoke.Body)
	}
	if me = authReq(g, "GET", "/api/me", "", cookie); me.Code != 401 {
		t.Fatalf("revoked device: %d %s", me.Code, me.Body)
	}
	for _, tc := range []struct{ path, body string }{{"/api/users/admin", `{"disabled":true}`}, {"/api/users/admin/delete", `{}`}, {"/api/users/admin", `{"role":"member"}`}} {
		rec := authReq(g, "POST", tc.path, tc.body, admin)
		if rec.Code != 409 {
			t.Errorf("last-admin guard %s %s: %d %s", tc.path, tc.body, rec.Code, rec.Body)
		}
	}
}

// Anonymized shape of the live version-2 file: recovery + onboarding + sessions.
func TestADLegacyLiveShapeMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web-accounts.json")
	fixture := `{"version":2,"users":[{"username":"quantri","name":"Người quản trị","role":"admin","books":[],"disabled":false,"passwordHash":"anonymized-hash","onboardedAt":"2026-10-26T14:32:00+07:00","mustChangePassword":false,"recovery":{"codeHash":"anonymized-recovery","mask":"AAAA-••••-••••-BBBB","createdAt":"2026-10-26T14:31:40+07:00"},"createdAt":"2026-10-26T14:26:00+07:00","updatedAt":"2026-10-26T21:45:38+07:00"},{"username":"congtac","name":"Thành viên","role":"writer","books":["p/story"],"passwordHash":"anonymous","createdAt":"2026-10-26T14:26:00+07:00"}],"sessions":[{"id":"anonymous","secretHash":"anonymous","username":"quantri","device":"Mac · Chrome","ip":"127.0.0.1","createdAt":"2026-10-26T14:29:35+07:00","lastSeen":"2026-10-27T08:20:39+07:00","expiresAt":"2099-11-25T14:29:35+07:00"}],"pairings":[]}`
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	store := identity.NewStore(path, library.IdentityOptions())
	users, err := store.Users()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(users)
	if strings.Contains(string(raw), `"books"`) || !strings.Contains(string(raw), `"projects":{"p":"collab"}`) {
		t.Fatalf("legacy project migration: %s", raw)
	}
	backup, err := os.ReadFile(path + ".v2.bak")
	if err != nil || string(backup) != fixture {
		t.Fatalf("original backup must be byte-identical: %v", err)
	}
	before, _ := os.ReadFile(path)
	if _, err := identity.NewStore(path, library.IdentityOptions()).Users(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("migration must be idempotent")
	}
}

func TestADRouteAuthorizationMatrix(t *testing.T) {
	const p = "/api/projects/p"
	const s = p + "/stories/story"
	// Each routed family goes through the same boundary before a handler or
	// worker is reached. Include write-via-POST and delete-via-POST endpoints.
	cases := []struct {
		method, path string
		level        identity.ProjectRole
		admin        bool
	}{}
	for _, path := range []string{p, p + "/stories", p + "/ideas", p + "/assets", p + "/voices", p + "/pronunciations", p + "/ledger", p + "/universe", p + "/search", p + "/validate", p + "/cocreate",
		s + "/meta", s + "/snapshot", s + "/events/live", s + "/chapters", s + "/chapters/1", s + "/chapters/1/plan", s + "/chapters/1/facts", s + "/chapters/1/revisions", s + "/outline", s + "/characters", s + "/foreshadow", s + "/quality", s + "/reviews/latest", s + "/exports", s + "/exports/file", s + "/exports/download", s + "/export", s + "/rules", s + "/style-samples", s + "/frame", s + "/retention", s + "/metrics", s + "/brain", s + "/address-terms", s + "/comments", s + "/audio/export-prefs", s + "/audio/events", s + "/audio/volumes", s + "/audio/speakers"} {
		cases = append(cases, struct {
			method, path string
			level        identity.ProjectRole
			admin        bool
		}{"GET", path, identity.ProjectView, false})
	}
	for _, path := range []string{"/api/jobs/p/story/audio/1/cancel", p + "/stories", p + "/ideas", p + "/assets", p + "/voices", p + "/cocreate/message", p + "/cocreate/commit", p + "/cocreate/cancel", s + "/input", s + "/next", s + "/steer", s + "/pause", s + "/resume", s + "/stop", s + "/close", s + "/rename", s + "/exports", s + "/sync", s + "/import", s + "/import/preview", s + "/import/commit", s + "/rewrite", s + "/simulate", s + "/style-samples", s + "/comments", s + "/relations", s + "/audio/volumes/1/render", s + "/audio/volumes/1/exports", s + "/audio/volumes/1/cancel"} {
		cases = append(cases, struct {
			method, path string
			level        identity.ProjectRole
			admin        bool
		}{"POST", path, identity.ProjectCollab, false})
	}
	for _, path := range []string{p + "/pronunciations", s + "/meta", s + "/characters", s + "/address-terms", s + "/frame", s + "/rules", s + "/audio/export-prefs", s + "/audio/speakers"} {
		cases = append(cases, struct {
			method, path string
			level        identity.ProjectRole
			admin        bool
		}{"PUT", path, identity.ProjectCollab, false})
	}
	for _, tc := range []struct{ method, path string }{{"DELETE", s}, {"DELETE", s + "/style-samples/sample"}, {"POST", s + "/share"}, {"POST", s + "/delete"}, {"PUT", p}, {"PUT", "/api/config?scope=project&project=p"}, {"PUT", "/api/rules/file?scope=project&project=p"}, {"POST", "/api/rules/file/delete?scope=story&project=p&story=story"}, {"PUT", p + "/ledger"}} {
		cases = append(cases, struct {
			method, path string
			level        identity.ProjectRole
			admin        bool
		}{tc.method, tc.path, identity.ProjectOwner, false})
	}
	for _, tc := range []struct{ method, path string }{{"POST", "/api/jobs/" + voiceInstallJobID + "/retry"}, {"GET", "/api/users"}, {"POST", "/api/users"}, {"POST", "/api/users/u"}, {"POST", "/api/users/u/delete"}, {"GET", "/api/users/u/sessions"}, {"POST", "/api/users/u/sessions/revoke-all"}, {"POST", "/api/users/u/pairings"}, {"GET", "/api/admin/audit"}, {"PUT", "/api/config"}, {"PUT", "/api/library"}, {"POST", "/api/projects"}, {"POST", "/api/projects/adopt"}, {"GET", "/api/fs/places"}, {"PUT", "/api/system/network"}, {"POST", "/api/tts/install"}} {
		cases = append(cases, struct {
			method, path string
			level        identity.ProjectRole
			admin        bool
		}{tc.method, tc.path, "", true})
	}
	rank := map[identity.ProjectRole]int{"": 0, identity.ProjectView: 1, identity.ProjectCollab: 2, identity.ProjectOwner: 3}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			for _, level := range []identity.ProjectRole{"", identity.ProjectView, identity.ProjectCollab, identity.ProjectOwner} {
				user := identity.User{Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"p": level}}
				want := !tc.admin && rank[level] >= rank[tc.level]
				if got := requestAllowed(user, r); got != want {
					t.Errorf("%s: allowed=%v want %v", level, got, want)
				}
			}
			if !requestAllowed(identity.User{Role: identity.RoleAdmin}, r) {
				t.Error("admin denied")
			}
			if requestAllowed(identity.User{Role: identity.RoleAdmin, Disabled: true}, r) {
				t.Error("disabled admin allowed")
			}
		})
	}
	for _, path := range []string{"/api/projects/share/stories/delete/input", "/api/projects/delete/stories/share/input"} {
		user := identity.User{Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"share": identity.ProjectCollab, "delete": identity.ProjectCollab}}
		if !requestAllowed(user, httptest.NewRequest("POST", path, nil)) {
			t.Errorf("resource IDs must not become operation names: %s", path)
		}
	}
}

func TestADImmediateStoredAuthorization(t *testing.T) {
	tg := newConfigTestGateway(t)
	if _, err := tg.g.accounts.CreateUser(identity.NewUser{Username: "member", Role: identity.RoleAdmin, Password: "member-pass"}); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("member", "member-pass")
	admin := tg.login("admin", "admin-pass")
	if rec := authReq(tg.g, "GET", "/api/users", "", cookie); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	for _, tc := range []struct {
		body, method, path string
		status             int
	}{
		{`{"role":"member","projects":{"proj":"collab"}}`, "GET", "/api/users", 403},
		{`{"projects":{"proj":"view"}}`, "POST", "/api/projects/proj/stories/01-story/input", 403},
		{`{"projects":{}}`, "GET", "/api/projects/proj/stories/01-story/meta", 403},
		{`{"disabled":true,"reason":"Tạm dừng"}`, "GET", "/api/me", 401},
	} {
		rec := authReq(tg.g, "POST", "/api/users/member", tc.body, admin)
		if rec.Code != 200 {
			t.Fatalf("update: %d %s", rec.Code, rec.Body)
		}
		rec = authReq(tg.g, tc.method, tc.path, `{}`, cookie)
		if rec.Code != tc.status {
			t.Fatalf("immediate %s: %d %s", tc.body, rec.Code, rec.Body)
		}
	}
	rec := authReq(tg.g, "POST", "/api/users/member", `{"disabled":false,"projects":{"proj":"view"}}`, admin)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	cookie = tg.login("member", "member-pass")
	if rec = authReq(tg.g, "GET", "/api/projects/proj/stories/01-story/meta", "", cookie); rec.Code != 200 {
		t.Fatalf("view before delete: %d %s", rec.Code, rec.Body)
	}
	if rec = authReq(tg.g, "POST", "/api/users/member/delete", `{"reason":"Đã kết thúc"}`, admin); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec = authReq(tg.g, "GET", "/api/me", "", cookie); rec.Code != 401 {
		t.Fatalf("deleted user session survived: %d", rec.Code)
	}
}

func TestADDeviceSignOutAndOptionalReasons(t *testing.T) {
	g, _ := newAuthGateway(t)
	admin := authLogin(t, g, "admin", "admin-pass")
	one := authLogin(t, g, "writer", "writer-pass")
	two := authLogin(t, g, "writer", "writer-pass")
	list := authReq(g, "GET", "/api/users/writer/sessions", "", admin)
	var sessions struct {
		Sessions []identity.Session `json:"sessions"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions.Sessions) != 2 {
		t.Fatalf("devices=%s", list.Body)
	}
	id := strings.Split(one, ".")[0]
	if rec := authReq(g, "POST", "/api/sessions/"+id+"/revoke", `{}`, admin); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := authReq(g, "GET", "/api/me", "", one); rec.Code != 401 {
		t.Fatal("one device still signed in")
	}
	if rec := authReq(g, "GET", "/api/me", "", two); rec.Code != 200 {
		t.Fatal("unrelated device revoked")
	}
	if rec := authReq(g, "POST", "/api/me/sessions/revoke-all", `{}`, two); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := authReq(g, "GET", "/api/me", "", two); rec.Code != 401 {
		t.Fatal("self revoke-all must revoke the current device too")
	}
	for _, body := range []string{`{"disabled":true}`, `{"disabled":false}`} {
		if rec := authReq(g, "POST", "/api/users/writer", body, admin); rec.Code != 200 {
			t.Fatalf("optional lock reason: %d %s", rec.Code, rec.Body)
		}
	}
	if rec := authReq(g, "POST", "/api/users/writer/delete", `{}`, admin); rec.Code != 200 {
		t.Fatalf("optional delete reason: %d %s", rec.Code, rec.Body)
	}
	for _, path := range []string{"/api/users/admin/delete", "/api/users/admin"} {
		body := `{}`
		if path == "/api/users/admin" {
			body = `{"disabled":true}`
		}
		if rec := authReq(g, "POST", path, body, admin); rec.Code != 409 {
			t.Fatalf("self/last-admin guard: %d", rec.Code)
		}
	}
}

func moveStoryToUnassignedProject(t *testing.T, root, source string) {
	t.Helper()
	lib, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := lib.CreateProject("hidden", library.ProjectFile{Name: "Dự án riêng"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, filepath.Join(dir, "kich_ban", filepath.Base(source))); err != nil {
		t.Fatal(err)
	}
}

// ---- Phase 04 · owner-scoped member API (quyết định 2026-10-28) ----
// Chủ dự án quản lý thành viên của dự án mình, không đụng /api/users (admin).

// memberFixture dựng gateway với project "proj" và các tài khoản theo vai:
// writer = owner proj (fixture sẵn), collab, viewer và outsider.
// Trả về cookie của từng vai.
func memberFixture(t *testing.T) (g *Gateway, owner, collab, viewer, outsider, admin string) {
	t.Helper()
	tg := newTestGateway(t)
	t.Cleanup(tg.g.Close)
	g = tg.g
	for _, u := range []identity.NewUser{
		{Username: "collab", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectCollab}, Password: "collab-pass"},
		{Username: "viewer", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"proj": identity.ProjectView}, Password: "viewer-pass"},
		{Username: "outsider", Role: identity.RoleMember, Password: "outsider-pass"},
	} {
		if _, err := g.accounts.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	return g, tg.login("writer", "writer-pass"), tg.login("collab", "collab-pass"),
		tg.login("viewer", "viewer-pass"), tg.login("outsider", "outsider-pass"), tg.login("admin", "admin-pass")
}

// TestADMemberListByRoles pin GET …/members: mọi vai có quyền đọc xem danh sách;
// kẻ ngoài dự án và chưa đăng nhập bị từ chối.
func TestADMemberListByRoles(t *testing.T) {
	g, owner, collab, viewer, outsider, admin := memberFixture(t)
	for _, cookie := range []string{owner, collab, viewer, admin} {
		rec := authReq(g, "GET", "/api/projects/proj/members", "", cookie)
		if rec.Code != 200 {
			t.Fatalf("member list = %d: %s", rec.Code, rec.Body)
		}
		var body struct {
			Members []identity.ProjectMember `json:"members"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		roles := map[string]identity.ProjectRole{}
		for _, m := range body.Members {
			roles[m.Username] = m.Role
		}
		if roles["writer"] != identity.ProjectOwner || roles["collab"] != identity.ProjectCollab || roles["viewer"] != identity.ProjectView {
			t.Fatalf("members = %+v", body.Members)
		}
	}
	if rec := authReq(g, "GET", "/api/projects/proj/members", "", outsider); rec.Code != 403 {
		t.Fatalf("outsider = %d: %s", rec.Code, rec.Body)
	}
	if rec := authReq(g, "GET", "/api/projects/proj/members", "", ""); rec.Code != 401 {
		t.Fatalf("anonymous = %d", rec.Code)
	}
}

// TestADOwnerInvitePatchRemove pin vòng đời membership do owner thực hiện:
// mời collab/view, đổi vai, gỡ member — kèm audit và invariant owner cuối.
func TestADOwnerInvitePatchRemove(t *testing.T) {
	g, owner, _, _, _, _ := memberFixture(t)
	memberByUsername := func(cookie, username string) (identity.ProjectRole, int) {
		rec := authReq(g, "GET", "/api/projects/proj/members", "", cookie)
		if rec.Code != 200 {
			return "", rec.Code
		}
		var body struct {
			Members []identity.ProjectMember `json:"members"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, m := range body.Members {
			if m.Username == username {
				return m.Role, 200
			}
		}
		return "", 200
	}

	// Mời outsider làm collab → 201 kèm member.
	rec := authReq(g, "POST", "/api/projects/proj/members", `{"username":"outsider","role":"collab"}`, owner)
	if rec.Code != 201 {
		t.Fatalf("invite = %d: %s", rec.Code, rec.Body)
	}
	if role, _ := memberByUsername(owner, "outsider"); role != identity.ProjectCollab {
		t.Fatalf("invited role = %q", role)
	}

	// Đổi vai collab → view qua PATCH.
	rec = authReq(g, "PATCH", "/api/projects/proj/members/outsider", `{"role":"view"}`, owner)
	if rec.Code != 200 {
		t.Fatalf("patch = %d: %s", rec.Code, rec.Body)
	}
	if role, _ := memberByUsername(owner, "outsider"); role != identity.ProjectView {
		t.Fatalf("patched role = %q", role)
	}

	// Gỡ member → 204; viewer role của outsider biến mất.
	rec = authReq(g, "DELETE", "/api/projects/proj/members/outsider", "", owner)
	if rec.Code != 204 {
		t.Fatalf("remove = %d: %s", rec.Code, rec.Body)
	}
	if role, _ := memberByUsername(owner, "outsider"); role != "" {
		t.Fatalf("removed member still listed: %q", role)
	}

	// Gỡ owner cuối cùng (writer) → 409 ErrLastOwner.
	rec = authReq(g, "DELETE", "/api/projects/proj/members/writer", "", owner)
	if rec.Code != 409 {
		t.Fatalf("remove last owner = %d: %s", rec.Code, rec.Body)
	}
	// Hạ owner cuối xuống collab → 409.
	rec = authReq(g, "PATCH", "/api/projects/proj/members/writer", `{"role":"collab"}`, owner)
	if rec.Code != 409 {
		t.Fatalf("demote last owner = %d: %s", rec.Code, rec.Body)
	}
	// PATCH role lạ / owner qua PATCH → 400.
	for _, body := range []string{`{"role":"owner"}`, `{"role":"editor"}`} {
		if rec := authReq(g, "PATCH", "/api/projects/proj/members/viewer", body, owner); rec.Code != 400 {
			t.Fatalf("patch %s = %d: %s", body, rec.Code, rec.Body)
		}
	}
	// Mời user không tồn tại → 404.
	if rec := authReq(g, "POST", "/api/projects/proj/members", `{"username":"khong-co","role":"view"}`, owner); rec.Code != 404 {
		t.Fatalf("invite missing user = %d: %s", rec.Code, rec.Body)
	}
}

// TestADTransferOwnership pin POST …/owner: chuyển quyền nguyên tử — owner cũ
// về collab, owner mới đúng một người; admin cũng chuyển được.
func TestADTransferOwnership(t *testing.T) {
	g, owner, collab, _, _, admin := memberFixture(t)

	rec := authReq(g, "POST", "/api/projects/proj/owner", `{"username":"collab"}`, owner)
	if rec.Code != 200 {
		t.Fatalf("transfer = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Members []identity.ProjectMember `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	owners, roles := 0, map[string]identity.ProjectRole{}
	for _, m := range body.Members {
		roles[m.Username] = m.Role
		if m.Role == identity.ProjectOwner {
			owners++
		}
	}
	if owners != 1 || roles["collab"] != identity.ProjectOwner || roles["writer"] != identity.ProjectCollab {
		t.Fatalf("after transfer = %+v", body.Members)
	}

	// Owner mới chuyển lại cho writer → đúng một owner.
	rec = authReq(g, "POST", "/api/projects/proj/owner", `{"username":"writer"}`, collab)
	if rec.Code != 200 {
		t.Fatalf("transfer back = %d: %s", rec.Code, rec.Body)
	}
	// Admin cũng chuyển được (admin = owner mọi project).
	rec = authReq(g, "POST", "/api/projects/proj/owner", `{"username":"collab"}`, admin)
	if rec.Code != 200 {
		t.Fatalf("admin transfer = %d: %s", rec.Code, rec.Body)
	}
	// Chuyển cho user không tồn tại → 404.
	if rec := authReq(g, "POST", "/api/projects/proj/owner", `{"username":"khong-co"}`, collab); rec.Code != 404 {
		t.Fatalf("transfer missing user = %d: %s", rec.Code, rec.Body)
	}
	// Chuyển cho tài khoản bị khoá → 400.
	if _, err := g.accounts.UpdateUser("viewer", identity.UserPatch{Disabled: &[]bool{true}[0]}); err != nil {
		t.Fatal(err)
	}
	if rec := authReq(g, "POST", "/api/projects/proj/owner", `{"username":"viewer"}`, collab); rec.Code != 400 {
		t.Fatalf("transfer to disabled = %d: %s", rec.Code, rec.Body)
	}
}

// TestADMemberAPIIDORAndEscalation pin: collab/view/outsider không đụng được
// members/owner; owner không vào được /api/users; IDOR cross-project 403.
func TestADMemberAPIIDORAndEscalation(t *testing.T) {
	g, owner, collab, viewer, outsider, admin := memberFixture(t)

	for name, cookie := range map[string]string{"collab": collab, "viewer": viewer, "outsider": outsider} {
		for _, tc := range []struct{ method, path, body string }{
			{"POST", "/api/projects/proj/members", `{"username":"outsider","role":"collab"}`},
			{"PATCH", "/api/projects/proj/members/viewer", `{"role":"collab"}`},
			{"DELETE", "/api/projects/proj/members/viewer", ""},
			{"POST", "/api/projects/proj/owner", `{"username":"collab"}`},
		} {
			if rec := authReq(g, tc.method, tc.path, tc.body, cookie); rec.Code != 403 {
				t.Fatalf("%s %s %s = %d: %s", name, tc.method, tc.path, rec.Code, rec.Body)
			}
		}
	}

	// Owner của proj vẫn là member toàn cục: /api/users 403 với mọi method.
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/users", ""},
		{"POST", "/api/users", `{"username":"x-user","role":"member","password":"some-pass-123"}`},
		{"POST", "/api/users/viewer", `{"role":"admin"}`},
		{"POST", "/api/users/viewer/delete", `{}`},
	} {
		if rec := authReq(g, tc.method, tc.path, tc.body, owner); rec.Code != 403 {
			t.Fatalf("owner hit %s %s = %d: %s", tc.method, tc.path, rec.Code, rec.Body)
		}
	}

	// IDOR cross-project: owner proj không đụng members của project khác.
	if rec := authReq(g, "GET", "/api/projects/hidden/members", "", owner); rec.Code != 403 && rec.Code != 404 {
		t.Fatalf("cross-project list = %d", rec.Code)
	}
	if rec := authReq(g, "POST", "/api/projects/hidden/members", `{"username":"viewer","role":"view"}`, owner); rec.Code != 403 {
		t.Fatalf("cross-project invite = %d: %s", rec.Code, rec.Body)
	}
	if rec := authReq(g, "POST", "/api/projects/hidden/owner", `{"username":"viewer"}`, owner); rec.Code != 403 {
		t.Fatalf("cross-project transfer = %d: %s", rec.Code, rec.Body)
	}

	// Admin gỡ được member của proj (admin = owner qua ProjectRole()).
	if rec := authReq(g, "DELETE", "/api/projects/proj/members/viewer", "", admin); rec.Code != 204 {
		t.Fatalf("admin remove = %d: %s", rec.Code, rec.Body)
	}
}
