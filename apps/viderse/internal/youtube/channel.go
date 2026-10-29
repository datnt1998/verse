package youtube

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ScannedVideo là một dòng video đọc từ trang kênh — kích thước của một trang
// phía nhà cung cấp, không phải toàn bộ lịch sử.
type ScannedVideo struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	ThumbnailURL  string     `json:"thumbnailUrl"`
	DurationSec   *int64     `json:"durationSec"`
	ViewCount     *int64     `json:"viewCount"`
	PublishedText *string    `json:"publishedText"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

// ChannelProfile là phần đầu kênh (header + metadata) trên trang đầu tiên.
type ChannelProfile struct {
	ID          string  `json:"id"`
	Handle      *string `json:"handle"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatarUrl"`
	// BannerURL là ảnh bìa cắt cho header web; nil khi kênh không có.
	BannerURL       *string `json:"bannerUrl"`
	URL             string  `json:"url"`
	SubscriberCount *int64  `json:"subscriberCount"`
	VideoCount      *int64  `json:"videoCount"`
}

var channelIDRe = regexp.MustCompile(`^UC[\w-]{22}$`)
var videoIDRe = regexp.MustCompile(`^[\w-]{11}$`)

const (
	maxTitleLength         = 300
	maxPublishedTextLength = 100
)

// IsChannelID nhận diện đúng dạng UC + 22 ký tự của channel id.
func IsChannelID(value string) bool {
	return channelIDRe.MatchString(value)
}

var channelPathRe = regexp.MustCompile(`/channel/(UC[\w-]{22})`)
var httpURLRe = regexp.MustCompile(`(?i)^https?://`)

// ResolveChannelID nhận "@handle", channel id hoặc URL kênh youtube.com bất kỳ
// và trả UC id. ID hợp lệ và URL /channel/UC… không tốn request; còn lại qua
// endpoint /navigation/resolve_url của session.
func (c *Client) ResolveChannelID(ctx context.Context, input string) (string, error) {
	raw := strings.TrimSpace(input)
	if channelIDRe.MatchString(raw) {
		return raw, nil
	}
	var url string
	switch {
	case strings.HasPrefix(raw, "@"):
		url = "https://www.youtube.com/" + raw
	case httpURLRe.MatchString(raw):
		url = raw
	default:
		url = "https://" + raw
	}
	if m := channelPathRe.FindStringSubmatch(url); m != nil {
		return m[1], nil
	}
	sess, err := c.session(ctx, "")
	if err != nil {
		return "", err
	}
	resp, err := sess.execute(ctx, "/navigation/resolve_url", jmap{"url": url})
	if err != nil {
		var hs *HTTPStatusError
		// InnerTube trả HTTP 404 cho handle/URL không tồn tại.
		if errors.As(err, &hs) && hs.Status == 404 {
			return "", &ChannelNotFoundError{Message: "Channel not found: " + raw}
		}
		return "", err
	}
	browseID := findBrowseID(jget(resp, "endpoint"))
	if !channelIDRe.MatchString(browseID) {
		return "", &ChannelNotFoundError{Message: "Not a YouTube channel: " + raw}
	}
	return browseID, nil
}

// findBrowseID tìm khoá browseId đầu tiên dưới một endpoint đã parse (payload
// của NavigationEndpoint là object con đầu tiên kết thúc bằng "Endpoint").
func findBrowseID(v any) string {
	m := jobj(v)
	if m == nil {
		return ""
	}
	for _, k := range jsortedKeys(m) {
		if strings.HasSuffix(k, "Endpoint") {
			if id := jstr(jget(m[k], "browseId")); id != "" {
				return id
			}
		}
	}
	return ""
}

