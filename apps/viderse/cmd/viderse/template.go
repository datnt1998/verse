package main

// `viderse template …`: mảng Thư viện › Template từ dòng lệnh — cùng đường
// kiểm recipe với API, cùng hình JSON với web. JSON ra stdout; lỗi JSON
// {error, code?, recovery?, fields?} ra stderr + exit 1 như mọi nhóm lệnh.
//
//   viderse template list                              Danh sách template (thẻ Thư viện)
//   viderse template show <id>                         Chi tiết: recipe, revision, giá, kênh
//   viderse template create <tên>                      Template mới với Mẫu chuẩn
//   viderse template duplicate <id> [tên]              Nhân bản
//   viderse template rename <id> <tên> [--revision N]  Đổi tên
//   viderse template set <id> (--recipe-file f | --recipe json | --name tên) --revision N
//                                                     Lưu recipe/tên với revision hiện tại
//   viderse template delete <id>                       Xoá (chặn khi kênh còn dùng)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

const templateUsage = `Cách dùng:
  viderse template list                                  Danh sách template
  viderse template show <id>                             Chi tiết template
  viderse template create <tên>                          Tạo template (Mẫu chuẩn)
  viderse template duplicate <id> [tên]                  Nhân bản template
  viderse template rename <id> <tên> [--revision N]      Đổi tên
  viderse template set <id> --revision N '<json>'
  viderse template set <id> --revision N --recipe-file <f>
  viderse template set <id> --revision N --recipe '<json>'
  viderse template set <id> --revision N --name '<tên>'
                                                         Lưu template (revision lệch → 409)
  viderse template import <tệp|thư mục> [--name <tên>] [--only <chữ>]…
                                                         Nhập template cũ (.ytf-template.zip,
                                                         thư mục chứa các tệp đó, hoặc thư mục
                                                         editor-workspace cũ); --only lọc theo tên
  viderse template import --from-data <thư mục dữ liệu> [--only <chữ>]…
                                                         Nhập thẳng template từ thư mục dữ liệu của
                                                         bản cũ (app.db + editor-workspace), đọc trên
                                                         bản sao, không cần chạy công cụ trước đây
  viderse template delete <id>                           Xoá template
`

