package web

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/naolint"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// review_items.go là module đọc duy nhất của phần thẩm duyệt: nó dựng payload
// GAP-36 cho GET …/reviews/latest, …/reviews/{n}, cửa duyệt /next, ma trận chất
// lượng, Vũ trụ chung và Bài học. File lưu của engine (reviews/NN.json) KHÔNG đổi:
// mọi trường mới chỉ được tính lúc đọc.
//
// Caller chỉ đọc view mà module trả: không truyền nil cho phần xưng hô, không tự
// chọn giữa "cả bài" với "chỉ items". Hai đường đọc có chủ đích: `cards()` (đầy đủ,
// kèm vòng lưu trữ) và `openErrors()` (badge danh sách truyện, chỉ thẻ hiện tại).

// Bốn dạng chip mục luật của GAP-36 / 17-kit › ReviewIssue.
const (
	reviewRefAddress       = naolint.IssueRef
	reviewRefUser          = "Quy tắc của bạn"
	reviewLevelError       = "error"
	reviewLevelWarning     = "warning"
	reviewApproveBlockedBy = "Chương %d còn mục Bắt sửa, chưa duyệt được."

	// reviewKeptState là trạng thái duy nhất của một quyết định "Giữ nguyên"
	// (GAP-47): review_keep.go ghi nó, reviewItemKept đọc nó.
	reviewKeptState = "kept"
)

// reviewItemAPI là một thẻ ReviewIssue dùng chung (Trình đọc, Bàn viết, Nhật ký,
// nhật ký điện thoại).
type reviewItemAPI struct {
	ID       string             `json:"id"`
	Level    string             `json:"level"`   // error = Bắt sửa · warning = Cảnh báo
	Ref      string             `json:"ref"`     // "Master §7" | "Bộ A §4.3" | "Xưng hô" | "Quy tắc của bạn"
	RefName  string             `json:"refName"` // tên mục trong gói; với xưng hô là chiều "A → B"
	Text     string             `json:"text"`
	Quote    string             `json:"quote"`
	Fix      string             `json:"fix"`
	Rule     string             `json:"rule,omitempty"`     // câu 4 §H: mã phụ ổn định, hiện chỉ có s19-recap
	Decision *reviewDecisionAPI `json:"decision,omitempty"` // GAP-47: có khi người dùng đã "Giữ nguyên"
}

// reviewDecisionAPI là quyết định "Giữ nguyên" kèm lý do (GAP-47).
type reviewDecisionAPI struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
	By     string `json:"by"`
	At     string `json:"at"`
}

// reviewPackAPI là gói thể loại mà bài thẩm duyệt chấm theo.
type reviewPackAPI struct {
	ID  string `json:"id"`
	Ten string `json:"ten"`
}

// reviewRoundAPI là một lần chấm cũ của cùng bài thẩm duyệt (reviews/rounds/).
type reviewRoundAPI struct {
	Round     int             `json:"round"`
	Verdict   string          `json:"verdict"`
	Items     []reviewItemAPI `json:"items"`
	RuleAware bool            `json:"rule_aware"` // vòng chấm sinh bởi schema có mã phụ `rule`
}

// reviewEntryAPI = ReviewEntry đã lưu + các trường GAP-36. Nhúng ReviewEntry để
// giữ nguyên toàn bộ trường cũ (issues, verdict, dimensions, summary…) nên FE cũ
// vẫn đọc được; file lưu không đổi vì DTO chỉ tồn tại ở tầng API.
type reviewEntryAPI struct {
	domain.ReviewEntry
	Items    []reviewItemAPI  `json:"items"`
	Previous []reviewRoundAPI `json:"previous"`
	Round    int              `json:"round"`
	At       string           `json:"at,omitempty"` // RFC3339 của lần lưu vòng chấm; không có thì bỏ trống
	Pack     *reviewPackAPI   `json:"pack"`
	Blocking bool             `json:"blocking"`
}

