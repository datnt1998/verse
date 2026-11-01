package suggest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/learn"
)

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func (s *Service) hotDrafts(ctx context.Context, now time.Time) ([]Draft, error) {
	hot, err := s.Store.HotVideos(ctx, now)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, h := range hot {
		if h.Queued && h.QueueRank <= 1 {
			continue // đã ở đầu hàng, không còn gì để đề xuất
		}
		d := Draft{
			Kind: KindHotVideo, Agent: "scout", ChannelID: h.ChannelID, ChannelName: h.ChannelName, Object: h.VideoID,
			Value: kindValue[KindHotVideo] + int(h.Score*10), TTL: defaultExpiry,
		}
		facts := []insight.Fact{
			{Label: "Điểm nổi trội", Value: ratio(h.Score), Source: "Lượt xem / mức thường của " + h.RivalTitle},
			{Label: "Lượt xem", Value: fmt.Sprint(h.Views), Source: "YouTube"},
		}
		if h.Queued {
			d.Title = fmt.Sprintf("Đưa “%s” lên đầu hàng", shortTitle(h.Title))
			d.Evidence = fmt.Sprintf("%s · %s · đang thứ %d ở cuối hàng", h.RivalTitle, ratio(h.Score), h.QueueRank)
			d.Reason = insight.Reason{Agent: "scout", Text: fmt.Sprintf("Lượt xem gấp %s mức thường của %s; video đang thứ %d trong hàng của %s.", ratio(h.Score), h.RivalTitle, h.QueueRank, h.ChannelName), Facts: facts}
			d.Primary = Primary{Label: "Lên đầu", Op: "prioritize", Params: mustJSON(map[string]any{"channelId": h.ChannelID, "queueId": h.QueueID, "videoId": h.VideoID})}
		} else {
			d.Title = fmt.Sprintf("Làm “%s” · %s", shortTitle(h.Title), ratio(h.Score))
			d.Evidence = fmt.Sprintf("%s · chưa có trong hàng của %s", h.RivalTitle, h.ChannelName)
			d.Reason = insight.Reason{Agent: "scout", Text: fmt.Sprintf("Lượt xem gấp %s mức thường của %s và chưa có trong hàng của %s.", ratio(h.Score), h.RivalTitle, h.ChannelName), Facts: facts}
			d.Primary = Primary{Label: "Làm video này", Op: "pick", Params: mustJSON(map[string]any{"channelId": h.ChannelID, "videoId": h.VideoID})}
			if s.Ops != nil {
				if n, err := s.Ops.PlanImages(ctx, h.ChannelID, h.VideoID); err == nil && n != nil {
					d.Primary.Cost = &Cost{Images: *n}
				}
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) failingDrafts(ctx context.Context, now time.Time) ([]Draft, error) {
	list, err := s.Store.FailingTemplates(ctx, now)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, f := range list {
		if f.Failed < FailingMin || f.Failed*2 < f.Total {
			continue
		}
		since := f.Since.Local().Format("2/1")
		out = append(out, Draft{
			Kind: KindTemplateFail, Agent: "qa", ChannelID: f.ChannelID, ChannelName: f.ChannelName, Object: f.TemplateID,
			Title:    fmt.Sprintf("Template %s: %d video chưa đạt Độ hoàn thiện để xuất", f.Template, f.Failed),
			Evidence: fmt.Sprintf("%s · %d/%d video %s từ %s", f.ChannelName, f.Failed, f.Total, f.Template, since),
			Reason: insight.Reason{Agent: "qa", Text: fmt.Sprintf("%d/%d video %s từ %s chưa đạt Độ hoàn thiện để xuất.", f.Failed, f.Total, f.Template, since), Facts: []insight.Fact{
				{Label: "Video chưa đạt", Value: fmt.Sprintf("%d/%d", f.Failed, f.Total), Source: "Kiểm định, 7 ngày gần nhất"},
			}},
			Primary: Primary{Label: "Mở template", Op: "open_template", Params: mustJSON(map[string]any{"templateId": f.TemplateID}), Href: "#/templates/" + f.TemplateID},
			Value:   kindValue[KindTemplateFail] + f.Failed, TTL: defaultExpiry,
		})
	}
	return out, nil
}

func (s *Service) learnDrafts(ctx context.Context) ([]Draft, error) {
	if s.Learn == nil {
		return nil, nil
	}
	cands, err := s.Learn.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	names, err := s.Store.ChannelNames(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Draft, 0, len(cands))
	for _, c := range cands {
		d, err := s.learnDraft(ctx, c)
		if err != nil {
			return nil, err
		}
		d.ChannelName = names[c.ChannelID]
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) learnDraft(ctx context.Context, c learn.Candidate) (Draft, error) {
	names, err := s.Store.ChannelNames(ctx)
	if err != nil {
		return Draft{}, err
	}
	scope := map[string]string{}
	object := ""
	agent := "director"
	who := names[c.ChannelID]
	if c.Agent == learn.AgentArtist {
		scope["imagePromptId"] = c.ImagePromptID
		object = "p:" + c.ImagePromptID + ":" + c.Key
		agent = "artist"
		if c.ImagePromptName != "" {
			who = c.ImagePromptName
		}
	} else {
		scope["channelId"] = c.ChannelID
		object = c.Key
	}
	jobIDs := make([]string, 0, len(c.Examples))
	for _, e := range c.Examples {
		jobIDs = append(jobIDs, e.JobID)
	}
	kind := "edits"
	evidence := fmt.Sprintf("%s · %d lần bạn sửa", who, c.Count)
	text := fmt.Sprintf("%d lần bạn %s.", c.Count, c.Verb)
	banner := fmt.Sprintf("Đạo diễn nhận ra: lần thứ %d bạn %s. Lưu thành quy tắc cho %s?", c.Count, c.Verb, who)
	label := "Lưu quy tắc"
	if c.Agent == learn.AgentArtist {
		kind = "review"
		evidence = fmt.Sprintf("%s · %d góp ý vẽ lại giống nhau", who, c.Count)
		text = fmt.Sprintf("%d lần bạn góp ý vẽ lại giống nhau.", c.Count)
		banner = fmt.Sprintf("Hoạ sĩ nhận ra: góp ý “%s” lần thứ %d với prompt %s. Thêm vào Ghi chú đã học của prompt?", c.Text, c.Count, who)
		label = "Thêm vào Ghi chú đã học"
	}
	params := map[string]any{
		"scope": scope, "agent": c.Agent, "text": c.Text, "key": c.Key, "jobIds": jobIDs,
		"learnedFrom": map[string]any{"kind": kind, "count": c.Count, "from": c.From, "to": c.To, "examples": nonNilExamples(c.Examples)},
	}
	if c.ImagePromptName != "" {
		params["promptName"] = c.ImagePromptName
	}
	if c.Text == "" {
		return Draft{}, fmt.Errorf("quy tắc rỗng cho kiểu sửa %s", c.Key)
	}
	return Draft{
		Kind: KindLearnRule, Agent: agent, ChannelID: c.ChannelID, ChannelName: names[c.ChannelID], Object: object,
		Title:    fmt.Sprintf("Lưu quy tắc “%s”?", c.Text),
		Evidence: evidence,
		Banner:   banner,
		Reason:   insight.Reason{Agent: agent, Text: text, Facts: []insight.Fact{{Label: "Số lần", Value: fmt.Sprint(c.Count), Source: "Lịch sử sửa của bạn"}}},
		Primary:  Primary{Label: label, Op: "add_rule", Params: mustJSON(params)},
		Value:    kindValue[KindLearnRule] + c.Count, TTL: defaultExpiry,
	}, nil
}

func nonNilExamples(e []learn.Example) []learn.Example {
	if e == nil {
		return []learn.Example{}
	}
	return e
}

func (s *Service) paceDrafts(ctx context.Context, now time.Time) ([]Draft, error) {
	list, err := s.Store.PaceFull(ctx, now)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, p := range list {
		if p.Days < PaceDays || p.Queued < PaceMinQueued || p.Pace >= 50 {
			continue
		}
		next := p.Pace + 1
		out = append(out, Draft{
			Kind: KindPace, Agent: "scout", ChannelID: p.ChannelID, ChannelName: p.ChannelName, Object: "pace",
			Title:    fmt.Sprintf("Tăng nhịp %s lên %d video/ngày?", p.ChannelName, next),
			Evidence: fmt.Sprintf("%s · đủ nhịp %d ngày liền · còn %d video trong hàng", p.ChannelName, p.Days, p.Queued),
			Reason: insight.Reason{Agent: "scout", Text: fmt.Sprintf("%d ngày liền %s làm đủ %d video trước 12:00 và hàng còn %d video chờ.", p.Days, p.ChannelName, p.Pace, p.Queued), Facts: []insight.Fact{
				{Label: "Nhịp hiện tại", Value: fmt.Sprintf("%d video/ngày", p.Pace), Source: "Cài đặt kênh"},
				{Label: "Hàng chờ", Value: fmt.Sprintf("%d video", p.Queued), Source: "Hàng chờ"},
			}},
			Primary: Primary{Label: "Tăng nhịp", Op: "set_pace", Params: mustJSON(map[string]any{"channelId": p.ChannelID, "pace": next})},
			Value:   kindValue[KindPace], TTL: defaultExpiry,
		})
	}
	return out, nil
}

func (s *Service) autonomyDrafts(ctx context.Context, names map[string]string) ([]Draft, error) {
	list, err := s.Autonomy.RaiseCandidates(ctx)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, r := range list {
		if r.CurrentMode != "ask" || r.Streak < RaiseAutonomyRun {
			continue
		}
		name := r.ChannelName
		if name == "" {
			name = names[r.ChannelID]
		}
		min := r.Min
		if min == 0 {
			min = 85
		}
		out = append(out, Draft{
			Kind: KindRaiseAutonomy, Agent: "qa", ChannelID: r.ChannelID, ChannelName: name, Object: "approve",
			Title:    fmt.Sprintf("Bật Duyệt theo điểm ≥ %d cho %s?", min, name),
			Evidence: fmt.Sprintf("%d video liền không vẽ lại ảnh", r.Streak),
			Reason: insight.Reason{Agent: "qa", Text: fmt.Sprintf("%s: %d video gần nhất bạn duyệt không vẽ lại ảnh nào, Kiểm định chấm mọi ảnh từ %d trở lên.", name, r.Streak, min), Facts: []insight.Fact{
				{Label: "Video liền không vẽ lại", Value: fmt.Sprint(r.Streak), Source: "Lịch sử duyệt"},
				{Label: "Ngưỡng điểm", Value: fmt.Sprint(min), Source: "Kiểm định"},
			}},
			Primary: Primary{Label: "Bật duyệt theo điểm", Op: "set_autonomy", Params: mustJSON(map[string]any{"channelId": r.ChannelID, "mode": "score", "min": min})},
			Value:   kindValue[KindRaiseAutonomy], TTL: defaultExpiry,
		})
	}
	return out, nil
}

func (s *Service) perfDrafts(ctx context.Context, names map[string]string) ([]Draft, error) {
	list, err := s.Perf.OpenInsights(ctx)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, p := range list {
		name := names[p.ChannelID]
		ev := fmt.Sprintf("%s · %d video", name, p.Videos)
		if p.Preliminary {
			ev = fmt.Sprintf("%s · Sơ bộ · %d video", name, p.Videos)
		}
		href := "#/channels/" + p.ChannelID + "?tab=videos"
		out = append(out, Draft{
			Kind: KindPerfInsight, Agent: "scout", ChannelID: p.ChannelID, ChannelName: name, Object: p.InsightID,
			Title:    p.Text,
			Evidence: ev,
			Reason:   insight.Reason{Agent: "scout", Text: p.Text, Facts: []insight.Fact{{Label: "Số video có số liệu", Value: fmt.Sprint(p.Videos), Source: "Số liệu sau đăng"}}},
			Primary:  Primary{Label: "Xem", Op: "open_insight", Params: mustJSON(map[string]any{"channelId": p.ChannelID, "insightId": p.InsightID}), Href: href},
			Value:    kindValue[KindPerfInsight], TTL: defaultExpiry,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ChannelID < out[j].ChannelID })
	return out, nil
}

func (s *Service) cleanupDrafts(ctx context.Context) ([]Draft, error) {
	n, count, err := s.Cleanup.PostedMP4(ctx)
	if err != nil {
		return nil, err
	}
	if n < CleanupMinBytes || count == 0 {
		return nil, nil
	}
	return []Draft{{
		Kind: KindCleanup, Agent: "qa", Object: "mp4",
		Title:    fmt.Sprintf("Dọn MP4 của video đã đăng, lấy lại %s?", FormatBytes(n)),
		Evidence: fmt.Sprintf("%d video đã đăng còn giữ file MP4", count),
		Reason: insight.Reason{Agent: "qa", Text: fmt.Sprintf("%d video đã đăng còn giữ file MP4, chiếm %s.", count, FormatBytes(n)), Facts: []insight.Fact{
			{Label: "Dung lượng", Value: FormatBytes(n), Source: "Thư mục lưu dự án"},
		}},
		Primary: Primary{Label: "Dọn MP4", Op: "cleanup", Params: mustJSON(map[string]any{"kind": "mp4"})},
		Value:   kindValue[KindCleanup], TTL: defaultExpiry,
	}}, nil
}
