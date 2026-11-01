package web

// HTTP của mảng Kênh của bạn: danh sách/tạo/xem/sửa/xoá kênh, liên kết đối
// thủ, danh sách template luân phiên, tạo nhận diện và phục vụ tệp avatar/ảnh
// bìa. Handler chỉ đọc request → gọi service → ghi JSON; hình wire tách hẳn
// khỏi kiểu domain.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// ChannelService là mặt nghiệp vụ mảng kênh — hiện thực là *channels.Service.
type ChannelService interface {
	List(ctx context.Context) ([]channels.ChannelView, error)
	Get(ctx context.Context, id string) (channels.ChannelView, error)
	Create(ctx context.Context, in channels.CreateRequest) (channels.ChannelView, string, error)
	Update(ctx context.Context, id string, in channels.UpdateRequest) (channels.ChannelView, error)
	Delete(ctx context.Context, id string) error
	Link(ctx context.Context, id, rivalID string) (channels.ChannelView, error)
	Unlink(ctx context.Context, id, rivalID string) (channels.ChannelView, bool, error)
	SetSourceMinDuration(ctx context.Context, id, rivalID string, minDurationSec *int) (channels.ChannelView, error)
	Templates(ctx context.Context, id string, req channels.TemplateRequest) ([]domain.ChannelTemplate, error)
	GenerateIdentity(ctx context.Context, id string, in channels.IdentityInput) (string, error)
	AssetFile(ctx context.Context, id, file string) (io.ReadSeekCloser, string, int64, error)
}

// ChannelDiag là điểm tới của lỗi nội bộ 500 như các mảng khác.
type ChannelDiag func(msg string)

// MountChannels đăng ký các route /api/owner-channels* trên gateway.
func MountChannels(g *Gateway, svc ChannelService) {
	MountChannelsDiag(g, svc, nil)
}

// MountChannelsDiag giống MountChannels nhưng có điểm tới cho lỗi nội bộ.
func MountChannelsDiag(g *Gateway, svc ChannelService, diag ChannelDiag) {
	g.Handle(http.MethodGet, "/api/owner-channels", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/owner-channels", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		createChannel(w, r, svc, diag)
	})
	g.Handle(http.MethodGet, "/api/owner-channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodHead, "/api/owner-channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodPatch, "/api/owner-channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodDelete, "/api/owner-channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/owner-channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		routeChannels(w, r, svc, diag)
	})
}

// ── wire ───────────────────────────────────────────────────────────────────

type channelSourceWire struct {
	RivalID   string  `json:"rivalId"`
	Title     string  `json:"title"`
	Handle    *string `json:"handle"`
	LinkedAt  string  `json:"linkedAt"`
	Reference bool    `json:"reference"`
	// MinDurationSec: bộ lọc Tự lấy theo đối thủ (chỉ lấy video dài từ ngần này
	// giây); null = không lọc.
	MinDurationSec *int `json:"minDurationSec"`
}

type channelTemplateWire struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type channelIdentityErrorWire struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Step: "text" (Đạo diễn) | "images" (Hoạ sĩ); rỗng = không rõ bước hỏng.
	Step string `json:"step,omitempty"`
}

type channelIdentityWire struct {
	State     string                    `json:"state"` // ready | generating | failed
	Error     *channelIdentityErrorWire `json:"error,omitempty"`
	RetryAt   *string                   `json:"retryAt,omitempty"`
	StartedAt *string                   `json:"startedAt,omitempty"`
	Part      *string                   `json:"part,omitempty"`
	JobID     *string                   `json:"jobId,omitempty"`
}

type channelWire struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	YoutubeURL        *string               `json:"youtubeUrl"`
	Language          string                `json:"language"`
	StyleNote         *string               `json:"styleNote"`
	DescriptionFooter *string               `json:"descriptionFooter"`
	AlwaysTags        []string              `json:"alwaysTags"`
	AutoPick          bool                  `json:"autoPick"`
	PacePerDay        int                   `json:"pacePerDay"`
	Autonomy          domain.Autonomy       `json:"autonomy"`
	Priority          string                `json:"priority"`
	TemplateCursor    int                   `json:"templateCursor"`
	Branding          brandingWire          `json:"branding"`
	AvatarURL         *string               `json:"avatarUrl"`
	BannerURL         *string               `json:"bannerUrl"`
	Sources           []channelSourceWire   `json:"sources"`
	Templates         []channelTemplateWire `json:"templates"`
	Identity          channelIdentityWire   `json:"identity"`
	CreatedAt         string                `json:"createdAt"`
	UpdatedAt         string                `json:"updatedAt"`
}

