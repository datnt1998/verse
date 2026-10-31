package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mediaRoots(ctx context.Context, pool *pgxpool.Pool, current string) ([]string, error) {
	settings := postgres.NewSettingsStore(pool)
	var alt []string
	var previous domain.StoragePrevious
	if _, err := settings.Get(ctx, domain.KeyStorageAltRoots, &alt); err != nil {
		return nil, err
	}
	if _, err := settings.Get(ctx, domain.KeyStoragePrevious, &previous); err != nil {
		return nil, err
	}
	candidates := append([]string{current}, alt...)
	if previous.Path != "" {
		candidates = append(candidates, previous.Path)
	}
	out := []string{}
	seen := map[string]bool{}
	for _, root := range candidates {
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("thư mục dữ liệu đã lưu không hợp lệ")
		}
		root = filepath.Clean(root)
		if !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	return out, nil
}
func mediaRootResolver(roots []string) func(string) (string, error) {
	return func(rel string) (string, error) {
		if err := media.ValidatePath(rel); err != nil {
			return "", err
		}
		for _, root := range roots {
			handle, err := os.OpenRoot(root)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return "", err
			}
			file, err := handle.Open(rel)
			handle.Close()
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return "", err
			}
			info, err := file.Stat()
			file.Close()
			if err != nil {
				return "", err
			}
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("tham chiếu không phải tệp thường: %s", rel)
			}
			return filepath.Join(root, filepath.FromSlash(rel)), nil
		}
		return "", fmt.Errorf("không tìm thấy tệp dữ liệu: %s: %w", rel, os.ErrNotExist)
	}
}
