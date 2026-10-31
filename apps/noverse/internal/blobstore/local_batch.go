package blobstore

import "context"

var _ BatchDeleter = (*Local)(nil)

func (l *Local) DeleteMany(ctx context.Context, keys []string) ([]DeleteResult, error) {
	return deleteSequential(ctx, l, keys), nil
}