// reviewReader là đầu đọc của module: nó gom những gì cần để dựng view của một
// truyện — gói thể loại, tiêu đề mục luật, log checkpoint, kho thế giới (các lần
// chấm cũ, quyết định "Giữ nguyên", vi phạm xưng hô).
type reviewReader struct {
	sr        *host.StoryReader
	pack      *reviewPackAPI
	packRules *assets.Pack
	titles    map[string]string
	cps       []domain.Checkpoint
	world     store.WorldStore
	decisions map[string]store.ReviewDecision

	// address giữ vi phạm xưng hô theo chương; cả tệp meta/rule_violations.jsonl
	// được đọc đúng một lần cho mỗi reader (không phải một lần mỗi chương).
	address       map[int][]rules.Violation
	addressLoaded bool

	// loadRounds đọc các vòng đã lưu trữ của một bài (mặc định: kho thế giới).
	// Đường badge openErrors không bao giờ gọi tới nó — đó là điều kiện để danh
	// sách truyện không mở reviews/rounds/. Test thay hàm này để chứng minh điều đó.
	loadRounds func(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error)
}

// newReviewReader đọc gói theo khoá style của truyện (cùng nguồn với rules_api)
// và log checkpoint của engine. Lỗi đọc cấu hình/gói không làm hỏng endpoint:
// pack = null, refName rơi về rule_text.
func newReviewReader(storyDir string, sr *host.StoryReader) *reviewReader {
	r := &reviewReader{sr: sr}
	// File mode resolves the style from the config chain. Hosted mode has no
	// config file: the style the engine recorded in run meta selects the
	// embedded pack, so no host path is read.
	if storyDir != "" {
		if cfg, err := bootstrap.LoadConfigAt(storyDir); err == nil {
			bundle := assets.Load(cfg.Style, assets.DefaultLoadOptions(sr.EngineDir()))
			if bundle.Err == nil && bundle.Pack != nil {
				r.pack = &reviewPackAPI{ID: bundle.Pack.Name, Ten: bundle.Pack.Ten}
				r.packRules = bundle.Pack
				r.titles = bundle.Pack.SectionTitles()
			}
		}
	} else if s := sr.Store(); s != nil {
		style := ""
		if meta, err := s.RunMeta.Load(); err == nil && meta != nil {
			style = meta.Style
		}
		bundle := assets.Load(style, assets.LoadOptions{})
		if bundle.Err == nil && bundle.Pack != nil {
			r.pack = &reviewPackAPI{ID: bundle.Pack.Name, Ten: bundle.Pack.Ten}
			r.packRules = bundle.Pack
			r.titles = bundle.Pack.SectionTitles()
		}
	}
	if s := sr.Store(); s != nil {
		r.cps = s.Checkpoints.All()
		r.world = s.World
		r.loadRounds = s.World.LoadReviewRounds
		r.decisions, _ = s.World.LoadReviewDecisions()
	}
	return r
}

// reviewCard là một bài thẩm duyệt đã lưu của truyện kèm view của nó.
type reviewCard struct {
	Chapter int
	Scope   store.ReviewScope
	View    reviewEntryAPI
}

// FileName là tên tệp của bài trong reviews/ ("03.json" / "03-global.json").
func (c reviewCard) FileName() string { return c.Scope.FileName(c.Chapter) }

// reviewFileName nhận đúng tên tệp bài thẩm duyệt trong reviews/: NN.json và
// NN-global.json. reviews/decisions.json (GAP-47) và reviews/rounds/ không phải
// bài thẩm duyệt nên không bao giờ được đọc ở đây.
var reviewFileName = regexp.MustCompile(`^(\d+)(-global)?\.json$`)

// reviewFileRef là vị trí một bài thẩm duyệt của truyện, không mang nội dung.
type reviewFileRef struct {
	Chapter int
	Scope   store.ReviewScope
}

