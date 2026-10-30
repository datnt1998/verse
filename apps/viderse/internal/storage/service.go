package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// WatchEvery mặc định: nhịp theo dõi ổ (design yêu cầu phát hiện trong ≤10s;
// mỗi nhịp chỉ là stat + access trên vài thư mục — rẻ).
const DefaultWatchEvery = 2 * time.Second

// Deps là các phụ thuộc của Service — mọi thứ chạm ngoài (ổ đĩa, DB settings,
// việc nền, thông báo, mở Finder, đồng hồ) đều tiêm qua đây.
type Deps struct {
	Home      string // thư mục nhà (respect HOME)
	ConfigDir string // ~/.viderse — thư mục luôn ở máy này
	// DefaultDir là thư mục dự án mặc định; trống → ConfigDir/projects.
	DefaultDir string
	Settings   Settings
	Volumes    Volumes
	Tasks      Tasks
	Notify     Notifications
	Opener     Opener
	// Emit phát sự kiện SSE "storage" (ref = id việc khi là tiến độ chuyển).
	Emit func(ref string)
	// Bump làm mới trạng thái đầu trang (status.Service.Bump).
	Bump       func()
	Clock      Clock
	WatchEvery time.Duration
	// NewTicker tiêm ticker cho watch — test đẩy nhịp bằng tay.
	NewTicker func(d time.Duration) Ticker
}

// Service là cụm nghiệp vụ của Cài đặt › Lưu trữ. Mọi phương thức an toàn
// cho goroutine; các việc dài chạy dưới context của Start.
type Service struct {
	deps  Deps
	clock Clock

	mu       sync.Mutex
	location string
	altRoots []string
	previous *domain.StoragePrevious
	holds    map[domain.Hold]bool
	missing  map[string]time.Time // root → lúc phát hiện mất
	volNames map[string]string    // root → tên ổ lần cuối đọc được
	moving   *moveState
	failed   *domain.StorageMoveFailed

	watchMu sync.Mutex
	watch   Ticker

	// Vòng đời: worker (chuyển thư mục) chạy dưới life; Shutdown huỷ và chờ.
	lifeMu     sync.Mutex
	life       context.Context
	lifeCancel context.CancelFunc
	workers    sync.WaitGroup

	// Số đo dung lượng: chỉ đọc trong request, đo ở nền (refreshUsageLoop).
	usageMu     sync.Mutex
	usageWalkMu sync.Mutex
	usageAt     time.Time
	usageUsed   int64
	usageList   []UsageGroup
	stayBytes   int64
	usageDirty  bool
	usageWake   chan struct{}
	pendingRefs []string
}

// New nạp trạng thái đã lưu (vị trí, bản cũ, altRoots) — một lần đọc settings
// khi dựng, không nền. Chưa khởi động watch/phục hồi: gọi Start.
func New(ctx context.Context, deps Deps) (*Service, error) {
	if deps.DefaultDir == "" {
		deps.DefaultDir = filepath.Join(deps.ConfigDir, "projects")
	}
	clock := deps.Clock
	if clock == nil {
		clock = systemClock{}
	}
	s := &Service{
		deps: deps, clock: clock,
		holds: map[domain.Hold]bool{}, missing: map[string]time.Time{},
		volNames: map[string]string{}, usageWake: make(chan struct{}, 1),
	}
	s.life, s.lifeCancel = context.WithCancel(context.Background())
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	// Ẩm snapshot ổ đĩa trước khi phục vụ: phân loại diskutil chạy ở đây, GET
	// /api/storage chỉ đọc bộ nhớ + một statfs.
	if r, ok := deps.Volumes.(volumeRefresher); ok {
		r.refreshVolumes(ctx)
	}
	// Đo lần đầu ngay khi dựng (một lần, có giới hạn) để request đầu tiên đã
	// có số; sau đó vòng nền giữ số mới.
	s.forceUsage(ctx)
	return s, nil
}

