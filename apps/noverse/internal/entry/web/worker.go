package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/diag"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	"github.com/datnt1998/verse/apps/noverse/internal/host/imp"
	"github.com/datnt1998/verse/apps/noverse/internal/host/sim"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/revision"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/events"
	"github.com/datnt1998/verse/platform/webguard"
)

type workerHost interface {
	// CommentStore is the backend-bound comment lifecycle (file rows or scoped
	// postgres rows); endpoints must never build it from the process cwd.
	CommentStore() *comments.Store
	Documents() *store.IO
	Snapshot() host.UISnapshot
	ModelReady() bool
	Events() <-chan host.Event
	Stream() <-chan string
	Done() <-chan struct{}
	PrepareUserRules(string) error
	StartPrepared(string) error
	Resume() (string, error)
	Continue(string, string) error
	Steer(string, string) error
	Abort() bool
	PauseAfterStep() bool
	// G4：提前放行退避等待；false = 没有请求在等待。
	RetryNow() bool
	AdvanceOneChapter() error
	AdvanceOneChapterWithComments(int, string) error
	SetAdvanceMode(domain.ChapterAdvanceMode) error
	Reopen(string) error
	Chapters() ([]host.ChapterInfo, error)
	ChapterText(int) (string, string, error)
	Export(context.Context, exp.Options) (*exp.Result, error)
	// GAP-6：共创（冷启动 / 阶段）。
	CoCreateStream(context.Context, []host.CoCreateMessage, func(string, string)) (host.CoCreateReply, error)
	StageCoCreateStream(context.Context, []host.CoCreateMessage, func(string, string)) (host.CoCreateReply, error)
	PauseForCoCreate() bool
	ResumeFromCoCreate(string) error
	CancelCoCreate()
	// GAP-8：长任务与运行时开关。
	ImportFrom(context.Context, imp.Options) (<-chan imp.Event, error)
	ImportResumeHint() string
	SimulateSource(context.Context, string) (<-chan sim.Event, error)
	ImportSimulationProfile(context.Context, string) (<-chan sim.Event, error)
	CheckRevisionChanges() ([]host.RevisionChange, error)
	RevisionDiffFor(int) (host.RevisionDiff, error)
	SyncChapterRevisions(context.Context) (*revision.Result, error)
	Diagnose() diag.Report
	DiagnoseView() host.DiagView
	SimulationProfileView() (*host.SimulationView, error)
	SimulationProfileFile() (string, error)
	CurrentModelSelection(string) (string, string, bool)
	ModelCatalog() []host.CatalogProvider
	RefreshModelContexts() error
	SwitchModel(string, string, string) error
	CurrentThinking(string) string
	AvailableThinkingLevels(string) []string
	SetRoleThinking(string, string) error
}

// workerServer 持有 worker 端点的共享状态：广播器、长任务登记表、共创会话、
// 以及 {path} 相对路径解析沙盒（故事目录 → 项目目录，绝对路径必须在书库根内）。
type workerServer struct {
	h          workerHost
	token      string
	shutdown   <-chan struct{}
	b          *broadcaster
	jobs       *jobTracker
	co         *coCreateManager
	storyDir   string
	projectDir string
	root       string
	diagMu     sync.Mutex
	diagSent   map[string]bool
	diagLast   map[string]any
	syncFixMu  sync.Mutex
	actionMu   sync.Mutex
	actionBusy bool
}

type message struct {
	kind string
	seq  uint64
	data any
}
type broadcaster struct {
	mu     sync.Mutex
	seq    uint64
	events []message
	stream string
	subs   map[chan message]struct{}
}

