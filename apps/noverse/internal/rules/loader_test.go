package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureRulesDirAt 验证备好目录 + README.txt：写入说明、始终覆盖为最新模板，
// 且 README.txt（非 .md）不会被扫描当成规则。
func TestEnsureRulesDirAt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	if err := ensureRulesDirAt(dir); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(dir, "README.txt")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("README.txt should be written: %v", err)
	}
	// 砍 YAML 后引导改讲"大白话 + 自动归一化"，不再教 front matter。
	if !strings.Contains(string(data), "chuẩn hoá") {
		t.Errorf("README.txt phải nói rõ yêu cầu ngôn ngữ tự nhiên sẽ được chuẩn hoá, got %q", data)
	}
	if strings.Contains(string(data), "front matter") {
		t.Errorf("README.txt không được dạy YAML front matter nữa, got %q", data)
	}

	// 始终覆盖为最新模板：旧版本写的过时文案再次 ensure 时被刷新
	if err := os.WriteFile(readme, []byte("旧版本写的过时文案"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureRulesDirAt(dir); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(readme); string(again) != homeRulesReadme {
		t.Errorf("README.txt should be refreshed to latest template, got %q", again)
	}

	// README.txt 不被当规则（扫描只认 .md）
	if srcs := RawFileSources(LoadOptions{HomeRulesDir: dir}); len(srcs) != 0 {
		t.Errorf("README.txt must not be scanned as a rule, got %d sources", len(srcs))
	}
}

// TestDefaultProjectRulesDir 锁死项目级规则目录镜像全局：./.ainovel/rules/。
func TestDefaultProjectRulesDir(t *testing.T) {
	proj := filepath.Join("/tmp", "demo-book")
	want := filepath.Join(proj, ".ainovel", "rules")
	if got := DefaultProjectRulesDir(proj); got != want {
		t.Errorf("DefaultProjectRulesDir=%q, want %q", got, want)
	}
	if got := DefaultProjectRulesDir(""); got != "" {
		t.Errorf("空项目根应返回空串，得到 %q", got)
	}
}

// TestDefaultOptions_ScansStoryRulesFromDotAinovel 端到端验证：
// DefaultOptions 把 cwd 下的 ./.ainovel/rules/ 接进最近层来源
// （无 project.json 祖先时该层为 SourceStory）。
func TestDefaultOptions_ScansStoryRulesFromDotAinovel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	story := t.TempDir()
	t.Chdir(story)
	rulesDir := filepath.Join(story, ".ainovel", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "book.md"), []byte("# Sở thích truyện này\nmỗi chương 4000 chữ"), 0o644); err != nil {
		t.Fatal(err)
	}

	srcs := RawFileSources(DefaultOptions())
	var got *RawSource
	for i := range srcs {
		if srcs[i].Kind == SourceStory {
			got = &srcs[i]
		}
	}
	if got == nil {
		t.Fatalf("应从 ./.ainovel/rules/ 扫到故事规则来源，得到 %+v", srcs)
	}
	if !strings.Contains(got.Text, "Sở thích truyện này") {
		t.Errorf("故事规则原文应被原样返回，得到 %q", got.Text)
	}
	if got.Label != "story:book.md" {
		t.Errorf("来源标签应为 story:book.md，得到 %q", got.Label)
	}
}

// Project rules precede story rules; global rules are not part of the chain.
func TestRawFileSources_ProjectStoryChain(t *testing.T) {
	global := t.TempDir()
	project := t.TempDir()
	story := t.TempDir()
	write := func(dir, name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(global, "shared.md", "global shared")
	write(global, "only-global.md", "global only")
	write(project, "shared.md", "project shared")
	write(project, "only-project.md", "project only")
	write(story, "shared.md", "story shared")
	write(story, "only-story.md", "story only")

	srcs := RawFileSources(LoadOptions{
		HomeRulesDir: global, ProjectRulesDir: project, StoryRulesDir: story,
	})
	if len(srcs) != 3 {
		t.Fatalf("want 3 project/story sources, got %d: %+v", len(srcs), srcs)
	}
	var shared *RawSource
	for i := range srcs {
		if srcs[i].Name == "shared.md" {
			shared = &srcs[i]
		}
	}
	if shared == nil || shared.Kind != SourceStory || shared.Text != "story shared" {
		t.Fatalf("同名文件应由最近层（story）胜出，得到 %+v", srcs)
	}
	kinds := []SourceKind{srcs[0].Kind, srcs[1].Kind, srcs[2].Kind}
	want := []SourceKind{SourceProject, SourceStory, SourceStory}
	for i := range kinds {
		if kinds[i] != want[i] {
			t.Fatalf("want project→story, got %v", kinds)
		}
	}
}

// TestOptionsForDir_ProjectAncestor 验证 OptionsForDir 把最近的 project.json
// 祖先目录接为项目层、cwd 为故事层。
func TestOptionsForDir_ProjectAncestor(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	story := filepath.Join(project, "kich_ban", "01-truyen")
	if err := os.MkdirAll(story, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{"name":"p"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := OptionsForDir(story)
	if got, want := opts.ProjectRulesDir, filepath.Join(project, ".ainovel", "rules"); got != want {
		t.Errorf("ProjectRulesDir=%q, want %q", got, want)
	}
	if got, want := opts.StoryRulesDir, filepath.Join(story, ".ainovel", "rules"); got != want {
		t.Errorf("StoryRulesDir=%q, want %q", got, want)
	}
}
