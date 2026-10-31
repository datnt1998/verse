package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storagegc"
	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
)

// runStorage is the storage administration command. Schema changes are owned
// here, never inside request handlers; the migration role comes from
// MIGRATION_DATABASE_URL (falling back to DATABASE_URL for dev) and is never
// printed.
func runStorage(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, `Quản trị lưu trữ (dev/deployment):

  ainovel-cli storage migrate up       Áp dụng mọi migration đang chờ
  ainovel-cli storage migrate status   Trạng thái từng migration
  ainovel-cli storage migrate version  Phiên bản schema hiện tại
  ainovel-cli storage copy --from-config A --to-config B --manifest M [--resume] [--concurrency N]
                                       Copy blob giữa hai store, đối soát SHA-256
  ainovel-cli storage import --root R --accounts A --source-id U --owner-map S=U --manifest M [--dry-run|--resume]
                                       Nhập thư viện file vào Postgres + BlobStore (lặp an toàn)
  ainovel-cli storage reconcile --manifest M [--verify-blobs] [--json]
                                       Đối soát nguồn ↔ DB/blob; thoát khác 0 khi có lệch
  ainovel-cli storage export-files --destination DIR [--revoke-sessions]
                                       Xuất ngược DB về thư viện file (thư mục mới)
  ainovel-cli storage gc [--dry-run] [--orphan-age 24h] [--staged-age 24h] [--staging-age 24h] [--json]
                                       Dọn object mồ côi, blob không tham chiếu,
                                       tombstone, export_cache hết hạn và staging
                                       quá hạn — yêu cầu MIGRATION_DATABASE_URL
  ainovel-cli storage purge-audit [--days N]
                                       Xoá audit cũ hơn N ngày (mặc định 365) —
                                       chỉ chạy được với vai trò bảo trì
                                       (MIGRATION_DATABASE_URL), không qua app
  ainovel-cli storage sso-import --noverse-store file|postgres
      [--noverse-accounts ABS --noverse-source-id UUID --root ABS --owner-map S=U,...]
      --viderse-accounts ABS --viderse-source-id UUID | --viderse-empty
      --manifest ABS --report ABS [--rename-map ABS]
      [--dry-run | --apply --accept-manifest SHA256 --accept-global-admins SHA256 --backup-dir ABS | --backup-only --backup-dir ABS]
                                       Cutover tài khoản chung: dry-run + manifest theo
                                       mặc định; apply duyệt manifest digest + digest danh
                                       sách admin nền tảng + backup-dir 0700 (owner-only),
                                       archive 0600 pg_dump --format=custom được pg_restore
                                       kiểm chứng: đúng database, đủ bảng relkind=r trong
                                       schema users; COPY count và max timestamptz khớp live
                                       (NULL/infinity), không so đồng hồ. Chọn mtime mới nhất,
                                       bằng nhau chọn tên tăng dần. Receipt giữ version và
                                       executable pg_restore tuyệt đối. Apply cần manifest
                                       đã duyệt còn trên đĩa; thiếu → từ chối, không ghi.
                                       Lệch duyệt → report status=approval_mismatch; chạy
                                       lại --dry-run, xem cả report VÀ manifest mới rồi duyệt.
                                       Mỗi apply/backup-only tạo inventory O_EXCL mới:
                                       sso-backup-inventory-<UTC yyyymmddThhmmssZ>[-N].json;
                                       report apply ghi đúng đường inventory, không ghi đè.
                                       DSN target chỉ qua MIGRATION_DATABASE_URL

Yêu cầu MIGRATION_DATABASE_URL (hoặc DATABASE_URL với STORE_DRIVER=postgres).
DSN không bao giờ được in ra.

Ưu tiên cấu hình (một hợp đồng duy nhất):
  - Hạ tầng runtime đọc từ env đã kiểm tra (STORE_DRIVER, DATABASE_URL, DB_*).
  - Cờ CLI rõ ràng thắng env tương ứng (serve --addr > HTTP_ADDR;
    worker tts --api > TTS_WORKER_API_URL).
  - Lệnh bảo trì dùng MIGRATION_DATABASE_URL trước, rồi DATABASE_URL;
    riêng gc bắt buộc MIGRATION_DATABASE_URL, không dùng DSN ứng dụng.
  - STORAGE_ENV (dev|staging|prod, trống = dev) là tiền tố khoá blob của cả
    serve và gc; giá trị lạ là lỗi, không đoán.
  - storage copy dùng file config 0600 tự chứa, không đọc env blob xung quanh.
`)
		return fmt.Errorf("thiếu lệnh con (migrate up|status|version | copy)")
	}
	switch args[0] {
	case "migrate":
		return runStorageMigrate(args[1:])
	case "copy":
		return runStorageCopy(args[1:])
	case "import":
		return runStorageImport(args[1:])
	case "reconcile":
		return runStorageReconcile(args[1:])
	case "export-files":
		return runStorageExportFiles(args[1:])
	case "purge-audit":
		return runStoragePurgeAudit(args[1:])
	case "gc":
		return runStorageGC(args[1:])
	case "sso-import":
		return runStorageSSOImport(args[1:])
	default:
		return fmt.Errorf("lệnh con không hợp lệ %q (migrate|copy|import|sso-import|reconcile|export-files|purge-audit|gc)", args[0])
	}
}

