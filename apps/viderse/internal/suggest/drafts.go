package suggest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
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
