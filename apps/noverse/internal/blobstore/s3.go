package blobstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
)

// S3Config is the validated configuration for the single S3-compatible
// driver. Every provider recipe (AWS, R2, GCS interop, MinIO, Wasabi, B2,
// Spaces) is expressed through these fields — there are no per-provider code
// paths.
type S3Config struct {
	// Endpoint is the HTTPS base endpoint (S3_ENDPOINT). Empty resolves to
	// AWS S3 via the SDK default resolver. Plain http:// is only honored for
	// MinIO on private/test networks when AllowPlainHTTP is explicitly set —
	// the driver never silently downgrades TLS.
	Endpoint string
	// PresignEndpoint is the browser-facing signing endpoint, never a host rewrite.
	PresignEndpoint string
	// Region is required for every recipe (S3_REGION): real bucket region for
	// AWS/Wasabi/B2/Spaces, "auto" for R2, "us-east-1" for GCS interop and
	// MinIO defaults.
	Region string
	// Bucket is required (S3_BUCKET).
	Bucket string
	// Static credentials (S3_ACCESS_KEY_ID / S3_SECRET_ACCESS_KEY). GCS uses
	// HMAC keys here, never a service-account JSON key.
	AccessKeyID     string
	SecretAccessKey string
	// ForcePathStyle maps S3_FORCE_PATH_STYLE: required by MinIO and GCS
	// interop, false for AWS/R2/virtual-hosted recipes.
	ForcePathStyle bool
	// AllowPlainHTTP opts in to insecure transport. Only set for
	// private-network/test endpoints (MinIO); never for public hostnames.
	AllowPlainHTTP bool

	// Upload bounds. PartSize >= 5 MiB, Concurrency workers, MaxParts caps a
	// streaming upload (10 000 × PartSize is the provider ceiling).
	PartSize    int64
	Concurrency int
	MaxParts    int
	// MaxPresignTTL bounds signed-URL lifetime; default 15 min.
	MaxPresignTTL time.Duration
	// HTTPClient overrides transport (tests inject corrupting round-trippers).
	HTTPClient *http.Client
}

const (
	defaultS3PartSize    = 8 << 20 // 8 MiB
	defaultS3Concurrency = 4
	defaultMaxParts      = 10000
	defaultPresignTTL    = 60 * time.Second
	defaultMaxPresignTTL = 15 * time.Minute
)

// One connection pool is reused across stores and operations. Contexts bound
// individual operations; a total HTTP timeout would also cut off healthy long
// downloads after their headers have arrived.
var s3HTTPClient = &http.Client{
	// S3 requests are signed for their original host. Never forward them through
	// redirects or an environment proxy that could bypass the destination guard.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 30 * time.Second, KeepAlive: 30 * time.Second,
			Control: storageEndpointControl,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		MaxConnsPerHost:       256,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

var ErrUnsafeEndpoint = errors.New("storage endpoint denied")
var ec2MetadataIPv6 = netip.MustParseAddr("fd00:ec2::254")

// Control runs after DNS resolution, before connect, for every candidate IP.
// Private networks remain supported, but link-local and EC2's IPv6 metadata
// address must never be reached, including through DNS rebinding.
func storageEndpointControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrUnsafeEndpoint
	}
	if zone := strings.LastIndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	ip, err := netip.ParseAddr(host)
	ip = ip.Unmap()
	if err != nil || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip == ec2MetadataIPv6 {
		return ErrUnsafeEndpoint
	}
	return nil
}

// ProbeCleanupError preserves DELETE as the source of a conditional PUT
// capability probe failure, even though the caller is currently performing PUT.
type ProbeCleanupError struct{ Err error }

func (e *ProbeCleanupError) Error() string { return "delete conditional probe: " + e.Err.Error() }
func (e *ProbeCleanupError) Unwrap() error { return e.Err }

