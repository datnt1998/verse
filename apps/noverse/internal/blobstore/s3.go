package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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
	// PublicBaseURL (S3_PUBLIC_BASE_URL) is the public CDN/base used ONLY for
	// blobs already marked public by the domain. Private blobs always go
	// through authorization + presign/streaming.
	PublicBaseURL string

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

// S3 is the single aws-sdk-go-v2 driver. Multipart upload is internal to Put;
// callers never see the SDK.
type S3 struct {
	cfg     S3Config
	client  *s3.Client
	presign *s3.PresignClient
	upload  *s3manager.Uploader
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
	if cfg.Endpoint != "" {
		u, err := url.Parse(cfg.Endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, fmt.Errorf("S3_ENDPOINT phải là URL http(s) có host: %w", ErrInvalidKey)
		}
		if u.Path != "" && u.Path != "/" {
			return nil, fmt.Errorf("S3_ENDPOINT không được chứa path (chỉ base endpoint): %w", ErrInvalidKey)
		}
		if u.Scheme == "http" && !cfg.AllowPlainHTTP {
			return nil, fmt.Errorf("S3_ENDPOINT http:// chỉ cho mạng riêng/test (MinIO) và phải bật S3_ALLOW_INSECURE_ENDPOINT=true: %w", ErrInvalidKey)
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
		if cfg.HTTPClient != nil {
			o.HTTPClient = cfg.HTTPClient
		}
	})
	uploader := s3manager.NewUploader(client, func(u *s3manager.Uploader) {
		u.PartSize = cfg.PartSize
		u.Concurrency = cfg.Concurrency
		// LeavePartsOnError stays false: a failed or canceled Put aborts the
		// multipart upload so no orphan parts linger in the bucket.
		u.LeavePartsOnError = false
	})
	return &S3{
		cfg:     cfg,
		client:  client,
		presign: s3.NewPresignClient(client),
		upload:  uploader,
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
	// readerOnly hides Seek: the uploader must stream sequentially so the
	// tee-computed SHA-256 counts every byte exactly once, and concurrent
	// ReadAt on shared input cannot double-count.
	h := sha256.New()
	tee := io.TeeReader(readerOnly{capReader(r, s.maxObjectBytes())}, h)
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(key),
		Body:        tee,
		ContentType: aws.String(defaultType(opts.ContentType)),
	}
	if opts.ExpectedSHA256 != "" {
		input.Metadata = map[string]string{"sha256": strings.ToLower(opts.ExpectedSHA256)}
	}
	_, err := s.upload.Upload(ctx, input)
	if err != nil {
		return BlobInfo{}, fmt.Errorf("upload %q: %w", key, err)
	}
	sha := hex.EncodeToString(h.Sum(nil))
	if opts.ExpectedSHA256 != "" && !strings.EqualFold(opts.ExpectedSHA256, sha) {
		// The object was committed before the digest was known; remove it so a
		// corrupted stream never leaves a bad blob under an immutable key.
		_, _ = s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(s.cfg.Bucket),
			Key:    aws.String(key),
		})
		return BlobInfo{}, fmt.Errorf("upload được %s, kỳ vọng %s: %w", sha, opts.ExpectedSHA256, ErrDigestMismatch)
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return BlobInfo{}, fmt.Errorf("verify %q sau upload: %w", key, err)
	}
	return blobInfoFromHead(key, head, sha), nil
}

func (s *S3) Open(ctx context.Context, key string, rng ByteRange) (io.ReadCloser, BlobInfo, error) {
	if err := ValidateKey(key); err != nil {
		return nil, BlobInfo{}, err
	}
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, BlobInfo{}, mapS3Error(err)
	}
	size := headSize(head)
	if _, _, err := resolveRange(rng, size); err != nil {
		return nil, BlobInfo{}, err
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}
	if rng.Offset != 0 || rng.Length >= 0 {
		input.Range = aws.String(rangeHeader(rng, size))
	}
	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, BlobInfo{}, mapS3Error(err)
	}
	return out.Body, blobInfoFromHead(key, head, ""), nil
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

func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration, opts DownloadOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = defaultPresignTTL
	}
	if ttl > s.cfg.MaxPresignTTL {
		ttl = s.cfg.MaxPresignTTL
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}
	if opts.Filename != "" {
		input.ResponseContentDisposition = aws.String(ContentDisposition("attachment", opts.Filename))
	}
	out, err := s.presign.PresignGetObject(ctx, input, func(po *s3.PresignOptions) {
		po.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("presign %q: %w", key, err)
	}
	return out.URL, nil
}

// PublicURL returns the public base URL for a key that the domain has already
// marked public; empty string when no S3_PUBLIC_BASE_URL is configured. Never
// used for private blobs.
func (s *S3) PublicURL(key string) string {
	if s.cfg.PublicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/" + key
}

// Bucket returns the configured bucket name.
func (s *S3) Bucket() string { return s.cfg.Bucket }

// Client exposes the underlying SDK client for admin/test operations that are
// not part of the BlobStore contract (bucket create, ListMultipartUploads).
func (s *S3) Client() *s3.Client { return s.client }

// rangeHeader builds an HTTP Range value for a satisfiable ByteRange.
func rangeHeader(rng ByteRange, size int64) string {
	start, length, _ := resolveRange(rng, size)
	end := start + length - 1
	if length <= 0 {
		end = start // empty tail read: bytes=start-start
	}
	return fmt.Sprintf("bytes=%d-%d", start, end)
}

// readerOnly strips Seeker/ReaderAt so streaming always goes through Read.
type readerOnly struct{ r io.Reader }

func (ro readerOnly) Read(p []byte) (int, error) { return ro.r.Read(p) }

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
