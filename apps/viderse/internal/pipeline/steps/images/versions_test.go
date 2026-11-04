package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

// recorderDrawer ghi lại đúng yêu cầu đã gửi nhà cung cấp.
type recorderDrawer struct {
	mu       sync.Mutex
	requests []imagegen.Request
	err      error
	phases   *[]string
}

func (d *recorderDrawer) Draw(_ context.Context, r imagegen.Request) (imagegen.Receipt, error) {
	d.mu.Lock()
	d.requests = append(d.requests, r)
	d.mu.Unlock()
	if r.OnPhase != nil && d.phases != nil {
		r.OnPhase("sending")
		r.OnPhase("drawing")
	}
	if d.err != nil {
		return imagegen.Receipt{}, d.err
	}
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a5XcAAAAASUVORK5CYII=")
	if err := os.WriteFile(r.Output, data, 0600); err != nil {
		return imagegen.Receipt{}, err
	}
	hash, _ := steps.SHA256File(r.Output)
	return imagegen.Receipt{Provider: "codex", Model: imagegen.CodexImageModel, PngSha256: hash, CallID: fmt.Sprint(len(d.requests))}, nil
}

func (d *recorderDrawer) sent() []imagegen.Request {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]imagegen.Request(nil), d.requests...)
}

// sceneFixture dựng một video có ảnh bìa đã duyệt và các cảnh có khung nguồn.
func sceneFixture(t *testing.T, sceneIDs ...string) (*pipeline.StepContext, *stepstest.FakeMedia, Manifest, []byte) {
	t.Helper()
	s, m := stepstest.Ctx(t, domain.CloneImages, stepstest.Job("job-1"))
	scenesDoc := scenes.Manifest{Version: 1, Revision: 1, FPS: 30, TotalFrames: 30 * len(sceneIDs), DurationSec: float64(len(sceneIDs)), Recipe: scenes.Recipe{ImagePrompt: "Template: documentary framing."}}
	shotMap := map[string]string{}
	for index, id := range sceneIDs {
		ref := fmt.Sprintf("scenes/keyframes/%s.jpg", id)
		m.Put(ref, []byte("reference "+id))
		hash, _ := steps.SHA256File(m.Root + "/" + ref)
		scenesDoc.Scenes = append(scenesDoc.Scenes, scenes.Scene{SceneID: id, StartFrame: index * 30, EndFrame: (index + 1) * 30, KeyframeFrame: index*30 + 15, KeyframePTS: float64(index*30+15) / 30, Text: "shot " + id, Keyframe: scenes.FileRef{Path: ref, SHA256: hash}})
		shotMap[id] = "Ada walks through shot " + id
	}
	cover := "scenes/keyframes/cover.jpg"
	m.Put(cover, []byte("cover reference"))
	coverHash, _ := steps.SHA256File(m.Root + "/" + cover)
	raw, _ := json.Marshal(scenesDoc)
	m.Put("scenes/manifest.json", raw)
	s.Upstream[domain.CloneDownload] = domain.CloneStepOutput{Data: map[string]any{"thumbnail": scenes.FileRef{Path: cover, SHA256: coverHash}}}
	manifest := CreateManifest(scenesDoc, scenes.FileRef{Path: cover, SHA256: coverHash}, "Video")
	manifest.Story = &Story{Shots: shotMap, Cast: []Cast{{Name: "Ada", Look: "short black hair"}}}
	manifest.TemplatePrompt = "Template: documentary framing."
	for index := range manifest.Items {
		item := &manifest.Items[index]
		if item.Kind == "thumbnail" {
			// Ảnh bìa đã có bản đang dùng để cổng duyệt ảnh không chặn.
			item.Status = "approved"
			item.Versions = []Version{{Version: 1, Path: "images/versions/thumbnail/1-cover.webp", SHA256: "sha-cover", StateName: VersionReady, Width: 1280, Height: 720, SourceFingerprint: SourceFingerprint(*item)}}
			one := 1
			item.CurrentVersion = &one
			m.Put("images/versions/thumbnail/1-cover.webp", []byte("cover image"))
			continue
		}
		item.Status = "pending"
	}
	manifest.QueueOnly = append([]string(nil), sceneIDs...)
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	return s, m, manifest, raw
}

// sceneItem trả ảnh của một cảnh trong manifest đang lưu.
func sceneItem(t *testing.T, manifest Manifest, id string) Item {
	t.Helper()
	for _, item := range manifest.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("missing item %s", id)
	return Item{}
}

