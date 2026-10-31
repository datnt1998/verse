package serverstorage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"syscall"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
	"github.com/datnt1998/verse/platform/blobstore"
)

type Tasks interface {
	Start(context.Context, domain.TaskSpec) (string, error)
	Update(context.Context, string, domain.TaskUpdate) error
	Finish(context.Context, string) error
}
type Options struct {
	Repository    Repository
	Media         *media.Service
	Box           *secretbox.Box
	Effective     blobstore.Config
	EnvLocked     bool
	Sources       map[string]string
	Resolve       func(string) (string, error)
	LocalRoots    []string
	Hold          func(context.Context, bool) error
	NotifyOffline func(context.Context, string) error
	Changed       func()
	Tasks         Tasks
	Recover       bool
	Release       func()
	ModeChanged   func(bool)
	Watch         func(context.Context) error
}
type testSession struct {
	mu                 sync.Mutex
	actor, fingerprint string
	revision           int64
	expires            time.Time
	config             blobstore.Config
	store              blobstore.BlobStore
	key                string
	result             TestResult
	success            bool
}
type Service struct {
	opts         Options
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	controlMu    sync.Mutex
	workerDone   chan struct{}
	tests        map[string]*testSession
	health       Health
	workerCancel context.CancelFunc
	workerID     string
	closeOnce    sync.Once
	startOnce    sync.Once
}

