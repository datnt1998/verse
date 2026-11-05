// Package posted giữ phần mark-posted của video (§21) sau khi V38 bỏ bộ đọc số
// liệu sau đăng: đánh dấu đã đăng, chốt facts metadata (link YouTube + tiêu đề
// + độ dài) và không đọc số liệu. Không có goroutine, không reader, không
// insight.
package posted

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Lỗi công khai của đường mark-posted.
var (
	// ErrNotReady: video chưa xuất xong.
	ErrNotReady = errors.New("Chỉ đánh dấu đã đăng cho video đã xuất xong")
	// ErrNotFound: không có video.
	ErrNotFound = errors.New("Không tìm thấy video")
	// ErrBadURL: link YouTube không hợp lệ.
	ErrBadURL = errors.New("Link YouTube không hợp lệ")
)

// Store là phần bền vững của mark-posted (facts + link YouTube).
type Store interface {
	EnsureFacts(ctx context.Context, jobID, title string, durationSec int) error
	SetYoutube(ctx context.Context, jobID, youtubeID string) error
	YoutubeID(ctx context.Context, jobID string) (string, error)
}

// Jobs đọc video để kiểm trạng thái trước khi đánh dấu.
type Jobs interface {
	Get(ctx context.Context, jobID string) (*domain.CloneJobView, error)
}

// Marker đánh dấu đã đăng qua kho clone (cùng đường với thao tác hàng loạt).
type Marker interface {
	MarkPosted(ctx context.Context, id string, posted bool) error
}

// Service là đường mark-posted sau V38.
type Service struct {
	Store   Store
	Jobs    Jobs
	Marker  Marker
	Publish func(eventType string, data any)
	// Title lấy tiêu đề metadata để chốt facts (nil → bỏ trống).
	Title func(ctx context.Context, jobID string) (string, error)
	Now   func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Post đánh dấu đã đăng, kèm link YouTube tuỳ chọn. Gọi lại với cùng nội dung
// là no-op.
func (s *Service) Post(ctx context.Context, jobID, youtubeURL string) (string, error) {
	youtubeID := ""
	if youtubeURL != "" {
		id, err := ParseVideoURL(youtubeURL)
		if err != nil {
			return "", err
		}
		youtubeID = id
	}
	view, err := s.Jobs.Get(ctx, jobID)
	if err != nil {
		return "", err
	}
	if view == nil {
		return "", ErrNotFound
	}
	if view.PostedAt == nil {
		if view.Status != domain.CloneSucceeded {
			return "", ErrNotReady
		}
		if err = s.Marker.MarkPosted(ctx, jobID, true); err != nil {
			return "", err
		}
	}
	if err = s.ensureFacts(ctx, jobID, durationOf(view.Source.DurationSec)); err != nil {
		return "", err
	}
	if youtubeID != "" {
		if err = s.Store.SetYoutube(ctx, jobID, youtubeID); err != nil {
			return "", err
		}
	} else if youtubeID, err = s.Store.YoutubeID(ctx, jobID); err != nil {
		return "", err
	}
	if s.Publish != nil {
		s.Publish(domain.CloneEventUpdated, map[string]any{"jobId": jobID})
	}
	return youtubeID, nil
}

func durationOf(v *float64) int {
	if v == nil || *v <= 0 {
		return 0
	}
	return int(*v + 0.5)
}

func (s *Service) ensureFacts(ctx context.Context, jobID string, duration int) error {
	title := ""
	if s.Title != nil {
		if t, err := s.Title(ctx, jobID); err == nil {
			title = t
		}
	}
	return s.Store.EnsureFacts(ctx, jobID, title, duration)
}

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseVideoURL lấy mã video từ link YouTube (watch, shorts, live, embed, youtu.be).
func ParseVideoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", ErrBadURL
	}
	var id string
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed") {
			id = parts[1]
		}
	default:
		return "", ErrBadURL
	}
	if !videoIDPattern.MatchString(id) {
		return "", ErrBadURL
	}
	return id, nil
}
