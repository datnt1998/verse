package images

import (
	"bytes"
	"context"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"io"
	"os"
	"path/filepath"
	"time"
)

func SniffCoverMime(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 12)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	head = head[:n]
	switch {
	case len(head) >= 8 && bytes.Equal(head[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}):
		return "image/png", nil
	case len(head) >= 3 && head[0] == 255 && head[1] == 216 && head[2] == 255:
		return "image/jpeg", nil
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "image/webp", nil
	}
	return "", steps.CloneError(domain.CloneErrInvalidRequest, "Ảnh bìa phải là PNG, JPEG hoặc WebP", "Chọn một tệp ảnh PNG, JPEG hoặc WebP")
}

// UploadCover accepts a media logical path, never a raw client filesystem
// path. Choosing the owner's file is approval of that thumbnail only.
func (h Handler) UploadCover(ctx context.Context, s *pipeline.StepContext, m Manifest, input string) (Manifest, error) {
	acquired, err := s.Acquire(ctx, input)
	if err != nil {
		return m, err
	}
	defer acquired.Release()
	if _, err = SniffCoverMime(acquired.Path); err != nil {
		return m, err
	}
	for index := range m.Items {
		item := &m.Items[index]
		if item.Kind != "thumbnail" {
			continue
		}
		n := 1
		for _, v := range item.Versions {
			n = max(n, v.Version+1)
		}
		hash, err := steps.SHA256File(acquired.Path)
		if err != nil {
			return m, err
		}
		relative := fmt.Sprintf("images/versions/thumbnail/%d-upload-%s.webp", n, hash[:16])
		out := filepath.Join(s.StagingDir, filepath.FromSlash(relative))
		if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return m, err
		}
		encode := h.Deps.Encode
		if encode == nil {
			encode = h.encode
		}
		encoded, err := encode(ctx, s, acquired.Path, out, 1280, 720)
		if err != nil {
			return m, err
		}
		version := Version{Version: n, GenerationID: "upload-" + hash, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), RefHash: item.Reference.SHA256, SourceFingerprint: SourceFingerprint(*item), Path: relative, SHA256: encoded.SHA256, Width: 1280, Height: 720, Bytes: encoded.Bytes, Mime: "image/webp"}
		item.Versions = append(item.Versions, version)
		item.CurrentVersion = &n
		item.ReviewVersion = nil
		item.Status = "approved"
		m.Revision++
		err = s.Publish(ctx, []media.Artifact{{Path: relative, File: out, ContentType: "image/webp"}}, func(txctx context.Context) error { return s.SaveCheckpoint(txctx, m) })
		return m, err
	}
	return m, steps.CloneError(domain.CloneErrInternal, "Manifest ảnh thiếu mục ảnh bìa", "")
}
