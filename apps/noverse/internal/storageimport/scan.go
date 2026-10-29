package storageimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ScanResult is one frozen source: the classified manifest plus the library
// structure (projects/stories/dirs) the decoders need. Nothing here holds an
// open handle, so the scan can finish before any write happens.
type ScanResult struct {
	Manifest    *Manifest
	SourceID    string
	Library     LibraryConfig
	Projects    []ProjectRef
	Stories     []StoryRef
	AccountsRaw []byte
	AccountsSHA string
	// Synthetic holds files that exist logically but not on disk (a legacy
	// engine directory has no story.json); the manifest still lists them so the
	// reverse export can materialize them.
	Synthetic map[string][]byte
}

// ProjectRef is a discovered project directory.
type ProjectRef struct {
	Slug   string // directory name; library.LegacyProjectID for the root project
	Dir    string
	Config ProjectMeta
}

// StoryRef is a discovered story directory inside a project.
type StoryRef struct {
	ProjectSlug string
	Slug        string
	Dir         string
	Meta        StoryMeta
}

// ScanOptions controls a scan.
type ScanOptions struct {
	Root        string
	Accounts    string
	SourceID    string
	SourceLabel string
}

// rootFS confines every read to the snapshot root. os.Root refuses absolute
// paths and ".." segments, and refuses to follow symlinks that escape.
type rootFS struct{ root *os.Root }

func (r rootFS) Open(name string) (fs.File, error) { return r.root.Open(name) }

