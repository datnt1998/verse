package channels

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/core/llm"
)

// ── AI giả ─────────────────────────────────────────────────────────────────

type fakeAI struct {
	mu     sync.Mutex
	models []TextModel
	drawer Drawer
}

func newFakeAI() *fakeAI { return &fakeAI{} }

func (f *fakeAI) set(models []TextModel, drawer Drawer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.models, f.drawer = models, drawer
}

func (f *fakeAI) Director() []TextModel {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]TextModel(nil), f.models...)
}

func (f *fakeAI) Drawer() Drawer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.drawer
}

type fakeTextModel struct {
	mu       sync.Mutex
	answer   string
	err      error
	onCall   func()
	messages [][]llm.Message
}

func (m *fakeTextModel) Generate(_ context.Context, messages []llm.Message, _ []llm.ToolSpec, _ ...llm.CallOption) (*llm.LLMResponse, error) {
	m.mu.Lock()
	m.messages = append(m.messages, messages)
	onCall := m.onCall
	err := m.err
	answer := m.answer
	m.mu.Unlock()
	if onCall != nil {
		onCall()
	}
	if err != nil {
		return nil, err
	}
	return &llm.LLMResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(answer)}}}, nil
}

func (m *fakeTextModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

type fakeDrawer struct {
	mu       sync.Mutex
	requests []imagegen.Request
	err      error
	onDraw   func(req imagegen.Request)
	failKind string
	// outW/outH ≠ 0 ghi ảnh với kích thước lệch ý (provider trả size khác request).
	outW, outH int
}

func (d *fakeDrawer) Draw(_ context.Context, req imagegen.Request) (imagegen.Receipt, error) {
	d.mu.Lock()
	d.requests = append(d.requests, req)
	onDraw := d.onDraw
	err := d.err
	failKind := d.failKind
	d.mu.Unlock()
	if onDraw != nil {
		onDraw(req)
	}
	if err != nil && (failKind == "" || strings.Contains(req.Key, failKind)) {
		return imagegen.Receipt{}, err
	}
	w, h := req.Width, req.Height
	d.mu.Lock()
	if d.outW != 0 && d.outH != 0 {
		w, h = d.outW, d.outH
	}
	d.mu.Unlock()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 251), G: uint8(y % 241), B: 120, A: 255})
		}
	}
	f, err := os.OpenFile(req.Output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return imagegen.Receipt{}, err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return imagegen.Receipt{}, err
	}
	if err := f.Close(); err != nil {
		return imagegen.Receipt{}, err
	}
	return imagegen.Receipt{Provider: "chatgpt", Model: "image"}, nil
}

func (d *fakeDrawer) QuotaReset() (time.Time, bool) {
	return time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), true
}

func (d *fakeDrawer) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.requests)
}

const validAnswer = `{"name":"Chuyện Kể Đêm Mưa","brief":"Kênh kể chuyện ma cho người thích rùng mình.","summary":"Tông tối, mưa đêm, ánh đèn vàng.","look":"Moody cinematic world of rain-soaked alleys, amber lamps, deep teal shadows.","avatar":"A photoreal portrait of a storyteller under an amber lamp in the rain, lettering of the channel name.","banner":"A wide cinematic banner of a rain-soaked alley at night with the channel name as headline."}`

// setupIdentity dựng rig với model chữ trả answer và drawer ghi PNG thật.
func setupIdentity(t *testing.T) (*testRig, *fakeTextModel, *fakeDrawer) {
	t.Helper()
	r := newRig(t)
	r.store.addRival("rival-1", "Chuyện Ma")
	r.addChannel("ch-1", "", "rival-1")
	model := &fakeTextModel{answer: validAnswer}
	drawer := &fakeDrawer{}
	r.ai.set([]TextModel{model}, drawer)
	return r, model, drawer
}

// ── công bố thành công ─────────────────────────────────────────────────────

