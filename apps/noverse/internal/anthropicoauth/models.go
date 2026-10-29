package anthropicoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultModel là model mặc định cho gói Claude Pro/Max.
const DefaultModel = "claude-sonnet-4-6"

// Model là một model Claude mà tài khoản có thể gọi.
type Model struct {
	Slug        string
	DisplayName string
}

// fallbackModels là danh sách tĩnh dùng khi GET /v1/models không dùng được với token OAuth.
var fallbackModels = []Model{
	{Slug: "claude-opus-4-8", DisplayName: "Claude Opus 4.8"},
	{Slug: DefaultModel, DisplayName: "Claude Sonnet 4.6"},
	{Slug: "claude-haiku-4-5", DisplayName: "Claude Haiku 4.5"},
	{Slug: "claude-opus-4-7", DisplayName: "Claude Opus 4.7"},
	{Slug: "claude-sonnet-4-5", DisplayName: "Claude Sonnet 4.5"},
	{Slug: "claude-opus-4-5", DisplayName: "Claude Opus 4.5"},
}

// FallbackModels trả về bản sao danh sách model tĩnh.
func FallbackModels() []Model { return append([]Model(nil), fallbackModels...) }

// ListModels lấy danh sách model trực tiếp (GET <base>/v1/models) bằng token OAuth; khi gọi
// thất bại hoặc kết quả rỗng thì rơi về danh sách tĩnh. Chỉ lỗi lấy token (chưa đăng nhập,
// refresh hỏng) mới được trả ra, vì khi đó không có gì để hiển thị.
func ListModels(ctx context.Context, baseURL string, tokens TokenSource) ([]Model, error) {
	if _, err := tokens.Credential(ctx); err != nil {
		return nil, err
	}
	if live, err := listLive(ctx, baseURL, tokens); err == nil && len(live) > 0 {
		return live, nil
	}
	return FallbackModels(), nil
}

func listLive(ctx context.Context, baseURL string, tokens TokenSource) ([]Model, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	// Dùng cùng transport với inference để được retry 401 và thông báo lỗi nhất quán.
	client := &http.Client{Transport: newTransport(http.DefaultTransport, tokens, nil)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, io.ErrUnexpectedEOF
	}
	return parseModels(body)
}

func parseModels(body []byte) ([]Model, error) {
	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Model
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if !strings.HasPrefix(id, "claude-") || seen[id] {
			continue
		}
		seen[id] = true
		name := strings.TrimSpace(m.DisplayName)
		if name == "" {
			name = id
		}
		out = append(out, Model{Slug: id, DisplayName: name})
	}
	return out, nil
}
