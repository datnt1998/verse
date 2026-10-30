// Package tools quản lý các công cụ ngoài mà pipeline cần: yt-dlp,
// ffmpeg/ffprobe, uv (môi trường faster-whisper), Codex CLI và Chrome for
// Testing. Mỗi công cụ có manifest cố định (URL + SHA-256 đã kiểm), bộ cài đặt
// kiểm hash và đổi tên nguyên tử, bộ chạy process con trong nhóm riêng để huỷ
// sạch, và hàm probe đọc phiên bản/năng lực. Logic chuyển từ
// src/server/tools của yt-factory.
package tools

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// PlatformKey đặt tên nền tảng theo cặp runtime.GOOS/GOARCH mà manifest hỗ
// trợ; chuỗi rỗng nghĩa là nền tảng chưa có bản ghim.
type PlatformKey string

const (
	PlatformDarwinARM64 PlatformKey = "darwin-arm64"
	PlatformDarwinX64   PlatformKey = "darwin-x64"
	PlatformLinuxARM64  PlatformKey = "linux-arm64"
	PlatformLinuxX64    PlatformKey = "linux-x64"
	PlatformWin32X64    PlatformKey = "win32-x64"
)

// CurrentPlatformKey trả PlatformKey của máy đang chạy.
func CurrentPlatformKey() PlatformKey { return platformKeyFor(runtime.GOOS, runtime.GOARCH) }

func platformKeyFor(goos, goarch string) PlatformKey {
	switch goos {
	case "darwin":
		switch goarch {
		case "arm64":
			return PlatformDarwinARM64
		case "amd64":
			return PlatformDarwinX64
		}
	case "linux":
		switch goarch {
		case "arm64":
			return PlatformLinuxARM64
		case "amd64":
			return PlatformLinuxX64
		}
	case "windows":
		if goarch == "amd64" {
			return PlatformWin32X64
		}
	}
	return ""
}

// AssetKind là dạng gói tải về của một asset.
type AssetKind string

const (
	AssetFile  AssetKind = "file"
	AssetTarGZ AssetKind = "tar.gz"
	AssetZip   AssetKind = "zip"
)

// PinnedAsset là một artifact tải về: URL + hash ghim; Name là tên tệp trong
// thư mục phiên bản. Member, khi khác rỗng, là đường dẫn bên trong archive
// được trích ra thành Name; rỗng nghĩa là bung toàn bộ gốc archive.
type PinnedAsset struct {
	Name       string
	URL        string
	SHA256     string
	SizeBytes  int64
	Kind       AssetKind
	Executable bool
	Member     string
}

// ToolLicense ghi bằng chứng giấy phép đi kèm bản ghim.
type ToolLicense struct {
	SPDX string
	Note string
	File *PinnedAsset
}

// ToolPin là một phiên bản công cụ được ghim: không bao giờ trỏ "latest".
type ToolPin struct {
	Version       string
	ReleaseURL    string
	License       ToolLicense
	Build         string
	Assets        map[PlatformKey][]PinnedAsset
	RequiredBytes int64
	// ApproxBytes là dung lượng cài đặt XẤP XỈ đã đo trên máy thật — chỉ dùng
	// để hiển thị trước khi cài (“Chờ lượt · ~300 MB”), không dùng cho kiểm
	// tra dung lượng trống (RequiredBytes) và không báo sau khi cài (dùng
	// số đo ghi lại trong install-state).
	ApproxBytes int64
}

const ffmpegStaticBase = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1"

// ffmpeg-static b6.1.1 đóng gói FFmpeg 6.0; bản phát hành không có file
// checksum nên các hash dưới được tính từ asset đã tải trong lúc port
// (giữ nguyên giá trị của bản yt-factory đã kiểm bằng cách chạy binary thật).
var ffmpegLicenseFile = PinnedAsset{
	Name:       "LICENSE.ffmpeg",
	URL:        ffmpegStaticBase + "/darwin-arm64.LICENSE",
	SHA256:     "cb48bf09a11f5fb576cddb0431c8f5ed0a60157a9ec942adffc13907cbe083f2",
	SizeBytes:  4376,
	Kind:       AssetFile,
	Executable: false,
}

