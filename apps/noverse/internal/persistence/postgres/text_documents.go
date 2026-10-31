package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
)

// TextDocuments shares config's scoped-document authorization and transaction
// boundary. Project rules use project_documents; story rules and samples use
// the schema's custom_rules and style_sample artifact families.
type TextDocuments struct{ config *ConfigDocuments }
type TextDocument struct {
	Name, Text string
	UpdatedAt  time.Time
	Version    int64
}

func NewTextDocuments(p *Pool) *TextDocuments { return &TextDocuments{config: NewConfigDocuments(p)} }
func (r *LibraryRepository) TextDocuments() *TextDocuments {
	return &TextDocuments{config: &ConfigDocuments{lib: r}}
}
func textKind(kind, story string) (string, error) {
	if kind == "rules" {
		if story != "" {
			return "custom_rules", nil
		}
		return "rules", nil
	}
	if kind == "style_sample" && story != "" {
		return kind, nil
	}
	return "", fmt.Errorf("loại tài liệu không hợp lệ: %w", errs.ErrConfig)
}
func (r *TextDocuments) List(ctx context.Context, actor *Actor, project, story, kind string) ([]TextDocument, error) {
	out := []TextDocument{}
	k, err := textKind(kind, story)
	if err != nil {
		return nil, err
	}
	err = r.config.lib.tx(ctx, func(tx pgx.Tx) error {
		p, s, _, err := r.config.memberScope(ctx, gen.New(tx), actor, project, story)
		if err != nil {
			return err
		}
		var rows pgx.Rows
		if story == "" {
			rows, err = tx.Query(ctx, `SELECT logical_name,text_content,updated_at,version FROM project_documents WHERE project_id=$1 AND kind=$2 ORDER BY logical_name`, p.ID, k)
		} else {
			rows, err = tx.Query(ctx, `SELECT artifact_key,text_content,updated_at,version FROM story_artifacts WHERE project_id=$1 AND story_id=$2 AND kind=$3 ORDER BY artifact_key`, p.ID, s.ID, k)
		}
		if err != nil {
			return sanitizeErr(err)
		}
		defer rows.Close()
		for rows.Next() {
			var d TextDocument
			if err := rows.Scan(&d.Name, &d.Text, &d.UpdatedAt, &d.Version); err != nil {
				return err
			}
			if story == "" {
				d.Name = strings.TrimPrefix(d.Name, ".ainovel/rules/")
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

// Mutate serializes the whole scope on its parent row: CAS, sample capacity,
// unique-name allocation and the write all happen in the same transaction.
// expected is an optional pgd-N token; callers without If-Match overwrite.
func (r *TextDocuments) Mutate(ctx context.Context, actor *Actor, project, story, kind, name, text, expected string, remove bool, limit int) (TextDocument, error) {
	var out TextDocument
	k, err := textKind(kind, story)
	if err != nil {
		return out, err
	}
	err = r.config.lib.tx(ctx, func(tx pgx.Tx) error {
		p, s, role, err := r.config.memberScope(ctx, gen.New(tx), actor, project, story)
		if err != nil {
			return err
		}
		if err := r.config.requireWriteRole(role, story); err != nil {
			return err
		}
		if story == "" {
			_, err = tx.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, p.ID)
		} else {
			_, err = tx.Exec(ctx, `SELECT id FROM stories WHERE project_id=$1 AND id=$2 FOR UPDATE`, p.ID, s.ID)
		}
		if err != nil {
			return err
		}
		key := name
		if story == "" {
			key = ".ainovel/rules/" + name
		}
		var current int64
		if story == "" {
			err = tx.QueryRow(ctx, `SELECT version FROM project_documents WHERE project_id=$1 AND kind=$2 AND logical_name=$3`, p.ID, k, key).Scan(&current)
		} else {
			err = tx.QueryRow(ctx, `SELECT version FROM story_artifacts WHERE project_id=$1 AND story_id=$2 AND kind=$3 AND artifact_key=$4`, p.ID, s.ID, k, key).Scan(&current)
		}
		if err != nil && !noRows(err) {
			return err
		}
		if expected != "" && expected != "*" && expected != fmt.Sprintf("pgd-%d", current) {
			return errs.ErrStorageConflict
		}
		if remove {
			if current == 0 {
				return errs.ErrStorageNotFound
			}
			if story == "" {
				_, err = tx.Exec(ctx, `DELETE FROM project_documents WHERE project_id=$1 AND kind=$2 AND logical_name=$3`, p.ID, k, key)
			} else {
				_, err = tx.Exec(ctx, `DELETE FROM story_artifacts WHERE project_id=$1 AND story_id=$2 AND kind=$3 AND artifact_key=$4`, p.ID, s.ID, k, key)
			}
			return err
		}
		if limit > 0 {
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM story_artifacts WHERE project_id=$1 AND story_id=$2 AND kind=$3`, p.ID, s.ID, k).Scan(&count); err != nil {
				return err
			}
			if count >= limit {
				return errs.ErrStorageConflict
			}
			base := strings.TrimSuffix(name, ".txt")
			for n := 2; current > 0; n++ {
				name = fmt.Sprintf("%s-%d.txt", base, n)
				key = name
				err = tx.QueryRow(ctx, `SELECT version FROM story_artifacts WHERE project_id=$1 AND story_id=$2 AND kind=$3 AND artifact_key=$4`, p.ID, s.ID, k, key).Scan(&current)
				if noRows(err) {
					current = 0
				} else if err != nil {
					return err
				}
			}
		}
		out.Name = name
		out.Text = text
		if story == "" {
			err = tx.QueryRow(ctx, `INSERT INTO project_documents(project_id,id,kind,logical_name,text_content) VALUES($1,$2,$3,$4,$5) ON CONFLICT(project_id,kind,logical_name) DO UPDATE SET text_content=excluded.text_content, updated_at=clock_timestamp(), version=project_documents.version+1 RETURNING updated_at,version`, p.ID, deterministicUUID("rules", UUIDString(p.ID)+"/"+key), k, key, text).Scan(&out.UpdatedAt, &out.Version)
		} else {
			err = tx.QueryRow(ctx, `INSERT INTO story_artifacts(project_id,story_id,kind,artifact_key,text_content) VALUES($1,$2,$3,$4,$5) ON CONFLICT(project_id,story_id,kind,artifact_key) DO UPDATE SET text_content=excluded.text_content, updated_at=clock_timestamp(), version=story_artifacts.version+1 RETURNING updated_at,version`, p.ID, s.ID, k, key, text).Scan(&out.UpdatedAt, &out.Version)
		}
		return sanitizeErr(err)
	})
	return out, err
}
