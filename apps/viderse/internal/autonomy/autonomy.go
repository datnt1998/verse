// Package autonomy là phần máy tự làm thay chủ kênh (07 §20): tự duyệt ảnh khi
// mọi ảnh đạt ngưỡng cố định 85 và tự thử lại lỗi đúng một lần. Chính sách là
// cố định sau V38 — không còn Mức tự chủ cấu hình được, và không đường nào ở
// đây tự đăng, tự xoá hay đổi template.
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
	// ErrInvalid: yêu cầu không hợp lệ.
	ErrInvalid = errors.New("autonomy: yêu cầu không hợp lệ")
)

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

// PendingReview là một video đang chờ duyệt mà kênh bật Tự duyệt.
type PendingReview struct {
	JobID         string
	ChannelID     string
	Title         string
	ImageRevision int
	Tries         int
	// AutoApprove là cờ Tự duyệt của kênh tại thời điểm đọc; run.go đọc lại
	// trước khi duyệt để toggle tắt giữa lượt poll không bị bỏ qua.
	AutoApprove bool
}

// ReviewOutcome ghi kết quả một lần xét điểm của một revision ảnh.
type ReviewOutcome struct {
	JobID    string
	Revision int
	Result   string // approved | below | error
	Tries    int
	Summary  string
	Verdict  Verdict
	Title    string
	// AutoApprove ghi kết quả này thuộc nhánh Tự duyệt (ngưỡng cố định 85) để
	// reason/notification dùng đúng câu.
	AutoApprove bool
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
	PendingReviews(ctx context.Context, limit int) ([]PendingReview, error)
	RecordReview(ctx context.Context, o ReviewOutcome) error
	// ChannelAutoApprove đọc lại cờ Tự duyệt ngay trước khi duyệt.
	ChannelAutoApprove(ctx context.Context, channelID string) (bool, error)
	// BeginAutoFix: lần lỗi thứ nhất của job-generation → đặt step về pending,
	// ghi autoFix=retrying + Hành động tự làm và trả true; lần thứ hai → ghi
	// failed và trả false.
	BeginAutoFix(ctx context.Context, jobID string, step domain.CloneStep, code string, now time.Time) (retry bool, err error)
	// SettleAutoFix đánh dấu recovered khi step được tự thử lại đã xong.
	SettleAutoFix(ctx context.Context, jobID string, step domain.CloneStep) error
}

// Service là cửa duy nhất của phần máy tự làm cho pipeline (tự thử lại, tự
// duyệt).
type Service struct {
	Store    Store
	Reviewer Reviewer
	// Publish phát sự kiện SSE sau khi máy tự làm điều gì đó.
	Publish func(eventType string, data any)
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

