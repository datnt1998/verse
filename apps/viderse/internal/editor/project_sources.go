package editor

import (
	"context"
	"errors"
	"math"
	"net/url"
	"sort"
)

var sourceCacheFields = []string{"thumbnailUrl", "waveformData", "reverseConformSrc", "reverseConformPath", "reverseConformKey", "reverseConformPreviewSrc", "reverseConformPreviewPath", "reverseConformPreviewKey", "reverseConformPreviewUsesProxy", "reverseConformStatus", "reverseConformPreviewIsSourceLevel", "reverseConformPreviewSourceDuration", "reverseConformPreviewFps", "reverseConformLocalStart"}

// visitProjectSourceItems limits resource discovery to the same editorial
// collections as source refresh, never walking keyframes or sample payloads.
func visitProjectSourceItems(p Project, consume func(Object) error) error {
	var visit func(any) error
	visit = func(value any) error {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				if err := visit(child); err != nil {
					return err
				}
			}
		case map[string]any:
			if err := consume(v); err != nil {
				return err
			}
			for _, key := range []string{"items", "compositions", "stashStack", "mainHolder"} {
				if err := visit(v[key]); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(p["timeline"])
}

// projectMediaResources captures only current aliases consumed by a workspace
// save. CAS promotes their exact immutable bindings in the project publication
// transaction; no historical alias lookup or metadata reserialization is needed.
func (s *Service) projectMediaResources(ctx context.Context, pid string, p Project) ([]Resource, error) {
	repo := s.repository()
	if repo == nil {
		return nil, errors.New("editor repository is required")
	}
	if err := ValidateID(pid); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	if err := visitProjectSourceItems(p, func(item Object) error {
		if id, _ := item["mediaId"].(string); id != "" {
			ids[id] = true
		}
		return nil
	}); err != nil {
		return nil, err
	}
	resources := []Resource{}
	links, err := repo.Get(ctx, "projects/"+pid+"/links")
	if err != nil && !mediaMissing(err) {
		return nil, err
	}
	if err == nil {
		value, revision, err := s.readMediaLinks(ctx, pid, true)
		if err != nil {
			return nil, err
		}
		if revision != links.Revision {
			return nil, ErrConflict
		}
		if links.Version == 0 {
			resources = append(resources, links)
		}
		for _, entry := range value.MediaIDs {
			ids[entry.ID] = true
		}
	}
	for id := range ids {
		if err := ValidateID(id); err != nil {
			return nil, err
		}
		head, err := repo.Get(ctx, "media/"+id)
		if mediaMissing(err) {
			// Saving never revives deleted media or invents missing metadata.
			continue
		}
		if err != nil {
			return nil, err
		}
		sourceKey := "media/" + id + "/source"
		source, sourceErr := repo.Get(ctx, sourceKey)
		if sourceErr != nil && !mediaMissing(sourceErr) {
			return nil, sourceErr
		}
		if head.Version != 0 && sourceErr == nil {
			continue
		}
		record, revision, err := s.readMedia(ctx, id)
		if err != nil {
			return nil, err
		}
		if revision != head.Revision {
			return nil, ErrConflict
		}
		if sourceErr == nil {
			resources = append(resources, head)
			if source.Revision != record.ContentHash {
				return nil, ErrConflict
			}
			continue
		}
		path, err := s.mediaSourcePath(ctx, id, record)
		if err != nil {
			if record.SourceFile == "" && mediaMissing(err) {
				// Metadata-only library entries remain valid. Only a declared
				// or discovered source requires a binary root.
				if head.Version == 0 {
					resources = append(resources, head)
				}
				continue
			}
			return nil, err
		}
		path, err = s.resolvePath(ctx, path)
		if err != nil {
			return nil, err
		}
		// A nonempty expected digest plus PreviousPath adopts an existing
		// binding while refusing a competing source head or tombstone.
		resources = append(resources, head, Resource{Key: sourceKey, Path: path, Revision: record.ContentHash, PreviousPath: path, UpdatedAt: s.now()})
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Key < resources[j].Key })
	return resources, nil
}

// refreshProjectSources mirrors media-source-refresh.ts. Only editorial
// collections are traversed; DSP/sample payloads must never be walked.
func (s *Service) refreshProjectSources(ctx context.Context, p Project) error {
	cache := map[string]*MediaRecord{}
	return visitProjectSourceItems(p, func(v Object) error {
		id, _ := v["mediaId"].(string)
		kind, _ := v["type"].(string)
		eligible := kind == "video" || kind == "audio" || kind == "image" || kind == "lottie" || (kind == "shape" && v["waveformVisualizer"] != nil)
		if id != "" && eligible {
			record, seen := cache[id]
			if !seen {
				item, err := s.GetMediaMetadata(ctx, id)
				if err != nil && !mediaMissing(err) {
					return err
				}
				if err == nil {
					record = &item
				}
				cache[id] = record
			}
			if record != nil {
				if kind == "shape" {
					wave, ok := v["waveformVisualizer"].(map[string]any)
					if ok && wave["sourceHash"] != record.ContentHash {
						delete(wave, "envelopeBase64")
						delete(wave, "durationSeconds")
						delete(wave, "normalization")
					}
				}
				if kind != "shape" {
					for _, field := range sourceCacheFields {
						delete(v, field)
					}
					src := "/api/editor/media/" + url.PathEscape(id) + "?v=" + url.QueryEscape(record.ContentHash)
					v["src"] = src
					if kind == "video" || kind == "audio" {
						v["audioSrc"] = src
					}
					number := func(value *float64) float64 {
						if value == nil {
							return 0
						}
						return *value
					}
					fps := number(record.Probe.FPS)
					if fps == 0 {
						fps = fillNum(p.Metadata(), "fps", 30)
					}
					v["sourceWidth"] = number(record.Probe.Width)
					v["sourceHeight"] = number(record.Probe.Height)
					v["sourceFps"] = fps
					v["sourceDuration"] = math.Round(number(record.Probe.DurationSec) * fps)
				}
			}
		}
		return nil
	})
}
