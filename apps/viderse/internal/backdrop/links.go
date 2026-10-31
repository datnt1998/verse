package backdrop

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ChannelLink là kênh nền mà một link dán vào chỉ tới.
type ChannelLink struct {
	Platform domain.PlatformID
	Handle   string // login Twitch (chữ thường) hoặc mid Bilibili
	URL      string // địa chỉ gốc của kênh
}

// ID trả mã thư viện "<platform>:<handle>".
func (l ChannelLink) ID() string { return string(l.Platform) + ":" + l.Handle }

// VideoLink là một video cụ thể (VOD, clip Twitch hoặc video Bilibili).
type VideoLink struct {
	Platform domain.PlatformID
	VideoID  string // v<digits> | <clip slug> | BV… | av…
	URL      string
}

// ID trả mã thư viện "<platform>:<videoId>".
func (l VideoLink) ID() string { return string(l.Platform) + ":" + l.VideoID }

var (
	httpScheme         = regexp.MustCompile(`(?i)^https?://`)
	twitchLogin        = regexp.MustCompile(`^[a-z0-9_]{1,25}$`)
	bilibiliSpacePath  = regexp.MustCompile(`^/(\d{1,20})(?:/(?:video|upload/video))?/?$`)
	bilibiliVideoPath  = regexp.MustCompile(`(?i)^/video/(BV[0-9A-Za-z]{10}|av\d+)/?$`)
	b23Path            = regexp.MustCompile(`^/[0-9A-Za-z]+/?$`)
	twitchVodPath      = regexp.MustCompile(`^/(?:[^/]+/)?videos/(\d+)/?$`)
	twitchClipPath     = regexp.MustCompile(`^/[^/]+/clip/([\w-]+)/?$`)
	twitchClipHostPath = regexp.MustCompile(`^/([\w-]+)/?$`)
)

// twitchReserved là các đoạn đường dẫn đầu của twitch.tv là trang của site,
// không phải tên kênh.
var twitchReserved = map[string]bool{
	"videos": true, "directory": true, "settings": true, "p": true,
	"downloads": true, "jobs": true, "store": true, "turbo": true,
	"subscriptions": true, "friends": true, "inventory": true, "wallet": true,
	"drops": true, "prime": true, "search": true, "popout": true, "embed": true,
	"login": true, "signup": true, "u": true, "user": true, "moderator": true,
	"broadcast": true,
}

// twitchVideoTabs là các tab dưới /<login>/videos vẫn thuộc chính kênh đó.
var twitchVideoTabs = map[string]bool{"all": true, "archive": true, "highlights": true, "uploads": true, "collections": true}

const b23Timeout = 10 * time.Second

const (
	invalidLinkMessage = "Dán link kênh Twitch hoặc trang cá nhân Bilibili"
	invalidLinkExample = "Ví dụ: https://www.twitch.tv/<tên kênh> hoặc https://space.bilibili.com/<số>"
	singleVideoMessage = `Link này là một video lẻ — thêm bằng "Thêm clip cố định" trong template, hoặc dán link kênh của người đăng`
)

// unsupportedHosts là các nền tảng người dùng hay dán vào: trả "unsupported"
// kèm tên dễ đọc để mời gửi yêu cầu, không phải link lỗi.
var unsupportedHosts = map[string]string{
	"douyin.com":      "Douyin",
	"iesdouyin.com":   "Douyin",
	"tiktok.com":      "TikTok",
	"kuaishou.com":    "Kuaishou",
	"xiaohongshu.com": "Xiaohongshu",
	"xhslink.com":     "Xiaohongshu",
	"youtube.com":     "YouTube",
	"youtu.be":        "YouTube",
	"instagram.com":   "Instagram",
	"facebook.com":    "Facebook",
	"fb.watch":        "Facebook",
	"x.com":           "X",
	"twitter.com":     "X",
	"vimeo.com":       "Vimeo",
	"youku.com":       "Youku",
	"iqiyi.com":       "iQIYI",
	"nicovideo.jp":    "Niconico",
	"dailymotion.com": "Dailymotion",
	"rumble.com":      "Rumble",
	"odysee.com":      "Odysee",
	"kick.com":        "Kick",
	"trovo.live":      "Trovo",
	"afreecatv.com":   "AfreecaTV",
	"twitcasting.tv":  "TwitCasting",
	"weibo.com":       "Weibo",
	"ixigua.com":      "Xigua",
	"huya.com":        "Huya",
	"douyu.com":       "Douyu",
	"acfun.cn":        "AcFun",
	"streamable.com":  "Streamable",
	"medal.tv":        "Medal",
	"niconico.com":    "Niconico",
	"pornhub.com":     "Pornhub",
	"soundcloud.com":  "SoundCloud",
	"bilibili.tv":     "Bilibili TV",
}

