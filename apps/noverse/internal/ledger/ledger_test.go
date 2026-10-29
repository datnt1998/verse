package ledger

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func testValues(prefix string) Values {
	var v Values
	for i := range v {
		v[i] = prefix + string(rune('a'+i))
	}
	return v
}
func TestLedgerRuleBoundaries(t *testing.T) {
	old := testValues("old")
	fresh := testValues("new")
	tests := []struct {
		name        string
		v           Values
		prior       []Story
		rule, level string
	}{
		{"combination exact", old, []Story{{ID: "old", Values: old}}, "combination", "error"},
		{"combination changed", fresh, []Story{{ID: "old", Values: old}}, "combination", "ok"},
	}
	for _, n := range []int{3, 4} {
		v := old
		for i := range n {
			v[i] = fresh[i]
		}
		level := "ok"
		if n == 3 {
			level = "error"
		}
		tests = append(tests, struct {
			name        string
			v           Values
			prior       []Story
			rule, level string
		}{name: "distance " + string(rune('0'+n)), v: v, prior: []Story{{Values: old}}, rule: "distance", level: level})
	}
	for _, n := range []int{1, 2} {
		prior := make([]Story, n)
		for i := range prior {
			prior[i] = Story{Values: old}
		}
		level := "ok"
		if n == 2 {
			level = "warning"
		}
		tests = append(tests, struct {
			name        string
			v           Values
			prior       []Story
			rule, level string
		}{name: "use " + string(rune('1'+n)), v: old, prior: prior, rule: "overuse", level: level})
	}
	tests = append(tests, struct {
		name        string
		v           Values
		prior       []Story
		rule, level string
	}{"opening third", old, []Story{{Values: old}, {Values: old}}, "opening", "warning"})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckValues(tc.v, tc.prior)
			if len(r.Checks) != 4 {
				t.Fatalf("four checks: %d", len(r.Checks))
			}
			for _, c := range r.Checks {
				if c.Rule == tc.rule && c.Level != tc.level {
					t.Fatalf("%s level=%s want=%s", tc.rule, c.Level, tc.level)
				}
			}
		})
	}
	// The eleventh prior story must not count.
	prior := make([]Story, 11)
	for i := range prior {
		prior[i].Values = fresh
	}
	prior[10].Values = old
	if CheckValues(old, prior).Checks[0].Level != "ok" {
		t.Fatal("eleventh story entered ten-story window")
	}
}

func TestLedgerAxisNamesMatchCanonicalSource(t *testing.T) {
	source, err := os.ReadFile("../../docs/prd/rules/he-thong-san-xuat-lien-tuc.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range Axes {
		if !strings.Contains(string(source), fmt.Sprintf("| %d. %s |", a.N, a.Name)) {
			t.Fatalf("axis %d name drift: %q", a.N, a.Name)
		}
	}
}

