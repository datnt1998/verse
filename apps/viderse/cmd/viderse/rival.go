package main

// `viderse rival …`: mảng đối thủ từ dòng lệnh — theo dõi kênh, xem danh
// sách/trang video/khung đăng và điều khiển quét. JSON ra stdout, lỗi JSON
// {error, code?, recovery?} ra stderr + exit 1 — cùng vỏ với HTTP.

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
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
)

const rivalUsage = `Cách dùng:
  viderse rival add <@handle|url|UC…>           Theo dõi kênh đối thủ
  viderse rival list                            Danh sách đối thủ + tổng hợp
  viderse rival scan <id> [--history] [--cap N] [--action pause|resume|cancel]
                                              Điều khiển quét (mặc định: kiểm tra video mới)
  viderse rival videos <id|all> [--sort outlier|newest|views] [--long]
                                [--within 0|7|30|90] [--q text] [--limit N] [--cursor c]
                                              Trang video đối thủ
  viderse rival posting-slots <id> [--zone vn|ny]
                                              Khung đăng lặp lại của kênh
  viderse rival remove <id>                     Bỏ theo dõi (xoá video + trạng thái quét)
`

// runRivalCommand xử lý `viderse rival ...`; trả về exit code.
func runRivalCommand(args []string, stdout, stderr io.Writer) int {
	// Trợ giúp tường minh là văn bản người đọc, exit 0; thiếu tham số là lỗi
	// máy đọc (JSON + exit 1) như mọi lỗi khác của nhóm lệnh.
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, rivalUsage)
		return 0
	}
	if len(args) == 0 {
		return rivalUsageError(stderr, "thiếu lệnh con (add|list|scan|videos|posting-slots|remove)")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return rivalUsageError(stderr, "sai cú pháp lệnh")
		}
		readSvc, closeDeps, err := openRivalReads()
		if err != nil {
			return rivalErrOut(stderr, err)
		}
		defer closeDeps()
		list, err := readSvc.List(ctx)
		if err != nil {
			return rivalErrOut(stderr, err)
		}
		rivalsWire := make([]any, 0, len(list.Rivals))
		for _, e := range list.Rivals {
			rivalsWire = append(rivalsWire, web.RivalToWire(e))
		}
		writeRivalJSON(stdout, map[string]any{
			"rivals":      rivalsWire,
			"totalVideos": list.TotalVideos,
			"newOutliers": list.NewOutliers,
		})
		return 0

	case "add":
		if len(args) != 2 {
			return rivalUsageError(stderr, "sai cú pháp lệnh")
		}
		mutations, reads, closeDeps, err := openRivalMutations()
		if err != nil {
			return rivalErrOut(stderr, err)
		}
		defer closeDeps()
		return runRivalAdd(ctx, args[1], mutations, reads, stdout, stderr)

	case "remove":
		if len(args) != 2 {
			return rivalUsageError(stderr, "sai cú pháp lệnh")
		}
		mutations, _, closeDeps, err := openRivalMutations()
		if err != nil {
			return rivalErrOut(stderr, err)
		}
		defer closeDeps()
		if err := mutations.Remove(ctx, args[1]); err != nil {
			return rivalErrOut(stderr, err)
		}
		writeRivalJSON(stdout, map[string]any{"removed": args[1]})
		return 0

	case "scan":
		return runRivalScan(ctx, args[1:], stdout, stderr)

	case "videos":
		return runRivalVideos(ctx, args[1:], stdout, stderr)

	case "posting-slots":
		return runRivalPostingSlots(ctx, args[1:], stdout, stderr)
	}
	return rivalUsageError(stderr, "sai cú pháp lệnh")
}

