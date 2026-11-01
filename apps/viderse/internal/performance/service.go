package performance

import (
	"context"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// PostedRow là một video đã đăng của kênh, kèm facts chụp lúc bắt đầu theo dõi.
type PostedRow struct {
	JobID        string
	PostedAt     time.Time
	TemplateID   string
	TemplateName string
	YoutubeID    string
	Title        string
	DurationSec  int
	PublishedAt  *time.Time
}

// StatRow là số đọc ở một mốc.
type StatRow struct {
	JobID     string
	Milestone Milestone
	Stat
	Source Source
}

// Candidate là video tới lượt thử đọc.
type Candidate struct {
	JobID       string
	YoutubeID   string
	PostedAt    time.Time
	PublishedAt *time.Time
	Attempts    int
	Have        map[Milestone]bool
}

// Seed là video đã đăng chưa có facts.
type Seed struct {
	JobID       string
	DurationSec int
}

// Applied là nhận xét đã áp dụng.
type Applied struct {
	Kind   string
	RuleID string
}

// Store là phần lưu trữ Service cần (postgres.PerformanceStore thoả mãn).
type Store interface {
	SeedsWithoutFacts(ctx context.Context, limit int) ([]Seed, error)
	EnsureFacts(ctx context.Context, jobID, title string, durationSec int) error
	// SetYoutube đổi link; link khác thì xoá mọi số đã đọc của link cũ.
	SetYoutube(ctx context.Context, jobID, youtubeID string) error
	YoutubeID(ctx context.Context, jobID string) (string, error)
	ListPosted(ctx context.Context, channelID string) ([]PostedRow, error)
	ListStats(ctx context.Context, channelID string) ([]StatRow, error)
	PostedChannelIDs(ctx context.Context) ([]string, error)
	ReadCandidates(ctx context.Context, now time.Time) ([]Candidate, error)
	SetPublished(ctx context.Context, jobID string, at time.Time) error
	SaveStat(ctx context.Context, jobID string, milestone Milestone, reading Reading, at time.Time) error
	ReadFailed(ctx context.Context, jobID string, next time.Time) error
	ReadOK(ctx context.Context, jobID string) error
	ListApplied(ctx context.Context, channelID string) (map[string]Applied, error)
	// MarkApplied trả false khi nhận xét đã được đánh dấu từ trước.
	MarkApplied(ctx context.Context, channelID, insightID string, applied Applied) (bool, error)
	SetTemplateWeight(ctx context.Context, channelID, templateID string, weight int) error
}

// Jobs là phần của dịch vụ video mà đánh dấu đã đăng cần.
type Jobs interface {
	Get(ctx context.Context, id string) (*domain.CloneJobView, error)
}

// PostMarker ghi / bỏ dấu đã đăng của video (cùng đường với thao tác hàng loạt).
type PostMarker interface {
	MarkPosted(ctx context.Context, jobID string, posted bool) error
}

// RuleInput là một quy tắc đã học cần thêm (P08LearnApi thoả RuleAdder).
type RuleInput struct {
	ChannelID string
	Field     string // "title" cho nhận xét kiểu tiêu đề
	Text      string
	Source    string // "Từ số liệu N video"
}

// RuleAdder thêm quy tắc đã học và trả mã quy tắc.
type RuleAdder interface {
	AddRule(ctx context.Context, in RuleInput) (ruleID string, err error)
}

// Service là cửa chung của HTTP, bộ lập lịch đọc số liệu và bộ sinh đề xuất.
type Service struct {
	Store  Store
	Reader Reader
	Jobs   Jobs
	Marker PostMarker
	// Title trả tiêu đề hiện tại của video (lỗi → facts không có tiêu đề).
	Title func(ctx context.Context, jobID string) (string, error)
	Rules RuleAdder
	// Configured cho biết đang dùng khoá Data API (để báo nguồn số liệu).
	Configured func(ctx context.Context) bool
	Now        func() time.Time
	Loc        *time.Location
	// Publish báo sự kiện cho giao diện (có thể nil).
	Publish func(event string, payload any)
	// Interval là nhịp quét mốc đọc của Run; mặc định 15 phút.
	Interval time.Duration
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Post đánh dấu đã đăng, kèm link YouTube tuỳ chọn. Gọi lại với cùng nội dung là no-op.
func (s *Service) Post(ctx context.Context, jobID, youtubeURL string) (string, error) {
	youtubeID := ""
	if youtubeURL != "" {
		id, err := ParseVideoURL(youtubeURL)
		if err != nil {
			return "", err
		}
		youtubeID = id
	}
	view, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		return "", err
	}
	if view == nil {
		return "", ErrNotFound
	}
	if view.PostedAt == nil {
		if view.Status != domain.CloneSucceeded {
			return "", ErrNotReady
		}
		if err = s.Marker.MarkPosted(ctx, jobID, true); err != nil {
			return "", err
		}
	}
	if err = s.ensureFacts(ctx, jobID, durationOf(view.Source.DurationSec)); err != nil {
		return "", err
	}
	if youtubeID != "" {
		if err = s.Store.SetYoutube(ctx, jobID, youtubeID); err != nil {
			return "", err
		}
	} else if youtubeID, err = s.Store.YoutubeID(ctx, jobID); err != nil {
		return "", err
	}
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": jobID})
	}
	return youtubeID, nil
}