// short renders every check as one line for the narrow comparison card.
// Sentences follow the Đồng sáng tác templates of the NewStory artboard.
func TestLedgerCheckShortTemplates(t *testing.T) {
	clean := testValues("new")
	old := testValues("old")
	prior := []Story{{ID: "proj/02-hai", Title: "Hai", Values: old}, {ID: "proj/01-mot", Title: "Một", Values: old}}
	// fix is the ProjBrain artboard's right-aligned suggestion: only the
	// duplicate combination (error) and overuse (warning) draw one; a passed
	// check and distance/opening are null.
	type wantCheck struct{ level, short, fix string }
	cases := []struct {
		name  string
		v     Values
		prior []Story
		want  map[string]wantCheck
	}{
		{
			name:  "ok",
			v:     clean,
			prior: prior,
			want: map[string]wantCheck{
				"combination": {"ok", "Bốn trục chính chưa trùng truyện nào.", ""},
				"distance":    {"ok", "Khác truyện vừa đăng 10/10 trục.", ""},
				"overuse":     {"ok", "Không trục nào dùng quá 2 lần.", ""},
				"opening":     {"ok", "Mở đầu khác 2 truyện trước.", ""},
			},
		},
		{
			name:  "error and warning",
			v:     old,
			prior: prior,
			want: map[string]wantCheck{
				"combination": {"error", "Giống “Hai” ở 4 trục chính (bộ, nghề, phản diện, vũ khí).", "Đổi ít nhất 1 trong 4 trục"},
				"distance":    {"error", "Chỉ khác truyện vừa đăng 0/10 trục.", ""},
				"overuse":     {"warning", "Nghề “oldc” đã có 2 truyện dùng · Phản diện “oldd” đã có 2 truyện dùng · Sân khấu “oldg” đã có 2 truyện dùng.", "Cân nhắc đổi"},
				"opening":     {"warning", "Mở đầu lặp 3 truyện liền.", ""},
			},
		},
		{
			name:  "no prior story",
			v:     clean,
			prior: nil,
			want: map[string]wantCheck{
				"combination": {"ok", "Bốn trục chính chưa trùng truyện nào.", ""},
				"distance":    {"ok", "Chưa có truyện liền trước để so khoảng cách.", ""},
				"overuse":     {"ok", "Không trục nào dùng quá 2 lần.", ""},
				"opening":     {"ok", "Mở đầu khác 2 truyện trước.", ""},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckValues(tc.v, tc.prior)
			if len(r.Checks) != 4 {
				t.Fatalf("four checks: %d", len(r.Checks))
			}
			for _, c := range r.Checks {
				want, ok := tc.want[c.Rule]
				if !ok {
					t.Fatalf("unexpected rule %q", c.Rule)
				}
				if c.Level != want.level {
					t.Fatalf("%s level=%s want=%s", c.Rule, c.Level, want.level)
				}
				if c.Short != want.short {
					t.Fatalf("%s short=%q want=%q", c.Rule, c.Short, want.short)
				}
				gotFix := ""
				if c.Fix != nil {
					gotFix = *c.Fix
				}
				if gotFix != want.fix {
					t.Fatalf("%s fix=%q want=%q", c.Rule, gotFix, want.fix)
				}
			}
		})
	}
}

// Exactly four rows, in contract order, whatever the data.
func TestLedgerAlwaysFourChecksInOrder(t *testing.T) {
	old := testValues("old")
	prior := []Story{{ID: "proj/02-hai", Title: "Hai", Values: old}, {ID: "proj/01-mot", Title: "Một", Values: old}}
	for name, r := range map[string]Result{
		"empty": CheckValues(testValues("new"), nil),
		"clean": CheckValues(testValues("new"), prior),
		"dup":   CheckValues(old, prior),
	} {
		got := make([]string, 0, len(r.Checks))
		for _, c := range r.Checks {
			if c.Short == "" {
				t.Fatalf("%s: %s has no short", name, c.Rule)
			}
			got = append(got, c.Rule)
		}
		want := []string{"combination", "distance", "overuse", "opening"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: %v want %v", name, got, want)
		}
	}
}

// Overused values of different axes stay in ONE warning row listing both axes.
func TestLedgerOveruseAggregatesAxes(t *testing.T) {
	old := testValues("old")
	v := testValues("new")
	v[2], v[3] = old[2], old[3]
	prior := []Story{{ID: "proj/02-hai", Title: "Hai", Values: old}, {ID: "proj/01-mot", Title: "Một", Values: old}}
	r := CheckValues(v, prior)
	var rows []Check
	for _, c := range r.Checks {
		if c.Rule == "overuse" {
			rows = append(rows, c)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("one overuse row: %d", len(rows))
	}
	if fmt.Sprint(rows[0].Axes) != "[3 4]" {
		t.Fatalf("axes=%v", rows[0].Axes)
	}
	want := "Nghề “oldc” đã có 2 truyện dùng · Phản diện “oldd” đã có 2 truyện dùng."
	if rows[0].Short != want || rows[0].Level != "warning" {
		t.Fatalf("overuse=%q level=%s", rows[0].Short, rows[0].Level)
	}
}
