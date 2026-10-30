package retention

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Nguồn: yt-factory src/server/retention.ts,
// src/editor/media-gc.ts — các kiểm tra an toàn đã port.

/* ----------------- pending: loại id thoát thư mục media ----------------- */

// JSON pending trên đĩa có id thoát ra khỏi thư mục media: bị loại khỏi chờ,
// không bao giờ được join thành đường xoá.
func TestPendingRejectsTraversalIDs(t *testing.T) {
	rig := newRetentionRig(t)
	media := filepath.Join(rig.root, "editor", "media")
	victim := filepath.Join(rig.root, "editor", "victim-dir")
	if err := os.MkdirAll(filepath.Join(media, "m-a"), 0o755); err != nil {
		t.Fatalf("media: %v", err)
	}
	touchOld(t, filepath.Join(media, "m-a"), 72*time.Hour)
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatalf("victim: %v", err)
	}
	writeFileT(t, filepath.Join(victim, "keep.bin"), "precious")
	// pending có id thoát đường và id hợp lệ.
	writeJSONT(t, rig.svc.pendingPath(), map[string]any{
		"ids": []string{"../victim-dir", "m-a", ".."},
	})
	if err := rig.svc.reclaimPending(context.Background()); err != nil {
		t.Fatalf("reclaimPending: %v", err)
	}
	if got := readFileT(t, filepath.Join(victim, "keep.bin")); got != "precious" {
		t.Fatalf("id thoát đường đã xoá thư mục khác")
	}
	if _, err := os.Lstat(filepath.Join(media, "m-a")); !os.IsNotExist(err) {
		t.Fatalf("id hợp lệ phải bị xoá")
	}
}

// pendingMedia() cũng chỉ nhận id an toàn.
func TestPendingMediaFiltersUnsafeIDs(t *testing.T) {
	rig := newRetentionRig(t)
	writeJSONT(t, rig.svc.pendingPath(), map[string]any{
		"ids": []string{"../x", "a/b", "m-ok", ""},
	})
	got := rig.svc.pendingMedia()
	if len(got) != 1 || !got["m-ok"] {
		t.Fatalf("pendingMedia = %v, chỉ được giữ m-ok", got)
	}
}

/* ----------------- ScratchDirs lỗi: giữ thư mục call-* ----------------- */

// ScratchDirs lỗi: thư mục call-* (có thể là phiên ảnh đã trả tiền) được giữ
// và lỗi nhóm hiện trên preview — không phải danh sách trống yên lặng.
func TestReceiptsErrorProtectsScratch(t *testing.T) {
	rig := newRetentionRig(t)
	rig.receipts.err = os.ErrPermission
	callDir := filepath.Join(rig.tmp, "viderse-image-call-paid")
	otherDir := filepath.Join(rig.tmp, "viderse-dl-old")
	for _, dir := range []string{callDir, otherDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeFileT(t, filepath.Join(dir, "x.bin"), "data")
		touchOld(t, dir, 24*time.Hour)
	}
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if len(prev.Errors) == 0 {
		t.Fatalf("lỗi receipt phải hiện trên preview")
	}
	// Run chỉ xoá nhóm không-liên-quan; call-* còn nguyên.
	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if _, err := os.Lstat(callDir); err != nil {
		t.Fatalf("call-* bị xoá khi receipt lỗi: %v", err)
	}
	if _, err := os.Lstat(otherDir); !os.IsNotExist(err) {
		t.Fatalf("thư mục temp khác phải bị xoá")
	}
}

/* ----------------- tham chiếu media qua root, fail closed ----------------- */

// Dự án ở ALT root giữ tham chiếu media: media ở root chính không bị xoá.
func TestHeldMediaAcrossRoots(t *testing.T) {
	rig := newRetentionRig(t)
	alt := filepath.Join(canonicalOr(t, t.TempDir()), "altroot")
	rig.roots.set(rig.root, []string{alt})
	mediaID := "11111111-2222-3333-4444-555555555555"
	media := filepath.Join(rig.root, "editor", "media", mediaID)
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatalf("media: %v", err)
	}
	writeFileT(t, filepath.Join(media, "f.png"), "img")
	old := time.Now().Add(-72 * time.Hour).UnixMilli()
	writeJSONT(t, filepath.Join(media, "metadata.json"), map[string]any{
		"createdAt": old, "lastUsedAt": old,
	})
	// Dự án trong alt root tham chiếu m-held.
	proj := filepath.Join(alt, "editor", "projects", "p1")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("proj: %v", err)
	}
	writeFileT(t, filepath.Join(proj, "project.json"), `{"media":["`+mediaID+`"]}`)
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	for _, it := range prev.Items {
		if it.Kind == "media" {
			t.Fatalf("media được giữ ở alt root mà vẫn lên preview: %+v", it)
		}
	}
}

// File tham chiếu không đọc được → nhóm media fail closed, không xoá gì.
func TestMediaFailsClosedOnUnreadableReference(t *testing.T) {
	rig := newRetentionRig(t)
	media := filepath.Join(rig.root, "editor", "media", "m-orphan")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatalf("media: %v", err)
	}
	writeFileT(t, filepath.Join(media, "f.png"), "img")
	old := time.Now().Add(-72 * time.Hour).UnixMilli()
	writeJSONT(t, filepath.Join(media, "metadata.json"), map[string]any{
		"createdAt": old, "lastUsedAt": old,
	})
	proj := filepath.Join(rig.root, "editor", "projects", "p1")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("proj: %v", err)
	}
	blocked := filepath.Join(proj, "locked.json")
	writeFileT(t, blocked, "{}")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o644) })
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if len(prev.Errors) == 0 {
		t.Fatalf("phải báo lỗi nhóm media")
	}
	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if _, err := os.Lstat(media); err != nil {
		t.Fatalf("media bị xoá dù tham chiếu không đọc được")
	}
}

