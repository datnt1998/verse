package scan

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// Chính sách thử lại của worker (chuyển nguyên văn từ bản TS).
const (
	RateLimitBase         = 30 * time.Second
	TransientBase         = 5 * time.Second
	BackoffMax            = 30 * time.Minute
	MaxConsecutiveFailure = 5
	MaxCursorRestarts     = 2
	// UploadBlockedPause là thời gian nghỉ thăm dò giờ đăng sau trang thử thách.
	UploadBlockedPause = 2 * time.Hour
	// UploadFailurePause là nghỉ sau 3 lỗi thăm dò liên tiếp.
	UploadFailurePause = 30 * time.Minute
	MaxUploadFailures  = 3

	busyChannelRetry = time.Second
	// cleanupBudget là trần thời gian nhả lease/ghi dọn khi worker dừng.
	cleanupBudget = 5 * time.Second
)

// Lỗi dừng của cả worker (nguyên nhân huỷ context, không phải lỗi mặt API).
var (
	errWorkerLeaseLost    = errors.New("scan: mất lease worker")
	errWorkerLeaseExpired = errors.New("scan: lease worker đã hết hạn")
	// errProbeFenced: video thăm dò nằm ở kênh đang bị process khác giữ fence;
	// worker chuyển sang chờ fence như danh sách, không chọn lại liên tục.
	errProbeFenced = errors.New("scan: kênh thăm dò đang bị giữ fence")
)

// Failure là phân loại lỗi của một lượt quét (classifyScanFailure của bản TS).
type Failure struct {
	Kind       FailureKind
	RetryAt    time.Time     // deferred: ô lịch toàn cục xa hơn ngân sách chờ
	RetryDelay time.Duration // rate_limited: Retry-After (0 = không có)
	Code       string        // transient/fatal: mã chẩn đoán
}

// FailureKind là loại lỗi đã phân loại.
type FailureKind int

const (
	FailureAbort FailureKind = iota
	FailureStopped
	FailureDeferred
	FailureRateLimited
	FailureTransient
	FailureBlocked
	FailureCursor
	FailureFatal
)

// ClassifyFailure xếp một lỗi của lượt quét vào hành động tương ứng.
func ClassifyFailure(err error) Failure {
	if err == nil {
		return Failure{Kind: FailureAbort}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Failure{Kind: FailureAbort}
	}
	if IsStopped(err) {
		return Failure{Kind: FailureStopped}
	}
	if StoreCodeOf(err) == StoreCursorInvalid {
		return Failure{Kind: FailureCursor}
	}
	var deferred *youtube.DeferredError
	if errors.As(err, &deferred) {
		return Failure{Kind: FailureDeferred, RetryAt: deferred.RetryAt}
	}
	var fe *youtube.FetchError
	if errors.As(err, &fe) {
		switch fe.Kind {
		case youtube.KindRateLimited:
			var delay time.Duration
			if fe.RetryDelay != nil {
				delay = *fe.RetryDelay
			}
			return Failure{Kind: FailureRateLimited, RetryDelay: delay}
		case youtube.KindTransient:
			return Failure{Kind: FailureTransient, Code: "transient"}
		case youtube.KindBlocked:
			return Failure{Kind: FailureBlocked}
		case youtube.KindCursorInvalid:
			return Failure{Kind: FailureCursor}
		default:
			return Failure{Kind: FailureFatal, Code: string(fe.Kind)}
		}
	}
	var notFound *youtube.ChannelNotFoundError
	if errors.As(err, &notFound) {
		return Failure{Kind: FailureFatal, Code: string(youtube.KindNotFound)}
	}
	return Failure{Kind: FailureTransient, Code: "unknown"}
}

// WorkerDeps của Worker: kho quét + provider YouTube + cổng nền tảng.
type WorkerDeps struct {
	Store    Store
	YouTube  Provider
	Service  *Service     // ghi việc nền + SSE + giờ chờ nền tảng; nil = tắt
	Platform PlatformGate // nil = không kiểm giờ chờ nền tảng
	Now      func() time.Time
	// Random trong [0,1) cho jitter backoff; mặc định nguồn ngẫu nhiên thật.
	Random func() float64
	// Heartbeat đè nhịp gia hạn lease (chỉ test); 0 = Heartbeat.
	Heartbeat time.Duration
	// BusyRetry đè nhịp chờ khi kênh đang được người khác giữ fence (chỉ test);
	// 0 = busyChannelRetry.
	BusyRetry time.Duration
}

// Worker là luồng quét nền duy nhất trong process: giữ lease trên
// platform_state('youtube'), xử lý một trang của một việc mỗi lượt dưới fence
// kênh, discovery trước lịch sử. Khi rảnh nó không poll DB: ngủ trên kênh wake,
// nhịp heartbeat (đọc hạn kế tiếp một lần) hoặc timer của hạn gần nhất.
type Worker struct {
	d              WorkerDeps
	now            func() time.Time
	random         func() float64
	wake           chan struct{}
	tick           chan struct{}
	heartbeatEvery time.Duration
	busyRetry      time.Duration

	mu           sync.Mutex
	channelLease *Lease
	jobID        string
	abortFetch   context.CancelFunc
	leaseUntil   atomic.Int64 // hạn lease worker (unix nano) — loop đọc không khoá
}

