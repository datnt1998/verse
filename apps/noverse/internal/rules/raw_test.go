package rules

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRawFileSources_ScansAllMarkdownInOrder 验证目录下多个 .md 都被扫到，
// 按文件名字典序返回；非 .md 文件被忽略；原文原样保留。
func TestRawFileSources_ScansAllMarkdownInOrder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b.md", "# B 偏好")
	write("a.md", "# A 偏好")
	write("ignore.txt", "not a rule")
	write("empty.md", "   ") // 空白文件应跳过

	srcs := RawFileSources(LoadOptions{ProjectRulesDir: dir})
	if len(srcs) != 2 {
		t.Fatalf("应扫到 a.md / b.md 两个来源（.txt 与空白跳过），得到 %d：%+v", len(srcs), srcs)
	}
	// 字典序：a 在前 b 在后
	if srcs[0].Label != "project:a.md" || srcs[1].Label != "project:b.md" {
		t.Errorf("应按字典序返回，得到 %q, %q", srcs[0].Label, srcs[1].Label)
	}
	for _, s := range srcs {
		if s.Kind != SourceProject {
			t.Errorf("project directory kind = %v", s.Kind)
		}
	}
}

// TestRawFileSources_DirMissing 验证目录不存在时静默跳过（返回 nil）。
func TestRawFileSources_DirMissing(t *testing.T) {
	srcs := RawFileSources(LoadOptions{HomeRulesDir: filepath.Join(t.TempDir(), "nope")})
	if len(srcs) != 0 {
		t.Errorf("缺失目录应返回 0 来源，得到 %d", len(srcs))
	}
	if len(RawFileSources(LoadOptions{})) != 0 {
		t.Error("空 LoadOptions 应返回 0 来源")
	}
}

// TestRawFileSources_IgnoresHiddenAndSubdirs 锁死：隐藏/编辑器临时文件（. 开头）被忽略、
// 子目录不递归——防止脏文件二进制内容当偏好正文注入 LLM。
func TestRawFileSources_IgnoresHiddenAndSubdirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real.md"), []byte("# real"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"._real.md", ".#lock.md", ".hidden.md"} {
		if err := os.WriteFile(filepath.Join(dir, dirty), []byte("\x00binary garbage\x00"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "nested.md"), []byte("# nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	srcs := RawFileSources(LoadOptions{ProjectRulesDir: dir})
	if len(srcs) != 1 || srcs[0].Label != "project:real.md" {
		t.Fatalf("应只扫到 real.md（隐藏/脏/子目录忽略），得到 %+v", srcs)
	}
}

// Legacy global inputs are ignored even when explicitly supplied.
func TestRawFileSources_IgnoresGlobal(t *testing.T) {
	base := t.TempDir()
	global := filepath.Join(base, "global")
	project := filepath.Join(base, "project")
	for _, d := range []string{global, project} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(global, "g.md"), []byte("# 全局"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "p.md"), []byte("# 本书"), 0o644); err != nil {
		t.Fatal(err)
	}

	srcs := RawFileSources(LoadOptions{HomeRulesDir: global, ProjectRulesDir: project, Sources: []RawSource{{Kind: SourceGlobal, Name: "stored.md", Text: "stored global"}}})
	if len(srcs) != 1 || srcs[0].Kind != SourceProject {
		t.Fatalf("only project rules may load, got %+v", srcs)
	}
}
