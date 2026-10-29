// contracts.go declares the domain storage contracts the Store facade exposes.
// The interfaces are extracted from the methods external callers actually use
// on the file-backed substores — they are not an invented generic CRUD layer.
// Every interface is satisfied by the file backend's concrete implementation
// (file*Store); the Postgres backend in internal/persistence/postgres must
// satisfy the same contracts before it can serve a domain, and startup refuses
// a backend that cannot supply all of them (see NewFromBackend).
package store

import (
	"github.com/datnt1998/noverse/internal/domain"
	llmtypes "github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/rules"
)

// BookStore persists book metadata (meta/book.json is facts; book.md derived).
type BookStore interface {
	Load() (*domain.BookMetadata, error)
	Save(book domain.BookMetadata) error
}

// ProgressStore persists the progress index (meta/progress.json).
type ProgressStore interface {
	Load() (*domain.Progress, error)
	Save(p *domain.Progress) error
	Init(totalChapters int) error
	SetTotalChapters(n int) error
	UpdatePhase(phase domain.Phase) error
	AdvancePhase(phase domain.Phase) error
	StartChapter(chapter int) error
	IsChapterCompleted(chapter int) (bool, error)
	MarkChapterComplete(chapter, wordCount int, hookType, dominantStrand string) error
	MarkComplete() error
	Reopen(chapters []int, reason, cause string) error
	ReopenContinue() error
	ClearInProgress() error
	UpdateVolumeArc(volume, arc int) error
	SetLayered(layered bool) error
	SetFlow(flow domain.FlowState) error
	SetPendingRewrites(chapters []int, reason, cause string) error
	ApplyReviewOutcome(flow domain.FlowState, chapters []int, reason, cause string, recheckedChapter int) (*domain.Progress, error)
	ValidatePendingRewrites(chapters []int) error
	CompleteRewrite(chapter int, recheck bool) error
	DropRechecksFrom(chapter int) error
	ClearPendingRewrites() error
	ValidateChapterWork(chapter int) error
	Rewrites() ([]domain.RewriteEntry, error)
	RewritesByChapter() (map[int][]domain.RewriteEntry, error)
}

// OutlineStore persists premise, flat/layered outlines, compass, retention
// fields, foundation audit and the outline-feedback pool.
type OutlineStore interface {
	SavePremise(content string) error
	LoadPremise() (string, error)
	SaveStoryRetention(fields domain.StoryRetention) error
	LoadStoryRetention() (domain.StoryRetention, error)
	SaveOutline(entries []domain.OutlineEntry) error
	LoadOutline() ([]domain.OutlineEntry, error)
	// ReconcileChapterRetention updates exactly one accepted chapter's
	// retention metadata (open_questions/promise/hook_type/midpoint_twist)
	// in flat+layered views under one lock; narrative fields must match.
	ReconcileChapterRetention(chapter int, entry domain.OutlineEntry) error
	GetChapterOutline(chapter int) (*domain.OutlineEntry, error)
	SaveLayeredOutline(volumes []domain.VolumeOutline) error
	LoadLayeredOutline() ([]domain.VolumeOutline, error)
	ClearLayeredOutline() error
	GetChapterFromLayered(chapter int) (*domain.OutlineEntry, error)
	LocateChapter(chapter int) (volume, arc int, err error)
	CheckArcBoundary(chapter int) (*ArcBoundary, error)
	CompletedArcBoundaries(lastCompleted int) ([]ArcBoundary, error)
	SaveCompass(compass domain.StoryCompass) error
	LoadCompass() (*domain.StoryCompass, error)
	SaveFoundationAudit(a domain.FoundationAudit) error
	LoadFoundationAudit() (*domain.FoundationAudit, error)
	AppendOutlineFeedback(fb ChapterFeedback) error
	LoadPendingOutlineFeedback() ([]ChapterFeedback, error)
	SaveOutlineFeedbackResolution(reason string, count int) error
	ClearOutlineFeedback() error
}

// DraftStore persists chapter plans, drafts and committed chapter text.
type DraftStore interface {
	SaveChapterPlan(plan domain.ChapterPlan) error
	LoadChapterPlan(chapter int) (*domain.ChapterPlan, error)
	SaveDraft(chapter int, content string) error
	AppendDraft(chapter int, content string) error
	LoadDraft(chapter int) (string, error)
	LoadChapterContent(chapter int) (string, int, error)
	SaveFinalChapter(chapter int, content string) error
	LoadChapterText(chapter int) (string, error)
	LoadChapterRange(from, to, maxRunes int) (map[int]string, error)
	ExtractDialogue(characterName string, aliases []string, maxSamples, maxCompletedChapter int) ([]string, error)
	ExtractStyleAnchors(maxAnchors, maxCompletedChapter int) ([]string, error)
}

