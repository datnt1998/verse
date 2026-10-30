package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Nguồn: yt-factory src/server/storage* + yêu cầu hiệu năng của V22 (GET
// /api/storage không đi cây; watch một stat mỗi nhịp).

func TestSnapshotServesFromCache(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 1024)
	before, err := rig.svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	// Thêm dữ liệu: GET không được đi cây nên số cũ vẫn nguyên.
	putUsed(t, rig.root, 4<<20)
	again, err := rig.svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if again.Projects.UsedBytes != before.Projects.UsedBytes {
		t.Fatalf("GET đã đi cây: %d → %d", before.Projects.UsedBytes, again.Projects.UsedBytes)
	}
	// Vòng nền đo lại thì số mới xuất hiện.
	rig.svc.refreshUsage(context.Background())
	after, err := rig.svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if after.Projects.UsedBytes <= before.Projects.UsedBytes {
		t.Fatalf("đo lại không thấy dữ liệu mới: %d", after.Projects.UsedBytes)
	}
}

func TestUsageRefreshHonoursContext(t *testing.T) {
	rig := newTestRig(t)
	for i := range 200 {
		writeFile(t, filepath.Join(rig.root, "images", fmt.Sprintf("d%03d", i), "a.png"), "x")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		rig.svc.refreshUsage(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("đo lại không dừng theo ctx")
	}
}

func TestWatchTickCostsOneStatPerRoot(t *testing.T) {
	rig := newTestRig(t)
	rig.svc.tick(context.Background())
	const ticks = 200
	start := time.Now()
	for range ticks {
		rig.svc.tick(context.Background())
	}
	elapsed := time.Since(start)
	per := elapsed / ticks
	t.Logf("watch: %d nhịp %v, mỗi nhịp %v", ticks, elapsed, per)
	if per > time.Millisecond {
		t.Fatalf("nhịp watch %v quá đắt cho idle (một stat + một access mỗi root)", per)
	}
}

func TestMoveProgressAtMostFourPerSecond(t *testing.T) {
	// Nhịp gom tiến độ dùng đồng hồ thật (đồng hồ giả đứng yên sẽ phát mọi
	// khối 1 MiB — không phải điều cần đo).
	rig := newTestRig(t, func(d *Deps) { d.Clock = nil })
	// 64 MiB để lần chép kéo dài đủ vài nhịp throttle.
	payload := make([]byte, 1<<20)
	for i := range payload {
		payload[i] = byte(i)
	}
	big := filepath.Join(rig.root, "sources", "v1", "big.bin")
	if err := os.MkdirAll(filepath.Dir(big), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for range 64 {
		if _, err := f.Write(payload); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Đích đã có dữ liệu để đi nhánh chép (không rename).
	dest := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1 << 30, id: [2]int32{20, 20},
	})
	if err := os.MkdirAll(filepath.Join(dest, "existing"), 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	writeFile(t, filepath.Join(dest, "existing", "keep.txt"), "x")

	start := time.Now()
	if _, _, err := rig.svc.Move(context.Background(), dest, ModeMove); err != nil {
		t.Fatalf("Move: %v", err)
	}
	rig.waitIdle(t)
	elapsed := time.Since(start)

	rig.tasks.mu.Lock()
	updates := len(rig.tasks.updates)
	rig.tasks.mu.Unlock()
	allowed := int(elapsed/progressEvery) + 2*len(Groups) + 4
	t.Logf("move 64 MiB: %v, %d cập nhật tiến độ (trần %d, %.1f/s)",
		elapsed, updates, allowed, float64(updates)/elapsed.Seconds())
	if updates > allowed {
		t.Fatalf("%d cập nhật tiến độ vượt trần %d trong %v", updates, allowed, elapsed)
	}
	// Tiến độ cuối phải phản ánh tổng byte.
	rig.tasks.mu.Lock()
	last := rig.tasks.updates[len(rig.tasks.updates)-1]
	rig.tasks.mu.Unlock()
	if last.Progress == nil || last.Progress.Total == 0 {
		t.Fatalf("thiếu tiến độ cuối: %+v", last)
	}
}

// TestSnapshotP95LargeTree đo p95 của đường đọc GET /api/storage trên cây
// 20 000 tệp: phục vụ từ bộ nhớ nên không phụ thuộc kích thước cây.
func TestSnapshotP95LargeTree(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture 20k tệp")
	}
	rig := newTestRig(t)
	build := func(group string, files int) {
		for i := range files {
			dir := filepath.Join(rig.root, group, fmt.Sprintf("d%04d", i/50))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("f%04d.bin", i)))
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			// Sparse: đặt kích thước 1 KiB mà không ghi thật.
			if err := f.Truncate(1024); err != nil {
				t.Fatalf("truncate: %v", err)
			}
			_ = f.Close()
		}
	}
	build("images", 6000)
	build("editor", 6000)
	build("sources", 4000)
	build("exports", 4000)

	rig.svc.forceUsage(context.Background())
	if got := rig.svc.usageOf().used; got < 20_000*1024 {
		t.Fatalf("đo thiếu: %d", got)
	}

	const requests = 200
	durations := make([]time.Duration, 0, requests)
	for range requests {
		start := time.Now()
		if _, err := rig.svc.Snapshot(context.Background()); err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[len(durations)/2]
	p95 := durations[len(durations)*95/100]
	t.Logf("GET /api/storage trên 20k tệp: p50=%v p95=%v (200 request)", p50, p95)
	if p95 > 20*time.Millisecond {
		t.Fatalf("p95 %v vượt 20ms", p95)
	}
}

