package web

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// Não dự án (D14) › Vũ trụ chung (GAP-44). Dữ liệu là Não truyện (GAP-39/49) của
// từng truyện người gọi được xem, đọc thẳng từ tệp engine như /brain; không có kho
// riêng. Truyện người gọi không xem được không góp vào bất kỳ con số hay thực thể nào.

// universeStoryRef là một truyện trong dự án mà thực thể/đoạn văn thuộc về.
// ID là mã truyện trong dự án (đoạn {s} của /api/projects/{p}/stories/{s}).
type universeStoryRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// universeEntity là một thực thể đã gộp giữa các truyện. Stories xếp truyện mới
// nhất trước; LastRole là vai (hoặc mô tả, với địa điểm/vật) ở truyện mới nhất.
type universeEntity struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Name     string             `json:"name"`
	Stories  []universeStoryRef `json:"stories"`
	LastRole string             `json:"lastRole"`
	// Kind là loại vai (lead/antagonist/ally/support, Gói S) ở truyện mới nhất có khai;
	// địa điểm/vật không có loại vai nên bỏ trống. Avatar ở bảng vũ trụ lấy màu từ đây.
	Kind domain.CharacterKind `json:"kind,omitempty"`

	// names là mọi tên/biệt danh của các thực thể đã gộp (ô Tìm trong dự án so trên đây).
	names []string
}

type universeCounts struct {
	Stories int `json:"stories"`
	Chars   int `json:"chars"`
	Places  int `json:"places"`
}

// universeCheck là một vi phạm Master §19 ("nhắc chuyện cũ tối đa hai câu, lồng
// vào hành động") do Biên tập viên phát với mã con `s19-recap` (GAP-36). Chỉ lấy
// từ các truyện có dùng ít nhất một thực thể chung. `checks: null` = chưa có lần
// chấm nào chạy lược đồ có `rule` (chưa kiểm được, FE hiện "—"); `[]` = đã chấm và
// sạch, để không báo Đạt sai.
type universeCheck struct {
	Ref     string `json:"ref"`
	Story   string `json:"story"`
	Ch      int    `json:"ch"`
	Text    string `json:"text"`
	QuoteID string `json:"quoteId"`
}

// universeKeptCheck là mục đã "Giữ nguyên": hình dạng của universeCheck cộng lý do
// và người/giờ quyết định (GAP-47, 06 › Bổ sung §J). Bản ghi cũ thiếu trường nào thì
// trường đó là "" — không suy ra, không lấy tên người gọi hiện tại.
type universeKeptCheck struct {
	universeCheck
	Reason    string `json:"reason"`
	DecidedBy string `json:"decidedBy"`
	DecidedAt string `json:"decidedAt"`
}

// Mã cố định của phép kiểm vũ trụ (câu 4): ref mục luật + mã con ổn định.
const (
	universeCheckRef  = "Master §19"
	universeCheckRule = "s19-recap"
)

type universeResponse struct {
	Name     string           `json:"name"`
	Counts   universeCounts   `json:"counts"`
	Entities []universeEntity `json:"entities"`
	Checks   []universeCheck  `json:"checks"` // nil → null: chưa có nguồn chấm Master §19

	// Kept là các mục Master §19 người dùng đã "Giữ nguyên" (GAP-47): không vào
	// Checks nhưng vẫn trả kèm, mang lý do và người/giờ quyết định, để dòng "N mục
	// đã giữ nguyên" mở được danh sách. Bỏ giữ xoá quyết định ⇒ mục về lại Checks.
	KeptCount int                 `json:"keptCount"`
	Kept      []universeKeptCheck `json:"kept"`
}

// projectStoryBrain là Não truyện của một truyện người gọi được xem.
type projectStoryBrain struct {
	ref    universeStoryRef
	dir    string
	reader *host.StoryReader
	brain  naoBrainResponse
}

