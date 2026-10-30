// Package toolsvc là tầng dịch vụ trên internal/tools cho mục Cài đặt ›
// Công cụ trên máy và màn Chuẩn bị máy: đánh giá trạng thái từng việc
// (tên theo việc làm, không theo binary), nhận diện cập nhật yt-dlp, và
// điều phối cài/sửa dưới dạng việc nền có tiến độ.
//
// Kết quả đánh giá được cache trong bộ nhớ với TTL ngắn vì probe spawn
// process: GET phục vụ từ cache, refresh tường minh (nút Kiểm tra, sau
// mỗi lượt cài, lúc khởi động) mới chạy probe. Mọi đổi trạng thái phát
// notify(ref) — web đẩy SSE "tools".
package toolsvc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// ToolID là id hàng công cụ — trùng id bước của Chuẩn bị máy.
type ToolID = domain.SetupStepID

const (
	ToolYtDlp         ToolID = domain.SetupStepYtDlp
	ToolFfmpeg        ToolID = domain.SetupStepFfmpeg
	ToolTranscription ToolID = domain.SetupStepTranscription
	ToolChrome        ToolID = domain.SetupStepChrome
	ToolCodex         ToolID = domain.SetupStepCodex
	ToolCodexLogin    ToolID = domain.SetupStepCodexLogin
)

// ToolIDs là sáu hàng theo đúng thứ tự của design.
var ToolIDs = domain.SetupStepIDs

// ToolState là nhãn trạng thái của một hàng theo tweak tools của artboard
// Settings: ready | update | missing | failed | installing | needs_login.
type ToolState = string

const (
	StateReady      ToolState = "ready"
	StateUpdate     ToolState = "update"
	StateMissing    ToolState = "missing"
	StateFailed     ToolState = "failed"
	StateInstalling ToolState = "installing"
	StateNeedsLogin ToolState = "needs_login"
)

// UpdateInfo là bản mới có sẵn của một công cụ.
type UpdateInfo struct {
	Current string
	Latest  string
}

// Row là một hàng của Cài đặt › Công cụ trên máy.
type Row struct {
	ID       ToolID
	State    ToolState
	Hint     string // câu phụ tiếng Việt (gồm phiên bản/nhắc cập nhật)
	Version  string // chỉ hiện trong Chi tiết
	Update   *UpdateInfo
	Progress *domain.InstallProgress
	Detail   string // đường dẫn binary / lỗi kỹ thuật cho Chi tiết
	RetryAt  *time.Time
	// SizeBytes là dung lượng bước này chiếm khi cài xong (0 = không đo được).
	SizeBytes int64
	// Provided = công cụ có sẵn trên máy, app không phải tải (Chrome hệ thống).
	Provided bool
}

// Snapshot là hình của GET /api/tools.
type Snapshot struct {
	Tools     []Row
	CheckedAt time.Time
}

// Tasks là interface việc nền của gói nghiệp vụ (internal/activity thoả).
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
}

// ChatGPTSession đọc kiểu đăng nhập ChatGPT qua kho phiên của AIConn
// (giá trị theo tools.CodexLogin: "chatgpt" | "api_key" | "not_logged_in" |
// "unknown"). Nil → fallback sang trạng thái phiên của binary Codex.
type ChatGPTSession interface {
	ChatGPTLogin(ctx context.Context) (string, error)
}

// Logger là điểm ghi nhật ký công cụ (internal/logs.Store thoả: mức là
// chuỗi info|warn|error).
type Logger interface {
	Append(tool string, level string, message string)
}

// probeTTL là tuổi tối đa của snapshot cache khi GET — probe spawn process
// nên refresh chỉ chạy khi cache cũ, có yêu cầu tường minh, hoặc sau cài.
const probeTTL = 5 * time.Minute

// updateCheckTTL là nhịp kiểm phiên bản yt-dlp mới trên GitHub.
const updateCheckTTL = 6 * time.Hour

// Deps là phụ thuộc của Service. Tasks là bắt buộc cho cài đặt; phần còn lại
// nil = tắt tính năng đó (không notify, không log, không phiên ChatGPT).
type Deps struct {
	Tools         *tools.Tools
	Managed       *tools.Managed
	Transcription *tools.Transcription
	Tasks         Tasks
	Log           Logger
	Notify        func(ref string)
	ChatGPT       ChatGPTSession
	Now           func() time.Time
	// DataDir là thư mục app cài công cụ và model (hiện ở vùng Nâng cao).
	DataDir string
}

// Service điều phối trạng thái và cài đặt công cụ.
type Service struct {
	tools   *tools.Tools
	mgr     *tools.Managed
	trans   *tools.Transcription
	tasks   Tasks
	log     Logger
	notify  func(ref string)
	chatgpt ChatGPTSession
	now     func() time.Time
	dataDir string

	mu         sync.Mutex
	wg         sync.WaitGroup  // lượt cài nền đang chạy
	baseCtx    context.Context // vòng đời serve; mọi goroutine nền đi theo
	cached     *Snapshot
	checkedAt  time.Time
	latest     map[ToolID]latestCheck // kết quả kiểm cập nhật theo công cụ
	running    map[ToolID]*installRun
	refreshing chan struct{} // non-nil: một lượt refresh đang chạy
}

type latestCheck struct {
	latest string
	at     time.Time
	err    error
}

