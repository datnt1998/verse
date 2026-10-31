package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf16"
)

// CaptionPage is the shared engine page representation, not a timeline item.
type CaptionPage struct {
	ID         string            `json:"id"`
	Text       string            `json:"text"`
	StartFrame int               `json:"startFrame"`
	EndFrame   int               `json:"endFrame"`
	Words      []CaptionPageWord `json:"words"`
	Estimated  bool              `json:"estimated"`
}
type PreparedCaptions struct {
	Cues      []Object      `json:"cues"`
	Words     []Object      `json:"words"`
	Pages     []CaptionPage `json:"pages"`
	Estimated bool          `json:"estimated"`
}
type CaptionPageWord struct {
	Text         string `json:"text"`
	StartFrame   int    `json:"startFrame"`
	EndFrame     int    `json:"endFrame"`
	Estimated    bool   `json:"estimated"`
	estimatedSet bool
}

func (w CaptionPageWord) MarshalJSON() ([]byte, error) {
	m := Object{"text": w.Text, "startFrame": w.StartFrame, "endFrame": w.EndFrame}
	if w.Estimated || w.estimatedSet {
		m["estimated"] = w.Estimated
	}
	return json.Marshal(m)
}

type fillCaptionWindow struct {
	ID, Text   string
	Start, End int
}

func fillUTF16Len(s string) int { return len(utf16.Encode([]rune(s))) }
func fillTokens(parts []string, start, end int, estimated bool) []CaptionPageWord {
	out := []CaptionPageWord{}
	total := 0
	for _, s := range parts {
		total += fillUTF16Len(s)
	}
	if total == 0 || end <= start {
		return out
	}
	consumed := 0
	for i, s := range parts {
		left := start + (end-start)*consumed/total
		consumed += fillUTF16Len(s)
		right := end
		if i+1 < len(parts) {
			right = start + (end-start)*consumed/total
		}
		if right > left {
			out = append(out, CaptionPageWord{Text: s, StartFrame: left, EndFrame: right, Estimated: estimated, estimatedSet: true})
		}
	}
	return out
}

