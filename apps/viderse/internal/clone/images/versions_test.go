package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func TestCancelledHistoryWithoutPromptStillSerializesPartsAsArray(t *testing.T) {
	service, _ := versionService(t)
	view := service.versionView(context.Background(), loaded{}, model.Item{Width: 1920, Height: 1080}, model.Version{Version: 2, StateName: model.VersionCancelled})
	body, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire["parts"]) != "[]" {
		t.Fatalf("cancelled history breaks the non-null parts array contract: %s", wire["parts"])
	}
}

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

// seedSelectedVersions đặt ảnh scene-1 có bản đang dùng 1 và bản đang xem 2
// với prompt/ảnh tham chiếu khác hẳn. Trả số bản đang xem.
func seedSelectedVersions(store *editStore, selectedState string) int {
	item := &store.m.Items[0]
	one := 1
	item.Status = "approved"
	item.CurrentVersion = &one
	fingerprint := model.SourceFingerprint(*item)
	item.Versions = []model.Version{
		{Version: 1, Path: "images/versions/scene-1/1-used.webp", StateName: model.VersionReady, SourceFingerprint: fingerprint},
		{Version: 2, Path: "images/versions/scene-1/2-selected.webp", StateName: selectedState, SourceFingerprint: fingerprint,
			Parts: model.PromptParts{
				{Kind: model.PartScene, Text: "SELECTED SCENE TWO", Source: model.PromptPartSource{Type: "artist", Label: "Hoạ sĩ"}},
				{Kind: model.PartStyle, Text: "SELECTED STYLE TWO", Source: model.PromptPartSource{Type: "template", Label: "Template"}},
				{Kind: model.PartAvoid, Text: "SELECTED AVOID TWO", Source: model.PromptPartSource{Type: "template", Label: "Luật tránh của Viderse"}},
			},
			Refs: []model.ImageRef{{ID: "video:selected", Role: model.RefStyle, Path: "images/versions/scene-1/2-ref.webp", SHA256: "sha-selected", Source: model.RefSource{Type: "video_image", Label: "Selected ref"}}}},
	}
	return 2
}

// TestRedrawVersionsKeepsSelectedBaseVersion: vẽ lại khi đang xem bản 2 lưu
// đúng bản nguồn 2 (không đụng bản đang dùng 1), sống qua checkpoint, và []
// ảnh tham chiếu là "bỏ hết" chứ không phải "không đổi".
func TestRedrawVersionsKeepsSelectedBaseVersion(t *testing.T) {
	service, store := versionService(t)
	two := seedSelectedVersions(store, model.VersionReady)
	empty := []RedrawRef{}
	out, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{
		ImageIDs:    []string{"scene-1"},
		BaseVersion: &two,
		Parts:       map[string]string{"scene": "EDITED SELECTED SCENE"},
		Refs:        empty,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Versions) != 1 || out.Versions[0].N != 3 {
		t.Fatalf("versions=%+v", out.Versions)
	}
	item := store.m.Items[0]
	if item.Draft == nil || item.Draft.SourceVersion == nil || *item.Draft.SourceVersion != 2 {
		t.Fatalf("mất bản nguồn đã chọn: %+v", item.Draft)
	}
	if item.Draft.Parts[model.PartScene] != "EDITED SELECTED SCENE" {
		t.Fatalf("mất phần prompt đã sửa: %+v", item.Draft.Parts)
	}
	if item.Draft.Refs == nil || len(*item.Draft.Refs) != 0 {
		t.Fatalf("[] ảnh tham chiếu không được giữ là bỏ hết: %+v", item.Draft.Refs)
	}
	if item.CurrentVersion == nil || *item.CurrentVersion != 1 {
		t.Fatalf("bản đang dùng bị đổi: %+v", item.CurrentVersion)
	}
	if item.Status != "pending" || store.m.Revision != 4 {
		t.Fatalf("item=%+v revision=%d", item, store.m.Revision)
	}
	raw, err := json.Marshal(store.m)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded model.Manifest
	if err := json.Unmarshal(raw, &reloaded); err != nil {
		t.Fatal(err)
	}
	draft := reloaded.Items[0].Draft
	if draft == nil || draft.SourceVersion == nil || *draft.SourceVersion != 2 || draft.Refs == nil || len(*draft.Refs) != 0 {
		t.Fatalf("bản nguồn không sống qua checkpoint: %+v", draft)
	}
	if reloaded.Items[0].CurrentVersion == nil || *reloaded.Items[0].CurrentVersion != 1 {
		t.Fatalf("bản đang dùng đổi sau checkpoint: %+v", reloaded.Items[0].CurrentVersion)
	}
}

// TestRedrawVersionsAcceptsFailedBaseVersion: bản lỗi còn prompt/ảnh tham chiếu
// đã lưu vẫn là nguồn hợp lệ cho lần thử lại — không lặng lẽ dùng bản khác.
func TestRedrawVersionsAcceptsFailedBaseVersion(t *testing.T) {
	service, store := versionService(t)
	two := seedSelectedVersions(store, model.VersionFailed)
	if _, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, BaseVersion: &two}); err != nil {
		t.Fatal(err)
	}
	draft := store.m.Items[0].Draft
	if draft == nil || draft.SourceVersion == nil || *draft.SourceVersion != 2 {
		t.Fatalf("draft=%+v", draft)
	}
}

