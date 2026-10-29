package audio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ffmpeg / ffprobe 封装。生成段落本身用引擎（voice.Service），但"按语速变调、
// 拼接、加静音、混音乐、cân bằng âm lượng、编 MP3"全部靠 ffmpeg。
// 缺失时返回明确的越南语错误（界面上是"cài ffmpeg"提示），不静默降级。

// runFFmpegProgress 执行 ffmpeg 并用 -progress pipe:1 读取真实进度：
// ffmpeg 往 stdout 打 "out_time_us=<µs>"，按已知总时长折算成 0..1 回调。
// 回调方自己节流/单调，这里只原样上报。
func runFFmpegProgress(ctx context.Context, totalSec float64, progress func(float64), args ...string) error {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		return ErrFFmpegMissing
	}
	cmd := exec.CommandContext(ctx, ffmpegBin, append([]string{"-hide_banner", "-nostdin", "-y", "-progress", "pipe:1"}, args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 4096), 64*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		value, ok := strings.CutPrefix(line, "out_time_us=")
		if !ok {
			if line == "progress=end" && progress != nil {
				progress(1)
			}
			continue
		}
		us, perr := strconv.ParseInt(value, 10, 64)
		if perr != nil || totalSec <= 0 || progress == nil {
			continue
		}
		frac := float64(us) / (totalSec * 1e6)
		if frac > 1 {
			frac = 1
		}
		progress(frac)
	}
	err = cmd.Wait()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 800 {
			msg = msg[len(msg)-800:]
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg lỗi: %v: %s", err, msg)
	}
	return nil
}

// ErrFFmpegMissing 表示本机没有 ffmpeg/ffprobe。
var ErrFFmpegMissing = errors.New("Máy chưa có ffmpeg. Cài bằng “brew install ffmpeg” rồi thử lại.")

// SampleRate 是音频链路统一采样率（VieNeu v3 Turbo 输出 48 kHz）。
const SampleRate = 48000

// ffmpegBin / ffprobeBin 可在测试里替换。
var (
	ffmpegBin  = "ffmpeg"
	ffprobeBin = "ffprobe"
)

// FFmpegAvailable 报告 ffmpeg 与 ffprobe 是否都在 PATH 上。
func FFmpegAvailable() bool {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		return false
	}
	if _, err := exec.LookPath(ffprobeBin); err != nil {
		return false
	}
	return true
}

// ProbeDuration 用 ffprobe 读媒体时长（秒）。
func ProbeDuration(ctx context.Context, path string) (float64, error) {
	if _, err := exec.LookPath(ffprobeBin); err != nil {
		return 0, ErrFFmpegMissing
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("không đọc được tệp âm thanh: %w", err)
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("không đọc được thời lượng tệp âm thanh: %w", err)
	}
	return sec, nil
}

