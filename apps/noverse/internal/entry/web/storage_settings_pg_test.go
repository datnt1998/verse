package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/blobstore/s3test"
)

type storageHTTPFixture struct {
	fx      *hostedFixture
	manager *bootstrap.ManagedBlobs
	cookie  string
}

func clearStorageEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if key == "STORAGE_DRIVER" || key == "BLOB_LOCAL_ROOT" || key == "BLOB_DELIVERY" || strings.HasPrefix(key, "S3_") {
			t.Setenv(key, value)
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func storageFixture(t *testing.T, key bool) *storageHTTPFixture {
	t.Helper()
	clearStorageEnv(t)
	if key {
		t.Setenv("CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{19}, 32)))
	} else {
		t.Setenv("CONFIG_ENCRYPTION_KEY", "")
		os.Unsetenv("CONFIG_ENCRYPTION_KEY")
	}
	fx := provisionHostedGateway(t, "storage-proj", "storage-story")
	manager, err := bootstrap.NewManagedBlobs(context.Background(), fx.pool, blobkey.Keys{})
	if err != nil {
		t.Fatal(err)
	}
	fx.g.SetAudioStore(&PGAudioDeps{Audio: postgres.NewAudio(fx.pool), Blobs: manager, Storage: manager})
	session, err := fx.accounts.Login("hosted-admin", "mat-khau-12345", "storage-test", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	return &storageHTTPFixture{fx: fx, manager: manager, cookie: session.Cookie}
}
func (f *storageHTTPFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "http://127.0.0.1"+path, bytes.NewReader(payload))
	req.AddCookie(&http.Cookie{Name: f.fx.g.cookiePolicy().Name, Value: f.cookie})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://127.0.0.1")
	rec := httptest.NewRecorder()
	f.fx.g.ServeHTTP(rec, req)
	return rec
}
func (f *storageHTTPFixture) version(t *testing.T) int64 {
	t.Helper()
	res := f.call(t, "GET", "/api/system/storage", nil)
	if res.Code != 200 {
		t.Fatalf("view: %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Version
}
func storageS3(t *testing.T) (bootstrap.StorageCandidate, *s3.Client) {
	t.Helper()
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("S3_TEST_ENDPOINT required")
		}
		t.Skip("S3 integration endpoint required")
	}
	access, secret := os.Getenv("S3_TEST_ACCESS_KEY"), os.Getenv("S3_TEST_SECRET_KEY")
	region := s3test.Env("S3_TEST_REGION", "garage")
	client := s3.NewFromConfig(aws.Config{Region: region, Credentials: credentials.NewStaticCredentialsProvider(access, secret, "")}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	bucket := s3test.Bucket(t, client, access, "storage-settings")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		if err == nil {
			for _, obj := range page.Contents {
				_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: obj.Key})
			}
		}
	})
	return bootstrap.StorageCandidate{Driver: "s3", Provider: "garage", Endpoint: endpoint, Region: region, Bucket: bucket, PathStyle: true, AllowHTTP: true, Delivery: "proxy", AccessKeyID: &access, Secret: &secret}, client
}

