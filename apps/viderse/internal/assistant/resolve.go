package assistant

import (
	"context"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
)

// scopeData là dữ liệu thật của phạm vi, đọc một lần cho mỗi câu: dùng để nói
// với mô hình có những kênh/đối thủ nào và để đổi TÊN mô hình nêu thành id.
type scopeData struct {
	kind     string
	jobID    string
	job      *domain.CloneJobView
	channels []channels.ChannelView
	scenes   int
}

func (s *Service) loadScope(ctx context.Context, sc Scope) (*scopeData, error) {
	d := &scopeData{kind: sc.kind(), jobID: sc.JobID}
	switch d.kind {
	case "job":
		job, err := s.d.Jobs.Get(ctx, sc.JobID)
		if err != nil || job == nil {
			return nil, fail(CodeNotFound, "Không tìm thấy video này.")
		}
		d.job = job
		if job.ChannelID != "" {
			if ch, err := s.d.Channels.Get(ctx, job.ChannelID); err == nil {
				d.channels = []channels.ChannelView{ch}
			}
		}
		if view, err := s.d.Images.Read(ctx, sc.JobID); err == nil {
			d.scenes = len(sceneItems(view))
		}
	case "channel":
		ch, err := s.d.Channels.Get(ctx, sc.ChannelID)
		if err != nil {
			return nil, fail(CodeNotFound, "Không tìm thấy kênh này.")
		}
		d.channels = []channels.ChannelView{ch}
	default:
		list, err := s.d.Channels.List(ctx)
		if err != nil {
			return nil, err
		}
		d.channels = list
	}
	return d, nil
}

func (d *scopeData) payload(text string) classifyPayload {
	ctx := classifyContext{Scope: d.kind, SceneCount: d.scenes, Channels: []payloadChannel{}}
	if d.job != nil {
		ctx.Video = clip(cleanLine(d.job.Source.Title), 120)
	}
	for _, c := range d.channels {
		pc := payloadChannel{Name: chanName(c), Rivals: []string{}}
		for _, src := range c.Sources {
			pc.Rivals = append(pc.Rivals, clip(cleanLine(src.Title), 60))
		}
		ctx.Channels = append(ctx.Channels, pc)
	}
	return classifyPayload{UserText: text, Context: ctx}
}

// resolved là kết quả đổi một bước của mô hình thành params thật, hoặc một câu
// hỏi/thông báo chưa làm được.
type resolved struct {
	steps []resolvedStep
	reply *Reply
}

type resolvedStep struct {
	intent IntentID
	params any
}

func one(intent IntentID, params any) resolved {
	return resolved{steps: []resolvedStep{{intent, params}}}
}

func replyOf(r *Reply) resolved { return resolved{reply: r} }

// resolveStep đổi một bước mô hình thành params thật. Mô hình chỉ nêu tên và
// số; id luôn đến từ dữ liệu thật của phạm vi.
func (s *Service) resolveStep(ctx context.Context, d *scopeData, text string, m modelStep) resolved {
	id := IntentID(m.Intent)
	spec, ok := specOf(id)
	if !ok {
		return replyOf(s.unknown(d, unsupportedOther))
	}
	if spec.level == jobLevel {
		if d.kind != "job" {
			return replyOf(s.unknown(d, unsupportedOther))
		}
		return s.resolveJob(ctx, d, id, m, text)
	}
	ch, ask := s.pickChannel(d, text, m.Channel)
	if ask != nil {
		return replyOf(&Reply{Ask: ask})
	}
	if ch == nil {
		return replyOf(s.unknown(d, unsupportedOther))
	}
	switch id {
	case SetPace:
		return one(id, paceParams{ChannelID: ch.Channel.ID, PerDay: *m.Count})
	case SetAutonomy:
		return one(id, autonomyParams{ChannelID: ch.Channel.ID, Preset: *m.Preset})
	case AddRule:
		return one(id, ruleParams{ChannelID: ch.Channel.ID, Text: cleanLine(*m.Rule)})
	case FilterAuto:
		src, ask := s.pickRival(*ch, text, m.Rival)
		if ask != nil {
			return replyOf(&Reply{Ask: ask})
		}
		if src == nil {
			return replyOf(s.noRivals(d, *ch))
		}
		return one(id, filterParams{ChannelID: ch.Channel.ID, RivalID: src.RivalID, MinDurationSec: *m.MinDurationMinutes * 60})
	case MakeLatest, MakeVideos:
		return s.resolveMake(ctx, d, *ch, text, id, m)
	case Prioritize:
		return s.resolvePrioritize(ctx, d, *ch, text, m)
	}
	return replyOf(s.unknown(d, unsupportedOther))
}

