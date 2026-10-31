package domain

import (
	"fmt"
	"time"
)

// View-model phía server cho Bàn dựng (hợp đồng dữ liệu §1–§2): nhóm trạng
// thái, hàng trợ lý và dòng phụ. Thuần hàm trên job + steps — không I/O.

// Nhóm trạng thái của Bàn dựng; mục hàng chờ chưa có job nhận nhóm "queued".
const (
	StageQueued    = "queued"
	StageWorking   = "working"
	StageReview    = "review"
	StageAttention = "attention"
	StagePaused    = "paused"
	StageReady     = "ready"
	StagePosted    = "posted"
)

// CloneStageGroups là mọi nhóm hợp lệ, theo thứ tự hiển thị.
var CloneStageGroups = []string{StageQueued, StageWorking, StageReview, StageAttention, StagePaused, StageReady, StagePosted}

// StageGroupOf trả nhóm Bàn dựng của job. Thứ tự quyết định: job đã thay thế
// không thuộc nhóm nào; Tạm dừng đứng trước Chờ duyệt; rồi cổng duyệt, xong,
// đã đăng, lỗi/huỷ/đỗ, và cuối cùng là đang làm.
func StageGroupOf(job CloneJob, steps []CloneStepRecord) string {
	if job.SupersededAt != nil {
		return ""
	}
	if job.PausedAt != nil {
		return StagePaused
	}
	for _, s := range steps {
		if (s.Step == CloneBuild || s.Step == CloneImages) && s.Status == CloneWaiting && s.Error != nil && s.Error.Code == CloneErrAwaitingReview {
			return StageReview
		}
	}
	for _, step := range steps {
		if step.Status == CloneWaiting && step.Error != nil && step.Error.Code == CloneErrQuota {
			return StageWorking
		}
	}
	if ChainDone(steps) {
		if job.PostedAt != nil {
			return StagePosted
		}
		return StageReady
	}
	switch job.Status {
	case CloneFailed, CloneCancelled, CloneWaiting:
		return StageAttention
	}
	return StageWorking
}

// ID của bốn trợ lý hiển thị trên thẻ video.
const (
	AgentScout    = "scout"
	AgentDirector = "director"
	AgentArtist   = "artist"
	AgentQA       = "qa"
)

// Trạng thái trợ lý (§1).
const (
	AgentWaiting  = "waiting"
	AgentRunning  = "running"
	AgentDone     = "done"
	AgentStopped  = "stopped"
	AgentNeedsYou = "needs_you"
)

// agentSteps gán step cho trợ lý: Trinh sát tải nguồn, Đạo diễn chia cảnh và
// viết tiêu đề, Hoạ sĩ vẽ ảnh, Kiểm định dựng và xuất.
var agentSteps = map[string][]CloneStep{
	AgentScout:    {CloneDownload, CloneBackdrop, CloneTranscript},
	AgentDirector: {CloneMetadata, CloneScenes},
	AgentArtist:   {CloneImages},
	AgentQA:       {CloneBuild, CloneScore, CloneExport},
}

// AgentIDs theo thứ tự hiển thị.
var AgentIDs = []string{AgentScout, AgentDirector, AgentArtist, AgentQA}

// agentVerbs là câu việc của từng trợ lý, dùng trong `now` và `subline`.
var agentVerbs = map[string]string{
	AgentScout:    "Trinh sát tải nguồn",
	AgentDirector: "Đạo diễn chia cảnh",
	AgentArtist:   "Hoạ sĩ vẽ ảnh",
	AgentQA:       "Kiểm định dựng video",
}

// agentNames là tên trợ lý theo ngôi người (dùng trong subline).
var agentNames = map[string]string{
	AgentScout:    "Trinh sát",
	AgentDirector: "Đạo diễn",
	AgentArtist:   "Hoạ sĩ",
	AgentQA:       "Kiểm định",
}

// CloneAgentOf trả trợ lý đang phụ trách step; "" khi step không thuộc ai.
func CloneAgentOf(step CloneStep) string {
	for id, steps := range agentSteps {
		for _, s := range steps {
			if s == step {
				return id
			}
		}
	}
	return ""
}

