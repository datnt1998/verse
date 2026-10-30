package scan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// Provider là phần của youtube.Client mà luồng quét cần (khai phía dùng).
type Provider interface {
	ResolveChannelRef(ctx context.Context, input string) (string, error)
	FirstChannelPage(ctx context.Context, channel string) (*youtube.ChannelVideoPage, error)
	NextChannelPage(ctx context.Context, cursor youtube.ChannelCursor) (*youtube.ChannelVideoPage, error)
	FetchUploadTime(ctx context.Context, videoID string) (*time.Time, bool, error)
}

// Settings là phần đọc cài đặt mà service cần (khoá clone.rescanHours).
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
}

// Tasks là phần của activity.Service mà luồng quét dùng: đóng dòng việc khi
// việc đã dừng hẳn và phát sự kiện thay đổi của dòng/thông báo sau khi kho đã
// ghi nguyên tử (SSE). Mọi lần ghi state/phase/action của dòng việc đi qua
// Store.SyncScanTask, không qua đây.
type Tasks interface {
	Finish(ctx context.Context, id string) error
	// Changed phát sự kiện cho ref (id việc nền hoặc id thông báo) mà không
	// sửa dữ liệu.
	Changed(ref string)
}

// TaskFinder là phần tra việc nền theo đối tượng (activity.Service); dọn dòng
// của kênh vừa bị xoá không tra qua task_id được nên cần tra theo object_ref.
type TaskFinder interface {
	FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*activity.Task, error)
}

// PlatformGate là cổng nền tảng bền: giờ chờ còn hiệu lực (cùng dữ liệu
// RecordBlocked ghi) và ghi một lần bị chặn/429.
type PlatformGate interface {
	Until(ctx context.Context, id domain.PlatformID) (*time.Time, error)
	RecordBlocked(ctx context.Context, id domain.PlatformID, retryAfter time.Duration) error
}

// Runtime là phần của worker mà service đánh thức sau một thay đổi từ API.
type Runtime interface {
	Wake()
}

// Deps của Service: mọi phụ thuộc bên ngoài đều qua interface/hàm tiêm vào.
type Deps struct {
	Store    Store
	YouTube  Provider
	Settings Settings
	Tasks    Tasks        // nil = không ghi việc nền/thông báo
	Platform PlatformGate // nil = không kiểm/ghi giờ chờ nền tảng
	Now      func() time.Time
	// AddBudget là trần thời gian một lần thêm đối thủ (mặc định 30s).
	AddBudget time.Duration
}

// Service là mặt nghiệp vụ của Trinh sát cho HTTP/CLI.
type Service struct {
	d   Deps
	now func() time.Time

	mu       sync.Mutex
	runtime  Runtime
	notify   func(ref string)
	stopped  bool
	progress map[string]time.Time // lần ghi tiến độ gần nhất (≤ 4/s)
}

// New dựng service; Store và YouTube bắt buộc (panic sớm ở bootstrap).
func New(d Deps) *Service {
	if d.Store == nil || d.YouTube == nil {
		panic("scan.New thiếu Store hoặc YouTube")
	}
	s := &Service{
		d:        d,
		now:      d.Now,
		progress: map[string]time.Time{},
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.d.AddBudget <= 0 {
		s.d.AddBudget = 30 * time.Second
	}
	return s
}

// SetNotifier gắn kênh SSE của tầng vận chuyển (serve); CLI không gọi.
func (s *Service) SetNotifier(fn func(ref string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notify = fn
}

// BindWorker nối service với worker sau khi cả hai được dựng.
func (s *Service) BindWorker(w Runtime) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime = w
}

// wake đánh thức worker sau một thay đổi từ phía người dùng.
func (s *Service) wake() {
	s.mu.Lock()
	runtime := s.runtime
	s.mu.Unlock()
	if runtime != nil {
		runtime.Wake()
	}
}

// emit phát sự kiện "rivals" cho một kênh; tầng web gộp nhịp cho cả luồng sự
// kiện của loại này (≤ 4 frame/s), nên scan chỉ chuyển tiếp thay đổi.
func (s *Service) emit(ref string) {
	s.mu.Lock()
	fn, stopped := s.notify, s.stopped
	s.mu.Unlock()
	if fn != nil && !stopped {
		fn(ref)
	}
}

// Stop ngừng phát sự kiện (serve shutdown, sau khi worker đã join).
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
}