func newBroadcaster() *broadcaster { return &broadcaster{subs: make(map[chan message]struct{})} }
func (b *broadcaster) emit(m message) {
	if ev, ok := m.data.(host.Event); m.kind == "event" && ok && ev.Internal() {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch m.kind {
	case "event":
		b.seq++
		m.seq = b.seq
		b.events = append(b.events, m)
		if len(b.events) > 500 {
			b.events = b.events[len(b.events)-500:]
		}
	case "clear":
		b.stream = ""
	case "delta":
		b.stream += m.data.(string)
		if len(b.stream) > 65536 {
			b.stream = b.stream[len(b.stream)-65536:]
		}
	}
	for ch := range b.subs {
		select {
		case ch <- m:
		default:
			delete(b.subs, ch)
			close(ch)
		}
	}
}
func (b *broadcaster) subscribe() ([]message, string, chan message) {
	replay, stream, ch, _ := b.subscribeAt(0)
	return replay, stream, ch
}

// subscribeAt 按 Last-Event-ID 续传：只补发 seq > lastID 的事件，并报告 lastID
// 是否还在环形缓冲内。不在（第一次连接、worker 重启、掉线太久）→ resumable=false，
// 调用方应先发 reset 再全量补发。订阅与快照在同一把锁里完成，续传与实时事件不会
// 重叠，也不会漏。
func (b *broadcaster) subscribeAt(lastID uint64) (replay []message, stream string, ch chan message, resumable bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if lastID > 0 {
		resumable = lastID <= b.seq
		if resumable && len(b.events) > 0 && lastID < b.events[0].seq-1 {
			resumable = false
		}
		if resumable {
			for _, m := range b.events {
				if m.seq > lastID {
					replay = append(replay, m)
				}
			}
		}
	}
	if !resumable {
		replay = append([]message(nil), b.events...)
	}
	ch = make(chan message, 128)
	b.subs[ch] = struct{}{}
	return replay, b.stream, ch, resumable
}
func (b *broadcaster) unsubscribe(ch chan message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}

// NewWorkerHandler consumes the single-consumer host channels once and fans out bounded SSE subscriptions.
func NewWorkerHandler(h workerHost, token string) http.Handler {
	return newWorkerHandler(h, token, nil)
}

func newWorkerHandler(h workerHost, token string, shutdown <-chan struct{}) http.Handler {
	b := newBroadcaster()
	s := newWorkerServer(h, token, shutdown, b)
	go s.pump()
	return s
}

// pump 消费 Host 的单消费者通道并扇出到 SSE；快照经 s.snapshot() 叠加
// worker 侧状态（共创占用、导入待确认），保证等待原因（G5）完整。
func (s *workerServer) pump() {
	h, b, shutdown := s.h, s.b, s.shutdown
	events, stream, done := h.Events(), h.Stream(), h.Done()
	pulse := time.NewTicker(time.Second)
	defer pulse.Stop()
	refresh := time.NewTicker(5 * time.Second)
	defer refresh.Stop()
	dirty := false
	for {
		select {
		case e, ok := <-events:
			if !ok {
				events = nil
				break
			}
			b.emit(message{kind: "event", data: e})
			dirty = true
		case str, ok := <-stream:
			if !ok {
				stream = nil
				break
			}
			if str == host.StreamClearSentinel {
				b.emit(message{kind: "clear", data: map[string]any{}})
			} else {
				b.emit(message{kind: "delta", data: str})
			}
			dirty = true
		case _, ok := <-done:
			if !ok {
				done = nil
				break
			}
			b.emit(message{kind: "done", data: map[string]any{}})
			dirty = true
		case <-pulse.C:
			if dirty {
				b.emit(message{kind: "snapshot", data: s.snapshot()})
				dirty = false
			}
		case <-refresh.C:
			b.emit(message{kind: "snapshot", data: s.snapshot()})
			dirty = false
		case <-shutdown:
			return
		}
		if events == nil && stream == nil && done == nil {
			return
		}
	}
}

func newWorkerServer(h workerHost, token string, shutdown <-chan struct{}, b *broadcaster) *workerServer {
	s := &workerServer{h: h, token: token, shutdown: shutdown, b: b, jobs: newJobTracker(), co: newCoCreateManager()}
	s.jobs.notify = func(prev, next Job, existed bool) {
		b.emit(message{kind: jobEventName(prev, existed, next), data: next})
	}
	if wd, err := os.Getwd(); err == nil {
		s.storyDir = wd
	}
	s.root = os.Getenv(library.RootEnv)
	if proj := library.FindProjectDir(s.storyDir); proj != "" {
		s.projectDir = proj
	}
	return s
}

// snapshot 在 Host 快照上叠加 worker 侧才能看到的等待原因（G5）：
// 共创会话进行中 → cocreate；导入停在待确认 → confirm。
func (s *workerServer) snapshot() host.UISnapshot {
	snap := s.h.Snapshot()
	switch {
	case s.co.active():
		snap.WaitingFor = "cocreate"
	case s.importAwaitingConfirm():
		snap.WaitingFor = "confirm"
	}
	s.updateWritingJob(snap)
	return snap
}

// importAwaitingConfirm 表示导入任务停在切分确认停点，等待用户裁定。
func (s *workerServer) importAwaitingConfirm() bool {
	ev, ok := s.jobs.latest("import")
	return ok && ev.Awaiting == "confirm"
}

// active 报告共创会话是否打开（冷启动或阶段共创）。
func (m *coCreateManager) active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session != nil && !m.committed
}

func (s *workerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handle(w, r)
}

