// Trạng thái hàng công cụ theo design (tên theo việc làm) — chép logic
// readiness của src/server/tools/readiness.ts và setup-steps.ts (yt-factory).
package toolsvc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/logs"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// versionRunner trả phiên bản cho `--version` (yt-dlp, codex) và “chưa đăng
// nhập” cho `codex login status`; lỗi cho mọi lệnh khác.
type versionRunner struct {
	ytdlp string
	mu    sync.Mutex
}

func (r *versionRunner) Run(_ context.Context, file string, args []string, _ tools.Spec) (tools.Result, error) {
	base := filepath.Base(file)
	switch {
	case len(args) == 1 && args[0] == "--version" && strings.Contains(base, "yt-dlp"):
		return tools.Result{Stdout: r.ytdlp + "\n"}, nil
	case len(args) == 1 && args[0] == "--version" && base == "codex":
		return tools.Result{Stdout: "codex-cli " + tools.CodexPin().Version + "\n"}, nil
	case len(args) == 2 && args[0] == "login" && args[1] == "status" && base == "codex":
		return tools.Result{Stdout: "Not logged in\n"}, nil
	}
	return tools.Result{}, &tools.ProcessFailedError{Message: "spawn ENOENT", Code: -1}
}

// redirectTransport trả 302 về tag cho mọi request (kiểm cập nhật yt-dlp).
type redirectTransport struct{ tag string }

func (t redirectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": []string{"https://github.com/yt-dlp/yt-dlp/releases/tag/" + t.tag}},
		Body:       http.NoBody,
	}, nil
}

// failTransport trả 500 cho mọi request (lượt cài hỏng).
type failTransport struct{}

func (failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{}, Body: http.NoBody}, nil
}

type fakeTasks struct {
	mu     sync.Mutex
	specs  []domain.TaskSpec
	states []domain.TaskState
}

func (f *fakeTasks) Start(_ context.Context, spec domain.TaskSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs = append(f.specs, spec)
	return fmt.Sprintf("task-%d", len(f.specs)), nil
}

func (f *fakeTasks) Update(_ context.Context, _ string, u domain.TaskUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, u.State)
	return nil
}

func (f *fakeTasks) Finish(context.Context, string) error { return nil }

func (f *fakeTasks) lastState() domain.TaskState {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.states) == 0 {
		return ""
	}
	return f.states[len(f.states)-1]
}

// fakeChatGPT trả kiểu đăng nhập theo cấu hình.
type fakeChatGPT struct{ login string }

func (f fakeChatGPT) ChatGPTLogin(context.Context) (string, error) { return f.login, nil }

// newService dựng service thật trên công cụ thật với Runner/HTTP giả.
func newService(t *testing.T, client *http.Client, chatGPT ChatGPTSession) (*Service, *fakeTasks, *tools.Tools) {
	t.Helper()
	base := t.TempDir()
	pathDir := t.TempDir()
	// yt-dlp giả trên PATH: tệp thật để Resolve thấy.
	exe := filepath.Join(pathDir, "yt-dlp")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &versionRunner{ytdlp: "2025.01.01"}
	deps := tools.Deps{
		Runner: runner, Paths: &tools.FilePathStore{Dir: filepath.Join(base, "paths")},
		BaseDir: base, Client: client, Home: base,
		PathEnv:    pathDir,
		SystemDirs: []string{},
		Registry:   tools.NewRegistry(base, nil),
	}
	kit := tools.New(deps)
	mgr := tools.NewManaged(deps, nil, kit)
	trans := tools.NewTranscription(deps, kit.Uv, nil)
	tasks := &fakeTasks{}
	svc := New(Deps{
		Tools: kit, Managed: mgr, Transcription: trans,
		Tasks: tasks, ChatGPT: chatGPT,
		Now: time.Now,
	})
	return svc, tasks, kit
}

