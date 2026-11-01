package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
)

// holdDrawer giữ lần vẽ ở giữa chừng để test đọc manifest đúng lúc đang vẽ.
type holdDrawer struct {
	ctx     *pipeline.StepContext
	started chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	mid     Manifest
	reqs    []imagegen.Request
}

func (d *holdDrawer) Draw(_ context.Context, r imagegen.Request) (imagegen.Receipt, error) {
	d.mu.Lock()
	d.reqs = append(d.reqs, r)
	d.mu.Unlock()
	// Đọc checkpoint ngay trong lúc handler đang đứng trong Draw: an toàn và
	// đúng thứ tự (handler không ghi trong lúc này).
	var mid Manifest
	if err := json.Unmarshal(d.ctx.Checkpoint, &mid); err != nil {
		return imagegen.Receipt{}, err
	}
	d.mu.Lock()
	d.mid = mid
	d.mu.Unlock()
	d.once.Do(func() { close(d.started) })
	<-d.release
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a5XcAAAAASUVORK5CYII=")
	if err := os.WriteFile(r.Output, data, 0600); err != nil {
		return imagegen.Receipt{}, err
	}
	hash, _ := steps.SHA256File(r.Output)
	return imagegen.Receipt{Provider: "codex", Model: imagegen.CodexImageModel, PngSha256: hash}, nil
}

func (d *holdDrawer) snapshot() Manifest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mid
}

// TestDrawingVersionIsWrittenWhileTheDrawRuns: từ lúc gửi, ảnh có dòng phiên
// bản `drawing` với đúng phần prompt, ảnh tham chiếu, nguồn vẽ và giờ bắt đầu;
// bản đang dùng giữ nguyên tới khi xong.
func TestDrawingVersionIsWrittenWhileTheDrawRuns(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	manifest := loadManifest(t, s)
	manifest.Items[0].Status = "pending"
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &holdDrawer{ctx: s, started: make(chan struct{}), release: make(chan struct{})}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
		d.PromptProvider = func(context.Context, domain.CloneJob, string) string { return "codex" }
	})
	done := make(chan error, 1)
	go func() { done <- func() error { _, err := h.Run(context.Background(), s); return err }() }()
	<-drawer.started
	mid := drawer.snapshot()
	item := mid.Items[0]
	drawing := item.Versions[len(item.Versions)-1]
	if drawing.State() != VersionDrawing || drawing.Version != 2 {
		t.Fatalf("giữa lúc vẽ phải có dòng drawing bản 2: %+v", drawing)
	}
	if drawing.Prompt == "" || len(drawing.Parts) == 0 || len(drawing.Wrapper.Guard) == 0 || len(drawing.Refs) == 0 {
		t.Fatalf("dòng drawing thiếu phần đã gửi: %+v", drawing)
	}
	if drawing.Provider != "codex" || drawing.StartedAt == "" || drawing.CostImages != 0 || drawing.Params == nil {
		t.Fatalf("dòng drawing thiếu nguồn/giờ/giá: %+v", drawing)
	}
	if item.CurrentVersion == nil || *item.CurrentVersion != 1 {
		t.Fatalf("bản đang dùng đổi trước khi vẽ xong: %+v", item.CurrentVersion)
	}
	if item.InFlight == nil || item.InFlight.Version != 2 {
		t.Fatalf("con trỏ chuyến vẽ=%+v", item.InFlight)
	}
	close(drawer.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after := loadManifest(t, s).Items[0]
	if len(after.Versions) != 2 {
		t.Fatalf("phải có đúng 2 bản, không nhân đôi dòng drawing: %+v", after.Versions)
	}
	if after.Versions[1].State() != VersionReady || after.Versions[1].Version != 2 {
		t.Fatalf("bản 2 phải ready sau khi xong: %+v", after.Versions[1])
	}
	// Video chưa dựng: bản mới chờ duyệt, bản đang dùng vẫn là bản 1 tới khi
	// chủ kênh duyệt (07 §26).
	if after.CurrentVersion == nil || *after.CurrentVersion != 1 {
		t.Fatalf("bản đang dùng phải giữ bản 1: %+v", after.CurrentVersion)
	}
	if after.ReviewVersion == nil || *after.ReviewVersion != 2 || after.Status != "review" {
		t.Fatalf("bản 2 phải chờ duyệt: %+v %q", after.ReviewVersion, after.Status)
	}
	if after.InFlight != nil {
		t.Fatal("con trỏ chuyến vẽ phải được xoá")
	}
}

