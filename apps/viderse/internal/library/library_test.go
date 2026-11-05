package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ── khung test ─────────────────────────────────────────────────────────────

// tempFiles là Files dưới thư mục tạm của test.
type tempFiles struct{ root string }

func (f tempFiles) Root() string { return f.root }

func (f tempFiles) Resolve(rel string) (string, error) {
	return filepath.Join(f.root, filepath.FromSlash(rel)), nil
}

// fakeRunner trả kết quả ffprobe/ffmpeg cài sẵn.
type fakeRunner struct {
	probe  string
	frames []byte
	err    error
	calls  int
}

func (r *fakeRunner) Run(_ context.Context, _ string, args []string) (string, error) {
	r.calls++
	if r.err != nil {
		return "", r.err
	}
	for _, arg := range args {
		if arg == "-show_format" {
			return r.probe, nil
		}
	}
	return string(r.frames), nil
}

// fakeBins trả đường binary giả; ok cài sẵn.
type fakeBins struct{ ok bool }

func (b fakeBins) Tools(context.Context) (string, string, bool) {
	if !b.ok {
		return "", "", false
	}
	return "/usr/bin/ffmpeg", "/usr/bin/ffprobe", true
}

// fakePrice trả giá ảnh cài sẵn.
type fakePrice struct {
	usd float64
	ok  bool
	err error
}

func (p fakePrice) PerImageUSD(context.Context) (float64, bool, error) {
	return p.usd, p.ok, p.err
}

// testService dựng service trên memStore với id đếm tăng và giờ cố định.
func testService(t *testing.T, store Store, deps ...func(*Deps)) *Service {
	t.Helper()
	var counter int
	var mu sync.Mutex
	d := Deps{
		Store: store,
		Files: tempFiles{root: t.TempDir()},
		Now:   func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
		NewID: func() string {
			mu.Lock()
			defer mu.Unlock()
			counter++
			return fmt.Sprintf("00000000-0000-4000-8000-%012d", counter)
		},
	}
	for _, fn := range deps {
		fn(&d)
	}
	return New(d)
}

// seedSnow thêm dòng hiệu ứng tuyết (như seed thật đã chạy).
func seedSnow(store *memStore) domain.Overlay {
	overlay := domain.Overlay{
		ID: "99999999-9999-4999-8999-999999999999", SHA256: strings.Repeat("b", 64),
		Name: "Tuyết", FilePath: "library/overlays/snow-loop.mp4", Mime: "video/mp4",
		Bytes: 1024, Background: domain.BgBlack, Builtin: "snow",
		CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	store.overlays[overlay.ID] = overlay
	return overlay
}

// ── template ───────────────────────────────────────────────────────────────

func TestCreateTemplateUsesStandardRecipeAndSnow(t *testing.T) {
	store := newMemStore()
	snow := seedSnow(store)
	svc := testService(t, store)
	ctx := context.Background()

	tpl, err := svc.Create(ctx, "  Mẫu thử  ")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "Mẫu thử" || tpl.Revision != 1 {
		t.Fatalf("template = %+v", tpl)
	}
	if tpl.Recipe.Scenes.Style != StyleFull || tpl.Recipe.Scenes.ScenePromptID != BuiltinScene {
		t.Fatalf("recipe = %+v", tpl.Recipe.Scenes)
	}
	if len(tpl.Recipe.Overlays) != 1 || tpl.Recipe.Overlays[0] != snow.ID {
		t.Fatalf("overlays = %+v", tpl.Recipe.Overlays)
	}
	if _, err := svc.Create(ctx, "   "); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("tên rỗng: %v", err)
	}
}

func TestCreateStandardTemplateName(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	id, err := svc.CreateStandardTemplate(ctx, "Kênh A")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := store.GetTemplate(ctx, id)
	if err != nil || tpl == nil {
		t.Fatalf("template = %+v %v", tpl, err)
	}
	if tpl.Name != "Mẫu chuẩn · Kênh A" {
		t.Fatalf("name = %q", tpl.Name)
	}
	plain, err := svc.CreateStandardTemplate(ctx, " ")
	if err != nil {
		t.Fatal(err)
	}
	plainTpl, _ := store.GetTemplate(ctx, plain)
	if plainTpl.Name != "Mẫu chuẩn" {
		t.Fatalf("name rỗng = %q", plainTpl.Name)
	}
	if plain == id {
		t.Fatal("mỗi lần gọi phải tạo template mới")
	}
}

