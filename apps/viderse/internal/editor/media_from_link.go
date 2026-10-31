package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const MaxLinkClipSec = 600

type AssetLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

var bilibiliVideoPath = regexp.MustCompile(`(?i)^/video/(BV[0-9A-Za-z]{10}|av\d+)/?$`)
var shortAssetPath = regexp.MustCompile(`^/[0-9A-Za-z]+/?$`)
var twitchVideoPath = regexp.MustCompile(`^/(videos/\d+|\w+/clip/[\w-]+)/?$`)
var twitchClipPath = regexp.MustCompile(`^/[\w-]+/?$`)
var linkHTTPScheme = regexp.MustCompile(`(?i)^https?://`)

func ParseAssetLink(input string) *AssetLink {
	value := strings.TrimSpace(input)
	if !linkHTTPScheme.MatchString(value) {
		value = "https://" + value
	}
	u, e := url.Parse(value)
	if e != nil || u.User != nil || u.Host == "" || u.Opaque != "" {
		return nil
	}
	u.Scheme = "https"
	host := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), "m.")
	platform := ""
	switch {
	case host == "bilibili.com" && bilibiliVideoPath.MatchString(u.Path), host == "b23.tv" && shortAssetPath.MatchString(u.Path):
		platform = "bilibili"
	case host == "twitch.tv" && twitchVideoPath.MatchString(u.Path), host == "clips.twitch.tv" && twitchClipPath.MatchString(u.Path):
		platform = "twitch"
	}
	if platform == "" || u.Port() != "" && u.Port() != "443" {
		return nil
	}
	return &AssetLink{Platform: platform, URL: u.String()}
}

type LinkCut struct {
	StartSec float64 `json:"startSec"`
	EndSec   float64 `json:"endSec"`
}

var linkClock = regexp.MustCompile(`^\d+:\d{1,2}(:\d{1,2})?$`)

func parseLinkClock(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !linkClock.MatchString(s) {
		return 0, false
	}
	parts := strings.Split(s, ":")
	var n float64
	for i, p := range parts {
		v, e := strconv.ParseFloat(p, 64)
		if e != nil || i > 0 && v >= 60 {
			return 0, false
		}
		n = n*60 + v
	}
	return n, true
}
func ResolveCut(from, to string, duration *float64) (*LinkCut, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" && to == "" {
		if duration != nil && *duration > MaxLinkClipSec {
			return nil, apiError("invalid_request", fmt.Sprintf("Video dài %d phút — nhập đoạn Từ/Đến (tối đa 10 phút)", int(math.Round(*duration/60))))
		}
		return nil, nil
	}
	start := 0.0
	end := 0.0
	var ok bool
	if from != "" {
		start, ok = parseLinkClock(from)
		if !ok {
			return nil, apiError("invalid_request", "Thời điểm phải có dạng phút:giây, ví dụ 1:05 hoặc 1:02:03")
		}
	}
	if to == "" {
		if duration == nil {
			return nil, apiError("invalid_request", "Nhập thời điểm Đến")
		}
		end = *duration
	} else {
		end, ok = parseLinkClock(to)
		if !ok {
			return nil, apiError("invalid_request", "Thời điểm phải có dạng phút:giây, ví dụ 1:05 hoặc 1:02:03")
		}
	}
	if end <= start {
		return nil, apiError("invalid_request", "Thời điểm Đến phải sau Từ")
	}
	if duration != nil && start >= *duration {
		return nil, apiError("invalid_request", "Thời điểm Từ vượt quá độ dài video")
	}
	if end-start > MaxLinkClipSec {
		return nil, apiError("invalid_request", "Đoạn cắt tối đa 10 phút")
	}
	if duration != nil {
		end = math.Min(end, *duration)
	}
	return &LinkCut{StartSec: start, EndSec: end}, nil
}

