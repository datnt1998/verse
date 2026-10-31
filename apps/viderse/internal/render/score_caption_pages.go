package render

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// ResolveCaptionPages normalizes saved cues without the ingest-time word
// reordering performed by PrepareCaptionData. This is the renderer's exact
// stored-cue interpretation (yt-factory 2c201e6 caption-pages.ts).
func ResolveCaptionPages(item editor.Object, fps float64) []editor.CaptionPage {
	duration := max(0, int(scoreNumber(item, "durationInFrames", 0)))
	estimated := scoreBool(item, "wordsEstimated", false)
	type window struct {
		id, text   string
		start, end int
	}
	windows := []window{}
	cues := scoreObjects(item["cues"])
	if len(cues) > 0 {
		sorted := make([]editor.Object, 0, len(cues))
		for _, cue := range cues {
			if scoreNumber(cue, "endSeconds", 0) > scoreNumber(cue, "startSeconds", 0) && scoreNumber(cue, "endSeconds", 0) > 0 {
				sorted = append(sorted, cue)
			}
		}
		sort.SliceStable(sorted, func(i, j int) bool {
			return scoreNumber(sorted[i], "startSeconds", 0) < scoreNumber(sorted[j], "startSeconds", 0)
		})
		for i, cue := range sorted {
			start := max(0, int(math.Floor(scoreNumber(cue, "startSeconds", 0)*fps+.5)))
			next := duration
			if i+1 < len(sorted) {
				next = int(math.Floor(scoreNumber(sorted[i+1], "startSeconds", 0)*fps + .5))
			}
			end := min(int(math.Floor(scoreNumber(cue, "endSeconds", 0)*fps+.5)), next, duration)
			id := scoreString(cue, "id")
			if id == "" {
				id = fmt.Sprintf("%s-%d", scoreString(item, "id"), i)
			}
			if end > start {
				windows = append(windows, window{id, scoreString(cue, "text"), start, end})
			}
		}
	}
	pieces := append([]editor.Object(nil), scoreObjects(item["words"])...)
	sort.SliceStable(pieces, func(i, j int) bool {
		a, b := scoreNumber(pieces[i], "startFrame", 0), scoreNumber(pieces[j], "startFrame", 0)
		if a == b {
			return scoreNumber(pieces[i], "endFrame", 0) < scoreNumber(pieces[j], "endFrame", 0)
		}
		return a < b
	})
	if len(cues) == 0 && len(pieces) > 0 {
		texts := make([]string, 0, len(pieces))
		for _, piece := range pieces {
			if text := strings.TrimSpace(scoreString(piece, "text")); text != "" {
				texts = append(texts, text)
			}
		}
		windows = append(windows, window{"words", strings.Join(texts, " "), max(0, int(scoreNumber(pieces[0], "startFrame", 0))), duration})
	}
	pages := []editor.CaptionPage{}
	index := 0
	for _, win := range windows {
		// A piece belongs to the window containing its start, never to the
		// next window merely because its end overlaps that window.
		for index < len(pieces) && int(scoreNumber(pieces[index], "startFrame", 0)) < win.start {
			index++
		}
		first := index
		for index < len(pieces) && int(scoreNumber(pieces[index], "startFrame", 0)) < win.end {
			index++
		}
		tokens := []editor.CaptionPageWord{}
		if index == first {
			if strings.TrimSpace(win.text) != "" && !strings.ContainsAny(win.text, "<{}") {
				tokens = scoreDistributeTokens(strings.Fields(win.text), win.start, win.end, true)
			}
		} else {
			for _, piece := range pieces[first:index] {
				start := min(max(int(scoreNumber(piece, "startFrame", 0)), win.start), win.end)
				end := min(max(int(scoreNumber(piece, "endFrame", 0)), win.start), win.end)
				if end <= start {
					continue
				}
				parts := strings.Fields(scoreString(piece, "text"))
				if len(parts) == 1 {
					tokens = append(tokens, editor.CaptionPageWord{Text: parts[0], StartFrame: start, EndFrame: end, Estimated: scoreBool(piece, "estimated", false)})
				} else {
					tokens = append(tokens, scoreDistributeTokens(parts, start, end, scoreBool(piece, "estimated", false))...)
				}
			}
		}
		groups := [][]editor.CaptionPageWord{}
		group := []editor.CaptionPageWord{}
		chars := 0
		for _, token := range tokens {
			added := scoreUTF16Length(token.Text)
			pause := false
			if len(group) > 0 {
				added++
				previous := group[len(group)-1]
				pause = !estimated && !previous.Estimated && !token.Estimated && float64(token.StartFrame-previous.EndFrame)*1000/fps >= 350
			}
			if len(group) > 0 && (pause || len(group) >= 7 || chars+added > 42) {
				groups = append(groups, group)
				group = nil
				chars = 0
			}
			group = append(group, token)
			chars += scoreUTF16Length(token.Text)
			if len(group) > 1 {
				chars++
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
		if len(groups) == 0 {
			if strings.TrimSpace(win.text) != "" {
				pages = append(pages, editor.CaptionPage{ID: win.id, Text: win.text, StartFrame: win.start, EndFrame: win.end, Words: []editor.CaptionPageWord{}, Estimated: estimated})
			}
			continue
		}
		for i, words := range groups {
			end := win.end
			last := words[len(words)-1]
			if i+1 < len(groups) {
				next := groups[i+1][0]
				end = next.StartFrame
				if !estimated && !last.Estimated && !next.Estimated && float64(next.StartFrame-last.EndFrame)*1000/fps >= 350 {
					end = last.EndFrame
				}
			}
			id := win.id
			if i > 0 {
				id = fmt.Sprintf("%s.%d", id, i+1)
			}
			texts := make([]string, len(words))
			pageEstimated := estimated
			for j, word := range words {
				texts[j] = word.Text
				pageEstimated = pageEstimated || word.Estimated
			}
			pages = append(pages, editor.CaptionPage{ID: id, Text: strings.Join(texts, " "), StartFrame: words[0].StartFrame, EndFrame: end, Words: words, Estimated: pageEstimated})
		}
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
			texts := make([]string, len(pages[neighbor].Words))
			for j, word := range pages[neighbor].Words {
				texts[j] = word.Text
			}
			pages[neighbor].Text = strings.Join(texts, " ")
			pages[neighbor].Estimated = pages[neighbor].Estimated || pages[i].Estimated
		}
		pages = append(pages[:i], pages[i+1:]...)
	}
	for i := range max(0, len(pages)-1) {
		gap := pages[i+1].StartFrame - pages[i].EndFrame
		if gap > 0 && float64(gap)*1000/fps < 350 {
			pages[i].EndFrame = pages[i+1].StartFrame
		}
	}
	return pages
}

func scoreUTF16Length(text string) int {
	length := 0
	for _, character := range text {
		length++
		if character > 0xffff {
			length++
		}
	}
	return length
}

func scoreDistributeTokens(parts []string, start, end int, estimated bool) []editor.CaptionPageWord {
	total := 0
	for _, part := range parts {
		total += scoreUTF16Length(part)
	}
	if total == 0 || end <= start {
		return nil
	}
	words := make([]editor.CaptionPageWord, 0, len(parts))
	consumed := 0
	for i, part := range parts {
		left := start + (end-start)*consumed/total
		consumed += scoreUTF16Length(part)
		right := end
		if i+1 < len(parts) {
			right = start + (end-start)*consumed/total
		}
		if right > left {
			words = append(words, editor.CaptionPageWord{Text: part, StartFrame: left, EndFrame: right, Estimated: estimated})
		}
	}
	return words
}
