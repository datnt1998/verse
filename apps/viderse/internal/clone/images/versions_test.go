package images

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func versionService(t *testing.T) (*Service, *editStore) {
	t.Helper()
	service, store := testService()
	service.Edits.Media = &stepstest.FakeMedia{Root: t.TempDir()}
	service.RefLimits = func(_ context.Context, provider string) (int, bool) {
		if provider == "" {
			return 4, true
		}
		return 4, true
	}
	return service, store
}

// TestRedrawVersionsPlansNextVersionAndQueue: vẽ lại trả số bản kế tiếp, đặt
// draft cho đúng ảnh và chỉ đánh dấu bước images chạy lại.
func TestRedrawVersionsPlansNextVersionAndQueue(t *testing.T) {
	service, store := versionService(t)
	out, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, Feedback: "tối hơn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Versions) != 1 || out.Versions[0].ImageID != "scene-1" || out.Versions[0].N != 2 {
		t.Fatalf("versions=%+v", out.Versions)
	}
	if !store.change.Rerun || store.change.InvalidateFrom != domain.CloneImages || !store.change.ImageRedraw {
		// Bước images đã đỗ nên phải xếp lại đúng nó và chỉ nó: rerun + ImageRedraw.
		t.Fatalf("change=%+v", store.change)
	}
	item := store.m.Items[0]
	if item.Status != "pending" || item.Draft == nil || item.Draft.Feedback != "tối hơn" {
		t.Fatalf("item=%+v", item)
	}
	if item.ReviewVersion != nil {
		t.Fatal("vẽ lại phải bỏ bản chờ duyệt cũ")
	}
	if len(store.m.QueueOnly) != 1 || store.m.QueueOnly[0] != "scene-1" {
		t.Fatalf("queueOnly=%v", store.m.QueueOnly)
	}
}

func TestRedrawVersionsPersistsEditorProjectOwnership(t *testing.T) {
	no, yes := false, true
	for _, project := range []*bool{nil, &no, &yes} {
		service, store := versionService(t)
		if _, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, Project: project}); err != nil {
			t.Fatal(err)
		}
		draft := store.m.Items[0].Draft
		if draft == nil || (draft.Project == nil) != (project == nil) || (project != nil && *draft.Project != *project) {
			t.Fatalf("queued ownership lost: project=%v draft=%+v", project, draft)
		}
	}
}

// TestRedrawVersionsRejectsPartsForManyImages: sửa prompt chỉ áp cho một ảnh.
func TestRedrawVersionsRejectsPartsForManyImages(t *testing.T) {
	service, _ := versionService(t)
	_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{
		ImageIDs: []string{"scene-1", "thumbnail"},
		Parts:    map[string]string{"scene": "chỉ một ảnh"},
	})
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrInvalidRequest {
		t.Fatalf("err=%v", err)
	}
}

// TestRedrawVersionsRejectsTooManyRefs: quá trần ảnh tham chiếu là 422 với
// đúng giới hạn của nguồn vẽ.
func TestRedrawVersionsRejectsTooManyRefs(t *testing.T) {
	service, _ := versionService(t)
	service.RefLimits = func(context.Context, string) (int, bool) { return 1, true }
	_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{
		ImageIDs: []string{"scene-1"},
		Refs:     []RedrawRef{{Upload: "media-1", Role: "layout"}, {Upload: "media-2", Role: "character"}},
	})
	var tooMany *TooManyRefsError
	if !errors.As(err, &tooMany) || tooMany.Limit != 1 {
		t.Fatalf("err=%v", err)
	}
}

// TestRedrawVersionsKeepsLibraryPromptUntouched: sửa prompt ảnh không đổi
// template hay prompt thư viện của video.
func TestRedrawVersionsKeepsLibraryPromptUntouched(t *testing.T) {
	service, store := versionService(t)
	store.m.TemplatePrompt = "template gốc"
	store.m.CoverPrompt = "prompt bìa gốc"
	if err := json.Unmarshal([]byte(`{"scenePromptId":"p-1","coverPromptId":"p-2"}`), &struct {
		ScenePromptID string `json:"scenePromptId"`
		CoverPromptID string `json:"coverPromptId"`
	}{}); err != nil {
		t.Fatal(err)
	}
	store.m.Overrides = &model.Overrides{ScenePromptID: "p-1", CoverPromptID: "p-2"}
	if _, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, Parts: map[string]string{"scene": "cảnh mới"}}); err != nil {
		t.Fatal(err)
	}
	if store.m.TemplatePrompt != "template gốc" || store.m.CoverPrompt != "prompt bìa gốc" {
		t.Fatalf("template/prompt đổi: %+v", store.m)
	}
	if store.m.Overrides.ScenePromptID != "p-1" || store.m.Overrides.CoverPromptID != "p-2" {
		t.Fatalf("prompt thư viện đổi: %+v", store.m.Overrides)
	}
}

