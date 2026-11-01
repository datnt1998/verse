package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	imagesmodel "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmretry"
)

// ── mô hình giả ──────────────────────────────────────────────────────────────

// scriptedModel trả JSON dựng sẵn theo câu người dùng (đọc từ payload) và ghi
// lại mọi tin nhắn để kiểm câu người dùng chỉ nằm trong payload.
type scriptedModel struct {
	mu       sync.Mutex
	answer   func(call int, text string) string
	err      error
	messages [][]llm.Message
}

func (m *scriptedModel) Generate(_ context.Context, msgs []llm.Message, _ []llm.ToolSpec, _ ...llm.CallOption) (*llm.LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs)
	if m.err != nil {
		return nil, m.err
	}
	var payload classifyPayload
	last := msgs[len(msgs)-1]
	if err := json.Unmarshal([]byte(msgs[1].TextContent()), &payload); err != nil && last.Role == llm.RoleUser && len(msgs) == 2 {
		return nil, fmt.Errorf("payload không phải JSON: %w", err)
	}
	out := m.answer(len(m.messages), payload.UserText)
	return &llm.LLMResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(out)}}}, nil
}

func (m *scriptedModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// planJSON dựng đầu ra hợp lệ của mô hình từ các bước (đủ mọi trường, null).
func planJSON(steps ...map[string]any) string {
	full := make([]map[string]any, 0, len(steps))
	for _, s := range steps {
		f := map[string]any{"intent": nil, "sceneNo": nil, "feedback": nil, "parts": nil, "count": nil, "minDurationMinutes": nil,
			"rival": nil, "channel": nil, "preset": nil, "rule": nil, "pick": nil, "videoTitle": nil, "drawSource": nil}
		for k, v := range s {
			f[k] = v
		}
		full = append(full, f)
	}
	raw, _ := json.Marshal(map[string]any{"outcome": "steps", "steps": full, "ask": nil, "unsupportedKind": nil})
	return string(raw)
}

func unsupportedJSON(kind string) string {
	raw, _ := json.Marshal(map[string]any{"outcome": "unsupported", "steps": []any{}, "ask": nil, "unsupportedKind": kind})
	return string(raw)
}

func askJSON(q string, opts ...string) string {
	raw, _ := json.Marshal(map[string]any{"outcome": "ask", "steps": []any{}, "ask": map[string]any{"question": q, "options": opts}, "unsupportedKind": nil})
	return string(raw)
}

// ── service giả: ghi lại mọi thao tác ghi ────────────────────────────────────

type log struct {
	mu    sync.Mutex
	lines []string
}

func (l *log) add(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *log) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

func (l *log) has(prefix string) bool {
	for _, line := range l.all() {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type world struct {
	log    *log
	status map[string]string // trạng thái ảnh theo id
	paused bool
	chans  []channels.ChannelView
	queue  []domain.CloneQueueItemView
	videos map[string][]domain.RivalVideo
	// Trạng thái video cho ba lệnh /provider, /export-anyway, /posted.
	activeStep  domain.CloneStep
	gateAllowed bool
	posted      bool
	connected   map[string]bool // nguồn vẽ đã kết nối
	drawSource  domain.ModelRef // nguồn vẽ riêng đang gắn với video
}

const (
	jobID  = "11111111-1111-4111-8111-111111111111"
	chanA  = "aaaaaaaa-1111-4111-8111-111111111111"
	chanB  = "bbbbbbbb-1111-4111-8111-111111111111"
	rivalN = "UCnocturne"
	rivalL = "UClatearchive"
)

func newWorld() *world {
	w := &world{log: &log{}, status: map[string]string{}, videos: map[string][]domain.RivalVideo{},
		activeStep: domain.CloneImages, connected: map[string]bool{"chatgpt": true, "grok": true, "gemini": false}}
	score := func(v float64) *float64 { return &v }
	w.chans = []channels.ChannelView{
		{Channel: domain.Channel{ID: chanA, Name: "Midnight Tales", PacePerDay: 2}, Sources: []domain.ChannelSource{
			{RivalID: rivalN, Title: "Nocturne Stories"}, {RivalID: rivalL, Title: "Late Archive"}}},
		{Channel: domain.Channel{ID: chanB, Name: "Quiet Hours", PacePerDay: 1}},
	}
	w.videos[rivalN] = []domain.RivalVideo{
		{ID: "vidN0000001", ChannelID: rivalN, Title: "The Station That Shut Down in 1962", Score: score(4.2)},
		{ID: "vidN0000002", ChannelID: rivalN, Title: "What I Saw Through the Night Bus Window", Score: score(2.8)},
		{ID: "vidN0000003", ChannelID: rivalN, Title: "The Lake House Guestbook", Score: score(2.7)},
	}
	w.videos[rivalL] = []domain.RivalVideo{{ID: "vidL0000001", ChannelID: rivalL, Title: "Archive Tape 17", Score: score(1.9)}}
	w.queue = []domain.CloneQueueItemView{
		{ID: "q1", ChannelID: chanA, Title: "Archive Tape 17", SourceURL: "https://youtu.be/vidL0000001"},
		{ID: "q2", ChannelID: chanA, Title: "The Station That Shut Down in 1962", SourceURL: "https://youtu.be/vidN0000001"},
	}
	return w
}

type fJobs struct{ w *world }

func (f fJobs) Get(_ context.Context, id string) (*domain.CloneJobView, error) {
	if id != jobID {
		return nil, errors.New("không có")
	}
	step := f.w.activeStep
	v := &domain.CloneJobView{CloneJob: domain.CloneJob{ID: id, ChannelID: chanA, ActiveStep: &step,
		Source: domain.CloneSourceSnapshot{Title: "The Hospital Night Shift"}}}
	if f.w.paused {
		now := time.Now()
		v.PausedAt = &now
	}
	if f.w.posted {
		now := time.Now()
		v.PostedAt = &now
	}
	return v, nil
}
func (f fJobs) Pause(_ context.Context, id string) (*domain.CloneJob, error) {
	f.w.log.add("jobs.pause %s", id)
	f.w.paused = true
	return nil, nil
}
func (f fJobs) Resume(_ context.Context, id string) (*domain.CloneJob, error) {
	f.w.log.add("jobs.resume %s", id)
	f.w.paused = false
	return nil, nil
}

type fImages struct{ w *world }

func (f fImages) view() cloneimages.View {
	v := cloneimages.View{Manifest: &cloneimages.ManifestView{Revision: 7}}
	if f.w.drawSource.Provider != "" {
		v.Manifest.Overrides = &imagesmodel.Overrides{DrawProvider: f.w.drawSource.Provider, DrawModel: f.w.drawSource.Model}
	}
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("scene-%02d", i)
		st := f.w.status[id]
		if st == "" {
			st = "review"
		}
		v.Items = append(v.Items, cloneimages.ItemView{ID: id, Kind: "scene", SceneID: id, Status: st})
	}
	v.Items = append(v.Items, cloneimages.ItemView{ID: "thumb", Kind: "thumbnail", Status: "review"})
	return v
}
func (f fImages) Read(context.Context, string) (cloneimages.View, error) { return f.view(), nil }
func (f fImages) Review(_ context.Context, id string, in cloneimages.ReviewInput) (cloneimages.ReviewResult, error) {
	for _, a := range in.Actions {
		f.w.log.add("images.review %s %s rev=%d", a.Op, a.ID, *in.ExpectedRevision)
		if a.Op == "skip" {
			f.w.status[a.ID] = "skipped"
		}
	}
	return cloneimages.ReviewResult{}, nil
}
func (f fImages) Redraw(_ context.Context, id, item string, in cloneimages.RedrawInput) (cloneimages.GenerateResult, error) {
	prompt := ""
	if in.Prompt != nil {
		prompt = *in.Prompt
	}
	f.w.log.add("images.redraw %s %s rev=%d prompt=%s", id, item, *in.ExpectedRevision, prompt)
	f.w.status[item] = "pending"
	return cloneimages.GenerateResult{Accepted: true, JobID: id, ItemID: item}, nil
}
func (f fImages) RedrawAll(_ context.Context, id string, in cloneimages.ResetInput) (cloneimages.GenerateResult, error) {
	f.w.log.add("images.redrawAll %s confirm=%d feedback=%s", id, in.ConfirmCount, in.Feedback)
	return cloneimages.GenerateResult{Accepted: true, JobID: id}, nil
}
func (f fImages) PlanOverrides(context.Context, string, cloneimages.OverrideInput) (cloneimages.OverridePlan, error) {
	return cloneimages.OverridePlan{}, nil
}
func (f fImages) ApplyOverrides(_ context.Context, id string, in cloneimages.OverrideInput) (cloneimages.OverridePlan, error) {
	f.w.log.add("images.noImages %s %v", id, *in.NoImages)
	return cloneimages.OverridePlan{}, nil
}
func (f fImages) SetDrawSource(_ context.Context, id, provider, modelID string) error {
	f.w.log.add("images.drawSource %s %s %s", id, provider, modelID)
	f.w.drawSource = domain.ModelRef{Provider: provider, Model: modelID}
	return nil
}

// fDrawSources chỉ coi nguồn có trong world.connected là đã kết nối.
type fDrawSources struct{ w *world }

func (f fDrawSources) Ref(_ context.Context, source string) (domain.ModelRef, bool, error) {
	if !f.w.connected[source] {
		return domain.ModelRef{}, false, nil
	}
	return domain.ModelRef{Provider: "prov-" + source, Model: "model-" + source}, true, nil
}

// fQuality là cổng Độ hoàn thiện; gateAllowed=true nghĩa là điểm đã đạt.
type fQuality struct{ w *world }

func (f fQuality) Gate(context.Context, string) (quality.GateDecision, error) {
	return quality.GateDecision{Allowed: f.w.gateAllowed}, nil
}
func (f fQuality) ExportAnyway(_ context.Context, id string) error {
	f.w.log.add("quality.exportAnyway %s", id)
	return nil
}

// fPosted ghi lại việc đánh dấu đã đăng kèm link người dùng dán.
type fPosted struct{ w *world }

func (f fPosted) Post(_ context.Context, id, link string) (string, error) {
	f.w.log.add("posted %s %q", id, link)
	f.w.posted = true
	return "https://youtu.be/dQw4w9WgXcQ", nil
}

type fBackdrops struct{ w *world }

func (f fBackdrops) Edit(_ context.Context, id string, in clonebackdrop.EditRequest) (clonebackdrop.Result, error) {
	f.w.log.add("backdrop.edit %s %s", id, in.Op)
	return clonebackdrop.Result{Accepted: true}, nil
}

type fMetadata struct{ w *world }

func (f fMetadata) Read(context.Context, string) (clonemetadata.View, error) {
	return clonemetadata.View{Revision: 3}, nil
}
func (f fMetadata) Rewrite(_ context.Context, id string, in clonemetadata.Rewrite) (clonemetadata.View, error) {
	f.w.log.add("metadata.rewrite %s %s dir=%s rev=%d", id, strings.Join(in.Parts, ","), in.Direction, *in.ExpectedRevision)
	return clonemetadata.View{}, nil
}

type fChannels struct{ w *world }

func (f fChannels) List(context.Context) ([]channels.ChannelView, error) { return f.w.chans, nil }
func (f fChannels) Get(_ context.Context, id string) (channels.ChannelView, error) {
	for _, c := range f.w.chans {
		if c.Channel.ID == id {
			return c, nil
		}
	}
	return channels.ChannelView{}, &channels.Error{Code: channels.CodeNotFound, Message: "Kênh không tồn tại."}
}
func (f fChannels) Update(_ context.Context, id string, in channels.UpdateRequest) (channels.ChannelView, error) {
	f.w.log.add("channels.update %s pace=%d", id, *in.PacePerDay)
	return f.Get(context.Background(), id)
}
func (f fChannels) SetSourceMinDuration(_ context.Context, id, rival string, sec *int) (channels.ChannelView, error) {
	f.w.log.add("channels.minDuration %s %s %d", id, rival, *sec)
	return f.Get(context.Background(), id)
}

type fQueue struct{ w *world }

func (f fQueue) List(context.Context) (domain.CloneQueueView, error) {
	limit := int64(400)
	return domain.CloneQueueView{Items: f.w.queue, ImageQuota: []domain.QueueImageQuota{{Provider: "grok", Used: 210, Limit: &limit, Period: "week", Role: "fallback"}}}, nil
}
func (f fQueue) Plan(_ context.Context, in domain.CloneEnqueueRequest) (domain.ClonePlan, error) {
	out := domain.ClonePlan{Position: "front"}
	for _, u := range in.URLs {
		id := strings.TrimPrefix(u, "https://youtu.be/")
		queued := false
		for _, q := range f.w.queue {
			if strings.Contains(q.SourceURL, id) {
				queued = true
			}
		}
		if queued {
			out.Duplicates = append(out.Duplicates, domain.ClonePlanDuplicate{URL: u, State: "queued"})
			continue
		}
		est := 7
		var title string
		var outlier *float64
		for _, vs := range f.w.videos {
			for _, v := range vs {
				if v.ID == id {
					title, outlier = v.Title, v.Score
				}
			}
		}
		out.Accepted = append(out.Accepted, domain.ClonePlanAccepted{URL: u, VideoID: id, Title: title, Outlier: outlier, ImageEstimate: &est})
	}
	return out, nil
}
func (f fQueue) Enqueue(ctx context.Context, in domain.CloneEnqueueRequest) (domain.CloneEnqueueResult, error) {
	plan, _ := f.Plan(ctx, in)
	out := domain.CloneEnqueueResult{Items: []domain.CloneQueueItem{}}
	for _, a := range plan.Accepted {
		f.w.log.add("queue.enqueue %s %s", in.ChannelID, a.VideoID)
		out.Items = append(out.Items, domain.CloneQueueItem{SourceVideoID: a.VideoID})
	}
	return out, nil
}
func (f fQueue) Patch(_ context.Context, in domain.QueuePatch) (domain.CloneQueueView, error) {
	f.w.log.add("queue.patch %s %s", *in.ID, in.Action)
	return domain.CloneQueueView{}, nil
}

type fRivals struct{ w *world }

func (f fRivals) Videos(_ context.Context, id string, in rivals.VideoQuery) (*rivals.VideoPage, error) {
	f.w.log.add("rivals.videos %s sort=%s days=%d q=%s", id, in.Sort, in.WithinDays, in.Query)
	return &rivals.VideoPage{Videos: f.w.videos[id]}, nil
}

type fAutonomy struct{ w *world }

func (f fAutonomy) ApplyPreset(_ context.Context, id, preset string) error {
	f.w.log.add("autonomy.preset %s %s", id, preset)
	return nil
}

type fRules struct{ w *world }

func (f fRules) AddRule(_ context.Context, id, text, source string) (string, error) {
	f.w.log.add("rules.add %s %s [%s]", id, text, source)
	return "rule-1", nil
}

type taskEvent struct {
	op, phase string
	state     domain.TaskState
	done      int64
	total     int64
	title     string
}

type fTasks struct {
	mu     sync.Mutex
	events []taskEvent
}

func (f *fTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, taskEvent{op: "start", title: spec.Title, phase: spec.Phase, total: spec.Progress.Total})
	return "task-1", nil
}
func (f *fTasks) Update(_ context.Context, _ string, u domain.TaskUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := taskEvent{op: "update", state: u.State}
	if u.Phase != nil {
		e.phase = *u.Phase
	}
	if u.Progress != nil {
		e.done, e.total = u.Progress.Done, u.Progress.Total
	}
	f.events = append(f.events, e)
	return nil
}
func (f *fTasks) Finish(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, taskEvent{op: "finish"})
	return nil
}
func (f *fTasks) snapshot() []taskEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]taskEvent(nil), f.events...)
}

