package serverstorage

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// ImportExisting inventories database-owned references only. Secrets, model
// installations, profiles and paid-image scratch are never recursively scanned.
func (s *Service) ImportExisting(ctx context.Context) error {
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return err
	}
	d, err := s.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return err
	}
	if d.Config.Driver != "local" {
		return nil
	}
	roots, err := s.opts.Repository.Roots(ctx)
	if err != nil {
		return err
	}
	if _, err := s.Inventory(ctx); err != nil {
		return err
	}
	for _, root := range roots {
		state, err := s.opts.Repository.State(ctx)
		if err != nil {
			return err
		}
		if _, err := s.opts.Repository.Resolve(ctx, state.Active, root.Path); err == nil {
			continue
		} else if !errors.Is(err, blobstore.ErrNotFound) {
			return err
		}
		path := filepath.Join(d.Config.LocalRoot, filepath.FromSlash(root.Path))
		if s.opts.Resolve != nil {
			path, err = s.opts.Resolve(root.Path)
			if err != nil {
				return err
			}
		}
		if err := s.opts.Media.Publish(ctx, []media.Artifact{{Path: root.Path, File: path, ContentType: root.ContentType, Filename: filepath.Base(root.Path)}}, nil); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Migrate(ctx context.Context) (string, error) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	current, err := s.opts.Repository.LatestMigration(ctx)
	if err != nil {
		return "", err
	}
	if current != nil && (current.Migration.State == "running" || current.Migration.State == "paused" || current.Migration.State == "error") {
		return current.Migration.TaskID, nil
	}
	if err := s.ImportExisting(ctx); err != nil {
		return "", err
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	if s.opts.Tasks != nil {
		id, err = s.opts.Tasks.Start(ctx, domain.TaskSpec{Kind: domain.TaskStorageMigrate, Title: "Chuyển kho lưu trữ", Object: domain.ObjectRef{Type: "storage", ID: id, Label: "Lưu trữ trên máy chủ"}, Phase: "Đang kiểm kê dữ liệu", Action: domain.TaskAction{Label: "Mở lưu trữ", Href: "/settings?section=storage"}})
		if err != nil {
			return "", err
		}
	}
	record, err := s.opts.Repository.CreateMigration(ctx, id)
	if err != nil {
		if s.opts.Tasks != nil {
			s.logFailure("finish_unstarted_migration", s.opts.Tasks.Finish(ctx, id))
		}
		return "", err
	}
	s.startWorker(record)
	s.changed()
	return id, nil
}
func (s *Service) MigrationAction(ctx context.Context, id, action string) error {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	record, err := s.opts.Repository.LatestMigration(ctx)
	if err != nil {
		return err
	}
	if record == nil || record.Migration.TaskID != id {
		return &Error{404, "Không tìm thấy lượt chuyển kho"}
	}
	from := record.Migration.State
	to := ""
	switch action {
	case "pause":
		if from != "running" {
			return conflict("Lượt chuyển không đang chạy")
		}
		to = "paused"
	case "cancel":
		if from != "running" && from != "paused" && from != "error" {
			return conflict("Lượt chuyển đã kết thúc")
		}
		to = "cancelled"
	case "resume":
		if from != "paused" {
			return conflict("Lượt chuyển không đang tạm dừng")
		}
		to = "running"
	case "retry":
		if from != "error" {
			return conflict("Lượt chuyển không có lỗi cần thử lại")
		}
		to = "running"
	default:
		return &Error{404, "Thao tác không hợp lệ"}
	}
	if err := s.opts.Repository.SetMigrationState(ctx, id, from, to); err != nil {
		return err
	}
	s.mu.Lock()
	stop := s.workerCancel
	done := s.workerDone
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if to == "running" {
		record.Migration.State = to
		s.startWorker(*record)
	}
	if s.opts.Tasks != nil {
		if to == "cancelled" {
			s.logFailure("finish_cancelled_migration", s.opts.Tasks.Finish(ctx, id))
		} else {
			taskState := domain.TaskPaused
			if to == "running" {
				taskState = domain.TaskRunning
			}
			s.logFailure("migration_task_state", s.opts.Tasks.Update(ctx, id, domain.TaskUpdate{State: taskState}))
		}
	}
	s.changed()
	return nil
}
func (s *Service) startWorker(record MigrationRecord) {
	ctx, cancel := context.WithCancel(s.ctx)
	done := make(chan struct{})
	s.mu.Lock()
	s.workerCancel = cancel
	s.workerID = record.Migration.TaskID
	s.workerDone = done
	s.mu.Unlock()
	s.wg.Add(1)
	go func() { defer s.wg.Done(); defer close(done); defer cancel(); s.migrate(ctx, record) }()
}
func (s *Service) migrate(ctx context.Context, record MigrationRecord) {
	target, _, err := s.opts.Media.Store(ctx, record.ToID)
	if err != nil {
		s.migrationFailed(record, err)
		return
	}
	samples := []rateSample{}
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		items, err := s.opts.Repository.ReconcileMigration(ctx, record.Migration.TaskID)
		if err != nil {
			if ctx.Err() == nil {
				s.migrationFailed(record, err)
			}
			return
		}
		progress := Migration{TaskID: record.Migration.TaskID, State: "running", From: record.Migration.From, To: record.Migration.To, StartedAt: record.Migration.StartedAt, Errors: []MigrationError{}, Groups: []MigrationGroup{}}
		var group MigrationGroup
		group.Group = "library"
		group.State = "running"
		for _, item := range items {
			progress.Files.Total++
			progress.Bytes.Total += item.Ref.Bytes
		}
		group.Files.Total = progress.Files.Total
		group.Bytes.Total = progress.Bytes.Total
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return
			}
			if item.State != "verified" {
				err = s.copyItem(ctx, record, target, item.Ref)
				code := ""
				if err != nil {
					code = errorCode(err)
					if errors.Is(err, blobstore.ErrDigestMismatch) {
						code = "content_mismatch"
					}
					if code != "wrong_key" && code != "unreachable_endpoint" && code != "content_mismatch" {
						code = "storage_error"
					}
				}
				if ctx.Err() != nil {
					return
				}
				if saveErr := s.opts.Repository.VerifyMigrationItem(ctx, record.Migration.TaskID, item.Ref, code, ""); saveErr != nil {
					if !errors.Is(saveErr, media.ErrConflict) {
						s.migrationFailed(record, saveErr)
					}
					return
				}
				if err == nil {
					item.State = "verified"
					copied += item.Ref.Bytes
				} else {
					item.State = "error"
					item.ErrorCode = code
				}
			}
			if item.State == "verified" {
				progress.Files.Done++
				progress.Bytes.Done += item.Ref.Bytes
				progress.Checked++
				group.Files.Done++
				group.Bytes.Done += item.Ref.Bytes
			} else {
				progress.Errors = append(progress.Errors, MigrationError{Group: item.Group, Title: filepath.Base(item.Ref.Path), Code: item.ErrorCode})
				group.State = "error"
			}
			samples = append(samples, rateSample{time.Now(), copied})
			samples, progress.RateBPS = rate(samples)
			progress.Groups = []MigrationGroup{group}
			record.Migration = progress
			if err := s.opts.Repository.SaveMigrationProgress(ctx, record); err != nil {
				s.migrationFailed(record, err)
				return
			}
			if s.opts.Tasks != nil {
				phase := "Đang chuyển và xác minh"
				s.logFailure("migration_task_progress", s.opts.Tasks.Update(ctx, record.Migration.TaskID, domain.TaskUpdate{Phase: &phase, Progress: &domain.TaskProgress{Done: progress.Files.Done, Total: progress.Files.Total}, State: domain.TaskRunning}))
			}
			s.changed()
		}
		if group.Files.Done == group.Files.Total {
			group.State = "done"
		}
		progress.Groups = []MigrationGroup{group}
		record.Migration = progress
		if err := s.opts.Repository.SaveMigrationProgress(ctx, record); err != nil {
			s.migrationFailed(record, err)
			return
		}
		if len(progress.Errors) > 0 {
			s.migrationFailed(record, nil)
			return
		}
		done, err := s.opts.Repository.TryCutover(ctx, record.Migration.TaskID)
		if err != nil {
			s.migrationFailed(record, err)
			return
		}
		if done {
			if s.opts.Tasks != nil {
				s.logFailure("finish_migration", s.opts.Tasks.Finish(ctx, record.Migration.TaskID))
			}
			s.checkHealth()
			s.changed()
			return
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func (s *Service) copyItem(ctx context.Context, record MigrationRecord, target blobstore.BlobStore, ref media.Ref) error {
	// Readback is mandatory even for previously copied/cancelled objects. A
	// verified target is reused without another PUT, never trusted by ETag alone.
	if _, err := blobstore.Verify(ctx, target, ref.Key, ref.SHA256, ref.Bytes); err == nil {
		return nil
	} else if !errors.Is(err, blobstore.ErrNotFound) {
		return err
	}
	source, _, resolved, release, err := s.opts.Media.ResolveAt(ctx, record.FromID, ref.Path)
	if err != nil {
		return err
	}
	defer release()
	body, _, err := source.Open(ctx, resolved.Key, blobstore.FullRange)
	if err != nil {
		return err
	}
	_, putErr := target.Put(ctx, ref.Key, body, blobstore.PutOptions{ExpectedSHA256: ref.SHA256, ContentType: ref.ContentType})
	closeErr := body.Close()
	if putErr != nil {
		return putErr
	}
	if closeErr != nil {
		return closeErr
	}
	_, err = blobstore.Verify(ctx, target, ref.Key, ref.SHA256, ref.Bytes)
	return err
}
func (s *Service) migrationFailed(record MigrationRecord, cause error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if cause != nil {
		record.Migration.Errors = append(record.Migration.Errors, MigrationError{Group: "library", Title: "Chuyển kho lưu trữ", Code: "storage_error"})
		s.logFailure("migration", cause)
	}
	s.logFailure("migration_progress", s.opts.Repository.SaveMigrationProgress(ctx, record))
	s.logFailure("migration_state", s.opts.Repository.SetMigrationState(ctx, record.Migration.TaskID, "running", "error"))
	if s.opts.Tasks != nil {
		s.logFailure("migration_task_error", s.opts.Tasks.Update(ctx, record.Migration.TaskID, domain.TaskUpdate{State: domain.TaskFailed}))
	}
	s.changed()
}

type rateSample struct {
	at    time.Time
	bytes int64
}

func rate(samples []rateSample) ([]rateSample, float64) {
	now := samples[len(samples)-1]
	first := 0
	for first < len(samples)-1 && now.at.Sub(samples[first].at) > 30*time.Second {
		first++
	}
	samples = samples[first:]
	span := now.at.Sub(samples[0].at).Seconds()
	if span < 10 {
		return samples, 0
	}
	return samples, float64(now.bytes-samples[0].bytes) / span
}
