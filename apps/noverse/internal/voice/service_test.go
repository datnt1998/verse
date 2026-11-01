package voice

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeEngine là engine thử: ghi lại thứ tự gọi và cho test điều khiển nhịp.
type fakeEngine struct {
	onSynth  func(ctx context.Context, req SynthRequest) (Audio, error)
	onAdd    func(ctx context.Context, name, wavPath string) (Voice, error)
	onVoice  func(ctx context.Context) ([]Voice, error)
	onStatus func(ctx context.Context) Status
	install  func(ctx context.Context, report func(InstallProgress)) error
	mu       sync.Mutex
	calls    []SynthRequest
	added    []string
	closeCnt int
}

func (f *fakeEngine) Name() string { return "fake" }

func (f *fakeEngine) Capabilities() Capabilities {
	return Capabilities{Cloning: true, SampleRate: 48000, Qualities: Qualities}
}

func (f *fakeEngine) Status(ctx context.Context) Status {
	if f.onStatus != nil {
		return f.onStatus(ctx)
	}
	return Status{Engine: "fake", State: StateReady, Ready: true}
}

func (f *fakeEngine) Voices(ctx context.Context) ([]Voice, error) {
	if f.onVoice != nil {
		return f.onVoice(ctx)
	}
	return []Voice{{ID: "g1", Name: "Giọng thử", Source: SourceBuiltin}}, nil
}

func (f *fakeEngine) Synthesize(ctx context.Context, req SynthRequest) (Audio, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	if f.onSynth != nil {
		return f.onSynth(ctx, req)
	}
	return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 1}, nil
}

func (f *fakeEngine) AddVoice(ctx context.Context, name, wavPath string) (Voice, error) {
	f.mu.Lock()
	f.added = append(f.added, name)
	f.mu.Unlock()
	if f.onAdd != nil {
		return f.onAdd(ctx, name, wavPath)
	}
	return Voice{ID: name, Name: name, Source: SourceClone}, nil
}

func (f *fakeEngine) Close() error {
	f.mu.Lock()
	f.closeCnt++
	f.mu.Unlock()
	return nil
}

func (f *fakeEngine) synthesized() []SynthRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SynthRequest(nil), f.calls...)
}

// registerFake đăng ký một engine thử dưới tên riêng rồi trả tên đó.
func registerFake(t *testing.T, f *fakeEngine) string {
	t.Helper()
	name := "fake-" + t.Name()
	Register(name, func(Config) (Engine, error) { return f, nil })
	return name
}

func TestServicePreviewJumpsRenderQueue(t *testing.T) {
	firstStarted := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var order []string
	fake := &fakeEngine{onSynth: func(_ context.Context, req SynthRequest) (Audio, error) {
		mu.Lock()
		order = append(order, req.Text)
		mu.Unlock()
		if req.Text == "render-1" {
			close(firstStarted)
			<-release
		}
		return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 1}, nil
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	t1, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "render-1"}, Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit render-1: %v", err)
	}
	<-firstStarted // render-1 đang chạy → hai việc sau phải xếp hàng
	t2, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "render-2"}, Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit render-2: %v", err)
	}
	t3, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "preview-1"}, Priority: PriorityPreview})
	if err != nil {
		t.Fatalf("submit preview-1: %v", err)
	}
	close(release)

	for _, tk := range []*Ticket{t1, t2, t3} {
		if _, err := tk.Wait(context.Background()); err != nil {
			t.Fatalf("wait: %v", err)
		}
	}
	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	want := []string{"render-1", "preview-1", "render-2"}
	if len(got) != len(want) {
		t.Fatalf("thứ tự gọi = %v, muốn %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("thứ tự gọi = %v, muốn %v", got, want)
		}
	}
}