type LinkFormat struct {
	URL        string   `json:"url"`
	VideoCodec *string  `json:"vcodec"`
	AudioCodec *string  `json:"acodec"`
	Height     *float64 `json:"height"`
	TBR        *float64 `json:"tbr"`
}
type LinkInfo struct {
	Title       string       `json:"title"`
	Duration    *float64     `json:"duration"`
	Uploader    *string      `json:"uploader"`
	Channel     *string      `json:"channel"`
	UploaderURL *string      `json:"uploader_url"`
	LiveStatus  *string      `json:"live_status"`
	Formats     []LinkFormat `json:"formats"`
}
type FetchLinkMediaOptions struct {
	URL, From, To, ProjectID string
	Overlay                  bool
	Info                     *LinkInfo
}
type LinkImportDeps struct {
	FetchText func(context.Context, string) (string, error)
}

func ytDlpMediaError(err error) error {
	var y *tools.YtDlpError
	if !errors.As(err, &y) {
		var p *tools.ProcessFailedError
		if errors.As(err, &p) {
			y = tools.ClassifyYtDlpError(p.Stderr, p.Stderr)
		} else {
			return err
		}
	}
	extra := Object{"ytDlpCode": string(y.Code), "detail": y.Message}
	switch y.Code {
	case tools.YtDlpLoginRequired:
		return apiError("invalid_request", "Video này cần đăng nhập để xem (ví dụ chỉ dành cho người đăng ký) — chọn video khác", extra)
	case tools.YtDlpUnavailable:
		return apiError("not_found", "Video không còn, bị ẩn hoặc bị chặn theo vùng", extra)
	case tools.YtDlpRateLimited:
		return apiError("internal_error", "Bilibili đang tạm chặn yêu cầu — đợi một lúc rồi thử lại", extra)
	default:
		return apiError("internal_error", "Không tải được video. Vào Cài đặt cập nhật yt-dlp rồi thử lại; Bilibili có thể đang tạm chặn, đợi một lúc", extra)
	}
}
func (s *Service) runMediaYTDLP(ctx context.Context, args []string, timeout time.Duration) (string, error) {
	r := s.Runner
	if r == nil {
		r = &tools.ExecRunner{}
	}
	binary := s.YTDLP
	if binary == "" {
		binary = "yt-dlp"
	}
	result, e := r.Run(ctx, binary, args, tools.Spec{Timeout: timeout, LogLimit: 16 << 20})
	if e != nil {
		return "", ytDlpMediaError(e)
	}
	return result.Stdout, nil
}
func (s *Service) ProbeLinkInfo(ctx context.Context, link string) (LinkInfo, error) {
	var info LinkInfo
	raw, e := s.runMediaYTDLP(ctx, []string{"-J", "--skip-download", "--no-playlist", link}, 5*time.Minute)
	if e != nil {
		return info, e
	}
	if e = json.Unmarshal([]byte(raw), &info); e != nil {
		return info, apiError("internal_error", "Invalid yt-dlp video metadata")
	}
	if info.Duration != nil && (math.IsNaN(*info.Duration) || math.IsInf(*info.Duration, 0) || *info.Duration < 0) {
		return info, apiError("internal_error", "Invalid video duration")
	}
	return info, nil
}
func linkFileClock(sec float64) string {
	n := int(sec)
	if n >= 3600 {
		return fmt.Sprintf("%d.%02d.%02d", n/3600, n/60%60, n%60)
	}
	return fmt.Sprintf("%d.%02d", n/60, n%60)
}

var linkTitleInvalid = regexp.MustCompile(`[\\/:*?"<>|]+`)