func TestIdentityPublishesNameAssetsAndBranding(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	jobID, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	if jobID == "" {
		t.Error("thiếu jobId")
	}
	view, err := r.svc.Get(context.Background(), "ch-1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Name != "Chuyện Kể Đêm Mưa" {
		t.Errorf("tên = %q", view.Name)
	}
	if view.Branding.Brief == "" || view.Branding.Look == "" || view.Branding.Summary == "" {
		t.Errorf("branding = %+v", view.Branding)
	}
	if view.AvatarPath == nil || view.BannerPath == nil {
		t.Fatalf("thiếu tài sản: %+v", view)
	}
	if !strings.HasSuffix(*view.AvatarPath, "-avatar.png") || !strings.HasSuffix(*view.BannerPath, "-banner.jpg") {
		t.Errorf("tên tệp = %q / %q", *view.AvatarPath, *view.BannerPath)
	}
	for _, rel := range []string{*view.AvatarPath, *view.BannerPath} {
		abs := filepath.Join(r.storage.root, rel)
		if st, err := os.Stat(abs); err != nil || st.Size() == 0 {
			t.Errorf("tệp %s thiếu: %v", rel, err)
		}
	}
	if view.IdentityState != domain.IdentityIdle {
		t.Errorf("identity state = %q, muốn idle sau công bố", view.IdentityState)
	}
	if drawer.count() != 2 {
		t.Errorf("số lần vẽ = %d, muốn 2", drawer.count())
	}
	if len(r.tasks.done) != 1 {
		t.Errorf("task chưa finish: %v", r.tasks.done)
	}
}

// Kênh tạo không tên: Đạo diễn đặt tên thì Mẫu chuẩn của kênh được đặt theo
// tên đó; lượt chỉ vẽ lại avatar (tên giữ nguyên) không đụng template.
func TestIdentityNamesStandardTemplateWhenNameChanges(t *testing.T) {
	r, _, _ := setupIdentity(t)
	ctx := context.Background()
	if _, err := r.svc.RunIdentity(ctx, "ch-1", IdentityInput{Part: domain.IdentityAll}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	if got := r.templates.namedCalls(); len(got) != 1 || got[0] != "ch-1=Chuyện Kể Đêm Mưa" {
		t.Fatalf("đặt tên Mẫu chuẩn = %v", got)
	}
	if _, err := r.svc.RunIdentity(ctx, "ch-1", IdentityInput{Part: domain.IdentityAvatar}); err != nil {
		t.Fatalf("RunIdentity avatar: %v", err)
	}
	if got := r.templates.namedCalls(); len(got) != 1 {
		t.Errorf("vẽ lại avatar vẫn đặt tên template: %v", got)
	}
}

func TestIdentityMalformedAnswerFailsTouchingNothing(t *testing.T) {
	r, model, drawer := setupIdentity(t)
	model.answer = `{"summary": "thiếu look"}`
	_, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if CodeOf(err) != CodeInvalidInput {
		t.Fatalf("code = %q, muốn invalid_input", CodeOf(err))
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.Name != "" || view.AvatarPath != nil || view.BannerPath != nil || view.Branding.Look != "" {
		t.Errorf("run hỏng đã đụng kênh: %+v", view)
	}
	if drawer.count() != 0 {
		t.Errorf("đã vẽ ảnh dù bước chữ hỏng: %d", drawer.count())
	}
	if view.IdentityState != domain.IdentityFailed || view.IdentityErrorCode == nil {
		t.Errorf("không ghi lỗi: %+v", view.Channel)
	}
	if view.IdentityFailedStep == nil || *view.IdentityFailedStep != domain.IdentityStepText {
		t.Errorf("bước hỏng = %v, muốn text (Đạo diễn)", view.IdentityFailedStep)
	}
}

// ── các phần ───────────────────────────────────────────────────────────────

func TestIdentityPartNameSkipsImages(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	// Nhận diện cũ có sẵn để part=name không kéo theo chữ "brief" bắt buộc.
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	fc.c.Branding = domain.Branding{Brief: "cũ", Summary: "cũ", Look: "cũ", Avatar: "cũ", Banner: "cũ"}
	r.store.mu.Unlock()

	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityName}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	if drawer.count() != 0 {
		t.Errorf("part=name đã vẽ %d ảnh", drawer.count())
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.Name == "" {
		t.Error("part=name không đặt tên")
	}
	if view.Branding.Brief != "cũ" {
		t.Errorf("brief = %q, muốn giữ cũ khi không gửi brief", view.Branding.Brief)
	}
}

func TestIdentityPartAvatarKeepsNameAndBanner(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	banner := "channels/ch-1/old-banner.jpg"
	fc.c.Name = "Tên Chủ"
	fc.c.Branding = domain.Branding{Brief: "mô tả", Look: "look", Avatar: "brief avatar", Banner: "brief banner"}
	fc.c.BannerPath = &banner
	r.store.mu.Unlock()

	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAvatar}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	if drawer.count() != 1 {
		t.Errorf("số lần vẽ = %d, muốn 1", drawer.count())
	}
	if req := drawer.requests[0]; req.Width != AvatarSize || req.Height != AvatarSize {
		t.Errorf("canvas = %dx%d, muốn 800x800", req.Width, req.Height)
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.Name != "Tên Chủ" {
		t.Errorf("tên = %q, part=avatar không được đổi tên", view.Name)
	}
	if view.BannerPath == nil || *view.BannerPath != banner {
		t.Errorf("banner = %v, muốn giữ cũ", view.BannerPath)
	}
	if view.Branding.Summary != "" {
		t.Errorf("summary = %q, part=avatar không chạy bước chữ", view.Branding.Summary)
	}
}

func TestIdentityBannerCanvasAndSafeBand(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityBanner}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	req := drawer.requests[0]
	if req.Width != BannerWidth || req.Height != BannerHeight {
		t.Errorf("canvas = %dx%d, muốn 2560x1440", req.Width, req.Height)
	}
	if !strings.Contains(req.Prompt, "1546x423") {
		t.Errorf("prompt thiếu băng an toàn 1546x423")
	}
}

// ── tham chiếu + lease ─────────────────────────────────────────────────────

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIdentitySendsReferenceAvatar(t *testing.T) {
	r, model, _ := setupIdentity(t)
	r.store.mu.Lock()
	r.store.rivals["rival-1"] = fakeRival{title: "Chuyện Ma", avatar: "https://yt3.googleusercontent.com/avatar.png"}
	r.store.mu.Unlock()
	r.svc.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(strings.NewReader("\x89PNG fake")),
		}, nil
	})}

	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityName}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	msgs := model.messages[0]
	var images int
	for _, block := range msgs[0].Content {
		if block.Type == llm.ContentImage {
			images++
		}
	}
	if images != 1 {
		t.Errorf("số ảnh tham chiếu gửi model = %d, muốn 1", images)
	}
}

