package backdrop

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// ListedVideo là một dòng danh mục đọc từ nền tảng, trước khi ghi vào thư viện.
type ListedVideo struct {
	VideoID      string
	Kind         domain.PlatformKind
	URL          string
	Title        *string
	DurationSec  *float64
	ThumbnailURL *string
	PublishedAt  *time.Time
}

// ChannelListing là kết quả một lượt đọc danh sách của một kênh.
type ChannelListing struct {
	// Videos xếp mới/đầu tiên trong từng loại; một mã không lặp.
	Videos []ListedVideo
	// VodsComplete đúng khi danh sách VOD trọn: nó dừng trước trần, nên một
	// VOD thiếu trong đó là đã bị xoá.
	VodsComplete bool
}

const (
	// TwitchPoolLimit là trần mỗi nguồn Twitch; trần lịch sử Bilibili là mười
	// trang, đọc mỗi lượt một trang.
	TwitchPoolLimit  = 100
	BilibiliMaxPages = 10
)

const listingTimeout = 90 * time.Second

// ListingArgs là bộ tham số đọc danh sách phẳng của yt-dlp: không tải, không
// thử lại bên trong (pacing quyết định), chỉ đọc một lớp.
var ListingArgs = []string{"--skip-download", "--retries", "0", "--extractor-retries", "0", "--socket-timeout", "20", "--flat-playlist", "-J"}

// flatEntry là một mục trong danh sách phẳng; mục sai hình bị bỏ, không làm
// hỏng cả danh sách.
type flatEntry struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	IEKey     string   `json:"ie_key"`
	Title     *string  `json:"title"`
	Duration  *float64 `json:"duration"`
	Thumbnail *string  `json:"thumbnail"`
	Timestamp *float64 `json:"timestamp"`
}

type flatListing struct {
	Entries []json.RawMessage `json:"entries"`
}

// SecureURL đổi ảnh http sang https: app chỉ phục vụ https.
func SecureURL(url *string) *string {
	if url == nil || *url == "" {
		return nil
	}
	value := *url
	if strings.HasPrefix(strings.ToLower(value), "http://") {
		value = "https://" + value[len("http://"):]
	}
	return &value
}

// twitchFullFrame là đuôi ảnh khung đầy đủ trên CDN Twitch; CDN phục vụ cùng
// khung ở cỡ khác khi đổi đuôi.
const twitchFullFrame = "-1920x1080.jpg"

// twitchThumbnail xin ảnh 640×360 thay vì khung 1920×1080 mà yt-dlp trả về:
// đủ nét cho thẻ lưới ~250 px trên màn 2×, nhẹ hơn ~4 lần (58 KB so với
// 216 KB) khi một trang lưới tải hàng chục thẻ. URL khác giữ nguyên.
func twitchThumbnail(url *string) *string {
	secure := SecureURL(url)
	if secure == nil || !strings.HasSuffix(*secure, twitchFullFrame) {
		return secure
	}
	value := strings.TrimSuffix(*secure, twitchFullFrame) + "-640x360.jpg"
	return &value
}

func parseFlatEntries(stdout string) ([]flatEntry, error) {
	var listing flatListing
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		return nil, &tools.YtDlpError{Message: "yt-dlp trả về danh sách không đọc được", Code: tools.YtDlpFailed}
	}
	out := make([]flatEntry, 0, len(listing.Entries))
	for _, raw := range listing.Entries {
		var entry flatEntry
		if err := json.Unmarshal(raw, &entry); err != nil || entry.ID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

// listPool đọc một nguồn Twitch tối đa limit mục và đổi sang dòng danh mục.
func listPool(ctx context.Context, run RunYtDlpFunc, platform domain.PlatformID, url string, limit int, toVideo func(flatEntry) (ListedVideo, bool)) ([]ListedVideo, error) {
	args := append(append([]string{}, ListingArgs...), "--playlist-end", strconv.Itoa(limit), url)
	stdout, err := run(ctx, platform, args, 1, listingTimeout, nil)
	if err != nil {
		return nil, err
	}
	entries, err := parseFlatEntries(stdout)
	if err != nil {
		return nil, err
	}
	out := make([]ListedVideo, 0, len(entries))
	for _, entry := range entries {
		if video, ok := toVideo(entry); ok {
			out = append(out, video)
		}
	}
	return out, nil
}

var vodIDPattern = regexp.MustCompile(`^v\d+$`)
var clipSlugPattern = regexp.MustCompile(`/clip/([\w-]+)/?(?:[?#].*)?$`)

func vodVideo(kind domain.PlatformKind) func(flatEntry) (ListedVideo, bool) {
	return func(entry flatEntry) (ListedVideo, bool) {
		if !vodIDPattern.MatchString(entry.ID) || entry.URL == "" {
			return ListedVideo{}, false
		}
		return ListedVideo{
			VideoID: entry.ID, Kind: kind, URL: entry.URL,
			Title: entry.Title, DurationSec: entry.Duration, ThumbnailURL: twitchThumbnail(entry.Thumbnail),
		}, true
	}
}

// clipVideo nhận clip theo slug trong URL (id số không dựng lại được địa chỉ clip).
func clipVideo(entry flatEntry) (ListedVideo, bool) {
	if entry.URL == "" {
		return ListedVideo{}, false
	}
	m := clipSlugPattern.FindStringSubmatch(entry.URL)
	if m == nil {
		return ListedVideo{}, false
	}
	video := ListedVideo{
		VideoID: m[1], Kind: domain.PlatformKindClip, URL: entry.URL,
		Title: entry.Title, DurationSec: entry.Duration, ThumbnailURL: twitchThumbnail(entry.Thumbnail),
	}
	if entry.Timestamp != nil {
		at := time.Unix(int64(*entry.Timestamp), 0).UTC()
		video.PublishedAt = &at
	}
	return video, true
}

// dedupe giữ lần xuất hiện đầu của mỗi mã, nên một video nằm ở hai nguồn chỉ
// có một dòng.
func dedupe(videos []ListedVideo) []ListedVideo {
	seen := make(map[string]bool, len(videos))
	out := videos[:0]
	for _, video := range videos {
		if seen[video.VideoID] {
			continue
		}
		seen[video.VideoID] = true
		out = append(out, video)
	}
	return out
}

// ListChannelVideos đọc danh sách của một kênh Twitch: VOD lưu trữ, highlight
// và clip. Một lời gọi hỏng làm hỏng cả lượt — danh sách dở khiến video còn
// sống trông như đã mất.
func ListChannelVideos(ctx context.Context, run RunYtDlpFunc, handle string) (ChannelListing, error) {
	root := "https://www.twitch.tv/" + handle
	platform := domain.PlatformTwitch
	vods, err := listPool(ctx, run, platform, root+"/videos?filter=archives&sort=time", TwitchPoolLimit, vodVideo(domain.PlatformKindVod))
	if err != nil {
		return ChannelListing{}, err
	}
	highlights, err := listPool(ctx, run, platform, root+"/videos?filter=highlights&sort=time", TwitchPoolLimit, vodVideo(domain.PlatformKindHighlight))
	if err != nil {
		return ChannelListing{}, err
	}
	clips, err := listPool(ctx, run, platform, root+"/clips?filter=clips&range=all", TwitchPoolLimit, clipVideo)
	if err != nil {
		return ChannelListing{}, err
	}
	return ChannelListing{
		Videos:       dedupe(append(append(vods, highlights...), clips...)),
		VodsComplete: len(vods) < TwitchPoolLimit,
	}, nil
}
