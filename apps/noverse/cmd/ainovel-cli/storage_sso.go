package main

// storage sso-import: one-time cutover of both apps' account files into the
// shared Postgres store. Dry-run by default; apply requires the approved
// manifest digest plus a backup directory holding a dump receipt. The target
// DSN comes ONLY from MIGRATION_DATABASE_URL — never from DATABASE_URL, an
// app role, or argv. Sources are absolute paths the owner picked; nothing is
// read from a default HOME and nothing is written to a source file.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
)

// ssoReport is the owner-facing, secret-free cutover record: usernames,
// roles, counts and digests only — never a hash, token, plaintext password
// or DSN.
type ssoReport struct {
	Version                int                              `json:"version"`
	Mode                   string                           `json:"mode"` // dry-run|apply|backup-only
	Status                 string                           `json:"status"`
	ManifestPath           string                           `json:"manifestPath,omitempty"`
	ManifestSHA256         string                           `json:"manifestSha256,omitempty"`
	LibraryManifestPath    string                           `json:"libraryManifestPath,omitempty"`
	Sources                []storageimport.SourceReport     `json:"sources"`
	Collisions             []storageimport.CutoverCollision `json:"collisions,omitempty"`
	Fatal                  string                           `json:"fatal,omitempty"`
	Renames                []storageimport.RenameEntry      `json:"renamesApplied,omitempty"`
	GlobalAdmins           []storageimport.GlobalAdmin      `json:"globalAdmins,omitempty"`
	GlobalAdminsDigest     string                           `json:"globalAdminsDigest"`
	LiveUsersWithoutGrants int                              `json:"liveUsersWithoutGrants"`
	ActiveAdminCount       int                              `json:"activeAdminCountAfter"`
	Backups                []storageimport.BackupReceipt    `json:"backups,omitempty"`
	DatabaseBackup         *storageimport.VerifiedBackup    `json:"databaseBackup,omitempty"`
	BackupInventoryPath    string                           `json:"backupInventoryPath,omitempty"`
	Identity               *storageimport.ApplyResult       `json:"identity,omitempty"`
	Library                *storageimport.Result            `json:"library,omitempty"`
	Notes                  []string                         `json:"notes,omitempty"`
	StartedAt              time.Time                        `json:"startedAt"`
	FinishedAt             time.Time                        `json:"finishedAt"`
}

