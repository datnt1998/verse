package imagegen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// Options là phụ thuộc tiêm vào của Service.
type Options struct {
	// Providers theo thứ tự ưu tiên: phần tử đầu là nguồn chính, còn lại là
	// dự phòng khi Fallback bật (bản cũ: primary = clone.imageProvider).
	Providers []Provider
	// Fallback cho phép vẽ lại bằng provider còn lại khi primary lỗi
	// (clone.imageFallback). Mặc định bật.
	Fallback *bool
	// Journal: biên nhận trả phí — bắt buộc.
	Journal Journal
	// DailyPaidImageLimit là trần ảnh trả phí mỗi ngày cho mỗi provider có
	// Paid() (D7). nil → mặc định 50; 0 = máy không được tự tiêu. Giá trị âm
	// cũng về mặc định (phòng cấu hình sai).
	DailyPaidImageLimit *int
	// Now/Sleep là đồng hồ và ngủ tiêm vào cho test.
	Now   func() time.Time
	Sleep func(ctx context.Context, d time.Duration) error
	// TransientRetries: số lần retry sau lỗi transport chắc chắn tạm thời
	// (bản cũ: 2 lần — chờ 5 s rồi 20 s). nil → 2; 0 = không retry. Retry là
	// một attempt trả phí mới.
	TransientRetries *int
}

// Service là điểm duy nhất đường ảnh trả phí đi qua — port của drawImage:
// provider đã chọn vẽ trước; với fallback bật, mọi lỗi provider trừ huỷ vẽ
// lại một lần bằng provider còn lại khi nó sẵn sàng. Lỗi biên nhận (DB) KHÔNG
// bao giờ rơi vào dự phòng trả phí. Provider vừa báo hết hạn mức bị bỏ qua
// tới mốc reset của nó. Khi cả hai lỗi, lỗi của primary là thứ hàng chờ thấy.
type Service struct {
	opts    Options
	limit   int
	now     func() time.Time
	sleep   func(ctx context.Context, d time.Duration) error
	retries int

	mu        sync.Mutex // chỉ giữ skipUntil/skipErr
	skipUntil map[string]time.Time
	skipErr   map[string]error
}

// quotaSkipFallback là thời gian bỏ qua provider khi nó báo quota mà không
// kèm mốc reset (bản cũ: 30 phút).
const quotaSkipFallback = 30 * time.Minute

