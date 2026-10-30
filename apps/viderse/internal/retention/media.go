package retention

import (
	"context"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mediaClaimGrace: media vừa nhập (hoặc vừa được dùng lại) còn được giữ trong
// 30 phút — đủ cho lần nhập đang chạy kịp liên kết nó.
const mediaClaimGrace = 30 * time.Minute

// pendingFile là danh sách media chờ dọn lại của lần xoá dự án trước.
const pendingFile = "media-reclaim-pending.json"

// mediaMeta là subset của metadata.json mà bộ dọn dùng.
type mediaMeta struct {
	ContentHash string `json:"contentHash"`
	Mime        string `json:"mime"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"createdAt"`
	Overlay     bool   `json:"overlay"`
	LastUsedAt  int64  `json:"lastUsedAt"`
}

// mediaRoot là thư mục media của editor.
func (s *Service) mediaRoot() string {
	return filepath.Join(s.deps.Roots.Root(), "editor", "media")
}

// mediaMetaPath là metadata.json của một media.
func (s *Service) mediaMetaPath(id string) string {
	return filepath.Join(s.mediaRoot(), id, "metadata.json")
}

// readMediaMeta đọc metadata; ok=false khi thiếu/hỏng.
func (s *Service) readMediaMeta(id string) (mediaMeta, bool) {
	var meta mediaMeta
	if !readJSON(s.mediaMetaPath(id), &meta) {
		return mediaMeta{}, false
	}
	return meta, true
}

// heldMedia là mọi id media còn được giữ: UUID trong JSON dự án ở MỌI root
// còn đọc được (chiều dự án → media) + id mà video đang chạy giữ. Lỗi đọc là
// fail closed: một thư mục hay tệp tham chiếu không đọc được thì không dọn
// gì cả — không được coi như "không có tham chiếu" (yt-factory media-gc.ts
// ném non-ENOENT lỗi; chỉ ENOENT là "root đó không có project nào").
func (s *Service) heldMedia(ctx context.Context) (map[string]bool, error) {
	held := map[string]bool{}
	_, roots := s.deps.Roots.Roots()
	for _, root := range roots {
		projects := filepath.Join(root, "editor", "projects")
		dirs, err := os.ReadDir(projects)
		if err != nil {
			if notExist(err) {
				continue
			}
			return held, fmt.Errorf("đọc dự án ở %s: %w", root, err)
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			dir := filepath.Join(projects, d.Name())
			files, err := os.ReadDir(dir)
			if err != nil {
				return held, fmt.Errorf("đọc dự án %s: %w", dir, err)
			}
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
					continue
				}
				if err := tokensIn(filepath.Join(dir, f.Name()), held); err != nil {
					return held, fmt.Errorf("đọc tham chiếu %s: %w", f.Name(), err)
				}
			}
		}
	}
	if s.deps.Videos != nil {
		ids, err := s.deps.Videos.HeldMedia(ctx)
		if err != nil {
			return held, err
		}
		for id := range ids {
			held[strings.ToLower(id)] = true
		}
	}
	return held, nil
}