// runStorageCopy copies every object from one blob store to another and
// verifies each target by re-downloading it and comparing SHA-256+size.
// Config files are mode-0600 JSON outside the repo (credentials never in
// argv); the manifest ledger (JSONL) makes the run resumable per immutable
// key. Same-hash targets are skipped, different-hash targets are conflicts —
// never overwritten — and any mismatch/conflict exits nonzero.
func runStorageCopy(args []string) error {
	flags := flag.NewFlagSet("storage copy", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	fromPath := flags.String("from-config", "", "đường dẫn config nguồn (JSON, mode 0600)")
	toPath := flags.String("to-config", "", "đường dẫn config đích (JSON, mode 0600)")
	manifest := flags.String("manifest", "", "đường dẫn manifest/ledger JSONL")
	resume := flags.Bool("resume", false, "tiếp tục từ manifest đã có")
	concurrency := flags.Int("concurrency", 4, "số object copy song song")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *fromPath == "" || *toPath == "" || *manifest == "" {
		return fmt.Errorf("cần --from-config, --to-config và --manifest")
	}

	fromCfg, err := bootstrap.BlobConfigFromFile(*fromPath)
	if err != nil {
		return fmt.Errorf("from-config: %w", err)
	}
	toCfg, err := bootstrap.BlobConfigFromFile(*toPath)
	if err != nil {
		return fmt.Errorf("to-config: %w", err)
	}
	src, err := bootstrap.OpenBlobStore(fromCfg)
	if err != nil {
		return fmt.Errorf("mở nguồn (%s): %w", fromCfg.SafeSummary(), err)
	}
	dst, err := bootstrap.OpenBlobStore(toCfg)
	if err != nil {
		return fmt.Errorf("mở đích (%s): %w", toCfg.SafeSummary(), err)
	}

	fmt.Printf("Copy %s → %s (manifest %s)\n", fromCfg.SafeSummary(), toCfg.SafeSummary(), *manifest)
	rep, err := blobstore.NewCopier(src, dst).Run(context.Background(), blobstore.CopyOptions{
		ManifestPath: *manifest,
		Resume:       *resume,
		Concurrency:  *concurrency,
		Progress: func(e blobstore.LedgerEntry) {
			fmt.Printf("%-9s %s (%dB %s) %s\n", e.Status, e.Key, e.Size, e.SHA256, e.Detail)
		},
	})
	fmt.Printf("Xong: nguồn=%d copied=%d matched=%d conflicts=%d mismatches=%d errors=%d\n",
		rep.SourceObjects, rep.Copied, rep.Matched, rep.Conflicts, rep.Mismatches, rep.Errors)
	if err != nil {
		return err
	}
	if rep.Conflicts+rep.Mismatches+rep.Errors > 0 {
		return fmt.Errorf("copy không sạch: conflicts=%d mismatches=%d errors=%d — không được chuyển pointer",
			rep.Conflicts, rep.Mismatches, rep.Errors)
	}
	return nil
}

func runStorageMigrate(args []string) error {
	flags := flag.NewFlagSet("storage migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("cần đúng một lệnh: up | status | version")
	}
	action := flags.Arg(0)
	switch action {
	case "up", "status", "version":
	default:
		return fmt.Errorf("lệnh không hợp lệ %q (up|status|version)", action)
	}

	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return err
	}
	dsn := cfg.MigrationDSN()
	if dsn == "" {
		return fmt.Errorf("thiếu MIGRATION_DATABASE_URL (hoặc DATABASE_URL với STORE_DRIVER=postgres)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	switch action {
	case "up":
		res, err := migrate.Up(ctx, db)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			fmt.Println("Không có migration nào chờ — schema đã mới nhất.")
			return nil
		}
		for _, r := range res {
			fmt.Printf("Áp dụng %s (%.0fms)\n", r.Source.Path, float64(r.Duration.Microseconds())/1000)
		}
		return nil
	case "status":
		st, err := migrate.Status(ctx, db)
		if err != nil {
			return err
		}
		for _, s := range st {
			state := "đã áp dụng"
			if !s.Applied {
				state = "chờ"
			}
			fmt.Printf("%05d %s %s\n", s.Version, state, s.Source)
		}
		return nil
	case "version":
		v, err := migrate.Version(ctx, db)
		if err != nil {
			return err
		}
		fmt.Printf("Phiên bản schema: %d\n", v)
		return nil
	}
	return nil
}

