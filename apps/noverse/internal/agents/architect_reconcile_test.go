package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

func TestArchitectReconcilesMidpointFromAcceptedProse(t *testing.T) {
	for _, name := range []string{"architect_short", "architect_long"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore(t.TempDir())
			mustReconcile(t, s.Init())
			mustReconcile(t, s.Progress.Init(28))
			entries := make([]domain.OutlineEntry, 28)
			for i := range entries {
				entries[i] = domain.OutlineEntry{Chapter: i + 1, Title: fmt.Sprintf("Chương %d", i+1), CoreEvent: "Đối chiếu chứng cứ", Hook: "Ai thao tác?", Scenes: []string{"Kiểm tra hồ sơ"}, HookType: "lat_thong_tin"}
			}
			if name == "architect_long" {
				mustReconcile(t, s.Progress.SetLayered(true))
				mustReconcile(t, s.Outline.SaveLayeredOutline([]domain.VolumeOutline{{Index: 1, Arcs: []domain.ArcOutline{{Index: 1, Chapters: entries[:16]}, {Index: 2, Chapters: entries[16:]}}}}))
			} else {
				mustReconcile(t, s.Outline.SaveOutline(entries))
			}
			for ch := 1; ch <= 20; ch++ {
				mustReconcile(t, s.Progress.MarkChapterComplete(ch, 100, "", ""))
			}
			mustReconcile(t, s.Drafts.SaveFinalChapter(16, "Ba nguồn xác nhận tác vụ ngoài lịch trước sự cố. Hàn yêu cầu bỏ nguyên nhân khỏi hồ sơ; thế cục chuyển sang tranh chấp chứng cứ, chưa xác nhận người thao tác."))
			mustReconcile(t, s.Drafts.SaveDraft(16, "Bản nháp khác, không phải bằng chứng nghiệm thu."))
			runArchitectMidpointReconciliation(t, s, name)
		})
	}
}

// Run only against an explicitly supplied disposable copy, never live storage.
func TestArchitectReconcilesMidpointLiveCopy(t *testing.T) {
	dir := os.Getenv("ARCHITECT_RECONCILE_COPY")
	if dir == "" {
		t.Skip("set ARCHITECT_RECONCILE_COPY to a disposable /tmp story copy")
	}
	if !strings.HasPrefix(dir, "/tmp/") {
		t.Fatal("live reproduction requires a disposable /tmp copy")
	}
	runArchitectMidpointReconciliation(t, store.NewStore(dir), "architect_long")
}

func mustReconcile(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func runArchitectMidpointReconciliation(t *testing.T, s *store.Store, name string) {
	t.Helper()
	ctx := context.Background()
	before := diag.Load(s)
	if len(diag.MidpointTwistPosition(&before)) != 1 {
		t.Fatal("fixture must start with the real §8 warning")
	}
	prose, err := s.Drafts.LoadChapterText(16)
	mustReconcile(t, err)
	if prose == "" {
		t.Fatal("missing accepted chapter 16 prose")
	}
	draft, err := s.Drafts.LoadDraft(16)
	mustReconcile(t, err)
	entry := before.Outline[15]
	entry.MidpointTwist = true
	revise := tools.NewReviseOutlineTool(s)
	args, err := llmcontract.EncodeArgs(revise.Schema(), map[string]any{"from_chapter": 16, "reconcile_completed": true, "replacement": []domain.OutlineEntry{entry}, "reason": "Đã đối chiếu chính văn: ba nguồn xác nhận tác vụ ngoài lịch; chuyển thế cục sang tranh chấp chứng cứ, không kết luận danh tính."})
	mustReconcile(t, err)
	model := &contractModel{fn: func(i int, msgs []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		switch i {
		case 0:
			return &llmtypes.LLMResponse{Message: assistantToolCall("novel_context", `{"chapter":16}`)}, nil
		case 1:
			return &llmtypes.LLMResponse{Message: assistantToolCall("read_chapter", `{"chapter":16,"source":"final"}`)}, nil
		case 2:
			last := msgs[len(msgs)-1]
			if last.Role != llmtypes.RoleTool {
				t.Fatalf("expected accepted-prose tool result, got %s", last.Role)
			}
			var got struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(last.TextContent()), &got); err != nil || got.Content != prose {
				t.Fatalf("architect did not receive accepted prose: %s", last.TextContent())
			}
			return &llmtypes.LLMResponse{Message: assistantToolCall("revise_outline", string(args))}, nil
		default:
			snap := diag.Load(s)
			if len(diag.MidpointTwistPosition(&snap)) != 0 {
				t.Fatalf("§8 still present after outline reconciliation: %s", msgs[len(msgs)-1].TextContent())
			}
			return &llmtypes.LLMResponse{Message: assistantText("Đã đối chiếu chính văn và đồng bộ nhãn cú lật, không sửa chính văn.", llmtypes.StopReasonStop)}, nil
		}
	}}
	models := &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "test", model, nil)}
	bundle := assets.Load("default", assets.LoadOptions{})
	mustReconcile(t, bundle.Err)
	runner, _, _ := BuildWorkers(bootstrap.Config{}, s, tools.NewStyleStatsIndex(s), models, bundle, nil, nil)
	cfg, _ := runner.AgentConfig(name)
	for _, tool := range cfg.Tools {
		switch tool.Name() {
		case "draft_chapter", "edit_chapter", "commit_chapter":
			t.Fatalf("architect gained prose write tool %s", tool.Name())
		case "read_chapter":
			readOnly, ok := tool.(agentcore.ReadOnlyTool)
			if !ok || !readOnly.ReadOnly(json.RawMessage(`{"chapter":16,"source":"final"}`)) {
				t.Fatal("architect prose read must be read-only")
			}
		}
	}
	_, err = runner.Run(ctx, name, "Đối chiếu chính văn Chương 16; chỉ đồng bộ nhãn cú lật §8 ở đại cương, không thêm sự kiện hay sửa chính văn.")
	mustReconcile(t, err)
	after := diag.Load(s)
	if len(diag.MidpointTwistPosition(&after)) != 0 {
		t.Fatal("§8 did not clear through real diagnostic reload")
	}
	want := before.Outline
	want[15].MidpointTwist = true
	if !reflect.DeepEqual(want, after.Outline) || !reflect.DeepEqual(before.Progress, after.Progress) {
		t.Fatal("reconciliation changed other outline fields or progress")
	}
	gotProse, err := s.Drafts.LoadChapterText(16)
	mustReconcile(t, err)
	gotDraft, err := s.Drafts.LoadDraft(16)
	mustReconcile(t, err)
	if gotProse != prose || gotDraft != draft {
		t.Fatal("architect changed accepted prose or draft")
	}
	if s.Checkpoints.LatestByStep(domain.GlobalScope(), "revise_outline") == nil {
		t.Fatal("missing outline reconciliation checkpoint")
	}
}