// leaseWatch canh hạn lease worker đang giữ một cách độc lập với nhịp
// heartbeat: timer bắn đúng Until và huỷ worker (kèm fetch đang bay); mỗi lần
// gia hạn thành công đặt lại hạn. Nhờ vậy hạn đã biết không phải chờ tick kế
// tiếp hay lượt trang kế tiếp.
type leaseWatch struct {
	mu      sync.Mutex
	cancel  context.CancelCauseFunc
	timer   *time.Timer
	until   time.Time
	stopped bool
}

func newLeaseWatch(cancel context.CancelCauseFunc) *leaseWatch {
	return &leaseWatch{cancel: cancel}
}

// arm đặt hạn mới cho watchdog (hoặc bắn ngay nếu đã quá hạn).
func (lw *leaseWatch) arm(until time.Time) {
	lw.mu.Lock()
	lw.until = until
	if lw.stopped {
		lw.mu.Unlock()
		return
	}
	d := time.Until(until)
	if d <= 0 {
		lw.mu.Unlock()
		lw.expire()
		return
	}
	if lw.timer == nil {
		lw.timer = time.AfterFunc(d, lw.expire)
	} else {
		lw.timer.Reset(d)
	}
	lw.mu.Unlock()
}

// expire huỷ worker khi hạn đã qua. Nếu một lần gia hạn vừa đẩy hạn ra xa,
// timer tự hẹn lại thay vì huỷ oan.
func (lw *leaseWatch) expire() {
	lw.mu.Lock()
	if lw.stopped {
		lw.mu.Unlock()
		return
	}
	now := time.Now()
	if now.Before(lw.until) {
		if lw.timer != nil {
			lw.timer.Reset(lw.until.Sub(now))
		}
		lw.mu.Unlock()
		return
	}
	lw.stopped = true
	lw.mu.Unlock()
	lw.cancel(errWorkerLeaseExpired)
}

func (lw *leaseWatch) stop() {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.stopped = true
	if lw.timer != nil {
		lw.timer.Stop()
	}
}

// NewWorker dựng worker; Store và YouTube bắt buộc.
func NewWorker(d WorkerDeps) *Worker {
	if d.Store == nil || d.YouTube == nil {
		panic("scan.NewWorker thiếu Store hoặc YouTube")
	}
	w := &Worker{
		d:    d,
		now:  d.Now,
		wake: make(chan struct{}, 1),
		tick: make(chan struct{}, 1),
	}
	if w.now == nil {
		w.now = time.Now
	}
	w.random = d.Random
	if w.random == nil {
		w.random = rand.Float64
	}
	w.heartbeatEvery = d.Heartbeat
	if w.heartbeatEvery <= 0 {
		w.heartbeatEvery = Heartbeat
	}
	w.busyRetry = d.BusyRetry
	if w.busyRetry <= 0 {
		w.busyRetry = busyChannelRetry
	}
	return w
}

// Wake đánh thức worker khi có thay đổi từ API (việc mới, resume, cap mới…).
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// rescanHours đọc khoảng quét lại (service giữ một đường đọc duy nhất).
func (w *Worker) rescanHours(ctx context.Context) int {
	if w.d.Service != nil {
		return w.d.Service.rescanHours(ctx)
	}
	return domain.RescanHoursDefault
}

// Run chiếm lease worker rồi chạy vòng lặp tới khi ctx huỷ hoặc lease mất.
// Khi lease đang do process khác giữ, worker ngủ và thử lại định kỳ. Huỷ từ
// caller (kể cả khi đang chờ câu lấy lease) là dừng sạch, không phải lỗi.
func (w *Worker) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	owner := newOwner()
	for {
		if ctx.Err() != nil {
			return nil
		}
		lease, err := w.acquire(ctx, owner)
		if err != nil {
			// Lỗi thật của kho vẫn lan ra; riêng lỗi do ctx caller vừa huỷ
			// trong lúc chờ câu lấy lease là dừng sạch.
			if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return nil
			}
			return err
		}
		if lease == nil {
			return nil
		}
		if ctx.Err() != nil {
			// Lease về cùng lúc ctx huỷ: vẫn trả lại có kiểm soát, không giữ
			// quyền của một worker đang dừng.
			w.releaseLease(*lease)
			return nil
		}
		err = w.serve(ctx, *lease, "")
		// Mất/hết hạn lease: quay lại nhận lease (một process khác có thể đang
		// giữ); lỗi khác của kho hoặc ctx dừng thì thoát.
		if errors.Is(err, errWorkerLeaseLost) || errors.Is(err, errWorkerLeaseExpired) {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		// Dừng theo ctx của caller không phải lỗi của worker.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return nil
		}
		return err
	}
}

