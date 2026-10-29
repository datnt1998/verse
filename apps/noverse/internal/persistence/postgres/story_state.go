package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/store"
)

// Các kind story_state được domain dùng ở phase này — CHECK trong migration
// phải chứa từng giá trị; thêm kind mới = migration mới, không ghi tràn kind.
const (
	stateKindBook            = "book"
	stateKindProgress        = "progress"
	stateKindPendingRevision = "pending_revision"
	stateKindCommentEvents   = "comment_events"
)

// loadState đọc một singleton typed; thiếu → found=false (absent ≠ lỗi).
func (n *Narrative) loadState(ctx context.Context, q *gen.Queries, scope ScopeRef, kind string, dst any) (bool, error) {
	row, err := q.GetStoryState(ctx, gen.GetStoryStateParams{
		ProjectID: scope.ProjectID, StoryID: scope.StoryID, Kind: kind})
	if err != nil {
		if noRows(err) {
			return false, nil
		}
		return false, sanitizeErr(err)
	}
	if err := json.Unmarshal(row.Payload, dst); err != nil {
		return false, fmt.Errorf("đọc %s: %w", kind, err)
	}
	return true, nil
}

func (n *Narrative) putState(ctx context.Context, q *gen.Queries, scope ScopeRef, kind string, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return sanitizeErr(q.UpsertStoryState(ctx, gen.UpsertStoryStateParams{
		ProjectID:     scope.ProjectID,
		StoryID:       scope.StoryID,
		Kind:          kind,
		Payload:       payload,
		SchemaVersion: 1,
	}))
}

// BookLoad đọc meta/book.json parity: thiếu → (nil,nil). Title/synopsis trong
// story_state.book là facts; stories.title chỉ là display sync một chiều
// (title_renamed chặn đồng bộ — BookSave cập nhật display trong cùng tx).
func (n *Narrative) BookLoad(ctx context.Context, scope ScopeRef) (*domain.BookMetadata, error) {
	var book *domain.BookMetadata
	err := n.tx(ctx, func(tx pgx.Tx) error {
		var b domain.BookMetadata
		found, err := n.loadState(ctx, n.q(tx), scope, stateKindBook, &b)
		if err != nil {
			return err
		}
		if found {
			nb := b.Normalized()
			book = &nb
		}
		return nil
	})
	return book, err
}

// BookSave ghi facts + đồng bộ display title một chiều khi title_renamed=false
// (story row CAS: dùng version đọc trong cùng tx nên update luôn thắng —
// rename user đã khoá title_renamed=true thì chỉ ghi facts).
func (n *Narrative) BookSave(ctx context.Context, scope ScopeRef, book domain.BookMetadata) error {
	book = book.Normalized()
	if err := book.Validate(); err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if err := n.putState(ctx, q, scope, stateKindBook, book); err != nil {
			return err
		}
		row, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		if row.TitleRenamed || (row.Title == book.Title && row.Synopsis == book.Synopsis) {
			return nil
		}
		_, err = q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID:      scope.ProjectID,
			ID:             scope.StoryID,
			Slug:           row.Slug,
			Title:          book.Title,
			ShortTitle:     row.ShortTitle,
			Synopsis:       book.Synopsis,
			TitleRenamed:   row.TitleRenamed,
			Status:         row.Status,
			Tags:           row.Tags,
			PublishedAt:    row.PublishedAt,
			IdeaDocumentID: row.IdeaDocumentID,
			Version:        row.Version,
		})
		return sanitizeErr(err)
	})
}

// ProgressLoad đọc progress index; thiếu → (nil,nil) parity file.
func (n *Narrative) ProgressLoad(ctx context.Context, scope ScopeRef) (*domain.Progress, error) {
	var progress *domain.Progress
	err := n.tx(ctx, func(tx pgx.Tx) error {
		var p domain.Progress
		found, err := n.loadState(ctx, n.q(tx), scope, stateKindProgress, &p)
		if err != nil {
			return err
		}
		if found {
			progress = &p
		}
		return nil
	})
	return progress, err
}

// ProgressSave ghi progress index.
func (n *Narrative) ProgressSave(ctx context.Context, scope ScopeRef, p *domain.Progress) error {
	if p == nil {
		return fmt.Errorf("progress nil: %w", errs.ErrConfig)
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.putState(ctx, n.q(tx), scope, stateKindProgress, *p)
	})
}

// PendingRevisionLoad đọc marker pending-revision (external edits);
// thiếu → (nil,nil) parity file Revisions.LoadPending.
func (n *Narrative) PendingRevisionLoad(ctx context.Context, scope ScopeRef) (*domain.PendingRevision, error) {
	var pending *domain.PendingRevision
	err := n.tx(ctx, func(tx pgx.Tx) error {
		var p domain.PendingRevision
		found, err := n.loadState(ctx, n.q(tx), scope, stateKindPendingRevision, &p)
		if err != nil {
			return err
		}
		if found {
			pending = &p
		}
		return nil
	})
	return pending, err
}

