// Package comments stores review feedback independently of the engine's story lease.
package comments

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrInvalid = errors.New("Góp ý không hợp lệ")
var ErrNotFound = errors.New("Không tìm thấy góp ý")

var Chips = []string{"Ngắn lại", "Thêm thoại", "Kịch tính hơn", "Giữ giọng văn", "Chậm nhịp lại"}

// Trạng thái vòng đời góp ý (§AI trong 06-data-contract.md).
const (
	StatusPending = "pending" // chờ gửi
	StatusSent    = "sent"    // đã chuyển cho Người viết
	StatusFixed   = "fixed"   // Người viết báo đã sửa, hoặc trích gốc mất và thấy đoạn thay
	StatusLost    = "lost"    // không tìm thấy chỗ neo sau khi chương đổi
	StatusUnsure  = "unsure"  // nhiều đoạn có thể là chỗ neo
	StatusDone    = "done"    // đã xử lý xong
)

// FixedBy ghi nguồn chuyển sang Đã sửa.
const (
	FixedByAgent     = "agent"      // writer trả fixed_comments
	FixedByQuoteGone = "quote_gone" // trích gốc biến mất, tìm được đoạn thay
)

// Anchor là mốc neo hiện tại của góp ý: trích đang khớp + ngữ cảnh trước/sau.
// Comment.Paragraph/Comment.Quote giữ vị trí gốc bất biến; Anchor theo nội dung.
type Anchor struct {
	Paragraph int    `json:"paragraph"`
	Quote     string `json:"quote"`
	Prefix    string `json:"prefix,omitempty"`
	Suffix    string `json:"suffix,omitempty"`
}

// Candidate là một đoạn gợi ý khi vị trí Không chắc (tối đa 3).
type Candidate struct {
	Paragraph int    `json:"paragraph"`
	Snippet   string `json:"snippet"`
}

// Counts tóm tắt số góp ý cần chú ý của một chương (hiện trên danh sách chương).
type Counts struct {
	Need    int `json:"need"`
	Pending int `json:"pending"`
}

type Comment struct {
	ID        string     `json:"id"`
	Paragraph *int       `json:"paragraph"`
	Quote     string     `json:"quote"`
	Chips     []string   `json:"chips"`
	Text      string     `json:"text"`
	Author    string     `json:"author"`
	CreatedAt time.Time  `json:"createdAt"`
	Status    string     `json:"status"`
	SentAt    *time.Time `json:"sentAt,omitempty"`
	FixedAt   *time.Time `json:"fixedAt,omitempty"`
	// ResolvedAt: thời điểm chuyển Đã xử lý (PATCH resolve hoặc duyệt kế tiếp).
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	FixedBy    string     `json:"fixedBy,omitempty"`
	// Unfixed: đã gửi cho Người viết nhưng chỗ đó chưa được sửa.
	Unfixed bool `json:"unfixed,omitempty"`
	// Moved: vị trí chỉ tìm được bằng khớp gần đúng (badge nét đứt).
	Moved bool `json:"moved,omitempty"`
	// Relinked: người dùng đã gắn lại vị trí thủ công.
	Relinked bool `json:"relinked,omitempty"`
	// Whole: góp ý cho cả chương (kèm trích gốc làm bối cảnh).
	Whole bool `json:"whole,omitempty"`
	// NewQuote: câu sau khi sửa, hiện cạnh trích gốc ở thẻ Đã sửa (Trước/Sau).
	NewQuote string `json:"newQuote,omitempty"`
	// Candidates: các đoạn gợi ý khi status==unsure.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Anchor: đoạn đã giải hiện tại; vắng khi chưa giải được. FE gắn badge theo
	// anchor.paragraph, không bao giờ theo paragraph gốc.
	Anchor *Anchor `json:"anchor,omitempty"`
}

type Input struct {
	Paragraph *int     `json:"paragraph"`
	Quote     string   `json:"quote"`
	Chips     []string `json:"chips"`
	Text      string   `json:"text"`
}