// SummaryStore persists chapter/arc/volume summaries.
type SummaryStore interface {
	SaveSummary(sum domain.ChapterSummary) error
	LoadSummary(chapter int) (*domain.ChapterSummary, error)
	LoadSummaryTitle(chapter int) (string, error)
	LoadRecentSummaries(current, count int) ([]domain.ChapterSummary, error)
	SaveArcSummary(sum domain.ArcSummary) error
	LoadArcSummary(volume, arc int) (*domain.ArcSummary, error)
	LoadArcSummaries(volume int) ([]domain.ArcSummary, error)
	HasArcSummary(volume, arc int) (bool, error)
	SaveVolumeSummary(sum domain.VolumeSummary) error
	LoadVolumeSummary(volume int) (*domain.VolumeSummary, error)
	LoadAllVolumeSummaries() ([]domain.VolumeSummary, error)
	HasVolumeSummary(volume int) (bool, error)
	FindCharacterAppearances(names []string, endChapter, recentWindow int) (map[string]int, error)
}

// RunMetaStore persists run metadata: permits, pending asks/steers, planning
// tier and comment-review grants.
type RunMetaStore interface {
	Load() (*domain.RunMeta, error)
	Save(meta domain.RunMeta) error
	Init(style, provider, model string) error
	GrantAdvancePermit(chapter int) error
	ClearAdvancePermit(chapter int) error
	SetAdvanceHold(hold domain.AdvanceHold) error
	ClearAdvanceHold(expected domain.AdvanceHold) error
	SetAdvanceMode(mode domain.ChapterAdvanceMode) error
	SetPendingAsk(ask domain.PendingAsk) error
	ClearPendingAsk(expected domain.PendingAsk) error
	SetPendingSteer(input string) error
	ClearPendingSteer() error
	SetPlanningTier(tier domain.PlanningTier) error
	SetPlanStart(rec domain.PlanStartRecord) error
	SetStartPrompt(prompt string) error
	GrantCommentReview(next int, review domain.CommentReview) error
	ClearCommentReview(review domain.CommentReview) error
}

// UserRulesStore persists user-rule snapshots, runtime rules, lesson
// candidates and the normalization cache.
type UserRulesStore interface {
	Load() (*rules.Snapshot, error)
	LoadCurrent() (*rules.Snapshot, error)
	Save(snap *rules.Snapshot) error
	ApplySnapshot(next *rules.Snapshot) (pinnedChapter int, err error)
	UpdatePin(fn func(rules.Snapshot) rules.Snapshot) error
	Runtime() ([]RuntimeRule, error)
	AddRuntime(rule RuntimeRule) error
	RemoveRuntime(id string) (rule RuntimeRule, found bool, err error)
	ReplaceRuntime(list []RuntimeRule) error
	NormCache() map[string]NormCacheEntry
	PutNormCache(entries map[string]NormCacheEntry) error
	LessonRules() ([]rules.Candidate, bool, error)
	CaptureLessonRules(candidates []rules.Candidate) ([]rules.Candidate, error)
}

// SignalStore persists commit/review signals: pending commit, last commit and
// last review.
type SignalStore interface {
	SavePendingCommit(pending domain.PendingCommit) error
	LoadPendingCommit() (*domain.PendingCommit, error)
	ClearPendingCommit() error
	SaveLastCommit(result domain.CommitResult) error
	LoadLastCommit() (*domain.CommitResult, error)
	LoadAndClearLastCommit() (*domain.CommitResult, error)
	ClearLastCommit() error
	SaveLastReview(r domain.ReviewEntry) error
	LoadLastReviewSignal() (*domain.ReviewEntry, error)
	LoadAndClearLastReview() (*domain.ReviewEntry, error)
	ClearLastReview() error
	ClearStaleSignals()
}

// RuntimeStore persists the runtime queue and per-task logs.
type RuntimeStore interface {
	AppendQueue(item domain.RuntimeQueueItem) (domain.RuntimeQueueItem, error)
	LoadQueue() ([]domain.RuntimeQueueItem, error)
	LoadQueueAfter(afterSeq int64) ([]domain.RuntimeQueueItem, error)
	AppendTaskLog(taskID string, entry domain.RuntimeTaskLogEntry) error
	LoadTaskLog(taskID string) ([]domain.RuntimeTaskLogEntry, error)
	Reset() error
}

// CharacterStore persists characters and their per-arc snapshots.
type CharacterStore interface {
	Save(chars []domain.Character) error
	Load() ([]domain.Character, error)
	SaveSnapshots(volume, arc int, snapshots []domain.CharacterSnapshot) error
	LoadSnapshots(volume, arc int) ([]domain.CharacterSnapshot, error)
	LoadLatestSnapshots() ([]domain.CharacterSnapshot, error)
	ResolveIDs(refs ...string) ([]string, error)
}

// CastStore persists the cast ledger and recent-active view.
type CastStore interface {
	Save(entries []domain.CastEntry) error
	Load() ([]domain.CastEntry, error)
	MergeAppearances(
		chapter int,
		characters []string,
		intros []domain.CastIntro,
		knownCore map[string]bool,
	) error
	RecentActive(limit int) ([]domain.CastEntry, error)
}

