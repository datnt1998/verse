package build

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
)

type Deps struct {
	Ffmpeg, FFprobe string
	Now             func() time.Time
	ExtractAudio    func(context.Context, *pipeline.StepContext, string, string) error
	MeasureLUFS     func(context.Context, *pipeline.StepContext, string) (float64, error)
	Envelope        func(context.Context, *pipeline.StepContext, string, string) (editor.Object, error)
}
type Handler struct{ Deps Deps }

var _ pipeline.StepHandler = Handler{}

type Checkpoint struct {
	Version            int                     `json:"version"`
	Generation         int                     `json:"generation"`
	Fingerprint        string                  `json:"fingerprint"`
	ProjectID          string                  `json:"projectId"`
	ProjectRevision    string                  `json:"projectRevision"`
	SourceSHA256       string                  `json:"sourceSha256"`
	AudioSHA256        string                  `json:"audioSha256"`
	SourceLoudnessLUFS float64                 `json:"sourceLoudnessLufs"`
	TemplateBinding    library.TemplateBinding `json:"templateSnapshot"`
}

func (h Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	if err := ctx.Err(); err != nil {
		return pipeline.StepResult{}, err
	}
	var snap library.Snapshot
	if err := json.Unmarshal(s.Job.TemplateSnapshot, &snap); err != nil {
		return pipeline.StepResult{}, err
	}
	if len(snap.Project) == 0 {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrTemplateChanged, "Template thiếu project đã chốt", "Chọn lại template để dựng")
	}
	template, err := editor.Parse(snap.Project)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	templateHash, err := validateTemplateSnapshot(snap, template, s.Job.TemplateID)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	store := editor.Store{Media: s.Media, StagingDir: s.StagingDir, Now: h.Deps.Now}
	artifacts := []media.Artifact{}
	acquire := func(path, expected string) (*media.Handle, error) {
		handle, err := s.Acquire(ctx, path)
		if err != nil {
			return nil, err
		}
		hash, err := steps.SHA256File(handle.Path)
		if err != nil {
			handle.Release()
			return nil, err
		}
		if expected != "" && hash != strings.TrimPrefix(expected, "sha256:") {
			handle.Release()
			return nil, steps.CloneError(domain.CloneErrStale, "Artifact đã thay đổi: "+path, "Chạy lại bước tạo artifact")
		}
		if path == "download/source.mp4" {
			return handle, nil
		}
		staged := filepath.Join(s.StagingDir, "inputs", filepath.FromSlash(path))
		err = steps.CopyFile(handle.Path, staged)
		handle.Release()
		if err != nil {
			return nil, err
		}
		return &media.Handle{Path: staged, Ref: media.Ref{Path: path, SHA256: hash}}, nil
	}
	// Verify upstream manifests against the fenced output records before any
	// editor media import. No filesystem-derived inference substitutes a digest.
	for _, step := range []domain.CloneStep{domain.CloneDownload, domain.CloneTranscript, domain.CloneScenes, domain.CloneImages} {
		if out, ok := s.Upstream[step]; ok {
			for _, a := range out.Artifacts {
				handle, err := s.Acquire(ctx, a.Path)
				if err != nil {
					return pipeline.StepResult{}, err
				}
				hash, err := steps.SHA256File(handle.Path)
				handle.Release()
				if err != nil {
					return pipeline.StepResult{}, err
				}
				if a.SHA256 != "" && hash != strings.TrimPrefix(a.SHA256, "sha256:") {
					return pipeline.StepResult{}, steps.CloneError(domain.CloneErrStale, "Artifact đã thay đổi: "+a.Path, "Chạy lại bước tạo artifact")
				}
			}
		}
	}
	// Check all bound template bytes before extraction or editor imports. Missing
	// media is drift too, not an unclassified filesystem error.
	for _, pin := range snap.Binding.Media {
		if err := verifyTemplateMedia(ctx, s, pin, snap.MediaFiles[pin.MediaID]); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	source, err := acquire("download/source.mp4", "")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer source.Release()
	sourceHash, err := steps.SHA256File(source.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	sceneFile, err := acquire("scenes/manifest.json", "")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer sceneFile.Release()
	sceneDoc, err := steps.DecodeJSONFile[scenes.Manifest](sceneFile.Path, "Invalid scene manifest")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if err = sceneDoc.Validate(); err != nil {
		return pipeline.StepResult{}, err
	}
	if err = validateBuildRecipe(s, sceneDoc); err != nil {
		return pipeline.StepResult{}, err
	}
	if sceneDoc.Source.Path != "download/source.mp4" || sceneDoc.Source.SHA256 != sourceHash {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrStale, "Cảnh không thuộc video nguồn hiện tại", "Phân tích lại cảnh")
	}
	imageFile, err := acquire("images/manifest.json", "")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer imageFile.Release()
	imageDoc, err := steps.DecodeJSONFile[images.Manifest](imageFile.Path, "Invalid image manifest")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	videoNoImages := sceneDoc.Recipe.ImagesMode == "none"
	if imageDoc.Overrides != nil && imageDoc.Overrides.NoImages != nil {
		videoNoImages = *imageDoc.Overrides.NoImages
	}
	byScene, thumbnailVersion, err := buildImageVersions(imageDoc, sceneDoc, s.Job.Source.Title)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	for _, scene := range sceneDoc.Scenes {
		if version, ok := byScene[scene.SceneID]; ok {
			if err = verifyBuildImage(ctx, s, version, strings.HasPrefix(sceneDoc.Recipe.Style, "cutout")); err != nil {
				return pipeline.StepResult{}, err
			}
		}
	}
	if err = verifyBuildImage(ctx, s, thumbnailVersion, false); err != nil {
		return pipeline.StepResult{}, err
	}
	captionFile, err := acquire("transcript/captions.json", "")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer captionFile.Release()
	captionBytes, err := os.ReadFile(captionFile.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	var captions struct {
		Cues []struct {
			ID      string   `json:"id"`
			Text    string   `json:"text"`
			StartMS float64  `json:"startMs"`
			EndMS   *float64 `json:"endMs"`
			Words   []struct {
				StartMS   float64 `json:"startMs"`
				EndMS     float64 `json:"endMs"`
				Text      string  `json:"text"`
				Estimated bool    `json:"estimated"`
			} `json:"words"`
		} `json:"cues"`
	}
	if err = json.Unmarshal(captionBytes, &captions); err != nil {
		return pipeline.StepResult{}, err
	}
	sceneHash, err := steps.SHA256File(sceneFile.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	imageHash, err := steps.SHA256File(imageFile.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	captionHash, err := steps.SHA256File(captionFile.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	fingerprint := steps.InputFingerprint(map[string]any{"generation": s.Job.Generation, "source": sourceHash, "scenes": sceneHash, "images": imageHash, "captions": captionHash, "template": templateHash, "binding": snap.Binding})
	projectID := fmt.Sprintf("clone-%s-g%d", s.Job.ID, s.Job.Generation)
	var previous Checkpoint
	if len(s.Checkpoint) > 0 {
		if err = json.Unmarshal(s.Checkpoint, &previous); err != nil {
			return pipeline.StepResult{}, err
		}
		if previous.Fingerprint != fingerprint {
			return pipeline.StepResult{}, steps.CloneError(domain.CloneErrStale, "Đầu vào dựng đã đổi", "Dựng lại với thế hệ mới")
		}
	}
	now := time.Now
	if h.Deps.Now != nil {
		now = h.Deps.Now
	}
	created := s.Job.CreatedAt
	if created.IsZero() {
		created = now()
	}
	project := editor.Clone(template)
	project["id"] = projectID
	project["name"] = s.Job.Source.Title
	project["createdAt"] = created.UnixMilli()
	project["updatedAt"] = created.UnixMilli()
	// Every bound asset is reimported below; do not keep project references to
	// the old media ids alongside the new content-addressed associations.
	project["mediaItems"] = []any{}
	audioFile := filepath.Join(s.StagingDir, "build", "source-audio.m4a")
	if err = os.MkdirAll(filepath.Dir(audioFile), 0755); err != nil {
		return pipeline.StepResult{}, err
	}
	extract := h.Deps.ExtractAudio
	if extract == nil {
		extract = h.extractAudio
	}
	if err = extract(ctx, s, source.Path, audioFile); err != nil {
		return pipeline.StepResult{}, err
	}
	source.Release()
	audioHash, err := steps.SHA256File(audioFile)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, media.Artifact{Path: "build/source-audio.m4a", File: audioFile, ContentType: "audio/mp4"})
	measure := h.Deps.MeasureLUFS
	if measure == nil {
		measure = h.measureLUFS
	}
	lufs, err := measure(ctx, s, audioFile)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	makeEnvelope := h.Deps.Envelope
	if makeEnvelope == nil {
		makeEnvelope = h.envelope
	}
	envelope, err := makeEnvelope(ctx, s, audioFile, "sha256:"+audioHash)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	mediaObjects := []editor.Object{}
	mediaIDs := []editor.Object{}
	importedRecords := map[string]editor.Object{}
	importedMetadata := map[string]editor.Object{}
	importFile := func(file, name, mime, hash string, width, height int, duration float64, tags []string) (string, string, error) {
		if !plainDigest(hash) {
			return "", "", fmt.Errorf("invalid media digest")
		}
		id := "media-" + hash[:32]
		sourcePath := "editor/media/" + id + "/" + name
		src := "/api/editor/media/" + id + "?v=" + url.QueryEscape("sha256:"+hash)
		metadataFile := filepath.Join(s.StagingDir, "editor", "media", id, "metadata.json")
		if record, ok := importedRecords[id]; ok {
			merged := append([]string{}, record["tags"].([]string)...)
			for _, tag := range tags {
				found := false
				for _, existing := range merged {
					if existing == tag {
						found = true
						break
					}
				}
				if !found {
					merged = append(merged, tag)
				}
			}
			record["tags"] = merged
			importedMetadata[id]["tags"] = merged
			if err := steps.WriteJSONAtomic(metadataFile, record); err != nil {
				return "", "", err
			}
			return id, src, nil
		}
		info, err := os.Stat(file)
		if err != nil {
			return "", "", err
		}
		fps := 0
		if strings.HasPrefix(mime, "video/") {
			fps = 30
		}
		codec := "unknown"
		if mime == "audio/mp4" && len(tags) > 0 && tags[0] == "clone-audio" {
			codec = "aac"
		}
		tags = append([]string{}, tags...)
		probe := editor.Object{"kind": strings.Split(mime, "/")[0], "mime": mime, "width": width, "height": height, "durationSec": duration, "fps": fps}
		if strings.HasPrefix(mime, "audio/") {
			probe["audioCodec"] = codec
		}
		record := editor.Object{"id": id, "fileName": name, "mime": mime, "size": info.Size(), "contentHash": "sha256:" + hash, "createdAt": created.UnixMilli(), "probe": probe, "sourceFile": name, "storageType": "workspace", "tags": tags}
		if err = steps.WriteJSONAtomic(metadataFile, record); err != nil {
			return "", "", err
		}
		artifacts = append(artifacts, media.Artifact{Path: sourcePath, File: file, ContentType: mime}, media.Artifact{Path: "editor/media/" + id + "/metadata.json", File: metadataFile, ContentType: "application/json"})
		metadata := editor.Object{"id": id, "fileName": name, "fileSize": info.Size(), "mimeType": mime, "contentHash": "sha256:" + hash, "storageType": "workspace", "width": width, "height": height, "duration": duration, "fps": fps, "codec": codec, "bitrate": 0, "tags": tags, "createdAt": created.UnixMilli(), "updatedAt": created.UnixMilli()}
		importedRecords[id] = record
		importedMetadata[id] = metadata
		mediaObjects = append(mediaObjects, metadata)
		mediaIDs = append(mediaIDs, editor.Object{"id": id, "addedAt": created.UnixMilli()})
		return id, src, nil
	}
	audioID, audioSrc, err := importFile(audioFile, "source-audio.m4a", "audio/mp4", audioHash, 0, 0, sceneDoc.DurationSec, []string{"clone-audio"})
	if err != nil {
		return pipeline.StepResult{}, err
	}
	templateMediaIDs := make([]string, 0, len(snap.MediaFiles))
	for id := range snap.MediaFiles {
		templateMediaIDs = append(templateMediaIDs, id)
	}
	sort.Strings(templateMediaIDs)
	for _, oldID := range templateMediaIDs {
		path := snap.MediaFiles[oldID]
		expected := ""
		for _, binding := range snap.Binding.Media {
			if binding.MediaID == oldID {
				expected = binding.ContentHash
				break
			}
		}
		file, err := acquire(path, expected)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		hash, err := steps.SHA256File(file.Path)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		w, hh, duration := findMediaGeometry(project.Timeline(), oldID)
		if strings.HasPrefix(mimeOf(path), "image/") {
			w, hh, err = imageGeometry(file.Path)
			if err != nil {
				return pipeline.StepResult{}, err
			}
		}
		if strings.HasPrefix(mimeOf(path), "video/") && duration <= 0 {
			facts, err := h.probe(ctx, s, file.Path)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			w, hh, duration = facts.Width, facts.Height, facts.Duration
		}
		id, src, err := importFile(file.Path, filepath.Base(path), mimeOf(path), hash, w, hh, duration, nil)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		rewriteMedia(project.Timeline(), oldID, id, src)
	}
	backdrops := []editor.Object{}
	if upstream, ok := s.Upstream[domain.CloneBackdrop]; ok {
		if value, ok := upstream.Data["manifest"]; ok {
			raw, err := json.Marshal(value)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			manifest, err := backdrop.Parse(raw)
			if err != nil {
				return pipeline.StepResult{}, err
			}
			for _, item := range backdrop.ReadyItems(manifest) {
				if item.MediaID == nil {
					continue
				}
				file, err := acquire(*item.MediaID, "")
				if err != nil {
					return pipeline.StepResult{}, err
				}
				hash, err := steps.SHA256File(file.Path)
				if err != nil {
					return pipeline.StepResult{}, err
				}
				facts, err := h.probe(ctx, s, file.Path)
				if err != nil {
					return pipeline.StepResult{}, err
				}
				id, src, err := importFile(file.Path, filepath.Base(*item.MediaID), "video/mp4", hash, facts.Width, facts.Height, facts.Duration, []string{"clone-backdrop"})
				if err != nil {
					return pipeline.StepResult{}, err
				}
				clip := canonicalBackdrop(id, src, facts, int(math.Round(facts.Duration*30)))
				clip["cloneBackdropId"] = item.ID
				backdrops = append(backdrops, clip)
			}
		}
	}
	resolveVersion := func(sceneID string) (images.Version, error) {
		if v, ok := byScene[sceneID]; ok {
			return v, nil
		}
		target := 0
		for n, scene := range sceneDoc.Scenes {
			if scene.SceneID == sceneID {
				target = n
				break
			}
		}
		// A skipped scene uses the nearest earlier image; leading skipped scenes
		// use the first following image. Distance never lets a later image win.
		for n := target - 1; n >= 0; n-- {
			if v, ok := byScene[sceneDoc.Scenes[n].SceneID]; ok {
				return v, nil
			}
		}
		for n := target + 1; n < len(sceneDoc.Scenes); n++ {
			if v, ok := byScene[sceneDoc.Scenes[n].SceneID]; ok {
				return v, nil
			}
		}
		return images.Version{}, steps.CloneError(domain.CloneErrAwaitingReview, "Chưa có ảnh cảnh hợp lệ", "Vẽ ảnh trước khi dựng")
	}
	fillScenes := []editor.FillScene{}
	for _, scene := range sceneDoc.Scenes {
		if videoNoImages {
			continue
		}
		v, err := resolveVersion(scene.SceneID)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		ref := scenes.FileRef{Path: v.Path, SHA256: v.SHA256}
		w, hh := v.Width, v.Height
		mime := "image/webp"
		if strings.HasPrefix(sceneDoc.Recipe.Style, "cutout") {
			ref = *v.Cutout
			mime = "image/png"
		}
		file, err := acquire(ref.Path, ref.SHA256)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		defer file.Release()
		hash, err := steps.SHA256File(file.Path)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		id, src, err := importFile(file.Path, filepath.Base(ref.Path), mime, hash, w, hh, 0, []string{"clone-scene"})
		if err != nil {
			return pipeline.StepResult{}, err
		}
		var subject *editor.SubjectBox
		if sceneDoc.Recipe.Style == "cutout-close" {
			subject, err = measureSubjectBox(file.Path)
			if err != nil {
				return pipeline.StepResult{}, err
			}
		}
		fillScenes = append(fillScenes, editor.FillScene{SceneID: scene.SceneID, Title: scene.Text, StartFrame: scene.StartFrame, EndFrame: scene.EndFrame, MediaID: id, Src: src, SubjectBox: subject})
	}
	thumbnailFile, err := acquire(thumbnailVersion.Path, thumbnailVersion.SHA256)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer thumbnailFile.Release()
	thumbnailID, _, err := importFile(thumbnailFile.Path, filepath.Base(thumbnailVersion.Path), thumbnailVersion.Mime, thumbnailVersion.SHA256, thumbnailVersion.Width, thumbnailVersion.Height, 0, []string{"clone-thumbnail"})
	if err != nil {
		return pipeline.StepResult{}, err
	}
	fillCues := []editor.FillCue{}
	for _, cue := range captions.Cues {
		if cue.EndMS == nil || *cue.EndMS <= cue.StartMS {
			continue
		}
		out := editor.FillCue{ID: cue.ID, Text: cue.Text, StartSec: cue.StartMS / 1000, EndSec: *cue.EndMS / 1000}
		for _, word := range cue.Words {
			out.Words = append(out.Words, editor.FillCueWord{StartMs: word.StartMS, EndMs: word.EndMS, Text: word.Text, Estimated: word.Estimated})
		}
		fillCues = append(fillCues, out)
	}
	seed := uint32(0x811c9dc5)
	for _, c := range []byte(s.Job.ID) {
		seed ^= uint32(c)
		seed *= 0x01000193
	}
	filled, err := editor.Fill(editor.FillInput{Project: project, DurationFrames: sceneDoc.TotalFrames, IDPrefix: projectID, Seed: seed, Scenes: fillScenes, Cues: fillCues, Audio: editor.FillAudio{MediaID: audioID, Src: audioSrc, DurationFrames: sceneDoc.TotalFrames, MeasuredLUFS: &lufs}, Media: mediaObjects, Envelope: envelope, Backdrops: backdrops})
	if err != nil {
		return pipeline.StepResult{}, err
	}
	projectArtifact, projectRevision, err := store.Stage(filled.Project)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, projectArtifact)
	linksFile := filepath.Join(s.StagingDir, "editor", "projects", projectID, "links.json")
	if err = steps.WriteJSONAtomic(linksFile, editor.Object{"version": "1.0", "mediaIds": mediaIDs}); err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, media.Artifact{Path: "editor/projects/" + projectID + "/links.json", File: linksFile, ContentType: "application/json"})
	checkpoint := Checkpoint{Version: 1, Generation: s.Job.Generation, Fingerprint: fingerprint, ProjectID: projectID, ProjectRevision: projectRevision, SourceSHA256: sourceHash, AudioSHA256: audioHash, SourceLoudnessLUFS: lufs, TemplateBinding: snap.Binding}
	file := filepath.Join(s.StagingDir, "build", "checkpoint.json")
	if err = steps.WriteJSONAtomic(file, checkpoint); err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, media.Artifact{Path: "build/checkpoint.json", File: file, ContentType: "application/json"})
	audioEvidence := BuildAudio{MediaID: audioID, SourceLUFS: lufs, ReferenceVersion: 1, DurationInFrames: sceneDoc.TotalFrames, SHA256: audioHash}
	if filled.Loudness != nil {
		audioEvidence.LoudnessTargetLUFS = &filled.Loudness.TargetLUFS
		audioEvidence.AppliedGainDB = &filled.Loudness.AppliedGainDB
	}
	manifest := Manifest{Version: 1, GeneratedAt: now().UTC().Format(time.RFC3339Nano), ProjectID: projectID, Generation: s.Job.Generation, BaseRevision: templateHash, CommittedRevision: projectRevision, RecipeFingerprint: fingerprint, ItemIDs: filled.ItemIDs, Audio: audioEvidence, Source: ManifestMedia{SHA256: sourceHash}, Envelope: editor.Object{"sourceHash": envelope["sourceHash"], "durationSeconds": envelope["durationSeconds"], "samplesPerSecond": envelope["samplesPerSecond"]}, Caption: filled.Captions, Thumbnail: ManifestMedia{MediaID: thumbnailID, SHA256: thumbnailVersion.SHA256}, Scenes: fillScenes, CommandCount: len(filled.Commands)}
	manifestFile := filepath.Join(s.StagingDir, "build", "manifest.json")
	if err = steps.WriteJSONAtomic(manifestFile, manifest); err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, media.Artifact{Path: "build/manifest.json", File: manifestFile, ContentType: "application/json"})
	if s.PauseRequested() {
		if err = s.SaveCheckpoint(ctx, checkpoint); err != nil {
			return pipeline.StepResult{}, err
		}
		return pipeline.StepResult{}, pipeline.ErrPaused
	}
	// Do not approve candidate images or export. The owner receives a real
	// schema15 draft whose durable audio/media no longer depends on source.mp4.
	return pipeline.StepResult{Artifacts: artifacts, Data: map[string]any{"projectId": projectID, "projectPath": projectArtifact.Path, "projectRevision": projectRevision, "fingerprint": fingerprint, "sourceIndependent": true, "audioPath": "build/source-audio.m4a", "durationFrames": sceneDoc.TotalFrames, "durationSec": float64(sceneDoc.TotalFrames) / math.Max(1, float64(sceneDoc.FPS))}, Waiting: steps.CloneError(domain.CloneErrAwaitingReview, "Bản dựng đang chờ duyệt", "")}, nil
}
