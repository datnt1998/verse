package domain

import "time"

type CloneRestartPlan struct {
	Template      ClonePlanTemplate `json:"template"`
	ImageEstimate *int              `json:"imageEstimate"`
	Position      string            `json:"position"`
	ETAStart      *time.Time        `json:"etaStart"`
}
