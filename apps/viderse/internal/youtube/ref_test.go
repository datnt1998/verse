// Ca test chuyển từ yt-factory src/server/youtube/channel.test.ts:
// resolveChannelId chỉ nhận @handle, URL kênh và mã UC — link video, host khác
// hay chuỗi tự do là đầu vào hỏng, không phải "kênh không tồn tại".
package youtube

import "testing"

func TestParseChannelRefAcceptsChannelForms(t *testing.T) {
	cases := []struct{ in, want string }{
		{"@SomeHandle", "https://www.youtube.com/@SomeHandle"},
		{"@SomeHandle/", "https://www.youtube.com/@SomeHandle"},
		{"https://www.youtube.com/@SomeHandle", "https://www.youtube.com/@SomeHandle"},
		{"https://www.youtube.com/@SomeHandle/videos", "https://www.youtube.com/@SomeHandle/videos"},
		{"youtube.com/@SomeHandle", "https://www.youtube.com/@SomeHandle"},
		{"https://www.youtube.com/c/SomeName", "https://www.youtube.com/c/SomeName"},
		{"https://www.youtube.com/user/SomeUser", "https://www.youtube.com/user/SomeUser"},
		{"https://m.youtube.com/channel/UCHnyfMqiRRG1u-2MsSQLbXA", "UCHnyfMqiRRG1u-2MsSQLbXA"},
		{"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2MsSQLbXA/videos", "UCHnyfMqiRRG1u-2MsSQLbXA"},
		{"  UCHnyfMqiRRG1u-2MsSQLbXA  ", "UCHnyfMqiRRG1u-2MsSQLbXA"},
		{"UCHnyfMqiRRG1u-2MsSQLbXA", "UCHnyfMqiRRG1u-2MsSQLbXA"},
	}
	for _, c := range cases {
		got, ok := ParseChannelRef(c.in)
		if !ok || got != c.want {
			t.Errorf("ParseChannelRef(%q) = %q, %v; muốn %q, true", c.in, got, ok, c.want)
		}
	}
}

func TestParseChannelRefRejectsNonChannelInput(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"@",
		"not a channel",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=PL123",
		"https://www.youtube.com/",
		"https://vimeo.com/channels/123",
		"ftp://youtube.com/@SomeHandle",
		// Một đoạn handle đúng nghĩa, không lồng path/query/khoảng trắng.
		"@name/path",
		"@name?bad",
		"@name bad",
		"https://www.youtube.com/@name/videos/extra",
		"https://www.youtube.com/anything/channel/UCHnyfMqiRRG1u-2MsSQLbXA",
		"https://www.youtube.com/channel/UCHnyfMqiRRG1u-2MsSQLbXA/not-a-tab",
		"https://www.youtube.com/channel/UCshort",
		"https://www.youtube.com/@name?tab=videos",
		"https://user@www.youtube.com/@SomeHandle",
		"https://www.youtube.com:8443/@SomeHandle",
		"https://www.youtube.com/c/SomeName/extra",
		"https://www.youtube.com/playlist?list=PL123",
	}
	for _, in := range cases {
		if got, ok := ParseChannelRef(in); ok {
			t.Errorf("ParseChannelRef(%q) = %q, true; muốn false", in, got)
		}
	}
}

func TestParseChannelRefKeepsInternationalHandles(t *testing.T) {
	for _, in := range []string{"@ガイド", "https://www.youtube.com/@ガイド", "https://www.youtube.com/c/日本語"} {
		if got, ok := ParseChannelRef(in); !ok || got == "" {
			t.Errorf("ParseChannelRef(%q) = %q, %v; muốn handle quốc tế hợp lệ", in, got, ok)
		}
	}
	// Ký tự mã hoá hoặc phân tách vẫn bị chặn.
	if _, ok := ParseChannelRef("https://www.youtube.com/@%2Fetc%2Fpasswd"); ok {
		t.Error("handle mã hoá dấu / phải bị từ chối")
	}
	if _, ok := ParseChannelRef("https://www.youtube.com/@na\x00me"); ok {
		t.Error("handle chứa ký tự điều khiển phải bị từ chối")
	}
}

func TestParseChannelRefVideoURLIsNotAChannelEvenOnYoutubeHost(t *testing.T) {
	// /watch, /shorts và /embed đều nằm trên host YouTube nhưng không phải kênh.
	for _, path := range []string{"/watch?v=abc", "/shorts/abc", "/embed/abc", "/live/abc"} {
		if _, ok := ParseChannelRef("https://www.youtube.com" + path); ok {
			t.Errorf("%s bị nhận là kênh", path)
		}
	}
}
