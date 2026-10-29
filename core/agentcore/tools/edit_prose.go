package tools

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NewProseEdit uses only mechanical equivalences, never fuzzy block anchors
// that could overwrite text changed since the writer last read a chapter.
func NewProseEdit(workDir string) *EditTool {
	t := NewEdit(workDir, nil)
	t.prose = true
	return t
}

type proseText struct {
	text         string
	starts, ends []int // normalized byte -> original byte span
}

func normalizeProse(text string, mapBytes bool) proseText {
	var out strings.Builder
	out.Grow(len(text))
	var mapped proseText
	if mapBytes {
		mapped.starts = make([]int, 0, len(text))
		mapped.ends = make([]int, 0, len(text))
	}
	var iter norm.Iter
	iter.InitString(norm.NFC, text)
	space := false
	for !iter.Done() {
		start := iter.Pos()
		segment := string(iter.Next())
		end := iter.Pos()
		for _, r := range segment {
			if r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\u2060' || r == '\ufeff' {
				continue
			}
			if unicode.IsSpace(r) {
				if space {
					if mapBytes {
						mapped.ends[len(mapped.ends)-1] = end
					}
					continue
				}
				r = ' '
				space = true
			} else {
				space = false
			}
			s := string(r)
			switch r {
			case '“', '”', '„', '‟':
				s = "\""
			case '‘', '’', '‚', '‛':
				s = "'"
			case '…':
				s = "..."
			}
			out.WriteString(s)
			if mapBytes {
				for range len(s) {
					mapped.starts = append(mapped.starts, start)
					mapped.ends = append(mapped.ends, end)
				}
			}
		}
	}
	mapped.text = out.String()
	return mapped
}

type proseSpan struct{ start, end int }

func proseMatches(content, old string) []proseSpan {
	actual := normalizeProse(content, true)
	// Horizontal boundary padding is not part of the prose anchor. Keep
	// explicit newlines, so replacements that include them retain their range.
	needle := normalizeProse(strings.Trim(old, " \t\u00a0"), false).text
	if needle == "" {
		return nil
	}
	var spans []proseSpan
	for offset := 0; offset <= len(actual.text)-len(needle); {
		i := strings.Index(actual.text[offset:], needle)
		if i < 0 {
			break
		}
		i += offset
		last := i + len(needle) - 1
		// Do not match part of an NFC segment, UTF-8 rune or expanded ellipsis.
		if (i == 0 || actual.starts[i] != actual.starts[i-1]) && (last+1 == len(actual.text) || actual.ends[last] != actual.ends[last+1]) {
			spans = append(spans, proseSpan{actual.starts[i], actual.ends[last]})
		}
		offset = i + 1
	}
	return spans
}

func matchProseEdit(path, bom, content, old, replacement string, all bool) (*editResult, error) {
	spans := proseMatches(content, old)
	if len(spans) == 0 && strings.Contains(old, `\n`) {
		// A tool result is JSON: models sometimes copy its escaped newlines.
		decoded := strings.NewReplacer(`\r\n`, "\n", `\n`, "\n", `\r`, "\n").Replace(old)
		spans = proseMatches(content, decoded)
	}
	if len(spans) == 0 {
		window := formatEditCandidates(content, old)
		if window == "" {
			lines := strings.Split(normalizeToLF(content), "\n")
			if len(lines) > 12 {
				lines = lines[:12]
			}
			window = "```text\n" + strings.Join(lines, "\n") + "\n```"
		}
		return nil, fmt.Errorf("old_string không còn khớp bản nháp; văn bản có thể đã thay đổi hoặc được diễn đạt lại. Hãy read_chapter(source=\"draft\") và chép nguyên văn đoạn hiện tại trước khi thử lại. Bản nháp hiện tại (các đoạn gần nhất):\n%s", window)
	}
	if len(spans) > 1 && !all {
		return nil, fmt.Errorf("found %d mechanically equivalent occurrences in %s; provide more context or use replace_all=true", len(spans), path)
	}
	var out strings.Builder
	cursor := 0
	for _, span := range spans {
		if span.start < cursor {
			return nil, fmt.Errorf("overlapping matches in %s; provide more context", path)
		}
		out.WriteString(content[cursor:span.start])
		out.WriteString(replacement)
		cursor = span.end
	}
	out.WriteString(content[cursor:])
	updated := out.String()
	if updated == content {
		return nil, fmt.Errorf("old_string and new_string are identical in %s", path)
	}
	// Preserve untouched bytes verbatim, including mixed line endings and NFD.
	return &editResult{path: path, bom: bom, ending: "\n", oldContent: content, newContent: updated}, nil
}

// ProseEditResult is a mechanical prose replacement applied in memory: the new
// content plus the unified diff the tool result reports.
type ProseEditResult struct {
	Content   string
	Diff      string
	FirstLine int
}

// ApplyProseEdit applies the exact matching rules of the prose edit tool to a
// string without touching the filesystem. Engine tools that own chapter text in
// a store call this and persist through their backend, so no chapter write is
// ever routed through the workspace editor.
func ApplyProseEdit(content, old, replacement string, all bool) (ProseEditResult, error) {
	r, err := matchProseEdit("draft", "", content, old, replacement, all)
	if err != nil {
		return ProseEditResult{}, err
	}
	diff, first := generateDiff(r.oldContent, r.newContent)
	return ProseEditResult{Content: r.newContent, Diff: diff, FirstLine: first}, nil
}
