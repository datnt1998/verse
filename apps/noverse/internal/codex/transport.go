package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/auth"
)

// defaultInstructions: backend Codex yêu cầu trường instructions; request không
// có system prompt (vd. thử kết nối) dùng chuỗi trung tính này.
const defaultInstructions = "You are a helpful assistant."

// unsupportedBodyKeys là các trường backend Codex trả 400 "Unsupported parameter"
// (đồng bộ với transformRequestBody của omp, #3117).
var unsupportedBodyKeys = []string{
	"max_output_tokens", "max_completion_tokens",
	"temperature", "top_p", "top_k", "min_p",
	"presence_penalty", "frequency_penalty", "repetition_penalty", "stop",
	"prompt_cache_retention",
}

type transport struct {
	base      http.RoundTripper
	tokens    TokenSource
	headers   map[string]string
	extraBody map[string]any
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("codex: đọc body request: %w", err)
		}
		body = data
	}
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/responses") {
		rewritten, err := t.rewriteBody(body)
		if err != nil {
			return nil, err
		}
		body = rewritten
	}

	ctx := req.Context()
	token, err := t.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := t.send(req, body, token)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	// 401: token bị thu hồi/hết hạn sớm — refresh một lần rồi gửi lại.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	token, err = t.tokens.Invalidate(ctx, token.Access)
	if err != nil {
		return nil, err
	}
	return t.send(req, body, token)
}

func (t *transport) send(orig *http.Request, body []byte, token Token) (*http.Response, error) {
	req := orig.Clone(orig.Context())
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }

	h := req.Header
	for name, value := range t.headers {
		if strings.TrimSpace(value) != "" {
			h.Set(name, value)
		}
	}
	h.Set("Authorization", "Bearer "+token.Access)
	h.Del("OpenAI-Organization")
	h.Del("OpenAI-Project")
	if token.AccountID != "" {
		h.Set("chatgpt-account-id", token.AccountID)
	}
	h.Set("OpenAI-Beta", "responses=experimental")
	h.Set("originator", auth.CodexOriginator)
	h.Set("version", clientVersion)
	if session := sessionFromContext(orig.Context()); session != "" {
		h.Set("session_id", session)
		h.Set("conversation_id", session)
	}
	if req.Method == http.MethodPost {
		h.Set("Accept", "text/event-stream")
		h.Set("Content-Type", "application/json")
	}
	return t.base.RoundTrip(req)
}

// rewriteBody áp phần hợp đồng Codex chỉ biểu diễn được trên JSON wire.
func (t *transport) rewriteBody(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var body map[string]any
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("codex: body request không phải JSON object: %w", err)
	}

	// extra_body trộn trước để hợp đồng Codex luôn thắng: extra_body dùng chung
	// kiểu {"temperature": 0.8} của provider khác không được làm hỏng request.
	for key, value := range t.extraBody {
		body[key] = value
	}
	body["store"] = false
	body["stream"] = true
	for _, key := range unsupportedBodyKeys {
		delete(body, key)
	}
	if s, _ := body["instructions"].(string); strings.TrimSpace(s) == "" {
		body["instructions"] = defaultInstructions
	}
	body["input"] = normalizeInput(body["input"])
	body["include"] = withEncryptedReasoning(body["include"])
	return json.Marshal(body)
}

// normalizeInput: backend Codex chỉ nhận input dạng danh sách item. Với
// store=false không có item nào được lưu phía server nên mọi tham chiếu theo id
// (item_reference, trường "id" của item phát lại) đều phải bỏ, nếu không backend
// trả 404 "Item not found".
func normalizeInput(input any) any {
	switch v := input.(type) {
	case string:
		return []any{map[string]any{
			"type":    "message",
			"role":    "user",
			"content": []any{map[string]any{"type": "input_text", "text": v}},
		}}
	case []any:
		out := make([]any, 0, len(v))
		for _, raw := range v {
			item, ok := raw.(map[string]any)
			if !ok {
				out = append(out, raw)
				continue
			}
			if item["type"] == "item_reference" {
				continue
			}
			if item["type"] != "computer_call" {
				delete(item, "id")
			}
			if callID, ok := item["call_id"].(string); ok {
				item["call_id"] = sanitizeCallID(callID)
			}
			out = append(out, item)
		}
		return out
	case nil:
		return []any{}
	default:
		return input
	}
}

// withEncryptedReasoning bật reasoning.encrypted_content: với store=false đây là
// cách duy nhất để phát lại reasoning item ở lượt sau.
func withEncryptedReasoning(include any) []any {
	const encrypted = "reasoning.encrypted_content"
	out := []any{}
	if list, ok := include.([]any); ok {
		for _, entry := range list {
			if entry == encrypted {
				continue
			}
			out = append(out, entry)
		}
	}
	return append(out, encrypted)
}

var invalidCallIDChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeCallID đưa call_id về ≤64 ký tự [a-zA-Z0-9_-] như Responses yêu cầu.
// Ánh xạ tất định nên function_call và function_call_output của cùng một lời
// gọi luôn khớp nhau (lịch sử có thể đến từ provider khác sau khi đổi model).
func sanitizeCallID(id string) string {
	clean := invalidCallIDChars.ReplaceAllString(id, "_")
	if clean == id && id != "" && len(id) <= 64 {
		return id
	}
	sum := sha256.Sum256([]byte(id))
	hash := hex.EncodeToString(sum[:])[:16]
	prefix := strings.TrimRight(clean, "_")
	if prefix == "" {
		prefix = "call"
	}
	if limit := 64 - len(hash) - 1; len(prefix) > limit {
		prefix = prefix[:limit]
	}
	return prefix + "_" + hash
}