// files liệt kê các vị trí bài thẩm duyệt: chương cũ trước, bài chương trước bài
// tổng hợp. Kho tệp chỉ đọc tên trong reviews/; kho không có tệp lấy mốc chương
// từ backend rồi để load bỏ qua scope chưa có. Không đọc vòng lưu trữ ở đây.
func (r *reviewReader) files() ([]reviewFileRef, error) {
	var refs []reviewFileRef
	if r.sr.EngineDir() == "" {
		// Reviews are typed rows in hosted storage; Docs.ListDir("reviews")
		// deliberately has no file projection. Chapter markers enumerate the
		// live review rows, while World resolves each scope and its rounds.
		chapters, err := r.sr.Store().ReviewChapters()
		if err != nil {
			return nil, err
		}
		refs = make([]reviewFileRef, 0, 2*len(chapters))
		for _, chapter := range chapters {
			refs = append(refs,
				reviewFileRef{Chapter: chapter, Scope: store.ReviewScopeChapter},
				reviewFileRef{Chapter: chapter, Scope: store.ReviewScopeGlobal})
		}
	} else {
		entries, err := r.sr.Store().Docs().ListDir("reviews")
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		refs = make([]reviewFileRef, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir {
				continue
			}
			m := reviewFileName.FindStringSubmatch(entry.Name)
			if m == nil {
				continue
			}
			chapter, err := strconv.Atoi(m[1])
			if err != nil || chapter < 1 {
				continue
			}
			scope := store.ReviewScopeChapter
			if m[2] != "" {
				scope = store.ReviewScopeGlobal
			}
			refs = append(refs, reviewFileRef{Chapter: chapter, Scope: scope})
		}
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Chapter != refs[j].Chapter {
			return refs[i].Chapter < refs[j].Chapter
		}
		return refs[i].Scope != store.ReviewScopeGlobal
	})
	return refs, nil
}

// load đọc đúng tệp hiện tại của một bài; nil khi tệp không còn.
func (r *reviewReader) load(ref reviewFileRef) (*domain.ReviewEntry, error) {
	if r.world == nil {
		return nil, nil
	}
	if ref.Scope == store.ReviewScopeGlobal {
		return r.world.LoadGlobalReview(ref.Chapter)
	}
	return r.world.LoadReview(ref.Chapter)
}

// cards trả view đầy đủ của mọi bài thẩm duyệt đã lưu của truyện, kèm các vòng
// lưu trữ. Màn nào cần "đã sửa / đã giữ" hoặc vòng cũ thì dùng đường này.
func (r *reviewReader) cards() ([]reviewCard, error) {
	refs, err := r.files()
	if err != nil {
		return nil, err
	}
	cards := make([]reviewCard, 0, len(refs))
	for _, ref := range refs {
		view, ok, err := r.cardView(ref.Chapter, ref.Scope)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		cards = append(cards, reviewCard{Chapter: ref.Chapter, Scope: ref.Scope, View: view})
	}
	return cards, nil
}

// openErrors đếm mục Bắt sửa còn mở của mọi bài thẩm duyệt của truyện — badge
// "N lỗi" trên danh sách truyện. Đường này chỉ đọc thẻ hiện tại: vòng đã bị thay
// không thể còn mở, nên reviews/rounds/ không bao giờ được mở ở đây (badge chạy
// một lần cho mỗi truyện trong mỗi lần tải danh sách).
func (r *reviewReader) openErrors() (int, error) {
	refs, err := r.files()
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	count := 0
	for _, ref := range refs {
		entry, err := r.load(ref)
		if err != nil {
			return 0, err
		}
		if entry == nil {
			continue
		}
		for _, item := range r.items(entry) {
			if seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			if reviewItemKept(item) || item.Level != reviewLevelError {
				continue
			}
			count++
		}
	}
	return count, nil
}

// cardView đọc bài thẩm duyệt ở (chương, tệp) và trả view đầy đủ của nó; ok = false
// khi tệp chưa có.
func (r *reviewReader) cardView(chapter int, scope store.ReviewScope) (reviewEntryAPI, bool, error) {
	entry, err := r.load(reviewFileRef{Chapter: chapter, Scope: scope})
	if err != nil || entry == nil {
		return reviewEntryAPI{}, false, err
	}
	return r.view(entry), true, nil
}

// chapterViews trả hai bài của một chương theo thứ tự cửa duyệt: bài tổng hợp
// (global/arc) trước rồi bài chương; bài chưa có là nil.
func (r *reviewReader) chapterViews(chapter int) (compound, chapterCard *reviewEntryAPI, err error) {
	global, ok, err := r.cardView(chapter, store.ReviewScopeGlobal)
	if err != nil {
		return nil, nil, err
	}
	if ok {
		compound = &global
	}
	chapterOnly, ok, err := r.cardView(chapter, store.ReviewScopeChapter)
	if err != nil {
		return nil, nil, err
	}
	if ok {
		chapterCard = &chapterOnly
	}
	return compound, chapterCard, nil
}