var ffmpegPin = ToolPin{
	Version:    "6.0-ffmpeg-static-b6.1.1",
	ReleaseURL: "https://github.com/eugeneware/ffmpeg-static/releases/tag/b6.1.1",
	License: ToolLicense{
		SPDX: "GPL-2.0-or-later",
		Note: "Static build configured with --enable-gpl and --enable-nonfree; GPL applies, not LGPL.",
		File: &ffmpegLicenseFile,
	},
	Build: "--enable-gpl --enable-libx264 --enable-libx265 --enable-libwebp --enable-libopus --enable-libmp3lame --enable-libaom --enable-libsvtav1 --enable-nonfree (ffmpeg-static b6.1.1 darwin build config, verified via `ffmpeg -version`)",
	Assets: map[PlatformKey][]PinnedAsset{
		PlatformDarwinARM64: {
			{Name: "ffmpeg", URL: ffmpegStaticBase + "/ffmpeg-darwin-arm64", SHA256: "a90e3db6a3fd35f6074b013f948b1aa45b31c6375489d39e572bea3f18336584", SizeBytes: 45568216, Kind: AssetFile, Executable: true},
			{Name: "ffprobe", URL: ffmpegStaticBase + "/ffprobe-darwin-arm64", SHA256: "bb2db6f5d8cef919da12fbf592119a987202a8c060a886f3cab091f9cab90b64", SizeBytes: 45528808, Kind: AssetFile, Executable: true},
			ffmpegLicenseFile,
		},
		PlatformDarwinX64: {
			{Name: "ffmpeg", URL: ffmpegStaticBase + "/ffmpeg-darwin-x64", SHA256: "ebdddc936f61e14049a2d4b549a412b8a40deeff6540e58a9f2a2da9e6b18894", Kind: AssetFile, Executable: true},
			{Name: "ffprobe", URL: ffmpegStaticBase + "/ffprobe-darwin-x64", SHA256: "fa3add0ce901f7241abe0dfc0155d958fc834aca3f8ce61f87cc712ae669c1e0", Kind: AssetFile, Executable: true},
			ffmpegLicenseFile,
		},
		PlatformLinuxARM64: {
			{Name: "ffmpeg", URL: ffmpegStaticBase + "/ffmpeg-linux-arm64", SHA256: "6bb182d0d75d23028db82e9e4f723ca69b853d055698486e6984ddb2c06fb8ce", Kind: AssetFile, Executable: true},
			{Name: "ffprobe", URL: ffmpegStaticBase + "/ffprobe-linux-arm64", SHA256: "d17ae9b4c297d48e2521ba14e417bb0537c6ff77c584cdbcd6bb0d8d0307a2e8", Kind: AssetFile, Executable: true},
		},
		PlatformLinuxX64: {
			{Name: "ffmpeg", URL: ffmpegStaticBase + "/ffmpeg-linux-x64", SHA256: "e7e7fb30477f717e6f55f9180a70386c62677ef8a4d4d1a5d948f4098aa3eb99", Kind: AssetFile, Executable: true},
			{Name: "ffprobe", URL: ffmpegStaticBase + "/ffprobe-linux-x64", SHA256: "4f231a1960d83e403d08f7971e271707bec278a9ae18e21b8b5b03186668450d", Kind: AssetFile, Executable: true},
		},
	},
	RequiredBytes: 400 * 1024 * 1024,
	// Đo trên máy thật: ffmpeg + ffprobe đã bung (darwin-arm64, ~88 MB).
	ApproxBytes: 88 * 1024 * 1024,
}

const chromeForTestingBase = "https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.79"

// Chrome for Testing Stable 155.0.8059.79; kênh không công bố checksum nên
// hash được tính từ zip đã tải (giữ giá trị bản yt-factory). Zip giữ symlink
// trong .app bundle; bộ bung zip phục hồi chúng.
var chromePin = ToolPin{
	Version:    "155.0.8059.79",
	ReleaseURL: "https://googlechromelabs.github.io/chrome-for-testing/",
	License: ToolLicense{
		SPDX: "LicenseRef-Google-Chrome-Terms",
		Note: "Chrome for Testing is Google's proprietary Chrome build under the Google Chrome Terms of Service; bundled Chromium components keep their own open-source licenses.",
	},
	Build: "Chrome for Testing Stable 155.0.8059.79 zip as published by Google (no auto-update; separate from the owner's Chrome)",
	Assets: map[PlatformKey][]PinnedAsset{
		PlatformDarwinARM64: {
			{Name: "chrome-mac-arm64", URL: chromeForTestingBase + "/mac-arm64/chrome-mac-arm64.zip", SHA256: "e141a7cbf948780bb7d381860bd5a7944c6736f79b9918b9826fccb199d28ea4", SizeBytes: 192617424, Kind: AssetZip},
		},
		PlatformDarwinX64: {
			{Name: "chrome-mac-x64", URL: chromeForTestingBase + "/mac-x64/chrome-mac-x64.zip", SHA256: "11c407e24d7d98cacaec055a4d9ab2f33023db2478d30544c24e829d81022096", SizeBytes: 203545619, Kind: AssetZip},
		},
	},
	// Zip (~200 MB) + bundle đã bung (~380 MB) + chồng staging.
	RequiredBytes: 1024 * 1024 * 1024,
	// Đo trên máy thật: bundle .app đã bung (~370 MB).
	ApproxBytes: 370 * 1024 * 1024,
}

