package retention

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// AutoEvery là nhịp tự dọn sau lần dọn lúc mở (Cài đặt ghi "mỗi 6 giờ").
const AutoEvery = 6 * time.Hour

// Item là một thứ sẽ bị xoá (hoặc đã xoá) cùng nhãn hiển thị.
type Item struct {
	Kind  string // temp-dir | codex-home | export-request | staged-export | media | media-pending
	Label string // câu tiếng Việt gom theo nhóm trên UI
	Path  string
	Bytes int64
}

// Candidate là một mục kèm hàm xoá và (tuỳ chọn) hàm kiểm lại điều kiện —
// dry run chỉ đếm, không gọi. Validator chạy lại ngay trước khi xoá trong
// một lần run: mục preview xong mà điều kiện đổi (file được hoàn tất, tham
// chiếu mới xuất hiện) sẽ được giữ.
type Candidate struct {
	Item      Item
	Remove    func(ctx context.Context) error
	Validator func(ctx context.Context) bool
}

// PreviewItem là một dòng của bảng "Xem trước những gì sẽ dọn".
type PreviewItem struct {
	Kind    string
	Label   string
	Count   int
	Bytes   int64
	Stopped bool // dòng "N tiến trình chạy dở bị bỏ quên"
}

// Preview là kết quả POST /api/storage/cleanup/preview. Bản run chỉ xoá
// đúng tập đã preview (pending giữ paths+validators+root lúc preview).
type Preview struct {
	ID      string
	Items   []PreviewItem
	Freed   int64
	Stopped []string
	// Errors là lỗi của một nhóm dò (ví dụ không đọc được receipt) — nhóm đó
	// không được xoá và người dùng thấy rõ thay vì một preview "trống".
	Errors []string
}

// RunResult là kết quả của một lần dọn thật. Errors và Kept cùng đi lên HTTP
// — người dùng thấy phần nào chưa dọn được thay vì một con số "thành công".
type RunResult struct {
	Removed int
	Freed   int64
	Stopped []string
	Errors  []string
	Kept    []string // mục đã preview nhưng điều kiện đổi → giữ lại
}

/* ------------------------------ interfaces ------------------------------ */

// Roots là phần thư mục dự án retention cần (storage.Service thoả).
type Roots interface {
	Root() string
	Roots() (current string, all []string)
}

// Settings là phần settings retention cần (ghi mốc lần dọn cuối).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
}

// Tasks là việc nền (activity.Service thoả).
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
}

// Reaper kết thúc nhóm process mồ côi; trả nhãn các nhóm đã dừng.
type Reaper interface {
	ReapOrphaned(ctx context.Context, dryRun bool) ([]string, error)
}

// Receipts đọc đường dẫn scratch của các lần vẽ ảnh chưa kết thúc — thư mục
// tạm mà chúng trỏ tới là phiên phục hồi, không được dọn.
type Receipts interface {
	ScratchDirs(ctx context.Context) ([]string, error)
}

// SourceVideo là một video đã xong còn giữ video gốc trên đĩa.
type SourceVideo struct {
	ID            string
	ProjectID     string
	SourceMediaID string
	Title         string
}

// PostedVideo là một video đã đăng còn giữ bản xuất MP4.
type PostedVideo struct {
	ID    string
	Title string
}

// Videos là trạng thái video mà retention cần: video đã xong (được xoá video
// gốc) và video đã đăng (được xoá MP4). Phase pipeline nối bản thật; khi chưa
// có video nào được ghi, EmptyVideos là câu trả lời đúng.
type Videos interface {
	Finished(ctx context.Context) ([]SourceVideo, error)
	Posted(ctx context.Context) ([]PostedVideo, error)
	// HeldMedia là id media mà việc/video còn chạy đang giữ.
	HeldMedia(ctx context.Context) (map[string]bool, error)
}

