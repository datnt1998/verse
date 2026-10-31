package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
	"github.com/datnt1998/verse/platform/pgtest"
)

const (
	ssoFixtureDir     = "../../internal/storageimport/testdata/sso"
	ssoFixtureNoverse = ssoFixtureDir + "/noverse-accounts.json"
	ssoFixtureViderse = ssoFixtureDir + "/viderse-accounts.json"
	ssoFixtureCollide = ssoFixtureDir + "/collision-viderse.json"
	ssoLibraryRoot    = "../../internal/store/testdata/storage-contract/library"
	ssoNoverseID      = "6f1d0f9e-3f2a-4c88-9d0e-1b1a2b3c4d5e"
	ssoViderseID      = "7a2e1c4d-5b6f-4a7b-8c9d-0e1f2a3b4c5d"
)

func absPath(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// migrateTestDSN provisions a migrated schema; the command itself opens the
// pool from MIGRATION_DATABASE_URL.
func migrateTestDSN(t *testing.T) string {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	db.Close()
	return dsn
}

func cutoverEnv(t *testing.T) string {
	t.Helper()
	dsn := migrateTestDSN(t)
	t.Setenv("MIGRATION_DATABASE_URL", dsn)
	t.Setenv("STORE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BLOB_LOCAL_ROOT", t.TempDir())
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("STORAGE_ENV", "")
	return dsn
}

type ssoArgs struct {
	store, novFile, vidFile, root, ownerMap, novID, vidID string
	manifest, report, renameMap                           string
	dryRun, apply, backupOnly                             bool
	acceptManifest, backupDir, acceptAdmins               string
	vidEmpty                                              bool
}

func (a ssoArgs) argv() []string {
	var args []string
	push := func(name, val string) {
		if val != "" {
			args = append(args, name, val)
		}
	}
	push("--noverse-store", a.store)
	push("--noverse-accounts", a.novFile)
	push("--root", a.root)
	push("--owner-map", a.ownerMap)
	push("--noverse-source-id", a.novID)
	if a.vidEmpty {
		args = append(args, "--viderse-empty")
	} else {
		push("--viderse-accounts", a.vidFile)
		push("--viderse-source-id", a.vidID)
	}
	push("--manifest", a.manifest)
	push("--report", a.report)
	push("--rename-map", a.renameMap)
	if a.dryRun {
		args = append(args, "--dry-run")
	}
	if a.apply {
		args = append(args, "--apply")
	}
	if a.backupOnly {
		args = append(args, "--backup-only")
	}
	push("--accept-manifest", a.acceptManifest)
	push("--backup-dir", a.backupDir)
	push("--accept-global-admins", a.acceptAdmins)
	return args
}

func manifestDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc manifest: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readReport(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc report: %v", err)
	}
	var r map[string]any
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("report hỏng: %v", err)
	}
	return r
}

