package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Library 绑定一个书库根目录与其实际加载的配置。
type Library struct {
	Root   string
	Config Config
}

// Open 打开书库根目录并加载 library.json（缺失时用默认配置）。
func Open(root string) (*Library, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("thư mục gốc không hợp lệ: %s", abs)
	}
	cfg, err := LoadConfig(abs)
	if err != nil {
		return nil, err
	}
	return &Library{Root: abs, Config: cfg}, nil
}

// ProjectRef 是扫描到的一个项目位置。
type ProjectRef struct {
	ID     string // 目录名；"_" 表示聚合旧式故事目录的匿名项目
	Dir    string // 绝对路径；匿名项目为书库根
	Legacy bool
}

// StoryDir 是扫描到的故事目录位置。
type StoryDir struct {
	Project   string     // 所属项目 id
	Name      string     // 故事目录名，即完整 id 的第二段
	Dir       string     // 绝对路径；hosted backend 为空（没有目录镜像）
	UpdatedAt *time.Time // file backend: 目录 mtime；hosted: 行 updated_at
}

// ScanResult 是 GET /api/library 需要的目录事实（不含引擎/权限信息）。
type ScanResult struct {
	Projects   []ProjectRef // 按 id 排序；"_" 若存在排在最后
	Candidates []string     // 无 project.json 但含 <dirs.stories> 的目录名，已排序
	Legacy     []string     // 归入 "_" 的旧式故事目录名，已排序
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func hasFile(dir, name string) bool {
	st, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !st.IsDir()
}

// dirModTime trả mtime của thư mục; lỗi đọc trả nil (không chặn danh sách).
func dirModTime(dir string) *time.Time {
	if st, err := os.Stat(dir); err == nil {
		t := st.ModTime()
		return &t
	}
	return nil
}

// isLegacyStoryDir 判断 root 下的旧式故事目录：无 project.json、无 <dirs.stories>，
// 但含 output/ 或 .ainovel/（旧布局）。
func (l *Library) isLegacyStoryDir(dir string) bool {
	if !isDir(dir) || hasFile(dir, ProjectFileName) {
		return false
	}
	if isDir(filepath.Join(dir, l.Config.Dirs.Stories)) {
		return false
	}
	return isDir(filepath.Join(dir, "output")) || isDir(filepath.Join(dir, ".ainovel"))
}

// Scan 扫描书库根目录，区分项目 / 候选 / 旧式故事目录。
func (l *Library) Scan() (*ScanResult, error) {
	entries, err := os.ReadDir(l.Root)
	if err != nil {
		return nil, err
	}
	res := &ScanResult{Projects: []ProjectRef{}, Candidates: []string{}, Legacy: []string{}}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(l.Root, name)
		if !isDir(dir) {
			continue
		}
		switch {
		case hasFile(dir, ProjectFileName):
			res.Projects = append(res.Projects, ProjectRef{ID: name, Dir: dir})
		case isDir(filepath.Join(dir, l.Config.Dirs.Stories)):
			res.Candidates = append(res.Candidates, name)
		case l.isLegacyStoryDir(dir):
			res.Legacy = append(res.Legacy, name)
		}
	}
	sort.Slice(res.Projects, func(i, j int) bool { return res.Projects[i].ID < res.Projects[j].ID })
	sort.Strings(res.Candidates)
	sort.Strings(res.Legacy)
	if len(res.Legacy) > 0 {
		res.Projects = append(res.Projects, ProjectRef{ID: LegacyProjectID, Dir: l.Root, Legacy: true})
	}
	return res, nil
}

// ResolveProject 返回项目目录；不存在时返回 ("", false)。
func (l *Library) ResolveProject(project string) (string, bool) {
	if project == LegacyProjectID {
		return l.Root, true
	}
	if !ValidProjectID(project) {
		return "", false
	}
	dir := filepath.Join(l.Root, project)
	if !isDir(dir) {
		return "", false
	}
	return dir, true
}

// projectDirForWrite 返回可写操作的项目目录；匿名项目用书库根，普通项目必须
// 真实存在且带 project.json（防止在候选目录里误建故事）。
func (l *Library) projectDirForWrite(project string) (string, error) {
	if project == LegacyProjectID {
		return l.Root, nil
	}
	dir, ok := l.ResolveProject(project)
	if !ok {
		return "", ErrNotFound
	}
	if !hasFile(dir, ProjectFileName) {
		return "", ErrNotFound
	}
	return dir, nil
}

// cleanSegment 校验单段路径名（拒绝空、.、..、分隔符、隐藏名）。
func cleanSegment(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.HasPrefix(name, ".") && !strings.ContainsAny(name, `/\`)
}

// ValidProjectID accepts the folder identifiers used by adopted projects as well
// as generated slugs. Authorization and migration must use this same boundary.
func ValidProjectID(name string) bool { return cleanSegment(name) }

// Stories 列出一个项目下的全部故事目录（按名字排序）。
func (l *Library) Stories(project string) ([]StoryDir, error) {
	var out []StoryDir
	if project == LegacyProjectID {
		scan, err := l.Scan()
		if err != nil {
			return nil, err
		}
		for _, name := range scan.Legacy {
			out = append(out, StoryDir{Project: project, Name: name, Dir: filepath.Join(l.Root, name), UpdatedAt: dirModTime(filepath.Join(l.Root, name))})
		}
		return out, nil
	}
	pdir, ok := l.ResolveProject(project)
	if !ok {
		return nil, ErrNotFound
	}
	parent := filepath.Join(pdir, l.Config.Dirs.Stories)
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(parent, name)
		if !isDir(dir) {
			continue
		}
		out = append(out, StoryDir{Project: project, Name: name, Dir: dir, UpdatedAt: dirModTime(dir)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ResolveStory 把 "<project>/<story>" 的两段解析为故事位置；不存在返回 false。
func (l *Library) ResolveStory(project, story string) (Story, bool) {
	if !ValidProjectID(project) || !cleanSegment(story) {
		return Story{}, false
	}
	if project == LegacyProjectID {
		dir := filepath.Join(l.Root, story)
		if !l.isLegacyStoryDir(dir) {
			return Story{}, false
		}
		return Story{Project: project, Story: story, ID: project + "/" + story, Dir: dir, ProjectDir: l.Root, Root: l.Root}, true
	}
	pdir, ok := l.ResolveProject(project)
	if !ok {
		return Story{}, false
	}
	dir := filepath.Join(pdir, l.Config.Dirs.Stories, story)
	if !isDir(dir) {
		return Story{}, false
	}
	return Story{Project: project, Story: story, ID: project + "/" + story, Dir: dir, ProjectDir: pdir, Root: l.Root}, true
}
