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
}
type ClonePlanDuplicate struct {
	URL   string `json:"url"`
	JobID string `json:"jobId"`
	State string `json:"state"`
}
type ClonePlanInvalid struct {
	URL    string `json:"url"`
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
