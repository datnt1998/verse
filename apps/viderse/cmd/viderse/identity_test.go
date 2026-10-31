package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/testidentity"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// dsnWithParams thêm query param vào DSN (application_name để đếm kết nối
// của từng pool trên pg_stat_activity).
func dsnWithParams(t *testing.T, dsn string, kvs ...string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	q := u.Query()
	for i := 0; i+1 < len(kvs); i += 2 {
		q.Set(kvs[i], kvs[i+1])
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// countConns đếm session Postgres đang mở theo application_name.
func countConns(t *testing.T, adminDSN, appName string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("kết nối đếm: %v", err)
	}
	defer conn.Close(context.Background())
	var n int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM pg_stat_activity WHERE application_name = $1`, appName).Scan(&n); err != nil {
		t.Fatalf("đếm kết nối: %v", err)
	}
	return n
}

// dumpConns log chi tiết phiên theo application_name — chẩn đoán pool nào leak.
func dumpConns(t *testing.T, adminDSN, appName string) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		return
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(),
		`SELECT pid, usename, state, left(query, 80) FROM pg_stat_activity WHERE application_name = $1`, appName)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var pid int
		var user, state, q string
		if err := rows.Scan(&pid, &user, &state, &q); err == nil {
			t.Logf("conn pid=%d user=%s state=%s query=%q", pid, user, state, q)
		}
	}
}

// waitConns chờ tới khi số session của appName đạt đúng want.
func waitConns(t *testing.T, adminDSN, appName string, want int, timeout time.Duration) {
	t.Helper()
	waitConnsWhere(t, adminDSN, appName, func(n int) bool { return n == want }, timeout)
}

// waitConnsAtLeast chờ tới khi số session của appName ≥ min.
func waitConnsAtLeast(t *testing.T, adminDSN, appName string, min int, timeout time.Duration) {
	t.Helper()
	waitConnsWhere(t, adminDSN, appName, func(n int) bool { return n >= min }, timeout)
}

func waitConnsWhere(t *testing.T, adminDSN, appName string, ok func(int) bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok(countConns(t, adminDSN, appName)) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("kết nối %q không đạt điều kiện sau %s (còn %d)", appName, timeout, countConns(t, adminDSN, appName))
}

// clearServeEnv cô lập serve khỏi môi trường thật: mọi biến ảnh hưởng parse
// phải được đặt tường minh trong test.
func clearServeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{
		"VERSE_IDENTITY_DATABASE_URL", "VIDERSE_ADDR",
		"TRUSTED_PROXIES", "PUBLIC_URL", "SECURE_COOKIES",
		"VERSE_COOKIE_DOMAIN", "VERSE_APP_ORIGINS",
	} {
		t.Setenv(k, "")
	}
	// Phần nghiệp vụ của serve có thể chạm HOME để lấy tool config — luôn
	// kiểm không có file tài khoản legacy được tạo ra.
	t.Cleanup(func() {
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".viderse", "web-accounts.json")); !os.IsNotExist(err) {
			t.Errorf("serve tạo web-accounts.json (err=%v)", err)
		}
	})
}

// TestViderseIdentityDSNNoFallback: DSN identity là bắt buộc — thiếu thì
// fail trước mọi side effect; DSN data/migrate không bao giờ được dùng thay;
// DSN sai → lỗi đã lọc credential, không DSN nào được mở.
func TestViderseIdentityDSNNoFallback(t *testing.T) {
	clearServeEnv(t)
	// Blackhole với connect_timeout ngắn: đủ để pool mở-thất-bại nhanh, đủ
	// chậm để lộ mọi đường vượt qua được kiểm tra cấu hình trước.
	bogusData := "postgres://u:p@10.255.255.1:1/bogus?connect_timeout=1"
	t.Setenv("VIDERSE_DATABASE_URL", bogusData)
	t.Setenv("VIDERSE_MIGRATION_DATABASE_URL", bogusData)

	// Thiếu hẳn VERSE_IDENTITY_DATABASE_URL: fail ngay, không fallback sang
	// VIDERSE_DATABASE_URL / migrate DSN / file (nếu có fallback, DSN data
	// blackhole sẽ treo thay vì trả lỗi ngay).
	err := runServe([]string{"-addr", "127.0.0.1:0"})
	if err == nil || !strings.Contains(err.Error(), "VERSE_IDENTITY_DATABASE_URL") {
		t.Fatalf("thiếu identity DSN = %v, muốn lỗi bắt buộc", err)
	}

	// DSN không parse được: lỗi generic đã lọc, không echo userinfo.
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", ":::không-parse-được")
	err = runServe([]string{"-addr", "127.0.0.1:0"})
	if err == nil {
		t.Fatal("DSN sai phải fail")
	}

	// Đường identity: DSN data thật để lỗi duy nhất là kết nối identity —
	// identitypg.Open phải lọc credential: không password hay userinfo.
	_, dataDSN := provisionCLI(t)
	t.Setenv("VIDERSE_DATABASE_URL", dataDSN)
	t.Setenv("VERSE_IDENTITY_DATABASE_URL",
		"postgres://user:matkhau123@10.255.255.1:65000/verseid?connect_timeout=1")
	err = runServe([]string{"-addr", "127.0.0.1:0"})
	if err == nil {
		t.Fatal("DSN blackhole phải fail")
	}
	for _, leak := range []string{"matkhau123", "user:matkhau123", "password="} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("lỗi lộ credential/DSN %q: %v", leak, err)
		}
	}
}

// TestSSOLocalBindRequiresLoopback: local SSO (VERSE_APP_ORIGINS không có
// PUBLIC_URL) chỉ bind loopback — wildcard/LAN bị từ chối trước khi mở pool.
func TestSSOLocalBindRequiresLoopback(t *testing.T) {
	for _, tc := range []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:18790", true},
		{"[::1]:18790", true},
		{"localhost:18790", true},
		{"0.0.0.0:18790", false},
		{"[::]:18790", false},
		{":18790", false},
		{"192.168.1.5:18790", false},
		{"100.64.0.1:18790", false},
	} {
		if got := ssoLoopbackOnly(tc.addr); got != tc.ok {
			t.Fatalf("ssoLoopbackOnly(%q) = %v, want %v", tc.addr, got, tc.ok)
		}
	}

	clearServeEnv(t)
	// Bind bị từ chối xảy ra TRƯỚC khi mở pool: DSN blackhole không làm treo
	// test và lỗi phải nói về loopback, không phải lỗi kết nối.
	t.Setenv("VIDERSE_DATABASE_URL", "postgres://u:p@10.255.255.1:1/bogus")
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", "postgres://u:p@10.255.255.1:1/bogus")
	t.Setenv("VERSE_APP_ORIGINS", "http://127.0.0.1:18686,http://127.0.0.1:18789")
	for _, addr := range []string{"0.0.0.0:0", "[::]:0", ":0"} {
		if err := runServe([]string{"-addr", addr}); err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("local SSO bind %q = %v, muốn từ chối loopback", addr, err)
		}
	}
}

// TestViderseGrantCLIPostgresOperator: `viderse user grant|revoke --app` đi
// qua parser chung và đụng kho bằng identity DSN — grant tạo row member,
// revoke xoá nó; thiếu --app/--actor/DSN bị từ chối trước khi mở kho.
func TestViderseGrantCLIPostgresOperator(t *testing.T) {
	clearServeEnv(t)
	fx := testidentity.Provision(t)
	fx.CreateMember(t, "linh", "linh-pass-9", false, identity.AppNoverse)
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", fx.IdentityDSN)

	grantsOf := func() map[identity.App]identity.Role {
		t.Helper()
		g, err := fx.Accounts.AppGrants("linh")
		if err != nil {
			t.Fatalf("đọc grants: %v", err)
		}
		return g
	}
	if err := runUser([]string{"grant", "linh", "--app", "viderse"}); err != nil {
		t.Fatalf("user grant: %v", err)
	}
	if grantsOf()[identity.AppViderse] != identity.RoleMember {
		t.Fatalf("grant không được ghi: %+v", grantsOf())
	}
	if err := runUser([]string{"revoke", "linh", "--app", "viderse"}); err != nil {
		t.Fatalf("user revoke: %v", err)
	}
	if _, ok := grantsOf()[identity.AppViderse]; ok {
		t.Fatalf("revoke không xoá row: %+v", grantsOf())
	}

	// Ngữ pháp bị từ chối trước khi mở kho — đặt DSN sai vẫn lỗi parse trước.
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", "postgres://u:p@10.255.255.1:1/x")
	for _, args := range [][]string{
		{"grant", "linh"},
		{"grant", "linh", "--app", "noverse", "--actor", "admin"},
		{"grant", "linh", "--dsn", "postgres://u:p@h/db", "--app", "noverse"},
		{"grant", "linh", "--app", "khac"},
		{"list"},
	} {
		if err := runUser(args); err == nil {
			t.Fatalf("user %v thành công — operator boundary hỏng", args)
		}
	}
}

// TestServeShutdownClosesBothPools: serve sống thật trên hai DB thật; sau
// SIGTERM mọi kết nối của pool data lẫn pool identity phải về 0 — closer của
// hai pool chạy trên đường shutdown.
func TestServeShutdownClosesBothPools(t *testing.T) {
	clearServeEnv(t)
	bootstrap.ConfigureIdentity()
	fx := testidentity.Provision(t)
	// Chỉ dựng schema + migration, KHÔNG giữ pool fixture: con đếm trên
	// pg_stat_activity phải phản ánh đúng pool của runServe.
	dataDSN := provisionSchemaOnly(t)
	adminDSN := pgtest.RequireURL(t)

	identityDSN := dsnWithParams(t, fx.IdentityDSN, "application_name", "vd_teardown_id")
	dataDSN = dsnWithParams(t, dataDSN, "application_name", "vd_teardown_data")
	t.Setenv("VIDERSE_DATABASE_URL", dataDSN)
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", identityDSN)

	// Cổng trống ngẫu nhiên, tránh va chạm suite khác.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	done := make(chan error, 1)
	go func() { done <- runServe([]string{"-addr", addr}) }()

	// Chờ server thật sự sống: /healthz 200 nghĩa là NotifyContext đã gắn —
	// gửi tín hiệu trước mốc này có thể giết cả tiến trình test.
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve không lên sau 30s; runServe trả %v", <-done)
		}
		time.Sleep(100 * time.Millisecond)
	}

	waitConnsAtLeast(t, adminDSN, "vd_teardown_id", 1, 10*time.Second)
	waitConnsAtLeast(t, adminDSN, "vd_teardown_data", 1, 10*time.Second)

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("gửi SIGTERM: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServe shutdown trả lỗi: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("runServe không thoát sau SIGTERM")
	}
	waitConns(t, adminDSN, "vd_teardown_id", 0, 10*time.Second)
	dumpConns(t, adminDSN, "vd_teardown_data")
	waitConns(t, adminDSN, "vd_teardown_data", 0, 10*time.Second)
}

// TestServeClosesDataPoolWhenIdentityFails: identity DSN blackhole (timeout
// ~10s của ConnectTimeout) — trong khi runServe chờ identity, pool data đã
// mở phải hiện trên pg_stat_activity, rồi phải về 0 khi runServe thất bại.
func TestServeClosesDataPoolWhenIdentityFails(t *testing.T) {
	clearServeEnv(t)
	dataDSN := provisionSchemaOnly(t)
	adminDSN := pgtest.RequireURL(t)
	dataDSN = dsnWithParams(t, dataDSN, "application_name", "vd_teardown_early")
	t.Setenv("VIDERSE_DATABASE_URL", dataDSN)
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", "postgres://u:p@10.255.255.1:65000/noidb")

	done := make(chan error, 1)
	go func() { done <- runServe([]string{"-addr", "127.0.0.1:0"}) }()

	// Pool data phải đã mở (ping) trong lúc identity chờ timeout.
	waitConns(t, adminDSN, "vd_teardown_early", 1, 20*time.Second)
	if err := <-done; err == nil {
		t.Fatal("serve phải thất bại khi identity DSN không kết nối được")
	} else if strings.Contains(err.Error(), ":p@") {
		t.Fatalf("lỗi lộ DSN: %v", err)
	}
	waitConns(t, adminDSN, "vd_teardown_early", 0, 10*time.Second)
}

// TestServePortBusyDoesNotTouchFirstServer: cổng bận → fail trước khi mở
// pool: server đầu vẫn trả lời và không có kết nối DB nào được mở.
func TestServePortBusyDoesNotTouchFirstServer(t *testing.T) {
	clearServeEnv(t)
	first := newTestHTTPServer(t) // stub listener, không DB
	defer first.Close()

	// DSN blackhole: listener bận → runServe lỗi trước khi mở pool, nên
	// không connect nào được thử — test thoát ngay thay vì treo ~10s.

	t.Setenv("VIDERSE_DATABASE_URL", "postgres://u:p@10.255.255.1:65000/bogus")
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", "postgres://u:p@10.255.255.1:65000/bogus")
	if err := runServe([]string{"-addr", first.Addr()}); err == nil || !strings.Contains(err.Error(), "bind") && !strings.Contains(err.Error(), "address") {
		t.Fatalf("cổng bận = %v, muốn lỗi bind", err)
	}
	// Server đầu còn nguyên: vẫn trả lời; không pool nào được mở (DSN
	// blackhole mà không treo chứng tỏ listener fail trước khi connect).
	resp, err := http.Get("http://" + first.Addr() + "/")
	if err != nil {
		t.Fatalf("server đầu bị ảnh hưởng: %v", err)
	}
	resp.Body.Close()
}

// newTestHTTPServer mở listener HTTP trả 200 — giữ cổng cho test cổng bận.
func newTestHTTPServer(t *testing.T) *stubServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &stubServer{ln: ln, srv: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}}
	go s.srv.Serve(ln)
	return s
}

type stubServer struct {
	ln  net.Listener
	srv *http.Server
}

func (s *stubServer) Addr() string { return s.ln.Addr().String() }

func (s *stubServer) Close() {
	_ = s.srv.Close()
	_ = s.ln.Close()
}

// provisionSchemaOnly dựng schema viderse + migration nhưng không giữ pool
// fixture — để đếm pg_stat_activity phản ánh đúng kết nối của runServe.
func provisionSchemaOnly(t *testing.T) string {
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
	return appDSN
}
