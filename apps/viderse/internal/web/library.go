package web

// HTTP của mảng Thư viện: template, prompt ảnh, lớp phủ. Handler chỉ đọc
// request → gọi service → ghi JSON; hình wire tách hẳn khỏi kiểu domain và
// theo API đã công bố. Mọi lỗi đi qua đúng một bảng map (writeLibraryError).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// LibraryService là mặt nghiệp vụ mảng Thư viện — hiện thực là *library.Service.
type LibraryService interface {
	List(ctx context.Context) ([]library.Summary, error)
	Get(ctx context.Context, id string) (*library.Detail, error)
	Create(ctx context.Context, name string) (*domain.Template, error)
	Duplicate(ctx context.Context, fromID, name string) (*domain.Template, error)
	Patch(ctx context.Context, id string, patch library.Patch) (*domain.Template, error)
	Rename(ctx context.Context, id, name string, expectRevision *int64) (*domain.Template, error)
	DeleteTemplate(ctx context.Context, id string) error

	ListPrompts(ctx context.Context, kind domain.PromptKind, q string) ([]domain.ImagePrompt, error)
	PromptUsage(ctx context.Context) (map[string][]library.TemplateRef, error)
	CreatePrompt(ctx context.Context, in library.PromptInput) (*domain.ImagePrompt, error)
	PatchPrompt(ctx context.Context, id string, in library.PromptPatch) (*domain.ImagePrompt, error)
	DeletePrompt(ctx context.Context, id string) error
	ExportPrompt(ctx context.Context, id string) (*library.PromptExport, error)
	ImportPrompt(ctx context.Context, data []byte) (*domain.ImagePrompt, error)

	ListOverlays(ctx context.Context) ([]domain.Overlay, error)
	OverlayUsage(ctx context.Context) (map[string][]library.TemplateRef, error)
	AddOverlayReader(ctx context.Context, filename, displayName string, r io.Reader, size int64) (*domain.Overlay, bool, error)
	RemoveOverlay(ctx context.Context, id string) error
	OverlayPath(ctx context.Context, id string) (string, *domain.Overlay, error)
}

// LibraryDiag là điểm tới của lỗi nội bộ 500 (như RivalDiag).
type LibraryDiag func(msg string)

// MountLibrary đăng ký các route Thư viện trên gateway. mix là route
// /api/templates/{id}/backdrop/mix của mảng Video nền (sở hữu riêng, mount
// cùng tiền tố); nil = trả 404 cho route đó.
func MountLibrary(g *Gateway, svc LibraryService, mix TemplateBackdropMix) {
	MountLibraryDiag(g, svc, mix, nil)
}

// MountLibraryDiag giống MountLibrary nhưng có điểm tới cho lỗi nội bộ.
func MountLibraryDiag(g *Gateway, svc LibraryService, mix TemplateBackdropMix, diag LibraryDiag) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete}
	g.Handle(http.MethodGet, "/api/templates", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listTemplates(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/templates", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		createTemplate(w, r, svc, diag)
	})
	for _, method := range methods {
		g.Handle(method, "/api/templates/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routeTemplates(w, r, svc, mix, diag)
		})
	}
	g.Handle(http.MethodGet, "/api/image-prompts", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listPrompts(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/image-prompts", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		createPrompt(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/image-prompts/import", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		importPrompt(w, r, svc, diag)
	})
	for _, method := range methods {
		g.Handle(method, "/api/image-prompts/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routePrompts(w, r, svc, diag)
		})
	}
	g.Handle(http.MethodGet, "/api/overlays", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listOverlays(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/overlays", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		addOverlay(w, r, svc, diag)
	})
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete} {
		g.Handle(method, "/api/overlays/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routeOverlays(w, r, svc, diag)
		})
	}
}

// ── wire template ──────────────────────────────────────────────────────────

type channelRefWire struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position,omitempty"`
}

type templatePromptsWire struct {
	SceneID   string `json:"sceneId"`
	SceneName string `json:"sceneName"`
	CoverID   string `json:"coverId"`
	CoverName string `json:"coverName"`
	Summary   string `json:"summary"`
}

type templateImagesWire struct {
	Mode     string `json:"mode"`
	Count    int    `json:"count"`
	Minutes  int    `json:"minutes"`
	PerVideo *int   `json:"perVideo"`
	Known    bool   `json:"known"`
}

