package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/platform/blobstore"
)

type cloneDeleteFixture struct {
	pool   *pgxpool.Pool
	store  *postgres.CloneStore
	files  *media.Service
	editor *postgres.EditorRepository
}

func newCloneDeleteFixture(t *testing.T) *cloneDeleteFixture {
	t.Helper()
	ctx := context.Background()
	pool := provision(t)
	seedCloneChannel(t, pool)
	root := t.TempDir()
	blobs := postgres.NewBlobRepository(pool)
	if _, err := blobs.Initialize(ctx, media.Descriptor{ID: "clone-delete", Config: blobstore.Config{Driver: "local", LocalRoot: root}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(blobs, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	return &cloneDeleteFixture{pool: pool, store: postgres.NewCloneStore(pool), files: files, editor: postgres.NewEditorRepository(pool)}
}

// adopt publish một file thành artifact của job rồi ghi head editor trỏ vào
// đúng binding đó — đúng dữ liệu thật khi Bàn dựng nhận artifact của build làm
// head (editor head luôn trỏ vào clone/<job>/…).
func (f *cloneDeleteFixture) adopt(t *testing.T, jobID, key, rel, body string) string {
	t.Helper()
	ctx := context.Background()
	revision := editor.Revision([]byte(body))
	path := "clone/" + jobID + "/" + strings.TrimPrefix(revision, "sha256:") + "/" + rel
	file := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.files.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: "application/json"}}, func(tx context.Context) error {
		return f.editor.CAS(tx, key, "", editor.Resource{Key: key, Path: path, Revision: revision})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO clone_artifacts(id,job_id,step,path,sha256,bytes,content_type)
		VALUES($1,$2,'build',$3,$4,$5,'application/json')`,
		"artifact-"+key, jobID, path, strings.TrimPrefix(revision, "sha256:"), len(body)); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f *cloneDeleteFixture) readBytes(t *testing.T, path string) string {
	t.Helper()
	handle, err := f.files.Acquire(context.Background(), path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	defer handle.Release()
	data, err := os.ReadFile(handle.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// editorDeleteFixture: hai video cùng dùng một media id toàn cục (media ID của
// fixture QA dùng chung giữa nhiều project), mỗi video có project clone riêng.
type editorDeleteFixture struct {
	*cloneDeleteFixture
	job, other                         *domain.CloneJob
	projectA, projectB                 string
	sharedBody, sharedPath             string
	projectPath, linksPath, exportPath string
	otherLinksBody, otherLinksPath     string
}

func seedEditorDelete(t *testing.T) *editorDeleteFixture {
	t.Helper()
	f := newCloneDeleteFixture(t)
	job := cloneTestJob(t, f.store, "editor-delete")
	other := cloneTestJob(t, f.store, "editor-delete-other")
	projectA := "clone-" + job.ID + "-g1"
	projectB := "clone-" + other.ID + "-g1"
	// Media toàn cục: một head, artifact nằm trong namespace của job sắp xoá.
	sharedBody := `{"name":"scene-warm"}`
	sharedPath := f.adopt(t, job.ID, "media/media-shared", "editor/media/media-shared/metadata.json", sharedBody)
	// Project của job: head dự án, links và một export.
	projectPath := f.adopt(t, job.ID, "projects/"+projectA, "editor/projects/"+projectA+"/project.json", `{"id":"`+projectA+`"}`)
	linksPath := f.adopt(t, job.ID, "projects/"+projectA+"/links", "editor/projects/"+projectA+"/links.json", `{"mediaIds":[{"id":"media-shared"}]}`)
	exportPath := f.adopt(t, job.ID, "exports/"+projectA+"/exp1", "editor/projects/"+projectA+"/exports/exp1/meta.json", `{"exportId":"exp1"}`)
	// Project khác vẫn dùng cùng media id.
	otherLinksBody := `{"mediaIds":[{"id":"media-shared"}]}`
	otherLinksPath := f.adopt(t, other.ID, "projects/"+projectB+"/links", "editor/projects/"+projectB+"/links.json", otherLinksBody)
	return &editorDeleteFixture{
		cloneDeleteFixture: f,
		job:                job,
		other:              other,
		projectA:           projectA,
		projectB:           projectB,
		sharedBody:         sharedBody,
		sharedPath:         sharedPath,
		projectPath:        projectPath,
		linksPath:          linksPath,
		exportPath:         exportPath,
		otherLinksBody:     otherLinksBody,
		otherLinksPath:     otherLinksPath,
	}
}

// Xoá một video phải gỡ head editor của CHÍNH project đó (head dự án, links,
// export) rồi mới nhả binding của chúng. Head media toàn cục dùng chung với
// project khác phải sống: bản sửa đầu tiên xoá mọi head trỏ vào artifact của
// job, làm project còn lại mất media ('Không tìm thấy dữ liệu editor').
func TestDeleteJobRetiresOwnProjectAndKeepsSharedEditorMedia(t *testing.T) {
	f := seedEditorDelete(t)
	ctx := context.Background()
	service := pipeline.Service{Store: f.store, Media: f.files}
	if err := f.store.SetJobStatus(ctx, f.job.ID, domain.CloneCancelled); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, f.job.ID); err != nil {
		t.Fatalf("xoá video: %v", err)
	}
	for _, tc := range []struct {
		name  string
		query string
		arg   string
		want  int
	}{
		{"job", `SELECT count(*) FROM clone_jobs WHERE id=$1`, f.job.ID, 0},
		{"project head", `SELECT count(*) FROM editor_projects WHERE project_id=$1`, f.projectA, 0},
		{"project tombstone", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "projects/" + f.projectA, 1},
		{"links head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "projects/" + f.projectA + "/links", 0},
		{"links tombstone", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "projects/" + f.projectA + "/links", 1},
		{"export head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "exports/" + f.projectA + "/exp1", 0},
		{"export tombstone", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "exports/" + f.projectA + "/exp1", 1},
		{"project binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.projectPath, 0},
		{"links binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.linksPath, 0},
		{"export binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.exportPath, 0},
		{"shared head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "media/media-shared", 1},
		{"shared not tombstoned", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "media/media-shared", 0},
		{"shared binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.sharedPath, 1},
		{"other job", `SELECT count(*) FROM clone_jobs WHERE id=$1`, f.other.ID, 1},
		{"other links head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "projects/" + f.projectB + "/links", 1},
		{"other links binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.otherLinksPath, 1},
	} {
		if n := countRows(t, f.pool, tc.query, tc.arg); n != tc.want {
			t.Fatalf("%s = %d, muốn %d", tc.name, n, tc.want)
		}
	}
	// Media dùng chung vẫn đọc được qua đúng đường editor: head → binding → bytes.
	head, err := f.editor.Get(ctx, "media/media-shared")
	if err != nil {
		t.Fatalf("head media dùng chung: %v", err)
	}
	if head.Path != f.sharedPath || head.Revision != editor.Revision([]byte(f.sharedBody)) {
		t.Fatalf("head media dùng chung: %+v", head)
	}
	if body := f.readBytes(t, head.Path); body != f.sharedBody {
		t.Fatalf("bytes media dùng chung = %q, muốn %q", body, f.sharedBody)
	}
	other := f.readBytes(t, f.otherLinksPath)
	if other != f.otherLinksBody {
		t.Fatalf("links project còn lại = %q, muốn %q", other, f.otherLinksBody)
	}
}

// Xoá job là một transaction: lỗi sau khi đã retire head (và đã nhả binding
// trong tx) phải trả lại head, tombstone, binding và job như trước.
func TestDeleteJobRollsBackRetiredProjectHeads(t *testing.T) {
	f := seedEditorDelete(t)
	ctx := context.Background()
	abort := errors.New("dừng sau khi xoá job")
	err := f.files.Publish(ctx, nil, func(tx context.Context) error {
		// Bỏ artifact của project trước để retire nhả binding ngay trong tx.
		if err := f.store.RemoveArtifactsByPath(tx, f.job.ID, []string{f.projectPath, f.linksPath, f.exportPath}); err != nil {
			return err
		}
		if err := f.store.DeleteJob(tx, f.job.ID); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("lỗi = %v, muốn %v", err, abort)
	}
	for _, tc := range []struct {
		name  string
		query string
		arg   string
		want  int
	}{
		{"job", `SELECT count(*) FROM clone_jobs WHERE id=$1`, f.job.ID, 1},
		{"project head", `SELECT count(*) FROM editor_projects WHERE project_id=$1`, f.projectA, 1},
		{"links head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "projects/" + f.projectA + "/links", 1},
		{"export head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "exports/" + f.projectA + "/exp1", 1},
		{"shared head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "media/media-shared", 1},
		{"no tombstone", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "projects/" + f.projectA, 0},
		{"project binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.projectPath, 1},
		{"shared binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.sharedPath, 1},
	} {
		if n := countRows(t, f.pool, tc.query, tc.arg); n != tc.want {
			t.Fatalf("%s = %d, muốn %d", tc.name, n, tc.want)
		}
	}
}

// Khi không còn project nào dùng media, chủ cuối xoá head qua đường canonical
// (ReclaimMediaOf/removeResource của editor) — head + tombstone + binding đi
// cùng transaction, đúng thứ tự retireJobEditorProject dựa vào.
func TestCanonicalMediaDeleteRetiresLastOwnerHeadAndBinding(t *testing.T) {
	f := seedEditorDelete(t)
	ctx := context.Background()
	service := pipeline.Service{Store: f.store, Media: f.files}
	if err := f.store.SetJobStatus(ctx, f.job.ID, domain.CloneCancelled); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, f.job.ID); err != nil {
		t.Fatalf("xoá video: %v", err)
	}
	head, err := f.editor.Get(ctx, "media/media-shared")
	if err != nil {
		t.Fatalf("head media dùng chung: %v", err)
	}
	if err = f.files.Publish(ctx, nil, func(tx context.Context) error {
		return f.editor.Delete(tx, "media/media-shared", head.Revision)
	}); err != nil {
		t.Fatalf("xoá head media cuối: %v", err)
	}
	for _, tc := range []struct {
		name  string
		query string
		arg   string
		want  int
	}{
		{"head", `SELECT count(*) FROM editor_resources WHERE resource_key=$1`, "media/media-shared", 0},
		{"tombstone", `SELECT count(*) FROM editor_deleted_resources WHERE resource_key=$1`, "media/media-shared", 1},
		{"binding", `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, f.sharedPath, 0},
	} {
		if n := countRows(t, f.pool, tc.query, tc.arg); n != tc.want {
			t.Fatalf("%s = %d, muốn %d", tc.name, n, tc.want)
		}
	}
}

