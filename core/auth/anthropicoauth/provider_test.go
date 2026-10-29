package anthropicoauth

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

	"github.com/datnt1998/verse/core/auth"
	"github.com/datnt1998/verse/core/llm/provider/anthropic"
	"github.com/datnt1998/verse/core/llm/wire"
)

type fakeTokens struct {
	mu          sync.Mutex
	current     string
	next        string
	invalidated []string
	invalidErr  error
}

func (f *fakeTokens) Credential(context.Context) (auth.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return auth.Credential{Access: f.current, AccountID: "11111111-2222-3333-4444-555555555555"}, nil
}

func (f *fakeTokens) Invalidate(_ context.Context, stale string) (auth.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, stale)
	if f.invalidErr != nil {
		return auth.Credential{}, f.invalidErr
	}
	f.current = f.next
	return auth.Credential{Access: f.current}, nil
}

func (f *fakeTokens) InstallID() string { return "install-test" }

type captured struct {
	path   string
	query  string
	header http.Header
	raw    []byte
	body   map[string]any
}

const messageOK = `{"id":"m1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"Xin chào"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`

// newAPI giả lập api.anthropic.com: "Bearer expired" → 401, "Bearer forbidden" → 403,
// "Bearer limited" → 429, còn lại trả Message (hoặc SSE nếu stream=true).
func newAPI(t *testing.T, sse string) (*httptest.Server, *[]captured) {
	t.Helper()
	var mu sync.Mutex
	var seen []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		mu.Lock()
		seen = append(seen, captured{path: r.URL.Path, query: r.URL.RawQuery, header: r.Header.Clone(), raw: data, body: body})
		mu.Unlock()
		switch r.Header.Get("Authorization") {
		case "Bearer expired":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"OAuth token has expired"}}`)
			return
		case "Bearer forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"permission_error","message":"OAuth authentication is currently not allowed for this organization"}}`)
			return
		case "Bearer limited":
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"Number of request tokens has exceeded your per-minute rate limit"}}`)
			return
		}
		if stream, _ := body["stream"].(bool); stream && sse != "" {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, messageOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func newModel(t *testing.T, srv *httptest.Server, tokens TokenSource) *anthropic.Provider {
	t.Helper()
	provider, err := New(Config{BaseURL: srv.URL, Tokens: tokens})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func chatRequest(tools bool) *litellm.Request {
	req := &litellm.Request{
		Model: "claude-sonnet-4-6",
		Messages: []litellm.Message{
			litellm.System("Bạn là trợ lý viết truyện."),
			litellm.UserText("Viết chương một."),
		},
		MaxTokens: litellm.IntPtr(256),
	}
	if tools {
		tool, err := litellm.NewTool("read_chapter", "Đọc chương", map[string]any{
			"type":       "object",
			"properties": map[string]any{"n": map[string]any{"type": "integer"}},
		})
		if err != nil {
			panic(err)
		}
		req.Tools = []litellm.Tool{tool}
	}
	return req
}

func TestRequestCarriesBearerOmpHeadersBetasAndBetaQuery(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "tok-live"}
	resp, err := newModel(t, srv, tokens).Chat(context.Background(), chatRequest(true))
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text() != "Xin chào" {
		t.Fatalf("content = %q", resp.Text())
	}
	got := (*seen)[0]
	if got.path != "/v1/messages" || got.query != "beta=true" {
		t.Fatalf("path/query = %s ? %s, muốn /v1/messages?beta=true", got.path, got.query)
	}
	h := got.header
	if h.Get("Authorization") != "Bearer tok-live" {
		t.Errorf("Authorization = %q", h.Get("Authorization"))
	}
	if h.Get("x-api-key") != "" {
		t.Errorf("x-api-key phải bị bỏ, có %q", h.Get("x-api-key"))
	}
	if h.Get("x-app") != "cli" || h.Get("anthropic-dangerous-direct-browser-access") != "true" {
		t.Errorf("thiếu x-app/dangerous-direct: %v", h)
	}
	if want := auth.ClaudeCodeUserAgent(); h.Get("User-Agent") != want {
		t.Errorf("User-Agent = %q, muốn %q", h.Get("User-Agent"), want)
	}
	if h.Get("X-Claude-Code-Session-Id") == "" || h.Get("X-Stainless-Lang") != "js" || h.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("thiếu header Claude Code: %v", h)
	}
	betas := strings.Split(h.Get("anthropic-beta"), ",")
	for _, want := range []string{betaClaudeCode, betaOAuth, betaInterleaved, betaCachingScope, betaFallbackCredit} {
		if !contains(betas, want) {
			t.Errorf("anthropic-beta thiếu %q: %v", want, betas)
		}
	}
	if contains(betas, "context-1m-2025-08-07") {
		t.Errorf("không được gửi context-1m: %v", betas)
	}
}

func TestUtilityRequestUsesReducedBetas(t *testing.T) {
	srv, seen := newAPI(t, "")
	if _, err := newModel(t, srv, &fakeTokens{current: "tok"}).Chat(context.Background(), chatRequest(false)); err != nil {
		t.Fatal(err)
	}
	betas := strings.Split((*seen)[0].header.Get("anthropic-beta"), ",")
	if contains(betas, betaClaudeCode) || !contains(betas, betaOAuth) || !contains(betas, betaStructuredOutputs) {
		t.Fatalf("bộ beta rút gọn sai: %v", betas)
	}
	if tools, ok := (*seen)[0].body["tools"].([]any); !ok || len(tools) != 0 {
		t.Fatalf("khi không có tool phải gửi tools:[], có %v", (*seen)[0].body["tools"])
	}
}

func TestBodyHasBillingAndIdentityBlocksAndToolPrefix(t *testing.T) {
	srv, seen := newAPI(t, "")
	if _, err := newModel(t, srv, &fakeTokens{current: "tok"}).Chat(context.Background(), chatRequest(true)); err != nil {
		t.Fatal(err)
	}
	got := (*seen)[0]
	system, _ := got.body["system"].([]any)
	if len(system) != 3 {
		t.Fatalf("system = %v, muốn 3 khối [billing, nhận dạng, gốc]", got.body["system"])
	}
	billing := system[0].(map[string]any)["text"].(string)
	if !strings.HasPrefix(billing, "x-anthropic-billing-header: cc_version="+auth.ClaudeCodeVersion()+".") || !strings.Contains(billing, "cc_entrypoint=cli;") {
		t.Errorf("billing header sai: %q", billing)
	}
	if strings.Contains(billing, "cch=00000") {
		t.Errorf("cch chưa được vá chữ ký: %q", billing)
	}
	identity := system[1].(map[string]any)
	if identity["text"] != auth.ClaudeCodeIdentity {
		t.Errorf("khối nhận dạng = %v", identity["text"])
	}
	if cc, _ := identity["cache_control"].(map[string]any); cc["ttl"] != "1h" {
		t.Errorf("cache_control khối nhận dạng = %v, muốn ttl 1h", identity["cache_control"])
	}
	if system[2].(map[string]any)["text"] != "Bạn là trợ lý viết truyện." {
		t.Errorf("khối system gốc bị mất: %v", system[2])
	}
	tools := got.body["tools"].([]any)
	if name := tools[0].(map[string]any)["name"]; name != "_read_chapter" {
		t.Errorf("tool name = %v, muốn _read_chapter", name)
	}
	meta := got.body["metadata"].(map[string]any)
	if uid, _ := meta["user_id"].(string); !strings.Contains(uid, `"session_id"`) || !strings.Contains(uid, "11111111-2222-3333-4444-555555555555") {
		t.Errorf("metadata.user_id = %q", uid)
	}
}

func TestCCHSignatureMatchesBodyHash(t *testing.T) {
	srv, seen := newAPI(t, "")
	if _, err := newModel(t, srv, &fakeTokens{current: "tok"}).Chat(context.Background(), chatRequest(true)); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(nil), (*seen)[0].raw...)
	idx := strings.Index(string(raw), "cch=")
	if idx < 0 {
		t.Fatal("không thấy cch")
	}
	sig := string(raw[idx+4 : idx+9])
	copy(raw[idx+4:], "00000")
	if want := formatCCH(xxh64(raw, cchSeed)); sig != want {
		t.Fatalf("cch = %s, hash body với placeholder = %s", sig, want)
	}
}

func TestXXH64KnownVectors(t *testing.T) {
	for in, want := range map[string]uint64{
		"":    0xEF46DB3751D8E999,
		"a":   0xD24EC4F1A98C6E5B,
		"abc": 0x44BC2CF5AD770999,
		"Nobody inspects the spammish repetition": 0xFBCEA83C8A378BF1,
	} {
		if got := xxh64([]byte(in), 0); got != want {
			t.Errorf("xxh64(%q) = %#x, muốn %#x", in, got, want)
		}
	}
}

func TestToolPrefixRoundTripInStream(t *testing.T) {
	sse := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"usage":{"input_tokens":3,"output_tokens":0}}}` + "\n\n" +
		"event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"_read_chapter","input":{}}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"n\":2}"}}` + "\n\n" +
		"event: content_block_stop\n" +
		`data: {"type":"content_block_stop","index":0}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"
	srv, _ := newAPI(t, sse)
	stream, err := newModel(t, srv, &fakeTokens{current: "tok"}).Stream(context.Background(), chatRequest(true))
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()
	var names []string
	for {
		ev, err := stream.Next()
		if err != nil {
			break
		}
		if start, ok := ev.(litellm.ToolUseStart); ok {
			names = append(names, start.Name)
		}
	}
	if len(names) != 1 || names[0] != "read_chapter" {
		t.Fatalf("tên tool trong stream = %v, muốn [read_chapter] (đã gỡ tiền tố)", names)
	}
}

