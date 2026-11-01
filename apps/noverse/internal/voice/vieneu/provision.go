package vieneu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/platform/toolchain"
)

// provisionRequiredBytes là chỗ trống tối thiểu trước khi chuẩn bị bộ đọc
// (mã nguồn + môi trường Python + mô hình giọng đọc). Thấp hơn thì dừng ngay
// thay vì tải hàng trăm MB rồi hỏng giữa chừng.
const provisionRequiredBytes int64 = 4 << 30

// provisionStateFile là trạng thái cài của bộ đọc, nằm cạnh src/ trong thư mục
// engine. Tag trong đây quyết định lượt chuẩn bị sau có phải làm lại không.
const provisionStateFile = "install-state.json"

// provisionLockFile là khoá một-lượt-ghi cho cả thư mục voice: worker và nút
// "Cài bộ đọc trên máy" (hai process khác nhau) không được cùng tải/dựng.
const provisionLockFile = ".install.lock"

// provisionPythonVersion là bản Python mà VieNeu v3.8.3 khoá trong uv.lock.
const provisionPythonVersion = "3.12"

// installState là nội dung install-state.json.
type installState struct {
	Tag           string    `json:"tag"`
	UVVersion     string    `json:"uvVersion,omitempty"`
	PythonVersion string    `json:"pythonVersion,omitempty"`
	InstalledAt   time.Time `json:"installedAt"`
	Bytes         int64     `json:"bytes,omitempty"`
	State         string    `json:"state"`
	// ModelReady chỉ được đặt sau khi máy đọc đã qua /health với mô hình đầy
	// đủ trên đĩa. hf/ có tệp chưa đủ: lượt tải dở cũng để lại tệp.
	ModelReady bool `json:"modelReady,omitempty"`
}

// provisionDeps là các điểm tiêm cho test: lượt chạy thật dùng binary/mạng,
// test thay bằng hàm giả để kiểm thứ tự, khoá và báo lỗi.
type provisionDeps struct {
	ensureTools  func(r *provisionRun, ctx context.Context) error
	ensurePython func(r *provisionRun, ctx context.Context) error
	syncSource   func(r *provisionRun, ctx context.Context) error
	syncDeps     func(r *provisionRun, ctx context.Context) error
	startModel   func(r *provisionRun, ctx context.Context) error
	freeDisk     func(dir string) int64
}

func defaultProvisionDeps() *provisionDeps {
	return &provisionDeps{
		ensureTools:  (*provisionRun).ensureTools,
		ensurePython: (*provisionRun).ensurePython,
		syncSource:   (*provisionRun).syncSource,
		syncDeps:     (*provisionRun).syncDeps,
		startModel:   (*provisionRun).startModel,
		freeDisk:     toolchain.FreeDiskBytes,
	}
}

// provisionRun là một lượt chuẩn bị: giữ mốc phần trăm đã phát (để không bao
// giờ lùi) và các hàm của lượt.
type provisionRun struct {
	e      *Engine
	deps   *provisionDeps
	report voice.ProvisionReporter
	uv     string
	last   int
}

// Provision chuẩn bị bộ đọc để worker dùng được: uv ghim, Python 3.12, mã
// nguồn đúng tag, thư viện Python, mô hình. Một lượt chạy đúng một lần thử —
// thử lại/giãn cách là việc của worker.
//
// Idempotent: đã cài đúng tag ghim thì trả về ngay (<50ms) mà không chạm
// mạng. Mọi lượt chạy đều giữ khoá ~/.ainovel/voice/.install.lock, nên lượt
// thứ hai chờ lượt đầu rồi tự nhận ra đã xong. Tag đổi thì chỉ làm lại mã
// nguồn + thư viện, giữ nguyên hf/ (mô hình) và voices/ (giọng đã nạp).
func (e *Engine) Provision(ctx context.Context, report voice.ProvisionReporter) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if report == nil {
		report = func(voice.ProvisionState) {}
	}
	deps := e.prov
	if deps == nil {
		deps = defaultProvisionDeps()
	}
	r := &provisionRun{e: e, deps: deps, report: report, last: -1}
	return r.run(ctx)
}