func (s *Service) FetchLinkMedia(ctx context.Context, opts FetchLinkMediaOptions, deps *LinkImportDeps) (MediaRecord, bool, error) {
	link := ParseAssetLink(opts.URL)
	if link == nil {
		return MediaRecord{}, false, apiError("invalid_request", "Dán link một video Twitch (video phát lại hoặc clip) hoặc Bilibili — không phải link kênh hay phát trực tiếp")
	}
	if opts.ProjectID != "" {
		if _, _, e := s.Store.Read(ctx, opts.ProjectID); e != nil {
			return MediaRecord{}, false, e
		}
	}
	ffmpeg := s.FFmpeg
	if ffmpeg == "" {
		return MediaRecord{}, false, apiError("internal_error", "ffmpeg chưa sẵn sàng — cài trong Cài đặt → Công cụ")
	}
	tmp, e := os.MkdirTemp(s.StagingDir, "editor-link-")
	if e != nil {
		return MediaRecord{}, false, e
	}
	defer os.RemoveAll(tmp)
	var info LinkInfo
	if opts.Info != nil {
		info = *opts.Info
	} else {
		info, e = s.ProbeLinkInfo(ctx, link.URL)
		if e != nil {
			return MediaRecord{}, false, e
		}
	}
	if info.LiveStatus != nil && (*info.LiveStatus == "is_live" || *info.LiveStatus == "is_upcoming") {
		return MediaRecord{}, false, apiError("invalid_request", "Buổi phát trực tiếp chưa kết thúc — đợi có video phát lại")
	}
	duration := info.Duration
	if duration != nil {
		duration = mediaNumber(math.Round(*duration))
	}
	cut, e := ResolveCut(opts.From, opts.To, duration)
	if e != nil {
		return MediaRecord{}, false, e
	}
	file := ""
	if cut != nil {
		file, e = s.fetchHLSCut(ctx, info, *cut, tmp, ffmpeg, deps)
		if e != nil {
			return MediaRecord{}, false, e
		}
	}
	if file == "" {
		height := 1080
		if link.Platform == "twitch" {
			height = 480
		}
		args := []string{"-f", "bv*+ba/b", "-S", fmt.Sprintf("res:%d,vcodec:h264,acodec:aac", height), "--merge-output-format", "mp4", "--concurrent-fragments", "8", "--ffmpeg-location", filepath.Dir(ffmpeg)}
		if cut != nil {
			args = append(args, "--download-sections", fmt.Sprintf("*%g-%g", cut.StartSec, cut.EndSec), "--force-keyframes-at-cuts")
		}
		args = append(args, "-o", filepath.Join(tmp, "asset.%(ext)s"), link.URL)
		if _, e = s.runMediaYTDLP(ctx, args, 20*time.Minute); e != nil {
			return MediaRecord{}, false, e
		}
		entries, e := os.ReadDir(tmp)
		if e != nil {
			return MediaRecord{}, false, e
		}
		for _, entry := range entries {
			if !entry.IsDir() && (entry.Name() == "asset.mp4" || entry.Name() == "asset.mkv" || entry.Name() == "asset.webm") {
				file = entry.Name()
				break
			}
		}
	}
	if file == "" {
		return MediaRecord{}, false, apiError("internal_error", "yt-dlp không tạo ra file video")
	}
	provider := "Bilibili"
	if link.Platform == "twitch" {
		provider = "Twitch"
	}
	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = provider
	}
	title = linkTitleInvalid.ReplaceAllString(title, " ")
	runes := []rune(title)
	if len(runes) > 80 {
		title = string(runes[:80])
	}
	title = strings.TrimSpace(title)
	suffix := ""
	if cut != nil {
		suffix = " (" + linkFileClock(cut.StartSec) + "-" + linkFileClock(cut.EndSec) + ")"
	}
	mime := "video/mp4"
	if filepath.Ext(file) == ".webm" {
		mime = "video/webm"
	}
	m, reused, e := s.ImportFileOnce(ctx, ImportFileOnceOptions{File: filepath.Join(tmp, file), FileName: title + suffix + filepath.Ext(file), DeclaredMIME: mime, MIMEPrefix: "video/", ProjectID: opts.ProjectID})
	if e != nil {
		return MediaRecord{}, false, e
	}
	if !reused {
		a := Object{"provider": provider, "sourceUrl": link.URL}
		author := info.Channel
		if author == nil {
			author = info.Uploader
		}
		if author != nil && *author != "" {
			a["author"] = *author
		}
		if info.UploaderURL != nil && *info.UploaderURL != "" {
			a["authorUrl"] = *info.UploaderURL
		}
		m, e = s.UpdateMediaMetadata(ctx, m.ID, Object{"attribution": a})
		if e != nil {
			return MediaRecord{}, false, e
		}
	}
	if opts.Overlay {
		overlay := true
		m, e = s.DetectMediaBackground(ctx, m.ID, true, &overlay)
	}
	return m, reused, e
}
func (s *Service) ImportMediaFromLink(ctx context.Context, input Object) (Object, error) {
	if input == nil {
		return nil, apiError("invalid_request", "Link import must be an object")
	}
	opts := FetchLinkMediaOptions{}
	for key, target := range map[string]*string{"url": &opts.URL, "from": &opts.From, "to": &opts.To, "projectId": &opts.ProjectID} {
		if v, ok := input[key]; ok {
			str, ok := v.(string)
			if !ok {
				return nil, apiError("invalid_request", key+" must be a string")
			}
			*target = str
		}
	}
	if opts.URL == "" || opts.ProjectID == "" {
		return nil, apiError("invalid_request", "url and projectId are required")
	}
	if v, ok := input["overlay"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, apiError("invalid_request", "overlay must be a boolean")
		}
		opts.Overlay = b
	}
	m, reused, e := s.FetchLinkMedia(ctx, opts, nil)
	if e != nil {
		return nil, e
	}
	return Object{"media": m, "reused": reused}, nil
}

