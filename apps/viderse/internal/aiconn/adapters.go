package aiconn

// Adapter gọi danh mục/model của từng nhà cung cấp. Mọi endpoint đều qua
// *http.Client tiêm vào để test bằng httptest; lỗi phân loại bằng
// *CatalogError (auth / unreachable / failed).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// catalogTimeout là trần một lượt gọi provider trong test/fetch/check.
const catalogTimeout = 30 * time.Second

// bodyCap giới hạn body đọc từ provider (danh mục vài trăm KB là đủ).
const bodyCap = 8 << 20

// do gọi một request JSON và trả status + body đã giới hạn.
func do(client *http.Client, req *http.Request) (int, []byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, &CatalogError{Kind: "unreachable", Message: cleanNetErr(err)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyCap))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// cleanNetErr rút gọn lỗi mạng thành một cụm ngắn không chứa địa chỉ nội bộ.
func cleanNetErr(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "hết thời gian kết nối"
	}
	var de *net.DNSError
	if errors.As(err, &de) {
		return "không phân giải được tên miền"
	}
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Op == "dial" {
		return "không kết nối được"
	}
	return "lỗi kết nối"
}

// httpError dựng CatalogError từ status provider: 401/403 → auth, 5xx →
// unreachable, còn lại → failed. Body chỉ lọt vào Message sau khi lọc bí mật.
func httpError(status int, body []byte) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &CatalogError{Kind: "auth", Status: status,
			Message: fmt.Sprintf("provider từ chối xác thực (%d)", status)}
	case status >= 500:
		return &CatalogError{Kind: "unreachable", Status: status,
			Message: fmt.Sprintf("provider lỗi %d", status)}
	default:
		return &CatalogError{Kind: "failed", Status: status,
			Message: fmt.Sprintf("provider trả %d: %s", status, sanitize(body))}
	}
}

// CredentialClientOptions cấu hình client gọi provider mang credential.
type CredentialClientOptions struct {
	Timeout time.Duration
	// Hosted: máy chủ đang chia sẻ cho người khác (PUBLIC_URL). Mỗi lần nối
	// phải kiểm địa chỉ THẬT đang nối — kiểm lúc lưu baseUrl là chưa đủ vì
	// DNS có thể đổi giữa hai lần phân giải (DNS rebinding).
	Hosted bool
	// LookupIP là resolver dùng cho kiểm tra lúc nối; nil → resolver hệ thống.
	LookupIP LookupIPFunc
}

// NewCredentialClient trả client HTTP cho mọi lời gọi mang credential: chặn
// redirect cross-origin và hạ cấp https→http, nên khoá/tài khoản không bị
// chuyển tiếp sang máy khác qua redirect của provider (SSRF phần redirect).
// Ở chế độ hosted, transport chỉ nối tới địa chỉ công khai (cùng quy tắc
// internalAddr với lúc lưu baseUrl), nên redirect, keep-alive nối lại hay
// DNS đổi về loopback đều bị chặn ở tầng dial.
func NewCredentialClient(o CredentialClientOptions) *http.Client {
	client := &http.Client{Timeout: o.Timeout, CheckRedirect: SameOriginRedirect}
	if o.Hosted {
		client.Transport = publicOnlyTransport(o.LookupIP)
	}
	return client
}

// SameOriginRedirect chỉ theo redirect cùng scheme+host, tối đa 5 lần.
// Cross-origin bị từ chối (mặc định của Go vẫn forward Authorization header)
// và https không bao giờ bị hạ xuống http.
func SameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("quá nhiều lần chuyển hướng")
	}
	if len(via) == 0 {
		return nil
	}
	first := via[0]
	if !strings.EqualFold(req.URL.Scheme, first.URL.Scheme) ||
		!strings.EqualFold(req.URL.Host, first.URL.Host) {
		return errors.New("từ chối chuyển hướng khác máy chủ của request mang credential")
	}
	if first.URL.Scheme == "https" && req.URL.Scheme != "https" {
		return errors.New("từ chối chuyển hướng https xuống http")
	}
	return nil
}

// Các mẫu che bí mật có thể lọt từ body provider vào thông điệp lỗi. Chạy
// TUẦN TỰ trên chuỗi đã cắt trần: mỗi lượt thay bằng "***" nên không bao giờ
// tạo khớp mới — bản cũ giữ tiền tố "Bearer "/"sk-" nên lặp vô hạn.
var secretPatterns = []*regexp.Regexp{
	// Bearer/Authorization: Bearer <token>
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]{3,}`),
	// Khoá dạng sk-…
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9._-]{3,}`),
	// Cặp tên-trường thường chứa khoá: x-api-key: …, api_key=…, access_token": …
	regexp.MustCompile(`(?i)(x-api-key|api[_-]?key|access_token|refresh_token)["']?\s*[:=]\s*["']?[A-Za-z0-9._~+/=-]{6,}`),
}

