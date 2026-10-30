// Ca test chuyển từ src/server/clone/images.test.ts và
// codex-image-output.test.ts của yt-factory phần vẽ qua Codex CLI cô lập:
// probe biên đọc, biên nhận scratch, capture output theo session, xếp loại
// lỗi quota/forbidden/refused, và đối chiếu phục hồi theo scratch.
package imagegen_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// runnerCall ghi lại một lần Runner.Run.
type runnerCall struct {
	File string
	Args []string
	Spec tools.Spec
}

// fakeRunner: probe `sandbox` trả theo luật biên (đọc được file trong refs,
// chặn auth.json/.env ngoài biên); `exec` chạy handler do test đặt.
type fakeRunner struct {
	calls    []runnerCall
	execN    int
	onExec   func(call runnerCall) (tools.Result, error)
	execArgv [][]string
}

func (r *fakeRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	call := runnerCall{File: file, Args: args, Spec: spec}
	r.calls = append(r.calls, call)
	if len(args) > 0 && args[0] == "sandbox" {
		// Probe cơ học: file cuối argv là mục đọc.
		target := args[len(args)-1]
		readable := filepath.Base(filepath.Dir(target)) == "refs"
		code := 1
		if readable {
			code = 0
		}
		return tools.Result{Code: code}, nil
	}
	r.execN++
	r.execArgv = append(r.execArgv, args)
	if r.onExec == nil {
		return tools.Result{Code: 0}, nil
	}
	return r.onExec(call)
}

// readyStatus dựng CodexStatus "ready" đủ điều kiện requireImageReadiness.
func readyStatus(exe string) tools.CodexStatus {
	return tools.CodexStatus{
		State:  tools.CodexReady,
		Binary: &tools.CodexBinaryInfo{Path: exe, Version: "0.162.1"},
		Login:  tools.CodexLoginChatGPT,
		Isolation: &tools.CodexIsolation{
			State:  tools.IsolationVerified,
			Checks: []tools.CodexIsolationCheck{{Name: "image-tool-allowlist", OK: true}},
		},
	}
}

// codexFixture dựng CodexProvider với runner giả + nguồn token giả.
type codexFixture struct {
	provider *imagegen.CodexProvider
	runner   *fakeRunner
	exe      string
	journal  *memoryJournal
}

// fakeTokens trả token cố định cho Auth của CodexProvider.
func fakeTokens(ctx context.Context) (tools.CodexAuthTokens, error) {
	return tools.CodexAuthTokens{AccessToken: "access-1", AccountID: "acct-1"}, nil
}

func newCodexFixture(t *testing.T, onExec func(call runnerCall) (tools.Result, error)) *codexFixture {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{onExec: onExec}
	st := readyStatus(exe)
	journal := newMemoryJournal()
	p := imagegen.NewCodexProvider(imagegen.CodexDeps{
		Runner:     runner,
		Auth:       fakeTokens,
		StatusFunc: func(ctx context.Context) (tools.CodexStatus, error) { return st, nil },
		PrepareCatalog: func(ctx context.Context, exe, home string) (string, error) {
			f := filepath.Join(home, "image-model-catalog.json")
			return f, os.WriteFile(f, []byte(`{}`), 0o600)
		},
	})
	return &codexFixture{provider: p, runner: runner, exe: exe, journal: journal}
}

// drawThrough gọi service.Draw với provider codex duy nhất (limit tắt).
func (fx *codexFixture) drawThrough(t *testing.T, req imagegen.Request) (imagegen.Receipt, error) {
	t.Helper()
	svc := imagegen.NewService(imagegen.Options{
		Providers: []imagegen.Provider{fx.provider},
		Journal:   fx.journal,
		Sleep:     func(ctx context.Context, d time.Duration) error { return nil },
	})
	return svc.Draw(context.Background(), req)
}

