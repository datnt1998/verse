package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Nguồn: yt-factory media-gc/retention safety.

/* ----------------- move: từ chối đích xung đột và đích nguy hiểm ----------------- */

// Đích có tệp khác nội dung ở đường dẫn sẽ ghi → từ chối, hai cây nguyên.
func TestMoveRejectsConflictingDestination(t *testing.T) {
	rig := newTestRig(t)
	writeFile(t, filepath.Join(rig.root, "images", "a.png"), "source-bytes")
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{9, 9},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := writeRootMarker(target); err != nil {
		t.Fatalf("marker: %v", err)
	}
	writeFile(t, filepath.Join(target, "images", "a.png"), "DEST-DATA")

	if _, _, err := rig.svc.Move(context.Background(), mount, ModeMove); CodeOf(err) != ErrDestConflict {
		t.Fatalf("Move = %v, muốn dest_conflict", err)
	}
	if got := readFile(t, filepath.Join(target, "images", "a.png")); got != "DEST-DATA" {
		t.Fatalf("dữ liệu đích bị đụng: %q", got)
	}
	if got := readFile(t, filepath.Join(rig.root, "images", "a.png")); got != "source-bytes" {
		t.Fatalf("dữ liệu nguồn bị đụng: %q", got)
	}
	if rig.svc.Root() != rig.root {
		t.Fatalf("vị trí đổi = %q", rig.svc.Root())
	}
}

// Tệp đích giống hệt nguồn (move-back bản cũ) không tính trùng.
func TestMoveAllowsIdenticalDestinationFiles(t *testing.T) {
	rig := newTestRig(t)
	writeFile(t, filepath.Join(rig.root, "sources", "v1", "same.mp4"), "identical")
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{10, 10},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := writeRootMarker(target); err != nil {
		t.Fatalf("marker: %v", err)
	}
	writeFile(t, filepath.Join(target, "sources", "v1", "same.mp4"), "identical")
	writeFile(t, filepath.Join(target, "sources", "v1", "extra.mp4"), "dest-only")

	if _, _, err := rig.svc.Move(context.Background(), mount, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rig.waitIdle(t)
	if got := readFile(t, filepath.Join(target, "sources", "v1", "extra.mp4")); got != "dest-only" {
		t.Fatalf("tệp riêng của đích phải còn: %q", got)
	}
	if rig.svc.Root() != target {
		t.Fatalf("vị trí = %q", rig.svc.Root())
	}
}

// Đích lồng/chứa/hệ thống/thư mục lạ không-trống đều bị chặn trước mọi I/O.
func TestMoveRejectsUnsafeTargets(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)

	cases := []struct {
		name   string
		target func() string
		code   string
	}{
		{"lồng trong root", func() string { return filepath.Join(rig.root, "nested") }, ErrInvalidPath},
		{"chứa root", func() string { return filepath.Dir(rig.root) }, ErrInvalidPath},
		{"chính home", func() string { return rig.home }, ErrInvalidPath},
		{"trong config", func() string { return filepath.Join(rig.home, ".viderse", "logs") }, ErrInvalidPath},
		{"hệ thống", func() string { return "/usr/local/vp" }, ErrInvalidPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target()
			if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); CodeOf(err) != tc.code {
				t.Fatalf("Move(%s) = %v, muốn %s", target, err, tc.code)
			}
			if rig.svc.Root() != rig.root {
				t.Fatalf("vị trí đổi = %q", rig.svc.Root())
			}
		})
	}

	// Thư mục lạ không-trống: new_only cũng không được "nhận nuôi".
	strange := filepath.Join(canonicalOr(t, t.TempDir()), "stranger")
	writeFile(t, filepath.Join(strange, "user.txt"), "not ours")
	if _, _, err := rig.svc.Move(context.Background(), strange, ModeNewOnly); CodeOf(err) != ErrDestNotEmpty {
		t.Fatalf("new_only vào thư mục lạ = %v, muốn dest_not_empty", err)
	}
	if _, err := os.Lstat(filepath.Join(strange, RootMarker)); !os.IsNotExist(err) {
		t.Fatalf("marker không được ghi vào thư mục lạ")
	}
	if got := readFile(t, filepath.Join(strange, "user.txt")); got != "not ours" {
		t.Fatalf("dữ liệu người dùng bị đụng")
	}
}

/* ----------------- recover: sập sau rename cùng ổ thì nhận đích ----------------- */