// ReanchorEvent là khung sự kiện comments.reanchored (SSE) cho một lần ghi
// chương: số góp ý mới Đã sửa / chưa sửa / cần gắn lại sau lần ghi đó.
type ReanchorEvent struct {
	Chapter int `json:"chapter"`
	Fixed   int `json:"fixed"`
	Unfixed int `json:"unfixed"`
	Need    int `json:"need"`
}

// PatchInput là thao tác PATCH trên một góp ý (§AI).
type PatchInput struct {
	Action    string `json:"action"`
	Paragraph *int   `json:"paragraph,omitempty"`
	Quote     string `json:"quote,omitempty"`
}

const (
	ActionResolve     = "resolve"
	ActionReopen      = "reopen"
	ActionRelink      = "relink"
	ActionResendWhole = "resend_whole"
)

// Store giữ nghiệp vụ góp ý (validate, vòng đời, neo mốc) trên một Repository
// lưu trữ — file backend theo thư mục <truyện>/.ainovel/gop-y, backend
// postgres theo project+story scope qua bảng comments.
type Store struct {
	repo Repository
}

// New tạo kho góp ý từ thư mục truyện (engine luôn là output/novel bên trong).
func New(storyDir string) *Store {
	return &Store{repo: newFileRepository(storyDir, filepath.Join(storyDir, "output", "novel"))}
}

// NewForEngine tạo kho góp ý từ thư mục engine (output/novel); dùng trong các
// công cụ ghi chương, nơi chỉ biết output dir chứ không biết story dir.
func NewForEngine(engineDir string) *Store {
	return &Store{repo: newFileRepository(filepath.Dir(filepath.Dir(engineDir)), engineDir)}
}

// NewFromRepository gắn facade lên một backend ngoài (postgres). Facade giữ
// nguyên nghiệp vụ nên contract file/DB không tách nhau.
func NewFromRepository(repo Repository) *Store {
	return &Store{repo: repo}
}

// AfterWrite chạy toàn bộ nghiệp vụ góp ý sau một lần ghi chương: đánh dấu các
// id writer báo đã sửa (fixed_comments), gắn lại mọi mốc neo trên nội dung mới
// và xếp sự kiện comments.reanchored để gateway phát. Không có góp ý nào thì
// im lặng. Trả về sự kiện cho chương vừa ghi (hoặc zero khi không có gì đổi).
func AfterWrite(engineDir string, chapter int, content string, fixedCommentIDs []string) (ReanchorEvent, error) {
	return NewForEngine(engineDir).AfterWriteAt(chapter, content, fixedCommentIDs)
}

// AfterWriteAt là AfterWrite trên Store bất kỳ backend — cùng luồng nghiệp vụ
// trên repository đã gắn.
func (s *Store) AfterWriteAt(chapter int, content string, fixedCommentIDs []string) (ReanchorEvent, error) {
	if !s.hasComments() {
		return ReanchorEvent{}, nil
	}
	marked := 0
	if len(fixedCommentIDs) > 0 {
		m, err := s.MarkFixed(chapter, fixedCommentIDs)
		if err != nil {
			return ReanchorEvent{}, err
		}
		marked = m
	}
	return s.reanchorAndPublish(chapter, content, marked)
}

// hasComments kiểm tra đã có góp ý nào trong truyện — các công cụ ghi chương
// gọi hook này cho mọi truyện, kể cả truyện chưa từng có góp ý.
func (s *Store) hasComments() bool {
	chapters, err := s.repo.Chapters()
	return err == nil && len(chapters) > 0
}

// commentFile là một bản ghi tệp; appliedAt của định dạng cũ được đọc để
// migrate sang status=sent + sentAt rồi bỏ khỏi bản ghi mới.
type commentFile struct {
	Comment
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
}

func (c Comment) anchorEligible() bool {
	if c.Paragraph == nil && c.Quote == "" {
		return false
	}
	if c.Whole {
		return false
	}
	return normalizeText(c.Quote) != "" || c.Anchor != nil
}