func runStorageSSOImport(args []string) error {
	flags := flag.NewFlagSet("storage sso-import", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	store := flags.String("noverse-store", "", "chế độ Noverse hiện tại: file|postgres (bắt buộc)")
	noverseAccounts := flags.String("noverse-accounts", "", "đường tuyệt đối web-accounts.json v4 của Noverse (bắt buộc khi file)")
	viderseAccounts := flags.String("viderse-accounts", "", "đường tuyệt đối web-accounts.json v4 của Viderse")
	viderseEmpty := flags.Bool("viderse-empty", false, "khai báo Viderse chưa từng có file tài khoản (thay --viderse-accounts)")
	root := flags.String("root", "", "đường tuyệt đối snapshot thư viện Noverse (bắt buộc khi file)")
	ownerMap := flags.String("owner-map", "", "dự án=username nguồn, cách nhau bởi dấu phẩy")
	noverseSourceID := flags.String("noverse-source-id", "", "UUID nguồn Noverse (bắt buộc khi file)")
	viderseSourceID := flags.String("viderse-source-id", "", "UUID nguồn Viderse (bắt buộc khi --viderse-accounts)")
	manifestPath := flags.String("manifest", "", "đường tuyệt đối manifest sẽ được owner duyệt (0600)")
	reportPath := flags.String("report", "", "đường tuyệt đối report JSON (0600)")
	renameMapPath := flags.String("rename-map", "", "đường tuyệt đối JSON [{sourceApp,sourceUsername,targetUsername,role?}]")
	dryRun := flags.Bool("dry-run", false, "mặc định: preflight + report, không ghi source/backup/DB")
	apply := flags.Bool("apply", false, "nhập sau khi duyệt manifest digest + backup-dir")
	backupOnly := flags.Bool("backup-only", false, "chỉ tạo .v4.bak cạnh file nguồn + receipt vào --backup-dir (không DB)")
	acceptManifest := flags.String("accept-manifest", "", "sha256 manifest owner đã duyệt (bắt buộc với --apply)")
	backupDir := flags.String("backup-dir", "", "thư mục chứa dump receipt, lưu bằng đường tuyệt đối (bắt buộc với --apply/--backup-only)")
	acceptAdmins := flags.String("accept-global-admins", "", "sha256 digest danh sách globalAdmins đã duyệt trong report (bắt buộc với --apply)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *backupDir != "" {
		absolute, err := filepath.Abs(*backupDir)
		if err != nil {
			return fmt.Errorf("phân giải --backup-dir: %w", err)
		}
		*backupDir = absolute
	}

	// ── flag validation ────────────────────────────────────────────────────
	switch *store {
	case "file", "postgres":
	default:
		return fmt.Errorf("cần --noverse-store file|postgres")
	}
	modes := 0
	for _, b := range []bool{*dryRun, *apply, *backupOnly} {
		if b {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("--dry-run, --apply và --backup-only loại trừ nhau")
	}
	if *backupOnly {
		if *acceptManifest != "" || *acceptAdmins != "" || *manifestPath != "" || *reportPath != "" {
			return fmt.Errorf("--backup-only không nhận --accept-manifest/--accept-global-admins/--manifest/--report")
		}
		if *backupDir == "" {
			return fmt.Errorf("--backup-only cần --backup-dir để ghi receipt")
		}
	} else {
		if *manifestPath == "" || *reportPath == "" {
			return fmt.Errorf("cần --manifest và --report (đường tuyệt đối, thư mục private)")
		}
	}
	if *apply && !*backupOnly {
		if *acceptManifest == "" {
			return fmt.Errorf("--apply cần --accept-manifest <sha256 của manifest đã duyệt>")
		}
		if len(*acceptManifest) != 64 || !isHexLower(*acceptManifest) {
			return fmt.Errorf("--accept-manifest phải là sha256 hex thường 64 ký tự")
		}
		if *backupDir == "" {
			return fmt.Errorf("--apply cần --backup-dir chứa dump receipt (pg_dump) của owner")
		}
	}
	if !*apply && (*acceptManifest != "" || *acceptAdmins != "") {
		return fmt.Errorf("--accept-manifest/--accept-global-admins chỉ có nghĩa với --apply")
	}
	for name, p := range map[string]string{
		"--manifest": *manifestPath, "--report": *reportPath,
		"--rename-map": *renameMapPath,
	} {
		if p != "" && !filepath.IsAbs(p) {
			return fmt.Errorf("%s phải là đường dẫn tuyệt đối (%q)", name, p)
		}
	}
	for name, p := range map[string]string{
		"--noverse-accounts": *noverseAccounts, "--viderse-accounts": *viderseAccounts,
	} {
		if p != "" && !filepath.IsAbs(p) {
			return fmt.Errorf("%s phải là đường dẫn tuyệt đối (%q)", name, p)
		}
	}

	var noverseSrc storageimport.IdentitySource
	switch *store {
	case "file":
		if *noverseAccounts == "" || *root == "" || *noverseSourceID == "" {
			return fmt.Errorf("--noverse-store=file cần --noverse-accounts, --root và --noverse-source-id")
		}
		if !filepath.IsAbs(*root) {
			return fmt.Errorf("--root phải là đường dẫn tuyệt đối (%q)", *root)
		}
		noverseSrc = storageimport.IdentitySource{
			App: storageimport.SourceNoverse, SourceID: *noverseSourceID, Path: *noverseAccounts,
			Label: filepath.Base(filepath.Clean(*root)),
		}
	case "postgres":
		if *noverseAccounts != "" || *root != "" || *ownerMap != "" || *noverseSourceID != "" {
			return fmt.Errorf("--noverse-store=postgres không nhập file/thư viện Noverse: bỏ --noverse-accounts/--root/--owner-map/--noverse-source-id")
		}
	}
	var viderseSrc storageimport.IdentitySource
	switch {
	case *viderseEmpty && *viderseAccounts != "":
		return fmt.Errorf("chọn --viderse-accounts HOẶC --viderse-empty, không cả hai")
	case *viderseEmpty:
		if *viderseSourceID != "" {
			return fmt.Errorf("--viderse-empty không có source file: bỏ --viderse-source-id")
		}
		viderseSrc = storageimport.IdentitySource{App: storageimport.SourceViderse, SourceID: "00000000-0000-0000-0000-000000000000", Empty: true}
	case *viderseAccounts == "":
		return fmt.Errorf("cần --viderse-accounts hoặc --viderse-empty — không tự coi file thiếu là kho trống")
	default:
		if *viderseSourceID == "" {
			return fmt.Errorf("cần --viderse-source-id khi nhập file Viderse")
		}
		viderseSrc = storageimport.IdentitySource{
			App: storageimport.SourceViderse, SourceID: *viderseSourceID, Path: *viderseAccounts,
		}
	}

	// Canonicalize each source id to lower-case UUID text before anything
	// hashes or locks on it: the library scan and the identity pass must see
	// the same string even when the owner typed upper case. An invalid id
	// stays verbatim so preflight reports it as a bad source, not a flag error.
	canonID := func(s string) string {
		if c, err := storageimport.CanonicalSourceID(s); err == nil {
			return c
		}
		return s
	}
	noverseSrc.SourceID = canonID(noverseSrc.SourceID)
	viderseSrc.SourceID = canonID(viderseSrc.SourceID)

	// One UUID namespaces every deterministic user id — two different files
	// under one id would let one source forge the other's resume ledger.
	if noverseSrc.SourceID != "" && viderseSrc.SourceID != "" &&
		noverseSrc.SourceID == viderseSrc.SourceID {
		return fmt.Errorf("--noverse-source-id và --viderse-source-id phải khác nhau — mỗi app cần UUID riêng")
	}
	noverseSourceIDCanonical := canonID(*noverseSourceID)

	// ── output-path safety: nothing this command writes may clobber an
	// input (source file, its .v4.bak/.lock twins, the rename map) or land
	// inside the frozen library root.
	{
		var outputs []string
		for _, p := range []string{*manifestPath, *reportPath} {
			if p != "" {
				outputs = append(outputs, p)
			}
		}
		if *manifestPath != "" && *store == "file" {
			outputs = append(outputs, *manifestPath+".library.json")
		}
		reservedPaths := append([]string(nil), outputs...)
		if *backupDir != "" {
			outputs = append(outputs, filepath.Join(*backupDir, "sso-backup-inventory-"+time.Now().UTC().Format("20060102T150405Z")+".json"))
		}
		var inputs []string
		for _, p := range []string{*renameMapPath, *noverseAccounts, *viderseAccounts} {
			if p == "" {
				continue
			}
			inputs = append(inputs, p, p+".v4.bak", p+".lock")
		}
		// Reserve the entire inventory namespace, including names not created
		// yet and retry suffixes, before any report, lock or backup is written.
		for _, p := range append(reservedPaths, inputs...) {
			if *backupDir != "" && isSSOInventoryPath(p, *backupDir) {
				return fmt.Errorf("đường %s trùng tên inventory dành riêng trong --backup-dir — từ chối ghi đè input/output", p)
			}
		}
		for i, o := range outputs {
			for _, other := range outputs[i+1:] {
				if sameFilePath(o, other) {
					return fmt.Errorf("đường output %s và %s trùng nhau — manifest/report/inventory phải khác đường", o, other)
				}
			}
			for _, in := range inputs {
				if sameFilePath(o, in) {
					return fmt.Errorf("đường output %s trùng input %s — từ chối ghi đè dữ liệu nguồn/duyệt", o, in)
				}
			}
			// An output that already exists as a pg_dump archive would
			// destroy the very backup evidence apply needs: --report
			// $BACKUP/x.dump can never clobber the dump.
			if isPGDumpArchivePath(o) {
				return fmt.Errorf("đường output %s là một archive pg_dump đang tồn tại — từ chối ghi đè bằng chứng backup", o)
			}
			if *root != "" && pathWithin(*root, o) {
				return fmt.Errorf("đường output %s nằm trong --root %s — snapshot nguồn phải giữ nguyên", o, *root)
			}
		}
	}
	// A deleted approval artifact is never recreated by apply, nor may its
	// absence produce an approval-mismatch report.
	if *apply {
		if _, err := os.ReadFile(*manifestPath); err != nil {
			return fmt.Errorf("không đọc được manifest đã duyệt: %w — chạy lại --dry-run, xem xét report mới VÀ manifest mới trước khi duyệt --apply", err)
		}
	}
	owners, err := parseOwnerMap(*ownerMap)
	if err != nil {
		return err
	}

	// ── source locks: the file store serializes writers on <path>.lock; a
	// held lock means a writer is still alive and the snapshot is not frozen.
	holdSourceLock := func(path string) (*flock.Flock, error) {
		if path == "" {
			return nil, nil
		}
		l := flock.New(path + ".lock")
		ok, err := l.TryLock()
		if err != nil {
			return nil, fmt.Errorf("khoá %s.lock: %w", path, err)
		}
		if !ok {
			return nil, fmt.Errorf("%s.lock đang được giữ — còn writer trên nguồn; chờ maintenance window", path)
		}
		return l, nil
	}
	var locks []*flock.Flock
	defer func() {
		for _, l := range locks {
			_ = l.Unlock()
		}
	}()
	takeLocks := func() error {
		for _, p := range []string{noverseSrc.Path, viderseSrc.Path} {
			if p == "" {
				continue
			}
			l, err := holdSourceLock(p)
			if err != nil {
				return err
			}
			if l != nil {
				locks = append(locks, l)
			}
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	started := time.Now().UTC()
	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	if *backupOnly {
		mode = "backup-only"
	}
	report := &ssoReport{Version: 1, Mode: mode, StartedAt: started}
	finish := func(status string) {
		report.Status = status
		report.FinishedAt = time.Now().UTC()
	}
	writeReport := func() error {
		if *reportPath == "" {
			return nil
		}
		return writePrivateJSONAtomic(*reportPath, report)
	}

	renames, err := storageimport.LoadRenameMap(*renameMapPath)
	if err != nil {
		return err
	}

	// ── backup-only: freeze-verified .v4.bak next to each source + receipts;
	// no DSN is opened and nothing touches the database.
	if *backupOnly {
		if err := storageimport.CheckPrivateBackupDir(*backupDir); err != nil {
			return err
		}
		if err := takeLocks(); err != nil {
			return err
		}
		var sources []storageimport.BackupSource
		for _, s := range []storageimport.IdentitySource{noverseSrc, viderseSrc} {
			if s.App == "" || s.Empty {
				continue
			}
			bs, err := storageimport.BackupSourceFor(s)
			if err != nil {
				return err
			}
			sources = append(sources, bs)
		}
		receipts, err := storageimport.EnsureSourceBackups(sources)
		if err != nil {
			return err
		}
		inv := map[string]any{"version": 1, "mode": "backup-only", "receipts": receipts, "at": time.Now().UTC()}
		invPath, err := writeSSOBackupInventory(*backupDir, inv, time.Now())
		if err != nil {
			return err
		}
		for _, r := range receipts {
			state := "đã kiểm chứng"
			if r.Created {
				state = "đã tạo"
			}
			fmt.Printf("backup %s (%s, sha256 %s…)\n", r.BackupPath, state, r.SourceSHA256[:12])
		}
		fmt.Printf("Inventory: %s\n", invPath)
		return nil
	}

	// ── target pool: MIGRATION_DATABASE_URL only, never the app DSN.
	dsn := strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("thiếu MIGRATION_DATABASE_URL — lệnh này không dùng DATABASE_URL/app role, không nhận DSN qua argv")
	}
	cfg, err := bootstrap.StorageConfigFromEnv()
	if err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, postgres.PoolConfig{
		DSN: dsn, MaxConns: cfg.MaxConns, ConnectTimeout: cfg.ConnectTimeout,
	})
	if err != nil {
		return fmt.Errorf("mở pool bảo trì: %w", err)
	}
	defer pool.Close()

	// ── library scan (file mode): same source id, same frozen snapshot.
	var scan *storageimport.ScanResult
	var libManifestPath string
	if *store == "file" {
		scan, err = storageimport.Scan(ctx, storageimport.ScanOptions{
			Root: *root, Accounts: *noverseAccounts, SourceID: noverseSourceIDCanonical,
			SourceLabel: filepath.Base(filepath.Clean(*root)),
		})
		if err != nil {
			return fmt.Errorf("quét thư viện: %w", err)
		}
		// Identity decodes the very bytes the scan hashed into the library
		// manifest: one read, one snapshot.
		noverseSrc.Raw = scan.AccountsRaw
		libManifestPath = *manifestPath + ".library.json"
		// The library manifest file is only written after the preflight and
		// (on apply) the manifest digest check.
		report.LibraryManifestPath = libManifestPath
	}

	plan, err := storageimport.PlanCutover(ctx, pool, noverseSrc, viderseSrc, renames)
	if err != nil {
		return err
	}
	report.Sources = plan.Sources()
	report.Collisions = plan.Collisions
	report.Fatal = plan.Fatal
	report.Renames = plan.Renames
	report.GlobalAdmins = plan.GlobalAdmins
	report.GlobalAdminsDigest = plan.GlobalAdminsDigest
	report.LiveUsersWithoutGrants = plan.LiveUsersWithoutGrants
	report.ActiveAdminCount = plan.ActiveAdminCountAfter
	if plan.LiveUsersWithoutGrants > 0 {
		report.Notes = append(report.Notes,
			fmt.Sprintf("%d tài khoản live trong đích không có membership app nào — kiểm tra thủ công, công cụ không tự cấp lại grant đã thu hồi", plan.LiveUsersWithoutGrants))
	}

	libSHA := ""
	if scan != nil {
		libSHA = scan.Manifest.SHA256
	}
	m := storageimport.ManifestForPlan(plan, *store, libSHA, *root, owners)
	mSHA, mBytes, err := storageimport.ManifestSHA256(m)
	if err != nil {
		return err
	}
	report.ManifestSHA256 = mSHA
	// A refusal at this stage writes the report with the CURRENT admin list
	// and the recomputed manifest digest, then stops with a message that
	// never carries a 64-hex digest — the owner reads the report.
	approvalMismatch := func() error {
		finish("approval_mismatch")
		if werr := writeReport(); werr != nil {
			return werr
		}
		return fmt.Errorf("manifest hoặc danh sách admin đã duyệt không còn khớp — chạy lại --dry-run, xem xét report mới %s VÀ manifest mới %s trước khi duyệt --apply; report lệch duyệt này không phải căn cứ phê duyệt", *reportPath, *manifestPath)
	}
	if *apply {
		// The file on disk must be exactly what the owner approved.
		onDisk, readErr := os.ReadFile(*manifestPath)
		if readErr == nil {
			sum := sha256.Sum256(onDisk)
			if hex.EncodeToString(sum[:]) != mSHA || !equalBytes(onDisk, mBytes) {
				return approvalMismatch()
			}
		} else {
			return fmt.Errorf("không đọc được manifest đã duyệt: %w — chạy lại --dry-run, xem xét report mới VÀ manifest mới trước khi duyệt --apply", readErr)
		}
		if *acceptManifest != mSHA {
			return approvalMismatch()
		}
		if *acceptAdmins == "" {
			return fmt.Errorf("--apply cần --accept-global-admins <digest danh sách globalAdmins trong report dry-run>")
		}
		if *acceptAdmins != plan.GlobalAdminsDigest {
			return approvalMismatch()
		}
	}
	report.ManifestPath = *manifestPath
	if err := writePrivateBytesAtomic(*manifestPath, mBytes); err != nil {
		return fmt.Errorf("ghi manifest: %w", err)
	}
	if !*apply && scan != nil {
		if err := writePrivateJSONAtomic(libManifestPath, scan.Manifest); err != nil {
			return fmt.Errorf("ghi library manifest: %w", err)
		}
	}
	if *apply && scan != nil {
		if err := writePrivateJSONAtomic(libManifestPath, scan.Manifest); err != nil {
			return fmt.Errorf("ghi library manifest: %w", err)
		}
	}

	if plan.Failed() {
		finish("preflight_failed")
		if werr := writeReport(); werr != nil {
			return werr
		}
		printSSOSummary(report)
		return fmt.Errorf("preflight có %d xung đột/lỗi — xem %s; không ghi gì vào DB", len(plan.Collisions), *reportPath)
	}
	if *dryRun || !*apply {
		finish("planned")
		if werr := writeReport(); werr != nil {
			return werr
		}
		printSSOSummary(report)
		fmt.Printf("Manifest sha256: %s — duyệt rồi chạy lại với --apply --accept-manifest %s --backup-dir <dir>\n", mSHA, mSHA)
		fmt.Printf("Admin nền tảng sau cutover (%d) — bắt buộc duyệt: --accept-global-admins %s\n",
			len(plan.GlobalAdmins), plan.GlobalAdminsDigest)
		return nil
	}

	// ── apply: approvals done above → verify the pg_dump backup against
	// this very database → take source locks → .v4.bak → one identity tx.
	dump, err := storageimport.CheckBackupDir(ctx, pool, *backupDir)
	if err != nil {
		return err
	}
	report.DatabaseBackup = dump
	if err := takeLocks(); err != nil {
		return err
	}
	receipts, err := storageimport.EnsureSourceBackups(plan.BackupSources())
	if err != nil {
		return err
	}
	report.Backups = receipts
	inv := map[string]any{
		"version": 1, "manifestSha256": mSHA, "receipts": receipts,
		"databaseDump": dump, "at": time.Now().UTC(),
	}
	invPath, err := writeSSOBackupInventory(*backupDir, inv, time.Now())
	if err != nil {
		return fmt.Errorf("ghi backup inventory: %w", err)
	}
	report.BackupInventoryPath = invPath
	if werr := writeReport(); werr != nil {
		return werr
	}

	applied, err := storageimport.ApplyCutover(ctx, pool, plan, dump, mSHA, invPath, started)
	if err != nil {
		finish("identity_failed")
		if werr := writeReport(); werr != nil {
			return werr
		}
		return fmt.Errorf("identity rollback hoàn toàn: %w", err)
	}
	report.Identity = applied
	report.Sources = plan.Sources()

	if scan != nil {
		finish("identity_committed")
		if werr := writeReport(); werr != nil {
			return werr
		}
		blobCfg, err := bootstrap.BlobConfigFromEnv()
		if err != nil {
			return err
		}
		blobs, err := bootstrap.OpenBlobStore(blobCfg)
		if err != nil {
			return fmt.Errorf("mở BlobStore: %w", err)
		}
		keys, err := activeKeys()
		if err != nil {
			return err
		}
		libResult, runErr := storageimport.Run(ctx, storageimport.Options{
			Scan: scan, Pool: pool, Blobs: blobs, Driver: string(blobCfg.Driver),
			OwnerMap: owners, ReadOnlyRoot: *root, Keys: keys, Now: started,
		})
		report.Library = libResult
		if runErr != nil || libResult.Failed() {
			finish("identity_committed_library_incomplete")
			if werr := writeReport(); werr != nil {
				return werr
			}
			printSSOSummary(report)
			if runErr != nil {
				return fmt.Errorf("identity đã commit; thư viện chưa hoàn tất (resume cùng manifest): %w", runErr)
			}
			return fmt.Errorf("identity đã commit; thư viện chưa hoàn tất (resume cùng manifest)")
		}
	}

	finish("complete")
	if werr := writeReport(); werr != nil {
		return werr
	}
	printSSOSummary(report)
	return nil
}

func isHexLower(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writePrivateJSONAtomic writes v as mode-0600 JSON through a same-directory
// temp file + rename so a crash never leaves a half report.
func writePrivateJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateBytesAtomic(path, append(data, '\n'))
}

func writePrivateBytesAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sso-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

var ssoInventoryName = regexp.MustCompile(`(?i)^sso-backup-inventory-[0-9]{8}T[0-9]{6}Z(?:-[1-9][0-9]*)?\.json$`)

func isSSOInventoryPath(path, dir string) bool {
	resolved := realPath(path)
	return sameFilePath(filepath.Dir(resolved), dir) && ssoInventoryName.MatchString(filepath.Base(resolved))
}

// writeSSOBackupInventory never replaces an earlier receipt, even when two
// applies finish in the same second. O_EXCL also refuses symlink destinations.
func writeSSOBackupInventory(dir string, inventory any, now time.Time) (string, error) {
	if dir == "" {
		return "", errors.New("thiếu --backup-dir")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("phân giải --backup-dir: %w", err)
	}
	if err := storageimport.CheckPrivateBackupDir(dir); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	base := "sso-backup-inventory-" + now.UTC().Format("20060102T150405Z")
	for suffix := 0; ; suffix++ {
		name := base
		if suffix > 0 {
			name += fmt.Sprintf("-%d", suffix)
		}
		path := filepath.Join(dir, name+".json")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		writeErr := func() error {
			if _, err := f.Write(data); err != nil {
				return err
			}
			return f.Sync()
		}()
		closeErr := f.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return "", errors.Join(err, os.Remove(path))
		}
		return path, nil
	}
}

func printSSOSummary(r *ssoReport) {
	fmt.Printf("sso-import %s → %s\n", r.Mode, r.Status)
	for _, s := range r.Sources {
		fmt.Printf("  nguồn %-7s users=%d audit=%d bearer-huỷ=%d/%d resume-skip=%d sha=%s\n",
			s.App, s.Users, s.Audit, s.SessionsDrop, s.PairingsDrop, s.ResumeSkips, shortSHA(s.SHA256))
	}
	if len(r.Collisions) > 0 {
		fmt.Printf("  XUNG ĐỘT (%d):\n", len(r.Collisions))
		for _, c := range r.Collisions {
			fmt.Printf("    [%s] %s\n", c.Kind, c.Detail)
		}
	}
	if r.Fatal != "" {
		fmt.Printf("  LỖI: %s\n", r.Fatal)
	}
	if len(r.GlobalAdmins) > 0 {
		names := make([]string, 0, len(r.GlobalAdmins))
		for _, a := range r.GlobalAdmins {
			entry := a.Username + " (" + a.Origin + ")"
			if a.Locked {
				entry += " [locked/tombstone]"
			}
			names = append(names, entry)
		}
		fmt.Printf("  Admin nền tảng sau cutover (%d): %s\n", len(r.GlobalAdmins), strings.Join(names, ", "))
	}
	if r.Identity != nil {
		fmt.Printf("  identity: +%d user (skip %d), +%d grant, +%d audit; revoke %d session/%d pairing\n",
			r.Identity.Inserted, r.Identity.Skipped, r.Identity.Grants, r.Identity.AuditInserted,
			r.Identity.SessionsRevoked, r.Identity.PairingsRevoked)
	}
	if r.Library != nil {
		fmt.Printf("  thư viện: %d bản ghi (%d mới, %d bỏ qua), %d blob (%d mới)\n",
			r.Library.RowsInserted+r.Library.RowsSkipped, r.Library.RowsInserted, r.Library.RowsSkipped,
			r.Library.BlobsUploaded+r.Library.BlobsSkipped, r.Library.BlobsUploaded)
	}
	if r.BackupInventoryPath != "" {
		fmt.Printf("  inventory: %s\n", r.BackupInventoryPath)
	}
	for _, n := range r.Notes {
		fmt.Printf("  ghi chú: %s\n", n)
	}
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// sameFilePath reports whether two paths name the same file: os.SameFile
// when both exist (hardlinks counted), otherwise cleaned absolute paths
// resolved through symlinks of existing parents and compared
// case-insensitively under Unicode NFC, so a differently-encoded name of
// the same file cannot hide a collision.
func sameFilePath(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	if errA == nil && errB == nil && os.SameFile(ia, ib) {
		return true
	}
	return realPath(a) == realPath(b)
}

// isPGDumpArchivePath reports whether path is an existing pg_dump archive
// (custom- or directory-format magic): such a file is backup evidence and
// may never become an output.
func isPGDumpArchivePath(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len("PGDMP"))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return string(head) == "PGDMP"
}

// pathWithin reports whether child sits at or below dir after resolution.
func pathWithin(dir, child string) bool {
	d := realPath(dir)
	c := realPath(child)
	return c == d || strings.HasPrefix(c, d+string(os.PathSeparator))
}

// realPath cleans the path, resolves the deepest existing ancestor so
// symlinked parents (common for temp dirs) collapse to the physical name,
// then normalizes to NFC: the filesystem may return decomposed names for an
// NFC input (and vice versa), and both spellings must compare equal.
func realPath(p string) string {
	p = filepath.Clean(p)
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return strings.ToLower(norm.NFC.String(resolved))
	}
	// The path itself may not exist yet; resolve the parent instead.
	parent := filepath.Dir(p)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		return strings.ToLower(norm.NFC.String(filepath.Join(resolved, filepath.Base(p))))
	}
	return strings.ToLower(norm.NFC.String(p))
}