// WorldStore persists the world model: rules, timeline, foreshadow, state
// changes, relationships, address terms, reviews and rule violations.
type WorldStore interface {
	LoadWorldRules() ([]domain.WorldRule, error)
	SaveWorldRules(rules []domain.WorldRule) error
	LoadTimeline() ([]domain.TimelineEvent, error)
	SaveTimeline(events []domain.TimelineEvent) error
	AppendTimelineEvents(newEvents []domain.TimelineEvent) error
	LoadRecentTimeline(current, window int) ([]domain.TimelineEvent, error)
	LoadForeshadowLedger() ([]domain.ForeshadowEntry, error)
	LoadActiveForeshadow() ([]domain.ForeshadowEntry, error)
	SaveForeshadowLedger(entries []domain.ForeshadowEntry) error
	UpdateForeshadow(chapter int, updates []domain.ForeshadowUpdate) error
	LoadStateChanges() ([]domain.StateChange, error)
	SaveStateChanges(changes []domain.StateChange) error
	AppendStateChanges(changes []domain.StateChange) error
	LoadRelationships() ([]domain.RelationshipEntry, error)
	SaveRelationships(entries []domain.RelationshipEntry) error
	UpdateRelationships(changes []domain.RelationshipEntry) error
	LoadRelationshipLog() ([]domain.RelationshipChange, error)
	ValidateRelationshipChange(chapter int, aRef, bRef, relation, typ string) error
	AppendRelationshipChange(chapter int, aRef, bRef, relation, reason, typ string) error
	LoadAddressTerms() ([]domain.AddressTerm, error)
	AddressAt(chapter int) ([]domain.AddressTerm, error)
	ValidateAddressTerms(chapter int, inputs []domain.AddressTermInput) error
	ValidateSetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) error
	SetAddressTerm(chapter int, fromRef, toRef, calls, self, reason string) ([]domain.AddressTerm, error)
	UpdateAddressTerms(chapter int, inputs []domain.AddressTermInput) error
	SaveReview(r domain.ReviewEntry) error
	LoadReview(chapter int) (*domain.ReviewEntry, error)
	LoadLastReview(fromChapter int) (*domain.ReviewEntry, error)
	LoadGlobalReview(chapter int) (*domain.ReviewEntry, error)
	HasArcReview(chapter int) (bool, error)
	HasGlobalReview(chapter int) (bool, error)
	LoadReviewRounds(chapter int, scope ReviewScope) ([]domain.ReviewEntry, error)
	LoadReviewsAffectingChapter(chapter int) ([]domain.ReviewEntry, error)
	LoadAuthorRevisionStyle() (*domain.AuthorRevisionStyle, error)
	SaveAuthorRevisionStyle(style domain.AuthorRevisionStyle) error
	LoadReviewDecisions() (map[string]ReviewDecision, error)
	SaveReviewDecision(itemID string, d ReviewDecision) error
	DeleteReviewDecision(itemID string) (bool, error)
	LoadStyleRules() (*domain.WritingStyleRules, error)
	SaveStyleRules(rules domain.WritingStyleRules) error
	SaveRuleViolations(chapter int, violations []rules.Violation) error
	LoadRuleViolations(chapter int) []rules.Violation
	RecentRuleViolations(last int) []ChapterViolations
	RuleViolationsByChapter() map[int][]rules.Violation
}

// CheckpointStore persists append-only step checkpoints.
type CheckpointStore interface {
	Append(scope domain.Scope, step, artifact, digest string) (*domain.Checkpoint, error)
	AppendArtifact(scope domain.Scope, step, artifact string) (*domain.Checkpoint, error)
	AppendArtifacts(scope domain.Scope, step string, artifacts ...string) (*domain.Checkpoint, error)
	Latest(scope domain.Scope) *domain.Checkpoint
	LatestByStep(scope domain.Scope, step string) *domain.Checkpoint
	LatestGlobal() *domain.Checkpoint
	All() []domain.Checkpoint
	Reset() error
	InitError() error
}

// SessionStore persists agent session logs.
type SessionStore interface {
	Log(rel string, msg llmtypes.AgentMessage) error
	LogCoCreate(entry any) error
	KeepToolResults(names ...string)
	SubAgentLogger(lookup ModelLookup) func(agentName, task string, msg llmtypes.AgentMessage)
}

// UsageStore persists token/cost usage state.
type UsageStore interface {
	Load() (*domain.UsageState, error)
	Save(state domain.UsageState) error
}

// SimulationStore persists the simulation profile.
type SimulationStore interface {
	Load() (*domain.SimulationProfile, error)
	Save(profile domain.SimulationProfile) error
}

// DecisionStore persists the append-only arbitration decision ledger.
type DecisionStore interface {
	Append(rec DecisionRecord) (DecisionRecord, error)
	Recent(n int) ([]DecisionRecord, error)
}

// ChapterRecordStore persists per-chapter canonical records.
type ChapterRecordStore interface {
	Load(chapter int) (*domain.ChapterRecord, error)
	LoadCompleted(chapters []int) ([]domain.ChapterRecord, error)
	Save(record domain.ChapterRecord) error
	Prepare(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error)
	Accept(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error)
}

// RevisionStore persists the pending-revision marker (external edits).
type RevisionStore interface {
	LoadPending() (*domain.PendingRevision, error)
	SavePending(pending domain.PendingRevision) error
	ClearPending() error
}