// runRivalScan xử lý `viderse rival scan <id> [flags]`: không cờ = kiểm tra
// video mới; --history = start; --cap đặt giới hạn; --action pause|resume|
// cancel. Khi không có serve nào giữ lease quét, CLI chạy worker ngay trong
// tiến trình cho tới khi kênh hết việc (RunInline).
func runRivalScan(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return rivalUsageError(stderr, "sai cú pháp lệnh")
	}
	id := args[0]
	var (
		history bool
		capSet  bool
		capVal  int
		action  domain.ScanAction
	)
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--history":
			history = true
		case "--cap":
			i++
			if i >= len(rest) {
				return rivalFlagError(stderr, "--cap cần một số")
			}
			v, err := strconv.Atoi(rest[i])
			if err != nil || v < domain.HistoryCapMin || v > domain.HistoryCapMax {
				return rivalFlagError(stderr, "--cap phải là số nguyên từ 150 tới 10000")
			}
			capSet, capVal = true, v
		case "--action":
			i++
			if i >= len(rest) {
				return rivalFlagError(stderr, "--action cần pause|resume|cancel")
			}
			switch rest[i] {
			case string(domain.ScanActionPause), string(domain.ScanActionResume), string(domain.ScanActionCancel):
				action = domain.ScanAction(rest[i])
			default:
				return rivalFlagError(stderr, "--action chỉ nhận pause|resume|cancel")
			}
		default:
			return rivalFlagError(stderr, "tham số không hợp lệ: "+rest[i])
		}
	}
	if action != "" && history {
		return rivalFlagError(stderr, "--action không đi cùng --history")
	}

	mutations, _, closeDeps, err := openRivalMutations()
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	defer closeDeps()

	// --cap và một hành động/history có thể đi cùng nhau: đặt giới hạn trước
	// rồi mới phát hành động, mỗi bước đều được áp (không nhánh nào che nhánh
	// kia). Không cờ nào = kiểm tra video mới.
	var st domain.ScanStatus
	if capSet {
		if st, err = mutations.SetCap(ctx, id, capVal); err != nil {
			return rivalErrOut(stderr, err)
		}
	}
	switch {
	case action != "":
		st, err = mutations.Control(ctx, id, action)
	case history:
		st, err = mutations.Control(ctx, id, domain.ScanActionStart)
	case capSet:
		// Chỉ đổi giới hạn: trạng thái SetCap ở trên là kết quả cuối.
		err = nil
	default:
		st, err = mutations.Control(ctx, id, domain.ScanActionCheck)
	}
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	// Chỉ lệnh "chạy" mới gọi RunInline — tính ý định một lần: --history
	// (start), --action resume, hoặc mặc định không cờ nào (kiểm tra video
	// mới). Chỉ --cap KHÔNG phải lệnh chạy: nó chỉ đổi cấu hình, chạy worker
	// trong process này sẽ thực thi việc xếp lịch khác và gọi provider.
	// RunInline tự trả ran=false ngay khi serve đang giữ lease worker.
	if scanRunIntent(history, capSet, action) {
		if ran, rerr := mutations.RunInline(ctx, id); rerr != nil {
			return rivalErrOut(stderr, rerr)
		} else if ran {
			if st2, serr := mutations.Status(ctx, id); serr == nil {
				st = st2
			}
		}
	}
	writeRivalJSON(stdout, web.ScanStatusToWire(st))
	return 0
}

// runRivalVideos xử lý `viderse rival videos <id|all> [flags]`.
func runRivalVideos(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return rivalUsageError(stderr, "sai cú pháp lệnh")
	}
	id := args[0]
	q := rivals.VideoQuery{Sort: rivals.SortOutlier, Limit: rivals.VideoPageDefault}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		flag := rest[i]
		next := func() (string, bool) {
			if i+1 < len(rest) {
				i++
				return rest[i], true
			}
			return "", false
		}
		switch flag {
		case "--long":
			q.Long = true
		case "--sort":
			v, ok := next()
			if !ok || (v != rivals.SortOutlier && v != rivals.SortNewest && v != rivals.SortViews) {
				return rivalFlagError(stderr, "--sort phải là outlier|newest|views")
			}
			q.Sort = v
		case "--within":
			v, ok := next()
			if !ok {
				return rivalFlagError(stderr, "--within phải là 0|7|30|90")
			}
			days, err := strconv.Atoi(v)
			if err != nil {
				return rivalFlagError(stderr, "--within phải là 0|7|30|90")
			}
			q.WithinDays = days
		case "--q":
			v, ok := next()
			if !ok {
				return rivalFlagError(stderr, "--q cần chuỗi tìm")
			}
			q.Query = v
		case "--limit":
			v, ok := next()
			if !ok {
				return rivalFlagError(stderr, "--limit cần một số")
			}
			limit, err := strconv.Atoi(v)
			if err != nil || limit < 1 || limit > rivals.VideoPageMax {
				return rivalFlagError(stderr, "--limit phải là số nguyên từ 1 tới 200")
			}
			q.Limit = limit
		case "--cursor":
			v, ok := next()
			if !ok {
				return rivalFlagError(stderr, "--cursor cần giá trị")
			}
			q.Cursor = v
		default:
			return rivalFlagError(stderr, "tham số không hợp lệ: "+flag)
		}
	}
	readSvc, closeDeps, err := openRivalReads()
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	defer closeDeps()
	page, err := readSvc.Videos(ctx, id, q)
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	videos := make([]any, 0, len(page.Videos))
	for _, v := range page.Videos {
		videos = append(videos, web.VideoToWire(v))
	}
	var next *string
	if page.NextCursor != "" {
		next = &page.NextCursor
	}
	writeRivalJSON(stdout, map[string]any{
		"videos": videos, "total": page.Total, "nextCursor": next,
	})
	return 0
}

