package vieneu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/toolchain"
)

// installFixture dựng một bản cài trọn vẹn (mã nguồn + venv + mô hình + state).
func installFixture(t *testing.T, e *Engine, tag string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(e.srcDir(), "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.srcDir(), "apps", "openai_speech.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(e.venvPython()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.venvPython(), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.hfHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.hfHome(), "model.onnx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.writeInstallState(installState{Tag: tag, State: "installed", ModelReady: true, InstalledAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

// noopDeps là bộ bước giả: không mạng, không binary, đếm được số lần chạy.
func noopDeps() *provisionDeps {
	noop := func(*provisionRun, context.Context) error { return nil }
	return &provisionDeps{
		ensureTools:  noop,
		ensurePython: noop,
		syncSource:   noop,
		syncDeps:     noop,
		startModel:   noop,
		freeDisk:     func(string) int64 { return 1 << 40 },
	}
}

type stateLog struct {
	mu     sync.Mutex
	states []voice.ProvisionState
}

func (l *stateLog) report(st voice.ProvisionState) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.states = append(l.states, st)
}

func (l *stateLog) all() []voice.ProvisionState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]voice.ProvisionState(nil), l.states...)
}

func (l *stateLog) last() voice.ProvisionState {
	all := l.all()
	if len(all) == 0 {
		return voice.ProvisionState{}
	}
	return all[len(all)-1]
}

func provisionError(t *testing.T, err error) *voice.ProvisionError {
	t.Helper()
	var pe *voice.ProvisionError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, muốn *voice.ProvisionError", err)
	}
	return pe
}

func TestProvisionNoOpWhenInstalledAtPinnedTag(t *testing.T) {
	e := testEngine(t)
	installFixture(t, e, pinnedTag)
	deps := noopDeps()
	var calls int32
	count := func(*provisionRun, context.Context) error { atomic.AddInt32(&calls, 1); return nil }
	deps.ensureTools, deps.ensurePython = count, count
	deps.syncSource, deps.syncDeps, deps.startModel = count, count, count
	e.prov = deps
	log := &stateLog{}

	start := time.Now()
	if err := e.Provision(context.Background(), log.report); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("no-op mất %v, muốn < 50ms", elapsed)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("đã cài đúng bản ghim mà vẫn chạy %d bước", got)
	}
	last := log.last()
	if last.State != voice.ProvisionStateReady || last.Percent != 100 {
		t.Fatalf("mốc cuối = %+v, muốn ready/100", last)
	}
}

func TestProvisionRepairsPartialModelDownload(t *testing.T) {
	e := testEngine(t)
	installFixture(t, e, pinnedTag)
	// Lượt tải dở để lại tệp trong hf/ nhưng chưa qua /health: state không có
	// modelReady, nên lượt sau phải bật máy đọc lại chứ không được coi là xong.
	if err := e.writeInstallState(installState{Tag: pinnedTag, State: "installed", InstalledAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	deps := noopDeps()
	var modelRan int32
	deps.startModel = func(*provisionRun, context.Context) error { atomic.AddInt32(&modelRan, 1); return nil }
	e.prov = deps
	if err := e.Provision(context.Background(), nil); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if modelRan != 1 {
		t.Fatalf("model chạy %d lần, muốn 1 (tải dở phải được sửa)", modelRan)
	}
	st := e.readInstallState()
	if st == nil || !st.ModelReady {
		t.Fatalf("install-state = %+v, muốn modelReady", st)
	}
}

func TestProvisionSerialisesConcurrentRuns(t *testing.T) {
	e := testEngine(t)
	var active, peak, runs int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	deps := noopDeps()
	deps.ensureTools = func(*provisionRun, context.Context) error {
		n := atomic.AddInt32(&active, 1)
		atomic.AddInt32(&runs, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		atomic.AddInt32(&active, -1)
		return nil
	}
	e.prov = deps

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			errs[k] = e.Provision(context.Background(), nil)
		}(i)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("lượt đầu không vào được bước tools")
	}
	select {
	case <-entered:
		t.Fatal("lượt thứ hai chạy song song — khoá cài không giữ")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("Provision: %v", err)
		}
	}
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Fatalf("số lượt chạy song song tối đa = %d, muốn 1", got)
	}
	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("số lượt chạy = %d, muốn 2 (lượt hai chờ khoá rồi tự chạy)", got)
	}
}

