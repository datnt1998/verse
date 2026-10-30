// Ca test chuyển từ yt-factory src/server/youtube/upload-time.test.ts.
package youtube

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func playerResp(renderer map[string]string, playability map[string]string) any {
	m := jmap{}
	if playability != nil {
		ps := jmap{}
		for k, v := range playability {
			ps[k] = v
		}
		m["playabilityStatus"] = ps
	}
	if renderer != nil {
		r := jmap{}
		for k, v := range renderer {
			r[k] = v
		}
		m["microformat"] = jmap{"playerMicroformatRenderer": r}
	}
	return m
}

func TestUploadTimePrefersPublishDateWithOffset(t *testing.T) {
	scheduled := playerResp(
		map[string]string{"publishDate": "2025-10-14T07:00:06-07:00", "uploadDate": "2025-10-10T22:15:00-07:00"},
		map[string]string{"status": "OK"},
	)
	got, err := ParseUploadTime(scheduled)
	if err != nil || got == nil {
		t.Fatalf("ParseUploadTime = %v, %v", got, err)
	}
	want, _ := time.Parse(time.RFC3339, "2025-10-14T14:00:06Z")
	if !got.Equal(want) {
		t.Fatalf("got %v, muốn %v", got, want)
	}
	upload := playerResp(map[string]string{"uploadDate": "2025-10-10T22:15:00-07:00"}, map[string]string{"status": "OK"})
	got, err = ParseUploadTime(upload)
	if err != nil || got == nil {
		t.Fatalf("ParseUploadTime = %v, %v", got, err)
	}
	want, _ = time.Parse(time.RFC3339, "2025-10-11T05:15:00Z")
	if !got.Equal(want) {
		t.Fatalf("got %v, muốn %v", got, want)
	}
}

func TestUploadTimeSettlesNullWithoutExactMoment(t *testing.T) {
	cases := map[string]any{
		"date only": playerResp(map[string]string{"publishDate": "2025-10-14", "uploadDate": "2025-10-14"}, map[string]string{"status": "OK"}),
		"removed":   playerResp(nil, map[string]string{"status": "ERROR", "reason": "This video is unavailable"}),
		"private":   playerResp(nil, map[string]string{"status": "LOGIN_REQUIRED", "reason": "This video is private"}),
		"not json":  "not json",
	}
	for name, raw := range cases {
		got, err := ParseUploadTime(raw)
		if err != nil || got != nil {
			t.Errorf("%s: ParseUploadTime = %v, %v; muốn nil,nil", name, got, err)
		}
	}
}

func TestUploadTimeBotCheckIsRefusal(t *testing.T) {
	bot := playerResp(nil, map[string]string{"status": "LOGIN_REQUIRED", "reason": "Sign in to confirm you’re not a bot"})
	_, err := ParseUploadTime(bot)
	var blocked *UploadTimeBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("mong đợi *UploadTimeBlockedError, nhận %v", err)
	}
	// Thời gian vẫn thắng khi YouTube gửi nó kèm trang từ chối.
	gated := playerResp(
		map[string]string{"publishDate": "2025-10-14T07:00:06-07:00"},
		map[string]string{"status": "LOGIN_REQUIRED", "reason": "Sign in to confirm you’re not a bot"},
	)
	got, err := ParseUploadTime(gated)
	if err != nil || got == nil {
		t.Fatalf("ParseUploadTime = %v, %v", got, err)
	}
}

func TestUploadTimeCountsOnlyChannelProvidedCaptions(t *testing.T) {
	withTracks := func(tracks ...map[string]string) any {
		list := make([]any, len(tracks))
		for i, track := range tracks {
			m := jmap{}
			for k, v := range track {
				m[k] = v
			}
			list[i] = m
		}
		return jmap{"captions": jmap{"playerCaptionsTracklistRenderer": jmap{"captionTracks": list}}}
	}
	cases := []struct {
		name string
		raw  any
		want bool
	}{
		{"kênh tự đăng phụ đề", withTracks(map[string]string{"baseUrl": "https://x", "languageCode": "vi"}), true},
		{"asr lẫn phụ đề kênh", withTracks(map[string]string{"baseUrl": "https://x", "kind": "asr"}, map[string]string{"baseUrl": "https://x", "languageCode": "en"}), true},
		{"chỉ phụ đề tự động", withTracks(map[string]string{"baseUrl": "https://x", "kind": "asr"}), false},
		{"hàng null/rỗng", jmap{"captions": jmap{"playerCaptionsTracklistRenderer": jmap{"captionTracks": []any{nil, jmap{}}}}}, false},
		{"không có caption", playerResp(map[string]string{"publishDate": "2025-10-14T07:00:06-07:00"}, nil), false},
		{"không phải object", "not json", false},
	}
	for _, c := range cases {
		if got := ParseChannelCaptions(c.raw); got != c.want {
			t.Errorf("%s: ParseChannelCaptions = %v, muốn %v", c.name, got, c.want)
		}
	}
}

func TestFetchUploadTimeReadsPlayerThroughPacer(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, nil)
	client := newFixtureClient(ft)
	at, hasCaptions, err := client.FetchUploadTime(context.Background(), "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("FetchUploadTime: %v", err)
	}
	if at == nil {
		t.Fatal("FetchUploadTime trả nil thời điểm đăng")
	}
	want, _ := time.Parse(time.RFC3339, "2025-10-14T14:00:06Z")
	if !at.Equal(want) {
		t.Fatalf("thời điểm đăng = %v, muốn %v", at, want)
	}
	if !hasCaptions {
		t.Fatal("fixture có track không phải asr nhưng hasCaptions=false")
	}
	assertKinds(t, ft, kindSW, kindPlayer)
}

func TestFetchUploadTimeAsrOnlyIsNoChannelCaptions(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindPlayer: func(tracedReq) (*http.Response, error) {
			return okResponse(`{"playabilityStatus":{"status":"OK"},"microformat":{"playerMicroformatRenderer":{"publishDate":"2025-10-14T07:00:06-07:00"}},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"kind":"asr","languageCode":"en"}]}}}`), nil
		},
	})
	client := newFixtureClient(ft)
	at, hasCaptions, err := client.FetchUploadTime(context.Background(), "dQw4w9WgXcQ")
	if err != nil || at == nil {
		t.Fatalf("FetchUploadTime = %v, %v", at, err)
	}
	if hasCaptions {
		t.Fatal("chỉ có phụ đề asr nhưng hasCaptions=true")
	}
}

func TestFetchUploadTimeBotChallengeIsBlocked(t *testing.T) {
	ft := (&fixtureTransport{}).install(t, map[reqKind]handlerFunc{
		kindPlayer: func(tracedReq) (*http.Response, error) {
			return okResponse(`{"playabilityStatus":{"status":"LOGIN_REQUIRED","reason":"Sign in to confirm you’re not a bot"}}`), nil
		},
	})
	client := newFixtureClient(ft)
	_, _, err := client.FetchUploadTime(context.Background(), "dQw4w9WgXcQ")
	kind, ok := KindOf(err)
	if !ok || kind != KindBlocked {
		t.Fatalf("FetchUploadTime lỗi = %v (kind %q, ok=%v); muốn blocked", err, kind, ok)
	}
}
