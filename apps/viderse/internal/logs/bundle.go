package logs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// bundleTailBytes là phần đuôi tối đa mỗi log đưa vào gói — các dòng mới
// nhất giải thích vấn đề hiện tại (cùng hằng 512 KB của yt-factory).
const bundleTailBytes = 512 * 1024

// ExtraFile là một tệp chữ do caller cung cấp thêm vào gói báo lỗi
// (settings đã lọc, trạng thái việc nền, phiên bản công cụ…). Nội dung được
// che theo bảng mẫu như mọi tệp khác.
type ExtraFile func(ctx context.Context) (name string, data []byte, err error)

// Bundle là gói báo lỗi dạng zip: tóm tắt máy + đuôi các log công cụ đã che,
// cùng các tệp phụ caller tiêm (không bao giờ gồm DB, media, cookie, khoá).
type Bundle struct {
	FileName string
	Bytes    []byte
	Files    int
}

// BundleOptions tinh chỉnh gói: Version là phiên bản app, Extras các tệp phụ.
type BundleOptions struct {
	Version string
	Extras  []ExtraFile
	Now     time.Time
}

// BuildBundle dựng gói trong bộ nhớ — log đã giới hạn nên gói vài MB.
func (s *Store) BuildBundle(ctx context.Context, opts BundleOptions) (*Bundle, error) {
	if s.tasks != nil {
		id, err := s.tasks.Start(ctx, domain.TaskSpec{
			Kind: domain.TaskLogs, Title: "Xuất nhật ký",
			Object: domain.ObjectRef{Type:"logs", ID:fmt.Sprint(s.sequence.Add(1)), Label:"Nhật ký"},
			Phase:"Đang tạo gói báo lỗi",
		})
		if err == nil { defer s.tasks.Finish(context.WithoutCancel(ctx), id) }
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	summary := map[string]any{
		"generatedAt": now.UTC().Format(time.RFC3339Nano),
		"appVersion":  opts.Version,
		"machine": map[string]any{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
			"cpus": runtime.NumCPU(),
			"go":   runtime.Version(),
		},
	}
	if err := addJSON(add, "summary.json", summary); err != nil {
		return nil, err
	}

	files := 1
	// Đuôi mọi log công cụ, đã che theo bảng mẫu và đường nhà.
	for _, file := range listLogs(s.dir) {
		tail, err := tailBytes(file, bundleTailBytes)
		if err != nil {
			continue
		}
		if err := add("logs/"+filepath.Base(file), []byte(s.Redact(string(tail)))); err != nil {
			return nil, err
		}
		files++
	}
	for _, extra := range opts.Extras {
		name, data, err := extra(ctx)
		if err != nil || len(data) == 0 || name == "" {
			continue
		}
		if err := add(name, []byte(s.Redact(string(data)))); err != nil {
			return nil, err
		}
		files++
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	stamp := strings.ReplaceAll(now.UTC().Format("2006-01-02T15:04:05"), ":", "-")
	return &Bundle{
		FileName: fmt.Sprintf("viderse-logs-%s.zip", stamp),
		Bytes:    buf.Bytes(),
		Files:    files,
	}, nil
}

func addJSON(add func(string, []byte) error, name string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return add(name, raw)
}

// listLogs liệt kê tệp .log của store (sắp theo tên).
func listLogs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}
