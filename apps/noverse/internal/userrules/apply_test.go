package userrules

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// lineModel là ChatModel giả: trả cấu trúc theo nguyên văn dòng quy tắc (user message),
// đếm số lần gọi. Dòng không có trong bảng → coi là lời dặn thuần (preferences).
type lineModel struct {
	mu      sync.Mutex
	calls   int
	phrases map[string]string   // nguyên văn → cụm cấm sinh ra
	options map[string][]string // nguyên văn mơ hồ → các cách hiểu (kèm một dòng uncertain)
	err     error
}

func (m *lineModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	user := ""
	for _, msg := range messages {
		if msg.Role == llmtypes.RoleUser {
			user = strings.TrimSpace(msg.TextContent())
		}
	}
	reply := `{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":[],"fatigue_words":[]},"preferences":"` + user + `","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`
	if opts, ok := m.options[user]; ok {
		o, _ := json.Marshal(opts)
		reply = `{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":[],"fatigue_words":[]},"preferences":"` + user + `","uncertain":["Chưa rõ mức cụ thể"],"is_uncertain":true,"uncertain_note":"Noverse chưa chắc mức cụ thể","options":` + string(o) + `}`
	}
	if p, ok := m.phrases[user]; ok {
		reply = `{"structured":{"genre":"","forbidden_chars":[],"forbidden_phrases":["` + p + `"],"fatigue_words":[]},"preferences":"","uncertain":[],"is_uncertain":false,"uncertain_note":null,"options":[]}`
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:    llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.TextBlock(reply)},
	}}, nil
}

func (m *lineModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, nil
}
func (m *lineModel) SupportsTools() bool { return false }

func (m *lineModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type fixture struct {
	svc      *Service
	st       *store.Store
	model    *lineModel
	storyDir string // thư mục luật cấp truyện
}

func newFixture(t *testing.T, model *lineModel) *fixture {
	t.Helper()
	st := store.NewStore(t.TempDir())
	storyRules := filepath.Join(t.TempDir(), "rules")
	if err := os.MkdirAll(storyRules, 0o755); err != nil {
		t.Fatal(err)
	}
	return &fixture{svc: NewService(st, model, rules.LoadOptions{StoryRulesDir: storyRules}), st: st, model: model, storyDir: storyRules}
}

func (f *fixture) writeRules(t *testing.T, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.storyDir, rules.ManagedRulesFile), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) setProgress(t *testing.T, inProgress int, completed ...int) {
	t.Helper()
	if err := f.st.Progress.Save(&domain.Progress{Phase: domain.PhaseWriting, InProgressChapter: inProgress, CompletedChapters: completed}); err != nil {
		t.Fatal(err)
	}
}

func hasPhrase(s *rules.Snapshot, phrase string) bool {
	for _, p := range s.Structured.ForbiddenPhrases {
		if p == phrase {
			return true
		}
	}
	return false
}

func TestRuntime_MigratesFromLegacySnapshot(t *testing.T) {
	f := newFixture(t, nil)
	legacy := rules.BuildSnapshot([]rules.Candidate{
		rules.SystemDefaults(),
		{Source: "startup_prompt", Preferences: "giọng lạnh"},
		{Source: store.RuntimeRuleSource, Preferences: "bớt dùng ẩn dụ"},
		{Source: store.RuntimeRuleSource, Preferences: "đừng để nhân vật khóc"},
	})
	if err := f.st.UserRules.Save(&legacy); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.UserRules.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("mỗi Sources runtime_update thành một mục, got %d: %+v", len(list), list)
	}
	if list[0].Text != "bớt dùng ẩn dụ" || list[1].Text != "đừng để nhân vật khóc" {
		t.Fatalf("nguyên văn phải lấy từ Preferences theo thứ tự: %+v", list)
	}
	if list[0].ID == "" || list[0].ID == list[1].ID || !list[0].Legacy {
		t.Fatalf("id phải duy nhất và đánh dấu legacy: %+v", list)
	}
	// Ghi thêm một mục: các mục suy ra từ snapshot cũ được lưu cùng.
	if err := f.st.UserRules.AddRuntime(store.RuntimeRule{ID: "rt-new", Text: "mới"}); err != nil {
		t.Fatal(err)
	}
	list, _ = f.st.UserRules.Runtime()
	if len(list) != 3 || list[2].ID != "rt-new" {
		t.Fatalf("sau khi thêm phải còn 3 mục: %+v", list)
	}
}