func (c Comment) displayStatus() string {
	if c.Status == "" {
		return StatusPending
	}
	return c.Status
}

func commentsEqual(a, b Comment) bool {
	if a.Status != b.Status || a.Moved != b.Moved || a.Whole != b.Whole ||
		a.Unfixed != b.Unfixed || a.NewQuote != b.NewQuote || a.FixedBy != b.FixedBy ||
		a.Relinked != b.Relinked {
		return false
	}
	if (a.Anchor == nil) != (b.Anchor == nil) {
		return false
	}
	if a.Anchor != nil && *a.Anchor != *b.Anchor {
		return false
	}
	if len(a.Candidates) != len(b.Candidates) {
		return false
	}
	for i := range a.Candidates {
		if a.Candidates[i] != b.Candidates[i] {
			return false
		}
	}
	return true
}

// applyReadResolution giải mốc neo ở đường đọc cho các trạng thái thuộc người
// dùng (pending/lost/unsure): khớp nguyên văn → pending (+dời), gần đúng duy
// nhất → pending + moved, nhiều → unsure, không → lost.
func applyReadResolution(c *Comment, paras []string) {
	hint := -1
	if c.Paragraph != nil {
		hint = *c.Paragraph
	}
	res := resolveAnchor(paras, c.Quote, hint, c.Anchor)
	switch res.outcome {
	case outcomeExact, outcomeFuzzy:
		c.Anchor = res.anchor
		c.Status = StatusPending
		c.Candidates = nil
		c.Moved = res.outcome == outcomeFuzzy
	case outcomeMany:
		c.Anchor = nil
		c.Status = StatusUnsure
		c.Candidates = res.candidates
	case outcomeNone:
		c.Anchor = nil
		c.Status = StatusLost
		c.Candidates = nil
	}
}

// applyAnchorTracking cập nhật mốc neo của góp ý đã gửi mà không đổi trạng thái
// (vòng đời sent→fixed/pending chỉ chạy khi chương được ghi).
func applyAnchorTracking(c *Comment, paras []string) {
	hint := -1
	if c.Paragraph != nil {
		hint = *c.Paragraph
	}
	res := resolveAnchor(paras, c.Quote, hint, c.Anchor)
	switch res.outcome {
	case outcomeExact:
		c.Anchor = res.anchor
		c.Moved = false
	case outcomeFuzzy:
		c.Anchor = res.anchor
		c.Moved = true
	default:
		c.Anchor = nil
	}
}

// applyWriteResolution áp luật §AI sau một lần ghi chương. Trả về loại chuyển
// trạng thái để tổng hợp sự kiện.
func applyWriteResolution(c *Comment, paras []string) (fixed, unfixed, need bool) {
	hint := -1
	if c.Paragraph != nil {
		hint = *c.Paragraph
	}
	res := resolveAnchor(paras, c.Quote, hint, c.Anchor)
	switch c.Status {
	case StatusPending, StatusLost, StatusUnsure:
		switch res.outcome {
		case outcomeExact, outcomeFuzzy:
			c.Anchor = res.anchor
			c.Status = StatusPending
			c.Candidates = nil
			c.Moved = res.outcome == outcomeFuzzy
		case outcomeMany:
			c.Anchor = nil
			c.Status = StatusUnsure
			c.Candidates = res.candidates
			need = true
		case outcomeNone:
			c.Anchor = nil
			c.Status = StatusLost
			c.Candidates = nil
			need = true
		}
	case StatusSent:
		switch res.outcome {
		case outcomeExact:
			// Câu còn nguyên: Người viết chưa sửa chỗ này → về chờ gửi.
			c.Anchor = res.anchor
			c.Status = StatusPending
			c.Unfixed = true
			c.Moved = false
			c.Candidates = nil
			unfixed = true
		case outcomeFuzzy:
			// Trích gốc mất, đoạn thay duy nhất: coi như đã sửa (quote_gone).
			c.Anchor = res.anchor
			c.Status = StatusFixed
			now := time.Now().UTC()
			c.FixedAt = &now
			c.FixedBy = FixedByQuoteGone
			c.NewQuote = res.anchor.Quote
			c.Moved = false
			c.Candidates = nil
			fixed = true
		case outcomeMany:
			c.Anchor = nil
			c.Status = StatusUnsure
			c.Candidates = res.candidates
			need = true
		case outcomeNone:
			c.Anchor = nil
			c.Status = StatusLost
			c.Candidates = nil
			need = true
		}
	case StatusFixed:
		// Thẻ Đã sửa theo câu mới (hoặc trích gốc) để badge không trôi; không
		// đổi trạng thái cho dù không còn khớp.
		needle := c.NewQuote
		if needle == "" {
			needle = c.Quote
		}
		if r := resolveAnchor(paras, needle, hint, c.Anchor); r.outcome == outcomeExact || r.outcome == outcomeFuzzy {
			c.Anchor = r.anchor
		}
	}
	return fixed, unfixed, need
}

