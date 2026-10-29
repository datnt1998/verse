package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/llmconfig"
)

// This file is the phase-06 gate: a hosted story must be creatable, writable,
// readable and commentable over HTTP with the library root present but empty
// and read-only. Any code path that still derives a file location from the
// engine directory (Dir() == "") writes into the process cwd or the library
// root — both are asserted empty after the run.
//
// The engine is real (host.New + the production worker set), driven by a local
// OpenAI-compatible httptest endpoint that replays the same plan → draft →
// check → commit sequence internal/host's scriptedWriterModel uses. No network,
// no real provider, no test-only production seam.

// cutoverChapterRe mirrors the engine's task wording so the scripted endpoint
// can tell which chapter it is writing.
var cutoverChapterRe = regexp.MustCompile(`(?:(?:Viết(?: lại)?|Gọt giũa) chương|写第)\s*(\d+)`)

// cutoverModelText flattens an OpenAI wire content field (string or parts).
func cutoverModelText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return ""
}

// cutoverWriterStep is the scripted writer's next tool call, keyed by how many
// tool results the conversation already carries — the same shape as
// internal/host's scriptedWriterModel.
func cutoverWriterStep(chapter, toolResults int) (string, map[string]any) {
	switch toolResults {
	case 0:
		return "plan_chapter", map[string]any{
			"chapter": chapter, "title": fmt.Sprintf("Chương %d", chapter),
			"goal": "Đẩy tiến tuyến chính", "conflict": "Nhân vật chính gặp trở ngại", "hook": "Kết thúc bằng hồi hộp",
		}
	case 1:
		return "draft_chapter", map[string]any{
			"chapter": chapter, "mode": "write",
			"content": strings.Repeat(fmt.Sprintf("Đoạn văn chính của chương %d, nhân vật chính dò dẫm tiến lên trong bóng tối.", chapter), 20),
		}
	case 2:
		return "check_consistency", map[string]any{"chapter": chapter}
	default:
		return "commit_chapter", map[string]any{
			"chapter": chapter, "title": fmt.Sprintf("Chương %d", chapter), "summary": fmt.Sprintf("Tóm tắt chương %d", chapter),
			"characters": []string{"Nhân vật chính"}, "key_events": []string{"Đẩy tiến"},
			"timeline_events": []any{}, "foreshadow_updates": []any{},
			"relationship_changes": []any{}, "state_changes": []any{}, "cast_intros": []any{},
			"hook_type": "crisis", "dominant_strand": "quest", "feedback": nil,
			"address_terms":  nil,
			"fixed_comments": nil,
			"places":         nil, "objects": nil,
		}
	}
}