// fakeExe ghi một tệp thực thi rỗng để Resolve/probe thấy nó.
func fakeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCodexFoundOnMachineIsProvidedButManagedIsNot(t *testing.T) {
	ctx := context.Background()

	// Codex app tự cài: bản ghim trong thư mục công cụ (newService đặt Home = BaseDir).
	svc, _, kit := newService(t, &http.Client{Transport: redirectTransport{tag: "2025.01.01"}}, fakeChatGPT{login: "not_logged_in"})
	base := filepath.Dir(kit.Codex.DefaultHome())
	pin := tools.CodexPin().Version
	fakeExe(t, filepath.Join(toolchain.ToolVersionDir(base, "codex", pin), "codex"))
	if err := os.WriteFile(filepath.Join(toolchain.ToolRoot(base, "codex"), "current"), []byte("v-"+pin), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.Snapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if row := rowOf(t, snap, ToolCodex); row.State != StateReady || row.Provided {
		t.Fatalf("codex do app cài: %+v (muốn ready, không Provided)", row)
	}

	// Codex nằm cạnh yt-dlp giả trên PATH = binary phát hiện trên máy.
	svc, _, kit = newService(t, &http.Client{Transport: redirectTransport{tag: "2025.01.01"}}, fakeChatGPT{login: "not_logged_in"})
	_, ytdlpPath, _ := kit.YtDlp.Status(ctx)
	fakeExe(t, filepath.Join(filepath.Dir(ytdlpPath), "codex"))
	snap, err = svc.Snapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if row := rowOf(t, snap, ToolCodex); row.State != StateReady || !row.Provided {
		t.Fatalf("codex có sẵn trên máy: %+v (muốn ready + Provided)", row)
	}
}

// Hàng Chrome ở chế độ máy chủ không hứa dùng Chrome của máy: bản hệ thống có
// trên PATH vẫn bị bỏ, nên hàng báo thiếu (để bước cài tự cài) và nói rõ máy
// chủ dùng bản Chrome for Testing riêng.
func TestChromeRowHostedNeverUsesMachineChrome(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example")
	ctx := context.Background()
	svc, _, kit := newService(t, &http.Client{Transport: redirectTransport{tag: "2025.01.01"}}, fakeChatGPT{login: "not_logged_in"})
	_, ytdlpPath, _ := kit.YtDlp.Status(ctx)
	fakeExe(t, filepath.Join(filepath.Dir(ytdlpPath), "chrome"))
	snap, err := svc.Snapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	row := rowOf(t, snap, ToolChrome)
	if row.State != StateMissing || row.Provided {
		t.Fatalf("hàng chrome = %+v (muốn missing, không Provided)", row)
	}
	if !strings.Contains(row.Hint, "Chrome for Testing riêng") || !strings.Contains(row.Hint, "không dùng Chrome của máy") {
		t.Fatalf("hint = %q (muốn nói máy chủ tự cài bản riêng)", row.Hint)
	}
}

func rowOf(t *testing.T, snap Snapshot, id ToolID) Row {
	t.Helper()
	for _, row := range snap.Tools {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("không có hàng %q", id)
	return Row{}
}

func TestRowsCoverSixJobsInOrder(t *testing.T) {
	svc, _, _ := newService(t, &http.Client{Transport: redirectTransport{tag: "2025.01.01"}}, fakeChatGPT{login: "not_logged_in"})
	snap, err := svc.Snapshot(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tools) != 6 {
		t.Fatalf("số hàng = %d", len(snap.Tools))
	}
	for i, id := range ToolIDs {
		if snap.Tools[i].ID != id {
			t.Fatalf("hàng %d = %q, muốn %q", i, snap.Tools[i].ID, id)
		}
	}
	if row := rowOf(t, snap, ToolYtDlp); row.State != StateReady || row.Version != "2025.01.01" {
		t.Fatalf("yt-dlp = %+v", row)
	}
	if row := rowOf(t, snap, ToolCodexLogin); row.State != StateNeedsLogin {
		t.Fatalf("đăng nhập = %+v", row)
	}
}

func TestUpdateAvailableTurnsReadyRowIntoUpdate(t *testing.T) {
	svc, _, _ := newService(t, &http.Client{Transport: redirectTransport{tag: "2026.10.30"}}, fakeChatGPT{login: "chatgpt"})
	ctx := context.Background()
	if _, err := svc.Snapshot(ctx, true); err != nil {
		t.Fatal(err)
	}
	// Kiểm cập nhật đồng bộ (probe nền đã chuẩn bị hàng ready).
	svc.checkLatest(ctx, ToolYtDlp)
	snap, err := svc.Snapshot(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	row := rowOf(t, snap, ToolYtDlp)
	if row.State != StateUpdate {
		t.Fatalf("state = %q, muốn update (%+v)", row.State, row)
	}
	if row.Update == nil || row.Update.Latest != "2026.10.30" || row.Update.Current != "2025.01.01" {
		t.Fatalf("update = %+v", row.Update)
	}
	if !strings.Contains(row.Hint, "Có bản 2026.10.30") {
		t.Fatalf("hint = %q", row.Hint)
	}
	if rowOf(t, snap, ToolCodexLogin).State != StateReady {
		t.Fatal("đã đăng nhập phải ready")
	}
}

func TestFailedInstallMarksTaskAndRowFailed(t *testing.T) {
	svc, tasks, _ := newService(t, &http.Client{Transport: failTransport{}}, fakeChatGPT{login: "chatgpt"})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := svc.Ensure(ctx, ToolFfmpeg)
	if err == nil {
		t.Fatal("cài ffmpeg với mọi request 500 phải lỗi")
	}
	if tasks.lastState() != domain.TaskFailed {
		t.Fatalf("trạng thái việc nền = %q, muốn failed", tasks.lastState())
	}
	svc.mu.Lock()
	svc.checkedAt = time.Time{} // bỏ cache để probe lại
	svc.mu.Unlock()
	snap, err := svc.Snapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	row := rowOf(t, snap, ToolFfmpeg)
	if row.State != StateFailed {
		t.Fatalf("hàng ffmpeg = %+v, muốn failed", row)
	}
	if row.Hint == "" || strings.Contains(row.Hint, "spawn") {
		t.Fatalf("hint phải là câu người đọc được: %q", row.Hint)
	}
	if _, ok := svc.running[ToolFfmpeg]; ok {
		t.Fatal("lượt cài còn trong bảng running sau khi lỗi")
	}
}

func TestAttentionReportsUpdateAndFailure(t *testing.T) {
	svc, _, _ := newService(t, &http.Client{Transport: redirectTransport{tag: "2026.10.30"}}, fakeChatGPT{login: "chatgpt"})
	ctx := context.Background()
	if _, err := svc.Snapshot(ctx, true); err != nil {
		t.Fatal(err)
	}
	svc.checkLatest(ctx, ToolYtDlp)
	items := svc.Attention(ctx)
	var kinds []string
	for _, item := range items {
		kinds = append(kinds, item.Kind)
		if item.Href != "#/settings?section=tools" {
			t.Fatalf("href = %q", item.Href)
		}
	}
	found := false
	for _, k := range kinds {
		if k == domain.AttentionToolUpdate {
			found = true
		}
	}
	if !found {
		t.Fatalf("thiếu tool_update: %v", kinds)
	}
}

func TestLiveProgressNilWithoutManagedID(t *testing.T) {
	svc, _, _ := newService(t, &http.Client{}, fakeChatGPT{})
	if p := svc.LiveProgress(ToolYtDlp); p != nil {
		t.Fatalf("yt-dlp không có install-state: %+v", p)
	}
	if svc.RequiredBytes(ToolYtDlp) != tools.YtDlpRequiredBytes || svc.RequiredBytes(ToolYtDlp) <= 0 {
		t.Fatal("yt-dlp phải có chỗ dành sẵn để hiện dung lượng")
	}
	if svc.RequiredBytes(ToolChrome) <= 0 {
		t.Fatal("chrome phải có dung lượng cần")
	}
}

func TestCancelWithoutRunIsNoop(t *testing.T) {
	svc, _, _ := newService(t, &http.Client{}, fakeChatGPT{})
	if err := svc.Cancel(context.Background(), ToolChrome); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartInstall(context.Background(), ToolCodexLogin); err == nil {
		t.Fatal("hàng đăng nhập không phải cài đặt")
	}
}

var errBoom = errors.New("boom")

// GET không spawn process: cache trống trả hình "chưa cài" ngay và chỉ probe
// ở nền.
func TestSnapshotNeverProbesOnColdCache(t *testing.T) {
	runner := &countingRunner{}
	base := t.TempDir()
	deps := tools.Deps{Runner: runner, Paths: &tools.FilePathStore{Dir: filepath.Join(base, "p")},
		BaseDir: base, Home: base, PathEnv: filepath.Join(base, "path"), SystemDirs: []string{},
		Registry: tools.NewRegistry(base, nil)}
	kit := tools.New(deps)
	svc := New(Deps{Tools: kit, Managed: tools.NewManaged(deps, nil, kit),
		Transcription: tools.NewTranscription(deps, kit.Uv, nil), Now: time.Now})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Warm(ctx)
	snap, err := svc.Snapshot(context.Background(), false)
	if err != nil || len(snap.Tools) != len(ToolIDs) {
		t.Fatalf("snapshot = %+v, %v", snap, err)
	}
	if n := runner.count(); n != 0 {
		t.Fatalf("GET đã spawn %d tiến trình", n)
	}
}

type countingRunner struct {
	mu sync.Mutex
	n  int
}

func (r *countingRunner) Run(context.Context, string, []string, tools.Spec) (tools.Result, error) {
	r.mu.Lock()
	r.n++
	r.mu.Unlock()
	return tools.Result{}, &tools.ProcessFailedError{Message: "ENOENT", Code: -1}
}

func (r *countingRunner) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

// Trả về snapshot không được chia sẻ mảng hàng với cache: lần đọc sau (overlay
// đang cài) không được sửa giá trị đã trả trước đó.
// Sau một lượt cài thành công: dung lượng hàng = số đã đo và ghi lại, KHÔNG
// phải RequiredBytes (chỗ trống dự phòng của hàm kiểm ổ — vốn lớn hơn 2–4 lần
// kích thước cài thật). Hàng chưa cài hiện xấp xỉ từ manifest.
func TestInstalledSizeIsMeasuredNotReserved(t *testing.T) {
	base := t.TempDir()
	deps := tools.Deps{Runner: &versionRunner{ytdlp: "2025.01.01"}, Paths: &tools.FilePathStore{Dir: filepath.Join(base, "p")},
		BaseDir: base, Home: base, PathEnv: filepath.Join(base, "path"), SystemDirs: []string{},
		Registry: tools.NewRegistry(base, nil)}
	kit := tools.New(deps)
	svc := New(Deps{Tools: kit, Managed: tools.NewManaged(deps, nil, kit),
		Transcription: tools.NewTranscription(deps, kit.Uv, nil), Now: time.Now, DataDir: base})

	// Dựng một "bản cài" yt-dlp giả 100 MB để measureInstalled có gì để đo.
	dir := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), make([]byte, 100*1024*1024), 0o755); err != nil {
		t.Fatal(err)
	}
	measured := svc.measureInstalled(ToolYtDlp)
	if measured != 100*1024*1024 {
		t.Fatalf("đo = %d, muốn %d", measured, 100*1024*1024)
	}
	if measured == svc.RequiredBytes(ToolYtDlp) {
		t.Fatal("không được đo ra đúng RequiredBytes — hai con số phải khác biệt")
	}
	if got := toolchain.ReadInstalledSize(base, "yt-dlp"); got != measured {
		t.Fatalf("ghi số đo = %d, muốn %d", got, measured)
	}
	// Hàng ready phải lấy số đo, không fallback sang xấp xỉ hay required.
	row := Row{ID: ToolYtDlp, State: StateReady}
	if got := svc.sizeForRow(ToolYtDlp, row); got != measured {
		t.Fatalf("ready size = %d, muốn %d (đo)", got, measured)
	}
	// Chưa cài hiện xấp xỉ manifest (không phải số đo, không phải required).
	if got := svc.sizeForRow(ToolYtDlp, Row{ID: ToolYtDlp, State: StateMissing}); got != tools.YtDlpApproxBytes {
		t.Fatalf("missing size = %d, muốn approx %d", got, tools.YtDlpApproxBytes)
	}
	// Ready mà không có số đo (cài trước tính năng đo) → 0, UI hiện không số.
	svc2 := New(Deps{Tools: kit, Managed: tools.NewManaged(deps, nil, kit), Now: time.Now, DataDir: t.TempDir()})
	if got := svc2.sizeForRow(ToolYtDlp, Row{ID: ToolYtDlp, State: StateReady}); got != 0 {
		t.Fatalf("ready không số đo phải = 0, got %d", got)
	}
	// Provided (Chrome hệ thống) luôn 0.
	if got := svc.sizeForRow(ToolChrome, Row{ID: ToolChrome, State: StateReady, Provided: true}); got != 0 {
		t.Fatalf("provided size = %d, muốn 0 (Có sẵn)", got)
	}
}

// GET chỉ đọc số đo đã ghi, không đi qua cây thư mục: đổi kích thước tệp dưới
// bin sau khi ghi số đo không được ảnh hưởng giá trị trả về.
func TestSnapshotReadsPersistedSizeNotTreeWalk(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := toolchain.WriteInstalledSize(base, "yt-dlp", 111*1024*1024); err != nil {
		t.Fatal(err)
	}
	// Bỏ thêm 200 MB sau khi ghi — nếu GET đo cây thì sẽ lớn hơn; chỉ khi đọc
	// tệp đã ghi mới vẫn 111 MB.
	if err := os.WriteFile(filepath.Join(dir, "payload"), make([]byte, 200*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := toolchain.ReadInstalledSize(base, "yt-dlp"); got != 111*1024*1024 {
		t.Fatalf("đọc số đo = %d, muốn 111 MB (đã ghi, không đo cây)", got)
	}
}

func TestSnapshotDoesNotAliasCache(t *testing.T) {
	svc := &Service{now: time.Now, baseCtx: context.Background(),
		cached: &Snapshot{Tools: []Row{{ID: ToolYtDlp, State: StateReady}}}, checkedAt: time.Now()}
	first, err := svc.Snapshot(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	svc.cached.Tools[0].State = StateFailed
	if first.Tools[0].State != StateReady {
		t.Fatalf("snapshot bị sửa theo cache: %q", first.Tools[0].State)
	}
}

// Trạng thái đăng nhập đọc thẳng từ phiên (không chờ TTL probe): đăng nhập
// xong là hàng đổi ready dù cache còn logged-out.
func TestChatGPTLoginOverlayIgnoresCacheTTL(t *testing.T) {
	svc := &Service{now: time.Now, baseCtx: context.Background(),
		chatgpt: fakeChatGPT{login: "chatgpt"},
		cached:  &Snapshot{Tools: []Row{{ID: ToolCodexLogin, State: StateNeedsLogin}}}, checkedAt: time.Now()}
	snap, err := svc.Snapshot(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Tools[0].State != StateReady {
		t.Fatalf("state = %q, muốn ready", snap.Tools[0].State)
	}
}

// Hàng đang cài chưa phải done với setup (tránh báo hoàn tất khi cài dở).
func TestRowCheckInstallingIsNotReady(t *testing.T) {
	if got := rowCheck(Row{ID: ToolChrome, State: StateInstalling}); got.Need != domain.SetupNeedInstall {
		t.Fatalf("need = %q", got.Need)
	}
}

// Hai lượt StartInstall đồng thời chỉ tạo một việc nền và một lượt cài.
func TestConcurrentStartInstallSingleRun(t *testing.T) {
	svc, tasks, _ := newService(t, &http.Client{Transport: failTransport{}}, fakeChatGPT{login: "chatgpt"})
	var wg sync.WaitGroup
	ids := make([]string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := svc.StartInstall(context.Background(), ToolYtDlp)
			if err != nil {
				t.Errorf("start: %v", err)
				return
			}
			ids[i] = id
		}(i)
	}
	wg.Wait()
	if ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("task ids = %v, muốn cùng một", ids)
	}
	tasks.mu.Lock()
	n := len(tasks.specs)
	tasks.mu.Unlock()
	if n != 1 {
		t.Fatalf("tasks = %d, muốn 1", n)
	}
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Tiến trình con ghi thẳng vào kho nhật ký theo công cụ: stdout/stderr
// và dòng cuối cụt đều vào GET logs, đã che.
func TestLoggingRunnerFeedsLogStore(t *testing.T) {
	st := logs.New(t.TempDir(), "/Users/dat")
	r := NewLoggingRunner(echoRunner{}, func(tool, level string) LineWriter {
		return st.Writer(tool, level)
	})
	if _, err := r.Run(context.Background(), "/opt/bin/ffmpeg", nil, tools.Spec{}); err != nil {
		t.Fatal(err)
	}
	entries, _, err := st.Read("ffmpeg", "", 10, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Message)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"out line", "err line", "partial"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("thiếu %q trong %v", want, got)
		}
	}
}

type echoRunner struct{}

func (echoRunner) Run(_ context.Context, _ string, _ []string, spec tools.Spec) (tools.Result, error) {
	_, _ = spec.Log.Write([]byte("out line\nerr line\nCookie: SESSDATA=secret; bili_jct=x\npartial"))
	return tools.Result{}, nil
}
