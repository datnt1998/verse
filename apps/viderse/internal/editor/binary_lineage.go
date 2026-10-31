package editor

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// originalUploadLineage prefers explicit replacement metadata. Only old
// records fall back to the private, server-written streaming SHA256 state.
// Invalid explicit lineage must never be overridden by a more convenient proof.
func originalUploadLineage(meta Object) (string, int64, bool) {
	_, hasDigest := meta["originalSha256"]
	_, hasSize := meta["originalSize"]
	if hasDigest || hasSize {
		digest := exportString(meta["originalSha256"])
		size := exportInt(meta["originalSize"])
		_, err := media.ContentKey(digest)
		return digest, size, err == nil && size > 0 && float64(size) == exportNumber(meta["originalSize"])
	}
	size := exportInt(meta["uploadedBytes"])
	state := exportString(meta["stagedHashState"])
	if size <= 0 || float64(size) != exportNumber(meta["uploadedBytes"]) || len(state) == 0 || len(state) > 512 {
		return "", 0, false
	}
	raw, err := base64.StdEncoding.DecodeString(state)
	if err != nil {
		return "", 0, false
	}
	hash := sha256.New()
	if err = hash.(encoding.BinaryUnmarshaler).UnmarshalBinary(raw); err != nil {
		return "", 0, false
	}
	return hex.EncodeToString(hash.Sum(nil)), size, true
}

func (s *Service) verifyBinaryBytes(ctx context.Context, path, digest string, size int64) error {
	handle, err := s.acquire(ctx, path)
	if err != nil {
		return err
	}
	defer handle.Release()
	file, err := os.Open(handle.Path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, readErr := io.Copy(hash, file)
	if err = errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	if n != size || hex.EncodeToString(hash.Sum(nil)) != digest {
		return fmt.Errorf("editor binary digest/size mismatch: %s", path)
	}
	return nil
}

// A bound file is not garbage by its prefix. It may only retire when this
// ready export proves it was the original upload replaced by its current head.
func (s *Service) retireOriginalUploadBinding(ctx context.Context, head Resource, pid, eid, current string, meta Object) error {
	digest, size, proved := originalUploadLineage(meta)
	if !proved || meta["status"] != "ready" || meta["id"] != eid || exportString(exportObject(meta["clone"])["projectRevision"]) == "" || digest == exportString(meta["stagedSha256"]) {
		return nil
	}
	backend, ok := s.Media.(interface{ Repository() media.Repository })
	if !ok {
		return nil
	}
	prefix := "editor/exports/" + pid + "/" + eid + "/files/"
	paths, err := s.listPaths(ctx, prefix)
	if err != nil {
		return err
	}
	store, err := backend.Repository().State(ctx)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if path == current {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
		if len(parts) != 2 || ValidateID(parts[0]) != nil || parts[1] != SanitizeFileName(exportString(meta["fileName"])) {
			continue
		}
		ref, err := backend.Repository().Resolve(ctx, store.Active, path)
		if isMissing(err) {
			continue
		}
		if err != nil {
			return err
		}
		if ref.SHA256 != digest || ref.Bytes != size {
			continue
		}
		// Both the replacement and original must match committed evidence before
		// the original can lose its binding. These reads release before Unbind.
		if err = s.verifyBinaryBytes(ctx, current, exportString(meta["stagedSha256"]), exportInt(meta["size"])); err != nil {
			return err
		}
		if err = s.verifyBinaryBytes(ctx, path, digest, size); err != nil {
			return err
		}
		retired := false
		err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
			now, err := s.repository().Get(tx, head.Key)
			if err != nil {
				return err
			}
			if now.Revision != head.Revision || now.Path != head.Path {
				return ErrConflict
			}
			currentFile, err := s.repository().Get(tx, head.Key+"/file")
			if err != nil {
				return err
			}
			if currentFile.Path != current || currentFile.Revision != "sha256:"+exportString(meta["stagedSha256"]) {
				return ErrConflict
			}
			roots, err := backend.Repository().Roots(tx)
			if err != nil {
				return err
			}
			for _, root := range roots {
				if root.Path == path {
					return nil
				}
			}
			active, err := backend.Repository().State(tx)
			if err != nil {
				return err
			}
			still, err := backend.Repository().Resolve(tx, active.Active, path)
			if isMissing(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if still.SHA256 != digest || still.Bytes != size {
				return ErrConflict
			}
			if err = backend.Repository().Unbind(tx, active.Active, path); err != nil {
				return err
			}
			retired = true
			return nil
		})
		if err != nil {
			return err
		}
		if retired {
			discardReleasedLocal(ctx, s.Media, path)
		}
	}
	return nil
}
