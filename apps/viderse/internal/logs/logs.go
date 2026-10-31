// Package logs giữ nhật ký của các công cụ máy: mỗi công cụ một tệp JSONL
// dưới <data>/logs/<tool>.log, giới hạn 256 KB (giữ 128 KB cuối khi quá tải —
// cùng cách của appendStepLog trong yt-factory), và gói báo lỗi dạng zip đã
// che bí mật. Không có biến toàn cục; mọi trạng thái nằm trong Store.
package logs

import (
	"context"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Level là mức nghiêm trọng của một dòng nhật ký (alias chuỗi để interface
// phía người dùng nhận string mà không cần import gói này).
type Level = string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// severity cho phép lọc mức tối thiểu: error < warn < info.
func severity(l Level) int {
	switch l {
	case LevelError:
		return 0
	case LevelWarn:
		return 1
	default:
		return 2
	}
}

// Entry là một dòng nhật ký đã che.
type Entry struct {
	TS      time.Time `json:"ts"`
	Tool    string    `json:"tool"`
	Level   Level     `json:"level"`
	Message string    `json:"message"`
}

// logLimitBytes là trần một tệp log; giữ lại logKeepBytes cuối khi chạm —
// cùng 256 KB / một nửa của yt-factory.
const (
	logLimitBytes = 256 * 1024
	logKeepBytes  = logLimitBytes / 2
)

// redactPatterns là các chuỗi bị che trước khi một dòng chạm đĩa — giữ đúng
// bảng của REDACTED_PATTERNS trong yt-factory.
var redactPatterns = []struct {
	re  *regexp.Regexp
	rep string
}{
	{regexp.MustCompile(`(?i)bearer\s+\S+`), "Bearer <redacted>"},
	// Cả dòng Cookie/Set-Cookie (mọi cặp name=value) — không chỉ giá trị đầu.
	{regexp.MustCompile(`(?i)\b(set-cookie|cookie)\s*:\s*[^\n]*`), "$1: <redacted>"},
	// Trường cookie đặt tên trong chuỗi/JSON không có header.
	{regexp.MustCompile(`(?i)\b(SESSDATA|bili_jct|buvid3|DedeUserID\w*|HSID|SSID|APISID|SAPISID|SID|sessionid|__Secure-[A-Za-z0-9_-]+)=([^;\s,"]+)`), "$1=<redacted>"},
	{regexp.MustCompile(`(?i)(cookie|token|secret|password|authorization|api[-_]?key)\s*[=:]\s*"?[^"\s,}]+"?`), "$1=<redacted>"},
	{regexp.MustCompile(`(?i)"[a-z0-9_-]*(?:token|secret|password|cookie|api[-_]?key|authorization)[a-z0-9_-]*"\s*:\s*"[^"]*"`), `"<redacted>":"<redacted>"`},
	// JWT trong URL/body (mã đăng nhập thiết bị, phiên nền tảng).
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}`), "<redacted-jwt>"},
	// Tham số nhạy cảm trong URL (cookie dán, khoá API trên query).
	{regexp.MustCompile(`(?i)((?:api[-_]?key|token|secret|signature|sessionid|sess|auth)[?&=])([^&\s"']+)`), "$1<redacted>"},
}

// Store ghi và đọc nhật ký công cụ trong <dir>/<tool>.log.
type Store struct {
	dir  string
	home string // gốc thư mục nhà để che tên người dùng trong đường dẫn
	tasks interface {
		Start(context.Context, domain.TaskSpec) (string, error)
		Finish(context.Context, string) error
	}
	sequence atomic.Uint64

	mu sync.Mutex // tuần tự hoá append+rotate trên mọi tệp
}

// New dựng store tại dir (tạo lười). home là thư mục nhà để che đường dẫn;
// rỗng → không che đường dẫn.
func New(dir, home string) *Store {
	return &Store{dir: dir, home: home}
}

// SetTasks connects bundle generation to the shared background activity list.
// Call before serving requests.
func (s *Store) SetTasks(tasks interface {
	Start(context.Context, domain.TaskSpec) (string, error)
	Finish(context.Context, string) error
}) { s.tasks = tasks }

// Close là điểm kết thúc của store (mọi tệp đã đóng ngay sau mỗi lần ghi);
// giữ để composition gọi khi tắt và tương lai thêm buffer.
func (s *Store) Close() error { return nil }

// Redact che bí mật trong một dòng chữ và thay tiền tố thư mục nhà bằng "~".
// API dùng chung cho mọi gói ghi log (AIConn, Platforms…): giữ ổn định.
func Redact(line, home string) string {
	s := Store{home: home}
	return s.Redact(line)
}

// Redact che một dòng chữ: bí mật theo bảng mẫu + tên người dùng trong
// đường dẫn nhà (/<home>/… → ~/…).
func (s *Store) Redact(line string) string {
	out := line
	for _, p := range redactPatterns {
		out = p.re.ReplaceAllString(out, p.rep)
	}
	if s.home != "" {
		out = strings.ReplaceAll(out, s.home+string(os.PathSeparator), "~/")
		out = strings.ReplaceAll(out, s.home, "~")
	}
	return out
}

// toolFile ánh id công cụ sang tên tệp an toàn (ký tự lạ → _).
func (s *Store) toolFile(tool string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, tool)
	if name == "" {
		name = "tool"
	}
	return filepath.Join(s.dir, name+".log")
}

// Append ghi một dòng JSONL (đã che) vào log của công cụ; tệp quá trần bị
// cắt còn logKeepBytes cuối trước khi ghi — dòng mới nhất luôn đọc được.
func (s *Store) Append(tool string, level Level, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file := s.toolFile(tool)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return
	}
	entry := fmt.Sprintf(`{"ts":%q,"tool":%q,"level":%q,"message":%q}`+"\n",
		time.Now().UTC().Format(time.RFC3339Nano), tool, string(level), s.Redact(message))
	if st, err := os.Stat(file); err == nil && st.Size()+int64(len(entry)) > logLimitBytes {
		if kept, err := tailBytes(file, logKeepBytes); err == nil {
			if err := writeFileAtomic(file, kept, 0o600); err != nil {
				return
			}
		}
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(entry)
	_ = f.Close()
}

// Writer trả io.Writer cho Spec.Log của tools: mỗi dòng ghi thành entry ở
// level cho trước, đã che. An toàn gọi đồng thời.
func (s *Store) Writer(tool string, level Level) *Writer {
	return &Writer{store: s, tool: tool, level: level}
}

// Writer là io.Writer tách dòng → Store.Append (dòng cuối cụt đuôi cũng ghi).
type Writer struct {
	store *Store
	tool  string
	level Level

	mu  sync.Mutex
	buf []byte
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.store.Append(w.tool, w.level, strings.TrimRight(string(w.buf[:i]), "\r"))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// Flush ghi phần đuôi chưa có newline.
func (w *Writer) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.store.Append(w.tool, w.level, string(w.buf))
		w.buf = nil
	}
}

// Read đọc tối đa limit dòng mới nhất (mới nhất sau cùng), lọc theo tool
// (rỗng = mọi công cụ), mức tối thiểu (info = tất cả), và con trỏ before
// (chỉ dòng ts < before; zero = không giới hạn). nextBefore khác nil khi
// còn trang cũ hơn — nó là ts của dòng cũ nhất trong trang này.
func (s *Store) Read(tool string, minLevel Level, limit int, before time.Time) ([]Entry, *time.Time, error) {
	if limit <= 0 {
		limit = 200
	}
	var names []string
	if tool != "" {
		if fileExists(s.toolFile(tool)) {
			names = []string{s.toolFile(tool)}
		}
	} else {
		entries, err := os.ReadDir(s.dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, nil
			}
			return nil, nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
				names = append(names, filepath.Join(s.dir, e.Name()))
			}
		}
	}
	var out []Entry
	for _, name := range names {
		entries, err := readEntries(name, minLevel, before)
		if err != nil {
			continue
		}
		out = append(out, entries...)
	}
	// Mới nhất sau cùng, cắt lấy limit dòng cuối.
	sort.SliceStable(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	if len(out) <= limit {
		return out, nil, nil
	}
	out = out[len(out)-limit:]
	oldest := out[0].TS
	return out, &oldest, nil
}