// TestRedrawVersionsRejectsUnknownProvider: nguồn vẽ lạ là 400.
func TestRedrawVersionsRejectsUnknownProvider(t *testing.T) {
	service, _ := versionService(t)
	_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, Provider: "midjourney"})
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrInvalidRequest {
		t.Fatalf("err=%v", err)
	}
}

// TestRedrawVersionsAcceptsProviderAlias: "chatgpt" là bí danh của codex.
func TestRedrawVersionsAcceptsProviderAlias(t *testing.T) {
	service, store := versionService(t)
	if _, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, Provider: "chatgpt"}); err != nil {
		t.Fatal(err)
	}
	if store.m.Items[0].Draft == nil || store.m.Items[0].Draft.Provider != "codex" {
		t.Fatalf("draft=%+v", store.m.Items[0].Draft)
	}
}

// fakeBinder ghi lại yêu cầu thay media của clip.
type fakeBinder struct {
	requests []RebindRequest
	result   *RebindResult
	err      error
}

func (f *fakeBinder) Rebind(_ context.Context, req RebindRequest) (*RebindResult, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}

// TestUseVersionSwitchesAndHandsEditorTheClipChange: dùng bản khác trả đúng
// thay đổi clip cho Editor khi project=false.
func TestUseVersionSwitchesAndHandsEditorTheClipChange(t *testing.T) {
	service, store := versionService(t)
	binder := &fakeBinder{result: &RebindResult{ProjectID: "clone-job-1-g1", ClipID: "clone-job-1-g1-scene-scene-1", MediaID: "media-new", Src: "/api/editor/media/media-new?v=sha256:new", PreviousMediaID: "media-old", Quality: "measuring"}}
	service.Binder = binder
	one := 1
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &one
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, Path: "images/versions/scene-1/2-x.webp", SHA256: "new", StateName: model.VersionReady, Width: 1920, Height: 1080, SourceFingerprint: model.SourceFingerprint(store.m.Items[0])})
	store.m.Items[0].NeedsReview = true
	store.artifacts = nil
	out, err := service.UseVersion(context.Background(), "job-1", "scene-1", UseVersionInput{N: 2, Project: new(false)})
	if err != nil {
		t.Fatal(err)
	}
	if out.UsedVersion != 2 || out.PreviousVersion == nil || *out.PreviousVersion != 1 {
		t.Fatalf("out=%+v", out)
	}
	if out.ClipID == nil || *out.ClipID != "clone-job-1-g1-scene-scene-1" || out.MediaID == nil || *out.MediaID != "media-new" {
		t.Fatalf("clip=%+v", out)
	}
	if len(binder.requests) != 1 || binder.requests[0].ApplyProject {
		t.Fatalf("binder=%+v", binder.requests)
	}
	if store.m.Items[0].CurrentVersion == nil || *store.m.Items[0].CurrentVersion != 2 || store.m.Items[0].NeedsReview {
		t.Fatalf("item=%+v", store.m.Items[0])
	}
}

// TestUseVersionRebindsProjectByDefault: mặc định máy chủ tự thay media của
// clip trong project đã dựng.
func TestUseVersionRebindsProjectByDefault(t *testing.T) {
	service, store := versionService(t)
	binder := &fakeBinder{result: &RebindResult{ProjectID: "clone-job-1-g1", ClipID: "clone-job-1-g1-scene-scene-1", MediaID: "media-new", Quality: "measuring"}}
	service.Binder = binder
	one := 1
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &one
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, Path: "images/versions/scene-1/2-x.webp", SHA256: "new", StateName: model.VersionReady, Width: 1920, Height: 1080, SourceFingerprint: model.SourceFingerprint(store.m.Items[0])})
	if _, err := service.UseVersion(context.Background(), "job-1", "scene-1", UseVersionInput{N: 2}); err != nil {
		t.Fatal(err)
	}
	if len(binder.requests) != 1 || !binder.requests[0].ApplyProject {
		t.Fatalf("binder=%+v", binder.requests)
	}
}

// TestUseVersionRejectsUnfinishedVersion: bản chưa vẽ xong không dùng được.
func TestUseVersionRejectsUnfinishedVersion(t *testing.T) {
	service, store := versionService(t)
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, StateName: model.VersionFailed})
	_, err := service.UseVersion(context.Background(), "job-1", "scene-1", UseVersionInput{N: 2})
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrConflict {
		t.Fatalf("err=%v", err)
	}
}