func TestDuplicateTemplate(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	src, err := svc.Create(ctx, "Gốc")
	if err != nil {
		t.Fatal(err)
	}
	dup, err := svc.Duplicate(ctx, src.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if dup.Name != "Gốc — Bản sao" || dup.Revision != 1 || dup.ID == src.ID {
		t.Fatalf("bản sao = %+v", dup)
	}
	if _, err := svc.Duplicate(ctx, "missing", ""); CodeOf(err) != CodeNotFound {
		t.Fatalf("nguồn lạ: %v", err)
	}
}

func TestPatchRevisionRaceOneWins(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Đua")
	if err != nil {
		t.Fatal(err)
	}
	recipe := []byte(`{"scenes":{"images":{"mode":"fixed","count":7}}}`)

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = svc.Patch(ctx, tpl.ID, Patch{Revision: tpl.Revision, Recipe: recipe})
		}(i)
	}
	wg.Wait()
	wins, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			wins++
		case CodeOf(err) == CodeConflict:
			conflicts++
			var apiErr *Error
			if errors.As(err, &apiErr) && !strings.Contains(apiErr.Recovery, "Tải lại trang") {
				t.Fatalf("recovery = %q", apiErr.Recovery)
			}
		default:
			t.Fatalf("lỗi lạ: %v", err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
	stored, _ := store.GetTemplate(ctx, tpl.ID)
	if stored.Revision != 2 || stored.Recipe.Scenes.Images.Count != 7 {
		t.Fatalf("sau khi lưu = %+v", stored)
	}
}

func TestPatchValidatesRecipeAndRefs(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Kiểm")

	_, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(`{"scenes":{"images":{"mode":"fixed","count":61}}}`)})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeInvalidInput || len(apiErr.Fields) != 1 || apiErr.Fields[0].Field != "scenes.images.count" {
		t.Fatalf("lỗi = %+v", err)
	}
	_, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(`{"scenes":{"coverPromptId":"00000000-0000-4000-8000-000000000404"}}`)})
	if !errors.As(err, &apiErr) || apiErr.Fields[0].Field != "scenes.coverPromptId" {
		t.Fatalf("lỗi tham chiếu = %+v", err)
	}
	if _, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Name: new("Tên mới"), Recipe: []byte(`{}`)}); err != nil {
		t.Fatalf("patch hợp lệ: %v", err)
	}
	got, _ := store.GetTemplate(ctx, tpl.ID)
	if got.Name != "Tên mới" || got.Revision != 2 {
		t.Fatalf("sau patch = %+v", got)
	}
}

func TestRenameTemplate(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Cũ")
	updated, err := svc.Rename(ctx, tpl.ID, "Mới", nil)
	if err != nil || updated.Name != "Mới" || updated.Revision != tpl.Revision {
		t.Fatalf("rename = %+v %v", updated, err)
	}
	stale := int64(99)
	if _, err := svc.Rename(ctx, tpl.ID, "Khác", &stale); CodeOf(err) != CodeConflict {
		t.Fatalf("revision cũ: %v", err)
	}
	if _, err := svc.Rename(ctx, "missing", "x", nil); CodeOf(err) != CodeNotFound {
		t.Fatalf("thiếu: %v", err)
	}
}

func TestDeleteTemplateBlockedWhileChannelUsesIt(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Đang dùng")
	store.channels[tpl.ID] = domain.ChannelRefList{{ID: "c1", Name: "Kênh Một"}, {ID: "c2", Name: "Kênh Hai"}}

	err := svc.DeleteTemplate(ctx, tpl.ID)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeConflict {
		t.Fatalf("lỗi = %v", err)
	}
	if len(apiErr.Channels) != 2 || apiErr.Channels[0] != "Kênh Một" {
		t.Fatalf("kênh = %+v", apiErr.Channels)
	}
	if tpl2, _ := store.GetTemplate(ctx, tpl.ID); tpl2 == nil {
		t.Fatal("không được xoá")
	}

	delete(store.channels, tpl.ID)
	if err := svc.DeleteTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("xoá: %v", err)
	}
	if err := svc.DeleteTemplate(ctx, tpl.ID); CodeOf(err) != CodeNotFound {
		t.Fatalf("xoá lần hai: %v", err)
	}
}

