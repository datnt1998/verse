package arbiter

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

// scriptedModel 按调用序号返回预设文本。
type scriptedModel struct {
	outputs        []string
	idx            int64
	lastCfg        llmtypes.CallConfig
	lastMsgs       []llmtypes.Message
	rejectThinking bool
}

func (m *scriptedModel) take() string {
	i := int(atomic.AddInt64(&m.idx, 1) - 1)
	if i >= len(m.outputs) {
		return m.outputs[len(m.outputs)-1]
	}
	return m.outputs[i]
}

func (m *scriptedModel) Generate(_ context.Context, messages []llmtypes.Message, _ []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	m.lastCfg = llmtypes.ResolveCallConfig(opts)
	m.lastMsgs = messages
	if m.rejectThinking && m.lastCfg.ThinkingLevel != llmtypes.ThinkingAuto {
		return nil, errors.New("thinking is only supported for reasoning chat models")
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role:    llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.TextBlock(m.take())},
	}}, nil
}

func TestDecidePlanStartDoesNotSendThinkingToChatModel(t *testing.T) {
	m := &scriptedModel{outputs: []string{
		`{"planner":"architect_short","task":"规划短篇","reason":"篇幅较短"}`,
	}, rejectThinking: true}
	if _, err := DecidePlanStart(t.Context(), m, "sys", "写一部短篇", ""); err != nil {
		t.Fatalf("decide: %v", err)
	}
	if m.lastCfg.ThinkingLevel != llmtypes.ThinkingAuto {
		t.Fatalf("Arbiter 不应向普通 chat 模型发送 thinking 参数, got %q", m.lastCfg.ThinkingLevel)
	}
	if m.lastCfg.MaxTokens != decideMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", m.lastCfg.MaxTokens, decideMaxTokens)
	}
}

func TestDecidePromptContractAppendsSchema(t *testing.T) {
	m := &scriptedModel{outputs: []string{
		`{"planner":"architect_short","task":"规划短篇","reason":"篇幅较短"}`,
	}}
	const semanticPrompt = "只根据需求判断规划方式。"
	if _, err := DecidePlanStart(t.Context(), m, semanticPrompt, "写一部短篇", ""); err != nil {
		t.Fatal(err)
	}
	got := m.lastMsgs[0].TextContent()
	for _, want := range []string{semanticPrompt, "<output-json-schema>", `"planner"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt contract 缺少 %q:\n%s", want, got)
		}
	}
}

func (m *scriptedModel) GenerateStream(ctx context.Context, msgs []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	resp, _ := m.Generate(ctx, msgs, tools, opts...)
	ch := make(chan llmtypes.StreamEvent, 1)
	ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventDone, Message: resp.Message}
	close(ch)
	return ch, nil
}

func (m *scriptedModel) SupportsTools() bool { return true }

type retryableTestError struct {
	retryable bool
}

func (e retryableTestError) Error() string             { return "provider unavailable" }
func (e retryableTestError) Retryable() bool           { return e.retryable }
func (e retryableTestError) RetryAfter() time.Duration { return time.Millisecond }

type failingThenValidModel struct {
	failures int64
	calls    int64
}

func (m *failingThenValidModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	call := atomic.AddInt64(&m.calls, 1)
	if call <= m.failures {
		return nil, retryableTestError{retryable: true}
	}
	return &llmtypes.LLMResponse{Message: llmtypes.Message{
		Role: llmtypes.RoleAssistant,
		Content: []llmtypes.ContentBlock{llmtypes.TextBlock(
			`{"planner":"architect_short","task":"规划短篇","reason":"篇幅较短"}`)},
	}}, nil
}

func (m *failingThenValidModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, errors.New("unused")
}

func (m *failingThenValidModel) SupportsTools() bool { return true }

func TestDecidePlanStart_ValidAndFeedbackRetry(t *testing.T) {
	// 第一次输出非法(planner 错),第二次带围栏但合法——反馈重试 + JSON 提取都要工作。
	m := &scriptedModel{outputs: []string{
		`{"planner":"writer","task":"x","reason":"r"}`,
		"```json\n{\"planner\":\"architect_short\",\"task\":\"写一个 20 章的悬疑短篇……\",\"reason\":\"用户显式要求短篇\"}\n```",
	}}
	d, err := DecidePlanStart(context.Background(), m, "sys", "20章悬疑短篇", "suspense")
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d.Planner != "architect_short" || !strings.Contains(d.Task, "悬疑") {
		t.Fatalf("裁定错误: %+v", d)
	}
	if got := atomic.LoadInt64(&m.idx); got != 2 {
		t.Fatalf("应恰好 2 次调用（1 非法 + 1 反馈重试成功），got %d", got)
	}
}

