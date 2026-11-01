package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// LeaseTTL là hạn gia hạn job lease; heartbeat mỗi HeartbeatEvery.
const (
	LeaseTTL       = 15 * time.Second
	HeartbeatEvery = 2 * time.Second
	// ProgressCoalesce: tối đa 4 sự kiện tiến độ/giây/job (hợp đồng clone.step.progress).
	ProgressCoalesce = 250 * time.Millisecond
)

// Store là tập thao tác bền vững runner cần — CloneStore (postgres) hiện thực;
// test dùng bản giả in-memory.
type Store interface {
	GetJob(ctx context.Context, jobID string) (*domain.CloneJob, error)
	ListArtifacts(context.Context, string) ([]domain.CloneArtifact, error)
	RecordArtifacts(context.Context, string, domain.CloneStep, []domain.CloneArtifact) error
	ListSteps(ctx context.Context, jobID string) ([]domain.CloneStepRecord, error)
	GetStep(ctx context.Context, jobID string, step domain.CloneStep) (*domain.CloneStepRecord, error)
	ClaimJobLease(ctx context.Context, jobID, owner string, ttl time.Duration) (*domain.CloneJob, error)
	HeartbeatJobLease(ctx context.Context, jobID, owner string, ttl time.Duration) (bool, error)
	ReleaseJobLease(ctx context.Context, jobID, owner string) error
	ClaimNextStep(ctx context.Context, jobID, workerID string) (*ClaimedStep, error)
	WriteStepProgress(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, p domain.CloneProgress) error
	WriteStepCheckpoint(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, checkpoint json.RawMessage) error
	WriteStepProcessGroup(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, pgid int) error
	WriteStepFingerprint(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, fingerprint string) error
	ApplyStepSuccess(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, output domain.CloneStepOutput) error
	ApplyStepWaiting(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, output *domain.CloneStepOutput, cerr *domain.CloneError) error
	ReportStepFailure(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int, cerr *domain.CloneError) error
	ReportStepCancelled(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int) error
	ParkStepPaused(ctx context.Context, jobID string, step domain.CloneStep, fence int64, attempt int) error
	SetJobStatus(ctx context.Context, jobID string, status domain.CloneStatus) error
}

// AutoFixer là Mức tự chủ của kênh nhìn từ runner: step vừa hỏng hẳn có được
// tự thử lại một lần không, và attempt nào đã xong thì đánh dấu đã phục hồi.
type AutoFixer interface {
	AutoFix(ctx context.Context, jobID string, step domain.CloneStep, cerr *domain.CloneError) bool
	AttemptSettled(ctx context.Context, jobID string, step domain.CloneStep, status domain.CloneStatus)
}

// Publisher phát sự kiện ra hub SSE. Nil → không phát.
type Publisher func(eventType string, data any)

// Settings đọc luồng chạy từ settings bền vững (Quy tắc chạy).
type Settings interface {
	JobConcurrency() int
	ImageConcurrency() int
}

// StepInputResolver tính đầu vào fingerprint của step; handler có thể thêm
// input riêng — mặc định {step, source, upstream}.
type StepInputResolver func(step domain.CloneStep, job domain.CloneJob, upstream map[domain.CloneStep]domain.CloneStepOutput) any

func defaultInputResolver(step domain.CloneStep, job domain.CloneJob, upstream map[domain.CloneStep]domain.CloneStepOutput) any {
	return map[string]any{"step": step, "source": job.Source, "upstream": upstream}
}

// Runner chạy chuỗi step của một job trong goroutine lane của worker.
type Runner struct {
	Store        Store
	Handlers     Registry
	Media        *media.Service
	Runner       tools.Runner
	Publish      Publisher
	StagingRoot  string // thư mục staging gốc; mỗi step dùng <root>/<jobID>/gen<N>/<step>
	ImageSlots   chan struct{}
	ImageLimiter *ImageLimiter
	Settings     Settings
	ResolveInput StepInputResolver
	Queue        QueueDrain
	OnHumanState func(context.Context, *domain.CloneJob, string)
	// AutoFix nil → không tự thử lại (Mức tự chủ không áp dụng).
	AutoFix    AutoFixer
	progressMu sync.Mutex
	progressAt map[string]time.Time
	// OnAttemptSettled (test hook): gọi sau mỗi attempt kết thúc.
	OnAttemptSettled func(step domain.CloneStep)
}