// markUsed gắn một bản đang dùng giả cho một ảnh trong manifest đang lưu.
func markUsed(t *testing.T, s *pipeline.StepContext, m *stepstest.FakeMedia, id string, version Version) Manifest {
	t.Helper()
	manifest := loadManifest(t, s)
	for index := range manifest.Items {
		item := &manifest.Items[index]
		if item.ID != id {
			continue
		}
		version.SourceFingerprint = SourceFingerprint(*item)
		version.Width, version.Height = item.Width, item.Height
		item.Versions = append(item.Versions, version)
		number := version.Version
		item.CurrentVersion = &number
		m.Put(version.Path, []byte("old image"))
	}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func handler(drawer Drawer, extra func(*Deps)) Handler {
	deps := Deps{Drawer: drawer, Analyse: analysis, Encode: encode}
	if extra != nil {
		extra(&deps)
	}
	return Handler{Deps: deps}
}

func loadManifest(t *testing.T, s *pipeline.StepContext) Manifest {
	t.Helper()
	var manifest Manifest
	if err := json.Unmarshal(s.Checkpoint, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

// TestSentTextIsExactlyWhatTheProviderReceives: sentText của phiên bản phải
// bằng từng byte chuỗi gửi nhà cung cấp, và các phần ghép lại đúng chuỗi đó.
func TestSentTextIsExactlyWhatTheProviderReceives(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0")
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if version.Prompt != sent[0].Prompt {
		t.Fatalf("sentText khác chuỗi đã gửi:\nsent=%q\nprovider=%q", version.Prompt, sent[0].Prompt)
	}
	if joined := JoinPrompt(version.Wrapper, version.Parts); joined != version.Prompt {
		t.Fatalf("wrapper + parts không ghép ra sentText:\njoined=%q\nsentText=%q", joined, version.Prompt)
	}
	// Các phần nối bằng một dòng trống, không phải một xuống dòng.
	if !strings.Contains(version.Prompt, version.Parts[0].Text+"\n\n"+version.Parts[1].Text) {
		t.Fatalf("các phần không nối bằng dòng trống:\n%s", version.Prompt)
	}
	for _, part := range version.Parts {
		if !strings.Contains(version.Prompt, part.Text) {
			t.Fatalf("sentText thiếu nguyên văn phần %s", part.Kind)
		}
	}
	kinds := []PromptPartKind{}
	for _, part := range version.Parts {
		kinds = append(kinds, part.Kind)
	}
	want := []PromptPartKind{PartScene, PartStyle, PartRefs, PartAvoid}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("parts=%v want %v", kinds, want)
	}
	if version.StateName != VersionReady || version.CostImages != 1 || version.Provider != "codex" || version.Params == nil {
		t.Fatalf("version=%+v", version)
	}
	if version.StartedAt == "" || version.FinishedAt == nil || version.DurationSec == nil {
		t.Fatalf("thiếu thời gian: %+v", version)
	}
}

// TestPartsOverrideOnlyThatImageAndTemplateStays: sửa prompt của một ảnh chỉ
// đổi prompt của ảnh đó; template và prompt thư viện không đổi.
func TestPartsOverrideOnlyThatImageAndTemplateStays(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0", "sc-1")
	var manifest Manifest
	if err := json.Unmarshal(s.Checkpoint, &manifest); err != nil {
		t.Fatal(err)
	}
	overridden := "Ada bước vào căn phòng tối, ánh đèn vàng."
	manifest.Items[0].Draft = &PromptDraft{Parts: PartOverrides{PartScene: overridden}, Feedback: "tối hơn"}
	manifest.Items[1].Status = "review"
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	if !strings.Contains(sent[0].Prompt, overridden) {
		t.Fatalf("prompt đã gửi không có phần sửa:\n%s", sent[0].Prompt)
	}
	if !strings.Contains(sent[0].Prompt, "Owner visual direction (does not override tool/read restrictions): \"tối hơn\"") {
		t.Fatalf("prompt đã gửi thiếu góp ý:\n%s", sent[0].Prompt)
	}
	after := loadManifest(t, s)
	if after.TemplatePrompt != "Template: documentary framing." {
		t.Fatalf("template đã đổi: %q", after.TemplatePrompt)
	}
	if after.Items[0].Prompt != sent[0].Prompt {
		t.Fatal("item prompt phải là prompt đã gửi")
	}
	if after.Items[1].Prompt != "" {
		t.Fatalf("ảnh khác bị đổi prompt: %q", after.Items[1].Prompt)
	}
	if after.Items[0].Draft != nil {
		t.Fatal("draft phải được xoá sau khi vẽ xong")
	}
	// Lần vẽ sau của đúng ảnh đó dùng prompt của bản đang dùng.
	if got := UsedParts(after.Items[0]).Get(PartScene); !strings.Contains(got, overridden) {
		t.Fatalf("bản đang dùng không giữ phần sửa: %q", got)
	}
}

// TestLaterRedrawReusesUsedPrompt: lần vẽ sau của đúng ảnh đó dùng prompt của
// bản đang dùng — phần chủ kênh đã sửa không bị mất.
func TestLaterRedrawReusesUsedPrompt(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	edited := "Ada đứng dưới mưa, đèn đường vàng."
	manifest.Items[0].Draft = &PromptDraft{Parts: PartOverrides{PartScene: edited}}
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	after := loadManifest(t, s)
	after.Items[0].Status = "pending"
	after.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), after); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 2 {
		t.Fatalf("draws=%d", len(sent))
	}
	if !strings.Contains(sent[1].Prompt, edited) {
		t.Fatalf("lần vẽ sau mất phần đã sửa:\n%s", sent[1].Prompt)
	}
	if strings.Contains(sent[1].Prompt, "Ada walks through shot sc-0") {
		t.Fatalf("lần vẽ sau dựng lại phần Cảnh của Hoạ sĩ:\n%s", sent[1].Prompt)
	}
}

