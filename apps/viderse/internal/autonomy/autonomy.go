// Package autonomy là Mức tự chủ của kênh (07 §20, 01 §9.4): máy được tự làm
// gì mà không hỏi — lấy video, duyệt ảnh theo điểm, thử lại lỗi một lần. Không
// đường nào ở đây tự đăng, tự xoá hay đổi template.
package autonomy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

var (
	// ErrChannelNotFound: kênh không tồn tại.
	ErrChannelNotFound = errors.New("autonomy: không tìm thấy kênh")
	// ErrInvalid: yêu cầu không hợp lệ (mức lạ, ngưỡng ngoài 70–95…).
	ErrInvalid = errors.New("autonomy: yêu cầu không hợp lệ")
	// ErrConfirmRequired: "Tự duyệt hết" chỉ bật khi yêu cầu mang xác nhận rõ.
	ErrConfirmRequired = errors.New("autonomy: cần xác nhận rõ")
)

// RaiseMinStreak là số video duyệt liền không vẽ lại mà Học cần thấy để gợi ý
// nâng mức (01 §9.4).
const RaiseMinStreak = 15

// RaiseSignal là tín hiệu Học gửi trợ lý: kênh đủ chuỗi duyệt sạch để đề xuất
// nâng Duyệt ảnh từ "Hỏi" lên "Theo điểm".
type RaiseSignal struct {
	ChannelID   string
	ChannelName string
	Streak      int
	CurrentMode string
	Min         int
}

// ItemScore là điểm của ảnh đang hiển thị của một mục trong manifest ảnh.
type ItemScore struct {
	ID      string
	Kind    string // scene | thumbnail
	Label   string // "cảnh 4" | "ảnh bìa"
	Skipped bool
	Score   *domain.ImageScore
}

// ReviewState là ảnh của một video đang ở Chờ duyệt: revision manifest và
// điểm từng mục; Ready báo mọi ảnh đã đủ để duyệt.
type ReviewState struct {
	Revision int
	Ready    bool
	Items    []ItemScore
}

// Reviewer đọc và duyệt ảnh của job — hiện thực ở clone/images (cùng đường
// với nút Duyệt hết của chủ kênh; không có đường vòng qua cổng duyệt).
type Reviewer interface {
	ReviewState(ctx context.Context, jobID string) (ReviewState, error)
	ApproveAll(ctx context.Context, jobID string, revision int) error
}

// BelowItem là một ảnh chưa đạt ngưỡng; Value âm khi chưa có điểm.
type BelowItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

// Verdict là kết quả đối chiếu điểm với ngưỡng.
type Verdict struct {
	Min     int         `json:"min"`
	Total   int         `json:"total"`
	Passed  int         `json:"passed"`
	Below   []BelowItem `json:"below"`
	AllPass bool        `json:"allPass"`
	// Lowest là điểm thấp nhất trong các ảnh đã chấm (0 khi chưa ảnh nào có điểm).
	Lowest int `json:"lowest"`
}

// Evaluate đối chiếu điểm với ngưỡng min: mục bị bỏ qua không tính; mục chưa
// có điểm là "chưa đạt" — không bao giờ tự duyệt ảnh chưa chấm.
func Evaluate(min int, items []ItemScore) Verdict {
	v := Verdict{Min: min, Below: []BelowItem{}}
	for _, it := range items {
		if it.Skipped {
			continue
		}
		v.Total++
		if it.Score != nil && (v.Lowest == 0 || it.Score.Value < v.Lowest) {
			v.Lowest = it.Score.Value
		}
		if it.Score != nil && it.Score.Value >= min {
			v.Passed++
			continue
		}
		value := -1
		if it.Score != nil {
			value = it.Score.Value
		}
		v.Below = append(v.Below, BelowItem{ID: it.ID, Label: it.Label, Value: value})
	}
	v.AllPass = v.Total > 0 && len(v.Below) == 0
	return v
}