func (s *Service) resolveJob(ctx context.Context, d *scopeData, id IntentID, m modelStep, text string) resolved {
	jp := jobParams{JobID: d.jobID}
	feedback := ""
	if m.Feedback != nil {
		feedback = cleanLine(*m.Feedback)
	}
	switch id {
	case RedrawImage, SkipImage:
		view, err := s.d.Images.Read(ctx, d.jobID)
		if err != nil {
			return replyOf(s.sceneMissing(d, *m.SceneNo))
		}
		for _, it := range sceneItems(view) {
			if it.Kind == "scene" && it.No == *m.SceneNo {
				return one(id, sceneParams{JobID: d.jobID, ItemID: it.ID, SceneNo: it.No, Feedback: feedback})
			}
		}
		return replyOf(s.sceneMissing(d, *m.SceneNo))
	case RedrawAll:
		view, err := s.d.Images.Read(ctx, d.jobID)
		if err != nil || len(view.Items) == 0 {
			return replyOf(s.unknown(d, unsupportedOther))
		}
		return one(id, redrawAllParams{JobID: d.jobID, Count: len(view.Items), Feedback: feedback})
	case EditMetadata:
		parts := m.Parts
		if len(parts) == 0 {
			parts = []string{"title", "description", "tags"}
		}
		return one(id, metadataParams{JobID: d.jobID, Parts: parts, Direction: feedback})
	case SetDrawSource:
		if m.DrawSource == nil || strings.TrimSpace(*m.DrawSource) == "" {
			return replyOf(s.unknown(d, unsupportedOther))
		}
		return one(id, drawSourceParams{JobID: d.jobID, Source: strings.ToLower(cleanLine(*m.DrawSource))})
	case MarkPosted:
		// Link (nếu có) đọc từ chính câu người dùng, không lấy từ mô hình.
		return one(id, postedParams{JobID: d.jobID, Link: youTubeLink(text)})
	default:
		return one(id, jp)
	}
}