func TestProvisionTagChangeKeepsModelAndVoices(t *testing.T) {
	e := testEngine(t)
	installFixture(t, e, "v0.0.1")
	voiceFile := filepath.Join(e.voicesDir(), "giong.json")
	if err := os.MkdirAll(e.voicesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voiceFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := noopDeps()
	var sourceRan, depsRan, modelRan int32
	deps.syncSource = func(*provisionRun, context.Context) error { atomic.AddInt32(&sourceRan, 1); return nil }
	deps.syncDeps = func(*provisionRun, context.Context) error { atomic.AddInt32(&depsRan, 1); return nil }
	deps.startModel = func(*provisionRun, context.Context) error { atomic.AddInt32(&modelRan, 1); return nil }
	e.prov = deps

	if err := e.Provision(context.Background(), nil); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if sourceRan != 1 || depsRan != 1 {
		t.Fatalf("source/deps chạy %d/%d lần, muốn 1/1", sourceRan, depsRan)
	}
	if modelRan != 0 {
		t.Fatal("tag đổi không được tải lại mô hình")
	}
	if _, err := os.Stat(filepath.Join(e.hfHome(), "model.onnx")); err != nil {
		t.Fatalf("tag đổi đã xoá mô hình: %v", err)
	}
	if _, err := os.Stat(voiceFile); err != nil {
		t.Fatalf("tag đổi đã xoá giọng đã nạp: %v", err)
	}
	st := e.readInstallState()
	if st == nil || st.Tag != pinnedTag {
		t.Fatalf("install-state = %+v, muốn tag %s", st, pinnedTag)
	}
}

func TestProvisionStopsWhenDiskTight(t *testing.T) {
	e := testEngine(t)
	deps := noopDeps()
	deps.freeDisk = func(string) int64 { return 100 << 20 }
	var ran int32
	deps.ensureTools = func(*provisionRun, context.Context) error { atomic.AddInt32(&ran, 1); return nil }
	e.prov = deps
	log := &stateLog{}

	err := e.Provision(context.Background(), log.report)
	pe := provisionError(t, err)
	if pe.Code != voice.ProvisionErrNoDisk {
		t.Fatalf("code = %q, muốn %q", pe.Code, voice.ProvisionErrNoDisk)
	}
	if ran != 0 {
		t.Fatal("thiếu đĩa mà vẫn chạy bước cài")
	}
	last := log.last()
	if last.State != voice.ProvisionStateError || last.Error == nil || last.Error.Code != voice.ProvisionErrNoDisk {
		t.Fatalf("mốc cuối = %+v, muốn error/no_disk", last)
	}
	if last.Percent != voice.ProvisionPercentUnknown {
		t.Fatalf("percent = %d, muốn -1", last.Percent)
	}
}

// Percent is the percent of the step that is running (10-audio.md §8.1): it
// never goes backwards inside one step, and a new step starts its own count
// instead of inheriting the previous step's number.
func TestProvisionPercentNeverGoesBackwards(t *testing.T) {
	e := testEngine(t)
	deps := noopDeps()
	deps.ensureTools = func(r *provisionRun, _ context.Context) error {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, 3, "a")
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, 6, "b")
		return nil
	}
	deps.syncDeps = func(r *provisionRun, _ context.Context) error {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, 80, "c")
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, 40, "lùi")
		return nil
	}
	deps.startModel = func(r *provisionRun, _ context.Context) error {
		// Bước mới: số của bước trước không được kéo sang.
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepModel, 5, "mẫu giọng")
		return nil
	}
	e.prov = deps
	log := &stateLog{}
	if err := e.Provision(context.Background(), log.report); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	lastByStep := map[string]int{}
	percentByStep := map[string][]int{}
	for _, st := range log.all() {
		if st.Percent < 0 {
			continue
		}
		if stepPrev, ok := lastByStep[st.Step]; ok && st.Percent < stepPrev {
			t.Fatalf("bước %s lùi: %d sau %d (%+v)", st.Step, st.Percent, stepPrev, st)
		}
		lastByStep[st.Step] = st.Percent
		percentByStep[st.Step] = append(percentByStep[st.Step], st.Percent)
	}
	if got := lastByStep[voice.ProvisionStepDeps]; got != 80 {
		t.Fatalf("mốc lùi không bị kéo về mốc trước: deps = %d, muốn 80", got)
	}
	if got := percentByStep[voice.ProvisionStepModel]; len(got) == 0 || got[0] != 5 {
		t.Fatalf("bước mới = %v, muốn bắt đầu từ 5 (không thừa hưởng 80 của bước trước)", got)
	}
	if last := log.last(); last.State != voice.ProvisionStateReady || last.Percent != 100 {
		t.Fatalf("mốc cuối = %+v, muốn ready/100", last)
	}
}

