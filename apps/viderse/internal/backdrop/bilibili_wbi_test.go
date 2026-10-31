package backdrop

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	wbiImg = "7cd084941338484aae1ad9425b84077c"
	wbiSub = "4932caff0ff746eab6f01bf08b70ac45"
)

// TestSignWBIWithKnownVector dùng đúng vector của bilibili-api.test.ts.
func TestSignWBIWithKnownVector(t *testing.T) {
	got := SignWBI(map[string]string{"foo": "114", "bar": "514", "zab": "1919810"}, wbiImg, wbiSub, 1702204169)
	want := "bar=514&foo=114&wts=1702204169&zab=1919810&w_rid=8f6f2b5b3d485fe1886cec6a0be8c5d4"
	if got != want {
		t.Fatalf("wbi = %q\nmuốn %q", got, want)
	}
	escaped := SignWBI(map[string]string{"value": "a b!'()*"}, wbiImg, wbiSub, 1702204169)
	if !strings.HasPrefix(escaped, "value=a%20b&wts=") {
		t.Fatalf("escape = %q", escaped)
	}
}

const pageFixture = `{
  "page": {"count": 3, "ps": 30},
  "list": {"vlist": [
    {"bvid": "BV1xx411c7mD", "title": "một", "length": "48:12", "pic": "//i0.hdslb.com/bfs/archive/first.jpg", "created": 1791613800, "author": "Người đăng"},
    {"bvid": "BV1yy411c7mE", "title": "hai", "length": "10:00", "pic": "//i0.hdslb.com/bfs/archive/other.jpg", "created": 1791000000, "author": "Người đăng"},
    {"bvid": "BV1zz411c7mF", "title": "ba", "length": "1:02:03", "pic": "", "created": 1790000000, "author": "Người đăng"}
  ]}
}`

func TestParseBilibiliPage(t *testing.T) {
	page, err := parseBilibiliPage(json.RawMessage(pageFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if page.Total != 3 || page.PageSize != 30 || len(page.Videos) != 3 {
		t.Fatalf("page = %+v", page)
	}
	if page.Videos[0].DurationSec == nil || *page.Videos[0].DurationSec != 2892 {
		t.Fatalf("duration = %v", page.Videos[0].DurationSec)
	}
	if page.Videos[2].DurationSec == nil || *page.Videos[2].DurationSec != 3723 {
		t.Fatalf("duration = %v", page.Videos[2].DurationSec)
	}
	if page.Videos[1].ThumbnailURL == nil || *page.Videos[1].ThumbnailURL != "https://i0.hdslb.com/bfs/archive/other.jpg" {
		t.Fatalf("thumbnail = %v", page.Videos[1].ThumbnailURL)
	}
	if page.Videos[0].PublishedAt == nil || page.Videos[0].PublishedAt.UnixMilli() != 1791613800000 {
		t.Fatalf("published = %v", page.Videos[0].PublishedAt)
	}
	if page.Title == nil || *page.Title != "Người đăng" {
		t.Fatalf("title = %v", page.Title)
	}
}
