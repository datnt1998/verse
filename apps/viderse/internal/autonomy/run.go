package autonomy

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// DispatchReviews chạy trong lượt poll của worker (sau khi lease attempt đã
// nhả): xét điểm các video đang Chờ duyệt mà kênh bật Tự duyệt.
// Toàn bộ ảnh ≥ ngưỡng → duyệt như nút "Duyệt hết" (cổng duyệt giữ nguyên) và
// ghi thông báo "Tự duyệt theo điểm"; có ảnh dưới ngưỡng → cả video ở lại Chờ
// duyệt kèm tóm tắt. Lỗi một video không chặn các video khác.
func (s *Service) DispatchReviews(ctx context.Context) error {
	if s.Reviewer == nil {
		return nil
	}
	pending, err := s.Store.PendingReviews(ctx, 50)
	if err != nil {
		return err
	}
	for _, p := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.reviewOne(ctx, p)
	}
	return nil
}

func (s *Service) reviewOne(ctx context.Context, p PendingReview) {
	if !p.AutoApprove {
		return
	}
	state, err := s.Reviewer.ReviewState(ctx, p.JobID)
	if err != nil {
		s.diag("tự duyệt %s: đọc ảnh: %v", p.JobID, err)
		return
	}
	if !state.Ready || state.Revision != p.ImageRevision {
		return // ảnh chưa đủ hoặc vừa đổi — lượt poll sau xét lại
	}
	// Ngưỡng cố định 85: mọi ảnh không bỏ qua phải có điểm và ≥ 85.
	verdict := Evaluate(domain.AutoApproveMin, state.Items)
	outcome := ReviewOutcome{JobID: p.JobID, Revision: state.Revision, Verdict: verdict, AutoApprove: true, Title: p.Title}
	if !verdict.AllPass {
		outcome.Result, outcome.Summary = ResultBelow, verdict.Summary()
		if err := s.Store.RecordReview(ctx, outcome); err != nil {
			s.diag("tự duyệt %s: ghi kết quả: %v", p.JobID, err)
			return
		}
		s.publishJob(p.JobID)
		return
	}
	// Toggle có thể tắt giữa lượt poll: đọc lại trước khi duyệt; tắt → bỏ lượt
	// này, không duyệt.
	on, err := s.Store.ChannelAutoApprove(ctx, p.ChannelID)
	if err != nil {
		s.diag("tự duyệt %s: đọc cờ kênh: %v", p.JobID, err)
		return
	}
	if !on {
		return
	}
	if err := s.Reviewer.ApproveAll(ctx, p.JobID, state.Revision); err != nil {
		outcome.Result, outcome.Tries, outcome.Summary = ResultError, p.Tries+1, err.Error()
		s.diag("tự duyệt %s: %v", p.JobID, err)
		if rerr := s.Store.RecordReview(ctx, outcome); rerr != nil {
			s.diag("tự duyệt %s: ghi lỗi: %v", p.JobID, rerr)
		}
		return
	}
	outcome.Result = ResultApproved
	if err := s.Store.RecordReview(ctx, outcome); err != nil {
		s.diag("tự duyệt %s: ghi thông báo: %v", p.JobID, err)
	}
	s.publishJob(p.JobID)
}

func (s *Service) publishJob(jobID string) {
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": jobID})
	}
}

// DefaultRetryDelay là khoảng nghỉ trước lần tự thử lại đầu tiên.
const DefaultRetryDelay = 3 * time.Second

// autoFixCode là chính sách tự sửa cố định sau V38 (07 §20): chỉ lỗi bước hỏng
// hẳn đã phân loại mới tự sửa, đúng một lần cho mỗi job-generation. Video chưa
// đạt Độ hoàn thiện (quality_gate) luôn hỏi nên không vào nhánh tự sửa; không
// đọc cấu hình Mức tự chủ (đã bỏ).
func autoFixCode(step domain.CloneStep, cerr *domain.CloneError) string {
	code := domain.AutoFixCodeOf(step, cerr)
	if code == "" || code == domain.AutoFixQuality {
		return ""
	}
	return code
}

// AutoFix được runner gọi khi một step vừa hỏng hẳn: theo chính sách cố định,
// thử lại đúng một lần với mã lỗi cho phép. Trả true khi step đã được đặt lại
// pending (runner claim lại ngay); false thì lỗi ở lại cho chủ kênh.
func (s *Service) AutoFix(ctx context.Context, jobID string, step domain.CloneStep, cerr *domain.CloneError) bool {
	code := autoFixCode(step, cerr)
	if code == "" {
		return false
	}
	delay := s.RetryDelay
	if delay == 0 {
		delay = DefaultRetryDelay
	}
	if delay > 0 {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
	}
	retry, err := s.Store.BeginAutoFix(ctx, jobID, step, code, s.now())
	if err != nil {
		s.diag("tự thử lại %s: %v", jobID, err)
		return false
	}
	if retry {
		s.publishJob(jobID)
	}
	return retry
}

// AttemptSettled được runner gọi sau mỗi attempt: step được tự thử lại mà đã
// xong thì autoFix → recovered.
func (s *Service) AttemptSettled(ctx context.Context, jobID string, step domain.CloneStep, status domain.CloneStatus) {
	if status != domain.CloneSucceeded {
		return
	}
	if err := s.Store.SettleAutoFix(ctx, jobID, step); err != nil {
		s.diag("tự thử lại %s: %v", jobID, err)
	}
}