func TestPriceEstimateUsesTemplateImageCount(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store, func(d *Deps) { d.Price = fakePrice{usd: 0.04, ok: true} })
	ctx := context.Background()
	fixed := DefaultRecipe() // 5 ảnh + 1 bìa
	estimate := svc.Estimate(ctx, fixed)
	if !estimate.Known || estimate.ImagesPerVideo == nil || *estimate.ImagesPerVideo != 5 {
		t.Fatalf("estimate = %+v", estimate)
	}
	if estimate.TotalUSD == nil || *estimate.TotalUSD < 0.2399 || *estimate.TotalUSD > 0.2401 {
		t.Fatalf("total = %+v", estimate.TotalUSD)
	}
	perMinute := DefaultRecipe()
	perMinute.Scenes.Images = domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: 2}
	estimate = svc.Estimate(ctx, perMinute)
	if estimate.ImagesPerVideo != nil || estimate.TotalUSD != nil || estimate.Mode != ImagesPerMinutes || estimate.PerMinutes != 2 {
		t.Fatalf("theo phút = %+v", estimate)
	}
	none := DefaultRecipe()
	none.Scenes.Images = domain.ImagesMode{Mode: ImagesNone}
	estimate = svc.Estimate(ctx, none)
	if estimate.TotalUSD == nil || *estimate.TotalUSD < 0.0399 {
		t.Fatalf("chỉ bìa = %+v", estimate)
	}
	unknown := testService(t, store, func(d *Deps) { d.Price = fakePrice{ok: false} })
	if est := unknown.Estimate(ctx, fixed); est.Known || est.TotalUSD != nil {
		t.Fatalf("giá chưa biết = %+v", est)
	}
}

func TestListSummaryPromptText(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Tóm tắt")
	store.channels[tpl.ID] = domain.ChannelRefList{{ID: "c1", Name: "Kênh"}}
	items, err := svc.List(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("list = %+v %v", items, err)
	}
	item := items[0]
	if item.ScenePromptName != "Theo nhận diện kênh" || item.CoverPromptName != "Ảnh sạch" {
		t.Fatalf("tên prompt = %+v", item)
	}
	if !item.ImageCountKnown || item.ImageCount != 5 || len(item.Channels) != 1 {
		t.Fatalf("tóm tắt = %+v", item)
	}
	detail, err := svc.Get(ctx, tpl.ID)
	if err != nil || detail.Name != "Tóm tắt" || detail.Price.Mode != ImagesFixed {
		t.Fatalf("chi tiết = %+v %v", detail, err)
	}
	if _, err := svc.Get(ctx, "missing"); CodeOf(err) != CodeNotFound {
		t.Fatalf("thiếu: %v", err)
	}
}

// ── prompt ─────────────────────────────────────────────────────────────────

func TestListPromptsBuiltinsFirstAndSearch(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bìa Nhà Em", Kind: domain.PromptCover, Composition: "hook", Prompt: new("vẽ nhà")}); err != nil {
		t.Fatal(err)
	}
	items, err := svc.ListPrompts(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 8 {
		t.Fatalf("số prompt = %d", len(items))
	}
	if !items[0].BuiltIn || items[0].ID != BuiltinScene || items[len(items)-1].ID == BuiltinScene {
		t.Fatalf("thứ tự = %+v", items)
	}
	// tìm không dấu, không hoa thường
	items, err = svc.ListPrompts(ctx, "", "theo nhan dien")
	if err != nil || len(items) != 1 || items[0].ID != BuiltinScene {
		t.Fatalf("tìm = %+v %v", items, err)
	}
	items, err = svc.ListPrompts(ctx, domain.PromptCover, "nha em")
	if err != nil || len(items) != 1 || items[0].Name != "Bìa Nhà Em" {
		t.Fatalf("lọc bìa = %+v %v", items, err)
	}
	if _, err := svc.ListPrompts(ctx, "video", ""); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("kind lạ: %v", err)
	}
}

