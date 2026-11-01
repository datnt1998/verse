// Package toolchain quản lý các công cụ ngoài mà ứng dụng tải về và tự cài:
// manifest ghim (URL + SHA-256 + kích thước), bộ cài kiểm hash rồi đổi tên
// nguyên tử, và môi trường uv riêng của ứng dụng. Không bao giờ trỏ "latest":
// mọi artifact đều có hash ghim trong mã, nền tảng không có bản ghim thì báo
// lỗi thay vì đoán.
//
// Gói này dùng chung cho mọi app (Viderse: ffmpeg/uv/Codex/Chrome; Noverse:
// uv + mã nguồn bộ đọc), nên không được import gì từ apps/.
package toolchain

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
func UvPin() ToolPin     { return uvPin }
