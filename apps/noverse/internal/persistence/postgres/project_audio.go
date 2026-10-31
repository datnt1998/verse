package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
)

// ProjectPronunciations is the hosted counterpart of the project dictionary
// .ainovel/cach-doc.json. It is a project document, not story audio_settings:
// edits must apply to every story sharing this project.
type ProjectPronunciations struct{ lib *LibraryRepository }

func NewProjectPronunciations(p *Pool) *ProjectPronunciations {
	return &ProjectPronunciations{lib: NewLibraryRepository(p)}
}

func (a *Audio) ProjectPronunciations() *ProjectPronunciations {
	return &ProjectPronunciations{lib: &LibraryRepository{pool: a.pool}}
}

const pronunciationDocument = ".ainovel/cach-doc.json"

func pronunciationRow(ctx context.Context, q *gen.Queries, id pgtype.UUID) ([]byte, int64, error) {
	row, err := q.GetProjectDocumentByName(ctx, gen.GetProjectDocumentByNameParams{ProjectID: id, Kind: "config", LogicalName: pronunciationDocument})
	if noRows(err) {
		return []byte("[]"), 0, nil
	}
	if err != nil {
		return nil, 0, sanitizeErr(err)
	}
	return []byte(row.TextContent.String), row.Version, nil
}

func (s *ProjectPronunciations) Get(ctx context.Context, actor *Actor, project string) ([]byte, int64, error) {
	var body []byte
	var version int64
	err := s.lib.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		p, err := s.lib.memberProject(ctx, q, actor, project, false)
		if err != nil {
			return err
		}
		body, version, err = pronunciationRow(ctx, q, p.ID)
		return err
	})
	return body, version, err
}

// Put serializes CAS against the project row, including the first write.
// Collaborators can edit project audio resources, exactly like file mode;
// project configuration's stricter owner-only floor does not apply here.
func (s *ProjectPronunciations) Put(ctx context.Context, actor *Actor, project string, body []byte, expected string) (int64, error) {
	var version int64
	err := s.lib.tx(ctx, func(tx pgx.Tx) error {
		q := gen.New(tx)
		p, role, err := s.lib.memberProjectRole(ctx, q, actor, project)
		if err != nil {
			return err
		}
		if role != "owner" && role != "collab" {
			return errs.ErrStorageForbidden
		}
		if _, err = tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, p.ID); err != nil {
			return sanitizeErr(err)
		}
		_, current, err := pronunciationRow(ctx, q, p.ID)
		if err != nil {
			return err
		}
		if expected != "" && expected != "*" && expected != fmt.Sprintf("pg-pron-%d", current) {
			return errs.ErrStorageConflict
		}
		_, err = q.UpsertProjectDocument(ctx, gen.UpsertProjectDocumentParams{
			ProjectID: p.ID, ID: deterministicUUID("project-audio", UUIDString(p.ID)+"/"+pronunciationDocument), Kind: "config", LogicalName: pronunciationDocument, TextContent: pgtype.Text{String: string(body), Valid: true},
		})
		if err != nil {
			return sanitizeErr(err)
		}
		_, version, err = pronunciationRow(ctx, q, p.ID)
		return err
	})
	return version, err
}
