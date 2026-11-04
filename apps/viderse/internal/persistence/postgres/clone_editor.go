package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
)

// jobEditorHeadsSQL liệt kê head editor thuộc đúng project của một job: head dự
// án, links/sidecars của nó và các export gắn với nó. $1 = id project, $2 =
// "projects/<id>", $3 = "exports/<id>". Thứ tự key ổn định để mọi tx khoá cùng
// thứ tự.
const jobEditorHeadsSQL = `
SELECT 'projects/' || p.project_id, p.content_revision FROM editor_projects p
WHERE p.project_id = $1
UNION ALL
SELECT r.resource_key, r.content_revision FROM editor_resources r
WHERE r.resource_key = $2 OR left(r.resource_key, length($2)+1) = $2 || '/'
   OR r.resource_key = $3 OR left(r.resource_key, length($3)+1) = $3 || '/'
ORDER BY 1`

// retireJobEditorProject xoá head editor của chính job trước khi binding của nó
// được nhả: head dự án, links/sidecars và export đi qua đúng đường canonical
// (khoá theo key, kiểm revision, ghi editor_deleted_resources, nhả binding khi
// hết chủ) nên video đã xoá không còn hiện lại như project sống.
//
// Head media toàn cục (`media/<id>`, `media/<id>/source`) KHÔNG bị xoá ở đây:
// cùng một media id có thể đang được project khác tham chiếu, và chỉ heldMedia
// của editor (đọc JSON project + snapshot còn giữ) mới xác định được chủ cuối.
// Đường canonical đó là ReclaimMediaOf/ReclaimUnreferencedMedia → removeResource;
// nó tombstone head rồi mới nhả binding. Vì thế BlobRepository.Unbind cũng
// không ép nhả path còn live ref — binding của head dùng chung ở lại và bytes
// vẫn đọc được.
func (s *CloneStore) retireJobEditorProject(ctx context.Context, job pgtype.UUID) error {
	db := mediaDB(ctx, s.pool)
	var jobID string
	var generation int
	err := db.QueryRow(ctx, `SELECT id::text, generation FROM clone_jobs WHERE id=$1`, job).Scan(&jobID, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	project := fmt.Sprintf("clone-%s-g%d", jobID, generation)
	heads, err := jobEditorHeads(ctx, db, project)
	if err != nil {
		return err
	}
	repo := &EditorRepository{pool: s.pool}
	// Khoá mọi head theo thứ tự key trước khi đọc revision và xoá: head chỉ đổi
	// trong tx giữ khoá của chính nó nên revision đọc sau khoá không thể cũ, và
	// hai lần xoá chồng nhau không tạo vòng khoá.
	for _, head := range heads {
		if err = repo.lock(ctx, head.key); err != nil {
			return err
		}
	}
	heads, err = jobEditorHeads(ctx, db, project)
	if err != nil {
		return err
	}
	for _, head := range heads {
		if err = repo.Delete(ctx, head.key, head.revision); err != nil {
			return err
		}
	}
	return nil
}

type editorOwner struct{ key, revision string }

func jobEditorHeads(ctx context.Context, db gen.DBTX, project string) ([]editorOwner, error) {
	rows, err := db.Query(ctx, jobEditorHeadsSQL, project, "projects/"+project, "exports/"+project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	heads := []editorOwner{}
	for rows.Next() {
		var head editorOwner
		if err = rows.Scan(&head.key, &head.revision); err != nil {
			return nil, err
		}
		heads = append(heads, head)
	}
	return heads, rows.Err()
}