func durationOf(v *float64) int {
	if v == nil || *v <= 0 {
		return 0
	}
	return int(*v + 0.5)
}

func (s *Service) ensureFacts(ctx context.Context, jobID string, duration int) error {
	title := ""
	if s.Title != nil {
		if t, err := s.Title(ctx, jobID); err == nil {
			title = t
		}
	}
	return s.Store.EnsureFacts(ctx, jobID, title, duration)
}

// seedFacts chụp facts cho video đã đăng bằng đường khác (thao tác hàng loạt).
func (s *Service) seedFacts(ctx context.Context) {
	seeds, err := s.Store.SeedsWithoutFacts(ctx, 100)
	if err != nil {
		return
	}
	for _, seed := range seeds {
		if ctx.Err() != nil {
			return
		}
		_ = s.ensureFacts(ctx, seed.JobID, seed.DurationSec)
	}
}

type computed struct {
	view     View
	all      []Insight
	videos   int
	applied  map[string]Applied
	byJob    map[string]*PostedStats
	postedAt map[string]time.Time
}

func (s *Service) compute(ctx context.Context, channelID string) (computed, error) {
	rows, err := s.Store.ListPosted(ctx, channelID)
	if err != nil {
		return computed{}, err
	}
	stats, err := s.Store.ListStats(ctx, channelID)
	if err != nil {
		return computed{}, err
	}
	applied, err := s.Store.ListApplied(ctx, channelID)
	if err != nil {
		return computed{}, err
	}
	byJob := map[string]*PostedStats{}
	posted := make([]PostedStats, len(rows))
	for i, r := range rows {
		posted[i] = PostedStats{JobID: r.JobID, YoutubeID: r.YoutubeID, PostedAt: r.PostedAt, Title: r.Title, TemplateID: r.TemplateID, TemplateTag: r.TemplateName}
		byJob[r.JobID] = &posted[i]
	}
	for _, st := range stats {
		p := byJob[st.JobID]
		if p == nil {
			continue
		}
		stat := st.Stat
		switch st.Milestone {
		case M24h:
			p.At24h = &stat
		case M7d:
			p.At7d = &stat
		case M28d:
			p.At28d = &stat
		}
	}
	videos := []Video{}
	views := []int64{}
	for i, r := range rows {
		p := &posted[i]
		if p.At7d == nil {
			continue
		}
		at := r.PostedAt
		if r.PublishedAt != nil {
			at = *r.PublishedAt
		}
		videos = append(videos, Video{JobID: r.JobID, Title: r.Title, TemplateID: r.TemplateID, TemplateName: r.TemplateName, DurationSec: r.DurationSec, At: at, Views7d: p.At7d.Views})
		views = append(views, p.At7d.Views)
	}
	out := computed{applied: applied, videos: len(videos)}
	view := View{Posted: posted, Insights: []Insight{}, Counted: len(videos), Source: SourcePublicPage}
	if s.Configured != nil && s.Configured(ctx) {
		view.Source = SourceDataAPI
	}
	if len(views) > 0 {
		m := Median(views)
		view.Median7d = &m
		if m > 0 {
			for i := range posted {
				if posted[i].At7d != nil {
					v := roundTo(float64(posted[i].At7d.Views)/float64(m), 2)
					posted[i].VsMedian7d = &v
				}
			}
		}
	}
	all, needMore := Analyze(channelID, videos, s.Loc)
	view.NeedMore = needMore
	out.all = all
	out.view = view
	return out, nil
}

