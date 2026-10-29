package web

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

type searchBody struct {
	Entities []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Stories []struct {
			ID string `json:"id"`
		} `json:"stories"`
	} `json:"entities"`
	Passages []struct {
		Story      string   `json:"story"`
		StoryTitle string   `json:"storyTitle"`
		Ch         int      `json:"ch"`
		Quote      string   `json:"quote"`
		HL         [][2]int `json:"hl"`
	} `json:"passages"`
	Total int `json:"total"`
}

func doSearch(t *testing.T, tg *testGateway, cookie, q string) (searchBody, string) {
	t.Helper()
	w := tg.do("GET", "/api/projects/_/search?q="+url.QueryEscape(q), cookie, "")
	if w.Code != 200 {
		t.Fatalf("search %q: %d %s", q, w.Code, w.Body.String())
	}
	var body searchBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("search body %q: %v", w.Body.String(), err)
	}
	return body, w.Body.String()
}

// utf16Slice cắt quote như String.prototype.slice của trình duyệt.
func utf16Slice(s string, start, end int) string {
	units := utf16.Encode([]rune(s))
	if start < 0 || end > len(units) || start > end {
		return "<ngoài phạm vi>"
	}
	return string(utf16.Decode(units[start:end]))
}

// TestProjectSearchFoldsDiacriticsAndHighlightsUTF16: tìm không phân biệt dấu và
// hoa/thường; hl là vị trí UTF-16 trong quote (emoji = 2 đơn vị); đoạn dài cắt cửa
// sổ có "…"; truyện không được xem không bao giờ có mặt.
func TestProjectSearchFoldsDiacriticsAndHighlightsUTF16(t *testing.T) {
	tg := newTestGateway(t)
	long := strings.Repeat("Mưa rơi trên mái tôn suốt đêm dài. ", 12) + "Cuối cùng đoạn ghi âm vang lên."
	projectBrainFixture(t, tg, map[string][]string{
		"oldbook": {"# Chương 1\n\n🎙 Tôi bấm nút phát. Đoạn ghi âm bảy mươi ba giây vang lên.\n\nKhông có gì.", long},
		"secret":  {"Bản ghi âm bí mật trong truyện ẩn."},
		"third":   {"Chị nói: “Nghe đoạn GHI AM này đi.” Rồi ghi âm lại lần nữa."},
	})
	moveStoryToUnassignedProject(t, tg.root, filepath.Join(tg.root, "secret"))
	reader := tg.login("reader", "reader-pass")

	for _, q := range []string{"ghi âm", "GHI AM", "  ghi   am "} {
		body, raw := doSearch(t, tg, reader, q)
		if strings.Contains(raw, "secret") || strings.Contains(raw, "bí mật") {
			t.Fatalf("lộ truyện ẩn với %q: %s", q, raw)
		}
		if body.Total != 3 || len(body.Passages) != 3 {
			t.Fatalf("q=%q passages = %s", q, raw)
		}
		for _, p := range body.Passages {
			if len(p.HL) == 0 {
				t.Fatalf("q=%q thiếu hl: %+v", q, p)
			}
			for _, hl := range p.HL {
				if got := strings.ToLower(utf16Slice(p.Quote, hl[0], hl[1])); got != "ghi âm" && got != "ghi am" {
					t.Fatalf("q=%q hl %v của %q cắt ra %q", q, hl, p.Quote, got)
				}
			}
		}
	}

	body, raw := doSearch(t, tg, reader, "ghi âm")
	// Hai chỗ khớp ở "third" xếp trước; rồi đoạn có emoji; đoạn dài sau cùng (cùng truyện, chương sau).
	first, second, third := body.Passages[0], body.Passages[1], body.Passages[2]
	if first.Story != "third" || first.Ch != 1 || len(first.HL) != 2 {
		t.Fatalf("thứ tự/độ khớp: %s", raw)
	}
	if second.Story != "oldbook" || second.Ch != 1 || second.Quote != "🎙 Tôi bấm nút phát. Đoạn ghi âm bảy mươi ba giây vang lên." {
		t.Fatalf("đoạn emoji: %s", raw)
	}
	// "🎙 Tôi bấm nút phát. Đoạn " = 25 rune nhưng 26 đơn vị UTF-16 (emoji ngoài BMP).
	if len(second.HL) != 1 || second.HL[0] != [2]int{26, 32} {
		t.Fatalf("hl UTF-16 = %v", second.HL)
	}
	if third.Story != "oldbook" || third.Ch != 2 || !strings.HasPrefix(third.Quote, "…") ||
		[]rune(third.Quote)[0] != '…' || len([]rune(third.Quote)) > searchQuoteRunes+2 || !strings.HasSuffix(third.Quote, "vang lên.") {
		t.Fatalf("cửa sổ đoạn dài: %q", third.Quote)
	}
	if third.StoryTitle == "" || first.StoryTitle == "" {
		t.Fatalf("thiếu tên truyện: %s", raw)
	}
}

