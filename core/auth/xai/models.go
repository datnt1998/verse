package xai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultModel là model mặc định cho gói SuperGrok / X Premium+.
const DefaultModel = "grok-4.6"

// Model là một model xAI mà tài khoản có thể gọi.
type Model struct {
	Slug        string
	DisplayName string
}

// fallbackModels là danh sách tĩnh dùng khi GET /v1/models không dùng được với token OAuth.
var fallbackModels = []Model{
	{Slug: DefaultModel, DisplayName: "Grok 4.6"},
	{Slug: "grok-4.3", DisplayName: "Grok 4.3"},
	{Slug: "grok-4.20-0309-reasoning", DisplayName: "Grok 4.20 (reasoning)"},
	{Slug: "grok-4.20-0309-non-reasoning", DisplayName: "Grok 4.20 (non-reasoning)"},
}

// FallbackModels trả về bản sao danh sách model tĩnh.
func FallbackModels() []Model { return append([]Model(nil), fallbackModels...) }

// ListModels lấy danh sách model trực tiếp (GET <base>/models) bằng token OAuth; khi
// gọi thất bại hoặc kết quả rỗng thì rơi về danh sách tĩnh. Chỉ lỗi lấy token (chưa
// đăng nhập, refresh hỏng) mới được trả ra, vì khi đó không có gì để hiển thị.
func ListModels(ctx context.Context, baseURL string, tokens TokenSource) ([]Model, error) {
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	if live, err := listLive(ctx, baseURL, tokens, token); err == nil && len(live) > 0 {
		return live, nil
	}
	return FallbackModels(), nil
}

func listLive(ctx context.Context, baseURL string, tokens TokenSource, token string) ([]Model, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	// Dùng cùng transport với inference để được retry 401 và thông báo 403 nhất quán.
	client := &http.Client{Transport: &transport{base: http.DefaultTransport, tokens: staticFirst{TokenSource: tokens, first: token}}}
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

// staticFirst trả token đã lấy cho lần gọi đầu để không đọc kho credential hai lần.
type staticFirst struct {
	TokenSource
	first string
}

func (s staticFirst) Token(context.Context) (string, error) { return s.first, nil }

func parseModels(body []byte) ([]Model, error) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Model
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if !isChatModel(id) || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Model{Slug: id, DisplayName: id})
	}
	return out, nil
}

// isChatModel loại model ảnh/video/embedding mà /v1/models trả lẫn với model chat.
func isChatModel(id string) bool {
	if !strings.HasPrefix(id, "grok-") {
		return false
	}
	for _, skip := range []string{"imagine", "image", "video", "embed", "tts", "stt"} {
		if strings.Contains(id, skip) {
			return false
		}
	}
	return true
}
