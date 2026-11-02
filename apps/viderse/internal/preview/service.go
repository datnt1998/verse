// Package preview dựng bản xem thử template: một project editor là bản sao
// project của template, được đổ ảnh cảnh đã duyệt + âm thanh nguồn + phụ đề
// của một video clone đã xong (N phút đầu). Không vẽ ảnh mới, không gọi
// provider, không tạo job clone, không đụng project của video nguồn; engine
// đổ là editor.Fill — cùng engine với bước build.
package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/build"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const (
	defaultMinutes = 3
	minMinutes     = 1
	maxMinutes     = 10
	previewPrefix  = "preview"
)

// Options là phụ thuộc của Service.
type Options struct {
	Templates interface {
		Get(ctx context.Context, id string) (*library.Detail, error)
		BeginTemplatePreview(ctx context.Context, id, runID string) error
		RecordTemplatePreview(ctx context.Context, id string, record library.TemplatePreviewRecord) (string, error)
		FailTemplatePreview(ctx context.Context, id, runID string, failure *library.PreviewError) error
		ClearTemplatePreviewRun(ctx context.Context, id, runID string) error
		RegisterTemplatePreviewProject(ctx context.Context, id, projectID, jobID string, minutes int) error
		PreviewProjectIDsForTemplate(ctx context.Context, id string) ([]string, error)
		ForgetPreviewProject(ctx context.Context, projectID string) error
		PreviewProjectRows(ctx context.Context) ([]library.PreviewProjectRow, error)
	}
	Jobs interface {
		List(ctx context.Context, channelID *string, limit int) (domain.CloneJobList, error)
		Get(ctx context.Context, id string) (*domain.CloneJobView, error)
	}
	Editor interface {
		LoadProject(ctx context.Context, id string) (editor.Project, string, error)
		DuplicateProjectAs(ctx context.Context, id, name, newID string) (editor.Project, string, error)
		RunCommands(ctx context.Context, id string, commands []editor.Command, options editor.ApplyCommandsOptions) (editor.Object, error)
		DeleteProject(ctx context.Context, id string) error
		ImportFileOnce(ctx context.Context, opts editor.ImportFileOnceOptions) (editor.MediaRecord, bool, error)
	}
	Media  Media
	Runner tools.Runner
	Ffmpeg func(ctx context.Context) (string, bool)
	Tasks  Tasks
	Now    func() time.Time
}

// Media là phần media.Service mà bản xem thử cần (mở tệp theo logical path).
type Media interface {
	Acquire(ctx context.Context, path string) (*media.Handle, error)
}

// Tasks là phần activity.Service cần dùng.
type Tasks interface {
	Start(ctx context.Context, spec domain.TaskSpec) (string, error)
	Update(ctx context.Context, id string, u domain.TaskUpdate) error
	Finish(ctx context.Context, id string) error
	FindTaskByObject(ctx context.Context, kind domain.TaskKind, object domain.ObjectRef) (*activity.Task, error)
}

// SourceJobs là đường đọc nhẹ cho danh sách nguồn (pipeline.Service hiện
// thực). Fallback là Jobs.List khi store/test cũ không có nó.
type SourceJobs interface {
	FinishedPreviewSources(ctx context.Context) ([]pipeline.PreviewSourceJob, error)
}

// Service dựng/đọc bản xem thử.
type Service struct {
	opts Options

	mu           sync.Mutex
	runs         map[string]*run
	sourceCache  map[string][]library.PreviewSource
	lifetime     context.Context
	stopLifetime context.CancelFunc
	wg           sync.WaitGroup
	closed       bool
}

// run là một lượt dựng đang giữ token; chỉ lượt này được cập nhật/kết thúc
// dòng việc của chính nó và công bố project.
type run struct {
	id         string
	templateID string
	taskID     string
	cancel     context.CancelFunc
	done       chan struct{}
}

// New dựng Service. Lifetime context sống tới Close: mọi lượt dựng bị huỷ và
// được chờ khi service đóng, trước khi pool/media đóng.
func New(opts Options) *Service {
	lifetime, stop := context.WithCancel(context.Background())
	return &Service{
		opts: opts, runs: map[string]*run{}, sourceCache: map[string][]library.PreviewSource{},
		lifetime: lifetime, stopLifetime: stop,
	}
}

func (s *Service) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

func errPreview(code, message, recovery string) *library.Error {
	return &library.Error{Code: code, Message: message, Recovery: recovery}
}

// templateHref là route canonical của SPA (#/templates/:id) — không phải route
// thư viện cũ, để dòng việc nền mở đúng màn template.
func templateHref(templateID string) string {
	return "#/templates/" + url.PathEscape(templateID)
}

func (s *Service) objectRef(templateID string) domain.ObjectRef {
	return domain.ObjectRef{Type: "template", ID: templateID, Label: "Bản xem thử"}
}

func previewRevision(detail *library.Detail) int64 {
	if detail.PreviewRevision != nil {
		return *detail.PreviewRevision
	}
	return 0
}

func taskView(task *activity.Task) *library.PreviewTask {
	if task == nil {
		return nil
	}
	done, total := int64(0), int64(0)
	if task.Progress != nil {
		done, total = task.Progress.Done, task.Progress.Total
	}
	return &library.PreviewTask{ID: task.ID, Phase: task.Phase, State: task.State, Done: done, Total: total, ActionHref: templateHref(task.Object.ID)}
}

func baseStatus(detail *library.Detail) *library.PreviewStatus {
	return &library.PreviewStatus{
		State: detail.PreviewState(), BuiltAt: detail.PreviewBuiltAt,
		ProjectID: detail.PreviewProjectID, JobID: detail.PreviewJobID,
		Minutes: detail.PreviewMinutes, TemplateRevision: previewRevision(detail),
		Error: detail.PreviewError, Sources: []library.PreviewSource{},
	}
}

func (s *Service) task(ctx context.Context, templateID string) (*library.PreviewTask, error) {
	if s.opts.Tasks == nil {
		return nil, nil
	}
	task, err := s.opts.Tasks.FindTaskByObject(ctx, domain.TaskTemplatePreview, s.objectRef(templateID))
	if err != nil {
		return nil, err
	}
	return taskView(task), nil
}

