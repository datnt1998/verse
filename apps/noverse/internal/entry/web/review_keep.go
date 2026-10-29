package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// GAP-47 / D4 · câu 14: "Giữ nguyên" một mục Cảnh báo kèm lý do, và bỏ giữ.
//
//	POST   /api/projects/{p}/stories/{s}/review/items/{itemId}/keep {reason}
//	DELETE /api/projects/{p}/stories/{s}/review/items/{itemId}/keep
//
// Quyết định lưu ở reviews/decisions.json (store.ReviewDecision), khoá là itemId
// ổn định của DTO (băm ref + chương + trích) nên sống qua các vòng chấm. Chỉ mục
// Cảnh báo mới giữ được: mục Bắt sửa phải sửa, không được giữ (409).
const (
	reviewItemNotFound       = "Không tìm thấy mục thẩm duyệt"
	reviewKeepNeedsReason    = "Cần ghi lý do giữ nguyên"
	reviewKeepWarningOnly    = "Mục Bắt sửa không giữ nguyên được, phải sửa"
	reviewItemIDChapterRegex = `^ch(\d+)-`
)

var reviewItemChapterPattern = regexp.MustCompile(reviewItemIDChapterRegex)

// serveReviewKeep xử lý hai method của /review/items/{itemId}/keep. Trả false khi
// đường dẫn không phải của nó (để caller chuyển tiếp worker).
func (g *Gateway) serveReviewKeep(w http.ResponseWriter, r *http.Request, user webauth.User, st library.Story, sr *host.StoryReader, tail string) bool {
	itemID, ok := strings.CutSuffix(tail, "/keep")
	if !ok || !strings.HasPrefix(itemID, "items/") {
		return false
	}
	itemID = strings.TrimPrefix(itemID, "items/")
	if itemID == "" || strings.Contains(itemID, "/") {
		return false
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		return false
	}
	chapter, ok := reviewItemChapter(itemID)
	if !ok || sr.Store() == nil {
		jsonError(w, http.StatusNotFound, reviewItemNotFound)
		return true
	}
	world := sr.Store().World
	if r.Method == http.MethodDelete {
		removed, err := world.DeleteReviewDecision(itemID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return true
		}
		if !removed {
			if _, found := findReviewItem(sr, st, chapter, itemID); !found {
				jsonError(w, http.StatusNotFound, reviewItemNotFound)
				return true
			}
		}
		g.publishReviewChanged(st)
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	item, found := findReviewItem(sr, st, chapter, itemID)
	if !found {
		jsonError(w, http.StatusNotFound, reviewItemNotFound)
		return true
	}
	if item.Level != reviewLevelWarning {
		jsonError(w, http.StatusConflict, reviewKeepWarningOnly)
		return true
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &in) {
		return true
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		jsonError(w, http.StatusBadRequest, reviewKeepNeedsReason)
		return true
	}
	decision := store.ReviewDecision{
		State:   reviewKeptState,
		Reason:  reason,
		By:      reviewActorName(user),
		At:      time.Now().Format(time.RFC3339),
		Ref:     item.Ref,
		RefName: item.RefName,
		Text:    item.Text,
		Quote:   item.Quote,
		Chapter: chapter,
	}
	if err := world.SaveReviewDecision(itemID, decision); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	g.publishReviewChanged(st)
	writeJSON(w, http.StatusOK, reviewDecisionAPI{
		State: decision.State, Reason: decision.Reason, By: decision.By, At: decision.At,
	})
	return true
}

// reviewItemChapter lấy số chương từ itemId "ch<N>-<băm>[-<n>]" — id luôn mang
// chương nên không phải quét mọi bài thẩm duyệt để tìm mục.
func reviewItemChapter(itemID string) (int, bool) {
	m := reviewItemChapterPattern.FindStringSubmatch(itemID)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// findReviewItem dựng lại thẻ của chương và tìm đúng itemId. Bài tổng hợp
// (global/arc) được xét trước bài chương, cùng thứ tự với cửa duyệt.
func findReviewItem(sr *host.StoryReader, st library.Story, chapter int, itemID string) (reviewItemAPI, bool) {
	compound, chapterCard, err := newReviewReader(st.Dir, sr).chapterViews(chapter)
	if err != nil {
		return reviewItemAPI{}, false
	}
	for _, entry := range []*reviewEntryAPI{compound, chapterCard} {
		if entry == nil {
			continue
		}
		for _, item := range entry.Items {
			if item.ID == itemID {
				return item, true
			}
		}
	}
	return reviewItemAPI{}, false
}

// reviewActorName là tên hiển thị ghi vào quyết định; thiếu thì dùng tên đăng nhập.
func reviewActorName(user webauth.User) string {
	return firstNonEmpty(strings.TrimSpace(user.Name), user.Username)
}

// publishReviewChanged phát story.changed part "reviews" để các màn đang mở làm
// mới thẻ thẩm duyệt (16-feedback-refresh).
func (g *Gateway) publishReviewChanged(st library.Story) {
	g.events().Publish(Event{
		Type: "story.changed", Project: st.Project, Story: st.Story,
		Data: map[string]any{"parts": []string{"reviews"}},
	})
}
