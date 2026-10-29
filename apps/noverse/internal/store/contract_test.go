package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

// ── storage-contract manifest ────────────────────────────────────────────────
//
// manifest.json is the phase-01 inventory: every persistent path class the
// file backend owns is classified facts/blob/derived/scratch/local-only/
// excluded. The gate "no unknown paths" is enforced by rejecting any entry
// whose class is not in the declared class set or lacks a reason.

const contractFixtureRoot = "testdata/storage-contract"

type contractManifest struct {
	Version     int               `json:"version"`
	PathClasses map[string]string `json:"pathClasses"`
	Paths       []struct {
		Path   string `json:"path"`
		Class  string `json:"class"`
		Reason string `json:"reason"`
	} `json:"paths"`
	Coverage struct {
		Required      []string `json:"required"`
		SourceSymlink string   `json:"source-symlink"`
	} `json:"coverage"`
	Fixtures []struct {
		Path   string   `json:"path"`
		SHA256 string   `json:"sha256"`
		Class  string   `json:"class"`
		Covers []string `json:"covers"`
	} `json:"fixtures"`
}

func loadContractManifest(t *testing.T) contractManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractFixtureRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest must exist: %v", err)
	}
	var m contractManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest must parse: %v", err)
	}
	return m
}

func copyFixtureTree(t *testing.T, srcRel string) string {
	t.Helper()
	src := filepath.Join(contractFixtureRoot, srcRel)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", srcRel, err)
	}
	return dst
}