// videoFromNode đọc một renderer listing thành dòng metadata. Mọi trường có thể
// null ngoài thực địa, nên phần thiếu thành nil; chỉ video id không dùng được
// mới làm rơi hẳn dòng.
func videoFromNode(node jmap, now time.Time) *ScannedVideo {
	if lv := jobj(node["lockupViewModel"]); lv != nil {
		if jstr(lv["contentType"]) != "LOCKUP_CONTENT_TYPE_VIDEO" {
			return nil
		}
		id := jstr(lv["contentId"])
		if !videoIDRe.MatchString(id) {
			return nil
		}
		var parts []string
		for _, row := range jarr(jget(lv, "metadata", "lockupMetadataViewModel", "metadata", "contentMetadataViewModel", "metadataRows")) {
			for _, p := range jarr(jobj(row)["metadataParts"]) {
				parts = append(parts, textOf(jobj(p)["text"]))
			}
		}
		var publishedText *string
		for _, p := range parts {
			if agoRe.MatchString(p) {
				s := p
				publishedText = &s
				break
			}
		}
		var viewText *string
		for _, p := range parts {
			if publishedText != nil && p == *publishedText {
				continue
			}
			if viewCountTextRe.MatchString(p) {
				s := p
				viewText = &s
				break
			}
		}
		durationText := lockupDurationText(lv)
		return scanned(id, textOf(jget(lv, "metadata", "lockupMetadataViewModel", "title")), durationText, viewText, publishedText, now)
	}
	if vr := jobj(node["videoRenderer"]); vr != nil {
		id := jstr(vr["videoId"])
		if !videoIDRe.MatchString(id) {
			return nil
		}
		var viewText *string
		if s := textOf(vr["viewCountText"]); s != "" {
			viewText = &s
		} else if s := textOf(vr["shortViewCountText"]); s != "" {
			viewText = &s
		}
		var publishedText *string
		if s := textOf(vr["publishedTimeText"]); s != "" {
			publishedText = &s
		}
		return scanned(id, textOf(vr["title"]), videoRendererDuration(vr), viewText, publishedText, now)
	}
	return nil
}

var agoRe = regexp.MustCompile(`(?i)\bago\b`)
var viewCountTextRe = regexp.MustCompile(`(?i)^\s*(\d|no\s)`)

// lockupDurationText tìm badge thời lượng đầu tiên parse được trong các overlay
// của ảnh thu nhỏ lockup.
func lockupDurationText(lv jmap) *string {
	for _, overlay := range jarr(jget(lv, "contentImage", "thumbnailViewModel", "overlays")) {
		om := jobj(overlay)
		for _, key := range []string{"thumbnailBottomOverlayViewModel", "thumbnailOverlayBadgeViewModel"} {
			ov := jobj(om[key])
			if ov == nil {
				continue
			}
			badges := jarr(ov["badges"])
			if len(badges) == 0 {
				badges = jarr(ov["thumbnailBadges"])
			}
			for _, b := range badges {
				text := jstr(jget(b, "thumbnailBadgeViewModel", "text"))
				if _, ok := ParseDuration(text); ok {
					return &text
				}
			}
		}
	}
	return nil
}

// videoRendererDuration đọc lengthText hoặc overlay time-status của videoRenderer cũ.
func videoRendererDuration(vr jmap) *string {
	if s := textOf(vr["lengthText"]); s != "" {
		return &s
	}
	for _, ov := range jarr(vr["thumbnailOverlays"]) {
		if s := textOf(jget(ov, "thumbnailOverlayTimeStatusRenderer", "text")); s != "" {
			return &s
		}
	}
	return nil
}

func scanned(id, title string, durationText, viewText, publishedText *string, now time.Time) *ScannedVideo {
	var published *string
	if publishedText != nil && *publishedText != "" {
		s := truncateRunes(*publishedText, maxPublishedTextLength)
		published = &s
	}
	v := &ScannedVideo{
		ID:            id,
		Title:         truncateRunes(title, maxTitleLength),
		ThumbnailURL:  thumbnailFor(id),
		PublishedText: published,
	}
	if durationText != nil {
		if sec, ok := ParseDuration(*durationText); ok {
			v.DurationSec = &sec
		}
	}
	if viewText != nil {
		if n, ok := ParseCompactNumber(*viewText); ok {
			v.ViewCount = &n
		}
	}
	if published != nil {
		if at, ok := ParseRelativeTime(*published, now); ok {
			v.PublishedAt = &at
		}
	}
	return v
}

