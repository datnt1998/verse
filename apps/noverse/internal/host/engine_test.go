package host

// Engine 端到端集成测试(engine-rfc.md §7 原型验收):
// 真实 store + 真实 Worker 工具 + 脚本化 ChatModel,验证
//  1. Route 驱动的完整写书链路:写第1章 → 写第2章 → 完本 → 引擎自然停机
//  2. Worker 失败路径:重试一次 → Arbiter worker_failure 裁定 abort → 暂停 + 审计落盘
//  3. 僵局路径:同指令无进展 ×3 → Arbiter deadlock 裁定 → 审计落盘 → abort 停机

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/agent/subagent"
	"github.com/datnt1998/verse/apps/noverse/internal/arbiter"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/flow"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

// scriptedChatModel 按回调产出响应的最小 ChatModel。
type scriptedChatModel struct {
	fn func(msgs []llmtypes.Message) llmtypes.Message
}

func TestFailureFactsKeepPartialStateAndWarnings(t *testing.T) {
	dir := t.TempDir()
	st := storepkg.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "premise.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &engine{store: st}
	workerErr := fmt.Errorf("writer exhausted: %w", agentcore.ErrMaxTurns)
	facts := e.failureFacts("worker_failure", &flow.Instruction{Agent: "writer", Task: "viết tiếp"}, workerErr)
	if facts.ErrorKind != "max_turns" || facts.Phase != string(domain.PhaseInit) {
		t.Fatalf("Phải giữ loại lỗi và các dữ kiện tiến độ đọc được: %+v", facts)
	}
	if len(facts.FactWarnings) == 0 {
		t.Fatalf("Dữ kiện nền không đọc được phải được chuyển cho Arbiter dưới dạng cảnh báo: %+v", facts)
	}
}

func TestIsNonSemanticWorkerFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context overflow", err: llmtypes.ErrContextOverflow, want: true},
		{name: "partial stream", err: agentcore.ErrStreamPartial, want: true},
		{name: "stream idle", err: llmtypes.ErrProviderStreamIdle, want: true},
		{name: "quota", err: llmtypes.ErrProviderQuota, want: true},
		{name: "rate limit", err: llmtypes.ErrProviderRateLimit, want: true},
		{name: "timeout", err: llmtypes.ErrProviderTimeout, want: true},
		{name: "auth", err: llmtypes.ErrProviderAuth, want: true},
		{name: "network wrapped", err: fmt.Errorf("provider: %w", llmtypes.ErrProviderNetwork), want: true},
		{name: "raw EOF", err: fmt.Errorf("upstream closed: EOF"), want: true},
		{name: "overloaded", err: llmtypes.ErrProviderOverloaded, want: true},
		{name: "flattened overloaded", err: fmt.Errorf("bad_response_status_code: Too many concurrent requests [provider, HTTP 500, openai]"), want: true},
		{name: "content filter", err: llmtypes.ErrProviderContentFilter, want: false},
		{name: "max turns", err: agentcore.ErrMaxTurns, want: false},
		{name: "stop guard", err: agentcore.ErrStopGuard, want: false},
		{name: "canceled", err: context.Canceled, want: false},
		{name: "tool validation", err: agentcore.ErrToolValidation, want: false},
		{name: "unknown", err: fmt.Errorf("unknown failure"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNonSemanticWorkerFailure(tt.err); got != tt.want {
				t.Fatalf("isNonSemanticWorkerFailure(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestInterventionDispatchTaskPreservesOriginalAuthority(t *testing.T) {
	const task = "Kiểm tra nội dung trùng lặp và sắp xếp viết lại cần thiết"
	const original = "  Về sau đừng giải thích lặp lại nguồn gốc năng lực; đừng sửa nội dung không liên quan.\n"

	got := interventionDispatchTask(task, original, nil)
	if !strings.Contains(got, task) {
		t.Fatalf("Mất nhiệm vụ phân phối: %q", got)
	}
	if !strings.Contains(got, original) {
		t.Fatalf("Can thiệp gốc của người dùng không được giữ nguyên văn: %q", got)
	}
	if !strings.Contains(got, "nguồn ủy quyền duy nhất cho lần sửa đổi này") {
		t.Fatalf("Thiếu chú thích ranh giới ủy quyền: %q", got)
	}
}

// 审计里的派单尚未执行或被 stale 时，也不能丢掉早先的授权边界。
func TestInterventionDispatchTaskPreservesScopeAcrossUnappliedDecision(t *testing.T) {
	scope := strings.Repeat("Mô tả nhãn kịch bản. ", 20) + "Chỉ sửa chương 7–8, không sửa chương khác."
	facts := arbiter.InterventionFacts{
		RecentDecisions: []arbiter.RecentDecision{
			{Input: scope, Answer: "Chỉ bỏ nhãn trong chương 7–8."},
			{Input: "Xử lí đi e"},
			{Input: "Xử lí đi e", PlannedAction: "giao editor: bỏ nhãn chương 7–8"},
		},
		NoActionStreak: 0,
	}
	got := interventionDispatchTask("Bỏ nhãn chương 7–8", "Xử lí đi e", facts.RecentDecisions)
	if !strings.Contains(got, scope) {
		t.Fatalf("mất giới hạn ủy quyền khi phán định trước chưa được áp dụng: %q", got)
	}
}

func (m *scriptedChatModel) Generate(_ context.Context, msgs []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return &llmtypes.LLMResponse{Message: m.fn(msgs)}, nil
}

func (m *scriptedChatModel) GenerateStream(ctx context.Context, msgs []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	resp, _ := m.Generate(ctx, msgs, tools, opts...)
	ch := make(chan llmtypes.StreamEvent, 1)
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventDone, Message: resp.Message, StopReason: resp.Message.StopReason}
	close(ch)
	return ch, nil
}

func (m *scriptedChatModel) SupportsTools() bool { return true }

// editThenCancelModel 复现 #84：每次 Worker 都成功产生一个内容不同的
// edit checkpoint，随后在同一 run 内返回 context canceled，始终没有 commit。
type editThenCancelModel struct {
	edits atomic.Int32
}

func (m *editThenCancelModel) Generate(_ context.Context, msgs []llmtypes.Message, _ []llmtypes.ToolSpec, _ ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == llmtypes.RoleTool {
		return nil, context.Canceled
	}
	n := int(m.edits.Add(1))
	return &llmtypes.LLMResponse{Message: testToolCallMsg("edit_chapter", map[string]any{
		"chapter":    1,
		"old_string": fmt.Sprintf("Phiên bản %d", n-1),
		"new_string": fmt.Sprintf("Phiên bản %d", n),
	})}, nil
}

func (m *editThenCancelModel) GenerateStream(ctx context.Context, msgs []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	resp, err := m.Generate(ctx, msgs, tools, opts...)
	if err != nil {
		return nil, err
	}
	ch := make(chan llmtypes.StreamEvent, 1)
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventDone, Message: resp.Message, StopReason: resp.Message.StopReason}
	close(ch)
	return ch, nil
}

func (m *editThenCancelModel) SupportsTools() bool { return true }

// providerNetworkModel 模拟 Worker 在任何模型输出前即遭遇瞬态网络故障。
// MaxRetries=0 时每次 subagent.Run 对应一次调用，便于验证 Engine 重试计数。
type providerNetworkModel struct {
	calls atomic.Int32
}

func (m *providerNetworkModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.calls.Add(1)
	return nil, fmt.Errorf("test provider EOF: %w", llmtypes.ErrProviderNetwork)
}

func (m *providerNetworkModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	m.calls.Add(1)
	return nil, fmt.Errorf("test provider EOF: %w", llmtypes.ErrProviderNetwork)
}

func (m *providerNetworkModel) SupportsTools() bool { return true }

func testToolCallMsg(name string, args any) llmtypes.Message {
	data, _ := json.Marshal(args)
	return llmtypes.Message{
		Role: llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.ToolCallBlock(llmtypes.ToolCall{
			ID: "tc-" + name, Name: name, Args: data,
		})},
		StopReason: llmtypes.StopReasonToolUse,
	}
}

