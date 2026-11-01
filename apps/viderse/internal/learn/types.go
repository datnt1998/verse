// Package learn giữ Quy tắc đã học: những gì trợ lý nhận ra từ việc chủ kênh sửa
// (tiêu đề, mô tả, tag) và góp ý vẽ lại. Một kiểu sửa lặp lại chỉ trở thành đề
// xuất; quy tắc chỉ có hiệu lực khi người dùng bấm Lưu.
package learn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AgentDirector = "director"
	AgentArtist   = "artist"

	SourceEdits       = "edits"
	SourceRedraw      = "redraw"
	SourcePerformance = "performance"
	SourceAssistant   = "assistant"
	SourceManual      = "manual"

	// MaxRulesPerScope là số quy tắc tối đa của một kênh (hoặc một prompt ảnh).
	MaxRulesPerScope = 20
	// EditThreshold: cùng một kiểu sửa lặp lại trong một kênh.
	EditThreshold = 3
	// RedrawThreshold: cùng một góp ý vẽ lại với cùng một prompt.
	RedrawThreshold = 2
	MaxTextRunes    = 200
	maxExamples     = 5
)

var (
	ErrChannelNotFound = errors.New("không tìm thấy kênh")
	ErrRuleNotFound    = errors.New("không tìm thấy quy tắc")
	ErrRuleLimit       = errors.New("kênh đã có đủ 20 quy tắc")
	ErrInvalid         = errors.New("quy tắc không hợp lệ")
)

type Example struct {
	Before string `json:"before"`
	After  string `json:"after"`
	JobID  string `json:"jobId"`
}

// LearnedFrom ghi nguồn của quy tắc; Kind là edits | review | perf | manual.
type LearnedFrom struct {
	Kind     string    `json:"kind"`
	Count    int       `json:"count"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Examples []Example `json:"examples"`
}

type Rule struct {
	ID            string
	ChannelID     *string
	ImagePromptID *string
	Agent         string
	Text          string
	PatternKey    string
	Source        string
	SourceLabel   string
	LearnedFrom   LearnedFrom
	Enabled       bool
	CreatedAt     time.Time
}

// Label là dòng nguồn hiện dưới quy tắc.
func (r Rule) Label() string {
	switch r.Source {
	case SourceEdits:
		return fmt.Sprintf("Học từ %d lần bạn sửa · %s", r.LearnedFrom.Count, dateRange(r.LearnedFrom.From, r.LearnedFrom.To))
	case SourceRedraw:
		return fmt.Sprintf("Học từ %d góp ý vẽ lại · %s", r.LearnedFrom.Count, dateRange(r.LearnedFrom.From, r.LearnedFrom.To))
	case SourcePerformance:
		if r.SourceLabel != "" {
			return r.SourceLabel
		}
		return "Từ số liệu"
	case SourceAssistant:
		return "Bạn dặn trợ lý"
	}
	return "Bạn thêm"
}

// WireKind đổi nguồn sang learnedFrom.kind của API.
func (r Rule) WireKind() string {
	switch r.Source {
	case SourceEdits:
		return "edits"
	case SourceRedraw:
		return "review"
	case SourcePerformance:
		return "perf"
	}
	return "manual"
}

func dateRange(from, to time.Time) string {
	from, to = from.Local(), to.Local()
	if from.IsZero() || to.IsZero() {
		return ""
	}
	fy, fm, fd := from.Date()
	ty, tm, td := to.Date()
	switch {
	case fy == ty && fm == tm && fd == td:
		return fmt.Sprintf("%d/%d", fd, int(fm))
	case fy == ty && fm == tm:
		return fmt.Sprintf("%d–%d/%d", fd, td, int(fm))
	}
	return fmt.Sprintf("%d/%d–%d/%d", fd, int(fm), td, int(tm))
}

// RuleInput tạo quy tắc theo kênh bằng tay, từ trợ lý hoặc từ số liệu.
type RuleInput struct {
	ChannelID string
	Field     string
	Text      string
	Source    string
	// ImagePromptID tạo ghi chú cho một prompt ảnh thay cho kênh.
	ImagePromptID string
	Agent         string
	LearnedFrom   *LearnedFrom
}

type RuleFilter struct {
	ChannelID     string
	ImagePromptID string
	// OnlyEnabled chỉ lấy quy tắc đang bật.
	OnlyEnabled bool
}

// Candidate là một kiểu sửa đã lặp đủ số lần nhưng chưa thành quy tắc.
type Candidate struct {
	Key             string
	Agent           string
	ChannelID       string
	ImagePromptID   string
	ImagePromptName string
	Text            string
	// Verb: “bỏ phần trong ngoặc ở cuối tiêu đề” (cho câu “lần thứ 3 bạn …”).
	Verb     string
	Count    int
	From, To time.Time
	Examples []Example
}

func (c Candidate) Source() string {
	if c.Agent == AgentArtist {
		return SourceRedraw
	}
	return SourceEdits
}

// MetadataEdit là một lần chủ kênh lưu sửa metadata, kèm bản Đạo diễn viết.
type MetadataEdit struct {
	JobID     string
	ChannelID string
	Baseline  Content
	Current   Content
}
type Content struct {
	Title       string
	Description string
	Tags        []string
}

// Feedback là một lần vẽ lại kèm góp ý (một lời gọi = một lần, dù nhiều ảnh).
type Feedback struct {
	JobID           string
	ChannelID       string
	ImagePromptID   string
	ImagePromptName string
	Text            string
}

type EditRow struct {
	JobID, ChannelID, Part, Key string
	Before, After               string
	At                          time.Time
}
type EditStat struct {
	ChannelID string
	Key       string
	Jobs      int
	First     time.Time
	Last      time.Time
	Examples  []Example
	Afters    []string
}
type FeedbackRow struct {
	JobID, ChannelID, ImagePromptID, Text, Norm string
	At                                          time.Time
}
type FeedbackStat struct {
	ImagePromptID string
	Norm          string
	Count         int
	First, Last   time.Time
	Examples      []Example
}

type Store interface {
	ListRules(context.Context, RuleFilter) ([]Rule, error)
	GetRule(context.Context, string) (Rule, bool, error)
	// InsertRule trả quy tắc trùng chữ (cùng phạm vi) nếu đã có; created=false.
	InsertRule(context.Context, Rule) (Rule, bool, error)
	UpdateRule(ctx context.Context, id string, enabled *bool, text *string) (Rule, bool, error)
	DeleteRule(context.Context, string) (bool, error)
	ChannelExists(context.Context, string) (bool, error)
	ReplaceEdits(ctx context.Context, jobID, channelID string, rows []EditRow) error
	EditStats(ctx context.Context, minJobs int) ([]EditStat, error)
	OtherEdits(ctx context.Context, channelID string, since time.Time, limit int) ([]EditRow, error)
	AddFeedback(context.Context, FeedbackRow) error
	FeedbackStats(ctx context.Context, minCount int) ([]FeedbackStat, error)
	FeedbackNorms(ctx context.Context, imagePromptID string) ([]string, error)
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
