package retention

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// Nguồn: yt-factory src/server/editor/media-gc.ts (sweepMediaOrphans).

const (
	mediaHeld    = "aaaaaaaa-1111-4111-8111-111111111111"
	mediaOrphan  = "bbbbbbbb-2222-4222-8222-222222222222"
	mediaOverlay = "cccccccc-3333-4333-8333-333333333333"
	mediaFresh   = "dddddddd-4444-4444-8444-444444444444"
)

func putMedia(t *testing.T, rig *retentionRig, id, meta string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(rig.root, "editor", "media", id)
	writeFileT(t, filepath.Join(dir, "metadata.json"), meta)
	writeFileT(t, filepath.Join(dir, "source.mp4"), "media-bytes")
	touchOld(t, dir, age)
	return dir
}

func TestMediaSweepOnlyUnreferencedAndOld(t *testing.T) {
	rig := newRetentionRig(t)
	putMedia(t, rig, mediaHeld, `{"createdAt":1}`, 2*time.Hour)
	orphanDir := putMedia(t, rig, mediaOrphan, `{"createdAt":1}`, 2*time.Hour)
	putMedia(t, rig, mediaOverlay, `{"createdAt":1,"overlay":true}`, 2*time.Hour)
	putMedia(t, rig, mediaFresh, `{"mime":"video/mp4"}`, time.Minute)
	writeFileT(t, filepath.Join(rig.root, "editor", "projects", "p1", "project.json"),
		`{"mediaItems":[{"id":"`+mediaHeld+`"}],"title":"x"}`)

	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	row, ok := itemByLabel(preview.Items, labelMedia)
	if !ok || row.Count != 1 {
		t.Fatalf("media row = %+v, muốn đúng media mồ côi", row)
	}

	result, err := rig.svc.RunCleanup(context.Background(), preview.ID)
	if err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("removed = %d, muốn 1", result.Removed)
	}
	if exists(orphanDir) {
		t.Fatalf("media mồ côi phải bị xoá")
	}
	for _, id := range []string{mediaHeld, mediaOverlay, mediaFresh} {
		if !exists(filepath.Join(rig.root, "editor", "media", id)) {
			t.Fatalf("media %s phải được giữ", id)
		}
	}
}

func TestMediaSweepDropsIndexEntry(t *testing.T) {
	rig := newRetentionRig(t)
	putMedia(t, rig, mediaOrphan, `{"createdAt":1}`, 2*time.Hour)
	indexPath := filepath.Join(rig.root, "editor", "media-index.json")
	writeFileT(t, indexPath, `{"version":"1","media":{"`+mediaOrphan+`":{"fileName":"a.mp4"},"other":{"fileName":"b.mp4"}}}`)

	preview, err := rig.svc.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatalf("PreviewCleanup: %v", err)
	}
	if _, err := rig.svc.RunCleanup(context.Background(), preview.ID); err != nil {
		t.Fatalf("RunCleanup: %v", err)
	}
	var index struct {
		Media map[string]any `json:"media"`
	}
	if ok := readJSON(indexPath, &index); !ok {
		t.Fatalf("index phải còn đọc được")
	}
	if _, ok := index.Media[mediaOrphan]; ok {
		t.Fatalf("dòng index của media đã xoá còn đó")
	}
	if _, ok := index.Media["other"]; !ok {
		t.Fatalf("index của media khác bị xoá nhầm")
	}
}

func TestMediaHeldByRunningVideo(t *testing.T) {
	rig := newRetentionRig(t)
	dir := putMedia(t, rig, mediaOrphan, `{"createdAt":1}`, 2*time.Hour)
	rig.videos.held = map[string]bool{mediaOrphan: true}
	candidates, err := rig.svc.mediaCandidates(context.Background())
	if err != nil {
		t.Fatalf("mediaCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("video đang giữ thì không được dọn: %+v", candidates)
	}
	if !exists(dir) {
		t.Fatalf("thư mục phải còn")
	}
}

func TestPendingMediaRetriedLater(t *testing.T) {
	rig := newRetentionRig(t)
	dir := filepath.Join(rig.root, "editor", "media", mediaOrphan)
	writeFileT(t, filepath.Join(dir, "metadata.json"), `{"createdAt":1,"lastUsedAt":`+"1"+`}`)
	writeFileT(t, filepath.Join(dir, "source.mp4"), "x")
	touchOld(t, dir, 2*time.Hour)
	if err := rig.svc.writePending(map[string]bool{mediaOrphan: true}); err != nil {
		t.Fatalf("writePending: %v", err)
	}

	rig.svc.RunAuto(context.Background())
	if exists(dir) {
		t.Fatalf("media chờ dọn quá hạn phải được xoá ở lần dọn tự động")
	}
	if exists(rig.svc.pendingPath()) {
		t.Fatalf("danh sách chờ phải bị xoá khi rỗng")
	}
}

func TestPendingMediaKeptWhileHeld(t *testing.T) {
	rig := newRetentionRig(t)
	dir := putMedia(t, rig, mediaOrphan, `{"createdAt":1}`, 2*time.Hour)
	if err := rig.svc.writePending(map[string]bool{mediaOrphan: true}); err != nil {
		t.Fatalf("writePending: %v", err)
	}
	rig.videos.held = map[string]bool{mediaOrphan: true}
	rig.svc.RunAuto(context.Background())
	if !exists(dir) {
		t.Fatalf("còn bị giữ thì không được xoá")
	}
	if !exists(rig.svc.pendingPath()) {
		t.Fatalf("danh sách chờ phải còn")
	}
}