func TestIdentitySecondRunConflicts(t *testing.T) {
	r, _, _ := setupIdentity(t)
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	fc.leaseToken = "đang-giữ"
	fc.leaseUntil = time.Now().Add(time.Minute)
	r.store.mu.Unlock()

	_, err := r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if CodeOf(err) != CodeConflict {
		t.Fatalf("code = %q, muốn conflict", CodeOf(err))
	}
}

func TestIdentityLostLeaseDoesNotPublish(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	drawer.onDraw = func(imagegen.Request) {
		r.store.mu.Lock()
		r.store.channels["ch-1"].leaseToken = "kẻ-khác"
		r.store.mu.Unlock()
	}
	_, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if CodeOf(err) != CodeConflict {
		t.Fatalf("code = %q, muốn conflict", CodeOf(err))
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.AvatarPath != nil || view.BannerPath != nil || view.Name != "" {
		t.Errorf("mất lease mà vẫn công bố: %+v", view)
	}
}

// ── hỏng + phục hồi ────────────────────────────────────────────────────────

func TestIdentityQuotaStoresRetryAt(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	reset := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	drawer.err = &imagegen.DrawError{Code: imagegen.ErrCodeQuota, Message: "Hết hạn mức ảnh hôm nay.", RetryAt: &reset}

	_, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if CodeOf(err) != CodeQuota {
		t.Fatalf("code = %q, muốn quota", CodeOf(err))
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.IdentityState != domain.IdentityFailed || view.IdentityErrorCode == nil || *view.IdentityErrorCode != imagegen.ErrCodeQuota {
		t.Fatalf("identity = %+v", view)
	}
	if view.IdentityRetryAt == nil || !view.IdentityRetryAt.Equal(reset) {
		t.Errorf("retryAt = %v, muốn %v", view.IdentityRetryAt, reset)
	}
	if view.IdentityFailedStep == nil || *view.IdentityFailedStep != domain.IdentityStepImages {
		t.Errorf("bước hỏng = %v, muốn images (Hoạ sĩ)", view.IdentityFailedStep)
	}
	if view.IdentityState != domain.IdentityFailed {
		t.Error("identity phải ở trạng thái hỏng")
	}
}

func TestIdentityInterruptedRecovery(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	fc.c.IdentityState = domain.IdentityRunning
	fc.leaseToken = "chết"
	fc.leaseUntil = r.now.Add(-time.Second)
	r.store.mu.Unlock()

	ids, err := r.store.MarkInterruptedIdentities(context.Background())
	if err != nil || len(ids) != 1 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.IdentityState != domain.IdentityFailed || *view.IdentityErrorCode != domain.IdentityErrInterrupted {
		t.Errorf("identity = %+v", view)
	}
}

// ── tranh chấp dữ liệu giữa run ────────────────────────────────────────────

func TestIdentityRenameMidRunKeepsOwnerName(t *testing.T) {
	r, model, _ := setupIdentity(t)
	model.onCall = func() {
		_, _ = r.svc.Update(context.Background(), "ch-1", UpdateRequest{Name: new("Tên Chủ Đặt")})
	}
	_, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if CodeOf(err) != CodeConflict {
		t.Fatalf("code = %q, muốn conflict", CodeOf(err))
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.Name != "Tên Chủ Đặt" {
		t.Errorf("tên = %q, muốn giữ tên chủ", view.Name)
	}
	if view.AvatarPath != nil {
		t.Error("đổi tên giữa chừng mà vẫn công bố tài sản")
	}
}

func TestIdentityDeleteMidRunDoesNotResurrect(t *testing.T) {
	r, model, _ := setupIdentity(t)
	model.onCall = func() {
		_ = r.svc.Delete(context.Background(), "ch-1")
	}
	_, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if err == nil {
		t.Fatal("run phải hỏng khi kênh bị xoá")
	}
	if _, gerr := r.svc.Get(context.Background(), "ch-1"); CodeOf(gerr) != CodeNotFound {
		t.Errorf("kênh bị hồi sinh: %v", gerr)
	}
}

func TestIdentityUnrelatedEditStillPublishes(t *testing.T) {
	r, model, _ := setupIdentity(t)
	model.onCall = func() {
		note := "kể chậm"
		notePtr := &note
		_, _ = r.svc.Update(context.Background(), "ch-1", UpdateRequest{StyleNote: &notePtr})
	}
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.Name == "" || view.StyleNote == nil {
		t.Errorf("sửa trường không liên quan chặn công bố: %+v", view)
	}
}

func TestIdentityOldAssetsKeptUntilSuccess(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	dir := filepath.Join(r.storage.root, "channels", "ch-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldAvatar := "channels/ch-1/old-avatar.png"
	oldBanner := "channels/ch-1/old-banner.jpg"
	for _, name := range []string{"old-avatar.png", "old-banner.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	fc.c.Branding = domain.Branding{Brief: "mô tả", Look: "look", Avatar: "avatar", Banner: "banner"}
	fc.c.AvatarPath = &oldAvatar
	fc.c.BannerPath = &oldBanner
	r.store.mu.Unlock()

	// Hỏng: tài sản cũ còn nguyên.
	drawer.err = errors.New("mạng hỏng")
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); err == nil {
		t.Fatal("phải hỏng")
	}
	view, _ := r.svc.Get(context.Background(), "ch-1")
	if view.AvatarPath == nil || *view.AvatarPath != oldAvatar || view.BannerPath == nil || *view.BannerPath != oldBanner {
		t.Fatalf("tài sản cũ bị đổi khi run hỏng: %+v", view)
	}
	for _, name := range []string{"old-avatar.png", "old-banner.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("tệp cũ %s bị xoá: %v", name, err)
		}
	}

	// Thành công: tệp cũ bị thay.
	drawer.err = nil
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); err != nil {
		t.Fatalf("RunIdentity: %v", err)
	}
	view, _ = r.svc.Get(context.Background(), "ch-1")
	if view.AvatarPath == nil || *view.AvatarPath == oldAvatar {
		t.Errorf("avatar chưa thay: %v", view.AvatarPath)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-avatar.png")); !os.IsNotExist(err) {
		t.Errorf("tệp avatar cũ còn: %v", err)
	}
}

// TestIdentityConcurrentGenerate chạy song song để -race bắt tranh chấp trên
// store/lease.
func TestIdentityConcurrentGenerate(t *testing.T) {
	r, _, _ := setupIdentity(t)
	var wg sync.WaitGroup
	results := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityName})
		}(i)
	}
	wg.Wait()
	var ok, conflicts int
	for _, err := range results {
		switch CodeOf(err) {
		case "":
			ok++
		case CodeConflict:
			conflicts++
		default:
			t.Errorf("lỗi lạ: %v", err)
		}
	}
	if ok == 0 || ok+conflicts != len(results) {
		t.Errorf("ok=%d conflicts=%d, muốn ít nhất một run chạy và phần còn lại bị từ chối", ok, conflicts)
	}
	_ = r.svc.Close(context.Background())
}

