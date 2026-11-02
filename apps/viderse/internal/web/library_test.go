package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
)

// fakeLibrary là LibraryService giả: hàm cài sẵn + lời gọi gần nhất.
type fakeLibrary struct {
	uploadFilename string
	uploadName     string
	summaries      []library.Summary
	detail         *library.Detail
	template       *domain.Template
	prompts        []domain.ImagePrompt
	prompt         *domain.ImagePrompt
	overlays       []domain.Overlay
	overlay        *domain.Overlay
	usage          map[string][]library.TemplateRef
	export         *library.PromptExport

	err error

	lastPatch      library.Patch
	lastPatchID    string
	lastPromptIn   library.PromptInput
	lastName       string
	lastFromID     string
	deletedID      string
	removedID      string
	lastImport     []string
	lastImportName string
	importResults  []library.ImportResult

	trialStatus   *library.CoverTrialStatus
	sampleStatus  *library.PromptSampleStatus
	lastTrialReq  library.CoverTrialRequest
	lastSampleReq library.PromptSampleRequest
	cancelScope   string
	cancelID      string
	imagePath     string
	scenes        []library.TrialScene
	sampleIDs     map[string][]string
}

func (f *fakeLibrary) List(context.Context) ([]library.Summary, error) {
	return f.summaries, f.err
}

