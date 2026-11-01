package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// Lớp phủ dùng chung (Thư viện › Lớp phủ): một dòng mỗi nội dung (sha256),
// tệp nằm dưới library/overlays/ của thư mục dự án và DB chỉ giữ đường dẫn
// tương đối. Gói 12 hiệu ứng có sẵn được nhúng vào binary, ghi ra một lần
// khi serve khởi động và không bỏ được; lớp phủ tự thêm thì bỏ được.

// OverlayDir là thư mục tương đối của tệp lớp phủ trong thư mục dự án.
const OverlayDir = "library/overlays"

// ListOverlays trả lớp phủ còn trong thư viện, mới nhất trước, kèm template
// đang dùng (đọc recipe của templates — chỉ đọc chéo mảng).
func (s *Service) ListOverlays(ctx context.Context) ([]domain.Overlay, error) {
	list, err := s.store.ListOverlays(ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []domain.Overlay{}
	}
	return list, nil
}

// OverlayUsage trả map overlay id → template đang gắn.
func (s *Service) OverlayUsage(ctx context.Context) (map[string][]TemplateRef, error) {
	return s.store.OverlayUsage(ctx)
}

// GetOverlay trả một lớp phủ còn trong thư viện.
func (s *Service) GetOverlay(ctx context.Context, id string) (*domain.Overlay, error) {
	overlay, err := s.store.OverlayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if overlay == nil {
		return nil, notFound("Lớp phủ")
	}
	return overlay, nil
}

// AddOverlayFile thêm một tệp từ đĩa (CLI `viderse overlay add`): đuôi/MIME
// lấy từ tên tệp thật, name chỉ là nhãn hiển thị.
func (s *Service) AddOverlayFile(ctx context.Context, path, name string) (*domain.Overlay, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, errBadMedia(err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, false, errBadMedia(err)
	}
	return s.AddOverlayReader(ctx, filepath.Base(path), name, file, stat.Size())
}

