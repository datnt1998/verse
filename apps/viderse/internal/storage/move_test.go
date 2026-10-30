package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Nguồn: yt-factory src/server/storage* (chuyển thư mục: cùng ổ đổi tên, khác
// ổ chép qua staging; huỷ và phục hồi giữ thư mục cũ nguyên).

// seedProject ghi dữ liệu mẫu vào hai nhóm.
func seedProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sources", "v1", "source.mp4"), "video-data")
	writeFile(t, filepath.Join(root, "editor", "projects", "p1", "project.json"), `{"id":"p1"}`)
}

func TestMoveSameVolumeRenames(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{1, 1}, // cùng fsid với "/"
	})
	taskID, done, err := rig.svc.Move(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if done || taskID == "" {
		t.Fatalf("move phải trả việc nền, nhận done=%v task=%q", done, taskID)
	}
	rig.waitIdle(t)
	want := filepath.Join(target, ProjectsFolderName) // gốc ổ → thư mục con
	if got := rig.svc.Root(); got != want {
		t.Fatalf("vị trí = %q, muốn %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(want, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("thiếu tệp sau khi chuyển: %v", err)
	}
	if _, err := os.Stat(rig.root); !os.IsNotExist(err) {
		t.Fatalf("cùng ổ phải đổi tên (thư mục cũ không còn), stat err = %v", err)
	}
	// Cùng ổ: đổi tên để thư mục cũ không còn; nếu có ghi bản cũ thì nhịp
	// watch kế tiếp phải quên nó (không còn gì để giữ).
	rig.svc.tick(context.Background())
	var prev domain.StoragePrevious
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStoragePrevious, &prev); found {
		t.Fatalf("bản cũ không còn tồn tại mà vẫn bị giữ: %+v", prev)
	}
}

func TestMoveDifferentVolumeCopiesAndKeepsOld(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{8, 8},
	})
	if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rig.waitIdle(t)
	want := filepath.Join(target, ProjectsFolderName)
	if got := rig.svc.Root(); got != want {
		t.Fatalf("vị trí = %q, muốn %q", got, want)
	}
	for _, rel := range []string{"sources/v1/source.mp4", "editor/projects/p1/project.json"} {
		if _, err := os.Stat(filepath.Join(want, rel)); err != nil {
			t.Fatalf("thiếu %s ở đích: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("khác ổ phải giữ thư mục cũ: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, stagingDirName)); !os.IsNotExist(err) {
		t.Fatalf("staging phải bị dọn, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, RootMarker)); err != nil {
		t.Fatalf("thiếu marker gốc: %v", err)
	}
	// Việc nền đã kết thúc; ít nhất một cập nhật tiến độ có nhóm.
	if _, ok := rig.tasks.lastUpdate(); !ok {
		t.Fatalf("không có cập nhật tiến độ nào")
	}
}

func TestMoveCancelLeavesOldFolderAndSetting(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{10, 10},
	})
	rig.tasks.block = make(chan struct{}, 1)
	rig.tasks.release = make(chan struct{})
	blockCh := rig.tasks.blockOnce()

	taskID, _, err := rig.svc.Move(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	select {
	case <-blockCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("không thấy cập nhật tiến độ đầu tiên")
	}
	if err := rig.svc.CancelMove(context.Background(), taskID); err != nil {
		t.Fatalf("CancelMove: %v", err)
	}
	close(rig.tasks.release)
	rig.waitIdle(t)

	if got := rig.svc.Root(); got != rig.root {
		t.Fatalf("huỷ không được đổi vị trí, nhận %q", got)
	}
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("huỷ phải giữ thư mục cũ: %v", err)
	}
	var ignored domain.StorageMove
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageMove, &ignored); found {
		t.Fatalf("huỷ xong phải xoá marker chuyển")
	}
	if holds := rig.svc.Holds(); len(holds) != 0 {
		t.Fatalf("holds còn %v", holds)
	}
	if rig.settings.storagePaused(t) {
		t.Fatalf("hàng chờ còn bị tạm dừng")
	}
}

func TestMoveCancelUnknownTask(t *testing.T) {
	rig := newTestRig(t)
	if err := rig.svc.CancelMove(context.Background(), "nope"); CodeOf(err) != ErrNoMove {
		t.Fatalf("huỷ khi không chuyển: %v", err)
	}
}

func TestMoveRecoversStagingAfterCrash(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{11, 11},
	})
	staging := filepath.Join(target, stagingDirName)
	writeFile(t, filepath.Join(staging, "sources", "v1", "source.mp4"), "partial")
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMove,
		domain.StorageMove{To: target, Staging: staging}); err != nil {
		t.Fatalf("set marker: %v", err)
	}

	rig.svc.recoverMove(context.Background())

	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging phải bị dọn, stat err = %v", err)
	}
	var ignored domain.StorageMove
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageMove, &ignored); found {
		t.Fatalf("marker phải bị xoá")
	}
	if got := rig.svc.Root(); got != rig.root {
		t.Fatalf("vị trí phải giữ nguyên, nhận %q", got)
	}
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("dữ liệu cũ phải nguyên: %v", err)
	}
}

