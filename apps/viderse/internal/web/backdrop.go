package web

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// BackdropService là mặt nghiệp vụ của Video nền mà HTTP cần.
type BackdropService interface {
	ParseSource(ctx context.Context, input string) (domain.ParsedSource, error)
	Channels(ctx context.Context) ([]backdrop.ChannelView, error)
	Channel(ctx context.Context, id string) (backdrop.ChannelView, error)
	AddChannel(ctx context.Context, input string) (backdrop.ChannelView, bool, error)
	RefreshChannel(ctx context.Context, id string, force bool) (backdrop.ChannelView, error)
	RemoveChannel(ctx context.Context, id string) error
	ChannelVideos(ctx context.Context, id string, opts backdrop.ChannelVideosOptions) (backdrop.VideoPage, error)
	Videos(ctx context.Context) ([]backdrop.VideoView, error)
	Video(ctx context.Context, id string) (backdrop.VideoView, error)
	AddVideo(ctx context.Context, url string, segment *domain.BackdropSegment) (backdrop.VideoView, error)
	UpdateVideoSegment(ctx context.Context, id string, segment domain.BackdropSegment) (backdrop.VideoView, error)
	RemoveVideo(ctx context.Context, id string) error
	VideoFile(ctx context.Context, id string) (string, error)
	RecordPlatformRequest(ctx context.Context, host string) (backdrop.PlatformRequestRecord, error)
	PlatformRequests(ctx context.Context) ([]backdrop.PlatformRequestRecord, error)
}

// BackdropDiag nhận nguyên nhân lỗi 500 đã qua logs.Redact.
type BackdropDiag func(msg string)

// MountBackdrop đăng ký các route Video nền không có điểm tới lỗi 500.
func MountBackdrop(g *Gateway, svc BackdropService) {
	MountBackdropDiag(g, svc, nil)
}

// MountBackdropDiag đăng ký các route Video nền; diag nhận nguyên nhân lỗi
// 500 đã qua logs.Redact. Phải gọi TRƯỚC MountPlatforms: route chính xác
// POST /api/platforms/requests thắng tiền tố /api/platforms/.
func MountBackdropDiag(g *Gateway, svc BackdropService, diag BackdropDiag) {
	g.Handle(http.MethodPost, "/api/sources/parse", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		parseSource(w, r, svc, diag)
	})
	g.Handle(http.MethodGet, "/api/backdrop/channels", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listBackdropChannels(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/backdrop/channels", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		addBackdropChannel(w, r, svc, diag)
	})
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodPost} {
		m := method
		g.Handle(m, "/api/backdrop/channels/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routeBackdropChannel(w, r, svc, diag)
		})
	}
	g.Handle(http.MethodGet, "/api/backdrop/videos", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listBackdropVideos(w, r, svc, diag)
	})
	g.Handle(http.MethodPost, "/api/backdrop/videos", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		addBackdropVideo(w, r, svc, diag)
	})
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		m := method
		g.Handle(m, "/api/backdrop/videos/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routeBackdropVideo(w, r, svc, diag)
		})
	}
	g.Handle(http.MethodPost, "/api/platforms/requests", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		recordPlatformRequest(w, r, svc, diag)
	})
}

// ── handlers ───────────────────────────────────────────────────────────────

type sourceParseRequest struct {
	Input string `json:"input"`
}

func parseSource(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	var req sourceParseRequest
	if !webapp.DecodeJSONCode(w, r, &req, backdrop.CodeInvalidInput) {
		return
	}
	source, err := svc.ParseSource(r.Context(), req.Input)
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ParsedSourceToWire(source))
}