// emit phát một dòng JSONL qua OnLine như Runner thật.
func emit(spec tools.Spec, stream tools.Stream, v any) {
	if spec.OnLine == nil {
		return
	}
	spec.OnLine(jsonLine(v), stream)
}

// imageSavedEvent: event hoàn tất ảnh với saved_path trong home của invocation
// — exec giả phải tự tạo file đó trong CODEX_HOME.
func execSavedImage(t *testing.T) func(call runnerCall) (tools.Result, error) {
	return func(call runnerCall) (tools.Result, error) {
		home := call.Spec.EnvExact["CODEX_HOME"]
		if home == "" {
			t.Error("exec thiếu CODEX_HOME")
			return tools.Result{Code: 1}, nil
		}
		dir := filepath.Join(home, "generated_images", "sess-1")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return tools.Result{}, err
		}
		png := filepath.Join(dir, "call_1.png")
		if err := os.WriteFile(png, testPNG, 0o600); err != nil {
			return tools.Result{}, err
		}
		emit(call.Spec, tools.StreamStdout, map[string]any{"type": "thread.started", "thread_id": "sess-1"})
		emit(call.Spec, tools.StreamStdout, map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"type": "image_generation", "id": "call_1", "status": "completed",
				"saved_path": png,
			},
		})
		return tools.Result{Code: 0}, nil
	}
}

// Đường vẽ đầy đủ: probe biên → exec JSONL → capture + receipt + biên nhận ok.
func TestCodexDrawEndToEnd(t *testing.T) {
	fx := newCodexFixture(t, execSavedImage(t))
	out := filepath.Join(t.TempDir(), "img.png")
	receipt, err := fx.drawThrough(t, imagegen.Request{
		Key: "c1", Prompt: "a lighthouse", Output: out, Width: 1280, Height: 720,
	})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if receipt.SessionID != "sess-1" || receipt.CallID != "call_1" || receipt.Model != imagegen.CodexImageModel {
		t.Fatalf("receipt = %+v", receipt)
	}
	data, _ := os.ReadFile(out)
	if string(data) != string(testPNG) {
		t.Fatal("output khác PNG đã lưu")
	}
	// Sidecar receipt giống bản cũ.
	if _, err := os.Stat(out + ".receipt.json"); err != nil {
		t.Fatalf("thiếu receipt.json: %v", err)
	}
	// Probe biên chạy trước exec; argv exec có --json + --ephemeral + prompt.
	var sandbox, execs int
	var execArgs []string
	for _, c := range fx.runner.calls {
		if len(c.Args) > 0 && c.Args[0] == "sandbox" {
			sandbox++
		} else if len(c.Args) > 0 && c.Args[0] == "exec" {
			execs++
			execArgs = c.Args
		}
	}
	if sandbox < 3 || execs != 1 {
		t.Fatalf("sandbox=%d exec=%d", sandbox, execs)
	}
	joined := strings.Join(execArgs, " ")
	for _, want := range []string{"exec", "--json", "--ephemeral", "--ignore-user-config", "--image"} {
		if want == "--image" {
			continue // không reference thì không --image
		}
		if !strings.Contains(joined, want) {
			t.Fatalf("argv thiếu %s: %s", want, joined)
		}
	}
	if execArgs[len(execArgs)-1] != "a lighthouse" {
		t.Fatalf("prompt cuối argv: %s", execArgs[len(execArgs)-1])
	}
	// Biên nhận: 1 attempt ok, có scratch + receipt.
	rows := fx.journal.attempts("c1")
	if len(rows) != 1 || rows[0].State != imagegen.ReceiptOK || rows[0].Receipt == nil {
		t.Fatalf("rows = %+v", rows)
	}
}