func TestDecide_InvalidOutputStopsAfterCorrectionCap(t *testing.T) {
	m := &scriptedModel{outputs: []string{"完全不是 JSON"}}
	_, err := DecidePlanStart(context.Background(), m, "sys", "需求", "")
	var failure *llmcontract.Failure
	if !errors.As(err, &failure) || failure.Kind != llmcontract.FailureContract {
		t.Fatalf("自愈上限耗尽应返回契约失败，得 %T %v", err, err)
	}
	if got := atomic.LoadInt64(&m.idx); got != 4 {
		t.Fatalf("首次调用 + 3 次重问共 4 次调用，不应有第 5 次，got %d", got)
	}
}

func TestDecide_RetryableModelErrorReportsSharedProgress(t *testing.T) {
	m := &failingThenValidModel{failures: 8}
	var progress []agentcore.ProgressPayload
	ctx := agentcore.WithToolProgress(context.Background(), func(p agentcore.ProgressPayload) {
		progress = append(progress, p)
	})

	if _, err := DecidePlanStart(ctx, m, "sys", "需求", ""); err != nil {
		t.Fatalf("decide: %v", err)
	}
	if got := atomic.LoadInt64(&m.calls); got != 9 {
		t.Fatalf("model calls = %d, want 9", got)
	}
	if len(progress) != 8 || progress[7].Kind != agentcore.ProgressRetry || progress[7].Agent != "arbiter" || progress[7].Attempt != 8 || progress[7].MaxRetries != 0 {
		t.Fatalf("progress = %+v", progress)
	}
}

func TestDecide_NonRetryableModelErrorFailsImmediately(t *testing.T) {
	m := &errorModel{err: retryableTestError{retryable: false}}
	if _, err := DecidePlanStart(context.Background(), m, "sys", "需求", ""); err == nil {
		t.Fatal("non-retryable model error should fail")
	}
	if got := atomic.LoadInt64(&m.calls); got != 1 {
		t.Fatalf("model calls = %d, want 1", got)
	}
}

type errorModel struct {
	err   error
	calls int64
}

func (m *errorModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	atomic.AddInt64(&m.calls, 1)
	return nil, m.err
}

func (m *errorModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	return nil, m.err
}

func (m *errorModel) SupportsTools() bool { return true }

