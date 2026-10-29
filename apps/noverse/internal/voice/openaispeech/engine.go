// Package openaispeech là engine giọng đọc nói chuyện với bất kỳ máy chủ TTS nào
// theo chuẩn OpenAI Speech API: POST /v1/audio/speech, GET/POST /v1/voices,
// GET /v1/models, GET /health. Nhờ vậy VieNeu chạy Docker, máy khác trong mạng,
// hay dịch vụ TTS tương thích OpenAI đều dùng được mà không phải sửa Go.
package openaispeech

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/datnt1998/noverse/internal/voice"
)

func init() {
	voice.Register("openai-speech", newEngine)
}

// Engine là engine từ xa: không cài, không quản tiến trình.
type Engine struct {
	cfg voice.Config
	c   *Client
}

func newEngine(cfg voice.Config) (voice.Engine, error) {
	return &Engine{
		cfg: cfg,
		c:   NewClient(ClientOptions{Endpoint: cfg.Endpoint, APIKey: cfg.APIKey, Model: cfg.Model}),
	}, nil
}

// noEndpoint là lỗi chưa cấu hình địa chỉ máy chủ.
var noEndpoint = errors.New("máy đọc openai-speech chưa có địa chỉ máy chủ (đặt tts.endpoint, vd http://127.0.0.1:8000)")

func (e *Engine) endpointReady() bool { return strings.TrimSpace(e.cfg.Endpoint) != "" }

// Name trả tên engine.
func (e *Engine) Name() string { return "openai-speech" }

// Capabilities mô tả năng lực của chuẩn OpenAI Speech API.
func (e *Engine) Capabilities() voice.Capabilities {
	return voice.Capabilities{
		Cloning:    true,
		Streaming:  true,
		SampleRate: 48000,
		Qualities:  voice.Qualities,
	}
}

// Status hỏi /health của máy chủ; chưa có endpoint thì báo missing chứ không lỗi.
func (e *Engine) Status(ctx context.Context) voice.Status {
	st := voice.Status{Engine: e.Name(), Model: e.c.model, Device: e.cfg.Device}
	if !e.endpointReady() {
		st.State = voice.StateMissing
		return st
	}
	h, err := e.c.Health(ctx)
	if err != nil {
		st.State = voice.StateError
		st.Error = err.Error()
		return st
	}
	if !h.OK() {
		st.State = voice.StateError
		st.Error = h.Error
		if st.Error == "" {
			st.Error = "máy đọc báo lỗi"
		}
		return st
	}
	st.State = voice.StateReady
	st.Ready = true
	if h.SampleRate > 0 {
		st.Model = fmt.Sprintf("%s (%d Hz)", e.c.model, h.SampleRate)
	}
	return st
}

// Voices liệt kê giọng của máy chủ.
func (e *Engine) Voices(ctx context.Context) ([]voice.Voice, error) {
	if !e.endpointReady() {
		return nil, noEndpoint
	}
	return e.c.Voices(ctx)
}

// Synthesize đọc một đoạn.
func (e *Engine) Synthesize(ctx context.Context, req voice.SynthRequest) (voice.Audio, error) {
	req = req.Normalized()
	if req.RefAudio != "" {
		return voice.Audio{}, fmt.Errorf("%w: máy đọc từ xa không nhận clip mẫu cho từng lượt; hãy thêm giọng trước", voice.ErrUnsupported)
	}
	if !e.endpointReady() {
		return voice.Audio{}, noEndpoint
	}
	return e.c.Speak(ctx, req.Text, req.VoiceID, SpeakOptions{SampleRate: 48000, Speed: req.Speed})
}

// AddVoice nạp clip mẫu thành giọng clone trên máy chủ.
func (e *Engine) AddVoice(ctx context.Context, name, wavPath string) (voice.Voice, error) {
	if _, err := os.Stat(wavPath); err != nil {
		return voice.Voice{}, err
	}
	if !e.endpointReady() {
		return voice.Voice{}, noEndpoint
	}
	if err := e.c.AddVoice(ctx, name, wavPath, "", true); err != nil {
		return voice.Voice{}, err
	}
	return voice.Voice{ID: name, Name: name, Source: voice.SourceClone, File: wavPath}, nil
}

// Close không có gì để dọn (máy chủ do người khác quản).
func (e *Engine) Close() error { return nil }
