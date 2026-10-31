package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// ConfigDocuments is the hosted home for the two config.json layers the file
// backend keeps under <project>/.ainovel and <story>/.ainovel (GAP-10 chain):
//
//   - project scope → project_documents (kind 'config', logical_name
//     '.ainovel/config.json'), mirroring the importer's canonical mapping so
//     reverse export restores the file at the same path;
//   - story scope → story_state (kind 'config', payload jsonb). The row guard
//     bumps version on every write, giving ETag tokens for /api/config CAS.
//
// The global layer (~/.ainovel/config.json) stays on the server filesystem —
// provider keys never enter the row store.
//
// Membership is re-checked inside every transaction (memberProjectRole), so a
// stale session grant cannot write past the handler's own authorization. The
// role floor matches the file backend's config contract: reads need project
// membership, project-scope writes need owner, story-scope writes need
// collab — the gateway additionally hides story targets it cannot write
// (checkConfigWrite → 404) exactly like a missing story.
type ConfigDocuments struct {
	lib *LibraryRepository
}

// NewConfigDocuments builds the scoped config store on the shared pool.
func NewConfigDocuments(p *Pool) *ConfigDocuments {
	return &ConfigDocuments{lib: NewLibraryRepository(p)}
}

// ConfigDocuments exposes scoped configuration on a hosted library.
func (r *LibraryRepository) ConfigDocuments() *ConfigDocuments {
	return &ConfigDocuments{lib: r}
}

// projectConfigLogicalName is the importer/export canonical name for
// <project>/.ainovel/config.json inside project_documents.
const projectConfigLogicalName = ".ainovel/config.json"

// memberScope resolves (project, story) inside the transaction and returns the
// actor's project role. Story resolution is a plain row read: the story is
// only reachable through a member-visible project.
func (r *ConfigDocuments) memberScope(ctx context.Context, q *gen.Queries, actor *Actor, projectSlug, storySlug string) (gen.Project, gen.Story, string, error) {
	var story gen.Story
	p, role, err := r.lib.memberProjectRole(ctx, q, actor, projectSlug)
	if err != nil {
		return p, story, role, err
	}
	if storySlug == "" {
		return p, story, role, nil
	}
	s, err := q.GetStoryBySlug(ctx, gen.GetStoryBySlugParams{ProjectID: p.ID, Slug: storySlug})
	if err != nil {
		if noRows(err) {
			return p, story, role, fmt.Errorf("truyện %s/%s: %w", projectSlug, storySlug, errs.ErrStorageNotFound)
		}
		return p, story, role, sanitizeErr(err)
	}
	return p, s, role, nil
}

// requireWriteRole applies the config write floor: project layer needs
// 'owner', story layer needs 'collab'. A caller lacking the role gets
// ErrStorageForbidden — the HTTP layer maps its own visibility rules on top.
func (r *ConfigDocuments) requireWriteRole(role, storySlug string) error {
	switch {
	case storySlug == "" && role != "owner":
		return fmt.Errorf("cấu hình dự án: %w", errs.ErrStorageForbidden)
	case storySlug != "" && role != "owner" && role != "collab":
		return fmt.Errorf("cấu hình truyện: %w", errs.ErrStorageForbidden)
	}
	return nil
}

// Get reads one layer as its JSON object bytes. storySlug "" is the project
// layer. A missing document returns (nil, 0, nil); the payload is always a
// JSON object (or JSON text for the project layer, which callers treat the
// same).
func (r *ConfigDocuments) Get(ctx context.Context, actor *Actor, projectSlug, storySlug string) (json.RawMessage, int64, error) {
	var data []byte
	var version int64
	err := r.lib.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		p, story, _, err := r.memberScope(ctx, q, actor, projectSlug, storySlug)
		if err != nil {
			return err
		}
		if storySlug == "" {
			doc, derr := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{
				ProjectID: p.ID, Kind: "config", LogicalName: projectConfigLogicalName,
			})
			if derr != nil {
				if noRows(derr) {
					return nil
				}
				return sanitizeErr(derr)
			}
			data = []byte(doc.TextContent.String)
			version = doc.Version
			return nil
		}
		row, serr := q.GetStoryState(ctx, gen.GetStoryStateParams{
			ProjectID: p.ID, StoryID: story.ID, Kind: "config",
		})
		if serr != nil {
			if noRows(serr) {
				return nil
			}
			return sanitizeErr(serr)
		}
		data = row.Payload
		version = row.Version
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return json.RawMessage(data), version, nil
}

// Put writes one layer inside a transaction that re-checks the actor's write
// role. body must already be a valid JSON object (the gateway validates the
// full Config contract before calling). The write bumps the row version so
// Version() tokens stay honest for CAS clients.
func (r *ConfigDocuments) Put(ctx context.Context, actor *Actor, projectSlug, storySlug string, body []byte, expected string) error {
	if len(body) == 0 {
		body = []byte(`{}`)
	}
	if !json.Valid(body) {
		return fmt.Errorf("cấu hình không phải JSON hợp lệ: %w", errs.ErrConfig)
	}
	return r.lib.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		p, story, role, err := r.memberScope(ctx, q, actor, projectSlug, storySlug)
		if err != nil {
			return err
		}
		if err := r.requireWriteRole(role, storySlug); err != nil {
			return err
		}
		// Lock the parent even for a missing document, so competing creates
		// and updates across gateway processes share the same CAS boundary.
		if storySlug == "" {
			_, err = tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, p.ID)
		} else {
			_, err = tx.Exec(ctx, `SELECT id FROM stories WHERE project_id=$1 AND id=$2 FOR UPDATE`, p.ID, story.ID)
		}
		if err != nil {
			return sanitizeErr(err)
		}
		if expected != "" && expected != "*" {
			var current int64
			if storySlug == "" {
				err = tx.QueryRow(ctx, `SELECT version FROM project_documents WHERE project_id=$1 AND kind='config' AND logical_name=$2`, p.ID, projectConfigLogicalName).Scan(&current)
			} else {
				err = tx.QueryRow(ctx, `SELECT version FROM story_state WHERE project_id=$1 AND story_id=$2 AND kind='config'`, p.ID, story.ID).Scan(&current)
			}
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
			if expected != fmt.Sprintf("pgc-%d", current) {
				return errs.ErrStorageConflict
			}
		}
		if storySlug == "" {
			_, err = q.UpsertProjectDocument(ctx, gen.UpsertProjectDocumentParams{
				ProjectID:   p.ID,
				ID:          deterministicUUID("config", UUIDString(p.ID)+"/"+projectConfigLogicalName),
				Kind:        "config",
				LogicalName: projectConfigLogicalName,
				TextContent: pgtype.Text{String: string(body), Valid: true},
			})
			return sanitizeErr(err)
		}
		return sanitizeErr(q.UpsertStoryState(ctx, gen.UpsertStoryStateParams{
			ProjectID: p.ID, StoryID: story.ID, Kind: "config",
			Payload: body, SchemaVersion: 1,
		}))
	})
}

// Version returns the ETag token for the layer: "pgc-<row version>" for an
// existing document, "pgc-0" when it does not exist. Reads do not require write
// rights (membership still applies through memberScope).
func (r *ConfigDocuments) Version(ctx context.Context, actor *Actor, projectSlug, storySlug string) (string, error) {
	_, version, err := r.Get(ctx, actor, projectSlug, storySlug)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pgc-%d", version), nil
}