// load đọc vị trí, bản cũ và altRoots từ settings.
func (s *Service) load(ctx context.Context) error {
	var location string
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageLocation, &location); err != nil {
		return err
	} else if !found || location == "" {
		location = s.deps.DefaultDir
	}
	if resolved, err := canonicalPath(location, s.deps.Home); err == nil {
		location = resolved
	}
	var previous *domain.StoragePrevious
	var stored domain.StoragePrevious
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStoragePrevious, &stored); err != nil {
		return err
	} else if found && stored.Path != "" {
		previous = &stored
	}
	var altRoots []string
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageAltRoots, &altRoots); err != nil {
		return err
	} else if !found {
		altRoots = nil
	}
	s.location, s.previous, s.altRoots = location, previous, altRoots
	var holds domain.StorageHolds
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageHolds, &holds); err != nil {
		return err
	} else if found {
		for _, h := range holds {
			s.holds[h] = true
		}
	}
	var failed domain.StorageMoveFailed
	if found, err := s.deps.Settings.Get(ctx, domain.KeyStorageMoveFailed, &failed); err != nil {
		return err
	} else if found && failed.Target != "" {
		s.failed = &failed
	}
	return nil
}

// volumeRefresher is implemented by the OS volume snapshot (CachedVolumes):
// refreshVolumes does the expensive classification off the request path and
// reports whether the mount set changed (plug/unplug).
type volumeRefresher interface {
	refreshVolumes(ctx context.Context) bool
}

// Start đảm bảo thư mục hiện tại tồn tại, phục hồi một lần chuyển bị cắt
// ngang, xoá bản cũ đã quá hạn và chạy vòng theo dõi ổ. Trả khi ctx kết thúc
// (được gọi trong goroutine của serve).
func (s *Service) Start(ctx context.Context) error {
	s.lifeMu.Lock()
	s.lifeCancel()
	s.life, s.lifeCancel = context.WithCancel(ctx)
	s.lifeMu.Unlock()
	// Phục hồi TRƯỚC ensureLocation: khởi động sau một lần rename bị cắt ngang
	// phải nhận đích làm vị trí thay vì dựng thư mục rỗng ở nguồn.
	s.recoverMove(ctx)
	s.reconcileHolds(ctx)
	if err := s.ensureLocation(ctx); err != nil {
		slog.Warn("storage: thư mục dự án không tạo được", "path", s.location, "err", err)
	}
	s.tick(ctx) // đánh giá online/offline ngay khi mở

	every := s.deps.WatchEvery
	if every <= 0 {
		every = DefaultWatchEvery
	}
	newTicker := s.deps.NewTicker
	if newTicker == nil {
		newTicker = func(d time.Duration) Ticker { return realTicker{time.NewTicker(d)} }
	}
	s.watchMu.Lock()
	s.watch = newTicker(every)
	watch := s.watch
	s.watchMu.Unlock()
	defer watch.Stop()

	// Số đo dung lượng do vòng nền cập nhật; request không bao giờ đi cây.
	go s.refreshUsageLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-watch.C():
			s.tick(ctx)
		}
	}
}

// Stop dừng vòng watch (khi Start không được gọi trong goroutine riêng).
func (s *Service) Stop() {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if s.watch != nil {
		s.watch.Stop()
	}
}

// lifetime trả context của service (không của request) cho việc nền.
func (s *Service) lifetime() context.Context {
	s.lifeMu.Lock()
	defer s.lifeMu.Unlock()
	return s.life
}

// Shutdown huỷ lần chuyển đang chạy và CHỜ worker dừng (huỷ qua ctx của
// server xong mới gọi). Nguồn/dữ liệu không đổi; lần sau mở phục hồi từ nhật
// ký. Nhật ký và task vẫn ghi tử tế nên không mất trạng thái.
func (s *Service) Shutdown(ctx context.Context) error {
	s.lifeMu.Lock()
	s.lifeCancel()
	s.lifeMu.Unlock()
	s.mu.Lock()
	if s.moving != nil && s.moving.cancel != nil {
		s.moving.cancel()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

/* ------------------------------ resolver ------------------------------ */

// Root trả thư mục dự án hiện tại (đường ghi). Thay đổi ngay khi người dùng
// đổi thư mục — không cần khởi động lại.
func (s *Service) Root() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.location
}

// Resolve đổi một đường dẫn tương đối (như lưu trong DB) thành tuyệt đối dưới
// thư mục hiện tại. Đường dẫn thoát khỏi root (.., tuyệt đối) bị từ chối.
func (s *Service) Resolve(rel string) (string, error) {
	if rel == "" {
		return "", codeError(ErrInvalidPath, "đường dẫn rỗng")
	}
	if filepath.IsAbs(rel) {
		return "", codeError(ErrInvalidPath, "đường dẫn trong DB phải là tương đối: %q", rel)
	}
	root := s.Root()
	joined := filepath.Join(root, rel)
	rootWithSep := root + string(filepath.Separator)
	if joined != root && !filepath.HasPrefix(joined, rootWithSep) {
		return "", codeError(ErrInvalidPath, "đường dẫn thoát khỏi thư mục dự án: %q", rel)
	}
	return joined, nil
}

// Candidates trả các đường dẫn đọc một file tương đối, theo thứ tự ưu tiên:
// thư mục hiện tại → các thư mục cũ (new_only) → bản cũ sau chuyển. Caller
// dùng cái đầu tiên tồn tại.
func (s *Service) Candidates(rel string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := append([]string{s.location}, s.altRoots...)
	if s.previous != nil {
		roots = append(roots, s.previous.Path)
	}
	var out []string
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		if filepath.IsAbs(rel) || rel == "" {
			return nil, codeError(ErrInvalidPath, "đường dẫn trong DB phải là tương đối: %q", rel)
		}
		joined := filepath.Join(root, rel)
		if joined != root && !filepath.HasPrefix(joined, root+string(filepath.Separator)) {
			return nil, codeError(ErrInvalidPath, "đường dẫn thoát khỏi thư mục dự án: %q", rel)
		}
		out = append(out, joined)
	}
	return out, nil
}