// A step that cannot measure its progress reports -1 and never a made-up 0, so
// the card can draw a spinner instead of a percentage.
func TestProvisionUnmeasurableStepReportsUnknown(t *testing.T) {
	e := testEngine(t)
	deps := noopDeps()
	deps.ensureTools = func(r *provisionRun, _ context.Context) error {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, voice.ProvisionPercentUnknown, "đang chuẩn bị")
		return nil
	}
	e.prov = deps
	log := &stateLog{}
	if err := e.Provision(context.Background(), log.report); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	for _, st := range log.all() {
		if st.Step != voice.ProvisionStepTools || st.State != voice.ProvisionStateProvisioning {
			continue
		}
		if st.Percent != voice.ProvisionPercentUnknown {
			t.Fatalf("bước không đo được = %d, muốn %d", st.Percent, voice.ProvisionPercentUnknown)
		}
		return
	}
	t.Fatal("không có mốc nào của bước công cụ")
}

func TestProvisionHashMismatchLeavesNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("không phải tarball"))
	}))
	defer srv.Close()

	old := sourcePin
	sourcePin = toolchain.PinnedAsset{
		Name:      "vieneu-test.tar.gz",
		URL:       srv.URL + "/src.tar.gz",
		SHA256:    strings.Repeat("ab", 32),
		SizeBytes: int64(len("không phải tarball")),
		Kind:      toolchain.AssetTarGZ,
	}
	defer func() { sourcePin = old }()

	e := testEngine(t)
	deps := noopDeps()
	deps.syncSource = (*provisionRun).syncSource
	e.prov = deps
	log := &stateLog{}

	err := e.Provision(context.Background(), log.report)
	entries, readErr := os.ReadDir(filepath.Join(e.dir, "downloads"))
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		t.Fatalf("lệch hash còn để lại %q", entry.Name())
	}
	if _, statErr := os.Stat(e.srcDir()); !os.IsNotExist(statErr) {
		t.Fatalf("lệch hash mà vẫn có src/: %v", statErr)
	}
	pe := provisionError(t, err)
	if pe.Code != voice.ProvisionErrHashMismatch {
		t.Fatalf("code = %q, muốn %q", pe.Code, voice.ProvisionErrHashMismatch)
	}
	if last := log.last(); last.Error == nil || last.Error.Code != voice.ProvisionErrHashMismatch {
		t.Fatalf("mốc cuối = %+v, muốn error/hash_mismatch", last)
	}
}

