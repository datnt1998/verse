package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/core/auth"
)

// Model là một model mà tài khoản ChatGPT hiện tại được dùng qua Codex.
type Model struct {
	Slug          string
	DisplayName   string
	ContextWindow int
	priority      int
}

// ListModels truy vấn danh sách model khả dụng của tài khoản (GET <base>/models),
// bỏ model ẩn, sắp theo thứ tự ưu tiên của backend. Danh sách phụ thuộc gói và
// header phiên bản client nên không thể hard-code.
func ListModels(ctx context.Context, baseURL string, tokens TokenSource) ([]Model, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	endpoint := baseURL + "/models?" + url.Values{"client_version": {clientVersion}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.Access)
	if token.AccountID != "" {
		req.Header.Set("chatgpt-account-id", token.AccountID)
	}
	req.Header.Set("originator", auth.CodexOriginator)
	req.Header.Set("version", clientVersion)
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codex: lấy danh sách model: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex: lấy danh sách model: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return parseModels(body)
}

func parseModels(body []byte) ([]Model, error) {
	var payload struct {
		Models []struct {
			Slug          string   `json:"slug"`
			DisplayName   string   `json:"display_name"`
			ContextWindow int      `json:"context_window"`
			Visibility    string   `json:"visibility"`
			Priority      *float64 `json:"priority"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("codex: giải mã danh sách model: %w", err)
	}
	out := make([]Model, 0, len(payload.Models))
	for _, m := range payload.Models {
		slug := strings.TrimSpace(m.Slug)
		visibility := strings.ToLower(m.Visibility)
		if slug == "" || visibility == "hide" || visibility == "hidden" {
			continue
		}
		priority := int(^uint(0) >> 1)
		if m.Priority != nil {
			priority = int(*m.Priority)
		}
		out = append(out, Model{Slug: slug, DisplayName: m.DisplayName, ContextWindow: m.ContextWindow, priority: priority})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].priority < out[j].priority })
	return out, nil
}