// Status trả trạng thái bản xem thử. Khi đang dựng, danh sách nguồn lấy từ
// lần đọc gần nhất thay vì quét lại job mỗi 1,5 giây: người dùng không chọn
// nguồn trong lúc dựng, còn lỗi thật của nguồn vẫn hiện ở lần dựng.
func (s *Service) Status(ctx context.Context, templateID string) (*library.PreviewStatus, error) {
	detail, err := s.opts.Templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	status := baseStatus(detail)
	task, err := s.task(ctx, templateID)
	if err != nil {
		return nil, err
	}
	status.Task = task
	building := task != nil && task.State == domain.TaskRunning
	if building {
		status.State = library.PreviewBuilding
	}
	sources, err := s.sourcesFor(ctx, detail, building)
	if err != nil {
		return nil, err
	}
	status.Sources = sources
	return status, nil
}

// Sources liệt kê video đã xong, còn hiệu lực, có đủ artifact để xem thử.
// Không phụ thuộc recipe: dùng cho màn khác (vẽ thử bìa/prompt).
func (s *Service) Sources(ctx context.Context) ([]library.PreviewSource, error) {
	return s.sources(ctx, nil)
}

// sourcesFor lọc nguồn theo recipe của template. Khi đang dựng, dùng bản đã
// đọc gần nhất thay vì quét lại.
func (s *Service) sourcesFor(ctx context.Context, detail *library.Detail, building bool) ([]library.PreviewSource, error) {
	s.mu.Lock()
	cached, ok := s.sourceCache[detail.ID]
	s.mu.Unlock()
	if building && ok {
		return append([]library.PreviewSource{}, cached...), nil
	}
	sources, err := s.sources(ctx, &detail.Recipe)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sourceCache[detail.ID] = append([]library.PreviewSource{}, sources...)
	s.mu.Unlock()
	return sources, nil
}

func (s *Service) sources(ctx context.Context, recipe *domain.Recipe) ([]library.PreviewSource, error) {
	jobs, err := s.sourceJobs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]library.PreviewSource, 0, len(jobs))
	for _, job := range jobs {
		source := library.PreviewSource{JobID: job.ID, Title: job.Title, ChannelName: job.ChannelName, DurationSec: job.DurationSec}
		if recipe != nil && recipe.Scenes.Images.Mode != library.ImagesNone && job.NoImages {
			source.Disabled = true
			source.Reason = "Video nguồn đã chọn “Không vẽ ảnh” nên không có ảnh cảnh cho template này"
		}
		out = append(out, source)
	}
	return out, nil
}

func (s *Service) sourceJobs(ctx context.Context) ([]pipeline.PreviewSourceJob, error) {
	if s.opts.Jobs == nil {
		return []pipeline.PreviewSourceJob{}, nil
	}
	if lister, ok := s.opts.Jobs.(SourceJobs); ok {
		return lister.FinishedPreviewSources(ctx)
	}
	// Store/test cũ: đọc bàn dựng có step rồi lọc đúng bốn artifact nguồn.
	list, err := s.opts.Jobs.List(ctx, nil, 10000)
	if err != nil {
		return nil, err
	}
	out := make([]pipeline.PreviewSourceJob, 0)
	for _, job := range list.Jobs {
		if job.Status != domain.CloneSucceeded || job.SupersededAt != nil || job.QueueItem != nil || !hasArtifacts(job.Steps) {
			continue
		}
		source := pipeline.PreviewSourceJob{ID: job.ID, Title: job.Source.Title, DurationSec: scenesDuration(job.Steps), NoImages: job.NoImages}
		if job.Source.ChannelTitle != nil {
			source.ChannelName = *job.Source.ChannelTitle
		}
		out = append(out, source)
	}
	return out, nil
}

// Start dựng bản xem thử trong nền; kiểm tra đầy đủ trước khi trả 202.
func (s *Service) Start(ctx context.Context, templateID string, req library.PreviewRequest) (*library.PreviewStatus, error) {
	minutes := req.Minutes
	if minutes == 0 {
		minutes = defaultMinutes
	}
	if minutes < minMinutes || minutes > maxMinutes {
		return nil, errPreview(library.CodeInvalidInput,
			fmt.Sprintf("Thời lượng xem thử phải từ %d đến %d phút", minMinutes, maxMinutes), "Nhập số phút từ 1 đến 10")
	}
	detail, err := s.opts.Templates.Get(ctx, templateID)
	if err != nil {
		return nil, err
	}
	task, err := s.task(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if task != nil && task.State == domain.TaskRunning {
		status := baseStatus(detail)
		status.State = library.PreviewBuilding
		status.Task = task
		sources, err := s.sourcesFor(ctx, detail, true)
		if err != nil {
			return nil, err
		}
		status.Sources = sources
		return status, nil
	}
	sources, err := s.sources(ctx, &detail.Recipe)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errPreview(library.CodeConflict, "Chưa có video nào dựng xong để xem thử", "Dựng xong một video clone rồi thử lại")
	}
	jobID := req.JobID
	var chosen *library.PreviewSource
	if jobID == "" {
		for i := range sources {
			if !sources[i].Disabled {
				chosen = &sources[i]
				break
			}
		}
		if chosen == nil {
			return nil, errPreview(library.CodeConflict, "Chưa có video nào dựng được với template này", sources[0].Reason)
		}
		jobID = chosen.JobID
	} else {
		for i := range sources {
			if sources[i].JobID == jobID {
				chosen = &sources[i]
				break
			}
		}
		if chosen == nil {
			return nil, errPreview(library.CodeConflict, "Video này chưa dựng xong nên chưa xem thử được", "Chọn video khác để xem thử")
		}
		if chosen.Disabled {
			return nil, errPreview(library.CodeConflict, chosen.Reason, "Chọn video khác để xem thử")
		}
	}
	job, err := s.opts.Jobs.Get(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errPreview(library.CodeConflict, "Video nguồn không còn tồn tại", "Chọn video khác để xem thử")
	}
	// Preflight đồng bộ: mọi lỗi có hành động trả ngay trước 202, không để
	// người dùng thấy một dòng việc hỏng không lý do sau khi bấm dựng.
	plan, err := s.prepare(ctx, detail, job, minutes)
	if err != nil {
		return nil, err
	}

	runID := editor.ID()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errPreview(library.CodeConflict, "Viderse đang tắt nên chưa dựng được bản xem thử", "Mở lại Viderse rồi thử lại")
	}
	if existing := s.runs[templateID]; existing != nil {
		s.mu.Unlock()
		status := baseStatus(detail)
		status.State = library.PreviewBuilding
		status.Task = &library.PreviewTask{ID: existing.taskID, Phase: "Đang chuẩn bị", State: domain.TaskRunning, ActionHref: templateHref(templateID)}
		status.Sources = sources
		return status, nil
	}
	if err := s.opts.Templates.BeginTemplatePreview(ctx, templateID, runID); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	runCtx, cancel := context.WithCancel(s.lifetime)
	current := &run{id: runID, templateID: templateID, cancel: cancel, done: make(chan struct{})}
	if s.opts.Tasks != nil {
		taskID, err := s.opts.Tasks.Start(runCtx, domain.TaskSpec{
			Kind: domain.TaskTemplatePreview, Title: "Dựng bản xem thử",
			Object: s.objectRef(templateID), Phase: "Đang chuẩn bị",
			Action: domain.TaskAction{Label: "Xem template", Href: templateHref(templateID)},
		})
		if err != nil {
			cancel()
			_ = s.opts.Templates.ClearTemplatePreviewRun(context.Background(), templateID, runID)
			s.mu.Unlock()
			return nil, err
		}
		current.taskID = taskID
	}
	s.runs[templateID] = current
	s.sourceCache[templateID] = append([]library.PreviewSource{}, sources...)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(current.done)
		defer cancel()
		buildErr := s.build(runCtx, current, detail, job, plan, minutes)
		s.finish(current, buildErr)
	}()
	s.mu.Unlock()

	status := baseStatus(detail)
	status.State = library.PreviewBuilding
	status.Task = &library.PreviewTask{ID: current.taskID, Phase: "Đang chuẩn bị", State: domain.TaskRunning, ActionHref: templateHref(templateID)}
	status.Sources = sources
	return status, nil
}

