package agents

import (
	"errors"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
	"github.com/datnt1998/verse/core/agentcore/schemalint"
	llmtypes "github.com/datnt1998/verse/core/llm"
)

// TestWorkerToolSchemasLint: duyệt mọi tool mà BuildWorkers đăng ký cho từng
// worker (lấy từ runner, không duy trì danh sách thủ công) và bắt schema mà
// validator lúc gọi hoặc chế độ strict của provider sẽ từ chối — đúng các lỗi
// từng đốt lượt model (save_foundation thiếu axes, property không required
// trên tool strict, items/properties không phải object schema, enum lệch
// type...).
func TestWorkerToolSchemasLint(t *testing.T) {
	model := &contractModel{fn: func(_ int, _ []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		return nil, errors.New("lint test never calls the model")
	}}
	s := store.NewStore(t.TempDir())
	models := &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "test", model, nil)}
	bundle := assets.Load("default", assets.LoadOptions{})
	if bundle.Err != nil {
		t.Fatal(bundle.Err)
	}
	runner, _, _ := BuildWorkers(bootstrap.Config{}, s, tools.NewStyleStatsIndex(s), models, bundle, nil, nil)

	var report strings.Builder
	total := 0
	for _, name := range []string{"architect_short", "architect_long", "writer", "editor"} {
		cfg, ok := runner.AgentConfig(name)
		if !ok {
			t.Fatalf("worker %s không có trong runner", name)
		}
		if len(cfg.Tools) == 0 {
			t.Fatalf("worker %s không đăng ký tool nào", name)
		}
		for _, tool := range cfg.Tools {
			for _, issue := range schemalint.CheckTool(tool) {
				total++
				report.WriteString("\n  ")
				report.WriteString(name)
				report.WriteString(" / ")
				report.WriteString(issue.String())
			}
		}
	}
	if total > 0 {
		t.Fatalf("có %d vấn đề schema tool:%s", total, report.String())
	}
}
