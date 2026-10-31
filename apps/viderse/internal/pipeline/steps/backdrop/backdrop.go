package backdrop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	catalogue "github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/blobstore"
)

type Store interface {
	Channel(context.Context, string) (domain.BackdropChannel, bool, error)
	ChannelVideos(context.Context, string) ([]domain.BackdropChannelVideo, error)
	ChannelVideo(context.Context, string, string) (domain.BackdropChannelVideo, bool, error)
	Video(context.Context, string) (domain.BackdropVideo, bool, error)
	MarkChannelVideoUnavailable(context.Context, string, string, string, time.Time) error
}
type Deps struct {
	Store          Store
	RefreshChannel func(context.Context, string, time.Duration) error
	ProbeVideo     func(context.Context, string, string) (domain.BackdropChannelVideo, error)
	FetchClip      func(context.Context, *pipeline.StepContext, Item) (media.Artifact, error)
	PlatformPause  func(context.Context, domain.PlatformID) (*time.Time, *string, error)
	Now            func() time.Time
}
type Handler struct{ d Deps }

func New(d Deps) *Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Handler{d: d}
}

// ProductionDeps keeps catalogue operations in the existing service. Each
// download receives the attempt's process runner rather than a detached worker.
func ProductionDeps(store Store, service *catalogue.Service) Deps {
	return Deps{Store: store, Now: service.Now, PlatformPause: service.PlatformPause, ProbeVideo: service.ProbeChannelVideo,
		RefreshChannel: func(ctx context.Context, id string, age time.Duration) error {
			row, found, err := store.Channel(ctx, id)
			if err != nil || !found {
				return err
			}
			if row.ListedAt != nil && service.Now().Sub(*row.ListedAt) < age {
				return nil
			}
			_, err = service.RefreshChannel(ctx, id, true)
			return err
		},
		FetchClip: func(ctx context.Context, s *pipeline.StepContext, item Item) (media.Artifact, error) {
			return service.EnsureClip(ctx, item.URL, domain.BackdropSegment{FromSec: float64(item.StartSec), ToSec: float64(item.EndSec)}, filepath.Join(s.StagingDir, "clips"), stepRunner{s})
		}}
}

type stepRunner struct{ s *pipeline.StepContext }

func (r stepRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return r.s.RunProcess(ctx, file, args, spec)
}

type Template struct {
	Backdrop      *domain.BackdropRecipe
	FixedSec      float64
	PlaybackSpeed float64
}

func ResolveTemplate(ctx context.Context, raw json.RawMessage, store Store) (Template, error) {
	t := Template{PlaybackSpeed: 1}
	if len(raw) == 0 || string(raw) == "null" {
		return t, nil
	}
	var recipe domain.Recipe
	if err := json.Unmarshal(raw, &recipe); err != nil {
		return t, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Snapshot template không đọc được"}
	}
	speed := recipe.BackgroundAppearance.Speed
	if speed == 0 {
		speed = 1
	}
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < domain.LimitAppearanceSpeedMin || speed > domain.LimitAppearanceSpeedMax {
		return t, requestError("Tốc độ nền không hợp lệ")
	}
	t.PlaybackSpeed = speed
	b := recipe.Backdrop
	if b == nil {
		return t, nil
	}
	if b.ClipSec < 10 || b.ClipSec > 600 || b.ClipCount != nil && (*b.ClipCount < 0 || *b.ClipCount > MaxItems) {
		return t, requestError("Cấu hình clip nền không hợp lệ")
	}
	for _, link := range b.Channels {
		if link.ChannelID == "" || len(link.Kinds) == 0 {
			return t, requestError("Kênh nền không hợp lệ")
		}
		for _, kind := range link.Kinds {
			if kind != "clip" && kind != "vod" && kind != "highlight" && kind != "video" {
				return t, requestError("Loại video nền không hợp lệ")
			}
		}
	}
	for _, id := range b.FixedVideoIDs {
		row, found, err := store.Video(ctx, id)
		if err != nil {
			return t, err
		}
		if found {
			t.FixedSec += row.Segment.Duration() / speed
		}
	}
	if len(b.Channels) > 0 {
		t.Backdrop = b
	}
	return t, nil
}
func DownloadedVideoSec(data map[string]any) *float64 {
	var v float64
	switch x := data["durationSec"].(type) {
	case float64:
		v = x
	case float32:
		v = float64(x)
	case int:
		v = float64(x)
	case int64:
		v = float64(x)
	case json.Number:
		v, _ = x.Float64()
	}
	if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return nil
	}
	return &v
}
func InitialManifest(t Template, videoSec *float64) (Manifest, string) {
	if t.Backdrop == nil {
		return Empty(0, nil), ""
	}
	if t.Backdrop.ClipCount != nil {
		return Empty(*t.Backdrop.ClipCount, nil), ""
	}
	if videoSec == nil {
		return Empty(10, nil), "Chưa biết độ dài video — lấy 10 clip nền"
	}
	target := int(math.Max(0, math.Ceil((*videoSec-t.FixedSec)*t.PlaybackSpeed)))
	return Empty(0, &target), ""
}
func PlatformOf(id string) domain.PlatformID {
	if strings.HasPrefix(id, "bilibili:") {
		return domain.PlatformBilibili
	}
	return domain.PlatformTwitch
}
func requestError(message string) error {
	return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: message}
}
func failure(err error) ItemError {
	code := "failed"
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) {
		code = coded.ErrorCode()
	}
	return ItemError{Code: code, Message: err.Error()}
}
func jobSeed(id string) uint32 {
	h := uint32(0x811c9dc5)
	for _, v := range utf16.Encode([]rune(id)) {
		h = (h ^ uint32(v)) * 0x01000193
	}
	return h
}

