// Package download fetches and normalizes the pipeline's source video.
package download

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type SourceFormat struct {
	FormatID       string   `json:"format_id"`
	Ext            string   `json:"ext"`
	Vcodec         string   `json:"vcodec"`
	Acodec         string   `json:"acodec"`
	Filesize       *float64 `json:"filesize"`
	FilesizeApprox *float64 `json:"filesize_approx"`
}
type SubtitleFormat struct {
	Ext string `json:"ext"`
}
type SubtitleTrack struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
}
type SourceChapter struct {
	StartTime *float64 `json:"start_time"`
	Title     string   `json:"title"`
}
type SourceInfo struct {
	Filesize          *float64                    `json:"filesize"`
	FilesizeApprox    *float64                    `json:"filesize_approx"`
	Ext               string                      `json:"ext"`
	Vcodec            string                      `json:"vcodec"`
	Acodec            string                      `json:"acodec"`
	ID                string                      `json:"id"`
	Timestamp         *float64                    `json:"timestamp"`
	UploadDate        *string                     `json:"upload_date"`
	Language          *string                     `json:"language"`
	Description       *string                     `json:"description"`
	Tags              []string                    `json:"tags"`
	Chapters          []SourceChapter             `json:"chapters"`
	Subtitles         map[string][]SubtitleFormat `json:"subtitles"`
	AutomaticCaptions map[string][]SubtitleFormat `json:"automatic_captions"`
	Title             string                      `json:"title"`
	Duration          *float64                    `json:"duration"`
	IsLive            *bool                       `json:"is_live"`
	LiveStatus        string                      `json:"live_status"`
	IsFromBeginning   *bool                       `json:"is_from_beginning"`
	Formats           []SourceFormat              `json:"formats"`
	RequestedFormats  []SourceFormat              `json:"requested_formats"`
}

type ProbeSummary struct {
	DurationSec  *float64 `json:"durationSec"`
	FormatName   string   `json:"formatName"`
	VideoCodec   string   `json:"videoCodec"`
	AudioCodec   string   `json:"audioCodec"`
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	FPS          *float64 `json:"fps"`
	VariableFPS  bool     `json:"variableFps"`
	RotationDeg  *float64 `json:"rotationDeg"`
	StartTimeSec *float64 `json:"startTimeSec"`
	HasAudio     bool     `json:"hasAudio"`
}
type DownloadProgress struct {
	Status                                                string
	DownloadedBytes, TotalBytes, SpeedBytesPerSec, EtaSec *float64
}

const progressPrefix = "YTFPROG|"

func finiteNumber(raw string) *float64 {
	if raw == "" || raw == "NA" {
		return nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return &n
}
func ParseDownloadProgress(line string) *DownloadProgress {
	if !strings.HasPrefix(line, progressPrefix) {
		return nil
	}
	fields := strings.Split(strings.TrimPrefix(line, progressPrefix), "|")
	field := func(i int) string {
		if i < len(fields) {
			return fields[i]
		}
		return ""
	}
	total := finiteNumber(field(2))
	if total == nil {
		total = finiteNumber(field(3))
	}
	return &DownloadProgress{Status: field(0), DownloadedBytes: finiteNumber(field(1)), TotalBytes: total, SpeedBytesPerSec: finiteNumber(field(4)), EtaSec: finiteNumber(field(5))}
}
func parseRate(raw string) *float64 {
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return nil
	}
	n, d := finiteNumber(parts[0]), finiteNumber(parts[1])
	if n == nil || d == nil || *d == 0 {
		return nil
	}
	v := *n / *d
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return nil
	}
	return &v
}

type probeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        *int   `json:"width"`
	Height       *int   `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	RFrameRate   string `json:"r_frame_rate"`
	SampleRate   string `json:"sample_rate"`
	StartTime    string `json:"start_time"`
	Duration     string `json:"duration"`
	Tags         *struct {
		Rotate string `json:"rotate"`
	} `json:"tags"`
	SideData []struct {
		Rotation *float64 `json:"rotation"`
	} `json:"side_data_list"`
	Disposition *struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

func ParseFfprobeJSON(raw []byte) (ProbeSummary, error) {
	var doc struct {
		Format *struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			StartTime  string `json:"start_time"`
		} `json:"format"`
		Streams []probeStream `json:"streams"`
	}
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return ProbeSummary{}, fmt.Errorf("ffprobe không trả về tài liệu JSON")
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ProbeSummary{}, fmt.Errorf("đọc ffprobe: %w", err)
	}
	var p ProbeSummary
	if doc.Format != nil {
		p.FormatName = doc.Format.FormatName
		p.DurationSec = finiteNumber(doc.Format.Duration)
		p.StartTimeSec = finiteNumber(doc.Format.StartTime)
	}
	videoSeen := false
	for _, s := range doc.Streams {
		if s.CodecType == "audio" && !p.HasAudio {
			p.HasAudio = true
			p.AudioCodec = s.CodecName
		}
		if s.CodecType != "video" || videoSeen || (s.Disposition != nil && s.Disposition.AttachedPic == 1) {
			continue
		}
		videoSeen = true
		p.VideoCodec, p.Width, p.Height = s.CodecName, s.Width, s.Height
		if p.DurationSec == nil {
			p.DurationSec = finiteNumber(s.Duration)
		}
		if p.StartTimeSec == nil {
			p.StartTimeSec = finiteNumber(s.StartTime)
		}
		avg, base := parseRate(s.AvgFrameRate), parseRate(s.RFrameRate)
		if avg != nil {
			fps := math.Round(*avg*1000) / 1000
			p.FPS = &fps
		}
		if avg != nil && base != nil {
			p.VariableFPS = math.Abs(*avg-*base)/math.Max(*avg, *base) > .02
		}
		for _, side := range s.SideData {
			if side.Rotation != nil {
				p.RotationDeg = side.Rotation
				break
			}
		}
		if p.RotationDeg == nil && s.Tags != nil {
			p.RotationDeg = finiteNumber(s.Tags.Rotate)
		}
	}
	if p.DurationSec != nil && *p.DurationSec <= 0 {
		p.DurationSec = nil
	}
	return p, nil
}
func DecideCompat(p ProbeSummary) string {
	for _, c := range strings.Split(p.FormatName, ",") {
		switch c {
		case "mov", "mp4", "m4a", "m4v", "3gp", "3g2", "mj2", "ismv":
			if p.VideoCodec != "" {
				return "keep"
			}
		}
	}
	return "transcode"
}
func formatBytes(f SourceFormat) *float64 {
	if f.Filesize != nil {
		return f.Filesize
	}
	return f.FilesizeApprox
}
func ExpectedDownloadBytes(info SourceInfo) *float64 {
	if len(info.RequestedFormats) > 0 {
		total := 0.0
		for _, f := range info.RequestedFormats {
			size := formatBytes(f)
			if size == nil {
				return nil
			}
			total += *size
		}
		return &total
	}
	if len(info.Formats) > 0 {
		if size := formatBytes(info.Formats[0]); size != nil {
			return size
		}
	}
	if info.Filesize != nil {
		return info.Filesize
	}
	return info.FilesizeApprox
}
func LivestreamReason(info SourceInfo) string {
	if info.LiveStatus == "is_upcoming" {
		return "Video chưa phát — chờ buổi phát trực tiếp kết thúc"
	}
	if (info.IsLive != nil && *info.IsLive) || info.LiveStatus == "is_live" {
		return "Video đang phát trực tiếp — nguồn chưa có độ dài ổn định"
	}
	return ""
}
func SubLangArg(key string) string { return "^" + regexp.QuoteMeta(key) + "$" }

// Maps do not preserve yt-dlp object order; sorting makes fallback track selection
// stable across retries and process restarts.
func PickSubtitleTrack(manualTracks, autoTracks map[string][]SubtitleFormat, language *string) *SubtitleTrack {
	keys := func(tracks map[string][]SubtitleFormat) []string {
		var out []string
		for key, formats := range tracks {
			if key == "live_chat" {
				continue
			}
			for _, f := range formats {
				if f.Ext == "json3" {
					out = append(out, key)
					break
				}
			}
		}
		sort.Strings(out)
		return out
	}
	base := func(key string) string {
		parts := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool { return r == '-' || r == '_' })
		if len(parts) == 0 {
			return ""
		}
		return parts[0]
	}
	manual, auto := keys(manualTracks), keys(autoTracks)
	var orig []string
	for _, key := range auto {
		if strings.HasSuffix(key, "-orig") {
			orig = append(orig, key)
		}
	}
	spoken := ""
	if language != nil && *language != "" {
		spoken = base(*language)
	} else if len(orig) > 0 {
		spoken = base(orig[0])
	}
	if spoken != "" {
		if language != nil {
			for _, key := range manual {
				if strings.EqualFold(key, *language) {
					return &SubtitleTrack{Key: key, Kind: "manual"}
				}
			}
		}
		for _, key := range manual {
			if base(key) == spoken {
				return &SubtitleTrack{Key: key, Kind: "manual"}
			}
		}
	}
	for _, key := range orig {
		if base(key) == spoken {
			return &SubtitleTrack{Key: key, Kind: "auto"}
		}
	}
	if len(orig) > 0 {
		return &SubtitleTrack{Key: orig[0], Kind: "auto"}
	}
	for _, key := range auto {
		if key == spoken {
			return &SubtitleTrack{Key: key, Kind: "auto"}
		}
	}
	for _, lang := range []string{"en", "vi"} {
		for _, key := range manual {
			if base(key) == lang {
				return &SubtitleTrack{Key: key, Kind: "manual"}
			}
		}
	}
	if len(manual) > 0 {
		return &SubtitleTrack{Key: manual[0], Kind: "manual"}
	}
	return nil
}

type Limits struct {
	MaxBytes                 int64
	Stall, Max, TranscodeMax time.Duration
	Retries                  int
}

func DownloadLimits(env map[string]string) Limits {
	positive := func(key string, fallback float64) float64 {
		raw := env[key]
		if env == nil {
			raw = os.Getenv(key)
		}
		n := finiteNumber(raw)
		if n == nil || *n <= 0 {
			return fallback
		}
		return *n
	}
	millis := func(key string, fallback time.Duration) time.Duration {
		n := positive(key, float64(fallback/time.Millisecond))
		if n > float64(math.MaxInt64/int64(time.Millisecond)) {
			return fallback
		}
		return time.Duration(n) * time.Millisecond
	}
	maxBytes := positive("YTF_CLONE_DOWNLOAD_MAX_BYTES", 20*1024*1024*1024)
	if maxBytes >= float64(math.MaxInt64) {
		maxBytes = 20 * 1024 * 1024 * 1024
	}
	raw := env["YTF_CLONE_DOWNLOAD_RETRIES"]
	if env == nil {
		raw = os.Getenv("YTF_CLONE_DOWNLOAD_RETRIES")
	}
	retries := 2
	if n := finiteNumber(raw); n != nil && *n >= 0 && *n < float64(math.MaxInt) {
		retries = int(*n)
	}
	return Limits{MaxBytes: int64(maxBytes), Stall: millis("YTF_CLONE_DOWNLOAD_STALL_MS", 5*time.Minute), Max: millis("YTF_CLONE_DOWNLOAD_MAX_MS", time.Hour), TranscodeMax: millis("YTF_CLONE_TRANSCODE_MAX_MS", 2*time.Hour), Retries: retries}
}

var sourceFormatArgs = []string{"-f", "bv*+ba/b", "-S", "res:360,vcodec:h264"}

type DownloadArgsOpts struct {
	InfoFile, OutTemplate string
	Cookies               []string
	FfmpegDir             string
	Track                 *SubtitleTrack
	Mp4Target             bool
	RateLimitKbps         int
}

func BuildDownloadArgs(o DownloadArgsOpts) []string {
	args := []string{"--load-info-json", o.InfoFile, "--no-playlist"}
	args = append(args, sourceFormatArgs...)
	args = append(args, "--ffmpeg-location", o.FfmpegDir, "--write-thumbnail", "--newline", "--progress", "--progress-template", "download:YTFPROG|%(progress.status)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s")
	args = append(args, o.Cookies...)
	if o.Mp4Target {
		args = append(args, "--merge-output-format", "mp4", "--remux-video", "mp4")
	}
	if o.Track != nil {
		flag := "--write-auto-subs"
		if o.Track.Kind == "manual" {
			flag = "--write-subs"
		}
		args = append(args, flag, "--sub-langs", SubLangArg(o.Track.Key), "--sub-format", "json3")
	}
	if o.RateLimitKbps > 0 {
		args = append(args, "--limit-rate", strconv.Itoa(o.RateLimitKbps)+"K")
	}
	return append(args, "-o", o.OutTemplate)
}

type TranscodeArgsOpts struct {
	Inputs              []string
	Output, H264Encoder string
	HasAudio            bool
}

func BuildTranscodeArgs(o TranscodeArgsOpts) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-progress", "pipe:1", "-nostats"}
	for _, input := range o.Inputs {
		args = append(args, "-i", input)
	}
	for i := range o.Inputs {
		args = append(args, "-map", fmt.Sprintf("%d:v?", i), "-map", fmt.Sprintf("%d:a?", i))
	}
	args = append(args, "-vf", "setpts=PTS-STARTPTS")
	switch o.H264Encoder {
	case "libx264":
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20")
	case "h264_videotoolbox":
		args = append(args, "-c:v", "h264_videotoolbox", "-q:v", "60")
	default:
		args = append(args, "-c:v", o.H264Encoder)
	}
	args = append(args, "-fps_mode:v", "vfr")
	if o.HasAudio {
		args = append(args, "-af", "aresample=async=1:first_pts=0", "-c:a", "aac", "-b:a", "192k")
	}
	return append(args, "-movflags", "+faststart", "-avoid_negative_ts", "make_zero", "-max_muxing_queue_size", "1024", o.Output)
}

type helpEntry struct{ Hint string }

var YtDlpHelp = map[string]helpEntry{
	"login_required": {Hint: "Cần tài khoản Google phụ đã đăng nhập YouTube: chọn hồ sơ trình duyệt trong Cài đặt → Cookies rồi thử lại."},
	"cookies_failed": {Hint: "Khi máy hỏi quyền truy cập, chọn “Luôn cho phép”, hoặc chọn hồ sơ khác trong Cài đặt → Cookies."},
	"unavailable":    {Hint: "Video có thể là riêng tư, đã bị xoá hoặc bị chặn theo vùng. Hãy chọn video khác."},
}

func stepError(code, message, recovery string) *domain.CloneError {
	e := &domain.CloneError{Code: code, Message: message}
	if recovery != "" {
		e.Recovery = &recovery
	}
	return e
}
func retryError(message, recovery string, now int64, after time.Duration) *domain.CloneError {
	e := stepError(domain.CloneErrQuota, message, recovery)
	at := time.UnixMilli(now).Add(after)
	e.RetryAt = &at
	e.RetryAtSource = domain.RetryAtWorker
	return e
}
func MapDownloadError(err *tools.YtDlpError, now int64) *domain.CloneError {
	message := "yt-dlp: " + err.Message
	switch err.Code {
	case tools.YtDlpLoginRequired:
		if strings.Contains(strings.ToLower(err.Message), "not a bot") {
			return retryError(message, "YouTube tạm chặn vì tải nhiều video liên tục. Chờ khoảng 30 phút rồi bấm Thử lại; nếu vẫn bị, kiểm tra Cài đặt → Cookies", now, 30*time.Minute)
		}
		return stepError(domain.CloneErrInvalidRequest, message, YtDlpHelp["login_required"].Hint)
	case tools.YtDlpCookiesFailed:
		return stepError(domain.CloneErrInvalidRequest, message, YtDlpHelp["cookies_failed"].Hint)
	case tools.YtDlpUnavailable:
		return stepError(domain.CloneErrSourceUnavailable, message, YtDlpHelp["unavailable"].Hint)
	default:
		return retryError(message, "Thử lại sau — lỗi tạm thời khi tải từ nguồn", now, 10*time.Minute)
	}
}

var postprocessPattern = regexp.MustCompile(`(?i)postprocess|remux|merging formats|conversion|muxer|muxing|ffmpeg`)

func IsPostprocessFailure(err *tools.YtDlpError) bool {
	return postprocessPattern.MatchString(err.Message + "\n" + err.StderrTail)
}

var piecePattern = regexp.MustCompile(`^source\.f\d+\.`)
var thumbnailPattern = regexp.MustCompile(`(?i)^source\..+\.(webp|jpe?g|png)$`)

type StagedMedia struct {
	Files       []string
	Mp4, Merged string
	Pieces      []string
}

func FindStagedMedia(dir string) (StagedMedia, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return StagedMedia{}, err
	}
	var out StagedMedia
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "source.") || !e.Type().IsRegular() {
			continue
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".webm", ".mkv", ".mp4", ".m4a", ".m4v", ".mov", ".ogg", ".ogv", ".flv", ".avi", ".ts", ".m2ts", ".3gp":
		default:
			continue
		}
		info, err := e.Info()
		if err != nil {
			return out, err
		}
		if info.Size() == 0 {
			continue
		}
		out.Files = append(out.Files, name)
		if name == "source.mp4" {
			out.Mp4 = name
		} else if piecePattern.MatchString(name) {
			out.Pieces = append(out.Pieces, name)
		} else if out.Merged == "" {
			out.Merged = name
		}
	}
	return out, nil
}
func FindCaptionFile(dir string, track SubtitleTrack) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	fallback := ""
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasPrefix(name, "source.") || !strings.HasSuffix(name, ".json3") {
			continue
		}
		if name == "source."+track.Key+".json3" {
			return name
		}
		if fallback == "" {
			fallback = name
		}
	}
	return fallback
}
func FindThumbnailFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, ext := range []string{"webp", "jpg", "jpeg", "png"} {
		for _, e := range entries {
			if e.Type().IsRegular() && e.Name() == "source."+ext {
				return e.Name()
			}
		}
	}
	for _, e := range entries {
		if e.Type().IsRegular() && thumbnailPattern.MatchString(e.Name()) {
			return e.Name()
		}
	}
	return ""
}

var mergeLine = regexp.MustCompile(`^\[(Merger|VideoRemuxer|VideoConvertor|Fixup\w*|EmbedThumbnail)`)