func TestStorageContractFile(t *testing.T) {
	m := loadContractManifest(t)

	t.Run("manifest_classifies_every_path", func(t *testing.T) {
		if len(m.Paths) == 0 {
			t.Fatal("manifest lists no paths")
		}
		for _, p := range m.Paths {
			if p.Path == "" {
				t.Fatal("manifest entry with empty path")
			}
			if _, ok := m.PathClasses[p.Class]; !ok {
				t.Fatalf("path %q has unknown class %q", p.Path, p.Class)
			}
			if strings.TrimSpace(p.Reason) == "" {
				t.Fatalf("path %q lacks a reason", p.Path)
			}
		}
	})

	t.Run("fixtures_exist_and_hash", func(t *testing.T) {
		listed := map[string]bool{}
		for _, f := range m.Fixtures {
			listed[f.Path] = true
			raw, err := os.ReadFile(filepath.Join(contractFixtureRoot, f.Path))
			if err != nil {
				t.Fatalf("fixture %s missing: %v", f.Path, err)
			}
			sum := sha256.Sum256(raw)
			if got := hex.EncodeToString(sum[:]); got != f.SHA256 {
				t.Fatalf("fixture %s hash drifted: manifest %s, got %s", f.Path, f.SHA256, got)
			}
			if _, ok := m.PathClasses[f.Class]; !ok {
				t.Fatalf("fixture %s has unknown class %q", f.Path, f.Class)
			}
		}
		// every file under the fixture root must be listed (except manifest itself)
		var unlisted []string
		root := filepath.Join(contractFixtureRoot)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel != "manifest.json" && !listed[rel] {
				unlisted = append(unlisted, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(unlisted) > 0 {
			sort.Strings(unlisted)
			t.Fatalf("fixture files missing from manifest: %v", unlisted)
		}
	})

	t.Run("coverage_required", func(t *testing.T) {
		covered := map[string]bool{}
		for _, f := range m.Fixtures {
			for _, c := range f.Covers {
				covered[c] = true
			}
		}
		for _, req := range m.Coverage.Required {
			if req == "source-symlink" {
				// created at test time via os.Symlink in library/contract_test.go
				continue
			}
			if !covered[req] {
				t.Fatalf("required coverage %q has no fixture", req)
			}
		}
	})

	engine := copyFixtureTree(t, "library/du-an/kich_ban/01-dem-trang/output/novel")

	t.Run("absent_vs_corrupt", func(t *testing.T) {
		s := NewStore(engine)
		// present book.json loads
		if book, err := s.Book.Load(); err != nil || book == nil {
			t.Fatalf("fixture book must load: %v %+v", err, book)
		}
		// absent book.json → nil, nil (not an error)
		if err := os.Remove(filepath.Join(engine, "meta", "book.json")); err != nil {
			t.Fatal(err)
		}
		if book, err := s.Book.Load(); err != nil || book != nil {
			t.Fatalf("absent book.json must be (nil,nil), got %v %+v", err, book)
		}
		// corrupt book.json → error, never a silent nil
		if err := os.WriteFile(filepath.Join(engine, "meta", "book.json"), []byte(`{"title":`), 0o644); err != nil {
			t.Fatal(err)
		}
		if book, err := s.Book.Load(); err == nil || book != nil {
			t.Fatalf("corrupt book.json must error, got %v %+v", err, book)
		} else if os.IsNotExist(err) {
			t.Fatalf("corrupt must not be reported as absent: %v", err)
		}
		// absent chapter record → nil, nil
		if rec, err := s.ChapterRecords.Load(99); err != nil || rec != nil {
			t.Fatalf("absent record must be (nil,nil): %v %+v", err, rec)
		}
	})

	t.Run("project_format_absent_is_v1", func(t *testing.T) {
		if err := os.Remove(filepath.Join(engine, "meta", "format.json")); err != nil {
			t.Fatal(err)
		}
		s := NewStore(engine)
		if v, err := s.LoadProjectFormatVersion(); err != nil || v != LegacyProjectFormatVersion {
			t.Fatalf("absent format must be v1: %d %v", v, err)
		}
		if err := s.SaveProjectFormatVersion(CurrentProjectFormatVersion); err != nil {
			t.Fatal(err)
		}
		if v, err := s.LoadProjectFormatVersion(); err != nil || v != CurrentProjectFormatVersion {
			t.Fatalf("saved format must be v2: %d %v", v, err)
		}
	})

	t.Run("fixture_baseline_facts", func(t *testing.T) {
		s := NewStore(engine)
		progress, err := s.Progress.Load()
		if err != nil || progress == nil {
			t.Fatalf("progress must load: %v", err)
		}
		if progress.Phase != domain.PhaseWriting || progress.CurrentChapter != 2 ||
			len(progress.CompletedChapters) != 1 || progress.CompletedChapters[0] != 1 {
			t.Fatalf("unexpected progress: %+v", progress)
		}
		rec, err := s.ChapterRecords.Load(1)
		if err != nil || rec == nil {
			t.Fatalf("record 1 must load: %v", err)
		}
		if rec.Chapter != 1 || rec.Revision != 1 || rec.Origin != domain.ChapterOriginGenerated || rec.Facts.Title != "Đêm thứ nhất" {
			t.Fatalf("unexpected record: %+v", rec)
		}
		// envelope: stored JSON keys are snake_case, chapter stored not inferred
		raw, err := os.ReadFile(filepath.Join(engine, "meta", "chapter_records", "000001.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"content_sha256"`) || !strings.Contains(string(raw), `"chapter": 1`) {
			t.Fatalf("record envelope must keep snake_case keys: %s", raw)
		}
		entries, err := s.Outline.LoadOutline()
		if err != nil || len(entries) != 4 {
			t.Fatalf("outline must load 4 entries: %v %d", err, len(entries))
		}
	})

	t.Run("deterministic_order_and_idempotent_append", func(t *testing.T) {
		s := NewStore(engine)
		all := s.Checkpoints.All()
		if len(all) != 3 {
			t.Fatalf("fixture checkpoints must load: %d", len(all))
		}
		for i := 1; i < len(all); i++ {
			if all[i].Seq <= all[i-1].Seq {
				t.Fatalf("checkpoint order must be deterministic ascending: %+v", all)
			}
		}
		// same-input append is idempotent: seq 2 has digest "chuong-1-v1"
		cp, err := s.Checkpoints.Append(all[1].Scope, all[1].Step, all[1].Artifact, all[1].Digest)
		if err != nil {
			t.Fatal(err)
		}
		if cp.Seq != all[1].Seq {
			t.Fatalf("idempotent append must return existing seq %d, got %d", all[1].Seq, cp.Seq)
		}
		if n := len(s.Checkpoints.All()); n != 3 {
			t.Fatalf("idempotent append must not grow the log: %d", n)
		}
	})

	t.Run("partial_jsonl_tail_recovered", func(t *testing.T) {
		s := NewStore(engine)
		recs, err := s.Decisions.Recent(0)
		if err != nil {
			t.Fatalf("uncommitted tail must be truncated, not fatal: %v", err)
		}
		if len(recs) != 2 {
			t.Fatalf("only committed lines may be returned: %d", len(recs))
		}
		raw, err := os.ReadFile(filepath.Join(engine, "meta", "decisions.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(raw), "\n") || strings.Contains(string(raw), `"seq":3`) {
			t.Fatalf("uncommitted tail must be discarded: %s", raw)
		}
	})

	t.Run("vietnamese_error_strings", func(t *testing.T) {
		s := NewStore(engine)
		if err := s.SaveProjectFormatVersion(0); err == nil || !strings.Contains(err.Error(), "Phiên bản định dạng dự án") {
			t.Fatalf("format error must keep Vietnamese message: %v", err)
		}
	})
}

func TestStorageAuthority(t *testing.T) {
	engine := copyFixtureTree(t, "library/du-an/kich_ban/01-dem-trang/output/novel")
	s := NewStore(engine)

	t.Run("book_json_is_facts_book_md_is_derived", func(t *testing.T) {
		// tamper the projection: the authoritative title must not change
		mdPath := filepath.Join(engine, "book.md")
		if err := os.WriteFile(mdPath, []byte("# Tiêu Đề Giả\n\nTóm tắt giả.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		book, err := s.Book.Load()
		if err != nil || book == nil {
			t.Fatal(err)
		}
		if book.Title != "Đêm Trắng Trên Kệ Sách" {
			t.Fatalf("title must come from meta/book.json: %q", book.Title)
		}
		// deleting the projection entirely must not affect the facts
		if err := os.Remove(mdPath); err != nil {
			t.Fatal(err)
		}
		if book, err := s.Book.Load(); err != nil || book == nil || book.Title != "Đêm Trắng Trên Kệ Sách" {
			t.Fatalf("missing projection must not change facts: %v %+v", err, book)
		}
	})

	t.Run("save_regenerates_projection", func(t *testing.T) {
		book, err := s.Book.Load()
		if err != nil || book == nil {
			t.Fatal(err)
		}
		if err := s.Book.Save(*book); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(engine, "book.md")); err != nil {
			t.Fatalf("save must regenerate the projection: %v", err)
		}
	})

	t.Run("chapter_record_is_baseline_not_workspace_prose", func(t *testing.T) {
		// the workspace chapter file and the accepted record differ in bytes;
		// the record wins for Load, the workspace stays user-editable
		rec, err := s.ChapterRecords.Load(1)
		if err != nil || rec == nil {
			t.Fatal(err)
		}
		prose, err := os.ReadFile(filepath.Join(engine, "chapters", "01.md"))
		if err != nil {
			t.Fatal(err)
		}
		if rec.Content == string(prose) {
			t.Fatal("fixture must keep record/workspace bytes distinct (trailing newline)")
		}
		if strings.TrimSpace(rec.Content) != strings.TrimSpace(string(prose)) {
			t.Fatalf("record baseline and chapter file disagree: %q vs %q", rec.Content, prose)
		}
		// edited, unaccepted draft of chapter 2 does not create a record
		if _, err := os.Stat(filepath.Join(engine, "drafts", "02.draft.md")); err != nil {
			t.Fatalf("edited draft fixture must exist: %v", err)
		}
		if rec2, err := s.ChapterRecords.Load(2); err != nil || rec2 != nil {
			t.Fatalf("unaccepted draft must not be an accepted record: %v %+v", err, rec2)
		}
	})

	t.Run("pending_revision_absent_is_nil", func(t *testing.T) {
		if p, err := s.Revisions.LoadPending(); err != nil || p != nil {
			t.Fatalf("absent pending revision must be (nil,nil): %v %+v", err, p)
		}
	})

	t.Run("record_chapter_mismatch_rejected", func(t *testing.T) {
		// a record whose chapter field disagrees with its filename must error
		sum := sha256.Sum256([]byte("x"))
		bad := `{"version":1,"chapter":3,"revision":1,"origin":"generated","content":"x","content_sha256":"` +
			hex.EncodeToString(sum[:]) + `","facts":{},"style_delta":{},"accepted_at":"2026-01-06T12:00:00Z"}`
		path := filepath.Join(engine, "meta", "chapter_records", "000007.json")
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ChapterRecords.Load(7); err == nil || !strings.Contains(err.Error(), "bản ghi ghi số chương") {
			t.Fatalf("chapter mismatch must error in Vietnamese: %v", err)
		}
	})
}
