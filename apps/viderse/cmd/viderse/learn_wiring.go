package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/learn"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/performance"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
	stepimages "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	stepmetadata "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/core/llmretry"
	"github.com/datnt1998/verse/platform/events"
)

// learnRuntime ghép Quy tắc đã học và Đề xuất với các service thật. Các trường
// phụ thuộc được gán trong lúc ráp serve, trước khi nhận request.
type learnRuntime struct {
	Learn   *learn.Service
	Suggest *suggest.Service

	Queue    *queue.Service
	Channels *channels.Service
	Autonomy *autonomy.Service
	Cleanup  *serverstorage.Cleanup
	Perf     *performance.Service

	notifySuggestions func(string)
	notifyRules       func(string)

	mu      sync.Mutex
	lastRun time.Time
	running bool
}

func newLearnRuntime(pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub, ai *aiWiring) *learnRuntime {
	rt := &learnRuntime{
		notifySuggestions: web.Notifier(hub, "suggestions"),
		notifyRules:       web.Notifier(hub, "learned.rules"),
	}
	rt.Learn = &learn.Service{Store: postgres.NewLearnStore(pool), Model: func(ctx context.Context) (llmretry.Generator, error) {
		chain, err := ai.Service.ResolveDirector(ctx)
		if err != nil {
			return nil, err
		}
		models := ai.Executors.textModels(ctx, chain)
		if len(models) == 0 {
			return nil, fmt.Errorf("chưa kết nối model Đạo diễn")
		}
		return cloneText{models}, nil
	}}
	rt.Suggest = &suggest.Service{
		Store: postgres.NewSuggestStore(pool),
		Prefs: suggestPrefs{settings: settings},
		Learn: rt.Learn,
		Ops:   suggestOps{rt: rt},
	}
	return rt
}

// Mount gắn endpoint Quy tắc đã học, Đề xuất, hoàn tác.
func (rt *learnRuntime) Mount(g *web.Gateway) {
	web.MountLearn(g, rt.Learn, rt.Suggest, rt.notifyRules, rt.notifySuggestions)
}

// BindPerformance nối nguồn perf_insight khi mảng số liệu sau đăng đã dựng.
func (rt *learnRuntime) BindPerformance(p *performance.Service) {
	rt.Perf = p
	rt.Suggest.Perf = perfSource{svc: p}
}

// BindAutonomy nối nguồn raise_autonomy.
func (rt *learnRuntime) BindAutonomy(a *autonomy.Service) {
	rt.Autonomy = a
	rt.Suggest.Autonomy = autonomySource{svc: a}
}

// BindCleanup nối nguồn và thao tác dọn MP4.
func (rt *learnRuntime) BindCleanup(c *serverstorage.Cleanup) {
	rt.Cleanup = c
	rt.Suggest.Cleanup = cleanupSource{c: c}
}

// Refresh chạy bộ sinh đề xuất ở nền; gọi dày cũng chỉ chạy một lần mỗi 30 giây.
func (rt *learnRuntime) Refresh(ctx context.Context) {
	rt.mu.Lock()
	if rt.running || time.Since(rt.lastRun) < 30*time.Second {
		rt.mu.Unlock()
		return
	}
	rt.running, rt.lastRun = true, time.Now()
	rt.mu.Unlock()
	go func() {
		defer func() { rt.mu.Lock(); rt.running = false; rt.mu.Unlock() }()
		rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := rt.Suggest.Refresh(rctx); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "Đề xuất:", err)
		}
		rt.notifySuggestions("")
	}()
}

// Run làm mới đề xuất định kỳ phòng khi không có lượt quét nào; dừng theo ctx.
func (rt *learnRuntime) Run(ctx context.Context) {
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			rt.Refresh(ctx)
			timer.Reset(15 * time.Minute)
		}
	}
}

