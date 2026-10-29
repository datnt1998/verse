package exp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/noverse/internal/domain"
)

// Run 执行一次导出。同步返回，IO 量小（本地文件读写）。
//
// 失败语义：
//   - deps/opts 非法 → 配置错误立即返回
//   - 无任何已完成章节 → 返回错误（让调用方明确）
//   - 范围内某章 chapters/{ch}.md 缺失 → 返回错误（progress 与文件系统不一致是事实层 bug，应让用户看见）
//   - 输出路径已存在且未指定 Overwrite → 返回错误
//
// Skipped 用于"范围内合法但尚未完成"的情况（用户传 to=100 但只写到 80）。
func Run(ctx context.Context, deps Deps, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("xuất truyện: kho dữ liệu chưa sẵn sàng")
	}

	outPath, outIsDir, err := resolveOutPath(opts.OutPath)
	if err != nil {
		return nil, err
	}
	if opts.Format == "" {
		formatSource := outPath
		if outIsDir {
			formatSource = ""
		}
		f, err := inferFormat(formatSource)
		if err != nil {
			return nil, err
		}
		opts.Format = f
	}
	if opts.Format != FormatTXT && opts.Format != FormatEPUB && opts.Format != FormatNarration {
		return nil, fmt.Errorf("định dạng chưa hỗ trợ %q", opts.Format)
	}

	progress, err := deps.Store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc tiến độ thất bại: %w", err)
	}
	if progress == nil || len(progress.CompletedChapters) == 0 {
		return nil, fmt.Errorf("chưa có chương hoàn thành nào để xuất")
	}
	book, err := deps.Store.Book.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc thông tin truyện thất bại: %w", err)
	}
	if book == nil {
		return nil, fmt.Errorf("thiếu thông tin truyện, không thể xuất")
	}

	completed := make(map[int]struct{}, len(progress.CompletedChapters))
	maxCh := 0
	for _, c := range progress.CompletedChapters {
		completed[c] = struct{}{}
		if c > maxCh {
			maxCh = c
		}
	}

	from := opts.From
	if from <= 0 {
		from = 1
	}
	to := opts.To
	if to <= 0 {
		to = maxCh
	}
	if from > to {
		return nil, fmt.Errorf("khoảng chương không hợp lệ: from=%d > to=%d", from, to)
	}

	var chapters, skipped []int
	for ch := from; ch <= to; ch++ {
		if _, ok := completed[ch]; ok {
			chapters = append(chapters, ch)
		} else {
			skipped = append(skipped, ch)
		}
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("khoảng %d..%d không có chương hoàn thành", from, to)
	}

	bodies := make(map[int]string, len(chapters))
	for _, ch := range chapters {
		text, err := deps.Store.Drafts.LoadChapterText(ch)
		if err != nil {
			return nil, fmt.Errorf("đọc chương %d thất bại: %w", ch, err)
		}
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("tiến độ đánh dấu chương %d hoàn thành nhưng chapters/%02d.md thiếu hoặc rỗng", ch, ch)
		}
		bodies[ch] = text
	}

	outline, _ := deps.Store.Outline.LoadOutline()
	var volumes []domain.VolumeOutline
	if progress.Layered {
		volumes, _ = deps.Store.Outline.LoadLayeredOutline()
	}

	if outPath == "" || outIsDir {
		dir := outPath
		if dir == "" {
			dir = deps.Store.Dir()
		}
		outPath = filepath.Join(dir, sanitizeFileName(book.Title)+opts.Format.FileSuffix())
	}

	// 目标已存在时的冲突策略：Overwrite / "replace" 覆盖；"keep" 让位到
	// 带时间戳的变体；默认（含未知值）报错，避免静默覆盖用户文件。
	// DryRun 只为“比较卡片”估算新文件：不看冲突、不写盘。
	switch {
	case opts.DryRun:
	case opts.Overwrite || opts.OnExists == "replace":
	case opts.OnExists == "keep":
		if _, err := os.Stat(outPath); err == nil {
			variant, err := keepVariantPath(outPath, time.Now())
			if err != nil {
				return nil, err
			}
			outPath = variant
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("kiểm tra tệp đầu ra thất bại: %w", err)
		}
	case opts.OnExists != "" && opts.OnExists != "fail":
		return nil, fmt.Errorf("onExists không hợp lệ %q (chỉ \"replace\", \"keep\" hoặc \"fail\")", opts.OnExists)
	default:
		if _, err := os.Stat(outPath); err == nil {
			return nil, &ExistsError{Path: outPath}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("kiểm tra tệp đầu ra thất bại: %w", err)
		}
	}

	titleIdx := buildTitleIndex(outline)
	for i, ch := range chapters {
		summary, err := deps.Store.Summaries.LoadSummary(ch)
		if err != nil {
			return nil, fmt.Errorf("đọc tóm tắt chương %d thất bại: %w", ch, err)
		}
		if summary != nil && strings.TrimSpace(summary.Title) != "" {
			titleIdx[ch] = summary.Title
		}
		if opts.Progress != nil {
			opts.Progress(i+1, len(chapters), ch, strings.TrimSpace(titleIdx[ch]))
		}
	}
	var locations map[int]chapterLocation
	if len(volumes) > 0 {
		locations = buildLocations(volumes)
	}

	lbl := exportLabels
	var data []byte
	switch opts.Format {
	case FormatTXT:
		data = []byte(renderTXT(book.Title, chapters, titleIdx, locations, bodies, lbl))
	case FormatNarration:
		data = []byte(renderNarration(chapters, bodies, opts.ReadNumbers))
	case FormatEPUB:
		buf, err := renderEPUB(*book, chapters, titleIdx, locations, bodies, lbl)
		if err != nil {
			return nil, fmt.Errorf("dựng EPUB thất bại: %w", err)
		}
		data = buf
	}

	if !opts.DryRun {
		if err := atomicWrite(outPath, data); err != nil {
			return nil, fmt.Errorf("ghi tệp %q thất bại: %s", filepath.Base(outPath), writeFailureReason(err))
		}
	}

	return &Result{
		Path:     outPath,
		Chapters: len(chapters),
		Bytes:    len(data),
		Skipped:  skipped,
	}, nil
}

