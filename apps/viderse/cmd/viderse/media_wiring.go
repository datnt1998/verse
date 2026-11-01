package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/storage"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/gofrs/flock"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openMedia and openCLIMedia share the same persisted generation, manifest,
// credentials and database fence. CLI never silently falls back to local.
func openMedia(ctx context.Context, pool *pgxpool.Pool, files *storage.Service) (*serverstorage.Service, error) {
	dir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	// A fresh server has no config directory yet; the lock must not depend on
	// some earlier command having created it.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "server-storage.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("một máy chủ Viderse khác đang dùng kho dữ liệu")
	}
	manager, err := newMediaRuntime(ctx, pool, files.Root, files.Resolve, true, func() { _ = lock.Unlock() }, true)
	if err != nil {
		lock.Unlock()
		return nil, err
	}
	manager.SetHold(files.SetServerStorageOffline)
	remote, err := manager.UsesS3(ctx)
	if err != nil {
		manager.Close(ctx)
		return nil, err
	}
	files.SetRemoteStorage(remote)
	manager.SetStorageWatcher(files.Start, files.SetRemoteStorage)
	return manager, nil
}

// unreferencedSample names at most three paths so the notice stays one line.
func unreferencedSample(paths []string) string {
	const shown = 3
	sample := strings.Join(paths[:min(len(paths), shown)], ", ")
	if len(paths) > shown {
		sample += ", …"
	}
	return sample
}

func openCLIMedia(ctx context.Context, pool *pgxpool.Pool, root func() string) (*serverstorage.Service, error) {
	return newMediaRuntime(ctx, pool, root, nil, false, nil, true)
}
func newMediaRuntime(ctx context.Context, pool *pgxpool.Pool, root func() string, resolve func(string) (string, error), recover bool, release func(), importExisting bool) (*serverstorage.Service, error) {
	dir, err := bootstrap.DefaultConfigDir()
	if err != nil {
		return nil, err
	}
	cfg, err := blobstore.ParseEnv(os.LookupEnv, root())
	if err != nil {
		return nil, err
	}
	if cfg.Delivery == "" {
		cfg.Delivery = "proxy"
	}
	cacheCfg, err := blobstore.ParseCacheEnv(os.LookupEnv, filepath.Join(dir, "cache", "blobstore"))
	if err != nil {
		return nil, err
	}
	cache, err := media.NewCache(cacheCfg.Dir, cacheCfg.MaxBytes)
	if err != nil {
		return nil, err
	}
	repo := postgres.NewBlobRepository(pool)
	box := secretbox.New(dir)
	if _, err := repo.State(ctx); errors.Is(err, pgx.ErrNoRows) && cfg.Driver == "s3" {
		roots, err := repo.Roots(ctx)
		if err != nil {
			return nil, err
		}
		if len(roots) > 0 {
			return nil, fmt.Errorf("dữ liệu cũ chưa chuyển; khởi động bằng kho local rồi chuyển trong Cài đặt")
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	state, err := repo.Initialize(ctx, media.Descriptor{ID: media.NamespaceID(cfg), Config: cfg})
	if err != nil {
		return nil, err
	}
	locked := false
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if blobstore.EnvVarLocks(name) {
			locked = true
		}
	}
	active, err := repo.Descriptor(ctx, state.Active)
	if err != nil {
		return nil, err
	}
	if locked && media.NamespaceID(active.Config) != media.NamespaceID(cfg) {
		return nil, fmt.Errorf("cấu hình môi trường khác kho đang hoạt động; cần chuyển và xác minh dữ liệu trước")
	}
	transport := media.New(repo, cache, func(d media.Descriptor) (blobstore.BlobStore, error) {
		effective := d.Config
		if len(d.Credentials) > 0 {
			plain, err := box.Reveal(d.Credentials)
			if err != nil {
				return nil, err
			}
			access, secret, err := serverstorage.DecodeCredentials(plain)
			if err != nil {
				return nil, err
			}
			effective.S3.AccessKeyID = access
			effective.S3.SecretAccessKey = secret
		} else if media.NamespaceID(effective) == media.NamespaceID(cfg) {
			effective.S3.AccessKeyID = cfg.S3.AccessKeyID
			effective.S3.SecretAccessKey = cfg.S3.SecretAccessKey
		}
		store, err := blobstore.Open(effective)
		if err != nil {
			return nil, err
		}
		return blobstore.NewMetrics(store, nil, time.Second), nil
	})
	sources := map[string]string{}
	for field, names := range map[string][]string{"provider": {"S3_PROVIDER"}, "endpoint": {"S3_ENDPOINT"}, "region": {"S3_REGION"}, "bucket": {"S3_BUCKET"}, "keys": {"S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"}, "delivery": {"BLOB_DELIVERY"}, "presignEndpoint": {"S3_PRESIGN_ENDPOINT"}, "forcePathStyle": {"S3_FORCE_PATH_STYLE"}, "allowInsecure": {"S3_ALLOW_INSECURE_ENDPOINT"}, "cache": {"BLOB_CACHE_DIR", "BLOB_CACHE_MAX_BYTES"}} {
		for _, name := range names {
			if _, ok := os.LookupEnv(name); ok {
				sources[field] = "env"
			}
		}
	}
	localRoots, err := mediaRoots(ctx, pool, root())
	if err != nil {
		return nil, err
	}
	resolve = mediaRootResolver(localRoots)
	manager, err := serverstorage.New(ctx, serverstorage.Options{Repository: repo, Media: transport, Box: box, Effective: cfg, EnvLocked: locked, Sources: sources, Resolve: resolve, LocalRoots: localRoots, Recover: recover, Release: release})
	if err != nil {
		return nil, err
	}
	if importExisting {
		editorRepository := postgres.NewEditorRepository(pool)
		editorWorkspace := &editor.Service{
			Store: editor.Store{Media: transport, Repository: editorRepository},
			Media: transport, Repository: editorRepository,
		}
		if err := editorWorkspace.AdoptBinaryRoots(ctx); err != nil {
			manager.Close(ctx)
			return nil, fmt.Errorf("xác minh tham chiếu binary editor trước khi mở kho: %w", err)
		}
		// Boot never refuses to serve because of an unreferenced file: a crash
		// can interrupt a publication, and the owner must be able to open the
		// app and see the problem. Migration still fails closed (ImportExisting).
		unreferenced, err := manager.BootImport(ctx)
		if err != nil {
			manager.Close(ctx)
			return nil, err
		}
		manager.SetUnreferencedDurablePaths(unreferenced)
	}
	return manager, nil
}