// S3 is the single aws-sdk-go-v2 driver. Multipart upload is internal to Put;
// callers never see the SDK.
type S3 struct {
	cfg     S3Config
	client  *s3.Client
	presign *s3.PresignClient
	upload  *s3manager.Uploader
	// A successful PUT does not prove enforcement: probe an existing disposable
	// key twice, once per store. Transport failures leave the capability unknown.
	conditionalState atomic.Uint32
	conditionalProbe chan struct{}
	clockOffset      atomic.Int64
	clockKnown       atomic.Bool
}

const (
	conditionalUnknown uint32 = iota
	conditionalSupported
	conditionalUnsupported
)

// Serialize mutable-key fallback writes across stores in this process. Hosted
// durable objects are content-addressed and DB-published; arbitrary-key copy
// callers must not race a separate process when the provider lacks conditions.
var s3FallbackLocks [64]sync.Mutex

// ConditionalPutSupport is a non-probing snapshot for storage health/metrics.
func (s *S3) ConditionalPutSupport() string {
	switch s.conditionalState.Load() {
	case conditionalSupported:
		return "supported"
	case conditionalUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

var _ BlobStore = (*S3)(nil)

// DriverName is the registry label stored in blobs.storage_driver.
func (s *S3) DriverName() string { return "s3" }

// NewS3 validates the config and builds the client with explicit static
// credentials — no ambient env/IMDS credential chain.
func NewS3(cfg S3Config) (*S3, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("S3_REGION bắt buộc (auto cho R2, us-east-1 cho GCS/MinIO): %w", ErrInvalidKey)
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET bắt buộc: %w", ErrInvalidKey)
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY bắt buộc: %w", ErrInvalidKey)
	}
	for _, endpoint := range []string{cfg.Endpoint, cfg.PresignEndpoint} {
		if endpoint == "" {
			continue
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, fmt.Errorf("địa chỉ kho phải là URL http(s) có host: %w", ErrInvalidKey)
		}
		if (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(endpoint, "#") {
			return nil, fmt.Errorf("địa chỉ kho không được chứa path, userinfo, query hoặc fragment: %w", ErrInvalidKey)
		}
		if u.Scheme == "http" && !cfg.AllowPlainHTTP {
			return nil, fmt.Errorf("địa chỉ kho http:// chỉ dùng khi đã cho phép mạng nội bộ: %w", ErrInvalidKey)
		}
	}
	if cfg.PartSize == 0 {
		cfg.PartSize = defaultS3PartSize
	}
	if cfg.PartSize < s3manager.MinUploadPartSize {
		return nil, fmt.Errorf("part size tối thiểu 5 MiB: %w", ErrInvalidKey)
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultS3Concurrency
	}
	if cfg.MaxParts <= 0 {
		cfg.MaxParts = defaultMaxParts
	}
	if cfg.MaxParts > defaultMaxParts || cfg.PartSize > (math.MaxInt64-1)/int64(cfg.MaxParts) {
		return nil, fmt.Errorf("giới hạn multipart không hợp lệ: %w", ErrInvalidKey)
	}
	if cfg.MaxPresignTTL <= 0 {
		cfg.MaxPresignTTL = defaultMaxPresignTTL
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
		// Compatible endpoints (GCS interop, R2, MinIO…) do not implement the
		// full CRC32 trailer family; WHEN_REQUIRED keeps integrity on
		// signatures that support it without breaking ones that don't.
		// Content SHA-256 is always computed and verified independently.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		o.HTTPClient = s3HTTPClient
		if cfg.HTTPClient != nil {
			o.HTTPClient = cfg.HTTPClient
		}
	})
	uploader := s3manager.NewUploader(client, func(u *s3manager.Uploader) {
		u.PartSize = cfg.PartSize
		u.Concurrency = cfg.Concurrency
		u.MaxUploadParts = int32(cfg.MaxParts)
		u.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		// LeavePartsOnError stays false: a failed or canceled Put aborts the
		// multipart upload so no orphan parts linger in the bucket.
		u.LeavePartsOnError = false
	})
	signClient := client
	if cfg.PresignEndpoint != "" {
		signClient = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.PresignEndpoint)
			o.UsePathStyle = cfg.ForcePathStyle
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
			o.HTTPClient = s3HTTPClient
		})
	}
	return &S3{
		cfg:              cfg,
		client:           client,
		presign:          s3.NewPresignClient(signClient),
		upload:           uploader,
		conditionalProbe: make(chan struct{}, 1),
	}, nil
}

