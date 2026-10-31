package channels

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ── phụ thuộc giả ──────────────────────────────────────────────────────────

type fakeTemplates struct {
	mu      sync.Mutex
	next    int
	created []string
	deleted []string
	named   []string // "<kênh>=<tên>" mỗi lần NameStandardTemplates
	failOn  string
}

func (f *fakeTemplates) CreateStandardTemplate(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn == "create" {
		return "", errors.New("hỏng tạo template")
	}
	f.next++
	id := "template-" + string(rune('0'+f.next))
	f.created = append(f.created, id)
	return id, nil
}

func (f *fakeTemplates) DeleteTemplate(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeTemplates) NameStandardTemplates(_ context.Context, channelID, channelName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.named = append(f.named, channelID+"="+channelName)
	return nil
}

func (f *fakeTemplates) namedCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.named...)
}

type fakeTasks struct {
	mu      sync.Mutex
	next    int
	starts  []domain.TaskSpec
	updates []domain.TaskUpdate
	done    []string
	notify  []string
}

func (f *fakeTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := "task-" + string(rune('0'+f.next))
	f.starts = append(f.starts, spec)
	return id, nil
}

func (f *fakeTasks) Update(_ context.Context, _ string, u domain.TaskUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, u)
	return nil
}

func (f *fakeTasks) Finish(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done = append(f.done, id)
	return nil
}

func (f *fakeTasks) FindTaskByObject(context.Context, domain.TaskKind, domain.ObjectRef) (*TaskInfo, error) {
	return nil, nil
}

func (f *fakeTasks) Notify(_ context.Context, _ string, text, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notify = append(f.notify, text)
	return "notice-1", nil
}

type fakeStorage struct{ root string }

func (f fakeStorage) Root() string { return f.root }

func (f fakeStorage) Resolve(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", errors.New("đường dẫn phải tương đối")
	}
	return filepath.Join(f.root, rel), nil
}

// ── khung test ─────────────────────────────────────────────────────────────

type testRig struct {
	svc       *Service
	store     *fakeStore
	templates *fakeTemplates
	tasks     *fakeTasks
	storage   fakeStorage
	ai        *fakeAI
	now       time.Time
	mu        sync.Mutex
}

func (r *testRig) advance(d time.Duration) {
	r.mu.Lock()
	r.now = r.now.Add(d)
	r.mu.Unlock()
}

func newRig(t *testing.T) *testRig {
	t.Helper()
	r := &testRig{
		now:     time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC),
		storage: fakeStorage{root: t.TempDir()},
		ai:      newFakeAI(),
	}
	r.templates = &fakeTemplates{}
	r.tasks = &fakeTasks{}
	r.store = newFakeStore(func() time.Time {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.now
	})
	r.svc = New(Options{
		Store:           r.store,
		Templates:       r.templates,
		Tasks:           r.tasks,
		Storage:         r.storage,
		AI:              r.ai,
		RunInBackground: true,
		Now: func() time.Time {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.now
		},
	})
	if err := r.svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = r.svc.Close(context.Background()) })
	return r
}

// addChannel tạo thẳng một kênh trong store giả (không qua Create).
func (r *testRig) addChannel(id, name string, rivals ...string) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	now := r.now
	fc := &fakeChannel{c: domain.Channel{
		ID: id, Name: name, Language: domain.LangVI, AutoPick: true, PacePerDay: 5,
		AlwaysTags: []string{}, IdentityState: domain.IdentityIdle, CreatedAt: now, UpdatedAt: now,
	}}
	for _, rivalID := range rivals {
		fc.sources = append(fc.sources, domain.ChannelSource{
			RivalID: rivalID, Title: r.store.rivals[rivalID].title, LinkedAt: now,
		})
	}
	r.store.channels[id] = fc
}

// ── tạo / đọc / sửa / xoá ──────────────────────────────────────────────────

