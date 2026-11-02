// Ca test Postgres của bản xem thử: token công bố (CAS), lỗi có hành động,
// danh mục project ẩn độc lập con trỏ, và dấu dọn còn lại khi xoá project
// best effort thất bại. Chạy trên Postgres thật; bỏ qua khi thiếu
// TEST_DATABASE_URL (cùng provision với settings_pg_test).
package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
)

type stubPreviewBuilder struct {
	dropErr error
}

func (stubPreviewBuilder) Sources(context.Context) ([]library.PreviewSource, error) {
	return []library.PreviewSource{}, nil
}
func (stubPreviewBuilder) Status(context.Context, string) (*library.PreviewStatus, error) {
	return &library.PreviewStatus{Sources: []library.PreviewSource{}}, nil
}
func (stubPreviewBuilder) Start(context.Context, string, library.PreviewRequest) (*library.PreviewStatus, error) {
	return &library.PreviewStatus{Sources: []library.PreviewSource{}}, nil
}
func (stubPreviewBuilder) Cancel(context.Context, string) error { return nil }
func (s stubPreviewBuilder) Drop(context.Context, string) error { return s.dropErr }

func previewTestService(t *testing.T, dropErr error) *library.Service {
	t.Helper()
	svc, _ := newLibraryService(t)
	svc.BindPreviewBuilder(stubPreviewBuilder{dropErr: dropErr})
	return svc
}

func TestTemplatePreviewTokenCASAndFailurePreserveProject(t *testing.T) {
	svc := previewTestService(t, nil)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Template xem thử")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.BeginTemplatePreview(ctx, tpl.ID, "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterTemplatePreviewProject(ctx, tpl.ID, "preview-a", "", 3); err != nil {
		t.Fatal(err)
	}
	previous, err := svc.RecordTemplatePreview(ctx, tpl.ID, library.TemplatePreviewRecord{
		ProjectID: "preview-a", Minutes: 3, TemplateRevision: 1,
		BuiltAt: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
		RunID:   "11111111-1111-4111-8111-111111111111",
	})
	if err != nil || previous != "" {
		t.Fatalf("record đầu = %q %v", previous, err)
	}
	got, err := svc.Get(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewProjectID != "preview-a" || got.PreviewState() != "fresh" {
		t.Fatalf("sau record = %+v", got.PreviewProjectID)
	}
	// Lượt mới thay bản cũ; lượt cũ (token đã mất) không được công bố đè.
	if err := svc.BeginTemplatePreview(ctx, tpl.ID, "22222222-2222-4222-8222-222222222222"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterTemplatePreviewProject(ctx, tpl.ID, "preview-b", "", 2); err != nil {
		t.Fatal(err)
	}
	previous, err = svc.RecordTemplatePreview(ctx, tpl.ID, library.TemplatePreviewRecord{
		ProjectID: "preview-b", Minutes: 2, TemplateRevision: 1,
		BuiltAt: time.Date(2026, 11, 2, 12, 1, 0, 0, time.UTC),
		RunID:   "22222222-2222-4222-8222-222222222222",
	})
	if err != nil || previous != "preview-a" {
		t.Fatalf("record hai = %q %v", previous, err)
	}
	if _, err := svc.RecordTemplatePreview(ctx, tpl.ID, library.TemplatePreviewRecord{
		ProjectID: "preview-stale", RunID: "11111111-1111-4111-8111-111111111111",
	}); !errors.Is(err, library.ErrPreviewSuperseded) {
		t.Fatalf("lượt cũ = %v", err)
	}
	got, _ = svc.Get(ctx, tpl.ID)
	if got.PreviewProjectID != "preview-b" {
		t.Fatalf("lượt cũ đè con trỏ: %q", got.PreviewProjectID)
	}
	// Lỗi dựng: giữ project/revision cũ, chỉ ghi lỗi hành động.
	if err := svc.BeginTemplatePreview(ctx, tpl.ID, "33333333-3333-4333-8333-333333333333"); err != nil {
		t.Fatal(err)
	}
	failure := &library.PreviewError{Code: library.CodeConflict, Message: "Video nguồn vừa bị thay", Recovery: "Dựng lại bản xem thử"}
	if err := svc.FailTemplatePreview(ctx, tpl.ID, "33333333-3333-4333-8333-333333333333", failure); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewError == nil || got.PreviewError.Message != failure.Message || got.PreviewError.Recovery != failure.Recovery {
		t.Fatalf("lỗi đã lưu = %+v", got.PreviewError)
	}
	if got.PreviewProjectID != "preview-b" {
		t.Fatalf("mất bản xem thử cũ: %q", got.PreviewProjectID)
	}
	// Bắt đầu lượt mới xoá lỗi cũ.
	if err := svc.BeginTemplatePreview(ctx, tpl.ID, "44444444-4444-4444-8444-444444444444"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, tpl.ID)
	if got.PreviewError != nil {
		t.Fatalf("lỗi cũ chưa xoá: %+v", got.PreviewError)
	}
}

func TestPreviewProjectsStayHiddenWhenDropFailsAndTemplateIsDeleted(t *testing.T) {
	svc := previewTestService(t, errors.New("không xoá được project"))
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Template còn dấu")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.BeginTemplatePreview(ctx, tpl.ID, "55555555-5555-4555-8555-555555555555"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterTemplatePreviewProject(ctx, tpl.ID, "preview-hidden", "", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordTemplatePreview(ctx, tpl.ID, library.TemplatePreviewRecord{
		ProjectID: "preview-hidden", Minutes: 3, TemplateRevision: 1, BuiltAt: time.Now(),
		RunID: "55555555-5555-4555-8555-555555555555",
	}); err != nil {
		t.Fatal(err)
	}
	// Mọi project từng đăng ký đều bị ẩn, kể cả khi con trỏ đổi.
	hidden, err := svc.PreviewEditorProjectIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hidden["preview-hidden"] {
		t.Fatalf("chưa ẩn: %+v", hidden)
	}
	// Xoá template: Drop thất bại thì dòng danh mục còn lại (template_id NULL)
	// để project vẫn bị ẩn và còn dấu dọn.
	if err := svc.DeleteTemplate(ctx, tpl.ID); err != nil {
		t.Fatal(err)
	}
	hidden, err = svc.PreviewEditorProjectIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hidden["preview-hidden"] {
		t.Fatalf("Drop hỏng làm lộ project: %+v", hidden)
	}
	rows, err := svc.PreviewProjectRows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ProjectID == "preview-hidden" {
			found = true
			if row.Current {
				t.Fatal("project của template đã xoá bị coi là hiện tại")
			}
		}
	}
	if !found {
		t.Fatalf("mất dấu dọn: %+v", rows)
	}
}