// GroupPath trả đường dẫn tuyệt đối của một nhóm trong thư mục hiện tại.
func (s *Service) GroupPath(group Group) (string, error) {
	switch group {
	case GroupImages, GroupEditor, GroupSources, GroupExports:
	default:
		return "", codeError(ErrInvalidPath, "nhóm không biết: %q", group)
	}
	return filepath.Join(s.Root(), group), nil
}

// Roots trả vị trí hiện tại và mọi root đọc được (để watcher/quét dữ liệu).
func (s *Service) Roots() (current string, all []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all = append([]string{s.location}, s.altRoots...)
	if s.previous != nil {
		all = append(all, s.previous.Path)
	}
	return s.location, all
}

/* ------------------------------- holds ------------------------------- */

// Holds trả các lý do đang giữ việc làm video (scheduler hỏi trước khi bắt
// đầu bước mới; hiệu lực tạm dừng = cờ người dùng || storage.queuePaused).
func (s *Service) Holds() []domain.Hold {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Hold, 0, len(s.holds))
	for h := range s.holds {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// acquireHold đánh dấu một lý do giữ. Tập hold persist (storage.holds) và cờ
// riêng storage.queuePaused — không đụng clone.queuePaused của người dùng,
// nên người tạm dừng bằng tay không bị bật lại khi hold nhả.
func (s *Service) acquireHold(ctx context.Context, h domain.Hold) {
	s.mu.Lock()
	if s.holds[h] {
		s.mu.Unlock()
		return
	}
	s.holds[h] = true
	holds := s.holdsList()
	s.mu.Unlock()

	s.persistHolds(ctx, holds)
	s.emit("")
	s.bump()
}

// releaseHold gỡ một lý do giữ; hết hold thì hạ storage.queuePaused.
func (s *Service) releaseHold(ctx context.Context, h domain.Hold) {
	s.mu.Lock()
	if !s.holds[h] {
		s.mu.Unlock()
		return
	}
	delete(s.holds, h)
	holds := s.holdsList()
	s.mu.Unlock()

	s.persistHolds(ctx, holds)
	s.emit("")
	s.bump()
}

// holdsList chụp tập hold (gọi với s.mu đã giữ).
func (s *Service) holdsList() domain.StorageHolds {
	out := make(domain.StorageHolds, 0, len(s.holds))
	for h := range s.holds {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// persistHolds ghi tập hold + cờ paus của storage (lỗi chỉ log — in-memory
// vẫn đúng, lần ghi sau sẽ sửa).
func (s *Service) persistHolds(ctx context.Context, holds domain.StorageHolds) {
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageHolds, holds); err != nil {
		slog.Warn("storage: ghi holds lỗi", "err", err)
	}
	if err := s.deps.Settings.Set(ctx, domain.KeyStoragePaused, len(holds) > 0); err != nil {
		slog.Warn("storage: ghi cờ tạm dừng lỗi", "err", err)
	}
}

// reconcileHolds dọn hold không còn căn cứ sau sập: HoldMoving của một lần
// chuyển bị cắt ngang (worker không còn) bị gỡ; hold còn lại giữ nguyên —
// chúng là bản ghi có căn cứ (offline/use_default) và cờ paus đã persist.
func (s *Service) reconcileHolds(ctx context.Context) {
	s.mu.Lock()
	delete(s.holds, domain.HoldMoving)
	holds := s.holdsList()
	s.mu.Unlock()
	s.persistHolds(ctx, holds)
}

func (s *Service) emit(ref string) {
	if s.deps.Emit != nil {
		s.deps.Emit(ref)
	}
}

func (s *Service) bump() {
	if s.deps.Bump != nil {
		s.deps.Bump()
	}
}

/* ------------------------------ location ------------------------------ */

// ensureLocation tạo thư mục hiện tại nếu thiếu và ghi marker. Lỗi (ổ đang
// rút) chỉ bị ghi log — watch sẽ báo offline.
func (s *Service) ensureLocation(ctx context.Context) error {
	root := s.Root()
	if !volumePresent(root) {
		return codeError(ErrVolumeGone, "ổ chứa %s đang không cắm", root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return writeRootMarker(root)
}

// writeRootMarker ghi tệp đánh dấu gốc dự án (idempotent).
func writeRootMarker(root string) error {
	marker := filepath.Join(root, RootMarker)
	if _, err := os.Lstat(marker); err == nil {
		return nil
	}
	data := fmt.Appendf(nil, `{"app":"viderse","version":1}`+"\n")
	return os.WriteFile(marker, data, 0o644)
}

// setLocation đổi thư mục hiện tại (ghi settings + bộ nhớ). previous/altRoots
// do caller ghi trước để một lần sập ở giữa không mất dấu dữ liệu.
func (s *Service) setLocation(ctx context.Context, path string) error {
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageLocation, path); err != nil {
		return err
	}
	s.mu.Lock()
	s.location = path
	s.mu.Unlock()
	s.markUsageDirty()
	return nil
}

// addAltRoot ghi một thư mục cũ còn dữ liệu đang dùng (new_only / đổi về
// thư mục mặc định khi ổ rút).
func (s *Service) addAltRoot(ctx context.Context, path string) error {
	s.mu.Lock()
	roots := append([]string(nil), s.altRoots...)
	found := false
	for _, r := range roots {
		if r == path {
			found = true
		}
	}
	// Không lọc theo vị trí hiện tại: ghi altRoot TRƯỚC khi đổi vị trí để sập
	// ở giữa vẫn đọc được dữ liệu cũ; resolver tự bỏ trùng.
	if !found && path != "" {
		roots = append(roots, path)
	}
	s.altRoots = roots
	s.mu.Unlock()
	return s.deps.Settings.Set(ctx, domain.KeyStorageAltRoots, roots)
}

// setPrevious ghi/bỏ bản cũ giữ 7 ngày.
func (s *Service) setPrevious(ctx context.Context, prev *domain.StoragePrevious) error {
	if prev != nil && prev.Mode == "" {
		prev.Mode = ModeMove
	}
	var err error
	if prev == nil {
		err = s.deps.Settings.Clear(ctx, domain.KeyStoragePrevious)
	} else {
		err = s.deps.Settings.Set(ctx, domain.KeyStoragePrevious, prev)
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.previous = prev
	s.mu.Unlock()
	return nil
}

/* -------------------------------- check ------------------------------- */

func (s *Service) resolveTarget(ctx context.Context, path string) (target string, createSubdir bool, vol Volume, err error) {
	abs, err := canonicalPath(path, s.deps.Home)
	if err != nil {
		return "", false, Volume{}, err
	}
	vol, err = s.deps.Volumes.Info(ctx, abs)
	if err != nil {
		return "", false, Volume{}, err
	}
	if abs == vol.Mount {
		return filepath.Join(abs, ProjectsFolderName), true, vol, nil
	}
	return abs, false, vol, nil
}

// validateTarget là kiểm tra duy nhất cho MỌI lần đổi thư mục (move và
// new_only): chặn đích lồng/quan hệ họ hàng với nguồn, chặn vùng hệ thống và
// home/config, và từ chối thư mục không-trống mà Viderse không sở hữu. Một
// thư mục lạ không bao giờ "thành của Viderse" chỉ bằng marker — bản cũ của
// nó sẽ bị xoá đệ quy về sau nếu không chặn ngay đây.
func (s *Service) validateTarget(ctx context.Context, target string) error {
	current := s.Root()
	switch {
	case underDir(target, current):
		return codeError(ErrInvalidPath, "thư mục đích nằm trong thư mục dự án hiện tại")
	case underDir(current, target):
		return codeError(ErrInvalidPath, "thư mục đích chứa thư mục dự án hiện tại")
	case target == s.deps.Home:
		return codeError(ErrInvalidPath, "thư mục nhà không dùng làm thư mục dự án")
	case target != s.deps.DefaultDir && underDir(target, s.deps.ConfigDir):
		return codeError(ErrInvalidPath, "thư mục này nằm trong dữ liệu cấu hình của Viderse")
	}
	for _, sys := range []string{"/System", "/Library", "/bin", "/sbin", "/etc", "/usr", "/Volumes", "/dev", "/opt", "/private/etc"} {
		if target == sys || underDir(target, sys) {
			return codeError(ErrInvalidPath, "không đặt thư mục dự án trong %s", sys)
		}
	}
	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):
		return nil // chưa có: sẽ tạo
	case err != nil:
		return err
	case !info.IsDir():
		return codeError(ErrInvalidPath, "%s không phải thư mục", target)
	}
	empty, err := dirEmpty(target)
	if err != nil {
		return err
	}
	if empty || hasRootMarker(target) {
		return nil // rỗng, hoặc thư mục dự án Viderse có sẵn (move-back)
	}
	return codeError(ErrDestNotEmpty, "thư mục không trống và không phải thư mục dự án của Viderse")
}

// hasRootMarker báo thư mục đã là gốc dự án Viderse (marker do chính app ghi).
func hasRootMarker(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, RootMarker))
	return err == nil && !info.IsDir()
}

