package backdrop

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// wbiMixin là bảng trộn khoá ký của Bilibili (chép nguyên từ yt-factory
// src/server/backdrop/bilibili-api.ts) — đổi bảng là mọi chữ ký sai.
var wbiMixin = [64]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

// wbiEscape thay encodeURIComponent của JS: dấu !'()* không bị mã hoá, còn
// khoảng trắng thành %20 (Go dùng + cho query nên phải tự thoát).
func wbiEscape(value string) string {
	value = strings.NewReplacer(
		"!", "%21", "'", "%27", "(", "%28", ")", "%29", "*", "%2A",
	).Replace(value)
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

// SignWBI ký tham số của một lời gọi API Bilibili: sắp tên, ghép chuỗi truy
// vấn, thêm wts và w_rid = md5(chuỗi + mixin) (chuyển từ signWbi của
// bilibili-api.ts; vector kiểm có trong bilibili-api.test.ts).
func SignWBI(params map[string]string, imgKey, subKey string, timestamp int64) string {
	key := imgKey + subKey
	mixin := make([]byte, 0, 32)
	for _, index := range wbiMixin {
		if index < len(key) {
			mixin = append(mixin, key[index])
		}
	}
	if len(mixin) > 32 {
		mixin = mixin[:32]
	}
	values := make(map[string]string, len(params)+1)
	for name, value := range params {
		values[name] = value
	}
	values["wts"] = strconv.FormatInt(timestamp, 10)
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, wbiEscape(name)+"="+wbiEscape(stripWBIUnsafe(values[name])))
	}
	query := strings.Join(parts, "&")
	sum := md5.Sum([]byte(query + string(mixin)))
	return query + "&w_rid=" + hex.EncodeToString(sum[:])
}

// stripWBIUnsafe bỏ các ký tự JS encodeURIComponent không mã hoá nhưng phía
// Bilibili mong đợi bị bỏ khỏi giá trị (giữ đúng hành vi bản TS).
func stripWBIUnsafe(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		}
		return r
	}, value)
}
