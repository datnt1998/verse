package download

// Chuyển ca test từ yt-factory src/server/clone/download.test.ts (2c201e6) và
// ca pickSubtitleTrack từ src/server/videos/parse.test.ts.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const progressLine = "YTFPROG|downloading|524288|2097152|NA|102400|18"

func TestParseDownloadProgress(t *testing.T) {
	p := ParseDownloadProgress(progressLine)
	if p == nil {
		t.Fatal("no progress parsed")
	}
	if p.Status != "downloading" || *p.DownloadedBytes != 524288 || *p.TotalBytes != 2097152 ||
		*p.SpeedBytesPerSec != 102400 || *p.EtaSec != 18 {
		t.Fatalf("%+v", p)
	}
	if ParseDownloadProgress("[download] 25% of ~2.0MiB") != nil {
		t.Fatal("non-template line parsed")
	}
	finished := ParseDownloadProgress("YTFPROG|finished|2097152|NA|NA|NA|NA")
	if finished == nil || finished.Status != "finished" || finished.TotalBytes != nil {
		t.Fatalf("%+v", finished)
	}
}

const ffprobeMp4 = `{
	"format": {"format_name": "mov,mp4,m4a,3gp,3g2,mj2", "duration": "60.012000", "start_time": "0.000000"},
	"streams": [
		{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"30/1","r_frame_rate":"30/1","disposition":{"attached_pic":0}},
		{"codec_type":"audio","codec_name":"aac","sample_rate":"48000"}
	]
}`

const ffprobeWebmVfrRotated = `{
	"format": {"format_name": "matroska,webm", "duration": "95.500000", "start_time": "-0.007000"},
	"streams": [
		{"codec_type":"video","codec_name":"vp9","width":1280,"height":720,"avg_frame_rate":"15/1","r_frame_rate":"60/1","start_time":"-0.007","side_data_list":[{"rotation":-90}],"disposition":{"attached_pic":0}},
		{"codec_type":"video","codec_name":"mjpeg","disposition":{"attached_pic":1}},
		{"codec_type":"audio","codec_name":"opus","start_time":"0.045"}
	]
}`

func TestParseFfprobeJsonMp4(t *testing.T) {
	probe, err := ParseFfprobeJSON([]byte(ffprobeMp4))
	if err != nil {
		t.Fatal(err)
	}
	if probe.DurationSec == nil || *probe.DurationSec != 60.012 ||
		probe.VideoCodec != "h264" || probe.AudioCodec != "aac" ||
		probe.Width == nil || *probe.Width != 1920 ||
		probe.FPS == nil || *probe.FPS != 30 || probe.VariableFPS || !probe.HasAudio {
		t.Fatalf("%+v", probe)
	}
}

func TestParseFfprobeJsonVfrRotationAttachedPics(t *testing.T) {
	probe, err := ParseFfprobeJSON([]byte(ffprobeWebmVfrRotated))
	if err != nil {
		t.Fatal(err)
	}
	if probe.VideoCodec != "vp9" || !probe.VariableFPS {
		t.Fatalf("%+v", probe)
	}
	if probe.RotationDeg == nil || *probe.RotationDeg != -90 {
		t.Fatalf("rotation: %+v", probe.RotationDeg)
	}
	if probe.StartTimeSec == nil || *probe.StartTimeSec != -0.007 {
		t.Fatalf("start: %+v", probe.StartTimeSec)
	}
	if probe.AudioCodec != "opus" || probe.DurationSec == nil || *probe.DurationSec != 95.5 {
		t.Fatalf("%+v", probe)
	}
}

func TestDecideCompat(t *testing.T) {
	keep, _ := ParseFfprobeJSON([]byte(ffprobeMp4))
	if DecideCompat(keep) != "keep" {
		t.Fatal("mp4 should be kept")
	}
	webm, _ := ParseFfprobeJSON([]byte(ffprobeWebmVfrRotated))
	if DecideCompat(webm) != "transcode" {
		t.Fatal("webm should transcode")
	}
	audioOnly := keep
	audioOnly.VideoCodec = ""
	if DecideCompat(audioOnly) != "transcode" {
		t.Fatal("video-less container should transcode")
	}
}

