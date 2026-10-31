package editor_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Real Garage round trip: mutable editor state is an immutable S3 object plus
// a transactionally swapped pointer, never an overwrite of the old object.
func TestWorkspaceS3ColdCacheRevisionAndMedia(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		if os.Getenv("REQUIRE_S3_TEST") != "" {
			t.Fatal("S3_TEST_ENDPOINT is required")
		}
		t.Skip("S3_TEST_ENDPOINT is not configured")
	}
	config, err := blobstore.ParseEnv(func(key string) (string, bool) {
		values := map[string]string{"STORAGE_DRIVER": "s3", "S3_PROVIDER": "garage", "S3_ENDPOINT": endpoint, "S3_REGION": os.Getenv("S3_TEST_REGION"), "S3_BUCKET": os.Getenv("S3_TEST_BUCKET"), "S3_ACCESS_KEY_ID": os.Getenv("S3_TEST_ACCESS_KEY"), "S3_SECRET_ACCESS_KEY": os.Getenv("S3_TEST_SECRET_KEY"), "S3_ALLOW_INSECURE_ENDPOINT": "true", "BLOB_DELIVERY": "proxy"}
		v, ok := values[key]
		return v, ok
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	pool := editorPersistencePool(t)
	blobs := postgres.NewBlobRepository(pool)
	ctx := context.Background()
	if _, err = blobs.Initialize(ctx, media.Descriptor{ID: media.NamespaceID(config), Config: config}); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewEditorRepository(pool)
	makeService := func() *editor.Service {
		cache, e := media.NewCache(t.TempDir(), 64<<20)
		if e != nil {
			t.Fatal(e)
		}
		transport := media.New(blobs, cache, func(d media.Descriptor) (blobstore.BlobStore, error) {
			d.Config.S3.AccessKeyID = config.S3.AccessKeyID
			d.Config.S3.SecretAccessKey = config.S3.SecretAccessKey
			return blobstore.Open(d.Config)
		})
		staging := t.TempDir()
		return &editor.Service{Store: editor.Store{Media: transport, Repository: repo, StagingDir: staging}, Media: transport, Repository: repo, StagingDir: staging}
	}
	service := makeService()
	project, revision, err := service.CreateProject(ctx, editor.CreateProjectInput{Name: "S3"})
	if err != nil {
		t.Fatal(err)
	}
	id := project["id"].(string)
	source, err := os.ReadFile("testdata/__fixtures__/image.png")
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.ImportMedia(ctx, editor.ImportMediaOptions{ProjectID: id, FileName: "image.png", DeclaredMIME: "image/png", Body: bytes.NewReader(source)})
	if err != nil {
		t.Fatal(err)
	}
	project["description"] = "Saved to new immutable object"
	next, err := service.SaveProject(ctx, id, project, revision)
	if err != nil {
		t.Fatal(err)
	}
	cold := makeService()
	loaded, got, err := cold.LoadProject(ctx, id)
	if err != nil || got != next || loaded["description"] != project["description"] {
		t.Fatalf("cold project=%v rev=%s err=%v", loaded, got, err)
	}
	records, err := cold.ListMedia(ctx, id)
	if err != nil || len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("cold media=%v %v", records, err)
	}
	handle, err := cold.AcquireMedia(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil || !bytes.Equal(actual, source) {
		t.Fatalf("cold source mismatch: %v", err)
	}
}
