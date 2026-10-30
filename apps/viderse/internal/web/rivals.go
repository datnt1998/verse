package web

// HTTP của mảng đối thủ (Khám phá): GET/POST /api/rivals, DELETE
// /api/rivals/:id, trang video, trạng thái + điều khiển quét, và khung đăng
// §12. Handler chỉ đọc request → gọi service → ghi JSON; hình wire theo hợp
// đồng dữ liệu, tách hẳn khỏi kiểu domain.

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// RivalReads là mặt đọc của mảng đối thủ — hiện thực là rivals.Service.
type RivalReads interface {
	List(ctx context.Context) (*rivals.ListResult, error)
	Rival(ctx context.Context, channelID string) (*rivals.ListEntry, error)
	Videos(ctx context.Context, channelID string, q rivals.VideoQuery) (*rivals.VideoPage, error)
	PostingSchedule(ctx context.Context, channelID, zone string) (*domain.PostingSchedule, error)
}

// RivalMutations là mặt ghi + trạng thái quét — hiện thực là scan.Service.
type RivalMutations interface {
	Add(ctx context.Context, input string) (domain.RivalChannel, error)
	Remove(ctx context.Context, channelID string) error
	Status(ctx context.Context, channelID string) (domain.ScanStatus, error)
	Control(ctx context.Context, channelID string, action domain.ScanAction) (domain.ScanStatus, error)
	SetCap(ctx context.Context, channelID string, cap int) (domain.ScanStatus, error)
}

// RivalDiag là điểm tới của lỗi nội bộ 500 như PlatformDiag.
type RivalDiag func(msg string)

// MountRivals đăng ký các route /api/rivals* trên gateway.
func MountRivals(g *Gateway, reads RivalReads, mutations RivalMutations) {
	MountRivalsDiag(g, reads, mutations, nil)
}

// MountRivalsDiag giống MountRivals nhưng có điểm tới cho lỗi nội bộ.
func MountRivalsDiag(g *Gateway, reads RivalReads, mutations RivalMutations, diag RivalDiag) {
	g.Handle(http.MethodGet, "/api/rivals", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		listRivals(w, r, reads, diag)
	})
	g.Handle(http.MethodPost, "/api/rivals", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		addRival(w, r, reads, mutations, diag)
	})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
		m := method
		g.Handle(m, "/api/rivals/", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
			routeRivals(w, r, reads, mutations, diag)
		})
	}
}

// ── wire ───────────────────────────────────────────────────────────────────

type rivalWire struct {
	ID              string        `json:"id"`
	Handle          *string       `json:"handle"`
	Title           string        `json:"title"`
	AvatarURL       *string       `json:"avatarUrl"`
	URL             string        `json:"url"`
	SubscriberCount *int64        `json:"subscriberCount"`
	VideoCount      int64         `json:"videoCount"`
	MedianViews     *int64        `json:"medianViews"`
	Outliers        int64         `json:"outliers"`
	NewOutliers     int64         `json:"newOutliers"`
	AddedAt         time.Time     `json:"addedAt"`
	ScannedAt       *time.Time    `json:"scannedAt"`
	Scan            rivalScanWire `json:"scan"`
}

type rivalScanWire struct {
	State            domain.HistoryState `json:"state"`
	DiscoveryRunning bool                `json:"discoveryRunning"`
}

type rivalListWire struct {
	Rivals      []rivalWire `json:"rivals"`
	TotalVideos int64       `json:"totalVideos"`
	NewOutliers int64       `json:"newOutliers"`
}

type videoWire struct {
	ID           string     `json:"id"`
	ChannelID    string     `json:"channelId"`
	Title        string     `json:"title"`
	DurationSec  *int       `json:"durationSec"`
	ViewCount    *int64     `json:"viewCount"`
	PublishedAt  *time.Time `json:"publishedAt"`
	ExactTime    bool       `json:"exactTime"`
	Score        *float64   `json:"score"`
	HasCaptions  *bool      `json:"hasCaptions"`
	ThumbnailURL string     `json:"thumbnailUrl"`
	URL          string     `json:"url"`
}

type videoPageWire struct {
	Videos     []videoWire `json:"videos"`
	Total      int64       `json:"total"`
	NextCursor *string     `json:"nextCursor"`
}

