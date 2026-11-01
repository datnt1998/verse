package imagegen

// Đường vẽ cho các nhà cung cấp khoá API mà màn Kết nối AI chấp nhận (V17):
// OpenAI Images (openai, xai, openai_compat), OpenRouter (chat completions
// trả ảnh) và Gemini (generateContent / predict). Cùng một lớp Provider của
// imagegen: journal ghi trước, Draw là điểm tiêu tiền duy nhất, mọi body/lỗi
// của provider đi qua sanitize trước khi lên thông điệp.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// APIStyle chọn giao thức vẽ của một provider.
type APIStyle int

const (
	// StyleOpenAIImages: POST <base>/images/generations, b64_json.
	StyleOpenAIImages APIStyle = iota
	// StyleOpenRouter: POST <base>/chat/completions trả message.images[].
	StyleOpenRouter
	// StyleGemini: POST <base>/v1beta/models/<m>:generateContent (hoặc
	// :predict với model Imagen).
	StyleGemini
)

// APIDeps là phụ thuộc của một provider vẽ HTTP.
type APIDeps struct {
	// Name là tên provider trong biên nhận/trần ngày — id thẻ Kết nối AI.
	Name string
	// Label là tên hiển thị của thẻ Kết nối AI (kindTable); rỗng → hiện Name.
	Label string
	// Model là id model được chọn (gpt-image-1.5, grok-2-image, …).
	Model string
	Style APIStyle
	// Client cho mọi HTTP; nil → http.DefaultClient.
	Client *http.Client
	// BaseURL gốc API (không có "/" cuối).
	BaseURL string
	// APIKey trả khoá đã niêm phong được mở tại lúc gọi (nil → Ready lỗi).
	APIKey func(ctx context.Context) (string, error)
	// Paid báo provider tính phí theo ảnh (mặc định true).
	Paid bool
	// ResolveFfmpeg/Runner cho payload không phải PNG (Imagen trả JPEG…).
	ResolveFfmpeg func(ctx context.Context) (string, error)
	Runner        tools.Runner
	// Timeout mặc định của một lần vẽ; 0 → 3 phút.
	Timeout time.Duration
}

// APIProvider vẽ ảnh qua một API HTTP của nhà cung cấp khoá.
type APIProvider struct {
	d APIDeps
}

// NewAPIProvider dựng provider vẽ HTTP.
func NewAPIProvider(d APIDeps) *APIProvider {
	if d.Client == nil {
		d.Client = http.DefaultClient
	}
	if d.Timeout <= 0 {
		d.Timeout = GrokImageTimeout
	}
	return &APIProvider{d: d}
}

func (p *APIProvider) Name() string  { return p.d.Name }
func (p *APIProvider) Label() string { return p.d.Label }
func (p *APIProvider) Model() string { return p.d.Model }
func (p *APIProvider) Paid() bool    { return p.d.Paid }

// Ready: khoá API đọc được. Không gọi mạng (không tốn tiền).
func (p *APIProvider) Ready(ctx context.Context) error {
	if p.d.APIKey == nil {
		return p.missingKey()
	}
	key, err := p.d.APIKey(ctx)
	if err != nil || strings.TrimSpace(key) == "" {
		return p.missingKey()
	}
	return nil
}

// missingKey là lỗi chưa có khoá — miễn phí (chưa gọi provider).
func (p *APIProvider) missingKey() error {
	return MarkUnpaid(&DrawError{Code: ErrCodeToolMissing,
		Message:  "Chưa có khoá API cho " + p.d.Name,
		Recovery: "Mở Cài đặt → Kết nối AI và kiểm tra khoá API của nhà cung cấp"})
}

// Draw thực hiện MỘT lần vẽ trả phí.
func (p *APIProvider) Draw(ctx context.Context, req Request, journal JournalHandle) (Receipt, error) {
	if err := p.Ready(ctx); err != nil {
		return Receipt{}, err
	}
	key, err := p.d.APIKey(ctx)
	if err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = p.d.Timeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if req.OnPhase != nil {
		req.OnPhase("drawing")
	}
	var raw []byte
	var requestID string
	switch p.d.Style {
	case StyleOpenRouter:
		raw, requestID, err = p.drawOpenRouter(callCtx, key, req)
	case StyleGemini:
		raw, requestID, err = p.drawGemini(callCtx, key, req)
	default:
		raw, requestID, err = p.drawOpenAI(callCtx, key, req)
	}
	if err != nil {
		return Receipt{}, err
	}
	if ctx.Err() != nil {
		return Receipt{}, cancelledErr()
	}
	if err := p.saveImage(ctx, req.Output, raw); err != nil {
		return Receipt{}, err
	}
	sum, _ := sha256File(req.Output)
	receipt := Receipt{
		Provider:  p.d.Name,
		Model:     p.d.Model,
		RequestID: requestID,
		PngSha256: sum,
	}
	if err := writeJSONAtomic(req.Output+".receipt.json", receipt); err != nil {
		return Receipt{}, &StoreError{Err: err}
	}
	return receipt, nil
}