// runTemplateCommand xử lý `viderse template ...`; trả về exit code.
func runTemplateCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, templateUsage)
		return 0
	}
	if len(args) == 0 {
		return libraryUsageError(stderr, "thiếu lệnh con (list|show|create|duplicate|rename|set|delete)", templateUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	svc, closeDeps, err := openLibrary(ctx)
	if err != nil {
		return libraryErrOut(stderr, err)
	}
	defer closeDeps()

	command, rest := args[0], args[1:]
	switch command {
	case "list":
		if len(rest) != 0 {
			return libraryUsageError(stderr, "sai cú pháp lệnh", templateUsage)
		}
		items, err := svc.List(ctx)
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		out := make([]web.TemplateSummaryWire, 0, len(items))
		for _, item := range items {
			out = append(out, web.TemplateSummaryToWire(item))
		}
		writeLibraryJSON(stdout, map[string]any{"templates": out})
		return 0
	case "show":
		if len(rest) != 1 {
			return libraryUsageError(stderr, "thiếu id template", templateUsage)
		}
		detail, err := svc.Get(ctx, rest[0])
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		writeLibraryJSON(stdout, web.TemplateDetailToWire(detail))
		return 0
	case "create":
		name := strings.TrimSpace(strings.Join(rest, " "))
		if name == "" {
			return libraryUsageError(stderr, "thiếu tên template", templateUsage)
		}
		tpl, err := svc.Create(ctx, name)
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		return outputTemplate(ctx, svc, tpl.ID, map[string]any{"templateId": tpl.ID}, stdout, stderr)
	case "duplicate":
		if len(rest) == 0 {
			return libraryUsageError(stderr, "thiếu id template để nhân bản", templateUsage)
		}
		name := strings.TrimSpace(strings.Join(rest[1:], " "))
		tpl, err := svc.Duplicate(ctx, rest[0], name)
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		return outputTemplate(ctx, svc, tpl.ID, map[string]any{"templateId": tpl.ID}, stdout, stderr)
	case "rename":
		if len(rest) < 2 {
			return libraryUsageError(stderr, "thiếu id hoặc tên mới", templateUsage)
		}
		flags, positional, code := parseTemplateFlags(rest[0], rest[1:], stderr)
		if code != 0 {
			return code
		}
		name := strings.TrimSpace(strings.Join(positional, " "))
		if name == "" {
			return libraryUsageError(stderr, "thiếu tên mới", templateUsage)
		}
		tpl, err := svc.Rename(ctx, rest[0], name, flags.revision())
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		return outputTemplate(ctx, svc, tpl.ID, nil, stdout, stderr)
	case "set":
		if len(rest) == 0 {
			return libraryUsageError(stderr, "thiếu id template", templateUsage)
		}
		flags, positional, code := parseTemplateFlags(rest[0], rest[1:], stderr)
		if code != 0 {
			return code
		}
		// Bản vá JSON có thể đứng sau id như một tham số vị trí (kiểu
		// `template set <id> '{"scenes":…}' --revision N`) hoặc qua cờ.
		if flags.recipe == nil && len(positional) > 0 {
			joined := strings.TrimSpace(strings.Join(positional, " "))
			if strings.HasPrefix(joined, "{") {
				flags.recipe = []byte(joined)
			}
		}
		revision, ok := flags.revisionValue()
		if !ok {
			return libraryFlagError(stderr, "thiếu --revision (revision hiện tại của template)")
		}
		patch := library.Patch{Revision: revision, Name: flags.name}
		if flags.recipe != nil {
			patch.Recipe = flags.recipe
		}
		if patch.Name == nil && patch.Recipe == nil {
			return libraryFlagError(stderr, "không có gì để lưu (dùng --recipe-file, --recipe hoặc --name)")
		}
		tpl, err := svc.Patch(ctx, rest[0], patch)
		if err != nil {
			return libraryErrOut(stderr, err)
		}
		return outputTemplate(ctx, svc, tpl.ID, nil, stdout, stderr)
	case "import":
		flags, positional, code := parseImportFlags(rest, stderr)
		if code != 0 {
			return code
		}
		var results []library.ImportResult
		if flags.fromData != "" {
			if len(positional) != 0 {
				return libraryUsageError(stderr, "--from-data không đi cùng đường dẫn", templateUsage)
			}
			items, err := library.OpenTemplateDataDir(flags.fromData)
			if err != nil {
				return libraryErrOut(stderr, err)
			}
			items = filterDataDirItems(items, flags.only)
			results = svc.ImportBundles(ctx, items, library.ImportOptions{Name: flags.name})
		} else {
			if len(positional) != 1 {
				return libraryUsageError(stderr, "thiếu tệp hoặc thư mục template", templateUsage)
			}
			targets, err := templateImportTargets(positional[0], flags.only)
			if err != nil {
				return libraryErrOut(stderr, err)
			}
			imported, err := svc.ImportTemplateFiles(ctx, targets, library.ImportOptions{Name: flags.name})
			if err != nil {
				return libraryErrOut(stderr, err)
			}
			results = imported
		}
		items, imported, failed := web.TemplateImportToWire(results)
		writeLibraryJSON(stdout, map[string]any{"items": items, "imported": imported, "failed": failed})
		if failed > 0 {
			return 1
		}
		return 0
	case "delete":
		if len(rest) != 1 {
			return libraryUsageError(stderr, "thiếu id template", templateUsage)
		}
		if err := svc.DeleteTemplate(ctx, rest[0]); err != nil {
			return libraryErrOut(stderr, err)
		}
		writeLibraryJSON(stdout, map[string]any{"removed": rest[0]})
		return 0
	}
	return libraryUsageError(stderr, "lệnh con không hợp lệ", templateUsage)
}

// outputTemplate đọc lại chi tiết rồi in cùng hình với HTTP.
func outputTemplate(ctx context.Context, svc *library.Service, id string, extra map[string]any, stdout, stderr io.Writer) int {
	detail, err := svc.Get(ctx, id)
	if err != nil {
		return libraryErrOut(stderr, err)
	}
	body := map[string]any{"template": web.TemplateDetailToWire(detail)}
	for key, value := range extra {
		body[key] = value
	}
	writeLibraryJSON(stdout, body)
	return 0
}

// templateFlags là cờ của rename/set.
type templateFlags struct {
	revisionSet bool
	revisionVal int64
	name        *string
	recipe      []byte
}

// revision trả con trỏ revision khi có cờ.
func (f templateFlags) revision() *int64 {
	if !f.revisionSet {
		return nil
	}
	value := f.revisionVal
	return &value
}

// revisionValue trả revision bắt buộc của `set`.
func (f templateFlags) revisionValue() (int64, bool) {
	return f.revisionVal, f.revisionSet
}

// parseTemplateFlags đọc --revision/--name/--recipe/--recipe-file sau id.
func parseTemplateFlags(_ string, args []string, stderr io.Writer) (templateFlags, []string, int) {
	var flags templateFlags
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() (string, bool) {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				return arg[eq+1:], true
			}
			if i+1 >= len(args) {
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case arg == "--revision" || strings.HasPrefix(arg, "--revision="):
			raw, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "--revision cần một số")
			}
			parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil || parsed <= 0 {
				return flags, nil, libraryFlagError(stderr, "--revision phải là số nguyên dương")
			}
			flags.revisionSet, flags.revisionVal = true, parsed
		case arg == "--name" || strings.HasPrefix(arg, "--name="):
			raw, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "--name cần một giá trị")
			}
			flags.name = &raw
		case arg == "--recipe" || strings.HasPrefix(arg, "--recipe="):
			raw, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "--recipe cần JSON")
			}
			flags.recipe = []byte(raw)
		case arg == "--recipe-file" || strings.HasPrefix(arg, "--recipe-file="):
			raw, ok := value()
			if !ok {
				return flags, nil, libraryFlagError(stderr, "--recipe-file cần đường dẫn")
			}
			data, err := os.ReadFile(raw)
			if err != nil {
				return flags, nil, libraryFlagError(stderr, "không đọc được tệp recipe: "+err.Error())
			}
			flags.recipe = data
		case strings.HasPrefix(arg, "--"):
			return flags, nil, libraryFlagError(stderr, "cờ không hợp lệ: "+arg)
		default:
			positional = append(positional, arg)
		}
	}
	return flags, positional, 0
}

