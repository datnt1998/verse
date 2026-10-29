package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

func ledgerValues(prefix string) ledger.Values {
	var v ledger.Values
	for i := range v {
		v[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return v
}
func ledgerStub(t *testing.T, tg *configTestGateway, v *ledger.Values) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := json.Marshal(map[string]any{"values": v, "brief": "Yêu cầu đã đổi", "changed": []int{4, 6}})
		writeJSON(w, 200, map[string]any{"id": "ledger-stub", "object": "chat.completion", "model": "stub", "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}}, "usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1}})
	}))
	t.Cleanup(server.Close)
	tg.globalConfig(t, fmt.Sprintf(`{"provider":"openai","model":"stub","providers":{"openai":{"type":"openai","api_key":"sk-stub-1234567890","base_url":%q,"models":[{"name":"stub"}]}}}`, server.URL+"/v1"))
}
func saveLedgerTestStory(t *testing.T, tg *configTestGateway, name string, v ledger.Values, outlined bool) string {
	t.Helper()
	dir := filepath.Join(tg.project, "kich_ban", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := library.WriteStoryFile(dir, library.StoryFile{Title: name}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "output", "novel")
	if err := ledger.Save(out, ledger.Record{Values: v, Brief: "Brief " + name, Result: ledger.CheckValues(v, nil)}); err != nil {
		t.Fatal(err)
	}
	if outlined {
		if err := store.NewStore(out).Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Mở đầu"}}); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func ledgerCheckFieldsInJSON(t *testing.T, body []byte) {
	t.Helper()
	if !bytes.Contains(body, []byte(`"short":"`)) {
		t.Fatalf("checks without short: %s", body)
	}
	// fix is string | null on every check payload that carries short.
	if !bytes.Contains(body, []byte(`"fix":null`)) && !bytes.Contains(body, []byte(`"fix":"`)) {
		t.Fatalf("checks without fix: %s", body)
	}
}

func TestLedgerAPIVisibilityAndCandidate(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	visible := ledgerValues("visible")
	hidden := ledgerValues("hidden")
	out := saveLedgerTestStory(t, tg, "01-story", visible, false)
	saveLedgerTestStory(t, tg, "99-hidden", hidden, false)
	moveStoryToUnassignedProject(t, tg.root, filepath.Join(tg.project, "kich_ban", "99-hidden"))
	if _, err := tg.g.accounts.CreateUser(webauth.NewUser{Username: "limited", Role: webauth.RoleMember, Projects: map[string]webauth.ProjectRole{"proj": webauth.ProjectView}, Password: "limited-pass"}); err != nil {
		t.Fatal(err)
	}
	cookie := tg.login("limited", "limited-pass")
	w := tg.do("GET", "/api/projects/proj/ledger", cookie, "")
	if w.Code != 200 {
		t.Fatalf("GET ledger: %d %s", w.Code, w.Body)
	}
	var response struct {
		Stories   []ledger.Story   `json:"stories"`
		Candidate *ledgerCandidate `json:"candidate"`
		Axes      []ledger.Axis    `json:"axes"`
		Median    *int             `json:"channelMedianDay7"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Stories) != 1 || response.Stories[0].ID != "proj/01-story" || strings.Contains(w.Body.String(), "hidden") {
		t.Fatalf("visibility: %s", w.Body)
	}
	if len(response.Axes) != 10 || response.Median != nil || response.Stories[0].Day7 != nil {
		t.Fatalf("real data only: %s", w.Body)
	}
	if response.Candidate == nil || response.Candidate.Story != "proj/01-story" || len(response.Candidate.Checks) != 4 || response.Candidate.Brief != "Brief 01-story" {
		t.Fatalf("candidate: %s", w.Body)
	}
	for _, check := range response.Candidate.Checks {
		if check.Short == "" {
			t.Fatalf("candidate %s has no short: %s", check.Rule, w.Body)
		}
	}
	ledgerCheckFieldsInJSON(t, w.Body.Bytes())
	if err := store.NewStore(out).Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1}}); err != nil {
		t.Fatal(err)
	}
	w = tg.do("GET", "/api/projects/proj/ledger", cookie, "")
	if decodeResp(t, w)["candidate"] != nil {
		t.Fatalf("outlined candidate: %s", w.Body)
	}
}

// TestLedgerAPIRowCarriesAddedDate: dòng sổ cái mang ngày thêm truyện (story.json
// › createdAt) làm dòng phụ thứ hai của bảng ProjBrain; truyện chưa có story.json
// thì null (app bỏ hẳn dòng phụ). Fail-before: body không có "createdAt".
func TestLedgerAPIRowCarriesAddedDate(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	v := ledgerValues("dated")
	saveLedgerTestStory(t, tg, "01-story", v, false)
	added := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	storyDir := filepath.Join(tg.project, "kich_ban", "01-story")
	if err := library.WriteStoryFile(storyDir, library.StoryFile{Title: "01-story", CreatedAt: &added}); err != nil {
		t.Fatal(err)
	}
	if sf, err := library.LoadStoryFile(storyDir); err != nil || sf == nil || sf.CreatedAt == nil {
		t.Fatalf("fixture story.json: %+v %v", sf, err)
	}
	saveLedgerTestStory(t, tg, "02-undated", ledgerValues("undated"), false)
	w := tg.do("GET", "/api/projects/proj/ledger", tg.login("admin", "admin-pass"), "")
	if w.Code != 200 {
		t.Fatalf("GET ledger: %d %s", w.Code, w.Body)
	}
	var response struct {
		Stories []map[string]any `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, story := range response.Stories {
		byID[story["id"].(string)] = story
	}
	if got := byID["proj/01-story"]["createdAt"]; got != "2026-10-24T00:00:00Z" {
		t.Fatalf("createdAt = %v, body %s", got, w.Body)
	}
	if got, ok := byID["proj/02-undated"]["createdAt"]; !ok || got != nil {
		t.Fatalf("undated createdAt = %v (present %v), body %s", got, ok, w.Body)
	}
}

func TestLedgerAPICheckSuggestAndStartGates(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	old := ledgerValues("old")
	v := old
	ledgerStub(t, tg, &v)
	saveLedgerTestStory(t, tg, "00-old", old, true)
	saveLedgerTestStory(t, tg, "00-older", old, true)
	admin := tg.login("admin", "admin-pass")
	w := tg.do("POST", "/api/projects/proj/ledger/check", admin, `{"brief":"Yêu cầu"}`)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body)
	}
	var result ledger.Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Blocking() || len(result.Checks) != 4 || result.Values != old {
		t.Fatalf("four real checks: %s", w.Body)
	}
	for _, check := range result.Checks {
		if check.Short == "" {
			t.Fatalf("check %s has no short: %s", check.Rule, w.Body)
		}
	}
	ledgerCheckFieldsInJSON(t, w.Body.Bytes())
	w = tg.do("POST", "/api/projects/proj/stories/01-story/input", admin, `{"text":"Yêu cầu"}`)
	if w.Code != 409 || decodeResp(t, w)["checks"] == nil {
		t.Fatalf("quick gate: %d %s", w.Code, w.Body)
	}
	ledgerCheckFieldsInJSON(t, w.Body.Bytes())
	var gate struct {
		Checks []ledger.Check `json:"checks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &gate); err != nil {
		t.Fatal(err)
	}
	for _, check := range gate.Checks {
		if check.Short == "" {
			t.Fatalf("gate check %s has no short: %s", check.Rule, w.Body)
		}
	}
	// Project cocreate gate must execute before moving the draft or committing a worker.
	done := make(chan struct{})
	var once sync.Once
	tg.g.launch = func(string, string, string, string) (*launchedWorker, error) { return fakeWorker(done, &once), nil }
	tg.g.transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		payload := `{"ok":true}`
		if r.URL.Path == "/api/cocreate" {
			payload = `{"active":true,"draft":"Yêu cầu gốc","messages":[]}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
	})
	draftDir := filepath.Join(tg.project, ".ainovel", "drafts", "test-draft")
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		t.Fatal(err)
	}
	w = tg.do("POST", "/api/projects/proj/cocreate/commit", admin, `{"draft":"test-draft","brief":"Yêu cầu đổi"}`)
	if w.Code != 409 || decodeResp(t, w)["checks"] == nil {
		t.Fatalf("cocreate gate: %d %s", w.Code, w.Body)
	}
	ledgerCheckFieldsInJSON(t, w.Body.Bytes())
	if _, err := os.Stat(draftDir); err != nil {
		t.Fatalf("blocked draft moved: %v", err)
	}
	// Warnings do not block, provided combination and distance pass.
	v = ledgerValues("new")
	v[2] = old[2]
	v[6] = old[6]
	w = tg.do("POST", "/api/projects/proj/ledger/check", admin, `{"brief":"Yêu cầu"}`)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Blocking() || result.Checks[2].Level != "warning" {
		t.Fatalf("expected warnings only: %s", w.Body)
	}
	w = tg.do("POST", "/api/projects/proj/stories/01-story/input", admin, `{"text":"Yêu cầu"}`)
	if w.Code != 200 {
		t.Fatalf("warning-only quick start: %d %s", w.Code, w.Body)
	}
	w = tg.do("POST", "/api/projects/proj/ledger/suggest", admin, `{"brief":"Yêu cầu"}`)
	if w.Code != 200 || decodeResp(t, w)["brief"] != "Yêu cầu đã đổi" {
		t.Fatalf("suggest: %d %s", w.Code, w.Body)
	}
}
func TestLedgerArchitectStoresBlockedBrief(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	v := ledgerValues("old")
	saveLedgerTestStory(t, tg, "00-old", v, true)
	st := storyEngineStore(tg)
	args, _ := json.Marshal(map[string]any{"type": "premise", "content": "Yêu cầu mới", "axes": v})
	result, err := tools.NewSaveFoundationTool(st).Execute(t.Context(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"blocking":true`) {
		t.Fatalf("brief gate: %s", result)
	}
	ledgerCheckFieldsInJSON(t, result)
	rec, err := ledger.Load(st.Dir())
	if err != nil || rec == nil || !rec.Result.Blocking() || rec.Brief != "Yêu cầu mới" {
		t.Fatalf("saved ledger: %+v %v", rec, err)
	}
	args, _ = json.Marshal(map[string]any{"type": "outline", "content": []domain.OutlineEntry{{Chapter: 1, Title: "Không được lưu"}}, "axes": nil})
	result, err = tools.NewSaveFoundationTool(st).Execute(t.Context(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"blocking":true`) {
		t.Fatalf("outline gate: %s", result)
	}
	outline, err := st.Outline.LoadOutline()
	if err != nil || len(outline) != 0 {
		t.Fatalf("blocked outline saved: %+v %v", outline, err)
	}
}

// TestLedgerAPIRowUsesEngineBookTitle: truyện tạo từ brief có story.json.title =
// tên tạm (dòng đầu brief); tên thật engine đặt nằm trong meta/book.json. Dòng sổ
// cái phải hiện tên sách thật, không phải câu brief. Fail-before: storyDisplayTitle
// đọc story.json trước nên trả về "Viết kịch bản…".
func TestLedgerAPIRowUsesEngineBookTitle(t *testing.T) {
	tg := newConfigTestGateway(t)
	defer tg.g.Close()
	out := saveLedgerTestStory(t, tg, "01-truyen", ledgerValues("working"), false)
	working := "Viết kịch bản (dàn ý chi tiết) cho một truyện audio tiếng Việt, thể loại trọng sinh"
	if err := library.WriteStoryFile(filepath.Join(tg.project, "kich_ban", "01-truyen"), library.StoryFile{Title: working}); err != nil {
		t.Fatal(err)
	}
	real := "Thiên Kim Thật Đòi Lại Giọng Nói"
	if err := store.NewStore(out).Book.Save(domain.BookMetadata{Title: real, Synopsis: "Cô con gái thật trở về đòi lại giọng nói."}); err != nil {
		t.Fatal(err)
	}
	w := tg.do("GET", "/api/projects/proj/ledger", tg.login("admin", "admin-pass"), "")
	if w.Code != 200 {
		t.Fatalf("GET ledger: %d %s", w.Code, w.Body)
	}
	var response struct {
		Stories []ledger.Story `json:"stories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var found *ledger.Story
	for i := range response.Stories {
		if response.Stories[i].ID == "proj/01-truyen" {
			found = &response.Stories[i]
		}
	}
	if found == nil || found.Title != real {
		t.Fatalf("story title = %+v, want %q: %s", response.Stories, real, w.Body)
	}
}
