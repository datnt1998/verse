package insight

import (
	"context"
	"strings"
	"time"
)

type Store interface {
	Get(context.Context, string, any) (bool, error)
	Set(context.Context, string, any) error
	ReadRecap(context.Context, time.Time, time.Time) (Recap, error)
}
type Service struct {
	Store    Store
	Now      func() time.Time
	Forecast func(context.Context, time.Time) (int, *time.Time, error)
	Narrate  func(context.Context, Recap) (string, error)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func (s *Service) Recap(ctx context.Context, since *time.Time) (Recap, error) {
	now := s.now()
	from := now
	if since != nil {
		from = *since
	} else {
		_, err := s.Store.Get(ctx, "recap.lastSeenAt", &from)
		if err != nil {
			return Recap{}, err
		}
	}
	recap, err := s.Store.ReadRecap(ctx, from, now)
	if err != nil {
		return Recap{}, err
	}
	if s.Forecast != nil {
		count, at, err := s.Forecast(ctx, now)
		if err != nil {
			return Recap{}, err
		}
		recap.Today.ExpectedDone = &count
		if at != nil {
			text := at.In(time.Local).Format("15:04")
			recap.Today.ByTime = &text
		}
	}
	if s.Narrate != nil && len(recap.NeedsYou) > 0 {
		if text, err := s.Narrate(ctx, recap); err == nil {
			text = strings.TrimSpace(text)
			alternate := "Khi bạn trở lại: " + strings.TrimPrefix(recap.Summary, "Trong lúc bạn vắng: ")
			if text == recap.Summary || text == alternate {
				recap.Summary = text
			}
		}
	}
	return recap, nil
}
func (s *Service) Seen(ctx context.Context) error {
	return s.Store.Set(ctx, "recap.lastSeenAt", s.now())
}
