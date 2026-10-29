// ffmpeg/ffprobe cho pipeline. Thứ tự chọn bản chạy: đường dẫn người dùng đã
// chọn → binary hệ thống → bản app tự quản. Mọi trạng thái đều được chứng
// minh bằng cách chạy binary: phiên bản, arch và bộ filter/encoder pipeline
// cần (ebur128, select/scdet, libwebp, pcm, aac, h264).
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const ffmpegToolID = "ffmpeg"

// FfmpegSettingKey là key PathStore của ffmpeg.
const FfmpegSettingKey = "tools:ffmpeg"

// Bộ năng lực tối thiểu pipeline đòi.
var (
	FfmpegFilters      = []string{"ebur128", "select", "scdet"}
	FfmpegEncoders     = []string{"libwebp", "aac", "pcm_s16le"}
	FfmpegH264Encoders = []string{"libx264", "h264_videotoolbox", "h264"}
)

// FfmpegCaps là mặt năng lực đã chứng minh của một binary.
type FfmpegCaps struct {
	Filters  map[string]bool
	Encoders map[string]bool
	H264     string // encoder h264 tìm được; "" khi không có
}

// FfmpegProbe là kết quả chạy thử một binary.
type FfmpegProbe struct {
	Path    string
	Version string
	Arch    string // "arm64"/"x64" đọc từ dòng configuration; "" khi không rõ
	Caps    FfmpegCaps
	Missing []string
}

// FfmpegState là trạng thái một binary đã probe.
type FfmpegState string

const (
	FfmpegReady        FfmpegState = "ready"
	FfmpegIncompatible FfmpegState = "incompatible"
	FfmpegMissing      FfmpegState = "missing"
	FfmpegBroken       FfmpegState = "broken"
)

// FfmpegStatus tóm tắt trạng thái ffmpeg của máy cho Cài đặt.
type FfmpegStatus struct {
	State    FfmpegState
	Source   ToolSource
	Path     string
	Version  string
	Arch     string
	Caps     FfmpegCaps
	Missing  []string
	Detected []FfmpegProbe // các bản hệ thống chạy được, khi chưa có bản dùng được
	Error    string        // lỗi khi state=broken
}

// Ffmpeg quản lý cặp ffmpeg/ffprobe.
type Ffmpeg struct{ d Deps }

var (
	ffmpegVersionRe = regexp.MustCompile(`ffmpeg version\s+(\S+)`)
	ffmpegArchRe    = regexp.MustCompile(`configuration:.*--arch=([\w-]+)`)
	ffmpegNeonRe    = regexp.MustCompile(`--enable-neon`)
	ffmpegArmRe     = regexp.MustCompile(`aarch64|arm64`)
	ffmpegX64Re     = regexp.MustCompile(`x86_64|amd64`)
)

// ParseFfmpegVersion đọc số phiên bản từ banner `ffmpeg -version`.
func ParseFfmpegVersion(stdout string) string {
	if m := ffmpegVersionRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	return ""
}

// ParseFfmpegArch đọc arch từ dòng configuration, chuẩn hoá về arm64/x64;
// rỗng khi không nhận ra.
func ParseFfmpegArch(versionOut string) string {
	raw := ""
	if m := ffmpegArchRe.FindStringSubmatch(versionOut); m != nil {
		raw = m[1]
	}
	switch {
	case ffmpegArmRe.MatchString(raw) || ffmpegNeonRe.MatchString(versionOut):
		return "arm64"
	case ffmpegX64Re.MatchString(raw):
		return "x64"
	}
	if ffmpegArmRe.MatchString(versionOut) {
		return "arm64"
	}
	if ffmpegX64Re.MatchString(versionOut) {
		return "x64"
	}
	return ""
}

// ParseFfmpegCaps đọc bảng filter/encoder từ `-filters` và `-encoders`.
func ParseFfmpegCaps(filtersOut, encodersOut string) FfmpegCaps {
	filterNames := map[string]bool{}
	for _, line := range strings.Split(filtersOut, "\n") {
		if f := strings.Fields(line); len(f) >= 2 {
			filterNames[f[1]] = true
		}
	}
	encoderNames := map[string]bool{}
	for _, line := range strings.Split(encodersOut, "\n") {
		if f := strings.Fields(line); len(f) >= 2 {
			encoderNames[f[1]] = true
		}
	}
	caps := FfmpegCaps{
		Filters:  map[string]bool{},
		Encoders: map[string]bool{},
	}
	for _, f := range FfmpegFilters {
		caps.Filters[f] = filterNames[f]
	}
	for _, e := range FfmpegEncoders {
		caps.Encoders[e] = encoderNames[e]
	}
	for _, e := range FfmpegH264Encoders {
		if encoderNames[e] {
			caps.H264 = e
			break
		}
	}
	return caps
}

