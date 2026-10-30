package toolsvc

// Bộ chuyển Runner của internal/tools: mọi tiến trình con (yt-dlp, ffmpeg,
// uv, Chrome, Codex) ghi stdout/stderr vào kho nhật ký theo đúng tên công cụ,
// đã che bí mật. Gắn ở composition (cmd) nên mọi lượt cài/chạy đều có log.

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// LineWriter là writer theo dòng có Flush (internal/logs.Writer thoả).
type LineWriter interface {
	io.Writer
	Flush()
}

// WriterFor mở một writer cho (công cụ, mức log) — thường là bao đóng của
// logs.Store.Writer, nên toolsvc không phụ thuộc kiểu của kho nhật ký.
type WriterFor func(tool, level string) LineWriter

// loggingRunner bọc Runner: chèn writer theo công cụ rồi đóng (flush) sau
// khi tiến trình kết thúc, kể cả khi huỷ/lỗi.
type loggingRunner struct {
	inner     tools.Runner
	writerFor WriterFor
}

// NewLoggingRunner trả Runner ghi log con. writerFor nil → giữ nguyên inner.
func NewLoggingRunner(inner tools.Runner, writerFor WriterFor) tools.Runner {
	if inner == nil || writerFor == nil {
		return inner
	}
	return loggingRunner{inner: inner, writerFor: writerFor}
}

func (r loggingRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	w := r.writerFor(toolOfFile(file), "info")
	if spec.Log != nil {
		spec.Log = io.MultiWriter(spec.Log, w)
	} else {
		spec.Log = w
	}
	res, err := r.inner.Run(ctx, file, args, spec)
	w.Flush()
	return res, err
}

// toolOfFile ánh xạ binary → id hàng công cụ (tên theo việc làm).
func toolOfFile(file string) string {
	base := strings.ToLower(filepath.Base(file))
	switch {
	case strings.Contains(base, "yt-dlp"), strings.Contains(base, "ytdlp"):
		return ToolYtDlp
	case strings.Contains(base, "ffmpeg"), strings.Contains(base, "ffprobe"):
		return ToolFfmpeg
	case strings.HasPrefix(base, "uv"), strings.HasPrefix(base, "python"):
		return ToolTranscription
	case strings.Contains(base, "chrome"):
		return ToolChrome
	case strings.Contains(base, "codex"):
		return ToolCodex
	}
	return base
}
