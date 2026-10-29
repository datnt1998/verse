package postgres_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/store/narrativetest"
)

// deterministicUUID re-implements the phase-04 anchor derivation for tests.
var deterministicUUID = func(ns, name string) pgtype.UUID {
	sum := sha256.Sum256([]byte("ainovel|v1|" + ns + "|" + name))
	var id pgtype.UUID
	copy(id.Bytes[:], sum[:16])
	id.Bytes[6] = id.Bytes[6]&0x0f | 0x50
	id.Bytes[8] = id.Bytes[8]&0x3f | 0x80
	id.Valid = true
	return id
}

// pgFixture anchors one project+story on a migrated schema and returns the
// repository, the resolved scope and raw access for SQL-level assertions.
type pgFixture struct {
	nar    *postgres.Narrative
	pool   *postgres.Pool
	scope  postgres.ScopeRef
	projID string // uuid string of the project row
}

// provisionNarrative creates project slug via gen (anchor like phase 04) and a
// story via the repository under test.
func provisionNarrative(t *testing.T, projectSlug, storySlug string) pgFixture {
	t.Helper()
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	ctx := context.Background()
	q := gen.New(pool.Pool())
	if _, err := q.EnsureProject(ctx, gen.EnsureProjectParams{
		ID:   deterministicUUID("project", projectSlug),
		Slug: projectSlug, Name: projectSlug,
	}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	nar := postgres.NewNarrative(pool)
	if _, err := nar.CreateStory(ctx, projectSlug, nil, postgres.StoryInfo{
		Slug: storySlug, Title: "Truyện thử", Status: "writing",
	}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, projectSlug, storySlug, nil, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return pgFixture{nar: nar, pool: pool, scope: scope}
}

// ── suite adapters ──────────────────────────────────────────────────────────

type pgBook struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgBook) Load() (*domain.BookMetadata, error) {
	return p.nar.BookLoad(context.Background(), p.scope)
}
func (p pgBook) Save(b domain.BookMetadata) error {
	return p.nar.BookSave(context.Background(), p.scope, b)
}

type pgProgress struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgProgress) Load() (*domain.Progress, error) {
	return p.nar.ProgressLoad(context.Background(), p.scope)
}
func (p pgProgress) Save(pr *domain.Progress) error {
	return p.nar.ProgressSave(context.Background(), p.scope, pr)
}

type pgOutline struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgOutline) SaveOutline(e []domain.OutlineEntry) error {
	return p.nar.SaveFlatOutline(context.Background(), p.scope, e)
}
func (p pgOutline) LoadOutline() ([]domain.OutlineEntry, error) {
	return p.nar.LoadFlatOutline(context.Background(), p.scope)
}
func (p pgOutline) SaveLayeredOutline(v []domain.VolumeOutline) error {
	return p.nar.SaveLayeredOutline(context.Background(), p.scope, v)
}
func (p pgOutline) LoadLayeredOutline() ([]domain.VolumeOutline, error) {
	return p.nar.LoadLayeredOutline(context.Background(), p.scope)
}
func (p pgOutline) ClearLayeredOutline() error {
	return p.nar.ClearLayeredOutline(context.Background(), p.scope)
}

type pgDrafts struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgDrafts) SaveDraft(chapter int, content string) error {
	return p.nar.SaveDraft(context.Background(), p.scope, chapter, content)
}
func (p pgDrafts) LoadDraft(chapter int) (string, error) {
	return p.nar.LoadDraft(context.Background(), p.scope, chapter)
}

type pgRecords struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgRecords) Load(chapter int) (*domain.ChapterRecord, error) {
	return p.nar.LoadRecord(context.Background(), p.scope, chapter)
}
func (p pgRecords) Accept(chapter int, origin domain.ChapterOrigin, content string, facts domain.ChapterFacts, style domain.StyleDelta) (*domain.ChapterRecord, error) {
	return p.nar.AcceptRecord(context.Background(), p.scope, chapter, origin, content, facts, style,
		postgres.ChapterAuthor{Kind: "agent", AgentName: "writer"})
}