// runRivalPostingSlots xử lý `viderse rival posting-slots <id> [--zone vn|ny]`.
func runRivalPostingSlots(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return rivalUsageError(stderr, "sai cú pháp lệnh")
	}
	id, zone := args[0], domain.ZoneVN
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--zone":
			i++
			if i >= len(args) || (args[i] != domain.ZoneVN && args[i] != domain.ZoneNY) {
				return rivalFlagError(stderr, "--zone phải là vn|ny")
			}
			zone = args[i]
		default:
			return rivalFlagError(stderr, "tham số không hợp lệ: "+args[i])
		}
	}
	readSvc, closeDeps, err := openRivalReads()
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	defer closeDeps()
	sched, err := readSvc.PostingSchedule(ctx, id, zone)
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	writeRivalJSON(stdout, web.PostingScheduleToWire(sched))
	return 0
}

// runRivalAdd theo dõi kênh rồi in đúng hình Rival của HTTP lấy lại từ kho —
// số đếm/trạng thái thật sau trang đầu + head check, không dựng số 0 giả.
func runRivalAdd(ctx context.Context, input string, mutations rivalMutations,
	reads rivalReads, stdout, stderr io.Writer) int {
	channel, err := mutations.Add(ctx, input)
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	entry, err := reads.Rival(ctx, channel.ID)
	if err != nil {
		return rivalErrOut(stderr, err)
	}
	writeRivalJSON(stdout, web.RivalToWire(*entry))
	return 0
}

// scanRunIntent là ý định chạy worker inline của `rival scan`: chỉ --history
// (start), --action resume hoặc mặc định không cờ nào mới chạy. pause/cancel/
// chỉ-setCap chỉ đổi trạng thái — không gọi provider, không xúc việc khác.
func scanRunIntent(history, capSet bool, action domain.ScanAction) bool {
	return history || action == domain.ScanActionResume || (!capSet && action == "")
}

// rivalReads là phần đọc lại sau khi theo dõi — *rivals.Service trong thật.
type rivalReads interface {
	Rival(ctx context.Context, channelID string) (*rivals.ListEntry, error)
}

// ── vỏ lỗi/JSON ────────────────────────────────────────────────────────────

// rivalErrCode là mã code của vỏ lỗi JSON cho một lỗi nghiệp vụ.
func rivalErrCode(err error) (code string, recovery string, retryAt *time.Time) {
	var se *scan.Error
	if errors.As(err, &se) {
		return se.Code, se.Recovery, se.RetryAt
	}
	switch {
	case errors.Is(err, rivals.ErrRivalNotFound):
		return scan.CodeNotFound, "", nil
	case errors.Is(err, rivals.ErrInvalidCursor):
		return "invalid_cursor", "", nil
	case errors.Is(err, rivals.ErrInvalidSort), errors.Is(err, rivals.ErrInvalidWithin),
		errors.Is(err, rivals.ErrInvalidZone), errors.Is(err, rivals.ErrInvalidLimit),
		errors.Is(err, rivals.ErrInvalidQuery):
		return scan.CodeInvalidInput, "", nil
	}
	return "", "", nil
}

// rivalErrOut ghi vỏ lỗi JSON {error, code?, recovery?, retryAt?} ra stderr và
// trả exit code 1.
func rivalErrOut(stderr io.Writer, err error) int {
	code, recovery, retryAt := rivalErrCode(err)
	body := map[string]any{"error": err.Error()}
	if code != "" {
		body["code"] = code
	}
	if recovery != "" {
		body["recovery"] = recovery
	}
	if retryAt != nil {
		body["retryAt"] = retryAt.UTC().Format(time.RFC3339)
	}
	raw, _ := json.Marshal(body)
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// rivalFlagError ghi lỗi tham số (vỏ JSON invalid_input) ra stderr, exit 1 —
// khác với thiếu tham số (in usage, exit 2).
func rivalFlagError(stderr io.Writer, msg string) int {
	body := map[string]any{"error": msg, "code": scan.CodeInvalidInput}
	raw, _ := json.Marshal(body)
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// rivalUsageError ghi lỗi thiếu/sai lệnh con dạng JSON ra stderr, exit 1 —
// luôn kèm gợi ý usage trong trường recovery để người đọc vẫn tìm được lệnh.
func rivalUsageError(stderr io.Writer, msg string) int {
	body := map[string]any{
		"error":    msg,
		"code":     scan.CodeInvalidInput,
		"recovery": strings.TrimRight(rivalUsage, "\n"),
	}
	raw, _ := json.Marshal(body)
	fmt.Fprintln(stderr, string(raw))
	return 1
}

// writeRivalJSON ghi một giá trị JSON ra stdout.
func writeRivalJSON(stdout io.Writer, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(stdout, string(raw))
}
