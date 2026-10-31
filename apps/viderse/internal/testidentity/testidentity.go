// Package testidentity dựng fixture kho tài khoản chung cho các test cần
// Postgres thật: schema riêng (pgtest), migration identity của Noverse áp từ
// thư mục SQL nguồn (không import apps/noverse — ranh giới archtest), rồi
// role viderse_identity thật từ script deploy. Bỏ qua khi thiếu
// TEST_DATABASE_URL.
package testidentity

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// rolePassword là mật khẩu throwaway của viderse_identity trong test —
// chỉ sống trên cụm test, không bao giờ là credential thật.
const rolePassword = "viderse-identity-test-only"

// Fixture là một schema identity đã migrate + role đã grant, kèm các
// connection đã mở sẵn cho test.
type Fixture struct {
	// Accounts là kho nhìn bằng role viderse_identity (ProjectAccess=false) —
	// đúng mặt store mà viderse serve dùng trong sản xuất.
	Accounts *identity.Store
	// Admin là kho nhìn bằng DSN quản trị (ProjectAccess=true) để fixture tạo
	// user/grant nhanh mà không qua HTTP.
	Admin *identity.Store
	// AdminDSN kết nối thẳng (search_path vào schema test) cho các truy vấn
	// kiểm chứng (vd. đọc must_change_password sau đổi mật khẩu).
	AdminDSN string
	// IdentityDSN là DSN role viderse_identity — giá trị đặt vào
	// VERSE_IDENTITY_DATABASE_URL khi test chạy runServe/runUser thật.
	IdentityDSN string

	closeAll func()
}

// Close đóng mọi pool fixture đã mở (schema do pgtest dọn).
func (f *Fixture) Close() { f.closeAll() }

// repoRoot đi từ apps/viderse/internal/testidentity về gốc repo.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s: %v", root, err)
	}
	return root
}

// roleDSN thay user của adminDSN bằng viderse_identity — giống cách
// identity_role_test của Noverse dựng DSN per-role.
func roleDSN(t *testing.T, adminDSN string) string {
	t.Helper()
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	u.User = url.UserPassword("viderse_identity", rolePassword)
	return u.String()
}

// applyRoleScript chạy 02-identity-role.sql qua psql đúng như operator —
// grant rơi vào schema test. Khóa advisory giữ suốt lượt chạy để các test
// song song không race CREATE ROLE.
func applyRoleScript(t *testing.T, adminDSN, schema string) {
	t.Helper()
	bin, err := exec.LookPath("psql")
	if err != nil {
		t.Fatalf("psql là bắt buộc để áp identity role script: %v", err)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("kết nối admin giữ khoá: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_xact_lock($1, $2)`, int32(2801), int32(7)); err != nil {
		t.Fatalf("lấy khoá role script: %v", err)
	}

	script := filepath.Join(repoRoot(t), "apps", "noverse", "deploy", "postgres-init", "02-identity-role.sql")
	// libpq không nhận tham số search_path của pgx; script schema-qualify
	// mọi bảng qua biến identity_schema nên bỏ tham số đó khỏi DSN cho psql.
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN cho psql: %v", err)
	}
	query := u.Query()
	query.Del("search_path")
	u.RawQuery = query.Encode()
	cmd := exec.Command(bin, u.String(),
		"-v", "ON_ERROR_STOP=1",
		"-v", "identity_password="+rolePassword,
		"-v", "identity_schema="+schema,
		"-f", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("áp identity role script: %v\n%s", err, out)
	}
	if strings.Contains(string(out), rolePassword) {
		t.Fatalf("psql echo mật khẩu role:\n%s", out)
	}
	// Khoá xact giữ tới khi conn đóng (defer) → rollback; script đã tự commit
	// qua kết nối riêng của psql nên grant tồn tại sau hàm này.
}

// Provision dựng một kho tài khoản chung hoàn chỉnh trong schema test riêng:
// migration Noverse + grants viderse_identity. Admin store dùng DSN quản
// trị; Accounts dùng đúng role least-privilege của Viderse.
func Provision(t *testing.T) *Fixture {
	t.Helper()
	adminDSN := pgtest.Provision(t)
	root := repoRoot(t)
	ctx := context.Background()

	// Schema identity do Noverse sở hữu: áp đúng bộ migration của nó đọc từ
	// thư mục SQL (import package của app khác là cấm theo archtest).
	db, err := pgmigrate.OpenDB(adminDSN)
	if err != nil {
		t.Fatalf("mở migration DB: %v", err)
	}
	if _, err := pgmigrate.Up(ctx, db, os.DirFS(filepath.Join(root, "apps", "noverse", "internal", "persistence", "migrations"))); err != nil {
		db.Close()
		t.Fatalf("migrate identity schema: %v", err)
	}
	db.Close()

	schema := pgtest.SchemaOf(adminDSN)
	applyRoleScript(t, adminDSN, schema)
	identityDSN := roleDSN(t, adminDSN)

	var closers []func()
	fx := &Fixture{AdminDSN: adminDSN, IdentityDSN: identityDSN}
	fx.closeAll = func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	t.Cleanup(fx.Close)

	open := func(dsn string, projectAccess bool) *identity.Store {
		pool, err := identitypg.Open(ctx, identitypg.PoolConfig{DSN: dsn})
		if err != nil {
			t.Fatalf("mở identity pool: %v", err)
		}
		closers = append(closers, pool.Close)
		return identity.NewFromRepository(
			identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: projectAccess}),
			identity.Options{},
		)
	}
	fx.Admin = open(adminDSN, true)
	fx.Accounts = open(identityDSN, false)
	return fx
}

