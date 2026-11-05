package main

// `viderse channel …`: mảng Kênh của bạn từ dòng lệnh — liệt kê, xem, tạo,
// sửa, xoá, liên kết/bỏ liên kết đối thủ, đổi template mặc định và tạo nhận
// diện. JSON ra stdout, lỗi {error, code?, recovery?, retryAt?} ra stderr +
// exit 1 — cùng vỏ với HTTP.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

const channelUsage = `Cách dùng:
  viderse channel list
  viderse channel show <id>
  viderse channel create [--name TÊN] [--url LINK] [--rival ID …] [--per-day N]
                         [--language auto|en|vi] [--no-auto-fetch] [--auto-approve]
                         [--default-template ID]
  viderse channel update <id> [--name TÊN] [--url LINK] [--language L] [--style-note VĂN]
                         [--footer ĐOẠN] [--tag TAG …] [--auto-fetch=true|false]
                         [--auto-approve=true|false] [--per-day N] [--default-template ID]
                         [--brief MÔ TẢ]
  viderse channel delete <id>
  viderse channel link <id> <rivalId>
  viderse channel unlink <id> <rivalId>
  viderse channel template <id> <templateId>     Đổi template mặc định (PATCH)
  viderse channel identity <id> [--part all|name|avatar|banner] [--brief MÔ TẢ] [--direction GÓP_Ý]

Lệnh cần VIDERSE_DATABASE_URL. identity cần cấu hình Trợ lý AI đã lưu (Cài đặt).
`

// runChannelCommand xử lý `viderse channel ...`; trả exit code.
func runChannelCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, channelUsage)
		return 2
	}
	ctx := context.Background()
	switch args[0] {
	case "list":
		return runChannelList(ctx, stdout, stderr)
	case "show":
		return runChannelShow(ctx, args[1:], stdout, stderr)
	case "create":
		return runChannelCreate(ctx, args[1:], stdout, stderr)
	case "update":
		return runChannelUpdate(ctx, args[1:], stdout, stderr)
	case "delete":
		return runChannelDelete(ctx, args[1:], stdout, stderr)
	case "link":
		return runChannelLink(ctx, args[1:], stdout, stderr)
	case "unlink":
		return runChannelUnlink(ctx, args[1:], stdout, stderr)
	case "template":
		return runChannelTemplate(ctx, args[1:], stdout, stderr)
	case "instruction":
		return runChannelInstruction(ctx, args[1:], stdout, stderr)
	case "identity":
		return runChannelIdentity(ctx, args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, channelUsage)
		return 0
	default:
		return channelUsageErr(stderr, fmt.Sprintf("lệnh con không hợp lệ %q", args[0]))
	}
}

// withChannelService mở service cho một lệnh (đóng khi xong).
func withChannelService(ctx context.Context, needAI bool, fn func(*channels.Service) int, stderr io.Writer) int {
	svc, stop, err := openChannelService(ctx, needAI)
	if err != nil {
		return channelErrOut(stderr, err)
	}
	defer stop()
	return fn(svc)
}

func runChannelList(ctx context.Context, stdout, stderr io.Writer) int {
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		list, err := svc.List(ctx)
		if err != nil {
			return channelErrOut(stderr, err)
		}
		out := make([]any, 0, len(list))
		for _, v := range list {
			out = append(out, web.ChannelToWire(v))
		}
		writeChannelJSON(stdout, out)
		return 0
	}, stderr)
}

func runChannelShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return channelUsageErr(stderr, "cần đúng một id kênh")
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, err := svc.Get(ctx, args[0])
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, web.ChannelToWire(view))
		return 0
	}, stderr)
}