// TestIdentityRefusedRunCreatesNoTask: lệnh thua lease không được tạo rồi xoá
// task của run đang thắng — chỉ một task identity được bắt đầu.
func TestIdentityRefusedRunCreatesNoTask(t *testing.T) {
	r, model, _ := setupIdentity(t)
	sawText := make(chan struct{}, 1)
	model.onCall = func() {
		select {
		case sawText <- struct{}{}:
		default:
		}
		time.Sleep(150 * time.Millisecond)
	}
	task1, err := r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if err != nil {
		t.Fatal(err)
	}
	<-sawText // run đầu chắc chắn đang giữ lease
	if _, err := r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); CodeOf(err) != CodeConflict {
		t.Fatalf("run thứ hai: code = %q, muốn conflict", CodeOf(err))
	}
	waitState(t, r, "ch-1", func(c domain.Channel) bool { return c.IdentityState != domain.IdentityRunning })
	r.tasks.mu.Lock()
	defer r.tasks.mu.Unlock()
	if len(r.tasks.starts) != 1 {
		t.Fatalf("task starts = %d, muốn 1", len(r.tasks.starts))
	}
	if r.tasks.done[0] != task1 {
		t.Fatalf("task kết = %v, muốn %q", r.tasks.done, task1)
	}
}

// TestIdentityInternalErrorSanitized: nguyên nhân kỹ thuật (đường dẫn, chi tiết
// upstream) không được lưu vào DB hay trả UI; chỉ câu trung lập.
func TestIdentityInternalErrorSanitized(t *testing.T) {
	r, model, _ := setupIdentity(t)
	model.err = errors.New("secret-path /var/lib/private blew up")
	_, runErr := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
	if runErr == nil || strings.Contains(runErr.Error(), "secret-path") {
		t.Fatalf("runErr = %v, không được lộ nguyên nhân", runErr)
	}
	view := getView(t, r, "ch-1")
	if view.IdentityState != domain.IdentityFailed || view.IdentityError == nil {
		t.Fatalf("state = %q err = %v", view.IdentityState, view.IdentityError)
	}
	if strings.Contains(*view.IdentityError, "secret-path") {
		t.Fatalf("error lộ nguyên nhân: %q", *view.IdentityError)
	}
	if view.IdentityErrorCode == nil || *view.IdentityErrorCode != CodeInternal {
		t.Fatalf("code = %v", view.IdentityErrorCode)
	}
}

