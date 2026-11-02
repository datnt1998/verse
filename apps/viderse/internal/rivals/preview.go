package rivals

// Chương thật của video gốc cho phần xem trước Khám phá: đọc
// download/source.info.json đã công bố nếu video đã được tải, không thì dò
// metadata YouTube một lần khi người dùng mở xem trước. Không bịa chương và
// không gọi nhà cung cấp thương mại.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

const (
	// maxSourceInfoBytes chặn đọc một tệp info quá lớn (yt-dlp thường < 1 MiB).
	maxSourceInfoBytes = 8 << 20

	// maxChapters chặn phản hồi chương dài bất thường.
	maxChapters = 200

	// previewProbeTTL là hạn dùng lại kết quả dò metadata YouTube cho một
	// video: mở lại xem trước trong hạn này không gọi mạng lần nữa.
	previewProbeTTL = 5 * time.Minute

	// previewCacheMax giới hạn số video giữ kết quả dò trong bộ nhớ.
	previewCacheMax = 256
)

const (
	// codePreviewUnavailable: chưa dò được metadata YouTube (mạng, video
	// riêng tư, máy chủ chưa sẵn sàng).
	codePreviewUnavailable = "preview_unavailable"
	// codeSourceUnreadable: artifact đã công bố nhưng không đọc được — phải
	// báo lỗi thật, không thay bằng dữ liệu dò mới.
	codeSourceUnreadable = "preview_source_unreadable"
)

// ErrArtifactAbsent báo artifact đã công bố nhưng tệp không còn trong kho
// media. Xem trước coi đây là "chưa tải" và dò metadata một lần; mọi lỗi đọc
// khác của artifact là lỗi thật, không được lặng lẽ thay bằng dữ liệu mạng.
var ErrArtifactAbsent = errors.New("artifact nguồn không còn trong kho media")

// errSourceInfoTooLarge báo source.info.json vượt trần đọc.
var errSourceInfoTooLarge = errors.New("source.info.json vượt 8 MiB")

// PreviewDeps là nguồn dữ liệu xem trước do tầng ráp cung cấp. Thiếu deps thì
// Preview trả lỗi cấu hình thật, không giả lập chương.
type PreviewDeps struct {
	// OpenArtifact mở một artifact đã công bố theo đường dẫn logic. Tệp đã
	// mất phải trả ErrArtifactAbsent; lỗi đọc khác là lỗi thật của artifact.
	OpenArtifact func(context.Context, string) (io.ReadCloser, error)
	// ProbeInfo dò metadata nguồn (yt-dlp --skip-download) và trả JSON thô;
	// phải tôn trọng ctx và tự giới hạn thời gian.
	ProbeInfo func(context.Context, string) ([]byte, error)
}

// VideoPreview là chương thật của một video nguồn.
type VideoPreview struct {
	VideoID  string
	Source   string // download | youtube
	State    string // ready | empty
	Chapters []domain.VideoChapter
}

// PreviewError mang câu hiển thị + cách xử lý cho lỗi đọc chương.
type PreviewError struct {
	Code     string
	Message  string
	Recovery string
	// Cause là lỗi đọc gốc: không đưa ra API, chỉ để test/nhật ký đối chiếu.
	Cause error
}

func (e *PreviewError) Error() string { return e.Message }

// Unwrap giữ lỗi gốc để errors.Is/As đối chiếu được nguyên nhân thật.
func (e *PreviewError) Unwrap() error { return e.Cause }

// Preview trả chương của một video đối thủ. channelID là phạm vi đang mở
// (rỗng/AllRivals = mọi đối thủ); video lạ hoặc ngoài phạm vi →
// ErrVideoNotFound.
func (s *Service) Preview(ctx context.Context, channelID, videoID string) (*VideoPreview, error) {
	if !validVideoID(videoID) {
		return nil, ErrVideoNotFound
	}
	actual, found, err := s.store.RivalVideoChannel(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("đọc kênh của video đối thủ: %w", err)
	}
	if !found {
		return nil, ErrVideoNotFound
	}
	if channelID != "" && channelID != domain.AllRivals && channelID != actual {
		return nil, ErrVideoNotFound
	}
	path, found, err := s.store.SourceInfoArtifactPath(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("đọc artifact nguồn của video đối thủ: %w", err)
	}
	if found && s.preview.OpenArtifact != nil {
		chapters, readErr := s.readArtifactChapters(ctx, path)
		switch {
		case readErr == nil:
			out := previewOf("download", chapters)
			out.VideoID = videoID
			return out, nil
		case ctx.Err() != nil:
			// Request đã bị huỷ: trả lỗi huỷ để tầng web không ghi 500.
			return nil, ctx.Err()
		case errors.Is(readErr, ErrArtifactAbsent):
			// Artifact đã công bố nhưng tệp không còn: coi như chưa tải,
			// dò metadata một lần bên dưới.
		case errors.Is(readErr, context.Canceled), errors.Is(readErr, context.DeadlineExceeded):
			return nil, readErr
		default:
			// Artifact có thật nhưng đọc/hỏng: báo lỗi thật kèm cách xử lý,
			// tuyệt đối không lặng lẽ trả dữ liệu dò mới từ YouTube.
			return nil, &PreviewError{
				Code:     codeSourceUnreadable,
				Message:  "Dữ liệu chương đã tải của video gốc không đọc được.",
				Recovery: "Quét lại kênh để làm mới dữ liệu nguồn, rồi mở lại xem trước.",
				Cause:    readErr,
			}
		}
	}
	key := previewCacheKey{rival: actual, video: videoID}
	if cached, ok := s.cache.get(key, s.now()); ok {
		return cached, nil
	}
	if s.preview.ProbeInfo == nil {
		return nil, &PreviewError{
			Code:     codePreviewUnavailable,
			Message:  "Chưa đọc được chương của video gốc.",
			Recovery: "Mở lại Khám phá sau khi máy chủ sẵn sàng.",
		}
	}
	raw, err := s.preview.ProbeInfo(ctx, videoID)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Lượt dò bị huỷ (không phải lỗi nội dung): không ghi 500, và
			// không đệm kết quả dở dang.
			return nil, err
		}
		return nil, &PreviewError{
			Code:     codePreviewUnavailable,
			Message:  "Chưa đọc được chương của video gốc.",
			Recovery: "Thử lại sau ít phút; video riêng tư hoặc đã xoá thì không có chương.",
			Cause:    err,
		}
	}
	chapters, err := parseChapters(bytes.NewReader(raw))
	if err != nil {
		return nil, &PreviewError{
			Code:     codePreviewUnavailable,
			Message:  "Video gốc trả về dữ liệu chương không đọc được.",
			Recovery: "Thử lại sau ít phút.",
			Cause:    err,
		}
	}
	out := previewOf("youtube", chapters)
	out.VideoID = videoID
	s.cache.put(key, out, s.now())
	return out, nil
}

