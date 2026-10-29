package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 校验问题代码（fixable=true 的项能被 Fix 安全处理）。
const (
	issueMissingProjectJSON = "missing-project-json" // 项目缺 project.json
	issueInvalidProjectJSON = "invalid-project-json" // project.json 解析失败
	issueMissingDir         = "missing-dir"          // 缺少标准目录
	issueMissingStoryJSON   = "missing-story-json"   // 故事缺 story.json
	issueInvalidStoryJSON   = "invalid-story-json"   // story.json 解析失败
	issueInvalidName        = "invalid-name"         // 目录名不符合新建规范（旧数据可保留）
	issueLegacyRules        = "legacy-rules"         // _chung/*.md 应移入 .ainovel/rules/
	issueRulesSymlink       = "rules-symlink"        // 故事规则是指向项目规则的符号链接
)

// chungDir 是旧版共享规则目录名。
const chungDir = "_chung"

// Validate 检查项目目录结构：返回问题列表与标准/实际混合的目录树。
func (l *Library) Validate(project string) (*Validation, error) {
	pdir, ok := l.ResolveProject(project)
	if !ok {
		return nil, ErrNotFound
	}
	v := &Validation{Issues: []ValidationIssue{}}
	root := &FolderNode{Name: project, Path: "", Kind: "dir", Role: "project", Status: "ok"}

	if project != LegacyProjectID {
		l.validateProjectHeader(pdir, root, v)
	}
	if err := l.validateLegacyRules(pdir, v); err != nil {
		return nil, err
	}
	// 标准目录节点。
	expected := []struct{ role, name string }{
		{"ideas", l.Config.Dirs.Ideas},
		{"assets", l.Config.Dirs.Assets},
		{"stories", l.Config.Dirs.Stories},
	}
	if project == LegacyProjectID {
		expected = nil
	}
	seen := map[string]bool{"project.json": true, ".ainovel": true, chungDir: true}
	for _, e := range expected {
		seen[e.name] = true
	}
	for _, e := range expected {
		dir := filepath.Join(pdir, e.name)
		node := &FolderNode{Name: e.name, Path: e.name, Kind: "dir", Role: e.role, Status: "ok"}
		if !isDir(dir) {
			node.Status = "missing"
			l.issue(v, e.name, "warn", issueMissingDir, true, "Thiếu thư mục chuẩn %s", e.name)
		}
		root.Children = append(root.Children, node)
	}
	// 故事节点：匿名项目的故事直接挂在根下，普通项目挂到 <dirs.stories> 下。
	stories, err := l.Stories(project)
	if err != nil {
		return nil, err
	}
	storyParent := root
	for _, c := range root.Children {
		if c.Role == "stories" {
			storyParent = c
		}
	}
	for _, sd := range stories {
		seen[sd.Name] = true
		rel, _ := filepath.Rel(pdir, sd.Dir)
		node := &FolderNode{Name: sd.Name, Path: filepath.ToSlash(rel), Kind: "dir", Role: "story", Status: "ok"}
		if !ValidDirName(sd.Name) {
			node.Status = "invalid"
			l.issue(v, node.Path, "info", issueInvalidName, false, "Tên thư mục không theo chuẩn mới (vẫn dùng được)")
		}
		l.validateStory(pdir, sd, node, v)
		storyParent.Children = append(storyParent.Children, node)
	}
	// 其余顶层条目 → other/optional。
	if project != LegacyProjectID {
		l.listOtherEntries(pdir, pdir, "", seen, root, 0)
	}
	l.sortTree(root)
	v.Tree = root
	v.OK = len(v.Issues) == 0
	return v, nil
}

