// Package s3test provisions disposable integration buckets. Garage uses its
// authenticated admin API; other providers may use the S3 bucket API.
package s3test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func Env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// Bucket creates a unique bucket and grants the supplied key owner/read/write
// access. Callers register object cleanup after this call, so it runs first.
func Bucket(t *testing.T, client *s3.Client, accessKey, base string) string {
	t.Helper()
	name := fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
	admin := strings.TrimRight(os.Getenv("S3_TEST_ADMIN_ENDPOINT"), "/")
	if admin != "" {
		var bucket struct {
			ID string `json:"id"`
		}
		adminCall(t, admin, "CreateBucket", map[string]any{"globalAlias": name}, &bucket)
		adminCall(t, admin, "AllowBucketKey", map[string]any{"bucketId": bucket.ID, "accessKeyId": accessKey, "permissions": map[string]bool{"read": true, "write": true, "owner": true}}, nil)
		t.Cleanup(func() { adminCall(t, admin, "DeleteBucket", map[string]any{"id": bucket.ID}, nil) })
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)}); err != nil {
			t.Fatalf("CreateBucket (Garage requires S3_TEST_ADMIN_ENDPOINT and GARAGE_ADMIN_TOKEN): %v", err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(name)}); err != nil {
				t.Errorf("DeleteBucket: %v", err)
			}
		})
	}
	return name
}

func adminCall(t *testing.T, endpoint, operation string, payload any, result any) {
	t.Helper()
	token := os.Getenv("GARAGE_ADMIN_TOKEN")
	if token == "" {
		t.Fatal("GARAGE_ADMIN_TOKEN required with S3_TEST_ADMIN_ENDPOINT")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	target := endpoint + "/v2/" + operation
	if operation == "DeleteBucket" {
		target += "?id=" + url.QueryEscape(payload.(map[string]any)["id"].(string))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Garage %s: HTTP %d: %s", operation, resp.StatusCode, data)
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			t.Fatal(err)
		}
	}
}