// Sập sau rename cùng ổ (trước setLocation): khởi động sau nhận đích luôn.
func TestRecoverAdoptsRenamedDestination(t *testing.T) {
	rig := newTestRig(t)
	writeFile(t, filepath.Join(rig.root, "sources", "v1", "data.bin"), "payload")
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{11, 11},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	// Nhật ký copy như lúc bắt đầu chuyển; rename xong rồi "sập".
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMove,
		domain.StorageMove{From: rig.root, To: target, Staging: filepath.Join(target, stagingDirName), Phase: domain.MovePhaseCopy}); err != nil {
		t.Fatalf("journal: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove empty dest: %v", err)
	}
	if err := os.Rename(rig.root, target); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := writeRootMarker(target); err != nil {
		t.Fatalf("marker: %v", err)
	}

	// "Khởi động lại": service mới trên cùng settings.
	svc2 := rig.restart(t)
	svc2.recoverMove(context.Background())
	if got := svc2.Root(); got != target {
		t.Fatalf("vị trí sau phục hồi = %q, muốn %q", got, target)
	}
	if got := readFile(t, filepath.Join(target, "sources", "v1", "data.bin")); got != "payload" {
		t.Fatalf("dữ liệu mất: %q", got)
	}
	var j domain.StorageMove
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageMove, &j); found {
		t.Fatalf("nhật ký phải xoá sau phục hồi: %+v", j)
	}
}

// Sập trong phase copy: nguồn nguyên, staging dọn, vị trí không đổi.
func TestRecoverCleansInterruptedCopy(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{12, 12},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	staging := filepath.Join(target, stagingDirName)
	writeFile(t, filepath.Join(staging, "images", "partial.png"), "half")
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMove,
		domain.StorageMove{From: rig.root, To: target, Staging: staging, Phase: domain.MovePhaseCopy}); err != nil {
		t.Fatalf("journal: %v", err)
	}
	svc2 := rig.restart(t)
	svc2.recoverMove(context.Background())
	if got := svc2.Root(); got != rig.root {
		t.Fatalf("vị trí = %q", got)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging phải bị dọn")
	}
	if got := readFile(t, filepath.Join(rig.root, "sources", "v1", "source.mp4")); got != "video-data" {
		t.Fatalf("nguồn phải nguyên: %q", got)
	}
}

// Sập sau khi cài đặt đã đổi nhưng nhật ký còn: hoàn tất phần còn lại.
func TestRecoverFinishesCommittedCutover(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{13, 13},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	if err := os.Rename(rig.root, target); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := writeRootMarker(target); err != nil {
		t.Fatalf("marker: %v", err)
	}
	// Vị trí ĐÃ đổi nhưng nhật ký còn treo (sập giữa commitMove).
	if err := rig.settings.Set(context.Background(), domain.KeyStorageLocation, target); err != nil {
		t.Fatalf("location: %v", err)
	}
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMove,
		domain.StorageMove{From: rig.root, To: target, Phase: domain.MovePhaseCommit}); err != nil {
		t.Fatalf("journal: %v", err)
	}
	svc2 := rig.restart(t)
	svc2.recoverMove(context.Background())
	var j domain.StorageMove
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageMove, &j); found {
		t.Fatalf("nhật ký phải xoá: %+v", j)
	}
	if got := svc2.Root(); got != target {
		t.Fatalf("vị trí = %q", got)
	}
}

// Đích không đọc được (ổ rút): nhật ký phải được GIỮ — không đoán xoá.
func TestRecoverKeepsJournalWhenDestinationUnreachable(t *testing.T) {
	rig := newTestRig(t)
	gone := filepath.Join(canonicalOr(t, t.TempDir()), "unreachable")
	staging := filepath.Join(gone, stagingDirName)
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMove,
		domain.StorageMove{From: filepath.Join(gone, "from"), To: gone, Staging: staging, Phase: domain.MovePhaseCommit}); err != nil {
		t.Fatalf("journal: %v", err)
	}
	svc2 := rig.restart(t)
	svc2.recoverMove(context.Background())
	var j domain.StorageMove
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageMove, &j); !found {
		t.Fatalf("nhật ký bị xoá khi đích chưa đọc được")
	}
}

// Root đang dùng bị ghi nhầm vào previous: không bao giờ bị xoá.
func TestActiveRootNeverScheduledForDeletion(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	if err := rig.settings.Set(context.Background(), domain.KeyStoragePrevious,
		domain.StoragePrevious{Path: rig.root, DeleteAfter: rig.clock.Now().Add(-time.Hour), Mode: ModeMove}); err != nil {
		t.Fatalf("previous: %v", err)
	}
	svc2 := rig.restart(t)
	svc2.settlePrevious(context.Background())
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("root đang dùng bị xoá: %v", err)
	}
	if err := svc2.DeletePrevious(context.Background()); CodeOf(err) != ErrNotDeletable {
		t.Fatalf("DeletePrevious trên root đang dùng = %v", err)
	}
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("root đang dùng bị xoá sau DeletePrevious: %v", err)
	}
}

/* ----------------- Move đồng thời tuần tự, new_only bị chặn lúc move ----------------- */

