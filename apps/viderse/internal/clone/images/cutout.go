package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// Cutout publishes a full-size foreground mask for one version. The checkpoint
// CAS prevents a concurrent edit from attaching it to a different image.
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
	if v.Cutout != nil && v.Cutout.Path != "" {
		return v.Cutout.Path, nil
	}
	if runtime.GOOS != "darwin" || s.Runner == nil {
		return "", cloneError(domain.CloneErrToolMissing, "Tách nền chưa khả dụng trên máy này")
	}
	if err := s.mutable(l); err != nil {
		return "", err
	}
	source, err := s.Edits.Acquire(ctx, l.snapshot, v.Path)
	if err != nil {
		return "", err
	}
	defer source.Release()
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
	data, err := os.ReadFile(output)
	if err != nil {
		return "", cloneError(domain.CloneErrToolMissing, "Không thể tách nền ảnh này")
	}
	f, err := os.Open(output)
	if err != nil {
		return "", err
	}
	cfg, decodeErr := png.DecodeConfig(f)
	f.Close()
	if decodeErr != nil || cfg.Width != v.Width || cfg.Height != v.Height {
		return "", cloneError(domain.CloneErrToolMissing, "Không thể tách nền ảnh này")
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	path := "images/cutouts/" + hash + ".png"
	v.Cutout = &scenes.FileRef{Path: path, SHA256: hash}
	l.manifest.Revision++
	err = s.Edits.Commit(ctx, l.snapshot, l.manifest, "", false, []media.Artifact{{Path: path, File: output, ContentType: "image/png", Filename: "subject.png"}})
	if err != nil {
		return "", err
	}
	return path, nil
}
