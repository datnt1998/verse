package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/core/llm"
	"github.com/datnt1998/verse/core/llmcontract"
	"github.com/datnt1998/verse/core/llmretry"
)

type Drawer interface {
	Draw(context.Context, imagegen.Request) (imagegen.Receipt, error)
}
type Encoded struct {
	Width, Height int
	SHA256        string
	Bytes         int64
}
type Deps struct {
	Drawer          Drawer
	Model           llmretry.Generator
	Ffmpeg, FFprobe string
	Look            func(context.Context, domain.CloneJob) (string, error)
	Analyse         func(context.Context, string, []string, []string, string, string, string) (Story, error)
	Encode          func(context.Context, *pipeline.StepContext, string, string, int, int) (Encoded, error)
	Cutout          func(context.Context, *pipeline.StepContext, string, string) error
	Now             func() time.Time
	ReadManifest    func(context.Context, domain.CloneJob) (*Manifest, error)
}
type Handler struct{ Deps Deps }

var _ pipeline.StepHandler = Handler{}

func (h Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	sceneHandle, err := s.Acquire(ctx, "scenes/manifest.json")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer sceneHandle.Release()
	doc, err := steps.DecodeJSONFile[scenes.Manifest](sceneHandle.Path, "Invalid scene manifest")
	sceneHandle.Release()
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if err = doc.Validate(); err != nil {
		return pipeline.StepResult{}, err
	}
	var thumb scenes.FileRef
	if upstream, ok := s.Upstream[domain.CloneDownload]; ok {
		if value, ok := upstream.Data["thumbnail"]; ok {
			raw, _ := json.Marshal(value)
			_ = json.Unmarshal(raw, &thumb)
		}
	}
	if thumb.Path == "" {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrInternal, "Thiếu ảnh bìa tham chiếu", "Chạy lại bước download")
	}
	var previous *Manifest
	if len(s.Checkpoint) == 0 && h.Deps.ReadManifest != nil {
		previous, err = h.Deps.ReadManifest(ctx, s.Job)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		if previous != nil && previous.Overrides != nil && previous.Overrides.NoImages != nil {
			doc.Recipe.ImagesMode = "auto"
			if *previous.Overrides.NoImages {
				doc.Recipe.ImagesMode = "none"
			}
		}
	}
	if len(s.Checkpoint) > 0 {
		var saved Manifest
		if err = json.Unmarshal(s.Checkpoint, &saved); err != nil {
			return pipeline.StepResult{}, err
		}
		if saved.Overrides != nil && saved.Overrides.NoImages != nil {
			doc.Recipe.ImagesMode = "auto"
			if *saved.Overrides.NoImages {
				doc.Recipe.ImagesMode = "none"
			}
		}
		previous = &saved
	}
	fresh := CreateManifest(doc, thumb, s.Job.Source.Title)
	fresh.SceneGeneration = s.Job.Generation
	m := fresh
	if previous != nil {
		m = *previous
		if m.SceneGeneration != 0 && m.SceneGeneration != s.Job.Generation {
			return pipeline.StepResult{}, steps.CloneError(domain.CloneErrStale, "Manifest ảnh thuộc generation cũ", "Chạy lại bước images")
		}
		if m.SceneRevision != doc.Revision {
			m = Reconcile(m, fresh)
		}
	}
	if _, err = drainIntents(ctx, s, &m); err != nil {
		return pipeline.StepResult{}, err
	}
	if !generationPending(m) {
		m.QueueOnly = nil
		if len(s.Checkpoint) == 0 && ImagesApproved(m) {
			if err := s.SaveCheckpoint(ctx, m); err != nil {
				return pipeline.StepResult{}, err
			}
		}
		file := filepath.Join(s.StagingDir, "images", "manifest.json")
		if err = steps.WriteJSONAtomic(file, m); err != nil {
			return pipeline.StepResult{}, err
		}
		return pipeline.StepResult{Artifacts: []media.Artifact{{Path: "images/manifest.json", File: file, ContentType: "application/json"}}, Data: map[string]any{"images": len(m.Items), "manifest": "images/manifest.json"}}, nil
	}
	if h.Deps.Drawer == nil {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrToolMissing, "Thiếu nguồn vẽ ảnh", "Kiểm tra Kết nối AI trong Cài đặt")
	}
	// Stage acquired references one at a time: retaining a DB read lease for
	// every image would exhaust a small connection pool before paid work starts.
	refs := make([]string, 0, len(m.Items))
	ids := make([]string, 0, len(m.Items))
	for _, i := range m.Items {
		ref, err := s.Acquire(ctx, i.Reference.Path)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		hash, err := steps.SHA256File(ref.Path)
		if err != nil {
			ref.Release()
			return pipeline.StepResult{}, err
		}
		if hash != i.Reference.SHA256 {
			ref.Release()
			return pipeline.StepResult{}, steps.CloneError(domain.CloneErrStale, "Ảnh tham chiếu đã thay đổi", "Phân tích lại cảnh")
		}
		staged := filepath.Join(s.StagingDir, "references", i.ID+filepath.Ext(ref.Path))
		err = steps.CopyFile(ref.Path, staged)
		ref.Release()
		if err != nil {
			return pipeline.StepResult{}, err
		}
		refs = append(refs, staged)
		ids = append(ids, i.ID)
	}
	look := ""
	if h.Deps.Look != nil {
		look, err = h.Deps.Look(ctx, s.Job)
		if err != nil {
			return pipeline.StepResult{}, err
		}
	}
	fingerprint := steps.InputFingerprint(map[string]any{"version": PromptVersion, "title": s.Job.Source.Title, "items": fresh.Items, "look": look, "style": m.Style, "thumbnail": m.Thumbnail, "templatePrompt": m.TemplatePrompt, "coverPrompt": m.CoverPrompt, "imagePromptMode": m.ImagePromptMode})
	if m.Story == nil || m.Story.Fingerprint != fingerprint {
		prompt := BuildManifestStoryPrompt(s.Job.Source.Title, m, look)
		analyse := h.Deps.Analyse
		if analyse == nil {
			analyse = h.analyse
		}
		story, err := analyse(ctx, prompt, refs, ids, fingerprint, look, m.Thumbnail)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		m.Story = &story
		if err = s.SaveCheckpoint(ctx, m); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	for {
		if err = h.runQueue(ctx, s, &m); err != nil {
			break
		}
		var drained bool
		drained, err = drainIntents(ctx, s, &m)
		if err != nil || !drained {
			break
		}
	}
	if err == nil {
		m.QueueOnly = nil
		m.Waiting = nil
	}
	manifestFile := filepath.Join(s.StagingDir, "images", "manifest.json")
	if writeErr := steps.WriteJSONAtomic(manifestFile, m); writeErr != nil {
		return pipeline.StepResult{}, writeErr
	}
	result := pipeline.StepResult{Artifacts: []media.Artifact{{Path: "images/manifest.json", File: manifestFile, ContentType: "application/json"}}, Data: map[string]any{"images": len(m.Items), "manifest": "images/manifest.json"}}
	if err != nil {
		var capErr *imagegen.CapError
		var drawErr *imagegen.DrawError
		if errors.As(err, &capErr) {
			drawErr = capErr.DrawError()
		} else {
			_ = errors.As(err, &drawErr)
		}
		if drawErr != nil && drawErr.Code == imagegen.ErrCodeQuota {
			retry := drawErr.RetryAt
			if retry == nil {
				now := time.Now()
				next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
				retry = &next
			}
			e := steps.CloneError(domain.CloneErrQuota, drawErr.Message, drawErr.Recovery)
			e.RetryAt = retry
			e.RetryAtSource = domain.RetryAtWorker
			if drawErr.RetryAtSource == "provider" {
				e.RetryAtSource = domain.RetryAtProvider
			}
			result.Waiting = e
			m.Waiting = e
			if writeErr := steps.WriteJSONAtomic(manifestFile, m); writeErr != nil {
				return pipeline.StepResult{}, writeErr
			}
			if saveErr := s.SaveCheckpoint(ctx, m); saveErr != nil {
				return pipeline.StepResult{}, saveErr
			}
			// D7 exhaustion is a parked step, not a failed/Cần xử lý step.
			return result, nil
		}
		return pipeline.StepResult{}, err
	}
	return result, nil
}