// maxObjectBytes is the streaming upload ceiling: MaxParts × PartSize.
func (s *S3) maxObjectBytes() int64 {
	return int64(s.cfg.MaxParts) * s.cfg.PartSize
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return BlobInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}
	// Verify before touching the final key. The spool is private, bounded by
	// the configured upload ceiling, and removed on every return path. The
	// caller is read and hashed once; the verified file is seekable so SDK
	// multipart retries do not require whole-part heap buffers.
	spool, err := os.CreateTemp("", "noverse-s3-put-*")
	if err != nil {
		return BlobInfo{}, fmt.Errorf("tạo tệp tạm upload: %w", err)
	}
	defer os.Remove(spool.Name())
	defer spool.Close()
	sum := newWriteSum(spool)
	n, err := io.Copy(sum, io.LimitReader(s3ContextReader{ctx: ctx, r: r}, s.maxObjectBytes()+1))
	if err != nil {
		return BlobInfo{}, fmt.Errorf("đọc nguồn blob: %w", err)
	}
	if n > s.maxObjectBytes() {
		return BlobInfo{}, ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}
	sha := sum.Digest()
	if opts.ExpectedSHA256 != "" && !strings.EqualFold(opts.ExpectedSHA256, sha) {
		return BlobInfo{}, fmt.Errorf("upload được %s, kỳ vọng %s: %w", sha, opts.ExpectedSHA256, ErrDigestMismatch)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return BlobInfo{}, fmt.Errorf("đọc lại tệp upload: %w", err)
	}
	cacheControl := opts.CacheControl
	if cacheControl == "" {
		cacheControl = "private, no-store"
	}
	disposition := opts.ContentDisposition
	if disposition == "" {
		disposition = "inline"
	}
	input := &s3.PutObjectInput{
		Bucket:             aws.String(s.cfg.Bucket),
		Key:                aws.String(key),
		Body:               spool,
		ContentLength:      aws.Int64(n),
		ContentType:        aws.String(defaultType(opts.ContentType)),
		CacheControl:       aws.String(cacheControl),
		ContentDisposition: aws.String(disposition),
		Metadata:           map[string]string{"sha256": sha},
	}
	// Garage v2.5.0 ignores conditions on multipart completion. Always preflight
	// multipart, independently of whether ordinary PutObject conditions work.
	fallback := n > s.cfg.PartSize
	var etag *string
	if !fallback {
		input.IfNoneMatch = aws.String("*")
		out, unsupported, err := s.putConditional(ctx, input)
		fallback = unsupported
		if !fallback {
			if s3Status(err, http.StatusPreconditionFailed, "PreconditionFailed") {
				return s.identicalBlob(ctx, key, sha, n)
			}
			if err != nil {
				return BlobInfo{}, fmt.Errorf("upload %q: %w", key, err)
			}
			etag = out.ETag
		}
	}
	if fallback {
		// Verified content-addressed keys can only be overwritten with identical
		// bytes. Non-addressed keys need a lock spanning HEAD and upload; CAS after
		// upload cannot protect their bytes from a competing writer in this process.
		object, parseErr := blobkey.Parse(key)
		if parseErr != nil || object.SHA256 == "" || object.SHA256 != sha {
			var hash uint64 = 14695981039346656037
			for i := range key {
				hash = (hash ^ uint64(key[i])) * 1099511628211
			}
			lock := &s3FallbackLocks[hash%uint64(len(s3FallbackLocks))]
			lock.Lock()
			defer lock.Unlock()
		}
		existing, err := s.identicalBlob(ctx, key, sha, n)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return BlobInfo{}, fmt.Errorf("kiểm tra %q trước upload: %w", key, err)
		}
		// A rejected conditional attempt may have consumed part of the file.
		if _, err := spool.Seek(0, io.SeekStart); err != nil {
			return BlobInfo{}, fmt.Errorf("đọc lại tệp upload: %w", err)
		}
		input.IfNoneMatch = nil
		out, err := s.upload.Upload(ctx, input)
		if err != nil {
			return BlobInfo{}, fmt.Errorf("upload %q: %w", key, err)
		}
		etag = out.ETag
	}
	return BlobInfo{
		Key: key, Size: n, ContentType: defaultType(opts.ContentType), SHA256: sha,
		ETag: strings.Trim(aws.ToString(etag), `"`), ModTime: time.Now().UTC(),
	}, nil
}