// rescanHours đọc khoảng quét lại của đối thủ (clone.rescanHours, mặc định 1,
// kẹp 1–24; giá trị xấu đọc như chưa ghi).
func (s *Service) rescanHours(ctx context.Context) int {
	hours := float64(domain.RescanHoursDefault)
	if s.d.Settings != nil {
		var v float64
		if found, err := s.d.Settings.Get(ctx, domain.KeyRescanHours, &v); err == nil && found {
			hours = v
		}
	}
	if math.IsNaN(hours) || math.IsInf(hours, 0) {
		hours = domain.RescanHoursDefault
	}
	return int(math.Min(domain.RescanHoursMax, math.Max(domain.RescanHoursMin, math.Round(hours))))
}

// platformPause trả giờ chờ còn hiệu lực của YouTube (nil = không chờ).
func (s *Service) platformPause(ctx context.Context) (*time.Time, error) {
	if s.d.Platform == nil {
		return nil, nil
	}
	return s.d.Platform.Until(ctx, domain.PlatformYouTube)
}

// blockedFor đổi giờ chờ nền tảng thành lỗi blocked của mặt API. Lỗi đọc lan
// ra nguyên vẹn: không đọc được rào chung thì không được phép request.
func (s *Service) blockedFor(ctx context.Context) error {
	until, err := s.platformPause(ctx)
	if err != nil {
		return err
	}
	if until == nil {
		return nil
	}
	return &Error{Code: CodeBlocked, Message: "YouTube đang tạm giới hạn yêu cầu", RetryAt: until, Recovery: RecoveryBlocked}
}

// recordProviderBlock ghi bền khi lời gọi YouTube của mặt API bị chặn/429.
func (s *Service) recordProviderBlock(ctx context.Context, err error) {
	kind, ok := youtube.KindOf(err)
	if !ok || (kind != youtube.KindRateLimited && kind != youtube.KindBlocked) {
		return
	}
	var delay time.Duration
	var fe *youtube.FetchError
	if errors.As(err, &fe) && fe.RetryDelay != nil {
		delay = *fe.RetryDelay
	}
	s.recordPlatformBlocked(ctx, delay)
}

// recordPlatformBlocked ghi một lần YouTube chặn/429 cho mọi process khác đọc.
func (s *Service) recordPlatformBlocked(ctx context.Context, retryAfter time.Duration) {
	if s.d.Platform != nil {
		_ = s.d.Platform.RecordBlocked(ctx, domain.PlatformYouTube, retryAfter)
	}
}

// ── thêm/bỏ đối thủ ─────────────────────────────────────────────────────────

