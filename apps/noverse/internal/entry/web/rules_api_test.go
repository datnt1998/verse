package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// normalizerStub là máy chủ tương thích OpenAI: trả cấu trúc theo nguyên văn quy tắc
// (user message cuối). "cấm X" → cụm cấm X; còn lại → lời dặn thuần. fail=true → HTTP 401.
type normalizerStub struct {
	*httptest.Server
	calls atomic.Int32
	fail  atomic.Bool
}

func newNormalizerStub(t *testing.T) *normalizerStub {
	t.Helper()
	s := &normalizerStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		if s.fail.Load() {
			// 401 là lỗi dứt khoát (không retry); 5xx sẽ bị retry đến hết timeout.
			http.Error(w, `{"error":{"message":"invalid api key","type":"invalid_request_error","code":"invalid_api_key"}}`, http.StatusUnauthorized)
			return
		}
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		user := ""
		for _, m := range req.Messages {
			if m.Role == "user" {
				if text, ok := m.Content.(string); ok {
					user = strings.TrimSpace(text)
				}
			}
		}
		out := map[string]any{"structured": map[string]any{"genre": "", "forbidden_chars": []string{}, "forbidden_phrases": []string{}, "fatigue_words": []any{}}, "preferences": user, "uncertain": []string{}, "is_uncertain": false, "uncertain_note": nil, "options": []string{}}
		if user == "chương đừng quá dài" {
			out["uncertain"] = []string{"Chưa rõ “quá dài” là bao nhiêu"}
			out["is_uncertain"] = true
			out["uncertain_note"] = "Noverse chưa chắc “quá dài” là bao nhiêu"
			out["options"] = []string{"Khoảng 3.000 chữ mỗi chương", "Ngắn hơn chương trước một chút"}
		}
		if phrase, ok := strings.CutPrefix(user, "cấm "); ok {
			out["structured"].(map[string]any)["forbidden_phrases"] = []string{phrase}
			out["preferences"] = ""
		}
		content, _ := json.Marshal(out)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "x", "object": "chat.completion", "model": "stub",
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": string(content)}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	t.Cleanup(s.Close)
	return s
}

func (tg *configTestGateway) useStub(t *testing.T, s *normalizerStub) {
	t.Helper()
	tg.globalConfig(t, fmt.Sprintf(`{"provider":"openai","model":"stub","providers":{"openai":{"type":"openai","api_key":"sk-stub-1234567890","base_url":%q,"models":[{"name":"stub"}]}}}`, s.URL+"/v1"))
}

const rulesStoryBase = "/api/projects/proj/stories/01-story/rules"

func storyEngineStore(tg *configTestGateway) *store.Store {
	return store.NewStore(filepath.Join(tg.story, "output", "novel"))
}