// putConditional probes against a disposable blobkey-owned staging key, never
// against caller data. Only a rejected second write proves enforcement.
func (s *S3) putConditional(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectOutput, bool, error) {
	if s.conditionalState.Load() == conditionalUnknown {
		select {
		case s.conditionalProbe <- struct{}{}:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
		if s.conditionalState.Load() == conditionalUnknown {
			state, err := s.probeConditional(ctx, aws.ToString(input.Key))
			if err != nil {
				<-s.conditionalProbe
				return nil, false, err
			}
			s.conditionalState.Store(state)
		}
		<-s.conditionalProbe
	}
	if s.conditionalState.Load() == conditionalUnsupported {
		return nil, true, nil
	}
	out, err := s.client.PutObject(ctx, input)
	if s3Status(err, http.StatusNotImplemented, "NotImplemented") {
		s.conditionalState.Store(conditionalUnsupported)
		return nil, true, nil
	}
	return out, false, err
}

func (s *S3) probeConditional(ctx context.Context, callerKey string) (state uint32, err error) {
	// Follow the caller's environment, not a hard-coded dev prefix: a production
	// key may only be permitted to write prod/. Legacy arbitrary keys use env.
	env, envErr := blobkey.ParseEnv(strings.SplitN(callerKey, "/", 2)[0])
	if envErr != nil {
		env, err = blobkey.EnvFromOS()
		if err != nil {
			return conditionalUnknown, err
		}
	}
	keys, err := blobkey.New(env)
	if err != nil {
		return conditionalUnknown, err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return conditionalUnknown, err
	}
	id := fmt.Sprintf("%x-%x-%x-%x-%x", token[:4], token[4:6], token[6:8], token[8:10], token[10:])
	key, err := keys.Staging(id)
	if err != nil {
		return conditionalUnknown, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if deleteErr := s.Delete(cleanup, key); deleteErr != nil {
			state, err = conditionalUnknown, errors.Join(err, &ProbeCleanupError{Err: deleteErr})
		}
	}()
	probe := &s3.PutObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), Body: strings.NewReader("conditional-write-probe")}
	if _, err = s.client.PutObject(ctx, probe); err != nil {
		return conditionalUnknown, err
	}
	probe.Body = strings.NewReader("conditional-write-probe")
	probe.IfNoneMatch = aws.String("*")
	_, err = s.client.PutObject(ctx, probe)
	switch {
	case s3Status(err, http.StatusPreconditionFailed, "PreconditionFailed"):
		return conditionalSupported, nil
	case err == nil, s3Status(err, http.StatusNotImplemented, "NotImplemented"):
		return conditionalUnsupported, nil
	default:
		return conditionalUnknown, err
	}
}

