package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
)

// A successful response to a conditional write says nothing about whether the
// server enforced its condition (Garage v2.5.0 ignores it).
func TestConditionalCapabilityProbe(t *testing.T) {
	for _, conditionalStatus := range []int{http.StatusPreconditionFailed, http.StatusOK, http.StatusNotImplemented} {
		t.Run(fmt.Sprint(conditionalStatus), func(t *testing.T) {
			var probeWrites, actualWrites, deletes atomic.Int32
			store, err := NewS3(S3Config{Endpoint: "http://s3.invalid", Region: "garage", Bucket: "test", AccessKeyID: "test", SecretAccessKey: "secret", AllowPlainHTTP: true, ForcePathStyle: true, HTTPClient: &http.Client{Transport: s3TestTransport(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				payload := ""
				if req.Method == http.MethodDelete {
					deletes.Add(1)
				}
				if req.Method == http.MethodPut {
					if strings.Contains(req.URL.Path, "/staging/") {
						probeWrites.Add(1)
						if req.Header.Get("If-None-Match") == "*" {
							status = conditionalStatus
						}
					} else {
						actualWrites.Add(1)
					}
				}
				if status >= 400 {
					code := "PreconditionFailed"
					if status == 501 {
						code = "NotImplemented"
					}
					payload = "<Error><Code>" + code + "</Code></Error>"
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload)), Request: req}, nil
			})}})
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			errs := make(chan error, 20)
			var wg sync.WaitGroup
			for range 20 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, unsupported, err := store.putConditional(context.Background(), &s3.PutObjectInput{Bucket: aws.String("test"), Key: aws.String("prod/test"), Body: strings.NewReader("data"), IfNoneMatch: aws.String("*")})
					if err == nil && unsupported != (conditionalStatus != 412) {
						err = fmt.Errorf("unsupported=%v for conditional status %d", unsupported, conditionalStatus)
					}
					errs <- err
				}()
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Error(err)
				}
			}
			if probeWrites.Load() != 2 || deletes.Load() != 1 {
				t.Errorf("probe PUTs=%d deletes=%d; want two PUTs and one cleanup for twenty concurrent callers", probeWrites.Load(), deletes.Load())
			}
			want := int32(0)
			if conditionalStatus == 412 {
				want = 20
			}
			if actualWrites.Load() != want {
				t.Errorf("actual PUTs=%d want %d (one request per supported upload)", actualWrites.Load(), want)
			}
			wantState := conditionalUnsupported
			if conditionalStatus == 412 {
				wantState = conditionalSupported
			}
			if store.conditionalState.Load() != wantState {
				t.Errorf("capability state=%d want%d", store.conditionalState.Load(), wantState)
			}
		})
	}
}

func TestConditionalProbeNetworkErrorRetries(t *testing.T) {
	var failed atomic.Bool
	store, err := NewS3(S3Config{Endpoint: "http://s3.invalid", Region: "garage", Bucket: "test", AccessKeyID: "test", SecretAccessKey: "secret", AllowPlainHTTP: true, ForcePathStyle: true, HTTPClient: &http.Client{Transport: s3TestTransport(func(req *http.Request) (*http.Response, error) {
		if !failed.Load() {
			return nil, errors.New("probe transport failure")
		}
		status := 200
		payload := ""
		if req.Header.Get("If-None-Match") == "*" {
			status = 412
			payload = "<Error><Code>PreconditionFailed</Code></Error>"
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload)), Request: req}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := blobkey.New(blobkey.EnvStaging)
	if err != nil {
		t.Fatal(err)
	}
	key, err := keys.Staging("6f0c2a8e-1b3d-4c5e-9f70-8a1b2c3d4e5f")
	if err != nil {
		t.Fatal(err)
	}
	input := func() *s3.PutObjectInput {
		return &s3.PutObjectInput{Bucket: aws.String("test"), Key: aws.String(key), Body: strings.NewReader("data"), IfNoneMatch: aws.String("*")}
	}
	if _, _, err = store.putConditional(context.Background(), input()); err == nil {
		t.Fatal("network error hidden")
	}
	if store.conditionalState.Load() != conditionalUnknown {
		t.Fatal("network error cached")
	}
	failed.Store(true)
	_, _, _ = store.putConditional(context.Background(), input())
	if store.conditionalState.Load() != conditionalSupported {
		t.Fatal("probe was not retried")
	}
}