func TestRulesAPI_UnderstandApplyEffectiveKeep(t *testing.T) {
	tg := newConfigTestGateway(t)
	stub := newNormalizerStub(t)
	tg.useStub(t, stub)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")

	// understand: không lưu gì.
	w := tg.do("POST", "/api/rules/understand", admin, `{"text":"cấm đột nhiên"}`)
	if w.Code != 200 {
		t.Fatalf("understand: %d %s", w.Code, w.Body)
	}
	u := decodeResp(t, w)
	if u["status"] != "ready" || len(u["uncertain"].([]any)) != 0 {
		t.Fatalf("understand: %v", u)
	}
	if got := u["structured"].(map[string]any)["forbidden_phrases"].([]any); len(got) != 1 || got[0] != "đột nhiên" {
		t.Fatalf("structured: %v", u["structured"])
	}
	if _, err := os.Stat(filepath.Join(tg.home, ".ainovel", "rules")); err == nil {
		t.Fatal("understand không được ghi tệp quy tắc")
	}

	// Chuẩn hoá lỗi → 200 + degraded, không phải lỗi.
	stub.fail.Store(true)
	w = tg.do("POST", "/api/rules/understand", admin, `{"text":"giọng văn lạnh"}`)
	if w.Code != 200 {
		t.Fatalf("understand degraded: %d %s", w.Code, w.Body)
	}
	if u = decodeResp(t, w); u["status"] != "degraded" || u["preferences"] != "giọng văn lạnh" {
		t.Fatalf("degraded: %v", u)
	}
	stub.fail.Store(false)

	// quy-tac.md theo cấp: PUT /api/rules/file hoạt động cho tên này.
	w = tg.do("PUT", "/api/rules/file", writer, `{"scope":"story","project":"proj","story":"01-story","name":"quy-tac.md","text":"- cấm đột nhiên\n- giọng văn lạnh\n"}`)
	if w.Code != 200 {
		t.Fatalf("PUT quy-tac.md: %d %s", w.Code, w.Body)
	}
	w = tg.do("PUT", "/api/rules/file", admin, `{"scope":"global","name":"quy-tac.md","text":"- cấm chữ lạ\n"}`)
	if w.Code != 200 {
		t.Fatalf("PUT global quy-tac.md: %d %s", w.Code, w.Body)
	}

	// effective: từng dòng có chip, snapshot chưa áp dụng thì chỉ có mặc định.
	w = tg.do("GET", rulesStoryBase+"/effective", writer, "")
	if w.Code != 200 {
		t.Fatalf("effective: %d %s", w.Code, w.Body)
	}
	var eff struct {
		Snapshot map[string]any `json:"snapshot"`
		Items    []struct {
			Text  string           `json:"text"`
			Scope string           `json:"scope"`
			Chips []map[string]any `json:"chips"`
		} `json:"items"`
		Runtime []any `json:"runtime"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &eff); err != nil {
		t.Fatal(err)
	}
	if len(eff.Items) != 3 || eff.Items[0].Scope != "global" || eff.Items[0].Text != "cấm chữ lạ" || eff.Items[1].Scope != "story" {
		t.Fatalf("items: %+v", eff.Items)
	}
	if c := eff.Items[1].Chips; len(c) != 1 || c[0]["kind"] != "ban" || c[0]["text"] != "đột nhiên" {
		t.Fatalf("chips: %v", c)
	}
	if eff.Runtime == nil {
		t.Fatal("runtime phải là mảng")
	}

	// Trọng tài đã học một luật; apply giữ nó và dựng snapshot mới.
	st := storyEngineStore(tg)
	if err := st.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, InProgressChapter: 2, CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	// Truyện đã chạy thì luôn có snapshot (ensureUserRules): chương 2 bắt đầu với bản này.
	initial := rules.BuildSnapshot([]rules.Candidate{rules.SystemDefaults()})
	if err := st.UserRules.Save(&initial); err != nil {
		t.Fatal(err)
	}
	if err := st.UserRules.AddRuntime(store.RuntimeRule{ID: "rt-1", Text: "cấm lặp lặp", Chapter: 1, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	callsBefore := stub.calls.Load()
	w = tg.do("POST", rulesStoryBase+"/apply", writer, `{}`)
	if w.Code != 200 {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	applied := decodeResp(t, w)
	if applied["effectiveFrom"] != float64(3) {
		t.Fatalf("effectiveFrom: %v", applied)
	}
	phrases := applied["snapshot"].(map[string]any)["structured"].(map[string]any)["forbidden_phrases"].([]any)
	// 4 mặc định + đột nhiên (truyện) + chữ lạ (toàn cục); luật runtime thêm tay không có candidate nên không đóng góp cụm cấm.
	if len(phrases) != 6 {
		t.Fatalf("phrases: %v", phrases)
	}
	// Apply dùng lại cache của effective: dòng quy tắc không gọi model nữa (chỉ nguồn mới/ runtime).
	if extra := stub.calls.Load() - callsBefore; extra > 1 {
		t.Fatalf("apply gọi model %d lần, các dòng đã có cache", extra)
	}

	// Chương 2 đang viết: vẫn thấy snapshot cũ.
	pinned, _ := st.UserRules.Load()
	for _, p := range pinned.Structured.ForbiddenPhrases {
		if p == "đột nhiên" {
			t.Fatal("chương đang viết phải giữ snapshot cũ")
		}
	}
	if current, _ := st.UserRules.LoadCurrent(); len(current.Structured.ForbiddenPhrases) <= len(pinned.Structured.ForbiddenPhrases) {
		t.Fatalf("snapshot hiện hành phải mới hơn bản ghim: %v", current.Structured.ForbiddenPhrases)
	}

	// keep: nối "- …" vào quy-tac.md của truyện, bỏ khỏi runtime.
	w = tg.do("POST", rulesStoryBase+"/runtime/rt-1/keep", writer, `{}`)
	if w.Code != 200 {
		t.Fatalf("keep: %d %s", w.Code, w.Body)
	}
	data, _ := os.ReadFile(filepath.Join(tg.story, ".ainovel", "rules", "quy-tac.md"))
	if !strings.HasSuffix(string(data), "- cấm lặp lặp\n") || !strings.Contains(string(data), "- cấm đột nhiên\n") {
		t.Fatalf("quy-tac.md: %q", data)
	}
	if list, _ := st.UserRules.Runtime(); len(list) != 0 {
		t.Fatalf("runtime: %+v", list)
	}
	w = tg.do("POST", rulesStoryBase+"/runtime/rt-1/keep", writer, `{}`)
	if w.Code != 404 {
		t.Fatalf("keep lần hai: %d", w.Code)
	}
	w = tg.do("DELETE", rulesStoryBase+"/runtime/rt-1", writer, "")
	if w.Code != 404 {
		t.Fatalf("delete id lạ: %d", w.Code)
	}
}

func TestRulesAPI_NeedsModelAndPermissions(t *testing.T) {
	tg := newConfigTestGateway(t) // chưa có cấu hình model
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	w := tg.do("POST", "/api/rules/understand", admin, `{"text":"cấm đột nhiên"}`)
	if w.Code != 409 || decodeResp(t, w)["needsConfig"] != true {
		t.Fatalf("understand không model: %d %s", w.Code, w.Body)
	}
	w = tg.do("POST", rulesStoryBase+"/apply", admin, `{}`)
	if w.Code != 409 || decodeResp(t, w)["needsConfig"] != true {
		t.Fatalf("apply không model: %d %s", w.Code, w.Body)
	}
	// effective không cần model: dòng chưa chuẩn hoá hạ cấp thành lời dặn.
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"story","project":"proj","story":"01-story","name":"quy-tac.md","text":"- cấm đột nhiên\n"}`)
	w = tg.do("GET", rulesStoryBase+"/effective", admin, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"degraded"`) {
		t.Fatalf("effective không model: %d %s", w.Code, w.Body)
	}
	// reader: không ghi được.
	if w = tg.do("POST", "/api/rules/understand", reader, `{"text":"x"}`); w.Code != 403 {
		t.Fatalf("reader understand: %d", w.Code)
	}
	if w = tg.do("POST", rulesStoryBase+"/apply", reader, `{}`); w.Code != 403 {
		t.Fatalf("reader apply: %d", w.Code)
	}
	if w = tg.do("GET", rulesStoryBase+"/effective", reader, ""); w.Code != 200 {
		t.Fatalf("reader effective: %d", w.Code)
	}
	// The project named only in the body is what gets authorized: its owner
	// reaches the model check, and a non-member's refusal reads the same
	// whether that project exists or not.
	writer := tg.login("writer", "writer-pass")
	if w = tg.do("POST", "/api/rules/understand", writer, `{"text":"x","scope":"project","project":"proj"}`); w.Code != 409 {
		t.Fatalf("owner understand on own project: %d %s", w.Code, w.Body)
	}
	outsider := tg.login("outsider", "outsider-pass")
	real := tg.do("POST", "/api/rules/understand?scope=project", outsider, `{"text":"x","scope":"project","project":"proj"}`)
	missing := tg.do("POST", "/api/rules/understand?scope=project", outsider, `{"text":"x","scope":"project","project":"ghost"}`)
	if real.Code != 403 || missing.Code != real.Code || missing.Body.String() != real.Body.String() {
		t.Fatalf("understand refusal depends on existence: %d %s vs %d %s", real.Code, real.Body, missing.Code, missing.Body)
	}
	if w = tg.do("POST", "/api/rules/understand", admin, `{"text":"  "}`); w.Code != 400 {
		t.Fatalf("understand rỗng: %d", w.Code)
	}
}

func TestRulesAPI_ViolationsLast(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	st := storyEngineStore(tg)
	if err := st.World.SaveRuleViolations(1, nil); err != nil {
		t.Fatal(err)
	}
	for ch := 2; ch <= 5; ch++ {
		if err := st.World.SaveRuleViolations(ch, nil); err != nil {
			t.Fatal(err)
		}
	}
	w := tg.do("GET", rulesStoryBase+"/violations?last=3", admin, "")
	if w.Code != 200 {
		t.Fatalf("violations: %d %s", w.Code, w.Body)
	}
	var out struct {
		Chapters []struct {
			Chapter    int   `json:"chapter"`
			Violations []any `json:"violations"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Chapters) != 3 || out.Chapters[0].Chapter != 5 || out.Chapters[2].Chapter != 3 || out.Chapters[0].Violations == nil {
		t.Fatalf("chapters: %+v", out.Chapters)
	}
	if w = tg.do("GET", rulesStoryBase+"/violations?last=abc", admin, ""); w.Code != 400 {
		t.Fatalf("last sai: %d", w.Code)
	}
	if w = tg.do("GET", rulesStoryBase+"/violations", admin, ""); !strings.Contains(w.Body.String(), `"chapter":5`) {
		t.Fatalf("mặc định last=3: %s", w.Body)
	}
}

