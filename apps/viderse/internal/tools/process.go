// Bộ chạy process con chung cho các công cụ: spawn theo argv (không qua
// shell), timeout theo lần gọi, huỷ bằng cách giết cả nhóm process (SIGTERM
// trước để ffmpeg/yt-dlp kịp kết thúc, SIGKILL sau ân hạn), giữ đuôi
// stdout/stderr có trần, callback theo dòng và writer log tuỳ chọn.
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

// Sentinel huỷ (ErrCancelled) và lỗi huỷ dùng chung nằm ở platform/toolchain:
// bộ cài dùng chung và bộ chạy process con phải cùng một sentinel để tầng
// trên (pipeline) nhận ra lượt bị huỷ.

// Stream đánh dấu luồng đầu ra của process con.
type Stream string

const (
	StreamStdout Stream = "stdout"
	StreamStderr Stream = "stderr"
)

// Result là kết quả một lần chạy: mã thoát và đuôi đầu ra có trần.
type Result struct {
	Code     int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// ProcessFailedError là lỗi khi process thoát mã không được chấp nhận, hết
// timeout, hoặc không spawn được. Stdout/Stderr là đuôi có trần.
type ProcessFailedError struct {
	Message  string
	Code     int
	Stdout   string
	Stderr   string
	TimedOut bool
	Err      error // lỗi spawn/wait/copy gốc nếu có
}

func (e *ProcessFailedError) Error() string { return e.Message }
func (e *ProcessFailedError) Unwrap() error { return e.Err }

// Spec mô tả một lần chạy process.
//
// Bảo đảm của runner: OnLine và Log luôn được gọi nối tiếp (tuần tự hoá giữa
// hai luồng) — consumer không cần khoá riêng, và toàn bộ callback xong trước
// khi Run trả. OnLine nhận mỗi dòng ≤ LineLimit (mặc định MaxLineBytes); dòng
// vượt trần làm run thất bại rõ ràng thay vì cắt lặng.
type Spec struct {
	// Dir là thư mục làm việc; rỗng → thư mục hiện tại.
	Dir string
	// Env gộp lên môi trường của app (cùng khoá thì ghi đè).
	Env map[string]string
	// EnvExact thay toàn bộ môi trường (dùng cho các cuộc gọi cô lập).
	EnvExact map[string]string
	// Timeout giết cả nhóm sau khoảng này; 0 → không giới hạn.
	Timeout time.Duration
	// LogLimit là trần byte giữ lại cho đuôi stdout+stderr; 0 → 256 KB.
	LogLimit int
	// OnLine được gọi với mỗi dòng đã giải mã và luồng phát ra; tách cả \r
	// (tiến độ ghi đè) lẫn \n. Không bao giờ gọi đồng thời giữa hai luồng.
	OnLine func(line string, stream Stream)
	// Log nhận byte thô của cả hai luồng (ghi log thư mục dự án, v.v.); gọi
	// nối tiếp dưới cùng khoá với OnLine.
	Log io.Writer
	// LineLimit là trần byte một dòng đưa tới OnLine; 0 → MaxLineBytes. Nâng
	// lên cho đầu ra hợp lệ rất lớn (một event JSONL mang ảnh base64), không
	// bao giờ hạ xuống dưới 1.
	LineLimit int
	// OKCodes là mã thoát được coi là thành công ngoài 0.
	OKCodes []int
	// Stdin là byte ghi vào stdin rồi đóng.
	Stdin []byte
	// Background chạy con ở độ ưu tiên CPU thấp (nice 10) cho việc nặng dài.
	Background bool
	// Match/Label ghi nhận nhóm con vào Registry (nếu Runner có gắn).
	Match string
	Label string
	// OnStart được gọi một lần ngay sau khi spawn, với pid của nhóm process
	// con (leader = pgid). Dùng để ghi nhóm vào nơi khác (vd. process_group
	// của step) trước khi con chạy tiếp.
	OnStart func(pid int)
}

// Runner là điểm gọi duy nhất để chạy một binary; gói khác inject bản giả
// trong test.
type Runner interface {
	Run(ctx context.Context, file string, args []string, spec Spec) (Result, error)
}

// ExecRunner là Runner thật: spawn process con trong nhóm riêng.
type ExecRunner struct {
	// Grace là khoảng chờ giữa SIGTERM và SIGKILL khi dừng; 0 → TerminateGrace.
	Grace time.Duration
	// Registry, nếu khác nil, ghi mọi nhóm con để lần chạy sau dọn nhóm mồ côi.
	Registry *Registry
}

// DefaultLogLimit là trần mặc định giữ lại cho đuôi stdout/stderr.
const DefaultLogLimit = 256 * 1024

// MaxLineBytes là trần một dòng khi OnLine được đặt: đủ cho JSONL/đầu ra
// marker, chặn một luồng không-newline giữ bộ đệm vô hạn.
const MaxLineBytes = 4 * 1024 * 1024

// BackgroundNice là mức nice của con chạy nền: dưới app và Chrome dựng hình,
// trên idle-only.
const BackgroundNice = 10

// delivery tuần tự hoá mọi gọi consumer: một mutex cho cả hai luồng, cập nhật
// tail, log thô và callback dòng dưới cùng khoá.
type delivery struct {
	mu      sync.Mutex
	onLine  func(string, Stream)
	log     io.Writer
	outTail *tailBuffer
	errTail *tailBuffer
	outErr  error // lỗi copy/dòng-oversize của một luồng, nếu có
}

// lineWriter là adapter cmd.Stdout/Stderr: ghi thô vào log+tail mỗi lần
// Write, và (khi có OnLine) tách dòng trên \n/\r với pending có trần
// max. Không có OnLine thì không tích luỹ gì ngoài tail có trần.
type lineWriter struct {
	d       *delivery
	tail    *tailBuffer
	stream  Stream
	max     int
	pending []byte
	skipLF  bool // \r đóng write trước: \n mở write này thuộc cùng \r\n
}

var errLineTooLong = errors.New("dòng đầu ra vượt LineLimit")

func (d *delivery) write(stream Stream, tail *tailBuffer, w *lineWriter, p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.onLine != nil {
		if err := w.feed(p); err != nil {
			if d.outErr == nil {
				d.outErr = err
			}
			return len(p), err
		}
	}
	tail.push(p)
	if d.log != nil {
		if _, err := d.log.Write(p); err != nil && d.outErr == nil {
			d.outErr = err
		}
	}
	return len(p), nil
}

// feed tích luỹ dữ liệu và phát dòng hoàn chỉnh tới OnLine; gọi với mu giữ.
func (w *lineWriter) feed(p []byte) error {
	start := 0
	if w.skipLF && len(p) > 0 {
		if p[0] == '\n' {
			start = 1
		}
		w.skipLF = false
	}
	for i := start; i < len(p); i++ {
		b := p[i]
		if b != '\n' && b != '\r' {
			continue
		}
		w.pending = append(w.pending, p[start:i]...)
		if len(w.pending) > w.max {
			return fmt.Errorf("%w: %d > %d, stream %s", errLineTooLong, len(w.pending), w.max, w.stream)
		}
		if w.d.onLine != nil {
			w.d.onLine(string(w.pending), w.stream)
		}
		w.pending = w.pending[:0]
		start = i + 1
		if b == '\r' {
			if i+1 < len(p) && p[i+1] == '\n' {
				// \r\n cùng write: nuốt cả \n.
				i++
				start = i + 1
			} else if i+1 == len(p) {
				w.skipLF = true
			}
		}
	}
	w.pending = append(w.pending, p[start:]...)
	if len(w.pending) > w.max {
		return fmt.Errorf("%w: %d > %d, stream %s", errLineTooLong, len(w.pending), w.max, w.stream)
	}
	return nil
}

// flush phát dòng dư cuối luồng (không \n đóng); gọi với mu giữ.
func (w *lineWriter) flush() {
	if len(w.pending) > 0 && w.d.onLine != nil {
		w.d.onLine(string(w.pending), w.stream)
	}
	w.pending = nil
}

// Run spawn file với args theo Spec. Con chạy trong nhóm process riêng nên
// huỷ/timeout giết cả cây process chứ không chỉ con trực tiếp. Đầu ra được
// os/exec sao chép hết trước khi Wait trả — run kết thúc khi child đó, không
// còn đuôi mất vì reader chưa kịp đọc.
func (r *ExecRunner) Run(ctx context.Context, file string, args []string, spec Spec) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, toolchain.CancelledErr(ctx)
	}
	limit := spec.LogLimit
	if limit <= 0 {
		limit = DefaultLogLimit
	}

	// Không dùng CommandContext: cơ chế huỷ của nó chỉ giết leader. Tự giết cả
	// nhóm process trong watcher bên dưới.
	cmd := exec.Command(file, args...)
	cmd.Dir = spec.Dir
	cmd.Env = buildEnv(spec)
	makeDetached(cmd)
	if spec.Stdin != nil {
		cmd.Stdin = bytes.NewReader(spec.Stdin)
	}

	var outTail, errTail tailBuffer
	outTail.limit = limit
	errTail.limit = limit
	d := &delivery{onLine: spec.OnLine, log: spec.Log, outTail: &outTail, errTail: &errTail}
	lineLimit := spec.LineLimit
	if lineLimit <= 0 {
		lineLimit = MaxLineBytes
	}
	outW := &lineWriter{d: d, tail: &outTail, stream: StreamStdout, max: lineLimit}
	errW := &lineWriter{d: d, tail: &errTail, stream: StreamStderr, max: lineLimit}
	cmd.Stdout = writerFunc(func(p []byte) (int, error) { return d.write(StreamStdout, &outTail, outW, p) })
	cmd.Stderr = writerFunc(func(p []byte) (int, error) { return d.write(StreamStderr, &errTail, errW, p) })

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{}, &ProcessFailedError{
			Message: fmt.Sprintf("không chạy được %s: %v", filepath.Base(file), err),
			Code:    -1,
			Err:     err,
		}
	}
	pid := cmd.Process.Pid
	if spec.Background {
		setBackgroundPriority(pid, BackgroundNice)
	}
	if spec.OnStart != nil {
		spec.OnStart(pid)
	}
	if r.Registry != nil {
		match := spec.Match
		if match == "" {
			match = file
		}
		label := spec.Label
		if label == "" {
			label = filepath.Base(file)
		}
		r.Registry.Track(pid, match, label)
	}

	// Watcher: ctx huỷ hoặc timeout → giết cả nhóm. Việc giết nhóm chạy trên
	// context nền: một khi quyết định dừng, huỷ của caller không được phép làm
	// đứt quãng chuỗi TERM→ân hạn→KILL.
	grace := r.Grace
	if grace <= 0 {
		grace = TerminateGrace
	}
	var timedOut, stopped atomic.Bool
	stopDone := make(chan struct{})
	stopCh := make(chan struct{}, 1)
	go func() {
		defer close(stopDone)
		var timer <-chan time.Time
		if spec.Timeout > 0 {
			t := time.NewTimer(spec.Timeout)
			defer t.Stop()
			timer = t.C
		}
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
		case <-timer:
			timedOut.Store(true)
		}
		stopped.Store(true)
		_, _ = TerminateGroup(context.Background(), GroupTarget{PGID: pid}, TerminateOptions{Grace: grace})
	}()

	waitErr := cmd.Wait()
	// Phát dòng dư cuối mỗi luồng trước khi trả.
	d.mu.Lock()
	outW.flush()
	errW.flush()
	copyErr := d.outErr
	d.mu.Unlock()
	close(stopCh)
	<-stopDone
	if r.Registry != nil && !groupAlive(pid) {
		r.Registry.UntrackForget(pid)
	}

	res := Result{
		Code:     exitCode(cmd, waitErr),
		Stdout:   outTail.text(),
		Stderr:   errTail.text(),
		Duration: time.Since(start),
	}
	if stopped.Load() && !timedOut.Load() {
		// Deadline của caller cũng là timeout: ProcessFailedError{TimedOut}
		// để consumer phân biệt với huỷ chủ động (context.Canceled).
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return res, &ProcessFailedError{
				Message:  fmt.Sprintf("%s hết %s deadline", filepath.Base(file), res.Duration.Truncate(time.Millisecond)),
				Code:     res.Code,
				Stdout:   res.Stdout,
				Stderr:   res.Stderr,
				TimedOut: true,
				Err:      ctx.Err(),
			}
		}
		return res, toolchain.CancelledErr(ctx)
	}
	if timedOut.Load() {
		return res, &ProcessFailedError{
			Message:  fmt.Sprintf("%s hết %s timeout", filepath.Base(file), spec.Timeout),
			Code:     res.Code,
			Stdout:   res.Stdout,
			Stderr:   res.Stderr,
			TimedOut: true,
			Err:      waitErr,
		}
	}
	if waitErr == nil && copyErr != nil {
		waitErr = copyErr
	}
	if waitErr != nil && !isExitError(waitErr) {
		return res, &ProcessFailedError{
			Message: fmt.Sprintf("%s: %v", filepath.Base(file), waitErr),
			Code:    res.Code,
			Stdout:  res.Stdout,
			Stderr:  res.Stderr,
			Err:     waitErr,
		}
	}
	ok := false
	for _, c := range append([]int{0}, spec.OKCodes...) {
		if res.Code == c {
			ok = true
			break
		}
	}
	if !ok {
		last := ""
		if t := strings.TrimSpace(res.Stderr); t != "" {
			lines := strings.Split(t, "\n")
			last = " " + strings.TrimSpace(lines[len(lines)-1])
		}
		return res, &ProcessFailedError{
			Message: fmt.Sprintf("%s thoát với mã %d:%s", filepath.Base(file), res.Code, last),
			Code:    res.Code,
			Stdout:  res.Stdout,
			Stderr:  res.Stderr,
		}
	}
	return res, nil
}