// ── vỏ lỗi/JSON dùng chung cho template + overlay ──────────────────────────

// libraryErrOut ghi vỏ lỗi JSON ra stderr, exit 1.
func libraryErrOut(stderr io.Writer, err error) int {
	body := map[string]any{"error": err.Error()}
	var apiErr *library.Error
	if errors.As(err, &apiErr) {
		body["error"] = apiErr.Message
		body["code"] = apiErr.Code
		if apiErr.Recovery != "" {
			body["recovery"] = apiErr.Recovery
		}
		if len(apiErr.Fields) > 0 {
			body["fields"] = apiErr.Fields
		}
		if len(apiErr.Channels) > 0 {
			body["channels"] = apiErr.Channels
		}
		if len(apiErr.UsedBy) > 0 {
			body["usedBy"] = apiErr.UsedBy
		}
	}
	raw, _ := json.Marshal(body)
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// libraryFlagError ghi lỗi tham số (invalid_input) ra stderr, exit 1.
func libraryFlagError(stderr io.Writer, msg string) int {
	raw, _ := json.Marshal(map[string]any{"error": msg, "code": library.CodeInvalidInput})
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// libraryUsageError ghi lỗi thiếu/sai lệnh con kèm usage trong recovery.
func libraryUsageError(stderr io.Writer, msg, usage string) int {
	raw, _ := json.Marshal(map[string]any{
		"error": msg, "code": library.CodeInvalidInput,
		"recovery": strings.TrimRight(usage, "\n"),
	})
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// writeLibraryJSON ghi một giá trị JSON ra stdout.
func writeLibraryJSON(stdout io.Writer, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(stdout, string(raw))
}
