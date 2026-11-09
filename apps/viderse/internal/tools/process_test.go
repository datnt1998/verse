// Test chuyển từ src/server/tools/tool-process.test.ts của yt-factory.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

const testShell = "/bin/sh"

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test cần POSIX")
	}
}

func TestRunProcessCapturesStdoutAndCode(t *testing.T) {
	r := &ExecRunner{}
	res, err := r.Run(context.Background(), testShell, []string{"-c", "echo hello"}, Spec{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 0 {
		t.Fatalf("code = %d", res.Code)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

func TestRunProcessRejectsNonZeroWithStderrTail(t *testing.T) {
	r := &ExecRunner{}
	_, err := r.Run(context.Background(), testShell, []string{"-c", "echo boom >&2; exit 3"}, Spec{Timeout: 10 * time.Second})
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) {
		t.Fatalf("err = %v, muốn ProcessFailedError", err)
	}
	if pfe.Code != 3 {
		t.Fatalf("code = %d", pfe.Code)
	}
	if !strings.Contains(pfe.Stderr, "boom") {
		t.Fatalf("stderr = %q", pfe.Stderr)
	}
}

func TestRunProcessOkCodesAcceptsNonZero(t *testing.T) {
	r := &ExecRunner{}
	res, err := r.Run(context.Background(), testShell, []string{"-c", "exit 7"}, Spec{
		Timeout: 10 * time.Second,
		OKCodes: []int{0, 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 7 {
		t.Fatalf("code = %d", res.Code)
	}
}

func TestRunProcessAbortKillsWholeProcessGroup(t *testing.T) {
	skipIfWindows(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "child-wrote")
	ctx, cancel := context.WithCancel(context.Background())
	r := &ExecRunner{}
	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, testShell, []string{"-c", fmt.Sprintf("sleep 30; touch %s", marker)}, Spec{})
		done <- err
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, toolchain.ErrCancelled) {
		t.Fatalf("err = %v, muốn toolchain.ErrCancelled", err)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("con cháu trong nhóm còn sống sau huỷ")
	}
}

func TestRunProcessTimeoutKillsSleepingChild(t *testing.T) {
	r := &ExecRunner{}
	_, err := r.Run(context.Background(), testShell, []string{"-c", "sleep 30"}, Spec{Timeout: 200 * time.Millisecond})
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err = %v, muốn ProcessFailedError timeout", err)
	}
}

func TestBoundedLogKeepsTailUnderLimit(t *testing.T) {
	// ~1.2 MB đầu ra, không nhồi vào argv (vượt giới hạn exec).
	script := "head -c 600000 /dev/zero | tr '\\0' x; head -c 600000 /dev/zero | tr '\\0' y"
	r := &ExecRunner{}
	res, err := r.Run(context.Background(), testShell, []string{"-c", script}, Spec{
		Timeout:  30 * time.Second,
		LogLimit: 64 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stdout) > 64*1024+16 {
		t.Fatalf("stdout giữ %d byte, vượt trần", len(res.Stdout))
	}
}

func TestOnLineReceivesProgressLines(t *testing.T) {
	r := &ExecRunner{}
	var lines []string
	_, err := r.Run(context.Background(), testShell,
		[]string{"-c", "printf 'progress:1\\nprogress:2\\n'; printf 'progress:3\\ndone'"},
		Spec{OnLine: func(line string, stream Stream) {
			if stream == StreamStdout {
				lines = append(lines, line)
			}
		}})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "|")
	if got != "progress:1|progress:2|progress:3|done" {
		t.Fatalf("lines = %q", got)
	}
}

func TestOnLineSplitsCarriageReturnProgress(t *testing.T) {
	r := &ExecRunner{}
	var lines []string
	_, err := r.Run(context.Background(), testShell,
		[]string{"-c", "printf 'a\\rb\\r\\nc\\nd'"},
		Spec{OnLine: func(line string, stream Stream) {
			lines = append(lines, line)
		}})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "|")
	if got != "a|b|c|d" {
		t.Fatalf("lines = %q", got)
	}
}

func TestFindOnPathResolvesRealBinaries(t *testing.T) {
	name := "sh"
	if runtime.GOOS == "windows" {
		name = "cmd.exe"
	}
	found := FindOnPath(name, os.Getenv("PATH"))
	if found != "" && !toolchain.FileExists(found) {
		t.Fatalf("FindOnPath trả %q không tồn tại", found)
	}
	if FindOnPath("viderse-definitely-not-a-binary-xyz", os.Getenv("PATH")) != "" {
		t.Fatal("binary không tồn tại bị resolve")
	}
}

func TestFreeDiskBytesPositiveForExistingDir(t *testing.T) {
	if free := toolchain.FreeDiskBytes(t.TempDir()); free <= 0 {
		t.Fatalf("FreeDiskBytes = %d", free)
	}
}

func TestFileVerificationCacheSharesAndReusesPassingCheck(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tool")
	if err := os.WriteFile(file, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := NewFileVerificationCache[string](func(v string) bool { return v == "ok" }, 0)
	runs := 0
	verify := func(ctx context.Context) (string, error) {
		runs++
		return "ok", nil
	}
	ctx := context.Background()

	type pair struct {
		v   string
		err error
	}
	ch := make(chan pair, 2)
	go func() { v, err := cache.Check(file, verify, ctx); ch <- pair{v, err} }()
	go func() { v, err := cache.Check(file, verify, ctx); ch <- pair{v, err} }()
	for range 2 {
		p := <-ch
		if p.err != nil || p.v != "ok" {
			t.Fatalf("kết quả = %+v", p)
		}
	}
	if runs != 1 {
		t.Fatalf("caller đồng thời không chia một lần kiểm: runs = %d", runs)
	}
	if _, err := cache.Check(file, verify, ctx); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("kết quả đạt không được dùng lại cho tệp y nguyên: runs = %d", runs)
	}

	if err := os.WriteFile(file, []byte("v2 replaced binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Check(file, verify, ctx); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Fatalf("thay tệp không kiểm lại: runs = %d", runs)
	}
}

func TestFileVerificationCacheKeepsFailedCheckOnlyForTTL(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tool")
	if err := os.WriteFile(file, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.UnixMilli(1000)
	cache := NewFileVerificationCache[string](func(v string) bool { return v == "ok" }, 10*time.Second)
	cache.now = func() time.Time { return now }
	ctx := context.Background()
	outcomes := []string{"broken", "ok"}
	probe := func(ctx context.Context) (string, error) {
		v := outcomes[0]
		outcomes = outcomes[1:]
		return v, nil
	}
	if v, _ := cache.Check(file, probe, ctx); v != "broken" {
		t.Fatalf("v = %q", v)
	}
	if v, _ := cache.Check(file, probe, ctx); v != "broken" {
		t.Fatalf("tool không khoẻ bị probe lại mỗi lần gọi: v = %q", v)
	}
	now = now.Add(10001 * time.Millisecond)
	if v, _ := cache.Check(file, probe, ctx); v != "ok" {
		t.Fatalf("sau TTL không probe lại: v = %q", v)
	}
	if v, _ := cache.Check(file, probe, ctx); v != "ok" {
		t.Fatalf("kết quả đạt không giữ cache: v = %q", v)
	}

	if err := os.WriteFile(file, []byte("v2 longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterFix := NewFileVerificationCache[string](func(v string) bool { return v == "ok" }, 10*time.Second)
	afterFix.now = func() time.Time { return now }
	if v, _ := afterFix.Check(file, func(ctx context.Context) (string, error) { return "broken", nil }, ctx); v != "broken" {
		t.Fatalf("v = %q", v)
	}
	if err := os.WriteFile(file, []byte("v3 even longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, _ := afterFix.Check(file, func(ctx context.Context) (string, error) { return "ok", nil }, ctx); v != "ok" {
		t.Fatalf("thay tệp không kiểm lại ngay: v = %q", v)
	}

	failing := NewFileVerificationCache[string](func(string) bool { return true }, 0)
	if _, err := failing.Check(file, func(ctx context.Context) (string, error) {
		return "", errors.New("probe crashed")
	}, ctx); err == nil || !strings.Contains(err.Error(), "probe crashed") {
		t.Fatalf("err = %v", err)
	}
	if v, err := failing.Check(file, func(ctx context.Context) (string, error) { return "recovered", nil }, ctx); err != nil || v != "recovered" {
		t.Fatalf("lỗi ném ra bị giữ cache: v = %q err = %v", v, err)
	}
}

func TestTailBufferKeepsExactlyLastNBytes(t *testing.T) {
	limit := 10
	chunkSets := [][]string{
		{"abcdefghijklmnopqrstuvwxyz"},
		{"abc", "defg", "hijklmnop"},
		{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"},
		{"12345", "67890", "ABCDE"},
		{"short"},
	}
	for _, chunks := range chunkSets {
		buf := &tailBuffer{limit: limit}
		for _, c := range chunks {
			buf.push([]byte(c))
		}
		joined := strings.Join(chunks, "")
		want := joined
		if len(joined) > limit {
			want = joined[len(joined)-limit:]
		}
		if buf.text() != want {
			t.Fatalf("%v → %q, muốn %q", chunks, buf.text(), want)
		}
	}
	many := &tailBuffer{limit: limit}
	var all strings.Builder
	for i := range 500 {
		piece := fmt.Sprintf("%d", i%10)
		all.WriteString(piece)
		many.push([]byte(piece))
	}
	s := all.String()
	if many.text() != s[len(s)-limit:] {
		t.Fatalf("luồng chunk nhỏ không giới hạn đúng: %q", many.text())
	}
}

func TestRunProcessStopLetsChildFinalizeBeforeKill(t *testing.T) {
	skipIfWindows(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "finalized")
	ctx, cancel := context.WithCancel(context.Background())
	r := &ExecRunner{}
	// Con có handler SIGTERM ghi marker rồi thoát: phải chạy được trước khi
	// SIGKILL tới.
	script := fmt.Sprintf(`trap 'touch %s; exit 0' TERM; echo ready; while :; do sleep 1; done`, marker)
	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, testShell, []string{"-c", script}, Spec{
			OnLine: func(line string, stream Stream) {
				if strings.Contains(line, "ready") {
					cancel()
				}
			},
		})
		done <- err
	}()
	if err := <-done; !errors.Is(err, toolchain.ErrCancelled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("SIGTERM không tới con hoặc handler không chạy")
	}
}

// TestCancelLeavesNoSurvivingChild là ca bắt buộc: script testdata in tiến độ
// rồi treo, con cháu bỏ qua SIGTERM; huỷ phải dọn sạch cả nhóm.
func TestCancelLeavesNoSurvivingChild(t *testing.T) {
	skipIfWindows(t)
	script, err := filepath.Abs("testdata/hang.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &ExecRunner{Grace: 300 * time.Millisecond}
	var grandchildPID int
	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, testShell, []string{script}, Spec{
			OnLine: func(line string, stream Stream) {
				var pid int
				if _, err := fmt.Sscanf(line, "grandchild:%d", &pid); err == nil && pid > 0 {
					grandchildPID = pid
				}
				if strings.Contains(line, "progress:20%") {
					cancel()
				}
			},
		})
		done <- err
	}()
	if err := <-done; !errors.Is(err, toolchain.ErrCancelled) {
		t.Fatalf("err = %v, muốn toolchain.ErrCancelled", err)
	}
	if grandchildPID == 0 {
		t.Fatal("không đọc được pid con cháu")
	}
	// Run đã chờ nhóm mất hẳn (SIGKILL + reap) trước khi trả, nên kill -0 phải
	// thấy cả leader lẫn con cháu đều chết — zombie cũng tính là còn.
	if pidAlive(grandchildPID) {
		t.Fatalf("con cháu %d còn sống sau huỷ", grandchildPID)
	}
	out, err := exec.Command("ps", "-A", "-o", "pid=,pgid=,command=").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "hang.sh") && !strings.Contains(line, "grep") {
			t.Fatalf("process hang.sh còn sót: %s", line)
		}
	}
}

func TestBackgroundChildrenRunBelowPriority(t *testing.T) {
	skipIfWindows(t)
	r := &ExecRunner{}
	// Con báo nice của chính nó qua ps sau khi stdin đóng (sau khi spawn trả về).
	report := "ps -o nice= -p $$"
	bg, err := r.Run(context.Background(), testShell, []string{"-c", report}, Spec{Background: true, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var nice int
	if _, err := fmt.Sscanf(strings.TrimSpace(bg.Stdout), "%d", &nice); err != nil {
		t.Fatalf("nice output = %q", bg.Stdout)
	}
	own := currentNice(t)
	if nice < own || nice < BackgroundNice {
		t.Fatalf("con nền chạy nice %d, app %d", nice, own)
	}
	fg, err := r.Run(context.Background(), testShell, []string{"-c", report}, Spec{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var fgNice int
	if _, err := fmt.Sscanf(strings.TrimSpace(fg.Stdout), "%d", &fgNice); err != nil {
		t.Fatal(err)
	}
	if fgNice != own {
		t.Fatalf("con thường chạy nice %d, app %d", fgNice, own)
	}
}

func currentNice(t *testing.T) int {
	t.Helper()
	out, err := exec.Command("ps", "-o", "nice=", "-p", fmt.Sprintf("%d", os.Getpid())).Output()
	if err != nil {
		t.Skipf("không đọc được nice: %v", err)
	}
	var n int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// OnLine của hai luồng không bao giờ chạy đồng thời: callback chạy dưới cùng
// một khoá nên vi phạm tuần tự lập tức lộ diện (race detector cũng bắt được).
func TestOnLineNeverRunsConcurrentlyAcrossStreams(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{}
	var mu sync.Mutex
	inFlight := 0
	violations := 0
	var lines []string
	_, err := r.Run(context.Background(), testShell, []string{"-c",
		`for i in $(seq 1 50); do echo "o$i"; echo "e$i" >&2; done`}, Spec{
		OnLine: func(line string, stream Stream) {
			mu.Lock()
			inFlight++
			if inFlight != 1 {
				violations++
			}
			mu.Unlock()
			time.Sleep(time.Microsecond * 100)
			mu.Lock()
			inFlight--
			lines = append(lines, string(stream)+":"+line)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if violations > 0 {
		t.Fatalf("OnLine chồng nhau %d lần", violations)
	}
	if len(lines) != 100 {
		t.Fatalf("lines = %d, muốn 100", len(lines))
	}
}

// Dòng vượt MaxLineBytes làm run thất bại rõ ràng thay vì cắt lặng hay giữ
// bộ đệm vô hạn — JSONL của hạ tầng phía trên không được âm thầm hỏng.
func TestOnLineOversizeLineFailsRun(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{}
	_, err := r.Run(context.Background(), testShell, []string{"-c",
		`head -c 4194305 /dev/zero | tr '\0' 'x'`}, Spec{
		OnLine: func(line string, stream Stream) {},
	})
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) {
		t.Fatalf("err = %v, muốn ProcessFailedError", err)
	}
}

func TestRunProcessPreservesLargeSingleLineJSONWithoutLineCallback(t *testing.T) {
	skipIfWindows(t)
	script := fmt.Sprintf(`printf '{"padding":"'; printf '%%%ds' ''; printf '","id":"source-video","duration":1800}'`, MaxLineBytes+1)
	res, err := (&ExecRunner{}).Run(context.Background(), testShell, []string{"-c", script}, Spec{
		Timeout: 10 * time.Second, LogLimit: 2 * MaxLineBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		ID       string `json:"id"`
		Duration int    `json:"duration"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &metadata); err != nil {
		t.Fatalf("metadata JSON was cut short: %v", err)
	}
	if metadata.ID != "source-video" || metadata.Duration != 1800 {
		t.Fatalf("metadata after the large field was lost: %+v", metadata)
	}
}

// Dòng không có \n kết vẫn được phát khi luồng đóng (partial line cuối).
func TestOnLineFlushesTrailingPartialLine(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{}
	var lines []string
	_, err := r.Run(context.Background(), testShell, []string{"-c", `printf "done"`}, Spec{
		OnLine: func(line string, stream Stream) { lines = append(lines, line) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "done" {
		t.Fatalf("lines = %v", lines)
	}
}

// Đọc hết đầu ra của con trước khi Run trả: child đóng stdout sớm nhưng còn
// in sau khi hết lệnh — đuôi không bị cắt.
func TestRunCollectsFullOutputBeforeReturning(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{}
	var got []string
	res, err := r.Run(context.Background(), testShell, []string{"-c",
		`echo first; echo second; echo third`}, Spec{
		OnLine: func(line string, stream Stream) { got = append(got, line) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "first" || got[2] != "third" {
		t.Fatalf("lines = %v", got)
	}
	if !strings.Contains(res.Stdout, "third") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

// Timeout trong lúc ân hạn (con bỏ qua SIGTERM): nhóm vẫn bị SIGKILL và lỗi
// trả về là ProcessFailedError TimedOut, không còn process sót.
func TestRunProcessCancelDuringGraceStillKillsGroup(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{Grace: 100 * time.Millisecond}
	// Con trực tiếp bỏ qua TERM và spawn cháu; cả nhóm phải chết sau ân hạn.
	script := `trap '' TERM; sh -c 'trap "" TERM; while :; do sleep 1; done' & while :; do sleep 1; done`
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := r.Run(ctx, testShell, []string{"-c", script}, Spec{})
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) {
		t.Fatalf("err = %v, muốn ProcessFailedError (TimedOut)", err)
	}
	if !pfe.TimedOut {
		t.Fatalf("TimedOut = false, err = %v", err)
	}
	// Không còn process nào của script sót.
	time.Sleep(200 * time.Millisecond)
	out, _ := exec.Command("sh", "-c", `ps -eo command | grep -E 'while :; do sleep 1; done' | grep -v grep`).CombinedOutput()
	if strings.Contains(string(out), "sleep 1") {
		t.Fatalf("process con sót: %s", out)
	}
}

// LineLimit nâng trần dòng: event JSONL lớn hợp lệ (ảnh base64) đi qua, còn
// mặc định vẫn từ chối đúng ngưỡng cũ.
func TestOnLineLineLimitRaisesRecordCap(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	r := &ExecRunner{}
	// Một dòng 5 MiB > MaxLineBytes mặc định nhưng dưới LineLimit 8 MiB.
	script := `head -c 5242880 /dev/zero | tr '\0' 'x'; echo`
	var got int
	_, err := r.Run(context.Background(), testShell, []string{"-c", script}, Spec{
		LineLimit: 8 * 1024 * 1024,
		OnLine:    func(line string, stream Stream) { got = len(line) },
	})
	if err != nil {
		t.Fatalf("LineLimit nâng không áp: %v", err)
	}
	if got != 5242880 {
		t.Fatalf("line = %d, muốn 5242880", got)
	}
	// Cùng đầu ra với trần mặc định vẫn lỗi.
	_, err = r.Run(context.Background(), testShell, []string{"-c", script}, Spec{
		OnLine: func(line string, stream Stream) {},
	})
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) {
		t.Fatalf("err = %v, muốn ProcessFailedError", err)
	}
}