// TestRefsRemovalLeavesOneReferenceSent: bỏ một ảnh tham chiếu thì lần vẽ chỉ
// gửi đúng một ảnh, và phiên bản ghi đúng ảnh đã gửi.
func TestRefsRemovalLeavesOneReferenceSent(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0", "sc-1")
	// Cảnh đầu đã có bản đang dùng để làm ảnh tham chiếu "Nhân vật".
	markUsed(t, s, m, "sc-1", Version{Version: 1, Path: "images/versions/sc-1/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	manifest := loadManifest(t, s)
	scene := sceneItem(t, manifest, "sc-0")
	manifest.Items[0].Draft = &PromptDraft{Refs: &[]ImageRef{{
		ID: "rival:sc-0", Role: RefLayout, Path: scene.Reference.Path, SHA256: scene.Reference.SHA256,
		Source: RefSource{Type: "rival_frame", Label: "Khung đối thủ · 00:00"},
	}}}
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 || len(sent[0].References) != 1 {
		t.Fatalf("ảnh tham chiếu gửi=%v", sent[0].References)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if len(version.Refs) != 1 || version.Refs[0].Role != RefLayout {
		t.Fatalf("refs ghi=%+v", version.Refs)
	}
	if !strings.Contains(version.Prompt, "1 of 1 listed image(s)") {
		t.Fatalf("prompt không nói đúng số ảnh gửi:\n%s", version.Prompt)
	}
}

// TestDefaultRefsSendSceneKeyframeAndFirstSceneImage: mặc định gửi khung nguồn
// của cảnh và ảnh đang dùng của cảnh đầu.
func TestDefaultRefsSendSceneKeyframeAndFirstSceneImage(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0", "sc-1")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	manifest := loadManifest(t, s)
	manifest.QueueOnly = []string{"sc-1"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 || len(sent[0].References) != 2 {
		t.Fatalf("mặc định phải gửi 2 ảnh tham chiếu: %v", sent)
	}
	version := loadManifest(t, s).Items[1].Versions[0]
	if len(version.Refs) != 2 || version.Refs[0].Source.Type != "rival_frame" || version.Refs[1].Source.Type != "video_image" {
		t.Fatalf("refs=%+v", version.Refs)
	}
	if version.Refs[1].Source.ImageID != "sc-0" {
		t.Fatalf("ảnh tham chiếu nhân vật phải từ cảnh đầu: %+v", version.Refs[1])
	}
}

// TestCancelBeforeSendCostsNoImage: huỷ trước khi gửi nhà cung cấp thì không
// tốn ảnh, phiên bản là cancelled, bản đang dùng giữ nguyên.
func TestCancelBeforeSendCostsNoImage(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	if err := s.SaveCheckpoint(context.Background(), loadManifest(t, s)); err != nil {
		t.Fatal(err)
	}
	cancels := NewCancelRegistry()
	cancels.Cancel(DrawKey("job-1", "sc-0", 2))
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 }
		d.Cancels = cancels
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(drawer.sent()) != 0 {
		t.Fatal("đã gửi nhà cung cấp dù bị huỷ trước khi gửi")
	}
	after := loadManifest(t, s)
	item := after.Items[0]
	if len(item.Versions) != 2 {
		t.Fatalf("versions=%+v", item.Versions)
	}
	cancelled := item.Versions[1]
	if cancelled.State() != VersionCancelled || cancelled.CostImages != 0 || cancelled.Version != 2 {
		t.Fatalf("phiên bản huỷ=%+v", cancelled)
	}
	if item.CurrentVersion == nil || *item.CurrentVersion != 1 {
		t.Fatalf("bản đang dùng đổi: %+v", item.CurrentVersion)
	}
	if item.Status != "approved" {
		t.Fatalf("trạng thái ảnh=%q", item.Status)
	}
}

// TestQuotaFailureCarriesProviderRetryAt: lỗi hạn mức ghi đúng lý do và giờ
// đặt lại thật của nhà cung cấp.
func TestQuotaFailureCarriesProviderRetryAt(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	reset := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	drawer := &recorderDrawer{err: &imagegen.DrawError{Code: imagegen.ErrCodeQuota, Message: "hết hạn mức", RetryAt: &reset}}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	// Hết hạn mức là bước chờ (parked), không phải bước lỗi.
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if version.State() != VersionFailed || version.VersionError == nil {
		t.Fatalf("version=%+v", version)
	}
	if version.VersionError.Reason != "quota" || version.VersionError.RetryAt == nil {
		t.Fatalf("lỗi=%+v", version.VersionError)
	}
	if *version.VersionError.RetryAt != reset.Format(time.RFC3339) {
		t.Fatalf("retryAt=%q want %q", *version.VersionError.RetryAt, reset.Format(time.RFC3339))
	}
	// Lỗi giữ nguyên phần prompt và ảnh tham chiếu để "Sửa prompt" mở lại.
	if len(version.Parts) == 0 || version.Prompt == "" {
		t.Fatalf("mất prompt đã gửi: %+v", version)
	}
}

// TestCapFailureWithoutProviderResetUsesWorkerMidnight: trần ngày của worker
// không có giờ nhà cung cấp thì hẹn nửa đêm.
func TestCapFailureWithoutProviderResetUsesWorkerMidnight(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{err: &imagegen.CapError{Provider: "codex", Used: 50, Limit: 50}}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	if version.VersionError == nil || version.VersionError.Reason != "quota" || version.VersionError.RetryAt == nil {
		t.Fatalf("lỗi=%+v", version.VersionError)
	}
	if version.CostImages != 0 {
		t.Fatalf("trần ngày không tốn ảnh: %d", version.CostImages)
	}
}

// TestProgressEventsFollowProviderPhases: SSE image.progress đi đúng thứ tự
// sending → drawing → receiving → done.
func TestProgressEventsFollowProviderPhases(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	var events []string
	s.PublishEvent = func(name string, data any) {
		if name != "image.progress" {
			t.Fatalf("event=%q", name)
		}
		payload, _ := json.Marshal(data)
		var decoded struct {
			JobID   string `json:"jobId"`
			ImageID string `json:"imageId"`
			N       int    `json:"n"`
			Phase   string `json:"phase"`
		}
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.JobID != "job-1" || decoded.ImageID != "sc-0" || decoded.N != 1 {
			t.Fatalf("payload=%s", payload)
		}
		events = append(events, decoded.Phase)
	}
	phases := &events
	drawer := &recorderDrawer{phases: phases}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	want := []string{"sending", "drawing", "receiving", "done"}
	if fmt.Sprint(events) != fmt.Sprint(want) {
		t.Fatalf("events=%v want %v", events, want)
	}
}

// TestRedrawKeepsBothVersionFilesPublished: mọi phiên bản đã vẽ được công bố
// (nên GC giữ tệp cũ khi còn trong lịch sử của video).
func TestRedrawKeepsBothVersionFilesPublished(t *testing.T) {
	s, m, manifest, _ := sceneFixture(t, "sc-0")
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	after := loadManifest(t, s)
	after.Items[0].Status = "pending"
	after.Items[0].Direction = "sáng hơn"
	after.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), after); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	final := loadManifest(t, s)
	if len(final.Items[0].Versions) != 2 {
		t.Fatalf("versions=%d", len(final.Items[0].Versions))
	}
	for _, version := range final.Items[0].Versions {
		if _, err := os.Stat(m.Root + "/" + version.Path); err != nil {
			t.Fatalf("tệp của bản %d không còn: %v", version.Version, err)
		}
	}
	if final.Items[0].Versions[1].DiffFromPrev == nil {
		t.Fatal("bản mới phải có so sánh phần Cảnh với bản trước")
	}
}

// TestBuiltVideoUsesNewVersionAndNeedsReview: video đã dựng thì bản mới thành
// bản đang dùng, ảnh được đánh dấu cần duyệt lại và media của clip được thay.
func TestBuiltVideoUsesNewVersionAndNeedsReview(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	if err := s.SaveCheckpoint(context.Background(), loadManifest(t, s)); err != nil {
		t.Fatal(err)
	}
	var rebound []RebindRequest
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 }
		d.Built = func(context.Context, domain.CloneJob) (string, bool, error) { return "clone-job-1-g1", true, nil }
		d.Rebind = func(_ context.Context, _ domain.CloneJob, req RebindRequest) error {
			rebound = append(rebound, req)
			return nil
		}
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	after := loadManifest(t, s)
	item := after.Items[0]
	if item.CurrentVersion == nil || *item.CurrentVersion != 2 {
		t.Fatalf("bản mới không thành bản đang dùng: %+v", item.CurrentVersion)
	}
	if !item.NeedsReview {
		t.Fatal("ảnh mới phải được đánh dấu cần duyệt lại")
	}
	if len(rebound) != 1 || rebound[0].SceneID != "sc-0" || rebound[0].Version != 2 {
		t.Fatalf("rebind=%+v", rebound)
	}
	if item.RebindError != "" {
		t.Fatalf("rebindError=%q", item.RebindError)
	}
}