type scanStatusWire struct {
	ChannelID     string             `json:"channelId"`
	HistoryCap    int                `json:"historyCap"`
	IndexedVideos int                `json:"indexedVideos"`
	RescanHours   int                `json:"rescanHours"`
	History       historyStatusWire  `json:"history"`
	Discovery     discoveryStateWire `json:"discovery"`
}

type historyStatusWire struct {
	State         domain.HistoryState `json:"state"`
	JobID         *string             `json:"jobId"`
	Scanned       int                 `json:"scanned"`
	Added         int                 `json:"added"`
	Pages         int                 `json:"pages"`
	ReachedEnd    bool                `json:"reachedEnd"`
	StartedAt     *time.Time          `json:"startedAt"`
	UpdatedAt     *time.Time          `json:"updatedAt"`
	NextAttemptAt *time.Time          `json:"nextAttemptAt"`
	ErrorCode     *string             `json:"errorCode"`
	Error         *string             `json:"error"`
	Recovery      *string             `json:"recovery"`
}

type discoveryStateWire struct {
	CheckedAt *time.Time `json:"checkedAt"`
	CaughtUp  bool       `json:"caughtUp"`
	Running   bool       `json:"running"`
	NextAt    *time.Time `json:"nextAt"`
}

type postingSlotWire struct {
	Weekdays   []int                `json:"weekdays"`
	Start      string               `json:"start"`
	End        string               `json:"end"`
	Count      int                  `json:"count"`
	WeeksHit   int                  `json:"weeksHit"`
	WeeksTotal int                  `json:"weeksTotal"`
	SinceWeeks int                  `json:"sinceWeeks"`
	Rhythm     domain.PostingRhythm `json:"rhythm"`
	IsTop      bool                 `json:"isTop"`
}

type postingScheduleWire struct {
	ChannelID   string             `json:"channelId"`
	Zone        domain.PostingZone `json:"zone"`
	Sample      int                `json:"sample"`
	Total       int                `json:"total"`
	Pending     int                `json:"pending"`
	PausedUntil *time.Time         `json:"pausedUntil"`
	PerWeek     *float64           `json:"perWeek"`
	WeeksTotal  int                `json:"weeksTotal"`
	OffSlot     int                `json:"offSlot"`
	Grid        [7][12]int         `json:"grid"`
	Slots       []postingSlotWire  `json:"slots"`
}

// RivalToWire chuyển một dòng danh sách sang hình wire — dùng chung cho HTTP
// và CLI (hợp đồng JSON của Rival một chỗ duy nhất).
func RivalToWire(e rivals.ListEntry) rivalWire {
	return rivalWire{
		ID:              e.Channel.ID,
		Handle:          optStr(e.Channel.Handle),
		Title:           e.Channel.Title,
		AvatarURL:       optStr(e.Channel.AvatarURL),
		URL:             e.Channel.URL,
		SubscriberCount: e.Channel.SubscriberCount,
		VideoCount:      e.VideoCount,
		MedianViews:     e.Channel.MedianViews,
		Outliers:        e.Outliers,
		NewOutliers:     e.NewOutliers,
		AddedAt:         e.Channel.AddedAt.UTC(),
		ScannedAt:       timeOrNil(e.Channel.ScannedAt),
		Scan: rivalScanWire{
			State:            e.HistoryState,
			DiscoveryRunning: e.DiscoveryRunning,
		},
	}
}

// VideoToWire chuyển một video đối thủ sang hình wire.
func VideoToWire(v domain.RivalVideo) videoWire {
	published := v.PublishedAt
	if v.UploadedAt != nil {
		published = v.UploadedAt
	}
	return videoWire{
		ID:           v.ID,
		ChannelID:    v.ChannelID,
		Title:        v.Title,
		DurationSec:  v.DurationSec,
		ViewCount:    v.ViewCount,
		PublishedAt:  timeOrNil(published),
		ExactTime:    v.UploadedAt != nil,
		Score:        v.Score,
		HasCaptions:  v.HasCaptions,
		ThumbnailURL: "https://i.ytimg.com/vi/" + v.ID + "/mqdefault.jpg",
		URL:          "https://www.youtube.com/watch?v=" + v.ID,
	}
}

