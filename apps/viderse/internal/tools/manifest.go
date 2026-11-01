// Package tools quản lý các công cụ ngoài mà pipeline cần: yt-dlp,
// ffmpeg/ffprobe, uv (môi trường faster-whisper), Codex CLI và Chrome for
// Testing. Manifest ghim, bộ cài kiểm hash/đổi tên nguyên tử và môi trường uv
// nằm ở platform/toolchain (dùng chung với Noverse); gói này giữ các bản ghim
// riêng của Viderse, bộ chạy process con trong nhóm riêng để huỷ sạch, và hàm
// probe đọc phiên bản/năng lực. Logic chuyển từ src/server/tools của
// yt-factory.
package tools

import (
	"github.com/datnt1998/verse/platform/toolchain"
)

const chromeForTestingBase = "https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.79"

// Chrome for Testing Stable 155.0.8059.79; kênh không công bố checksum nên
// hash được tính từ zip đã tải (giữ giá trị bản yt-factory). Zip giữ symlink
// trong .app bundle; bộ bung zip phục hồi chúng.
var chromePin = toolchain.ToolPin{
	Version:    "155.0.8059.79",
	ReleaseURL: "https://googlechromelabs.github.io/chrome-for-testing/",
	License: toolchain.ToolLicense{
		SPDX: "LicenseRef-Google-Chrome-Terms",
		Note: "Chrome for Testing is Google's proprietary Chrome build under the Google Chrome Terms of Service; bundled Chromium components keep their own open-source licenses.",
	},
	Build: "Chrome for Testing Stable 155.0.8059.79 zip as published by Google (no auto-update; separate from the owner's Chrome)",
	Assets: map[toolchain.PlatformKey][]toolchain.PinnedAsset{
		toolchain.PlatformDarwinARM64: {
			{Name: "chrome-mac-arm64", URL: chromeForTestingBase + "/mac-arm64/chrome-mac-arm64.zip", SHA256: "e141a7cbf948780bb7d381860bd5a7944c6736f79b9918b9826fccb199d28ea4", SizeBytes: 192617424, Kind: toolchain.AssetZip},
		},
		toolchain.PlatformDarwinX64: {
			{Name: "chrome-mac-x64", URL: chromeForTestingBase + "/mac-x64/chrome-mac-x64.zip", SHA256: "11c407e24d7d98cacaec055a4d9ab2f33023db2478d30544c24e829d81022096", SizeBytes: 203545619, Kind: toolchain.AssetZip},
		},
	},
	// Zip (~200 MB) + bundle đã bung (~380 MB) + chồng staging.
	RequiredBytes: 1024 * 1024 * 1024,
	// Đo trên máy thật: bundle .app đã bung (~370 MB).
	ApproxBytes: 370 * 1024 * 1024,
}

const codexReleaseBase = "https://github.com/openai/codex/releases/download/rust-v0.162.1"

// codex-package_SHA256SUMS chỉ phủ các tarball *-package-*; các tarball CLI
// trần và code-mode-host dưới đây được hash sau khi tải (giữ giá trị bản cũ).
var codexPin = toolchain.ToolPin{
	Version:    "0.162.1",
	ReleaseURL: "https://github.com/openai/codex/releases/tag/rust-v0.162.1",
	License: toolchain.ToolLicense{
		SPDX: "Apache-2.0",
		Note: "OpenAI Codex CLI, Apache-2.0.",
	},
	Build: "OpenAI official release artifact rust-v0.162.1 (codex-cli 0.162.1).",
	Assets: map[toolchain.PlatformKey][]toolchain.PinnedAsset{
		toolchain.PlatformDarwinARM64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-aarch64-apple-darwin.tar.gz", SHA256: "b293a0fcdd871900a44c0a4f40a1aa401b31e02854f790ebdb8bd5d1177fd1fe", SizeBytes: 98104184, Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-aarch64-apple-darwin"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-aarch64-apple-darwin.tar.gz", SHA256: "1f464a2894adbc916e431b6bf19bef2cb485cf7e13e636d42578c581962b3727", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-code-mode-host-aarch64-apple-darwin"},
		},
		toolchain.PlatformDarwinX64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-x86_64-apple-darwin.tar.gz", SHA256: "d68e39d8f62639f33f7d00656554cb9c3ef7809e664a3827e62ab3ace3c21b21", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-x86_64-apple-darwin"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-x86_64-apple-darwin.tar.gz", SHA256: "cd52af8a6884b18239aaf36d0c99bc990e7dcf75b0917155474ffde88a111d37", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-code-mode-host-x86_64-apple-darwin"},
		},
		toolchain.PlatformLinuxARM64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-aarch64-unknown-linux-musl.tar.gz", SHA256: "bf3ca2719f1143b1440fbd85c149379e696c782c50ad7991f8bfcec60b2600c1", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-aarch64-unknown-linux-musl"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-aarch64-unknown-linux-musl.tar.gz", SHA256: "1395ce9367f8acd280674dff083edf25bf221d908b8351d5bd4f3fdde9181e9a", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-code-mode-host-aarch64-unknown-linux-musl"},
		},
		toolchain.PlatformLinuxX64: {
			{Name: "codex", URL: codexReleaseBase + "/codex-x86_64-unknown-linux-musl.tar.gz", SHA256: "86f268d81b898f3e144c5ecff2ad2fda2c5802e042d6195b72538fe0fa125057", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-x86_64-unknown-linux-musl"},
			{Name: "codex-code-mode-host", URL: codexReleaseBase + "/codex-code-mode-host-x86_64-unknown-linux-musl.tar.gz", SHA256: "458239ea45c1fc508d89695f180ab352f2834709a8a46929a4466f4f04399ada", Kind: toolchain.AssetTarGZ, Executable: true, Member: "codex-code-mode-host-x86_64-unknown-linux-musl"},
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

// Các hàm pin trả bản sao của manifest ghim: dữ liệu ghim là bất biến, caller
// không chỉnh được bản gốc trong process. Cấu trúc ToolPin dùng chung cho
// cả spec tự định nghĩa (test).
func ChromePin() toolchain.ToolPin { return chromePin }
func CodexPin() toolchain.ToolPin  { return codexPin }
