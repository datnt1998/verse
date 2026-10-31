package editor

import (
	"context"
	"math"
	"net/url"
)

var sourceCacheFields = []string{"thumbnailUrl", "waveformData", "reverseConformSrc", "reverseConformPath", "reverseConformKey", "reverseConformPreviewSrc", "reverseConformPreviewPath", "reverseConformPreviewKey", "reverseConformPreviewUsesProxy", "reverseConformStatus", "reverseConformPreviewIsSourceLevel", "reverseConformPreviewSourceDuration", "reverseConformPreviewFps", "reverseConformLocalStart"}

// refreshProjectSources mirrors media-source-refresh.ts. Only editorial
// collections are traversed; DSP/sample payloads must never be walked.
func (s *Service) refreshProjectSources(ctx context.Context, p Project) error {
	cache := map[string]*MediaRecord{}
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
