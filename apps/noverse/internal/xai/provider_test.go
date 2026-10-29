package xai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/noverse/internal/llm/wire"
)

type fakeTokens struct {
	mu          sync.Mutex
	current     string
	next        string
	invalidated []string
	invalidErr  error
}

func (f *fakeTokens) Token(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current, nil
}

func (f *fakeTokens) Invalidate(_ context.Context, stale string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, stale)
	if f.invalidErr != nil {
		return "", f.invalidErr
	}
	f.current = f.next
	return f.current, nil
}

type captured struct {
	path   string
	header http.Header
	body   map[string]any
}

const chatOK = `{"id":"c1","model":"grok-4.6","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Xin chào"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`

// newAPI giả lập api.x.ai: 401 với "Bearer expired", 403 với "Bearer nogrok", còn lại trả chat completion.
func newAPI(t *testing.T, models string) (*httptest.Server, *[]captured) {
	t.Helper()
	var mu sync.Mutex
	var seen []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		mu.Lock()
		seen = append(seen, captured{path: r.URL.Path, header: r.Header.Clone(), body: body})
		mu.Unlock()
		switch r.Header.Get("Authorization") {
		case "Bearer expired":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"token expired"}}`)
			return
		case "Bearer nogrok":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"plan does not include API access"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			_, _ = io.WriteString(w, chatOK)
		case "/v1/models":
			if models == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, models)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func chat(t *testing.T, srv *httptest.Server, tokens TokenSource, extra map[string]any) (*litellm.Response, error) {
	t.Helper()
	p, err := New(Config{BaseURL: srv.URL + "/v1", Tokens: tokens, Headers: map[string]string{"X-Custom": "1"}, ExtraBody: extra})
	if err != nil {
		t.Fatal(err)
	}
	return p.Chat(context.Background(), &litellm.Request{
		Model:    "grok-4.6",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "Viết một câu."}}}},
	})
}

func TestChatSendsBearerAndExtras(t *testing.T) {
	srv, seen := newAPI(t, "")
	resp, err := chat(t, srv, &fakeTokens{current: "tok-1"}, map[string]any{"service_tier": "flex"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text() != "Xin chào" {
		t.Fatalf("content = %q", resp.Text())
	}
	got := (*seen)[0]
	if got.path != "/v1/chat/completions" || got.header.Get("Authorization") != "Bearer tok-1" || got.header.Get("X-Custom") != "1" {
		t.Fatalf("request = %+v", got)
	}
	if got.body["model"] != "grok-4.6" || got.body["service_tier"] != "flex" {
		t.Fatalf("body = %v", got.body)
	}
}

func TestUnauthorizedRefreshesOnceAndRetriesWithNewToken(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "expired", next: "fresh"}
	resp, err := chat(t, srv, tokens, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text() != "Xin chào" {
		t.Fatalf("content = %q", resp.Text())
	}
	if len(tokens.invalidated) != 1 || tokens.invalidated[0] != "expired" {
		t.Fatalf("Invalidate phải được gọi đúng một lần với token stale: %v", tokens.invalidated)
	}
	if len(*seen) != 2 || (*seen)[0].header.Get("Authorization") != "Bearer expired" || (*seen)[1].header.Get("Authorization") != "Bearer fresh" {
		t.Fatalf("requests = %+v", *seen)
	}
	// Body phải được gửi lại đầy đủ ở lần thử thứ hai.
	if (*seen)[1].body["model"] != "grok-4.6" {
		t.Fatalf("retry body = %v", (*seen)[1].body)
	}
}

func TestUnauthorizedTwiceDoesNotLoop(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "expired", next: "expired"}
	if _, err := chat(t, srv, tokens, nil); err == nil {
		t.Fatal("token vẫn bị từ chối thì phải trả lỗi")
	}
	if len(*seen) != 2 || len(tokens.invalidated) != 1 {
		t.Fatalf("chỉ được thử lại đúng một lần: requests=%d invalidated=%v", len(*seen), tokens.invalidated)
	}
}

func TestForbiddenShowsVietnameseTierMessageWithoutRefresh(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "nogrok", next: "other"}
	_, err := chat(t, srv, tokens, nil)
	if err == nil {
		t.Fatal("403 phải là lỗi")
	}
	var le *litellm.LiteLLMError
	if !errors.As(err, &le) || le.StatusCode != 403 || le.Code != "xai_oauth_tier_denied" {
		t.Fatalf("err = %#v", err)
	}
	for _, want := range []string{"chưa được xAI cấp quyền gọi API", "provider \"grok\"", "https://x.ai/grok", "plan does not include API access"} {
		if !strings.Contains(le.Message, want) {
			t.Errorf("thông báo thiếu %q: %q", want, le.Message)
		}
	}
	if len(tokens.invalidated) != 0 || len(*seen) != 1 {
		t.Fatalf("403 không được kích hoạt refresh/retry: invalidated=%v requests=%d", tokens.invalidated, len(*seen))
	}
}

func TestListModelsLiveFiltersNonChatAndFallsBack(t *testing.T) {
	live := `{"data":[{"id":"grok-4.6"},{"id":"grok-4.3"},{"id":"grok-imagine-image"},{"id":"grok-4.3"},{"id":"text-embedding-x"}]}`
	srv, _ := newAPI(t, live)
	got, err := ListModels(context.Background(), srv.URL+"/v1", &fakeTokens{current: "tok"})
	if err != nil || len(got) != 2 || got[0].Slug != "grok-4.6" || got[1].Slug != "grok-4.3" {
		t.Fatalf("live = %+v err=%v", got, err)
	}

	// /v1/models không dùng được với token OAuth → danh sách tĩnh.
	srv404, _ := newAPI(t, "")
	got, err = ListModels(context.Background(), srv404.URL+"/v1", &fakeTokens{current: "tok"})
	if err != nil || len(got) != len(FallbackModels()) || got[0].Slug != DefaultModel {
		t.Fatalf("fallback = %+v err=%v", got, err)
	}
	// 403 cũng rơi về danh sách tĩnh (inference mới là nơi báo lỗi gói).
	got, err = ListModels(context.Background(), srv404.URL+"/v1", &fakeTokens{current: "nogrok"})
	if err != nil || got[0].Slug != DefaultModel {
		t.Fatalf("403 fallback = %+v err=%v", got, err)
	}
}

func TestListModelsPropagatesTokenError(t *testing.T) {
	boom := errors.New("chưa đăng nhập")
	_, err := ListModels(context.Background(), "http://127.0.0.1:1", errTokens{boom})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

type errTokens struct{ err error }

func (e errTokens) Token(context.Context) (string, error)              { return "", e.err }
func (e errTokens) Invalidate(context.Context, string) (string, error) { return "", e.err }

func TestValidateBaseURLPinsBearerToXAI(t *testing.T) {
	for _, ok := range []string{"", "https://api.x.ai/v1", "https://eu-west-1.api.x.ai/v1"} {
		if err := ValidateBaseURL(ok); err != nil {
			t.Errorf("ValidateBaseURL(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"http://api.x.ai/v1", "https://api.openai.com/v1", "https://x.ai.evil.example/v1", "://bad"} {
		if err := ValidateBaseURL(bad); err == nil {
			t.Errorf("ValidateBaseURL(%q) = nil, want lỗi", bad)
		}
	}
}

func TestNewRequiresTokenSource(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("thiếu nguồn token phải báo lỗi")
	}
}

func TestStreamRetriesOn401AndDeliversChunks(t *testing.T) {
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "Bearer expired" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			`{"id":"c1","model":"grok-4.6","choices":[{"index":0,"delta":{"role":"assistant","content":"Xin "}}]}`,
			`{"id":"c1","model":"grok-4.6","choices":[{"index":0,"delta":{"content":"chào"},"finish_reason":"stop"}]}`,
		} {
			_, _ = io.WriteString(w, "data: "+chunk+"\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	tokens := &fakeTokens{current: "expired", next: "fresh"}
	p, err := New(Config{BaseURL: srv.URL + "/v1", Tokens: tokens})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := p.Stream(context.Background(), &litellm.Request{
		Model:    "grok-4.6",
		Messages: []litellm.Message{{Role: litellm.RoleUser, Blocks: []litellm.Block{litellm.TextBlock{Text: "Viết một câu."}}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()
	var text strings.Builder
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		if delta, ok := chunk.(litellm.ContentDelta); ok {
			text.WriteString(delta.Text)
		}
	}
	if text.String() != "Xin chào" {
		t.Fatalf("stream text = %q", text.String())
	}
	if len(auths) != 2 || auths[0] != "Bearer expired" || auths[1] != "Bearer fresh" {
		t.Fatalf("auth headers = %v", auths)
	}
}
