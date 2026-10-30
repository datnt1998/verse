package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
)

// stagingDirName là thư mục trung gian của một lần chuyển, nằm TRONG đích:
// chép xong một phần thì đổi tên (cùng filesystem, rẻ) vào chỗ cuối; huỷ hoặc
// sập chỉ cần xoá staging — thư mục cũ vẫn nguyên, vị trí không đổi.
const stagingDirName = ".viderse-incoming"

// previousKeepDays là thời gian giữ bản cũ sau khi chuyển xong (§23).
const previousKeepDays = 7

// progressEvery là nhịp gom tiến độ: tối đa 4 cập nhật/giây (§11, chống storm
// DB + SSE khi chép hàng nghìn tệp). Mọi lần đẩy — kể cả đổi nhóm/phase — đi
// qua nhịp này; chỉ một lần flush cuối được qua thẳng.
const progressEvery = 250 * time.Millisecond

// moveState là trạng thái của lần đổi thư mục đang chạy trong process. Đặt
// trước mọi I/O và giữ tới khi cài đặt đã ghi xong — hai lần chuyển không
// bao giờ chồng nhau, kể cả cửa sổ kết thúc.
type moveState struct {
	taskID string
	cancel context.CancelFunc
}

// Move bắt đầu đổi thư mục dự án. mode=new_only đổi ngay (không việc nền);
// mode=move trả taskId của việc nền storage_move.
func (s *Service) Move(ctx context.Context, path, mode string) (taskID string, done bool, err error) {
	target, _, _, err := s.resolveTarget(ctx, path)
	if err != nil {
		return "", false, err
	}
	if isCloudSyncPath(target, s.deps.Home) {
		return "", false, codeError(ErrCloudSync, "thư mục đồng bộ đám mây không dùng được")
	}
	if !volumePresent(target) {
		return "", false, codeError(ErrVolumeGone, "ổ của %s đang không cắm", target)
	}
	if mode != ModeNewOnly && mode != ModeMove {
		return "", false, codeError(ErrInvalidPath, "mode phải là move hoặc new_only")
	}
	if target == s.Root() {
		return "", true, nil // đã ở đích
	}
	if err := s.validateTarget(ctx, target); err != nil {
		return "", false, err
	}

	// Đặt chỗ lần đổi NGAY: hai Move song song chỉ một cái qua.
	s.mu.Lock()
	if s.moving != nil {
		s.mu.Unlock()
		return "", false, codeError(ErrMoveInProgress, "đang có một lần đổi thư mục khác")
	}
	moving := &moveState{}
	s.moving = moving
	s.mu.Unlock()

	if mode == ModeNewOnly {
		err := s.switchNewOnly(ctx, target)
		s.clearMoving(moving)
		return "", err == nil, err
	}
	return s.startMove(ctx, moving, target)
}

// clearMoving nhả chỗ của đúng lần đổi này (không gỡ nhầm lần mới hơn).
func (s *Service) clearMoving(moving *moveState) {
	s.mu.Lock()
	if s.moving == moving {
		s.moving = nil
	}
	s.mu.Unlock()
}