func (s *S3) identicalBlob(ctx context.Context, key, sha string, size int64) (BlobInfo, error) {
	info, err := s.Stat(ctx, key)
	if err != nil {
		return BlobInfo{}, err
	}
	if info.SHA256 != sha || info.Size != size {
		return BlobInfo{}, fmt.Errorf("key đã chứa nội dung khác: %w", ErrDigestMismatch)
	}
	return info, nil
}

func s3Status(err error, status int, code string) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == code {
		return true
	}
	var responseErr interface{ HTTPStatusCode() int }
	return errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == status
}

func (s *S3) Open(ctx context.Context, key string, rng ByteRange) (io.ReadCloser, BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return nil, BlobInfo{}, err
	}
	if rng.Offset < 0 {
		return nil, BlobInfo{}, ErrInvalidRange
	}
	// HTTP ranges cannot represent an empty window. Only that uncommon case
	// needs metadata without a GET; normal and partial reads are one GET.
	if rng.Length == 0 {
		info, err := s.Stat(ctx, key)
		if err != nil {
			return nil, BlobInfo{}, err
		}
		if _, _, err := resolveRange(rng, info.Size); err != nil {
			return nil, BlobInfo{}, err
		}
		return io.NopCloser(strings.NewReader("")), info, nil
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}
	if rng.Offset != 0 || rng.Length > 0 {
		value := fmt.Sprintf("bytes=%d-", rng.Offset)
		if rng.Length > 0 && rng.Length-1 <= math.MaxInt64-rng.Offset {
			value += strconv.FormatInt(rng.Offset+rng.Length-1, 10)
		}
		input.Range = aws.String(value)
	}
	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		mapped := mapS3Error(err)
		if errors.Is(mapped, ErrInvalidRange) && rng.Offset == 0 {
			// S3 rejects any Range on an empty object, whereas the BlobStore
			// contract permits offset zero. Do not penalize ordinary reads
			// with an unconditional HEAD for this edge case.
			info, statErr := s.Stat(ctx, key)
			if statErr != nil {
				return nil, BlobInfo{}, statErr
			}
			if info.Size == 0 {
				return io.NopCloser(strings.NewReader("")), info, nil
			}
		}
		return nil, BlobInfo{}, mapped
	}
	s.recordClock(out.ResultMetadata)
	size := aws.ToInt64(out.ContentLength)
	if out.ContentRange != nil {
		_, total, ok := strings.Cut(aws.ToString(out.ContentRange), "/")
		if !ok {
			out.Body.Close()
			return nil, BlobInfo{}, fmt.Errorf("Content-Range S3 không hợp lệ")
		}
		size, err = strconv.ParseInt(total, 10, 64)
		if err != nil || size < 0 {
			out.Body.Close()
			return nil, BlobInfo{}, fmt.Errorf("Content-Range S3 không có kích thước hợp lệ")
		}
	}
	if _, _, err := resolveRange(rng, size); err != nil {
		out.Body.Close()
		return nil, BlobInfo{}, err
	}
	return out.Body, BlobInfo{
		Key: key, Size: size, ContentType: aws.ToString(out.ContentType),
		SHA256: out.Metadata["sha256"], ETag: strings.Trim(aws.ToString(out.ETag), `"`),
		ModTime: aws.ToTime(out.LastModified),
	}, nil
}

func (s *S3) Stat(ctx context.Context, key string) (BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return BlobInfo{}, err
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return BlobInfo{}, mapS3Error(err)
	}
	return blobInfoFromHead(key, head, ""), nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return mapS3Error(err)
	}
	return nil // S3 delete is idempotent: absent key is success
}

