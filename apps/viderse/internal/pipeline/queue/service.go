package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pausedAtKey = "clone.queuePausedAt"
const queueLock int64 = 27900501
const discoveryLock int64 = 27900502

type Settings interface {
	Get(context.Context, string, any) (bool, error)
	Set(context.Context, string, any) error
	QueuePaused(context.Context) (bool, error)
}

type Options struct {
	Pool         *pgxpool.Pool
	Store        Store
	Runner       pipeline.JobRunner
	Settings     Settings
	Now          func() time.Time
	Scan         func(context.Context, string) error
	Probe        func(context.Context, string) (VideoMetadata, error)
	ImageSources func(context.Context) ([]domain.QueueImageQuota, error)
	Notify       func(string)
	Diag         func(error)
}

type Service struct {
	d      Options
	store  Store
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	wg     sync.WaitGroup
}

func New(d Options) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d: d, store: d.Store, wake: make(chan struct{}, 1)}
}
func (s *Service) SetRunner(r pipeline.JobRunner) { s.d.Runner = r }
func (s *Service) emit() {
	if s.d.Notify != nil {
		s.d.Notify(domain.CloneEventQueue)
	}
}
func (s *Service) diag(err error) {
	if err != nil && s.d.Diag != nil {
		s.d.Diag(err)
	}
}
func invalid(message string) error {
	return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: message}
}
func notFound(message string) error {
	return &domain.CloneError{Code: domain.CloneErrNotFound, Message: message}
}

// Rules are read at acquisition time; malformed stored numbers use defaults.
type Rules struct {
	Workers, ImageWorkers, RescanHours int
	Paused                             bool
	PausedAt                           *time.Time
}

func (s *Service) Rules(ctx context.Context) (Rules, error) {
	out := Rules{Workers: domain.JobConcurrencyDefault, ImageWorkers: domain.ImageConcurrencyDefault, RescanHours: domain.RescanHoursDefault}
	for _, v := range []struct {
		key    string
		target *int
		lo, hi int
	}{{domain.KeyJobConcurrency, &out.Workers, 1, 6}, {domain.KeyImageConcurrency, &out.ImageWorkers, 1, 8}, {domain.KeyRescanHours, &out.RescanHours, 1, 24}} {
		var raw json.RawMessage
		found, err := s.d.Settings.Get(ctx, v.key, &raw)
		if err != nil {
			return out, err
		}
		if found {
			*v.target = ruleNumber(raw, *v.target, v.lo, v.hi)
		}
	}
	var err error
	out.Paused, err = s.d.Settings.QueuePaused(ctx)
	if err != nil {
		return out, err
	}
	if out.Paused {
		var at time.Time
		found, e := s.d.Settings.Get(ctx, pausedAtKey, &at)
		if e != nil {
			return out, e
		}
		if found && !at.IsZero() {
			out.PausedAt = &at
		}
	}
	return out, nil
}

