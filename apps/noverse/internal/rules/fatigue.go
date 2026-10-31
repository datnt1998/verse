package rules

import (
	"math"
	"sort"
	"strings"
)

// FatigueWord là ngưỡng hiệu lực cho thẻ chống giọng văn máy.
// PerChapter dùng chương chuẩn, không phải độ dài của chương gần nhất.
type FatigueWord struct {
	Word       string  `json:"word"`
	Per1000    float64 `json:"per1000"`
	PerChapter int     `json:"perChapter"`
	Source     string  `json:"source,omitempty"` // default | pack | project | story | learned; absent only for ambiguous legacy snapshots
}

func fatigueCandidateSource(candidate Candidate, word string) string {
	if source := candidate.FatigueSources[word]; source != "" {
		return source
	}
	return candidate.Source
}

func fatigueSource(source string) string {
	switch {
	case source == "system_defaults":
		return "default"
	case source == "genre_pack" || strings.HasPrefix(source, "pack:"):
		return "pack"
	case strings.HasPrefix(source, "project:"):
		return "project"
	case source == "runtime_update":
		return "learned"
	case source == "startup_prompt" || strings.HasPrefix(source, "story:"):
		return "story"
	default:
		return ""
	}
}

// Legacy snapshots do not retain per-word candidates. A single explicit scope
// proves the source of an explicit threshold; mixed scopes do not. Never guess
// from current files, which may have changed since the snapshot was applied.
func historicalFatigueSource(snapshot Snapshot, explicit bool) string {
	source := ""
	for _, candidateSource := range snapshot.Sources {
		if candidateSource == "system_defaults" || candidateSource == "default:lessons" || candidateSource == "global:lessons" {
			continue
		}
		if !explicit {
			return ""
		}
		kind := fatigueSource(candidateSource)
		if kind == "" || (source != "" && fatigueSource(source) != kind) {
			return ""
		}
		source = candidateSource
	}
	if source == "" && !explicit {
		return "system_defaults"
	}
	return source
}

// EffectiveFatigue dùng cùng phép ghép với checker, nhưng giữ nguồn thắng theo
// từng từ. Không suy nguồn từ việc ngưỡng khác mặc định: một ghi đè bằng đúng
// mức nền vẫn là quy tắc tường minh của người dùng.
func EffectiveFatigue(snapshot *Snapshot, pack Candidate) []FatigueWord {
	structured := Effective(snapshot, pack)
	defaults := SystemDefaults().Structured.FatigueWords
	var explicit Structured
	if snapshot != nil {
		explicit = explicitStructured(*snapshot)
	}
	out := make([]FatigueWord, 0, len(structured.FatigueWords))
	for word, limit := range structured.FatigueWords {
		source := "system_defaults"
		if pack.Source == "genre_pack" {
			if _, exists := pack.Structured.FatigueWords[word]; exists {
				source = fatigueCandidateSource(pack, word)
			}
		}
		if snapshot != nil {
			_, userOverride := explicit.FatigueWords[word]
			if pack.Source != "genre_pack" || userOverride {
				if saved := snapshot.FatigueSources[word]; saved != "" {
					source = saved
				} else if base, exists := defaults[word]; !exists || limit != base || userOverride {
					source = historicalFatigueSource(*snapshot, true)
				} else if snapshot.UserStructured == nil {
					source = historicalFatigueSource(*snapshot, false)
				}
			}
		}
		out = append(out, FatigueWord{
			Word: word, Per1000: limit.Per1000(),
			PerChapter: int(math.Round(limit.Per1000() * FatigueRefChapterTieng / 1000)),
			Source: fatigueSource(source),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Word < out[j].Word })
	return out
}
