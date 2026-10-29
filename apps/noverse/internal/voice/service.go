package voice

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Priority quyết định thứ tự rời hàng đợi. Nghe thử luôn đi trước tạo hàng loạt:
// người dùng đang chờ nghe một đoạn, còn tạo hàng loạt thì chạy nền.
type Priority int

const (
	// PriorityPreview là nghe thử (một đoạn ngắn, người dùng đang đợi).
	PriorityPreview Priority = iota
	// PriorityRender là tạo hàng loạt cho audio truyện / đọc to.
	PriorityRender
)

// Request là một lượt đọc kèm thông tin hàng đợi.
type Request struct {
	SynthRequest
	// JobID nhóm các lượt đọc lại để Pause/Resume/Cancel cùng lúc (vd một chương).
	// Rỗng = lượt lẻ.
	JobID    string   `json:"jobId,omitempty"`
	Priority Priority `json:"-"`
}

// JobState là tình trạng một job trong hàng đợi.
type JobState struct {
	JobID   string `json:"jobId"`
	Paused  bool   `json:"paused"`
	Queued  int    `json:"queued"`
	Running int    `json:"running"`
}

// InstallProgress là một mốc tiến độ cài bộ đọc.
type InstallProgress struct {
	State   string    `json:"state"` // idle | running | done | error
	Step    string    `json:"step"`
	Percent int       `json:"percent"`
	Message string    `json:"message"`
	Bytes   int64     `json:"bytes"` // dung lượng đã tải (mô hình) tại mốc này
	Error   string    `json:"error,omitempty"`
	Updated time.Time `json:"updated"`
}

// InstallRunning cho biết tiến độ có đang chạy hay không.
func (p InstallProgress) InstallRunning() bool { return p.State == "running" }

type taskResult struct {
	audio Audio
	err   error
}

type task struct {
	id       string
	req      SynthRequest
	job      string
	prio     Priority
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan taskResult
	finished chan struct{} // đóng khi vé đã có kết quả
	// Các trường dưới đây chỉ đọc/ghi khi giữ Service.mu.
	settled bool
	running bool
}

type jobInfo struct {
	paused bool
}

// Ticket là vé của một lượt đọc đã xếp hàng.
type Ticket struct {
	svc  *Service
	id   string
	done chan taskResult
}

// ID trả mã lượt đọc (dùng cho log/nhật ký).
func (t *Ticket) ID() string { return t.id }

// Wait chờ kết quả. ctx huỷ thì trả lỗi của ctx; huỷ qua Cancel thì trả ErrCanceled.
func (t *Ticket) Wait(ctx context.Context) (Audio, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case res := <-t.done:
		return res.audio, res.err
	case <-ctx.Done():
		return Audio{}, ctx.Err()
	}
}

// Cancel huỷ riêng lượt đọc này (đang xếp hàng hoặc đang chạy).
func (t *Ticket) Cancel() {
	if t.svc != nil {
		t.svc.cancelTasks(func(x *task) bool { return x.id == t.id })
	}
}

// Service là hàng đợi dùng chung: mỗi engine một hàng đợi một luồng, ưu tiên
// preview > render, Pause/Resume/Cancel theo job, đo RTF.
type Service struct {
	cfg Config

	mu     sync.Mutex
	cond   *sync.Cond
	engine Engine
	engErr error

	preview []*task
	render  []*task
	tasks   map[string]*task
	jobs    map[string]*jobInfo
	seq     int
	worker  bool
	closed  bool

	rtf     float64
	rtfSeen bool

	install     InstallProgress
	installStop context.CancelFunc
	installSubs map[int]func(InstallProgress)
	subNext     int
}

