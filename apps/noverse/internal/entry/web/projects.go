package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/identity"
)

// §3 的 API 类型（camelCase，时间用 RFC3339，空时间为 null）。

type projectSummary struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	UniverseName string     `json:"universeName"` // "" = dùng tên dự án (GAP-44, câu 3)
	Description  string     `json:"description"`
	Legacy       bool       `json:"legacy"`
	StoryCount   int        `json:"storyCount"`
	RunningCount int        `json:"runningCount"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

type projectDetail struct {
	projectSummary
	Notes     string     `json:"notes"`
	CreatedAt *time.Time `json:"createdAt"`
	Path      string     `json:"path"`
}

type storySummary struct {
	ID                string     `json:"id"`
	Project           string     `json:"project"`
	Story             string     `json:"story"`
	Folder            string     `json:"folder"`
	Title             string     `json:"title"`
	ShortTitle        string     `json:"shortTitle,omitempty"` // Gói AF: {Tên ngắn} ≤32
	Status            string     `json:"status"`
	Tags              []string   `json:"tags"`
	Idea              string     `json:"idea"`
	Phase             string     `json:"phase"`
	CompletedChapters int        `json:"completedChapters"`
	TotalChapters     int        `json:"totalChapters"`
	TotalWords        int        `json:"totalWords"`
	OpenErrors        int        `json:"openErrors"`
	Volumes           int        `json:"volumes"`
	Open              bool       `json:"open"`
	Running           bool       `json:"running"`
	Locked            bool       `json:"locked"`
	UpdatedAt         *time.Time `json:"updatedAt"`
	ExportsCount      int        `json:"exportsCount"`
	TempName          bool       `json:"tempName"`
}

var tempNamePattern = regexp.MustCompile(`^\d+-truyen-moi$`)

// storyTitleMu nối tiếp PUT /meta với đồng bộ tự động story.json (watcher) —
// cả hai đều load → sửa → ghi nguyên tệp story.json, phải xếp hàng để khỏi mất field.
var storyTitleMu sync.Mutex

// displayTitle returns the effective story title: tên người dùng đổi tay
// (TitleRenamed) > tên sách engine đặt trong meta/book.json > story.json.title.
// Bản ghi cũ không có titleRenamed → tiêu đề lấy từ yêu cầu lúc tạo bị thay bởi
// tên sách (máy lỗi trước đây cắt dòng đầu brief làm title).
func displayTitle(sf *library.StoryFile, engineTitle string) string {
	engine := strings.TrimSpace(engineTitle)
	if sf != nil && sf.TitleRenamed && strings.TrimSpace(sf.Title) != "" {
		return sf.Title
	}
	if engine != "" {
		return engine
	}
	if sf != nil {
		return sf.Title
	}
	return ""
}

// engineBookTitle đọc tên sách engine đặt (meta/book.json) qua thư mục output cấu hình.
func (g *Gateway) engineBookTitle(st library.Story, user *identity.User) string {
	if engine, err := g.openFactStore(st, user); err == nil {
		if book, err := engine.Book.Load(); err == nil && book != nil {
			return book.Title
		}
	}
	return ""
}

// syncStoryTitle đồng bộ story.json theo tên sách engine đặt (meta/book.json):
// chỉ ghi khi người dùng chưa đổi tên tay (titleRenamed) và tiêu đề khác. Tên thư mục
// không đổi — thư mục là id ổn định. Gọi ở mọi đường đọc story.json để truyện cũ được vá lười.
func (g *Gateway) syncStoryTitle(ctx context.Context, lib library.Repository, st library.Story, engineTitle string, actor *library.Actor) {
	if engineTitle == "" {
		return
	}
	storyTitleMu.Lock()
	defer storyTitleMu.Unlock()
	sf, err := lib.LoadStoryFile(ctx, actor, st)
	if err != nil || sf == nil || sf.TitleRenamed || sf.Title == engineTitle {
		return
	}
	sf.Title = engineTitle
	if err := lib.WriteStoryFile(ctx, actor, st, *sf); err != nil {
		slog.Warn("đồng bộ tên truyện thất bại", "module", "web", "story_slug", st.ID, "err", err)
	}
}

// lib mở repository thư viện cho một request: file backend theo --root, hoặc
// repository hosted được serve cài qua SetLibraryRepository (STORE_DRIVER=
// postgres). actor là danh tính request — nil chỉ dành cho đường nội bộ đã
// phân quyền; hosted repository tự kiểm membership trong transaction.
func (g *Gateway) lib(ctx context.Context, actor *library.Actor) (library.Repository, error) {
	if g.libRepo != nil {
		return g.libRepo, nil
	}
	root := g.RootPath()
	if root == "" {
		return nil, errNoLibrary
	}
	lib, err := library.Open(root)
	if err != nil {
		return nil, err
	}
	return library.FileRepository(lib), nil
}

// actorOf ánh xạ tài khoản request thành actor bất biến cho tầng repository —
// repository kiểm lại membership trong tx, không tin check ở handler.
func actorOf(user identity.User) *library.Actor {
	return &library.Actor{Username: user.Username, GlobalRole: string(user.Role)}
}

// errNoLibrary: chưa chọn nơi lưu truyện (cài mới). filepath.Abs("") là thư mục hiện tại nên
// mọi đường vào thư viện phải đi qua lib() hoặc kiểm tra RootPath() trước.
var errNoLibrary = errors.New("Chưa chọn nơi lưu truyện")

// libraryConfigPath là library.json của thư viện hiện tại; "" khi chưa chọn nơi lưu.
func (g *Gateway) libraryConfigPath() string {
	if root := g.RootPath(); root != "" {
		return library.ConfigPath(root)
	}
	return ""
}

// libraryUnconfigured trả 409 cho các đường phụ thuộc thư viện khi chưa chọn nơi lưu; true = đã trả lời.
func (g *Gateway) libraryUnconfigured(w http.ResponseWriter, r *http.Request) bool {
	// Hosted (STORE_DRIVER=postgres) resolves the library from the repository,
	// not from a filesystem root: Root is legitimately empty there and must not
	// turn every project route into 409.
	if g.libRepo != nil || g.RootPath() != "" {
		return false
	}
	path := r.URL.Path
	needs := path == "/api/projects" || strings.HasPrefix(path, "/api/projects/")
	if path == "/api/reveal" {
		q := r.URL.Query()
		target, scope := q.Get("target"), q.Get("scope")
		if r.Method == http.MethodPost {
			var req revealRequest
			if body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)); err == nil {
				_ = json.Unmarshal(body, &req)
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			target, scope = req.Target, req.Scope
		}
		needs = !(target == "rules" && strings.TrimSpace(scope) == "global")
	}
	if needs {
		jsonError(w, http.StatusConflict, errNoLibrary.Error())
	}
	return needs
}

func isAdmin(user identity.User) bool { return user.Role == identity.RoleAdmin }

// libError maps library/repository errors with the shared policy (httperr.go):
// missing membership stays 404 (no existence leak), missing write permission
// 403, file-only capability 501, CAS/queue 409. Unexpected causes are logged
// server-side and answered with the generic 500.
func libError(w http.ResponseWriter, err error) {
	jsonMappedError(w, err)
}

// serveLibrary 处理 /api/library（GET 列表、PUT 配置）。返回 false 表示路径不归它管。
func (g *Gateway) serveLibrary(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	if r.URL.Path != "/api/library" {
		return false
	}
	if r.Method == "GET" && g.RootPath() == "" && g.libRepo == nil {
		g.writeUnconfiguredLibrary(w, user)
		return true
	}
	var lib library.Repository
	if r.Method != "PUT" || g.RootPath() != "" || g.libRepo != nil {
		var err error
		if lib, err = g.lib(r.Context(), actorOf(user)); err != nil {
			libraryError(w, err)
			return true
		}
	}
	switch r.Method {
	case "GET":
		actor := actorOf(user)
		scan, err := lib.Scan(r.Context(), actor)
		if err != nil {
			libError(w, err)
			return true
		}
		projects := []projectSummary{}
		for _, p := range scan.Projects {
			if !user.CanReadProject(p.ID) {
				continue
			}
			projects = append(projects, g.projectSummary(r.Context(), lib, user, p))
		}
		candidates := []string{}
		if isAdmin(user) {
			candidates = scan.Candidates
		}
		writeJSON(w, 200, map[string]any{
			"configured":     true,
			"storageMode":    g.libraryStorageMode(),
			"storageWarning": library.LibraryMountWarning(lib.RootPath()),
			"rootDisplay":    library.DiscoverPlaces().Display(lib.RootPath()),
			"rootPath":       g.responsePath(lib.RootPath(), isAdmin(user)),
			"config":         lib.Config(),
			"projects":       projects,
			"candidates":     candidates,
		})
	case "PUT":
		if !isAdmin(user) {
			jsonError(w, 403, "Không có quyền")
			return true
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			jsonError(w, 400, "JSON không hợp lệ")
			return true
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			jsonError(w, 400, jsonBodyMessage(err))
			return true
		}
		// 14 §G2：{root} 换 thư viện lúc chạy；其余形状仍是 LibraryConfig。
		if _, hasRoot := probe["root"]; hasRoot {
			var req struct {
				Root string `json:"root"`
			}
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				jsonError(w, 400, jsonBodyMessage(err))
				return true
			}
			g.switchLibraryRoot(w, req.Root)
			return true
		}
		if lib == nil {
			jsonError(w, http.StatusConflict, errNoLibrary.Error())
			return true
		}
		var cfg library.Config
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			jsonError(w, 400, jsonBodyMessage(err))
			return true
		}
		if msg := validateLibraryConfig(cfg); msg != "" {
			jsonError(w, 400, msg)
			return true
		}
		def := library.DefaultConfig()
		if cfg.Dirs.Ideas == "" {
			cfg.Dirs.Ideas = def.Dirs.Ideas
		}
		if cfg.Dirs.Assets == "" {
			cfg.Dirs.Assets = def.Dirs.Assets
		}
		if cfg.Dirs.Stories == "" {
			cfg.Dirs.Stories = def.Dirs.Stories
		}
		if cfg.Dirs.Exports == "" {
			cfg.Dirs.Exports = def.Dirs.Exports
		}
		if cfg.Dirs.Audio == "" {
			cfg.Dirs.Audio = def.Dirs.Audio
		}
		if cfg.SlugWords <= 0 {
			cfg.SlugWords = def.SlugWords
		}
		if cfg.AutoExport.Pattern == "" {
			cfg.AutoExport.Pattern = def.AutoExport.Pattern
		}
		if cfg.AutoExport.OnExists == "" {
			cfg.AutoExport.OnExists = def.AutoExport.OnExists
		}
		if cfg.AutoExport.Formats == nil {
			cfg.AutoExport.Formats = def.AutoExport.Formats
		}
		// 与 SaveConfig 写入的值保持一致，响应里不能出现 version: 0。
		cfg.Version = library.ConfigVersion
		if g.libRepo != nil {
			jsonError(w, 501, "cấu hình thư viện không đổi được trên backend hosted")
			return true
		}
		if err := library.SaveConfig(lib.RootPath(), cfg); err != nil {
			jsonError(w, 500, err.Error())
			return true
		}
		writeJSON(w, 200, cfg)
	default:
		jsonError(w, 404, "không tìm thấy")
	}
	return true
}

// switchLibraryRoot 实现 PUT /api/library {root}（14 §G2）：换书库根、停空闲 worker、
// 持久化到 ~/.ainovel/serve.json；有引擎在跑时拒绝（409）。
func (g *Gateway) switchLibraryRoot(w http.ResponseWriter, root string) {
	if library.LibraryRootsRestricted() && g.libRepo == nil {
		real, err := library.DiscoverPlaces().Confine(root)
		if err != nil {
			fsError(w, err)
			return
		}
		root = real
	}
	if running := g.runningWorkers(); len(running) > 0 {
		jsonError(w, 409, "Đang có truyện chạy — hãy đóng tất cả trước khi đổi thư viện")
		return
	}
	abs, err := library.CheckRoot(root)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if abs == g.RootPath() {
		cfg, err := library.LoadConfig(abs)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "root": abs, "rootDisplay": library.DiscoverPlaces().Display(abs), "config": cfg})
		return
	}
	// 空闲 worker 的 cwd 还在旧书库下，换根前统一停掉。
	g.closeWorkers()
	g.setRoot(abs)
	if err := library.SaveServeRoot(abs); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	cfg, err := library.LoadConfig(abs)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if warning := library.LibraryMountWarning(abs); warning != "" {
		slog.Warn(warning)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "root": abs, "rootDisplay": library.DiscoverPlaces().Display(abs), "config": cfg})
}

// validateLibraryConfig 校验 PUT /api/library 的字段值；返回空串表示通过。
func validateLibraryConfig(cfg library.Config) string {
	for name, dir := range map[string]string{
		"dirs.ideas": cfg.Dirs.Ideas, "dirs.assets": cfg.Dirs.Assets,
		"dirs.stories": cfg.Dirs.Stories, "dirs.exports": cfg.Dirs.Exports,
		"dirs.audio": cfg.Dirs.Audio,
	} {
		if dir != "" && !library.ValidDirName(dir) {
			return "Tên thư mục không hợp lệ: " + name
		}
	}
	if cfg.SlugWords < 0 || cfg.SlugWords > 12 {
		return "slugWords phải trong 1–12"
	}
	for _, f := range cfg.AutoExport.Formats {
		if f != "txt" && f != "epub" && f != "narration" {
			return "Định dạng xuất không hỗ trợ: " + f
		}
	}
	if cfg.AutoExport.OnExists != "" && cfg.AutoExport.OnExists != "replace" && cfg.AutoExport.OnExists != "keep" {
		return "autoExport.onExists phải là replace hoặc keep"
	}
	return ""
}

// serveProjects 处理 /api/projects 及以下全部路由。返回 false 表示路径不归它管。
func (g *Gateway) serveProjects(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	path := r.URL.Path
	if path != "/api/projects" && !strings.HasPrefix(path, "/api/projects/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/api/projects")
	if rest == "" || rest == "/" {
		if r.Method != "POST" {
			jsonError(w, 404, "không tìm thấy")
			return true
		}
		g.createProject(w, r, user)
		return true
	}
	segs := strings.Split(strings.TrimPrefix(rest, "/"), "/")
	// /api/projects/adopt（固定子路由，先于 :p 匹配）。
	if segs[0] == "adopt" && len(segs) == 1 {
		if r.Method != "POST" {
			jsonError(w, 404, "không tìm thấy")
			return true
		}
		g.adoptProject(w, r, user)
		return true
	}
	p := segs[0]
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, 500, err.Error())
		return true
	}
	if len(segs) == 1 {
		switch r.Method {
		case "GET":
			g.getProject(w, r, lib, user, p)
		case "PUT":
			g.updateProject(w, r, lib, user, p)
		default:
			jsonError(w, 404, "không tìm thấy")
		}
		return true
	}
	switch segs[1] {
	case "ledger":
		g.serveLedger(w, r, lib, user, p, segs[2:])
	case "universe":
		g.serveProjectUniverse(w, r, lib, user, p, segs[2:])
	case "search":
		g.serveProjectSearch(w, r, lib, user, p, segs[2:])
	case "stories":
		g.serveStories(w, r, lib, user, p, segs[2:])
	case "cocreate":
		g.serveProjectCoCreate(w, r, lib, user, p, segs[2:])
	case "voices":
		g.serveProjectVoices(w, r, user, p)
	case "pronunciations":
		g.serveProjectPronunciations(w, r, user, p)
	case "ideas":
		g.serveIdeas(w, r, lib, user, p, segs[2:])
	case "assets":
		if len(segs) != 2 {
			jsonError(w, 404, "không tìm thấy")
			return true
		}
		switch r.Method {
		case "GET":
			g.listAssets(w, r, lib, user, p)
		case "POST":
			g.uploadAsset(w, r, lib, user, p)
		default:
			jsonError(w, 404, "không tìm thấy")
		}
	case "validate":
		if len(segs) == 2 && r.Method == "GET" {
			g.validateProject(w, r, lib, user, p)
			return true
		}
		if len(segs) == 3 && segs[2] == "fix" && r.Method == "POST" {
			g.fixProject(w, r, lib, user, p)
			return true
		}
		jsonError(w, 404, "không tìm thấy")
	default:
		jsonError(w, 404, "không tìm thấy")
	}
	return true
}

// requireProject 检查项目存在且用户可读；不满足时已写响应并返回 false。
// "_" 只有在存在旧式故事时才算存在；普通项目必须有 project.json。
func (g *Gateway) requireProject(w http.ResponseWriter, lib library.Repository, ctx context.Context, actor *library.Actor, user identity.User, p string) bool {
	exists := false
	if p == library.LegacyProjectID {
		if dirs, err := lib.Stories(ctx, actor, library.LegacyProjectID); err == nil && len(dirs) > 0 {
			exists = true
		}
	} else if ref, ok := lib.ResolveProject(ctx, actor, p); ok {
		if pf, err := lib.ProjectFile(ctx, actor, p); err == nil && pf != nil {
			exists = true
		} else if ref.Dir != "" {
			if st2, err := os.Stat(filepath.Join(ref.Dir, library.ProjectFileName)); err == nil && !st2.IsDir() {
				exists = true
			}
		}
	}
	if !exists || !user.CanReadProject(p) {
		// Readers of an unadopted folder get an actionable state instead of a generic 404.
		if !exists && user.CanReadProject(p) {
			if scan, err := lib.Scan(ctx, actor); err == nil && slices.Contains(scan.Candidates, p) {
				writeJSON(w, 404, map[string]any{"error": "Thư mục này chưa được nhận vào thư viện", "candidate": true})
				return false
			}
		}
		jsonError(w, 404, "không tìm thấy dự án")
		return false
	}
	return true
}

func (g *Gateway) createProject(w http.ResponseWriter, r *http.Request, user identity.User) {
	if !isAdmin(user) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Notes       string `json:"notes"`
		// ShareGlobal: true (mặc định) = chép quy tắc “Mọi truyện” vào dự án, trợ lý AI kế thừa qua lớp cấu hình;
		// false = dự án không có quy tắc riêng và giữ ảnh chụp roles/style/budget hiện tại của “Mọi truyện”.
		ShareGlobal *bool `json:"shareGlobal"`
		// LegacyLanguage 只为容忍旧客户端（14 §B.4 已删 language）：解码后丢弃，
		// 否则 decodeJSON 的 DisallowUnknownFields 会把旧请求打成 400。
		LegacyLanguage json.RawMessage `json:"language,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	slug := strings.TrimSpace(req.Slug)
	if slug == "" && name == "" {
		jsonError(w, 400, "Nhập tên dự án")
		return
	}
	if name != "" && projectNameTaken(r.Context(), lib, actorOf(user), name) {
		jsonError(w, 409, "Đã có dự án cùng tên — đặt tên khác nhé")
		return
	}
	f := library.ProjectFile{Name: name, Description: req.Description, Notes: req.Notes}
	var dir string
	if slug == "" {
		dir, err = createProjectAutoSlug(r.Context(), lib, actorOf(user), name, f)
	} else {
		dir, err = lib.CreateProject(r.Context(), actorOf(user), slug, f)
	}
	if err != nil {
		libError(w, err)
		return
	}
	// Hosted projects have no directory: global rules/config are not mirrored
	// into a row store (inheritance stays in the config layer), and writing to
	// a relative path would litter the process cwd. filepath.Base maps both
	// backends onto the project id (dir path on file, slug on hosted).
	if ref, ok := lib.ResolveProject(r.Context(), actorOf(user), filepath.Base(dir)); ok && ref.Dir != "" {
		if req.ShareGlobal == nil || *req.ShareGlobal {
			if err := copyGlobalRules(ref.Dir); err != nil {
				slog.Warn("chép quy tắc toàn cục vào dự án thất bại", "module", "web", "dir", ref.Dir, "err", err)
			}
		} else if err := copyGlobalConfig(ref.Dir); err != nil {
			slog.Warn("chép cấu hình toàn cục vào dự án thất bại", "module", "web", "dir", ref.Dir, "err", err)
		}
	}
	writeJSON(w, 201, g.projectDetailFor(r.Context(), lib, user, filepath.Base(dir)))
}