// TestCancelVersionRestoresQueuedImage: huỷ khi chưa chạy thì ảnh về trạng
// thái trước, hàng chờ sạch, không tốn ảnh.
func TestCancelVersionRestoresQueuedImage(t *testing.T) {
	service, store := versionService(t)
	store.m.Items[0].Status = "pending"
	store.m.Items[0].Draft = &model.PromptDraft{BaseStatus: "approved", Feedback: "tối hơn"}
	store.m.QueueOnly = []string{"scene-1"}
	out, err := service.CancelVersion(context.Background(), "job-1", "scene-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled || out.CostImages != 0 || out.UsedVersion != 0 {
		t.Fatalf("out=%+v", out)
	}
	if store.m.Items[0].Status != "approved" || store.m.Items[0].Draft != nil {
		t.Fatalf("item=%+v", store.m.Items[0])
	}
	if len(store.m.QueueOnly) != 0 {
		t.Fatalf("queueOnly=%v", store.m.QueueOnly)
	}
}

// TestCancelVersionRejectsFinishedVersion: bản đã vẽ xong không huỷ được.
func TestCancelVersionRejectsFinishedVersion(t *testing.T) {
	service, store := versionService(t)
	store.m.Items[0].Versions[0].Path = "images/versions/scene-1/1-a.webp"
	_, err := service.CancelVersion(context.Background(), "job-1", "scene-1", 1)
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrConflict {
		t.Fatalf("err=%v", err)
	}
}

// TestCancelVersionSignalsRunningDraw: đang vẽ thì huỷ qua sổ huỷ dùng chung.
func TestCancelVersionSignalsRunningDraw(t *testing.T) {
	service, store := versionService(t)
	service.Cancels = model.NewCancelRegistry()
	store.m.Items[0].InFlight = &model.Flight{Version: 2}
	out, err := service.CancelVersion(context.Background(), "job-1", "scene-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled {
		t.Fatalf("out=%+v", out)
	}
	if !service.Cancels.Cancelled(model.DrawKey("job-1", "scene-1", 2)) {
		t.Fatal("sổ huỷ không nhận ý định huỷ")
	}
}

// TestUseVersionBindsClipWhenVersionAlreadyUsed: worker tự gán bản vừa vẽ xong
// làm bản đang dùng nên /use nhận n == currentVersion — vẫn phải Rebind để
// project/clip đổi media và trả clipId/mediaId cho Editor, nếu không ảnh mới
// không bao giờ vào timeline và không có thông báo Hoàn tác.
func TestUseVersionBindsClipWhenVersionAlreadyUsed(t *testing.T) {
	service, store := versionService(t)
	binder := &fakeBinder{result: &RebindResult{ProjectID: "clone-job-1-g1", ClipID: "clone-job-1-g1-scene-scene-1", MediaID: "media-new", Src: "/api/editor/media/media-new?v=sha256:new", Quality: "measuring"}}
	service.Binder = binder
	two := 2
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &two
	store.m.Items[0].NeedsReview = true
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, Path: "images/versions/scene-1/2-x.webp", SHA256: "new", StateName: model.VersionReady, SourceFingerprint: model.SourceFingerprint(store.m.Items[0])})
	out, err := service.UseVersion(context.Background(), "job-1", "scene-1", UseVersionInput{N: 2, Project: new(bool)})
	if err != nil {
		t.Fatal(err)
	}
	if len(binder.requests) != 1 {
		t.Fatalf("bản đang dùng vẫn phải rebind clip: %+v", binder.requests)
	}
	if out.ClipID == nil || *out.ClipID != "clone-job-1-g1-scene-scene-1" || out.MediaID == nil || *out.MediaID != "media-new" {
		t.Fatalf("out=%+v", out)
	}
	if out.UsedVersion != 2 {
		t.Fatalf("out=%+v", out)
	}
}

// TestUseVersionEnqueuesWhileImagesRun: đổi bản đang dùng trong lúc bước images
// còn chạy (vẽ lại vừa xong một ảnh) phải xếp ý định cho worker — commit thẳng
// đụng lease đang sống và trả "Video đang xử lý — chờ xong rồi thử lại", để lại
// lỗi dán trên khung xem mãi không tự hết.
func TestUseVersionEnqueuesWhileImagesRun(t *testing.T) {
	service, store := versionService(t)
	one := 1
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &one
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, Path: "images/versions/scene-1/2-x.webp", SHA256: "new", StateName: model.VersionReady, SourceFingerprint: model.SourceFingerprint(store.m.Items[0])})
	service.Edits.Store = runningEditStore{store}
	out, err := service.UseVersion(context.Background(), "job-1", "scene-1", UseVersionInput{N: 2, Project: new(bool)})
	if err != nil {
		t.Fatalf("đổi bản trong lúc images đang chạy không được xung đột: %v", err)
	}
	if !store.change.ImageReview || len(store.change.ImageIDs) != 1 || store.change.ImageIDs[0] != "scene-1" {
		t.Fatalf("đổi bản phải xếp ý định item cho worker: %+v", store.change)
	}
	if store.change.Rerun || store.change.InvalidateFrom != "" {
		t.Fatalf("đổi bản không được đánh dấu chạy lại bước nào: %+v", store.change)
	}
	if out.UsedVersion != 2 {
		t.Fatalf("out=%+v", out)
	}
}