// startMove ghi nhật ký, tạo việc nền và chạy worker. Chỉ được gọi khi đã
// giữ chỗ s.moving.
func (s *Service) startMove(ctx context.Context, moving *moveState, target string) (string, bool, error) {
	old := s.Root()
	staging := filepath.Join(target, stagingDirName)

	// So trùng trước khi đụng gì: đích có tệp khác nội dung ở đường dẫn sẽ
	// ghi → từ chối, không ghi đè dữ liệu người dùng.
	if err := checkConflicts(old, target); err != nil {
		s.clearMoving(moving)
		return "", false, err
	}

	// Nhật ký (copy): sập thì khởi động sau xoá staging — nguồn vẫn nguyên.
	journal := domain.StorageMove{From: old, To: target, Staging: staging, Phase: domain.MovePhaseCopy}
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageMove, journal); err != nil {
		s.clearMoving(moving)
		return "", false, err
	}
	s.acquireHold(ctx, domain.HoldMoving)

	var id string
	if s.deps.Tasks != nil {
		var err error
		id, err = s.deps.Tasks.Start(ctx, domain.TaskSpec{
			Kind:  domain.TaskStorageMove,
			Title: "Chuyển thư mục dự án",
			Object: domain.ObjectRef{
				Type: "storage", ID: target,
				Label: fmt.Sprintf("Sang %s", filepath.Base(target)),
			},
			Phase:    "Đang chuẩn bị",
			Progress: &domain.TaskProgress{Done: 0, Total: 0},
			Action:   domain.TaskAction{Label: "Huỷ chuyển", Op: domain.TaskOpCancel},
		})
		if err != nil {
			s.releaseHold(ctx, domain.HoldMoving)
			_ = s.deps.Settings.Clear(ctx, domain.KeyStorageMove)
			s.clearMoving(moving)
			return "", false, err
		}
	}
	journal.TaskID = id
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageMove, journal); err != nil {
		slog.Warn("storage: cập nhật task vào nhật ký lỗi", "err", err)
	}

	// Worker chạy dưới vòng đời service (không phải của request): shutdown
	// huỷ và chờ nó qua Shutdown.
	runCtx, cancel := context.WithCancel(s.lifetime())
	s.mu.Lock()
	moving.taskID = id
	moving.cancel = cancel
	s.mu.Unlock()
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		s.runMove(runCtx, journal)
	}()
	return id, false, nil
}

// CancelMove huỷ lần chuyển đang chạy: thư mục cũ nguyên, vị trí không đổi.
func (s *Service) CancelMove(ctx context.Context, taskID string) error {
	s.mu.Lock()
	moving := s.moving
	s.mu.Unlock()
	if moving == nil || moving.cancel == nil || (taskID != "" && moving.taskID != taskID) {
		return codeError(ErrNoMove, "không có lần chuyển nào đang chạy")
	}
	moving.cancel()
	return nil
}

// switchNewOnly đổi vị trí sang target mà không đụng dữ liệu cũ: thư mục cũ
// thành altRoot (dự án cũ vẫn mở được). Đã giữ chỗ s.moving.
func (s *Service) switchNewOnly(ctx context.Context, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return codeError(ErrVolumeGone, "không tạo được thư mục %s", target)
	}
	if err := writeRootMarker(target); err != nil {
		return err
	}
	old := s.Root()
	if err := s.addAltRoot(ctx, old); err != nil {
		return err
	}
	if err := s.setLocation(ctx, target); err != nil {
		return err
	}
	if err := s.recordSwitch(ctx, ModeNewOnly); err != nil {
		return err
	}
	// Ghi bản cũ ở chế độ "không tự xoá": màn donenew cần path + dung lượng
	// thư mục cũ; dữ liệu đó vẫn đang được đọc qua altRoots.
	if old != target {
		prev := &domain.StoragePrevious{Path: old, Bytes: boundedBytes(ctx, old), Mode: ModeNewOnly}
		if err := s.setPrevious(ctx, prev); err != nil {
			return err
		}
	}
	s.emit("")
	return nil
}

// recordSwitch nhớ lần đổi thư mục gần nhất — màn hoàn tất của "chuyển dữ
// liệu" và "chỉ video mới" khác nhau, và phải còn đúng sau khi tải lại.
func (s *Service) recordSwitch(ctx context.Context, mode string) error {
	return s.deps.Settings.Set(ctx, domain.KeyStorageLastSwitch,
		domain.StorageSwitch{Mode: mode, At: s.clock.Now()})
}

