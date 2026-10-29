package host

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// RootRelativeText 把文本里的绝对路径改写成相对根目录的形式，事件/日志文案不带服务器绝对路径
// （11-api-contract：Nhật ký、lỗi xuất không chứa đường dẫn tuyệt đối máy chủ）。
//
//   - 位于某个 root 之下 → 相对该 root（"<root>/exports/a.epub" → "exports/a.epub"）；
//     恰为 root 本身 → root 的目录名。
//   - 其余位于用户主目录之下 → "~/…"。
//
// roots 依次尝试原样与解析符号链接后的形式，较长（更具体）的前缀优先。旧版日志里
// "~" 紧贴绝对路径（"~/Users/…"）的错误写法一并归一。
func RootRelativeText(text string, roots ...string) string {
	if !strings.Contains(text, string(filepath.Separator)) {
		return text
	}
	type rule struct {
		prefix string
		home   bool
	}
	var rules []rule
	seen := map[string]bool{}
	add := func(p string, home bool) {
		for _, v := range pathVariants(p) {
			if !seen[v] {
				seen[v] = true
				rules = append(rules, rule{v, home})
			}
		}
	}
	for _, root := range roots {
		add(root, false)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home, true)
	}
	sort.SliceStable(rules, func(i, j int) bool { return len(rules[i].prefix) > len(rules[j].prefix) })
	for _, r := range rules {
		text = replacePathPrefix(text, r.prefix, r.home)
	}
	return text
}

// pathVariants 返回可用于匹配的绝对路径形式（清理后 + 符号链接解析后）；非绝对路径、根目录不参与。
func pathVariants(p string) []string {
	p = strings.TrimSpace(p)
	if p == "" || !filepath.IsAbs(p) {
		return nil
	}
	clean := filepath.Clean(p)
	if clean == string(filepath.Separator) || clean == filepath.VolumeName(clean)+string(filepath.Separator) {
		return nil
	}
	out := []string{clean}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != clean {
		out = append(out, resolved)
	}
	return out
}

func replacePathPrefix(text, prefix string, home bool) string {
	sep := string(filepath.Separator)
	var b strings.Builder
	for {
		i := strings.Index(text, prefix)
		if i < 0 {
			break
		}
		head, rest := text[:i], text[i+len(prefix):]
		// 前缀须从路径起点开始：前一个字符若是名字字符，说明只是更长路径的一部分。
		if before, _ := utf8.DecodeLastRuneInString(head); i > 0 && before != '~' && isPathNameRune(before) {
			b.WriteString(text[:i+len(prefix)])
			text = rest
			continue
		}
		head = strings.TrimSuffix(head, "~")
		switch {
		case strings.HasPrefix(rest, sep):
			b.WriteString(head)
			if home {
				b.WriteString("~" + sep)
			}
			text = rest[len(sep):]
		case rest == "" || !startsWithPathNameRune(rest):
			b.WriteString(head)
			if home {
				b.WriteString("~")
			} else {
				b.WriteString(filepath.Base(prefix))
			}
			text = rest
		default:
			// "<root>.tmp-1" 之类：不是该 root 下的路径，原样保留。
			b.WriteString(text[:i+len(prefix)])
			text = rest
		}
	}
	b.WriteString(text)
	return b.String()
}

func isPathNameRune(r rune) bool {
	return r == '.' || r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func startsWithPathNameRune(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return isPathNameRune(r)
}

// MapEventText 对事件所有展示文本字段（摘要、详情、工具标签、参数/结果、重试错误）应用 fn，
// 供路径改写这类与语言无关的处理使用。
func MapEventText(ev Event, fn func(string) string) Event {
	ev = mapEventMessages(ev, fn)
	ev.Args, ev.Result = fn(ev.Args), fn(ev.Result)
	return ev
}

// mapEventMessages 对事件的提示文案字段（摘要、详情、工具标签、重试错误）应用 fn；
// 工具参数/结果是原始 I/O，由调用方决定是否处理。
func mapEventMessages(ev Event, fn func(string) string) Event {
	ev.Summary, ev.Detail, ev.Tool = fn(ev.Summary), fn(ev.Detail), fn(ev.Tool)
	if len(ev.Retries) > 0 {
		retries := make([]EventRetry, len(ev.Retries))
		for i, r := range ev.Retries {
			r.Error = fn(r.Error)
			retries[i] = r
		}
		ev.Retries = retries
	}
	return ev
}

// logRoot 是事件日志相对化的根：TUI 与 web worker 都以故事目录为工作目录运行。
var logRoot = sync.OnceValue(func() string {
	wd, _ := os.Getwd()
	return wd
})