// New dựng service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{
		tools: d.Tools, mgr: d.Managed, trans: d.Transcription,
		tasks: d.Tasks, log: d.Log, notify: d.Notify,
		chatgpt: d.ChatGPT, now: d.Now, dataDir: d.DataDir,
		baseCtx: context.Background(),
		latest:  map[ToolID]latestCheck{}, running: map[ToolID]*installRun{},
	}
}

// DataDir là thư mục app cài công cụ và model.
func (s *Service) DataDir() string { return s.dataDir }

func (s *Service) emit(ref string) {
	if s.notify != nil {
		s.notify(ref)
	}
}

func (s *Service) logf(tool ToolID, level, format string, args ...any) {
	if s.log != nil {
		s.log.Append(tool, level, fmt.Sprintf(format, args...))
	}
}

// Snapshot trả hình công cụ. fresh=true buộc probe lại (nút Kiểm tra). Mặc
// định trả cache còn hạn; cache cũ thì trả cache đó và probe lại trong nền —
// GET không bao giờ chờ probe. Chưa từng probe → probe đồng bộ một lần.
func (s *Service) Snapshot(ctx context.Context, fresh bool) (Snapshot, error) {
	if fresh {
		return s.Refresh(ctx)
	}
	s.mu.Lock()
	cached := s.cached
	if cached == nil {
		// Chưa probe lần nào: trả hình "chưa cài" và probe ở nền — GET không
		// spawn process.
		initial := Snapshot{Tools: make([]Row, len(ToolIDs))}
		for i, id := range ToolIDs {
			initial.Tools[i] = Row{ID: id, State: StateMissing, SizeBytes: s.ApproxBytes(id)}
		}
		s.startRefreshLocked()
		s.mu.Unlock()
		return initial, nil
	}
	stale := s.checkedAt.IsZero() || s.now().Sub(s.checkedAt) > probeTTL
	snap := cloneSnapshot(*cached)
	if stale {
		s.startRefreshLocked()
	}
	s.mu.Unlock()
	return s.applyLive(snap), nil
}

// cloneSnapshot sao chép slice hàng để overlay không sửa cache dùng chung
// (nhiều handler encode đồng thời).
func cloneSnapshot(s Snapshot) Snapshot {
	s.Tools = append([]Row(nil), s.Tools...)
	return s
}

// Refresh probe lại mọi công cụ và cập nhật cache; refresh đồng thời gộp
// vào lượt đang chạy (probe spawn process nên không lặp).
func (s *Service) Refresh(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	if s.refreshing != nil {
		wait := s.refreshing
		s.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		}
		s.mu.Lock()
		snap := cloneSnapshot(*s.cached)
		s.mu.Unlock()
		return s.applyLive(snap), nil
	}
	done := make(chan struct{})
	s.refreshing = done
	s.mu.Unlock()

	snap := s.probe(ctx)

	s.mu.Lock()
	s.cached = &snap
	s.checkedAt = s.now()
	s.refreshing = nil
	close(done)
	s.mu.Unlock()
	return s.applyLive(cloneSnapshot(snap)), nil
}

// startRefreshLocked khởi một refresh nền khi cache cũ (caller giữ mu);
// đổi trạng thái phát SSE sau khi cache mới được ghi.
func (s *Service) startRefreshLocked() {
	if s.refreshing != nil {
		return
	}
	done := make(chan struct{})
	s.refreshing = done
	base := s.baseCtx
	go func() {
		ctx, cancel := context.WithTimeout(base, 3*time.Minute)
		defer cancel()
		snap := s.probe(ctx)
		s.mu.Lock()
		s.cached = &snap
		s.checkedAt = s.now()
		s.refreshing = nil
		close(done)
		s.mu.Unlock()
		s.emit("")
	}()
}

// Warm bắt đầu probe nền sau khởi động để GET đầu tiên không chờ; ctx là
// vòng đời serve — mọi lượt refresh nền dừng khi ctx huỷ.
func (s *Service) Warm(ctx context.Context) {
	s.mu.Lock()
	if ctx != nil {
		s.baseCtx = ctx
	}
	s.startRefreshLocked()
	s.mu.Unlock()
}

// applyLive phủ tiến độ trực tiếp (install-state.json của lượt đang chạy)
// lên snapshot: hàng "installing" luôn đúng giữa hai lần probe.
func (s *Service) applyLive(snap Snapshot) Snapshot {
	// Trạng thái đăng nhập đọc từ phiên trong bộ nhớ (rẻ, không spawn) nên
	// luôn mới hơn cache probe — đăng nhập xong là hàng đổi ngay.
	login := ""
	if s.chatgpt != nil {
		if v, err := s.chatgpt.ChatGPTLogin(context.Background()); err == nil {
			login = v
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range snap.Tools {
		if snap.Tools[i].ID == ToolCodexLogin && login != "" {
			switch login {
			case "chatgpt", "api_key":
				snap.Tools[i].State = StateReady
				snap.Tools[i].Hint = ""
			case "not_logged_in":
				snap.Tools[i].State = StateNeedsLogin
			}
		}
	}
	for i := range snap.Tools {
		id := snap.Tools[i].ID
		if run, ok := s.running[id]; ok && run.alive() {
			snap.Tools[i].State = StateInstalling
			snap.Tools[i].Progress = liveProgress(s.mgr, id)
			snap.Tools[i].Hint = "Đang cài…"
		}
		if u, ok := s.latest[id]; ok && u.err == nil && u.latest != "" {
			snap.Tools[i] = s.applyUpdateInfo(snap.Tools[i], u.latest)
		}
	}
	return snap
}