// RunInline chạy vòng lặp của worker chỉ cho một kênh (CLI): thử lease worker
// đúng một lần, trả ran=false ngay khi một worker khác đang giữ (serve đang
// chạy) để CLI chỉ xếp việc và in trạng thái thay vì chờ.
func (w *Worker) RunInline(ctx context.Context, channelID string) (bool, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	lease, err := w.d.Store.AcquireWorkerLease(ctx, newOwner(), LeaseTTL)
	if err != nil {
		return false, err
	}
	if lease == nil {
		return false, nil
	}
	err = w.serve(ctx, *lease, channelID)
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		err = nil
	}
	return true, err
}

// acquire giữ lease worker, chờ nhẹ khi process khác đang giữ.
func (w *Worker) acquire(ctx context.Context, owner string) (*WorkerLease, error) {
	for {
		if ctx.Err() != nil {
			return nil, nil
		}
		lease, err := w.d.Store.AcquireWorkerLease(ctx, owner, LeaseTTL)
		if err != nil {
			return nil, err
		}
		if lease != nil {
			return lease, nil
		}
		if err := w.sleep(ctx, busyChannelRetry); err != nil {
			return nil, nil
		}
	}
}

// serve chạy vòng lặp + heartbeat + watchdog hạn lease, rồi join heartbeat
// trước khi nhả lease.
func (w *Worker) serve(ctx context.Context, lease WorkerLease, onlyChannel string) error {
	// Mọi fetch/commit chạy dưới workCtx: watchdog huỷ nó đúng hạn lease đã
	// biết, không chờ nhịp heartbeat.
	workCtx, stopWork := context.WithCancelCause(ctx)
	watch := newLeaseWatch(stopWork)
	defer watch.stop()
	defer stopWork(nil)
	watch.arm(lease.Until)
	w.leaseUntil.Store(lease.Until.UnixNano())

	stop := make(chan struct{})
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		w.heartbeat(workCtx, stopWork, lease, stop, watch)
	}()
	err := w.loop(workCtx, onlyChannel)
	close(stop)
	<-hbDone
	watch.stop()
	w.cancelFetch()
	w.releaseLease(lease)
	if cause := context.Cause(workCtx); cause != nil &&
		!errors.Is(cause, context.Canceled) && !errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return err
}

// releaseLease trả lease worker trong một ngân sách ngắn, kể cả khi ctx của
// worker đã huỷ — không giữ quyền của process đang dừng.
func (w *Worker) releaseLease(lease WorkerLease) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupBudget)
	defer cancel()
	_ = w.d.Store.ReleaseWorkerLease(ctx, lease)
}

// heartbeat gia hạn lease worker + fence kênh đang giữ mỗi 15 giây. Mất lease
// (gia hạn trả nil, hoặc lỗi kéo dài quá hạn) huỷ cả worker lẫn fetch đang bay.
func (w *Worker) heartbeat(ctx context.Context, cancel context.CancelCauseFunc, lease WorkerLease, stop chan struct{}, watch *leaseWatch) {
	ticker := time.NewTicker(w.heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
		}
		// Nhịp heartbeat cũng đánh thức vòng rảnh đọc lại hạn kế tiếp một lần.
		select {
		case w.tick <- struct{}{}:
		default:
		}
		// Gia hạn chạy dưới hạn lease đang giữ: DB treo quá hạn không thể kéo
		// dài quyền ghi (watchdog cũng huỷ cả workCtx đúng hạn).
		renewCtx, cancelRenew := context.WithDeadline(ctx, lease.Until)
		renewed, err := w.d.Store.RenewWorkerLease(renewCtx, lease, LeaseTTL)
		cancelRenew()
		if err != nil {
			if ctx.Err() != nil {
				return // hạn/mất lease đã huỷ worker
			}
			continue // DB bận nhưng còn hạn: nhịp sau thử lại
		}
		if renewed == nil {
			cancel(errWorkerLeaseLost)
			w.cancelFetch()
			return
		}
		lease = *renewed
		watch.arm(lease.Until)
		w.leaseUntil.Store(lease.Until.UnixNano())
		w.mu.Lock()
		ch := w.channelLease
		jobID := w.jobID
		w.mu.Unlock()
		if ch != nil {
			next, err := w.d.Store.RenewChannelLease(ctx, *ch)
			if err == nil && next != nil {
				w.mu.Lock()
				if w.channelLease != nil && w.channelLease.Token == next.Token {
					w.channelLease.Until = next.Until
				}
				w.mu.Unlock()
			} else if err == nil {
				w.cancelFetch()
			}
		}
		if jobID != "" {
			job, err := w.d.Store.Job(ctx, jobID)
			if err == nil && (job == nil || job.State != domain.ScanRunning) {
				w.cancelFetch()
			}
		}
	}
}

// leaseExpired báo lease worker đã quá hạn theo đồng hồ cục bộ.
func (w *Worker) leaseExpired() bool {
	until := w.leaseUntil.Load()
	return until != 0 && w.now().UnixNano() > until
}

// cancelFetch huỷ fetch đang bay của trang/thăm dò hiện tại.
func (w *Worker) cancelFetch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.abortFetch != nil {
		w.abortFetch()
	}
}

