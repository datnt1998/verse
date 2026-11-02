package covertrial

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// Service vẽ thử ảnh thật (một ảnh/lần) cho template hoặc prompt. Việc chạy
// nền bằng context riêng nên đóng trang vẫn xong; huỷ/thử lại chỉ khi người
// dùng bấm. Nguồn chỉ được đọc — không sửa job, project hay manifest.
type Service struct {
	opts Options

	mu      sync.Mutex
	flights map[string]*flight
	wg      sync.WaitGroup
	closed  bool

	// startMu nối phép kiểm "đang có lần vẽ khác" với phép ghi hàng mới để hai
	// cú bấm liên tiếp không tạo hai lời gọi trả phí.
	startMu sync.Mutex
}

// flight là MỘT lần vẽ đang chạy. fence khoá lúc chốt trạng thái cuối: huỷ đã
// nhận thì không bao giờ ghi ready, và kết quả đã chốt thì không bị huỷ đè.
// done đóng sau khi hàng lịch sử đã chốt — người xoá template/prompt chờ nó
// trước khi nhặt media_path.
type flight struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	mu        sync.Mutex
	cancelled bool
	settled   bool
}

// Drawer là đúng *imagegen.Service ở chỗ ráp; test thay bằng bản giả.
type Drawer interface {
	Draw(ctx context.Context, req imagegen.Request) (imagegen.Receipt, error)
}

// DrawerResolution là provider hiện tại của đường vẽ + trần ảnh tham chiếu.
type DrawerResolution struct {
	Drawer   Drawer
	Provider string
	Model    string
	Label    string
	RefLimit int
}

// Tasks là phần activity.Service cần dùng.
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
	FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*activity.Task, error)
}

// Media là phần media.Service cần dùng (acquire để đọc, publish/unbind để giữ
// ảnh thử bền và nhả media cũ an toàn với GC).
type Media interface {
	Acquire(ctx context.Context, path string) (*media.Handle, error)
	Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error
	Unbind(ctx context.Context, paths []string, commit func(context.Context) error) error
}

// Options là phụ thuộc của Service.
type Options struct {
	Templates interface {
		Get(ctx context.Context, id string) (*library.Detail, error)
	}
	Prompts interface {
		GetPrompt(ctx context.Context, id string) (*domain.ImagePrompt, error)
	}
	Jobs interface {
		Get(ctx context.Context, id string) (*domain.CloneJobView, error)
	}
	// Sources liệt kê video đã xong (dùng chung bộ chọn của bản xem thử).
	Sources func(ctx context.Context) ([]library.PreviewSource, error)
	Store   library.TrialStore
	Media   Media
	Tasks   Tasks
	// ResolveDrawer trả provider đang cấu hình; lỗi là *imagegen.DrawError đã
	// sanitize (tool_missing/quota).
	ResolveDrawer func(ctx context.Context) (DrawerResolution, error)
	// Quota trả hạn mức ảnh hiện tại của provider đứng đầu chuỗi vẽ.
	Quota func(ctx context.Context) (library.TrialQuota, error)
	// Slot là pacer dùng chung với bước ảnh của pipeline.
	Slot  func(ctx context.Context, run func(context.Context) error) error
	Now   func() time.Time
	NewID func() string
}

// New dựng Service.
func New(opts Options) *Service {
	s := &Service{opts: opts, flights: map[string]*flight{}}
	if s.opts.Now == nil {
		s.opts.Now = time.Now
	}
	if s.opts.NewID == nil {
		s.opts.NewID = newUUID
	}
	return s
}

// Recover đánh dấu các lần vẽ dở dang của tiến trình trước; không tự vẽ lại.
func (s *Service) Recover(ctx context.Context) error {
	now := s.opts.Now()
	msg := "Lần vẽ thử bị gián đoạn khi ứng dụng khởi động lại"
	recovery := "Bấm Thử lại để vẽ một ảnh mới"
	_, coverErr := s.opts.Store.FailRunningCoverTrials(ctx, now, msg, recovery)
	_, sampleErr := s.opts.Store.FailRunningPromptSamples(ctx, now, msg, recovery)
	if coverErr != nil {
		return coverErr
	}
	return sampleErr
}

