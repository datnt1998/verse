// Package bootcfg nối tầng voice với cấu hình của ứng dụng (khoá `tts` trong
// config.json toàn cục). Chỉ cần import trống là Service dùng chung
// (voice.Shared) tự có cấu hình — tách ra riêng để internal/voice không phải
// phụ thuộc bootstrap mà vẫn dùng được ở mọi nơi.
package bootcfg

import (
	"log/slog"
	"os"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

func init() {
	voice.SetConfigProvider(FromBootstrap)
}

// FromBootstrap đọc khoá `tts` của config.json toàn cục. Không đọc được thì dùng
// mặc định (engine vieneu, chất lượng standard) — giọng đọc hỏng cấu hình không
// được làm chết cả ứng dụng.
func FromBootstrap() voice.Config {
	path := bootstrap.DefaultConfigPath()
	if path == "" {
		return voice.Config{}.WithDefaults()
	}
	fileCfg, err := bootstrap.LoadConfigFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("không đọc được cấu hình giọng đọc, dùng mặc định", "module", "voice", "path", path, "err", err)
		}
		return voice.Config{}.WithDefaults()
	}
	return FromTTS(fileCfg.TTS)
}

// FromTTS map khoá `tts` trong config.json sang voice.Config.
func FromTTS(t bootstrap.TTSConfig) voice.Config {
	return voice.Config{
		Engine:   t.Engine,
		Model:    t.Model,
		Device:   t.Device,
		Quality:  t.Quality,
		Endpoint: t.Endpoint,
		APIKey:   t.APIKey,
		Home:     voice.DefaultHome(),
	}.WithDefaults()
}

// ToTTS map ngược voice.Config sang khoá `tts` (ghi cấu hình, test).
func ToTTS(c voice.Config) bootstrap.TTSConfig {
	c = c.WithDefaults()
	return bootstrap.TTSConfig{
		Engine:   c.Engine,
		Model:    c.Model,
		Device:   c.Device,
		Quality:  c.Quality,
		Endpoint: c.Endpoint,
		APIKey:   c.APIKey,
	}
}
