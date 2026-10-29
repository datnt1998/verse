package openaispeech

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// applyAtempo đổi tốc độ đọc mà giữ nguyên cao độ (ffmpeg atempo). Máy đọc VieNeu
// bỏ qua trường speed trong yêu cầu, nên tốc độ do tầng này lo.
func applyAtempo(ctx context.Context, wav []byte, speed float64) ([]byte, WAVInfo, error) {
	if math.Abs(speed-1) < 0.01 {
		info, err := InspectWAV(wav)
		return wav, info, err
	}
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, WAVInfo{}, fmt.Errorf("cần ffmpeg để đổi tốc độ đọc (cài: brew install ffmpeg): %w", err)
	}
	cmd := exec.CommandContext(ctx, bin,
		"-hide_banner", "-loglevel", "error",
		"-f", "wav", "-i", "pipe:0",
		"-filter:a", atempoFilter(speed),
		"-f", "wav", "pipe:1",
	)
	cmd.Stdin = bytes.NewReader(wav)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, WAVInfo{}, fmt.Errorf("ffmpeg đổi tốc độ đọc thất bại: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	normalized, info, err := NormalizeWAV(out.Bytes())
	if err != nil {
		return nil, WAVInfo{}, err
	}
	return normalized, info, nil
}

// atempoFilter ghép chuỗi atempo: mỗi bộ lọc chỉ nhận 0.5–2.0 nên tốc độ ngoài
// khoảng đó phải chia thành nhiều bước.
func atempoFilter(speed float64) string {
	if speed <= 0 {
		return "atempo=1"
	}
	var parts []string
	s := speed
	for s > 2 {
		parts = append(parts, "atempo=2")
		s /= 2
	}
	for s < 0.5 {
		parts = append(parts, "atempo=0.5")
		s /= 0.5
	}
	parts = append(parts, "atempo="+strconv.FormatFloat(s, 'f', 4, 64))
	return strings.Join(parts, ",")
}
