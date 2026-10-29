package imagegen

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Trạng thái của một attempt trên journal.
const (
	ReceiptRunning       = "running"        // đã ghi, chưa có kết quả — nghi đã qua điểm tiêu tiền
	ReceiptOK            = "ok"             // ảnh đã lưu và provenance hợp lệ
	ReceiptFailed        = "failed"         // thất bại đã kết luận
	ReceiptFailedUnknown = "failed_unknown" // crash chưa phân định: khoá key, không vẽ lại mù
	ReceiptAbandoned     = "abandoned"      // chủ hữu quyết định vẽ lại; giữ làm bằng chứng
)

// JournalEntry là một attempt của một draw key — mỗi attempt tối đa một lần
// có thể tiêu tiền.
type JournalEntry struct {
	Key        string
	Attempt    int
	Provider   string
	Model      string
	Output     string
	State      string
	Charged    bool
	ErrorCode  string
	ErrorText  string
	Receipt    *Receipt
	Scratch    json.RawMessage
	StartedAt  time.Time
	FinishedAt *time.Time
}

// Admission là yêu cầu mở một attempt, gồm luôn trần ngày để store chấp nhận
// hoặc từ chối NGUYÊN TỬ với việc ghi attempt (không read-then-insert).
type Admission struct {
	Entry JournalEntry
	// PaidCap > 0: provider tính phí theo ảnh — store phải chặn khi số attempt
	// charged của provider từ DayStart đã đạt PaidCap. PaidCap < 0: không chặn.
	PaidCap  int
	DayStart time.Time
}

// Journal là biên nhận bền vững của các lần vẽ trả phí.
type Journal interface {
	// LockKey giành quyền sở hữu logic của draw key qua ranh giới process
	// (advisory lock DB, hoặc mutex trong-process cho journal không-DB).
	// Trả hàm mở; luôn gọi nó (defer).
	LockKey(ctx context.Context, key string) (unlock func(), err error)
	// Begin ghi MỘT attempt trả phí trong cùng giao dịch với việc kiểm trần
	// ngày: vượt trần → *CapError (không ghi gì); đã có attempt running của
	// key → ErrAttemptConcluded (caller phải settle lại). Attempt = MAX+1 chỉ
	// là số thứ tự; độc quyền do LockKey + unique running index bảo đảm.
	Begin(ctx context.Context, a Admission) (JournalEntry, error)
	// Latest trả attempt mới nhất chưa abandoned (nil khi chưa có).
	Latest(ctx context.Context, key string) (*JournalEntry, error)
	// Finish chỉ hoàn tất attempt đang running của ta; attempt đã bị chủ khác
	// kết luận → ErrAttemptConcluded (fence, không ghi đè quyết định).
	Finish(ctx context.Context, key string, attempt int, state string, charged bool, receipt *Receipt, errCode, errText string, finishedAt time.Time) error
	// UpdateScratch ghi đè scratch của attempt còn running.
	UpdateScratch(ctx context.Context, key string, attempt int, scratch json.RawMessage) error
	// Charged đếm attempt của provider đã đánh dấu có thể tốn phí từ since.
	Charged(ctx context.Context, provider string, since time.Time) (int64, error)
	// Abandon gỡ trạng thái chặn của các attempt failed/failed_unknown (bản
	// ghi giữ làm bằng chứng).
	Abandon(ctx context.Context, key string) error
}

// journalHandle là JournalHandle ràng với một attempt cụ thể.
type journalHandle struct {
	journal Journal
	key     string
	attempt int
}

func (h journalHandle) UpdateScratch(ctx context.Context, scratch json.RawMessage) error {
	return h.journal.UpdateScratch(ctx, h.key, h.attempt, scratch)
}

// nextLocalMidnight trả nửa đêm kế tiếp theo lịch của loc — DST-safe
// (AddDate, không phải +24h: ngày chuyển giờ có 23/25 tiếng).
func nextLocalMidnight(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
}

// localDayStart trả nửa đêm của ngày chứa now theo loc của now.
func localDayStart(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

// capError dựng lỗi trần ngày với mốc reset kế tiếp.
func capError(provider string, used int64, limit int, now time.Time) *CapError {
	return &CapError{Provider: provider, Used: used, Limit: limit, RetryAt: nextLocalMidnight(now)}
}

// ErrCapReached: store báo trần đã hết trong Begin (không ghi attempt).
// Store tự dựng (in-memory, test) dùng hàm này để giữ đúng sentinel.
func ErrCapReached(provider string, used int64, limit int) error {
	return fmt.Errorf("trần ảnh trả phí của %s: %w", provider, &CapReachedError{Provider: provider, Used: used, Limit: limit})
}

// CapReachedError là sentinel của store khi từ chối admission vì trần.
type CapReachedError struct {
	Provider string
	Used     int64
	Limit    int
}

func (e *CapReachedError) Error() string {
	return fmt.Sprintf("đã dùng %d/%d ảnh trả phí của %s", e.Used, e.Limit, e.Provider)
}