// gateView trả bài mà cửa duyệt xét cho chương: bài tổng hợp (global/arc) của
// cùng chương thắng bài chương — cùng lựa chọn với StoryReader.LatestReview.
// ok = false khi chương chưa có bài nào.
func (r *reviewReader) gateView(chapter int) (reviewEntryAPI, bool, error) {
	compound, chapterCard, err := r.chapterViews(chapter)
	if err != nil {
		return reviewEntryAPI{}, false, err
	}
	entry := compound
	if entry == nil {
		entry = chapterCard
	}
	if entry == nil {
		return reviewEntryAPI{}, false, nil
	}
	return *entry, true, nil
}

// latest trả view của bài thẩm duyệt mới nhất của truyện; ok = false khi chưa có.
func (r *reviewReader) latest() (reviewEntryAPI, bool, error) {
	entry, err := r.sr.LatestReview()
	if err != nil || entry == nil {
		return reviewEntryAPI{}, false, err
	}
	return r.view(entry), true, nil
}

// view dựng view của một bài đã lưu. Mọi màn dùng chung đúng một hình dạng này:
// thẻ hiện tại (issues[] của engine cộng vi phạm xưng hô của chương), quyết định
// "Giữ nguyên" đã gắn, vòng chấm hiện tại (round/at) và các vòng đã lưu trữ.
func (r *reviewReader) view(entry *domain.ReviewEntry) reviewEntryAPI {
	archived := r.archivedRounds(entry)
	out := reviewEntryAPI{
		ReviewEntry: *entry,
		Items:       []reviewItemAPI{},
		Previous:    []reviewRoundAPI{},
		Round:       r.roundOf(entry, len(archived)),
		At:          r.roundAt(entry),
		Pack:        r.pack,
	}
	if out.Issues == nil {
		out.Issues = []domain.ConsistencyIssue{}
	}
	// Các lần chấm cũ: cũ nhất trước, dùng đúng bộ chuyển mục của lần hiện tại
	// (không gắn mục soát xưng hô hiện thời vào quá khứ).
	for i := range archived {
		out.Previous = append(out.Previous, reviewRoundAPI{
			Round:     i + 1,
			Verdict:   archived[i].Verdict,
			Items:     r.issueItems(&archived[i], map[string]int{}),
			RuleAware: archived[i].RuleAware,
		})
	}
	out.Items = r.items(entry)
	// blocking tính trên đúng danh sách thẻ đã trả: mục Bắt sửa của bài thẩm
	// duyệt (engine đã đưa vào hàng chờ sửa) và mục xưng hô mức error (câu 13).
	// Cửa duyệt dùng cùng danh sách nên luôn khớp hàng chờ.
	for _, item := range out.Items {
		if item.Level == reviewLevelError {
			out.Blocking = true
			break
		}
	}
	return out
}

// items dựng thẻ hiện tại của một bài: issues[] của engine theo đúng thứ tự, rồi
// vi phạm xưng hô đã lưu của chương (D9) — luôn tính như nhau cho mọi màn. Bài do
// engine lưu từ đợt này đã gộp sẵn mục xưng hô vào issues[] (save_review) nên
// không gộp lại lần nữa; chỉ bài cũ chưa có mục xưng hô mới cần đọc từ
// meta/rule_violations.jsonl.
func (r *reviewReader) items(entry *domain.ReviewEntry) []reviewItemAPI {
	used := map[string]int{}
	items := r.issueItems(entry, used)
	if entryHasAddressIssue(entry) {
		return items
	}
	return append(items, r.addressItems(entry.Chapter, used)...)
}

// entryHasAddressIssue: engine đã gộp vi phạm xưng hô của chương vào bài này chưa.
func entryHasAddressIssue(entry *domain.ReviewEntry) bool {
	for _, issue := range entry.Issues {
		if issue.RuleRef == reviewRefAddress {
			return true
		}
	}
	return false
}

// addressViolations lấy vi phạm xưng hô đã ghi cho chương: commit_chapter lưu
// naolint.Finding dưới dạng rules.Violation, nên không cần kho mới (D9). Cả tệp
// meta/rule_violations.jsonl được đọc một lần cho mỗi reader.
func (r *reviewReader) addressViolations(chapter int) []rules.Violation {
	if !r.addressLoaded {
		r.addressLoaded = true
		r.address = map[int][]rules.Violation{}
		if r.world != nil {
			for ch, violations := range r.world.RuleViolationsByChapter() {
				var address []rules.Violation
				for _, v := range violations {
					if v.Rule == naolint.AddressRule {
						address = append(address, v)
					}
				}
				if len(address) > 0 {
					r.address[ch] = address
				}
			}
		}
	}
	if chapter < 1 {
		return nil
	}
	return r.address[chapter]
}