type templateBackdropWire struct {
	ChannelCount int  `json:"channelCount"`
	VideoCount   int  `json:"videoCount"`
	ClipCount    *int `json:"clipCount"`
	ClipSec      int  `json:"clipSec"`
	Enabled      bool `json:"enabled"`
}

type templatePreviewWire struct {
	State   string     `json:"state"`
	BuiltAt *time.Time `json:"builtAt"`
}

type TemplateSummaryWire struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Revision     int64                `json:"revision"`
	Style        string               `json:"style"`
	Framing      string               `json:"framing,omitempty"`
	Images       templateImagesWire   `json:"images"`
	Prompts      templatePromptsWire  `json:"prompts"`
	Backdrop     templateBackdropWire `json:"backdrop"`
	OverlayCount int                  `json:"overlayCount"`
	Channels     []channelRefWire     `json:"channels"`
	Preview      templatePreviewWire  `json:"preview"`
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
}

type TemplateDetailWire struct {
	TemplateSummaryWire
	Recipe  domain.Recipe     `json:"recipe"`
	Price   priceEstimateWire `json:"price"`
	SavedAt time.Time         `json:"savedAt"` // = updatedAt, tên hợp đồng cho UI
}

type priceEstimateWire struct {
	PerImage       float64  `json:"perImage"`
	Known          bool     `json:"known"`
	ImagesPerVideo *int     `json:"imagesPerVideo"`
	IncludesCover  bool     `json:"includesCover"`
	Mode           string   `json:"mode"`
	PerMinutes     int      `json:"perMinutes,omitempty"`
	TotalUSD       *float64 `json:"totalUSD"`
}

// TemplateSummaryToWire đổi tóm tắt nghiệp vụ sang hình wire.
func TemplateSummaryToWire(s library.Summary) TemplateSummaryWire {
	preview := templatePreviewWire{State: s.PreviewState(), BuiltAt: s.PreviewBuiltAt}
	images := templateImagesWire{
		Mode: s.Recipe.Scenes.Images.Mode, Count: s.Recipe.Scenes.Images.Count,
		Minutes: s.Recipe.Scenes.Images.Minutes, Known: s.ImageCountKnown,
	}
	if s.ImageCountKnown {
		count := s.ImageCount
		images.PerVideo = &count
	}
	backdrop := templateBackdropWire{
		ChannelCount: s.BackdropChannels, VideoCount: s.BackdropVideos,
		ClipSec: 60, Enabled: s.Recipe.Backdrop != nil,
	}
	if b := s.Recipe.Backdrop; b != nil {
		backdrop.ClipCount, backdrop.ClipSec = b.ClipCount, b.ClipSec
	}
	channels := make([]channelRefWire, 0, len(s.Channels))
	for _, ch := range s.Channels {
		channels = append(channels, channelRefWire{ID: ch.ID, Name: ch.Name, Position: ch.Position})
	}
	return TemplateSummaryWire{
		ID: s.ID, Name: s.Name, Revision: s.Revision,
		Style: s.Recipe.Scenes.Style, Framing: s.Recipe.Scenes.Framing,
		Images: images,
		Prompts: templatePromptsWire{
			SceneID: s.Recipe.Scenes.ScenePromptID, SceneName: displayPromptName(s.ScenePromptName),
			CoverID: s.Recipe.Scenes.CoverPromptID, CoverName: displayPromptName(s.CoverPromptName),
			Summary: promptSummary(s),
		},
		Backdrop:     backdrop,
		OverlayCount: s.OverlayCount,
		Channels:     channels,
		Preview:      preview,
		CreatedAt:    s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// promptSummary là câu "Prompt <tên> · <cách ghép> · bìa <tên>" của thẻ.
func promptSummary(s library.Summary) string {
	style := "Toàn khung"
	if s.Recipe.Scenes.Style == library.StyleCutout {
		style = "Tách nền"
		if s.Recipe.Scenes.Framing == library.FramingClose {
			style += " · Cận cảnh"
		}
	}
	return "Prompt " + displayPromptName(s.ScenePromptName) + " · " + style + " · bìa " + displayPromptName(s.CoverPromptName)
}

// displayPromptName tránh chuỗi rỗng khi prompt đã bị xoá tay khỏi DB.
func displayPromptName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Prompt đã xoá"
	}
	return name
}

