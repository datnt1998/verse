package images

import (
	"encoding/json"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"strings"
	"unicode/utf8"
)

const PromptVersion = 4
const HookTextMax = 40

type Cast struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Look string `json:"look"`
}
type Story struct {
	Fingerprint   string            `json:"fingerprint"`
	Look          string            `json:"look,omitempty"`
	Cast          []Cast            `json:"cast"`
	Shots         map[string]string `json:"shots"`
	ThumbnailText string            `json:"thumbnailText,omitempty"`
}
type Version struct {
	Version           int               `json:"version"`
	GenerationID      string            `json:"generationId"`
	CreatedAt         string            `json:"createdAt"`
	Prompt            string            `json:"prompt"`
	PromptVersion     int               `json:"promptVersion"`
	RefHash           string            `json:"refHash"`
	SourceFingerprint string            `json:"sourceFingerprint"`
	Path              string            `json:"path"`
	SHA256            string            `json:"sha256"`
	Width             int               `json:"width"`
	Height            int               `json:"height"`
	Bytes             int64             `json:"bytes"`
	Mime              string            `json:"mime"`
	Receipt           *imagegen.Receipt `json:"receipt,omitempty"`
	Source            string            `json:"source,omitempty"`
	Cutout            *scenes.FileRef   `json:"cutout,omitempty"`
}
type Flight struct {
	GenerationID      string `json:"generationId"`
	Version           int    `json:"version"`
	Prompt            string `json:"prompt"`
	PromptVersion     int    `json:"promptVersion"`
	SourceFingerprint string `json:"sourceFingerprint"`
	PNG               string `json:"png"`
}
type Item struct {
	ID             string              `json:"id"`
	Kind           string              `json:"kind"`
	SceneID        string              `json:"sceneId,omitempty"`
	StartFrame     int                 `json:"startFrame,omitempty"`
	EndFrame       int                 `json:"endFrame,omitempty"`
	Width          int                 `json:"width"`
	Height         int                 `json:"height"`
	Reference      scenes.FileRef      `json:"reference"`
	Narration      string              `json:"narration"`
	Prompt         string              `json:"prompt"`
	PromptVersion  int                 `json:"promptVersion"`
	Direction      string              `json:"direction,omitempty"`
	Modest         bool                `json:"modest,omitempty"`
	Status         string              `json:"status"`
	Versions       []Version           `json:"versions"`
	CurrentVersion *int                `json:"currentVersion"`
	ReviewVersion  *int                `json:"reviewVersion"`
	InFlight       *Flight             `json:"inFlight,omitempty"`
	SkipReason     string              `json:"skipReason,omitempty"`
	Error          string              `json:"error,omitempty"`
	Context        *scenes.TextContext `json:"context"`
}

// Overrides records video-level selections; absent selections follow the template.
type Overrides struct {
	ScenePromptID  string `json:"scenePromptId,omitempty"`
	ScenePromptSet bool   `json:"scenePromptSet,omitempty"`
	CoverPromptID  string `json:"coverPromptId,omitempty"`
	CoverPromptSet bool   `json:"coverPromptSet,omitempty"`
	NoImages       *bool  `json:"noImages,omitempty"`
}
type Manifest struct {
	Version         int                `json:"version"`
	Revision        int                `json:"revision"`
	SceneRevision   int                `json:"sceneRevision"`
	Style           string             `json:"style"`
	Thumbnail       string             `json:"thumbnail"`
	TemplatePrompt  string             `json:"templatePrompt,omitempty"`
	Story           *Story             `json:"story,omitempty"`
	Items           []Item             `json:"items"`
	SceneGeneration int                `json:"sceneGeneration,omitempty"`
	ImageMode       string             `json:"imageMode,omitempty"`
	ImagePromptMode string             `json:"imagePromptMode,omitempty"`
	CoverPrompt     string             `json:"coverPrompt,omitempty"`
	Overrides       *Overrides         `json:"overrides,omitempty"`
	QueueOnly       []string           `json:"queueOnly,omitempty"`
	Waiting         *domain.CloneError `json:"waiting,omitempty"`
}

