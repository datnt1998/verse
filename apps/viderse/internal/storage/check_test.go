package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Nguồn: yt-factory src/server/storage* (kiểm tra trước khi chuyển thư mục),
// hợp đồng dữ liệu §23.

// putUsed ghi một tệp để thư mục dự án có dung lượng đã dùng.
func putUsed(t *testing.T, root string, n int) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sources", "v1", "source.mp4"), strings.Repeat("a", n))
}

func checkOf(t *testing.T, result CheckResult, kind CheckKind) Check {
	t.Helper()
	for _, c := range result.Checks {
		if c.Kind == kind {
			return c
		}
	}
	t.Fatalf("thiếu kiểm tra %q trong %+v", kind, result.Checks)
	return Check{}
}

func TestCheckAllPassOnSameVolume(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 4<<20)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 200 << 30, id: [2]int32{1, 1}, // cùng fsid với ổ hiện tại
	})

	result, err := rig.svc.Check(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.OK {
		t.Fatalf("mong đợi ok, có kiểm tra hỏng: %+v", result.Checks)
	}
	wantNeed := int64(4<<20) * 11 / 10
	if result.NeedBytes != wantNeed {
		t.Fatalf("needBytes = %d, muốn %d (10%% dự phòng của 4 MiB)", result.NeedBytes, wantNeed)
	}
	if result.ETASec != nil {
		t.Fatalf("cùng ổ không có ETA, nhận %d", *result.ETASec)
	}
	for _, kind := range []CheckKind{CheckSpace, CheckWrite, CheckCloud, CheckFS, CheckSameVolume} {
		if got := checkOf(t, result, kind); got.Status != CheckOK {
			t.Fatalf("%s: %+v, muốn ok", kind, got)
		}
	}
	if text := checkOf(t, result, CheckSameVolume).Text; !strings.Contains(text, "Cùng ổ") {
		t.Fatalf("câu cùng ổ = %q", text)
	}
}

func TestCheckDifferentVolumeNeedsSpaceAndETA(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 8<<20)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1 << 30, id: [2]int32{9, 9},
	})
	result, err := rig.svc.Check(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.OK {
		t.Fatalf("8 MiB trên 1 GiB phải đủ: %+v", result.Checks)
	}
	if result.ETASec == nil {
		t.Fatalf("khác ổ phải có ETA")
	}
	if got := checkOf(t, result, CheckSameVolume); got.Status != CheckOK || !strings.Contains(got.Text, "Khác ổ") {
		t.Fatalf("câu khác ổ = %+v", got)
	}
}

func TestCheckFailsWhenSpaceTooSmall(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 4<<20)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "USB", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 1024, id: [2]int32{2, 2},
	})
	result, err := rig.svc.Check(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.OK {
		t.Fatalf("không đủ chỗ mà vẫn ok")
	}
	if got := checkOf(t, result, CheckSpace); got.Status != CheckFail {
		t.Fatalf("space = %+v, muốn fail", got)
	}
}

func TestCheckNewOnlyNeedsTwentyGigabytes(t *testing.T) {
	rig := newTestRig(t)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 10 << 30, id: [2]int32{3, 3},
	})
	result, err := rig.svc.Check(context.Background(), target, ModeNewOnly)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.NeedBytes != 20<<30 {
		t.Fatalf("needBytes = %d, muốn 20 GiB", result.NeedBytes)
	}
	if result.OK {
		t.Fatalf("10 GiB < 20 GiB mà vẫn ok")
	}
	if got := checkOf(t, result, CheckSpace); got.Status != CheckFail || !strings.Contains(got.Text, "video mới") {
		t.Fatalf("câu chỗ cho video mới = %+v", got)
	}
}

func TestCheckRejectsUnwritableFolder(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 1024)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{4, 4},
	})
	if err := os.Chmod(target, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o755) })

	result, err := rig.svc.Check(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.OK {
		t.Fatalf("thư mục chỉ đọc mà vẫn ok")
	}
	if got := checkOf(t, result, CheckWrite); got.Status != CheckFail {
		t.Fatalf("write = %+v, muốn fail", got)
	}
}