func TestPreviousExpiresAfterSevenDays(t *testing.T) {
	rig := newTestRig(t)
	prevDir := filepath.Join(t.TempDir(), "old-projects")
	writeFile(t, filepath.Join(prevDir, "a.txt"), "x")
	prev := &domain.StoragePrevious{
		Path: prevDir, Bytes: 1, DeleteAfter: rig.clock.Now().Add(-time.Minute),
	}
	if err := rig.svc.setPrevious(context.Background(), prev); err != nil {
		t.Fatalf("setPrevious: %v", err)
	}
	rig.svc.tick(context.Background())
	if _, err := os.Stat(prevDir); !os.IsNotExist(err) {
		t.Fatalf("bản cũ quá hạn phải bị xoá, stat err = %v", err)
	}
	var ignored domain.StoragePrevious
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStoragePrevious, &ignored); found {
		t.Fatalf("bản cũ phải bị quên")
	}
}

func TestPreviousNotExpiredStays(t *testing.T) {
	rig := newTestRig(t)
	prevDir := filepath.Join(t.TempDir(), "old-projects")
	writeFile(t, filepath.Join(prevDir, "a.txt"), "x")
	prev := &domain.StoragePrevious{
		Path: prevDir, Bytes: 1, DeleteAfter: rig.clock.Now().Add(24 * time.Hour),
	}
	if err := rig.svc.setPrevious(context.Background(), prev); err != nil {
		t.Fatalf("setPrevious: %v", err)
	}
	rig.svc.tick(context.Background())
	if _, err := os.Stat(prevDir); err != nil {
		t.Fatalf("bản cũ còn hạn phải giữ: %v", err)
	}
}

func TestDeletePreviousRemovesNow(t *testing.T) {
	rig := newTestRig(t)
	prevDir := filepath.Join(t.TempDir(), "old-projects")
	writeFile(t, filepath.Join(prevDir, "a.txt"), "x")
	if err := rig.svc.setPrevious(context.Background(), &domain.StoragePrevious{
		Path: prevDir, Bytes: 1, DeleteAfter: rig.clock.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("setPrevious: %v", err)
	}
	if err := rig.svc.DeletePrevious(context.Background()); err != nil {
		t.Fatalf("DeletePrevious: %v", err)
	}
	if _, err := os.Stat(prevDir); !os.IsNotExist(err) {
		t.Fatalf("bản cũ phải bị xoá ngay, stat err = %v", err)
	}
	if err := rig.svc.DeletePrevious(context.Background()); CodeOf(err) != ErrNotFound {
		t.Fatalf("xoá lần hai: %v", err)
	}
}

func TestNewOnlyKeepsOldAsAltRoot(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{12, 12},
	})
	_, done, err := rig.svc.Move(context.Background(), target, ModeNewOnly)
	if err != nil {
		t.Fatalf("Move new_only: %v", err)
	}
	if !done {
		t.Fatalf("new_only phải xong ngay")
	}
	want := filepath.Join(target, ProjectsFolderName)
	if got := rig.svc.Root(); got != want {
		t.Fatalf("vị trí = %q, muốn %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("new_only không được đụng dữ liệu cũ: %v", err)
	}
	candidates, err := rig.svc.Candidates("sources/v1/source.mp4")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(candidates) < 2 || candidates[1] != filepath.Join(rig.root, "sources", "v1", "source.mp4") {
		t.Fatalf("đọc phải rơi về thư mục cũ, nhận %v", candidates)
	}
}

func TestUseDefaultForNewSwitchesWithoutMovingData(t *testing.T) {
	rig := newTestRig(t)
	ext := rig.mkTarget(t, "external", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{13, 13},
	})
	if _, _, err := rig.svc.Move(context.Background(), ext, ModeNewOnly); err != nil {
		t.Fatalf("Move new_only: %v", err)
	}
	if err := rig.svc.UseDefaultForNew(context.Background()); err != nil {
		t.Fatalf("UseDefaultForNew: %v", err)
	}
	if got := rig.svc.Root(); got != rig.root {
		t.Fatalf("vị trí = %q, muốn mặc định %q", got, rig.root)
	}
	if _, err := os.Stat(ext); err != nil {
		t.Fatalf("dữ liệu trên ổ cũ phải còn: %v", err)
	}
}

func TestMoveRejectsCloudAndConcurrent(t *testing.T) {
	rig := newTestRig(t)
	cloud := filepath.Join(rig.home, "Library", "CloudStorage", "Google Drive")
	if err := os.MkdirAll(cloud, 0o755); err != nil {
		t.Fatalf("mkdir cloud: %v", err)
	}
	if _, _, err := rig.svc.Move(context.Background(), cloud, ModeMove); CodeOf(err) != ErrCloudSync {
		t.Fatalf("đồng bộ đám mây: %v", err)
	}

	rig.tasks.block = make(chan struct{}, 1)
	rig.tasks.release = make(chan struct{})
	blockCh := rig.tasks.blockOnce()
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{14, 14},
	})
	if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	<-blockCh
	if _, _, err := rig.svc.Move(context.Background(), filepath.Join(target, "x"), ModeMove); CodeOf(err) != ErrMoveInProgress {
		t.Fatalf("chuyển chồng: %v", err)
	}
	if err := rig.svc.CancelMove(context.Background(), "task-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	close(rig.tasks.release)
	rig.waitIdle(t)
}

func TestMoveToCurrentLocationIsNoop(t *testing.T) {
	rig := newTestRig(t)
	_, done, err := rig.svc.Move(context.Background(), rig.root, ModeMove)
	if err != nil || !done {
		t.Fatalf("cùng chỗ: done=%v err=%v", done, err)
	}
}