// EmptyVideos là trạng thái thật khi chưa có video nào: không có gì để dọn
// theo trạng thái video và không media nào bị giữ bởi bảng video.
type EmptyVideos struct{}

func (EmptyVideos) Finished(context.Context) ([]SourceVideo, error)    { return nil, nil }
func (EmptyVideos) Posted(context.Context) ([]PostedVideo, error)      { return nil, nil }
func (EmptyVideos) HeldMedia(context.Context) (map[string]bool, error) { return nil, nil }

/* -------------------------------- service -------------------------------- */

// GenClock cho phép test điều khiển thời gian (nil → time.Now).
type GenClock interface {
	Now() time.Time
}

type clockFunc func() time.Time

func (f clockFunc) Now() time.Time { return f() }

// Deps là phụ thuộc của Service.
type Deps struct {
	Roots     Roots
	ConfigDir string // ~/.viderse (cache, log…)
	TmpDir    string
	Reaper    Reaper
	Receipts  Receipts
	Videos    Videos
	Tasks     Tasks
	Settings  Settings
	// Changed báo cho storage đổi cache + phát SSE sau mỗi lần dọn.
	Changed func(ref string)
	Clock   GenClock
}

// Service chạy dọn dẹp. Mọi phương thức an toàn cho goroutine; manual và tự
// động không chạy chồng nhau.
type Service struct {
	deps Deps
	now  func() time.Time

	mu      sync.Mutex // bảo vệ preview
	runMu   sync.Mutex // một lần dọn tại một thời điểm
	preview *pendingPreview
	locks   *keyedLocks
	reclaim reclaimCache
}

// New dựng service; videos nil → EmptyVideos (chưa có video nào).
func New(deps Deps) *Service {
	if deps.Videos == nil {
		deps.Videos = EmptyVideos{}
	}
	now := time.Now
	if deps.Clock != nil {
		now = deps.Clock.Now
	}
	return &Service{deps: deps, now: now, locks: newKeyedLocks()}
}

/* ------------------------------- preview/run ------------------------------ */

// PreviewCleanup dựng danh sách preview (không xoá gì). previewID cũ bị thay
// thế — run chỉ nhận đúng lần preview mới nhất.
func (s *Service) PreviewCleanup(ctx context.Context) (*Preview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	candidates, stopped, groupErrs, err := s.collect(ctx, true)
	if err != nil {
		return nil, err
	}
	prev := &Preview{
		ID:      randID(8),
		Items:   groupPreview(candidates, stopped),
		Stopped: stopped,
		Errors:  groupErrs,
	}
	for _, c := range candidates {
		prev.Freed += c.Item.Bytes
	}
	s.mu.Lock()
	s.preview = &pendingPreview{
		id: prev.ID, root: s.deps.Roots.Root(),
		candidates: candidates, stopped: stopped,
	}
	s.mu.Unlock()
	return prev, nil
}

// pendingPreview gắn một lần preview với tập đã được phê duyệt: root tại
// thời điểm dò (đổi root thì preview đó không còn hiệu lực) và danh sách
// candidates thực sự — run chỉ xoá đúng chúng, sau khi Validator re-check.
type pendingPreview struct {
	id         string
	root       string
	candidates []Candidate
	stopped    []string
}

// RunCleanup xoá đúng những gì lần preview mới nhất liệt kê.
func (s *Service) RunCleanup(ctx context.Context, previewID string) (RunResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.mu.Lock()
	pending := s.preview
	if pending != nil && pending.id == previewID {
		s.preview = nil // một lần preview chỉ dùng được một lần — consume ngay
	}
	s.mu.Unlock()
	if pending == nil || previewID == "" || pending.id != previewID {
		return RunResult{}, &Error{Code: "preview_required", Message: "cần xem trước lại trước khi dọn"}
	}
	// Preview gắn với root lúc dò: đổi thư mục dự án sau preview thì bắt
	// buộc dò lại — không bao giờ xoá mục ở root khác mà người dùng chưa thấy.
	if root := s.deps.Roots.Root(); root != pending.root {
		return RunResult{}, &Error{Code: "preview_required", Message: "thư mục dự án đã đổi — xem trước lại trước khi dọn"}
	}
	taskID, _ := s.cleanupTask(ctx, "Dọn dữ liệu Lưu trữ")
	result := s.execute(ctx, pending.candidates, pending.stopped)
	s.finishTask(taskID)
	if err := s.markLastRun(ctx); err != nil {
		slog.Warn("retention: ghi mốc dọn cuối lỗi", "err", err)
	}
	return result, nil
}