func TestInterventionDecision_ValidateAgainst(t *testing.T) {
	writing := InterventionFacts{Phase: string(domain.PhaseWriting), CompletedChapters: 10}
	// ask qua kiểm tra cấu trúc bất kể cờ; ask_user tắt thì DecideIntervention tự chọn.
	writingAsk := writing
	writingAsk.AskUserEnabled = true
	completeAsk := InterventionFacts{Phase: string(domain.PhaseComplete), CompletedChapters: 10, AskUserEnabled: true}
	complete := InterventionFacts{Phase: string(domain.PhaseComplete), CompletedChapters: 10}

	cases := []struct {
		name    string
		d       InterventionDecision
		f       InterventionFacts
		wantErr bool
	}{
		{"空决策", InterventionDecision{Reason: "r"}, writing, true},
		{"缺 reason", InterventionDecision{Answer: "好的"}, writing, true},
		{"查询类", InterventionDecision{Answer: "已完成 10 章", Reason: "查询"}, writing, false},
		{"返工组合", InterventionDecision{
			Hold:     &AdvanceHoldOp{After: domain.AdvanceHoldAfterRewritesDrained, Reason: "重写第3章语气"},
			Dispatch: &DispatchOp{Agent: "editor", Task: "重写第3章:语气改冷"},
			Reason:   "改已写章节",
		}, writing, false},
		{"非法派单目标", InterventionDecision{Dispatch: &DispatchOp{Agent: "coordinator", Task: "x"}, Reason: "r"}, writing, true},
		{"写作期 reopen", InterventionDecision{Reopen: &ReopenOp{Chapters: []int{3}}, Reason: "r"}, writing, true},
		{"完本期 reopen", InterventionDecision{Reopen: &ReopenOp{Chapters: []int{3}}, Reason: "返工"}, complete, false},
		{"完本期 reopen 越界", InterventionDecision{Reopen: &ReopenOp{Chapters: []int{99}}, Reason: "r"}, complete, true},
		{"完本期直接派单", InterventionDecision{Dispatch: &DispatchOp{Agent: "writer", Task: "x"}, Reason: "r"}, complete, true},
		{"规划期禁止 writer", InterventionDecision{Dispatch: &DispatchOp{Agent: "writer", Task: "写第1章"}, Reason: "r"}, InterventionFacts{Phase: string(domain.PhaseOutline)}, true},
		{"规划期允许 architect", InterventionDecision{Dispatch: &DispatchOp{Agent: "architect_long", Task: "补齐大纲"}, Reason: "r"}, InterventionFacts{Phase: string(domain.PhaseOutline)}, false},
		{"一次性暂停缺条件", InterventionDecision{Hold: &AdvanceHoldOp{Reason: "停"}, Reason: "r"}, writing, true},
		{"一次性暂停缺摘要", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtBoundary}, Reason: "r"}, writing, true},
		{"目标章节暂停", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtChapter, TargetChapter: 15, Reason: "写到第15章"}, Reason: "r"}, writing, false},
		{"目标章节未填写", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtChapter, Reason: "写到目标章"}, Reason: "r"}, writing, true},
		{"目标章节已经完成", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtChapter, TargetChapter: 10, Reason: "写到第10章"}, Reason: "r"}, writing, true},
		{"非目标暂停携带章节", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtBoundary, TargetChapter: 15, Reason: "停"}, Reason: "r"}, writing, true},
		{"取消一次性暂停", InterventionDecision{Hold: &AdvanceHoldOp{Cancel: true}, Answer: "继续", Reason: "r"}, writing, false},
		{"完本期设置一次性暂停", InterventionDecision{Hold: &AdvanceHoldOp{After: domain.AdvanceHoldAtBoundary, Reason: "停"}, Reason: "r"}, complete, true},
		// Gói P · P4 — ask：停下等用户回答，只有选项与章节，不附带其它动作。
		{"ask 合法", InterventionDecision{
			Ask:    &AskOp{Question: "Giữ hay cắt hai cảnh thừa ở chương 5?", Chapter: 5, Options: []AskOptionOp{{Label: "Giữ, dời dàn ý", Reply: "Giữ cả hai cảnh", IsDefault: true}, {Label: "Cắt cho khớp"}}},
			Reason: "cần người dùng chọn hướng đi",
		}, writingAsk, false},
		{"ask_user tắt vẫn qua kiểm tra (máy chủ tự chọn)", InterventionDecision{
			Ask:    &AskOp{Question: "Giữ hay cắt?", Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}},
			Reason: "hỏi người dùng",
		}, writing, false},
		{"ask 缺问题", InterventionDecision{Ask: &AskOp{Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}}, Reason: "r"}, writing, true},
		{"ask 选项不足两个", InterventionDecision{Ask: &AskOp{Question: "q", Options: []AskOptionOp{{Label: "A"}}}, Reason: "r"}, writing, true},
		{"ask 两个默认", InterventionDecision{Ask: &AskOp{Question: "q", Options: []AskOptionOp{{Label: "A", IsDefault: true}, {Label: "B", IsDefault: true}}}, Reason: "r"}, writing, true},
		{"ask 不许配派单", InterventionDecision{
			Ask:      &AskOp{Question: "q", Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}},
			Dispatch: &DispatchOp{Agent: "editor", Task: "x"}, Reason: "r"}, writing, true},
		{"完本期禁 ask", InterventionDecision{Ask: &AskOp{Question: "q", Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}}, Reason: "r"}, completeAsk, true},
		{"ask 不得覆盖在途提问", InterventionDecision{Ask: &AskOp{Question: "câu khác?", Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}}, Reason: "r"},
			InterventionFacts{Phase: string(domain.PhaseWriting), AskUserEnabled: true, PendingAsk: &domain.PendingAsk{Question: "câu đang chờ?"}}, true},
		{"ask 同一提问幂等", InterventionDecision{Ask: &AskOp{Question: "câu đang chờ?", Options: []AskOptionOp{{Label: "A"}, {Label: "B"}}}, Reason: "r"},
			InterventionFacts{Phase: string(domain.PhaseWriting), AskUserEnabled: true, PendingAsk: &domain.PendingAsk{Question: "câu đang chờ?"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.d.ValidateAgainst(tc.f)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got %v", tc.wantErr, err)
			}
		})
	}
}

