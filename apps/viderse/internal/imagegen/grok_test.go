// Ca test chuyển từ src/server/tools/grok.test.ts của yt-factory (phần vẽ
// ảnh): body đăng ký, edits endpoint với reference, xếp loại 401/403/429,
// chưa đăng nhập không gọi mạng, và ánh xạ aspect ratio.
package imagegen_test

import (
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
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
)

// staticTokens là xai.TokenSource cố định cho test.
type staticTokens struct {
	token string
	err   error
	calls *int
}

func (s staticTokens) Token(ctx context.Context) (string, error) {
	if s.calls != nil {
		*s.calls++
	}
	return s.token, s.err
}

func (s staticTokens) Invalidate(ctx context.Context, stale string) (string, error) {
	return s.token, s.err
}

// grokFixture dựng provider trỏ vào httptest server; handler đăng ký theo
// path để URL không khớp fail thay vì rò request thật.
type grokFixture struct {
	provider *imagegen.GrokProvider
	server   *httptest.Server
	requests chan capturedRequest
}

type capturedRequest struct {
	Path        string
	Method      string
	Auth        string
	ContentType string
	Body        []byte
}

func newGrokFixture(t *testing.T, handler http.HandlerFunc) *grokFixture {
	t.Helper()
	requests := make(chan capturedRequest, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- capturedRequest{
			Path:        r.URL.Path,
			Method:      r.Method,
			Auth:        r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	p := imagegen.NewGrokProvider(imagegen.GrokDeps{
		Tokens:    staticTokens{token: "test-access-token"},
		Client:    server.Client(),
		BaseURL:   server.URL + "/v1",
		ReadyFunc: func(ctx context.Context) error { return nil },
	})
	return &grokFixture{provider: p, server: server, requests: requests}
}

func okImageResponse(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("x-request-id", "req-test-1")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(testPNG)}},
	})
}

// "the image call sends the subscription body, writes a valid PNG and a
// Grok receipt"
func TestGrokDrawSendsSubscriptionBody(t *testing.T) {
	fx := newGrokFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("path = %s", r.URL.Path)
		}
		okImageResponse(w)
	})
	out := filepath.Join(t.TempDir(), "shot.png")
	req := imagegen.Request{
		Key: "g1", Prompt: "a brave duke on the bridge",
		Output: out, Width: 1280, Height: 720,
	}
	receipt, err := fx.provider.Draw(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	call := <-fx.requests
	if call.Method != http.MethodPost || call.Auth != "Bearer test-access-token" || call.ContentType != "application/json" {
		t.Fatalf("request = %+v", call)
	}
	var body struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		AspectRatio    string `json:"aspect_ratio"`
		Resolution     string `json:"resolution"`
		N              int    `json:"n"`
		ResponseFormat string `json:"response_format"`
	}
	if err := json.Unmarshal(call.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != "grok-imagine-image" || body.Prompt != "a brave duke on the bridge" ||
		body.AspectRatio != "16:9" || body.Resolution != "1k" || body.N != 1 || body.ResponseFormat != "b64_json" {
		t.Fatalf("body = %+v", body)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != string(testPNG) {
		t.Fatalf("output: %v", err)
	}
	if err := imagegen.ValidatePngFile(out); err != nil {
		t.Fatalf("output không phải PNG hợp lệ: %v", err)
	}
	// Receipt + sidecar giống bản cũ.
	if receipt.Provider != "grok" || receipt.Model != "grok-imagine-image" || receipt.RequestID != "req-test-1" {
		t.Fatalf("receipt = %+v", receipt)
	}
	side, err := os.ReadFile(out + ".receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var stored imagegen.Receipt
	if err := json.Unmarshal(side, &stored); err != nil {
		t.Fatal(err)
	}
	if stored != receipt {
		t.Fatalf("sidecar = %+v", stored)
	}
	sum, _ := imagegenSha(out)
	if receipt.PngSha256 != sum {
		t.Fatalf("pngSha256 = %s, want %s", receipt.PngSha256, sum)
	}
}

// "reference draws use the JSON edits endpoint with a base64 data URI and
// the same model and auth"
func TestGrokReferenceDrawUsesEditsEndpoint(t *testing.T) {
	fx := newGrokFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Errorf("path = %s — reference phải qua edits", r.URL.Path)
		}
		okImageResponse(w)
	})
	root := t.TempDir()
	reference := filepath.Join(root, "reference.png")
	if err := os.WriteFile(reference, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "output.png")
	_, err := fx.provider.Draw(context.Background(), imagegen.Request{
		Key: "g2", Prompt: "new face, same composition", Reference: reference, Output: out,
	}, nil)
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	call := <-fx.requests
	var body struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Image  struct {
			URL string `json:"url"`
		} `json:"image"`
		ResponseFormat string `json:"response_format"`
		N              int    `json:"n"`
	}
	if err := json.Unmarshal(call.Body, &body); err != nil {
		t.Fatal(err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNG)
	if body.Model != "grok-imagine-image" || body.Prompt != "new face, same composition" ||
		body.Image.URL != want || body.ResponseFormat != "b64_json" || body.N != 1 {
		t.Fatalf("body = %+v", body)
	}
	// Reference không phải ảnh → invalid_request trước khi gọi mạng.
	if err := os.WriteFile(reference, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = fx.provider.Draw(context.Background(), imagegen.Request{
		Key: "g3", Prompt: "x", Reference: reference, Output: filepath.Join(root, "bad.png"),
	}, nil)
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeInvalidRequest {
		t.Fatalf("mong invalid_request, được %v", err)
	}
}

// "403 subscription wording is a quota pause, not an auth failure — no
// refresh is attempted"
func TestGrok403IsQuotaNotAuth(t *testing.T) {
	fx := newGrokFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte("You have run out of credits or need a Grok subscription"))
	})
	out := filepath.Join(t.TempDir(), "shot.png")
	_, err := fx.provider.Draw(context.Background(), imagegen.Request{Key: "g4", Prompt: "x", Output: out}, nil)
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeQuota {
		t.Fatalf("mong quota, được %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatal("output không được tạo khi quota")
	}
}