type brandingWire struct {
	Brief   string `json:"brief"`
	Summary string `json:"summary"`
	Look    string `json:"look"`
	Avatar  string `json:"avatar"`
	Banner  string `json:"banner"`
}

type channelListWire struct {
	Channels []channelWire `json:"channels"`
}

// ChannelToWire chuyển một kênh sang hình wire — dùng chung cho HTTP và CLI.
func ChannelToWire(v channels.ChannelView) channelWire {
	c := v.Channel
	out := channelWire{
		ID:                c.ID,
		Name:              c.Name,
		YoutubeURL:        c.YoutubeURL,
		Language:          c.Language,
		StyleNote:         c.StyleNote,
		DescriptionFooter: c.DescriptionFooter,
		AlwaysTags:        c.AlwaysTags,
		AutoPick:          c.AutoPick && c.Autonomy.AutoPicks(),
		PacePerDay:        c.PacePerDay,
		Autonomy:          c.Autonomy,
		Priority:          c.Priority,
		TemplateCursor:    c.TemplateCursor,
		Branding: brandingWire{
			Brief:   c.Branding.Brief,
			Summary: c.Branding.Summary,
			Look:    c.Branding.Look,
			Avatar:  c.Branding.Avatar,
			Banner:  c.Branding.Banner,
		},
		Sources:   make([]channelSourceWire, 0, len(v.Sources)),
		Templates: make([]channelTemplateWire, 0, len(v.Templates)),
		Identity:  identityToWire(c),
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if out.AlwaysTags == nil {
		out.AlwaysTags = []string{}
	}
	if c.AvatarPath != nil {
		url := brandingURL(c.ID, *c.AvatarPath)
		out.AvatarURL = &url
	}
	if c.BannerPath != nil {
		url := brandingURL(c.ID, *c.BannerPath)
		out.BannerURL = &url
	}
	for _, s := range v.Sources {
		out.Sources = append(out.Sources, channelSourceWire{
			RivalID:        s.RivalID,
			Title:          s.Title,
			Handle:         s.Handle,
			LinkedAt:       s.LinkedAt.UTC().Format(time.RFC3339),
			Reference:      s.Reference,
			MinDurationSec: s.MinDurationSec,
		})
	}
	for _, t := range v.Templates {
		out.Templates = append(out.Templates, channelTemplateWire{ID: t.ID, Name: t.Name, Position: t.Position})
	}
	return out
}

// brandingURL dựng đường tải tài sản từ đường tương đối trong DB.
func brandingURL(id, rel string) string {
	return "/api/owner-channels/" + id + "/branding/" + filepath.Base(rel)
}

// identityToWire ánh xạ trạng thái nội bộ sang ba trạng thái UI hiểu:
// idle → ready (không có run sống), running → generating, failed → failed.
func identityToWire(c domain.Channel) channelIdentityWire {
	out := channelIdentityWire{JobID: c.IdentityTaskID, Part: c.IdentityPart}
	switch c.IdentityState {
	case domain.IdentityRunning:
		out.State = "generating"
	case domain.IdentityFailed:
		out.State = "failed"
	default:
		out.State = "ready"
	}
	if c.IdentityState == domain.IdentityFailed {
		code := ""
		message := ""
		if c.IdentityErrorCode != nil {
			code = *c.IdentityErrorCode
		}
		if c.IdentityError != nil {
			message = *c.IdentityError
		}
		out.Error = &channelIdentityErrorWire{Code: code, Message: message}
		if c.IdentityFailedStep != nil {
			out.Error.Step = *c.IdentityFailedStep
		}
	}
	if c.IdentityRetryAt != nil {
		s := c.IdentityRetryAt.UTC().Format(time.RFC3339)
		out.RetryAt = &s
	}
	if c.IdentityStartedAt != nil {
		s := c.IdentityStartedAt.UTC().Format(time.RFC3339)
		out.StartedAt = &s
	}
	return out
}

// ── handlers ───────────────────────────────────────────────────────────────

func listChannels(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag) {
	list, err := svc.List(r.Context())
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	out := channelListWire{Channels: make([]channelWire, 0, len(list))}
	for _, v := range list {
		out.Channels = append(out.Channels, ChannelToWire(v))
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type createChannelBody struct {
	Name          string   `json:"name"`
	YoutubeURL    *string  `json:"youtubeUrl"`
	CompetitorIDs []string `json:"competitorIds"`
	AutoPick      *bool    `json:"autoPick"`
	PacePerDay    *int     `json:"pacePerDay"`
	Language      string   `json:"language"`
	// Autonomy/Priority: Mức tự chủ (07 §20); Confirmed là xác nhận rõ cho
	// "Tự duyệt hết" (D6).
	Autonomy  *domain.Autonomy `json:"autonomy"`
	Priority  *string          `json:"priority"`
	Confirmed bool             `json:"confirmed"`
}

type createChannelResponse struct {
	Channel       channelWire `json:"channel"`
	IdentityJobID *string     `json:"identityJobId"`
}

func createChannel(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag) {
	var body createChannelBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	// UI bắt buộc ít nhất một đối thủ (đối thủ là nguồn video mẫu và là tham
	// chiếu nhận diện); CLI giữ tuỳ chọn. Danh sách toàn chuỗi trắng vẫn là 400.
	competitors := 0
	for _, id := range body.CompetitorIDs {
		if strings.TrimSpace(id) != "" {
			competitors++
		}
	}
	if competitors == 0 {
		writeChannelCode(w, http.StatusBadRequest,
			"Chọn ít nhất 1 đối thủ làm nguồn.", channels.CodeInvalidInput,
			"Đối thủ là nguồn video mẫu cho kênh.", nil)
		return
	}
	view, jobID, err := svc.Create(r.Context(), channels.CreateRequest{
		Name:          body.Name,
		YoutubeURL:    body.YoutubeURL,
		CompetitorIDs: body.CompetitorIDs,
		AutoPick:      body.AutoPick,
		PacePerDay:    body.PacePerDay,
		Language:      body.Language,
		Autonomy:      body.Autonomy,
		Priority:      body.Priority,
		Confirmed:     body.Confirmed,
	})
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	out := createChannelResponse{Channel: ChannelToWire(view)}
	if jobID != "" {
		out.IdentityJobID = &jobID
	}
	webapp.WriteJSON(w, http.StatusCreated, out)
}

// routeChannels khớp phần sau /api/owner-channels/ theo hình
// {id} | {id}/sources[/{rivalId}] | {id}/templates | {id}/branding[/{file}].
func routeChannels(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/owner-channels/")
	parts := strings.SplitN(rest, "/", 3)
	if parts[0] == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	id := parts[0]
	tail := ""
	extra := ""
	if len(parts) > 1 {
		tail = parts[1]
	}
	if len(parts) > 2 {
		extra = parts[2]
	}
	ctx := r.Context()
	switch {
	case tail == "" && r.Method == http.MethodGet:
		view, err := svc.Get(ctx, id)
		if err != nil {
			writeChannelError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, ChannelToWire(view))
	case tail == "" && r.Method == http.MethodPatch:
		patchChannel(w, r, svc, diag, id)
	case tail == "" && r.Method == http.MethodDelete:
		if err := svc.Delete(ctx, id); err != nil {
			writeChannelError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case tail == "sources" && extra == "" && r.Method == http.MethodPost:
		linkSource(w, r, svc, diag, id)
	case tail == "sources" && extra != "" && r.Method == http.MethodDelete:
		unlinkSource(w, r, svc, diag, id, extra)
	case tail == "sources" && extra != "" && r.Method == http.MethodPatch:
		patchSource(w, r, svc, diag, id, extra)
	case tail == "templates" && extra == "" && r.Method == http.MethodPost:
		templates(w, r, svc, diag, id)
	case tail == "branding" && extra == "" && r.Method == http.MethodPost:
		generateIdentity(w, r, svc, diag, id)
	case tail == "branding" && extra != "" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		serveBranding(w, r, svc, diag, id, extra)
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

type updateChannelBody struct {
	Name              *string          `json:"name"`
	YoutubeURL        json.RawMessage  `json:"youtubeUrl"`
	Language          *string          `json:"language"`
	StyleNote         json.RawMessage  `json:"styleNote"`
	DescriptionFooter json.RawMessage  `json:"descriptionFooter"`
	AlwaysTags        *[]string        `json:"alwaysTags"`
	AutoPick          *bool            `json:"autoPick"`
	PacePerDay        *int             `json:"pacePerDay"`
	Brief             *string          `json:"brief"`
	Autonomy          *domain.Autonomy `json:"autonomy"`
	Priority          *string          `json:"priority"`
	Confirmed         bool             `json:"confirmed"`
}

// nullableField đọc một trường vắng / null / chuỗi: vắng = không đổi, null =
// xoá giá trị, chuỗi = đặt giá trị.
func nullableField(raw json.RawMessage) (**string, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if string(raw) == "null" {
		var nilValue *string
		return &nilValue, true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false, err
	}
	value := &s
	return &value, true, nil
}

func patchChannel(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id string) {
	var body updateChannelBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	var in channels.UpdateRequest
	in.Name = body.Name
	in.Language = body.Language
	in.AlwaysTags = body.AlwaysTags
	in.AutoPick = body.AutoPick
	in.PacePerDay = body.PacePerDay
	in.Brief = body.Brief
	in.Autonomy = body.Autonomy
	in.Priority = body.Priority
	in.Confirmed = body.Confirmed
	var err error
	if in.YoutubeURL, _, err = nullableField(body.YoutubeURL); err != nil {
		writeChannelCode(w, http.StatusBadRequest, "youtubeUrl không hợp lệ.", channels.CodeInvalidInput, "", nil)
		return
	}
	if in.StyleNote, _, err = nullableField(body.StyleNote); err != nil {
		writeChannelCode(w, http.StatusBadRequest, "styleNote không hợp lệ.", channels.CodeInvalidInput, "", nil)
		return
	}
	if in.DescriptionFooter, _, err = nullableField(body.DescriptionFooter); err != nil {
		writeChannelCode(w, http.StatusBadRequest, "descriptionFooter không hợp lệ.", channels.CodeInvalidInput, "", nil)
		return
	}
	view, err := svc.Update(r.Context(), id, in)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ChannelToWire(view))
}

type linkSourceBody struct {
	RivalID string `json:"rivalId"`
}

func linkSource(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id string) {
	var body linkSourceBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	view, err := svc.Link(r.Context(), id, body.RivalID)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ChannelToWire(view))
}

type unlinkSourceResponse struct {
	AutoPickPaused bool `json:"autoPickPaused"`
}

type patchSourceBody struct {
	// MinDurationSec vắng/null = bỏ lọc; số giây = chỉ lấy video dài từ ngần đó.
	MinDurationSec *int `json:"minDurationSec"`
}

func patchSource(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id, rivalID string) {
	var body patchSourceBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	view, err := svc.SetSourceMinDuration(r.Context(), id, rivalID, body.MinDurationSec)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ChannelToWire(view))
}

func unlinkSource(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id, rivalID string) {
	_, paused, err := svc.Unlink(r.Context(), id, rivalID)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, unlinkSourceResponse{AutoPickPaused: paused})
}

type templatesBody struct {
	Action     string `json:"action"`
	TemplateID string `json:"templateId"`
	Position   *int   `json:"position"`
}

type templatesResponse struct {
	Templates []channelTemplateWire `json:"templates"`
}

func templates(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id string) {
	var body templatesBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	list, err := svc.Templates(r.Context(), id, channels.TemplateRequest{
		Action: body.Action, TemplateID: body.TemplateID, Position: body.Position,
	})
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	out := templatesResponse{Templates: make([]channelTemplateWire, 0, len(list))}
	for _, t := range list {
		out.Templates = append(out.Templates, channelTemplateWire{ID: t.ID, Name: t.Name, Position: t.Position})
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type brandingBody struct {
	Part      string `json:"part"`
	Brief     string `json:"brief"`
	Direction string `json:"direction"`
}

type brandingResponse struct {
	Channel       channelWire `json:"channel"`
	IdentityJobID *string     `json:"identityJobId"`
}

func generateIdentity(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id string) {
	var body brandingBody
	if !webapp.DecodeJSONCode(w, r, &body, channels.CodeInvalidInput) {
		return
	}
	part := body.Part
	if part == "" {
		part = domain.IdentityAll
	}
	jobID, err := svc.GenerateIdentity(r.Context(), id, channels.IdentityInput{
		Part: part, Brief: body.Brief, Direction: body.Direction,
	})
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	view, err := svc.Get(r.Context(), id)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	out := brandingResponse{Channel: ChannelToWire(view)}
	if jobID != "" {
		out.IdentityJobID = &jobID
	}
	webapp.WriteJSON(w, http.StatusAccepted, out)
}

// serveBranding phục vụ avatar/ảnh bìa của chính kênh. Tên tệp phải khớp mẫu
// uuid-avatar.png / uuid-banner.jpg và đang được kênh dùng; ?download=1 đặt
// Content-Disposition để tải về.
func serveBranding(w http.ResponseWriter, r *http.Request, svc ChannelService, diag ChannelDiag, id, file string) {
	if source, ok := svc.(interface {
		AssetPath(context.Context, string, string) (string, error)
		Media() *media.Service
	}); ok && source.Media() != nil {
		path, err := source.AssetPath(r.Context(), id, file)
		if err != nil {
			writeChannelError(w, err, diag)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		if err := source.Media().ServeHTTP(w, r, path); err != nil {
			writeChannelError(w, err, diag)
		}
		return
	}
	f, name, _, err := svc.AssetFile(r.Context(), id, file)
	if err != nil {
		writeChannelError(w, err, diag)
		return
	}
	defer f.Close()
	if strings.HasSuffix(name, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	// Tên tệp chứa uuid nên nội dung không đổi: cache dài, riêng tư.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	http.ServeContent(w, r, name, time.Time{}, f)
}

// ── lỗi ────────────────────────────────────────────────────────────────────

// writeChannelError map lỗi nghiệp vụ sang status + vỏ {error, code,
// recovery?, retryAt?} ở MỘT chỗ cho mọi route của mảng kênh.
func writeChannelError(w http.ResponseWriter, err error, diag ChannelDiag) {
	var ce *channels.Error
	if errors.As(err, &ce) {
		status := http.StatusInternalServerError
		switch ce.Code {
		case channels.CodeInvalidInput:
			status = http.StatusBadRequest
		case channels.CodeNotFound:
			status = http.StatusNotFound
		case channels.CodeConflict, channels.CodeCancelled, channels.CodeInterrupted, channels.CodeConfirmRequired:
			status = http.StatusConflict
		case channels.CodeQuota:
			status = http.StatusTooManyRequests
		case channels.CodeToolMissing:
			status = http.StatusServiceUnavailable
		}
		if status == http.StatusInternalServerError {
			writeChannelInternal(w, err, diag)
			return
		}
		if ce.Field != "" {
			webapp.WriteJSON(w, status, map[string]any{"error": ce.Message, "code": ce.Code,
				"fields": []map[string]string{{"field": ce.Field, "message": ce.Message}}})
			return
		}
		writeChannelCode(w, status, ce.Message, ce.Code, ce.Recovery, ce.RetryAt)
		return
	}
	writeChannelInternal(w, err, diag)
}

// writeChannelCode ghi vỏ lỗi {error, code, recovery?, retryAt?}.
func writeChannelCode(w http.ResponseWriter, status int, msg, code, recovery string, retryAt *time.Time) {
	body := map[string]any{"error": msg, "code": code}
	if recovery != "" {
		body["recovery"] = recovery
	}
	if retryAt != nil {
		body["retryAt"] = retryAt.UTC().Format(time.RFC3339)
	}
	webapp.WriteJSON(w, status, body)
}

// writeChannelInternal trả vỏ 500 trung lập — nguyên nhân chỉ đi tới diag.
func writeChannelInternal(w http.ResponseWriter, err error, diag ChannelDiag) {
	if diag != nil {
		diag(err.Error())
	}
	writeChannelCode(w, http.StatusInternalServerError, "lỗi nội bộ", "internal", "", nil)
}
