package bootstrap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTTSConfigDefaults(t *testing.T) {
	var c Config
	c.FillDefaults()
	if c.TTS.Engine != "vieneu" || c.TTS.Model != "vieneu-v3-turbo" || c.TTS.Device != "cpu" || c.TTS.Quality != "standard" {
		t.Fatalf("mặc định tts = %+v", c.TTS)
	}
	c.TTS.Quality = "HIGH"
	c.TTS.Device = "GPU"
	c.FillDefaults()
	if c.TTS.Quality != "high" {
		t.Fatalf("chất lượng phải chuẩn hoá về chữ thường, được %q", c.TTS.Quality)
	}
	if c.TTS.Device != "gpu" {
		t.Fatalf("thiết bị phải chuẩn hoá, được %q", c.TTS.Device)
	}
	c.TTS.Quality = "siêu nhanh"
	c.FillDefaults()
	if c.TTS.Quality != "standard" {
		t.Fatalf("chất lượng lạ phải rơi về standard, được %q", c.TTS.Quality)
	}
}

func TestMergeConfigTTSFieldWise(t *testing.T) {
	base := Config{TTS: TTSConfig{Engine: "vieneu", Model: "vieneu-v3-turbo", Device: "cpu", Quality: "standard", Endpoint: "http://127.0.0.1:8000", APIKey: "secret"}}
	overlay := Config{TTS: TTSConfig{Quality: "high"}}
	got := MergeConfig(base, overlay)
	if got.TTS.Quality != "high" {
		t.Fatalf("quality = %q, muốn high", got.TTS.Quality)
	}
	if got.TTS.Engine != "vieneu" || got.TTS.Endpoint != "http://127.0.0.1:8000" || got.TTS.APIKey != "secret" {
		t.Fatalf("các khoá không khai báo ở lớp gần phải thừa hưởng: %+v", got.TTS)
	}
	// Lớp gần đổi engine → giữ nguyên các khoá khác.
	got = MergeConfig(base, Config{TTS: TTSConfig{Engine: "openai-speech", Endpoint: "http://may-khac:9000"}})
	if got.TTS.Engine != "openai-speech" || got.TTS.Endpoint != "http://may-khac:9000" || got.TTS.Quality != "standard" {
		t.Fatalf("merge engine = %+v", got.TTS)
	}
}

func TestTTSConfigJSONRoundTrip(t *testing.T) {
	raw := `{"provider":"p","model":"m","tts":{"engine":"openai-speech","endpoint":"http://x:8000","api_key":"k"}}`
	var c Config
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.TTS.Engine != "openai-speech" || c.TTS.Endpoint != "http://x:8000" || c.TTS.APIKey != "k" {
		t.Fatalf("tts đọc từ JSON = %+v", c.TTS)
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"tts":{`) {
		t.Fatalf("JSON thiếu khoá tts: %s", out)
	}
	// Cấu hình không có tts thì không được tự thêm khoá rỗng vào tệp.
	empty, err := json.Marshal(Config{Provider: "p", ModelName: "m"})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if strings.Contains(string(empty), "tts") {
		t.Fatalf("tts rỗng vẫn bị ghi ra JSON: %s", empty)
	}
}