// Add theo dõi một kênh đối thủ mới: phân giải ref, đọc trang đầu tab Videos
// (Mới nhất), commit baseline trong một giao dịch dưới fence kênh rồi tính mức
// thường + điểm nổi trội. Không khởi động quét lịch sử.
func (s *Service) Add(ctx context.Context, input string) (domain.RivalChannel, error) {
	if err := s.blockedFor(ctx); err != nil {
		return domain.RivalChannel{}, err
	}
	ref, ok := youtube.ParseChannelRef(input)
	if !ok {
		return domain.RivalChannel{}, &Error{
			Code:    CodeInvalidInput,
			Message: "Đầu vào không phải kênh YouTube — dán @handle, link kênh hoặc mã UC….",
		}
	}
	ctx, cancel := context.WithTimeout(ctx, s.d.AddBudget)
	defer cancel()

	page, err := s.d.YouTube.FirstChannelPage(ctx, ref)
	if err != nil {
		// 429/blocked trên đường phân giải/trang đầu đi cùng đường ghi bền như
		// worker (Retry-After được tôn trọng): mọi process cùng tôn trọng.
		s.recordProviderBlock(ctx, err)
		return domain.RivalChannel{}, s.mapProviderError(err)
	}
	profile := page.Profile
	if profile == nil || !youtube.IsChannelID(profile.ID) {
		return domain.RivalChannel{}, &Error{
			Code:     CodeUnexpectedPage,
			Message:  "Trang kênh không mang hồ sơ đọc được",
			Recovery: RecoveryFatal,
		}
	}
	channelID := profile.ID

	// Đã theo dõi → 409 exists kèm bản ghi hiện có (không fetch thêm).
	if existing, err := s.d.Store.Channel(ctx, channelID); err == nil {
		return domain.RivalChannel{}, existsError(existing)
	} else if StoreCodeOf(err) != StoreChannelMissing {
		return domain.RivalChannel{}, err
	}

	lease := Lease{ChannelID: channelID, Token: newToken(), Owner: "add", Until: s.now().Add(LeaseTTL)}
	// Nhả fence trên mọi đường thoát (kể cả lỗi head check): Add không giữ kênh.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.d.Store.ReleaseChannelLease(cleanup, lease)
	}()

	var newest *string
	if len(page.Videos) > 0 {
		newest = &page.Videos[0].ID
	}
	checkedAt := s.now()
	if _, err := s.d.Store.CommitPage(ctx, CommitPageInput{
		ChannelID:     channelID,
		Videos:        page.Videos,
		NextCursor:    page.NextCursor,
		Origin:        domain.OriginBaseline,
		Lease:         lease,
		NewestVideoID: newest,
		Profile:       profile,
		Now:           checkedAt,
	}); err != nil {
		if StoreCodeOf(err) == StoreChannelExists {
			if winner, rerr := s.d.Store.Channel(ctx, channelID); rerr == nil {
				return domain.RivalChannel{}, existsError(winner)
			}
		}
		return domain.RivalChannel{}, s.mapStoreError(channelID, err)
	}
	if _, err := s.d.Store.CompleteHeadCheck(ctx, channelID, HeadCheckInput{
		CheckedAt: checkedAt,
		NewestID:  newest,
		Lease:     lease,
		Now:       s.now(),
	}); err != nil {
		return domain.RivalChannel{}, s.mapStoreError(channelID, err)
	}

	channel, err := s.d.Store.Channel(ctx, channelID)
	if err != nil {
		return domain.RivalChannel{}, err
	}
	s.emit(channel.ID)
	s.wake()
	return channel, nil
}

func existsError(existing domain.RivalChannel) error {
	cur := existing
	return &Error{Code: CodeExists, Message: "Đã theo dõi kênh này", Existing: &cur}
}

// Remove gỡ theo dõi: xoá kênh, video, trạng thái và việc quét (cascade).
func (s *Service) Remove(ctx context.Context, channelID string) error {
	channel, err := s.d.Store.Channel(ctx, channelID)
	if err != nil {
		return s.mapStoreError(channelID, err)
	}
	if err := s.d.Store.DeleteChannel(ctx, channelID); err != nil {
		return s.mapStoreError(channelID, err)
	}
	s.finishTasks(ctx, channel)
	s.emit(channel.ID)
	s.wake()
	return nil
}

// finishTasks xoá dòng việc nền của một kênh vừa bị gỡ (kênh đã xoá nên
// không tra qua task_id của job được — tra theo object_ref).
func (s *Service) finishTasks(ctx context.Context, channel domain.RivalChannel) {
	if s.d.Tasks == nil {
		return
	}
	for _, label := range taskLabels(channel.Title) {
		object := domain.ObjectRef{Type: "rival", ID: channel.ID, Label: label}
		if finder, ok := s.d.Tasks.(TaskFinder); ok {
			if task, err := finder.FindTaskByObject(ctx, domain.TaskChannelScan, object); err == nil && task != nil {
				_ = s.d.Tasks.Finish(ctx, task.ID)
			}
		}
	}
}

