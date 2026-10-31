package youtube

import (
	"testing"
)

func TestVideoMetadataReadsPlayerDetailsAndPrivateState(t *testing.T) {
	raw := map[string]any{
		"playabilityStatus": map[string]any{"status": "OK"},
		"videoDetails":      map[string]any{"videoId": "abcdefghijk", "title": "Nguồn thật", "lengthSeconds": "120", "channelId": "UCaaaaaaaaaaaaaaaaaaaaaa", "author": "Kênh nguồn", "viewCount": "2400", "thumbnail": map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/abcdefghijk/default.jpg"}}}},
	}
	meta, err := ParseVideoMetadata(raw)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "Nguồn thật" || meta.DurationSec == nil || *meta.DurationSec != 120 || meta.ViewCount == nil || *meta.ViewCount != 2400 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	_, err = ParseVideoMetadata(map[string]any{"playabilityStatus": map[string]any{"status": "LOGIN_REQUIRED", "reason": "This is a private video"}})
	if err == nil {
		t.Fatal("private player accepted")
	}
}