// runFFmpeg 执行一次 ffmpeg，错误里带尾部输出（便于排查）。
func runFFmpeg(ctx context.Context, args ...string) error {
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		return ErrFFmpegMissing
	}
	cmd := exec.CommandContext(ctx, ffmpegBin, append([]string{"-hide_banner", "-nostdin", "-y"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 800 {
			msg = msg[len(msg)-800:]
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg lỗi: %v: %s", err, msg)
	}
	return nil
}

// audioFilters 拼出"变调 + 变速"的滤镜链。引擎忽略 speed，所以语速由这里实现。
func audioFilters(speed, pitch float64) string {
	if speed <= 0 {
		speed = 1
	}
	if pitch <= 0 {
		pitch = 1
	}
	var parts []string
	if pitch != 1 {
		parts = append(parts, fmt.Sprintf("asetrate=%d", int(float64(SampleRate)*pitch)))
		parts = append(parts, fmt.Sprintf("aresample=%d", SampleRate))
		parts = append(parts, fmt.Sprintf("atempo=%.6f", 1/pitch))
	}
	if speed != 1 {
		parts = append(parts, atempoChain(speed)...)
	}
	return strings.Join(parts, ",")
}

// atempoChain 把任意语速拆成 atempo 支持的 0.5–2.0 段。
func atempoChain(speed float64) []string {
	var out []string
	for speed > 2 {
		out = append(out, "atempo=2.0")
		speed /= 2
	}
	for speed < 0.5 {
		out = append(out, "atempo=0.5")
		speed /= 0.5
	}
	out = append(out, fmt.Sprintf("atempo=%.6f", speed))
	return out
}

// adjustAudio 对一段 wav 应用语速/变调，返回新的 wav 字节（48 kHz 单声道 s16）。
func adjustAudio(ctx context.Context, wav []byte, speed, pitch float64) ([]byte, error) {
	filters := audioFilters(speed, pitch)
	if filters == "" {
		return wav, nil
	}
	dir, err := os.MkdirTemp("", "ainovel-audio-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "in.wav")
	out := filepath.Join(dir, "out.wav")
	if err := os.WriteFile(in, wav, 0o600); err != nil {
		return nil, err
	}
	if err := runFFmpeg(ctx, "-i", in, "-af", filters, "-ar", strconv.Itoa(SampleRate), "-ac", "1", "-c:a", "pcm_s16le", out); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

// toPCM 把任意音频转成 48 kHz 单声道 s16 wav（拼接前的统一格式）。
func toPCM(ctx context.Context, in, out string) error {
	return runFFmpeg(ctx, "-i", in, "-ar", strconv.Itoa(SampleRate), "-ac", "1", "-c:a", "pcm_s16le", out)
}

// concatWAVs 用 concat demuxer 拼接同格式 wav。
func concatWAVs(ctx context.Context, inputs []string, out string) error {
	if len(inputs) == 0 {
		return errors.New("không có đoạn nào để ghép")
	}
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	listPath := filepath.Join(dir, ".concat-"+filepath.Base(out)+".txt")
	var b strings.Builder
	for _, in := range inputs {
		abs, err := filepath.Abs(in)
		if err != nil {
			abs = in
		}
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(abs, "'", `'\''`))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o600); err != nil {
		return err
	}
	defer os.Remove(listPath)
	return runFFmpeg(ctx, "-f", "concat", "-safe", "0", "-i", listPath,
		"-ar", strconv.Itoa(SampleRate), "-ac", "1", "-c:a", "pcm_s16le", out)
}

// concatWAVsProgress 与 concatWAVs 相同，但按输入总时长上报真实进度。
func concatWAVsProgress(ctx context.Context, inputs []string, out string, totalSec float64, progress func(float64)) error {
	if len(inputs) == 0 {
		return errors.New("không có đoạn nào để ghép")
	}
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	listPath := filepath.Join(dir, ".concat-"+filepath.Base(out)+".txt")
	var b strings.Builder
	for _, in := range inputs {
		abs, err := filepath.Abs(in)
		if err != nil {
			abs = in
		}
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(abs, "'", `'\''`))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o600); err != nil {
		return err
	}
	defer os.Remove(listPath)
	return runFFmpegProgress(ctx, totalSec, progress, "-f", "concat", "-safe", "0", "-i", listPath,
		"-ar", strconv.Itoa(SampleRate), "-ac", "1", "-c:a", "pcm_s16le", out)
}

// encodeAudio applies measured two-pass normalization before encoding.
// tags 为空时只编码（in-app 播放文件走这条路，tagged 版在 export 用）。
func encodeAudio(ctx context.Context, in, out, format string, loudness bool) error {
	return encodeAudioTagged(ctx, in, out, format, loudness, tagSpec{}, "")
}

// tagSpec 是导出成品的"元数据 + 封面 + 章节"打包（Gói AF）。空值 = 不贴。
type tagSpec struct {
	tags     tagSet        // 文本标签（title/artist/album/…）
	coverPNG string        // 封面 PNG 路径（1400²，已缓存）；M4B 走 covr，MP3/WAV 走 ID3
	marks    []ChapterMark // 整卷文件的章节帧（仅 ID3 需要；M4B 由 ffmetadata 映射）
	totalSec float64       // 末章 END 时间
	quality  string        // story quality selects the final encoding profile
}

// encodeAudioTagged 编码 + 按 §AF 写标签：
//   - MP3: ffmpeg 只编码（-id3v2_version 0），Go 写 ID3v2.4（文本/APIC/COMM/TLAN/CHAP/CTOC）。
//   - WAV: ffmpeg 编码 + LIST/INFO 元数据，Go 追加 `id3 ` 块（同一 ID3 序列化，含 CHAP/CTOC）。
//   - M4B: ffmpeg remux — ©nam ©ART ©alb aART trkn disk ©gen ©day covr + stik=2 + 章节轨。
func encodeAudioTagged(ctx context.Context, in, out, format string, loudness bool, spec tagSpec, metadata string) error {
	return encodeAudioTaggedProgress(ctx, in, out, format, loudness, spec, metadata, 0, nil)
}

// encodeAudioChaptersProgress 与 encodeAudioTagged 相同，但 -progress 上报
// 两遍的真实进度：测响度（0→0.4）与正式编码（0.4→1）。progress 可为 nil。
func encodeAudioTaggedProgress(ctx context.Context, in, out, format string, loudness bool, spec tagSpec, metadata string, totalSec float64, progress func(float64)) error {
	report := func(frac float64) {
		if progress != nil {
			progress(frac)
		}
	}
	args := []string{"-i", in}
	chapterInput := -1
	if metadata != "" {
		args = append(args, "-f", "ffmetadata", "-i", metadata)
		chapterInput = 1
	}
	// M4B 封面：第二路视频输入，attached_pic → covr 原子。
	if format == "m4b" && spec.coverPNG != "" {
		args = append(args, "-i", spec.coverPNG)
	}
	args = append(args, "-map", "0:a")
	if format == "m4b" && spec.coverPNG != "" {
		coverIdx := 1
		if chapterInput >= 0 {
			coverIdx = 2
		}
		args = append(args, "-map", fmt.Sprintf("%d:v", coverIdx),
			"-c:v", "png", "-disposition:v:0", "attached_pic")
	}
	if metadata != "" {
		args = append(args, "-map_metadata", itoa(chapterInput), "-map_chapters", itoa(chapterInput))
	}
	// §AF: artist 永远是 "Noverse Audio"；旧的 "Noverse" 与 voice 状态
	// （"Chưa chọn giọng"）不再进任何字段。MP3 标签由 id3v24 写——这里
	// 不让 lame 写自己的头，避免双 ID3。
	if format != "mp3" {
		args = append(args, tagArgs(spec.tags)...)
	}
	if format == "m4b" {
		args = append(args, "-metadata", "media_type="+m4bMediaType)
	}
	measureFrac, encodeFrac := 0.0, 0.0
	if loudness {
		filter, err := measuredLoudnormProgress(ctx, in, totalSec, func(f float64) { report(f * 0.4) })
		if err != nil {
			return err
		}
		if filter != "" {
			args = append(args, "-af", filter)
		}
		measureFrac, encodeFrac = 0.4, 0.6
	} else {
		encodeFrac = 1.0
	}
	sampleRate, bitrate := "44100", "128k"
	if spec.quality == QualityHigh {
		sampleRate, bitrate = "48000", "192k"
	}
	args = append(args, "-ar", sampleRate)
	switch format {
	case "wav":
		args = append(args, "-c:a", "pcm_s16le")
	case "m4b":
		args = append(args, "-c:a", "aac", "-b:a", bitrate, "-f", "mp4", "-movflags", "+faststart")
	default:
		args = append(args, "-c:a", "libmp3lame", "-b:a", bitrate, "-id3v2_version", "0")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(out), ".encode-*."+format)
	if err != nil {
		return err
	}
	encoded := tmp.Name()
	defer os.Remove(encoded)
	if err := tmp.Close(); err != nil {
		return err
	}
	args = append(args, encoded)
	if err := runFFmpegProgress(ctx, totalSec, func(f float64) { report(measureFrac + f*encodeFrac) }, args...); err != nil {
		return err
	}
	if err := postTag(ctx, encoded, format, spec); err != nil {
		return err
	}
	return publishExportFile(encoded, out)
}

// postTag 把 Go 手写的 ID3v2.4 塞进 MP3 头 / WAV `id3 ` 块。
// spec.tags 为空（in-app 播放）或格式为 m4b（已在 mux 时写好）时 no-op。
func postTag(_ context.Context, out, format string, spec tagSpec) error {
	if format != "mp3" && format != "wav" {
		return nil
	}
	if spec.tags.Title == "" {
		return nil
	}
	var cover []byte
	if spec.coverPNG != "" {
		data, err := os.ReadFile(spec.coverPNG)
		if err != nil {
			return fmt.Errorf("không đọc được bìa: %w", err)
		}
		cover = data
	}
	tags := spec.tags
	tags.Cover = cover
	id3 := id3v24(tags, spec.marks, spec.totalSec)
	if format == "wav" {
		return appendWavID3(out, id3)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return writeFileAtomic(out, append(id3, stripID3(raw)...))
}

// stripID3 去掉已有的 ID3v2 头（含 footer flag 0x10 的尾部）。
func stripID3(data []byte) []byte {
	for len(data) >= 10 && bytes.Equal(data[:3], []byte("ID3")) {
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 | int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		skip := 10 + size
		if skip > len(data) {
			return data
		}
		data = data[skip:]
	}
	return data
}

// measuredLoudnormProgress 是 measuredLoudnorm 带 -progress 的版本。
func measuredLoudnormProgress(ctx context.Context, in string, totalSec float64, progress func(float64)) (string, error) {
	cmd := exec.CommandContext(ctx, ffmpegBin, "-hide_banner", "-nostdin", "-progress", "pipe:1", "-i", in,
		"-af", "loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json", "-f", "null", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 4096), 64*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		value, ok := strings.CutPrefix(line, "out_time_us=")
		if !ok {
			continue
		}
		if us, perr := strconv.ParseInt(value, 10, 64); perr == nil && totalSec > 0 && progress != nil {
			frac := float64(us) / (totalSec * 1e6)
			if frac > 1 {
				frac = 1
			}
			progress(frac)
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("không đo được âm lượng: %w: %s", err, stderr.String())
	}
	text := stderr.String()
	start, end := strings.LastIndex(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return "", errors.New("không đọc được kết quả đo âm lượng")
	}
	var measured map[string]string
	if err := json.Unmarshal([]byte(text[start:end+1]), &measured); err != nil {
		return "", err
	}
	if measured["input_i"] == "-inf" {
		return "", nil
	}
	for _, key := range []string{"input_i", "input_tp", "input_lra", "input_thresh", "target_offset"} {
		if _, err := strconv.ParseFloat(measured[key], 64); err != nil {
			return "", fmt.Errorf("kết quả đo âm lượng không hợp lệ: %s", key)
		}
	}
	return fmt.Sprintf("loudnorm=I=-16:TP=-1.5:LRA=11:measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true",
		measured["input_i"], measured["input_tp"], measured["input_lra"], measured["input_thresh"], measured["target_offset"]), nil
}

func measuredLoudnorm(ctx context.Context, in string) (string, error) {
	return measuredLoudnormProgress(ctx, in, 0, nil)
}

// muxSilence 生成静音 wav 文件（拼接用）。
func writeSilenceWAV(path string, ms int) error {
	data := silenceWAV(ms, SampleRate)
	if data == nil {
		return errors.New("thời lượng im lặng không hợp lệ")
	}
	return writeFileAtomic(path, data)
}