// New dựng Service. Engine được tạo lười ở lần dùng đầu để khởi động nhanh.
func New(cfg Config) *Service {
	s := &Service{
		cfg:         cfg.WithDefaults(),
		tasks:       map[string]*task{},
		jobs:        map[string]*jobInfo{},
		installSubs: map[int]func(InstallProgress){},
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Config trả cấu hình đang dùng.
func (s *Service) Config() Config { return s.cfg }

// engineLocked trả engine, dựng ở lần gọi đầu.
func (s *Service) engineLocked() (Engine, error) {
	if s.engine != nil {
		return s.engine, nil
	}
	if s.engErr != nil {
		return nil, s.engErr
	}
	eng, err := NewEngine(s.cfg)
	if err != nil {
		s.engErr = err
		return nil, err
	}
	s.engine = eng
	return eng, nil
}

// EngineName trả tên engine đang cấu hình.
func (s *Service) EngineName() string { return s.cfg.Engine }

// Capabilities trả năng lực engine (rỗng nếu engine không dựng được).
func (s *Service) Capabilities() Capabilities {
	s.mu.Lock()
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return Capabilities{SampleRate: 48000, Qualities: Qualities}
	}
	return eng.Capabilities()
}

// statusTimeout là trần một lần hỏi trạng thái engine. Status là đường đọc (màn
// audio, Cài đặt) nên không bao giờ được chờ engine vô hạn: máy đọc treo thì trả
// "error" sau trần này để UI hiện lỗi thay vì quay mãi.
var statusTimeout = 5 * time.Second

// Status trả trạng thái engine, đã gộp trạng thái cài đặt và RTF đo được.
func (s *Service) Status(ctx context.Context) Status {
	s.mu.Lock()
	inst := s.install
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return Status{Engine: s.cfg.Engine, Model: s.cfg.Model, Device: s.cfg.Device, State: StateError, Error: err.Error()}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	st := eng.Status(callCtx)
	if st.Engine == "" {
		st.Engine = s.cfg.Engine
	}
	if callCtx.Err() != nil && !st.Ready && !inst.InstallRunning() {
		st.State = StateError
		st.Error = "máy đọc không phản hồi kịp"
	}
	switch {
	case inst.InstallRunning():
		st.State = StateInstalling
		st.Ready = false
		st.Error = ""
	case inst.State == "error" && inst.Error != "" && !st.Ready:
		// Lượt cài vừa hỏng: trạng thái engine vẫn là "chưa cài", nhưng UI cần câu
		// lỗi thật (S/audio/ready → message) thay vì câu chung "chưa sẵn sàng".
		st.Error = inst.Error
	}
	// Engine nào tự đo RTF thì lấy số của nó (loại trừ thời gian khởi động, nên đúng
	// hơn số đo bao từ tầng này — vốn tính cả lần bật máy đọc đầu tiên).
	if rep, ok := eng.(RTFReporter); ok {
		st.RTF = rep.RTF()
	} else {
		st.RTF = s.RTF()
	}
	return st
}

// Installed cho biết engine có cần cài và đã cài chưa.
func (s *Service) Installed() bool {
	s.mu.Lock()
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return false
	}
	ins, ok := eng.(Installer)
	if !ok {
		return true // engine từ xa không cần cài
	}
	return ins.Installed()
}

// DiskUsage là dung lượng engine chiếm trên đĩa (0 nếu engine không báo được).
func (s *Service) DiskUsage() int64 {
	s.mu.Lock()
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return 0
	}
	if sz, ok := eng.(Sizer); ok {
		return sz.DiskUsage()
	}
	return 0
}

// Voices liệt kê giọng engine biết (bật máy đọc nếu đang tắt).
func (s *Service) Voices(ctx context.Context) ([]Voice, error) {
	s.mu.Lock()
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return eng.Voices(ctx)
}

// AddVoice thêm giọng clone từ clip mẫu.
func (s *Service) AddVoice(ctx context.Context, name, wavPath string) (Voice, error) {
	s.mu.Lock()
	eng, err := s.engineLocked()
	s.mu.Unlock()
	if err != nil {
		return Voice{}, err
	}
	if !eng.Capabilities().Cloning {
		return Voice{}, fmt.Errorf("%w: máy đọc %q không clone giọng", ErrUnsupported, eng.Name())
	}
	return eng.AddVoice(ctx, name, wavPath)
}

// Synthesize xếp một lượt đọc vào hàng tạo hàng loạt rồi chờ kết quả.
func (s *Service) Synthesize(ctx context.Context, req SynthRequest) (Audio, error) {
	t, err := s.Submit(ctx, Request{SynthRequest: req, Priority: PriorityRender})
	if err != nil {
		return Audio{}, err
	}
	return t.Wait(ctx)
}