// pickChannel chọn kênh mà bước nói tới. Phạm vi video/kênh đã cố định kênh;
// phạm vi "all" lấy theo tên, một kênh duy nhất thì chọn luôn, còn lại hỏi.
func (s *Service) pickChannel(d *scopeData, text string, said *string) (*channels.ChannelView, *Ask) {
	if d.kind != "all" {
		if len(d.channels) == 0 {
			return nil, nil
		}
		return &d.channels[0], nil
	}
	if said != nil && strings.TrimSpace(*said) != "" {
		var hits []int
		for i, c := range d.channels {
			if matchName(*said, c.Channel.Name) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 1 {
			return &d.channels[hits[0]], nil
		}
		if len(hits) > 1 {
			return nil, channelAsk(d, text, hits)
		}
	}
	switch len(d.channels) {
	case 0:
		return nil, nil
	case 1:
		return &d.channels[0], nil
	}
	all := make([]int, len(d.channels))
	for i := range all {
		all[i] = i
	}
	return nil, channelAsk(d, text, all)
}

func askOption(text, label string) Option {
	return Option{Label: label, Text: clip(text, MaxTextRunes-len([]rune(label))-12) + " · kênh " + label}
}

func channelAsk(d *scopeData, text string, idx []int) *Ask {
	if len(idx) > 4 {
		idx = idx[:4]
	}
	a := &Ask{Question: "Bạn muốn làm việc này cho kênh nào?"}
	for _, i := range idx {
		a.Options = append(a.Options, askOption(text, chanName(d.channels[i])))
	}
	return a
}

// pickRival chọn đối thủ (đã theo dõi) mà bước nói tới.
func (s *Service) pickRival(ch channels.ChannelView, text string, said *string) (*domain.ChannelSource, *Ask) {
	if len(ch.Sources) == 0 {
		return nil, nil
	}
	var hits []int
	if said != nil && strings.TrimSpace(*said) != "" {
		for i, src := range ch.Sources {
			if matchName(*said, src.Title) || (src.Handle != nil && matchName(*said, *src.Handle)) {
				hits = append(hits, i)
			}
		}
		if len(hits) == 1 {
			return &ch.Sources[hits[0]], nil
		}
	}
	if len(hits) == 0 {
		if len(ch.Sources) == 1 {
			return &ch.Sources[0], nil
		}
		for i := range ch.Sources {
			hits = append(hits, i)
		}
	}
	if len(hits) > 4 {
		hits = hits[:4]
	}
	a := &Ask{Question: "Bạn muốn lấy từ đối thủ nào?"}
	for _, i := range hits {
		a.Options = append(a.Options, askOption(text, cleanLine(ch.Sources[i].Title)))
	}
	return nil, a
}

func (s *Service) noRivals(d *scopeData, ch channels.ChannelView) *Reply {
	u := s.unknown(d, unsupportedOther)
	u.Unknown.Lead = fmt.Sprintf("%s chưa theo dõi đối thủ nào nên trợ lý chưa có video để làm. Thêm đối thủ ở Kênh › Nguồn & tự động. Có phải bạn muốn:", chanName(ch))
	return u
}

func (s *Service) sceneMissing(d *scopeData, no int) *Reply {
	u := s.unknown(d, unsupportedOther)
	u.Unknown.Lead = fmt.Sprintf("Video này không có cảnh %d. Có phải bạn muốn:", no)
	return u
}

// resolveMake tìm video của đối thủ theo cách người dùng chọn (mới nhất, nổi
// nhất tuần, nổi nhất mọi thời, theo tên) và bỏ các video đã có trong hàng hoặc
// đã làm — mỗi video là MỘT bước.
func (s *Service) resolveMake(ctx context.Context, d *scopeData, ch channels.ChannelView, text string, id IntentID, m modelStep) resolved {
	src, ask := s.pickRival(ch, text, m.Rival)
	if ask != nil {
		return replyOf(&Reply{Ask: ask})
	}
	if src == nil {
		return replyOf(s.noRivals(d, ch))
	}
	pick := pickNewest
	if m.Pick != nil && id == MakeVideos {
		pick = *m.Pick
	}
	want := 1
	if m.Count != nil && *m.Count > 1 {
		want = *m.Count
	}
	if want > MaxSteps {
		return replyOf(s.tooMany(d))
	}
	query := rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 20}
	switch pick {
	case pickTopWeek:
		query.Sort, query.WithinDays = rivals.SortOutlier, 7
	case pickTopAll:
		query.Sort = rivals.SortOutlier
	case pickTitle:
		if m.VideoTitle == nil || strings.TrimSpace(*m.VideoTitle) == "" {
			return replyOf(s.unknown(d, unsupportedOther))
		}
		query.Query = cleanLine(*m.VideoTitle)
	}
	page, err := s.d.Rivals.Videos(ctx, src.RivalID, query)
	if err != nil || page == nil || len(page.Videos) == 0 {
		u := s.unknown(d, unsupportedOther)
		u.Unknown.Lead = fmt.Sprintf("Trợ lý chưa thấy video nào của %s hợp với câu này. Có phải bạn muốn:", cleanLine(src.Title))
		return replyOf(u)
	}
	out := resolved{}
	for _, v := range page.Videos {
		if len(out.steps) >= want {
			break
		}
		params := makeParams{ChannelID: ch.Channel.ID, RivalID: src.RivalID, VideoID: v.ID, Title: v.Title}
		// Bỏ video đã có trong hàng/đã làm: hỏi chính service hàng chờ.
		if _, err := s.prepare(ctx, id, mustParams(params)); err != nil {
			continue
		}
		out.steps = append(out.steps, resolvedStep{id, params})
	}
	if len(out.steps) == 0 {
		u := s.unknown(d, unsupportedOther)
		u.Unknown.Lead = fmt.Sprintf("Các video của %s hợp với câu này đã có trong hàng hoặc đã làm rồi. Có phải bạn muốn:", cleanLine(src.Title))
		return replyOf(u)
	}
	return out
}