// copyGlobalRules 把 ~/.ainovel/rules/*.md（含 quy-tac.md）复制进项目的 .ainovel/rules/；
// 不复制 roles/style/budget，AI 设置经正常的配置分层继承“Mọi truyện”。
func copyGlobalRules(projectDir string) error {
	src := rulesHomeDir()
	if src == "" {
		return nil
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	dst := rulesDirIn(projectDir)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// copyGlobalConfig 把全局 config.json 中的 roles/style/budget 复制到项目级
// （不复制 providers 与 api_key）。
func copyGlobalConfig(projectDir string) error {
	global, err := bootstrap.LoadConfigFile(bootstrap.DefaultConfigPath())
	if err != nil {
		return err
	}
	cfg := bootstrap.Config{
		Roles:  global.Roles,
		Style:  global.Style,
		Budget: global.Budget,
	}
	return bootstrap.SaveConfig(filepath.Join(projectDir, ".ainovel", "config.json"), cfg)
}

// projectNameTaken 判断是否已有同名项目（不区分大小写与首尾空白）。
func projectNameTaken(ctx context.Context, lib library.Repository, actor *library.Actor, name string) bool {
	scan, err := lib.Scan(ctx, actor)
	if err != nil {
		return false
	}
	for _, ref := range scan.Projects {
		if ref.Legacy {
			continue
		}
		if pf, err := lib.ProjectFile(ctx, actor, ref.ID); err == nil && pf != nil && strings.EqualFold(strings.TrimSpace(pf.Name), name) {
			return true
		}
	}
	return false
}

// createProjectAutoSlug 由显示名自动生成目录名（用户不可见）；目录名被其他项目占用时追加 -2、-3…。
func createProjectAutoSlug(ctx context.Context, lib library.Repository, actor *library.Actor, name string, f library.ProjectFile) (string, error) {
	base := library.Slugify(name, 0)
	if runes := []rune(base); len(runes) > 90 {
		base = strings.TrimRight(string(runes[:90]), "-")
	}
	for n := 1; n <= 99; n++ {
		slug := base
		if n > 1 {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		if !library.ValidDirName(slug) {
			continue
		}
		dir, err := lib.CreateProject(ctx, actor, slug, f)
		if errors.Is(err, library.ErrExists) {
			continue
		}
		return dir, err
	}
	return "", library.ErrExists
}

func (g *Gateway) adoptProject(w http.ResponseWriter, r *http.Request, user identity.User) {
	if !isAdmin(user) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	dir, err := lib.AdoptProject(r.Context(), actorOf(user), strings.TrimSpace(req.Dir), strings.TrimSpace(req.Name))
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 201, g.projectDetailFor(r.Context(), lib, user, filepath.Base(dir)))
}

// projectDetailFor 组装一个项目的完整响应：可读故事数 + project.json 字段。
func (g *Gateway) projectDetailFor(ctx context.Context, lib library.Repository, user identity.User, p string) projectDetail {
	ref, ok := lib.ResolveProject(ctx, actorOf(user), p)
	legacy := p == library.LegacyProjectID
	if !ok {
		ref = library.ProjectRef{ID: p, Legacy: legacy}
	}
	d := projectDetail{
		projectSummary: g.projectSummary(ctx, lib, user, ref),
		Path:           p,
	}
	if legacy {
		return d
	}
	if pf, err := lib.ProjectFile(ctx, actorOf(user), p); err == nil && pf != nil {
		d.Name = pf.Name
		d.Description = pf.Description
		d.Notes = pf.Notes
		d.CreatedAt = pf.CreatedAt
	}
	return d
}

func (g *Gateway) getProject(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	actor := actorOf(user)
	if !g.requireProject(w, lib, r.Context(), actor, user, p) {
		return
	}
	stories := []storySummary{}
	if dirs, err := lib.Stories(r.Context(), actor, p); err == nil {
		for _, sd := range dirs {
			id := sd.Project + "/" + sd.Name
			if !user.CanRead(id) {
				continue
			}
			st, ok := lib.ResolveStory(r.Context(), actor, sd.Project, sd.Name)
			if !ok {
				continue
			}
			stories = append(stories, g.storySummary(lib, st, &user))
		}
	}
	ideas, err := lib.ListIdeas(r.Context(), actor, p)
	if err != nil {
		libError(w, err)
		return
	}
	// Assets are a file-backend capability. A hosted library has no asset store
	// yet, so the overview reports an empty section instead of failing the
	// whole project page; the dedicated /assets endpoint still answers 501.
	assets, err := lib.Assets(r.Context(), actor, p)
	if err != nil {
		if !errors.Is(err, errs.ErrStorageUnsupported) {
			libError(w, err)
			return
		}
		assets = []library.Asset{}
	}
	if assets == nil {
		assets = []library.Asset{}
	}
	writeJSON(w, 200, map[string]any{
		"project": g.projectDetailFor(r.Context(), lib, user, p),
		"stories": stories,
		"ideas":   ideas,
		"assets":  assets,
	})
}

func (g *Gateway) updateProject(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	if p == library.LegacyProjectID {
		jsonError(w, 409, "Dự án ẩn danh không sửa được")
		return
	}
	actor := actorOf(user)
	if !g.requireProject(w, lib, r.Context(), actor, user, p) {
		return
	}
	if !user.CanManageProject(p) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Notes       string `json:"notes"`
		// UniverseName trống = xoá tên vũ trụ, quay về dùng tên dự án (câu 3).
		UniverseName string `json:"universeName"`
		// LegacyLanguage 同 createProject：旧客户端可能仍发送，解码后丢弃。
		LegacyLanguage json.RawMessage `json:"language,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	pf, err := lib.ProjectFile(r.Context(), actor, p)
	if err != nil {
		libError(w, err)
		return
	}
	if pf == nil {
		pf = &library.ProjectFile{}
	}
	pf.Name = req.Name
	pf.Description = req.Description
	pf.Notes = req.Notes
	pf.UniverseName = strings.TrimSpace(req.UniverseName)
	var saveErr error
	if repo, ok := lib.(metadataRepository); ok {
		saveErr = repo.UpdateProjectCAS(r.Context(), actor, p, *pf, strings.Trim(r.Header.Get("If-Match"), "\""))
	} else {
		saveErr = lib.UpdateProject(r.Context(), actor, p, *pf)
	}
	if saveErr != nil {
		g.metadataWriteError(w, r, user, saveErr)
		return
	}
	writeJSON(w, 200, g.projectDetailFor(r.Context(), lib, user, p))
}

// serveStories 处理 /api/projects/:p/stories[...] 下的路由；segs 为 stories 之后的段。
func (g *Gateway) serveStories(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string, segs []string) {
	actor := actorOf(user)
	if len(segs) == 0 {
		switch r.Method {
		case "GET":
			if !g.requireProject(w, lib, r.Context(), actor, user, p) {
				return
			}
			stories := []storySummary{}
			dirs, err := lib.Stories(r.Context(), actor, p)
			if err != nil {
				libError(w, err)
				return
			}
			for _, sd := range dirs {
				id := sd.Project + "/" + sd.Name
				if !user.CanRead(id) {
					continue
				}
				st, ok := lib.ResolveStory(r.Context(), actor, sd.Project, sd.Name)
				if ok {
					stories = append(stories, g.storySummary(lib, st, &user))
				}
			}
			writeJSON(w, 200, map[string]any{"stories": stories})
		case "POST":
			g.createStory(w, r, lib, user, p)
		default:
			jsonError(w, 404, "không tìm thấy")
		}
		return
	}
	s := segs[0]
	rest := ""
	if len(segs) > 1 {
		rest = "/" + strings.Join(segs[1:], "/")
	}
	st, ok := lib.ResolveStory(r.Context(), actor, p, s)
	if !ok {
		jsonError(w, 404, "không tìm thấy truyện")
		return
	}
	// Project-scoped permissions cover every story, including future stories.
	if !storyAccessAllowed(user, st, r.Method, rest) {
		jsonError(w, 404, "không tìm thấy truyện")
		return
	}
	if !(rest == "" && r.Method == http.MethodDelete) &&
		(r.Method != http.MethodGet || (strings.HasPrefix(rest, "/audio") && rest != "/audio/events")) {
		slot := g.slot(st.ID)
		if !slot.beginOperation() {
			jsonError(w, 409, "Truyện đang được xoá")
			return
		}
		defer slot.endOperation()
		if st.Dir != "" {
			if _, err := os.Stat(st.Dir); err != nil {
				jsonError(w, 404, "Không tìm thấy truyện")
				return
			}
		}
	}
	if r.Method == http.MethodGet && (rest == "/events/live" || rest == "/audio/events") {
		stream, cancel := g.authenticatedStoryStream(r, st.ID)
		r = stream
		defer cancel()
	}
	// 网关自有子路由先于直读/worker 转发。
	switch rest {
	case "":
		if r.Method == http.MethodDelete {
			g.deleteStory(w, r, lib, user, st)
			return
		}
	case "/meta":
		g.serveStoryMeta(w, r, lib, user, st)
		return
	case "/rename":
		if r.Method != "POST" {
			jsonError(w, 404, "không tìm thấy")
			return
		}
		g.renameStory(w, r, lib, user, st)
		return
	case "/close":
		if r.Method != "POST" {
			jsonError(w, 404, "không tìm thấy")
			return
		}
		g.closeBook(st.ID)
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if rest == "" {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	// 其余：直读钩子先匹配（不启动 worker），未命中再转发 worker。
	if rest == "/input" && r.Method == http.MethodPost && !g.checkInitialLedgerInput(w, r, lib, user, st) {
		return
	}
	if strings.HasPrefix(rest, "/audio") && g.serveStoryAudio(w, r, user, st, rest) {
		return
	}
	if g.serveStoryRead(w, r, user, st, rest) {
		return
	}
	if rest == "/events/live" {
		rest = "/events"
	}
	r.Header.Set("X-Ainovel-Username", user.Username)
	g.proxyToWorker(w, r, st.ID, strings.TrimPrefix(rest, "/"))
}

// All story reads require view; writes require collab; delete/share require owner.
func storyAccessAllowed(user identity.User, st library.Story, method, rest string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return user.CanRead(st.ID)
	}
	if method == http.MethodDelete || rest == "/delete" || rest == "/share" || strings.HasSuffix(rest, "/delete") || strings.HasSuffix(rest, "/share") {
		// Collab unkeep Cảnh báo (quyết định 2026-10-28): DELETE …/keep đi tới
		// handler như POST — các DELETE khác giữ owner-only.
		if method == http.MethodDelete && keepTail(rest) {
			return user.CanWrite(st.ID)
		}
		return user.CanManageProject(st.Project)
	}
	return user.CanWrite(st.ID)
}

// keepTail nhận đúng đuôi /review/items/{id}/keep (cùng hình serveReviewKeep).
func keepTail(rest string) bool {
	return strings.HasPrefix(rest, "/review/items/") && strings.HasSuffix(rest, "/keep")
}

func (g *Gateway) createStory(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	actor := actorOf(user)
	if !g.requireProject(w, lib, r.Context(), actor, user, p) {
		return
	}
	if !user.CanCollaborateProject(p) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Name string `json:"name"`
		Idea string `json:"idea"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Idea != "" {
		if _, _, err := lib.GetIdea(r.Context(), actor, p, req.Idea); err != nil {
			if errors.Is(err, library.ErrIdeaNotFound) {
				jsonError(w, 400, "ý tưởng không tồn tại: "+req.Idea)
			} else {
				libError(w, err)
			}
			return
		}
	}
	st, _, err := lib.CreateStory(r.Context(), actor, p, req.Name, req.Idea)
	if err != nil {
		libError(w, err)
		return
	}
	// Access is assigned to the project, so newly created stories need no grant.
	writeJSON(w, 201, g.storySummary(lib, st, &user))
}

// serveStoryMeta 处理 GET/PUT S/meta；授权已在 serveStories 的 storyAccessAllowed 完成。
func (g *Gateway) serveStoryMeta(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, st library.Story) {
	actor := actorOf(user)
	switch r.Method {
	case "GET":
		f, err := lib.LoadStoryFile(r.Context(), actor, st)
		if err != nil {
			libError(w, err)
			return
		}
		if f == nil {
			f = &library.StoryFile{Status: "idea", Tags: []string{}}
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		if f.Status == "" {
			f.Status = "idea"
		}
		// Tên hiển thị vá lười: chưa đổi tay → theo meta/book.json (vá truyện đặt tên
		// bằng dòng đầu yêu cầu), còn yêu cầu nhập ghi sẵn tên vào book.json rồi vào đây.
		engineTitle := g.engineBookTitle(st, &user)
		f.Title = displayTitle(f, engineTitle)
		g.syncStoryTitle(r.Context(), lib, st, engineTitle, actor)
		var builds []StoryBuildRange
		if sr, err := g.openStoryReader(user, st); err == nil {
			builds = storyBuilds(sr)
		}
		writeJSON(w, 200, struct {
			*library.StoryFile
			Builds []StoryBuildRange `json:"builds"`
		}{f, builds})
	case "PUT":
		var req library.StoryFile
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Status != "" && !library.StoryStatus[req.Status] {
			jsonError(w, 400, "status không hợp lệ: "+req.Status)
			return
		}
		// Gói AF: shortTitle ≤ 32 ký tự, không chứa ký tự tệp cấm (A7).
		req.ShortTitle = strings.TrimSpace(req.ShortTitle)
		if utf8.RuneCountInString(req.ShortTitle) > 32 {
			jsonError(w, 400, "Tên ngắn không được quá 32 ký tự.")
			return
		}
		if strings.ContainsAny(req.ShortTitle, `/\\:*?"<>|`) {
			jsonError(w, 400, "Tên ngắn không được chứa ký tự / \\ : * ? \" < > |.")
			return
		}
		// Đọc → sửa → ghi story.json xếp hàng với đồng bộ tiêu đề tự động (watcher).
		storyTitleMu.Lock()
		old, err := lib.LoadStoryFile(r.Context(), actor, st)
		if err != nil {
			storyTitleMu.Unlock()
			libError(w, err)
			return
		}
		f := req
		if f.Tags == nil {
			f.Tags = []string{}
		}
		if f.Status == "" {
			f.Status = "idea"
		}
		if old != nil && old.CreatedAt != nil {
			f.CreatedAt = old.CreatedAt
		} else if f.CreatedAt == nil {
			now := time.Now()
			f.CreatedAt = &now
		}
		// titleRenamed = tên khác tên sách engine (meta/book.json): chỉ tên lệch tên sách mới
		// là đổi tay; lưu tên trùng book (hoặc xoá trắng) thì truyện tiếp tục đồng bộ tự động.
		newTitle := strings.TrimSpace(f.Title)
		f.TitleRenamed = newTitle != "" && newTitle != strings.TrimSpace(g.engineBookTitle(st, &user))
		var saveErr error
		if repo, ok := lib.(metadataRepository); ok {
			saveErr = repo.WriteStoryFileCAS(r.Context(), actor, st, f, strings.Trim(r.Header.Get("If-Match"), "\""))
		} else {
			saveErr = lib.WriteStoryFile(r.Context(), actor, st, f)
		}
		if saveErr != nil {
			storyTitleMu.Unlock()
			g.metadataWriteError(w, r, user, saveErr)
			return
		}
		storyTitleMu.Unlock()
		writeJSON(w, 200, f)
	default:
		jsonError(w, 404, "không tìm thấy")
	}
}

// renameStory 处理 POST S/rename；授权已在 serveStories 的 storyAccessAllowed 完成。
func (g *Gateway) renameStory(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, st library.Story) {
	var req struct {
		Slug string `json:"slug"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if open, _ := g.workerState(st.ID); open {
		jsonError(w, 409, "Truyện đang mở — hãy đóng trước khi đổi tên")
		return
	}
	// File flock only exists in file mode; hosted exclusivity is the story
	// lease row claimed by the worker, which workerState above already covers.
	if st.Dir != "" && library.Locked(g.storyOutputDir(st)) {
		jsonError(w, 409, "Truyện đang bị tiến trình khác giữ")
		return
	}
	renamed, err := lib.RenameStory(r.Context(), actorOf(user), st.Project, st.Story, strings.TrimSpace(req.Slug))
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 200, g.storySummary(lib, renamed, &user))
}

func (g *Gateway) serveIdeas(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string, segs []string) {
	actor := actorOf(user)
	if len(segs) == 0 {
		switch r.Method {
		case "GET":
			if !g.requireProject(w, lib, r.Context(), actor, user, p) {
				return
			}
			ideas, err := lib.ListIdeas(r.Context(), actor, p)
			if err != nil {
				libError(w, err)
				return
			}
			writeJSON(w, 200, map[string]any{"ideas": ideas})
		case "POST":
			if !g.requireProject(w, lib, r.Context(), actor, user, p) {
				return
			}
			if p == library.LegacyProjectID {
				jsonError(w, 409, "Dự án ẩn danh không hỗ trợ ý tưởng")
				return
			}
			if !user.CanCollaborateProject(p) {
				jsonError(w, 403, "Không có quyền")
				return
			}
			var req struct {
				Text  string `json:"text"`
				Title string `json:"title"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			if strings.TrimSpace(req.Text) == "" {
				jsonError(w, 400, "text không được rỗng")
				return
			}
			if len(req.Text) > 20000 {
				jsonError(w, 400, "text vượt quá 20000 ký tự")
				return
			}
			idea, err := lib.CreateIdea(r.Context(), actor, p, req.Text, req.Title)
			if err != nil {
				libError(w, err)
				return
			}
			writeJSON(w, 201, idea)
		default:
			jsonError(w, 404, "không tìm thấy")
		}
		return
	}
	if len(segs) != 1 || r.Method != "GET" {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	if !g.requireProject(w, lib, r.Context(), actor, user, p) {
		return
	}
	idea, text, err := lib.GetIdea(r.Context(), actor, p, segs[0])
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": idea.ID, "title": idea.Title, "excerpt": idea.Excerpt,
		"createdAt": idea.CreatedAt, "updatedAt": idea.UpdatedAt, "usedBy": idea.UsedBy, "text": text,
	})
}

func (g *Gateway) listAssets(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	assets, err := lib.Assets(r.Context(), actorOf(user), p)
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"assets": assets})
}