// Chưa sẵn sàng (isolation_unverified): exec không bao giờ được gọi.
func TestCodexNotReadyBlocksDraw(t *testing.T) {
	fx := newCodexFixture(t, execSavedImage(t))
	st := readyStatus(fx.exe)
	st.Isolation = &tools.CodexIsolation{State: tools.IsolationUnverified}
	fx.provider = imagegen.NewCodexProvider(imagegen.CodexDeps{
		Runner:     fx.runner,
		Auth:       fakeTokens,
		StatusFunc: func(ctx context.Context) (tools.CodexStatus, error) { return st, nil },
	})
	_, err := fx.drawThrough(t, imagegen.Request{
		Key: "c2", Prompt: "x", Output: filepath.Join(t.TempDir(), "o.png"),
	})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeToolMissing {
		t.Fatalf("mong tool_missing, được %v", err)
	}
	if fx.runner.execN != 0 {
		t.Fatalf("exec bị gọi %d lần", fx.runner.execN)
	}
}

// Tool ngoài allowlist trong stream → isolation_unverified, không ảnh.
func TestCodexForbiddenToolAborts(t *testing.T) {
	fx := newCodexFixture(t, func(call runnerCall) (tools.Result, error) {
		emit(call.Spec, tools.StreamStdout, map[string]any{"type": "thread.started", "thread_id": "sess-2"})
		emit(call.Spec, tools.StreamStdout, map[string]any{
			"type": "item.started", "item": map[string]any{"type": "command_execution"},
		})
		return tools.Result{Code: 0}, nil
	})
	_, err := fx.drawThrough(t, imagegen.Request{
		Key: "c3", Prompt: "x", Output: filepath.Join(t.TempDir(), "o.png"),
	})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeToolMissing || !strings.Contains(de.Message, "isolation_unverified") {
		t.Fatalf("mong isolation_unverified, được %v", err)
	}
	rows := fx.journal.attempts("c3")
	// Model đã chạy khi tool ngoài allowlist xuất hiện — không chứng minh được
	// là chưa tốn phí, nên attempt giữ charged (thận trọng về chi phí).
	if len(rows) != 1 || rows[0].State != imagegen.ReceiptFailed || !rows[0].Charged {
		t.Fatalf("attempt ngoài allowlist phải giữ charged: %+v", rows)
	}
}

// Quota event mang resets_at của provider → lỗi quota kèm retryAt provider.
func TestCodexQuotaEventMapsRetryAt(t *testing.T) {
	fx := newCodexFixture(t, func(call runnerCall) (tools.Result, error) {
		emit(call.Spec, tools.StreamStdout, map[string]any{"type": "thread.started", "thread_id": "sess-3"})
		emit(call.Spec, tools.StreamStdout, map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"type": "image_generation",
				"failure": map[string]any{
					"type":      "usage_limit_exceeded",
					"resets_at": 2000000000,
				},
			},
		})
		return tools.Result{Code: 0}, nil
	})
	// Một service dùng chung cho cả hai lần vẽ để quota-skip in-process được
	// kiểm đúng (bản cũ: provider hết quota bị bỏ qua tới mốc reset).
	svc := imagegen.NewService(imagegen.Options{
		Providers: []imagegen.Provider{fx.provider},
		Journal:   fx.journal,
		Sleep:     func(ctx context.Context, d time.Duration) error { return nil },
	})
	_, err := svc.Draw(context.Background(), imagegen.Request{
		Key: "c4", Prompt: "x", Output: filepath.Join(t.TempDir(), "o.png"),
	})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeQuota {
		t.Fatalf("mong quota, được %v", err)
	}
	if de.RetryAt == nil || !de.RetryAt.Equal(time.UnixMilli(2000000000000)) || de.RetryAtSource != "provider" {
		t.Fatalf("retryAt = %+v", de.RetryAt)
	}
	// Attempt quota không đếm phí.
	rows := fx.journal.attempts("c4")
	if len(rows) != 1 || rows[0].Charged {
		t.Fatalf("attempt quota không được đếm phí: %+v", rows)
	}
	// Lần gọi tiếp bung quota ngay — exec không chạy thêm.
	_, err = svc.Draw(context.Background(), imagegen.Request{
		Key: "c4b", Prompt: "x", Output: filepath.Join(t.TempDir(), "o2.png"),
	})
	var de2 *imagegen.DrawError
	if !errors.As(err, &de2) || de2.Code != imagegen.ErrCodeQuota {
		t.Fatalf("draw thứ hai mong quota, được %v", err)
	}
	if fx.runner.execN != 1 {
		t.Fatalf("exec bị gọi %d lần — quota-skip phải chặn lần hai", fx.runner.execN)
	}
}

