package web

import (
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

type qualitySectionAPI struct {
	Ref     string `json:"ref"`
	RefName string `json:"refName"`
}

type qualityRewriteAPI struct {
	At    string `json:"at"`    // RFC3339 lúc chương thực sự vào viết lại
	Cause string `json:"cause"` // review | user
}

type qualityChapterAPI struct {
	N        int                 `json:"n"`
	Title    string              `json:"title"`
	Status   string              `json:"status"`
	Rewrites []qualityRewriteAPI `json:"rewrites"` // câu 9 §H: luôn là mảng, [] khi không có nguồn
}

type qualityCellAPI struct {
	Ref     string `json:"ref"`
	Ch      int    `json:"ch"`
	Error   int    `json:"error"`
	Warning int    `json:"warning"`
	Fixed   int    `json:"fixed"`
	Kept    int    `json:"kept"`
}

type qualityIssueAPI struct {
	reviewItemAPI
	Ch         int    `json:"ch"`
	State      string `json:"state"`
	KeptReason string `json:"keptReason,omitempty"`
}

type qualityResponse struct {
	Pack     *reviewPackAPI      `json:"pack"`
	Sections []qualitySectionAPI `json:"sections"`
	Chapters []qualityChapterAPI `json:"chapters"`
	Cells    []qualityCellAPI    `json:"cells"`
	Issues   []qualityIssueAPI   `json:"issues"`
}

// The story dispatcher enforces visibility before reaching this read-only route.
func (g *Gateway) serveQuality(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	reader := newReviewReader(st.Dir, sr)
	issues, reviewed, err := qualityReviewIssues(reader)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	chapters, err := sr.Chapters()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	progress, err := sr.Progress()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rewrites, err := sr.Store().Progress.RewritesByChapter()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rewritesOf := func(ch int) []qualityRewriteAPI {
		entries := rewrites[ch]
		out := make([]qualityRewriteAPI, 0, len(entries))
		for _, entry := range entries {
			out = append(out, qualityRewriteAPI{At: entry.At, Cause: entry.Cause})
		}
		return out
	}
	out := qualityResponse{
		Pack: reader.pack, Sections: qualitySections(reader, issues),
		Chapters: make([]qualityChapterAPI, 0, len(chapters)),
		Cells:    []qualityCellAPI{}, Issues: issues,
	}
	known := make(map[int]bool, len(chapters))
	for _, chapter := range chapters {
		// Future outline-only chapters have no reviewed version and no D13 status.
		_, hasReview := reviewed[chapter.Chapter]
		if !chapter.Completed && !hasReview &&
			(progress == nil || !slices.Contains(progress.PendingRewrites, chapter.Chapter)) {
			continue
		}
		out.Chapters = append(out.Chapters, qualityChapterAPI{N: chapter.Chapter, Title: chapter.Title, Status: qualityChapterStatus(chapter.Chapter, progress, rewrites[chapter.Chapter]), Rewrites: rewritesOf(chapter.Chapter)})
		known[chapter.Chapter] = true
	}
	// Imported reviews may outlive their outline/progress entry. They still need a column.
	for ch := range reviewed {
		if !known[ch] {
			title := ""
			if summary, err := sr.Store().Summaries.LoadSummary(ch); err == nil && summary != nil {
				title = summary.Title
			}
			out.Chapters = append(out.Chapters, qualityChapterAPI{N: ch, Title: title, Status: qualityChapterStatus(ch, progress, rewrites[ch]), Rewrites: rewritesOf(ch)})
		}
	}
	sort.Slice(out.Chapters, func(i, j int) bool { return out.Chapters[i].N < out.Chapters[j].N })
	type cellKey struct {
		ref string
		ch  int
	}
	cells := make(map[cellKey]int)
	for _, issue := range issues {
		key := cellKey{issue.Ref, issue.Ch}
		index, ok := cells[key]
		if !ok {
			index = len(out.Cells)
			cells[key] = index
			out.Cells = append(out.Cells, qualityCellAPI{Ref: issue.Ref, Ch: issue.Ch})
		}
		cell := &out.Cells[index]
		switch issue.State {
		case "kept":
			cell.Kept++
		case "fixed":
			cell.Fixed++
		default:
			if issue.Level == reviewLevelError {
				cell.Error++
			} else {
				cell.Warning++
			}
		}
	}
	sectionRank := make(map[string]int, len(out.Sections))
	for i, section := range out.Sections {
		sectionRank[section.Ref] = i
	}
	sort.Slice(out.Cells, func(i, j int) bool {
		a, b := out.Cells[i], out.Cells[j]
		if a.Ref != b.Ref {
			return sectionRank[a.Ref] < sectionRank[b.Ref]
		}
		return a.Ch < b.Ch
	})
	writeJSON(w, http.StatusOK, out)
}

// Current and archived items come exclusively from the shared review view.
// A stable item is counted once even when both chapter and global cards mention it;
// presence in either latest card beats an archived occurrence in the other card.
// The returned map records which chapters have a saved review (they need a column
// even when the outline/progress no longer mentions them).
func qualityReviewIssues(reader *reviewReader) ([]qualityIssueAPI, map[int]bool, error) {
	cards, err := reader.cards()
	if err != nil {
		return nil, nil, err
	}
	reviewed := make(map[int]bool, len(cards))
	issues := []qualityIssueAPI{}
	seen := map[string]bool{}
	add := func(ch int, item reviewItemAPI, state string) {
		if seen[item.ID] {
			return
		}
		seen[item.ID] = true
		issue := qualityIssueAPI{reviewItemAPI: item, Ch: ch, State: state}
		if reviewItemKept(item) {
			issue.State = reviewKeptState
			issue.KeptReason = item.Decision.Reason
		}
		issues = append(issues, issue)
	}
	// One chapter at a time so the matrix sees the cards in the same order as
	// before: current chapter card, current global card, then their archived
	// rounds newest first (a fixed issue of round 1 outranks round 2's fix).
	for i := 0; i < len(cards); {
		chapter := cards[i].Chapter
		j := i
		for j < len(cards) && cards[j].Chapter == chapter {
			j++
		}
		group := cards[i:j]
		reviewed[chapter] = true
		for _, card := range group {
			for _, item := range card.View.Items {
				add(chapter, item, "open")
			}
		}
		for _, card := range group {
			for k := len(card.View.Previous) - 1; k >= 0; k-- {
				for _, item := range card.View.Previous[k].Items {
					add(chapter, item, "fixed")
				}
			}
		}
		i = j
	}
	return issues, reviewed, nil
}

func qualitySections(reader *reviewReader, issues []qualityIssueAPI) []qualitySectionAPI {
	byRef := make(map[string]string)
	for _, issue := range issues {
		if _, ok := byRef[issue.Ref]; !ok {
			byRef[issue.Ref] = issue.RefName
		}
	}
	sections := make([]qualitySectionAPI, 0, len(byRef))
	appendRef := func(ref string) {
		if title, ok := byRef[ref]; ok {
			sections = append(sections, qualitySectionAPI{Ref: ref, RefName: title})
			delete(byRef, ref)
		}
	}
	refs := reader.packRules.OrderedSectionRefs()
	// Master is always the first layer, even when a custom pack lists it later.
	for _, ref := range refs {
		if strings.HasPrefix(ref, "Master §") {
			appendRef(ref)
		}
	}
	for _, ref := range refs {
		appendRef(ref)
	}
	// Address terms, user rules and references absent from the pack retain encounter order.
	for _, issue := range issues {
		appendRef(issue.Ref)
	}
	return sections
}

// qualityChapterStatus maps progress + the rewrite log to the sheet's status.
// Câu 9 §H: "rewritten" chỉ khi log viết lại có mục của chương và chương không
// đang viết lại/chờ viết lại nữa; không suy từ bằng chứng thẩm duyệt.
func qualityChapterStatus(ch int, progress *domain.Progress, rewrites []domain.RewriteEntry) string {
	if progress != nil {
		if slices.Contains(progress.PendingRewrites, ch) {
			if progress.InProgressChapter == ch {
				return "rewriting"
			}
			return "previous"
		}
		if len(rewrites) > 0 {
			return "rewritten"
		}
		if slices.Contains(progress.CompletedChapters, ch) {
			return "approved"
		}
	}
	return "previous"
}

// storyOpenErrors đếm mục Bắt sửa còn mở của một truyện cho badge trên danh sách
// truyện. Dùng đường đọc riêng của module (openErrors) nên không mở các vòng đã
// lưu trữ; store của thẻ truyện được dùng lại thay vì mở thêm một store.
func storyOpenErrors(st library.Story, engine *store.Store) (int, error) {
	return newReviewReader(st.Dir, host.NewStoryReaderWithStore(engine)).openErrors()
}
