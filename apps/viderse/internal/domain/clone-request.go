package domain

import "time"

type CloneEnqueueRequest struct {
	ChannelID string        `json:"channelId"`
	URLs      []string      `json:"urls"`
	Priority  ClonePriority `json:"priority,omitempty"`
	NoImages  bool          `json:"noImages,omitempty"`
}
type ClonePlanAccepted struct {
	URL                string   `json:"url"`
	VideoID            string   `json:"videoId"`
	Title              string   `json:"title"`
	DurationSec        *float64 `json:"durationSec"`
	Outlier            *float64 `json:"outlier"`
	HasCaptions        bool     `json:"hasCaptions"`
	ThumbnailURL       *string  `json:"thumbnailUrl"`
	SourceChannelTitle *string  `json:"sourceChannelTitle"`
	ViewCount          *int64   `json:"viewCount"`
	PublishedAt        *time.Time `json:"publishedAt"`
	// ImageEstimate là số ảnh trả phí dự kiến sẽ vẽ (nguyên tắc "nói trước
	// khi tốn"); nil khi chưa ước lượng được (thiếu recipe/duration).
	ImageEstimate      *int      `json:"imageEstimate"`
	// ImageCalls là số lần gọi vẽ trả phí dự kiến (gồm vẽ lại dự phòng).
	ImageCalls         *int      `json:"imageCalls"`
	// PriceUSD là giá ước tính của lượt vẽ; nil — hầu hết provider không báo giá.
	PriceUSD           *float64  `json:"priceUSD"`
}
type ClonePlanDuplicate struct {
	URL   string `json:"url"`
	// JobID là job đang hoạt động của video này; null khi video mới chỉ nằm
	// trong hàng chờ (state "queued").
	JobID *string `json:"jobId"`
	State string  `json:"state"`
	Title string  `json:"title,omitempty"`
}
type ClonePlanInvalid struct {
	URL    string `json:"url"`
	// Reason ∈ "unreadable" | "private" | "not_youtube".
	Reason string `json:"reason"`
}
type ClonePlanTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	FromRotation bool   `json:"fromRotation"`
}
type ClonePlan struct {
	Accepted      []ClonePlanAccepted  `json:"accepted"`
	Duplicates    []ClonePlanDuplicate `json:"duplicates"`
	Invalid       []ClonePlanInvalid   `json:"invalid"`
	Position      string               `json:"position"`
	ETAStart      *time.Time           `json:"etaStart"`
	PaceLeftToday int                  `json:"paceLeftToday"`
	OverflowDays  int                  `json:"overflowDays"`
	Template      ClonePlanTemplate    `json:"template"`
}
type CloneEnqueueResult struct {
	Items      []CloneQueueItem     `json:"items"`
	Duplicates []ClonePlanDuplicate `json:"duplicates"`
}