func TestAddRuntimeRule_AppendsToRuntimeList(t *testing.T) {
	f := newFixture(t, &lineModel{phrases: map[string]string{"cấm chữ lạ": "chữ lạ"}})
	f.setProgress(t, 4, 1, 2, 3)
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), "cấm chữ lạ"); err != nil {
		t.Fatal(err)
	}
	list, _ := f.st.UserRules.Runtime()
	if len(list) != 1 || list[0].Text != "cấm chữ lạ" || list[0].Chapter != 4 || list[0].ID == "" || list[0].CreatedAt.IsZero() {
		t.Fatalf("Trọng tài phải ghi vào danh sách runtime: %+v", list)
	}
	if list[0].Candidate.Degraded || len(list[0].Candidate.Structured.ForbiddenPhrases) != 1 {
		t.Fatalf("candidate phải lưu kết quả chuẩn hoá: %+v", list[0].Candidate)
	}
}

func TestApply_KeepsRuntimeAndPinsInProgressChapter(t *testing.T) {
	model := &lineModel{phrases: map[string]string{
		"cấm chữ lạ":    "chữ lạ",
		"cấm đột nhiên": "đột nhiên",
	}}
	f := newFixture(t, model)
	f.setProgress(t, 0, 1, 2, 3)
	if _, err := f.svc.Build(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), "cấm chữ lạ"); err != nil {
		t.Fatal(err)
	}
	f.writeRules(t, "- cấm đột nhiên\n")

	// Chương 4 đang viết dở khi bấm Áp dụng.
	f.setProgress(t, 4, 1, 2, 3)
	res, err := f.svc.Apply(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.EffectiveFrom != 5 || res.PinnedChapter != 4 {
		t.Fatalf("hiệu lực từ chương sau chương đang viết: %+v", res)
	}
	if !hasPhrase(res.Snapshot, "đột nhiên") || !hasPhrase(res.Snapshot, "chữ lạ") || !hasPhrase(res.Snapshot, "theo một nghĩa nào đó") {
		t.Fatalf("snapshot mới phải gồm file + runtime + mặc định: %v", res.Snapshot.Structured.ForbiddenPhrases)
	}
	// Runtime vẫn còn.
	if list, _ := f.st.UserRules.Runtime(); len(list) != 1 {
		t.Fatalf("Apply phải giữ runtime: %+v", list)
	}
	// Chương 4 đang viết vẫn thấy snapshot cũ (không có "đột nhiên").
	pinned, err := f.st.UserRules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if hasPhrase(pinned, "đột nhiên") {
		t.Fatal("chương đang viết phải giữ snapshot cũ")
	}
	if !hasPhrase(pinned, "chữ lạ") {
		t.Fatal("snapshot cũ có luật runtime đã học")
	}
	// Một luật Trọng tài học tiếp giữa chừng vẫn có hiệu lực ngay cho chương đang viết.
	model.phrases["cấm lặp"] = "lặp lặp"
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), "cấm lặp"); err != nil {
		t.Fatal(err)
	}
	if pinned, _ = f.st.UserRules.Load(); !hasPhrase(pinned, "lặp lặp") || hasPhrase(pinned, "đột nhiên") {
		t.Fatalf("pin phải nhận luật Trọng tài nhưng không nhận file mới: %v", pinned.Structured.ForbiddenPhrases)
	}
	// Chương 4 xong, chương 5 bắt đầu: thấy snapshot mới.
	f.setProgress(t, 5, 1, 2, 3, 4)
	next, _ := f.st.UserRules.Load()
	if !hasPhrase(next, "đột nhiên") {
		t.Fatal("chương kế tiếp phải dùng snapshot mới")
	}
}

