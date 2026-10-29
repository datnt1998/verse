package agents

// 端到端验证 save_review 硬停 + 任务感知 StopGuard 的组合行为（build.go editor
// 配置的真实接线：StopAfterToolResult 命中 save_review/save_*_summary、
// StopGuardFactory 用真 guard、工具落真 checkpoint）。
//
// 场景一（摘要任务先复核）：editor 被派生成弧摘要，却先调了 save_review——
// 硬停触发但 guard 否决，注入催促后 editor 走到 save_arc_summary 才真正退出。
// 这是恢复 save_review 硬停的安全前提，防止弧摘要永不落盘的死循环回归。
//
// 场景二（评审任务一步收尾）：editor 被派评审，save_review 落盘即硬停放行，
// 不再多跑一轮 LLM 收尾。

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/agents/guard"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore"
	"github.com/datnt1998/verse/core/agentcore/subagent"
)

// editorStopAfterToolResult 与 build.go 中 editor 的配置保持同一判据。
func editorStopAfterToolResult(toolName string, _ json.RawMessage) bool {
	return toolName == "save_review" || toolName == "save_arc_summary" || toolName == "save_volume_summary"
}

func checkpointTool(t *testing.T, st *store.Store, name, step string) agentcore.Tool {
	t.Helper()
	return agentcore.NewFuncTool(name, "fake "+name, map[string]any{"type": "object"},
		func(context.Context, json.RawMessage) (json.RawMessage, error) {
			if _, err := st.Checkpoints.Append(domain.ArcScope(1, 1), step, "artifact", "digest"); err != nil {
				t.Fatalf("append checkpoint %s: %v", step, err)
			}
			return json.RawMessage(`"saved"`), nil
		})
}

func runEditorLike(t *testing.T, st *store.Store, task string, model llmtypes.ChatModel, tools []agentcore.Tool) {
	t.Helper()
	cfg := subagent.Config{
		Name:                "editor",
		Description:         "test editor",
		Model:               model,
		SystemPrompt:        "test",
		Tools:               tools,
		MaxTurns:            10,
		StopAfterToolResult: editorStopAfterToolResult,
		StopGuardFactory: func(_, task string) agentcore.StopGuard {
			return guard.NewEditorStopGuard(st, task, nil)
		},
	}
	tool := subagent.NewRunner(cfg).AsTool()
	args, _ := json.Marshal(map[string]string{"agent": "editor", "task": task})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("subagent execute: %v", err)
	}
}

func TestEditorFlow_SummaryTaskSurvivesEarlyReview(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	var calls atomic.Int32
	model := &contractModel{fn: func(i int, _ []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		switch i {
		case 0:
			// 跑偏：摘要任务却先复核。
			return &llmtypes.LLMResponse{Message: assistantToolCall("save_review", `{}`)}, nil
		default:
			// guard 否决硬停并注入催促后，本轮才产出摘要。
			calls.Add(1)
			return &llmtypes.LLMResponse{Message: assistantToolCall("save_arc_summary", `{}`)}, nil
		}
	}}

	runEditorLike(t, st, "生成第 1 卷第 1 弧摘要（save_arc_summary）", model, []agentcore.Tool{
		checkpointTool(t, st, "save_review", "review"),
		checkpointTool(t, st, "save_arc_summary", "arc_summary"),
	})

	if calls.Load() == 0 {
		t.Fatal("save_review 硬停被 guard 否决后，editor 应继续走到 save_arc_summary——若 run 在复核后直接结束，说明终态退出绕过了 guard，弧摘要死循环会回归")
	}
	all := st.Checkpoints.All()
	var hasSummary bool
	for _, cp := range all {
		if cp.Step == "arc_summary" {
			hasSummary = true
		}
	}
	if !hasSummary {
		t.Fatal("弧摘要必须最终落盘")
	}
}

func TestEditorFlow_ReviewTaskStopsAtSaveReview(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	model := &contractModel{fn: func(i int, _ []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		if i == 0 {
			return &llmtypes.LLMResponse{Message: assistantToolCall("save_review", `{}`)}, nil
		}
		t.Fatal("评审任务 save_review 落盘后应硬停，模型不应获得额外轮次")
		return nil, nil
	}}

	runEditorLike(t, st, "对第 1 卷第 1 弧做弧级评审（scope=arc）", model, []agentcore.Tool{
		checkpointTool(t, st, "save_review", "review"),
	})

	if got := model.calls(); got != 1 {
		t.Fatalf("评审任务应恰好一次模型调用后收尾，got %d", got)
	}
}

// 用户干预派单（真实 host 拼装的权威段首 + 否定句"không tạo tóm tắt tập"）：
// 不得被当作摘要任务；每次 save_review 硬停后 guard 让 editor 继续逐章落盘，
// 模型自然结束时才放行。回归：曾经第 7 章落盘后被逼去写卷摘要，第 8 章从未入队。
func TestEditorFlow_InterventionSavesEveryChapterBeforeExit(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	model := &contractModel{fn: func(i int, msgs []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		switch i {
		case 0:
			return &llmtypes.LLMResponse{Message: assistantToolCall("save_review", `{"chapter":7}`)}, nil
		case 1:
			return &llmtypes.LLMResponse{Message: assistantToolCall("save_review", `{"chapter":8}`)}, nil
		default:
			return &llmtypes.LLMResponse{Message: assistantText("Đã đưa chương 7 và 8 vào hàng đợi.", llmtypes.StopReasonStop)}, nil
		}
	}}

	task := "Lưu save_review xóa nhãn CẢNH 7 ở chương 7 và CẢNH 8 ở chương 8. Không tạo tóm tắt tập, không rà soát thêm.\n\n" +
		guard.InterventionAuthorityHeader + "\nBỏ riêng chữ Cảnh 8 và Cảnh 7"
	saveReview := agentcore.NewFuncTool("save_review", "fake save_review", map[string]any{"type": "object"},
		func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
			if _, err := st.Checkpoints.Append(domain.GlobalScope(), "review", "reviews", string(args)); err != nil {
				t.Fatalf("append review checkpoint: %v", err)
			}
			return json.RawMessage(`"saved"`), nil
		})
	runEditorLike(t, st, task, model, []agentcore.Tool{
		saveReview,
		checkpointTool(t, st, "save_volume_summary", "volume_summary"),
	})

	var reviews int
	for _, cp := range st.Checkpoints.All() {
		switch cp.Step {
		case "review":
			reviews++
		case "volume_summary":
			t.Fatal("干预派单不得被误判为卷摘要任务")
		}
	}
	if reviews != 2 {
		t.Fatalf("两章都必须落盘，got %d 次 save_review", reviews)
	}
	if got := model.calls(); got != 3 {
		t.Fatalf("两次保存 + 一次自然收尾，应恰好 3 次模型调用，got %d", got)
	}
}
