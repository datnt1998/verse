package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/naolint"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// reviewViewStory dựng một truyện trên store tạm (không HTTP): trả engine store
// và đường dẫn truyện để mở reviewReader như handler vẫn làm.
func reviewViewStory(t *testing.T) (*store.Store, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	storyDir := t.TempDir()
	s := store.NewStore(filepath.Join(storyDir, "output", "novel"))
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return s, storyDir
}

// reviewViewReader mở đầu đọc mới — quyết định và bài đã lưu chỉ được đọc lúc
// dựng reader, đúng như một request mới.
func reviewViewReader(storyDir string) *reviewReader {
	return newReviewReader(storyDir, host.NewStoryReader(storyDir))
}

// TestReviewViewJoinsKeepDecision pin GAP-47 ở tầng view: quyết định "Giữ nguyên"
// gắn vào đúng mục theo id ổn định (sống qua lần đọc sau), và đây là định nghĩa
// duy nhất của "đã giữ" mà mọi màn dùng.
func TestReviewViewJoinsKeepDecision(t *testing.T) {
	s, storyDir := reviewViewStory(t)
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish",
		Issues: []domain.ConsistencyIssue{
			{Severity: "warning", Description: "Nhịp chậm", Quote: "đoạn 2"},
			{Severity: "warning", Description: "Câu dài", Quote: "Một câu dài."},
		},
	}); err != nil {
		t.Fatalf("review: %v", err)
	}
	view, ok, err := reviewViewReader(storyDir).cardView(1, store.ReviewScopeChapter)
	if err != nil || !ok {
		t.Fatalf("card ch1 = %v, %v", ok, err)
	}
	if len(view.Items) != 2 {
		t.Fatalf("items = %+v", view.Items)
	}
	keptID := view.Items[0].ID
	for _, item := range view.Items {
		if reviewItemKept(item) {
			t.Fatalf("chưa ai giữ mà view báo đã giữ: %+v", item)
		}
	}
	if err := s.World.SaveReviewDecision(keptID, store.ReviewDecision{
		State: reviewKeptState, Reason: "Cố ý giữ nhịp.", By: "admin", At: "2026-10-26T10:00:00Z",
	}); err != nil {
		t.Fatalf("decision: %v", err)
	}

	again, ok, err := reviewViewReader(storyDir).cardView(1, store.ReviewScopeChapter)
	if err != nil || !ok {
		t.Fatalf("card ch1 lần 2 = %v, %v", ok, err)
	}
	var kept, open int
	for _, item := range again.Items {
		if item.ID != keptID {
			if reviewItemKept(item) {
				t.Fatalf("quyết định gắn sai mục: %+v", item)
			}
			open++
			continue
		}
		kept++
		if !reviewItemKept(item) || item.Decision.Reason != "Cố ý giữ nhịp." ||
			item.Decision.By != "admin" || item.Decision.At != "2026-10-26T10:00:00Z" {
			t.Fatalf("decision = %+v", item.Decision)
		}
	}
	if kept != 1 || open != 1 {
		t.Fatalf("kept=%d open=%d, muốn 1/1: %+v", kept, open, again.Items)
	}
}

