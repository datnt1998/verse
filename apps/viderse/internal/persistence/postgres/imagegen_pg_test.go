// Ca test biên nhận ảnh trả phí trên Postgres thật: attempt kế thừa chi phí,
// scratch chỉ ghi khi running, charged đếm đúng, abandon giữ bằng chứng, và
// phục hồi crash qua imagegen.Service — không bao giờ vẽ lại mù quáng.
//
// Chạy trên schema riêng provision bằng pgtest với cặp role của Viderse rồi
// áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

// crashProvider là provider giả chỉ đếm lần vẽ cho các ca phục hồi.
type crashProvider struct {
	draws int
}

func (p *crashProvider) Name() string  { return "codex" }
func (p *crashProvider) Model() string { return imagegen.CodexImageModel }
func (p *crashProvider) Paid() bool    { return true }
func (p *crashProvider) Ready(ctx context.Context) error {
	return nil
}
func (p *crashProvider) Draw(ctx context.Context, req imagegen.Request, journal imagegen.JournalHandle) (imagegen.Receipt, error) {
	p.draws++
	return imagegen.Receipt{Provider: "codex", Model: imagegen.CodexImageModel, PngSha256: "fresh"}, nil
}
func (p *crashProvider) Reconcile(ctx context.Context, scratch json.RawMessage, req imagegen.Request) (imagegen.Receipt, bool, error) {
	return imagegen.Receipt{}, false, nil
}

// PNG 1×1 hợp lệ.
var testPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x04, 0x00, 0x00, 0x00, 0xB5, 0x1C, 0x0C, 0x02, 0x00, 0x00, 0x00,
	0x0B, 0x49, 0x44, 0x41, 0x54, 0x78, 0xDA, 0x63, 0xFC, 0xFF, 0x1F, 0x00,
	0x03, 0x03, 0x02, 0x00, 0xEF, 0x9A, 0xE5, 0x77, 0x00, 0x00, 0x00, 0x00,
	0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// Begin → attempt tăng dần, Finish ok/failed, charged đếm qua ngày.
func TestReceiptStoreAttemptsAndCharged(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)

	e1, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "k", Provider: "codex", Model: "m", Output: "/tmp/a.png"}, PaidCap: -1})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if e1.Attempt != 1 || e1.State != imagegen.ReceiptRunning || !e1.Charged {
		t.Fatalf("entry = %+v", e1)
	}
	// Attempt running đếm là đã tốn phí (thận trọng).
	if n, err := store.Charged(ctx, "codex", e1.StartedAt.Add(-time.Hour)); err != nil || n != 1 {
		t.Fatalf("charged = %d, %v", n, err)
	}
	rc := &imagegen.Receipt{Provider: "codex", Model: "m", PngSha256: "sha1"}
	if err := store.Finish(ctx, "k", e1.Attempt, imagegen.ReceiptOK, true, rc, "", "", time.Now()); err != nil {
		t.Fatalf("finish: %v", err)
	}
	latest, err := store.Latest(ctx, "k")
	if err != nil || latest == nil {
		t.Fatalf("latest: %v", err)
	}
	if latest.State != imagegen.ReceiptOK || latest.Receipt == nil || latest.Receipt.PngSha256 != "sha1" || latest.FinishedAt == nil {
		t.Fatalf("latest = %+v", latest)
	}
	// Retry cùng key → attempt 2; attempt failed-free không đếm phí.
	e2, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "k", Provider: "codex", Model: "m", Output: "/tmp/a.png"}, PaidCap: -1})
	if err != nil || e2.Attempt != 2 {
		t.Fatalf("begin 2: %v %+v", err, e2)
	}
	if err := store.Finish(ctx, "k", 2, imagegen.ReceiptFailed, false, nil, "tool_missing", "chưa đăng nhập", time.Now()); err != nil {
		t.Fatalf("finish 2: %v", err)
	}
	if n, err := store.Charged(ctx, "codex", e1.StartedAt.Add(-time.Hour)); err != nil || n != 1 {
		t.Fatalf("charged sau failed-free = %d, %v", n, err)
	}
	if n, err := store.Charged(ctx, "grok", e1.StartedAt.Add(-time.Hour)); err != nil || n != 0 {
		t.Fatalf("charged grok = %d", n)
	}
	if n, err := store.Charged(ctx, "codex", time.Now().Add(time.Hour)); err != nil || n != 0 {
		t.Fatalf("charged tương lai = %d", n)
	}
}