// Hai Move cùng lúc: chỉ một lần qua được chỗ.
func TestMoveSerializesConcurrentCalls(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{14, 14},
	})
	const n = 4
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := rig.svc.Move(context.Background(), target, ModeMove)
			results[i] = err
		}(i)
	}
	wg.Wait()
	rig.waitIdle(t)
	won := 0
	for _, err := range results {
		switch {
		case err == nil:
			won++
		case CodeOf(err) == ErrMoveInProgress:
		case CodeOf(err) == ErrDestConflict || CodeOf(err) == ErrInvalidPath || CodeOf(err) == ErrDestNotEmpty:
			// thua cuộc chỗ nhưng đi xa hơn vẫn phải bị chặn an toàn; tới lúc
			// lần thắng đang chép dở thì đích đã có tệp mà chưa có marker
		default:
			t.Fatalf("lỗi lạ: %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d lần Move cùng thắng chỗ", won)
	}
	if holds := rig.svc.Holds(); len(holds) != 0 {
		t.Fatalf("holds còn %v sau khi xong", holds)
	}
}

// new_only trong lúc move đang chạy phải bị chặn; sau khi xong mới được.
func TestNewOnlyBlockedDuringMove(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{15, 15},
	})
	rig.tasks.mu.Lock()
	rig.tasks.block = make(chan struct{}, 1)
	rig.tasks.release = make(chan struct{})
	block := rig.tasks.block
	rig.tasks.mu.Unlock()
	if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	<-block // worker đang chạy
	other := rig.mkTarget(t, "other", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{16, 16},
	})
	if _, _, err := rig.svc.Move(context.Background(), other, ModeNewOnly); CodeOf(err) != ErrMoveInProgress {
		t.Fatalf("new_only khi đang chuyển = %v", err)
	}
	if err := rig.svc.UseDefaultForNew(context.Background()); CodeOf(err) != ErrMoveInProgress {
		t.Fatalf("use_default khi đang chuyển = %v", err)
	}
	close(rig.tasks.release)
	rig.waitIdle(t)
	if _, _, err := rig.svc.Move(context.Background(), other, ModeNewOnly); err != nil {
		t.Fatalf("new_only sau khi xong = %v", err)
	}
}

/* ----------------- shutdown huỷ và chờ worker move ----------------- */

// Shutdown huỷ và chờ worker move — không bỏ dở giữa chừng.
func TestShutdownCancelsAndJoinsMove(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{17, 17},
	})
	if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rig.svc.mu.Lock()
	for rig.svc.moving == nil || rig.svc.moving.cancel == nil {
		rig.svc.mu.Unlock()
		time.Sleep(time.Millisecond)
		rig.svc.mu.Lock()
	}
	rig.svc.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rig.svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// Nguồn nguyên; vị trí không đổi (nhật ký đã xoá theo nhánh huỷ).
	if got := rig.svc.Root(); got != rig.root {
		t.Fatalf("vị trí = %q", got)
	}
	if got := readFile(t, filepath.Join(rig.root, "sources", "v1", "source.mp4")); got != "video-data" {
		t.Fatalf("nguồn phải nguyên: %q", got)
	}
}

/* ----------------- holds qua restart, pause người dùng giữ nguyên ----------------- */

// Hold persist qua restart; HoldMoving của lần chuyển bị cắt ngang được gỡ —
// cờ paus của storage hạ nhưng cờ người dùng không đụng.
func TestHoldsReconcileAcrossRestart(t *testing.T) {
	rig := newTestRig(t)
	if err := rig.settings.Set(context.Background(), domain.KeyStorageHolds,
		domain.StorageHolds{domain.HoldMoving, domain.HoldOffline}); err != nil {
		t.Fatalf("holds: %v", err)
	}
	if err := rig.settings.Set(context.Background(), domain.KeyStoragePaused, true); err != nil {
		t.Fatalf("paused: %v", err)
	}
	svc2 := rig.restart(t)
	svc2.reconcileHolds(context.Background())
	if containsHold(svc2.Holds(), domain.HoldMoving) {
		t.Fatalf("HoldMoving của lần chuyển chết phải bị gỡ: %v", svc2.Holds())
	}
	if !containsHold(svc2.Holds(), domain.HoldOffline) {
		t.Fatalf("HoldOffline phải sống tiếp: %v", svc2.Holds())
	}
	if !rig.settings.storagePaused(t) {
		t.Fatalf("cờ storage phải còn vì còn offline")
	}
}

