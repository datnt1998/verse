package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/errs"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/store"
)

// ReviewScopeKey chuẩn hoá (chapter, scope) thành khóa scope_key dạng file
// backend: "03" | "03-global". Review không theo chương (global không gắn mốc)
// dùng key "global".
func ReviewScopeKey(chapter int, scope store.ReviewScope) string {
	if scope == store.ReviewScopeGlobal {
		if chapter > 0 {
			return fmt.Sprintf("%02d-global", chapter)
		}
		return "global"
	}
	return fmt.Sprintf("%02d", chapter)
}

// SaveReview lưu một bản chấm như một round mới của scope_key: file backend
// archive bản cũ thành reviews/rounds/<k>; DB giữ nguyên mọi round (append).
// Cùng input trên cùng round mới nhất là idempotent — không append trùng.
// Reviewer mặc định agent=editor (người chấm trong file mode); reviewer
// khác truyền qua SaveReviewWith.
func (n *Narrative) SaveReview(ctx context.Context, scope ScopeRef, r domain.ReviewEntry) error {
	return n.SaveReviewWith(ctx, scope, r, ChapterAuthor{Kind: "agent", AgentName: "editor"})
}

// SaveReviewWith lưu review với attribution tường minh (user/agent/import).
func (n *Narrative) SaveReviewWith(ctx context.Context, scope ScopeRef, r domain.ReviewEntry, reviewer ChapterAuthor) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if _, err := q.LockStoryByID(ctx, gen.LockStoryByIDParams{ProjectID: scope.ProjectID, ID: scope.StoryID}); err != nil {
			if noRows(err) {
				return errs.ErrStorageNotFound
			}
			return sanitizeErr(err)
		}
		key := ReviewScopeKey(r.Chapter, store.ReviewScopeOf(r.Scope))
		latest, err := q.LatestReviewByScope(ctx, gen.LatestReviewByScopeParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, ScopeKey: key})
		if err == nil && canonicalJSON(latest.Payload) == canonicalJSON(payload) {
			return nil // same input → idempotent, no duplicate round
		}
		if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		var chapterID pgtype.UUID
		var chapterRev pgtype.Int4
		if r.Chapter > 0 {
			if ch, err := n.ensureChapter(ctx, q, scope, r.Chapter); err == nil && ch.CurrentRevision.Valid {
				chapterID = ch.ID
				chapterRev = ch.CurrentRevision
			}
		}
		var reviewerKind, agentName string
		var reviewerID pgtype.UUID
		switch reviewer.Kind {
		case "user":
			u, err := q.GetUserByUsername(ctx, reviewer.Username)
			if err != nil {
				if noRows(err) {
					return fmt.Errorf("người chấm %s: %w", reviewer.Username, errs.ErrStorageNotFound)
				}
				return sanitizeErr(err)
			}
			reviewerKind, reviewerID = "user", u.ID
		case "import":
			reviewerKind = "import"
		default:
			if strings.TrimSpace(reviewer.AgentName) == "" {
				reviewer.AgentName = "editor"
			}
			reviewerKind, agentName = "agent", reviewer.AgentName
		}
		id, err := randomUUID()
		if err != nil {
			return err
		}
		_, err = q.InsertReviewNextRound(ctx, gen.InsertReviewNextRoundParams{
			ProjectID:       scope.ProjectID,
			StoryID:         scope.StoryID,
			ID:              id,
			ScopeKey:        key,
			ChapterID:       chapterID,
			ChapterRevision: chapterRev,
			Payload:         payload,
			ReviewerKind:    reviewerKind,
			ReviewerUserID:  reviewerID,
			AgentName:       pgtype.Text{String: agentName, Valid: agentName != ""},
		})
		return sanitizeErr(err)
	})
}

func canonicalJSON(b []byte) string {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return string(b)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(b)
	}
	return string(out)
}

// LoadReview đọc round mới nhất của một scope_key; thiếu → (nil,nil).
func (n *Narrative) LoadReview(ctx context.Context, scope ScopeRef, chapter int, scopeKind store.ReviewScope) (*domain.ReviewEntry, error) {
	var out *domain.ReviewEntry
	err := n.tx(ctx, func(tx pgx.Tx) error {
		row, err := n.q(tx).LatestReviewByScope(ctx, gen.LatestReviewByScopeParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			ScopeKey:  ReviewScopeKey(chapter, scopeKind),
		})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		var r domain.ReviewEntry
		if err := json.Unmarshal(row.Payload, &r); err != nil {
			return fmt.Errorf("đọc review: %w", err)
		}
		out = &r
		return nil
	})
	return out, err
}

