package anthropicoauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Mã lỗi gắn vào body thay thế (litellm đọc error.code → LiteLLMError.Code) để tầng trên
// nhận ra nguyên nhân mà không phải đoán theo status.
const (
	CodeUnauthorized = "anthropic_oauth_unauthorized"
	CodeForbidden    = "anthropic_oauth_forbidden"
	CodeRateLimited  = "anthropic_oauth_rate_limited"
)

const (
	unauthorizedMessage = "Anthropic từ chối phiên đăng nhập Claude Pro/Max (token đã bị thu hồi hoặc hết hạn và làm mới không được) — hãy đăng nhập lại tài khoản"
	forbiddenMessage    = "Gói Claude Pro/Max của bạn chưa được Anthropic cho phép gọi API qua đăng nhập OAuth " +
		"(tài khoản bị hạn chế hoặc gói không kèm quyền dùng Claude Code; đăng nhập lại hay làm mới token không đổi được điều này). " +
		"Hãy dùng API key Anthropic với provider \"anthropic\", hoặc kiểm tra gói tại https://claude.ai/settings"
	rateLimitedMessage = "Gói Claude Pro/Max đã chạm giới hạn sử dụng (hạn mức theo phiên/tuần của gói hoặc giới hạn tốc độ, HTTP 429)"
)

// apiError thay body lỗi 401/403/429 bằng thông báo tiếng Việt (giữ nguyên status); chi tiết
// gốc của Anthropic được giữ lại phía sau. Status khác trả nguyên.
func apiError(resp *http.Response, raw []byte) *http.Response {
	var code, message string
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		code, message = CodeUnauthorized, unauthorizedMessage
	case http.StatusForbidden:
		code, message = CodeForbidden, forbiddenMessage
	case http.StatusTooManyRequests:
		code, message = CodeRateLimited, rateLimitedMessage+retryAfterHint(resp.Header)
	default:
		return withBody(resp, raw)
	}
	if detail := upstreamDetail(raw); detail != "" {
		message += " (Anthropic: " + detail + ")"
	}
	out := encodeJSON(map[string]any{"type": "error", "error": map[string]any{"code": code, "message": message}})
	resp = withBody(resp, out)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func withBody(resp *http.Response, body []byte) *http.Response {
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header = resp.Header.Clone()
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return resp
}

// retryAfterHint đổi header Retry-After (giây hoặc HTTP-date) thành gợi ý thời gian chờ.
func retryAfterHint(h http.Header) string {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return ""
	}
	var wait time.Duration
	if secs, err := strconv.ParseFloat(v, 64); err == nil && secs >= 0 {
		wait = time.Duration(secs * float64(time.Second))
	} else if at, err := http.ParseTime(v); err == nil {
		wait = time.Until(at)
	} else {
		return ""
	}
	if wait <= 0 {
		return " — có thể thử lại ngay"
	}
	switch {
	case wait < time.Minute:
		return fmt.Sprintf(" — thử lại sau khoảng %d giây", int(wait.Seconds()+0.5))
	case wait < time.Hour:
		return fmt.Sprintf(" — thử lại sau khoảng %d phút", int(wait.Minutes()+0.5))
	}
	return fmt.Sprintf(" — thử lại sau khoảng %.1f giờ", wait.Hours())
}

// upstreamDetail rút error.message từ body lỗi của Anthropic ({"type":"error","error":{…}}).
func upstreamDetail(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error.Message != "" {
		text = payload.Error.Message
		if payload.Error.Type != "" {
			text = payload.Error.Type + ": " + text
		}
	}
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}