func TestRulesAPI_ApplyRecheckJobKeepsChaptersIntact(t *testing.T) {
	tg := newConfigTestGateway(t)
	stub := newNormalizerStub(t)
	tg.useStub(t, stub)
	admin := tg.login("admin", "admin-pass")
	st := storyEngineStore(tg)
	chapterText := "Anh ta nói: đột nhiên mọi thứ im lặng."
	if err := st.Drafts.SaveFinalChapter(1, chapterText); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, CompletedChapters: []int{1}}); err != nil {
		t.Fatal(err)
	}
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"story","project":"proj","story":"01-story","name":"quy-tac.md","text":"- cấm đột nhiên\n"}`)

	w := tg.do("POST", rulesStoryBase+"/apply", admin, `{"recheck":true}`)
	if w.Code != 200 {
		t.Fatalf("apply recheck: %d %s", w.Code, w.Body)
	}
	jobID, _ := decodeResp(t, w)["jobId"].(string)
	if !strings.Contains(jobID, "rules.recheck") {
		t.Fatalf("jobId: %q", jobID)
	}
	hub := tg.g.jobsHub()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if j, ok := hub.get(jobID); ok && j.Status == JobDoneS {
			if j.Kind != "rules.recheck" {
				t.Fatalf("kind: %s", j.Kind)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job rules.recheck không hoàn tất")
		}
		time.Sleep(20 * time.Millisecond)
	}
	got := st.World.RecentRuleViolations(3)
	if len(got) != 1 || len(got[0].Violations) == 0 {
		t.Fatalf("recheck phải ghi vi phạm 'đột nhiên': %+v", got)
	}
	if text, _ := st.Drafts.LoadChapterText(1); text != chapterText {
		t.Fatalf("recheck không được sửa chương: %q", text)
	}
}

func TestRulesAPI_UnderstandAndEffectiveCarryOptions(t *testing.T) {
	tg := newConfigTestGateway(t)
	stub := newNormalizerStub(t)
	tg.useStub(t, stub)
	admin := tg.login("admin", "admin-pass")

	w := tg.do("POST", "/api/rules/understand", admin, `{"text":"chương đừng quá dài"}`)
	u := decodeResp(t, w)
	opts, _ := u["options"].([]any)
	if w.Code != 200 || len(u["uncertain"].([]any)) != 1 || len(opts) != 2 || opts[0] != "Khoảng 3.000 chữ mỗi chương" {
		t.Fatalf("understand options: %d %v", w.Code, u)
	}
	u = decodeResp(t, tg.do("POST", "/api/rules/understand", admin, `{"text":"giọng văn lạnh"}`))
	if o, ok := u["options"].([]any); !ok || len(o) != 0 {
		t.Fatalf("rõ ràng → options []: %v", u["options"])
	}
	if u["uncertainNote"] != "" {
		t.Fatalf("hiểu chắc → uncertainNote rỗng: %v", u["uncertainNote"])
	}

	tg.do("PUT", "/api/rules/file", admin, `{"scope":"story","project":"proj","story":"01-story","name":"quy-tac.md","text":"- chương đừng quá dài\n- giọng văn lạnh\n"}`)
	var eff struct {
		Items []struct {
			Options       []string `json:"options"`
			Uncertain     bool     `json:"uncertain"`
			UncertainNote string   `json:"uncertainNote"`
		} `json:"items"`
	}
	w = tg.do("GET", rulesStoryBase+"/effective", admin, "")
	if err := json.Unmarshal(w.Body.Bytes(), &eff); err != nil || len(eff.Items) != 2 || len(eff.Items[0].Options) != 2 || eff.Items[1].Options == nil || len(eff.Items[1].Options) != 0 {
		t.Fatalf("effective options: %v %s", err, w.Body)
	}
	// Câu 26a: từng quy tắc có cờ chưa chắc + một câu ghi chú; quy tắc hiểu chắc
	// thì false + rỗng.
	if !eff.Items[0].Uncertain || eff.Items[0].UncertainNote != "Noverse chưa chắc “quá dài” là bao nhiêu" {
		t.Fatalf("quy tắc mơ hồ: %+v", eff.Items[0])
	}
	if eff.Items[1].Uncertain || eff.Items[1].UncertainNote != "" {
		t.Fatalf("quy tắc rõ: %+v", eff.Items[1])
	}
}

func TestRulesAPI_EffectiveWithoutStory(t *testing.T) {
	tg := newConfigTestGateway(t)
	stub := newNormalizerStub(t)
	tg.useStub(t, stub)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"global","name":"quy-tac.md","text":"- cấm chữ lạ\n"}`)
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"project","project":"proj","name":"quy-tac.md","text":"- chương đừng quá dài\n"}`)
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"story","project":"proj","story":"01-story","name":"quy-tac.md","text":"- cấm riêng truyện\n"}`)

	type resp struct {
		Snapshot map[string]any `json:"snapshot"`
		Items    []struct {
			Text    string           `json:"text"`
			Scope   string           `json:"scope"`
			Chips   []map[string]any `json:"chips"`
			Options []string         `json:"options"`
		} `json:"items"`
	}
	get := func(cookie, url string) (int, resp, map[string]any) {
		w := tg.do("GET", url, cookie, "")
		var r resp
		_ = json.Unmarshal(w.Body.Bytes(), &r)
		var raw map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &raw)
		return w.Code, r, raw
	}

	code, r, raw := get(admin, "/api/rules/effective?scope=global")
	if code != 200 || len(r.Items) != 1 || r.Items[0].Scope != "global" || r.Items[0].Text != "cấm chữ lạ" {
		t.Fatalf("global: %d %+v", code, r)
	}
	if _, has := raw["runtime"]; has {
		t.Fatalf("không có runtime: %v", raw)
	}
	if code, r, _ = get(admin, "/api/rules/effective"); code != 200 || len(r.Items) != 1 {
		t.Fatalf("scope mặc định = global: %d %+v", code, r)
	}
	code, r, _ = get(admin, "/api/rules/effective?scope=project&project=proj")
	if code != 200 || len(r.Items) != 2 || r.Items[0].Scope != "global" || r.Items[1].Scope != "project" || len(r.Items[1].Options) != 2 {
		t.Fatalf("project (global + project, không có cấp truyện): %d %+v", code, r)
	}
	if phrases := r.Snapshot["structured"].(map[string]any)["forbidden_phrases"].([]any); len(phrases) != 5 {
		t.Fatalf("snapshot = 4 mặc định + chữ lạ (không có cụm của truyện): %v", phrases)
	}
	if code, _, _ = get(admin, "/api/rules/effective?scope=story"); code != 400 {
		t.Fatalf("scope story không hỗ trợ: %d", code)
	}
	if code, _, _ = get(admin, "/api/rules/effective?scope=project"); code != 400 {
		t.Fatalf("thiếu project: %d", code)
	}
	if code, _, _ = get(admin, "/api/rules/effective?scope=project&project=nope"); code != 404 {
		t.Fatalf("project lạ: %d", code)
	}
	if code, _, _ = get(reader, "/api/rules/effective?scope=project&project=proj"); code != 200 {
		t.Fatalf("reader có quyền đọc proj/*: %d", code)
	}
	// Đệm toàn cục ghi ra ~/.ainovel/rules-cache.json; lần GET sau không gọi model.
	if _, err := os.Stat(filepath.Join(tg.home, ".ainovel", "rules-cache.json")); err != nil {
		t.Fatalf("thiếu tệp đệm: %v", err)
	}
	before := stub.calls.Load()
	get(admin, "/api/rules/effective?scope=project&project=proj")
	if stub.calls.Load() != before {
		t.Fatalf("GET lặp lại phải dùng đệm: %d → %d", before, stub.calls.Load())
	}
	// Không model: 200 + degraded, không 409.
	tg.globalConfig(t, `{}`)
	tg.do("PUT", "/api/rules/file", admin, `{"scope":"global","name":"quy-tac.md","text":"- dòng mới chưa có đệm\n"}`)
	w := tg.do("GET", "/api/rules/effective?scope=global", admin, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"degraded"`) {
		t.Fatalf("không model: %d %s", w.Code, w.Body)
	}
}
