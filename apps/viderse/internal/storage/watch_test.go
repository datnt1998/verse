package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Nguồn: yt-factory src/server/storage* (theo dõi ổ) + design V22 (mất ổ →
// giữ việc + báo trong 10 giây).

func TestWatchIntervalUnderTenSeconds(t *testing.T) {
	if DefaultWatchEvery > 10*time.Second {
		t.Fatalf("nhịp watch %v vượt cam kết 10 giây", DefaultWatchEvery)
	}
}

func TestWatchOfflineThenOnline(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	rig.svc.tick(context.Background())
	if offline, _ := rig.svc.Offline(); offline {
		t.Fatalf("thư mục còn đó mà báo offline")
	}

	gone := rig.root + "-gone"
	if err := os.Rename(rig.root, gone); err != nil {
		t.Fatalf("rename: %v", err)
	}
	rig.svc.tick(context.Background())

	offline, since := rig.svc.Offline()
	if !offline || since == nil {
		t.Fatalf("phải phát hiện mất thư mục")
	}
	if !containsHold(rig.svc.Holds(), "offline") {
		t.Fatalf("holds = %v, thiếu offline", rig.svc.Holds())
	}
	if !rig.settings.storagePaused(t) {
		t.Fatalf("hàng chờ phải tạm dừng")
	}
	calls := rig.notify.all()
	if len(calls) != 1 || !strings.Contains(calls[0], "needs_attention") || !strings.Contains(calls[0], "Không thấy thư mục dự án") {
		t.Fatalf("thông báo = %v", calls)
	}
	if items := rig.svc.AttentionItems(context.Background()); len(items) != 1 || items[0].Href != "#/settings?section=storage" {
		t.Fatalf("mục cần xử lý = %+v", items)
	}
	// Nhịp thứ hai không lặp thông báo.
	rig.svc.tick(context.Background())
	if len(rig.notify.all()) != 1 {
		t.Fatalf("thông báo bị lặp: %v", rig.notify.all())
	}

	if err := os.Rename(gone, rig.root); err != nil {
		t.Fatalf("rename lại: %v", err)
	}
	rig.svc.tick(context.Background())
	if offline, _ := rig.svc.Offline(); offline {
		t.Fatalf("ổ về mà vẫn báo offline")
	}
	if holds := rig.svc.Holds(); len(holds) != 0 {
		t.Fatalf("holds = %v, phải rỗng", holds)
	}
	if rig.settings.storagePaused(t) {
		t.Fatalf("hàng chờ phải chạy lại")
	}
}

func TestWatchKeepsUserPause(t *testing.T) {
	rig := newTestRig(t)
	if err := rig.settings.Set(context.Background(), domain.KeyQueuePaused, true); err != nil {
		t.Fatalf("pause: %v", err)
	}
	gone := rig.root + "-gone"
	if err := os.Rename(rig.root, gone); err != nil {
		t.Fatalf("rename: %v", err)
	}
	rig.svc.tick(context.Background())
	if err := os.Rename(gone, rig.root); err != nil {
		t.Fatalf("rename lại: %v", err)
	}
	rig.svc.tick(context.Background())
	if !rig.settings.userPaused(t) {
		t.Fatalf("cờ tạm dừng do người dùng bật không được tắt")
	}
	if rig.settings.storagePaused(t) {
		t.Fatalf("ổ quay lại rồi mà hold còn treo")
	}
}

func TestRecheckReportsCurrentState(t *testing.T) {
	rig := newTestRig(t)
	online, err := rig.svc.Recheck(context.Background())
	if err != nil || !online {
		t.Fatalf("Recheck = %v, %v; muốn online", online, err)
	}
	gone := rig.root + "-gone"
	if err := os.Rename(rig.root, gone); err != nil {
		t.Fatalf("rename: %v", err)
	}
	online, err = rig.svc.Recheck(context.Background())
	if err != nil || online {
		t.Fatalf("Recheck = %v, %v; muốn offline", online, err)
	}
}

func TestResolverReadsAltRootsThenPrevious(t *testing.T) {
	rig := newTestRig(t)
	alt := filepath.Join(t.TempDir(), "old")
	prev := filepath.Join(t.TempDir(), "older")
	if err := rig.svc.addAltRoot(context.Background(), alt); err != nil {
		t.Fatalf("addAltRoot: %v", err)
	}
	if err := rig.svc.setPrevious(context.Background(), &domain.StoragePrevious{
		Path: prev, Bytes: 1, DeleteAfter: rig.clock.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("setPrevious: %v", err)
	}
	candidates, err := rig.svc.Candidates("images/a.png")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	want := []string{
		filepath.Join(rig.root, "images/a.png"),
		filepath.Join(alt, "images/a.png"),
		filepath.Join(prev, "images/a.png"),
	}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %v, muốn %v", candidates, want)
	}
	for i := range want {
		if candidates[i] != want[i] {
			t.Fatalf("candidates[%d] = %q, muốn %q", i, candidates[i], want[i])
		}
	}
}

func TestResolverRejectsEscapes(t *testing.T) {
	rig := newTestRig(t)
	for _, rel := range []string{"../outside", "/abs/path", ""} {
		if _, err := rig.svc.Resolve(rel); CodeOf(err) != ErrInvalidPath {
			t.Fatalf("Resolve(%q) = %v, muốn invalid_path", rel, err)
		}
		if _, err := rig.svc.Candidates(rel); CodeOf(err) != ErrInvalidPath {
			t.Fatalf("Candidates(%q) = %v, muốn invalid_path", rel, err)
		}
	}
	got, err := rig.svc.Resolve("images/a.png")
	if err != nil || got != filepath.Join(rig.root, "images/a.png") {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
	if _, err := rig.svc.GroupPath("nope"); CodeOf(err) != ErrInvalidPath {
		t.Fatalf("GroupPath lạ: %v", err)
	}
	if group, err := rig.svc.GroupPath(GroupEditor); err != nil || group != filepath.Join(rig.root, "editor") {
		t.Fatalf("GroupPath = %q, %v", group, err)
	}
}

func TestBrowseRootsAndHiddenFiltering(t *testing.T) {
	rig := newTestRig(t)
	home := rig.home
	for _, dir := range []string{"Documents", ".hidden", "Library", "Music"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	writeFile(t, filepath.Join(home, "notes.txt"), "x")

	result, err := rig.svc.Browse(context.Background(), "")
	if err != nil {
		t.Fatalf("Browse rỗng: %v", err)
	}
	if len(result.Roots) == 0 || result.Roots[0].Path != home {
		t.Fatalf("roots = %+v, muốn bắt đầu bằng thư mục nhà", result.Roots)
	}
	if len(result.Dirs) != len(result.Roots) {
		t.Fatalf("path rỗng: dirs phải bằng roots")
	}

	result, err = rig.svc.Browse(context.Background(), "~")
	if err != nil {
		t.Fatalf("Browse ~: %v", err)
	}
	if result.Path != home {
		t.Fatalf("path = %q, muốn %q", result.Path, home)
	}
	names := map[string]bool{}
	for _, d := range result.Dirs {
		names[d.Name] = true
	}
	if !names["Documents"] || !names["Music"] {
		t.Fatalf("thiếu thư mục thường: %+v", result.Dirs)
	}
	if names[".hidden"] || names["Library"] || names["notes.txt"] {
		t.Fatalf("lọt mục ẩn/hệ thống/tệp: %+v", result.Dirs)
	}

	if _, err := rig.svc.Browse(context.Background(), filepath.Join(home, "notes.txt")); CodeOf(err) != ErrNotFound {
		t.Fatalf("duyệt tệp: %v", err)
	}
}

func TestOpenUsesInjectedOpener(t *testing.T) {
	rig := newTestRig(t)
	if err := rig.svc.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if rig.opener.path != rig.root {
		t.Fatalf("mở %q, muốn %q", rig.opener.path, rig.root)
	}
	if err := os.Rename(rig.root, rig.root+"-gone"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := rig.svc.Open(context.Background()); CodeOf(err) != ErrVolumeGone {
		t.Fatalf("mở khi ổ mất: %v", err)
	}
}

func containsHold(holds []domain.Hold, want domain.Hold) bool {
	for _, h := range holds {
		if h == want {
			return true
		}
	}
	return false
}