// newCutoverModelServer serves an OpenAI-compatible chat endpoint that scripts
// the writer. Requests whose tool set has no plan_chapter (any non-writer role)
// get a plain text answer so unrelated agents terminate instead of looping on
// unknown tools.
func newCutoverModelServer(t *testing.T) *httptest.Server {
	t.Helper()
	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			jsonError(w, http.StatusNotFound, "unexpected model path "+r.URL.Path)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		var req struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		mu.Lock()
		calls++
		mu.Unlock()

		chapter, toolResults, writer := 0, 0, false
		for _, tool := range req.Tools {
			if tool.Function.Name == "plan_chapter" {
				writer = true
			}
		}
		for _, m := range req.Messages {
			switch m.Role {
			case "tool":
				toolResults++
			case "user":
				if match := cutoverChapterRe.FindStringSubmatch(cutoverModelText(m.Content)); match != nil {
					chapter, _ = strconv.Atoi(match[1])
				}
			}
		}
		if chapter <= 0 {
			chapter = 1
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		write := func(payload string) bool {
			if _, err := io.WriteString(w, payload); err != nil {
				return false
			}
			if flusher != nil {
				flusher.Flush()
			}
			return true
		}
		if !req.Stream {
			// Non-streaming callers (nothing in the engine run needs it today):
			// answer with the same script in chat.completion shape.
			w.Header().Set("Content-Type", "application/json")
			var message map[string]any
			if writer {
				name, args := cutoverWriterStep(chapter, toolResults)
				data, _ := json.Marshal(args)
				message = map[string]any{
					"role": "assistant", "content": nil,
					"tool_calls": []any{map[string]any{
						"id": "call_" + name, "type": "function",
						"function": map[string]any{"name": name, "arguments": string(data)},
					}},
				}
			} else {
				message = map[string]any{"role": "assistant", "content": "ok"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model": "scripted", "choices": []any{map[string]any{
					"index": 0, "message": message, "finish_reason": "tool_calls",
				}},
				"usage": map[string]any{"prompt_tokens": 12, "completion_tokens": 8, "total_tokens": 20},
			})
			return
		}

		var chunk map[string]any
		finish := "tool_calls"
		if writer {
			name, args := cutoverWriterStep(chapter, toolResults)
			data, _ := json.Marshal(args)
			chunk = map[string]any{
				"model": "scripted",
				"choices": []any{map[string]any{
					"index": 0,
					"delta": map[string]any{
						"role": "assistant",
						"tool_calls": []any{map[string]any{
							"index": 0, "id": "call_" + name, "type": "function",
							"function": map[string]any{"name": name, "arguments": string(data)},
						}},
					},
				}},
			}
		} else {
			finish = "stop"
			chunk = map[string]any{
				"model":   "scripted",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "ok"}}},
			}
		}
		payload, err := json.Marshal(chunk)
		if err != nil {
			return
		}
		if !write("data: " + string(payload) + "\n\n") {
			return
		}
		stop, _ := json.Marshal(map[string]any{
			"model": "scripted",
			"choices": []any{map[string]any{
				"index": 0, "delta": map[string]any{}, "finish_reason": finish,
			}},
		})
		if !write("data: " + string(stop) + "\n\n") {
			return
		}
		usage, _ := json.Marshal(map[string]any{
			"model":   "scripted",
			"choices": []any{},
			"usage":   map[string]any{"prompt_tokens": 12, "completion_tokens": 8, "total_tokens": 20},
		})
		if !write("data: " + string(usage) + "\n\n") {
			return
		}
		write("data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// preseedCutoverStory puts the freshly created (over HTTP) story into the
// writing phase with a three-chapter outline and per-chapter review approval —
// exactly the state a user reaches before asking for the first chapter.
func preseedCutoverStory(t *testing.T, pool *postgres.Pool, project, story string) {
	t.Helper()
	ctx := context.Background()
	st, err := bootstrap.OpenStoryStoreInternal(ctx, pool, project, story)
	if err != nil {
		t.Fatalf("open scoped store: %v", err)
	}
	if err := st.Init(); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("progress init: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	outline := make([]domain.OutlineEntry, 0, 3)
	for i := 1; i <= 3; i++ {
		outline = append(outline, domain.OutlineEntry{Chapter: i, Title: fmt.Sprintf("Chương %d", i), CoreEvent: "việc"})
	}
	if err := st.Outline.SaveOutline(outline); err != nil {
		t.Fatalf("outline: %v", err)
	}
	if err := st.RunMeta.Init("default", "cutover", "scripted"); err != nil {
		t.Fatalf("run meta: %v", err)
	}
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatalf("advance mode: %v", err)
	}
}

// installCutoverLauncher replaces the child-process launch with an in-process
// worker: the same env-scoped store + self-claimed lease a hosted child builds
// in openWorkerStorage, served over a loopback listener so the gateway talks to
// it through its normal HTTP proxy.
func installCutoverLauncher(t *testing.T, g *Gateway, pool *postgres.Pool, modelURL string) {
	t.Helper()
	g.launchHosted = func(key, project, story string) (*launchedWorker, error) {
		ctx := context.Background()
		nar := postgres.NewNarrative(pool)
		scope, err := nar.Resolve(ctx, project, story, nil, false)
		if err != nil {
			return nil, err
		}
		lease, err := postgres.NewStoryLease(ctx, nar, scope, workerLeaseTTL)
		if err != nil {
			return nil, err
		}
		backend := postgres.NewBackend(ctx, pool, nar, scope, postgres.WithLeaseFence(lease))
		st, err := store.NewFromBackend(backend)
		if err != nil {
			_ = lease.Close()
			return nil, err
		}
		cfg := bootstrap.Config{
			Provider: "cutover", ModelName: "scripted", Style: "default",
			AdvanceMode: "review",
			Providers: map[string]llmconfig.ProviderConfig{
				"cutover": {Type: "openai", BaseURL: modelURL, APIKey: "test-only"},
			},
		}
		bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(cfg.OutputDir))
		if bundle.Err != nil {
			_ = lease.Close()
			return nil, bundle.Err
		}
		h, err := host.New(cfg, bundle, host.WithStorage(host.Storage{Store: st, Lease: lease}))
		if err != nil {
			_ = lease.Close()
			return nil, err
		}
		stop := make(chan struct{})
		done := make(chan struct{})
		server := httptest.NewServer(newWorkerHandler(h, g.workerToken, stop))
		var once sync.Once
		closeWorker := func() {
			once.Do(func() {
				close(stop)
				server.Close()
				h.Close()
				_ = lease.Close()
				close(done)
			})
		}
		return &launchedWorker{
			addr:  strings.TrimPrefix(server.URL, "http://"),
			stdin: closerFunc(func() error { closeWorker(); return nil }),
			done:  done,
			kill:  func() error { closeWorker(); return nil },
		}, nil
	}
}