// TestOrphanedDrawingVersionBecomesFailed: dòng `drawing` không còn chuyến vẽ
// (tiến trình chết, job dừng) thành failed với lý do thật, không vẽ lại tốn ảnh.
func TestOrphanedDrawingVersionBecomesFailed(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	manifest := loadManifest(t, s)
	manifest.Items[0].Status = "running"
	manifest.Items[0].Versions = append(manifest.Items[0].Versions, Version{
		Version: 2, StateName: VersionDrawing, StartedAt: "2026-11-01T10:00:00Z",
		Parts: PromptParts{{Kind: PartScene, Text: "Ada in shot sc-0"}}, Prompt: "Ada in shot sc-0",
	})
	manifest.QueueOnly = nil
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	clock := time.Date(2026, 11, 1, 10, 0, 30, 0, time.UTC)
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 }
		d.Now = func() time.Time { return clock }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if len(drawer.sent()) != 0 {
		t.Fatal("dòng mồ côi không được vẽ lại tốn ảnh")
	}
	item := loadManifest(t, s).Items[0]
	orphan := item.Versions[1]
	if orphan.State() != VersionFailed || orphan.VersionError == nil || orphan.VersionError.Reason != "provider" {
		t.Fatalf("dòng mồ côi phải thành failed kèm lý do: %+v", orphan)
	}
	if orphan.FinishedAt == nil || orphan.DurationSec == nil || *orphan.DurationSec != 30 {
		t.Fatalf("dòng mồ côi thiếu giờ kết thúc/thời lượng: %+v", orphan)
	}
	if item.Status != "approved" {
		t.Fatalf("trạng thái ảnh=%q, phải về đúng bản đang dùng", item.Status)
	}
	if item.CurrentVersion == nil || *item.CurrentVersion != 1 {
		t.Fatalf("bản đang dùng đổi: %+v", item.CurrentVersion)
	}
}

// TestDrawingVersionResumesAfterRestart: chuyến vẽ còn con trỏ InFlight được
// chạy tiếp đúng prompt đã chốt, dùng lại số bản — không sinh bản thứ ba.
func TestDrawingVersionResumesAfterRestart(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0")
	manifest := loadManifest(t, s)
	item := &manifest.Items[0]
	item.Status = "running"
	item.InFlight = &Flight{
		GenerationID: "gen-restart", Version: 1, Prompt: "PROMPT ĐÃ CHỐT TRƯỚC KHI CHẾT", PromptVersion: item.PromptVersion,
		SourceFingerprint: SourceFingerprint(*item), PNG: "images/flights/sc-0/gen-restart.png",
		Parts:    PromptParts{{Kind: PartScene, Text: "PROMPT ĐÃ CHỐT TRƯỚC KHI CHẾT"}},
		Provider: "codex", StartedAt: "2026-11-01T10:00:00Z", BaseStatus: "pending",
	}
	item.Versions = []Version{{Version: 1, StateName: VersionDrawing, StartedAt: item.InFlight.StartedAt, Parts: item.InFlight.Parts, Prompt: item.InFlight.Prompt}}
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 }
		d.PromptProvider = func(context.Context, domain.CloneJob, string) string { return "codex" }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	sent := drawer.sent()
	if len(sent) != 1 || sent[0].Prompt != "PROMPT ĐÃ CHỐT TRƯỚC KHI CHẾT" {
		t.Fatalf("chạy tiếp phải dùng đúng prompt đã chốt: %+v", sent)
	}
	after := loadManifest(t, s).Items[0]
	if len(after.Versions) != 1 || after.Versions[0].State() != VersionReady {
		t.Fatalf("bản vẽ tiếp phải thay dòng drawing, không nhân đôi: %+v", after.Versions)
	}
	if after.Versions[0].Provider != "codex" {
		t.Fatalf("bản vẽ tiếp thiếu nguồn vẽ: %+v", after.Versions[0])
	}
}

