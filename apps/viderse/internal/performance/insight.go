package performance

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Video là một video đã đăng có số 7 ngày — đầu vào của phép so sánh.
type Video struct {
	JobID        string
	Title        string
	TemplateID   string
	TemplateName string
	DurationSec  int
	At           time.Time // lúc đăng trên YouTube nếu biết, không thì lúc đánh dấu
	Views7d      int64
}

// bucket là một nhóm so sánh của một chiều.
type bucket struct {
	key   string
	label string // dạng chữ thường dùng giữa câu
	name  string // nhãn nhóm hiện trong thanh so sánh
	views []int64
	apply *Apply
}

// Median trả trung vị; rỗng → 0.
func Median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// Analyze so các nhóm theo template, kiểu tiêu đề, độ dài và giờ đăng. Chỉ tính
// video đã có số 7 ngày; ít hơn MinVideos thì chưa so (needMore > 0). Trả mọi
// nhận xét đạt ngưỡng, xếp theo mức chênh giảm dần — người gọi tự cắt MaxInsights.
func Analyze(channelID string, videos []Video, loc *time.Location) (insights []Insight, needMore int) {
	if len(videos) < MinVideos {
		return []Insight{}, MinVideos - len(videos)
	}
	if loc == nil {
		loc = time.Local
	}
	preliminary := len(videos) < PreliminaryBelow
	out := []Insight{}
	for _, dim := range []Dimension{DimTemplate, DimTitlePattern, DimLength, DimPostHour} {
		groups := map[string]*bucket{}
		for _, v := range videos {
			b := classify(dim, v, loc)
			if b == nil {
				continue
			}
			g := groups[b.key]
			if g == nil {
				g = b
				groups[b.key] = g
			}
			g.views = append(g.views, v.Views7d)
		}
		if in, ok := compare(channelID, dim, groups, preliminary); ok {
			out = append(out, in)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Ratio > out[j].Ratio })
	return out, 0
}

func compare(channelID string, dim Dimension, groups map[string]*bucket, preliminary bool) (Insight, bool) {
	type scored struct {
		*bucket
		median int64
	}
	qualified := []scored{}
	for _, g := range groups {
		if len(g.views) >= MinGroup {
			qualified = append(qualified, scored{g, Median(g.views)})
		}
	}
	if len(qualified) < 2 {
		return Insight{}, false
	}
	sort.Slice(qualified, func(i, j int) bool {
		if qualified[i].median != qualified[j].median {
			return qualified[i].median > qualified[j].median
		}
		return qualified[i].key < qualified[j].key
	})
	best, worst := qualified[0], qualified[len(qualified)-1]
	if worst.median <= 0 {
		return Insight{}, false
	}
	ratio := float64(best.median) / float64(worst.median)
	if ratio < MinRatio {
		return Insight{}, false
	}
	listed := make([]Group, len(qualified))
	for i, g := range qualified {
		listed[i] = Group{Label: g.name, N: len(g.views), Median7d: g.median}
	}
	x := formatRatio(ratio)
	var text string
	switch dim {
	case DimTemplate:
		text = fmt.Sprintf("Template %s được xem gấp %s lần %s", best.label, x, worst.label)
	case DimTitlePattern:
		text = fmt.Sprintf("Tiêu đề %s: lượt xem 7 ngày gấp %s lần", best.label, x)
	case DimLength:
		text = fmt.Sprintf("Video %s: lượt xem 7 ngày gấp %s lần video %s", best.label, x, worst.label)
	default:
		text = fmt.Sprintf("Đăng %s: lượt xem 7 ngày gấp %s lần đăng %s", best.label, x, worst.label)
	}
	return Insight{
		ID: string(dim) + ":" + best.key, ChannelID: channelID, Dimension: dim, Text: text,
		Groups: listed, Preliminary: preliminary, Apply: best.apply, Ratio: ratio,
	}, true
}

// formatRatio viết số kiểu Việt Nam: "1,8".
func formatRatio(r float64) string {
	return strings.Replace(fmt.Sprintf("%.1f", r), ".", ",", 1)
}

func classify(dim Dimension, v Video, loc *time.Location) *bucket {
	switch dim {
	case DimTemplate:
		if v.TemplateID == "" || v.TemplateName == "" {
			return nil
		}
		return &bucket{key: v.TemplateID, label: v.TemplateName, name: v.TemplateName,
			apply: &Apply{Op: "template.rotation", Params: map[string]any{"templateId": v.TemplateID, "weight": 2}}}
	case DimTitlePattern:
		if strings.TrimSpace(v.Title) == "" {
			return nil
		}
		switch titlePattern(v.Title) {
		case "question":
			return &bucket{key: "question", label: "dạng câu hỏi", name: "Dạng câu hỏi",
				apply: ruleApply("Ưu tiên đặt tiêu đề dạng câu hỏi")}
		case "number":
			return &bucket{key: "number", label: "có con số", name: "Có con số",
				apply: ruleApply("Ưu tiên tiêu đề có con số cụ thể")}
		default:
			return &bucket{key: "statement", label: "dạng câu khẳng định", name: "Câu khẳng định",
				apply: ruleApply("Đặt tiêu đề dạng câu khẳng định, tránh câu hỏi")}
		}
	case DimLength:
		switch {
		case v.DurationSec <= 0:
			return nil
		case v.DurationSec < 180:
			return &bucket{key: "short", label: "ngắn dưới 3 phút", name: "Dưới 3 phút"}
		case v.DurationSec < 480:
			return &bucket{key: "mid", label: "dài 3 đến 8 phút", name: "3 đến 8 phút"}
		default:
			return &bucket{key: "long", label: "dài trên 8 phút", name: "Trên 8 phút"}
		}
	default:
		if v.At.IsZero() {
			return nil
		}
		switch h := v.At.In(loc).Hour(); {
		case h >= 5 && h < 11:
			return &bucket{key: "morning", label: "buổi sáng", name: "Buổi sáng (5–11 giờ)"}
		case h >= 11 && h < 17:
			return &bucket{key: "afternoon", label: "buổi chiều", name: "Buổi chiều (11–17 giờ)"}
		case h >= 17 && h < 23:
			return &bucket{key: "evening", label: "buổi tối", name: "Buổi tối (17–23 giờ)"}
		default:
			return &bucket{key: "night", label: "khuya", name: "Khuya (23–5 giờ)"}
		}
	}
}

func ruleApply(text string) *Apply {
	return &Apply{Op: "add_rule", Params: map[string]any{"field": "title", "text": text}}
}

var questionMarks = []string{"?", "tại sao", "vì sao", "làm sao", "như thế nào", "thế nào", "là gì", "có phải", "bao giờ", "ở đâu"}

// titlePattern xếp tiêu đề vào question | number | statement (theo thứ tự ưu tiên).
func titlePattern(title string) string {
	lower := strings.ToLower(title)
	for _, m := range questionMarks {
		if strings.Contains(lower, m) {
			return "question"
		}
	}
	for _, r := range lower {
		if r >= '0' && r <= '9' {
			return "number"
		}
	}
	return "statement"
}