// addressItems dựng thẻ "Xưng hô" từ vi phạm đã lưu của bài cũ (chưa được engine
// gộp vào issues[]): tên mục là chiều "A → B", hoặc "Bảng xưng hô của truyện" khi
// dữ liệu lưu cũ không ghi chiều.
// Dùng chung naolint.Issue với save_review nên câu chữ và mức khớp nhau.
func (r *reviewReader) addressItems(chapter int, used map[string]int) []reviewItemAPI {
	violations := r.addressViolations(chapter)
	items := make([]reviewItemAPI, 0, len(violations))
	for _, v := range violations {
		issue := naolint.Issue(v, chapter)
		item := reviewItemAPI{
			Level:   reviewLevel(string(v.Severity)),
			Ref:     reviewRefAddress,
			RefName: firstNonEmpty(strings.TrimSpace(v.Target), naolint.IssueName),
			Text:    issue.Description,
			Quote:   issue.Quote,
			Fix:     issue.Suggestion,
		}
		item.ID = domain.ReviewItemID(chapter, used, item.Ref, item.Quote)
		items = append(items, r.withDecision(item))
	}
	return items
}

// roundAt là thời điểm lưu vòng chấm hiện tại: checkpoint step=review của đúng
// tệp thẩm duyệt (đường tệp đã phân biệt chương/cung với tổng hợp). Truyện nhập
// tay hoặc bản ghi cũ không có checkpoint ⇒ "".
func (r *reviewReader) roundAt(entry *domain.ReviewEntry) string {
	artifact := store.ReviewScopeOf(entry.Scope).Artifact(entry.Chapter)
	for i := len(r.cps) - 1; i >= 0; i-- {
		cp := r.cps[i]
		if cp.Step == "review" && cp.Artifact == artifact {
			return cp.OccurredAt.Format(time.RFC3339)
		}
	}
	return ""
}

// archivedRounds đọc các lần chấm cũ của đúng bài thẩm duyệt này
// (reviews/rounds/NN[-global]-r<K>.json), cũ nhất trước.
func (r *reviewReader) archivedRounds(entry *domain.ReviewEntry) []domain.ReviewEntry {
	if r.loadRounds == nil {
		return nil
	}
	rounds, err := r.loadRounds(entry.Chapter, store.ReviewScopeOf(entry.Scope))
	if err != nil {
		return nil
	}
	return rounds
}

// roundOf: có bản lưu trữ ⇒ lần chấm hiện tại = số bản lưu trữ + 1 (nhất quán với
// reviews/rounds/). Truyện cũ chưa có bản lưu trữ thì rơi về số checkpoint.
func (r *reviewReader) roundOf(entry *domain.ReviewEntry, archived int) int {
	if archived > 0 {
		return archived + 1
	}
	return reviewRound(r.cps, entry.Chapter)
}

// issueItems dựng thẻ cho từng issue, giữ đúng thứ tự issues[] để chỉ số khớp.
// used giữ bộ đếm hậu tố id giữa các lần gọi trong cùng một payload.
func (r *reviewReader) issueItems(entry *domain.ReviewEntry, used map[string]int) []reviewItemAPI {
	items := make([]reviewItemAPI, 0, len(entry.Issues))
	for _, issue := range entry.Issues {
		ref, refName := r.issueRef(issue)
		if ref == reviewRefAddress && refName == "" {
			refName = naolint.IssueName
		}
		item := reviewItemAPI{
			Level:   reviewLevel(issue.Severity),
			Ref:     ref,
			RefName: refName,
			Text:    strings.TrimSpace(issue.Description),
			Quote:   strings.TrimSpace(firstNonEmpty(issue.Quote, issue.Evidence)),
			Fix:     strings.TrimSpace(issue.Suggestion),
			Rule:    strings.TrimSpace(issue.Rule),
		}
		item.ID = domain.ReviewItemID(entry.Chapter, used, item.Ref, item.Quote)
		items = append(items, r.withDecision(item))
	}
	return items
}

