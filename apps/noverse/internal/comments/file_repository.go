package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

// fileRepository là Repository theo thư mục: <truyện>/.ainovel/gop-y/<chương>.json
// với khoá flock từng tệp và chính văn đọc từ <truyện>/output/novel. Giữ nguyên
// saga file cũ (ghi atomic qua tệp tạm + rename) — store nghiệp vụ không còn
// chạm đĩa trực tiếp.
type fileRepository struct {
	dir       string // <truyện>/.ainovel/gop-y
	engineDir string // <truyện>/output/novel — đọc chính văn để giải mốc neo
}

func newFileRepository(storyDir, engineDir string) *fileRepository {
	return &fileRepository{dir: filepath.Join(storyDir, ".ainovel", "gop-y"), engineDir: engineDir}
}

// commentsDir exposes the on-disk directory for callers that only inspect it
// (hasComments fast path).
func (r *fileRepository) commentsDir() string { return r.dir }

func (r *fileRepository) file(chapter int) string {
	return filepath.Join(r.dir, fmt.Sprintf("%d.json", chapter))
}

func (r *fileRepository) readRaw(chapter int) ([]commentFile, error) {
	raw := []commentFile{}
	data, err := os.ReadFile(r.file(chapter))
	if err != nil {
		if os.IsNotExist(err) {
			return raw, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("Đọc góp ý: %w", err)
	}
	if raw == nil {
		raw = []commentFile{}
	}
	return raw, nil
}

func (r *fileRepository) writeList(chapter int, out []Comment) error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	path := r.file(chapter)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(r.dir, ".gop-y-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// withLock runs fn while holding the chapter file's flock; the read happens
// inside so the mutate/persist window is atomic against other file writers.
func (r *fileRepository) withLock(chapter int, fn func() error) error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	lock := flock.New(r.file(chapter)+".lock", flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Close()
	return fn()
}

func (r *fileRepository) Transact(chapter int, fn func([]Comment) ([]Comment, bool, error)) error {
	if err := validateChapter(chapter); err != nil {
		return err
	}
	return r.withLock(chapter, func() error {
		raw, err := r.readRaw(chapter)
		if err != nil {
			return err
		}
		// Migrate + làm mới mốc neo trên chính văn hiện tại (đọc lười: chỉ khi
		// có góp ý có thể neo).
		var paras []string
		needText := false
		for _, c := range raw {
			if c.anchorEligible() && c.Status != StatusDone {
				needText = true
				break
			}
		}
		if needText {
			if text, err := r.ChapterText(chapter); err == nil && text != "" {
				paras = normalizedParagraphs(text)
			}
		}
		list, migrated := migrateFile(raw, paras, time.Now().UTC())
		out, changed, err := fn(list)
		if err != nil {
			return err
		}
		if !changed && !migrated {
			return nil
		}
		return r.writeList(chapter, out)
	})
}

func (r *fileRepository) TransactAt(chapter int, paras []string, fn func([]Comment) ([]Comment, bool, error)) error {
	if err := validateChapter(chapter); err != nil {
		return err
	}
	return r.withLock(chapter, func() error {
		raw, err := r.readRaw(chapter)
		if err != nil {
			return err
		}
		list, _ := migrateFile(raw, nil, time.Now().UTC())
		out, changed, err := fn(list)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return r.writeList(chapter, out)
	})
}

// ChapterText đọc chính văn đang hiển thị: bản nháp nếu đang viết lại (đó là
// bản người dùng nhìn thấy), không thì bản cuối. Chỉ dùng cho các thao tác
// đọc/tạo/gắn lại — các lần ghi luôn truyền nội dung vừa ghi tường minh.
func (r *fileRepository) ChapterText(chapter int) (string, error) {
	for _, rel := range []string{
		filepath.Join("drafts", fmt.Sprintf("%02d.draft.md", chapter)),
		filepath.Join("chapters", fmt.Sprintf("%02d.md", chapter)),
	} {
		data, err := os.ReadFile(filepath.Join(r.engineDir, rel))
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), nil
		}
	}
	return "", nil
}

func (r *fileRepository) Chapters() ([]int, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, nil
	}
	out := []int{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == eventsFileName {
			continue
		}
		var chapter int
		if _, err := fmt.Sscanf(e.Name(), "%d.json", &chapter); err != nil || chapter < 1 {
			continue
		}
		out = append(out, chapter)
	}
	sort.Ints(out)
	return out, nil
}

func (r *fileRepository) ReadRaw(chapter int) ([]Comment, error) {
	raw, err := r.readRaw(chapter)
	if err != nil {
		return nil, err
	}
	list, _ := migrateFile(raw, nil, time.Now().UTC())
	return list, nil
}

func (r *fileRepository) WithEvents(fn func([]ReanchorEvent) []ReanchorEvent) error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(r.dir, eventsFileName)
	lock := flock.New(path+".lock", flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Close()
	events := []ReanchorEvent{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &events)
	} else if !os.IsNotExist(err) {
		return err
	}
	events = fn(events)
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(r.dir, ".gop-y-events-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