// OnRivals bọc notifier "rivals" của vòng quét đối thủ: sau mỗi lượt quét có
// tin mới thì đề xuất được tính lại.
func (rt *learnRuntime) OnRivals(ctx context.Context, next func(string)) func(string) {
	return func(ref string) {
		next(ref)
		rt.Refresh(ctx)
	}
}

// ---- metadata -------------------------------------------------------------

// MetadataRules là quy tắc đã học đang bật của kênh chủ job, cho bước metadata.
func (rt *learnRuntime) MetadataRules(ctx context.Context, job domain.CloneJob) ([]stepmetadata.LearnedRule, error) {
	rules, err := rt.Learn.ChannelRules(ctx, job.ChannelID)
	if err != nil {
		return nil, err
	}
	out := make([]stepmetadata.LearnedRule, len(rules))
	for i, r := range rules {
		out[i] = stepmetadata.LearnedRule{ID: r.ID, Text: r.Text}
	}
	return out, nil
}

// ImageNotes là Ghi chú đã học của prompt cảnh và prompt bìa video đang dùng.
func (rt *learnRuntime) ImageNotes(ctx context.Context, job domain.CloneJob, ov *stepimages.Overrides) (stepimages.LearnedNotes, error) {
	var snapshot library.Snapshot
	if len(job.TemplateSnapshot) > 0 {
		if err := json.Unmarshal(job.TemplateSnapshot, &snapshot); err != nil {
			return stepimages.LearnedNotes{}, nil
		}
	}
	sceneID, coverID := snapshot.ScenePrompt.ID, snapshot.CoverPrompt.ID
	if ov != nil {
		if ov.ScenePromptSet {
			sceneID = ov.ScenePromptID
		}
		if ov.CoverPromptSet {
			coverID = ov.CoverPromptID
		}
	}
	var notes stepimages.LearnedNotes
	var err error
	if sceneID != "" {
		if notes.Scene, err = rt.Learn.Notes(ctx, sceneID); err != nil {
			return notes, err
		}
	}
	if coverID != "" {
		if notes.Cover, err = rt.Learn.Notes(ctx, coverID); err != nil {
			return notes, err
		}
	}
	return notes, nil
}

// metadataLearner nối clonemetadata.Learner (Đạo diễn học từ lần sửa của bạn).
type metadataLearner struct{ rt *learnRuntime }

var _ clonemetadata.Learner = metadataLearner{}

func (m metadataLearner) ObserveEdit(ctx context.Context, jobID, channelID string, baseline, current clonemetadata.Content) (any, error) {
	cands, err := m.rt.Learn.ObserveMetadataEdit(ctx, learn.MetadataEdit{
		JobID: jobID, ChannelID: channelID,
		Baseline: learn.Content{Title: baseline.Title, Description: baseline.Description, Tags: baseline.Tags},
		Current:  learn.Content{Title: current.Title, Description: current.Description, Tags: current.Tags},
	})
	if err != nil {
		return nil, err
	}
	return m.offer(ctx, jobID, learn.AgentDirector, cands)
}
func (m metadataLearner) OpenFor(ctx context.Context, jobID string) (any, error) {
	return m.open(ctx, jobID, learn.AgentDirector)
}
func (m metadataLearner) offer(ctx context.Context, jobID, agent string, cands []learn.Candidate) (any, error) {
	return m.rt.offer(ctx, jobID, agent, cands)
}
func (m metadataLearner) open(ctx context.Context, jobID, agent string) (any, error) {
	return m.rt.open(ctx, jobID, agent)
}

// imagesLearner nối cloneimages.Learner (Hoạ sĩ học từ góp ý vẽ lại).
type imagesLearner struct{ rt *learnRuntime }