// validateProjectHeader 检查 project.json 与 .ainovel/ 节点。
func (l *Library) validateProjectHeader(pdir string, root *FolderNode, v *Validation) {
	pj := &FolderNode{Name: "project.json", Path: "project.json", Kind: "file", Role: "project", Status: "ok"}
	switch pf, err := LoadProjectFile(pdir); {
	case err != nil:
		pj.Status = "invalid"
		l.issue(v, pj.Path, "error", issueInvalidProjectJSON, false, "project.json hỏng: %v", err)
	case pf == nil:
		pj.Status = "missing"
		l.issue(v, pj.Path, "error", issueMissingProjectJSON, true, "Thiếu project.json")
	}
	root.Children = append(root.Children, pj)

	meta := &FolderNode{Name: ".ainovel", Path: ".ainovel", Kind: "dir", Role: "config", Status: "ok"}
	meta.Children = append(meta.Children, l.fileNode(pdir, ".ainovel/config.json", "config", true))
	rules := l.rulesNode(pdir, filepath.Join(pdir, ".ainovel", "rules"), ".ainovel/rules", v)
	if rules.Status == "missing" {
		l.issue(v, rules.Path, "warn", issueMissingDir, true, "Thiếu thư mục quy tắc .ainovel/rules")
	}
	meta.Children = append(meta.Children, rules)
	root.Children = append(root.Children, meta)
}

// validateStory 检查单个故事目录：story.json、规则符号链接与子树。
func (l *Library) validateStory(pdir string, sd StoryDir, node *FolderNode, v *Validation) {
	sf := &FolderNode{Name: "story.json", Path: joinSlash(node.Path, "story.json"), Kind: "file", Role: "storyMeta", Status: "ok"}
	switch f, err := LoadStoryFile(sd.Dir); {
	case err != nil:
		sf.Status = "invalid"
		l.issue(v, sf.Path, "error", issueInvalidStoryJSON, false, "story.json hỏng trong %s: %v", sd.Name, err)
	case f == nil:
		sf.Status = "missing"
		l.issue(v, sf.Path, "warn", issueMissingStoryJSON, true, "Thiếu story.json trong %s", sd.Name)
	}
	node.Children = append(node.Children, sf)

	entries, err := os.ReadDir(sd.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == StoryFileName {
			continue // 已作为 storyMeta 节点加入
		}
		rel := joinSlash(node.Path, name)
		switch {
		case name == "output":
			child := &FolderNode{Name: name, Path: rel, Kind: "dir", Role: "engine", Status: "engine"}
			// 只标记到 output/novel 一级，不展开引擎内部。
			if isDir(filepath.Join(sd.Dir, "output", "novel")) {
				child.Children = []*FolderNode{{Name: "novel", Path: rel + "/novel", Kind: "dir", Role: "engine", Status: "engine"}}
			}
			node.Children = append(node.Children, child)
		case name == l.Config.Dirs.Exports:
			child := &FolderNode{Name: name, Path: rel, Kind: "dir", Role: "exports", Status: "ok"}
			l.listDirNodes(filepath.Join(sd.Dir, name), rel, child, 2)
			node.Children = append(node.Children, child)
		case name == ".ainovel":
			child := &FolderNode{Name: name, Path: rel, Kind: "dir", Role: "config", Status: "ok"}
			l.storyRulesNode(pdir, sd, rel, child, v)
			node.Children = append(node.Children, child)
		default:
			kind := "file"
			if e.IsDir() {
				kind = "dir"
			}
			node.Children = append(node.Children, &FolderNode{Name: name, Path: rel, Kind: kind, Role: "other", Status: "optional"})
		}
	}
}

// storyRulesNode 检查故事的 .ainovel/rules：指向项目共享规则的符号链接应删除。
func (l *Library) storyRulesNode(pdir string, sd StoryDir, rel string, meta *FolderNode, v *Validation) {
	meta.Children = append(meta.Children, l.rulesNode(pdir, filepath.Join(sd.Dir, ".ainovel", "rules"), joinSlash(rel, "rules"), v))
}

