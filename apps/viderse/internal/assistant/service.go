// Package assistant biến một câu tiếng Việt thành đúng các thao tác mà giao diện
// đã có. Mô hình Đạo diễn chỉ PHÂN LOẠI (schema cố định, câu người dùng chỉ là
// dữ liệu); mọi id do code phân giải từ dữ liệu thật; không có gì chạy trước
// khi người dùng xác nhận bằng `Run` / `RunPlan`, và mỗi lần chạy gọi lại đúng
// các service mà nút bấm gọi (xem bảng ở intents.go).
package assistant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/core/llmcontract"
)

const (
	planTTL       = 30 * time.Minute
	runTTL        = 30 * time.Minute
	maxPlans      = 200
	rateWindow    = time.Minute
	rateMax       = 12
	taskObjectTyp = "assistant"
)

// Service là trợ lý. Zero value không dùng được; dựng bằng New.
type Service struct {
	d   Deps
	cls classifier
	now func() time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu    sync.Mutex
	plans map[string]*storedPlan
	runs  map[string]*runState
	calls []time.Time
}

type storedPlan struct {
	scope   Scope
	steps   []Step
	made    time.Time
	started map[string]bool
}

// New dựng Service. Deps.Director là bắt buộc để phân loại; thiếu service nào
// thì intent cần service đó báo lỗi thật khi phân giải/chạy.
func New(d Deps) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		d: d, cls: classifier{model: d.Director}, now: time.Now,
		ctx: ctx, cancel: cancel,
		plans: map[string]*storedPlan{}, runs: map[string]*runState{},
	}
}

// Close dừng các lượt chạy nền và đợi chúng thoát.
func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Service) diag(err error) {
	if s.d.Diag != nil && err != nil {
		s.d.Diag(err)
	}
}

func newID(prefix string) string {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand không lỗi trên nền tảng hỗ trợ
	}
	return prefix + hex.EncodeToString(b)
}

// ── plan ────────────────────────────────────────────────────────────────────

// Plan phân loại câu rồi dựng kế hoạch (1–5 bước), câu hỏi làm rõ hoặc thông
// báo chưa làm được. KHÔNG chạy gì. answer (nếu có) là lựa chọn người dùng vừa
// bấm cho câu hỏi trước; nó chỉ là dữ liệu bổ sung của câu.
func (s *Service) Plan(ctx context.Context, scope Scope, text string) (Reply, error) {
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n == 0 || n > MaxTextRunes {
		return Reply{}, fail(CodeTextInvalid, "Câu nhắn phải từ 1 tới %d ký tự.", MaxTextRunes)
	}
	d, err := s.loadScope(ctx, scope)
	if err != nil {
		var ae *Error
		if errors.As(err, &ae) {
			return Reply{}, err
		}
		s.diag(err)
		return Reply{}, fail(CodeAssistantFailed, "Trợ lý chưa đọc được dữ liệu. Thử lại sau ít phút.")
	}
	if !s.allow() {
		return Reply{}, fail(CodeRateLimited, "Bạn nhắn hơi nhanh. Đợi một chút rồi nhắn lại.")
	}
	model, err := s.cls.classify(ctx, d.payload(text))
	if err != nil {
		return Reply{}, s.classifyError(ctx, err)
	}
	switch model.Outcome {
	case outcomeAsk:
		ask := &Ask{Question: cleanLine(model.Ask.Question)}
		for _, o := range model.Ask.Options {
			ask.Options = append(ask.Options, askOption(text, cleanLine(o)))
		}
		return Reply{Ask: ask}, nil
	case outcomeUnsupported:
		kind := unsupportedOther
		if model.UnsupportedKind != nil {
			kind = *model.UnsupportedKind
		}
		return *s.unknown(d, kind), nil
	}
	var steps []resolvedStep
	for _, m := range model.Steps {
		r := s.resolveStep(ctx, d, text, m)
		if r.reply != nil {
			return *r.reply, nil
		}
		steps = append(steps, r.steps...)
	}
	if len(steps) > MaxSteps {
		return *s.tooMany(d), nil
	}
	return s.buildPlan(ctx, scope, d, steps)
}