// truncateRunes cắt chuỗi theo số ký tự (rune), tránh cắt giữa UTF-8.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// thumbnailFor: 16:9 320×180 không letterbox; URL ổn định, không hết hạn.
func thumbnailFor(videoID string) string {
	return fmt.Sprintf("https://i.ytimg.com/vi/%s/mqdefault.jpg", videoID)
}

// headerTexts trả các dòng chữ của header kênh: ["@handle"],
// ["21.3M subscribers", "540 videos"].
func headerTexts(page jmap) []string {
	var out []string
	for _, row := range jarr(jget(page, "header", "pageHeaderRenderer", "content", "pageHeaderViewModel", "metadata", "contentMetadataViewModel", "metadataRows")) {
		for _, p := range jarr(jobj(row)["metadataParts"]) {
			out = append(out, textOf(jobj(p)["text"]))
		}
	}
	return out
}

// headerBanner là ảnh bìa nhỏ nhất còn đủ rộng header (YouTube xếp lớn nhất trước).
func headerBanner(page jmap) *string {
	sources := jarr(jget(page, "header", "pageHeaderRenderer", "content", "pageHeaderViewModel", "banner", "imageBannerViewModel", "image", "sources"))
	var best *string
	for _, src := range sources {
		s := jobj(src)
		if s == nil {
			continue
		}
		width, _ := s["width"].(float64)
		if width >= 1280 {
			if url := jstr(s["url"]); url != "" {
				u := url
				best = &u // vòng lặp ghi đè → ảnh đủ rộng cuối cùng (nhỏ nhất)
			}
		}
	}
	if best == nil && len(sources) > 0 {
		if url := jstr(jobj(sources[0])["url"]); url != "" {
			best = &url
		}
	}
	return best
}

var subscriberRe = regexp.MustCompile(`(?i)subscriber`)
var videoCountRe = regexp.MustCompile(`(?i)\bvideos?\b`)
var handleRe = regexp.MustCompile(`/(@[^/?#]+)`)

// profileOf đọc hồ sơ kênh từ metadata + header của trang home.
func (c *Client) profileOf(id string, page jmap) *ChannelProfile {
	meta := jobj(jget(page, "metadata", "channelMetadataRenderer"))
	texts := headerTexts(page)
	profile := &ChannelProfile{
		ID:  id,
		URL: "https://www.youtube.com/channel/" + id,
	}
	profile.Title = id
	if meta != nil {
		if t := jstr(meta["title"]); t != "" {
			profile.Title = t
		}
		if d := jstr(meta["description"]); d != "" {
			profile.Description = &d
		}
		if thumbs := jarr(jget(meta, "avatar", "thumbnails")); len(thumbs) > 0 {
			if url := jstr(jobj(thumbs[0])["url"]); url != "" {
				profile.AvatarURL = &url
			}
		}
		if m := handleRe.FindStringSubmatch(jstr(meta["vanityChannelUrl"])); m != nil {
			h := m[1]
			profile.Handle = &h
		}
	}
	if profile.Handle == nil {
		for _, t := range texts {
			if strings.HasPrefix(t, "@") {
				h := t
				profile.Handle = &h
				break
			}
		}
	}
	profile.BannerURL = headerBanner(page)
	for _, t := range texts {
		if profile.SubscriberCount == nil && subscriberRe.MatchString(t) {
			if n, ok := ParseCompactNumber(t); ok {
				profile.SubscriberCount = &n
			}
		}
		if profile.VideoCount == nil && videoCountRe.MatchString(t) {
			if n, ok := ParseCompactNumber(t); ok {
				profile.VideoCount = &n
			}
		}
	}
	return profile
}
