package library

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// errTrialMedia là lỗi tiêm cho đường dọn media vẽ thử.
var errTrialMedia = errors.New("media lỗi")

// trialFilesFake ghi lại thứ tự gọi và chỉ xoá hàng cha qua commit — đúng
// hợp đồng của TrialFiles: hàng cha không được xoá ngoài transaction media.
type trialFilesFake struct {
	paths    []string
	pathsErr error

	unbindErr error
	dropErr   error

	unbindCalls int
	dropCalls   int
	commitCalls int

	unbindPaths []string
	order       *[]string
}

func (f *trialFilesFake) note(step string) {
	if f.order != nil {
		*f.order = append(*f.order, step)
	}
}

func (f *trialFilesFake) PathsForTemplate(context.Context, string) ([]string, error) {
	f.note("paths")
	if f.pathsErr != nil {
		return nil, f.pathsErr
	}
	return append([]string(nil), f.paths...), nil
}

func (f *trialFilesFake) UnbindFiles(ctx context.Context, paths []string, commit func(context.Context) error) error {
	f.unbindCalls++
	f.unbindPaths = append([]string(nil), paths...)
	f.note("unbind")
	if f.unbindErr != nil {
		// Transaction media rollback: commit không chạy.
		return f.unbindErr
	}
	f.commitCalls++
	f.note("delete")
	return commit(ctx)
}

func (f *trialFilesFake) DropPromptSamples(ctx context.Context, _ string, commit func(context.Context) error) error {
	f.dropCalls++
	f.note("drop")
	if f.dropErr != nil {
		return f.dropErr
	}
	f.commitCalls++
	f.note("delete")
	return commit(ctx)
}

// coverTrialSpy chỉ cần ghi lại CancelTemplate (huỷ + chờ lần vẽ đang chạy
// phải xảy ra trước khi đọc media_path).
type coverTrialSpy struct{ order *[]string }

func (coverTrialSpy) Status(context.Context, string) (*CoverTrialStatus, error) { return nil, nil }

func (coverTrialSpy) Start(context.Context, string, CoverTrialRequest) (*CoverTrialStatus, error) {
	return nil, nil
}

func (coverTrialSpy) Cancel(context.Context, string, string) error { return nil }

func (coverTrialSpy) ImagePath(context.Context, string, string) (string, error) { return "", nil }

func (s coverTrialSpy) CancelTemplate(context.Context, string) {
	if s.order != nil {
		*s.order = append(*s.order, "cancel")
	}
}

// conflictStore từ chối xoá ở store (đua giữa kiểm và xoá), như postgres khi
// template vừa được gắn vào kênh.
type conflictStore struct {
	*memStore
	err error
}

func (s *conflictStore) DeleteTemplate(context.Context, string) error { return s.err }

func TestDeleteTemplateStopsWhenTrialPathsFail(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Giữ lại")
	if err != nil {
		t.Fatal(err)
	}
	files := &trialFilesFake{pathsErr: errTrialMedia}
	svc.BindTrialFiles(files)

	if err := svc.DeleteTemplate(ctx, tpl.ID); !errors.Is(err, errTrialMedia) {
		t.Fatalf("lỗi = %v", err)
	}
	if got, _ := store.GetTemplate(ctx, tpl.ID); got == nil {
		t.Fatal("template bị xoá dù chưa đọc được media_path")
	}
	if files.unbindCalls != 0 {
		t.Fatalf("unbind = %d lượt, muốn 0", files.unbindCalls)
	}
}

func TestDeleteTemplateStopsWhenUnbindFails(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Giữ lại")
	if err != nil {
		t.Fatal(err)
	}
	files := &trialFilesFake{paths: []string{"library/cover-trials/a.png"}, unbindErr: errTrialMedia}
	svc.BindTrialFiles(files)

	if err := svc.DeleteTemplate(ctx, tpl.ID); !errors.Is(err, errTrialMedia) {
		t.Fatalf("lỗi = %v", err)
	}
	if got, _ := store.GetTemplate(ctx, tpl.ID); got == nil {
		t.Fatal("template bị xoá dù media chưa nhả được")
	}
	if files.commitCalls != 0 {
		t.Fatalf("commit = %d lượt, muốn 0 (rollback)", files.commitCalls)
	}
}

