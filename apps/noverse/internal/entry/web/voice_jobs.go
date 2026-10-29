package web

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// 14-jobs-dock §1.7 — việc của máy: cài bộ đọc trên máy (POST /api/tts/install).
//
// Hàng này KHÔNG thuộc truyện nào (Project/Story rỗng) và chỉ quản trị viên thấy,
// cùng quyền với chính endpoint cài đặt. Trước đây nút “Cài bộ đọc trên máy” chỉ
// quay vòng ở thẻ hướng dẫn nên dock luôn trống, rồi lỗi thì không ai thấy.
const (
	// voiceJobsKind là đoạn loại việc trong ID và là khoá nguồn việc.
	voiceJobsKind = "voice"
	// voiceInstallJobID là ID ổn định của hàng cài bộ đọc.
	voiceInstallJobID = voiceJobsKind + "/install"
	// voiceSettingsRoute là màn liên quan của việc này (“Mở màn liên quan” của dock).
	voiceSettingsRoute = "/settings/voice"
)

// voiceJobsProvider đưa lượt cài bộ đọc trên máy vào chung danh sách việc chạy nền.
type voiceJobsProvider struct{}

var _ JobsProvider = voiceJobsProvider{}

func newVoiceJobsProvider() JobsProvider { return voiceJobsProvider{} }

func (voiceJobsProvider) Kind() string { return voiceJobsKind }

// Jobs trả hàng hiện tại, hoặc rỗng khi chưa từng cài.
func (voiceJobsProvider) Jobs() []Job {
	if row, ok := voiceInstallRow(ttsService().InstallState()); ok {
		return []Job{row}
	}
	return nil
}

// Watch phát mỗi mốc tiến độ; lượt cài biến mất thì phát Removed.
func (p voiceJobsProvider) Watch(ctx context.Context, emit func(JobChange)) {
	unsub := ttsService().SubscribeInstall(func(progress voice.InstallProgress) {
		row, ok := voiceInstallRow(progress)
		if !ok {
			emit(JobChange{Job: Job{ID: voiceInstallJobID}, Removed: true})
			return
		}
		emit(JobChange{Job: row})
	})
	defer unsub()
	<-ctx.Done()
}

// Control: retry chạy lại lượt cài, cancel dừng lượt đang chạy.
// Tạm dừng không có nghĩa với việc một lần chạy của lệnh cài.
func (voiceJobsProvider) Control(_ context.Context, id, op string) error {
	if id != voiceInstallJobID {
		return errJobNotFound
	}
	svc := ttsService()
	switch op {
	case "retry":
		switch svc.InstallState().State {
		case "running":
			return errJobUnsupported
		case "done":
			return errors.New("Bộ đọc trên máy đã cài xong")
		}
		// Lượt cài phải sống tiếp kể cả khi người dùng đóng tab: không dùng ctx của request.
		if err := svc.StartInstall(context.Background()); err != nil {
			if errors.Is(err, voice.ErrInstallRunning) {
				return errJobUnsupported
			}
			return err
		}
		return nil
	case "cancel":
		svc.CancelInstall()
		return nil
	default:
		return errJobUnsupported
	}
}

// voiceInstallRow đổi một mốc tiến độ thành hàng của dock. state rỗng/idle = chưa
// từng cài nên không có hàng nào (ok=false).
func voiceInstallRow(p voice.InstallProgress) (Job, bool) {
	job := Job{
		ID:        voiceInstallJobID,
		Kind:      voiceJobsKind,
		Title:     "Cài bộ đọc trên máy",
		Href:      voiceSettingsRoute,
		StartedAt: p.Updated,
	}
	switch p.State {
	case "running":
		job.Status = JobRunning
		percent := float64(p.Percent) / 100
		job.Progress = &percent
		job.Step = strings.TrimSpace(p.Message)
	case "done":
		finished := p.Updated
		job.Status = JobDoneS
		job.Progress = float64Ptr(1)
		job.FinishedAt = &finished
		job.Result = &JobResult{Title: "Đã cài xong bộ đọc trên máy", Sub: strings.TrimSpace(p.Message)}
	case "error":
		job.Status = JobFailed
		job.Step = strings.TrimSpace(p.Message)
		job.Error = strings.TrimSpace(p.Error)
		if job.Error == "" || job.Error == nil {
			job.Error = job.Step
		}
	default:
		return Job{}, false
	}
	if job.StartedAt.IsZero() {
		job.StartedAt = time.Now()
	}
	return job, true
}

func float64Ptr(v float64) *float64 { return &v }