// resolveOutPath 展开开头的 "~"（TUI 参数不经 shell，不会自动展开），并判断目标是否
// 为目录：已存在的目录或以分隔符结尾的路径都视为"放进这个目录"，文件名取书名。
// 否则对目录做 rename 会失败（file exists），或把 "~" 当成字面目录名建出来。
func resolveOutPath(path string) (resolved string, isDir bool, err error) {
	if path == "" {
		return "", false, nil
	}
	isDir = strings.HasSuffix(path, "/") || strings.HasSuffix(path, string(filepath.Separator))
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("không mở rộng được ~: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		isDir = true
	}
	return path, isDir, nil
}

// inferFormat 从输出路径后缀推断格式。空路径回退 TXT；未知后缀报错（避免静默错误）。
// FormatNarration 不参与推断（同样落 .txt），只能由调用方显式指定。
func inferFormat(path string) (Format, error) {
	if path == "" {
		return FormatTXT, nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case "", ".txt":
		return FormatTXT, nil
	case ".epub":
		return FormatEPUB, nil
	default:
		return "", fmt.Errorf("không suy ra được định dạng từ đuôi %q (hỗ trợ .txt / .epub)", filepath.Ext(path))
	}
}

// atomicWrite 与 store/io.go 的 WriteFile 同形：tmp + sync + rename。
// 不复用 store.IO 是因为输出路径可能在 store.Dir() 之外。
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// writeFailureReason 把写盘失败归成越南语原因；os 错误原文带绝对路径与英文，只用于兜底分支的
// 底层 errno 文本（不含路径）。
func writeFailureReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrExist):
		return "đã có thư mục hoặc tệp trùng tên"
	case errors.Is(err, fs.ErrPermission):
		return "không có quyền ghi vào thư mục đích"
	case errors.Is(err, fs.ErrNotExist):
		return "không tìm thấy thư mục đích"
	case errors.Is(err, syscall.ENOSPC):
		return "ổ đĩa đã đầy"
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error()
	}
	return err.Error()
}

// sanitizeFileName 替换文件名里在大多数文件系统上不允许或易混淆的字符。
// 不做激进的转码，只挡住路径分隔符和控制字符。
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "novel"
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\x00", "_",
	)
	return replacer.Replace(name)
}