func TestStorageSettingsEnvAndEncryptionHTTP(t *testing.T) {
	t.Run("env lock", func(t *testing.T) {
		f := storageFixture(t, true)
		t.Setenv("STORAGE_DRIVER", "local")
		t.Setenv("BLOB_LOCAL_ROOT", t.TempDir())
		manager, err := bootstrap.NewManagedBlobs(context.Background(), f.fx.pool, blobkey.Keys{})
		if err != nil {
			t.Fatal(err)
		}
		f.fx.g.pgAudio.Storage = manager
		res := f.call(t, "PUT", "/api/system/storage", bootstrap.StorageCandidate{Version: f.version(t), Driver: "local", LocalRoot: t.TempDir()})
		if res.Code != 409 || !strings.Contains(res.Body.String(), "máy chủ") {
			t.Fatalf("env lock: %d %s", res.Code, res.Body.String())
		}
	})
	t.Run("no encryption and no secret disclosure", func(t *testing.T) {
		f := storageFixture(t, false)
		secret := "never-disclose-secret-123456789"
		access := "never-disclose-access-123456789"
		var logs bytes.Buffer
		previous := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
		defer slog.SetDefault(previous)
		res := f.call(t, "PUT", "/api/system/storage", bootstrap.StorageCandidate{Version: f.version(t), Driver: "s3", AccessKeyID: &access, Secret: &secret})
		if res.Code != 409 {
			t.Fatalf("secret save: %d %s", res.Code, res.Body.String())
		}
		view := f.call(t, "GET", "/api/system/storage", nil)
		if !strings.Contains(view.Body.String(), `"available":false`) {
			t.Fatal("missing unavailable encryption")
		}
		for _, text := range []string{res.Body.String(), view.Body.String(), logs.String()} {
			if strings.Contains(text, secret) || strings.Contains(text, access) {
				t.Fatal("credentials disclosed")
			}
		}
	})
}

