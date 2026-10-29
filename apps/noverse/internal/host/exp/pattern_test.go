package exp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderPattern_AllVars(t *testing.T) {
	rel, err := RenderPattern("tap-{tap}/{truyen}.chuong-{tu}-{den}.{ngay}.{ext}", PatternVars{
		Story:  "01-tra-xanh",
		Volume: 2,
		From:   5,
		To:     12,
		Date:   time.Date(2026, 10, 25, 9, 0, 0, 0, time.Local),
		Format: "txt",
	})
	if err != nil {
		t.Fatalf("RenderPattern: %v", err)
	}
	want := filepath.Join("tap-2", "01-tra-xanh.chuong-5-12.20261025.txt")
	if rel != want {
		t.Errorf("rel = %q, want %q", rel, want)
	}
}

func TestRenderPattern_DefaultContract(t *testing.T) {
	// 契约默认 pattern：tap-{tap}/{truyen}.tap-{tap}.{ext}
	rel, err := RenderPattern("tap-{tap}/{truyen}.tap-{tap}.{ext}", PatternVars{
		Story: "truyen-a", Volume: 1, Format: "epub",
	})
	if err != nil {
		t.Fatalf("RenderPattern: %v", err)
	}
	if rel != filepath.Join("tap-1", "truyen-a.tap-1.epub") {
		t.Errorf("rel = %q", rel)
	}
}

func TestRenderPattern_Rejects(t *testing.T) {
	cases := map[string]string{
		"unknown var": "{truyen}.{ext}.{bogus}",
		"dotdot":      "../{truyen}.{ext}",
		"empty seg":   "a//{truyen}.{ext}",
		"unclosed":    "{truyen",
		"empty":       "",
		"abs-ish dot": "./{truyen}.{ext}",
	}
	for name, pattern := range cases {
		if _, err := RenderPattern(pattern, PatternVars{Story: "s", Volume: 1, Format: "txt"}); err == nil {
			t.Errorf("%s: pattern %q should be rejected", name, pattern)
		}
	}
}

func TestResolvePatternPath_JoinsUnderExportsDir(t *testing.T) {
	got, err := ResolvePatternPath("/lib/story/xuat-ban", "tap-{tap}/f.{ext}", PatternVars{Volume: 1, Format: "txt"})
	if err != nil {
		t.Fatalf("ResolvePatternPath: %v", err)
	}
	if got != "/lib/story/xuat-ban/tap-1/f.txt" {
		t.Errorf("got %q", got)
	}
}

func TestRun_OnExistsKeep(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1})
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target, OnExists: "keep"})
	if err != nil {
		t.Fatalf("Run keep: %v", err)
	}
	if res.Path == target {
		t.Error("keep should pick a timestamped variant, not the same path")
	}
	if !strings.HasPrefix(filepath.Base(res.Path), "out-") || !strings.HasSuffix(res.Path, ".txt") {
		t.Errorf("variant name = %q", res.Path)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "preexisting" {
		t.Error("original file must be preserved under keep")
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Errorf("variant file missing: %v", err)
	}
}

func TestRun_OnExistsReplace(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1})
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target, OnExists: "replace"})
	if err != nil {
		t.Fatalf("Run replace: %v", err)
	}
	if res.Path != target {
		t.Errorf("replace should write the same path, got %q", res.Path)
	}
}

func TestRun_OnExistsInvalid(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1})
	_, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: filepath.Join(dir, "o.txt"), OnExists: "bogus"})
	if err == nil {
		t.Fatal("invalid onExists should error")
	}
}

func TestRun_VietnameseHeadings(t *testing.T) {
	s, dir := newTestStore(t, "Truyện Việt", []int{1, 2})
	out := filepath.Join(dir, "vi.txt")
	if _, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: out}); err != nil {
		t.Fatalf("Run vi: %v", err)
	}
	data, _ := os.ReadFile(out)
	text := string(data)
	if !strings.Contains(text, "Chương 1") || !strings.Contains(text, "Chương 2") {
		t.Errorf("vi chapter headings missing:\n%s", text)
	}
	if strings.Contains(text, "第") {
		t.Errorf("vi export must not contain Chinese headings:\n%s", text)
	}
	// vi 书名不加《》书名号。
	if strings.Contains(text, "《") {
		t.Errorf("bản xuất vi không được bọc tiêu đề trong 《》:\n%s", text)
	}
}

// GAP-27：keep thêm số khi cùng giây vẫn trùng, không báo lỗi.
func TestKeepVariantPathAppendsCounterOnSameSecond(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	now := time.Date(2026, 10, 25, 9, 8, 7, 0, time.UTC)
	first, err := keepVariantPath(target, now)
	if err != nil || filepath.Base(first) != "out-20261025-090807.txt" {
		t.Fatalf("first = %q err=%v", first, err)
	}
	if err := os.WriteFile(first, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := keepVariantPath(target, now)
	if err != nil || filepath.Base(second) != "out-20261025-090807-2.txt" {
		t.Fatalf("second = %q err=%v", second, err)
	}
	if err := os.WriteFile(second, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := keepVariantPath(target, now)
	if err != nil || filepath.Base(third) != "out-20261025-090807-3.txt" {
		t.Fatalf("third = %q err=%v", third, err)
	}
}
