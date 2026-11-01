package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// 06-copy › Lưu trữ trên máy (#79): chỗ trống luôn là của ổ đang giữ thư mục
// lưu dự án và luôn kèm tên ổ. Ổ ngoài rút thì KHÔNG được lấy số của ổ trong
// máy — API trả null chỗ trống (wire ở internal/web) và tên ổ thật.
func TestSnapshotKeepsExternalVolumeNameWhenDriveUnplugged(t *testing.T) {
	ctx := context.Background()
	const root = "/Volumes/Samsung T7/Studio 2026/Kênh YouTube/Viderse Projects"
	rig := newTestRig(t, func(d *Deps) {
		settings := newFakeSettings()
		if err := settings.Set(ctx, domain.KeyStorageLocation, root); err != nil {
			t.Fatalf("Set location: %v", err)
		}
		d.Settings = settings
		// Ổ ngoài đã rút: mọi path dưới /Volumes rơi về ổ trong máy (đúng hành
		// vi statfs thật) — Snapshot không được báo đó là ổ chứa dự án.
		d.Volumes = &fakeVolumes{vols: []Volume{{
			Name: "Macintosh HD", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
			FreeBytes: 212 << 30, Mount: "/", id: [2]int32{1, 1},
		}}}
	})
	rig.svc.tick(ctx)

	snap, err := rig.svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Projects.Online {
		t.Fatalf("Projects.Online = true, muốn false (ổ đã rút)")
	}
	if got := snap.Projects.Volume.Name; got != "Samsung T7" {
		t.Fatalf("Volume.Name = %q, muốn \"Samsung T7\" (tên ổ theo đường dẫn)", got)
	}
	if got := snap.Projects.Volume.Kind; got != VolumeExternal {
		t.Fatalf("Volume.Kind = %q, muốn %q", got, VolumeExternal)
	}
	if got := snap.Projects.Volume.FreeBytes; got != 0 {
		t.Fatalf("Volume.FreeBytes = %d, muốn 0 (không đọc được chỗ trống)", got)
	}
	if got := snap.Projects.Volume.FreeBytes; got == 212<<30 {
		t.Fatalf("Volume.FreeBytes lấy số của ổ trong máy — sai nghĩa")
	}
}

// Khởi động lại khi ổ đã rút: tên ổ + hệ tệp phải còn (nhớ từ lần đọc được),
// không rơi về ổ trong máy.
func TestSnapshotRemembersVolumeInfoAcrossRestart(t *testing.T) {
	ctx := context.Background()
	base := canonicalOr(t, t.TempDir())
	drive := filepath.Join(base, "Samsung T7")
	root := filepath.Join(drive, "Viderse Projects")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	settings := newFakeSettings()
	if err := settings.Set(ctx, domain.KeyStorageLocation, root); err != nil {
		t.Fatalf("Set location: %v", err)
	}
	external := Volume{
		Name: "Samsung T7", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		Bus: "USB", FreeBytes: 1_540_000_000_000, Mount: drive, id: [2]int32{2, 2},
	}
	first := newTestRig(t, func(d *Deps) {
		d.Settings = settings
		d.Volumes = &fakeVolumes{vols: []Volume{external}}
	})
	first.svc.tick(ctx)
	if snap, err := first.svc.Snapshot(ctx); err != nil || !snap.Projects.Online || snap.Projects.Volume.Name != "Samsung T7" {
		t.Fatalf("lần đầu: snap=%+v err=%v", snap.Projects, err)
	}

	// Rút ổ: thư mục biến mất, backend chỉ còn ổ trong máy.
	if err := os.RemoveAll(drive); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	second := newTestRig(t, func(d *Deps) {
		d.Settings = settings
		d.Volumes = &fakeVolumes{vols: []Volume{{
			Name: "Macintosh HD", Kind: VolumeInternal, FS: "APFS", FSType: "apfs",
			FreeBytes: 212 << 30, Mount: "/", id: [2]int32{1, 1},
		}}}
	})
	second.svc.tick(ctx)
	snap, err := second.svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Projects.Online {
		t.Fatalf("Projects.Online = true, muốn false")
	}
	if got := snap.Projects.Volume.Name; got != "Samsung T7" {
		t.Fatalf("Volume.Name = %q, muốn \"Samsung T7\" (nhớ từ lần trước)", got)
	}
	if got := snap.Projects.Volume.Kind; got != VolumeExternal {
		t.Fatalf("Volume.Kind = %q, muốn %q", got, VolumeExternal)
	}
	if got := snap.Projects.Volume.FS; got != "APFS" {
		t.Fatalf("Volume.FS = %q, muốn \"APFS\" (nhớ từ lần trước)", got)
	}
	if got := snap.Projects.Volume.FreeBytes; got != 0 {
		t.Fatalf("Volume.FreeBytes = %d, muốn 0", got)
	}
}