func TestSSOImportCLIRequiresAbsolutePathsAndModes(t *testing.T) {
	base := ssoArgs{store: "postgres", vidFile: "x.json", vidID: ssoViderseID}
	if err := runStorageSSOImport(base.argv()); err == nil {
		t.Fatal("thiếu store phải lỗi trước — ở đây store postgres có, manifest/report thiếu")
	}
	// Relative source path refused before anything else.
	base.manifest, base.report = "rel-manifest.json", "rel-report.json"
	err := runStorageSSOImport(base.argv())
	if err == nil || !strings.Contains(err.Error(), "tuyệt đối") {
		t.Fatalf("đường tương đối phải lỗi: %v", err)
	}
	tmp := t.TempDir()
	base.manifest, base.report = filepath.Join(tmp, "m.json"), filepath.Join(tmp, "r.json")
	base.vidFile = "relative.json"
	err = runStorageSSOImport(base.argv())
	if err == nil || !strings.Contains(err.Error(), "tuyệt đối") {
		t.Fatalf("source tương đối phải lỗi: %v", err)
	}
	// apply without approvals/backup-dir.
	base.vidFile = absPath(t, ssoFixtureViderse)
	err = runStorageSSOImport(ssoArgs{
		store: "postgres", vidFile: base.vidFile, vidID: ssoViderseID,
		manifest: base.manifest, report: base.report, apply: true,
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "accept-manifest") {
		t.Fatalf("apply thiếu accept-manifest phải lỗi: %v", err)
	}
	err = runStorageSSOImport(ssoArgs{
		store: "postgres", vidFile: base.vidFile, vidID: ssoViderseID,
		manifest: base.manifest, report: base.report, apply: true,
		acceptManifest: strings.Repeat("a", 64),
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "backup-dir") {
		t.Fatalf("apply thiếu backup-dir phải lỗi: %v", err)
	}
	// postgres mode refuses file-mode flags.
	err = runStorageSSOImport(ssoArgs{
		store: "postgres", novFile: absPath(t, ssoFixtureNoverse), novID: ssoNoverseID,
		vidFile: base.vidFile, vidID: ssoViderseID,
		manifest: base.manifest, report: base.report,
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("store=postgres nhận file phải lỗi: %v", err)
	}
	// Mutually exclusive modes.
	err = runStorageSSOImport(ssoArgs{
		store: "postgres", vidFile: base.vidFile, vidID: ssoViderseID,
		manifest: base.manifest, report: base.report, dryRun: true, apply: true,
	}.argv())
	if err == nil {
		t.Fatal("dry-run + apply cùng lúc phải lỗi")
	}
}

func TestSSOImportCLIRequiresMigrationDSN(t *testing.T) {
	t.Setenv("MIGRATION_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://app:app@127.0.0.1:5432/app") // must NOT be used
	t.Setenv("STORE_DRIVER", "postgres")
	err := runStorageSSOImport(ssoArgs{
		store: "postgres", vidFile: absPath(t, ssoFixtureViderse), vidID: ssoViderseID,
		manifest: filepath.Join(t.TempDir(), "m.json"), report: filepath.Join(t.TempDir(), "r.json"),
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "MIGRATION_DATABASE_URL") {
		t.Fatalf("thiếu MIGRATION_DATABASE_URL phải lỗi, không fallback app role: %v", err)
	}
}

func TestSSOImportDryRunReportsCollisionWithoutWrites(t *testing.T) {
	cutoverEnv(t)
	tmp := t.TempDir()
	novCopy := filepath.Join(tmp, "web-accounts.json")
	data, _ := os.ReadFile(ssoFixtureNoverse)
	if err := os.WriteFile(novCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	vidCopy := filepath.Join(tmp, "viderse-accounts.json")
	data, _ = os.ReadFile(ssoFixtureCollide)
	if err := os.WriteFile(vidCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(tmp, "sso-manifest.json")
	report := filepath.Join(tmp, "sso-report.json")
	err := runStorageSSOImport(ssoArgs{
		store: "file", novFile: novCopy, novID: ssoNoverseID,
		root: absPath(t, ssoLibraryRoot), ownerMap: "du-an=tac-gia,_=quan-tri",
		vidFile: vidCopy, vidID: ssoViderseID,
		manifest: manifest, report: report, dryRun: true,
	}.argv())
	if err == nil {
		t.Fatal("collision phải thoát khác 0")
	}
	rep := readReport(t, report)
	if rep["status"] != "preflight_failed" {
		t.Fatalf("status=%v", rep["status"])
	}
	collisions, _ := rep["collisions"].([]any)
	if len(collisions) == 0 {
		t.Fatal("report thiếu collision")
	}
	// dry-run writes nothing to source dirs, no backup, no DB rows.
	if _, err := os.Stat(novCopy + ".v4.bak"); !os.IsNotExist(err) {
		t.Fatal("dry-run không được tạo .v4.bak")
	}
	if _, err := os.Stat(vidCopy + ".v4.bak"); !os.IsNotExist(err) {
		t.Fatal("dry-run không được tạo .v4.bak")
	}
	st, _ := os.Stat(report)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("report mode %o", st.Mode().Perm())
	}
	// Report contains no credential material.
	if strings.Contains(string(mustRead(t, report)), "$argon2id$") {
		t.Fatal("report chứa hash")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSSOAdminPromotionRequiresExplicitApproval(t *testing.T) {
	dsn := cutoverEnv(t)
	tmp := t.TempDir()
	vidCopy := filepath.Join(tmp, "viderse-accounts.json")
	data, _ := os.ReadFile(ssoFixtureViderse)
	if err := os.WriteFile(vidCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(tmp, "sso-manifest.json")
	report := filepath.Join(tmp, "sso-report.json")
	backupDir := filepath.Join(tmp, "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Only files that are not a database dump: what the backup-only mode and
	// the dry-run leave around must not pass as the pg_dump evidence.
	if err := os.WriteFile(filepath.Join(backupDir, "sso-backup-inventory-20260101T000000Z.json"), []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "before-sso.dump"), []byte("not a dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := ssoArgs{
		store: "postgres", vidFile: vidCopy, vidID: ssoViderseID,
		manifest: manifest, report: report, dryRun: true,
	}
	if err := runStorageSSOImport(base.argv()); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	rep := readReport(t, report)
	adminDigest, _ := rep["globalAdminsDigest"].(string)
	if adminDigest == "" {
		t.Fatal("report phải có globalAdminsDigest")
	}
	if admins, _ := rep["globalAdmins"].([]any); len(admins) == 0 {
		t.Fatal("report phải liệt kê admin sau cutover")
	}
	mSHA := manifestDigest(t, manifest)
	applyArgs := base
	applyArgs.dryRun, applyArgs.apply = false, true
	applyArgs.acceptManifest = mSHA
	applyArgs.backupDir = backupDir

	// Without the admin digest: refuse before writing anything.
	err := runStorageSSOImport(applyArgs.argv())
	if err == nil || !strings.Contains(err.Error(), "accept-global-admins") {
		t.Fatalf("thiếu duyệt admin phải lỗi: %v", err)
	}
	if _, err := os.Stat(vidCopy + ".v4.bak"); !os.IsNotExist(err) {
		t.Fatal("refusal phải trước cả backup")
	}
	// Wrong digest also refuses.
	applyArgs.acceptAdmins = strings.Repeat("0", 64)
	err = runStorageSSOImport(applyArgs.argv())
	if err == nil || regexp.MustCompile(`[0-9a-fA-F]{64}`).MatchString(err.Error()) {
		t.Fatalf("digest sai phải lỗi không lộ digest: %v", err)
	}
	mismatch := readReport(t, report)
	if mismatch["status"] != "approval_mismatch" || mismatch["globalAdminsDigest"] != adminDigest ||
		mismatch["manifestSha256"] != mSHA {
		t.Fatalf("report phải chứa thông tin duyệt hiện tại: %+v", mismatch)
	}
	// Approved digests still refuse while nothing in the dir verifies as a
	// dump of THIS database (a magic-only stub is not an archive).
	applyArgs.acceptAdmins = adminDigest
	if err := runStorageSSOImport(applyArgs.argv()); err == nil {
		t.Fatal("apply without a verified dump must refuse")
	}
	if _, err := os.Stat(vidCopy + ".v4.bak"); !os.IsNotExist(err) {
		t.Fatal("the missing-dump refusal must come before any source backup")
	}
	pgDumpSchema(t, dsn, filepath.Join(backupDir, "before-sso.dump"))
	// Approved digest applies and grants platform admin.
	if err := runStorageSSOImport(applyArgs.argv()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var role string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT global_role FROM users WHERE username='vid-admin'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Fatalf("vid-admin phải là admin nền tảng sau approve, có %s", role)
	}
	var app string
	if err := pool.Pool().QueryRow(context.Background(), `
SELECT g.app FROM app_grants g JOIN users u ON u.id=g.user_id WHERE u.username='vid-member'`).Scan(&app); err != nil {
		t.Fatal(err)
	}
	if app != "viderse" {
		t.Fatalf("grant sai app: %s", app)
	}
}

// TestSSOImportFileMode runs the complete cutover: strict identity phase then
// the full library import under the same source id, in one command.
func TestSSOImportFileModeIdentityThenLibrary(t *testing.T) {
	dsn := cutoverEnv(t)
	tmp := t.TempDir()
	novCopy := filepath.Join(tmp, "web-accounts.json")
	data, _ := os.ReadFile(ssoFixtureNoverse)
	if err := os.WriteFile(novCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	vidCopy := filepath.Join(tmp, "viderse-accounts.json")
	data, _ = os.ReadFile(ssoFixtureViderse)
	if err := os.WriteFile(vidCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(tmp, "sso-manifest.json")
	report := filepath.Join(tmp, "sso-report.json")
	backupDir := filepath.Join(tmp, "backup")
	os.MkdirAll(backupDir, 0o700)
	pgDumpSchema(t, dsn, filepath.Join(backupDir, "before-sso.dump"))

	args := ssoArgs{
		store: "file", novFile: novCopy, novID: ssoNoverseID,
		root: absPath(t, ssoLibraryRoot), ownerMap: "du-an=tac-gia,_=quan-tri",
		vidFile: vidCopy, vidID: ssoViderseID,
		manifest: manifest, report: report, dryRun: true,
	}
	if err := runStorageSSOImport(args.argv()); err != nil {
		t.Fatalf("dry-run file mode: %v", err)
	}
	rep := readReport(t, report)
	if rep["status"] != "planned" {
		t.Fatalf("status=%v collisions=%v", rep["status"], rep["collisions"])
	}
	if _, err := os.Stat(manifest + ".library.json"); err != nil {
		t.Fatalf("thiếu library manifest: %v", err)
	}
	args.dryRun, args.apply = false, true
	args.acceptManifest = manifestDigest(t, manifest)
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	args.backupDir, err = filepath.Rel(currentDir, backupDir)
	if err != nil {
		t.Fatal(err)
	}
	args.acceptAdmins, _ = rep["globalAdminsDigest"].(string)
	if err := runStorageSSOImport(args.argv()); err != nil {
		t.Fatalf("apply file mode: %v", err)
	}
	rep = readReport(t, report)
	if rep["status"] != "complete" {
		t.Fatalf("status=%v", rep["status"])
	}
	firstInventory, ok := rep["backupInventoryPath"].(string)
	if !ok || !filepath.IsAbs(firstInventory) || !ssoInventoryName.MatchString(filepath.Base(firstInventory)) {
		t.Fatalf("report phải chỉ đúng inventory mới: %v", rep["backupInventoryPath"])
	}
	firstInventoryBytes := mustRead(t, firstInventory)
	databaseBackup, ok := rep["databaseBackup"].(map[string]any)
	if !ok {
		t.Fatalf("report thiếu verified database backup: %v", rep)
	}
	restorePath, _ := databaseBackup["pgRestorePath"].(string)
	restoreVersion, _ := databaseBackup["pgRestore"].(string)
	if !filepath.IsAbs(restorePath) || restoreVersion == "" {
		t.Fatalf("receipt cần executable tuyệt đối và version pg_restore: %v", databaseBackup)
	}
	if _, err := os.Stat(novCopy + ".v4.bak"); err != nil {
		t.Fatalf("thiếu .v4.bak: %v", err)
	}
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	assertCommittedBackup := func(report map[string]any, inventory string, dump map[string]any) string {
		t.Helper()
		identity, ok := report["identity"].(map[string]any)
		if !ok {
			t.Fatalf("report thiếu identity đã commit: %v", report["identity"])
		}
		runID, _ := identity["cutoverRunId"].(string)
		var recordedInventory, recordedPath, recordedSHA string
		if err := pool.Pool().QueryRow(context.Background(), `
SELECT summary->'cutoverBackup'->>'inventoryPath',
       summary->'cutoverBackup'->>'dumpPath',
       summary->'cutoverBackup'->>'dumpSha256'
FROM import_runs WHERE id=$1 AND source_label='sso-cutover-identity' AND status='verified'`,
			runID).Scan(&recordedInventory, &recordedPath, &recordedSHA); err != nil {
			t.Fatalf("identity.cutoverRunId phải chỉ đúng transaction đã commit: %v", err)
		}
		if recordedInventory != inventory || recordedPath != dump["path"] || recordedSHA != dump["sha256"] {
			t.Fatalf("bản ghi commit không khớp inventory/dump: %s %s %s", recordedInventory, recordedPath, recordedSHA)
		}
		return runID
	}
	firstRunID := assertCommittedBackup(rep, firstInventory, databaseBackup)
	var users int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 9 {
		t.Fatalf("users=%d muốn 9", users)
	}
	var stories int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM stories`).Scan(&stories); err != nil {
		t.Fatal(err)
	}
	if stories == 0 {
		t.Fatal("thư viện chưa nhập stories — identity-only cutover là sai")
	}
	var members int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM project_members WHERE removed_at IS NULL`).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members == 0 {
		t.Fatal("project_members rỗng")
	}
	// Resume: same apply again inserts nothing.
	pgDumpSchema(t, dsn, filepath.Join(backupDir, "before-resume.dump"))
	if err := runStorageSSOImport(args.argv()); err != nil {
		t.Fatalf("resume apply: %v", err)
	}
	resumed := readReport(t, report)
	secondInventory, ok := resumed["backupInventoryPath"].(string)
	if !ok || secondInventory == firstInventory || !ssoInventoryName.MatchString(filepath.Base(secondInventory)) {
		t.Fatalf("hai apply phải để lại hai inventory riêng: %q, %v", firstInventory, resumed["backupInventoryPath"])
	}
	if !equalBytes(firstInventoryBytes, mustRead(t, firstInventory)) {
		t.Fatal("resume ghi đè inventory đầu")
	}
	secondDump, ok := resumed["databaseBackup"].(map[string]any)
	if !ok || secondDump["path"] != filepath.Join(backupDir, "before-resume.dump") ||
		secondDump["sha256"] == databaseBackup["sha256"] {
		t.Fatalf("resume phải dùng dump mới sau khi thống kê live thay đổi: %v", resumed["databaseBackup"])
	}
	if secondRunID := assertCommittedBackup(resumed, secondInventory, secondDump); secondRunID == firstRunID {
		t.Fatal("resume phải ghi transaction mới, không ghi đè bản ghi commit cũ")
	}
	assertCommittedBackup(rep, firstInventory, databaseBackup)
	for _, path := range []string{firstInventory, secondInventory} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("inventory phải tồn tại và có mode 0600: %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(backupDir, "sso-backup-inventory.json")); !os.IsNotExist(err) {
		t.Fatalf("không được tạo inventory alias tĩnh: %v", err)
	}
	var users2 int
	pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&users2)
	if users2 != users {
		t.Fatalf("resume nhập lại: %d→%d", users, users2)
	}
	// Report secrets check end-to-end.
	if strings.Contains(string(mustRead(t, report)), "$argon2id$") ||
		strings.Contains(string(mustRead(t, manifest)), "$argon2id$") {
		t.Fatal("report/manifest chứa hash")
	}
}

// pgDumpSchema writes a real pg_dump custom archive of the provisioned test
// schema — the only kind of backup evidence apply accepts.
func pgDumpSchema(t *testing.T, provisionedDSN, dest string) {
	t.Helper()
	for _, binary := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Fatalf("%s không có trên PATH: %v", binary, err)
		}
	}
	dsn := pgtest.RequireURL(t)
	if i := strings.Index(dsn, "?"); i >= 0 {
		dsn = dsn[:i]
	}
	out, err := exec.Command("pg_dump", "--format=custom",
		"--schema="+pgtest.SchemaOf(provisionedDSN), "--file="+dest, dsn).CombinedOutput()
	if err != nil {
		t.Fatalf("pg_dump: %v\n%s", err, out)
	}
	if err := os.Chmod(dest, 0o600); err != nil {
		t.Fatalf("chmod archive 0600: %v", err)
	}
}

func TestSSOImportRejectsEqualSourceIDs(t *testing.T) {
	err := runStorageSSOImport(ssoArgs{
		store:   "file",
		novFile: absPath(t, ssoFixtureNoverse), novID: ssoNoverseID,
		root:    absPath(t, ssoLibraryRoot),
		vidFile: absPath(t, ssoFixtureViderse), vidID: ssoNoverseID, // same UUID
		manifest: filepath.Join(t.TempDir(), "m.json"),
		report:   filepath.Join(t.TempDir(), "r.json"),
		dryRun:   true,
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "khác nhau") {
		t.Fatalf("source id trùng phải bị từ chối: %v", err)
	}
}

func TestSSOImportRefusesOutputOverSource(t *testing.T) {
	tmp := t.TempDir()
	novCopy := filepath.Join(tmp, "web-accounts.json")
	data, _ := os.ReadFile(ssoFixtureNoverse)
	if err := os.WriteFile(novCopy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, novCopy)
	// --report equal to a source path refuses before opening the DB or
	// writing anything.
	err := runStorageSSOImport(ssoArgs{
		store: "file", novFile: novCopy, novID: ssoNoverseID,
		root:    absPath(t, ssoLibraryRoot),
		vidFile: absPath(t, ssoFixtureViderse), vidID: ssoViderseID,
		manifest: filepath.Join(tmp, "m.json"), report: novCopy, dryRun: true,
	}.argv())
	if err == nil {
		t.Fatal("report trùng file nguồn phải bị từ chối")
	}
	if got := mustRead(t, novCopy); len(got) != len(before) {
		t.Fatal("nguồn bị ghi đè — cấm tuyệt đối")
	}
	// Same for the other outputs and for paths inside --root.
	err = runStorageSSOImport(ssoArgs{
		store: "file", novFile: novCopy, novID: ssoNoverseID,
		root:    absPath(t, ssoLibraryRoot),
		vidFile: absPath(t, ssoFixtureViderse), vidID: ssoViderseID,
		manifest: novCopy + ".v4.bak", report: filepath.Join(tmp, "r.json"), dryRun: true,
	}.argv())
	if err == nil {
		t.Fatal("manifest trùng .v4.bak phải bị từ chối")
	}
	root := absPath(t, ssoLibraryRoot)
	err = runStorageSSOImport(ssoArgs{
		store: "file", novFile: novCopy, novID: ssoNoverseID,
		root:    root,
		vidFile: absPath(t, ssoFixtureViderse), vidID: ssoViderseID,
		manifest: filepath.Join(root, "m.json"), report: filepath.Join(tmp, "r.json"), dryRun: true,
	}.argv())
	if err == nil || !strings.Contains(err.Error(), "--root") {
		t.Fatalf("manifest trong --root phải bị từ chối: %v", err)
	}
}

// TestSSOImportReadsNoverseAccountsOnce: the library scan and the identity
// decode must consume one snapshot of the accounts file. The path is a FIFO
// that yields its content exactly once — a second read could only block or
// see other bytes — so the dry-run must finish and report the same digest for
// the identity source and the library manifest's accounts entry.
func TestSSOImportReadsNoverseAccountsOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO cần unix")
	}
	cutoverEnv(t)
	data, err := os.ReadFile(ssoFixtureNoverse)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	fifo := filepath.Join(t.TempDir(), "web-accounts.json")
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	// One writer, one buffer. Cleanup opens a nonblocking reader so a
	// writer still waiting for its first reader (run refused early) exits.
	go func() {
		f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		f.Write(data)
		f.Close()
	}()
	t.Cleanup(func() {
		if r, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			r.Close()
		}
	})
	tmp := t.TempDir()
	manifest := filepath.Join(tmp, "sso-manifest.json")
	report := filepath.Join(tmp, "sso-report.json")
	argv := ssoArgs{
		store: "file", novFile: fifo, novID: ssoNoverseID,
		root: absPath(t, ssoLibraryRoot), ownerMap: "du-an=tac-gia,_=quan-tri",
		vidFile: absPath(t, ssoFixtureViderse), vidID: ssoViderseID,
		manifest: manifest, report: report, dryRun: true,
	}.argv()
	done := make(chan error, 1)
	go func() { done <- runStorageSSOImport(argv) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("dry-run over a read-once accounts file: %v", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("dry-run blocked reading the accounts file a second time")
	}

	var rep struct {
		Sources []storageimport.SourceReport `json:"sources"`
	}
	readJSON(t, report, &rep)
	var identitySHA string
	for _, s := range rep.Sources {
		if s.App == storageimport.SourceNoverse {
			identitySHA = s.SHA256
		}
	}
	var lib struct {
		Files []storageimport.ManifestFile `json:"files"`
	}
	readJSON(t, manifest+".library.json", &lib)
	var librarySHA string
	for _, f := range lib.Files {
		if f.Path == storageimport.PathAccounts {
			librarySHA = f.SHA256
		}
	}
	if identitySHA != want || librarySHA != want {
		t.Fatalf("accounts digest identity=%q library=%q, want both %q", identitySHA, librarySHA, want)
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// TestSSOApplyRequiresAdminDigestWithoutViderseAdmins: the approval gate is
// unconditional — a cutover where Viderse contributes no admin still demands
// --accept-global-admins over the full list (existing DB admins included).
func TestSSOApplyRequiresAdminDigestWithoutViderseAdmins(t *testing.T) {
	dsn := cutoverEnv(t)
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Pool().Exec(context.Background(), `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('00000000-0000-0000-0000-0000000000aa', 'admin-cu', 'Admin cũ', 'admin', 'active')`); err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	vidCopy := filepath.Join(tmp, "viderse-accounts.json")
	member := map[string]any{
		"username": "vid-member", "name": "Vid Member", "role": "member",
		"projects": map[string]string{}, "disabled": false,
		"passwordHash":       "$argon2id$v=19$m=65536,t=3,p=4$cnVubmluZ3NhbHQy$cnVubmluZ2hhc2gy",
		"onboardedAt":        "2026-02-02T00:00:00Z",
		"mustChangePassword": false,
		"createdAt":          "2026-02-02T00:00:00Z",
		"updatedAt":          "2026-02-02T00:00:00Z",
	}
	vidData, err := json.Marshal(map[string]any{
		"version": 4, "users": []any{member}, "sessions": []any{}, "pairings": []any{}, "audit": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vidCopy, vidData, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(tmp, "sso-manifest.json")
	report := filepath.Join(tmp, "sso-report.json")
	backupDir := filepath.Join(tmp, "backup")
	os.MkdirAll(backupDir, 0o700)
	pgDumpSchema(t, dsn, filepath.Join(backupDir, "before-sso.dump"))

	base := ssoArgs{
		store: "postgres", vidFile: vidCopy, vidID: ssoViderseID,
		manifest: manifest, report: report, dryRun: true,
	}
	if err := runStorageSSOImport(base.argv()); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	rep := readReport(t, report)
	adminDigest, _ := rep["globalAdminsDigest"].(string)
	if adminDigest == "" {
		t.Fatal("globalAdminsDigest phải luôn tồn tại")
	}
	admins, _ := rep["globalAdmins"].([]any)
	if len(admins) != 1 {
		t.Fatalf("chỉ admin DB hiện có được liệt kê: %v", rep["globalAdmins"])
	}
	applyArgs := base
	applyArgs.dryRun, applyArgs.apply = false, true
	applyArgs.acceptManifest = manifestDigest(t, manifest)
	applyArgs.backupDir = backupDir
	// No flag → refuse; wrong digest → refuse; even though Viderse has no
	// admin to approve.
	if err := runStorageSSOImport(applyArgs.argv()); err == nil ||
		!strings.Contains(err.Error(), "accept-global-admins") {
		t.Fatalf("apply thiếu duyệt admin phải lỗi: %v", err)
	}
	applyArgs.acceptAdmins = strings.Repeat("0", 64)
	if err := runStorageSSOImport(applyArgs.argv()); err == nil {
		t.Fatal("digest sai phải lỗi")
	}
	applyArgs.acceptAdmins = adminDigest
	if err := runStorageSSOImport(applyArgs.argv()); err != nil {
		t.Fatalf("apply với digest đúng: %v", err)
	}
}

func TestSSOApprovalMismatchWritesCurrentReportOnly(t *testing.T) {
	dsn := cutoverEnv(t)
	dir := t.TempDir()
	vid := filepath.Join(dir, "viderse.json")
	if err := os.WriteFile(vid, mustRead(t, ssoFixtureViderse), 0o600); err != nil {
		t.Fatal(err)
	}
	args := ssoArgs{
		store: "postgres", vidFile: vid, vidID: ssoViderseID,
		manifest: filepath.Join(dir, "manifest.json"), report: filepath.Join(dir, "report.json"), dryRun: true,
	}
	if err := runStorageSSOImport(args.argv()); err != nil {
		t.Fatal(err)
	}
	oldManifest := mustRead(t, args.manifest)
	oldReport := readReport(t, args.report)
	args.acceptManifest = manifestDigest(t, args.manifest)
	args.acceptAdmins = oldReport["globalAdminsDigest"].(string)
	args.dryRun, args.apply = false, true
	args.backupDir = t.TempDir()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Pool().Exec(context.Background(), `
INSERT INTO users (id,username,display_name,global_role,status)
VALUES ('00000000-0000-0000-0000-0000000000aa','admin-moi','Admin mới','admin','active')`); err != nil {
		t.Fatal(err)
	}
	for _, what := range []string{"admins", "manifest"} {
		t.Run(what, func(t *testing.T) {
			call := args
			if what == "manifest" {
				call.acceptManifest = strings.Repeat("0", 64)
			}
			err := runStorageSSOImport(call.argv())
			if err == nil || regexp.MustCompile(`[0-9a-fA-F]{64}`).MatchString(err.Error()) ||
				!strings.Contains(err.Error(), args.report) || !strings.Contains(err.Error(), args.manifest) ||
				!strings.Contains(err.Error(), "chạy lại --dry-run") ||
				!strings.Contains(err.Error(), "report mới") || !strings.Contains(err.Error(), "VÀ manifest mới") ||
				strings.Contains(err.Error(), "rồi duyệt lại digest mới") {
				t.Fatalf("lệch duyệt phải yêu cầu dry-run và review cả report/manifest mới, không duyệt từ report lệch: %v", err)
			}
			report := readReport(t, args.report)
			if report["status"] != "approval_mismatch" || report["globalAdminsDigest"] == args.acceptAdmins ||
				report["manifestSha256"] != args.acceptManifest || len(report["globalAdmins"].([]any)) != 2 {
				t.Fatalf("report phải có manifest và admin hiện tại: %+v", report)
			}
			if !equalBytes(mustRead(t, args.manifest), oldManifest) {
				t.Fatal("lệch duyệt không được sửa manifest")
			}
			if _, err := os.Stat(vid + ".v4.bak"); !os.IsNotExist(err) {
				t.Fatalf("lệch duyệt không được tạo backup: %v", err)
			}
			entries, err := os.ReadDir(args.backupDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("lệch duyệt không được ghi inventory: %v, %v", entries, err)
			}
		})
	}
	var users, runs int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM import_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if users != 1 || runs != 0 {
		t.Fatalf("lệch duyệt phải không ghi DB: %d users, %d runs", users, runs)
	}
}

func TestSSOOutputPathsRefuseUnicodeAliasesHardlinksAndArchives(t *testing.T) {
	dsn := cutoverEnv(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "tài-khoản.json")
	data := mustRead(t, ssoFixtureViderse)
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "hardlink.json")
	if err := os.Link(source, link); err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(dir, "before.dump")
	pgDumpSchema(t, dsn, dump)
	dumpBefore := mustRead(t, dump)
	for name, output := range map[string]string{
		"unicode":  filepath.Join(dir, "ta\u0300i-khoa\u0309n.json"),
		"hardlink": link,
		"archive":  dump,
	} {
		t.Run(name, func(t *testing.T) {
			err := runStorageSSOImport(ssoArgs{
				store: "postgres", vidFile: source, vidID: ssoViderseID,
				manifest: filepath.Join(dir, "manifest.json"), report: output, dryRun: true,
			}.argv())
			if err == nil || (!strings.Contains(err.Error(), "trùng input") && !strings.Contains(err.Error(), "archive pg_dump")) {
				t.Fatalf("output nguy hiểm phải dừng trước DB: %v", err)
			}
		})
	}
	if !equalBytes(mustRead(t, source), data) || !equalBytes(mustRead(t, dump), dumpBefore) {
		t.Fatal("nguồn/backup bị sửa")
	}
}

func TestSSOApplyMissingApprovedManifestWritesNothing(t *testing.T) {
	dsn := cutoverEnv(t)
	for _, keepReport := range []bool{true, false} {
		name := "report-missing"
		if keepReport {
			name = "report-existing"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			nov := filepath.Join(dir, "noverse.json")
			vid := filepath.Join(dir, "viderse.json")
			for path, fixture := range map[string]string{nov: ssoFixtureNoverse, vid: ssoFixtureViderse} {
				if err := os.WriteFile(path, mustRead(t, fixture), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := ssoArgs{
				store: "file", novFile: nov, novID: ssoNoverseID,
				root: absPath(t, ssoLibraryRoot), ownerMap: "du-an=tac-gia,_=quan-tri",
				vidFile: vid, vidID: ssoViderseID,
				manifest: filepath.Join(dir, "manifest.json"), report: filepath.Join(dir, "report.json"), dryRun: true,
			}
			if err := runStorageSSOImport(args.argv()); err != nil {
				t.Fatalf("dry-run: %v", err)
			}
			approvedReport := readReport(t, args.report)
			args.acceptManifest = manifestDigest(t, args.manifest)
			args.acceptAdmins = approvedReport["globalAdminsDigest"].(string)
			args.dryRun, args.apply = false, true
			args.backupDir = t.TempDir()
			libraryBefore := mustRead(t, args.manifest+".library.json")
			reportBefore := mustRead(t, args.report)
			reportInfo, err := os.Stat(args.report)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(args.manifest); err != nil {
				t.Fatal(err)
			}
			if !keepReport {
				if err := os.Remove(args.report); err != nil {
					t.Fatal(err)
				}
			}
			err = runStorageSSOImport(args.argv())
			if err == nil || !strings.Contains(err.Error(), "manifest đã duyệt") ||
				!strings.Contains(err.Error(), "--dry-run") {
				t.Fatalf("manifest đã xóa phải từ chối rõ ràng: %v", err)
			}
			if _, err := os.Stat(args.manifest); !os.IsNotExist(err) {
				t.Fatalf("apply không được tái tạo manifest: %v", err)
			}
			if keepReport {
				info, err := os.Stat(args.report)
				if err != nil || !info.ModTime().Equal(reportInfo.ModTime()) ||
					!equalBytes(mustRead(t, args.report), reportBefore) {
					t.Fatalf("thiếu manifest không được viết report lệch duyệt: %v", err)
				}
			} else if _, err := os.Stat(args.report); !os.IsNotExist(err) {
				t.Fatalf("thiếu manifest không được tạo report: %v", err)
			}
			if !equalBytes(mustRead(t, args.manifest+".library.json"), libraryBefore) {
				t.Fatal("thiếu manifest không được sửa library manifest")
			}
			for _, source := range []string{nov, vid} {
				for _, suffix := range []string{".lock", ".v4.bak"} {
					if _, err := os.Stat(source + suffix); !os.IsNotExist(err) {
						t.Fatalf("không được tạo %s%s: %v", source, suffix, err)
					}
				}
			}
			entries, err := os.ReadDir(args.backupDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("thiếu manifest không được ghi inventory: %v, %v", entries, err)
			}
		})
	}
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var rows int
	if err := pool.Pool().QueryRow(context.Background(), `
SELECT (SELECT count(*) FROM users) + (SELECT count(*) FROM app_grants) +
       (SELECT count(*) FROM import_runs) + (SELECT count(*) FROM import_items)`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("thiếu manifest không được ghi DB: %d rows", rows)
	}
}

func TestSSOBackupInventoryExclusiveNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeDir, err := filepath.Rel(currentDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 31, 6, 14, 0, 0, time.FixedZone("offset", 7*60*60))
	var previous string
	for i, suffix := range []string{"", "-1", "-2"} {
		path, err := writeSSOBackupInventory(relativeDir, map[string]int{"attempt": i}, now)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dir, "sso-backup-inventory-20261030T231400Z"+suffix+".json")
		if path != want {
			t.Fatalf("inventory=%q, cần %q", path, want)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("inventory phải có mode 0600: %v", err)
		}
		if previous != "" && readReport(t, previous)["attempt"] != float64(i-1) {
			t.Fatal("inventory trước đã bị ghi đè")
		}
		previous = path
	}
	invalid := make(chan int)
	if _, err := writeSSOBackupInventory(dir, invalid, now); err == nil {
		t.Fatal("JSON không hợp lệ phải trả lỗi")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("JSON không hợp lệ không được tạo inventory: %v, %v", entries, err)
	}
	if err := os.Chmod(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSSOBackupInventory(dir, map[string]int{"attempt": 3}, now); err == nil {
		t.Fatal("thư mục group-readable phải bị từ chối")
	}
}

func TestSSOInventoryNamespaceRefusesProspectiveInputsAndOutputs(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "viderse.json")
	data := mustRead(t, ssoFixtureViderse)
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	base := ssoArgs{
		store: "postgres", vidFile: source, vidID: ssoViderseID,
		manifest: filepath.Join(dir, "manifest.json"), report: filepath.Join(dir, "report.json"),
		dryRun: true, backupDir: backup,
	}
	for _, name := range []string{"sso-backup-inventory-20990101T000000Z.json", "sso-backup-inventory-20990101T000000Z-123.json"} {
		for _, kind := range []string{"manifest", "report", "source", "rename-map"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				args := base
				path := filepath.Join(backup, name)
				switch kind {
				case "manifest":
					args.manifest = path
				case "report":
					args.report = path
				case "source":
					args.vidFile = path
				case "rename-map":
					args.renameMap = path
				}
				err := runStorageSSOImport(args.argv())
				if err == nil || !strings.Contains(err.Error(), "tên inventory dành riêng") {
					t.Fatalf("tên inventory tương lai phải bị từ chối trước đọc input/DB: %v", err)
				}
			})
		}
	}
	entries, err := os.ReadDir(backup)
	if err != nil || len(entries) != 0 || !equalBytes(data, mustRead(t, source)) {
		t.Fatalf("kiểm tra namespace không được ghi input/output: %v, %v", entries, err)
	}
}

func TestSSOBackupInventoryRefusesSymlinkReplacement(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.json")
	before := []byte(`{"input":"unchanged"}`)
	if err := os.WriteFile(source, before, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 31, 6, 14, 0, 0, time.UTC)
	base := filepath.Join(dir, "sso-backup-inventory-20261031T061400Z.json")
	if err := os.Symlink(source, base); err != nil {
		t.Fatal(err)
	}
	path, err := writeSSOBackupInventory(dir, map[string]int{"attempt": 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "sso-backup-inventory-20261031T061400Z-1.json") ||
		!equalBytes(before, mustRead(t, source)) {
		t.Fatalf("O_EXCL phải bỏ qua symlink, không ghi input: %s", path)
	}
}

func TestSSOBackupOnlyCreatesDistinctInventoriesWithoutDatabase(t *testing.T) {
	t.Setenv("MIGRATION_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	dir := t.TempDir()
	source := filepath.Join(dir, "viderse.json")
	before := mustRead(t, ssoFixtureViderse)
	if err := os.WriteFile(source, before, 0o600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup")
	if err := os.Mkdir(backup, 0o750); err != nil {
		t.Fatal(err)
	}
	args := ssoArgs{
		store: "postgres", vidFile: source, vidID: ssoViderseID,
		backupOnly: true, backupDir: backup,
	}
	if err := runStorageSSOImport(args.argv()); err == nil {
		t.Fatal("backup-only phải từ chối thư mục không owner-only")
	}
	for _, suffix := range []string{".lock", ".v4.bak"} {
		if _, err := os.Stat(source + suffix); !os.IsNotExist(err) {
			t.Fatalf("kiểm tra thư mục phải trước tạo %s: %v", suffix, err)
		}
	}
	if err := os.Chmod(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := runStorageSSOImport(args.argv()); err != nil {
			t.Fatalf("backup-only không DSN: %v", err)
		}
	}
	entries, err := os.ReadDir(backup)
	if err != nil || len(entries) != 2 {
		t.Fatalf("hai backup-only phải giữ hai inventory: %v, %v", entries, err)
	}
	for _, entry := range entries {
		if !ssoInventoryName.MatchString(entry.Name()) {
			t.Fatalf("inventory sai pattern: %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("inventory phải có mode 0600: %v", err)
		}
	}
	if !equalBytes(before, mustRead(t, source)) || !equalBytes(before, mustRead(t, source+".v4.bak")) {
		t.Fatal("backup-only phải giữ nguyên nguồn và backup byte-equal")
	}
}
