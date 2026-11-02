package rivals

// Cursor keyset của trang video: base64url(JSON có phiên bản) — không ký.
// Đây là vị trí đọc do client giữ, không phải ranh giới tin cậy; thứ cần là
// kiểm hình dạng chặt để cursor rác/mismatch trả 400 invalid_cursor thay vì
// đọc sai trang.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Cursor là khoá keyset của dòng cuối một trang video — phần nội dung của
// cursor opaque trả ra API. Đúng một trong bốn trường khoá có nghĩa theo sort:
// Score (outlier), Views (views), Pos (newest của một kênh), Published
// (newest gộp kênh). Khoá nil = con trỏ đang ở đuôi NULL của thứ tự đó
// (trang trước kết thúc bằng dòng có giá trị NULL).
type Cursor struct {
	Sort      VideoSort
	Score     *float64
	Views     *int64
	Pos       *int64
	Published *time.Time
	ID        string // khoá phụ cuối của mọi sort
}

// cursorVersion là phiên bản hình JSON; đổi hình thì tăng để cursor cũ bị từ.
const cursorVersion = 2

// cursorMaxLen chặn cursor dài bất thường trước khi giải mã.
const cursorMaxLen = 512

// cursorSig là dấu vân tay (sha256/8 byte hex) của bộ lọc: cursor của truy vấn
// khác bị coi là không hợp lệ thay vì lặng lẽ đọc sai trang. Băm chỉ để gọn và
// tránh ký tự điều khiển trong JSON — không phải ranh giới tin cậy.
func cursorSig(q PageQuery) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%t\x00%d\x00%s\x00%s\x00%s\x00%t\x00%t",
		q.ChannelID, q.Sort, q.Long, q.WithinDays, q.Pattern,
		q.MadeFor, q.NotMadeFor, q.MadeAny, q.NotMadeAny)))
	return hex.EncodeToString(sum[:8])
}

// cursorPayload là hình JSON bên trong cursor opaque. Views/Pos dùng
// json.Number để từ chối số lẻ thay vì lặng lẽ cắt.
type cursorPayload struct {
	V         int          `json:"v"`
	Sort      VideoSort    `json:"s"`
	Score     *float64     `json:"sc,omitempty"`
	Views     *json.Number `json:"vw,omitempty"`
	Pos       *json.Number `json:"po,omitempty"`
	Published *time.Time   `json:"ts,omitempty"`
	ID        string       `json:"id"`
	Sig       string       `json:"g"`
}

// cursorOf dựng cursor của trang sau từ dòng cuối trang hiện tại.
func cursorOf(q PageQuery, last domain.RivalVideo) *Cursor {
	c := &Cursor{Sort: q.Sort, ID: last.ID}
	switch q.Sort {
	case SortViews:
		c.Views = last.ViewCount
	case SortNewest:
		if q.ChannelID == "" {
			c.Published = publishedKey(last)
		} else {
			pos := last.Position
			c.Pos = &pos
		}
	default:
		c.Score = last.Score
	}
	return c
}

// publishedKey là khoá "mới nhất" gộp kênh: giờ đăng chính xác rồi ước lượng.
func publishedKey(v domain.RivalVideo) *time.Time {
	if v.UploadedAt != nil {
		return v.UploadedAt
	}
	return v.PublishedAt
}

// likePattern escape %, _ và \ của chuỗi tìm rồi bọc %…%; rỗng → "". Store
// gấp mẫu (hoa thường, dấu) bằng cùng hàm với cột tiêu đề trước khi so.
func likePattern(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(query) + "%"
}

// encode ghi cursor ra chuỗi opaque base64url(JSON).
func (c *Cursor) encode(sig string) (string, error) {
	p := cursorPayload{
		V: cursorVersion, Sort: c.Sort, Score: c.Score, Published: c.Published,
		ID: c.ID, Sig: sig,
	}
	if c.Views != nil {
		n := json.Number(fmt.Sprintf("%d", *c.Views))
		p.Views = &n
	}
	if c.Pos != nil {
		n := json.Number(fmt.Sprintf("%d", *c.Pos))
		p.Pos = &n
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeCursor giải và kiểm một cursor nhận được: độ dài, base64, phiên bản,
// sort của payload phải khớp sort đang yêu cầu, hình khoá theo sort, giá trị
// hữu hạn/nguyên, id video hợp lệ, và dấu vân tay bộ lọc. Sai bất kỳ bước nào
// → ErrInvalidCursor.
func decodeCursor(raw, sig string, expectedSort VideoSort, scopeHasChannel bool) (*Cursor, error) {
	if raw == "" || len(raw) > cursorMaxLen || strings.ContainsAny(raw, " \t\r\n") {
		return nil, ErrInvalidCursor
	}
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var p cursorPayload
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, ErrInvalidCursor
	}
	// More() trả false ở dấu đóng ("}"/"]") của trường hợp sai lệch nhưng nó
	// KHÔNG chặn dữ liệu thừa sau JSON — phải decode lần hai và đòi EOF.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, ErrInvalidCursor
	}
	if p.V != cursorVersion || p.Sig != sig || p.Sort != expectedSort ||
		!validVideoID(p.ID) || !videoSorts[p.Sort] {
		return nil, ErrInvalidCursor
	}
	c := &Cursor{Sort: p.Sort, ID: p.ID}
	switch p.Sort {
	case SortOutlier:
		if p.Views != nil || p.Pos != nil || p.Published != nil {
			return nil, ErrInvalidCursor
		}
		if p.Score != nil && (math.IsNaN(*p.Score) || math.IsInf(*p.Score, 0)) {
			return nil, ErrInvalidCursor
		}
		c.Score = p.Score
	case SortViews:
		if p.Score != nil || p.Pos != nil || p.Published != nil {
			return nil, ErrInvalidCursor
		}
		if c.Views, err = optionalNonNegInt(p.Views); err != nil {
			return nil, err
		}
	case SortNewest:
		if p.Score != nil || p.Views != nil {
			return nil, ErrInvalidCursor
		}
		if scopeHasChannel {
			if p.Published != nil || p.Pos == nil {
				return nil, ErrInvalidCursor
			}
			if c.Pos, err = optionalSignedInt(p.Pos); err != nil {
				return nil, err
			}
		} else {
			if p.Pos != nil {
				return nil, ErrInvalidCursor
			}
			if p.Published != nil && p.Published.IsZero() {
				return nil, ErrInvalidCursor
			}
			c.Published = p.Published
		}
	}
	return c, nil
}

// optionalSignedInt từ chối số lẻ/không phải số nguyên nhưng CHẤP NHẬN âm:
// vị trí video của một kênh là có dấu — discovery khoá một khối ngay dưới
// vị trí nhỏ nhất (min - DiscoveryBlock), nên video mới mang position âm.
func optionalSignedInt(n *json.Number) (*int64, error) {
	if n == nil {
		return nil, nil
	}
	v, err := n.Int64()
	if err != nil {
		return nil, ErrInvalidCursor
	}
	return &v, nil
}

// optionalNonNegInt như optionalSignedInt nhưng từ chối âm — dành cho view
// count (vùng giá trị không âm).
func optionalNonNegInt(n *json.Number) (*int64, error) {
	v, err := optionalSignedInt(n)
	if err != nil || v == nil {
		return v, err
	}
	if *v < 0 {
		return nil, ErrInvalidCursor
	}
	return v, nil
}

// validVideoID kiểm id video YouTube 11 ký tự [A-Za-z0-9_-].
func validVideoID(id string) bool {
	if len(id) != 11 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
