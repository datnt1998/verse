package exp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_ExistsErrorIsTypedAndDryRunNeverWrites(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1, 2})
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("preexisting"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target})
	var exists *ExistsError
	if !errors.As(err, &exists) || exists.Path != target {
		t.Fatalf("want *ExistsError for %s, got %v", target, err)
	}
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Path != target || res.Chapters != 2 || res.Bytes == 0 {
		t.Fatalf("dry run result %+v", res)
	}
	if data, _ := os.ReadFile(target); string(data) != "preexisting" {
		t.Fatalf("dry run wrote the file: %q", data)
	}
	fresh := filepath.Join(dir, "fresh.txt")
	if _, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: fresh, DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Fatalf("dry run created %s", fresh)
	}
}

func TestRun_KeepAddsTimestampAndLeavesOriginal(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1})
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("preexisting"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target, OnExists: "keep"})
	if err != nil {
		t.Fatalf("keep: %v", err)
	}
	base := filepath.Base(res.Path)
	if res.Path == target || !strings.HasPrefix(base, "out-") || !strings.HasSuffix(base, ".txt") {
		t.Fatalf("keep path %q", res.Path)
	}
	if data, _ := os.ReadFile(target); string(data) != "preexisting" {
		t.Fatalf("original clobbered: %q", data)
	}
}

func TestRun_ProgressReportsEveryChapter(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1, 2, 3})
	var got []int
	_, err := Run(context.Background(), Deps{Store: s}, Options{
		OutPath: filepath.Join(dir, "p.txt"),
		Progress: func(done, total, chapter int, _ string) {
			if total != 3 || done != len(got)+1 {
				t.Errorf("progress done=%d total=%d", done, total)
			}
			got = append(got, chapter)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("progress chapters %v", got)
	}
}

func TestCountChaptersMatchesExport(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1, 2, 3})
	for _, name := range []string{"c.txt", "c.epub"} {
		res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: filepath.Join(dir, name)})
		if err != nil {
			t.Fatal(err)
		}
		if n, ok := CountChapters(res.Path); !ok || n != 3 {
			t.Fatalf("%s: CountChapters = %d,%v want 3", name, n, ok)
		}
	}
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("không phải bản xuất"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := CountChapters(other); ok {
		t.Fatal("foreign file should be unrecognized")
	}
}
