package serverstorage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/blobstore/s3test"
)

type garageKey struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

// Test admin payloads are the Garage API's JSON boundary, not app DTOs.
func garageAdmin(t *testing.T, op string, payload any, result any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := strings.TrimRight(os.Getenv("S3_TEST_ADMIN_ENDPOINT"), "/") + "/v2/" + op
	if op == "DeleteKey" {
		endpoint += "?id=" + url.QueryEscape(payload.(map[string]string)["id"])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GARAGE_ADMIN_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("Garage %s status %d", op, response.StatusCode)
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			t.Fatal(err)
		}
	}
}
func TestGarageSeparateRuntimeKeysCannotReadOtherAppBucket(t *testing.T) {
	config := garageConfig(t)
	if os.Getenv("S3_TEST_ADMIN_ENDPOINT") == "" {
		if os.Getenv("REQUIRE_S3_TEST") == "1" {
			t.Fatal("Garage admin endpoint required")
		}
		t.Skip("Garage admin endpoint not set")
	}
	var viderse, noverse garageKey
	garageAdmin(t, "CreateKey", map[string]string{"name": "viderse-backend-isolation"}, &viderse)
	t.Cleanup(func() { garageAdmin(t, "DeleteKey", map[string]string{"id": viderse.AccessKeyID}, nil) })
	garageAdmin(t, "CreateKey", map[string]string{"name": "noverse-backend-isolation"}, &noverse)
	t.Cleanup(func() { garageAdmin(t, "DeleteKey", map[string]string{"id": noverse.AccessKeyID}, nil) })
	vBucket := s3test.Bucket(t, nil, viderse.AccessKeyID, "vs3backend-viderse")
	nBucket := s3test.Bucket(t, nil, noverse.AccessKeyID, "vs3backend-noverse")
	open := func(bucket string, key garageKey) blobstore.BlobStore {
		store, err := blobstore.NewS3(blobstore.S3Config{Endpoint: config.Endpoint, Region: config.Region, Bucket: bucket, AccessKeyID: key.AccessKeyID, SecretAccessKey: key.SecretAccessKey, ForcePathStyle: true, AllowPlainHTTP: true})
		if err != nil {
			t.Fatal(err)
		}
		return store
	}
	vStore := open(vBucket, viderse)
	nStore := open(nBucket, noverse)
	ctx := context.Background()
	key := "staging/isolation/owned"
	for _, store := range []blobstore.BlobStore{vStore, nStore} {
		if _, err := store.Put(ctx, key, bytes.NewReader([]byte("owned only by this app")), blobstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := store.Delete(context.Background(), key); err != nil {
				t.Error(err)
			}
		})
	}
	if _, err := vStore.Stat(ctx, key); err != nil {
		t.Fatal("Viderse cannot read its own bucket")
	}
	_, err := open(nBucket, viderse).Stat(ctx, key)
	var api smithy.APIError
	if !errors.As(err, &api) || (api.ErrorCode() != "AccessDenied" && api.ErrorCode() != "Forbidden") {
		t.Fatalf("Viderse key read another app bucket: code=%v", err)
	}
}