// Check chạy các kiểm tra của POST /api/storage/check trước khi cho bấm.
func (s *Service) Check(ctx context.Context, path string, mode MoveMode) (CheckResult, error) {
	if mode != ModeMove && mode != ModeNewOnly {
		return CheckResult{}, codeError(ErrInvalidPath, "mode phải là move hoặc new_only")
	}
	target, createSubdir, vol, err := s.resolveTarget(ctx, path)
	if err != nil {
		return CheckResult{}, err
	}
	if !volumePresent(target) {
		return CheckResult{}, codeError(ErrVolumeGone, "ổ của %s đang không cắm", target)
	}
	current := s.Root()
	// Ranh giới an toàn trước mọi nút "chuyển": lồng trong thư mục hiện tại,
	// vùng hệ thống, hay đích không trống mà không phải của Viderse đều là
	// hàng fail rõ ràng (không phải lỗi server).
	targetErr := error(nil)
	if target != current {
		targetErr = s.validateTarget(ctx, target)
	}
	checks := make([]Check, 0, 6)
	if targetErr != nil {
		checks = append(checks, Check{Kind: CheckPath, Status: CheckFail, Text: targetErr.Error()})
		return CheckResult{OK: false, Path: target, CreateSubdir: createSubdir, Checks: checks}, nil
	}
	// Check là thao tác người dùng: đo lại một lần (có giới hạn) thay vì dùng
	// số nền có thể cũ tới 5 phút.
	s.forceUsage(ctx)
	used := s.usageOf().used

	var needBytes int64
	if mode == ModeMove {
		needBytes = int64(float64(used) * 1.1)
	} else {
		needBytes = 20 << 30 // chỉ video mới: cần sẵn 20 GB
	}

	// same_volume (chỉ ý nghĩa khi move; new_only báo nhẹ).
	curVol, curErr := s.deps.Volumes.Info(ctx, current)
	sameVolume := curErr == nil && curVol.fsid() == vol.fsid()
	if mode == ModeMove {
		if sameVolume {
			checks = append(checks, Check{Kind: CheckSameVolume, Status: CheckOK,
				Text: fmt.Sprintf("Cùng ổ %s: chuyển trong vài giây, không tốn thêm chỗ", curVol.Name)})
		} else {
			checks = append(checks, Check{Kind: CheckSameVolume, Status: CheckOK,
				Text: fmt.Sprintf("Khác ổ: sẽ chép %s sang %s", formatBytes(used), vol.Name)})
		}
	}

	// cloud — từ chối hẳn.
	if isCloudSyncPath(target, s.deps.Home) || vol.Kind == VolumeCloudSync {
		checks = append(checks, Check{Kind: CheckCloud, Status: CheckFail,
			Text: "Thư mục đồng bộ đám mây: không dùng được"})
	} else {
		checks = append(checks, Check{Kind: CheckCloud, Status: CheckOK,
			Text: "Không nằm trong thư mục đồng bộ đám mây"})
	}

	// space.
	switch {
	case sameVolume && mode == ModeMove:
		checks = append(checks, Check{Kind: CheckSpace, Status: CheckOK,
			Text: "Chuyển cùng ổ: không tốn thêm chỗ"})
	case vol.FreeBytes >= needBytes:
		text := fmt.Sprintf("Đủ chỗ: cần %s (gồm 10%% dự phòng), ổ còn %s", formatBytes(needBytes), formatBytes(vol.FreeBytes))
		if mode == ModeNewOnly {
			text = fmt.Sprintf("Đủ chỗ: video mới cần sẵn %s, ổ còn %s", formatBytes(needBytes), formatBytes(vol.FreeBytes))
		}
		checks = append(checks, Check{Kind: CheckSpace, Status: CheckOK, Text: text})
	default:
		text := fmt.Sprintf("Không đủ chỗ: cần %s, ổ chỉ còn %s", formatBytes(needBytes), formatBytes(vol.FreeBytes))
		if mode == ModeNewOnly {
			text = fmt.Sprintf("Không đủ chỗ cho video mới: cần sẵn %s, ổ chỉ còn %s",
				formatBytes(needBytes), formatBytes(vol.FreeBytes))
		}
		checks = append(checks, Check{Kind: CheckSpace, Status: CheckFail, Text: text})
	}

	// write — thử ghi thật vào tổ tiên gần nhất tồn tại.
	writeOK := false
	if dirWritable(target) {
		if base, err := firstExisting(target); err == nil {
			if st, err2 := os.Stat(base); err2 == nil && !st.IsDir() {
				base = filepath.Dir(base)
			}
			writeOK = writeTestFile(base) == nil
		}
	}
	if writeOK {
		checks = append(checks, Check{Kind: CheckWrite, Status: CheckOK, Text: "Ghi được vào đây"})
	} else {
		checks = append(checks, Check{Kind: CheckWrite, Status: CheckFail,
			Text: "Không ghi được vào thư mục này"})
	}

	// fs — FAT32 từ chối (tệp >4 GB), exFAT cảnh báo.
	switch vol.FSType {
	case "msdos":
		checks = append(checks, Check{Kind: CheckFS, Status: CheckFail,
			Text: "FAT32 không chứa được tệp lớn hơn 4 GB"})
	case "exfat":
		checks = append(checks, Check{Kind: CheckFS, Status: CheckWarn,
			Text: "exFAT dùng được; nhớ đẩy ổ ra trước khi rút"})
	default:
		checks = append(checks, Check{Kind: CheckFS, Status: CheckOK,
			Text: fmt.Sprintf("%s: hỗ trợ tệp lớn hơn 4 GB", vol.FS)})
	}

	ok := true
	for _, c := range checks {
		if c.Status == CheckFail {
			ok = false
		}
	}
	var eta *int
	if ok && mode == ModeMove && !sameVolume && used > 0 {
		estimate := int(used / (100 << 20)) // ~100 MB/s như artboard (81 GB ≈ 14 phút)
		eta = &estimate
	}
	return CheckResult{OK: ok, Path: target, CreateSubdir: createSubdir,
		Checks: checks, NeedBytes: needBytes, ETASec: eta}, nil
}