// Preview xếp một lượt nghe thử (ưu tiên cao hơn render) rồi chờ kết quả.
func (s *Service) Preview(ctx context.Context, req SynthRequest) (Audio, error) {
	t, err := s.Submit(ctx, Request{SynthRequest: req, Priority: PriorityPreview})
	if err != nil {
		return Audio{}, err
	}
	return t.Wait(ctx)
}

// Submit xếp một lượt đọc và trả vé ngay, không chờ.
func (s *Service) Submit(ctx context.Context, req Request) (*Ticket, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("chữ cần đọc đang trống")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req.SynthRequest = req.SynthRequest.Normalized()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("dịch vụ giọng đọc đã đóng")
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.seq++
	tctx, cancel := context.WithCancel(ctx)
	t := &task{
		id:       "v" + strconv.Itoa(s.seq),
		req:      req.SynthRequest,
		job:      req.JobID,
		prio:     req.Priority,
		ctx:      tctx,
		cancel:   cancel,
		done:     make(chan taskResult, 1),
		finished: make(chan struct{}),
	}
	s.tasks[t.id] = t
	if t.prio == PriorityPreview {
		s.preview = append(s.preview, t)
	} else {
		s.render = append(s.render, t)
	}
	s.ensureWorkerLocked()
	s.mu.Unlock()
	// Người gọi huỷ ctx thì vé phải trả lỗi ngay, kể cả khi máy đọc còn đang bận việc
	// khác — không đợi đến lượt mới biết là đã huỷ.
	go s.watchContext(t)
	return &Ticket{svc: s, id: t.id, done: t.done}, nil
}

func (s *Service) ensureWorkerLocked() {
	if s.worker || s.closed {
		return
	}
	s.worker = true
	go s.loop()
}

// loop là luồng duy nhất gọi engine.Synthesize, nên máy đọc không bao giờ bị gọi
// song song từ hàng đợi này.
func (s *Service) loop() {
	for {
		s.mu.Lock()
		if s.closed {
			s.drainLocked(errors.New("dịch vụ giọng đọc đã đóng"))
			s.worker = false
			s.mu.Unlock()
			return
		}
		t := s.pickLocked()
		if t == nil {
			if len(s.preview) == 0 && len(s.render) == 0 {
				s.worker = false
				s.mu.Unlock()
				return
			}
			// Còn việc nhưng đều đang tạm dừng: chờ Resume/Submit.
			s.cond.Wait()
			s.mu.Unlock()
			continue
		}
		eng, err := s.engineLocked()
		t.running = true
		s.mu.Unlock()

		if err != nil {
			s.mu.Lock()
			s.settleLocked(t, taskResult{err: err})
			s.mu.Unlock()
			t.cancel()
			continue
		}

		start := time.Now()
		audio, synthErr := eng.Synthesize(t.ctx, t.req)
		if synthErr == nil {
			s.observeRTF(time.Since(start), audio)
		}
		s.mu.Lock()
		if synthErr != nil && t.ctx.Err() != nil && errors.Is(synthErr, context.Canceled) {
			synthErr = ErrCanceled
		}
		s.settleLocked(t, taskResult{audio: audio, err: synthErr})
		s.mu.Unlock()
		t.cancel()
	}
}

// pickLocked lấy việc kế tiếp: preview trước, rồi render, bỏ qua việc đã huỷ và
// job đang tạm dừng.
func (s *Service) pickLocked() *task {
	for {
		t := s.popRunnableLocked(&s.preview)
		if t == nil {
			t = s.popRunnableLocked(&s.render)
		}
		if t == nil {
			return nil
		}
		if err := t.ctx.Err(); err != nil {
			s.settleLocked(t, taskResult{err: err})
			t.cancel()
			continue
		}
		return t
	}
}

func (s *Service) popRunnableLocked(q *[]*task) *task {
	for i, t := range *q {
		if t.settled || t.running {
			*q = append((*q)[:i], (*q)[i+1:]...)
			return s.popRunnableLocked(q)
		}
		// jobs[""] chỉ tồn tại sau khi Pause("") → việc lẻ cũng bị giữ lại đúng
		// nghĩa "tạm dừng cả hàng đợi"; job có tên theo trạng thái riêng của nó.
		if j, ok := s.jobs[t.job]; ok && j.paused {
			continue
		}
		*q = append((*q)[:i], (*q)[i+1:]...)
		return t
	}
	return nil
}

