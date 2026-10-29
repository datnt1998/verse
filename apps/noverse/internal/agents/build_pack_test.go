package agents

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/assets"
	corecontext "github.com/datnt1998/noverse/internal/agent/context"
	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

// newPackTestWorkers chạy BuildWorkers đúng đường dây production (provider giả
// chỉ tạo client, không gọi mạng) rồi trả runner để đọc lại SystemPrompt đã ráp.
func newPackTestWorkers(t *testing.T, cfg bootstrap.Config, bundle assets.Bundle) map[string]string {
	t.Helper()
	cfg.Provider, cfg.ModelName = "proxy", "m"
	if cfg.Providers == nil {
		cfg.Providers = map[string]bootstrap.ProviderConfig{"proxy": {
			Type: "openai", APIKey: "k", BaseURL: "https://example.com/v1",
			Models: []bootstrap.ModelConfig{{Name: "m", ContextWindow: 200000}},
		}}
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		t.Fatalf("NewModelSet: %v", err)
	}
	st := store.NewStore(t.TempDir())
	runner, _, _ := BuildWorkers(cfg, st, tools.NewStyleStatsIndex(st), models, bundle, nil, nil)

	prompts := make(map[string]string, 4)
	for _, name := range []string{"architect_short", "architect_long", "writer", "editor"} {
		c, ok := runner.AgentConfig(name)
		if !ok {
			t.Fatalf("runner thiếu agent %s", name)
		}
		prompts[name] = c.SystemPrompt
	}
	return prompts
}

func writeTestPackDir(t *testing.T, dir, manifest string, luat map[string]string, vanPhong string) {
	t.Helper()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("goi.json", manifest)
	for name, content := range luat {
		write(filepath.Join("luat", name), content)
	}
	if vanPhong != "" {
		write("van-phong.md", vanPhong)
	}
}

// TestPackRules_ReachAllThreeRoles (nghiệm thu P0-3): câu đánh dấu đặt trong
// luat/ của gói phải xuất hiện nguyên văn trong system prompt của writer,
// editor và cả hai biến thể architect, qua chính hàm ráp của runtime.
func TestPackRules_ReachAllThreeRoles(t *testing.T) {
	sentinel := "Câu đánh dấu gói ZXQW-731 phải lọt nguyên văn vào ba vai."
	home := t.TempDir()
	writeTestPackDir(t, filepath.Join(home, "packs", "goi-sen"),
		`{"ten":"Sentinel","luat":["luat-a.md","luat-b.md"]}`,
		map[string]string{"luat-a.md": "Luật đầu tiên.", "luat-b.md": sentinel},
		"văn phong của gói")

	cfg := bootstrap.Config{Style: "goi-sen"}
	bundle := assets.Load("goi-sen", assets.LoadOptions{HomeStyleDir: home})
	if bundle.Err != nil {
		t.Fatalf("nạp gói lỗi: %v", bundle.Err)
	}
	prompts := newPackTestWorkers(t, cfg, bundle)

	for _, name := range []string{"architect_short", "architect_long", "writer", "editor"} {
		if !containsAll(prompts[name], sentinel) {
			t.Fatalf("prompt %s thiếu câu đánh dấu của gói", name)
		}
		if !containsAll(prompts[name], "Luật gói thể loại", "[LỖI]") {
			t.Fatalf("prompt %s thiếu heading và ghi chú ưu tiên", name)
		}
	}
	// Writer nhận van-phong.md của gói thay cho styles/<tên>.md.
	if !containsAll(prompts["writer"], "văn phong của gói") {
		t.Fatal("writer phải nhận van-phong.md của gói")
	}
}

// TestPackVoice_VoiceSectionsRemoved (nghiệm thu P0-6): với gói A, prompt
// writer không còn hai câu ở voice.md mà Master §7/§15.1 thay thế.
func TestPackVoice_VoiceSectionsRemoved(t *testing.T) {
	cfg := bootstrap.Config{Style: "nu-tan-bao-thu"}
	bundle := assets.Load("nu-tan-bao-thu", assets.LoadOptions{})
	if bundle.Err != nil {
		t.Fatalf("nạp gói lỗi: %v", bundle.Err)
	}
	prompts := newPackTestWorkers(t, cfg, bundle)

	for _, banned := range []string{
		"không nhất thiết mỗi chương đều phải làm hồi hộp",
		"tránh xé nhỏ văn bản thành các đoạn vụn",
	} {
		if containsAll(prompts["writer"], banned) {
			t.Fatalf("prompt writer của gói A không được còn %q", banned)
		}
	}
	// Các vai khác cũng nhận luật nguyên văn: mục Master §7 có mặt ở cả ba.
	if !containsAll(prompts["editor"], "## 7.") {
		t.Fatal("editor phải nhận master-rules-v3.md nguyên văn")
	}
}

// TestNoPack_NoRulesSection: không có gói thì prompt giữ nguyên, không có khối
// luật gói được nối thêm.
func TestNoPack_NoRulesSection(t *testing.T) {
	cfg := bootstrap.Config{Style: "default"}
	bundle := assets.Load("default", assets.LoadOptions{})
	prompts := newPackTestWorkers(t, cfg, bundle)
	for name, p := range prompts {
		if containsAll(p, "## Luật gói thể loại (nạp nguyên văn)") {
			t.Fatalf("prompt %s không có gói thì không được có khối luật", name)
		}
	}
}

// TestPackSystemPromptTokenEstimate đo số token system prompt của từng vai cho
// style `default` và gói `nu-tan-bao-thu` (Quyết định 2: PR phải báo số đo).
func TestPackSystemPromptTokenEstimate(t *testing.T) {
	agents4 := []string{"architect_short", "architect_long", "writer", "editor"}
	estimates := make(map[string]map[string]int, 2)
	for _, style := range []string{"default", "nu-tan-bao-thu"} {
		cfg := bootstrap.Config{Style: style}
		bundle := assets.Load(style, assets.LoadOptions{})
		if bundle.Err != nil {
			t.Fatalf("nạp gói lỗi: %v", bundle.Err)
		}
		prompts := newPackTestWorkers(t, cfg, bundle)
		estimates[style] = make(map[string]int, len(agents4))
		for _, name := range agents4 {
			tokens := corecontext.EstimateTokens(llmtypes.SystemMsg(prompts[name]))
			estimates[style][name] = tokens
			t.Logf("style=%s agent=%s system_prompt_tokens=%d", style, name, tokens)
		}
	}
	for _, name := range agents4 {
		if estimates["nu-tan-bao-thu"][name] <= estimates["default"][name] {
			t.Fatalf("agent %s: nạp gói phải làm prompt lớn hơn (%d ≤ %d)",
				name, estimates["nu-tan-bao-thu"][name], estimates["default"][name])
		}
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