func testTextMsg(text string) llmtypes.Message {
	return llmtypes.Message{
		Role:       llmtypes.RoleAssistant,
		Content:    []llmtypes.ContentBlock{llmtypes.TextBlock(text)},
		StopReason: llmtypes.StopReasonStop,
	}
}

// reviewArgs dựng tham số save_review từ giá trị domain.ReviewEntry có kiểu,
// theo đúng hình dạng schema strict (trường tùy chọn bỏ trống thành null) để
// fixture không chép tay hình dạng JSON của bản ghi.
func reviewArgs(t *testing.T, entry domain.ReviewEntry) json.RawMessage {
	t.Helper()
	args, err := llmcontract.EncodeArgs(tools.NewSaveReviewTool(nil, nil).Schema(), entry)
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	return args
}

// chapterRe 从派单任务里取章号：Z4Flow 已把 Route 任务译成越南语
// （Viết chương N / Viết lại chương N / Gọt giũa chương N），旧中文形态仍兼容。
var chapterRe = regexp.MustCompile(`(?:(?:Viết(?: lại)?|Gọt giũa) chương|写第)\s*(\d+)`)

// scriptedWriterModel 按对话内已有的 tool 结果数决定下一步,
// 走完整 plan → draft → check → commit 序列(真实工具,真实落盘)。
func scriptedWriterModel() *scriptedChatModel {
	return &scriptedChatModel{fn: func(msgs []llmtypes.Message) llmtypes.Message {
		chapter := 0
		toolResults := 0
		for _, m := range msgs {
			if m.Role == llmtypes.RoleUser {
				if match := chapterRe.FindStringSubmatch(m.TextContent()); match != nil {
					chapter, _ = strconv.Atoi(match[1])
				}
			}
			if m.Role == llmtypes.RoleTool {
				toolResults++
			}
		}
		switch toolResults {
		case 0:
			return testToolCallMsg("plan_chapter", map[string]any{
				"chapter": chapter, "title": fmt.Sprintf("Chương %d", chapter),
				"goal": "Đẩy tiến tuyến chính", "conflict": "Nhân vật chính gặp trở ngại", "hook": "Kết thúc bằng hồi hộp",
			})
		case 1:
			return testToolCallMsg("draft_chapter", map[string]any{
				"chapter": chapter, "mode": "write",
				"content": strings.Repeat(fmt.Sprintf("Đoạn văn chính của chương %d, nhân vật chính dò dẫm tiến lên trong bóng tối.", chapter), 20),
			})
		case 2:
			return testToolCallMsg("check_consistency", map[string]any{"chapter": chapter})
		default:
			return testToolCallMsg("commit_chapter", map[string]any{
				"chapter": chapter, "title": fmt.Sprintf("Chương %d", chapter), "summary": fmt.Sprintf("Tóm tắt chương %d", chapter),
				"characters": []string{"Nhân vật chính"}, "key_events": []string{"Đẩy tiến"},
				"timeline_events": []any{}, "foreshadow_updates": []any{},
				"relationship_changes": []any{}, "state_changes": []any{}, "cast_intros": []any{},
				"hook_type": "crisis", "dominant_strand": "quest", "feedback": nil,
				"address_terms":  nil,
				"fixed_comments": nil,
				"places":         nil, "objects": nil,
			})
		}
	}}
}

// newTestEngine 组装带真实 store/observer 的引擎;返回引擎、事件采集与完成信号。
func newTestEngine(t *testing.T, st *storepkg.Store, workers *subagent.Runner, arbiterModel llmtypes.ChatModel) (*engine, *[]Event, chan struct{}) {
	t.Helper()
	if err := st.RunMeta.Init("default", "test", "test"); err != nil {
		t.Fatalf("init run meta: %v", err)
	}
	var mu sync.Mutex
	events := &[]Event{}
	done := make(chan struct{}, 1)
	obs := newObserver(st, func(ev Event) {
		mu.Lock()
		*events = append(*events, ev)
		mu.Unlock()
	}, func(string) {}, func() {}, nil)
	e := &engine{
		store:           st,
		workers:         workers,
		arbiterModel:    arbiterModel,
		failurePrompt:   "sys",
		planStartPrompt: "sys",
		style:           "default",
		observer:        obs,
		refresh:         func() {},
		emitEvent: func(ev Event) {
			mu.Lock()
			*events = append(*events, ev)
			mu.Unlock()
		},
		notify: func(string, string, string, string) {},
		onDone: func() {
			select {
			case done <- struct{}{}:
			default:
			}
		},
	}
	e.gate = NewChapterAdvanceGate(st, func(string) { e.abort() }, func(string, string) {})
	return e, events, done
}

