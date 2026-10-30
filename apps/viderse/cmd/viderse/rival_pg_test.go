// Ca test CLI trên Postgres thật: `--cap` phải đi cùng `--history`/`--action`
// thay vì che nhau, kiểm trên trạng thái việc đã ghi khi một tiến trình khác
// đang giữ lease quét (nên CLI không tự chạy worker inline).
//
// Bỏ qua khi thiếu TEST_DATABASE_URL.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// cliRoleSpec là cặp role riêng của suite CLI — khoá advisory khác các suite
// khác để chạy song song trên cùng cụm không tranh nhau.
var cliRoleSpec = pgtest.RoleSpec{
	MigrateRole:   "viderse_migrate",
	AppRole:       "viderse_app",
	LockKey1:      2790,
	LockKey2:      3,
	InitScriptRef: "deploy/postgres-init/01-roles.sh",
}

// provisionCLI dựng schema riêng và trả pool app + DSN app (để đặt env cho
// đường mở DB của CLI).
func provisionCLI(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t, cliRoleSpec)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool, err := postgres.OpenPool(ctx, appDSN)
	if err != nil {
		t.Fatalf("mở pool app: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, appDSN
}

// heldLease ghi một lease quét còn hiệu lực để RunInline không chạy worker.
func heldLease(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO platform_state (platform, scanner_owner, scanner_lease_until)
		VALUES ('youtube', 'serve-test', now() + interval '5 minutes')
		ON CONFLICT (platform) DO UPDATE
		SET scanner_owner = 'serve-test',
		    scanner_lease_until = now() + interval '5 minutes'`); err != nil {
		t.Fatal(err)
	}
}

// historyJobState đọc trạng thái việc lịch sử mới nhất của kênh.
func historyJobState(t *testing.T, pool *pgxpool.Pool, channelID string) (string, int) {
	t.Helper()
	var state string
	err := pool.QueryRow(context.Background(),
		`SELECT state FROM scan_jobs WHERE channel_id = $1 AND kind = 'history'
		 ORDER BY updated_at DESC LIMIT 1`, channelID).Scan(&state)
	if err != nil {
		t.Fatalf("đọc việc lịch sử: %v", err)
	}
	var cap int
	if err := pool.QueryRow(context.Background(),
		`SELECT history_cap FROM scan_state WHERE channel_id = $1`, channelID).Scan(&cap); err != nil {
		t.Fatalf("đọc cap: %v", err)
	}
	return state, cap
}

// TestRivalScanCapComposesWithHistoryAndAction khoá hành vi: cap đi cùng
// history/action thì CẢ HAI đều được áp.
func TestRivalScanCapComposesWithHistoryAndAction(t *testing.T) {
	pool, appDSN := provisionCLI(t)
	t.Setenv("VIDERSE_DATABASE_URL", appDSN)
	ctx := context.Background()

	channelID := "UC0000000000000000000001"
	if _, err := pool.Exec(ctx,
		`INSERT INTO rival_channels (id, handle, title, url) VALUES ($1, '@a', 'Kênh A', 'https://youtube.com/@a')`,
		channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO scan_state (channel_id) VALUES ($1)`, channelID); err != nil {
		t.Fatal(err)
	}
	heldLease(t, pool)

	// --history --cap 150: tạo việc lịch sử VÀ đặt cap.
	var out, errBuf bytes.Buffer
	if code := runRivalCommand([]string{"scan", channelID, "--history", "--cap", "150"}, &out, &errBuf); code != 0 {
		t.Fatalf("history+cap = %d: %s", code, errBuf.String())
	}
	state, gotCap := historyJobState(t, pool, channelID)
	if state != "queued" {
		t.Fatalf("sau --history: việc %q, muốn queued (cap %d)", state, gotCap)
	}
	if gotCap != 150 {
		t.Fatalf("cap = %d, muốn 150", gotCap)
	}

	// --cap 300 --action pause: đổi cap VÀ dừng việc đang queued.
	out.Reset()
	errBuf.Reset()
	if code := runRivalCommand([]string{"scan", channelID, "--cap", "300", "--action", "pause"}, &out, &errBuf); code != 0 {
		t.Fatalf("cap+pause = %d: %s", code, errBuf.String())
	}
	state, gotCap = historyJobState(t, pool, channelID)
	if state != "paused" {
		t.Fatalf("sau --cap --action pause: việc %q, muốn paused", state)
	}
	if gotCap != 300 {
		t.Fatalf("cap = %d, muốn 300", gotCap)
	}

	// Chỉ --cap: đổi giới hạn, không đụng trạng thái việc.
	out.Reset()
	errBuf.Reset()
	if code := runRivalCommand([]string{"scan", channelID, "--cap", "500"}, &out, &errBuf); code != 0 {
		t.Fatalf("chỉ cap = %d: %s", code, errBuf.String())
	}
	state, gotCap = historyJobState(t, pool, channelID)
	if state != "paused" {
		t.Fatalf("chỉ --cap đổi trạng thái việc: %q", state)
	}
	if gotCap != 500 {
		t.Fatalf("cap = %d, muốn 500", gotCap)
	}

	// Stdout vẫn là ScanStatus JSON của lần chạy cuối.
	var status map[string]any
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("stdout không phải JSON: %q", out.String())
	}
	if status["channelId"] != channelID || status["historyCap"].(float64) != 500 {
		t.Fatalf("status = %v", status)
	}
	_ = time.Now
}

