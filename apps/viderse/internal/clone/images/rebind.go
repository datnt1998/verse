package images

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// ProjectBinder thay media của một clip trong project đã dựng bằng ảnh của bản
// mới: giữ nguyên chuyển động, keyframe và thời lượng, ghi vào media của
// project, cập nhật manifest bước dựng rồi đo lại Độ hoàn thiện (07 §26).
type ProjectBinder struct {
	Edits   BuildSteps
	Editor  ProjectMedia
	Measure func(context.Context, string) error
	// MeasureGrace: chờ tối đa bao lâu để biết lần đo khởi động được; 0 = 2 giây.
	MeasureGrace time.Duration
}

// BuildSteps là phần kho bước dựng mà việc thay media cần.
type BuildSteps interface {
	Read(context.Context, string, domain.CloneStep) (jobedit.Snapshot, error)
	Acquire(context.Context, jobedit.Snapshot, string) (*media.Handle, error)
	Commit(context.Context, jobedit.Snapshot, any, domain.CloneStep, bool, []media.Artifact) error
	// CommitBuildMedia ghi manifest bước build sau khi thay media của clip —
	// đi qua được job lease còn sống của lượt vẽ lại ảnh (07 §26).
	CommitBuildMedia(context.Context, jobedit.Snapshot, any, []media.Artifact) error
}

// ProjectMedia là phần Editor mà việc thay media cần.
type ProjectMedia interface {
	ImportFileOnce(context.Context, editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error)
	LoadProject(context.Context, string) (editor.Project, string, error)
	RunCommands(context.Context, string, []editor.Command, editor.ApplyCommandsOptions) (editor.Object, error)
}