// RunJob thực thi chuỗi job tới khi xong/đỗ/bị huỷ — port của runJob():
// claim lease → reconcile → vòng claim+run; heartbeat gia hạn lease và phát
// hiện huỷ/tạm dừng. Trả status cuối của job.
func (r *Runner) RunJob(ctx context.Context, jobID, workerID string) (domain.CloneStatus, error) {
	job, err := r.Store.ClaimJobLease(ctx, jobID, workerID, LeaseTTL)
	if err != nil {
		return "", err
	}
	if job == nil {
		return "", fmt.Errorf("job %s: %w", jobID, ErrConflict)
	}
	defer r.Store.ReleaseJobLease(context.WithoutCancel(ctx), jobID, workerID)
	if r.Queue != nil {
		defer r.Queue.RequestDrain()
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Heartbeat: gia hạn lease + phát hiện huỷ/tạm dừng supersede.
	go func() {
		tick := time.NewTicker(HeartbeatEvery)
		defer tick.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-tick.C:
				ok, err := r.Store.HeartbeatJobLease(jobCtx, jobID, workerID, LeaseTTL)
				if err != nil || !ok {
					cancel()
					return
				}
				j, err := r.Store.GetJob(jobCtx, jobID)
				if err != nil || j == nil || j.CancelRequestedAt != nil || j.SupersededAt != nil {
					cancel()
					return
				}
			}
		}
	}()

	if err := r.ReconcileInterrupted(ctx, jobID); err != nil {
		return "", err
	}

	for {
		if ctx.Err() != nil || jobCtx.Err() != nil {
			return r.finalStatus(jobID)
		}
		job, err := r.Store.GetJob(jobCtx, jobID)
		if err != nil {
			return "", err
		}
		if job == nil || job.CancelRequestedAt != nil || job.SupersededAt != nil {
			_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneCancelled)
			return domain.CloneCancelled, nil
		}
		if job.PausedAt != nil {
			// Job bị chủ nhân tạm dừng: về pending, nhả lane, chờ resume.
			_ = r.Store.SetJobStatus(ctx, jobID, domain.ClonePending)
			return domain.ClonePending, nil
		}

		claimedAt := time.Now()
		claimed, err := r.Store.ClaimNextStep(jobCtx, jobID, workerID)
		if err != nil {
			return "", err
		}
		if claimed == nil {
			done, status, err := r.settleNoClaim(ctx, jobID, claimedAt)
			if err != nil {
				return "", err
			}
			if done {
				job, _ := r.Store.GetJob(ctx, jobID)
				var waiting *domain.CloneStepRecord
				if status == domain.CloneWaiting {
					steps, _ := r.Store.ListSteps(ctx, jobID)
					for i := range steps {
						if steps[i].Status == domain.CloneWaiting {
							waiting = &steps[i]
							break
						}
					}
				}
				r.humanState(ctx, job, status, waiting)
				return status, nil
			}
			continue
		}

		handler := r.Handlers[claimed.Step.Step]
		if handler == nil {
			cerr := &domain.CloneError{
				Code:    domain.CloneErrInvalidRequest,
				Message: fmt.Sprintf("no handler registered for step %q", claimed.Step.Step),
			}
			_ = r.Store.ReportStepFailure(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, cerr)
			_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneFailed)
			r.humanState(ctx, job, domain.CloneFailed, nil)
			return domain.CloneFailed, nil
		}

		r.runAttempt(jobCtx, job, handler, claimed, workerID, jobID)
		if r.OnAttemptSettled != nil {
			r.OnAttemptSettled(claimed.Step.Step)
		}
		if r.AutoFix != nil {
			if done, _ := r.Store.GetStep(jobCtx, jobID, claimed.Step.Step); done != nil {
				r.AutoFix.AttemptSettled(jobCtx, jobID, claimed.Step.Step, done.Status)
			}
		}

		// Đỗ: đọc lại step — waiting có retryAt không tự retry thì job đỗ
		// và nhả lane (AutoRetryAt=nil). waiting provider/quota/gate → đỗ.
		settled, _ := r.Store.GetStep(jobCtx, jobID, claimed.Step.Step)
		if settled != nil && settled.Status == domain.CloneWaiting && settled.Error != nil {
			if domain.AutoRetryAt(*settled, time.Now()) == nil {
				_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneWaiting)
				r.humanState(ctx, job, domain.CloneWaiting, settled)
				return domain.CloneWaiting, nil
			}
		}
	}
}