func (s *Store) List(chapter int) ([]Comment, error) {
	var result []Comment
	err := s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
		result = append([]Comment{}, list...)
		return list, false, nil
	})
	return result, err
}

// CountsByChapter đếm nhanh số góp ý cần chú ý của mọi chương (danh sách
// chương): need = lost+unsure, pending = chờ gửi. Đọc thô không migrate.
func (s *Store) CountsByChapter() (map[int]Counts, error) {
	out := map[int]Counts{}
	chapters, err := s.repo.Chapters()
	if err != nil {
		return out, nil
	}
	for _, chapter := range chapters {
		raw, err := s.repo.ReadRaw(chapter)
		if err != nil {
			continue
		}
		var c Counts
		for _, r := range raw {
			switch storedStatus(r) {
			case StatusLost, StatusUnsure:
				c.Need++
			case StatusPending:
				c.Pending++
			}
		}
		if c.Need > 0 || c.Pending > 0 {
			out[chapter] = c
		}
	}
	return out, nil
}

func (s *Store) Add(chapter int, in Input, author string) (Comment, error) {
	var result Comment
	in.Quote, in.Text = strings.TrimSpace(in.Quote), strings.TrimSpace(in.Text)
	if in.Paragraph != nil && (*in.Paragraph < 0 || in.Quote == "") {
		return result, fmt.Errorf("%w: đoạn cần vị trí và trích dẫn", ErrInvalid)
	}
	if len(in.Chips) == 0 && in.Text == "" {
		return result, fmt.Errorf("%w: hãy chọn yêu cầu hoặc nhập ý kiến", ErrInvalid)
	}
	for _, chip := range in.Chips {
		valid := false
		for _, allowed := range Chips {
			if chip == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return result, fmt.Errorf("%w: yêu cầu nhanh không hợp lệ", ErrInvalid)
		}
	}
	if in.Chips == nil {
		in.Chips = []string{}
	}
	result = Comment{
		ID: rand.Text(), Paragraph: in.Paragraph, Quote: in.Quote, Chips: in.Chips, Text: in.Text,
		Author: author, CreatedAt: time.Now().UTC(), Status: StatusPending,
	}
	if in.Paragraph == nil && in.Quote == "" {
		result.Whole = true
	}
	err := s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
		// Giải mốc neo lúc tạo trên cùng văn bản vừa nạp để làm mới danh sách:
		// đoạn chỉ định chứa trích thì neo vào đó; nếu không, chỉ nhận khi khớp
		// duy nhất. Không đoán giữa hai đoạn trùng nhau (anchor để trống).
		if !result.Whole && normalizeText(result.Quote) != "" {
			if text, err := s.repo.ChapterText(chapter); err == nil && text != "" {
				paras := normalizedParagraphs(text)
				res := resolveAnchor(paras, result.Quote, ptrVal(result.Paragraph), nil)
				if res.outcome == outcomeExact || res.outcome == outcomeFuzzy {
					result.Anchor = res.anchor
					result.Moved = res.outcome == outcomeFuzzy
				}
			}
		}
		return append(list, result), true, nil
	})
	return result, err
}

