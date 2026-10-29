package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/webauth"
)

// G1：POST /api/reveal mở thư mục bằng trình quản lý tệp trên máy chủ;
// GET /api/reveal chỉ trả đường dẫn (để UI chép khi không mở được).
// G2：POST /api/system/pick-folder mở hộp chọn thư mục của hệ điều hành.
//
// Cả hai chỉ chạy khi request đến trực tiếp từ loopback (không qua proxy).

// isDirectLoopback 判断请求是否来自同机浏览器且没有经过反向代理转发。
func isDirectLoopback(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("Forwarded") != "" {
		return false
	}
	return isLoopbackRequest(r)
}

const revealLoopbackError = "Chỉ mở được thư mục khi dùng ngay trên máy chủ"

// revealTarget 是 14 §G1 允许的 target（audio 由音频工坊追加）。
var revealTargets = map[string]bool{
	"library": true, "project": true, "story": true,
	"exports": true, "activity": true, "rules": true, "audio": true, "config": true,
	// D12 — hai kho ghi chú Não truyện mở bằng Obsidian trên máy chủ.
	"nao-story": true, "nao-project": true,
}

type revealRequest struct {
	Project string `json:"project"`
	Story   string `json:"story"`
	Target  string `json:"target"`
	Scope   string `json:"scope"`
}

// revealPath 解析 target 对应的绝对路径；status 非 0 表示应直接返回该错误。
// 叶子目录（rules/exports/activity）不存在时会创建，保证“Mở thư mục”总能打开。
func (g *Gateway) revealPath(user webauth.User, req revealRequest) (path string, status int, msg string) {
	// Reveal/open-folder is a desktop file capability: a hosted library has
	// no directory tree to open, so every target answers 501 rather than
	// resolving a path inside the server cwd.
	if g.libRepo != nil {
		return "", http.StatusNotImplemented, "mở thư mục không hỗ trợ trên kho dữ liệu Postgres"
	}
	target := strings.TrimSpace(req.Target)
	if !revealTargets[target] {
		return "", http.StatusBadRequest, "target không hợp lệ: " + target
	}
	lib, err := g.lib(context.Background(), actorOf(user))
	globalRules := (target == "rules" || target == "config") && strings.TrimSpace(req.Scope) == "global"
	if errors.Is(err, errNoLibrary) && !globalRules {
		return "", http.StatusConflict, err.Error()
	}
	if err != nil && !errors.Is(err, errNoLibrary) {
		return "", http.StatusInternalServerError, err.Error()
	}
	project, story := strings.TrimSpace(req.Project), strings.TrimSpace(req.Story)
	if target == "library" {
		if !isAdmin(user) {
			return "", http.StatusForbidden, "Không có quyền"
		}
		return g.RootPath(), 0, ""
	}
	if target == "config" {
		return g.revealConfigFile(user, req)
	}
	if naoVaultTarget(target) {
		return g.revealNaoVault(user, lib, req)
	}
	if target == "rules" && strings.TrimSpace(req.Scope) == "global" {
		if !isAdmin(user) {
			return "", http.StatusForbidden, "Không có quyền"
		}
		dir := rulesHomeDir()
		if dir == "" {
			return "", http.StatusInternalServerError, "không xác định được thư mục home"
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", http.StatusInternalServerError, err.Error()
		}
		return dir, 0, ""
	}
	if project == "" {
		return "", http.StatusBadRequest, "thiếu tham số project"
	}
	if !user.CanReadProject(project) {
		return "", http.StatusNotFound, "không tìm thấy dự án"
	}
	pref, ok := lib.ResolveProject(context.Background(), actorOf(user), project)
	pdir := pref.Dir
	if !ok {
		return "", http.StatusNotFound, "không tìm thấy dự án"
	}
	// target=rules 且 scope 省略时，有 story 就用故事层，否则用项目层。
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "project"
		if story != "" {
			scope = "story"
		}
	}
	switch target {
	case "project":
		return pdir, 0, ""
	case "rules":
		switch scope {
		case "project":
			dir := rulesDirIn(pdir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", http.StatusInternalServerError, err.Error()
			}
			return dir, 0, ""
		case "story":
			if story == "" {
				return "", http.StatusBadRequest, "thiếu tham số story"
			}
		default:
			return "", http.StatusBadRequest, "scope không hợp lệ: " + scope
		}
	}
	if story == "" {
		return "", http.StatusBadRequest, "thiếu tham số story"
	}
	st, ok := lib.ResolveStory(context.Background(), actorOf(user), project, story)
	if !ok {
		return "", http.StatusNotFound, "không tìm thấy truyện"
	}
	if !user.CanRead(st.ID) {
		return "", http.StatusNotFound, "không tìm thấy truyện"
	}
	switch target {
	case "story":
		return st.Dir, 0, ""
	case "rules":
		dir := rulesDirIn(st.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", http.StatusInternalServerError, err.Error()
		}
		return dir, 0, ""
	case "exports":
		dir := filepath.Join(st.Dir, lib.Config().Dirs.Exports)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", http.StatusInternalServerError, err.Error()
		}
		return dir, 0, ""
	case "audio":
		dir := filepath.Join(st.Dir, lib.Config().Dirs.Audio)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", http.StatusInternalServerError, err.Error()
		}
		return dir, 0, ""
	case "activity":
		dir := filepath.Join(g.storyOutputDir(st), "meta")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", http.StatusInternalServerError, err.Error()
		}
		return dir, 0, ""
	}
	return "", http.StatusBadRequest, "target không hợp lệ: " + target
}