// parseInputURL đọc URL người dùng dán (thiếu scheme thì thêm https://).
func parseInputURL(input string) (*url.URL, bool) {
	value := strings.TrimSpace(input)
	if !httpScheme.MatchString(value) {
		value = "https://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" {
		return nil, false
	}
	return u, true
}

// bareHost bỏ tiền tố www./m. và hạ chữ thường.
func bareHost(u *url.URL) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), "m.")
}

// channelOfURL trả kênh mà một URL Twitch hoặc trang cá nhân Bilibili chỉ tới;
// false với mọi thứ khác (kể cả link một video).
func channelOfURL(u *url.URL) (ChannelLink, bool) {
	switch bareHost(u) {
	case "twitch.tv":
		segments := splitPath(u.Path)
		if len(segments) == 0 {
			return ChannelLink{}, false
		}
		login := strings.ToLower(segments[0])
		if !twitchLogin.MatchString(login) || twitchReserved[login] {
			return ChannelLink{}, false
		}
		// `/videos/all`, `/videos/archive`… vẫn ở kênh; `/videos/<số>` là một
		// video nên bị từ chối. `/about` và `/clips` không nhận thêm đoạn nào.
		if len(segments) > 1 {
			page := strings.ToLower(segments[1])
			switch page {
			case "videos":
				if len(segments) > 2 && !twitchVideoTabs[strings.ToLower(segments[2])] {
					return ChannelLink{}, false
				}
			case "clips", "about":
				if len(segments) > 2 {
					return ChannelLink{}, false
				}
			default:
				return ChannelLink{}, false
			}
		}
		return ChannelLink{Platform: domain.PlatformTwitch, Handle: login, URL: "https://www.twitch.tv/" + login}, true
	case "space.bilibili.com":
		m := bilibiliSpacePath.FindStringSubmatch(u.Path)
		if m == nil {
			return ChannelLink{}, false
		}
		mid := m[1]
		return ChannelLink{Platform: domain.PlatformBilibili, Handle: mid, URL: "https://space.bilibili.com/" + mid}, true
	}
	return ChannelLink{}, false
}