// Scan walks the frozen snapshot once, classifies every path and hashes the
// bytes. It never writes to the source tree.
func Scan(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	if opts.SourceID == "" {
		return nil, errors.New("thiếu source id")
	}
	root, err := os.OpenRoot(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("mở snapshot nguồn: %w", err)
	}
	defer func() { _ = root.Close() }()
	rfs := rootFS{root: root}

	lib, err := loadLibrary(rfs)
	if err != nil {
		return nil, err
	}
	res := &ScanResult{SourceID: opts.SourceID, Library: lib, Synthetic: map[string][]byte{}}

	structure := map[string]string{} // relative dir -> "project" | "story"
	projects, stories, err := discover(rfs, lib)
	if err != nil {
		return nil, err
	}
	res.Projects, res.Stories = projects, stories
	for _, p := range projects {
		structure[p.Dir] = "project"
	}
	for _, s := range stories {
		structure[s.Dir] = "story"
		structure[path.Dir(s.Dir)] = "project"
	}

	manifest := &Manifest{
		Version:     1,
		SourceID:    opts.SourceID,
		SourceLabel: opts.SourceLabel,
		Stats:       map[string]int{},
	}
	err = fs.WalkDir(rfs, ".", func(rel string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if walkErr != nil {
			// Confinement/IO failure: record the path as quarantined so the
			// import cannot claim the source was fully classified.
			manifest.Files = append(manifest.Files, ManifestFile{
				Path: rel, Kind: KindExclude, Domain: DomainLibrary, Role: "unreadable",
				Reason: "không đọc được: " + scrubError(walkErr),
			})
			manifest.Stats[DispQuarantined]++
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			manifest.Files = append(manifest.Files, ManifestFile{
				Path: rel, Kind: KindExclude, Domain: DomainLibrary, Role: "unreadable",
				Reason: "không đọc được metadata: " + scrubError(err),
			})
			manifest.Stats[DispQuarantined]++
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			manifest.Files = append(manifest.Files, ManifestFile{
				Path: rel, Kind: KindExclude, Domain: DomainLibrary, Role: "symlink",
				Reason: "symlink không được nhập (chỉ file thật trong snapshot)",
			})
			manifest.Stats[DispQuarantined]++
			return nil
		}
		if !info.Mode().IsRegular() {
			manifest.Files = append(manifest.Files, ManifestFile{
				Path: rel, Kind: KindExclude, Domain: DomainLibrary, Role: "special",
				Reason: "không phải file thường",
			})
			manifest.Stats[DispQuarantined]++
			return nil
		}

		kind, domain, role, reason := classify(rel, structure, lib)
		digest, size, err := hashFile(rfs, rel)
		if err != nil {
			return fmt.Errorf("băm %s: %w", rel, err)
		}
		manifest.Files = append(manifest.Files, ManifestFile{
			Path: rel, Kind: kind, Domain: domain, Role: role,
			Size: size, SHA256: digest, Reason: reason,
		})
		manifest.Stats[kind]++
		if kind == KindFacts {
			manifest.Stats[DispImported]++
		} else if kind == KindBlob {
			manifest.Stats[DispBlob]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, p := range res.Projects {
		if p.Dir != "" {
			continue
		}
		// The anonymous project "_" has no project.json in file mode (the
		// library root is the project). The database needs a row, so the
		// manifest gets a synthesized file that the reverse export writes back.
		if _, err := fs.Stat(rfs, "project.json"); err == nil {
			continue
		}
		data, err := json.MarshalIndent(p.Config, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		res.Synthetic["project.json"] = data
		manifest.Files = append(manifest.Files, ManifestFile{
			Path: "project.json", Kind: KindFacts, Domain: DomainProject, Role: "project_meta",
			Size: int64(len(data)), SHA256: RawDigest(data),
			Reason: "project.json suy ra cho dự án ẩn danh \"_\" (chế độ file không có file này)",
		})
		manifest.Stats[KindFacts]++
		manifest.Stats[DispImported]++
	}
	for _, s := range res.Stories {
		storyPath := path.Join(s.Dir, "story.json")
		if _, err := fs.Stat(rfs, storyPath); err == nil {
			continue
		}
		meta := s.Meta
		if meta.Status == "" {
			meta.Status = "idea"
		}
		if meta.Title == "" {
			if raw, err := readFileFS(rfs, path.Join(s.Dir, "output/novel/meta/book.json")); err == nil {
				var book struct {
					Title string `json:"title"`
				}
				if json.Unmarshal(raw, &book) == nil {
					meta.Title = book.Title
				}
			}
		}
		data, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		res.Synthetic[storyPath] = data
		manifest.Files = append(manifest.Files, ManifestFile{
			Path: storyPath, Kind: KindFacts, Domain: DomainStory, Role: "story_meta",
			Size: int64(len(data)), SHA256: RawDigest(data),
			Reason: "story.json suy ra từ thư mục engine kiểu cũ (không có file nguồn)",
		})
		manifest.Stats[KindFacts]++
		manifest.Stats[DispImported]++
	}

	if opts.Accounts != "" {
		raw, err := os.ReadFile(opts.Accounts)
		if err != nil {
			return nil, fmt.Errorf("đọc file tài khoản: %w", err)
		}
		res.AccountsRaw = raw
		res.AccountsSHA = RawDigest(raw)
		manifest.Files = append(manifest.Files, ManifestFile{
			Path: PathAccounts, Kind: KindFacts, Domain: DomainAccounts,
			Role: "accounts", Size: int64(len(raw)), SHA256: res.AccountsSHA,
		})
		manifest.Stats[KindFacts]++
		manifest.Stats[DispImported]++
	}

	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	manifest.SHA256 = manifestDigest(manifest.Files)
	res.Manifest = manifest
	return res, nil
}

// manifestDigest hashes the identity fields only: size/digest/kind per path.
// Timestamps and reasons are excluded so the same bytes always agree.
func manifestDigest(files []ManifestFile) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%d\x00%s\x00%s\n", f.Path, f.Kind, f.Domain, f.Size, f.SHA256, f.Role)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashFile(rfs rootFS, rel string) (string, int64, error) {
	f, err := rfs.Open(rel)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// scrubError keeps absolute host paths out of the manifest/report.
func scrubError(err error) string {
	if err == nil {
		return ""
	}
	var perr *fs.PathError
	if errors.As(err, &perr) {
		return perr.Op + ": " + scrubPath(perr.Err.Error())
	}
	return scrubPath(err.Error())
}

func scrubPath(s string) string {
	if i := strings.LastIndexByte(s, '/'); i >= 0 && strings.Contains(s[:i], "/") {
		return s[i+1:]
	}
	return s
}

// LibraryConfig mirrors the on-disk library.json (dirs override + autoExport).
type LibraryConfig struct {
	Version    int            `json:"version"`
	Dirs       Dirs           `json:"dirs"`
	SlugWords  int            `json:"slugWords,omitempty"`
	AutoExport map[string]any `json:"autoExport,omitempty"`
}

// Dirs is the configurable library layout.
type Dirs struct {
	Ideas   string `json:"ideas"`
	Assets  string `json:"assets"`
	Stories string `json:"stories"`
	Exports string `json:"exports"`
	Audio   string `json:"audio"`
}

func (d Dirs) withDefaults() Dirs {
	if d.Ideas == "" {
		d.Ideas = "y-tuong"
	}
	if d.Assets == "" {
		d.Assets = "tai-san"
	}
	if d.Stories == "" {
		d.Stories = "kich_ban"
	}
	if d.Exports == "" {
		d.Exports = "xuat-ban"
	}
	if d.Audio == "" {
		d.Audio = "audio"
	}
	return d
}

func loadLibrary(rfs rootFS) (LibraryConfig, error) {
	cfg := LibraryConfig{}
	raw, err := readFileFS(rfs, ".ainovel/library.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			cfg.Dirs = Dirs{}.withDefaults()
			return cfg, nil
		}
		return cfg, fmt.Errorf("đọc library.json: %w", err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("library.json hỏng: %w", err)
	}
	cfg.Dirs = cfg.Dirs.withDefaults()
	return cfg, nil
}

func readFileFS(rfs rootFS, rel string) ([]byte, error) {
	f, err := rfs.Open(rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// discover finds project dirs (dir containing project.json) at the root and
// story dirs under each project's configured stories dir. It deliberately does
// not guess: a dir without project.json is a candidate, not a project.
func discover(rfs rootFS, lib LibraryConfig) ([]ProjectRef, []StoryRef, error) {
	var projects []ProjectRef
	entries, err := fs.ReadDir(rfs, ".")
	if err != nil {
		return nil, nil, err
	}
	candidates := []string{"."}
	for _, e := range entries {
		if e.IsDir() && e.Name() != ".ainovel" {
			candidates = append(candidates, e.Name())
		}
	}
	for _, dir := range candidates {
		metaPath := path.Join(dir, "project.json")
		metaPath = strings.TrimPrefix(metaPath, "./")
		raw, err := readFileFS(rfs, metaPath)
		if err != nil {
			continue
		}
		var meta ProjectMeta
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, nil, fmt.Errorf("%s hỏng: %w", metaPath, err)
		}
		slug := dir
		if dir == "." {
			slug = "_"
			dir = ""
		}
		projects = append(projects, ProjectRef{Slug: slug, Dir: dir, Config: meta})
	}
	// Legacy engine layout: a root-level directory that holds output/novel or
	// audio/ but no project.json is a story of the anonymous project "_"
	// (library.LegacyProjectID). It is imported under that project, never
	// silently dropped and never guessed to be a project.
	var legacy []StoryRef
	hasLegacy := false
	for _, dir := range candidates {
		if dir == "." || dir == ".ainovel" {
			continue
		}
		if _, err := fs.Stat(rfs, path.Join(dir, "project.json")); err == nil {
			continue
		}
		isStory := false
		for _, marker := range []string{"output/novel", "story.json"} {
			if info, err := fs.Stat(rfs, path.Join(dir, marker)); err == nil && info.IsDir() {
				isStory = true
			}
		}
		if !isStory {
			if _, err := fs.Stat(rfs, path.Join(dir, "audio")); err == nil {
				isStory = true
			}
		}
		if !isStory {
			continue
		}
		meta := StoryMeta{Status: "idea"}
		if raw, err := readFileFS(rfs, path.Join(dir, "story.json")); err == nil {
			_ = json.Unmarshal(raw, &meta)
			if meta.Status == "" {
				meta.Status = "idea"
			}
		}
		legacy = append(legacy, StoryRef{ProjectSlug: "_", Slug: dir, Dir: dir, Meta: meta})
		hasLegacy = true
	}
	if hasLegacy {
		known := false
		for _, p := range projects {
			if p.Slug == "_" {
				known = true
			}
		}
		if !known {
			projects = append(projects, ProjectRef{Slug: "_", Dir: "", Config: ProjectMeta{Name: "(không tên)"}})
		}
	}
	var stories []StoryRef
	for _, p := range projects {
		if p.Dir == "" {
			continue
		}
		storiesDir := path.Join(p.Dir, lib.Dirs.Stories)
		storiesDir = strings.TrimPrefix(storiesDir, "./")
		children, err := fs.ReadDir(rfs, storiesDir)
		if err != nil {
			continue
		}
		for _, child := range children {
			if !child.IsDir() {
				continue
			}
			storyDir := path.Join(storiesDir, child.Name())
			raw, err := readFileFS(rfs, path.Join(storyDir, "story.json"))
			if err != nil {
				continue
			}
			var meta StoryMeta
			if err := json.Unmarshal(raw, &meta); err != nil {
				return nil, nil, fmt.Errorf("%s/story.json hỏng: %w", storyDir, err)
			}
			stories = append(stories, StoryRef{ProjectSlug: p.Slug, Slug: child.Name(), Dir: storyDir, Meta: meta})
		}
	}
	stories = append(stories, legacy...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].Slug < projects[j].Slug })
	sort.Slice(stories, func(i, j int) bool { return stories[i].Dir < stories[j].Dir })
	return projects, stories, nil
}

// classify assigns exactly one class/domain/role to a path. The ordering is
// most-specific first; anything unmatched is a fact candidate only when it is
// a regular JSON/text file inside a known project/story, otherwise it is
// quarantined.
func classify(rel string, structure map[string]string, lib LibraryConfig) (kind, domain, role, reason string) {
	dir := path.Dir(rel)
	base := path.Base(rel)

	// Root-level library state.
	if rel == ".ainovel/library.json" {
		return KindFacts, DomainLibrary, "library_config", ""
	}
	if rel == "project.json" {
		return KindFacts, DomainProject, "project_meta", ""
	}
	if strings.HasPrefix(rel, ".ainovel/") {
		if strings.HasSuffix(rel, ".lock") {
			return KindScratch, DomainLibrary, "lock", "khoá flock, không phải dữ liệu"
		}
		return KindLocal, DomainLibrary, "root_config", "cấu hình máy cục bộ (provider/secret) không chuyển lên VPS"
	}

	// Project-level (direct child of a project dir).
	if structure[dir] == "project" {
		return classifyProject(rel, dir, base, lib)
	}
	// Story-level.
	if structure[dir] == "story" {
		return classifyStory(rel, dir, base, lib)
	}
	if kind, dom, role, reason, ok := classifyStoryDeep(rel, structure, base); ok {
		return kind, dom, role, reason
	}
	// Files under a project but in an unclassified subdirectory.
	if parent := nearest(structure, dir); parent != "" {
		return KindFacts, DomainProject, "project_file", ""
	}
	// A directory that is neither a project (no project.json) nor a story under
	// one is a candidate the library never adopted; it is excluded with a
	// reason rather than imported into a project it does not belong to.
	if first := strings.SplitN(rel, "/", 2)[0]; first != "." && first != ".ainovel" {
		return KindExclude, DomainLibrary, "not_adopted", "thư mục chưa được adopt (thiếu project.json)"
	}
	if strings.HasSuffix(base, ".lock") {
		return KindScratch, DomainLibrary, "lock", "khoá flock ngoài cây dự án"
	}
	return KindFacts, DomainLibrary, "unknown", "path chưa có quy tắc phân loại"
}

func nearest(structure map[string]string, dir string) string {
	for dir != "." && dir != "/" {
		if structure[dir] != "" {
			return structure[dir]
		}
		dir = path.Dir(dir)
	}
	return ""
}

func classifyProject(rel, dir, base string, lib LibraryConfig) (string, string, string, string) {
	switch {
	case base == "project.json":
		return KindFacts, DomainProject, "project_meta", ""
	case rel == path.Join(dir, ".ainovel", "config.json"):
		return KindFacts, DomainProject, "project_config", ""
	case strings.HasPrefix(rel, path.Join(dir, ".ainovel", "rules")+"/"):
		return KindFacts, DomainProject, "project_rules", ""
	case rel == path.Join(dir, ".ainovel", "cach-doc.json"):
		return KindFacts, DomainProject, "pronunciation", ""
	case strings.HasPrefix(rel, path.Join(dir, ".ainovel", "drafts")+"/"):
		return KindFacts, DomainProject, "project_draft", ""
	case strings.HasSuffix(rel, ".lock"):
		return KindScratch, DomainProject, "lock", "khoá flock"
	case strings.Contains(base, ".tmp-"):
		return KindScratch, DomainProject, "temp", "file tạm của ghi nguyên tử"
	case strings.HasPrefix(rel, path.Join(dir, lib.Dirs.Assets)+"/"):
		if strings.HasSuffix(base, "giong.json") {
			return KindFacts, DomainAsset, "voice_index", ""
		}
		return KindBlob, DomainAsset, "asset", ""
	case strings.HasPrefix(rel, path.Join(dir, lib.Dirs.Ideas)+"/"):
		return KindFacts, DomainProject, "idea", ""
	case strings.HasPrefix(rel, path.Join(dir, lib.Dirs.Exports)+"/"):
		if strings.HasSuffix(base, ".json") {
			return KindFacts, DomainAsset, "export_meta", ""
		}
		return KindBlob, DomainAsset, "export", ""
	}
	return KindFacts, DomainProject, "project_file", ""
}

func classifyStory(rel, dir, base string, lib LibraryConfig) (string, string, string, string) {
	for _, rule := range storyRules {
		if rule.match(rel, dir, base, lib) {
			return rule.kind, rule.domain, rule.role, rule.reason
		}
	}
	return KindFacts, DomainStory, "story_file", ""
}

// classifyStoryDeep handles files below a story dir (structure lookup misses
// those because only the story root is tagged).
func classifyStoryDeep(rel string, structure map[string]string, base string) (string, string, string, string, bool) {
	d := path.Dir(rel)
	for d != "." && d != "/" {
		if structure[d] == "story" {
			kind, dom, role, reason := classifyStoryRelative(strings.TrimPrefix(rel, d+"/"), base)
			return kind, dom, role, reason, true
		}
		d = path.Dir(d)
	}
	return "", "", "", "", false
}

type storyRule struct {
	match  func(rel, dir, base string, lib LibraryConfig) bool
	kind   string
	domain string
	role   string
	reason string
}

func under(dir, sub string) bool { return strings.HasPrefix(dir, sub) }

var storyRules = []storyRule{
	{match: func(rel, dir, base string, _ LibraryConfig) bool { return base == "story.json" },
		kind: KindFacts, domain: DomainStory, role: "story_meta"},
	{match: func(rel, dir, base string, _ LibraryConfig) bool {
		return rel == path.Join(dir, ".ainovel", "config.json")
	},
		kind: KindFacts, domain: DomainStory, role: "story_config"},
	{match: func(rel, dir, base string, _ LibraryConfig) bool {
		return strings.HasPrefix(rel, path.Join(dir, ".ainovel", "rules")+"/")
	}, kind: KindFacts, domain: DomainStory, role: "story_rules"},
	{match: func(rel, dir, base string, _ LibraryConfig) bool {
		return strings.HasSuffix(base, ".lock")
	}, kind: KindScratch, domain: DomainStory, role: "lock", reason: "khoá flock"},
	{match: func(rel, dir, base string, _ LibraryConfig) bool {
		return strings.Contains(base, ".tmp-")
	}, kind: KindScratch, domain: DomainStory, role: "temp", reason: "file tạm của ghi nguyên tử"},
}

// classifyStoryRelative classifies a path relative to its story root.
func classifyStoryRelative(rel, base string) (string, string, string, string) {
	switch {
	case rel == "story.json":
		return KindFacts, DomainStory, "story_meta", ""
	case strings.HasPrefix(rel, "audio/cache/"):
		return KindDerived, DomainAudio, "audio_cache", "cache WAV theo hash tổng hợp, tái tạo được từ script + giọng"
	case strings.HasPrefix(rel, "audio/export-cache/"):
		return KindDerived, DomainAudio, "export_cache_dir", "cache xuất audio, tái tạo từ manifest nội dung"
	case strings.HasPrefix(rel, "audio/chuong/"):
		if strings.HasSuffix(base, ".json") {
			return KindFacts, DomainAudio, "audio_chapter_meta", ""
		}
		return KindBlob, DomainAudio, "audio_chapter", ""
	case strings.HasPrefix(rel, "audio/ung-vien/"):
		return KindBlob, DomainAudio, "audio_candidate", ""
	case strings.HasPrefix(rel, "audio/ban-doc/"):
		return KindFacts, DomainAudio, "audio_script", ""
	case rel == "audio/settings.json":
		return KindFacts, DomainAudio, "audio_settings", ""
	case rel == "audio/vai.json":
		return KindFacts, DomainAudio, "audio_cast", ""
	case rel == "audio/flags.json":
		return KindFacts, DomainAudio, "audio_flags", ""
	case rel == "audio/queue.json":
		return KindFacts, DomainAudio, "audio_queue", ""
	case rel == "audio/export-requests.json":
		return KindFacts, DomainAudio, "audio_export_requests", ""
	case strings.HasPrefix(rel, "audio/"):
		if ct := ContentType(base); strings.HasPrefix(ct, "text/") || ct == "application/json" {
			return KindFacts, DomainAudio, "audio_file", ""
		}
		return KindBlob, DomainAudio, "audio_asset", ""
	case strings.HasPrefix(rel, "xuat-ban/"), strings.HasPrefix(rel, "exports/"):
		if strings.HasSuffix(base, ".json") {
			return KindFacts, DomainAsset, "export_meta", ""
		}
		return KindBlob, DomainAsset, "export", ""
	case strings.HasPrefix(rel, ".ainovel/gop-y/"):
		switch {
		case base == "_events.json":
			return KindFacts, DomainComment, "comment_events", ""
		case strings.HasSuffix(base, ".json"):
			return KindFacts, DomainComment, "comments", ""
		default:
			return KindScratch, DomainComment, "lock", "khoá flock của comments"
		}
	case strings.HasPrefix(rel, ".ainovel/"):
		return KindFacts, DomainStory, "story_document", ""
	case strings.HasPrefix(rel, "output/novel/logs/"), strings.HasPrefix(rel, "logs/"):
		return KindDerived, DomainEngine, "logs", "log chẩn đoán, không phải dữ liệu nghiệp vụ"
	case strings.HasPrefix(rel, "output/novel/nao/"):
		return KindDerived, DomainEngine, "nao", "vault markdown dẫn xuất từ JSON ledger"
	case strings.HasSuffix(rel, ".lock"):
		return KindScratch, DomainEngine, "lock", "khoá flock"
	case strings.Contains(base, ".tmp-"):
		return KindScratch, DomainEngine, "temp", "file tạm của ghi nguyên tử"
	case strings.HasPrefix(rel, "output/novel/meta/import.init-"):
		return KindScratch, DomainEngine, "import_scratch", "workspace import chưa commit"
	case strings.HasPrefix(rel, "output/novel/meta/import/"):
		if base == "source.txt" {
			return KindBlob, DomainAsset, "import_source", ""
		}
		return KindFacts, DomainEngine, "import_artifact", ""
	case rel == "output/novel/meta/book.json":
		return KindFacts, DomainEngine, "engine_book", ""
	case rel == "output/novel/book.md":
		return KindDerived, DomainEngine, "projection", "bản chiếu của meta/book.json"
	case rel == "output/novel/meta/format.json":
		return KindFacts, DomainEngine, "engine_format", ""
	case rel == "output/novel/meta/progress.json":
		return KindFacts, DomainEngine, "engine_progress", ""
	case rel == "output/novel/meta/run.json":
		return KindFacts, DomainEngine, "engine_run", ""
	case strings.HasPrefix(rel, "output/novel/meta/runtime"):
		return KindFacts, DomainEngine, "engine_runtime", ""
	case strings.HasPrefix(rel, "output/novel/meta/chapter_records/"):
		return KindFacts, DomainEngine, "chapter_record", ""
	case strings.HasPrefix(rel, "output/novel/meta/pending_"):
		return KindFacts, DomainEngine, "engine_pending", ""
	case rel == "output/novel/meta/checkpoints.jsonl":
		return KindFacts, DomainStream, "checkpoint_stream", ""
	case rel == "output/novel/meta/decisions.jsonl":
		return KindFacts, DomainStream, "decision_stream", ""
	case rel == "output/novel/meta/usage.json":
		return KindFacts, DomainStream, "usage_doc", ""
	case rel == "output/novel/meta/usage.json.tmp-0" || strings.HasPrefix(base, "usage.json.tmp-"):
		return KindScratch, DomainStream, "temp", "file tạm của usage.json"
	case rel == "output/novel/meta/rewrites.jsonl":
		return KindFacts, DomainStream, "rewrite_stream", ""
	case rel == "output/novel/meta/rule_violations.jsonl":
		return KindFacts, DomainStream, "violation_stream", ""
	case rel == "output/novel/meta/state_changes.jsonl":
		return KindFacts, DomainStream, "world_change_stream", ""
	case strings.HasPrefix(rel, "output/novel/meta/sessions/"):
		return KindFacts, DomainStream, "session_stream", ""
	case strings.HasPrefix(rel, "output/novel/meta/snapshots/"):
		return KindFacts, DomainEngine, "snapshot", ""
	case rel == "output/novel/meta/diag-export.md":
		return KindDerived, DomainEngine, "diagnostic", "bản chẩn đoán, tái tạo được"
	case rel == "output/novel/meta/user_rules_cache.json":
		return KindDerived, DomainEngine, "cache", "cache quy tắc, tái tạo từ user_rules.json"
	case strings.HasPrefix(rel, "output/novel/meta/"):
		return KindFacts, DomainEngine, "engine_doc", ""
	case rel == "output/novel/outline.json":
		return KindFacts, DomainEngine, "outline_flat", ""
	case rel == "output/novel/layered_outline.json":
		return KindFacts, DomainEngine, "outline_layered", ""
	case rel == "output/novel/premise.md":
		return KindFacts, DomainEngine, "outline_premise", ""
	case rel == "output/novel/characters.json":
		return KindFacts, DomainEngine, "characters", ""
	case rel == "output/novel/world_rules.json":
		return KindFacts, DomainEngine, "world_rules", ""
	case rel == "output/novel/foreshadow_ledger.json":
		return KindFacts, DomainEngine, "foreshadow", ""
	case rel == "output/novel/address_terms.json":
		return KindFacts, DomainEngine, "address_terms", ""
	case rel == "output/novel/relationship_state.json":
		return KindFacts, DomainEngine, "relationship_state", ""
	case rel == "output/novel/relationship_log.jsonl":
		return KindFacts, DomainStream, "relationship_stream", ""
	case rel == "output/novel/timeline.jsonl":
		return KindFacts, DomainStream, "timeline_stream", ""
	case strings.HasPrefix(rel, "output/novel/reviews/rounds/"):
		return KindFacts, DomainReview, "review_round", ""
	case strings.HasPrefix(rel, "output/novel/reviews/"):
		if base == "decisions.json" {
			return KindFacts, DomainReview, "review_decisions", ""
		}
		return KindFacts, DomainReview, "review_latest", ""
	case strings.HasPrefix(rel, "output/novel/chapters/"):
		return KindFacts, DomainEngine, "chapter_prose", ""
	case strings.HasPrefix(rel, "output/novel/drafts/"):
		return KindFacts, DomainEngine, "draft", ""
	case strings.HasPrefix(rel, "output/novel/summaries/"):
		return KindFacts, DomainEngine, "summary", ""
	case strings.HasSuffix(rel, ".md"):
		return KindDerived, DomainEngine, "projection", "markdown dẫn xuất từ JSON nguồn"
	}
	return KindFacts, DomainEngine, "engine_file", ""
}

// timePtr parses an optional RFC3339 timestamp from source JSON.
func timePtr(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}

func filepathClean(p string) string { return filepath.ToSlash(filepath.Clean(p)) }