func (f *fakeLibrary) Get(_ context.Context, id string) (*library.Detail, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.detail == nil {
		return nil, &library.Error{Code: library.CodeNotFound, Message: "Template không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
	}
	return f.detail, nil
}

func (f *fakeLibrary) OpenTemplateExport(context.Context, string) (*library.TemplateExport, error) {
	return nil, f.err
}

func (f *fakeLibrary) PreviewStatus(context.Context, string) (*library.PreviewStatus, error) {
	return &library.PreviewStatus{State: "none", Sources: []library.PreviewSource{}}, f.err
}

func (f *fakeLibrary) StartTemplatePreview(_ context.Context, _ string, req library.PreviewRequest) (*library.PreviewStatus, error) {
	return &library.PreviewStatus{State: library.PreviewBuilding, Sources: []library.PreviewSource{}}, f.err
}

func (f *fakeLibrary) CancelTemplatePreview(context.Context, string) error { return f.err }

func (f *fakeLibrary) CoverTrialStatus(context.Context, string) (*library.CoverTrialStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.trialStatus == nil {
		return &library.CoverTrialStatus{Sources: []library.PreviewSource{}, History: []library.CoverTrial{}, Cost: library.TrialCost{Images: 1}}, nil
	}
	return f.trialStatus, nil
}

func (f *fakeLibrary) StartCoverTrial(_ context.Context, _ string, req library.CoverTrialRequest) (*library.CoverTrialStatus, error) {
	f.lastTrialReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.CoverTrialStatus(context.Background(), "")
}

func (f *fakeLibrary) CancelCoverTrial(_ context.Context, templateID, trialID string) error {
	f.cancelScope, f.cancelID = templateID, trialID
	return f.err
}
func (f *fakeLibrary) CoverTrialImagePath(context.Context, string, string) (string, error) {
	return f.imagePath, f.err
}

func (f *fakeLibrary) PromptSampleStatus(context.Context, string) (*library.PromptSampleStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.sampleStatus == nil {
		return &library.PromptSampleStatus{Sources: []library.PreviewSource{}, History: []library.PromptSample{}, Cost: library.TrialCost{Images: 1}}, nil
	}
	return f.sampleStatus, nil
}

func (f *fakeLibrary) StartPromptSample(_ context.Context, _ string, req library.PromptSampleRequest) (*library.PromptSampleStatus, error) {
	f.lastSampleReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.PromptSampleStatus(context.Background(), "")
}

func (f *fakeLibrary) CancelPromptSample(_ context.Context, promptID, sampleID string) error {
	f.cancelScope, f.cancelID = promptID, sampleID
	return f.err
}
func (f *fakeLibrary) PromptSampleImagePath(context.Context, string, string) (string, error) {
	return f.imagePath, f.err
}
func (f *fakeLibrary) PromptScenes(context.Context, string, string) ([]library.TrialScene, error) {
	return f.scenes, f.err
}
func (f *fakeLibrary) PromptSampleIDs(context.Context, []string) map[string][]string {
	return f.sampleIDs
}

func (f *fakeLibrary) Create(_ context.Context, name string) (*domain.Template, error) {
	f.lastName = name
	return f.template, f.err
}

func (f *fakeLibrary) Duplicate(_ context.Context, fromID, name string) (*domain.Template, error) {
	f.lastFromID, f.lastName = fromID, name
	return f.template, f.err
}

func (f *fakeLibrary) Patch(_ context.Context, id string, patch library.Patch) (*domain.Template, error) {
	f.lastPatchID, f.lastPatch = id, patch
	if f.err != nil {
		return nil, f.err
	}
	return f.template, nil
}

func (f *fakeLibrary) Rename(_ context.Context, id, name string, _ *int64) (*domain.Template, error) {
	f.lastName = name
	if f.err != nil {
		return nil, f.err
	}
	return f.template, nil
}

func (f *fakeLibrary) DeleteTemplate(_ context.Context, id string) error {
	f.deletedID = id
	return f.err
}

func (f *fakeLibrary) ImportTemplateFiles(_ context.Context, files []string, opts library.ImportOptions) ([]library.ImportResult, error) {
	f.lastImport = files
	f.lastImportName = opts.Name
	return f.importResults, f.err
}

func (f *fakeLibrary) ListPrompts(context.Context, domain.PromptKind, string) ([]domain.ImagePrompt, error) {
	return f.prompts, f.err
}

func (f *fakeLibrary) PromptUsage(context.Context) (map[string][]library.TemplateRef, error) {
	return f.usage, f.err
}

func (f *fakeLibrary) CreatePrompt(_ context.Context, in library.PromptInput) (*domain.ImagePrompt, error) {
	f.lastPromptIn = in
	return f.prompt, f.err
}

func (f *fakeLibrary) PatchPrompt(_ context.Context, id string, _ library.PromptPatch) (*domain.ImagePrompt, error) {
	f.lastPatchID = id
	return f.prompt, f.err
}

func (f *fakeLibrary) DeletePrompt(_ context.Context, id string) error {
	f.deletedID = id
	return f.err
}

func (f *fakeLibrary) ExportPrompt(context.Context, string) (*library.PromptExport, error) {
	return f.export, f.err
}

func (f *fakeLibrary) ImportPrompt(context.Context, []byte) (*domain.ImagePrompt, error) {
	return f.prompt, f.err
}

func (f *fakeLibrary) ListOverlays(context.Context) ([]domain.Overlay, error) {
	return f.overlays, f.err
}

func (f *fakeLibrary) OverlayUsage(context.Context) (map[string][]library.TemplateRef, error) {
	return f.usage, f.err
}

func (f *fakeLibrary) AddOverlayReader(_ context.Context, filename, displayName string, _ io.Reader, _ int64) (*domain.Overlay, bool, error) {
	f.uploadFilename, f.uploadName = filename, displayName
	return f.overlay, false, f.err
}

func (f *fakeLibrary) RemoveOverlay(_ context.Context, id string) error {
	f.removedID = id
	return f.err
}

func (f *fakeLibrary) OverlayPath(_ context.Context, id string) (string, *domain.Overlay, error) {
	if f.err != nil {
		return "", nil, f.err
	}
	return "", nil, &library.Error{Code: library.CodeNotFound, Message: "Lớp phủ không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
}

// newLibraryHandlers dựng gateway + cookie với fakeLibrary đã ráp route.
func newLibraryHandlers(t *testing.T, fake *fakeLibrary) (*testGateway, string, *[]string) {
	t.Helper()
	tg := newTestGateway(t)
	diagnostics := new([]string)
	MountLibraryDiag(tg.g, fake, nil, func(msg string) { *diagnostics = append(*diagnostics, msg) })
	return tg, tg.sessionCookie(), diagnostics
}

// testTemplate dựng một template mẫu cho wire.
func testTemplate() domain.Template {
	recipe := library.StandardRecipe()
	return domain.Template{
		ID: "11111111-1111-4111-8111-111111111111", Name: "Mẫu thử", Recipe: recipe,
		Revision: 3, CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	}
}

// testSummary dựng tóm tắt mẫu.
func testSummary() library.Summary {
	tpl := testTemplate()
	return library.Summary{
		TemplateSummary: domain.TemplateSummary{Template: tpl, Channels: domain.ChannelRefList{{ID: "c1", Name: "Kênh Một", Position: 0}}},
		ScenePromptName: "Theo nhận diện kênh", CoverPromptName: "Ảnh sạch",
		ImageCount: 5, ImageCountKnown: true, OverlayCount: 1, BackdropChannels: 0,
	}
}

func TestTemplatePreviewAndExportRoutes(t *testing.T) {
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		return w
	}

	w := call(http.MethodGet, "/api/templates/11111111-1111-4111-8111-111111111111/preview", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var status library.PreviewStatus
	decodeJSON(t, w.Body.Bytes(), &status)
	if status.State != "none" || status.Sources == nil {
		t.Fatalf("status = %+v", status)
	}

	w = call(http.MethodPost, "/api/templates/11111111-1111-4111-8111-111111111111/preview", `{"minutes":3}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	decodeJSON(t, w.Body.Bytes(), &status)
	if status.State != library.PreviewBuilding {
		t.Fatalf("start state = %+v", status)
	}

	if w = call(http.MethodDelete, "/api/templates/11111111-1111-4111-8111-111111111111/preview", ""); w.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d %s", w.Code, w.Body.String())
	}

	fake.err = &library.Error{Code: library.CodeNotFound, Message: "Template không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
	w = call(http.MethodGet, "/api/templates/11111111-1111-4111-8111-111111111111/export", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	var failure map[string]any
	decodeJSON(t, w.Body.Bytes(), &failure)
	if failure["code"] != library.CodeNotFound {
		t.Fatalf("failure = %v", failure)
	}
}

func TestLibraryTemplateListShape(t *testing.T) {
	fake := &fakeLibrary{summaries: []library.Summary{testSummary()}}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/templates", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []TemplateSummaryWire `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &body)
	if len(body.Items) != 1 {
		t.Fatalf("items = %+v", body.Items)
	}
	item := body.Items[0]
	if item.ID != "11111111-1111-4111-8111-111111111111" || item.Name != "Mẫu thử" || item.Revision != 3 {
		t.Fatalf("item = %+v", item)
	}
	if item.Images.PerVideo == nil || *item.Images.PerVideo != 5 || item.Images.Mode != "fixed" {
		t.Fatalf("images = %+v", item.Images)
	}
	if item.Preview.State != domain.PreviewNone || len(item.Channels) != 1 {
		t.Fatalf("preview/kênh = %+v / %+v", item.Preview, item.Channels)
	}
	if item.OverlayCount != 1 || item.Backdrop.Enabled {
		t.Fatalf("đếm = %+v", item)
	}
}

func TestLibraryTemplateDetailHasRecipeAndPrice(t *testing.T) {
	summary := testSummary()
	fake := &fakeLibrary{detail: &library.Detail{
		Summary: summary,
		Price:   library.PriceEstimate{PerImageUSD: 0.04, Known: true, ImagesPerVideo: new(5), IncludesCover: true, Mode: "fixed", TotalUSD: new(0.24)},
	}}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/templates/"+summary.ID, cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body TemplateDetailWire
	decodeJSON(t, w.Body.Bytes(), &body)
	if body.Recipe.Scenes.Style != "full" || body.Recipe.Scenes.ScenePromptID != "builtin:scene" || body.Recipe.Scenes.CoverPromptID != "builtin:cover-clean" {
		t.Fatalf("recipe = %+v", body.Recipe.Scenes)
	}
	if body.SavedAt.IsZero() || !body.Price.Known || body.Price.PerImage != 0.04 || body.Price.TotalUSD == nil || *body.Price.TotalUSD != 0.24 {
		t.Fatalf("price = %+v", body.Price)
	}
	if body.Price.ImagesPerVideo == nil || *body.Price.ImagesPerVideo != 5 {
		t.Fatalf("price ảnh = %+v", body.Price)
	}
}

func TestLibraryCreateAndDuplicate(t *testing.T) {
	tpl := testTemplate()
	fake := &fakeLibrary{template: &tpl, detail: &library.Detail{Summary: testSummary()}}
	tg, cookie, _ := newLibraryHandlers(t, fake)

	w := tg.do(http.MethodPost, "/api/templates", cookie, `{"name":"Mới"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("tạo: %d %s", w.Code, w.Body.String())
	}
	if fake.lastName != "Mới" || fake.lastFromID != "" {
		t.Fatalf("lời gọi = %q %q", fake.lastName, fake.lastFromID)
	}

	w = tg.do(http.MethodPost, "/api/templates", cookie, `{"fromId":"11111111-1111-4111-8111-111111111111"}`)
	if w.Code != http.StatusCreated || fake.lastFromID != tpl.ID || fake.lastName != "" {
		t.Fatalf("nhân bản: %d %+v", w.Code, fake)
	}

	if w := tg.do(http.MethodPost, "/api/templates", cookie, `{"name":"x","bogus":1}`); w.Code != http.StatusBadRequest {
		t.Fatalf("field lạ: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryPatchRevisionGuardAndConflict(t *testing.T) {
	tpl := testTemplate()
	fake := &fakeLibrary{template: &tpl, detail: &library.Detail{Summary: testSummary()}}
	tg, cookie, _ := newLibraryHandlers(t, fake)

	w := tg.do(http.MethodPatch, "/api/templates/"+tpl.ID, cookie, `{"revision":3,"recipe":{"scenes":{"images":{"mode":"fixed","count":9}}}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", w.Code, w.Body.String())
	}
	if fake.lastPatch.Revision != 3 || fake.lastPatch.Recipe == nil || !strings.Contains(string(fake.lastPatch.Recipe), `"count":9`) {
		t.Fatalf("patch nhận = %+v", fake.lastPatch)
	}

	fake.err = &library.Error{Code: library.CodeConflict, Message: "Template vừa được sửa ở nơi khác", Recovery: "Tải lại trang rồi thử lại"}
	w = tg.do(http.MethodPatch, "/api/templates/"+tpl.ID, cookie, `{"revision":2}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("409: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	decodeJSON(t, w.Body.Bytes(), &body)
	if body["code"] != library.CodeConflict || body["recovery"] != "Tải lại trang rồi thử lại" {
		t.Fatalf("body = %+v", body)
	}

	fake.err = nil
	w = tg.do(http.MethodPatch, "/api/templates/"+tpl.ID, cookie, `{"recipe":{}}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Thiếu revision") {
		t.Fatalf("thiếu revision: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryValidateFieldErrorsGoToWire(t *testing.T) {
	tpl := testTemplate()
	fake := &fakeLibrary{
		template: &tpl,
		err: &library.Error{
			Code: library.CodeInvalidInput, Message: "Cấu hình template không hợp lệ",
			Fields: []library.FieldError{{Field: "scenes.images.count", Message: "Số ảnh cảnh không hợp lệ", Recovery: "Nhập số từ 1 đến 60"}},
		},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodPatch, "/api/templates/"+tpl.ID, cookie, `{"revision":1,"recipe":{"scenes":{"images":{"mode":"fixed","count":0}}}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Code   string               `json:"code"`
		Fields []library.FieldError `json:"fields"`
	}
	decodeJSON(t, w.Body.Bytes(), &body)
	if body.Code != library.CodeInvalidInput || len(body.Fields) != 1 || body.Fields[0].Field != "scenes.images.count" {
		t.Fatalf("body = %+v", body)
	}
}

func TestLibraryDeleteBlockedByChannels(t *testing.T) {
	tpl := testTemplate()
	fake := &fakeLibrary{
		template: &tpl,
		err: &library.Error{
			Code: library.CodeConflict, Message: "Đang dùng ở 2 kênh. Bỏ khỏi kênh trước rồi xoá.",
			Recovery: "Mở kênh đang dùng…", Channels: []string{"Kênh Một", "Kênh Hai"},
		},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodDelete, "/api/templates/"+tpl.ID, cookie, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Message  string   `json:"error"`
		Channels []string `json:"channels"`
	}
	decodeJSON(t, w.Body.Bytes(), &body)
	if !strings.Contains(body.Message, "Đang dùng ở 2 kênh") || len(body.Channels) != 2 {
		t.Fatalf("body = %+v", body)
	}

	fake.err = nil
	w = tg.do(http.MethodDelete, "/api/templates/"+tpl.ID, cookie, "")
	if w.Code != http.StatusNoContent || fake.deletedID != tpl.ID {
		t.Fatalf("xoá: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryPromptsWireAndErrors(t *testing.T) {
	prompt := domain.ImagePrompt{
		ID: "22222222-2222-4222-8222-222222222222", Name: "Của tôi", Kind: domain.PromptCover,
		Composition: "hook", Prompt: "vẽ", UpdatedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
	}
	fake := &fakeLibrary{
		prompts: []domain.ImagePrompt{prompt}, prompt: &prompt,
		usage: map[string][]library.TemplateRef{prompt.ID: {{ID: "t1", Name: "Template Một"}}},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/image-prompts?kind=cover&q=v%C4%A9", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []ImagePromptWire `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &body)
	if len(body.Items) != 1 || body.Items[0].Samples == nil || len(body.Items[0].Samples) != 0 {
		t.Fatalf("items = %+v", body.Items)
	}
	if len(body.Items[0].UsedBy) != 1 || body.Items[0].UsedBy[0].Name != "Template Một" {
		t.Fatalf("usedBy = %+v", body.Items[0].UsedBy)
	}

	fake.err = &library.Error{Code: library.CodeBuiltin, Message: "Prompt có sẵn của Viderse không sửa và không xoá được", Recovery: "Nhân bản để có bản của bạn rồi sửa"}
	w = tg.do(http.MethodPatch, "/api/image-prompts/builtin:scene", cookie, `{"prompt":"x"}`)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), library.CodeBuiltin) {
		t.Fatalf("403: %d %s", w.Code, w.Body.String())
	}

	fake.err = &library.Error{Code: library.CodeInUse, Message: "Đang dùng ở Template Một.", Recovery: "Bỏ chọn…", UsedBy: []string{"t1"}}
	w = tg.do(http.MethodDelete, "/api/image-prompts/"+prompt.ID, cookie, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("409: %d %s", w.Code, w.Body.String())
	}
	var problem struct {
		Code   string   `json:"code"`
		UsedBy []string `json:"usedBy"`
	}
	decodeJSON(t, w.Body.Bytes(), &problem)
	if problem.Code != library.CodeInUse || len(problem.UsedBy) != 1 {
		t.Fatalf("body = %+v", problem)
	}
}

func TestLibraryPromptExportImport(t *testing.T) {
	prompt := domain.ImagePrompt{ID: "33333333-3333-4333-8333-333333333333", Name: "Xuất", Kind: domain.PromptScene}
	fake := &fakeLibrary{
		prompt:  &prompt,
		export:  &library.PromptExport{Format: library.PromptExportFormat, Version: 1, Name: "Xuất", Kind: "scene", Prompt: "nội dung"},
		prompts: []domain.ImagePrompt{prompt},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/image-prompts/"+prompt.ID+"/export", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, ".json") {
		t.Fatalf("content-disposition = %q", cd)
	}
	var exported library.PromptExport
	decodeJSON(t, w.Body.Bytes(), &exported)
	if exported.Format != library.PromptExportFormat || exported.Prompt != "nội dung" {
		t.Fatalf("export = %+v", exported)
	}

	w = tg.do(http.MethodPost, "/api/image-prompts/import", cookie, `{"format":"viderse-image-prompt","version":1,"name":"Xuất","kind":"scene","prompt":"nội dung"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}

	fake.err = &library.Error{Code: library.CodeInvalidInput, Message: "Tệp không phải prompt của Viderse", Recovery: "Chọn tệp .json…"}
	w = tg.do(http.MethodPost, "/api/image-prompts/import", cookie, `{"format":"other"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("import lỗi: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryOverlaysWire(t *testing.T) {
	overlay := domain.Overlay{
		ID: "44444444-4444-4444-8444-444444444444", SHA256: strings.Repeat("a", 64), Name: "Mưa",
		Mime: "video/mp4", Bytes: 100, Background: domain.BgBlack, Builtin: "rain",
		CreatedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
	}
	fake := &fakeLibrary{
		overlays: []domain.Overlay{overlay},
		usage:    map[string][]library.TemplateRef{overlay.ID: {{ID: "t1", Name: "Template Một"}}},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/overlays", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []OverlayWire `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &body)
	if len(body.Items) != 1 || body.Items[0].URL != "/api/overlays/"+overlay.ID+"/file" || body.Items[0].BuiltIn != "rain" {
		t.Fatalf("items = %+v", body.Items)
	}
	if len(body.Items[0].UsedBy) != 1 {
		t.Fatalf("usedBy = %+v", body.Items[0].UsedBy)
	}

	w = tg.do(http.MethodGet, "/api/overlays/"+overlay.ID+"/file", cookie, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("file thiếu: %d %s", w.Code, w.Body.String())
	}

	w = tg.do(http.MethodPost, "/api/overlays", cookie, `{}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "multipart") {
		t.Fatalf("thiếu multipart: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryOverlaysMultipartUpload(t *testing.T) {
	overlay := domain.Overlay{
		ID: "55555555-5555-4555-8555-555555555555", SHA256: strings.Repeat("b", 64), Name: "Mưa",
		Mime: "video/mp4", Bytes: 3, Background: domain.BgNone,
	}
	fake := &fakeLibrary{overlay: &overlay}
	tg, cookie, _ := newLibraryHandlers(t, fake)

	var buf strings.Builder
	boundary := "xyz"
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"mua.mp4\"\r\n")
	buf.WriteString("Content-Type: video/mp4\r\n\r\nabc\r\n")
	buf.WriteString("--" + boundary + "--\r\n")

	r := httptest.NewRequest(http.MethodPost, "/api/overlays", strings.NewReader(buf.String()))
	r.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
	r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	var body OverlayWire
	decodeJSON(t, w.Body.Bytes(), &body)
	if body.ID != overlay.ID || body.URL == "" {
		t.Fatalf("body = %+v", body)
	}
	if fake.uploadFilename != "mua.mp4" {
		t.Fatalf("filename truyền service = %q", fake.uploadFilename)
	}
}

func TestLibraryInternalErrorGoesToDiag(t *testing.T) {
	fake := &fakeLibrary{err: context.DeadlineExceeded}
	tg, cookie, diagnostics := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodGet, "/api/templates", cookie, "")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"code":"internal"`) {
		t.Fatalf("500: %d %s", w.Code, w.Body.String())
	}
	if len(*diagnostics) != 1 || !strings.Contains((*diagnostics)[0], "deadline") {
		t.Fatalf("diag = %v", *diagnostics)
	}
}

func TestLibraryTemplateMixDelegation(t *testing.T) {
	fake := &fakeLibrary{}
	tg := newTestGateway(t)
	called := ""
	mix := fakeMix{serve: func(_ http.ResponseWriter, _ *http.Request, templateID string) { called = templateID }}
	MountLibrary(tg.g, fake, mix)
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodGet, "/api/templates/11111111-1111-4111-8111-111111111111/backdrop/mix?lengthSec=600", cookie, "")
	if w.Code != http.StatusOK || called != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("mix: %d called=%q", w.Code, called)
	}
}

// ── nhập template cũ ───────────────────────────────────────────────────────

// templateImportBody dựng multipart với một tệp .ytf-template.zip giả và
// trường name tuỳ chọn.
func templateImportBody(t *testing.T, fileName string, data []byte, name string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if name != "" {
		if err := writer.WriteField("name", name); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

// postMultipart gửi một request multipart qua gateway thử nghiệm.
func postMultipart(t *testing.T, tg *testGateway, cookie, path, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, body)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Host = "127.0.0.1:8789"
	r.Header.Set("Content-Type", contentType)
	r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
	w := httptest.NewRecorder()
	tg.g.ServeHTTP(w, r)
	return w
}

func TestLibraryTemplateImportReturnsPerFileResults(t *testing.T) {
	fake := &fakeLibrary{importResults: []library.ImportResult{
		{Source: "one.ytf-template.zip", Name: "Đêm khuya", TemplateID: "tpl-1", OK: true, Warnings: []string{"Kênh nền twitch:x chưa có"}, MediaImported: 3, MediaReused: 1},
		{Source: "two.ytf-template.zip", Name: "two.ytf-template.zip", OK: false, Error: "Tệp không phải template của yt-factory"},
	}}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	body, contentType := templateImportBody(t, "one.ytf-template.zip", []byte("PK\x03\x04 dữ liệu"), "")
	w := postMultipart(t, tg, cookie, "/api/templates/import", contentType, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Items    []TemplateImportItemWire `json:"items"`
		Imported int                      `json:"imported"`
		Failed   int                      `json:"failed"`
	}
	decodeJSON(t, w.Body.Bytes(), &out)
	if len(out.Items) != 2 || out.Imported != 1 || out.Failed != 1 {
		t.Fatalf("kết quả = %+v", out)
	}
	if out.Items[0].Name != "Đêm khuya" || len(out.Items[0].Warnings) != 1 || out.Items[0].Media != 3 || out.Items[0].MediaReused != 1 {
		t.Fatalf("dòng 1 = %+v", out.Items[0])
	}
	if out.Items[1].OK || out.Items[1].Error == "" {
		t.Fatalf("dòng 2 = %+v", out.Items[1])
	}
	if len(fake.lastImport) != 1 {
		t.Fatalf("service nhận %d tệp", len(fake.lastImport))
	}
	// Tệp tải lên đi qua đĩa tạm rồi phải được dọn.
	if _, err := os.Stat(fake.lastImport[0]); !os.IsNotExist(err) {
		t.Fatalf("tệp tạm còn lại: %v", err)
	}
}

func TestLibraryTemplateImportRejectsEmptyUpload(t *testing.T) {
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	empty, emptyType := templateImportBody(t, "", nil, "")
	w := postMultipart(t, tg, cookie, "/api/templates/import", emptyType, empty)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("thiếu tệp: %d %s", w.Code, w.Body.String())
	}
}

func TestLibraryTemplateImportRequiresMultipart(t *testing.T) {
	fake := &fakeLibrary{}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	w := tg.do(http.MethodPost, "/api/templates/import", cookie, `{"file":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "tệp tải lên") {
		t.Fatalf("body = %s", w.Body.String())
	}
	if len(fake.lastImport) != 0 {
		t.Fatalf("service vẫn được gọi: %v", fake.lastImport)
	}
}

// fakeMix là TemplateBackdropMix đếm lời gọi.
type fakeMix struct {
	serve func(w http.ResponseWriter, r *http.Request, templateID string)
}

func (f fakeMix) ServeMix(w http.ResponseWriter, r *http.Request, templateID string) {
	f.serve(w, r, templateID)
	w.WriteHeader(http.StatusOK)
}

// jsonRoundTrip giữ import encoding/json dùng trong test.
func jsonRoundTrip(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

var _ = jsonRoundTrip

func TestCoverTrialRoutes(t *testing.T) {
	fake := &fakeLibrary{
		trialStatus: &library.CoverTrialStatus{
			Sources: []library.PreviewSource{{JobID: "j1", Title: "Video xong", DurationSec: 300}},
			History: []library.CoverTrial{{
				ID: "t1", TemplateID: "11111111-1111-4111-8111-111111111111", JobID: "j1",
				JobTitle: "Video xong", PromptID: "builtin:cover-clean", PromptName: "Ảnh sạch",
				Composition: "clean", State: library.TrialReady, CostImages: 1,
			}},
			Cost:  library.TrialCost{Images: 1, Provider: "ChatGPT", Label: "ChatGPT", QuotaLeft: 7},
			Ready: true,
		},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		return w
	}
	const base = "/api/templates/11111111-1111-4111-8111-111111111111/cover-trials"
	w := call(http.MethodGet, base, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var status struct {
		Sources []library.PreviewSource `json:"sources"`
		History []struct {
			ID       string `json:"id"`
			ImageURL string `json:"imageUrl"`
		} `json:"history"`
		Cost  library.TrialCost `json:"cost"`
		Ready bool              `json:"ready"`
	}
	decodeJSON(t, w.Body.Bytes(), &status)
	if len(status.Sources) != 1 || len(status.History) != 1 || !status.Ready {
		t.Fatalf("status = %+v", status)
	}
	if status.History[0].ImageURL != base+"/t1/image" {
		t.Fatalf("imageUrl = %q", status.History[0].ImageURL)
	}
	if status.Cost.Images != 1 || status.Cost.Provider != "ChatGPT" || status.Cost.QuotaLeft != 7 {
		t.Fatalf("cost = %+v", status.Cost)
	}

	w = call(http.MethodPost, base, `{"jobId":"j1","confirm":true}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	if !fake.lastTrialReq.Confirm || fake.lastTrialReq.JobID != "j1" {
		t.Fatalf("req = %+v", fake.lastTrialReq)
	}

	fake.err = &library.Error{Code: library.CodeConfirmRequired, Message: "Cần xác nhận", Recovery: "Xác nhận trong hộp thoại"}
	w = call(http.MethodPost, base, `{"jobId":"j1"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("confirm: %d %s", w.Code, w.Body.String())
	}

	fake.err = &library.Error{Code: library.CodeQuota, Message: "Hết hạn mức", Recovery: "Chờ mai"}
	w = call(http.MethodPost, base, `{"jobId":"j1","confirm":true}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("quota: %d %s", w.Code, w.Body.String())
	}
}

func TestPromptSampleRoutes(t *testing.T) {
	fake := &fakeLibrary{
		sampleStatus: &library.PromptSampleStatus{
			Sources: []library.PreviewSource{{JobID: "j1", Title: "Video xong"}},
			History: []library.PromptSample{{
				ID: "s1", PromptID: "p1", PromptName: "Mẫu", PromptKind: "cover",
				Composition: "clean", VideoID: "j1", State: library.TrialReady, CostImages: 1,
			}},
			Cost:  library.TrialCost{Images: 1, Provider: "ChatGPT", Label: "ChatGPT", QuotaLeft: 3},
			Ready: true,
		},
		scenes:    []library.TrialScene{{ID: "sc1", Label: "Cảnh 1 · Mở đầu"}},
		sampleIDs: map[string][]string{"p1": {"s1"}},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		return w
	}
	const base = "/api/image-prompts/p1/samples"
	w := call(http.MethodGet, base, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var status struct {
		History []struct {
			ID       string `json:"id"`
			ImageURL string `json:"imageUrl"`
		} `json:"history"`
	}
	decodeJSON(t, w.Body.Bytes(), &status)
	if len(status.History) != 1 || status.History[0].ImageURL != base+"/s1/image" {
		t.Fatalf("history = %+v", status.History)
	}

	w = call(http.MethodGet, base+"/scenes?videoId=j1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("scenes: %d %s", w.Code, w.Body.String())
	}
	var scenes struct {
		Items []library.TrialScene `json:"items"`
	}
	decodeJSON(t, w.Body.Bytes(), &scenes)
	if len(scenes.Items) != 1 || scenes.Items[0].ID != "sc1" {
		t.Fatalf("scenes = %+v", scenes.Items)
	}

	w = call(http.MethodPost, base, `{"videoId":"j1","confirm":true}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	if !fake.lastSampleReq.Confirm || fake.lastSampleReq.VideoID != "j1" {
		t.Fatalf("req = %+v", fake.lastSampleReq)
	}
}

// TestTrialCancelRoutesSurfaceBusinessState: UI dựa vào mã HTTP để biết huỷ đã
// nhận (204) hay quá muộn (409) và hàng không còn (404) — không được thành 500.
func TestTrialCancelRoutesSurfaceBusinessState(t *testing.T) {
	const templateID = "11111111-1111-4111-8111-111111111111"
	fake := &fakeLibrary{}
	tg, cookie, diagnostics := newLibraryHandlers(t, fake)
	call := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader("{}"))
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Host, r.RemoteAddr = "127.0.0.1:8789", "192.0.2.1:1234"
		r.AddCookie(&http.Cookie{Name: testCookie, Value: cookie})
		w := httptest.NewRecorder()
		tg.g.ServeHTTP(w, r)
		return w
	}

	coverCancel := "/api/templates/" + templateID + "/cover-trials/t1/cancel"
	w := call(http.MethodPost, coverCancel)
	if w.Code != http.StatusNoContent {
		t.Fatalf("huỷ = %d %s", w.Code, w.Body.String())
	}
	if fake.cancelScope != templateID || fake.cancelID != "t1" {
		t.Fatalf("huỷ nhận %q/%q", fake.cancelScope, fake.cancelID)
	}

	fake.err = &library.Error{Code: library.CodeConflict, Message: "Lần vẽ này đã xong", Recovery: "Tải lại trạng thái để xem kết quả"}
	w = call(http.MethodPost, coverCancel)
	if w.Code != http.StatusConflict {
		t.Fatalf("huỷ quá muộn = %d %s", w.Code, w.Body.String())
	}
	body := jsonRoundTrip(t, w.Body.Bytes())
	if body["code"] != library.CodeConflict || body["recovery"] == "" {
		t.Fatalf("body = %v", body)
	}

	sampleCancel := "/api/image-prompts/p1/samples/s1/cancel"
	w = call(http.MethodPost, sampleCancel)
	if w.Code != http.StatusConflict {
		t.Fatalf("huỷ mẫu quá muộn = %d %s", w.Code, w.Body.String())
	}
	if fake.cancelScope != "p1" || fake.cancelID != "s1" {
		t.Fatalf("huỷ mẫu nhận %q/%q", fake.cancelScope, fake.cancelID)
	}

	fake.err = &library.Error{Code: library.CodeNotFound, Message: "Lần vẽ thử không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
	w = call(http.MethodPost, coverCancel)
	if w.Code != http.StatusNotFound {
		t.Fatalf("huỷ hàng không còn = %d %s", w.Code, w.Body.String())
	}
	if len(*diagnostics) != 0 {
		t.Fatalf("lỗi nghiệp vụ bị ghi thành lỗi nội bộ: %v", *diagnostics)
	}
}

// Prompt wire phải kèm URL ảnh mẫu thật để lưới thẻ vẽ được ảnh mà không gọi
// thêm một route cho từng prompt.
func TestPromptWireSampleURLs(t *testing.T) {
	prompt := domain.ImagePrompt{
		ID: "22222222-2222-4222-8222-222222222222", Name: "Của tôi", Kind: domain.PromptCover,
		Composition: "hook", Prompt: "vẽ", UpdatedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
	}
	fake := &fakeLibrary{
		prompts:   []domain.ImagePrompt{prompt},
		sampleIDs: map[string][]string{prompt.ID: {"s1"}},
	}
	tg, cookie, _ := newLibraryHandlers(t, fake)
	want := "/api/image-prompts/" + prompt.ID + "/samples/s1/image"

	var list struct {
		Items []ImagePromptWire `json:"items"`
	}
	w := tg.do(http.MethodGet, "/api/image-prompts", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list.Items) != 1 || len(list.Items[0].SampleURLs) != 1 || list.Items[0].SampleURLs[0] != want {
		t.Fatalf("sampleUrls list = %+v", list.Items)
	}
	if len(list.Items[0].Samples) != 1 || list.Items[0].Samples[0] != "s1" {
		t.Fatalf("samples list = %+v", list.Items[0].Samples)
	}

	var one ImagePromptWire
	w = tg.do(http.MethodGet, "/api/image-prompts/"+prompt.ID, cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	decodeJSON(t, w.Body.Bytes(), &one)
	if len(one.SampleURLs) != 1 || one.SampleURLs[0] != want {
		t.Fatalf("sampleUrls get = %+v", one.SampleURLs)
	}
}
