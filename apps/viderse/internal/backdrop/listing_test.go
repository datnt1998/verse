package backdrop_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// listingRunner trả danh sách phẳng theo tiền tố URL (vod/highlight/clip).
type listingRunner struct {
	entries map[string][]map[string]any
	calls   []string
	failOn  string
}

func (r *listingRunner) run(_ context.Context, _ domain.PlatformID, args []string, _ int, _ time.Duration, _ func(string)) (string, error) {
	url := args[len(args)-1]
	r.calls = append(r.calls, url)
	if r.failOn != "" && url == r.failOn {
		return "", &tools.YtDlpError{Message: "yt-dlp hỏng", Code: tools.YtDlpFailed}
	}
	key := "vod"
	switch {
	case contains(url, "/clips"):
		key = "clip"
	case contains(url, "filter=highlights"):
		key = "highlight"
	}
	payload := map[string]any{"entries": r.entries[key]}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func vodEntry(id string, duration float64) map[string]any {
	return map[string]any{
		"id": id, "url": "https://www.twitch.tv/videos/" + id[1:],
		"title": "VOD " + id, "duration": duration,
	}
}

func clipEntry(slug string) map[string]any {
	return map[string]any{
		"id": "12345", "url": "https://www.twitch.tv/somechannel/clip/" + slug + "?tt_content=url",
		"title": "Clip " + slug, "duration": 30.0, "timestamp": 1700000000,
	}
}

func TestListChannelVideosPoolsAndDedupe(t *testing.T) {
	runner := &listingRunner{entries: map[string][]map[string]any{
		"vod": {
			vodEntry("v1", 3600), vodEntry("v2", 1800), vodEntry("v2", 1800),
			{"id": "not-a-vod", "url": "https://www.twitch.tv/videos/9"},
			{"id": "v3"},
		},
		"highlight": {vodEntry("v4", 600)},
		"clip":      {clipEntry("AbcDef"), clipEntry("AbcDef"), {"id": "9", "url": "https://example.com/x"}},
	}}
	listing, err := backdrop.ListChannelVideos(context.Background(), runner.run, "somechannel")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listing.Videos) != 4 {
		t.Fatalf("videos = %d, muốn 4 (trùng bị bỏ, mục sai hình bị bỏ)", len(listing.Videos))
	}
	kinds := map[string]int{}
	for _, video := range listing.Videos {
		kinds[video.Kind]++
	}
	if kinds[domain.PlatformKindVod] != 2 || kinds[domain.PlatformKindHighlight] != 1 || kinds[domain.PlatformKindClip] != 1 {
		t.Fatalf("kinds = %v", kinds)
	}
	if !listing.VodsComplete {
		t.Fatal("3 VOD < trần 100 nên danh sách VOD phải trọn")
	}
	for _, video := range listing.Videos {
		if video.Kind == domain.PlatformKindClip && video.VideoID != "AbcDef" {
			t.Fatalf("mã clip = %q, muốn slug", video.VideoID)
		}
	}
	if len(runner.calls) != 3 {
		t.Fatalf("calls = %d, muốn 3 nguồn", len(runner.calls))
	}
	if !contains(runner.calls[0], "filter=archives") || !contains(runner.calls[2], "range=all") {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestListChannelVideosRequestsCardSizedTwitchThumbnails(t *testing.T) {
	const frame = "https://static-cdn.jtvnw.net/twitch-video-assets/a/landscape/thumb/thumb-0000000000"
	clip := clipEntry("AbcDef")
	clip["thumbnail"] = "http:" + frame[len("https:"):] + "-1920x1080.jpg"
	vod := vodEntry("v1", 600)
	vod["thumbnail"] = "https://static-cdn.jtvnw.net/cf_vods/x/thumb/thumb0-320x180.jpg"
	runner := &listingRunner{entries: map[string][]map[string]any{"vod": {vod}, "clip": {clip}}}
	listing, err := backdrop.ListChannelVideos(context.Background(), runner.run, "somechannel")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := map[string]string{
		"AbcDef": frame + "-640x360.jpg",                                            // khung đầy đủ → cỡ thẻ, http → https
		"v1":     "https://static-cdn.jtvnw.net/cf_vods/x/thumb/thumb0-320x180.jpg", // cỡ khác giữ nguyên
	}
	for _, video := range listing.Videos {
		if video.ThumbnailURL == nil || *video.ThumbnailURL != want[video.VideoID] {
			t.Errorf("%s thumbnail = %v, muốn %s", video.VideoID, video.ThumbnailURL, want[video.VideoID])
		}
	}
}

func TestListChannelVideosVodsTruncated(t *testing.T) {
	runner := &listingRunner{entries: map[string][]map[string]any{}}
	for i := range backdrop.TwitchPoolLimit {
		runner.entries["vod"] = append(runner.entries["vod"], vodEntry(fmt.Sprintf("v%d", i+1), 600))
	}
	listing, err := backdrop.ListChannelVideos(context.Background(), runner.run, "somechannel")
	if err != nil {
		t.Fatal(err)
	}
	if listing.VodsComplete {
		t.Fatal("đủ trần 100 VOD thì danh sách chưa trọn: không được đánh dấu đã xoá")
	}
	if len(listing.Videos) != backdrop.TwitchPoolLimit {
		t.Fatalf("videos = %d", len(listing.Videos))
	}
}

func TestListChannelVideosOnePoolFailureFailsListing(t *testing.T) {
	runner := &listingRunner{
		entries: map[string][]map[string]any{"vod": {vodEntry("v1", 60)}},
		failOn:  "https://www.twitch.tv/somechannel/clips?filter=clips&range=all",
	}
	_, err := backdrop.ListChannelVideos(context.Background(), runner.run, "somechannel")
	if err == nil {
		t.Fatal("một nguồn hỏng phải làm hỏng cả lượt")
	}
}

func TestListChannelVideosBadJSON(t *testing.T) {
	_, err := backdrop.ListChannelVideos(context.Background(), func(context.Context, domain.PlatformID, []string, int, time.Duration, func(string)) (string, error) {
		return "không phải json", nil
	}, "somechannel")
	var toolErr *tools.YtDlpError
	if !errors.As(err, &toolErr) || toolErr.Code != tools.YtDlpFailed {
		t.Fatalf("err = %v", err)
	}
}