func runChannelCreate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("channel create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Tên kênh (để trống: nhận diện đặt tên)")
	link := fs.String("url", "", "Link YouTube của kênh")
	language := fs.String("language", "", "Ngôn ngữ: auto|en|vi")
	perDay := fs.Int("per-day", channels.PerDayDefault, "Nhịp video mỗi ngày (1–20)")
	noAutoFetch := fs.Bool("no-auto-fetch", false, "Tắt Tự lấy")
	autoApprove := fs.Bool("auto-approve", false, "Bật Tự duyệt ảnh (mọi ảnh ≥85)")
	defaultTemplate := fs.String("default-template", "", "ID template Thư viện làm mặc định (trống: tạo Mẫu chuẩn)")
	var rivals multiFlag
	fs.Var(&rivals, "rival", "ID đối thủ để liên kết (lặp lại được)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	autoFetch := !*noAutoFetch
	var defaultTemplateID *string
	if *defaultTemplate != "" {
		defaultTemplateID = defaultTemplate
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, jobID, err := svc.Create(ctx, channels.CreateRequest{
			Name:              *name,
			YoutubeURL:        optionalString(*link),
			CompetitorIDs:     rivals,
			AutoFetch:         &autoFetch,
			AutoApprove:       autoApprove,
			PerDay:            perDay,
			Language:          *language,
			DefaultTemplateID: defaultTemplateID,
		})
		if err != nil {
			return channelErrOut(stderr, err)
		}
		out := map[string]any{"channel": web.ChannelToWire(view)}
		if jobID != "" {
			out["identityJobId"] = jobID
		}
		writeChannelJSON(stdout, out)
		return 0
	}, stderr)
}

func runChannelUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return channelUsageErr(stderr, "cần id kênh trước các cờ")
	}
	id := args[0]
	fs := flag.NewFlagSet("channel update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Tên kênh")
	link := fs.String("url", "", "Link YouTube (chuỗi rỗng = xoá)")
	language := fs.String("language", "", "Ngôn ngữ: auto|en|vi")
	styleNote := fs.String("style-note", "", "Giọng văn (chuỗi rỗng = xoá)")
	footer := fs.String("footer", "", "Đoạn cuối mô tả (chuỗi rỗng = xoá)")
	perDay := fs.Int("per-day", 0, "Nhịp video mỗi ngày (1–20)")
	brief := fs.String("brief", "", "Mô tả kênh trong nhận diện")
	autoFetch := fs.String("auto-fetch", "", "Bật/tắt Tự lấy: true|false")
	autoApprove := fs.String("auto-approve", "", "Bật/tắt Tự duyệt: true|false")
	defaultTemplate := fs.String("default-template", "", "ID template làm mặc định")
	var tags multiFlag
	fs.Var(&tags, "tag", "Tag luôn thêm (lặp lại được; thay cả danh sách)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { visited[f.Name] = true })
	var in channels.UpdateRequest
	if visited["name"] {
		in.Name = name
	}
	if visited["url"] {
		v := link
		in.YoutubeURL = &v
	}
	if visited["language"] {
		in.Language = language
	}
	if visited["style-note"] {
		v := styleNote
		in.StyleNote = &v
	}
	if visited["footer"] {
		v := footer
		in.DescriptionFooter = &v
	}
	if visited["per-day"] {
		in.PerDay = perDay
	}
	if visited["brief"] {
		in.Brief = brief
	}
	if visited["auto-fetch"] {
		value := strings.EqualFold(*autoFetch, "true") || *autoFetch == "1"
		if !value && !strings.EqualFold(*autoFetch, "false") && *autoFetch != "0" {
			return channelFlagError(stderr, "auto-fetch phải là true hoặc false")
		}
		in.AutoFetch = &value
	}
	if visited["auto-approve"] {
		value := strings.EqualFold(*autoApprove, "true") || *autoApprove == "1"
		if !value && !strings.EqualFold(*autoApprove, "false") && *autoApprove != "0" {
			return channelFlagError(stderr, "auto-approve phải là true hoặc false")
		}
		in.AutoApprove = &value
	}
	if visited["default-template"] {
		v := defaultTemplate
		in.DefaultTemplateID = &v
	}
	if visited["tag"] {
		list := []string(tags)
		in.AlwaysTags = &list
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, err := svc.Update(ctx, id, in)
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, web.ChannelToWire(view))
		return 0
	}, stderr)
}

func runChannelDelete(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return channelUsageErr(stderr, "cần đúng một id kênh")
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		if err := svc.Delete(ctx, args[0]); err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, map[string]any{"deleted": true})
		return 0
	}, stderr)
}

func runChannelLink(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		return channelUsageErr(stderr, "cần id kênh và id đối thủ")
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, err := svc.Link(ctx, args[0], args[1])
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, web.ChannelToWire(view))
		return 0
	}, stderr)
}