// AddOverlayReader thêm một tệp (đường HTTP multipart dùng cùng hàm):
// filename là tên tệp thật (quyết định đuôi/MIME), displayName là nhãn hiển
// thị (rỗng = lấy tên tệp bỏ đuôi). Tệp được ghi tạm, đọc chữ ký nội dung
// đối chiếu đuôi, băm sha256, đo bằng ffprobe, đoán nền rồi mới vào thư viện.
// reused=true khi nội dung đã có sẵn (không tạo thêm dòng).
func (s *Service) AddOverlayReader(ctx context.Context, filename, displayName string, r io.Reader, size int64) (*domain.Overlay, bool, error) {
	mime := MimeForFile(filename)
	if mime == "" {
		return nil, false, invalidInput(
			"Tệp không phải video hoặc ảnh dùng được",
			"Chọn tệp "+strings.Join(allowedMimeList(), ", "),
		)
	}
	display := strings.TrimSpace(displayName)
	if display == "" {
		display = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}
	display = truncateRunes(display, domain.LimitOverlayNameMaxChars)
	if display == "" {
		return nil, false, invalidInput("Tệp lớp phủ chưa có tên", "Chọn tệp video hoặc ảnh")
	}
	switch {
	case size <= 0:
		return nil, false, &Error{Code: CodeBadMedia, Message: "Tệp lớp phủ đang rỗng", Recovery: "Chọn tệp khác"}
	case size > OverlayLimit():
		message, recovery := OverlayTooLarge()
		return nil, false, invalidInput(message, recovery)
	}
	if s.files == nil && s.media == nil {
		return nil, false, fmt.Errorf("thiếu kho lưu lớp phủ")
	}
	dir := ""
	if s.media == nil {
		var err error
		dir, err = s.overlayAbsDir()
		if err != nil {
			return nil, false, err
		}
	}
	ext := strings.ToLower(filepath.Ext(filename))
	temp, err := os.CreateTemp(dir, "viderse-overlay-*"+ext)
	if err != nil {
		return nil, false, fmt.Errorf("tạo tệp tạm lớp phủ: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	hash := sha256.New()
	written, err := copyBounded(ctx, io.MultiWriter(temp, hash), r, OverlayLimit()+1)
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, false, errBadMedia(err)
	}
	if written > OverlayLimit() {
		message, recovery := OverlayTooLarge()
		return nil, false, invalidInput(message, recovery)
	}
	if written == 0 {
		return nil, false, &Error{Code: CodeBadMedia, Message: "Tệp lớp phủ đang rỗng", Recovery: "Chọn tệp khác"}
	}
	sum := hex.EncodeToString(hash.Sum(nil))

	// Chữ ký nội dung phải khớp đuôi được nhận: tệp audio-only hoặc container
	// khác đổi tên thành .mp4/.png không qua được bước này.
	head := make([]byte, 64)
	if f, err := os.Open(tempPath); err == nil {
		n, _ := f.Read(head)
		head = head[:n]
		f.Close()
	} else {
		return nil, false, errBadMedia(err)
	}
	if sniffMediaClass(head) != expectedClass(mime) {
		return nil, false, &Error{
			Code:     CodeBadMedia,
			Message:  "Nội dung tệp không khớp loại " + mime,
			Recovery: "Chọn đúng tệp video hoặc ảnh",
		}
	}

	// Dedupe theo nội dung: cùng byte thì dùng lại dòng cũ. Tệp của dòng cũ
	// mất khỏi đĩa thì công bố lại bytes vừa tải vào đúng đường dẫn của dòng
	// đó (GET /file tự phục hồi). Dòng đã bị bỏ thì khôi phục + đổi tên.
	existing, removed, err := s.store.OverlayBySHA(ctx, sum)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if err := s.publishOverlay(ctx, existing, tempPath, func(txctx context.Context) error {
			if removed {
				return s.store.RestoreOverlay(txctx, existing.ID, display)
			}
			return nil
		}); err != nil {
			return nil, false, err
		}
		if removed {
			existing.Name = display
			s.emit(TopicOverlay, existing.ID)
		}
		return existing, true, nil
	}

	ffmpeg, ffprobe, ok := s.toolPaths(ctx)
	if !ok {
		return nil, false, errToolMissing()
	}
	info, err := probeFile(ctx, s.runner, ffprobe, tempPath)
	if err != nil {
		return nil, false, errBadMedia(err)
	}
	background := detectBackground(ctx, s.runner, ffmpeg, tempPath, info.DurationSec)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	id := s.newID()
	rel := filepath.ToSlash(filepath.Join(OverlayDir, id+ext))
	overlay := domain.Overlay{
		ID: id, SHA256: sum, Name: display, FilePath: rel, Mime: mime,
		Bytes: written, DurationSec: info.DurationSec, Width: info.Width, Height: info.Height,
		Background: background, CreatedAt: s.now().UTC(),
	}
	if err := s.publishOverlay(ctx, &overlay, tempPath, func(txctx context.Context) error {
		return s.store.InsertOverlay(txctx, overlay)
	}); err != nil {
		if isUniqueViolation(err) {
			again, removed, lookupErr := s.store.OverlayBySHA(ctx, sum)
			if lookupErr != nil {
				return nil, false, lookupErr
			}
			if again != nil {
				if err := s.publishOverlay(ctx, again, tempPath, func(txctx context.Context) error {
					if removed {
						return s.store.RestoreOverlay(txctx, again.ID, display)
					}
					return nil
				}); err != nil {
					return nil, false, err
				}
				if removed {
					again.Name = display
				}
				return again, true, nil
			}
		}
		return nil, false, err
	}
	s.emit(TopicOverlay, id)
	return &overlay, false, nil
}