func TestCheckRejectsCloudSyncFolder(t *testing.T) {
	rig := newTestRig(t)
	cloudRoot := filepath.Join(rig.home, "Library", "CloudStorage", "Dropbox")
	if err := os.MkdirAll(cloudRoot, 0o755); err != nil {
		t.Fatalf("mkdir cloud: %v", err)
	}
	result, err := rig.svc.Check(context.Background(), filepath.Join(cloudRoot, "Viderse"), ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.OK {
		t.Fatalf("thư mục đồng bộ đám mây mà vẫn ok")
	}
	if got := checkOf(t, result, CheckCloud); got.Status != CheckFail {
		t.Fatalf("cloud = %+v, muốn fail", got)
	}
}

func TestCheckFilesystemVerdicts(t *testing.T) {
	cases := []struct {
		name   string
		fstype string
		want   CheckStatus
	}{
		{"fat32", "msdos", CheckFail},
		{"exfat", "exfat", CheckWarn},
		{"apfs", "apfs", CheckOK},
		{"smb", "smbfs", CheckOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rig := newTestRig(t)
			putUsed(t, rig.root, 1024)
			target := rig.mkTarget(t, "dest", Volume{
				Name: "Ổ ngoài", Kind: VolumeExternal, FS: displayFS(tc.fstype), FSType: tc.fstype,
				FreeBytes: 100 << 30, id: [2]int32{5, 5},
			})
			result, err := rig.svc.Check(context.Background(), target, ModeMove)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if got := checkOf(t, result, CheckFS); got.Status != tc.want {
				t.Fatalf("%s: %+v, muốn %s", tc.fstype, got, tc.want)
			}
			if tc.want == CheckFail && result.OK {
				t.Fatalf("%s phải làm hỏng kết quả chung", tc.fstype)
			}
		})
	}
}

func TestCheckVolumeRootCreatesSubfolder(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 1024)
	mount := rig.mkTarget(t, "Samsung T7", Volume{
		Name: "Samsung T7", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{6, 6},
	})
	result, err := rig.svc.Check(context.Background(), mount, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.CreateSubdir {
		t.Fatalf("chọn gốc ổ phải tạo thư mục con")
	}
	if want := filepath.Join(mount, ProjectsFolderName); result.Path != want {
		t.Fatalf("đích = %q, muốn %q", result.Path, want)
	}
}

func TestCheckRejectsRelativeAndCloudMove(t *testing.T) {
	rig := newTestRig(t)
	if _, err := rig.svc.Check(context.Background(), "somewhere/relative", ModeMove); CodeOf(err) != ErrInvalidPath {
		t.Fatalf("đường dẫn tương đối: %v", err)
	}
	if _, err := rig.svc.Check(context.Background(), rig.root, MoveMode("bogus")); CodeOf(err) != ErrInvalidPath {
		t.Fatalf("mode lạ: %v", err)
	}
}

func TestCheckUsesFreshUsage(t *testing.T) {
	rig := newTestRig(t)
	putUsed(t, rig.root, 2048)
	// Làm nóng cache bằng snapshot, rồi thêm dữ liệu: check phải thấy số mới.
	if _, err := rig.svc.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	putUsed(t, rig.root, 3<<20)
	target := rig.mkTarget(t, "dest", Volume{
		Name: "Ổ ngoài", Kind: VolumeExternal, FS: "APFS", FSType: "apfs",
		FreeBytes: 100 << 30, id: [2]int32{7, 7},
	})
	result, err := rig.svc.Check(context.Background(), target, ModeMove)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.NeedBytes < int64(3<<20) {
		t.Fatalf("needBytes = %d, phải tính cả dữ liệu mới", result.NeedBytes)
	}
}

var _ = time.Second

func TestCheckMissingVolumePath(t *testing.T) {
	rig := newTestRig(t)
	if _, err := rig.svc.Check(context.Background(), "/Volumes/KhongCoODau/Viderse", ModeMove); CodeOf(err) != ErrVolumeGone {
		t.Fatalf("ổ chưa cắm: %v", err)
	}
}