func (s *workerServer) handle(w http.ResponseWriter, r *http.Request) {
	h, token, b, shutdown := s.h, s.token, s.b, s.shutdown
	if token == "" || r.Header.Get("X-Ainovel-Worker-Token") != token {
		jsonError(w, 401, "Mã truy cập worker không hợp lệ")
		return
	}
	if r.Method == http.MethodPost && !webguard.JSONContent(r) {
		jsonError(w, 400, "Content-Type phải là application/json")
		return
	}
	if !s.ensureCoCreateLoaded(w) {
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/api/")
	if !h.ModelReady() && actionNeedsModel(r, p) {
		message := needsModelMessage
		if source, ok := h.(interface{ ReadOnlyReason() string }); ok && source.ReadOnlyReason() != "" {
			message = "Chưa thể dùng mô hình: " + source.ReadOnlyReason() + " — kiểm tra ở Cài đặt › Nhà cung cấp"
		}
		if recorder, ok := h.(interface{ RecordActionError(string, string) }); ok {
			recorder.RecordActionError(r.Header.Get("X-Ainovel-Username"), message)
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": message, "needsConfig": true})
		return
	}
	s.recordActivityInput(r, p)
	switch {
	case r.Method == "GET" && p == "snapshot":
		s.actionMu.Lock()
		pending := s.actionBusy
		s.actionMu.Unlock()
		writeJSON(w, 200, struct {
			host.UISnapshot
			// ActionPending: a dispatched model-backed action (start, steer…) is
			// still in progress even if the engine loop has not started yet.
			ActionPending bool `json:"actionPending,omitempty"`
		}{s.snapshot(), pending})
	case r.Method == "GET" && p == "events":
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		f, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, 500, "Máy chủ không hỗ trợ luồng sự kiện")
			return
		}
		replay, stream, ch, resumable := b.subscribeAt(events.ResumeID(r))
		defer b.unsubscribe(ch)
		out := events.NewDeadlineWriter(w)
		send := func(m message) bool {
			data, _ := json.Marshal(m.data)
			if m.kind == "delta" {
				data, _ = json.Marshal(map[string]string{"text": m.data.(string)})
			}
			if m.seq != 0 {
				fmt.Fprintf(out, "id: %d\n", m.seq)
			}
			_, err := fmt.Fprintf(out, "event: %s\ndata: %s\n\n", m.kind, data)
			f.Flush()
			return err == nil
		}
		// 续传（Last-Event-ID 仍在缓冲内）时不发 reset：客户端保留已收到的
		// 事件，服务端只补 seq > lastID 的部分，掉线期间不丢也不重复。
		if !resumable && !send(message{kind: "reset", data: map[string]any{}}) {
			return
		}
		if !send(message{kind: "snapshot", data: s.snapshot()}) {
			return
		}
		for _, m := range replay {
			if !send(m) {
				return
			}
		}
		if !send(message{kind: "clear", data: map[string]any{}}) {
			return
		}
		if stream != "" && !send(message{kind: "delta", data: stream}) {
			return
		}
		for _, m := range s.jobs.replays() {
			if !send(m) {
				return
			}
		}
		if !send(message{kind: "jobs.reset", data: map[string]any{"jobs": s.jobs.listJobs()}}) {
			return
		}
		ping := time.NewTicker(15 * time.Second)
		defer ping.Stop()
		for {
			select {
			case m, ok := <-ch:
				if !ok || !send(m) {
					return
				}
			case <-ping.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
				f.Flush()
			case <-r.Context().Done():
				return
			case <-shutdown:
				return
			}
		}
	case r.Method == "POST" && p == "input":
		var v struct {
			Text string `json:"text"`
		}
		if !decodeJSON(w, r, &v) {
			return
		}
		if strings.TrimSpace(v.Text) == "" {
			jsonError(w, 400, "Nội dung không được để trống")
			return
		}
		snap := s.snapshot()
		action := "continue"
		var run func() error
		switch {
		case snap.Phase == "" && snap.BookTitle == "":
			action = "start"
			if recorder, ok := h.(interface{ RecordUserEvent(string, string, string) }); ok && r.Header.Get("X-Ainovel-Username") != "" {
				recorder.RecordUserEvent(r.Header.Get("X-Ainovel-Username"), "start", v.Text)
			}
			run = func() error {
				if err := h.PrepareUserRules(v.Text); err != nil {
					return err
				}
				return h.StartPrepared(v.Text)
			}
		case snap.IsRunning:
			action = "steer"
			run = func() error { return h.Steer(v.Text, r.Header.Get("X-Ainovel-Username")) }
		default:
			run = func() error { return h.Continue(v.Text, r.Header.Get("X-Ainovel-Username")) }
		}
		pending, err := s.dispatchAction("Đang xử lý yêu cầu · kết quả sẽ hiện trong nhật ký", run)
		writeDispatched(w, pending, err, map[string]any{"ok": true, "action": action})
	case r.Method == "POST" && p == "resume":
		labels := make(chan string, 1)
		pending, err := s.dispatchAction("Đang khôi phục sáng tác", func() error {
			label, err := h.Resume()
			labels <- label
			return err
		})
		label := ""
		select {
		case label = <-labels:
		default:
		}
		writeDispatched(w, pending, err, map[string]any{"ok": true, "label": label})
	case r.Method == "POST" && p == "abort":
		var v struct {
			AfterStep bool `json:"afterStep"`
		}
		if !decodeJSON(w, r, &v) {
			return
		}
		stopped := false
		if v.AfterStep {
			stopped = h.PauseAfterStep()
		} else {
			stopped = h.Abort()
		}
		writeJSON(w, 200, map[string]any{"ok": true, "stopped": stopped})
	case r.Method == "POST" && p == "retry":
		// G4：提前放行正在退避等待的模型请求；没有等待者 → 409。
		if !h.RetryNow() {
			jsonError(w, 409, "Không có lượt nào đang chờ thử lại")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	case r.Method == "POST" && p == "next":
		s.postNext(w, r)
	case r.Method == "POST" && p == "review":
		var v struct {
			On bool `json:"on"`
		}
		if !decodeJSON(w, r, &v) {
			return
		}
		mode := domain.ChapterAdvanceMode("auto")
		if v.On {
			mode = domain.ChapterAdvanceMode("review")
		}
		commandResult(w, h.SetAdvanceMode(mode), map[string]any{"ok": true})
	case r.Method == "POST" && p == "reopen":
		var v struct {
			Direction string `json:"direction"`
		}
		if !decodeJSON(w, r, &v) {
			return
		}
		pending, err := s.dispatchAction("Đang mở lại truyện", func() error { return h.Reopen(v.Direction) })
		writeDispatched(w, pending, err, map[string]any{"ok": true})
	case r.Method == "GET" && p == "chapters":
		chapters, err := h.Chapters()
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		if chapters == nil {
			chapters = []host.ChapterInfo{}
		}
		writeJSON(w, 200, map[string]any{"chapters": chapters})
	case r.Method == "GET" && strings.HasPrefix(p, "chapters/"):
		n, err := strconv.Atoi(strings.TrimPrefix(p, "chapters/"))
		if err != nil || n <= 0 {
			jsonError(w, 404, "Không tìm thấy chương")
			return
		}
		title, text, err := h.ChapterText(n)
		if errors.Is(err, host.ErrChapterNotFound) {
			jsonError(w, 404, err.Error())
			return
		}
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"chapter": n, "title": title, "text": text})
	case r.Method == "GET" && p == "export":
		format := r.URL.Query().Get("format")
		if format != "txt" && format != "epub" && format != "narration" {
			jsonError(w, 400, "Định dạng xuất không hợp lệ (chỉ hỗ trợ txt, epub, narration)")
			return
		}
		dir, err := os.MkdirTemp("", "ainovel-export-")
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		defer os.RemoveAll(dir)
		res, err := h.Export(r.Context(), exp.Options{Format: exp.Format(format), OutPath: dir + string(os.PathSeparator), ReadNumbers: isTruthyQuery(r.URL.Query().Get("readNumbers"))})
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		file, err := os.Open(res.Path)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		defer file.Close()
		title := s.snapshot().BookTitle
		if title == "" {
			title = "book"
		}
		w.Header().Set("Content-Disposition", contentDisposition(title+exp.Format(format).FileSuffix()))
		if format == "epub" {
			w.Header().Set("Content-Type", "application/epub+zip")
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		io.Copy(w, file)
	case s.handleLive(w, r, p):
	default:
		jsonError(w, 404, "không tìm thấy")
	}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// jsonError writes an error envelope. A 500 never carries the text the caller
// passed: that text is the cause (SQL, constraint/table names, hosts, paths)
// and belongs in the server log only — see httperr.go for the policy.
func jsonError(w http.ResponseWriter, status int, msg string) {
	if status == http.StatusInternalServerError {
		logRequestFailure(errors.New(msg))
		msg = errInternalMessage
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// jsonConflictJob 是 §AE 的 409：error + jobId（让前端指回正在跑的同一任务）。
func jsonConflictJob(w http.ResponseWriter, msg, jobID string) {
	writeJSON(w, http.StatusConflict, map[string]any{"error": msg, "jobId": jobID})
}
func commandResult(w http.ResponseWriter, err error, v any) {
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, v)
}
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		jsonError(w, 400, jsonBodyMessage(err))
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		jsonError(w, 400, "Chỉ được gửi một đối tượng JSON")
		return false
	}
	return true
}
