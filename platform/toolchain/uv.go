// uv (Astral) là công cụ tải Python và dựng môi trường ảo cho các app. Gói này
// giữ phần dùng chung: bản ghim, pipeline cài, phép đọc phiên bản, và môi
// trường UV_* trỏ hết vào thư mục dữ liệu của app nên không gì rơi vào
// ~/.local hay cache của người dùng.
package toolchain

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

// UvToolID là tên thư mục công cụ dưới <data>/bin.
const UvToolID = "uv"

var uvVersionRe = regexp.MustCompile(`uv\s+(\d+\.\d+\.\d+)`)

// ParseUvVersion đọc `uv X.Y.Z`.
func ParseUvVersion(stdout string) string {
	if m := uvVersionRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	return ""
}

// UvBinaryName là tên tệp thực thi của uv trên nền tảng đang chạy.
func UvBinaryName() string {
	if runtime.GOOS == "windows" {
		return "uv.exe"
	}
	return "uv"
}

// UvPath trả binary uv trong bản cài app quản; "" khi chưa cài.
func UvPath(baseDir string) string {
	dir := ActiveVersionDir(baseDir, UvToolID)
	if dir == "" {
		return ""
	}
	exe := filepath.Join(dir, UvBinaryName())
	if fileExists(exe) {
		return exe
	}
	return ""
}

// ProbeUv chạy `uv --version` của một binary bất kỳ; ok=false khi binary
// không chạy hoặc banner không đọc được.
func ProbeUv(ctx context.Context, exe string) (version string, ok bool) {
	if !fileExists(exe) {
		return "", false
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, exe, "--version").Output()
	if err != nil {
		return "", false
	}
	version = ParseUvVersion(string(out))
	return version, version != ""
}

// UvProber chạy thử một binary uv vừa bung; app có runner riêng thì truyền
// vào, nil nghĩa là dùng ProbeUv.
type UvProber func(ctx context.Context, exe string) (version string, ok bool)

// UvInstallSpec trả pipeline cài bản uv ghim vào bin/uv.
func UvInstallSpec(probe UvProber) InstallSpec {
	if probe == nil {
		probe = ProbeUv
	}
	return InstallSpec{
		ToolID: UvToolID,
		Pin:    UvPin(),
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			exe := filepath.Join(staging, UvBinaryName())
			version, ok := probe(ctx, exe)
			if !ok {
				return "", fmt.Errorf("uv tải về không chạy được")
			}
			if version != UvPin().Version {
				return "", fmt.Errorf("uv tự kiểm lệch phiên bản: %s != %s", version, UvPin().Version)
			}
			return version, nil
		},
	}
}

// UvEnv là môi trường app-owned để uv không đụng home hay cache của người
// dùng. Mọi app truyền thư mục dữ liệu của mình, nên hai app trên cùng máy
// không chia sẻ Python hay cache.
func UvEnv(root string) map[string]string {
	return map[string]string{
		"UV_INSTALL_DIR":        filepath.Join(root, "bin", "uv-tools"),
		"UV_PYTHON_INSTALL_DIR": filepath.Join(root, "python"),
		"UV_CACHE_DIR":          filepath.Join(root, "cache", "uv"),
		"UV_TOOL_DIR":           filepath.Join(root, "bin", "uv-tools"),
		"UV_NO_CONFIG":          "1",
	}
}
