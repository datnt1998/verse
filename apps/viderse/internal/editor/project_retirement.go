package editor

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

var generatedProjectRevision = regexp.MustCompile(`^[A-Z2-7]{26}\.json$`)

// A tombstone, matching immutable payload and absence of every binding/root
// distinguish a released project revision from an unknown startup file. This
// repairs only the old DeleteProject omission; it is not a prefix collector.
func (s *Service) reclaimDeletedProjectRevisions(ctx context.Context) error {
	proof, ok := s.repository().(interface {
		LockDeletedProject(context.Context, string) (bool, error)
	})
	if !ok {
		return nil
	}
	backend, ok := s.Media.(interface {
		Active(context.Context) (blobstore.BlobStore, media.Descriptor, media.State, error)
		Repository() media.Repository
	})
	if !ok {
		return nil
	}
	_, descriptor, _, err := backend.Active(ctx)
	if err != nil {
		return err
	}
	if descriptor.Config.Driver != "local" {
		return nil
	}
	root, err := os.OpenRoot(descriptor.Config.LocalRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	projects, err := editorRootEntries(root, "editor/projects")
	if isMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, directory := range projects {
		id := directory.Name()
		if !directory.IsDir() || ValidateID(id) != nil {
			continue
		}
		prefix := "editor/projects/" + id + "/revisions/"
		entries, err := editorRootEntries(root, prefix)
		if isMissing(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !generatedProjectRevision.MatchString(entry.Name()) {
				continue
			}
			path := prefix + entry.Name()
			err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
				deleted, err := proof.LockDeletedProject(tx, id)
				if err != nil || !deleted {
					return err
				}
				roots, err := backend.Repository().Roots(tx)
				if err != nil {
					return err
				}
				for _, live := range roots {
					if live.Path == path {
						return nil
					}
				}
				state, err := backend.Repository().State(tx)
				if err != nil {
					return err
				}
				_, err = backend.Repository().Resolve(tx, state.Active, path)
				if err == nil {
					return nil
				}
				if !errors.Is(err, blobstore.ErrNotFound) {
					return err
				}
				file, err := root.Open(filepath.FromSlash(path))
				if isMissing(err) {
					return nil
				}
				if err != nil {
					return err
				}
				data, readErr := io.ReadAll(file)
				closeErr := file.Close()
				if readErr != nil {
					return readErr
				}
				if closeErr != nil {
					return closeErr
				}
				project, err := Parse(data)
				if err != nil || fillString(Object(project), "id") != id || fillNum(Object(project), "schemaVersion", 0) != SchemaVersion {
					return nil
				}
				// Removal occurs under the publication and project fences. All durable
				// ownership was already retired in the original deletion transaction.
				err = root.Remove(filepath.FromSlash(path))
				if isMissing(err) {
					return nil
				}
				return err
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