func previewOf(source string, chapters []domain.VideoChapter) *VideoPreview {
	state := "ready"
	if len(chapters) == 0 {
		state = "empty"
	}
	return &VideoPreview{Source: source, State: state, Chapters: chapters}
}

func (s *Service) readArtifactChapters(ctx context.Context, path string) ([]domain.VideoChapter, error) {
	rc, err := s.preview.OpenArtifact(ctx, path)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	// Đọc quá trần một byte để phân biệt "hết tệp" với "tệp quá lớn" thay vì
	// cắt lặng lẽ rồi báo JSON hỏng.
	raw, err := io.ReadAll(io.LimitReader(rc, maxSourceInfoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxSourceInfoBytes {
		return nil, errSourceInfoTooLarge
	}
	return parseChapters(bytes.NewReader(raw))
}

// previewCacheKey gồm cả đối thủ: cùng một mã video đổi chủ sau khi quét lại
// không được trả chương đã đệm cho kênh khác.
type previewCacheKey struct {
	rival string
	video string
}

type previewCacheEntry struct {
	preview *VideoPreview
	expires time.Time
}

// previewCache đệm kết quả dò metadata theo (đối thủ, video) trong
// previewProbeTTL. Chỉ đệm kết quả dò thành công; artifact đã công bố luôn
// được đọc lại vì đó là nguồn sự thật.
type previewCache struct {
	mu      sync.Mutex
	entries map[previewCacheKey]previewCacheEntry
}

func (c *previewCache) get(key previewCacheKey, now time.Time) (*VideoPreview, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return clonePreview(entry.preview), true
}

func (c *previewCache) put(key previewCacheKey, p *VideoPreview, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[previewCacheKey]previewCacheEntry, previewCacheMax)
	}
	if len(c.entries) >= previewCacheMax {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		// Vẫn đầy thì bỏ mục hết hạn sớm nhất — bộ đệm có trần, không phình.
		for len(c.entries) >= previewCacheMax {
			var oldest previewCacheKey
			var oldestAt time.Time
			first := true
			for k, e := range c.entries {
				if first || e.expires.Before(oldestAt) {
					oldest, oldestAt, first = k, e.expires, false
				}
			}
			delete(c.entries, oldest)
		}
	}
	c.entries[key] = previewCacheEntry{preview: clonePreview(p), expires: now.Add(previewProbeTTL)}
}

func clonePreview(p *VideoPreview) *VideoPreview {
	if p == nil {
		return nil
	}
	out := *p
	out.Chapters = append([]domain.VideoChapter(nil), p.Chapters...)
	return &out
}

// sourceInfoChapters là phần source.info.json bước xem trước đọc.
type sourceInfoChapters struct {
	Chapters []struct {
		StartTime *float64 `json:"start_time"`
		Title     string   `json:"title"`
	} `json:"chapters"`
}

// parseChapters đọc danh sách chương từ JSON nguồn, bỏ mục hỏng thay vì bịa.
func parseChapters(raw io.Reader) ([]domain.VideoChapter, error) {
	var info sourceInfoChapters
	if err := json.NewDecoder(raw).Decode(&info); err != nil {
		return nil, err
	}
	out := make([]domain.VideoChapter, 0, len(info.Chapters))
	for _, c := range info.Chapters {
		title := strings.TrimSpace(c.Title)
		if c.StartTime == nil || title == "" || math.IsNaN(*c.StartTime) || math.IsInf(*c.StartTime, 0) || *c.StartTime < 0 {
			continue
		}
		out = append(out, domain.VideoChapter{StartSec: *c.StartTime, Title: title})
		if len(out) == maxChapters {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartSec < out[j].StartSec })
	return out, nil
}
