package rules

import (
	"strings"
	"testing"
)

func TestLint_CleanText(t *testing.T) {
	if vs := Lint("# 第一章 风起\n他迈步向前。\n夜色渐深。"); len(vs) != 0 {
		t.Errorf("clean text should pass: %+v", vs)
	}
}

func TestLint_MarkdownResidue(t *testing.T) {
	text := "# 第一章\n这是**重点**内容。\n## 小标题\n正文。"
	vs := Lint(text)
	bold := findViolation(vs, "markdown_residue", "**")
	if bold == nil || bold.Actual != 2 {
		t.Errorf("expected ** residue x2: %+v", vs)
	}
	heading := findViolation(vs, "markdown_residue", "#")
	if heading == nil || heading.Actual != 1 {
		t.Errorf("expected 1 heading beyond first line: %+v", vs)
	}
}

func TestLint_NonCJKFragments(t *testing.T) {
	text := "# 第一章\n他发现了一个pattern，这个pattern像DNA一样规律。"
	vs := Lint(text)
	var v *Violation
	for i := range vs {
		if vs[i].Rule == "non_cjk_fragments" {
			v = &vs[i]
			break
		}
	}
	if v == nil {
		t.Fatalf("expected non_cjk violation: %+v", vs)
	}
	if v.Actual != 3 {
		t.Errorf("total count: got %v want 3", v.Actual)
	}
	if !strings.Contains(v.Target, "pattern") || !strings.Contains(v.Target, "DNA") {
		t.Errorf("examples should be distinct: %q", v.Target)
	}
	if v.Severity != SeverityWarning {
		t.Errorf("severity: %v", v.Severity)
	}
}

// TestCheckChapterOpeningReplay 锁死 opening_replay：新章开头把上一章
// 结尾整段重抄时报 error violation，但不阻断（Lint/CheckChapter 契约
// 只出事实）。复读过短的钩子回环不得误报。
func TestCheckChapterOpeningReplay(t *testing.T) {
	replayed := "Mưa rơi trên mái ngói cũ, Ngọc khép cửa tiệm rồi đi vào trong. Lá thư cô giấu trong túi áo bỗng nặng như một cục đá. Gió lùa qua khe cửa, mang theo mùi đất ẩm của con hẻm cuối mùa."
	prev := strings.Join([]string{
		"Đoạn giữa chương trước, nội dung không liên quan.",
		replayed,
	}, "\n\n")

	cases := []struct {
		name     string
		text     string
		previous []string
		want     bool
	}{
		{
			name: "copied ending",
			text: "# Chương 3\n\n" + replayed + "\n\nSáng hôm sau trời vẫn mưa.",
			previous: []string{
				prev,
			},
			want: true,
		},
		{
			name:     "short echo allowed",
			text:     "\"Mưa rơi trên mái ngói cũ,\" hắn lẩm bẩm, nhớ lại khung cảnh quen thuộc.\n\nRồi hắn lật sang trang mới của cuộc đời mình.",
			previous: []string{prev},
		},
		{
			name:     "fresh opening",
			text:     "Sáng hôm sau trời đã tạnh, tiệm sách mở cửa sớm hơn thường lệ.\n\nCô xếp lại chồng sách cũ trên quầy.",
			previous: []string{prev},
		},
		{
			name: "no previous chapter",
			text: replayed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := CheckChapter(tc.text, Structured{}, tc.previous)
			var got *Violation
			for i := range vs {
				if vs[i].Rule == "opening_replay" {
					got = &vs[i]
				}
			}
			if tc.want {
				if got == nil {
					t.Fatalf("muốn opening_replay violation, nhận: %+v", vs)
				}
				if got.Severity != SeverityError {
					t.Fatalf("opening_replay severity = %v, muốn error", got.Severity)
				}
			} else if got != nil {
				t.Fatalf("không được báo opening_replay: %+v", vs)
			}
		})
	}
}
