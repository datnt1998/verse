package notify

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestScoreText(t *testing.T) {
	for _, c := range []struct {
		score float64
		want  string
	}{{84, "8,4"}, {82.25, "8,2"}, {100, "10,0"}, {0, "0,0"}, {79.96, "8,0"}} {
		if got := ScoreText(c.score); got != c.want {
			t.Errorf("ScoreText(%v) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestGroupTitles(t *testing.T) {
	score := func(v float64) *float64 { return &v }
	chapter := func(n int, s *float64) PushNotification {
		return PushNotification{Type: "done", Story: "p/s", Chapter: n, Title: "Xong chương", Body: "Tên chương", URL: "/read/p%2Fs/20", Score: s}
	}
	cases := []struct {
		name  string
		items []PushNotification
		title string
		body  string
	}{
		{"một chương giữ nguyên", []PushNotification{chapter(7, score(84))}, "Xong chương", "Tên chương"},
		{"năm chương có điểm", []PushNotification{chapter(20, score(80)), chapter(21, score(84)), chapter(22, score(82)), chapter(23, score(80)), chapter(24, score(85))}, "Xong 5 chương (20–24)", "Điểm trung bình 8,2"},
		{"ba chương không điểm", []PushNotification{chapter(1, nil), chapter(2, nil), chapter(3, nil)}, "Xong 3 chương (1–3)", "Đã hoàn thành 3 chương · Tên chương"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := group(c.items)
			if got.Title != c.title || got.Body != c.body {
				t.Fatalf("group = %q / %q, want %q / %q", got.Title, got.Body, c.title, c.body)
			}
		})
	}
}

func TestDedupe(t *testing.T) {
	type step struct {
		advance time.Duration
		read    bool // đánh dấu đã đọc mọi mục trước khi ghi
		n       PushNotification
		want    int // tổng số mục hộp thư sau bước này
	}
	gate := func(ch int) PushNotification { return PushNotification{Type: "gate", Story: "p/s", Chapter: ch} }
	job := func(typ, id string) PushNotification { return PushNotification{Type: typ, Story: "p/s", Job: id} }
	sec := func(session string) PushNotification { return PushNotification{Type: "security", Session: session} }
	cases := []struct {
		name  string
		steps []step
	}{
		{"cổng cũ chưa đọc: phát lại bị chặn dù đã lâu", []step{{n: gate(25), want: 1}, {advance: 5 * time.Hour, n: gate(25), want: 1}}},
		{"cổng cũ đã đọc quá cửa sổ: báo lại", []step{{n: gate(25), want: 1}, {advance: 11 * time.Minute, read: true, n: gate(25), want: 2}}},
		{"cổng chương khác không bị chặn", []step{{n: gate(25), want: 1}, {n: gate(26), want: 2}}},
		{"việc nền cùng mã trong cửa sổ bị chặn", []step{{n: job("job", "p/s/export/1"), want: 1}, {advance: time.Minute, n: job("job", "p/s/export/1"), want: 1}}},
		{"việc nền cùng mã sau cửa sổ được báo lại", []step{{n: job("import", "p/s/import/1"), want: 1}, {advance: 11 * time.Minute, n: job("import", "p/s/import/1"), want: 2}}},
		{"việc nền khác mã", []step{{n: job("sync", "p/s/sync/1"), want: 1}, {n: job("sync", "p/s/sync/2"), want: 2}}},
		{"đăng nhập cùng phiên bị chặn", []step{{n: sec("a"), want: 1}, {n: sec("a"), want: 1}, {n: sec("b"), want: 2}}},
		{"thông báo thử không có phiên luôn được ghi", []step{{n: sec(""), want: 1}, {n: sec(""), want: 2}}},
		{"lỗi không chống trùng", []step{{n: PushNotification{Type: "error", Story: "p/s"}, want: 1}, {n: PushNotification{Type: "error", Story: "p/s"}, want: 2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
			var clock atomic.Value
			clock.Store(now)
			s := openTest(t, Options{Clock: func() time.Time { return clock.Load().(time.Time) }})
			for i, st := range c.steps {
				clock.Store(clock.Load().(time.Time).Add(st.advance))
				if st.read {
					if _, e := s.Read("a", nil, true); e != nil {
						t.Fatal(e)
					}
				}
				mustRecord(t, s, "a", st.n)
				s.mu.Lock()
				got := len(s.user("a").Inbox)
				s.mu.Unlock()
				if got != st.want {
					t.Fatalf("bước %d: %d mục, muốn %d", i, got, st.want)
				}
			}
		})
	}
}

func TestDedupeAgainstPendingDigest(t *testing.T) {
	s := openTest(t, Options{})
	mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: 20})
	mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: 20})
	mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: 21})
	s.mu.Lock()
	pending := len(s.user("a").Digests["p/s"].Items)
	s.mu.Unlock()
	if pending != 2 {
		t.Fatalf("nhóm chờ có %d chương, muốn 2 (chương 20 phát lại phải bị chặn)", pending)
	}
}

func TestSwitchToEachFlushesPendingDigest(t *testing.T) {
	s := openTest(t, Options{})
	for ch := 20; ch < 23; ch++ {
		mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: ch, Body: "Tên", URL: "/read/p%2Fs/20"})
	}
	if rows, _, _ := s.Inbox("a", ""); len(rows) != 0 {
		t.Fatal("chưa đủ 5 chương thì chưa có mục hộp thư")
	}
	p := s.Prefs("a")
	p.Digest = "each"
	if _, e := s.PutPrefs("a", p, p.Version); e != nil {
		t.Fatal(e)
	}
	rows, unread, _ := s.Inbox("a", "")
	if len(rows) != 1 || unread != 1 || !strings.HasPrefix(rows[0].Title, "Xong 3 chương (20–22)") {
		t.Fatalf("chương đang chờ phải được trả ngay khi đổi sang mỗi chương: %+v", rows)
	}
	s.mu.Lock()
	left := len(s.user("a").Digests)
	s.mu.Unlock()
	if left != 0 {
		t.Fatalf("nhóm chờ còn %d sau khi trả", left)
	}
}
