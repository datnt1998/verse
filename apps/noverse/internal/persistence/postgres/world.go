package postgres

import (
	"context"
	"slices"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// pgWorldStore keeps every world fact on the document surface (rules,
// timeline, foreshadow, relationships, address terms, style rules, violations)
// and serves the review family from the typed reviews/review_decisions rows.
// Reviews are rounds with kept decisions — copying them into documents would
// fork the same fact into two sources.
type pgWorldStore struct {
	inner store.WorldStore
	nar   *Narrative
	ctx   context.Context
	scope ScopeRef
}

// ── reviews (typed rows) ────────────────────────────────────────────────────

func (w *pgWorldStore) SaveReview(r domain.ReviewEntry) error {
	return w.nar.SaveReview(w.ctx, w.scope, r)
}

func (w *pgWorldStore) LoadReview(chapter int) (*domain.ReviewEntry, error) {
	return w.nar.LoadReview(w.ctx, w.scope, chapter, store.ReviewScopeChapter)
}

func (w *pgWorldStore) LoadGlobalReview(chapter int) (*domain.ReviewEntry, error) {
	return w.nar.LoadReview(w.ctx, w.scope, chapter, store.ReviewScopeGlobal)
}

func (w *pgWorldStore) LoadLastReview(fromChapter int) (*domain.ReviewEntry, error) {
	return w.nar.LatestGlobalReviewAtOrBefore(w.ctx, w.scope, fromChapter)
}

func (w *pgWorldStore) HasArcReview(chapter int) (bool, error) {
	rv, err := w.LoadReview(chapter)
	if err != nil {
		return false, err
	}
	return rv != nil && rv.Scope == "arc", nil
}

func (w *pgWorldStore) HasGlobalReview(chapter int) (bool, error) {
	rv, err := w.LoadGlobalReview(chapter)
	if err != nil {
		return false, err
	}
	return rv != nil && rv.Scope == "global", nil
}

func (w *pgWorldStore) LoadReviewRounds(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error) {
	return w.nar.LoadReviewRounds(w.ctx, w.scope, chapter, scope)
}

// LoadReviewsAffectingChapter trả các review đang hiệu lực có đưa chương vào
// hàng đợi viết lại, mới → cũ. Arc/global review ghi ở chương kết thúc nên
// không thể tra theo tên file như mode file; payload giữ affected chapters.
func (w *pgWorldStore) LoadReviewsAffectingChapter(chapter int) ([]domain.ReviewEntry, error) {
	all, err := w.nar.LatestReviewsForStory(w.ctx, w.scope)
	if err != nil {
		return nil, err
	}
	var out []domain.ReviewEntry
	for _, r := range all {
		if slices.Contains(r.AffectedChapters, chapter) ||
			(r.Scope == "chapter" && r.Chapter == chapter && r.Verdict != "accept" && len(r.AffectedChapters) == 0) {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.ReviewEntry) int { return b.Chapter - a.Chapter })
	return out, nil
}

func (w *pgWorldStore) LoadReviewDecisions() (map[string]store.ReviewDecision, error) {
	return w.nar.LoadReviewDecisions(w.ctx, w.scope)
}

func (w *pgWorldStore) SaveReviewDecision(itemID string, d store.ReviewDecision) error {
	return w.nar.SaveReviewDecision(w.ctx, w.scope, itemID, d)
}

func (w *pgWorldStore) DeleteReviewDecision(itemID string) (bool, error) {
	return w.nar.DeleteReviewDecision(w.ctx, w.scope, itemID)
}

// ── documents (delegated to the file rules over the document surface) ───────

func (w *pgWorldStore) LoadWorldRules() ([]domain.WorldRule, error) { return w.inner.LoadWorldRules() }
func (w *pgWorldStore) SaveWorldRules(r []domain.WorldRule) error   { return w.inner.SaveWorldRules(r) }

func (w *pgWorldStore) LoadTimeline() ([]domain.TimelineEvent, error) { return w.inner.LoadTimeline() }
func (w *pgWorldStore) SaveTimeline(e []domain.TimelineEvent) error   { return w.inner.SaveTimeline(e) }
func (w *pgWorldStore) AppendTimelineEvents(e []domain.TimelineEvent) error {
	return w.inner.AppendTimelineEvents(e)
}
func (w *pgWorldStore) LoadRecentTimeline(current, window int) ([]domain.TimelineEvent, error) {
	return w.inner.LoadRecentTimeline(current, window)
}

func (w *pgWorldStore) LoadForeshadowLedger() ([]domain.ForeshadowEntry, error) {
	return w.inner.LoadForeshadowLedger()
}
func (w *pgWorldStore) LoadActiveForeshadow() ([]domain.ForeshadowEntry, error) {
	return w.inner.LoadActiveForeshadow()
}
func (w *pgWorldStore) SaveForeshadowLedger(e []domain.ForeshadowEntry) error {
	return w.inner.SaveForeshadowLedger(e)
}
func (w *pgWorldStore) UpdateForeshadow(chapter int, updates []domain.ForeshadowUpdate) error {
	return w.inner.UpdateForeshadow(chapter, updates)
}

func (w *pgWorldStore) LoadStateChanges() ([]domain.StateChange, error) {
	return w.inner.LoadStateChanges()
}
func (w *pgWorldStore) SaveStateChanges(c []domain.StateChange) error {
	return w.inner.SaveStateChanges(c)
}
func (w *pgWorldStore) AppendStateChanges(c []domain.StateChange) error {
	return w.inner.AppendStateChanges(c)
}

func (w *pgWorldStore) LoadRelationships() ([]domain.RelationshipEntry, error) {
	return w.inner.LoadRelationships()
}
func (w *pgWorldStore) SaveRelationships(e []domain.RelationshipEntry) error {
	return w.inner.SaveRelationships(e)
}
func (w *pgWorldStore) UpdateRelationships(c []domain.RelationshipEntry) error {
	return w.inner.UpdateRelationships(c)
}
func (w *pgWorldStore) LoadRelationshipLog() ([]domain.RelationshipChange, error) {
	return w.inner.LoadRelationshipLog()
}
func (w *pgWorldStore) ValidateRelationshipChange(chapter int, aRef, bRef, relation, typ string) error {
	return w.inner.ValidateRelationshipChange(chapter, aRef, bRef, relation, typ)
}
func (w *pgWorldStore) AppendRelationshipChange(chapter int, aRef, bRef, relation, reason, typ string) error {
	return w.inner.AppendRelationshipChange(chapter, aRef, bRef, relation, reason, typ)
}

func (w *pgWorldStore) LoadAddressTerms() ([]domain.AddressTerm, error) {
	return w.inner.LoadAddressTerms()
}
func (w *pgWorldStore) AddressAt(chapter int) ([]domain.AddressTerm, error) {
	return w.inner.AddressAt(chapter)
}
func (w *pgWorldStore) ValidateAddressTerms(chapter int, in []domain.AddressTermInput) error {
	return w.inner.ValidateAddressTerms(chapter, in)
}
func (w *pgWorldStore) ValidateSetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) error {
	return w.inner.ValidateSetAddressTerm(chapter, fromRef, toRef, calls, self, reason)
}
func (w *pgWorldStore) SetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) ([]domain.AddressTerm, error) {
	return w.inner.SetAddressTerm(chapter, fromRef, toRef, calls, self, reason)
}
func (w *pgWorldStore) UpdateAddressTerms(chapter int, in []domain.AddressTermInput) error {
	return w.inner.UpdateAddressTerms(chapter, in)
}

