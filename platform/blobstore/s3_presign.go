package blobstore

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// PresignHead signs HEAD itself; a signed GET cannot be reused for metadata.
func (s *S3) PresignHead(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	out, err := s.presign.PresignHeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = s.EffectivePresignTTL(ttl) })
	if err != nil {
		return "", fmt.Errorf("presign HEAD %q: %w", key, err)
	}
	return out.URL, nil
}

// PresignPut signs an upload to an app-selected key. The caller must send the
// Content-Type, Cache-Control, Content-Disposition and x-amz-meta-sha256 headers
// selected by opts, then independently Verify before publishing a reference.
// This does not enforce immutability or authorize a browser upload by itself.
func (s *S3) PresignPut(ctx context.Context, key string, ttl time.Duration, opts PutOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	input := &s3.PutObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.CacheControl != "" {
		input.CacheControl = aws.String(opts.CacheControl)
	}
	if opts.ContentDisposition != "" {
		input.ContentDisposition = aws.String(opts.ContentDisposition)
	}
	if opts.ExpectedSHA256 != "" {
		input.Metadata = map[string]string{"sha256": opts.ExpectedSHA256}
	}
	out, err := s.presign.PresignPutObject(ctx, input, func(o *s3.PresignOptions) { o.Expires = s.EffectivePresignTTL(ttl) })
	if err != nil {
		return "", fmt.Errorf("presign PUT %q: %w", key, err)
	}
	return out.URL, nil
}
