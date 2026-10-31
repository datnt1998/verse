package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

func TestStorageSettingsBrowserEndpointClockAndProbeHTTP(t *testing.T) {
	candidate, client := storageS3(t)
	f := storageFixture(t, true)
	target, err := url.Parse(candidate.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	publicProxy := httputil.NewSingleHostReverseProxy(target)
	public := httptest.NewServer(publicProxy)
	defer public.Close()
	candidate.Delivery = "redirect"
	res := f.call(t, "POST", "/api/system/storage/test", candidate)
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"code":"browser_endpoint_required"`) {
		t.Fatalf("internal redirect: %d %s", res.Code, res.Body.String())
	}
	// Garage accepts future signatures; an accepted response still has to fail
	// setup when its Date differs from the app clock by more than five minutes.
	skewProxy := httputil.NewSingleHostReverseProxy(target)
	skewProxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Date", time.Now().UTC().Add(10*time.Minute).Format(http.TimeFormat))
		return nil
	}
	skew := httptest.NewServer(skewProxy)
	skewed := candidate
	skewed.Delivery = "proxy"
	skewed.Endpoint = skew.URL
	res = f.call(t, "POST", "/api/system/storage/test", skewed)
	skew.Close()
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"code":"clock_skew"`) {
		t.Fatalf("Date clock check: %d %s", res.Code, res.Body.String())
	}
	candidate.PresignEndpoint = public.URL
	_, err = client.PutBucketCors(context.Background(), &s3.PutBucketCorsInput{Bucket: aws.String(candidate.Bucket), CORSConfiguration: &s3types.CORSConfiguration{CORSRules: []s3types.CORSRule{{AllowedOrigins: []string{"http://127.0.0.1"}, AllowedMethods: []string{"GET", "HEAD"}, AllowedHeaders: []string{"range", "if-none-match", "if-range"}, ExposeHeaders: []string{"content-range", "accept-ranges", "etag"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	res = f.call(t, "POST", "/api/system/storage/test", candidate)
	var result bootstrap.StorageTestResult
	if err = json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 || !result.OK || result.ProbeURL == "" {
		t.Fatalf("redirect probe result: status=%d ok=%t steps=%+v", res.Code, result.OK, result.Steps)
	}
	signed, err := url.Parse(result.ProbeURL)
	if err != nil {
		t.Fatal("invalid signed URL")
	}
	publicURL, _ := url.Parse(public.URL)
	if signed.Host != publicURL.Host {
		t.Fatal("presign used internal host instead of public signer")
	}
	var cors struct {
		CORSRules []struct{ AllowedOrigins, AllowedHeaders []string }
	}
	raw, _ := json.Marshal(result.CORS)
	if err = json.Unmarshal(raw, &cors); err != nil {
		t.Fatal(err)
	}
	if len(cors.CORSRules) != 1 || cors.CORSRules[0].AllowedOrigins[0] != "http://127.0.0.1" {
		t.Fatal("CORS policy has incorrect app origin")
	}
	for _, h := range cors.CORSRules[0].AllowedHeaders {
		if h != strings.ToLower(h) {
			t.Fatal("Garage browser preflight requires lowercase allowed header names")
		}
	}
	preflight, err := http.NewRequest(http.MethodOptions, result.ProbeURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	preflight.Header.Set("Origin", "http://127.0.0.1")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	preflight.Header.Set("Access-Control-Request-Headers", "range,if-none-match")
	response, err := http.DefaultClient.Do(preflight)
	if err != nil {
		t.Fatal("CORS preflight transport failed")
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Access-Control-Allow-Origin") != "http://127.0.0.1" {
		t.Fatalf("browser-shaped preflight status=%d", response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodGet, result.ProbeURL, nil)
	request.Header.Set("Origin", "http://127.0.0.1")
	request.Header.Set("Range", "bytes=2-8")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal("public signed Range fetch failed")
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 206 || string(data) != "verse-s" {
		t.Fatalf("public signed Range status=%d bytes=%q", response.StatusCode, data)
	}
	// Make the durable expiry due and run the real reaper: cleanup must not
	// depend on the browser touching the URL and survives manager reconstruction.
	if _, err = f.fx.pool.Pool().Exec(context.Background(), `UPDATE storage_probes SET expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	fresh, err := bootstrap.NewManagedBlobs(context.Background(), f.fx.pool, f.fx.g.pgAudio.Keys)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- fresh.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err = f.fx.pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM storage_probes`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			response, err = http.DefaultClient.Get(result.ProbeURL)
			if err != nil {
				t.Fatal("fetch expired probe failed")
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode != 404 {
				t.Fatalf("probe not deleted: %d", response.StatusCode)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expired CORS probe was not reaped")
}
