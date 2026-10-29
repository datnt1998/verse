// Ca test chuyển từ src/server/clone/image-fallback.test.ts của yt-factory:
// thứ tự dự phòng, skip quota, xoá output dở, giữ class lỗi của primary —
// cùng các ca thêm của trần ảnh trả phí ngày (D7) và khoá idempotent theo
// biên nhận.
package imagegen_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func drawErrorCode(t *testing.T, err error) string {
	t.Helper()
	var de *imagegen.DrawError
	if !errors.As(err, &de) {
		t.Fatalf("lỗi không phải DrawError: %v", err)
	}
	return de.Code
}

// "a spent ChatGPT quota redraws with Grok, and later images skip ChatGPT
// until its reset"
func TestQuotaFallsBackAndSkipsPrimaryUntilReset(t *testing.T) {
	env := newEnv(t, []any{quotaErr()}, []any{drawn("grok"), drawn("grok")}, nil)
	r1, err := env.svc.Draw(context.Background(), env.request())
	if err != nil {
		t.Fatalf("draw 1: %v", err)
	}
	if r1.PngSha256 != "grok" {
		t.Fatalf("draw 1 provider: %+v", r1)
	}
	r2, err := env.svc.Draw(context.Background(), env.request())
	if err != nil {
		t.Fatalf("draw 2: %v", err)
	}
	if r2.PngSha256 != "grok" {
		t.Fatalf("draw 2 provider: %+v", r2)
	}
	if got, want := providerCalls(*env.calls), []string{"codex", "grok", "grok"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

// "any other ChatGPT failure also falls back, and the partial output is
// cleared first"
func TestOtherFailureFallsBackAndClearsPartialOutput(t *testing.T) {
	env := newEnv(t,
		[]any{&imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "Codex không trả về ảnh hợp lệ"}},
		[]any{drawn("grok")}, nil)
	var grokChecked bool
	env.fakes[0].onDraw = func(req imagegen.Request) error {
		return os.WriteFile(req.Output, []byte("partial"), 0o600)
	}
	env.fakes[1].onDraw = func(req imagegen.Request) error {
		grokChecked = true
		if _, err := os.Stat(req.Output); !os.IsNotExist(err) {
			return fmt.Errorf("fallback phải bắt đầu từ output trống")
		}
		return nil
	}
	r, err := env.svc.Draw(context.Background(), env.request())
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if r.PngSha256 != "grok" || !grokChecked {
		t.Fatalf("receipt=%+v grokChecked=%v", r, grokChecked)
	}
}

// "fallback off, Grok logged out, or a cancel: the ChatGPT error stands and
// Grok is never called"
func TestNoFallbackCasesKeepPrimaryError(t *testing.T) {
	ctx := context.Background()

	off := false
	env := newEnv(t, []any{quotaErr()}, nil, func(o *imagegen.Options) { o.Fallback = &off })
	_, err := env.svc.Draw(ctx, env.request())
	if drawErrorCode(t, err) != imagegen.ErrCodeQuota {
		t.Fatalf("code = %v", err)
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("calls = %v", got)
	}

	env = newEnv(t, []any{quotaErr()}, nil, func(o *imagegen.Options) {
		o.Providers[1].(*fakeProvider).readyErr = errors.New("logged_out")
	})
	_, err = env.svc.Draw(ctx, env.request())
	if drawErrorCode(t, err) != imagegen.ErrCodeQuota {
		t.Fatalf("code = %v", err)
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("calls = %v", got)
	}

	env = newEnv(t, []any{&imagegen.DrawError{Code: imagegen.ErrCodeCancelled, Message: "Đã huỷ tạo ảnh"}}, nil, nil)
	_, err = env.svc.Draw(ctx, env.request())
	if drawErrorCode(t, err) != imagegen.ErrCodeCancelled {
		t.Fatalf("code = %v", err)
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("calls = %v", got)
	}
}

// "an output already on disk is never deleted to make room for a fallback"
func TestPreexistingOutputIsNeverDeletedForFallback(t *testing.T) {
	env := newEnv(t, []any{&imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "EEXIST"}}, nil, nil)
	req := env.request()
	if err := os.WriteFile(req.Output, []byte("recovered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.Draw(context.Background(), req); err == nil {
		t.Fatal("mong lỗi")
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("calls = %v", got)
	}
	data, _ := os.ReadFile(req.Output)
	if string(data) != "recovered" {
		t.Fatalf("output bị ghi đè: %q", data)
	}
}

// "both failing keeps the primary's code and retry time, with the fallback
// reason appended"
func TestBothFailingKeepsPrimaryCodeAndRetry(t *testing.T) {
	retryAt := time.Now().Add(time.Hour)
	primary := &imagegen.DrawError{
		Code: imagegen.ErrCodeQuota, Message: "Hạn mức tạo ảnh đã hết",
		RetryAt: &retryAt, RetryAtSource: "provider",
	}
	env := newEnv(t, []any{primary}, []any{
		&imagegen.DrawError{Code: imagegen.ErrCodeToolMissing, Message: "Phiên đăng nhập Grok đã hết hạn"},
	}, nil)
	_, err := env.svc.Draw(context.Background(), env.request())
	var de *imagegen.DrawError
	if !errors.As(err, &de) {
		t.Fatalf("lỗi không phải DrawError: %v", err)
	}
	if de.Code != imagegen.ErrCodeQuota || de.RetryAt == nil || !de.RetryAt.Equal(retryAt) || de.RetryAtSource != "provider" {
		t.Fatalf("de = %+v", de)
	}
	if !strings.Contains(de.Message, "Grok cũng lỗi: Phiên đăng nhập Grok đã hết hạn") {
		t.Fatalf("message = %q", de.Message)
	}
}

// "busy and refused primaries keep their class when the fallback fails too,
// so the queue still retries or redraws"
func TestBusyAndRefusedPrimariesKeepTheirClass(t *testing.T) {
	// Busy được retry tại chỗ (bản cũ: 4 lần) — script đủ cho mọi vòng: mỗi
	// vòng codex busy → grok lỗi thường, lỗi cuối giữ class busy.
	env := newEnv(t, nil, nil, nil)
	for range 5 {
		env.fakes[0].script = append(env.fakes[0].script, imagegen.NewProviderBusyError(time.Now()))
		env.fakes[1].script = append(env.fakes[1].script, &imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "boom"})
	}
	_, err := env.svc.Draw(context.Background(), env.request())
	if !imagegen.IsProviderBusy(err) {
		t.Fatalf("mong ProviderBusyError, được %v", err)
	}

	env = newEnv(t,
		[]any{imagegen.NewImageRefusedError("content policy")},
		[]any{&imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "boom"}}, nil)
	_, err = env.svc.Draw(context.Background(), env.request())
	if !imagegen.IsImageRefused(err) {
		t.Fatalf("mong ImageRefusedError, được %v", err)
	}
}

