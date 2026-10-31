package main

// `viderse overlay …`: thư viện lớp phủ dùng chung từ dòng lệnh — một dòng
// mỗi nội dung (sha256), hiệu ứng có sẵn được ghi ra thư mục dự án một lần.
// JSON ra stdout; lỗi JSON ra stderr + exit 1, cùng vỏ với `template`.
//
//   viderse overlay list              Danh sách lớp phủ (mới nhất trước)
//   viderse overlay add <tệp>         Thêm một video/ảnh (trùng byte → dùng lại)
//   viderse overlay remove <id>       Bỏ khỏi thư viện (template đang dùng vẫn giữ)

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

const overlayUsage = `Cách dùng:
  viderse overlay list            Danh sách lớp phủ (mới nhất trước; 12 hiệu ứng có sẵn)
  viderse overlay add <tệp>       Thêm video/ảnh vào thư viện (đo ffprobe, đoán nền)
  viderse overlay remove <id>     Bỏ khỏi thư viện (giữ tệp cho template đang dùng)
`

// runOverlayCommand xử lý `viderse overlay ...`; trả về exit code.
func runOverlayCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, overlayUsage)
		return 0
	}
	if len(args) == 0 {
		return libraryUsageError(stderr, "thiếu lệnh con (list|add|remove)", overlayUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	svc, closeDeps, err := openLibrary(ctx)
	if err != nil {
		return libraryErrOut(stderr, err)
	}
	defer closeDeps()

	switch command, rest := args[0], args[1:]; command {
	case "list":
		if len(rest) != 0 {
			return libraryUsageError(stderr, "sai cú pháp lệnh", overlayUsage)
		}
		if code := listOverlaysStdout(ctx, svc, stdout, stderr, ""); code != 0 {
			return code
		}
		return 0
	case "add":
		if len(rest) != 1 {
			return libraryUsageError(stderr, "thiếu đường dẫn tệp", overlayUsage)
		}
		overlay, _, err := svc.AddOverlayFile(ctx, rest[0], "")
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		usage, err := svc.OverlayUsage(ctx)
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		writeLibraryJSON(stdout, map[string]any{"overlay": web.OverlayToWire(*overlay, usage)})
		return 0
	case "remove":
		if len(rest) != 1 {
			return libraryUsageError(stderr, "thiếu id lớp phủ", overlayUsage)
		}
		if err := svc.RemoveOverlay(ctx, rest[0]); err != nil {
			return libraryErrOut(stderr, err)
		}
		return listOverlaysStdout(ctx, svc, stdout, stderr, rest[0])
	}
	return libraryUsageError(stderr, "lệnh con không hợp lệ", overlayUsage)
}

// listOverlaysStdout in {"overlays": [...], "removed"?: id}.
func listOverlaysStdout(ctx context.Context, svc *library.Service, stdout, stderr io.Writer, removed string) int {
	items, err := svc.ListOverlays(ctx)
	if err != nil {
		return libraryErrOut(stderr, err)
	}
	usage, err := svc.OverlayUsage(ctx)
	if err != nil {
		return libraryErrOut(stderr, err)
	}
	out := make([]web.OverlayWire, 0, len(items))
	for _, item := range items {
		out = append(out, web.OverlayToWire(item, usage))
	}
	body := map[string]any{"overlays": out}
	if removed != "" {
		body["removed"] = strings.TrimSpace(removed)
	}
	writeLibraryJSON(stdout, body)
	return 0
}