func drainIntents(ctx context.Context, s *pipeline.StepContext, m *Manifest) (bool, error) {
	if s.DrainImageIntents == nil {
		return false, nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return false, err
	}
	result, err := s.DrainImageIntents(ctx, raw)
	if err != nil {
		return false, err
	}
	var next Manifest
	if err := json.Unmarshal(result, &next); err != nil {
		return false, err
	}
	changed := next.Revision != m.Revision
	*m = next
	return changed, nil
}

func (h Handler) runQueue(ctx context.Context, s *pipeline.StepContext, m *Manifest) error {
	if s.ImageSlot == nil {
		return steps.CloneError(domain.CloneErrToolMissing, "Thiếu image-slot semaphore", "")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now
	if h.Deps.Now != nil {
		now = h.Deps.Now
	}
	encode := h.Deps.Encode
	if encode == nil {
		encode = h.encode
	}
	cutout := h.Deps.Cutout
	if cutout == nil {
		cutout = h.cutout
	}
	var mu sync.Mutex
	var first error
	var wg sync.WaitGroup
	save := func() error { m.Revision++; return s.SaveCheckpoint(ctx, *m) }
	for index := range m.Items {
		mu.Lock()
		pending := m.ImageMode != "none" && (m.Items[index].Status == "pending" || m.Items[index].Status == "running")
		if len(m.QueueOnly) > 0 {
			selected := false
			for _, id := range m.QueueOnly {
				if m.Items[index].ID == id {
					selected = true
					break
				}
			}
			pending = pending && selected
		}
		mu.Unlock()
		if !pending {
			continue
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			err := s.WithImageSlot(ctx, func(slotctx context.Context) (slotErr error) {
				// Latch every failure before the slot wrapper can admit its next waiter.
				// Branch-local unlock defers run before this callback-wide guard.
				defer func() {
					if slotErr != nil {
						mu.Lock()
						if first == nil {
							first = slotErr
						}
						mu.Unlock()
					}
				}()
				mu.Lock()
				if first != nil {
					mu.Unlock()
					return nil
				}
				if s.PauseRequested() {
					first = pipeline.ErrPaused
					mu.Unlock()
					return pipeline.ErrPaused
				}
				item := &m.Items[index]
				if item.InFlight == nil {
					prompt, err := BuildManifestImagePrompt(*m, *item)
					if err != nil {
						mu.Unlock()
						return err
					}
					version := 1
					for _, v := range item.Versions {
						version = max(version, v.Version+1)
					}
					generation := steps.FingerprintHex(map[string]any{"job": s.Job.ID, "generation": s.Job.Generation, "id": item.ID, "version": version, "promptVersion": item.PromptVersion, "source": SourceFingerprint(*item), "prompt": prompt})[:24]
					item.InFlight = &Flight{GenerationID: generation, Version: version, Prompt: prompt, PromptVersion: item.PromptVersion, SourceFingerprint: SourceFingerprint(*item), PNG: filepath.ToSlash(filepath.Join("images", "flights", item.ID, generation+".png"))}
				}
				item.Status = "running"
				item.Error = ""
				if err := save(); err != nil {
					mu.Unlock()
					return err
				}
				flight := *item.InFlight
				localItem := *item
				mu.Unlock()
				png := filepath.Join(s.StagingDir, filepath.FromSlash(flight.PNG))
				if err := os.MkdirAll(filepath.Dir(png), 0755); err != nil {
					return err
				}
				key := fmt.Sprintf("%s:%d:%s:%s", s.Job.ID, s.Job.Generation, localItem.ID, flight.GenerationID)
				receipt, err := h.Deps.Drawer.Draw(slotctx, imagegen.Request{Key: key, Prompt: flight.Prompt, Output: png, Width: localItem.Width, Height: localItem.Height})
				if imagegen.IsImageRefused(err) && !localItem.Modest {
					mu.Lock()
					m.Items[index].Modest = true
					localItem.Modest = true
					flight.Prompt, err = BuildManifestImagePrompt(*m, localItem)
					if err != nil {
						mu.Unlock()
						return err
					}
					m.Items[index].InFlight.Prompt = flight.Prompt
					if e := save(); e != nil {
						mu.Unlock()
						return e
					}
					mu.Unlock()
					receipt, err = h.Deps.Drawer.Draw(slotctx, imagegen.Request{Key: key, Prompt: flight.Prompt, Output: png, Width: localItem.Width, Height: localItem.Height})
				}
				if err != nil {
					mu.Lock()
					defer mu.Unlock()
					item = &m.Items[index]
					if imagegen.IsImageRefused(err) {
						remaining := 0
						for _, i := range m.Items {
							if i.Kind == "scene" && i.Status != "skipped" {
								remaining++
							}
						}
						if Current(*item) != nil {
							item.Status = "approved"
							item.InFlight = nil
							return save()
						}
						if item.Kind == "scene" && remaining > 1 {
							item.Status = "skipped"
							item.SkipReason = "refused"
							item.InFlight = nil
							return save()
						}
						item.Status = "failed"
						item.InFlight = nil
					} else {
						item.Status = "pending"
					} // flight survives quota/crash/encode recovery
					item.Error = err.Error()
					if e := save(); e != nil {
						return e
					}
					return err
				}
				relative := fmt.Sprintf("images/versions/%s/%d-%s.webp", localItem.ID, flight.Version, flight.GenerationID)
				webp := filepath.Join(s.StagingDir, filepath.FromSlash(relative))
				if err = os.MkdirAll(filepath.Dir(webp), 0755); err != nil {
					return err
				}
				encoded, err := encode(slotctx, s, png, webp, localItem.Width, localItem.Height)
				if err != nil {
					return err
				}
				version := Version{Version: flight.Version, GenerationID: flight.GenerationID, CreatedAt: now().UTC().Format(time.RFC3339Nano), Prompt: flight.Prompt, PromptVersion: flight.PromptVersion, RefHash: localItem.Reference.SHA256, SourceFingerprint: flight.SourceFingerprint, Path: relative, SHA256: encoded.SHA256, Width: encoded.Width, Height: encoded.Height, Bytes: encoded.Bytes, Mime: "image/webp", Receipt: &receipt}
				artifacts := []media.Artifact{{Path: relative, File: webp, ContentType: "image/webp"}, {Path: flight.PNG, File: png, ContentType: "image/png"}}
				if localItem.Kind == "scene" && strings.HasPrefix(m.Style, "cutout") {
					cut := strings.TrimSuffix(relative, ".webp") + ".cutout.png"
					cutfile := filepath.Join(s.StagingDir, filepath.FromSlash(cut))
					if err = cutout(slotctx, s, png, cutfile); err != nil {
						return err
					}
					hash, err := steps.SHA256File(cutfile)
					if err != nil {
						return err
					}
					version.Cutout = &scenes.FileRef{Path: cut, SHA256: hash}
					artifacts = append(artifacts, media.Artifact{Path: cut, File: cutfile, ContentType: "image/png"})
				}
				mu.Lock()
				defer mu.Unlock()
				// Publish and manifest checkpoint share the media transaction. A crash
				// cannot expose a selected version without its immutable bytes.
				previous := m.Items[index]
				item = &m.Items[index]
				item.Versions = append(item.Versions, version)
				n := version.Version
				item.ReviewVersion = &n
				item.Status = "review"
				item.InFlight = nil
				item.Prompt = flight.Prompt
				m.Revision++
				if err = s.Publish(slotctx, artifacts, func(txctx context.Context) error { return s.SaveCheckpoint(txctx, *m) }); err != nil {
					m.Items[index] = previous
					return err
				}
				done := int64(0)
				for _, i := range m.Items {
					if i.Status == "review" || i.Status == "approved" || i.Status == "skipped" {
						done++
					}
				}
				total := int64(len(m.Items))
				return s.Progress(ctx, domain.CloneProgress{UnitsDone: done, UnitsTotal: &total, Phase: "images"})
			})
			if err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(index)
	}
	wg.Wait()
	if errors.Is(first, pipeline.ErrPaused) {
		if err := s.SaveCheckpoint(ctx, *m); err != nil {
			return err
		}
	}
	return first
}

// visionModel adds acquired reference bytes to the first user turn. Contract
// corrections retain the exact same reference ordering without disk tools.
type visionModel struct {
	llmretry.Generator
	Images []llm.ContentBlock
}

func (v visionModel) Generate(ctx context.Context, msg []llm.Message, tools []llm.ToolSpec, opts ...llm.CallOption) (*llm.LLMResponse, error) {
	copied := append([]llm.Message(nil), msg...)
	for n := range copied {
		if copied[n].Role == llm.RoleUser {
			copied[n].Content = append(append([]llm.ContentBlock(nil), copied[n].Content...), v.Images...)
			break
		}
	}
	return v.Generator.Generate(ctx, copied, tools, opts...)
}
func (h Handler) analyse(ctx context.Context, prompt string, refs, ids []string, fingerprint, look, thumbnail string) (Story, error) {
	if h.Deps.Model == nil {
		return Story{}, steps.CloneError(domain.CloneErrToolMissing, "Thiếu model viết mô tả ảnh", "Kiểm tra Kết nối AI trong Cài đặt")
	}
	model := visionModel{Generator: h.Deps.Model}
	for _, file := range refs {
		data, err := os.ReadFile(file)
		if err != nil {
			return Story{}, err
		}
		model.Images = append(model.Images, llm.ImageBlock(base64.StdEncoding.EncodeToString(data), http.DetectContentType(data)))
	}
	schema := map[string]any{"type": "object", "required": []string{"cast", "shots"}, "properties": map[string]any{"cast": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"name", "role", "look"}, "properties": map[string]any{"name": map[string]any{"type": "string"}, "role": map[string]any{"type": "string"}, "look": map[string]any{"type": "string"}}}}, "shots": map[string]any{"type": "array", "items": map[string]any{"type": "object", "required": []string{"id", "prompt"}, "properties": map[string]any{"id": map[string]any{"type": "string"}, "prompt": map[string]any{"type": "string"}}}}, "thumbnailText": map[string]any{"type": "string"}}}
	var parsed Story
	_, err := llmcontract.Execute[json.RawMessage](ctx, model, llmcontract.Request[json.RawMessage]{Agent: "artist", Contract: llmcontract.Contract{Name: "clone.story", Schema: schema}, SystemPrompt: prompt, Payload: "Write the cast sheet and shots for the attached reference images.", Validate: func(raw *json.RawMessage) error {
		var err error
		parsed, err = ParseStoryAnswer(string(*raw), ids, fingerprint, look, thumbnail)
		return err
	}})
	return parsed, err
}