// viewableProjectStories liệt kê các truyện của dự án mà người gọi được xem, theo
// thứ tự thư mục (tiền tố NN- = thứ tự tạo, cũ trước). Cùng điều kiện với danh sách
// truyện của dự án (user.CanRead).
func viewableProjectStories(ctx context.Context, lib library.Repository, user identity.User, p string) ([]library.Story, error) {
	actor := actorOf(user)
	dirs, err := lib.Stories(ctx, actor, p)
	if err != nil {
		return nil, err
	}
	stories := make([]library.Story, 0, len(dirs))
	for _, sd := range dirs {
		if !user.CanRead(sd.Project + "/" + sd.Name) {
			continue
		}
		if st, ok := lib.ResolveStory(ctx, actor, sd.Project, sd.Name); ok {
			stories = append(stories, st)
		}
	}
	return stories, nil
}

// storyDisplayTitle là tên truyện hiển thị: tên sách của engine (meta/book.json
// là nguồn sự thật duy nhất cho thông tin ra ngoài), rồi story.json, rồi tên
// thư mục. Truyện tạo từ brief có story.json.title = tên tạm (dòng đầu brief);
func (g *Gateway) storyDisplayTitle(ctx context.Context, lib library.Repository, user identity.User, st library.Story) string {
	if sr, err := g.openStoryReader(user, st); err == nil {
		if title := strings.TrimSpace(sr.BookTitle()); title != "" {
			return title
		}
	}
	if sf, err := lib.LoadStoryFile(ctx, actorOf(user), st); err == nil && sf != nil && sf.Title != "" {
		return sf.Title
	}
	return st.Story
}

// projectBrains đọc Não truyện của mọi truyện người gọi được xem trong dự án.
func (g *Gateway) projectBrains(ctx context.Context, lib library.Repository, user identity.User, p string) ([]projectStoryBrain, error) {
	stories, err := viewableProjectStories(ctx, lib, user, p)
	if err != nil {
		return nil, err
	}
	out := make([]projectStoryBrain, 0, len(stories))
	for _, st := range stories {
		sr, err := g.openStoryReader(user, st)
		if err != nil {
			return nil, err
		}
		brain, err := g.naoBrain(sr, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, projectStoryBrain{
			ref:    universeStoryRef{ID: st.Story, Title: g.storyDisplayTitle(ctx, lib, user, st)},
			dir:    st.Dir,
			reader: sr,
			brain:  brain,
		})
	}
	return out, nil
}

