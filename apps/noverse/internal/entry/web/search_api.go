package web

import (
	"net/http"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// Não dự án (D14) › Tìm trong dự án (GAP-45): một ô tìm cho mọi truyện người gọi
// được xem. So khớp không phân biệt dấu tiếng Việt và hoa/thường; vị trí tô sáng
// tính theo đơn vị mã UTF-16 trong `quote` (khớp String.slice của trình duyệt).

const (
	searchQueryMaxRunes = 200
	searchPassageLimit  = 50
	searchEntityLimit   = 50
	// searchQuoteRunes là độ dài tối đa của trích đoạn; đoạn văn dài hơn thì cắt một
	// cửa sổ quanh chỗ khớp đầu tiên, giữ searchQuoteLead ký tự phía trước.
	searchQuoteRunes = 240
	searchQuoteLead  = 80
	searchEllipsis   = '…'
)

type searchPassage struct {
	Story      string   `json:"story"`
	StoryTitle string   `json:"storyTitle"`
	Ch         int      `json:"ch"`
	Quote      string   `json:"quote"`
	HL         [][2]int `json:"hl"`

	exact, hits, story, para int
}

type searchResponse struct {
	Entities []universeEntity `json:"entities"`
	Passages []searchPassage  `json:"passages"`
	// Total là số đoạn khớp trước khi cắt còn searchPassageLimit.
	Total int `json:"total"`
}

func (g *Gateway) serveProjectSearch(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string, segs []string) {
	if len(segs) != 0 || r.Method != http.MethodGet {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		jsonError(w, http.StatusBadRequest, "Nhập chữ cần tìm")
		return
	}
	if utf8.RuneCountInString(q) > searchQueryMaxRunes {
		jsonError(w, http.StatusBadRequest, "Chữ cần tìm quá dài")
		return
	}
	needle, _ := searchFold(q)
	if len(needle) == 0 {
		jsonError(w, http.StatusBadRequest, "Nhập chữ cần tìm")
		return
	}
	exact := searchExactForm(q)
	brains, err := g.projectBrains(r.Context(), lib, user, p)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := searchResponse{Entities: searchEntities(brains, needle), Passages: []searchPassage{}}
	for si, sb := range brains {
		progress, err := sb.reader.Progress()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if progress == nil {
			continue
		}
		chapters := slices.Clone(progress.CompletedChapters)
		slices.Sort(chapters)
		for _, ch := range slices.Compact(chapters) {
			if ch <= 0 {
				continue
			}
			text, err := sb.reader.Store().Drafts.LoadChapterText(ch)
			if err != nil {
				jsonError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for pi, para := range searchParagraphs(text) {
				passage, ok := searchMatch(para, needle, exact)
				if !ok {
					continue
				}
				passage.Story, passage.StoryTitle, passage.Ch = sb.ref.ID, sb.ref.Title, ch
				passage.story, passage.para = si, pi
				resp.Passages = append(resp.Passages, passage)
			}
		}
	}
	resp.Total = len(resp.Passages)
	// "Kết quả xếp theo độ khớp chữ": khớp đúng cả dấu trước, rồi nhiều chỗ khớp,
	// rồi truyện mới trước, chương và đoạn theo thứ tự đọc.
	sort.SliceStable(resp.Passages, func(i, j int) bool {
		a, b := resp.Passages[i], resp.Passages[j]
		switch {
		case a.exact != b.exact:
			return a.exact > b.exact
		case a.hits != b.hits:
			return a.hits > b.hits
		case a.story != b.story:
			return a.story > b.story
		case a.Ch != b.Ch:
			return a.Ch < b.Ch
		}
		return a.para < b.para
	})
	if len(resp.Passages) > searchPassageLimit {
		resp.Passages = resp.Passages[:searchPassageLimit]
	}
	writeJSON(w, http.StatusOK, resp)
}

// searchEntities trả nhân vật, địa điểm, vật (đã gộp giữa các truyện như Vũ trụ
// chung) có tên hoặc biệt danh chứa chữ cần tìm, không phân biệt dấu.
func searchEntities(brains []projectStoryBrain, needle []rune) []universeEntity {
	out := []universeEntity{}
	for _, entity := range mergeProjectEntities(brains, "char", "place", "obj") {
		if len(out) == searchEntityLimit {
			break
		}
		if slices.ContainsFunc(entity.names, func(name string) bool {
			folded, _ := searchFold(name)
			return searchIndex(folded, needle, 0) >= 0
		}) {
			out = append(out, entity)
		}
	}
	return out
}

// searchParagraphs tách chính văn thành đoạn như trình đọc: mỗi dòng không rỗng là
// một đoạn; dòng tiêu đề Markdown ("# …") và dòng ngắt cảnh không phải đoạn văn.
func searchParagraphs(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.Trim(line, "-*_ ") == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// searchFold bỏ dấu, đưa về chữ thường, đ→d và gộp khoảng trắng; src[i] là chỉ số
// rune trong chuỗi gốc sinh ra rune thứ i của kết quả.
func searchFold(s string) (folded []rune, src []int) {
	space := true
	for i, r := range []rune(s) {
		if unicode.IsSpace(r) {
			if !space {
				folded, src = append(folded, ' '), append(src, i)
			}
			space = true
			continue
		}
		space = false
		if r == 'đ' || r == 'Đ' {
			folded, src = append(folded, 'd'), append(src, i)
			continue
		}
		for _, d := range norm.NFD.String(string(r)) {
			if unicode.Is(unicode.Mn, d) {
				continue
			}
			folded, src = append(folded, unicode.ToLower(d)), append(src, i)
		}
	}
	if n := len(folded); n > 0 && folded[n-1] == ' ' {
		folded, src = folded[:n-1], src[:n-1]
	}
	return folded, src
}

// searchExactForm là dạng so "khớp đúng cả dấu": NFC, chữ thường, gộp khoảng trắng.
func searchExactForm(s string) string {
	return entityKey(s)
}

func searchIndex(haystack, needle []rune, from int) int {
	for i := from; i+len(needle) <= len(haystack); i++ {
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// searchMatch tìm mọi chỗ khớp (không chồng nhau) trong một đoạn và dựng trích đoạn.
func searchMatch(para string, needle []rune, exact string) (searchPassage, bool) {
	para = norm.NFC.String(para)
	runes := []rune(para)
	folded, src := searchFold(para)
	var spans [][2]int // theo chỉ số rune của đoạn gốc
	passage := searchPassage{}
	for at := searchIndex(folded, needle, 0); at >= 0; at = searchIndex(folded, needle, at+len(needle)) {
		start, end := src[at], src[at+len(needle)-1]+1
		// Dấu rời (chuỗi chưa dựng sẵn) đi theo chữ cuối cùng của chỗ khớp.
		for end < len(runes) && unicode.Is(unicode.Mn, runes[end]) {
			end++
		}
		spans = append(spans, [2]int{start, end})
		if entityKey(string(runes[start:end])) == exact {
			passage.exact++
		}
	}
	if len(spans) == 0 {
		return passage, false
	}
	passage.hits = len(spans)
	from, to := 0, len(runes)
	if len(runes) > searchQuoteRunes {
		from = max(0, spans[0][0]-searchQuoteLead)
		to = min(len(runes), from+searchQuoteRunes)
		from = max(0, to-searchQuoteRunes)
	}
	quote := slices.Clone(runes[from:to])
	shift := -from
	if from > 0 {
		quote = append([]rune{searchEllipsis}, quote...)
		shift++
	}
	if to < len(runes) {
		quote = append(quote, searchEllipsis)
	}
	// offset16[i] = số đơn vị UTF-16 trước rune thứ i của quote.
	offset16 := make([]int, len(quote)+1)
	for i, r := range quote {
		offset16[i+1] = offset16[i] + max(1, utf16.RuneLen(r))
	}
	passage.Quote = string(quote)
	passage.HL = [][2]int{}
	for _, span := range spans {
		if span[0] < from || span[1] > to {
			continue
		}
		passage.HL = append(passage.HL, [2]int{offset16[span[0]+shift], offset16[span[1]+shift]})
	}
	return passage, true
}