// taskLabels là nhãn object_ref của hai dòng việc một kênh có thể có.
func taskLabels(title string) []string {
	return []string{title, title + " · video mới"}
}

// ── trạng thái + điều khiển ────────────────────────────────────────────────

// Status đọc trạng thái quét của một kênh (một snapshot).
func (s *Service) Status(ctx context.Context, channelID string) (domain.ScanStatus, error) {
	snap, err := s.d.Store.Status(ctx, channelID)
	if err != nil {
		return domain.ScanStatus{}, s.mapStoreError(channelID, err)
	}
	return s.statusOf(ctx, snap), nil
}

// statusOf dựng ScanStatus từ snapshot + cài đặt hiện tại.
func (s *Service) statusOf(ctx context.Context, snap StatusSnapshot) domain.ScanStatus {
	rescan := s.rescanHours(ctx)
	st := domain.ScanStatus{
		ChannelID:     snap.Channel.ID,
		HistoryCap:    domain.HistoryCapDefault,
		IndexedVideos: 0,
		RescanHours:   rescan,
	}
	if snap.HasState {
		st.HistoryCap = snap.State.HistoryCap
		st.IndexedVideos = snap.State.IndexedVideos
		st.History.ReachedEnd = snap.State.HistoryEnd
	}
	historyState := domain.HistoryState(domain.HistoryIdle)
	if snap.HasState && snap.State.HistoryEnd {
		historyState = domain.HistoryState(domain.ScanComplete)
	}
	if j := snap.HistoryJob; j != nil {
		historyState = domain.HistoryState(j.State)
		if j.State == domain.ScanRunning && !snap.WorkerLive {
			historyState = domain.HistoryState(domain.ScanQueued)
		}
		st.History = domain.HistoryStatus{
			State:      historyState,
			JobID:      j.ID,
			Scanned:    j.Scanned,
			Added:      j.Added,
			Pages:      j.Pages,
			ReachedEnd: j.ReachedEnd || st.History.ReachedEnd,
			StartedAt:  j.StartedAt,
			UpdatedAt:  &j.UpdatedAt,
		}
		if j.State == domain.ScanBackoff {
			st.History.NextAttemptAt = j.NextAttemptAt
		}
		switch j.State {
		case domain.ScanError, domain.ScanBackoff, domain.ScanPaused, domain.ScanCapped:
			st.History.ErrorCode = j.ErrorCode
			st.History.Error = j.LastError
			st.History.Recovery = j.Recovery
		}
	} else {
		st.History.State = historyState
	}
	st.Discovery.CheckedAt = snap.Channel.ScannedAt
	st.Discovery.CaughtUp = (!snap.HasState || snap.State.DiscoveryCaughtUp) && snap.DiscoveryJob == nil
	st.Discovery.Running = snap.DiscoveryJob != nil && liveState(snap.DiscoveryJob.State)
	if snap.Channel.ScannedAt != nil {
		next := snap.Channel.ScannedAt.Add(time.Duration(rescan) * time.Hour)
		st.Discovery.NextAt = &next
	}
	return st
}

func liveState(state domain.ScanJobState) bool {
	return state == domain.ScanQueued || state == domain.ScanRunning || state == domain.ScanBackoff
}