func waitEngineDone(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Engine không dừng trong thời hạn")
	}
}

func mustInterventionFacts(t *testing.T, st *storepkg.Store) arbiter.InterventionFacts {
	t.Helper()
	facts, err := arbiter.CollectInterventionFacts(st)
	if err != nil {
		t.Fatalf("CollectInterventionFacts: %v", err)
	}
	return facts
}

func TestEngine_ReviewPermitWritesExactlyOneNewChapter(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Một", CoreEvent: "a"},
		{Chapter: 2, Title: "Hai", CoreEvent: "b"},
		{Chapter: 3, Title: "Ba", CoreEvent: "c"},
	}); err != nil {
		t.Fatal(err)
	}
	writer := subagent.Config{
		Name: "writer", Description: "test writer", Model: scriptedWriterModel(), SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st), tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st), tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns: 10, StopAfterTools: []string{"commit_chapter"},
	}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatal(err)
	}
	if err := st.RunMeta.GrantAdvancePermit(1); err != nil {
		t.Fatal(err)
	}
	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("load progress: %v", err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("Một giấy phép phải ổn định đúng một chương mới: %v", progress.CompletedChapters)
	}
	meta, _ := st.RunMeta.Load()
	if meta.AdvancePermitChapter != 0 {
		t.Fatalf("Sau khi nộp ổn định, giấy phép phải được dùng: %+v", meta)
	}
}

func TestEngine_StalePairedDispatchDoesNotBypassHold(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	e, _, _ := newTestEngine(t, st, subagent.NewRunner(), nil)
	e.pending = []controlOp{{
		hold:     &arbiter.AdvanceHoldOp{After: domain.AdvanceHoldAtBoundary, Reason: "Dừng lại trước"},
		dispatch: &arbiter.DispatchOp{Agent: "editor", Task: "Nhiệm vụ lỗi thời"},
		facts:    arbiter.InterventionFacts{Phase: string(domain.PhaseOutline)},
	}}

	if e.applyPendingOps(context.Background()) {
		t.Fatal("Lệnh phân phối cặp có dữ kiện lỗi thời không rơi vào next thì không được vượt Gate")
	}
	if e.next != nil || e.deferGateForNext {
		t.Fatalf("Lệnh phân phối lỗi thời không được để lại chỉ thị thực thi được: next=%+v defer=%v", e.next, e.deferGateForNext)
	}
	meta, _ := st.RunMeta.Load()
	if meta.AdvanceHold != nil {
		t.Fatalf("Khi lệnh phân phối cặp lỗi thời không được để lại hold mồ côi: %+v", meta.AdvanceHold)
	}
	if e.gate.HandleBoundary() {
		t.Fatal("Khi không có hold mồ côi, Gate không được giả tạo tạm dừng")
	}
}

// TestEngine_WritesBookToCompletion 完整链路:两章非分层书从 writing 写到 complete。
func TestEngine_WritesBookToCompletion(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Chương một", CoreEvent: "Khởi đầu"},
		{Chapter: 2, Title: "Chương hai", CoreEvent: "Kết cục"},
	}); err != nil {
		t.Fatalf("outline: %v", err)
	}

	writer := subagent.Config{
		Name: "writer", Description: "test writer",
		Model:        scriptedWriterModel(),
		SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st),
			tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st),
			tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns:       10,
		StopAfterTools: []string{"commit_chapter"},
	}
	e, events, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("load progress: %v", err)
	}
	if progress.Phase != domain.PhaseComplete {
		t.Fatalf("Viết đủ hai chương phải hoàn thành truyện, got phase=%s completed=%v", progress.Phase, progress.CompletedChapters)
	}
	if len(progress.CompletedChapters) != 2 {
		t.Fatalf("Phải hoàn thành 2 chương, got %v", progress.CompletedChapters)
	}
	// 事件形状:每章一条 DISPATCH(engine 发起),TOOL 行来自进度中继
	var dispatches, toolRows int
	for _, ev := range *events {
		switch ev.Category {
		case "DISPATCH":
			dispatches++
		case "TOOL":
			toolRows++
		}
	}
	if dispatches < 2 {
		t.Fatalf("Phải có ít nhất 2 sự kiện DISPATCH, got %d", dispatches)
	}
	if toolRows == 0 {
		t.Fatal("Tiến độ tool của Worker không được chiếu qua relay (thiếu dòng TOOL)")
	}
}

// TestEngine_WorkerFailureConsultsArbiterAndAborts 失败路径:
// 空转 writer 被 StopGuard 升级 → 重试一次 → Arbiter 裁定 abort → 暂停 + 审计。
func TestEngine_WorkerFailureConsultsArbiterAndAborts(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một", CoreEvent: "s"}}); err != nil {
		t.Fatalf("outline: %v", err)
	}

	var runs atomic.Int32
	// writer 每轮只回文字不落盘 → guard.NewWriterStopGuard 连续拦截后升级 → Execute 报错
	idle := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg("Tôi viết xong rồi (thật ra chẳng làm gì)")
	}}
	writer := subagent.Config{
		Name: "writer", Description: "idle writer",
		Model: idle, SystemPrompt: "test", MaxTurns: 20,
		StopGuardFactory: func(_, _ string) agentcore.StopGuard {
			runs.Add(1)
			return failNTimesGuard()
		},
	}
	// Arbiter 裁定 abort
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg(`{"action":"abort","dispatch":null,"reason":"writer quay rỗng liên tục, đề nghị kiểm tra thủ công cấu hình model"}`)
	}}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	if got := runs.Load(); got != 2 {
		t.Fatalf("Lần thất bại đầu phải thử lại một lần (tổng 2 lần spawn), got %d", got)
	}
	recs, err := st.Decisions.Recent(10)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var found bool
	for _, r := range recs {
		if r.Kind == "worker_failure" && r.Decider == "arbiter" {
			found = true
			if !strings.Contains(string(r.Decision), "abort") {
				t.Fatalf("Nội dung phán định phải chứa abort: %s", r.Decision)
			}
		}
	}
	if !found {
		t.Fatalf("Phán định worker_failure phải được ghi xuống đĩa: %+v", recs)
	}
}

