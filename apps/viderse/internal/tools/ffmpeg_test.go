// Test chuyển từ src/server/tools/ffmpeg.test.ts của yt-factory.
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/platform/toolchain"
)

// ffmpegRunner trả banner giả theo lệnh được gọi.
type ffmpegRunner struct {
	versionOut  string
	filtersOut  string
	encodersOut string
	err         error
}

func (f *ffmpegRunner) Run(ctx context.Context, file string, args []string, spec Spec) (Result, error) {
	if f.err != nil {
		return Result{}, f.err
	}
	if len(args) > 0 {
		switch args[len(args)-1] {
		case "-version":
			return Result{Stdout: f.versionOut}, nil
		case "-filters":
			return Result{Stdout: f.filtersOut}, nil
		case "-encoders":
			return Result{Stdout: f.encodersOut}, nil
		}
	}
	return Result{}, &ProcessFailedError{Message: "spawn ENOENT", Code: -1}
}

const ffVersionBanner = `ffmpeg version 6.0-essentials_build-www.gyan.dev Copyright (c) 2000-2023
built with gcc 12.2.0 (Rev10, Built by MSYS2 project)
configuration: --enable-gpl --enable-libx264 --enable-libx265 --arch=arm64 --enable-neon
libavutil      58.  2.100 / 58.  2.100
`

const ffFilters = `Filters:
 T.. ebur128           V->N       EBU R128 scanner
 TSC select            V->V       Select video frames to pass in output
 TS. scdet             V->V       Detect video scene change
`

const ffEncoders = `Encoders:
 V..... libwebp              libwebp WebP image
 A..D.. aac                  AAC (Advanced Audio Coding)
 A.D... pcm_s16le            PCM signed 16-bit little-endian
 V..... libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10
`

func okFfmpegRunner() *ffmpegRunner {
	return &ffmpegRunner{versionOut: ffVersionBanner, filtersOut: ffFilters, encodersOut: ffEncoders}
}

func TestParseFfmpegVersionBanner(t *testing.T) {
	if v := ParseFfmpegVersion(ffVersionBanner); v != "6.0-essentials_build-www.gyan.dev" {
		t.Fatalf("version = %q", v)
	}
	if a := ParseFfmpegArch(ffVersionBanner); a != "arm64" {
		t.Fatalf("arch = %q", a)
	}
	if a := ParseFfmpegArch("configuration: --enable-gpl"); a != "" {
		t.Fatalf("arch = %q", a)
	}
	if a := ParseFfmpegArch("configuration: --target-os=linux --arch=x86_64"); a != "x64" {
		t.Fatalf("arch = %q", a)
	}
}

func TestParseFfmpegCaps(t *testing.T) {
	caps := ParseFfmpegCaps(ffFilters, ffEncoders)
	if !caps.Filters["ebur128"] || !caps.Filters["select"] || !caps.Filters["scdet"] {
		t.Fatalf("caps = %+v", caps)
	}
	if !caps.Encoders["libwebp"] || !caps.Encoders["aac"] || !caps.Encoders["pcm_s16le"] {
		t.Fatalf("caps = %+v", caps)
	}
	if caps.H264 != "libx264" {
		t.Fatalf("h264 = %q", caps.H264)
	}
	if missing := MissingFfmpegCaps(caps); len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	// Bản thiếu bộ lọc báo đúng phần thiếu.
	caps = ParseFfmpegCaps(" T.. ebur128           V->N       EBU R128 scanner\n", " A..D.. aac                  AAC\n")
	missing := MissingFfmpegCaps(caps)
	want := map[string]bool{"filter:select": true, "filter:scdet": true, "encoder:libwebp": true, "encoder:pcm_s16le": true, "encoder:h264": true}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v", missing)
	}
	for _, m := range missing {
		if !want[m] {
			t.Fatalf("missing = %v", missing)
		}
	}
}

func TestFfmpegProbeReadsVersionAndCaps(t *testing.T) {
	exe := writeExecutable(t, t.TempDir(), "ffmpeg")
	f := &Ffmpeg{d: Deps{Runner: okFfmpegRunner(), BaseDir: t.TempDir()}}
	probe := f.Probe(context.Background(), exe)
	if probe == nil {
		t.Fatal("probe nil")
	}
	if probe.Version != "6.0-essentials_build-www.gyan.dev" || probe.Arch != "arm64" {
		t.Fatalf("probe = %+v", probe)
	}
	if len(probe.Missing) != 0 {
		t.Fatalf("missing = %v", probe.Missing)
	}
}

func TestFfmpegProbeNilOnFailure(t *testing.T) {
	f := &Ffmpeg{d: Deps{Runner: &fakeRunner{}, BaseDir: t.TempDir()}}
	if probe := f.Probe(context.Background(), "/missing/ffmpeg"); probe != nil {
		t.Fatalf("probe = %+v", probe)
	}
}

func TestFfmpegResolveOrderConfiguredSystemManaged(t *testing.T) {
	base := t.TempDir()
	// Managed: dựng một bản active.
	verDir := toolchain.ToolVersionDir(base, ffmpegToolID, toolchain.FfmpegPin().Version)
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(verDir, "ffmpeg"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolchain.ToolRoot(base, ffmpegToolID), "current"), []byte("v-"+toolchain.FfmpegPin().Version), 0o644); err != nil {
		t.Fatal(err)
	}
	// System: một dir trong PATH.
	sysDir := t.TempDir()
	sysExe := filepath.Join(sysDir, "ffmpeg")
	if err := os.WriteFile(sysExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Configured.
	cfgDir := t.TempDir()
	cfgExe := filepath.Join(cfgDir, "ffmpeg")
	if err := os.WriteFile(cfgExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := &FilePathStore{Dir: t.TempDir()}
	if err := store.Set(FfmpegSettingKey, cfgExe); err != nil {
		t.Fatal(err)
	}
	f := &Ffmpeg{d: Deps{Runner: okFfmpegRunner(), Paths: store, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	res := f.Resolve(context.Background())
	if res == nil || res.Ffmpeg != cfgExe || res.Source != SourceConfigured {
		t.Fatalf("res = %+v", res)
	}
	// Bỏ cấu hình → hệ thống thắng managed.
	f.d.Paths = &FilePathStore{Dir: t.TempDir()}
	res = f.Resolve(context.Background())
	if res == nil || res.Ffmpeg != sysExe || res.Source != SourceSystem {
		t.Fatalf("res = %+v", res)
	}
	// Bỏ cả hệ thống → managed.
	f.d.PathEnv = t.TempDir()
	f.d.Home = t.TempDir() // không có .local/bin/ffmpeg
	res = f.Resolve(context.Background())
	if res == nil || res.Source != SourceManaged {
		t.Fatalf("res = %+v", res)
	}
}

func TestFfmpegStatusConfiguredBrokenIsExplicit(t *testing.T) {
	base := t.TempDir()
	store := &FilePathStore{Dir: t.TempDir()}
	if err := store.Set(FfmpegSettingKey, "/missing/ffmpeg"); err != nil {
		t.Fatal(err)
	}
	f := &Ffmpeg{d: Deps{Runner: okFfmpegRunner(), Paths: store, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	st := f.Status(context.Background())
	if st.State != FfmpegBroken || st.Source != SourceConfigured {
		t.Fatalf("status = %+v", st)
	}
}