func TestApply_UsesNormCache_NoSecondModelCall(t *testing.T) {
	model := &lineModel{phrases: map[string]string{"cấm đột nhiên": "đột nhiên"}}
	f := newFixture(t, model)
	f.writeRules(t, "- cấm đột nhiên\n- giọng văn lạnh\n")
	if _, err := f.svc.Apply(t.Context(), "yêu cầu gốc"); err != nil {
		t.Fatal(err)
	}
	first := model.callCount()
	if first != 3 { // 2 dòng + startup_prompt
		t.Fatalf("lần đầu phải chuẩn hoá 3 nguồn, got %d", first)
	}
	if _, err := f.svc.Apply(t.Context(), "yêu cầu gốc"); err != nil {
		t.Fatal(err)
	}
	if model.callCount() != first {
		t.Fatalf("áp dụng lại nội dung không đổi không được gọi model nữa: %d → %d", first, model.callCount())
	}
	if _, err := f.svc.Effective(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.callCount() != first {
		t.Fatalf("Effective cũng dùng cache: %d", model.callCount())
	}
	// Sửa một dòng chỉ chuẩn hoá lại dòng đó.
	f.writeRules(t, "- cấm đột nhiên\n- giọng văn ấm\n")
	if _, err := f.svc.Apply(t.Context(), "yêu cầu gốc"); err != nil {
		t.Fatal(err)
	}
	if model.callCount() != first+1 {
		t.Fatalf("chỉ dòng đổi mới gọi model: %d", model.callCount())
	}
}

func TestApply_DegradedResultsAreNotCached(t *testing.T) {
	model := &lineModel{err: errors.New("boom")}
	f := newFixture(t, model)
	f.writeRules(t, "- giọng văn lạnh\n")
	res, err := f.svc.Apply(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Snapshot.Status != rules.StatusDegraded {
		t.Fatalf("lỗi model → snapshot degraded, got %s", res.Snapshot.Status)
	}
	model.err = nil
	res, err = f.svc.Apply(t.Context(), "")
	if err != nil || res.Snapshot.Status != rules.StatusReady {
		t.Fatalf("lần sau phải chuẩn hoá lại thay vì dùng kết quả hạ cấp: %v %+v", err, res)
	}
}

func TestApply_CancelledWritesNothing(t *testing.T) {
	f := newFixture(t, &lineModel{})
	f.writeRules(t, "- giọng văn lạnh\n")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := f.svc.Apply(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if snap, _ := f.st.UserRules.LoadCurrent(); snap != nil {
		t.Fatal("Apply bị huỷ không được ghi snapshot")
	}
}

func TestEffective_ItemsHaveChipsPerLine(t *testing.T) {
	f := newFixture(t, &lineModel{phrases: map[string]string{"cấm đột nhiên": "đột nhiên"}})
	f.writeRules(t, "# tiêu đề\n- cấm đột nhiên\n- giọng văn lạnh\n")
	eff, err := f.svc.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Items) != 2 || eff.Items[0].Scope != "story" || eff.Items[0].Text != "cấm đột nhiên" {
		t.Fatalf("items: %+v", eff.Items)
	}
	if c := eff.Items[0].Chips; len(c) != 1 || c[0].Kind != rules.ChipBan || c[0].Text != "đột nhiên" {
		t.Fatalf("chip cấm: %+v", c)
	}
	if c := eff.Items[1].Chips; len(c) != 1 || c[0].Kind != rules.ChipPref {
		t.Fatalf("chip lời dặn: %+v", c)
	}
	if eff.Snapshot == nil || eff.Runtime == nil || eff.Items[0].Uncertain {
		t.Fatalf("snapshot/runtime/uncertain: %+v", eff)
	}
	if eff.Items[0].UncertainNote != "" {
		t.Fatalf("quy tắc hiểu chắc không được có uncertainNote: %+v", eff.Items[0])
	}
}

func TestKeepRuntime_AppendsLineAndRemovesItem(t *testing.T) {
	f := newFixture(t, &lineModel{})
	f.writeRules(t, "- cấm đột nhiên")
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), "bớt dùng ẩn dụ"); err != nil {
		t.Fatal(err)
	}
	list, _ := f.st.UserRules.Runtime()
	id := list[0].ID
	if _, err := f.svc.KeepRuntime(id); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(f.storyDir, rules.ManagedRulesFile))
	if string(data) != "- cấm đột nhiên\n- bớt dùng ẩn dụ\n" {
		t.Fatalf("phải nối dòng '- …' vào quy-tac.md: %q", data)
	}
	if list, _ = f.st.UserRules.Runtime(); len(list) != 0 {
		t.Fatalf("mục runtime phải bị bỏ: %+v", list)
	}
	if _, err := f.svc.KeepRuntime(id); !errors.Is(err, ErrRuntimeRuleNotFound) {
		t.Fatalf("keep lần hai: %v", err)
	}
}

