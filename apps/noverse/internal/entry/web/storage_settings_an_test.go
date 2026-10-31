package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func storageCandidateEnv(t *testing.T, c bootstrap.StorageCandidate) {
	t.Helper()
	for key, value := range map[string]string{"STORAGE_DRIVER": "s3", "S3_PROVIDER": c.Provider, "S3_ENDPOINT": c.Endpoint, "S3_PRESIGN_ENDPOINT": c.PresignEndpoint, "S3_REGION": c.Region, "S3_BUCKET": c.Bucket, "S3_ACCESS_KEY_ID": *c.AccessKeyID, "S3_SECRET_ACCESS_KEY": *c.Secret, "S3_FORCE_PATH_STYLE": "true", "S3_ALLOW_INSECURE_ENDPOINT": "true", "BLOB_DELIVERY": c.Delivery} {
		t.Setenv(key, value)
	}
}

func TestStorageSettingsAdoptEnvironmentAndUsage(t *testing.T) {
	candidate, _ := storageS3(t)
	clearStorageEnv(t)
	t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{43}, 32)))
	fx := provisionHostedGateway(t, "adopt-project", "adopt-story")
	storageCandidateEnv(t, candidate)
	cfg, err := bootstrap.BlobConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	store, err := bootstrap.OpenBlobStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := postgres.NewNarrative(fx.pool).Resolve(context.Background(), fx.project, fx.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("legacy-environment-ready-audio")
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	_, err = postgres.UploadAndPublishBlob(context.Background(), postgres.NewAudio(fx.pool), store, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "s3", SHA256: digest, Size: int64(len(data)), ContentType: "audio/wav"}, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	clearStorageEnv(t)
	if _, err = bootstrap.NewManagedBlobs(context.Background(), fx.pool, blobkey.Keys{}); err == nil {
		t.Fatal("legacy S3 rows silently adopted into default local store")
	}
	storageCandidateEnv(t, candidate)
	t.Run("requires encryption key", func(t *testing.T) {
		t.Setenv("CONFIG_ENCRYPTION_KEY", "")
		os.Unsetenv("CONFIG_ENCRYPTION_KEY")
		if _, err := bootstrap.AdoptBlobEnv(context.Background(), fx.pool, blobkey.Keys{}); !errors.Is(err, bootstrap.ErrStorageEncryption) {
			t.Fatalf("adoption without encryption key: %v", err)
		}
	})
	adopted, err := bootstrap.AdoptBlobEnv(context.Background(), fx.pool, blobkey.Keys{})
	if err != nil || !adopted.Changed {
		t.Fatalf("adoption: %+v %v", adopted, err)
	}
	var identity string
	if err = fx.pool.Pool().QueryRow(context.Background(), `SELECT store_id FROM blobs WHERE object_key=$1`, key).Scan(&identity); err != nil || identity == "" {
		t.Fatal("adoption did not record legacy blob store identity")
	}
	again, err := bootstrap.AdoptBlobEnv(context.Background(), fx.pool, blobkey.Keys{})
	if err != nil || again.Changed || again.Version != adopted.Version {
		t.Fatalf("adoption is not idempotent: %+v %v", again, err)
	}
	clearStorageEnv(t)
	manager, err := bootstrap.NewManagedBlobs(context.Background(), fx.pool, blobkey.Keys{})
	if err != nil {
		t.Fatal(err)
	}
	rc, _, err := manager.Open(context.Background(), key, blobstore.FullRange)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || !bytes.Equal(restored, data) {
		t.Fatal("adopt -> unset env -> restart lost old blob")
	}
	// Simulate a pre-00014 ready row. Maintenance backfill is explicit and
	// paginated, never a startup goroutine running with the application role.
	if _, err = fx.pool.Pool().Exec(context.Background(), `UPDATE blobs SET usage_kind='unclassified',version=version+1,updated_at=clock_timestamp() WHERE object_key=$1`, key); err != nil {
		t.Fatal(err)
	}
	if count, err := bootstrap.BackfillStorageUsage(context.Background(), fx.pool, true); err != nil || count != 1 {
		t.Fatalf("usage preview: %d %v", count, err)
	}
	usage, total, err := manager.Usage(context.Background())
	if err != nil || usage["audio"].Count != 0 || total.Count != 1 {
		t.Fatalf("unclassified total: %v %+v %v", usage, total, err)
	}
	if count, err := bootstrap.BackfillStorageUsage(context.Background(), fx.pool, false); err != nil || count != 1 {
		t.Fatalf("usage backfill: %d %v", count, err)
	}
	if count, err := bootstrap.BackfillStorageUsage(context.Background(), fx.pool, false); err != nil || count != 0 {
		t.Fatalf("usage backfill not idempotent: %d %v", count, err)
	}
	avatar, err := blobkey.Keys{}.Avatar("00000000-0000-4000-8000-000000000001", digest, "png")
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range []struct{ key, mime string }{{avatar, "image/png"}, {blobkey.Keys{}.BackupPG(time.Now().UTC()), "application/octet-stream"}} {
		_, err = postgres.UploadAndPublishBlob(context.Background(), postgres.NewAudio(fx.pool), manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: object.key, Driver: "s3", SHA256: digest, Size: int64(len(data)), ContentType: object.mime}, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
	}
	usage, total, err = manager.Usage(context.Background())
	if err != nil || usage["audio"].Count != 1 || usage["cover"].Count != 0 || usage["import"].Count != 0 || total.Count != 1 || total.Bytes != int64(len(data)) {
		t.Fatalf("avatar/backup leaked into six usage kinds: %+v %+v %v", usage, total, err)
	}
}

