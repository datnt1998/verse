package media

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/gofrs/flock"
)

func (c *Cache) recoverWork() error {
	entries, err := os.ReadDir(filepath.Join(c.dir, "work"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "input-") {
			continue
		}
		dir := filepath.Join(c.dir, "work", entry.Name())
		owner := filepath.Join(dir, "owner.lock")
		if _, err := os.Lstat(owner); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		lock := flock.New(owner)
		ok, err := lock.TryLock()
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		removeErr := os.RemoveAll(dir)
		unlockErr := lock.Unlock()
		if removeErr != nil {
			return removeErr
		}
		if unlockErr != nil {
			return unlockErr
		}
	}
	return nil
}
func removePartials(dir, hash string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	prefix := ".hydrate-" + hash + "-"
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".part") {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}
func (c *Cache) workspace(ctx context.Context, store blobstore.BlobStore, ref Ref) (*Handle, error) {
	hidden, err := os.MkdirTemp(filepath.Join(c.dir, "work"), ".input-")
	if err != nil {
		return nil, err
	}
	owner := flock.New(filepath.Join(hidden, "owner.lock"))
	ok, err := owner.TryLock()
	if err != nil || !ok {
		os.RemoveAll(hidden)
		if err == nil {
			err = ErrBusy
		}
		return nil, err
	}
	dir := filepath.Join(filepath.Dir(hidden), strings.TrimPrefix(filepath.Base(hidden), "."))
	if err := os.Rename(hidden, dir); err != nil {
		owner.Unlock()
		os.RemoveAll(hidden)
		return nil, err
	}
	cleanup := func() {
		if err := owner.Unlock(); err != nil {
			slog.Warn("cache: không nhả được khoá tệp làm việc")
		}
		if err := os.RemoveAll(dir); err != nil {
			slog.Warn("cache: không dọn được tệp làm việc")
		}
	}
	path := filepath.Join(dir, "input")
	if err := download(ctx, store, ref, path); err != nil {
		cleanup()
		return nil, err
	}
	return &Handle{Path: path, Ref: ref, release: cleanup}, nil
}