func TestToolPrefixRoundTripInJSONResponse(t *testing.T) {
	raw := `{"id":"m","type":"message","content":[{"type":"text","text":"x"},{"type":"tool_use","id":"t1","name":"_read_chapter","input":{"n":1}},{"type":"tool_use","id":"t2","name":"web_search","input":{}}]}`
	out := string(unprefixMessage([]byte(raw)))
	if !strings.Contains(out, `"name":"read_chapter"`) || !strings.Contains(out, `"name":"web_search"`) || strings.Contains(out, `_read_chapter`) {
		t.Fatalf("unprefixMessage = %s", out)
	}
}

func TestRequestToolHistoryIsPrefixed(t *testing.T) {
	body := `{"model":"m","max_tokens":10,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"read_chapter","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}],"tool_choice":{"type":"tool","name":"read_chapter"},"tools":[{"name":"read_chapter","input_schema":{"type":"object"}},{"name":"web_search"}]}`
	out, shape, err := rewriteMessages([]byte(body), rewriteOptions{InstallID: "i", SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"name":"_read_chapter"`, `"name":"web_search"`} {
		if !strings.Contains(s, want) {
			t.Errorf("thiếu %s trong %s", want, s)
		}
	}
	if strings.Count(s, `"name":"_read_chapter"`) != 3 {
		t.Errorf("muốn tiền tố ở tools, tool_use và tool_choice: %s", s)
	}
	if !shape.HasTools {
		t.Error("shape.HasTools phải true")
	}
}

func TestUnauthorizedRefreshesOnceAndReplays(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "expired", next: "fresh"}
	if _, err := newModel(t, srv, tokens).Chat(context.Background(), chatRequest(true)); err != nil {
		t.Fatalf("Chat sau refresh: %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("số request = %d, muốn 2 (401 rồi gửi lại)", len(*seen))
	}
	if (*seen)[0].header.Get("Authorization") != "Bearer expired" || (*seen)[1].header.Get("Authorization") != "Bearer fresh" {
		t.Errorf("Authorization: %q → %q", (*seen)[0].header.Get("Authorization"), (*seen)[1].header.Get("Authorization"))
	}
	if len(tokens.invalidated) != 1 || tokens.invalidated[0] != "expired" {
		t.Errorf("invalidated = %v", tokens.invalidated)
	}
	if (*seen)[1].query != "beta=true" || (*seen)[1].body["system"] == nil {
		t.Errorf("bản gửi lại phải được viết lại đầy đủ: %v %v", (*seen)[1].query, (*seen)[1].body)
	}
}

func TestSecondUnauthorizedIsReportedInVietnamese(t *testing.T) {
	srv, seen := newAPI(t, "")
	tokens := &fakeTokens{current: "expired", next: "expired"}
	_, err := newModel(t, srv, tokens).Chat(context.Background(), chatRequest(false))
	if err == nil {
		t.Fatal("muốn lỗi khi refresh xong vẫn bị 401")
	}
	if len(*seen) != 2 {
		t.Errorf("chỉ được refresh/gửi lại đúng một lần, số request = %d", len(*seen))
	}
	if !strings.Contains(err.Error(), "đăng nhập lại") {
		t.Errorf("lỗi = %v", err)
	}
}

func TestForbiddenAndRateLimitedMapToVietnamese(t *testing.T) {
	for _, tc := range []struct {
		token string
		code  string
		want  string
	}{
		{"forbidden", CodeForbidden, "chưa được Anthropic cho phép"},
		{"limited", CodeRateLimited, "chạm giới hạn sử dụng"},
	} {
		srv, seen := newAPI(t, "")
		tokens := &fakeTokens{current: tc.token, next: "other"}
		_, err := newModel(t, srv, tokens).Chat(context.Background(), chatRequest(false))
		if err == nil {
			t.Fatalf("%s: muốn lỗi", tc.token)
		}
		var le *litellm.LiteLLMError
		if !errors.As(err, &le) || le.Code != tc.code {
			t.Errorf("%s: code = %v, muốn %s (%v)", tc.token, le, tc.code, err)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: lỗi = %v, muốn chứa %q", tc.token, err, tc.want)
		}
		if len(*seen) != 1 || len(tokens.invalidated) != 0 {
			t.Errorf("%s: 403/429 không được refresh/gửi lại (request=%d, invalidated=%v)", tc.token, len(*seen), tokens.invalidated)
		}
	}
	// 429 kèm Retry-After → gợi ý thời gian chờ.
	srv, _ := newAPI(t, "")
	_, err := newModel(t, srv, &fakeTokens{current: "limited"}).Chat(context.Background(), chatRequest(false))
	if err == nil || !strings.Contains(err.Error(), "2 phút") {
		t.Errorf("429 thiếu gợi ý Retry-After: %v", err)
	}
}

func TestAdoptRequiredVersionRetriesOnce(t *testing.T) {
	auth.Configure(auth.Options{ClaudeCodeVersionEnv: "VIDERSE_CLAUDE_CODE_VERSION"})
	t.Setenv("VIDERSE_CLAUDE_CODE_VERSION", "")
	var calls int
	var userAgents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		calls++
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"claude_code_version_too_old: version 99.0.0 or newer is required"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, messageOK)
	}))
	defer srv.Close()
	if _, err := newModel(t, srv, &fakeTokens{current: "tok"}).Chat(context.Background(), chatRequest(false)); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if calls != 2 || !strings.Contains(userAgents[1], "99.0.0") {
		t.Fatalf("calls=%d UAs=%v, muốn gửi lại với 99.0.0", calls, userAgents)
	}
}

func TestListModelsFallsBackToStaticList(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	models, err := ListModels(context.Background(), srv.URL, &fakeTokens{current: "tok"})
	if err != nil || len(models) == 0 || models[0].Slug != "claude-opus-4-8" {
		t.Fatalf("models=%v err=%v", models, err)
	}
}

func TestListModelsUsesLiveListWithBearer(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"claude-opus-4-8","display_name":"Claude Opus 4.8"},{"id":"claude-opus-4-8"},{"id":"other"}]}`)
	}))
	defer srv.Close()
	models, err := ListModels(context.Background(), srv.URL, &fakeTokens{current: "tok-m"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok-m" || gotPath != "/v1/models" {
		t.Errorf("auth=%q path=%q", gotAuth, gotPath)
	}
	if len(models) != 1 || models[0].DisplayName != "Claude Opus 4.8" {
		t.Errorf("models = %v", models)
	}
}

func TestValidateBaseURL(t *testing.T) {
	for _, ok := range []string{"", "https://api.anthropic.com", "https://eu.api.anthropic.com/"} {
		if err := ValidateBaseURL(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://api.anthropic.com", "https://evil.example.com", "https://anthropic.com.evil.io", "::"} {
		if err := ValidateBaseURL(bad); err == nil {
			t.Errorf("%q phải bị từ chối", bad)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