// TestQualityViewKeptPartition pin cách chia "đã giữ / còn mở" của ma trận chất
// lượng: mục đã giữ không còn tính là mở (kể cả khi nó chỉ còn trong vòng lưu
// trữ), và mọi chương có bài thẩm duyệt vẫn có cột.
func TestQualityViewKeptPartition(t *testing.T) {
	s, storyDir := reviewViewStory(t)
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish",
		Issues: []domain.ConsistencyIssue{{Severity: "warning", Description: "Nhịp chậm", Quote: "đoạn 2"}},
	}); err != nil {
		t.Fatalf("review ch1: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 2, Scope: "chapter", Verdict: "rewrite",
		Issues: []domain.ConsistencyIssue{{Severity: "error", Description: "Kết chương an toàn", Quote: "Tôi đi ngủ."}},
	}); err != nil {
		t.Fatalf("review ch2: %v", err)
	}
	// Chương 3: mục của vòng 1 bị thay bằng bài sạch, nên nó chỉ còn trong bản
	// lưu trữ; quyết định giữ vẫn thắng trạng thái "đã sửa".
	if err := s.World.SaveReview(domain.ReviewEntry{
		Chapter: 3, Scope: "chapter", Verdict: "polish",
		Issues: []domain.ConsistencyIssue{{Severity: "warning", Description: "Từ lặp", Quote: "rất rất"}},
	}); err != nil {
		t.Fatalf("review ch3 vòng 1: %v", err)
	}
	if err := s.World.SaveReview(domain.ReviewEntry{Chapter: 3, Scope: "chapter", Verdict: "accept"}); err != nil {
		t.Fatalf("review ch3 vòng 2: %v", err)
	}
	archived, ok, err := reviewViewReader(storyDir).cardView(3, store.ReviewScopeChapter)
	if err != nil || !ok || len(archived.Previous) != 1 || len(archived.Previous[0].Items) != 1 {
		t.Fatalf("vòng lưu trữ ch3 = %+v, %v", archived.Previous, err)
	}
	archivedID := archived.Previous[0].Items[0].ID
	if err := s.World.SaveReviewDecision(archivedID, store.ReviewDecision{
		State: reviewKeptState, Reason: "Giữ cách láy.", By: "admin", At: "2026-10-26T10:00:00Z",
	}); err != nil {
		t.Fatalf("decision: %v", err)
	}

	issues, reviewed, err := qualityReviewIssues(reviewViewReader(storyDir))
	if err != nil {
		t.Fatalf("quality issues: %v", err)
	}
	for _, ch := range []int{1, 2, 3} {
		if !reviewed[ch] {
			t.Fatalf("chương %d có bài thẩm duyệt mà thiếu cột: %v", ch, reviewed)
		}
	}
	states := map[string]qualityIssueAPI{}
	for _, issue := range issues {
		states[issue.ID] = issue
	}
	if got := states[archivedID]; got.State != reviewKeptState || got.KeptReason != "Giữ cách láy." {
		t.Fatalf("mục đã giữ chỉ còn trong lưu trữ = %+v", got)
	}
	var openError, openWarning int
	for _, issue := range issues {
		if issue.State != "open" {
			continue
		}
		switch issue.Level {
		case reviewLevelError:
			openError++
		case reviewLevelWarning:
			openWarning++
		}
	}
	// ch1 cảnh báo, ch2 bắt sửa; ch3 đã giữ nên không còn mở.
	if openError != 1 || openWarning != 1 {
		t.Fatalf("open error/warning = %d/%d, muốn 1/1: %+v", openError, openWarning, issues)
	}
}

// TestReviewViewAddressItemsAlwaysComputed pin D9 ở tầng view: vi phạm xưng hô đã
// lưu của chương luôn được tính (không caller nào truyền vào), bài engine đã gộp
// thì không gộp lần hai, và bộ lọc của Vũ trụ/Bài học chỉ bỏ đúng thẻ dựng thêm.
func TestReviewViewAddressItemsAlwaysComputed(t *testing.T) {
	s, storyDir := reviewViewStory(t)
	violation := rules.Violation{
		Rule:     naolint.AddressRule,
		Target:   "Tri Hạ → Nhược Vy",
		Actual:   "em",
		Limit:    "cô",
		Severity: rules.SeverityError,
		Message:  "[LỖI] chương 1, dòng 5: Tri Hạ tự xưng \"em\". Trích: Em về trước đi.",
	}
	legacy := domain.ReviewEntry{
		Chapter: 1, Scope: "chapter", Verdict: "polish",
		Issues: []domain.ConsistencyIssue{{
			Severity: "warning", Description: "Nhịp chậm", RuleRef: "Master §7", Quote: "đoạn 2",
		}},
	}
	if err := s.World.SaveReview(legacy); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := s.World.SaveRuleViolations(1, []rules.Violation{
		violation,
		{Rule: "fatigue_words", Target: "nhìn", Severity: rules.SeverityWarning, Message: "bỏ qua"},
	}); err != nil {
		t.Fatalf("violations: %v", err)
	}

	view, ok, err := reviewViewReader(storyDir).cardView(1, store.ReviewScopeChapter)
	if err != nil || !ok {
		t.Fatalf("card ch1 = %v, %v", ok, err)
	}
	if len(view.Items) != 2 {
		t.Fatalf("items = %+v, muốn issue + xưng hô", view.Items)
	}
	address := view.Items[1]
	if address.Ref != reviewRefAddress || address.Level != reviewLevelError ||
		address.RefName != "Tri Hạ → Nhược Vy" || address.Quote != "Em về trước đi." {
		t.Fatalf("thẻ xưng hô = %+v", address)
	}
	if !view.Blocking {
		t.Fatal("lỗi xưng hô mức error một mình phải blocking")
	}

	// Bài engine đã gộp mục xưng hô vào issues[]: một thẻ duy nhất, và nó không
	// còn là thẻ view dựng thêm nên bộ lọc không bỏ.
	merged := legacy
	merged.Issues = append(merged.Issues, naolint.Issue(violation, 1))
	if err := s.World.SaveReview(merged); err != nil {
		t.Fatalf("review gộp: %v", err)
	}
	again, _, err := reviewViewReader(storyDir).cardView(1, store.ReviewScopeChapter)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Items) != 2 || again.Items[1].Ref != reviewRefAddress {
		t.Fatalf("bài gộp phải còn đúng một thẻ xưng hô: %+v", again.Items)
	}
}