func (i imagesLearner) ObserveRedraw(ctx context.Context, jobID, channelID, promptID, promptName, feedback string) (any, error) {
	cands, err := i.rt.Learn.ObserveFeedback(ctx, learn.Feedback{JobID: jobID, ChannelID: channelID, ImagePromptID: promptID, ImagePromptName: promptName, Text: feedback})
	if err != nil {
		return nil, err
	}
	return i.rt.offer(ctx, jobID, learn.AgentArtist, cands)
}
func (i imagesLearner) OpenFor(ctx context.Context, jobID string) (any, error) {
	return i.rt.open(ctx, jobID, learn.AgentArtist)
}

// offer tạo đề xuất learn_rule và trả đề xuất đầu (nil nếu không có) cho trường `learn` của API.
func (rt *learnRuntime) offer(ctx context.Context, jobID, agent string, cands []learn.Candidate) (any, error) {
	if len(cands) == 0 {
		return rt.open(ctx, jobID, agent)
	}
	s, err := rt.Suggest.OfferLearn(ctx, cands)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return rt.open(ctx, jobID, agent)
	}
	rt.notifySuggestions(derefString(s.ChannelID))
	return s, nil
}
func (rt *learnRuntime) open(ctx context.Context, jobID, agent string) (any, error) {
	s, err := rt.Suggest.LearnFor(ctx, jobID, agent)
	if err != nil || s == nil {
		return nil, err
	}
	return s, nil
}
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---- Cài đặt thông báo ------------------------------------------------------

// suggestPrefs đọc/ghi loại đề xuất bị tắt trong cùng khoá cài đặt thông báo.
type suggestPrefs struct{ settings *postgres.SettingsStore }

const notificationPreferencesKey = "notifications.preferences"

func (p suggestPrefs) load(ctx context.Context) (activity.NotificationSettings, error) {
	prefs := activity.DefaultNotificationSettings()
	_, err := p.settings.Get(ctx, notificationPreferencesKey, &prefs)
	if prefs.Suggestions.Off == nil {
		prefs.Suggestions.Off = []activity.SuggestionOff{}
	}
	return prefs, err
}
func (p suggestPrefs) SuggestionOff(ctx context.Context) ([]suggest.Off, error) {
	prefs, err := p.load(ctx)
	out := make([]suggest.Off, len(prefs.Suggestions.Off))
	for i, o := range prefs.Suggestions.Off {
		out[i] = suggest.Off{ChannelID: o.ChannelID, Kind: o.Kind}
	}
	return out, err
}
func (p suggestPrefs) DisableSuggestion(ctx context.Context, kind, channelID string) error {
	prefs, err := p.load(ctx)
	if err != nil {
		return err
	}
	for _, o := range prefs.Suggestions.Off {
		if o.Kind == kind && ((o.ChannelID == nil && channelID == "") || (o.ChannelID != nil && *o.ChannelID == channelID)) {
			return nil
		}
	}
	off := activity.SuggestionOff{Kind: kind}
	if channelID != "" {
		off.ChannelID = &channelID
	}
	prefs.Suggestions.Off = append(prefs.Suggestions.Off, off)
	return p.settings.Set(ctx, notificationPreferencesKey, prefs)
}

// ---- Nguồn tín hiệu -----------------------------------------------------------

type perfSource struct{ svc *performance.Service }

func (p perfSource) OpenInsights(ctx context.Context) ([]suggest.PerfSignal, error) {
	list, err := p.svc.OpenInsights(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]suggest.PerfSignal, len(list))
	for i, s := range list {
		out[i] = suggest.PerfSignal{ChannelID: s.ChannelID, InsightID: s.InsightID, Dimension: s.Dimension, Text: s.Text, Preliminary: s.Preliminary, Videos: s.Videos, ApplyOp: s.ApplyOp}
	}
	return out, nil
}

type autonomySource struct{ svc *autonomy.Service }

func (a autonomySource) RaiseCandidates(ctx context.Context) ([]suggest.RaiseSignal, error) {
	list, err := a.svc.RaiseCandidates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]suggest.RaiseSignal, len(list))
	for i, s := range list {
		out[i] = suggest.RaiseSignal{ChannelID: s.ChannelID, ChannelName: s.ChannelName, Streak: s.Streak, Min: s.Min, CurrentMode: s.CurrentMode}
	}
	return out, nil
}