// MissingFfmpegCaps liệt kê phần năng lực còn thiếu dưới dạng "filter:x"/
// "encoder:x".
func MissingFfmpegCaps(caps FfmpegCaps) []string {
	var missing []string
	for _, f := range FfmpegFilters {
		if !caps.Filters[f] {
			missing = append(missing, "filter:"+f)
		}
	}
	for _, e := range FfmpegEncoders {
		if !caps.Encoders[e] {
			missing = append(missing, "encoder:"+e)
		}
	}
	if caps.H264 == "" {
		missing = append(missing, "encoder:h264")
	}
	return missing
}

// Probe chạy một binary ffmpeg và báo phiên bản + mặt năng lực; nil khi
// binary không chạy hoặc banner không đọc được.
func (f *Ffmpeg) Probe(ctx context.Context, exe string) *FfmpegProbe {
	// Đường dẫn không tồn tại không thể là binary; chặn trước khi spawn để
	// ứng viên hỏng không che ứng viên sau.
	if !fileExists(exe) {
		return nil
	}
	run := func(args ...string) (Result, error) {
		return f.d.Runner.Run(ctx, exe, args, Spec{Timeout: 30 * time.Second})
	}
	v, err := run("-hide_banner", "-version")
	if err != nil {
		return nil
	}
	filters, err := run("-hide_banner", "-filters")
	if err != nil {
		return nil
	}
	encoders, err := run("-hide_banner", "-encoders")
	if err != nil {
		return nil
	}
	version := ParseFfmpegVersion(v.Stdout)
	if version == "" {
		return nil
	}
	caps := ParseFfmpegCaps(filters.Stdout, encoders.Stdout)
	return &FfmpegProbe{
		Path:    exe,
		Version: version,
		Arch:    ParseFfmpegArch(v.Stdout),
		Caps:    caps,
		Missing: MissingFfmpegCaps(caps),
	}
}

// Candidates liệt kê executable hệ thống đáng thử: PATH + chỗ cài chuẩn.
func (f *Ffmpeg) Candidates() []string {
	var out []string
	if onPath := FindOnPath("ffmpeg", f.d.pathEnv()); onPath != "" {
		out = append(out, onPath)
	}
	for _, dir := range f.d.standardDirs() {
		out = append(out, filepath.Join(dir, "ffmpeg"))
	}
	if runtime.GOOS != "windows" {
		out = append(out, filepath.Join(f.d.home(), ".local", "bin", "ffmpeg"))
	}
	return dedupeStrings(out)
}

// ConfiguredPath trả ffmpeg do người dùng chọn; "" khi không có.
func (f *Ffmpeg) ConfiguredPath() string { return f.d.configuredPath(FfmpegSettingKey) }

// SetConfiguredPath ghi/xoá ffmpeg do người dùng chọn.
func (f *Ffmpeg) SetConfiguredPath(exe string) error {
	return f.d.SetConfiguredPath(FfmpegSettingKey, exe)
}