func (w *pgWorldStore) LoadAuthorRevisionStyle() (*domain.AuthorRevisionStyle, error) {
	return w.inner.LoadAuthorRevisionStyle()
}
func (w *pgWorldStore) SaveAuthorRevisionStyle(s domain.AuthorRevisionStyle) error {
	return w.inner.SaveAuthorRevisionStyle(s)
}

func (w *pgWorldStore) LoadStyleRules() (*domain.WritingStyleRules, error) {
	return w.inner.LoadStyleRules()
}
func (w *pgWorldStore) SaveStyleRules(r domain.WritingStyleRules) error {
	return w.inner.SaveStyleRules(r)
}

func (w *pgWorldStore) SaveRuleViolations(chapter int, v []rules.Violation) error {
	return w.inner.SaveRuleViolations(chapter, v)
}
func (w *pgWorldStore) LoadRuleViolations(chapter int) []rules.Violation {
	return w.inner.LoadRuleViolations(chapter)
}
func (w *pgWorldStore) RecentRuleViolations(last int) []store.ChapterViolations {
	return w.inner.RecentRuleViolations(last)
}
func (w *pgWorldStore) RuleViolationsByChapter() map[int][]rules.Violation {
	return w.inner.RuleViolationsByChapter()
}

var _ store.WorldStore = (*pgWorldStore)(nil)
