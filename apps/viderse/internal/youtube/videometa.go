package youtube

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// VideoMetadata is the player metadata needed before enqueueing a video.
// Missing fields remain nil; blocked/private players are never fabricated.
type VideoMetadata struct {
	ID           string
	Title        string
	ChannelID    *string
	ChannelTitle *string
	DurationSec  *float64
	ThumbnailURL *string
	ViewCount    *int64
	PublishedAt  *time.Time
	HasCaptions  bool
}

type VideoUnavailableError struct{ Reason string }

func (e *VideoUnavailableError) Error() string { return e.Reason }

func ParseVideoMetadata(raw any) (VideoMetadata, error) {
	page, ok := raw.(map[string]any)
	if !ok {
		return VideoMetadata{}, errors.New("không đọc được video")
	}
	status := jobj(page["playabilityStatus"])
	if code := jstr(status["status"]); code != "" && code != "OK" {
		reason := strings.ToLower(jstr(status["reason"]))
		if strings.Contains(reason, "private") {
			return VideoMetadata{}, &VideoUnavailableError{Reason: "private"}
		}
		return VideoMetadata{}, &VideoUnavailableError{Reason: "unreadable"}
	}
	details := jobj(page["videoDetails"])
	id, title := jstr(details["videoId"]), jstr(details["title"])
	if id == "" || title == "" {
		return VideoMetadata{}, errors.New("không đọc được tên video")
	}
	meta := VideoMetadata{ID: id, Title: title, HasCaptions: ParseChannelCaptions(raw)}
	if v := jstr(details["channelId"]); v != "" {
		meta.ChannelID = &v
	}
	if v := jstr(details["author"]); v != "" {
		meta.ChannelTitle = &v
	}
	if n, e := strconv.ParseFloat(jstr(details["lengthSeconds"]), 64); e == nil && n > 0 {
		meta.DurationSec = &n
	}
	if n, e := strconv.ParseInt(jstr(details["viewCount"]), 10, 64); e == nil && n >= 0 {
		meta.ViewCount = &n
	}
	for _, v := range jarr(jget(details, "thumbnail", "thumbnails")) {
		if u := jstr(jobj(v)["url"]); u != "" {
			meta.ThumbnailURL = &u
		}
	}
	at, e := ParseUploadTime(raw)
	if e != nil {
		return VideoMetadata{}, e
	}
	meta.PublishedAt = at
	// A date without a time is real source data too (unlike posting-slot scans,
	// which specifically require exact moments).
	if at == nil {
		renderer := jobj(jget(page, "microformat", "playerMicroformatRenderer"))
		if t, e := time.Parse("2006-01-02", jstr(renderer["publishDate"])); e == nil {
			meta.PublishedAt = &t
		}
	}
	return meta, nil
}

// FetchVideoMetadata shares the authenticated/paced InnerTube player client
// with FetchUploadTime; a plan probe never downloads media or creates a job.
func (c *Client) FetchVideoMetadata(ctx context.Context, id string) (VideoMetadata, error) {
	sess, err := c.session(ctx, "")
	if err != nil {
		return VideoMetadata{}, normalizeError(err, false, c.now())
	}
	raw, err := sess.execute(ctx, "/player", jmap{"videoId": id})
	if err != nil {
		return VideoMetadata{}, normalizeError(err, false, c.now())
	}
	return ParseVideoMetadata(raw)
}
