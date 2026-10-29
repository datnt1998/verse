package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func openTemp(t *testing.T) (*Library, string) {
	t.Helper()
	root := t.TempDir()
	lib, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return lib, root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanClassifiesProjectCandidateLegacy(t *testing.T) {
	lib, root := openTemp(t)
	// 项目：有 project.json。
	mustWrite(t, filepath.Join(root, "du-an", "project.json"), `{"name":"Dự Án"}`)
	// 候选：无 project.json 但有 kich_ban/。
	if err := os.MkdirAll(filepath.Join(root, "cu-dir", "kich_ban"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 旧式故事：只有 output/。
	if err := os.MkdirAll(filepath.Join(root, "truyen-cu", "output", "novel"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 无关目录与隐藏目录被忽略。
	for _, d := range []string{"linh-tinh", ".git"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scan, err := lib.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Projects) != 2 || scan.Projects[0].ID != "du-an" || scan.Projects[1].ID != "_" || !scan.Projects[1].Legacy {
		t.Fatalf("projects: %+v", scan.Projects)
	}
	if len(scan.Candidates) != 1 || scan.Candidates[0] != "cu-dir" {
		t.Fatalf("candidates: %v", scan.Candidates)
	}
	if len(scan.Legacy) != 1 || scan.Legacy[0] != "truyen-cu" {
		t.Fatalf("legacy: %v", scan.Legacy)
	}
}

func TestResolveStory(t *testing.T) {
	lib, root := openTemp(t)
	mustWrite(t, filepath.Join(root, "du-an", "project.json"), `{}`)
	sdir := filepath.Join(root, "du-an", "kich_ban", "01-mot")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "truyen-cu", "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, ok := lib.ResolveStory("du-an", "01-mot")
	if !ok || st.ID != "du-an/01-mot" || st.Dir != sdir || st.ProjectDir != filepath.Join(root, "du-an") {
		t.Fatalf("story: %+v ok=%v", st, ok)
	}
	st, ok = lib.ResolveStory("_", "truyen-cu")
	if !ok || st.ID != "_/truyen-cu" || st.ProjectDir != root {
		t.Fatalf("legacy story: %+v ok=%v", st, ok)
	}
	for _, bad := range [][2]string{{"du-an", ".."}, {"du-an", ".git"}, {"du-an", "missing"}, {"/etc", "x"}, {"du-an", "a/b"}} {
		if _, ok := lib.ResolveStory(bad[0], bad[1]); ok {
			t.Fatalf("resolved %v", bad)
		}
	}
	// 候选目录里的故事在收养前不可解析……不，解析按目录结构走：
	// kich_ban 子目录仍被视为该目录下的故事（收养由 requireProject 把关）。
	if err := os.MkdirAll(filepath.Join(root, "cu-dir", "kich_ban", "02-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := lib.ResolveStory("cu-dir", "02-x"); !ok {
		t.Fatal("candidate story should resolve structurally")
	}
}

func TestCreateProjectAndAdopt(t *testing.T) {
	lib, root := openTemp(t)
	if _, err := lib.CreateProject("Bad Slug", ProjectFile{}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("bad slug: %v", err)
	}
	dir, err := lib.CreateProject("du-an", ProjectFile{Name: "Dự Án"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.CreateProject("du-an", ProjectFile{}); !errors.Is(err, ErrExists) {
		t.Fatalf("dup: %v", err)
	}
	pf, err := LoadProjectFile(dir)
	if err != nil || pf == nil || pf.Name != "Dự Án" || pf.CreatedAt == nil {
		t.Fatalf("project.json: %+v %v", pf, err)
	}
	for _, sub := range []string{".ainovel/rules", "y-tuong", "tai-san", "kich_ban"} {
		if !isDir(filepath.Join(dir, sub)) {
			t.Fatalf("missing %s", sub)
		}
	}
	// Adopt：必须是候选。
	if err := os.MkdirAll(filepath.Join(root, "cu-dir", "kich_ban", "01-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.AdoptProject("truyen-cu", ""); !errors.Is(err, ErrNotCandidate) {
		t.Fatalf("adopt non-candidate: %v", err)
	}
	dir, err = lib.AdoptProject("cu-dir", "Tên Đẹp")
	if err != nil {
		t.Fatal(err)
	}
	pf, _ = LoadProjectFile(dir)
	if pf == nil || pf.Name != "Tên Đẹp" {
		t.Fatalf("adopted project.json: %+v", pf)
	}
	for _, sub := range []string{".ainovel/rules", "y-tuong", "tai-san"} {
		if !isDir(filepath.Join(dir, sub)) {
			t.Fatalf("adopt missing %s", sub)
		}
	}
	// 收养后不再是候选，故事照常列出。
	scan, _ := lib.Scan()
	for _, c := range scan.Candidates {
		if c == "cu-dir" {
			t.Fatal("adopted dir still candidate")
		}
	}
	stories, _ := lib.Stories("cu-dir")
	if len(stories) != 1 || stories[0].Name != "01-x" {
		t.Fatalf("stories after adopt: %+v", stories)
	}
}

func TestCreateStoryNumberingAndTempName(t *testing.T) {
	lib, root := openTemp(t)
	dir, err := lib.CreateProject("du-an", ProjectFile{})
	if err != nil {
		t.Fatal(err)
	}
	st, temp, err := lib.CreateStory("du-an", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Story != "01-truyen-moi" || !temp || st.ID != "du-an/01-truyen-moi" {
		t.Fatalf("first story: %+v temp=%v", st, temp)
	}
	st2, temp2, err := lib.CreateStory("du-an", "Truyện Kể Dài", "")
	if err != nil {
		t.Fatal(err)
	}
	if st2.Story != "02-truyen-ke-dai" || temp2 {
		t.Fatalf("second story: %+v temp=%v", st2, temp2)
	}
	if _, _, err := lib.CreateStory("du-an", "Truyện kể dài", ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "02-truyen-ke-dai") {
		t.Fatalf("duplicate named story: %v", err)
	}
	if again, temp, err := lib.CreateStory("du-an", "", ""); err != nil || again.Story != "03-truyen-moi" || !temp {
		t.Fatalf("temporary names may repeat: %+v temp=%v err=%v", again, temp, err)
	}
	// 预先存在的更大编号会被跳过。
	if err := os.MkdirAll(filepath.Join(dir, "kich_ban", "07-co-san"), 0o755); err != nil {
		t.Fatal(err)
	}
	st3, _, err := lib.CreateStory("du-an", "Ba", "")
	if err != nil {
		t.Fatal(err)
	}
	if st3.Story != "08-ba" {
		t.Fatalf("numbering: %s", st3.Story)
	}
	sf, err := LoadStoryFile(st3.Dir)
	if err != nil || sf == nil || sf.Title != "Ba" || sf.Status != "idea" || sf.CreatedAt == nil {
		t.Fatalf("story.json: %+v %v", sf, err)
	}
	// 匿名项目：目录直接落在根下且可再被扫描识别。
	st4, _, err := lib.CreateStory("_", "Root Story", "")
	if err != nil {
		t.Fatal(err)
	}
	if st4.Story != "01-root-story" || st4.Dir != filepath.Join(root, st4.Story) {
		t.Fatalf("legacy create: %+v", st4)
	}
	if _, ok := lib.ResolveStory("_", st4.Story); !ok {
		t.Fatal("new legacy story not resolvable")
	}
}

func TestRenameStory(t *testing.T) {
	lib, _ := openTemp(t)
	if _, err := lib.CreateProject("du-an", ProjectFile{}); err != nil {
		t.Fatal(err)
	}
	st, _, err := lib.CreateStory("du-an", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.RenameStory("du-an", st.Story, "bad name"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("bad slug: %v", err)
	}
	renamed, err := lib.RenameStory("du-an", st.Story, "ten-moi")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Story != "01-ten-moi" || renamed.ID != "du-an/01-ten-moi" {
		t.Fatalf("renamed: %+v", renamed)
	}
	if _, ok := lib.ResolveStory("du-an", "01-truyen-moi"); ok {
		t.Fatal("old name still resolves")
	}
	if _, err := lib.RenameStory("du-an", "khong-co", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// 目标 "NN-slug" 已存在 → ErrExists（同名不同后缀的两个 02- 目录手工构造）。
	mustWrite(t, filepath.Join(lib.Root, "du-an", "kich_ban", "02-chap-giu", "story.json"), `{}`)
	mustWrite(t, filepath.Join(lib.Root, "du-an", "kich_ban", "02-ten-moi", "story.json"), `{}`)
	if _, err := lib.RenameStory("du-an", "02-chap-giu", "ten-moi"); !errors.Is(err, ErrExists) {
		t.Fatalf("exists: %v", err)
	}
	// 无编号的旧目录补上下一个编号（根下无数字目录 → 01）。
	if err := os.MkdirAll(filepath.Join(lib.Root, "truyen-cu", "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	renamed, err = lib.RenameStory("_", "truyen-cu", "ten-cu-moi")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Story != "01-ten-cu-moi" {
		t.Fatalf("legacy rename: %s", renamed.Story)
	}
}

func TestIdeasLifecycle(t *testing.T) {
	lib, _ := openTemp(t)
	if _, err := lib.CreateProject("du-an", ProjectFile{}); err != nil {
		t.Fatal(err)
	}
	idea, err := lib.CreateIdea("du-an", "Ý tưởng đầu tiên\n\nNội dung chi tiết ở đây.", "")
	if err != nil {
		t.Fatal(err)
	}
	if idea.ID == "" || idea.Title != "Ý tưởng đầu tiên" || !strings.Contains(idea.Excerpt, "Nội dung") || idea.CreatedAt == nil {
		t.Fatalf("idea: %+v", idea)
	}
	got, text, err := lib.GetIdea("du-an", idea.ID)
	if err != nil || got.ID != idea.ID || !strings.Contains(text, "Nội dung chi tiết") {
		t.Fatalf("get: %+v", got)
	}
	// 引用追踪：故事 story.json.idea 指到该想法。
	st, _, err := lib.CreateStory("du-an", "Một", idea.ID)
	if err != nil {
		t.Fatal(err)
	}
	ideas, err := lib.ListIdeas("du-an")
	if err != nil {
		t.Fatal(err)
	}
	if len(ideas) != 1 || len(ideas[0].UsedBy) != 1 || ideas[0].UsedBy[0] != st.ID {
		t.Fatalf("usedBy: %+v", ideas)
	}
	if _, _, err := lib.GetIdea("du-an", "../escape"); err == nil {
		t.Fatal("traversal allowed")
	}
	if _, _, err := lib.GetIdea("du-an", "khong-co"); !errors.Is(err, ErrIdeaNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// "_" 不支持想法。
	if _, err := lib.CreateIdea("_", "x", ""); !errors.Is(err, ErrLegacy) {
		t.Fatalf("legacy idea: %v", err)
	}
}

func TestAssets(t *testing.T) {
	lib, _ := openTemp(t)
	dir, err := lib.CreateProject("du-an", ProjectFile{})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "tai-san", "anh", "bia.png"), "PNG")
	mustWrite(t, filepath.Join(dir, "tai-san", "loi.txt"), "text")
	assets, err := lib.Assets("du-an")
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, a := range assets {
		paths[a.Path] = a.Kind
	}
	if paths["anh"] != "dir" || paths["anh/bia.png"] != "file" || paths["loi.txt"] != "file" || len(assets) != 3 {
		t.Fatalf("assets: %+v", assets)
	}
	if a, err := lib.Assets("_"); err != nil || len(a) != 0 {
		t.Fatalf("legacy assets: %v %v", a, err)
	}
}

func TestValidateAndFix(t *testing.T) {
	lib, root := openTemp(t)
	// 收养一个手工搭的“半旧”项目：候选 → 收养后再制造问题太绕，直接搭项目目录。
	dir := filepath.Join(root, "dit-dang")
	mustWrite(t, filepath.Join(dir, "project.json"), `{"name":"Dit Dang"}`)
	mustWrite(t, filepath.Join(dir, "_chung", "van-phong.md"), "# quy tắc chung")
	story := filepath.Join(dir, "kich_ban", "01-tra-xanh")
	if err := os.MkdirAll(filepath.Join(story, ".ainovel", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(story, "output", "novel"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "..", "_chung", "van-phong.md"), filepath.Join(story, ".ainovel", "rules", "van-phong.md")); err != nil {
		t.Fatal(err)
	}
	v, err := lib.Validate("dit-dang")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, i := range v.Issues {
		codes[i.Code] = true
	}
	if !codes[issueLegacyRules] || !codes[issueRulesSymlink] || !codes[issueMissingDir] || !codes[issueMissingStoryJSON] {
		t.Fatalf("issues: %+v", v.Issues)
	}
	if v.OK || v.Tree == nil {
		t.Fatal("validation should fail with issues and include a tree")
	}
	v2, err := lib.Fix("dit-dang", nil)
	if err != nil {
		t.Fatal(err)
	}
	// _chung 文件移入项目 .ainovel/rules；符号链接删除。
	if !hasFile(filepath.Join(dir, ".ainovel", "rules"), "van-phong.md") {
		t.Fatal("_chung rule not moved")
	}
	if _, err := os.Lstat(filepath.Join(story, ".ainovel", "rules", "van-phong.md")); !os.IsNotExist(err) {
		t.Fatal("rules symlink not removed")
	}
	if !hasFile(story, "story.json") {
		t.Fatal("story.json not created")
	}
	// 标题留空，StorySummary 才能回退到引擎 meta 标题（不把目录 slug 当标题）。
	if f, err := LoadStoryFile(story); err != nil || f == nil || f.Title != "" {
		t.Fatalf("story.json 标题应为空: %+v %v", f, err)
	}
	for _, sub := range []string{"y-tuong", "tai-san"} {
		if !isDir(filepath.Join(dir, sub)) {
			t.Fatalf("missing dir not created: %s", sub)
		}
	}
	remaining := map[string]bool{}
	for _, i := range v2.Issues {
		remaining[i.Code] = true
	}
	if remaining[issueLegacyRules] || remaining[issueRulesSymlink] || remaining[issueMissingDir] || remaining[issueMissingStoryJSON] {
		t.Fatalf("unfixed issues: %+v", v2.Issues)
	}
	// 只修指定 code：再来一遍验证 codes 过滤。
	v3, err := lib.Fix("dit-dang", []string{"missing-dir"})
	if err != nil {
		t.Fatal(err)
	}
	_ = v3
}

func TestLocked(t *testing.T) {
	dir := t.TempDir()
	if Locked(dir) {
		t.Fatal("unheld dir reported locked")
	}
	fl := flock.New(filepath.Join(dir, lockFileName))
	held, err := fl.TryLock()
	if err != nil || !held {
		t.Fatalf("hold lock: %v", err)
	}
	defer fl.Close()
	if !Locked(dir) {
		t.Fatal("held dir not reported locked")
	}
	if err := fl.Close(); err != nil {
		t.Fatal(err)
	}
	if Locked(dir) {
		t.Fatal("released dir still locked")
	}
}

func TestValidateRulesDirSymlink(t *testing.T) {
	lib, root := openTemp(t)
	dir := filepath.Join(root, "du-an")
	mustWrite(t, filepath.Join(dir, "project.json"), `{"name":"Dự Án"}`)
	mustWrite(t, filepath.Join(dir, ".ainovel", "rules", "van-phong.md"), "# quy tắc dự án")
	// 规则目录里指向项目其他位置的符号链接（不指向共享规则）：不报问题，但 kind 要标成 dir。
	if err := os.MkdirAll(filepath.Join(dir, "tai-san", "mau"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "tai-san", "mau"), filepath.Join(dir, ".ainovel", "rules", "mau")); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"y-tuong", "tai-san"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	story := filepath.Join(dir, "kich_ban", "01-tra-xanh")
	if err := os.MkdirAll(filepath.Join(story, ".ainovel"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(story, "story.json"), `{"title":"Trà Xanh","status":"writing"}`)
	// 故事规则目录整目录符号链接到项目规则（旧版共享规则布局）。
	if err := os.Symlink(filepath.Join("..", "..", "..", ".ainovel", "rules"), filepath.Join(story, ".ainovel", "rules")); err != nil {
		t.Fatal(err)
	}
	v, err := lib.Validate("du-an")
	if err != nil {
		t.Fatal(err)
	}
	var symlinkIssue bool
	for _, i := range v.Issues {
		if i.Code == issueRulesSymlink && i.Path == "kich_ban/01-tra-xanh/.ainovel/rules" {
			symlinkIssue = true
		}
	}
	if !symlinkIssue {
		t.Fatalf("故事规则目录符号链接未报告: %+v", v.Issues)
	}
	// 符号链接目录不能展开：否则树里会混入项目级规则，误导“删除”到项目文件。
	if n := findNode(v.Tree, "kich_ban/01-tra-xanh/.ainovel/rules"); n == nil || len(n.Children) != 0 {
		t.Fatalf("符号链接规则目录被展开: %+v", n)
	}
	// 指向项目内其他位置的符号链接目录：kind 必须是 dir（不是 file）。
	if n := findNode(v.Tree, ".ainovel/rules/mau"); n == nil || n.Kind != "dir" || n.Status != "optional" {
		t.Fatalf("symlink dir node: %+v", n)
	}
	if _, err := lib.Fix("du-an", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(story, ".ainovel", "rules")); !os.IsNotExist(err) {
		t.Fatalf("规则目录符号链接未删除: %v", err)
	}
	// 项目规则必须完好（不能被链接删除波及）。
	if got, err := os.ReadFile(filepath.Join(dir, ".ainovel", "rules", "van-phong.md")); err != nil || !strings.Contains(string(got), "quy tắc dự án") {
		t.Fatalf("项目规则被破坏: %q %v", got, err)
	}
}

func TestValidateFixRulesFileSymlink(t *testing.T) {
	lib, root := openTemp(t)
	dir := filepath.Join(root, "du-an")
	mustWrite(t, filepath.Join(dir, "project.json"), `{"name":"Dự Án"}`)
	mustWrite(t, filepath.Join(dir, "_chung", "van-phong.md"), "# quy tắc chung")
	for _, sub := range []string{"y-tuong", "tai-san"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	story := filepath.Join(dir, "kich_ban", "01-tra-xanh")
	mustWrite(t, filepath.Join(story, "story.json"), `{"title":"Trà Xanh","status":"writing"}`)
	// 项目规则文件符号链接到 _chung（旧版共享规则布局）。
	if err := os.MkdirAll(filepath.Join(dir, ".ainovel", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "_chung", "van-phong.md"), filepath.Join(dir, ".ainovel", "rules", "van-phong.md")); err != nil {
		t.Fatal(err)
	}
	v, err := lib.Validate("du-an")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, i := range v.Issues {
		codes[i.Code] = true
	}
	if !codes[issueLegacyRules] || !codes[issueRulesSymlink] {
		t.Fatalf("缺少 legacy-rules/rules-symlink: %+v", v.Issues)
	}
	if _, err := lib.Fix("du-an", nil); err != nil {
		t.Fatal(err)
	}
	// 结果应是真实文件（不是符号链接），内容来自 _chung。
	rulePath := filepath.Join(dir, ".ainovel", "rules", "van-phong.md")
	info, err := os.Lstat(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("符号链接未被替换为真实规则文件")
	}
	if got, _ := os.ReadFile(rulePath); !strings.Contains(string(got), "quy tắc chung") {
		t.Fatalf("规则内容丢失: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "_chung")); !os.IsNotExist(err) {
		t.Fatalf("_chung 未清理: %v", err)
	}
}

// findNode 在目录树里按相对路径找节点（测试用）。
func findNode(n *FolderNode, path string) *FolderNode {
	if n == nil {
		return nil
	}
	if n.Path == path {
		return n
	}
	for _, c := range n.Children {
		if got := findNode(c, path); got != nil {
			return got
		}
	}
	return nil
}

func TestCreateIdeaExplicitTitle(t *testing.T) {
	lib, _ := openTemp(t)
	if _, err := lib.CreateProject("du-an", ProjectFile{}); err != nil {
		t.Fatal(err)
	}
	// 正文自带一级标题时，显式标题必须胜出（契约：title 优先于首行）。
	idea, err := lib.CreateIdea("du-an", "# Ý tưởng cũ\n\nMột buổi chiều ở Đà Lạt.", "Hoàng hôn Đà Lạt")
	if err != nil {
		t.Fatal(err)
	}
	if idea.Title != "Hoàng hôn Đà Lạt" {
		t.Fatalf("explicit title lost: %q", idea.Title)
	}
	if strings.Contains(idea.Excerpt, "Ý tưởng cũ") || !strings.Contains(idea.Excerpt, "Đà Lạt") {
		t.Fatalf("excerpt: %q", idea.Excerpt)
	}
	detail, text, err := lib.GetIdea("du-an", idea.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Hoàng hôn Đà Lạt" || !strings.Contains(text, "Một buổi chiều") {
		t.Fatalf("detail: %+v %q", detail, text)
	}
	// 未被引用时 usedBy 是 []，不能是 null（契约类型 string[]）。
	if detail.UsedBy == nil || len(detail.UsedBy) != 0 {
		t.Fatalf("usedBy should be empty slice: %#v", detail.UsedBy)
	}
	// 首行标题（无显式 title）保持原行为。
	derived, err := lib.CreateIdea("du-an", "# Từ dòng đầu\n\nThân bài.", "")
	if err != nil {
		t.Fatal(err)
	}
	if derived.Title != "Từ dòng đầu" {
		t.Fatalf("derived title: %q", derived.Title)
	}
}

func TestValidateTreeHasSingleStoryJSON(t *testing.T) {
	lib, root := openTemp(t)
	dir := filepath.Join(root, "du-an")
	mustWrite(t, filepath.Join(dir, "project.json"), `{"name":"Dự Án"}`)
	mustWrite(t, filepath.Join(dir, "kich_ban", "01-a", "story.json"), `{"title":"A","status":"idea"}`)
	v, err := lib.Validate("du-an")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	var walk func(*FolderNode)
	walk = func(node *FolderNode) {
		if node.Path == "kich_ban/01-a/story.json" {
			n++
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(v.Tree)
	if n != 1 {
		t.Fatalf("story.json xuất hiện %d lần trong cây (phải 1)", n)
	}
}