// TemplateDetailToWire ghép chi tiết.
func TemplateDetailToWire(d *library.Detail) TemplateDetailWire {
	return TemplateDetailWire{
		TemplateSummaryWire: TemplateSummaryToWire(d.Summary),
		Recipe:              d.Template.Recipe,
		SavedAt:             d.Template.UpdatedAt,
		Price: priceEstimateWire{
			PerImage: d.Price.PerImageUSD, Known: d.Price.Known,
			ImagesPerVideo: d.Price.ImagesPerVideo, IncludesCover: d.Price.IncludesCover,
			Mode: d.Price.Mode, PerMinutes: d.Price.PerMinutes, TotalUSD: d.Price.TotalUSD,
		},
	}
}

// ── handler template ───────────────────────────────────────────────────────

func listTemplates(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	items, err := svc.List(r.Context())
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	out := make([]TemplateSummaryWire, 0, len(items))
	for _, item := range items {
		out = append(out, TemplateSummaryToWire(item))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// templateCreateRequest là body POST /api/templates: tạo mới (name) hoặc nhân
// bản (fromId) — cùng route như menu thẻ Thư viện.
type templateCreateRequest struct {
	Name   string `json:"name"`
	FromID string `json:"fromId"`
}

func createTemplate(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	var req templateCreateRequest
	if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
		return
	}
	ctx := r.Context()
	if strings.TrimSpace(req.FromID) != "" {
		tpl, err := svc.Duplicate(ctx, req.FromID, req.Name)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		writeTemplateByID(w, ctx, svc, diag, tpl.ID, http.StatusCreated)
		return
	}
	tpl, err := svc.Create(ctx, req.Name)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	writeTemplateByID(w, ctx, svc, diag, tpl.ID, http.StatusCreated)
}

// writeTemplateByID đọc lại chi tiết sau khi ghi để client nhận đủ hình.
func writeTemplateByID(w http.ResponseWriter, ctx context.Context, svc LibraryService, diag LibraryDiag, id string, status int) {
	detail, err := svc.Get(ctx, id)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, status, TemplateDetailToWire(detail))
}

// templatePatchRequest là body PATCH /api/templates/:id. revision bắt buộc;
// recipe vắng = giữ nguyên; name vắng = giữ nguyên.
type templatePatchRequest struct {
	Revision int64           `json:"revision"`
	Name     *string         `json:"name"`
	Recipe   json.RawMessage `json:"recipe"`
}

func routeTemplates(w http.ResponseWriter, r *http.Request, svc LibraryService, mix TemplateBackdropMix, diag LibraryDiag) {
	if IsTemplateBackdropMix(r.URL.Path) {
		if r.Method != http.MethodGet || mix == nil {
			webapp.Error(w, http.StatusNotFound, "không tìm thấy")
			return
		}
		ServeTemplateBackdropMix(w, r, mix)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	id := strings.Trim(rest, "/")
	if id == "" || strings.Contains(id, "/") {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		detail, err := svc.Get(ctx, id)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, TemplateDetailToWire(detail))
	case http.MethodPatch:
		var req templatePatchRequest
		if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
			return
		}
		if req.Revision <= 0 {
			writeLibraryError(w, &library.Error{
				Code:     library.CodeInvalidInput,
				Message:  "Thiếu revision khi lưu template",
				Recovery: "Tải lại trang rồi lưu lại",
			}, diag)
			return
		}
		tpl, err := svc.Patch(ctx, id, library.Patch{Revision: req.Revision, Name: req.Name, Recipe: []byte(req.Recipe)})
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		writeTemplateByID(w, ctx, svc, diag, tpl.ID, http.StatusOK)
	case http.MethodDelete:
		if err := svc.DeleteTemplate(ctx, id); err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		webapp.Error(w, http.StatusMethodNotAllowed, "không hỗ trợ")
	}
}

// ── wire prompt ────────────────────────────────────────────────────────────

type promptTemplateRefWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ImagePromptWire struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Kind        string                  `json:"kind"`
	BuiltIn     bool                    `json:"builtIn"`
	Composition string                  `json:"composition,omitempty"`
	Prompt      string                  `json:"prompt"`
	UpdatedAt   time.Time               `json:"updatedAt"`
	UsedBy      []promptTemplateRefWire `json:"usedBy"`
	Samples     []string                `json:"samples"`
}