// TestVersionViewStripsStoredDirection: bản vẽ trước khi góp ý chuyển vào
// wrapper có câu "Owner visual direction…" lưu trong phần Cảnh — màn xem phải
// bỏ câu đó cho mọi bản, kể cả bản cũ (sentText giữ nguyên văn).
func TestVersionViewStripsStoredDirection(t *testing.T) {
	service, store := versionService(t)
	one := 1
	store.m.Items[0].Status = "approved"
	store.m.Items[0].CurrentVersion = &one
	stored := "Khung cảnh phố cổ Hội An đêm mưa\nOwner visual direction (does not override tool/read restrictions): \"tối hơn\""
	store.m.Items[0].Versions = []model.Version{{
		Version: 1, Prompt: stored, Path: "images/versions/scene-1/1-a.webp", SHA256: "sha",
		Parts:             model.PromptParts{{Kind: model.PartScene, Text: stored, Source: model.PromptPartSource{Type: "artist", Label: "Hoạ sĩ"}}},
		SourceFingerprint: model.SourceFingerprint(store.m.Items[0]),
	}}
	view, err := service.Read(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	version := view.Images[0].Versions[0]
	scene := ""
	for _, part := range version.Parts {
		if part.Kind == model.PartScene {
			scene = part.Text
		}
	}
	if scene != "Khung cảnh phố cổ Hội An đêm mưa" {
		t.Fatalf("phần Cảnh phải bỏ câu hướng đã lưu: %q", scene)
	}
	if version.SentText != stored {
		t.Fatal("sentText giữ nguyên văn đã gửi nhà cung cấp")
	}
}

// TestImagesViewShapesVersionsAndLegacyParts: GET images trả đủ trường 07 §26;
// bản cũ chưa tách phần thì sentText thành một phần và có cờ partsLegacy.
func TestImagesViewShapesVersionsAndLegacyParts(t *testing.T) {
	service, store := versionService(t)
	one := 1
	store.m.Items[0].CurrentVersion = &one
	store.m.Items[0].Status = "approved"
	store.m.Items[0].Versions = []model.Version{{Version: 1, Prompt: "prompt cũ", Path: "images/versions/scene-1/1-a.webp", SHA256: "sha", SourceFingerprint: model.SourceFingerprint(store.m.Items[0])}}
	view, err := service.Read(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Images) != 2 {
		t.Fatalf("images=%d", len(view.Images))
	}
	image := view.Images[0]
	if image.ID != "scene-1" || image.Kind != "scene" || image.Index != 1 || image.Label != "Cảnh 1" || image.UsedVersion != 1 || image.RefLimit != 4 {
		t.Fatalf("image=%+v", image)
	}
	if view.Images[1].Kind != "cover" || view.Images[1].Label != "Ảnh bìa" {
		t.Fatalf("cover=%+v", view.Images[1])
	}
	version := image.Versions[0]
	if !version.PartsLegacy || version.SentText != "prompt cũ" || len(version.Parts) != 1 || version.State != model.VersionReady {
		t.Fatalf("version=%+v", version)
	}
	if version.PreviewURL != nil {
		t.Fatal("previewUrl phải luôn null")
	}
}

var _ = jobedit.Change{}

// TestCancelVersionClearsOrphanedDrawingRow: dòng `drawing` còn lại khi chuyến
// vẽ đã mất vẫn huỷ được từ giao diện — không tốn ảnh, ảnh về bản đang dùng.
func TestCancelVersionClearsOrphanedDrawingRow(t *testing.T) {
	service, store := versionService(t)
	used := 1
	store.m.Items[0].CurrentVersion = &used
	store.m.Items[0].Status = "running"
	store.m.Items[0].Versions = append(store.m.Items[0].Versions, model.Version{Version: 2, StateName: model.VersionDrawing, StartedAt: "2026-11-01T10:00:00Z"})
	out, err := service.CancelVersion(context.Background(), "job-1", "scene-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled || out.CostImages != 0 || out.UsedVersion != 1 {
		t.Fatalf("out=%+v", out)
	}
	item := store.m.Items[0]
	if item.Versions[1].State() != model.VersionCancelled || item.Versions[1].CostImages != 0 {
		t.Fatalf("bản 2=%+v", item.Versions[1])
	}
	if item.Status != "approved" || item.InFlight != nil {
		t.Fatalf("item=%+v", item)
	}
	if !store.change.ImageRedraw || store.change.InvalidateFrom != domain.CloneImages {
		t.Fatalf("change=%+v", store.change)
	}
}
