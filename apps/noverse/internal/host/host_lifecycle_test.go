package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	storepkg "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestUpgradeProjectMigratesLegacyBook(t *testing.T) {
	dir := t.TempDir()
	st := storepkg.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	premise := "# 《寿元账》\n\n## 核心冲突\n\n凡人以寿元换取灵性，在求生与守住人性之间挣扎。\n\n## 主角目标\n\n活下去。"
	if err := st.Outline.SavePremise(premise); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	progress := []byte(`{"novel_name":"寿元账"}`)
	if err := os.WriteFile(filepath.Join(dir, "meta", "progress.json"), progress, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := upgradeProject(st); err != nil {
		t.Fatalf("upgradeProject: %v", err)
	}
	book, err := st.Book.Load()
	if err != nil {
		t.Fatalf("Load book: %v", err)
	}
	if book == nil || book.Title != "寿元账" || book.Synopsis != "凡人以寿元换取灵性，在求生与守住人性之间挣扎。" {
		t.Fatalf("unexpected migrated book: %+v", book)
	}
	if checkpoint := st.Checkpoints.LatestByStep(domain.GlobalScope(), "book"); checkpoint == nil {
		t.Fatal("book checkpoint was not recorded")
	}
	version, err := st.LoadProjectFormatVersion()
	if err != nil || version != storepkg.CurrentProjectFormatVersion {
		t.Fatalf("format version = %d, err = %v", version, err)
	}
}

func TestInterventionStopsWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	st := storepkg.NewStore(dir)
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.RunMeta.Init("default", "test", "model"); err != nil {
		t.Fatalf("RunMeta.Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta", "run.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{store: st, events: make(chan Event, 4)}
	err := h.doIntervention("Sửa tính cách nhân vật chính", false)
	if err == nil || !strings.Contains(err.Error(), "Ghi lưu can thiệp thất bại") {
		t.Fatalf("expected persistence error, got %v", err)
	}
	// 公共 Steer 必须等待异步任务并把同一业务错误返回给 TUI；不能只表示 goroutine
	// 启动成功，否则界面永远收不到真实失败。
	err = h.Steer("Sửa tính cách nhân vật chính", "reader")
	if err == nil || !strings.Contains(err.Error(), "Ghi lưu can thiệp thất bại") {
		t.Fatalf("Steer should return persistence error, got %v", err)
	}
	var users []Event
	for len(h.events) > 0 {
		ev := <-h.events
		if ev.Category == "USER" {
			users = append(users, ev)
		}
	}
	if len(users) != 1 || users[0].Summary != "Sửa tính cách nhân vật chính" || users[0].Input != "Sửa tính cách nhân vật chính" || users[0].Command != "steer" || users[0].Username != "reader" {
		t.Fatalf("expected one raw structured user action, got %+v", users)
	}
	err = h.Continue("  Tiếp tục  ", "reader")
	if err == nil || !strings.Contains(err.Error(), "Ghi lưu can thiệp thất bại") {
		t.Fatalf("Continue should return persistence error, got %v", err)
	}
	users = nil
	for len(h.events) > 0 {
		ev := <-h.events
		if ev.Category == "USER" {
			users = append(users, ev)
		}
	}
	if len(users) != 1 || users[0].Command != "continue" || users[0].Input != "  Tiếp tục  " || users[0].Username != "reader" {
		t.Fatalf("expected one attributed continue preserving raw input, got %+v", users)
	}
	h.engine, h.cocreating = &engine{}, true
	if err := h.ResumeFromCoCreate("Chốt hướng đi hồi ba"); err == nil {
		t.Fatal("internal continuation must still return persistence failure")
	}
	for len(h.events) > 0 {
		if ev := <-h.events; ev.Category == "USER" {
			t.Fatalf("co-create commit audit must not gain a synthetic second user action: %+v", ev)
		}
	}
}

func TestCloseWaitsForRegisteredAsyncWork(t *testing.T) {
	h := &Host{
		observer: &observer{},
		engine:   &engine{},
		events:   make(chan Event, 1),
		streamCh: make(chan string, 1),
		done:     make(chan struct{}, 1),
	}
	started := make(chan struct{})
	release := make(chan struct{})
	if !h.launchAsync(func() {
		close(started)
		<-release
	}) {
		t.Fatal("launchAsync unexpectedly refused")
	}
	<-started
	closed := make(chan struct{})
	go func() {
		h.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned before async work finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after async work finished")
	}
}
