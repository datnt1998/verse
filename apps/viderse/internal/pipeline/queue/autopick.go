package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Discover is serialized across processes by a session advisory lock. Its
// two owner workers only discover/enqueue; the queue drain alone starts jobs.
func (s *Service) Discover(ctx context.Context) error {
	if s.d.Scan == nil {
		return errors.New("chưa có bộ quét đối thủ cho Tự lấy")
	}
	conn, err := pgx.ConnectConfig(ctx, s.d.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	repo := s.store.With(conn)
	held, err := repo.TryAdvisoryLock(ctx, discoveryLock)
	if err != nil || !held {
		return err
	}
	defer repo.UnlockAdvisory(context.WithoutCancel(ctx), discoveryLock)
	owners, err := s.store.Owners(ctx)
	if err != nil {
		return err
	}
	discoveryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The dedicated pgx connection cannot reconnect transparently. Losing its
	// session invalidates the advisory fence and cancels ongoing scans.
	done := make(chan struct{})
	fenceDone := make(chan struct{})
	defer func() { close(done); <-fenceDone }()
	go func() {
		defer close(fenceDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-discoveryCtx.Done():
				return
			case <-ticker.C:
				if conn.Ping(discoveryCtx) != nil {
					cancel()
					return
				}
			}
		}
	}()
	enabled := make(chan domain.Channel, len(owners))
	for _, o := range owners {
		if o.AutoFetch {
			enabled <- o
		}
	}
	close(enabled)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for o := range enabled {
				if discoveryCtx.Err() != nil {
					return
				}
				if e := s.discoverOwner(discoveryCtx, o); e != nil {
					mu.Lock()
					errs = append(errs, e)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if discoveryCtx.Err() != nil {
		return discoveryCtx.Err()
	}
	return errors.Join(errs...)
}

func (s *Service) discoverOwner(ctx context.Context, owner domain.Channel) error {
	// FK NOT NULL bảo đảm kênh luôn có template mặc định; không còn nhánh
	// "kênh chưa có template".
	rules, err := s.Rules(ctx)
	if err != nil {
		return err
	}
	links, err := s.store.ListSourcesWithChannels(ctx, owner.ID)
	if err != nil {
		return err
	}
	last, err := s.store.LastAutoQueuedChannel(ctx, owner.ID)
	if err != nil {
		return err
	}
	order := []string{}
	candidates := map[string][]string{}
	var lastError error
	for _, link := range links {
		if err = ctx.Err(); err != nil {
			return err
		}
		order = append(order, link.RivalID)
		if link.AutoBaselineAt == nil {
			// First observation establishes a cutoff after scanning: existing/history
			// uploads are not backfilled into automatic work.
			if err = s.d.Scan(ctx, link.RivalID); err != nil {
				lastError = err
				continue
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = s.store.SetSourceBaseline(ctx, owner.ID, link.RivalID, s.d.Now()); err != nil {
				return err
			}
			continue
		}
		if link.RivalScannedAt == nil || s.d.Now().Sub(*link.RivalScannedAt) > time.Duration(rules.RescanHours)*time.Hour {
			if err = s.d.Scan(ctx, link.RivalID); err != nil {
				lastError = err
				continue
			}
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		minDuration := 0
		if link.MinDurationSec != nil {
			minDuration = *link.MinDurationSec
		}
		ids, e := s.store.ListEligibleVideos(ctx, link.RivalID, *link.AutoBaselineAt, owner.ID, minDuration)
		if e != nil {
			lastError = e
			continue
		}
		candidates[link.RivalID] = ids
	}
	var policy scoutPolicy
	if note := owner.AgentInstructions.InstructionFor(domain.AgentScout); strings.TrimSpace(note) != "" {
		policy, err = s.resolveScoutPolicy(ctx, note, links, candidates)
		if err != nil {
			// Chỉ dẫn không hiểu/không hỗ trợ được ghi nhận như một lần quét
			// thất bại thật; không xếp hàng ngầm bỏ qua chỉ dẫn.
			return errors.Join(err, s.recordDiscovery(ctx, owner.ID, 0, "Chưa lấy được video", err))
		}
		if policy.restricts() {
			candidates, err = s.applyScoutPolicy(ctx, policy, links, candidates)
			if err != nil {
				return errors.Join(err, s.recordDiscovery(ctx, owner.ID, 0, "Chưa lấy được video", err))
			}
		}
	}
	ids := InterleaveCandidates(order, candidates, last)
	created := 0
	for offset := 0; offset < len(ids); offset += 200 {
		if err = ctx.Err(); err != nil {
			return err
		}
		urls := make([]string, min(200, len(ids)-offset))
		for i := range urls {
			urls[i] = watchURL(ids[offset+i])
		}
		out, e := s.enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: owner.ID, URLs: urls}, domain.OriginAuto)
		if e != nil {
			lastError = e
			continue
		}
		created += len(out.Items)
	}
	message := "Không có video mới"
	if created > 0 {
		message = fmt.Sprintf("Đã thêm %d video vào hàng chờ", created)
	} else if lastError != nil {
		message = "Chưa lấy được video"
	} else if policy.restricts() {
		message = "Không có video mới khớp chỉ dẫn Trinh sát"
	}
	return s.recordDiscovery(ctx, owner.ID, created, message, lastError)
}
func (s *Service) recordDiscovery(ctx context.Context, id string, created int, message string, err error) error {
	var detail *string
	if err != nil {
		text := err.Error()
		detail = &text
	}
	return s.d.Settings.Set(ctx, "clone.autoFetch.lastRun:"+id, struct {
		At      time.Time `json:"at"`
		Created int       `json:"created"`
		Result  string    `json:"result"`
		Error   *string   `json:"error"`
	}{s.d.Now(), created, message, detail})
}
