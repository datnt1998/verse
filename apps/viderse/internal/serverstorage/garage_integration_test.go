package serverstorage_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/blobstore/s3test"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

type fixture struct {
	manager *serverstorage.Service
	repo    *postgres.BlobRepository
	media   *media.Service
	library *postgres.LibraryStore
	root    string
	changed chan struct{}
	box     *secretbox.Box
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pgmigrate.Up(context.Background(), db, migrations.FS); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	pool, err := postgres.OpenPool(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	root := t.TempDir()
	cfg := blobstore.Config{Driver: "local", LocalRoot: root, Delivery: "proxy"}
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(context.Background(), media.Descriptor{ID: "local", Config: cfg}); err != nil {
		t.Fatal(err)
	}
	cache, err := media.NewCache(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	box := secretbox.New(t.TempDir())
	transport := media.New(repo, cache, func(d media.Descriptor) (blobstore.BlobStore, error) {
		if len(d.Credentials) > 0 {
			plain, err := box.Reveal(d.Credentials)
			if err != nil {
				return nil, err
			}
			access, secret, err := serverstorage.DecodeCredentials(plain)
			if err != nil {
				return nil, err
			}
			d.Config.S3.AccessKeyID = access
			d.Config.S3.SecretAccessKey = secret
		}
		store, err := blobstore.Open(d.Config)
		if err != nil {
			return nil, err
		}
		return blobstore.NewMetrics(store, nil, time.Second), nil
	})
	changed := make(chan struct{}, 64)
	manager, err := serverstorage.New(context.Background(), serverstorage.Options{Repository: repo, Media: transport, Box: box, Effective: cfg, Changed: func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return fixture{manager, repo, transport, postgres.NewLibraryStore(pool), root, changed, box}
}
func garageConfig(t *testing.T) serverstorage.S3Config {
	t.Helper()
	if os.Getenv("S3_TEST_ENDPOINT") == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("S3_TEST_ENDPOINT required")
		}
		t.Skip("S3_TEST_ENDPOINT not set")
	}
	cfg := serverstorage.S3Config{Provider: "garage", Endpoint: os.Getenv("S3_TEST_ENDPOINT"), Region: os.Getenv("S3_TEST_REGION"), Bucket: os.Getenv("S3_TEST_BUCKET"), AccessKeyID: os.Getenv("S3_TEST_ACCESS_KEY"), SecretAccessKey: os.Getenv("S3_TEST_SECRET_KEY"), AllowInsecure: true, ForcePathStyle: true, Delivery: "proxy"}
	seed, err := blobstore.NewS3(blobstore.S3Config{Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: cfg.Bucket, AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, AllowPlainHTTP: true, ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	// Every repository has its own roots and read leases. Sharing a bucket
	// would let its GC mistake another fixture's aged live objects for orphans.
	cfg.Bucket = s3test.Bucket(t, seed.Client(), cfg.AccessKeyID, "viderse-storage")
	store, err := blobstore.NewS3(blobstore.S3Config{Endpoint: cfg.Endpoint, Region: cfg.Region, Bucket: cfg.Bucket, AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, AllowPlainHTTP: true, ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// This store is bound only to the bucket created above, never the
		// environment's shared bucket or a long-running development fixture.
		for {
			page, err := store.List(ctx, "", "", 1000)
			if err != nil {
				t.Errorf("list isolated fixture objects: %v", err)
				return
			}
			if len(page.Infos) == 0 {
				return
			}
			for _, info := range page.Infos {
				if err := store.Delete(ctx, info.Key); err != nil {
					t.Errorf("delete isolated fixture object: %v", err)
					return
				}
			}
		}
	})
	return cfg
}
func (f fixture) publish(t *testing.T, label string) media.Ref {
	t.Helper()
	ctx := context.Background()
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(token[:])
	uuid := id[:8] + "-" + id[8:12] + "-4" + id[13:16] + "-a" + id[17:20] + "-" + id[20:]
	path := "library/overlays/" + uuid + ".mp4"
	source := filepath.Join(t.TempDir(), "input.mp4")
	payload := []byte(label + id)
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	err := f.media.Publish(ctx, []media.Artifact{{Path: path, File: source, ContentType: "video/mp4", Filename: "input.mp4"}}, func(txctx context.Context) error {
		state, err := f.repo.State(txctx)
		if err != nil {
			return err
		}
		ref, err := f.repo.Resolve(txctx, state.Active, path)
		if err != nil {
			return err
		}
		return f.library.InsertOverlay(txctx, domain.Overlay{ID: uuid, SHA256: ref.SHA256, Name: label, FilePath: path, Mime: "video/mp4", Bytes: ref.Bytes, Background: domain.BgBlack, CreatedAt: time.Now()})
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := f.repo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.repo.Resolve(ctx, state.Active, path)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
func waitMigration(t *testing.T, f fixture, want string) {
	t.Helper()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		record, err := f.repo.LatestMigration(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if record != nil && record.Migration.State == want {
			return
		}
		if record != nil && record.Migration.State == "error" {
			t.Fatalf("migration failed: %+v", record.Migration)
		}
		select {
		case <-f.changed:
		case <-deadline.C:
			t.Fatalf("migration never reached %s", want)
		}
	}
}
func TestGarageTestSessionBindingOnlineCutoverAndPartialUsage(t *testing.T) {
	config := garageConfig(t)
	f := newFixture(t)
	ctx := context.Background()
	ref := f.publish(t, "before migration")
	tested, err := f.manager.Test(ctx, "admin", config)
	if err != nil || tested.Error != nil || len(tested.Steps) != 6 {
		t.Fatalf("test: %+v %v", tested, err)
	}
	if tested.Steps[3].Status != "na" || tested.Steps[4].ID != "app_range" || tested.Steps[4].Status != "ok" {
		t.Fatalf("proxy test did not exercise app range: %+v", tested.Steps)
	}
	if _, err := f.manager.Save(ctx, "other-admin", config, tested.TestID); err == nil {
		t.Fatal("test transferred to another actor")
	}
	changed := config
	changed.Delivery = "redirect"
	if _, err := f.manager.Save(ctx, "admin", changed, tested.TestID); err == nil {
		t.Fatal("test transferred to changed configuration")
	}
	saved, err := f.manager.Save(ctx, "admin", config, tested.TestID)
	if err != nil || saved.NeedsMigration == nil || saved.NeedsMigration.Files != 1 {
		t.Fatalf("save: %+v %v", saved, err)
	}
	state, err := f.repo.State(ctx)
	if err != nil || state.Active != "local" || state.Pending == "" {
		t.Fatalf("save changed active prematurely: %+v %v", state, err)
	}
	if _, err := f.manager.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	waitMigration(t, f, "done")
	state, err = f.repo.State(ctx)
	if err != nil || state.Active == "local" || state.Previous != "local" {
		t.Fatalf("cutover: %+v %v", state, err)
	}
	handle, err := f.media.Acquire(ctx, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(f.root, ref.Path))
	if err != nil || string(original) != string(bytes) {
		t.Fatal("migration changed source or lost target")
	}
	snap, err := f.manager.Snapshot(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(snap)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"storage", "workCache", "ops", "migration"} {
		if fields[hidden] != nil {
			t.Fatalf("member leaked %s", hidden)
		}
	}
	for _, group := range snap.Usage {
		if group.Group == "library" {
			if group.Files == nil || *group.Files != 1 {
				t.Fatalf("library usage=%+v", group)
			}
		} else if group.Files != nil || group.Bytes != nil {
			t.Fatalf("unimplemented producer presented as counted: %+v", group)
		}
	}
}
func TestGarageRedirectRequiresBrowserProbeAndSuccessfulDelete(t *testing.T) {
	config := garageConfig(t)
	config.Delivery = "redirect"
	config.PresignEndpoint = config.Endpoint
	f := newFixture(t)
	ctx := context.Background()
	tested, err := f.manager.Test(ctx, "admin", config)
	if err != nil || tested.Error != nil || tested.ProbeURL == "" || tested.Steps[4].Status != "pending" {
		t.Fatalf("redirect test: %+v %v", tested, err)
	}
	if _, err := f.manager.Save(ctx, "admin", config, tested.TestID); err == nil {
		t.Fatal("server-only CORS test accepted")
	}
	if _, err := f.manager.Probe(ctx, "other", tested.TestID, true, ""); err == nil {
		t.Fatal("probe not actor-bound")
	}
	completed, err := f.manager.Probe(ctx, "admin", tested.TestID, false, "cors")
	if err != nil || completed.Error == nil || completed.Error.Code != "cors_blocked" || completed.Steps[5].Status != "ok" {
		t.Fatalf("probe: %+v %v", completed, err)
	}
	if _, err := f.manager.Save(ctx, "admin", config, tested.TestID); err == nil {
		t.Fatal("CORS failure saved")
	}
	tested, err = f.manager.Test(ctx, "admin", config)
	if err != nil {
		t.Fatal(err)
	}
	completed, err = f.manager.Probe(ctx, "admin", tested.TestID, true, "")
	if err != nil || completed.Error != nil || completed.Steps[5].Status != "ok" {
		t.Fatalf("browser success: %+v %v", completed, err)
	}
	if _, err = f.manager.Save(ctx, "admin", config, tested.TestID); err != nil {
		t.Fatal(err)
	}
}
func TestConfigCorruptKeyStillTestsButCannotSave(t *testing.T) {
	config := garageConfig(t)
	f := newFixture(t)
	if err := os.WriteFile(f.box.Path(), make([]byte, 31), 0600); err != nil {
		t.Fatal(err)
	}
	tested, err := f.manager.Test(context.Background(), "admin", config)
	if err != nil || tested.Error != nil {
		t.Fatalf("test should not need machine key: %+v %v", tested, err)
	}
	_, err = f.manager.Save(context.Background(), "admin", config, tested.TestID)
	var typed *serverstorage.Error
	if !errors.As(err, &typed) || typed.Status != 423 {
		t.Fatalf("save without key: %v", err)
	}
}