// seedStuckRewrite 造出"第 2 章已完成并排进返工队列"的现场。
func seedStuckRewrite(t *testing.T, st *storepkg.Store) {
	t.Helper()
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(5); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(2, 3000, "", ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := st.Progress.SetPendingRewrites([]int{2}, "Thẩm duyệt yêu cầu viết lại", storepkg.RewriteCauseReview); err != nil {
		t.Fatalf("pending: %v", err)
	}
	if err := st.Progress.SetFlow(domain.FlowRewriting); err != nil {
		t.Fatalf("flow: %v", err)
	}
}

// TestEngine_DeadlockAbortDropsStuckRewrite 锁死 issue #110 的死锁面：僵局熔断时
// 卡死的返工章必须出队。PendingRewrites 是持久化事实，只暂停不出队的话重启会立刻
// 重放同一条死指令，把整本书永久锁死。
func TestEngine_DeadlockAbortDropsStuckRewrite(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	seedStuckRewrite(t, st)
	e, events, _ := newTestEngine(t, st, subagent.NewRunner(), nil)

	inst := &flow.Instruction{Agent: "writer", Task: "Viết lại chương 2", Chapter: 2}
	e.lastKey, e.repeats = instructionKey(inst), deadlockAbortAt-1

	if stop := e.trackDeadlock(context.Background(), &inst); !stop {
		t.Fatal("Ngắt mạch bế tắc vẫn phải dừng chờ can thiệp thủ công")
	}
	p, err := st.Progress.Load()
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if len(p.PendingRewrites) != 0 {
		t.Fatalf("Khi ngắt mạch, chương viết lại bị kẹt phải ra khỏi hàng đợi: %v", p.PendingRewrites)
	}
	if p.Flow != domain.FlowWriting {
		t.Fatalf("Sau khi hàng đợi trống, flow phải về writing, thực tế %s", p.Flow)
	}
	var notified bool
	for _, ev := range *events {
		if strings.Contains(ev.Summary, "đưa ra khỏi hàng đợi viết lại") {
			notified = true
		}
	}
	if !notified {
		t.Fatalf("Bỏ qua viết lại phải thông báo rõ cho người dùng: %+v", *events)
	}
}

// TestEngine_DropStuckRewriteOnlyTouchesQueuedChapter 出队是破坏性动作，误伤面必须钉死：
// 只有"排在返工队列里的那一章"可以被移出，其余指令一律不动队列。
func TestEngine_DropStuckRewriteOnlyTouchesQueuedChapter(t *testing.T) {
	cases := []struct {
		name string
		inst *flow.Instruction
	}{
		{"Lệnh không phải writer", &flow.Instruction{Agent: "editor", Task: "Thẩm duyệt cấp cung"}},
		{"Lệnh writer không liên quan đến chương", &flow.Instruction{Agent: "writer", Task: "viết tiếp"}},
		{"Chương viết tiếp không nằm trong hàng đợi", &flow.Instruction{Agent: "writer", Task: "Viết chương 3", Chapter: 3}},
		{"Lệnh rỗng", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := storepkg.NewStore(t.TempDir())
			seedStuckRewrite(t, st)
			e, _, _ := newTestEngine(t, st, subagent.NewRunner(), nil)
			if e.dropStuckRewrite(tc.inst) {
				t.Fatal("không được ra khỏi hàng đợi")
			}
			p, err := st.Progress.Load()
			if err != nil {
				t.Fatalf("progress: %v", err)
			}
			if len(p.PendingRewrites) != 1 || p.PendingRewrites[0] != 2 {
				t.Fatalf("Hàng đợi viết lại không được bị ảnh hưởng nhầm: %v", p.PendingRewrites)
			}
		})
	}
}

// TestEngine_TransientProviderFailuresDoNotBecomeDeadlock 回归第 135 章故障链：
// 两轮网络失败后的 worker_failure=retry 不能在下一轮被 trackDeadlock 当成
// “同一写作任务连续无进展”并触发 deadlock 改派。
func TestEngine_TransientProviderFailuresDoNotBecomeDeadlock(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(1); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Chương một", CoreEvent: "Khởi đầu"}}); err != nil {
		t.Fatalf("outline: %v", err)
	}

	network := &providerNetworkModel{}
	writer := subagent.Config{
		Name: "writer", Description: "network failing writer",
		Model: network, SystemPrompt: "test", MaxTurns: 5, MaxRetries: 0,
	}
	var arbiterCalls atomic.Int32
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		if arbiterCalls.Add(1) == 1 {
			return testTextMsg(`{"action":"retry","dispatch":null,"reason":"Lỗi mạng tạm thời, thử lại nhiệm vụ gốc"}`)
		}
		return testTextMsg(`{"action":"abort","dispatch":null,"reason":"Mạng liên tục không khả dụng, tạm dừng chờ khôi phục"}`)
	}}
	e, events, done := newTestEngine(t, st, subagent.NewRunner(writer), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	if got := network.calls.Load(); got != 4 {
		t.Fatalf("Hai chu kỳ thất bại của Engine mỗi chu kỳ chạy Worker 2 lần, got %d", got)
	}
	recs, err := st.Decisions.Recent(10)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var workerFailures, deadlocks int
	for _, rec := range recs {
		switch rec.Kind {
		case "worker_failure":
			workerFailures++
		case "deadlock":
			deadlocks++
		}
	}
	if workerFailures != 2 || deadlocks != 0 {
		t.Fatalf("Thất bại mạng chỉ được vào worker_failure, got worker_failure=%d deadlock=%d records=%+v", workerFailures, deadlocks, recs)
	}
	var failedDispatches, duplicateErrors int
	for _, ev := range *events {
		if ev.Category == "DISPATCH" && ev.Failed && ev.Kind == "network" && strings.Contains(ev.Detail, "test provider EOF") {
			failedDispatches++
		}
		if ev.Category == "ERROR" && strings.Contains(ev.Detail, "test provider EOF") {
			duplicateErrors++
		}
	}
	if failedDispatches != 4 || duplicateErrors != 0 {
		t.Fatalf("Mỗi lần Worker thất bại chỉ nên cập nhật DISPATCH, got dispatch=%d duplicate_error=%d events=%+v", failedDispatches, duplicateErrors, *events)
	}
}