// TestIdentityBriefOnlyStillWritesText: kênh đã có brief (người viết mô tả)
// nhưng chưa từng sinh thiết kế → phần chữ vẫn chạy để có look/prompt ảnh.
func TestIdentityBriefOnlyStillWritesText(t *testing.T) {
	r, model, _ := setupIdentity(t)
	if _, err := r.svc.Update(context.Background(), "ch-1", UpdateRequest{Brief: new("Chủ thích tự viết")}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAvatar}); err != nil {
		t.Fatal(err)
	}
	if model.calls() == 0 {
		t.Fatal("không gọi text dù chưa có thiết kế")
	}
	view := getView(t, r, "ch-1")
	if view.Branding.Brief != "Chủ thích tự viết" {
		t.Fatalf("brief chủ viết bị đè: %q", view.Branding.Brief)
	}
	if view.Branding.Look == "" {
		t.Fatal("thiết kế không được publish")
	}
}

// TestIdentityPublishesExactCanvas: provider trả ảnh lệch kích thước thì file
// công bố phải đúng 800×800 / 2560×1440 (cover-crop + resize).
func TestIdentityPublishesExactCanvas(t *testing.T) {
	r, _, drawer := setupIdentity(t)
	drawer.outW, drawer.outH = 1024, 640
	if _, err := r.svc.RunIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); err != nil {
		t.Fatal(err)
	}
	view := getView(t, r, "ch-1")
	af, err := os.Open(filepath.Join(r.storage.root, *view.AvatarPath))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(af)
	af.Close()
	if err != nil || cfg.Width != 800 || cfg.Height != 800 {
		t.Fatalf("avatar = %v %v, muốn 800x800", cfg, err)
	}
	bf, err := os.Open(filepath.Join(r.storage.root, *view.BannerPath))
	if err != nil {
		t.Fatal(err)
	}
	bcfg, _, err := image.DecodeConfig(bf)
	bf.Close()
	if err != nil || bcfg.Width != 2560 || bcfg.Height != 1440 {
		t.Fatalf("banner = %v %v, muốn 2560x1440", bcfg, err)
	}
}