func TestExpectedDownloadBytes(t *testing.T) {
	if got := ExpectedDownloadBytes(SourceInfo{RequestedFormats: []SourceFormat{
		{FormatID: "400", Filesize: new(1000000.0)},
		{FormatID: "251", Filesize: new(200000.0)},
	}}); got == nil || *got != 1200000 {
		t.Fatalf("%v", got)
	}
	if got := ExpectedDownloadBytes(SourceInfo{FilesizeApprox: new(42000.0)}); got == nil || *got != 42000 {
		t.Fatalf("%v", got)
	}
	if got := ExpectedDownloadBytes(SourceInfo{RequestedFormats: []SourceFormat{{FormatID: "x"}}}); got != nil {
		t.Fatalf("%v", got)
	}
	if got := ExpectedDownloadBytes(SourceInfo{}); got != nil {
		t.Fatalf("%v", got)
	}
}

func TestLivestreamReason(t *testing.T) {
	if LivestreamReason(SourceInfo{IsLive: new(true)}) == "" {
		t.Fatal("live accepted")
	}
	if LivestreamReason(SourceInfo{LiveStatus: "is_live"}) == "" {
		t.Fatal("is_live accepted")
	}
	if LivestreamReason(SourceInfo{LiveStatus: "is_upcoming"}) == "" {
		t.Fatal("upcoming accepted")
	}
	if LivestreamReason(SourceInfo{LiveStatus: "post_live"}) != "" {
		t.Fatal("post_live rejected")
	}
	if LivestreamReason(SourceInfo{LiveStatus: "not_live"}) != "" {
		t.Fatal("not_live rejected")
	}
}

func TestSubLangArgEscapes(t *testing.T) {
	if got := SubLangArg("en"); got != "^en$" {
		t.Fatalf("%q", got)
	}
	if got := SubLangArg("en-orig"); got != "^en-orig$" {
		t.Fatalf("%q", got)
	}
	if got := SubLangArg("a.b(c)"); got != `^a\.b\(c\)$` {
		t.Fatalf("%q", got)
	}
}