// watchContext chốt vé ngay khi ctx của người gọi bị huỷ.
func (s *Service) watchContext(t *task) {
	select {
	case <-t.ctx.Done():
	case <-t.finished:
		return
	}
	s.mu.Lock()
	if s.settleLocked(t, taskResult{err: t.ctx.Err()}) {
		s.dropLocked(t)
	}
	s.mu.Unlock()
	t.cancel()
}

// dropLocked bỏ một vé khỏi hai hàng đợi (vé đã chốt thì không cần giữ chỗ).
func (s *Service) dropLocked(t *task) {
	s.preview = filterTasks(s.preview, func(x *task) bool { return x != t })
	s.render = filterTasks(s.render, func(x *task) bool { return x != t })
}

// settleLocked giao kết quả đúng một lần cho vé và dọn khỏi sổ.
func (s *Service) settleLocked(t *task, res taskResult) bool {
	if t.settled {
		return false
	}
	t.settled = true
	close(t.finished)
	delete(s.tasks, t.id)
	select {
	case t.done <- res:
	default:
	}
	return true
}

func (s *Service) drainLocked(err error) {
	for _, t := range append(append([]*task{}, s.preview...), s.render...) {
		s.settleLocked(t, taskResult{err: err})
		t.cancel()
	}
	s.preview, s.render = nil, nil
}

// Pause tạm dừng nhận việc mới của một job (jobID rỗng = cả hàng đợi). Việc đang
// chạy không bị cắt giữa chừng — đó là việc của Cancel.
func (s *Service) Pause(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("dịch vụ giọng đọc đã đóng")
	}
	if jobID == "" {
		s.jobs[""] = &jobInfo{paused: true}
		for _, j := range s.jobs {
			j.paused = true
		}
	} else {
		s.jobs[jobID] = &jobInfo{paused: true}
	}
	s.cond.Broadcast()
	return nil
}

// Resume cho job chạy tiếp.
func (s *Service) Resume(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID == "" {
		for _, j := range s.jobs {
			j.paused = false
		}
		delete(s.jobs, "")
	} else if j, ok := s.jobs[jobID]; ok {
		j.paused = false
	}
	s.cond.Broadcast()
	return nil
}

// Paused cho biết job có đang tạm dừng không.
func (s *Service) Paused(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[jobID]
	return ok && j.paused
}

// Cancel huỷ mọi việc của một job (jobID rỗng = cả hàng đợi), kể cả việc đang
// chạy. Trả số việc đã huỷ.
func (s *Service) Cancel(jobID string) (int, error) {
	return s.cancelTasks(func(t *task) bool { return jobID == "" || t.job == jobID }), nil
}

func (s *Service) cancelTasks(match func(*task) bool) int {
	s.mu.Lock()
	var victims []*task
	for _, t := range s.tasks {
		if match(t) {
			victims = append(victims, t)
		}
	}
	s.preview = filterTasks(s.preview, func(t *task) bool { return !match(t) })
	s.render = filterTasks(s.render, func(t *task) bool { return !match(t) })
	for _, t := range victims {
		s.settleLocked(t, taskResult{err: ErrCanceled})
	}
	s.mu.Unlock()
	for _, t := range victims {
		t.cancel() // cắt luôn lượt đang chạy trong engine
	}
	return len(victims)
}

func filterTasks(q []*task, keep func(*task) bool) []*task {
	out := q[:0]
	for _, t := range q {
		if keep(t) {
			out = append(out, t)
		}
	}
	return out
}

// JobState trả tình trạng một job (jobID rỗng = toàn hàng đợi).
func (s *Service) JobState(jobID string) JobState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := JobState{JobID: jobID}
	if j, ok := s.jobs[jobID]; ok {
		st.Paused = j.paused
	}
	for _, t := range s.tasks {
		if jobID != "" && t.job != jobID {
			continue
		}
		if t.running {
			st.Running++
		} else {
			st.Queued++
		}
	}
	return st
}

func (s *Service) observeRTF(wall time.Duration, a Audio) {
	if wall <= 0 || a.DurationSec < 0.2 {
		return
	}
	v := wall.Seconds() / a.DurationSec
	s.mu.Lock()
	if !s.rtfSeen {
		s.rtf, s.rtfSeen = v, true
	} else {
		s.rtf = 0.7*s.rtf + 0.3*v
	}
	s.mu.Unlock()
}