// setActive ghi fence + việc đang xử lý cho heartbeat biết. Heartbeat giữ một
// bản sao riêng của lease để gia hạn không sửa trường mà trang/thăm dò đang đọc
// (reader chỉ dùng bản gốc của lượt, store tự kiểm token+hạn).
func (w *Worker) setActive(lease Lease, jobID string, cancel context.CancelFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	active := lease
	w.channelLease = &active
	w.jobID = jobID
	w.abortFetch = cancel
}

func (w *Worker) clearActive() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.channelLease = nil
	w.jobID = ""
	w.abortFetch = nil
}

// loop là vòng chính: một trang một việc mỗi lượt.
func (w *Worker) loop(ctx context.Context, onlyChannel string) error {
	busy := map[string]bool{}
	fencedProbe := ""
	// Kênh vừa bị fence ở đường danh sách: nếu lựa chọn kế tiếp trả lại đúng
	// kênh đó thì chờ có kiểm soát thay vì chọn lại liên tục.
	lastFenced := ""
	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if w.leaseExpired() {
			return errWorkerLeaseExpired
		}
		rescan := w.rescanHours(ctx)

		// Nền tảng đang chờ: không request nào cho tới hết giờ chờ.
		if paused, err := w.platformPause(ctx); err != nil {
			return err
		} else if paused != nil {
			if err := w.idle(ctx, rescan); err != nil {
				return err
			}
			continue
		}

		if onlyChannel == "" {
			w.scheduleDueDiscovery(ctx, rescan)
		}
		job, err := w.nextRunnable(ctx, onlyChannel, busy)
		if err != nil {
			return err
		}
		if job == nil {
			if onlyChannel != "" {
				return nil // inline: không còn việc chạy được của kênh
			}
			if len(busy) > 0 {
				clear(busy)
				// Kênh đang bị giữ fence: việc của nó chưa claim được. Ngủ
				// tới hết hạn fence (Next) hoặc một nhịp chờ có kiểm soát —
				// không quay vòng claim; kênh khác còn chạy được thì đi tiếp.
				if err := w.idleFenced(ctx, rescan); err != nil {
					return err
				}
				continue
			}
			done, err := w.probeUploadTime(ctx)
			if errors.Is(err, errProbeFenced) {
				// Cùng ngữ nghĩa chờ fence như danh sách. Nếu lựa chọn vẫn trả
				// đúng kênh vừa bị fence (kho chưa lọc kịp hoặc chỉ còn nó),
				// ngủ một nhịp có kiểm soát thay vì chọn lại liên tục.
				ch := probeFencedChannel(err)
				if ch != "" && ch == fencedProbe {
					if err := w.sleep(ctx, w.busyRetry); err != nil {
						return err
					}
					continue
				}
				fencedProbe = ch
				if err := w.idleFenced(ctx, rescan); err != nil {
					return err
				}
				continue
			}
			fencedProbe = ""
			if err != nil {
				return err
			}
			if done {
				continue
			}
			if err := w.idle(ctx, rescan); err != nil {
				return err
			}
			continue
		}
		if job.ChannelID == lastFenced && onlyChannel == "" {
			// Kho vẫn trả lại đúng ứng viên vừa bị fence: ngủ một nhịp có
			// kiểm soát rồi mới thử lại, không chọn lại trong vòng lặp không
			// hẹn giờ. Nhịp ngắn để việc khác vừa chạy được vẫn không bị bỏ
			// lâu; wake/heartbeat/ctx đều đánh thức được.
			lastFenced = ""
			if err := w.sleep(ctx, w.busyRetry); err != nil {
				return err
			}
			continue
		}
		leaseCh, err := w.d.Store.AcquireChannelLease(ctx, job.ChannelID, "worker", LeaseTTL)
		if err != nil {
			return err
		}
		if leaseCh == nil {
			if onlyChannel != "" {
				// Inline: kênh đang được người khác giữ fence — chờ có kiểm soát
				// (ctx/wake/nhịp) thay vì quay vòng claim trên cùng một việc.
				if err := w.sleep(ctx, w.busyRetry); err != nil {
					return err
				}
				continue
			}
			lastFenced = job.ChannelID
			busy[job.ChannelID] = true
			continue
		}
		lastFenced = ""
		delete(busy, job.ChannelID)
		claimed, err := w.d.Store.ClaimJob(ctx, job.ID, *leaseCh)
		if err != nil {
			_ = w.d.Store.ReleaseChannelLease(ctx, *leaseCh)
			if IsStopped(err) {
				continue
			}
			return err
		}
		if claimed == nil {
			_ = w.d.Store.ReleaseChannelLease(ctx, *leaseCh)
			continue
		}
		pageErr := w.runOnePage(ctx, claimed, *leaseCh)
		_ = w.d.Store.ReleaseChannelLease(ctx, *leaseCh)
		if pageErr != nil {
			return pageErr
		}
	}
}

// nextRunnable lấy việc kế tiếp: inline chỉ xét đúng kênh được yêu cầu.
func (w *Worker) nextRunnable(ctx context.Context, onlyChannel string, busy map[string]bool) (*Job, error) {
	if onlyChannel != "" {
		return w.d.Store.NextRunnableJobForChannel(ctx, onlyChannel)
	}
	return w.d.Store.NextRunnableJob(ctx, busy)
}

