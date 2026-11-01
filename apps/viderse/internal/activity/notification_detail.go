package activity

import "github.com/datnt1998/verse/apps/viderse/internal/insight"

type NotificationChannelDetail struct {
	ChannelID string `json:"channelId"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
}
type NotificationAction struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}
type NotificationMP4Detail struct {
	DurationSec *float64 `json:"durationSec,omitempty"`
	Bytes       *int64   `json:"bytes,omitempty"`
}
type NotificationDetail struct {
	Channels []NotificationChannelDetail `json:"channels,omitempty"`
	Cause    string                      `json:"cause,omitempty"`
	Recovery *NotificationAction         `json:"recovery,omitempty"`
	MP4      *NotificationMP4Detail      `json:"mp4,omitempty"`
	// Reason giải thích việc máy tự làm (action_result): Tự duyệt theo điểm.
	Reason *insight.Reason `json:"reason,omitempty"`
}
