package audio

// Gói AE — ghép chạy nền：jobId 立即返回、409 冲突、进度单调、retry 从失败章
// 续跑、整章中间产物缓存（第二次导出只 nối+mã hoá）。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 假 ffmpeg ──
// go build 出来的小程序：解析 ffmpeg 参数，把每个 -i 输入和 concat 列表里的文件
// 当作存在的输入（缺失 → 报错退出 1），按 -progress pipe:1 往 stdout 打
// out_time_us 递增行，loudnorm 往 stderr 打测量 JSON，最后把一段固定 wav
// 拷到输出文件。全部调用记进 FAKEFFMPEG_LOG（一行一次调用）。

const fakeFFmpegSrc = `package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if log := os.Getenv("FAKEFFMPEG_LOG"); log != "" {
		if f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintln(f, strings.Join(os.Args[1:], " "))
			f.Close()
		}
	}
	var inputs []string
	out, progress := "", false
	loudnorm := false
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i":
			i++
			if i < len(args) {
				inputs = append(inputs, args[i])
			}
		case "-progress":
			i++
			progress = true
		case "-af":
			i++
			if i < len(args) && strings.Contains(args[i], "loudnorm") {
				loudnorm = true
			}
		}
	}
	if n := len(args); n > 0 {
		out = args[n-1]
	}
	// concat demuxer：-i 指向列表文件，展开里面的 file '…' 行作真实输入。
	var realInputs []string
	for _, in := range inputs {
		if strings.HasSuffix(in, ".txt") {
			data, err := os.ReadFile(in)
			if err != nil {
				fmt.Fprintf(os.Stderr, "missing concat list: %v\n", err)
				os.Exit(1)
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "file '") && strings.HasSuffix(line, "'") {
					realInputs = append(realInputs, strings.TrimSuffix(strings.TrimPrefix(line, "file '"), "'"))
				}
			}
			continue
		}
		realInputs = append(realInputs, in)
	}
	for _, in := range realInputs {
		if in == "" || in == "-" {
			continue
		}
		if _, err := os.Stat(in); err != nil {
			fmt.Fprintf(os.Stderr, "input %s: %v\n", in, err)
			os.Exit(1)
		}
	}
	if progress {
		for _, us := range []int{100000, 200000, 400000} {
			fmt.Printf("out_time_us=%d\n", us)
		}
		fmt.Println("progress=end")
	}
	if loudnorm {
		fmt.Fprintln(os.Stderr, "{\"input_i\":\"-20.0\",\"input_tp\":\"-3.0\",\"input_lra\":\"7.0\",\"input_thresh\":\"-30.0\",\"target_offset\":\"0.5\"}")
	}
	if out == "-" || out == "" || strings.HasSuffix(out, "null") {
		return
	}
	in, err := os.Open(os.Getenv("FAKEFFMPEG_WAV"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "no fixture: %v\n", err)
		os.Exit(1)
	}
	dst, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "out: %v\n", err)
		os.Exit(1)
	}
	if _, err := io.Copy(dst, in); err != nil {
		fmt.Fprintf(os.Stderr, "copy: %v\n", err)
		os.Exit(1)
	}
	dst.Close()
	in.Close()
}
`

// fakeFFmpeg 编译假 ffmpeg（测试间共享编译结果），并把 ffmpegBin/ffprobeBin
// 换成它；返回调用日志路径。
func fakeFFmpeg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fakeFFmpegSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "fakeffmpeg")
	if out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput(); err != nil {
		t.Fatalf("build fake ffmpeg: %v: %s", err, out)
	}
	fixture := filepath.Join(dir, "in.wav")
	if err := os.WriteFile(fixture, testWAV(48000, 1, 48000, 8000), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "calls.log")
	oldFF, oldProbe := ffmpegBin, ffprobeBin
	ffmpegBin, ffprobeBin = bin, bin
	t.Setenv("FAKEFFMPEG_LOG", logPath)
	t.Setenv("FAKEFFMPEG_WAV", fixture)
	t.Cleanup(func() { ffmpegBin, ffprobeBin = oldFF, oldProbe })
	return logPath
}