// failNTimesGuard 立即升级的 StopGuard(模拟空转熔断)。
func failNTimesGuard() agentcore.StopGuard {
	return func(context.Context, agentcore.StopInfo) agentcore.StopDecision {
		return agentcore.StopDecision{Allow: false, Escalate: true}
	}
}

// TestEngine_RetriesUnfinishedPlanStart 启动裁定失败后的自愈路径:StartPrompt 已落盘、
// PlanStart 缺位(启动时模型故障)→ 引擎起动时现场补裁 → 固化 PlanStartRecord → 派发规划师。
// 规划师不落盘 → 走既有僵局路径停机,证明补裁后引擎回到正常轨道。
func TestEngine_RetriesUnfinishedPlanStart(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(0); err != nil {
		t.Fatalf("progress: %v", err)
	}
	// 模拟 StartPrepared 失败现场:输入事实在,裁定事实缺位。
	if err := st.RunMeta.SetStartPrompt("Phàm Nhân Tu Tiên"); err != nil {
		t.Fatalf("start prompt: %v", err)
	}

	// Arbiter:首次调用是补裁(plan_start),之后是僵局咨询(abort 收尾)。
	var arbCalls atomic.Int32
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		if arbCalls.Add(1) == 1 {
			return testTextMsg(`{"planner":"architect_long","task":"Lập khung ba tập xoay quanh Phàm Nhân Tu Tiên","reason":"Đề tài tu tiên trường thiên"}`)
		}
		return testTextMsg(`{"action":"abort","dispatch":null,"reason":"Người lập kế hoạch quay rỗng, dừng máy"}`)
	}}
	// 规划师成功返回但不落任何盘 → Route 始终返回同一补齐指令 → 僵局。
	architect := subagent.Config{
		Name: "architect_long", Description: "idle planner",
		Model: &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
			return testTextMsg("Đã lập kế hoạch (thực ra chưa ghi xuống đĩa)")
		}},
		SystemPrompt: "test", MaxTurns: 3,
	}
	e, events, done := newTestEngine(t, st, subagent.NewRunner(architect), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil || meta.PlanStart == nil {
		t.Fatalf("Sau phán định bổ sung PlanStart phải được cố định, meta=%+v err=%v", meta, err)
	}
	if meta.PlanStart.Planner != "architect_long" || meta.PlanStart.RawPrompt != "Phàm Nhân Tu Tiên" || meta.PlanStart.DecisionID == "" {
		t.Fatalf("PlanStartRecord thiếu trường: %+v", meta.PlanStart)
	}
	recs, err := st.Decisions.Recent(10)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var planStartRec bool
	for _, r := range recs {
		if r.Kind == "plan_start" && strings.Contains(string(r.Decision), "architect_long") {
			planStartRec = true
		}
	}
	if !planStartRec {
		t.Fatalf("Phán định bổ sung phải để lại audit plan_start: %+v", recs)
	}
	var dispatched, healed bool
	for _, ev := range *events {
		if ev.Category == "DISPATCH" {
			dispatched = true
		}
		if strings.Contains(ev.Summary, "Đã bổ sung phán định khởi động") {
			healed = true
		}
	}
	if !dispatched || !healed {
		t.Fatalf("Sau phán định bổ sung phải điều phối người lập kế hoạch và hiển thị sự kiện bổ sung, dispatched=%v healed=%v", dispatched, healed)
	}
}

// TestEngine_PlanStartRetryFailurePauses 补裁失败不允许无声停机:
// Arbiter 持续不可用 → 显式暂停回显 + plan_start 审计带 error + 零派发。
func TestEngine_PlanStartRetryFailurePauses(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(0); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.RunMeta.SetStartPrompt("Phàm Nhân Tu Tiên"); err != nil {
		t.Fatalf("start prompt: %v", err)
	}

	var e *engine
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		e.abort() // 模拟宿主取消持续失败的调用，失败路径由 context 明确结束。
		return testTextMsg("Đây không phải JSON")
	}}
	e, events, done := newTestEngine(t, st, subagent.NewRunner(), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	for _, ev := range *events {
		if ev.Category == "DISPATCH" {
			t.Fatal("Phán định bổ sung thất bại thì không được điều phối worker nào")
		}
	}
	var paused bool
	for _, ev := range *events {
		if strings.Contains(ev.Summary, "Phán định khởi động thất bại") {
			paused = true
		}
	}
	if !paused {
		t.Fatalf("Phán định bổ sung thất bại phải hiển thị rõ lý do tạm dừng, events=%+v", *events)
	}
	recs, err := st.Decisions.Recent(5)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var errRec bool
	for _, r := range recs {
		if r.Kind == "plan_start" && r.Error != "" && len(r.Decision) == 0 {
			errRec = true
		}
	}
	if !errRec {
		t.Fatalf("Phán định thất bại phải ghi xuống đĩa kèm error: %+v", recs)
	}
}

