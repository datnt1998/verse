// Test chuyển từ src/server/tools/configured-path.test.ts của yt-factory.
package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePathStoreRoundTripAndDelete(t *testing.T) {
	store := &FilePathStore{Dir: t.TempDir()}
	if _, ok := store.Get("tools:codex"); ok {
		t.Fatal("đọc key vắng thành công")
	}
	if err := store.Set("tools:codex", "/opt/codex"); err != nil {
		t.Fatal(err)
	}
	if got, ok := store.Get("tools:codex"); !ok || got != "/opt/codex" {
		t.Fatalf("get = %q %v", got, ok)
	}
	if err := store.Set("tools:codex", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get("tools:codex"); ok {
		t.Fatal("xoá không dứt điểm")
	}
}

func TestFilePathStoreMalformedReadsAsUnset(t *testing.T) {
	dir := t.TempDir()
	store := &FilePathStore{Dir: dir}
	// Bản ghi hỏng coi như vắng — không nhầm là đường dẫn hợp lệ.
	if err := os.WriteFile(filepath.Join(dir, "tools_codex.json"), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get("tools:codex"); ok {
		t.Fatal("bản ghi hỏng đọc được")
	}
	// Trường path rỗng cũng là vắng.
	if err := os.WriteFile(filepath.Join(dir, "tools_codex.json"), []byte(`{"path":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Get("tools:codex"); ok {
		t.Fatal("path rỗng đọc được")
	}
}

func TestResolutionOrderPutsConfiguredFirstSystemMiddleManagedLast(t *testing.T) {
	got := resolutionOrder("/cfg", []string{"/sys1", "", "/sys2"}, "/managed")
	if len(got) != 4 {
		t.Fatalf("got = %v", got)
	}
	if got[0].path != "/cfg" || got[0].source != SourceConfigured {
		t.Fatalf("got = %v", got)
	}
	if got[1].path != "/sys1" || got[2].path != "/sys2" ||
		got[1].source != SourceSystem || got[2].source != SourceSystem {
		t.Fatalf("got = %v", got)
	}
	if got[3].path != "/managed" || got[3].source != SourceManaged {
		t.Fatalf("got = %v", got)
	}
}
