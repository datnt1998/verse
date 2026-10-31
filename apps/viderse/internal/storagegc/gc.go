// Package storagegc collects only content-addressed objects in one known app
// store. Preview is the default; apply always rechecks roots under the same
// per-placement lock used by publishers, hydrators and HTTP readers.
package storagegc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

type Repository interface {
	Collect(context.Context, string, blobstore.BlobInfo, time.Time, bool, func(context.Context) error) (bool, error)
}
type Result struct {
	Candidates int                  `json:"candidates"`
	Deleted    int                  `json:"deleted"`
	Kept       int                  `json:"kept"`
	Bytes      int64                `json:"bytes"`
	Objects    []blobstore.BlobInfo `json:"-"`
}
type Service struct {
	Repository Repository
	Media      *media.Service
	Grace      time.Duration
	Now        func() time.Time
}

func (s Service) Run(ctx context.Context, storeID string, apply bool) (Result, error) {
	var out Result
	if s.Grace < time.Hour {
		return out, errors.New("thời gian chờ dọn kho tối thiểu một giờ")
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	cutoff := now.Add(-s.Grace)
	store, d, err := s.Media.Store(ctx, storeID)
	if err != nil {
		return out, err
	}
	if d.Config.Driver != "s3" {
		return out, errors.New("dọn object chỉ áp dụng cho kho S3; không xoá thư mục nguồn")
	}
	cursor := ""
	for {
		page, err := store.List(ctx, "v1/sha256/", cursor, 200)
		if err != nil {
			return out, err
		}
		for _, info := range page.Infos {
			sum := info.Key[strings.LastIndex(info.Key, "/")+1:]
			key, err := media.ContentKey(sum)
			if err != nil || key != info.Key {
				return out, errors.New("kho chứa object ngoài codec đã đăng ký; dừng dọn để bảo toàn dữ liệu")
			}
			if info.ModTime.IsZero() || info.ModTime.After(cutoff) {
				out.Kept++
				continue
			}
			eligible, err := s.Repository.Collect(ctx, storeID, info, cutoff, apply, func(deleteCtx context.Context) error { return store.Delete(deleteCtx, info.Key) })
			if err != nil {
				return out, err
			}
			if !eligible {
				out.Kept++
				continue
			}
			out.Candidates++
			out.Bytes += info.Size
			out.Objects = append(out.Objects, info)
			if apply {
				out.Deleted++
			}
		}
		if page.NextCursor == "" {
			return out, nil
		}
		if page.NextCursor == cursor {
			return out, errors.New("kho trả con trỏ lặp; dừng dọn")
		}
		cursor = page.NextCursor
	}
}

// Apply revisits exactly a previewed set, never a fresh bucket-wide selection.
func (s Service) Apply(ctx context.Context, storeID string, objects []blobstore.BlobInfo) (Result, error) {
	var out Result
	if s.Grace < time.Hour {
		return out, errors.New("thời gian chờ dọn kho tối thiểu một giờ")
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	cutoff := now.Add(-s.Grace)
	store, d, err := s.Media.Store(ctx, storeID)
	if err != nil {
		return out, err
	}
	if d.Config.Driver != "s3" {
		return out, errors.New("không xoá thư mục nguồn")
	}
	for _, info := range objects {
		eligible, err := s.Repository.Collect(ctx, storeID, info, cutoff, true, func(deleteCtx context.Context) error { return store.Delete(deleteCtx, info.Key) })
		if err != nil {
			return out, err
		}
		if !eligible {
			out.Kept++
			continue
		}
		out.Candidates++
		out.Deleted++
		out.Bytes += info.Size
	}
	return out, nil
}
