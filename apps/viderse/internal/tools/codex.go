// Quản lý Codex CLI. Việc phát hiện chạy binary thật — wrapper npm có thể
// còn trên PATH khi binary native đi kèm đã mất, nên kiểu `which` không có
// nghĩa. Status tách missing / broken / outdated / logged_out /
// isolation_unverified / ready.
//
// Bản cài app-owned nằm dưới <data>/bin/codex gồm cả `codex` và
// `codex-code-mode-host` (exec dừng khi thiếu binary host cạnh bên). App
// không bao giờ xoá hay cài lại gói npm toàn cục của người dùng; sửa nghĩa
// là binary app-owned trở thành bản được chọn.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const codexToolID = "codex"

// CodexSettingKey là key PathStore của codex.
const CodexSettingKey = "tools:codex"

// codexLoginStateFile là tệp ghi phiên `codex login` đang chạy.
const codexLoginStateFile = "codex-login.json"

// codexLoginMatch là mảnh command-line của nhóm `codex login` — bằng chứng
// danh tính trước khi signal.
const codexLoginMatch = " login"

// CodexState là trạng thái tổng hợp của codex.
type CodexState string

const (
	CodexMissing             CodexState = "missing"
	CodexBroken              CodexState = "broken"
	CodexOutdated            CodexState = "outdated"
	CodexLoggedOut           CodexState = "logged_out"
	CodexIsolationUnverified CodexState = "isolation_unverified"
	CodexReady               CodexState = "ready"
)

// CodexLogin là kiểu đăng nhập Codex báo về.
type CodexLogin string

const (
	CodexLoginChatGPT CodexLogin = "chatgpt"
	CodexLoginAPIKey  CodexLogin = "api_key"
	CodexLoginNone    CodexLogin = "not_logged_in"
	CodexLoginUnknown CodexLogin = "unknown"
)

// CodexBinaryInfo mô tả binary codex đã probe được.
type CodexBinaryInfo struct {
	Path    string
	Version string
	Source  ToolSource
}

// CodexBrokenProbe là ứng viên phát hiện nhưng không dùng được.
type CodexBrokenProbe struct {
	Path  string
	Error string
}

// CodexStatus là trạng thái đầy đủ cho Cài đặt.
type CodexStatus struct {
	State      CodexState
	Binary     *CodexBinaryInfo
	Broken     []CodexBrokenProbe
	Detected   []CodexDetected // các bản hệ thống chạy được ngoài bản đã chọn
	Login      CodexLogin
	Isolation  *CodexIsolation
	MinVersion string
}

// CodexDetected là một bản hệ thống chạy được ngoài bản đã chọn.
type CodexDetected struct {
	Path    string
	Version string
}

// Codex quản lý binary codex, phiên login và preflight cô lập.
type Codex struct {
	d                 Deps
	verifiedIsolation *FileVerificationCache[CodexIsolation]

	// mu tuần tự hoá mọi thao tác phiên login: hai StartLogin đồng thời chỉ
	// sinh một spawn, CancelLogin giữ khoá suốt quá trình dừng nhóm.
	mu    sync.Mutex
	spawn func(file string, args []string, env map[string]string) (int, func() error, error) // seam test; nil → spawnDetached
}

var (
	codexVersionRe = regexp.MustCompile(`codex-cli\s+(\d+\.\d+\.\d+)`)
	codexAnyVerRe  = regexp.MustCompile(`(\d+\.\d+\.\d+)`)
)

// ParseCodexVersion đọc banner `codex-cli X.Y.Z`.
func ParseCodexVersion(stdout string) string {
	if m := codexVersionRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	if m := codexAnyVerRe.FindStringSubmatch(stdout); m != nil {
		return m[1]
	}
	return ""
}

// CompareVersions so sánh kiểu semver trên phiên bản chấm số: -1 | 0 | 1.
func CompareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	var out []int
	for _, p := range strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' }) {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		out = append(out, n)
	}
	return out
}