func ptrVal(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

func (s *Store) Delete(chapter int, id string) error {
	return s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
		for i, c := range list {
			if c.ID == id {
				return append(list[:i], list[i+1:]...), true, nil
			}
		}
		return list, false, ErrNotFound
	})
}

// Patch thực hiện các thao tác §AI trên một góp ý. Lỗi trả về là câu tiếng
// Việt (ErrInvalid → 400, ErrNotFound → 404).
func (s *Store) Patch(chapter int, id string, in PatchInput) (Comment, error) {
	var result Comment
	err := s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
		idx := -1
		for i, c := range list {
			if c.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return list, false, ErrNotFound
		}
		c := &list[idx]
		now := time.Now().UTC()
		switch in.Action {
		case ActionResolve:
			if c.Status != StatusDone {
				c.Status = StatusDone
				c.ResolvedAt = &now
			}
		case ActionReopen:
			// "Người viết chưa sửa chỗ này" và mở lại góp ý đã xử lý đều về
			// Chờ gửi; cờ unfixed nhớ đây từng là góp ý đã gửi mà chưa được sửa.
			if c.Status == StatusPending {
				break
			}
			c.Status = StatusPending
			c.Unfixed = true
			c.Candidates = nil
			c.FixedAt, c.FixedBy, c.NewQuote, c.ResolvedAt = nil, "", "", nil
			if c.anchorEligible() {
				if text, err := s.repo.ChapterText(chapter); err == nil && text != "" {
					if res := resolveAnchor(normalizedParagraphs(text), c.Quote, ptrVal(c.Paragraph), c.Anchor); res.outcome == outcomeExact || res.outcome == outcomeFuzzy {
						c.Anchor = res.anchor
						c.Moved = res.outcome == outcomeFuzzy
					}
				}
			}
		case ActionRelink:
			text, err := s.repo.ChapterText(chapter)
			if err != nil || strings.TrimSpace(text) == "" {
				return list, false, fmt.Errorf("%w: chương chưa có nội dung để gắn lại", ErrInvalid)
			}
			paras := normalizedParagraphs(text)
			if in.Paragraph == nil && in.Quote == "" {
				return list, false, fmt.Errorf("%w: hãy chọn đoạn hoặc câu để gắn lại", ErrInvalid)
			}
			quote := strings.TrimSpace(in.Quote)
			if in.Paragraph == nil {
				// Chỉ có câu: chấp nhận khi khớp đúng một đoạn.
				res := resolveAnchor(paras, quote, -1, nil)
				if res.outcome != outcomeExact {
					return list, false, fmt.Errorf("%w: câu này không tìm được đúng một đoạn", ErrInvalid)
				}
				c.Anchor = res.anchor
				c.Candidates = nil
				c.Moved = false
			} else {
				p := *in.Paragraph
				if p < 0 || p >= len(paras) {
					return list, false, fmt.Errorf("%w: đoạn đã chọn không còn trong chương", ErrInvalid)
				}
				if quote == "" {
					for _, cand := range c.Candidates {
						if cand.Paragraph == p {
							quote = cand.Snippet
							break
						}
					}
				}
				if quote == "" || !strings.Contains(paras[p], normalizeText(quote)) {
					// Người dùng bấm "Gắn vào đây" trên đoạn gợi ý: neo cả đoạn
					// khi không có câu con cụ thể.
					if quote != "" {
						return list, false, fmt.Errorf("%w: câu đã chọn không nằm trong đoạn này", ErrInvalid)
					}
					c.Anchor = buildAnchor(paras, p, 0, len([]rune(paras[p])))
				} else {
					at := indexRunes([]rune(paras[p]), []rune(normalizeText(quote)))
					c.Anchor = &Anchor{Paragraph: p, Quote: normalizeText(quote), Prefix: contextBefore(paras, p, at), Suffix: contextAfter(paras, p, at+len([]rune(normalizeText(quote))))}
				}
				c.Candidates = nil
				c.Moved = false
			}
			c.Status = StatusPending
			c.Relinked = true
			c.Unfixed = false
			c.FixedAt, c.FixedBy, c.NewQuote, c.ResolvedAt = nil, "", "", nil
			c.Whole = false
		case ActionResendWhole:
			// Gửi lại cho cả chương kèm câu cũ: về Chờ gửi, không còn neo đoạn.
			c.Status = StatusPending
			c.Whole = true
			c.Unfixed = false
			c.Candidates = nil
			c.FixedAt, c.FixedBy, c.NewQuote, c.ResolvedAt = nil, "", "", nil
		default:
			return list, false, fmt.Errorf("%w: thao tác không được hỗ trợ", ErrInvalid)
		}
		result = *c
		return list, true, nil
	})
	return result, err
}