// TestReviewViewArchivedRoundsPerScope pin lưu trữ lần chấm ở tầng view và kiểu
// scope duy nhất của store: mỗi tệp có vòng riêng, round = số vòng lưu trữ + 1,
// và scope=arc dùng chung tệp chương (không có tệp global cho arc).
func TestReviewViewArchivedRoundsPerScope(t *testing.T) {
	s, storyDir := reviewViewStory(t)
	save := func(entry domain.ReviewEntry) {
		t.Helper()
		if err := s.World.SaveReview(entry); err != nil {
			t.Fatalf("save review %+v: %v", entry, err)
		}
	}
	save(domain.ReviewEntry{Chapter: 3, Scope: "chapter", Verdict: "polish"})
	save(domain.ReviewEntry{Chapter: 3, Scope: "chapter", Verdict: "accept"})
	save(domain.ReviewEntry{Chapter: 3, Scope: "global", Verdict: "rewrite"})
	save(domain.ReviewEntry{Chapter: 3, Scope: "global", Verdict: "polish"})
	save(domain.ReviewEntry{Chapter: 4, Scope: "arc", Verdict: "accept"})

	cards, err := reviewViewReader(storyDir).cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	var got []string
	for _, card := range cards {
		got = append(got, card.FileName())
		if card.View.Round != 2 && card.Chapter == 3 {
			t.Fatalf("%s round = %d, muốn 2 sau một lần lưu trữ", card.FileName(), card.View.Round)
		}
		if card.Chapter == 3 && (len(card.View.Previous) != 1 || card.View.Previous[0].Round != 1) {
			t.Fatalf("%s previous = %+v", card.FileName(), card.View.Previous)
		}
	}
	want := []string{"03.json", "03-global.json", "04.json"}
	if len(got) != len(want) {
		t.Fatalf("cards = %v, muốn %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cards = %v, muốn %v", got, want)
		}
	}
	if verdict := cards[0].View.Previous[0].Verdict; verdict != "polish" {
		t.Fatalf("vòng lưu trữ của bài chương = %q, muốn polish", verdict)
	}
	if verdict := cards[1].View.Previous[0].Verdict; verdict != "rewrite" {
		t.Fatalf("vòng lưu trữ của bài tổng hợp = %q, muốn rewrite", verdict)
	}
	// Cửa duyệt xét bài tổng hợp của cùng chương trước bài chương.
	gate, ok, err := reviewViewReader(storyDir).gateView(3)
	if err != nil || !ok || gate.Scope != string(store.ReviewScopeGlobal) || gate.Round != 2 {
		t.Fatalf("gateView(3) = %+v, %v", gate, err)
	}
	if _, ok, err := reviewViewReader(storyDir).gateView(9); err != nil || ok {
		t.Fatalf("chương chưa có bài: ok=%v err=%v", ok, err)
	}

	// Một kiểu scope: arc nằm ở tệp chương, global ở tệp global, và vòng lưu trữ
	// cũng theo đúng tệp đó.
	chapterRounds, err := s.World.LoadReviewRounds(3, store.ReviewScopeChapter)
	if err != nil || len(chapterRounds) != 1 || chapterRounds[0].Verdict != "polish" {
		t.Fatalf("vòng chương = %+v, %v", chapterRounds, err)
	}
	globalRounds, err := s.World.LoadReviewRounds(3, store.ReviewScopeGlobal)
	if err != nil || len(globalRounds) != 1 || globalRounds[0].Verdict != "rewrite" {
		t.Fatalf("vòng tổng hợp = %+v, %v", globalRounds, err)
	}
	if got := store.ReviewScopeOf("arc").Artifact(4); got != "reviews/04.json" {
		t.Fatalf("artifact của arc = %q", got)
	}
	if got := store.ReviewScopeOf("global").ArchiveArtifact(4, 1); got != "reviews/rounds/04-global-r1.json" {
		t.Fatalf("archive của global = %q", got)
	}
	// Truyện chưa có bài nào: latest báo chưa có, không bịa view.
	emptyStore := store.NewStore(filepath.Join(t.TempDir(), "output", "novel"))
	if err := emptyStore.Init(); err != nil {
		t.Fatalf("init rỗng: %v", err)
	}
	emptyDir := filepath.Dir(filepath.Dir(emptyStore.Dir()))
	if _, ok, err := reviewViewReader(emptyDir).latest(); err != nil || ok {
		t.Fatalf("truyện không có bài: ok=%v err=%v", ok, err)
	}
}