func TestRedrawSelectedBaseChecksInheritedRefsBeforeCommit(t *testing.T) {
	service, store := versionService(t)
	two := seedSelectedVersions(store, model.VersionReady)
	service.RefLimits = func(context.Context, string) (int, bool) { return 0, true }
	_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, BaseVersion: &two})
	var tooMany *TooManyRefsError
	if !errors.As(err, &tooMany) || tooMany.Limit != 0 {
		t.Fatalf("inherited refs must respect the provider limit: %v", err)
	}
	if store.commits != 0 || store.m.Revision != 3 || store.m.Items[0].Draft != nil || *store.m.Items[0].CurrentVersion != 1 {
		t.Fatalf("rejected inherited refs changed the image: %+v", store.m.Items[0])
	}
	// Removing the inherited refs is a valid recovery for a provider with no
	// reference support; an empty override must not restore the selected refs.
	if _, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{
		ImageIDs: []string{"scene-1"}, BaseVersion: &two, Refs: []RedrawRef{},
	}); err != nil {
		t.Fatal(err)
	}
	draft := store.m.Items[0].Draft
	if draft == nil || draft.Refs == nil || len(*draft.Refs) != 0 || *store.m.Items[0].CurrentVersion != 1 {
		t.Fatalf("empty reference override was not preserved: %+v", draft)
	}
}

func TestRedrawSelectedBaseUsesVideoProviderReferenceLimit(t *testing.T) {
	for _, videoProvider := range []string{"chatgpt", "grok", "gemini-1a2b"} {
		t.Run(videoProvider, func(t *testing.T) {
			service, store := versionService(t)
			two := seedSelectedVersions(store, model.VersionReady)
			selected := &store.m.Items[0].Versions[1]
			selected.Refs = append(selected.Refs, model.ImageRef{ID: "video:second", Role: model.RefLayout})
			store.m.Overrides = &model.Overrides{DrawProvider: videoProvider}
			service.RefLimits = func(_ context.Context, provider string) (int, bool) {
				if provider == videoProvider {
					if provider == "chatgpt" {
						return 4, true
					}
					if provider == "grok" {
						return 1, true
					}
					return 0, true
				}
				// The global provider has the opposite limit.
				if videoProvider == "chatgpt" {
					return 1, true
				}
				return 4, true
			}
			_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{ImageIDs: []string{"scene-1"}, BaseVersion: &two})
			if videoProvider == "chatgpt" {
				if err != nil {
					t.Fatal(err)
				}
				if store.m.Items[0].Draft == nil || *store.m.Items[0].Draft.SourceVersion != 2 {
					t.Fatal("selected source lost")
				}
			} else {
				var tooMany *TooManyRefsError
				wantLimit := 1
				if videoProvider == "gemini-1a2b" {
					wantLimit = 0
				}
				if !errors.As(err, &tooMany) || tooMany.Limit != wantLimit {
					t.Fatalf("expected video limit rejection: %v", err)
				}
				if store.commits != 0 || store.m.Items[0].Draft != nil {
					t.Fatal("rejected refs changed the manifest")
				}
			}
		})
	}
}

func TestRedrawRejectsAnotherDrawWhileImageFlightIsLive(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			service, store := versionService(t)
			two := seedSelectedVersions(store, model.VersionFailed)
			store.m.Items[0].InFlight = &model.Flight{Version: 2}
			input := RedrawVersionsInput{ImageIDs: []string{"scene-1"}}
			if explicit {
				input.BaseVersion = &two
			}
			_, err := service.RedrawVersions(context.Background(), "job-1", input)
			requireCode(t, err, domain.CloneErrConflict)
			if store.commits != 0 || store.m.Revision != 3 || store.m.Items[0].Draft != nil {
				t.Fatal("busy rejection changed the image")
			}
		})
	}
}

// TestRedrawVersionsRejectsUnusableBaseVersion: bản nguồn 0/âm/không có/đang vẽ
// hoặc dùng cho nhiều ảnh là lỗi trước khi ghi manifest — không đổi revision,
// bản đang dùng hay draft.
func TestRedrawVersionsRejectsUnusableBaseVersion(t *testing.T) {
	zero, negative, missing, two := 0, -1, 9, 2
	tests := []struct {
		name   string
		ids    []string
		base   *int
		mutate func(*model.Item)
		code   string
	}{
		{name: "zero", ids: []string{"scene-1"}, base: &zero, code: domain.CloneErrInvalidRequest},
		{name: "negative", ids: []string{"scene-1"}, base: &negative, code: domain.CloneErrInvalidRequest},
		{name: "missing", ids: []string{"scene-1"}, base: &missing, code: domain.CloneErrNotFound},
		{name: "drawing", ids: []string{"scene-1"}, base: &two, mutate: func(i *model.Item) { i.Versions[1].StateName = model.VersionDrawing }, code: domain.CloneErrConflict},
		{name: "cancelled", ids: []string{"scene-1"}, base: &two, mutate: func(i *model.Item) { i.Versions[1].StateName = model.VersionCancelled }, code: domain.CloneErrInvalidRequest},
		{name: "stale", ids: []string{"scene-1"}, base: &two, mutate: func(i *model.Item) { i.Versions[1].SourceFingerprint = "other-source" }, code: domain.CloneErrStale},
		{name: "many", ids: []string{"scene-1", "thumbnail"}, base: &two, code: domain.CloneErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, store := versionService(t)
			seedSelectedVersions(store, model.VersionReady)
			if test.mutate != nil {
				test.mutate(&store.m.Items[0])
			}
			_, err := service.RedrawVersions(context.Background(), "job-1", RedrawVersionsInput{
				ImageIDs:    test.ids,
				BaseVersion: test.base,
			})
			requireCode(t, err, test.code)
			if store.commits != 0 {
				t.Fatal("đã ghi manifest cho yêu cầu không hợp lệ")
			}
			if store.m.Revision != 3 {
				t.Fatalf("revision=%d", store.m.Revision)
			}
			item := store.m.Items[0]
			if item.CurrentVersion == nil || *item.CurrentVersion != 1 || item.Draft != nil {
				t.Fatalf("item bị đổi: %+v", item)
			}
		})
	}
}

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