// finish kết thúc một lượt: chỉ lượt đang giữ slot mới được ghi/kết thúc dòng
// việc của nó. Lượt cũ (đã bị huỷ/thay) không được xoá dòng việc của lượt mới.
func (s *Service) finish(current *run, buildErr error) {
	background := context.Background()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs[current.templateID] != current {
		return
	}
	delete(s.runs, current.templateID)
	if buildErr == nil {
		if s.opts.Tasks != nil && current.taskID != "" {
			_ = s.opts.Tasks.Finish(background, current.taskID)
		}
		return
	}
	if errors.Is(buildErr, context.Canceled) || errors.Is(buildErr, context.DeadlineExceeded) || errors.Is(buildErr, library.ErrPreviewSuperseded) {
		_ = s.opts.Templates.ClearTemplatePreviewRun(background, current.templateID, current.id)
		if s.opts.Tasks != nil && current.taskID != "" {
			_ = s.opts.Tasks.Finish(background, current.taskID)
		}
		return
	}
	failure := previewFailure(buildErr)
	// Bản xem thử cũ (project/revision) giữ nguyên; chỉ ghi lỗi + nhả token.
	_ = s.opts.Templates.FailTemplatePreview(background, current.templateID, current.id, failure)
	if s.opts.Tasks != nil && current.taskID != "" {
		phase := failure.Message
		_ = s.opts.Tasks.Update(background, current.taskID, domain.TaskUpdate{Phase: &phase, State: domain.TaskFailed})
	}
}

func previewFailure(err error) *library.PreviewError {
	var business *library.Error
	if errors.As(err, &business) {
		return &library.PreviewError{Code: business.Code, Message: business.Message, Recovery: business.Recovery}
	}
	return &library.PreviewError{Code: "internal", Message: err.Error()}
}

func (r *run) progress(ctx context.Context, s *Service, phase string, done, total int64) {
	if s.opts.Tasks == nil || r.taskID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs[r.templateID] != r {
		return
	}
	_ = s.opts.Tasks.Update(ctx, r.taskID, domain.TaskUpdate{Phase: &phase, Progress: &domain.TaskProgress{Done: done, Total: total}})
}

// Cancel huỷ lần dựng đang chạy và chờ nó dừng (có trần theo ctx). Token công
// bố + dòng việc được nhả trong cùng khoá với Start, nên một Start ngay sau
// không thể nhận lại dòng việc rồi bị Cancel xoá.
func (s *Service) Cancel(ctx context.Context, templateID string) error {
	s.mu.Lock()
	current := s.runs[templateID]
	if current != nil {
		delete(s.runs, templateID)
		current.cancel()
		_ = s.opts.Templates.ClearTemplatePreviewRun(context.Background(), templateID, current.id)
		if s.opts.Tasks != nil && current.taskID != "" {
			_ = s.opts.Tasks.Finish(context.Background(), current.taskID)
		}
	} else if s.opts.Tasks != nil {
		// Không có lượt trong process này: dọn dòng việc còn sót nếu có. Giữ
		// khoá để Start đang chờ không nhận lại dòng vừa bị xoá.
		if task, err := s.opts.Tasks.FindTaskByObject(ctx, domain.TaskTemplatePreview, s.objectRef(templateID)); err == nil && task != nil {
			_ = s.opts.Tasks.Finish(ctx, task.ID)
		}
	}
	s.mu.Unlock()
	if current == nil {
		return nil
	}
	select {
	case <-current.done:
		return nil
	case <-ctx.Done():
		// Token đã nhả và dòng việc đã kết thúc; phần dọn project do goroutine
		// tự làm bằng context nền, có danh mục giữ dấu nếu nó chưa xong.
		return nil
	}
}

// Drop xoá project xem thử theo mã project và bỏ dấu danh mục khi xoá xong.
func (s *Service) Drop(ctx context.Context, projectID string) error {
	if projectID == "" || s.opts.Editor == nil {
		return nil
	}
	if err := s.opts.Editor.DeleteProject(ctx, projectID); err != nil {
		return err
	}
	return s.opts.Templates.ForgetPreviewProject(ctx, projectID)
}