// ScanStatusToWire chuyển trạng thái quét sang hình wire.
func ScanStatusToWire(s domain.ScanStatus) scanStatusWire {
	return scanStatusWire{
		ChannelID:     s.ChannelID,
		HistoryCap:    s.HistoryCap,
		IndexedVideos: s.IndexedVideos,
		RescanHours:   s.RescanHours,
		History: historyStatusWire{
			State:         s.History.State,
			JobID:         optStr(s.History.JobID),
			Scanned:       s.History.Scanned,
			Added:         s.History.Added,
			Pages:         s.History.Pages,
			ReachedEnd:    s.History.ReachedEnd,
			StartedAt:     timeOrNil(s.History.StartedAt),
			UpdatedAt:     timeOrNil(s.History.UpdatedAt),
			NextAttemptAt: timeOrNil(s.History.NextAttemptAt),
			ErrorCode:     optStr(s.History.ErrorCode),
			Error:         optStr(s.History.Error),
			Recovery:      optStr(s.History.Recovery),
		},
		Discovery: discoveryStateWire{
			CheckedAt: timeOrNil(s.Discovery.CheckedAt),
			CaughtUp:  s.Discovery.CaughtUp,
			Running:   s.Discovery.Running,
			NextAt:    timeOrNil(s.Discovery.NextAt),
		},
	}
}

