package retention

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Nguồn: yt-factory src/server/clone/source-cleanup.ts và deletePostedCloneExport
// (hai thẻ xoá riêng của design: video gốc của video đã xong, MP4 đã đăng).

const sourceMediaID = "eeeeeeee-5555-4555-8555-555555555555"

func TestReclaimListCountsBothCards(t *testing.T) {
	rig := newRetentionRig(t)
	writeFileT(t, filepath.Join(rig.root, "sources", "v1", "source.mp4"), "1234567890")
	writeFileT(t, filepath.Join(rig.root, "sources", "v1", "info.json"), `{"title":"x"}`)
	writeFileT(t, filepath.Join(rig.root, "exports", "v1", "export.mp4"), "12345")
	rig.videos.finished = []SourceVideo{{ID: "v1", ProjectID: "p1", SourceMediaID: sourceMediaID}}
	rig.videos.posted = []PostedVideo{{ID: "v1"}}

	items, err := rig.svc.ReclaimList(context.Background())
	if err != nil {
		t.Fatalf("ReclaimList: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	src, mp4 := items[0], items[1]
	if src.ID != ReclaimSource || src.Count != 1 || src.Bytes != 10 {
		t.Fatalf("thẻ src = %+v", src)
	}
	if mp4.ID != ReclaimMP4 || mp4.Count != 1 || mp4.Bytes != 5 {
		t.Fatalf("thẻ mp4 = %+v", mp4)
	}
	if src.Title == "" || src.Desc == "" || mp4.Title == "" || mp4.Desc == "" {
		t.Fatalf("thiếu câu hiển thị: %+v", items)
	}
}

func TestReclaimSourceKeepsEvidenceAndDropsMedia(t *testing.T) {
	rig := newRetentionRig(t)
	srcDir := filepath.Join(rig.root, "sources", "v1")
	writeFileT(t, filepath.Join(srcDir, "source.mp4"), "video")
	writeFileT(t, filepath.Join(srcDir, "info.json"), `{"title":"x"}`)
	mediaDir := putMedia(t, rig, sourceMediaID, `{"createdAt":1}`, 2*time.Hour)
	projectPath := filepath.Join(rig.root, "editor", "projects", "p1", "project.json")
	writeFileT(t, projectPath, `{"mediaItems":[{"id":"`+sourceMediaID+`"}],"title":"x"}`)
	writeFileT(t, filepath.Join(rig.root, "editor", "projects", "p1", "media-links.json"),
		`{"version":"1","ids":["`+sourceMediaID+`"]}`)

	rig.videos.finished = []SourceVideo{{ID: "v1", ProjectID: "p1", SourceMediaID: sourceMediaID}}
	item, err := rig.svc.Reclaim(context.Background(), ReclaimSource)
	if err != nil {
		t.Fatalf("Reclaim src: %v", err)
	}
	if item.Count != 1 || item.Bytes != 5 {
		t.Fatalf("kết quả = %+v", item)
	}
	if exists(filepath.Join(srcDir, "source.mp4")) {
		t.Fatalf("video gốc phải bị xoá")
	}
	if !exists(filepath.Join(srcDir, "info.json")) {
		t.Fatalf("bằng chứng nhỏ phải được giữ")
	}
	if exists(mediaDir) {
		t.Fatalf("media của video gốc phải bị dọn")
	}

	var doc struct {
		MediaItems []map[string]any `json:"mediaItems"`
	}
	raw, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("đọc project: %v", err)
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("project hỏng: %v", err)
	}
	if len(doc.MediaItems) != 0 {
		t.Fatalf("link media phải bị gỡ: %+v", doc.MediaItems)
	}
}

func TestReclaimSourceKeepsMediaUsedOnTimeline(t *testing.T) {
	rig := newRetentionRig(t)
	writeFileT(t, filepath.Join(rig.root, "sources", "v1", "source.mp4"), "video")
	mediaDir := putMedia(t, rig, sourceMediaID, `{"createdAt":1}`, 2*time.Hour)
	writeFileT(t, filepath.Join(rig.root, "editor", "projects", "p1", "project.json"),
		`{"mediaItems":[{"id":"`+sourceMediaID+`"}],"timeline":{"clips":[{"mediaId":"`+sourceMediaID+`"}]}}`)
	rig.videos.finished = []SourceVideo{{ID: "v1", ProjectID: "p1", SourceMediaID: sourceMediaID}}

	if _, err := rig.svc.Reclaim(context.Background(), ReclaimSource); err != nil {
		t.Fatalf("Reclaim src: %v", err)
	}
	if !exists(mediaDir) {
		t.Fatalf("media còn trên timeline phải được giữ")
	}
	var doc struct {
		MediaItems []map[string]any `json:"mediaItems"`
	}
	raw, _ := os.ReadFile(filepath.Join(rig.root, "editor", "projects", "p1", "project.json"))
	_ = json.Unmarshal(raw, &doc)
	if len(doc.MediaItems) != 1 {
		t.Fatalf("media còn dùng thì link phải giữ: %+v", doc.MediaItems)
	}
}

func TestReclaimMP4KeepsOtherExportArtifacts(t *testing.T) {
	rig := newRetentionRig(t)
	exportDir := filepath.Join(rig.root, "exports", "v1")
	writeFileT(t, filepath.Join(exportDir, "export.mp4"), "1234")
	writeFileT(t, filepath.Join(exportDir, "cover.png"), "img")
	writeFileT(t, filepath.Join(exportDir, "export.json"), `{"status":"done"}`)
	rig.videos.posted = []PostedVideo{{ID: "v1"}}

	item, err := rig.svc.Reclaim(context.Background(), ReclaimMP4)
	if err != nil {
		t.Fatalf("Reclaim mp4: %v", err)
	}
	if item.Count != 1 || item.Bytes != 4 {
		t.Fatalf("kết quả = %+v", item)
	}
	if exists(filepath.Join(exportDir, "export.mp4")) {
		t.Fatalf("MP4 phải bị xoá")
	}
	if !exists(filepath.Join(exportDir, "cover.png")) || !exists(filepath.Join(exportDir, "export.json")) {
		t.Fatalf("phần còn lại của bản xuất phải giữ")
	}
}

func TestReclaimUnknownKind(t *testing.T) {
	rig := newRetentionRig(t)
	if _, err := rig.svc.Reclaim(context.Background(), "bogus"); err == nil {
		t.Fatalf("nhóm lạ phải lỗi")
	}
}

func TestReclaimWithNoVideosIsEmptyButReal(t *testing.T) {
	rig := newRetentionRig(t)
	items, err := rig.svc.ReclaimList(context.Background())
	if err != nil {
		t.Fatalf("ReclaimList: %v", err)
	}
	for _, it := range items {
		if it.Count != 0 || it.Bytes != 0 {
			t.Fatalf("chưa có video thì thẻ phải rỗng: %+v", it)
		}
	}
	if _, err := rig.svc.Reclaim(context.Background(), ReclaimMP4); err != nil {
		t.Fatalf("xoá khi không có gì: %v", err)
	}
}
