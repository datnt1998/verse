package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func draftSlug(id string) (string, error) {
	if !library.ValidDraftID(id) {
		return "", fmt.Errorf("%w: %q", library.ErrInvalidName, id)
	}
	return "draft-" + id, nil
}

// DraftDir checks a draft's existence and membership without inventing a path.
func (r *LibraryRepository) DraftDir(ctx context.Context, actor *library.Actor, project, id string) (string, error) {
	slug, err := draftSlug(id)
	if err != nil {
		return "", err
	}
	err = r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		_, err = q.GetDraftBySlug(ctx, gen.GetDraftBySlugParams{ProjectID: p.ID, Slug: slug})
		if noRows(err) {
			return errs.ErrStorageNotFound
		}
		return sanitizeErr(err)
	})
	return "", err
}

func (r *LibraryRepository) CreateDraft(ctx context.Context, actor *library.Actor, project, id string) (string, error) {
	slug, err := draftSlug(id)
	if err != nil {
		return "", err
	}
	err = r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		if _, err = q.LockProjectRow(ctx, p.ID); err != nil {
			return sanitizeErr(err)
		}
		if _, err = q.GetDraftBySlug(ctx, gen.GetDraftBySlugParams{ProjectID: p.ID, Slug: slug}); err == nil {
			return nil
		} else if !noRows(err) {
			return sanitizeErr(err)
		}
		uuid, err := randomUUID()
		if err != nil {
			return err
		}
		_, err = q.InsertStory(ctx, gen.InsertStoryParams{ProjectID: p.ID, ID: uuid, Slug: slug, Status: "draft", Tags: []string{}})
		if err != nil {
			return libConstraintErr(err)
		}
		// Seed the idle lease in the same transaction. Removal/promotion can
		// then serialize even with the very first worker's claim, not just
		// workers whose lease row already existed.
		_, err = tx.Exec(ctx, `INSERT INTO story_leases(project_id,story_id) VALUES($1,$2)`, p.ID, uuid)
		return sanitizeErr(err)
	})
	return "", err
}

// ResolveDraft is deliberately separate from public story resolution. Only the
// gateway's authorized draft-worker launch path should request this scope.
func (n *Narrative) ResolveDraft(ctx context.Context, project, id string, actor *Actor, write bool) (ScopeRef, error) {
	slug, err := draftSlug(id)
	if err != nil {
		return ScopeRef{}, err
	}
	var scope ScopeRef
	err = n.tx(ctx, func(tx pgx.Tx) error {
		lib := &LibraryRepository{pool: n.pool}
		q := gen.New(tx)
		p, err := lib.memberProject(ctx, q, actor, project, write)
		if err != nil {
			return err
		}
		row, err := q.GetDraftBySlug(ctx, gen.GetDraftBySlugParams{ProjectID: p.ID, Slug: slug})
		if noRows(err) {
			return errs.ErrStorageNotFound
		}
		if err != nil {
			return sanitizeErr(err)
		}
		scope = ScopeRef{ProjectID: p.ID, StoryID: row.ID, StorySlug: slug, Draft: true}
		return nil
	})
	return scope, err
}

// lockDraftLease serializes promotion/removal with all fenced writes. Promotion
// requires a stopped worker; removal revokes its fence even on another server.
func lockDraftLease(ctx context.Context, tx pgx.Tx, p, s pgtype.UUID, promote bool) error {
	var active bool
	err := tx.QueryRow(ctx, `SELECT holder_id IS NOT NULL AND lease_until > clock_timestamp() FROM story_leases WHERE project_id=$1 AND story_id=$2 FOR UPDATE`, p, s).Scan(&active)
	if err != nil && !noRows(err) {
		return sanitizeErr(err)
	}
	if promote && active {
		return fmt.Errorf("phiên nháp đang hoạt động: %w", errs.ErrStorageConflict)
	}
	_, err = tx.Exec(ctx, `DELETE FROM story_leases WHERE project_id=$1 AND story_id=$2`, p, s)
	return sanitizeErr(err)
}