// mediaIDsOnDisk liệt kê id thư mục media hợp lệ.
func (s *Service) mediaIDsOnDisk() []string {
	entries, err := os.ReadDir(s.mediaRoot())
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && isPortableID(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	return ids
}

// mediaEligible báo media này có thể xoá: không bị giữ, không phải overlay,
// và đã qua thời gian giữ (lần dùng cuối hoặc lúc tạo).
func (s *Service) mediaEligible(id string, held map[string]bool, now time.Time) (bool, int64) {
	if held[strings.ToLower(id)] {
		return false, 0
	}
	dir := filepath.Join(s.mediaRoot(), id)
	info, err := os.Lstat(dir)
	if err != nil {
		return false, 0
	}
	meta, hasMeta := s.readMediaMeta(id)
	if hasMeta {
		if meta.Overlay {
			return false, 0 // thư viện lớp phủ giữ vĩnh viễn
		}
		if meta.LastUsedAt > 0 && now.Sub(time.UnixMilli(meta.LastUsedAt)) < mediaClaimGrace {
			return false, 0 // vừa được dùng lại: chờ caller liên kết
		}
		born := info.ModTime()
		if meta.CreatedAt > 0 {
			born = time.UnixMilli(meta.CreatedAt)
		}
		if now.Sub(born) < mediaClaimGrace {
			return false, 0
		}
	} else if now.Sub(info.ModTime()) < mediaClaimGrace {
		return false, 0 // lần nhập đang dở (chưa kịp ghi metadata)
	}
	return true, fsio.Bytes(dir)
}

// mediaEligibleFresh đo lại held + điều kiện của một id ngay trước khi xoá
// (trong khoá media/{id}): tham chiếu mới xuất hiện sau lúc dò vẫn thắng GC.
func (s *Service) mediaEligibleFresh(ctx context.Context, id string) (bool, error) {
	held, err := s.heldMedia(ctx)
	if err != nil {
		return false, err
	}
	ok, _ := s.mediaEligible(id, held, s.now())
	return ok, nil
}

// mediaCandidates là media không còn ai giữ (dọn tay, chạy sau preview).
func (s *Service) mediaCandidates(ctx context.Context) ([]Candidate, error) {
	held, err := s.heldMedia(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	var out []Candidate
	for _, id := range s.mediaIDsOnDisk() {
		ok, bytes := s.mediaEligible(id, held, now)
		if !ok {
			continue
		}
		dir := filepath.Join(s.mediaRoot(), id)
		out = append(out, Candidate{
			Item: Item{Kind: "media", Label: labelMedia, Path: dir, Bytes: bytes},
			// Xoá phải re-check held+grace dưới khoá media/{id}: một tham
			// chiếu hoặc lượt dùng mới xuất hiện sau lúc dò vẫn thắng GC —
			// chỗ dọn không phải chỗ quyết định duy nhất.
			Remove: func(ctx context.Context) error {
				unlock := s.locks.lock("media/" + id)
				defer unlock()
				ok, err := s.mediaEligibleFresh(ctx, id)
				if err != nil {
					return err // không chứng minh được → giữ (fail closed)
				}
				if !ok {
					return nil // đã có chủ mới giữ
				}
				return os.RemoveAll(dir)
			},
			Validator: func(ctx context.Context) bool {
				ok, err := s.mediaEligibleFresh(ctx, id)
				return err == nil && ok
			},
		})
	}
	return out, nil
}

// pendingPath là danh sách chờ dọn của editor.
func (s *Service) pendingPath() string {
	return filepath.Join(s.deps.Roots.Root(), "editor", pendingFile)
}

// pendingMedia đọc danh sách id đang chờ dọn lại. Mọi id phải là tên thư mục
// một thành phần an toàn — JSON trên đĩa là dữ liệu người dùng, không được
// tin để join path (`../projects` thoát khỏi thư mục media tới editor).
func (s *Service) pendingMedia() map[string]bool {
	var payload struct {
		IDs []string `json:"ids"`
	}
	ids := map[string]bool{}
	if readJSON(s.pendingPath(), &payload) {
		for _, id := range payload.IDs {
			if isPortableID(id) {
				ids[id] = true
			}
		}
	}
	return ids
}

// writePending ghi danh sách chờ dọn (bỏ hẳn khi rỗng).
func (s *Service) writePending(ids map[string]bool) error {
	if len(ids) == 0 {
		if err := os.Remove(s.pendingPath()); err != nil && !notExist(err) {
			return err
		}
		return nil
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	return writeJSONAtomic(s.pendingPath(), map[string]any{"ids": list})
}

// reclaimMediaOf dọn media của một dự án vừa xoá (chạy ngay sau khi xoá):
// media vừa được dùng lại bị hoãn vào danh sách chờ.
func (s *Service) reclaimMediaOf(ctx context.Context, ids []string) (int64, error) {
	held, err := s.heldMedia(ctx)
	if err != nil {
		return 0, err
	}
	now := s.now()
	pending := s.pendingMedia()
	var freed int64
	for _, id := range ids {
		if !isPortableID(id) || held[strings.ToLower(id)] {
			continue
		}
		dir := filepath.Join(s.mediaRoot(), id)
		ok, _ := s.mediaEligible(id, held, now)
		if !ok {
			if exists(dir) {
				pending[id] = true // còn hạn giữ: thử lại ở lần dọn sau
			}
			continue
		}
		unlock := s.locks.lock("media/" + id)
		bytes := fsio.Bytes(dir)
		err := os.RemoveAll(dir)
		unlock()
		if err != nil {
			return freed, err
		}
		delete(pending, id)
		freed += bytes
		_ = s.dropIndexEntry(dir)
	}
	return freed, s.writePending(pending)
}

// reclaimPending retry các media bị hoãn; id đã được chủ khác giữ thì bỏ.
func (s *Service) reclaimPending(ctx context.Context) error {
	pending := s.pendingMedia()
	if len(pending) == 0 {
		return nil
	}
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	held, err := s.heldMedia(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, id := range ids {
		if !isPortableID(id) {
			delete(pending, id) // id lạ trong file trên đĩa: loại khỏi chờ, không bao giờ join path
			continue
		}
		dir := filepath.Join(s.mediaRoot(), id)
		if !exists(dir) {
			delete(pending, id) // đã bị dọn ở lần trước
			continue
		}
		if held[strings.ToLower(id)] {
			continue // chủ mới đã giữ: thử lại lần sau
		}
		ok, _ := s.mediaEligible(id, held, now)
		if !ok {
			continue // vẫn trong hạn giữ: để lần sau
		}
		unlock := s.locks.lock("media/" + id)
		err := os.RemoveAll(dir)
		unlock()
		if err != nil {
			continue
		}
		delete(pending, id)
		_ = s.dropIndexEntry(dir)
	}
	return s.writePending(pending)
}

// dropIndexEntry gỡ dòng media khỏi editor/media-index.json nếu có (index
// thiếu/hỏng thì để nguyên — phase editor tự dựng lại từ đĩa).
func (s *Service) dropIndexEntry(dir string) error {
	id := filepath.Base(dir)
	path := filepath.Join(s.deps.Roots.Root(), "editor", "media-index.json")
	var payload struct {
		Version string                    `json:"version"`
		Media   map[string]map[string]any `json:"media"`
	}
	if !readJSON(path, &payload) {
		return nil
	}
	if _, ok := payload.Media[id]; !ok {
		return nil
	}
	unlock := s.locks.lock("media-index")
	defer unlock()
	delete(payload.Media, id)
	return writeJSONAtomic(path, payload)
}