// "with Grok as the main source, a Grok failure falls back to a ready ChatGPT"
func TestGrokPrimaryFallsBackToCodex(t *testing.T) {
	env := newEnv(t, []any{drawn("codex")}, []any{
		&imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "Grok không trả về ảnh hợp lệ (HTTP 500)"},
	}, func(o *imagegen.Options) {
		o.Providers = []imagegen.Provider{o.Providers[1], o.Providers[0]}
	})
	r, err := env.svc.Draw(context.Background(), env.request())
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if r.PngSha256 != "codex" {
		t.Fatalf("receipt = %+v", r)
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"grok", "codex"}) {
		t.Fatalf("calls = %v", got)
	}
}

// "fallback carries the exact reference and prompt to either provider"
func TestFallbackCarriesExactRequest(t *testing.T) {
	for _, primary := range []string{"codex", "grok"} {
		env := newEnv(t, nil, nil, func(o *imagegen.Options) {
			if primary == "grok" {
				o.Providers = []imagegen.Provider{o.Providers[1], o.Providers[0]}
			}
		})
		// Script theo provider, không theo vị trí: primary refused, còn lại ok.
		env.fakes[0].script = []any{imagegen.NewImageRefusedError("content policy")}
		env.fakes[1].script = []any{imagegen.NewImageRefusedError("content policy")}
		env.fakes[0].reconcile = nil
		if primary == "codex" {
			env.fakes[1].script = []any{drawn("second")}
		} else {
			env.fakes[0].script = []any{drawn("second")}
		}
		req := env.request()
		req.Reference = "/tmp/validated-reference.png"
		r, err := env.svc.Draw(context.Background(), req)
		if err != nil {
			t.Fatalf("draw: %v", err)
		}
		if r.PngSha256 != "second" {
			t.Fatalf("receipt = %+v", r)
		}
		seen := *env.calls
		if len(seen) != 2 {
			t.Fatalf("calls = %v", seen)
		}
		if seen[0].Provider != primary {
			t.Fatalf("primary = %s, calls = %v", primary, seen)
		}
		for _, c := range seen {
			if c.Req.Reference != req.Reference || c.Req.Prompt != req.Prompt || c.Req.Output != req.Output {
				t.Fatalf("request không khớp: %+v", c.Req)
			}
		}
	}
}