// UseDefaultForNew là "Lưu video mới vào máy này": đổi vị trí về thư mục mặc
// định trong máy (ổ ngoài đang rút thành altRoot — cắm lại đọc tiếp được).
func (s *Service) UseDefaultForNew(ctx context.Context) error {
	s.mu.Lock()
	if s.moving != nil {
		s.mu.Unlock()
		return codeError(ErrMoveInProgress, "đang có một lần đổi thư mục khác")
	}
	moving := &moveState{}
	s.moving = moving
	s.mu.Unlock()
	defer s.clearMoving(moving)

	target := s.deps.DefaultDir
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := writeRootMarker(target); err != nil {
		return err
	}
	old := s.Root()
	if old != target {
		if err := s.addAltRoot(ctx, old); err != nil {
			return err
		}
		if err := s.setLocation(ctx, target); err != nil {
			return err
		}
	}
	if err := s.recordSwitch(ctx, ModeNewOnly); err != nil {
		return err
	}
	if old != target {
		prev := &domain.StoragePrevious{Path: old, Bytes: boundedBytes(ctx, old), Mode: ModeNewOnly}
		if err := s.setPrevious(ctx, prev); err != nil {
			return err
		}
	}
	s.releaseHold(ctx, domain.HoldOffline)
	// Còn root mất (ổ cũ) thì báo hold use_default cho scheduler.
	s.mu.Lock()
	missing := len(s.missing) > 0
	s.mu.Unlock()
	if missing {
		s.acquireHold(ctx, domain.HoldUseDefault)
	}
	s.emit("")
	return nil
}

// DeletePrevious xoá ngay bản cũ giữ 7 ngày (nút "Xoá bản cũ ngay").
func (s *Service) DeletePrevious(ctx context.Context) error {
	s.mu.Lock()
	prev := s.previous
	busy := s.moving != nil
	s.mu.Unlock()
	if busy {
		return codeError(ErrMoveInProgress, "đang có một lần đổi thư mục khác")
	}
	if prev == nil {
		return codeError(ErrNotFound, "không có bản cũ nào")
	}
	if prev.Mode == ModeNewOnly {
		return codeError(ErrNotDeletable, "thư mục cũ vẫn đang được dùng cho dự án cũ")
	}
	if err := s.deletePath(ctx, prev.Path); err != nil {
		return err
	}
	if err := s.setPrevious(ctx, nil); err != nil {
		return err
	}
	s.emit("")
	return nil
}

// deletePath xoá một cây — nhưng từ chối mọi đường dẫn còn là (hoặc chứa /
// nằm trong) thư mục dự án đang dùng. Bản cũ ghi nhầm vào đường đang hoạt
// động không bao giờ bị xoá.
func (s *Service) deletePath(ctx context.Context, path string) error {
	root := s.Root()
	if path == root || underDir(path, root) || underDir(root, path) {
		return codeError(ErrNotDeletable, "%s đang là (hoặc chứa) thư mục dự án", filepath.Base(path))
	}
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("kiểm %s: %w", path, err)
	}
	if err := fsio.RemoveAll(ctx, path); err != nil {
		return fmt.Errorf("xoá %s: %w", path, err)
	}
	return nil
}

// underDir báo path nằm dưới dir (không kể bằng nhau).
func underDir(path, dir string) bool {
	return dir != "" && strings.HasPrefix(path, dir+string(os.PathSeparator))
}

/* ------------------------------- worker ------------------------------- */

