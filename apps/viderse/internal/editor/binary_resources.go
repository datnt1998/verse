package editor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// publishJSONFile commits the binary's durable owner and its authoritative JSON
// head together. Neither an interrupted upload nor a failed metadata CAS can
// expose a half-replaced file or leave committed bytes without a root adapter.
func (s *Service) publishJSONFile(ctx context.Context, key, legacyPath string, value Object, expected string, file media.Artifact, fileKey, fileRevision string, previous Resource) (string, error) {
	repo := s.repository()
	if repo == nil {
		return "", errors.New("editor repository is required")
	}
	prior, err := repo.Get(ctx, key)
	if err != nil && !isMissing(err) {
		return "", err
	}
	binary, err := repo.Get(ctx, fileKey)
	if isMissing(err) {
		binary = previous
	} else if err != nil {
		return "", err
	}
	artifact, revision, err := s.stageJSON("editor/revisions/"+ID()+".json", value)
	if err != nil {
		return "", err
	}
	defer os.Remove(artifact.File)
	next := Resource{Key: key, Path: artifact.Path, Revision: revision, PreviousPath: legacyPath, UpdatedAt: s.now()}
	nextFile := Resource{Key: fileKey, Path: file.Path, Revision: fileRevision, PreviousPath: binary.Path, UpdatedAt: s.now()}
	err = s.Media.Publish(ctx, []media.Artifact{file, artifact}, func(tx context.Context) error {
		if err := repo.CAS(tx, fileKey, binary.Revision, nextFile); err != nil {
			return err
		}
		return repo.CAS(tx, key, expected, next)
	})
	if err != nil {
		discardReleasedLocal(context.WithoutCancel(ctx), s.Media, file.Path, artifact.Path)
		return "", err
	}
	discardReleasedLocal(ctx, s.Media, prior.Path, binary.Path, legacyPath)
	return revision, nil
}

// AdoptBinaryRoots is the verified adapter for persisted metadata produced
// before binary heads existed. It never classifies every editor blob as live:
// only an authoritative ready export/source record can introduce a root, after
// its exact bytes, digest and size have been checked. Existing inventories stay
// fail-closed for unknown or corrupt files.
func (s *Service) AdoptBinaryRoots(ctx context.Context) error {
	repo := s.repository()
	if repo == nil {
		return errors.New("editor repository is required")
	}
	rows, err := repo.List(ctx, "media/")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if strings.Count(row.Key, "/") != 1 {
			continue
		}
		id := strings.TrimPrefix(row.Key, "media/")
		record, _, err := s.readMedia(ctx, id)
		if err != nil {
			return err
		}
		path, err := s.mediaSourcePath(ctx, id, record)
		if err != nil {
			return err
		}
		if err = s.adoptBinary(ctx, row, "media/"+id+"/source", path, record.ContentHash, record.Size); err != nil {
			return err
		}
	}
	exports, err := repo.List(ctx, "exports/")
	if err != nil {
		return err
	}
	for _, row := range exports {
		parts := strings.Split(row.Key, "/")
		if len(parts) != 3 {
			continue
		}
		meta, _, err := s.exportMeta(ctx, parts[1], parts[2])
		if err != nil {
			return err
		}
		if meta["status"] != "ready" {
			continue
		}
		path := exportString(meta["filePath"])
		if path == "" {
			path = "editor/projects/" + parts[1] + "/exports/" + parts[2] + "/" + SanitizeFileName(exportString(meta["fileName"]))
		}
		digest := "sha256:" + exportString(meta["stagedSha256"])
		if err = s.adoptBinary(ctx, row, row.Key+"/file", path, digest, int64(exportNumber(meta["size"]))); err != nil {
			return err
		}
		// A prior replacement left by the pre-adapter producer is garbage only
		// after the current authoritative ready file has been verified and rooted.
		if err = s.reclaimSupersededExportFiles(ctx, row, parts[1], parts[2], path, exportString(meta["fileName"])); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) adoptBinary(ctx context.Context, head Resource, key, path, digest string, size int64) error {
	if _, err := media.ContentKey(strings.TrimPrefix(digest, "sha256:")); err != nil || !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("editor binary %s has invalid content digest", path)
	}
	resolved, err := s.resolvePath(ctx, path)
	if err != nil {
		return err
	}
	path = resolved
	existing, err := s.repository().Get(ctx, key)
	if err == nil {
		if existing.Path != path || existing.Revision != digest {
			return fmt.Errorf("editor binary head disagrees with metadata: %s", key)
		}
		return nil
	}
	if !isMissing(err) {
		return err
	}
	handle, err := s.acquire(ctx, path)
	if err != nil {
		return fmt.Errorf("verify editor binary %s: %w", path, err)
	}
	defer handle.Release()
	file, err := os.Open(handle.Path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != size || "sha256:"+hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("editor binary digest/size mismatch: %s", path)
	}
	actual := path
	if handle.Ref.Path != "" {
		actual = handle.Ref.Path
	}
	return s.Media.Publish(ctx, nil, func(tx context.Context) error {
		current, err := s.repository().Get(tx, head.Key)
		if err != nil {
			return err
		}
		if current.Revision != head.Revision || current.Path != head.Path {
			return ErrConflict
		}
		return s.repository().CAS(tx, key, "", Resource{Key: key, Path: actual, Revision: digest, UpdatedAt: s.now()})
	})
}