// rulesNode 生成规则目录节点：目录本身或其条目若是指向项目内共享规则的符号链接，
// 标为 rules-symlink（Fix 只删链接；真实规则仍在项目级生效）。
// 目录本身是符号链接时不展开，否则会枚举到项目级规则并误删。
func (l *Library) rulesNode(pdir, dir, rel string, v *Validation) *FolderNode {
	node := &FolderNode{Name: "rules", Path: rel, Kind: "dir", Role: "rules", Status: "ok"}
	info, err := os.Lstat(dir)
	if err != nil {
		node.Status = "missing"
		return node
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if l.rulesSymlinkIntoProject(pdir, dir) {
			node.Status = "invalid"
			l.issue(v, rel, "warn", issueRulesSymlink, true,
				"Quy tắc đang là liên kết vào thư mục quy tắc dùng chung (sẽ xoá liên kết; quy tắc dự án vẫn áp dụng)")
		} else {
			node.Status = "optional"
		}
		return node
	}
	if !info.IsDir() {
		node.Status = "invalid"
		return node
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return node
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		childPath := joinSlash(rel, e.Name())
		li, lerr := os.Lstat(full)
		link := lerr == nil && li.Mode()&os.ModeSymlink != 0
		kind := "file"
		if e.IsDir() {
			kind = "dir"
		} else if link {
			// 符号链接的 DirEntry 不是目录；跟随一次只为标注 kind。
			if st, serr := os.Stat(full); serr == nil && st.IsDir() {
				kind = "dir"
			}
		}
		child := &FolderNode{Name: e.Name(), Path: childPath, Kind: kind, Role: "rules", Status: "ok"}
		switch {
		case link && l.rulesSymlinkIntoProject(pdir, full):
			child.Status = "invalid"
			l.issue(v, childPath, "warn", issueRulesSymlink, true,
				"Quy tắc %s là liên kết vào thư mục quy tắc dùng chung (sẽ xoá liên kết)", e.Name())
		case link:
			child.Status = "optional"
		case kind == "dir":
			l.listDirNodes(full, childPath, child, 2)
		}
		node.Children = append(node.Children, child)
	}
	return node
}

// rulesSymlinkIntoProject 判断符号链接是否指向项目内的共享规则来源
// （_chung/ 或项目自身的 .ainovel/rules/），用路径拼接而非 Stat 以容忍悬空链接。
func (l *Library) rulesSymlinkIntoProject(pdir, link string) bool {
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	target = filepath.Clean(target)
	for _, base := range []string{
		filepath.Join(pdir, chungDir),
		filepath.Join(pdir, ".ainovel", "rules"),
	} {
		// 链接可以指向规则目录本身（整目录链接），也可以指向其中某个文件。
		if target == base || strings.HasPrefix(target, base+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// validateLegacyRules 检查项目根的 _chung/*.md（旧共享规则位置）。
func (l *Library) validateLegacyRules(pdir string, v *Validation) error {
	dir := filepath.Join(pdir, chungDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // 没有 _chung 或不可读都不算问题
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		l.issue(v, joinSlash(chungDir, e.Name()), "warn", issueLegacyRules, true,
			"Quy tắc cũ %s nên chuyển vào .ainovel/rules/", e.Name())
	}
	return nil
}

func (l *Library) issue(v *Validation, path, level, code string, fixable bool, format string, args ...any) {
	v.Issues = append(v.Issues, ValidationIssue{Path: path, Level: level, Code: code, Fixable: fixable, Message: fmt.Sprintf(format, args...)})
}

// fileNode 生成单个期望文件的节点；missing=false 表示文件可选（不存在时状态 optional）。
func (l *Library) fileNode(pdir, rel, role string, optional bool) *FolderNode {
	node := &FolderNode{Name: filepath.Base(rel), Path: rel, Kind: "file", Role: role, Status: "ok"}
	if !hasFile(pdir, rel) {
		node.Status = "optional"
		if !optional {
			node.Status = "missing"
		}
	}
	return node
}

// listDirNodes 递归列出目录内容节点（other/optional），limit 为剩余深度。
func (l *Library) listDirNodes(dir, rel string, parent *FolderNode, limit int) {
	if limit <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		childRel := joinSlash(rel, e.Name())
		kind := "file"
		if e.IsDir() {
			kind = "dir"
		}
		child := &FolderNode{Name: e.Name(), Path: childRel, Kind: kind, Role: "other", Status: "optional"}
		if e.IsDir() {
			l.listDirNodes(filepath.Join(dir, e.Name()), childRel, child, limit-1)
		}
		parent.Children = append(parent.Children, child)
	}
}

// listOtherEntries 把 seen 之外的顶层条目加为 other 节点。
func (l *Library) listOtherEntries(pdir, dir, rel string, seen map[string]bool, root *FolderNode, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && e.Name() != ".ainovel" {
			continue
		}
		if seen[e.Name()] {
			continue
		}
		childRel := joinSlash(rel, e.Name())
		kind := "file"
		if e.IsDir() {
			kind = "dir"
		}
		child := &FolderNode{Name: e.Name(), Path: childRel, Kind: kind, Role: "other", Status: "optional"}
		if e.IsDir() && depth < 2 {
			l.listDirNodes(filepath.Join(dir, e.Name()), childRel, child, 2)
		}
		root.Children = append(root.Children, child)
	}
}

func (l *Library) sortTree(node *FolderNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.Kind != b.Kind {
			return a.Kind == "dir"
		}
		return a.Name < b.Name
	})
	for _, c := range node.Children {
		l.sortTree(c)
	}
}

