// vidverse là CLI của Vidverse: serve chạy giao diện web (thiết lập lần đầu,
// đăng nhập, app shell).
package main

import (
	"fmt"
	"os"
)

const usage = `Cách dùng:
  vidverse serve [-addr HOST:PORT]   Chạy web UI (mặc định 127.0.0.1:8788)
  vidverse version                   In phiên bản

Biến môi trường:
  VIDVERSE_ADDR                      Địa chỉ lắng nghe (ghi đè bởi -addr)
  TRUSTED_PROXIES / PUBLIC_URL / SECURE_COOKIES   Ranh giới proxy (platform/webguard)
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "serve":
		if err := runServe(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Lỗi: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Println("vidverse dev")
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
	default:
		fmt.Fprintf(os.Stderr, "Lệnh không hợp lệ %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}
