package blobstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

var _ BatchDeleter = (*S3)(nil)
var _ MultipartReaper = (*S3)(nil)

func (s *S3) DeleteMany(ctx context.Context, keys []string) ([]DeleteResult, error) {
	results := make([]DeleteResult, len(keys))
	for i, key := range keys {
		results[i] = DeleteResult{Key: key, Err: ValidateKey(key)}
	}
	for start := 0; start < len(keys); start += DeleteBatchSize {
		end := min(start+DeleteBatchSize, len(keys))
		objects := make([]s3types.ObjectIdentifier, 0, end-start)
		for i := start; i < end; i++ {
			if results[i].Err == nil {
				objects = append(objects, s3types.ObjectIdentifier{Key: aws.String(keys[i])})
			}
		}
		if len(objects) == 0 {
			continue
		}
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(s.cfg.Bucket), Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)}})
		if err != nil {
			for i := start; i < len(results); i++ {
				if results[i].Err == nil {
					results[i].Err = mapS3Error(err)
				}
			}
			return results, mapS3Error(err)
		}
		failures := make(map[string]error, len(out.Errors))
		for _, failure := range out.Errors {
			failures[aws.ToString(failure.Key)] = fmt.Errorf("delete object: %s: %s", aws.ToString(failure.Code), aws.ToString(failure.Message))
		}
		for i := start; i < end; i++ {
			if results[i].Err == nil {
				results[i].Err = failures[keys[i]]
			}
		}
	}
	return results, nil
}

type multipartCursor struct {
	Key      string
	UploadID string
}

func (s *S3) ListMultipartUploads(ctx context.Context, prefix, cursor string, limit int) (MultipartPage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	// MinIO only supports exact object names in the server-side multipart
	// prefix filter. Page the bucket's unfinished uploads on every provider
	// and enforce the requested environment prefix locally instead.
	input := &s3.ListMultipartUploadsInput{Bucket: aws.String(s.cfg.Bucket), MaxUploads: aws.Int32(int32(limit))}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return MultipartPage{}, fmt.Errorf("invalid multipart cursor: %w", err)
		}
		var marker multipartCursor
		if err := json.Unmarshal(raw, &marker); err != nil {
			return MultipartPage{}, fmt.Errorf("invalid multipart cursor: %w", err)
		}
		input.KeyMarker = aws.String(marker.Key)
		input.UploadIdMarker = aws.String(marker.UploadID)
	}
	out, err := s.client.ListMultipartUploads(ctx, input)
	if err != nil {
		return MultipartPage{}, mapS3Error(err)
	}
	page := MultipartPage{Uploads: make([]MultipartUpload, 0, len(out.Uploads))}
	for _, u := range out.Uploads {
		if !strings.HasPrefix(aws.ToString(u.Key), prefix) {
			continue
		}
		page.Uploads = append(page.Uploads, MultipartUpload{Key: aws.ToString(u.Key), UploadID: aws.ToString(u.UploadId), Initiated: aws.ToTime(u.Initiated)})
	}
	if aws.ToBool(out.IsTruncated) {
		if aws.ToString(out.NextKeyMarker) == "" {
			return MultipartPage{}, fmt.Errorf("multipart listing returned no next key marker")
		}
		raw, _ := json.Marshal(multipartCursor{Key: aws.ToString(out.NextKeyMarker), UploadID: aws.ToString(out.NextUploadIdMarker)})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

func (s *S3) AbortMultipartUpload(ctx context.Context, upload MultipartUpload) error {
	if err := ValidateKey(upload.Key); err != nil {
		return err
	}
	if upload.UploadID == "" {
		return fmt.Errorf("missing multipart upload ID")
	}
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(upload.Key), UploadId: aws.String(upload.UploadID)})
	var apiError smithy.APIError
	if errors.As(err, &apiError) && apiError.ErrorCode() == "NoSuchUpload" {
		return nil
	}
	if IsNotFound(mapS3Error(err)) {
		return nil
	}
	return mapS3Error(err)
}