func TestDropRuntime(t *testing.T) {
	f := newFixture(t, &lineModel{})
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), "bớt dùng ẩn dụ"); err != nil {
		t.Fatal(err)
	}
	list, _ := f.st.UserRules.Runtime()
	if err := f.svc.DropRuntime(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = f.st.UserRules.Runtime(); len(list) != 0 {
		t.Fatalf("còn: %+v", list)
	}
	if err := f.svc.DropRuntime("rt-none"); !errors.Is(err, ErrRuntimeRuleNotFound) {
		t.Fatalf("xoá id lạ: %v", err)
	}
	// Xoá hết rồi Apply: luật không quay lại từ snapshot cũ.
	res, err := f.svc.Apply(t.Context(), "")
	if err != nil || strings.Contains(res.Snapshot.Preferences, "ẩn dụ") {
		t.Fatalf("luật đã xoá không được còn trong snapshot sau Apply: %v %q", err, res.Snapshot.Preferences)
	}
}

func TestUnderstand_ModelErrorDegradesInsteadOfFailing(t *testing.T) {
	n := NewNormalizer(&lineModel{err: errors.New("provider down")})
	u, err := n.Understand(t.Context(), "đừng dùng từ 'đột nhiên'")
	if err != nil {
		t.Fatalf("lỗi chuẩn hoá phải hạ cấp, không phải lỗi: %v", err)
	}
	if u.Status != rules.StatusDegraded || u.Preferences != "đừng dùng từ 'đột nhiên'" {
		t.Fatalf("hạ cấp phải giữ nguyên văn làm preferences: %+v", u)
	}
	if len(u.Chips) != 1 || u.Chips[0].Kind != rules.ChipPref || u.Uncertain == nil {
		t.Fatalf("chips/uncertain: %+v", u)
	}
}

func TestUnderstand_CancelledContextIsError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewNormalizer(&lineModel{err: errors.New("x")}).Understand(ctx, "luật"); !errors.Is(err, context.Canceled) {
		t.Fatalf("khách ngắt kết nối phải trả lỗi ctx: %v", err)
	}
}

func TestUnderstand_ReturnsStructuredWithoutSaving(t *testing.T) {
	model := &lineModel{phrases: map[string]string{"cấm đột nhiên": "đột nhiên"}}
	f := newFixture(t, model)
	u, err := NewNormalizer(model).Understand(t.Context(), "cấm đột nhiên")
	if err != nil || u.Status != rules.StatusReady || len(u.Structured.ForbiddenPhrases) != 1 {
		t.Fatalf("%+v %v", u, err)
	}
	if snap, _ := f.st.UserRules.LoadCurrent(); snap != nil {
		t.Fatal("understand không được lưu gì")
	}
}

