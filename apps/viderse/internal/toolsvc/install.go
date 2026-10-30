package toolsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// pumpInterval là nhịp đọc install-state để cập nhật việc nền + phát SSE:
// tối đa ~4 cập nhật/giây, coalesce theo giá trị (đổi giá trị mới ghi/phát).
const pumpInterval = 250 * time.Millisecond

// installRun là một lượt cài được theo dõi: task của nó trên bảng việc nền,
// cờ sống, và kết quả cuối cho caller Ensure/Retry.
type installRun struct {
	initDone chan struct{} // đóng khi taskID đã sẵn (caller join chờ)
	taskID   string
	cancel   context.CancelFunc
	done     chan struct{}
	runCtx   context.Context
	err      error
}

func (r *installRun) alive() bool {
	select {
	case <-r.done:
		return false
	default:
		return true
	}
}

// StartInstall khởi một lượt cài nền cho công cụ: tạo việc nền kind=setup,
// chạy installer, rồi pump tiến độ. Trả taskID — lượt đã chạy thì trả lại id
// của nó (gộp lượt, không cài kép).
func (s *Service) StartInstall(ctx context.Context, id ToolID) (string, error) {
	run, err := s.reserveInstall(ctx, id)
	if err != nil {
		return "", err
	}
	select {
	case <-run.initDone:
		return run.taskID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// reserveInstall giành đúng một lượt cho mỗi công cụ: kiểm tra và đặt
// placeholder dưới cùng một khoá, tạo việc nền xong mới chạy goroutine. Mọi
// caller (HTTP lẫn Ensure) nhận đúng handle của lượt.
func (s *Service) reserveInstall(ctx context.Context, id ToolID) (*installRun, error) {
	if id == ToolCodexLogin {
		return nil, fmt.Errorf("bước %q không phải cài đặt — cần đăng nhập", id)
	}
	s.mu.Lock()
	if run, ok := s.running[id]; ok && run.alive() {
		s.mu.Unlock()
		return run, nil
	}
	run := &installRun{done: make(chan struct{}), initDone: make(chan struct{})}
	s.running[id] = run
	s.mu.Unlock()

	if err := s.initInstallRun(ctx, id, run); err != nil {
		s.mu.Lock()
		if s.running[id] == run {
			delete(s.running, id)
		}
		s.mu.Unlock()
		run.err = err
		close(run.initDone)
		close(run.done)
		return nil, err
	}
	close(run.initDone)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runInstall(id, run)
	}()
	return run, nil
}

// Shutdown chờ mọi lượt cài (và tiến trình con của chúng) kết thúc trước khi
// tầng lưu trữ đóng; ctx chặn thời gian chờ.
func (s *Service) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ensure cài công cụ và chờ kết quả — setup orchestration và test dùng;
// hành vi việc nền + tiến độ giống hệt StartInstall.
func (s *Service) Ensure(ctx context.Context, id ToolID) error {
	run, err := s.reserveInstall(ctx, id)
	if err != nil {
		return err
	}
	select {
	case <-run.done:
		return run.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cancel yêu cầu lượt cài đang chạy dừng (sentinel của installer + ctx của
// worker). Không có lượt đang chạy là no-op.
func (s *Service) Cancel(ctx context.Context, id ToolID) error {
	s.mu.Lock()
	run, ok := s.running[id]
	s.mu.Unlock()
	if ok && run.alive() && run.cancel != nil {
		run.cancel()
	}
	if mid, isManaged := managedID(id); isManaged {
		s.mgr.Cancel(mid)
	}
	if id == ToolYtDlp {
		s.tools.YtDlp.CancelInstall()
	}
	return nil
}

// initInstallRun tạo việc nền và context của một lượt đã giành chỗ.
func (s *Service) initInstallRun(ctx context.Context, id ToolID, run *installRun) error {
	spec := domain.TaskSpec{
		Kind:  domain.TaskSetup,
		Title: installTitle(id),
		Object: domain.ObjectRef{
			Type:  "tool",
			ID:    id,
			Label: installTitle(id),
		},
		Phase: "Đang chuẩn bị",
		Action: domain.TaskAction{
			Label: "Huỷ",
			Op:    domain.TaskOpCancel,
		},
	}
	taskID, err := s.tasks.Start(ctx, spec)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(s.baseCtx)
	run.taskID = taskID
	run.cancel = cancel
	run.runCtx = runCtx
	return nil
}

// runInstall là thân của một lượt cài: pump tiến độ → chạy installer → kết
// thúc việc nền (thành công xoá dòng; lỗi/huỷ giữ dòng failed + Thử lại) →
// probe lại ở nền.
func (s *Service) runInstall(id ToolID, run *installRun) {
	ctx := run.runCtx
	stopPump := s.startPump(ctx, id, run)
	err := s.installTool(ctx, run, id)
	close(stopPump)

	s.mu.Lock()
	if s.running[id] == run {
		delete(s.running, id)
	}
	s.mu.Unlock()

	failedPhase := ""
	action := domain.TaskAction{Label: "Thử lại", Op: domain.TaskOpRetry}
	if err == nil {
		// Đo và ghi dung lượng cài đặt trong tác vụ cài — GET chỉ đọc số này.
		s.measureInstalled(id)
		s.logf(id, "info", "cài xong")
		if err := s.tasks.Finish(context.Background(), run.taskID); err != nil {
			s.logf(id, "warn", "ghi kết thúc việc nền: %v", err)
		}
	} else {
		failedPhase = friendlyInstallError(err.Error())
		s.logf(id, "error", "cài lỗi: %v", err)
		u := domain.TaskUpdate{State: domain.TaskFailed, Phase: &failedPhase, Action: &action}
		if uerr := s.tasks.Update(context.Background(), run.taskID, u); uerr != nil {
			s.logf(id, "warn", "ghi thất bại việc nền: %v", uerr)
		}
	}
	run.err = err
	close(run.done)
	s.emit(id)

	// Probe lại nền để hàng đổi trạng thái thật (update/ready/failed).
	s.mu.Lock()
	s.startRefreshLocked()
	s.mu.Unlock()
}

// installTool chạy bộ cài của công cụ — managed qua sổ, yt-dlp qua pipeline
// tự cập nhật của nó.
func (s *Service) installTool(ctx context.Context, run *installRun, id ToolID) error {
	if mid, ok := managedID(id); ok {
		_, err := s.mgr.Install(ctx, mid)
		return err
	}
	if id == ToolYtDlp {
		_, err := s.tools.YtDlp.Install(ctx)
		return err
	}
	return fmt.Errorf("công cụ %q không cài được tự động", id)
}

// startPump cập nhật việc nền theo install-state mỗi pumpInterval (≤4 lần/
// giây): phase, progress, % — đổi giá trị mới ghi DB và phát SSE.
// Trả kênh dừng: đóng nó kết thúc pump và ghi lần cuối.
func (s *Service) startPump(ctx context.Context, id ToolID, run *installRun) chan<- struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pumpInterval)
		defer ticker.Stop()
		last := ""
		push := func() {
			p := liveProgress(s.mgr, id)
			key := ""
			var phase string
			var progress *domain.TaskProgress
			if p != nil {
				phase = PhaseLabel(p)
				if p.BytesTotal != nil && *p.BytesTotal > 0 {
					progress = &domain.TaskProgress{Done: p.BytesDone, Total: *p.BytesTotal}
				}
				key = fmt.Sprintf("%s|%d|%v", phase, p.BytesDone, p.BytesTotal != nil)
			} else {
				phase = "Đang cài…"
			}
			if key == last {
				return
			}
			last = key
			u := domain.TaskUpdate{Phase: &phase}
			if progress != nil {
				u.Progress = progress
			}
			if err := s.tasks.Update(ctx, run.taskID, u); err == nil {
				s.emit(id)
			}
		}
		for {
			select {
			case <-stop:
				push()
				return
			case <-ticker.C:
				push()
			case <-ctx.Done():
				return
			}
		}
	}()
	return stop
}

// PhaseLabel đổi giai đoạn cài thành câu pha tiếng Việt — dùng cho việc nền
// và cho trường phaseLabel của /api/tools, /api/setup để web không phải dịch
// lại enum (giữ đúng nhãn của setup-steps.ts ở bản cũ).
func PhaseLabel(p *domain.InstallProgress) string {
	if p.Detail != "" {
		switch p.Detail {
		case "uv":
			return "Đang chuẩn bị trình cài Python"
		case "python 3.11 + venv + pinned wheels":
			return "Đang cài thư viện chép lời"
		case "faster-whisper models":
			return "Đang tải mô hình nhận giọng nói"
		case "silero VAD load":
			return "Đang kiểm tra lại"
		}
		if len(p.Detail) > len("python 3.11 + venv + pinned wheels") &&
			p.Detail[:len("python 3.11 + venv + pinned wheels")] == "python 3.11 + venv + pinned wheels" {
			return "Đang cài thư viện chép lời"
		}
	}
	switch p.Phase {
	case "checking":
		return "Đang kiểm tra"
	case "downloading":
		return "Đang tải về"
	case "installing":
		return "Đang cài"
	case "self-check":
		return "Đang kiểm tra lại"
	case "activating":
		return "Đang hoàn tất"
	}
	return "Đang cài…"
}

// installTitle là nhãn theo việc làm của hàng — giữ đúng copy của design.
func installTitle(id ToolID) string {
	switch id {
	case ToolYtDlp:
		return "Tải video và phụ đề"
	case ToolFfmpeg:
		return "Xử lý hình và tiếng"
	case ToolTranscription:
		return "Tách tiếng và tạo phụ đề"
	case ToolChrome:
		return "Dựng và kiểm tra video"
	case ToolCodex:
		return "Công cụ vẽ ảnh"
	case ToolCodexLogin:
		return "Đăng nhập ChatGPT"
	}
	return id
}