// runMove là thân của việc chuyển nền. Nhật ký định hướng mọi cửa sổ sập:
// chưa commit thì nguồn còn nguyên và vị trí chưa đổi; đã commit thì phục
// hồi chạy lại đúng bước cài đặt (idempotent).
func (s *Service) runMove(ctx context.Context, j domain.StorageMove) {
	// Nhả chỗ lần đổi CUỐI CÙNG — sau khi cài đặt đã ghi xong; một lần đổi
	// khác không chen được vào cửa sổ kết thúc.
	defer func() {
		s.mu.Lock()
		if s.moving != nil && s.moving.taskID == j.TaskID {
			s.moving = nil
		}
		s.mu.Unlock()
	}()

	fromVol, _ := s.deps.Volumes.Info(ctx, j.From)
	toVol, _ := s.deps.Volumes.Info(ctx, j.To)
	sameVolume := fromVol.FSType != "" && toVol.FSType != "" && fromVol.fsid() == toVol.fsid()

	committed, err := s.doMove(ctx, j.TaskID, j.From, j.To, j.Staging, sameVolume)
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 30*time.Second)
	_ = fsio.RemoveAll(cleanCtx, j.Staging)
	cleanCancel()

	bg := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 10*time.Second)
	}
	if err != nil {
		setCtx, cancel := bg()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			_ = s.deps.Settings.Clear(setCtx, domain.KeyStorageMove)
			s.finishMoveTask(j.TaskID, false, "")
		} else {
			slog.Warn("storage: chuyển thư mục không xong", "err", err)
			_ = s.deps.Settings.Clear(setCtx, domain.KeyStorageMove)
			s.recordFailed(setCtx, j.TaskID, j.To, "Chuyển không xong — dữ liệu ở thư mục cũ vẫn nguyên")
			s.finishMoveTask(j.TaskID, true, "Chuyển không xong — dữ liệu ở thư mục cũ vẫn nguyên")
		}
		cancel()
		s.releaseHold(context.Background(), domain.HoldMoving)
		s.emit("")
		return
	}

	// Dữ liệu đã ở đích hoàn chỉnh: đánh dấu commit TRƯỚC khi đổi cài đặt.
	if committed {
		j.Phase = domain.MovePhaseCommit
		if e := s.markMovePhase(j); e != nil {
			slog.Warn("storage: ghi phase commit lỗi — khởi động sau phục hồi", "err", e)
		}
	}
	if err := s.commitMove(j); err != nil {
		slog.Error("storage: chuyển xong nhưng cài đặt không ghi được", "err", err)
		setCtx, cancel := bg()
		s.recordFailed(setCtx, j.TaskID, j.To, "Chép xong nhưng không đổi được cài đặt")
		cancel()
		s.finishMoveTask(j.TaskID, true, "Chép xong nhưng không đổi được cài đặt")
		s.releaseHold(context.Background(), domain.HoldMoving)
		s.emit("")
		return
	}
	s.finishMoveTask(j.TaskID, false, "")
	s.releaseHold(context.Background(), domain.HoldMoving)
	s.emit("")
}

// markMovePhase cập nhật phase trong nhật ký (đọc-ghi lại record đã lưu).
func (s *Service) markMovePhase(j domain.StorageMove) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.deps.Settings.Set(ctx, domain.KeyStorageMove, j)
}

// commitMove hoàn tất lần chuyển đã có dữ liệu ở đích: đổi vị trí → ghi bản
// cũ → xoá nhật ký. Idempotent — worker và phục hồi khởi động gọi cùng hàm
// này. Thứ tự đảm bảo không cửa sổ nào để nguồn đang-dùng bị xếp vào xoá.
func (s *Service) commitMove(j domain.StorageMove) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.setLocation(ctx, j.To); err != nil {
		return fmt.Errorf("đổi vị trí: %w", err)
	}
	if dir, statErr := os.Stat(j.From); statErr == nil && dir.IsDir() {
		prev := &domain.StoragePrevious{
			Path:        j.From,
			Bytes:       boundedBytes(ctx, j.From),
			DeleteAfter: s.clock.Now().Add(previousKeepDays * 24 * time.Hour),
			Mode:        ModeMove,
		}
		if err := s.setPrevious(ctx, prev); err != nil {
			return fmt.Errorf("ghi bản cũ: %w", err)
		}
	}
	if err := s.recordSwitch(ctx, ModeMove); err != nil {
		slog.Warn("storage: ghi lần đổi thư mục lỗi", "err", err)
	}
	if err := s.deps.Settings.Clear(ctx, domain.KeyStorageMove); err != nil {
		return fmt.Errorf("xoá nhật ký: %w", err)
	}
	s.dropFailed(ctx)
	return nil
}

