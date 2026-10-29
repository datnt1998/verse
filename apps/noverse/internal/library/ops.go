package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// CreateProject 在书库根新建项目目录、project.json、.ainovel/rules 与全部 dirs.*。
// slug 必须符合 ValidDirName；目录已存在返回 ErrExists。
func (l *Library) CreateProject(slug string, f ProjectFile) (string, error) {
	if !ValidDirName(slug) {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, slug)
	}
	dir := filepath.Join(l.Root, slug)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", ErrExists
		}
		return "", err
	}
	// 失败时整体回滚：此目录由我们刚创建，删除是安全的。
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	if f.Name == "" {
		f.Name = slug
	}
	if f.CreatedAt == nil {
		now := time.Now()
		f.CreatedAt = &now
	}
	if err := WriteProjectFile(dir, f); err != nil {
		return "", err
	}
	if err := l.ensureProjectDirs(dir); err != nil {
		return "", err
	}
	ok = true
	return dir, nil
}

// ensureProjectDirs 创建项目的标准子目录（幂等）。
func (l *Library) ensureProjectDirs(dir string) error {
	for _, sub := range []string{
		filepath.Join(".ainovel", "rules"),
		l.Config.Dirs.Ideas,
		l.Config.Dirs.Assets,
		l.Config.Dirs.Stories,
	} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// AdoptProject 把候选目录认领为项目：只写 project.json 并补齐缺失的标准目录。
// dirName 必须出现在 Scan().Candidates 中，否则 ErrNotCandidate。
func (l *Library) AdoptProject(dirName, name string) (string, error) {
	if !ValidProjectID(dirName) {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, dirName)
	}
	scan, err := l.Scan()
	if err != nil {
		return "", err
	}
	candidate := false
	for _, c := range scan.Candidates {
		if c == dirName {
			candidate = true
			break
		}
	}
	if !candidate {
		return "", ErrNotCandidate
	}
	dir := filepath.Join(l.Root, dirName)
	if name == "" {
		name = dirName
	}
	now := time.Now()
	f := ProjectFile{Name: name, CreatedAt: &now}
	if err := WriteProjectFile(dir, f); err != nil {
		return "", err
	}
	if err := l.ensureProjectDirs(dir); err != nil {
		return "", err
	}
	return dir, nil
}

var numberedDirPattern = regexp.MustCompile(`^(\d+)-(.+)$`)

// splitNumbered 拆分 "NN-slug" 目录名；无编号返回 (-1, name)。
func splitNumbered(name string) (int, string) {
	m := numberedDirPattern.FindStringSubmatch(name)
	if m == nil {
		return -1, name
	}
	var n int
	for _, r := range m[1] {
		n = n*10 + int(r-'0')
	}
	return n, m[2]
}

// nextStoryNumber 返回父目录下现有 "NN-" 编号的最大值；无编号返回 0。
func nextStoryNumber(parent string) int {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return 0
	}
	max := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, _ := splitNumbered(e.Name()); n > max {
			max = n
		}
	}
	return max
}

// createStoryMu serialises the duplicate check with the matching mkdir for one gateway process.
var createStoryMu sync.Mutex

// CreateStory 在项目中新建故事目录 "NN-<slug>"：NN 取现有最大编号 +1（两位），
// slug 由工作名 Slugify 得到；无工作名时用 "truyen-moi"（tempName=true，提示后续改名）。
// 已有同 slug 的非临时故事时返回 ErrExists，防止重复提交建出第二个目录。
// 返回新故事位置与 tempName 标记。
func (l *Library) CreateStory(project, name, ideaID string) (Story, bool, error) {
	pdir, err := l.projectDirForWrite(project)
	if err != nil {
		return Story{}, false, err
	}
	parent := pdir
	if project != LegacyProjectID {
		parent = filepath.Join(pdir, l.Config.Dirs.Stories)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return Story{}, false, err
		}
	}
	name = strings.TrimSpace(name)
	slug := "truyen-moi"
	if name != "" {
		slug = Slugify(name, l.Config.SlugWords)
	}
	// 目录名上限 100 字符（NN- 占 3）。
	if utf8.RuneCountInString(slug) > 96 {
		slug = strings.TrimRight(string([]rune(slug)[:96]), "-")
	}
	tempName := slug == "truyen-moi"
	createStoryMu.Lock()
	defer createStoryMu.Unlock()
	if !tempName {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return Story{}, false, err
		}
		for _, e := range entries {
			if _, existing := splitNumbered(e.Name()); e.IsDir() && existing == slug {
				return Story{}, false, fmt.Errorf("đã có truyện «%s» cùng tên trong dự án: %w", e.Name(), ErrExists)
			}
		}
	}
	n := nextStoryNumber(parent) + 1
	var dirName, dir string
	for {
		dirName = fmt.Sprintf("%02d-%s", n, slug)
		dir = filepath.Join(parent, dirName)
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			break
		}
		n++
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return Story{}, false, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	if project == LegacyProjectID {
		// 匿名项目的故事必须能被旧式布局检测认到（output/ 或 .ainovel/）。
		if err := os.MkdirAll(filepath.Join(dir, ".ainovel"), 0o755); err != nil {
			return Story{}, false, err
		}
	}
	now := time.Now()
	f := StoryFile{Title: name, Status: "idea", Idea: ideaID, Tags: []string{}, CreatedAt: &now}
	if err := WriteStoryFile(dir, f); err != nil {
		return Story{}, false, err
	}
	ok = true
	return Story{Project: project, Story: dirName, ID: project + "/" + dirName, Dir: dir, ProjectDir: pdir, Root: l.Root}, tempName, nil
}