// TestProjectSearchEntitiesAndVisibility: thực thể khớp theo tên/biệt danh không dấu,
// danh sách truyện của thực thể bỏ truyện người gọi không được xem.
func TestProjectSearchEntitiesAndVisibility(t *testing.T) {
	tg := newTestGateway(t)
	projectBrainFixture(t, tg, nil)
	reader := tg.login("reader", "reader-pass")
	admin := tg.login("admin", "admin-pass")

	body, raw := doSearch(t, tg, reader, "ban xoi")
	if len(body.Entities) != 1 || body.Entities[0].ID != "char:ba-tu" || len(body.Entities[0].Stories) != 3 ||
		body.Entities[0].Stories[0].ID != "third" || body.Entities[0].Stories[1].ID != "secret" || body.Entities[0].Stories[2].ID != "oldbook" {
		t.Fatalf("reader entities: %s", raw)
	}
	if body, raw := doSearch(t, tg, reader, "mat vu"); len(body.Entities) != 1 {
		t.Fatalf("view phải xem được mọi truyện cùng dự án: %s", raw)
	}
	if body, raw := doSearch(t, tg, admin, "mat vu"); len(body.Entities) != 1 || body.Entities[0].Stories[0].ID != "secret" {
		t.Fatalf("admin thấy truyện ẩn: %s", raw)
	}
	if body, raw := doSearch(t, tg, reader, "cho van"); len(body.Entities) != 1 || body.Entities[0].Type != "place" ||
		len(body.Entities[0].Stories) != 2 || body.Entities[0].Stories[0].ID != "secret" || body.Entities[0].Stories[1].ID != "oldbook" {
		t.Fatalf("địa điểm: %s", raw)
	}
}

// TestProjectSearchCapsAndRejectsEmpty: tối đa searchPassageLimit đoạn (total giữ số
// thật); q rỗng/chỉ khoảng trắng → 400.
func TestProjectSearchCapsAndRejectsEmpty(t *testing.T) {
	tg := newTestGateway(t)
	paragraphs := make([]string, 0, 60)
	for range 60 {
		paragraphs = append(paragraphs, "Đoạn ghi âm số mấy cũng được.")
	}
	projectBrainFixture(t, tg, map[string][]string{"third": {strings.Join(paragraphs, "\n\n")}})
	reader := tg.login("reader", "reader-pass")

	body, raw := doSearch(t, tg, reader, "ghi am")
	if len(body.Passages) != searchPassageLimit || body.Total != 60 {
		t.Fatalf("cap: %d/%d %s", len(body.Passages), body.Total, raw[:min(len(raw), 300)])
	}
	for _, q := range []string{"", "   ", strings.Repeat("a", searchQueryMaxRunes+1)} {
		if w := tg.do("GET", "/api/projects/_/search?q="+url.QueryEscape(q), reader, ""); w.Code != 400 {
			t.Fatalf("q=%q → %d %s", q, w.Code, w.Body.String())
		}
	}
	if w := tg.do("GET", "/api/projects/khong-co/search?q=a", reader, ""); w.Code != 403 {
		t.Fatalf("dự án không có: %d", w.Code)
	}
}
