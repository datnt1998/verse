package editor

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
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