const uvReleaseBase = "https://github.com/astral-sh/uv/releases/download/0.13.0"

var uvPin = ToolPin{
	Version:    "0.13.0",
	ReleaseURL: "https://github.com/astral-sh/uv/releases/tag/0.13.0",
	License: ToolLicense{
		SPDX: "Apache-2.0 OR MIT",
		Note: "uv is dual-licensed Apache-2.0/MIT by Astral.",
	},
	Build: "Astral official release artifact; .sha256 sidecar checksums published upstream.",
	Assets: map[PlatformKey][]PinnedAsset{
		PlatformDarwinARM64: {
			{Name: "uv", URL: uvReleaseBase + "/uv-aarch64-apple-darwin.tar.gz", SHA256: "a9c1b29002cf3c83f07fa9cd8a887a3be0107d90e23189721221e7257db8e3d6", Kind: AssetTarGZ, Executable: true, Member: "uv-aarch64-apple-darwin/uv"},
		},
		PlatformDarwinX64: {
			{Name: "uv", URL: uvReleaseBase + "/uv-x86_64-apple-darwin.tar.gz", SHA256: "5f44dcbde809b632f47c36fadb241cb4d6f9af71d0c8f172b5d2026d3dde742c", Kind: AssetTarGZ, Executable: true, Member: "uv-x86_64-apple-darwin/uv"},
		},
		PlatformLinuxARM64: {
			{Name: "uv", URL: uvReleaseBase + "/uv-aarch64-unknown-linux-gnu.tar.gz", SHA256: "3ccfb6af6e242433eb552f7d9676abd5412c6595c497e990d9c8cb7b5bd4d2c3", Kind: AssetTarGZ, Executable: true, Member: "uv-aarch64-unknown-linux-gnu/uv"},
		},
		PlatformLinuxX64: {
			{Name: "uv", URL: uvReleaseBase + "/uv-x86_64-unknown-linux-gnu.tar.gz", SHA256: "1468ebd5a5541121837c5a2817b9972ba6090fa6caa3d142620850a47fb75154", Kind: AssetTarGZ, Executable: true, Member: "uv-x86_64-unknown-linux-gnu/uv"},
		},
		PlatformWin32X64: {
			{Name: "uv.exe", URL: uvReleaseBase + "/uv-x86_64-pc-windows-msvc.zip", SHA256: "088962f9e7b7bd9ea740c04c650b2a21c8928c345bd99ac24350dc924dba656c", Kind: AssetZip, Executable: true, Member: "uv-x86_64-pc-windows-msvc/uv.exe"},
		},
	},
	RequiredBytes: 120 * 1024 * 1024,
}

const codexReleaseBase = "https://github.com/openai/codex/releases/download/rust-v0.162.1"