func TestInstallRunsProvisionPipeline(t *testing.T) {
	e := testEngine(t)
	e.prov = noopDeps()
	var steps []string
	err := e.Install(context.Background(), func(p voice.InstallProgress) {
		steps = append(steps, p.Step)
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := []string{
		voice.ProvisionStepTools, voice.ProvisionStepPython, voice.ProvisionStepSource,
		voice.ProvisionStepDeps, voice.ProvisionStepModel, "done",
	}
	for _, step := range want {
		found := false
		for _, got := range steps {
			if got == step {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("thiếu bước %q trong %v", step, steps)
		}
	}
}

// fakeUV đặt một binary uv giả vào đúng chỗ app quản (<home>/bin/uv). Nhờ vậy
// Provision thấy "đã cài uv" nên không chạm mạng, còn các bước chạy uv thì
// chạy đúng script này — bài test đi qua đường báo lỗi thật của adapter.
func fakeUV(t *testing.T, home, script string) string {
	t.Helper()
	root := filepath.Join(home, "bin", "uv")
	version := "v-" + toolchain.UvPin().Version
	dir := filepath.Join(root, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, toolchain.UvBinaryName())
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return exe
}

// Lỗi của uv mang đường dẫn, URL và traceback; chúng chỉ được nằm ở LogTail
// (Nâng cao), không bao giờ bị ghép vào Message mà thành viên đọc thấy.
func TestProvisionKeepsUVDiagnosticsOutOfPublicMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script uv giả dùng sh")
	}
	e := testEngine(t)
	fakeUV(t, e.cfg.Home, `#!/bin/sh
case "$1" in
  python) exit 0 ;;
esac
cat >&2 <<'OUT'
Resolved 42 packages in 0.12s
error: Failed to download torch==2.4.0 from https://files.pythonhosted.org/packages/torch.whl
  at /opt/ainovel/voice/vieneu/src/.venv/lib/python3.12/site-packages/torch
Traceback (most recent call last):
OUT
exit 1
`)
	deps := noopDeps()
	deps.ensureTools = (*provisionRun).ensureTools
	deps.syncDeps = (*provisionRun).syncDeps
	e.prov = deps
	log := &stateLog{}

	err := e.Provision(context.Background(), log.report)
	pe := provisionError(t, err)
	if pe.Code != voice.ProvisionErrDepsFailed {
		t.Fatalf("code = %q, muốn %q", pe.Code, voice.ProvisionErrDepsFailed)
	}
	for _, leak := range []string{"/opt/ainovel", "https://files.pythonhosted.org", "Traceback"} {
		if strings.Contains(pe.Message, leak) {
			t.Fatalf("message rò chẩn đoán %q: %q", leak, pe.Message)
		}
	}
	for _, want := range []string{"files.pythonhosted.org", "/opt/ainovel", "Traceback"} {
		if !strings.Contains(pe.LogTail, want) {
			t.Fatalf("logTail thiếu %q: %q", want, pe.LogTail)
		}
	}
	last := log.last()
	if last.State != voice.ProvisionStateError || last.Error == nil {
		t.Fatalf("mốc cuối = %+v, muốn error", last)
	}
	if strings.Contains(last.Message, "Traceback") {
		t.Fatalf("state.Message rò traceback: %q", last.Message)
	}
	if !strings.Contains(last.Error.LogTail, "files.pythonhosted.org") {
		t.Fatalf("state.Error.LogTail thiếu chẩn đoán: %q", last.Error.LogTail)
	}
}

// uv không cho biết tổng số việc của `sync`, nên mỗi dòng output không phải
// một phần trăm: bước thư viện phải luôn là "không đo được" dù output dài.
func TestProvisionDepsProgressIsNeverFabricated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script uv giả dùng sh")
	}
	e := testEngine(t)
	fakeUV(t, e.cfg.Home, `#!/bin/sh
case "$1" in
  python) exit 0 ;;
esac
i=0
while [ $i -lt 40 ]; do
  echo "Downloading package-$i (https://files.pythonhosted.org/p/$i.whl)"
  i=$((i+1))
done
exit 0
`)
	deps := noopDeps()
	deps.ensureTools = (*provisionRun).ensureTools
	deps.syncDeps = (*provisionRun).syncDeps
	e.prov = deps
	log := &stateLog{}

	if err := e.Provision(context.Background(), log.report); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	var depsStates int
	for _, st := range log.all() {
		if st.State != voice.ProvisionStateProvisioning || st.Step != voice.ProvisionStepDeps {
			continue
		}
		depsStates++
		if st.Percent != voice.ProvisionPercentUnknown {
			t.Fatalf("bước thư viện bịa %% từ số dòng output: %+v", st)
		}
	}
	if depsStates < 40 {
		t.Fatalf("chỉ thấy %d mốc bước thư viện, muốn ≥ 40 (output thật phải được báo)", depsStates)
	}
	if last := log.last(); last.State != voice.ProvisionStateReady || last.Percent != 100 {
		t.Fatalf("mốc cuối = %+v, muốn ready/100", last)
	}
}

// Lệch hash của uv phải giữ phân loại hash_mismatch qua chính Provision: worker
// chỉ dừng tự thử lại khi thấy mã này (install_test của toolchain chỉ thử
// Installer, không thử adapter).
func TestProvisionToolsChecksumMapsToHashMismatch(t *testing.T) {
	e := testEngine(t)
	assetPath := filepath.Join(t.TempDir(), "uv.tar.gz")
	if err := os.WriteFile(assetPath, []byte("không phải uv ghim"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := uvInstallSpec
	uvInstallSpec = func(toolchain.UvProber) toolchain.InstallSpec {
		spec := toolchain.UvInstallSpec(nil)
		spec.Pin.Assets = map[toolchain.PlatformKey][]toolchain.PinnedAsset{
			toolchain.CurrentPlatformKey(): {
				{Name: "uv", URL: "file://" + assetPath, SHA256: strings.Repeat("f", 64),
					Kind: toolchain.AssetFile, Executable: true},
			},
		}
		return spec
	}
	defer func() { uvInstallSpec = old }()

	deps := noopDeps()
	deps.ensureTools = (*provisionRun).ensureTools
	e.prov = deps
	log := &stateLog{}

	err := e.Provision(context.Background(), log.report)
	pe := provisionError(t, err)
	if pe.Code != voice.ProvisionErrHashMismatch {
		t.Fatalf("code = %q, muốn %q (lệch hash không được thành uv_failed)", pe.Code, voice.ProvisionErrHashMismatch)
	}
	last := log.last()
	if last.State != voice.ProvisionStateError || last.Error == nil || last.Error.Code != voice.ProvisionErrHashMismatch {
		t.Fatalf("mốc cuối = %+v, muốn error/hash_mismatch", last)
	}
}