// CreateMember tạo một member trên kho chung; apps rỗng = không grant nào.
// Member thường đi qua store role-viderse (Apps để trống nghĩa là không
// grant — store phía Noverse sẽ mặc định cấp noverse); member cần
// must_change phải qua store quản trị vì viderse_identity không được INSERT
// temporary_passwords — và khi apps rỗng grant noverse mặc định được thu hồi
// lại để giữ đúng ý "không grant".
func (f *Fixture) CreateMember(t *testing.T, username, password string, mustChange bool, apps ...identity.App) {
	t.Helper()
	store := f.Accounts
	if mustChange {
		store = f.Admin
	}
	in := identity.NewUser{
		Username:           username,
		Name:               username,
		Role:               identity.RoleMember,
		Password:           password,
		MustChangePassword: mustChange,
		Apps:               apps,
	}
	if _, err := store.CreateUser(in); err != nil {
		t.Fatalf("tạo member %q: %v", username, err)
	}
	if mustChange && len(apps) == 0 {
		// Store quản trị tự cấp noverse khi Apps rỗng — thu hồi để fixture
		// đúng là "không grant nào".
		if err := f.Admin.OperatorRevokeAppGrant(username, identity.AppNoverse); err != nil {
			t.Fatalf("thu hồi grant mặc định của %q: %v", username, err)
		}
	}
}

// IssueTempPassword mô phỏng admin reset: đặt must_change + mật khẩu tạm một
// lần và thu hồi mọi phiên hiện có — đúng đường sản xuất (UserPatch của
// store quản trị, không ghi DB trực tiếp).
func (f *Fixture) IssueTempPassword(t *testing.T, username, tempPassword string) {
	t.Helper()
	mustChange := true
	if _, err := f.Admin.UpdateUser(username, identity.UserPatch{
		Password:           &tempPassword,
		MustChangePassword: &mustChange,
	}); err != nil {
		t.Fatalf("cấp mật khẩu tạm cho %q: %v", username, err)
	}
}

// MustChangeFlag đọc cờ bắt đổi mật khẩu trực tiếp từ DB bằng kết nối quản
// trị — dùng để chứng minh đổi mật khẩu đã clear flag bằng query giới hạn
// cột của role viderse_identity.
func (f *Fixture) MustChangeFlag(t *testing.T, username string) bool {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), f.AdminDSN)
	if err != nil {
		t.Fatalf("kết nối kiểm chứng: %v", err)
	}
	defer conn.Close(context.Background())
	var flag bool
	if err := conn.QueryRow(context.Background(),
		`SELECT must_change_password FROM users WHERE username = $1`, username).Scan(&flag); err != nil {
		t.Fatalf("đọc must_change_password: %v", err)
	}
	return flag
}

// SessionCount đếm phiên còn hiệu lực của user (kiểm chứng revoke).
func (f *Fixture) SessionCount(t *testing.T, username string) int {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), f.AdminDSN)
	if err != nil {
		t.Fatalf("kết nối kiểm chứng: %v", err)
	}
	defer conn.Close(context.Background())
	var n int
	if err := conn.QueryRow(context.Background(), `
		SELECT count(*) FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE u.username = $1 AND s.revoked_at IS NULL`, username).Scan(&n); err != nil {
		t.Fatalf("đếm session: %v", err)
	}
	return n
}

// DSNForEnv trả bản đồ env cần cho serve/CLI của Viderse trỏ vào fixture.
func (f *Fixture) DSNForEnv() map[string]string {
	return map[string]string{"VERSE_IDENTITY_DATABASE_URL": f.IdentityDSN}
}

// OpenViderseStore mở thêm một kho role viderse_identity độc lập trên cùng
// fixture — mô phỏng một tiến trình app khác cùng kho. Caller tự đóng.
func (f *Fixture) OpenViderseStore(t *testing.T) (*identity.Store, func()) {
	t.Helper()
	pool, err := identitypg.Open(context.Background(), identitypg.PoolConfig{DSN: f.IdentityDSN})
	if err != nil {
		t.Fatalf("mở identity pool thứ hai: %v", err)
	}
	return identity.NewFromRepository(
		identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: false}),
		identity.Options{},
	), pool.Close
}

// DebugDump trả các dòng tài khoản/tạm để chẩn đoán (test-only).
func (f *Fixture) DebugDump(t *testing.T, username string) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), f.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(), `
		SELECT u.username, u.must_change_password, u.status,
		       tp.password_hash IS NOT NULL, tp.used_at, tp.revoked_at, tp.expires_at,
		       uc.password_hash IS NOT NULL
		FROM users u
		LEFT JOIN temporary_passwords tp ON tp.user_id = u.id
		LEFT JOIN user_credentials uc ON uc.user_id = u.id
		WHERE u.username = $1`, username)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var un, status string
		var mc, tpPresent, ucPresent bool
		var used, revoked, exp any
		if err := rows.Scan(&un, &mc, &status, &tpPresent, &used, &revoked, &exp, &ucPresent); err != nil {
			t.Fatal(err)
		}
		t.Logf("user=%s must_change=%v status=%s temp_present=%v used=%v revoked=%v exp=%v cred_present=%v",
			un, mc, status, tpPresent, used, revoked, exp, ucPresent)
	}
}
