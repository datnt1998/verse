package web

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// hostedFixture is a fully hosted gateway: PG accounts (so User.Projects comes
// from project_members), the PG library repository and the PG fact store —
// exactly the composition root serve.go builds for STORE_DRIVER=postgres.
type hostedFixture struct {
	g        *Gateway
	pool     *postgres.Pool
	repo     *postgres.LibraryRepository
	accounts *webauth.Store
	project  string
	story    string

	owner    webauth.User
	viewer   webauth.User
	outsider webauth.User
	admin    webauth.User
}

func provisionHostedGateway(t *testing.T, projectSlug, storySlug string) *hostedFixture {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	repo := postgres.NewLibraryRepository(pool)
	accounts := webauth.NewFromRepository(postgres.NewAccounts(pool))
	for _, u := range []struct {
		name string
		role webauth.Role
	}{
		{"hosted-admin", webauth.RoleAdmin},
		{"hosted-owner", webauth.RoleMember},
		{"hosted-viewer", webauth.RoleMember},
		{"hosted-outsider", webauth.RoleMember},
	} {
		if _, err := accounts.CreateUser(webauth.NewUser{
			Username: u.name, Name: u.name, Role: u.role, Password: "mat-khau-12345",
		}); err != nil {
			t.Fatalf("create %s: %v", u.name, err)
		}
	}

	// Anchor the project row without an owner row of its own, then grant the
	// exact membership matrix the handlers see.
	q := gen.New(pool.Pool())
	project, err := q.InsertProject(ctx, gen.InsertProjectParams{
		ID: deterministicLibraryUUID(projectSlug), Slug: projectSlug, Name: "Dự án HTTP",
		Description: "mô tả", Notes: "ghi chú", UniverseName: "Vũ trụ",
	})
	if err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	for _, m := range []struct{ name, role string }{
		{"hosted-owner", "owner"}, {"hosted-viewer", "view"},
	} {
		usr, err := q.GetUserByUsername(ctx, m.name)
		if err != nil {
			t.Fatalf("read %s: %v", m.name, err)
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: project.ID, UserID: usr.ID, Role: m.role}); err != nil {
			t.Fatalf("member %s: %v", m.name, err)
		}
	}
	owner := &library.Actor{Username: "hosted-owner"}
	st, _, err := repo.CreateStory(ctx, owner, projectSlug, "Truyện HTTP", "")
	if err != nil {
		t.Fatalf("create story: %v", err)
	}
	if st.Story == "" || !strings.HasSuffix(st.Story, "truyen-http") {
		t.Fatalf("story slug: %+v", st)
	}

	g := NewGateway("", accounts, version.Info{})
	t.Cleanup(g.Close)
	g.SetLibraryRepository(repo)
	g.SetFactStore(func(st library.Story, user *webauth.User) (*store.Store, error) {
		if user == nil {
			return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
		}
		actor := &postgres.Actor{Username: user.Username, GlobalRole: string(user.Role)}
		return bootstrap.OpenStoryStore(ctx, pool, st.Project, st.Story, actor)
	})

	return &hostedFixture{
		g: g, pool: pool, repo: repo, accounts: accounts,
		project: projectSlug, story: st.Story,
		owner:    loginUser(t, accounts, "hosted-owner"),
		viewer:   loginUser(t, accounts, "hosted-viewer"),
		outsider: loginUser(t, accounts, "hosted-outsider"),
		admin:    loginUser(t, accounts, "hosted-admin"),
	}
}

// deterministicLibraryUUID derives stable UUIDs for test anchors.
func deterministicLibraryUUID(seed string) pgtype.UUID {
	sum := sha256.Sum256([]byte("ainovel|test|" + seed))
	var id pgtype.UUID
	copy(id.Bytes[:], sum[:16])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x40
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}

