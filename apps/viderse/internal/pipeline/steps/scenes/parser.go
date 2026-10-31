package scenes

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

const DefaultFPS = 30.
const DefaultThreshold = 10.
const DefaultMinSceneSec = .2
const DefaultSceneCount = 5

type DetectFrame struct {
	Index          int
	PTSSec         float64
	Score, SCDTime *float64
}
type Candidate struct {
	TimeSec    float64 `json:"timeSec"`
	Score      float64 `json:"score"`
	FrameIndex *int    `json:"frameIndex"`
}
type Dropped struct {
	TimeSec *float64 `json:"timeSec"`
	Score   float64  `json:"score"`
	Reason  string   `json:"reason"`
}
type Normalized struct {
	Boundaries []float64
	Kept       []Candidate
	Dropped    []Dropped
}
type Span struct {
	Index      int     `json:"index"`
	StartSec   float64 `json:"startSec"`
	EndSec     float64 `json:"endSec"`
	StartFrame int     `json:"startFrame"`
	EndFrame   int     `json:"endFrame"`
	Frames     int     `json:"frames"`
}
type Cue struct {
	ID      string   `json:"id"`
	StartMS float64  `json:"startMs"`
	EndMS   *float64 `json:"endMs"`
	Text    string   `json:"text"`
}
type TextContext struct {
	CueID    string `json:"cueId"`
	Position string `json:"position"`
	Text     string `json:"text"`
}
type TextAssignment struct {
	Text    string       `json:"text"`
	CueIDs  []string     `json:"cueIds"`
	Context *TextContext `json:"context"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// ReadSceneMetadata retains only cut records when threshold is nonnegative, so
// memory for a long decode grows with cuts rather than every decoded frame.
func ReadSceneMetadata(r io.Reader, threshold float64) ([]DetectFrame, error) {
	out := []DetectFrame{}
	var current *DetectFrame
	flush := func() {
		if current != nil && (threshold < 0 || (current.Score != nil && *current.Score >= threshold) || current.SCDTime != nil) {
			out = append(out, *current)
		}
	}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if strings.HasPrefix(line, "frame:") {
			flush()
			current = nil
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			idx, err := strconv.Atoi(strings.TrimPrefix(fields[0], "frame:"))
			if err != nil {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimPrefix(fields[2], "pts_time:"), 64)
			if err != nil || !finite(v) {
				continue
			}
			current = &DetectFrame{Index: idx, PTSSec: v}
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil || !finite(v) {
			continue
		}
		switch key {
		case "lavfi.scd.score", "lavfi.scdet.score":
			current.Score = new(v)
		case "lavfi.scd.time", "lavfi.scdet.time":
			current.SCDTime = new(v)
		}
	}
	flush()
	return out, scan.Err()
}
func ParseSceneMetadata(text string) []DetectFrame {
	out, _ := ReadSceneMetadata(strings.NewReader(text), -1)
	return out
}
func SceneCandidates(frames []DetectFrame, threshold float64) []Candidate {
	out := []Candidate{}
	for i, f := range frames {
		if f.Score != nil && *f.Score >= threshold {
			out = append(out, Candidate{f.PTSSec, *f.Score, new(f.Index)})
		}
		if f.SCDTime != nil && *f.SCDTime != f.PTSSec && i > 0 {
			prev := frames[i-1]
			if prev.PTSSec == *f.SCDTime && prev.Score != nil && *prev.Score >= threshold {
				out = append(out, Candidate{*f.SCDTime, *prev.Score, new(prev.Index)})
			}
		}
	}
	return out
}
func NormalizeBoundaries(candidates []Candidate, duration, minSec, offset, fps float64) Normalized {
	minSec = math.Max(minSec, 1/fps)
	out := Normalized{Boundaries: []float64{}, Kept: []Candidate{}, Dropped: []Dropped{}}
	shifted := []Candidate{}
	drop := func(c Candidate, reason string) {
		var at *float64
		if finite(c.TimeSec) {
			at = new(c.TimeSec)
		}
		out.Dropped = append(out.Dropped, Dropped{at, c.Score, reason})
	}
	for _, c := range candidates {
		c.TimeSec -= offset
		if !finite(c.TimeSec) {
			drop(c, "nonfinite")
		} else if c.TimeSec <= .001 || c.TimeSec >= duration-.001 {
			drop(c, "edge")
		} else {
			shifted = append(shifted, c)
		}
	}
	sort.SliceStable(shifted, func(i, j int) bool {
		if shifted[i].TimeSec == shifted[j].TimeSec {
			return shifted[i].Score > shifted[j].Score
		}
		return shifted[i].TimeSec < shifted[j].TimeSec
	})
	dedup := []Candidate{}
	for _, c := range shifted {
		n := len(dedup)
		if n > 0 && c.TimeSec-dedup[n-1].TimeSec <= .001 {
			if c.Score > dedup[n-1].Score {
				drop(dedup[n-1], "duplicate")
				dedup[n-1] = c
			} else {
				drop(c, "duplicate")
			}
		} else {
			dedup = append(dedup, c)
		}
	}
	for _, c := range dedup {
		n := len(out.Kept)
		if n > 0 && c.TimeSec-out.Kept[n-1].TimeSec < minSec {
			if c.Score > out.Kept[n-1].Score {
				drop(out.Kept[n-1], "below-min-scene")
				out.Kept[n-1] = c
			} else {
				drop(c, "below-min-scene")
			}
		} else {
			out.Kept = append(out.Kept, c)
		}
	}
	for _, c := range out.Kept {
		out.Boundaries = append(out.Boundaries, c.TimeSec)
	}
	return out
}
func SelectTargetBoundaries(n Normalized, duration float64, count int, minSec, fps float64) Normalized {
	minSec = math.Max(minSec, 1/fps)
	count = max(1, min(count, int(math.Floor(duration/(2*minSec)))))
	step := duration / float64(count)
	used := map[int]bool{}
	out := Normalized{Boundaries: []float64{}, Kept: []Candidate{}, Dropped: append([]Dropped{}, n.Dropped...)}
	for i := 1; i < count; i++ {
		ideal := step * float64(i)
		prev := 0.
		if len(out.Boundaries) > 0 {
			prev = out.Boundaries[len(out.Boundaries)-1]
		}
		best := -1
		for j, c := range n.Kept {
			distance := math.Abs(c.TimeSec - ideal)
			if used[j] || distance > step/2 || c.TimeSec-prev < minSec {
				continue
			}
			if best < 0 || distance < math.Abs(n.Kept[best].TimeSec-ideal) {
				best = j
			}
		}
		if best >= 0 {
			used[best] = true
			out.Kept = append(out.Kept, n.Kept[best])
			out.Boundaries = append(out.Boundaries, n.Kept[best].TimeSec)
		} else {
			out.Boundaries = append(out.Boundaries, ideal)
		}
	}
	for i, c := range n.Kept {
		if !used[i] {
			out.Dropped = append(out.Dropped, Dropped{new(c.TimeSec), c.Score, "target-count"})
		}
	}
	return out
}
func SpansFromBoundaries(boundaries []float64, duration, fps float64) ([]Span, int, error) {
	if !finite(duration) || duration <= 0 || !finite(fps) || fps <= 0 {
		return nil, 0, fmt.Errorf("duration and fps must be finite and positive")
	}
	total := int(math.Round(duration * fps))
	if total < 1 {
		return nil, 0, fmt.Errorf("source shorter than a rounded frame")
	}
	sorted := append([]float64{}, boundaries...)
	sort.Float64s(sorted)
	frames := []int{0}
	times := []float64{0}
	for _, t := range sorted {
		if !finite(t) || t <= 0 || t >= duration {
			continue
		}
		f := int(math.Round(t * fps))
		if f <= frames[len(frames)-1] || f >= total {
			continue
		}
		frames = append(frames, f)
		times = append(times, t)
	}
	frames = append(frames, total)
	times = append(times, duration)
	spans := make([]Span, 0, len(frames)-1)
	for i := 0; i < len(frames)-1; i++ {
		spans = append(spans, Span{i, times[i], times[i+1], frames[i], frames[i+1], frames[i+1] - frames[i]})
	}
	return spans, total, nil
}
func AssignSceneText(start, end float64, cues []Cue) TextAssignment {
	start *= 1000
	end *= 1000
	overlap := []Cue{}
	var prev, next *Cue
	prevDist, nextDist := math.Inf(1), math.Inf(1)
	for _, cue := range cues {
		if cue.EndMS != nil && cue.StartMS < end && *cue.EndMS > start {
			overlap = append(overlap, cue)
			continue
		}
		anchor := cue.StartMS
		if cue.EndMS != nil {
			anchor = *cue.EndMS
		}
		if anchor <= start && start-anchor < prevDist {
			prev = new(cue)
			prevDist = start - anchor
		} else if cue.StartMS >= end && cue.StartMS-end < nextDist {
			next = new(cue)
			nextDist = cue.StartMS - end
		}
	}
	sort.SliceStable(overlap, func(i, j int) bool { return overlap[i].StartMS < overlap[j].StartMS })
	out := TextAssignment{CueIDs: []string{}}
	parts := []string{}
	for _, c := range overlap {
		out.CueIDs = append(out.CueIDs, c.ID)
		if text := strings.TrimSpace(c.Text); text != "" {
			parts = append(parts, text)
		}
	}
	out.Text = strings.Join(parts, " ")
	if len(overlap) == 0 {
		if prev != nil && prevDist <= nextDist {
			out.Context = &TextContext{prev.ID, "previous", prev.Text}
		} else if next != nil {
			out.Context = &TextContext{next.ID, "next", next.Text}
		}
	}
	return out
}