func listBackdropChannels(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	views, err := svc.Channels(r.Context())
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	out := make([]backdropChannelWire, 0, len(views))
	for _, view := range views {
		out = append(out, BackdropChannelToWire(view))
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type addChannelRequest struct {
	Input string `json:"input"`
	URL   string `json:"url"`
}

func addBackdropChannel(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	var req addChannelRequest
	if !webapp.DecodeJSONCode(w, r, &req, backdrop.CodeInvalidInput) {
		return
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		input = strings.TrimSpace(req.URL)
	}
	view, created, err := svc.AddChannel(r.Context(), input)
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	webapp.WriteJSON(w, status, BackdropChannelToWire(view))
}

// routeBackdropChannel chia /api/backdrop/channels/{id}[/videos|/refresh].
func routeBackdropChannel(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/backdrop/channels/"), "/")
	if rest == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		view, err := svc.Channel(r.Context(), id)
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, BackdropChannelToWire(view))
	case len(parts) == 1 && r.Method == http.MethodDelete:
		if err := svc.RemoveChannel(r.Context(), id); err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case len(parts) == 2 && parts[1] == "videos" && r.Method == http.MethodGet:
		page, err := svc.ChannelVideos(r.Context(), id, ChannelVideosOptionsFromQuery(r))
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, BackdropVideoPageToWire(page))
	case len(parts) == 2 && parts[1] == "refresh" && r.Method == http.MethodPost:
		view, err := svc.RefreshChannel(r.Context(), id, true)
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, BackdropChannelToWire(view))
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

// ChannelVideosOptionsFromQuery đọc bộ lọc trang danh mục từ query string.
func ChannelVideosOptionsFromQuery(r *http.Request) backdrop.ChannelVideosOptions {
	query := r.URL.Query()
	options := backdrop.ChannelVideosOptions{
		Sort:   query.Get("sort"),
		Cursor: query.Get("cursor"),
	}
	for _, raw := range query["kind"] {
		for _, kind := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(kind); trimmed != "" {
				options.Kinds = append(options.Kinds, trimmed)
			}
		}
	}
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil {
		options.Limit = limit
	}
	return options
}

func listBackdropVideos(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	views, err := svc.Videos(r.Context())
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	out := make([]backdropVideoWire, 0, len(views))
	for _, view := range views {
		out = append(out, BackdropVideoToWire(view))
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type addVideoRequest struct {
	URL     string          `json:"url"`
	Segment *segmentRequest `json:"segment"`
}

type segmentRequest struct {
	FromSec *float64 `json:"fromSec"`
	ToSec   *float64 `json:"toSec"`
}

// domainStrict đòi đủ hai đầu: object segment thiếu trường là lỗi, không phải
// "dùng mặc định" (khác với không truyền segment).
func (s *segmentRequest) domainStrict() (*domain.BackdropSegment, error) {
	if s.FromSec == nil || s.ToSec == nil {
		return nil, backdrop.ErrorfRecovery(backdrop.CodeInvalidInput,
			"Đoạn dùng phải có đủ điểm đầu và điểm cuối", "Nhập cả giây bắt đầu và kết thúc")
	}
	return &domain.BackdropSegment{FromSec: *s.FromSec, ToSec: *s.ToSec}, nil
}

func addBackdropVideo(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	var req addVideoRequest
	if !webapp.DecodeJSONCode(w, r, &req, backdrop.CodeInvalidInput) {
		return
	}
	var segment *domain.BackdropSegment
	if req.Segment != nil {
		parsed, err := req.Segment.domainStrict()
		if err != nil {
			webapp.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": err.Error(), "code": backdrop.CodeInvalidInput,
			})
			return
		}
		segment = parsed
	}
	view, err := svc.AddVideo(r.Context(), req.URL, segment)
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusCreated, BackdropVideoToWire(view))
}

// routeBackdropVideo chia /api/backdrop/videos/{id}[/file].
func routeBackdropVideo(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/backdrop/videos/"), "/")
	if rest == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		view, err := svc.Video(r.Context(), id)
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, BackdropVideoToWire(view))
	case len(parts) == 1 && r.Method == http.MethodDelete:
		if err := svc.RemoveVideo(r.Context(), id); err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		var req segmentPatchRequest
		if !webapp.DecodeJSONCode(w, r, &req, backdrop.CodeInvalidInput) {
			return
		}
		if req.Segment == nil || req.Segment.FromSec == nil || req.Segment.ToSec == nil {
			webapp.Error(w, http.StatusBadRequest, "Đoạn dùng không hợp lệ")
			return
		}
		view, err := svc.UpdateVideoSegment(r.Context(), id, domain.BackdropSegment{FromSec: *req.Segment.FromSec, ToSec: *req.Segment.ToSec})
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, BackdropVideoToWire(view))
	case len(parts) == 2 && parts[1] == "file" && r.Method == http.MethodGet:
		path, err := svc.VideoFile(r.Context(), id)
		if err != nil {
			WriteBackdropError(w, err, diag)
			return
		}
		serveBackdropFile(w, r, path)
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