func TestDeleteTemplateDeletesRowInsideMediaTransaction(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Xoá được")
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	files := &trialFilesFake{paths: []string{"library/cover-trials/a.png", "library/cover-trials/b.png"}, order: &order}
	svc.BindTrialFiles(files)
	svc.BindCoverTrial(coverTrialSpy{order: &order})

	if err := svc.DeleteTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if got, _ := store.GetTemplate(ctx, tpl.ID); got != nil {
		t.Fatal("template còn sau khi xoá")
	}
	want := []string{"cancel", "paths", "unbind", "delete"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("thứ tự = %v, muốn %v", order, want)
	}
	if !reflect.DeepEqual(files.unbindPaths, files.paths) {
		t.Fatalf("paths nhả = %v, muốn %v", files.unbindPaths, files.paths)
	}
}

func TestDeleteTemplateSurfacesChannelConflictFromMediaTransaction(t *testing.T) {
	base := newMemStore()
	store := &conflictStore{memStore: base, err: ErrInUse}
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Đua kênh")
	if err != nil {
		t.Fatal(err)
	}
	files := &trialFilesFake{paths: []string{"library/cover-trials/a.png"}}
	svc.BindTrialFiles(files)

	err = svc.DeleteTemplate(ctx, tpl.ID)
	if CodeOf(err) != CodeConflict {
		t.Fatalf("lỗi = %v, muốn 409", err)
	}
	if files.commitCalls != 1 {
		t.Fatalf("commit = %d lượt, muốn 1 (store từ chối trong transaction)", files.commitCalls)
	}
	if got, _ := base.GetTemplate(ctx, tpl.ID); got == nil {
		t.Fatal("template phải còn khi store từ chối xoá")
	}
}

func TestDeletePromptStopsWhenSampleCleanupFails(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	prompt, err := svc.CreatePrompt(ctx, PromptInput{Name: "Giữ lại", Kind: domain.PromptScene, Prompt: new("nội dung")})
	if err != nil {
		t.Fatal(err)
	}
	files := &trialFilesFake{dropErr: errTrialMedia}
	svc.BindTrialFiles(files)

	if err := svc.DeletePrompt(ctx, prompt.ID); !errors.Is(err, errTrialMedia) {
		t.Fatalf("lỗi = %v", err)
	}
	if got, _ := store.GetPrompt(ctx, prompt.ID); got == nil {
		t.Fatal("prompt bị xoá dù ảnh mẫu chưa dọn được")
	}
	if files.commitCalls != 0 {
		t.Fatalf("commit = %d lượt, muốn 0 (rollback)", files.commitCalls)
	}
}

func TestDeletePromptDeletesRowInsideSampleTransaction(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	prompt, err := svc.CreatePrompt(ctx, PromptInput{Name: "Xoá được", Kind: domain.PromptScene, Prompt: new("nội dung")})
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	files := &trialFilesFake{order: &order}
	svc.BindTrialFiles(files)

	if err := svc.DeletePrompt(ctx, prompt.ID); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if got, _ := store.GetPrompt(ctx, prompt.ID); got != nil {
		t.Fatal("prompt còn sau khi xoá")
	}
	want := []string{"drop", "delete"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("thứ tự = %v, muốn %v", order, want)
	}
	if files.dropCalls != 1 || files.commitCalls != 1 {
		t.Fatalf("drop = %d, commit = %d, muốn 1/1", files.dropCalls, files.commitCalls)
	}
}

func TestDeletePromptWithoutTrialFilesStillDeletes(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	prompt, err := svc.CreatePrompt(ctx, PromptInput{Name: "Không media", Kind: domain.PromptScene, Prompt: new("nội dung")})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePrompt(ctx, prompt.ID); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if got, _ := store.GetPrompt(ctx, prompt.ID); got != nil {
		t.Fatal("prompt còn sau khi xoá")
	}
}
