package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/fsio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Reclaim ids của hai thẻ "xoá riêng" trong Cài đặt › Lưu trữ.
const (
	ReclaimSource = "src" // video gốc của video đã xong
	ReclaimMP4    = "mp4" // MP4 của video đã đăng
)

// ReclaimItem là một thẻ xoá riêng trên UI.
type ReclaimItem struct {
	ID    string
	Title string
	Bytes int64
	Count int
	Desc  string
}

// reclaimCache giữ kết quả dò thẻ xoá (30 giây): GET /api/storage/reclaim
// không đi cây mỗi request; move/dọn gọi Changed để xoá cache.
const reclaimTTL = 30 * time.Second

type reclaimCache struct {
	mu    sync.Mutex
	at    time.Time
	items []ReclaimItem
}

// ReclaimList dựng hai thẻ xoá của mục Lưu trữ. Video do phase pipeline ghi;
// chưa có video nào → count/bytes bằng 0 (sự thật, không phải chỗ trống).
func (s *Service) ReclaimList(ctx context.Context) ([]ReclaimItem, error) {
	s.reclaim.mu.Lock()
	if !s.reclaim.at.IsZero() && s.now().Sub(s.reclaim.at) < reclaimTTL {
		items := append([]ReclaimItem(nil), s.reclaim.items...)
		s.reclaim.mu.Unlock()
		return items, nil
	}
	s.reclaim.mu.Unlock()

	src, err := s.sourceItem(ctx)
	if err != nil {
		return nil, err
	}
	mp4, err := s.mp4Item(ctx)
	if err != nil {
		return nil, err
	}
	items := []ReclaimItem{src, mp4}
	s.reclaim.mu.Lock()
	s.reclaim.at, s.reclaim.items = s.now(), items
	s.reclaim.mu.Unlock()
	return items, nil
}

// invalidateReclaim quên cache thẻ xoá.
func (s *Service) invalidateReclaim() {
	s.reclaim.mu.Lock()
	s.reclaim.at = time.Time{}
	s.reclaim.mu.Unlock()
}

// sourceItem dựng thẻ "Video gốc của video đã xong".
func (s *Service) sourceItem(ctx context.Context) (ReclaimItem, error) {
	item := ReclaimItem{ID: ReclaimSource, Title: "Video gốc của video đã xong"}
	videos, err := s.deps.Videos.Finished(ctx)
	if err != nil {
		return item, err
	}
	for _, v := range videos {
		bytes := sourceArtifactBytes(s.deps.Roots.Root(), v.ID)
		mediaBytes, mediaExists := s.mediaBytes(v.SourceMediaID)
		if bytes > 0 || mediaExists {
			if bytes == 0 {
				bytes = mediaBytes
			}
			item.Bytes += bytes
			item.Count++
		}
	}
	// Câu mô tả nói đúng số video bị ảnh hưởng (artboard: “của 14 video đã xong”).
	item.Desc = fmt.Sprintf("Xoá video tải từ đối thủ của %d video đã xong. Project vẫn mở và xuất lại được; chỉ khi Làm lại từ đầu mới phải tải lại.", item.Count)
	return item, nil
}