func TestChangedEmitsAfterRefresh(t *testing.T) {
	rig := newTestRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go rig.svc.refreshUsageLoop(ctx) // vòng số đo nền (Start chặn ở vòng watch)
	before, err := rig.svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	putUsed(t, rig.root, 8<<20)
	rig.svc.Changed("ref-1")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		after, err := rig.svc.Snapshot(context.Background())
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if after.Projects.UsedBytes > before.Projects.UsedBytes {
			// Sự kiện chỉ phát sau khi số đo mới: lúc này refs đã rỗng.
			rig.svc.usageMu.Lock()
			pending := len(rig.svc.pendingRefs)
			rig.svc.usageMu.Unlock()
			if pending != 0 {
				t.Fatalf("còn %d ref chưa phát", pending)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Changed không làm mới số đo trong 5s")
}

func TestPreviousModeMoveVsNewOnly(t *testing.T) {
	rig := newTestRig(t)
	seedProject(t, rig.root)
	ext := rig.mkTarget(t, "ext", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1 << 30, id: [2]int32{21, 21},
	})
	if _, _, err := rig.svc.Move(context.Background(), ext, ModeNewOnly); err != nil {
		t.Fatalf("Move new_only: %v", err)
	}
	rig.waitIdle(t)
	var prev domain.StoragePrevious
	if found, err := rig.settings.Get(context.Background(), domain.KeyStoragePrevious, &prev); err != nil || !found {
		t.Fatalf("new_only phải ghi bản cũ để màn donenew có path/bytes: found=%v err=%v", found, err)
	}
	if prev.Mode != ModeNewOnly || !prev.DeleteAfter.IsZero() || prev.Path != rig.root {
		t.Fatalf("previous = %+v", prev)
	}
	var last domain.StorageSwitch
	if found, _ := rig.settings.Get(context.Background(), domain.KeyStorageLastSwitch, &last); !found || last.Mode != ModeNewOnly {
		t.Fatalf("lastSwitch = %+v", last)
	}
	if err := rig.svc.DeletePrevious(context.Background()); CodeOf(err) != ErrNotDeletable {
		t.Fatalf("xoá bản cũ của new_only: %v", err)
	}
	// Quá 7 ngày cũng không tự xoá.
	rig.clock.Advance(30 * 24 * time.Hour)
	rig.svc.tick(context.Background())
	if _, err := os.Stat(filepath.Join(rig.root, "sources", "v1", "source.mp4")); err != nil {
		t.Fatalf("bản cũ của new_only bị xoá: %v", err)
	}
}
