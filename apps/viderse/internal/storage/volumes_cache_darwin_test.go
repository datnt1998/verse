//go:build darwin

package storage

import (
	"context"
	"sync"
	"syscall"
	"testing"
	"time"
)

// countingVolumes là backend ổ đĩa đếm số lần bị hỏi — chứng minh GET không
// chạm vào backend sau khi snapshot đã ấm.
type countingVolumes struct {
	mu    sync.Mutex
	vols  []Volume
	list  int
	probe int
}

func (c *countingVolumes) List(context.Context) ([]Volume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list++
	return append([]Volume(nil), c.vols...), nil
}

func (c *countingVolumes) Info(_ context.Context, path string) (Volume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probe++
	best, bestLen := Volume{}, -1
	for _, v := range c.vols {
		if onMount(path, v.Mount) && len(v.Mount) > bestLen {
			best, bestLen = v, len(v.Mount)
		}
	}
	if bestLen < 0 {
		return Volume{}, codeError(ErrNotFound, "không tìm thấy ổ cho %s", path)
	}
	return best, nil
}

func (c *countingVolumes) add(v Volume) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vols = append(c.vols, v)
}

func (c *countingVolumes) calls() (list, probe int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.list, c.probe
}

// TestCachedVolumesNeverHitsBackendOnRead: sau khi ấm, mọi lần đọc (và cả khi
// đồng hồ nhảy xa qua mọi TTL cũ) không hỏi backend lần nào; ổ mới chỉ vào
// snapshot qua refreshVolumes của nhịp watch.
func TestCachedVolumesNeverHitsBackendOnRead(t *testing.T) {
	inner := &countingVolumes{vols: []Volume{{
		Name: "Macintosh HD", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 200 << 30, Mount: "/", id: [2]int32{1, 1},
	}}}
	clock := newFakeClock()
	cache := CachedVolumes(inner)
	r := cache.(volumeRefresher)
	ctx := context.Background()

	// Lần đầu chỉ dựng snapshot (không phải "đổi tập ổ" nên không phát sự kiện).
	if r.refreshVolumes(ctx) {
		t.Fatal("lần ấm đầu tiên không tính là đổi tập ổ")
	}
	if vols, _ := cache.List(ctx); len(vols) != 1 {
		t.Fatalf("snapshot phải có 1 ổ, nhận %d", len(vols))
	}
	for range 5 {
		clock.Advance(time.Hour)
		for range 40 {
			if _, err := cache.Info(ctx, "/Users/x/.viderse/projects"); err != nil {
				t.Fatalf("Info: %v", err)
			}
			if _, err := cache.List(ctx); err != nil {
				t.Fatalf("List: %v", err)
			}
		}
	}
	if list, probe := inner.calls(); list != 1 || probe != 0 {
		t.Fatalf("đọc sau khi ấm phải không chạm backend: list=%d probe=%d", list, probe)
	}

	// Chưa có nhịp watch: ổ vừa cắm chưa vào snapshot.
	inner.add(Volume{Name: "Samsung T7", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1500 << 30, Mount: "/Volumes/Samsung T7", id: [2]int32{2, 2}})
	if vol, _ := cache.Info(ctx, "/Volumes/Samsung T7/Viderse Projects"); vol.Kind != VolumeInternal {
		t.Fatalf("trước nhịp watch vẫn phải trả snapshot cũ, nhận %+v", vol)
	}
	if !r.refreshVolumes(ctx) {
		t.Fatal("nhịp watch phải thấy tập ổ đổi")
	}
	if r.refreshVolumes(ctx) {
		t.Fatal("refresh khi tập ổ không đổi phải trả false (không phát sự kiện thừa)")
	}
	vol, err := cache.Info(ctx, "/Volumes/Samsung T7/Viderse Projects")
	if err != nil || vol.Mount != "/Volumes/Samsung T7" || vol.Kind != VolumeExternal {
		t.Fatalf("sau nhịp watch phải thấy ổ mới: %+v err=%v", vol, err)
	}
	if list, probe := inner.calls(); list != 3 || probe != 0 {
		t.Fatalf("Info phải trả từ snapshot: list=%d probe=%d", list, probe)
	}
}

