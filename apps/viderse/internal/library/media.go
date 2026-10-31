package library

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Đo tệp phương tiện cho lớp phủ: ffprobe lấy thời lượng/kích thước, ffmpeg
// lấy 8 khung rgb24 64×36 để đoán nền (đen / xanh / không). Phân loại nền
// port từ yt-factory src/server/editor/media-background.ts.

// Kích thước khung mẫu và số khung (giữ của yt-factory).
const (
	bgSampleWidth  = 64
	bgSampleHeight = 36
	bgSampleCount  = 8
	bgBorder       = 4
)

// Runner chạy một binary và trả stdout — cmd/viderse nối vào tools.Runner.
type Runner interface {
	Run(ctx context.Context, file string, args []string) (stdout string, err error)
}

// Binaries trả đường ffmpeg/ffprobe đã chọn; ok=false khi máy chưa có.
type Binaries interface {
	Tools(ctx context.Context) (ffmpeg, ffprobe string, ok bool)
}

// ProbeInfo là siêu dữ liệu đọc từ tệp.
type ProbeInfo struct {
	DurationSec *float64
	Width       *int
	Height      *int
}

// probeOutput là hình `ffprobe -print_format json`.
type probeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Duration  string `json:"duration"`
	} `json:"streams"`
}

// probeFile đọc siêu dữ liệu bằng ffprobe. Lỗi chạy là lỗi nội bộ (tệp hỏng
// hoặc ffprobe lỗi) — caller đổi thành bad_media.
func probeFile(ctx context.Context, runner Runner, ffprobe, file string) (ProbeInfo, error) {
	stdout, err := runner.Run(ctx, ffprobe, []string{
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", file,
	})
	if err != nil {
		return ProbeInfo{}, fmt.Errorf("ffprobe: %w", err)
	}
	var out probeOutput
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return ProbeInfo{}, fmt.Errorf("đọc kết quả ffprobe: %w", err)
	}
	info := ProbeInfo{}
	if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil && d > 0 {
		info.DurationSec = &d
	}
	for _, stream := range out.Streams {
		if stream.CodecType != "video" || stream.Width <= 0 || stream.Height <= 0 {
			continue
		}
		width, height := stream.Width, stream.Height
		info.Width, info.Height = &width, &height
		if info.DurationSec == nil {
			if d, err := strconv.ParseFloat(stream.Duration, 64); err == nil && d > 0 {
				info.DurationSec = &d
			}
		}
		break
	}
	// Bắt buộc một stream video/ảnh có kích thước: audio-only hoặc container
	// không có hình là tệp lớp phủ không dùng được.
	if info.Width == nil || info.Height == nil {
		return ProbeInfo{}, fmt.Errorf("không có stream hình trong tệp")
	}
	return info, nil
}

// sampleFrames kéo tối đa 8 khung đều nhau thành một khối rgb24 64×36. Video
// dùng fps=N/duration để trải mẫu khắp tệp; ảnh tĩnh (hoặc chưa biết dài bao
// nhiêu) dùng fps=1 — với ảnh một khung thì đó chính là ảnh.
func sampleFrames(ctx context.Context, runner Runner, ffmpeg, file string, durationSec *float64) ([]byte, error) {
	rate := "1"
	if durationSec != nil && *durationSec > 0 {
		rate = strconv.FormatFloat(bgSampleCount / *durationSec, 'f', 6, 64)
	}
	stdout, err := runner.Run(ctx, ffmpeg, []string{
		"-v", "error", "-i", file,
		"-vf", fmt.Sprintf("fps=%s,scale=%d:%d", rate, bgSampleWidth, bgSampleHeight),
		"-frames:v", strconv.Itoa(bgSampleCount),
		"-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1",
	})
	if err != nil {
		return nil, fmt.Errorf("lấy khung mẫu của %s: %w", file, err)
	}
	return []byte(stdout), nil
}

// pixelStats là bộ đếm của classifyBackground.
type pixelStats struct {
	border      int
	total       int
	darkBorder  int
	darkTotal   int
	greenBorder int
}