// PostingScheduleToWire chuyển lịch đăng §12 sang hình wire.
func PostingScheduleToWire(s *domain.PostingSchedule) postingScheduleWire {
	out := postingScheduleWire{
		ChannelID:   s.ChannelID,
		Zone:        s.Zone,
		Sample:      s.Sample,
		Total:       s.Total,
		Pending:     s.Pending,
		PausedUntil: timeOrNil(s.PausedUntil),
		PerWeek:     s.PerWeek,
		WeeksTotal:  s.WeeksTotal,
		OffSlot:     s.OffSlot,
		Grid:        s.Grid,
		Slots:       make([]postingSlotWire, 0, len(s.Slots)),
	}
	for _, sl := range s.Slots {
		out.Slots = append(out.Slots, postingSlotWire{
			Weekdays:   sl.Weekdays,
			Start:      sl.Start,
			End:        sl.End,
			Count:      sl.Count,
			WeeksHit:   sl.WeeksHit,
			WeeksTotal: sl.WeeksTotal,
			SinceWeeks: sl.SinceWeeks,
			Rhythm:     sl.Rhythm,
			IsTop:      sl.IsTop,
		})
	}
	return out
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timeOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// ── handlers ───────────────────────────────────────────────────────────────

func listRivals(w http.ResponseWriter, r *http.Request, reads RivalReads, diag RivalDiag) {
	list, err := reads.List(r.Context())
	if err != nil {
		writeRivalInternal(w, err, diag)
		return
	}
	out := rivalListWire{
		Rivals:      make([]rivalWire, 0, len(list.Rivals)),
		TotalVideos: list.TotalVideos,
		NewOutliers: list.NewOutliers,
	}
	for _, e := range list.Rivals {
		out.Rivals = append(out.Rivals, RivalToWire(e))
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type addRivalBody struct {
	Input string `json:"input"`
}

func addRival(w http.ResponseWriter, r *http.Request, reads RivalReads, mutations RivalMutations, diag RivalDiag) {
	var body addRivalBody
	if !webapp.DecodeJSONCode(w, r, &body, scan.CodeInvalidInput) {
		return
	}
	channel, err := mutations.Add(r.Context(), body.Input)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	entry, err := reads.Rival(r.Context(), channel.ID)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusCreated, RivalToWire(*entry))
}

// routeRivals khớp phần sau /api/rivals/ theo hình
// {id} | {id}/videos | {id}/scan | {id}/posting-slots.
func routeRivals(w http.ResponseWriter, r *http.Request, reads RivalReads, mutations RivalMutations, diag RivalDiag) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/rivals/")
	parts := strings.SplitN(rest, "/", 2)
	if parts[0] == "" {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	id, tail := parts[0], ""
	if len(parts) == 2 {
		tail = parts[1]
	}
	ctx := r.Context()
	switch {
	case tail == "" && r.Method == http.MethodDelete:
		if err := mutations.Remove(ctx, id); err != nil {
			writeRivalError(w, r.Context(), err, reads, diag)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case tail == "videos" && r.Method == http.MethodGet:
		getRivalVideos(w, r, reads, diag, id)
	case tail == "scan" && r.Method == http.MethodGet:
		st, err := mutations.Status(ctx, id)
		if err != nil {
			writeRivalError(w, r.Context(), err, reads, diag)
			return
		}
		webapp.WriteJSON(w, http.StatusOK, ScanStatusToWire(st))
	case tail == "scan" && r.Method == http.MethodPost:
		postScanAction(w, r, mutations, reads, diag, id)
	case tail == "scan" && r.Method == http.MethodPatch:
		patchScanCap(w, r, mutations, reads, diag, id)
	case tail == "posting-slots" && r.Method == http.MethodGet:
		getPostingSlots(w, r, reads, diag, id)
	default:
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
	}
}

func getRivalVideos(w http.ResponseWriter, r *http.Request, reads RivalReads, diag RivalDiag, id string) {
	params := r.URL.Query()
	q := rivals.VideoQuery{
		Sort:   params.Get("sort"),
		Query:  params.Get("q"),
		Cursor: params.Get("cursor"),
	}
	// long chỉ nhận vắng/0/1 — giá trị lạ là client sai, không lặng lẽ thành false.
	switch params.Get("long") {
	case "", "0":
	case "1":
		q.Long = true
	default:
		writeRivalCode(w, http.StatusBadRequest, "long phải là 0 hoặc 1", scan.CodeInvalidInput, "", nil)
		return
	}
	if q.Sort == "" {
		q.Sort = rivals.SortOutlier
	}
	if v := params.Get("within"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil {
			writeRivalCode(w, http.StatusBadRequest, "khoảng thời gian không hợp lệ", scan.CodeInvalidInput, "", nil)
			return
		}
		q.WithinDays = days
	}
	if v := params.Get("limit"); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil {
			writeRivalCode(w, http.StatusBadRequest, "limit không hợp lệ", scan.CodeInvalidInput, "", nil)
			return
		}
		q.Limit = limit
	} else {
		q.Limit = rivals.VideoPageDefault
	}
	page, err := reads.Videos(r.Context(), id, q)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	out := videoPageWire{
		Videos: make([]videoWire, 0, len(page.Videos)),
		Total:  page.Total,
	}
	for _, v := range page.Videos {
		out.Videos = append(out.Videos, VideoToWire(v))
	}
	if page.NextCursor != "" {
		out.NextCursor = &page.NextCursor
	}
	webapp.WriteJSON(w, http.StatusOK, out)
}

type scanActionBody struct {
	Action domain.ScanAction `json:"action"`
}

func postScanAction(w http.ResponseWriter, r *http.Request, mutations RivalMutations, reads RivalReads, diag RivalDiag, id string) {
	var body scanActionBody
	if !webapp.DecodeJSONCode(w, r, &body, scan.CodeInvalidInput) {
		return
	}
	switch body.Action {
	case domain.ScanActionStart, domain.ScanActionPause, domain.ScanActionResume, domain.ScanActionCancel, domain.ScanActionCheck:
	default:
		writeRivalCode(w, http.StatusBadRequest, "action phải là start|pause|resume|cancel|check", scan.CodeInvalidInput, "", nil)
		return
	}
	st, err := mutations.Control(r.Context(), id, body.Action)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ScanStatusToWire(st))
}

type scanCapBody struct {
	HistoryCap int `json:"historyCap"`
}

func patchScanCap(w http.ResponseWriter, r *http.Request, mutations RivalMutations, reads RivalReads, diag RivalDiag, id string) {
	// Cap là số nguyên dương: thiếu/số lẻ/chuỗi/sai kiểu đều là invalid_cap,
	// không rơi vào lỗi decode chung.
	var body scanCapBody
	if !webapp.DecodeJSONCode(w, r, &body, scan.CodeInvalidCap) {
		return
	}
	if body.HistoryCap == 0 {
		writeRivalCode(w, http.StatusBadRequest, "historyCap là bắt buộc",
			scan.CodeInvalidCap, "", nil)
		return
	}
	st, err := mutations.SetCap(r.Context(), id, body.HistoryCap)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, ScanStatusToWire(st))
}

