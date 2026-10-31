// Package bundle streams finished clone deliverables from their committed
// records. Clients supply job IDs only, never storage paths.
package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const MaxJobs = 50
const partMax = 116

type Exports interface {
	AcquireExport(context.Context, string, string) (*media.Handle, editor.ExportRecord, error)
}
type Ffmpeg interface {
	Resolve(context.Context) *tools.FfmpegResolved
}
type Service struct {
	Edits     *jobedit.Service
	Exports   Exports
	Ffmpeg    Ffmpeg
	Runner    tools.Runner
	OwnerName func(context.Context, string) (string, error)
	Now       func() time.Time
}
type entry struct {
	Name string
	Open func(context.Context) (io.ReadCloser, error)
}
type Prepared struct {
	FileName string
	entries  []entry
}

func invalid(message string) error {
	return &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: message}
}
func missing(message string) error {
	return &domain.CloneError{Code: domain.CloneErrNotFound, Message: message}
}

func SanitizePart(raw, fallback string) string {
	var out strings.Builder
	dot := false
	for _, r := range raw {
		if strings.ContainsRune("\\/:*?\"<>|", r) || unicode.IsControl(r) {
			r = ' '
		}
		if r == '.' {
			if dot {
				continue
			}
			dot = true
		} else {
			dot = false
		}
		out.WriteRune(r)
	}
	clean := strings.Join(strings.Fields(out.String()), " ")
	runes := []rune(clean)
	if len(runes) > partMax {
		clean = strings.TrimSpace(string(runes[:partMax]))
	}
	if clean == "" || clean == "." {
		return fallback
	}
	return clean
}
func UploadInfo(title, description string, tags []string) string {
	text := "TIÊU ĐỀ\n" + title + "\n\nMÔ TẢ\n" + description
	if len(tags) > 0 {
		text += "\n\nTAGS\n" + strings.Join(tags, ", ")
	}
	return text + "\n"
}

type leasedFile struct {
	*os.File
	handle *media.Handle
}

