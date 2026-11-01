package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
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
//
// Nguồn tiến độ phụ thuộc chế độ: bản file/desktop đọc service cục bộ, bản
// hosted đọc trạng thái máy đọc trong Postgres (cùng voice.InstallProgress, nên
// dock không phải biết gì thêm).
type voiceJobsProvider struct{ g *Gateway }

var _ JobsProvider = voiceJobsProvider{}

func newVoiceJobsProvider(g *Gateway) JobsProvider { return voiceJobsProvider{g: g} }

func (voiceJobsProvider) Kind() string { return voiceJobsKind }

// hosted báo tiến độ đến từ trạng thái máy đọc trong cơ sở dữ liệu. Lúc dựng
// gateway chỉ thấy nil, nên dùng audioStoreClosed (tín hiệu của SetAudioStore)
// làm mốc “đã chọn hosted backend”, thay vì tin mãi ảnh chụp lúc dựng.
func (p voiceJobsProvider) hosted() bool {
	if p.g == nil {
		return false
	}
	deps, injected, _ := p.g.audioDeps()
	return deps != nil || injected
}

// voiceHostedPollInterval 是 hosted 安装状态的轮询间隔；测试改小它以免
// 每个用例都等满一秒。
var voiceHostedPollInterval = time.Second

func (p voiceJobsProvider) progress() voice.InstallProgress {
	if p.hosted() {
		// The provider interface has no request context; a dock row must never
		// wait on a slow database.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return p.g.hostedTTSInstallState(ctx)
	}
	return ttsService().InstallState()
}

// Jobs trả hàng hiện tại, hoặc rỗng khi chưa từng cài.
func (p voiceJobsProvider) Jobs() []Job {
	if row, ok := voiceInstallRow(p.progress()); ok {
		return []Job{row}
	}
	return nil
}

// Watch phát mỗi mốc tiến độ; lượt cài biến mất thì phát Removed.
//
// NewGateway 在 SetAudioStore 之前就启动了 watcher，所以这里不能只选一次
// 后端：先按本地安装状态监听，注入 hosted 依赖（audioStoreCh 关闭）时取消
// 本地订阅、同一个 goroutine 转到 hosted 轮询。只有一条循环，不会重复。
func (p voiceJobsProvider) Watch(ctx context.Context, emit func(JobChange)) {
	if p.hosted() {
		p.watchHosted(ctx, emit)
		return
	}
	var injected <-chan struct{}
	if p.g != nil {
		_, _, injected = p.g.audioDeps()
	}
	p.watchLocal(ctx, emit, injected)
}

// watchLocal 监听本机安装进度，直到 ctx 结束或 hosted 后端注入。
func (p voiceJobsProvider) watchLocal(ctx context.Context, emit func(JobChange), injected <-chan struct{}) {
	var switched atomic.Bool
	unsub := ttsService().SubscribeInstall(func(progress voice.InstallProgress) {
		if switched.Load() {
			return
		}
		emitVoiceInstall(emit, progress)
	})
	defer unsub()
	select {
	case <-ctx.Done():
	case <-injected:
		switched.Store(true)
		unsub()
		p.watchHosted(ctx, emit)
	}
}

