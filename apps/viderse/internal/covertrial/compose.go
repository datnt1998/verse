package covertrial

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// source là video đã xong đang được đọc làm nguồn vẽ thử. Chỉ đọc: manifest
// ảnh và artifact của job, không ghi checkpoint/project nào.
type source struct {
	job      *domain.CloneJobView
	manifest images.Manifest
	story    images.Story
}

// loadSource đọc job + images/manifest.json, đòi đúng trạng thái "đã làm xong"
// (không bị thay) và có câu chuyện ảnh của lần phân tích.
func (s *Service) loadSource(ctx context.Context, jobID string) (*source, error) {
	if s.opts.Jobs == nil {
		return nil, trialUnavailable("Chưa đọc được danh sách video")
	}
	if strings.TrimSpace(jobID) == "" {
		return nil, invalid("Chưa chọn video nguồn", "Chọn một video đã làm xong rồi vẽ thử")
	}
	job, err := s.opts.Jobs.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, library.ErrNotFound) {
			return nil, conflict("Video nguồn không còn", "Chọn video khác để vẽ thử")
		}
		return nil, err
	}
	if job == nil || job.Status != domain.CloneSucceeded || job.SupersededAt != nil || job.QueueItem != nil {
		return nil, conflict("Video nguồn chưa làm xong hoặc đã bị thay", "Chọn một video đã làm xong để vẽ thử")
	}
	manifest, err := s.readImagesManifest(ctx, job.Steps)
	if err != nil {
		return nil, conflict("Video nguồn chưa có ảnh đã vẽ", "Chọn video khác hoặc dựng lại ảnh cho video này")
	}
	if manifest.Story == nil {
		return nil, conflict("Video nguồn chưa có câu chuyện ảnh", "Chọn video khác đã qua bước phân tích ảnh")
	}
	return &source{job: job, manifest: manifest, story: *manifest.Story}, nil
}