func TestStorageSettingsGarageClassificationAndLiveSwapHTTP(t *testing.T) {
	candidate, client := storageS3(t)
	f := storageFixture(t, true)
	bad := candidate
	wrong := "wrong-secret-never-log"
	bad.Secret = &wrong
	res := f.call(t, "POST", "/api/system/storage/test", bad)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"code":"wrong_key"`) {
		t.Fatalf("wrong key: %d %s", res.Code, res.Body.String())
	}
	missing := candidate
	missing.Bucket = "missing-storage-settings-bucket"
	res = f.call(t, "POST", "/api/system/storage/test", missing)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"code":"no_bucket"`) {
		t.Fatalf("missing bucket: %d %s", res.Code, res.Body.String())
	}
	candidate.Version = f.version(t)
	res = f.call(t, "PUT", "/api/system/storage", candidate)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"needsMigration":false`) {
		t.Fatalf("live save: %d %s", res.Code, res.Body.String())
	}
	scope, err := postgres.NewNarrative(f.fx.pool).Resolve(context.Background(), f.fx.project, f.fx.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("uploaded-after-live-swap")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	key, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
	if err != nil {
		t.Fatal(err)
	}
	_, err = postgres.UploadAndPublishBlob(context.Background(), f.fx.g.pgAudio.Audio, f.manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "local", SHA256: digest, Size: int64(len(payload)), ContentType: "audio/wav"}, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	object, err := client.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(candidate.Bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(object.Body)
	object.Body.Close()
	if err != nil || !bytes.Equal(data, payload) {
		t.Fatalf("upload not routed to new bucket: %q %v", data, err)
	}
	stream := func(headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "http://127.0.0.1/api/blobs/stream/"+key, nil)
		req.AddCookie(&http.Cookie{Name: f.fx.g.cookiePolicy().Name, Value: f.cookie})
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		rec := httptest.NewRecorder()
		f.fx.g.ServeHTTP(rec, req)
		return rec
	}
	rangeResponse := stream(map[string]string{"Range": "bytes=2-8"})
	if rangeResponse.Code != 206 || !bytes.Equal(rangeResponse.Body.Bytes(), payload[2:9]) {
		t.Fatalf("proxy range: %d %q", rangeResponse.Code, rangeResponse.Body.Bytes())
	}
	etag := rangeResponse.Header().Get("ETag")
	if unchanged := stream(map[string]string{"If-None-Match": etag}); unchanged.Code != 304 || unchanged.Body.Len() != 0 {
		t.Fatalf("proxy ETag: %d", unchanged.Code)
	}
	if changed := stream(map[string]string{"Range": "bytes=2-8", "If-Range": "\"different\""}); changed.Code != 200 || !bytes.Equal(changed.Body.Bytes(), payload) {
		t.Fatalf("proxy If-Range mismatch: %d", changed.Code)
	}
	stale := candidate
	stale.Version = candidate.Version
	if conflict := f.call(t, "PUT", "/api/system/storage", stale); conflict.Code != 409 {
		t.Fatalf("stale settings CAS: %d", conflict.Code)
	}
	// Maintenance resolves the same durable encrypted config, without any blob env.
	var persisted string
	var encrypted []byte
	var keyID string
	if err := f.fx.pool.Pool().QueryRow(context.Background(), `SELECT config::text,credentials,key_id FROM blob_stores WHERE id=(SELECT active_store FROM storage_settings WHERE singleton)`).Scan(&persisted, &encrypted, &keyID); err != nil {
		t.Fatal(err)
	}
	if len(encrypted) == 0 || !strings.HasPrefix(keyID, "aes256gcm-v1:") || strings.Contains(persisted, *candidate.Secret) || strings.Contains(persisted, *candidate.AccessKeyID) || bytes.Contains(encrypted, []byte(*candidate.Secret)) {
		t.Fatal("saved credentials were not encrypted at rest")
	}
	cfg, err := bootstrap.ResolveBlobConfig(context.Background(), f.fx.pool)
	if err != nil || cfg.S3.Bucket != candidate.Bucket {
		t.Fatalf("maintenance config: %s %v", cfg.SafeSummary(), err)
	}
	restart, err := bootstrap.NewManagedBlobs(context.Background(), f.fx.pool, blobkey.Keys{})
	if err != nil {
		t.Fatal(err)
	}
	rc, _, err := restart.Open(context.Background(), key, blobstore.FullRange)
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	view := f.call(t, "GET", "/api/system/storage", nil)
	if strings.Contains(view.Body.String(), *candidate.Secret) || strings.Contains(view.Body.String(), *candidate.AccessKeyID) {
		t.Fatal("saved secret/access key disclosed")
	}
}

func TestStorageSettingsMigrationConcurrentRestartAndCancelHTTP(t *testing.T) {
	first, _ := storageS3(t)
	second, _ := storageS3(t)
	f := storageFixture(t, true)
	scope, err := postgres.NewNarrative(f.fx.pool).Resolve(context.Background(), f.fx.project, f.fx.story, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	type object struct {
		key, digest string
		data        []byte
	}
	var mu sync.Mutex
	objects := []object{}
	upload := func(i int) error {
		data := []byte(fmt.Sprintf("migration-verified-payload-%06d", i))
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		key, err := blobkey.Keys{}.AudioSegment(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), digest, "wav")
		if err != nil {
			return err
		}
		_, err = postgres.UploadAndPublishBlob(context.Background(), f.fx.g.pgAudio.Audio, f.manager, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: blobstore.DriverName(f.manager), SHA256: digest, Size: int64(len(data)), ContentType: "audio/wav"}, bytes.NewReader(data))
		if err == nil {
			mu.Lock()
			objects = append(objects, object{key, digest, data})
			mu.Unlock()
		}
		return err
	}
	for i := range 70 {
		if err := upload(i); err != nil {
			t.Fatal(err)
		}
	}
	migrate := func(candidate bootstrap.StorageCandidate, restart bool) {
		t.Helper()
		candidate.Version = f.version(t)
		res := f.call(t, "PUT", "/api/system/storage", candidate)
		if res.Code != 200 || !strings.Contains(res.Body.String(), `"needsMigration":true`) {
			t.Fatalf("prepare: %d %s", res.Code, res.Body.String())
		}
		res = f.call(t, "POST", "/api/system/storage/migration", map[string]any{"version": f.version(t)})
		if res.Code != 200 {
			t.Fatalf("start: %d %s", res.Code, res.Body.String())
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- f.manager.Run(ctx) }()
		defer func() { cancel(); <-done }()
		readStore := f.manager
		stopReads := make(chan struct{})
		readsDone := make(chan struct{})
		readErr := make(chan error, 1)
		go func() {
			defer close(readsDone)
			for {
				select {
				case <-stopReads:
					return
				default:
				}
				mu.Lock()
				snapshot := append([]object(nil), objects...)
				mu.Unlock()
				for _, obj := range snapshot {
					rc, _, err := readStore.Open(context.Background(), obj.key, blobstore.FullRange)
					if err != nil {
						select {
						case readErr <- err:
						default:
						}
						return
					}
					data, err := io.ReadAll(rc)
					rc.Close()
					if err != nil || !bytes.Equal(data, obj.data) {
						select {
						case readErr <- fmt.Errorf("read mismatch %v", err):
						default:
						}
						return
					}
				}
			}
		}()
		defer func() { close(stopReads); <-readsDone }()
		for i := 100; i < 110; i++ {
			if err := upload(i + int(candidate.Version)*100); err != nil {
				t.Fatal(err)
			}
		}
		deadline := time.Now().Add(40 * time.Second)
		restarted := false
		for time.Now().Before(deadline) {
			status, err := f.manager.MigrationStatus(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-readErr:
				t.Fatal(err)
			default:
			}
			if restart && !restarted && status.CopiedCount > 0 && status.State == "running" {
				cancel()
				<-done
				fresh, err := bootstrap.NewManagedBlobs(context.Background(), f.fx.pool, blobkey.Keys{})
				if err != nil {
					t.Fatal(err)
				}
				f.manager = fresh
				f.fx.g.pgAudio.Blobs, f.fx.g.pgAudio.Storage = fresh, fresh
				ctx, cancel = context.WithCancel(context.Background())
				defer cancel()
				done = make(chan error, 1)
				go func() { done <- fresh.Run(ctx) }()
				restarted = true
			}
			if status.State == "completed" {
				if restart && !restarted {
					t.Fatal("migration did not exercise restart")
				}
				return
			}
			if status.State == "error" {
				t.Fatalf("migration error: %+v", status)
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("migration did not finish")
	}
	migrate(first, true)
	migrate(second, false)
	// Start a third move, pause it, then cancel. No source rows or bytes change.
	local := bootstrap.StorageCandidate{Version: f.version(t), Driver: "local", LocalRoot: t.TempDir(), Delivery: "proxy"}
	res := f.call(t, "PUT", "/api/system/storage", local)
	if res.Code != 200 {
		t.Fatalf("prepare cancel: %s", res.Body.String())
	}
	res = f.call(t, "POST", "/api/system/storage/migration", map[string]any{"version": f.version(t)})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	cancelContext, stopCancel := context.WithCancel(context.Background())
	cancelDone := make(chan error, 1)
	go func() { cancelDone <- f.manager.Run(cancelContext) }()
	defer func() { stopCancel(); <-cancelDone }()
	partialDeadline := time.Now().Add(10 * time.Second)
	partial := false
	for time.Now().Before(partialDeadline) {
		status, err := f.manager.MigrationStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if status.CopiedCount > 0 && status.CopiedCount < status.TotalCount {
			partial = true
			break
		}
		if status.State == "error" || status.State == "completed" {
			t.Fatalf("cancel fixture did not remain partial: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !partial {
		t.Fatal("cancel never exercised a partially copied migration")
	}
	for _, action := range []string{"pause", "resume", "cancel"} {
		res = f.call(t, "POST", "/api/system/storage/migration/"+action, map[string]any{"version": f.version(t)})
		if res.Code != 200 {
			t.Fatalf("%s: %s", action, res.Body.String())
		}
	}
	cfg, err := bootstrap.ResolveBlobConfig(context.Background(), f.fx.pool)
	if err != nil || cfg.S3.Bucket != second.Bucket {
		t.Fatalf("cancel switched active store: %v", err)
	}
	mu.Lock()
	snapshot := append([]object(nil), objects...)
	mu.Unlock()
	for _, obj := range snapshot {
		rc, _, err := f.manager.Open(context.Background(), obj.key, blobstore.FullRange)
		if err != nil {
			t.Fatal(err)
		}
		digest, size, err := blobstore.DigestAll(rc)
		rc.Close()
		if err != nil || digest != obj.digest || size != int64(len(obj.data)) {
			t.Fatal("final verification failed")
		}
	}
}