// platformPause trả giờ chờ nền tảng còn hiệu lực (nil = không chờ).
func (w *Worker) platformPause(ctx context.Context) (*time.Time, error) {
	if w.d.Platform == nil {
		return nil, nil
	}
	until, err := w.d.Platform.Until(ctx, domain.PlatformYouTube)
	if err != nil {
		return nil, err
	}
	if until == nil || !until.After(w.now()) {
		return nil, nil
	}
	return until, nil
}

// scheduleDueDiscovery xếp việc discovery cho các kênh đến hạn quét lại. Lịch
// tự động không khôi phục việc đã dừng (replaceStopped=false).
func (w *Worker) scheduleDueDiscovery(ctx context.Context, rescanHours int) {
	due, err := w.d.Store.DueChannels(ctx, rescanHours, w.now())
	if err != nil {
		return // vòng sau thử lại
	}
	for _, channelID := range due {
		created, err := w.d.Store.StartDiscovery(ctx, channelID, false)
		if err == nil && created && w.d.Service != nil {
			w.d.Service.emit(channelID)
		}
	}
}

// idle chặn tới hạn gần nhất còn ở tương lai, hoặc tới wake/nhịp heartbeat/ctx.
// Một lần đọc lịch duy nhất (NextWakeup) phân biệt "có việc ngay" với "không có":
// việc queued/recovered, backoff/rescan đến hạn khiến worker rời rảnh ngay; hạn
// đã qua không bao giờ trở thành giá trị ngủ. Giờ chờ nền tảng là rào "không
// sớm hơn" và được đọc lại mỗi tick — không phải hạn của việc. Không có hạn nào
// thì không đọc DB nữa: chỉ nhịp heartbeat lặp lần đọc lịch đó.
func (w *Worker) idle(ctx context.Context, rescanHours int) error {
	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		now := w.now()
		var deadline *time.Time
		pause, err := w.platformPause(ctx)
		if err != nil {
			return err
		}
		if pause != nil && pause.After(now) {
			// Rào "không sớm hơn": đang chờ nền tảng thì không việc nào chạy
			// được, kể cả việc đang actionable — ngủ tới hết giờ chờ.
			deadline = pause
		} else {
			wakeup, err := w.d.Store.NextWakeup(ctx, rescanHours, now)
			if err != nil {
				return err
			}
			if wakeup.Actionable {
				return nil // có việc chạy được ngay: về vòng chính
			}
			if wakeup.Next != nil && wakeup.Next.After(now) {
				deadline = wakeup.Next
			}
		}

		var timer *time.Timer
		var fire <-chan time.Time
		if deadline != nil {
			d := deadline.Sub(now)
			if d < IdleFloor {
				d = IdleFloor
			}
			timer = time.NewTimer(d)
			fire = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return context.Cause(ctx)
		case <-w.wake:
			if timer != nil {
				timer.Stop()
			}
			return nil
		case <-w.tick:
			if timer != nil {
				timer.Stop()
			}
			continue // nhịp heartbeat: đọc lại hạn một lần rồi ngủ tiếp
		case <-fire:
			return nil
		}
	}
}

// idleFenced là idle khi việc/kênh hiện tại đang bị process khác giữ fence:
// việc/kênh khác còn claim được (Actionable) thì đi tiếp ngay; nếu không thì
// ngủ tới hạn fence kế tiếp hoặc một nhịp chờ có kiểm soát.
func (w *Worker) idleFenced(ctx context.Context, rescanHours int) error {
	for {
		now := w.now()
		pause, err := w.platformPause(ctx)
		if err != nil {
			return err
		}
		if pause != nil && pause.After(now) {
			return w.sleepUntil(ctx, *pause)
		}
		wakeup, err := w.d.Store.NextWakeup(ctx, rescanHours, now)
		if err != nil {
			return err
		}
		if wakeup.Actionable {
			return nil // kênh/việc khác chạy được ngay: để vòng chính đi tiếp
		}
		if wakeup.Next != nil && wakeup.Next.After(now) {
			return w.sleepUntil(ctx, *wakeup.Next)
		}
		// Không biết hạn nào (kể cả fence): chờ ngắn có kiểm soát rồi thử lại.
		if err := w.sleep(ctx, w.busyRetry); err != nil {
			return err
		}
		return nil
	}
}

// probeFencedChannel lấy id kênh từ lỗi thăm dò bị fence.
func probeFencedChannel(err error) string {
	const prefix = "scan: kênh thăm dò đang bị giữ fence: "
	if err == nil {
		return ""
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return ""
	}
	return strings.TrimPrefix(msg, prefix)
}

// sleepUntil ngủ tới mốc at (sàn IdleFloor), thức dậy khi ctx huỷ.
func (w *Worker) sleepUntil(ctx context.Context, at time.Time) error {
	d := at.Sub(w.now())
	if d < IdleFloor {
		d = IdleFloor
	}
	return w.sleep(ctx, d)
}

// sleep ngủ có thể bị đánh thức bởi Wake(), nhịp heartbeat hoặc ctx.
func (w *Worker) sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-w.wake:
		return nil
	case <-w.tick:
		return nil
	case <-timer.C:
		return nil
	}
}