// Control thực hiện một thao tác của người dùng lên luồng quét của kênh:
// start/pause/cancel cho lịch sử; resume tiếp tục việc đã dừng (lịch sử
// paused/error, discovery paused/error/cancelled); check xếp một lượt kiểm tra.
func (s *Service) Control(ctx context.Context, channelID string, action domain.ScanAction) (domain.ScanStatus, error) {
	var err error
	switch action {
	case domain.ScanActionStart:
		var job *Job
		job, err = s.d.Store.StartBackfill(ctx, channelID)
		if err == nil && job != nil {
			s.syncTask(ctx, job, taskSync{})
		}
	case domain.ScanActionPause, domain.ScanActionCancel:
		err = s.d.Store.ControlBackfill(ctx, channelID, action)
	case domain.ScanActionResume:
		// Lịch sử: paused/error → hàng chờ; discovery: run đã dừng → run mới.
		// Lịch sử đã đủ cap không phải lỗi của thao tác tiếp tục: trạng thái
		// vẫn "capped" cho tới khi người dùng nâng cap.
		err = s.d.Store.ControlBackfill(ctx, channelID, action)
		if StoreCodeOf(err) == StoreAlreadyCapped {
			err = nil
		}
		if err == nil {
			if _, derr := s.d.Store.StartDiscovery(ctx, channelID, true); derr != nil &&
				StoreCodeOf(derr) != StoreAlreadyCapped {
				err = derr
			}
		}
	case domain.ScanActionCheck:
		// Check chỉ xếp một lượt kiểm tra video mới; nó không đụng lịch sử đã
		// được người dùng tạm dừng/huỷ (resume/start/cap-unpark mới được đụng).
		_, err = s.d.Store.StartDiscovery(ctx, channelID, true)
	default:
		return domain.ScanStatus{}, &Error{Code: CodeInvalidInput, Message: "Thao tác quét không hợp lệ"}
	}
	if err != nil {
		return domain.ScanStatus{}, s.mapStoreError(channelID, err)
	}
	s.refreshTask(ctx, channelID, domain.ScanHistory)
	s.refreshTask(ctx, channelID, domain.ScanDiscovery)
	s.emit(channelID)
	if action != domain.ScanActionPause && action != domain.ScanActionCancel {
		s.wake()
	}
	return s.Status(ctx, channelID)
}

// SetCap đổi giới hạn quét lịch sử (150–10.000).
func (s *Service) SetCap(ctx context.Context, channelID string, cap int) (domain.ScanStatus, error) {
	if cap < domain.HistoryCapMin || cap > domain.HistoryCapMax {
		return domain.ScanStatus{}, &Error{
			Code:    CodeInvalidCap,
			Message: "--cap phải là số nguyên từ 150 tới 10000",
		}
	}
	if err := s.d.Store.SetHistoryCap(ctx, channelID, cap); err != nil {
		return domain.ScanStatus{}, s.mapStoreError(channelID, err)
	}
	s.refreshTask(ctx, channelID, domain.ScanHistory)
	s.emit(channelID)
	s.wake()
	return s.Status(ctx, channelID)
}

// RunInline chạy vòng lặp worker cho riêng một kênh (CLI): chỉ khi lease
// worker đang trống; trả ran=false khi một worker khác đang giữ (caller chỉ
// xếp việc và in trạng thái). Worker ad-hoc chia sẻ durable deps của service.
func (s *Service) RunInline(ctx context.Context, channelID string) (bool, error) {
	worker := NewWorker(WorkerDeps{
		Store:    s.d.Store,
		YouTube:  s.d.YouTube,
		Service:  s,
		Platform: s.d.Platform,
		Now:      s.now,
	})
	return worker.RunInline(ctx, channelID)
}

// ── việc nền (background_tasks) ─────────────────────────────────────────────

// taskSync điều khiển một lần đồng bộ dòng Việc nền.
type taskSync struct {
	// progress cho phép ghi tiến độ; lần ghi bị giới hạn ≤ 4/s/kênh ở đây.
	progress bool
}

// Notice là một thông báo "Cần xử lý" đã sẵn để ghi (cùng dạng với thông báo
// của activity: kind/text/href/nhãn hành động).
type Notice struct {
	Kind        string
	Text        string
	Href        string
	ActionLabel string
}

