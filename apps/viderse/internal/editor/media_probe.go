package editor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type ProbeResult struct {
	Kind        string   `json:"kind"`
	MIME        string   `json:"mime"`
	DurationSec *float64 `json:"durationSec"`
	Width       *float64 `json:"width"`
	Height      *float64 `json:"height"`
	FPS         *float64 `json:"fps"`
	VideoCodec  *string  `json:"videoCodec"`
	AudioCodec  *string  `json:"audioCodec"`
	SampleRate  *float64 `json:"sampleRate"`
	Channels    *float64 `json:"channels"`
}

func mediaNumber(n float64) *float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return &n
}
func mediaString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func mediaRound(n float64) *float64 { return mediaNumber(math.Round(n*1000) / 1000) }
func parseMediaNumber(s string) *float64 {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return nil
	}
	return mediaNumber(n)
}
func probeLottie(b []byte, mime string) (ProbeResult, error) {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	var o Object
	if e := json.Unmarshal(b, &o); e != nil {
		return ProbeResult{}, apiError("probe_failed", "Lottie JSON is malformed")
	}
	number := func(k string) (float64, bool) {
		n, ok := o[k].(float64)
		return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0)
	}
	fr, fok := number("fr")
	ip, iok := number("ip")
	op, ook := number("op")
	w, wok := number("w")
	h, hok := number("h")
	_, layers := o["layers"].([]any)
	if !fok || fr <= 0 || !iok || !ook || op < ip || !wok || w <= 0 || !hok || h <= 0 || !layers {
		return ProbeResult{}, apiError("unsupported_format", "JSON is not a valid Lottie animation")
	}
	return ProbeResult{Kind: "image", MIME: mime, DurationSec: mediaNumber((op - ip) / fr), Width: &w, Height: &h, FPS: &fr}, nil
}

const lottieJSONLimit int64 = 20 << 20
const dotLottieExpandedLimit int64 = 50 << 20

