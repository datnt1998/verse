package imagegen

// Bản chuyển phần vẽ ảnh của src/server/tools/grok.ts (yt-factory): Grok
// (xAI) vẽ qua images API của gói SuperGrok / X Premium+ đăng nhập OAuth.
// Token lấy từ core/auth qua xai.TokenSource (tự refresh + rotate); HTTP qua
// client tiêm vào; secrets không bao giờ vào log/message — mọi body/lỗi của
// provider đi qua sanitize trước khi lên thông điệp.

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
	"regexp"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/core/auth/xai"
)

const (
	// grokDefaultBaseURL là gốc API xAI (OpenAI-compatible).
	grokDefaultBaseURL = "https://api.x.ai/v1"
	// GrokImageTimeout là timeout mặc định của một lần vẽ (3 phút như bản cũ).
	GrokImageTimeout = 3 * time.Minute
)

// GrokDeps là phụ thuộc của GrokProvider.
type GrokDeps struct {
	// Tokens cấp access token OAuth còn hạn (xai.ManagerTokens trên auth.Manager).
	Tokens xai.TokenSource
	// Client cho mọi HTTP; nil → http.DefaultClient.
	Client *http.Client
	// BaseURL ghi đè gốc API (test httptest); rỗng → api.x.ai/v1.
	BaseURL string
	// ResolveFfmpeg trả đường ffmpeg để transcode payload không-phải-PNG;
	// nil → payload không-phải-PNG là tool_missing.
	ResolveFfmpeg func(ctx context.Context) (string, error)
	// Runner cho lời gọi ffmpeg transcode. Bắt buộc khi payload có thể không
	// phải PNG — composition root tiêm tools.ExecRunner; nil → lỗi cấu hình
	// thay vì tự dựng runner ngầm.
	Runner tools.Runner
	// ReadyFunc ghi đè probe sẵn sàng (test không muốn gọi token endpoint);
	// nil → hỏi Tokens.
	ReadyFunc func(ctx context.Context) error
}

// GrokProvider vẽ ảnh qua xAI images API (model grok-imagine-image).
type GrokProvider struct {
	d GrokDeps
}

// NewGrokProvider dựng provider Grok.
func NewGrokProvider(d GrokDeps) *GrokProvider {
	if d.Client == nil {
		d.Client = http.DefaultClient
	}
	return &GrokProvider{d: d}
}

func (p *GrokProvider) Name() string  { return "grok" }
func (p *GrokProvider) Model() string { return GrokImageModel }
func (p *GrokProvider) Paid() bool    { return true }

// Ready: có credential OAuth sẵn sàng (token còn hạn hoặc refresh được).
func (p *GrokProvider) Ready(ctx context.Context) error {
	if p.d.ReadyFunc != nil {
		return p.d.ReadyFunc(ctx)
	}
	if p.d.Tokens == nil {
		return p.notLoggedIn()
	}
	if _, err := p.d.Tokens.Token(ctx); err != nil {
		return p.notLoggedIn()
	}
	return nil
}

func (p *GrokProvider) notLoggedIn() error {
	return MarkUnpaid(&DrawError{Code: ErrCodeToolMissing,
		Message:  "Chưa đăng nhập Grok",
		Recovery: "Mở Cài đặt → Kết nối AI; đăng nhập Grok"})
}

// grokAspectRatios là các tỉ lệ images API chấp nhận; auto để model tự chọn.
var grokAspectRatios = map[string]float64{
	"16:9": 16.0 / 9.0,
	"9:16": 9.0 / 16.0,
	"1:1":  1,
	"4:3":  4.0 / 3.0,
	"3:4":  3.0 / 4.0,
	"3:2":  3.0 / 2.0,
	"2:3":  2.0 / 3.0,
	"2:1":  2,
	"1:2":  1.0 / 2.0,
}

// GrokAspectRatio trả tỉ lệ được hỗ trợ gần canvas của caller nhất; "auto"
// khi kích thước không dùng được.
func GrokAspectRatio(width, height int) string {
	if width <= 0 || height <= 0 {
		return "auto"
	}
	target := float64(width) / float64(height)
	best := "auto"
	bestDist := 1e18
	for key, ratio := range grokAspectRatios {
		if d := abs(ratio - target); d < bestDist {
			best, bestDist = key, d
		}
	}
	return best
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// decodeGrokB64 giải base64 của payload ảnh (không ép PNG — JPEG/WebP đi qua
// ffmpeg ở writePng).
func decodeGrokB64(value string) ([]byte, error) {
	bad := errors.New("grok trả về ảnh không hợp lệ")
	if len(value) > ((MaxImageBytes/3+1)*4)+64 || len(value)%4 != 0 {
		return nil, bad
	}
	for _, c := range []byte(value) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return nil, bad
		}
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(b) == 0 || len(b) > MaxImageBytes || base64.StdEncoding.EncodeToString(b) != value {
		return nil, bad
	}
	return b, nil
}

