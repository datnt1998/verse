package queue

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

type VideoMetadata struct {
	Source      domain.CloneSourceSnapshot
	Outlier     *float64
	ViewCount   *int64
	PublishedAt *time.Time
	HasCaptions bool
}

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseVideoURL accepts only actual YouTube video URLs (not channel, playlist,
// userinfo or arbitrary network targets). The probe receives only the video ID.
func ParseVideoURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", "not_youtube"
	}
	host := strings.ToLower(u.Hostname())
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed") {
			id = parts[1]
		}
	default:
		return "", "not_youtube"
	}
	if !videoIDPattern.MatchString(id) {
		return "", "unreadable"
	}
	return id, ""
}
func watchURL(id string) string { return "https://www.youtube.com/watch?v=" + id }

func (s *Service) Plan(ctx context.Context, in domain.CloneEnqueueRequest) (domain.ClonePlan, error) {
	p, _, err := s.plan(ctx, in)
	return p, err
}
func (s *Service) plan(ctx context.Context, in domain.CloneEnqueueRequest) (domain.ClonePlan, map[string]VideoMetadata, error) {
	out := domain.ClonePlan{Accepted: []domain.ClonePlanAccepted{}, Duplicates: []domain.ClonePlanDuplicate{}, Invalid: []domain.ClonePlanInvalid{}, Position: "back"}
	meta := map[string]VideoMetadata{}
	if len(in.URLs) == 0 || len(in.URLs) > 200 {
		return out, meta, invalid("Cần từ 1 đến 200 link video")
	}
	priority := in.Priority
	if priority == "" {
		priority = domain.CloneBatch
		if len(in.URLs) == 1 {
			priority = domain.ClonePick
		}
	}
	if priority != domain.CloneBatch && priority != domain.ClonePick {
		return out, meta, invalid("Mức ưu tiên không hợp lệ")
	}
	if priority == domain.ClonePick {
		out.Position = "front"
	}
	owner, err := s.store.Owner(ctx, in.ChannelID)
	if err != nil {
		return out, meta, err
	}
	if owner == nil {
		return out, meta, notFound("Kênh không tồn tại")
	}
	templates, err := s.store.Templates(ctx, in.ChannelID)
	if err != nil {
		return out, meta, err
	}
	if in.TemplateID != nil {
		for _, t := range templates {
			if t.ID == *in.TemplateID {
				out.Template = domain.ClonePlanTemplate{ID: t.ID, Name: t.Name}
				break
			}
		}
		if out.Template.ID == "" {
			return out, meta, invalid("Template không thuộc danh sách của kênh")
		}
	} else if len(templates) > 0 {
		idx := owner.TemplateCursor % len(templates)
		if idx < 0 {
			idx = 0
		}
		t := templates[idx]
		out.Template = domain.ClonePlanTemplate{ID: t.ID, Name: t.Name, FromRotation: true}
	}
	var recipe *domain.Recipe
	if out.Template.ID != "" {
		raw, e := s.store.TemplateRecipe(ctx, out.Template.ID)
		if e != nil {
			return out, meta, e
		}
		var r domain.Recipe
		if json.Unmarshal(raw, &r) == nil {
			recipe = &r
		}
	}
	ids := []string{}
	keys := []string{}
	for _, u := range in.URLs {
		if id, reason := ParseVideoURL(u); reason == "" {
			ids = append(ids, id)
			keys = append(keys, domain.CloneSourceKey(in.ChannelID, id))
		}
	}
	jobs, err := s.store.FindJobsBySourceKeys(ctx, keys)
	if err != nil {
		return out, meta, err
	}
	queued, err := s.store.FindQueueRowsBySourceKeys(ctx, keys)
	if err != nil {
		return out, meta, err
	}
	duplicates := map[string]domain.ClonePlanDuplicate{}
	duplicateSources := map[string]domain.CloneSourceSnapshot{}
	for _, q := range queued {
		duplicates[q.SourceVideoID] = domain.ClonePlanDuplicate{JobID: q.JobID, State: string(q.State), Title: q.Source.Title}
		duplicateSources[q.SourceVideoID] = q.Source
	}
	for _, j := range jobs {
		id := j.ID
		duplicates[domain.CloneSourceVideoID(j.SourceKey)] = domain.ClonePlanDuplicate{JobID: &id, State: j.StageGroup, Title: j.Source.Title}
		duplicateSources[domain.CloneSourceVideoID(j.SourceKey)] = j.Source
	}
	videos, err := s.store.ListVideosByIDs(ctx, ids)
	if err != nil {
		return out, meta, err
	}
	channelIDs := []string{}
	for _, v := range videos {
		channelIDs = append(channelIDs, v.ChannelID)
	}
	channels, err := s.store.ChannelInfoOf(ctx, channelIDs)
	if err != nil {
		return out, meta, err
	}
	sources, err := s.store.ListSourcesWithChannels(ctx, in.ChannelID)
	if err != nil {
		return out, meta, err
	}
	linked := make(map[string]bool, len(sources))
	for _, source := range sources {
		linked[source.RivalID] = true
	}
	requestSeen := map[string]bool{}
	for _, rawURL := range in.URLs {
		id, reason := ParseVideoURL(rawURL)
		if reason != "" {
			out.Invalid = append(out.Invalid, domain.ClonePlanInvalid{URL: rawURL, Reason: reason})
			continue
		}
		if d, found := duplicates[id]; found {
			source := duplicateSources[id]
			d.ThumbnailURL, d.DurationSec, d.ViewCount, d.SourceChannelTitle, d.PublishedAt, d.Outlier = source.ThumbnailURL, source.DurationSec, source.ViewCount, source.ChannelTitle, source.PublishedAt, source.Outlier
			if source.HasCaptions != nil {
				d.HasCaptions = *source.HasCaptions
			}
			if v, ok := videos[id]; ok {
				ch := channels[v.ChannelID]
				if d.ThumbnailURL == nil {
					thumb := "https://i.ytimg.com/vi/" + id + "/mqdefault.jpg"
					d.ThumbnailURL = &thumb
				}
				if d.DurationSec == nil && v.DurationSec != nil {
					seconds := float64(*v.DurationSec)
					d.DurationSec = &seconds
				}
				if d.ViewCount == nil {
					d.ViewCount = v.ViewCount
				}
				if source.HasCaptions == nil && v.HasCaptions != nil {
					d.HasCaptions = *v.HasCaptions
				}
				if d.SourceChannelTitle == nil {
					d.SourceChannelTitle = &ch.Title
				}
				if d.PublishedAt == nil {
					d.PublishedAt = v.PublishedAt
				}
				if d.Outlier == nil && v.ViewCount != nil && ch.MedianViews != nil && *ch.MedianViews > 0 {
					o := float64(*v.ViewCount) / float64(*ch.MedianViews)
					d.Outlier = &o
				}
			}
			d.URL = rawURL
			out.Duplicates = append(out.Duplicates, d)
			continue
		}
		if requestSeen[id] {
			continue
		}
		requestSeen[id] = true
		var m VideoMetadata
		if v, found := videos[id]; found {
			ch := channels[v.ChannelID]
			thumb := "https://i.ytimg.com/vi/" + id + "/mqdefault.jpg"
			channelID := v.ChannelID
			channelTitle := ch.Title
			m.Source = domain.CloneSourceSnapshot{ID: id, URL: watchURL(id), Title: v.Title, ChannelID: &channelID, ChannelTitle: &channelTitle, ThumbnailURL: &thumb, ScannedAt: s.d.Now()}
			if v.DurationSec != nil {
				seconds := float64(*v.DurationSec)
				m.Source.DurationSec = &seconds
			}
			m.ViewCount = v.ViewCount
			m.PublishedAt = v.PublishedAt
			if v.HasCaptions != nil {
				m.HasCaptions = *v.HasCaptions
			}
			if v.ViewCount != nil && ch.MedianViews != nil && *ch.MedianViews > 0 {
				o := float64(*v.ViewCount) / float64(*ch.MedianViews)
				m.Outlier = &o
			}
		} else if s.d.Probe != nil {
			m, err = s.d.Probe(ctx, id)
			if err != nil {
				var unavailable *youtube.VideoUnavailableError
				reason := "unreadable"
				if errors.As(err, &unavailable) && unavailable.Reason == "private" {
					reason = "private"
				}
				out.Invalid = append(out.Invalid, domain.ClonePlanInvalid{URL: rawURL, Reason: reason})
				continue
			}
		} else {
			return out, meta, errors.New("chưa có bộ đọc thông tin video cho kế hoạch")
		}
		if m.Source.ID == "" || m.Source.Title == "" {
			out.Invalid = append(out.Invalid, domain.ClonePlanInvalid{URL: rawURL, Reason: "unreadable"})
			continue
		}
		if m.Source.URL == "" {
			m.Source.URL = watchURL(id)
		}
		m.Source.ViewCount, m.Source.PublishedAt, m.Source.Outlier = m.ViewCount, m.PublishedAt, m.Outlier
		hasCaptions := m.HasCaptions
		m.Source.HasCaptions = &hasCaptions
		rivalLinked := m.Source.ChannelID != nil && linked[*m.Source.ChannelID]
		meta[id] = m
		estimate, calls := estimateImages(recipe, m.Source.DurationSec, in.NoImages)
		out.Accepted = append(out.Accepted, domain.ClonePlanAccepted{URL: rawURL, VideoID: id, Title: m.Source.Title, DurationSec: m.Source.DurationSec, Outlier: m.Outlier, HasCaptions: m.HasCaptions, ThumbnailURL: m.Source.ThumbnailURL, SourceChannelTitle: m.Source.ChannelTitle, SourceChannelID: m.Source.ChannelID, RivalLinked: rivalLinked, ViewCount: m.ViewCount, PublishedAt: m.PublishedAt, ImageEstimate: estimate, ImageCalls: calls})
		if calls != nil {
			out.ImageCalls += *calls
		}
	}
	lanes, _, _, _, err := s.loadLanes(ctx)
	if err != nil {
		return out, meta, err
	}
	used, err := s.store.PaceUsedToday(ctx, LocalMidnight(s.d.Now()))
	if err != nil {
		return out, meta, err
	}
	out.PaceLeftToday = max(0, owner.PacePerDay-int(used[owner.ID]))
	if len(out.Accepted) == 0 {
		return out, meta, nil
	}
	target := -1
	queuedBatch := 0
	for i := range lanes {
		if lanes[i].ChannelID == owner.ID {
			target = i
			for _, item := range lanes[i].Items {
				if item.Priority == "batch" {
					queuedBatch++
				}
			}
			break
		}
	}
	if target < 0 {
		lanes = append(lanes, Lane{ChannelID: owner.ID, Pace: owner.PacePerDay, UsedToday: int(used[owner.ID]), TemplateOK: len(templates) > 0})
		target = len(lanes) - 1
	}
	lane := &lanes[target]
	// All accepted videos are included in simulation, not just the first.
	firstID := ""
	position := 0.0
	if priority == domain.ClonePick {
		position = float64(s.d.Now().UnixNano()) / 1e9
		for _, l := range lanes {
			for _, i := range l.Items {
				if i.Priority == "pick" && i.Position > position {
					position = i.Position
				}
			}
		}
	} else {
		for _, i := range lane.Items {
			if i.Priority == "batch" && i.Position > position {
				position = i.Position
			}
		}
	}
	for idx, a := range out.Accepted {
		seconds := int64(0)
		if a.DurationSec != nil {
			seconds = int64(*a.DurationSec)
		}
		id := "__plan_" + a.VideoID
		if idx == 0 {
			firstID = id
		}
		position++
		lane.Items = append(lane.Items, Item{ID: id, ChannelID: owner.ID, Priority: string(priority), Position: position, DurationSec: seconds, CreatedAt: s.d.Now()})
	}
	rules, err := s.Rules(ctx)
	if err != nil {
		return out, meta, err
	}
	basis, err := s.basis(ctx)
	if err != nil {
		return out, meta, err
	}
	active, err := s.store.ListActiveJobs(ctx)
	if err != nil {
		return out, meta, err
	}
	estimates := Estimate(lanes, EstimateInput{Now: s.d.Now(), Workers: rules.Workers, Basis: basis, Active: activeETA(active), Paused: rules.Paused})
	out.ETAStart = estimates[firstID].Start
	if priority == domain.CloneBatch {
		out.OverflowDays = max(0, int(math.Ceil(float64(queuedBatch+len(out.Accepted)+int(used[owner.ID]))/float64(owner.PacePerDay)))-1)
	}
	return out, meta, nil
}