// RTF trả tỉ lệ thời gian tạo / thời lượng audio gần đây (nil = chưa đo được).
func (s *Service) RTF() *float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.rtfSeen {
		return nil
	}
	v := s.rtf
	return &v
}

// installTimeout là trần thời gian một lượt cài. Tải mã nguồn + thư viện + mô hình
// ~600 MB mất vài phút; quá trần này nghĩa là mạng treo, và UI không được phép
// đứng ở “Đang cài…” mãi — biến thành lỗi để người dùng thử lại.
var installTimeout = 30 * time.Minute

// StartInstall chạy cài bộ đọc ở nền; tiến độ xem qua InstallState/SubscribeInstall.
// ctx huỷ thì lượt cài dừng (CLI truyền ctx của nó, gateway truyền context.Background).
func (s *Service) StartInstall(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.install.InstallRunning() {
		s.mu.Unlock()
		return ErrInstallRunning
	}
	eng, err := s.engineLocked()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	ins, ok := eng.(Installer)
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("%w: máy đọc %q không cài trên máy", ErrUnsupported, eng.Name())
	}
	runCtx, cancel := context.WithTimeout(ctx, installTimeout)
	s.installStop = cancel
	s.mu.Unlock()

	s.setInstall(InstallProgress{State: "running", Step: "prepare", Message: "Đang chuẩn bị cài bộ đọc…"})

	go func() {
		installErr := ins.Install(runCtx, func(p InstallProgress) {
			p.State = "running"
			s.setInstall(p)
		})
		final := InstallProgress{State: "done", Step: "done", Percent: 100, Message: "Đã cài xong bộ đọc"}
		if installErr != nil {
			final = InstallProgress{State: "error", Step: "error", Message: "Cài bộ đọc thất bại", Error: installErr.Error()}
			switch {
			case errors.Is(installErr, context.Canceled):
				final.Message = "Đã dừng cài bộ đọc"
				final.Error = ""
			case errors.Is(installErr, context.DeadlineExceeded):
				final.Message = "Cài bộ đọc quá lâu"
				final.Error = "Quá 30 phút mà chưa cài xong — kiểm tra mạng rồi thử lại"
			}
		}
		s.mu.Lock()
		s.installStop = nil
		s.mu.Unlock()
		cancel()
		s.setInstall(final)
	}()
	return nil
}

// CancelInstall dừng lượt cài đang chạy.
func (s *Service) CancelInstall() {
	s.mu.Lock()
	stop := s.installStop
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// InstallState trả mốc tiến độ mới nhất.
func (s *Service) InstallState() InstallProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.install
}

// SubscribeInstall đăng ký nhận tiến độ cài; trả hàm huỷ đăng ký. Callback chạy
// trên goroutine của lượt cài nên không được gọi ngược vào Service.
func (s *Service) SubscribeInstall(fn func(InstallProgress)) func() {
	if fn == nil {
		return func() {}
	}
	s.mu.Lock()
	id := s.subNext
	s.subNext++
	s.installSubs[id] = fn
	cur := s.install
	s.mu.Unlock()
	if cur.State != "" {
		fn(cur)
	}
	return func() {
		s.mu.Lock()
		delete(s.installSubs, id)
		s.mu.Unlock()
	}
}

func (s *Service) setInstall(p InstallProgress) {
	if p.Percent < 0 {
		p.Percent = 0
	}
	if p.Percent > 100 {
		p.Percent = 100
	}
	p.Updated = time.Now()
	s.mu.Lock()
	s.install = p
	subs := make([]func(InstallProgress), 0, len(s.installSubs))
	for _, fn := range s.installSubs {
		subs = append(subs, fn)
	}
	s.mu.Unlock()
	for _, fn := range subs {
		fn(p)
	}
}

// Close dừng engine (tắt tiến trình con) và huỷ mọi việc còn trong hàng đợi.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	stop := s.installStop
	eng := s.engine
	s.drainLocked(errors.New("dịch vụ giọng đọc đã đóng"))
	s.cond.Broadcast()
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
	if eng != nil {
		return eng.Close()
	}
	return nil
}