// writerFunc cho phép một hàm đóng vai io.Writer cho cmd.Stdout/Stderr: Go
// spawn một goroutine copy nội bộ và Wait chờ nó đọc hết.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func isExitError(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee)
}

func exitCode(cmd *exec.Cmd, waitErr error) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	return -1
}

// buildEnv dựng môi trường con: EnvExact thay hẳn; Env gộp và ghi đè lên
// môi trường app (khoá trùng chỉ giữ giá trị mới).
func buildEnv(spec Spec) []string {
	if spec.EnvExact != nil {
		return envMapToList(spec.EnvExact)
	}
	if len(spec.Env) == 0 {
		return nil // kế thừa môi trường hiện tại
	}
	environ := os.Environ()
	out := make([]string, 0, len(environ)+len(spec.Env))
	for _, kv := range environ {
		key := kv[:strings.IndexByte(kv, '=')]
		if _, override := spec.Env[key]; !override {
			out = append(out, kv)
		}
	}
	for k, v := range spec.Env {
		out = append(out, k+"="+v)
	}
	return out
}

func envMapToList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// tailBuffer giữ đúng limit byte cuối của một luồng: push thêm vào và cắt từ
// đầu nên không sao chép lại theo mỗi chunk.
type tailBuffer struct {
	chunks [][]byte
	head   int
	size   int
	limit  int
}

