package store

import (
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestChapterRecordAcceptStampsBuild 锁死 P0-2：每次接纳章节都给记录盖上当前
// 构建戳 "<version>+<short commit>"，事后能从 truyện 反查哪个 build 写的。
func TestChapterRecordAcceptStampsBuild(t *testing.T) {
	version.SetBuild(version.Info{Version: "v9.9.9", Commit: "abc1234def"})
	defer version.SetBuild(version.Info{})

	st := NewStore(t.TempDir())
	record, err := st.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, "Nội dung chương một.", domain.ChapterFacts{
		Title: "Chương 1", Summary: "Tóm tắt",
	}, domain.StyleDelta{})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if record.Version != domain.ChapterRecordVersion {
		t.Fatalf("record version = %d, want %d", record.Version, domain.ChapterRecordVersion)
	}
	if record.Build != "v9.9.9+abc1234" {
		t.Fatalf("record build = %q, want v9.9.9+abc1234", record.Build)
	}
	// 落盘后重新加载，确认 build 戳持久化在 JSON 里。
	loaded, err := st.ChapterRecords.Load(1)
	if err != nil || loaded == nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Build != "v9.9.9+abc1234" {
		t.Fatalf("persisted build = %q", loaded.Build)
	}
}

// TestChapterRecordLoadsV1WithoutBuild 保证旧格式（v1，无 build 字段）的记录
// 仍然可读，Build 为空表示"不知道哪个构建写的"。
func TestChapterRecordLoadsV1WithoutBuild(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	content := "Chương cũ được build cũ viết."
	v1 := `{
		"version": 1,
		"chapter": 3,
		"revision": 2,
		"origin": "generated",
		"content": ` + strconv.Quote(content) + `,
		"content_sha256": "` + domain.ChapterContentSHA256(content) + `",
		"facts": {"title": "C3", "summary": "S", "characters": [], "key_events": [],
			"timeline_events": [], "foreshadow_updates": [], "relationship_changes": [], "state_changes": [], "cast_intros": []},
		"style_delta": {},
		"accepted_at": "` + time.Now().UTC().Format(time.RFC3339) + `"
	}`
	path := filepath.Join(dir, ChapterRecordPath(3))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}

	record, err := st.ChapterRecords.Load(3)
	if err != nil {
		t.Fatalf("v1 record must still load: %v", err)
	}
	if record == nil || record.Version != 1 {
		t.Fatalf("record = %+v", record)
	}
	if record.Build != "" {
		t.Fatalf("v1 record build should be empty (unknown), got %q", record.Build)
	}
	if record.Content != content {
		t.Fatalf("content = %q", record.Content)
	}
}

// TestChapterRecordRejectsFutureVersion 防止静默吞掉更高版本的记录格式。
func TestChapterRecordRejectsFutureVersion(t *testing.T) {
	st := NewStore(t.TempDir())
	err := st.ChapterRecords.Save(domain.ChapterRecord{
		Version: domain.ChapterRecordVersion + 1, Chapter: 1, Revision: 1,
		Origin: domain.ChapterOriginGenerated, Content: "x",
		ContentSHA256: domain.ChapterContentSHA256("x"), AcceptedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("v3 record must be rejected")
	}
}