func TestCreatePromptValidationAndDuplicates(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()

	_, err := svc.CreatePrompt(ctx, PromptInput{Name: "Ảnh sạch", Kind: domain.PromptScene})
	if CodeOf(err) != CodeConflict {
		t.Fatalf("trùng prompt có sẵn: %v", err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "Của Tôi", Kind: domain.PromptScene, Prompt: new("a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "của tôi", Kind: domain.PromptScene, Prompt: new("b")}); CodeOf(err) != CodeConflict {
		t.Fatalf("trùng hoa thường: %v", err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "Thiếu bố cục", Kind: domain.PromptCover}); !hasField(err, "composition") {
		t.Fatalf("thiếu bố cục: %v", err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bố cục lạ", Kind: domain.PromptCover, Composition: "wide"}); !hasField(err, "composition") {
		t.Fatalf("bố cục lạ: %v", err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "", Kind: domain.PromptScene}); !hasField(err, "name") {
		t.Fatalf("tên rỗng: %v", err)
	}
	if _, err := svc.CreatePrompt(ctx, PromptInput{Name: "X", Kind: domain.PromptScene, FromID: "missing"}); !hasField(err, "fromId") {
		t.Fatalf("fromId lạ: %v", err)
	}
	// nhân bản prompt có sẵn: lấy nguyên nội dung, đổi tên
	copied, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bản của tôi", FromID: BuiltinCoverHook})
	if err != nil {
		t.Fatal(err)
	}
	if copied.Kind != domain.PromptCover || copied.Composition != "hook" || copied.BuiltIn {
		t.Fatalf("bản sao = %+v", copied)
	}
}

func TestPatchAndDeleteBuiltinPromptForbidden(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	if _, err := svc.PatchPrompt(ctx, BuiltinScene, PromptPatch{Prompt: new("x")}); CodeOf(err) != CodeBuiltin {
		t.Fatalf("sửa có sẵn: %v", err)
	}
	if err := svc.DeletePrompt(ctx, BuiltinCoverClean); CodeOf(err) != CodeBuiltin {
		t.Fatalf("xoá có sẵn: %v", err)
	}
	if err := svc.DeletePrompt(ctx, "missing"); CodeOf(err) != CodeNotFound {
		t.Fatalf("xoá thiếu: %v", err)
	}
}

func TestPromptInUseBlocksKindChangeAndDelete(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	prompt, err := svc.CreatePrompt(ctx, PromptInput{Name: "Của tôi", Kind: domain.PromptScene, Prompt: new("a")})
	if err != nil {
		t.Fatal(err)
	}
	tpl, _ := svc.Create(ctx, "Dùng prompt")
	_, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(fmt.Sprintf(`{"scenes":{"scenePromptId":%q}}`, prompt.ID))})
	if err != nil {
		t.Fatal(err)
	}
	newKind := string(domain.PromptCover)
	_, err = svc.PatchPrompt(ctx, prompt.ID, PromptPatch{Kind: &newKind, Composition: new("hook")})
	if CodeOf(err) != CodeInUse {
		t.Fatalf("đổi loại: %v", err)
	}
	err = svc.DeletePrompt(ctx, prompt.ID)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeInUse || len(apiErr.UsedBy) != 1 || apiErr.UsedBy[0] != tpl.ID {
		t.Fatalf("xoá: %+v", err)
	}
	if !strings.Contains(apiErr.Message, "Dùng prompt") {
		t.Fatalf("câu = %q", apiErr.Message)
	}
	// đổi nội dung vẫn được
	if _, err := svc.PatchPrompt(ctx, prompt.ID, PromptPatch{Prompt: new("b")}); err != nil {
		t.Fatalf("sửa nội dung: %v", err)
	}
}

func TestPromptExportImportRoundTrip(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	created, err := svc.CreatePrompt(ctx, PromptInput{Name: "Xuất", Kind: domain.PromptCover, Composition: "split", Prompt: new("nội dung")})
	if err != nil {
		t.Fatal(err)
	}
	exported, err := svc.ExportPrompt(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(exported)
	if exported.Format != PromptExportFormat || exported.Version != 1 || exported.Name != "Xuất" {
		t.Fatalf("export = %+v", exported)
	}
	imported, err := svc.ImportPrompt(ctx, raw)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.ID == created.ID {
		t.Fatal("import phải tạo id mới")
	}
	if imported.Name != "Xuất (nhập)" || imported.Kind != created.Kind || imported.Composition != created.Composition || imported.Prompt != created.Prompt {
		t.Fatalf("nhập = %+v", imported)
	}
	again, err := svc.ImportPrompt(ctx, raw)
	if err != nil || again.Name != "Xuất (nhập 2)" {
		t.Fatalf("nhập lần hai = %+v %v", again, err)
	}
	if _, err := svc.ImportPrompt(ctx, []byte(`{"format":"other"}`)); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("tệp lạ: %v", err)
	}
	if _, err := svc.ImportPrompt(ctx, []byte(`not json`)); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("JSON hỏng: %v", err)
	}
}

