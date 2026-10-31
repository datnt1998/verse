package insight

import (
	"fmt"
	"strings"
	"time"
)

type Need struct {
	JobID string `json:"jobId"`
	Label string `json:"label"`
	State string `json:"state"`
}
type Channel struct {
	ChannelID string `json:"channelId"`
	Done      int    `json:"done"`
	Cap       int    `json:"cap"`
	Waiting   int    `json:"waiting"`
	Ready     int    `json:"ready"`
}
type AutoAction struct {
	Kind   string  `json:"kind"`
	Text   string  `json:"text"`
	Count  int     `json:"count"`
	Href   string  `json:"href"`
	Reason *Reason `json:"reason,omitempty"`
	// Undo có khi việc tự làm hoàn tác được; client gửi lại nguyên op + params tới POST /api/undo.
	Undo *Undo `json:"undo,omitempty"`
}

// Undo là một thao tác hoàn tác trong danh sách cho phép của POST /api/undo.
type Undo struct {
	Op     string         `json:"op"`
	Params map[string]any `json:"params"`
}
type Today struct {
	Queued       int     `json:"queued"`
	ExpectedDone *int    `json:"expectedDone"`
	ByTime       *string `json:"byTime"`
	ImagesLeft   *int    `json:"imagesLeft"`
	ImagesTotal  *int    `json:"imagesTotal"`
}
type Recap struct {
	From        time.Time    `json:"from"`
	To          time.Time    `json:"to"`
	Summary     string       `json:"summary"`
	NeedsYou    []Need       `json:"needsYou"`
	Auto        []AutoAction `json:"auto"`
	Suggestions []any        `json:"suggestions"`
	Today       Today        `json:"today"`
	Channels    []Channel    `json:"channels"`
	HasNew      bool         `json:"hasNew"`
}

func (r *Recap) Summarize() {
	review, attention := 0, 0
	for _, n := range r.NeedsYou {
		switch n.State {
		case "review":
			review++
		case "attention":
			attention++
		}
	}
	var parts []string
	if review > 0 {
		parts = append(parts, fmt.Sprintf("%d video chờ duyệt", review))
	}
	if attention > 0 {
		parts = append(parts, fmt.Sprintf("%d cần xử lý", attention))
	}
	if len(parts) == 0 {
		r.Summary = "Không có video chờ bạn."
		return
	}
	r.Summary = "Trong lúc bạn vắng: " + strings.Join(parts, ", ") + "."
}