type segmentPatchRequest struct {
	Segment *segmentRequest `json:"segment"`
}

func serveBackdropFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := os.Open(path)
	if err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, "không đọc được tệp video")
		return
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	}
	http.ServeContent(w, r, filepath.Base(path), stat.ModTime(), file)
}

type platformRequestRequest struct {
	Host string `json:"host"`
}

func recordPlatformRequest(w http.ResponseWriter, r *http.Request, svc BackdropService, diag BackdropDiag) {
	var req platformRequestRequest
	if !webapp.DecodeJSONCode(w, r, &req, backdrop.CodeInvalidInput) {
		return
	}
	record, err := svc.RecordPlatformRequest(r.Context(), req.Host)
	if err != nil {
		WriteBackdropError(w, err, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, platformRequestWire{
		Host: record.Host, Label: record.Label, Count: record.Count,
		FirstAt: record.FirstAt.UTC().Format(time.RFC3339),
		LastAt:  record.LastAt.UTC().Format(time.RFC3339),
	})
}

// ── wire ───────────────────────────────────────────────────────────────────

type parsedSourceWire struct {
	Kind        string   `json:"kind"`
	Platform    string   `json:"platform,omitempty"`
	ID          string   `json:"id,omitempty"`
	Handle      string   `json:"handle,omitempty"`
	URL         string   `json:"url,omitempty"`
	Title       *string  `json:"title,omitempty"`
	DurationSec *float64 `json:"durationSec,omitempty"`
	Host        string   `json:"host,omitempty"`
	Label       string   `json:"label,omitempty"`
	Message     string   `json:"message,omitempty"`
	Recovery    string   `json:"recovery,omitempty"`
}

// ParsedSourceToWire đổi kết quả phân loại link sang hình §24 ParsedSource.
func ParsedSourceToWire(source domain.ParsedSource) parsedSourceWire {
	return parsedSourceWire{
		Kind: source.Kind, Platform: string(source.Platform), ID: source.ID, Handle: source.Handle,
		URL: source.URL, Title: source.Title, DurationSec: source.DurationSec,
		Host: source.Host, Label: source.Label, Message: source.Message, Recovery: source.Recovery,
	}
}

type backdropChannelWire struct {
	ID            string                      `json:"id"`
	Platform      string                      `json:"platform"`
	Handle        string                      `json:"handle"`
	URL           string                      `json:"url"`
	Title         *string                     `json:"title"`
	AvatarURL     *string                     `json:"avatarUrl"`
	AddedAt       string                      `json:"addedAt"`
	ListedAt      *string                     `json:"listedAt"`
	ListError     *backdropChannelErrorWire   `json:"listError"`
	ScanNextPage  *int                        `json:"scanNextPage"`
	ScanTotal     int                         `json:"scanTotal"`
	ScanStopKnown bool                        `json:"scanStopKnown"`
	Counts        map[string]int              `json:"counts"`
	PausedUntil   *string                     `json:"pausedUntil"`
	PauseReason   *string                     `json:"pauseReason"`
	UsedBy        []backdropTemplateUsageWire `json:"usedBy"`
	Bilibili      *backdropBilibiliStatusWire `json:"bilibili,omitempty"`
}

type backdropChannelErrorWire struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type backdropTemplateUsageWire struct {
	TemplateID  string   `json:"templateId"`
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Kinds       []string `json:"kinds"`
	ChosenCount int      `json:"chosenCount"`
}

type backdropBilibiliStatusWire struct {
	Budget      backdropBudgetWire    `json:"budget"`
	PausedUntil *string               `json:"pausedUntil"`
	PauseReason *string               `json:"pauseReason"`
	NextPageAt  *string               `json:"nextPageAt"`
	Pending     []backdropPendingWire `json:"pending"`
}

type backdropBudgetWire struct {
	Short backdropBudgetWindowWire `json:"short"`
	Day   backdropBudgetWindowWire `json:"day"`
}

type backdropBudgetWindowWire struct {
	Used        int    `json:"used"`
	Limit       int    `json:"limit"`
	AvailableAt string `json:"availableAt"`
}

type backdropPendingWire struct {
	ChannelID string `json:"channelId"`
	NextPage  *int   `json:"nextPage"`
	MaxPages  int    `json:"maxPages"`
}

// BackdropChannelToWire đổi view-model kênh nền sang JSON.
func BackdropChannelToWire(view backdrop.ChannelView) backdropChannelWire {
	row := view.Channel
	wire := backdropChannelWire{
		ID: row.ID, Platform: string(row.Platform), Handle: row.Handle, URL: row.URL,
		Title: row.Title, AvatarURL: row.AvatarURL,
		AddedAt:      row.AddedAt.UTC().Format(time.RFC3339),
		ScanNextPage: row.ScanNextPage, ScanTotal: row.ScanTotal, ScanStopKnown: row.ScanStopKnown,
		Counts:      map[string]int{},
		PausedUntil: timeWire(view.PausedUntil),
		PauseReason: view.PauseReason,
		UsedBy:      make([]backdropTemplateUsageWire, 0, len(view.UsedBy)),
	}
	if row.ListedAt != nil {
		wire.ListedAt = timeWire(row.ListedAt)
	}
	if row.ListErrorCode != "" || row.ListError != "" {
		wire.ListError = &backdropChannelErrorWire{Code: row.ListErrorCode, Message: row.ListError}
	}
	for kind, count := range view.Counts {
		wire.Counts[kind] = count
	}
	for _, usage := range view.UsedBy {
		wire.UsedBy = append(wire.UsedBy, backdropTemplateUsageWire{
			TemplateID: usage.TemplateID, Name: usage.Name, Mode: usage.Mode,
			Kinds: usage.Kinds, ChosenCount: usage.ChosenCount,
		})
	}
	if view.Bilibili != nil {
		wire.Bilibili = bilibiliStatusToWire(*view.Bilibili)
	}
	return wire
}

func bilibiliStatusToWire(status backdrop.BilibiliStatus) *backdropBilibiliStatusWire {
	out := &backdropBilibiliStatusWire{
		Budget:      budgetToWire(status.Budget),
		PausedUntil: timeWire(status.PausedUntil),
		PauseReason: status.PauseReason,
		NextPageAt:  timeWire(status.NextPageAt),
		Pending:     make([]backdropPendingWire, 0, len(status.Pending)),
	}
	for _, pending := range status.Pending {
		out.Pending = append(out.Pending, backdropPendingWire{ChannelID: pending.ChannelID, NextPage: pending.NextPage, MaxPages: pending.MaxPages})
	}
	return out
}

// budgetToWire đổi hạn mức cuộn: short là cửa sổ 10 phút, day là cửa sổ ngày.
func budgetToWire(report platforms.BudgetReport) backdropBudgetWire {
	out := backdropBudgetWire{}
	for _, window := range report.Windows {
		wire := backdropBudgetWindowWire{Used: window.Used, Limit: window.Limit, AvailableAt: window.AvailableAt.UTC().Format(time.RFC3339)}
		if window.Window < 24*time.Hour {
			out.Short = wire
		} else {
			out.Day = wire
		}
	}
	if out.Short.AvailableAt == "" {
		out.Short.AvailableAt = time.Now().UTC().Format(time.RFC3339)
	}
	if out.Day.AvailableAt == "" {
		out.Day.AvailableAt = out.Short.AvailableAt
	}
	return out
}

type backdropVideoItemWire struct {
	VideoID      string                    `json:"videoId"`
	Kind         string                    `json:"kind"`
	Title        string                    `json:"title"`
	DurationSec  *float64                  `json:"durationSec"`
	PublishedAt  *string                   `json:"publishedAt"`
	ThumbnailURL *string                   `json:"thumbnailUrl"`
	URL          string                    `json:"url"`
	Unavailable  *backdropUnavailableWire  `json:"unavailable,omitempty"`
	ChosenBy     []backdropTemplateRefWire `json:"chosenBy"`
}

type backdropUnavailableWire struct {
	Reason string `json:"reason"`
	At     string `json:"at"`
}

type backdropTemplateRefWire struct {
	TemplateID string `json:"templateId"`
	Name       string `json:"name"`
}

type backdropVideoPageWire struct {
	Items       []backdropVideoItemWire `json:"items"`
	Counts      map[string]int          `json:"counts"`
	Total       int                     `json:"total"`
	Unavailable int                     `json:"unavailable"`
	NextCursor  string                  `json:"nextCursor,omitempty"`
}

// BackdropVideoPageToWire đổi một trang danh mục sang JSON.
func BackdropVideoPageToWire(page backdrop.VideoPage) backdropVideoPageWire {
	out := backdropVideoPageWire{
		Items:  make([]backdropVideoItemWire, 0, len(page.Items)),
		Counts: map[string]int{}, Total: page.Total, Unavailable: page.Unavailable,
		NextCursor: page.NextCursor,
	}
	for kind, count := range page.Counts {
		out.Counts[kind] = count
	}
	for _, item := range page.Items {
		wire := backdropVideoItemWire{
			VideoID: item.Video.VideoID, Kind: item.Video.Kind, Title: item.Video.Title,
			DurationSec: item.Video.DurationSec, ThumbnailURL: item.Video.ThumbnailURL,
			URL: item.Video.URL, PublishedAt: timeWire(item.Video.PublishedAt),
			ChosenBy: make([]backdropTemplateRefWire, 0, len(item.ChosenBy)),
		}
		if item.Video.UnavailableAt != nil {
			wire.Unavailable = &backdropUnavailableWire{
				Reason: item.Video.UnavailableReason,
				At:     item.Video.UnavailableAt.UTC().Format(time.RFC3339),
			}
		}
		for _, ref := range item.ChosenBy {
			wire.ChosenBy = append(wire.ChosenBy, backdropTemplateRefWire{TemplateID: ref.TemplateID, Name: ref.Name})
		}
		out.Items = append(out.Items, wire)
	}
	return out
}

type backdropVideoFileWire struct {
	Path   string `json:"path"`
	Height int    `json:"height"`
	Bytes  int64  `json:"bytes"`
}

type backdropVideoWire struct {
	ID          string                    `json:"id"`
	Platform    string                    `json:"platform"`
	URL         string                    `json:"url"`
	Title       string                    `json:"title"`
	DurationSec *float64                  `json:"durationSec"`
	Segment     backdropSegmentWire       `json:"segment"`
	File        *backdropVideoFileWire    `json:"file,omitempty"`
	State       string                    `json:"state"`
	Progress    *float64                  `json:"progress,omitempty"`
	Error       *backdropChannelErrorWire `json:"errorDetail,omitempty"`
	UsedBy      []backdropTemplateRefWire `json:"usedBy"`
	AddedAt     string                    `json:"addedAt"`
	UpdatedAt   string                    `json:"updatedAt"`
}

type backdropSegmentWire struct {
	FromSec float64 `json:"fromSec"`
	ToSec   float64 `json:"toSec"`
}

// BackdropVideoToWire đổi một video lẻ sang JSON.
func BackdropVideoToWire(view backdrop.VideoView) backdropVideoWire {
	row := view.Video
	wire := backdropVideoWire{
		ID: row.ID, Platform: string(row.Platform), URL: row.URL, Title: row.Title,
		DurationSec: row.DurationSec,
		Segment:     backdropSegmentWire{FromSec: row.Segment.FromSec, ToSec: row.Segment.ToSec},
		State:       row.State, Progress: row.Progress,
		UsedBy:  make([]backdropTemplateRefWire, 0, len(view.UsedBy)),
		AddedAt: row.AddedAt.UTC().Format(time.RFC3339), UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.FilePath != "" {
		wire.File = &backdropVideoFileWire{Path: row.FilePath, Height: row.FileHeight, Bytes: row.FileBytes}
	}
	if row.ErrorCode != "" || row.Error != "" {
		wire.Error = &backdropChannelErrorWire{Code: row.ErrorCode, Message: row.Error}
	}
	for _, ref := range view.UsedBy {
		wire.UsedBy = append(wire.UsedBy, backdropTemplateRefWire{TemplateID: ref.TemplateID, Name: ref.Name})
	}
	return wire
}

type platformRequestWire struct {
	Host    string `json:"host"`
	Label   string `json:"label"`
	Count   int    `json:"count"`
	FirstAt string `json:"firstAt"`
	LastAt  string `json:"lastAt"`
}

// WriteBackdropMix ghi câu trả lời route trộn nền; cmd gọi qua adapter ServeMix.
func WriteBackdropMix(w http.ResponseWriter, r *http.Request, report backdrop.MixReport) {
	webapp.WriteJSON(w, http.StatusOK, BackdropMixToWire(report))
}

type backdropMixWire struct {
	FixedSec   float64                  `json:"fixedSec"`
	PerChannel []backdropMixChannelWire `json:"perChannel"`
	LoopSec    float64                  `json:"loopSec"`
	Need       int                      `json:"need"`
	Capped     bool                     `json:"capped"`
}

type backdropMixChannelWire struct {
	ChannelID   string `json:"channelId"`
	Clips       int    `json:"clips"`
	FreshVideos int    `json:"freshVideos"`
	Repeats     bool   `json:"repeats"`
}

// BackdropMixToWire đổi ước lượng nền sang JSON.
func BackdropMixToWire(report backdrop.MixReport) backdropMixWire {
	out := backdropMixWire{
		FixedSec: report.FixedSec, LoopSec: report.LoopSec, Need: report.Need, Capped: report.Capped,
		PerChannel: make([]backdropMixChannelWire, 0, len(report.PerChannel)),
	}
	for _, channel := range report.PerChannel {
		out.PerChannel = append(out.PerChannel, backdropMixChannelWire{
			ChannelID: channel.ChannelID, Clips: channel.Clips,
			FreshVideos: channel.FreshVideos, Repeats: channel.Repeats,
		})
	}
	return out
}

// ── lỗi ────────────────────────────────────────────────────────────────────

// WriteBackdropError map lỗi nghiệp vụ của Video nền sang HTTP; cmd dùng cho
// adapter route mix.
func WriteBackdropError(w http.ResponseWriter, err error, diag BackdropDiag) {
	var apiErr *backdrop.Error
	if errors.As(err, &apiErr) {
		status := http.StatusInternalServerError
		switch apiErr.Code {
		case backdrop.CodeInvalidInput, backdrop.CodeInvalidCursor:
			status = http.StatusBadRequest
		case backdrop.CodeNotFound:
			status = http.StatusNotFound
		case backdrop.CodeConflict, backdrop.CodeInUse, backdrop.CodeUnavailable:
			status = http.StatusConflict
		case backdrop.CodeLoginRequired:
			status = http.StatusUnauthorized
		case backdrop.CodeRateLimited:
			status = http.StatusTooManyRequests
		case backdrop.CodeUnsupportedPlatform:
			status = http.StatusUnprocessableEntity
		}
		if status == http.StatusInternalServerError {
			writeBackdropInternal(w, err, diag)
			return
		}
		body := map[string]any{"error": apiErr.Message, "code": apiErr.Code}
		if apiErr.Recovery != "" {
			body["recovery"] = apiErr.Recovery
		}
		if apiErr.RetryAt != nil {
			body["retryAt"] = apiErr.RetryAt.UTC().Format(time.RFC3339)
		}
		webapp.WriteJSON(w, status, body)
		return
	}
	// Lỗi nền tảng đã dịch (paused/budget/busy) và lỗi yt-dlp mang mã.
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) {
		status := http.StatusBadGateway
		switch coded.ErrorCode() {
		case backdrop.CodeRateLimited:
			status = http.StatusTooManyRequests
		case backdrop.CodeLoginRequired:
			status = http.StatusUnauthorized
		case backdrop.CodeUnavailable, backdrop.CodeInUse:
			status = http.StatusConflict
		}
		body := map[string]any{"error": err.Error(), "code": coded.ErrorCode()}
		if until, ok := err.(interface{ RetryAt() time.Time }); ok && !until.RetryAt().IsZero() {
			body["retryAt"] = until.RetryAt().UTC().Format(time.RFC3339)
		}
		webapp.WriteJSON(w, status, body)
		return
	}
	writeBackdropInternal(w, err, diag)
}

func writeBackdropInternal(w http.ResponseWriter, err error, diag BackdropDiag) {
	if diag != nil {
		diag(err.Error())
	}
	webapp.Error(w, http.StatusInternalServerError, "lỗi nội bộ")
}

func timeWire(value *time.Time) *string {
	if value == nil {
		return nil
	}
	out := value.UTC().Format(time.RFC3339)
	return &out
}