// responsePath keeps server filesystem locations private from non-administrators.
func (g *Gateway) responsePath(path string, admin bool) string {
	if admin || path == "" {
		return path
	}
	return publicExportPath(g.RootPath(), path)
}

// serveReveal 处理 /api/reveal；返回 false 表示路径不归它管。
func (g *Gateway) serveReveal(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	if r.URL.Path != "/api/reveal" {
		return false
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		path, status, msg := g.revealPath(user, revealRequest{
			Project: q.Get("project"), Story: q.Get("story"),
			Target: q.Get("target"), Scope: q.Get("scope"),
		})
		if status != 0 {
			jsonError(w, status, msg)
			return true
		}
		// Kho ghi chú Não truyện: không trả đường dẫn (D12 + invariant #7); GET chỉ cho
		// UI biết máy chủ có mở được bằng Obsidian ngay tại máy này hay không.
		if naoVaultTarget(strings.TrimSpace(q.Get("target"))) {
			writeJSON(w, http.StatusOK, map[string]any{"canReveal": isAdmin(user) && isDirectLoopback(r)})
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": g.responsePath(path, isAdmin(user)), "canReveal": isAdmin(user) && isDirectLoopback(r)})
	case http.MethodPost:
		if !isAdmin(user) {
			jsonError(w, http.StatusForbidden, "Không có quyền")
			return true
		}
		var req revealRequest
		if !decodeJSON(w, r, &req) {
			return true
		}
		path, status, msg := g.revealPath(user, req)
		if status != 0 {
			jsonError(w, status, msg)
			return true
		}
		if !isDirectLoopback(r) {
			jsonError(w, http.StatusForbidden, revealLoopbackError)
			return true
		}
		if err := g.openRevealed(strings.TrimSpace(req.Target), path); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return true
		}
		if naoVaultTarget(strings.TrimSpace(req.Target)) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
	return true
}

// revealConfigFile 解析 target=config：返回该层 config.json 的绝对路径（文件未必存在）。
// 权限与 rules 一致：global 仅 admin；project/story 需能读该对象。上级目录不存在时创建，
// 保证文件缺失时 POST 仍能打开其所在目录。
func (g *Gateway) revealConfigFile(user webauth.User, req revealRequest) (path string, status int, msg string) {
	project, story := strings.TrimSpace(req.Project), strings.TrimSpace(req.Story)
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "global"
		if story != "" {
			scope = "story"
		} else if project != "" {
			scope = "project"
		}
	}
	if scope == "global" && !isAdmin(user) {
		return "", http.StatusForbidden, "Không có quyền"
	}
	if scope == "project" || scope == "story" {
		if project != "" && !user.CanReadProject(project) {
			return "", http.StatusNotFound, "không tìm thấy dự án"
		}
	}
	t, err := g.resolveConfigScope(scope, project, story)
	switch {
	case errors.Is(err, errNoLibrary):
		return "", http.StatusConflict, err.Error()
	case errors.Is(err, library.ErrNotFound):
		return "", http.StatusNotFound, "không tìm thấy"
	case err != nil:
		return "", http.StatusBadRequest, err.Error()
	}
	if scope == "story" && !user.CanRead(t.story.ID) {
		return "", http.StatusNotFound, "không tìm thấy truyện"
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return "", http.StatusInternalServerError, err.Error()
	}
	return t.path, 0, ""
}

