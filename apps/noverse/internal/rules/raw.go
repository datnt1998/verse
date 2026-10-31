package rules

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RawSource 是一个待归一化的原始来源（rules 文件的整段文本）。
//
// 砍 YAML 后，rules 文件就是普通自然语言提示词；归一化只需要原文，不再做 front matter 解析。
type RawSource struct {
	Label string     // 来源标签，进入 Snapshot.Sources（如 global:my-style.md）
	Name  string     // 文件名（去重键：多层同名时只保留最近一层）
	Kind  SourceKind // 优先级层级
	Text  string     // 文件原始内容
}

// RawFileSources enumerates project → story rules; legacy global inputs are ignored.
//
// 与 readDirFromDisk 同样的扫描约定（顶层 .md、字典序、跳过隐藏文件），但不解析 YAML，
// 整段文本原样交给归一化器。同名文件在多层出现时只保留最近一层的版本（GAP-10：
// nearer wins），其余文件按名合并。ManagedRulesFile（quy-tac.md）是例外：各层都保留，
// 因为界面把它当作逐层累加的规则清单。System defaults / 启动 prompt / 运行中要求由 service 另行提供。
func RawFileSources(opts LoadOptions) []RawSource {
	var out []RawSource
	out = append(out, rawDir(opts.ProjectRulesDir, SourceProject)...)
	out = append(out, rawDir(opts.StoryRulesDir, SourceStory)...)
	for _, source := range opts.Sources {
		if source.Kind == SourceProject || source.Kind == SourceStory {
			out = append(out, source)
		}
	}
	// 同名文件就近覆盖：靠后的层（更近）胜出，保序输出仍按 全局→项目→故事。
	best := make(map[string]int, len(out))
	for i, s := range out {
		best[dedupeKey(s)] = i
	}
	var merged []RawSource
	for i, s := range out {
		if best[dedupeKey(s)] == i {
			merged = append(merged, s)
		}
	}
	return merged
}

// dedupeKey 是"同名就近覆盖"的键；ManagedRulesFile 带上层级使各层互不覆盖。
func dedupeKey(s RawSource) string {
	if s.Name == ManagedRulesFile {
		return s.Kind.String() + "/" + s.Name
	}
	return s.Name
}

func rawDir(dir string, kind SourceKind) []RawSource {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// 目录不存在是常态，静默跳过；但权限/路径其实是文件这类错误必须留痕——
		// 否则用户写了规则却完全没生效、零反馈，排查成本极高（见 known_rules_path_stale_readme）。
		if !os.IsNotExist(err) {
			slog.Warn("Đọc thư mục luật thất bại, đã bỏ qua", "module", "rules", "dir", dir, "err", err)
		}
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var out []RawSource
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("Đọc tệp luật thất bại, đã bỏ qua", "module", "rules", "file", path, "err", err)
			continue
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}
		out = append(out, RawSource{
			Label: kind.String() + ":" + name,
			Name:  name,
			Kind:  kind,
			Text:  text,
		})
	}
	return out
}