// FailureNotice là thông báo "Cần xử lý" cho một việc vừa chuyển sang failed.
// Hàm thuần: cùng câu chữ với thông báo lỗi quét trước đây.
func FailureNotice(title, channelID string, view TaskView) Notice {
	return Notice{
		Kind:        "needs_attention",
		Text:        fmt.Sprintf("%s · %s: %s", title, channelID, view.Phase),
		Href:        "#/explore",
		ActionLabel: "",
	}
}

// TaskSync là kết quả một lần ghi nguyên tử dòng việc nền: id dòng, cách hiện
// đã ghi (suy từ việc đang khoá) và id thông báo khi chính lần ghi này đưa
// việc sang failed (rỗng ở mọi lần khác).
type TaskSync struct {
	TaskID         string
	View           TaskView
	NotificationID string
}

// TaskView là phần trình bày (trạng thái, pha, hành động) của dòng Việc nền,
// suy ra duy nhất từ việc đang khoá — không phải dữ liệu người gọi cung cấp.
type TaskView struct {
	State  domain.TaskState
	Phase  string
	Action domain.TaskAction
}

// TaskPresentation ánh xạ việc hiện tại sang dòng Việc nền. live=false khi
// việc đã dừng hẳn (hết/huỷ/chạm trần): không có dòng nào để hiện. Hàm thuần
// để tầng kho và service dùng chung đúng một cách hiện.
func TaskPresentation(job Job) (TaskView, bool) {
	switch job.State {
	case domain.ScanComplete, domain.ScanCancelled, domain.ScanCapped:
		return TaskView{}, false
	case domain.ScanPaused:
		return TaskView{
			State:  domain.TaskPaused,
			Phase:  "Đã tạm dừng",
			Action: domain.TaskAction{Label: "Tiếp tục", Op: domain.TaskOpResume},
		}, true
	case domain.ScanBackoff:
		view := TaskView{
			State: domain.TaskWaiting,
			Phase: "Đang chờ thử lại",
		}
		if job.Kind == domain.ScanDiscovery {
			// Endpoint huỷ của Trinh sát chỉ tác động lịch sử; discovery đang
			// chờ thử lại phải dùng hành động không phá huỷ (Mở kênh), không
			// được mượn nút Huỷ của lịch sử.
			view.Action = domain.TaskAction{Label: "Mở", Href: taskHref(job.ChannelID)}
		} else {
			view.Action = domain.TaskAction{Label: "Huỷ", Op: domain.TaskOpCancel}
		}
		return view, true
	case domain.ScanError:
		phase := strings.TrimSpace(job.LastError)
		if phase == "" {
			phase = "Lỗi khi quét"
		}
		return TaskView{
			State:  domain.TaskFailed,
			Phase:  phase,
			Action: domain.TaskAction{Label: "Thử lại", Op: domain.TaskOpRetry},
		}, true
	}
	if job.Kind == domain.ScanDiscovery {
		return TaskView{
			State:  domain.TaskRunning,
			Phase:  "Đang kiểm tra video mới",
			Action: domain.TaskAction{Label: "Mở", Href: taskHref(job.ChannelID)},
		}, true
	}
	return TaskView{
		State:  domain.TaskRunning,
		Phase:  "Đang đọc tab Videos",
		Action: domain.TaskAction{Label: "Huỷ", Op: domain.TaskOpCancel},
	}, true
}

// taskSpecOf là phần dữ liệu tĩnh của dòng Việc nền (loại, tiêu đề, đối tượng);
// trạng thái/pha/hành động luôn lấy từ TaskPresentation.
func taskSpecOf(job *Job, channelTitle string) domain.TaskSpec {
	kind := domain.ScanJobKind(job.Kind)
	title := "Quét lịch sử"
	if kind == domain.ScanDiscovery {
		title = "Kiểm tra video mới"
	}
	view, _ := TaskPresentation(*job)
	return domain.TaskSpec{
		Kind:  domain.TaskChannelScan,
		Title: title,
		Object: domain.ObjectRef{
			Type:  "rival",
			ID:    job.ChannelID,
			Label: taskLabelOf(job, channelTitle),
		},
		Phase:  view.Phase,
		Action: view.Action,
	}
}

