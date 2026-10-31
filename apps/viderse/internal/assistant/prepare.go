package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	clonebackdrop "github.com/datnt1998/verse/apps/viderse/internal/clone/backdrop"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	clonemetadata "github.com/datnt1998/verse/apps/viderse/internal/clone/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// prepared là một bước đã kiểm lại với dữ liệu HIỆN TẠI: thẻ để hiện (step) và
// việc sẽ chạy (run). Cả `plan` (dựng thẻ) lẫn `run` (làm thật) đi qua prepare
// nên một bước không bao giờ chạy trên dữ liệu cũ hơn lúc xác nhận mà không bị
// kiểm lại.
type prepared struct {
	step Step
	run  func(ctx context.Context) (string, error)
}

func stepFail(err error, format string, args ...any) error {
	return &StepError{Message: fmt.Sprintf(format, args...), Err: err}
}

// prepare dựng thẻ + hành động cho một intent từ params (đã phân giải hoặc do
// client gửi) — luôn kiểm lại với service thật.
func (s *Service) prepare(ctx context.Context, id IntentID, raw json.RawMessage) (*prepared, error) {
	spec, ok := specOf(id)
	if !ok {
		return nil, fail(CodeInvalid, "Trợ lý không làm việc này.")
	}
	var (
		p   *prepared
		err error
	)
	switch id {
	case RedrawImage:
		p, err = prepareRedraw(ctx, s, raw, false)
	case SkipImage:
		p, err = prepareRedraw(ctx, s, raw, true)
	case RedrawAll:
		p, err = prepareRedrawAll(ctx, s, raw)
	case EditMetadata:
		p, err = prepareMetadata(ctx, s, raw)
	case RerollBackdrop:
		p, err = prepareBackdrop(ctx, s, raw)
	case NoImages:
		p, err = prepareNoImages(ctx, s, raw)
	case Pause, Resume:
		p, err = preparePause(ctx, s, id, raw)
	case SetPace:
		p, err = preparePace(ctx, s, raw)
	case FilterAuto:
		p, err = prepareFilter(ctx, s, raw)
	case MakeLatest, MakeVideos:
		p, err = prepareMake(ctx, s, id, raw)
	case Prioritize:
		p, err = preparePrioritize(ctx, s, raw)
	case SetAutonomy:
		p, err = prepareAutonomy(ctx, s, raw)
	case AddRule:
		p, err = prepareRule(ctx, s, raw)
	default:
		return nil, fail(CodeInvalid, "Trợ lý không làm việc này.")
	}
	if err != nil {
		return nil, err
	}
	p.step.Intent = id
	p.step.Params = raw
	if p.step.Agent == "" {
		p.step.Agent = spec.agent
	}
	if p.step.Reruns == nil {
		p.step.Reruns = []Rerun{}
	}
	return p, nil
}

func badParams(err error) error {
	return fail(CodeInvalid, "%s", err.Error())
}

// ── video (job) ─────────────────────────────────────────────────────────────

var digits = regexp.MustCompile(`\d+`)

type sceneItem struct {
	ID     string
	No     int
	Kind   string
	Status string
}

// sceneItems đánh số cảnh đúng như màn Duyệt ảnh: số trong sceneId, không có
// thì theo thứ tự (từ 1).
func sceneItems(v cloneimages.View) []sceneItem {
	out := make([]sceneItem, 0, len(v.Items))
	for i, it := range v.Items {
		no := i + 1
		if m := digits.FindString(it.SceneID); m != "" {
			if n, err := strconv.Atoi(m); err == nil && n > 0 {
				no = n
			}
		}
		out = append(out, sceneItem{ID: it.ID, No: no, Kind: it.Kind, Status: it.Status})
	}
	return out
}

func (s *Service) job(ctx context.Context, id string) (*domain.CloneJobView, error) {
	job, err := s.d.Jobs.Get(ctx, id)
	if err != nil || job == nil {
		return nil, stepFail(err, "Không tìm thấy video này.")
	}
	return job, nil
}

func jobSub(job *domain.CloneJobView) string {
	return clip(cleanLine(job.Source.Title), 70)
}