// ---- trần ảnh trả phí ngày (D7) ----

func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// Nil → mặc định D7 = 50; âm (cấu hình sai) cũng về 50; 0 giữ nghĩa "không
// tự tiêu".
func TestDailyLimitDefault(t *testing.T) {
	fp := &fakeProvider{name: "codex", paid: true, calls: &[]fakeDrawCall{}}
	for _, tc := range []struct {
		name string
		opt  *int
		want int
	}{
		{"không đặt", nil, imagegen.DefaultDailyPaidImageLimit},
		{"âm", new(-1), imagegen.DefaultDailyPaidImageLimit},
		{"không tự tiêu", new(0), 0},
		{"tường minh", new(7), 7},
	} {
		svc := imagegen.NewService(imagegen.Options{
			Providers:           []imagegen.Provider{fp},
			Journal:             newMemoryJournal(),
			DailyPaidImageLimit: tc.opt,
		})
		if got := svc.DailyLimit(); got != tc.want {
			t.Fatalf("%s: DailyLimit = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// Biên trần: 49/50 lần đầu qua, lần 51 bị chặn kèm retryAt nửa đêm hôm sau —
// provider không được gọi thêm.
func TestDailyCapBoundaries(t *testing.T) {
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	fp := &fakeProvider{name: "codex", paid: true, calls: calls}
	svc := imagegen.NewService(imagegen.Options{
		Providers:           []imagegen.Provider{fp},
		Journal:             newMemoryJournal(),
		DailyPaidImageLimit: new(50),
		Sleep:               func(ctx context.Context, d time.Duration) error { return nil },
	})
	env := &testEnv{svc: svc, calls: calls, dir: dir}
	for i := range 50 {
		fp.script = []any{drawn("codex")}
		if _, err := svc.Draw(context.Background(), env.request()); err != nil {
			t.Fatalf("draw %d: %v", i+1, err)
		}
	}
	if len(*calls) != 50 {
		t.Fatalf("calls = %d", len(*calls))
	}
	fp.script = []any{drawn("codex")}
	_, err := svc.Draw(context.Background(), env.request())
	var ce *imagegen.CapError
	if !errors.As(err, &ce) {
		t.Fatalf("mong CapError, được %v", err)
	}
	if ce.Limit != 50 || ce.Used != 50 {
		t.Fatalf("cap = %+v", ce)
	}
	if want := dayStart(time.Now()).Add(24 * time.Hour); !ce.RetryAt.Equal(want) {
		t.Fatalf("retryAt = %v, want %v", ce.RetryAt, want)
	}
	if len(*calls) != 50 {
		t.Fatalf("provider bị gọi quá trần: %d", len(*calls))
	}
}

// Trần 0 = máy không bao giờ tự tiêu: lần vẽ đầu đã bị chặn.
func TestDailyCapZeroNeverSpends(t *testing.T) {
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	svc := imagegen.NewService(imagegen.Options{
		Providers: []imagegen.Provider{
			&fakeProvider{name: "codex", paid: true, calls: calls, script: []any{drawn("codex")}},
		},
		Journal:             newMemoryJournal(),
		DailyPaidImageLimit: new(0),
	})
	req := imagegen.Request{Key: "k1", Prompt: "x", Output: filepath.Join(dir, "o.png")}
	_, err := svc.Draw(context.Background(), req)
	var ce *imagegen.CapError
	if !errors.As(err, &ce) {
		t.Fatalf("mong CapError, được %v", err)
	}
	if ce.Limit != 0 || len(*calls) != 0 {
		t.Fatalf("cap=%+v calls=%d", ce, len(*calls))
	}
}

// Attempt lỗi chắc-chưa-tiêu-phí (busy/refused/quota) không đếm vào trần ngày.
func TestCapOnlyCountsChargedAttempts(t *testing.T) {
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	fp := &fakeProvider{name: "codex", paid: true, calls: calls}
	j := newMemoryJournal()
	svc := imagegen.NewService(imagegen.Options{
		Providers:           []imagegen.Provider{fp},
		Journal:             j,
		DailyPaidImageLimit: new(1),
		Sleep:               func(ctx context.Context, d time.Duration) error { return nil },
	})
	// Busy được retry tại chỗ (4 lần) rồi bung lỗi — không đốt trần.
	fp.script = []any{
		imagegen.NewProviderBusyError(time.Now()),
		imagegen.NewProviderBusyError(time.Now()),
		imagegen.NewProviderBusyError(time.Now()),
		imagegen.NewProviderBusyError(time.Now()),
		imagegen.NewProviderBusyError(time.Now()),
	}
	req := imagegen.Request{Key: "k1", Prompt: "x", Output: filepath.Join(dir, "a.png")}
	if _, err := svc.Draw(context.Background(), req); !imagegen.IsProviderBusy(err) {
		t.Fatalf("mong busy, được %v", err)
	}
	// Lớp busy phải giữ code "interrupted" và miễn phí trên biên nhận.
	rows := j.attempts("k1")
	if len(rows) != 5 {
		t.Fatalf("attempts = %d", len(rows))
	}
	for _, r := range rows {
		if r.Charged || r.ErrorCode != "interrupted" {
			t.Fatalf("attempt busy = %+v", r)
		}
	}
	fp.script = []any{imagegen.NewImageRefusedError("content policy")}
	req.Key = "k2"
	req.Output = filepath.Join(dir, "b.png")
	if _, err := svc.Draw(context.Background(), req); !imagegen.IsImageRefused(err) {
		t.Fatalf("mong refused, được %v", err)
	}
	// Trần 1 vẫn cho một lần vẽ trả phí thật.
	fp.script = []any{drawn("codex")}
	req.Key = "k3"
	req.Output = filepath.Join(dir, "c.png")
	if _, err := svc.Draw(context.Background(), req); err != nil {
		t.Fatalf("draw trả phí đầu phải qua: %v", err)
	}
	fp.script = []any{drawn("codex")}
	req.Key = "k4"
	req.Output = filepath.Join(dir, "d.png")
	var ce *imagegen.CapError
	if _, err := svc.Draw(context.Background(), req); !errors.As(err, &ce) {
		t.Fatalf("mong CapError, được %v", err)
	}
}

// Provider không tính phí theo ảnh không chịu trần.
func TestUnpaidProviderIgnoresCap(t *testing.T) {
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	fp := &fakeProvider{name: "codex", paid: false, calls: calls, script: []any{drawn("codex"), drawn("codex"), drawn("codex")}}
	svc := imagegen.NewService(imagegen.Options{
		Providers:           []imagegen.Provider{fp},
		Journal:             newMemoryJournal(),
		DailyPaidImageLimit: new(1),
	})
	for i := range 3 {
		req := imagegen.Request{Key: fmt.Sprint(i), Prompt: "x", Output: filepath.Join(dir, fmt.Sprint(i)+".png")}
		if _, err := svc.Draw(context.Background(), req); err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
	}
}

// Trần của primary không cản fallback miễn phí: ảnh vẫn ra bằng provider
// không tính phí còn lại.
func TestCapFallsBackToOtherProvider(t *testing.T) {
	env := newEnv(t, nil, []any{drawn("grok")}, func(o *imagegen.Options) {
		o.DailyPaidImageLimit = new(0)
		o.Providers[1].(*fakeProvider).paid = false
	})
	r, err := env.svc.Draw(context.Background(), env.request())
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if r.PngSha256 != "grok" {
		t.Fatalf("receipt = %+v", r)
	}
	if got := providerCalls(*env.calls); !reflect.DeepEqual(got, []string{"grok"}) {
		t.Fatalf("calls = %v — primary không được gọi khi chạm trần", got)
	}
}

// ---- khoá idempotent + phục hồi biên nhận ----

// Cùng key đã 'ok' trả receipt cũ, không gọi provider lần nữa.
func TestIdempotentKeyReturnsStoredReceipt(t *testing.T) {
	env := newEnv(t, []any{drawn("codex")}, nil, nil)
	req := env.request()
	r1, err := env.svc.Draw(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := env.svc.Draw(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r1.PngSha256 != r2.PngSha256 || len(*env.calls) != 1 {
		t.Fatalf("r1=%+v r2=%+v calls=%v", r1, r2, *env.calls)
	}
	if n := len(env.journal.attempts(req.Key)); n != 1 {
		t.Fatalf("attempts = %d", n)
	}
}

// Crash sau khi output đã lưu: cứu ảnh từ file đích, không vẽ lại.
func TestCrashRecoveryAdoptsFinishedOutput(t *testing.T) {
	env := newEnv(t, nil, nil, nil)
	dir := t.TempDir()
	out := filepath.Join(dir, "done.png")
	if err := os.WriteFile(out, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	key := "crash-1"
	seedRunning(t, env.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: out,
	})
	r, err := env.svc.Draw(context.Background(), imagegen.Request{Key: key, Prompt: "x", Output: out})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if len(*env.calls) != 0 {
		t.Fatalf("provider bị gọi khi phục hồi: %v", *env.calls)
	}
	if r.PngSha256 == "" || r.Attribution != "output-adopted" {
		t.Fatalf("receipt = %+v", r)
	}
	if state := env.journal.attempts(key)[0].State; state != imagegen.ReceiptOK {
		t.Fatalf("state = %s", state)
	}
}

// Crash chưa rõ: không output → failed_unknown, key bị khoá, không vẽ lại mù;
// chỉ vẽ lại được sau Abandon tường minh.
func TestCrashRecoveryWithoutOutputLocksKey(t *testing.T) {
	env := newEnv(t, []any{drawn("codex")}, nil, nil)
	dir := t.TempDir()
	out := filepath.Join(dir, "lost.png")
	key := "crash-2"
	seedRunning(t, env.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: out,
	})
	req := imagegen.Request{Key: key, Prompt: "x", Output: out}
	_, err := env.svc.Draw(context.Background(), req)
	if drawErrorCode(t, err) != imagegen.ErrCodeUncertain {
		t.Fatalf("mong failed_unknown, được %v", err)
	}
	if len(*env.calls) != 0 {
		t.Fatalf("provider bị gọi khi attempt bất định: %v", *env.calls)
	}
	if state := env.journal.attempts(key)[0].State; state != imagegen.ReceiptFailedUnknown {
		t.Fatalf("state = %s", state)
	}
	_, err = env.svc.Draw(context.Background(), req)
	if drawErrorCode(t, err) != imagegen.ErrCodeUncertain {
		t.Fatalf("lần 2 mong failed_unknown, được %v", err)
	}
	if len(*env.calls) != 0 {
		t.Fatalf("provider bị gọi: %v", *env.calls)
	}
	if err := env.svc.Abandon(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	r, err := env.svc.Draw(context.Background(), req)
	if err != nil {
		t.Fatalf("redraw: %v", err)
	}
	if r.PngSha256 != "codex" || len(*env.calls) != 1 {
		t.Fatalf("r=%+v calls=%v", r, *env.calls)
	}
	if n := len(env.journal.attempts(key)); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
}

// Output khai báo trên biên nhận phải khớp request — lệch là invalid_request.
func TestRecoveryRejectsMismatchedOutput(t *testing.T) {
	env := newEnv(t, nil, nil, nil)
	key := "crash-3"
	seedRunning(t, env.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: "/tmp/other.png",
	})
	_, err := env.svc.Draw(context.Background(), imagegen.Request{
		Key: key, Prompt: "x", Output: filepath.Join(env.dir, "mismatch.png"),
	})
	if drawErrorCode(t, err) != imagegen.ErrCodeInvalidRequest {
		t.Fatalf("mong invalid_request, được %v", err)
	}
}

// Retry chủ động cùng key sau attempt failed: attempt mới kế thừa đúng chi phí.
func TestExplicitRedrawStartsNewAttempt(t *testing.T) {
	off := false
	env := newEnv(t,
		[]any{&imagegen.DrawError{Code: imagegen.ErrCodeInternal, Message: "boom"}, drawn("codex")},
		nil, func(o *imagegen.Options) { o.Fallback = &off })
	req := env.request()
	if _, err := env.svc.Draw(context.Background(), req); err == nil {
		t.Fatal("mong lỗi")
	}
	r, err := env.svc.Draw(context.Background(), req)
	if err != nil {
		t.Fatalf("redraw: %v", err)
	}
	if r.PngSha256 != "codex" {
		t.Fatalf("r = %+v", r)
	}
	rows := env.journal.attempts(req.Key)
	if len(rows) != 2 || rows[0].Attempt != 1 || rows[1].Attempt != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].State != imagegen.ReceiptFailed || rows[1].State != imagegen.ReceiptOK {
		t.Fatalf("states = %s/%s", rows[0].State, rows[1].State)
	}
}

// Lỗi transport chắc chắn tạm thời retry đúng nhịp (bản cũ: 2 lần — 5 s/20 s).
func TestTransientTransportRetries(t *testing.T) {
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	fp := &fakeProvider{name: "codex", paid: true, calls: calls, script: []any{
		&tools.ProcessFailedError{Message: "x", Code: 1, Stderr: "connection reset"},
		&tools.ProcessFailedError{Message: "x", Code: 1, Stderr: "temporarily unavailable"},
		drawn("codex"),
	}}
	svc := imagegen.NewService(imagegen.Options{
		Providers: []imagegen.Provider{fp},
		Journal:   newMemoryJournal(),
		Sleep:     func(ctx context.Context, d time.Duration) error { return nil },
	})
	req := imagegen.Request{Key: "k", Prompt: "x", Output: filepath.Join(dir, "o.png")}
	r, err := svc.Draw(context.Background(), req)
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if r.PngSha256 != "codex" || len(*calls) != 3 {
		t.Fatalf("r=%+v calls=%d", r, len(*calls))
	}
}