func readEntries(file string, minLevel Level, before time.Time) ([]Entry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		var e Entry
		line := sc.Bytes()
		if err := json.Unmarshal(line, &e); err != nil {
			continue // dòng cụt sau lần cắt — bỏ, không gãy đọc
		}
		if e.Tool == "" {
			continue
		}
		if severity(e.Level) > severity(minLevel) {
			continue
		}
		if !before.IsZero() && !e.TS.Before(before) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// tailBytes đọc n byte cuối của tệp; nhỏ hơn n thì đọc hết.
func tailBytes(file string, n int64) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := st.Size() - n
	if start < 0 {
		start = 0
	}
	// Bắt đầu ở đầu dòng đầu tiên còn nguyên sau điểm cắt.
	if start > 0 {
		if _, err := f.Seek(start, 0); err != nil {
			return nil, err
		}
		rest := make([]byte, st.Size()-start)
		if _, err := f.ReadAt(rest, start); err != nil {
			return nil, err
		}
		if i := bytes.IndexByte(rest, '\n'); i >= 0 && i+1 < len(rest) {
			return rest[i+1:], nil
		}
		return rest, nil
	}
	return io.ReadAll(f)
}

// writeFileAtomic ghi nội dung qua tệp tạm + rename (không để lại nửa tệp).
func writeFileAtomic(file string, data []byte, perm os.FileMode) error {
	tmp := fmt.Sprintf("%s.%d.tmp", file, os.Getpid())
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
