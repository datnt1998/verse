package backdrop

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// channelVideoCursorJSON là hình mã hoá của con trỏ trang danh mục. Kèm bộ lọc
// (kênh + loại) để dùng con trỏ của bộ lọc khác là lỗi, không phải trang lạ.
type channelVideoCursorJSON struct {
	Channel string `json:"c"`
	Kinds   string `json:"k"`
	At      *int64 `json:"p,omitempty"`
	Video   string `json:"v"`
}

// encodeCursor mã hoá mốc keyset thành chuỗi mờ (base64url).
func encodeCursor(cursor ChannelVideoCursor) string {
	payload := channelVideoCursorJSON{Channel: cursor.ChannelID, Kinds: cursor.KindsKey, Video: cursor.VideoID}
	if cursor.PublishedAt != nil {
		ms := cursor.PublishedAt.UnixMilli()
		payload.At = &ms
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeCursor đọc con trỏ và kiểm nó thuộc đúng kênh + bộ lọc.
func decodeCursor(raw, channelID string, kinds []string) (*ChannelVideoCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalidCursor()
	}
	var payload channelVideoCursorJSON
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, invalidCursor()
	}
	if payload.Channel != channelID || payload.Kinds != kindsKey(kinds) || payload.Video == "" {
		return nil, invalidCursor()
	}
	cursor := ChannelVideoCursor{ChannelID: payload.Channel, KindsKey: payload.Kinds, VideoID: payload.Video}
	if payload.At != nil {
		at := time.UnixMilli(*payload.At).UTC()
		cursor.PublishedAt = &at
	}
	return &cursor, nil
}

// kindsKey là hình chuẩn của bộ lọc loại để nhét vào con trỏ.
func kindsKey(kinds []string) string {
	if len(kinds) == 0 {
		return ""
	}
	out := append([]string(nil), kinds...)
	sort.Strings(out)
	return strings.Join(out, ",")
}