func (g *Gateway) validateProject(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	v, err := lib.Validate(r.Context(), actorOf(user), p)
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (g *Gateway) fixProject(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string) {
	if !isAdmin(user) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	var req struct {
		Codes []string `json:"codes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	v, err := lib.Fix(r.Context(), actorOf(user), p, req.Codes)
	if err != nil {
		libError(w, err)
		return
	}
	writeJSON(w, 200, v)
}

// storyOutputDir 解析故事的引擎输出目录（worker 的 Host.OutputDir，绝对路径）。
// 配置损坏时退回默认相对路径，避免一个坏 config 让整个列表不可用。
func (g *Gateway) storyOutputDir(st library.Story) string {
	cfg, err := bootstrap.LoadConfigAt(st.Dir)
	if err != nil {
		return filepath.Join(st.Dir, "output", "novel")
	}
	cfg.FillDefaults()
	out := cfg.OutputDir
	if !filepath.IsAbs(out) {
		out = filepath.Join(st.Dir, out)
	}
	return out
}

// storyVolumes 统计卷数：分层大纲的卷数；有平铺大纲/卷首语时按 1 卷。
func storyVolumes(engine *store.Store) int {
	if volumes, err := engine.Outline.LoadLayeredOutline(); err == nil && len(volumes) > 0 {
		return len(volumes)
	}
	if entries, err := engine.Outline.LoadOutline(); err == nil && len(entries) > 0 {
		return 1
	}
	return 0
}

// storySummary 汇总一个故事卡片所需的全部字段（引擎信息 + story.json + 锁状态）。
func (g *Gateway) storySummary(lib library.Repository, st library.Story, user *identity.User) storySummary {
	sum := storySummary{
		ID: st.ID, Project: st.Project, Story: st.Story, Title: st.Story,
		Status: "idea", Tags: []string{}, TempName: tempNamePattern.MatchString(st.Story),
	}
	if st.Project == library.LegacyProjectID {
		sum.Folder = st.Story
	} else {
		sum.Folder = lib.Config().Dirs.Stories + "/" + st.Story
	}
	var updated time.Time
	var actor *library.Actor
	if user != nil {
		actor = actorOf(*user)
	}
	sf, sfErr := lib.LoadStoryFile(context.Background(), actor, st)
	if sfErr == nil && sf != nil {
		if sf.Title != "" {
			sum.Title = sf.Title
		}
		if library.StoryStatus[sf.Status] {
			sum.Status = sf.Status
		}
		if sf.Tags != nil {
			sum.Tags = sf.Tags
		}
		sum.Idea = sf.Idea
		sum.ShortTitle = strings.TrimSpace(sf.ShortTitle)
	}
	// Một store cho cả thẻ truyện: PeekBook, số quyển và badge lỗi đều đọc từ
	// cùng một lần mở (mỗi store.NewStore đọc lại meta/checkpoints.jsonl).
	// PG factory failure is reported, never papered over with a directory read;
	// the card simply carries no engine-derived fields for this request.
	engine, engineErr := g.openFactStore(st, user)
	if engineErr != nil {
		slog.Warn("mở kho dữ kiện truyện thất bại", "module", "web", "story_slug", st.ID, "err", engineErr)
		engine = nil
	}
	var info host.BookInfo
	var err error
	if engine != nil {
		info, err = host.PeekBookStore(engine)
	}
	if info.Phase == "" && err == nil && engine == nil {
		// no engine facts available; keep the story-card fields derived above
	} else if err != nil {
		slog.Warn("đọc trạng thái engine thất bại", "module", "web", "story_slug", st.ID, "err", err)
	} else {
		sum.Phase = info.Phase
		sum.CompletedChapters = info.CompletedChapters
		sum.TotalChapters = info.TotalChapters
		sum.TotalWords = info.TotalWords
		updated = info.UpdatedAt
		// Tên sách engine đặt (meta/book.json) là nguồn sự thật trừ khi người dùng đổi tay
		// (titleRenamed): truyện tạo từ brief không còn title lấy từ yêu cầu, còn bản ghi cũ
		// bị lỗi trước đây (title = dòng đầu brief) tự vá về tên sách.
		sum.Title = displayTitle(sf, info.Title)
		if sum.Title == "" {
			sum.Title = st.Story
		}
		// Vá lười: truyện đặt tên bằng dòng đầu yêu cầu (lỗi cũ) được ghi lại bằng tên sách.
		g.syncStoryTitle(context.Background(), lib, st, info.Title, actor)
		// story.json 缺省/无 status 时按引擎进度推断。
		if sf == nil || !library.StoryStatus[sf.Status] {
			switch {
			case info.Phase == string(domain.PhaseComplete):
				sum.Status = "done"
			case info.CompletedChapters > 0:
				sum.Status = "writing"
			}
		}
	}
	if engine != nil {
		sum.Volumes = storyVolumes(engine)
		if count, err := storyOpenErrors(st, engine); err == nil {
			sum.OpenErrors = count
		} else {
			slog.Warn("đọc chất lượng truyện thất bại", "module", "web", "story_slug", st.ID, "err", err)
		}
	}
	if st.Dir != "" {
		if st2, err2 := os.Stat(st.Dir); err2 == nil && st2.ModTime().After(updated) {
			updated = st2.ModTime()
		}
	} else if st.UpdatedAt != nil && st.UpdatedAt.After(updated) {
		// Hosted: the timestamp is the story row's updated_at, never a mtime.
		updated = *st.UpdatedAt
	}
	if !updated.IsZero() {
		t := updated
		sum.UpdatedAt = &t
	}
	sum.ExportsCount = g.countExportFiles(lib, st)
	sum.Open, sum.Running = g.workerState(st.ID)
	if !sum.Open && st.Dir != "" {
		sum.Locked = library.Locked(g.storyOutputDir(st))
	}
	return sum
}

func (g *Gateway) countExportFiles(lib library.Repository, st library.Story) int {
	if st.Dir == "" {
		if !g.pgMediaOK() || g.pgAudio.Cache == nil {
			return 0
		}
		files, err := g.pgHostedExportFiles(context.Background(), st, false)
		if err != nil {
			slog.Warn("đọc danh sách bản xuất thất bại", "module", "web", "story_slug", st.ID, "err", err)
			return 0
		}
		return len(files)
	}
	base := filepath.Join(st.Dir, lib.Config().Dirs.Exports)
	count := 0
	_ = filepath.WalkDir(base, func(_ string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			count++
		}
		return nil
	})
	return count
}

// projectSummary 汇总项目卡片（故事数只计当前用户可读的）。
func (g *Gateway) projectSummary(ctx context.Context, lib library.Repository, user identity.User, ref library.ProjectRef) projectSummary {
	sum := projectSummary{ID: ref.ID, Legacy: ref.Legacy}
	if ref.Legacy {
		sum.Name = "_"
	}
	actor := actorOf(user)
	var pf *library.ProjectFile
	if !ref.Legacy {
		var err error
		pf, err = lib.ProjectFile(ctx, actor, ref.ID)
		if err != nil {
			slog.Warn("đọc project.json thất bại", "module", "web", "project_slug", ref.ID, "err", err)
		}
	}
	if pf != nil {
		sum.Name = pf.Name
		sum.UniverseName = pf.UniverseName
		sum.Description = pf.Description
	}
	var latest time.Time
	if dirs, err := lib.Stories(ctx, actor, ref.ID); err == nil {
		for _, sd := range dirs {
			id := sd.Project + "/" + sd.Name
			if !user.CanRead(id) {
				continue
			}
			sum.StoryCount++
			if open, running := g.workerState(id); open && running {
				sum.RunningCount++
			}
			if sd.UpdatedAt != nil && sd.UpdatedAt.After(latest) {
				latest = *sd.UpdatedAt
			}
		}
	}
	if !latest.IsZero() {
		t := latest
		sum.UpdatedAt = &t
	}
	return sum
}
