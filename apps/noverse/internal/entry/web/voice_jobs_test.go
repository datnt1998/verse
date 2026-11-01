package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/identity"
)

// testTtsEngine là bộ đọc giả: cài theo kịch bản của test, không chạm đĩa/mạng.
type testTtsEngine struct {
	mu        sync.Mutex
	installed bool
	steps     []voice.InstallProgress
	fail      error
	gate      chan struct{} // đóng để cho lượt cài chạy tiếp
	starts    int
}

func (e *testTtsEngine) Name() string { return "tts-test" }

func (e *testTtsEngine) Capabilities() voice.Capabilities {
	return voice.Capabilities{SampleRate: 48000, Qualities: voice.Qualities}
}

func (e *testTtsEngine) Status(context.Context) voice.Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.installed {
		return voice.Status{Engine: e.Name(), State: voice.StateReady, Ready: true}
	}
	return voice.Status{Engine: e.Name(), State: voice.StateMissing}
}

func (e *testTtsEngine) Voices(context.Context) ([]voice.Voice, error) { return nil, nil }
func (e *testTtsEngine) AddVoice(context.Context, string, string) (voice.Voice, error) {
	return voice.Voice{}, errors.New("không dùng trong test")
}
func (e *testTtsEngine) Synthesize(context.Context, voice.SynthRequest) (voice.Audio, error) {
	return voice.Audio{}, errors.New("không dùng trong test")
}
func (e *testTtsEngine) Close() error { return nil }

func (e *testTtsEngine) Installed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.installed
}