// ── storage import / reconcile / export-files ───────────────────────────────

// runStoragePurgeAudit runs audit retention under the maintenance principal.
// Audit rows are append-only to the application role: the SQL trigger accepts
// the delete only when the connection is noverse_migrate (or a superuser) AND
// the transaction sets the maintenance marker. Using DATABASE_URL here fails
// loudly instead of silently granting the app purge power.
func runStoragePurgeAudit(args []string) error {
	flags := flag.NewFlagSet("storage purge-audit", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	days := flags.Int("days", 365, "xoá audit cũ hơn N ngày")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *days <= 0 {
		return fmt.Errorf("--days phải > 0")
	}
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return err
	}
	dsn := cfg.MigrationDSN()
	if dsn == "" {
		return fmt.Errorf("thiếu MIGRATION_DATABASE_URL (hoặc DATABASE_URL với STORE_DRIVER=postgres)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, postgres.PoolConfig{
		DSN: dsn, MaxConns: cfg.MaxConns, ConnectTimeout: cfg.ConnectTimeout,
	})
	if err != nil {
		return fmt.Errorf("mở pool bảo trì: %w", err)
	}
	defer pool.Close()
	cutoff := time.Now().UTC().AddDate(0, 0, -*days)
	n, err := identitypg.NewAccounts(pool.Pool(), identitypg.Options{
		ProjectAccess: true, ValidProjectID: library.ValidProjectID,
	}).PurgeAuditBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("xoá audit cũ: %w", err)
	}
	fmt.Printf("Đã xoá %d mục audit trước %s.\n", n, cutoff.Format(time.RFC3339))
	return nil
}

// openImportTargets opens the Postgres pool and blob store from the
// environment. Credentials never reach argv.
func openImportTargets(ctx context.Context) (*postgres.Pool, blobstore.BlobStore, string, error) {
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return nil, nil, "", err
	}
	dsn := cfg.MigrationDSN()
	if dsn == "" {
		return nil, nil, "", fmt.Errorf("thiếu MIGRATION_DATABASE_URL (hoặc DATABASE_URL với STORE_DRIVER=postgres)")
	}
	pool, err := postgres.Open(ctx, postgres.PoolConfig{
		DSN:            dsn,
		MaxConns:       cfg.MaxConns,
		ConnectTimeout: cfg.ConnectTimeout,
	})
	if err != nil {
		return nil, nil, "", err
	}
	blobCfg, err := bootstrap.BlobConfigFromEnv()
	if err != nil {
		pool.Close()
		return nil, nil, "", err
	}
	blobs, err := bootstrap.OpenBlobStore(blobCfg)
	if err != nil {
		pool.Close()
		return nil, nil, "", fmt.Errorf("mở BlobStore: %w", err)
	}
	return pool, blobs, string(blobCfg.Driver), nil
}

