package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DirEntry mirrors os.DirEntry without the fs package surface so the
// document backend can answer directory listings it synthesizes from rows.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ioOps là bề mặt vận hành mà IO ủy quyền: fileOps ghi filesystem thật,
// IOOps is the storage primitive seam: filesystem for the file backend, typed
// document rows for the postgres backend. Backends outside this package
// implement it (store.NewIOFromOps) so the same substore logic runs on both.
type IOOps interface {
	Read(rel string) ([]byte, error)
	Write(rel string, data []byte) error
	Append(rel string, data []byte) error
	Sync(rel string) error
	Remove(rel string) error
	RemoveTree(rel string) error
	Truncate(rel string, size int64) error
	ListDir(rel string) ([]DirEntry, error)
	Exists(rel string) (bool, error)
	MkdirAll(dirs []string) error
	// BeginWrite/EndWrite bound an exclusive write region (lockWrite): the
	// file backend no-ops (the mutex already serializes), postgres pins one
	// transaction for every op in the region. EndWrite finishes that region
	// and returns its commit failure, which the region's caller receives.
	BeginWrite()
	EndWrite() error
}

// ioMutex exposes only shared locking. Exclusive regions go through
// IO.lockWrite so a region can never end without reporting its commit.
type ioMutex struct {
	rw sync.RWMutex
}

func (m *ioMutex) RLock()   { m.rw.RLock() }
func (m *ioMutex) RUnlock() { m.rw.RUnlock() }

// IO 封装文件系统读写操作，提供加锁和原子写入。
// 每个子存储持有独立的 IO 实例，拥有各自的锁。
type IO struct {
	ops IOOps
	mu  ioMutex
}

func (io *IO) labels() mdLabels { return mdLabelsVI }

func newIO(dir string) *IO {
	return NewIOFromOps(fileOps{dir: dir})
}

// NewIO creates an atomic JSON/file store rooted at dir.
func NewIO(dir string) *IO { return newIO(dir) }

// NewIOFromOps builds an IO over a non-filesystem ops implementation. Used by
// the postgres backend to run the same substore logic on document rows.
func NewIOFromOps(ops IOOps) *IO {
	return &IO{ops: ops}
}

func (io *IO) ReadFile(rel string) ([]byte, error) {
	io.mu.RLock()
	defer io.mu.RUnlock()
	return io.ReadFileUnlocked(rel)
}

func (io *IO) ReadFileUnlocked(rel string) ([]byte, error) {
	return io.ops.Read(rel)
}

func (io *IO) WriteFileUnlocked(rel string, data []byte) error {
	return io.ops.Write(rel, data)
}

func (io *IO) ReadJSON(rel string, v any) error {
	io.mu.RLock()
	defer io.mu.RUnlock()
	return io.ReadJSONUnlocked(rel, v)
}