// ── một trang của một việc ─────────────────────────────────────────────────

// runOnePage đọc + commit một trang của việc; lỗi của YouTube được ghi dưới
// fence rồi kết thúc lượt, lỗi DB lan ra để worker dừng (serve báo lỗi).
func (w *Worker) runOnePage(ctx context.Context, job *Job, lease Lease) error {
	// Nền tảng có thể bị chặn giữa hai lượt: không request, không ghi lỗi.
	if paused, err := w.platformPause(ctx); err != nil {
		return err
	} else if paused != nil {
		return nil // việc vẫn running; lượt sau tiếp tục dưới cùng fence
	}

	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	w.setActive(lease, job.ID, cancel)
	defer w.clearActive()
	sync := w.sync()
	// Trang đầu sau khi claim: dòng việc nền nhận process hiện tại làm chủ
	// (khôi phục sau crash) — chỉ một lần cho mỗi lượt claim.
	sync(ctx, job, taskSync{})

	history := job.Kind == domain.ScanHistory
	state, err := w.d.Store.EnsureState(ctx, job.ChannelID)
	if err != nil {
		if IsStopped(err) {
			return nil
		}
		return err
	}
	if history && state.IndexedVideos >= state.HistoryCap {
		if err := w.d.Store.SetJobCapped(ctx, job.ID, lease); err != nil && !IsStopped(err) {
			return err
		}
		w.refreshTask(ctx, job)
		return nil
	}

	var page Page
	if job.Remainder != "" {
		// Phần dư đã lưu luôn được commit trước khi đi theo cursor.
		stored, perr := youtube.ParseScannedVideos(job.Remainder)
		if perr != nil {
			return w.recordFailure(ctx, job, lease, perr)
		}
		page.Videos = stored
		if job.Cursor != "" {
			cursor, cerr := youtube.ParseChannelCursor(job.Cursor)
			if cerr != nil {
				return w.recordFailure(ctx, job, lease, cerr)
			}
			page.NextCursor = &cursor
		}
	} else {
		var fetched *youtube.ChannelVideoPage
		if job.Cursor != "" {
			cursor, cerr := youtube.ParseChannelCursor(job.Cursor)
			if cerr != nil {
				return w.recordFailure(ctx, job, lease, cerr)
			}
			fetched, err = w.d.YouTube.NextChannelPage(fetchCtx, cursor)
		} else {
			fetched, err = w.d.YouTube.FirstChannelPage(fetchCtx, job.ChannelID)
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Fetch bị heartbeat huỷ (lease/job mất): bỏ lượt, không ghi.
			if fetchCtx.Err() != nil {
				return nil
			}
			return w.recordFailure(ctx, job, lease, err)
		}
		page = Page{Videos: fetched.Videos, NextCursor: fetched.NextCursor}
	}

	// Cắt trang theo cap (history) hoặc khối khoá (discovery); phần dư lưu lại.
	commit := page.Videos
	var remainder []youtube.ScannedVideo
	if history {
		room := state.HistoryCap - state.IndexedVideos
		known, kerr := w.d.Store.FindKnownVideoIDs(ctx, job.ChannelID, videoIDs(commit))
		if kerr != nil {
			return kerr
		}
		fresh := 0
		cut := -1
		for i, v := range commit {
			if known[v.ID] {
				continue
			}
			if fresh >= room {
				cut = i
				break
			}
			fresh++
		}
		if cut >= 0 {
			remainder = commit[cut:]
			commit = commit[:cut]
		}
	} else {
		fit := DiscoveryBlock - job.NextOrdinal
		if len(commit) > fit {
			remainder = commit[fit:]
			commit = commit[:fit]
		}
	}

	input := CommitPageInput{
		ChannelID:  job.ChannelID,
		Videos:     commit,
		NextCursor: page.NextCursor,
		Remainder:  remainder,
		Origin:     domain.OriginDiscovery,
		Lease:      lease,
		JobID:      job.ID,
		Now:        w.now(),
	}
	if history {
		input.Origin = domain.OriginHistory
	} else if job.Cursor == "" && job.Remainder == "" && job.NewestID == "" {
		// Trang đầu của run từ đầu tab mang id mới nhất cho mốc head.
		var newest *string
		if len(commit) > 0 {
			newest = new(commit[0].ID)
		} else {
			newest = new("")
		}
		input.NewestVideoID = newest
	}
	result, err := w.d.Store.CommitPage(ctx, input)
	if err != nil {
		if ctx.Err() != nil || IsStopped(err) {
			return nil
		}
		switch StoreCodeOf(err) {
		case StoreKeyRange, StoreLimit:
			return w.setError(ctx, job, lease, "invalid_page", err, RecoveryFatal)
		default:
			return w.recordFailure(ctx, job, lease, err)
		}
	}
	if w.d.Service != nil {
		w.d.Service.emit(job.ChannelID)
	}
	// Tiến độ là lần ghi duy nhất ở đây; writer tự giới hạn ≤ 4/s/kênh.
	cur := *job
	cur.Scanned += result.Scanned
	cur.Added += result.Added
	sync(ctx, &cur, taskSync{progress: true})

	ended := page.NextCursor == nil && len(remainder) == 0
	if history {
		if ended {
			if err := w.d.Store.CompleteJob(ctx, job.ID, lease); err != nil && !IsStopped(err) {
				return err
			}
			w.refreshTask(ctx, job)
			return nil
		}
		if st, err := w.d.Store.EnsureState(ctx, job.ChannelID); err == nil && st.IndexedVideos >= st.HistoryCap {
			if err := w.d.Store.SetJobCapped(ctx, job.ID, lease); err == nil {
				w.refreshTask(ctx, job)
			}
		}
		return nil
	}
	// Discovery: gặp anchor hoặc hết tab → head check đóng việc trong một tx.
	if result.Overlap || ended {
		current, err := w.d.Store.Job(ctx, job.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return nil
		}
		in := HeadCheckInput{
			CheckedAt: w.now(),
			Lease:     lease,
			JobID:     job.ID,
			Now:       w.now(),
		}
		if current.NewestID != "" {
			in.NewestID = &current.NewestID
		}
		if _, err := w.d.Store.CompleteHeadCheck(ctx, job.ChannelID, in); err != nil {
			if IsStopped(err) {
				return nil
			}
			return w.recordFailure(ctx, job, lease, err)
		}
		w.refreshTask(ctx, job)
		if w.d.Service != nil {
			w.d.Service.emit(job.ChannelID)
		}
		return nil
	}
	if result.NextOrdinal >= DiscoveryBlock {
		return w.setError(ctx, job, lease, "gap_incomplete",
			errors.New("no known video found within the 10,000-video recent-gap budget"), RecoveryGap)
	}
	return nil
}

