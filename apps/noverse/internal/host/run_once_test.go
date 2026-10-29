package host

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"github.com/datnt1998/noverse/internal/agent"
	"github.com/datnt1998/noverse/internal/agent/subagent"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/flow"
	storepkg "github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

// onceTestWriterStore 预置一本处于写作期、共 3 章的新书(与 eval fixture
// 播种形态同构:phase=writing,无 arbiter,无 StartPrepared)。
func onceTestWriterStore(t *testing.T) *storepkg.Store {
	t.Helper()
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
	return st
}

// onceTestHost 组装一个最小 Host:真实 store/observer/gate + 假 worker,
// 走 RunInstructionOnce → engine.startOnce → runEnded → Done 全链路。
func onceTestHost(t *testing.T, st *storepkg.Store, model llmtypes.ChatModel) (*Host, *engine) {
	t.Helper()
	writer := subagent.Config{
		Name: "writer", Description: "test writer", Model: model, SystemPrompt: "test",
		Tools: []agentcore.Tool{
			tools.NewPlanChapterTool(st), tools.NewDraftChapterTool(st),
			tools.NewCheckConsistencyTool(st), tools.NewCommitChapterTool(st, tools.NewStyleStatsIndex(st)),
		},
		MaxTurns: 10, StopAfterTools: []string{"commit_chapter"},
	}
	e, _, _ := newTestEngine(t, st, subagent.NewRunner(writer), nil)
	h := &Host{
		store:    st,
		engine:   e,
		observer: e.observer,
		usage:    &UsageTracker{},
		events:   make(chan Event, 32),
		streamCh: make(chan string, 32),
		done:     make(chan struct{}, 4),
	}
	e.onDone = h.runEnded
	e.onPause = func(summary string) { h.abortWithEvent(summary, "warn") }
	return h, e
}

// countFirstCalls 包装模型,按"对话内尚无 tool 结果"的首次调用计数:
// 每次 worker 派发恰好有一次这样的调用,即单指令运行的派发次数。
type countFirstCalls struct {
	base  *scriptedChatModel
	runs  *atomic.Int32
	block func()
}

func (m *countFirstCalls) Generate(ctx context.Context, msgs []llmtypes.Message, specs []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return m.base.Generate(ctx, msgs, specs, opts...)
}

func (m *countFirstCalls) GenerateStream(ctx context.Context, msgs []llmtypes.Message, specs []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	toolResults := 0
	for _, msg := range msgs {
		if msg.Role == llmtypes.RoleTool {
			toolResults++
		}
	}
	if toolResults == 0 {
		m.runs.Add(1)
	}
	if m.block != nil {
		m.block()
	}
	return m.base.GenerateStream(ctx, msgs, specs, opts...)
}

func (m *countFirstCalls) SupportsTools() bool { return true }

func waitHostDone(t *testing.T, h *Host) {
	t.Helper()
	select {
	case <-h.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("Engine không dừng trong thời hạn")
	}
}

func TestRunInstructionOnce_IgnoresPendingPause(t *testing.T) {
	st := onceTestWriterStore(t)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	// 模拟"引擎曾在跑、用户已请求边界暂停、循环随后停下"的残留现场:
	// pauseAfterStep 为真但引擎已停。
	e.mu.Lock()
	e.running = true
	e.mu.Unlock()
	h.lifecycle = lifecycleRunning
	if !h.PauseAfterStep() {
		t.Fatal("pause request rejected while engine marked running")
	}
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
	h.lifecycle = lifecycleIdle

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	if got := runs.Load(); got != 1 {
		t.Fatalf("đúng một worker phải chạy, thực tế %d", runs.Load())
	}
	snap := h.Snapshot()
	if snap.OnceResult == nil || !snap.OnceResult.Finished || snap.OnceResult.Agent != "writer" || snap.OnceResult.Err != "" {
		t.Fatalf("OnceResult không đúng: %+v", snap.OnceResult)
	}
	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
		t.Fatalf("chương 1 phải được hoàn thành: %v", progress.CompletedChapters)
	}
}