// sanitize che chuỗi dạng token/khoá trong văn bản provider trước khi nó lọt
// vào lỗi hiển thị. Có trần độ dài và LUÔN dừng.
func sanitize(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, "***")
	}
	return s
}

// jsonReq dựng request JSON với header auth tuỳ chọn.
func jsonReq(ctx context.Context, method, url string, headers map[string]string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// ── OpenAI-compatible (openai, openrouter, deepseek, xai, openai_compat) ────

// OpenAICompatAdapter nói giao thức OpenAI: GET /models, POST
// /chat/completions với Bearer key.
type OpenAICompatAdapter struct {
	Client *http.Client
}

func (a *OpenAICompatAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

// FetchCatalog lấy GET <base>/models.
func (a *OpenAICompatAdapter) FetchCatalog(ctx context.Context, p domain.Provider, apiKey string) ([]domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	req, err := jsonReq(ctx, http.MethodGet, p.BaseURL+"/models",
		map[string]string{"Authorization": "Bearer " + apiKey}, nil)
	if err != nil {
		return nil, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, httpError(status, body)
	}
	var list struct {
		Data []struct {
			ID            string `json:"id"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
				Image      string `json:"image"`
			} `json:"pricing"`
			Architecture struct {
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, &CatalogError{Kind: "failed", Message: "danh sách model không đọc được"}
	}
	out := make([]domain.AIModel, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		am := domain.AIModel{ID: m.ID, ContextK: m.ContextLength / 1000}
		if len(m.Architecture.InputModalities) > 0 {
			am.Caps = capsFromModalities(m.Architecture.InputModalities, m.Architecture.OutputModalities)
		}
		am.InPerM = parsePricePerM(m.Pricing.Prompt)
		am.OutPerM = parsePricePerM(m.Pricing.Completion)
		am.PerImage = parsePrice(m.Pricing.Image)
		out = append(out, am)
	}
	return out, nil
}

// CheckModel gọi một lượt chat tối thiểu (max_tokens=1) — lượt ngắn duy nhất
// kiểm chứng được cả khoá lẫn khả năng gọi của model.
func (a *OpenAICompatAdapter) CheckModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	payload := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
	}
	req, err := jsonReq(ctx, http.MethodPost, p.BaseURL+"/chat/completions",
		map[string]string{"Authorization": "Bearer " + apiKey}, payload)
	if err != nil {
		return domain.AIModel{}, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return domain.AIModel{}, err
	}
	if status != http.StatusOK {
		return domain.AIModel{}, httpError(status, body)
	}
	return domain.AIModel{ID: model}, nil
}

// capsFromModalities dịch modalities của OpenRouter sang caps §15.
func capsFromModalities(in, out []string) []domain.Capability {
	var caps []domain.Capability
	text := false
	for _, m := range out {
		if m == "text" {
			text = true
		}
	}
	if text || len(out) == 0 {
		caps = append(caps, domain.CapText)
	}
	for _, m := range in {
		if m == "image" && text {
			caps = append(caps, domain.CapVision)
		}
	}
	for _, m := range out {
		if m == "image" {
			caps = append(caps, domain.CapImage)
		}
	}
	return caps
}

// parsePricePerM đọc giá theo token của OpenRouter (USD/token → USD/1M).
func parsePricePerM(s string) float64 {
	v := parsePrice(s)
	if v == 0 {
		return 0
	}
	return v * 1_000_000
}

// parsePrice đọc một trường giá dạng chuỗi.
func parsePrice(s string) float64 {
	var v float64
	if _, err := fmt.Sscanf(s, "%g", &v); err != nil {
		return 0
	}
	return v
}

// ── Gemini ────────────────────────────────────────────────────────────────

// GeminiAdapter nói API generativelanguage: GET /v1beta/models?key=…
type GeminiAdapter struct {
	Client *http.Client
}

func (a *GeminiAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a *GeminiAdapter) FetchCatalog(ctx context.Context, p domain.Provider, apiKey string) ([]domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")
	url := base + "/v1beta/models?pageSize=1000"
	req, err := jsonReq(ctx, http.MethodGet, url, map[string]string{"x-goog-api-key": apiKey}, nil)
	if err != nil {
		return nil, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, httpError(status, body)
	}
	var list struct {
		Models []struct {
			Name             string   `json:"name"`
			SupportedActions []string `json:"supportedGenerationMethods"`
			InputTokenLimit  int      `json:"inputTokenLimit"`
			OutputTokenLimit int      `json:"outputTokenLimit"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, &CatalogError{Kind: "failed", Message: "danh sách model không đọc được"}
	}
	out := make([]domain.AIModel, 0, len(list.Models))
	for _, m := range list.Models {
		id := strings.TrimPrefix(m.Name, "models/")
		if id == "" {
			continue
		}
		// Chỉ giữ model sinh nội dung; embedding/AQA bỏ qua khỏi danh mục.
		gen := false
		for _, act := range m.SupportedActions {
			if act == "generateContent" {
				gen = true
			}
		}
		if !gen {
			continue
		}
		out = append(out, domain.AIModel{ID: id, ContextK: m.InputTokenLimit / 1000})
	}
	return out, nil
}

// CheckModel: GET /v1beta/models/<id> (miễn phí) — xác nhận model tồn tại và
// khoá đọc được; caps suy từ tên + metadata.
func (a *GeminiAdapter) CheckModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")
	req, err := jsonReq(ctx, http.MethodGet, base+"/v1beta/models/"+model,
		map[string]string{"x-goog-api-key": apiKey}, nil)
	if err != nil {
		return domain.AIModel{}, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return domain.AIModel{}, err
	}
	if status != http.StatusOK {
		return domain.AIModel{}, httpError(status, body)
	}
	var m struct {
		InputTokenLimit int `json:"inputTokenLimit"`
	}
	_ = json.Unmarshal(body, &m)
	return domain.AIModel{ID: model, ContextK: m.InputTokenLimit / 1000}, nil
}

// ── Anthropic ─────────────────────────────────────────────────────────────

// AnthropicAdapter nói /v1/models với x-api-key + anthropic-version.
type AnthropicAdapter struct {
	Client *http.Client
}

func (a *AnthropicAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

const anthropicVersion = "2023-06-01"

func (a *AnthropicAdapter) headers(apiKey string) map[string]string {
	return map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": anthropicVersion,
	}
}

func (a *AnthropicAdapter) FetchCatalog(ctx context.Context, p domain.Provider, apiKey string) ([]domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")
	req, err := jsonReq(ctx, http.MethodGet, base+"/v1/models?limit=1000", a.headers(apiKey), nil)
	if err != nil {
		return nil, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, httpError(status, body)
	}
	var list struct {
		Data []struct {
			ID        string `json:"id"`
			MaxTokens int    `json:"max_tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, &CatalogError{Kind: "failed", Message: "danh sách model không đọc được"}
	}
	out := make([]domain.AIModel, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		// Claude đều đọc được ảnh.
		out = append(out, domain.AIModel{ID: m.ID, Caps: []domain.Capability{domain.CapText, domain.CapVision}})
	}
	return out, nil
}

// CheckModel gọi /v1/messages với max_tokens=1 — lượt ngắn tối thiểu.
func (a *AnthropicAdapter) CheckModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")
	payload := map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	req, err := jsonReq(ctx, http.MethodPost, base+"/v1/messages", a.headers(apiKey), payload)
	if err != nil {
		return domain.AIModel{}, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return domain.AIModel{}, err
	}
	if status != http.StatusOK {
		return domain.AIModel{}, httpError(status, body)
	}
	return domain.AIModel{ID: model, Caps: []domain.Capability{domain.CapText, domain.CapVision}}, nil
}

// ── Ollama ────────────────────────────────────────────────────────────────

// OllamaAdapter nói API local: GET /api/tags, POST /api/show và
// /api/generate — hoàn toàn miễn phí, không khoá.
type OllamaAdapter struct {
	Client *http.Client
}

func (a *OllamaAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a *OllamaAdapter) FetchCatalog(ctx context.Context, p domain.Provider, _ string) ([]domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")
	req, err := jsonReq(ctx, http.MethodGet, base+"/api/tags", nil, nil)
	if err != nil {
		return nil, err
	}
	status, body, err := do(a.client(), req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, httpError(status, body)
	}
	var list struct {
		Models []struct {
			Name    string `json:"name"`
			Details struct {
				Families []string `json:"families"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, &CatalogError{Kind: "failed", Message: "danh sách model không đọc được"}
	}
	out := make([]domain.AIModel, 0, len(list.Models))
	for _, m := range list.Models {
		if m.Name == "" {
			continue
		}
		// /api/show là nguồn DUY NHẤT đáng tin cho capability: model chỉ
		// embedding (bge-m3…) không có "completion" và phải là caps rỗng —
		// không được cấp "text". Chỉ chạy lúc lấy danh
		// mục (add/refresh), không bao giờ trên GET.
		caps, ctxK, ok := a.showCaps(ctx, base, m.Name)
		if !ok {
			// /api/show lỗi/không trả capabilities (bản Ollama cũ): giữ
			// heuristic theo families và để models/check xác nhận sau.
			caps = heuristicOllamaCaps(m.Details.Families)
			ctxK = 0
		}
		out = append(out, domain.AIModel{ID: m.Name, Caps: caps, ContextK: ctxK})
	}
	return out, nil
}

// heuristicOllamaCaps là suy đoán thận trọng khi /api/show không trả
// capabilities: chỉ nhận vision theo kiến trúc đa phương thức quen thuộc.
func heuristicOllamaCaps(families []string) []domain.Capability {
	caps := []domain.Capability{domain.CapText}
	for _, f := range families {
		switch strings.ToLower(f) {
		case "mllama", "clip", "llava", "gemma3", "qwen3vl", "minicpm-v":
			caps = append(caps, domain.CapVision)
		}
	}
	return caps
}

// showCaps đọc capabilities + cửa sổ ngữ cảnh từ /api/show (miễn phí, local).
// ok=false khi endpoint lỗi hoặc bản Ollama không khai capabilities.
func (a *OllamaAdapter) showCaps(ctx context.Context, base, model string) ([]domain.Capability, int, bool) {
	req, err := jsonReq(ctx, http.MethodPost, base+"/api/show", nil, map[string]string{"model": model})
	if err != nil {
		return nil, 0, false
	}
	status, body, err := do(a.client(), req)
	if err != nil || status != http.StatusOK {
		return nil, 0, false
	}
	var show struct {
		Capabilities []string       `json:"capabilities"`
		ModelInfo    map[string]any `json:"model_info"`
	}
	if err := json.Unmarshal(body, &show); err != nil || show.Capabilities == nil {
		return nil, 0, false
	}
	caps := []domain.Capability{}
	for _, c := range show.Capabilities {
		switch c {
		case "completion":
			caps = append(caps, domain.CapText)
		case "vision":
			caps = append(caps, domain.CapVision)
		}
	}
	return caps, ollamaContextK(show.ModelInfo), true
}

// CheckModel: /api/show (metadata miễn phí — có capabilities + context) rồi
// một lượt generate tối thiểu (num_predict=1, stream tắt) để chứng minh model
// nạp được — vẫn hoàn toàn local và miễn phí.
func (a *OllamaAdapter) CheckModel(ctx context.Context, p domain.Provider, apiKey, model string) (domain.AIModel, error) {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	base := strings.TrimRight(p.BaseURL, "/")

	showReq, err := jsonReq(ctx, http.MethodPost, base+"/api/show", nil, map[string]string{"model": model})
	if err != nil {
		return domain.AIModel{}, err
	}
	status, body, err := do(a.client(), showReq)
	if err != nil {
		return domain.AIModel{}, err
	}
	if status != http.StatusOK {
		if status == http.StatusNotFound {
			return domain.AIModel{}, &CatalogError{Kind: "failed", Status: status,
				Message: "ollama chưa có model " + model}
		}
		return domain.AIModel{}, httpError(status, body)
	}
	var show struct {
		Capabilities []string       `json:"capabilities"`
		ModelInfo    map[string]any `json:"model_info"`
	}
	if err := json.Unmarshal(body, &show); err != nil {
		return domain.AIModel{}, &CatalogError{Kind: "failed", Message: "thông tin model không đọc được"}
	}
	// Capability theo /api/show: model chỉ-embedding không có "completion" thì
	// không thể sinh chữ và không cần lượt generate.
	completion := false
	caps := []domain.Capability{}
	for _, c := range show.Capabilities {
		switch c {
		case "completion":
			completion = true
			caps = append(caps, domain.CapText)
		case "vision":
			caps = append(caps, domain.CapVision)
		}
	}
	if len(show.Capabilities) == 0 {
		// Bản Ollama cũ không trả capabilities — giữ hành vi trước đây.
		completion = true
		caps = []domain.Capability{domain.CapText}
	}
	ctxK := ollamaContextK(show.ModelInfo)
	if !completion {
		return domain.AIModel{ID: model, Caps: caps, ContextK: ctxK}, nil
	}

	genReq, err := jsonReq(ctx, http.MethodPost, base+"/api/generate", nil, map[string]any{
		"model": model, "prompt": "hi", "stream": false,
		"options": map[string]any{"num_predict": 1},
	})
	if err != nil {
		return domain.AIModel{}, err
	}
	status, body, err = do(a.client(), genReq)
	if err != nil {
		return domain.AIModel{}, err
	}
	if status != http.StatusOK {
		return domain.AIModel{}, httpError(status, body)
	}
	return domain.AIModel{ID: model, Caps: caps, ContextK: ctxK}, nil
}

// ollamaContextK rút cửa sổ ngữ cảnh từ model_info của /api/show (khoá dạng
// "<arch>.context_length").
func ollamaContextK(info map[string]any) int {
	for k, v := range info {
		if !strings.HasSuffix(k, ".context_length") {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n) / 1000
		}
	}
	return 0
}

// ── ChatGPT (Codex CLI) ───────────────────────────────────────────────────

// CodexCatalog là nguồn danh mục model của gói ChatGPT — chạy `codex debug
// models --bundled` qua tools.Codex ở composition root (aiconn không import
// internal/tools).
type CodexCatalog func(ctx context.Context) ([]domain.AIModel, error)

// ChatGPTAdapter lấy danh mục qua Codex CLI + một model ảnh cố định của app.
type ChatGPTAdapter struct {
	// Catalog chạy `debug models --bundled` trên binary đã chọn; bắt buộc.
	Catalog CodexCatalog
}

func (a *ChatGPTAdapter) FetchCatalog(ctx context.Context, p domain.Provider, _ string) ([]domain.AIModel, error) {
	if a.Catalog == nil {
		return nil, &CatalogError{Kind: "failed", Message: "chưa cấu hình danh mục Codex"}
	}
	models, err := a.Catalog(ctx)
	if err != nil {
		return nil, &CatalogError{Kind: "unreachable", Message: "không đọc được danh mục Codex"}
	}
	// Model vẽ của app không nằm trong catalog reasoning.
	models = append(models, domain.AIModel{
		ID: "gpt-image-2", Caps: []domain.Capability{domain.CapImage}, Recommended: true,
	})
	return models, nil
}

// CheckModel với gói ChatGPT: model phải có trong danh mục đã lấy — lượt gọi
// thật sẽ tốn hạn mức của gói nên "gọi thử ngắn" ở đây là đối chiếu danh mục.
func (a *ChatGPTAdapter) CheckModel(ctx context.Context, p domain.Provider, _ string, model string) (domain.AIModel, error) {
	models, err := a.FetchCatalog(ctx, p, "")
	if err != nil {
		return domain.AIModel{}, err
	}
	for _, m := range models {
		if m.ID == model {
			return m, nil
		}
	}
	return domain.AIModel{}, &CatalogError{Kind: "failed", Message: "không có model " + model + " trong danh mục ChatGPT"}
}

// ── Grok (xAI OAuth) ──────────────────────────────────────────────────────

// GrokModels là nguồn danh mục text của gói Grok — xai.ListModels trên
// credential OAuth của app (composition root cung cấp).
type GrokModels func(ctx context.Context) ([]domain.AIModel, error)

// GrokAdapter lấy model text qua API xAI OAuth + model ảnh cố định của app.
type GrokAdapter struct {
	Models GrokModels
}

func (a *GrokAdapter) FetchCatalog(ctx context.Context, p domain.Provider, _ string) ([]domain.AIModel, error) {
	var models []domain.AIModel
	if a.Models != nil {
		var err error
		models, err = a.Models(ctx)
		if err != nil {
			return nil, &CatalogError{Kind: "unreachable", Message: "không đọc được danh mục Grok"}
		}
	}
	models = append(models, domain.AIModel{
		ID: "grok-imagine-image", Caps: []domain.Capability{domain.CapImage}, Recommended: true,
	})
	return models, nil
}

// CheckModel: đối chiếu danh mục (lượt gọi thật tốn hạn mức gói).
func (a *GrokAdapter) CheckModel(ctx context.Context, p domain.Provider, _ string, model string) (domain.AIModel, error) {
	models, err := a.FetchCatalog(ctx, p, "")
	if err != nil {
		return domain.AIModel{}, err
	}
	for _, m := range models {
		if m.ID == model {
			return m, nil
		}
	}
	return domain.AIModel{}, &CatalogError{Kind: "failed", Message: "không có model " + model + " trong danh mục Grok"}
}