// Only the generated replacement directory belonging to this ready export is
// eligible. Retained history, unknown names, roots or unverified files are not
// deleted. This is a one-time repair of the old producer's post-commit leftovers.
func (s *Service) reclaimSupersededExportFiles(ctx context.Context, head Resource, pid, eid, current, fileName string) error {
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
	prefix := "editor/exports/" + pid + "/" + eid + "/files/"
	if !strings.HasPrefix(current, prefix) {
		return nil
	}
	root, err := os.OpenRoot(descriptor.Config.LocalRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	dirs, err := editorRootEntries(root, prefix)
	if isMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	roots, err := backend.Repository().Roots(ctx)
	if err != nil {
		return err
	}
	live := map[string]bool{}
	for _, r := range roots {
		live[r.Path] = true
	}
	for _, dir := range dirs {
		if !dir.IsDir() || ValidateID(dir.Name()) != nil {
			continue
		}
		path := prefix + dir.Name() + "/" + SanitizeFileName(fileName)
		if path == current || live[path] {
			continue
		}
		// A still-bound file may belong to an interrupted publisher. It is not a
		// released leftover and must retain the inventory's fail-closed behavior.
		resolver, ok := s.Media.(interface {
			Resolve(context.Context, string) (blobstore.BlobStore, media.Descriptor, media.Ref, func(), error)
		})
		if !ok {
			return nil
		}
		_, _, _, release, err := resolver.Resolve(ctx, path)
		if release != nil {
			release()
		}
		if err == nil {
			continue
		}
		if !isMissing(err) {
			return err
		}
		entries, err := editorRootEntries(root, prefix+dir.Name())
		if err != nil {
			return err
		}
		if len(entries) != 1 || entries[0].Name() != SanitizeFileName(fileName) || !entries[0].Type().IsRegular() {
			continue
		}
		// Check the current metadata again under the publication fence before a
		// cleanup candidate is handed to confined local removal.
		if err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
			now, e := s.repository().Get(tx, head.Key)
			if e != nil {
				return e
			}
			if now.Revision != head.Revision {
				return ErrConflict
			}
			return nil
		}); err != nil {
			return err
		}
		if err = root.Remove(filepath.FromSlash(path)); err != nil && !isMissing(err) {
			return err
		}
	}
	return nil
}

func editorRootEntries(root *os.Root, path string) ([]os.DirEntry, error) {
	file, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ReadDir(-1)
}
