package imagegen_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
)

// testPNG là PNG 1×1 hợp lệ (fixture của các suite gốc).
var testPNG = mustB64("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a5XcAAAAASUVORK5CYII=")

func mustB64(v string) []byte {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- journal in-memory cho test không-DB ----

type memoryJournal struct {
	mu   sync.Mutex
	rows map[string][]imagegen.JournalEntry

	// Lỗi tiêm để kiểm tra ranh giới trước/sau lời gọi trả phí. Khi được đặt,
	// thao tác tương ứng thất bại TRƯỚC khi chạm rows (mô phỏng store chết).
	lockErr   error
	beginErr  error
	finishErr error
}

func newMemoryJournal() *memoryJournal {
	return &memoryJournal{rows: map[string][]imagegen.JournalEntry{}}
}

func (j *memoryJournal) LockKey(ctx context.Context, key string) (func(), error) {
	if j.lockErr != nil {
		return nil, j.lockErr
	}
	return func() {}, nil
}

func (j *memoryJournal) Begin(ctx context.Context, a imagegen.Admission) (imagegen.JournalEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.beginErr != nil {
		return imagegen.JournalEntry{}, j.beginErr
	}
	e := a.Entry
	rows := j.rows[e.Key]
	if a.PaidCap >= 0 {
		var used int64
		for _, keyRows := range j.rows {
			for _, r := range keyRows {
				if r.Provider == e.Provider && r.Charged && !r.StartedAt.Before(a.DayStart) {
					used++
				}
			}
		}
		if used >= int64(a.PaidCap) {
			return imagegen.JournalEntry{}, imagegen.ErrCapReached(e.Provider, used, a.PaidCap)
		}
	}
	for _, r := range rows {
		if r.State == imagegen.ReceiptRunning {
			return imagegen.JournalEntry{}, imagegen.ErrAttemptConcluded
		}
	}
	e.Attempt = len(rows) + 1
	if e.StartedAt.IsZero() {
		e.StartedAt = time.Now()
	}
	e.State = imagegen.ReceiptRunning
	e.Charged = true
	j.rows[e.Key] = append(rows, e)
	return e, nil
}

func (j *memoryJournal) Latest(ctx context.Context, key string) (*imagegen.JournalEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	rows := j.rows[key]
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].State != imagegen.ReceiptAbandoned {
			e := rows[i]
			return &e, nil
		}
	}
	return nil, nil
}

func (j *memoryJournal) Finish(ctx context.Context, key string, attempt int, state string, charged bool, receipt *imagegen.Receipt, errCode, errText string, finishedAt time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.finishErr != nil {
		return j.finishErr
	}
	rows := j.rows[key]
	for i := range rows {
		if rows[i].Attempt == attempt {
			if rows[i].State != imagegen.ReceiptRunning {
				return imagegen.ErrAttemptConcluded
			}
			rows[i].State = state
			rows[i].Charged = charged
			rows[i].Receipt = receipt
			rows[i].ErrorCode = errCode
			rows[i].ErrorText = errText
			t := finishedAt
			rows[i].FinishedAt = &t
			j.rows[key] = rows
			return nil
		}
	}
	return fmt.Errorf("attempt %d của %s không tồn tại", attempt, key)
}

func (j *memoryJournal) UpdateScratch(ctx context.Context, key string, attempt int, scratch json.RawMessage) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	rows := j.rows[key]
	for i := range rows {
		if rows[i].Attempt == attempt && rows[i].State == imagegen.ReceiptRunning {
			rows[i].Scratch = scratch
			j.rows[key] = rows
			return nil
		}
	}
	return nil
}

func (j *memoryJournal) Charged(ctx context.Context, provider string, since time.Time) (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	var n int64
	for _, rows := range j.rows {
		for _, r := range rows {
			if r.Provider == provider && r.Charged && !r.StartedAt.Before(since) {
				n++
			}
		}
	}
	return n, nil
}

func (j *memoryJournal) Abandon(ctx context.Context, key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	rows := j.rows[key]
	for i := range rows {
		if rows[i].State == imagegen.ReceiptFailed || rows[i].State == imagegen.ReceiptFailedUnknown {
			rows[i].State = imagegen.ReceiptAbandoned
		}
	}
	j.rows[key] = rows
	return nil
}

func (j *memoryJournal) attempts(key string) []imagegen.JournalEntry {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]imagegen.JournalEntry{}, j.rows[key]...)
}