func (s *S3) List(ctx context.Context, prefix, cursor string, limit int) (ListPage, error) {
	if limit <= 0 {
		limit = localListLimit
	}
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.Bucket),
		MaxKeys: aws.Int32(int32(limit)),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if cursor != "" {
		// The cursor is the last key of the previous page — stable and
		// resumable across processes, unlike a raw ContinuationToken.
		input.StartAfter = aws.String(cursor)
	}
	out, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return ListPage{}, mapS3Error(err)
	}
	page := ListPage{}
	for _, obj := range out.Contents {
		page.Infos = append(page.Infos, BlobInfo{
			Key:     aws.ToString(obj.Key),
			Size:    aws.ToInt64(obj.Size),
			ETag:    strings.Trim(aws.ToString(obj.ETag), `"`),
			ModTime: aws.ToTime(obj.LastModified),
		})
	}
	if aws.ToBool(out.IsTruncated) && len(page.Infos) > 0 {
		page.NextCursor = page.Infos[len(page.Infos)-1].Key
	}
	return page, nil
}

// EffectivePresignTTL applies the default S3 signing lifetime and maximum.
func EffectivePresignTTL(ttl time.Duration) time.Duration {
	return effectivePresignTTL(ttl, defaultMaxPresignTTL)
}

// EffectivePresignTTL exposes the actual lifetime, including this store's cap,
// so delivery metadata agrees with the SDK URL.
func (s *S3) EffectivePresignTTL(ttl time.Duration) time.Duration {
	return effectivePresignTTL(ttl, s.cfg.MaxPresignTTL)
}

func effectivePresignTTL(ttl, maximum time.Duration) time.Duration {
	if ttl <= 0 {
		ttl = defaultPresignTTL
	}
	if ttl > maximum {
		ttl = maximum
	}
	return ttl
}

func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration, opts DownloadOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	ttl = s.EffectivePresignTTL(ttl)
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}
	disposition := opts.Disposition
	if disposition != "" && disposition != "inline" && disposition != "attachment" {
		return "", fmt.Errorf("disposition không hợp lệ: %w", ErrInvalidKey)
	}
	if opts.Filename != "" {
		if disposition == "" {
			disposition = "attachment"
		}
		input.ResponseContentDisposition = aws.String(ContentDisposition(disposition, opts.Filename))
	} else if disposition != "" {
		input.ResponseContentDisposition = aws.String(disposition)
	}
	out, err := s.presign.PresignGetObject(ctx, input, func(po *s3.PresignOptions) {
		po.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("presign %q: %w", key, err)
	}
	return out.URL, nil
}

// Bucket returns the configured bucket name.
func (s *S3) Bucket() string { return s.cfg.Bucket }

// Client exposes the underlying SDK client for admin/test operations that are
// not part of the BlobStore contract (bucket create, ListMultipartUploads).
func (s *S3) Client() *s3.Client { return s.client }

// The source need not implement cancellation itself; check the context at
// every spool read. A blocked caller-owned Reader must still unblock itself.
type s3ContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r s3ContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func headSize(h *s3.HeadObjectOutput) int64 {
	if h.ContentLength == nil {
		return 0
	}
	return *h.ContentLength
}

func blobInfoFromHead(key string, h *s3.HeadObjectOutput, sha string) BlobInfo {
	if sha == "" {
		sha = h.Metadata["sha256"]
	}
	return BlobInfo{
		Key:         key,
		Size:        headSize(h),
		ContentType: aws.ToString(h.ContentType),
		SHA256:      sha,
		ETag:        strings.Trim(aws.ToString(h.ETag), `"`),
		ModTime:     aws.ToTime(h.LastModified),
	}
}

// mapS3Error normalizes SDK/service errors onto the contract sentinels.
func mapS3Error(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return fmt.Errorf("%w: %s", ErrNotFound, apiErr.ErrorMessage())
		case "InvalidRange", "RequestedRangeNotSatisfiable", "416":
			return fmt.Errorf("%w: %s", ErrInvalidRange, apiErr.ErrorMessage())
		}
	}
	var nf *s3types.NotFound
	if errors.As(err, &nf) {
		return ErrNotFound
	}
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nsk) {
		return ErrNotFound
	}
	return err
}