func TestPromptUsage(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "A")
	usage, err := svc.PromptUsage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(usage[BuiltinScene]) != 1 || usage[BuiltinScene][0].ID != tpl.ID {
		t.Fatalf("usage = %+v", usage)
	}
}

// ── snapshot ───────────────────────────────────────────────────────────────

func TestSnapshotResolvesBuiltinAndCustomPrompts(t *testing.T) {
	store := newMemStore()
	snow := seedSnow(store)
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Chốt")

	snap, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Style != StyleFull || snap.ImagePromptMode != "blend" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.ImagePrompt == "" || snap.ScenePrompt.ID != BuiltinScene || !snap.ScenePrompt.BuiltIn {
		t.Fatalf("imagePrompt = %+v", snap.ScenePrompt)
	}
	if snap.Thumbnail != "clean" || snap.CoverPrompt.Prompt == "" {
		t.Fatalf("bìa = %+v", snap)
	}
	if len(snap.Overlays) != 1 || snap.Overlays[0].ID != snow.ID || snap.Overlays[0].FilePath == "" {
		t.Fatalf("lớp phủ = %+v", snap.Overlays)
	}
	if snap.Revision != 1 || snap.TemplateName != "Chốt" || !snap.CompressExport {
		t.Fatalf("snapshot = %+v", snap)
	}

	custom, err := svc.CreatePrompt(ctx, PromptInput{Name: "Của tôi", Kind: domain.PromptScene, Prompt: new("vẽ theo ý tôi")})
	if err != nil {
		t.Fatal(err)
	}
	customCover, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bìa tôi", Kind: domain.PromptCover, Composition: "closeup", Prompt: new("mặt to")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(fmt.Sprintf(`{"scenes":{"scenePromptId":%q,"coverPromptId":%q}}`, custom.ID, customCover.ID))})
	if err != nil {
		t.Fatal(err)
	}
	snap, err = svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ImagePromptMode != "custom" || snap.ImagePrompt != "vẽ theo ý tôi" || snap.Thumbnail != "closeup" || snap.CoverPrompt.Prompt != "mặt to" {
		t.Fatalf("snapshot custom = %+v", snap)
	}
}

func TestSnapshotMissingPromptIsInvalidAndTombstonedOverlaySkipped(t *testing.T) {
	store := newMemStore()
	snow := seedSnow(store)
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Hỏng")
	broken := *tpl
	broken.Recipe.Scenes.ScenePromptID = "00000000-0000-4000-8000-000000000999"
	store.templates[tpl.ID] = broken
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); !hasField(err, "scenes.scenePromptId") {
		t.Fatalf("prompt mất: %v", err)
	}
	// Lớp phủ tự thêm đã bỏ thì snapshot bỏ qua (hiệu ứng có sẵn không bỏ được).
	custom := snow
	custom.Builtin = ""
	store.overlays[snow.ID] = custom
	if err := svc.RemoveOverlay(ctx, snow.ID); err != nil {
		t.Fatal(err)
	}
	store.templates[tpl.ID] = *tpl
	snap, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Overlays) != 0 {
		t.Fatalf("lớp phủ đã bỏ vẫn còn: %+v", snap.Overlays)
	}
	if _, err := svc.SnapshotTemplate(ctx, "missing"); CodeOf(err) != CodeNotFound {
		t.Fatalf("thiếu: %v", err)
	}
}

// ── trợ giúp ───────────────────────────────────────────────────────────────

// hasField báo lỗi invalid_input có đúng trường.
func hasField(err error, field string) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeInvalidInput {
		return false
	}
	for _, f := range apiErr.Fields {
		if f.Field == field {
			return true
		}
	}
	return false
}

func TestPatchBadBackdropKeepsRecipeAndRevision(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Nền")

	bad := []string{
		`{"backdrop":{"clipSec":"60"}}`,
		`{"backdrop":{"unexpected":true}}`,
		`{"backdrop":[1,2]}`,
		`{"scenes":{"images":{"mode":"fixed","count":9}}} rác phía sau`,
	}
	for _, body := range bad {
		_, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(body)})
		if CodeOf(err) != CodeInvalidInput {
			t.Fatalf("%s → %v", body, err)
		}
		stored, _ := store.GetTemplate(ctx, tpl.ID)
		if stored.Revision != 1 || stored.Recipe.Backdrop == nil != (tpl.Recipe.Backdrop == nil) {
			t.Fatalf("%s đã ghi đè recipe: %+v", body, stored.Recipe.Backdrop)
		}
	}
}