// OnChapterWritten gắn lại mọi mốc neo sau một lần ghi chương rồi xếp sự kiện
// comments.reanchored. Lỗi ghi sự kiện không làm hỏng kết quả gắn lại.
func (s *Store) OnChapterWritten(chapter int, content string) (ReanchorEvent, error) {
	return s.reanchorAndPublish(chapter, content, 0)
}

// reanchorAndPublish áp luật §AI trên nội dung vừa ghi rồi xếp sự kiện gộp số
// đã sửa do writer báo (agentFixed) vào nhóm đã sửa của lần ghi này.
func (s *Store) reanchorAndPublish(chapter int, content string, agentFixed int) (ReanchorEvent, error) {
	paras := normalizedParagraphs(content)
	var ev ReanchorEvent
	ev.Chapter = chapter
	err := s.repo.TransactAt(chapter, paras, func(list []Comment) ([]Comment, bool, error) {
		changed := false
		for i := range list {
			c := &list[i]
			if c.Status == StatusDone || !c.anchorEligible() {
				continue
			}
			before := *c
			f, u, need := applyWriteResolution(c, paras)
			if f {
				ev.Fixed++
			}
			if u {
				ev.Unfixed++
			}
			if need {
				ev.Need++
			}
			if !commentsEqual(before, *c) {
				changed = true
			}
		}
		return list, changed || len(list) > 0, nil
	})
	if err != nil {
		return ev, err
	}
	ev.Fixed += agentFixed
	if err := s.AppendEvent(ev); err != nil {
		return ev, fmt.Errorf("ghi sự kiện gắn lại góp ý: %w", err)
	}
	return ev, nil
}

// MarkFixed đánh dấu các id writer báo đã sửa (fixed_comments). Batches lũy
// đẳng: id không tồn tại hoặc không còn trạng thái sent bị bỏ qua.
func (s *Store) MarkFixed(chapter int, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	marked := 0
	err := s.repo.TransactAt(chapter, nil, func(list []Comment) ([]Comment, bool, error) {
		changed := false
		now := time.Now().UTC()
		for i := range list {
			c := &list[i]
			if !want[c.ID] || c.Status != StatusSent {
				continue
			}
			c.Status = StatusFixed
			c.FixedAt = &now
			c.FixedBy = FixedByAgent
			c.Unfixed = false
			c.Candidates = nil
			marked++
			changed = true
		}
		return list, changed, nil
	})
	return marked, err
}