func (g *Gateway) serveProjectUniverse(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string, segs []string) {
	if len(segs) != 0 || r.Method != http.MethodGet {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	brains, err := g.projectBrains(r.Context(), lib, user, p)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := universeResponse{Name: projectUniverseName(r.Context(), lib, actorOf(user), p)}
	resp.Entities = []universeEntity{}
	inUniverse := map[string]bool{}
	for _, entity := range mergeProjectEntities(brains, "char", "place") {
		if len(entity.Stories) < 2 {
			continue
		}
		resp.Entities = append(resp.Entities, entity)
		for _, ref := range entity.Stories {
			inUniverse[ref.ID] = true
		}
		if entity.Type == "char" {
			resp.Counts.Chars++
		} else {
			resp.Counts.Places++
		}
	}
	resp.Counts.Stories = len(inUniverse)
	resp.Checks, resp.Kept, err = universeChecks(brains, inUniverse)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.KeptCount = len(resp.Kept)
	sort.SliceStable(resp.Entities, func(i, j int) bool {
		a, b := resp.Entities[i], resp.Entities[j]
		if len(a.Stories) != len(b.Stories) {
			return len(a.Stories) > len(b.Stories)
		}
		if a.Type != b.Type {
			return a.Type == "char"
		}
		return a.Name < b.Name
	})
	writeJSON(w, http.StatusOK, resp)
}

// projectUniverseName là tên vũ trụ chung: project.universeName, không có thì tên
// dự án; dự án ẩn danh "_" hoặc chưa đặt tên → "" (FE ẩn dòng tên).
func projectUniverseName(ctx context.Context, lib library.Repository, actor *library.Actor, p string) string {
	if p == library.LegacyProjectID {
		return ""
	}
	if pf, err := lib.ProjectFile(ctx, actor, p); err == nil && pf != nil {
		if name := strings.TrimSpace(pf.UniverseName); name != "" {
			return name
		}
		return pf.Name
	}
	return ""
}

// universeChecks gom vi phạm Master §19 › `s19-recap` của các truyện dùng chung ít
// nhất một thực thể (shared lấy từ thực thể đã gộp). Dùng đúng bộ chuyển mục của
// API thẩm duyệt (cả bài hiện tại lẫn reviews/rounds/), nên `quoteId` trùng mã mục
// ổn định của GAP-47 mà Trình đọc dùng để mở đoạn.
//
// Trả nil checks khi chưa có bài nào chạy lược đồ có `rule` (chưa kiểm được → JSON
// null); trả [] (khác nil) khi đã chấm mà sạch. Mục của các lần chấm cũ trùng mã bị
// gộp. Mục người dùng đã "Giữ nguyên" (GAP-47) không vào checks mà sang kept.
func universeChecks(brains []projectStoryBrain, shared map[string]bool) (checks []universeCheck, kept []universeKeptCheck, err error) {
	checks = []universeCheck{}
	kept = []universeKeptCheck{}
	evaluated := false
	seen := map[string]bool{}
	for _, sb := range brains {
		if !shared[sb.ref.ID] {
			continue
		}
		cards, err := newReviewReader(sb.dir, sb.reader).cards()
		if err != nil {
			return nil, nil, err
		}
		for _, card := range cards {
			dto := card.View
			add := func(items []reviewItemAPI) {
				for _, item := range items {
					if item.Ref != universeCheckRef || item.Rule != universeCheckRule {
						continue
					}
					// ID ổn định của GAP-47 chỉ duy nhất trong một truyện; hai truyện
					// có thể trùng nội dung nên khoá gộp phải kèm mã truyện.
					key := sb.ref.ID + "\x00" + item.ID
					if seen[key] {
						continue
					}
					seen[key] = true
					row := universeCheck{
						Ref:     item.Ref,
						Story:   sb.ref.ID,
						Ch:      dto.Chapter,
						Text:    item.Text,
						QuoteID: item.ID,
					}
					// Decision do reviewReader gắn từ reviews/decisions.json cùng
					// bộ đọc GAP-47 với API thẩm duyệt, không đọc lại tệp ở đây.
					if reviewItemKept(item) {
						kept = append(kept, universeKeptCheck{
							universeCheck: row,
							Reason:        item.Decision.Reason,
							DecidedBy:     item.Decision.By,
							DecidedAt:     item.Decision.At,
						})
						continue
					}
					checks = append(checks, row)
				}
			}
			// Vũ trụ chung chỉ nhận Master §19 › s19-recap (06-data-contract: checks
			// lọc theo rule). Thẻ xưng hô mang ref "Xưng hô" và không có rule nên
			// không bao giờ lọt qua điều kiện dưới đây.
			if dto.RuleAware {
				evaluated = true
				add(dto.Items)
			}
			for i := range dto.Previous {
				if !dto.Previous[i].RuleAware {
					continue
				}
				evaluated = true
				add(dto.Previous[i].Items)
			}
		}
	}
	if !evaluated {
		return nil, kept, nil
	}
	return checks, kept, nil
}

// entityKey chuẩn hoá tên/biệt danh để so giữa các truyện: NFC, chữ thường, gộp
// khoảng trắng. GIỮ dấu tiếng Việt: "Lan" và "Lân" là hai người khác nhau, nên không
// dùng cách bỏ dấu của ô tìm kiếm ở đây.
func entityKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(norm.NFC.String(name))), " ")
}