func (s *Service) classifyError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, errNoModel) {
		return fail(CodeModelUnavailable, "Trợ lý cần model Đạo diễn. Đăng nhập ChatGPT ở Cài đặt › Kết nối AI rồi nhắn lại.")
	}
	var f *llmcontract.Failure
	if errors.As(err, &f) && f.Kind == llmcontract.FailureRequest {
		s.diag(err)
		return fail(CodeModelUnavailable, "Model Đạo diễn chưa trả lời được. Kiểm tra kết nối AI rồi nhắn lại.")
	}
	s.diag(err)
	return fail(CodeAssistantFailed, "Trợ lý chưa hiểu được câu này từ model. Thử nhắn lại, hoặc đổi cách nói.")
}

// allow giới hạn số câu phân loại mỗi phút: mỗi câu là một lượt gọi model.
func (s *Service) allow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	kept := s.calls[:0]
	for _, t := range s.calls {
		if now.Sub(t) < rateWindow {
			kept = append(kept, t)
		}
	}
	s.calls = kept
	if len(s.calls) >= rateMax {
		return false
	}
	s.calls = append(s.calls, now)
	return true
}

func (s *Service) buildPlan(ctx context.Context, scope Scope, d *scopeData, steps []resolvedStep) (Reply, error) {
	plan := &Plan{ID: newID("plan_")}
	for _, st := range steps {
		p, err := s.prepare(ctx, st.intent, mustParams(st.params))
		if err != nil {
			// Một bước không dựng được (dữ liệu vừa đổi, trùng…) → nói thật lý do.
			u := s.unknown(d, unsupportedOther)
			u.Unknown.Lead = stepLead(err)
			return *u, nil
		}
		p.step.ID = fmt.Sprintf("s%d", len(plan.Steps)+1)
		plan.Steps = append(plan.Steps, p.step)
		if p.step.Cost != nil {
			plan.Total.Images += p.step.Cost.Images
		}
	}
	if plan.Total.Images > 0 {
		plan.Total.Note = s.quotaNote(ctx)
		if s.d.ImagesLeft != nil {
			if left, err := s.d.ImagesLeft(ctx); err == nil && left >= 0 {
				plan.ImagesLeftToday = &left
			}
		}
	}
	s.mu.Lock()
	s.pruneLocked()
	s.plans[plan.ID] = &storedPlan{scope: scope, steps: plan.Steps, made: s.now(), started: map[string]bool{}}
	s.mu.Unlock()
	return Reply{Plan: plan}, nil
}

func stepLead(err error) string {
	return strings.TrimSpace(userMessage(err)) + " Có phải bạn muốn:"
}

var providerName = map[string]string{"grok": "Grok", "chatgpt": "ChatGPT"}

// quotaNote ghi hạn mức nguồn dự phòng khi nhà cung cấp BÁO hạn mức; không
// báo thì để trống (không bịa con số).
func (s *Service) quotaNote(ctx context.Context) string {
	if s.d.Queue == nil {
		return ""
	}
	view, err := s.d.Queue.List(ctx)
	if err != nil {
		return ""
	}
	for _, q := range view.ImageQuota {
		if q.Role != "fallback" || q.Limit == nil {
			continue
		}
		name := providerName[strings.ToLower(q.Provider)]
		if name == "" {
			name = q.Provider
		}
		period := "hôm nay"
		if q.Period == "week" {
			period = "tuần này"
		}
		left := *q.Limit - q.Used
		if left < 0 {
			left = 0
		}
		return fmt.Sprintf("%s (dự phòng) còn %d / %d %s", name, left, *q.Limit, period)
	}
	return ""
}

func (s *Service) pruneLocked() {
	now := s.now()
	for id, p := range s.plans {
		if now.Sub(p.made) > planTTL {
			delete(s.plans, id)
		}
	}
	for len(s.plans) >= maxPlans {
		var oldest string
		var at time.Time
		for id, p := range s.plans {
			if oldest == "" || p.made.Before(at) {
				oldest, at = id, p.made
			}
		}
		delete(s.plans, oldest)
	}
	for id, r := range s.runs {
		if r.finished && now.Sub(r.updated) > runTTL {
			delete(s.runs, id)
		}
	}
}

// ── parse / run (một intent) ────────────────────────────────────────────────