// Error là lỗi nghiệp vụ của retention (tầng web đổi sang HTTP).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// RunAuto là lần dọn tự động (lúc mở + mỗi 6 giờ): chỉ phần tạm, không đụng
// media chưa tham chiếu. Bỏ qua khi một lần dọn tay đang chạy.
func (s *Service) RunAuto(ctx context.Context) {
	if !s.runMu.TryLock() {
		return
	}
	defer s.runMu.Unlock()

	candidates, stopped, groupErrs, err := s.collect(ctx, false)
	if err != nil {
		return
	}
	if len(groupErrs) > 0 {
		slog.Warn("retention: một nhóm dò lỗi — phần đó giữ nguyên", "errors", groupErrs)
	}
	// Media chờ dọn lại của lần xoá trước: mỗi lần dọn đều thử lại.
	if err := s.reclaimPending(ctx); err != nil {
		slog.Warn("retention: dọn media chờ lỗi", "err", err)
	}
	if len(candidates) == 0 && len(stopped) == 0 {
		return // không có gì để làm: không ghi dòng việc nền vô nghĩa
	}
	taskID, _ := s.cleanupTask(ctx, "Dọn dữ liệu định kỳ")
	result := s.execute(ctx, candidates, stopped)
	s.finishTask(taskID)
	if len(result.Errors) > 0 {
		slog.Warn("retention: tự dọn còn lỗi", "errors", len(result.Errors))
	}
	if result.Removed > 0 || len(result.Stopped) > 0 {
		if err := s.markLastRun(ctx); err != nil {
			slog.Warn("retention: ghi mốc dọn cuối lỗi", "err", err)
		}
	}
}

// Start chạy lần dọn đầu (không chặn khởi động) rồi lặp mỗi 6 giờ tới khi ctx
// kết thúc.
func (s *Service) Start(ctx context.Context) error {
	go func() {
		run := func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("retention: dọn tự động panic", "panic", r)
				}
			}()
			s.RunAuto(ctx)
		}
		run()
		t := time.NewTicker(AutoEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	}()
	return nil
}

// execute xoá từng candidate, giữ lỗi lại (một mục lỗi không chặn phần còn
// lại) và chạy reaper.
func (s *Service) execute(ctx context.Context, candidates []Candidate, stopped []string) RunResult {
	result := RunResult{Stopped: stopped}
	if s.deps.Reaper != nil {
		labels, err := s.deps.Reaper.ReapOrphaned(ctx, false)
		if err != nil {
			result.Errors = append(result.Errors, "process: "+err.Error())
		} else {
			result.Stopped = labels
		}
	}
	for _, c := range candidates {
		if err := ctx.Err(); err != nil {
			break
		}
		if c.Validator != nil && !c.Validator(ctx) {
			result.Kept = append(result.Kept, c.Item.Path) // điều kiện đổi: giữ lại
			continue
		}
		if err := c.Remove(ctx); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", c.Item.Path, err))
			continue
		}
		result.Removed++
		result.Freed += c.Item.Bytes
		if c.Item.Kind == "media" {
			// Dọn xong media: gỡ dòng của nó khỏi index.
			if err := s.dropIndexEntry(c.Item.Path); err != nil {
				result.Errors = append(result.Errors, err.Error())
			}
		}
	}
	if s.deps.Changed != nil {
		s.deps.Changed("")
	}
	return result
}