// TestUnbuiltVideoLeavesVersionForReview: video chưa dựng thì bản mới chỉ là
// bản chờ duyệt, không tự thay bản đang dùng.
func TestUnbuiltVideoLeavesVersionForReview(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) { d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 } })
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	item := loadManifest(t, s).Items[0]
	if item.CurrentVersion != nil || item.ReviewVersion == nil || *item.ReviewVersion != 1 || item.Status != "review" {
		t.Fatalf("item=%+v", item)
	}
	if item.NeedsReview {
		t.Fatal("video chưa dựng thì không cần duyệt lại")
	}
}

// selectedSourceFixture dựng một cảnh có bản đang dùng (1) và bản chủ kênh
// đang xem (2) khác hẳn prompt/ảnh tham chiếu, kèm tệp tham chiếu của cả hai
// để lỗi chọn nhầm nguồn lộ ra.
func selectedSourceFixture(t *testing.T) (*pipeline.StepContext, *stepstest.FakeMedia, Manifest) {
	t.Helper()
	s, media, manifest, _ := sceneFixture(t, "sc-0")
	item := &manifest.Items[0]
	item.Status = "approved"
	one := 1
	item.CurrentVersion = &one
	fingerprint := SourceFingerprint(*item)
	usedRef := ImageRef{ID: "video:used", Role: RefLayout, Path: "images/versions/sc-0/1-used-ref.webp", SHA256: "sha-used", Source: RefSource{Type: "video_image", Label: "Used ref", ImageID: "used"}}
	selectedA := ImageRef{ID: "video:selected-a", Role: RefCharacter, Path: "images/versions/sc-0/2-ref-a.webp", SHA256: "sha-a", Source: RefSource{Type: "video_image", Label: "Selected ref A", ImageID: "selected-a"}}
	selectedB := ImageRef{ID: "video:selected-b", Role: RefStyle, Path: "images/versions/sc-0/2-ref-b.webp", SHA256: "sha-b", Source: RefSource{Type: "video_image", Label: "Selected ref B", ImageID: "selected-b"}}
	item.Versions = []Version{
		{Version: 1, Path: "images/versions/sc-0/1-used.webp", SHA256: "sha-used", StateName: VersionReady, SourceFingerprint: fingerprint, Parts: PromptParts{
			{Kind: PartScene, Text: "USED SCENE ONE", Source: PromptPartSource{Type: "artist", Label: "Hoạ sĩ"}},
			{Kind: PartStyle, Text: "USED STYLE ONE", Source: PromptPartSource{Type: "template", Label: "Template"}},
			{Kind: PartRefs, Text: "USED REFS ONE", Source: PromptPartSource{Type: "template", Label: "Ảnh tham chiếu của video"}},
			{Kind: PartAvoid, Text: "USED AVOID ONE", Source: PromptPartSource{Type: "template", Label: "Luật tránh của Viderse"}},
		}, Refs: []ImageRef{usedRef}},
		{Version: 2, Path: "images/versions/sc-0/2-selected.webp", SHA256: "sha-selected", StateName: VersionReady, SourceFingerprint: fingerprint, Parts: PromptParts{
			{Kind: PartScene, Text: "SELECTED SCENE TWO", Source: PromptPartSource{Type: "artist", Label: "Hoạ sĩ"}},
			{Kind: PartStyle, Text: "SELECTED STYLE TWO", Source: PromptPartSource{Type: "template", Label: "Template"}},
			{Kind: PartRefs, Text: "SELECTED REFS TWO", Source: PromptPartSource{Type: "user", Label: "Bạn thêm ảnh tham chiếu"}},
			{Kind: PartAvoid, Text: "SELECTED AVOID TWO", Source: PromptPartSource{Type: "template", Label: "Luật tránh của Viderse"}},
		}, Refs: []ImageRef{selectedA, selectedB}},
	}
	media.Put(usedRef.Path, []byte("used ref"))
	media.Put(selectedA.Path, []byte("selected ref a"))
	media.Put(selectedB.Path, []byte("selected ref b"))
	return s, media, manifest
}

