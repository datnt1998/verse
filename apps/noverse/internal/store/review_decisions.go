package store

import (
	"os"
)

// GAP-47: quyết định của người dùng trên từng mục thẩm duyệt ("Giữ nguyên" kèm
// lý do). Lưu ở reviews/decisions.json, khoá là itemId ổn định của DTO
// (băm ref + chương + trích). Tách khỏi reviews/NN.json để engine ghi lại bài
// thẩm duyệt mà không làm mất quyết định, và để quyết định sống qua nhiều vòng chấm.
const reviewDecisionsFile = "reviews/decisions.json"

// ReviewDecision là một quyết định "Giữ nguyên". State hiện chỉ có "kept";
// Ref/RefName/Text/Quote/Chapter chép lại nội dung mục để lần chấm sau còn
// nguyên ngữ cảnh (mục có thể không còn trong bài thẩm duyệt mới).
type ReviewDecision struct {
	State   string `json:"state"`
	Reason  string `json:"reason"`
	By      string `json:"by"`
	At      string `json:"at"`
	Ref     string `json:"ref,omitempty"`
	RefName string `json:"refName,omitempty"`
	Text    string `json:"text,omitempty"`
	Quote   string `json:"quote,omitempty"`
	Chapter int    `json:"chapter,omitempty"`
}

type reviewDecisionsDoc struct {
	Items map[string]ReviewDecision `json:"items"`
}

// LoadReviewDecisions đọc toàn bộ quyết định; chưa có tệp thì trả map rỗng.
func (s *fileWorldStore) LoadReviewDecisions() (map[string]ReviewDecision, error) {
	var doc reviewDecisionsDoc
	if err := s.io.ReadJSON(reviewDecisionsFile, &doc); err != nil {
		if os.IsNotExist(err) {
			return map[string]ReviewDecision{}, nil
		}
		return nil, err
	}
	if doc.Items == nil {
		doc.Items = map[string]ReviewDecision{}
	}
	return doc.Items, nil
}

// SaveReviewDecision ghi đè quyết định của một mục (giữ nguyên các mục khác).
func (s *fileWorldStore) SaveReviewDecision(itemID string, d ReviewDecision) error {
	if itemID == "" {
		return nil
	}
	return s.io.WithWriteLock(func() error {
		doc, err := s.readReviewDecisionsUnlocked()
		if err != nil {
			return err
		}
		doc.Items[itemID] = d
		return s.io.WriteJSONUnlocked(reviewDecisionsFile, doc)
	})
}

// DeleteReviewDecision bỏ quyết định của một mục; trả về true nếu có gì để bỏ.
func (s *fileWorldStore) DeleteReviewDecision(itemID string) (bool, error) {
	if itemID == "" {
		return false, nil
	}
	removed := false
	err := s.io.WithWriteLock(func() error {
		doc, err := s.readReviewDecisionsUnlocked()
		if err != nil {
			return err
		}
		if _, ok := doc.Items[itemID]; !ok {
			return nil
		}
		delete(doc.Items, itemID)
		removed = true
		return s.io.WriteJSONUnlocked(reviewDecisionsFile, doc)
	})
	return removed, err
}

func (s *fileWorldStore) readReviewDecisionsUnlocked() (reviewDecisionsDoc, error) {
	var doc reviewDecisionsDoc
	if err := s.io.ReadJSONUnlocked(reviewDecisionsFile, &doc); err != nil {
		if !os.IsNotExist(err) {
			return doc, err
		}
	}
	if doc.Items == nil {
		doc.Items = map[string]ReviewDecision{}
	}
	return doc, nil
}