// TestEngine_DeadlockConsultsArbiter 僵局路径:规划补齐指令连续重现
// → 第 3 次咨询 Arbiter → abort 停机 + deadlock 审计。
func TestEngine_DeadlockConsultsArbiter(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("progress: %v", err)
	}
	// 规划期 + tier 已知 + 缺项恒在 → Route 每轮产出同一补齐指令
	if err := st.RunMeta.SetPlanningTier(domain.PlanningTierLong); err != nil {
		t.Fatalf("tier: %v", err)
	}

	// architect 无守卫、成功返回但不落任何盘 → Route 指令恒定
	lazy := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg("Đã biết (không làm gì cả)")
	}}
	architect := subagent.Config{
		Name: "architect_long", Description: "lazy architect",
		Model: lazy, SystemPrompt: "test", MaxTurns: 5,
	}
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg(`{"action":"abort","dispatch":null,"reason":"Người lập kế hoạch liên tục không có sản phẩm"}`)
	}}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(architect), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	recs, err := st.Decisions.Recent(10)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var found bool
	for _, r := range recs {
		if r.Kind == "deadlock" && r.Decider == "arbiter" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Phán định deadlock phải được ghi xuống đĩa: %+v", recs)
	}
}

// TestEngine_IntermediateCheckpointsDoNotMaskDeadlock 锁定 #84：Writer 反复修改
// 草稿会产生新 digest 和新 edit checkpoint，但只要 Route 仍是同一个
// “打磨第 1 章”，就说明 Engine 级后置条件(commit)未完成，必须继续累计僵局。
func TestEngine_IntermediateCheckpointsDoNotMaskDeadlock(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(1); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Chương một", CoreEvent: "Khởi đầu"}}); err != nil {
		t.Fatalf("outline: %v", err)
	}
	if err := st.Drafts.SaveDraft(1, "Phiên bản 0 bản thảo đầu"); err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(1, len([]rune("Phiên bản 0 bản thảo đầu")), "mystery", "quest"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := st.Progress.SetPendingRewrites([]int{1}, "Kiểm thử gọt giũa không nộp", storepkg.RewriteCauseReview); err != nil {
		t.Fatalf("pending rewrite: %v", err)
	}
	if err := st.Progress.SetFlow(domain.FlowPolishing); err != nil {
		t.Fatalf("flow: %v", err)
	}

	writerModel := &editThenCancelModel{}
	writer := subagent.Config{
		Name: "writer", Description: "edit then cancel writer",
		Model: writerModel, SystemPrompt: "test",
		Tools:    []agentcore.Tool{tools.NewEditChapterTool(st)},
		MaxTurns: 5,
	}
	// 即使 Arbiter 对 worker_failure / deadlock 一直要求 retry，现有第 5 次
	// 硬熔断也必须在派发前截停，不得被 edit checkpoint 重置。
	arb := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		return testTextMsg(`{"action":"retry","dispatch":null,"reason":"Tiếp tục thử lại"}`)
	}}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), arb)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	if got := writerModel.edits.Load(); got != deadlockAbortAt-1 {
		t.Fatalf("deadlock phải ngắt cứng trước lần điều phối thứ %d, thực tế edit %d lần", deadlockAbortAt, got)
	}
	var edits int
	for _, cp := range st.Checkpoints.All() {
		if cp.Scope.Matches(domain.ChapterScope(1)) && cp.Step == "edit" {
			edits++
		}
	}
	if edits != deadlockAbortAt-1 {
		t.Fatalf("Phải giữ %d edit checkpoint khác nhau, thực tế %d", deadlockAbortAt-1, edits)
	}
	recs, err := st.Decisions.Recent(10)
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	var hasWorkerFailure, hasDeadlockWithCause bool
	for _, rec := range recs {
		switch rec.Kind {
		case "worker_failure":
			hasWorkerFailure = true
		case "deadlock":
			var facts arbiter.FailureFacts
			if err := json.Unmarshal(rec.Facts, &facts); err != nil {
				t.Fatalf("decode deadlock facts: %v", err)
			}
			if facts.ErrorKind == "canceled" && strings.Contains(facts.Error, "context canceled") {
				hasDeadlockWithCause = true
			}
		}
	}
	if !hasWorkerFailure || !hasDeadlockWithCause {
		t.Fatalf("Phải ghi worker_failure trước, deadlock phải giữ lỗi cuối: %+v", recs)
	}
}