// resolvePrioritize tìm video ĐANG trong hàng của kênh theo cách người dùng
// chọn; không có thì nói thật.
func (s *Service) resolvePrioritize(ctx context.Context, d *scopeData, ch channels.ChannelView, text string, m modelStep) resolved {
	view, err := s.d.Queue.List(ctx)
	if err != nil {
		return replyOf(s.unknown(d, unsupportedOther))
	}
	var items []domain.CloneQueueItemView
	for _, it := range view.Items {
		if it.ChannelID == ch.Channel.ID {
			items = append(items, it)
		}
	}
	notFound := func() resolved {
		u := s.unknown(d, unsupportedOther)
		u.Unknown.Lead = fmt.Sprintf("Trợ lý không thấy video đó trong hàng chờ của %s. Có phải bạn muốn:", chanName(ch))
		return replyOf(u)
	}
	pick := ""
	if m.Pick != nil {
		pick = *m.Pick
	}
	if pick == pickTitle {
		if m.VideoTitle == nil {
			return notFound()
		}
		for _, it := range items {
			if matchName(*m.VideoTitle, it.Title) {
				return one(Prioritize, prioritizeParams{ChannelID: ch.Channel.ID, QueueItemID: it.ID, Title: it.Title})
			}
		}
		return notFound()
	}
	// newest / top_week / top_all: lấy video theo cách chọn của đối thủ, rồi
	// tìm cái nằm trong hàng.
	var srcs []domain.ChannelSource
	if m.Rival != nil && strings.TrimSpace(*m.Rival) != "" {
		src, ask := s.pickRival(ch, text, m.Rival)
		if ask != nil {
			return replyOf(&Reply{Ask: ask})
		}
		if src != nil {
			srcs = []domain.ChannelSource{*src}
		}
	} else {
		srcs = ch.Sources
	}
	query := rivals.VideoQuery{Sort: rivals.SortNewest, Limit: 20}
	switch pick {
	case pickTopWeek:
		query.Sort, query.WithinDays = rivals.SortOutlier, 7
	case pickTopAll:
		query.Sort = rivals.SortOutlier
	}
	for _, src := range srcs {
		page, err := s.d.Rivals.Videos(ctx, src.RivalID, query)
		if err != nil || page == nil {
			continue
		}
		for _, v := range page.Videos {
			for _, it := range items {
				if strings.Contains(it.SourceURL, v.ID) {
					return one(Prioritize, prioritizeParams{ChannelID: ch.Channel.ID, QueueItemID: it.ID, Title: it.Title})
				}
			}
		}
	}
	return notFound()
}

// ── chưa làm được ───────────────────────────────────────────────────────────

func (s *Service) tooMany(d *scopeData) *Reply {
	u := s.unknown(d, unsupportedOther)
	u.Unknown.Lead = fmt.Sprintf("Mỗi lần trợ lý làm tối đa %d bước. Hãy chia câu thành các phần nhỏ hơn. Có phải bạn muốn:", MaxSteps)
	return u
}

// unknown dựng thẻ “Chưa làm được việc này”: câu dẫn theo lý do + ĐÚNG 2 gợi ý,
// mỗi gợi ý là một câu hợp lệ cho phạm vi hiện tại (không bao giờ là việc cũng
// không làm được).
func (s *Service) unknown(d *scopeData, kind string) *Reply {
	u := &Unknown{Suggestions: suggestionsFor(d)}
	if kind == unsupportedDelete {
		u.Lead = "Trợ lý chỉ làm việc trên video đang mở. Xoá kênh nằm ở Kênh › Hồ sơ kênh. Có phải bạn muốn:"
		if len(d.channels) == 1 {
			u.Link = &Link{Label: "Mở Kênh › Hồ sơ kênh", Href: "#/channels/" + d.channels[0].Channel.ID + "?tab=settings"}
		}
	} else {
		u.Lead = "Trợ lý chưa làm được việc này. Có phải bạn muốn:"
	}
	return &Reply{Unknown: u}
}

func suggestionsFor(d *scopeData) []Suggestion {
	switch d.kind {
	case "job":
		return []Suggestion{
			{Label: "Tạm dừng video này", Text: "Tạm dừng video này", Icon: "pause"},
			{Label: "Đổi nền khác", Text: "Đổi nền khác cho video này", Icon: "gallery"},
		}
	case "channel":
		out := []Suggestion{{Label: "Mỗi ngày làm 3 video", Text: "Mỗi ngày làm 3 video thôi", Icon: "calendar"}}
		if len(d.channels) == 1 && len(d.channels[0].Sources) > 0 {
			r := cleanLine(d.channels[0].Sources[0].Title)
			return append(out, Suggestion{Label: "Làm video mới nhất của " + r, Text: "Làm video mới nhất của " + r, Icon: "add"})
		}
		return append(out, Suggestion{Label: "Mức tự chủ Cân bằng", Text: "Đặt mức tự chủ Cân bằng", Icon: "shield"})
	default:
		name := "kênh của tôi"
		if len(d.channels) > 0 {
			name = chanName(d.channels[0])
		}
		return []Suggestion{
			{Label: "Mỗi ngày làm 3 video", Text: "Mỗi ngày làm 3 video thôi cho " + name, Icon: "calendar"},
			{Label: "Mức tự chủ Cân bằng", Text: "Đặt mức tự chủ Cân bằng cho " + name, Icon: "shield"},
		}
	}
}
