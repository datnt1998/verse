// Package storageimport inventories the declared durable namespaces only.
// Authentication, tools/models, process state and imagegen scratch stay local.
package storageimport

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

type Entry struct {
	Path  string `json:"path"`
	Group string `json:"group"`
	Bytes int64  `json:"bytes"`
}

var namespaces = []struct{ path, group string }{{"library/overlays", "library"}, {"library/backdrops", "library"}, {"channels", "library"}, {"images", "images"}, {"editor", "editor"}, {"sources", "sources"}, {"exports", "exports"}}

// Inventory reports durable bytes without walking the configuration directory.
// It refuses symlinks and unknown paths instead of importing secrets by guess.
func Inventory(ctx context.Context, root string) ([]Entry, error) {
	out := []Entry{}
	handle, err := os.OpenRoot(root)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	err = fs.WalkDir(handle.FS(), ".", func(rel string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if excluded(rel, entry.Name()) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("từ chối liên kết trong dữ liệu bền vững: %s", rel)
		}
		group := ""
		for _, namespace := range namespaces {
			if rel == namespace.path || strings.HasPrefix(rel, namespace.path+"/") {
				group = namespace.group
				break
			}
		}
		if rel == "library" && entry.IsDir() {
			return nil
		}
		if group == "" {
			out = append(out, Entry{Path: rel, Group: "unknown"})
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if err := media.ValidatePath(rel); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("tệp không thường trong dữ liệu bền vững: %s", rel)
		}
		out = append(out, Entry{Path: rel, Group: group, Bytes: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
func excluded(rel, name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".tmp") {
		return true
	}
	return rel == "exports/profiles" || rel == "editor/media-reclaim-pending.json"
}

// RequireKnown refuses a forward migration if durable byte namespaces have no
// current root adapter. It is not evidence that editor/render producers exist.
func RequireKnown(entries []Entry, roots []media.Root) error {
	known := make(map[string]bool, len(roots))
	for _, root := range roots {
		known[root.Path] = true
	}
	unknown := []string{}
	for _, entry := range entries {
		if !known[entry.Path] {
			unknown = append(unknown, entry.Path)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("chưa xác định được tham chiếu bền vững cho %s; giữ nguyên kho cũ", strings.Join(unknown, ", "))
	}
	return nil
}
