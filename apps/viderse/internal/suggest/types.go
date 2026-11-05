// Package suggest sinh và quản lý Đề xuất của trợ lý: đúng ba loại (video nổi
// bật, template hay lỗi, dọn MP4 đã đăng), khoá trùng kind + kênh + đối tượng,
// tối đa 5 đề xuất mở xếp theo giá trị, bỏ qua thì 7 ngày không quay lại. Đề
// xuất không bao giờ tự chạy và không bao giờ toast: nút chính chạy một thao
// tác có sẵn, chỉ sau khi người dùng bấm.
package suggest

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
)

const (
	KindHotVideo     = "hot_video"
	KindTemplateFail = "template_failing"
	KindCleanup      = "cleanup"
	StateOpen        = "open"
	StateAccepted    = "accepted"
	StateDismissed   = "dismissed"
	StateExpired     = "expired"
	MaxOpen          = 5
	Cooldown         = 7 * 24 * time.Hour
	HotVideoWindow   = 72 * time.Hour
	HotVideoMinScore = 3.0
	FailingMin       = 3
	FailingWindow    = 7 * 24 * time.Hour
	CleanupMinBytes  = int64(5) << 30
	defaultExpiry    = 7 * 24 * time.Hour
)

// Kinds là tập loại Đề xuất đang sống; mọi hàng khác chỉ còn là lịch sử.
var Kinds = []string{KindHotVideo, KindTemplateFail, KindCleanup}

// Value xếp thứ tự hiển thị: số lớn hiện trước.
var kindValue = map[string]int{KindHotVideo: 100, KindTemplateFail: 80, KindCleanup: 40}

var (
	ErrNotFound   = errors.New("không tìm thấy đề xuất")
	ErrGone       = errors.New("đề xuất không còn mở")
	ErrBadOp      = errors.New("thao tác không được hỗ trợ")
	ErrNavigation = errors.New("đề xuất này chỉ mở màn hình, không có thao tác để chạy")
)

// ConfirmRequiredError: thao tác tốn ảnh hoặc đưa video vào làm cần xác nhận.
type ConfirmRequiredError struct{ Images int }

func (e *ConfirmRequiredError) Error() string { return "cần xác nhận trước khi chạy" }

type Cost struct {
	Images int `json:"images,omitempty"`
}
type Primary struct {
	Label  string          `json:"label"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
	Cost   *Cost           `json:"cost,omitempty"`
	// Href có nghĩa nút chính chỉ đi tới màn hình đó, không gọi accept.
	Href string `json:"href,omitempty"`
}

type Suggestion struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Agent       string         `json:"agent"`
	ChannelID   *string        `json:"channelId"`
	ChannelName *string        `json:"channelName"`
	Title       string         `json:"title"`
	Evidence    string         `json:"evidence"`
	Reason      insight.Reason `json:"reason"`
	Primary     Primary        `json:"primary"`
	CreatedAt   time.Time      `json:"createdAt"`
	ExpiresAt   time.Time      `json:"expiresAt"`
	State       string         `json:"state"`
	Value       int            `json:"-"`
	Object      string         `json:"-"`
	Cooldown    *time.Time     `json:"-"`
}

// Draft là một đề xuất do bộ sinh đề nghị, trước khi qua khoá trùng.
type Draft struct {
	Kind        string
	Agent       string
	ChannelID   string
	ChannelName string
	Object      string
	Title       string
	Evidence    string
	Reason      insight.Reason
	Primary     Primary
	Value       int
	TTL         time.Duration
}

type Store interface {
	// Upsert theo khoá trùng. Hàng đang mở được cập nhật nội dung; hàng đã xử
	// lý chỉ mở lại khi hết cooldown. reopened=false và created=false khi bị chặn.
	Upsert(ctx context.Context, d Draft, now time.Time) (s Suggestion, changed bool, err error)
	Get(ctx context.Context, id string) (Suggestion, bool, error)
	// Open trả mọi đề xuất đang mở, chưa hết hạn và thuộc loại còn sống, giá
	// trị cao trước.
	Open(ctx context.Context, now time.Time) ([]Suggestion, error)
	Resolve(ctx context.Context, id, state string, cooldownUntil *time.Time, now time.Time) (bool, error)
	// ExpireStale đóng các đề xuất mở đã quá hạn.
	ExpireStale(ctx context.Context, now time.Time) error
	// CloseOpenExcept đóng (expired) các đề xuất mở của loại này mà lần sinh
	// vừa rồi không còn đề nghị nữa.
	CloseOpenExcept(ctx context.Context, kind string, keepIDs []string, now time.Time) error
	// TrimOpen giữ tối đa n đề xuất mở có giá trị cao nhất, còn lại expired.
	TrimOpen(ctx context.Context, n int, now time.Time) error
	ChannelNames(ctx context.Context) (map[string]string, error)
	Signals
}

// Signals là các tín hiệu đọc thẳng từ dữ liệu của app (không gọi model).
type Signals interface {
	HotVideos(ctx context.Context, now time.Time) ([]HotVideo, error)
	FailingTemplates(ctx context.Context, now time.Time) ([]FailingTemplate, error)
}

type HotVideo struct {
	ChannelID, ChannelName string
	RivalTitle             string
	VideoID, Title         string
	Score                  float64
	Views                  int64
	PublishedAt            time.Time
	// Queued: đã nằm trong hàng; QueueID, QueueRank (1-based trong kênh), QueueLen.
	Queued    bool
	QueueID   string
	QueueRank int
	QueueLen  int
}
type FailingTemplate struct {
	ChannelID, ChannelName string
	TemplateID, Template   string
	Code, Message          string
	Failed, Total          int
	Since                  time.Time
}

// CleanupSource báo dung lượng MP4 đã đăng có thể xoá.
type CleanupSource interface {
	PostedMP4(ctx context.Context) (bytes int64, count int, err error)
}

// Prefs là công tắc chung của Đề xuất (Cài đặt thông báo › Đề xuất của trợ lý).
type Prefs interface {
	SuggestionsEnabled(ctx context.Context) (bool, error)
}