// describeSpawnError phân biệt binary mất và wrapper npm còn vỏ.
func describeSpawnError(exe string, err error) string {
	var pfe *ProcessFailedError
	if errors.As(err, &pfe) && pfe.Err != nil {
		if errors.Is(pfe.Err, os.ErrNotExist) {
			return fmt.Sprintf("binary thiếu hoặc launcher hỏng: %s", exe)
		}
		detail := strings.TrimSpace(pfe.Stderr + " " + pfe.Err.Error())
		if regexp.MustCompile(`(?i)ENOENT|No such file|not found`).MatchString(detail) {
			return fmt.Sprintf("wrapper còn nhưng binary đi kèm đã mất (%s)", exe)
		}
	}
	detail := err.Error()
	if i := strings.LastIndex(detail, "\n"); i >= 0 {
		detail = detail[i+1:]
	}
	return detail
}

func (c *Codex) probe(ctx context.Context, exe string, source ToolSource) (*CodexBinaryInfo, *CodexBrokenProbe) {
	if !fileExists(exe) {
		return nil, &CodexBrokenProbe{Path: exe, Error: "binary không tồn tại ở đường dẫn này"}
	}
	res, err := c.d.Runner.Run(ctx, exe, []string{"--version"}, Spec{Timeout: 30 * time.Second})
	if err != nil {
		return nil, &CodexBrokenProbe{Path: exe, Error: describeSpawnError(exe, err)}
	}
	version := ParseCodexVersion(res.Stdout)
	if version == "" {
		out := strings.TrimSpace(res.Stdout)
		if len(out) > 80 {
			out = out[:80]
		}
		return nil, &CodexBrokenProbe{Path: exe, Error: "phiên bản không đọc được: " + out}
	}
	return &CodexBinaryInfo{Path: exe, Version: version, Source: source}, nil
}

// ManagedDir trả thư mục phiên bản đang dùng của bản cài app quản.
func (c *Codex) ManagedDir() string {
	return ActiveVersionDir(c.d.BaseDir, codexToolID)
}

// ManagedPath trả binary codex app quản; "" khi chưa cài.
func (c *Codex) ManagedPath() string {
	dir := c.ManagedDir()
	if dir == "" {
		return ""
	}
	name := "codex"
	if runtime.GOOS == "windows" {
		name = "codex.exe"
	}
	exe := filepath.Join(dir, name)
	if fileExists(exe) {
		return exe
	}
	return ""
}

// DefaultHome là kho login mặc định của Codex — auth.json nằm đây.
func (c *Codex) DefaultHome() string {
	return filepath.Join(c.d.home(), ".codex")
}

// Candidates liệt kê codex hệ thống đáng thử.
func (c *Codex) Candidates() []string {
	var out []string
	if onPath := FindOnPath("codex", c.d.pathEnv()); onPath != "" {
		out = append(out, onPath)
	}
	for _, dir := range c.d.standardDirs() {
		out = append(out, filepath.Join(dir, "codex"))
	}
	return dedupeStrings(out)
}

// ConfiguredPath trả codex do người dùng chọn; "" khi không có.
func (c *Codex) ConfiguredPath() string { return c.d.configuredPath(CodexSettingKey) }

// SetConfiguredPath ghi/xoá codex do người dùng chọn.
func (c *Codex) SetConfiguredPath(exe string) error {
	return c.d.SetConfiguredPath(CodexSettingKey, exe)
}

// Resolve trả binary codex đầu tiên probe được theo thứ tự cấu hình → hệ
// thống → app quản; "" khi không có.
func (c *Codex) Resolve(ctx context.Context) string {
	for _, cand := range resolutionOrder(c.ConfiguredPath(), c.Candidates(), c.ManagedPath()) {
		if info, _ := c.probe(ctx, cand.path, cand.source); info != nil {
			return cand.path
		}
	}
	return ""
}

