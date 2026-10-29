package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/agent"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/jackc/pgx/v5/pgtype"
)

const testCookie = "viderse_session"

type fakeQueries struct {
	run     gen.AgentRun
	script  gen.Script
	runErr  error
	scripts []gen.Script
	runs    []gen.AgentRun
}

func (f *fakeQueries) GetRun(_ context.Context, id pgtype.UUID) (gen.AgentRun, error) {
	if f.runErr != nil || id != f.run.ID {
		return gen.AgentRun{}, errors.New("not found")
	}
	return f.run, nil
}

func (f *fakeQueries) ListRuns(_ context.Context, _ int32) ([]gen.AgentRun, error) {
	return f.runs, nil
}

func (f *fakeQueries) GetScript(_ context.Context, id pgtype.UUID) (gen.Script, error) {
	if id != f.script.ID {
		return gen.Script{}, errors.New("not found")
	}
	return f.script, nil
}

func (f *fakeQueries) ListScripts(_ context.Context, _ int32) ([]gen.Script, error) {
	return f.scripts, nil
}

type fakeRunner struct {
	submitted script.Brief
	runID     pgtype.UUID
}

func (f *fakeRunner) Submit(_ context.Context, brief script.Brief) (pgtype.UUID, error) {
	f.submitted = brief
	return f.runID, nil
}

type testGateway struct {
	t       *testing.T
	g       *Gateway
	runner  *fakeRunner
	queries *fakeQueries
}

func mustUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	id, err := agent.ParseUUID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func newTestGateway(t *testing.T) *testGateway {
	t.Helper()
	runID := mustUUID(t, "11111111-2222-3333-4444-555555555555")
	scriptID := mustUUID(t, "66666666-7777-8888-9999-000000000000")
	run := gen.AgentRun{
		ID:       runID,
		Status:   "done",
		Provider: "openrouter",
		Model:    "model-x",
		Brief:    []byte(`{"topic":"chủ đề","target_seconds":60}`),
		CreatedAt: pgtype.Timestamptz{
			Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Valid: true},
	}
	scr := gen.Script{
		ID:             scriptID,
		RunID:          runID,
		Title:          "Kịch bản A",
		Sections:       []byte(`[{"kind":"intro","title":"Mở đầu","narration":"xin chào","rehook":false,"cta":false,"scenes":[]}]`),
		FullText:       "xin chào",
		WordCount:      2,
		EstDurationSec: 3,
	}
	queries := &fakeQueries{run: run, script: scr, runs: []gen.AgentRun{run}, scripts: []gen.Script{scr}}
	runner := &fakeRunner{runID: runID}

	accounts := identity.NewStore(filepath.Join(t.TempDir(), "web-accounts.json"), identity.Options{})
	if _, err := accounts.CreateUser(identity.NewUser{
		Username: "admin", Role: identity.RoleAdmin, Password: "admin-pass",
	}); err != nil {
		t.Fatal(err)
	}
	return &testGateway{t: t, g: NewGateway(accounts, queries, runner), runner: runner, queries: queries}
}

func (tg *testGateway) do(method, path, cookie, body string) *httptest.ResponseRecorder {
	tg.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	}
	if method == http.MethodPost || method == http.MethodPut || body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func (tg *testGateway) sessionCookie() string {
	tg.t.Helper()
	w := tg.do(http.MethodPost, "/api/login", "", `{"username":"admin","password":"admin-pass"}`)
	if w.Code != http.StatusOK {
		tg.t.Fatalf("login status %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == testCookie && c.Value != "" {
			return c.Value
		}
	}
	tg.t.Fatalf("no session cookie (status %d)", w.Code)
	return ""
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body không phải JSON: %q", w.Body.String())
	}
	return m
}

func TestRunRoutesRequireSession(t *testing.T) {
	tg := newTestGateway(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/runs"},
		{http.MethodPost, "/api/runs"},
		{http.MethodGet, "/api/runs/11111111-2222-3333-4444-555555555555"},
		{http.MethodGet, "/api/scripts"},
		{http.MethodGet, "/api/scripts/66666666-7777-8888-9999-000000000000"},
	} {
		w := tg.do(tc.method, tc.path, "", "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestListRuns(t *testing.T) {
	tg := newTestGateway(t)
	w := tg.do(http.MethodGet, "/api/runs", tg.sessionCookie(), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	runs, _ := decode(t, w)["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("runs %v", runs)
	}
	row := runs[0].(map[string]any)
	if row["id"] != "11111111-2222-3333-4444-555555555555" || row["status"] != "done" {
		t.Fatalf("run wire %v", row)
	}
	if row["brief"].(map[string]any)["topic"] != "chủ đề" {
		t.Fatalf("brief wire %v", row["brief"])
	}
}

func TestGetRun(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/runs/11111111-2222-3333-4444-555555555555", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if decode(t, w)["run"].(map[string]any)["model"] != "model-x" {
		t.Fatalf("run wire %v", decode(t, w))
	}

	w = tg.do(http.MethodGet, "/api/runs/khong-phai-uuid", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("bad uuid status %d", w.Code)
	}

	w = tg.do(http.MethodGet, "/api/runs/99999999-9999-9999-9999-999999999999", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing run status %d", w.Code)
	}
}

func TestCreateRun(t *testing.T) {
	tg := newTestGateway(t)
	w := tg.do(http.MethodPost, "/api/runs", tg.sessionCookie(), `{"topic":"về nước","target_seconds":30}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if tg.runner.submitted.Topic != "về nước" {
		t.Fatalf("brief không vào runner: %+v", tg.runner.submitted)
	}
	if decode(t, w)["run"].(map[string]any)["id"] != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("run wire %v", decode(t, w))
	}
}

func TestListAndGetScript(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.sessionCookie()

	w := tg.do(http.MethodGet, "/api/scripts", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	scripts, _ := decode(t, w)["scripts"].([]any)
	if len(scripts) != 1 || scripts[0].(map[string]any)["title"] != "Kịch bản A" {
		t.Fatalf("scripts %v", scripts)
	}

	w = tg.do(http.MethodGet, "/api/scripts/66666666-7777-8888-9999-000000000000", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	scr := decode(t, w)["script"].(map[string]any)
	sections, _ := scr["sections"].([]any)
	if len(sections) != 1 || sections[0].(map[string]any)["title"] != "Mở đầu" {
		t.Fatalf("sections %v", scr["sections"])
	}

	w = tg.do(http.MethodGet, "/api/scripts/khong-phai-uuid", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("bad uuid status %d", w.Code)
	}
}
