package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// Bug 02/11: một truyện ghim provider/model riêng trong .ainovel/config.json của
// nó. Khi gói thuê bao của provider đó hết hạn, người dùng đổi provider ở cài đặt
// chung nhưng truyện vẫn chạy provider cũ. Từ nay provider/model/mức suy luận chỉ
// có hiệu lực ở lớp toàn cục; các lớp gần vẫn giữ ngân sách/quy tắc/văn phong.
func TestLoadConfigChain_StoryProviderPinIsIgnored(t *testing.T) {
	writeGlobal(t, `{
  "provider": "openrouter",
  "model": "google/gemini-2.5-flash",
  "reasoning_effort": "high",
  "providers": {"openrouter": {"api_key": "sk-test-123456", "models": [{"name": "google/gemini-2.5-flash"}]}},
  "roles": {"writer": {"provider": "openrouter", "model": "google/gemini-2.5-flash", "reasoning_effort": "high"}}
}`)
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	story := filepath.Join(project, "kich_ban", "01-truyen")
	if err := os.MkdirAll(filepath.Join(story, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{"name":"p"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pinned := `{
  "provider": "xai-oauth",
  "model": "grok-4.7",
  "reasoning_effort": "low",
  "providers": {"xai-oauth": {"models": [{"name": "grok-4.7"}]}},
  "roles": {
    "architect": {"provider": "xai-oauth", "model": "grok-4.7", "reasoning_effort": "low"},
    "writer": {"provider": "xai-oauth", "model": "grok-4.7", "reasoning_effort": "low"}
  },
  "style": "romance"
}`
	if err := os.WriteFile(filepath.Join(story, ".ainovel", "config.json"), []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}

	levels, cfg, err := LoadConfigChain(story)
	if err != nil {
		t.Fatalf("LoadConfigChain: %v", err)
	}
	if cfg.Provider != "openrouter" || cfg.ModelName != "google/gemini-2.5-flash" {
		t.Fatalf("provider/model phải dùng chung từ lớp toàn cục, được %q/%q", cfg.Provider, cfg.ModelName)
	}
	if cfg.ReasoningEffort != "high" {
		t.Fatalf("mức suy luận phải dùng chung, được %q", cfg.ReasoningEffort)
	}
	if writer := cfg.Roles["writer"]; writer.Provider != "openrouter" || writer.Model != "google/gemini-2.5-flash" {
		t.Fatalf("model theo vai phải dùng chung, được %+v", writer)
	}
	if _, ok := cfg.Roles["architect"]; ok {
		t.Fatalf("vai chỉ khai ở lớp truyện không được lọt vào cấu hình hiệu lực: %+v", cfg.Roles["architect"])
	}
	if _, ok := cfg.Providers["xai-oauth"]; ok {
		t.Fatalf("provider chỉ khai ở lớp truyện không được lọt vào cấu hình hiệu lực")
	}
	if cfg.Style != "romance" {
		t.Fatalf("văn phong vẫn theo cấp truyện, được %q", cfg.Style)
	}

	// Cùng một quy tắc cho nội dung trả về của /api/config: lớp truyện không
	// hiện lại ghim cũ, để lần Lưu sau dọn nó khỏi tài liệu.
	for _, level := range levels {
		if level.Scope == "global" {
			continue
		}
		raw, err := ConfigLevelJSON(level)
		if err != nil {
			t.Fatalf("ConfigLevelJSON(%s): %v", level.Scope, err)
		}
		for _, key := range modelSelectionKeys {
			if _, ok := raw[key]; ok {
				t.Fatalf("lớp %s còn hiện khoá %q: %+v", level.Scope, key, raw)
			}
		}
		if level.Scope == "story" && raw["style"] != "romance" {
			t.Fatalf("lớp truyện phải giữ văn phong: %+v", raw)
		}
	}
}