// Huỷ giữa chừng không được trả ảnh: exec trả lỗi cancel → lỗi timeout-class
// của bản cũ (CancelledError → classify "timeout" → internal_error).
func TestCodexCancelledRun(t *testing.T) {
	fx := newCodexFixture(t, func(call runnerCall) (tools.Result, error) {
		emit(call.Spec, tools.StreamStdout, map[string]any{"type": "thread.started", "thread_id": "sess-4"})
		return tools.Result{}, tools.ErrCancelled
	})
	_, err := fx.drawThrough(t, imagegen.Request{
		Key: "c5", Prompt: "x", Output: filepath.Join(t.TempDir(), "o.png"),
	})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeInternal {
		t.Fatalf("mong internal_error (timeout), được %v", err)
	}
}

// Output base64 trong event được giải mã + ghi thẳng (không qua file home).
func TestCodexBase64Output(t *testing.T) {
	fx := newCodexFixture(t, func(call runnerCall) (tools.Result, error) {
		emit(call.Spec, tools.StreamStdout, map[string]any{"type": "thread.started", "thread_id": "sess-5"})
		emit(call.Spec, tools.StreamStdout, map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"type": "image_generation_call", "id": "call_b64", "status": "completed",
				"result": base64.StdEncoding.EncodeToString(testPNG),
			},
		})
		return tools.Result{Code: 0}, nil
	})
	out := filepath.Join(t.TempDir(), "o.png")
	receipt, err := fx.drawThrough(t, imagegen.Request{Key: "c6", Prompt: "x", Output: out})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if receipt.CallID != "call_b64" {
		t.Fatalf("receipt = %+v", receipt)
	}
	data, _ := os.ReadFile(out)
	if string(data) != string(testPNG) {
		t.Fatal("output khác PNG")
	}
}

// ---- phục hồi theo scratch (reconcileImageInvocation của bản cũ) ----

// deadPID trả pid vừa chết (spawn true rồi reap).
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("không chạy được /usr/bin/true: %v", err)
	}
	pid := cmd.Process.Pid
	for range 100 {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("pid không chết")
	return 0
}