func (s *Service) readImagesManifest(ctx context.Context, stepsOut []domain.CloneStepRecord) (images.Manifest, error) {
	var out images.Manifest
	path := artifactPath(stepsOut, "images/manifest.json")
	if path == "" {
		return out, errNoImagesArtifact
	}
	handle, err := s.opts.Media.Acquire(ctx, path)
	if err != nil {
		return out, err
	}
	defer handle.Release()
	data, err := os.ReadFile(handle.Path)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

var errNoImagesArtifact = errors.New("thiếu images/manifest.json")

func artifactPath(stepsOut []domain.CloneStepRecord, rel string) string {
	for _, step := range stepsOut {
		if step.Output == nil {
			continue
		}
		for _, artifact := range step.Output.Artifacts {
			if artifact.Path == rel || strings.HasSuffix(artifact.Path, "/"+rel) {
				return artifact.Path
			}
		}
	}
	return ""
}

func sceneDuration(stepsOut []domain.CloneStepRecord) float64 {
	for _, step := range stepsOut {
		if step.Step != domain.CloneScenes || step.Output == nil {
			continue
		}
		if value, ok := step.Output.Data["durationSec"].(float64); ok {
			return value
		}
	}
	return 0
}

// thumbnailItem trả item ảnh bìa của manifest nguồn.
func thumbnailItem(m images.Manifest) (images.Item, bool) {
	for _, item := range m.Items {
		if item.Kind == "thumbnail" {
			return item, true
		}
	}
	return images.Item{}, false
}

// sceneItems trả các cảnh có lời kể, đúng thứ tự trong manifest.
func sceneItems(m images.Manifest) []images.Item {
	out := make([]images.Item, 0, len(m.Items))
	for _, item := range m.Items {
		if item.Kind == "scene" && item.Status != "skipped" {
			out = append(out, item)
		}
	}
	return out
}

// pickScene chọn cảnh theo sceneID (rỗng → cảnh đầu) trong manifest nguồn.
func pickScene(m images.Manifest, sceneID string) (images.Item, error) {
	scenes := sceneItems(m)
	if len(scenes) == 0 {
		return images.Item{}, conflict("Video nguồn không có ảnh cảnh", "Chọn video có ảnh cảnh hoặc dùng chỉ dẫn ảnh bìa")
	}
	if sceneID == "" {
		return scenes[0], nil
	}
	for _, item := range scenes {
		if item.ID == sceneID || item.SceneID == sceneID {
			return item, nil
		}
	}
	return images.Item{}, conflict("Cảnh đã chọn không còn trong video nguồn", "Chọn cảnh khác rồi thử lại")
}

// coverComposition trả bố cục bìa của prompt, mặc định "clean".
func coverComposition(p *domain.ImagePrompt) string {
	if p != nil && strings.TrimSpace(string(p.Composition)) != "" {
		return string(p.Composition)
	}
	return "clean"
}

func requiresHook(composition string) bool {
	return composition == "hook" || composition == "hook-arrow"
}

// validateComposition đòi chữ hook thật khi bố cục cần chữ — không tự bịa chữ.
func validateComposition(story images.Story, composition string) error {
	if requiresHook(composition) && strings.TrimSpace(story.ThumbnailText) == "" {
		return conflict("Video nguồn chưa có chữ hook cho ảnh bìa", "Chọn chỉ dẫn bố cục không cần chữ hook, hoặc chọn video khác")
	}
	return nil
}

// refPlan là một ảnh tham chiếu dự kiến lấy từ artifact nguồn.
type refPlan struct {
	id     string
	role   images.RefRole
	path   string
	sha    string
	source images.RefSource
}

// planRefs chọn ảnh tham chiếu thật từ nguồn: ảnh bìa nguồn giữ bố cục, ảnh
// cảnh đầu giữ nhân vật. Cắt theo trần của provider để đúng số ảnh đã gửi.
func planRefs(src *source, item images.Item, kind string, limit int) []refPlan {
	if limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	plans := make([]refPlan, 0, limit)
	add := func(id string, role images.RefRole, path, sha string, source images.RefSource) {
		if len(plans) >= limit || path == "" || seen[path] {
			return
		}
		seen[path] = true
		plans = append(plans, refPlan{id: id, role: role, path: path, sha: sha, source: source})
	}
	if kind == "cover" {
		// Ảnh bìa nguồn giữ góc máy/bố cục; chữ và logo sẽ được thay theo prompt.
		if thumb, ok := thumbnailItem(src.manifest); ok {
			if v := images.Current(thumb); v != nil {
				add("cover-source", images.RefLayout, v.Path, v.SHA256, images.RefSource{Type: "video_image", Label: "Ảnh bìa nguồn", VideoID: src.job.ID, ImageID: thumb.ID})
			}
		}
	} else if item.Reference.Path != "" {
		add("rival-frame", images.RefLayout, item.Reference.Path, item.Reference.SHA256, images.RefSource{Type: "rival_frame", Label: "Khung đối thủ", VideoID: src.job.ID})
	}
	for _, scene := range sceneItems(src.manifest) {
		if scene.ID == item.ID {
			continue
		}
		if v := images.Current(scene); v != nil {
			add("first-scene", images.RefCharacter, v.Path, v.SHA256, images.RefSource{Type: "video_image", Label: "Nhân vật", VideoID: src.job.ID, ImageID: scene.ID})
			break
		}
	}
	return plans
}

// acquiredRefs giữ handle của các ảnh tham chiếu đã mở (đọc được, đúng sha) và
// trả ImageRef khớp đúng thứ tự đã gửi. release phải gọi sau Draw.
func (s *Service) acquiredRefs(ctx context.Context, plans []refPlan) ([]images.ImageRef, []string, func()) {
	handles := make([]*media.Handle, 0, len(plans))
	refs := make([]images.ImageRef, 0, len(plans))
	files := make([]string, 0, len(plans))
	release := func() {
		for _, handle := range handles {
			handle.Release()
		}
	}
	for _, plan := range plans {
		handle, err := s.opts.Media.Acquire(ctx, plan.path)
		if err != nil {
			continue
		}
		if plan.sha != "" {
			sum, err := steps.SHA256File(handle.Path)
			if err != nil || sum != plan.sha {
				handle.Release()
				continue
			}
		}
		handles = append(handles, handle)
		files = append(files, handle.Path)
		refs = append(refs, images.ImageRef{
			ID:     plan.id,
			Role:   plan.role,
			Path:   plan.path,
			SHA256: plan.sha,
			Source: plan.source,
		})
	}
	return refs, files, release
}

// composeCover viết prompt ảnh bìa thử theo prompt đang chọn. Trả prompt đã
// ghép, kích thước canvas và danh sách tệp tham chiếu kèm handle.
func (s *Service) composeCover(ctx context.Context, src *source, prompt *domain.ImagePrompt, provider string, refLimit int) (string, int, int, []string, func(), error) {
	item, ok := thumbnailItem(src.manifest)
	if !ok {
		return "", 0, 0, nil, nil, conflict("Video nguồn chưa có ảnh bìa", "Chọn video khác để vẽ thử")
	}
	composition := coverComposition(prompt)
	if err := validateComposition(src.story, composition); err != nil {
		return "", 0, 0, nil, nil, err
	}
	if item.Width <= 0 || item.Height <= 0 {
		item.Width, item.Height = 1280, 720
	}
	plans := planRefs(src, item, "cover", refLimit)
	refs, files, release := s.acquiredRefs(ctx, plans)
	composed, err := buildImagePrompt(src, item, composition, prompt, refs, len(files), provider)
	if err != nil {
		release()
		return "", 0, 0, nil, nil, err
	}
	return composed, item.Width, item.Height, files, release, nil
}

// composeScene viết prompt ảnh cảnh thử theo prompt đang chọn.
func (s *Service) composeScene(ctx context.Context, src *source, prompt *domain.ImagePrompt, sceneID, provider string, refLimit int) (string, int, int, []string, func(), error) {
	item, err := pickScene(src.manifest, sceneID)
	if err != nil {
		return "", 0, 0, nil, nil, err
	}
	if item.Width <= 0 || item.Height <= 0 {
		item.Width, item.Height = 1920, 1080
	}
	plans := planRefs(src, item, "scene", refLimit)
	refs, files, release := s.acquiredRefs(ctx, plans)
	composed, err := buildImagePrompt(src, item, "clean", prompt, refs, len(files), provider)
	if err != nil {
		release()
		return "", 0, 0, nil, nil, err
	}
	return composed, item.Width, item.Height, files, release, nil
}

// buildImagePrompt ghép prompt y như đường ảnh của pipeline (cùng
// images.BuildPrompt + images.JoinPrompt), chỉ đổi phần nguồn prompt.
func buildImagePrompt(src *source, item images.Item, composition string, prompt *domain.ImagePrompt, refs []images.ImageRef, attached int, provider string) (string, error) {
	story := src.story
	if prompt != nil && prompt.Kind == "scene" && !prompt.BuiltIn {
		// Prompt cảnh của chủ kênh thay phần nhìn của kênh (đúng luật pipeline:
		// ImagePromptMode custom → Look rỗng).
		story.Look = ""
	}
	text := ""
	if prompt != nil {
		text = prompt.Prompt
	}
	in := images.PromptInput{
		Story:     story,
		Item:      item,
		Style:     src.manifest.Style,
		Thumbnail: composition,
		Template:  text,
		TemplateSource: images.PromptPartSource{
			Type:  "image_prompt",
			ID:    prompt.ID,
			Label: "Chỉ dẫn đang chọn",
		},
		Refs:         refs,
		AttachedRefs: attached,
		Provider:     provider,
	}
	parts, wrapper, err := images.BuildPrompt(in)
	if err != nil {
		return "", promptBuildError(err)
	}
	return images.JoinPrompt(wrapper, parts), nil
}

func promptBuildError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no shot written"):
		return conflict("Video nguồn chưa có câu chuyện cho ảnh này", "Chọn video khác hoặc chạy lại bước phân tích ảnh")
	case strings.Contains(msg, "no hook text"):
		return conflict("Video nguồn chưa có chữ hook cho ảnh bìa", "Chọn chỉ dẫn bố cục không cần chữ hook, hoặc chọn video khác")
	default:
		return conflict("Chưa ghép được chỉ dẫn vẽ thử", "Chọn video khác rồi thử lại")
	}
}

