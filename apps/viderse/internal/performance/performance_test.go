package performance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// ── nguồn đọc ───────────────────────────────────────────────────────────────

func TestDataAPIReaderReadsStatsAndHidesKey(t *testing.T) {
	var gotKey, gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotID = r.URL.Query().Get("key"), r.URL.Query().Get("id")
		_, _ = io.WriteString(w, `{"items":[{"snippet":{"publishedAt":"2026-10-01T10:00:00Z"},"statistics":{"viewCount":"1234","likeCount":"56","commentCount":"7"}}]}`)
	}))
	defer srv.Close()
	reader := &DataAPIReader{BaseURL: srv.URL}
	got, err := reader.ReadWithKey(context.Background(), "AIza-secret", "abcdefghijk")
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "AIza-secret" || gotID != "abcdefghijk" {
		t.Fatalf("query key=%q id=%q", gotKey, gotID)
	}
	if got.Views != 1234 || got.Likes == nil || *got.Likes != 56 || got.Comments == nil || *got.Comments != 7 || got.Source != SourceDataAPI {
		t.Fatalf("reading = %+v", got)
	}
	if got.PublishedAt == nil || !got.PublishedAt.Equal(time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("publishedAt = %v", got.PublishedAt)
	}
}

func TestDataAPIReaderErrorsNeverContainKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":400,"message":"API key not valid","errors":[{"reason":"keyInvalid"}]}}`)
	}))
	reader := &DataAPIReader{BaseURL: srv.URL}
	_, err := reader.ReadWithKey(context.Background(), "AIza-secret", "abcdefghijk")
	var rejected *KeyRejectedError
	if !errors.As(err, &rejected) || strings.Contains(err.Error(), "AIza-secret") {
		t.Fatalf("rejected key error = %v", err)
	}
	srv.Close()
	_, err = reader.ReadWithKey(context.Background(), "AIza-secret", "abcdefghijk")
	if err == nil || strings.Contains(err.Error(), "AIza-secret") {
		t.Fatalf("transport error leaks key: %v", err)
	}
}

func TestDataAPIReaderMissingItemIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"items":[]}`) }))
	defer srv.Close()
	_, err := (&DataAPIReader{BaseURL: srv.URL}).ReadWithKey(context.Background(), "k", "abcdefghijk")
	if !errors.Is(err, ErrVideoUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

// innerTubeFixture trả sw.js_data cho bootstrap và player có viewCount cho /player.
type innerTubeFixture struct {
	t     *testing.T
	views string
	hits  []string
}

func (f *innerTubeFixture) RoundTrip(req *http.Request) (*http.Response, error) {
	f.hits = append(f.hits, req.URL.Path)
	respond := func(body string) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	}
	switch {
	case strings.Contains(req.URL.Path, "sw.js_data"):
		raw, err := os.ReadFile("../youtube/testdata/sw-js-data.txt")
		if err != nil {
			f.t.Fatal(err)
		}
		return respond(string(raw))
	case strings.HasSuffix(req.URL.Path, "/youtubei/v1/player"):
		return respond(fmt.Sprintf(`{"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"abcdefghijk","title":"T","lengthSeconds":"120","viewCount":%q}}`, f.views))
	}
	return nil, errors.New("unexpected request " + req.URL.Path)
}

func TestPublicPageReaderUsesInnerTubeWithoutKey(t *testing.T) {
	fixture := &innerTubeFixture{t: t, views: "4321"}
	client := youtube.NewClient(youtube.Options{
		HTTPClient: &http.Client{Transport: fixture},
		Pacer:      &youtube.Pacer{MinGap: time.Nanosecond, MaxGap: time.Nanosecond},
	})
	auto := AutoReader{
		Key:    func(context.Context) (string, error) { return "", nil },
		Data:   &DataAPIReader{BaseURL: "http://127.0.0.1:1"}, // không được gọi
		Public: PublicPageReader{Client: client},
	}
	if auto.Configured(context.Background()) {
		t.Fatal("no key must not be configured")
	}
	got, err := auto.Read(context.Background(), "abcdefghijk")
	if err != nil {
		t.Fatal(err)
	}
	if got.Views != 4321 || got.Source != SourcePublicPage || got.Likes != nil || got.Comments != nil {
		t.Fatalf("reading = %+v", got)
	}
}

func TestAutoReaderPrefersDataAPIThenFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "good" {
			_, _ = io.WriteString(w, `{"items":[{"statistics":{"viewCount":"10"}}]}`)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"code":403,"message":"quota","errors":[{"reason":"quotaExceeded"}]}}`)
	}))
	defer srv.Close()
	public := readerFunc(func(context.Context, string) (Reading, error) {
		return Reading{Views: 99, Source: SourcePublicPage}, nil
	})
	auto := AutoReader{Key: func(context.Context) (string, error) { return "good", nil }, Data: &DataAPIReader{BaseURL: srv.URL}, Public: public}
	if got, err := auto.Read(context.Background(), "abcdefghijk"); err != nil || got.Source != SourceDataAPI || got.Views != 10 {
		t.Fatalf("data api path: %+v %v", got, err)
	}
	auto.Key = func(context.Context) (string, error) { return "over-quota", nil }
	if got, err := auto.Read(context.Background(), "abcdefghijk"); err != nil || got.Source != SourcePublicPage || got.Views != 99 {
		t.Fatalf("fallback path: %+v %v", got, err)
	}
}

type readerFunc func(context.Context, string) (Reading, error)

func (f readerFunc) Read(ctx context.Context, id string) (Reading, error) { return f(ctx, id) }

func TestParseVideoURL(t *testing.T) {
	good := map[string]string{
		"https://www.youtube.com/watch?v=abcdefghijk":         "abcdefghijk",
		"https://youtu.be/abcdefghijk?t=3":                    "abcdefghijk",
		"https://www.youtube.com/shorts/abc-defg_hi":          "abc-defg_hi",
		"https://m.youtube.com/watch?v=abcdefghijk&feature=x": "abcdefghijk",
		"youtube.com/watch?v=abcdefghijk":                     "abcdefghijk",
		"https://www.youtube.com/live/abcdefghijk":            "abcdefghijk",
	}
	for raw, want := range good {
		if got, err := ParseVideoURL(raw); err != nil || got != want {
			t.Errorf("%s → %q %v, want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "not a url", "https://evil.com/watch?v=abcdefghijk", "https://www.youtube.com/watch?v=short", "https://www.youtube.com/@kenh", "https://user:pw@youtube.com/watch?v=abcdefghijk", "ftp://youtube.com/watch?v=abcdefghijk"} {
		if _, err := ParseVideoURL(raw); !errors.Is(err, ErrBadURL) {
			t.Errorf("%q accepted: %v", raw, err)
		}
	}
}

// ── nhận xét ────────────────────────────────────────────────────────────────

func nineVideos() []Video {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC) // 16:00 giờ VN
	at := func(hour int) time.Time { return base.Add(time.Duration(hour-9) * time.Hour) }
	mk := func(i int, title, tpl string, dur int, hour int, views int64) Video {
		return Video{JobID: fmt.Sprintf("job%d", i), Title: title, TemplateID: tpl, TemplateName: strings.ToUpper(tpl), DurationSec: dur, At: at(hour), Views7d: views}
	}
	return []Video{
		mk(1, "Tại sao A?", "tpla", 150, 9, 3000),
		mk(2, "Vì sao B?", "tpla", 160, 9, 2800),
		mk(3, "Làm sao C?", "tpla", 170, 10, 3200),
		mk(4, "Chuyện A", "tplb", 600, 20, 1500),
		mk(5, "Chuyện B", "tplb", 620, 20, 1400),
		mk(6, "Chuyện C", "tplb", 640, 21, 1600),
		mk(7, "Kể A", "tplb", 650, 21, 1300),
		mk(8, "Kể B", "tpla", 140, 9, 2900),
		mk(9, "Kể C", "tplb", 610, 20, 1450),
	}
}

func TestAnalyzeNineVideosAtMostThreePreliminaryInsights(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	insights, needMore := Analyze("ch1", nineVideos(), loc)
	if needMore != 0 {
		t.Fatalf("needMore = %d", needMore)
	}
	if len(insights) == 0 {
		t.Fatal("expected insights")
	}
	if len(insights) > MaxInsights+1 {
		t.Fatalf("too many raw insights: %d", len(insights))
	}
	dims := map[Dimension]bool{}
	for _, in := range insights {
		dims[in.Dimension] = true
		if !in.Preliminary {
			t.Errorf("%s not preliminary with 9 videos", in.ID)
		}
		if in.Ratio < MinRatio || !strings.Contains(in.Text, ",") {
			t.Errorf("weak or unformatted insight: %+v", in)
		}
		for _, g := range in.Groups {
			if g.N < MinGroup {
				t.Errorf("group %q has %d < %d videos", g.Label, g.N, MinGroup)
			}
		}
	}
	for _, want := range []Dimension{DimTemplate, DimTitlePattern, DimLength} {
		if !dims[want] {
			t.Errorf("missing %s insight in %v", want, dims)
		}
	}
	for i := 1; i < len(insights); i++ {
		if insights[i-1].Ratio < insights[i].Ratio {
			t.Errorf("not sorted by ratio")
		}
	}
}

func TestAnalyzeNeedMoreBelowSix(t *testing.T) {
	insights, needMore := Analyze("ch1", nineVideos()[:4], time.UTC)
	if len(insights) != 0 || needMore != 2 {
		t.Fatalf("insights=%d needMore=%d", len(insights), needMore)
	}
	if _, needMore = Analyze("ch1", nil, nil); needMore != MinVideos {
		t.Fatalf("empty needMore=%d", needMore)
	}
}

func TestAnalyzeTwentyVideosNotPreliminary(t *testing.T) {
	videos := []Video{}
	for i := 0; i < 20; i++ {
		v := nineVideos()[i%9]
		v.JobID = fmt.Sprintf("j%d", i)
		videos = append(videos, v)
	}
	insights, _ := Analyze("ch1", videos, time.UTC)
	for _, in := range insights {
		if in.Preliminary {
			t.Fatalf("%s preliminary at 20 videos", in.ID)
		}
	}
}

func TestAnalyzeTiedGroupsGiveNoInsight(t *testing.T) {
	videos := []Video{}
	for i := 0; i < 8; i++ {
		tpl := "a"
		if i%2 == 1 {
			tpl = "b"
		}
		videos = append(videos, Video{JobID: fmt.Sprint(i), TemplateID: tpl, TemplateName: tpl, Views7d: 1000 + int64(i%2)})
	}
	for _, in := range func() []Insight { r, _ := Analyze("c", videos, time.UTC); return r }() {
		if in.Dimension == DimTemplate {
			t.Fatalf("near-equal templates produced insight %q", in.Text)
		}
	}
}

// ── áp dụng, vs trung vị, luân phiên ────────────────────────────────────────

type fakeStore struct {
	posted  []PostedRow
	stats   []StatRow
	applied map[string]Applied
	weights map[string]int
	facts   map[string]string
}

func (f *fakeStore) SeedsWithoutFacts(context.Context, int) ([]Seed, error) { return nil, nil }
func (f *fakeStore) EnsureFacts(_ context.Context, id, title string, _ int) error {
	if f.facts == nil {
		f.facts = map[string]string{}
	}
	f.facts[id] = title
	return nil
}
func (f *fakeStore) SetYoutube(context.Context, string, string) error  { return nil }
func (f *fakeStore) YoutubeID(context.Context, string) (string, error) { return "", nil }
func (f *fakeStore) ListPosted(context.Context, string) ([]PostedRow, error) {
	return f.posted, nil
}
func (f *fakeStore) ListStats(context.Context, string) ([]StatRow, error) { return f.stats, nil }
func (f *fakeStore) PostedChannelIDs(context.Context) ([]string, error)   { return []string{"ch1"}, nil }
func (f *fakeStore) ReadCandidates(context.Context, time.Time) ([]Candidate, error) {
	return nil, nil
}
func (f *fakeStore) SetPublished(context.Context, string, time.Time) error { return nil }
func (f *fakeStore) SaveStat(context.Context, string, Milestone, Reading, time.Time) error {
	return nil
}
func (f *fakeStore) ReadFailed(context.Context, string, time.Time) error { return nil }
func (f *fakeStore) ReadOK(context.Context, string) error                { return nil }
func (f *fakeStore) ListApplied(context.Context, string) (map[string]Applied, error) {
	out := map[string]Applied{}
	for k, v := range f.applied {
		out[k] = v
	}
	return out, nil
}
func (f *fakeStore) MarkApplied(_ context.Context, _ string, id string, a Applied) (bool, error) {
	if f.applied == nil {
		f.applied = map[string]Applied{}
	}
	if _, ok := f.applied[id]; ok {
		return false, nil
	}
	f.applied[id] = a
	return true, nil
}
func (f *fakeStore) SetTemplateWeight(_ context.Context, _ string, tpl string, w int) error {
	if f.weights == nil {
		f.weights = map[string]int{}
	}
	f.weights[tpl] = w
	return nil
}

type fakeRules struct{ got []RuleInput }

func (f *fakeRules) AddRule(_ context.Context, in RuleInput) (string, error) {
	f.got = append(f.got, in)
	return "rule-1", nil
}

func storeFor(videos []Video) *fakeStore {
	st := &fakeStore{}
	for _, v := range videos {
		st.posted = append(st.posted, PostedRow{JobID: v.JobID, PostedAt: v.At, TemplateID: v.TemplateID, TemplateName: v.TemplateName, Title: v.Title, DurationSec: v.DurationSec})
		st.stats = append(st.stats, StatRow{JobID: v.JobID, Milestone: M7d, Stat: Stat{Views: v.Views7d}, Source: SourcePublicPage})
	}
	return st
}

func TestPerformanceViewShowsAtMostThreeAndVsMedian(t *testing.T) {
	svc := &Service{Store: storeFor(nineVideos()), Loc: time.FixedZone("ICT", 7*3600)}
	view, err := svc.Performance(context.Background(), "ch1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Insights) == 0 || len(view.Insights) > MaxInsights || view.NeedMore != 0 || view.Counted != 9 {
		t.Fatalf("view = insights %d needMore %d counted %d", len(view.Insights), view.NeedMore, view.Counted)
	}
	for _, in := range view.Insights {
		if !in.Preliminary {
			t.Errorf("%s must be preliminary", in.ID)
		}
	}
	if view.Median7d == nil || *view.Median7d != 1600 {
		t.Fatalf("median = %v", view.Median7d)
	}
	for _, p := range view.Posted {
		if p.At7d == nil || p.VsMedian7d == nil {
			t.Fatalf("missing vsMedian7d on %s", p.JobID)
		}
		if want := float64(p.At7d.Views) / 1600; *p.VsMedian7d < want-0.01 || *p.VsMedian7d > want+0.01 {
			t.Errorf("%s vsMedian7d = %v, want ≈ %v", p.JobID, *p.VsMedian7d, want)
		}
	}
}

func TestPerformanceNeedMoreBelowSix(t *testing.T) {
	svc := &Service{Store: storeFor(nineVideos()[:5])}
	view, err := svc.Performance(context.Background(), "ch1")
	if err != nil || view.NeedMore != 1 || len(view.Insights) != 0 {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestBlankStatsStayBlank(t *testing.T) {
	st := storeFor(nineVideos()[:3])
	st.stats = nil // đọc lỗi: không có số nào
	svc := &Service{Store: st}
	view, err := svc.Performance(context.Background(), "ch1")
	if err != nil || len(view.Posted) != 3 || view.Posted[0].At7d != nil || view.Posted[0].VsMedian7d != nil || view.NeedMore != MinVideos {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestApplyTitleInsightCreatesRuleWithNumericSource(t *testing.T) {
	st := storeFor(nineVideos())
	rules := &fakeRules{}
	svc := &Service{Store: st, Rules: rules, Loc: time.FixedZone("ICT", 7*3600)}
	var titleInsight string
	view, _ := svc.Performance(context.Background(), "ch1")
	all, _ := Analyze("ch1", nineVideos(), svc.Loc)
	for _, in := range all {
		if in.Dimension == DimTitlePattern {
			titleInsight = in.ID
		}
	}
	if titleInsight == "" {
		t.Fatalf("no title insight among %d", len(all))
	}
	res, err := svc.Apply(context.Background(), "ch1", titleInsight)
	if err != nil || res.Kind != "rule" || res.RuleID != "rule-1" || res.AlreadyApplied {
		t.Fatalf("apply = %+v %v", res, err)
	}
	if len(rules.got) != 1 || rules.got[0].Source != "Từ số liệu 9 video" || rules.got[0].Field != "title" || rules.got[0].ChannelID != "ch1" || rules.got[0].Text == "" {
		t.Fatalf("rule input = %+v", rules.got)
	}
	// áp lần hai: không thêm quy tắc, báo đã áp dụng
	again, err := svc.Apply(context.Background(), "ch1", titleInsight)
	if err != nil || !again.AlreadyApplied || len(rules.got) != 1 {
		t.Fatalf("second apply = %+v %v rules=%d", again, err, len(rules.got))
	}
	// nhận xét đã áp dụng biến khỏi danh sách và khỏi tín hiệu perf_insight
	view, _ = svc.Performance(context.Background(), "ch1")
	for _, in := range view.Insights {
		if in.ID == titleInsight {
			t.Fatal("applied insight still listed")
		}
	}
	signals, err := svc.OpenInsights(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range signals {
		if s.InsightID == titleInsight || s.Videos != 9 || s.ChannelID != "ch1" || !s.Preliminary {
			t.Fatalf("bad signal %+v", s)
		}
	}
	if _, err = svc.Apply(context.Background(), "ch1", "title_pattern:nope"); !errors.Is(err, ErrInsightGone) {
		t.Fatalf("unknown insight = %v", err)
	}
}

func TestApplyTemplateInsightDoublesRotationWeight(t *testing.T) {
	st := storeFor(nineVideos())
	svc := &Service{Store: st}
	res, err := svc.Apply(context.Background(), "ch1", "template:tpla")
	if err != nil || res.Kind != "rotation" || st.weights["tpla"] != 2 {
		t.Fatalf("apply = %+v %v weights=%v", res, err, st.weights)
	}
}

// ── đọc định kỳ ─────────────────────────────────────────────────────────────

type tickStore struct {
	fakeStore
	cands  []Candidate
	saved  map[Milestone]Reading
	failed int
}

func (s *tickStore) ReadCandidates(context.Context, time.Time) ([]Candidate, error) {
	return s.cands, nil
}
func (s *tickStore) SaveStat(_ context.Context, _ string, m Milestone, r Reading, _ time.Time) error {
	if s.saved == nil {
		s.saved = map[Milestone]Reading{}
	}
	s.saved[m] = r
	return nil
}
func (s *tickStore) ReadFailed(context.Context, string, time.Time) error { s.failed++; return nil }

func TestTickSavesDueMilestonesAndSurvivesFailures(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	posted := now.Add(-8 * 24 * time.Hour) // đã qua 24h và 7d
	st := &tickStore{cands: []Candidate{{JobID: "j1", YoutubeID: "abcdefghijk", PostedAt: posted, Have: map[Milestone]bool{}}}}
	svc := &Service{Store: st, Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (Reading, error) {
		return Reading{Views: 700, Source: SourcePublicPage}, nil
	})}
	svc.Tick(context.Background())
	if st.saved[M7d].Views != 700 || st.saved[M28d].Views != 0 {
		t.Fatalf("saved = %+v", st.saved)
	}
	if _, ok := st.saved[M24h]; ok {
		t.Fatal("24h grace expired (read 7 days late) but was saved")
	}
	// đọc lỗi: không số, không panic, ghi để thử lại
	st2 := &tickStore{cands: st.cands}
	svc.Store = st2
	svc.Reader = readerFunc(func(context.Context, string) (Reading, error) { return Reading{}, errors.New("mạng lỗi") })
	svc.Tick(context.Background())
	if len(st2.saved) != 0 || st2.failed != 1 {
		t.Fatalf("failure path saved=%v failed=%d", st2.saved, st2.failed)
	}
}