// NewService dựng service trên journal + providers đã tiêm.
func NewService(opts Options) *Service {
	limit := DefaultDailyPaidImageLimit
	if opts.DailyPaidImageLimit != nil && *opts.DailyPaidImageLimit >= 0 {
		limit = *opts.DailyPaidImageLimit
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	retries := 2
	if opts.TransientRetries != nil {
		retries = *opts.TransientRetries
		if retries < 0 {
			retries = 0
		}
	}
	fallback := opts.Fallback == nil || *opts.Fallback
	s := &Service{opts: opts, limit: limit, now: now, sleep: sleep, retries: retries,
		skipUntil: map[string]time.Time{}, skipErr: map[string]error{}}
	if !fallback {
		f := false
		s.opts.Fallback = &f
	}
	return s
}

// DailyLimit trả trần ảnh trả phí ngày đang áp.
func (s *Service) DailyLimit() int { return s.limit }

// fallbackOn đọc cờ dự phòng (mặc định bật như bản cũ).
func (s *Service) fallbackOn() bool {
	return s.opts.Fallback == nil || *s.opts.Fallback
}

// providers trả (primary, secondaries) theo thứ tự đã cấu hình.
func (s *Service) providers() (Provider, []Provider) {
	if len(s.opts.Providers) == 0 {
		return nil, nil
	}
	return s.opts.Providers[0], s.opts.Providers[1:]
}

// ProviderLabel là nhãn hiển thị của provider (bản cũ: ChatGPT | Grok).
func ProviderLabel(name string) string {
	switch name {
	case "codex":
		return "ChatGPT"
	case "grok":
		return "Grok"
	default:
		return name
	}
}

// sleepCtx ngủ theo ctx — bản thực của Sleep khi không tiêm.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Draw thực hiện một yêu cầu vẽ: độc quyền logic theo Key qua ranh giới
// process, dự phòng theo provider, retry busy/tạm thời theo quy tắc bản cũ.
func (s *Service) Draw(ctx context.Context, req Request) (Receipt, error) {
	if req.Key == "" {
		return Receipt{}, drawErr(ErrCodeInvalidRequest, "Thiếu khoá idempotent của lần vẽ", "")
	}
	if req.Output == "" {
		return Receipt{}, drawErr(ErrCodeInvalidRequest, "Thiếu đường dẫn ảnh đích", "")
	}
	if s.opts.Journal == nil {
		return Receipt{}, &StoreError{Err: errors.New("imagegen chưa có journal")}
	}
	primary, _ := s.providers()
	if primary == nil {
		return Receipt{}, drawErr(ErrCodeInternal, "imagegen chưa có provider", "")
	}

	// Độc quyền key là quyết định của tầng lưu trữ: advisory lock DB (qua
	// process) hoặc mutex của journal không-DB. Giữ suốt chuỗi
	// settle → draw → retry → fallback.
	unlock, err := s.opts.Journal.LockKey(ctx, req.Key)
	if err != nil {
		return Receipt{}, &StoreError{Err: err}
	}
	defer unlock()

	// Đối chiếu attempt bị gián đoạn trước: không bao giờ vẽ lại mù.
	settled, err := s.settle(ctx, req)
	if err != nil {
		return Receipt{}, err
	}
	if settled != nil {
		return *settled, nil
	}

	var lastErr error
	for busy := 0; ; busy++ {
		receipt, err := s.drawTransient(ctx, req)
		if err == nil {
			return receipt, nil
		}
		lastErr = err
		if !IsProviderBusy(err) || busy >= ProviderBusyRetries {
			return Receipt{}, err
		}
		if s.sleep(ctx, ProviderBusyRetryDelay) != nil {
			return Receipt{}, lastErr
		}
	}
}

// settle đối chiếu attempt mới nhất của key trước khi tính tiếp:
//   - ok → trả receipt (idempotent, không vẽ lại);
//   - running → thử cứu output theo luật phục hồi của bản cũ; không cứu được
//     thì đánh failed_unknown và khoá key — không vẽ lại mù;
//   - failed_unknown → lỗi vẫn đứng cho tới khi chủ hữu Abandon;
//   - failed → cho phép một attempt mới (vẽ lại tường minh cùng key).
//
// Caller đã giữ LockKey của key nên không cần khoá phụ.
func (s *Service) settle(ctx context.Context, req Request) (*Receipt, error) {
	e, err := s.opts.Journal.Latest(ctx, req.Key)
	if err != nil {
		return nil, &StoreError{Err: err}
	}
	if e == nil {
		return nil, nil
	}
	if e.Output != "" && req.Output != "" && e.Output != req.Output {
		return nil, drawErr(ErrCodeInvalidRequest, "Phiên tạo ảnh không khớp output", "")
	}
	switch e.State {
	case ReceiptOK:
		if e.Receipt == nil {
			return nil, &DrawError{Code: ErrCodeInternal, Message: "Biên nhận ok thiếu provenance"}
		}
		return e.Receipt, nil
	case ReceiptFailedUnknown:
		return nil, uncertainErr(e)
	case ReceiptFailed:
		return nil, nil // attempt trước đã kết luận: cho phép vẽ lại cùng key
	case ReceiptRunning:
		return s.recover(ctx, *e, req)
	}
	return nil, nil // abandoned/khác: cho phép attempt mới
}

// uncertainErr dựng lỗi của attempt bất định — mặt của nó phải được chủ
// kênh nhìn thấy thay vì một lần vẽ lại âm thầm.
func uncertainErr(e *JournalEntry) *DrawError {
	return &DrawError{
		Code:     ErrCodeUncertain,
		Message:  fmt.Sprintf("Lần vẽ %s (attempt %d) bị gián đoạn chưa rõ đã tạo ảnh hay chưa — đã khoá để không tốn phí hai lần", e.Provider, e.Attempt),
		Recovery: "Kiểm tra ảnh đích; nếu chưa có, yêu cầu vẽ lại tường minh (Abandon rồi Draw)",
	}
}

// providerByName tìm provider theo tên đã đăng ký.
func (s *Service) providerByName(name string) Provider {
	for _, p := range s.opts.Providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// recover áp luật phục hồi của bản cũ cho attempt running: provider (nếu có
// scratch) đối chiếu qua Reconcile; ngược lại kiểm tra tệp output. Không cứu
// được → failed_unknown + khoá key.
func (s *Service) recover(ctx context.Context, e JournalEntry, req Request) (*Receipt, error) {
	p := s.providerByName(e.Provider)
	adopted := false
	var receipt *Receipt
	var recErr error
	if p != nil && len(e.Scratch) > 0 {
		rc, ok, err := p.Reconcile(ctx, e.Scratch, req)
		switch {
		case err == nil && ok:
			adopted = true
			receipt = &rc
		case err == nil:
			// Scratch đọc được nhưng phiên không còn gì để cứu.
		case errors.Is(err, ErrScratchInvalid):
			// Scratch hỏng/nghèo: rơi về kiểm tra output chung.
		default:
			var de *DrawError
			if errors.As(err, &de) && de.Code == ErrCodeConflict {
				return nil, err // phiên vẫn sống: chưa kết luận, để nguyên running
			}
			recErr = err
		}
	}
	if !adopted && receipt == nil && recErr == nil && e.Output != "" {
		// Luật chung: output đã lưu hợp lệ thì attempt đã trả xong — cứu.
		if rc, ok := adoptFinishedOutput(e); ok {
			adopted = true
			receipt = &rc
		}
	}
	now := s.now()
	if adopted && receipt != nil {
		if err := s.opts.Journal.Finish(ctx, e.Key, e.Attempt, ReceiptOK, true, receipt, "", "", now); err != nil {
			if errors.Is(err, ErrAttemptConcluded) {
				return nil, &DrawError{Code: ErrCodeConflict, Message: "Attempt đã được chủ khác kết luận"}
			}
			return nil, &StoreError{Err: err}
		}
		return receipt, nil
	}
	// Không chứng minh được: khoá key, bề mặt ra ngoài.
	msg := "không đối chiếu được output của attempt bị ngắt"
	if recErr != nil {
		msg = recErr.Error()
	}
	if err := s.opts.Journal.Finish(ctx, e.Key, e.Attempt, ReceiptFailedUnknown, true, nil, ErrCodeUncertain, msg, now); err != nil {
		if errors.Is(err, ErrAttemptConcluded) {
			return nil, &DrawError{Code: ErrCodeConflict, Message: "Attempt đã được chủ khác kết luận"}
		}
		return nil, &StoreError{Err: err}
	}
	e.State = ReceiptFailedUnknown
	if recErr != nil {
		return nil, recErr
	}
	return nil, uncertainErr(&e)
}

// adoptFinishedOutput cứu output của attempt bị ngắt khi file đích đã là PNG
// hợp lệ: sha tính lại, provenance đánh dấu là output-adopted.
func adoptFinishedOutput(e JournalEntry) (Receipt, bool) {
	if err := ValidatePngFile(e.Output); err != nil {
		return Receipt{}, false
	}
	sum, err := sha256File(e.Output)
	if err != nil {
		return Receipt{}, false
	}
	return Receipt{
		Provider:    e.Provider,
		Model:       e.Model,
		PngSha256:   sum,
		Attribution: "output-adopted",
	}, true
}

// drawTransient chạy một chuỗi retry transport của lần vẽ (mỗi lần gọi là một
// attempt trả phí riêng trên journal). Lỗi biên nhận không bao giờ retry.
func (s *Service) drawTransient(ctx context.Context, req Request) (Receipt, error) {
	var lastErr error
	for retry := 0; ; retry++ {
		receipt, err := s.drawOnce(ctx, req)
		if err == nil {
			return receipt, nil
		}
		if _, ok := asStoreError(err); ok {
			return Receipt{}, err
		}
		lastErr = err
		var pfe *tools.ProcessFailedError
		transient := errors.As(err, &pfe) && transientRe.MatchString(pfe.Stderr)
		if !transient || retry >= s.retries {
			return Receipt{}, err
		}
		delay := 20 * time.Second
		if retry == 0 {
			delay = 5 * time.Second
		}
		if s.sleep(ctx, delay) != nil {
			return Receipt{}, lastErr
		}
	}
}

// noteQuota ghi nhớ provider vừa báo hết hạn mức để bỏ qua nó tới mốc reset
// (in-process như bản cũ: restart thì nhận lại từ lỗi tiếp theo).
func (s *Service) noteQuota(provider string, err error) {
	de, ok := drawErrorOf(err)
	if !ok || de.Code != ErrCodeQuota {
		return
	}
	until := s.now().Add(quotaSkipFallback)
	if de.RetryAt != nil && de.RetryAt.After(s.now()) {
		until = *de.RetryAt
	}
	s.mu.Lock()
	s.skipUntil[provider] = until
	s.skipErr[provider] = err
	s.mu.Unlock()
}

// QuotaReset trả mốc reset THẬT mà provider đã báo (resets_at / retry_at) nếu
// provider đang bị bỏ qua vì hạn mức. Nguồn: noteQuota (lỗi quota có RetryAt
// từ provider). Không có → false; UI rơi về mốc nửa đêm của trần ngày.
func (s *Service) QuotaReset(provider string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.skipUntil[provider]
	if !ok || !until.After(s.now()) {
		return time.Time{}, false
	}
	return until, true
}

// noteCap ghi nhớ provider vừa chạm trần ngày của app.
func (s *Service) noteCap(provider string, err error) {
	var ce *CapError
	if !errors.As(err, &ce) {
		return
	}
	s.mu.Lock()
	s.skipUntil[provider] = ce.RetryAt
	s.skipErr[provider] = err
	s.mu.Unlock()
}

// skipped trả lỗi chờ (quota/cap) đã ghi nhớ của provider nếu đang bị bỏ qua.
func (s *Service) skipped(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.skipUntil[provider].After(s.now()) {
		return s.skipErr[provider]
	}
	return nil
}

// markSkip đưa provider vào danh sách bỏ qua theo lỗi vừa gặp.
func (s *Service) markSkip(provider string, err error) {
	s.noteQuota(provider, err)
	s.noteCap(provider, err)
}

// usable: provider không bị bỏ qua (quota/cap) và Ready.
func (s *Service) usable(ctx context.Context, p Provider) bool {
	if s.skipped(p.Name()) != nil {
		return false
	}
	return p.Ready(ctx) == nil
}

// drawWith thực hiện MỘT attempt của provider: Begin (ghi trước, kèm kiểm
// trần nguyên tử) → Draw (điểm tiêu tiền) → Finish (ghi sau). outputPreexisting
// cho biết tệp đích đã có trước lượt này (không được nhận vơ là ảnh của ta).
func (s *Service) drawWith(ctx context.Context, p Provider, req Request, outputPreexisting bool) (Receipt, error) {
	now := s.now()
	paidCap := -1
	if p.Paid() {
		paidCap = s.limit
	}
	entry, err := s.opts.Journal.Begin(ctx, Admission{
		Entry: JournalEntry{
			Key: req.Key, Provider: p.Name(), Model: p.Model(), Output: req.Output, StartedAt: now,
		},
		PaidCap:  paidCap,
		DayStart: localDayStart(now),
	})
	if err != nil {
		var cre *CapReachedError
		if errors.As(err, &cre) {
			return Receipt{}, capError(cre.Provider, cre.Used, cre.Limit, now)
		}
		if errors.Is(err, ErrAttemptConcluded) {
			return Receipt{}, &DrawError{Code: ErrCodeConflict, Message: "Attempt khác đang chạy cho cùng khoá vẽ"}
		}
		return Receipt{}, &StoreError{Err: err}
	}
	receipt, drawErr := p.Draw(ctx, req, journalHandle{journal: s.opts.Journal, key: entry.Key, attempt: entry.Attempt})
	if drawErr == nil {
		if err := s.opts.Journal.Finish(ctx, entry.Key, entry.Attempt, ReceiptOK, true, &receipt, "", "", s.now()); err != nil {
			if errors.Is(err, ErrAttemptConcluded) {
				return Receipt{}, &DrawError{Code: ErrCodeConflict, Message: "Attempt đã được chủ khác kết luận"}
			}
			// Ảnh đã lưu trên đĩa; attempt còn running để lần Draw sau
			// Reconcile cứu. Không bao giờ xoá ảnh, không bao giờ trả phí lại.
			return Receipt{}, &StoreError{Err: err}
		}
		return receipt, nil
	}
	// Provider báo lỗi nhưng ảnh hợp lệ đã nằm trên đĩa: lượt này đã trả phí
	// xong — nhận ảnh thay vì rơi vào dự phòng (không tiêu hai lần).
	if !outputPreexisting {
		if rc, ok := adoptFinishedOutput(JournalEntry{
			Key: entry.Key, Attempt: entry.Attempt, Provider: p.Name(), Model: p.Model(), Output: req.Output,
		}); ok {
			if ferr := s.opts.Journal.Finish(ctx, entry.Key, entry.Attempt, ReceiptOK, true, &rc, "", "", s.now()); ferr != nil {
				if errors.Is(ferr, ErrAttemptConcluded) {
					return Receipt{}, &DrawError{Code: ErrCodeConflict, Message: "Attempt đã được chủ khác kết luận"}
				}
				return Receipt{}, &StoreError{Err: ferr}
			}
			return rc, nil
		}
	}
	// Lỗi chắc chắn chưa tiêu phí thì attempt không đếm vào trần ngày.
	charged := !IsUnpaid(drawErr)
	code, msg := errCodeText(drawErr)
	if ferr := s.opts.Journal.Finish(ctx, entry.Key, entry.Attempt, ReceiptFailed, charged, nil, code, msg, s.now()); ferr != nil {
		if errors.Is(ferr, ErrAttemptConcluded) {
			return Receipt{}, drawErr
		}
		return Receipt{}, &StoreError{Err: ferr}
	}
	return Receipt{}, drawErr
}

// errCodeText trích code + message đã che bí mật từ lỗi của attempt. Bí mật
// được che lần cuối ngay trước khi vào biên nhận (defense in depth).
func errCodeText(err error) (string, string) {
	if de, ok := drawErrorOf(err); ok {
		return de.Code, redactLogLine(head(de.Message, 1000))
	}
	return ErrCodeInternal, redactLogLine(head(err.Error(), 1000))
}

// drawOnce là một lần rẽ đầy đủ theo drawImage của bản cũ: primary trước,
// fallback khi lỗi provider không phải huỷ và chưa có ảnh hợp lệ trên đĩa.
// Lỗi biên nhận KHÔNG rơi vào fallback (không tiêu thêm tiền).
func (s *Service) drawOnce(ctx context.Context, req Request) (Receipt, error) {
	primary, rest := s.providers()
	fallback := s.fallbackOn()

	// Tệp đã có TRƯỚC lượt vẽ này là ảnh phục hồi hoặc của chủ khác: không bao
	// giờ xoá để nhường chỗ cho dự phòng.
	preexisting := false
	if _, statErr := os.Stat(req.Output); statErr == nil {
		preexisting = true
	}

	// Primary đang bị bỏ qua (quota/cap) → đi thẳng tới fallback nếu có.
	if skipErr := s.skipped(primary.Name()); skipErr != nil {
		if fallback {
			for _, p := range rest {
				if s.usable(ctx, p) {
					return s.drawWith(ctx, p, req, preexisting)
				}
			}
		}
		return Receipt{}, skipErr
	}

	receipt, perr := s.drawWith(ctx, primary, req, preexisting)
	if perr == nil {
		return receipt, nil
	}
	// Lỗi biên nhận: không phải provider, không được tiêu thêm đồng nào.
	if _, ok := asStoreError(perr); ok {
		return Receipt{}, perr
	}
	s.markSkip(primary.Name(), perr)
	if !fallback || preexisting || ctx.Err() != nil || isCancelled(perr) {
		return Receipt{}, perr
	}
	// Primary lỗi có thể để lại tệp dở; dự phòng cần output trống để ghi độc
	// quyền. Chỉ xoá tệp do chính lượt này tạo ra.
	if err := os.Remove(req.Output); err != nil && !os.IsNotExist(err) {
		return Receipt{}, perr
	}
	var secondary Provider
	for _, p := range rest {
		if s.usable(ctx, p) {
			secondary = p
			break
		}
	}
	if secondary == nil {
		return Receipt{}, perr
	}
	receipt, serr := s.drawWith(ctx, secondary, req, false)
	if serr == nil {
		return receipt, nil
	}
	if _, ok := asStoreError(serr); ok {
		return Receipt{}, serr
	}
	s.markSkip(secondary.Name(), serr)
	if isCancelled(serr) {
		return Receipt{}, serr
	}
	if de, ok := drawErrorOf(perr); ok {
		reason := serr.Error()
		if sde, ok := drawErrorOf(serr); ok {
			reason = sde.Message
		}
		combined := &DrawError{
			Code:          de.Code,
			Message:       fmt.Sprintf("%s (%s cũng lỗi: %s)", de.Message, ProviderLabel(secondary.Name()), head(redactLogLine(reason), 200)),
			Recovery:      de.Recovery,
			RetryAt:       de.RetryAt,
			RetryAtSource: de.RetryAtSource,
		}
		if IsProviderBusy(perr) {
			return Receipt{}, &ProviderBusyError{DrawError: *combined}
		}
		if IsImageRefused(perr) {
			return Receipt{}, &ImageRefusedError{DrawError: *combined}
		}
		return Receipt{}, combined
	}
	return Receipt{}, perr
}

// isCancelled: lỗi huỷ theo cả hai nguồn (ctx của app và ErrCancelled của runner).
func isCancelled(err error) bool {
	if errors.Is(err, tools.ErrCancelled) || errors.Is(err, context.Canceled) {
		return true
	}
	de, ok := drawErrorOf(err)
	return ok && de.Code == ErrCodeCancelled
}

// ResetQuotaSkips quên các skip in-process (đường kiểm thử của bản cũ).
func (s *Service) ResetQuotaSkips() {
	s.mu.Lock()
	s.skipUntil = map[string]time.Time{}
	s.skipErr = map[string]error{}
	s.mu.Unlock()
}

// Abandon mở khoá một draw key đã failed/failed_unknown để vẽ lại tường
// minh (bản ghi giữ làm bằng chứng).
func (s *Service) Abandon(ctx context.Context, key string) error {
	if err := s.opts.Journal.Abandon(ctx, key); err != nil {
		return &StoreError{Err: err}
	}
	return nil
}

// Charged đếm attempt trả phí của provider từ mốc since — cùng số với trần
// ngày dùng.
func (s *Service) Charged(ctx context.Context, provider string, since time.Time) (int64, error) {
	n, err := s.opts.Journal.Charged(ctx, provider, since)
	if err != nil {
		return 0, &StoreError{Err: err}
	}
	return n, nil
}
