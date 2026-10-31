package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// Local logical files have no bucket GC. Recheck the binding while holding the
// same content lock as publication, so a concurrent new owner cannot lose it.
func retireLocalCutouts(ctx context.Context, files *media.Service, paths []string) {
	for _, path := range paths {
		if media.ValidatePath(path) != nil { continue }
		hash := strings.TrimSuffix(filepath.Base(path), ".png")
		key, err := media.ContentKey(hash)
		if err != nil { continue }
		_ = files.Publish(ctx, nil, func(tx context.Context) error {
			_, descriptor, state, err := files.Active(tx)
			if err != nil || descriptor.Config.Driver != "local" { return err }
			repo := files.Repository()
			if err = repo.Lock(tx, state.Active, key); err != nil { return err }
			_, err = repo.Resolve(tx, state.Active, path)
			if err == nil { return nil }
			if !errors.Is(err, blobstore.ErrNotFound) { return err }
			root, err := os.OpenRoot(descriptor.Config.LocalRoot)
			if err != nil { return err }
			defer root.Close()
			err = root.Remove(filepath.FromSlash(path))
			if errors.Is(err, os.ErrNotExist) { return nil }
			return err
		})
	}
}
