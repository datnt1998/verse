package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
)

type exportDoneMsg struct {
	result *exp.Result
	err    error
}

func startExport(rt *host.Host, args []string) (tea.Cmd, error) {
	opts, err := parseExportArgs(args)
	if err != nil {
		return nil, err
	}
	return func() tea.Msg {
		res, err := rt.Export(context.Background(), opts)
		return exportDoneMsg{result: res, err: err}
	}, nil
}

func parseExportArgs(args []string) (exp.Options, error) {
	var opts exp.Options
	for _, a := range args {
		if a == "--overwrite" || a == "--keep" {
			mode := "replace"
			if a == "--keep" {
				mode = "keep"
			}
			if opts.OnExists != "" && opts.OnExists != mode {
				return exp.Options{}, fmt.Errorf("chỉ dùng một trong --overwrite hoặc --keep")
			}
			opts.OnExists = mode
			continue
		}
		if a == "doc-so" || a == "--doc-so" {
			opts.ReadNumbers = true
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok {
			switch strings.ToLower(k) {
			case "from":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return exp.Options{}, fmt.Errorf("from phải là số nguyên không âm: %q", v)
				}
				opts.From = n
			case "to":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return exp.Options{}, fmt.Errorf("to phải là số nguyên không âm: %q", v)
				}
				opts.To = n
			case "format":
				opts.Format = exp.Format(strings.ToLower(v))
			default:
				return exp.Options{}, fmt.Errorf("tham số không hợp lệ %q (hỗ trợ: from / to / format)", k)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return exp.Options{}, fmt.Errorf("cờ không hợp lệ %q", a)
		}
		// 位置参数是格式名时按格式处理，否则当输出路径。
		switch exp.Format(strings.ToLower(a)) {
		case exp.FormatTXT, exp.FormatEPUB, exp.FormatNarration:
			if opts.Format != "" && opts.Format != exp.Format(strings.ToLower(a)) {
				return exp.Options{}, fmt.Errorf("định dạng đã chọn %q, không thể thêm %q", opts.Format, a)
			}
			opts.Format = exp.Format(strings.ToLower(a))
			continue
		}
		if opts.OutPath != "" {
			return exp.Options{}, fmt.Errorf("chỉ hỗ trợ một tham số đường dẫn: %q", a)
		}
		opts.OutPath = a
	}
	return opts, nil
}

func formatExportSuccess(res *exp.Result) string {
	var b strings.Builder
	wd, _ := os.Getwd()
	fmt.Fprintf(&b, "✓ Đã xuất %d chương / %s đến %s", res.Chapters, humanBytes(res.Bytes), host.RootRelativeText(res.Path, wd))
	if n := len(res.Skipped); n > 0 {
		fmt.Fprintf(&b, " (bỏ qua %d chương chưa hoàn thành: %s)", n, briefIntList(res.Skipped, 5))
	}
	return b.String()
}

// formatExportFailure 与 formatExportSuccess 同口径：路径相对故事目录（工作目录），不显示绝对路径。
func formatExportFailure(err error) string {
	wd, _ := os.Getwd()
	return "Xuất file thất bại: " + host.RootRelativeText(err.Error(), wd)
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func briefIntList(xs []int, max int) string {
	if len(xs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(xs))
	for i, x := range xs {
		if i >= max {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, strconv.Itoa(x))
	}
	return strings.Join(parts, ",")
}
