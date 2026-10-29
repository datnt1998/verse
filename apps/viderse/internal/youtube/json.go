// Package youtube là client InnerTube (web) của YouTube: phân giải kênh, đọc
// trang video của kênh theo thứ tự "Mới nhất", đọc con trỏ phân trang và giờ
// đăng chính xác của video. Mọi yêu cầu HTTP đi qua một bộ điều tốc (Pacer)
// dùng chung và một *http.Client được tiêm vào, nên test thay transport bằng
// httptest được. Không có trạng thái toàn cục: một *Client tự chứa session,
// bộ điều tốc và đồng hồ của nó.
//
// Port từ yt-factory src/server/youtube/{client,channel,channel-pages,
// parse,upload-time}.ts: thư viện youtubei.js được thay bằng một lớp đọc JSON
// tối thiểu chỉ hiểu các renderer mà luồng quét cần (tab, chip lọc, lockup
// video, continuation).
package youtube

import (
	"sort"
	"strings"
)

// jmap là một object JSON lỏng. Toàn bộ parser đọc DOM thô của InnerTube qua
// các hàm dưới đây thay vì struct cố định, vì renderer của YouTube đổi khoá
// liên tục: phần chưa biết phải bị bỏ qua, phần thiếu thành nil.
type jmap = map[string]any

// jobj ép một giá trị JSON về object; các kiểu khác thành nil.
func jobj(v any) jmap {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// jarr ép một giá trị JSON về mảng; các kiểu khác thành nil.
func jarr(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// jstr ép một giá trị JSON về chuỗi; các kiểu khác thành "".
func jstr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// jget đọc một đường khoá trong object; chặn nào không phải object thì trả nil.
func jget(v any, keys ...string) any {
	cur := v
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

// jsortedKeys trả tên khoá theo thứ tự bảng chữ để DFS ổn định giữa các lần chạy
// (map của Go có thứ tự duyệt ngẫu nhiên). Thứ tự video vẫn đúng vì mọi danh
// sách renderer đều nằm trong mảng.
func jsortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// walkDFS duyệt cây JSON theo chiều sâu; con của object đi theo thứ tự khoá đã
// sắp, con của mảng theo thứ tự mảng. Trả false từ fn dừng toàn bộ lượt duyệt.
func walkDFS(v any, fn func(node jmap) bool) bool {
	switch n := v.(type) {
	case map[string]any:
		if !fn(n) {
			return false
		}
		for _, k := range jsortedKeys(n) {
			if !walkDFS(n[k], fn) {
				return false
			}
		}
	case []any:
		for _, item := range n {
			if !walkDFS(item, fn) {
				return false
			}
		}
	}
	return true
}

// collectNodes tìm mọi object con có khoá wrapperKey (một renderer/viewModel)
// và trả phần thân bên trong, theo thứ tự duyệt.
func collectNodes(root any, wrapperKey string) []jmap {
	var out []jmap
	walkDFS(root, func(node jmap) bool {
		if inner, ok := node[wrapperKey]; ok {
			if m := jobj(inner); m != nil {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}

// textOf đọc chữ hiển thị của một nút text InnerTube: dạng attributed
// ({content}), dạng runs ({runs:[{text}]}) hoặc {simpleText}. Thiếu chữ là "".
func textOf(v any) string {
	m := jobj(v)
	if m == nil {
		return ""
	}
	if s := jstr(m["content"]); s != "" {
		return s
	}
	if s := jstr(m["simpleText"]); s != "" {
		return s
	}
	var b strings.Builder
	for _, run := range jarr(m["runs"]) {
		b.WriteString(jstr(jobj(run)["text"]))
	}
	return b.String()
}
