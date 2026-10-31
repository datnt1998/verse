package blobstore

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// OperationMetric is an aggregate since process start. The only dimensions are
// bounded operation, driver and outcome names; no object identity is retained.
type OperationMetric struct {
	Op                 string  `json:"op"`
	Driver             string  `json:"driver"`
	Outcome            string  `json:"outcome"`
	Count              uint64  `json:"count"`
	Bytes              int64   `json:"bytes"`
	DurationSeconds    float64 `json:"durationSeconds"`
	MaxDurationSeconds float64 `json:"maxDurationSeconds"`
}

type metricLabel struct{ op, outcome string }

// MetricsStore instruments the application boundary, not individual SDK retry
// attempts. Open measures until Close and counts bytes actually read.
// A partially consumed stream records only the bytes consumed by the caller.
type MetricsStore struct {
	store   BlobStore
	driver  string
	logger  *slog.Logger
	slow    time.Duration
	mu      sync.Mutex
	metrics map[metricLabel]OperationMetric
}

var _ BlobStore = (*MetricsStore)(nil)

// NewMetrics defaults the slow-operation threshold to one second. It never
// logs keys, URLs, input options or error strings (SDK errors can contain URLs).
func NewMetrics(store BlobStore, logger *slog.Logger, slow time.Duration) BlobStore {
	if logger == nil {
		logger = slog.Default()
	}
	if slow <= 0 {
		slow = time.Second
	}
	driver := DriverName(store)
	if driver != "local" && driver != "s3" {
		driver = "unknown"
	}
	m := &MetricsStore{store: store, driver: driver, logger: logger, slow: slow, metrics: make(map[metricLabel]OperationMetric)}
	batch, hasBatch := store.(BatchDeleter)
	multipart, hasMultipart := store.(MultipartReaper)
	if hasBatch && hasMultipart {
		return &batchMultipartMetrics{batchMetrics: &batchMetrics{MetricsStore: m, batch: batch}, multipart: multipart}
	}
	if hasBatch {
		return &batchMetrics{MetricsStore: m, batch: batch}
	}
	if hasMultipart {
		return &multipartMetrics{MetricsStore: m, multipart: multipart}
	}
	return m
}

// DriverName preserves the underlying persistence capability unchanged; metric
// labels separately normalize unrecognized drivers to "unknown".
func (m *MetricsStore) DriverName() string { return DriverName(m.store) }

// ConditionalPutSupport forwards the observed capability without network I/O.
func (m *MetricsStore) ConditionalPutSupport() string {
	if capability, ok := m.store.(interface{ ConditionalPutSupport() string }); ok {
		return capability.ConditionalPutSupport()
	}
	return "not_applicable"
}

func (m *MetricsStore) EffectivePresignTTL(ttl time.Duration) time.Duration {
	if p, ok := m.store.(interface {
		EffectivePresignTTL(time.Duration) time.Duration
	}); ok {
		return p.EffectivePresignTTL(ttl)
	}
	return EffectivePresignTTL(ttl)
}

type batchMetrics struct {
	*MetricsStore
	batch BatchDeleter
}

func (m *batchMetrics) DeleteMany(ctx context.Context, keys []string) ([]DeleteResult, error) {
	start := time.Now()
	results, err := m.batch.DeleteMany(ctx, keys)
	metricErr := err
	if metricErr == nil {
		for _, result := range results {
			if result.Err != nil {
				metricErr = result.Err
				break
			}
		}
	}
	m.record("delete_many", start, 0, metricErr)
	return results, err
}

type multipartMetrics struct {
	*MetricsStore
	multipart MultipartReaper
}

func (m *multipartMetrics) ListMultipartUploads(ctx context.Context, prefix, cursor string, limit int) (MultipartPage, error) {
	start := time.Now()
	page, err := m.multipart.ListMultipartUploads(ctx, prefix, cursor, limit)
	m.record("list_multipart", start, 0, err)
	return page, err
}
func (m *multipartMetrics) AbortMultipartUpload(ctx context.Context, upload MultipartUpload) error {
	start := time.Now()
	err := m.multipart.AbortMultipartUpload(ctx, upload)
	m.record("abort_multipart", start, 0, err)
	return err
}

type batchMultipartMetrics struct {
	*batchMetrics
	multipart MultipartReaper
}

func (m *batchMultipartMetrics) ListMultipartUploads(ctx context.Context, prefix, cursor string, limit int) (MultipartPage, error) {
	return (&multipartMetrics{MetricsStore: m.MetricsStore, multipart: m.multipart}).ListMultipartUploads(ctx, prefix, cursor, limit)
}
func (m *batchMultipartMetrics) AbortMultipartUpload(ctx context.Context, upload MultipartUpload) error {
	return (&multipartMetrics{MetricsStore: m.MetricsStore, multipart: m.multipart}).AbortMultipartUpload(ctx, upload)
}