// TestEngine_PauseWithEditorDispatchWaitsForRewriteQueue 修复验证(评审阻断2):
// Arbiter 返工裁定 = 停靠点 + 派 editor 入队。停靠点必须等 editor 建立返工队列、
// writer 重写排空之后才消费——不能在 editor 执行前被"队列已排空"误判消费。
func TestEngine_PauseWithEditorDispatchWaitsForRewriteQueue(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Một", CoreEvent: "a"},
		{Chapter: 2, Title: "Hai", CoreEvent: "b"},
		{Chapter: 3, Title: "Ba", CoreEvent: "c"},
	}); err != nil {
		t.Fatalf("outline: %v", err)
	}
	// The hold waits for the successful rewrite and its targeted recheck.
	if err := st.Progress.StartChapter(1); err != nil {
		t.Fatalf("start ch1: %v", err)
	}
	if err := st.Progress.MarkChapterComplete(1, 1200, "crisis", "quest"); err != nil {
		t.Fatalf("complete ch1: %v", err)
	}

	// editor:一次 save_review(verdict=rewrite, affected=[1]) 把第 1 章入队。
	editorModel := &scriptedChatModel{fn: func(msgs []llmtypes.Message) llmtypes.Message {
		toolResults := 0
		for _, m := range msgs {
			if m.Role == llmtypes.RoleTool {
				toolResults++
			}
		}
		if toolResults == 0 {
			progress, err := st.Progress.Load()
			if err != nil {
				t.Fatal(err)
			}
			if progress != nil && slices.Contains(progress.PendingRechecks, 1) {
				source, err := st.World.LoadReview(1)
				if err != nil {
					t.Fatal(err)
				}
				if source == nil {
					t.Fatal("pending recheck lost its source review")
				}
				return testToolCallMsg("save_review", reviewArgs(t, domain.ReviewEntry{
					Chapter: 1, Scope: "chapter", Recheck: true, Verdict: "accept",
					Summary:    "Đã kiểm lại giọng văn",
					Dimensions: []domain.DimensionScore{{Dimension: "aesthetic", Score: 80, Comment: "Đã đọc bản viết lại"}},
					Issues:     []domain.ConsistencyIssue{},
					RecheckResults: []domain.RecheckResult{{
						FindingID: domain.ReviewEntryItemIDs(*source)[0], Status: "unknown",
						Note: "Vòng nguồn không lưu đoạn trích cụ thể để đối chiếu chắc chắn",
					}},
				}))
			}
			return testToolCallMsg("save_review", reviewArgs(t, domain.ReviewEntry{
				Chapter: 1, Scope: "chapter", Verdict: "rewrite",
				Summary: "Giọng văn chương 1 cần viết lại",
				Dimensions: []domain.DimensionScore{
					{Dimension: "consistency", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "character", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "pacing", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "continuity", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "foreshadow", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "hook", Score: 85, Comment: "Đạt (trích: nguyên văn)"},
					{Dimension: "aesthetic", Score: 55, Comment: "Giọng văn không phù hợp (trích: đoạn đầu nguyên văn)"},
				},
				Issues: []domain.ConsistencyIssue{{
					Type: "aesthetic", Severity: domain.SeverityError, Description: "Giọng văn", Evidence: "nguyên văn",
					Suggestion: "chỉnh lạnh hơn", Chapters: []int{1}, RequiresChange: true,
				}},
			}))
		}
		return testTextMsg("done")
	}}
	editor := subagent.Config{
		Name: "editor", Description: "test editor", Model: editorModel,
		SystemPrompt: "test", MaxTurns: 6,
		Tools:          []agentcore.Tool{tools.NewSaveReviewTool(st, nil)},
		StopAfterTools: []string{"save_review"},
	}
	writer := subagent.Config{
		Name: "writer", Description: "test writer", Model: scriptedWriterModel(),
		SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st),
			tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st),
			tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns: 10, StopAfterTools: []string{"commit_chapter"},
	}

	e, _, done := newTestEngine(t, st, subagent.NewRunner(editor, writer), nil)
	// 模拟 Arbiter 返工裁定:hold + dispatch editor(引擎未运行 → 立即应用)。
	e.applyControlOp(context.Background(), controlOp{
		hold:     &arbiter.AdvanceHoldOp{After: domain.AdvanceHoldAfterRewritesDrained, Reason: "Viết lại giọng văn chương 1, sửa xong tạm dừng nghiệm thu"},
		dispatch: &arbiter.DispatchOp{Agent: "editor", Task: "Rà lại chương 1: chỉnh giọng văn lạnh hơn, dùng issues[].chapters và requires_change để vào hàng đợi"},
		facts:    mustInterventionFacts(t, st),
	})
	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("load progress: %v", err)
	}
	// 核心断言①:停靠点没有在 editor 入队前消费——第 1 章确实经历了重写
	//(重写 commit 会把它从队列 drain 掉)。
	if len(progress.PendingRewrites) != 0 {
		t.Fatalf("Hàng đợi viết lại phải đã trống, got %v", progress.PendingRewrites)
	}
	if len(progress.PendingRechecks) != 0 {
		t.Fatalf("Hold phải chờ kiểm lại xong, pending_rechecks=%v", progress.PendingRechecks)
	}
	current, err := st.World.LoadReview(1)
	if err != nil || current == nil || !current.Recheck {
		t.Fatalf("Phải lưu vòng kiểm lại trước khi dùng hold: review=%+v err=%v", current, err)
	}
	if progress.ChapterWordCounts[1] == 1200 {
		t.Fatal("Chương 1 phải thực sự được viết lại (số chữ phải thay đổi)")
	}
	// 核心断言②:排空后停靠点消费,引擎暂停——第 2 章不应被续写。
	if len(progress.CompletedChapters) != 1 {
		t.Fatalf("Điểm dừng phải tạm dừng trước khi viết tiếp chương 2, completed=%v", progress.CompletedChapters)
	}
	meta, _ := st.RunMeta.Load()
	if meta != nil && meta.AdvanceHold != nil {
		t.Fatalf("Tạm dừng một lần phải đã được dùng, got %+v", meta.AdvanceHold)
	}
}

// TestEngine_BoundaryHoldDoesNotDispatchAnotherWorker 回归：
// 用户干预只裁定出 boundary hold（无派单）时，引擎必须在当前边界立即
// 消费 hold 并暂停，不得再多写一章。
func TestEngine_BoundaryHoldDoesNotDispatchAnotherWorker(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Một", CoreEvent: "a"},
		{Chapter: 2, Title: "Hai", CoreEvent: "b"},
		{Chapter: 3, Title: "Ba", CoreEvent: "c"},
	}); err != nil {
		t.Fatalf("outline: %v", err)
	}

	writer := subagent.Config{
		Name: "writer", Description: "test writer", Model: scriptedWriterModel(),
		SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st),
			tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st),
			tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns: 10, StopAfterTools: []string{"commit_chapter"},
	}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)
	if !e.start(nil) {
		t.Fatal("engine start")
	}
	// 第 1 章写作期间到达 hold-only 干预（与真实 Steer 时序一致）。
	e.enqueue(controlOp{
		hold:  &arbiter.AdvanceHoldOp{After: domain.AdvanceHoldAtBoundary, Reason: "Dừng một chút để tôi xem"},
		facts: mustInterventionFacts(t, st),
	})
	waitEngineDone(t, done)

	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("load progress: %v", err)
	}
	// 干预在第 1 章运行中到达 → 第 1 章写完;停靠点在边界立即消费 → 第 2 章不得开写。
	if n := len(progress.CompletedChapters); n > 1 {
		t.Fatalf("Sau boundary hold không được viết thêm chương nào, completed=%v", progress.CompletedChapters)
	}
	meta, _ := st.RunMeta.Load()
	if meta != nil && meta.AdvanceHold != nil {
		t.Fatalf("Tạm dừng một lần phải đã được dùng, got %+v", meta.AdvanceHold)
	}
}