func roundTo(v float64, places int) float64 {
	p := 1.0
	for range places {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}

func openInsights(c computed) []Insight {
	open := []Insight{}
	for _, in := range c.all {
		if _, done := c.applied[in.ID]; done {
			continue
		}
		open = append(open, in)
		if len(open) == MaxInsights {
			break
		}
	}
	return open
}

// Performance dựng GET /api/channels/:id/performance.
func (s *Service) Performance(ctx context.Context, channelID string) (View, error) {
	s.seedFacts(ctx)
	c, err := s.compute(ctx, channelID)
	if err != nil {
		return View{}, err
	}
	c.view.Insights = openInsights(c)
	return c.view, nil
}

// OpenInsights trả các nhận xét chưa áp dụng của mọi kênh (tín hiệu perf_insight).
func (s *Service) OpenInsights(ctx context.Context) ([]InsightSignal, error) {
	ids, err := s.Store.PostedChannelIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := []InsightSignal{}
	for _, id := range ids {
		c, err := s.compute(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, in := range openInsights(c) {
			sig := InsightSignal{ChannelID: id, InsightID: in.ID, Dimension: string(in.Dimension), Text: in.Text, Preliminary: in.Preliminary, Videos: c.videos}
			if in.Apply != nil {
				sig.ApplyOp = in.Apply.Op
			}
			out = append(out, sig)
		}
	}
	return out, nil
}

// Apply áp dụng một nhận xét: thêm quy tắc đã học hoặc cho template chạy nhiều hơn.
// Gọi lại trên nhận xét đã áp dụng trả kết quả cũ với AlreadyApplied.
func (s *Service) Apply(ctx context.Context, channelID, insightID string) (ApplyResult, error) {
	c, err := s.compute(ctx, channelID)
	if err != nil {
		return ApplyResult{}, err
	}
	if done, ok := c.applied[insightID]; ok {
		return ApplyResult{Kind: done.Kind, RuleID: done.RuleID, AlreadyApplied: true}, nil
	}
	var found *Insight
	for i := range c.all {
		if c.all[i].ID == insightID {
			found = &c.all[i]
			break
		}
	}
	if found == nil || found.Apply == nil {
		return ApplyResult{}, ErrInsightGone
	}
	result := ApplyResult{}
	switch found.Apply.Op {
	case "add_rule":
		if s.Rules == nil {
			return ApplyResult{}, ErrNoRuleAdder
		}
		text, _ := found.Apply.Params["text"].(string)
		field, _ := found.Apply.Params["field"].(string)
		id, err := s.Rules.AddRule(ctx, RuleInput{ChannelID: channelID, Field: field, Text: text, Source: fmt.Sprintf("Từ số liệu %d video", c.videos)})
		if err != nil {
			return ApplyResult{}, err
		}
		result = ApplyResult{Kind: "rule", RuleID: id}
	case "template.rotation":
		templateID, _ := found.Apply.Params["templateId"].(string)
		if err := s.Store.SetTemplateWeight(ctx, channelID, templateID, 2); err != nil {
			return ApplyResult{}, err
		}
		result = ApplyResult{Kind: "rotation"}
	default:
		return ApplyResult{}, ErrInsightGone
	}
	first, err := s.Store.MarkApplied(ctx, channelID, insightID, Applied{Kind: result.Kind, RuleID: result.RuleID})
	if err != nil {
		return ApplyResult{}, err
	}
	result.AlreadyApplied = !first
	return result, nil
}

// Run quét mốc đọc định kỳ tới khi ctx kết thúc.
func (s *Service) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		s.Tick(ctx)
		timer.Reset(interval)
	}
}

// dueMilestones trả các mốc đã tới hạn, chưa có số và chưa quá hạn muộn.
func dueMilestones(base, now time.Time, have map[Milestone]bool) []Milestone {
	due := []Milestone{}
	for _, m := range Milestones {
		if have[m] {
			continue
		}
		start := base.Add(m.After())
		if !now.Before(start) && !now.After(start.Add(m.Grace())) {
			due = append(due, m)
		}
	}
	return due
}

// Tick đọc số liệu cho các video tới mốc. Mọi lỗi chỉ ghi lại để thử lại sau, không chặn gì.
func (s *Service) Tick(ctx context.Context) {
	s.seedFacts(ctx)
	now := s.now()
	candidates, err := s.Store.ReadCandidates(ctx, now)
	if err != nil {
		return
	}
	for _, c := range candidates {
		if ctx.Err() != nil {
			return
		}
		base := c.PostedAt
		if c.PublishedAt != nil {
			base = *c.PublishedAt
		}
		if c.PublishedAt != nil && len(dueMilestones(base, now, c.Have)) == 0 {
			continue
		}
		reading, err := s.Reader.Read(ctx, c.YoutubeID)
		if err != nil {
			backoff := time.Duration(1<<min(c.Attempts, 5)) * 15 * time.Minute
			_ = s.Store.ReadFailed(ctx, c.JobID, now.Add(min(backoff, 6*time.Hour)))
			continue
		}
		if reading.PublishedAt != nil && (c.PublishedAt == nil || !c.PublishedAt.Equal(*reading.PublishedAt)) {
			if e := s.Store.SetPublished(ctx, c.JobID, *reading.PublishedAt); e == nil {
				base = *reading.PublishedAt
			}
		}
		_ = s.Store.ReadOK(ctx, c.JobID)
		for _, m := range dueMilestones(base, now, c.Have) {
			_ = s.Store.SaveStat(ctx, c.JobID, m, reading, now)
		}
	}
}