// ImagePromptToWire đổi prompt sang hình wire; usedBy suy từ templates.
func ImagePromptToWire(p domain.ImagePrompt, usage map[string][]library.TemplateRef) ImagePromptWire {
	refs := usage[p.ID]
	usedBy := make([]promptTemplateRefWire, 0, len(refs))
	for _, ref := range refs {
		usedBy = append(usedBy, promptTemplateRefWire{ID: ref.ID, Name: ref.Name})
	}
	return ImagePromptWire{
		ID: p.ID, Name: p.Name, Kind: p.Kind, BuiltIn: p.BuiltIn,
		Composition: p.Composition, Prompt: p.Prompt, UpdatedAt: p.UpdatedAt,
		UsedBy: usedBy, Samples: []string{},
	}
}

type promptCreateRequest struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Composition string  `json:"composition"`
	Prompt      *string `json:"prompt"`
	FromID      string  `json:"fromId"`
}

type promptPatchRequest struct {
	Name        *string `json:"name"`
	Kind        *string `json:"kind"`
	Composition *string `json:"composition"`
	Prompt      *string `json:"prompt"`
}

func listPrompts(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	query := r.URL.Query()
	items, err := svc.ListPrompts(r.Context(), query.Get("kind"), query.Get("q"))
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	usage, err := svc.PromptUsage(r.Context())
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	out := make([]ImagePromptWire, 0, len(items))
	for _, item := range items {
		out = append(out, ImagePromptToWire(item, usage))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func createPrompt(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	var req promptCreateRequest
	if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
		return
	}
	p, err := svc.CreatePrompt(r.Context(), library.PromptInput{
		Name: req.Name, Kind: req.Kind, Composition: req.Composition,
		Prompt: req.Prompt, FromID: req.FromID,
	})
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	writePromptByID(w, r.Context(), svc, diag, p.ID, http.StatusCreated)
}

// writePromptByID đọc lại prompt để kèm usedBy/samples.
func writePromptByID(w http.ResponseWriter, ctx context.Context, svc LibraryService, diag LibraryDiag, id string, status int) {
	usage, err := svc.PromptUsage(ctx)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	items, err := svc.ListPrompts(ctx, "", "")
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	for _, item := range items {
		if item.ID == id {
			webapp.WriteJSON(w, status, ImagePromptToWire(item, usage))
			return
		}
	}
	writeLibraryError(w, &library.Error{Code: library.CodeNotFound, Message: "Prompt ảnh không tồn tại", Recovery: "Tải lại trang rồi thử lại"}, diag)
}

func routePrompts(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/image-prompts/")
	parts := strings.SplitN(strings.Trim(rest, "/"), "/", 2)
	id := parts[0]
	tail := ""
	if len(parts) == 2 {
		tail = parts[1]
	}
	if id == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	ctx := r.Context()
	switch {
	case tail == "" && r.Method == http.MethodGet:
		items, err := svc.ListPrompts(ctx, "", "")
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		usage, err := svc.PromptUsage(ctx)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		for _, item := range items {
			if item.ID == id {
				webapp.WriteJSON(w, http.StatusOK, ImagePromptToWire(item, usage))
				return
			}
		}
		writeLibraryError(w, &library.Error{Code: library.CodeNotFound, Message: "Prompt ảnh không tồn tại", Recovery: "Tải lại trang rồi thử lại"}, diag)
	case tail == "" && r.Method == http.MethodPatch:
		var req promptPatchRequest
		if !webapp.DecodeJSONCode(w, r, &req, library.CodeInvalidInput) {
			return
		}
		if _, err := svc.PatchPrompt(ctx, id, library.PromptPatch{
			Name: req.Name, Kind: req.Kind, Composition: req.Composition, Prompt: req.Prompt,
		}); err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		writePromptByID(w, ctx, svc, diag, id, http.StatusOK)
	case tail == "" && r.Method == http.MethodDelete:
		if err := svc.DeletePrompt(ctx, id); err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case tail == "export" && r.Method == http.MethodGet:
		exportPrompt(w, r, svc, diag, id)
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

func exportPrompt(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag, id string) {
	export, err := svc.ExportPrompt(r.Context(), id)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	filename := safeFilename(export.Name) + ".json"
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	webapp.WriteJSON(w, http.StatusOK, export)
}

// importPrompt nhận tệp .json thô (application/json) hoặc multipart (trường
// "file") để UI tải tệp lên bằng form.
func importPrompt(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	const importLimit = 64 << 10
	data, err := readImportBody(w, r, importLimit)
	if err != nil {
		writeLibraryError(w, &library.Error{
			Code:     library.CodeInvalidInput,
			Message:  "Không đọc được tệp prompt",
			Recovery: "Chọn tệp .json xuất từ Thư viện › Prompt ảnh",
		}, diag)
		return
	}
	p, err := svc.ImportPrompt(r.Context(), data)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	writePromptByID(w, r.Context(), svc, diag, p.ID, http.StatusCreated)
}

// readImportBody đọc tệp nhập: multipart (file) hoặc body JSON thô.
func readImportBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(limit); err != nil {
			return nil, err
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, limit))
	}
	return io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
}