func TestBuildDownloadArgs(t *testing.T) {
	args := BuildDownloadArgs(DownloadArgsOpts{
		InfoFile:    "/s/source.info.json",
		OutTemplate: "/s/source.%(ext)s",
		Cookies:     []string{"--cookies-from-browser", "chrome:Default"},
		FfmpegDir:   "/app/bin/ffmpeg-v6",
		Track:       &SubtitleTrack{Key: "en", Kind: "manual"},
		Mp4Target:   true,
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--merge-output-format mp4 --remux-video mp4",
		"--progress-template", "--newline", "--write-subs",
		"--sub-langs ^en$", "--ffmpeg-location /app/bin/ffmpeg-v6",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	for _, bad := range []string{"--skip-download", "--no-progress"} {
		for _, a := range args {
			if a == bad {
				t.Fatalf("unexpected %q", bad)
			}
		}
	}
	autoNoTarget := BuildDownloadArgs(DownloadArgsOpts{
		InfoFile: "/s/i.json", OutTemplate: "/s/o.%(ext)s", FfmpegDir: "/f",
		Track: &SubtitleTrack{Key: "vi-orig", Kind: "auto"},
	})
	contains := func(list []string, v string) bool {
		for _, a := range list {
			if a == v {
				return true
			}
		}
		return false
	}
	if !contains(autoNoTarget, "--write-auto-subs") || contains(autoNoTarget, "--remux-video") {
		t.Fatalf("%v", autoNoTarget)
	}
}

func TestBuildTranscodeArgs(t *testing.T) {
	args := BuildTranscodeArgs(TranscodeArgsOpts{
		Inputs:      []string{"/s/source.f137.mp4", "/s/source.f140.m4a"},
		Output:      "/s/source.mp4",
		H264Encoder: "libx264",
		HasAudio:    true,
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-i /s/source.f137.mp4 -i /s/source.f140.m4a",
		"-map 0:v? -map 0:a? -map 1:v? -map 1:a?",
		"setpts=PTS-STARTPTS", "-c:v libx264", "-fps_mode:v vfr", "-c:a aac", "+faststart",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	silent := BuildTranscodeArgs(TranscodeArgsOpts{
		Inputs: []string{"/s/source.webm"}, Output: "/s/out.mp4",
		H264Encoder: "h264_videotoolbox", HasAudio: false,
	})
	contains := func(v string) bool {
		for _, a := range silent {
			if a == v {
				return true
			}
		}
		return false
	}
	if !contains("-c:v") || !contains("h264_videotoolbox") || contains("-c:a") {
		t.Fatalf("%v", silent)
	}
}

func TestFindStagedMedia(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("source.mp4", "a")
	write("source.f137.mp4.part", "b")
	write("source.webp", "c")
	staged, err := FindStagedMedia(dir)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Mp4 != "source.mp4" || len(staged.Pieces) != 0 {
		t.Fatalf("%+v", staged)
	}
	if err := os.Remove(filepath.Join(dir, "source.mp4")); err != nil {
		t.Fatal(err)
	}
	write("source.f400.webm", "v")
	write("source.f251.webm", "a")
	staged, err = FindStagedMedia(dir)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Mp4 != "" || !reflect2(staged.Pieces, []string{"source.f251.webm", "source.f400.webm"}) {
		t.Fatalf("%+v", staged)
	}
}

func reflect2(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFindThumbnailAndCaptionFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "source.webp"), []byte("img"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "source.en.json3"), []byte("{}"), 0o600)
	if got := FindThumbnailFile(dir); got != "source.webp" {
		t.Fatalf("%q", got)
	}
	if got := FindCaptionFile(dir, SubtitleTrack{Key: "en", Kind: "manual"}); got != "source.en.json3" {
		t.Fatalf("%q", got)
	}
	if got := FindCaptionFile(dir, SubtitleTrack{Key: "vi", Kind: "auto"}); got != "source.en.json3" {
		t.Fatalf("single track fallback: %q", got)
	}
	if got := FindThumbnailFile(t.TempDir()); got != "" {
		t.Fatalf("%q", got)
	}
}

func TestMapDownloadError(t *testing.T) {
	login := MapDownloadError(&tools.YtDlpError{Message: "Sign in to see this members-only video", Code: tools.YtDlpLoginRequired}, 0)
	if login.Code != domain.CloneErrInvalidRequest || login.Recovery == nil || *login.Recovery != YtDlpHelp["login_required"].Hint {
		t.Fatalf("%+v", login)
	}
	bot := MapDownloadError(&tools.YtDlpError{Message: "Sign in to confirm you're not a bot", Code: tools.YtDlpLoginRequired}, 1700000000000)
	if bot.Code != domain.CloneErrQuota {
		t.Fatalf("bot check must retry later, got %+v", bot)
	}
	if bot.RetryAt == nil || bot.RetryAt.UnixMilli() < 1700000000000+29*60_000 {
		t.Fatalf("retryAt: %+v", bot.RetryAt)
	}
	cookies := MapDownloadError(&tools.YtDlpError{Message: "Failed to decrypt with DPAPI", Code: tools.YtDlpCookiesFailed}, 0)
	if cookies.Code != domain.CloneErrInvalidRequest || cookies.Recovery == nil || *cookies.Recovery != YtDlpHelp["cookies_failed"].Hint {
		t.Fatalf("%+v", cookies)
	}
	gone := MapDownloadError(&tools.YtDlpError{Message: "Video unavailable", Code: tools.YtDlpUnavailable}, 0)
	if gone.Code != domain.CloneErrSourceUnavailable || gone.Recovery == nil || *gone.Recovery != YtDlpHelp["unavailable"].Hint {
		t.Fatalf("%+v", gone)
	}
	failed := MapDownloadError(&tools.YtDlpError{Message: "HTTP Error 429", Code: tools.YtDlpFailed}, 1700000000000)
	if failed.Code != domain.CloneErrQuota || failed.RetryAt == nil || failed.RetryAt.UnixMilli() <= 1700000000000 {
		t.Fatalf("%+v", failed)
	}
}

func TestIsPostprocessFailure(t *testing.T) {
	if !IsPostprocessFailure(&tools.YtDlpError{Message: "Postprocessing: Conversion failed!", Code: tools.YtDlpFailed, StderrTail: "ERROR: Postprocessing: Conversion failed!"}) {
		t.Fatal("postprocess not detected")
	}
	if !IsPostprocessFailure(&tools.YtDlpError{Message: `muxing mp4: "vp8" not supported`, Code: tools.YtDlpFailed}) {
		t.Fatal("muxing not detected")
	}
	if IsPostprocessFailure(&tools.YtDlpError{Message: "HTTP Error 500", Code: tools.YtDlpFailed}) {
		t.Fatal("plain HTTP misclassified")
	}
}

func TestDownloadLimits(t *testing.T) {
	defaults := DownloadLimits(map[string]string{})
	if defaults.MaxBytes != 20*1024*1024*1024 || defaults.Retries != 2 {
		t.Fatalf("%+v", defaults)
	}
	custom := DownloadLimits(map[string]string{
		"YTF_CLONE_DOWNLOAD_MAX_BYTES": "1048576",
		"YTF_CLONE_DOWNLOAD_STALL_MS":  "5000",
		"YTF_CLONE_DOWNLOAD_MAX_MS":    "60000",
		"YTF_CLONE_TRANSCODE_MAX_MS":   "120000",
		"YTF_CLONE_DOWNLOAD_RETRIES":   "0",
	})
	if custom.MaxBytes != 1048576 || custom.Stall.Milliseconds() != 5000 ||
		custom.Max.Milliseconds() != 60000 || custom.TranscodeMax.Milliseconds() != 120000 || custom.Retries != 0 {
		t.Fatalf("%+v", custom)
	}
}

/* -------- pickSubtitleTrack (videos/parse.test.ts) -------- */

func tracks(exts ...string) []SubtitleFormat {
	out := make([]SubtitleFormat, 0, len(exts))
	for _, ext := range exts {
		out = append(out, SubtitleFormat{Ext: ext})
	}
	return out
}

func TestPickSubtitleTrack(t *testing.T) {
	auto := map[string][]SubtitleFormat{"en-orig": tracks("json3"), "en": tracks("json3"), "vi": tracks("json3")}
	got := PickSubtitleTrack(map[string][]SubtitleFormat{"nl-NL": tracks("json3"), "en-US": tracks("json3")}, auto, new("en"))
	if got == nil || got.Key != "en-US" || got.Kind != "manual" {
		t.Fatalf("%+v", got)
	}
	got = PickSubtitleTrack(map[string][]SubtitleFormat{"vi": tracks("json3")}, auto, new("en"))
	if got == nil || got.Key != "en-orig" || got.Kind != "auto" {
		t.Fatalf("%+v", got)
	}
	got = PickSubtitleTrack(
		map[string][]SubtitleFormat{"vi-VN": tracks("json3"), "en": tracks("json3")},
		map[string][]SubtitleFormat{"vi-orig": tracks("json3"), "en": tracks("json3")},
		nil,
	)
	if got == nil || got.Key != "vi-VN" || got.Kind != "manual" {
		t.Fatalf("%+v", got)
	}
	got = PickSubtitleTrack(
		map[string][]SubtitleFormat{"fr": tracks("json3"), "en": tracks("json3")},
		map[string][]SubtitleFormat{"vi": tracks("json3")},
		new("ja"),
	)
	if got == nil || got.Key != "en" || got.Kind != "manual" {
		t.Fatalf("%+v", got)
	}
	got = PickSubtitleTrack(
		map[string][]SubtitleFormat{"live_chat": tracks("json3"), "en": tracks("vtt")},
		map[string][]SubtitleFormat{},
		new("en"),
	)
	if got != nil {
		t.Fatalf("%+v", got)
	}
}

/* --------------------------- handler-level tests --------------------------- */

var fakeInfo = map[string]any{
	"id": "abcdefghijk", "title": "Fixture video", "duration": 60,
	"is_live": false, "live_status": "not_live", "language": "en",
	"subtitles":          map[string]any{"en": []any{map[string]any{"ext": "json3"}}},
	"automatic_captions": map[string]any{},
	"requested_formats":  []any{map[string]any{"format_id": "137", "ext": "mp4", "vcodec": "avc1.640028", "acodec": "none", "filesize": 1024}},
}

var mp4Probe = ProbeSummary{
	DurationSec: new(60.012), FormatName: "mov,mp4,m4a,3gp,3g2,mj2",
	VideoCodec: "h264", AudioCodec: "aac",
	Width: new(1920), Height: new(1080), FPS: new(30.0),
	StartTimeSec: new(0.0), HasAudio: true,
}

type fakeStepMedia map[string]string

func (m fakeStepMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	return nil, errors.New("unexpected acquire " + path)
}
func (m fakeStepMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return errors.New("unexpected publish")
}

func fakeCtx(t *testing.T) (*pipeline.StepContext, string) {
	t.Helper()
	staging := t.TempDir()
	return &pipeline.StepContext{
		Job: domain.CloneJob{ID: "job-test", Source: domain.CloneSourceSnapshot{
			ID: "abcdefghijk", URL: "https://youtu.be/abcdefghijk", Title: "Fixture video",
		}},
		Step:            domain.CloneStepRecord{Step: domain.CloneDownload, Attempt: 1},
		StagingDir:      staging,
		Upstream:        map[domain.CloneStep]domain.CloneStepOutput{},
		ReportProgress:  func(context.Context, domain.CloneProgress) error { return nil },
		WriteCheckpoint: func(context.Context, json.RawMessage) error { return nil },
		IsPaused:        func() bool { return false },
	}, staging
}

type configuredTools struct{}

func (configuredTools) Get(key string) (string, bool) {
	if key == "tools:yt-dlp" {
		return "/managed/yt-dlp", true
	}
	return "", false
}
func (configuredTools) Set(string, string) error { return nil }

func fakeDeps(overrides func(*Deps)) *Deps {
	d := &Deps{
		Tools: tools.Deps{Runner: &scriptedRunner{}, Paths: configuredTools{}},
		ResolveFfmpeg: func(context.Context) *tools.FfmpegResolved {
			return &tools.FfmpegResolved{
				Ffmpeg: "/managed/ffmpeg", Ffprobe: "/managed/ffprobe",
				Probe: &tools.FfmpegProbe{Caps: tools.FfmpegCaps{H264: "libx264"}},
			}
		},
		CookieArgs: func(context.Context) ([]string, error) { return nil, nil },
		ProbeFile:  func(context.Context, string, string) (ProbeSummary, error) { return mp4Probe, nil },
		Transcode:  func(context.Context, TranscodeJob) error { return nil },
		FreeDisk:   func(string) int64 { return -1 },
		Now:        func() int64 { return 1700000000000 },
	}
	if overrides != nil {
		overrides(d)
	}
	return d
}

// scriptedRunner plays canned yt-dlp/ffmpeg answers keyed on args.
type scriptedRunner struct {
	yt    func(args []string, emit func(string)) (string, error)
	calls [][]string
	run   func(file string, args []string, emit func(string)) (tools.Result, error)
}

func (r *scriptedRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	if r.yt != nil && strings.Contains(filepath.Base(file), "yt-dlp") {
		r.calls = append(r.calls, args)
		out, err := r.yt(args, func(line string) {
			if spec.OnLine != nil {
				spec.OnLine(line, tools.StreamStdout)
			}
		})
		var ytErr *tools.YtDlpError
		if errors.As(err, &ytErr) {
			err = &tools.ProcessFailedError{Message: ytErr.Message, Code: 1, Stderr: "ERROR: " + ytErr.Message}
		}
		return tools.Result{Stdout: out}, err
	}
	if r.run != nil {
		return r.run(file, args, func(line string) {
			if spec.OnLine != nil {
				spec.OnLine(line, tools.StreamStdout)
			}
		})
	}
	return tools.Result{}, nil
}

func writeStaging(t *testing.T, staging, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(staging, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerPublishesMp4InfoThumbnailCaptions(t *testing.T) {
	ctx, staging := fakeCtx(t)
	runner := &scriptedRunner{}
	deps := fakeDeps(nil)
	runner.yt = func(args []string, emit func(string)) (string, error) {
		for _, a := range args {
			if a == "-J" {
				raw, _ := json.Marshal(fakeInfo)
				return string(raw), nil
			}
		}
		writeStaging(t, staging, "source.mp4", "mp4")
		writeStaging(t, staging, "source.webp", "thumb")
		writeStaging(t, staging, "source.en.json3", `{"events":[]}`)
		return "", nil
	}
	deps.Tools.Runner = runner
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data["sourceMediaId"] != "download/source.mp4" {
		t.Fatalf("mediaId: %v", result.Data["sourceMediaId"])
	}
	if result.Data["transcoded"] != false || result.Data["durationSec"] != 60.012 {
		t.Fatalf("data: %+v", result.Data)
	}
	paths := []string{}
	for _, a := range result.Artifacts {
		paths = append(paths, a.Path)
	}
	sort.Strings(paths)
	want := []string{
		"download/source.en.json3", "download/source.info.json",
		"download/source.mp4", "download/source.webp",
	}
	if !reflect2(paths, want) {
		t.Fatalf("artifacts: %v", paths)
	}
	caption, ok := result.Data["caption"].(map[string]any)
	if !ok || caption["language"] != "en" || caption["kind"] != "manual" {
		t.Fatalf("caption: %+v", result.Data["caption"])
	}
	if !containsArg(runner.calls[0], "-J") {
		t.Fatalf("first call: %v", runner.calls[0])
	}
	if !containsArg(runner.calls[1], "--merge-output-format") || containsArg(runner.calls[1], "--skip-download") {
		t.Fatalf("media pass: %v", runner.calls[1])
	}
}

func containsArg(args []string, v string) bool {
	for _, a := range args {
		if a == v {
			return true
		}
	}
	return false
}

func TestHandlerSucceedsWithoutSubtitles(t *testing.T) {
	ctx, staging := fakeCtx(t)
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		for _, a := range args {
			if a == "-J" {
				info := map[string]any{}
				raw, _ := json.Marshal(fakeInfo)
				_ = json.Unmarshal(raw, &info)
				info["subtitles"] = map[string]any{}
				raw, _ = json.Marshal(info)
				return string(raw), nil
			}
		}
		writeStaging(t, staging, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data["caption"] != nil {
		t.Fatalf("caption: %+v", result.Data["caption"])
	}
	found := false
	for _, a := range result.Artifacts {
		if a.Path == "download/source.mp4" {
			found = true
		}
	}
	if !found {
		t.Fatalf("artifacts: %+v", result.Artifacts)
	}
}

func TestHandlerMapsLoginError(t *testing.T) {
	ctx, _ := fakeCtx(t)
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		return "", &tools.YtDlpError{Message: "Sign in to see this members-only video", Code: tools.YtDlpLoginRequired}
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	_, err := New(deps).Run(context.Background(), ctx)
	var cerr *domain.CloneError
	if !errors.As(err, &cerr) || cerr.Code != domain.CloneErrInvalidRequest ||
		cerr.Recovery == nil || *cerr.Recovery != YtDlpHelp["login_required"].Hint {
		t.Fatalf("err: %v", err)
	}
}

func TestHandlerRejectsLivestream(t *testing.T) {
	ctx, _ := fakeCtx(t)
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		for _, a := range args {
			if a == "-J" {
				info := map[string]any{}
				raw, _ := json.Marshal(fakeInfo)
				_ = json.Unmarshal(raw, &info)
				info["is_live"] = true
				info["live_status"] = "is_live"
				raw, _ = json.Marshal(info)
				return string(raw), nil
			}
		}
		return "", errors.New("media download must not start for a livestream")
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	_, err := New(deps).Run(context.Background(), ctx)
	var cerr *domain.CloneError
	if !errors.As(err, &cerr) || cerr.Code != domain.CloneErrSourceUnavailable {
		t.Fatalf("err: %v", err)
	}
}

func TestHandlerAbortsCancelledRun(t *testing.T) {
	ctx, _ := fakeCtx(t)
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		return "", tools.ErrCancelled
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	_, err := New(deps).Run(context.Background(), ctx)
	if !errors.Is(err, tools.ErrCancelled) {
		t.Fatalf("cancelled swallowed: %v", err)
	}
}

func TestHandlerRetriesFailedPass(t *testing.T) {
	ctx, staging := fakeCtx(t)
	mediaRuns := 0
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		for _, a := range args {
			if a == "-J" {
				info := map[string]any{}
				raw, _ := json.Marshal(fakeInfo)
				_ = json.Unmarshal(raw, &info)
				info["subtitles"] = map[string]any{}
				raw, _ = json.Marshal(info)
				return string(raw), nil
			}
		}
		mediaRuns++
		if mediaRuns == 1 {
			return "", &tools.YtDlpError{Message: "HTTP Error 500", Code: tools.YtDlpFailed, StderrTail: "ERROR: HTTP Error 500"}
		}
		writeStaging(t, staging, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mediaRuns != 2 {
		t.Fatalf("runs: %d", mediaRuns)
	}
	if result.Data["sourceMediaId"] != "download/source.mp4" {
		t.Fatalf("data: %+v", result.Data)
	}
}

func TestPostprocessFailureRerunsWithoutMp4Target(t *testing.T) {
	ctx, staging := fakeCtx(t)
	runs := []bool{}
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		for _, a := range args {
			if a == "-J" {
				info := map[string]any{}
				raw, _ := json.Marshal(fakeInfo)
				_ = json.Unmarshal(raw, &info)
				info["subtitles"] = map[string]any{}
				raw, _ = json.Marshal(info)
				return string(raw), nil
			}
		}
		hasMp4 := containsArg(args, "--merge-output-format")
		runs = append(runs, hasMp4)
		if hasMp4 {
			return "", &tools.YtDlpError{Message: "Postprocessing: Conversion failed!", Code: tools.YtDlpFailed, StderrTail: "ERROR: Postprocessing: Conversion failed!"}
		}
		writeStaging(t, staging, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || !runs[0] || runs[1] {
		t.Fatalf("runs: %v", runs)
	}
	if result.Data["sourceMediaId"] != "download/source.mp4" {
		t.Fatalf("data: %+v", result.Data)
	}
}
