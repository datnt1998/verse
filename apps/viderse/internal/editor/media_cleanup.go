package editor

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"os"
)

// unbindPaths removes logical ownership on both drivers. Local source files have
// no bucket collector, so explicit editor reclamation removes their confined
// paths after detaching them. S3 content is left to the shared grace-period GC,
// because another logical path may still reference the same content key.
func (s *Service) unbindPaths(ctx context.Context, paths []string, commits ...func(context.Context) error) error {
	type localPath struct {
		store   blobstore.BlobStore
		path    string
		release func()
	}
	var local []localPath
	resolver, ok := s.Media.(interface {
		Resolve(context.Context, string) (blobstore.BlobStore, media.Descriptor, media.Ref, func(), error)
	})
	if ok {
		for _, path := range paths {
			store, descriptor, ref, release, err := resolver.Resolve(ctx, path)
			if isMissing(err) {
				continue
			}
			if err != nil {
				for _, entry := range local {
					entry.release()
				}
				return err
			}
			if descriptor.Config.Driver == "local" {
				local = append(local, localPath{store, ref.Path, release})
			} else {
				release()
			}
		}
	}
	defer func() {
		for _, entry := range local {
			entry.release()
		}
	}()
	var commit func(context.Context) error
	if len(commits) > 0 {
		commit = commits[0]
	}
	if err := s.Media.Unbind(ctx, paths, commit); err != nil {
		return err
	}
	var failures []error
	for _, entry := range local {
		if err := entry.store.Delete(ctx, entry.path); err != nil && !isMissing(err) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// discardReleasedLocal removes a local logical file after a publication commit
// released its binding. Pointer-owned JSON revision files have no bucket-side
// grace collector, so the confined file is deleted only when nothing still
// binds the path — clone_artifacts, another pointer, or a rebound name all keep
// it. Release errors are silent: unbound paths are never re-bound.
type discardBackend interface {
	Resolve(context.Context, string) (blobstore.BlobStore, media.Descriptor, media.Ref, func(), error)
	Active(context.Context) (blobstore.BlobStore, media.Descriptor, media.State, error)
}

func discardReleasedLocal(ctx context.Context, backend Media, paths ...string) {
	svc, ok := backend.(discardBackend)
	if !ok || svc == nil {
		return
	}
	_, descriptor, _, err := svc.Active(ctx)
	if err != nil || descriptor.Config.Driver != "local" {
		return
	}
	root, err := os.OpenRoot(descriptor.Config.LocalRoot)
	if err != nil {
		return
	}
	defer root.Close()
	for _, path := range paths {
		if path == "" {
			continue
		}
		_, _, _, release, err := svc.Resolve(ctx, path)
		if release != nil {
			release()
		}
		switch {
		case err == nil:
			continue // another live owner still binds the path
		case !errors.Is(err, blobstore.ErrNotFound):
			continue // fail closed on unresolved manifests
		}
		if err := media.ValidatePath(path); err != nil {
			continue
		}
		if err := root.Remove(filepath.FromSlash(path)); err != nil && !isMissing(err) {
			continue
		}
	}
}