// loginUser returns the authenticated User for a PG-backed account: Projects
// is populated from project_members, i.e. the same authorization data the
// request pipeline carries.
func loginUser(t *testing.T, accounts *webauth.Store, username string) webauth.User {
	t.Helper()
	sess, err := accounts.Login(username, "mat-khau-12345", "test", "127.0.0.1")
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return webauth.User{
		Username: sess.User.Username,
		Name:     sess.User.Name,
		Role:     sess.User.Role,
		Projects: sess.User.Projects,
	}
}

// callLibrary invokes the library list hook (GET /api/library is the project
// list on both backends; GET /api/projects only accepts POST).
func callLibrary(g *Gateway, user webauth.User) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/library", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	w := httptest.NewRecorder()
	g.serveLibrary(w, r, user)
	return w
}

// callProjects invokes the project route hook with an explicit user — the
// session layer is bypassed on purpose so a stale/forged Projects map can be
// tested against the repository's own membership check.
func callProjects(g *Gateway, user webauth.User, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8686"
	if method == http.MethodPost || method == http.MethodPut {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	g.serveProjects(w, r, user)
	return w
}

// TestLibraryBackendIDOROverHTTP is the phase-05 gate on the hosted library
// surface: member vs outsider on real request paths, plus a stale grant whose
// only defense is the repository's in-transaction membership check.
func TestLibraryBackendIDOROverHTTP(t *testing.T) {
	f := provisionHostedGateway(t, "du-an-a", "truyen-http")

	t.Run("member_reads", func(t *testing.T) {
		w := callLibrary(f.g, f.owner)
		if w.Code != http.StatusOK {
			t.Fatalf("owner list: %d %s", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		projects, _ := body["projects"].([]any)
		if len(projects) != 1 {
			t.Fatalf("owner must see the project: %s", w.Body.String())
		}
		if projects[0].(map[string]any)["id"] != f.project {
			t.Fatalf("project id: %v", projects[0])
		}

		w = callProjects(f.g, f.owner, http.MethodGet, "/api/projects/"+f.project, "")
		if w.Code != http.StatusOK {
			t.Fatalf("owner detail: %d %s", w.Code, w.Body.String())
		}
		body = decodeBody(t, w)
		stories, _ := body["stories"].([]any)
		if len(stories) != 1 || stories[0].(map[string]any)["story"] != f.story {
			t.Fatalf("owner stories: %s", w.Body.String())
		}
		if ideas, _ := body["ideas"].([]any); len(ideas) != 0 {
			t.Fatalf("ideas must be an empty array: %s", w.Body.String())
		}

		// Viewer reads the same project through the same route.
		if w := callProjects(f.g, f.viewer, http.MethodGet, "/api/projects/"+f.project, ""); w.Code != http.StatusOK {
			t.Fatalf("viewer detail: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("outsider_rejected", func(t *testing.T) {
		w := callLibrary(f.g, f.outsider)
		if w.Code != http.StatusOK {
			t.Fatalf("outsider list must not error: %d %s", w.Code, w.Body.String())
		}
		if projects, _ := decodeBody(t, w)["projects"].([]any); len(projects) != 0 {
			t.Fatalf("outsider must not list another tenant's project: %s", w.Body.String())
		}
		for _, path := range []string{
			"/api/projects/" + f.project,
			"/api/projects/" + f.project + "/stories",
			"/api/projects/" + f.project + "/ideas",
			"/api/projects/" + f.project + "/stories/" + f.story + "/meta",
		} {
			if w := callProjects(f.g, f.outsider, http.MethodGet, path, ""); w.Code != http.StatusNotFound {
				t.Fatalf("outsider %s must be 404: %d %s", path, w.Code, w.Body.String())
			}
		}
	})

	t.Run("stale_grant_rejected_by_repository", func(t *testing.T) {
		// The user object claims view/owner (a stale file-mode grant or a
		// handler-level shortcut) while project_members has no row. Only the
		// repository's in-transaction check stands between this actor and the
		// rows, so it must answer 404 — and never leak data.
		stale := f.outsider
		stale.Projects = map[string]webauth.ProjectRole{f.project: webauth.ProjectView}
		w := callProjects(f.g, stale, http.MethodGet, "/api/projects/"+f.project, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("stale read grant must be 404: %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), `"stories"`) {
			t.Fatalf("404 must not carry story data: %s", w.Body.String())
		}

		// Same for a write: viewer-with-owner-grant passes the handler's
		// CanManageProject and must still be refused by the repository.
		overGranted := f.viewer
		overGranted.Projects = map[string]webauth.ProjectRole{f.project: webauth.ProjectOwner}
		w = callProjects(f.g, overGranted, http.MethodPut, "/api/projects/"+f.project, `{"name":"Đổi trộm"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("stale write grant must be 403: %d %s", w.Code, w.Body.String())
		}
		pf, err := f.repo.ProjectFile(context.Background(), &library.Actor{Username: "hosted-admin", GlobalRole: "admin"}, f.project)
		if err != nil {
			t.Fatalf("read project back: %v", err)
		}
		if pf.Name == "Đổi trộm" {
			t.Fatal("repository must not apply an unauthorized rename")
		}
	})

	t.Run("viewer_write_forbidden", func(t *testing.T) {
		w := callProjects(f.g, f.viewer, http.MethodPut, "/api/projects/"+f.project, `{"name":"Đổi"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("viewer update: %d %s", w.Code, w.Body.String())
		}
		w = callProjects(f.g, f.viewer, http.MethodPost, "/api/projects/"+f.project+"/stories", `{"name":"Truyện lậu"}`)
		if w.Code != http.StatusForbidden {
			t.Fatalf("viewer create story: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("owner_creates_story_numbered", func(t *testing.T) {
		w := callProjects(f.g, f.owner, http.MethodPost, "/api/projects/"+f.project+"/stories", `{"name":"Truyện hai"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("owner create story: %d %s", w.Code, w.Body.String())
		}
		if got, _ := decodeBody(t, w)["story"].(string); !strings.HasPrefix(got, "02-") {
			t.Fatalf("second story must be numbered 02: %v", got)
		}
	})

	t.Run("file_only_capability_is_501", func(t *testing.T) {
		w := callProjects(f.g, f.owner, http.MethodGet, "/api/projects/"+f.project+"/assets", "")
		if w.Code != http.StatusNotImplemented {
			t.Fatalf("assets on hosted backend must be 501: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("story_meta_reads_rows", func(t *testing.T) {
		w := callProjects(f.g, f.owner, http.MethodGet, "/api/projects/"+f.project+"/stories/"+f.story+"/meta", "")
		if w.Code != http.StatusOK {
			t.Fatalf("owner story meta: %d %s", w.Code, w.Body.String())
		}
		if title, _ := decodeBody(t, w)["title"].(string); title != "Truyện HTTP" {
			t.Fatalf("story title from rows: %s", w.Body.String())
		}
	})

	t.Run("nil_actor_rejected_by_store_factory", func(t *testing.T) {
		ctx := context.Background()
		if _, err := bootstrap.OpenStoryStore(ctx, f.pool, f.project, f.story, nil); !errors.Is(err, errs.ErrStorageForbidden) {
			t.Fatalf("nil actor must be rejected: %v", err)
		}
		scoped, err := bootstrap.OpenStoryStore(ctx, f.pool, f.project, f.story,
			&postgres.Actor{Username: "hosted-owner"})
		if err != nil {
			t.Fatalf("member must open: %v", err)
		}
		if err := scoped.Book.Save(domain.BookMetadata{Title: "Sách PG", Synopsis: "mở"}); err != nil {
			t.Fatalf("scoped write: %v", err)
		}
		if _, err := bootstrap.OpenStoryStore(ctx, f.pool, f.project, f.story,
			&postgres.Actor{Username: "hosted-outsider"}); !errors.Is(err, errs.ErrStorageNotFound) {
			t.Fatalf("outsider must be not-found: %v", err)
		}
	})
}
