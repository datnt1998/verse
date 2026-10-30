package retention

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Nguồn: yt-factory src/server/retention.ts (dọn tạm lúc mở + mỗi 6 giờ, xem
// trước rồi mới xoá).

func TestTempDirsOnlyStaleAndUnprotected(t *testing.T) {
	rig := newRetentionRig(t)
	stale := filepath.Join(rig.tmp, "viderse-image-call-old")
	fresh := filepath.Join(rig.tmp, "viderse-image-call-new")
	protected := filepath.Join(rig.tmp, "viderse-image-call-live")
	other := filepath.Join(rig.tmp, "someone-else")
	for _, dir := range []string{stale, fresh, protected, other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeFileT(t, filepath.Join(dir, "payload.bin"), "abc")
	}
	touchOld(t, stale, 24*time.Hour)
	touchOld(t, protected, 24*time.Hour)
	touchOld(t, other, 24*time.Hour)
	rig.svc.deps.Receipts = fakeReceipts{dirs: []string{protected}}

	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if preview.Freed == 0 {
		t.Fatalf("phải có byte để dọn")
	}
	row, ok := itemByLabel(preview.Items, labelTemp)
	if !ok || row.Count != 1 {
		t.Fatalf("temp row = %+v, muốn 1 thư mục", row)
	}

	if _, err := rig.svc.RunCleanup(context.Background(), preview.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	for _, path := range []string{stale} {
		if exists(path) {
			t.Fatalf("%s phải bị xoá", path)
		}
	}
	for _, path := range []string{fresh, protected, other} {
		if !exists(path) {
			t.Fatalf("%s phải được giữ", path)
		}
	}
}

func TestCodexHomeCandidates(t *testing.T) {
	rig := newRetentionRig(t)
	oldHome := filepath.Join(rig.config, "cache", "codex-home", "call-abc")
	freshHome := filepath.Join(rig.config, "cache", "codex-home", "call-new")
	for _, dir := range []string{oldHome, freshHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	touchOld(t, oldHome, 12*time.Hour)
	candidates, err := rig.svc.codexHomeCandidates(context.Background())
	if err != nil {
		t.Fatalf("codexHomeCandidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Item.Path != oldHome {
		t.Fatalf("candidates = %+v", candidates)
	}
}

func TestExportRequestCandidates(t *testing.T) {
	rig := newRetentionRig(t)
	requests := filepath.Join(rig.root, "exports", "requests")
	oldDone := filepath.Join(requests, "done-old.json")
	newDone := filepath.Join(requests, "done-new.json")
	running := filepath.Join(requests, "running.json")
	oldAt := rig.now.Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	newAt := rig.now.Add(-time.Hour).Format(time.RFC3339)
	writeFileT(t, oldDone, `{"id":"done-old","status":"done","createdAt":"`+oldAt+`","finishedAt":"`+oldAt+`"}`)
	writeFileT(t, newDone, `{"id":"done-new","status":"done","createdAt":"`+newAt+`","finishedAt":"`+newAt+`"}`)
	writeFileT(t, running, `{"id":"running","status":"running","createdAt":"`+oldAt+`"}`)

	profiles := filepath.Join(rig.root, "exports", "profiles")
	writeFileT(t, filepath.Join(profiles, "done-old", "Cookies"), "x")
	writeFileT(t, filepath.Join(profiles, "running", "Cookies"), "x")

	candidates, err := rig.svc.exportRequestCandidates(context.Background())
	if err != nil {
		t.Fatalf("exportRequestCandidates: %v", err)
	}
	got := map[string]bool{}
	for _, c := range candidates {
		got[filepath.Base(c.Item.Path)] = true
	}
	if !got["done-old.json"] || !got["done-old"] {
		t.Fatalf("thiếu bản ghi/profile đã xong quá hạn: %v", got)
	}
	if got["done-new.json"] || got["running.json"] || got["running"] {
		t.Fatalf("xoá nhầm mục còn sống: %v", got)
	}
}

func TestStagedExportCandidates(t *testing.T) {
	rig := newRetentionRig(t)
	dir := filepath.Join(rig.root, "exports", "v1")
	oldAt := rig.now.Add(-30 * time.Hour).Format(time.RFC3339)
	newAt := rig.now.Add(-time.Hour).Format(time.RFC3339)
	veryOld := rig.now.Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	writeFileT(t, filepath.Join(dir, "old", "export.json"), `{"status":"staged","createdAt":"`+oldAt+`"}`)
	writeFileT(t, filepath.Join(dir, "fresh", "export.json"), `{"status":"staged","createdAt":"`+newAt+`"}`)
	writeFileT(t, filepath.Join(dir, "done", "export.json"), `{"status":"done","createdAt":"`+veryOld+`"}`)

	candidates, err := rig.svc.stagedExportCandidates(context.Background())
	if err != nil {
		t.Fatalf("stagedExportCandidates: %v", err)
	}
	if len(candidates) != 1 || filepath.Base(candidates[0].Item.Path) != "old" {
		t.Fatalf("candidates = %+v", candidates)
	}
}

func TestPreviewRequiresMatchingID(t *testing.T) {
	rig := newRetentionRig(t)
	dir := filepath.Join(rig.tmp, "viderse-dl-old")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	touchOld(t, dir, 24*time.Hour)
	if _, err := rig.svc.RunCleanup(context.Background(), "nope"); err == nil {
		t.Fatalf("phải đòi preview trước")
	}
	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if _, err := rig.svc.RunCleanup(context.Background(), "khác"); err == nil {
		t.Fatalf("previewId cũ phải bị từ chối")
	}
	if _, err := rig.svc.RunCleanup(context.Background(), preview.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if _, err := rig.svc.RunCleanup(context.Background(), preview.ID); err == nil {
		t.Fatalf("preview đã dùng phải bị từ chối")
	}
}

func TestPreviewReportsProcessesAndLastRun(t *testing.T) {
	rig := newRetentionRig(t)
	rig.reaper.dead = []string{"yt-dlp ở video 3"}
	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	row, ok := itemByLabel(preview.Items, "tiến trình chạy dở bị bỏ quên")
	if !ok || row.Count != 1 || !row.Stopped {
		t.Fatalf("row process = %+v", row)
	}
	if _, err := rig.svc.RunCleanup(context.Background(), preview.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	var lastRun string
	found, err := rig.settings.Get(context.Background(), domain.KeyStorageCleanAt, &lastRun)
	if err != nil || !found || lastRun != rig.now.Format(time.RFC3339) {
		t.Fatalf("mốc dọn cuối = %q (found=%v, err=%v)", lastRun, found, err)
	}
	if rig.tasks.started != 1 || rig.tasks.finished != 1 {
		t.Fatalf("việc nền dọn = %d/%d", rig.tasks.started, rig.tasks.finished)
	}
}

func TestAutoSweepSkipsMedia(t *testing.T) {
	rig := newRetentionRig(t)
	mediaID := "11111111-2222-3333-4444-555555555555"
	mediaDir := filepath.Join(rig.root, "editor", "media", mediaID)
	writeFileT(t, filepath.Join(mediaDir, "metadata.json"), `{"contentHash":"x","createdAt":1}`)
	writeFileT(t, filepath.Join(mediaDir, "source.mp4"), "x")
	touchOld(t, mediaDir, 2*time.Hour)

	rig.svc.RunAuto(context.Background())
	if !exists(mediaDir) {
		t.Fatalf("dọn tự động không được đụng media chưa tham chiếu")
	}

	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if row, ok := itemByLabel(preview.Items, labelMedia); !ok || row.Count != 1 {
		t.Fatalf("dọn tay phải thấy media: %+v", preview.Items)
	}
}