// ManagedPath trả ffmpeg trong bản cài app quản; "" khi chưa cài.
func (f *Ffmpeg) ManagedPath() string {
	dir := ActiveVersionDir(f.d.BaseDir, ffmpegToolID)
	if dir == "" {
		return ""
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	exe := filepath.Join(dir, name)
	if fileExists(exe) {
		return exe
	}
	return ""
}

// Resolved là binary ffmpeg pipeline nên gọi cùng probe của nó.
type FfmpegResolved struct {
	Ffmpeg  string
	Ffprobe string // sibling cùng thư mục; "" khi không có
	Source  ToolSource
	Probe   *FfmpegProbe
}

// Resolve trả binary đầu tiên trong thứ tự cấu hình → hệ thống → app quản mà
// probe đầy đủ năng lực; nil khi không có bản dùng được.
func (f *Ffmpeg) Resolve(ctx context.Context) *FfmpegResolved {
	for _, c := range resolutionOrder(f.ConfiguredPath(), f.Candidates(), f.ManagedPath()) {
		probe := f.Probe(ctx, c.path)
		if probe != nil && len(probe.Missing) == 0 {
			return &FfmpegResolved{
				Ffmpeg:  c.path,
				Ffprobe: f.ffprobeSibling(c.path),
				Source:  c.source,
				Probe:   probe,
			}
		}
	}
	return nil
}

// ffprobeSibling tìm ffprobe cạnh ffmpeg; rớt về PATH khi vắng.
func (f *Ffmpeg) ffprobeSibling(ffmpegPath string) string {
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	sibling := filepath.Join(filepath.Dir(ffmpegPath), name)
	if fileExists(sibling) {
		return sibling
	}
	return FindOnPath(name, f.d.pathEnv())
}

// Status tổng hợp cho Cài đặt: bản đã chọn là bản đã probe; đường dẫn cấu
// hình hỏng báo broken dù hệ thống còn bản khác.
func (f *Ffmpeg) Status(ctx context.Context) FfmpegStatus {
	cands := resolutionOrder(f.ConfiguredPath(), f.Candidates(), f.ManagedPath())
	var detected []FfmpegProbe
	var firstProbe *FfmpegProbe
	var firstSource ToolSource
	for _, c := range cands {
		if c.source == SourceConfigured {
			probe := f.Probe(ctx, c.path)
			if probe == nil {
				return FfmpegStatus{State: FfmpegBroken, Source: c.source, Path: c.path,
					Error: "Không chạy được ffmpeg ở đường dẫn đã chọn"}
			}
			if len(probe.Missing) > 0 {
				return FfmpegStatus{State: FfmpegIncompatible, Source: c.source,
					Path: probe.Path, Version: probe.Version, Arch: probe.Arch, Caps: probe.Caps, Missing: probe.Missing}
			}
			return FfmpegStatus{State: FfmpegReady, Source: c.source,
				Path: probe.Path, Version: probe.Version, Arch: probe.Arch, Caps: probe.Caps}
		}
		probe := f.Probe(ctx, c.path)
		if probe == nil {
			continue
		}
		if firstProbe == nil {
			firstProbe, firstSource = probe, c.source
		}
		if c.source == SourceSystem {
			detected = append(detected, *probe)
		}
		if len(probe.Missing) == 0 {
			return FfmpegStatus{State: FfmpegReady, Source: c.source,
				Path: probe.Path, Version: probe.Version, Arch: probe.Arch, Caps: probe.Caps}
		}
	}
	if firstProbe != nil {
		return FfmpegStatus{State: FfmpegIncompatible, Source: firstSource,
			Path: firstProbe.Path, Version: firstProbe.Version, Arch: firstProbe.Arch,
			Caps: firstProbe.Caps, Missing: firstProbe.Missing, Detected: detected}
	}
	return FfmpegStatus{State: FfmpegMissing, Detected: detected}
}

// InstallSpec trả pipeline cài cặp ffmpeg+ffprobe ghim vào bin/ffmpeg.
func (f *Ffmpeg) InstallSpec() InstallSpec {
	return InstallSpec{
		ToolID: ffmpegToolID,
		Pin:    FfmpegPin(),
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			name := "ffmpeg"
			if runtime.GOOS == "windows" {
				name = "ffmpeg.exe"
			}
			exe := filepath.Join(staging, name)
			probe := f.Probe(ctx, exe)
			if probe == nil {
				return "", fmt.Errorf("ffmpeg tải về không chạy được")
			}
			if len(probe.Missing) > 0 {
				return "", fmt.Errorf("ffmpeg tải về thiếu năng lực: %s", strings.Join(probe.Missing, ", "))
			}
			probeExe := f.ffprobeSibling(exe)
			if probeExe == "" {
				probeExe = filepath.Join(staging, "ffprobe")
			}
			pv, err := f.d.Runner.Run(ctx, probeExe, []string{"-hide_banner", "-version"}, Spec{Timeout: 30 * time.Second})
			if err != nil || !strings.Contains(pv.Stdout, "ffprobe version") {
				return "", fmt.Errorf("ffprobe tải về không chạy được")
			}
			return probe.Version, nil
		},
	}
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