// watchHosted hỏi trạng thái máy đọc mỗi voiceHostedPollInterval: máy Mac công
// bố qua worker listener, không có kênh đẩy nào tới tiến trình web.
func (p voiceJobsProvider) watchHosted(ctx context.Context, emit func(JobChange)) {
	tick := time.NewTicker(voiceHostedPollInterval)
	defer tick.Stop()
	var last voice.InstallProgress
	have := false
	for {
		progress := p.progress()
		if !have || !progress.Same(last) {
			emitVoiceInstall(emit, progress)
			last, have = progress, true
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func emitVoiceInstall(emit func(JobChange), progress voice.InstallProgress) {
	row, ok := voiceInstallRow(progress)
	if !ok {
		emit(JobChange{Job: Job{ID: voiceInstallJobID}, Removed: true})
		return
	}
	emit(JobChange{Job: row})
}

// voiceStepIndex là số thứ tự bước chuẩn bị trên thẻ Máy đọc (Bước i/6):
// tools · python · source · deps · model · start.
func voiceStepIndex(step string) (idx, total int) {
	total = len(voice.ProvisionSteps)
	for i, s := range voice.ProvisionSteps {
		if s == step {
			return i + 1, total
		}
	}
	return 0, total
}

// voiceInstallStepText là dòng bước của hàng dock: "Bước 3/6 · Tải bộ đọc…" ở
// bước không đo được, "Bước 5/6 · Tải mẫu giọng · 299/581 MB" ở bước mẫu giọng.
func voiceInstallStepText(p voice.InstallProgress) string {
	idx, total := voiceStepIndex(p.Step)
	label := voiceStepLabel(p.Step)
	if label == "" {
		return p.Message
	}
	step := fmt.Sprintf("Bước %d/%d · %s", idx, total, label)
	if p.Step == voice.ProvisionStepModel && p.BytesDone > 0 && p.BytesTotal > 0 {
		return fmt.Sprintf("%s · %s", step, audioHumanBytes(p.BytesDone)+"/"+audioHumanBytes(p.BytesTotal))
	}
	return step
}

// voiceStepLabel là nhãn một bước chuẩn bị; đây là câu hiển thị, không phải mã,
// nên giữ tiếng Việt ngay đây (UI chỉ hiển thị).
func voiceStepLabel(step string) string {
	switch step {
	case voice.ProvisionStepTools:
		return "Tải công cụ cài đặt"
	case voice.ProvisionStepPython:
		return "Chuẩn bị môi trường chạy"
	case voice.ProvisionStepSource:
		return "Tải bộ đọc"
	case voice.ProvisionStepDeps:
		return "Cài thư viện"
	case voice.ProvisionStepModel:
		return "Tải mẫu giọng"
	case voice.ProvisionStepStart:
		return "Khởi động bộ đọc"
	default:
		return ""
	}
}

// audioHumanBytes in một dung lượng gọn như "299 MB" cho dòng bước của dock.
func audioHumanBytes(v int64) string {
	if v >= 1<<30 {
		return fmt.Sprintf("%.0f GB", float64(v)/(1<<30))
	}
	return fmt.Sprintf("%.0f MB", float64(v)/(1<<20))
}

// Control: retry chạy lại lượt cài, cancel dừng lượt đang chạy.
// Tạm dừng không có nghĩa với việc một lần chạy của lệnh cài.
func (p voiceJobsProvider) Control(ctx context.Context, id, op string) error {
	if id != voiceInstallJobID {
		return errJobNotFound
	}
	if p.hosted() {
		return p.controlHosted(ctx, op)
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

// controlHosted đẩy lệnh sang máy đọc: retry = xếp lệnh install, cancel không
// có nghĩa vì lượt cài chạy trên máy khác và không có kênh dừng.
func (p voiceJobsProvider) controlHosted(ctx context.Context, op string) error {
	deps, _, _ := p.g.audioDeps()
	if deps == nil || deps.Jobs == nil {
		return errJobUnsupported
	}
	snap := p.g.workerSnapshot(ctx)
	switch op {
	case "retry":
		if !snap.Found {
			return errors.New("Chưa có máy đọc nào kết nối")
		}
		if snap.State.State == voice.ProvisionStateReady {
			return errors.New("Bộ đọc trên máy đã cài xong")
		}
		_, err := deps.Jobs.BumpWorkerCommand(ctx, snap.ID, "install")
		return err
	case "cancel":
		return errJobUnsupported
	default:
		return errJobUnsupported
	}
}

// voiceInstallRow đổi một mốc tiến độ thành hàng của dock (hàng prepErr của
// 10-audio.md §8.6: "Đang chuẩn bị máy đọc" + bước i/6 + % khi bước đo được,
// vòng xoay thôi khi không). state rỗng/idle = chưa từng cài nên không có hàng.
func voiceInstallRow(p voice.InstallProgress) (Job, bool) {
	job := Job{
		ID:        voiceInstallJobID,
		Kind:      voiceJobsKind,
		Title:     "Đang chuẩn bị máy đọc",
		Href:      voiceSettingsRoute,
		StartedAt: p.Updated,
	}
	switch p.State {
	case "running":
		job.Status = JobRunning
		if p.Percent != nil {
			percent := float64(*p.Percent) / 100
			job.Progress = &percent
		}
		job.Step = strings.TrimSpace(voiceInstallStepText(p))
	case "done":
		finished := p.Updated
		job.Status = JobDoneS
		job.Progress = float64Ptr(1)
		job.FinishedAt = &finished
		job.Result = &JobResult{Title: "Đã cài xong bộ đọc trên máy", Sub: strings.TrimSpace(p.Message)}
	case "error":
		job.Status = JobFailed
		job.Step = strings.TrimSpace(p.Message)
		errText := strings.TrimSpace(p.Error)
		if errText == "" {
			errText = job.Step
		}
		// Mã lỗi 8.2 giúp FE chọn đúng câu và giữ nút Thử lại/Huỷ (prepErr).
		if p.ErrorCode != "" {
			job.Error = JobErrorInfo{Message: errText, Cause: p.ErrorCode}
		} else {
			job.Error = errText
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
