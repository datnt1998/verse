package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// Deps supplies application services, not alternative implementations of
// publication. Final artifacts are committed only by the fenced pipeline runner.
type Deps struct {
	Tools         tools.Deps
	ResolveFfmpeg func(context.Context) *tools.FfmpegResolved
	CookieArgs    func(context.Context) ([]string, error)
	ProbeFile     func(context.Context, string, string) (ProbeSummary, error)
	Transcode     func(context.Context, TranscodeJob) error
	// FreeDisk returns available bytes, or -1 if the filesystem cannot report it.
	FreeDisk func(string) int64
	// Now returns Unix milliseconds, matching yt-factory retryAt timestamps.
	Now           func() int64
	Limits        *Limits
	RateLimitKbps int
}
type Handler struct{ deps Deps }

func New(deps *Deps) *Handler {
	h := &Handler{}
	if deps != nil {
		h.deps = *deps
	}
	return h
}

var _ pipeline.StepHandler = (*Handler)(nil)

func (h *Handler) now() int64 {
	if h.deps.Now != nil {
		return h.deps.Now()
	}
	return time.Now().UnixMilli()
}

type checkpoint struct {
	Version     int    `json:"version"`
	URL         string `json:"url"`
	InfoSHA256  string `json:"infoSha256,omitempty"`
	Phase       string `json:"phase"`
	MediaReady  bool   `json:"mediaReady"`
	RemuxFailed bool   `json:"remuxFailed"`
	Transcoded  bool   `json:"transcoded"`
	Downloaded  int64  `json:"downloaded"`
	Total       *int64 `json:"total,omitempty"`
}