func splitPath(path string) []string {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseAssetLink nhận diện link MỘT video Twitch (VOD hoặc clip) hoặc Bilibili
// (BV/av), chuẩn hoá https. Link kênh, phát trực tiếp và trang space bị từ chối
// vì chúng không phải một video đã kết thúc.
func ParseAssetLink(input string) (VideoLink, bool) {
	u, ok := parseInputURL(input)
	if !ok {
		return VideoLink{}, false
	}
	u.Scheme = "https"
	host := bareHost(u)
	path := u.Path
	switch {
	case host == "bilibili.com" && bilibiliVideoPath.MatchString(path):
		id := bilibiliVideoPath.FindStringSubmatch(path)[1]
		u.Path = "/video/" + id
		return VideoLink{Platform: domain.PlatformBilibili, VideoID: id, URL: u.String()}, true
	case host == "b23.tv" && b23Path.MatchString(path):
		return VideoLink{Platform: domain.PlatformBilibili, VideoID: strings.Trim(path, "/"), URL: u.String()}, true
	case host == "twitch.tv" && twitchVodPath.MatchString(path):
		id := "v" + twitchVodPath.FindStringSubmatch(path)[1]
		return VideoLink{Platform: domain.PlatformTwitch, VideoID: id, URL: u.String()}, true
	case host == "twitch.tv" && twitchClipPath.MatchString(path):
		id := twitchClipPath.FindStringSubmatch(path)[1]
		return VideoLink{Platform: domain.PlatformTwitch, VideoID: id, URL: u.String()}, true
	case host == "clips.twitch.tv" && twitchClipHostPath.MatchString(path):
		id := twitchClipHostPath.FindStringSubmatch(path)[1]
		return VideoLink{Platform: domain.PlatformTwitch, VideoID: id, URL: u.String()}, true
	}
	return VideoLink{}, false
}

// invalidLink là link lỗi, gợi ý phục hồi theo việc nó có phải link một video.
func invalidLink(input string) *Error {
	recovery := invalidLinkExample
	if _, ok := ParseAssetLink(input); ok {
		recovery = singleVideoMessage
	}
	return ErrorfRecovery(CodeInvalidInput, invalidLinkMessage, recovery)
}

// b23Only đúng khi URL là link rút gọn b23.tv chính xác: host "b23.tv"
// (không www/m, không cổng, không userinfo) và scheme http/https.
func b23Only(u *url.URL) bool {
	if u == nil || u.User != nil {
		return false
	}
	if strings.EqualFold(u.Scheme, "http") == false && strings.EqualFold(u.Scheme, "https") == false {
		return false
	}
	return strings.EqualFold(u.Host, "b23.tv")
}

// b23Location là seam đọc Location của link rút gọn. Doer production (bọc nhịp
// Bilibili) hiện thực bằng transport thật của client + CheckRedirect cấm theo
// redirect. Nhánh fallback resolveB23 (doer không có Location) chỉ dành cho
// doer giả trong test — doer đó phải tự đảm bảo một lần Do, không follow.
type b23Location interface {
	Location(ctx context.Context, u *url.URL) (string, error)
}

// roundTripDoer đổi HTTPDoer thành Transport để bọc bằng http.Client an toàn.
type roundTripDoer struct{ doer HTTPDoer }

func (r roundTripDoer) RoundTrip(req *http.Request) (*http.Response, error) { return r.doer.Do(req) }

// transportOf lấy Transport CỦA client: đi qua transport thì request chỉ là
// một lần truy vấn theo đúng nghĩa vụ RoundTripper — redirect chỉ được phép
// nếu CHÍNH client ngoài cho phép (b23: cấm). Trước đây lấy nhầm `client.Do`
// làm transport nên client bên trong tự theo redirect: 301 của b23.tv bị nuốt,
// request thứ hai chạm bilibili.com và nhận 412 → ghi giờ chờ 2 giờ.
func transportOf(doer HTTPDoer) http.RoundTripper {
	if client, ok := doer.(*http.Client); ok {
		if client.Transport != nil {
			return client.Transport
		}
		return http.DefaultTransport
	}
	// Doer khác (chỉ gặp trong test) đóng vai một RoundTripper: một Do một
	// phản hồi, không theo redirect.
	return roundTripDoer{doer: doer}
}

// resolveB23 đọc MỘT lần chuyển hướng của link rút gọn b23.tv: deadline 10
// giây, không theo bất kỳ redirect nào, KHÔNG đọc body (chỉ lấy Location).
func resolveB23(ctx context.Context, doer HTTPDoer, u *url.URL) (*url.URL, error) {
	if !b23Only(u) {
		return nil, ErrorfRecovery(CodeInvalidInput, invalidLinkMessage, invalidLinkExample)
	}
	// Caller không tiêm doer (CLI/test) thì dùng client mặc định, để đường b23
	// không bao giờ chạm RoundTripper nil.
	doer = doerFor(doer, http.DefaultClient)
	var location string
	if reader, ok := doer.(b23Location); ok {
		found, err := reader.Location(ctx, u)
		if err != nil {
			var rl *RateLimitError
			if errors.As(err, &rl) {
				return nil, err
			}
			return nil, ErrorfRecovery(CodeInvalidInput, "Không mở được link rút gọn b23.tv", b23Recovery)
		}
		location = found
	} else {
		reqCtx, cancel := context.WithTimeout(ctx, b23Timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, ErrorfRecovery(CodeInvalidInput, "Không mở được link rút gọn b23.tv", b23Recovery)
		}
		client := &http.Client{
			Timeout:       b23Timeout,
			Transport:     transportOf(doer),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, ErrorfRecovery(CodeInvalidInput, "Không mở được link rút gọn b23.tv", b23Recovery)
		}
		resp.Body.Close() // chỉ đọc header Location, không đọc body
		location = resp.Header.Get("Location")
	}
	if location == "" {
		return nil, nil
	}
	target, err := u.Parse(location)
	if err != nil {
		return nil, nil
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, nil
	}
	return target, nil
}

const b23Recovery = "Mở link đó trong trình duyệt và dán lại địa chỉ trang cá nhân (space.bilibili.com/…)"

// ParseChannelLink đọc link kênh người dùng dán. `b23.tv/<mã>` tốn một lần đọc
// chuyển hướng và chỉ được nhận khi đáp xuống một trang space.
func ParseChannelLink(ctx context.Context, input string, doer HTTPDoer) (ChannelLink, error) {
	u, ok := parseInputURL(input)
	if !ok || (u.Scheme != "https" && u.Scheme != "http") {
		return ChannelLink{}, invalidLink(input)
	}
	if b23Only(u) {
		target, err := resolveB23(ctx, doer, u)
		if err != nil {
			return ChannelLink{}, err
		}
		if target == nil {
			return ChannelLink{}, ErrorfRecovery(CodeInvalidInput, invalidLinkMessage, invalidLinkExample)
		}
		channel, ok := channelOfURL(target)
		if !ok {
			recovery := invalidLinkExample
			if _, isVideo := ParseAssetLink(target.String()); isVideo {
				recovery = singleVideoMessage
			}
			return ChannelLink{}, ErrorfRecovery(CodeInvalidInput, invalidLinkMessage, recovery)
		}
		return channel, nil
	}
	channel, ok := channelOfURL(u)
	if !ok {
		return ChannelLink{}, invalidLink(input)
	}
	return channel, nil
}

// ChannelOfID dựng kênh từ mã thư viện qua đúng phép kiểm của một link dán
// vào, nên mã lấy từ template nhập khẩu không thể thành URL thư viện từ chối.
func ChannelOfID(id string) (ChannelLink, bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || parts[1] == "" {
		return ChannelLink{}, false
	}
	var root string
	switch parts[0] {
	case string(domain.PlatformTwitch):
		root = "https://www.twitch.tv/"
	case string(domain.PlatformBilibili):
		root = "https://space.bilibili.com/"
	default:
		return ChannelLink{}, false
	}
	u, err := url.Parse(root + url.PathEscape(parts[1]))
	if err != nil {
		return ChannelLink{}, false
	}
	channel, ok := channelOfURL(u)
	if !ok || channel.ID() != id {
		return ChannelLink{}, false
	}
	return channel, true
}

// ParseSource phân loại một link dán vào ô thêm nguồn nền: kênh, video lẻ,
// nền tảng chưa hỗ trợ (kèm tên dễ đọc) hoặc link lỗi kèm gợi ý. `b23.tv` tốn
// một lần đọc chuyển hướng; không có lời gọi mạng nào khác.
func ParseSource(ctx context.Context, input string, doer HTTPDoer) (domain.ParsedSource, error) {
	u, ok := parseInputURL(input)
	if !ok || strings.TrimSpace(input) == "" {
		err := invalidLink(input)
		return domain.ParsedSource{Kind: domain.SourceInvalid, Message: err.Message, Recovery: err.Recovery}, nil
	}
	host := bareHost(u)
	if label, unsupported := unsupportedHosts[host]; unsupported {
		return domain.ParsedSource{Kind: domain.SourceUnsupported, Host: host, Label: label}, nil
	}
	if b23Only(u) {
		target, err := resolveB23(ctx, doer, u)
		if err != nil {
			e := asError(err)
			return domain.ParsedSource{Kind: domain.SourceInvalid, Message: e.Message, Recovery: e.Recovery}, nil
		}
		if target == nil {
			return domain.ParsedSource{Kind: domain.SourceInvalid, Message: invalidLinkMessage, Recovery: invalidLinkExample}, nil
		}
		if channel, ok := channelOfURL(target); ok {
			return channelSource(channel), nil
		}
		if video, ok := ParseAssetLink(target.String()); ok {
			return videoSource(video), nil
		}
		return domain.ParsedSource{Kind: domain.SourceInvalid, Message: invalidLinkMessage, Recovery: singleVideoMessage}, nil
	}
	if channel, ok := channelOfURL(u); ok {
		return channelSource(channel), nil
	}
	if video, ok := ParseAssetLink(input); ok {
		return videoSource(video), nil
	}
	err := invalidLink(input)
	return domain.ParsedSource{Kind: domain.SourceInvalid, Message: err.Message, Recovery: err.Recovery}, nil
}

func channelSource(link ChannelLink) domain.ParsedSource {
	return domain.ParsedSource{
		Kind:     domain.SourceChannel,
		Platform: link.Platform,
		ID:       link.ID(),
		Handle:   link.Handle,
		URL:      link.URL,
	}
}

func videoSource(link VideoLink) domain.ParsedSource {
	return domain.ParsedSource{
		Kind:     domain.SourceVideo,
		Platform: link.Platform,
		ID:       link.ID(),
		URL:      link.URL,
	}
}

// doerFor trả doer tiêm hoặc client mặc định.
func doerFor(doer HTTPDoer, fallback HTTPDoer) HTTPDoer {
	if doer != nil {
		return doer
	}
	return fallback
}