func (t *tailBuffer) push(chunk []byte) {
	// Luôn sao chép: chunk mượn buffer đọc dùng lại mỗi lần Read.
	if len(chunk) >= t.limit {
		t.chunks = t.chunks[:0]
		t.head = 0
		cp := make([]byte, t.limit)
		copy(cp, chunk[len(chunk)-t.limit:])
		t.chunks = append(t.chunks, cp)
		t.size = t.limit
		return
	}
	cp := make([]byte, len(chunk))
	copy(cp, chunk)
	t.chunks = append(t.chunks, cp)
	t.size += len(cp)
	for t.size > t.limit {
		first := t.chunks[t.head]
		excess := t.size - t.limit
		if len(first) <= excess {
			t.head++
			t.size -= len(first)
		} else {
			t.chunks[t.head] = first[excess:]
			t.size -= excess
		}
	}
	if t.head > 32 && t.head*2 > len(t.chunks) {
		t.chunks = append([][]byte(nil), t.chunks[t.head:]...)
		t.head = 0
	}
}

func (t *tailBuffer) text() string {
	var b bytes.Buffer
	for _, c := range t.chunks[t.head:] {
		b.Write(c)
	}
	return strings.ReplaceAll(b.String(), "\r\n", "\n")
}

// FindOnPath tìm tên thực thi trong PATH; trả bản chạy được đầu tiên hoặc "".
func FindOnPath(name, pathEnv string) string {
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if isExecutableFile(candidate) {
			return candidate
		}
	}
	return ""
}