func (h *Handler) Run(ctx context.Context, step *pipeline.StepContext) (result pipeline.StepResult, runErr error) {
	if step == nil {
		return result, fmt.Errorf("thiếu ngữ cảnh bước tải")
	}
	u, err := url.Parse(step.Job.Source.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return result, stepError(domain.CloneErrInvalidRequest, "Link video nguồn không hợp lệ", "")
	}
	if step.StagingDir == "" || step.WriteCheckpoint == nil {
		return result, fmt.Errorf("thiếu thư mục hoặc bộ ghi checkpoint bước tải")
	}
	if h.deps.RateLimitKbps < 0 {
		return result, fmt.Errorf("giới hạn tốc độ tải không hợp lệ")
	}
	limits := DownloadLimits(nil)
	if h.deps.Limits != nil {
		limits = *h.deps.Limits
	}
	if limits.MaxBytes <= 0 || limits.Stall <= 0 || limits.Max <= 0 || limits.TranscodeMax <= 0 || limits.Retries < 0 {
		return result, fmt.Errorf("giới hạn bước tải không hợp lệ")
	}
	cp := checkpoint{Version: 1, URL: step.Job.Source.URL, Phase: "metadata"}
	if len(step.Checkpoint) > 0 {
		cp = checkpoint{}
		if err := json.Unmarshal(step.Checkpoint, &cp); err != nil {
			return result, fmt.Errorf("đọc checkpoint tải: %w", err)
		}
		if cp.Version != 1 || cp.URL != step.Job.Source.URL || cp.Downloaded < 0 || (cp.Total != nil && *cp.Total < 0) || (cp.MediaReady && cp.InfoSHA256 == "") {
			return result, fmt.Errorf("checkpoint tải không khớp nguồn")
		}
	}
	// Pausing cancels only this invocation's child groups. The checkpoint write
	// uses the still-live parent context and leaves .part files in stable staging.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var paused atomic.Bool
	pauseDone := make(chan struct{})
	if step.PauseRequested() {
		paused.Store(true)
		cancel()
	}
	go func() {
		defer close(pauseDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if step.PauseRequested() {
					paused.Store(true)
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-pauseDone
		if ctx.Err() == nil && (paused.Load() || step.PauseRequested()) {
			if err := step.SaveCheckpoint(ctx, cp); err != nil {
				runErr = err
				result = pipeline.StepResult{}
				return
			}
			runErr = pipeline.ErrPaused
			result = pipeline.StepResult{}
			return
		}
		if runErr != nil && isDiskFailure(runErr) {
			runErr = h.diskError()
		}
	}()
	if err := runCtx.Err(); err != nil {
		return result, fmt.Errorf("%w: %w", toolchain.ErrCancelled, err)
	}
	if err := os.MkdirAll(step.StagingDir, 0o700); err != nil {
		return result, err
	}
	if err := step.SaveCheckpoint(runCtx, cp); err != nil {
		return result, err
	}

	toolDeps := h.deps.Tools
	if step.ProcessRunner != nil {
		toolDeps.Runner = runnerFunc(step.ProcessRunner)
	}
	if toolDeps.Runner == nil {
		return result, fmt.Errorf("thiếu bộ chạy process của bước tải")
	}
	managed := tools.New(toolDeps)
	var ff *tools.FfmpegResolved
	if h.deps.ResolveFfmpeg != nil {
		ff = h.deps.ResolveFfmpeg(runCtx)
	} else {
		ff = managed.Ffmpeg.Resolve(runCtx)
	}
	if err := runCtx.Err(); err != nil {
		return result, fmt.Errorf("%w: %w", toolchain.ErrCancelled, err)
	}
	if ff == nil || ff.Ffmpeg == "" {
		return result, stepError(domain.CloneErrToolMissing, "ffmpeg/ffprobe chưa sẵn sàng", "Mở Cài đặt → Công cụ và cài ffmpeg")
	}
	if ff.Ffprobe == "" {
		return result, stepError(domain.CloneErrToolMissing, "ffprobe không có cạnh "+ff.Ffmpeg, "Mở Cài đặt → Công cụ và cài lại ffmpeg")
	}
	var cookies []string
	if h.deps.CookieArgs != nil {
		cookies, err = h.deps.CookieArgs(runCtx)
		if err != nil {
			return result, err
		}
	}
	infoFile := filepath.Join(step.StagingDir, "source.info.json")
	var infoRaw []byte
	if cp.InfoSHA256 != "" {
		infoRaw, err = os.ReadFile(infoFile)
		if err != nil {
			return result, fmt.Errorf("đọc thông tin nguồn để tải tiếp: %w", err)
		}
		sum := sha256.Sum256(infoRaw)
		if hex.EncodeToString(sum[:]) != cp.InfoSHA256 {
			return result, fmt.Errorf("thông tin nguồn không khớp checkpoint tải")
		}
	} else {
		if err := reportProgress(runCtx, step, 0, nil, "metadata"); err != nil {
			return result, err
		}
		args := []string{"-J", "--skip-download", "--no-playlist"}
		args = append(args, sourceFormatArgs...)
		args = append(args, cookies...)
		args = append(args, step.Job.Source.URL)
		raw, e := managed.YtDlp.Run(runCtx, args, tools.RunYtDlpOptions{})
		if e != nil {
			return result, h.mapError(e)
		}
		infoRaw = []byte(raw)
	}
	info, err := parseSourceInfo(infoRaw)
	if err != nil {
		return result, stepError(domain.CloneErrSourceUnavailable, "Thông tin nguồn không hợp lệ: "+err.Error(), "Cập nhật công cụ tải trong Cài đặt rồi thử lại")
	}
	if cp.InfoSHA256 == "" {
		if err := stageInfo(infoFile, infoRaw); err != nil {
			return result, err
		}
		sum := sha256.Sum256(infoRaw)
		cp.InfoSHA256 = hex.EncodeToString(sum[:])
		if err := step.SaveCheckpoint(runCtx, cp); err != nil {
			return result, err
		}
	}
	if reason := LivestreamReason(info); reason != "" {
		return result, stepError(domain.CloneErrSourceUnavailable, reason, "Chọn một video đã đăng xong hoặc thử lại sau khi phát trực tiếp kết thúc")
	}
	expected := ExpectedDownloadBytes(info)
	if expected != nil && (*expected > float64(limits.MaxBytes) || *expected >= float64(math.MaxInt64)) {
		return result, stepError(domain.CloneErrInvalidRequest, fmt.Sprintf("Nguồn ~%s vượt giới hạn %s", humanSize(*expected), tools.HumanBytes(limits.MaxBytes)), "Chọn video ngắn hơn hoặc tăng YTF_CLONE_DOWNLOAD_MAX_BYTES")
	}
	needed := float64(512 * 1024 * 1024)
	if expected != nil {
		needed += *expected * 1.6
	}
	if err := h.guardDisk(runCtx, step.StagingDir, needed); err != nil {
		return result, err
	}
	track := PickSubtitleTrack(info.Subtitles, info.AutomaticCaptions, info.Language)
	if cp.Total == nil && expected != nil {
		total := int64(*expected)
		cp.Total = &total
	}
	if !cp.MediaReady {
		cp.Phase = "download"
		if err := step.SaveCheckpoint(runCtx, cp); err != nil {
			return result, err
		}
		opts := DownloadArgsOpts{InfoFile: infoFile, OutTemplate: filepath.Join(step.StagingDir, "source.%(ext)s"), Cookies: cookies, FfmpegDir: filepath.Dir(ff.Ffmpeg), Track: track, RateLimitKbps: h.deps.RateLimitKbps}
		if err := h.download(runCtx, step, managed.YtDlp, opts, limits, &cp); err != nil {
			return result, err
		}
		cp.MediaReady = true
		cp.Phase = "probe"
		if err := step.SaveCheckpoint(runCtx, cp); err != nil {
			return result, err
		}
	}
	if err := reportProgress(runCtx, step, cp.Downloaded, cp.Total, "probe"); err != nil {
		return result, err
	}
	staged, err := FindStagedMedia(step.StagingDir)
	if err != nil {
		return result, err
	}
	source := ""
	var probe ProbeSummary
	probed := false
	if staged.Mp4 != "" {
		source = filepath.Join(step.StagingDir, staged.Mp4)
	}
	if source == "" {
		var inputs []string
		if staged.Merged != "" {
			inputs = []string{filepath.Join(step.StagingDir, staged.Merged)}
		} else {
			for _, name := range staged.Pieces {
				inputs = append(inputs, filepath.Join(step.StagingDir, name))
			}
		}
		if len(inputs) == 0 {
			return result, stepError(domain.CloneErrInternal, "yt-dlp chạy xong nhưng không có tệp media nào trong thư mục tạm", "")
		}
		first, e := h.probe(runCtx, toolDeps.Runner, ff.Ffprobe, inputs[0])
		if e != nil {
			return result, e
		}
		if len(inputs) == 1 && DecideCompat(first) == "keep" {
			source = inputs[0]
			probe = first
			probed = true
		} else {
			cp.Phase = "transcode"
			cp.Transcoded = true
			if err := step.SaveCheckpoint(runCtx, cp); err != nil {
				return result, err
			}
			// Keep the partial transcode outside source.* so recovery cannot
			// mistake an interrupted MP4 for a completed yt-dlp output.
			source = filepath.Join(step.StagingDir, "compat.part.mp4")
			if err := h.guardDisk(runCtx, step.StagingDir, needed); err != nil {
				return result, err
			}
			if err := h.transcode(runCtx, TranscodeJob{Step: step, FF: ff, Runner: toolDeps.Runner, Inputs: inputs, SourceDurationSec: first.DurationSec, HasAudio: first.HasAudio || len(inputs) > 1, Target: source, Limits: limits}); err != nil {
				return result, err
			}
		}
	}
	if !probed {
		probe, err = h.probe(runCtx, toolDeps.Runner, ff.Ffprobe, source)
		if err != nil {
			return result, err
		}
	}
	if probe.VideoCodec == "" {
		return result, stepError(domain.CloneErrSourceUnavailable, "Nguồn tải về không có hình ảnh video", "Chọn video khác — nguồn chỉ có audio không dựng được video")
	}
	if probe.DurationSec == nil || *probe.DurationSec <= 0 || math.IsNaN(*probe.DurationSec) || math.IsInf(*probe.DurationSec, 0) {
		return result, stepError(domain.CloneErrSourceUnavailable, "Không đọc được thời lượng ổn định của nguồn", "Video có thể đang được xử lý — thử lại sau hoặc chọn video khác")
	}
	finalSource := filepath.Join(step.StagingDir, "source.mp4")
	if source != finalSource {
		// A same-directory rename is atomic and does not duplicate source bytes.
		// Publication/import deduplication belongs to internal/media.
		if err := os.Rename(source, finalSource); err != nil {
			return result, err
		}
	}
	cp.Phase = "import"
	if err := step.SaveCheckpoint(runCtx, cp); err != nil {
		return result, err
	}
	if err := reportProgress(runCtx, step, cp.Downloaded, cp.Total, "import"); err != nil {
		return result, err
	}
	sum, size, err := hashFile(runCtx, finalSource)
	if err != nil {
		return result, err
	}
	artifacts := []media.Artifact{
		{Path: "download/source.mp4", File: finalSource, ContentType: "video/mp4", Filename: "source.mp4"},
		{Path: "download/source.info.json", File: infoFile, ContentType: "application/json", Filename: "source.info.json"},
	}
	var thumbnail, caption any // Step manifests intentionally contain heterogeneous JSON values.
	thumbName := FindThumbnailFile(step.StagingDir)
	if thumbName != "" {
		file := filepath.Join(step.StagingDir, thumbName)
		hash, _, e := hashFile(runCtx, file)
		if e != nil {
			return result, e
		}
		mime := "image/jpeg"
		switch strings.ToLower(filepath.Ext(thumbName)) {
		case ".webp":
			mime = "image/webp"
		case ".png":
			mime = "image/png"
		}
		artifacts = append(artifacts, media.Artifact{Path: "download/" + thumbName, File: file, ContentType: mime, Filename: thumbName})
		thumbnail = map[string]any{"path": "download/" + thumbName, "sha256": hash}
	}
	if track != nil {
		if captionName := FindCaptionFile(step.StagingDir, *track); captionName != "" {
			file := filepath.Join(step.StagingDir, captionName)
			hash, bytes, e := hashFile(runCtx, file)
			if e != nil {
				return result, e
			}
			artifacts = append(artifacts, media.Artifact{Path: "download/" + captionName, File: file, ContentType: "application/json", Filename: captionName})
			caption = map[string]any{"path": "download/" + captionName, "kind": track.Kind, "language": track.Key, "sha256": hash, "bytes": bytes}
		}
	}
	formats := make([]map[string]any, 0, len(info.RequestedFormats))
	for _, f := range info.RequestedFormats {
		formats = append(formats, map[string]any{"format_id": nullableString(f.FormatID), "ext": nullableString(f.Ext), "vcodec": nullableString(f.Vcodec), "acodec": nullableString(f.Acodec), "filesize": formatBytes(f)})
	}
	title := info.Title
	if title == "" {
		title = step.Job.Source.Title
	}
	result = pipeline.StepResult{Artifacts: artifacts, Data: map[string]any{
		"sourceMediaId": "download/source.mp4", "sourceSha256": sum, "bytes": size,
		"durationSec": *probe.DurationSec, "width": probe.Width, "height": probe.Height, "fps": probe.FPS, "variableFps": probe.VariableFPS,
		"videoCodec": nullableString(probe.VideoCodec), "audioCodec": nullableString(probe.AudioCodec), "hasAudio": probe.HasAudio, "transcoded": cp.Transcoded,
		"rotationDeg": probe.RotationDeg, "startTimeSec": probe.StartTimeSec, "title": title, "liveStatus": nullableString(info.LiveStatus),
		"originFormats": formats, "thumbnail": thumbnail, "caption": caption,
	}}
	cp.Phase = "publish"
	if err := step.SaveCheckpoint(runCtx, cp); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := reportProgress(runCtx, step, cp.Downloaded, cp.Total, "publish"); err != nil {
		return pipeline.StepResult{}, err
	}
	return result, nil
}

func (h *Handler) download(ctx context.Context, step *pipeline.StepContext, y *tools.YtDlp, opts DownloadArgsOpts, limits Limits, cp *checkpoint) error {
	g := newActivityGate(ctx, limits.Stall, limits.Max)
	defer g.close()
	lastCheckpoint := time.Now()
	for attempt := 0; attempt <= limits.Retries; attempt++ {
		opts.Mp4Target = !cp.RemuxFailed
		var callbackErr error
		_, err := y.Run(g.ctx, BuildDownloadArgs(opts), tools.RunYtDlpOptions{
			Timeout: limits.Max, LogLimit: 4 * 1024 * 1024, Log: activityWriter{gate: g, stall: limits.Stall},
			OnLine: func(line string, _ tools.Stream) {
				if callbackErr != nil {
					return
				}
				phase := ""
				if p := ParseDownloadProgress(line); p != nil {
					if p.DownloadedBytes != nil && *p.DownloadedBytes >= 0 && *p.DownloadedBytes < float64(math.MaxInt64) {
						cp.Downloaded = int64(*p.DownloadedBytes)
					}
					if p.TotalBytes != nil && *p.TotalBytes >= 0 && *p.TotalBytes < float64(math.MaxInt64) {
						total := int64(*p.TotalBytes)
						cp.Total = &total
					}
					phase = "download"
					if p.Status == "finished" {
						phase = "merge"
					}
				} else if mergeLine.MatchString(strings.TrimSpace(line)) {
					phase = "merge"
				}
				if cp.Downloaded > limits.MaxBytes || (cp.Total != nil && *cp.Total > limits.MaxBytes) {
					callbackErr = stepError(domain.CloneErrInvalidRequest, "Nguồn vượt giới hạn "+tools.HumanBytes(limits.MaxBytes), "Chọn video ngắn hơn hoặc tăng YTF_CLONE_DOWNLOAD_MAX_BYTES")
				}
				if callbackErr == nil && phase != "" {
					cp.Phase = phase
					callbackErr = reportProgress(g.ctx, step, cp.Downloaded, cp.Total, phase)
				}
				if callbackErr == nil && time.Since(lastCheckpoint) >= time.Second {
					callbackErr = step.SaveCheckpoint(g.ctx, *cp)
					lastCheckpoint = time.Now()
				}
				if callbackErr != nil {
					g.cancel()
				}
			},
		})
		if callbackErr != nil {
			return callbackErr
		}
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %w", toolchain.ErrCancelled, ctx.Err())
		}
		if why := g.why(); why != "" {
			message := "ngừng tiến triển"
			if why == "deadline" {
				message = "vượt giới hạn thời gian"
			}
			return retryError(fmt.Sprintf("Tải nguồn %s (stall %ds, max %dph)", message, int(limits.Stall/time.Second), int(limits.Max/time.Minute)), "Kiểm tra mạng rồi chạy lại — phần .part đã tải được giữ để tiếp tục", h.now(), 5*time.Minute)
		}
		if err == nil {
			return nil
		}
		if isDiskFailure(err) {
			return h.diskError()
		}
		var ytErr *tools.YtDlpError
		if errors.As(err, &ytErr) && ytErr.Code == tools.YtDlpFailed {
			if !cp.RemuxFailed && IsPostprocessFailure(ytErr) {
				cp.RemuxFailed = true
				if err := step.SaveCheckpoint(ctx, *cp); err != nil {
					return err
				}
				attempt--
				continue
			}
			if attempt < limits.Retries {
				continue
			}
		}
		return h.mapError(err)
	}
	return fmt.Errorf("bước tải hết lượt nhưng chưa có kết quả")
}

func parseSourceInfo(raw []byte) (SourceInfo, error) {
	var info SourceInfo
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return info, fmt.Errorf("thiếu tài liệu JSON")
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return info, err
	}
	checkSize := func(n *float64) bool { return n == nil || (*n >= 0 && !math.IsInf(*n, 0) && !math.IsNaN(*n)) }
	if !checkSize(info.Filesize) || !checkSize(info.FilesizeApprox) {
		return info, fmt.Errorf("kích thước nguồn không hợp lệ")
	}
	for _, formats := range [][]SourceFormat{info.Formats, info.RequestedFormats} {
		for _, f := range formats {
			if !checkSize(f.Filesize) || !checkSize(f.FilesizeApprox) {
				return info, fmt.Errorf("kích thước định dạng không hợp lệ")
			}
		}
	}
	return info, nil
}
func stageInfo(file string, raw []byte) error {
	tmp := file + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmp, file); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(file))
	if err != nil {
		return err
	}
	syncErr := dir.Sync()
	closeErr = dir.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func hashFile(ctx context.Context, file string) (string, int64, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	size, readErr := io.Copy(hash, contextReader{ctx: ctx, r: f})
	closeErr := f.Close()
	if readErr != nil {
		return "", 0, readErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func humanSize(n float64) string {
	if n >= float64(math.MaxInt64) {
		return fmt.Sprintf("%.0f byte", n)
	}
	return tools.HumanBytes(int64(n))
}
func (h *Handler) diskError() *domain.CloneError {
	return retryError("Không đủ chỗ", "Giải phóng dung lượng đĩa rồi chạy lại bước tải", h.now(), 30*time.Minute)
}
func isDiskFailure(err error) bool {
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return true
	}
	text := err.Error()
	var failed *tools.ProcessFailedError
	if errors.As(err, &failed) {
		text += "\n" + failed.Stderr
	}
	var ytErr *tools.YtDlpError
	if errors.As(err, &ytErr) {
		text += "\n" + ytErr.StderrTail
	}
	text = strings.ToLower(text)
	return strings.Contains(text, "no space left on device") || strings.Contains(text, "disk quota exceeded")
}
func (h *Handler) guardDisk(ctx context.Context, path string, needed float64) error {
	var free int64
	if h.deps.FreeDisk != nil {
		free = h.deps.FreeDisk(path)
	} else {
		volume, err := storage.DefaultVolumes(h.deps.Tools.Home).Info(ctx, path)
		if err != nil {
			return fmt.Errorf("kiểm tra dung lượng tải nguồn: %w", err)
		}
		free = volume.FreeBytes
	}
	if free >= 0 && float64(free) < needed {
		return retryError(fmt.Sprintf("Không đủ chỗ: cần %s, ổ chỉ còn %s", humanSize(needed), tools.HumanBytes(free)), "Giải phóng dung lượng đĩa rồi chạy lại bước tải", h.now(), 30*time.Minute)
	}
	return nil
}
func (h *Handler) mapError(err error) error {
	if errors.Is(err, toolchain.ErrCancelled) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if isDiskFailure(err) {
		return h.diskError()
	}
	var ytErr *tools.YtDlpError
	if errors.As(err, &ytErr) {
		return MapDownloadError(ytErr, h.now())
	}
	return err
}
func (h *Handler) probe(ctx context.Context, runner tools.Runner, binary, file string) (ProbeSummary, error) {
	if h.deps.ProbeFile != nil {
		return h.deps.ProbeFile(ctx, binary, file)
	}
	res, err := runner.Run(ctx, binary, []string{"-v", "error", "-print_format", "json", "-show_format", "-show_streams", file}, tools.Spec{Timeout: 2 * time.Minute})
	if err != nil {
		return ProbeSummary{}, err
	}
	return ParseFfprobeJSON([]byte(res.Stdout))
}