func probeDotLottie(file string) (ProbeResult, error) {
	z, e := zip.OpenReader(file)
	if e != nil {
		return ProbeResult{}, apiError("probe_failed", "dotLottie archive is malformed: "+e.Error())
	}
	defer z.Close()
	raw, e := os.Open(file)
	if e != nil {
		return ProbeResult{}, e
	}
	defer raw.Close()
	docs := map[string][]byte{}
	var declared, expanded int64
	for _, f := range z.File {
		if f.UncompressedSize64 > uint64(dotLottieExpandedLimit) {
			return ProbeResult{}, apiError("unsupported_format", "dotLottie exceeds the 50 MiB uncompressed limit")
		}
		declared += int64(f.UncompressedSize64)
		if declared > dotLottieExpandedLimit {
			return ProbeResult{}, apiError("unsupported_format", "dotLottie exceeds the 50 MiB uncompressed limit")
		}
		offset, e := f.DataOffset()
		if e != nil {
			return ProbeResult{}, apiError("probe_failed", e.Error())
		}
		header := make([]byte, 30)
		start := offset - int64(30+len(f.Name)+len(f.Extra))
		if start >= 0 {
			if _, e = raw.ReadAt(header, start); e == nil && bytes.Equal(header[:4], []byte{'P', 'K', 3, 4}) && binary.LittleEndian.Uint32(header[22:26]) > uint32(dotLottieExpandedLimit) {
				return ProbeResult{}, apiError("unsupported_format", "dotLottie exceeds the 50 MiB uncompressed limit")
			}
		}
		r, e := f.Open()
		if e != nil {
			return ProbeResult{}, apiError("probe_failed", e.Error())
		}
		retain := f.Name == "manifest.json" || (strings.HasPrefix(f.Name, "animations/") && strings.HasSuffix(f.Name, ".json") && !strings.Contains(strings.TrimPrefix(f.Name, "animations/"), "/"))
		cap := dotLottieExpandedLimit - expanded
		if retain && cap > lottieJSONLimit {
			cap = lottieJSONLimit
		}
		var b []byte
		var n int64
		if retain {
			b, e = io.ReadAll(io.LimitReader(r, cap+1))
			n = int64(len(b))
		} else {
			n, e = io.Copy(io.Discard, io.LimitReader(r, cap+1))
		}
		closeErr := r.Close()
		if n > cap {
			return ProbeResult{}, apiError("unsupported_format", "dotLottie exceeds its uncompressed JSON or archive limit")
		}
		if e != nil {
			return ProbeResult{}, apiError("probe_failed", e.Error())
		}
		if closeErr != nil {
			return ProbeResult{}, closeErr
		}
		expanded += n
		if retain {
			docs[f.Name] = b
		}
	}
	manifest, ok := docs["manifest.json"]
	if !ok {
		return ProbeResult{}, apiError("unsupported_format", "ZIP is not a dotLottie archive")
	}
	var m struct {
		Animations []struct {
			ID string `json:"id"`
		} `json:"animations"`
	}
	if e = json.Unmarshal(manifest, &m); e != nil {
		return ProbeResult{}, apiError("probe_failed", "Lottie JSON is malformed")
	}
	if len(m.Animations) == 0 || m.Animations[0].ID == "" {
		return ProbeResult{}, apiError("unsupported_format", "dotLottie manifest has no animation")
	}
	animation, ok := docs["animations/"+m.Animations[0].ID+".json"]
	if !ok {
		return ProbeResult{}, apiError("probe_failed", "dotLottie manifest references a missing animation")
	}
	return probeLottie(animation, "application/zip+dotlottie")
}
func ProbeMediaFile(ctx context.Context, file string, runner CommandRunner, ffprobe string) (ProbeResult, error) {
	f, e := os.Open(file)
	if e != nil {
		return ProbeResult{}, apiError("probe_failed", "File is not readable")
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || !stat.Mode().IsRegular() {
		return ProbeResult{}, apiError("probe_failed", "File is not readable")
	}
	if stat.Size() == 0 {
		return ProbeResult{}, apiError("probe_failed", "File is empty")
	}
	head := make([]byte, min(stat.Size(), 256<<10))
	if _, e = io.ReadFull(f, head); e != nil {
		return ProbeResult{}, apiError("probe_failed", e.Error())
	}
	if config, format, e := image.DecodeConfig(bytes.NewReader(head)); e == nil {
		mime := "image/" + format
		if format == "jpeg" {
			mime = "image/jpeg"
		}
		return ProbeResult{Kind: "image", MIME: mime, Width: mediaNumber(float64(config.Width)), Height: mediaNumber(float64(config.Height))}, nil
	}
	if bytes.HasPrefix(head, []byte{'P', 'K', 3, 4}) {
		return probeDotLottie(file)
	}
	trim := bytes.TrimSpace(bytes.TrimPrefix(head, []byte{0xef, 0xbb, 0xbf}))
	if len(trim) == 0 || trim[0] == '{' {
		if stat.Size() > lottieJSONLimit {
			return ProbeResult{}, apiError("unsupported_format", "Lottie JSON exceeds the 20 MiB read limit")
		}
		if int64(len(head)) == stat.Size() {
			return probeLottie(head, "application/lottie+json")
		}
		b := make([]byte, stat.Size())
		copy(b, head)
		if _, e = io.ReadFull(f, b[len(head):]); e != nil {
			return ProbeResult{}, apiError("probe_failed", e.Error())
		}
		return probeLottie(b, "application/lottie+json")
	}
	mime := http.DetectContentType(head)
	if bytes.HasPrefix(head, []byte{'I', 'I', 42, 0}) || bytes.HasPrefix(head, []byte{'M', 'M', 0, 42}) {
		mime = "image/tiff"
	}
	container := mediaContainerMIME(head)
	if container == "" && !strings.HasPrefix(mime, "image/") {
		return ProbeResult{}, apiError("unsupported_format", "Unsupported media format")
	}
	if runner == nil {
		runner = &tools.ExecRunner{}
	}
	if ffprobe == "" {
		ffprobe = "ffprobe"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, e := runner.Run(ctx, ffprobe, []string{"-v", "error", "-print_format", "json", "-show_format", "-show_streams", file}, tools.Spec{Timeout: 5 * time.Second, LogLimit: 1 << 20})
	if e != nil {
		return ProbeResult{}, apiError("probe_failed", "Media probe failed: "+e.Error())
	}
	var out struct {
		Format struct {
			Duration string `json:"duration"`
			Name     string `json:"format_name"`
		} `json:"format"`
		Streams []struct {
			Type          string `json:"codec_type"`
			Codec         string `json:"codec_name"`
			Width, Height int
			Rate          string `json:"avg_frame_rate"`
			SampleRate    string `json:"sample_rate"`
			Channels      int
			Duration      string `json:"duration"`
			Disposition   struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if e = json.Unmarshal([]byte(res.Stdout), &out); e != nil {
		return ProbeResult{}, apiError("probe_failed", "Invalid ffprobe response")
	}
	p := ProbeResult{MIME: container, DurationSec: parseMediaNumber(out.Format.Duration)}
	if strings.HasPrefix(mime, "image/") {
		p.Kind = "image"
		p.MIME = mime
		p.DurationSec = nil
	}
	for _, stream := range out.Streams {
		if p.DurationSec == nil && p.Kind != "image" {
			p.DurationSec = parseMediaNumber(stream.Duration)
		}
		switch stream.Type {
		case "video":
			if stream.Disposition.AttachedPic != 0 || p.VideoCodec != nil {
				continue
			}
			if p.Kind != "image" {
				p.Kind = "video"
				p.VideoCodec = mediaString(stream.Codec)
				parts := strings.Split(stream.Rate, "/")
				if len(parts) == 2 {
					a, e1 := strconv.ParseFloat(parts[0], 64)
					b, e2 := strconv.ParseFloat(parts[1], 64)
					if e1 == nil && e2 == nil && b > 0 {
						p.FPS = mediaRound(a / b)
					}
				}
			}
			if stream.Width > 0 {
				p.Width = mediaNumber(float64(stream.Width))
			}
			if stream.Height > 0 {
				p.Height = mediaNumber(float64(stream.Height))
			}
		case "audio":
			if p.AudioCodec != nil {
				continue
			}
			codec := stream.Codec
			if strings.HasPrefix(codec, "pcm_") {
				codec = "pcm"
			}
			p.AudioCodec = mediaString(codec)
			p.SampleRate = parseMediaNumber(stream.SampleRate)
			if stream.Channels > 0 {
				p.Channels = mediaNumber(float64(stream.Channels))
			}
		}
	}
	if p.Kind == "" && p.AudioCodec != nil {
		p.Kind = "audio"
		if p.MIME == "video/mp4" {
			p.MIME = "audio/mp4"
		}
		if p.MIME == "video/webm" {
			p.MIME = "audio/webm"
		}
	}
	if p.Kind == "" {
		return ProbeResult{}, apiError("probe_failed", "Container has no audio or video tracks")
	}
	if p.DurationSec != nil {
		p.DurationSec = mediaRound(*p.DurationSec)
	}
	return p, nil
}
func mediaContainerMIME(h []byte) string {
	switch {
	case len(h) >= 8 && (string(h[4:8]) == "ftyp" || string(h[4:8]) == "moov" || string(h[4:8]) == "free" || string(h[4:8]) == "mdat"):
		if len(h) >= 12 && string(h[8:12]) == "qt  " {
			return "video/quicktime"
		}
		return "video/mp4"
	case bytes.HasPrefix(h, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		return "video/webm"
	case bytes.HasPrefix(h, []byte("fLaC")):
		return "audio/flac"
	case bytes.HasPrefix(h, []byte("OggS")):
		return "audio/ogg"
	case len(h) >= 12 && string(h[:4]) == "RIFF" && string(h[8:12]) == "WAVE":
		return "audio/wav"
	case bytes.HasPrefix(h, []byte("ID3")) || (len(h) > 1 && h[0] == 0xff && h[1]&0xe0 == 0xe0):
		return "audio/mpeg"
	}
	return ""
}