type runState struct {
	h        *Handler
	jobID    string
	skipped  map[domain.PlatformID]bool
	warnings []string
}

func (r *runState) skip(p domain.PlatformID) {
	if r.skipped[p] {
		return
	}
	r.skipped[p] = true
	label := "Twitch"
	if p == domain.PlatformBilibili {
		label = "Bilibili"
	}
	r.warnings = append(r.warnings, label+" đang tạm chặn yêu cầu — bỏ qua các kênh này lần này")
}
func (r *runState) refresh(ctx context.Context, b *domain.BackdropRecipe) error {
	for _, link := range b.Channels {
		if err := ctx.Err(); err != nil {
			return err
		}
		p := PlatformOf(link.ChannelID)
		if r.skipped[p] {
			continue
		}
		row, found, err := r.h.d.Store.Channel(ctx, link.ChannelID)
		if err != nil {
			return err
		}
		if !found {
			r.warnings = append(r.warnings, "Kênh nền "+strings.TrimPrefix(link.ChannelID, string(p)+":")+" không còn trong thư viện")
			continue
		}
		age := 24 * time.Hour
		if p == domain.PlatformBilibili {
			age = 6 * time.Hour
		} else {
			for _, k := range link.Kinds {
				if k == "vod" {
					age = time.Hour
				}
			}
		}
		if err := r.h.d.RefreshChannel(ctx, link.ChannelID, age); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			f := failure(err)
			if f.Code == "rate_limited" {
				r.skip(p)
			}
			label := strings.TrimPrefix(link.ChannelID, string(p)+":")
			if row.Title != nil {
				label = *row.Title
			}
			r.warnings = append(r.warnings, fmt.Sprintf("Không làm mới được kênh %s (%s) — dùng danh sách đã lưu", label, f.Message))
		}
	}
	return nil
}
func (r *runState) pick(ctx context.Context, b *domain.BackdropRecipe, items []Item, salt uint32, count int, excluded map[string]bool) ([]Item, uint32, error) {
	candidates := map[string][]catalogue.Candidate{}
	channels := make([]domain.RecipeBackdropChannel, 0, len(b.Channels))
	for _, link := range b.Channels {
		channels = append(channels, domain.RecipeBackdropChannel{ChannelID: link.ChannelID, Kinds: link.Kinds, Pinned: link.Pinned})
		rows, err := r.h.d.Store.ChannelVideos(ctx, link.ChannelID)
		if err != nil {
			return nil, salt, err
		}
		for _, v := range rows {
			title := v.Title
			candidates[link.ChannelID] = append(candidates[link.ChannelID], catalogue.Candidate{ChannelID: v.ChannelID, VideoID: v.VideoID, Kind: v.Kind, URL: v.URL, Title: &title, DurationSec: v.DurationSec, Unavailable: v.UnavailableAt != nil})
		}
	}
	used := map[string]bool{}
	cuts := map[string]bool{}
	avoid := map[string]bool{}
	for k, v := range excluded {
		avoid[k] = v
	}
	for _, v := range items {
		used[v.VideoID] = true
		cuts[fmt.Sprintf("%s@%d", v.VideoID, v.StartSec)] = true
	}
	out := []Item{}
	budget := count * 3
	for len(out) < count && budget > 0 {
		if err := ctx.Err(); err != nil {
			return nil, salt, err
		}
		skipped := map[string]bool{}
		for _, link := range b.Channels {
			skipped[link.ChannelID] = r.skipped[PlatformOf(link.ChannelID)]
		}
		seed := jobSeed(r.jobID) ^ (salt * 0x9e3779b1)
		picks := catalogue.PickBackdropSources(catalogue.PickRequest{Channels: channels, Candidates: candidates, Seed: seed, Count: count - len(out), UsedVideoIDs: used, ExcludedVideoIDs: avoid, SkippedChannelIDs: skipped, ClipSec: float64(b.ClipSec)})
		if len(picks) == 0 {
			break
		}
		salt++
		for i, p := range picks {
			if budget <= 0 {
				break
			}
			budget--
			if r.skipped[PlatformOf(p.ChannelID)] {
				continue
			}
			cut := p.Cut
			if cut == nil {
				video, err := r.h.d.ProbeVideo(ctx, p.ChannelID, p.VideoID)
				if err != nil {
					if ctx.Err() != nil {
						return nil, salt, ctx.Err()
					}
					if failure(err).Code == "rate_limited" {
						r.skip(PlatformOf(p.ChannelID))
					}
					avoid[p.VideoID] = true
					continue
				}
				if video.DurationSec != nil {
					cut = catalogue.CutRange(*video.DurationSec, float64(b.ClipSec), catalogue.Mulberry32(seed^(uint32(i+1)*0x85ebca6b)))
					title := video.Title
					p.Title = &title
				}
			}
			if cut == nil {
				avoid[p.VideoID] = true
				continue
			}
			start, end := int(math.Floor(cut.StartSec)), int(math.Floor(cut.EndSec))
			key := fmt.Sprintf("%s@%d", p.VideoID, start)
			if end-start < 10 || cuts[key] {
				continue
			}
			cuts[key] = true
			used[p.VideoID] = true
			all := append(append([]Item{}, items...), out...)
			out = append(out, Item{ID: NextItemID(all), ChannelID: p.ChannelID, VideoID: p.VideoID, Kind: p.Kind, URL: p.URL, Title: p.Title, StartSec: start, EndSec: end, Source: "random", Status: "pending"})
		}
	}
	return out, salt, nil
}
func (r *runState) pickGoal(ctx context.Context, b *domain.BackdropRecipe, items []Item, salt uint32, count int, target *int, excluded map[string]bool) ([]Item, uint32, error) {
	if target == nil {
		return r.pick(ctx, b, items, salt, max(0, count), excluded)
	}
	all := append([]Item{}, items...)
	for PlannedSec(all) < *target && len(all) < MaxItems {
		batch := min(MaxItems-len(all), int(math.Ceil(float64(*target-PlannedSec(all))/float64(b.ClipSec))))
		fresh, next, err := r.pick(ctx, b, all, salt, batch, excluded)
		salt = next
		if err != nil {
			return nil, salt, err
		}
		if len(fresh) == 0 {
			break
		}
		all = append(all, fresh...)
	}
	return all[len(items):], salt, nil
}
func artifactMediaID(jobID string, a media.Artifact) (string, error) {
	f, err := os.Open(a.File)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return "clone/" + jobID + "/" + hex.EncodeToString(h.Sum(nil)) + "/" + a.Path, nil
}
func (h *Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	ctx = catalogue.WithRunner(ctx, stepRunner{s})
	t, err := ResolveTemplate(ctx, s.Job.TemplateSnapshot, h.d.Store)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	m, note := InitialManifest(t, DownloadedVideoSec(s.Upstream[domain.CloneDownload].Data))
	artifacts := []media.Artifact{}
	if len(s.Checkpoint) > 0 && string(s.Checkpoint) != "null" {
		c, err := ParseCheckpoint(s.Checkpoint)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		m = c.Manifest
		artifacts = c.Artifacts
		note = ""
	}
	r := &runState{h: h, jobID: s.Job.ID, skipped: map[domain.PlatformID]bool{}}
	if note != "" {
		r.warnings = append(r.warnings, note)
	}
	// Preserve the unknown-duration note, but recalculate transient warnings.
	for _, w := range m.Warnings {
		if strings.HasPrefix(w, "Chưa biết độ dài video") {
			r.warnings = append(r.warnings, w)
		}
	}
	save := func() error {
		m.Warnings = ShortfallWarnings(r.warnings, m.Items, m.WantedCount, m.TargetSec)
		m = Seal(m)
		return s.SaveCheckpoint(ctx, Checkpoint{Manifest: m, Artifacts: artifacts})
	}
	pause := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.PauseRequested() {
			if err := save(); err != nil {
				return err
			}
			return pipeline.ErrPaused
		}
		return nil
	}
	if err := pause(); err != nil {
		return pipeline.StepResult{}, err
	}
	progress := func(done int, total *int, phase string) error {
		if s.ReportProgress == nil {
			return nil
		}
		var n *int64
		if total != nil {
			v := int64(*total)
			n = &v
		}
		return s.Progress(ctx, domain.CloneProgress{UnitsDone: int64(done), UnitsTotal: n, Phase: phase})
	}
	if t.Backdrop != nil {
		if err := progress(0, nil, "refresh"); err != nil {
			return pipeline.StepResult{}, err
		}
		for _, link := range t.Backdrop.Channels {
			until, _, err := h.d.PlatformPause(ctx, PlatformOf(link.ChannelID))
			if err != nil {
				return pipeline.StepResult{}, err
			}
			if until != nil && until.After(h.d.Now()) {
				r.skip(PlatformOf(link.ChannelID))
			}
		}
		if err := r.refresh(ctx, t.Backdrop); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	staged := map[string]bool{}
	artifactIDs := map[string]string{}
	keptArtifacts := []media.Artifact{}
	for _, a := range artifacts {
		if stat, err := os.Stat(a.File); err == nil && stat.Size() > 0 {
			id, err := artifactMediaID(s.Job.ID, a)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			staged[id] = true
			artifactIDs[a.File] = id
			keptArtifacts = append(keptArtifacts, a)
		} else if err != nil && !os.IsNotExist(err) {
			return pipeline.StepResult{}, err
		}
	}
	artifacts = keptArtifacts
	kept := []Item{}
	for _, item := range m.Items {
		if item.Status == "failed" && item.Source == "random" {
			continue
		}
		ready := false
		if item.Status == "ready" && item.MediaID != nil {
			ready = staged[*item.MediaID]
			if !ready {
				handle, err := s.Acquire(ctx, *item.MediaID)
				if err == nil {
					handle.Release()
					ready = true
				} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, blobstore.ErrNotFound) {
					var coded interface{ ErrorCode() string }
					if !errors.As(err, &coded) || coded.ErrorCode() != "not_found" {
						return pipeline.StepResult{}, err
					}
				}
			}
		}
		if !ready {
			item.Status = "pending"
			item.MediaID = nil
			item.Error = nil
		}
		kept = append(kept, item)
	}
	m.Items = kept
	if t.Backdrop != nil {
		if err := progress(0, nil, "pick"); err != nil {
			return pipeline.StepResult{}, err
		}
		fresh, salt, err := r.pickGoal(ctx, t.Backdrop, m.Items, m.Salt, m.WantedCount-len(m.Items), m.TargetSec, nil)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		m.Salt = salt
		m.Items = append(m.Items, fresh...)
	}
	if m.TargetSec != nil {
		m.WantedCount = len(m.Items)
	}
	m.Fallback = "none"
	if err := save(); err != nil {
		return pipeline.StepResult{}, err
	}
	todo := []int{}
	for i, item := range m.Items {
		if item.Status == "pending" {
			todo = append(todo, i)
		}
	}
	total := len(todo)
	for done, index := range todo {
		if err := pause(); err != nil {
			return pipeline.StepResult{}, err
		}
		if err := progress(done, &total, "fetch"); err != nil {
			return pipeline.StepResult{}, err
		}
		item := m.Items[index]
		tried := map[string]bool{}
		for attempt := 1; attempt <= 3; attempt++ {
			p := PlatformOf(item.ChannelID)
			if r.skipped[p] {
				item.Status = "failed"
				item.Error = &ItemError{Code: "rate_limited", Message: "Nền tảng đang tạm chặn yêu cầu"}
				break
			}
			tried[item.VideoID] = true
			a, err := h.d.FetchClip(ctx, s, item)
			if err == nil {
				id, idErr := artifactMediaID(s.Job.ID, a)
				if idErr != nil {
					return pipeline.StepResult{}, idErr
				}
				item.Status = "ready"
				item.MediaID = &id
				item.Error = nil
				artifactIDs[a.File] = id
				artifacts = append(artifacts, a)
				break
			}
			if ctx.Err() != nil {
				return pipeline.StepResult{}, ctx.Err()
			}
			f := failure(err)
			item.Status = "failed"
			item.Error = &f
			if f.Code == "rate_limited" {
				r.skip(p)
			}
			if f.Code == "unavailable" || f.Code == "login_required" {
				if err := h.d.Store.MarkChannelVideoUnavailable(ctx, item.ChannelID, item.VideoID, f.Code, h.d.Now()); err != nil {
					return pipeline.StepResult{}, err
				}
			}
			if f.Code == "rate_limited" || item.Source == "owner" || attempt == 3 || t.Backdrop == nil {
				break
			}
			others := append([]Item{}, m.Items[:index]...)
			others = append(others, m.Items[index+1:]...)
			fresh, salt, err := r.pick(ctx, t.Backdrop, others, m.Salt, 1, tried)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			m.Salt = salt
			if len(fresh) == 0 {
				break
			}
			id := item.ID
			item = fresh[0]
			item.ID = id
			// Persist the replacement before any network request for its window.
			m.Items[index] = item
			if err := save(); err != nil {
				return pipeline.StepResult{}, err
			}
		}
		m.Items[index] = item
		if err := save(); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	ready := ReadyItems(m)
	wants := m.WantedCount > 0
	if m.TargetSec != nil {
		wants = *m.TargetSec > 0
	}
	var waiting *domain.CloneError
	if len(ready) == 0 && wants {
		if t.FixedSec > 0 {
			m.Fallback = "template-clips"
			r.warnings = append(r.warnings, "Kênh nền lỗi — dùng clip cố định của template")
		} else {
			at := h.d.Now().Add(30 * time.Minute)
			provider := false
			if t.Backdrop != nil {
				for _, link := range t.Backdrop.Channels {
					until, _, err := h.d.PlatformPause(ctx, PlatformOf(link.ChannelID))
					if err != nil {
						return pipeline.StepResult{}, err
					}
					if until != nil && until.After(h.d.Now()) && (!provider || until.Before(at)) {
						at = *until
						provider = true
					}
				}
			}
			source := domain.RetryAtWorker
			if provider {
				source = domain.RetryAtProvider
			}
			recovery := "Kiểm tra kênh nền trong thư viện hoặc thêm clip cố định vào template, rồi bấm Thử lại"
			waiting = &domain.CloneError{Code: domain.CloneErrQuota, Message: "Chưa lấy được clip nền từ kênh", Recovery: &recovery, RetryAt: &at, RetryAtSource: source}
		}
	}
	if err := save(); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := pause(); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := progress(total, &total, "fetch"); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := os.MkdirAll(s.StagingDir, 0o755); err != nil {
		return pipeline.StepResult{}, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	file := filepath.Join(s.StagingDir, "backdrop.json")
	if err := os.WriteFile(file, b, 0o644); err != nil {
		return pipeline.StepResult{}, err
	}
	// Drop files replaced during re-picks; only final ready clips are published.
	live := map[string]bool{}
	for _, v := range ready {
		live[*v.MediaID] = true
	}
	final := []media.Artifact{}
	for _, a := range artifacts {
		if live[artifactIDs[a.File]] {
			final = append(final, a)
		}
	}
	final = append(final, media.Artifact{Path: ManifestFile, File: file, ContentType: "application/json", Filename: "backdrop.json"})
	return pipeline.StepResult{Artifacts: final, Data: StepData(m), Waiting: waiting}, nil
}
