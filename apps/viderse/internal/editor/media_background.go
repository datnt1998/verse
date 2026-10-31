package editor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const MediaBackgroundVersion = 1
const BackgroundSampleWidth = 64
const BackgroundSampleHeight = 36
const BackgroundSampleCount = 8

type MediaBackgroundDetection struct {
	Kind     string `json:"kind"`
	KeyColor string `json:"keyColor,omitempty"`
}

func ClassifyBackground(frames []byte, width, height int) MediaBackgroundDetection {
	none := MediaBackgroundDetection{Kind: "none"}
	if width <= 0 || height <= 0 || width > len(frames)/3 || height > len(frames)/3/width {
		return none
	}
	frameBytes := width * height * 3
	var border, total, darkBorder, darkTotal, greenBorder int
	var rs, gs, bs [256]int
	for frame := range len(frames) / frameBytes {
		for y := range height {
			for x := range width {
				offset := frame*frameBytes + (y*width+x)*3
				r, g, b := float64(frames[offset]), float64(frames[offset+1]), float64(frames[offset+2])
				dark := .299*r+.587*g+.114*b < 24
				total++
				if dark {
					darkTotal++
				}
				if !(y < 4 || y >= height-4 || x < 4 || x >= width-4) {
					continue
				}
				border++
				if dark {
					darkBorder++
				}
				if g > 1.4*r && g > 1.4*b && g > 70 {
					greenBorder++
					rs[int(r)]++
					gs[int(g)]++
					bs[int(b)]++
				}
			}
		}
	}
	if border > 0 && float64(darkBorder)/float64(border) >= .85 && float64(darkTotal)/float64(total) >= .5 {
		return MediaBackgroundDetection{Kind: "black"}
	}
	if border > 0 && float64(greenBorder)/float64(border) >= .7 {
		median := func(hist [256]int) int {
			n := 0
			for v, count := range hist {
				n += count
				if n > greenBorder/2 {
					return v
				}
			}
			return 0
		}
		return MediaBackgroundDetection{Kind: "green", KeyColor: fmt.Sprintf("#%02x%02x%02x", median(rs), median(gs), median(bs))}
	}
	return none
}
func (s *Service) sampleMediaFrames(ctx context.Context, file string, duration *float64) ([]byte, error) {
	rate := "1"
	if duration != nil && *duration > 0 {
		rate = strconv.FormatFloat(BackgroundSampleCount / *duration, 'f', 6, 64)
	}
	runner := s.Runner
	if runner == nil {
		runner = &tools.ExecRunner{}
	}
	ffmpeg := s.FFmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	result, e := runner.Run(ctx, ffmpeg, []string{"-v", "error", "-i", file, "-vf", fmt.Sprintf("fps=%s,scale=%d:%d", rate, BackgroundSampleWidth, BackgroundSampleHeight), "-frames:v", strconv.Itoa(BackgroundSampleCount), "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1"}, tools.Spec{Timeout: 5 * time.Minute, LogLimit: BackgroundSampleWidth*BackgroundSampleHeight*3*BackgroundSampleCount + 65536})
	if e != nil {
		return nil, e
	}
	return []byte(result.Stdout), nil
}
func (s *Service) SaveMediaBackground(ctx context.Context, id string, detection MediaBackgroundDetection, hash string, overlay *bool) (MediaRecord, error) {
	if e := ValidateID(id); e != nil {
		return MediaRecord{}, e
	}
	if detection.Kind != "black" && detection.Kind != "green" && detection.Kind != "none" {
		return MediaRecord{}, apiError("invalid_request", "Invalid background kind")
	}
	l := resourceLock("media/" + id)
	l.Lock()
	defer l.Unlock()
	m, rev, e := s.readMedia(ctx, id)
	if e != nil {
		return MediaRecord{}, e
	}
	if m.ContentHash != hash {
		return MediaRecord{}, apiError("conflict", "Media changed while its background was being detected")
	}
	m.Background = &MediaBackground{Kind: detection.Kind, KeyColor: detection.KeyColor, Version: MediaBackgroundVersion, DetectedAt: s.now().UnixMilli(), ContentHash: hash}
	if overlay != nil {
		m.Overlay = *overlay
	}
	m.UpdatedAt = s.now().UnixMilli()
	e = s.saveMedia(ctx, m, rev)
	return m.MediaRecord, e
}
func (s *Service) DetectMediaBackground(ctx context.Context, id string, force bool, overlay *bool) (MediaRecord, error) {
	m, e := s.GetMediaMetadata(ctx, id)
	if e != nil {
		return MediaRecord{}, e
	}
	if m.Probe.Kind != "video" && m.Probe.Kind != "image" {
		return MediaRecord{}, apiError("invalid_request", "Chỉ video hoặc ảnh mới dò được nền")
	}
	if !force && m.Background != nil && m.Background.Version == MediaBackgroundVersion && m.Background.ContentHash == m.ContentHash {
		if overlay != nil && m.Overlay != *overlay {
			return s.UpdateMediaMetadata(ctx, id, Object{"overlay": *overlay})
		}
		return m, nil
	}
	for attempt := range 2 {
		h, e := s.AcquireMedia(ctx, id)
		if e != nil {
			return MediaRecord{}, e
		}
		frames, e := s.sampleMediaFrames(ctx, h.Path, m.Probe.DurationSec)
		h.Release()
		if e != nil {
			return MediaRecord{}, e
		}
		result, e := s.SaveMediaBackground(ctx, id, ClassifyBackground(frames, BackgroundSampleWidth, BackgroundSampleHeight), m.ContentHash, overlay)
		if e == nil {
			return result, nil
		}
		var conflict *Error
		if attempt != 0 || !errors.As(e, &conflict) || conflict.Code != "conflict" {
			return MediaRecord{}, e
		}
		m, e = s.GetMediaMetadata(ctx, id)
		if e != nil {
			return MediaRecord{}, e
		}
	}
	return MediaRecord{}, apiError("internal_error", "Không dò được nền media")
}
func sortMediaNewest(records []MediaRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].ID < records[j].ID
		}
		return records[i].CreatedAt > records[j].CreatedAt
	})
}