// classifyBackground đoán nền từ các khung rgb24 đã lấy mẫu:
//   - black: ≥85% điểm ảnh viền có luma <24 và ≥50% toàn khung như vậy;
//   - green: ≥70% điểm ảnh viền nghiêng lục (G>1.4R && G>1.4B && G>70);
//   - còn lại: none.
func classifyBackground(frames []byte, width, height int) domain.OverlayBackground {
	frameBytes := width * height * 3
	frameCount := len(frames) / frameBytes
	var stats pixelStats
	for frame := range frameCount {
		base := frame * frameBytes
		for y := range height {
			onBorderY := y < bgBorder || y >= height-bgBorder
			for x := range width {
				onBorder := onBorderY || x < bgBorder || x >= width-bgBorder
				offset := base + (y*width+x)*3
				r := float64(frames[offset])
				g := float64(frames[offset+1])
				b := float64(frames[offset+2])
				luma := 0.299*r + 0.587*g + 0.114*b
				greenish := g > 1.4*r && g > 1.4*b && g > 70
				stats.total++
				if luma < 24 {
					stats.darkTotal++
				}
				if !onBorder {
					continue
				}
				stats.border++
				if luma < 24 {
					stats.darkBorder++
				}
				if greenish {
					stats.greenBorder++
				}
			}
		}
	}
	if stats.border > 0 && ratio(stats.darkBorder, stats.border) >= 0.85 && ratio(stats.darkTotal, stats.total) >= 0.5 {
		return domain.BgBlack
	}
	if stats.border > 0 && ratio(stats.greenBorder, stats.border) >= 0.7 {
		return domain.BgGreen
	}
	return domain.BgNone
}

func ratio(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

// detectBackground chạy cả hai bước; tệp không lấy được khung nào trả none.
func detectBackground(ctx context.Context, runner Runner, ffmpeg, file string, durationSec *float64) domain.OverlayBackground {
	frames, err := sampleFrames(ctx, runner, ffmpeg, file, durationSec)
	if err != nil || len(frames) == 0 {
		return domain.BgNone
	}
	return classifyBackground(frames, bgSampleWidth, bgSampleHeight)
}

// ── MIME ───────────────────────────────────────────────────────────────────

// MimeForFile trả MIME theo đuôi tệp (yt-factory OVERLAY_MIME); "" khi đuôi
// không được nhận.
func MimeForFile(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return ""
	}
	switch strings.ToLower(name[dot:]) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// allowedMimeList là danh sách đuôi nhận được cho câu lỗi (thứ tự cố định).
func allowedMimeList() []string {
	return []string{".jpeg", ".jpg", ".m4v", ".mov", ".mp4", ".png", ".webm", ".webp"}
}

// ── đọc chữ ký nội dung ────────────────────────────────────────────────────

// sniffMediaClass đoán lớp media từ byte đầu: "bmff" (MP4/MOV/M4V), "webm",
// "png", "jpeg", "webp"; "" khi không nhận diện được.
func sniffMediaClass(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:4], []byte("\x89PNG")):
		return "png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "jpeg"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "webm"
	case isBMFF(data):
		return "bmff"
	}
	return ""
}

// isBMFF nhận container ISO BMFF (MP4/MOV/M4V): box đầu có size hợp lệ và type
// là một trong các box mở đầu quen thuộc.
func isBMFF(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	size := binary.BigEndian.Uint32(data[:4])
	if size != 1 && (size < 8 || int64(size) > int64(len(data))+1<<30) {
		return false
	}
	switch string(data[4:8]) {
	case "ftyp", "moov", "mdat", "free", "wide", "skip", "styp":
		return true
	}
	return false
}

// expectedClass là lớp nội dung mà một MIME được nhận phải khớp.
func expectedClass(mime string) string {
	switch mime {
	case "video/mp4", "video/quicktime":
		return "bmff"
	case "video/webm":
		return "webm"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	}
	return ""
}

// errToolMissing dựng lỗi thiếu ffmpeg/ffprobe.
func errToolMissing() *Error {
	return &Error{
		Code:     CodeToolMissing,
		Message:  "ffmpeg chưa sẵn sàng — cài trong Cài đặt → Công cụ",
		Recovery: "Cài ffmpeg trong Cài đặt → Công cụ rồi thử lại",
	}
}

// errBadMedia bọc lỗi đọc tệp thành lỗi mặt API.
func errBadMedia(err error) *Error {
	return &Error{
		Code:     CodeBadMedia,
		Message:  "Không đọc được tệp: " + firstLine(err.Error()),
		Recovery: "Chọn tệp video hoặc ảnh khác",
	}
}

// firstLine cắt câu lỗi gốc tới dòng đầu (ffprobe in nhiều dòng).
func firstLine(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[:i]
	}
	return msg
}

// isErrNotFound gói lỗi store không có hàng.
func isErrNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
