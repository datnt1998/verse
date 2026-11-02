package postgres_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// TestPostgresReviewDocumentView pins the save_review checkpoint contract:
// the tool saves the typed review round and then digests its file-backend path
// (reviews/NN[-global].json). That path must render the latest valid row, must
// change when a new round changes the content, and must never be writable as a
// document (rows stay the single source of review truth).
func TestPostgresReviewDocumentView(t *testing.T) {
	s, b, _ := backendFixture(t, "project-review-doc", "truyen-review-doc")

	globalPath := "reviews/05-global.json"
	if _, err := b.Docs().ReadFile(globalPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing review must be not-exist, got %v", err)
	}

	first := domain.ReviewEntry{
		Chapter:          5,
		Scope:            "global",
		Verdict:          "polish",
		Summary:          "cần chỉnh cục bộ",
		AffectedChapters: []int{2, 3},
	}
	if err := s.World.SaveReview(first); err != nil {
		t.Fatalf("save global review: %v", err)
	}

	data, err := b.Docs().ReadFile(globalPath)
	if err != nil {
		t.Fatalf("read review document %s: %v", globalPath, err)
	}
	if !bytes.Contains(data, []byte("\n  \"")) {
		t.Fatalf("review document must be rendered in the file backend's indented shape: %q", data)
	}
	var got domain.ReviewEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse review document: %v", err)
	}
	if got.Chapter != 5 || got.Scope != "global" || got.Verdict != "polish" {
		t.Fatalf("review document = %+v, want chapter 5 global polish", got)
	}
	if exists, err := b.Docs().ExistsUnlocked(globalPath); err != nil || !exists {
		t.Fatalf("review document must exist: %v %v", exists, err)
	}

	cp, err := s.Checkpoints.AppendArtifact(domain.ChapterScope(5), "review", globalPath)
	if err != nil {
		t.Fatalf("review checkpoint over the document view: %v", err)
	}
	if cp == nil || cp.Digest == "" || cp.Artifact != globalPath {
		t.Fatalf("review checkpoint = %+v", cp)
	}

	// A saved review file is a derived read-only view, not a second store.
	if err := b.Docs().WriteFileUnlocked(globalPath, []byte(`{"chapter":5}`)); err == nil {
		t.Fatal("a document write to a review round must be refused")
	}
	if err := b.Docs().RemoveFileUnlocked(globalPath); err == nil {
		t.Fatal("a document delete of a review round must be refused")
	}

	// A new typed round replaces the current review: the digest must follow the
	// content, so an identical checkpoint is not mistaken for the old round.
	second := first
	second.Verdict = "rewrite"
	if err := s.World.SaveReview(second); err != nil {
		t.Fatalf("save second review round: %v", err)
	}
	cp2, err := s.Checkpoints.AppendArtifact(domain.ChapterScope(5), "review", globalPath)
	if err != nil {
		t.Fatalf("second review checkpoint: %v", err)
	}
	if cp2.Digest == cp.Digest {
		t.Fatal("a new review round must change the artifact digest")
	}

	// Chapter and global reviews are distinct paths in the same typed family.
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 5, Scope: "chapter", Verdict: "accept", Summary: "đạt",
	}); err != nil {
		t.Fatalf("save chapter review: %v", err)
	}
	chapterData, err := b.Docs().ReadFile("reviews/05.json")
	if err != nil {
		t.Fatalf("read chapter review document: %v", err)
	}
	var chapterReview domain.ReviewEntry
	if err := json.Unmarshal(chapterData, &chapterReview); err != nil {
		t.Fatalf("parse chapter review document: %v", err)
	}
	if chapterReview.Scope != "chapter" || chapterReview.Verdict != "accept" {
		t.Fatalf("chapter review document = %+v", chapterReview)
	}

	ref, err := postgres.ResolveDocument(globalPath)
	if err != nil {
		t.Fatalf("resolve review path: %v", err)
	}
	if ref.Class != postgres.DocClassReview || ref.Chapter != 5 || ref.Key != "05-global" {
		t.Fatalf("review document ref = %+v", ref)
	}
}