// Xoá video đang tạm dừng là xung đột: UI khoá nút và yêu cầu dừng hẳn trước,
// nên service phải trả ErrConflict (web map thành 409) chứ không xoá nửa vời.
func TestDeleteJobRejectsPausedVideo(t *testing.T) {
	f := newCloneDeleteFixture(t)
	ctx := context.Background()
	job := cloneTestJob(t, f.store, "delete-paused")
	if err := f.store.SetJobStatus(ctx, job.ID, domain.CloneCancelled); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetPaused(ctx, job.ID, true); err != nil {
		t.Fatal(err)
	}
	service := pipeline.Service{Store: f.store, Media: f.files}
	if err := service.Delete(ctx, job.ID); err != pipeline.ErrConflict {
		t.Fatalf("xoá video tạm dừng: %v, muốn ErrConflict", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM clone_jobs WHERE id=$1`, job.ID); n != 1 {
		t.Fatalf("job còn %d hàng, muốn 1", n)
	}
}

// "Dừng hẳn" = POST cancel, rồi mới xoá được. Huỷ phải ghi request + đóng step
// + bỏ tạm dừng + status cancelled trong cùng transaction, nếu không job vẫn
// mang paused_at và Delete trả 409 vĩnh viễn. Test đọc lại state từ DB, không
// tin object trả về.
func TestCancelPausedVideoClearsPauseSoDeleteSucceeds(t *testing.T) {
	f := newCloneDeleteFixture(t)
	ctx := context.Background()
	job := cloneTestJob(t, f.store, "cancel-paused")
	if err := f.store.SetPaused(ctx, job.ID, true); err != nil {
		t.Fatal(err)
	}
	service := pipeline.Service{Store: f.store, Media: f.files}
	if err := service.Delete(ctx, job.ID); err != pipeline.ErrConflict {
		t.Fatalf("xoá trước khi huỷ: %v, muốn ErrConflict", err)
	}
	cancelled, err := service.Cancel(ctx, job.ID)
	if err != nil {
		t.Fatalf("huỷ video tạm dừng: %v", err)
	}
	stored, err := f.store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.CloneCancelled || stored.PausedAt != nil || stored.StageGroup == domain.StagePaused {
		t.Fatalf("state sau huỷ: status=%s paused=%v stage=%s", stored.Status, stored.PausedAt, stored.StageGroup)
	}
	if stored.CancelRequestedAt == nil || cancelled.CancelRequestedAt == nil {
		t.Fatal("huỷ phải giữ cancel_requested_at cho worker/fence")
	}
	if err := service.Delete(ctx, job.ID); err != nil {
		t.Fatalf("xoá sau huỷ: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM clone_jobs WHERE id=$1`, job.ID); n != 0 {
		t.Fatalf("job còn %d hàng, muốn 0", n)
	}
}

func TestRestartPausedVideoRetiresPauseSoOldGenerationCanBeDeleted(t *testing.T) {
	f := newCloneDeleteFixture(t)
	ctx := context.Background()
	old := cloneTestJob(t, f.store, "restart-paused")
	if err := f.store.SetJobStatus(ctx, old.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetPaused(ctx, old.ID, true); err != nil {
		t.Fatal(err)
	}
	next, err := f.store.StartJob(ctx, postgres.StartJobInput{
		ChannelID: old.ChannelID, SourceKey: old.SourceKey, Source: old.Source,
		TemplateSnapshot: old.TemplateSnapshot, ModelRefs: old.ModelRefs, Replaces: old.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	retired, err := f.store.GetJob(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retired.Status != domain.CloneCancelled || retired.SupersededAt == nil || retired.PausedAt != nil {
		t.Fatalf("restarted generation still holds a pause: status=%s superseded=%v paused=%v", retired.Status, retired.SupersededAt, retired.PausedAt)
	}
	service := pipeline.Service{Store: f.store, Media: f.files}
	if err := service.Delete(ctx, old.ID); err != nil {
		t.Fatalf("delete retired generation: %v", err)
	}
	if _, err := f.store.GetJob(ctx, next.Job.ID); err != nil {
		t.Fatalf("deleting retired generation removed its replacement: %v", err)
	}
}