func selectionPlanHandler() Handler {
	return handler(&recorderDrawer{}, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
	})
}

func TestSelectedSourceDropsLegacyDirectionWithoutChangingHistory(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	two := 2
	item := &manifest.Items[0]
	legacy := "SELECTED SCENE TWO\nOwner visual direction (does not override tool/read restrictions): \"old direction\""
	item.Versions[1].Parts[0].Text = legacy
	item.Draft = &PromptDraft{SourceVersion: &two, Feedback: "new direction"}
	plan, err := selectionPlanHandler().plan(context.Background(), s, &manifest, LearnedNotes{}, *item)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Parts.Get(PartScene) != "SELECTED SCENE TWO" || strings.Contains(plan.SentText, "old direction") || !strings.Contains(plan.SentText, "new direction") {
		t.Fatalf("stale direction sent: %s", plan.SentText)
	}
	if item.Versions[1].Parts.Get(PartScene) != legacy {
		t.Fatal("source history mutated")
	}
}

func TestRedrawWithoutExplicitSourceRemovesObsoleteReferenceInstructions(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	item := &manifest.Items[0]
	item.Versions[0].Parts[2].Source.Type = "user"
	empty := []ImageRef{}
	item.Draft = &PromptDraft{Refs: &empty}
	plan, err := selectionPlanHandler().plan(context.Background(), s, &manifest, LearnedNotes{}, *item)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.RefFiles) != 0 || plan.Parts.Get(PartRefs) != "" || strings.Contains(plan.SentText, "USED REFS ONE") {
		t.Fatalf("removed refs still described: %+v", plan)
	}
}