func estimateImages(recipe *domain.Recipe, seconds *float64, noImages bool) (*int, *int) {
	if noImages {
		n := 0
		return &n, &n
	}
	if recipe == nil {
		return nil, nil
	}
	images := recipe.Scenes.Images
	n := 0
	switch images.Mode {
	case "none":
		return &n, &n
	case "fixed":
		n = images.Count
	case "per-minutes":
		if seconds == nil || images.Minutes <= 0 {
			return nil, nil
		}
		n = int(math.Ceil(*seconds / 60 / float64(images.Minutes)))
	default:
		return nil, nil
	}
	if recipe.Scenes.CoverSource == "ai" {
		n++
	}
	calls := int(math.Ceil(float64(n) * redrawFactor))
	return &n, &calls
}
func activeETA(jobs []domain.CloneJob) []ActiveJobETA {
	out := make([]ActiveJobETA, 0, len(jobs))
	for _, j := range jobs {
		minutes := defaultSrcMin
		if j.Source.DurationSec != nil && *j.Source.DurationSec > 0 {
			minutes = *j.Source.DurationSec / 60
		}
		out = append(out, ActiveJobETA{Minutes: minutes, StartedAt: j.CreatedAt})
	}
	return out
}

// YouTubeProbe adapts the shared paced player client into the plan seam.
func YouTubeProbe(client *youtube.Client) func(context.Context, string) (VideoMetadata, error) {
	return func(ctx context.Context, id string) (VideoMetadata, error) {
		v, e := client.FetchVideoMetadata(ctx, id)
		if e != nil {
			return VideoMetadata{}, e
		}
		return VideoMetadata{Source: domain.CloneSourceSnapshot{ID: v.ID, URL: watchURL(v.ID), Title: v.Title, ChannelID: v.ChannelID, ChannelTitle: v.ChannelTitle, DurationSec: v.DurationSec, ThumbnailURL: v.ThumbnailURL, ScannedAt: time.Now()}, ViewCount: v.ViewCount, PublishedAt: v.PublishedAt, HasCaptions: v.HasCaptions}, nil
	}
}