// TestVolumeWatchPublishesMountChange: nhịp watch phân loại ổ mới và phát sự
// kiện storage đúng một lần cho mỗi lần tập ổ đổi.
func TestVolumeWatchPublishesMountChange(t *testing.T) {
	inner := &countingVolumes{vols: []Volume{{
		Name: "Macintosh HD", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 200 << 30, Mount: "/", id: [2]int32{1, 1},
	}}}
	var mu sync.Mutex
	events := 0
	rig := newTestRig(t, func(d *Deps) {
		d.Volumes = CachedVolumes(inner)
		d.Emit = func(string) { mu.Lock(); events++; mu.Unlock() }
	})
	ctx := context.Background()
	rig.svc.tick(ctx)
	if got := events; got != 0 {
		t.Fatalf("nhịp đầu không đổi ổ thì không phát sự kiện, nhận %d", got)
	}
	inner.add(Volume{Name: "Samsung T7", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1500 << 30, Mount: "/Volumes/Samsung T7", id: [2]int32{2, 2}})
	rig.svc.tick(ctx)
	rig.svc.tick(ctx)
	if got := events; got != 1 {
		t.Fatalf("phải phát đúng một sự kiện khi ổ cắm vào, nhận %d", got)
	}
}

// fakeStatfs gán statfs giả: mọi path dưới /Volumes/Samsung T7 là ổ ngoài,
// còn lại là ổ trong "/".
func fakeStatfs(path string, st *syscall.Statfs_t) error {
	*st = syscall.Statfs_t{
		Bsize: 4096, Bavail: 1000,
		Fstypename: [16]int8{'a', 'p', 'f', 's'},
	}
	if onMount(path, "/Volumes/Samsung T7") {
		st.Fsid = syscall.Fsid{Val: [2]int32{2, 2}}
		copyName(st.Mntonname[:], "/Volumes/Samsung T7")
		return nil
	}
	st.Fsid = syscall.Fsid{Val: [2]int32{1, 1}}
	copyName(st.Mntonname[:], "/")
	return nil
}

// copyName ghi một chuỗi vào mảng int8 kết thúc-NUL của statfs.
func copyName(dst []int8, s string) {
	for i := 0; i < len(s) && i < len(dst); i++ {
		dst[i] = int8(s[i])
	}
}

// TestSystemVolumesClassifiesOnce: diskutil chạy một lần cho mỗi fsid; probe
// của GET chỉ statfs, không bao giờ gọi diskutil — kể cả ổ vừa cắm.
func TestSystemVolumesClassifiesOnce(t *testing.T) {
	spawns := 0
	sys := &systemVolumes{
		home:   "/Users/x",
		statfs: fakeStatfs,
		diskutil: func(context.Context, string) (diskutilInfo, error) {
			spawns++
			return diskutilInfo{Internal: false, VolumeName: "Samsung T7", BusProtocol: "USB"}, nil
		},
	}
	ctx := context.Background()
	if _, err := sys.probe("/Volumes/Samsung T7/Viderse Projects"); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if spawns != 0 {
		t.Fatalf("probe (đường GET) không được gọi diskutil, nhận %d", spawns)
	}
	first, err := sys.volumeFromMount(ctx, "/Volumes/Samsung T7")
	if err != nil {
		t.Fatalf("volumeFromMount: %v", err)
	}
	if first.Name != "Samsung T7" || first.Kind != VolumeExternal || first.Bus != "USB" {
		t.Fatalf("phân loại sai: %+v", first)
	}
	// Probe (đường GET) sau khi đã phân loại mang cùng tên/loại/cổng từ bộ nhớ.
	if got, err := sys.probe("/Volumes/Samsung T7/Viderse Projects"); err != nil || got.Bus != "USB" || got.Kind != VolumeExternal {
		t.Fatalf("probe sau phân loại = %+v, %v", got, err)
	}
	for range 20 {
		if _, err := sys.volumeFromMount(ctx, "/Volumes/Samsung T7"); err != nil {
			t.Fatalf("volumeFromMount: %v", err)
		}
	}
	if spawns != 1 {
		t.Fatalf("diskutil phải chạy đúng một lần cho mỗi fsid, nhận %d", spawns)
	}
}