func (e *testTtsEngine) Install(ctx context.Context, report func(voice.InstallProgress)) error {
	e.mu.Lock()
	e.installed = false
	e.starts++
	steps, gate, fail := e.steps, e.gate, e.fail
	e.mu.Unlock()
	for _, step := range steps {
		report(step)
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail != nil {
		return fail
	}
	e.mu.Lock()
	e.installed = true
	e.mu.Unlock()
	return nil
}

// useTtsService cắm Service dùng chung bằng engine giả rồi trả nó về như cũ.
func useTtsService(t *testing.T, eng *testTtsEngine) *voice.Service {
	t.Helper()
	name := "tts-test"
	voice.Register(name, func(voice.Config) (voice.Engine, error) { return eng, nil })
	svc := voice.New(voice.Config{Engine: name, Home: t.TempDir()}.WithDefaults())
	voice.SetShared(svc)
	t.Cleanup(func() {
		voice.SetShared(nil)
		_ = svc.Close()
	})
	return svc
}

func jobsOf(t *testing.T, tg *testGateway, cookie string) []Job {
	t.Helper()
	w := tg.do("GET", "/api/jobs", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/jobs → %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("jobs: %v (%s)", err, w.Body.String())
	}
	return body.Jobs
}

// waitJob chờ tới khi hàng có trạng thái mong muốn; trả nil nếu không tới.
func waitJob(t *testing.T, tg *testGateway, cookie, id, status string) *Job {
	t.Helper()
	return waitJobWhere(t, tg, cookie, id, func(j *Job) bool { return j.Status == status }, status)
}

// waitJobWhere chờ tới khi hàng thoả điều kiện (vd đã báo đủ tiến độ).
func waitJobWhere(t *testing.T, tg *testGateway, cookie, id string, ok func(*Job) bool, want string) *Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last *Job
	for {
		for _, job := range jobsOf(t, tg, cookie) {
			if job.ID != id {
				continue
			}
			copied := job
			last = &copied
			if ok(&copied) {
				return &copied
			}
		}
		if time.Now().After(deadline) {
			if last == nil {
				t.Fatalf("không thấy việc %q trong danh sách", id)
			}
			t.Fatalf("việc %q chưa tới điều kiện %q, hiện là %+v", id, want, *last)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Nút “Cài bộ đọc trên máy” phải có một hàng trong dock: tiến độ thật, xong thì
// có kết quả, và chỉ quản trị viên thấy (cùng quyền với POST /api/tts/install).
func TestVoiceInstallJobVisibleInDock(t *testing.T) {
	eng := &testTtsEngine{
		steps: []voice.InstallProgress{
			{Step: "source", Percent: 5, Message: "Đang tải mã nguồn VieNeu-TTS…"},
			{Step: "deps", Percent: 40, Message: "Đang cài thư viện Python…"},
		},
		gate: make(chan struct{}),
	}
	useTtsService(t, eng)
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	w := tg.do("POST", "/api/tts/install", admin, `{}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /api/tts/install → %d %s", w.Code, w.Body.String())
	}
	var started map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started["job"] != voiceInstallJobID {
		t.Fatalf("job = %v, muốn %q", started["job"], voiceInstallJobID)
	}

	// Chờ riêng mốc tiến độ cuối (40%): hàng "running" xuất hiện ngay từ mốc 0%,
	// nên chờ theo trạng thái thuần có thể bắt hàng chưa kịp báo bước.
	running := waitJobWhere(t, tg, admin, voiceInstallJobID,
		func(j *Job) bool {
			return j.Status == JobRunning && j.Progress != nil && *j.Progress == 0.4 && j.Step == "Đang cài thư viện Python…"
		}, "running · 40% · đang cài thư viện")
	if running.Kind != voiceJobsKind || running.Title == "" || running.Href != voiceSettingsRoute {
		t.Fatalf("hàng cài bộ đọc = %+v", running)
	}
	if running.Project != "" || running.Story != "" {
		t.Fatalf("việc của máy không được gắn truyện: %+v", running)
	}
	for _, j := range jobsOf(t, tg, reader) {
		if j.ID == voiceInstallJobID {
			t.Fatalf("người đọc không được thấy việc của máy: %+v", j)
		}
	}

	close(eng.gate)
	done := waitJob(t, tg, admin, voiceInstallJobID, JobDoneS)
	if done.Progress == nil || *done.Progress != 1 || done.FinishedAt == nil {
		t.Fatalf("việc xong phải có 100%% và giờ xong: %+v", done)
	}
	if done.Result == nil || done.Result.Title != "Đã cài xong bộ đọc trên máy" {
		t.Fatalf("kết quả = %+v", done.Result)
	}
}

// Lượt cài hỏng phải hiện lỗi + cho Thử lại; thử lại chạy tiếp và xong.
func TestVoiceInstallJobFailsThenRetry(t *testing.T) {
	eng := &testTtsEngine{
		steps: []voice.InstallProgress{{Step: "source", Percent: 5, Message: "Đang tải mã nguồn VieNeu-TTS…"}},
		fail:  errors.New("chưa có uv trên máy"),
	}
	useTtsService(t, eng)
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	if w := tg.do("POST", "/api/tts/install", admin, `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("POST /api/tts/install → %d %s", w.Code, w.Body.String())
	}
	failed := waitJob(t, tg, admin, voiceInstallJobID, JobFailed)
	if failed.Error != "chưa có uv trên máy" {
		t.Fatalf("lỗi = %q, muốn câu lỗi của engine", failed.Error)
	}

	// Việc cài bộ đọc trên máy chỉ dành cho quản trị viên.
	if w := tg.do("POST", "/api/jobs/"+voiceInstallJobID+"/retry", reader, "{}"); w.Code != http.StatusForbidden {
		t.Fatalf("retry bằng người đọc → %d, muốn 403", w.Code)
	}

	eng.mu.Lock()
	eng.fail = nil
	eng.mu.Unlock()
	if w := tg.do("POST", "/api/jobs/"+voiceInstallJobID+"/retry", admin, "{}"); w.Code != http.StatusOK {
		t.Fatalf("POST retry → %d %s", w.Code, w.Body.String())
	}
	done := waitJob(t, tg, admin, voiceInstallJobID, JobDoneS)
	if done.Error != nil {
		t.Fatalf("việc xong còn lỗi cũ: %+v", done)
	}
	eng.mu.Lock()
	starts := eng.starts
	eng.mu.Unlock()
	if starts != 2 {
		t.Fatalf("engine được cài %d lượt, muốn 2 (lượt đầu hỏng + lượt thử lại)", starts)
	}
}

// Tiến độ cài bộ đọc là trạng thái của máy chủ (đường dẫn/log cài đặt): hai
// đường đọc nó — polling và SSE — chỉ dành cho quản trị viên, như POST cài đặt.
func TestTtsInstallReadsAdminOnly(t *testing.T) {
	useTtsService(t, &testTtsEngine{})
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	reader := tg.login("reader", "reader-pass")

	if w := tg.do("GET", "/api/tts/install", reader, ""); w.Code != http.StatusForbidden {
		t.Fatalf("GET /api/tts/install bằng người đọc → %d, muốn 403: %s", w.Code, w.Body.String())
	}
	// SSE: phải bị chặn trước khi mở luồng, nên đặt hạn cho request — nếu cổng
	// mất, handler treo tới khi client ngắt và test thấy 200 thay vì 403.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	events := tg.doWith("GET", "/api/tts/install/events", reader, "", func(r *http.Request) { *r = *r.WithContext(ctx) })
	if events.Code != http.StatusForbidden {
		t.Fatalf("GET /api/tts/install/events bằng người đọc → %d, muốn 403", events.Code)
	}
	if w := tg.do("GET", "/api/tts/install", admin, ""); w.Code != http.StatusOK {
		t.Fatalf("GET /api/tts/install bằng admin → %d %s", w.Code, w.Body.String())
	}
}

// Việc của máy (Project/Story rỗng) không thuộc truyện nào nên bộ lọc theo quyền
// đọc không che được: khung job.* của nó chỉ được tới quản trị viên.
func TestMachineJobFramesOnlyForAdmins(t *testing.T) {
	tg := newTestGateway(t)
	hub := tg.g.events()
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	member := identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"_": identity.ProjectView}}
	asub, _, _ := hub.subscribe(admin, 0, nil)
	defer hub.unsubscribe(asub)
	msub, _, _ := hub.subscribe(member, 0, nil)
	defer hub.unsubscribe(msub)

	tg.g.jobsHub().apply(JobChange{Job: Job{ID: voiceInstallJobID, Kind: voiceJobsKind, Title: "Cài bộ đọc trên máy", Status: JobRunning}})

	select {
	case ev := <-asub.C:
		if ev.Type != "job.created" {
			t.Fatalf("khung cho admin = %s", ev.Type)
		}
	default:
		t.Fatal("admin không nhận được khung việc của máy")
	}
	select {
	case ev := <-msub.C:
		t.Fatalf("thành viên nhận được khung việc của máy: %+v", ev)
	default:
	}
	if !(Event{Type: "job.created", AdminOnly: true}).visibleTo(admin) {
		t.Fatal("khung AdminOnly phải tới quản trị viên")
	}
	if (Event{Type: "job.created", AdminOnly: true}).visibleTo(member) {
		t.Fatal("khung AdminOnly không được tới thành viên")
	}
}
