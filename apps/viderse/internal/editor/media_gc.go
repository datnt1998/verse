package editor

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const MediaClaimGrace = 30 * time.Minute

var mediaUUIDToken = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func (s *Service) mediaProjectIDs(ctx context.Context) ([]string, error) {
	ps, e := s.listPaths(ctx, "editor/projects/")
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	for _, p := range ps {
		parts := strings.Split(strings.TrimPrefix(p, "editor/projects/"), "/")
		if len(parts) > 1 && portableID.MatchString(parts[0]) {
			set[parts[0]] = true
		}
	}
	if s.Repository != nil {
		rs, e := s.Repository.List(ctx, "projects/")
		if e != nil {
			return nil, e
		}
		for _, r := range rs {
			parts := strings.Split(strings.TrimPrefix(r.Key, "projects/"), "/")
			if len(parts) > 0 && portableID.MatchString(parts[0]) {
				set[parts[0]] = true
			}
		}
	}
	ids := []string{}
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func addMediaTokens(b []byte, held map[string]bool) error {
	var value any
	if e := json.Unmarshal(b, &value); e != nil {
		return e
	}
	for _, id := range mediaUUIDToken.FindAllString(string(b), -1) {
		held[strings.ToLower(id)] = true
	}
	return nil
}
func (s *Service) projectMediaTokens(ctx context.Context, pid string) (map[string]bool, error) {
	if e := ValidateID(pid); e != nil {
		return nil, e
	}
	held := map[string]bool{}
	prefix := "editor/projects/" + pid + "/"
	ps, e := s.listPaths(ctx, prefix)
	if e != nil {
		return nil, e
	}
	active := map[string]bool{}
	if s.Repository != nil {
		rs, e := s.Repository.List(ctx, "projects/"+pid)
		if e != nil {
			return nil, e
		}
		for _, r := range rs {
			if r.Key != "projects/"+pid && !strings.HasPrefix(r.Key, "projects/"+pid+"/") {
				continue
			}
			if r.Path == "" {
				return nil, apiError("internal_error", "Project resource has no path")
			}
			h, e := s.acquire(ctx, r.Path)
			if e != nil {
				return nil, e
			}
			b, e := os.ReadFile(h.Path)
			h.Release()
			if e != nil {
				return nil, e
			}
			if e = addMediaTokens(b, held); e != nil {
				return nil, e
			}
			active[r.Key] = true
		}
	}
	for _, p := range ps {
		name := strings.TrimPrefix(p, prefix)
		if strings.Contains(name, "/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		key := ""
		switch name {
		case "project.json":
			key = "projects/" + pid
		case "links.json", "media-links.json":
			key = "projects/" + pid + "/links"
		case "animation-presets.json":
			key = "projects/" + pid + "/sidecars/animation-presets"
		case "render-queue.json":
			key = "projects/" + pid + "/sidecars/render-queue"
		}
		if active[key] {
			continue
		}
		h, e := s.acquire(ctx, p)
		if mediaMissing(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		b, e := os.ReadFile(h.Path)
		h.Release()
		if e != nil {
			return nil, e
		}
		if e = addMediaTokens(b, held); e != nil {
			return nil, e
		}
	}
	return held, nil
}
func (s *Service) ProjectMediaIDs(ctx context.Context, pid string) ([]string, error) {
	tokens, e := s.projectMediaTokens(ctx, pid)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for id := range tokens {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func (s *Service) heldMedia(ctx context.Context) (map[string]bool, error) {
	held := map[string]bool{}
	pids, e := s.mediaProjectIDs(ctx)
	if e != nil {
		return nil, e
	}
	for _, pid := range pids {
		tokens, e := s.projectMediaTokens(ctx, pid)
		if e != nil {
			return nil, e
		}
		for id := range tokens {
			held[id] = true
		}
	}
	if s.MediaReferences != nil {
		ids, e := s.MediaReferences(ctx)
		if e != nil {
			return nil, e
		}
		for _, id := range ids {
			held[strings.ToLower(id)] = true
		}
	}
	return held, nil
}
func (s *Service) reclaimMedia(ctx context.Context, ids []string, whole, dry bool) (Object, []string, error) {
	result := Object{"mediaIds": []string{}, "bytes": int64(0)}
	deferred := []string{}
	held, e := s.heldMedia(ctx)
	if e != nil {
		return nil, nil, e
	}
	reclaimed := []string{}
	var bytes int64
	seen := map[string]bool{}
	for _, id := range ids {
		if e := ValidateID(id); e != nil {
			return nil, nil, e
		}
		if seen[id] || held[strings.ToLower(id)] {
			continue
		}
		seen[id] = true
		m, _, e := s.readMedia(ctx, id)
		if mediaMissing(e) {
			if whole {
				n, ok, err := s.reclaimUnfinishedMedia(ctx, id, dry)
				if err != nil {
					return nil, nil, err
				}
				if ok {
					reclaimed = append(reclaimed, id)
					bytes += n
				}
			}
			continue
		}
		if e != nil {
			return nil, nil, e
		}
		hashLock := resourceLock("media-import/" + m.ContentHash)
		hashLock.Lock()
		l := resourceLock("media/" + id)
		l.Lock()
		m, _, e = s.readMedia(ctx, id)
		if mediaMissing(e) {
			l.Unlock()
			hashLock.Unlock()
			continue
		}
		if e != nil {
			l.Unlock()
			hashLock.Unlock()
			return nil, nil, e
		}
		fresh := s.now().UnixMilli()-m.LastUsedAt < MediaClaimGrace.Milliseconds() && m.LastUsedAt != 0
		if m.Overlay || fresh || (whole && s.now().UnixMilli()-m.CreatedAt < MediaClaimGrace.Milliseconds()) {
			if !m.Overlay && fresh {
				deferred = append(deferred, id)
			}
			l.Unlock()
			hashLock.Unlock()
			continue
		}
		current, e := s.heldMedia(ctx)
		if e != nil {
			l.Unlock()
			hashLock.Unlock()
			return nil, nil, e
		}
		if current[strings.ToLower(id)] {
			l.Unlock()
			hashLock.Unlock()
			continue
		}
		n := m.Size
		ps, e := s.listPaths(ctx, "editor/media/"+id+"/")
		if e == nil {
			n = 0
			for _, p := range ps {
				h, err := s.acquire(ctx, p)
				if err != nil {
					e = err
					break
				}
				stat, err := os.Stat(h.Path)
				h.Release()
				if err != nil {
					e = err
					break
				}
				n += stat.Size()
			}
			if n < m.Size {
				n = m.Size
			}
		}
		if e == nil && !dry {
			e = s.removeMediaResources(ctx, id)
		}
		l.Unlock()
		hashLock.Unlock()
		if e != nil {
			return nil, nil, e
		}
		reclaimed = append(reclaimed, id)
		bytes += n
	}
	result["mediaIds"] = reclaimed
	result["bytes"] = bytes
	return result, deferred, nil
}
func (s *Service) ReclaimUnreferencedMedia(ctx context.Context, dryRun bool) (Object, error) {
	ids, e := s.mediaSweepIDs(ctx)
	if e != nil {
		return nil, e
	}
	out, _, e := s.reclaimMedia(ctx, ids, true, dryRun)
	if e == nil && !dryRun {
		_, e = s.RebuildMediaIndex(ctx)
	}
	return out, e
}
func (s *Service) updateMediaPending(ctx context.Context, change func(map[string]bool)) error {
	l := resourceLock("media-reclaim-pending")
	l.Lock()
	defer l.Unlock()
	o, rev, e := s.readJSON(ctx, "workspace/media-reclaim-pending", "editor/media-reclaim-pending.json")
	if e != nil && !mediaMissing(e) {
		return e
	}
	ids := map[string]bool{}
	if o != nil {
		b, e := json.Marshal(o["ids"])
		if e != nil {
			return e
		}
		var values []string
		if e = json.Unmarshal(b, &values); e != nil {
			return e
		}
		for _, id := range values {
			if e = ValidateID(id); e != nil {
				return e
			}
			ids[id] = true
		}
	}
	change(ids)
	values := []string{}
	for id := range ids {
		values = append(values, id)
	}
	sort.Strings(values)
	_, e = s.writeJSON(ctx, "workspace/media-reclaim-pending", "editor/media-reclaim-pending.json", Object{"ids": values}, rev)
	return e
}
func (s *Service) ReclaimMediaOf(ctx context.Context, ids []string) (Object, error) {
	out, deferred, e := s.reclaimMedia(ctx, ids, false, false)
	if e != nil {
		return nil, e
	}
	if len(deferred) > 0 {
		e = s.updateMediaPending(ctx, func(ids map[string]bool) {
			for _, id := range deferred {
				ids[id] = true
			}
		})
	}
	return out, e
}
func (s *Service) ReclaimPendingMedia(ctx context.Context) (Object, error) {
	ids := []string{}
	if e := s.updateMediaPending(ctx, func(p map[string]bool) {
		for id := range p {
			ids = append(ids, id)
		}
	}); e != nil {
		return nil, e
	}
	sort.Strings(ids)
	out, deferred, e := s.reclaimMedia(ctx, ids, false, false)
	if e != nil {
		return nil, e
	}
	still := map[string]bool{}
	for _, id := range deferred {
		still[id] = true
	}
	e = s.updateMediaPending(ctx, func(p map[string]bool) {
		for _, id := range ids {
			if !still[id] {
				delete(p, id)
			}
		}
	})
	return out, e
}

func (s *Service) mediaSweepIDs(ctx context.Context) ([]string, error) {
	ids, e := s.mediaIDs(ctx)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	paths, e := s.listPaths(ctx, "editor/media/")
	if e != nil {
		return nil, e
	}
	for _, p := range paths {
		parts := strings.SplitN(strings.TrimPrefix(p, "editor/media/"), "/", 2)
		if len(parts) == 2 && portableID.MatchString(parts[0]) && !seen[parts[0]] {
			ids = append(ids, parts[0])
			seen[parts[0]] = true
		}
	}
	sort.Strings(ids)
	return ids, nil
}
func (s *Service) reclaimUnfinishedMedia(ctx context.Context, id string, dry bool) (int64, bool, error) {
	l := resourceLock("media/" + id)
	l.Lock()
	defer l.Unlock()
	if _, _, e := s.readMedia(ctx, id); !mediaMissing(e) {
		return 0, false, e
	}
	held, e := s.heldMedia(ctx)
	if e != nil {
		return 0, false, e
	}
	if held[strings.ToLower(id)] {
		return 0, false, nil
	}
	ps, e := s.listPaths(ctx, "editor/media/"+id+"/")
	if e != nil {
		return 0, false, e
	}
	if len(ps) == 0 {
		return 0, false, nil
	}
	var bytes int64
	for _, p := range ps {
		h, e := s.acquire(ctx, p)
		if e != nil {
			return 0, false, e
		}
		stat, e := os.Stat(h.Path)
		birth := h.Ref.VerifiedAt
		h.Release()
		if e != nil {
			return 0, false, e
		}
		if birth.IsZero() {
			birth = stat.ModTime()
		}
		if s.now().Sub(birth) < MediaClaimGrace {
			return 0, false, nil
		}
		bytes += stat.Size()
	}
	if !dry {
		if e = s.removeMediaResources(ctx, id); e != nil {
			return 0, false, e
		}
	}
	return bytes, true, nil
}