func (s *Service) Patch(ctx context.Context, in domain.QueuePatch) (domain.CloneQueueView, error) {
	// Validate the entire request before any side effect.
	if in.ID != nil {
		var id pgtype.UUID
		if err := id.Scan(*in.ID); err != nil || !id.Valid {
			return domain.CloneQueueView{}, invalid("Mục hàng chờ không hợp lệ")
		}
	}
	if in.Position != nil && (math.IsNaN(*in.Position) || math.IsInf(*in.Position, 0)) {
		return domain.CloneQueueView{}, invalid("Vị trí hàng chờ không hợp lệ")
	}
	if in.Action != "" && in.Action != "remove" && in.Action != "restore" && in.Action != "front" {
		return domain.CloneQueueView{}, invalid("Thao tác hàng chờ không hợp lệ")
	}
	if (in.Position != nil || in.Action != "") && in.ID == nil {
		return domain.CloneQueueView{}, invalid("Thiếu mục hàng chờ")
	}
	if in.Paused != nil {
		// Keep user intent separate from storage safety holds.
		if err := s.d.Settings.Set(ctx, domain.KeyQueuePaused, *in.Paused); err != nil {
			return domain.CloneQueueView{}, err
		}
		var at any
		if *in.Paused {
			at = s.d.Now()
		}
		if err := s.d.Settings.Set(ctx, pausedAtKey, at); err != nil {
			return domain.CloneQueueView{}, err
		}
	}
	for _, v := range []struct {
		key    string
		n      *int
		lo, hi int
	}{{domain.KeyJobConcurrency, in.Workers, 1, 6}, {domain.KeyImageConcurrency, in.ImageWorkers, 1, 8}, {domain.KeyRescanHours, in.RescanHours, 1, 24}} {
		if v.n != nil {
			n := max(v.lo, min(v.hi, *v.n))
			if err := s.d.Settings.Set(ctx, v.key, n); err != nil {
				return domain.CloneQueueView{}, err
			}
		}
	}
	var removed *domain.QueueRemovedReceipt
	if in.ID != nil {
		tx, err := s.d.Pool.Begin(ctx)
		if err != nil {
			return domain.CloneQueueView{}, err
		}
		defer tx.Rollback(ctx)
		repo := s.store.With(tx)
		item, err := repo.GetQueueItem(ctx, *in.ID)
		if err != nil {
			return domain.CloneQueueView{}, err
		}
		if item == nil || item.State != domain.QueueQueued || item.JobID != nil {
			if in.Action == "restore" {
				return domain.CloneQueueView{}, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục đã bị dọn hoặc video đã chạy, không thể khôi phục"}
			}
			return domain.CloneQueueView{}, notFound("Mục hàng chờ không còn tồn tại")
		}
		if err = repo.LockOwnerChannel(ctx, item.ChannelID); err != nil {
			return domain.CloneQueueView{}, err
		}
		// Other queue edits serialize on the channel. Re-read after taking the
		// lock so the remove receipt captures the latest position and priority.
		item, err = repo.GetQueueItem(ctx, *in.ID)
		if err != nil {
			return domain.CloneQueueView{}, err
		}
		if item == nil || item.State != domain.QueueQueued || item.JobID != nil || (item.RemovedAt != nil && in.Action != "restore") {
			return domain.CloneQueueView{}, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Mục hàng chờ đã thay đổi"}
		}
		if in.Action == "remove" || in.Action == "restore" {
			var changed bool
			if in.Action == "remove" {
				changed, err = repo.RemoveQueueItem(ctx, item.ID, s.d.Now())
				removed = &domain.QueueRemovedReceipt{ID: item.ID, Position: item.Position, Priority: item.Priority}
			} else {
				changed, err = repo.RestoreQueueItem(ctx, item.ID)
			}
			if err == nil && !changed {
				return domain.CloneQueueView{}, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video đã chạy hoặc đã được xếp lại, không thể đổi hàng chờ"}
			}
		} else if in.Action == "front" {
			queued, e := repo.ListQueuedItems(ctx)
			if e != nil {
				return domain.CloneQueueView{}, e
			}
			pos := float64(s.d.Now().UnixNano()) / 1e9
			for _, q := range queued {
				if q.Priority == domain.ClonePick && q.Position <= pos {
					pos = q.Position - 1
				}
			}
			err = repo.PromotePick(ctx, item.ID, pos)
		} else if in.Position != nil {
			err = repo.UpdateQueuePosition(ctx, item.ID, *in.Position)
		}
		if err != nil {
			return domain.CloneQueueView{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.CloneQueueView{}, err
		}
	}
	s.emit()
	s.RequestDrain()
	if s.d.Runner != nil {
		s.d.Runner.Wake()
	}
	view, err := s.List(ctx)
	if err != nil {
		return domain.CloneQueueView{}, err
	}
	view.Removed = removed
	return view, nil
}

func (s *Service) loadLanes(ctx context.Context) ([]Lane, map[string]domain.CloneQueueItem, map[string]domain.Channel, map[string][]domain.ChannelTemplate, error) {
	items, err := s.store.ListQueuedItems(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	owners, err := s.store.Owners(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	starts, err := s.store.LaneLastStarts(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	used, err := s.store.PaceUsedToday(ctx, LocalMidnight(s.d.Now()))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	byOwner := map[string]domain.Channel{}
	templates := map[string][]domain.ChannelTemplate{}
	laneMap := map[string]*Lane{}
	for _, o := range owners {
		byOwner[o.ID] = o
		t, e := s.store.Templates(ctx, o.ID)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		templates[o.ID] = t
		l := &Lane{ChannelID: o.ID, Pace: o.PacePerDay, UsedToday: int(used[o.ID]), TemplateOK: len(t) > 0}
		if l.Pace < 1 {
			l.Pace = DefaultPace
		}
		if last := starts[o.ID]; last.LastStart != nil {
			l.LastStart = *last.LastStart
			if last.LastSequence != nil {
				l.LastSequence = last.LastSequence.UnixNano()
			}
		}
		laneMap[o.ID] = l
	}
	byID := map[string]domain.CloneQueueItem{}
	for _, item := range items {
		l := laneMap[item.ChannelID]
		if l == nil {
			continue
		}
		byID[item.ID] = item
		l.Items = append(l.Items, itemForSchedule(item))
	}
	lanes := make([]Lane, 0, len(laneMap))
	for _, l := range laneMap {
		if len(l.Items) > 0 {
			lanes = append(lanes, *l)
		}
	}
	sort.Slice(lanes, func(i, j int) bool { return laneOrder(lanes[i], lanes[j]) < 0 })
	return lanes, byID, byOwner, templates, nil
}
func itemForSchedule(item domain.CloneQueueItem) Item {
	seconds := int64(0)
	if item.Source.DurationSec != nil {
		seconds = int64(*item.Source.DurationSec)
	}
	return Item{ID: item.ID, ChannelID: item.ChannelID, Priority: string(item.Priority), Position: item.Position, DurationSec: seconds, CreatedAt: item.CreatedAt}
}
func (s *Service) basis(ctx context.Context) (Basis, error) {
	rows, err := s.store.ListRecentStepDurations(ctx)
	if err != nil {
		return Basis{}, err
	}
	samples := make([]Sample, len(rows))
	for i, r := range rows {
		samples[i] = Sample{Step: r.Step, Elapsed: r.Elapsed, SourceSeconds: r.SourceSeconds, SceneCount: r.SceneCount, FinishedAt: r.FinishedAt}
	}
	return ObservedBasis(samples), nil
}

// Enqueue resolves once, then writes every accepted video to the common queue.
// Pick is a priority, never a second start-now execution path.
func (s *Service) Enqueue(ctx context.Context, in domain.CloneEnqueueRequest) (domain.CloneEnqueueResult, error) {
	return s.enqueue(ctx, in, "")
}
func (s *Service) enqueue(ctx context.Context, in domain.CloneEnqueueRequest, origin domain.CloneOrigin) (domain.CloneEnqueueResult, error) {
	plan, meta, err := s.plan(ctx, in)
	if err != nil {
		return domain.CloneEnqueueResult{}, err
	}
	priority := in.Priority
	if priority == "" {
		priority = domain.CloneBatch
		if len(in.URLs) == 1 {
			priority = domain.ClonePick
		}
	}
	out := domain.CloneEnqueueResult{Items: []domain.CloneQueueItem{}, Duplicates: plan.Duplicates}
	if len(plan.Invalid) > 0 {
		return out, invalid("Có link video không đọc được")
	}
	tx, err := s.d.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	repo := s.store.With(tx)
	if err = repo.LockOwnerChannel(ctx, in.ChannelID); err != nil {
		return out, err
	}
	lane, err := repo.ListLaneItems(ctx, in.ChannelID)
	if err != nil {
		return out, err
	}
	if len(lane)+len(plan.Accepted) > LaneCap {
		return out, invalid("Hàng chờ của kênh đã đầy")
	}
	keys := make([]string, len(plan.Accepted))
	for i, a := range plan.Accepted {
		keys[i] = domain.CloneSourceKey(in.ChannelID, a.VideoID)
	}
	existingJobs, err := repo.FindJobsBySourceKeys(ctx, keys)
	if err != nil {
		return out, err
	}
	existingQueue, err := repo.FindQueueRowsBySourceKeys(ctx, keys)
	if err != nil {
		return out, err
	}
	seen := map[string]bool{}
	for _, j := range existingJobs {
		seen[j.SourceKey] = true
		id := j.ID
		out.Duplicates = append(out.Duplicates, domain.ClonePlanDuplicate{URL: j.Source.URL, JobID: &id, State: j.StageGroup, Title: j.Source.Title})
	}
	for _, q := range existingQueue {
		seen[q.SourceKey] = true
		out.Duplicates = append(out.Duplicates, domain.ClonePlanDuplicate{URL: q.Source.URL, JobID: q.JobID, State: string(q.State), Title: q.Source.Title})
	}
	position := 0.0
	if priority == domain.ClonePick {
		position = float64(s.d.Now().UnixNano()) / 1e9
		queued, e := repo.ListQueuedItems(ctx)
		if e != nil {
			return out, e
		}
		for _, q := range queued {
			if q.Priority == domain.ClonePick && q.Position > position {
				position = q.Position
			}
		}
	} else {
		for _, q := range lane {
			if q.Priority == domain.CloneBatch && q.Position > position {
				position = q.Position
			}
		}
	}
	for _, a := range plan.Accepted {
		key := domain.CloneSourceKey(in.ChannelID, a.VideoID)
		if seen[key] {
			continue
		}
		seen[key] = true
		position++
		source := meta[a.VideoID].Source
		item := domain.CloneQueueItem{ID: newID(), ChannelID: in.ChannelID, SourceVideoID: a.VideoID, SourceKey: key, Source: source, Priority: priority, Origin: domain.OriginBulk, Position: position, State: domain.QueueQueued, NoImages: in.NoImages, OverrideRevision: 1, CreatedAt: s.d.Now()}
		if priority == domain.ClonePick {
			item.Origin = domain.OriginManual
		}
		if origin != "" {
			item.Origin = origin
		}
		item.TemplateID = in.TemplateID
		if err = repo.InsertQueueItem(ctx, item); err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	s.emit()
	s.RequestDrain()
	return out, nil
}

// Drain is serialized by a session-scoped PostgreSQL advisory lock. The
// runner owns the atomic item->job transaction and independently rechecks it.
func (s *Service) Drain(ctx context.Context) (int, error) {
	conn, err := pgx.ConnectConfig(ctx, s.d.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())
	lockRepo := s.store.With(conn)
	held, err := lockRepo.TryAdvisoryLock(ctx, queueLock)
	if err != nil || !held {
		return 0, err
	}
	defer lockRepo.UnlockAdvisory(context.WithoutCancel(ctx), queueLock)
	// Use the existing periodic drain sweep even while paused or fully busy.
	s.diag(s.store.PruneQueueHistory(ctx, s.d.Now()))
	rules, err := s.Rules(ctx)
	if err != nil || rules.Paused {
		return 0, err
	}
	if s.d.Runner == nil {
		return 0, errors.New("chưa có worker cho hàng chờ")
	}
	s.d.Runner.Wake()
	used, err := s.store.CountActiveJobs(ctx)
	if err != nil {
		return 0, err
	}
	free := rules.Workers - used
	if free <= 0 {
		return 0, nil
	}
	lanes, byID, owners, _, err := s.loadLanes(ctx)
	if err != nil {
		return 0, err
	}
	started := 0
	for _, item := range Order(lanes, OrderWorkset) {
		if started >= free {
			break
		}
		owner := owners[item.ChannelID]
		row := byID[item.ID]
		var lane *Lane
		for i := range lanes {
			if lanes[i].ChannelID == item.ChannelID {
				lane = &lanes[i]
				break
			}
		}
		if lane == nil || !lane.TemplateOK {
			continue
		}
		if row.Priority == domain.CloneBatch && row.Origin == domain.OriginAuto && !owner.AutoPick {
			continue
		}
		if row.Priority == domain.CloneBatch && lane.UsedToday >= lane.Pace {
			continue
		}
		// Pause is live and may have changed while a template was snapshotted.
		paused, e := s.d.Settings.QueuePaused(ctx)
		if e != nil {
			return started, e
		}
		if paused {
			break
		}
		job, e := s.d.Runner.Start(ctx, row)
		if e != nil {
			var ce *domain.CloneError
			if errors.As(e, &ce) && (ce.Code == domain.CloneErrNotFound || ce.Code == domain.CloneErrConflict || ce.Code == domain.CloneErrSourceUnavailable) {
				continue
			}
			return started, e
		}
		if job != nil {
			started++
			if row.Priority == domain.CloneBatch {
				lane.UsedToday++
			}
		}
	}
	if started > 0 {
		s.emit()
		s.d.Runner.Wake()
	}
	return started, nil
}

func newID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
