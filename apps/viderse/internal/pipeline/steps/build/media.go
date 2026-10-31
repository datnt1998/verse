package build

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type mediaFacts struct {
	Width, Height int
	Duration      float64
	Channels      int
}

func (h Handler) probe(ctx context.Context, s *pipeline.StepContext, file string) (mediaFacts, error) {
	if s.ProcessRunner == nil || h.Deps.FFprobe == "" {
		return mediaFacts{}, fmt.Errorf("missing ffprobe")
	}
	r, err := s.RunProcess(ctx, h.Deps.FFprobe, []string{"-v", "error", "-show_streams", "-show_format", "-of", "json", file}, tools.Spec{Timeout: time.Minute, LogLimit: 1 << 20})
	if err != nil {
		return mediaFacts{}, err
	}
	var d struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType               string `json:"codec_type"`
			Width, Height, Channels int
		} `json:"streams"`
	}
	if err = json.Unmarshal([]byte(r.Stdout), &d); err != nil {
		return mediaFacts{}, err
	}
	out := mediaFacts{}
	out.Duration, _ = strconv.ParseFloat(d.Format.Duration, 64)
	for _, stream := range d.Streams {
		if stream.CodecType == "video" {
			out.Width, out.Height = stream.Width, stream.Height
		}
		if stream.CodecType == "audio" {
			out.Channels = stream.Channels
		}
	}
	return out, nil
}
func mimeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	}
	return "application/octet-stream"
}
func rewriteMedia(value any, oldID, newID, src string) {
	switch v := value.(type) {
	case map[string]any:
		if v["coverMediaId"] == oldID {
			v["coverMediaId"] = newID
		}
		if v["mediaId"] == oldID {
			v["mediaId"] = newID
			if v["type"] != "shape" {
				v["src"] = src
				if v["type"] == "audio" || v["type"] == "video" {
					v["audioSrc"] = src
				}
			}
		}
		for _, child := range v {
			rewriteMedia(child, oldID, newID, src)
		}
	case []any:
		for _, child := range v {
			rewriteMedia(child, oldID, newID, src)
		}
	}
}
func sourceNumber(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}
func findMediaGeometry(value any, id string) (int, int, float64) {
	switch v := value.(type) {
	case map[string]any:
		if v["mediaId"] == id {
			fps := sourceNumber(v, "sourceFps")
			if fps <= 0 {
				fps = 30
			}
			return int(sourceNumber(v, "sourceWidth")), int(sourceNumber(v, "sourceHeight")), sourceNumber(v, "sourceDuration") / fps
		}
		for _, child := range v {
			w, h, d := findMediaGeometry(child, id)
			if w > 0 || h > 0 || d > 0 {
				return w, h, d
			}
		}
	case []any:
		for _, child := range v {
			w, h, d := findMediaGeometry(child, id)
			if w > 0 || h > 0 || d > 0 {
				return w, h, d
			}
		}
	}
	return 0, 0, 0
}
func canonicalBackdrop(id, src string, facts mediaFacts, frames int) editor.Object {
	return editor.Object{"id": "pool-" + id, "type": "video", "mediaId": id, "src": src, "label": "Nền video", "from": 0, "durationInFrames": frames, "sourceStart": 0, "sourceEnd": int(math.Round(facts.Duration * 30)), "sourceDuration": int(math.Round(facts.Duration * 30)), "sourceFps": 30, "sourceWidth": facts.Width, "sourceHeight": facts.Height, "embeddedAudioMuted": true}
}

func imageGeometry(file string) (int, int, error) {
	input, err := os.Open(file)
	if err != nil {
		return 0, 0, err
	}
	config, _, decodeErr := image.DecodeConfig(input)
	closeErr := input.Close()
	if decodeErr != nil {
		return 0, 0, decodeErr
	}
	if closeErr != nil {
		return 0, 0, closeErr
	}
	if config.Width <= 0 || config.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid image dimensions")
	}
	return config.Width, config.Height, nil
}

func measureSubjectBox(file string) (*editor.SubjectBox, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	left, top, right, bottom := bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha > 16*257 {
				left = min(left, x)
				top = min(top, y)
				right = max(right, x+1)
				bottom = max(bottom, y+1)
			}
		}
	}
	if right <= left || bottom <= top {
		return nil, fmt.Errorf("no foreground subject")
	}
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	return &editor.SubjectBox{X: float64(left-bounds.Min.X) / w, Y: float64(top-bounds.Min.Y) / h, Width: float64(right-left) / w, Height: float64(bottom-top) / h}, nil
}