// Parsed là kết quả `parse` của §9: một intent + thẻ xác nhận.
type Parsed struct {
	Intent  IntentID
	Params  []byte
	Confirm Confirm
	Reply   *Reply // khác nil khi không thành một intent duy nhất
}

// Confirm là thẻ “Vẽ lại cảnh 5 · 1 ảnh”.
type Confirm struct {
	Title  string
	Detail string
	Cost   Cost
	Reruns []string
}

// Parse là Plan giới hạn một bước: câu có nhiều việc thì không đoán — trả
// thông báo để người dùng nói từng việc.
func (s *Service) Parse(ctx context.Context, scope Scope, text string) (Parsed, error) {
	reply, err := s.Plan(ctx, scope, text)
	if err != nil {
		return Parsed{}, err
	}
	if reply.Plan == nil {
		return Parsed{Reply: &reply}, nil
	}
	if len(reply.Plan.Steps) != 1 {
		d, err := s.loadScope(ctx, scope)
		if err != nil {
			return Parsed{}, err
		}
		u := s.unknown(d, unsupportedOther)
		u.Unknown.Lead = "Câu này có nhiều việc. Hãy nói từng việc một, hoặc dùng kế hoạch nhiều bước. Có phải bạn muốn:"
		return Parsed{Reply: u}, nil
	}
	st := reply.Plan.Steps[0]
	c := Confirm{Title: st.Title, Detail: st.Detail}
	if st.Cost != nil {
		c.Cost = *st.Cost
	}
	for _, r := range st.Reruns {
		c.Reruns = append(c.Reruns, r.Agent)
	}
	if c.Reruns == nil {
		c.Reruns = []string{}
	}
	return Parsed{Intent: st.Intent, Params: st.Params, Confirm: c}, nil
}

// Run chạy MỘT intent do người dùng đã xác nhận. params được giải mã nghiêm
// ngặt và kiểm lại với dữ liệu hiện tại trước khi làm.
func (s *Service) Run(ctx context.Context, intent IntentID, params []byte) (string, error) {
	p, err := s.prepare(ctx, intent, params)
	if err != nil {
		return "", err
	}
	msg, err := p.run(ctx)
	if err != nil {
		s.diag(err)
	}
	return msg, err
}

// ── run-plan ────────────────────────────────────────────────────────────────

// StepState là trạng thái một bước của lượt chạy.
type StepState string

const (
	StepQueued  StepState = "queued"
	StepRunning StepState = "running"
	StepDone    StepState = "done"
	StepFailed  StepState = "failed"
)

// RunStep là một bước của lượt chạy.
type RunStep struct {
	ID      string    `json:"id"`
	State   StepState `json:"state"`
	Message string    `json:"message,omitempty"`
}

// Run (trạng thái) là tiến độ của một lượt run-plan.
type RunStatus struct {
	ID    string    `json:"runId"`
	State string    `json:"state"`
	Steps []RunStep `json:"steps"`
}

type runState struct {
	id       string
	steps    []RunStep
	finished bool
	failed   bool
	updated  time.Time
}

func (r *runState) status() RunStatus {
	state := "running"
	if r.finished {
		state = "done"
		if r.failed {
			state = "failed"
		}
	}
	return RunStatus{ID: r.id, State: state, Steps: append([]RunStep(nil), r.steps...)}
}

