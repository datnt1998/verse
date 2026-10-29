// uv (Astral) quản lý Python riêng của app + venv ghim (faster-whisper).
// Thứ tự chọn: đường dẫn người dùng → binary hệ thống → bản app quản. Mọi
// lời gọi uv chạy với UV_* trỏ vào thư mục app nên không gì rơi vào ~/.local
// hay cache của người dùng.
package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const uvToolID = "uv"

// UvSettingKey là key PathStore của uv.
const UvSettingKey = "tools:uv"

// UvPython là phiên bản Python do app quản.
const UvPython = "3.11"

// Uv quản lý binary uv và môi trường Python ghim.
type Uv struct{ d Deps }

// UvState là trạng thái uv.
type UvState string

const (
	UvReady   UvState = "ready"
	UvMissing UvState = "missing"
	UvBroken  UvState = "broken"
)

// UvDetected là một bản uv hệ thống chạy được.
type UvDetected struct {
	Path    string
	Version string
}

// UvStatus tóm tắt trạng thái uv cho Cài đặt.
type UvStatus struct {
	State    UvState
	Source   ToolSource
	Path     string
	Version  string
	Detected []UvDetected
	Error    string
}

var uvVersionRe = regexp.MustCompile(`uv\s+(\d+\.\d+\.\d+)`)

// ParseUvVersion đọc `uv X.Y.Z`.
func ParseUvVersion(stdout string) string {
	if m := uvVersionRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	return ""
}

// Probe chạy `uv --version`; nil khi binary không chạy hoặc không đọc được.
func (u *Uv) Probe(ctx context.Context, exe string) *UvDetected {
	if !fileExists(exe) {
		return nil
	}
	res, err := u.d.Runner.Run(ctx, exe, []string{"--version"}, Spec{Timeout: 15 * time.Second})
	if err != nil {
		return nil
	}
	version := ParseUvVersion(res.Stdout)
	if version == "" {
		return nil
	}
	return &UvDetected{Path: exe, Version: version}
}

// Candidates liệt kê uv hệ thống đáng thử.
func (u *Uv) Candidates() []string {
	var out []string
	if onPath := FindOnPath("uv", u.d.pathEnv()); onPath != "" {
		out = append(out, onPath)
	}
	for _, dir := range u.d.standardDirs() {
		out = append(out, filepath.Join(dir, "uv"))
	}
	if runtime.GOOS != "windows" {
		out = append(out, filepath.Join(u.d.home(), ".local", "bin", "uv"))
	}
	return dedupeStrings(out)
}

// ConfiguredPath trả uv do người dùng chọn; "" khi không có.
func (u *Uv) ConfiguredPath() string { return u.d.configuredPath(UvSettingKey) }

// SetConfiguredPath ghi/xoá uv do người dùng chọn.
func (u *Uv) SetConfiguredPath(exe string) error {
	return u.d.SetConfiguredPath(UvSettingKey, exe)
}

// ManagedPath trả uv trong bản cài app quản; "" khi chưa cài.
func (u *Uv) ManagedPath() string {
	dir := ActiveVersionDir(u.d.BaseDir, uvToolID)
	if dir == "" {
		return ""
	}
	name := "uv"
	if runtime.GOOS == "windows" {
		name = "uv.exe"
	}
	exe := filepath.Join(dir, name)
	if fileExists(exe) {
		return exe
	}
	return ""
}

// Resolve trả binary uv pipeline nên chạy, theo thứ tự chọn; "" khi không có.
func (u *Uv) Resolve(ctx context.Context) string {
	for _, c := range resolutionOrder(u.ConfiguredPath(), u.Candidates(), u.ManagedPath()) {
		if u.Probe(ctx, c.path) != nil {
			return c.path
		}
	}
	return ""
}

