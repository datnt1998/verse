package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	lastPatch    library.Patch
	lastPatchID  string
	lastPromptIn library.PromptInput
	lastName     string
	lastFromID   string
	deletedID    string
	removedID    string
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
		ScenePromptName: "Theo nhận diện kênh", CoverPromptName: "Chữ hook",
		ImageCount: 5, ImageCountKnown: true, OverlayCount: 1, BackdropChannels: 0,
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
	if item.Prompts.Summary != "Prompt Theo nhận diện kênh · Tách nền · bìa Chữ hook" {
		t.Fatalf("prompt summary = %q", item.Prompts.Summary)
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
	if body.Recipe.Scenes.Style != "cutout" || body.Recipe.Scenes.ScenePromptID != "builtin:scene" {
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