func stepsOfAgent(agent string, steps []CloneStepRecord) []CloneStepRecord {
	want := map[CloneStep]bool{}
	for _, s := range agentSteps[agent] {
		want[s] = true
	}
	out := make([]CloneStepRecord, 0, len(want))
	for _, r := range steps {
		if want[r.Step] {
			out = append(out, r)
		}
	}
	return out
}

// AgentViews tính hàng trợ lý (§1) cho một job. `now` lấy câu đang làm từ
// phase của step đang chạy; `metric` tóm tắt số liệu đã có.
func AgentViews(job CloneJob, steps []CloneStepRecord) []CloneAgentView {
	review := StageGroupOf(job, steps) == StageReview
	out := make([]CloneAgentView, 0, len(AgentIDs))
	for _, id := range AgentIDs {
		rows := stepsOfAgent(id, steps)
		view := CloneAgentView{ID: id, State: AgentWaiting}
		var running *CloneStepRecord
		done, stopped := len(rows) > 0, false
		for i := range rows {
			r := &rows[i]
			if r.Status == CloneSucceeded {
				continue
			}
			done = false
			if r.Status == CloneRunning {
				running = r
			}
			if r.Status == CloneFailed || r.Status == CloneCancelled || (r.Status == CloneWaiting && r.Error != nil && r.Error.Code != CloneErrAwaitingReview) {
				stopped = true
			}
		}
		switch {
		case running != nil:
			view.State = AgentRunning
			now := runningSentence(id, *running)
			view.Now = &now
		case done:
			view.State = AgentDone
		case stopped:
			view.State = AgentStopped
		case id == AgentQA && review:
			view.State = AgentNeedsYou
		}
		view.Metric = agentMetric(id, rows)
		out = append(out, view)
	}
	return out
}

// runningSentence ghép câu "đang làm" của một step: động từ của trợ lý + phase
// + đếm đơn vị khi có. Bước ảnh đọc "Hoạ sĩ đang vẽ cảnh 5/6".
func runningSentence(agent string, r CloneStepRecord) string {
	unit := progressCount(r.Progress)
	switch agent {
	case AgentArtist:
		if unit != "" {
			return fmt.Sprintf("Đang vẽ cảnh %s", unit)
		}
		return "Đang vẽ ảnh"
	case AgentScout:
		if r.Progress.Phase != "" {
			return fmt.Sprintf("Trinh sát đang %s", r.Progress.Phase)
		}
		return "Trinh sát đang tải nguồn"
	case AgentDirector:
		if r.Step == CloneScenes {
			return "Đạo diễn đang chia cảnh"
		}
		return "Đạo diễn đang viết tiêu đề"
	case AgentQA:
		if r.Progress.Phase != "" {
			return fmt.Sprintf("Kiểm định đang %s", r.Progress.Phase)
		}
		return "Kiểm định đang dựng video"
	}
	return ""
}

// progressCount trả "done/total" khi step đo được tiến độ; "" khi chưa biết tổng.
func progressCount(p CloneProgress) string {
	if p.UnitsTotal != nil && *p.UnitsTotal > 0 {
		return fmt.Sprintf("%d/%d", p.UnitsDone, *p.UnitsTotal)
	}
	return ""
}

// agentMetric gom số liệu của trợ lý từ output và tiến độ step: số cảnh, số
// ảnh đã vẽ, nhà vẽ… Không bịa số: thiếu dữ liệu thì metric là nil.
func agentMetric(agent string, rows []CloneStepRecord) *CloneAgentMetric {
	switch agent {
	case AgentArtist:
		for i := range rows {
			r := &rows[i]
			if r.Progress.UnitsTotal != nil && *r.Progress.UnitsTotal > 0 {
				provider, _ := stringField(r.Output, "provider")
				value := fmt.Sprintf("%d / %d", r.Progress.UnitsDone, *r.Progress.UnitsTotal)
				if provider != "" {
					value += " · " + provider
				}
				return &CloneAgentMetric{Label: "Ảnh đã vẽ", Value: value}
			}
			if r.Output != nil {
				if n, ok := intField(r.Output, "imageCount"); ok && n > 0 {
					return &CloneAgentMetric{Label: "Ảnh đã vẽ", Value: fmt.Sprintf("%d", n)}
				}
			}
		}
	case AgentDirector:
		var scenes, tags int
		var have bool
		for i := range rows {
			r := &rows[i]
			if r.Output == nil {
				continue
			}
			if r.Step == CloneScenes {
				if n, ok := intField(r.Output, "sceneCount"); ok {
					scenes = n
					have = true
				}
			}
			if r.Step == CloneMetadata {
				if n, ok := intField(r.Output, "tagCount"); ok {
					tags = n
				}
			}
		}
		if have {
			return &CloneAgentMetric{Label: "Cảnh", Value: fmt.Sprintf("%d cảnh · %d tag", scenes, tags)}
		}
	}
	return nil
}