// settleNoClaim: không có gì claim được → chain xong, step đỗ, hoặc chờ
// backoff worker-tự-retry trong chỗ (port của nhánh nothing-claimable).
func (r *Runner) settleNoClaim(ctx context.Context, jobID string, claimedAt time.Time) (bool, domain.CloneStatus, error) {
	steps, err := r.Store.ListSteps(ctx, jobID)
	if err != nil {
		return false, "", err
	}
	if domain.ChainDone(steps) {
		_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneSucceeded)
		return true, domain.CloneSucceeded, nil
	}
	var waiting *domain.CloneStepRecord
	for i := range steps {
		if steps[i].Status == domain.CloneWaiting {
			waiting = &steps[i]
			break
		}
	}
	if waiting != nil {
		if waiting.Error != nil && waiting.Error.Code == domain.CloneErrAwaitingReview {
			_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneWaiting)
			return true, domain.CloneWaiting, nil
		}
		if at := domain.AutoRetryAt(*waiting, claimedAt); at != nil {
			// Backoff ngắn do worker tự xếp: ngủ trong chỗ, đọc lại step định
			// kỳ — "Thử lại ngay" của chủ nhân cắt chờ ngay.
			stepName := waiting.Step
			for {
				cur, err := r.Store.GetStep(ctx, jobID, stepName)
				if err != nil || cur == nil || cur.Status != domain.CloneWaiting {
					break
				}
				at := domain.AutoRetryAt(*cur, time.Now())
				if at == nil || !at.After(time.Now()) {
					break
				}
				select {
				case <-ctx.Done():
					return true, domain.CloneStatus(""), nil
				case <-time.After(minDur(5*time.Second, time.Until(*at))):
				}
			}
			return false, "", nil
		}
		_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneWaiting)
		return true, domain.CloneWaiting, nil
	}
	for _, s := range steps {
		if s.Status == domain.CloneFailed {
			_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneFailed)
			return true, domain.CloneFailed, nil
		}
		if s.Status == domain.CloneCancelled {
			_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneCancelled)
			return true, domain.CloneCancelled, nil
		}
	}
	// Rows còn lại mà không claim được: lease đã mất — trả status hiện tại.
	job, _ := r.Store.GetJob(ctx, jobID)
	if job != nil {
		return true, job.Status, nil
	}
	return true, domain.CloneStatus(""), nil
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// runAttempt: một attempt của một step — fingerprint input, dựng StepContext,
// chạy handler, publish artifacts + fenced commit, phân loại lỗi.
func (r *Runner) runAttempt(ctx context.Context, job *domain.CloneJob, handler StepHandler, claimed *ClaimedStep, workerID, jobID string) {
	ctx, stopAttempt := context.WithCancel(ctx)
	defer stopAttempt()
	resolve := r.ResolveInput
	if resolve == nil {
		resolve = defaultInputResolver
	}
	input := resolve(claimed.Step.Step, *job, claimed.Upstream)
	fingerprint := InputFingerprint(input)
	_ = r.Store.WriteStepFingerprint(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, fingerprint)

	stagingDir := r.stagingDir(job, claimed.Step)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		cerr := &domain.CloneError{Code: domain.CloneErrInternal, Message: err.Error()}
		_ = r.Store.ReportStepFailure(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, cerr)
		return
	}

	isPaused := func() bool {
		current, err := r.Store.GetJob(ctx, jobID)
		return err == nil && current != nil && current.PausedAt != nil
	}

	artifactPaths := make(map[string]string)
	artifacts, err := r.Store.ListArtifacts(ctx, jobID)
	if err != nil {
		r.settleAttemptError(ctx, jobID, claimed, err)
		return
	}
	for _, artifact := range artifacts {
		rest := strings.TrimPrefix(artifact.Path, "clone/"+jobID+"/")
		_, relative, found := strings.Cut(rest, "/")
		if found {
			artifactPaths[relative] = artifact.Path
		}
	}
	sctx := &StepContext{
		Job:        *job,
		Step:       claimed.Step,
		StagingDir: stagingDir,
		Upstream:   claimed.Upstream,
		Checkpoint: claimed.Step.Checkpoint,
		Media: &stepMedia{
			base: r.Media, jobID: jobID, upstream: claimed.Upstream, paths: artifactPaths,
			step: claimed.Step.Step,
			record: func(txctx context.Context, artifacts []domain.CloneArtifact) error {
				return r.Store.RecordArtifacts(txctx, jobID, claimed.Step.Step, artifacts)
			},
			fence: func(txctx context.Context) error {
				return r.Store.WriteStepFingerprint(txctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, fingerprint)
			},
		},
		ReportProgress: func(c context.Context, p domain.CloneProgress) error {
			err := r.Store.WriteStepProgress(c, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, p)
			r.progressMu.Lock()
			if r.progressAt == nil {
				r.progressAt = make(map[string]time.Time)
			}
			now := time.Now()
			if err == nil && now.Sub(r.progressAt[jobID]) >= ProgressCoalesce {
				r.publish(domain.CloneEventProgress, map[string]any{"jobId": jobID, "step": claimed.Step.Step, "progress": p})
				r.progressAt[jobID] = now
			}
			r.progressMu.Unlock()
			return err
		},
		WriteCheckpoint: func(c context.Context, b json.RawMessage) error {
			return r.Store.WriteStepCheckpoint(c, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, b)
		},
		ProcessRunner: r.wrapProcessRunner(jobID, claimed),
		ImageSlot: func(c context.Context, run func(context.Context) error) error {
			invoke := func(drawCtx context.Context) error {
				if isPaused() {
					return ErrPaused
				}
				return run(drawCtx)
			}
			if r.ImageLimiter != nil {
				return r.ImageLimiter.WithSlot(c, invoke)
			}
			if r.ImageSlots == nil {
				return invoke(c)
			}
			select {
			case r.ImageSlots <- struct{}{}:
				defer func() { <-r.ImageSlots }()
				return invoke(c)
			case <-c.Done():
				return c.Err()
			}
		},
		IsPaused: isPaused,
	}
	if store, ok := r.Store.(interface {
		DrainImageIntents(context.Context, string, int64, int, json.RawMessage) (json.RawMessage, error)
	}); ok && claimed.Step.Step == domain.CloneImages {
		sctx.DrainImageIntents = func(c context.Context, checkpoint json.RawMessage) (json.RawMessage, error) {
			return store.DrainImageIntents(c, jobID, claimed.Fence, claimed.Step.Attempt, checkpoint)
		}
	}

	result, err := handler.Run(ctx, sctx)
	if err != nil {
		r.settleAttemptError(ctx, jobID, claimed, err)
		return
	}
	err = r.commitResult(ctx, jobID, claimed, result)
	if err != nil {
		r.settleAttemptError(ctx, jobID, claimed, err)
		return
	}
	r.publish(domain.CloneEventUpdated, map[string]any{"jobId": jobID, "step": claimed.Step.Step})
}

