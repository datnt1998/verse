package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/jackc/pgx/v5"
)

// MigrateGlobalRules copies the server's legacy global rule files into every
// hosted project. Copies, administrative audit, and done marker commit together.
func (r *TextDocuments) MigrateGlobalRules(ctx context.Context, homeRules string) error {
	return r.config.lib.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, rules.GlobalMigrationKey); err != nil {
			return err
		}
		var done bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM noverse_meta WHERE key=$1)`, rules.GlobalMigrationKey).Scan(&done); err != nil {
			return err
		}
		if done {
			return nil
		}
		files, err := rules.LegacyGlobalFiles(homeRules)
		if err != nil {
			return err
		}
		projects, err := gen.New(tx).ListProjects(ctx)
		if err != nil {
			return err
		}
		for _, project := range projects {
			for name, text := range files {
				key := ".ainovel/rules/" + name
				var current string
				err := tx.QueryRow(ctx, `SELECT text_content FROM project_documents WHERE project_id=$1 AND kind='rules' AND logical_name=$2 FOR UPDATE`, project.ID, key).Scan(&current)
				if err != nil && !noRows(err) {
					return err
				}
				merged := rules.MergeLegacyGlobal(name, text, current, err == nil)
				if err == nil && merged == current {
					continue
				}
				if _, err := tx.Exec(ctx, `INSERT INTO project_documents(project_id,id,kind,logical_name,text_content) VALUES($1,$2,'rules',$3,$4) ON CONFLICT(project_id,kind,logical_name) DO UPDATE SET text_content=excluded.text_content, updated_at=clock_timestamp(), version=project_documents.version+1`, project.ID, deterministicUUID("rules", UUIDString(project.ID)+"/"+key), key, merged); err != nil {
					return err
				}
			}
		}
		details, err := json.Marshal(map[string]string{"reason": fmt.Sprintf("Đã chuyển %d tệp quy tắc vào %d dự án; giữ quy tắc riêng của dự án.", len(files), len(projects))})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(id,actor_kind,action,target_kind,target_id,details) VALUES($1,'system',$2,'perm',$3,$4)`, deterministicUUID("migration", rules.GlobalMigrationKey), rules.GlobalMigrationAction, rules.GlobalMigrationKey, details); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO noverse_meta(key,value) VALUES($1,'done')`, rules.GlobalMigrationKey)
		return err
	})
}
