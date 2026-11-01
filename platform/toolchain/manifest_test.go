// Test chuyển từ src/server/tools/tool-manifest.test.ts của yt-factory.
package toolchain

import (
	"runtime"
	"strings"
	"testing"
)

func TestPlatformKeyCoversCurrentMachine(t *testing.T) {
	key := CurrentPlatformKey()
	if key == "" && runtime.GOOS == "darwin" {
		t.Fatal("darwin cần được ghim cho mọi công cụ")
	}
	if runtime.GOOS == "darwin" {
		assets, err := PinnedAssets(FfmpegPin(), key)
		if err != nil || len(assets) == 0 {
			t.Fatalf("ffmpeg không có asset cho %s", key)
		}
	}
}

func TestPinnedAssetsRejectsUnsupportedPlatform(t *testing.T) {
	if _, err := PinnedAssets(FfmpegPin(), PlatformKey("haiku-x64")); err == nil {
		t.Fatal("không báo rõ nền tảng không hỗ trợ")
	}
}

func TestUvPinCoversEverySupportedPlatform(t *testing.T) {
	for _, key := range []PlatformKey{
		PlatformDarwinARM64, PlatformDarwinX64, PlatformLinuxARM64, PlatformLinuxX64, PlatformWin32X64,
	} {
		assets, err := PinnedAssets(UvPin(), key)
		if err != nil {
			t.Fatal(err)
		}
		if len(assets) == 0 || !assets[0].Executable {
			t.Fatalf("%s thiếu binary thực thi", key)
		}
		if len(assets[0].SHA256) != 64 {
			t.Fatalf("checksum của %s không phải SHA-256", key)
		}
	}
}

func TestUvPinUsesLatestRelease(t *testing.T) {
	if UvPin().Version != "0.13.0" {
		t.Fatalf("uv pin = %q", UvPin().Version)
	}
}

func TestParseSha256SumsHandlesStandardFormat(t *testing.T) {
	a := strings.Repeat("ab", 32)
	b := strings.Repeat("cd", 32)
	sums := ParseSha256Sums(a + "  yt-dlp_macos.zip\n" + b + " *yt-dlp_linux\n# comment\n")
	if sums["yt-dlp_macos.zip"] != a {
		t.Fatalf("sums = %v", sums)
	}
	if sums["yt-dlp_linux"] != b {
		t.Fatalf("sums = %v", sums)
	}
	if len(sums) != 2 {
		t.Fatalf("sums = %v", sums)
	}
}

func TestEveryPinnedAssetIsPinnedBySha256(t *testing.T) {
	for _, pin := range []ToolPin{FfmpegPin(), UvPin()} {
		for platform, assets := range pin.Assets {
			for _, a := range assets {
				if len(a.SHA256) != 64 {
					t.Fatalf("%s @ %s (%s) thiếu SHA-256", pin.Version, platform, a.Name)
				}
			}
		}
	}
}
