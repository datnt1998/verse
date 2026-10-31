package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

func metadataVersionToken(kind string, version int64) string {
	return fmt.Sprintf("pgm-%s-%d", kind, version)
}

// MetadataVersion reads the project/story row version under the same membership
// boundary as its metadata. An empty story selects the project itself.
func (r *LibraryRepository) MetadataVersion(ctx context.Context, actor *library.Actor, project, story string) (string, error) {
	var token string
	err := r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, false)
		if err != nil {
			return err
		}
		if story == "" {
			token = metadataVersionToken("project", p.Version)
			return nil
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", project, story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		token = metadataVersionToken("story", row.Version)
		return nil
	})
	return token, err
}

// UpdateProject is the unconditional repository operation used by internal
// callers. HTTP clients use UpdateProjectCAS to preserve their read version.
func (r *LibraryRepository) UpdateProject(ctx context.Context, actor *library.Actor, project string, f library.ProjectFile) error {
	return r.UpdateProjectCAS(ctx, actor, project, f, "")
}

// UpdateProjectCAS compares and writes inside one row-locked transaction, so
// separate gateway processes cannot both accept the same stale version.
// Empty or "*" expected preserves the existing unconditional-save contract.
func (r *LibraryRepository) UpdateProjectCAS(ctx context.Context, actor *library.Actor, project string, f library.ProjectFile, expected string) error {
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		if _, err := q.LockProjectRow(ctx, p.ID); err != nil {
			if noRows(err) {
				return fmt.Errorf("dự án %s: %w", project, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		// The first read may have preceded another writer. Read again while
		// holding the lock; the row trigger advances version on every write.
		p, err = r.memberProject(ctx, q, libActs(actor), project, true)
		if err != nil {
			return err
		}
		if expected != "" && expected != "*" && expected != metadataVersionToken("project", p.Version) {
			return errs.ErrStorageConflict
		}
		n, err := q.UpdateProjectFile(ctx, gen.UpdateProjectFileParams{
			ID: p.ID, Name: f.Name, Description: f.Description, Notes: f.Notes, UniverseName: f.UniverseName,
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("dự án %s: %w", project, errs.ErrStorageNotFound)
		}
		return nil
	})
}

// WriteStoryFile writes metadata unconditionally for internal callers, keeping
// the same row-lock and field-preservation rules as the HTTP CAS operation.
func (r *LibraryRepository) WriteStoryFile(ctx context.Context, actor *library.Actor, st library.Story, f library.StoryFile) error {
	return r.WriteStoryFileCAS(ctx, actor, st, f, "")
}

// WriteStoryFileCAS compares the client's version after acquiring the story
// row lock. Idea names resolve within the project; synopsis remains unchanged
// because story metadata does not expose that field.
func (r *LibraryRepository) WriteStoryFileCAS(ctx context.Context, actor *library.Actor, st library.Story, f library.StoryFile, expected string) error {
	return r.tx(ctx, func(tx pgx.Tx) error {
		q := r.q(tx)
		p, err := r.memberProject(ctx, q, libActs(actor), st.Project, true)
		if err != nil {
			return err
		}
		row, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: st.Story})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", st.Project, st.Story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		locked, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: p.ID, ID: row.ID})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("truyện %s/%s: %w", st.Project, st.Story, errs.ErrStorageNotFound)
			}
			return sanitizeErr(err)
		}
		if expected != "" && expected != "*" && expected != metadataVersionToken("story", locked.Version) {
			return errs.ErrStorageConflict
		}
		ideaDoc := pgtype.UUID{}
		if name := strings.TrimSpace(f.Idea); name != "" {
			doc, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{ProjectID: p.ID, Kind: "idea", LogicalName: name})
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("ý tưởng %s: %w", name, library.ErrIdeaNotFound)
				}
				return sanitizeErr(err)
			}
			ideaDoc = doc.ID
		}
		status := f.Status
		if status == "" {
			status = "idea"
		}
		tags := f.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err = q.UpdateStoryCAS(ctx, gen.UpdateStoryCASParams{
			ProjectID: p.ID, ID: locked.ID, Slug: st.Story,
			Title: f.Title, ShortTitle: f.ShortTitle, Synopsis: locked.Synopsis,
			TitleRenamed: f.TitleRenamed, Status: status, Tags: tags,
			PublishedAt: pgTimestamptzPtr(f.PublishedAt), IdeaDocumentID: ideaDoc, Version: locked.Version,
		})
		if err != nil {
			if noRows(err) {
				return fmt.Errorf("ghi truyện %s: %w", st.Story, errs.ErrStorageConflict)
			}
			return libConstraintErr(err)
		}
		return nil
	})
}