// sync trả hàm đồng bộ dòng Việc nền của service (no-op khi CLI không có).
func (w *Worker) sync() func(context.Context, *Job, taskSync) {
	if w.d.Service == nil {
		return func(context.Context, *Job, taskSync) {}
	}
	return w.d.Service.syncTask
}

// refreshTask đọc lại việc rồi đồng bộ dòng Việc nền (sau khi trạng thái đổi).
func (w *Worker) refreshTask(ctx context.Context, job *Job) {
	if w.d.Service == nil {
		return
	}
	if cur, err := w.d.Store.Job(ctx, job.ID); err == nil && cur != nil {
		w.d.Service.syncTask(ctx, cur, taskSync{})
	}
}

// videoIDs liệt kê id của một trang.
func videoIDs(videos []youtube.ScannedVideo) []string {
	ids := make([]string, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}
	return ids
}

// recordFailure ghi một lỗi đã phân loại dưới fence; ghi hỏng vì fence/việc
// dừng chỉ làm rơi lượt (trả nil), lỗi DB thật lan ra để worker dừng.
func (w *Worker) recordFailure(ctx context.Context, job *Job, lease Lease, cause error) error {
	f := ClassifyFailure(cause)
	switch f.Kind {
	case FailureAbort, FailureStopped:
		return nil
	case FailureDeferred:
		return w.backoff(ctx, job, lease, Backoff{
			NextAttemptAt: f.RetryAt,
			Attempts:      job.Attempts,
			ErrorCode:     "rate_limited_wait",
			Err:           errors.New("waiting for the global rate-limit window"),
		})
	case FailureBlocked:
		w.recordPlatformBlocked(ctx, 0)
		return w.setError(ctx, job, lease, "blocked", cause, RecoveryBlocked)
	case FailureFatal:
		return w.setError(ctx, job, lease, f.Code, cause, RecoveryFatal)
	case FailureCursor:
		if job.Restarts >= MaxCursorRestarts {
			err := w.writeFailure(func() error {
				return w.d.Store.SetJobPaused(ctx, job.ID, lease, JobError{
					ErrorCode: "cursor_restart_limit",
					Err:       cause,
					Recovery:  RecoveryRestarts,
				})
			})
			w.refreshTask(ctx, job)
			return err
		}
		err := w.writeFailure(func() error {
			return w.d.Store.RestartJobCursor(ctx, job.ID, lease, cause)
		})
		w.refreshTask(ctx, job)
		return err
	case FailureRateLimited, FailureTransient:
		attempts := job.Attempts + 1
		code := f.Code
		if f.Kind == FailureRateLimited {
			code = "rate_limited"
			// 429/Retry-After được ghi bền cho mọi process cùng tôn trọng.
			w.recordPlatformBlocked(ctx, f.RetryDelay)
		}
		if attempts >= MaxConsecutiveFailure {
			return w.setErrorAttempts(ctx, job, lease, code, cause, RecoveryFailures, new(attempts))
		}
		delay := f.RetryDelay
		if f.Kind == FailureRateLimited && delay > 0 {
			// Retry-After của server được tôn trọng nguyên vẹn.
		} else {
			base := TransientBase
			if f.Kind == FailureRateLimited {
				base = RateLimitBase
			}
			delay = scanBackoffDelay(base, job.Attempts, w.random)
		}
		return w.backoff(ctx, job, lease, Backoff{
			NextAttemptAt: w.now().Add(delay),
			Attempts:      attempts,
			ErrorCode:     code,
			Err:           cause,
		})
	}
	return nil
}