// Status tổng hợp cho Cài đặt.
func (u *Uv) Status(ctx context.Context) UvStatus {
	var detected []UvDetected
	for _, c := range resolutionOrder(u.ConfiguredPath(), u.Candidates(), u.ManagedPath()) {
		probe := u.Probe(ctx, c.path)
		if probe == nil {
			if c.source == SourceConfigured {
				return UvStatus{State: UvBroken, Source: c.source, Path: c.path,
					Error: "Không chạy được uv ở đường dẫn đã chọn"}
			}
			continue
		}
		if c.source == SourceSystem {
			detected = append(detected, *probe)
		}
		return UvStatus{State: UvReady, Source: c.source, Path: probe.Path, Version: probe.Version, Detected: detected}
	}
	return UvStatus{State: UvMissing, Detected: detected}
}

// InstallSpec trả pipeline cài bản uv ghim vào bin/uv.
func (u *Uv) InstallSpec() InstallSpec {
	return InstallSpec{
		ToolID: uvToolID,
		Pin:    UvPin(),
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			name := "uv"
			if runtime.GOOS == "windows" {
				name = "uv.exe"
			}
			exe := filepath.Join(staging, name)
			probe := u.Probe(ctx, exe)
			if probe == nil {
				return "", fmt.Errorf("uv tải về không chạy được")
			}
			if probe.Version != UvPin().Version {
				return "", fmt.Errorf("uv tự kiểm lệch phiên bản: %s != %s", probe.Version, UvPin().Version)
			}
			return probe.Version, nil
		},
	}
}

// Env là môi trường app-owned để uv không đụng home hay cache của người dùng.
func (u *Uv) Env() map[string]string {
	root := u.d.BaseDir
	return map[string]string{
		"UV_INSTALL_DIR":        filepath.Join(root, "bin", "uv-tools"),
		"UV_PYTHON_INSTALL_DIR": filepath.Join(root, "python"),
		"UV_CACHE_DIR":          filepath.Join(root, "cache", "uv"),
		"UV_TOOL_DIR":           filepath.Join(root, "bin", "uv-tools"),
		"UV_NO_CONFIG":          "1",
	}
}

// PythonDir là nơi `uv python install` đặt interpreter của app.
func (u *Uv) PythonDir() string {
	return filepath.Join(u.d.BaseDir, "python", "cpython-"+UvPython)
}

// EnvFingerprint là dấu vân tay quyết định một env có phải dựng lại: bao gồm
// phiên bản uv, python, nền tảng và nội dung requirements.
func (u *Uv) EnvFingerprint(requirementsFile, extra string) (string, error) {
	content, err := os.ReadFile(requirementsFile)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "uv:%s\npython:%s\nplatform:%s/%s\n%s\n", UvPin().Version, UvPython, runtime.GOOS, runtime.GOARCH, extra)
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))[:24], nil
}

// PythonEnvInfo mô tả một venv app quản.
type PythonEnvInfo struct {
	EnvDir           string
	Python           string
	Fingerprint      string
	RequirementsFile string
}

// EnvInfo dựng mô tả của một env (kèm fingerprint hiện tại của requirements).
func (u *Uv) EnvInfo(envDir, requirementsFile string) (PythonEnvInfo, error) {
	fingerprint, err := u.EnvFingerprint(requirementsFile, "")
	if err != nil {
		return PythonEnvInfo{}, err
	}
	name := "python"
	if runtime.GOOS == "windows" {
		name = "python.exe"
	}
	return PythonEnvInfo{
		EnvDir:           envDir,
		Python:           filepath.Join(envDir, "bin", name),
		Fingerprint:      fingerprint,
		RequirementsFile: requirementsFile,
	}, nil
}

// EnvBuiltFingerprint trả fingerprint đã ghi của env đã dựng, hoặc "" khi
// vắng/chưa hoàn chỉnh/lệch fingerprint hiện tại.
func (u *Uv) EnvBuiltFingerprint(info PythonEnvInfo) string {
	data, err := os.ReadFile(filepath.Join(info.EnvDir, "env-meta.json"))
	if err != nil {
		return ""
	}
	var meta struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(data, &meta); err != nil || meta.Fingerprint != info.Fingerprint {
		return ""
	}
	if !fileExists(info.Python) {
		return ""
	}
	return meta.Fingerprint
}