func TestDecideInterventionAcceptsTargetChapterHold(t *testing.T) {
	m := &scriptedModel{outputs: []string{`{
		"answer":"将连续写到第15章后暂停",
		"rules":null,
		"hold":{"cancel":false,"after":"chapter","target_chapter":15,"reason":"写到第15章后暂停"},
		"reopen":null,
		"ask":null,
		"dispatch":null,
		"reason":"用户指定了一次性运行终点"
	}`}}
	d, _, err := DecideIntervention(t.Context(), m, "sys", InterventionFacts{
		Phase: string(domain.PhaseWriting), CompletedChapters: 10, NextChapter: 11,
	}, "写到第15章")
	if err != nil {
		t.Fatal(err)
	}
	if d.Hold == nil || d.Hold.After != domain.AdvanceHoldAtChapter || d.Hold.TargetChapter != 15 {
		t.Fatalf("目标章节 hold 解码错误: %+v", d.Hold)
	}
}

func TestDecideInterventionAcceptsAsk(t *testing.T) {
	m := &scriptedModel{outputs: []string{`{
		"answer":"Chương 5 dài hơn dàn ý hai cảnh, tôi cần bạn chọn hướng.",
		"rules":null,
		"hold":null,
		"reopen":null,
		"ask":{"question":"Chương 5 dài hơn dàn ý hai cảnh (cảnh bếp và cuộc gọi cho luật sư Hàn). Giữ nguyên rồi dời dàn ý, hay cắt hai cảnh cho khớp?",
			"options":[{"label":"Giữ nguyên, dời dàn ý","reply":"Giữ cảnh bếp và cuộc gọi, dời dàn ý sang các chương sau","is_default":true},
				{"label":"Cắt cho khớp dàn ý","reply":null,"is_default":false}],
			"chapter":5},
		"dispatch":null,
		"reason":"chương viết vượt dàn ý, cần người dùng phán quyết"
	}`}}
	d, _, err := DecideIntervention(t.Context(), m, "sys", InterventionFacts{
		Phase: string(domain.PhaseWriting), CompletedChapters: 5, NextChapter: 6, AskUserEnabled: true,
	}, "chương 5 có vẻ dài hơn dàn ý")
	if err != nil {
		t.Fatal(err)
	}
	if d.Ask == nil || d.Ask.Chapter != 5 || len(d.Ask.Options) != 2 {
		t.Fatalf("ask 解码错误: %+v", d.Ask)
	}
	if !d.Ask.Options[0].IsDefault || d.Ask.Options[0].Reply == "" {
		t.Fatalf("ask 默认项/回复文本缺失: %+v", d.Ask.Options)
	}
}

// Gói P · 27/10 — ask_user tắt: máy chủ từ chối ask, tự chọn phương án
// mặc định (hướng bám dàn ý do Trọng tài đề xuất) rồi phán định lại một lần
// qua chính kênh can thiệp; thẻ “Đang chờ bạn” không bao giờ xuất hiện.
func TestDecideInterventionAutoPicksAskWhenDisabled(t *testing.T) {
	m := &scriptedModel{outputs: []string{`{
		"answer":"Chương 5 dài hơn dàn ý hai cảnh.",
		"rules":null,
		"hold":null,
		"reopen":null,
		"ask":{"question":"Giữ hai cảnh rồi dời dàn ý, hay cắt cho khớp?",
			"options":[{"label":"Cắt cho khớp dàn ý","reply":"Cắt hai cảnh thừa, giữ đại cương nguyên văn","is_default":true},
				{"label":"Giữ nguyên, dời dàn ý","reply":null,"is_default":false}],
			"chapter":5},
		"dispatch":null,
		"reason":"cần người dùng phán quyết"
	}`, `{
		"answer":"Đã ghi nhận: cắt hai cảnh thừa ở chương 5.",
		"rules":null,
		"hold":null,
		"reopen":null,
		"ask":null,
		"dispatch":{"agent":"editor","task":"Cắt hai cảnh thừa ở chương 5, giữ nguyên đại cương"},
		"reason":"triển khai lựa chọn bám dàn ý"
	}`}}
	d, picks, err := DecideIntervention(t.Context(), m, "sys", InterventionFacts{
		Phase: string(domain.PhaseWriting), CompletedChapters: 5, NextChapter: 6,
	}, "chương 5 có vẻ dài hơn dàn ý")
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 {
		t.Fatalf("phải có đúng một lần tự chọn: %+v", picks)
	}
	if picks[0].Choice != "Cắt hai cảnh thừa, giữ đại cương nguyên văn" {
		t.Fatalf("phải chọn reply của phương án mặc định, nhận: %+v", picks[0])
	}
	if d.Ask != nil || d.Dispatch == nil || d.Dispatch.Agent != "editor" {
		t.Fatalf("ask phải bị từ chối và đi tiếp bằng phán định mới: %+v", d)
	}
	if d.Answer != "Đã ghi nhận: cắt hai cảnh thừa ở chương 5." {
		t.Fatalf("answer của lượt sau phải được giữ: %+v", d.Answer)
	}
	// Lượt phán định lại phải nhận đúng reply đã chọn làm văn bản can thiệp.
	var payload struct {
		Intervention string `json:"intervention"`
	}
	raw := m.lastMsgs[len(m.lastMsgs)-1].TextContent()
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload.Intervention != "Cắt hai cảnh thừa, giữ đại cương nguyên văn" {
		t.Fatalf("lượt sau phải phán định trên reply đã chọn: %s", raw)
	}
}