func TestCreateGuessesLanguageAndStartsIdentity(t *testing.T) {
	r := newRig(t)
	r.store.addRival("rival-1", "Chuyện ma kể đêm")
	r.store.samples["rival-1"] = LanguageSample{Title: "Chuyện ma kể đêm", Titles: []string{"Đêm đầu tiên ở nhà hoang"}}

	view, jobID, err := r.svc.Create(context.Background(), CreateRequest{
		CompetitorIDs: []string{"rival-1"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.Language != domain.LangVI {
		t.Errorf("ngôn ngữ = %q, muốn vi", view.Language)
	}
	if len(view.Sources) != 1 || !view.Sources[0].Reference {
		t.Errorf("nguồn = %+v, muốn một đối thủ tham chiếu", view.Sources)
	}
	if len(view.Templates) != 1 {
		t.Errorf("template = %+v, muốn Mẫu chuẩn đã gắn", view.Templates)
	}
	if jobID == "" {
		t.Error("thiếu identityJobId")
	}
	if len(r.templates.created) != 1 {
		t.Errorf("template tạo %v, muốn đúng một", r.templates.created)
	}
}

func TestCreateCompensatesWhenTxFails(t *testing.T) {
	r := newRig(t)
	r.store.addRival("rival-1", "Kênh A")
	r.store.createErr = errors.New("mất kết nối DB")

	_, _, err := r.svc.Create(context.Background(), CreateRequest{CompetitorIDs: []string{"rival-1"}})
	if err == nil {
		t.Fatal("Create phải hỏng khi tx hỏng")
	}
	if len(r.templates.created) != 1 || len(r.templates.deleted) != 1 {
		t.Errorf("created=%v deleted=%v, muốn tạo rồi xoá bù trừ", r.templates.created, r.templates.deleted)
	}
}

func TestCreateValidation(t *testing.T) {
	r := newRig(t)
	cases := []struct {
		name string
		in   CreateRequest
	}{
		{"nhịp 0", CreateRequest{PacePerDay: new(0)}},
		{"nhịp 51", CreateRequest{PacePerDay: new(51)}},
		{"ngôn ngữ lạ", CreateRequest{Language: "fr"}},
	}
	for _, tc := range cases {
		if _, _, err := r.svc.Create(context.Background(), tc.in); CodeOf(err) != CodeInvalidInput {
			t.Errorf("%s: code = %q, muốn invalid_input", tc.name, CodeOf(err))
		}
	}
}

func TestListEmptyNamesLast(t *testing.T) {
	r := newRig(t)
	r.addChannel("a", "Zeta")
	r.addChannel("b", "")
	r.addChannel("c", "alpha")
	list, err := r.svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := []string{list[0].Name, list[1].Name, list[2].Name}
	want := []string{"alpha", "Zeta", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("thứ tự = %v, muốn %v", got, want)
		}
	}
}

// Chủ tự đặt tên kênh: Mẫu chuẩn còn tên trần được đặt theo tên mới (đã trim);
// sửa trường khác không đụng template.
func TestUpdateNameNamesStandardTemplate(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "")
	ctx := context.Background()
	if _, err := r.svc.Update(ctx, "ch-1", UpdateRequest{PacePerDay: new(3)}); err != nil {
		t.Fatal(err)
	}
	if got := r.templates.namedCalls(); len(got) != 0 {
		t.Fatalf("sửa nhịp đã đặt tên template: %v", got)
	}
	if _, err := r.svc.Update(ctx, "ch-1", UpdateRequest{Name: new("  Kênh Mới  ")}); err != nil {
		t.Fatal(err)
	}
	if got := r.templates.namedCalls(); len(got) != 1 || got[0] != "ch-1=Kênh Mới" {
		t.Fatalf("đặt tên Mẫu chuẩn = %v", got)
	}
}

func TestUpdateNormalizesTagsAndClears(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	tags := []string{"  ma  ", "MA", "kinh dị", ""}
	var url *string
	view, err := r.svc.Update(context.Background(), "ch-1", UpdateRequest{
		AlwaysTags: &tags,
		YoutubeURL: &url,
		PacePerDay: new(7),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(view.AlwaysTags) != 2 || view.AlwaysTags[0] != "ma" || view.AlwaysTags[1] != "kinh dị" {
		t.Errorf("tags = %v, muốn [ma kinh dị]", view.AlwaysTags)
	}
	if view.YoutubeURL != nil {
		t.Errorf("youtubeUrl = %v, muốn xoá", *view.YoutubeURL)
	}
	if view.PacePerDay != 7 {
		t.Errorf("pace = %d, muốn 7", view.PacePerDay)
	}
	if _, err := r.svc.Update(context.Background(), "ch-1", UpdateRequest{PacePerDay: new(99)}); CodeOf(err) != CodeInvalidInput {
		t.Errorf("pace 99 code = %q, muốn invalid_input", CodeOf(err))
	}
}

func TestDeleteRemovesFilesAndChannel(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	dir := filepath.Join(r.storage.root, "channels", "ch-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x-avatar.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Delete(context.Background(), "ch-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("thư mục kênh còn: %v", err)
	}
	if _, err := r.svc.Get(context.Background(), "ch-1"); CodeOf(err) != CodeNotFound {
		t.Errorf("Get sau xoá code = %q, muốn not_found", CodeOf(err))
	}
}

// ── liên kết ───────────────────────────────────────────────────────────────

func TestLinkUnknownRivalIsNotFound(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	_, err := r.svc.Link(context.Background(), "ch-1", "rival-lạ")
	if CodeOf(err) != CodeNotFound {
		t.Fatalf("code = %q, muốn not_found", CodeOf(err))
	}
	if !strings.Contains(err.Error(), "Đối thủ") {
		t.Errorf("câu lỗi %q phải dùng từ Đối thủ", err.Error())
	}
}

func TestLinkOnlyTouchesLinks(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	r.store.addRival("rival-1", "Kênh B")
	before, _ := r.svc.Get(context.Background(), "ch-1")

	view, err := r.svc.Link(context.Background(), "ch-1", "rival-1")
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if len(view.Sources) != 1 {
		t.Fatalf("sources = %+v", view.Sources)
	}
	if view.Name != before.Name || view.Language != before.Language || view.PacePerDay != before.PacePerDay {
		t.Error("Link đổi trường ngoài liên kết")
	}
}

func TestUnlinkLastPausesAutoPick(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A", "rival-1")
	r.store.addRival("rival-1", "Kênh B")
	view, paused, err := r.svc.Unlink(context.Background(), "ch-1", "rival-1")
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if !paused {
		t.Error("autoPickPaused = false, muốn true khi bỏ đối thủ cuối")
	}
	if view.AutoPick {
		t.Error("autoPick còn bật sau khi bỏ đối thủ cuối")
	}
	if len(view.Sources) != 0 {
		t.Errorf("sources = %+v, muốn rỗng", view.Sources)
	}
}

func TestUnlinkMovesReference(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A", "rival-1", "rival-2")
	r.store.addRival("rival-1", "Kênh B")
	r.store.addRival("rival-2", "Kênh C")
	if _, _, err := r.svc.Unlink(context.Background(), "ch-1", "rival-1"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	view, err := r.svc.Get(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(view.Sources) != 1 || view.Sources[0].RivalID != "rival-2" || !view.Sources[0].Reference {
		t.Errorf("sources = %+v, muốn rival-2 thành tham chiếu", view.Sources)
	}
}

// ── template ───────────────────────────────────────────────────────────────

func TestTemplateRotation(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	for _, id := range []string{"t-a", "t-b", "t-c"} {
		r.store.addTemplate(id, id)
	}
	ctx := context.Background()
	if _, err := r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateAdd, TemplateID: "t-a"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateAdd, TemplateID: "t-b"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateAdd, TemplateID: "t-c"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	pos := 0
	list, err := r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateMove, TemplateID: "t-c", Position: &pos})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	want := []string{"t-c", "t-a", "t-b"}
	for i, tmpl := range list {
		if tmpl.ID != want[i] || tmpl.Position != i {
			t.Fatalf("sau move = %+v, muốn %v với position liền mạch", list, want)
		}
	}
	list, err = r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateRemove, TemplateID: "t-a"})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(list) != 2 || list[0].ID != "t-c" || list[1].Position != 1 {
		t.Fatalf("sau remove = %+v", list)
	}
	list, err = r.svc.Templates(ctx, "ch-1", TemplateRequest{Action: TemplateStandard})
	if err != nil {
		t.Fatalf("standard: %v", err)
	}
	if len(list) != 3 || len(r.templates.created) != 1 {
		t.Fatalf("standard = %+v created=%v", list, r.templates.created)
	}
	if list[2].ID != r.templates.created[0] {
		t.Errorf("standard không nối vào cuối: %+v", list)
	}
}

func TestTemplateAddUnknownIsNotFound(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	_, err := r.svc.Templates(context.Background(), "ch-1", TemplateRequest{Action: TemplateAdd, TemplateID: "t-lạ"})
	if CodeOf(err) != CodeNotFound {
		t.Fatalf("code = %q, muốn not_found", CodeOf(err))
	}
}

func TestUnlinkUnknownIsNotFound(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A", "rival-1")
	r.store.addRival("rival-1", "B")
	r.store.addRival("rival-2", "C")
	if _, _, err := r.svc.Unlink(context.Background(), "ch-1", "rival-2"); CodeOf(err) != CodeNotFound {
		t.Fatalf("code = %q, muốn not_found", CodeOf(err))
	}
}

func TestGuessLanguage(t *testing.T) {
	cases := []struct {
		name   string
		sample LanguageSample
		want   string
	}{
		{"dấu tiếng Việt", LanguageSample{Title: "Chuyện ma kể đêm"}, domain.LangVI},
		{"tiêu đề video có dấu", LanguageSample{Title: "Ghost Stories", Titles: []string{"Đêm đầu tiên"}}, domain.LangVI},
		{"chỉ Latin", LanguageSample{Title: "Ghost Stories", Description: "True horror tales every night.", Titles: []string{"The haunted house"}}, domain.LangEN},
		{"rỗng", LanguageSample{}, ""},
		{"chữ Hán", LanguageSample{Title: "恐怖故事"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.addChannel("ch-1", "Kênh", "rival-1")
			r.store.addRival("rival-1", "B")
			r.store.samples["rival-1"] = tc.sample
			if got := r.svc.guessLanguage(context.Background(), "ch-1"); got != tc.want {
				t.Fatalf("guess = %q, muốn %q", got, tc.want)
			}
		})
	}
}

func TestAssetPathStrict(t *testing.T) {
	r := newRig(t)
	r.addChannel("ch-1", "Kênh A")
	dir := filepath.Join(r.storage.root, "channels", "ch-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := "11111111-1111-4111-8111-111111111111-avatar.png"
	if err := os.WriteFile(filepath.Join(dir, file), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := "channels/ch-1/" + file
	r.store.mu.Lock()
	r.store.channels["ch-1"].c.AvatarPath = &rel
	r.store.mu.Unlock()

	f, name, _, err := r.svc.AssetFile(context.Background(), "ch-1", file)
	if err != nil || name != file {
		t.Fatalf("AssetFile = %q %v", name, err)
	}
	f.Close()
	for _, bad := range []string{
		"../" + file,
		"11111111-1111-4111-8111-111111111112-avatar.png",
		"11111111-1111-4111-8111-111111111111-banner.jpg",
		"x-avatar.png",
	} {
		if _, _, _, err := r.svc.AssetFile(context.Background(), "ch-1", bad); CodeOf(err) != CodeNotFound {
			t.Errorf("%q: code = %q, muốn not_found", bad, CodeOf(err))
		}
	}
}
