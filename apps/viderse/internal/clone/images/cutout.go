package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type cutoutIndex interface {
	FindCutout(context.Context, string) (string, error)
	OwnCutout(context.Context, string, string, string) error
}

type cutoutLock struct {
	mu    sync.Mutex
	users int
}

var cutoutLocks = struct {
	sync.Mutex
	entries map[string]*cutoutLock
}{entries: make(map[string]*cutoutLock)}

func lockCutout(hash string) func() {
	cutoutLocks.Lock()
	l := cutoutLocks.entries[hash]
	if l == nil {
		l = &cutoutLock{}
		cutoutLocks.entries[hash] = l
	}
	l.users++
	cutoutLocks.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		cutoutLocks.Lock()
		l.users--
		if l.users == 0 {
			delete(cutoutLocks.entries, hash)
		}
		cutoutLocks.Unlock()
	}
}

// Cutout derives an immutable preview without mutating the image checkpoint.
func (s *Service) Cutout(ctx context.Context, id, itemID string, number int) (string, error) {
	l, err := s.load(ctx, id, nil, false)
	if err != nil {
		return "", err
	}
	var item *model.Item
	for i := range l.manifest.Items {
		if l.manifest.Items[i].ID == itemID {
			item = &l.manifest.Items[i]
			break
		}
	}
	if item == nil {
		return "", cloneError(domain.CloneErrNotFound, "Không tìm thấy ảnh")
	}
	var v *model.Version
	if number == 0 {
		v = model.Displayed(*item)
	} else {
		for i := range item.Versions {
			if item.Versions[i].Version == number {
				v = &item.Versions[i]
				break
			}
		}
	}
	if v == nil {
		return "", cloneError(domain.CloneErrNotFound, "Không tìm thấy phiên bản ảnh")
	}
	if runtime.GOOS != "darwin" || s.Runner == nil {
		return "", cloneError(domain.CloneErrToolMissing, "Tách nền chưa khả dụng trên máy này")
	}
	index, ok := s.Edits.Store.(cutoutIndex)
	if !ok || s.Edits.Media == nil {
		return "", cloneError(domain.CloneErrToolMissing, "Kho tệp chưa sẵn sàng")
	}
	source, err := s.Edits.Acquire(ctx, l.snapshot, v.Path)
	if err != nil {
		return "", err
	}
	defer source.Release()
	f, err := os.Open(source.Path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if v.SHA256 != "" && v.SHA256 != hash {
		return "", cloneError(domain.CloneErrInternal, "Ảnh gốc không khớp dữ liệu đã lưu")
	}
	unlock := lockCutout(hash)
	defer unlock()
	path, err := index.FindCutout(ctx, hash)
	if err != nil {
		return "", err
	}
	if path != "" {
		err = s.Edits.Media.Publish(ctx, nil, func(tx context.Context) error { return index.OwnCutout(tx, id, hash, path) })
		return path, err
	}
	dir, err := os.MkdirTemp("", "viderse-cutout-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "cutout.png")
	_, err = s.Runner.Run(ctx, "/usr/bin/osascript", []string{"-l", "JavaScript", "-e", model.CutoutScript, source.Path, output}, tools.Spec{Timeout: 2 * time.Minute})
	if err != nil {
		return "", cloneError(domain.CloneErrToolMissing, "Không thể tách nền ảnh này")
	}
	f, err = os.Open(output)
	if err != nil {
		return "", cloneError(domain.CloneErrToolMissing, "Không thể tách nền ảnh này")
	}
	cfg, decodeErr := png.DecodeConfig(f)
	f.Close()
	if decodeErr != nil || cfg.Width != v.Width || cfg.Height != v.Height {
		return "", cloneError(domain.CloneErrToolMissing, "Không thể tách nền ảnh này")
	}
	f, err = os.Open(output)
	if err != nil {
		return "", err
	}
	resultHash := sha256.New()
	_, err = io.Copy(resultHash, f)
	closeErr = f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	path = "clone/cutouts/" + hash + "/" + hex.EncodeToString(resultHash.Sum(nil)) + ".png"
	err = s.Edits.Media.Publish(ctx, []media.Artifact{{Path: path, File: output, ContentType: "image/png", Filename: "subject.png"}}, func(tx context.Context) error { return index.OwnCutout(tx, id, hash, path) })
	if err != nil {
		return "", err
	}
	return path, nil
}