// Reconcile: API HTTP không để lại phiên trên đĩa — luật chung là đủ.
func (p *APIProvider) Reconcile(ctx context.Context, scratch json.RawMessage, req Request) (Receipt, bool, error) {
	return Receipt{}, false, nil
}

// drawOpenAI gọi Images API kiểu OpenAI và trả ảnh base64.
func (p *APIProvider) drawOpenAI(ctx context.Context, key string, req Request) ([]byte, string, error) {
	body := map[string]any{
		"model": p.d.Model, "prompt": req.Prompt, "n": 1,
		"size": openAISize(req.Width, req.Height), "response_format": "b64_json",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", MarkUnpaid(err)
	}
	status, raw, header, err := p.post(ctx, strings.TrimRight(p.d.BaseURL, "/")+"/images/generations", key, payload)
	if err != nil {
		return nil, "", err
	}
	if status != http.StatusOK {
		return nil, "", classifyAPIError(status, raw)
	}
	var parsed struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Data) == 0 || parsed.Data[0].B64 == "" {
		return nil, "", p.invalidImage()
	}
	img, err := decodeB64Image(parsed.Data[0].B64)
	if err != nil {
		return nil, "", p.invalidImage()
	}
	return img, header.Get("x-request-id"), nil
}

// drawOpenRouter gọi chat completions với modalities image và đọc
// choices[0].message.images[].image_url.url (data URI).
func (p *APIProvider) drawOpenRouter(ctx context.Context, key string, req Request) ([]byte, string, error) {
	body := map[string]any{
		"model":      p.d.Model,
		"modalities": []string{"image", "text"},
		"messages": []map[string]any{{
			"role":    "user",
			"content": req.Prompt,
		}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", MarkUnpaid(err)
	}
	status, raw, header, err := p.post(ctx, strings.TrimRight(p.d.BaseURL, "/")+"/chat/completions", key, payload)
	if err != nil {
		return nil, "", err
	}
	if status != http.StatusOK {
		return nil, "", classifyAPIError(status, raw)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Images []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 ||
		len(parsed.Choices[0].Message.Images) == 0 {
		return nil, "", p.invalidImage()
	}
	uri := parsed.Choices[0].Message.Images[0].ImageURL.URL
	img, err := decodeDataURI(uri)
	if err != nil {
		return nil, "", p.invalidImage()
	}
	return img, header.Get("x-request-id"), nil
}

// drawGemini gọi generateContent (model ảnh của Gemini) hoặc predict (Imagen).
func (p *APIProvider) drawGemini(ctx context.Context, key string, req Request) ([]byte, string, error) {
	base := strings.TrimRight(p.d.BaseURL, "/") + "/v1beta/models/" + p.d.Model
	if strings.Contains(strings.ToLower(p.d.Model), "imagen") {
		payload, err := json.Marshal(map[string]any{
			"instances":  []map[string]any{{"prompt": req.Prompt}},
			"parameters": map[string]any{"sampleCount": 1},
		})
		if err != nil {
			return nil, "", MarkUnpaid(err)
		}
		status, raw, header, err := p.post(ctx, base+":predict", key, payload)
		if err != nil {
			return nil, "", err
		}
		if status != http.StatusOK {
			return nil, "", classifyAPIError(status, raw)
		}
		var parsed struct {
			Predictions []struct {
				BytesB64 string `json:"bytesBase64Encoded"`
			} `json:"predictions"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Predictions) == 0 ||
			parsed.Predictions[0].BytesB64 == "" {
			return nil, "", p.invalidImage()
		}
		img, err := decodeB64Image(parsed.Predictions[0].BytesB64)
		if err != nil {
			return nil, "", p.invalidImage()
		}
		return img, header.Get("x-request-id"), nil
	}
	payload, err := json.Marshal(map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{{"text": req.Prompt}},
		}},
	})
	if err != nil {
		return nil, "", MarkUnpaid(err)
	}
	status, raw, header, err := p.post(ctx, base+":generateContent", key, payload)
	if err != nil {
		return nil, "", err
	}
	if status != http.StatusOK {
		return nil, "", classifyAPIError(status, raw)
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						Data string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", p.invalidImage()
	}
	for _, c := range parsed.Candidates {
		for _, part := range c.Content.Parts {
			if part.InlineData.Data != "" {
				img, derr := decodeB64Image(part.InlineData.Data)
				if derr != nil {
					return nil, "", p.invalidImage()
				}
				return img, header.Get("x-request-id"), nil
			}
		}
	}
	return nil, "", p.invalidImage()
}

// post gửi JSON tới provider; Gemini đặt khoá ở query, còn lại Bearer.
func (p *APIProvider) post(ctx context.Context, url, key string, payload []byte) (int, []byte, http.Header, error) {
	if p.d.Style == StyleGemini {
		url += "?key=" + key
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, nil, MarkUnpaid(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if p.d.Style == StyleGemini {
		req.Header.Set("x-goog-api-key", key)
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := p.d.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, nil, cancelledErr()
		}
		return 0, nil, nil, drawErr(ErrCodeInternal,
			"Không nhận được phản hồi từ "+p.d.Name+" — chưa rõ đã tạo ảnh hay chưa: "+
				sanitizeProviderText(err.Error()), "Kiểm tra ảnh đích trước khi vẽ lại")
	}
	defer resp.Body.Close()
	body, rerr := io.ReadAll(io.LimitReader(resp.Body, int64(MaxImageBytes)*2))
	if rerr != nil {
		return resp.StatusCode, nil, resp.Header, drawErr(ErrCodeInternal,
			"Không đọc được phản hồi của "+p.d.Name, "Tạo lại ảnh này")
	}
	return resp.StatusCode, body, resp.Header, nil
}

// classifyAPIError xếp một phản hồi lỗi thành lớp lỗi (miễn phí khi provider
// từ chối tường minh trước khi vẽ).
func classifyAPIError(status int, body []byte) error {
	safe := sanitizeProviderText(string(body))
	switch status {
	case http.StatusUnauthorized:
		return MarkUnpaid(&DrawError{Code: ErrCodeToolMissing,
			Message:  "Khoá API bị từ chối",
			Recovery: "Mở Cài đặt → Kết nối AI và cập nhật khoá API"})
	case http.StatusTooManyRequests:
		return NewProviderBusyError(time.Now())
	}
	if status >= 400 && status < 500 {
		return MarkUnpaid(classifyRejection(status, safe, fmt.Sprintf("HTTP %d", status)))
	}
	msg := fmt.Sprintf("Provider trả HTTP %d", status)
	if safe != "" {
		msg += ": " + head(safe, 200)
	}
	return drawErr(ErrCodeInternal, msg, "Tạo lại ảnh này")
}

// invalidImage là lỗi provider trả về dữ liệu không phải ảnh.
func (p *APIProvider) invalidImage() error {
	return drawErr(ErrCodeInternal, p.d.Name+" trả về ảnh không hợp lệ", "Tạo lại ảnh này")
}

// saveImage ghi ảnh trả về: PNG hợp lệ ghi nguyên; định dạng khác chuyển
// qua ffmpeg app-managed (giống Grok).
func (p *APIProvider) saveImage(ctx context.Context, output string, b []byte) error {
	if len(b) >= 8 && bytes.Equal(b[:8], pngMagic) {
		if err := ValidatePng(b); err != nil {
			return p.invalidImage()
		}
		return writeAtomicExclusive(output, b, 0o600)
	}
	if p.d.ResolveFfmpeg == nil || p.d.Runner == nil {
		return &DrawError{Code: ErrCodeToolMissing,
			Message:  "Cần ffmpeg để đọc ảnh " + p.d.Name + " trả về",
			Recovery: "Cài ffmpeg trong Cài đặt"}
	}
	ff, err := p.d.ResolveFfmpeg(ctx)
	if err != nil || ff == "" {
		return &DrawError{Code: ErrCodeToolMissing,
			Message:  "Cần ffmpeg để đọc ảnh " + p.d.Name + " trả về",
			Recovery: "Cài ffmpeg trong Cài đặt"}
	}
	work, err := os.MkdirTemp("", "viderse-api-image-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source")
	target := filepath.Join(work, "image.png")
	if err := os.WriteFile(source, b, 0o600); err != nil {
		return err
	}
	if _, err := p.d.Runner.Run(ctx, ff, []string{
		"-hide_banner", "-loglevel", "error", "-y", "-i", source,
		"-frames:v", "1", "-f", "image2", target,
	}, tools.Spec{Timeout: 60 * time.Second}); err != nil {
		return err
	}
	if err := copyAtomicExclusive(target, output, 0o600); err != nil {
		return err
	}
	if err := ValidatePngFile(output); err != nil {
		os.Remove(output)
		return p.invalidImage()
	}
	return nil
}

// openAISize ánh canvas sang kích thước Images API chấp nhận.
func openAISize(width, height int) string {
	if width > 0 && height > 0 && width > height {
		return "1536x1024"
	}
	if width > 0 && height > 0 && height > width {
		return "1024x1536"
	}
	return "1024x1024"
}

// decodeB64Image giải base64 của payload ảnh (giới hạn kích thước như Grok).
func decodeB64Image(value string) ([]byte, error) {
	if len(value) > ((MaxImageBytes/3+1)*4)+64 {
		return nil, errors.New("ảnh quá lớn")
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(b) == 0 || len(b) > MaxImageBytes {
		return nil, errors.New("ảnh không giải mã được")
	}
	return b, nil
}

// decodeDataURI giải "data:image/png;base64,…".
func decodeDataURI(uri string) ([]byte, error) {
	i := strings.Index(uri, "base64,")
	if !strings.HasPrefix(uri, "data:") || i < 0 {
		return nil, errors.New("ảnh không phải data URI base64")
	}
	return decodeB64Image(uri[i+len("base64,"):])
}
