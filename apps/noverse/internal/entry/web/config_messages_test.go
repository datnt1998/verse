package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// englishLeaks 是适配器/encoding/json/bootstrap 旧英文文案的特征片段：config 接口回给用户的
// 校验错误一律越南语（合约 11 §约定），出现任一片段即视为漏翻。
var englishLeaks = []string{"is required", "[validation", "[network", "config error", "provider error", "default model", "json:", "unknown field", "must be", "unexpected end", "request failed", "connection refused", "dial tcp", "Incorrect API key"}

func assertVietnamese(t *testing.T, what, msg string) {
	t.Helper()
	for _, leak := range englishLeaks {
		if strings.Contains(msg, leak) {
			t.Fatalf("%s: thông báo lọt tiếng Anh %q: %s", what, leak, msg)
		}
	}
}

// E2EAUTH-2：Onboard/Cài đặt “Thử kết nối” khi thiếu khoá API, cấu hình sai, mất mạng —
// thông báo phải là tiếng Việt ngay từ máy chủ.
func TestConfigTestEndpointMessagesAreVietnamese(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	rejecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer rejecting.Close()

	cases := []struct {
		name, body, want, detail string
	}{
		{
			name: "preset có type nhưng thiếu khoá",
			body: `{"provider":"openrouter","config":{"providers":{"openrouter":{"type":"openrouter","base_url":"https://openrouter.ai/api/v1"}}}}`,
			want: "Chưa có khoá API cho OpenRouter — dán khoá API rồi thử lại",
		},
		{
			name: "proxy tự đặt thiếu khoá (adapter báo)",
			body: `{"provider":"proxy","model":"m","config":{"providers":{"proxy":{"type":"openai","base_url":"http://127.0.0.1:1/v1"}}}}`,
			want: `Chưa có khoá API cho "proxy" — dán khoá API rồi thử lại`,
		},
		{
			name: "api sai",
			body: `{"provider":"openai","config":{"providers":{"openai":{"type":"openai","api_key":"sk-x","api":"weird"}}}}`,
			want: `Cấu hình chưa hợp lệ: provider "openai": api chỉ nhận "chat" hoặc "responses": lỗi cấu hình`,
		},
		{
			name: "stream_idle_timeout sai",
			body: `{"provider":"openai","config":{"providers":{"openai":{"type":"openai","api_key":"sk-x","stream_idle_timeout":"abc"}}}}`,
			want: "Cấu hình chưa hợp lệ: provider \"openai\" stream_idle_timeout: thời lượng \"abc\" không hợp lệ",
		},
		{
			name: "provider lạ không có type",
			body: `{"provider":"mystery","model":"x","config":{"providers":{"mystery":{}}}}`,
			want: `Cấu hình chưa hợp lệ: model mặc định: phân tích loại provider thất bại: provider "mystery" thiếu type`,
		},
		{
			name:   "mất mạng",
			body:   `{"provider":"openai","config":{"providers":{"openai":{"type":"openai","api_key":"sk-x","base_url":"http://127.0.0.1:1/v1"}}}}`,
			want:   "Không kết nối được OpenAI tại http://127.0.0.1:1 — kiểm tra máy chủ đã chạy và địa chỉ đúng",
			detail: "connection refused",
		},
		{
			name:   "Ollama chưa chạy",
			body:   `{"provider":"ollama","config":{"providers":{"ollama":{"type":"ollama","base_url":"http://127.0.0.1:1/v1"}}}}`,
			want:   "Không kết nối được Ollama tại http://127.0.0.1:1 — kiểm tra máy chủ đã chạy và địa chỉ đúng",
			detail: "connection refused",
		},
		{
			name:   "khoá bị từ chối",
			body:   fmt.Sprintf(`{"provider":"openai","config":{"providers":{"openai":{"type":"openai","api_key":"sk-wrong","base_url":%q}}}}`, rejecting.URL),
			want:   "OpenAI từ chối khoá API (Xác thực thất bại, HTTP 401, openai",
			detail: "Incorrect API key provided",
		},
	}
	for _, c := range cases {
		w := tg.do("POST", "/api/config/test", admin, c.body)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", c.name, w.Code, w.Body.String())
		}
		m := decodeResp(t, w)
		msg, _ := m["message"].(string)
		if m["ok"] != false || !strings.HasPrefix(msg, c.want) {
			t.Fatalf("%s: muốn bắt đầu bằng %q, nhận %v", c.name, c.want, m)
		}
		assertVietnamese(t, c.name, msg)
		if detail, _ := m["detail"].(string); !strings.Contains(detail, c.detail) {
			t.Fatalf("%s: detail phải giữ nguyên văn lỗi gốc chứa %q, nhận %v", c.name, c.detail, m)
		}
	}
}

// Bản nháp không mang api_key (GET chỉ trả api_key_set) → thử bằng khoá đã lưu ở cấp toàn cục,
// cùng quy ước “api_key vắng = giữ khoá cũ” của PUT /api/config.
func TestConfigTestEndpointUsesSavedAPIKey(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"fake","choices":[{"message":{"content":"OK"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	tg.globalConfig(t, fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","providers":{"openai":{"type":"openai","api_key":"sk-saved-123456789","base_url":%q}}}`, srv.URL))

	body := fmt.Sprintf(`{"provider":"openai","model":"gpt-4o","config":{"provider":"openai","providers":{"openai":{"type":"openai","base_url":%q}}}}`, srv.URL)
	w := tg.do("POST", "/api/config/test", admin, body)
	if m := decodeResp(t, w); m["ok"] != true {
		t.Fatalf("bản nháp không kèm khoá phải dùng khoá đã lưu: %v", m)
	}
	if auth != "Bearer sk-saved-123456789" {
		t.Fatalf("provider giả nhận Authorization %q", auth)
	}
}

// Lỗi giải mã thân JSON của các endpoint cấu hình (PUT /api/config tự giải mã; /api/config/test qua decodeJSON).
func TestConfigEndpointsJSONErrorsAreVietnamese(t *testing.T) {
	tg := newConfigTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	cases := []struct {
		method, path, body, want string
	}{
		{"PUT", "/api/config?scope=global", `{"bogus":1}`, `Trường không được hỗ trợ: "bogus"`},
		{"PUT", "/api/config?scope=global", `{"provider":5}`, `Trường "provider" sai kiểu: cần chuỗi, nhận số`},
		{"PUT", "/api/config?scope=global", `{"provider":`, "JSON không hợp lệ: dữ liệu bị cắt cụt"},
		{"PUT", "/api/config?scope=global", `{"provider" "x"}`, "JSON không hợp lệ: sai cú pháp ở vị trí 13"},
		{"POST", "/api/config/test", `{"provider":"openai","bogus":1}`, `Trường không được hỗ trợ: "bogus"`},
		{"POST", "/api/config/test", `{"provider":"openai"} {}`, "Chỉ được gửi một đối tượng JSON"},
	}
	for _, c := range cases {
		w := tg.do(c.method, c.path, admin, c.body)
		if w.Code != 400 {
			t.Fatalf("%s %s %s: %d %s", c.method, c.path, c.body, w.Code, w.Body.String())
		}
		got, _ := decodeResp(t, w)["error"].(string)
		if got != c.want {
			t.Fatalf("%s %s: muốn %q, nhận %q", c.method, c.body, c.want, got)
		}
		assertVietnamese(t, c.body, got)
	}
}
