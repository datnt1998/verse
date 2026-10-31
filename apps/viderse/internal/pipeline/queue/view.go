package queue

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func (s *Service) List(ctx context.Context) (domain.CloneQueueView, error) {
	out := domain.CloneQueueView{Running: []domain.QueueRunningJob{}, Items: []domain.CloneQueueItemView{}, Pace: []domain.QueuePace{}, ImageQuota: []domain.QueueImageQuota{}, DoneToday: []domain.QueueDoneToday{}, Rotation: domain.QueueRotation{Order: []string{}, Skipped: []domain.QueueSkippedChannel{}}}
	rules, err := s.Rules(ctx)
	if err != nil {
		return out, err
	}
	out.Paused = rules.Paused
	out.PausedAt = rules.PausedAt
	out.Workers.Total = rules.Workers
	out.ImageWorkers = rules.ImageWorkers
	out.RescanHours = rules.RescanHours
	lanes, items, owners, templates, err := s.loadLanes(ctx)
	if err != nil {
		return out, err
	}
	active, err := s.store.ListActiveJobs(ctx)
	if err != nil {
		return out, err
	}
	out.Workers.Used = len(active)
	basis, err := s.basis(ctx)
	if err != nil {
		return out, err
	}
	simulated := make([]Lane, len(lanes))
	copy(simulated, lanes)
	for i := range simulated {
		kept := make([]Item, 0, len(simulated[i].Items))
		for _, it := range simulated[i].Items {
			row := items[it.ID]
			if row.Priority == domain.ClonePick || row.Origin != domain.OriginAuto || owners[row.ChannelID].AutoPick {
				kept = append(kept, it)
			}
		}
		simulated[i].Items = kept
	}
	estimates := Estimate(simulated, EstimateInput{Now: s.d.Now(), Workers: rules.Workers, Basis: basis, Active: activeETA(active), Paused: rules.Paused})
	for _, j := range active {
		steps, e := s.store.ListJobSteps(ctx, j.ID)
		if e != nil {
			return out, e
		}
		idx := 0
		agent := "scout"
		if j.ActiveStep != nil {
			idx = domain.IndexStep(*j.ActiveStep)
			agent = domain.CloneAgentOf(*j.ActiveStep)
		}
		now := domain.Subline(j, steps, s.d.Now())
		started := j.CreatedAt
		templateName := ""
		for _, t := range templates[j.ChannelID] {
			if t.ID == j.TemplateID {
				templateName = t.Name
			}
		}
		out.Running = append(out.Running, domain.QueueRunningJob{JobID: j.ID, ChannelID: j.ChannelID, Title: j.Source.Title, Agent: agent, Now: now, StepIndex: idx, ThumbnailURL: j.Source.ThumbnailURL, TemplateName: templateName, DurationSec: j.Source.DurationSec, StartedAt: &started, PausedAt: j.PausedAt})
	}
	used, err := s.store.PaceUsedToday(ctx, LocalMidnight(s.d.Now()))
	if err != nil {
		return out, err
	}
	ownerRows, err := s.store.Owners(ctx)
	if err != nil {
		return out, err
	}
	for _, o := range ownerRows {
		out.Pace = append(out.Pace, domain.QueuePace{ChannelID: o.ID, Used: int(used[o.ID]), Limit: o.PacePerDay})
	}
	for _, lane := range lanes {
		hasBatch := false
		hasEligibleBatch := false
		for _, it := range lane.Items {
			if it.Priority == "batch" {
				hasBatch = true
				row := items[it.ID]
				if row.Origin != domain.OriginAuto || owners[it.ChannelID].AutoPick {
					hasEligibleBatch = true
				}
			}
		}
		if !hasBatch {
			continue
		}
		out.Rotation.Order = append(out.Rotation.Order, lane.ChannelID)
		reason := ""
		var until *time.Time
		switch {
		case !lane.TemplateOK:
			reason = domain.SkipNoTemplate
		case !hasEligibleBatch:
			reason = domain.SkipPausedChannel
		case lane.UsedToday >= lane.Pace:
			reason = domain.SkipPaceFull
			at := LocalMidnight(s.d.Now()).AddDate(0, 0, 1)
			until = &at
		}
		if reason != "" {
			out.Rotation.Skipped = append(out.Rotation.Skipped, domain.QueueSkippedChannel{ChannelID: lane.ChannelID, Reason: reason, Until: until})
		} else if out.Rotation.Next == nil {
			id := lane.ChannelID
			out.Rotation.Next = &id
		}
	}
	// The view shows every queued row, even beyond the bounded ETA workset.
	for _, it := range Order(lanes, ItemsFetchCap) {
		row := items[it.ID]
		eta := estimates[it.ID]
		wait := eta.Wait
		if row.Priority == domain.CloneBatch && row.Origin == domain.OriginAuto && !owners[row.ChannelID].AutoPick {
			eta.Start = nil
			wait = domain.WaitPausedChannel
		}
		if rules.Paused {
			eta.Start = nil
			wait = domain.WaitQueuePaused
		}
		if wait == "" {
			if out.Workers.Used >= rules.Workers {
				wait = domain.WaitWorkersBusy
			} else {
				wait = domain.WaitRotation
			}
		}
		tplName := ""
		ts := templates[row.ChannelID]
		if len(ts) > 0 {
			idx := owners[row.ChannelID].TemplateCursor % len(ts)
			if idx < 0 {
				idx = 0
			}
			tplName = ts[idx].Name
		}
		if row.TemplateID != nil {
			for _, t := range ts {
				if t.ID == *row.TemplateID {
					tplName = t.Name
				}
			}
		}
		source := ""
		if row.Source.ChannelTitle != nil {
			source = *row.Source.ChannelTitle
		}
		out.Items = append(out.Items, domain.CloneQueueItemView{ID: row.ID, ChannelID: row.ChannelID, Title: row.Source.Title, Source: source, SourceURL: row.Source.URL, ThumbnailURL: row.Source.ThumbnailURL, TemplateName: tplName, DurationSec: row.Source.DurationSec, Priority: row.Priority, Position: row.Position, ETAStart: eta.Start, WaitReason: wait})
	}
	done, err := s.store.ListDoneTodayJobs(ctx, LocalMidnight(s.d.Now()))
	if err != nil {
		return out, err
	}
	for _, j := range done {
		state := j.StageGroup
		if state == "posted" {
			state = "ready"
		}
		out.DoneToday = append(out.DoneToday, domain.QueueDoneToday{JobID: j.JobID, Title: j.Title, At: j.UpdatedAt, State: state})
	}
	if s.d.ImageSources != nil {
		out.ImageQuota, err = s.d.ImageSources(ctx)
		if err != nil {
			return out, err
		}
		charged, e := s.store.CountChargedImagesToday(ctx, LocalMidnight(s.d.Now()))
		if e != nil {
			return out, e
		}
		for i := range out.ImageQuota {
			provider := out.ImageQuota[i].Provider
			if provider == "chatgpt" {
				provider = "codex"
			}
			out.ImageQuota[i].Used = charged[provider]
		}
	}
	return out, nil
}