// validateSourceImage đảm bảo video nguồn còn đọc được manifest và prompt
// dựng được — dùng ở Start để không tạo hàng chờ chết.
func (s *Service) validateDrawable(src *source, kind string, prompt *domain.ImagePrompt, sceneID string, provider string) error {
	item := images.Item{}
	switch kind {
	case "cover":
		var ok bool
		item, ok = thumbnailItem(src.manifest)
		if !ok {
			return conflict("Video nguồn chưa có ảnh bìa", "Chọn video khác để vẽ thử")
		}
		if err := validateComposition(src.story, coverComposition(prompt)); err != nil {
			return err
		}
	default:
		var err error
		item, err = pickScene(src.manifest, sceneID)
		if err != nil {
			return err
		}
		if item.Width <= 0 || item.Height <= 0 {
			item.Width, item.Height = 1920, 1080
		}
	}
	composition := "clean"
	if kind == "cover" {
		composition = coverComposition(prompt)
	}
	_, err := buildImagePrompt(src, item, composition, prompt, nil, 0, provider)
	return err
}

// trialUnavailable dựng lỗi 503 khi phần đọc video nguồn chưa gắn.
func trialUnavailable(msg string) error {
	return &library.Error{Code: library.CodeToolMissing, Message: msg, Recovery: "Khởi động lại ứng dụng rồi thử lại"}
}

// conflict dựng lỗi 409 nghiệp vụ với câu phục hồi thật.
func conflict(msg, recovery string) error {
	return &library.Error{Code: library.CodeConflict, Message: msg, Recovery: recovery}
}

// invalid dựng lỗi 400 nghiệp vụ.
func invalid(msg, recovery string) error {
	return &library.Error{Code: library.CodeInvalidInput, Message: msg, Recovery: recovery}
}

// quotaError dựng lỗi 429 khi hạn mức ảnh đã hết trước khi gửi.
func quotaError(msg, recovery string, retryAt *time.Time) error {
	return &library.Error{Code: library.CodeQuota, Message: msg, Recovery: recovery, RetryAt: retryAt}
}

// drawProvider chọn label cho thông báo lỗi trước khi có biên nhận.
func drawProvider(res DrawerResolution) string {
	if res.Label != "" {
		return res.Label
	}
	return imagegen.ProviderLabel(res.Provider)
}