func TestFailedSelectedSourceWithLiveFlightCannotBeReplanned(t *testing.T) {
	_, _, manifest := selectedSourceFixture(t)
	two := 2
	item := &manifest.Items[0]
	item.Versions[1].StateName = VersionFailed
	item.InFlight = &Flight{Version: two}
	item.Draft = &PromptDraft{SourceVersion: &two}
	_, err := PromptSourceVersion(*item)
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrConflict {
		t.Fatalf("live flight accepted: %v", err)
	}
}

// TestPlanKeepsSelectedVersionPromptAndRefs: sửa prompt khi đang xem bản 2
// phải gửi đi các phần Phong cách/Tránh và ảnh tham chiếu của đúng bản 2, kể
// cả sau khi manifest được lưu/đọc lại và bản đang dùng đổi — không lấy bản 1.
func TestPlanKeepsSelectedVersionPromptAndRefs(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	two := 2
	manifest.Items[0].Draft = &PromptDraft{Provider: "codex", SourceVersion: &two, Parts: PartOverrides{PartScene: "EDITED SELECTED SCENE"}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var loaded Manifest
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Items[0].Draft == nil || loaded.Items[0].Draft.SourceVersion == nil || *loaded.Items[0].Draft.SourceVersion != 2 {
		t.Fatalf("bản nguồn không sống qua checkpoint: %+v", loaded.Items[0].Draft)
	}
	h := selectionPlanHandler()
	for _, used := range []int{1, 2} {
		n := used
		loaded.Items[0].CurrentVersion = &n
		plan, err := h.plan(context.Background(), s, &loaded, LearnedNotes{}, loaded.Items[0])
		if err != nil {
			t.Fatalf("used=%d: %v", used, err)
		}
		if got := plan.Parts.Get(PartScene); got != "EDITED SELECTED SCENE" {
			t.Fatalf("used=%d scene=%q", used, got)
		}
		if got := plan.Parts.Get(PartStyle); got != "SELECTED STYLE TWO" {
			t.Fatalf("used=%d style=%q", used, got)
		}
		if got := plan.Parts.Get(PartAvoid); got != "SELECTED AVOID TWO" {
			t.Fatalf("used=%d avoid=%q", used, got)
		}
		if len(plan.Refs) != 2 || plan.Refs[0].ID != "video:selected-a" || plan.Refs[1].ID != "video:selected-b" {
			t.Fatalf("used=%d refs=%+v", used, plan.Refs)
		}
		if refsText := plan.Parts.Get(PartRefs); refsText != loaded.Items[0].Versions[1].Parts.Get(PartRefs) {
			t.Fatalf("used=%d unchanged refs instructions were lost: %q", used, refsText)
		}
		if strings.Contains(plan.SentText, "USED STYLE ONE") || strings.Contains(plan.SentText, "USED AVOID ONE") || strings.Contains(plan.SentText, "USED REFS ONE") || strings.Contains(plan.SentText, "Used ref") {
			t.Fatalf("used=%d prompt lẫn bản đang dùng:\n%s", used, plan.SentText)
		}
		if plan.Wrapper.Provider != "codex" || len(plan.Wrapper.Tool) == 0 {
			t.Fatalf("used=%d wrapper mất chỉ dẫn công cụ: %+v", used, plan.Wrapper)
		}
		if loaded.Items[0].CurrentVersion == nil || *loaded.Items[0].CurrentVersion != used {
			t.Fatalf("used=%d current bị đổi: %+v", used, loaded.Items[0].CurrentVersion)
		}
	}
}

func TestSelectedSourceKeepsEditedRefInstructionsWhenAttachmentsUnchanged(t *testing.T) {
	for _, explicitSource := range []bool{false, true} {
		for _, explicitRefs := range []bool{false, true} {
			s, _, manifest := selectedSourceFixture(t)
			two := 2
			item := &manifest.Items[0]
			item.Draft = &PromptDraft{SourceVersion: &two, Parts: PartOverrides{PartRefs: "Keep the selected face, but use only its lighting."}}
			if !explicitSource {
				item.CurrentVersion = &two
				item.Draft.SourceVersion = nil
			}
			if explicitRefs {
				refs := append([]ImageRef(nil), item.Versions[1].Refs...)
				item.Draft.Refs = &refs
			}
			plan, err := selectionPlanHandler().plan(context.Background(), s, &manifest, LearnedNotes{}, *item)
			if err != nil {
				t.Fatal(err)
			}
			if got := plan.Parts.Get(PartRefs); got != item.Draft.Parts[PartRefs] {
				t.Fatalf("explicitRefs=%v lost edited instructions: %q", explicitRefs, got)
			}
			if len(plan.Refs) != 2 || plan.Refs[0].ID != "video:selected-a" || plan.Refs[1].ID != "video:selected-b" {
				t.Fatalf("explicitRefs=%v changed attachments: %+v", explicitRefs, plan.Refs)
			}
		}
	}
}

func TestSelectedSourceDoesNotSilentlyDropRefsAfterProviderLimitChanges(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	two := 2
	manifest.Items[0].Draft = &PromptDraft{SourceVersion: &two}
	h := handler(&recorderDrawer{}, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 1 }
	})
	_, err := h.plan(context.Background(), s, &manifest, LearnedNotes{}, manifest.Items[0])
	var invalid *domain.CloneError
	if !errors.As(err, &invalid) || invalid.Code != domain.CloneErrInvalidRequest {
		t.Fatalf("selected refs were silently truncated: %v", err)
	}
}