// RenameStory 把故事目录改名为 "NN-<slug>"（保留原编号，无编号补下一个）。
// slug 必须是不带编号前缀的合法目录名。目标已存在返回 ErrExists。
func (l *Library) RenameStory(project, story, slug string) (Story, error) {
	if !ValidDirName(slug) {
		return Story{}, fmt.Errorf("%w: %q", ErrInvalidName, slug)
	}
	st, ok := l.ResolveStory(project, story)
	if !ok {
		return Story{}, ErrNotFound
	}
	parent := filepath.Dir(st.Dir)
	n, _ := splitNumbered(st.Story)
	if n <= 0 {
		n = nextStoryNumber(parent) + 1
	}
	newName := fmt.Sprintf("%02d-%s", n, slug)
	if newName == st.Story {
		return st, nil
	}
	target := filepath.Join(parent, newName)
	if _, err := os.Lstat(target); err == nil {
		return Story{}, ErrExists
	}
	if err := os.Rename(st.Dir, target); err != nil {
		return Story{}, err
	}
	return Story{Project: project, Story: newName, ID: project + "/" + newName, Dir: target, ProjectDir: st.ProjectDir, Root: l.Root}, nil
}

// ideaNamePattern 匹配 <YYYYMMDD-HHmm>-<slug>.md 想法文件名。
var ideaNamePattern = regexp.MustCompile(`^(\d{8}-\d{4})-(.+)\.md$`)

func (l *Library) ideasDir(project string) (string, error) {
	pdir, err := l.projectDirForWrite(project)
	if err != nil {
		return "", err
	}
	if project == LegacyProjectID {
		return "", ErrLegacy
	}
	return filepath.Join(pdir, l.Config.Dirs.Ideas), nil
}

// firstLineTitle 从想法文本取标题：去掉首行的 Markdown 标记后截断。
func firstLineTitle(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#*"))
	if utf8.RuneCountInString(line) > 120 {
		line = string([]rune(line)[:120])
	}
	return line
}

func excerptOf(text string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > max {
		return string([]rune(text)[:max])
	}
	return text
}

func (l *Library) readIdea(dir, id string) (*Idea, string, error) {
	path := filepath.Join(dir, id+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrIdeaNotFound
		}
		return nil, "", err
	}
	st, _ := os.Stat(path)
	text := string(raw)
	idea := &Idea{ID: id}
	if m := ideaNamePattern.FindStringSubmatch(id + ".md"); m != nil {
		if t, err := time.ParseInLocation("20060102-1504", m[1], time.UTC); err == nil {
			idea.CreatedAt = &t
		}
	}
	if st != nil {
		mt := st.ModTime()
		idea.UpdatedAt = &mt
	}
	// 标题优先取正文第一行（CreateIdea 写入 "# 标题"）。
	title := firstLineTitle(text)
	if title != "" {
		_, rest, _ := strings.Cut(text, "\n")
		idea.Title = title
		idea.Excerpt = excerptOf(strings.TrimSpace(rest), 200)
	} else {
		idea.Title = id
		idea.Excerpt = excerptOf(text, 200)
	}
	return idea, text, nil
}

