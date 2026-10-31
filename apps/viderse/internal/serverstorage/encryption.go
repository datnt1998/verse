package serverstorage

import (
	"context"
	"errors"
	"os"

	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

func (s *Service) encryptionReadiness(ctx context.Context) (secretbox.Readiness, error) {
	payloads, err := s.opts.Repository.SealedCredentials(ctx)
	if err != nil {
		return secretbox.Readiness{}, err
	}
	if len(payloads) > 0 {
		if _, err := os.Lstat(s.opts.Box.Path()); err != nil {
			issue := "bad_permissions"
			if errors.Is(err, os.ErrNotExist) {
				issue = "corrupt"
			}
			return secretbox.Readiness{Path: s.opts.Box.Path(), Issue: issue}, nil
		}
	}
	readiness := s.opts.Box.Readiness()
	if !readiness.OK {
		return readiness, nil
	}
	for _, payload := range payloads {
		plain, err := s.opts.Box.Reveal(payload)
		if err == nil {
			_, _, err = DecodeCredentials(plain)
		}
		if err != nil {
			readiness.OK = false
			readiness.Issue = "corrupt"
			return readiness, nil
		}
	}
	return readiness, nil
}