// LoginStatus chạy `codex login status` với home cho trước; không bao giờ
// in credential.
func (c *Codex) LoginStatus(ctx context.Context, exe, codexHome string) CodexLogin {
	res, err := c.d.Runner.Run(ctx, exe, []string{"login", "status"}, Spec{
		Timeout: 30 * time.Second,
		Env:     map[string]string{"CODEX_HOME": codexHome},
	})
	if err != nil {
		return CodexLoginUnknown
	}
	// Một số phiên bản in kết quả qua stderr — đọc cả hai.
	out := strings.ToLower(res.Stdout + "\n" + res.Stderr)
	switch {
	case strings.Contains(out, "chatgpt"):
		return CodexLoginChatGPT
	case strings.Contains(out, "api key"), strings.Contains(out, "api-key"), strings.Contains(out, "apikey"):
		return CodexLoginAPIKey
	case strings.Contains(out, "not logged in"), strings.Contains(out, "logged out"):
		return CodexLoginNone
	}
	return CodexLoginUnknown
}

// Status đầy đủ: từng ứng viên đều được probe bằng cách chạy nó; khi có
// binary dùng được thì login + preflight cô lập chạy trên chính binary đó.
func (c *Codex) Status(ctx context.Context) CodexStatus {
	var broken []CodexBrokenProbe
	var detected []CodexDetected
	var binary *CodexBinaryInfo

	for _, cand := range resolutionOrder(c.ConfiguredPath(), c.Candidates(), c.ManagedPath()) {
		info, bad := c.probe(ctx, cand.path, cand.source)
		if bad != nil {
			broken = append(broken, *bad)
			continue
		}
		if binary == nil {
			binary = info
		} else if cand.source == SourceSystem {
			detected = append(detected, CodexDetected{Path: info.Path, Version: info.Version})
		}
	}

	base := CodexStatus{
		Binary:     binary,
		Broken:     broken,
		Detected:   detected,
		MinVersion: CodexMinVersion,
		Login:      CodexLoginUnknown,
	}
	if binary == nil {
		base.State = CodexMissing
		if len(broken) > 0 {
			base.State = CodexBroken
		}
		return base
	}
	if CompareVersions(binary.Version, CodexMinVersion) < 0 {
		base.State = CodexOutdated
		return base
	}
	login := c.LoginStatus(ctx, binary.Path, c.DefaultHome())
	base.Login = login
	if login == CodexLoginNone || login == CodexLoginAPIKey {
		base.State = CodexLoggedOut
		return base
	}
	isolation := c.Isolation(ctx, binary.Path)
	base.Isolation = &isolation
	if isolation.State != IsolationVerified {
		base.State = CodexIsolationUnverified
		return base
	}
	base.State = CodexReady
	return base
}

// Isolation trả preflight cô lập của một binary; cache theo identity tệp vì
// hợp đồng cô lập là tính chất của binary — chỉ kiểm lại khi nó bị thay.
func (c *Codex) Isolation(ctx context.Context, exe string) CodexIsolation {
	verify := func(ctx context.Context) (CodexIsolation, error) {
		return c.IsolationPreflight(ctx, exe)
	}
	if c.verifiedIsolation != nil {
		is, err := c.verifiedIsolation.Check(exe, verify, ctx)
		if err == nil {
			return is
		}
	}
	is, err := verify(ctx)
	if err != nil {
		return CodexIsolation{State: IsolationUnverified,
			Checks: []CodexIsolationCheck{{Name: "preflight", OK: false, Detail: err.Error()}}}
	}
	return is
}

// InstallSpec trả pipeline cài cặp codex (CLI + code-mode host) ghim.
func (c *Codex) InstallSpec() InstallSpec {
	return InstallSpec{
		ToolID: codexToolID,
		Pin:    CodexPin(),
		SelfCheck: func(ctx context.Context, ic *InstallContext, staging string) (string, error) {
			name := "codex"
			host := "codex-code-mode-host"
			if runtime.GOOS == "windows" {
				name, host = "codex.exe", "codex-code-mode-host.exe"
			}
			exe := filepath.Join(staging, name)
			info, bad := c.probe(ctx, exe, SourceManaged)
			if bad != nil {
				return "", fmt.Errorf("codex tải về không chạy được: %s", bad.Error)
			}
			if CompareVersions(info.Version, CodexMinVersion) < 0 {
				return "", fmt.Errorf("codex tải về %s cũ hơn yêu cầu %s", info.Version, CodexMinVersion)
			}
			if !fileExists(filepath.Join(staging, host)) {
				return "", fmt.Errorf("codex-code-mode-host thiếu trong bản cài")
			}
			return info.Version, nil
		},
	}
}