// PendingRevisionSave ghi marker pending-revision.
func (n *Narrative) PendingRevisionSave(ctx context.Context, scope ScopeRef, pending domain.PendingRevision) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		return n.putState(ctx, n.q(tx), scope, stateKindPendingRevision, pending)
	})
}

// PendingRevisionClear xoá marker — parity ClearPending (xoá file).
func (n *Narrative) PendingRevisionClear(ctx context.Context, scope ScopeRef) error {
	return n.tx(ctx, func(tx pgx.Tx) error {
		return sanitizeErr(n.q(tx).DeleteStoryState(ctx, gen.DeleteStoryStateParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Kind: stateKindPendingRevision}))
	})
}

// ── checkpoints / decisions (append-only streams) ────────────────────────────

// CheckpointAppend giữ idempotence của file backend: cùng
// (scope, step, artifact, digest) trả bản ghi cũ thay vì append mới — seq
// chỉ tăng trên nội dung mới, dưới khoá hàng story để serialize.
func (n *Narrative) CheckpointAppend(ctx context.Context, scope ScopeRef, sc domain.Scope, step, artifact, digest string) (*domain.Checkpoint, error) {
	var out *domain.Checkpoint
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		// Story row lock serializes seq allocation across writers.
		if _, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID}); err != nil {
			if noRows(err) {
				return errs.ErrStorageNotFound
			}
			return sanitizeErr(err)
		}
		dup, err := q.GetCheckpointByTuple(ctx, gen.GetCheckpointByTupleParams{
			ProjectID:   scope.ProjectID,
			StoryID:     scope.StoryID,
			ScopeKey:    sc.String(),
			Step:        step,
			ArtifactRef: artifact,
			Sha256:      digest,
		})
		if err == nil {
			cp := checkpointFromRow(sc, dup)
			out = &cp
			return nil
		}
		if !noRows(err) {
			return sanitizeErr(err)
		}
		payload, err := json.Marshal(map[string]any{"scope": sc, "step": step, "artifact": artifact, "digest": digest})
		if err != nil {
			return err
		}
		row, err := q.AppendCheckpoint(ctx, gen.AppendCheckpointParams{
			ProjectID:   scope.ProjectID,
			StoryID:     scope.StoryID,
			ScopeKey:    sc.String(),
			Step:        step,
			ArtifactRef: artifact,
			Sha256:      digest,
			OccurredAt:  pgtype.Timestamptz{Time: nowUTC(), Valid: true},
			Payload:     payload,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		cp := checkpointFromRow(sc, row)
		out = &cp
		return nil
	})
	return out, err
}

func nowUTC() time.Time { return time.Now().UTC() }

func checkpointFromRow(sc domain.Scope, r gen.Checkpoint) domain.Checkpoint {
	// scope is authoritative from scope_key (string form round-trips); parse
	// back via the payload for fidelity.
	var doc struct {
		Scope domain.Scope `json:"scope"`
	}
	_ = json.Unmarshal(r.Payload, &doc)
	if doc.Scope.Kind != "" {
		sc = doc.Scope
	}
	return domain.Checkpoint{
		Seq:        r.Seq,
		Scope:      sc,
		Step:       r.Step,
		Artifact:   r.ArtifactRef,
		Digest:     r.Sha256,
		OccurredAt: r.OccurredAt.Time.UTC(),
	}
}

// Checkpoints đọc toàn bộ log theo seq (deterministic ascending).
func (n *Narrative) Checkpoints(ctx context.Context, scope ScopeRef) ([]domain.Checkpoint, error) {
	out := []domain.Checkpoint{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		rows, err := n.q(tx).ListCheckpoints(ctx, gen.ListCheckpointsParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range rows {
			out = append(out, checkpointFromRow(domain.Scope{}, r))
		}
		return nil
	})
	return out, err
}

// DecisionAppend ghi một bản ghi decisions stream (append-only); seq gán bởi
// DB trong cùng câu INSERT — thứ tự tuyến tính theo stream.
func (n *Narrative) DecisionAppend(ctx context.Context, scope ScopeRef, rec store.DecisionRecord) (store.DecisionRecord, error) {
	var out store.DecisionRecord
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		payload, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		seq, err := q.AppendStoryEvent(ctx, gen.AppendStoryEventParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			Stream:    "decisions",
			Payload:   payload,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		out = rec
		if out.ID == "" {
			out.ID = fmt.Sprintf("dec-%08d", seq)
		}
		return nil
	})
	return out, err
}

// DecisionsRecent trả n bản ghi decisions mới nhất theo seq giảm (n<=0 = tất cả).
func (n *Narrative) DecisionsRecent(ctx context.Context, scope ScopeRef, count int) ([]store.DecisionRecord, error) {
	out := []store.DecisionRecord{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		limit := int32(count)
		if limit <= 0 {
			limit = 1 << 20
		}
		rows, err := q.ListStoryEvents(ctx, gen.ListStoryEventsParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			Stream:    "decisions",
			Seq:       0,
			Limit:     limit,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		for i := range rows {
			var rec store.DecisionRecord
			if err := json.Unmarshal(rows[i].Payload, &rec); err != nil {
				return fmt.Errorf("đọc decisions: %w", err)
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}