/* ------------------------------- watch ------------------------------- */

// watchedRoots là các thư mục cần còn truy cập được.
func (s *Service) watchedRoots() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := append([]string{s.location}, s.altRoots...)
	return roots
}

// tick là một nhịp theo dõi: stat từng root, phát hiện chuyển mất/hiện, xử
// lý bản cũ quá hạn. Rẻ: một stat + một access mỗi root mỗi nhịp.
func (s *Service) tick(ctx context.Context) {
	s.settlePrevious(ctx)
	// Ổ cắm/rút được phát hiện ở đây: snapshot phân loại ổ mới (một lần
	// diskutil) và phát sự kiện storage nếu tập ổ đổi — GET không spawn gì.
	if r, ok := s.deps.Volumes.(volumeRefresher); ok && r.refreshVolumes(ctx) {
		s.emit("")
	}
	for _, root := range s.watchedRoots() {
		usable := dirUsable(root)
		s.mu.Lock()
		_, wasMissing := s.missing[root]
		if !usable && !wasMissing {
			s.missing[root] = s.clock.Now()
		}
		if usable && wasMissing {
			delete(s.missing, root)
		}
		s.mu.Unlock()

		if usable {
			// Chỉ hỏi tên ổ một lần cho mỗi root: nhịp watch giữ đúng một
			// stat + một access, không gọi diskutil lặp lại.
			s.mu.Lock()
			known := s.volNames[root] != ""
			s.mu.Unlock()
			if !known {
				if info, err := s.deps.Volumes.Info(ctx, root); err == nil && info.Name != "" {
					s.mu.Lock()
					s.volNames[root] = info.Name
					s.mu.Unlock()
				}
			}
		}

		switch {
		case !usable && !wasMissing:
			s.onOffline(ctx, root)
		case usable && wasMissing:
			s.onOnline(ctx, root)
		}
	}
}

