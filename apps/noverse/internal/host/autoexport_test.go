package host

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/host/exp"
	"github.com/datnt1998/noverse/internal/library"
)

// newTestWatcher 直接装配 watcher（绕过 EnableAutoExport 的配置加载与
// goroutine 生命周期），事件经 notify 捕获。
func newTestWatcher(t *testing.T, r *StoryReader, storyDir string, ae library.AutoExport, formats []exp.Format) (*autoExportWatcher, *[]string) {
	t.Helper()
	events := &[]string{}
	var mu sync.Mutex
	w := &autoExportWatcher{
		cfg:         ae,
		formats:     formats,
		storySlug:   filepath.Base(storyDir),
		exportsDir:  filepath.Join(storyDir, "xuat-ban"),
		reader:      r,
		notify:      func(level, summary string) { mu.Lock(); *events = append(*events, level+"|"+summary); mu.Unlock() },
		completed:   map[int]bool{},
		volumesDone: map[int]bool{},
		volumesArm:  map[int]VolumeRange{},
	}
	w.snapshotBaseline()
	return w, events
}

func findFile(t *testing.T, root, wantName string) string {
	t.Helper()
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(path) == wantName {
			found = path
		}
		return nil
	})
	return found
}

func TestAutoExport_OnChapterAndOnBook(t *testing.T) {
	r, storyDir := newFlatStory(t, nil)
	ae := library.AutoExport{
		OnChapter: true, OnBook: true,
		Formats: []string{"txt"}, Pattern: "tap-{tap}/{truyen}.{tu}-{den}.{ext}",
		OnExists: "replace",
	}
	w, events := newTestWatcher(t, r, storyDir, ae, []exp.Format{exp.FormatTXT})

	// 提交第 1 章 → onChapter 触发，导出 ch1 所在（唯一）卷的当前范围。
	s := r.store
	if err := s.Drafts.SaveFinalChapter(1, "nội dung"); err != nil {
		t.Fatalf("save ch1: %v", err)
	}
	if err := s.Progress.StartChapter(1); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(1, 2, "", ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	w.poll(context.Background())

	slug := filepath.Base(storyDir)
	first := findFile(t, filepath.Join(storyDir, "xuat-ban"), slug+".1-1.txt")
	if first == "" {
		t.Fatalf("onChapter export file missing under xuat-ban; events=%v", *events)
	}
	if len(*events) == 0 || !strings.Contains((*events)[0], "Đã xuất") {
		t.Errorf("expected success event, got %v", *events)
	}

	// 完本 → onBook 导出全书 [1..1]（onExists=replace 可再次写同一路径）。
	if err := s.Progress.MarkComplete(); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	w.poll(context.Background())
	if len(*events) < 2 {
		t.Fatalf("expected book-complete export event, got %v", *events)
	}
}

func TestAutoExport_BaselineDoesNotReexport(t *testing.T) {
	// 启用时已有完成章：视为基线，不补导出。
	r, storyDir := newFlatStory(t, []int{1})
	ae := library.AutoExport{OnChapter: true, Formats: []string{"txt"}, Pattern: "{truyen}.{ext}", OnExists: "replace"}
	w, events := newTestWatcher(t, r, storyDir, ae, []exp.Format{exp.FormatTXT})
	w.poll(context.Background())
	if len(*events) != 0 {
		t.Errorf("baseline should not trigger exports, got %v", *events)
	}
	if entries, _ := os.ReadDir(filepath.Join(storyDir, "xuat-ban")); len(entries) != 0 {
		t.Errorf("xuat-ban should be empty, got %d entries", len(entries))
	}
}

func TestAutoExport_OnVolumeWaitsForSummary(t *testing.T) {
	r, storyDir := newLayeredStory(t, []int{1})
	ae := library.AutoExport{OnVolume: true, Formats: []string{"txt"}, Pattern: "tap-{tap}/{truyen}.{ext}", OnExists: "replace"}
	w, events := newTestWatcher(t, r, storyDir, ae, []exp.Format{exp.FormatTXT})

	s := r.store
	// 提交 ch2 = v1 末章（弧 2 是卷末弧）：先 armed，卷摘要未落盘不导出。
	if err := s.Drafts.SaveFinalChapter(2, "nội dung"); err != nil {
		t.Fatalf("save ch2: %v", err)
	}
	if err := s.Progress.StartChapter(2); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(2, 2, "", ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	w.poll(context.Background())
	if len(*events) != 0 {
		t.Fatalf("volume export should wait for volume summary, got %v", *events)
	}

	// 卷摘要落盘 → 下一轮 poll 导出 v1 [1..2]。
	if err := s.Summaries.SaveVolumeSummary(domain.VolumeSummary{Volume: 1, Title: "Tập một", Summary: "xong"}); err != nil {
		t.Fatalf("save vol summary: %v", err)
	}
	w.poll(context.Background())
	slug := filepath.Base(storyDir)
	if findFile(t, filepath.Join(storyDir, "xuat-ban"), slug+".txt") == "" {
		t.Fatalf("volume export file missing; events=%v", *events)
	}

	// 再 poll 不重复导出。
	n := len(*events)
	w.poll(context.Background())
	if len(*events) != n {
		t.Errorf("re-poll should not re-export volume, events=%v", *events)
	}
}

// TestEnableAutoExport_EndToEnd 走完整入口：library.json 配置 → EnableAutoExport
// → 进度事实变化 → 导出文件 + SYSTEM 事件。
func TestEnableAutoExport_EndToEnd(t *testing.T) {
	root := t.TempDir()
	cfg := library.DefaultConfig()
	cfg.AutoExport = library.AutoExport{
		OnChapter: true, Formats: []string{"txt"},
		Pattern: "{truyen}.{ext}", OnExists: "replace",
	}
	if err := os.MkdirAll(filepath.Join(root, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(library.ConfigPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r, storyDir := newFlatStory(t, nil)
	// 故事目录放到书库根下，模拟真实布局（不强制，但贴近 worker 运行形态）。
	_ = storyDir
	h := &Host{
		store:    r.store,
		cfg:      bootstrap.Config{},
		events:   make(chan Event, 16),
		streamCh: make(chan string, 4),
		done:     make(chan struct{}, 4),
	}
	h.runCtx, h.runCancel = context.WithCancel(context.Background())
	defer h.asyncWG.Wait() // 先声明后进 LIFO：下面 runCancel 先执行，watcher 退出后 Wait 才返回
	defer h.runCancel()

	old := autoExportPollInterval
	autoExportPollInterval = 20 * time.Millisecond
	defer func() { autoExportPollInterval = old }()

	if err := h.EnableAutoExport(root); err != nil {
		t.Fatalf("EnableAutoExport: %v", err)
	}
	if err := h.store.Drafts.SaveFinalChapter(1, "nội dung"); err != nil {
		t.Fatalf("save ch1: %v", err)
	}
	if err := h.store.Progress.StartChapter(1); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := h.store.Progress.MarkChapterComplete(1, 2, "", ""); err != nil {
		t.Fatalf("complete: %v", err)
	}

	slug := filepath.Base(storyDir)
	want := filepath.Join(storyDir, "xuat-ban", slug+".txt")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(want); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("auto export file not written: %v (events see below)", err)
	}

	// 事件通道里应有 SYSTEM success 事件。
	gotEvent := false
	timeout := time.After(2 * time.Second)
	for !gotEvent {
		select {
		case ev := <-h.events:
			if ev.Category == "SYSTEM" && ev.Level == "success" && strings.Contains(ev.Summary, "Đã xuất") {
				gotEvent = true
			}
		case <-timeout:
			t.Fatal("no SYSTEM success event for auto export")
		}
	}
}

func TestEnableAutoExport_NoRootNoop(t *testing.T) {
	h := &Host{}
	if err := h.EnableAutoExport(""); err != nil {
		t.Errorf("empty root should be a no-op, got %v", err)
	}
}