// finishMoveTask gỡ dòng việc nền (lỗi: đánh dấu failed + Thử lại trước).
func (s *Service) finishMoveTask(taskID string, failed bool, phase string) {
	if s.deps.Tasks == nil || taskID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if failed {
		p := phase
		_ = s.deps.Tasks.Update(ctx, taskID, domain.TaskUpdate{
			Phase: &p, State: domain.TaskFailed,
			Action: &domain.TaskAction{Label: "Thử lại", Op: domain.TaskOpRetry},
		})
		return
	}
	if err := s.deps.Tasks.Finish(ctx, taskID); err != nil {
		slog.Warn("storage: gỡ việc nền lỗi", "err", err)
	}
}

// recordFailed nhớ lần chuyển hỏng gần nhất để UI mở lại (Thử lại).
func (s *Service) recordFailed(ctx context.Context, taskID, target, phase string) {
	failed := domain.StorageMoveFailed{TaskID: taskID, Target: target, Phase: phase, At: s.clock.Now()}
	if err := s.deps.Settings.Set(ctx, domain.KeyStorageMoveFailed, failed); err != nil {
		slog.Warn("storage: ghi lần chuyển hỏng lỗi", "err", err)
		return
	}
	s.mu.Lock()
	s.failed = &failed
	s.mu.Unlock()
}

// dropFailed quên lần chuyển hỏng khi một lần chuyển khác đã xong.
func (s *Service) dropFailed(ctx context.Context) {
	s.mu.Lock()
	s.failed = nil
	s.mu.Unlock()
	_ = s.deps.Settings.Clear(ctx, domain.KeyStorageMoveFailed)
}

// MoveInfo là trạng thái lần đổi thư mục cho GET /api/storage: đang chạy và
// lần hỏng gần nhất (để UI không kẹt ở màn đang-chuyển).
type MoveInfo struct {
	Active bool
	TaskID string
	Failed *domain.StorageMoveFailed
}

func (s *Service) MoveInfo() MoveInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := MoveInfo{}
	if s.moving != nil {
		info.Active, info.TaskID = true, s.moving.taskID
	}
	if s.failed != nil {
		copy := *s.failed
		info.Failed = &copy
	}
	return info
}

// RetryMove chạy lại lần chuyển hỏng gần nhất (nút Thử lại của UI).
func (s *Service) RetryMove(ctx context.Context) (string, bool, error) {
	s.mu.Lock()
	failed := s.failed
	s.mu.Unlock()
	if failed == nil || failed.Target == "" {
		return "", false, codeError(ErrNoMove, "không có lần chuyển hỏng nào")
	}
	return s.Move(ctx, failed.Target, ModeMove)
}

// DismissMove gác lần chuyển hỏng (nút Bỏ qua — xoá bản ghi, không xoá file).
func (s *Service) DismissMove(ctx context.Context) error {
	s.mu.Lock()
	if s.moving != nil {
		s.mu.Unlock()
		return codeError(ErrMoveInProgress, "đang có một lần đổi thư mục khác")
	}
	s.mu.Unlock()
	s.dropFailed(ctx)
	s.emit("")
	return nil
}

// moveItem là một mục cấp một của thư mục gốc trong một lần chuyển.
type moveItem struct {
	name  string
	group Group
	bytes int64
}

