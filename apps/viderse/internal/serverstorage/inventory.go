package serverstorage

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

// Inventory validates every configured legacy root before any manifest write.
// Pipeline layouts deliberately fail closed until milestone B's real adapters.
func (s *Service) Inventory(ctx context.Context) ([]storageimport.Entry, error) {
	state, err := s.opts.Repository.State(ctx)
	if err != nil {
		return nil, err
	}
	descriptor, err := s.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return nil, err
	}
	if descriptor.Config.Driver != "local" {
		return nil, errors.New("chỉ nhập cây dữ liệu khi kho local đang hoạt động")
	}
	roots, err := s.opts.Repository.Roots(ctx)
	if err != nil {
		return nil, err
	}
	localRoots := s.opts.LocalRoots
	if len(localRoots) == 0 {
		localRoots = []string{descriptor.Config.LocalRoot}
	}
	out := []storageimport.Entry{}
	for _, root := range localRoots {
		entries, err := storageimport.Inventory(ctx, root)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	if err := storageimport.RequireKnown(out, roots); err != nil {
		return nil, err
	}
	return out, nil
}