// PrepareCaptionData matches orderCueWordFrames and buildCaptionPages: words
// remain in reading order, silence remains silence, and only unmeasured timing
// is marked estimated. JavaScript character lengths use UTF-16 code units.
func PrepareCaptionData(cues []FillCue, fps float64, duration int) PreparedCaptions {
	valid := []FillCue{}
	for _, cue := range cues {
		if (cue.EndSec > cue.StartSec || len(cue.Words) > 0) && cue.StartSec >= 0 && cue.StartSec*fps < float64(duration) {
			valid = append(valid, cue)
		}
	}
	sort.SliceStable(valid, func(i, j int) bool { return valid[i].StartSec < valid[j].StartSec })
	words := []CaptionPageWord{}
	ordered := []fillCaptionWindow{}
	lastStart := -1
	for _, cue := range valid {
		first := len(words)
		for _, word := range cue.Words {
			start := int(fillRound(word.StartMs / 1000 * fps))
			estimated := word.Estimated
			if len(words) > 0 {
				previous := &words[len(words)-1]
				if start <= previous.StartFrame {
					start = previous.EndFrame
					estimated = true
				} else if start < previous.EndFrame {
					previous.EndFrame = start
				}
			}
			end := max(start+1, int(fillRound(word.EndMs/1000*fps)))
			words = append(words, CaptionPageWord{Text: word.Text, StartFrame: start, EndFrame: end, Estimated: estimated})
		}
		start := int(fillRound(cue.StartSec * fps))
		end := int(fillRound(cue.EndSec * fps))
		if len(words) > first {
			start = min(start, words[first].StartFrame)
			end = max(end, words[len(words)-1].EndFrame)
			lastStartBefore := lastStart
			lastStart = words[len(words)-1].StartFrame
			start = max(lastStartBefore+1, start)
		} else {
			start = max(lastStart+1, start)
			lastStart = start
		}
		end = min(duration, end)
		if end > start {
			ordered = append(ordered, fillCaptionWindow{cue.ID, cue.Text, start, end})
		}
	}
	windows := []fillCaptionWindow{}
	for i, win := range ordered {
		if i+1 < len(ordered) {
			win.End = min(win.End, ordered[i+1].Start)
		}
		win.End = min(win.End, duration)
		win.Start = max(0, win.Start)
		if win.End > win.Start {
			windows = append(windows, win)
		}
	}
	sort.SliceStable(words, func(i, j int) bool {
		if words[i].StartFrame != words[j].StartFrame {
			return words[i].StartFrame < words[j].StartFrame
		}
		return words[i].EndFrame < words[j].EndFrame
	})
	pages := []CaptionPage{}
	degraded := false
	index := 0
	for _, win := range windows {
		for index < len(words) && words[index].StartFrame < win.Start {
			degraded = true
			index++
		}
		pieces := []CaptionPageWord{}
		for index < len(words) && words[index].StartFrame < win.End {
			pieces = append(pieces, words[index])
			index++
		}
		tokens := []CaptionPageWord{}
		if len(pieces) == 0 {
			if strings.TrimSpace(win.Text) != "" && !strings.ContainsAny(win.Text, "<{}") {
				tokens = fillTokens(strings.Fields(win.Text), win.Start, win.End, true)
				if len(tokens) > 0 {
					degraded = true
				}
			}
		} else {
			for _, piece := range pieces {
				start := min(max(piece.StartFrame, win.Start), win.End)
				end := min(max(piece.EndFrame, win.Start), win.End)
				if end <= start {
					degraded = true
					continue
				}
				parts := strings.Fields(piece.Text)
				if len(parts) == 0 {
					degraded = true
					continue
				}
				if len(parts) == 1 {
					tokens = append(tokens, CaptionPageWord{Text: parts[0], StartFrame: start, EndFrame: end, Estimated: piece.Estimated})
				} else {
					produced := fillTokens(parts, start, end, piece.Estimated)
					if len(produced) == 0 {
						degraded = true
					}
					tokens = append(tokens, produced...)
				}
			}
		}
		groups := [][]CaptionPageWord{}
		group := []CaptionPageWord{}
		chars := 0
		for _, token := range tokens {
			added := fillUTF16Len(token.Text)
			pause := false
			if len(group) > 0 {
				added++
				previous := group[len(group)-1]
				pause = !previous.Estimated && !token.Estimated && float64(token.StartFrame-previous.EndFrame)*1000/fps >= 350
			}
			if len(group) > 0 && (pause || len(group) >= 7 || chars+added > 42) {
				groups = append(groups, group)
				group = []CaptionPageWord{}
				chars = 0
			}
			group = append(group, token)
			chars += fillUTF16Len(token.Text)
			if len(group) > 1 {
				chars++
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
		if len(groups) == 0 {
			if strings.TrimSpace(win.Text) != "" {
				pages = append(pages, CaptionPage{ID: win.ID, Text: win.Text, StartFrame: win.Start, EndFrame: win.End, Words: []CaptionPageWord{}})
			}
			continue
		}
		for i, group := range groups {
			end := win.End
			last := group[len(group)-1]
			if i+1 < len(groups) {
				next := groups[i+1][0]
				if !last.Estimated && !next.Estimated && float64(next.StartFrame-last.EndFrame)*1000/fps >= 350 {
					end = last.EndFrame
				} else {
					end = next.StartFrame
				}
			}
			id := win.ID
			if i > 0 {
				id = fmt.Sprintf("%s.%d", win.ID, i+1)
			}
			estimated := false
			texts := make([]string, len(group))
			for j, w := range group {
				texts[j] = w.Text
				estimated = estimated || w.Estimated
			}
			pages = append(pages, CaptionPage{ID: id, Text: strings.Join(texts, " "), StartFrame: group[0].StartFrame, EndFrame: end, Words: group, Estimated: estimated})
		}
	}
	if index < len(words) {
		degraded = true
	}
	for i := len(pages) - 1; i >= 0; i-- {
		if pages[i].EndFrame > pages[i].StartFrame {
			continue
		}
		neighbor := i - 1
		if neighbor < 0 {
			neighbor = i + 1
		}
		if neighbor >= 0 && neighbor < len(pages) && len(pages[neighbor].Words) > 0 && len(pages[i].Words) > 0 {
			pages[neighbor].Words = append(pages[neighbor].Words, pages[i].Words...)
			texts := []string{}
			for _, w := range pages[neighbor].Words {
				texts = append(texts, w.Text)
			}
			pages[neighbor].Text = strings.Join(texts, " ")
			pages[neighbor].Estimated = pages[neighbor].Estimated || pages[i].Estimated
		}
		pages = append(pages[:i], pages[i+1:]...)
	}
	for i := 0; i+1 < len(pages); i++ {
		gap := pages[i+1].StartFrame - pages[i].EndFrame
		if gap > 0 && float64(gap)*1000/fps < 350 {
			pages[i].EndFrame = pages[i+1].StartFrame
		}
	}
	out := PreparedCaptions{Cues: []Object{}, Words: []Object{}, Pages: pages, Estimated: degraded}
	for _, page := range pages {
		out.Cues = append(out.Cues, Object{"id": page.ID, "text": page.Text, "startSeconds": float64(page.StartFrame) / fps, "endSeconds": float64(page.EndFrame) / fps})
		for _, w := range page.Words {
			out.Words = append(out.Words, Object{"text": w.Text, "startFrame": w.StartFrame, "endFrame": w.EndFrame, "estimated": w.Estimated})
		}
	}
	return out
}

// EffectiveCaptionPreset preserves the owner's choice except karaoke with
// estimated word timing, which the canonical preview and export render as fade.
func EffectiveCaptionPreset(preset string, wordsEstimated, pageEstimated bool) string {
	if preset == "" {
		return "none"
	}
	if preset == "karaoke" && (wordsEstimated || pageEstimated) {
		return "fade"
	}
	return preset
}

func fillValidCaptionWords(words []Object, duration float64) bool {
	last := -math.MaxFloat64
	for _, w := range words {
		start := fillNum(w, "startFrame", math.NaN())
		end := fillNum(w, "endFrame", math.NaN())
		if fillString(w, "text") == "" || !fillFinite(start) || !fillFinite(end) || start < 0 || start != math.Floor(start) || end != math.Floor(end) || end <= start || end > duration || start < last {
			return false
		}
		last = start
	}
	return true
}