func (io *IO) ReadJSONUnlocked(rel string, v any) error {
	data, err := io.ReadFileUnlocked(rel)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (io *IO) WriteJSON(rel string, v any) (err error) {
	defer io.lockWrite()(&err)
	return io.WriteJSONUnlocked(rel, v)
}

func (io *IO) WriteJSONUnlocked(rel string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return io.WriteFileUnlocked(rel, data)
}

func (io *IO) WriteMarkdown(rel string, content string) (err error) {
	defer io.lockWrite()(&err)
	return io.WriteFileUnlocked(rel, []byte(content))
}

// WriteMarkdownUnlocked 写出 .md sidecar。约定：每个 .md 都是对应 .json 的
// best-effort 人类可读视图，绝非数据源——运行时与导出一律从 .json 重新渲染。
// 各 Save 方法在同一写锁内先写 .json 再写此 .md，是两次独立的 tmp+rename；
// 二者之间崩溃会留下 .md 落后于 .json，这是可接受的（无人把 .md 当数据读，
// 下次写同一 scope 即自愈）。故意不为此加两文件原子提交——那是过度设计。
func (io *IO) WriteMarkdownUnlocked(rel string, content string) error {
	return io.WriteFileUnlocked(rel, []byte(content))
}

func (io *IO) AppendLine(rel string, data []byte) (err error) {
	defer io.lockWrite()(&err)
	return io.AppendLineUnlocked(rel, data)
}

func (io *IO) AppendLineUnlocked(rel string, data []byte) error {
	return io.ops.Append(rel, data)
}

// syncFileUnlocked 在幂等重放时确认已存在的追加记录已持久化。
// 调用方负责持有 io.mu 写锁。
func (io *IO) syncFileUnlocked(rel string) error {
	return io.ops.Sync(rel)
}

func (io *IO) RemoveFile(rel string) (err error) {
	defer io.lockWrite()(&err)
	return io.RemoveFileUnlocked(rel)
}

func (io *IO) RemoveFileUnlocked(rel string) error {
	return io.ops.Remove(rel)
}

// RemoveTreeUnlocked xoá cả một "thư mục" tài liệu (mọi key dưới prefix).
func (io *IO) RemoveTreeUnlocked(rel string) error {
	return io.ops.RemoveTree(rel)
}

// TruncateFileUnlocked cắt file về đúng size byte đầu — giao thức sửa đuôi
// append log. Backend quan hệ thực hiện bằng cách xoá các record sau mốc.
func (io *IO) TruncateFileUnlocked(rel string, size int64) error {
	return io.ops.Truncate(rel, size)
}

// ListDirUnlocked liệt kê một thư mục tài liệu; thiếu → ErrNotExist parity
// os.ReadDir. Backend quan hệ synthesize từ key của các hàng đã lưu.
func (io *IO) ListDirUnlocked(rel string) ([]DirEntry, error) {
	return io.ops.ListDir(rel)
}

// ListDir liệt kê thư mục tài liệu dưới read lock, cùng semantics với
// ListDirUnlocked (thiếu thư mục → lỗi os.ErrNotExist).
func (io *IO) ListDir(rel string) ([]DirEntry, error) {
	io.mu.RLock()
	defer io.mu.RUnlock()
	return io.ops.ListDir(rel)
}

// ExistsUnlocked báo tài liệu có tồn tại (parity os.Stat success).
func (io *IO) ExistsUnlocked(rel string) (bool, error) {
	return io.ops.Exists(rel)
}

// WithWriteLock runs fn in one exclusive write region and returns fn's error,
// or the region's commit failure when fn succeeded.
func (io *IO) WithWriteLock(fn func() error) (err error) {
	defer io.lockWrite()(&err)
	return fn()
}

// lockWrite takes the exclusive lock and opens the backend write region. The
// returned func ends the region and unlocks; it stores the region's commit
// failure in *err unless the body already failed. Use it as
// `defer io.lockWrite()(&err)` with a named error result.
func (io *IO) lockWrite() func(*error) {
	io.mu.rw.Lock()
	io.ops.BeginWrite()
	return func(err *error) {
		defer io.mu.rw.Unlock()
		if cerr := io.ops.EndWrite(); cerr != nil && *err == nil {
			*err = cerr
		}
	}
}

// EnsureDirs 创建指定的子目录。
func (io *IO) EnsureDirs(dirs []string) error {
	return io.ops.MkdirAll(dirs)
}

// ── fileOps: filesystem implementation ──────────────────────────────────────

type fileOps struct{ dir string }

func (f fileOps) path(rel string) string { return filepath.Join(f.dir, rel) }

func (f fileOps) Read(rel string) ([]byte, error) {
	return os.ReadFile(f.path(rel))
}

func (f fileOps) Write(rel string, data []byte) error {
	p := f.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func (f fileOps) Append(rel string, data []byte) error {
	p := f.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if _, err = file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func (f fileOps) Sync(rel string) error {
	file, err := os.OpenFile(f.path(rel), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (f fileOps) Remove(rel string) error {
	err := os.Remove(f.path(rel))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (f fileOps) RemoveTree(rel string) error {
	return os.RemoveAll(f.path(rel))
}

func (f fileOps) Truncate(rel string, size int64) error {
	return os.Truncate(f.path(rel), size)
}

func (f fileOps) ListDir(rel string) ([]DirEntry, error) {
	entries, err := os.ReadDir(f.path(rel))
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return out, nil
}

func (f fileOps) Exists(rel string) (bool, error) {
	if _, err := os.Stat(f.path(rel)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (f fileOps) MkdirAll(dirs []string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(f.dir, d), 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func (f fileOps) BeginWrite()     {}
func (f fileOps) EndWrite() error { return nil }
