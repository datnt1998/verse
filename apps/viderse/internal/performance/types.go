// Package performance đọc số liệu công khai của video đã đăng (24 giờ, 7 ngày,
// 28 ngày), rút nhận xét cho kênh và đưa kết quả về Tự lấy / Template luân phiên.
// Lỗi đọc không bao giờ chặn việc khác: số liệu thiếu chỉ để trống.
package performance

import (
	"errors"
	"time"
)

// Milestone là mốc đọc số liệu sau khi đăng.
type Milestone string

const (
	M24h Milestone = "24h"
	M7d  Milestone = "7d"
	M28d Milestone = "28d"
)

// Milestones theo thứ tự thời gian.
var Milestones = []Milestone{M24h, M7d, M28d}

// After là khoảng từ lúc đăng tới mốc; Grace là khoảng muộn tối đa còn chấp
// nhận số đọc được (đọc quá muộn sẽ gán nhầm lượt xem của thời điểm khác).
func (m Milestone) After() time.Duration {
	switch m {
	case M24h:
		return 24 * time.Hour
	case M7d:
		return 7 * 24 * time.Hour
	default:
		return 28 * 24 * time.Hour
	}
}

func (m Milestone) Grace() time.Duration {
	switch m {
	case M24h:
		return 24 * time.Hour
	case M7d:
		return 3 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// Source ghi số được đọc bằng đường nào.
type Source string

const (
	SourceDataAPI    Source = "data_api"
	SourcePublicPage Source = "public_page"
)

// Stat là số đọc ở một mốc. Trang công khai chỉ có lượt xem nên Likes và
// Comments là nil.
type Stat struct {
	Views    int64     `json:"views"`
	Likes    *int64    `json:"likes"`
	Comments *int64    `json:"comments"`
	ReadAt   time.Time `json:"readAt"`
}

// PostedStats là một video đã đăng trong GET /api/channels/:id/performance.
type PostedStats struct {
	JobID       string    `json:"jobId"`
	YoutubeID   string    `json:"youtubeId,omitempty"`
	PostedAt    time.Time `json:"postedAt"`
	At24h       *Stat     `json:"at24h,omitempty"`
	At7d        *Stat     `json:"at7d,omitempty"`
	At28d       *Stat     `json:"at28d,omitempty"`
	VsMedian7d  *float64  `json:"vsMedian7d,omitempty"`
	Title       string    `json:"title"`
	TemplateID  string    `json:"templateId,omitempty"`
	TemplateTag string    `json:"templateName,omitempty"`
}

// Dimension là chiều so sánh của nhận xét (ia-flows §9.5).
type Dimension string

const (
	DimTemplate     Dimension = "template"
	DimTitlePattern Dimension = "title_pattern"
	DimLength       Dimension = "length"
	DimPostHour     Dimension = "post_hour"
)

// Group là một nhóm video trong nhận xét.
type Group struct {
	Label    string `json:"label"`
	N        int    `json:"n"`
	Median7d int64  `json:"median7d"`
}

// Apply là việc nút của nhận xét sẽ làm; nhận xét không có việc tự động thì bỏ trống.
type Apply struct {
	Op     string         `json:"op"` // add_rule | template.rotation
	Params map[string]any `json:"params"`
}

// Insight là một nhận xét.
type Insight struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channelId"`
	Dimension   Dimension `json:"dimension"`
	Text        string    `json:"text"`
	Groups      []Group   `json:"groups"`
	Preliminary bool      `json:"preliminary"`
	Apply       *Apply    `json:"apply,omitempty"`

	// Ratio là tỉ lệ trung vị nhóm tốt nhất / nhóm kém nhất (dùng để xếp hạng).
	Ratio float64 `json:"-"`
}

// View là kết quả GET /api/channels/:id/performance.
type View struct {
	Posted   []PostedStats `json:"posted"`
	Insights []Insight     `json:"insights"`
	NeedMore int           `json:"needMore"`
	// Counted là số video đã có số 7 ngày (nhãn "Sơ bộ · N video").
	Counted  int    `json:"counted"`
	Median7d *int64 `json:"median7d"`
	Source   Source `json:"source"`
}

// ApplyResult là kết quả áp dụng một nhận xét.
type ApplyResult struct {
	Kind           string `json:"kind"` // rule | rotation
	RuleID         string `json:"ruleId,omitempty"`
	AlreadyApplied bool   `json:"-"`
}

// InsightSignal là tín hiệu perf_insight cho bộ sinh đề xuất.
type InsightSignal struct {
	ChannelID   string
	InsightID   string
	Dimension   string
	Text        string
	Preliminary bool
	Videos      int
	ApplyOp     string
}

// Ngưỡng theo ia-flows §9.5.
const (
	MinVideos        = 6   // video có số 7 ngày tối thiểu để so sánh
	MinGroup         = 3   // video tối thiểu mỗi nhóm
	MinRatio         = 1.3 // chênh tối thiểu
	PreliminaryBelow = 20  // dưới ngưỡng này gắn "Sơ bộ"
	MaxInsights      = 3
)

var (
	// ErrInsightGone: nhận xét không còn (số liệu đổi nên tính lại ra khác).
	ErrInsightGone = errors.New("Nhận xét này không còn nữa")
	// ErrNoRuleAdder: chưa có nơi nhận quy tắc đã học.
	ErrNoRuleAdder = errors.New("Chưa lưu được quy tắc lúc này")
	// ErrNotPosted: video chưa ở trạng thái có thể đánh dấu đã đăng.
	ErrNotReady = errors.New("Chỉ đánh dấu đã đăng cho video đã xuất xong")
	// ErrNotFound: không có video.
	ErrNotFound = errors.New("Không tìm thấy video")
	// ErrBadURL: link YouTube không hợp lệ.
	ErrBadURL = errors.New("Link YouTube không hợp lệ")
)