func runChannelUnlink(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		return channelUsageErr(stderr, "cần id kênh và id đối thủ")
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, paused, err := svc.Unlink(ctx, args[0], args[1])
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, map[string]any{
			"channel":         web.ChannelToWire(view),
			"autoFetchPaused": paused,
		})
		return 0
	}, stderr)
}

// runChannelTemplate đổi template mặc định của kênh: `channel template <id>
// <templateId>` (thay PATCH defaultTemplateId). Xoá/đổi rotation cũ không còn.
func runChannelTemplate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		return channelUsageErr(stderr, "dùng: viderse channel template <idKênh> <templateId>")
	}
	var v *string = &args[1]
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, err := svc.Update(ctx, args[0], channels.UpdateRequest{DefaultTemplateID: &v})
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, web.ChannelToWire(view))
		return 0
	}, stderr)
}

// runChannelInstruction dặn trợ lý về kênh: `channel instruction <idKênh>
// <trợ lý> [chỉ dẫn]`; bỏ chỉ dẫn = xoá chỉ dẫn của trợ lý đó.
func runChannelInstruction(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || len(args) > 3 {
		return channelUsageErr(stderr, "dùng: viderse channel instruction <idKênh> <trợ lý> [chỉ dẫn]")
	}
	agent := args[1]
	text := ""
	if len(args) == 3 {
		text = args[2]
	}
	return withChannelService(ctx, false, func(svc *channels.Service) int {
		view, err := svc.Update(ctx, args[0], channels.UpdateRequest{AgentInstructions: map[string]string{agent: text}})
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, web.ChannelToWire(view))
		return 0
	}, stderr)
}

func runChannelIdentity(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return channelUsageErr(stderr, "cần id kênh trước các cờ")
	}
	id := args[0]
	fs := flag.NewFlagSet("channel identity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	part := fs.String("part", "all", "Phần tạo: all|name|avatar|banner")
	brief := fs.String("brief", "", "Mô tả kênh (rỗng: model viết từ đối thủ)")
	direction := fs.String("direction", "", "Góp ý hướng hình ảnh")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	return withChannelService(ctx, true, func(svc *channels.Service) int {
		jobID, err := svc.RunIdentity(ctx, id, channels.IdentityInput{
			Part: *part, Brief: *brief, Direction: *direction,
		})
		if err != nil {
			return channelErrOut(stderr, err)
		}
		writeChannelJSON(stdout, map[string]any{"identityJobId": jobID, "done": true})
		return 0
	}, stderr)
}

// ── cờ, JSON, lỗi ──────────────────────────────────────────────────────────

// multiFlag là cờ lặp lại được (--rival a --rival b).
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, strings.TrimSpace(v))
	return nil
}

func optionalString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func writeChannelJSON(stdout io.Writer, v any) { writeRivalJSON(stdout, v) }

// channelErrOut ghi vỏ lỗi JSON {error, code?, recovery?, retryAt?} ra stderr,
// exit 1.
func channelErrOut(stderr io.Writer, err error) int {
	body := map[string]any{"error": err.Error()}
	var ce *channels.Error
	if errors.As(err, &ce) {
		body["error"] = ce.Message
		body["code"] = ce.Code
		if ce.Recovery != "" {
			body["recovery"] = ce.Recovery
		}
		if ce.RetryAt != nil {
			body["retryAt"] = ce.RetryAt.UTC().Format(time.RFC3339)
		}
	}
	encoded, _ := json.MarshalIndent(body, "", "  ")
	fmt.Fprintln(stderr, string(encoded))
	return 1
}

// channelFlagError ghi lỗi tham số (vỏ invalid_input) ra stderr, exit 1.
func channelFlagError(stderr io.Writer, msg string) int {
	return channelErrOut(stderr, &channels.Error{Code: channels.CodeInvalidInput, Message: msg})
}

// channelUsageErr ghi lỗi thiếu/sai lệnh con, kèm usage trong recovery.
func channelUsageErr(stderr io.Writer, msg string) int {
	return channelErrOut(stderr, &channels.Error{Code: channels.CodeInvalidInput, Message: msg,
		Recovery: "Xem `viderse channel --help`."})
}