// ResolveAllFixed chuyển mọi góp ý Đã sửa sang Đã xử lý khi truyện được duyệt
// lần kế tiếp (§AI: tự fixed→done ở lần duyệt tiếp theo của truyện).
func (s *Store) ResolveAllFixed() error {
	chapters, err := s.repo.Chapters()
	if err != nil {
		return nil
	}
	for _, chapter := range chapters {
		if err := s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
			changed := false
			now := time.Now().UTC()
			for i := range list {
				if list[i].Status == StatusFixed {
					list[i].Status = StatusDone
					list[i].ResolvedAt = &now
					changed = true
				}
			}
			return list, changed, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// eventsFileName là nhật ký sự kiện gắn lại chờ gateway phát SSE.
const eventsFileName = "_events.json"

// AppendEvent xếp một sự kiện vào nhật ký; DrainEvents lấy và xoá nguyên tử.
func (s *Store) AppendEvent(ev ReanchorEvent) error {
	return s.repo.WithEvents(func(events []ReanchorEvent) []ReanchorEvent {
		return append(events, ev)
	})
}

// DrainEvents đọc toàn bộ sự kiện đang chờ và xoá chúng trong một khóa.
func (s *Store) DrainEvents() ([]ReanchorEvent, error) {
	var out []ReanchorEvent
	err := s.repo.WithEvents(func(events []ReanchorEvent) []ReanchorEvent {
		out = append(out, events...)
		return events[:0]
	})
	return out, err
}

// Apply holds only the comments lease, not the engine lease. Feedback is marked
// sent only after dispatch succeeds; a failed dispatch leaves it pending.
// Batches are idempotent: only pending feedback is composed and dispatched.
func (s *Store) Apply(chapter int, dispatch func(string) error) (int, error) {
	sent := 0
	err := s.repo.Transact(chapter, func(list []Comment) ([]Comment, bool, error) {
		instruction := Compose(chapter, list)
		if err := dispatch(instruction); err != nil {
			return list, false, err
		}
		now := time.Now().UTC()
		for i := range list {
			if list[i].Status == StatusPending {
				list[i].Status = StatusSent
				list[i].SentAt = &now
				list[i].Unfixed = false
				sent++
			}
		}
		return list, sent > 0, nil
	})
	if err != nil {
		return 0, err
	}
	return sent, nil
}

// Compose orders paragraph feedback by resolved position, then creation time.
// Whole-chapter requests follow the paragraphs; already sent feedback is never
// repeated. Mỗi góp ý ghi kèm id để Người viết trả fixed_comments.
func Compose(chapter int, list []Comment) string {
	pending := make([]Comment, 0, len(list))
	for _, c := range list {
		if c.Status == StatusPending || (c.Status == "" && c.SentAt == nil) {
			pending = append(pending, c)
		}
	}
	if len(pending) == 0 {
		return ""
	}
	paragraphOf := func(c Comment) int {
		if c.Anchor != nil {
			return c.Anchor.Paragraph
		}
		if c.Paragraph != nil {
			return *c.Paragraph
		}
		return 1 << 30
	}
	sort.SliceStable(pending, func(i, j int) bool {
		a, b := pending[i], pending[j]
		aw := a.Whole || (a.Paragraph == nil && a.Anchor == nil)
		bw := b.Whole || (b.Paragraph == nil && b.Anchor == nil)
		if aw != bw {
			return !aw
		}
		if paragraphOf(a) != paragraphOf(b) {
			return paragraphOf(a) < paragraphOf(b)
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
	var out strings.Builder
	fmt.Fprintf(&out, "Người viết: sửa chương %d theo các góp ý dưới đây, giữ nguyên các phần không được yêu cầu sửa. Sau khi sửa, tiếp tục viết theo giấy phép duyệt.\n", chapter)
	for _, c := range pending {
		fmt.Fprintf(&out, "\nGóp ý %s:\n", c.ID)
		whole := c.Whole || (c.Paragraph == nil && c.Anchor == nil)
		if whole {
			out.WriteString("Phạm vi: cả chương\n")
		}
		if c.Quote != "" {
			fmt.Fprintf(&out, "Trích dẫn: %q\n", c.Quote)
		}
		out.WriteString("Yêu cầu: ")
		for i, chip := range c.Chips {
			if i > 0 {
				out.WriteString("; ")
			}
			out.WriteString(chip)
		}
		if c.Text != "" {
			if len(c.Chips) > 0 {
				out.WriteString("; ")
			}
			out.WriteString(c.Text)
		}
		out.WriteByte('\n')
	}
	out.WriteString("\nKhi nộp chương, khai báo các góp ý đã sửa xong qua tham số fixed_comments của commit_chapter (danh sách id ở đầu mỗi mục trên). Góp ý không báo id nào được coi là chưa sửa.\n")
	return out.String()
}