func joinSlash(parts ...string) string {
	return strings.Join(parts, "/")
}

// Fix 修复 codes 指定的问题（nil/空 = 全部 fixable 项），然后重新校验返回。
func (l *Library) Fix(project string, codes []string) (*Validation, error) {
	before, err := l.Validate(project)
	if err != nil {
		return nil, err
	}
	pdir, _ := l.ResolveProject(project)
	want := map[string]bool{}
	for _, c := range codes {
		want[c] = true
	}
	var fixErrs []error
	for _, issue := range before.Issues {
		if !issue.Fixable || (len(want) > 0 && !want[issue.Code]) {
			continue
		}
		var ferr error
		switch issue.Code {
		case issueMissingProjectJSON:
			ferr = WriteProjectFile(pdir, ProjectFile{Name: project})
		case issueMissingDir:
			ferr = os.MkdirAll(filepath.Join(pdir, filepath.FromSlash(issue.Path)), 0o755)
		case issueMissingStoryJSON:
			// 标题留空：StorySummary 会依次回退到引擎 meta 标题、目录名，
			// 避免把目录 slug 写死成用户可见的标题。
			dir := filepath.Join(pdir, filepath.FromSlash(filepath.Dir(issue.Path)))
			// status 留空：由引擎进度推断（已完成的书不应被标成 idea）。
			ferr = WriteStoryFile(dir, StoryFile{Tags: []string{}})
		case issueLegacyRules:
			ferr = l.moveLegacyRule(pdir, issue.Path)
			if ferr == nil {
				removeEmptyDir(filepath.Join(pdir, chungDir))
			}
		case issueRulesSymlink:
			ferr = os.Remove(filepath.Join(pdir, filepath.FromSlash(issue.Path)))
		}
		if ferr != nil {
			fixErrs = append(fixErrs, fmt.Errorf("%s: %w", issue.Path, ferr))
		}
	}
	after, err := l.Validate(project)
	if err != nil {
		return nil, err
	}
	if len(fixErrs) > 0 {
		return after, errors.Join(fixErrs...)
	}
	return after, nil
}

// moveLegacyRule 把 _chung/<name> 移进 .ainovel/rules/；重名时追加序号而非覆盖。
func (l *Library) moveLegacyRule(pdir, rel string) error {
	src := filepath.Join(pdir, filepath.FromSlash(rel))
	name := filepath.Base(src)
	rulesDir := filepath.Join(pdir, ".ainovel", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(rulesDir, name)
	if _, err := os.Lstat(target); err == nil {
		stem := strings.TrimSuffix(name, ".md")
		for i := 2; ; i++ {
			target = filepath.Join(rulesDir, fmt.Sprintf("%s-%d.md", stem, i))
			if _, err := os.Lstat(target); os.IsNotExist(err) {
				break
			}
		}
	}
	return os.Rename(src, target)
}

// removeEmptyDir 在目录为空时删除（忽略失败：非空或不存在都无所谓）。
func removeEmptyDir(dir string) {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}