type rig struct {
	svc   *Service
	w     *world
	model *scriptedModel
	tasks *fTasks
	now   time.Time
}

func newRig(t *testing.T, answer func(call int, text string) string) *rig {
	t.Helper()
	w := newWorld()
	model := &scriptedModel{answer: answer}
	tasks := &fTasks{}
	r := &rig{w: w, model: model, tasks: tasks, now: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}
	r.svc = New(Deps{
		Director: func(context.Context) (llmretry.Generator, error) { return model, nil },
		Jobs:     fJobs{w}, Images: fImages{w}, Backdrops: fBackdrops{w}, Metadata: fMetadata{w}, Channels: fChannels{w},
		Queue: fQueue{w}, Rivals: fRivals{w}, Autonomy: fAutonomy{w}, Rules: fRules{w}, Tasks: tasks,
		DrawSources: fDrawSources{w}, Quality: fQuality{w}, Posted: fPosted{w},
		ImagesLeft: func(context.Context) (int, error) { return 7, nil },
	})
	r.svc.now = func() time.Time { return r.now }
	t.Cleanup(r.svc.Close)
	return r
}

// waitRun đợi lượt chạy nền xong (tối đa 5 giây).
func (r *rig) waitRun(t *testing.T, id string) RunStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, ok := r.svc.RunStatus(id); ok && st.State != "running" {
			return st
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("lượt chạy %s không kết thúc", id)
	return RunStatus{}
}