// fakeFFmpegLog 读出 ffmpeg 调用日志（不存在 = 一次都没调）。
func fakeFFmpegLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// seedExportableChapters 给一章写"已 chốt 且已有每段音频"的 script+缓存。
func seedExportableChapters(t *testing.T, svc *Service, volume int, chapters []int, segments int) {
	t.Helper()
	for _, ch := range chapters {
		segs := make([]Segment, 0, segments)
		for i := range segments {
			key := fmt.Sprintf("seg-%d-%d", ch, i)
			segs = append(segs, Segment{
				ID: fmt.Sprintf("s%04d", i+1), Speaker: NarratorID,
				Text: fmt.Sprintf("Đoạn %d của chương %d.", i+1, ch), PauseMs: 100,
				Rendered: &SegmentAudio{Key: key, DurationSec: 0.5},
			})
			if err := WriteSegmentAudio(svc.layout, volume, ch, key, testWAV(48000, 1, 24000, 8000)); err != nil {
				t.Fatal(err)
			}
		}
		now := time.Now()
		if err := WriteScript(svc.layout, &Script{
			Volume: volume, Chapter: ch, Status: StatusLocked, LockedAt: &now, Segments: segs,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// awaitExport 轮询持久化导出请求到终态。
func awaitExport(t *testing.T, svc *Service, volume int) ExportRequest {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, r := range svc.ExportRequests() {
			if r.Volume != volume {
				continue
			}
			if r.State == JobDone || r.State == JobError || r.State == JobCanceled {
				return r
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	requests := svc.ExportRequests()
	t.Fatalf("export never finished: %+v", requests)
	return ExportRequest{}
}

func TestStartExportConflictWhileRunning(t *testing.T) {
	log := fakeFFmpeg(t)
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1, 2}, 2)
	// 第一次：排上"等待"状态——还没有被 worker 拿走时就是 JobQueued。
	svc.mu.Lock()
	svc.exports[1] = ExportRequest{Volume: 1, State: JobQueued, Source: []SourceChapter{{Chapter: 1}}, Options: ExportOptions{Format: "m4b"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	svc.mu.Unlock()
	svc.kick()
	err := svc.StartExport(1, "Truyện", []SourceChapter{{Chapter: 1}, {Chapter: 2}}, ExportOptions{Format: "m4b"})
	var busy *ExportBusyError
	if !errors.As(err, &busy) {
		t.Fatalf("second export while queued must be busy: %v", err)
	}
	req := awaitExport(t, svc, 1)
	if req.State != JobDone {
		t.Fatalf("queued export must still finish: %+v", req)
	}
	if lines := fakeFFmpegLog(t, log); len(lines) == 0 {
		t.Fatal("export ran without ffmpeg")
	}
}

func TestExportProgressMonotonicWithFakeFFmpeg(t *testing.T) {
	fakeFFmpeg(t)
	svc := newTestService(t)
	svc.synth = &stoppedSynth{}
	voice := "Mai Anh"
	if _, err := svc.SaveCast(&Cast{Mode: ModeCast, Speakers: []Speaker{{ID: NarratorID, Name: "Dẫn truyện", Role: RoleNarrator, VoiceID: &voice}}}); err != nil {
		t.Fatal(err)
	}
	seedExportableChapters(t, svc, 1, []int{1, 2, 3}, 3)
	// Drain while the export runs: the hub keeps a 32-event buffer and drops
	// overflow, so collecting only after the export loses the final 100 %
	// event on slower/loaded runs.
	subCtx, stopSub := context.WithCancel(context.Background())
	defer stopSub()
	sub := svc.Hub().Subscribe(subCtx)
	type progressLog struct {
		pcts  []float64
		steps []string
	}
	collected := make(chan progressLog, 1)
	go func() {
		var out progressLog
		for ev := range sub {
			if ev.Type != EventExport {
				continue
			}
			if p, ok := ev.Data["percent"].(float64); ok {
				out.pcts = append(out.pcts, p)
			}
			if st, ok := ev.Data["step"].(string); ok && st != "" {
				out.steps = append(out.steps, st)
			}
			if ev.Data["state"] == "done" {
				break
			}
		}
		collected <- out
	}()
	if err := svc.StartExport(1, "Truyện", []SourceChapter{{Chapter: 1, Title: "Một"}, {Chapter: 2, Title: "Hai"}, {Chapter: 3, Title: "Ba"}},
		ExportOptions{Format: "mp3", ChapterTitles: false, Loudness: true}); err != nil {
		t.Fatal(err)
	}
	req := awaitExport(t, svc, 1)
	if req.State != JobDone {
		t.Fatalf("export failed: %s (cause=%s chapter=%d)", req.Error, req.ErrorCause, req.ErrorChapter)
	}
	var log progressLog
	select {
	case log = <-collected:
	case <-time.After(2 * time.Second):
		t.Fatal("export finished without publishing the terminal progress event")
	}
	pcts, steps := log.pcts, log.steps
	if len(pcts) < 3 {
		t.Fatalf("expected several progress events, got %v", pcts)
	}
	for i := 1; i < len(pcts); i++ {
		if pcts[i] < pcts[i-1] {
			t.Fatalf("progress not monotonic: %v", pcts)
		}
	}
	if pcts[len(pcts)-1] != 100 {
		t.Fatalf("last progress = %v, want 100", pcts)
	}
	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "mã hoá") || !strings.Contains(joined, "nối") {
		t.Fatalf("missing concat/encode steps: %q", joined)
	}
	if req.Result == nil || len(req.Result.Files) == 0 {
		t.Fatalf("result files missing: %+v", req.Result)
	}
}

// Per-segment step text reaches subscribers only through the 250 ms throttle,
// so a fast ffmpeg can finish a whole chapter between two ticks. Drive the
// reporter's clock directly instead of racing the real one.
func TestExportReporterThrottlesSegmentSteps(t *testing.T) {
	svc := newTestService(t)
	subCtx, stopSub := context.WithCancel(context.Background())
	defer stopSub()
	sub := svc.Hub().Subscribe(subCtx)
	drain := func() []string {
		var steps []string
		for {
			select {
			case ev := <-sub:
				if ev.Type == EventExport {
					steps = append(steps, ev.Data["step"].(string))
				}
			default:
				return steps
			}
		}
	}
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	r := newExportReporter(svc, 1)
	r.now = func() time.Time { return clock }

	r.begin(4, false)
	if got := drain(); len(got) != 1 || got[0] != "Đang chuẩn bị ghép" {
		t.Fatalf("begin must publish immediately, got %q", got)
	}
	r.step("Đang ghép chương 1 · đoạn 1/2")
	clock = clock.Add(100 * time.Millisecond)
	r.advance(1)
	if got := drain(); len(got) != 0 {
		t.Fatalf("progress inside the tick window must be held back, got %q", got)
	}
	clock = clock.Add(200 * time.Millisecond)
	r.step("Đang ghép chương 1 · đoạn 2/2")
	if got := drain(); len(got) != 1 || got[0] != "Đang ghép chương 1 · đoạn 2/2" {
		t.Fatalf("first step after the tick must carry the segment text, got %q", got)
	}
}

func TestExportRetryResumesFromFailedChapter(t *testing.T) {
	log := fakeFFmpeg(t)
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1, 2, 3}, 2)
	src := []SourceChapter{{Chapter: 1}, {Chapter: 2}, {Chapter: 3}}
	if err := svc.StartExport(1, "Truyện", src, ExportOptions{Format: "mp3"}); err != nil {
		t.Fatal(err)
	}
	if req := awaitExport(t, svc, 1); req.State != JobDone {
		t.Fatalf("first export: %+v", req)
	}
	// 删掉第 2 章的一段缓存 → 下次 export 必须在第 2 章失败。
	if err := os.Remove(svc.layout.CachePath(1, 2, "seg-2-0")); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartExport(1, "Truyện", src, ExportOptions{Format: "mp3", OnExists: "replace"}); err != nil {
		t.Fatal(err)
	}
	failed := awaitExport(t, svc, 1)
	if failed.State != JobError || failed.ErrorChapter != 2 || failed.ErrorCause != "ffmpeg" {
		t.Fatalf("expected failure at chapter 2: %+v", failed)
	}
	before := len(fakeFFmpegLog(t, log))
	// 补回缺失的段文件再 retry：应直接用已缓存的第 1/3 章。
	if err := WriteSegmentAudio(svc.layout, 1, 2, "seg-2-0", testWAV(48000, 1, 24000, 8000)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ControlExport(1, "retry"); err != nil {
		t.Fatal(err)
	}
	done := awaitExport(t, svc, 1)
	if done.State != JobDone {
		t.Fatalf("retry failed: %+v", done)
	}
	calls := fakeFFmpegLog(t, log)[before:]
	var rebuildCh1, rebuildCh3 bool
	for _, line := range calls {
		if strings.Contains(line, "c01-s") {
			rebuildCh1 = true
		}
		if strings.Contains(line, "c03-s") {
			rebuildCh3 = true
		}
	}
	if rebuildCh1 || rebuildCh3 {
		t.Fatalf("retry rebuilt already-done chapters:\n%s", strings.Join(calls, "\n"))
	}
}

func TestExportCacheHitOnSecondRun(t *testing.T) {
	log := fakeFFmpeg(t)
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1, 2}, 2)
	src := []SourceChapter{{Chapter: 1}, {Chapter: 2}}
	for _, opts := range []ExportOptions{{Format: "mp3"}, {Format: "mp3", OnExists: "replace"}} {
		if err := svc.StartExport(1, "Truyện", src, opts); err != nil {
			t.Fatal(err)
		}
		if req := awaitExport(t, svc, 1); req.State != JobDone {
			t.Fatalf("export: %+v", req)
		}
	}
	// The second identical export reuses the finished container as well as the
	// chapter cache; it must not concatenate or encode the volume again.
	calls := fakeFFmpegLog(t, log)
	segConvs, volumeConcats := 0, 0
	for _, line := range calls {
		if strings.Contains(line, "-s000") {
			segConvs++
		}
		if strings.Contains(line, ".concat-volume.wav.txt") {
			volumeConcats++
		}
	}
	if segConvs != 4 || volumeConcats != 1 {
		t.Fatalf("second export must reuse the finished result (segConvs=%d want 4, volumeConcats=%d want 1):\n%s",
			segConvs, volumeConcats, strings.Join(calls, "\n"))
	}
	// 输入变化（段被重新生成 = 文件删掉再写）→ 指纹变 → 那章重新构建。
	if err := os.Remove(svc.layout.CachePath(1, 1, "seg-1-0")); err != nil {
		t.Fatal(err)
	}
	if err := WriteSegmentAudio(svc.layout, 1, 1, "seg-1-0", testWAV(48000, 1, 24000, 9000)); err != nil {
		t.Fatal(err)
	}
	mark := len(fakeFFmpegLog(t, log))
	if err := svc.StartExport(1, "Truyện", src, ExportOptions{Format: "mp3", OnExists: "replace"}); err != nil {
		t.Fatal(err)
	}
	if req := awaitExport(t, svc, 1); req.State != JobDone {
		t.Fatalf("export after input change: %+v", req)
	}
	third := fakeFFmpegLog(t, log)[mark:]
	rebuilt1, rebuilt2 := false, false
	for _, line := range third {
		if strings.Contains(line, "c01-s") {
			rebuilt1 = true
		}
		if strings.Contains(line, "c02-s") {
			rebuilt2 = true
		}
	}
	if !rebuilt1 || rebuilt2 {
		t.Fatalf("cache invalidation wrong (ch1=%v ch2=%v):\n%s", rebuilt1, rebuilt2, strings.Join(third, "\n"))
	}
}

func TestExportIntroMissingFailsWithCause(t *testing.T) {
	fakeFFmpeg(t)
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1}, 1)
	if err := svc.StartExport(1, "Truyện", []SourceChapter{{Chapter: 1}}, ExportOptions{Format: "mp3", Intro: true}); err != nil {
		t.Fatal(err)
	}
	req := awaitExport(t, svc, 1)
	if req.State != JobError || req.ErrorCause != "intro_missing" || !req.Retryable {
		t.Fatalf("missing intro must fail with intro_missing: %+v", req)
	}
	// "Ghép không nhạc"：retry {intro:false} 不带音乐也能跑完。
	if err := svc.ControlExportBody(1, "retry", map[string]any{"intro": false}); err != nil {
		t.Fatal(err)
	}
	req = awaitExport(t, svc, 1)
	if req.State != JobDone {
		t.Fatalf("retry without music failed: %+v", req)
	}
}

// TestExportSecondRunIsFast measures the complete first export versus returning
// its persisted finished result, without concatenating or encoding again.
func TestExportSecondRunIsFast(t *testing.T) {
	if !FFmpegAvailable() {
		t.Skip("cần ffmpeg thật")
	}
	svc := newTestService(t)
	seedExportableChapters(t, svc, 1, []int{1, 2, 3}, 20)
	src := []SourceChapter{{Chapter: 1}, {Chapter: 2}, {Chapter: 3}}
	run := func(opts ExportOptions) (time.Duration, ExportRequest) {
		if err := svc.StartExport(1, "Truyện", src, opts); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		req := awaitExport(t, svc, 1)
		return time.Since(start), req
	}
	first, req := run(ExportOptions{Format: "mp3"})
	if req.State != JobDone {
		t.Fatalf("first export: %+v", req)
	}
	second, req := run(ExportOptions{Format: "mp3", OnExists: "replace"})
	if req.State != JobDone {
		t.Fatalf("second export: %+v", req)
	}
	t.Logf("export timing: first=%v second=%v (x%.1f faster)", first, second, float64(first)/float64(second))
	if second >= first {
		t.Fatalf("cached export not faster: first=%v second=%v", first, second)
	}
}