func (r *provisionRun) run(ctx context.Context) error {
	e := r.e
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		return r.fail(ctx, voice.ProvisionErrUVFailed, err)
	}
	if err := os.MkdirAll(e.cfg.Home, 0o755); err != nil {
		return r.fail(ctx, voice.ProvisionErrUVFailed, err)
	}
	lock := flock.New(filepath.Join(e.cfg.Home, provisionLockFile))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return r.fail(ctx, voice.ProvisionErrStartFailed, err)
	}
	if !locked {
		return r.fail(ctx, voice.ProvisionErrStartFailed, errors.New("không giữ được khoá cài bộ đọc"))
	}
	defer func() { _ = lock.Unlock() }()

	state := e.readInstallState()
	installed := e.Installed()
	atPin := state != nil && state.Tag == pinnedTag && state.State == "installed"
	modelReady := state != nil && state.ModelReady && dirHasEntries(e.hfHome())
	if installed && atPin && modelReady {
		r.emit(voice.ProvisionStateReady, "", 100, "Bộ đọc đã sẵn sàng")
		return nil
	}

	if free := r.deps.freeDisk(e.dir); free >= 0 && free < provisionRequiredBytes {
		return r.fail(ctx, voice.ProvisionErrNoDisk,
			fmt.Errorf("cần ít nhất 4 GB trống để cài bộ đọc, chỉ còn %.1f GB", float64(free)/(1<<30)))
	}

	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, 0, "Đang chuẩn bị công cụ tải (uv)…")
	if err := r.deps.ensureTools(r, ctx); err != nil {
		return r.fail(ctx, voice.ProvisionErrUVFailed, err)
	}

	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepPython, 8, "Đang chuẩn bị Python "+provisionPythonVersion+"…")
	if err := r.deps.ensurePython(r, ctx); err != nil {
		return r.fail(ctx, voice.ProvisionErrUVFailed, err)
	}

	if !installed || !atPin {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, 15, "Đang tải mã nguồn VieNeu-TTS…")
		if err := r.deps.syncSource(r, ctx); err != nil {
			return r.fail(ctx, sourceErrorCode(err), err)
		}
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, 25, "Đang cài thư viện Python (lần đầu vài phút)…")
		if err := r.deps.syncDeps(r, ctx); err != nil {
			return r.fail(ctx, voice.ProvisionErrDepsFailed, err)
		}
	} else {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, 15, "Mã nguồn đã đúng bản ghim")
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, 65, "Thư viện Python đã sẵn sàng")
	}

	if !modelReady {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepModel, 65, "Đang tải mô hình giọng đọc (~600 MB)…")
		if err := r.deps.startModel(r, ctx); err != nil {
			return r.fail(ctx, voice.ProvisionErrModelFailed, err)
		}
	} else {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepModel, 99, "Mô hình đã có sẵn trên máy")
	}

	st := installState{
		Tag:           pinnedTag,
		UVVersion:     toolchain.UvPin().Version,
		PythonVersion: provisionPythonVersion,
		InstalledAt:   time.Now().UTC(),
		Bytes:         dirSize(e.dir),
		State:         "installed",
		ModelReady:    true,
	}
	if err := e.writeInstallState(st); err != nil {
		return r.fail(ctx, voice.ProvisionErrUVFailed, err)
	}
	r.emit(voice.ProvisionStateReady, "", 100, "Bộ đọc đã sẵn sàng")
	return nil
}

// sourceErrorCode phân biệt lệch hash (dấu hiệu bảo mật, không thử lại) với
// lỗi mạng thường.
func sourceErrorCode(err error) string {
	var ce *toolchain.ChecksumError
	if errors.As(err, &ce) {
		return voice.ProvisionErrHashMismatch
	}
	return voice.ProvisionErrDownloadFailed
}

// emit phát một mốc trạng thái. Phần trăm chỉ tăng: mốc sau thấp hơn mốc
// trước bị kéo lên, còn -1 (không đo được) không làm đổi mốc đã phát.
func (r *provisionRun) emit(state, step string, percent int, msg string) {
	if percent >= 0 {
		if percent < r.last {
			percent = r.last
		}
		r.last = percent
	}
	r.report(voice.ProvisionState{
		State:     state,
		Step:      step,
		Percent:   percent,
		Message:   msg,
		Engine:    r.e.Name(),
		Model:     r.e.cfg.Quality,
		Version:   pinnedTag,
		UpdatedAt: time.Now().UTC(),
	})
}

// fail phát trạng thái lỗi và trả lỗi mang mã. Huỷ của ctx được giữ trong
// chuỗi lỗi để tầng trên (Install/worker) nhận ra bằng errors.Is.
func (r *provisionRun) fail(ctx context.Context, code string, err error) error {
	pe := &voice.ProvisionError{Code: code, Message: err.Error(), LogTail: r.e.logTail(6)}
	r.report(voice.ProvisionState{
		State:     voice.ProvisionStateError,
		Step:      "",
		Percent:   -1,
		Message:   err.Error(),
		Error:     pe,
		Engine:    r.e.Name(),
		Model:     r.e.cfg.Quality,
		Version:   pinnedTag,
		UpdatedAt: time.Now().UTC(),
	})
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("%w: %w", cerr, pe)
	}
	return pe
}