func TestEngine_TargetChapterHoldStopsAtRequestedChapter(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(3); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatal(err)
	}
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Một", CoreEvent: "a"},
		{Chapter: 2, Title: "Hai", CoreEvent: "b"},
		{Chapter: 3, Title: "Ba", CoreEvent: "c"},
	}); err != nil {
		t.Fatal(err)
	}

	writer := subagent.Config{
		Name: "writer", Description: "test writer", Model: scriptedWriterModel(), SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st), tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st), tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns: 10, StopAfterTools: []string{"commit_chapter"},
	}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)
	if err := st.RunMeta.SetAdvanceHold(domain.AdvanceHold{
		After: domain.AdvanceHoldAtChapter, TargetChapter: 2, Reason: "Viết đến chương 2",
	}); err != nil {
		t.Fatal(err)
	}
	if !e.start(nil) {
		t.Fatal("engine start")
	}
	waitEngineDone(t, done)

	progress, err := st.Progress.Load()
	if err != nil || progress == nil {
		t.Fatalf("load progress: %v", err)
	}
	if !slices.Equal(progress.CompletedChapters, []int{1, 2}) {
		t.Fatalf("Phải dừng đúng ở chương 2, completed=%v", progress.CompletedChapters)
	}
	meta, _ := st.RunMeta.Load()
	if meta.AdvanceHold != nil {
		t.Fatalf("Hold chương mục tiêu phải đã được dùng: %+v", meta.AdvanceHold)
	}
}

// TestEngine_ExitRaceRestoresPendingDispatch 回归(评审阻断3):
// 干预入队与引擎退出竞态时,残留的裁定派单不得无声丢弃——PendingSteer 必须回存,
// pause 类事实动作必须补执行。
func TestEngine_ExitRaceRestoresPendingDispatch(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}

	// worker 挂起直到 ctx 取消:制造"入队后引擎被 abort"的窗口。
	blocked := &scriptedChatModel{fn: func([]llmtypes.Message) llmtypes.Message {
		time.Sleep(50 * time.Millisecond)
		return testTextMsg("...")
	}}
	writer := subagent.Config{Name: "writer", Description: "slow", Model: blocked, SystemPrompt: "t", MaxTurns: 100}
	// 需要 outline 让 Route 派 writer
	if err := st.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Một", CoreEvent: "a"}, {Chapter: 2, Title: "Hai", CoreEvent: "b"}}); err != nil {
		t.Fatalf("outline: %v", err)
	}
	e, _, done := newTestEngine(t, st, subagent.NewRunner(writer), nil)

	if !e.start(nil) {
		t.Fatal("engine start")
	}
	// worker 运行中:入队 pause+dispatch,随即 abort(动作永远等不到下个边界)。
	e.enqueue(controlOp{
		hold:     &arbiter.AdvanceHoldOp{After: domain.AdvanceHoldAfterRewritesDrained, Reason: "nghiệm thu"},
		dispatch: &arbiter.DispatchOp{Agent: "writer", Task: "Viết lại chương 1"},
		text:     "Viết lại chương 1 rồi dừng lại",
		facts:    mustInterventionFacts(t, st),
	})
	e.abort()
	waitEngineDone(t, done)

	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil {
		t.Fatalf("load meta: %v", err)
	}
	if meta.PendingSteer != "Viết lại chương 1 rồi dừng lại" {
		t.Fatalf("Lệnh phân công còn sót phải lưu lại PendingSteer để khôi phục phát lại, got %q", meta.PendingSteer)
	}
	if meta.AdvanceHold == nil {
		t.Fatal("Hành động dữ kiện hold phải được bổ sung thực thi khi dọn dẹp lúc thoát")
	}
}

// Gói P · P4 — arbiter ask：裁定"停下来问用户"必须落盘 PendingAsk 并让引擎
// 在边界用问题文本收尾暂停；用户后续干预走同一条通道。
func TestEngineApplyControlOpPersistsAsk(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	e, _, _ := newTestEngine(t, st, nil, nil)
	if err := e.applyControlOp(context.Background(), controlOp{
		ask: &arbiter.AskOp{
			Question: "Giữ nguyên rồi dời dàn ý, hay cắt hai cảnh cho khớp?",
			Chapter:  5,
			Options: []arbiter.AskOptionOp{
				{Label: "Giữ nguyên, dời dàn ý", Reply: "Giữ cả hai cảnh", IsDefault: true},
				{Label: "Cắt cho khớp dàn ý"},
			},
		},
		text:  "lệch dàn ý",
		facts: mustInterventionFacts(t, st),
	}); err != nil {
		t.Fatalf("applyControlOp ask: %v", err)
	}
	meta, err := st.RunMeta.Load()
	if err != nil || meta == nil || meta.PendingAsk == nil {
		t.Fatalf("PendingAsk must persist: %v %+v", err, meta)
	}
	if meta.PendingAsk.Chapter != 5 || len(meta.PendingAsk.Options) != 2 ||
		!meta.PendingAsk.Options[0].IsDefault || meta.PendingAsk.Options[0].Reply != "Giữ cả hai cảnh" {
		t.Fatalf("PendingAsk projection mismatch: %+v", meta.PendingAsk)
	}
	// 引擎在边界停下时，暂停摘要必须带上 Trọng tài 的问题（不是通用文案）。
	paused := ""
	e.onPause = func(summary string) { paused = summary }
	if !e.pauseAtBoundary() {
		t.Fatal("ask phải yêu cầu dừng ở ranh giới")
	}
	if !strings.Contains(paused, "Giữ nguyên rồi dời dàn ý") {
		t.Fatalf("pause summary phải chứa câu hỏi, got %q", paused)
	}
	// 相同提问幂等，不会覆盖在途提问。
	if err := e.applyControlOp(context.Background(), controlOp{
		ask: &arbiter.AskOp{Question: "Giữ nguyên rồi dời dàn ý, hay cắt hai cảnh cho khớp?", Chapter: 5,
			Options: []arbiter.AskOptionOp{{Label: "Giữ nguyên, dời dàn ý", Reply: "Giữ cả hai cảnh", IsDefault: true}, {Label: "Cắt cho khớp dàn ý"}}},
		facts: mustInterventionFacts(t, st),
	}); err != nil {
		t.Fatalf("idempotent re-apply: %v", err)
	}
}
