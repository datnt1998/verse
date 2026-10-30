package web

import (
	"context"
	"net/http"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// Activity là phần activity.Service mà HTTP cần — khai ở phía dùng.
type Activity interface {
	Snapshot(ctx context.Context) (activity.Snapshot, error)
	MarkRead(ctx context.Context, ids []string) error
	Errors(ctx context.Context) ([]activity.ErrorItem, error)
	ErrorsCount(ctx context.Context) (int, error)
}

// MountActivity đăng ký các route của Việc nền và thông báo (§11).
func MountActivity(g *Gateway, svc Activity) {
	g.Handle(http.MethodGet, "/api/activity", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		snap, err := svc.Snapshot(r.Context())
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, activityToWire(snap))
	})
	g.Handle(http.MethodPost, "/api/notifications/read", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		var req struct {
			IDs []string `json:"ids"`
			All bool     `json:"all"`
		}
		if !webapp.DecodeJSON(w, r, &req) {
			return
		}
		if !req.All && len(req.IDs) == 0 {
			webapp.Error(w, http.StatusBadRequest, "cần ids hoặc all")
			return
		}
		ids := req.IDs
		if req.All {
			ids = nil // nil → store coi là "tất cả"
		}
		if err := svc.MarkRead(r.Context(), ids); err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	g.Handle(http.MethodGet, "/api/errors", func(w http.ResponseWriter, r *http.Request, _ identity.User) {
		items, err := svc.Errors(r.Context())
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		count, err := svc.ErrorsCount(r.Context())
		if err != nil {
			webapp.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		webapp.WriteJSON(w, http.StatusOK, errorsToWire(items, count))
	})
}

// activityWire là hình của GET /api/activity theo §11.
type activityWire struct {
	Running   []taskWire         `json:"running"`
	VideoJobs int                `json:"videoJobs"`
	Queued    int                `json:"queued"`
	Unread    []notificationWire `json:"unread"`
}

type taskWire struct {
	ID       string        `json:"id"`
	Kind     string        `json:"kind"`
	Agent    string        `json:"agent,omitempty"`
	Title    string        `json:"title"`
	Object   objectWire    `json:"objectRef"`
	Phase    string        `json:"phase"`
	Progress *progressWire `json:"progress"`
	ETASec   *int          `json:"etaSec"`
	State    string        `json:"state"`
	Action   actionWire    `json:"action"`
}

type objectWire struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type progressWire struct {
	Done   int64       `json:"done"`
	Total  int64       `json:"total"`
	Groups []groupWire `json:"groups,omitempty"`
}

type groupWire struct {
	Group string `json:"group"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

type actionWire struct {
	Label string `json:"label"`
	Href  string `json:"href,omitempty"`
	Op    string `json:"op,omitempty"`
}

type notificationWire struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Count       int     `json:"count"`
	Text        string  `json:"text"`
	Href        string  `json:"href"`
	ActionLabel string  `json:"actionLabel"`
	CreatedAt   string  `json:"createdAt"`
	ReadAt      *string `json:"readAt"`
}

func activityToWire(snap activity.Snapshot) activityWire {
	out := activityWire{
		Running:   make([]taskWire, 0, len(snap.Tasks)),
		VideoJobs: snap.VideoJobs,
		Queued:    snap.Queued,
		Unread:    make([]notificationWire, 0, len(snap.Unread)),
	}
	for _, t := range snap.Tasks {
		out.Running = append(out.Running, taskToWire(t))
	}
	for _, n := range snap.Unread {
		out.Unread = append(out.Unread, notificationToWire(n))
	}
	return out
}

func taskToWire(t activity.Task) taskWire {
	tw := taskWire{
		ID: t.ID, Kind: t.Kind, Agent: t.Agent, Title: t.Title,
		Object: objectWire{Type: t.Object.Type, ID: t.Object.ID, Label: t.Object.Label},
		Phase:  t.Phase, ETASec: t.ETASec, State: t.State,
		Action: actionWire{Label: t.Action.Label, Href: t.Action.Href, Op: t.Action.Op},
	}
	if t.Progress != nil {
		pw := &progressWire{Done: t.Progress.Done, Total: t.Progress.Total}
		for _, g := range t.Progress.Groups {
			pw.Groups = append(pw.Groups, groupWire{Group: g.Group, Done: g.Done, Total: g.Total})
		}
		tw.Progress = pw
	}
	return tw
}

func notificationToWire(n activity.Notification) notificationWire {
	nw := notificationWire{
		ID: n.ID, Kind: n.Kind, Count: n.Count, Text: n.Text,
		Href: n.Href, ActionLabel: n.ActionLabel,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
	}
	if n.ReadAt != nil {
		at := n.ReadAt.UTC().Format(time.RFC3339)
		nw.ReadAt = &at
	}
	return nw
}

// errorWire là một mục "Lỗi gần đây" đã render câu chữ tiếng Việt.
type errorWire struct {
	ID     string      `json:"id"`
	Time   string      `json:"time"`
	What   string      `json:"what"`
	Why    string      `json:"why"`
	Status string      `json:"status"`
	Tone   string      `json:"tone"`
	Href   string      `json:"href,omitempty"`
	Link   string      `json:"link,omitempty"`
	Op     string      `json:"op,omitempty"`
	Object *objectWire `json:"object,omitempty"`
}

type errorsWire struct {
	Items   []errorWire `json:"items"`
	Count7d int         `json:"count7d"`
}

func errorsToWire(items []activity.ErrorItem, count int) errorsWire {
	now := time.Now()
	out := errorsWire{Items: make([]errorWire, 0, len(items)), Count7d: count}
	for _, it := range items {
		tone := "warn"
		if it.Kind != "task_failed" && it.Status != "Cần xử lý" {
			tone = "muted"
		}
		ew := errorWire{
			ID: it.ID, Time: shortTime(it.At, now), What: it.What, Why: it.Why,
			Status: it.Status, Tone: tone, Href: it.Href, Link: it.Link,
			Op: it.Action.Op,
		}
		if it.Object != nil {
			ew.Object = &objectWire{Type: it.Object.Type, ID: it.Object.ID, Label: it.Object.Label}
		}
		out.Items = append(out.Items, ew)
	}
	return out
}

// shortTime render mốc thời gian của mục lỗi: hôm nay "15:04", hôm qua
// "Hôm qua", còn lại "02/01".
func shortTime(at, now time.Time) string {
	at = at.Local()
	now = now.Local()
	y1, m1, d1 := at.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return at.Format("15:04")
	}
	yesterday := now.AddDate(0, 0, -1)
	if y, m, d := yesterday.Date(); y == y1 && m == m1 && d == d1 {
		return "Hôm qua"
	}
	return at.Format("02/01")
}