// Reconcile dọn project xem thử mồ côi sau crash/tắt máy: dòng danh mục không
// còn là con trỏ hiện tại (hoặc template đã xoá) bị xoá rồi bỏ dấu. Chạy một
// lần lúc khởi động, trước khi nhận request; không phải daemon thử lại.
func (s *Service) Reconcile(ctx context.Context) error {
	if s.opts.Editor == nil {
		return nil
	}
	rows, err := s.opts.Templates.PreviewProjectRows(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Current || row.ProjectID == "" {
			continue
		}
		// Từng project là best effort: lỗi xoá (kể cả project đã biến mất) giữ
		// nguyên dấu — project vẫn bị ẩn và lần khởi động sau thử lại.
		if err := s.opts.Editor.DeleteProject(ctx, row.ProjectID); err != nil {
			continue
		}
		_ = s.opts.Templates.ForgetPreviewProject(ctx, row.ProjectID)
	}
	// Tệp tạm (thư mục âm thanh/ảnh tách nền) do defer dọn khi chạy bình
	// thường; bản cũ hơn 1 giờ là rác của lần crash trước. Không phải TTL của
	// dữ liệu nghiệp vụ và không ảnh hưởng nguồn/ảnh đã publish.
	sweepPreviewTemp(s.now())
	return nil
}

func sweepPreviewTemp(now time.Time) {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "viderse-preview-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) < time.Hour {
			continue
		}
		_ = os.RemoveAll(filepath.Join(os.TempDir(), name))
	}
}

// Close huỷ và chờ mọi lượt dựng dừng (có trần theo ctx). Phải chạy trước khi
// media/pool đóng; phần dọn dùng context nền nên vẫn ghi được nhờ Close chờ.
func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, current := range s.runs {
		current.cancel()
	}
	s.mu.Unlock()
	s.stopLifetime()
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

// ── đọc artifact ───────────────────────────────────────────────────────────

// artifactRecord là một artifact đã publish của job nguồn.
type artifactRecord struct {
	path   string
	sha256 string
	bytes  int64
}

func hasArtifacts(steps []domain.CloneStepRecord) bool {
	for _, rel := range pipeline.PreviewSourceArtifacts {
		if _, ok := artifactRecordOf(steps, rel); !ok {
			return false
		}
	}
	return true
}

func scenesDuration(steps []domain.CloneStepRecord) float64 {
	for _, step := range steps {
		if step.Step != domain.CloneScenes || step.Output == nil {
			continue
		}
		if value, ok := step.Output.Data["durationSec"].(float64); ok {
			return value
		}
	}
	return 0
}

func artifactRecordOf(steps []domain.CloneStepRecord, rel string) (artifactRecord, bool) {
	for _, step := range steps {
		if step.Output == nil {
			continue
		}
		for _, artifact := range step.Output.Artifacts {
			if artifact.Path == rel || strings.HasSuffix(artifact.Path, "/"+rel) {
				return artifactRecord{path: artifact.Path, sha256: artifact.SHA256, bytes: artifact.Bytes}, true
			}
		}
	}
	return artifactRecord{}, false
}

// openArtifact mở artifact nguồn và xác minh nội dung bằng sha256 đã ghi. Tệp
// hỏng/đọc sai là lỗi có hành động, không phải "bỏ qua rồi thử tệp khác".
func (s *Service) openArtifact(ctx context.Context, steps []domain.CloneStepRecord, rel, missingMessage, recovery string) (*media.Handle, error) {
	record, ok := artifactRecordOf(steps, rel)
	if !ok {
		return nil, errPreview(library.CodeConflict, missingMessage, recovery)
	}
	if err := media.ValidatePath(record.path); err != nil {
		return nil, errPreview(library.CodeConflict, missingMessage, recovery)
	}
	handle, err := s.opts.Media.Acquire(ctx, record.path)
	if err != nil {
		return nil, errPreview(library.CodeConflict, missingMessage, recovery)
	}
	if record.sha256 != "" {
		hash, err := sha256File(handle.Path)
		if err != nil || hash != record.sha256 {
			handle.Release()
			return nil, errPreview(library.CodeConflict, missingMessage, recovery)
		}
	}
	return handle, nil
}

func readArtifact[T any](ctx context.Context, s *Service, steps []domain.CloneStepRecord, rel, missingMessage, recovery string) (T, error) {
	var out T
	handle, err := s.openArtifact(ctx, steps, rel, missingMessage, recovery)
	if err != nil {
		return out, err
	}
	defer handle.Release()
	data, err := os.ReadFile(handle.Path)
	if err != nil {
		return out, errPreview(library.CodeConflict, missingMessage, recovery)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, errPreview(library.CodeConflict, missingMessage, recovery)
	}
	return out, nil
}

// ── chuẩn bị (preflight) ───────────────────────────────────────────────────

// buildPlan là dữ liệu nguồn đã kiểm tra trước 202; build dùng đúng plan này
// nên không đọc/đoán lại nguồn rồi lệch với bước kiểm.
type buildPlan struct {
	durationSec    float64
	durationFrames int
	sceneImages    []images.Item
	captions       transcript.CaptionManifest
}