// CodexLoginSession là một phiên `codex login` đang chạy.
type CodexLoginSession struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

func (c *Codex) loginStatePath() string {
	return filepath.Join(c.d.BaseDir, "cache", codexLoginStateFile)
}

// LoginSession trả phiên login còn sống, tự xoá bản ghi khi pid đã chết.
func (c *Codex) LoginSession() *CodexLoginSession {
	data, err := os.ReadFile(c.loginStatePath())
	if err != nil {
		return nil
	}
	var s CodexLoginSession
	if err := json.Unmarshal(data, &s); err != nil || s.PID <= 0 {
		return nil
	}
	if !pidAlive(s.PID) {
		_ = os.Remove(c.loginStatePath())
		return nil
	}
	return &s
}

func (c *Codex) writeLoginSession(s *CodexLoginSession) {
	if s == nil {
		_ = os.Remove(c.loginStatePath())
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.loginStatePath()), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.loginStatePath(), data, 0o600)
}

// StartLogin mở `codex login` (flow trình duyệt/thiết bị do CLI sở hữu).
// Chỉ chạy khi người dùng chủ động bấm; phiên trả về theo dõi qua
// LoginSession và dừng qua CancelLogin.
func (c *Codex) StartLogin(ctx context.Context) (*CodexLoginSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.LoginSession(); existing != nil {
		return existing, nil
	}
	exe := c.Resolve(ctx)
	if exe == "" {
		return nil, fmt.Errorf("Codex chưa được cài — cài hoặc sửa Codex trước")
	}
	spawn := c.spawn
	if spawn == nil {
		spawn = spawnDetached
	}
	pid, wait, err := spawn(exe, []string{"login"}, map[string]string{
		"CODEX_HOME": c.DefaultHome(),
	})
	if err != nil {
		return nil, err
	}
	s := &CodexLoginSession{PID: pid, StartedAt: time.Now().UTC()}
	c.writeLoginSession(s)
	if er, ok := c.d.Runner.(*ExecRunner); ok && er.Registry != nil {
		er.Registry.Track(pid, codexLoginMatch, "codex-login")
	}
	go func() {
		_ = wait()
		// Chỉ xoá bản ghi nếu nó vẫn là phiên này — một phiên login mới ghi
		// đè sau không bị goroutine cũ xoá nhầm.
		if cur := c.loginSessionFile(); cur != nil && cur.PID == pid {
			c.writeLoginSession(nil)
		}
	}()
	return s, nil
}

// loginSessionFile đọc bản ghi phiên mà không kiểm tra sống/chết của pid.
func (c *Codex) loginSessionFile() *CodexLoginSession {
	data, err := os.ReadFile(c.loginStatePath())
	if err != nil {
		return nil
	}
	var s CodexLoginSession
	if err := json.Unmarshal(data, &s); err != nil || s.PID <= 0 {
		return nil
	}
	return &s
}

// CancelLogin dừng flow login đang chạy: SIGTERM rồi SIGKILL sau ân hạn —
// chỉ khi pid đã ghi vẫn dẫn một nhóm `codex login`.
func (c *Codex) CancelLogin(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.LoginSession()
	if s == nil {
		return nil
	}
	// Bản ghi xoá sau khi nhóm đã dứt: xoá trước có thể để lại nhóm login
	// không ai sở hữu khi TerminateGroup thất bại.
	_, err := TerminateGroup(ctx, GroupTarget{PGID: s.PID, Identity: []string{codexLoginMatch}}, TerminateOptions{})
	c.writeLoginSession(nil)
	return err
}