func TestUpdateTemplateRejectsMissingOrKindMismatchPrompt(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, _ := svc.Create(ctx, "Prompt đua")

	// Prompt của bạn đang được template trỏ tới.
	scene, err := svc.CreatePrompt(ctx, PromptInput{Name: "Cảnh riêng", Kind: domain.PromptScene, Prompt: new("x")})
	if err != nil {
		t.Fatal(err)
	}
	recipe := fmt.Sprintf(`{"scenes":{"scenePromptId":%q}}`, scene.ID)
	if _, err := svc.Patch(ctx, tpl.ID, Patch{Revision: tpl.Revision, Recipe: []byte(recipe)}); err != nil {
		t.Fatal(err)
	}
	// Xoá ở store (mô phỏng xoá đua thắng khoá) rồi ghi lại → lỗi tham chiếu.
	delete(store.prompts, scene.ID)
	_, err = svc.Patch(ctx, tpl.ID, Patch{Revision: tpl.Revision + 1, Recipe: []byte(`{"scenes":{"images":{"mode":"fixed","count":3}}}`)})
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("patch với prompt đã mất: %v", err)
	}
	// Đổi kind ở store rồi ghi → cũng bị chặn.
	cover, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bìa riêng", Kind: domain.PromptCover, Composition: "hook", Prompt: new("y")})
	if err != nil {
		t.Fatal(err)
	}
	mutated := *cover
	mutated.Kind = domain.PromptScene
	store.prompts[cover.ID] = mutated
	recipe = fmt.Sprintf(`{"scenes":{"coverPromptId":%q}}`, cover.ID)
	_, err = svc.Patch(ctx, tpl.ID, Patch{Revision: tpl.Revision + 1, Recipe: []byte(recipe)})
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("patch với prompt đổi loại: %v", err)
	}
}

func TestDeletePromptBlockedByTemplateAtStore(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	prompt, _ := svc.CreatePrompt(ctx, PromptInput{Name: "Đang dùng", Kind: domain.PromptScene, Prompt: new("x")})
	tpl, _ := svc.Create(ctx, "Dùng prompt")
	recipe := fmt.Sprintf(`{"scenes":{"scenePromptId":%q}}`, prompt.ID)
	if _, err := svc.Patch(ctx, tpl.ID, Patch{Revision: tpl.Revision, Recipe: []byte(recipe)}); err != nil {
		t.Fatal(err)
	}
	// Store từ chối xoá khi còn template dùng (đua giữa kiểm và xoá).
	if err := store.DeletePrompt(ctx, prompt.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("store xoá: %v", err)
	}
	if err := svc.DeletePrompt(ctx, prompt.ID); CodeOf(err) != CodeInUse {
		t.Fatalf("service xoá: %v", err)
	}
}

func TestGetMalformedIDIsNotFound(t *testing.T) {
	store := &notFoundStore{memStore: newMemStore()}
	svc := testService(t, store)
	if _, err := svc.Get(context.Background(), "not-a-uuid"); CodeOf(err) != CodeNotFound {
		t.Fatalf("malformed id: %v", err)
	}
}

// notFoundStore mô phỏng store trả ErrNotFound cho id không đọc được (như
// postgres với UUID hỏng).
type notFoundStore struct{ *memStore }

func (s *notFoundStore) GetTemplate(context.Context, string) (*domain.Template, error) {
	return nil, ErrNotFound
}

func TestImageCountForCeilBoundaries(t *testing.T) {
	per := func(min int) domain.ImagesMode { return domain.ImagesMode{Mode: ImagesPerMinutes, Minutes: min} }
	cases := []struct {
		mode     domain.ImagesMode
		duration float64
		want     int
	}{
		{per(2), 121, 2},
		{per(2), 120, 1},
		{per(1), 61, 2},
		{per(1), 60, 1},
		{per(1), 1, 1},
		{per(1), 60 * 120, 60},
		{domain.ImagesMode{Mode: ImagesFixed, Count: 5}, 999, 5},
		{domain.ImagesMode{Mode: ImagesNone}, 999, 0},
	}
	for _, tc := range cases {
		if got := ImageCountFor(tc.mode, tc.duration); got != tc.want {
			t.Fatalf("%+v %.0fs = %d, muốn %d", tc.mode, tc.duration, got, tc.want)
		}
	}
}