// referenceDataURI đọc ảnh tham chiếu cục bộ thành data URI — không bao giờ
// để provider fetch URL hay đường dẫn không tin cậy.
func referenceDataURI(file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	var mime string
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], pngMagic):
		mime = "image/png"
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		mime = "image/jpeg"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		mime = "image/webp"
	default:
		return "", drawErr(ErrCodeInvalidRequest, "Ảnh tham chiếu phải là PNG, JPEG hoặc WebP hợp lệ", "")
	}
	if len(b) > MaxImageBytes {
		return "", drawErr(ErrCodeInvalidRequest, "Ảnh tham chiếu phải là PNG, JPEG hoặc WebP hợp lệ", "")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// writePng lưu bytes thành PNG hợp lệ tại output; payload không-phải-PNG đi
// qua ffmpeg app-managed.
func (p *GrokProvider) writePng(ctx context.Context, output string, b []byte) error {
	if len(b) >= 8 && bytes.Equal(b[:8], pngMagic) {
		if err := ValidatePng(b); err != nil {
			return p.invalidImage()
		}
		return writeAtomicExclusive(output, b, 0o600)
	}
	if p.d.ResolveFfmpeg == nil {
		return &DrawError{Code: ErrCodeToolMissing,
			Message:  "Cần ffmpeg để đọc ảnh Grok trả về",
			Recovery: "Cài ffmpeg trong Cài đặt"}
	}
	ff, err := p.d.ResolveFfmpeg(ctx)
	if err != nil || ff == "" {
		return &DrawError{Code: ErrCodeToolMissing,
			Message:  "Cần ffmpeg để đọc ảnh Grok trả về",
			Recovery: "Cài ffmpeg trong Cài đặt"}
	}
	work, err := os.MkdirTemp("", "ytf-grok-image-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source")
	target := filepath.Join(work, "image.png")
	if err := os.WriteFile(source, b, 0o600); err != nil {
		return err
	}
	if p.d.Runner == nil {
		// Ảnh không phải PNG cần ffmpeg để chuyển; thiếu Runner là lỗi cấu hình
		// (composition root phải tiêm tools.ExecRunner).
		return drawErr(ErrCodeInternal, "Thiếu runner chuyển ảnh Grok sang PNG", "Kiểm tra cấu hình tools")
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

func (p *GrokProvider) invalidImage() *DrawError {
	return drawErr(ErrCodeInternal, "Grok trả về ảnh không hợp lệ", "Tạo lại ảnh này")
}

// grokEntitlementRe: câu chữ gói thuê bao = hạn mức tuần của gói hoặc thiếu
// gói — không bao giờ là lỗi auth.
var grokEntitlementRe = regexp.MustCompile(`(?i)out of credits|grok subscription|spending limit|subscription required|do not have an active`)

// classifyGrokImageFailure xếp HTTP status + body thành lỗi đúng class. Một
// HTTP response nghĩa là provider đã từ chối tường minh — các lớp này miễn
// phí (MarkUnpaid), trừ 5xx chưa rõ (giữ charged).
func classifyGrokImageFailure(status int, text string) error {
	switch {
	case status == 401:
		return MarkUnpaid(&DrawError{Code: ErrCodeToolMissing,
			Message:  "Phiên đăng nhập Grok đã hết hạn",
			Recovery: "Mở Cài đặt → Tạo ảnh minh hoạ; đăng nhập Grok lại"})
	case status == 429:
		return NewProviderBusyError(time.Now())
	case status == 403 && grokEntitlementRe.MatchString(text):
		quota := &DrawError{Code: ErrCodeQuota,
			Message:  "Hạn mức tạo ảnh Grok đã hết; hàng đợi đã tạm dừng",
			Recovery: "Gói Grok có giới hạn dùng chung theo tuần — chờ tới kỳ sau hoặc kiểm tra gói đăng ký, rồi chạy lại"}
		return MarkUnpaid(quota)
	}
	safe := sanitizeProviderText(text)
	fallback := fmt.Sprintf("HTTP %d", status)
	if safe == "" {
		safe = fallback
	}
	if status >= 400 && status < 500 {
		// 4xx khác: provider từ chối request trước khi vẽ — không tính phí.
		return MarkUnpaid(classifyRejection(status, safe, fallback))
	}
	return classifyRejection(status, safe, fallback)
}

// classifyRejection dựng DrawError từ chữ của provider.
func classifyRejection(status int, safe, fallback string) error {
	classified := ClassifyImageFailure(safe, nil)
	if IsImageRefused(classified) || IsProviderBusy(classified) {
		return classified
	}
	if de, ok := drawErrorOf(classified); ok && de.Code == ErrCodeQuota {
		return classified
	}
	msg := fmt.Sprintf("Grok không trả về ảnh hợp lệ (HTTP %d)", status)
	if safe != "" && safe != fallback {
		msg += ": " + safe
	}
	return drawErr(ErrCodeInternal, msg, "Tạo lại ảnh này; không có ảnh thay thế giả")
}

// grokImageBody là body của POST images/generations|edits.
type grokImageBody struct {
	Model          string        `json:"model"`
	Prompt         string        `json:"prompt"`
	AspectRatio    string        `json:"aspect_ratio,omitempty"`
	Image          *grokImageRef `json:"image,omitempty"`
	Resolution     string        `json:"resolution"`
	N              int           `json:"n"`
	ResponseFormat string        `json:"response_format"`
}

type grokImageRef struct {
	URL string `json:"url"`
}

// Draw thực hiện MỘT lần vẽ trả phí qua images API của gói.
func (p *GrokProvider) Draw(ctx context.Context, req Request, journal JournalHandle) (Receipt, error) {
	if p.d.Tokens == nil {
		return Receipt{}, p.notLoggedIn()
	}
	if err := p.Ready(ctx); err != nil {
		return Receipt{}, err
	}
	token, err := p.d.Tokens.Token(ctx)
	if err != nil {
		return Receipt{}, p.notLoggedIn()
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = GrokImageTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := grokImageBody{
		Model:          GrokImageModel,
		Prompt:         req.Prompt,
		Resolution:     "1k",
		N:              1,
		ResponseFormat: "b64_json",
	}
	if req.Width > 0 && req.Height > 0 {
		body.AspectRatio = GrokAspectRatio(req.Width, req.Height)
	}
	base := p.d.BaseURL
	if base == "" {
		base = grokDefaultBaseURL
	}
	url := base + "/images/generations"
	if req.Reference != "" {
		uri, err := referenceDataURI(req.Reference)
		if err != nil {
			return Receipt{}, MarkUnpaid(err)
		}
		body.Image = &grokImageRef{URL: uri}
		url = base + "/images/edits"
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	hreq, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Receipt{}, MarkUnpaid(err)
	}
	hreq.Header.Set("Authorization", "Bearer "+token)
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json")
	resp, err := p.d.Client.Do(hreq)
	if err != nil {
		if ctx.Err() != nil {
			// Client đã nhận request (hoặc chưa) — không chứng minh được; huỷ
			// sau khi gửi KHÔNG đồng nghĩa provider huỷ, nên vẫn tính phí.
			return Receipt{}, cancelledErr()
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return Receipt{}, drawErr(ErrCodeInternal, "Tạo ảnh Grok bị timeout", "Tạo lại ảnh này")
		}
		// POST không có phản hồi: xAI có thể đã nhận và tính phí. Không tự
		// retry/không retry ngầm; attempt giữ charged và bề mặt lỗi để người
		// vận hành quyết định (không có idempotency key phía provider).
		return Receipt{}, drawErr(ErrCodeInternal,
			"Không nhận được phản hồi từ Grok — chưa rõ đã tạo ảnh hay chưa: "+sanitizeProviderText(err.Error()),
			"Kiểm tra ảnh đích trước khi vẽ lại")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return Receipt{}, classifyGrokImageFailure(resp.StatusCode, string(text))
	}
	var parsed struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, int64(MaxImageBytes)*2)).Decode(&parsed); err != nil || len(parsed.Data) == 0 || parsed.Data[0].B64 == "" {
		return Receipt{}, p.invalidImage()
	}
	if ctx.Err() != nil {
		return Receipt{}, cancelledErr()
	}
	raw, err := decodeGrokB64(parsed.Data[0].B64)
	if err != nil {
		return Receipt{}, p.invalidImage()
	}
	if err := p.writePng(ctx, req.Output, raw); err != nil {
		return Receipt{}, err
	}
	sum, _ := sha256File(req.Output)
	receipt := Receipt{
		Provider:  "grok",
		Model:     GrokImageModel,
		RequestID: resp.Header.Get("x-request-id"),
		PngSha256: sum,
	}
	if err := writeJSONAtomic(req.Output+".receipt.json", receipt); err != nil {
		// Ảnh đã nguyên tử trên đĩa; sidecar lỗi không được xoá ảnh và không
		// được dẫn tới vẽ lại.
		return Receipt{}, &StoreError{Err: err}
	}
	return receipt, nil
}

// Reconcile của Grok không có scratch phục hồi riêng: API HTTP không để lại
// session trên đĩa, nên luật chung (kiểm tra output) là đủ.
func (p *GrokProvider) Reconcile(ctx context.Context, scratch json.RawMessage, req Request) (Receipt, bool, error) {
	return Receipt{}, false, nil
}