// ListIdeas 列出项目的想法文件（按 id 排序），并标注被哪些故事引用。
func (l *Library) ListIdeas(project string) ([]Idea, error) {
	dir, err := l.ideasDir(project)
	if err != nil {
		if errors.Is(err, ErrLegacy) {
			return []Idea{}, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Idea{}, nil
		}
		return nil, err
	}
	used := l.usedIdeas(project)
	out := []Idea{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			continue
		}
		idea, _, err := l.readIdea(dir, strings.TrimSuffix(name, ".md"))
		if err != nil {
			continue // 坏文件不阻塞列表
		}
		idea.UsedBy = used[idea.ID]
		if idea.UsedBy == nil {
			idea.UsedBy = []string{}
		}
		sort.Strings(idea.UsedBy)
		out = append(out, *idea)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// usedIdeas 返回 idea id → 引用它的故事 id 列表（story.json.idea）。
func (l *Library) usedIdeas(project string) map[string][]string {
	used := map[string][]string{}
	stories, err := l.Stories(project)
	if err != nil {
		return used
	}
	for _, sd := range stories {
		if f, ferr := LoadStoryFile(sd.Dir); ferr == nil && f != nil && f.Idea != "" {
			used[f.Idea] = append(used[f.Idea], sd.Project+"/"+sd.Name)
		}
	}
	return used
}

// GetIdea 读取一个想法及其正文。
func (l *Library) GetIdea(project, id string) (*Idea, string, error) {
	dir, err := l.ideasDir(project)
	if err != nil {
		return nil, "", err
	}
	if !cleanIdeaID(id) {
		return nil, "", ErrIdeaNotFound
	}
	idea, text, err := l.readIdea(dir, id)
	if err != nil {
		return nil, "", err
	}
	// 详情与列表必须一致：usedBy 为空时是 []，不是 null。
	idea.UsedBy = l.usedIdeas(project)[id]
	if idea.UsedBy == nil {
		idea.UsedBy = []string{}
	}
	sort.Strings(idea.UsedBy)
	return idea, text, nil
}

func cleanIdeaID(id string) bool {
	return id != "" && !strings.HasPrefix(id, ".") && !strings.ContainsAny(id, `/\`)
}

// CreateIdea 把一段文字存为 <dirs.ideas>/<YYYYMMDD-HHmm>-<slug>.md；返回新想法。
func (l *Library) CreateIdea(project, text, title string) (*Idea, error) {
	dir, err := l.ideasDir(project)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	derived := title == ""
	if derived {
		title = firstLineTitle(text)
	}
	now := time.Now().UTC() // identifier timestamps are UTC so moving hosts never renames a draft
	base := now.Format("20060102-1504") + "-" + Slugify(title, 6)
	id := base
	for i := 2; ; i++ {
		if _, err := os.Lstat(filepath.Join(dir, id+".md")); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	content := text
	switch {
	case strings.HasPrefix(strings.TrimSpace(text), "#") && !derived:
		// 显式标题优先：用标题替换正文首行标题，列表与详情才能显示用户填的标题。
		_, rest, _ := strings.Cut(text, "\n")
		content = "# " + title + "\n" + rest
	case !strings.HasPrefix(strings.TrimSpace(text), "#"):
		content = "# " + title + "\n\n" + text
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(content), 0o644); err != nil {
		return nil, err
	}
	idea, _, err := l.readIdea(dir, id)
	if err != nil {
		return nil, err
	}
	if derived {
		// 标题取自首行时，摘要应跳过正文第一行，避免与标题重复。
		_, rest, _ := strings.Cut(text, "\n")
		idea.Excerpt = excerptOf(strings.TrimSpace(rest), 200)
	}
	idea.UsedBy = []string{}
	return idea, nil
}

// maxAssets 与契约一致：资产列表最多 500 条。
const maxAssets = 500

// Assets 递归列出 <project>/<dirs.assets> 下的条目（按路径排序，上限 500）。
func (l *Library) Assets(project string) ([]Asset, error) {
	pdir, err := l.projectDirForWrite(project)
	if err != nil {
		return nil, err
	}
	if project == LegacyProjectID {
		return []Asset{}, nil
	}
	base := filepath.Join(pdir, l.Config.Dirs.Assets)
	out := []Asset{}
	_ = filepath.WalkDir(base, func(path string, e os.DirEntry, err error) error {
		if err != nil || len(out) >= maxAssets {
			if len(out) >= maxAssets && e != nil && e.IsDir() {
				return filepath.SkipDir
			}
			return err
		}
		if path == base {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return nil
		}
		mt := info.ModTime()
		a := Asset{Path: filepath.ToSlash(rel), UpdatedAt: &mt}
		if e.IsDir() {
			a.Kind = "dir"
		} else {
			a.Kind = "file"
			a.Size = info.Size()
		}
		out = append(out, a)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