// settleAttemptError phân loại lỗi attempt giống bản cũ: ErrPaused → parked
func (r *Runner) settleAttemptError(ctx context.Context, jobID string, claimed *ClaimedStep, err error) {
	if errors.Is(err, ErrPaused) || (ctx.Err() == nil && pausedJob(r, ctx, jobID)) {
		_ = r.Store.ParkStepPaused(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt)
		_ = r.Store.SetJobStatus(ctx, jobID, domain.ClonePending)
		return
	}
	if errors.Is(err, ErrFence) {
		return // attempt khác chiếm fence — không ghi đè
	}
	var cerr *domain.CloneError
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, toolchain.ErrCancelled):
		cerr = &domain.CloneError{Code: domain.CloneErrCancelled, Message: "bước bị huỷ"}
	case errors.As(err, &cerr):
		// đã là CloneError — giữ code/recovery/retryAt
	default:
		cerr = &domain.CloneError{Code: domain.CloneErrInternal, Message: err.Error()}
	}
	if cerr.Code == domain.CloneErrCancelled {
		_ = r.Store.ReportStepCancelled(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt)
		_ = r.Store.SetJobStatus(ctx, jobID, domain.CloneCancelled)
		return
	}
	// Lỗi hỏng hẳn: kênh cho thử lại một lần thì step về pending ngay (không
	// qua failed, không báo "Cần xử lý") và vòng RunJob claim lại; không thì
	// lỗi ở lại cho chủ kênh như thường.
	if r.AutoFix != nil && !domain.CloneErrRetryable(cerr.Code) && r.AutoFix.AutoFix(ctx, jobID, claimed.Step.Step, cerr) {
		return
	}
	_ = r.Store.ReportStepFailure(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, cerr)
}