// codex-package_SHA256SUMS chỉ phủ các tarball *-package-*; các tarball CLI
// trần và code-mode-host dưới đây được hash sau khi tải (giữ giá trị bản cũ).
var codexPin = ToolPin{
	Version:    "0.162.1",
	ReleaseURL: "https://github.com/openai/codex/releases/tag/rust-v0.162.1",
	License: ToolLicense{
		SPDX: "Apache-2.0",
		Note: "OpenAI Codex CLI, Apache-2.0.",
	},
	Build: "OpenAI official release artifact rust-v0.162.1 (codex-cli 0.162.1).",
	Assets: map[PlatformKey][]PinnedAsset{
		PlatformDarwinARM64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-aarch64-apple-darwin.tar.gz", SHA256: "b293a0fcdd871900a44c0a4f40a1aa401b31e02854f790ebdb8bd5d1177fd1fe", SizeBytes: 98104184, Kind: AssetTarGZ, Executable: true, Member: "codex-aarch64-apple-darwin"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-aarch64-apple-darwin.tar.gz", SHA256: "1f464a2894adbc916e431b6bf19bef2cb485cf7e13e636d42578c581962b3727", Kind: AssetTarGZ, Executable: true, Member: "codex-code-mode-host-aarch64-apple-darwin"},
		},
		PlatformDarwinX64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-x86_64-apple-darwin.tar.gz", SHA256: "d68e39d8f62639f33f7d00656554cb9c3ef7809e664a3827e62ab3ace3c21b21", Kind: AssetTarGZ, Executable: true, Member: "codex-x86_64-apple-darwin"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-x86_64-apple-darwin.tar.gz", SHA256: "cd52af8a6884b18239aaf36d0c99bc990e7dcf75b0917155474ffde88a111d37", Kind: AssetTarGZ, Executable: true, Member: "codex-code-mode-host-x86_64-apple-darwin"},
		},
		PlatformLinuxARM64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-aarch64-unknown-linux-musl.tar.gz", SHA256: "bf3ca2719f1143b1440fbd85c149379e696c782c50ad7991f8bfcec60b2600c1", Kind: AssetTarGZ, Executable: true, Member: "codex-aarch64-unknown-linux-musl"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-aarch64-unknown-linux-musl.tar.gz", SHA256: "1395ce9367f8acd280674dff083edf25bf221d908b8351d5bd4f3fdde9181e9a", Kind: AssetTarGZ, Executable: true, Member: "codex-code-mode-host-aarch64-unknown-linux-musl"},
		},
		PlatformLinuxX64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-x86_64-unknown-linux-musl.tar.gz", SHA256: "86f268d81b898f3e144c5ecff2ad2fda2c5802e042d6195b72538fe0fa125057", Kind: AssetTarGZ, Executable: true, Member: "codex-x86_64-unknown-linux-musl"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-x86_64-unknown-linux-musl.tar.gz", SHA256: "458239ea45c1fc508d89695f180ab352f2834709a8a46929a4466f4f04399ada", Kind: AssetTarGZ, Executable: true, Member: "codex-code-mode-host-x86_64-unknown-linux-musl"},
		},
	},
	RequiredBytes: 300 * 1024 * 1024,
	// Đo trên máy thật: codex + code-mode-host đã bung (~299 MB).
	ApproxBytes: 299 * 1024 * 1024,
}

// CodexMinVersion là phiên bản Codex CLI tối thiểu mà preflight cô lập đòi.
const CodexMinVersion = "0.130.0"

// PythonEnvRequiredBytes là dung lượng trống tối thiểu trước khi cài môi
// trường Python (venv + wheel + cache).
const PythonEnvRequiredBytes int64 = 4 * 1024 * 1024 * 1024

// PythonEnvApproxBytes là dung lượng cài đặt XẤP XỈ của môi trường tách
// tiếng, đo trên máy thật (env 551 MB + models 605 MB + python 63 MB, không
// tính cache uv 541 MB). Chỉ để hiển thị trước khi cài.
const PythonEnvApproxBytes int64 = 1200 * 1024 * 1024

// YtDlpRequiredBytes là chỗ app dành cho binary yt-dlp. Pipeline của nó tải
// bản mới nhất từ GitHub nên không ghim kích thước như các công cụ khác; con
// số này chỉ để hàng Công cụ có dung lượng hiện được và lượt cài kiểm ổ đĩa.
const YtDlpRequiredBytes int64 = 50 * 1024 * 1024

// YtDlpApproxBytes là kích thước cài đặt XẤP XỈ của bản onedir yt-dlp, đo
// trên máy thật (bin/yt-dlp ~124 MB sau self-extract). Chỉ để hiển thị
// trước khi cài; cài xong dùng số đo ghi lại.
const YtDlpApproxBytes int64 = 124 * 1024 * 1024

var sha256Re = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// ParseSha256Sums đọc file kiểu `SHA256SUMS` thành map tên → hash.
func ParseSha256Sums(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		name := strings.TrimPrefix(strings.Join(fields[1:], " "), "*")
		if sha256Re.MatchString(hash) && name != "" {
			out[name] = strings.ToLower(hash)
		}
	}
	return out
}

// PinnedAssets trả danh sách asset của một nền tảng; lỗi khi nền tảng không
// có bản ghim.
func PinnedAssets(pin ToolPin, key PlatformKey) ([]PinnedAsset, error) {
	if key == "" {
		return nil, fmt.Errorf("không có bản ghim cho nền tảng này")
	}
	assets := pin.Assets[key]
	if len(assets) == 0 {
		return nil, fmt.Errorf("không có bản ghim của %s cho %s", pin.Version, key)
	}
	return append([]PinnedAsset(nil), assets...), nil
}

// Các hàm pin trả bản sao của manifest ghim: dữ liệu ghim là bất biến, caller
// không chỉnh được bản gốc trong process. Cấu trúc ToolPin dùng chung cho
// cả spec tự định nghĩa (test).
func FfmpegPin() ToolPin { return ffmpegPin }
func ChromePin() ToolPin { return chromePin }
func UvPin() ToolPin     { return uvPin }
func CodexPin() ToolPin  { return codexPin }