// naoVaultTarget: hai đích D12 — kho ghi chú Não truyện/của dự án, mở bằng Obsidian.
func naoVaultTarget(target string) bool {
	return target == "nao-story" || target == "nao-project"
}

// revealNaoVault phân giải thư mục kho ghi chú để mở bằng Obsidian (D12, nhóm Nâng cao).
// Chỉ quản trị. Kho là bản sinh một chiều từ dữ kiện gốc (ADR 0001) nên ở đây không
// tự tạo: chưa có kho thì trả 404 để UI báo "chưa có", không mở một thư mục rỗng.
func (g *Gateway) revealNaoVault(user webauth.User, lib library.Repository, req revealRequest) (path string, status int, msg string) {
	if !isAdmin(user) {
		return "", http.StatusForbidden, "Không có quyền"
	}
	project := strings.TrimSpace(req.Project)
	if project == "" {
		return "", http.StatusBadRequest, "thiếu tham số project"
	}
	pref, ok := lib.ResolveProject(context.Background(), actorOf(user), project)
	pdir := pref.Dir
	if !ok || pdir == "" {
		return "", http.StatusNotFound, "không tìm thấy dự án"
	}
	if strings.TrimSpace(req.Target) == "nao-project" {
		return naoVaultDir(filepath.Join(pdir, "nao"), "Chưa có kho ghi chú của dự án")
	}
	story := strings.TrimSpace(req.Story)
	if story == "" {
		return "", http.StatusBadRequest, "thiếu tham số story"
	}
	st, ok := lib.ResolveStory(context.Background(), actorOf(user), project, story)
	if !ok {
		return "", http.StatusNotFound, "không tìm thấy truyện"
	}
	return naoVaultDir(filepath.Join(g.storyOutputDir(st), "nao"), "Chưa có kho ghi chú của truyện")
}

// naoVaultDir trả đường dẫn tuyệt đối của kho; chưa có thì 404 kèm câu không lộ đường dẫn.
func naoVaultDir(dir, missing string) (string, int, string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", http.StatusInternalServerError, err.Error()
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return "", http.StatusNotFound, missing
	}
	return abs, 0, ""
}

// obsidianURL là liên kết mở một thư mục thành vault trên máy chủ (Obsidian hiểu
// `path` là đường dẫn tuyệt đối đã mã hoá). Dấu cách mã hoá thành %20 để Obsidian
// không đọc thành dấu cộng.
func obsidianURL(dir string) string {
	return "obsidian://open?path=" + strings.ReplaceAll(url.QueryEscape(dir), "+", "%20")
}