func TestDecideInterventionAutoPickCap(t *testing.T) {
	ask := `{"answer":"","rules":null,"hold":null,"reopen":null,
		"ask":{"question":"Giữ hay cắt?","options":[{"label":"Cắt","reply":"cắt","is_default":true},{"label":"Giữ","reply":"giữ","is_default":false}],"chapter":5},
		"dispatch":null,"reason":"r"}`
	m := &scriptedModel{outputs: []string{ask, ask, ask}}
	_, picks, err := DecideIntervention(t.Context(), m, "sys", InterventionFacts{
		Phase: string(domain.PhaseWriting), CompletedChapters: 5, NextChapter: 6,
	}, "lệch dàn ý")
	if err == nil {
		t.Fatal("model cứ đòi hỏi phải bị từ chối sau khi chạm trần tự chọn")
	}
	if len(picks) != maxAskAutoPicks {
		t.Fatalf("autoPicks phải dừng ở trần %d, nhận %d", maxAskAutoPicks, len(picks))
	}
}

func TestFailureDecision_Validate(t *testing.T) {
	facts := FailureFacts{Kind: "worker_failure", Phase: string(domain.PhaseWriting)}
	ok := FailureDecision{Action: "reroute", Dispatch: &DispatchOp{Agent: "architect_long", Task: "先 expand_arc"}, Reason: "错误指明出路"}
	if err := ok.ValidateAgainst(facts); err != nil {
		t.Fatalf("合法 reroute 被拒: %v", err)
	}
	bad := FailureDecision{Action: "reroute", Reason: "r"}
	if err := bad.ValidateAgainst(facts); err == nil {
		t.Fatal("reroute 无 dispatch 应被拒")
	}
	if err := (&FailureDecision{Action: "escalate", Reason: "r"}).ValidateAgainst(facts); err == nil {
		t.Fatal("非法 action 应被拒")
	}
	planning := FailureFacts{Kind: "worker_failure", Phase: string(domain.PhaseOutline)}
	writer := FailureDecision{Action: "reroute", Dispatch: &DispatchOp{Agent: "writer", Task: "写第1章"}, Reason: "尝试绕过规划"}
	if err := writer.ValidateAgainst(planning); err == nil {
		t.Fatal("失败裁定不得在规划期派发 writer")
	}
}