type cleanupSource struct{ c *serverstorage.Cleanup }

func (c cleanupSource) PostedMP4(ctx context.Context) (int64, int, error) {
	items, err := c.c.ReclaimList(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, it := range items {
		if it.ID == retention.ReclaimMP4 {
			return it.Bytes, it.Count, nil
		}
	}
	return 0, 0, nil
}

// ---- Thao tác ------------------------------------------------------------------

var errLearnNotReady = errors.New("chưa sẵn sàng, thử lại sau ít giây")

// suggestOps chạy thao tác của nút chính bằng đúng service mà UI dùng.
type suggestOps struct{ rt *learnRuntime }

func (o suggestOps) Prioritize(ctx context.Context, _ string, queueID string) error {
	if o.rt.Queue == nil {
		return errLearnNotReady
	}
	_, err := o.rt.Queue.Patch(ctx, domain.QueuePatch{ID: &queueID, Action: "front"})
	return err
}

func watchURL(videoID string) string { return "https://www.youtube.com/watch?v=" + videoID }

func (o suggestOps) PlanImages(ctx context.Context, channelID, videoID string) (*int, error) {
	if o.rt.Queue == nil {
		return nil, errLearnNotReady
	}
	plan, err := o.rt.Queue.Plan(ctx, domain.CloneEnqueueRequest{ChannelID: channelID, URLs: []string{watchURL(videoID)}, Priority: domain.ClonePick})
	if err != nil || len(plan.Accepted) == 0 {
		return nil, err
	}
	return plan.Accepted[0].ImageCalls, nil
}

func (o suggestOps) Pick(ctx context.Context, channelID, videoID string) error {
	if o.rt.Queue == nil {
		return errLearnNotReady
	}
	_, err := o.rt.Queue.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: channelID, URLs: []string{watchURL(videoID)}, Priority: domain.ClonePick})
	return err
}

func (o suggestOps) SetPace(ctx context.Context, channelID string, pace int) (int, error) {
	if o.rt.Channels == nil {
		return 0, errLearnNotReady
	}
	cur, err := o.rt.Channels.Get(ctx, channelID)
	if err != nil {
		return 0, err
	}
	_, err = o.rt.Channels.Update(ctx, channelID, channels.UpdateRequest{PacePerDay: &pace})
	return cur.PacePerDay, err
}

func (o suggestOps) SetApproveScore(ctx context.Context, channelID string, min int) (json.RawMessage, error) {
	if o.rt.Autonomy == nil {
		return nil, errLearnNotReady
	}
	prev, err := o.rt.Autonomy.SetApprove(ctx, channelID, domain.ApproveScore, min)
	if err != nil {
		return nil, err
	}
	return json.Marshal(prev)
}

func (o suggestOps) RestoreAutonomy(ctx context.Context, channelID string, previous json.RawMessage) error {
	if o.rt.Autonomy == nil {
		return errLearnNotReady
	}
	var prev domain.Autonomy
	if err := json.Unmarshal(previous, &prev); err != nil {
		return err
	}
	return o.rt.Autonomy.Restore(ctx, channelID, prev)
}

func (o suggestOps) ReclaimMP4(ctx context.Context) (int64, error) {
	if o.rt.Cleanup == nil {
		return 0, errLearnNotReady
	}
	item, err := o.rt.Cleanup.Reclaim(ctx, retention.ReclaimMP4)
	return item.Bytes, err
}

func (o suggestOps) RemoveQueue(ctx context.Context, ids []string) error {
	if o.rt.Queue == nil {
		return errLearnNotReady
	}
	for _, id := range ids {
		id := id
		if _, err := o.rt.Queue.Patch(ctx, domain.QueuePatch{ID: &id, Action: "remove"}); err != nil {
			return err
		}
	}
	return nil
}