func TestStorageSettingsSamePhysicalStoreAndCurrentHTTP(t *testing.T) {
	candidate, _ := storageS3(t)
	f := storageFixture(t, true)
	candidate.Version = f.version(t)
	if res := f.call(t, "PUT", "/api/system/storage", candidate); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	scope, err := postgres.NewNarrative(f.fx.pool).Resolve(context.Background(), f.fx.project, f.fx.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("same-store-switch-delivery")
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	_, err = postgres.UploadAndPublishBlob(context.Background(), f.fx.g.pgAudio.Audio, f.manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "s3", SHA256: digest, Size: int64(len(data)), ContentType: "audio/wav"}, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	changed := candidate
	changed.Version = f.version(t)
	changed.Delivery = "redirect"
	changed.PresignEndpoint = candidate.Endpoint
	changed.Secret, changed.AccessKeyID = nil, nil
	res := f.call(t, "PUT", "/api/system/storage", changed)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"needsMigration":false`) {
		t.Fatalf("same store incorrectly requires migration: %d %s", res.Code, res.Body.String())
	}
	request := httptest.NewRequest("GET", "http://127.0.0.1/api/blobs/stream/"+key, nil)
	request.AddCookie(&http.Cookie{Name: f.fx.g.cookiePolicy().Name, Value: f.cookie})
	response := httptest.NewRecorder()
	f.fx.g.ServeHTTP(response, request)
	if response.Code != 307 || response.Header().Get("Location") == "" {
		t.Fatalf("in-place delivery not live: %d", response.Code)
	}
	storageCandidateEnv(t, candidate)
	locked, err := bootstrap.NewManagedBlobs(context.Background(), f.fx.pool, blobkey.Keys{})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.g.pgAudio.Storage = locked
	res = f.call(t, "POST", "/api/system/storage/test", map[string]bool{"current": true})
	var check bootstrap.StorageTestResult
	if err = json.Unmarshal(res.Body.Bytes(), &check); err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 || !check.OK {
		t.Fatalf("envlocked current test without credentials: %d %+v", res.Code, check.Steps)
	}
}

func TestStorageSettingsPendingAndStoryErrorHTTP(t *testing.T) {
	candidate, _ := storageS3(t)
	f := storageFixture(t, true)
	scope, err := postgres.NewNarrative(f.fx.pool).Resolve(context.Background(), f.fx.project, f.fx.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("missing-source-for-real-migration-error")
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	_, err = postgres.UploadAndPublishBlob(context.Background(), f.fx.g.pgAudio.Audio, f.manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "local", SHA256: digest, Size: int64(len(data)), ContentType: "audio/wav"}, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	coverKey, err := blobkey.Keys{}.Asset(postgres.UUIDString(scope.ProjectID), digest, "png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = postgres.UploadAndPublishBlob(context.Background(), f.fx.g.pgAudio.Audio, f.manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: coverKey, Driver: "local", SHA256: digest, Size: int64(len(data)), ContentType: "image/png"}, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	candidate.Version = f.version(t)
	res := f.call(t, "PUT", "/api/system/storage", candidate)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	view := f.call(t, "GET", "/api/system/storage", nil)
	var body struct {
		Pending *struct{ Provider, Bucket, EndpointHost string } `json:"pending"`
	}
	if err = json.Unmarshal(view.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Pending == nil || body.Pending.Bucket != candidate.Bucket || body.Pending.Provider != "garage" {
		t.Fatalf("pending target absent: %s", view.Body.String())
	}
	if err = f.manager.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if err = f.manager.Delete(context.Background(), coverKey); err != nil {
		t.Fatal(err)
	}
	res = f.call(t, "POST", "/api/system/storage/migration", map[string]any{"version": f.version(t)})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.manager.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := f.manager.MigrationStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if status.State == "error" {
			if len(status.Errors) != 2 {
				t.Fatalf("migration missing-source errors: %+v", status)
			}
			audio, cover := false, false
			for _, item := range status.Errors {
				if item.Code != "source_missing" {
					t.Fatalf("source missing mislabeled as target bucket failure: %+v", item)
				}
				if item.Kind == "audio" && item.Story == postgres.UUIDString(scope.StoryID) && item.StoryTitle != "" {
					audio = true
				}
				if item.Kind == "cover" && item.Story == "" && item.StoryTitle == "" {
					cover = true
				}
			}
			if !audio || !cover {
				t.Fatalf("story/shared-cover error metadata: %+v", status)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("real source-copy error not recorded")
}