// doMove thực hiện chép/đổi tên và trả committed=true khi dữ liệu đã hoàn
// chỉnh ở đích. sameVolume: thử rename nguyên thư mục khi đích trống.
func (s *Service) doMove(ctx context.Context, taskID, old, target, staging string, sameVolume bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false, fmt.Errorf("tạo đích %s: %w", target, err)
	}
	if sameVolume {
		if empty, err := dirEmpty(target); err == nil && empty {
			if err := os.Remove(target); err != nil && !isNotExist(err) {
				return false, fmt.Errorf("dọn đích rỗng: %w", err)
			}
			if err := os.Rename(old, target); err == nil {
				return true, nil
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return false, fmt.Errorf("tạo lại đích: %w", err)
			}
		}
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return false, fmt.Errorf("tạo staging: %w", err)
	}

	entries, err := os.ReadDir(old)
	if err != nil {
		return false, fmt.Errorf("đọc %s: %w", old, err)
	}
	var items []moveItem
	totals := map[Group]int64{}
	var total int64
	for _, e := range entries {
		if e.Name() == stagingDirName {
			continue
		}
		it := moveItem{name: e.Name(), bytes: fsio.Bytes(filepath.Join(old, e.Name()))}
		if isGroup(Group(e.Name())) {
			it.group = Group(e.Name())
		}
		totals[it.group] += it.bytes
		total += it.bytes
		items = append(items, it)
	}
	var groups []Group
	for _, g := range Groups {
		if totals[g] > 0 {
			groups = append(groups, g)
		}
	}
	if totals[""] > 0 {
		groups = append(groups, "")
	}

	progress := newMoveProgress(s, taskID, total, groups, totals)
	progress.report(ctx, "Đang chuẩn bị")

	for _, g := range groups {
		progress.startGroup(ctx, g)
		for _, it := range items {
			if it.group != g {
				continue
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}
			staged := filepath.Join(staging, it.name)
			if err := copyTree(ctx, filepath.Join(old, it.name), staged, func(n int64) { progress.add(g, n) }); err != nil {
				return false, fmt.Errorf("chép %s: %w", it.name, err)
			}
			if err := verifyCopy(ctx, filepath.Join(old, it.name), staged); err != nil {
				return false, err
			}
			if err := commitEntry(ctx, staged, filepath.Join(target, it.name)); err != nil {
				return false, err
			}
		}
		progress.finishGroup(g)
	}
	progress.flush()
	if err := writeRootMarker(target); err != nil {
		return false, err
	}
	return true, nil
}

// checkConflicts so từng tệp nguồn với đích trước khi làm gì: đích có tệp
// khác nội dung tại đường dẫn sẽ ghi thì từ chối — một lần chuyển không bao
// giờ ghi đè dữ liệu đã có. Tệp đích giống hệt (move-lại-bản-cũ) không tính.
func checkConflicts(old, target string) error {
	var conflict error
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil || conflict != nil {
			return err
		}
		rel, err := filepath.Rel(old, path)
		if err != nil {
			return nil
		}
		if rel == stagingDirName || underDir(rel, stagingDirName) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		final := filepath.Join(target, rel)
		dst, err := os.Lstat(final)
		if err != nil {
			return nil // không có ở đích: không trùng
		}
		switch {
		case info.IsDir() && dst.IsDir():
			return nil
		case !info.IsDir() && !dst.IsDir():
			if info.Size() == dst.Size() {
				if same, _ := sameFileHash(path, final); same {
					return nil // bản giống hệt: chép đè cũng không mất gì
				}
			}
		}
		conflict = codeError(ErrDestConflict, "đích đã có “%s” khác nội dung — không ghi đè", rel)
		return conflict
	}
	if err := filepath.Walk(old, walk); err != nil {
		return err
	}
	return conflict
}