// readInstallState đọc install-state.json; nil khi vắng/hỏng (coi như chưa rõ
// bản đang cài, lượt sau sẽ làm lại mã nguồn + thư viện).
func (e *Engine) readInstallState() *installState {
	data, err := os.ReadFile(filepath.Join(e.dir, provisionStateFile))
	if err != nil {
		return nil
	}
	var st installState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	return &st
}

// writeInstallState ghi tạm rồi đổi tên để không bao giờ đọc phải tệp dở.
func (e *Engine) writeInstallState(st installState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := filepath.Join(e.dir, fmt.Sprintf(".install-state-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(e.dir, provisionStateFile))
}

// ensureTools cài uv ghim vào <home>/bin/uv (app quản), không cần uv hệ thống
// và không cần brew.
func (r *provisionRun) ensureTools(ctx context.Context) error {
	home := r.e.cfg.Home
	if exe := toolchain.UvPath(home); exe != "" {
		r.uv = exe
		return nil
	}
	spec := toolchain.UvInstallSpec(nil)
	selfCheck := spec.SelfCheck
	spec.SelfCheck = func(ctx context.Context, ic *toolchain.InstallContext, staging string) (string, error) {
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, 4, "Đang cài uv (bộ tải Python)…")
		return selfCheck(ctx, ic, staging)
	}
	inst := &toolchain.Installer{BaseDir: home}
	if _, err := inst.Install(ctx, spec); err != nil {
		return fmt.Errorf("cài uv thất bại: %w", err)
	}
	exe := toolchain.UvPath(home)
	if exe == "" {
		return errors.New("cài uv xong nhưng không tìm thấy binary")
	}
	r.uv = exe
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepTools, 8, "Đã có uv "+toolchain.UvPin().Version)
	return nil
}

// ensurePython cài CPython 3.12 vào <home>/python bằng uv app quản.
func (r *provisionRun) ensurePython(ctx context.Context) error {
	home := r.e.cfg.Home
	if err := os.MkdirAll(filepath.Join(home, "python"), 0o755); err != nil {
		return err
	}
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepPython, 10, "Đang tải Python "+provisionPythonVersion+"…")
	if err := r.runUV(ctx, home, nil, "python", "install", provisionPythonVersion); err != nil {
		return fmt.Errorf("cài Python %s thất bại: %w · %s", provisionPythonVersion, err, r.e.logTail(4))
	}
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepPython, 15, "Python "+provisionPythonVersion+" đã sẵn sàng")
	return nil
}

// syncSource đưa <home>/vieneu/src về đúng tag ghim: tarball codeload có hash
// ghim trước (không cần git); codeload hỏng mới quay về git clone.
func (r *provisionRun) syncSource(ctx context.Context) error {
	dlDir := filepath.Join(r.e.dir, "downloads")
	if err := os.MkdirAll(dlDir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dlDir, sourcePin.Name)
	inst := &toolchain.Installer{BaseDir: r.e.cfg.Home}
	tick := func(done int64) error {
		pct := 15
		if sourcePin.SizeBytes > 0 {
			pct = 15 + int(min(done*10/sourcePin.SizeBytes, 10))
		}
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, pct, "Đang tải mã nguồn VieNeu-TTS…")
		return nil
	}
	if err := inst.DownloadVerified(ctx, sourcePin, archive, tick); err != nil {
		var ce *toolchain.ChecksumError
		if errors.As(err, &ce) || ctx.Err() != nil {
			return err
		}
		r.e.cfg.Logf("vieneu: tải tarball mã nguồn thất bại (%v) — thử git clone", err)
		return r.cloneSource(ctx)
	}
	return r.installSourceArchive(archive)
}

// installSourceArchive bung tarball vào thư mục tạm rồi đổi tên nguyên tử vào
// src/, nên src/ không bao giờ là cây nửa vời. hf/ và voices/ là thư mục anh
// em, không bị chạm.
func (r *provisionRun) installSourceArchive(archive string) error {
	src := r.e.srcDir()
	parent := filepath.Dir(src)
	staging, err := os.MkdirTemp(parent, ".src-staging-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	if err := toolchain.ExtractArchive(toolchain.AssetTarGZ, archive, staging, ""); err != nil {
		return fmt.Errorf("bung mã nguồn thất bại: %w", err)
	}
	root := staging
	entries, err := os.ReadDir(staging)
	if err != nil {
		return err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		root = filepath.Join(staging, entries[0].Name())
	}
	old := ""
	if _, err := os.Stat(src); err == nil {
		old = src + ".old-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if err := os.Rename(src, old); err != nil {
			return err
		}
	}
	if err := os.Rename(root, src); err != nil {
		if old != "" {
			_ = os.Rename(old, src)
		}
		return err
	}
	if old != "" {
		_ = os.RemoveAll(old)
	}
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, 25, "Mã nguồn VieNeu-TTS "+pinnedTag+" đã sẵn sàng")
	return nil
}