// TestPlanSelectedVersionExplicitEmptyRefsStaysEmpty: bỏ hết ảnh tham chiếu
// của bản 2 gửi đi đúng 0 ảnh và prompt không còn phần Tham chiếu.
func TestPlanSelectedVersionExplicitEmptyRefsStaysEmpty(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	two := 2
	empty := []ImageRef{}
	manifest.Items[0].Draft = &PromptDraft{SourceVersion: &two, Refs: &empty}
	h := selectionPlanHandler()
	plan, err := h.plan(context.Background(), s, &manifest, LearnedNotes{}, manifest.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refs) != 0 || len(plan.RefFiles) != 0 {
		t.Fatalf("refs=%+v files=%v", plan.Refs, plan.RefFiles)
	}
	if got := plan.Parts.Get(PartRefs); got != "" {
		t.Fatalf("prompt vẫn mô tả ảnh tham chiếu đã bỏ: %q", got)
	}
	if strings.Contains(plan.SentText, "Selected ref") || strings.Contains(plan.SentText, "USED REFS ONE") {
		t.Fatalf("sentText còn nhắc ảnh tham chiếu:\n%s", plan.SentText)
	}
}

// TestPlanWithoutSelectedSourceKeepsUsedFlow: yêu cầu không có baseVersion giữ
// nguyên cách cũ — ảnh tham chiếu của bản đang dùng, phần Hoạ sĩ dựng lại.
func TestPlanWithoutSelectedSourceKeepsUsedFlow(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	manifest.Items[0].Draft = &PromptDraft{Parts: PartOverrides{PartScene: "EDITED USED FLOW"}}
	h := selectionPlanHandler()
	plan, err := h.plan(context.Background(), s, &manifest, LearnedNotes{}, manifest.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Parts.Get(PartScene); got != "EDITED USED FLOW" {
		t.Fatalf("scene=%q", got)
	}
	if got := plan.Parts.Get(PartStyle); got == "USED STYLE ONE" {
		t.Fatalf("không có bản nguồn mà lại chép phần Phong cách bản cũ: %q", got)
	}
	if len(plan.Refs) != 1 || plan.Refs[0].ID != "video:used" {
		t.Fatalf("refs=%+v", plan.Refs)
	}
}

// TestPlanRejectsUnusableSelectedSource: bản nguồn mất/đang vẽ/không khớp nguồn
// là lỗi, không lặng lẽ quay về bản đang dùng.
func TestPlanRejectsUnusableSelectedSource(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Item)
		code   string
	}{
		{name: "missing", mutate: func(i *Item) { n := 9; i.Draft.SourceVersion = &n }, code: domain.CloneErrNotFound},
		{name: "zero", mutate: func(i *Item) { n := 0; i.Draft.SourceVersion = &n }, code: domain.CloneErrInvalidRequest},
		{name: "drawing", mutate: func(i *Item) { i.Versions[1].StateName = VersionDrawing }, code: domain.CloneErrConflict},
		{name: "stale", mutate: func(i *Item) { i.Versions[1].SourceFingerprint = "other-source" }, code: domain.CloneErrStale},
		{name: "cancelled", mutate: func(i *Item) { i.Versions[1].StateName = VersionCancelled }, code: domain.CloneErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, _, manifest := selectedSourceFixture(t)
			two := 2
			manifest.Items[0].Draft = &PromptDraft{SourceVersion: &two}
			test.mutate(&manifest.Items[0])
			_, err := selectionPlanHandler().plan(context.Background(), s, &manifest, LearnedNotes{}, manifest.Items[0])
			var cloneErr *domain.CloneError
			if !errors.As(err, &cloneErr) || cloneErr.Code != test.code {
				t.Fatalf("err=%v want code %s", err, test.code)
			}
		})
	}
}

