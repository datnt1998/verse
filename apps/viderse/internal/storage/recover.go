package storage

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
)

// recoverMove phục hồi một lần chuyển bị cắt ngang theo NHẬT KÝ — chạy trước
// ensureLocation nên khởi động sau một lần rename vẫn nhận đúng đích:
//
//   - phase commit (dữ liệu đã hoàn chỉnh ở To): chạy lại commitMove
//     (idempotent) — đổi vị trí, ghi bản cũ, xoá nhật ký.
//   - vị trí đã là To (sập sau setLocation, trước khi xoá nhật ký): hoàn tất
//     phần còn lại của commitMove.
//   - phase copy: nguồn còn thì xoá staging + nhật ký là đủ (dữ liệu nguồn
//     nguyên, vị trí chưa đổi). Nguồn mất mà đích đã là gốc dự án hoàn chỉnh
//     (rename cùng ổ đã xong trước khi sập) thì nhận đích luôn.
//   - cả hai đầu đều không đọc được (ổ đang rút): GIỮ nhật ký — lần mở sau,
//     khi ổ quay lại, phục hồi tiếp. Không đoán khi không nhìn thấy.
func (s *Service) recoverMove(ctx context.Context) {
	var j domain.StorageMove
	found, err := s.deps.Settings.Get(ctx, domain.KeyStorageMove, &j)
	if err != nil || !found || j.To == "" {
		return
	}
	// Dọn staging luôn an toàn: nó chỉ chứa bản chép dở.
	if j.Staging != "" {
		cleanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := fsio.RemoveAll(cleanCtx, j.Staging); err != nil && ctx.Err() == nil {
			slog.Warn("storage: dọn staging của lần chuyển cũ lỗi", "path", j.Staging, "err", err)
		}
		cancel()
	}

	toReachable := func() bool {
		info, err := os.Lstat(j.To)
		return err == nil && info.IsDir()
	}
	fromReachable := func() bool {
		if j.From == "" {
			return false
		}
		info, err := os.Lstat(j.From)
		return err == nil && info.IsDir()
	}

	switch {
	case s.Root() == j.To:
		// Cài đặt đã đổi xong: chỉ còn phần sau setLocation của commitMove.
		if err := s.commitMove(j); err != nil {
			slog.Warn("storage: hoàn tất chuyển cũ lỗi", "to", j.To, "err", err)
		}
		s.releaseHold(ctx, domain.HoldMoving)
	case j.Phase == domain.MovePhaseCommit:
		if !toReachable() {
			slog.Warn("storage: đích của lần chuyển đã commit không đọc được — giữ nhật ký", "to", j.To)
			return // ổ đang rút: thử lại lần mở sau, không đụng dữ liệu
		}
		if err := s.commitMove(j); err != nil {
			slog.Warn("storage: phục hồi lần chuyển đã commit lỗi", "to", j.To, "err", err)
			return
		}
		s.releaseHold(ctx, domain.HoldMoving)
	case !fromReachable() && toReachable() && hasRootMarker(j.To):
		// Rename cùng ổ đã xong rồi mới sập (cửa sổ giữa os.Rename và ghi
		// phase commit): đích là gốc dự án hoàn chỉnh — nhận nó.
		slog.Info("storage: nhận đích sau lần đổi tên bị cắt ngang", "from", j.From, "to", j.To)
		j.Phase = domain.MovePhaseCommit
		if err := s.commitMove(j); err != nil {
			slog.Warn("storage: nhận đích của lần chuyển cũ lỗi", "to", j.To, "err", err)
			return
		}
		s.releaseHold(ctx, domain.HoldMoving)
	case !fromReachable() && !toReachable():
		// Không nhìn thấy cả hai đầu: ổ đang rút — giữ nhật ký, lần sau tính.
		slog.Warn("storage: không đọc được nguồn lẫn đích của lần chuyển cũ — giữ nhật ký", "from", j.From, "to", j.To)
		return
	default:
		// Copy dở: nguồn còn nguyên — xoá nhật ký là đủ.
		if err := s.deps.Settings.Clear(ctx, domain.KeyStorageMove); err != nil {
			slog.Warn("storage: xoá nhật ký chuyển cũ lỗi", "err", err)
			return
		}
		s.releaseHold(ctx, domain.HoldMoving)
	}
	// Lần chuyển hỏng này hiện lên UI như một thất bại có-thể-thử-lại.
	if taskID := j.TaskID; taskID != "" {
		s.recordFailed(ctx, taskID, j.To, "Chuyển bị gián đoạn — thư mục cũ vẫn nguyên")
	}
}
