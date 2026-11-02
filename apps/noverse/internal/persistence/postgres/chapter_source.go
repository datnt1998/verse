package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// ChapterSource is the committed text identity of one chapter, read under the
// same row lock the revision writer takes. SHA256 is the revision store's
// canonical content digest (domain.ChapterContentSHA256 of the accepted
// content), not the document payload.
type ChapterSource struct {
	ID       pgtype.UUID
	Revision int32
	SHA256   string
}

// LockChapterSourceTx pins the chapter row and reads the exact revision its
// current_revision pointer names. AcceptRecord holds the same row lock
// (LockChapterByID) while it appends a revision and moves the pointer, so a
// caller inside this transaction can bind derived rows to this source without
// a revision landing between the read and the commit.
//
// ok=false means the chapter is gone or has no committed revision (including a
// pointer whose revision row is missing); callers treat that as a retryable
// source change, never as an empty chapter.
func (n *Narrative) LockChapterSourceTx(ctx context.Context, tx pgx.Tx, scope ScopeRef, chapter int) (ChapterSource, bool, error) {
	q := n.q(tx)
	row, err := q.LockChapterByNumber(ctx, gen.LockChapterByNumberParams{
		ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(chapter)})
	if noRows(err) {
		return ChapterSource{}, false, nil
	}
	if err != nil {
		return ChapterSource{}, false, sanitizeErr(err)
	}
	if !row.CurrentRevision.Valid {
		return ChapterSource{}, false, nil
	}
	version, err := q.GetChapterVersion(ctx, gen.GetChapterVersionParams{
		ProjectID: scope.ProjectID, ChapterID: row.ID, RevisionNo: row.CurrentRevision.Int32})
	if noRows(err) {
		return ChapterSource{}, false, nil
	}
	if err != nil {
		return ChapterSource{}, false, sanitizeErr(err)
	}
	return ChapterSource{ID: row.ID, Revision: version.RevisionNo, SHA256: version.Sha256}, true, nil
}

// SaveComposedScriptBundle binds derived audio text to the accepted revision it
// was composed from. Holding the chapter lock until commit prevents a source
// edit from landing between the source check and the script write.
func (a *Audio) SaveComposedScriptBundle(ctx context.Context, scope ScopeRef, ref ChapterRef, composedSHA256 string, script Script, segments []Segment, edited []string, expected int64) (Script, error) {
	var saved Script
	err := a.tx(ctx, func(tx pgx.Tx) error {
		nar := Narrative{pool: a.pool}
		source, ok, err := nar.LockChapterSourceTx(ctx, tx, scope, ref.Number)
		if err != nil {
			return err
		}
		if !ok || source.ID != ref.ID || int(source.Revision) != ref.CurrentRevision || source.SHA256 != composedSHA256 {
			return fmt.Errorf("nguồn chương vừa thay đổi: %w", errs.ErrStorageConflict)
		}
		saved, err = a.SaveScriptBundleTx(ctx, tx, scope, script, segments, edited, expected)
		return err
	})
	return saved, err
}