func SourceFingerprint(i Item) string {
	return steps.FingerprintHex(map[string]any{"kind": i.Kind, "sceneId": i.SceneID, "startFrame": i.StartFrame, "endFrame": i.EndFrame, "refHash": i.Reference.SHA256, "width": i.Width, "height": i.Height, "text": i.Narration, "context": i.Context})
}
func CreateManifest(m scenes.Manifest, thumbnail scenes.FileRef, title string) Manifest {
	out := Manifest{Version: 1, Revision: 1, SceneRevision: m.Revision, Style: m.Recipe.Style, Thumbnail: m.Recipe.Thumbnail, TemplatePrompt: m.Recipe.ImagePrompt, CoverPrompt: m.Recipe.CoverPrompt, Items: []Item{}}
	if m.Recipe.ImagesMode == "none" {
		out.ImageMode = "none"
	}
	if m.Recipe.ImagesMode != "none" {
		for _, s := range m.Scenes {
			out.Items = append(out.Items, Item{ID: s.SceneID, Kind: "scene", SceneID: s.SceneID, StartFrame: s.StartFrame, EndFrame: s.EndFrame, Width: 1920, Height: 1080, Reference: s.Keyframe, Narration: s.Text, Context: s.Context, PromptVersion: PromptVersion, Status: "pending", Versions: []Version{}})
		}
	}
	out.Items = append(out.Items, Item{ID: "thumbnail", Kind: "thumbnail", Width: 1280, Height: 720, Reference: thumbnail, Narration: title, PromptVersion: 1, Status: "pending", Versions: []Version{}})
	return out
}
func current(i Item, number *int) *Version {
	if number == nil {
		return nil
	}
	for n := range i.Versions {
		v := &i.Versions[n]
		if v.Version == *number && v.SourceFingerprint == SourceFingerprint(i) {
			return v
		}
	}
	return nil
}
func Current(i Item) *Version   { return current(i, i.CurrentVersion) }
func Candidate(i Item) *Version { return current(i, i.ReviewVersion) }
func Displayed(i Item) *Version {
	if i.ReviewVersion != nil {
		return current(i, i.ReviewVersion)
	}
	return Current(i)
}
func validVersion(m Manifest, i Item, v *Version) bool {
	return v != nil && (m.Style != "cutout" || i.Kind != "scene" || v.Cutout != nil && v.Cutout.Path != "")
}
func imageGate(m Manifest, approved bool) bool {
	cover, scene := false, m.ImageMode == "none"
	for _, i := range m.Items {
		if i.Kind == "thumbnail" {
			cover = true
		}
		if i.Kind == "scene" && i.Status != "skipped" {
			scene = true
		}
		if i.Kind == "scene" && i.Status == "skipped" {
			continue
		}
		v := Current(i)
		if !approved && i.Status == "review" {
			v = Candidate(i)
		}
		if approved && i.Status != "approved" || !approved && i.Status != "approved" && i.Status != "review" || !validVersion(m, i, v) {
			return false
		}
	}
	return cover && scene
}
func ImagesReady(m Manifest) bool    { return imageGate(m, false) }
func ImagesApproved(m Manifest) bool { return imageGate(m, true) }
func Reconcile(old, fresh Manifest) Manifest {
	fresh.Revision = old.Revision + 1
	fresh.Story = old.Story
	fresh.Overrides = old.Overrides
	if old.Overrides != nil {
		if old.Overrides.ScenePromptSet {
			fresh.TemplatePrompt = old.TemplatePrompt
			fresh.ImagePromptMode = old.ImagePromptMode
		}
		if old.Overrides.CoverPromptSet {
			fresh.CoverPrompt = old.CoverPrompt
			fresh.Thumbnail = old.Thumbnail
		}
		if old.Overrides.NoImages != nil {
			fresh.ImageMode = ""
			if *old.Overrides.NoImages {
				fresh.ImageMode = "none"
			}
		}
	}
	if fresh.ImageMode == "none" {
		kept := []Item{}
		for _, i := range fresh.Items {
			if i.Kind == "thumbnail" {
				kept = append(kept, i)
			}
		}
		fresh.Items = kept
	}
	sameStyle := old.Style == fresh.Style && old.TemplatePrompt == fresh.TemplatePrompt && old.ImagePromptMode == fresh.ImagePromptMode
	for n := range fresh.Items {
		for _, previous := range old.Items {
			if previous.ID != fresh.Items[n].ID {
				continue
			}
			if sameStyle && (fresh.Items[n].Kind != "thumbnail" || old.Thumbnail == fresh.Thumbnail && old.CoverPrompt == fresh.CoverPrompt) && SourceFingerprint(previous) == SourceFingerprint(fresh.Items[n]) {
				reference := fresh.Items[n].Reference
				fresh.Items[n] = previous
				fresh.Items[n].Reference = reference
			} else {
				fresh.Items[n].Versions = previous.Versions
				fresh.Items[n].CurrentVersion = nil
				fresh.Items[n].ReviewVersion = nil
			}
		}
	}
	return fresh
}