// Rebind thay media của clip của một cảnh (hoặc ảnh bìa) bằng đúng ảnh của
// phiên bản đã chọn.
func (b *ProjectBinder) Rebind(ctx context.Context, req RebindRequest) (*RebindResult, error) {
	if b == nil || b.Edits == nil || b.Editor == nil {
		return nil, nil
	}
	build, err := b.Edits.Read(ctx, req.JobID, domain.CloneBuild)
	if err != nil {
		if isMissingStep(err) {
			return nil, nil
		}
		return nil, err
	}
	projectID := snapshotProjectID(build)
	if projectID == "" {
		return nil, nil
	}
	imagesSnapshot, err := b.Edits.Read(ctx, req.JobID, domain.CloneImages)
	if err != nil {
		return nil, err
	}
	path, sha, mime := req.Path, req.SHA256, req.Mime
	if req.Cutout != nil && req.Cutout.Path != "" {
		path, sha, mime = req.Cutout.Path, req.Cutout.SHA256, "image/png"
	}
	handle, err := b.Edits.Acquire(ctx, imagesSnapshot, path)
	if err != nil {
		return nil, err
	}
	defer handle.Release()
	// Importing publishes the same digest under an editor alias and needs its
	// exclusive publication lock. Never retain Acquire's shared lease across
	// that write: copy while pinned, then release before importing the copy.
	staged, err := os.CreateTemp("", "viderse-image-rebind-*"+filepath.Ext(path))
	if err != nil {
		return nil, err
	}
	defer os.Remove(staged.Name())
	source, err := os.Open(handle.Path)
	if err != nil {
		_ = staged.Close()
		return nil, err
	}
	_, copyErr := io.Copy(staged, source)
	sourceErr := source.Close()
	stageErr := staged.Close()
	handle.Release()
	if err := errors.Join(copyErr, sourceErr, stageErr); err != nil {
		return nil, err
	}
	record, _, err := b.Editor.ImportFileOnce(ctx, editor.ImportFileOnceOptions{
		ProjectID: projectID, File: staged.Name(), FileName: filepath.Base(path),
		DeclaredMIME: mime, ContentHash: "sha256:" + sha,
	})
	if err != nil {
		return nil, err
	}
	src := "/api/editor/media/" + url.PathEscape(record.ID) + "?v=" + url.QueryEscape(record.ContentHash)
	result := &RebindResult{ProjectID: projectID, MediaID: record.ID, Src: src, Quality: "none"}
	manifestFile := buildManifestFile(build)
	scenes, err := manifestScenes(manifestFile)
	if err != nil {
		return nil, err
	}
	if req.Kind == "scene" {
		clipID := projectClipID(projectID, req.SceneID)
		previous, err := clipMediaID(b.Editor, ctx, projectID, clipID)
		if err != nil {
			return nil, err
		}
		result.ClipID = clipID
		result.PreviousMediaID = previous
		if req.ApplyProject {
			if err := b.applyClip(ctx, projectID, clipID, record.ID, src); err != nil {
				return nil, err
			}
		}
		if err := setSceneMedia(scenes, req.SceneID, record.ID, src); err != nil {
			return nil, err
		}
	} else {
		if err := setThumbnailMedia(scenes, record.ID, sha); err != nil {
			return nil, err
		}
	}
	for attempt := 0; ; attempt++ {
		err := b.commitBuild(ctx, build, scenes)
		if err == nil {
			break
		}
		var conflict *domain.CloneError
		if attempt == 2 || !errors.As(err, &conflict) || conflict.Code != domain.CloneErrConflict {
			return nil, err
		}
		latest, readErr := b.Edits.Read(ctx, req.JobID, domain.CloneBuild)
		if readErr != nil {
			return nil, readErr
		}
		// Only merge independent scene/thumbnail publications. Never turn a
		// generation, worker-fence or same-image conflict into a stale overwrite.
		if latest.Job.Generation != build.Job.Generation || snapshotProjectID(latest) != projectID ||
			latest.Step.Fence != build.Step.Fence || latest.Step.Attempt != build.Step.Attempt ||
			latest.Step.Status != build.Step.Status || !bytes.Equal(latest.Step.Checkpoint, build.Step.Checkpoint) {
			return nil, err
		}
		fresh, readErr := manifestScenes(buildManifestFile(latest))
		if readErr != nil {
			return nil, readErr
		}
		target := func(manifest map[string]any) any {
			if req.Kind != "scene" {
				return manifest["thumbnail"]
			}
			entries, _ := manifest["scenes"].([]any)
			for _, entry := range entries {
				if scene, ok := entry.(map[string]any); ok && text(scene["sceneId"]) == req.SceneID {
					return scene
				}
			}
			return nil
		}
		if !reflect.DeepEqual(target(buildManifestFile(build)), target(fresh)) {
			return nil, err
		}
		if req.Kind == "scene" {
			readErr = setSceneMedia(fresh, req.SceneID, record.ID, src)
		} else {
			readErr = setThumbnailMedia(fresh, record.ID, sha)
		}
		if readErr != nil {
			return nil, readErr
		}
		// Import and the project command already succeeded; retry only the
		// manifest publication, preserving the other scene's accepted binding.
		build, scenes = latest, fresh
	}
	if b.Measure != nil {
		result.Quality = b.startMeasure(ctx, req.JobID)
	}
	return result, nil
}

// measureDeadline là trần của một lần đo chạy nền; Supervisor còn có hạn riêng.
const measureDeadline = 10 * time.Minute

// defaultMeasureGrace là khoảng chờ tối đa để biết lần đo có khởi động được không.
const defaultMeasureGrace = 2 * time.Second

// startMeasure chạy đo Độ hoàn thiện ngoài request: media đã thay và manifest đã
// ghi nên kết quả thay luôn được trả; đo lỗi sớm (thiếu Chrome…) chỉ đổi Quality
// thành "unavailable", còn đo chạy lâu thì để nền với ngữ cảnh riêng. Kết quả đo
// cũ bị bỏ khi project đã đổi (render.Service kiểm binding lúc hoàn tất).
func (b *ProjectBinder) startMeasure(ctx context.Context, jobID string) string {
	measureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), measureDeadline)
	done := make(chan error, 1)
	go func() {
		defer cancel()
		done <- b.Measure(measureCtx, jobID)
	}()
	grace := b.MeasureGrace
	if grace <= 0 {
		grace = defaultMeasureGrace
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			return "unavailable"
		}
	case <-timer.C:
	case <-ctx.Done():
	}
	return "measuring"
}