// mergeProjectEntities gộp thực thể cùng loại giữa các truyện: hai thực thể là một
// khi tên hoặc một biệt danh của chúng trùng nhau sau entityKey (gộp bắc cầu, ví dụ
// truyện 1 "Bà Tư" + biệt danh "Bà Tư bán xôi", truyện 3 gọi "Bà Tư bán xôi").
// Mã và tên lấy ở truyện xuất hiện sớm nhất để không đổi khi có truyện mới.
func mergeProjectEntities(brains []projectStoryBrain, types ...string) []universeEntity {
	type member struct {
		story int
		name  string
		role  string
		kind  string
	}
	type group struct {
		typ     string
		members []member
		names   []string
		parent  int
	}
	var groups []*group
	find := func(i int) int {
		for groups[i].parent != i {
			groups[i].parent = groups[groups[i].parent].parent
			i = groups[i].parent
		}
		return i
	}
	byKey := map[string]int{}
	for si, sb := range brains {
		for _, entity := range sb.brain.Entities {
			if !slices.Contains(types, entity.Type) {
				continue
			}
			keys, names := []string{}, []string{}
			for _, name := range append([]string{entity.Name}, entity.Aliases...) {
				if key := entityKey(name); key != "" && !slices.Contains(keys, key) {
					keys = append(keys, key)
					names = append(names, strings.TrimSpace(name))
				}
			}
			if len(keys) == 0 {
				continue
			}
			id := len(groups)
			groups = append(groups, &group{
				typ:     entity.Type,
				members: []member{{story: si, name: strings.TrimSpace(entity.Name), role: firstNonEmpty(entity.Role, entity.Desc), kind: entity.Kind}},
				names:   names,
				parent:  id,
			})
			for _, key := range keys {
				full := entity.Type + "\x00" + key
				if other, ok := byKey[full]; ok {
					a, b := find(other), find(id)
					if a != b {
						// Gốc là nhóm tạo trước (truyện sớm hơn) để tên/mã ổn định.
						if b < a {
							a, b = b, a
						}
						groups[b].parent = a
						groups[a].members = append(groups[a].members, groups[b].members...)
						groups[a].names = append(groups[a].names, groups[b].names...)
					}
					continue
				}
				byKey[full] = id
			}
		}
	}
	out := []universeEntity{}
	usedIDs := map[string]int{}
	for i, gr := range groups {
		if find(i) != i {
			continue
		}
		first := gr.members[0]
		id := gr.typ + ":" + naoEntitySlug(first.name)
		// Hai tên khác nhau chỉ vì dấu ("Lan"/"Lân") cho cùng slug: thêm hậu tố để mã không trùng.
		if n := usedIDs[id]; n > 0 {
			usedIDs[id] = n + 1
			id = id + "-" + strconv.Itoa(n+1)
		} else {
			usedIDs[id] = 1
		}
		entity := universeEntity{ID: id, Type: gr.typ, Name: first.name, names: gr.names}
		seen := map[int]bool{}
		latest := -1
		for _, m := range gr.members {
			seen[m.story] = true
			latest = max(latest, m.story)
		}
		for _, m := range gr.members {
			if m.story == latest && m.role != "" {
				entity.LastRole = m.role
				break
			}
		}
		// Loại vai: lấy ở truyện mới nhất có khai, lùi dần để truyện cũ chưa có `kind`
		// không làm mất màu avatar của thực thể dùng chung.
		for si := latest; si >= 0 && entity.Kind == ""; si-- {
			for _, m := range gr.members {
				if m.story == si && m.kind != "" {
					entity.Kind = domain.CharacterKind(m.kind)
					break
				}
			}
		}
		for si := len(brains) - 1; si >= 0; si-- {
			if seen[si] {
				entity.Stories = append(entity.Stories, brains[si].ref)
			}
		}
		out = append(out, entity)
	}
	return out
}
