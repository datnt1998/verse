// Smoke thật của đường vẽ ảnh trả phí: ĐÚNG MỘT ảnh thật qua Codex CLI cô
// lập với trần ngày = 1, rồi lần vẽ thứ hai phải bị trần từ chối kèm retryAt
// và không có process codex thứ hai nào được spawn.
//
// Tắt mặc định: chỉ chạy khi VIDERSE_IMAGEGEN_SMOKE=1 (cần Codex đã đăng nhập
// ChatGPT + preflight cô lập verified). TEST_DATABASE_URL dùng cho biên nhận
// Postgres; thiếu thì rơi về journal in-memory — không ảnh hưởng ảnh trả phí.
package imagegen_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// countingRunner đếm riêng các lần exec vẽ ảnh (cuối argv là prompt ảnh) —
// probe preflight miễn phí (`exec --help`, advertisement "Inspect local
// capabilities") không được tính.
type countingRunner struct {
	inner tools.Runner
	n     int
}

func (r *countingRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	if len(args) > 1 && args[0] == "exec" && args[1] != "--help" &&
		!strings.HasPrefix(args[len(args)-1], "Inspect local capabilities") {
		r.n++
	}
	return r.inner.Run(ctx, file, args, spec)
}

func TestSmokeOnePaidImageThenCapRefuses(t *testing.T) {
	if os.Getenv("VIDERSE_IMAGEGEN_SMOKE") != "1" {
		t.Skip("smoke trả phí: đặt VIDERSE_IMAGEGEN_SMOKE=1 để chạy")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	runner := &countingRunner{inner: &tools.ExecRunner{}}
	deps := tools.Deps{
		Runner:  runner,
		Paths:   &tools.FilePathStore{Dir: filepath.Join(home, ".viderse", "tools")},
		BaseDir: filepath.Join(home, ".viderse"),
		Home:    home,
	}
	tl := tools.New(deps)
	ctx := context.Background()

	provider := imagegen.NewCodexProvider(imagegen.CodexDeps{
		Codex:  tl.Codex,
		Runner: runner,
	})
	// Chặn sớm nếu chưa đăng nhập/cô lập chưa verified: báo lỗi chính xác.
	if err := provider.Ready(ctx); err != nil {
		t.Fatalf("codex chưa sẵn sàng — không vẽ (BLOCKED): %v", err)
	}

	var journal imagegen.Journal
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		journal = pgJournal(t, dsn)
	} else {
		journal = newMemoryJournal()
	}
	outDir := "/tmp/viderse-imagegen-smoke"
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(outDir, "smoke.png")
	os.Remove(out)
	os.Remove(out + ".receipt.json")

	svc := imagegen.NewService(imagegen.Options{
		Providers:           []imagegen.Provider{provider},
		Journal:             journal,
		DailyPaidImageLimit: new(1), // trần 1: một ảnh trả phí duy nhất
	})
	receipt, err := svc.Draw(ctx, imagegen.Request{
		Key:    "smoke-" + time.Now().Format("20060102-150405"),
		Prompt: "a small plain blue circle on a white background",
		Output: out, Width: 1024, Height: 1024,
	})
	if err != nil {
		t.Fatalf("lần vẽ đầu (ảnh trả phí duy nhất): %v", err)
	}
	if runner.n != 1 {
		t.Fatalf("số process exec = %d, want 1", runner.n)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	// Báo cáo kích thước + dung lượng thật.
	w, h := pngDims(t, out)
	t.Logf("SMOKE IMAGE: path=%s size=%dx%d bytes=%d provider=%s model=%s session=%s call=%s",
		out, w, h, st.Size(), receipt.Provider, receipt.Model, receipt.SessionID, receipt.CallID)
	if w == 0 || h == 0 || st.Size() < 100 {
		t.Fatalf("ảnh không hợp lệ: %dx%d %d bytes", w, h, st.Size())
	}

	// Lần vẽ thứ hai: trần từ chối kèm retryAt, không spawn thêm process.
	_, err = svc.Draw(ctx, imagegen.Request{
		Key:    "smoke-second-" + time.Now().Format("150405.000"),
		Prompt: "another image",
		Output: filepath.Join(outDir, "smoke-2.png"),
	})
	var ce *imagegen.CapError
	if !errors.As(err, &ce) {
		t.Fatalf("lần vẽ thứ hai phải bị trần chặn: %v", err)
	}
	if !ce.RetryAt.After(time.Now()) {
		t.Fatalf("retryAt = %v", ce.RetryAt)
	}
	t.Logf("CAP REFUSED: provider=%s used=%d limit=%d retryAt=%s", ce.Provider, ce.Used, ce.Limit, ce.RetryAt)
	if runner.n != 1 {
		t.Fatalf("process codex thứ hai đã spawn: n=%d", runner.n)
	}
}

// pngDims đọc kích thước từ IHDR (ảnh đã qua ValidatePng của provider).
func pngDims(t *testing.T, file string) (uint32, uint32) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil || len(data) < 24 {
		t.Fatalf("đọc png: %v", err)
	}
	w := uint32(data[16])<<24 | uint32(data[17])<<16 | uint32(data[18])<<8 | uint32(data[19])
	h := uint32(data[20])<<24 | uint32(data[21])<<16 | uint32(data[22])<<8 | uint32(data[23])
	return w, h
}

// pgJournal provision một schema Postgres tạm (pgtest) + migration rồi trả
// ImageReceiptStore cho smoke.
func pgJournal(t *testing.T, dsn string) imagegen.Journal {
	t.Helper()
	t.Setenv("TEST_DATABASE_URL", dsn)
	schemaDSN := pgtest.Provision(t)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(schemaDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool, err := postgres.OpenPool(ctx, schemaDSN)
	if err != nil {
		t.Fatalf("mở pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return postgres.NewImageReceiptStore(pool)
}
