package imp

import (
	"context"
	"fmt"
	"strings"
)

const filePreview = "preview.json"

// PreviewState is the durable import confirmation shown after reconnecting.
type PreviewState struct {
	SchemaVersion int            `json:"schemaVersion"`
	Preview       *Preview       `json:"preview"`
	Path          string         `json:"path"`
	Options       PreviewOptions `json:"options"`
}

type PreviewOptions struct {
	SchemaVersion int    `json:"schemaVersion"`
	StoryStatus   string `json:"storyStatus"`
	ContinueAfter bool   `json:"continueAfter"`
	Guide         string `json:"guide"`
}

// PreparePreview executes only ingestion and Architect segmentation. Confirmation
// and the later import consume this same digest-bound segmentation artifact.
func PreparePreview(ctx context.Context, deps Deps, opts Options) (Preview, error) {
	if deps.Store == nil || deps.Segment.Model == nil {
		return Preview{}, fmt.Errorf("Chưa cấu hình Kiến trúc sư để chia chương")
	}
	if deps.Budgets == (RunBudgets{}) {
		deps.Budgets = budgetsFromDeps(deps)
	}
	r := &runner{deps: deps, opts: opts, ws: OpenWorkspace(deps.Store.Dir()), events: make(chan Event, 32)}
	if err := r.checkSourceIdentity(); err != nil {
		return Preview{}, err
	}
	if !r.ws.Active() {
		if err := r.ingest(ctx); err != nil {
			return Preview{}, err
		}
	}
	old, _ := readArtifact[Segmentation](r.ws, fileSegmentation)
	if err := r.applyOptions(); err != nil {
		return Preview{}, err
	}
	if err := r.applyGuidance(); err != nil {
		return Preview{}, err
	}
	src, err := r.ws.LoadSource()
	if err != nil {
		return Preview{}, err
	}
	guide, err := r.ws.LoadGuidance()
	if err != nil {
		return Preview{}, err
	}
	digest := segmentInputDigest(Digest(src), guide, segmentPromptVersion)
	if old == nil || old.InputDigest != digest {
		if err := r.segment(ctx); err != nil {
			return Preview{}, err
		}
	}
	art, err := readArtifact[Segmentation](r.ws, fileSegmentation)
	if err != nil {
		return Preview{}, err
	}
	// Repeated previews retain the original newlySplit markers, not an empty diff.
	if cached, err := readArtifact[Preview](r.ws, filePreview); err == nil && cached.InputDigest == art.InputDigest {
		return cached.Payload, nil
	}
	m, err := r.ws.LoadManifest()
	if err != nil {
		return Preview{}, err
	}
	var before *Segmentation
	if old != nil && old.InputDigest != art.InputDigest {
		before = &old.Payload
	}
	out := previewSegmentation(src, m, &art.Payload, before, guide)
	if err := writeArtifact(r.ws, filePreview, art.InputDigest, out); err != nil {
		return Preview{}, err
	}
	return out, nil
}

// LoadPreviewState never calls a model and never returns a stale segmentation.
func LoadPreviewState(bookDir string) (PreviewState, error) {
	out := PreviewState{SchemaVersion: 1, Options: PreviewOptions{SchemaVersion: 1}}
	ws := OpenWorkspace(bookDir)
	if !ws.Active() {
		return out, nil
	}
	m, err := ws.LoadManifest()
	if err != nil {
		return out, err
	}
	out.Path = m.SourceName
	in, err := ws.LoadIntent()
	if err != nil {
		return out, err
	}
	guide, err := ws.LoadGuidance()
	if err != nil {
		return out, err
	}
	out.Options = PreviewOptions{SchemaVersion: 1, StoryStatus: in.StoryResolution, ContinueAfter: in.ContinueAfterImport, Guide: guide}
	if out.Options.StoryStatus == "" {
		out.Options.StoryStatus = storyOpen
	}
	src, err := ws.LoadSource()
	if err != nil {
		return out, err
	}
	art, err := readArtifact[Segmentation](ws, fileSegmentation)
	if err != nil {
		if !ws.has(fileSegmentation) {
			return out, nil
		}
		return out, err
	}
	if art.InputDigest != segmentInputDigest(Digest(src), guide, segmentPromptVersion) {
		return out, nil
	}
	if cached, err := readArtifact[Preview](ws, filePreview); err == nil && cached.InputDigest == art.InputDigest {
		out.Preview = &cached.Payload
	} else {
		p := previewSegmentation(src, m, &art.Payload, nil, guide)
		out.Preview = &p
	}
	return out, nil
}

func previewSegmentation(src []byte, m *Manifest, seg, before *Segmentation, guide string) Preview {
	out := Preview{SchemaVersion: 1, Filename: m.SourceName, Size: m.SizeBytes, Words: len(strings.Fields(string(src))), Encoding: m.Encoding, SplitHow: "Kiến trúc sư nhận diện ranh giới chương", Chapters: []PreviewChapter{}, Warnings: append([]string{}, seg.Notes...)}
	if guide != "" {
		out.SplitHow = "Kiến trúc sư chia theo hướng dẫn của bạn"
	}
	split := make(map[int]bool)
	if before != nil {
		for _, old := range before.Chapters {
			var children []int
			for i, ch := range seg.Chapters {
				if ch.Start >= old.Start && ch.End <= old.End {
					children = append(children, i)
				}
			}
			if len(children) > 1 {
				for _, i := range children {
					split[i] = true
				}
			}
		}
	}
	for i, ch := range seg.Chapters {
		body := strings.TrimSpace(seg.Content(src, i))
		first, rest, _ := strings.Cut(body, "\n")
		if strings.TrimSpace(first) == ch.Title {
			body = strings.TrimSpace(rest)
		}
		first, _, _ = strings.Cut(body, "\n")
		first = strings.TrimSpace(first)
		out.Chapters = append(out.Chapters, PreviewChapter{N: ch.Number, Title: ch.Title, Words: len(strings.Fields(body)), FirstLine: first, NewlySplit: split[i]})
	}
	warnLongChapters(&out)
	return out
}

// applyOptions persists explicit choices made at confirmation, including turning
// --continue off. A resumed job and Host handoff read the same intent.
func (r *runner) applyOptions() error {
	if !r.ws.Active() {
		return nil
	}
	in, err := r.ws.LoadIntent()
	if err != nil {
		return err
	}
	changed := false
	if r.opts.StoryResolution != "" && in.StoryResolution != r.opts.StoryResolution {
		in.StoryResolution = r.opts.StoryResolution
		changed = true
	}
	if (r.opts.ContinueAfterSet || r.opts.ContinueAfter) && in.ContinueAfterImport != r.opts.ContinueAfter {
		in.ContinueAfterImport = r.opts.ContinueAfter
		changed = true
	}
	if changed {
		return r.ws.writeJSON(fileIntent, in)
	}
	return nil
}