func (s *Service) images(ctx context.Context, jobID string) (cloneimages.View, int, error) {
	view, err := s.d.Images.Read(ctx, jobID)
	if err != nil {
		return view, 0, stepFail(err, "%s", userMessage(err))
	}
	if view.Manifest == nil {
		return view, 0, stepFail(nil, "Video này chưa có ảnh để sửa. %s", strings.TrimSpace(view.Reason))
	}
	return view, view.Manifest.Revision, nil
}

func prepareRedraw(ctx context.Context, s *Service, raw json.RawMessage, skip bool) (*prepared, error) {
	p, err := decodeParams[sceneParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	view, rev, err := s.images(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	var item *sceneItem
	for _, it := range sceneItems(view) {
		if it.ID == p.ItemID && it.No == p.SceneNo && it.Kind == "scene" {
			it := it
			item = &it
			break
		}
	}
	if item == nil {
		return nil, stepFail(nil, "Video này không có cảnh %d.", p.SceneNo)
	}
	feedback := cleanLine(p.Feedback)
	step := Step{Sub: jobSub(job), Agent: agentArtist, Reruns: []Rerun{
		{Agent: agentArtist, Label: fmt.Sprintf("cảnh %d", p.SceneNo)}, {Agent: agentQA, Label: "dựng lại"}}}
	if skip {
		step.Title = fmt.Sprintf("Bỏ ảnh cảnh %d", p.SceneNo)
		step.Detail = fmt.Sprintf("Cảnh %d dùng ảnh của cảnh bên cạnh; Hoạ sĩ không vẽ cảnh này nữa. Kiểm định dựng lại video.", p.SceneNo)
		step.Reruns = []Rerun{{Agent: agentQA, Label: "dựng lại"}}
		step.Agent = agentArtist
		return &prepared{step: step, run: func(ctx context.Context) (string, error) {
			_, err := s.d.Images.Review(ctx, p.JobID, cloneimages.ReviewInput{ExpectedRevision: &rev,
				Actions: []cloneimages.Action{{ID: item.ID, Op: "skip"}}})
			if err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return fmt.Sprintf("Đã bỏ ảnh cảnh %d.", p.SceneNo), nil
		}}, nil
	}
	step.Title = fmt.Sprintf("Vẽ lại cảnh %d", p.SceneNo)
	step.Cost = &Cost{Images: 1}
	if feedback != "" {
		step.Detail = fmt.Sprintf("Hoạ sĩ vẽ lại theo góp ý: “%s”. Ảnh hiện tại được giữ tới khi ảnh mới xong.", clip(feedback, 200))
	} else {
		step.Detail = fmt.Sprintf("Hoạ sĩ vẽ lại cảnh %d bằng prompt hiện có. Ảnh hiện tại được giữ tới khi ảnh mới xong.", p.SceneNo)
	}
	return &prepared{step: step, run: func(ctx context.Context) (string, error) {
		in := cloneimages.RedrawInput{ExpectedRevision: &rev}
		if feedback != "" {
			in.Prompt = &feedback
		}
		if _, err := s.d.Images.Redraw(ctx, p.JobID, item.ID, in); err != nil {
			return "", stepFail(err, "%s", userMessage(err))
		}
		return fmt.Sprintf("Hoạ sĩ đang vẽ lại cảnh %d.", p.SceneNo), nil
	}}, nil
}

func prepareRedrawAll(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[redrawAllParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	view, rev, err := s.images(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	if n := len(view.Items); n == 0 || n != p.Count {
		return nil, stepFail(nil, "Bộ ảnh đã đổi (hiện có %d ảnh, kế hoạch tính %d). Hãy nhắn lại để trợ lý tính giá mới.", len(view.Items), p.Count)
	}
	feedback := cleanLine(p.Feedback)
	step := Step{Title: "Vẽ lại cả bộ ảnh", Sub: jobSub(job), Agent: agentArtist, Cost: &Cost{Images: p.Count},
		Reruns: []Rerun{{Agent: agentArtist, Label: fmt.Sprintf("%d ảnh", p.Count)}, {Agent: agentQA, Label: "dựng lại"}}}
	if feedback != "" {
		step.Detail = fmt.Sprintf("Hoạ sĩ vẽ lại %d ảnh theo góp ý: “%s”. Tốn %d ảnh. Ảnh hiện tại được giữ tới khi ảnh mới xong.", p.Count, clip(feedback, 200), p.Count)
	} else {
		step.Detail = fmt.Sprintf("Hoạ sĩ vẽ lại %d ảnh. Tốn %d ảnh. Ảnh hiện tại được giữ tới khi ảnh mới xong.", p.Count, p.Count)
	}
	return &prepared{step: step, run: func(ctx context.Context) (string, error) {
		_, err := s.d.Images.RedrawAll(ctx, p.JobID, cloneimages.ResetInput{ExpectedRevision: &rev, ConfirmCount: p.Count, Feedback: feedback})
		if err != nil {
			return "", stepFail(err, "%s", userMessage(err))
		}
		return fmt.Sprintf("Hoạ sĩ đang vẽ lại %d ảnh.", p.Count), nil
	}}, nil
}

var partLabel = map[string]string{"title": "tiêu đề", "description": "mô tả", "tags": "tags"}

func prepareMetadata(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[metadataParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	if len(p.Parts) == 0 {
		return nil, fail(CodeInvalid, "Chọn phần cần viết lại.")
	}
	seen := map[string]bool{}
	labels := make([]string, 0, len(p.Parts))
	for _, part := range p.Parts {
		label, ok := partLabel[part]
		if !ok || seen[part] {
			return nil, fail(CodeInvalid, "Phần nội dung không hợp lệ.")
		}
		seen[part] = true
		labels = append(labels, label)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	direction := cleanLine(p.Direction)
	title := "Viết lại " + strings.Join(labels, ", ")
	detail := fmt.Sprintf("Đạo diễn viết lại %s", strings.Join(labels, ", "))
	if direction != "" {
		detail += fmt.Sprintf(" theo góp ý: “%s”", clip(direction, 200))
	}
	detail += ". Bản cũ vẫn xem lại được trong lịch sử."
	return &prepared{step: Step{Title: title, Sub: jobSub(job), Agent: agentDirector, Detail: detail,
		Reruns: []Rerun{{Agent: agentDirector, Label: strings.Join(labels, ", ")}}},
		run: func(ctx context.Context) (string, error) {
			view, err := s.d.Metadata.Read(ctx, p.JobID)
			if err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			rev := view.Revision
			if _, err = s.d.Metadata.Rewrite(ctx, p.JobID, clonemetadata.Rewrite{ExpectedRevision: &rev, Parts: p.Parts, Direction: direction}); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Đạo diễn đã viết lại " + strings.Join(labels, ", ") + ".", nil
		}}, nil
}

func prepareBackdrop(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[jobParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	return &prepared{step: Step{Title: "Đổi nền khác", Sub: jobSub(job), Agent: agentScout,
		Detail: "Trinh sát chọn lại video nền cho video này, rồi Kiểm định dựng lại.",
		Reruns: []Rerun{{Agent: agentScout, Label: "nền"}, {Agent: agentQA, Label: "dựng lại"}}},
		run: func(ctx context.Context) (string, error) {
			if _, err := s.d.Backdrops.Edit(ctx, p.JobID, clonebackdrop.EditRequest{Op: "reroll"}); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Trinh sát đang chọn nền khác.", nil
		}}, nil
}

func prepareNoImages(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[jobParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	_, rev, err := s.images(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	yes := true
	plan, err := s.d.Images.PlanOverrides(ctx, p.JobID, cloneimages.OverrideInput{ExpectedRevision: &rev, NoImages: &yes})
	if err != nil {
		return nil, stepFail(err, "%s", userMessage(err))
	}
	step := Step{Title: "Làm video này không cần ảnh", Sub: jobSub(job), Agent: agentArtist,
		Detail: "Hoạ sĩ không vẽ ảnh cho video này; Kiểm định dựng lại bằng nền và chữ.",
		Reruns: []Rerun{{Agent: agentQA, Label: "dựng lại"}}}
	if plan.Cost.Images > 0 {
		step.Cost = &Cost{Images: plan.Cost.Images}
	}
	confirm := plan.Cost.Images
	return &prepared{step: step, run: func(ctx context.Context) (string, error) {
		in := cloneimages.OverrideInput{ExpectedRevision: &rev, NoImages: &yes}
		if confirm > 0 {
			in.ConfirmCount = &confirm
		}
		if _, err := s.d.Images.ApplyOverrides(ctx, p.JobID, in); err != nil {
			return "", stepFail(err, "%s", userMessage(err))
		}
		return "Video này sẽ làm không cần ảnh.", nil
	}}, nil
}

var stepAgent = map[domain.CloneStep]string{
	domain.CloneDownload: agentScout, domain.CloneBackdrop: agentScout, domain.CloneTranscript: agentScout,
	domain.CloneMetadata: agentDirector, domain.CloneScenes: agentDirector,
	domain.CloneImages: agentArtist,
}

func preparePause(ctx context.Context, s *Service, id IntentID, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[jobParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	job, err := s.job(ctx, p.JobID)
	if err != nil {
		return nil, err
	}
	agent := agentQA
	if job.ActiveStep != nil {
		if a, ok := stepAgent[*job.ActiveStep]; ok {
			agent = a
		}
	}
	if id == Pause {
		if job.PausedAt != nil {
			return nil, stepFail(nil, "Video này đã tạm dừng rồi.")
		}
		return &prepared{step: Step{Title: "Tạm dừng video này", Sub: jobSub(job), Agent: agent,
			Detail: "Video đang chạy làm nốt bước hiện tại rồi dừng; bạn chạy tiếp lúc nào cũng được.",
		}, run: func(ctx context.Context) (string, error) {
			if _, err := s.d.Jobs.Pause(ctx, p.JobID); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Đã tạm dừng video này.", nil
		}}, nil
	}
	if job.PausedAt == nil {
		return nil, stepFail(nil, "Video này không đang tạm dừng.")
	}
	return &prepared{step: Step{Title: "Chạy tiếp video này", Sub: jobSub(job), Agent: agent,
		Detail: "Video chạy tiếp từ bước đang dừng."},
		run: func(ctx context.Context) (string, error) {
			if _, err := s.d.Jobs.Resume(ctx, p.JobID); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Video này chạy tiếp.", nil
		}}, nil
}

// ── kênh ────────────────────────────────────────────────────────────────────

func (s *Service) channel(ctx context.Context, id string) (channels.ChannelView, error) {
	v, err := s.d.Channels.Get(ctx, id)
	if err != nil {
		var ce *channels.Error
		if errors.As(err, &ce) && ce.Code == channels.CodeNotFound {
			return v, stepFail(err, "Không tìm thấy kênh này.")
		}
		return v, stepFail(err, "%s", userMessage(err))
	}
	return v, nil
}

func chanName(v channels.ChannelView) string {
	if n := cleanLine(v.Channel.Name); n != "" {
		return n
	}
	return "Kênh chưa đặt tên"
}

func preparePace(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[paceParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	if p.PerDay < channels.PaceMin || p.PerDay > channels.PaceMax {
		return nil, stepFail(nil, "Nhịp mỗi ngày phải từ %d tới %d video.", channels.PaceMin, channels.PaceMax)
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	name := chanName(ch)
	if ch.Channel.PacePerDay == p.PerDay {
		return nil, stepFail(nil, "%s đã làm %d video mỗi ngày rồi.", name, p.PerDay)
	}
	return &prepared{step: Step{Title: fmt.Sprintf("Mỗi ngày làm %d video", p.PerDay), Sub: name + " · từ hôm nay", Agent: agentScout,
		Detail: fmt.Sprintf("Nhịp của %s đổi từ %d sang %d video mỗi ngày.", name, ch.Channel.PacePerDay, p.PerDay)},
		run: func(ctx context.Context) (string, error) {
			n := p.PerDay
			if _, err := s.d.Channels.Update(ctx, p.ChannelID, channels.UpdateRequest{PacePerDay: &n}); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return fmt.Sprintf("%s làm %d video mỗi ngày.", name, n), nil
		}}, nil
}

func prepareFilter(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[filterParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	if p.MinDurationSec < 1 || p.MinDurationSec > channels.MaxSourceMinDurationSec {
		return nil, stepFail(nil, "Thời lượng tối thiểu phải từ 1 giây tới %d giờ.", channels.MaxSourceMinDurationSec/3600)
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	var rival string
	for _, src := range ch.Sources {
		if src.RivalID == p.RivalID {
			rival = cleanLine(src.Title)
		}
	}
	if rival == "" {
		return nil, stepFail(nil, "%s chưa theo dõi đối thủ này.", chanName(ch))
	}
	name := chanName(ch)
	minutes := durationText(p.MinDurationSec)
	return &prepared{step: Step{Title: "Tự lấy: bỏ video dưới " + minutes, Sub: name + " · " + rival + " · từ lần quét sau", Agent: agentScout,
		Detail: fmt.Sprintf("Tự lấy chỉ chọn video của %s dài từ %s. Video đã có trong hàng không đổi.", rival, minutes)},
		run: func(ctx context.Context) (string, error) {
			sec := p.MinDurationSec
			if _, err := s.d.Channels.SetSourceMinDuration(ctx, p.ChannelID, p.RivalID, &sec); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Tự lấy đã bỏ video dưới " + minutes + " của " + rival + ".", nil
		}}, nil
}

// durationText viết số giây thành “30 phút” / “1 giờ” / “90 giây”.
func durationText(sec int) string {
	switch {
	case sec%3600 == 0:
		return fmt.Sprintf("%d giờ", sec/3600)
	case sec%60 == 0:
		return fmt.Sprintf("%d phút", sec/60)
	default:
		return fmt.Sprintf("%d giây", sec)
	}
}

func youtubeURL(videoID string) string { return "https://youtu.be/" + videoID }

func prepareMake(ctx context.Context, s *Service, id IntentID, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[makeParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	linked := false
	for _, src := range ch.Sources {
		if src.RivalID == p.RivalID {
			linked = true
		}
	}
	if !linked {
		return nil, stepFail(nil, "%s chưa theo dõi đối thủ của video này.", chanName(ch))
	}
	req := domain.CloneEnqueueRequest{ChannelID: p.ChannelID, URLs: []string{youtubeURL(p.VideoID)}}
	plan, err := s.d.Queue.Plan(ctx, req)
	if err != nil {
		return nil, stepFail(err, "%s", userMessage(err))
	}
	if len(plan.Invalid) > 0 {
		return nil, stepFail(nil, "Link video không đọc được.")
	}
	if len(plan.Accepted) == 0 {
		return nil, stepFail(nil, "“%s” đã có trong hàng hoặc đã làm rồi.", clip(cleanLine(p.Title), 70))
	}
	acc := plan.Accepted[0]
	title := "Làm “" + clip(cleanLine(acc.Title), 60) + "”"
	if acc.Outlier != nil {
		title += " · ×" + viNumber(*acc.Outlier)
	}
	where := "Cuối hàng"
	if plan.Position == "front" {
		where = "Đầu hàng"
	}
	step := Step{Title: title, Sub: where + " · " + chanName(ch), Agent: agentScout, EnqueuesVideo: true}
	if acc.ImageEstimate != nil && *acc.ImageEstimate > 0 {
		step.Cost = &Cost{Images: *acc.ImageEstimate}
		step.Detail = fmt.Sprintf("Thêm video vào hàng của %s. Trinh sát tải nguồn rồi cả nhóm làm tiếp; ước tính %d ảnh.", chanName(ch), *acc.ImageEstimate)
	} else {
		step.Detail = fmt.Sprintf("Thêm video vào hàng của %s. Trinh sát tải nguồn rồi cả nhóm làm tiếp.", chanName(ch))
	}
	return &prepared{step: step, run: func(ctx context.Context) (string, error) {
		out, err := s.d.Queue.Enqueue(ctx, req)
		if err != nil {
			return "", stepFail(err, "%s", userMessage(err))
		}
		if len(out.Items) == 0 {
			return "", stepFail(nil, "“%s” đã có trong hàng hoặc đã làm rồi.", clip(cleanLine(p.Title), 70))
		}
		return "Đã thêm video vào hàng của " + chanName(ch) + ".", nil
	}}, nil
}

// viNumber viết số thập phân kiểu Việt (dấu phẩy), một chữ số lẻ: 4,2.
func viNumber(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1)
}

func preparePrioritize(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[prioritizeParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	view, err := s.d.Queue.List(ctx)
	if err != nil {
		return nil, stepFail(err, "%s", userMessage(err))
	}
	rank, found := 0, -1
	var title string
	for _, it := range view.Items {
		if it.ChannelID != p.ChannelID {
			continue
		}
		rank++
		if it.ID == p.QueueItemID {
			found, title = rank, it.Title
			break
		}
	}
	if found < 0 {
		return nil, stepFail(nil, "Video này không còn trong hàng chờ của %s.", chanName(ch))
	}
	if found == 1 {
		return nil, stepFail(nil, "“%s” đã ở đầu hàng.", clip(cleanLine(title), 70))
	}
	return &prepared{step: Step{Title: "Lên đầu hàng: “" + clip(cleanLine(title), 60) + "”", Sub: fmt.Sprintf("%s · đang thứ %d", chanName(ch), found), Agent: agentScout,
		Detail: fmt.Sprintf("Đưa video lên đầu hàng của %s; các video khác lùi một bậc.", chanName(ch))},
		run: func(ctx context.Context) (string, error) {
			item := p.QueueItemID
			if _, err := s.d.Queue.Patch(ctx, domain.QueuePatch{ID: &item, Action: "front"}); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Đã đưa video lên đầu hàng.", nil
		}}, nil
}

var presetLabel = map[string]string{"careful": "Cẩn thận", "balanced": "Cân bằng", "auto": "Tự chạy"}

func prepareAutonomy(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[autonomyParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	label, ok := presetLabel[p.Preset]
	if !ok {
		return nil, fail(CodeInvalid, "Mức tự chủ chỉ là Cẩn thận, Cân bằng hoặc Tự chạy.")
	}
	if s.d.Autonomy == nil {
		return nil, stepFail(nil, "Mức tự chủ chưa sẵn sàng.")
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	name := chanName(ch)
	return &prepared{step: Step{Title: "Mức tự chủ: " + label, Sub: name, Agent: agentQA,
		Detail: fmt.Sprintf("Đặt Mức tự chủ của %s thành %s. Bạn đổi lại bất cứ lúc nào ở Kênh › Nguồn & tự động.", name, label)},
		run: func(ctx context.Context) (string, error) {
			if err := s.d.Autonomy.ApplyPreset(ctx, p.ChannelID, p.Preset); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return name + " chạy ở mức " + label + ".", nil
		}}, nil
}

// MaxRuleRunes là độ dài tối đa của một quy tắc trợ lý lưu.
const MaxRuleRunes = 200

func prepareRule(ctx context.Context, s *Service, raw json.RawMessage) (*prepared, error) {
	p, err := decodeParams[ruleParams](raw)
	if err != nil {
		return nil, badParams(err)
	}
	text := cleanLine(p.Text)
	if text == "" || len([]rune(text)) > MaxRuleRunes {
		return nil, stepFail(nil, "Quy tắc phải từ 1 tới %d ký tự.", MaxRuleRunes)
	}
	if s.d.Rules == nil {
		return nil, stepFail(nil, "Quy tắc đã học chưa sẵn sàng.")
	}
	ch, err := s.channel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	name := chanName(ch)
	return &prepared{step: Step{Title: "Lưu quy tắc: “" + clip(text, 70) + "”", Sub: name, Agent: agentDirector,
		Detail: fmt.Sprintf("Từ video sau, Đạo diễn làm theo quy tắc này cho %s. Bạn tắt hoặc xoá ở Kênh › Quy tắc đã học.", name)},
		run: func(ctx context.Context) (string, error) {
			if _, err := s.d.Rules.AddRule(ctx, p.ChannelID, text, "assistant"); err != nil {
				return "", stepFail(err, "%s", userMessage(err))
			}
			return "Đã lưu quy tắc cho " + name + ".", nil
		}}, nil
}