func New(ctx context.Context, opts Options) (*Service, error) {
	if opts.Repository == nil || opts.Media == nil || opts.Box == nil {
		return nil, errors.New("thiếu phụ thuộc kho máy chủ")
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{opts: opts, ctx: ctx, cancel: cancel, tests: make(map[string]*testSession)}
	health, err := opts.Repository.Health(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	if health != nil {
		s.health = *health
	}
	previous, err := opts.Repository.LatestMigration(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	if opts.Recover && previous != nil && previous.Migration.State == "running" {
		if err := opts.Repository.SetMigrationState(ctx, previous.Migration.TaskID, "running", "paused"); err != nil {
			cancel()
			return nil, err
		}
	}
	return s, nil
}
func (s *Service) Media() *media.Service { return s.opts.Media }

// SetHold is called by composition before Start, never by request handlers.
func (s *Service) SetHold(hold func(context.Context, bool) error) { s.opts.Hold = hold }
func (s *Service) SetStorageWatcher(watch func(context.Context) error, modeChanged func(bool)) {
	s.opts.Watch = watch
	s.opts.ModeChanged = modeChanged
}
func (s *Service) UsesS3(ctx context.Context) (bool, error) {
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return false, err
	}
	d, err := s.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return false, err
	}
	return d.Config.Driver == "s3", nil
}

// BindRuntime attaches application-owned notifications and task reporting.
// Composition calls it before Start or serving requests.
func (s *Service) BindRuntime(tasks Tasks, notify func(context.Context, string) error, changed func()) {
	s.opts.Tasks = tasks
	s.opts.NotifyOffline = notify
	s.opts.Changed = changed
}
func (s *Service) Start() {
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.monitor() }()
		if s.opts.Watch != nil {
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				if err := s.opts.Watch(s.ctx); err != nil && s.ctx.Err() == nil {
					s.logFailure("disk_watcher", err)
				}
			}()
		}
	})
}
func (s *Service) Close(ctx context.Context) error {
	s.cancel()
	s.mu.Lock()
	if s.workerCancel != nil {
		s.workerCancel()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		s.closeOnce.Do(func() {
			if s.opts.Release != nil {
				s.opts.Release()
			}
		})
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func randomID() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}
func (s *Service) changed() {
	if s.opts.Changed != nil {
		s.opts.Changed()
	}
}
func (s *Service) StorageDown() (string, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.health.Online || s.health.Since == nil {
		return "", time.Time{}, false
	}
	return "Không kết nối được kho lưu trữ", *s.health.Since, true
}
func (s *Service) Snapshot(ctx context.Context, admin bool) (Snapshot, error) {
	usage, err := s.opts.Repository.Usage(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	out := Snapshot{Role: "member", Usage: usage}
	if !admin {
		return out, nil
	}
	out.Role = "admin"
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return out, err
	}
	d, err := s.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return out, err
	}
	readiness, err := s.encryptionReadiness(ctx)
	if err != nil {
		return out, err
	}
	cfg := d.Config
	if readiness.OK || len(d.Credentials) == 0 {
		cfg, err = s.effectiveDescriptor(d)
		if err != nil {
			readiness.OK = false
			readiness.Issue = "corrupt"
			cfg = d.Config
		}
	}
	d.Config = cfg
	s.mu.Lock()
	health := s.health
	s.mu.Unlock()
	dto := &Storage{Kind: storeLabel(d).Kind, Delivery: cfg.Delivery, Sources: s.sources(cfg), EncryptionKey: readiness, Online: health.Online, CheckedAt: health.CheckedAt, LastError: health.ErrorCode}
	if cfg.Driver == "s3" {
		dto.Provider = providerWire(cfg.Provider)
		dto.EndpointHost = endpointHost(cfg.S3.Endpoint)
		dto.PresignEndpoint = cfg.S3.PresignEndpoint
		dto.Region = cfg.S3.Region
		dto.Bucket = cfg.S3.Bucket
		dto.ForcePathStyle = &cfg.S3.ForcePathStyle
		dto.AllowInsecure = &cfg.S3.AllowPlainHTTP
		if len(cfg.S3.AccessKeyID) >= 4 {
			dto.AccessKeyLast4 = cfg.S3.AccessKeyID[len(cfg.S3.AccessKeyID)-4:]
		}
	}
	if cfg.Driver == "local" {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(cfg.LocalRoot, &stat); err != nil {
			return out, err
		}
		dto.Disk = &Disk{Path: cfg.LocalRoot, FreeBytes: stat.Bavail * uint64(stat.Bsize)}
	}
	if state.Previous != "" {
		old, err := s.opts.Repository.Descriptor(ctx, state.Previous)
		if err != nil {
			return out, err
		}
		files, bytes, err := s.opts.Repository.PreviousTotals(ctx, state.Previous)
		if err != nil {
			return out, err
		}
		label := storeLabel(old)
		dto.Previous = &Previous{Kind: label.Kind, Label: label.Label, Files: files, Bytes: bytes}
		if old.Config.Driver == "local" {
			dto.Previous.Path = old.Config.LocalRoot
		}
	}
	out.Storage = dto
	used, err := s.opts.Media.Cache().Used()
	if err != nil {
		return out, err
	}
	out.WorkCache = &WorkCache{UsedBytes: used, LimitBytes: s.opts.Media.Cache().Limit()}
	out.Ops = s.operations(cfg.Delivery)
	migration, err := s.opts.Repository.LatestMigration(ctx)
	if err != nil {
		return out, err
	}
	if migration != nil {
		from, err := s.opts.Repository.Descriptor(ctx, migration.FromID)
		if err != nil {
			return out, err
		}
		to, err := s.opts.Repository.Descriptor(ctx, migration.ToID)
		if err != nil {
			return out, err
		}
		migration.Migration.From = storeLabel(from)
		migration.Migration.To = storeLabel(to)
		out.Migration = &migration.Migration
	}
	return out, nil
}
func (s *Service) sources(cfg blobstore.Config) map[string]string {
	out := make(map[string]string)
	for _, key := range []string{"provider", "endpoint", "region", "bucket", "keys", "delivery", "presignEndpoint", "forcePathStyle", "allowInsecure", "cache"} {
		out[key] = "default"
		if cfg.Driver == "s3" && !s.opts.EnvLocked {
			out[key] = "app"
		}
		if value := s.opts.Sources[key]; value != "" {
			out[key] = value
		}
	}
	// §23.1 has no separate kind/configLocked source. Preserve a visible
	// env lock even when only STORAGE_DRIVER or an unknown S3_* name is set.
	if s.opts.EnvLocked {
		out["provider"] = "env"
	}
	return out
}
func (s *Service) effectiveDescriptor(d media.Descriptor) (blobstore.Config, error) {
	cfg := d.Config
	if len(d.Credentials) > 0 {
		plain, err := s.opts.Box.Reveal(d.Credentials)
		if err != nil {
			return cfg, err
		}
		var pair credentials
		if err := pair.decode(plain); err != nil {
			return cfg, err
		}
		cfg.S3.AccessKeyID = pair.AccessKeyID
		cfg.S3.SecretAccessKey = pair.SecretAccessKey
	} else if media.NamespaceID(cfg) == media.NamespaceID(s.opts.Effective) {
		cfg.S3.AccessKeyID = s.opts.Effective.S3.AccessKeyID
		cfg.S3.SecretAccessKey = s.opts.Effective.S3.SecretAccessKey
	}
	return cfg, nil
}
func (s *Service) operations(delivery string) []OpStat {
	result := map[string]*OpStat{}
	for _, op := range []string{"put", "get", "presign", "delete"} {
		result[op] = &OpStat{Op: op}
	}
	sums := map[string]float64{}
	for _, store := range s.opts.Media.Stores() {
		snapshot, ok := store.(interface {
			Snapshot() []blobstore.OperationMetric
		})
		if !ok {
			continue
		}
		for _, metric := range snapshot.Snapshot() {
			op := metric.Op
			if op == "open" {
				op = "get"
			}
			if op == "presign_get" {
				op = "presign"
			}
			stat := result[op]
			if stat == nil {
				continue
			}
			stat.Count += int64(metric.Count)
			if metric.Outcome != "ok" {
				stat.Errors += int64(metric.Count)
			}
			stat.Bytes += metric.Bytes
			sums[op] += metric.DurationSeconds * 1000
			if maximum := metric.MaxDurationSeconds * 1000; maximum > stat.MaxMS {
				stat.MaxMS = maximum
			}
		}
	}
	out := make([]OpStat, 0, 4)
	for _, op := range []string{"put", "get", "presign", "delete"} {
		if op == "presign" && delivery == "proxy" {
			continue
		}
		stat := result[op]
		if stat.Count > 0 {
			stat.AvgMS = sums[op] / float64(stat.Count)
		}
		out = append(out, *stat)
	}
	return out
}
func (s *Service) logFailure(operation string, err error) {
	if err != nil {
		slog.Error("storage operation failed", "operation", operation, "code", errorCode(err))
	}
}
