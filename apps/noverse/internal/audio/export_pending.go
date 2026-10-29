package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportRequest persists "Ghép chạy nền" (Gói AE · 06 §AE), including failures
// for the jobs dock: POST /exports tạo một request JobQueued → runPendingExports
// chạy nó → xong JobDone, lỗi JobError (retry chạy tiếp nhờ export cache).
type ExportRequest struct {
	Volume      int             `json:"volume"`
	BookTitle   string          `json:"bookTitle"`
	Source      []SourceChapter `json:"source"`
	Options     ExportOptions   `json:"options"`
	State       string          `json:"state"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	DurationSec float64         `json:"durationSec"`

	// WaitReady = cũ "Xuất khi xong": chờ mọi chương đã chốt có giọng rồi mới chạy.
	WaitReady bool `json:"waitReady,omitempty"`
	// Tiến độ sống (không cần chính xác sau restart — cache chương lo phần resume).
	Percent      float64 `json:"percent,omitempty"`
	Step         string  `json:"step,omitempty"`
	ChaptersDone int     `json:"chaptersDone,omitempty"`
	EtaSec       int     `json:"etaSec,omitempty"`
	// Lỗi có cấu trúc theo §AE: error:{message, chapter, retryable, cause}.
	ErrorCause   string `json:"errorCause,omitempty"`
	ErrorChapter int    `json:"errorChapter,omitempty"`
	Retryable    bool   `json:"retryable,omitempty"`
	// Result 给 jobs dock 的结果卡（files[]/zipUrl）。
	Result *ExportRequestResult `json:"result,omitempty"`
}

// ExportRequestResult 对应 §AE 的 result:{files:[{name,sizeBytes,url}], zipUrl?}。
type ExportRequestResult struct {
	Files       []ExportFile `json:"files"`
	ZipURL      string       `json:"zipUrl,omitempty"`
	DurationSec float64      `json:"durationSec"`
}

// ExportBusyError 表示同一卷已经有一个 ghép 在跑/在等（POST /exports → 409+jobId）。
type ExportBusyError struct{ Volume int }

func (e *ExportBusyError) Error() string {
	return "Tập này đang có một việc ghép đang chạy."
}

// ExportStageError 带 §AE 的错误归属：原因码 + 失败的章节 + 给用户的越南语消息。
type ExportStageError struct {
	Cause   string
	Chapter int
	Msg     string
	Err     error
}

func (e *ExportStageError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "Không ghép được tập này."
}

func (e *ExportStageError) Unwrap() error { return e.Err }

const pendingExportFile = "export-requests.json"

func (s *Service) loadExportRequests() error {
	s.exports = map[int]ExportRequest{}
	data, err := os.ReadFile(filepath.Join(s.layout.AudioDir, pendingExportFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &s.exports); err != nil {
		return err
	}
	for v, request := range s.exports {
		if request.State == JobRendering {
			request.State = JobQueued
			request.Step = ""
			s.exports[v] = request
		}
	}
	return nil
}

// exportWriteFile 是落盘入口，测试可替换以观察写入时序。
var exportWriteFile = writeFileAtomic

// persistExportRequests 把当前 pending-exports 快照落盘。
//
// 关键不变量：**不得在持有 s.mu 时做文件 I/O**。writeFileAtomic 里有 fsync，
// 磁盘压力大时可能停顿数秒；若此时还占着 s.mu，ExportRequests()（HTTP 进度
// 接口、SSE 快照、测试轮询）会一起被阻塞，导出看起来"卡在 85% 不再前进"。
// 因此这里先短暂取 s.mu 只做内存序列化，释放后再写文件。写入顺序由
// exportFileMu 串行化。
func (s *Service) persistExportRequests() error {
	s.exportFileMu.Lock()
	defer s.exportFileMu.Unlock()
	s.mu.Lock()
	data, err := json.MarshalIndent(s.exports, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return exportWriteFile(filepath.Join(s.layout.AudioDir, pendingExportFile), data)
}

func (s *Service) ExportRequests() []ExportRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ExportRequest, 0, len(s.exports))
	for _, request := range s.exports {
		request.Source = nil
		result = append(result, request)
	}
	return result
}

// exportChaptersReady 报告整卷是否"每章都有已生成的段"（不含播放整章文件）。
func (s *Service) exportChaptersReady(volume int, source []SourceChapter, lockedOnly bool) bool {
	for _, chapter := range source {
		script, err := ReadScript(s.layout, volume, chapter.Chapter)
		if err != nil || script == nil || len(script.Segments) == 0 {
			return false
		}
		if lockedOnly && script.Status != StatusLocked {
			return false
		}
		for _, seg := range script.Segments {
			if seg.Rendered == nil || seg.Rendered.DurationSec <= 0 {
				return false
			}
		}
	}
	return true
}

// StartExport 登记一个后台 ghép（§AE）：同步做参数校验与"能不能跑"检查，
// 然后落盘 + kick；真正的拼接在 runPendingExports 里跑。
// 同一卷已有 JobQueued/JobRendering 的请求 → ExportBusyError（409）。
func (s *Service) StartExport(volume int, bookTitle string, source []SourceChapter, opts ExportOptions) error {
	selected, _, opts, err := normalizeExportOptions(source, volume, opts)
	if err != nil {
		return err
	}
	if !FFmpegAvailable() {
		return ErrFFmpegMissing
	}
	wait := opts.WhenReady
	if !wait && !s.exportChaptersReady(volume, selected, false) {
		// 与旧同步导出同一批校验，让表单错误立刻 409。
		for _, sc := range selected {
			script, rerr := ReadScript(s.layout, volume, sc.Chapter)
			if rerr != nil || script == nil || len(script.Segments) == 0 {
				return conflict("Chương %d chưa có bản đọc.", sc.Chapter)
			}
			for _, seg := range script.Segments {
				if seg.Rendered == nil || seg.Rendered.DurationSec <= 0 {
					return conflict("Chương %d còn đoạn chưa có giọng đọc — tạo giọng xong mới ghép được.", sc.Chapter)
				}
			}
		}
	}
	// onExists=fail 只对"立刻跑"的导出做同步检查——whenReady 的排队导出等后台
	// 跑到落盘那步才算撞车（旧语义：JobError → retry）。
	if !wait && opts.OnExists == "fail" {
		path := filepath.Join(s.layout.VolumeExportDir(volume), s.exportVolumeName(volume, bookTitle, selected, opts))
		if _, err := os.Stat(path); err == nil {
			return conflict("Tệp %s đã có. Chọn giữ cả hai hoặc thay thế.", filepath.Base(path))
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	s.mu.Lock()
	if old, ok := s.exports[volume]; ok && (old.State == JobQueued || old.State == JobRendering) {
		s.mu.Unlock()
		return &ExportBusyError{Volume: volume}
	}
	now := time.Now()
	chapters := make([]SourceChapter, len(source))
	for i, chapter := range source {
		chapters[i] = SourceChapter{Chapter: chapter.Chapter, Title: chapter.Title}
	}
	previous, existed := s.exports[volume]
	opts.WhenReady = false // gate 记在 request.WaitReady 上，重跑/续跑不再重新排队
	s.exports[volume] = ExportRequest{Volume: volume, BookTitle: bookTitle, Source: chapters,
		Options: opts, State: JobQueued, WaitReady: wait, CreatedAt: now, UpdatedAt: now, Retryable: true}
	s.mu.Unlock()
	err = s.persistExportRequests()
	if err != nil {
		s.mu.Lock()
		if current, ok := s.exports[volume]; ok && current.UpdatedAt.Equal(now) {
			if existed {
				s.exports[volume] = previous
			} else {
				delete(s.exports, volume)
			}
		}
		s.mu.Unlock()
		return err
	}
	s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": volume, "state": "waiting", "percent": 0}))
	s.kick()
	return nil
}

// scheduleExport 兼容旧调用（Export 的 WhenReady 分支）。
func (s *Service) scheduleExport(volume int, title string, source []SourceChapter, opts ExportOptions) (ExportResult, error) {
	opts.WhenReady = true
	if err := s.StartExport(volume, title, source, opts); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Files: []ExportFile{}, ChapterMarks: []ChapterMark{}, Pending: true}, nil
}

// ── 进度上报（exportReporter）──
//
// 阶段权重（§AE）：每章约等份占 render 段；整卷 nối、编码、收尾各占固定份额。
// emit 节流 ≤4/s 且只在 percent 增加时广播；每章完成与阶段切换强制发一次并落盘
// （落盘让重启续跑能跳过已完成章）。

const (
	exportRenderSpan     = 0.70 // 不 split 时各章占 0..70%
	exportRenderSpanCut  = 0.97 // split 时各章占 0..97%
	exportProgressTickMs = 250  // ≤4 次/秒
)

type exportReporter struct {
	s      *Service
	volume int

	start      time.Time
	unitsTotal float64
	unitsDone  float64
	renderSpan float64

	inPhase bool
	phaseLo float64 // 全局百分比的相位窗口 [lo, hi)
	phaseHi float64
	phaseF  float64
	curStep string
	lastPct float64
	lastAt  time.Time
	now     func() time.Time
}

func newExportReporter(s *Service, volume int) *exportReporter {
	return &exportReporter{s: s, volume: volume, start: time.Now(), now: time.Now}
}

// begin 在读完 script 后调用：units = Σ(段数+1 拼接+1 编码)。
func (r *exportReporter) begin(units float64, split bool) {
	r.unitsTotal = units
	r.renderSpan = exportRenderSpan
	if split {
		r.renderSpan = exportRenderSpanCut
	}
	r.curStep = "Đang chuẩn bị ghép"
	r.emit(true)
}

func (r *exportReporter) percent() float64 {
	if r.inPhase {
		return r.phaseLo + r.phaseF*(r.phaseHi-r.phaseLo)
	}
	if r.unitsTotal <= 0 {
		return 0
	}
	u := r.unitsDone + r.phaseF
	if u > r.unitsTotal {
		u = r.unitsTotal
	}
	return u / r.unitsTotal * r.renderSpan * 100
}

// emit 节流：percent 增加或步骤变化才考虑发送，且距上次 ≥250ms；
// force=true（章完成 / 阶段切换 / 收尾）直接发并落盘。
func (r *exportReporter) emit(force bool) {
	if r == nil {
		return
	}
	now := r.now()
	pct := r.percent()
	increased := pct > r.lastPct+0.0001
	due := now.Sub(r.lastAt) >= exportProgressTickMs*time.Millisecond
	if !force && !(increased && due) {
		return
	}
	if pct < r.lastPct {
		pct = r.lastPct // 单调：编码等相位换算偶尔抖动时不下掉
	}
	r.lastPct = pct
	r.lastAt = now
	eta := 0
	elapsed := now.Sub(r.start).Seconds()
	if pct > 2 {
		eta = int(elapsed/pct*(100-pct) + 0.5)
	}
	s := r.s
	s.mu.Lock()
	persist := false
	if req, ok := s.exports[r.volume]; ok && req.State == JobRendering {
		req.Percent = pct
		req.Step = r.curStep
		req.EtaSec = eta
		req.UpdatedAt = now
		s.exports[r.volume] = req
		persist = force
	}
	s.mu.Unlock()
	// 落盘在锁外：fsync 停顿不能阻塞 ExportRequests()/SSE 读者。
	if persist {
		if err := s.persistExportRequests(); err != nil {
			s.logf("Không lưu được tiến độ xuất audio", "error", err)
		}
	}
	s.hub.Publish(NewEvent(EventExport, map[string]any{
		"volume": r.volume, "percent": pct, "state": "rendering", "step": r.curStep, "etaSec": eta,
	}))
}

// step 更新显示的步骤文案（越南语），进度允许时立刻广播。
func (r *exportReporter) step(text string) {
	if r == nil {
		return
	}
	r.curStep = text
	r.emit(false)
}

// advance 计完 n 个工作单元（每段 1、每章拼接 1、每章编码 1）。
func (r *exportReporter) advance(n float64) {
	if r == nil {
		return
	}
	r.unitsDone += n
	r.phaseF = 0
	r.emit(false)
}

// frac 报告当前 ffmpeg 调用的 0..1 进度（concat / encode 内部）。
func (r *exportReporter) frac(f float64) {
	if r == nil {
		return
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	r.phaseF = f
	r.emit(false)
}

// chapterDone 每章完成：强制广播 + 落盘（重启续跑靠它数已完成章）。
func (r *exportReporter) chapterDone(chapter int) {
	if r == nil {
		return
	}
	s := r.s
	s.mu.Lock()
	if req, ok := s.exports[r.volume]; ok && req.State == JobRendering {
		req.ChaptersDone++
		s.exports[r.volume] = req
	}
	s.mu.Unlock()
	r.emit(true)
}

// beginPhase/endPhase 进入/离开"整卷"阶段（nối → encode → 收尾），
// 各拿固定的全局百分比窗口，窗口内进度由 frac 填。
func (r *exportReporter) beginPhase(lo, hi float64) {
	if r == nil {
		return
	}
	r.inPhase = true
	r.phaseLo, r.phaseHi, r.phaseF = lo*100, hi*100, 0
}

func (r *exportReporter) endPhase() {
	if r == nil {
		return
	}
	r.phaseF = 1
	r.emit(true)
	r.inPhase = false
	r.phaseLo = r.phaseHi
	r.phaseF = 0
	r.unitsDone = r.unitsTotal // 相位窗口接管渲染段的进度，归一化防回退
}

// finish 把状态推满 100%（runner 在写结果前调用）。
func (r *exportReporter) finish() {
	if r == nil {
		return
	}
	r.inPhase = false
	r.lastPct = 0
	r.unitsDone = r.unitsTotal
	s := r.s
	s.mu.Lock()
	if req, ok := s.exports[r.volume]; ok && req.State == JobRendering {
		req.Percent = 100
		req.Step = ""
		req.EtaSec = 0
		s.exports[r.volume] = req
	}
	s.mu.Unlock()
	s.hub.Publish(NewEvent(EventExport, map[string]any{
		"volume": r.volume, "percent": float64(100), "state": "rendering", "etaSec": 0,
	}))
}

// ── 后台执行 ──

func (s *Service) runPendingExports() {
	for _, snapshot := range s.ExportRequests() {
		if snapshot.State != JobQueued {
			continue
		}
		s.mu.Lock()
		request := s.exports[snapshot.Volume]
		s.mu.Unlock()
		if request.State != JobQueued {
			continue
		}
		selected, _, selectionErr := selectExportChapters(request.Source, request.Volume, request.Options.Chapters)
		if request.WaitReady && selectionErr == nil && !s.exportChaptersReady(request.Volume, selected, true) {
			continue
		}
		if s.ctx.Err() != nil {
			return
		}
		ctx, cancel := context.WithTimeout(s.ctx, 20*time.Minute)
		s.mu.Lock()
		current := s.exports[request.Volume]
		if current.State != JobQueued || !current.CreatedAt.Equal(request.CreatedAt) {
			s.mu.Unlock()
			cancel()
			continue
		}
		request.State = JobRendering
		request.Percent = 0
		request.Step = "Đang chuẩn bị ghép"
		request.UpdatedAt = time.Now()
		s.exports[request.Volume] = request
		// cancel 必须与 JobRendering 同时可见，否则并发的取消会漏掉运行中的导出。
		if s.exportCancels == nil {
			s.exportCancels = make(map[int]context.CancelFunc)
		}
		s.exportCancels[request.Volume] = cancel
		s.mu.Unlock()
		err := s.persistExportRequests()
		if err != nil {
			s.mu.Lock()
			s.exports[request.Volume] = current
			delete(s.exportCancels, request.Volume)
			s.mu.Unlock()
			cancel()
			s.logf("Không lưu được yêu cầu xuất audio", "error", err)
			continue
		}
		prog := newExportReporter(s, request.Volume)
		result, err := s.runExport(ctx, request.Volume, request.BookTitle, request.Source, request.Options, prog)
		cancel()
		request.State = JobDone
		request.Percent = 100
		request.Step = ""
		request.EtaSec = 0
		request.DurationSec = result.DurationSec
		request.Result = &ExportRequestResult{Files: result.Files, ZipURL: result.ZipURL, DurationSec: result.DurationSec}
		if err != nil {
			request.State = JobError
			request.Result = nil
			request.Error = err.Error()
			request.Retryable = true
			var stage *ExportStageError
			if errors.As(err, &stage) {
				request.Error = stage.Error()
				request.ErrorCause = stage.Cause
				request.ErrorChapter = stage.Chapter
			}
		}
		if s.ctx.Err() != nil {
			request.State = JobQueued
			request.Error = ""
			request.ErrorCause = ""
			request.ErrorChapter = 0
			request.Result = nil
		}
		request.UpdatedAt = time.Now()
		s.mu.Lock()
		delete(s.exportCancels, request.Volume)
		current = s.exports[request.Volume]
		if current.State == JobCanceled || !current.CreatedAt.Equal(request.CreatedAt) {
			s.mu.Unlock()
			continue
		}
		s.exports[request.Volume] = request
		s.mu.Unlock()
		saveErr := s.persistExportRequests()
		if saveErr != nil {
			s.logf("Không lưu được kết quả xuất audio", "error", saveErr)
		}
		s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": request.Volume, "state": request.State, "percent": 100, "durationSec": request.DurationSec, "error": request.Error}))
	}
}

// ControlExport controls the persisted export job without changing render jobs.
func (s *Service) ControlExport(volume int, op string) error {
	return s.ControlExportBody(volume, op, nil)
}

// ControlExportBody 与 ControlExport 相同，retry 额外读 {intro:false}
// （"Ghép không nhạc"——§AE：从失败章继续，但丢掉片头/片尾音乐）。
func (s *Service) ControlExportBody(volume int, op string, body map[string]any) error {
	s.mu.Lock()
	request, ok := s.exports[volume]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	previous := request
	switch op {
	case "retry":
		if request.State != JobError {
			s.mu.Unlock()
			return conflict("Chỉ thử lại được việc xuất bị lỗi.")
		}
		if body != nil {
			if intro, ok := body["intro"].(bool); ok && !intro {
				request.Options.Intro = false
			}
		}
		request.State = JobQueued
		request.Error = ""
		request.ErrorCause = ""
		request.ErrorChapter = 0
		request.Retryable = false
		request.Percent = 0
		request.Step = ""
		request.ChaptersDone = 0
		request.Result = nil
	case "cancel":
		if request.State == JobDone || request.State == JobCanceled {
			s.mu.Unlock()
			return conflict("Việc xuất này đã kết thúc.")
		}
		request.State = JobCanceled
		request.Error = ""
	default:
		s.mu.Unlock()
		return conflict("Thao tác xuất không được hỗ trợ.")
	}
	request.UpdatedAt = time.Now()
	s.exports[volume] = request
	cancel := s.exportCancels[volume]
	s.mu.Unlock()
	err := s.persistExportRequests()
	if err != nil {
		s.mu.Lock()
		s.exports[volume] = previous
		s.mu.Unlock()
		return err
	}
	if op == "cancel" && cancel != nil {
		cancel()
	}
	state := request.State
	if state == JobQueued {
		state = "waiting"
	}
	s.hub.Publish(NewEvent(EventExport, map[string]any{"volume": volume, "state": state, "percent": 0}))
	if op == "retry" {
		s.kick()
	}
	return nil
}

func (s *Service) synthesisText(text, tone string) string {
	if !s.synth.Capabilities().EmotionCues {
		return text
	}
	// Only the documented nonverbal cues belong in synthesis text; other
	// delivery directions remain in SynthRequest.Tone.
	switch cue := strings.ToLower(strings.TrimSpace(tone)); cue {
	case "cười", "thở dài", "hắng giọng":
		return "[" + cue + "] " + text
	default:
		return text
	}
}

func chapterMetadata(marks []ChapterMark, duration float64) string {
	var b strings.Builder
	b.WriteString(";FFMETADATA1\n")
	for i, mark := range marks {
		end := duration
		if i+1 < len(marks) {
			end = marks[i+1].AtSec
		}
		fmt.Fprintf(&b, "[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n",
			int64(mark.AtSec*1000), int64(end*1000), mark.Label)
	}
	return b.String()
}