func TestServicePauseHoldsJobUntilResume(t *testing.T) {
	started := make(chan string, 8)
	fake := &fakeEngine{onSynth: func(_ context.Context, req SynthRequest) (Audio, error) {
		started <- req.Text
		return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 1}, nil
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	if err := svc.Pause("job-a"); err != nil {
		t.Fatalf("pause: %v", err)
	}
	paused, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "a"}, JobID: "job-a", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit paused: %v", err)
	}
	other, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "b"}, JobID: "job-b", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit other: %v", err)
	}
	if _, err := other.Wait(context.Background()); err != nil {
		t.Fatalf("job khác phải chạy được khi job-a tạm dừng: %v", err)
	}
	select {
	case text := <-started:
		if text == "a" {
			t.Fatal("việc của job đang tạm dừng vẫn được đưa vào máy đọc")
		}
	case <-time.After(150 * time.Millisecond):
	}
	if st := svc.JobState("job-a"); st.Queued != 1 || !st.Paused {
		t.Fatalf("JobState = %+v, muốn queued=1 paused", st)
	}

	if err := svc.Resume("job-a"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := paused.Wait(context.Background()); err != nil {
		t.Fatalf("wait sau resume: %v", err)
	}
}

func TestServiceCancelDropsQueuedWork(t *testing.T) {
	firstStarted := make(chan struct{})
	release := make(chan struct{})
	fake := &fakeEngine{onSynth: func(ctx context.Context, req SynthRequest) (Audio, error) {
		if req.Text == "hold" {
			close(firstStarted)
			select {
			case <-release:
			case <-ctx.Done():
				return Audio{}, ctx.Err()
			}
		}
		return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 1}, nil
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	hold, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "hold"}, JobID: "other", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit hold: %v", err)
	}
	<-firstStarted
	a, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "a"}, JobID: "job-c", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit a: %v", err)
	}
	b, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "b"}, JobID: "job-c", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	n, err := svc.Cancel("job-c")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if n != 2 {
		t.Fatalf("cancel trả %d việc, muốn 2", n)
	}
	for _, tk := range []*Ticket{a, b} {
		if _, err := tk.Wait(context.Background()); !errors.Is(err, ErrCanceled) {
			t.Fatalf("vé bị huỷ trả %v, muốn ErrCanceled", err)
		}
	}
	close(release)
	if _, err := hold.Wait(context.Background()); err != nil {
		t.Fatalf("việc ngoài job không được bị huỷ: %v", err)
	}
	for _, call := range fake.synthesized() {
		if call.Text == "a" || call.Text == "b" {
			t.Fatalf("việc đã huỷ vẫn được đưa vào máy đọc: %q", call.Text)
		}
	}
}