// missingLabel là câu gọi tên chỗ vừa mất: ổ ngoài/ổ mạng thì theo tên ổ;
// thư mục mặc định trong máy (hoặc dưới home) thì là "thư mục dự án" — ổ trong
// máy không rút được, cái mất là thư mục.
func (s *Service) missingLabel(root string) string {
	if root == s.deps.DefaultDir || (s.deps.Home != "" && strings.HasPrefix(root, s.deps.Home+"/")) {
		return "thư mục dự án"
	}
	s.mu.Lock()
	name := s.volNames[root]
	s.mu.Unlock()
	if name == "" {
		name = filepath.Base(root)
	}
	return "ổ " + name
}

// onOffline xử lý một root vừa biến mất: giữ việc, thông báo và chấm Cần xử lý.
func (s *Service) onOffline(ctx context.Context, root string) {
	if root == s.Root() {
		s.acquireHold(ctx, domain.HoldOffline)
	}
	if s.deps.Notify != nil {
		if _, err := s.deps.Notify.Notify(ctx, "needs_attention",
			"Không thấy "+s.missingLabel(root), "#/settings?section=storage", ""); err != nil {
			slog.Warn("storage: ghi thông báo lỗi", "err", err)
		}
	}
	s.emit("")
	s.bump()
}

// onOnline xử lý một root vừa quay lại: nhả hold tương ứng.
func (s *Service) onOnline(ctx context.Context, root string) {
	if root == s.Root() {
		s.releaseHold(ctx, domain.HoldOffline)
	}
	s.mu.Lock()
	anyMissing := len(s.missing) > 0
	s.mu.Unlock()
	if !anyMissing {
		s.releaseHold(ctx, domain.HoldUseDefault)
	}
	s.emit("")
	s.bump()
}