// seedRunning tạo sẵn một attempt 'running' (mô phỏng process chết giữa chừng)
// mà không qua trần ngày.
func seedRunning(t *testing.T, j *memoryJournal, e imagegen.JournalEntry) {
	t.Helper()
	if _, err := j.Begin(context.Background(), imagegen.Admission{Entry: e, PaidCap: -1}); err != nil {
		t.Fatal(err)
	}
}

// ---- provider giả: scripted như fakeDrawers của bản TS ----

type fakeDrawCall struct {
	Provider string
	Req      imagegen.Request
}

type fakeProvider struct {
	name      string
	paid      bool
	readyErr  error
	script    []any // imagegen.Receipt hoặc error
	onDraw    func(req imagegen.Request) error
	calls     *[]fakeDrawCall
	reconcile *fakeReconcile
}

type fakeReconcile struct {
	receipt imagegen.Receipt
	ok      bool
	err     error
}

func (p *fakeProvider) Name() string { return p.name }
func (p *fakeProvider) Model() string {
	if p.name == "codex" {
		return imagegen.CodexImageModel
	}
	return imagegen.GrokImageModel
}
func (p *fakeProvider) Paid() bool { return p.paid }
func (p *fakeProvider) Ready(ctx context.Context) error {
	return p.readyErr
}
func (p *fakeProvider) Draw(ctx context.Context, req imagegen.Request, journal imagegen.JournalHandle) (imagegen.Receipt, error) {
	*p.calls = append(*p.calls, fakeDrawCall{Provider: p.name, Req: req})
	if p.onDraw != nil {
		if err := p.onDraw(req); err != nil {
			return imagegen.Receipt{}, err
		}
	}
	if len(p.script) == 0 {
		return imagegen.Receipt{}, fmt.Errorf("unexpected %s draw", p.name)
	}
	next := p.script[0]
	p.script = p.script[1:]
	switch v := next.(type) {
	case error:
		return imagegen.Receipt{}, v
	case imagegen.Receipt:
		return v, nil
	}
	return imagegen.Receipt{}, fmt.Errorf("script entry không hợp lệ: %T", next)
}
func (p *fakeProvider) Reconcile(ctx context.Context, scratch json.RawMessage, req imagegen.Request) (imagegen.Receipt, bool, error) {
	if p.reconcile != nil {
		return p.reconcile.receipt, p.reconcile.ok, p.reconcile.err
	}
	return imagegen.Receipt{}, false, nil
}

// drawn tương đương `drawn(provider)` của bản TS.
func drawn(provider string) imagegen.Receipt {
	return imagegen.Receipt{Provider: "grok", Model: "grok-imagine-image", PngSha256: provider}
}

// quotaErr tương đương `quota()` của bản TS.
func quotaErr() *imagegen.DrawError {
	return &imagegen.DrawError{
		Code:     imagegen.ErrCodeQuota,
		Message:  "Hạn mức tạo ảnh đã hết; hàng đợi đã tạm dừng",
		Recovery: "chờ",
	}
}

// testEnv gói service + journal + calls cho các ca fallback/cap.
type testEnv struct {
	svc     *imagegen.Service
	journal *memoryJournal
	calls   *[]fakeDrawCall
	fakes   []*fakeProvider
	dir     string
	counter int
}

// newEnv dựng service với hai provider scripted (codex primary, grok fallback).
func newEnv(t *testing.T, codexScript, grokScript []any, opts func(*imagegen.Options)) *testEnv {
	t.Helper()
	dir := t.TempDir()
	calls := &[]fakeDrawCall{}
	fakes := []*fakeProvider{
		{name: "codex", paid: true, script: codexScript, calls: calls},
		{name: "grok", paid: true, script: grokScript, calls: calls},
	}
	journal := newMemoryJournal()
	o := imagegen.Options{
		Providers: []imagegen.Provider{fakes[0], fakes[1]},
		Journal:   journal,
		Sleep:     func(ctx context.Context, d time.Duration) error { return nil },
	}
	if opts != nil {
		opts(&o)
	}
	return &testEnv{svc: imagegen.NewService(o), journal: journal, calls: calls, fakes: fakes, dir: dir}
}

// request dựng request với key duy nhất mỗi lần gọi.
func (e *testEnv) request() imagegen.Request {
	e.counter++
	return imagegen.Request{
		Key:    fmt.Sprintf("gen-%d", e.counter),
		Prompt: "a lighthouse",
		Output: fmt.Sprintf("%s/image-%d.png", e.dir, e.counter),
		Width:  1280, Height: 720,
	}
}

func providerCalls(calls []fakeDrawCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.Provider
	}
	return out
}