func getPostingSlots(w http.ResponseWriter, r *http.Request, reads RivalReads, diag RivalDiag, id string) {
	zone := r.URL.Query().Get("zone")
	if zone == "" {
		zone = domain.ZoneVN
	}
	sched, err := reads.PostingSchedule(r.Context(), id, zone)
	if err != nil {
		writeRivalError(w, r.Context(), err, reads, diag)
		return
	}
	webapp.WriteJSON(w, http.StatusOK, PostingScheduleToWire(sched))
}

// ── lỗi ────────────────────────────────────────────────────────────────────

// writeRivalError map lỗi nghiệp vụ sang status + vỏ {error, code, recovery?,
// retryAt?}: scan.Error theo mã (invalid_input→400, not_found→404,
// exists→409 kèm rival, rate_limited→429 kèm retryAt, blocked→503,
// unexpected_page→502, invalid_cap→400); lỗi đọc của rivals (not_found 404,
// invalid_* 400, invalid_cursor 400); lỗi còn lại là lỗi nội bộ 500.
func writeRivalError(w http.ResponseWriter, ctx context.Context, err error, reads RivalReads, diag RivalDiag) {
	var se *scan.Error
	if errors.As(err, &se) {
		status := http.StatusInternalServerError
		switch se.Code {
		case scan.CodeInvalidInput, scan.CodeInvalidCap:
			status = http.StatusBadRequest
		case scan.CodeNotFound:
			status = http.StatusNotFound
		case scan.CodeExists:
			status = http.StatusConflict
		case scan.CodeRateLimited:
			status = http.StatusTooManyRequests
		case scan.CodeBlocked:
			status = http.StatusServiceUnavailable
		case scan.CodeUnexpectedPage, scan.CodeTransient:
			status = http.StatusBadGateway
		case scan.CodeLeaseBusy:
			status = http.StatusServiceUnavailable
		}
		if status == http.StatusInternalServerError {
			writeRivalInternal(w, err, diag)
			return
		}
		body := map[string]any{"error": se.Message, "code": se.Code}
		if se.Recovery != "" {
			body["recovery"] = se.Recovery
		}
		if se.RetryAt != nil {
			body["retryAt"] = se.RetryAt.UTC().Format(time.RFC3339)
		}
		if se.Code == scan.CodeExists && se.Existing != nil && reads != nil {
			entry, lerr := reads.Rival(ctx, se.Existing.ID)
			if lerr != nil {
				// Không dựng số đếm giả: đọc lại hỏng thì đây là lỗi nội bộ.
				writeRivalInternal(w, lerr, diag)
				return
			}
			body["rival"] = RivalToWire(*entry)
		}
		webapp.WriteJSON(w, status, body)
		return
	}
	switch {
	case errors.Is(err, rivals.ErrRivalNotFound):
		writeRivalCode(w, http.StatusNotFound, "Không tìm thấy kênh đối thủ.", scan.CodeNotFound, "", nil)
	case errors.Is(err, rivals.ErrInvalidCursor):
		writeRivalCode(w, http.StatusBadRequest, "Cursor không hợp lệ.", "invalid_cursor", "", nil)
	case errors.Is(err, rivals.ErrInvalidSort), errors.Is(err, rivals.ErrInvalidWithin),
		errors.Is(err, rivals.ErrInvalidZone), errors.Is(err, rivals.ErrInvalidLimit),
		errors.Is(err, rivals.ErrInvalidQuery):
		writeRivalCode(w, http.StatusBadRequest, err.Error(), scan.CodeInvalidInput, "", nil)
	default:
		writeRivalInternal(w, err, diag)
	}
}

// writeRivalCode ghi vỏ lỗi {error, code, recovery?, retryAt?}.
func writeRivalCode(w http.ResponseWriter, status int, msg, code, recovery string, retryAt *time.Time) {
	body := map[string]any{"error": msg, "code": code}
	if recovery != "" {
		body["recovery"] = recovery
	}
	if retryAt != nil {
		body["retryAt"] = retryAt.UTC().Format(time.RFC3339)
	}
	webapp.WriteJSON(w, status, body)
}

// writeRivalInternal trả vỏ 500 trung lập — nguyên nhân chỉ đi tới diag.
func writeRivalInternal(w http.ResponseWriter, err error, diag RivalDiag) {
	if diag != nil {
		diag(err.Error())
	}
	writeRivalCode(w, http.StatusInternalServerError, "lỗi nội bộ", "internal", "", nil)
}
