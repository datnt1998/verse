package render

import (
	"bytes"
	"encoding/json"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

func qualityStaleItems(score *quality.Score, measured, current editor.Project, fresh bool) map[string]bool {
	stale := map[string]bool{}
	if score != nil {
		for _, item := range score.Items {
			stale[item.ID] = !fresh
		}
	}
	if !fresh && measured != nil && current != nil && captionTimingOnlyChange(measured, current) {
		for id := range stale {
			stale[id] = id == "captionAlignment" || id == "captionKaraoke"
		}
	}
	return stale
}

// Keep the measured document so a revision change is not mistaken for a change
// to every check. Other edits remain conservatively stale; this presentation
// never changes the revision-bound export decision.
func captionTimingOnlyChange(before, after editor.Project) bool {
	normalize := func(project editor.Project) ([]byte, []byte) {
		copy := editor.Clone(project)
		delete(copy, "updatedAt")
		original, _ := json.Marshal(copy)
		var walk func(any)
		walk = func(value any) {
			switch value := value.(type) {
			case map[string]any:
				if value["type"] == "subtitle" {
					delete(value, "words")
					delete(value, "wordsEstimated")
					for _, cue := range scoreObjects(value["cues"]) {
						delete(cue, "startSeconds")
						delete(cue, "endSeconds")
					}
				}
				for _, child := range value {
					walk(child)
				}
			case []any:
				for _, child := range value {
					walk(child)
				}
			}
		}
		walk(map[string]any(copy))
		raw, _ := json.Marshal(copy)
		return original, raw
	}
	beforeOriginal, beforeTiming := normalize(before)
	afterOriginal, afterTiming := normalize(after)
	return !bytes.Equal(beforeOriginal, afterOriginal) && bytes.Equal(beforeTiming, afterTiming)
}
