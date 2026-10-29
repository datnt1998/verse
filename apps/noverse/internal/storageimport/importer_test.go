package storageimport_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/blobstore"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/persistence/migrate"
	"github.com/datnt1998/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/storageimport"
	storepkg "github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/webauth"
)

const (
	fixtureRoot     = "../store/testdata/storage-contract/library"
	fixtureAccounts = "../store/testdata/storage-contract/accounts/web-accounts.v3.json"
	fixtureSourceID = "6f1d0f9e-3f2a-4c88-9d0e-1b1a2b3c4d5e"
)

func scanFixture(t *testing.T, root, accounts string) *storageimport.ScanResult {
	t.Helper()
	scan, err := storageimport.Scan(context.Background(), storageimport.ScanOptions{
		Root: root, Accounts: accounts, SourceID: fixtureSourceID, SourceLabel: "fixture",
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return scan
}

func openTargets(t *testing.T) (*postgres.Pool, blobstore.BlobStore) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration db: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	t.Cleanup(func() { _ = blobs.Close() })
	return pool, blobs
}

func runImport(t *testing.T, scan *storageimport.ScanResult, root string, pool *postgres.Pool, blobs blobstore.BlobStore) *storageimport.Result {
	t.Helper()
	result, err := storageimport.Run(context.Background(), storageimport.Options{
		Scan: scan, Pool: pool, Blobs: blobs, Driver: "local", ReadOnlyRoot: root,
		OwnerMap: map[string]string{"du-an": "tac-gia", "_": "quan-tri"},
		Now:      time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("import: %v (conflicts=%v quarantined=%v)", err, result.Conflicts, result.Quarantined)
	}
	if result.Failed() {
		t.Fatalf("import not green: conflicts=%v quarantined=%v", result.Conflicts, result.Quarantined)
	}
	return result
}

// TestScanClassifiesEveryPath pins the phase-01 requirement: every persistent
// path gets exactly one class, and the manifest hash is reproducible.
func TestScanClassifiesEveryPath(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	if len(scan.Manifest.Files) == 0 {
		t.Fatal("manifest rỗng")
	}
	seen := map[string]bool{}
	for _, f := range scan.Manifest.Files {
		if f.Path == storageimport.PathAccounts {
			continue
		}
		if seen[f.Path] {
			t.Fatalf("path trùng trong manifest: %s", f.Path)
		}
		seen[f.Path] = true
		switch f.Kind {
		case storageimport.KindFacts, storageimport.KindBlob, storageimport.KindDerived,
			storageimport.KindScratch, storageimport.KindLocal, storageimport.KindExclude:
		default:
			t.Fatalf("%s: class lạ %q", f.Path, f.Kind)
		}
		if f.Kind == storageimport.KindDerived && f.Reason == "" {
			t.Fatalf("%s: đường dẫn derived phải có lý do", f.Path)
		}
		if f.SHA256 == "" || len(f.SHA256) != 64 {
			t.Fatalf("%s: thiếu raw hash", f.Path)
		}
	}
	// every story and project discovered
	if len(scan.Projects) != 2 {
		t.Fatalf("cần 2 dự án (du-an + dự án ẩn danh _), có %d", len(scan.Projects))
	}
	found := map[string]bool{}
	for _, s := range scan.Stories {
		found[s.ProjectSlug+"/"+s.Slug] = true
	}
	for _, want := range []string{"du-an/01-dem-trang", "_/truyen-cu"} {
		if !found[want] {
			t.Fatalf("thiếu truyện %s trong %v", want, found)
		}
	}
	again := scanFixture(t, fixtureRoot, fixtureAccounts)
	if again.Manifest.SHA256 != scan.Manifest.SHA256 {
		t.Fatalf("manifest hash không ổn định: %s vs %s", scan.Manifest.SHA256, again.Manifest.SHA256)
	}
}

// TestDeterministicIDs pins the idempotency contract: the same logical key in
// the same source always yields the same target id.
func TestDeterministicIDs(t *testing.T) {
	a := storageimport.DeterministicUUID(fixtureSourceID, "chapter", "p/s/1")
	b := storageimport.DeterministicUUID(fixtureSourceID, "chapter", "p/s/1")
	c := storageimport.DeterministicUUID(fixtureSourceID, "chapter", "p/s/2")
	if a != b {
		t.Fatalf("cùng khoá phải cùng id: %s vs %s", a, b)
	}
	if a == c {
		t.Fatal("khoá khác phải khác id")
	}
	if len(a) != 36 {
		t.Fatalf("không phải UUID: %q", a)
	}
}

// TestImportIsIdempotent is the phase gate: a second run of the same frozen
// source adds zero rows and zero blobs, and reconciliation is clean.
func TestImportIsIdempotent(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	first := runImport(t, scan, fixtureRoot, pool, blobs)
	if first.RowsInserted == 0 || first.BlobsUploaded == 0 {
		t.Fatalf("lần đầu phải nhập dữ liệu: rows=%d blobs=%d", first.RowsInserted, first.BlobsUploaded)
	}

	rescan := scanFixture(t, fixtureRoot, fixtureAccounts)
	second := runImport(t, rescan, fixtureRoot, pool, blobs)
	if second.RowsInserted != 0 {
		t.Fatalf("lần hai phải thêm 0 dòng, thực tế %d", second.RowsInserted)
	}
	if second.BlobsUploaded != 0 {
		t.Fatalf("lần hai phải tải 0 blob, thực tế %d", second.BlobsUploaded)
	}

	report, err := storageimport.Reconcile(context.Background(), storageimport.ReconcileOptions{
		Manifest: rescan.Manifest, SourceID: fixtureSourceID, Pool: pool, Blobs: blobs, VerifyBlobs: true,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.OK() {
		t.Fatalf("đối soát phải sạch:\n%s", report.FormatReconcile())
	}
	if report.BlobsVerified != first.BlobsUploaded {
		t.Fatalf("xác minh %d blob, đã tải %d", report.BlobsVerified, first.BlobsUploaded)
	}
}

// TestImportDoesNotClobberLiveEdit pins the "no blind overwrite" rule: after a
// post-import edit the second run reports a conflict instead of reverting it.
func TestImportDoesNotClobberLiveEdit(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx,
		`UPDATE stories SET title = 'Người dùng sửa sau import', updated_at = now(), version = version + 1 WHERE slug = '01-dem-trang'`); err != nil {
		t.Fatalf("sửa live: %v", err)
	}
	// A rerun must not revert the edit (no blind upsert) and reconciliation
	// must name the row as a mismatch.
	second := runImport(t, scanFixture(t, fixtureRoot, fixtureAccounts), fixtureRoot, pool, blobs)
	if second.RowsInserted != 0 {
		t.Fatalf("lần hai phải thêm 0 dòng, thực tế %d", second.RowsInserted)
	}
	rescan := scanFixture(t, fixtureRoot, fixtureAccounts)
	report, err := storageimport.Reconcile(context.Background(), storageimport.ReconcileOptions{
		Manifest: rescan.Manifest, SourceID: fixtureSourceID, Pool: pool, Blobs: blobs, VerifyBlobs: true,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.OK() {
		t.Fatal("đối soát phải phát hiện bản sửa live")
	}
	found := false
	for _, m := range report.Mismatches {
		if strings.Contains(m, "story/01-dem-trang") {
			found = true
		}
	}
	if !found {
		t.Fatalf("phải báo lệch story, có: %v", report.Mismatches)
	}
	var title string
	if err := pool.Pool().QueryRow(ctx, `SELECT title FROM stories WHERE slug='01-dem-trang'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Người dùng sửa sau import" {
		t.Fatalf("bản sửa live bị ghi đè: %q", title)
	}
}

func runImportExpectConflict(t *testing.T, scan *storageimport.ScanResult, pool *postgres.Pool, blobs blobstore.BlobStore) *storageimport.Result {
	t.Helper()
	result, err := storageimport.Run(context.Background(), storageimport.Options{
		Scan: scan, Pool: pool, Blobs: blobs, Driver: "local", ReadOnlyRoot: fixtureRoot,
		OwnerMap: map[string]string{"du-an": "tac-gia", "_": "quan-tri"},
	})
	if err == nil || !result.Failed() {
		t.Fatalf("mong đợi xung đột, err=%v conflicts=%v", err, result.Conflicts)
	}
	return result
}

// TestDriftInSourceIsLoud pins the frozen-snapshot rule: changing one byte
// under an already-imported path is a conflict, never a silent second import.
func TestDriftInSourceIsLoud(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	path := filepath.Join(root, "du-an", "project.json")
	if err := os.WriteFile(path, []byte(`{"name":"Đổi tên sau khi đóng băng","description":"","notes":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	drifted := scanFixture(t, root, fixtureAccounts)
	if drifted.Manifest.SHA256 == scan.Manifest.SHA256 {
		t.Fatal("manifest phải đổi khi nguồn đổi")
	}
	second := runImportExpectConflict(t, drifted, pool, blobs)
	if len(second.Conflicts) == 0 {
		t.Fatal("phải báo xung đột drift")
	}
	var name string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT name FROM projects WHERE slug='du-an'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name == "Đổi tên sau khi đóng băng" {
		t.Fatal("drift không được âm thầm ghi đè")
	}
}

// TestReconcileCatchesCorruptBlob pins the "missing/same-size corrupt blob"
// gate row: reconcile exits with a mismatch when the stored object is damaged.
func TestReconcileCatchesCorruptBlob(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	var objectKey string
	if err := pool.Pool().QueryRow(context.Background(),
		`SELECT object_key FROM blobs ORDER BY object_key LIMIT 1`).Scan(&objectKey); err != nil {
		t.Fatal(err)
	}
	rc, _, err := blobs.Open(context.Background(), objectKey, blobstore.FullRange)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("blob rỗng")
	}
	data[0] ^= 0xff
	if _, err := blobs.Put(context.Background(), objectKey, strings.NewReader(string(data)), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	report, err := storageimport.Reconcile(context.Background(), storageimport.ReconcileOptions{
		Manifest: scan.Manifest, SourceID: fixtureSourceID, Pool: pool, Blobs: blobs, VerifyBlobs: true,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.OK() {
		t.Fatal("đối soát phải phát hiện blob hỏng")
	}
	if len(report.Mismatches) == 0 {
		t.Fatal("phải có mục lệch")
	}
}

// copyFixtureWithCharacters copies the contract library and adds the
// characters file the fixture keeps out (so the character domain has a source
// file to import). It returns the library root.
func copyFixtureWithCharacters(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	characters := []domain.Character{
		{ID: "thu-thu", Name: "Thủ thư trẻ", Role: "nhân vật chính", Kind: domain.CharacterKindLead, Description: "Giữ cuốn sổ cuối cùng.", Arc: "từ sợ hãi đến can đảm", Traits: []string{"điềm tĩnh", "tò mò"}},
		{ID: "nguoi-la", Name: "Người lạ", Role: "phản diện", Kind: domain.CharacterKindAntagonist, Description: "Đến tìm cuốn sổ.", Arc: "lộ diện", Traits: []string{"lạnh lùng"}},
	}
	data, err := json.MarshalIndent(characters, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", "output", "novel", "characters.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	signals := filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", "output", "novel", "meta", "signals.json")
	if err := os.WriteFile(signals, []byte(`{"pendingCommit":{"chapter":1}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// productionStore builds the Store facade over the same backend the hosted
// engine uses, pinned to one imported story.
func productionStore(t *testing.T, pool *postgres.Pool, project, story string) *storepkg.Store {
	t.Helper()
	ctx := context.Background()
	nar := postgres.NewNarrative(pool)
	scope, err := nar.Resolve(ctx, project, story, nil, false)
	if err != nil {
		t.Fatalf("resolve scope: %v", err)
	}
	st, err := storepkg.NewFromBackend(postgres.NewBackend(ctx, pool, nar, scope))
	if err != nil {
		t.Fatalf("store from backend: %v", err)
	}
	return st
}

// TestImportReadableThroughProductionStore is the fidelity gate for the
// import: after importing the file library, the production Store facade must
// read plans, accepted chapter text, characters and the accepted chapter
// record with every field the source had.
func TestImportReadableThroughProductionStore(t *testing.T) {
	root := copyFixtureWithCharacters(t)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	st := productionStore(t, pool, "du-an", "01-dem-trang")
	engineDir := filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", "output", "novel")

	// plan: drafts/02.plan.json is the chapter_plan artifact of chapter 2.
	var wantPlan domain.ChapterPlan
	readJSONFile(t, filepath.Join(engineDir, "drafts", "02.plan.json"), &wantPlan)
	gotPlan, err := st.Drafts.LoadChapterPlan(2)
	if err != nil {
		t.Fatalf("đọc plan: %v", err)
	}
	if gotPlan == nil {
		t.Fatal("Store production không đọc được plan đã nhập")
	}
	if !reflect.DeepEqual(*gotPlan, wantPlan) {
		t.Fatalf("plan khác nguồn:\n got %+v\nwant %+v", *gotPlan, wantPlan)
	}

	// chapter text: chapters/01.md is the chapter_text artifact.
	wantText, err := os.ReadFile(filepath.Join(engineDir, "chapters", "01.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotText, err := st.Drafts.LoadChapterText(1)
	if err != nil {
		t.Fatalf("đọc chương: %v", err)
	}
	if gotText != string(wantText) {
		t.Fatalf("chương khác nguồn:\n got %q\nwant %q", gotText, string(wantText))
	}

	// characters: story_state(kind='characters').
	var wantChars []domain.Character
	readJSONFile(t, filepath.Join(engineDir, "characters.json"), &wantChars)
	gotChars, err := st.Characters.Load()
	if err != nil {
		t.Fatalf("đọc nhân vật: %v", err)
	}
	if !reflect.DeepEqual(gotChars, wantChars) {
		t.Fatalf("nhân vật khác nguồn:\n got %+v\nwant %+v", gotChars, wantChars)
	}

	// accepted chapter record: every field, including facts/style/digest/time.
	var wantRecord domain.ChapterRecord
	readJSONFile(t, filepath.Join(engineDir, "meta", "chapter_records", "000001.json"), &wantRecord)
	gotRecord, err := st.ChapterRecords.Load(1)
	if err != nil {
		t.Fatalf("đọc bản ghi chương: %v", err)
	}
	if gotRecord == nil {
		t.Fatal("Store production không đọc được bản ghi chương")
	}
	if gotRecord.Chapter != wantRecord.Chapter || gotRecord.Revision != wantRecord.Revision ||
		gotRecord.Build != wantRecord.Build || gotRecord.Origin != wantRecord.Origin ||
		gotRecord.ContentSHA256 != wantRecord.ContentSHA256 {
		t.Fatalf("bản ghi chương khác nguồn:\n got %+v\nwant %+v", *gotRecord, wantRecord)
	}
	if gotRecord.Content != domain.NormalizeChapterContent(wantRecord.Content) {
		t.Fatal("nội dung bản ghi chương khác nguồn")
	}
	if !gotRecord.AcceptedAt.Equal(wantRecord.AcceptedAt) {
		t.Fatalf("accepted_at khác nguồn: got %s want %s", gotRecord.AcceptedAt, wantRecord.AcceptedAt)
	}
	if !reflect.DeepEqual(gotRecord.Facts, wantRecord.Facts) {
		t.Fatalf("facts khác nguồn:\n got %+v\nwant %+v", gotRecord.Facts, wantRecord.Facts)
	}
	if !reflect.DeepEqual(gotRecord.StyleDelta, wantRecord.StyleDelta) {
		t.Fatalf("style_delta khác nguồn:\n got %+v\nwant %+v", gotRecord.StyleDelta, wantRecord.StyleDelta)
	}
}

// TestImportReconcilesClean pins that a faithful import still reconciles with
// zero mismatches, so the stricter comparison does not degrade into a check
// that always reports differences.
func TestImportReconcilesClean(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)
	report, err := storageimport.Reconcile(context.Background(), storageimport.ReconcileOptions{
		Manifest: scan.Manifest, SourceID: fixtureSourceID, Pool: pool, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !report.OK() {
		t.Fatalf("đối soát phải sạch sau nhập:\n%s", report.FormatReconcile())
	}
}

// TestReconcileReportsMissingChapterFacts damages the stored revision the way
// a partial migration would (facts lost) and requires reconciliation to name
// the chapter instead of reporting a clean read-back.
func TestReconcileReportsMissingChapterFacts(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	ctx := context.Background()
	tx, err := pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL noverse.allow_immutable_write = 'on'`); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE chapter_versions SET facts = '{}'
WHERE project_id = (SELECT project_id FROM stories WHERE slug = '01-dem-trang')`); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("mô phỏng mất facts: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	report, err := storageimport.Reconcile(ctx, storageimport.ReconcileOptions{
		Manifest: scan.Manifest, SourceID: fixtureSourceID, Pool: pool, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if report.OK() {
		t.Fatalf("đối soát phải báo lệch khi mất facts:\n%s", report.FormatReconcile())
	}
	found := false
	for _, m := range report.Mismatches {
		if strings.Contains(m, "chapter/000001") {
			found = true
		}
	}
	if !found {
		t.Fatalf("phải báo lệch chapter/000001, có: %v", report.Mismatches)
	}
}

// TestExportRoundTrip opens the exported tree with the FILE backend and a
// fresh account store: everything a rollback needs must be there without
// reusing the source accounts.
func TestExportRoundTrip(t *testing.T) {
	root := copyFixtureWithCharacters(t)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	destination := t.TempDir()
	exported, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exported.FilesWritten == 0 || exported.BlobsWritten == 0 {
		t.Fatalf("export rỗng: files=%d blobs=%d", exported.FilesWritten, exported.BlobsWritten)
	}
	if exported.Checkpoints != 3 {
		t.Fatalf("export phải ghi 3 checkpoint, có %d", exported.Checkpoints)
	}

	// file Store over the exported story directory.
	engineDir := filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang", "output", "novel")
	st := storepkg.NewStore(engineDir)

	sourceEngine := filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", "output", "novel")
	var wantPlan domain.ChapterPlan
	readJSONFile(t, filepath.Join(sourceEngine, "drafts", "02.plan.json"), &wantPlan)
	gotPlan, err := st.Drafts.LoadChapterPlan(2)
	if err != nil || gotPlan == nil || !reflect.DeepEqual(*gotPlan, wantPlan) {
		t.Fatalf("plan sau export: %+v, err=%v", gotPlan, err)
	}
	wantText, err := os.ReadFile(filepath.Join(sourceEngine, "chapters", "01.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotText, err := st.Drafts.LoadChapterText(1)
	if err != nil || gotText != string(wantText) {
		t.Fatalf("chương sau export khác nguồn (err=%v)", err)
	}
	var wantChars []domain.Character
	readJSONFile(t, filepath.Join(sourceEngine, "characters.json"), &wantChars)
	gotChars, err := st.Characters.Load()
	if err != nil || !reflect.DeepEqual(gotChars, wantChars) {
		t.Fatalf("nhân vật sau export khác nguồn: %+v err=%v", gotChars, err)
	}
	var wantRecord domain.ChapterRecord
	readJSONFile(t, filepath.Join(sourceEngine, "meta", "chapter_records", "000001.json"), &wantRecord)
	gotRecord, err := st.ChapterRecords.Load(1)
	if err != nil {
		t.Fatalf("bản ghi chương sau export không mở được: %v", err)
	}
	if gotRecord == nil || gotRecord.ContentSHA256 != wantRecord.ContentSHA256 ||
		!reflect.DeepEqual(gotRecord.Facts, wantRecord.Facts) ||
		!gotRecord.AcceptedAt.Equal(wantRecord.AcceptedAt) {
		t.Fatalf("bản ghi chương sau export khác nguồn: %+v", gotRecord)
	}
	// retained revision history is present, not only the current record.
	if _, err := os.Stat(filepath.Join(engineDir, "meta", "chapter_versions", "000001", "1.json")); err != nil {
		t.Fatalf("thiếu lịch sử revision sau export: %v", err)
	}
	// draft-only chapter keeps its draft (it has no accepted record).
	wantDraft, err := os.ReadFile(filepath.Join(sourceEngine, "drafts", "02.draft.md"))
	if err != nil {
		t.Fatal(err)
	}
	if draft, err := st.Drafts.LoadDraft(2); err != nil || draft != string(wantDraft) {
		t.Fatalf("bản nháp chương 2 sau export: %q err=%v", draft, err)
	}
	// importer singletons outside the document mapping keep their content
	// (jsonb re-encodes whitespace, so compare decoded equality).
	for _, rel := range []string{"meta/signals.json"} {
		var wantDoc, gotDoc any
		readJSONFile(t, filepath.Join(sourceEngine, rel), &wantDoc)
		readJSONFile(t, filepath.Join(engineDir, rel), &gotDoc)
		if !reflect.DeepEqual(gotDoc, wantDoc) {
			t.Fatalf("%s khác nguồn sau export: %v", rel, gotDoc)
		}
	}
	var wantEvents, gotEvents any
	readJSONFile(t, filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", ".ainovel", "gop-y", "_events.json"), &wantEvents)
	readJSONFile(t, filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang", ".ainovel", "gop-y", "_events.json"), &gotEvents)
	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Fatalf("gop-y/_events.json khác nguồn sau export: %v", gotEvents)
	}
	// checkpoints, the decision stream and the review round are readable by
	// the file backend.
	if got := len(st.Checkpoints.All()); got != 3 {
		t.Fatalf("checkpoint sau export: %d", got)
	}
	if review, err := st.World.LoadReview(1); err != nil || review == nil || review.Chapter != 1 {
		t.Fatalf("bài thẩm duyệt sau export: %+v err=%v", review, err)
	}
	if decisions, err := st.Decisions.Recent(10); err != nil || len(decisions) == 0 {
		t.Fatalf("sự kiện quyết định sau export: %d, err=%v", len(decisions), err)
	}

	// accounts: read through a fresh webauth store, never the fixture file.
	accountsPath := filepath.Join(destination, storageimport.PathAccountsExport)
	accounts := webauth.NewStore(accountsPath)
	users, err := accounts.Users()
	if err != nil {
		t.Fatalf("mở tài khoản đã export: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("tài khoản sau export: %d", len(users))
	}
	byName := map[string]webauth.UserListView{}
	for _, u := range users {
		byName[u.Username] = u
	}
	author, ok := byName["tac-gia"]
	if !ok {
		t.Fatalf("thiếu tài khoản tac-gia: %+v", users)
	}
	if author.Projects["du-an"] != webauth.ProjectOwner {
		t.Fatalf("quyền dự án sau export: %+v", author.Projects)
	}
	var exportedFile struct {
		Users []struct {
			Username     string `json:"username"`
			PasswordHash string `json:"passwordHash"`
		} `json:"users"`
	}
	readJSONFile(t, accountsPath, &exportedFile)
	for _, u := range exportedFile.Users {
		if !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
			t.Fatalf("tài khoản %s mất credential sau export", u.Username)
		}
	}

	// project files keep their source path: never project/project/...
	if _, err := os.Stat(filepath.Join(destination, "du-an", ".ainovel", "config.json")); err != nil {
		t.Fatalf("thiếu cấu hình dự án sau export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "du-an", "du-an")); !os.IsNotExist(err) {
		t.Fatalf("export lặp tiền tố dự án: %v", err)
	}
	var leaked []string
	_ = filepath.Walk(engineDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(p, "chapter_plan/") {
			rel, _ := filepath.Rel(destination, p)
			leaked = append(leaked, rel)
		}
		return nil
	})
	if len(leaked) > 0 {
		t.Fatalf("export giữ tiền tố chapter_plan/: %v", leaked)
	}

	// the exported tree is a real library: it scans back to the same projects
	// and stories (named project plus the anonymous legacy project).
	lib, err := library.Open(destination)
	if err != nil {
		t.Fatalf("mở thư viện đã export: %v", err)
	}
	res, err := lib.Scan()
	if err != nil {
		t.Fatalf("quét thư viện đã export: %v", err)
	}
	projects := map[string]bool{}
	for _, p := range res.Projects {
		projects[p.ID] = true
	}
	if !projects["du-an"] || !projects["_"] {
		t.Fatalf("thư viện export thiếu dự án: %+v", res.Projects)
	}
	stories, err := lib.Stories("du-an")
	if err != nil || len(stories) != 1 || stories[0].Name != "01-dem-trang" {
		t.Fatalf("truyện sau export: %+v err=%v", stories, err)
	}
	if _, ok := lib.ResolveStory("_", "truyen-cu"); !ok {
		t.Fatal("truyện cũ (root) mất sau export")
	}
}

// TestExportIncludesPostCutoverContent requires the export to enumerate the
// live database, not the import ledger: a project created after the cutover
// and a blob written after the cutover must both be present.
func TestExportIncludesPostCutoverContent(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	ctx := context.Background()
	lib := postgres.NewLibraryRepository(pool)
	if _, err := lib.CreateProject(ctx, nil, "du-an-moi", libraryProjectFile()); err != nil {
		t.Fatalf("tạo dự án sau cutover: %v", err)
	}
	key := "dev/p/" + projectIDBySlug(t, pool, "du-an-moi") + "/assets/" + strings.Repeat("a", 64) + ".txt"
	body := []byte("blob tạo sau cutover")
	if _, err := blobs.Put(ctx, key, strings.NewReader(string(body)), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO blobs (id, project_id, object_key, storage_driver, sha256, size_bytes, content_type, state)
VALUES ($1, (SELECT id FROM projects WHERE slug='du-an-moi'), $2, 'local', $3, $4, 'text/plain', 'ready')`,
		storageimport.DeterministicUUID(fixtureSourceID, "test_blob", key), key,
		storageimport.RawDigest(body), len(body)); err != nil {
		t.Fatalf("ghi blob sau cutover: %v", err)
	}

	destination := t.TempDir()
	report, err := storageimport.ExportFiles(ctx, storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if report.Projects != 3 {
		t.Fatalf("export phải thấy 3 dự án sống, thấy %d", report.Projects)
	}
	if _, err := os.Stat(filepath.Join(destination, "du-an-moi", "project.json")); err != nil {
		t.Fatalf("thiếu dự án tạo sau cutover: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(destination, "du-an-moi", "tai-san", "blobs", "*"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("thiếu blob tạo sau cutover: %v %v", matches, err)
	}
}

// TestExportFailsWhenBlobUnreadable requires a missing object to fail the
// export instead of producing a rollback with a hole.
func TestExportFailsWhenBlobUnreadable(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	var objectKey string
	if err := pool.Pool().QueryRow(context.Background(),
		`SELECT object_key FROM blobs ORDER BY object_key LIMIT 1`).Scan(&objectKey); err != nil {
		t.Fatal(err)
	}
	if err := blobs.Delete(context.Background(), objectKey); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err == nil {
		t.Fatal("export phải fail khi thiếu blob")
	}
}

// TestExportSkipsSoftDeletedContent requires a deleted story to stay deleted:
// a rollback must not recreate its files.
func TestExportSkipsSoftDeletedContent(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, fixtureRoot, pool, blobs)

	if _, err := pool.Pool().Exec(context.Background(), `
UPDATE stories SET deleted_at = now(), updated_at = now(), version = version + 1
WHERE slug = '01-dem-trang'`); err != nil {
		t.Fatalf("xoá mềm truyện: %v", err)
	}
	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang")); !os.IsNotExist(err) {
		t.Fatalf("export làm sống lại truyện đã xoá mềm: %v", err)
	}
}

// TestImportRejectsFileChangedAfterScan pins the frozen-snapshot rule for fact
// files: bytes that no longer match the manifest hash fail the run.
func TestImportRejectsFileChangedAfterScan(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)

	path := filepath.Join(root, "du-an", "project.json")
	if err := os.WriteFile(path, []byte(`{"name":"Đổi sau khi quét","description":"","notes":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := storageimport.Run(context.Background(), storageimport.Options{
		Scan: scan, Pool: pool, Blobs: blobs, Driver: "local", ReadOnlyRoot: root,
		OwnerMap: map[string]string{"du-an": "tac-gia", "_": "quan-tri"},
	})
	if err == nil || !result.Failed() {
		t.Fatalf("import phải fail khi file đổi sau quét: err=%v conflicts=%v", err, result.Conflicts)
	}
	found := false
	for _, q := range result.Quarantined {
		if strings.Contains(q, "du-an/project.json") {
			found = true
		}
	}
	if !found {
		t.Fatalf("phải cách ly du-an/project.json, có: %v", result.Quarantined)
	}
	var name string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT name FROM projects WHERE slug='du-an'`).Scan(&name); err == nil && name == "Đổi sau khi quét" {
		t.Fatal("không được nhập nội dung đã đổi")
	}
}

// TestExportKeepsExtensionlessStoryFile imports a story-level file with no
// extension (a valid story file, not an artifact key that looks like a path)
// and requires the export to preserve its bytes instead of refusing the run.
func TestExportKeepsExtensionlessStoryFile(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	notes := filepath.Join(root, "du-an", "kich_ban", "01-dem-trang", "notes")
	if err := os.WriteFile(notes, []byte("ghi chú tay của tác giả\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err != nil {
		t.Fatalf("export phải nhận file không phần mở rộng: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang", "output", "novel", "notes"))
	if err != nil || string(got) != "ghi chú tay của tác giả\n" {
		t.Fatalf("file không phần mở rộng mất sau export: %q err=%v", got, err)
	}
}

// TestExportKeepsUnanchoredGlobalReview saves a global review without a
// chapter anchor through the Postgres store and requires the export to name it
// the way the file backend does (reviews/00-global.json), then load it back.
func TestExportKeepsUnanchoredGlobalReview(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	ctx := context.Background()
	nar := postgres.NewNarrative(pool)
	scope, err := nar.Resolve(ctx, "du-an", "01-dem-trang", nil, true)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := domain.ReviewEntry{
		Chapter: 0, Scope: "global", Verdict: "accept",
		Summary: "Tổng duyệt toàn truyện", Issues: []domain.ConsistencyIssue{},
	}
	if err := nar.SaveReview(ctx, scope, want); err != nil {
		t.Fatalf("lưu global review: %v", err)
	}

	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(ctx, storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err != nil {
		t.Fatalf("export với global review không mốc: %v", err)
	}
	engineDir := filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang", "output", "novel")
	if _, err := os.Stat(filepath.Join(engineDir, "reviews", "00-global.json")); err != nil {
		t.Fatalf("thiếu reviews/00-global.json: %v", err)
	}
	got, err := storepkg.NewStore(engineDir).World.LoadGlobalReview(0)
	if err != nil || got == nil || got.Summary != want.Summary || got.Scope != "global" {
		t.Fatalf("đọc lại global review sau export: %+v err=%v", got, err)
	}
}

// TestExportSkipsDeletedChapter requires a soft-deleted chapter to stay
// deleted: its prose, record and chapter-scoped artifacts must not be written.
func TestExportSkipsDeletedChapter(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	if _, err := pool.Pool().Exec(context.Background(), `
UPDATE chapters SET deleted_at = now(), updated_at = now(), version = version + 1
WHERE number = 1 AND story_id IN (SELECT id FROM stories WHERE slug = '01-dem-trang')`); err != nil {
		t.Fatalf("xoá mềm chương: %v", err)
	}
	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err != nil {
		t.Fatalf("export: %v", err)
	}
	engineDir := filepath.Join(destination, "du-an", "kich_ban", "01-dem-trang", "output", "novel")
	for _, rel := range []string{"chapters/01.md", "meta/chapter_records/000001.json"} {
		if _, err := os.Stat(filepath.Join(engineDir, rel)); !os.IsNotExist(err) {
			t.Fatalf("export làm sống lại chương đã xoá mềm: %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(engineDir, "drafts", "02.draft.md")); err != nil {
		t.Fatalf("export phải giữ bản nháp chương còn sống: %v", err)
	}
}

// TestImportDraftConflictIsNotOverwritten simulates a resumed run that lost
// its ledger row: a differing stored draft is preserved and reported as a
// conflict, never silently overwritten.
func TestImportDraftConflictIsNotOverwritten(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx, `
UPDATE chapters SET draft_text = 'nháp sửa sau khi nhập', updated_at = now(), version = version + 1
WHERE number = 2 AND story_id IN (SELECT id FROM stories WHERE slug = '01-dem-trang')`); err != nil {
		t.Fatalf("sửa nháp: %v", err)
	}
	ltx, err := pool.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ltx.Exec(ctx, `SET LOCAL noverse.allow_immutable_write = 'on'`); err != nil {
		_ = ltx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := ltx.Exec(ctx, `
DELETE FROM import_items WHERE source_id = $1::uuid
AND relative_path = 'du-an/kich_ban/01-dem-trang/output/novel/drafts/02.draft.md'`, fixtureSourceID); err != nil {
		_ = ltx.Rollback(ctx)
		t.Fatalf("mất ledger: %v", err)
	}
	if err := ltx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result, runErr := storageimport.Run(ctx, storageimport.Options{
		Scan: scan, Pool: pool, Blobs: blobs, Driver: "local", ReadOnlyRoot: root,
		OwnerMap: map[string]string{"du-an": "tac-gia", "_": "quan-tri"},
	})
	found := false
	if result != nil {
		for _, c := range result.Conflicts {
			if strings.Contains(c, "chapter_draft") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("phải báo xung đột bản nháp (runErr=%v, conflicts=%v)", runErr, result.Conflicts)
	}
	var draft string
	if err := pool.Pool().QueryRow(ctx, `
SELECT draft_text FROM chapters
WHERE number = 2 AND story_id IN (SELECT id FROM stories WHERE slug = '01-dem-trang')`).Scan(&draft); err != nil {
		t.Fatal(err)
	}
	if draft != "nháp sửa sau khi nhập" {
		t.Fatalf("bản nháp trong DB bị ghi đè: %q", draft)
	}
}

// TestLegacyProseRoundTripsNormalizedDigest imports prose-only legacy content
// carrying a BOM and CRLF line endings: the synthesized baseline revision must
// satisfy the content/hash invariant the exporter checks.
func TestLegacyProseRoundTripsNormalizedDigest(t *testing.T) {
	root := t.TempDir()
	copyTree(t, fixtureRoot, root)
	prose := filepath.Join(root, "truyen-cu", "output", "novel", "chapters", "01.md")
	content := "\ufeff# Dòng đầu\r\n\r\nDòng hai.\r\n"
	if err := os.WriteFile(prose, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	scan := scanFixture(t, root, fixtureAccounts)
	pool, blobs := openTargets(t)
	runImport(t, scan, root, pool, blobs)

	destination := t.TempDir()
	if _, err := storageimport.ExportFiles(context.Background(), storageimport.ExportOptions{
		SourceID: fixtureSourceID, Destination: destination, Pool: pool, Blobs: blobs,
	}); err != nil {
		t.Fatalf("export: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "truyen-cu", "output", "novel", "chapters", "01.md"))
	if err != nil || string(got) != content {
		t.Fatalf("văn chương khác nguồn sau export (err=%v)", err)
	}
}

func readJSONFile(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("%s hỏng: %v", path, err)
	}
}

func projectIDBySlug(t *testing.T, pool *postgres.Pool, slug string) string {
	t.Helper()
	var id string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT id::text FROM projects WHERE slug=$1`, slug).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func libraryProjectFile() library.ProjectFile {
	return library.ProjectFile{Name: "Dự án sau cutover", Description: "tạo sau khi nhập"}
}

// TestManifestRoundTrip pins that the manifest written to disk hashes to the
// same value it was created with.
func TestManifestRoundTrip(t *testing.T) {
	scan := scanFixture(t, fixtureRoot, fixtureAccounts)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := storageimport.WriteManifest(path, scan.Manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := storageimport.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SHA256 != scan.Manifest.SHA256 || len(loaded.Files) != len(scan.Manifest.Files) {
		t.Fatalf("manifest không round-trip: %s vs %s", loaded.SHA256, scan.Manifest.SHA256)
	}
	data, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) == "" {
		t.Fatal("unreachable")
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
}

func ioReadAll(rc interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var out []byte
	buf := make([]byte, 32*1024)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			return out, err
		}
	}
}
