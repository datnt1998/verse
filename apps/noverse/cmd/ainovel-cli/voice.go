package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	// Đăng ký engine có sẵn + nguồn cấu hình tts từ config.json.
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/bootcfg"
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/openaispeech"
	_ "github.com/datnt1998/verse/apps/noverse/internal/voice/vieneu"
)

// runVoiceCommand xử lý `ainovel-cli voice …`: cài bộ đọc trên máy, xem trạng thái,
// liệt kê giọng, và đọc thử ra tệp WAV. Đây là công cụ của máy (không phụ thuộc
// truyện đang mở), dùng cùng internal/voice với gateway và TUI.
func runVoiceCommand(args []string, out, errw io.Writer) int {
	if len(args) == 0 {
		printVoiceUsage(out)
		return 2
	}
	switch args[0] {
	case "install":
		return voiceInstallCommand(args[1:], out, errw)
	case "status":
		return voiceStatusCommand(args[1:], out, errw)
	case "voices":
		return voiceVoicesCommand(args[1:], out, errw)
	case "say":
		return voiceSayCommand(args[1:], out, errw)
	case "help", "-h", "--help":
		printVoiceUsage(out)
		return 0
	default:
		fmt.Fprintf(errw, "Lệnh voice không hợp lệ: %q\n\n", args[0])
		printVoiceUsage(errw)
		return 2
	}
}

func printVoiceUsage(w io.Writer) {
	fmt.Fprint(w, `Dùng: ainovel-cli voice <lệnh>

  install            Cài bộ đọc trên máy (VieNeu-TTS, chạy cục bộ)
  status             Xem trạng thái bộ đọc (đã cài chưa, đang chạy, tốc độ tạo)
  voices             Liệt kê giọng đọc có sẵn
  say "<chữ>"        Đọc thử ra tệp WAV
        --voice <tên>     giọng đọc (mặc định: giọng đầu tiên của máy đọc)
        -o <tệp>          tệp WAV đầu ra (mặc định: giong-doc-<thời điểm>.wav)
        --quality <q>     fast | standard | high (mặc định theo cấu hình)
        --speed <n>       tốc độ đọc, 0.5–2.0 (mặc định 1)
        --ref <tệp.wav>   đọc bằng clip mẫu thay cho giọng có sẵn
`)
}

// voiceService dựng Service theo cấu hình toàn cục (khoá tts trong config.json).
func voiceService() *voice.Service {
	return voice.New(voice.Shared().Config())
}