// TestCloseWaitsInFlightAndBlocksNew: prepare đang giữa chừng khi Close gọi
// phải được chờ chạy xong; gọi tiếp sau Close bị conflict ngay.
func TestCloseWaitsInFlightAndBlocksNew(t *testing.T) {
	r, _, _ := setupIdentity(t)
	r.store.acquireGate = make(chan struct{})
	r.store.acquireEnter = make(chan struct{}, 1)
	genErr := make(chan error, 1)
	go func() {
		_, err := r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll})
		genErr <- err
	}()
	<-r.store.acquireEnter // prepare đã admit và đang giữa acquire
	closeDone := make(chan struct{})
	go func() {
		_ = r.svc.Close(context.Background())
		close(closeDone)
	}()
	select {
	case <-closeDone:
		t.Fatal("Close trả xong khi prepare còn đang chạy")
	case <-time.After(80 * time.Millisecond):
	}
	close(r.store.acquireGate)
	if err := <-genErr; err != nil {
		t.Fatalf("GenerateIdentity = %v", err)
	}
	select {
	case <-closeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Close không trả sau khi run xong")
	}
	if _, err := r.svc.GenerateIdentity(context.Background(), "ch-1", IdentityInput{Part: domain.IdentityAll}); CodeOf(err) != CodeConflict {
		t.Fatalf("sau Close: code = %q, muốn conflict", CodeOf(err))
	}
}

// TestReconcileTimerMarksExpired: lease chết sót lại được timer đánh dấu hỏng
// khi tới hạn, không cần restart.
func TestReconcileTimerMarksExpired(t *testing.T) {
	r, _, _ := setupIdentity(t)
	r.store.mu.Lock()
	fc := r.store.channels["ch-1"]
	fc.leaseToken = "dead"
	fc.leaseUntil = r.now.Add(-time.Minute)
	fc.c.IdentityState = domain.IdentityRunning
	r.store.nextExpiry = &fc.leaseUntil
	r.store.mu.Unlock()
	r.svc.scheduleInterruptReconcile(context.Background())
	waitState(t, r, "ch-1", func(c domain.Channel) bool {
		return c.IdentityState == domain.IdentityFailed
	})
}

// waitState đợi view kênh đạt điều kiện (poll nhanh, tối đa 15 s — run thật
// phải encode JPEG 2560×1440 nên chậm rõ dưới -race).
func waitState(t *testing.T, r *testRig, id string, ok func(domain.Channel) bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ok(getView(t, r, id).Channel) {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("kênh không đạt trạng thái: %+v", getView(t, r, id).Channel)
}

func getView(t *testing.T, r *testRig, id string) ChannelView {
	t.Helper()
	view, err := r.svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return view
}
