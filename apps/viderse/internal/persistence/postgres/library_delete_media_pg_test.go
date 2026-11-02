// Ca test Postgres: xoá template/prompt trong transaction media là nguyên tử —
// hàng cha, cascade cover_trials / hàng ảnh mẫu và blob_refs cùng commit hoặc
// cùng rollback. Lỗi giữa đường để lại đủ hàng + tham chiếu để xoá lại.
// Chạy trên Postgres thật; bỏ qua khi thiếu TEST_DATABASE_URL (cùng provision
// với settings_pg_test).
package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

type mediaDeleteFixture struct {
	pool   *pgxpool.Pool
	svc    *library.Service
	store  *postgres.LibraryStore
	trials *postgres.CoverTrialStore
	files  *media.Service
}

func newMediaDeleteFixture(t *testing.T) mediaDeleteFixture {
	t.Helper()
	pool := provision(t)
	ctx := context.Background()
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{
		ID:     "library-delete-pg",
		Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()},
	}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) {
		return blobstore.NewLocal(d.Config.LocalRoot)
	})
	store := postgres.NewLibraryStore(pool)
	return mediaDeleteFixture{
		pool:   pool,
		svc:    library.New(library.Deps{Store: store}),
		store:  store,
		trials: postgres.NewCoverTrialStore(pool),
		files:  files,
	}
}

// bindRef gắn một tham chiếu media đã bind cho path (blob + blob_refs).
func (f mediaDeleteFixture) bindRef(t *testing.T, path string) {
	t.Helper()
	sha := strings.Repeat("a", 64)
	key := "v1/sha256/aa/" + sha
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `INSERT INTO blobs(object_key,sha256,bytes) VALUES($1,$2,4)`, key, sha); err != nil {
		t.Fatalf("blob: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO blob_refs(logical_path,object_key,content_type,filename)
		VALUES($1,$2,'image/png','trial.png')`, path, key); err != nil {
		t.Fatalf("blob_ref: %v", err)
	}
}

func (f mediaDeleteFixture) seedCoverTrial(t *testing.T, templateID, path string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO cover_trials
		(id, template_id, job_id, prompt_id, composition, state, media_path)
		VALUES (gen_random_uuid(), $1, gen_random_uuid(), 'builtin:cover', 'clean', 'ready', $2)`,
		templateID, path); err != nil {
		t.Fatalf("cover_trials: %v", err)
	}
}

func (f mediaDeleteFixture) seedPromptSample(t *testing.T, promptID, path string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO image_prompt_samples
		(id, prompt_id, prompt_kind, video_id, state, media_path)
		VALUES (gen_random_uuid(), $1, 'scene', gen_random_uuid(), 'ready', $2)`,
		promptID, path); err != nil {
		t.Fatalf("image_prompt_samples: %v", err)
	}
}

func TestDeleteTemplateCommitsTrialRowsAndMediaRefsTogether(t *testing.T) {
	f := newMediaDeleteFixture(t)
	ctx := context.Background()
	tpl, err := f.svc.Create(ctx, "Xoá kèm media")
	if err != nil {
		t.Fatal(err)
	}
	const path = "library/cover-trials/atomic.png"
	f.bindRef(t, path)
	f.seedCoverTrial(t, tpl.ID, path)

	if err := f.files.Unbind(ctx, []string{path}, func(txctx context.Context) error {
		return f.store.DeleteTemplate(txctx, tpl.ID)
	}); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM templates WHERE id=$1`, tpl.ID); n != 0 {
		t.Fatalf("template còn %d hàng", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM cover_trials WHERE template_id=$1`, tpl.ID); n != 0 {
		t.Fatalf("lịch sử vẽ thử còn %d hàng", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, path); n != 0 {
		t.Fatalf("tham chiếu media còn %d hàng", n)
	}
}

func TestDeleteTemplateRollsBackTrialMediaWhenTransactionFails(t *testing.T) {
	f := newMediaDeleteFixture(t)
	ctx := context.Background()
	tpl, err := f.svc.Create(ctx, "Xoá lỗi")
	if err != nil {
		t.Fatal(err)
	}
	const path = "library/cover-trials/rollback.png"
	f.bindRef(t, path)
	f.seedCoverTrial(t, tpl.ID, path)

	stop := errors.New("dừng sau khi xoá hàng")
	err = f.files.Unbind(ctx, []string{path}, func(txctx context.Context) error {
		if err := f.store.DeleteTemplate(txctx, tpl.ID); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("lỗi = %v, muốn %v", err, stop)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM templates WHERE id=$1`, tpl.ID); n != 1 {
		t.Fatalf("template còn %d hàng, muốn 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM cover_trials WHERE template_id=$1`, tpl.ID); n != 1 {
		t.Fatalf("lịch sử còn %d hàng, muốn 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, path); n != 1 {
		t.Fatalf("tham chiếu media còn %d hàng, muốn 1", n)
	}
}

func TestDeletePromptCommitsSamplesAndMediaRefsTogether(t *testing.T) {
	f := newMediaDeleteFixture(t)
	ctx := context.Background()
	prompt, err := f.svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Xoá kèm mẫu", Kind: domain.PromptScene, Prompt: new("nội dung"),
	})
	if err != nil {
		t.Fatal(err)
	}
	const path = "library/prompt-samples/atomic.png"
	f.bindRef(t, path)
	f.seedPromptSample(t, prompt.ID, path)

	if err := f.files.Unbind(ctx, []string{path}, func(txctx context.Context) error {
		if err := f.trials.DeletePromptSamplesByPrompt(txctx, prompt.ID); err != nil {
			return err
		}
		return f.store.DeletePrompt(txctx, prompt.ID)
	}); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM image_prompts WHERE id=$1`, prompt.ID); n != 0 {
		t.Fatalf("prompt còn %d hàng", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM image_prompt_samples WHERE prompt_id=$1`, prompt.ID); n != 0 {
		t.Fatalf("ảnh mẫu còn %d hàng", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, path); n != 0 {
		t.Fatalf("tham chiếu media còn %d hàng", n)
	}
}

func TestDeletePromptRollsBackSamplesWhenTransactionFails(t *testing.T) {
	f := newMediaDeleteFixture(t)
	ctx := context.Background()
	prompt, err := f.svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Xoá lỗi", Kind: domain.PromptScene, Prompt: new("nội dung"),
	})
	if err != nil {
		t.Fatal(err)
	}
	const path = "library/prompt-samples/rollback.png"
	f.bindRef(t, path)
	f.seedPromptSample(t, prompt.ID, path)

	stop := errors.New("dừng sau khi xoá hàng")
	err = f.files.Unbind(ctx, []string{path}, func(txctx context.Context) error {
		if err := f.trials.DeletePromptSamplesByPrompt(txctx, prompt.ID); err != nil {
			return err
		}
		if err := f.store.DeletePrompt(txctx, prompt.ID); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("lỗi = %v, muốn %v", err, stop)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM image_prompts WHERE id=$1`, prompt.ID); n != 1 {
		t.Fatalf("prompt còn %d hàng, muốn 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM image_prompt_samples WHERE prompt_id=$1`, prompt.ID); n != 1 {
		t.Fatalf("ảnh mẫu còn %d hàng, muốn 1", n)
	}
	if n := countRows(t, f.pool, `SELECT count(*) FROM blob_refs WHERE logical_path=$1`, path); n != 1 {
		t.Fatalf("tham chiếu media còn %d hàng, muốn 1", n)
	}
}