// Counts reads grouped rows, never a bounded list page or per-job snapshots.
func (s *Service) Counts(ctx context.Context) (domain.CloneCounts, error) {
	out := domain.CloneCounts{ByOwner: map[string]domain.CloneBucketCounts{}}
	grouped, err := s.store.ListJobStageCounts(ctx)
	if err != nil {
		return out, err
	}
	queued, err := s.store.ListQueueCountsByChannel(ctx)
	if err != nil {
		return out, err
	}
	done, err := s.store.ListDoneTodayCounts(ctx, LocalMidnight(s.d.Now()))
	if err != nil {
		return out, err
	}
	for owner, groups := range grouped {
		c := domain.CloneBucketCounts{Working: int(groups["working"]), Review: int(groups["review"]), Attention: int(groups["attention"]), Paused: int(groups["paused"]), Ready: int(groups["ready"]), Posted: int(groups["posted"])}
		out.ByOwner[owner] = c
	}
	for owner, n := range queued {
		c := out.ByOwner[owner]
		c.Queued = int(n)
		out.ByOwner[owner] = c
	}
	for owner, n := range done {
		c := out.ByOwner[owner]
		c.DoneToday = int(n)
		out.ByOwner[owner] = c
	}
	for _, c := range out.ByOwner {
		out.Totals.Queued += c.Queued
		out.Totals.Working += c.Working
		out.Totals.Review += c.Review
		out.Totals.Attention += c.Attention
		out.Totals.Paused += c.Paused
		out.Totals.Ready += c.Ready
		out.Totals.Posted += c.Posted
		out.Totals.DoneToday += c.DoneToday
	}
	return out, nil
}