type pgChars struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgChars) Save(chars []domain.Character) error {
	return p.nar.SaveCharacters(context.Background(), p.scope, chars)
}
func (p pgChars) Load() ([]domain.Character, error) {
	return p.nar.LoadCharacters(context.Background(), p.scope)
}

type pgCheckpoints struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
	all   []domain.Checkpoint
	err   error
}

func (p *pgCheckpoints) Append(scope domain.Scope, step, artifact, digest string) (*domain.Checkpoint, error) {
	cp, err := p.nar.CheckpointAppend(context.Background(), p.scope, scope, step, artifact, digest)
	if err != nil {
		return nil, err
	}
	p.all = append(p.all, *cp)
	return cp, nil
}
func (p *pgCheckpoints) All() []domain.Checkpoint {
	if p.err != nil {
		return nil
	}
	all, err := p.nar.Checkpoints(context.Background(), p.scope)
	if err != nil {
		p.err = err
		return nil
	}
	return all
}

type pgDecisions struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgDecisions) Append(rec store.DecisionRecord) (store.DecisionRecord, error) {
	return p.nar.DecisionAppend(context.Background(), p.scope, rec)
}
func (p pgDecisions) Recent(n int) ([]store.DecisionRecord, error) {
	return p.nar.DecisionsRecent(context.Background(), p.scope, n)
}

type pgRevisions struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgRevisions) LoadPending() (*domain.PendingRevision, error) {
	return p.nar.PendingRevisionLoad(context.Background(), p.scope)
}
func (p pgRevisions) SavePending(pending domain.PendingRevision) error {
	return p.nar.PendingRevisionSave(context.Background(), p.scope, pending)
}
func (p pgRevisions) ClearPending() error {
	return p.nar.PendingRevisionClear(context.Background(), p.scope)
}

type pgReviews struct {
	nar   *postgres.Narrative
	scope postgres.ScopeRef
}

func (p pgReviews) SaveReview(r domain.ReviewEntry) error {
	return p.nar.SaveReview(context.Background(), p.scope, r)
}
func (p pgReviews) LoadReview(chapter int, scope store.ReviewScope) (*domain.ReviewEntry, error) {
	return p.nar.LoadReview(context.Background(), p.scope, chapter, scope)
}
func (p pgReviews) LoadReviewRounds(chapter int, scope store.ReviewScope) ([]domain.ReviewEntry, error) {
	return p.nar.LoadReviewRounds(context.Background(), p.scope, chapter, scope)
}
func (p pgReviews) SaveReviewDecision(itemID string, d store.ReviewDecision) error {
	return p.nar.SaveReviewDecision(context.Background(), p.scope, itemID, d)
}
func (p pgReviews) LoadReviewDecisions() (map[string]store.ReviewDecision, error) {
	return p.nar.LoadReviewDecisions(context.Background(), p.scope)
}
func (p pgReviews) DeleteReviewDecision(itemID string) (bool, error) {
	return p.nar.DeleteReviewDecision(context.Background(), p.scope, itemID)
}

func pgHarness(f pgFixture) narrativetest.Harness {
	cps := &pgCheckpoints{nar: f.nar, scope: f.scope}
	return narrativetest.Harness{
		Book:        pgBook{nar: f.nar, scope: f.scope},
		Progress:    pgProgress{nar: f.nar, scope: f.scope},
		Outline:     pgOutline{nar: f.nar, scope: f.scope},
		Drafts:      pgDrafts{nar: f.nar, scope: f.scope},
		Records:     pgRecords{nar: f.nar, scope: f.scope},
		Characters:  pgChars{nar: f.nar, scope: f.scope},
		Checkpoints: cps,
		Decisions:   pgDecisions{nar: f.nar, scope: f.scope},
		Revisions:   pgRevisions{nar: f.nar, scope: f.scope},
		Reviews:     pgReviews{nar: f.nar, scope: f.scope},
		Comments:    f.nar.CommentsStore(f.scope),
	}
}

// TestNarrativeContractPostgresBackend chạy đúng contract suite của file
// backend trên Postgres — parity gate của phase 05 cho miền truyện.
func TestNarrativeContractPostgresBackend(t *testing.T) {
	f := provisionNarrative(t, "project-a", "truyen-a")
	narrativetest.Run(t, pgHarness(f))
}
