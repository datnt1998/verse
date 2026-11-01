package llm_test

// WithTransport: lời gọi provider phải đi qua transport do caller cấp — chỗ
// buộc chính sách địa chỉ ở tầng nối cho baseUrl do người dùng nhập.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/datnt1998/verse/core/llm"
)

// recordingTransport ghi lại request và trả một chat completion tối thiểu.
type recordingTransport struct {
	mu   sync.Mutex
	urls []string
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.urls = append(t.urls, req.URL.String())
	t.mu.Unlock()
	const body = `{"id":"c1","object":"chat.completion","created":1,"model":"m1",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (t *recordingTransport) calls() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.urls...)
}

func TestNewModelUsesSuppliedTransport(t *testing.T) {
	rt := &recordingTransport{}
	model, err := llm.NewModel("openai", "m1",
		llm.WithAPIKey("sk-test"),
		llm.WithBaseURL("http://provider.invalid"),
		llm.WithTransport(rt),
	)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	resp, err := model.Generate(context.Background(),
		[]llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.ContentText, Text: "hi"}}}}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp == nil || resp.Message.TextContent() != "ok" {
		t.Fatalf("response = %+v", resp)
	}
	calls := rt.calls()
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "http://provider.invalid/") {
		t.Fatalf("request không đi qua transport được cấp: %v", calls)
	}
}