/* ----------------- mục sau preview không bị run ăn theo ----------------- */

// Mục sinh ra SAU preview không bị run ăn theo; staged-export đã chuyển sang
// done sau preview cũng được giữ.
func TestRunDeletesOnlyPreviewedSet(t *testing.T) {
	rig := newRetentionRig(t)
	stale := filepath.Join(rig.tmp, "viderse-dl-old")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFileT(t, filepath.Join(stale, "x.bin"), "stale")
	touchOld(t, stale, 24*time.Hour)
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	// Thêm một mục đủ-điều-kiện-xoá SAU preview.
	fresh := filepath.Join(rig.tmp, "viderse-dl-new")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFileT(t, filepath.Join(fresh, "x.bin"), "fresh")
	touchOld(t, fresh, 24*time.Hour)

	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("mục đã preview phải bị xoá")
	}
	if _, err := os.Lstat(fresh); err != nil {
		t.Fatalf("mục sinh sau preview bị ăn theo")
	}
}

// Preview gắn root lúc dò: đổi root sau đó bắt buộc preview lại.
func TestRunRequiresPreviewAfterRootChange(t *testing.T) {
	rig := newRetentionRig(t)
	stale := filepath.Join(rig.tmp, "viderse-dl-old")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	touchOld(t, stale, 24*time.Hour)
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	rig.roots.set(filepath.Join(canonicalOr(t, t.TempDir()), "other-root"), nil)
	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); CodeOfRetention(err) != "preview_required" {
		t.Fatalf("đổi root mà run vẫn qua: %v", err)
	}
	if _, err := os.Lstat(stale); err != nil {
		t.Fatalf("run lậu đã xoá: %v", err)
	}
}

// PreviewId chỉ dùng một lần — cũ không xoá được lần hai.
func TestPreviewIDIsConsumed(t *testing.T) {
	rig := newRetentionRig(t)
	stale := filepath.Join(rig.tmp, "viderse-dl-old")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	touchOld(t, stale, 24*time.Hour)
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	stale2 := filepath.Join(rig.tmp, "viderse-dl-again")
	if err := os.MkdirAll(stale2, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	touchOld(t, stale2, 24*time.Hour)
	if _, err := rig.svc.RunCleanup(context.Background(), prev.ID); CodeOfRetention(err) != "preview_required" {
		t.Fatalf("previewId cũ dùng lại được: %v", err)
	}
	if _, err := os.Lstat(stale2); err != nil {
		t.Fatalf("previewId cũ đã xoá mục mới")
	}
}

// Staged-export chuyển sang done sau preview: run phải giữ nó (Validator).
func TestRunKeepsStagedExportThatFinished(t *testing.T) {
	rig := newRetentionRig(t)
	recDir := filepath.Join(rig.root, "exports", "video-1", "e-done")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	meta := filepath.Join(recDir, "export.json")
	writeJSONT(t, meta, map[string]any{
		"status":    "staged",
		"createdAt": time.Now().Add(-72 * time.Hour).Format(time.RFC3339),
	})
	writeFileT(t, filepath.Join(recDir, "out.mp4"), "video")
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	found := false
	for _, it := range prev.Items {
		if it.Kind == "staged-export" {
			found = true
		}
	}
	if !found {
		t.Fatalf("staged-export quá hạn phải lên preview: %+v", prev.Items)
	}
	// Giữa preview và run: xuất xong → đổi trạng thái.
	writeJSONT(t, meta, map[string]any{
		"status":    "done",
		"createdAt": time.Now().Add(-72 * time.Hour).Format(time.RFC3339),
	})
	res, err := rig.svc.RunCleanup(context.Background(), prev.ID)
	if err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(recDir, "out.mp4")); err != nil {
		t.Fatalf("bản xuất đã chốt bị xoá: %v", err)
	}
	if len(res.Kept) == 0 {
		t.Fatalf("mục giữ lại phải được báo: %+v", res)
	}
}

// Lỗi của một mục không nuốt: trả về trong Errors thay vì im lặng.
func TestRunReportsItemErrors(t *testing.T) {
	rig := newRetentionRig(t)
	parent := filepath.Join(rig.tmp, "viderse-dl-parent")
	stale := filepath.Join(parent, "inner")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFileT(t, filepath.Join(stale, "x.bin"), "x")
	touchOld(t, stale, 24*time.Hour)
	touchOld(t, parent, 24*time.Hour)
	prev, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	// Khoá thư mục cha ngay trước run: RemoveAll thất bại trên nhiều FS.
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	res, err := rig.svc.RunCleanup(context.Background(), prev.ID)
	if err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if len(res.Errors) == 0 && len(res.Kept) == 0 {
		t.Skip("FS cho phép xoá trong thư mục chỉ-đọc — không mô phỏng được lỗi")
	}
}

// helpers

func readFileT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	return string(data)
}

func writeJSONT(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func CodeOfRetention(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