func TestCollectInterventionFacts(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(30); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.Book.Save(domain.BookMetadata{Title: "测试书", Synopsis: "测试简介"}); err != nil {
		t.Fatalf("book: %v", err)
	}
	if err := st.RunMeta.Init("default", "openrouter", "m"); err != nil {
		t.Fatalf("run meta: %v", err)
	}
	if err := st.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
		t.Fatalf("advance mode: %v", err)
	}
	if err := st.RunMeta.SetAdvanceHold(domain.AdvanceHold{After: domain.AdvanceHoldAtChapter, TargetChapter: 20, Reason: "写到第20章"}); err != nil {
		t.Fatalf("advance hold: %v", err)
	}
	if err := st.RunMeta.SetPendingAsk(domain.PendingAsk{Question: "Giữ hay cắt?", Chapter: 5}); err != nil {
		t.Fatalf("pending ask: %v", err)
	}

	if _, err := st.Decisions.Append(storepkg.DecisionRecord{Kind: "intervention", Decider: "arbiter", Input: "上次干预", Reason: "已入队"}); err != nil {
		t.Fatalf("append decision: %v", err)
	}

	f, err := CollectInterventionFacts(st)
	if err != nil {
		t.Fatalf("CollectInterventionFacts: %v", err)
	}
	if f.Title != "测试书" {
		t.Fatalf("facts 应含书名, got %+v", f)
	}
	if len(f.RecentDecisions) != 1 || f.RecentDecisions[0].Input != "上次干预" {
		t.Fatalf("干预记忆缺失: %+v", f.RecentDecisions)
	}
	if f.AdvanceMode != string(domain.ChapterAdvanceReview) || !f.HasAdvanceHold || f.AdvanceHoldAfter != string(domain.AdvanceHoldAtChapter) || f.AdvanceHoldTargetChapter != 20 {
		t.Fatalf("推进控制事实缺失: %+v", f)
	}
	if len(f.FoundationMissing) == 0 {
		t.Fatal("新书应有基础设定缺项")
	}
	// Gói P · P4 — 在途提问是干预事实：Arbiter 必须看到正等待回答的问题。
	if f.PendingAsk == nil || f.PendingAsk.Question != "Giữ hay cắt?" || f.PendingAsk.Chapter != 5 {
		t.Fatalf("在途提问事实缺失: %+v", f.PendingAsk)
	}

	// /reopen 是可枚举事实，必须进 facts：重开后的书章数已写满，缺了它模型会
	// 据 completed=total 推断"已完结"、无视 phase=writing（实测事故）。
	if err := st.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase: %v", err)
	}
	if err := st.Progress.MarkComplete(); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := st.Progress.ReopenContinue(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	f, err = CollectInterventionFacts(st)
	if err != nil {
		t.Fatalf("CollectInterventionFacts after reopen: %v", err)
	}
	if f.ReopenCount != 1 || f.Phase != string(domain.PhaseWriting) {
		t.Fatalf("重开事实缺失: phase=%s reopen_count=%d", f.Phase, f.ReopenCount)
	}
}

// 干预记忆要带回 Arbiter 自己的回答与动作，并数出末尾连续"只说不做"的轮数：
// 失败裁定（无 decision）既不计数也不打断。
func TestCollectInterventionFactsConversationMemory(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := st.Progress.Init(10); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := st.RunMeta.Init("default", "openrouter", "m"); err != nil {
		t.Fatalf("run meta: %v", err)
	}
	appendDecision := func(input string, d *InterventionDecision, errText string) {
		t.Helper()
		rec := storepkg.DecisionRecord{Kind: "intervention", Decider: "arbiter", Input: input, Error: errText}
		if d != nil {
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			rec.Decision = data
			rec.Reason = d.Reason
		}
		if _, err := st.Decisions.Append(rec); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	appendDecision("Rà soát 8 chương đầu", &InterventionDecision{Dispatch: &DispatchOp{Agent: "editor", Task: "Rà soát diễn đạt chương 1–8"}, Reason: "giao việc"}, "")
	scope := strings.Repeat("Mô tả lỗi trình bày. ", 20) + "Chỉ sửa chương 7–8, không sửa chương khác."
	reply := strings.Repeat("Tôi hiểu lỗi trình bày. ", 25) + "Chỉ bỏ nhãn, không viết lại tình tiết."
	appendDecision(scope, &InterventionDecision{Answer: reply, Reason: "hỏi đáp"}, "")
	appendDecision("NỘI./NGOẠI. …", nil, "arbiter: context canceled")
	appendDecision("Rồi xử lí ngay đi", &InterventionDecision{Answer: "Anh muốn bỏ nhãn ở chương nào?", Reason: "hỏi lại"}, "")

	f, err := CollectInterventionFacts(st)
	if err != nil {
		t.Fatalf("CollectInterventionFacts: %v", err)
	}
	if f.NoActionStreak != 2 {
		t.Fatalf("no_action_streak = %d want 2", f.NoActionStreak)
	}
	if len(f.RecentDecisions) != 4 {
		t.Fatalf("recent = %+v", f.RecentDecisions)
	}
	if got := f.RecentDecisions[0].PlannedAction; !strings.Contains(got, "giao editor") {
		t.Fatalf("thiếu action của lượt giao việc: %q", got)
	}
	if got := f.RecentDecisions[3].Answer; got != "Anh muốn bỏ nhãn ở chương nào?" {
		t.Fatalf("thiếu answer của lượt hỏi lại: %q", got)
	}
	if f.RecentDecisions[1].Input != scope || f.RecentDecisions[1].Answer != reply {
		t.Fatal("ủy quyền hoặc giới hạn cuối câu bị cắt mất")
	}
	if f.RecentDecisions[2].Answer != "" || f.RecentDecisions[2].PlannedAction != "" {
		t.Fatal("phán định thất bại không được coi là câu trả lời hay hành động")
	}
	appendDecision(strings.Repeat("x", 9000)+" chỉ sửa chương 7–8", &InterventionDecision{Answer: "Cần xác nhận phạm vi."}, "")
	f, err = CollectInterventionFacts(st)
	if err != nil {
		t.Fatal(err)
	}
	if !f.RecentDecisions[len(f.RecentDecisions)-1].InputTruncated {
		t.Fatal("mất cờ báo lịch sử bị cắt, có thể mất giới hạn ủy quyền")
	}
}

func TestCollectInterventionFactsDoesNotExposeLayeredEstimateAsTotal(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(66); err != nil {
		t.Fatal(err)
	}
	volumes := []domain.VolumeOutline{{
		Index: 1, Title: "卷一", Arcs: []domain.ArcOutline{
			{Index: 1, Title: "当前弧", Chapters: []domain.OutlineEntry{{Title: "一"}, {Title: "二"}}},
			{Index: 2, Title: "骨架弧", EstimatedChapters: 64},
		},
	}}
	if err := st.Outline.SaveLayeredOutline(volumes); err != nil {
		t.Fatal(err)
	}
	p, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.Layered = true
	if err := st.Progress.Save(p); err != nil {
		t.Fatal(err)
	}

	facts, err := CollectInterventionFacts(st)
	if err != nil {
		t.Fatal(err)
	}
	if !facts.DynamicPlanning || facts.OutlinedChapters != 2 {
		t.Fatalf("动态规划事实错误: %+v", facts)
	}
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "total_chapters") || strings.Contains(string(raw), `:66`) {
		t.Fatalf("内部估算不得作为总章数进入 Arbiter: %s", raw)
	}
}