// assertNoBusinessFiles refuses any entry under dir, recursively. A 0555 root
// already fails writes, but the assertion proves nothing even tried.
func assertNoBusinessFiles(t *testing.T, dir, label string) {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("%s %q: %v", label, dir, err)
	}
	var found []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dir {
			return nil
		}
		found = append(found, strings.TrimPrefix(p, dir+string(os.PathSeparator)))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s %q: %v", label, dir, err)
	}
	if len(found) > 0 {
		sort.Strings(found)
		t.Fatalf("%s %q must stay empty; found %d entrie(s): %v", label, dir, len(found), found)
	}
}

// cutoverSnapshot is the slice of the worker snapshot the gate polls.
type cutoverSnapshot struct {
	IsRunning            bool   `json:"isRunning"`
	Phase                string `json:"phase"`
	CompletedCount       int    `json:"completedCount"`
	AdvanceMode          string `json:"advanceMode"`
	AdvancePermitChapter int    `json:"advancePermitChapter"`
	WaitingFor           string `json:"waitingFor"`
	RecoveryLabel        string `json:"recoveryLabel"`
}

// waitCutoverChapter parks the test until the engine wrote one chapter and
// stopped at the review gate.
func waitCutoverChapter(t *testing.T, g *Gateway, cookie, base string) cutoverSnapshot {
	t.Helper()
	deadline := time.Now().Add(120 * time.Second)
	var last string
	for {
		w := doFrom(t, g, http.MethodGet, base+"/snapshot", cookie, "", "127.0.0.1:1234", nil)
		if w.Code == http.StatusOK {
			last = w.Body.String()
			var snap cutoverSnapshot
			if err := json.Unmarshal(w.Body.Bytes(), &snap); err == nil {
				if !snap.IsRunning && snap.Phase == string(domain.PhaseWriting) && snap.CompletedCount >= 1 {
					return snap
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("engine did not finish chapter 1 within the deadline; last snapshot %s (status %d)", last, w.Code)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestStorageCutoverHostedOverHTTP is the phase-06 gate: create → one scripted
// chapter → read → comment, all over HTTP, with an empty read-only library
// root and an empty process cwd.
func TestStorageCutoverHostedOverHTTP(t *testing.T) {
	// Isolate every host-derived path before any production code runs: a
	// hosted run that still derives a directory writes here and is caught.
	work := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	const projectSlug = "du-an-cutover"
	// The fixture anchors the project + membership matrix and one throwaway
	// story; the story under test is created over HTTP below.
	f := provisionHostedGateway(t, projectSlug, "truyen-neo")

	// The library root exists, is empty and is read-only: hosted mode must
	// never write into it, and nothing may even try.
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod library root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	f.g.setRoot(root)

	model := newCutoverModelServer(t)
	installCutoverLauncher(t, f.g, f.pool, model.URL)

	session := sessionFrom(t, doFrom(t, f.g, http.MethodPost, "/api/login", "", `{"username":"hosted-owner","password":"mat-khau-12345"}`, "192.0.2.1:1234", nil))

	// 1. Create the story over HTTP. Hosted creation has no directory to make.
	created := doFrom(t, f.g, http.MethodPost, "/api/projects/"+projectSlug+"/stories", session,
		`{"name":"Truyện cắt thuỷ"}`, "192.0.2.1:1234", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("create story: %d %s", created.Code, created.Body.String())
	}
	var storyBody struct {
		ID      string `json:"id"`
		Project string `json:"project"`
		Story   string `json:"story"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &storyBody); err != nil {
		t.Fatalf("decode story %q: %v", created.Body.String(), err)
	}
	if storyBody.Project != projectSlug || storyBody.Story == "" {
		t.Fatalf("created story identity: %+v", storyBody)
	}
	base := "/api/projects/" + projectSlug + "/stories/" + storyBody.Story

	// The store's own creation path must not have written a directory either.
	assertNoBusinessFiles(t, root, "library root")
	assertNoBusinessFiles(t, work, "process cwd")

	preseedCutoverStory(t, f.pool, storyBody.Project, storyBody.Story)
	preseeded := doFrom(t, f.g, http.MethodGet, base+"/chapters", session, "", "127.0.0.1:1234", nil)
	if preseeded.Code != http.StatusOK {
		t.Fatalf("chapters before writing: %d %s", preseeded.Code, preseeded.Body.String())
	}

	// 2. Approve chapter 1: review mode grants one permit and the scripted
	//    writer runs plan → draft → check → commit in rows only.
	next := doFrom(t, f.g, http.MethodPost, base+"/next", session, `{}`, "192.0.2.1:1234", nil)
	if next.Code != http.StatusOK {
		t.Fatalf("approve chapter 1: %d %s", next.Code, next.Body.String())
	}
	snap := waitCutoverChapter(t, f.g, session, base)
	if snap.CompletedCount != 1 {
		t.Fatalf("chapter 1 must be the only completed chapter, got %+v", snap)
	}
	// The permit is cleared only after the advance gate has seen the commit
	// checkpoint with no interrupted pending commit left behind, so this is the
	// proof that commit_chapter finished all of its steps on rows.
	if snap.AdvancePermitChapter != 0 {
		t.Fatalf("advance permit not consumed: %+v", snap)
	}
	if snap.AdvanceMode != string(domain.ChapterAdvanceReview) || snap.WaitingFor != "gate" {
		t.Fatalf("engine must stop at the review gate: %+v", snap)
	}

	// 3. Read the chapter back over HTTP (gateway read hook, not the worker).
	read := doFrom(t, f.g, http.MethodGet, base+"/chapters/1", session, "", "127.0.0.1:1234", nil)
	if read.Code != http.StatusOK {
		t.Fatalf("read chapter 1: %d %s", read.Code, read.Body.String())
	}
	if !strings.Contains(read.Body.String(), "Đoạn văn chính của chương 1") {
		t.Fatalf("chapter 1 text missing from the read: %s", read.Body.String())
	}
	list := doFrom(t, f.g, http.MethodGet, base+"/chapters", session, "", "127.0.0.1:1234", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"completed":true`) {
		t.Fatalf("chapter list must report the written chapter: %d %s", list.Code, list.Body.String())
	}

	// 4. Comment on the chapter through the hosted comment rows and read it back.
	commented := doFrom(t, f.g, http.MethodPost, base+"/chapters/1/comments", session,
		`{"quote":"dò dẫm","text":"Câu này cần mạnh hơn","chips":["Kịch tính hơn"]}`, "192.0.2.1:1234", nil)
	if commented.Code != http.StatusCreated {
		t.Fatalf("add comment: %d %s", commented.Code, commented.Body.String())
	}
	comments := doFrom(t, f.g, http.MethodGet, base+"/chapters/1/comments", session, "", "127.0.0.1:1234", nil)
	if comments.Code != http.StatusOK || !strings.Contains(comments.Body.String(), "Câu này cần mạnh hơn") {
		t.Fatalf("comment round-trip: %d %s", comments.Code, comments.Body.String())
	}

	// 5. Nothing may have been written beside the rows: no engine directory, no
	//    scratch residue in the cwd, no file in the read-only library root.
	assertNoBusinessFiles(t, root, "library root")
	assertNoBusinessFiles(t, work, "process cwd")

	// Stop the in-process worker before the fixture closes the gateway, so the
	// lease is released and the test fails on a stuck run instead of a timeout.
	f.g.closeBook(storyBody.ID)
	if _, running := f.g.workerState(storyBody.ID); running {
		t.Fatal("worker still reports a running engine after the review gate")
	}
}
