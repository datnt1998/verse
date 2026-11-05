package web

import (
	"context"
	"hash/fnv"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

type channelSummaryStats interface {
	SummaryStats(context.Context, time.Time, time.Time) (map[string][2]int, map[string][2]string, error)
	SummaryGroups(context.Context) (map[string][2]int, error)
}

type summaryIssue struct {
	TemplateID string `json:"templateId"`
	Text       string `json:"text"`
}
type channelSummary struct {
	ChannelID string  `json:"channelId"`
	Name      string  `json:"name"`
	Initials  string  `json:"initials"`
	Color     string  `json:"color"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
	Today     struct {
		Done int `json:"done"`
		Cap  int `json:"cap"`
	} `json:"today"`
	WaitingYou     int           `json:"waitingYou"`
	Ready          int           `json:"ready"`
	PostedThisWeek int           `json:"postedThisWeek"`
	AutoFetch      bool          `json:"autoFetch"`
	TemplateIssue  *summaryIssue `json:"templateIssue,omitempty"`
}

// MountChannelSummary exposes board channel rows through the authenticated gateway.
func MountChannelSummary(g *Gateway, channels ChannelService, stats channelSummaryStats) {
	g.Handle(http.MethodGet, "/api/channels/summary", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		w.Header().Set("Cache-Control", "no-store")
		views, err := channels.List(r.Context())
		if err != nil {
			writeChannelError(w, err, nil)
			return
		}
		groups, err := stats.SummaryGroups(r.Context())
		if err != nil {
			writeChannelError(w, err, nil)
			return
		}
		now := time.Now()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		week := day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
		counts, issues, err := stats.SummaryStats(r.Context(), day, week)
		if err != nil {
			webapp.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Không tải được tổng quan kênh."})
			return
		}
		out := make([]channelSummary, 0, len(views))
		for _, view := range views {
			c := view.Channel
			v := channelSummary{ChannelID: c.ID, Name: c.Name, Initials: summaryInitials(c.Name), Color: summaryColor(c.ID), AutoFetch: c.AutoFetch, PostedThisWeek: counts[c.ID][1]}
			v.Today.Done, v.Today.Cap = counts[c.ID][0], c.PerDay
			if c.AvatarPath != nil {
				url := brandingURL(c.ID, *c.AvatarPath)
				v.AvatarURL = &url
			}
			if issue, ok := issues[c.ID]; ok {
				v.TemplateIssue = &summaryIssue{TemplateID: issue[0], Text: "Template " + issue[1] + " thiếu clip nền"}
			}
			v.WaitingYou, v.Ready = groups[c.ID][0], groups[c.ID][1]
			out = append(out, v)
		}
		webapp.WriteJSON(w, http.StatusOK, out)
	})
}

func summaryInitials(name string) string {
	var letters []rune
	for _, word := range strings.Fields(name) {
		for _, r := range word {
			letters = append(letters, unicode.ToUpper(r))
			break
		}
		if len(letters) == 2 {
			break
		}
	}
	return string(letters)
}
func summaryColor(id string) string {
	colors := [...]string{"#2E2236", "#302418", "#1F2C27"}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return colors[h.Sum32()%uint32(len(colors))]
}