// FirstExecutable trả ứng viên tuyệt đối đầu tiên tồn tại và chạy được.
func FirstExecutable(candidates []string) string {
	for _, c := range candidates {
		if isExecutableFile(c) {
			return c
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// FileVerificationCache là ghi nhớ trong đời process cho một phép kiểm đắt
// của một tệp trên đĩa (probe năng lực binary, hash checkpoint). Khoá là
// device, inode, kích cỡ và mtime của tệp nên thay/sửa tệp sẽ kiểm lại. Kết
// quả IsVerified chấp nhận được giữ đến khi tệp đổi; kết quả không chấp nhận
// chỉ giữ FailureTTL để đọc status không spawn probe mỗi lần gọi; lỗi ném ra
// không bao giờ được giữ. Caller đồng thời cùng một tệp dùng chung một lần
// kiểm đang chạy.
type FileVerificationCache[T any] struct {
	isVerified func(T) bool
	failureTTL time.Duration
	now        func() time.Time
	mu         sync.Mutex
	entries    map[string]*verifyEntry[T]
}

type verifyEntry[T any] struct {
	identity  string
	done      chan struct{}
	value     T
	err       error
	expiresAt time.Time
}

// NewFileVerificationCache dựng cache; IsVerified quyết định kết quả nào được
// giữ lâu dài. failureTTL 0 → 15s.
func NewFileVerificationCache[T any](isVerified func(T) bool, failureTTL time.Duration) *FileVerificationCache[T] {
	if failureTTL <= 0 {
		failureTTL = 15 * time.Second
	}
	return &FileVerificationCache[T]{isVerified: isVerified, failureTTL: failureTTL, now: time.Now, entries: map[string]*verifyEntry[T]{}}
}

// Check trả kết quả kiểm của tệp: dùng lại bản ghi còn hiệu lực, chạy verify
// khi tệp đổi hoặc chưa có. Lỗi stat (tệp biến mất giữa chừng) chạy verify
// trực tiếp.
func (c *FileVerificationCache[T]) Check(file string, verify func(context.Context) (T, error), ctx context.Context) (T, error) {
	identity, ok := fileIdentity(file)
	if !ok {
		return verify(ctx)
	}
	c.mu.Lock()
	if hit, ok := c.entries[file]; ok && hit.identity == identity && hit.expiresAt.After(c.now()) {
		done := hit.done
		c.mu.Unlock()
		select {
		case <-done:
			return hit.value, hit.err
		case <-ctx.Done():
			var zero T
			return zero, toolchain.CancelledErr(ctx)
		}
	}
	entry := &verifyEntry[T]{identity: identity, done: make(chan struct{}), expiresAt: time.Now().Add(100 * 365 * 24 * time.Hour)}
	c.entries[file] = entry
	c.mu.Unlock()

	go func() {
		entry.value, entry.err = verify(ctx)
		c.mu.Lock()
		if entry.err != nil {
			if c.entries[file] == entry {
				delete(c.entries, file)
			}
		} else if !c.isVerified(entry.value) {
			entry.expiresAt = c.now().Add(c.failureTTL)
		}
		c.mu.Unlock()
		close(entry.done)
	}()

	select {
	case <-entry.done:
		return entry.value, entry.err
	case <-ctx.Done():
		var zero T
		return zero, toolchain.CancelledErr(ctx)
	}
}

// HumanBytes định dạng dung lượng cho câu chữ UI.
func HumanBytes(bytes int64) string {
	if bytes < 0 {
		return "không rõ"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(bytes)
	u := 0
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	return strconv.FormatFloat(v, 'f', 0, 64) + " " + units[u]
}

func parentDir(p string) string { return filepath.Dir(p) }

// spawnDetached mở một process nền sống độc lập với caller (login flow, v.v.):
// nhóm riêng qua Setpgid, stdio vào /dev/null, env gộp lên môi trường app.
// Trả pid và hàm wait để caller thu zombie khi con thoát.
func spawnDetached(file string, args []string, env map[string]string) (pid int, wait func() error, err error) {
	cmd := exec.Command(file, args...)
	makeDetached(cmd)
	if len(env) > 0 {
		cmd.Env = buildEnv(Spec{Env: env})
	}
	if err := cmd.Start(); err != nil {
		return 0, nil, err
	}
	return cmd.Process.Pid, cmd.Wait, nil
}