// EnsurePythonEnv dựng venv ghim: `uv python install` vào thư mục app,
// `uv venv`, `uv pip sync` với requirements khóa hash. Ghi qua staging dir +
// rename nguyên tử nên huỷ không để lại nửa env đang dùng.
func (u *Uv) EnsurePythonEnv(ctx context.Context, uvBin, envDir, requirementsFile string, onProgress func(detail string)) (PythonEnvInfo, error) {
	info, err := u.EnvInfo(envDir, requirementsFile)
	if err != nil {
		return PythonEnvInfo{}, err
	}
	if u.EnvBuiltFingerprint(info) == info.Fingerprint {
		return info, nil
	}
	env := u.Env()
	staging := fmt.Sprintf("%s.build-%d", envDir, time.Now().UnixMilli())
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)

	if onProgress != nil {
		onProgress("python")
	}
	if _, err := u.d.Runner.Run(ctx, uvBin,
		[]string{"python", "install", UvPython, "--install-dir", u.PythonDir(), "--no-config"},
		Spec{Env: env, Timeout: 10 * time.Minute}); err != nil {
		return PythonEnvInfo{}, err
	}

	if onProgress != nil {
		onProgress("venv")
	}
	pythonExe, err := u.findInstalledPython(ctx, uvBin)
	if err != nil {
		return PythonEnvInfo{}, err
	}
	if _, err := u.d.Runner.Run(ctx, uvBin,
		[]string{"venv", "--python", pythonExe, "--seed", staging},
		Spec{Env: env, Timeout: 2 * time.Minute}); err != nil {
		return PythonEnvInfo{}, err
	}

	if onProgress != nil {
		onProgress("pip sync")
	}
	stagingPython := filepath.Join(staging, "bin", "python")
	if runtime.GOOS == "windows" {
		stagingPython = filepath.Join(staging, "Scripts", "python.exe")
	}
	if _, err := u.d.Runner.Run(ctx, uvBin,
		[]string{"pip", "sync", requirementsFile, "--python", stagingPython, "--no-config"},
		Spec{Env: env, Timeout: 45 * time.Minute}); err != nil {
		return PythonEnvInfo{}, err
	}

	meta, _ := json.Marshal(map[string]string{
		"fingerprint":      info.Fingerprint,
		"requirementsFile": filepath.Base(requirementsFile),
		"builtAt":          time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err := os.WriteFile(filepath.Join(staging, "env-meta.json"), meta, 0o644); err != nil {
		return PythonEnvInfo{}, err
	}
	backup := fmt.Sprintf("%s.old-%d", envDir, time.Now().UnixMilli())
	if fileExists(envDir) {
		if err := os.Rename(envDir, backup); err != nil {
			return PythonEnvInfo{}, err
		}
	}
	if err := os.Rename(staging, envDir); err != nil {
		if fileExists(backup) {
			_ = os.Rename(backup, envDir)
		}
		return PythonEnvInfo{}, err
	}
	_ = os.RemoveAll(backup)
	return info, nil
}

// findInstalledPython hỏi uv về interpreter vừa cài (`uv python find` không
// nhận --install-dir từ uv 0.13; biến môi trường giới hạn cùng thư mục).
func (u *Uv) findInstalledPython(ctx context.Context, uvBin string) (string, error) {
	env := u.Env()
	env["UV_PYTHON_INSTALL_DIR"] = u.PythonDir()
	res, err := u.d.Runner.Run(ctx, uvBin,
		[]string{"python", "find", UvPython, "--managed-python", "--no-config"},
		Spec{Env: env, Timeout: time.Minute})
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	exe := ""
	if len(lines) > 0 {
		exe = strings.TrimSpace(lines[len(lines)-1])
	}
	if exe == "" || !fileExists(exe) {
		return "", fmt.Errorf("uv python find không trả interpreter: %s", strings.TrimSpace(res.Stdout))
	}
	return exe, nil
}