// recordPlatformBlocked ghi giờ chờ nền tảng qua service (cùng dữ liệu UI đọc).
func (w *Worker) recordPlatformBlocked(ctx context.Context, retryAfter time.Duration) {
	if w.d.Service != nil {
		w.d.Service.recordPlatformBlocked(ctx, retryAfter)
		return
	}
	if w.d.Platform != nil {
		_ = w.d.Platform.RecordBlocked(ctx, domain.PlatformYouTube, retryAfter)
	}
}

// backoff ghi backoff rồi làm mới dòng Việc nền (trạng thái waiting).
func (w *Worker) backoff(ctx context.Context, job *Job, lease Lease, b Backoff) error {
	err := w.writeFailure(func() error {
		return w.d.Store.SetJobBackoff(ctx, job.ID, lease, b)
	})
	if err == nil {
		w.refreshTask(ctx, job)
	}
	return err
}

// setError đỗ việc ở trạng thái error kèm câu hướng dẫn.
func (w *Worker) setError(ctx context.Context, job *Job, lease Lease, code string, cause error, recovery string) error {
	return w.setErrorAttempts(ctx, job, lease, code, cause, recovery, nil)
}

func (w *Worker) setErrorAttempts(ctx context.Context, job *Job, lease Lease, code string, cause error, recovery string, attempts *int) error {
	err := w.writeFailure(func() error {
		return w.d.Store.SetJobError(ctx, job.ID, lease, JobError{
			ErrorCode: code,
			Err:       cause,
			Recovery:  recovery,
			Attempts:  attempts,
		})
	})
	if err == nil {
		w.refreshTask(ctx, job)
	}
	return err
}

// writeFailure chạy một ghi lỗi; lỗi "đã dừng" của chính ghi đó bị nuốt
// (fence đổi chủ hay việc đã dừng không còn là lỗi của lượt này).
func (w *Worker) writeFailure(write func() error) error {
	if err := write(); err != nil {
		if IsStopped(err) {
			return nil
		}
		return err
	}
	return nil
}

// ── thăm dò giờ đăng chính xác ─────────────────────────────────────────────

// probeUploadTime đọc giờ đăng (+phụ đề kênh) của một video dưới fence kênh
// khi không còn việc listing; trả true khi một thăm dò đã chạy.
func (w *Worker) probeUploadTime(ctx context.Context) (bool, error) {
	if paused, err := w.platformPause(ctx); err != nil {
		return false, err
	} else if paused != nil {
		return false, nil
	}
	candidate, err := w.d.Store.NextUploadCandidate(ctx, w.now())
	if err != nil {
		return false, err
	}
	if candidate == nil {
		return false, nil
	}
	lease, err := w.d.Store.AcquireChannelLease(ctx, candidate.ChannelID, "probe", LeaseTTL)
	if err != nil {
		return false, err
	}
	if lease == nil {
		// Kênh bị process khác giữ: chờ fence (kèm id để tránh chọn lặp).
		return false, fmt.Errorf("%w: %s", errProbeFenced, candidate.ChannelID)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), cleanupBudget)
		defer cancel()
		_ = w.d.Store.ReleaseChannelLease(cleanup, *lease)
	}()

	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	w.setActive(*lease, "", cancel)
	defer w.clearActive()

	uploadedAt, hasCaptions, ferr := w.d.YouTube.FetchUploadTime(fetchCtx, candidate.VideoID)
	if ferr != nil {
		if ctx.Err() != nil {
			return false, context.Cause(ctx)
		}
		if fetchCtx.Err() != nil {
			return false, nil // heartbeat huỷ fetch: bỏ lượt
		}
		f := ClassifyFailure(ferr)
		switch f.Kind {
		case FailureBlocked:
			w.recordPlatformBlocked(ctx, 0)
			_ = w.d.Store.PauseUploads(ctx, w.now().Add(UploadBlockedPause))
			return false, nil
		case FailureRateLimited, FailureDeferred:
			delay := f.RetryDelay
			if f.Kind == FailureDeferred {
				delay = f.RetryAt.Sub(w.now())
			}
			if delay > 0 {
				w.recordPlatformBlocked(ctx, delay)
				_ = w.d.Store.PauseUploads(ctx, w.now().Add(delay))
			}
			return false, nil
		default:
			failures, berr := w.d.Store.BumpUploadFailures(ctx)
			if berr != nil {
				return false, berr
			}
			if failures >= MaxUploadFailures {
				_ = w.d.Store.PauseUploads(ctx, w.now().Add(UploadFailurePause))
				_ = w.d.Store.ResetUploadFailures(ctx)
			}
			return true, nil
		}
	}
	_ = w.d.Store.ResetUploadFailures(ctx)
	if err := w.d.Store.RecordUploadTime(ctx, *lease, candidate.VideoID, uploadedAt, hasCaptions, w.now()); err != nil {
		if IsStopped(err) {
			return false, nil
		}
		return false, err
	}
	if w.d.Service != nil {
		w.d.Service.emit(candidate.ChannelID)
	}
	return true, nil
}
