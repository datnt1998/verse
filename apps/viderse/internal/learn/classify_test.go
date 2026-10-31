package learn

import "testing"

func keys(ch []Change) map[string]bool {
	m := map[string]bool{}
	for _, c := range ch {
		m[c.Key] = true
	}
	return m
}

func TestClassifyTitle(t *testing.T) {
	cases := []struct {
		before, after string
		want          string
	}{
		{"The Lost Station (Full Documentary)", "The Lost Station", KeyTrailingParens},
		{"The Lost Station 🔥", "The Lost Station", KeyTitleEmoji},
		{"The Lost Station", "The Lost Station 2026", KeyYearAdded},
		{"The Lost Station 2026", "The Lost Station", KeyYearRemoved},
		{"The Incredible Story Of The Radio Station That Shut Down", "The Radio Station That Shut Down", KeyTitleShorter},
		{"Một tiêu đề", "Tiêu đề khác hẳn", KeyOther},
	}
	for _, c := range cases {
		if !keys(ClassifyText("title", c.before, c.after))[c.want] {
			t.Errorf("%q → %q: want %s, got %+v", c.before, c.after, c.want, ClassifyText("title", c.before, c.after))
		}
	}
	if got := ClassifyText("title", "Same", " Same "); got != nil {
		t.Fatalf("unchanged text classified: %+v", got)
	}
}

func TestClassifyTags(t *testing.T) {
	k := keys(ClassifyTags([]string{"History", "radio"}, []string{"history", "documentary"}))
	if !k[KeyTagAdd+"documentary"] || !k[KeyTagRemove+"radio"] || len(k) != 2 {
		t.Fatalf("tags: %+v", k)
	}
}

func TestAppendNotes(t *testing.T) {
	if got := AppendNotes("Draw a cat", nil); got != "Draw a cat" {
		t.Fatal(got)
	}
	got := AppendNotes("Draw a cat\n", []string{"Không vẽ chữ"})
	if got != "Draw a cat\n\nGhi chú đã học:\n- Không vẽ chữ" {
		t.Fatalf("%q", got)
	}
}
