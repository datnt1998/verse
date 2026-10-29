// Ca test chuyển từ yt-factory src/server/youtube/upload-time.test.ts.
package youtube

import (
	"errors"
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