// taskLabelOf là nhãn của object_ref: cùng channel id nhưng khác nhãn cho
// discovery để hai việc không gộp thành một dòng.
func taskLabelOf(job *Job, channelTitle string) string {
	if job.Kind == domain.ScanDiscovery {
		return channelTitle + " · video mới"
	}
	return channelTitle
}

// taskHref là đích của hành động Mở của dòng discovery.
func taskHref(channelID string) string { return "#/explore?rival=" + channelID }

// syncTask ghi/cập nhật dòng việc nền của một job qua đúng một lời gọi kho
// nguyên tử; kho khoá việc hiện tại nên state/phase/action luôn khớp việc,
// không phụ thuộc ảnh chụp của nơi gọi. Lần ghi tiến độ bị giới hạn ở chính
// writer này (≤ 4/s/kênh).
func (s *Service) syncTask(ctx context.Context, job *Job, mode taskSync) {
	if s.d.Tasks == nil || job == nil {
		return
	}
	switch job.State {
	case domain.ScanComplete, domain.ScanCancelled, domain.ScanCapped:
		if job.TaskID != "" {
			_ = s.d.Tasks.Finish(ctx, job.TaskID)
			_ = s.d.Store.ClearTaskLink(ctx, job.ID)
		}
		return
	}
	var channel domain.RivalChannel
	if ch, err := s.d.Store.Channel(ctx, job.ChannelID); err == nil {
		channel = ch
	}
	title := job.ChannelID
	if channel.Title != "" {
		title = channel.Title
	}
	spec := taskSpecOf(job, title)
	if _, live := TaskPresentation(*job); !live {
		return
	}
	// Một lần xin ngân sách duy nhất cho mọi đường ghi tiến độ của lượt này:
	// dòng mới cần con số khởi tạo; các lần sau chỉ khi mode.progress. nil =
	// kho giữ nguyên con số đang hiện.
	var progress *domain.TaskProgress
	if job.TaskID == "" || mode.progress {
		progress = s.progressOf(ctx, job, true)
	}
	spec.Progress = progress

	// MỌI lần ghi state/phase/action đi qua đúng một lời gọi kho nguyên tử:
	// kho khoá việc hiện tại, tự suy lại cách hiện (TaskPresentation), ghi
	// dòng + nối việc và thông báo lỗi (nếu đây là lần chuyển sang failed)
	// trong cùng giao dịch. Vì vậy một ảnh chụp "đang chạy"/"lỗi" cũ không
	// thể dựng lại dòng hay phát thông báo lỗi sau khi việc đã đổi trạng thái.
	res, err := s.d.Store.SyncScanTask(ctx, job.ID, job.ChannelID, spec)
	if err != nil {
		return // job_stopped/kênh mất: không phát sự kiện cho dòng không ghi được
	}
	s.d.Tasks.Changed(res.TaskID)
	if res.NotificationID != "" {
		s.d.Tasks.Changed(res.NotificationID)
	}
}

// progressOf trả tiến độ khi được phép và đã đủ khoảng cách (≤ 4/s/kênh).
func (s *Service) progressOf(ctx context.Context, job *Job, allow bool) *domain.TaskProgress {
	if !allow || job.Kind != domain.ScanHistory {
		return nil
	}
	s.mu.Lock()
	if last, ok := s.progress[job.ChannelID]; ok && s.now().Sub(last) < ProgressInterval {
		s.mu.Unlock()
		return nil
	}
	s.progress[job.ChannelID] = s.now()
	s.mu.Unlock()
	total := int64(domain.HistoryCapDefault)
	done := int64(0)
	if st, err := s.d.Store.EnsureState(ctx, job.ChannelID); err == nil {
		done = int64(st.IndexedVideos)
		total = int64(st.HistoryCap)
	}
	return &domain.TaskProgress{Done: done, Total: total}
}