// copyBounded sao chép tối đa limit byte, kiểm ctx giữa các lần đọc.
func copyBounded(ctx context.Context, w io.Writer, r io.Reader, limit int64) (int64, error) {
	var total int64
	buf := make([]byte, 256<<10)
	for total < limit {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		want := limit - total
		if want > int64(len(buf)) {
			want = int64(len(buf))
		}
		n, readErr := r.Read(buf[:want])
		if n > 0 {
			wn, werr := w.Write(buf[:n])
			total += int64(wn)
			if werr != nil {
				return total, werr
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
	return total, nil
}

// publishOverlay keeps the domain mutation inside the manifest transaction.
// Files-only dependencies are retained for local in-memory store tests.
func (s *Service) publishOverlay(ctx context.Context, overlay *domain.Overlay, from string, commit func(context.Context) error) error {
	if s.media != nil {
		return s.media.Publish(ctx, []media.Artifact{{
			Path: overlay.FilePath, File: from, ContentType: overlay.Mime,
			Filename: overlay.Name + filepath.Ext(overlay.FilePath),
		}}, commit)
	}
	if err := s.publishOverlayBytes(overlay, from); err != nil {
		return err
	}
	return commit(ctx)
}

// publishOverlayBytes đảm bảo tệp của một dòng còn trên đĩa; thiếu thì đặt
// bytes đã tải (ở from) vào đúng đường dẫn của dòng một cách nguyên tử.
func (s *Service) publishOverlayBytes(overlay *domain.Overlay, from string) error {
	target, err := s.files.Resolve(overlay.FilePath)
	if err != nil {
		return fmt.Errorf("đường dẫn lớp phủ: %w", err)
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	// Tệp đích bị mất: ghi lại qua tệp tạm cùng thư mục rồi đổi tên; hai
	// repair đua nhau thì rename cuối thắng, nội dung trùng sha nên vô hại.
	dir, err := s.overlayAbsDir()
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".tmp-repair-*")
	if err != nil {
		return fmt.Errorf("tạo tệp tạm lớp phủ: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	src, err := os.Open(from)
	if err != nil {
		return errBadMedia(err)
	}
	defer src.Close()
	if _, err := copyBounded(context.Background(), temp, src, OverlayLimit()+1); err != nil {
		temp.Close()
		return errBadMedia(err)
	}
	if err := temp.Close(); err != nil {
		return errBadMedia(err)
	}
	if err := os.Rename(tempPath, target); err != nil {
		return fmt.Errorf("phục hồi tệp lớp phủ: %w", err)
	}
	return nil
}

// requireLiveOverlay báo lỗi khi tệp của một dòng đã mất khỏi đĩa.
func (s *Service) requireLiveOverlay(_ context.Context, overlay *domain.Overlay) (string, error) {
	path, err := s.files.Resolve(overlay.FilePath)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", &Error{
			Code:     CodeBadMedia,
			Message:  "Tệp lớp phủ không còn trên đĩa",
			Recovery: "Thêm lại tệp này vào thư viện",
		}
	}
	return path, nil
}

// AcquireOverlay pins a working file until its consumer releases the handle.
func (s *Service) AcquireOverlay(ctx context.Context, id string) (*media.Handle, *domain.Overlay, error) {
	overlay, err := s.GetOverlay(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if s.media != nil {
		handle, err := s.media.Acquire(ctx, overlay.FilePath)
		return handle, overlay, err
	}
	path, err := s.requireLiveOverlay(ctx, overlay)
	if err != nil {
		return nil, nil, err
	}
	return &media.Handle{Path: path}, overlay, nil
}

// OverlayPath is the files-only adapter. Backend consumers use AcquireOverlay.
func (s *Service) OverlayPath(ctx context.Context, id string) (string, *domain.Overlay, error) {
	overlay, err := s.GetOverlay(ctx, id)
	if err != nil {
		return "", nil, err
	}
	path, err := s.requireLiveOverlay(ctx, overlay)
	if err != nil {
		return "", nil, err
	}
	return path, overlay, nil
}

// RemoveOverlay bỏ một lớp phủ tự thêm khỏi thư viện: đặt removed_at, giữ tệp
// để template đang dùng vẫn phủ được. Bỏ lại lần nữa là no-op. Hiệu ứng có
// sẵn không bỏ được.
func (s *Service) RemoveOverlay(ctx context.Context, id string) error {
	overlay, err := s.store.OverlayByID(ctx, id)
	if err != nil {
		return err
	}
	if overlay == nil {
		return notFound("Lớp phủ")
	}
	if overlay.Builtin != "" {
		return &Error{
			Code:     CodeBuiltin,
			Message:  "Hiệu ứng có sẵn không xoá được",
			Recovery: "Bỏ hiệu ứng khỏi template nếu không muốn dùng",
		}
	}
	if err := s.store.RemoveOverlay(ctx, id); err != nil {
		return err
	}
	s.emit(TopicOverlay, id)
	return nil
}

// ── hiệu ứng có sẵn ────────────────────────────────────────────────────────

// SeedBuiltins ghi các hiệu ứng có sẵn chưa từng có dòng vào thư viện dưới
// thư mục dự án. Idempotent: hiệu ứng đã có dòng (kể cả đã bỏ) hoặc nội dung
// đã có trong thư viện thì bỏ qua, không băm lại. Trả số hiệu ứng đã thêm.
//
// Hàm này chỉ được gọi từ việc nền lúc serve khởi động (hoặc CLI), KHÔNG từ
// một GET.
func (s *Service) SeedBuiltins(ctx context.Context) (int, error) {
	effects, err := BuiltinEffects()
	if err != nil {
		return 0, err
	}
	if s.files == nil && s.media == nil {
		return 0, fmt.Errorf("thiếu kho lưu hiệu ứng có sẵn")
	}
	existing, err := s.store.ExistingBuiltins(ctx)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, effect := range effects {
		if err := ctx.Err(); err != nil {
			return added, err
		}
		if existing[effect.ID] {
			continue
		}
		if row, _, err := s.store.OverlayBySHA(ctx, effect.SHA); err != nil {
			return added, err
		} else if row != nil {
			// Người dùng đã tự thêm đúng tệp này: để yên bản của họ.
			continue
		}
		ok, err := s.seedBuiltin(ctx, effect)
		if err != nil {
			return added, err
		}
		if ok {
			added++
		}
	}
	if added > 0 {
		s.emit(TopicOverlay, "")
	}
	return added, nil
}

// seedBuiltin ghi một hiệu ứng có sẵn; false khi đua với tiến trình khác.
func (s *Service) seedBuiltin(ctx context.Context, effect builtinEffect) (bool, error) {
	data, err := builtinOverlayBytes(effect)
	if err != nil {
		return false, err
	}
	dir := ""
	if s.media == nil {
		dir, err = s.overlayAbsDir()
		if err != nil {
			return false, err
		}
	}
	rel := filepath.ToSlash(filepath.Join(OverlayDir, effect.ID+"-loop.mp4"))
	temp, err := os.CreateTemp(dir, "viderse-builtin-"+effect.ID+"-*")
	if err != nil {
		return false, fmt.Errorf("tạo tệp tạm hiệu ứng: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return false, fmt.Errorf("ghi hiệu ứng %s: %w", effect.ID, err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return false, err
	}
	if err := ctx.Err(); err != nil {
		os.Remove(tempPath)
		return false, err
	}
	// Siêu dữ liệu là tuỳ chọn: máy chưa có ffmpeg vẫn có hiệu ứng (nền đen
	// như yt-factory, thời lượng để trống).
	var info ProbeInfo
	background := domain.BgBlack
	if ffmpeg, ffprobe, ok := s.toolPaths(ctx); ok {
		if probed, err := probeFile(ctx, s.runner, ffprobe, tempPath); err == nil {
			info = probed
			if probed.DurationSec != nil {
				background = detectBackground(ctx, s.runner, ffmpeg, tempPath, probed.DurationSec)
			}
		}
	}
	overlay := domain.Overlay{
		ID: s.newID(), SHA256: effect.SHA, Name: effect.Name, FilePath: rel,
		Mime: "video/mp4", Bytes: int64(len(data)),
		DurationSec: info.DurationSec, Width: info.Width, Height: info.Height,
		Background: background, Builtin: effect.ID, CreatedAt: s.now().UTC(),
	}
	if err := s.publishOverlay(ctx, &overlay, tempPath, func(txctx context.Context) error {
		return s.store.InsertOverlay(txctx, overlay)
	}); err != nil {
		// Đường dẫn hiệu ứng là dùng chung giữa các tiến trình seed: thua cuộc
		// không được xoá tệp của tiến trình thắng — nội dung trùng sha nên cứ
		// để file lại, seed sau sẽ ghi đè nếu cần.
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// findBuiltinOverlay tìm dòng của một hiệu ứng có sẵn còn trong thư viện.
func (s *Service) findBuiltinOverlay(ctx context.Context, effect string) (*domain.Overlay, bool, error) {
	list, err := s.store.ListOverlays(ctx)
	if err != nil {
		return nil, false, err
	}
	for i := range list {
		if list[i].Builtin == effect {
			return &list[i], true, nil
		}
	}
	return nil, false, nil
}

// overlayAbsDir tạo thư mục library/overlays/ và trả đường dẫn tuyệt đối.
func (s *Service) overlayAbsDir() (string, error) {
	path, err := s.files.Resolve(OverlayDir)
	if err != nil {
		return "", fmt.Errorf("đường dẫn thư mục lớp phủ: %w", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("tạo thư mục lớp phủ: %w", err)
	}
	return path, nil
}

// toolPaths trả ffmpeg/ffprobe đã chọn; false khi thiếu runner/ffmpeg.
func (s *Service) toolPaths(ctx context.Context) (ffmpeg, ffprobe string, ok bool) {
	if s.bins == nil || s.runner == nil {
		return "", "", false
	}
	ffmpeg, ffprobe, ok = s.bins.Tools(ctx)
	if !ok || ffmpeg == "" || ffprobe == "" {
		return "", "", false
	}
	return ffmpeg, ffprobe, true
}