// sameFileHash so hai tệp bằng sha256 (đã biết cùng kích thước).
func sameFileHash(a, b string) (bool, error) {
	ha, err := hashFile(a)
	if err != nil {
		return false, err
	}
	hb, err := hashFile(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

// commitEntry đưa một mục từ staging về chỗ cuối. Không bao giờ ghi đè tệp
// khác nội dung: trùng tệp khác nhau → lỗi trùng (dừng lần chuyển, staging
// và dữ liệu đích đều còn nguyên); trùng tệp giống hệt → chỉ bỏ bản staging.
func commitEntry(ctx context.Context, staged, final string) error {
	st, err := os.Lstat(staged)
	if err != nil {
		if isNotExist(err) {
			return nil // không có gì để đưa (nhóm trống)
		}
		return err
	}
	dst, derr := os.Lstat(final)
	switch {
	case derr != nil && isNotExist(derr):
		if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
			return err
		}
		return os.Rename(staged, final)
	case derr != nil:
		return derr
	case !st.IsDir() && !dst.IsDir():
		if st.Size() == dst.Size() {
			if same, err := sameFileHash(staged, final); err == nil && same {
				return os.Remove(staged) // đích đã có bản giống hệt
			}
		}
		return codeError(ErrDestConflict, "đích đã có “%s” khác nội dung — không ghi đè", filepath.Base(final))
	case st.IsDir() != dst.IsDir():
		return codeError(ErrDestConflict, "đích có “%s” chắn đường", filepath.Base(final))
	}
	// Cả hai là thư mục: gộp từng mục con (vẫn không ghi đè tệp).
	entries, err := os.ReadDir(staged)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := commitEntry(ctx, filepath.Join(staged, e.Name()), filepath.Join(final, e.Name())); err != nil {
			return err
		}
	}
	return os.Remove(staged)
}

// verifyCopy so một mục nguồn với bản vừa chép: tổng byte khớp và băm mẫu
// (tệp lớn nhất + tệp đầu theo tên) giống nhau.
func verifyCopy(ctx context.Context, src, staged string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if fsio.Bytes(src) != fsio.Bytes(staged) {
			return fmt.Errorf("kích thước chép lệch ở %s", filepath.Base(src))
		}
		for _, rel := range sampleFiles(src) {
			if err := compareHash(ctx, filepath.Join(src, rel), filepath.Join(staged, rel)); err != nil {
				return err
			}
		}
		return nil
	}
	return compareHash(ctx, src, staged)
}

// sampleFiles chọn tối đa 3 tệp đại diện trong một cây: tệp lớn nhất và hai
// tệp đầu theo tên. Băm cả cây quá đắt cho 81 GB; mẫu đủ bắt lỗi rỗng/ngắn.
func sampleFiles(root string) []string {
	type found struct {
		rel  string
		size int64
	}
	var files []found
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, found{rel: rel, size: info.Size()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		if files[i].size != files[j].size {
			return files[i].size > files[j].size
		}
		return files[i].rel < files[j].rel
	})
	var out []string
	for _, f := range files {
		if len(out) == 3 {
			break
		}
		out = append(out, f.rel)
	}
	sort.Strings(out)
	return out
}

// compareHash băm hai tệp và so (dừng theo ctx giữa hai lần đọc).
func compareHash(ctx context.Context, a, b string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ha, err := hashFileCtx(ctx, a)
	if err != nil {
		return err
	}
	hb, err := hashFileCtx(ctx, b)
	if err != nil {
		return err
	}
	if ha != hb {
		return fmt.Errorf("băm lệch ở %s", filepath.Base(a))
	}
	return nil
}

// copyTree chép một tệp hoặc cây thư mục; onDone nhận số byte vừa chép xong.
func copyTree(ctx context.Context, src, dst string, onDone func(int64)) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, dst); err != nil {
			return err
		}
		return nil
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(ctx, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), onDone); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(ctx, src, dst, info, onDone)
}

func copyFile(ctx context.Context, src, dst string, info os.FileInfo, onDone func(int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			_ = out.Close()
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				_ = out.Close()
				return werr
			}
			onDone(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = out.Close()
			return rerr
		}
	}
	return out.Close()
}