func voiceInstallCommand(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("voice install", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	svc := voiceService()
	defer svc.Close()

	if svc.Installed() {
		fmt.Fprintln(out, "Bộ đọc trên máy đã được cài. Dùng `ainovel-cli voice status` để xem trạng thái.")
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan voice.InstallProgress, 1)
	unsub := svc.SubscribeInstall(func(p voice.InstallProgress) {
		if p.State == "done" || p.State == "error" {
			select {
			case done <- p:
			default:
			}
		}
	})
	defer unsub()

	if err := svc.StartInstall(ctx); err != nil {
		fmt.Fprintf(errw, "Không bắt đầu cài được: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, "Đang cài bộ đọc trên máy (VieNeu-TTS). Lần đầu cần tải khoảng 600 MB…")

	last := ""
	for {
		select {
		case p := <-done:
			if p.State == "error" {
				fmt.Fprintf(errw, "\nCài bộ đọc thất bại: %s\n", p.Error)
				return 1
			}
			fmt.Fprintf(out, "\n%s\n", p.Message)
			return 0
		case <-time.After(400 * time.Millisecond):
			p := svc.InstallState()
			line := installLine(p)
			if line != last {
				fmt.Fprintln(out, line)
				last = line
			}
		case <-ctx.Done():
			fmt.Fprintln(errw, "\nĐã dừng cài bộ đọc.")
			return 1
		}
	}
}

func installLine(p voice.InstallProgress) string {
	msg := p.Message
	if p.BytesDone > 0 {
		msg = fmt.Sprintf("%s (%.0f MB)", msg, float64(p.BytesDone)/(1<<20))
	}
	if p.Percent == nil {
		return fmt.Sprintf("[ ---] %s", msg)
	}
	return fmt.Sprintf("[%3d%%] %s", *p.Percent, msg)
}

func voiceStatusCommand(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("voice status", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	svc := voiceService()
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st := svc.Status(ctx)
	caps := svc.Capabilities()

	fmt.Fprintf(out, "Máy đọc:      %s\n", st.Engine)
	fmt.Fprintf(out, "Trạng thái:   %s\n", stateLabel(st.State))
	if st.Model != "" {
		fmt.Fprintf(out, "Mô hình:      %s\n", st.Model)
	}
	fmt.Fprintf(out, "Thiết bị:     %s\n", st.Device)
	fmt.Fprintf(out, "Chất lượng:   %s\n", svc.Config().Quality)
	fmt.Fprintf(out, "Đã cài:       %s\n", yesNo(svc.Installed()))
	fmt.Fprintf(out, "Tần số:       %d Hz\n", caps.SampleRate)
	fmt.Fprintf(out, "Clone giọng:  %s\n", yesNo(caps.Cloning))
	if rtf := st.RTF; rtf != nil {
		fmt.Fprintf(out, "Tốc độ tạo:   1 phút audio ≈ %.0f giây\n", *rtf*60)
	}
	if st.Error != "" {
		fmt.Fprintf(out, "Lỗi:          %s\n", st.Error)
	}
	if p := svc.InstallState(); p.State == "running" {
		fmt.Fprintf(out, "Đang cài:     %s\n", installLine(p))
	}
	fmt.Fprintf(out, "Thư mục:      %s\n", filepath.Join(svc.Config().Home, "vieneu"))
	return 0
}

func voiceVoicesCommand(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("voice voices", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	svc := voiceService()
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	voices, err := svc.Voices(ctx)
	if err != nil {
		fmt.Fprintf(errw, "Không lấy được danh sách giọng: %v\n", err)
		return 1
	}
	if len(voices) == 0 {
		fmt.Fprintln(out, "Máy đọc chưa có giọng nào.")
		return 0
	}
	sort.SliceStable(voices, func(i, j int) bool { return voices[i].Name < voices[j].Name })
	fmt.Fprintf(out, "%-24s %-8s %-8s %s\n", "TÊN", "GIỚI TÍNH", "NGUỒN", "MÃ")
	for _, v := range voices {
		fmt.Fprintf(out, "%-24s %-8s %-8s %s\n", v.Name, genderLabel(v.Gender), sourceLabel(v.Source), v.ID)
	}
	return 0
}

func voiceSayCommand(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("voice say", flag.ContinueOnError)
	fs.SetOutput(errw)
	var (
		voiceID  = fs.String("voice", "", "giọng đọc có sẵn")
		refAudio = fs.String("ref", "", "clip mẫu để đọc theo giọng đó")
		output   = fs.String("o", "", "tệp WAV đầu ra")
		quality  = fs.String("quality", "", "fast|standard|high")
		speed    = fs.Float64("speed", 1, "tốc độ đọc 0.5–2.0")
	)
	// flag của Go dừng ở đối số đầu tiên, mà người dùng hay viết `say "chữ" --voice X`;
	// nên tách cờ ra trước rồi mới parse.
	flags, positional := splitFlags(args)
	if err := fs.Parse(flags); err != nil {
		return 2
	}
	text := strings.TrimSpace(strings.Join(positional, " "))
	if text == "" {
		fmt.Fprintln(errw, "Thiếu chữ cần đọc. Ví dụ: ainovel-cli voice say \"Xin chào\"")
		return 2
	}
	path := *output
	if path == "" {
		path = fmt.Sprintf("giong-doc-%s.wav", time.Now().Format("20060102-150405"))
	}

	svc := voiceService()
	defer svc.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(out, "Đang đọc %d ký tự…\n", len([]rune(text)))
	start := time.Now()
	audio, err := svc.Preview(ctx, voice.SynthRequest{
		Text:     text,
		VoiceID:  *voiceID,
		RefAudio: *refAudio,
		Quality:  *quality,
		Speed:    *speed,
	})
	if err != nil {
		if errors.Is(err, voice.ErrNotInstalled) {
			fmt.Fprintln(errw, "Bộ đọc trên máy chưa được cài. Chạy: ainovel-cli voice install")
			return 1
		}
		fmt.Fprintf(errw, "Đọc thất bại: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, audio.WAV, 0o644); err != nil {
		fmt.Fprintf(errw, "Không ghi được tệp: %v\n", err)
		return 1
	}
	abs, _ := filepath.Abs(path)
	fmt.Fprintf(out, "Đã ghi %s · %d Hz · %.1f giây audio · tạo trong %.1f giây\n",
		abs, audio.SampleRate, audio.DurationSec, time.Since(start).Seconds())
	return 0
}

// splitFlags tách cờ và đối số vị trí để cờ đứng sau chữ vẫn hiểu được.
// Mọi cờ của lệnh voice đều nhận giá trị nên cặp (cờ, giá trị) đi liền nhau.
func splitFlags(args []string) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	return flags, positional
}

func stateLabel(state string) string {
	switch state {
	case voice.StateMissing:
		return "chưa cài bộ đọc trên máy"
	case voice.StateInstalling:
		return "đang cài"
	case voice.StateStopped:
		return "đã cài, đang tắt (tự bật khi cần)"
	case voice.StateStarting:
		return "đang khởi động"
	case voice.StateReady:
		return "sẵn sàng"
	case voice.StateError:
		return "lỗi"
	}
	return state
}

func genderLabel(g string) string {
	switch strings.ToLower(g) {
	case "male", "nam":
		return "nam"
	case "female", "nữ", "nu":
		return "nữ"
	}
	if g == "" {
		return "—"
	}
	return g
}

func sourceLabel(s string) string {
	switch s {
	case voice.SourceBuiltin:
		return "có sẵn"
	case voice.SourceClone:
		return "clone"
	case voice.SourceProject:
		return "dự án"
	}
	return s
}

func yesNo(v bool) string {
	if v {
		return "có"
	}
	return "không"
}