func (r *LibraryRepository) CommitDraft(ctx context.Context, actor *library.Actor, project, id, name string) (library.Story, bool, error) {
	slug, err := draftSlug(id)
	if err != nil {
		return library.Story{}, false, err
	}
	var out library.Story
	var temp bool
	err = r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		// Same lock and numbering algorithm as CreateStory. All committers serialize.
		if _, err = q.LockProjectRow(ctx, p.ID); err != nil {
			return sanitizeErr(err)
		}
		row, err := q.LockDraftBySlug(ctx, gen.LockDraftBySlugParams{ProjectID: p.ID, Slug: slug})
		if noRows(err) {
			return errs.ErrStorageNotFound
		}
		if err != nil {
			return sanitizeErr(err)
		}
		if err = lockDraftLease(ctx, tx, p.ID, row.ID, true); err != nil {
			return err
		}
		rows, err := q.ListStoriesByProject(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		target, isTemp := storySlug(name, nextStoryNumber(rows))
		temp = isTemp
		promoted, err := q.PromoteDraft(ctx, gen.PromoteDraftParams{ProjectID: p.ID, ID: row.ID, Slug: target, Title: strings.TrimSpace(name)})
		if err != nil {
			return libConstraintErr(err)
		}
		out = library.Story{Project: project, Story: target, ID: project + "/" + target, UpdatedAt: tsPtr(promoted.UpdatedAt)}
		return nil
	})
	return out, temp, err
}

func removeDraftRows(ctx context.Context, tx pgx.Tx, p, s pgtype.UUID) error {
	if err := lockDraftLease(ctx, tx, p, s, false); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL noverse.allow_immutable_write = 'on'`); err != nil {
		return sanitizeErr(err)
	}
	// A cold co-create worker has no chapters, reviews, jobs or accepted history.
	// Dropping document/blob references is sufficient: the existing maintenance
	// GC's grace window and shared-reference checks own physical blob deletion.
	for _, table := range []string{"story_artifacts", "story_events", "checkpoints", "agent_sessions", "story_state", "outlines", "characters", "volumes"} {
		if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE project_id=$1 AND story_id=$2`, p, s); err != nil {
			return sanitizeErr(err)
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM stories WHERE project_id=$1 AND id=$2 AND status='draft'`, p, s)
	return sanitizeErr(err)
}

func (r *LibraryRepository) RemoveDraft(ctx context.Context, actor *library.Actor, project, id string) error {
	slug, err := draftSlug(id)
	if err != nil {
		return err
	}
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		row, err := q.LockDraftBySlug(ctx, gen.LockDraftBySlugParams{ProjectID: p.ID, Slug: slug})
		if noRows(err) {
			return nil
		}
		if err != nil {
			return sanitizeErr(err)
		}
		return removeDraftRows(ctx, tx, p.ID, row.ID)
	})
}

func (r *LibraryRepository) CleanupDrafts(ctx context.Context, actor *library.Actor, maxAge time.Duration) (int, error) {
	scan, err := r.Scan(ctx, actor)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, project := range scan.Projects {
		err = r.tx(ctx, func(tx pgx.Tx) error {
			q := r.q(tx)
			p, err := r.memberProject(ctx, q, libActs(actor), project.ID, true)
			if err != nil {
				return err
			}
			// Saved conversations and live leases protect a long-running session.
			rows, err := tx.Query(ctx, `SELECT s.id FROM stories s WHERE s.project_id=$1 AND s.status='draft' AND s.updated_at < $2
    AND NOT EXISTS(SELECT 1 FROM story_state d WHERE d.project_id=s.project_id AND d.story_id=s.id AND d.updated_at >= $2)
    AND NOT EXISTS(SELECT 1 FROM story_leases l WHERE l.project_id=s.project_id AND l.story_id=s.id AND l.lease_until > clock_timestamp()) FOR NO KEY UPDATE OF s`, p.ID, time.Now().Add(-maxAge))
			if err != nil {
				return sanitizeErr(err)
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[pgtype.UUID])
			if err != nil {
				return sanitizeErr(err)
			}
			for _, id := range ids {
				// A lease may have been claimed after the candidate query's snapshot.
				if err = lockDraftLease(ctx, tx, p.ID, id, true); errors.Is(err, errs.ErrStorageConflict) {
					continue
				} else if err != nil {
					return err
				}
				if err = removeDraftRows(ctx, tx, p.ID, id); err != nil {
					return err
				}
				removed++
			}
			return nil
		})
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}
