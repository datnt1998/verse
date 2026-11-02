package library

// Test bộ xuất template: gói ghi ra phải mở lại được bằng đúng bộ nhập, giữ
// project + mọi media tham chiếu, tên mục an toàn, mã nội dung khớp bytes; và
// nhập lại gói đó tạo template mới với mã media đã đổi.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type exportMediaReader struct {
	records     map[string]editor.MediaRecord
	files       map[string]string
	metadataErr map[string]error
	acquireErr  map[string]error
}

func (r *exportMediaReader) GetMediaMetadata(_ context.Context, id string) (editor.MediaRecord, error) {
	if err := r.metadataErr[id]; err != nil {
		return editor.MediaRecord{}, err
	}
	record, ok := r.records[id]
	if !ok {
		return editor.MediaRecord{}, os.ErrNotExist
	}
	return record, nil
}

func (r *exportMediaReader) AcquireMedia(_ context.Context, id string) (*media.Handle, error) {
	if err := r.acquireErr[id]; err != nil {
		return nil, err
	}
	record, ok := r.records[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &media.Handle{Path: r.files[id], Ref: media.Ref{Bytes: record.Size}}, nil
}

func TestExportTemplateRoundTripsThroughBundleReaderAndImport(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join("testdata", "ytf-project-standard.json"))
	if err != nil {
		t.Fatal(err)
	}
	project, err := editor.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	ids := projectMediaIDs(project)
	if len(ids) == 0 {
		t.Fatal("fixture project has no media")
	}

	dir := t.TempDir()
	reader := &exportMediaReader{records: map[string]editor.MediaRecord{}, files: map[string]string{}}
	// Tên tệp cố ý chứa đường dẫn đi lên: mục zip vẫn phải an toàn.
	for index, id := range ids {
		content := []byte(fmt.Sprintf("media-bytes-%d-%s", index, id))
		sum := sha256.Sum256(content)
		hash := "sha256:" + hex.EncodeToString(sum[:])
		file := filepath.Join(dir, fmt.Sprintf("source-%d.bin", index))
		if err := os.WriteFile(file, content, 0o600); err != nil {
			t.Fatal(err)
		}
		reader.records[id] = editor.MediaRecord{
			ID: id, FileName: "../../evil name.png", MIME: "image/png",
			Size: int64(len(content)), ContentHash: hash,
			Probe: editor.ProbeResult{Kind: "image"},
		}
		reader.files[id] = file
	}

	store := newMemStore()
	svc := testService(t, store, func(deps *Deps) { deps.Importer = newFakeImporter() })
	svc.BindMediaReader(reader)
	if err := store.InsertTemplate(ctx, domain.Template{
		ID: "00000000-0000-4000-8000-000000000001", Name: "Mẫu xuất",
		Recipe: StandardRecipe(), Project: raw, Revision: 3,
	}); err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	info, err := svc.ExportTemplate(ctx, "00000000-0000-4000-8000-000000000001", &archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.MediaCount != len(ids) || !strings.HasSuffix(info.FileName, ".ytf-template.zip") {
		t.Fatalf("info = %+v", info)
	}

	zipPath := filepath.Join(t.TempDir(), info.FileName)
	if err := os.WriteFile(zipPath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := OpenTemplateBundle(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	if bundle.Name != "Mẫu xuất" || len(bundle.Media) != len(ids) {
		t.Fatalf("bundle = name %q media %d", bundle.Name, len(bundle.Media))
	}
	if got := projectMediaIDs(bundle.Project); len(got) != len(ids) {
		t.Fatalf("project media = %v", got)
	}
	for index, item := range bundle.Media {
		if item.FileName != "../../evil name.png" {
			t.Fatalf("fileName must stay original in manifest: %q", item.FileName)
		}
		if !strings.HasPrefix(item.entry, "media/") || strings.Contains(item.entry, "..") {
			t.Fatalf("zip entry must be sanitized: %q", item.entry)
		}
		body, err := bundle.OpenMedia(index)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if item.ContentHash != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatalf("hash mismatch for %s", item.ID)
		}
	}
	// Nhập lại: template mới, project đổi mã media, số media giữ nguyên.
	result, err := svc.ImportTemplate(ctx, bundle, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.MediaImported != len(ids) {
		t.Fatalf("import = %+v", result)
	}
	imported, err := store.GetTemplate(ctx, result.TemplateID)
	if err != nil || imported == nil {
		t.Fatalf("imported template: %v", err)
	}
	reparsed, err := editor.Parse(imported.Project)
	if err != nil {
		t.Fatal(err)
	}
	remapped := projectMediaIDs(reparsed)
	if len(remapped) != len(ids) {
		t.Fatalf("remapped media = %v", remapped)
	}
	for _, id := range remapped {
		if !strings.HasPrefix(id, "media-") {
			t.Fatalf("import must remap media ids to local ids, got %q", id)
		}
	}
}

// TestExportTemplateRejectsCorruptSameSizeMedia ghim lỗi P2: kiểm sha256 chứ
// không chỉ kích thước, và lỗi xảy ra trước byte đầu tiên của gói.
func TestExportTemplateRejectsCorruptSameSizeMedia(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	good := []byte("good-media-bytes")
	corrupt := []byte("evil-media-bytes")
	if len(good) != len(corrupt) {
		t.Fatalf("fixture phải cùng kích thước: %d/%d", len(good), len(corrupt))
	}
	id := "media-corrupt"
	reader := &exportMediaReader{
		records: map[string]editor.MediaRecord{id: exportMediaRecord(id, good)},
		files:   map[string]string{id: writeExportBlob(t, dir, "corrupt.bin", corrupt)},
	}
	store := newMemStore()
	svc := testService(t, store)
	svc.BindMediaReader(reader)
	templateID := insertExportTemplate(t, store, exportProjectFor(id))

	var out bytes.Buffer
	_, err := svc.ExportTemplate(ctx, templateID, &out)
	if err == nil {
		t.Fatal("media hỏng cùng kích thước phải bị từ chối")
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("lỗi phải nói rõ sha256: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("không được ghi byte nào trước khi kiểm xong: %d", out.Len())
	}
	assertExportInternal(t, err)
}

// TestExportTemplateDistinguishesMissingMediaFromFailures ghim lỗi P2: chỉ
// lỗi "không có" mới là media đã bị xoá; lỗi tạm thời/huỷ request phải nổi lên
// như lỗi nội bộ thay vì báo 400 "media đã bị xoá".
func TestExportTemplateDistinguishesMissingMediaFromFailures(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("db unavailable")
	const id = "media-state"

	setup := func(t *testing.T, reader *exportMediaReader) (*Service, string) {
		t.Helper()
		store := newMemStore()
		svc := testService(t, store)
		svc.BindMediaReader(reader)
		return svc, insertExportTemplate(t, store, exportProjectFor(id))
	}
	export := func(t *testing.T, svc *Service, templateID string) (bytes.Buffer, error) {
		t.Helper()
		var out bytes.Buffer
		_, err := svc.ExportTemplate(ctx, templateID, &out)
		if out.Len() != 0 {
			t.Fatalf("lỗi trước header nhưng đã ghi %d byte", out.Len())
		}
		return out, err
	}

	t.Run("metadata không có", func(t *testing.T) {
		svc, templateID := setup(t, &exportMediaReader{})
		_, err := export(t, svc, templateID)
		if CodeOf(err) != CodeInvalidInput || !strings.Contains(err.Error(), "đã bị xoá") {
			t.Fatalf("muốn 400 media đã bị xoá, nhận %v", err)
		}
	})

	t.Run("metadata lỗi tạm thời", func(t *testing.T) {
		svc, templateID := setup(t, &exportMediaReader{metadataErr: map[string]error{id: dbErr}})
		_, err := export(t, svc, templateID)
		if !errors.Is(err, dbErr) {
			t.Fatalf("phải giữ nguyên lỗi gốc: %v", err)
		}
		assertExportInternal(t, err)
	})

	t.Run("huỷ request", func(t *testing.T) {
		svc, templateID := setup(t, &exportMediaReader{metadataErr: map[string]error{id: context.Canceled}})
		_, err := export(t, svc, templateID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("phải giữ nguyên context.Canceled: %v", err)
		}
		assertExportInternal(t, err)
	})

	t.Run("acquire không có", func(t *testing.T) {
		svc, templateID := setup(t, &exportMediaReader{
			records:    map[string]editor.MediaRecord{id: exportMediaRecord(id, []byte("x"))},
			acquireErr: map[string]error{id: os.ErrNotExist},
		})
		_, err := export(t, svc, templateID)
		if CodeOf(err) != CodeInvalidInput || !strings.Contains(err.Error(), "đã bị xoá") {
			t.Fatalf("muốn 400 media đã bị xoá, nhận %v", err)
		}
	})

	t.Run("acquire lỗi tạm thời", func(t *testing.T) {
		svc, templateID := setup(t, &exportMediaReader{
			records:    map[string]editor.MediaRecord{id: exportMediaRecord(id, []byte("x"))},
			acquireErr: map[string]error{id: dbErr},
		})
		_, err := export(t, svc, templateID)
		if !errors.Is(err, dbErr) {
			t.Fatalf("phải giữ nguyên lỗi gốc: %v", err)
		}
		assertExportInternal(t, err)
	})

	t.Run("đọc tệp lỗi", func(t *testing.T) {
		content := []byte("media-bytes")
		svc, templateID := setup(t, &exportMediaReader{
			records: map[string]editor.MediaRecord{id: exportMediaRecord(id, content)},
			files:   map[string]string{id: t.TempDir()}, // thư mục: mở được, đọc lỗi
		})
		_, err := export(t, svc, templateID)
		if err == nil {
			t.Fatal("lỗi đọc tệp phải nổi lên")
		}
		assertExportInternal(t, err)
	})
}

// TestExportTemplateRefusesArchiveOverImportLimit ghim lỗi P2: gói lớn hơn
// trần nhập đang áp dụng (kể cả phần khung zip) bị từ chối trước header, còn
// đúng biên thì xuất được.
func TestExportTemplateRefusesArchiveOverImportLimit(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example")
	limit := TemplateBundleLimit()
	budget, ok := templateExportContentBudget(1)
	if !ok || budget <= 0 || budget >= limit {
		t.Fatalf("ngân sách nội dung = %d (trần %d)", budget, limit)
	}
	const id = "media-limit"

	t.Run("vượt biên", func(t *testing.T) {
		size := budget + 1
		dir := t.TempDir()
		path := filepath.Join(dir, "over.bin")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		record := editor.MediaRecord{
			ID: id, FileName: "over.mp4", MIME: "video/mp4", Size: size,
			ContentHash: "sha256:" + strings.Repeat("0", 64),
		}
		store := newMemStore()
		svc := testService(t, store)
		svc.BindMediaReader(&exportMediaReader{
			records: map[string]editor.MediaRecord{id: record},
			files:   map[string]string{id: path},
		})
		templateID := insertExportTemplate(t, store, exportProjectFor(id))
		var out bytes.Buffer
		_, err = svc.ExportTemplate(context.Background(), templateID, &out)
		if CodeOf(err) != CodeInvalidInput || !strings.Contains(err.Error(), "quá lớn") {
			t.Fatalf("muốn 400 quá lớn, nhận %v", err)
		}
		if out.Len() != 0 {
			t.Fatalf("không được ghi byte nào: %d", out.Len())
		}
	})

	t.Run("đúng biên", func(t *testing.T) {
		// Manifest thật chiếm thêm vài trăm byte, nên đo nó ở một kích thước
		// gần biên (cùng số chữ số), rồi đặt nội dung đúng phần còn lại.
		open := func(size int64) (*TemplateExport, error) {
			dir := t.TempDir()
			path := filepath.Join(dir, "fit.bin")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(size); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			record := editor.MediaRecord{
				ID: id, FileName: "fit.mp4", MIME: "video/mp4", Size: size,
				ContentHash: sha256Zeros(t, size),
			}
			store := newMemStore()
			svc := testService(t, store)
			svc.BindMediaReader(&exportMediaReader{
				records: map[string]editor.MediaRecord{id: record},
				files:   map[string]string{id: path},
			})
			templateID := insertExportTemplate(t, store, exportProjectFor(id))
			return svc.OpenTemplateExport(context.Background(), templateID)
		}

		probe, err := open(budget - (1 << 20))
		if err != nil {
			t.Fatalf("gói gần biên phải xuất được: %v", err)
		}
		manifestLen := int64(len(probe.manifest))
		probe.Close()
		if manifestLen <= 0 || manifestLen >= 1<<20 {
			t.Fatalf("manifest dài bất thường: %d", manifestLen)
		}

		size := budget - manifestLen
		export, err := open(size)
		if err != nil {
			t.Fatalf("đúng biên phải xuất được (size %d, manifest %d): %v", size, manifestLen, err)
		}
		defer export.Close()
		if export.Info.MediaCount != 1 {
			t.Fatalf("MediaCount = %d", export.Info.MediaCount)
		}
	})
}

// ── fixture ────────────────────────────────────────────────────────────────

func exportProjectFor(ids ...string) []byte {
	refs := make([]string, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, fmt.Sprintf(`{"id":%q}`, id))
	}
	return []byte(fmt.Sprintf(`{"schemaVersion":%d,"mediaItems":[%s]}`, editor.SchemaVersion, strings.Join(refs, ",")))
}

func insertExportTemplate(t *testing.T, store *memStore, project []byte) string {
	t.Helper()
	id := "00000000-0000-4000-8000-0000000000aa"
	if err := store.InsertTemplate(context.Background(), domain.Template{
		ID: id, Name: "Mẫu xuất", Recipe: StandardRecipe(), Project: project, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func exportMediaRecord(id string, content []byte) editor.MediaRecord {
	sum := sha256.Sum256(content)
	return editor.MediaRecord{
		ID: id, FileName: "clip.mp4", MIME: "video/mp4", Size: int64(len(content)),
		ContentHash: "sha256:" + hex.EncodeToString(sum[:]), Probe: editor.ProbeResult{Kind: "video"},
	}
}

func writeExportBlob(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func sha256Zeros(t *testing.T, size int64) string {
	t.Helper()
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	for remaining := size; remaining > 0; {
		chunk := int64(len(buffer))
		if remaining < chunk {
			chunk = remaining
		}
		if _, err := hash.Write(buffer[:chunk]); err != nil {
			t.Fatal(err)
		}
		remaining -= chunk
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func assertExportInternal(t *testing.T, err error) {
	t.Helper()
	var apiErr *Error
	if errors.As(err, &apiErr) {
		t.Fatalf("lỗi nội bộ không được thành lỗi nghiệp vụ %+v", apiErr)
	}
}