// safeFilename bỏ ký tự đường dẫn khỏi tên tệp tải về.
func safeFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r':
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if cleaned == "" {
		return "prompt"
	}
	return cleaned
}

// ── wire lớp phủ ───────────────────────────────────────────────────────────

type OverlayWire struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Mime        string                  `json:"mime"`
	Bytes       int64                   `json:"bytes"`
	DurationSec *float64                `json:"durationSec"`
	Width       *int                    `json:"width"`
	Height      *int                    `json:"height"`
	Background  string                  `json:"background"`
	BuiltIn     string                  `json:"builtIn"`
	CreatedAt   time.Time               `json:"createdAt"`
	UsedBy      []promptTemplateRefWire `json:"usedBy"`
	URL         string                  `json:"url"`
}

func OverlayToWire(o domain.Overlay, usage map[string][]library.TemplateRef) OverlayWire {
	refs := usage[o.ID]
	usedBy := make([]promptTemplateRefWire, 0, len(refs))
	for _, ref := range refs {
		usedBy = append(usedBy, promptTemplateRefWire{ID: ref.ID, Name: ref.Name})
	}
	return OverlayWire{
		ID: o.ID, Name: o.Name, Mime: o.Mime, Bytes: o.Bytes, DurationSec: o.DurationSec,
		Width: o.Width, Height: o.Height, Background: o.Background, BuiltIn: o.Builtin,
		CreatedAt: o.CreatedAt, UsedBy: usedBy, URL: "/api/overlays/" + o.ID + "/file",
	}
}

func listOverlays(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	items, err := svc.ListOverlays(r.Context())
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	usage, err := svc.OverlayUsage(r.Context())
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	out := make([]OverlayWire, 0, len(items))
	for _, item := range items {
		out = append(out, OverlayToWire(item, usage))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// addOverlay nhận multipart/form-data: trường "file" bắt buộc, "name" tuỳ chọn.
func addOverlay(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeLibraryError(w, &library.Error{
			Code:     library.CodeInvalidInput,
			Message:  "Gửi tệp lớp phủ dưới dạng multipart/form-data",
			Recovery: "Chọn tệp rồi tải lên lại",
		}, diag)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, library.MaxOverlayBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeLibraryError(w, &library.Error{
			Code:     library.CodeInvalidInput,
			Message:  "Tệp tải lên hỏng hoặc quá lớn",
			Recovery: "Chọn tệp nhỏ hơn 256 MB",
		}, diag)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeLibraryError(w, &library.Error{
			Code:     library.CodeInvalidInput,
			Message:  "Thiếu tệp lớp phủ",
			Recovery: "Chọn tệp video hoặc ảnh rồi tải lên lại",
		}, diag)
		return
	}
	defer file.Close()
	name := strings.TrimSpace(r.FormValue("name"))
	overlay, _, err := svc.AddOverlayReader(r.Context(), path.Base(header.Filename), name, file, header.Size)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	usage, err := svc.OverlayUsage(r.Context())
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusCreated, OverlayToWire(*overlay, usage))
}

