package store

import (
	"testing"

	"github.com/datnt1998/noverse/internal/rules"
)

func TestRecentRuleViolations_LastNLatestPerChapter(t *testing.T) {
	st := NewStore(t.TempDir())
	if got := st.World.RecentRuleViolations(3); len(got) != 0 {
		t.Fatalf("chưa có bản ghi: %v", got)
	}
	v := func(target string) []rules.Violation {
		return []rules.Violation{{Rule: "forbidden_phrases", Target: target, Actual: 1, Severity: rules.SeverityError}}
	}
	for ch := 1; ch <= 5; ch++ {
		if err := st.World.SaveRuleViolations(ch, v("a")); err != nil {
			t.Fatal(err)
		}
	}
	// Chương 4 viết lại và sạch: bản ghi mới nhất (rỗng) thay bản cũ.
	if err := st.World.SaveRuleViolations(4, nil); err != nil {
		t.Fatal(err)
	}
	got := st.World.RecentRuleViolations(3)
	if len(got) != 3 || got[0].Chapter != 5 || got[1].Chapter != 4 || got[2].Chapter != 3 {
		t.Fatalf("last=3 phải là chương 5,4,3: %+v", got)
	}
	if len(got[1].Violations) != 0 || len(got[0].Violations) != 1 || got[0].At == "" {
		t.Fatalf("mỗi chương lấy bản mới nhất: %+v", got)
	}
	if n := len(st.World.RecentRuleViolations(99)); n != 5 {
		t.Fatalf("last lớn hơn số chương: %d", n)
	}
	if st.World.RecentRuleViolations(0) != nil {
		t.Fatal("last<=0 → nil")
	}
}