// TestRunSelectedSourceSendsSelectedVersionPrompt: chạy bước vẽ thật với
// baseVersion=2 gửi nhà cung cấp đúng prompt/ảnh tham chiếu của bản 2, không
// phải bản đang dùng 1; bản 3 lưu lại giữ nguyên style/avoid của bản 2 và bản
// đang dùng vẫn là 1 (video chưa dựng).
func TestRunSelectedSourceSendsSelectedVersionPrompt(t *testing.T) {
	s, _, manifest := selectedSourceFixture(t)
	item := &manifest.Items[0]
	item.Status = "pending"
	two := 2
	item.Draft = &PromptDraft{Provider: "codex", SourceVersion: &two, Parts: PartOverrides{PartScene: "EDITED SELECTED SCENE"}}
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	prompt := sent[0].Prompt
	for _, want := range []string{"EDITED SELECTED SCENE", "SELECTED STYLE TWO", "SELECTED AVOID TWO", "SELECTED REFS TWO"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt thiếu %q:\n%s", want, prompt)
		}
	}
	for _, unwanted := range []string{"USED STYLE ONE", "USED AVOID ONE", "USED REFS ONE", "Used ref"} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("prompt lẫn %q:\n%s", unwanted, prompt)
		}
	}
	if len(sent[0].References) != 2 {
		t.Fatalf("references=%v", sent[0].References)
	}
	after := loadManifest(t, s)
	if after.Items[0].CurrentVersion == nil || *after.Items[0].CurrentVersion != 1 {
		t.Fatalf("bản đang dùng bị đổi: %+v", after.Items[0].CurrentVersion)
	}
	var drawnVersion *Version
	for index := range after.Items[0].Versions {
		if after.Items[0].Versions[index].Version == 3 {
			drawnVersion = &after.Items[0].Versions[index]
		}
	}
	if drawnVersion == nil {
		t.Fatalf("không có bản 3: %+v", after.Items[0].Versions)
	}
	if got := drawnVersion.Parts.Get(PartStyle); got != "SELECTED STYLE TWO" {
		t.Fatalf("bản 3 style=%q", got)
	}
	if got := drawnVersion.Parts.Get(PartAvoid); got != "SELECTED AVOID TWO" {
		t.Fatalf("bản 3 avoid=%q", got)
	}
	if got := drawnVersion.Parts.Get(PartScene); got != "EDITED SELECTED SCENE" {
		t.Fatalf("bản 3 scene=%q", got)
	}
}