func stringField(out *CloneStepOutput, key string) (string, bool) {
	if out == nil || out.Data == nil {
		return "", false
	}
	v, ok := out.Data[key].(string)
	return v, ok
}

func intField(out *CloneStepOutput, key string) (int, bool) {
	if out == nil || out.Data == nil {
		return 0, false
	}
	switch v := out.Data[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

// Subline tính dòng phụ của mọi trạng thái (§2) từ job + steps + giờ hiện
// tại: câu việc đang làm, giờ hết chờ hạn mức, hoặc tóm tắt xong.
func Subline(job CloneJob, steps []CloneStepRecord, now time.Time) string {
	group := StageGroupOf(job, steps)
	switch group {
	case StagePaused:
		return "Đã tạm dừng"
	case StageReview:
		return "Chờ bạn duyệt ảnh"
	case StageReady:
		return summarizeDone(steps, now)
	case StagePosted:
		return "Đã đăng"
	case StageAttention:
		return attentionSubline(job, steps)
	}
	// working: ưu tiên chờ hạn mức rồi câu đang làm của step đang chạy.
	if line := quotaSubline(steps); line != "" {
		return line
	}
	views := AgentViews(job, steps)
	for _, v := range views {
		if v.State == AgentRunning && v.Now != nil {
			return *v.Now
		}
	}
	if job.Status == ClonePending {
		return "Đang chờ lượt"
	}
	return ""
}

// quotaSubline: step đỗ vì hết hạn mức — "Chờ hạn mức ảnh · 07:00" (không là lỗi).
func quotaSubline(steps []CloneStepRecord) string {
	for i := range steps {
		r := &steps[i]
		if r.Status != CloneWaiting || r.Error == nil || r.Error.Code != CloneErrQuota {
			continue
		}
		if r.Error.RetryAt != nil {
			return fmt.Sprintf("Chờ hạn mức ảnh · %02d:%02d", r.Error.RetryAt.Hour(), r.Error.RetryAt.Minute())
		}
		return "Chờ hạn mức ảnh"
	}
	return ""
}

// attentionSubline giải thích vì sao job rơi vào Cần xử lý.
func attentionSubline(job CloneJob, steps []CloneStepRecord) string {
	if line := quotaSubline(steps); line != "" {
		return line
	}
	for i := range steps {
		r := &steps[i]
		if (r.Status == CloneFailed || r.Status == CloneWaiting) && r.Error != nil && r.Error.Message != "" {
			return fmt.Sprintf("%s · %s", agentNames[CloneAgentOf(r.Step)], r.Error.Message)
		}
	}
	if job.Status == CloneCancelled {
		return "Đã huỷ"
	}
	return "Cần bạn xử lý"
}

// summarizeDone: "7 ảnh · 42 phút trước" — số ảnh đã vẽ cùng lúc xong tương đối.
func summarizeDone(steps []CloneStepRecord, now time.Time) string {
	var finished *time.Time
	images := 0
	for i := range steps {
		r := &steps[i]
		if r.FinishedAt != nil && (finished == nil || r.FinishedAt.After(*finished)) {
			finished = r.FinishedAt
		}
		if r.Step == CloneImages && r.Output != nil {
			if n, ok := intField(r.Output, "imageCount"); ok {
				images = n
			}
		}
	}
	ago := ""
	if finished != nil {
		ago = " · " + agoText(now.Sub(*finished))
	}
	if images > 0 {
		return fmt.Sprintf("%d ảnh%s", images, ago)
	}
	if ago != "" {
		return "Xong" + ago
	}
	return "Xong"
}

// agoText: "vừa xong" / "N phút trước" / "N giờ trước" / "N ngày trước".
func agoText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "vừa xong"
	case d < time.Hour:
		return fmt.Sprintf("%d phút trước", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d giờ trước", int(d.Hours()))
	default:
		return fmt.Sprintf("%d ngày trước", int(d.Hours()/24))
	}
}

// CloneJobView đầy đủ của một hàng Bàn dựng: job + subline + agentView + steps.
// Mục hàng chờ chưa có job trở thành CloneJobView với StageGroup "queued" và
// QueueItem mang mục hàng gốc.
// PickDefaultJob chooses the highest-priority actionable job in the scope.
func PickDefaultJob(jobs []CloneJobView) string {
	best := -1
	rank := func(group string) int {
		switch group {
		case StageAttention:
			return 0
		case StageReview:
			return 1
		case StageWorking:
			return 2
		case StageReady:
			return 3
		default:
			return 4
		}
	}
	for i := range jobs {
		r := rank(jobs[i].StageGroup)
		if r == 4 {
			continue
		}
		if best < 0 || r < rank(jobs[best].StageGroup) ||
			(r == rank(jobs[best].StageGroup) && ((r < 2 && jobs[i].CreatedAt.Before(jobs[best].CreatedAt)) ||
				(r >= 2 && jobs[i].CreatedAt.After(jobs[best].CreatedAt)))) {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return jobs[best].ID
}
type CloneBulkEligibility struct {
	Retry      []string `json:"retry"`
	Approve    []string `json:"approve"`
	Bundle     []string `json:"bundle"`
	MarkPosted []string `json:"markPosted"`
	Cancel     []string `json:"cancel"`
	Delete     []string `json:"delete"`
}

// BulkEligibility mirrors the actions available to each persisted job.
func BulkEligibility(jobs []CloneJobView) CloneBulkEligibility {
	out := CloneBulkEligibility{[]string{}, []string{}, []string{}, []string{}, []string{}, []string{}}
	for _, job := range jobs {
		if job.QueueItem != nil || job.StageGroup == StageQueued || job.StageGroup == "" {
			continue
		}
		switch job.StageGroup {
		case StageAttention:
			out.Retry = append(out.Retry, job.ID)
		case StageReview:
			out.Approve = append(out.Approve, job.ID)
		case StageWorking:
			out.Cancel = append(out.Cancel, job.ID)
		case StageReady:
			out.MarkPosted = append(out.MarkPosted, job.ID)
			var project, export string
			for _, step := range job.Steps {
				if step.Output == nil {
					continue
				}
				if step.Step == CloneBuild {
					project, _ = step.Output.Data["projectId"].(string)
				}
				if step.Step == CloneExport && step.Status == CloneSucceeded {
					if value, _ := step.Output.Data["projectId"].(string); value != "" {
						project = value
					}
					export, _ = step.Output.Data["exportId"].(string)
					if removed, _ := step.Output.Data["removedAt"].(string); removed != "" {
						export = ""
					}
				}
			}
			if job.Status == CloneSucceeded && project != "" && export != "" {
				out.Bundle = append(out.Bundle, job.ID)
			}
		}
		if job.StageGroup != StageWorking {
			out.Delete = append(out.Delete, job.ID)
		}
	}
	return out
}


type CloneJobList struct {
	Jobs        []CloneJobView `json:"jobs"`
	QueuedTotal int            `json:"queuedTotal"`
	DefaultJobID *string       `json:"defaultJobId"`
	BulkEligibility CloneBulkEligibility `json:"bulkEligibility"`
}

// CloneBucketCounts đếm job theo nhóm cho badge và tổng quan.
type CloneBucketCounts struct {
	Queued    int `json:"queued"`
	Working   int `json:"working"`
	Review    int `json:"review"`
	Attention int `json:"attention"`
	Paused    int `json:"paused"`
	Ready     int `json:"ready"`
	Posted    int `json:"posted"`
	DoneToday int `json:"doneToday"`
}

// CloneCounts: tổng toàn app cùng từng kênh; khoá "none" giữ job chưa gán kênh.
type CloneCounts struct {
	Totals  CloneBucketCounts            `json:"totals"`
	ByOwner map[string]CloneBucketCounts `json:"byOwner"`
}