// TestRivalScanCapOnlyNeverRunsInline: chỉ `--cap` không được gọi worker
// inline — cả khi lease tự do (worker sẽ thật sự xử lý việc xếp sẵn và gọi
// YouTube) lẫn khi serve giữ lease. Việc lịch sử phải nguyên trạng.
func TestRivalScanCapOnlyNeverRunsInline(t *testing.T) {
	ctx := context.Background()

	// trap đếm mọi request HTTP mà một worker inline có thể tạo (youtube
	// client dùng http.DefaultTransport khi không tiêm client riêng).
	var calls int
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, fmt.Errorf("provider bị chặn trong test")
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	seed := func(t *testing.T, channelID string) (*pgxpool.Pool, string) {
		pool, appDSN := provisionCLI(t)
		if _, err := pool.Exec(ctx,
			`INSERT INTO rival_channels (id, handle, title, url) VALUES ($1, '@c', 'Kênh C', 'https://youtube.com/@c')`,
			channelID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO scan_state (channel_id) VALUES ($1)`, channelID); err != nil {
			t.Fatal(err)
		}
		// Việc lịch sử đang queued — nếu CLI chạy inline nó sẽ bị xử lý.
		if _, err := pool.Exec(ctx,
			`INSERT INTO scan_jobs (id, channel_id, kind, state, requested_cap)
			 VALUES (gen_random_uuid(), $1, 'history', 'queued', 500)`, channelID); err != nil {
			t.Fatal(err)
		}
		return pool, appDSN
	}
	jobState := func(t *testing.T, pool *pgxpool.Pool, channelID string) string {
		var state string
		if err := pool.QueryRow(ctx,
			`SELECT state FROM scan_jobs WHERE channel_id = $1 AND kind = 'history' ORDER BY updated_at DESC LIMIT 1`,
			channelID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		return state
	}

	t.Run("lease tự do", func(t *testing.T) {
		pool, appDSN := seed(t, "UC0000000000000000000010")
		t.Setenv("VIDERSE_DATABASE_URL", appDSN)
		before := calls

		var out, errBuf bytes.Buffer
		if code := runRivalCommand([]string{"scan", "UC0000000000000000000010", "--cap", "400"}, &out, &errBuf); code != 0 {
			t.Fatalf("chỉ --cap = %d: %s", code, errBuf.String())
		}
		if calls != before {
			t.Fatalf("chỉ --cap tạo %d request provider — phải 0 (worker đã chạy)", calls-before)
		}
		if state := jobState(t, pool, "UC0000000000000000000010"); state != "queued" {
			t.Fatalf("việc lịch sử = %q, muốn queued — không được xử lý", state)
		}
		if _, cap := historyJobState(t, pool, "UC0000000000000000000010"); cap != 400 {
			t.Fatalf("cap = %d, muốn 400", cap)
		}
	})

	t.Run("lease bị giữ", func(t *testing.T) {
		pool, appDSN := seed(t, "UC0000000000000000000011")
		heldLease(t, pool)
		t.Setenv("VIDERSE_DATABASE_URL", appDSN)
		before := calls

		var out, errBuf bytes.Buffer
		if code := runRivalCommand([]string{"scan", "UC0000000000000000000011", "--cap", "400"}, &out, &errBuf); code != 0 {
			t.Fatalf("chỉ --cap = %d: %s", code, errBuf.String())
		}
		if calls != before {
			t.Fatalf("chỉ --cap tạo %d request provider — phải 0", calls-before)
		}
		if state := jobState(t, pool, "UC0000000000000000000011"); state != "queued" {
			t.Fatalf("việc lịch sử = %q, muốn queued", state)
		}
	})
}

// roundTripperFunc cho phép một hàm làm http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