// Snapshot returns a detached, deterministically ordered aggregate.
func (m *MetricsStore) Snapshot() []OperationMetric {
	m.mu.Lock()
	result := make([]OperationMetric, 0, len(m.metrics))
	for _, metric := range m.metrics {
		result = append(result, metric)
	}
	m.mu.Unlock()
	sort.Slice(result, func(i, j int) bool {
		if result[i].Op != result[j].Op {
			return result[i].Op < result[j].Op
		}
		return result[i].Outcome < result[j].Outcome
	})
	return result
}

func operationOutcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrPresignUnsupported):
		return "unsupported"
	default:
		return "error"
	}
}

func (m *MetricsStore) record(op string, start time.Time, bytes int64, err error) {
	elapsed := time.Since(start)
	outcome := operationOutcome(err)
	label := metricLabel{op, outcome}
	m.mu.Lock()
	metric := m.metrics[label]
	metric.Op, metric.Driver, metric.Outcome = op, m.driver, outcome
	metric.Count++
	metric.Bytes += bytes
	metric.DurationSeconds += elapsed.Seconds()
	if elapsed.Seconds() > metric.MaxDurationSeconds {
		metric.MaxDurationSeconds = elapsed.Seconds()
	}
	m.metrics[label] = metric
	m.mu.Unlock()
	if elapsed > m.slow {
		attrs := []any{"op", op, "driver", m.driver, "outcome", outcome, "duration", elapsed, "bytes", bytes}
		var requestID interface{ ServiceRequestID() string }
		if errors.As(err, &requestID) {
			// Request IDs are opaque provider identifiers, not URLs or raw errors.
			id := requestID.ServiceRequestID()
			if safeRequestID(id) {
				attrs = append(attrs, "request_id", id)
			}
		}
		m.logger.Warn("slow blob operation", attrs...)
	}
}

func safeRequestID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '/' || c == '+' || c == '=') {
			return false
		}
	}
	return true
}

type metricReader struct {
	io.Reader
	bytes int64
}

func (r *metricReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytes += int64(n)
	return n, err
}

func (m *MetricsStore) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (BlobInfo, error) {
	start := time.Now()
	reader := &metricReader{Reader: r}
	info, err := m.store.Put(ctx, key, reader, opts)
	m.record("put", start, reader.bytes, err)
	return info, err
}

func (m *MetricsStore) Open(ctx context.Context, key string, rng ByteRange) (io.ReadCloser, BlobInfo, error) {
	start := time.Now()
	body, info, err := m.store.Open(ctx, key, rng)
	if err != nil {
		m.record("open", start, 0, err)
		return body, info, err
	}
	return &metricBody{ReadCloser: body, metrics: m, start: start, ctx: ctx}, info, nil
}

type metricBody struct {
	io.ReadCloser
	metrics *MetricsStore
	start   time.Time
	ctx     context.Context
	bytes   int64
	err     error
	once    sync.Once
}

func (b *metricBody) finish() {
	b.once.Do(func() {
		err := b.err
		if err == nil {
			err = b.ctx.Err()
		}
		b.metrics.record("open", b.start, b.bytes, err)
	})
}
func (b *metricBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.bytes += int64(n)
	if err != nil && err != io.EOF {
		b.err = err
	}
	// Close owns finalization so a late close error is not hidden by EOF.
	return n, err
}
func (b *metricBody) Close() error {
	err := b.ReadCloser.Close()
	if b.err == nil {
		b.err = err
	}
	b.finish()
	return err
}

func (m *MetricsStore) Stat(ctx context.Context, key string) (BlobInfo, error) {
	start := time.Now()
	info, err := m.store.Stat(ctx, key)
	m.record("stat", start, 0, err)
	return info, err
}
func (m *MetricsStore) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := m.store.Delete(ctx, key)
	m.record("delete", start, 0, err)
	return err
}
func (m *MetricsStore) List(ctx context.Context, prefix, cursor string, limit int) (ListPage, error) {
	start := time.Now()
	page, err := m.store.List(ctx, prefix, cursor, limit)
	m.record("list", start, 0, err)
	return page, err
}
func (m *MetricsStore) PresignGet(ctx context.Context, key string, ttl time.Duration, opts DownloadOptions) (string, error) {
	start := time.Now()
	url, err := m.store.PresignGet(ctx, key, ttl, opts)
	m.record("presign_get", start, 0, err)
	return url, err
}