func TestExtractJSON(t *testing.T) {
	cases := map[string]string{
		`{"a":1}`:                        `{"a":1}`,
		"前缀 ```json\n{\"a\":\"}\"}\n```": `{"a":"}"}`, // 字符串里的花括号不干扰平衡
		"没有对象":                           "",
		`{"nested":{"b":2},"c":3} 尾巴`:    `{"nested":{"b":2},"c":3}`,
	}
	for in, want := range cases {
		if got := llmcontract.ExtractJSONObject(in); got != want {
			t.Errorf("extractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

// nativeModel 声明支持原生 JSON Schema 的模型:decide 应走 native 分支。
type nativeModel struct {
	*scriptedModel
	stop llmtypes.StopReason
}

func (m *nativeModel) Capabilities() llmtypes.Capabilities {
	return llmtypes.Capabilities{
		Provider:   "openai",
		Model:      "gpt-test",
		Structured: llmtypes.StructuredCapabilities{JSONSchema: llmtypes.SupportYes, Strict: llmtypes.SupportYes},
	}
}

func (m *nativeModel) Generate(ctx context.Context, msgs []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	resp, err := m.scriptedModel.Generate(ctx, msgs, tools, opts...)
	if resp != nil && m.stop != "" {
		resp.Message.StopReason = m.stop
	}
	return resp, err
}

// 契约测试(RFC §11.1):根为 object、全属性(含嵌套)required、dispatch 为可空对象。
func TestContractSchemasAreStrictReady(t *testing.T) {
	for _, c := range []llmcontract.Contract{planStartContract, failureContract, interventionContract} {
		if c.Schema["type"] != "object" {
			t.Fatalf("%s: 根必须是 object", c.Name)
		}
		if err := llmcontract.ValidateStrictReady(c.Schema); err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if len(c.Fingerprint()) != 12 {
			t.Fatalf("%s: fingerprint 异常: %q", c.Name, c.Fingerprint())
		}
	}
	dispatch := failureContract.Schema["properties"].(map[string]any)["dispatch"].(map[string]any)
	types, ok := dispatch["type"].([]string)
	if !ok || !slices.Contains(types, "null") || !slices.Contains(types, "object") {
		t.Fatalf("dispatch 应为可空对象: %v", dispatch["type"])
	}
	var d FailureDecision
	if err := json.Unmarshal([]byte(`{"action":"retry","dispatch":null,"reason":"瞬时错误"}`), &d); err != nil {
		t.Fatalf("含 dispatch:null 的样本应可解码: %v", err)
	}
	if err := d.ValidateAgainst(FailureFacts{Phase: "writing"}); err != nil {
		t.Fatalf("样本应过校验: %v", err)
	}
}

func TestDecideNativeSendsSchemaAndDecodesFullOutput(t *testing.T) {
	m := &nativeModel{scriptedModel: &scriptedModel{outputs: []string{
		`{"planner":"architect_short","task":"规划短篇","reason":"篇幅较短"}`,
	}}}
	const semanticPrompt = "只根据需求判断规划方式。"
	d, err := DecidePlanStart(t.Context(), m, semanticPrompt, "写一部短篇", "")
	if err != nil || d.Planner != "architect_short" {
		t.Fatalf("native 裁定失败: %+v %v", d, err)
	}
	rf := m.lastCfg.ResponseFormat
	if rf == nil || rf.Type != llmtypes.ResponseFormatJSONSchema || rf.JSONSchema == nil {
		t.Fatalf("native 模式应发送 response_format: %+v", rf)
	}
	if rf.JSONSchema.Name != "arbiter_plan_start" || rf.JSONSchema.Strict == nil || !*rf.JSONSchema.Strict {
		t.Fatalf("schema 参数不符: %+v", rf.JSONSchema)
	}
	if got := m.lastMsgs[0].TextContent(); got != semanticPrompt {
		t.Fatalf("native 模式不应向提示词重复注入 schema:\n%s", got)
	}
}

// native 模式下解码失败=provider 契约违约:立即报错,不走 extractJSON 兜底、不重问。
func TestDecideNativeFencedOutputIsContractViolation(t *testing.T) {
	m := &nativeModel{scriptedModel: &scriptedModel{outputs: []string{
		"```json\n{\"planner\":\"architect_short\",\"task\":\"x\",\"reason\":\"y\"}\n```",
	}}}
	_, err := DecidePlanStart(t.Context(), m, "sys", "写一部短篇", "")
	if err == nil || !strings.Contains(err.Error(), "vi phạm hợp đồng") {
		t.Fatalf("期望契约违约错误, got %v", err)
	}
	if m.idx != 1 {
		t.Fatalf("契约违约不应重问, 调用了 %d 次", m.idx)
	}
}

// native 模式业务校验失败仍反馈重问,且重问请求保留 schema。
func TestDecideNativeValidateFailureFeedbackKeepsSchema(t *testing.T) {
	m := &nativeModel{scriptedModel: &scriptedModel{outputs: []string{
		`{"action":"reroute","dispatch":null,"reason":"需要换路"}`,
		`{"action":"retry","dispatch":null,"reason":"瞬时错误可重试"}`,
	}}}
	d, err := DecideFailure(t.Context(), m, "sys", FailureFacts{Kind: "worker_failure", Phase: "writing"})
	if err != nil || d.Action != "retry" {
		t.Fatalf("反馈重问后应成功: %+v %v", d, err)
	}
	if m.idx != 2 {
		t.Fatalf("应恰好两次调用, got %d", m.idx)
	}
	if m.lastCfg.ResponseFormat == nil {
		t.Fatal("重问请求丢失了 schema")
	}
}

// native 模式先分类终止原因:截断/拒答/空响应是独立错误事实,不进重问循环。
func TestDecideNativeStopReasonClassification(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		stop    llmtypes.StopReason
		wantErr string
	}{
		{"length 截断", `{"planner":`, llmtypes.StopReasonLength, "cắt vì độ dài"},
		{"safety 拒答", `无法协助`, llmtypes.StopReasonSafety, "từ chối trả lời"},
		{"空响应", ``, llmtypes.StopReasonStop, "nội dung rỗng"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &nativeModel{scriptedModel: &scriptedModel{outputs: []string{tc.output}}, stop: tc.stop}
			_, err := DecidePlanStart(t.Context(), m, "sys", "写一部短篇", "")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望 %q 错误, got %v", tc.wantErr, err)
			}
			if m.idx != 1 {
				t.Fatalf("终止原因不应重问, 调用了 %d 次", m.idx)
			}
		})
	}
}

// marshalPayload 失败必须暴露:静默伪造 {} 会让模型基于假事实误判。
func TestMarshalPayloadErrors(t *testing.T) {
	if _, err := marshalPayload(func() {}); err == nil {
		t.Fatal("不可序列化载荷应报错")
	}
	s, err := marshalPayload(map[string]int{"a": 1})
	if err != nil || !strings.Contains(s, `"a"`) {
		t.Fatalf("正常载荷: %q %v", s, err)
	}
}