// TestStoryOpenErrorsSkipsArchivedRounds pin chi phí badge trên danh sách truyện:
// openErrors chỉ đọc thẻ hiện tại của mỗi bài, không mở reviews/rounds/. Badge vẫn
// đúng khi các tệp lưu trữ hỏng, và vòng đã bị thay (mục lỗi cũ) không bao giờ
// được tính là còn mở.
func TestStoryOpenErrorsSkipsArchivedRounds(t *testing.T) {
	s, storyDir := reviewViewStory(t)
	// Ba lần chấm cùng chương: lần 1, 2 thành reviews/rounds/, lần 3 là bài hiện tại.
	for _, verdict := range []string{"polish", "rewrite", "accept"} {
		if err := s.World.SaveReview(domain.ReviewEntry{
			Chapter: 1, Scope: "chapter", Verdict: verdict,
			Issues: []domain.ConsistencyIssue{{
				Severity: "error", Description: "Kết chương an toàn", Quote: "Tôi đi ngủ.", RequiresChange: true,
			}},
		}); err != nil {
			t.Fatalf("review %s: %v", verdict, err)
		}
	}
	engine := filepath.Join(storyDir, "output", "novel")
	roundsDir := filepath.Join(engine, "reviews", "rounds")
	archives, err := os.ReadDir(roundsDir)
	if err != nil || len(archives) != 2 {
		t.Fatalf("lưu trữ = %v, %v; muốn 2 vòng", archives, err)
	}
	// Đường đầy đủ (ma trận chất lượng) đọc lưu trữ — seam bên dưới không được
	// vô hiệu hoá view.
	view, ok, err := reviewViewReader(storyDir).cardView(1, store.ReviewScopeChapter)
	if err != nil || !ok || len(view.Previous) != 2 || view.Round != 3 {
		t.Fatalf("view đầy đủ = round %d, %d vòng, %v", view.Round, len(view.Previous), err)
	}
	for _, archive := range archives {
		if err := os.WriteFile(filepath.Join(roundsDir, archive.Name()), []byte("{hỏng"), 0o644); err != nil {
			t.Fatalf("làm hỏng lưu trữ: %v", err)
		}
	}

	// Badge thật (đường của GET /stories) vẫn đúng: 1 mục Bắt sửa của bài hiện tại,
	// không phải 3 mục lỗi của cả ba lần chấm, và không hỏng vì lưu trữ rác.
	if got, err := storyOpenErrors(library.Story{Dir: storyDir}, store.NewStore(engine)); err != nil || got != 1 {
		t.Fatalf("badge với vòng lưu trữ hỏng = %d, %v; muốn 1", got, err)
	}

	// Và không đường nào của badge chạm tới reviews/rounds/: thay bộ đọc lưu trữ
	// bằng một hàm chết.
	reader := reviewViewReader(storyDir)
	reader.loadRounds = func(int, store.ReviewScope) ([]domain.ReviewEntry, error) {
		t.Fatal("badge không được đọc reviews/rounds/")
		return nil, nil
	}
	if got, err := reader.openErrors(); err != nil || got != 1 {
		t.Fatalf("openErrors = %d, %v; muốn 1", got, err)
	}
}