func (s *Service) prepare(ctx context.Context, detail *library.Detail, job *domain.CloneJobView, minutes int) (*buildPlan, error) {
	if job.Status != domain.CloneSucceeded || job.SupersededAt != nil {
		return nil, errPreview(library.CodeConflict, "Video nguồn đã cũ hoặc đã bị thay", "Chọn video khác để xem thử")
	}
	project, err := editor.Parse(detail.Project)
	if err != nil {
		return nil, errPreview(library.CodeInvalidInput, "Project của template không đọc được", "Mở template trong editor rồi lưu lại")
	}
	fps := objectNumber(project.Metadata(), "fps", 0)
	if fps <= 0 {
		return nil, errPreview(library.CodeInvalidInput, "Template thiếu fps", "Mở template trong editor rồi lưu lại")
	}
	sourceSec := scenesDuration(job.Steps)
	if sourceSec <= 0 {
		manifest, err := readArtifact[scenes.Manifest](ctx, s, job.Steps, "scenes/manifest.json",
			"Manifest cảnh của video không đọc được", "Dựng lại video nguồn rồi thử lại")
		if err != nil {
			return nil, err
		}
		sourceSec = manifest.DurationSec
	}
	durationSec := math.Min(float64(minutes*60), sourceSec)
	if durationSec <= 0 {
		return nil, errPreview(library.CodeConflict, "Không đọc được thời lượng video nguồn", "Chọn video khác để xem thử")
	}
	plan := &buildPlan{
		durationSec:    durationSec,
		durationFrames: int(math.Round(durationSec * fps)),
		captions:       transcript.CaptionManifest{},
	}
	// Số cảnh đúng bằng số ảnh pipeline tạo ra cho đúng cửa sổ này — không
	// nhân/chặn sàn riêng, để bản xem thử có đúng số cảnh video thật sẽ có.
	sceneCount := library.ImageCountFor(detail.Recipe.Scenes.Images, durationSec)
	if sceneCount > 0 {
		manifest, err := readArtifact[images.Manifest](ctx, s, job.Steps, "images/manifest.json",
			"Manifest ảnh của video không đọc được", "Dựng lại video nguồn rồi thử lại")
		if err != nil {
			return nil, err
		}
		if !images.ImagesApproved(manifest) {
			return nil, errPreview(library.CodeConflict, "Video này chưa duyệt xong ảnh", "Duyệt ảnh của video hoặc chọn video khác")
		}
		available := approvedSceneImages(manifest)
		if len(available) == 0 {
			return nil, errPreview(library.CodeConflict, "Video này không có ảnh cảnh đã duyệt", "Chọn video khác để xem thử")
		}
		for index := range min(sceneCount, len(available)) {
			version := images.Current(available[index])
			if version == nil {
				return nil, errPreview(library.CodeConflict,
					"Ảnh cảnh của video chưa có bản được duyệt", "Duyệt ảnh của video hoặc chọn video khác")
			}
			if detail.Recipe.Scenes.Style == library.StyleCutout {
				if version.Cutout == nil && runtime.GOOS != "darwin" {
					return nil, errPreview(library.CodeConflict,
						"Ảnh cảnh chưa có bản tách nền và tách nền cục bộ chỉ chạy trên macOS",
						"Chọn video nguồn dùng template không tách nền")
				}
				if version.Cutout != nil {
					// Chỉ xác minh: nhả lease đọc ngay, không giữ qua lúc dựng
					// (build cần khoá nội dung độc quyền để nhập media).
					cutoutHandle, err := s.acquireSourceRef(ctx, job.ID, job.Steps, *version.Cutout)
					if err != nil {
						return nil, err
					}
					cutoutHandle.Release()
				}
			}
			// Xác minh tệp ảnh thật (mở được + băm khớp) trước 202, không để
			// tệp hỏng biến thành lỗi mờ trong lúc dựng.
			imageHandle, err := s.acquireSourceRef(ctx, job.ID, job.Steps, scenes.FileRef{Path: version.Path, SHA256: version.SHA256})
			if err != nil {
				return nil, err
			}
			imageHandle.Release()
		}
		plan.sceneImages = available
	}
	captions, err := readArtifact[transcript.CaptionManifest](ctx, s, job.Steps, "transcript/captions.json",
		"Manifest phụ đề của video không đọc được", "Dựng lại video nguồn rồi thử lại")
	if err != nil {
		return nil, err
	}
	plan.captions = captions
	audioHandle, err := s.openArtifact(ctx, job.Steps, "build/source-audio.m4a",
		"Video nguồn thiếu âm thanh hoặc âm thanh hỏng", "Dựng lại video nguồn rồi thử lại")
	if err != nil {
		return nil, err
	}
	audioHandle.Release()
	if _, ok := s.ffmpeg(ctx); !ok {
		return nil, errPreview(library.CodeToolMissing, "ffmpeg chưa sẵn sàng", "Mở Cài đặt → Công cụ và cài ffmpeg")
	}
	return plan, nil
}

// ── build ──────────────────────────────────────────────────────────────────