// openURL 供测试替换：用系统默认程序打开一个 liên kết（macOS `open`、
// Windows `rundll32`、Linux `xdg-open`）。
var openURL = func(raw string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		cmd = exec.Command("xdg-open", raw)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("không mở được kho ghi chú: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openRevealed mở đích đã phân giải: thư mục → trình quản lý tệp; config.json → chọn đúng tệp
// (chưa có tệp thì mở thư mục chứa nó); kho ghi chú Não truyện → Obsidian.
func (g *Gateway) openRevealed(target, path string) error {
	switch target {
	case "config":
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return revealFile(path)
		}
		return openFolder(filepath.Dir(path))
	case "nao-story", "nao-project":
		return openURL(obsidianURL(path))
	}
	return openFolder(path)
}

// revealFile 供测试替换：在文件管理器中选中该文件（macOS `open -R`、Windows `explorer /select,`；
// Linux 无统一的“选中”，退化为用 xdg-open 打开所在目录）。
var revealFile = func(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,"+path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("không mở được thư mục: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openFolder 供测试替换：用系统文件管理器打开目录。
var openFolder = func(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("không mở được thư mục: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// pickFolder 供测试替换：返回 (path, cancelled, error)。
// 取消由用户的“Cancel”触发，属于正常结果而不是错误。
var pickFolder = func(start string) (string, bool, error) {
	switch runtime.GOOS {
	case "darwin":
		script := "POSIX path of (choose folder with prompt \"Chọn thư mục\""
		if start != "" {
			script += " default location (POSIX file " + appleScriptString(start) + ")"
		}
		script += ")"
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			if isPickerCancel(err) {
				return "", true, nil
			}
			return "", false, fmt.Errorf("không chọn được thư mục: %w", err)
		}
		return strings.TrimSpace(string(out)), false, nil
	case "linux":
		if path, ok, err := pickWithZenity(start); ok {
			return path, false, err
		}
		if path, ok, err := pickWithKdialog(start); ok {
			return path, false, err
		}
		return "", false, errPickUnsupported
	default:
		return "", false, errPickUnsupported
	}
}

// errPickUnsupported 表示平台没有可用的 thư mục选择器（映射为 501）。
var errPickUnsupported = errors.New("Máy chủ không hỗ trợ chọn thư mục")

func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// isPickerCancel 判断选择器进程是否因用户取消而退出（osascript -128、zenity 1）。
func isPickerCancel(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	if exit.ExitCode() == 1 {
		return true
	}
	return strings.Contains(string(exit.Stderr), "-128") || strings.Contains(string(exit.Stderr), "canceled")
}

func pickWithZenity(start string) (string, bool, error) {
	bin, err := exec.LookPath("zenity")
	if err != nil {
		return "", false, nil
	}
	args := []string{"--file-selection", "--directory", "--title=Chọn thư mục"}
	if start != "" {
		args = append(args, "--filename="+strings.TrimRight(start, string(filepath.Separator))+string(filepath.Separator))
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		if isPickerCancel(err) {
			return "", true, errPickerCancelled
		}
		return "", true, fmt.Errorf("không chọn được thư mục: %w", err)
	}
	return strings.TrimSpace(string(out)), true, nil
}

func pickWithKdialog(start string) (string, bool, error) {
	bin, err := exec.LookPath("kdialog")
	if err != nil {
		return "", false, nil
	}
	args := []string{"--getexistingdirectory"}
	if start != "" {
		args = append(args, start)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		if isPickerCancel(err) {
			return "", true, errPickerCancelled
		}
		return "", true, fmt.Errorf("không chọn được thư mục: %w", err)
	}
	return strings.TrimSpace(string(out)), true, nil
}

// errPickerCancelled 让 linux 分支把“用户取消”与真正的失败区分开。
var errPickerCancelled = errors.New("đã huỷ chọn thư mục")

// servePickFolder 处理 POST /api/system/pick-folder.
func (g *Gateway) servePickFolder(w http.ResponseWriter, r *http.Request, user webauth.User) {
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	var req struct {
		Start string `json:"start"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !isDirectLoopback(r) {
		jsonError(w, http.StatusForbidden, "Chỉ chọn được thư mục khi dùng ngay trên máy chủ")
		return
	}
	path, cancelled, err := pickFolder(strings.TrimSpace(req.Start))
	if errors.Is(err, errPickUnsupported) {
		jsonError(w, http.StatusNotImplemented, err.Error())
		return
	}
	if errors.Is(err, errPickerCancelled) {
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cancelled || path == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path})
}
