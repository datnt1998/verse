package youtube

import (
	"context"
	"errors"
	"regexp"
	"time"
)

// UploadTime là thời điểm đăng chính xác của một video, phục vụ khung đăng của
// đối thủ. Trang kênh chỉ nói "3 days ago"; player response của watch page mang
// microformat.playerMicroformatRenderer.publishDate (ISO 8601 có offset).
// Một request /player cho một video, qua session chia sẻ nên bộ điều tốc
// toàn cục vẫn can nhịp như mọi request listing.

// withTime nhận dạng "2024-05-14T07:00…": giá trị chỉ-ngày không có giờ nên vô
// dụng cho khung đăng.
var withTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`)

var botReasonRe = regexp.MustCompile(`(?i)bot|sign in to confirm`)

func exactMoment(value string) *time.Time {
	if value == "" || !withTime.MatchString(value) {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &t
}

// ParseUploadTime đọc thời điểm đăng trong một player response thô; trả nil khi
// video không có (private, đã xoá, chỉ có ngày). Trang thử thách bot ném
// *UploadTimeBlockedError vì nó che mọi video.
func ParseUploadTime(raw any) (*time.Time, error) {
	page, ok := raw.(map[string]any)
	if !ok {
		return nil, nil
	}
	playability := jobj(page["playabilityStatus"])
	microformat := jobj(page["microformat"])
	var renderer jmap
	if microformat != nil {
		renderer = jobj(microformat["playerMicroformatRenderer"])
	}
	// publishDate là lúc lên sóng (premiere/lên lịch), uploadDate là lúc file tới.
	if renderer != nil {
		if m := exactMoment(jstr(renderer["publishDate"])); m != nil {
			return m, nil
		}
		if m := exactMoment(jstr(renderer["uploadDate"])); m != nil {
			return m, nil
		}
	}
	reason := jstr(playability["reason"])
	if renderer == nil && jstr(playability["status"]) == "LOGIN_REQUIRED" && botReasonRe.MatchString(reason) {
		return nil, &UploadTimeBlockedError{Reason: reason}
	}
	return nil, nil
}

// ParseChannelCaptions báo video có phụ đề do kênh đăng (track có kind khác
// "asr" — phụ đề tự động không tính). Không có danh sách caption → false.
func ParseChannelCaptions(raw any) bool {
	page, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	tracks := jarr(jget(page, "captions", "playerCaptionsTracklistRenderer", "captionTracks"))
	for _, raw := range tracks {
		// Hàng null/rác không phải caption track: bỏ qua, không suy ra "kênh tự đăng".
		track := jobj(raw)
		if track == nil || jstr(track["baseUrl"]) == "" {
			continue
		}
		if jstr(track["kind"]) != "asr" {
			return true
		}
	}
	return false
}

// FetchUploadTime gọi /player cho một video và trả thời điểm đăng chính xác
// (nil khi không có) cùng cờ phụ đề do kênh đăng. Lỗi đi qua cùng lớp chuẩn
// hoá với đọc trang kênh (ctx và DeferredError nguyên vẹn, còn lại FetchError).
func (c *Client) FetchUploadTime(ctx context.Context, videoID string) (*time.Time, bool, error) {
	sess, err := c.session(ctx, "")
	if err != nil {
		return nil, false, normalizeError(err, false, c.now())
	}
	resp, err := sess.execute(ctx, "/player", jmap{"videoId": videoID})
	if err != nil {
		return nil, false, normalizeError(err, false, c.now())
	}
	at, err := ParseUploadTime(resp)
	if err != nil {
		// Trang thử thách bot là "nền tảng đang chặn", không phải video lỗi:
		// worker dừng thăm dò một lúc thay vì đếm lỗi từng video.
		var blocked *UploadTimeBlockedError
		if errors.As(err, &blocked) {
			return nil, false, newFetchError(KindBlocked, err.Error(), 0, nil)
		}
		return nil, false, normalizeError(err, false, c.now())
	}
	return at, ParseChannelCaptions(resp), nil
}