// LoadReviewRounds trả các round đã lưu của scope_key, cũ nhất trước — trừ
// round hiện hành (đọc qua LoadReview) theo đúng contract file.
func (n *Narrative) LoadReviewRounds(ctx context.Context, scope ScopeRef, chapter int, scopeKind store.ReviewScope) ([]domain.ReviewEntry, error) {
	out := []domain.ReviewEntry{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		rows, err := n.q(tx).ListReviewRounds(ctx, gen.ListReviewRoundsParams{
			ProjectID: scope.ProjectID,
			StoryID:   scope.StoryID,
			ScopeKey:  ReviewScopeKey(chapter, scopeKind),
		})
		if err != nil {
			return sanitizeErr(err)
		}
		if len(rows) > 0 {
			rows = rows[:len(rows)-1]
		}
		for _, row := range rows {
			var r domain.ReviewEntry
			if err := json.Unmarshal(row.Payload, &r); err != nil {
				return fmt.Errorf("đọc review round: %w", err)
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// SaveReviewDecision ghi quyết định "kept" của một mục, gắn vào review mới
// nhất của truyện (decisions sống lâu hơn một round nhưng FK đòi review_id —
// chọn review mới nhất làm neo; payload giữ decision đầy đủ).
func (n *Narrative) SaveReviewDecision(ctx context.Context, scope ScopeRef, itemID string, d store.ReviewDecision) error {
	payload, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		reviews, err := q.LatestReviewsForStory(ctx, gen.LatestReviewsForStoryParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		if len(reviews) == 0 {
			return fmt.Errorf("chưa có bài chấm để neo quyết định: %w", errs.ErrStorageNotFound)
		}
		var actorID pgtype.UUID
		if strings.TrimSpace(d.By) != "" {
			if u, err := q.GetUserByUsername(ctx, d.By); err == nil {
				actorID = u.ID
			} else if !noRows(err) {
				return sanitizeErr(err)
			}
		}
		return sanitizeErr(q.UpsertReviewDecision(ctx, gen.UpsertReviewDecisionParams{
			ProjectID:   scope.ProjectID,
			ReviewID:    reviews[0].ID,
			ItemKey:     itemID,
			ActorUserID: actorID,
			Payload:     payload,
		}))
	})
}

// LoadReviewDecisions đọc map itemID → quyết định (parity file
// reviews/decisions.json); chưa có → map rỗng.
func (n *Narrative) LoadReviewDecisions(ctx context.Context, scope ScopeRef) (map[string]store.ReviewDecision, error) {
	out := map[string]store.ReviewDecision{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		rows, err := n.q(tx).ListReviewDecisions(ctx, gen.ListReviewDecisionsParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, row := range rows {
			var d store.ReviewDecision
			if err := json.Unmarshal(row.Payload, &d); err != nil {
				return fmt.Errorf("đọc review decision: %w", err)
			}
			out[row.ItemKey] = d
		}
		return nil
	})
	return out, err
}

// DeleteReviewDecision xoá quyết định của một mục — parity file (bool found).
func (n *Narrative) DeleteReviewDecision(ctx context.Context, scope ScopeRef, itemID string) (bool, error) {
	found := false
	err := n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		reviews, err := q.LatestReviewsForStory(ctx, gen.LatestReviewsForStoryParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, r := range reviews {
			tag, err := q.DeleteReviewDecision(ctx, gen.DeleteReviewDecisionParams{
				ProjectID: scope.ProjectID, ReviewID: r.ID, ItemKey: itemID})
			if err != nil {
				return sanitizeErr(err)
			}
			if tag > 0 {
				found = true
			}
		}
		return nil
	})
	return found, err
}

// LatestGlobalReviewAtOrBefore đọc review global mới nhất ở chương ≤ number —
// row twin của việc quét ngược reviews/NN-global.json (LoadLastReview).
func (n *Narrative) LatestGlobalReviewAtOrBefore(ctx context.Context, scope ScopeRef, number int) (*domain.ReviewEntry, error) {
	if number <= 0 {
		return nil, nil
	}
	var out *domain.ReviewEntry
	err := n.tx(ctx, func(tx pgx.Tx) error {
		row, err := n.q(tx).LatestGlobalReviewAtOrBefore(ctx, gen.LatestGlobalReviewAtOrBeforeParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(number)})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		var r domain.ReviewEntry
		if err := json.Unmarshal(row.Payload, &r); err != nil {
			return fmt.Errorf("đọc review global: %w", err)
		}
		out = &r
		return nil
	})
	return out, err
}

// LatestReviewsForStory đọc round mới nhất của mọi scope_key còn hiệu lực
// (kèm số chương), dùng cho HasArc/HasGlobal và LoadReviewsAffectingChapter.
func (n *Narrative) LatestReviewsForStory(ctx context.Context, scope ScopeRef) ([]domain.ReviewEntry, error) {
	out := []domain.ReviewEntry{}
	err := n.tx(ctx, func(tx pgx.Tx) error {
		rows, err := n.q(tx).ListLatestReviews(ctx, gen.ListLatestReviewsParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, row := range rows {
			var r domain.ReviewEntry
			if err := json.Unmarshal(row.Payload, &r); err != nil {
				return fmt.Errorf("đọc review %s: %w", row.ScopeKey, err)
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// InvalidateReviewsFromChapter vô hiệu hoá mọi review ở chương ≥ fromChapter
// và xoá quyết định "kept" gắn với chúng — một transaction, đúng nghĩa
// invalidate-on-rewrite của phase 05 (file mode xoá file + rounds đã lưu).
func (n *Narrative) InvalidateReviewsFromChapter(ctx context.Context, scope ScopeRef, fromChapter int) error {
	if fromChapter <= 0 {
		return fmt.Errorf("from chapter must be > 0")
	}
	return n.tx(ctx, func(tx pgx.Tx) error {
		q := n.q(tx)
		if err := q.InvalidateReviewsFromChapter(ctx, gen.InvalidateReviewsFromChapterParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID, Number: int32(fromChapter)}); err != nil {
			return sanitizeErr(err)
		}
		return sanitizeErr(q.DeleteReviewDecisionsOfInvalidated(ctx, gen.DeleteReviewDecisionsOfInvalidatedParams{
			ProjectID: scope.ProjectID, StoryID: scope.StoryID}))
	})
}