// TestRunInstructionOnce_PauseBeforeFirstDispatch 复现窗口:pause 请求在
// startOnce 返回之后才、首个 worker 派发之前到达。借 e.mu 的饥饿模式排队做
// 确定性交错:测试先占锁,pause 写入方与引擎循环依次排队,保证 pauseAfterStep
// 恰好在循环读取边界检查前落位。
func TestRunInstructionOnce_PauseBeforeFirstDispatch(t *testing.T) {
	st := onceTestWriterStore(t)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	var wg sync.WaitGroup
	e.mu.Lock()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !e.startOnce(&flow.Instruction{Agent: "writer", Task: "Viết chương 1"}) {
			t.Error("startOnce phải khởi động được")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.mu.Lock()
		e.pauseAfterStep = true
		e.mu.Unlock()
	}()
	time.Sleep(10 * time.Millisecond)
	e.mu.Unlock()
	wg.Wait()

	waitHostDone(t, h)
	e.wait()

	if got := runs.Load(); got != 1 {
		t.Fatalf("pause 晚于启动到达也不得拦首次派发, thực tế %d lần", got)
	}
	snap := h.Snapshot()
	if snap.OnceResult == nil || !snap.OnceResult.Finished || snap.OnceResult.Agent != "writer" {
		t.Fatalf("OnceResult không đúng: %+v", snap.OnceResult)
	}
}

func TestRunInstructionOnce_FastWorkerRunsExactlyOnce(t *testing.T) {
	st := onceTestWriterStore(t)
	var runs atomic.Int32
	model := &countFirstCalls{base: scriptedWriterModel(), runs: &runs}
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	if got := runs.Load(); got != 1 {
		t.Fatalf("worker nhanh cũng chỉ chạy đúng một lần, thực tế %d", runs.Load())
	}
	// 收尾等价正常停机:lifecycle 落到非 running 终态,Done 已触发。
	h.mu.Lock()
	state := h.lifecycle
	h.mu.Unlock()
	if state == lifecycleRunning {
		t.Fatal("lifecycle vẫn running sau khi lượt chạy đơn lệnh kết thúc")
	}
}

func TestRunInstructionOnce_RejectsWhileRunning(t *testing.T) {
	st := onceTestWriterStore(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var blockOnce sync.Once
	model := &countFirstCalls{
		base: scriptedWriterModel(),
		runs: &atomic.Int32{},
		block: func() {
			blockOnce.Do(func() { close(entered); <-release })
		},
	}
	var releaseOnce sync.Once
	releaseWorker := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseWorker)
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce đầu: %v", err)
	}
	<-entered
	if err := h.RunInstructionOnce("writer", "Viết chương 2"); err == nil {
		t.Fatal("gọi khi engine đang chạy phải báo lỗi")
	}
	releaseWorker()
	waitHostDone(t, h)
	e.wait()
}

func TestRunInstructionOnce_WorkerErrorStillCompletes(t *testing.T) {
	st := onceTestWriterStore(t)
	model := &providerNetworkModel{}
	h, e := onceTestHost(t, st, model)

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	e.wait()

	if got := model.calls.Load(); got != 1 {
		t.Fatalf("worker lỗi cũng chỉ chạy đúng một lần, thực tế %d", got)
	}
	snap := h.Snapshot()
	if snap.OnceResult == nil || !snap.OnceResult.Finished || snap.OnceResult.Agent != "writer" {
		t.Fatalf("OnceResult không ghi nhận worker: %+v", snap.OnceResult)
	}
	if snap.OnceResult.Err == "" {
		t.Fatal("OnceResult.Err phải ghi lại lỗi worker")
	}
}

func TestRunInstructionOnce_RejectsInvalidAgent(t *testing.T) {
	st := onceTestWriterStore(t)
	h, _ := onceTestHost(t, st, scriptedWriterModel())

	for _, agent := range []string{"architect_long", "architect", "compass", "", "EDITOR"} {
		if err := h.RunInstructionOnce(agent, "task"); err == nil {
			t.Fatalf("agent %q phải bị từ chối", agent)
		}
	}
	if err := h.RunInstructionOnce("writer", "   "); err == nil {
		t.Fatal("task rỗng phải bị từ chối")
	}

	// editor 是合法 agent:入口必须受理。runner 没有注册 editor worker,
	// 派发失败也按同一停机路径收尾——Done 照常触发,结果落进 OnceResult。
	if err := h.RunInstructionOnce("editor", "Đánh giá chương 1"); err != nil {
		t.Fatalf("editor phải được chấp nhận: %v", err)
	}
	waitHostDone(t, h)
	snap := h.Snapshot()
	if snap.OnceResult == nil || snap.OnceResult.Agent != "editor" || !snap.OnceResult.Finished || snap.OnceResult.Err == "" {
		t.Fatalf("OnceResult cho editor không đúng: %+v", snap.OnceResult)
	}
}

func TestRunInstructionOnce_CloseAfterDoneDoesNotPanic(t *testing.T) {
	st := onceTestWriterStore(t)
	h, _ := onceTestHost(t, st, scriptedWriterModel())

	if err := h.RunInstructionOnce("writer", "Viết chương 1"); err != nil {
		t.Fatalf("RunInstructionOnce: %v", err)
	}
	waitHostDone(t, h)
	h.Close()
	h.Close()
}