// Playlist HTTP is pinned to a resolved public address; redirects repeat this
// validation. No environment proxy may turn a public URL into a private hop.
func publicMediaIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}
func mediaRemoteURL(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u == nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, apiError("invalid_request", "Unsafe media URL")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, apiError("invalid_request", "Private media host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && !publicMediaIP(ip) {
		return nil, apiError("invalid_request", "Private media address is not allowed")
	}
	return u, nil
}
func fetchMediaPlaylist(ctx context.Context, raw string) (string, error) {
	if _, e := mediaRemoteURL(raw); e != nil {
		return "", e
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		if len(ips) == 0 {
			return nil, apiError("invalid_request", "Media host has no public address")
		}
		for _, ip := range ips {
			if !publicMediaIP(ip.IP) {
				return nil, apiError("invalid_request", "Private media address is not allowed")
			}
		}
		d := net.Dialer{Timeout: 15 * time.Second}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return apiError("invalid_request", "Too many media redirects")
		}
		_, e := mediaRemoteURL(req.URL.String())
		return e
	}}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("User-Agent", "viderse")
	res, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("playlist HTTP %d", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 8<<20+1))
	if e != nil {
		return "", e
	}
	if len(b) > 8<<20 {
		return "", apiError("payload_too_large", "Media playlist exceeds 8 MiB")
	}
	return string(b), nil
}

type SlicedMediaPlaylist struct {
	Playlist  string
	OffsetSec float64
	Variant   string
}

var hlsResolution = regexp.MustCompile(`RESOLUTION=\d+x(\d+)`)
var hlsURI = regexp.MustCompile(`URI="([^"]+)"`)