// collect dựng candidates. full=false: chỉ phần tạm (bỏ media). Trả thêm
// lỗi của từng nhóm — nhóm lỗi fail closed (không xoá gì của nó) nhưng người
// dùng phải thấy trên preview thay vì một danh sách trống yên lặng.
func (s *Service) collect(ctx context.Context, full bool) ([]Candidate, []string, []string, error) {
	var candidates []Candidate
	var stopped []string
	var groupErrs []string
	if s.deps.Reaper != nil {
		labels, err := s.deps.Reaper.ReapOrphaned(ctx, true)
		if err == nil {
			stopped = labels
		} else {
			groupErrs = append(groupErrs, "Không dò được tiến trình chạy dở: "+err.Error())
		}
	}
	finders := []func(context.Context) ([]Candidate, error){
		s.tempCandidates,
		s.codexHomeCandidates,
		s.exportRequestCandidates,
		s.stagedExportCandidates,
	}
	if full {
		finders = append(finders, s.mediaCandidates)
	}
	for _, find := range finders {
		found, err := find(ctx)
		if err != nil {
			groupErrs = append(groupErrs, err.Error())
		}
		candidates = append(candidates, found...)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Item.Path < candidates[j].Item.Path })
	return candidates, stopped, groupErrs, nil
}

// groupPreview gộp candidates theo nhãn cho bảng preview.
func groupPreview(candidates []Candidate, stopped []string) []PreviewItem {
	order := []string{
		"Tệp tạm khi dựng và tải video",
		"Bản ghi xuất đã xong",
		"Bản xuất chưa chốt",
		"Media không còn video hay template nào dùng",
	}
	byLabel := map[string]*PreviewItem{}
	for _, c := range candidates {
		row, ok := byLabel[c.Item.Label]
		if !ok {
			row = &PreviewItem{Kind: c.Item.Kind, Label: c.Item.Label}
			byLabel[c.Item.Label] = row
		}
		row.Count++
		row.Bytes += c.Item.Bytes
		if c.Item.Kind == "media" {
			row.Kind = "media"
		}
	}
	var out []PreviewItem
	for _, label := range order {
		if row, ok := byLabel[label]; ok {
			out = append(out, *row)
			delete(byLabel, label)
		}
	}
	var rest []string
	for label := range byLabel {
		rest = append(rest, label)
	}
	sort.Strings(rest)
	for _, label := range rest {
		out = append(out, *byLabel[label])
	}
	if len(stopped) > 0 {
		out = append(out, PreviewItem{Kind: "process", Label: "tiến trình chạy dở bị bỏ quên", Count: len(stopped), Stopped: true})
	}
	return out
}

// markLastRun ghi mốc dọn gần nhất.
func (s *Service) markLastRun(ctx context.Context) error {
	if s.deps.Settings == nil {
		return nil
	}
	return s.deps.Settings.Set(ctx, domain.KeyStorageCleanAt, s.now().Format(time.RFC3339))
}

// cleanupTask mở một dòng việc nền kind cleanup nếu có Tasks.
func (s *Service) cleanupTask(ctx context.Context, title string) (string, error) {
	if s.deps.Tasks == nil {
		return "", nil
	}
	return s.deps.Tasks.Start(ctx, domain.TaskSpec{
		Kind:   domain.TaskCleanup,
		Title:  title,
		Object: domain.ObjectRef{Type: "storage", ID: "cleanup", Label: "Dọn dữ liệu"},
		Phase:  "Đang dọn",
	})
}

// finishTask gỡ dòng việc nền.
func (s *Service) finishTask(taskID string) {
	if s.deps.Tasks == nil || taskID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.deps.Tasks.Finish(ctx, taskID); err != nil {
		slog.Warn("retention: gỡ việc nền lỗi", "err", err)
	}
}

// exists báo tệp/thư mục tồn tại.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