// refreshTask đồng bộ dòng việc nền của việc còn sống thuộc một loại.
func (s *Service) refreshTask(ctx context.Context, channelID, kind string) {
	if s.d.Tasks == nil {
		return
	}
	snap, err := s.d.Store.Status(ctx, channelID)
	if err != nil {
		return
	}
	var job *Job
	if kind == domain.ScanDiscovery {
		job = snap.DiscoveryJob
	} else {
		job = snap.HistoryJob
	}
	if job == nil || !job.Live() {
		if job != nil && job.TaskID != "" {
			_ = s.d.Tasks.Finish(ctx, job.TaskID)
			_ = s.d.Store.ClearTaskLink(ctx, job.ID)
		}
		return
	}
	s.syncTask(ctx, job, taskSync{})
}

// ── ánh lỗi ─────────────────────────────────────────────────────────────────

// mapProviderError đổi lỗi của youtube.Client thành *Error theo hợp đồng API.
func (s *Service) mapProviderError(err error) error {
	var deferred *youtube.DeferredError
	if errors.As(err, &deferred) {
		at := deferred.RetryAt
		return &Error{Code: CodeRateLimited, Message: err.Error(), RetryAt: &at}
	}
	if kind, ok := youtube.KindOf(err); ok {
		switch kind {
		case youtube.KindRateLimited:
			var fe *youtube.FetchError
			var at *time.Time
			if errors.As(err, &fe) && fe.RetryDelay != nil {
				at = new(s.now().Add(*fe.RetryDelay))
			}
			return &Error{Code: CodeRateLimited, Message: err.Error(), RetryAt: at}
		case youtube.KindBlocked:
			return &Error{Code: CodeBlocked, Message: err.Error(), Recovery: RecoveryBlocked}
		case youtube.KindNotFound:
			return &Error{Code: CodeNotFound, Message: "Không tìm thấy kênh YouTube này"}
		case youtube.KindTransient:
			return &Error{Code: CodeTransient, Message: err.Error()}
		default:
			return &Error{Code: CodeUnexpectedPage, Message: err.Error(), Recovery: RecoveryFatal}
		}
	}
	var notFound *youtube.ChannelNotFoundError
	if errors.As(err, &notFound) {
		return &Error{Code: CodeNotFound, Message: "Không tìm thấy kênh YouTube này"}
	}
	var blocked *youtube.UploadTimeBlockedError
	if errors.As(err, &blocked) {
		return &Error{Code: CodeBlocked, Message: err.Error(), Recovery: RecoveryBlocked}
	}
	return &Error{Code: CodeTransient, Message: err.Error()}
}

// mapStoreError đổi lỗi kho (fence, việc, kênh) thành *Error.
func (s *Service) mapStoreError(channelID string, err error) error {
	switch StoreCodeOf(err) {
	case StoreChannelMissing:
		return &Error{Code: CodeNotFound, Message: "Không tìm thấy kênh đối thủ"}
	case StoreChannelExists:
		if ch, rerr := s.d.Store.Channel(ctxBackground(), channelID); rerr == nil {
			return existsError(ch)
		}
		return &Error{Code: CodeExists, Message: "Đã theo dõi kênh này"}
	case StoreLeaseLost:
		return &Error{Code: CodeLeaseBusy, Message: "Kênh đang được quét; hãy thử lại khi trang hiện tại hoàn tất"}
	case "":
		return err
	default:
		return &Error{Code: CodeTransient, Message: err.Error()}
	}
}

// ctxBackground tách truy vấn đọc lại khỏi ctx đã huỷ của request (thông tin
// chỉ để dựng lỗi exists).
func ctxBackground() context.Context { return context.Background() }
