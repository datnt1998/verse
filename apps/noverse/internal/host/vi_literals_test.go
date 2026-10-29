package host

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// cjkLiteralAllow 列出允许含汉字的字符串字面量：只能是读旧数据的兼容表/解析器，
// 不是展示给用户的文案。键为相对 internal/ 的文件路径；值为 nil 表示整文件都是兼容表。
var cjkLiteralAllow = map[string][]string{
	"host/event_legacy.go":            nil,                        // 旧 tui.log 中文事件 → 越南语
	"host/user_events.go":             {"[继续]", "[用户干预]", "[干预]"}, // 旧用户事件前缀 → command
	"tools/premise_structure.go":      nil,                        // 旧 premise 中文标题
	"agents/guard/subagent_guards.go": {"卷摘要", "弧摘要"},
	"host/resume.go":                  {"核心冲突"},
	"entry/tui/panels_sidebar.go":     {"待命"},
	"domain/chapter_heading.go":       {`^\s*(?i:(chương|chapter)\s*(\d+)|第\s*(\d+)\s*章)`},
	"entry/web/activity_history.go":   {`(?i)(?:chương|chapter|第)\s*(\d+)`},
	"host/exp/txt.go":                 {`^#+\s+((?i:chương|chapter)\s*\d+|第.+?章)`},
	"host/imp/preview.go":             {`^\s*(?i:(chương|chapter)\s*(\d+)|第\s*[零〇一二三四五六七八九十百千万两\d]+\s*章)`, "Không tìm thấy tiêu đề chương (Chương N / Chapter N / 第N章); khi nhập, mô hình sẽ phân tách theo ngữ nghĩa"},
	"store/session.go":                {`(?i)(?:chương|chapter)\s*(\d+)|第\s*(\d+)\s*章`},
	"stylestat/stylestat.go":          {`(?i)^#{0,2}\s*(?:chương\s*\d+|chapter\s*\d+|第\s*[零〇一二三四五六七八九十百千万\d]+\s*章)`},
}

// scanStringLiterals 扫描 internal/ 与 cmd/ 下所有非测试 Go 文件的字符串字面量（注释不算），
// 对匹配 re 的字面量，若不在 allow 白名单里就报错。键为相对 internal/ 的文件路径；
// 值为 nil 表示整文件都是兼容表。
func scanStringLiterals(t *testing.T, re *regexp.Regexp, allow map[string][]string, advice string) {
	t.Helper()
	for _, root := range []string{"..", filepath.Join("..", "..", "cmd")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel("..", path)
			rel = filepath.ToSlash(rel)
			allowed, whole := allow[rel]
			if whole && allowed == nil {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING || !re.MatchString(lit.Value) {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					value = lit.Value
				}
				for _, a := range allowed {
					if a == value {
						return true
					}
				}
				t.Errorf("%s: %s: %s", fset.Position(lit.Pos()), advice, lit.Value)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestNoCJKUserFacingLiterals 守住"所有到达用户的 Go 字符串都是越南语"：含汉字者必须在兼容白名单里。
func TestNoCJKUserFacingLiterals(t *testing.T) {
	scanStringLiterals(t, regexp.MustCompile(`[\x{4e00}-\x{9fff}]`), cjkLiteralAllow,
		"chuỗi tiếng Trung tới người dùng, hãy dịch sang tiếng Việt (hoặc ghi vào cjkLiteralAllow nếu là bảng tương thích)")
}

// fullwidthLiteralAllow 是在 cjkLiteralAllow 之外，允许含全角/CJK 标点的字面量：
// 只能是清理文本用的字符集，不是展示给用户的文案。
var fullwidthLiteralAllow = map[string][]string{
	"host/resume.go":         {"《》«»\""},            // 去掉书名号的字符集
	"rules/integrity.go":     {".!?…\"”’»)」』—~。！？"}, // 终稿合法收尾字符集
	"stylestat/stylestat.go": {"\"“”‘’「」『』"},        // 句子首尾引号字符集
}

// TestNoFullwidthPunctuationUserFacingLiterals 守住用户可见文案不夹带全角标点（U+3000–303F、
// U+FF00–FFEF，如 （ ） 〈 〉 『 』 【 】）：Hán tự đã có test trên, dấu toàn chiều thì phải dùng ASCII/Việt.
func TestNoFullwidthPunctuationUserFacingLiterals(t *testing.T) {
	allow := map[string][]string{}
	for k, v := range cjkLiteralAllow {
		allow[k] = v
	}
	for k, v := range fullwidthLiteralAllow {
		allow[k] = append(allow[k], v...)
	}
	scanStringLiterals(t, regexp.MustCompile(`[\x{3000}-\x{303F}\x{FF00}-\x{FFEF}]`), allow,
		"dấu câu toàn chiều trong chuỗi người dùng thấy, dùng dấu ASCII/tiếng Việt (hoặc ghi vào fullwidthLiteralAllow nếu là bộ ký tự dọn văn bản)")
}
