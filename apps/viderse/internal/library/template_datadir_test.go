package library

// Test nguồn "thư mục dữ liệu cũ": đọc registry template trong app.db (trên bản
// sao) + thư mục project/media, rồi nhập bằng đúng đường quy đổi của tệp xuất.
// Fixture dựng app.db thật bằng driver SQLite của chương trình, nên test đi qua
// đúng câu SQL và đúng cách mở bản sao.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// dataDirFixture là thư mục dữ liệu cũ dựng trong test.
type dataDirFixture struct {
	dir       string
	workspace string
	ids       []string
}

// newDataDirFixture dựng thư mục dữ liệu cũ từ các project fixture; mỗi
// project một hàng registry, media tổng hợp đúng sha256.
func newDataDirFixture(t *testing.T, projects ...string) *dataDirFixture {
	t.Helper()
	dir := t.TempDir()
	fixture := &dataDirFixture{dir: dir, workspace: filepath.Join(dir, "editor-workspace")}
	db, err := sql.Open("sqlite3", filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table templates (project_id text primary key, created_at integer not null, preview_project_id text)`); err != nil {
		t.Fatal(err)
	}
	for index, projectFile := range projects {
		raw, err := os.ReadFile(projectFile)
		if err != nil {
			t.Fatal(err)
		}
		project, err := editor.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		id := fmt.Sprintf("11111111-1111-4111-8111-%012d", index+1)
		projectDir := filepath.Join(fixture.workspace, "projects", id)
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "project.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, mediaID := range projectMediaIDs(project) {
			writeWorkspaceMedia(t, fixture.workspace, mediaID)
		}
		if _, err := db.Exec(`insert into templates (project_id, created_at) values (?, ?)`, id, 1700000000000+index); err != nil {
			t.Fatal(err)
		}
		fixture.ids = append(fixture.ids, id)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// writeWorkspaceMedia ghi `media/<id>/metadata.json` + tệp nguồn tổng hợp.
func writeWorkspaceMedia(t *testing.T, workspace, id string) {
	t.Helper()
	dir := filepath.Join(workspace, "media", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fileName := "media-" + id[:8] + ".mp4"
	data := []byte(strings.Repeat(id[:4], 8))
	sum := sha256.Sum256(data)
	meta := map[string]any{
		"id": id, "fileName": fileName, "mime": "video/mp4",
		"size": len(data), "contentHash": "sha256:" + hex.EncodeToString(sum[:]),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateDataDirImportsEveryRegisteredTemplate(t *testing.T) {
	fixture := newDataDirFixture(t, "testdata/ytf-project-standard.json", "testdata/ytf-project-backdrop-cutout.json")
	items, err := OpenTemplateDataDir(fixture.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("số template đọc được = %d", len(items))
	}
	if items[0].Name != "Mẫu chuẩn · Velvety Murmurs" || items[1].Name != "Thử · Nền Twitch + Tách nền" {
		t.Fatalf("thứ tự/tên = %q, %q", items[0].Name, items[1].Name)
	}
	svc := testService(t, newMemStore(), func(d *Deps) { d.Importer = newFakeImporter() })
	results := svc.ImportBundles(context.Background(), items, ImportOptions{})
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("kết quả = %+v", results)
	}
	// Hai fixture dùng chung bốn media nên lần nhập thứ hai dùng lại theo nội dung.
	if results[0].MediaImported != 7 || results[1].MediaImported+results[1].MediaReused != 16 {
		t.Fatalf("media nhập = %d, %d (+%d dùng lại)", results[0].MediaImported, results[1].MediaImported, results[1].MediaReused)
	}
	// Template Twitch giữ đúng cài đặt đã đặt trong project, kèm cảnh báo kênh nền.
	twitch := results[1]
	if !strings.Contains(strings.Join(twitch.Warnings, "\n"), "twitch:misurukawaii") {
		t.Fatalf("thiếu cảnh báo kênh nền: %v", twitch.Warnings)
	}
}

func TestTemplateDataDirKeepsGoingWhenOneTemplateIsBroken(t *testing.T) {
	fixture := newDataDirFixture(t, "testdata/ytf-project-standard.json", "testdata/ytf-project-backdrop-cutout.json")
	// Xoá tệp media của template thứ hai: template đó hỏng, template đầu vẫn nhập.
	broken := filepath.Join(fixture.workspace, "media", "01ed3088-769c-48e1-937c-4a63a1771f4d", "media-01ed3088.mp4")
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	items, err := OpenTemplateDataDir(fixture.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Bundle == nil || items[1].Bundle != nil {
		t.Fatalf("bundle = %+v", items)
	}
	if !strings.Contains(errorText(items[1].Err), "không còn trên đĩa") {
		t.Fatalf("lỗi = %v", items[1].Err)
	}
	svc := testService(t, newMemStore(), func(d *Deps) { d.Importer = newFakeImporter() })
	results := svc.ImportBundles(context.Background(), items, ImportOptions{})
	if len(results) != 2 || !results[0].OK || results[1].OK {
		t.Fatalf("kết quả = %+v", results)
	}
	if results[1].Error == "" || results[1].Source != fixture.ids[1] {
		t.Fatalf("dòng hỏng = %+v", results[1])
	}
}

func TestTemplateDataDirRejectsForeignDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenTemplateDataDir(dir); err == nil || !strings.Contains(errorText(err), "editor-workspace") {
		t.Fatalf("lỗi = %v", err)
	}
	workspace := filepath.Join(dir, "editor-workspace", "projects")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenTemplateDataDir(dir); err == nil || !strings.Contains(errorText(err), "app.db") {
		t.Fatalf("thiếu app.db: %v", err)
	}
	// app.db là SQLite nhưng không có bảng templates.
	db, err := sql.Open("sqlite3", filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table other (x integer)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenTemplateDataDir(dir); err == nil || !strings.Contains(errorText(err), "bảng templates") {
		t.Fatalf("thiếu bảng: %v", err)
	}
}

func TestTemplateDataDirLeavesSourceUntouched(t *testing.T) {
	fixture := newDataDirFixture(t, "testdata/ytf-project-standard.json")
	before, err := os.Stat(filepath.Join(fixture.dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenTemplateDataDir(fixture.dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filepath.Join(fixture.dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime() != after.ModTime() || before.Size() != after.Size() {
		t.Fatal("app.db nguồn bị đụng vào")
	}
	entries, err := os.ReadDir(fixture.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-journal") || strings.HasSuffix(entry.Name(), "-wal") {
			t.Fatalf("để lại tệp phụ trong thư mục dữ liệu: %s", entry.Name())
		}
	}
}