// issueRef chuẩn hoá mã mục về bốn dạng hiển thị của GAP-36 và lấy tên mục từ gói;
// mục không có trong gói thì rơi về nguyên văn rule_text, rồi "".
func (r *reviewReader) issueRef(issue domain.ConsistencyIssue) (string, string) {
	ref := domain.ReviewIssueRef(issue)
	if title := r.titles[ref]; title != "" && ref != reviewRefUser && ref != reviewRefAddress {
		return ref, title
	}
	return ref, strings.TrimSpace(issue.RuleText)
}

// reviewLevel quy severity của engine về hai mức hiển thị: critical/error = Bắt sửa.
func reviewLevel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error":
		return reviewLevelError
	default:
		return reviewLevelWarning
	}
}

// reviewItemKept: người dùng đã "Giữ nguyên" mục này chưa (GAP-47). Định nghĩa duy
// nhất của "đã giữ" — quyết định đã gắn vào thẻ bởi withDecision.
func reviewItemKept(item reviewItemAPI) bool {
	return item.Decision != nil && item.Decision.State == reviewKeptState
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// withDecision gắn quyết định "Giữ nguyên" đã lưu cho mục (GAP-47).
func (r *reviewReader) withDecision(item reviewItemAPI) reviewItemAPI {
	if d, ok := r.decisions[item.ID]; ok && d.State != "" {
		item.Decision = &reviewDecisionAPI{State: d.State, Reason: d.Reason, By: d.By, At: d.At}
	}
	return item
}

// reviewRound đếm số lần chấm đã ghi cho chương trong meta/checkpoints.jsonl
// (step=review, scope=chapter N — cả bài chương lẫn bài tổng hợp đều dùng scope
// chương). reviews/ chỉ giữ bản mới nhất và save_review không ghi đè nội dung
// khác, nên checkpoint là nguồn duy nhất của "lần chấm"; Append khử trùng theo
// digest nên mỗi nội dung chấm mới là một lần. Truyện nhập/tạo tay không có
// checkpoint → 1.
func reviewRound(cps []domain.Checkpoint, chapter int) int {
	n := 0
	for _, cp := range cps {
		if cp.Step == "review" && cp.Scope.Kind == domain.ScopeChapter && cp.Scope.Chapter == chapter {
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

// reviewEventAPI là payload thẩm duyệt gắn vào dòng Biên tập viên của Nhật ký
// (D4: "Dòng Biên tập viên có danh sách mục ngay bên dưới", MLog dùng bản thu gọn).
type reviewEventAPI struct {
	Round int             `json:"round"`
	Items []reviewItemAPI `json:"items"`
}

// attachActivityReviews gắn payload thẩm duyệt cho các sự kiện của Biên tập viên
// có số chương. Mỗi chương dựng một lần; chương chưa có bài thẩm duyệt thì bỏ
// trống để FE tự quyết định (không bịa mục).
func attachActivityReviews(r *reviewReader, events []activityEvent) {
	cache := map[int]*reviewEventAPI{}
	for i := range events {
		ev := &events[i]
		if activityAgent(ev.Agent) != "editor" || ev.Chapter < 1 {
			continue
		}
		payload, seen := cache[ev.Chapter]
		if !seen {
			if view, ok, err := r.gateView(ev.Chapter); err == nil && ok {
				payload = &reviewEventAPI{Round: view.Round, Items: view.Items}
			}
			cache[ev.Chapter] = payload
		}
		ev.Review = payload
	}
}

// blockingReviewItems trả chương bị chặn và các mục Bắt sửa còn mở của bài thẩm
// duyệt mới nhất cho chương (D4: cửa duyệt chỉ mở khi không còn mục Bắt sửa).
// Chọn bài giống StoryReader.LatestReview: bài tổng hợp (global/arc) cùng chương
// thắng bài chương. chapter ≤ 0 (client không gửi) → lấy chương của bài mới nhất
// trong reviews/.
func blockingReviewItems(st library.Story, sr *host.StoryReader, chapter int) (int, []reviewItemAPI) {
	if chapter <= 0 {
		latest, err := sr.LatestReview()
		if err != nil || latest == nil {
			return 0, nil
		}
		chapter = latest.Chapter
	}
	view, ok, err := newReviewReader(st.Dir, sr).gateView(chapter)
	if err != nil || !ok {
		return 0, nil
	}
	blocking := make([]reviewItemAPI, 0, len(view.Items))
	for _, item := range view.Items {
		if item.Level == reviewLevelError {
			blocking = append(blocking, item)
		}
	}
	return chapter, blocking
}