// sourceArtifactBytes là dung lượng video gốc còn trên đĩa của một video.
func sourceArtifactBytes(root, videoID string) int64 {
	if !isPortableID(videoID) {
		return 0
	}
	dir := filepath.Join(root, "sources", videoID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		if e.IsDir() || !strings.Contains(e.Name(), "source") || strings.HasSuffix(e.Name(), ".json") {
			continue // bằng chứng nhỏ (info, thumbnail) ở lại
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}

// mediaBytes trả dung lượng thư mục media (và có tồn tại không).
func (s *Service) mediaBytes(id string) (int64, bool) {
	if id == "" || !isPortableID(id) {
		return 0, false
	}
	dir := filepath.Join(s.mediaRoot(), id)
	if !exists(dir) {
		return 0, false
	}
	return fsio.Bytes(dir), true
}

// mp4Item dựng thẻ "MP4 của video đã đăng".
func (s *Service) mp4Item(ctx context.Context) (ReclaimItem, error) {
	item := ReclaimItem{ID: ReclaimMP4, Title: "MP4 của video đã đăng"}
	videos, err := s.deps.Videos.Posted(ctx)
	if err != nil {
		return item, err
	}
	for _, v := range videos {
		if bytes := exportBytes(s.deps.Roots.Root(), v.ID); bytes > 0 {
			item.Bytes += bytes
			item.Count++
		}
	}
	item.Desc = fmt.Sprintf("%d video đã đăng lên YouTube. Giữ project, ảnh bìa và ảnh cảnh; cần lại thì xuất lại là có MP4.", item.Count)
	return item, nil
}

// exportBytes cộng dung lượng MP4 của một video.
func exportBytes(root, videoID string) int64 {
	if !isPortableID(videoID) {
		return 0
	}
	dir := filepath.Join(root, "exports", videoID)
	var total int64
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".mp4") {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Reclaim xoá một nhóm (sau xác nhận phía client). Không preview vì chỉ chạm
// file của video đã xong/đã đăng — đúng thẻ của design.
func (s *Service) Reclaim(ctx context.Context, id string) (ReclaimItem, error) {
	switch id {
	case ReclaimSource:
		item, err := s.reclaimSources(ctx)
		s.invalidateReclaim()
		if err == nil && s.deps.Changed != nil {
			s.deps.Changed("")
		}
		return item, err
	case ReclaimMP4:
		item, err := s.reclaimExports(ctx)
		s.invalidateReclaim()
		if err == nil && s.deps.Changed != nil {
			s.deps.Changed("")
		}
		return item, err
	default:
		return ReclaimItem{}, &Error{Code: "unknown_kind", Message: "nhóm xoá không biết: " + id}
	}
}

// reclaimSources xoá video gốc của video đã xong, giữ bằng chứng nhỏ và gỡ
// liên kết media của project (nếu timeline không dùng).
func (s *Service) reclaimSources(ctx context.Context) (ReclaimItem, error) {
	item, err := s.sourceItem(ctx)
	if err != nil {
		return item, err
	}
	videos, err := s.deps.Videos.Finished(ctx)
	if err != nil {
		return item, err
	}
	var freed int64
	var count int
	var failed error
	for _, v := range videos {
		if !isPortableID(v.ID) {
			continue
		}
		dir := filepath.Join(s.deps.Roots.Root(), "sources", v.ID)
		removed, err := removeSourceFiles(dir)
		if err != nil {
			failed = err
			continue
		}
		if removed > 0 {
			count++
		}
		freed += removed
		if v.ProjectID != "" && v.SourceMediaID != "" {
			unlock := s.locks.lock("project/" + v.ProjectID)
			if dropErr := s.dropMediaLink(v.ProjectID, v.SourceMediaID); dropErr != nil {
				failed = dropErr
			}
			unlock()
			mediaFreed, err := s.reclaimMediaOf(ctx, []string{v.SourceMediaID})
			if err == nil && removed == 0 {
				// Không có bản artifact: chỗ lấy lại được là của media.
				freed += mediaFreed
			}
		}
	}
	if err := s.reclaimPending(ctx); err != nil && failed == nil {
		failed = err
	}
	item.Bytes, item.Count = freed, count
	return item, failed
}

// removeSourceFiles xoá các bản video gốc trong một thư mục video, giữ tệp
// JSON bằng chứng (info, phụ đề nhỏ).
func removeSourceFiles(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if notExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var freed int64
	for _, e := range entries {
		if e.IsDir() || !strings.Contains(e.Name(), "source") || strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return freed, err
		}
		freed += info.Size()
	}
	return freed, nil
}

// reclaimExports xoá MP4 của video đã đăng, giữ phần còn lại của bản xuất.
func (s *Service) reclaimExports(ctx context.Context) (ReclaimItem, error) {
	item, err := s.mp4Item(ctx)
	if err != nil {
		return item, err
	}
	videos, err := s.deps.Videos.Posted(ctx)
	if err != nil {
		return item, err
	}
	var freed int64
	var count int
	for _, v := range videos {
		bytes, err := removeMP4s(s.deps.Roots.Root(), v.ID)
		if err != nil {
			return item, err
		}
		if bytes > 0 {
			freed += bytes
			count++
		}
	}
	item.Bytes, item.Count = freed, count
	return item, nil
}

// removeMP4s xoá mọi tệp .mp4 của một video và dọn thư mục rỗng còn lại.
func removeMP4s(root, videoID string) (int64, error) {
	if !isPortableID(videoID) {
		return 0, nil
	}
	dir := filepath.Join(root, "exports", videoID)
	if !exists(dir) {
		return 0, nil
	}
	var freed int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mp4") {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		freed += info.Size()
		return nil
	})
	if err != nil {
		return freed, err
	}
	pruneEmptyDirs(dir)
	return freed, nil
}

// pruneEmptyDirs xoá các thư mục rỗng dưới dir, rồi chính dir nếu rỗng.
func pruneEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			child := filepath.Join(dir, e.Name())
			pruneEmptyDirs(child)
		}
	}
	if remaining, err := os.ReadDir(dir); err == nil && len(remaining) == 0 {
		_ = os.Remove(dir)
	}
}

// dropMediaLink gỡ một media khỏi project.json và media-links.json — chỉ khi
// phần còn lại của project không còn nhắc id đó (media trên timeline thì
// giữ). Ghi nguyên tử.
func (s *Service) dropMediaLink(projectID, mediaID string) error {
	if !isPortableID(projectID) || mediaID == "" {
		return nil
	}
	dir := filepath.Join(s.deps.Roots.Root(), "editor", "projects", projectID)
	projectPath := filepath.Join(dir, "project.json")
	raw, err := os.ReadFile(projectPath)
	if err != nil {
		if notExist(err) {
			return nil
		}
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil // project hỏng: không sửa mù
	}
	items, _ := doc["mediaItems"].([]any)
	if items == nil {
		return nil
	}
	kept := make([]any, 0, len(items))
	found := false
	for _, entry := range items {
		row, ok := entry.(map[string]any)
		if ok && row["id"] == mediaID {
			found = true
			continue
		}
		kept = append(kept, entry)
	}
	if !found {
		return nil
	}
	// Phần còn lại (timeline, clip…) nhắc tới media → giữ liên kết.
	doc["mediaItems"] = kept
	rest, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(rest)), strings.ToLower(mediaID)) {
		return nil
	}
	if err := writeJSONAtomic(projectPath, json.RawMessage(rest)); err != nil {
		return err
	}
	// media-links.json: bỏ id khỏi danh sách liên kết.
	linksPath := filepath.Join(dir, "media-links.json")
	var links struct {
		Version string   `json:"version"`
		IDs     []string `json:"ids"`
	}
	if readJSON(linksPath, &links) && len(links.IDs) > 0 {
		keptLinks := make([]string, 0, len(links.IDs))
		for _, id := range links.IDs {
			if id != mediaID {
				keptLinks = append(keptLinks, id)
			}
		}
		links.IDs = keptLinks
		_ = writeJSONAtomic(linksPath, links)
	}
	return nil
}