func TestRecheck_RecordsViolationsAndNeverEditsChapters(t *testing.T) {
	f := newFixture(t, &lineModel{})
	chapters := map[int]string{
		1: "Chương một sạch sẽ. Không có gì lạ.",
		2: "Anh ta nói: theo một nghĩa nào đó, đáng chú ý là mọi thứ đã đổi.",
	}
	for ch, text := range chapters {
		if err := f.st.Drafts.SaveFinalChapter(ch, text); err != nil {
			t.Fatal(err)
		}
	}
	f.setProgress(t, 0, 1, 2)
	before := map[int][]byte{}
	for ch := range chapters {
		before[ch], _ = os.ReadFile(filepath.Join(f.st.Dir(), "chapters", map[int]string{1: "01.md", 2: "02.md"}[ch]))
	}
	snap := rules.BuildSnapshot([]rules.Candidate{rules.SystemDefaults(), {Source: "x", Structured: rules.Structured{ForbiddenPhrases: []string{"mọi thứ đã đổi"}}}})
	var steps []int
	res, err := f.svc.Recheck(t.Context(), &snap, func(done, total, ch int) { steps = append(steps, done*10+total) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Checked != 2 || res.WithViolations != 1 || res.Violations < 2 {
		t.Fatalf("kết quả: %+v", res)
	}
	if len(steps) != 2 || steps[1] != 22 {
		t.Fatalf("progress: %v", steps)
	}
	recent := f.st.World.RecentRuleViolations(3)
	if len(recent) != 2 || recent[0].Chapter != 2 || len(recent[0].Violations) < 2 || len(recent[1].Violations) != 0 {
		t.Fatalf("vi phạm ghi theo chương: %+v", recent)
	}
	for ch, want := range before {
		got, _ := os.ReadFile(filepath.Join(f.st.Dir(), "chapters", map[int]string{1: "01.md", 2: "02.md"}[ch]))
		if string(got) != string(want) {
			t.Fatalf("Recheck không được sửa chương %d", ch)
		}
	}
}

func TestRecheck_CancelStopsWithoutFurtherWrites(t *testing.T) {
	f := newFixture(t, &lineModel{})
	for ch := 1; ch <= 3; ch++ {
		_ = f.st.Drafts.SaveFinalChapter(ch, "nội dung")
	}
	f.setProgress(t, 0, 1, 2, 3)
	ctx, cancel := context.WithCancel(t.Context())
	res, err := f.svc.Recheck(ctx, nil, func(done, _, _ int) {
		if done == 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || res.Checked != 1 {
		t.Fatalf("huỷ sau chương 1: %+v %v", res, err)
	}
	_ = time.Second
}

const ambiguousRule = "chương đừng quá dài"

var ambiguousOptions = []string{"Khoảng 3.000 chữ mỗi chương", "Ngắn hơn chương trước một chút"}

func TestUnderstand_ReturnsOptionsForUncertainRule(t *testing.T) {
	model := &lineModel{options: map[string][]string{ambiguousRule: ambiguousOptions}}
	u, err := NewNormalizer(model).Understand(t.Context(), ambiguousRule)
	if err != nil {
		t.Fatal(err)
	}
	if len(u.Uncertain) != 1 || strings.Join(u.Options, "|") != strings.Join(ambiguousOptions, "|") {
		t.Fatalf("options phải đi cùng uncertain: %+v", u)
	}
	// Rõ ràng → options là mảng rỗng (không nil).
	u, _ = NewNormalizer(&lineModel{}).Understand(t.Context(), "giọng văn lạnh")
	if u.Options == nil || len(u.Options) != 0 {
		t.Fatalf("options phải là []: %#v", u.Options)
	}
	// Hạ cấp → không có cách hiểu.
	u, _ = NewNormalizer(&lineModel{err: errors.New("down")}).Understand(t.Context(), ambiguousRule)
	if u.Status != rules.StatusDegraded || u.Options == nil || len(u.Options) != 0 {
		t.Fatalf("degraded → options []: %+v", u)
	}
}

// TestUncertainNoteFlowsToRuleItem: ghi chú "chưa chắc" của mô hình thành
// uncertain/uncertainNote của từng quy tắc (Câu 26a); quy tắc hiểu chắc thì
// false + chuỗi rỗng.
func TestUncertainNoteFlowsToRuleItem(t *testing.T) {
	model := &lineModel{options: map[string][]string{ambiguousRule: ambiguousOptions}}
	u, err := NewNormalizer(model).Understand(t.Context(), ambiguousRule)
	if err != nil {
		t.Fatal(err)
	}
	if u.UncertainNote != "Noverse chưa chắc mức cụ thể" {
		t.Fatalf("understand uncertainNote = %q", u.UncertainNote)
	}

	f := newFixture(t, model)
	f.writeRules(t, "- "+ambiguousRule+"\n- giọng văn lạnh\n")
	eff, err := f.svc.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Items) != 2 {
		t.Fatalf("items = %+v", eff.Items)
	}
	if !eff.Items[0].Uncertain || eff.Items[0].UncertainNote != "Noverse chưa chắc mức cụ thể" {
		t.Fatalf("quy tắc mơ hồ = %+v", eff.Items[0])
	}
	if eff.Items[1].Uncertain || eff.Items[1].UncertainNote != "" {
		t.Fatalf("quy tắc rõ = %+v", eff.Items[1])
	}
	if len(eff.Runtime) != 0 {
		t.Fatalf("runtime = %+v", eff.Runtime)
	}
}

func TestCleanOptions(t *testing.T) {
	uncertain := []string{"mơ hồ"}
	got := cleanOptions([]string{" a ", "", "a", "b  c", "b c", "d", "e"}, uncertain)
	if strings.Join(got, "|") != "a|b  c|d" {
		t.Fatalf("bỏ rỗng/trùng, tối đa 3: %q", got)
	}
	if cleanOptions([]string{"a"}, nil) != nil {
		t.Fatal("không uncertain thì không options")
	}
}

func TestEffective_OptionsOnItemsAndRuntime_CachedAndDegradedEmpty(t *testing.T) {
	model := &lineModel{options: map[string][]string{ambiguousRule: ambiguousOptions}}
	f := newFixture(t, model)
	f.writeRules(t, "- "+ambiguousRule+"\n- giọng văn lạnh\n")
	if _, _, err := f.svc.AddRuntimeRule(t.Context(), ambiguousRule); err != nil {
		t.Fatal(err)
	}
	calls := model.callCount()
	eff, err := f.svc.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(eff.Items[0].Options, "|") != strings.Join(ambiguousOptions, "|") || eff.Items[1].Options == nil || len(eff.Items[1].Options) != 0 {
		t.Fatalf("items options: %+v", eff.Items)
	}
	if len(eff.Runtime) != 1 || len(eff.Runtime[0].Options) != 2 {
		t.Fatalf("runtime options: %+v", eff.Runtime)
	}
	// Effective lần hai lấy options từ đệm, không gọi model.
	before := model.callCount()
	if _, err := f.svc.Effective(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.callCount() != before {
		t.Fatalf("options phải nằm trong đệm: %d → %d (trước đó %d)", before, model.callCount(), calls)
	}
	// Model lỗi + chưa có đệm → degraded, options [].
	bad := newFixture(t, &lineModel{err: errors.New("down")})
	bad.writeRules(t, "- "+ambiguousRule+"\n")
	eff, _ = bad.svc.Effective(t.Context())
	if eff.Items[0].Status != rules.StatusDegraded || eff.Items[0].Options == nil || len(eff.Items[0].Options) != 0 {
		t.Fatalf("degraded: %+v", eff.Items[0])
	}
}

func TestFileRules_EffectiveByScopeWithDiskCache(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	write := func(dir, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rules.ManagedRulesFile), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(home, "- cấm đột nhiên\n")
	write(project, "- "+ambiguousRule+"\n")
	model := &lineModel{phrases: map[string]string{"cấm đột nhiên": "đột nhiên"}, options: map[string][]string{ambiguousRule: ambiguousOptions}}
	cache := filepath.Join(t.TempDir(), "rules-cache.json")

	// Đường dẫn global cũ không còn là một cấp quy tắc.
	eff, err := NewFileRules(model, rules.LoadOptions{HomeRulesDir: home}, cache).Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Items) != 0 {
		t.Fatalf("global rules must be ignored: %+v", eff.Items)
	}
	if hasPhrase(eff.Snapshot, "đột nhiên") || !hasPhrase(eff.Snapshot, "theo một nghĩa nào đó") {
		t.Fatalf("snapshot must contain defaults, not global rules: %v", eff.Snapshot.Structured.ForbiddenPhrases)
	}
	// Cấp dự án chỉ đọc project; vẫn giữ options của dòng mơ hồ.
	opts := rules.LoadOptions{HomeRulesDir: home, ProjectRulesDir: project}
	eff, err = NewFileRules(model, opts, cache).Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Items) != 1 || eff.Items[0].Scope != "project" || len(eff.Items[0].Options) != 2 {
		t.Fatalf("project items: %+v", eff.Items)
	}
	// Đệm ghi ra đĩa: bộ đọc mới (như lần GET sau) không gọi model nữa.
	before := model.callCount()
	if _, err := NewFileRules(model, opts, cache).Effective(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.callCount() != before {
		t.Fatalf("đệm trên đĩa phải được dùng lại: %d → %d", before, model.callCount())
	}
	// Không model, không đệm → hạ cấp, không lỗi.
	eff, err = NewFileRules(nil, opts, "").Effective(t.Context())
	if err != nil || eff.Items[0].Status != rules.StatusDegraded {
		t.Fatalf("không model: %v %+v", err, eff)
	}
}