// cloneSource là đường dự phòng khi codeload không tải được: giữ nguyên hành
// vi git trước đây (fetch/checkout nếu đã có .git, ngược lại clone nông).
func (r *provisionRun) cloneSource(ctx context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("không tải được mã nguồn VieNeu-TTS: codeload lỗi và máy chưa có git (macOS: chạy `xcode-select --install`)")
	}
	src := r.e.srcDir()
	if _, err := os.Stat(filepath.Join(src, ".git")); err == nil {
		if err := runQuiet(ctx, src, r.e.logPath(), "git", "fetch", "--depth", "1", "origin", "refs/tags/"+pinnedTag); err == nil {
			if err := runQuiet(ctx, src, r.e.logPath(), "git", "checkout", "-f", "FETCH_HEAD"); err == nil {
				r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, 25, "Mã nguồn VieNeu-TTS "+pinnedTag+" đã sẵn sàng")
				return nil
			}
		}
	}
	if err := os.RemoveAll(src); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		return err
	}
	if err := runQuiet(ctx, filepath.Dir(src), r.e.logPath(), "git",
		"clone", "--depth", "1", "--branch", pinnedTag, repoURL, src); err != nil {
		return fmt.Errorf("tải mã nguồn VieNeu-TTS thất bại: %w", err)
	}
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepSource, 25, "Mã nguồn VieNeu-TTS "+pinnedTag+" đã sẵn sàng")
	return nil
}

// syncDeps cài thư viện Python đúng uv.lock vào src/.venv.
func (r *provisionRun) syncDeps(ctx context.Context) error {
	src := r.e.srcDir()
	if err := os.MkdirAll(src, 0o755); err != nil {
		return err
	}
	pct := 25
	err := r.runUV(ctx, src, func(line string) {
		if pct < 63 {
			pct += 2
		}
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, pct, "Đang cài thư viện: "+clip(line, 90))
	}, "sync", "--frozen", "--python", provisionPythonVersion)
	if err != nil {
		return fmt.Errorf("cài thư viện Python thất bại: %w · %s", err, r.e.logTail(4))
	}
	r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepDeps, 65, "Đã cài xong thư viện Python")
	return nil
}

// startModel bật máy đọc; lần đầu máy đọc tự tải mô hình về hf/ và /health chỉ
// trả ok sau khi mô hình sẵn sàng.
func (r *provisionRun) startModel(ctx context.Context) error {
	_, err := r.e.ensureStarted(ctx, r.e.cfg.Quality, func(bytes int64) {
		pct := 65 + int(min(bytes*34/modelBytes, 34))
		r.emit(voice.ProvisionStateProvisioning, voice.ProvisionStepModel, pct,
			fmt.Sprintf("Đang tải mô hình giọng đọc… (%.0f MB)", float64(bytes)/(1<<20)))
	})
	if err != nil {
		return fmt.Errorf("khởi động bộ đọc thất bại: %w · %s", err, r.e.logTail(6))
	}
	return nil
}

// runUV chạy binary uv app quản với môi trường UV_* trỏ vào thư mục dữ liệu
// của app (không đụng ~/.local hay cache người dùng), ghi log và đẩy từng
// dòng ra onLine.
func (r *provisionRun) runUV(ctx context.Context, dir string, onLine func(string), args ...string) error {
	if r.uv == "" {
		return errors.New("chưa có uv")
	}
	return runStreamingEnv(ctx, dir, r.uv, args, r.e.logPath(), uvEnv(r.e.cfg.Home), onLine)
}

// uvEnv là môi trường chạy uv: môi trường process hiện tại với các khoá UV_*
// bị thay bằng giá trị app-owned (biến trùng khoá trong env làm hành vi phụ
// thuộc libc, nên xoá trước khi thêm).
func uvEnv(home string) []string {
	owned := toolchain.UvEnv(home)
	out := make([]string, 0, len(os.Environ())+len(owned))
	for _, kv := range os.Environ() {
		key, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, replaced := owned[key]; replaced {
				continue
			}
		}
		out = append(out, kv)
	}
	for key, value := range owned {
		out = append(out, key+"="+value)
	}
	return out
}

// dirHasEntries đúng khi dir tồn tại và có ít nhất một mục con.
func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}