func absoluteMediaURL(base, ref string) (string, error) {
	b, e := url.Parse(base)
	if e != nil {
		return "", e
	}
	r, e := url.Parse(ref)
	if e != nil {
		return "", e
	}
	u := b.ResolveReference(r).String()
	if _, e = mediaRemoteURL(u); e != nil {
		return "", e
	}
	return u, nil
}
func SliceMediaPlaylist(text, base string, start, end float64) (*SlicedMediaPlaylist, error) {
	type item struct {
		tags     []string
		duration float64
		uri      string
	}
	type variant struct {
		height float64
		url    string
	}
	items := []item{}
	variants := []variant{}
	var pending []string
	duration := 0.0
	mapTag := ""
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			height := 0.0
			if m := hlsResolution.FindStringSubmatch(line); len(m) > 1 {
				height, _ = strconv.ParseFloat(m[1], 64)
			}
			for _, next := range lines[index+1:] {
				next = strings.TrimSpace(next)
				if next != "" && !strings.HasPrefix(next, "#") {
					u, e := absoluteMediaURL(base, next)
					if e != nil {
						return nil, e
					}
					variants = append(variants, variant{height, u})
					break
				}
			}
			continue
		}
		if len(variants) > 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "#EXT-X-KEY"):
			return nil, nil
		case strings.HasPrefix(line, "#EXT-X-MAP"):
			var mapErr error
			mapTag = hlsURI.ReplaceAllStringFunc(line, func(v string) string {
				m := hlsURI.FindStringSubmatch(v)
				u, e := absoluteMediaURL(base, m[1])
				if e != nil {
					mapErr = e
				}
				return `URI="` + u + `"`
			})
			if mapErr != nil {
				return nil, mapErr
			}
		case strings.HasPrefix(line, "#EXTINF:"):
			duration, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(line, "#EXTINF:"), ","), 64)
			if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 {
				return nil, apiError("invalid_request", "Invalid playlist duration")
			}
		case strings.HasPrefix(line, "#EXT-X-BYTERANGE"), strings.HasPrefix(line, "#EXT-X-DISCONTINUITY"):
			pending = append(pending, line)
		case strings.HasPrefix(line, "#"):
			continue
		default:
			u, e := absoluteMediaURL(base, line)
			if e != nil {
				return nil, e
			}
			items = append(items, item{pending, duration, u})
			pending = nil
			duration = 0
		}
	}
	if len(variants) > 0 {
		sort.SliceStable(variants, func(i, j int) bool { return math.Abs(variants[i].height-480) < math.Abs(variants[j].height-480) })
		return &SlicedMediaPlaylist{Variant: variants[0].url}, nil
	}
	if len(items) == 0 {
		return nil, nil
	}
	elapsed := 0.0
	first, last := -1, -1
	offset := 0.0
	for i, item := range items {
		if first < 0 && elapsed+item.duration > start {
			first = i
			offset = math.Max(0, start-elapsed)
		}
		elapsed += item.duration
		if first >= 0 {
			last = i
		}
		if elapsed >= end {
			break
		}
	}
	if first < 0 {
		return nil, nil
	}
	kept := items[first : last+1]
	target := 1.0
	for _, item := range kept {
		target = math.Max(target, math.Ceil(item.duration))
	}
	out := []string{"#EXTM3U", "#EXT-X-VERSION:6", "#EXT-X-PLAYLIST-TYPE:VOD", fmt.Sprintf("#EXT-X-TARGETDURATION:%.0f", target)}
	if mapTag != "" {
		out = append(out, mapTag)
	}
	for _, item := range kept {
		out = append(out, item.tags...)
		out = append(out, fmt.Sprintf("#EXTINF:%.3f,", item.duration), item.uri)
	}
	out = append(out, "#EXT-X-ENDLIST")
	return &SlicedMediaPlaylist{Playlist: strings.Join(out, "\n") + "\n", OffsetSec: offset}, nil
}
func pickHLSFormats(formats []LinkFormat) (*LinkFormat, *LinkFormat) {
	videos := []LinkFormat{}
	audios := []LinkFormat{}
	isHLS := func(f LinkFormat) bool {
		u, e := url.Parse(f.URL)
		return e == nil && strings.HasSuffix(u.Path, ".m3u8")
	}
	for _, f := range formats {
		if !isHLS(f) {
			continue
		}
		if f.VideoCodec != nil && *f.VideoCodec != "none" {
			videos = append(videos, f)
		}
		if f.VideoCodec != nil && *f.VideoCodec == "none" {
			audios = append(audios, f)
		}
	}
	if len(videos) == 0 {
		return nil, nil
	}
	value := func(n *float64, def float64) float64 {
		if n == nil {
			return def
		}
		return *n
	}
	avc := func(f LinkFormat) bool {
		return f.VideoCodec != nil && (strings.HasPrefix(strings.ToLower(*f.VideoCodec), "avc1") || strings.HasPrefix(strings.ToLower(*f.VideoCodec), "h264"))
	}
	sort.SliceStable(videos, func(i, j int) bool {
		a, b := videos[i], videos[j]
		da, db := math.Abs(value(a.Height, 2160)-480), math.Abs(value(b.Height, 2160)-480)
		if da != db {
			return da < db
		}
		if avc(a) != avc(b) {
			return avc(a)
		}
		return value(a.TBR, 0) > value(b.TBR, 0)
	})
	v := videos[0]
	if v.AudioCodec != nil && *v.AudioCodec != "" && *v.AudioCodec != "none" {
		return &v, nil
	}
	sort.SliceStable(audios, func(i, j int) bool { return value(audios[i].TBR, 0) > value(audios[j].TBR, 0) })
	if len(audios) == 0 {
		return &v, nil
	}
	return &v, &audios[0]
}
func (s *Service) slicedHLSPlaylist(ctx context.Context, raw string, cut LinkCut, dir, name string, deps *LinkImportDeps) (*SlicedMediaPlaylist, error) {
	fetch := fetchMediaPlaylist
	customFetch := deps != nil && deps.FetchText != nil
	if customFetch {
		fetch = deps.FetchText
	}
	for range 8 {
		if _, e := mediaRemoteURL(raw); e != nil {
			return nil, e
		}
		text, e := fetch(ctx, raw)
		if e != nil {
			return nil, apiError("internal_error", "Không tải được danh sách phát của video — thử lại", Object{"detail": e.Error()})
		}
		sliced, e := SliceMediaPlaylist(text, raw, cut.StartSec, cut.EndSec)
		if e != nil || sliced == nil {
			return sliced, e
		}
		if sliced.Variant != "" {
			raw = sliced.Variant
			continue
		}
		if !customFetch {
			if e = validateMediaPlaylistHosts(ctx, sliced.Playlist); e != nil {
				return nil, e
			}
		}
		if e = os.WriteFile(filepath.Join(dir, name), []byte(sliced.Playlist), 0600); e != nil {
			return nil, e
		}
		return sliced, nil
	}
	return nil, apiError("invalid_request", "Too many nested HLS playlists")
}
func (s *Service) fetchHLSCut(ctx context.Context, info LinkInfo, cut LinkCut, tmp, ffmpeg string, deps *LinkImportDeps) (string, error) {
	v, a := pickHLSFormats(info.Formats)
	if v == nil {
		return "", nil
	}
	video, e := s.slicedHLSPlaylist(ctx, v.URL, cut, tmp, "video.m3u8", deps)
	if e != nil || video == nil {
		return "", e
	}
	var audio *SlicedMediaPlaylist
	if a != nil {
		audio, e = s.slicedHLSPlaylist(ctx, a.URL, cut, tmp, "audio.m3u8", deps)
		if e != nil || audio == nil {
			return "", e
		}
	}
	output := "asset.mkv"
	if v.VideoCodec != nil && (strings.HasPrefix(strings.ToLower(*v.VideoCodec), "avc1") || strings.HasPrefix(strings.ToLower(*v.VideoCodec), "h264")) {
		output = "asset.mp4"
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file,http,https,tcp,tls,crypto"}
	if video.OffsetSec > 0 {
		args = append(args, "-ss", strconv.FormatFloat(video.OffsetSec, 'f', -1, 64))
	}
	args = append(args, "-i", filepath.Join(tmp, "video.m3u8"), "-map", "0:v")
	if audio != nil {
		if audio.OffsetSec > 0 {
			args = append(args, "-ss", strconv.FormatFloat(audio.OffsetSec, 'f', -1, 64))
		}
		args = append(args, "-i", filepath.Join(tmp, "audio.m3u8"), "-map", "1:a?")
	} else {
		args = append(args, "-map", "0:a?")
	}
	args = append(args, "-t", strconv.FormatFloat(cut.EndSec-cut.StartSec, 'f', -1, 64), "-c", "copy", "-y", filepath.Join(tmp, output))
	r := s.Runner
	if r == nil {
		r = &tools.ExecRunner{}
	}
	if _, e = r.Run(ctx, ffmpeg, args, tools.Spec{Timeout: 10 * time.Minute}); e != nil {
		return "", apiError("internal_error", "Không cắt được đoạn video — thử lại", Object{"detail": e.Error()})
	}
	return output, nil
}

func validateMediaPlaylistHosts(ctx context.Context, playlist string) error {
	checked := map[string]bool{}
	for _, line := range strings.Split(playlist, "\n") {
		raw := strings.TrimSpace(line)
		if strings.HasPrefix(raw, "#EXT-X-MAP") {
			match := hlsURI.FindStringSubmatch(raw)
			if len(match) < 2 {
				return apiError("invalid_request", "HLS initialization URL is missing")
			}
			raw = match[1]
		} else if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		u, e := mediaRemoteURL(raw)
		if e != nil {
			return e
		}
		host := u.Hostname()
		if checked[host] {
			continue
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return e
		}
		if len(ips) == 0 {
			return apiError("invalid_request", "Media host has no public address")
		}
		for _, ip := range ips {
			if !publicMediaIP(ip.IP) {
				return apiError("invalid_request", "Private media address is not allowed")
			}
		}
		checked[host] = true
	}
	return nil
}