// settlePrevious xoá bản cũ khi quá hạn 7 ngày (và khi thư mục đó đã không còn).
func (s *Service) settlePrevious(ctx context.Context) {
	s.mu.Lock()
	prev := s.previous
	s.mu.Unlock()
	if prev == nil {
		return
	}
	if _, err := os.Lstat(prev.Path); err != nil {
		if isNotExist(err) {
			_ = s.setPrevious(ctx, nil)
			s.emit("")
		}
		return // lỗi khác (ổ đang rút): giữ, thử lại nhịp sau
	}
	if prev.Mode == ModeNewOnly || prev.DeleteAfter.IsZero() {
		return // bản cũ còn là dữ liệu đang đọc: không bao giờ tự xoá
	}
	if s.clock.Now().Before(prev.DeleteAfter) {
		return
	}
	if err := s.deletePath(ctx, prev.Path); err != nil {
		slog.Warn("storage: xoá bản cũ lỗi", "path", prev.Path, "err", err)
		return
	}
	_ = s.setPrevious(ctx, nil)
	s.emit("")
}

// Recheck chạy một nhịp watch ngay (nút Thử lại của banner offline) và trả
// thư mục hiện tại đã truy cập được chưa.
func (s *Service) Recheck(ctx context.Context) (bool, error) {
	s.tick(ctx)
	return dirUsable(s.Root()), nil
}

// Offline báo thư mục hiện tại đang bị mất (theo watch, không stat mỗi GET).
func (s *Service) Offline() (offline bool, since *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.missing[s.location]; ok {
		when := t
		return true, &when
	}
	return false, nil
}