// ownedInvocation dựng scratch + work dir hợp lệ của một invocation đã chết:
// ownership.json đúng owner, home con chứa generated_images/sess/…png.
func ownedInvocation(t *testing.T, target string, pid int) (scratch json.RawMessage, work, home string) {
	t.Helper()
	work, err := os.MkdirTemp("", "ytf-image-call-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })
	if err := os.Chmod(work, 0o700); err != nil {
		t.Fatal(err)
	}
	owner := "dead-owner-1"
	if err := os.WriteFile(filepath.Join(work, "ownership.json"), []byte(`{"owner":"`+owner+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	homes := filepath.Join(work, "homes")
	home = filepath.Join(homes, "call-1")
	dir := filepath.Join(home, "generated_images", "sess-dead")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "call_dead.png"), testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	sc := map[string]any{
		"owner": owner, "pid": pid, "work": work, "home": home,
		"target": target, "codexVersion": "0.162.1", "sessionId": "sess-dead",
	}
	scratch, err = json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	return scratch, work, home
}

// Phiên chết + output còn trong home → cứu ảnh, attempt 'ok'.
func TestReconcileAdoptsOwnedSessionImage(t *testing.T) {
	fx := newCodexFixture(t, nil)
	dir := t.TempDir()
	out := filepath.Join(dir, "recovered.png")
	scratch, _, _ := ownedInvocation(t, out, deadPID(t))
	key := "dead-1"
	seedRunning(t, fx.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: out,
	})
	if err := fx.journal.UpdateScratch(context.Background(), key, 1, scratch); err != nil {
		t.Fatal(err)
	}
	r, err := fx.drawThrough(t, imagegen.Request{Key: key, Prompt: "x", Output: out})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if r.CallID != "call_dead" || r.Attribution != "owned-session-file" {
		t.Fatalf("receipt = %+v", r)
	}
	data, _ := os.ReadFile(out)
	if string(data) != string(testPNG) {
		t.Fatal("output khác PNG đã lưu của phiên")
	}
	if fx.runner.execN != 0 {
		t.Fatal("exec bị gọi trong lúc phục hồi")
	}
	rows := fx.journal.attempts(key)
	if rows[0].State != imagegen.ReceiptOK {
		t.Fatalf("state = %s", rows[0].State)
	}
}

// Ảnh đích tồn tại nhưng khác byte với bản phiên → stale, không ghi đè.
func TestReconcileMismatchedExistingOutputIsStale(t *testing.T) {
	fx := newCodexFixture(t, nil)
	dir := t.TempDir()
	out := filepath.Join(dir, "clash.png")
	scratch, _, _ := ownedInvocation(t, out, deadPID(t))
	// PNG khác byte (fixture khác bit cuối cùng bằng cách lật một byte trong
	// vùng dữ liệu giữa, vẫn là PNG hợp lệ? Không — cứ ghi nội dung khác;
	// điểm cần là sha khác).
	other := append([]byte{}, testPNG...)
	if err := os.WriteFile(out, other, 0o600); err != nil {
		t.Fatal(err)
	}
	// Làm target khác thật: đổi byte đầu của chunk dữ liệu phụ → PNG vẫn qua
	// được validate? Không cần: sha khác là đủ, file có thể bất kỳ.
	if err := os.WriteFile(out, append(other, 0x00), 0o600); err != nil {
		t.Fatal(err)
	}
	key := "dead-2"
	seedRunning(t, fx.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: out,
	})
	if err := fx.journal.UpdateScratch(context.Background(), key, 1, scratch); err != nil {
		t.Fatal(err)
	}
	_, err := fx.drawThrough(t, imagegen.Request{Key: key, Prompt: "x", Output: out})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeStale {
		t.Fatalf("mong stale, được %v", err)
	}
	data, _ := os.ReadFile(out)
	if len(data) != len(other)+1 {
		t.Fatal("output bị ghi đè")
	}
}

// Phiên còn sống (pid của chính test) → conflict, không kết luận.
func TestReconcileLiveInvocationConflicts(t *testing.T) {
	fx := newCodexFixture(t, nil)
	dir := t.TempDir()
	out := filepath.Join(dir, "live.png")
	scratch, _, _ := ownedInvocation(t, out, os.Getpid())
	key := "dead-3"
	seedRunning(t, fx.journal, imagegen.JournalEntry{
		Key: key, Provider: "codex", Model: imagegen.CodexImageModel, Output: out,
	})
	if err := fx.journal.UpdateScratch(context.Background(), key, 1, scratch); err != nil {
		t.Fatal(err)
	}
	_, err := fx.drawThrough(t, imagegen.Request{Key: key, Prompt: "x", Output: out})
	var de *imagegen.DrawError
	if !errors.As(err, &de) || de.Code != imagegen.ErrCodeConflict {
		t.Fatalf("mong conflict, được %v", err)
	}
	if state := fx.journal.attempts(key)[0].State; state != imagegen.ReceiptRunning {
		t.Fatalf("attempt sống phải giữ running, được %s", state)
	}
}