func (f *leasedFile) Close() error { err := f.File.Close(); f.handle.Release(); return err }
func openHandle(handle *media.Handle) (io.ReadCloser, error) {
	file, err := os.Open(handle.Path)
	if err != nil {
		handle.Release()
		return nil, err
	}
	return &leasedFile{file, handle}, nil
}
func (s *Service) artifactEntry(ctx context.Context, snapshot jobedit.Snapshot, path, name string) (entry, error) {
	handle, err := s.Edits.Acquire(ctx, snapshot, path)
	if err != nil {
		return entry{}, err
	}
	stat, err := os.Stat(handle.Path)
	handle.Release()
	if err != nil {
		return entry{}, err
	}
	if !stat.Mode().IsRegular() {
		return entry{}, invalid("Tệp video không hợp lệ")
	}
	return entry{Name: name, Open: func(ctx context.Context) (io.ReadCloser, error) {
		handle, err := s.Edits.Acquire(ctx, snapshot, path)
		if err != nil {
			return nil, err
		}
		return openHandle(handle)
	}}, nil
}
func bytesEntry(name string, value []byte) entry {
	return entry{Name: name, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(value)), nil }}
}
func stepOf(snapshot jobedit.Snapshot, name domain.CloneStep) domain.CloneStepRecord {
	for _, step := range snapshot.Steps {
		if step.Step == name {
			return step
		}
	}
	return domain.CloneStepRecord{}
}
func dataString(step domain.CloneStepRecord, key string) string {
	if step.Output == nil {
		return ""
	}
	value, _ := step.Output.Data[key].(string)
	return value
}
func ExportIDs(snapshot jobedit.Snapshot) (string, string, error) {
	export := stepOf(snapshot, domain.CloneExport)
	if dataString(export, "removedAt") != "" {
		return "", "", invalid("Video đã xoá file xuất — xuất lại hoặc bỏ chọn video này")
	}
	if snapshot.Job.Status != domain.CloneSucceeded || export.Status != domain.CloneSucceeded {
		return "", "", invalid("Chỉ tải được video đã xuất xong — bỏ chọn video chưa xong")
	}
	projectID := dataString(export, "projectId")
	if projectID == "" {
		projectID = dataString(stepOf(snapshot, domain.CloneBuild), "projectId")
	}
	exportID := dataString(export, "exportId")
	if projectID == "" || exportID == "" {
		return "", "", invalid("Video chưa có bản xuất đã công bố")
	}
	return projectID, exportID, nil
}
func (s *Service) metadata(ctx context.Context, snapshot jobedit.Snapshot) (*metadata.Output, error) {
	snapshot.Step = stepOf(snapshot, domain.CloneMetadata)
	if !jobedit.Published(snapshot, "metadata/metadata.json") {
		return nil, nil
	}
	handle, err := s.Edits.Acquire(ctx, snapshot, "metadata/metadata.json")
	if err != nil {
		return nil, err
	}
	defer handle.Release()
	file, err := os.Open(handle.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var value metadata.Output
	if err = json.NewDecoder(file).Decode(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
func (s *Service) imageManifest(ctx context.Context, id string) (jobedit.Snapshot, *images.Manifest, error) {
	snapshot, err := s.Edits.Read(ctx, id, domain.CloneImages)
	if err != nil {
		return snapshot, nil, err
	}
	if len(snapshot.Manifest) == 0 {
		return snapshot, nil, nil
	}
	var manifest images.Manifest
	if err = json.Unmarshal(snapshot.Manifest, &manifest); err != nil {
		return snapshot, nil, err
	}
	return snapshot, &manifest, nil
}
func (s *Service) jpg(ctx context.Context, snapshot jobedit.Snapshot, path string) ([]byte, error) {
	if s.Ffmpeg == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Chưa có công cụ đổi ảnh bìa sang JPG"}
	}
	resolved := s.Ffmpeg.Resolve(ctx)
	if resolved == nil || resolved.Ffmpeg == "" {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Chưa có công cụ đổi ảnh bìa sang JPG"}
	}
	handle, err := s.Edits.Acquire(ctx, snapshot, path)
	if err != nil {
		return nil, err
	}
	defer handle.Release()
	work, err := os.MkdirTemp("", "viderse-thumbnail-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	file := filepath.Join(work, "thumbnail.jpg")
	runner := s.Runner
	if runner == nil {
		runner = &tools.ExecRunner{}
	}
	_, err = runner.Run(ctx, resolved.Ffmpeg, []string{"-hide_banner", "-loglevel", "error", "-y", "-i", handle.Path, "-map_metadata", "-1", "-frames:v", "1", "-q:v", "2", file}, tools.Spec{Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	return os.ReadFile(file)
}
func (s *Service) Thumbnail(ctx context.Context, id string) (string, []byte, error) {
	snapshot, err := s.Edits.Store.Read(ctx, id, domain.CloneImages)
	if err != nil {
		return "", nil, err
	}
	title := snapshot.Job.Source.Title
	info, err := s.metadata(ctx, snapshot)
	if err != nil {
		return "", nil, err
	}
	if info != nil && info.Title != "" {
		title = info.Title
	}
	snapshot, manifest, err := s.imageManifest(ctx, id)
	if err != nil {
		return "", nil, err
	}
	if manifest != nil {
		for _, item := range manifest.Items {
			if item.Kind == "thumbnail" {
				if current := images.Current(item); current != nil {
					data, err := s.jpg(ctx, snapshot, current.Path)
					return SanitizePart(title, "video") + ".jpg", data, err
				}
			}
		}
	}
	return "", nil, missing("Video này chưa có ảnh bìa")
}
func (s *Service) Prepare(ctx context.Context, ids []string) (*Prepared, error) {
	if len(ids) == 0 || len(ids) > MaxJobs {
		return nil, invalid("Chọn từ 1 đến 50 video để tải")
	}
	if s.Edits == nil || s.Edits.Store == nil {
		return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Kho video chưa sẵn sàng"}
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	prepared := &Prepared{FileName: "video-clone-" + now.UTC().Format("2006-01-02") + ".zip"}
	folders := map[string]bool{}
	for _, id := range ids {
		if id == "" || len(id) > 64 || strings.ContainsAny(id, "/\\% \r\n\t") {
			return nil, invalid("Mã video không hợp lệ")
		}
		snapshot, err := s.Edits.Store.Read(ctx, id, domain.CloneExport)
		if err != nil {
			return nil, err
		}
		projectID, exportID, err := ExportIDs(snapshot)
		if err != nil {
			return nil, err
		}
		if s.Exports == nil {
			return nil, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "Kho bản xuất chưa sẵn sàng"}
		}
		handle, _, err := s.Exports.AcquireExport(ctx, projectID, exportID)
		if err != nil {
			return nil, err
		}
		handle.Release()
		info, err := s.metadata(ctx, snapshot)
		if err != nil {
			return nil, err
		}
		title := snapshot.Job.Source.Title
		if info != nil && info.Title != "" {
			title = info.Title
		}
		title = SanitizePart(title, "video")
		owner := "Chưa gán kênh"
		if s.OwnerName != nil {
			owner, err = s.OwnerName(ctx, snapshot.Job.ChannelID)
			if err != nil {
				return nil, err
			}
		}
		finished := snapshot.Job.CreatedAt
		step := stepOf(snapshot, domain.CloneExport)
		if step.FinishedAt != nil {
			finished = *step.FinishedAt
		}
		base := SanitizePart(finished.Format("2006-01-02")+" "+title, "video")
		folder := SanitizePart(owner, "Kenh") + "/" + base
		for suffix := 2; folders[folder]; suffix++ {
			folder = SanitizePart(owner, "Kenh") + "/" + base + fmt.Sprintf(" (%d)", suffix)
		}
		folders[folder] = true
		prepared.entries = append(prepared.entries, entry{Name: folder + "/" + title + ".mp4", Open: func(ctx context.Context) (io.ReadCloser, error) {
			handle, _, err := s.Exports.AcquireExport(ctx, projectID, exportID)
			if err != nil {
				return nil, err
			}
			return openHandle(handle)
		}})
		imageSnapshot, manifest, err := s.imageManifest(ctx, id)
		if err != nil {
			return nil, err
		}
		if manifest != nil {
			scenes := make([]images.Item, 0, len(manifest.Items))
			for _, item := range manifest.Items {
				if item.Kind == "thumbnail" {
					if current := images.Current(item); current != nil {
						jpg, err := s.jpg(ctx, imageSnapshot, current.Path)
						if err != nil {
							return nil, err
						}
						prepared.entries = append(prepared.entries, bytesEntry(folder+"/thumbnail.jpg", jpg))
					}
				}
				if item.Kind == "scene" {
					scenes = append(scenes, item)
				}
			}
			sort.SliceStable(scenes, func(i, j int) bool { return scenes[i].StartFrame < scenes[j].StartFrame })
			n := 0
			for _, item := range scenes {
				if current := images.Current(item); current != nil {
					n++
					entry, err := s.artifactEntry(ctx, imageSnapshot, current.Path, fmt.Sprintf("%s/anh/canh-%02d.webp", folder, n))
					if err != nil {
						return nil, err
					}
					prepared.entries = append(prepared.entries, entry)
				}
			}
		}
		if info != nil {
			prepared.entries = append(prepared.entries, bytesEntry(folder+"/thong-tin.txt", []byte(UploadInfo(info.Title, info.Description, info.Tags))))
		}
	}
	return prepared, nil
}
func (p *Prepared) Write(ctx context.Context, out io.Writer) error {
	archive := zip.NewWriter(out)
	for _, entry := range p.entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		input, err := entry.Open(ctx)
		if err != nil {
			return err
		}
		target, err := archive.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Store})
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(target, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return archive.Close()
}
