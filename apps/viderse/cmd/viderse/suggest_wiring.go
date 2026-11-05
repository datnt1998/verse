package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/platform/events"
)

// suggestRuntime ghép Đề xuất với các service thật. Các trường phụ thuộc được
// gán trong lúc ráp serve, trước khi nhận request.
type suggestRuntime struct {
	Suggest *suggest.Service

	Queue   *queue.Service
	Cleanup *serverstorage.Cleanup

	notifySuggestions func(string)

	mu      sync.Mutex
	lastRun time.Time
	running bool
}

func newSuggestRuntime(pool *pgxpool.Pool, settings *postgres.SettingsStore, hub *events.Hub) *suggestRuntime {
	rt := &suggestRuntime{notifySuggestions: web.Notifier(hub, "suggestions")}
	rt.Suggest = &suggest.Service{
		Store: postgres.NewSuggestStore(pool),
		Prefs: suggestPrefs{settings: settings},
		Ops:   suggestOps{rt: rt},
	}
	return rt
}

// Mount gắn endpoint Đề xuất.
func (rt *suggestRuntime) Mount(g *web.Gateway) {
	web.MountSuggestions(g, rt.Suggest, rt.notifySuggestions)
}

// BindCleanup nối nguồn và thao tác dọn MP4.
func (rt *suggestRuntime) BindCleanup(c *serverstorage.Cleanup) {
	rt.Cleanup = c
	rt.Suggest.Cleanup = cleanupSource{c: c}
}

// Refresh chạy bộ sinh đề xuất ở nền; gọi dày cũng chỉ chạy một lần mỗi 30 giây.
func (rt *suggestRuntime) Refresh(ctx context.Context) {
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
func (rt *suggestRuntime) Run(ctx context.Context) {
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
func (rt *suggestRuntime) OnRivals(ctx context.Context, next func(string)) func(string) {
	return func(ref string) {
		next(ref)
		rt.Refresh(ctx)
	}
}

// ---- Cài đặt thông báo ------------------------------------------------------

// suggestPrefs đọc công tắc chung của Đề xuất trong khoá cài đặt thông báo.
type suggestPrefs struct{ settings *postgres.SettingsStore }

const notificationPreferencesKey = "notifications.preferences"

func (p suggestPrefs) SuggestionsEnabled(ctx context.Context) (bool, error) {
	prefs := activity.DefaultNotificationSettings()
	if _, err := p.settings.Get(ctx, notificationPreferencesKey, &prefs); err != nil {
		return false, err
	}
	return prefs.Suggestions, nil
}

// ---- Nguồn tín hiệu -----------------------------------------------------------

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

var errSuggestNotReady = errors.New("chưa sẵn sàng, thử lại sau ít giây")

// suggestOps chạy thao tác của nút chính bằng đúng service mà UI dùng.
type suggestOps struct{ rt *suggestRuntime }

func (o suggestOps) Prioritize(ctx context.Context, _ string, queueID string) error {
	if o.rt.Queue == nil {
		return errSuggestNotReady
	}
	_, err := o.rt.Queue.Patch(ctx, domain.QueuePatch{ID: &queueID, Action: "front"})
	return err
}

func watchURL(videoID string) string { return "https://www.youtube.com/watch?v=" + videoID }

func (o suggestOps) PlanImages(ctx context.Context, channelID, videoID string) (*int, error) {
	if o.rt.Queue == nil {
		return nil, errSuggestNotReady
	}
	plan, err := o.rt.Queue.Plan(ctx, domain.CloneEnqueueRequest{ChannelID: channelID, URLs: []string{watchURL(videoID)}})
	if err != nil || len(plan.Accepted) == 0 {
		return nil, err
	}
	return plan.Accepted[0].ImageCalls, nil
}

func (o suggestOps) Pick(ctx context.Context, channelID, videoID string) error {
	if o.rt.Queue == nil {
		return errSuggestNotReady
	}
	_, err := o.rt.Queue.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: channelID, URLs: []string{watchURL(videoID)}})
	return err
}

func (o suggestOps) ReclaimMP4(ctx context.Context) (int64, error) {
	if o.rt.Cleanup == nil {
		return 0, errSuggestNotReady
	}
	item, err := o.rt.Cleanup.Reclaim(ctx, retention.ReclaimMP4)
	return item.Bytes, err
}