// "401 means the login expired: tool_missing pointing at Settings"
func TestGrok401IsToolMissing(t *testing.T) {
	fx := newGrokFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte("Unauthorized"))
	})
	out := filepath.Join(t.TempDir(), "shot.png")
	_, err := fx.provider.Draw(context.Background(), imagegen.Request{Key: "g5", Prompt: "x", Output: out}, nil)
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeToolMissing {
		t.Fatalf("mong tool_missing, được %v", err)
	}
}

// "429 is a provider-busy rejection the queue retries"
func TestGrok429IsBusy(t *testing.T) {
	fx := newGrokFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte("busy"))
	})
	out := filepath.Join(t.TempDir(), "shot.png")
	_, err := fx.provider.Draw(context.Background(), imagegen.Request{Key: "g6", Prompt: "x", Output: out}, nil)
	if !imagegen.IsProviderBusy(err) {
		t.Fatalf("mong ProviderBusyError, được %v", err)
	}
}

// "drawing without a login asks for Settings before any network"
func TestGrokNoLoginNoNetwork(t *testing.T) {
	hit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	defer server.Close()
	p := imagegen.NewGrokProvider(imagegen.GrokDeps{
		Tokens:  staticTokens{err: errors.New("chưa đăng nhập")},
		Client:  server.Client(),
		BaseURL: server.URL,
	})
	_, err := p.Draw(context.Background(), imagegen.Request{
		Key: "g7", Prompt: "x", Output: filepath.Join(t.TempDir(), "o.png"),
	}, nil)
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeToolMissing {
		t.Fatalf("mong tool_missing, được %v", err)
	}
	if hit {
		t.Fatal("request thoát ra khi chưa đăng nhập")
	}
}

// "grokAspectRatio maps the canvases callers pass"
func TestGrokAspectRatio(t *testing.T) {
	cases := map[[2]int]string{
		{1280, 720}:  "16:9",
		{1920, 1080}: "16:9",
		{800, 800}:   "1:1",
		{2560, 1440}: "16:9",
		{0, 0}:       "auto",
	}
	for size, want := range cases {
		if got := imagegen.GrokAspectRatio(size[0], size[1]); got != want {
			t.Fatalf("GrokAspectRatio(%d,%d) = %s, want %s", size[0], size[1], got, want)
		}
	}
}

// imagegenSha đọc sha256 của file cho kiểm tra receipt.
func imagegenSha(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