func (s *Service) build(ctx context.Context, current *run, detail *library.Detail, job *domain.CloneJobView, plan *buildPlan, minutes int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	revision := detail.Revision
	project, err := editor.Parse(detail.Project)
	if err != nil {
		return errPreview(library.CodeInvalidInput, "Project của template không đọc được", "Mở template trong editor rồi lưu lại")
	}
	fps := objectNumber(project.Metadata(), "fps", 0)
	if fps <= 0 {
		return errPreview(library.CodeInvalidInput, "Template thiếu fps", "Mở template trong editor rồi lưu lại")
	}
	durationSec := plan.durationSec
	durationFrames := plan.durationFrames
	// Framing nằm trên recipe đã chuẩn hoá (project ghi "cutout-close", recipe
	// ghi style=cutout + framing=close). So sánh chuỗi style là ngõ cụt.
	cutout := detail.Recipe.Scenes.Style == library.StyleCutout
	closeUp := detail.Recipe.Scenes.Framing == library.FramingClose
	sceneCount := library.ImageCountFor(detail.Recipe.Scenes.Images, durationSec)
	// Thư mục tạm dùng cho cả ảnh chờ nhập lẫn âm thanh: một chỗ dọn, kể cả
	// khi lượt dựng bị huỷ giữa chừng.
	tempDir, err := os.MkdirTemp("", "viderse-preview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	current.progress(ctx, s, "Đang đọc ảnh đã duyệt", 1, 4)
	mediaObjects := []editor.Object{}
	sceneMedia := []editor.FillScene{}
	if sceneCount > 0 {
		sceneImages := plan.sceneImages
		if len(sceneImages) == 0 {
			return errPreview(library.CodeConflict, "Video này không có ảnh cảnh đã duyệt", "Chọn video khác để xem thử")
		}
		type importedMedia struct{ id, src string }
		imported := map[string]importedMedia{}
		resolveImage := func(index int) (editor.FillScene, error) {
			item := sceneImages[index%len(sceneImages)]
			version := images.Current(item)
			if version == nil {
				return editor.FillScene{}, errPreview(library.CodeConflict,
					"Ảnh cảnh của video chưa có bản được duyệt", "Duyệt ảnh của video hoặc chọn video khác")
			}
			ref := scenes.FileRef{Path: version.Path, SHA256: version.SHA256}
			mime := version.Mime
			generated := ""
			if cutout {
				if version.Cutout != nil {
					ref = *version.Cutout
					mime = "image/png"
				} else {
					// Ảnh duyệt không kèm bản tách nền: chạy lại bộ tách nền cục bộ
					// (cùng script của bước vẽ ảnh), không gọi AI/provider.
					path, hash, cleanup, err := s.cutoutSource(ctx, job.ID, job.Steps, ref)
					if err != nil {
						return editor.FillScene{}, err
					}
					defer cleanup()
					ref = scenes.FileRef{Path: path, SHA256: hash}
					mime = "image/png"
					generated = path
				}
			}
			if cached, ok := imported[ref.SHA256]; ok {
				var subject *editor.SubjectBox
				if closeUp {
					if generated == "" {
						handle, err := s.acquireSourceRef(ctx, job.ID, job.Steps, ref)
						if err != nil {
							return editor.FillScene{}, err
						}
						subject, err = measureSubjectBox(handle.Path)
						handle.Release()
						if err != nil {
							return editor.FillScene{}, cutoutSubjectError(ref)
						}
					} else {
						measured, err := measureSubjectBox(generated)
						if err != nil {
							return editor.FillScene{}, cutoutSubjectError(ref)
						}
						subject = measured
					}
				}
				return editor.FillScene{MediaID: cached.id, Src: cached.src, SubjectBox: subject}, nil
			}
			var file string
			if generated != "" {
				file = generated
			} else {
				handle, err := s.acquireSourceRef(ctx, job.ID, job.Steps, ref)
				if err != nil {
					return editor.FillScene{}, err
				}
				// ImportFileOnce công bố cùng mã nội dung và cần khoá độc
				// quyền; chép bytes trong lúc còn lease rồi nhả trước khi nhập.
				staged, stageErr := stageSource(ctx, tempDir, handle)
				handle.Release()
				if stageErr != nil {
					return editor.FillScene{}, stageErr
				}
				defer os.Remove(staged)
				file = staged
			}
			var subject *editor.SubjectBox
			if closeUp {
				measured, err := measureSubjectBox(file)
				if err != nil {
					return editor.FillScene{}, cutoutSubjectError(ref)
				}
				subject = measured
			}
			rec, _, err := s.opts.Editor.ImportFileOnce(ctx, editor.ImportFileOnceOptions{
				File: file, FileName: "preview-" + item.ID + filepath.Ext(ref.Path),
				DeclaredMIME: mime, ContentHash: ref.SHA256, MIMEPrefix: "image/",
			})
			if err != nil {
				return editor.FillScene{}, err
			}
			object := mediaObject(rec)
			mediaObjects = append(mediaObjects, object)
			value := importedMedia{id: rec.ID, src: mediaSourceURL(rec.ID, rec.ContentHash)}
			imported[ref.SHA256] = value
			return editor.FillScene{MediaID: value.id, Src: value.src, SubjectBox: subject}, nil
		}
		current.progress(ctx, s, "Đang nhập ảnh cảnh", 2, 4)
		for index := range sceneCount {
			if err := ctx.Err(); err != nil {
				return err
			}
			chosen, err := resolveImage(index)
			if err != nil {
				return err
			}
			chosen.SceneID = fmt.Sprintf("p%d", index)
			chosen.Title = fmt.Sprintf("Cảnh %d", index+1)
			chosen.StartFrame = int(float64(index) * float64(durationFrames) / float64(sceneCount))
			chosen.EndFrame = int(float64(index+1) * float64(durationFrames) / float64(sceneCount))
			sceneMedia = append(sceneMedia, chosen)
		}
	} else {
		current.progress(ctx, s, "Template không vẽ ảnh cảnh", 2, 4)
	}
	current.progress(ctx, s, "Đang chuẩn bị âm thanh", 3, 4)
	audioHandle, err := s.openArtifact(ctx, job.Steps, "build/source-audio.m4a",
		"Video nguồn thiếu âm thanh hoặc âm thanh hỏng", "Dựng lại video nguồn rồi thử lại")
	if err != nil {
		return err
	}
	defer audioHandle.Release()
	ffmpeg, ok := s.ffmpeg(ctx)
	if !ok {
		return errPreview(library.CodeToolMissing, "ffmpeg chưa sẵn sàng", "Mở Cài đặt → Công cụ và cài ffmpeg")
	}
	audioFile := filepath.Join(tempDir, "preview-audio.m4a")
	if _, err := s.opts.Runner.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", audioHandle.Path,
		"-map", "0:a:0", "-vn", "-sn", "-dn", "-t", fmt.Sprintf("%.3f", durationSec), "-c:a", "copy", audioFile},
		tools.Spec{Timeout: 5 * time.Minute, LogLimit: 4096}); err != nil {
		return err
	}
	// ffmpeg đã đọc xong tệp nguồn: nhả lease trước khi băm/nhập bản sao để
	// bước công bố lấy được khoá nội dung độc quyền.
	audioHandle.Release()
	audioHash, err := sha256File(audioFile)
	if err != nil {
		return err
	}
	audioRecord, _, err := s.opts.Editor.ImportFileOnce(ctx, editor.ImportFileOnceOptions{
		File: audioFile, FileName: "preview-audio.m4a", DeclaredMIME: "audio/mp4",
		ContentHash: audioHash, MIMEPrefix: "audio/",
	})
	if err != nil {
		return err
	}
	audioObject := mediaObject(audioRecord)
	mediaObjects = append(mediaObjects, audioObject)
	pcm := filepath.Join(tempDir, "envelope.f32le")
	if _, err := s.opts.Runner.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", audioFile,
		"-vn", "-ac", "2", "-ar", "48000", "-f", "f32le", pcm},
		tools.Spec{Timeout: 5 * time.Minute, LogLimit: 1024}); err != nil {
		return err
	}
	pcmFile, err := os.Open(pcm)
	if err != nil {
		return err
	}
	envelope, err := build.DecodeEnvelopePCM(pcmFile, "sha256:"+audioHash)
	_ = pcmFile.Close()
	if err != nil {
		return err
	}
	audioFrames := durationFrames
	if duration := audioRecord.Probe.DurationSec; duration != nil && *duration > 0 {
		if frames := int(math.Round(*duration * fps)); frames > 0 && frames < audioFrames {
			audioFrames = frames
		}
	}

	current.progress(ctx, s, "Đang đổ vào template", 4, 4)
	cues := clippedCues(plan.captions.Cues, durationSec)
	seed := uint32(0x811c9dc5)
	for _, c := range []byte(job.ID + detail.ID) {
		seed ^= uint32(c)
		seed *= 0x01000193
	}
	fill, err := editor.BuildTemplateFill(editor.FillInput{
		Project: project, DurationFrames: durationFrames, IDPrefix: previewPrefix, Seed: seed,
		Scenes: sceneMedia, Cues: cues,
		Audio: editor.FillAudio{MediaID: audioRecord.ID, Src: mediaSourceURL(audioRecord.ID, audioRecord.ContentHash), DurationFrames: audioFrames},
		Media: mediaObjects, Envelope: envelope,
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	name := trimName("Xem thử · " + detail.Name)
	previewID := editor.ID()
	// Ghi danh mục ẩn TRƯỚC khi project tồn tại. Template vừa bị xoá thì
	// insert thất bại và không có project mồ côi nào được tạo.
	if err := s.opts.Templates.RegisterTemplatePreviewProject(ctx, detail.ID, previewID, job.ID, minutes); err != nil {
		if errors.Is(err, library.ErrNotFound) {
			return errPreview(library.CodeConflict, "Template vừa bị xoá", "Tải lại thư viện template")
		}
		return err
	}
	duplicated, baseRevision, err := s.opts.Editor.DuplicateProjectAs(ctx, detail.ID, name, previewID)
	if err != nil {
		_ = s.opts.Templates.ForgetPreviewProject(context.Background(), previewID)
		return err
	}
	previewProject := previewID
	cleanup := true
	defer func() {
		if !cleanup || previewProject == "" {
			return
		}
		// Context nền: dọn được cả khi lượt bị huỷ (Close vẫn đang chờ ta).
		if err := s.opts.Editor.DeleteProject(context.Background(), previewProject); err == nil {
			_ = s.opts.Templates.ForgetPreviewProject(context.Background(), previewProject)
		}
	}()
	if duplicated != nil {
		if id, _ := duplicated["id"].(string); id != "" && id != previewID {
			return errPreview(library.CodeInvalidInput, "Project xem thử tạo sai mã", "Thử lại")
		}
	}
	if _, err := s.opts.Editor.RunCommands(ctx, previewProject, fill.Commands, editor.ApplyCommandsOptions{DryRun: true, BaseRevision: baseRevision}); err != nil {
		return err
	}
	if _, err := s.opts.Editor.RunCommands(ctx, previewProject, fill.Commands, editor.ApplyCommandsOptions{BaseRevision: baseRevision}); err != nil {
		return err
	}
	// Revision đọc lúc bắt đầu, không phải revision hiện tại: sửa template giữa
	// chừng để lại bản xem thử cũ (stale), không giả làm mới.
	if err := ctx.Err(); err != nil {
		return err
	}
	// Video nguồn có thể bị thay trong lúc dựng: bản xem thử không được ghi
	// nhận từ một job đã superseded.
	currentJob, err := s.opts.Jobs.Get(ctx, job.ID)
	if err != nil || currentJob == nil || currentJob.Status != domain.CloneSucceeded || currentJob.SupersededAt != nil {
		return errPreview(library.CodeConflict, "Video nguồn vừa bị thay trong lúc dựng", "Dựng lại bản xem thử")
	}
	previous, err := s.opts.Templates.RecordTemplatePreview(ctx, detail.ID, library.TemplatePreviewRecord{
		ProjectID: previewProject, JobID: job.ID, Minutes: minutes, TemplateRevision: revision, BuiltAt: s.now().UTC(), RunID: current.id,
	})
	if err != nil {
		return err
	}
	cleanup = false
	if previous != "" && previous != previewProject {
		// Dọn bản cũ sau khi công bố bản mới. Lỗi xoá để dòng danh mục lại —
		// project cũ vẫn bị ẩn và lần khởi động sau thử dọn tiếp.
		if err := s.opts.Editor.DeleteProject(context.Background(), previous); err == nil {
			_ = s.opts.Templates.ForgetPreviewProject(context.Background(), previous)
		}
	}
	return nil
}

// acquireSourceRef mở đúng tệp của một FileRef trong job nguồn và xác minh
// nội dung bằng SHA256. Ưu tiên catalog artifact đã publish; nếu không có thì
// dùng hợp đồng lưu trữ thật của stepMedia.Publish:
// clone/<jobID>/<sha256>/<relative path>. Không có "tồn tại giả": tệp phải mở
// được và băm khớp, nếu không là lỗi có hành động.
func (s *Service) acquireSourceRef(ctx context.Context, jobID string, steps []domain.CloneStepRecord, ref scenes.FileRef) (*media.Handle, error) {
	if err := validateSourceRef(ref); err != nil {
		return nil, err
	}
	candidates := make([]string, 0, 2)
	if published, ok := artifactRecordOf(steps, ref.Path); ok {
		candidates = append(candidates, published.path)
	}
	deterministic := "clone/" + jobID + "/" + ref.SHA256 + "/" + ref.Path
	if len(candidates) == 0 || candidates[0] != deterministic {
		candidates = append(candidates, deterministic)
	}
	for _, logical := range candidates {
		if err := media.ValidatePath(logical); err != nil {
			continue
		}
		handle, err := s.opts.Media.Acquire(ctx, logical)
		if err != nil {
			continue
		}
		hash, err := sha256File(handle.Path)
		if err != nil || hash != ref.SHA256 {
			handle.Release()
			continue
		}
		return handle, nil
	}
	return nil, errPreview(library.CodeConflict,
		"Không đọc được ảnh đã duyệt của video nguồn (thiếu tệp hoặc sai mã nội dung)",
		"Dựng lại video nguồn rồi thử lại")
}

func validateSourceRef(ref scenes.FileRef) error {
	if !isSHA256Hex(ref.SHA256) {
		return errPreview(library.CodeConflict, "Ảnh nguồn thiếu mã nội dung hợp lệ", "Dựng lại video nguồn rồi thử lại")
	}
	if err := media.ValidatePath(ref.Path); err != nil || !strings.HasPrefix(ref.Path, "images/") {
		return errPreview(library.CodeConflict, "Đường dẫn ảnh nguồn không hợp lệ", "Dựng lại video nguồn rồi thử lại")
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func cutoutSubjectError(ref scenes.FileRef) error {
	return errPreview(library.CodeConflict,
		"Ảnh "+filepath.Base(ref.Path)+" không tách được chủ thể để dựng cận cảnh",
		"Chọn video khác hoặc duyệt lại ảnh có chủ thể rõ")
}

// cutoutSource chạy bộ tách nền cục bộ có sẵn (script của bước vẽ ảnh) trên
// tệp ảnh nguồn đã xác minh; trả tệp PNG tạm + mã băm + hàm dọn.
func (s *Service) cutoutSource(ctx context.Context, jobID string, steps []domain.CloneStepRecord, ref scenes.FileRef) (string, string, func(), error) {
	if runtime.GOOS != "darwin" {
		return "", "", nil, errPreview(library.CodeConflict,
			"Tách nền cục bộ chỉ chạy trên macOS", "Chọn video nguồn dùng template không tách nền")
	}
	handle, err := s.acquireSourceRef(ctx, jobID, steps, ref)
	if err != nil {
		return "", "", nil, err
	}
	defer handle.Release()
	output, err := os.CreateTemp("", "viderse-preview-cutout-*.png")
	if err != nil {
		return "", "", nil, err
	}
	outputPath := output.Name()
	if err := output.Close(); err != nil {
		_ = os.Remove(outputPath)
		return "", "", nil, err
	}
	cleanup := func() { _ = os.Remove(outputPath) }
	if _, err := s.opts.Runner.Run(ctx, "/usr/bin/osascript", []string{"-l", "JavaScript", "-e", images.CutoutScript, handle.Path, outputPath}, tools.Spec{Timeout: 2 * time.Minute, LogLimit: 4096}); err != nil {
		cleanup()
		return "", "", nil, err
	}
	hash, err := sha256File(outputPath)
	if err != nil {
		cleanup()
		return "", "", nil, err
	}
	return outputPath, hash, cleanup, nil
}

// stageSource copies a leased source into an unleased temp file inside dir.
// ImportFileOnce publishes the same content digest and needs its exclusive
// publication lock, so the shared read lease must never be held across it.
// The copy is context-aware and bounded by the source size: a changed file
// cannot be staged as a different, larger payload.
func stageSource(ctx context.Context, dir string, handle *media.Handle) (string, error) {
	info, err := os.Stat(handle.Path)
	if err != nil {
		return "", err
	}
	source, err := os.Open(handle.Path)
	if err != nil {
		return "", err
	}
	target, err := os.CreateTemp(dir, "preview-source-*"+filepath.Ext(handle.Path))
	if err != nil {
		return "", errors.Join(err, source.Close())
	}
	name := target.Name()
	written, copyErr := copyBounded(ctx, target, source, info.Size()+1)
	sourceErr := source.Close()
	closeErr := target.Close()
	if copyErr == nil && sourceErr == nil && closeErr == nil && written != info.Size() {
		copyErr = fmt.Errorf("tệp nguồn sai kích thước: %s (%d/%d)", filepath.Base(handle.Path), written, info.Size())
	}
	if err := errors.Join(copyErr, sourceErr, closeErr); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// copyBounded copies at most limit bytes, checking ctx between chunks.
func copyBounded(ctx context.Context, w io.Writer, r io.Reader, limit int64) (int64, error) {
	var total int64
	buf := make([]byte, 256<<10)
	for total < limit {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		want := limit - total
		if want > int64(len(buf)) {
			want = int64(len(buf))
		}
		n, readErr := r.Read(buf[:want])
		if n > 0 {
			wn, writeErr := w.Write(buf[:n])
			total += int64(wn)
			if writeErr != nil {
				return total, writeErr
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
	return total, nil
}

func (s *Service) ffmpeg(ctx context.Context) (string, bool) {
	if s.opts.Ffmpeg == nil {
		return "", false
	}
	return s.opts.Ffmpeg(ctx)
}

// ── helpers ────────────────────────────────────────────────────────────────

func approvedSceneImages(manifest images.Manifest) []images.Item {
	out := make([]images.Item, 0)
	for _, item := range manifest.Items {
		if item.Kind != "scene" || images.Current(item) == nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func clippedCues(cues []transcript.Cue, windowSec float64) []editor.FillCue {
	out := make([]editor.FillCue, 0, len(cues))
	windowMs := windowSec * 1000
	for _, cue := range cues {
		if cue.EndMs == nil || *cue.EndMs <= cue.StartMs || cue.StartMs >= windowMs {
			continue
		}
		fill := editor.FillCue{ID: cue.ID, Text: cue.Text, StartSec: cue.StartMs / 1000, EndSec: math.Min(*cue.EndMs, windowMs) / 1000}
		for _, word := range cue.Words {
			if word.StartMs >= windowMs {
				continue
			}
			fill.Words = append(fill.Words, editor.FillCueWord{StartMs: word.StartMs, EndMs: math.Min(word.EndMs, windowMs), Text: word.Text, Estimated: word.Estimated()})
		}
		out = append(out, fill)
	}
	return out
}

func measureSubjectBox(file string) (*editor.SubjectBox, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	left, top, right, bottom := bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha > 16*257 {
				left, top = min(left, x), min(top, y)
				right, bottom = max(right, x+1), max(bottom, y+1)
			}
		}
	}
	if right <= left || bottom <= top {
		return nil, fmt.Errorf("no foreground subject")
	}
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	return &editor.SubjectBox{X: float64(left-bounds.Min.X) / w, Y: float64(top-bounds.Min.Y) / h, Width: float64(right-left) / w, Height: float64(bottom-top) / h}, nil
}

func mediaSourceURL(id, hash string) string {
	return "/api/editor/media/" + url.PathEscape(id) + "?v=" + url.QueryEscape(hash)
}

func mediaObject(record editor.MediaRecord) editor.Object {
	probe := record.Probe
	fps, codec := 0.0, "unknown"
	duration, width, height := 0.0, 0.0, 0.0
	if probe.DurationSec != nil {
		duration = *probe.DurationSec
	}
	if probe.Width != nil {
		width = *probe.Width
	}
	if probe.Height != nil {
		height = *probe.Height
	}
	if probe.Kind == "video" {
		if probe.FPS != nil {
			fps = *probe.FPS
		}
		if fps == 0 {
			fps = 30
		}
		if probe.VideoCodec != nil {
			codec = *probe.VideoCodec
		}
	}
	if probe.Kind == "audio" && probe.AudioCodec != nil {
		codec = *probe.AudioCodec
	}
	tags := record.Tags
	if tags == nil {
		tags = []string{}
	}
	return editor.Object{
		"id": record.ID, "fileName": record.FileName, "fileSize": record.Size,
		"mimeType": record.MIME, "contentHash": record.ContentHash, "storageType": "workspace",
		"width": width, "height": height, "duration": duration, "fps": fps,
		"codec": codec, "bitrate": 0, "tags": tags,
		"createdAt": record.CreatedAt, "updatedAt": record.CreatedAt,
	}
}

func objectNumber(m editor.Object, key string, fallback float64) float64 {
	switch value := m[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	}
	return fallback
}

func trimName(name string) string {
	runes := []rune(name)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return name
}

func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