// TestWrapperAndPartsJoinToSentText: nguyên văn gửi = hàng rào + các phần +
// chỉ dẫn công cụ, và wrapper ghi đúng nguồn vẽ của phần không sửa được.
func TestWrapperAndPartsJoinToSentText(t *testing.T) {
	s, _, _, _ := sceneFixture(t, "sc-0")
	drawer := &recorderDrawer{}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 4 }
		d.PromptProvider = func(context.Context, domain.CloneJob, string) string { return "codex" }
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	version := loadManifest(t, s).Items[0].Versions[0]
	sent := drawer.sent()
	if len(sent) != 1 {
		t.Fatalf("draws=%d", len(sent))
	}
	if joined := JoinPrompt(version.Wrapper, version.Parts); joined != version.Prompt {
		t.Fatalf("wrapper + parts không ghép ra sentText:\njoined=%q\nsent=%q", joined, version.Prompt)
	}
	if version.Prompt != sent[0].Prompt {
		t.Fatal("sentText khác chuỗi nhà cung cấp nhận")
	}
	if len(version.Wrapper.Guard) == 0 || len(version.Wrapper.Tool) == 0 || version.Wrapper.Provider != "codex" {
		t.Fatalf("wrapper Codex=%+v", version.Wrapper)
	}
	if !strings.HasPrefix(version.Prompt, version.Wrapper.Guard[0]) {
		t.Fatal("hàng rào an toàn phải đứng đầu nguyên văn")
	}
	if !strings.HasSuffix(version.Prompt, version.Wrapper.Tool[len(version.Wrapper.Tool)-1]) {
		t.Fatal("chỉ dẫn công cụ phải đứng cuối nguyên văn")
	}
}

// TestGrokWrapperHasNoToolInstructions: wrapper của Grok chỉ có hàng rào an
// toàn; phần không sửa được không nói tới công cụ của Codex.
func TestGrokWrapperHasNoToolInstructions(t *testing.T) {
	s, _, manifest, _ := sceneFixture(t, "sc-0")
	manifest.Items[0].Draft = &PromptDraft{Provider: "grok"}
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
	version := loadManifest(t, s).Items[0].Versions[0]
	if len(version.Wrapper.Guard) == 0 || len(version.Wrapper.Tool) != 0 || version.Wrapper.Provider != "" {
		t.Fatalf("wrapper Grok=%+v", version.Wrapper)
	}
	if fmt.Sprint(version.Parts.Text()) == "" {
		t.Fatal("mất các phần prompt")
	}
}

// cancelRacingDrawer huỷ lần vẽ đúng lúc nhà cung cấp chưa nhận yêu cầu (chưa
// có pha "sending"): lần vẽ đó không được tính ảnh.
type cancelRacingDrawer struct {
	cancels *CancelRegistry
	key     string
}

func (d *cancelRacingDrawer) Draw(ctx context.Context, _ imagegen.Request) (imagegen.Receipt, error) {
	d.cancels.Cancel(d.key)
	return imagegen.Receipt{}, context.Canceled
}

// TestCancelBeforeProviderSendCostsNoImage: Huỷ lần vẽ từ giao diện rơi đúng
// lúc chưa gửi thì bản ghi cancelled, costImages 0, bản đang dùng giữ nguyên.
func TestCancelBeforeProviderSendCostsNoImage(t *testing.T) {
	s, m, _, _ := sceneFixture(t, "sc-0")
	markUsed(t, s, m, "sc-0", Version{Version: 1, Path: "images/versions/sc-0/1-old.webp", SHA256: "sha-old", StateName: VersionReady})
	manifest := loadManifest(t, s)
	manifest.Items[0].Status = "pending"
	manifest.QueueOnly = []string{"sc-0"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	cancels := NewCancelRegistry()
	drawer := &cancelRacingDrawer{cancels: cancels, key: DrawKey("job-1", "sc-0", 2)}
	h := handler(drawer, func(d *Deps) {
		d.RefLimit = func(context.Context, domain.CloneJob, string) int { return 0 }
		d.Cancels = cancels
	})
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	item := loadManifest(t, s).Items[0]
	if len(item.Versions) != 2 {
		t.Fatalf("huỷ phải thay dòng drawing, không thêm bản: %+v", item.Versions)
	}
	cancelled := item.Versions[1]
	if cancelled.State() != VersionCancelled || cancelled.CostImages != 0 {
		t.Fatalf("bản bị huỷ=%+v", cancelled)
	}
	if item.CurrentVersion == nil || *item.CurrentVersion != 1 || item.Status != "approved" {
		t.Fatalf("bản đang dùng/trạng thái đổi: %+v %q", item.CurrentVersion, item.Status)
	}
	if item.InFlight != nil {
		t.Fatal("con trỏ chuyến vẽ phải được xoá sau khi huỷ")
	}
}
