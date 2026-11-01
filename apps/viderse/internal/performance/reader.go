package performance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// Reading là một lần đọc số công khai của một video.
type Reading struct {
	Views       int64
	Likes       *int64
	Comments    *int64
	PublishedAt *time.Time
	Source      Source
}

// Reader đọc số công khai theo mã video YouTube.
type Reader interface {
	Read(ctx context.Context, videoID string) (Reading, error)
}

// ErrVideoUnavailable: video riêng tư, đã xoá hoặc chưa có trên YouTube.
var ErrVideoUnavailable = errors.New("không đọc được video này trên YouTube")

// KeyProvider trả khoá Data API ("" = chưa cấu hình).
type KeyProvider func(ctx context.Context) (string, error)

// DataAPIReader đọc qua YouTube Data API v3 (videos.list, 1 đơn vị hạn mức).
type DataAPIReader struct {
	BaseURL string // mặc định https://www.googleapis.com
	HTTP    *http.Client
}

// KeyRejectedError: Google từ chối khoá.
type KeyRejectedError struct{ Reason string }

func (e *KeyRejectedError) Error() string { return "Google không nhận khoá này" }

type videosResponse struct {
	Items []struct {
		Snippet struct {
			PublishedAt string `json:"publishedAt"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

// ReadWithKey đọc một video bằng khoá cho trước. Khoá không bao giờ vào lỗi trả ra.
func (r *DataAPIReader) ReadWithKey(ctx context.Context, key, videoID string) (Reading, error) {
	base := strings.TrimRight(r.BaseURL, "/")
	if base == "" {
		base = "https://www.googleapis.com"
	}
	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	q := url.Values{"part": {"snippet,statistics"}, "id": {videoID}, "key": {key}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/youtube/v3/videos?"+q.Encode(), nil)
	if err != nil {
		return Reading{}, errors.New("không tạo được yêu cầu Data API")
	}
	resp, err := client.Do(req)
	if err != nil {
		// url.Error mang cả URL (có khoá): chỉ giữ nguyên nhân.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return Reading{}, fmt.Errorf("Data API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Reading{}, fmt.Errorf("Data API: %w", err)
	}
	var out videosResponse
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode != http.StatusOK {
		reason := ""
		if out.Error != nil && len(out.Error.Errors) > 0 {
			reason = out.Error.Errors[0].Reason
		}
		switch reason {
		case "keyInvalid", "API_KEY_INVALID", "ipRefererBlocked", "accessNotConfigured", "forbidden":
			return Reading{}, &KeyRejectedError{Reason: reason}
		}
		if resp.StatusCode == http.StatusBadRequest && out.Error != nil && strings.Contains(strings.ToLower(out.Error.Message), "api key") {
			return Reading{}, &KeyRejectedError{Reason: "keyInvalid"}
		}
		return Reading{}, fmt.Errorf("Data API trả mã %d %s", resp.StatusCode, reason)
	}
	if len(out.Items) == 0 {
		return Reading{}, ErrVideoUnavailable
	}
	item := out.Items[0]
	views, err := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
	if err != nil || views < 0 {
		return Reading{}, ErrVideoUnavailable
	}
	reading := Reading{Views: views, Source: SourceDataAPI}
	if n, e := strconv.ParseInt(item.Statistics.LikeCount, 10, 64); e == nil && n >= 0 {
		reading.Likes = &n
	}
	if n, e := strconv.ParseInt(item.Statistics.CommentCount, 10, 64); e == nil && n >= 0 {
		reading.Comments = &n
	}
	if t, e := time.Parse(time.RFC3339, item.Snippet.PublishedAt); e == nil {
		reading.PublishedAt = &t
	}
	return reading, nil
}

// VideoFetcher là phần của youtube.Client mà bộ đọc trang công khai cần.
type VideoFetcher interface {
	FetchVideoMetadata(ctx context.Context, id string) (youtube.VideoMetadata, error)
}

// PublicPageReader đọc trang công khai qua InnerTube (cùng bộ điều tốc dùng
// chung của youtube.Client). Chỉ có lượt xem.
type PublicPageReader struct{ Client VideoFetcher }

func (r PublicPageReader) Read(ctx context.Context, videoID string) (Reading, error) {
	meta, err := r.Client.FetchVideoMetadata(ctx, videoID)
	if err != nil {
		var unavailable *youtube.VideoUnavailableError
		if errors.As(err, &unavailable) {
			return Reading{}, ErrVideoUnavailable
		}
		return Reading{}, err
	}
	if meta.ViewCount == nil {
		return Reading{}, ErrVideoUnavailable
	}
	return Reading{Views: *meta.ViewCount, PublishedAt: meta.PublishedAt, Source: SourcePublicPage}, nil
}

// AutoReader dùng Data API khi đã cấu hình khoá, không thì (hoặc khi Data API
// lỗi) đọc trang công khai.
type AutoReader struct {
	Key    KeyProvider
	Data   *DataAPIReader
	Public Reader
}

func (a AutoReader) Configured(ctx context.Context) bool {
	if a.Key == nil {
		return false
	}
	key, err := a.Key(ctx)
	return err == nil && key != ""
}

func (a AutoReader) Read(ctx context.Context, videoID string) (Reading, error) {
	var dataErr error
	if a.Key != nil && a.Data != nil {
		key, err := a.Key(ctx)
		if err == nil && key != "" {
			reading, e := a.Data.ReadWithKey(ctx, key, videoID)
			if e == nil {
				return reading, nil
			}
			if errors.Is(e, ErrVideoUnavailable) {
				return Reading{}, e
			}
			dataErr = e
		}
	}
	if a.Public == nil {
		if dataErr != nil {
			return Reading{}, dataErr
		}
		return Reading{}, errors.New("chưa có nguồn đọc số liệu")
	}
	reading, err := a.Public.Read(ctx, videoID)
	if err != nil && dataErr != nil {
		return Reading{}, errors.Join(dataErr, err)
	}
	return reading, err
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