// Người dùng tạm dừng bằng tay trong lúc hold không bị bật lại khi hold nhả.
func TestUserPauseSurvivesHoldRelease(t *testing.T) {
	rig := newTestRig(t)
	if err := rig.settings.Set(context.Background(), domain.KeyQueuePaused, true); err != nil {
		t.Fatalf("user pause: %v", err)
	}
	rig.svc.acquireHold(context.Background(), domain.HoldOffline)
	rig.svc.releaseHold(context.Background(), domain.HoldOffline)
	if !rig.settings.userPaused(t) {
		t.Fatalf("cờ người dùng bị tắt theo hold")
	}
	if rig.settings.storagePaused(t) {
		t.Fatalf("cờ storage phải hạ khi hết hold")
	}
}

/* ----------------- move hỏng: hiện ở failed, thử lại và gác được ----------------- */

// Lần chuyển hỏng/dở hiện ở move.failed, thử lại được, gác được — không khoá
// màn Lưu trữ như một lần đang chạy.
func TestFailedMoveExposedRetryableDismissable(t *testing.T) {
	rig := newTestRig(t)
	// Seed một lần hỏng từ "lần chạy trước" qua settings như sau reconcile.
	mount := rig.mkTarget(t, "dest", Volume{
		Name: "Máy", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{18, 18},
	})
	target := filepath.Join(mount, ProjectsFolderName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := rig.settings.Set(context.Background(), domain.KeyStorageMoveFailed,
		domain.StorageMoveFailed{TaskID: "t_old", Target: target, Phase: "Chuyển không xong", At: rig.clock.Now()}); err != nil {
		t.Fatalf("failed: %v", err)
	}
	svc2 := rig.restart(t)
	info := svc2.MoveInfo()
	if info.Active {
		t.Fatalf("không có worker mà báo active")
	}
	if info.Failed == nil || info.Failed.Target != target {
		t.Fatalf("failed = %+v", info.Failed)
	}
	// Retry: chạy lại về đúng đích cũ.
	taskID, done, err := svc2.RetryMove(context.Background())
	if err != nil {
		t.Fatalf("RetryMove: %v", err)
	}
	if done || taskID == "" {
		t.Fatalf("retry phải trả task nền: %q done=%v", taskID, done)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		svc2.mu.Lock()
		moving := svc2.moving
		svc2.mu.Unlock()
		if moving == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := svc2.Root(); got != target {
		t.Fatalf("vị trí sau retry = %q", got)
	}
	// Dismiss (không còn failed) trả lỗi nhẹ.
	if err := svc2.DismissMove(context.Background()); CodeOf(err) != "" {
		t.Fatalf("Dismiss: %v", err)
	}
	if svc2.MoveInfo().Failed != nil {
		t.Fatalf("failed phải được quên")
	}
}

/* ----------------- usage loop chỉ đi cây khi có tín hiệu ----------------- */

// Idle không đi cây: wake-loop chỉ chạy khi có Changed/check, không định kỳ.
func TestUsageLoopWalksOnlyOnSignal(t *testing.T) {
	rig := newTestRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rig.svc.refreshUsageLoop(ctx)
	putUsed(t, rig.root, 4<<20)
	time.Sleep(60 * time.Millisecond) // idle: không ai được đi cây
	before, _ := rig.svc.Snapshot(context.Background())
	putUsed(t, rig.root, 4<<20)
	time.Sleep(30 * time.Millisecond)
	after, _ := rig.svc.Snapshot(context.Background())
	if after.Projects.UsedBytes != before.Projects.UsedBytes {
		t.Fatalf("idle mà số đổi — đã đi cây ngoài wake")
	}
}

// Mọi cập nhật tiến độ đều qua nhịp 250ms — không cửa hậu cho đổi nhóm/phase.
func TestProgressHonoursHardCap(t *testing.T) {
	rig := newTestRig(t, func(d *Deps) { d.Clock = nil })
	// Nhiều nhóm nhỏ: 4 nhóm + other, mỗi nhóm vài tệp.
	for _, g := range []string{"images", "editor", "sources", "exports"} {
		for i := range 3 {
			writeFile(t, filepath.Join(rig.root, g, fmt.Sprintf("f%d.bin", i)), "x")
		}
	}
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1 << 30, id: [2]int32{19, 19},
	})
	start := time.Now()
	if _, _, err := rig.svc.Move(context.Background(), target, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rig.waitIdle(t)
	elapsed := time.Since(start)
	rig.tasks.mu.Lock()
	updates := len(rig.tasks.updates)
	rig.tasks.mu.Unlock()
	// Trần thật: elapsed/250ms + tối đa 2 (lần đầu + flush cuối).
	allowed := int(elapsed/progressEvery) + 2
	if updates > allowed {
		t.Fatalf("%d cập nhật trong %v vượt trần %d (đổi nhóm/phase không được qua nhịp)", updates, elapsed, allowed)
	}
	t.Logf("%d cập nhật tiến độ trong %v (trần %d)", updates, elapsed, allowed)
}
