package blobstore

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3DeleteManyBatchesAndRetriesOnlyFailedKeys(t *testing.T) {
	var sizes []int
	attempts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Objects []struct {
				Key string `xml:"Key"`
			} `xml:"Object"`
		}
		if err := xml.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "bad XML", 400)
			return
		}
		sizes = append(sizes, len(request.Objects))
		var response strings.Builder
		response.WriteString(`<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
		for _, o := range request.Objects {
			attempts[o.Key]++
			if o.Key == "dev/key-0007" && attempts[o.Key] == 1 {
				fmt.Fprintf(&response, `<Error><Key>%s</Key><Code>InternalError</Code><Message>transient</Message></Error>`, o.Key)
			}
		}
		response.WriteString(`</DeleteResult>`)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, response.String())
	}))
	defer server.Close()
	store, err := NewS3(S3Config{Endpoint: server.URL, Region: "us-east-1", Bucket: "test-batch", AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true, AllowPlainHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 1001)
	for i := range keys {
		keys[i] = fmt.Sprintf("dev/key-%04d", i)
	}
	results, err := DeleteMany(context.Background(), store, keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1001 {
		t.Fatalf("results=%d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("key %s: %v", r.Key, r.Err)
		}
	}
	if fmt.Sprint(sizes) != "[1000 1 1]" {
		t.Fatalf("1001 keys must use two batches plus one failed-key retry: sizes=%v", sizes)
	}
	for key, n := range attempts {
		want := 1
		if key == "dev/key-0007" {
			want = 2
		}
		if n != want {
			t.Fatalf("%s attempted %d times, want %d", key, n, want)
		}
	}
}

func TestLocalAndSequentialBatchFallback(t *testing.T) {
	local, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	for _, batch := range []BlobStore{local, struct{ BlobStore }{local}} {
		if _, err := local.Put(context.Background(), "dev/valid", strings.NewReader("bytes"), PutOptions{}); err != nil {
			t.Fatal(err)
		}
		out, err := DeleteMany(context.Background(), batch, []string{"../escape", "dev/valid", "dev/absent"})
		if err != nil || len(out) != 3 || out[0].Err == nil || out[1].Err != nil || out[2].Err != nil {
			t.Fatalf("fallback outcomes: %+v %v", out, err)
		}
		if _, err := local.Stat(context.Background(), "dev/valid"); !IsNotFound(err) {
			t.Fatalf("valid key not deleted: %v", err)
		}
	}
}