func routeOverlays(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/overlays/")
	parts := strings.SplitN(strings.Trim(rest, "/"), "/", 2)
	id := parts[0]
	tail := ""
	if len(parts) == 2 {
		tail = parts[1]
	}
	if id == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	ctx := r.Context()
	switch {
	case tail == "" && r.Method == http.MethodDelete:
		if err := svc.RemoveOverlay(ctx, id); err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case tail == "file" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		serveOverlayFile(w, r, svc, diag, id)
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

// serveOverlayFile phục vụ tệp lớp phủ với Range (trình duyệt tua được video).
func serveOverlayFile(w http.ResponseWriter, r *http.Request, svc LibraryService, diag LibraryDiag, id string) {
	if source, ok := svc.(interface {
		GetOverlay(context.Context, string) (*domain.Overlay, error)
		Media() *media.Service
	}); ok && source.Media() != nil {
		overlay, err := source.GetOverlay(r.Context(), id)
		if err != nil {
			writeLibraryError(w, err, diag)
			return
		}
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName(overlay)}))
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		if err := source.Media().ServeHTTP(w, r, overlay.FilePath); err != nil {
			writeLibraryError(w, err, diag)
		}
		return
	}
	filePath, overlay, err := svc.OverlayPath(r.Context(), id)
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		writeLibraryError(w, &library.Error{
			Code:     library.CodeBadMedia,
			Message:  "Tệp lớp phủ không còn trên đĩa",
			Recovery: "Thêm lại tệp này vào thư viện",
		}, diag)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeLibraryError(w, err, diag)
		return
	}
	w.Header().Set("Content-Type", overlay.Mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName(overlay)}))
	}
	http.ServeContent(w, r, overlay.Name, info.ModTime(), file)
}

// downloadName ghép tên hiển thị với đuôi tệp thật để trình duyệt mở đúng.
func downloadName(overlay *domain.Overlay) string {
	ext := path.Ext(overlay.FilePath)
	if ext == "" || strings.HasSuffix(strings.ToLower(overlay.Name), ext) {
		return overlay.Name
	}
	return overlay.Name + ext
}

// ── lỗi ────────────────────────────────────────────────────────────────────

// writeLibraryError là bảng map duy nhất của mảng: mã nghiệp vụ → status, body
// giữ đủ dữ liệu phục hồi (fields/channels/usedBy/retryAt).
func writeLibraryError(w http.ResponseWriter, err error, diag LibraryDiag) {
	var apiErr *library.Error
	if errors.As(err, &apiErr) {
		status := http.StatusInternalServerError
		switch apiErr.Code {
		case library.CodeInvalidInput:
			status = http.StatusBadRequest
		case library.CodeNotFound:
			status = http.StatusNotFound
		case library.CodeConflict, library.CodeInUse:
			status = http.StatusConflict
		case library.CodeBuiltin:
			status = http.StatusForbidden
		case library.CodeBadMedia:
			status = http.StatusBadRequest
		case library.CodeToolMissing:
			status = http.StatusServiceUnavailable
		}
		if status == http.StatusInternalServerError {
			writeLibraryInternal(w, err, diag)
			return
		}
		body := map[string]any{"error": apiErr.Message, "code": apiErr.Code}
		if apiErr.Code == library.CodeInUse {
			// API công bố gọi trường này "reason"; giữ cả code (đồng nhất với
			// các lỗi khác) và reason (đúng chữ hợp đồng) để web đọc cái nào cũng được.
			body["reason"] = library.CodeInUse
		}
		if apiErr.Recovery != "" {
			body["recovery"] = apiErr.Recovery
		}
		if len(apiErr.Fields) > 0 {
			body["fields"] = apiErr.Fields
		}
		if len(apiErr.Channels) > 0 {
			body["channels"] = apiErr.Channels
		}
		if len(apiErr.UsedBy) > 0 {
			body["usedBy"] = apiErr.UsedBy
		}
		if apiErr.RetryAt != nil {
			body["retryAt"] = apiErr.RetryAt.UTC().Format(time.RFC3339)
		}
		webapp.WriteJSON(w, status, body)
		return
	}
	if errors.Is(err, library.ErrNotFound) {
		webapp.WriteJSON(w, http.StatusNotFound, map[string]any{
			"error": "không tìm thấy", "code": library.CodeNotFound, "recovery": "Tải lại trang rồi thử lại",
		})
		return
	}
	writeLibraryInternal(w, err, diag)
}

func writeLibraryInternal(w http.ResponseWriter, err error, diag LibraryDiag) {
	if diag != nil {
		diag(err.Error())
	}
	webapp.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "lỗi nội bộ", "code": "internal"})
}