// Close huỷ mọi lần vẽ đang chạy và chờ chúng dừng (có trần theo ctx).
func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	cancels := make([]context.CancelFunc, 0, len(s.flights))
	for _, f := range s.flights {
		cancels = append(cancels, f.cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) now() time.Time { return s.opts.Now() }

func (s *Service) newID() string { return s.opts.NewID() }

// beginFlight đăng ký một lần vẽ và đếm nó vào WaitGroup TRƯỚC khi trả về, để
// Close không thể chờ hụt một lần vẽ vừa đăng ký.
func (s *Service) beginFlight(key string, cancel context.CancelFunc) *flight {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	f := &flight{cancel: cancel, done: make(chan struct{})}
	s.flights[key] = f
	s.wg.Add(1)
	return f
}

// endFlight gỡ flight, đánh thức người đang chờ hàng chốt và trả lượt chờ.
// Gọi đúng một lần cho mỗi beginFlight.
func (s *Service) endFlight(key string) {
	s.mu.Lock()
	f := s.flights[key]
	delete(s.flights, key)
	s.mu.Unlock()
	if f == nil {
		return
	}
	f.once.Do(func() {
		close(f.done)
		s.wg.Done()
	})
}

func (s *Service) flightFor(key string) *flight {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flights[key]
}

func (s *Service) inFlight(key string) bool { return s.flightFor(key) != nil }

// cancelFlights huỷ mọi lần vẽ có khoá bắt đầu bằng prefix và trả chúng để
// người gọi chờ hàng lịch sử chốt xong.
func (s *Service) cancelFlights(prefix string) []*flight {
	s.mu.Lock()
	out := make([]*flight, 0)
	for key, f := range s.flights {
		if strings.HasPrefix(key, prefix) {
			out = append(out, f)
		}
	}
	s.mu.Unlock()
	for _, f := range out {
		f.cancel()
	}
	return out
}

// waitFlights chờ các lần vẽ chốt xong, tôn trọng ctx và trần waitFlightsTimeout.
func waitFlights(ctx context.Context, flights []*flight) error {
	if len(flights) == 0 {
		return nil
	}
	timer := time.NewTimer(waitFlightsTimeout)
	defer timer.Stop()
	for _, f := range flights {
		select {
		case <-f.done:
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return context.DeadlineExceeded
		}
	}
	return nil
}

// acceptCancel nhận huỷ dưới fence; false khi lần vẽ đã chốt trạng thái cuối
// (huỷ lúc này là quá muộn, không được đè kết quả).
func (f *flight) acceptCancel() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.settled {
		return false
	}
	f.cancelled = true
	f.cancel()
	return true
}

func (s *Service) withSlot(ctx context.Context, run func(context.Context) error) error {
	if s.opts.Slot == nil {
		return run(ctx)
	}
	return s.opts.Slot(ctx, run)
}

// ── template cover trial ───────────────────────────────────────────────────

// Status trả nguồn, 12 lần gần nhất, hạn mức và lý do chặn thật.
func (s *Service) Status(ctx context.Context, templateID string) (*library.CoverTrialStatus, error) {
	detail, err := s.opts.Templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	history := s.coverHistory(ctx, templateID)
	sources, sourcesErr := s.sources(ctx)
	prompt, promptErr := s.coverPrompt(ctx, detail)
	quota, quotaErr := s.quota(ctx)
	res, resErr := s.resolveDrawer(ctx)
	out := &library.CoverTrialStatus{
		Sources: sources,
		History: history,
		Task:    s.task(ctx, domain.ObjectRef{Type: "template", ID: templateID}),
		Quota:   quota,
		Cost:    trialCost(res, quota),
		Prompt:  promptRef(prompt),
	}
	out.Blocked, out.Ready = coverBlocked(sourcesErr, len(sources), promptErr, resErr, quotaErr, quota)
	return out, nil
}