// Summary là câu tóm tắt vì sao video ở lại Chờ duyệt: "cảnh 4 được 72".
func (v Verdict) Summary() string {
	parts := make([]string, 0, len(v.Below))
	for _, b := range v.Below {
		if b.Value < 0 {
			parts = append(parts, b.Label+" chưa có điểm")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s được %d", b.Label, b.Value))
	}
	return strings.Join(parts, " · ")
}

// ItemLabel đặt tên hiển thị của mục ảnh: cảnh theo thứ tự trong manifest.
func ItemLabel(kind string, sceneIndex int) string {
	if kind == "thumbnail" {
		return "ảnh bìa"
	}
	return fmt.Sprintf("cảnh %d", sceneIndex)
}

// PendingReview là một video đang chờ duyệt mà kênh cho phép máy xét điểm.
type PendingReview struct {
	JobID         string
	ChannelID     string
	Title         string
	ImageRevision int
	Autonomy      domain.Autonomy
	Tries         int
}

// ReviewOutcome ghi kết quả một lần xét điểm của một revision ảnh.
type ReviewOutcome struct {
	JobID    string
	Revision int
	Result   string // approved | below | error
	Tries    int
	Summary  string
	Verdict  Verdict
	Autonomy domain.Autonomy
	Title    string
}

// Kết quả xét điểm.
const (
	ResultApproved = "approved"
	ResultBelow    = "below"
	ResultError    = "error"
)

// MaxReviewTries là số lần thử duyệt tối đa cho một revision khi cổng duyệt
// báo lỗi; sau đó video ở lại Chờ duyệt cho chủ kênh.
const MaxReviewTries = 3

// Store là phần bền vững Service cần; hiện thực ở persistence/postgres.
type Store interface {
	ChannelAutonomy(ctx context.Context, channelID string) (a domain.Autonomy, name string, found bool, err error)
	// SetChannelAutonomy đổi mức và trả mức trước đó (để Hoàn tác).
	SetChannelAutonomy(ctx context.Context, channelID string, next domain.Autonomy) (prev domain.Autonomy, found bool, err error)
	JobAutonomy(ctx context.Context, jobID string) (a domain.Autonomy, found bool, err error)
	PendingReviews(ctx context.Context, limit int) ([]PendingReview, error)
	RecordReview(ctx context.Context, o ReviewOutcome) error
	// BeginAutoFix: lần lỗi thứ nhất của job-generation → đặt step về pending,
	// ghi autoFix=retrying + Hành động tự làm và trả true; lần thứ hai → ghi
	// failed và trả false.
	BeginAutoFix(ctx context.Context, jobID string, step domain.CloneStep, code string, now time.Time) (retry bool, err error)
	// SettleAutoFix đánh dấu recovered khi step được tự thử lại đã xong.
	SettleAutoFix(ctx context.Context, jobID string, step domain.CloneStep) error
	RaiseCandidates(ctx context.Context, minStreak int) ([]RaiseSignal, error)
}

// Service là cửa duy nhất của Mức tự chủ cho trợ lý (set_autonomy), Học
// (raise_autonomy) và pipeline (tự thử lại, tự duyệt).
type Service struct {
	Store    Store
	Reviewer Reviewer
	// Publish phát sự kiện SSE sau khi máy tự làm điều gì đó.
	Publish func(eventType string, data any)
	// ChannelChanged báo kênh đổi để web tải lại (cùng đường với sửa kênh).
	ChannelChanged func(channelID string)
	// Diag nhận lỗi của việc nền (nil → bỏ).
	Diag func(string)
	// RetryDelay là khoảng nghỉ trước lần tự thử lại (0 → 3 giây).
	RetryDelay time.Duration
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) diag(format string, args ...any) {
	if s.Diag != nil {
		s.Diag(fmt.Sprintf(format, args...))
	}
}

// GetAutonomy đọc Mức tự chủ của kênh.
func (s *Service) GetAutonomy(ctx context.Context, channelID string) (domain.Autonomy, error) {
	a, _, found, err := s.Store.ChannelAutonomy(ctx, channelID)
	if err != nil {
		return domain.Autonomy{}, err
	}
	if !found {
		return domain.Autonomy{}, ErrChannelNotFound
	}
	return a, nil
}

// Preset trả tên mức hiện tại (careful|balanced|auto|custom).
func (s *Service) Preset(ctx context.Context, channelID string) (string, error) {
	a, err := s.GetAutonomy(ctx, channelID)
	return a.Preset, err
}

// ApplyPreset áp một mức sẵn (careful|balanced|auto) — set_autonomy của trợ
// lý. Hộp xác nhận của trợ lý là xác nhận của chủ kênh; "Tự duyệt hết" và
// Tuỳ chỉnh không đi qua đường này.
func (s *Service) ApplyPreset(ctx context.Context, channelID, preset string) error {
	p, ok := domain.PresetAutonomy(preset)
	if !ok {
		return fmt.Errorf("%w: mức %q", ErrInvalid, preset)
	}
	cur, err := s.GetAutonomy(ctx, channelID)
	if err != nil {
		return err
	}
	// Giữ ngưỡng điểm chủ kênh đã chọn khi đổi mức.
	p.Approve.Min = cur.Approve.Min
	_, found, err := s.Store.SetChannelAutonomy(ctx, channelID, p.Normalize())
	if err != nil {
		return err
	}
	if !found {
		return ErrChannelNotFound
	}
	s.publishChannel(channelID)
	return nil
}

// SetApprove đổi dòng Duyệt ảnh — "raise_autonomy" của Học sau khi chủ kênh
// đồng ý. mode chỉ là ask|score; "all" cần đường PATCH kênh có xác nhận. Trả
// mức trước đó để Hoàn tác.
func (s *Service) SetApprove(ctx context.Context, channelID, mode string, min int) (domain.Autonomy, error) {
	switch mode {
	case domain.ApproveAsk, domain.ApproveScore:
	case domain.ApproveAll:
		return domain.Autonomy{}, ErrConfirmRequired
	default:
		return domain.Autonomy{}, fmt.Errorf("%w: cách duyệt %q", ErrInvalid, mode)
	}
	cur, err := s.GetAutonomy(ctx, channelID)
	if err != nil {
		return domain.Autonomy{}, err
	}
	if min == 0 {
		min = cur.Approve.Min
	}
	next := cur
	next.Approve = domain.AutonomyApprove{Mode: mode, Min: min}
	if err := next.Validate(); err != nil {
		return domain.Autonomy{}, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	prev, found, err := s.Store.SetChannelAutonomy(ctx, channelID, next.Normalize())
	if err != nil {
		return domain.Autonomy{}, err
	}
	if !found {
		return domain.Autonomy{}, ErrChannelNotFound
	}
	s.publishChannel(channelID)
	return prev, nil
}

// Restore đặt lại mức đã đọc trước đó (Hoàn tác của trợ lý/Học).
func (s *Service) Restore(ctx context.Context, channelID string, prev domain.Autonomy) error {
	if err := prev.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	_, found, err := s.Store.SetChannelAutonomy(ctx, channelID, prev.Normalize())
	if err != nil {
		return err
	}
	if !found {
		return ErrChannelNotFound
	}
	s.publishChannel(channelID)
	return nil
}

// RaiseCandidates trả các kênh đủ chuỗi duyệt sạch để Học đề xuất nâng mức.
func (s *Service) RaiseCandidates(ctx context.Context) ([]RaiseSignal, error) {
	return s.Store.RaiseCandidates(ctx, RaiseMinStreak)
}

func (s *Service) publishChannel(channelID string) {
	if s.ChannelChanged != nil {
		s.ChannelChanged(channelID)
	}
}