// Scratch chỉ cập nhật khi attempt còn running.
func TestReceiptScratchOnlyWhileRunning(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)

	e, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "s", Provider: "codex", Model: "m", Output: "/tmp/s.png"}, PaidCap: -1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateScratch(ctx, "s", e.Attempt, json.RawMessage(`{"pid":1}`)); err != nil {
		t.Fatalf("scratch: %v", err)
	}
	latest, _ := store.Latest(ctx, "s")
	if scratchPID(latest.Scratch) != 1 {
		t.Fatalf("scratch = %s", latest.Scratch)
	}
	if err := store.Finish(ctx, "s", e.Attempt, imagegen.ReceiptOK, true, nil, "", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateScratch(ctx, "s", e.Attempt, json.RawMessage(`{"pid":2}`)); err != nil {
		t.Fatal(err)
	}
	latest, _ = store.Latest(ctx, "s")
	if scratchPID(latest.Scratch) != 1 {
		t.Fatalf("scratch sau finish = %s — attempt hoàn tất là bất biến", latest.Scratch)
	}
}

// scratchPID đọc trường pid của scratch jsonb (Postgres chuẩn hoá khoảng trắng).
func scratchPID(raw json.RawMessage) int {
	var v struct {
		PID int `json:"pid"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.PID
}

// Abandon gỡ chặn nhưng giữ bản ghi làm bằng chứng.
func TestReceiptAbandonKeepsEvidence(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)

	e, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "a", Provider: "codex", Model: "m", Output: "/tmp/a.png"}, PaidCap: -1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, "a", e.Attempt, imagegen.ReceiptFailedUnknown, true, nil, "failed_unknown", "mất dấu", time.Now()); err != nil {
		t.Fatal(err)
	}
	latest, _ := store.Latest(ctx, "a")
	if latest == nil || latest.State != imagegen.ReceiptFailedUnknown {
		t.Fatalf("latest = %+v", latest)
	}
	if err := store.Abandon(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if latest, _ := store.Latest(ctx, "a"); latest != nil {
		t.Fatalf("latest sau abandon = %+v", latest)
	}
	// Bằng chứng vẫn đếm phí.
	if n, err := store.Charged(ctx, "codex", e.StartedAt.Add(-time.Hour)); err != nil || n != 1 {
		t.Fatalf("charged = %d", n)
	}
	// Begin mới vẫn kế thừa số attempt (evidence không mất).
	e2, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "a", Provider: "codex", Model: "m", Output: "/tmp/a.png"}, PaidCap: -1})
	if err != nil || e2.Attempt != 2 {
		t.Fatalf("begin sau abandon: %v %+v", err, e2)
	}
}

// Begin thứ hai trên cùng key khi attempt đầu còn running bị từ chối: mỗi key
// chỉ có một attempt chạy, nên không thể có hai lần tiêu phí song song.
func TestReceiptSecondRunningBeginConflicts(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)

	adm := imagegen.Admission{
		Entry:   imagegen.JournalEntry{Key: "u", Provider: "codex", Model: "m", Output: "/tmp/u.png"},
		PaidCap: -1,
	}
	if _, err := store.Begin(ctx, adm); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(ctx, adm); !errors.Is(err, imagegen.ErrAttemptConcluded) {
		t.Fatalf("begin lần 2 = %v, mong ErrAttemptConcluded", err)
	}
	if n, err := store.Charged(ctx, "codex", time.Now().Add(-time.Hour)); err != nil || n != 1 {
		t.Fatalf("charged = %d, %v", n, err)
	}
	// Attempt đã kết luận thì mở được attempt mới cho cùng key.
	if err := store.Finish(ctx, "u", 1, imagegen.ReceiptFailed, false, nil, "tool_missing", "chưa đăng nhập", time.Now()); err != nil {
		t.Fatal(err)
	}
	if e, err := store.Begin(ctx, adm); err != nil || e.Attempt != 2 {
		t.Fatalf("begin sau finish: %v %+v", err, e)
	}
}

// Phục hồi crash trên Postgres thật: attempt running + output hợp lệ → cứu
// ảnh, không vẽ lại; thiếu output → failed_unknown và key bị khoá.
func TestReceiptCrashRecoveryOnPostgres(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)
	dir := t.TempDir()

	// Ca 1: output đã lưu xong trước khi chết.
	out := filepath.Join(dir, "saved.png")
	if err := os.WriteFile(out, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "cr1", Provider: "codex", Model: "m", Output: out}, PaidCap: -1}); err != nil {
		t.Fatal(err)
	}
	prov := &crashProvider{}
	svc := imagegen.NewService(imagegen.Options{
		Providers: []imagegen.Provider{prov},
		Journal:   store,
	})
	r, err := svc.Draw(ctx, imagegen.Request{Key: "cr1", Prompt: "x", Output: out})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if prov.draws != 0 {
		t.Fatalf("provider bị gọi %d lần khi phục hồi", prov.draws)
	}
	if r.PngSha256 == "" || r.Attribution != "output-adopted" {
		t.Fatalf("receipt = %+v", r)
	}
	latest, _ := store.Latest(ctx, "cr1")
	if latest.State != imagegen.ReceiptOK {
		t.Fatalf("state = %s", latest.State)
	}

	// Ca 2: chết trước khi output lưu → failed_unknown, không vẽ lại mù.
	out2 := filepath.Join(dir, "lost.png")
	if _, err := store.Begin(ctx, imagegen.Admission{Entry: imagegen.JournalEntry{Key: "cr2", Provider: "codex", Model: "m", Output: out2}, PaidCap: -1}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Draw(ctx, imagegen.Request{Key: "cr2", Prompt: "x", Output: out2})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeUncertain {
		t.Fatalf("mong failed_unknown, được %v", err)
	}
	if prov.draws != 0 {
		t.Fatalf("provider bị gọi khi attempt bất định: %d", prov.draws)
	}
	_, err = svc.Draw(ctx, imagegen.Request{Key: "cr2", Prompt: "x", Output: out2})
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeUncertain {
		t.Fatalf("lần 2 mong failed_unknown, được %v", err)
	}
	// Abandon tường minh mở khoá — lần vẽ mới là attempt 2.
	if err := svc.Abandon(ctx, "cr2"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Draw(ctx, imagegen.Request{Key: "cr2", Prompt: "x", Output: out2}); err != nil {
		t.Fatalf("redraw: %v", err)
	}
	if prov.draws != 1 {
		t.Fatalf("draws = %d", prov.draws)
	}
}

// Trần ngày được kiểm NGUYÊN TỬ với việc ghi attempt trên Postgres: 49/50 qua,
// lần 51 bị chặn kèm số đã dùng; trần 0 chặn ngay lần đầu.
func TestReceiptCapOnPostgres(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	store := postgres.NewImageReceiptStore(pool)
	dayStart := time.Date(2026, 10, 30, 0, 0, 0, 0, time.Local)
	startedAt := dayStart.Add(9 * time.Hour)
	adm := func(key string, cap int) imagegen.Admission {
		return imagegen.Admission{
			Entry:    imagegen.JournalEntry{Key: key, Provider: "codex", Model: "m", Output: "/tmp/" + key + ".png", StartedAt: startedAt},
			PaidCap:  cap,
			DayStart: dayStart,
		}
	}
	for i := 1; i <= 50; i++ {
		e, err := store.Begin(ctx, adm(fmt.Sprintf("cap-%d", i), 50))
		if err != nil {
			t.Fatalf("begin %d: %v", i, err)
		}
		if err := store.Finish(ctx, e.Key, e.Attempt, imagegen.ReceiptOK, true, nil, "", "", startedAt); err != nil {
			t.Fatalf("finish %d: %v", i, err)
		}
	}
	_, err := store.Begin(ctx, adm("cap-51", 50))
	var cre *imagegen.CapReachedError
	if !errors.As(err, &cre) {
		t.Fatalf("lần 51 = %v, mong CapReachedError", err)
	}
	if cre.Used != 50 || cre.Limit != 50 || cre.Provider != "codex" {
		t.Fatalf("cap = %+v", cre)
	}
	// Không ghi attempt nào khi bị chặn.
	if latest, _ := store.Latest(ctx, "cap-51"); latest != nil {
		t.Fatalf("attempt bị chặn vẫn được ghi: %+v", latest)
	}
	// Trần 0 = máy không bao giờ tự tiêu.
	if _, err := store.Begin(ctx, adm("cap-zero", 0)); !errors.As(err, &cre) || cre.Limit != 0 {
		t.Fatalf("trần 0 = %v", err)
	}
	// Ngày mới (sau nửa đêm) mở lại hạn mức.
	next := dayStart.Add(25 * time.Hour)
	e, err := store.Begin(ctx, imagegen.Admission{
		Entry:    imagegen.JournalEntry{Key: "cap-next", Provider: "codex", Model: "m", Output: "/tmp/n.png", StartedAt: next},
		PaidCap:  50,
		DayStart: localMidnight(next),
	})
	if err != nil {
		t.Fatalf("begin ngày mới: %v", err)
	}
	if e.Attempt != 1 {
		t.Fatalf("attempt = %d", e.Attempt)
	}
}

// localMidnight là nửa đêm của ngày chứa t theo loc của t.
func localMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
