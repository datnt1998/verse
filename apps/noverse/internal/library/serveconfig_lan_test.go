package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serve.json giữ cả root lẫn lan: ghi trường này không được xoá trường kia.
func TestServeConfigRootAndLANPreserveEachOther(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, "thu-vien")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := SaveServeLAN(true); err != nil {
		t.Fatal(err)
	}
	if err := SaveServeRoot(dir); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadServeConfig()
	if err != nil || cfg.Root != dir || !cfg.LAN {
		t.Fatalf("after root save: %+v %v", cfg, err)
	}
	if err := SaveServeLAN(false); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadServeConfig()
	if err != nil || cfg.Root != dir || cfg.LAN {
		t.Fatalf("after lan save: %+v %v", cfg, err)
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".ainovel", "serve.json"))
	if !strings.Contains(string(raw), `"root"`) || !strings.Contains(string(raw), `"lan"`) {
		t.Fatalf("serve.json: %s", raw)
	}
}