// RunPlan chạy các bước người dùng giữ tích của kế hoạch planID, lần lượt, nền.
// Mỗi bước dùng đúng một lần.
func (s *Service) RunPlan(ctx context.Context, planID string, stepIDs []string) (RunStatus, error) {
	s.mu.Lock()
	s.pruneLocked()
	plan, ok := s.plans[planID]
	if !ok {
		s.mu.Unlock()
		return RunStatus{}, fail(CodePlanExpired, "Kế hoạch đã hết hạn. Nhắn lại để trợ lý lập kế hoạch mới.")
	}
	if len(stepIDs) == 0 {
		s.mu.Unlock()
		return RunStatus{}, fail(CodeInvalid, "Chọn ít nhất một bước để làm.")
	}
	byID := map[string]Step{}
	for _, st := range plan.steps {
		byID[st.ID] = st
	}
	seen := map[string]bool{}
	chosen := make([]Step, 0, len(stepIDs))
	for _, id := range stepIDs {
		st, ok := byID[id]
		if !ok || seen[id] {
			s.mu.Unlock()
			return RunStatus{}, fail(CodeInvalid, "Bước %q không thuộc kế hoạch này.", id)
		}
		if plan.started[id] {
			s.mu.Unlock()
			return RunStatus{}, fail(CodeConflict, "Bước này đã được làm rồi.")
		}
		seen[id] = true
		chosen = append(chosen, st)
	}
	// Giữ thứ tự của kế hoạch, không theo thứ tự client gửi.
	ordered := make([]Step, 0, len(chosen))
	for _, st := range plan.steps {
		if seen[st.ID] {
			ordered = append(ordered, st)
			plan.started[st.ID] = true
		}
	}
	run := &runState{id: newID("run_"), updated: s.now()}
	for _, st := range ordered {
		run.steps = append(run.steps, RunStep{ID: st.ID, State: StepQueued})
	}
	s.runs[run.id] = run
	snapshot := run.status()
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.execute(run, ordered)
	}()
	return snapshot, nil
}

// RunStatus trả tiến độ một lượt chạy.
func (s *Service) RunStatus(runID string) (RunStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[runID]
	if !ok {
		return RunStatus{}, false
	}
	return r.status(), true
}

func (s *Service) setStep(r *runState, i int, state StepState, msg string) {
	s.mu.Lock()
	r.steps[i].State, r.steps[i].Message = state, msg
	r.updated = s.now()
	s.mu.Unlock()
}

// execute chạy tuần tự các bước đã chọn; một bước lỗi không chặn các bước độc
// lập khác nhưng lượt chạy tính là failed. Tiến độ hiện ở Việc nền.
func (s *Service) execute(r *runState, steps []Step) {
	ctx := s.ctx
	title := fmt.Sprintf("Trợ lý: Làm %d bước", len(steps))
	taskID := ""
	if s.d.Tasks != nil {
		id, err := s.d.Tasks.Start(ctx, domain.TaskSpec{
			Kind: domain.TaskAssistant, Agent: agentDirector, Title: title,
			Object:   domain.ObjectRef{Type: taskObjectTyp, ID: r.id, Label: "Trợ lý"},
			Phase:    "Đang bắt đầu",
			Progress: &domain.TaskProgress{Done: 0, Total: int64(len(steps))},
		})
		if err != nil {
			s.diag(err)
		} else {
			taskID = id
		}
	}
	update := func(phase string, done int, state domain.TaskState) {
		if taskID == "" {
			return
		}
		u := domain.TaskUpdate{Phase: &phase, Progress: &domain.TaskProgress{Done: int64(done), Total: int64(len(steps))}, State: state}
		if err := s.d.Tasks.Update(context.WithoutCancel(ctx), taskID, u); err != nil {
			s.diag(err)
		}
	}
	done, failedMsg := 0, ""
	for i, st := range steps {
		s.setStep(r, i, StepRunning, "")
		update("Đang làm: "+st.Title, done, domain.TaskRunning)
		msg, err := s.runOne(ctx, st)
		if err != nil {
			s.setStep(r, i, StepFailed, userMessage(err))
			if failedMsg == "" {
				failedMsg = st.Title + ": " + userMessage(err)
			}
		} else {
			s.setStep(r, i, StepDone, msg)
			done++
		}
		if ctx.Err() != nil {
			break
		}
	}
	s.mu.Lock()
	r.finished = true
	r.failed = failedMsg != ""
	r.updated = s.now()
	s.mu.Unlock()
	if taskID == "" {
		return
	}
	end := context.WithoutCancel(ctx)
	if failedMsg != "" {
		update(failedMsg, done, domain.TaskFailed)
	}
	if err := s.d.Tasks.Finish(end, taskID); err != nil {
		s.diag(err)
	}
}

// runOne kiểm lại bước với dữ liệu hiện tại rồi làm thật.
func (s *Service) runOne(ctx context.Context, st Step) (string, error) {
	p, err := s.prepare(ctx, st.Intent, st.Params)
	if err != nil {
		return "", err
	}
	msg, err := p.run(ctx)
	if err != nil {
		s.diag(err)
	}
	return msg, err
}