// Review is the pure image-review.ts transition. A pending candidate never
// displaces the selected image; approval chooses it explicitly.
func Review(m Manifest, id, op, prompt string, version *int) (Manifest, error) {
	var err error
	m, err = CloneManifest(m)
	if err != nil {
		return m, err
	}
	index := -1
	for n := range m.Items {
		if m.Items[n].ID == id {
			index = n
			break
		}
	}
	if index < 0 {
		return m, &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy ảnh"}
	}
	i := &m.Items[index]
	if i.Status == "running" || i.InFlight != nil {
		return m, &domain.CloneError{Code: domain.CloneErrConflict, Message: "Ảnh đang tạo hoặc cần phục hồi output đã lưu"}
	}
	if m.ImageMode == "none" && op != "approve" && op != "select" {
		return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Template này không tạo ảnh — đổi ảnh bìa bằng tải lên"}
	}
	switch op {
	case "approve", "select":
		chosen := version
		if chosen == nil {
			chosen = i.ReviewVersion
		}
		if chosen == nil {
			return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có ảnh để duyệt"}
		}
		found := false
		for _, v := range i.Versions {
			if v.Version == *chosen {
				found = true
			}
		}
		if !found {
			return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Chưa có ảnh để duyệt"}
		}
		if current(*i, chosen) == nil {
			return m, &domain.CloneError{Code: domain.CloneErrStale, Message: "Version ảnh không còn khớp cảnh hoặc nội dung nguồn"}
		}
		if !validVersion(m, *i, current(*i, chosen)) {
			return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Version ảnh chưa có bản tách nền — vẽ lại ảnh này rồi duyệt"}
		}
		i.CurrentVersion = chosen
		i.ReviewVersion = nil
		i.Status = "approved"
		i.SkipReason = ""
	case "editPrompt", "regenerate":
		if strings.TrimSpace(prompt) != "" {
			i.Direction = strings.TrimSpace(prompt)
			i.PromptVersion++
		}
		i.Status = "pending"
		i.ReviewVersion = nil
		i.Error = ""
		i.SkipReason = ""
	case "skip":
		remaining := 0
		for _, j := range m.Items {
			if j.Kind == "scene" && j.Status != "skipped" {
				remaining++
			}
		}
		if i.Kind != "scene" || remaining <= 1 {
			return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Cần giữ ít nhất một ảnh cảnh; không thể bỏ qua ảnh bìa"}
		}
		i.Status = "skipped"
		i.SkipReason = "owner"
		i.CurrentVersion = nil
		i.ReviewVersion = nil
	case "restore":
		i.Status = "pending"
		i.SkipReason = ""
	default:
		return m, &domain.CloneError{Code: domain.CloneErrInvalidRequest, Message: "Hành động duyệt ảnh không hợp lệ"}
	}
	m.Revision++
	return m, nil
}
func CloneManifest(m Manifest) (Manifest, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return Manifest{}, err
	}
	var result Manifest
	err = json.Unmarshal(raw, &result)
	return result, err
}
func ItemPrompt(m Manifest, i Item) string {
	if i.Kind == "thumbnail" && m.CoverPrompt != "" {
		return m.CoverPrompt
	}
	return m.TemplatePrompt
}
func generationPending(m Manifest) bool {
	if m.ImageMode == "none" {
		return false
	}
	for _, i := range m.Items {
		if i.Status != "pending" && i.Status != "running" {
			continue
		}
		if len(m.QueueOnly) == 0 {
			return true
		}
		for _, id := range m.QueueOnly {
			if i.ID == id {
				return true
			}
		}
	}
	return false
}
func ParseStoryAnswer(text string, ids []string, fingerprint, look, thumbnail string) (Story, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return Story{}, fmt.Errorf("analysis answer has no JSON object")
	}
	var a struct {
		Cast  []Cast `json:"cast"`
		Shots []struct {
			ID     string `json:"id"`
			Prompt string `json:"prompt"`
		} `json:"shots"`
		ThumbnailText string `json:"thumbnailText"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &a); err != nil {
		return Story{}, err
	}
	if len(a.Cast) > 24 {
		return Story{}, fmt.Errorf("too many cast members")
	}
	for _, c := range a.Cast {
		if c.Name == "" || utf8.RuneCountInString(c.Name) > 120 || c.Look == "" || utf8.RuneCountInString(c.Look) > 4000 || utf8.RuneCountInString(c.Role) > 600 {
			return Story{}, fmt.Errorf("invalid cast")
		}
	}
	out := Story{Fingerprint: fingerprint, Look: look, Cast: a.Cast, Shots: map[string]string{}}
	for _, shot := range a.Shots {
		for _, id := range ids {
			if id == shot.ID && shot.Prompt != "" && utf8.RuneCountInString(shot.Prompt) <= 8000 {
				out.Shots[id] = shot.Prompt
			}
		}
	}
	for _, id := range ids {
		if out.Shots[id] == "" {
			return Story{}, fmt.Errorf("analysis answer misses shot %s", id)
		}
	}
	if thumbnail == "hook" || thumbnail == "hook-arrow" {
		out.ThumbnailText = strings.ToUpper(strings.Join(strings.Fields(a.ThumbnailText), " "))
		if out.ThumbnailText == "" || utf8.RuneCountInString(out.ThumbnailText) > HookTextMax {
			return Story{}, fmt.Errorf("invalid thumbnailText")
		}
	}
	return out, nil
}