// applyClip thay media của đúng một clip bằng một lệnh của Editor, giữ nguyên
// chuyển động, keyframe, transform và thời lượng của clip.
func (b *ProjectBinder) applyClip(ctx context.Context, projectID, clipID, mediaID, src string) error {
	commands := []editor.Command{{Type: "item.update", Params: editor.Object{"id": clipID, "updates": editor.Object{"mediaId": mediaID, "src": src}}}}
	_, err := b.Editor.RunCommands(ctx, projectID, commands, editor.ApplyCommandsOptions{})
	return err
}

// clipMediaID đọc media đang gán cho một clip.
func clipMediaID(service ProjectMedia, ctx context.Context, projectID, clipID string) (string, error) {
	project, _, err := service.LoadProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	for _, item := range projectItems(project) {
		if text(item["id"]) == clipID {
			return text(item["mediaId"]), nil
		}
	}
	return "", nil
}

// projectItems trả các item trong timeline của project.
func projectItems(project editor.Project) []editor.Object {
	timeline, _ := project["timeline"].(map[string]any)
	raw, _ := timeline["items"].([]any)
	out := make([]editor.Object, 0, len(raw))
	for _, entry := range raw {
		if object, ok := entry.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

// commitBuild ghi manifest bước dựng đã cập nhật media của cảnh/bìa.
func (b *ProjectBinder) commitBuild(ctx context.Context, build jobedit.Snapshot, manifest map[string]any) error {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	file, err := writeTemp(raw)
	if err != nil {
		return err
	}
	defer os.Remove(file)
	checkpoint := build.Step.Checkpoint
	return b.Edits.CommitBuildMedia(ctx, build, json.RawMessage(checkpoint), []media.Artifact{{Path: "build/manifest.json", File: file, ContentType: "application/json"}})
}

// snapshotProjectID đọc id project editor từ đầu ra bước dựng.
func snapshotProjectID(snapshot jobedit.Snapshot) string {
	if snapshot.Step.Output == nil {
		return ""
	}
	if value, ok := snapshot.Step.Output.Data["projectId"].(string); ok {
		return value
	}
	return ""
}

// buildManifestFile đọc nội dung manifest bước dựng.
func buildManifestFile(snapshot jobedit.Snapshot) map[string]any {
	var manifest map[string]any
	if err := json.Unmarshal(snapshot.Manifest, &manifest); err != nil {
		return map[string]any{}
	}
	return manifest
}

// manifestScenes trả danh sách cảnh của manifest bước dựng.
func manifestScenes(manifest map[string]any) (map[string]any, error) {
	if _, ok := manifest["scenes"]; !ok {
		return nil, fmt.Errorf("build manifest has no scenes")
	}
	return manifest, nil
}

// setSceneMedia gán media của một cảnh trong manifest bước dựng.
func setSceneMedia(manifest map[string]any, sceneID, mediaID, src string) error {
	raw, _ := manifest["scenes"].([]any)
	for _, entry := range raw {
		scene, ok := entry.(map[string]any)
		if !ok || text(scene["sceneId"]) != sceneID {
			continue
		}
		scene["mediaId"] = mediaID
		scene["src"] = src
		return nil
	}
	return fmt.Errorf("scene %s is missing from the build manifest", sceneID)
}

// setThumbnailMedia gán ảnh bìa trong manifest bước dựng.
func setThumbnailMedia(manifest map[string]any, mediaID, sha string) error {
	thumbnail, ok := manifest["thumbnail"].(map[string]any)
	if !ok {
		return fmt.Errorf("build manifest has no thumbnail")
	}
	thumbnail["mediaId"] = mediaID
	thumbnail["sha256"] = sha
	return nil
}

func text(value any) string {
	value2, _ := value.(string)
	return strings.TrimSpace(value2)
}

// writeTemp ghi nội dung manifest đã cập nhật ra tệp tạm để công bố.
func writeTemp(raw []byte) (string, error) {
	file, err := os.CreateTemp("", "viderse-build-manifest-*.json")
	if err != nil {
		return "", err
	}
	if _, err = file.Write(raw); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	if err = file.Close(); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func isMissingStep(err error) bool {
	var cloneErr *domain.CloneError
	return errors.As(err, &cloneErr) && cloneErr.Code == domain.CloneErrNotFound
}