// hashFileCtx băm sha256 một tệp, ngắt theo ctx mỗi block (tệp lớn phải huỷ
// được khi server tắt).
func hashFileCtx(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, rerr := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", rerr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFile băm sha256 không giới hạn (chỉ cho đường nhỏ trong so trùng).
func hashFile(path string) (string, error) {
	return hashFileCtx(context.Background(), path)
}

/* ------------------------------ progress ------------------------------ */

// moveProgress gom và đẩy tiến độ nhóm: MỌI lần đẩy đều qua nhịp 250ms —
// đổi nhóm/phase cũng không qua thẳng; chỉ flush cuối lần chuyển được phép
// vượt nhịp để trạng thái cuối không bị nuốt.
type moveProgress struct {
	svc    *Service
	taskID string
	total  int64
	groups []Group
	totals map[Group]int64

	mu      sync.Mutex
	done    int64
	byGroup map[Group]int64
	lastAt  time.Time
}

func newMoveProgress(s *Service, taskID string, total int64, groups []Group, totals map[Group]int64) *moveProgress {
	byGroup := make(map[Group]int64, len(totals)+1)
	for g := range totals {
		byGroup[g] = 0
	}
	return &moveProgress{svc: s, taskID: taskID, total: total, groups: groups, totals: totals, byGroup: byGroup}
}

// add cộng byte vừa chép và đẩy tiến độ nếu đã qua nhịp.
func (p *moveProgress) add(g Group, n int64) {
	p.mu.Lock()
	p.done += n
	p.byGroup[g] += n
	p.mu.Unlock()
	p.send(context.Background(), "", false)
}

// startGroup/finishGroup báo chuyển nhóm — vẫn qua nhịp 250ms như mọi đẩy.
func (p *moveProgress) startGroup(ctx context.Context, g Group) {
	p.send(ctx, "Đang chép "+groupLabel(g), false)
}

func (p *moveProgress) finishGroup(g Group) {
	p.send(context.Background(), "", false)
}

// report đẩy tiến độ phase (vẫn qua nhịp).
func (p *moveProgress) report(ctx context.Context, phase string) {
	p.send(ctx, phase, false)
}

// flush đẩy trạng thái cuối một lần (lần duy nhất được qua nhịp).
func (p *moveProgress) flush() {
	p.send(context.Background(), "", true)
}

func (p *moveProgress) send(ctx context.Context, phase string, final bool) {
	if p.svc.deps.Tasks == nil || p.taskID == "" {
		return
	}
	p.mu.Lock()
	if !final && p.svc.clock.Now().Sub(p.lastAt) < progressEvery {
		p.mu.Unlock()
		return
	}
	p.lastAt = p.svc.clock.Now()
	done := p.done
	groups := make([]domain.TaskGroupProgress, 0, len(p.groups))
	for _, g := range p.groups {
		groups = append(groups, domain.TaskGroupProgress{
			Group: groupKey(g), Done: p.byGroup[g], Total: p.totals[g],
		})
	}
	p.mu.Unlock()

	u := domain.TaskUpdate{
		Progress: &domain.TaskProgress{Done: done, Total: p.total, Groups: groups},
	}
	if phase != "" {
		u.Phase = &phase
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.svc.deps.Tasks.Update(writeCtx, p.taskID, u); err != nil {
		slog.Warn("storage: cập nhật tiến độ chuyển lỗi", "err", err)
	}
	p.svc.emit(p.taskID)
}

// groupKey đổi nhóm nội bộ sang khoá wire; mục lạ báo là "other".
func groupKey(g Group) string {
	if g == "" {
		return "other"
	}
	return g
}

// groupLabel là câu tiếng Việt của nhóm trong phase tiến độ.
func groupLabel(g Group) string {
	switch g {
	case GroupImages:
		return "ảnh đã vẽ"
	case GroupEditor:
		return "project editor"
	case GroupSources:
		return "video gốc"
	case GroupExports:
		return "bản xuất"
	default:
		return "tệp khác"
	}
}