// activeKeys returns the object-key scheme of STORAGE_ENV. A bad value is an
// error: maintenance commands never guess the environment.
func activeKeys() (blobkey.Keys, error) {
	keys, err := blobkey.FromEnv()
	if err != nil {
		return blobkey.Keys{}, fmt.Errorf("cấu hình STORAGE_ENV: %w", err)
	}
	return keys, nil
}

func parseOwnerMap(value string) (map[string]string, error) {
	out := map[string]string{}
	if value == "" {
		return out, nil
	}
	for _, pair := range strings.Split(value, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		slug, user, ok := strings.Cut(pair, "=")
		if !ok || slug == "" || user == "" {
			return nil, fmt.Errorf("--owner-map cần dạng slug=username (nhận %q)", pair)
		}
		out[strings.TrimSpace(slug)] = strings.TrimSpace(user)
	}
	return out, nil
}

// runStorageImport is the idempotent file→Postgres+BlobStore importer. It
// always writes the frozen manifest first, then either reports (--dry-run) or
// imports. A rerun of the same manifest adds no rows and no blobs.
func runStorageImport(args []string) error {
	flags := flag.NewFlagSet("storage import", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", "", "thư mục gốc thư viện (bản sao chỉ đọc)")
	accountsPath := flags.String("accounts", "", "đường dẫn web-accounts.json")
	sourceID := flags.String("source-id", "", "UUID định danh nguồn (giữ nguyên giữa các lần chạy)")
	ownerMap := flags.String("owner-map", "", "dự án=username, cách nhau bởi dấu phẩy")
	manifestPath := flags.String("manifest", "", "đường dẫn manifest JSON (ghi nếu chưa có)")
	dryRun := flags.Bool("dry-run", false, "chỉ phân loại/giải mã và báo xung đột")
	resume := flags.Bool("resume", false, "tiếp tục run đã có (mặc định vẫn lặp an toàn)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" || *sourceID == "" || *manifestPath == "" {
		return fmt.Errorf("cần --root, --source-id và --manifest")
	}
	owners, err := parseOwnerMap(*ownerMap)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	scan, err := storageimport.Scan(ctx, storageimport.ScanOptions{
		Root: *root, Accounts: *accountsPath, SourceID: *sourceID,
		SourceLabel: filepath.Base(filepath.Clean(*root)),
	})
	if err != nil {
		return err
	}
	if err := storageimport.WriteManifest(*manifestPath, scan.Manifest); err != nil {
		return fmt.Errorf("ghi manifest: %w", err)
	}

	keys, err := activeKeys()
	if err != nil {
		return err
	}
	opts := storageimport.Options{
		Scan: scan, DryRun: *dryRun, Resume: *resume, OwnerMap: owners,
		ReadOnlyRoot: *root, Keys: keys,
	}
	if !*dryRun {
		pool, blobs, driver, err := openImportTargets(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()
		opts.Pool, opts.Blobs, opts.Driver = pool, blobs, driver
	}
	result, runErr := storageimport.Run(ctx, opts)
	fmt.Println(result.Summary())
	if runErr != nil {
		return runErr
	}
	if result.Failed() {
		return fmt.Errorf("import chưa đạt cửa: %d xung đột, %d cách ly", len(result.Conflicts), len(result.Quarantined))
	}
	return nil
}

func runStorageReconcile(args []string) error {
	flags := flag.NewFlagSet("storage reconcile", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	manifestPath := flags.String("manifest", "", "manifest JSON của lần nhập")
	verifyBlobs := flags.Bool("verify-blobs", false, "tải lại từng blob và so SHA-256")
	asJSON := flags.Bool("json", false, "in JSON thay vì tóm tắt tiếng Việt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return fmt.Errorf("cần --manifest")
	}
	manifest, err := storageimport.LoadManifest(*manifestPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	pool, blobs, _, err := openImportTargets(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	keys, err := activeKeys()
	if err != nil {
		return err
	}
	report, err := storageimport.Reconcile(ctx, storageimport.ReconcileOptions{
		Manifest: manifest, SourceID: manifest.SourceID, Pool: pool,
		Blobs: blobs, VerifyBlobs: *verifyBlobs, Keys: keys,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(report.FormatReconcile())
	}
	if !report.OK() {
		return fmt.Errorf("đối soát có lệch")
	}
	return nil
}

// runStorageGC collects unreachable storage: tombstones, expired export-cache
// entries and their exclusive blobs, unreferenced blobs past the grace window,
// abandoned staging objects and orphan objects. Every action is logged; a
// second run is a no-op. --dry-run reports the same candidates without
// deleting anything or taking row locks.
func runStorageGC(args []string) error {
	flags := flag.NewFlagSet("storage gc", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dryRun := flags.Bool("dry-run", false, "chỉ báo cáo, không xoá gì")
	orphanAge := flags.Duration("orphan-age", storagegc.DefaultGrace, "tuổi tối thiểu của blob không tham chiếu/object mồ côi")
	stagingAge := flags.Duration("staging-age", storagegc.DefaultStagingAge, "tuổi tối thiểu của object trong staging/")
	stagedAge := flags.Duration("staged-age", storagegc.DefaultStagedAge, "tuổi tối thiểu của hàng staged không còn job đang hoạt động/giữ lease")
	asJSON := flags.Bool("json", false, "in JSON thay vì tóm tắt tiếng Việt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *orphanAge <= 0 || *stagingAge <= 0 || *stagedAge <= 0 {
		return fmt.Errorf("--orphan-age, --staged-age và --staging-age phải > 0")
	}
	if strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL")) == "" {
		return fmt.Errorf("storage gc yêu cầu MIGRATION_DATABASE_URL của vai trò bảo trì; không dùng DATABASE_URL của ứng dụng")
	}
	keys, err := activeKeys()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, blobs, _, err := openImportTargets(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	report, runErr := storagegc.Run(ctx, storagegc.Options{
		Pool: pool, Blobs: blobs, Keys: keys,
		Grace: *orphanAge, StagedAge: *stagedAge, StagingAge: *stagingAge, DryRun: *dryRun,
	})
	if *asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(report.Format())
	}
	return runErr
}

func runStorageExportFiles(args []string) error {
	flags := flag.NewFlagSet("storage export-files", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	manifestPath := flags.String("manifest", "", "manifest của lần nhập (để biết nguồn)")
	sourceID := flags.String("source-id", "", "UUID nguồn (nếu không truyền --manifest)")
	destination := flags.String("destination", "", "thư mục đích (phải trống hoặc chưa tồn tại)")
	revoke := flags.Bool("revoke-sessions", false, "thu hồi toàn bộ phiên/pairing sau khi xuất")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *destination == "" {
		return fmt.Errorf("cần --destination")
	}
	id := *sourceID
	if *manifestPath != "" {
		manifest, err := storageimport.LoadManifest(*manifestPath)
		if err != nil {
			return err
		}
		id = manifest.SourceID
	}
	if id == "" {
		return fmt.Errorf("cần --manifest hoặc --source-id")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	pool, blobs, _, err := openImportTargets(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	report, err := storageimport.ExportFiles(ctx, storageimport.ExportOptions{
		SourceID: id, Destination: *destination, Pool: pool, Blobs: blobs,
		RevokeSessions: *revoke,
	})
	fmt.Print(report.FormatExport())
	return err
}
