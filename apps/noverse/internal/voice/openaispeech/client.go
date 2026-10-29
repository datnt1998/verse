package openaispeech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Client nói chuyện với một máy chủ TTS tương thích OpenAI Speech API.
type Client struct {
	base        string
	key         string
	model       string
	ignoreSpeed bool // máy chủ bỏ qua speed (VieNeu) → chỉnh bằng ffmpeg ở client
	hc          *http.Client
}

// ClientOptions là tham số dựng Client.
type ClientOptions struct {
	Endpoint    string
	APIKey      string
	Model       string
	IgnoreSpeed bool
}

// NewClient dựng client; endpoint có thể kèm tiền tố đường dẫn (vd http://host/tts).
func NewClient(o ClientOptions) *Client {
	return &Client{
		base:        strings.TrimRight(o.Endpoint, "/"),
		key:         o.APIKey,
		model:       o.Model,
		ignoreSpeed: o.IgnoreSpeed,
		hc:          &http.Client{},
	}
}

// Health là phản hồi GET /health của máy chủ.
type Health struct {
	Status     string `json:"status"`
	Backend    string `json:"backend"`
	SampleRate int    `json:"sample_rate"`
	Active     int    `json:"active"`
	Waiting    int    `json:"waiting"`
	Error      string `json:"error"`
}

// OK cho biết máy chủ sẵn sàng nhận việc.
func (h Health) OK() bool { return h.Status == "ok" }

// Health gọi GET /health.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var h Health
	if err := c.getJSON(ctx, "/health", &h); err != nil {
		return h, err
	}
	return h, nil
}

// Models gọi GET /v1/models để biết model/backend thật của máy chủ.
func (c *Client) Models(ctx context.Context) ([]ModelInfo, error) {
	var body struct {
		Data []ModelInfo `json:"data"`
	}
	if err := c.getJSON(ctx, "/v1/models", &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// ModelInfo là một model trong GET /v1/models.
type ModelInfo struct {
	ID         string `json:"id"`
	Backend    string `json:"backend"`
	SampleRate int    `json:"sample_rate"`
	MaxStreams int    `json:"max_streams"`
}

type wireVoice struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Gender      string   `json:"gender"`
	Featured    int      `json:"featured"`
	Aliases     []string `json:"aliases"`
}

// Voices gọi GET /v1/voices.
func (c *Client) Voices(ctx context.Context) ([]voice.Voice, error) {
	var body struct {
		Data []wireVoice `json:"data"`
	}
	if err := c.getJSON(ctx, "/v1/voices", &body); err != nil {
		return nil, err
	}
	out := make([]voice.Voice, 0, len(body.Data))
	for _, v := range body.Data {
		name := v.Name
		if name == "" {
			name = v.ID
		}
		out = append(out, voice.Voice{
			ID:     v.ID,
			Name:   name,
			Gender: normalizeGender(v.Gender),
			Source: voice.SourceBuiltin,
		})
	}
	return out, nil
}

// SpeakOptions điều chỉnh một lượt đọc.
type SpeakOptions struct {
	SampleRate int
	Speed      float64
}

// Speak gọi POST /v1/audio/speech và trả WAV đã vá header.
func (c *Client) Speak(ctx context.Context, text, voiceID string, o SpeakOptions) (voice.Audio, error) {
	if strings.TrimSpace(text) == "" {
		return voice.Audio{}, errors.New("chữ cần đọc đang trống")
	}
	sr := o.SampleRate
	if sr == 0 {
		sr = 48000
	}
	payload := map[string]any{
		"model":           c.model,
		"input":           text,
		"response_format": "wav",
		"sample_rate":     sr,
	}
	if voiceID != "" {
		payload["voice"] = voiceID
	}
	if o.Speed > 0 && o.Speed != 1 {
		payload["speed"] = o.Speed
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return voice.Audio{}, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/audio/speech", bytes.NewReader(body), "application/json")
	if err != nil {
		return voice.Audio{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return voice.Audio{}, fmt.Errorf("đọc audio từ máy đọc: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return voice.Audio{}, httpError(resp, raw)
	}
	wav, info, err := NormalizeWAV(raw)
	if err != nil {
		return voice.Audio{}, fmt.Errorf("máy đọc trả dữ liệu không phải WAV: %w", err)
	}
	if c.ignoreSpeed && o.Speed > 0 && o.Speed != 1 {
		wav, info, err = applyAtempo(ctx, wav, o.Speed)
		if err != nil {
			return voice.Audio{}, err
		}
	}
	return voice.Audio{WAV: wav, SampleRate: info.SampleRate, DurationSec: info.DurationSec()}, nil
}

// AddVoice gọi POST /v1/voices (multipart) để nạp một clip mẫu thành giọng clone.
// Máy chủ trả 409 nếu tên trùng giọng có sẵn.
func (c *Client) AddVoice(ctx context.Context, name, wavPath, description string, denoise bool) error {
	f, err := os.Open(wavPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("name", name); err != nil {
		return err
	}
	if description != "" {
		if err := mw.WriteField("description", description); err != nil {
			return err
		}
	}
	if err := mw.WriteField("denoise", strconv.FormatBool(denoise)); err != nil {
		return err
	}
	part, err := mw.CreateFormFile("file", filepath.Base(wavPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	resp, err := c.do(ctx, http.MethodPost, "/v1/voices", &buf, mw.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return httpError(resp, raw)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return httpError(resp, raw)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("máy đọc trả JSON không đọc được (%s): %w", path, err)
	}
	return nil
}

// do gửi request, tự thử lại khi máy đọc báo bận (429 + Retry-After).
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	// Đọc hết thân vào bộ nhớ để thử lại được (multipart/JSON đều nhỏ).
	var payload []byte
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}
	const attempts = 4
	for i := 0; ; i++ {
		var rd io.Reader
		if payload != nil {
			rd = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if c.key != "" {
			req.Header.Set("Authorization", "Bearer "+c.key)
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("không gọi được máy đọc %s: %w", c.base, err)
		}
		if resp.StatusCode != http.StatusTooManyRequests || i == attempts-1 {
			return resp, nil
		}
		wait := retryAfter(resp)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func retryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
			return min(time.Duration(secs)*time.Second, 30*time.Second)
		}
	}
	return time.Second
}

// httpError biến phản hồi lỗi của máy đọc thành lỗi Go có thông điệp đọc được.
func httpError(resp *http.Response, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	var shape struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &shape) == nil && shape.Error.Message != "" {
		msg = shape.Error.Message
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("máy đọc từ chối khoá truy cập (HTTP %d): %s", resp.StatusCode, msg)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", voice.ErrBusy, msg)
	}
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("máy đọc lỗi HTTP %d: %s", resp.StatusCode, msg)
}

func normalizeGender(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "male", "m", "nam":
		return "male"
	case "female", "f", "nữ", "nu":
		return "female"
	}
	return ""
}

// BaseURL trả địa chỉ máy chủ (dùng cho log/chẩn đoán).
func (c *Client) BaseURL() string {
	u, err := url.Parse(c.base)
	if err != nil {
		return c.base
	}
	return u.String()
}
