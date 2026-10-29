package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// 27/10：ask_user giải quyết theo đúng chuỗi 3 cấp như advance_mode —
// Truyện (cwd) ghi đè Dự án, Dự án ghi đè Mọi truyện (~/.ainovel).
func TestAskUserThreeLevelChain(t *testing.T) {
	writeGlobal(t, `{"provider":"openrouter","model":"m","ask_user":true}`)
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	story := filepath.Join(project, "kich_ban", "01-truyen")
	if err := os.MkdirAll(story, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{"name":"p"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON := func(dir, content string) {
		t.Helper()
		d := filepath.Join(dir, ".ainovel")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "config.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON(project, `{"ask_user":false}`)
	writeJSON(story, `{"ask_user":true}`)

	_, cfg, err := LoadConfigChain(story)
	if err != nil {
		t.Fatalf("LoadConfigChain: %v", err)
	}
	if !cfg.ArbiterAskUser() {
		t.Fatal("truyện bật phải thắng dự án tắt")
	}

	// Truyện không khai báo → theo dự án (tắt thắng toàn cục bật).
	writeJSON(story, `{"style":"default"}`)
	_, cfg, err = LoadConfigChain(story)
	if err != nil {
		t.Fatalf("LoadConfigChain: %v", err)
	}
	if cfg.ArbiterAskUser() {
		t.Fatal("dự án tắt phải thắng toàn cục bật")
	}

	// Chỉ toàn cục khai báo → truyện kế thừa giá trị đó.
	writeJSON(project, `{"style":"default"}`)
	_, cfg, err = LoadConfigChain(story)
	if err != nil {
		t.Fatalf("LoadConfigChain: %v", err)
	}
	if !cfg.ArbiterAskUser() {
		t.Fatal("toàn cục bật phải truyền xuống truyện")
	}
}

func TestAskUserConfigLayering(t *testing.T) {
	// Mặc định: tắt (nil cũng là tắt).
	if (Config{}).ArbiterAskUser() {
		t.Fatal("ask_user mặc định phải tắt")
	}
	on, off := true, false
	if !(Config{AskUser: &on}).ArbiterAskUser() {
		t.Fatal("ask_user=true phải bật")
	}
	// Lớp ngoài bật, lớp trong không khai báo → giữ bật.
	merged := MergeConfig(Config{AskUser: &on}, Config{Style: "x"})
	if !merged.ArbiterAskUser() {
		t.Fatal("lớp trong không khai báo phải thừa hưởng ask_user=true")
	}
	// Lớp trong tắt được giá trị lớp ngoài.
	merged = MergeConfig(Config{AskUser: &on}, Config{AskUser: &off})
	if merged.ArbiterAskUser() {
		t.Fatal("ask_user=false ở lớp gần phải thắng")
	}
	merged = MergeConfig(Config{AskUser: &off}, Config{AskUser: &on})
	if !merged.ArbiterAskUser() {
		t.Fatal("ask_user=true ở lớp gần phải thắng")
	}
}