// Start xếp một lần vẽ thử ảnh bìa. Confirm phải true để UI đã nói đúng giá
// một ảnh; máy chủ không vẽ khi chưa xác nhận.
func (s *Service) Start(ctx context.Context, templateID string, req library.CoverTrialRequest) (*library.CoverTrialStatus, error) {
	if !req.Confirm {
		return nil, &library.Error{
			Code:     library.CodeConfirmRequired,
			Message:  "Cần xác nhận dùng 1 ảnh trước khi vẽ thử",
			Recovery: "Bấm Vẽ thử rồi xác nhận trong hộp thoại",
		}
	}
	if strings.TrimSpace(req.JobID) == "" {
		return nil, invalid("Chưa chọn video nguồn", "Chọn một video đã làm xong rồi vẽ thử")
	}
	detail, err := s.opts.Templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if _, err := s.resolveDrawer(ctx); err != nil {
		return nil, err
	}
	quota, err := s.quota(ctx)
	if err != nil {
		return nil, conflict("Chưa đọc được hạn mức ảnh hôm nay", "Thử lại sau ít phút")
	}
	if quota.Paid && quota.Left <= 0 {
		return nil, quotaError("Đã hết hạn mức ảnh hôm nay", refillRecovery(quota.ResetAt), quota.ResetAt)
	}
	src, err := s.loadSource(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	prompt, err := s.coverPrompt(ctx, detail)
	if err != nil {
		return nil, err
	}
	res, err := s.resolveDrawer(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.validateDrawable(src, "cover", prompt, "", res.Provider); err != nil {
		return nil, err
	}
	// Từ đây tới khi ghi hàng là một khối: hai cú bấm liên tiếp phải thấy nhau
	// thì mới không có hai lời gọi trả phí cho cùng một template.
	s.startMu.Lock()
	history := s.coverHistory(ctx, templateID)
	for _, trial := range history {
		if trial.State == library.TrialRunning && s.inFlight(flightKey("t", templateID, trial.ID)) {
			s.startMu.Unlock()
			return nil, conflict("Đang vẽ một ảnh thử khác", "Chờ lần vẽ hiện tại xong hoặc huỷ nó trước")
		}
	}
	row := library.CoverTrial{
		ID:          s.newID(),
		TemplateID:  templateID,
		JobID:       src.job.ID,
		JobTitle:    src.job.Source.Title,
		PromptID:    prompt.ID,
		PromptName:  prompt.Name,
		Composition: coverComposition(prompt),
		State:       library.TrialRunning,
		Provider:    res.Provider,
		Model:       res.Model,
		HookText:    src.story.ThumbnailText,
		CreatedAt:   s.now(),
	}
	if row.ID == "" {
		s.startMu.Unlock()
		return nil, trialUnavailable("Chưa sẵn sàng vẽ thử")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	key := flightKey("t", templateID, row.ID)
	f := s.beginFlight(key, cancel)
	if f == nil {
		s.startMu.Unlock()
		cancel()
		return nil, trialUnavailable("Ứng dụng đang tắt")
	}
	taskID := s.startCoverTask(ctx, templateID, detail.Name)
	if err := s.opts.Store.InsertCoverTrial(ctx, row); err != nil {
		s.endFlight(key)
		s.startMu.Unlock()
		cancel()
		s.finishTask(ctx, taskID)
		return nil, err
	}
	s.startMu.Unlock()
	go s.runCover(runCtx, f, templateID, row, taskID, res)
	return s.Status(ctx, templateID)
}

// Cancel huỷ lần vẽ đang chạy của template.
func (s *Service) Cancel(ctx context.Context, templateID, trialID string) error {
	row, err := s.opts.Store.GetCoverTrial(ctx, templateID, trialID)
	if err != nil {
		return coverStoreError(err)
	}
	if row.State != library.TrialRunning {
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	key := flightKey("t", templateID, trialID)
	if f := s.flightFor(key); f != nil {
		// Huỷ dưới fence: nếu lần vẽ đã chốt ready thì đây là huỷ quá muộn —
		// trả 409 thay vì đè lên kết quả người dùng vừa có.
		if f.acceptCancel() {
			return nil
		}
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	// Hàng mồ côi (không có goroutine trong tiến trình này): đọc lại rồi đóng
	// sổ, để không đè lên kết quả mà một lần vẽ vừa chốt trước khi gỡ flight.
	if row, err = s.opts.Store.GetCoverTrial(ctx, templateID, trialID); err != nil {
		return coverStoreError(err)
	}
	if row.State != library.TrialRunning {
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	now := s.now()
	row.State = library.TrialCancelled
	row.CostImages = 0
	row.FinishedAt = &now
	row.Error = nil
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	if err := s.opts.Store.UpdateCoverTrial(writeCtx, *row); err != nil {
		return coverStoreError(err)
	}
	return nil
}

// ImagePath trả media của một lần vẽ đã xong.
func (s *Service) ImagePath(ctx context.Context, templateID, trialID string) (string, error) {
	row, err := s.opts.Store.GetCoverTrial(ctx, templateID, trialID)
	if err != nil {
		return "", coverStoreError(err)
	}
	if row.State != library.TrialReady || row.MediaPath == "" {
		return "", conflict("Ảnh thử chưa sẵn sàng", "Chờ lần vẽ xong rồi mở lại")
	}
	return row.MediaPath, nil
}

// CancelTemplate huỷ mọi lần vẽ của template và chờ hàng lịch sử chốt xong
// (trước khi xoá template) để mọi media_path vừa publish đều còn dấu mà nhả.
func (s *Service) CancelTemplate(ctx context.Context, templateID string) {
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), waitFlightsTimeout)
	defer cancel()
	_ = waitFlights(waitCtx, s.cancelFlights("t:"+templateID+":"))
}

func (s *Service) coverHistory(ctx context.Context, templateID string) []library.CoverTrial {
	history, err := s.opts.Store.ListCoverTrials(ctx, templateID, 12)
	if err != nil {
		return []library.CoverTrial{}
	}
	if history == nil {
		history = []library.CoverTrial{}
	}
	now := s.now()
	for i := range history {
		row := &history[i]
		if row.State != library.TrialRunning || s.inFlight(flightKey("t", templateID, row.ID)) {
			continue
		}
		row.State = library.TrialFailed
		row.CostImages = 0
		row.FinishedAt = &now
		row.Error = &library.TrialError{
			Code:     "interrupted",
			Message:  "Lần vẽ thử bị gián đoạn",
			Recovery: "Bấm Thử lại để vẽ một ảnh mới",
		}
		if err := s.opts.Store.UpdateCoverTrial(ctx, *row); err != nil {
			// Đọc vẫn trả trạng thái đã sửa; lần đọc sau sẽ thử lại.
			continue
		}
	}
	// Hàng running mồ côi nghĩa là tiến trình trước đã chết: việc nền đã lưu
	// cũng mồ côi, đóng nó để Bảng điều khiển không treo "đang vẽ" mãi.
	s.finishOrphanTask(ctx, domain.ObjectRef{Type: "template", ID: templateID}, "t:"+templateID+":")
	return history
}

func (s *Service) coverPrompt(ctx context.Context, detail *library.Detail) (*domain.ImagePrompt, error) {
	id := strings.TrimSpace(detail.Recipe.Scenes.CoverPromptID)
	if id == "" {
		id = library.BuiltinCoverClean
	}
	return s.opts.Prompts.GetPrompt(ctx, id)
}

// ── prompt sample trial ────────────────────────────────────────────────────

// PromptStatus trả nguồn, ảnh mẫu và hạn mức của một prompt.
func (s *Service) PromptStatus(ctx context.Context, promptID string) (*library.PromptSampleStatus, error) {
	prompt, err := s.opts.Prompts.GetPrompt(ctx, promptID)
	if err != nil {
		return nil, promptError(err)
	}
	history := s.sampleHistory(ctx, promptID)
	sources, sourcesErr := s.sources(ctx)
	quota, quotaErr := s.quota(ctx)
	res, resErr := s.resolveDrawer(ctx)
	out := &library.PromptSampleStatus{
		Sources: sources,
		History: history,
		Task:    s.task(ctx, domain.ObjectRef{Type: "image_prompt", ID: promptID}),
		Quota:   quota,
		Cost:    trialCost(res, quota),
		Prompt:  promptRef(prompt),
	}
	out.Blocked, out.Ready = sampleBlocked(sourcesErr, len(sources), resErr, quotaErr, quota)
	return out, nil
}

// StartPromptSample xếp một lần vẽ ảnh mẫu cho prompt.
func (s *Service) StartPromptSample(ctx context.Context, promptID string, req library.PromptSampleRequest) (*library.PromptSampleStatus, error) {
	if !req.Confirm {
		return nil, &library.Error{
			Code:     library.CodeConfirmRequired,
			Message:  "Cần xác nhận dùng 1 ảnh trước khi vẽ thử",
			Recovery: "Bấm Vẽ thử rồi xác nhận trong hộp thoại",
		}
	}
	if strings.TrimSpace(req.VideoID) == "" {
		return nil, invalid("Chưa chọn video nguồn", "Chọn một video đã làm xong rồi vẽ thử")
	}
	prompt, err := s.opts.Prompts.GetPrompt(ctx, promptID)
	if err != nil {
		return nil, promptError(err)
	}
	res, err := s.resolveDrawer(ctx)
	if err != nil {
		return nil, err
	}
	quota, err := s.quota(ctx)
	if err != nil {
		return nil, conflict("Chưa đọc được hạn mức ảnh hôm nay", "Thử lại sau ít phút")
	}
	if quota.Paid && quota.Left <= 0 {
		return nil, quotaError("Đã hết hạn mức ảnh hôm nay", refillRecovery(quota.ResetAt), quota.ResetAt)
	}
	src, err := s.loadSource(ctx, req.VideoID)
	if err != nil {
		return nil, err
	}
	kind := string(prompt.Kind)
	if kind != "scene" && kind != "cover" {
		return nil, conflict("Chỉ dẫn chưa rõ loại ảnh", "Lưu chỉ dẫn rồi thử lại")
	}
	sceneID := ""
	if kind == "scene" {
		sceneID = req.SceneID
	}
	if err := s.validateDrawable(src, kind, prompt, sceneID, res.Provider); err != nil {
		return nil, err
	}
	// Khối kiểm-đang-vẽ + ghi hàng: hai cú bấm liên tiếp không tạo hai lời gọi
	// trả phí cho cùng một prompt.
	s.startMu.Lock()
	history := s.sampleHistory(ctx, promptID)
	for _, sample := range history {
		if sample.State == library.TrialRunning && s.inFlight(flightKey("s", promptID, sample.ID)) {
			s.startMu.Unlock()
			return nil, conflict("Đang vẽ một ảnh mẫu khác", "Chờ lần vẽ hiện tại xong hoặc huỷ nó trước")
		}
	}
	composition := ""
	if kind == "cover" {
		composition = coverComposition(prompt)
	}
	row := library.PromptSample{
		ID:          s.newID(),
		PromptID:    prompt.ID,
		PromptName:  prompt.Name,
		PromptKind:  kind,
		Composition: composition,
		VideoID:     src.job.ID,
		VideoTitle:  src.job.Source.Title,
		SceneID:     sceneID,
		HookText:    hookTextFor(src.story, kind),
		State:       library.TrialRunning,
		Provider:    res.Provider,
		Model:       res.Model,
		CreatedAt:   s.now(),
	}
	if row.ID == "" {
		s.startMu.Unlock()
		return nil, trialUnavailable("Chưa sẵn sàng vẽ thử")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	key := flightKey("s", promptID, row.ID)
	f := s.beginFlight(key, cancel)
	if f == nil {
		s.startMu.Unlock()
		cancel()
		return nil, trialUnavailable("Ứng dụng đang tắt")
	}
	taskID := s.startSampleTask(ctx, promptID, prompt.Name)
	if err := s.opts.Store.InsertPromptSample(ctx, row); err != nil {
		s.endFlight(key)
		s.startMu.Unlock()
		cancel()
		s.finishTask(ctx, taskID)
		return nil, err
	}
	s.startMu.Unlock()
	go s.runPromptSample(runCtx, f, promptID, row, taskID, res)
	return s.PromptStatus(ctx, promptID)
}

// CancelPromptSample huỷ lần vẽ ảnh mẫu đang chạy.
func (s *Service) CancelPromptSample(ctx context.Context, promptID, sampleID string) error {
	row, err := s.opts.Store.GetPromptSample(ctx, promptID, sampleID)
	if err != nil {
		return sampleStoreError(err)
	}
	if row.State != library.TrialRunning {
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	key := flightKey("s", promptID, sampleID)
	if f := s.flightFor(key); f != nil {
		if f.acceptCancel() {
			return nil
		}
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	// Hàng mồ côi: đọc lại để không đè kết quả vừa chốt trước khi gỡ flight.
	if row, err = s.opts.Store.GetPromptSample(ctx, promptID, sampleID); err != nil {
		return sampleStoreError(err)
	}
	if row.State != library.TrialRunning {
		return conflict("Lần vẽ này đã xong", "Tải lại trạng thái để xem kết quả")
	}
	now := s.now()
	row.State = library.TrialCancelled
	row.CostImages = 0
	row.FinishedAt = &now
	row.Error = nil
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	if err := s.opts.Store.UpdatePromptSample(writeCtx, *row); err != nil {
		return sampleStoreError(err)
	}
	return nil
}

// PromptSamplePath trả media của một ảnh mẫu đã xong.
func (s *Service) PromptSamplePath(ctx context.Context, promptID, sampleID string) (string, error) {
	row, err := s.opts.Store.GetPromptSample(ctx, promptID, sampleID)
	if err != nil {
		return "", sampleStoreError(err)
	}
	if row.State != library.TrialReady || row.MediaPath == "" {
		return "", conflict("Ảnh mẫu chưa sẵn sàng", "Chờ lần vẽ xong rồi mở lại")
	}
	return row.MediaPath, nil
}

// CancelPrompt huỷ mọi lần vẽ ảnh mẫu của prompt và chờ hàng lịch sử chốt
// xong (trước khi xoá prompt) để mọi media_path vừa publish đều còn dấu mà nhả.
func (s *Service) CancelPrompt(ctx context.Context, promptID string) {
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), waitFlightsTimeout)
	defer cancel()
	_ = waitFlights(waitCtx, s.cancelFlights("s:"+promptID+":"))
}

// Scenes liệt kê cảnh thật của video nguồn để chọn khi vẽ thử prompt cảnh.
func (s *Service) Scenes(ctx context.Context, promptID, videoID string) ([]library.TrialScene, error) {
	if _, err := s.opts.Prompts.GetPrompt(ctx, promptID); err != nil {
		return nil, promptError(err)
	}
	if strings.TrimSpace(videoID) == "" {
		return nil, invalid("Chưa chọn video nguồn", "Chọn một video đã làm xong để xem cảnh")
	}
	src, err := s.loadSource(ctx, videoID)
	if err != nil {
		return nil, err
	}
	items := sceneItems(src.manifest)
	out := make([]library.TrialScene, 0, len(items))
	for index, item := range items {
		out = append(out, library.TrialScene{ID: item.ID, Label: sceneLabel(index, item.Narration)})
	}
	return out, nil
}

// SampleIDs trả media id ảnh mẫu mới nhất của từng prompt (wire Thư viện).
func (s *Service) SampleIDs(ctx context.Context, promptIDs []string) (map[string][]string, error) {
	ids := make([]string, 0, len(promptIDs))
	seen := map[string]bool{}
	for _, id := range promptIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return s.opts.Store.ListPromptSampleIDs(ctx, ids)
}

func (s *Service) sampleHistory(ctx context.Context, promptID string) []library.PromptSample {
	history, err := s.opts.Store.ListPromptSamples(ctx, promptID, 12)
	if err != nil {
		return []library.PromptSample{}
	}
	if history == nil {
		history = []library.PromptSample{}
	}
	now := s.now()
	for i := range history {
		row := &history[i]
		if row.State != library.TrialRunning || s.inFlight(flightKey("s", promptID, row.ID)) {
			continue
		}
		row.State = library.TrialFailed
		row.CostImages = 0
		row.FinishedAt = &now
		row.Error = &library.TrialError{
			Code:     "interrupted",
			Message:  "Lần vẽ thử bị gián đoạn",
			Recovery: "Bấm Thử lại để vẽ một ảnh mới",
		}
		if err := s.opts.Store.UpdatePromptSample(ctx, *row); err != nil {
			continue
		}
	}
	// Việc nền đã lưu của tiến trình trước cũng mồ côi theo hàng running.
	s.finishOrphanTask(ctx, domain.ObjectRef{Type: "image_prompt", ID: promptID}, "s:"+promptID+":")
	return history
}

// ── chia sẻ ────────────────────────────────────────────────────────────────

func (s *Service) sources(ctx context.Context) ([]library.PreviewSource, error) {
	if s.opts.Sources == nil {
		return []library.PreviewSource{}, nil
	}
	out, err := s.opts.Sources(ctx)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []library.PreviewSource{}
	}
	return out, nil
}

func (s *Service) quota(ctx context.Context) (*library.TrialQuota, error) {
	if s.opts.Quota == nil {
		return nil, errors.New("chưa gắn hạn mức ảnh")
	}
	q, err := s.opts.Quota(ctx)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Service) resolveDrawer(ctx context.Context) (DrawerResolution, error) {
	if s.opts.ResolveDrawer == nil {
		return DrawerResolution{}, trialUnavailable("Chưa sẵn sàng vẽ ảnh")
	}
	return s.opts.ResolveDrawer(ctx)
}

func (s *Service) task(ctx context.Context, object domain.ObjectRef) *library.PreviewTask {
	if s.opts.Tasks == nil {
		return nil
	}
	task, err := s.opts.Tasks.FindTaskByObject(ctx, domain.TaskImageTrial, object)
	if err != nil || task == nil {
		return nil
	}
	out := &library.PreviewTask{ID: task.ID, Phase: task.Phase, State: string(task.State), ActionHref: task.Action.Href}
	if task.Progress != nil {
		out.Done, out.Total = task.Progress.Done, task.Progress.Total
	}
	return out
}

func (s *Service) startCoverTask(ctx context.Context, templateID, name string) string {
	if s.opts.Tasks == nil {
		return ""
	}
	id, _ := s.opts.Tasks.Start(ctx, domain.TaskSpec{
		Kind:   domain.TaskImageTrial,
		Agent:  "artist",
		Title:  "Đang vẽ thử ảnh bìa",
		Object: domain.ObjectRef{Type: "template", ID: templateID, Label: name},
		Phase:  "Đang chuẩn bị",
		Progress: &domain.TaskProgress{
			Done: 0, Total: 1,
		},
		Action: domain.TaskAction{Label: "Mở mẫu video", Href: "#/library/templates/" + templateID},
	})
	return id
}

func (s *Service) startSampleTask(ctx context.Context, promptID, name string) string {
	if s.opts.Tasks == nil {
		return ""
	}
	id, _ := s.opts.Tasks.Start(ctx, domain.TaskSpec{
		Kind:   domain.TaskImageTrial,
		Agent:  "artist",
		Title:  "Đang vẽ ảnh mẫu",
		Object: domain.ObjectRef{Type: "image_prompt", ID: promptID, Label: name},
		Phase:  "Đang chuẩn bị",
		Progress: &domain.TaskProgress{
			Done: 0, Total: 1,
		},
	})
	return id
}

func (s *Service) phase(ctx context.Context, taskID, phase string) {
	if s.opts.Tasks == nil || taskID == "" {
		return
	}
	_ = s.opts.Tasks.Update(context.WithoutCancel(ctx), taskID, domain.TaskUpdate{Phase: &phase})
}

func (s *Service) finishTask(ctx context.Context, taskID string) {
	if s.opts.Tasks == nil || taskID == "" {
		return
	}
	_ = s.opts.Tasks.Finish(context.WithoutCancel(ctx), taskID)
}

// finishOrphanTask đóng việc nền đã lưu của một phạm vi khi không còn lần vẽ
// nào sống trong tiến trình này — hàng running mồ côi sau khi khởi động lại
// không được để Bảng điều khiển treo "đang vẽ" mãi.
func (s *Service) finishOrphanTask(ctx context.Context, object domain.ObjectRef, prefix string) {
	if s.opts.Tasks == nil {
		return
	}
	s.mu.Lock()
	live := false
	for key := range s.flights {
		if strings.HasPrefix(key, prefix) {
			live = true
			break
		}
	}
	s.mu.Unlock()
	if live {
		return
	}
	task := s.task(ctx, object)
	if task == nil {
		return
	}
	s.finishTask(ctx, task.ID)
}

func flightKey(scope, owner, id string) string { return scope + ":" + owner + ":" + id }

func promptRef(prompt *domain.ImagePrompt) *library.TrialPrompt {
	if prompt == nil {
		return nil
	}
	return &library.TrialPrompt{ID: prompt.ID, Name: prompt.Name, Composition: string(prompt.Composition)}
}

// trialCost báo đúng giá của MỘT lần vẽ: chỉ có ảnh khi đã có nguồn vẽ thật,
// provider là id thật (không phải nhãn hiển thị), và quotaLeft chỉ khi provider
// tính phí theo ảnh. Chưa cấu hình provider thì không hứa ảnh nào.
func trialCost(res DrawerResolution, quota *library.TrialQuota) library.TrialCost {
	out := library.TrialCost{Provider: res.Provider, Label: drawProvider(res), QuotaLeft: -1}
	if res.Drawer == nil || res.Provider == "" {
		return out
	}
	out.Images = 1
	if quota != nil && quota.Paid {
		out.QuotaLeft = quota.Left
	}
	return out
}

func coverBlocked(sourcesErr error, sourceCount int, promptErr, resErr, quotaErr error, quota *library.TrialQuota) (string, bool) {
	reason := ""
	switch {
	case sourcesErr != nil:
		reason = "Chưa đọc được danh sách video đã làm xong · Thử lại sau ít phút"
	case sourceCount == 0:
		reason = "Chưa có video nào làm xong để vẽ thử · Dựng xong một video rồi thử lại"
	case promptErr != nil:
		reason = blockedText(promptErr)
	case resErr != nil:
		reason = blockedText(resErr)
	case quotaErr != nil:
		reason = "Chưa đọc được hạn mức ảnh hôm nay · Thử lại sau ít phút"
	case quota != nil && quota.Paid && quota.Left <= 0:
		reason = "Đã hết hạn mức ảnh hôm nay · " + refillRecovery(quota.ResetAt)
	}
	return reason, reason == ""
}

func sampleBlocked(sourcesErr error, sourceCount int, resErr, quotaErr error, quota *library.TrialQuota) (string, bool) {
	reason := ""
	switch {
	case sourcesErr != nil:
		reason = "Chưa đọc được danh sách video đã làm xong · Thử lại sau ít phút"
	case sourceCount == 0:
		reason = "Chưa có video nào làm xong để vẽ thử · Dựng xong một video rồi thử lại"
	case resErr != nil:
		reason = blockedText(resErr)
	case quotaErr != nil:
		reason = "Chưa đọc được hạn mức ảnh hôm nay · Thử lại sau ít phút"
	case quota != nil && quota.Paid && quota.Left <= 0:
		reason = "Đã hết hạn mức ảnh hôm nay · " + refillRecovery(quota.ResetAt)
	}
	return reason, reason == ""
}

func blockedText(err error) string {
	var le *library.Error
	if errors.As(err, &le) {
		if le.Recovery != "" {
			return le.Message + " · " + le.Recovery
		}
		return le.Message
	}
	return "Chưa sẵn sàng vẽ thử · Thử lại sau ít phút"
}

func refillRecovery(resetAt *time.Time) string {
	if resetAt == nil {
		return "Chờ hạn mức đặt lại rồi thử lại"
	}
	return "Chờ đến " + resetAt.Local().Format("15:04 02/01") + " rồi thử lại"
}

func hookTextFor(story images.Story, kind string) string {
	if kind != "cover" {
		return ""
	}
	return story.ThumbnailText
}

func coverStoreError(err error) error {
	if errors.Is(err, library.ErrNotFound) {
		return notFoundCover()
	}
	return err
}

func sampleStoreError(err error) error {
	if errors.Is(err, library.ErrNotFound) {
		return notFoundSample()
	}
	return err
}

func notFoundCover() error {
	return &library.Error{Code: library.CodeNotFound, Message: "Lần vẽ thử không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
}

func notFoundSample() error {
	return &library.Error{Code: library.CodeNotFound, Message: "Ảnh mẫu không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
}

func promptError(err error) error {
	if errors.Is(err, library.ErrNotFound) {
		return &library.Error{Code: library.CodeNotFound, Message: "Chỉ dẫn không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
	}
	return err
}

func sceneLabel(index int, narration string) string {
	text := strings.TrimSpace(strings.Join(strings.Fields(narration), " "))
	runes := []rune(text)
	if len(runes) > 90 {
		text = string(runes[:87]) + "…"
	}
	if text == "" {
		return fmt.Sprintf("Cảnh %d", index+1)
	}
	return fmt.Sprintf("Cảnh %d · %s", index+1, text)
}