func pausedJob(r *Runner, ctx context.Context, jobID string) bool {
	j, err := r.Store.GetJob(ctx, jobID)
	return err == nil && j != nil && j.PausedAt != nil
}

// commitResult publishes bytes, artifact rows and the fenced step transition
// in the same Publication transaction. The URL id includes content identity.
func (r *Runner) commitResult(ctx context.Context, jobID string, claimed *ClaimedStep, result StepResult) error {
	output := domain.CloneStepOutput{Artifacts: make([]domain.CloneArtifact, 0, len(result.Artifacts)), Data: result.Data}
	published := make([]media.Artifact, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		file, err := os.Open(artifact.File)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, err := io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		if err := media.ValidatePath(artifact.Path); err != nil {
			return err
		}
		artifact.Path = "clone/" + jobID + "/" + digest + "/" + artifact.Path
		published = append(published, artifact)
		output.Artifacts = append(output.Artifacts, domain.CloneArtifact{
			ID: InputFingerprint(artifact.Path), Path: artifact.Path, SHA256: digest, Bytes: size,
			Step: claimed.Step.Step, ContentType: artifact.ContentType, Filename: artifact.Filename,
		})
	}
	commit := func(txctx context.Context) error {
		if result.Waiting != nil {
			return r.Store.ApplyStepWaiting(txctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, &output, result.Waiting)
		}
		return r.Store.ApplyStepSuccess(txctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, output)
	}
	if len(published) == 0 {
		return commit(ctx)
	}
	return r.Media.Publish(ctx, published, commit)
}

// stagingDir: thư mục ổn định theo job-generation-step — đường output trả phí
// (draw_key) không đổi giữa các lần thử lại.
func (r *Runner) stagingDir(job *domain.CloneJob, step domain.CloneStepRecord) string {
	if r.StagingRoot == "" {
		r.StagingRoot = filepath.Join(os.TempDir(), "viderse-clone-staging")
	}
	return filepath.Join(r.StagingRoot, job.ID, fmt.Sprintf("gen%d", job.Generation), string(step.Step))
}

// wrapProcessRunner: runner process của step — ExecRunner ghi pgid lên row
// (process_group) để recovery dọn nhóm mồ côi.
func (r *Runner) wrapProcessRunner(jobID string, claimed *ClaimedStep) ProcessRunner {
	if r.Runner == nil {
		return nil
	}
	inner := r.Runner
	store := r.Store
	return func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
		if spec.Match == "" {
			spec.Match = jobID
			spec.Label = "clone:" + jobID + ":" + string(claimed.Step.Step)
		}
		originalOnStart := spec.OnStart
		spec.OnStart = func(pid int) {
			_ = store.WriteStepProcessGroup(ctx, jobID, claimed.Step.Step, claimed.Fence, claimed.Step.Attempt, pid)
			if originalOnStart != nil {
				originalOnStart(pid)
			}
		}
		return inner.Run(ctx, file, args, spec)
	}
}

func (r *Runner) finalStatus(jobID string) (domain.CloneStatus, error) {
	j, err := r.Store.GetJob(context.Background(), jobID)
	if err != nil || j == nil {
		return "", err
	}
	return j.Status, nil
}

func (r *Runner) publish(eventType string, data any) {
	if r.Publish != nil {
		r.Publish(eventType, data)
	}
}

// ReconcileInterrupted: sau khi worker chết, step còn 'running' của job mà
// lease đã chết được chuyển waiting:interrupted — resume chạy lại từ
// checkpoint. Port rút gọn của reconcileInterruptedJob (verify publication
// qua media không áp dụng — bytes đi vào blob store nguyên tử).
func (r *Runner) ReconcileInterrupted(ctx context.Context, jobID string) error {
	steps, err := r.Store.ListSteps(ctx, jobID)
	if err != nil {
		return err
	}
	for _, s := range steps {
		if s.Status != domain.CloneRunning {
			continue
		}
		cerr := &domain.CloneError{
			Code:     domain.CloneErrInterrupted,
			Message:  "worker died before the attempt finished",
			Recovery: new("resume the job to retry this step"),
		}
		// Interrupted có retryable=true → waiting, không retryAt → claim
		// ngay khi job được resume/re-run.
		_ = r.Store.ReportStepFailure(ctx, jobID, s.Step, s.Fence, s.Attempt, cerr)
	}
	return nil
}
