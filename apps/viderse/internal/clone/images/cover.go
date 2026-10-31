package images

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

const MaxCoverBytes int64 = 20 << 20

// ValidateCover decodes the actual image, not an extension or claimed MIME.
// Bound dimensions before decoding to prevent tiny compressed decompression bombs.
func ValidateCover(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", cloneError(domain.CloneErrNotFound, "Không đọc được tệp ảnh bìa")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxCoverBytes {
		return "", cloneError(domain.CloneErrInvalidRequest, "Ảnh bìa phải là tệp ảnh không quá 20 MiB")
	}
	config, format, err := image.DecodeConfig(io.LimitReader(f, MaxCoverBytes+1))
	if err != nil || (format != "png" && format != "jpeg" && format != "webp") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return "", cloneError(domain.CloneErrInvalidRequest, "Ảnh bìa phải là PNG, JPEG hoặc WebP đọc được")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	decoded, actual, err := image.Decode(io.LimitReader(f, MaxCoverBytes+1))
	if err != nil || actual != format || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return "", cloneError(domain.CloneErrInvalidRequest, "Ảnh bìa không đọc được")
	}
	return "image/" + format, nil
}

func (s *Service) UploadCover(ctx context.Context, id, file string, input RevisionInput) (CoverResult, error) {
	l, err := s.load(ctx, id, input.ExpectedRevision, true)
	if err != nil {
		return CoverResult{}, err
	}
	if err = s.mutable(l); err != nil {
		return CoverResult{}, err
	}
	wasApproved := model.ImagesApproved(l.manifest)
	if _, err = ValidateCover(file); err != nil {
		return CoverResult{}, err
	}
	index := -1
	for n, i := range l.manifest.Items {
		if i.Kind == "thumbnail" {
			index = n
			break
		}
	}
	if index < 0 {
		return CoverResult{}, cloneError(domain.CloneErrInternal, "Manifest ảnh thiếu mục ảnh bìa")
	}
	item := &l.manifest.Items[index]
	if item.Status == "running" || item.InFlight != nil {
		return CoverResult{}, cloneError(domain.CloneErrConflict, "Ảnh bìa đang tạo hoặc cần phục hồi")
	}
	if s.Ffmpeg == nil || s.Runner == nil {
		return CoverResult{}, cloneError(domain.CloneErrToolMissing, "Cần FFmpeg có libwebp")
	}
	ff := s.Ffmpeg.Resolve(ctx)
	if ff == nil || ff.Ffmpeg == "" || ff.Ffprobe == "" || ff.Probe == nil || !ff.Probe.Caps.Encoders["libwebp"] {
		return CoverResult{}, cloneError(domain.CloneErrToolMissing, "Cần FFmpeg có libwebp")
	}
	dir, err := os.MkdirTemp("", "viderse-cover-")
	if err != nil {
		return CoverResult{}, err
	}
	defer os.RemoveAll(dir)
	// Reuse the real pipeline encoder and its output codec/dimension checks.
	step := &pipeline.StepContext{StagingDir: dir, ProcessRunner: s.Runner.Run}
	handler := model.Handler{Deps: model.Deps{Ffmpeg: ff.Ffmpeg, FFprobe: ff.Ffprobe}}
	output := filepath.Join(dir, "cover.webp")
	encoded, err := handler.EncodeCover(ctx, step, file, output)
	if err != nil {
		return CoverResult{}, cloneError(domain.CloneErrInvalidRequest, fmt.Sprintf("Ảnh bìa không đọc được: %.220s", err.Error()))
	}
	number := 1
	for _, v := range item.Versions {
		number = max(number, v.Version+1)
	}
	path := fmt.Sprintf("images/versions/thumbnail/%d-upload-%s.webp", number, encoded.SHA256[:16])
	item.Versions = append(item.Versions, model.Version{Version: number, GenerationID: "upload-" + encoded.SHA256, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), PromptVersion: item.PromptVersion, RefHash: item.Reference.SHA256, SourceFingerprint: model.SourceFingerprint(*item), Path: path, SHA256: encoded.SHA256, Width: 1280, Height: 720, Bytes: encoded.Bytes, Mime: "image/webp", Source: "upload"})
	item.CurrentVersion = &number
	item.ReviewVersion = nil
	item.Status = "approved"
	item.SkipReason = ""
	l.manifest.Revision++
	if !hasPending(l.manifest) {
		l.manifest.Waiting = nil
	}
	// Existing rendered frames stay intact; the first completed review releases build.
	from := domain.CloneStep("")
	if firstReviewCompleted(l.snapshot, wasApproved, l.manifest) {
		from = domain.CloneBuild
	}
	err = s.Edits.Commit(ctx, l.snapshot, l.manifest, from, false, []media.Artifact{{Path: path, File: output, ContentType: "image/webp"}})
	if err != nil {
		return CoverResult{}, err
	}
	return CoverResult{JobID: id, Revision: l.manifest.Revision, Version: number}, nil
}