func TestServiceCancelRunningStopsSynthesis(t *testing.T) {
	entered := make(chan struct{})
	fake := &fakeEngine{onSynth: func(ctx context.Context, _ SynthRequest) (Audio, error) {
		close(entered)
		<-ctx.Done() // chỉ thoát khi bị huỷ
		return Audio{}, ctx.Err()
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	tk, err := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "dài"}, JobID: "j", Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-entered
	if _, err := svc.Cancel("j"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := tk.Wait(context.Background())
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("vé trả %v, muốn ErrCanceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("huỷ việc đang chạy không dừng được máy đọc")
	}
}

func TestServiceCallerContextCancelsQueuedTicket(t *testing.T) {
	firstStarted := make(chan struct{})
	release := make(chan struct{})
	fake := &fakeEngine{onSynth: func(_ context.Context, req SynthRequest) (Audio, error) {
		if req.Text == "hold" {
			close(firstStarted)
			<-release
		}
		return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 1}, nil
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	hold, _ := svc.Submit(context.Background(), Request{SynthRequest: SynthRequest{Text: "hold"}, Priority: PriorityRender})
	<-firstStarted

	ctx, cancel := context.WithCancel(context.Background())
	tk, err := svc.Submit(ctx, Request{SynthRequest: SynthRequest{Text: "chờ"}, Priority: PriorityRender})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	cancel()
	if _, err := tk.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("vé trả %v, muốn context.Canceled", err)
	}
	close(release)
	if _, err := hold.Wait(context.Background()); err != nil {
		t.Fatalf("hold: %v", err)
	}
	for _, call := range fake.synthesized() {
		if call.Text == "chờ" {
			t.Fatal("việc có ctx đã huỷ vẫn được đưa vào máy đọc")
		}
	}
}

func TestServiceMeasuresRTF(t *testing.T) {
	fake := &fakeEngine{onSynth: func(context.Context, SynthRequest) (Audio, error) {
		time.Sleep(20 * time.Millisecond)
		return Audio{WAV: []byte("wav"), SampleRate: 48000, DurationSec: 2}, nil
	}}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()

	if rtf := svc.RTF(); rtf != nil {
		t.Fatalf("RTF trước khi đo phải là nil, được %v", *rtf)
	}
	if _, err := svc.Synthesize(context.Background(), SynthRequest{Text: "một đoạn"}); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	rtf := svc.RTF()
	if rtf == nil {
		t.Fatal("RTF chưa được đo sau lượt đọc đầu")
	}
	if *rtf <= 0 || *rtf > 0.5 {
		t.Fatalf("RTF = %v, muốn khoảng 20ms/2s = 0.01", *rtf)
	}
}

// installerFake là engine cài được, để thử luồng tiến độ cài.
type installerFake struct {
	fakeEngine
	steps []InstallProgress
	gate  chan struct{}
}

func (i *installerFake) Installed() bool { return false }

func (i *installerFake) Install(ctx context.Context, report func(InstallProgress)) error {
	for _, step := range i.steps {
		report(step)
	}
	if i.gate != nil {
		select {
		case <-i.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestServiceInstallProgressAndBusy(t *testing.T) {
	fake := &installerFake{
		steps: []InstallProgress{
			{Step: "deps", Percent: new(40), Message: "đang cài"},
			{Step: "model", Percent: new(80), Message: "đang tải"},
		},
		gate: make(chan struct{}),
	}
	svc := New(Config{Engine: registerFake(t, &fake.fakeEngine)})
	Register(svc.Config().Engine, func(Config) (Engine, error) { return fake, nil })
	defer svc.Close()

	var mu sync.Mutex
	var seen []InstallProgress
	unsub := svc.SubscribeInstall(func(p InstallProgress) {
		mu.Lock()
		seen = append(seen, p)
		mu.Unlock()
	})
	defer unsub()

	if err := svc.StartInstall(context.Background()); err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.StartInstall(context.Background()); !errors.Is(err, ErrInstallRunning) {
		t.Fatalf("lượt cài thứ hai trả %v, muốn ErrInstallRunning", err)
	}
	if st := svc.InstallState(); !st.InstallRunning() {
		t.Fatalf("InstallState = %+v, muốn đang chạy", st)
	}
	close(fake.gate)

	deadline := time.Now().Add(2 * time.Second)
	for {
		st := svc.InstallState()
		if st.State == "done" {
			if st.Percent == nil || *st.Percent != 100 {
				t.Fatalf("phần trăm cuối = %v, muốn 100", st.Percent)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lượt cài không kết thúc, trạng thái %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	got := len(seen)
	mu.Unlock()
	if got < 3 {
		t.Fatalf("chỉ nhận %d mốc tiến độ, muốn ≥ 3 (2 bước + kết thúc)", got)
	}
}

func TestServiceReportsMissingInstaller(t *testing.T) {
	fake := &fakeEngine{}
	svc := New(Config{Engine: registerFake(t, fake)})
	defer svc.Close()
	if err := svc.StartInstall(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("engine không cài được phải trả ErrUnsupported, được %v", err)
	}
	if !svc.Installed() {
		t.Fatal("engine không có Installer phải coi là đã sẵn sàng")
	}
}

// brokenInstall là engine chưa cài; lượt cài hỏng ngay (vd thiếu uv).
type brokenInstall struct{ installerFake }

func (b *brokenInstall) Status(context.Context) Status {
	return Status{Engine: "fake", State: StateMissing}
}

func (b *brokenInstall) Install(context.Context, func(InstallProgress)) error {
	return errors.New("chưa có uv trên máy")
}

// waitInstallState chờ tới khi lượt cài rời trạng thái running.
func waitInstallState(t *testing.T, svc *Service) InstallProgress {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		st := svc.InstallState()
		if !st.InstallRunning() {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("lượt cài không kết thúc, trạng thái %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Lượt cài hỏng phải để lại câu lỗi thật trong InstallState VÀ trong Status —
// Status là nguồn của S/audio/ready nên thiếu nó thì thẻ hướng dẫn chỉ nói
// "chưa sẵn sàng" mà không nói vì sao.
func TestServiceFailedInstallSurfacesError(t *testing.T) {
	eng := &brokenInstall{installerFake{steps: []InstallProgress{{Step: "deps", Percent: new(20), Message: "đang cài thư viện"}}}}
	name := "broken-" + t.Name()
	Register(name, func(Config) (Engine, error) { return eng, nil })
	svc := New(Config{Engine: name})
	defer svc.Close()

	if err := svc.StartInstall(context.Background()); err != nil {
		t.Fatalf("start install: %v", err)
	}
	st := waitInstallState(t, svc)
	if st.State != "error" || st.Error != "chưa có uv trên máy" {
		t.Fatalf("InstallState = %+v, muốn state error kèm lỗi của engine", st)
	}
	if got := svc.Status(context.Background()); got.Error != "chưa có uv trên máy" || got.State != StateMissing {
		t.Fatalf("Status = %+v, muốn state missing kèm lỗi cài", got)
	}
}

// stuckInstall treo mãi cho tới khi ctx bị huỷ (mạng chết giữa lúc tải mô hình).
type stuckInstall struct{ installerFake }

func (s *stuckInstall) Status(context.Context) Status {
	return Status{Engine: "fake", State: StateMissing}
}

func (s *stuckInstall) Install(ctx context.Context, _ func(InstallProgress)) error {
	<-ctx.Done()
	return ctx.Err()
}

// Quá trần thời gian thì lượt cài phải thành lỗi tiếng Việt có "Thử lại" được,
// không được đứng ở "Đang cài…" mãi và không được lộ "context deadline exceeded".
func TestServiceInstallStopsAfterTimeout(t *testing.T) {
	old := installTimeout
	installTimeout = 40 * time.Millisecond
	defer func() { installTimeout = old }()

	eng := &stuckInstall{}
	name := "stuck-" + t.Name()
	Register(name, func(Config) (Engine, error) { return eng, nil })
	svc := New(Config{Engine: name})
	defer svc.Close()

	if err := svc.StartInstall(context.Background()); err != nil {
		t.Fatalf("start install: %v", err)
	}
	st := waitInstallState(t, svc)
	if st.State != "error" || st.Message != "Cài bộ đọc quá lâu" {
		t.Fatalf("InstallState = %+v, muốn lỗi quá thời gian", st)
	}
	if strings.Contains(strings.ToLower(st.Error), "deadline") || st.Error == "" {
		t.Fatalf("lỗi phải là câu tiếng Việt cho người dùng, được %q", st.Error)
	}
}

// Status là đường đọc của màn Audio và Cài đặt › Giọng đọc: engine treo (nhận
// kết nối mà không trả lời) thì Status phải trả lỗi sau trần statusTimeout, không
// để màn hình quay mãi. Trước khi sửa, ctx của request không có hạn và Status đi
// thẳng vào engine nên treo vô hạn.
func TestStatusDoesNotWaitOnWedgedEngine(t *testing.T) {
	old := statusTimeout
	statusTimeout = 80 * time.Millisecond
	defer func() { statusTimeout = old }()

	eng := &fakeEngine{onStatus: func(ctx context.Context) Status {
		<-ctx.Done()
		return Status{}
	}}
	name := "wedged-" + t.Name()
	Register(name, func(Config) (Engine, error) { return eng, nil })
	svc := New(Config{Engine: name})
	defer svc.Close()

	done := make(chan Status, 1)
	go func() { done <- svc.Status(context.Background()) }()
	select {
	case st := <-done:
		if st.State != StateError || st.Error == "" {
			t.Fatalf("Status = %+v, muốn state=error kèm câu lỗi", st)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Status treo quá 3 giây dù trần là %s", statusTimeout)
	}
}
