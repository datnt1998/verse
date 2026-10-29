package store

import (
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/comments"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// BackendKind names a concrete storage backend. Selection is per process/run
// via STORE_DRIVER; there is no fallback from postgres to file.
type BackendKind string

const (
	BackendFile     BackendKind = "file"
	BackendPostgres BackendKind = "postgres"
)

// Substores is the complete set of domain substores a backend must provide.
// A backend that leaves any field nil is rejected by NewFromBackend — startup
// must fail loudly on an incomplete backend rather than panic mid-run.
type Substores struct {
	Progress       ProgressStore
	Book           BookStore
	Outline        OutlineStore
	Drafts         DraftStore
	Summaries      SummaryStore
	RunMeta        RunMetaStore
	UserRules      UserRulesStore
	Signals        SignalStore
	Runtime        RuntimeStore
	Characters     CharacterStore
	Cast           CastStore
	World          WorldStore
	Checkpoints    CheckpointStore
	Sessions       SessionStore
	Usage          UsageStore
	Simulation     SimulationStore
	Decisions      DecisionStore
	ChapterRecords ChapterRecordStore
	Revisions      RevisionStore
}

// requiredDomains lists every domain the facade must be able to serve;
// kept in one place so the capability gate and Substores stay in sync.
var requiredDomains = []struct {
	name    string
	present func(Substores) bool
}{
	{"progress", func(d Substores) bool { return d.Progress != nil }},
	{"book", func(d Substores) bool { return d.Book != nil }},
	{"outline", func(d Substores) bool { return d.Outline != nil }},
	{"drafts", func(d Substores) bool { return d.Drafts != nil }},
	{"summaries", func(d Substores) bool { return d.Summaries != nil }},
	{"run_meta", func(d Substores) bool { return d.RunMeta != nil }},
	{"user_rules", func(d Substores) bool { return d.UserRules != nil }},
	{"signals", func(d Substores) bool { return d.Signals != nil }},
	{"runtime", func(d Substores) bool { return d.Runtime != nil }},
	{"characters", func(d Substores) bool { return d.Characters != nil }},
	{"cast", func(d Substores) bool { return d.Cast != nil }},
	{"world", func(d Substores) bool { return d.World != nil }},
	{"checkpoints", func(d Substores) bool { return d.Checkpoints != nil }},
	{"sessions", func(d Substores) bool { return d.Sessions != nil }},
	{"usage", func(d Substores) bool { return d.Usage != nil }},
	{"simulation", func(d Substores) bool { return d.Simulation != nil }},
	{"decisions", func(d Substores) bool { return d.Decisions != nil }},
	{"chapter_records", func(d Substores) bool { return d.ChapterRecords != nil }},
	{"revisions", func(d Substores) bool { return d.Revisions != nil }},
}

// missingDomains lists the domain names a backend cannot supply.
func (d Substores) missingDomains() []string {
	var missing []string
	for _, dom := range requiredDomains {
		if !dom.present(d) {
			missing = append(missing, dom.name)
		}
	}
	return missing
}

// CapabilityError reports that a backend cannot perform an operation or
// supply a domain. It is a startup/contract failure, never a runtime retry
// hint: callers must not stub around it.
type CapabilityError struct {
	Backend BackendKind
	Op      string
	Domains []string
}

func (e *CapabilityError) Error() string {
	switch {
	case len(e.Domains) > 0:
		return fmt.Sprintf("backend %s lacks domains %v: %v", e.Backend, e.Domains, errs.ErrStorageUnsupported)
	case e.Op != "":
		return fmt.Sprintf("backend %s does not implement %s: %v", e.Backend, e.Op, errs.ErrStorageUnsupported)
	default:
		return fmt.Sprintf("backend %s capability gap: %v", e.Backend, errs.ErrStorageUnsupported)
	}
}

func (e *CapabilityError) Unwrap() error { return errs.ErrStorageUnsupported }

// ReviewLister is the optional backend capability behind LatestReview: the
// newest review (chapter or global) for the story. File mode enumerates the
// review documents; postgres reads the review rows. A backend without it has
// no review surface, and the facade fails loudly instead of scanning a fake
// directory.
type ReviewLister interface {
	LatestReview() (*domain.ReviewEntry, error)
}

// ChapterMarkerLister is the optional backend capability behind the chapter
// list markers: which chapters have a plan document and which have a review.
// File mode lists the engine directories; postgres reads plan artifacts and
// review rows. No caller may fall back to os.ReadDir when this is absent.
type ChapterMarkerLister interface {
	PlanChapters() ([]int, error)
	ReviewChapters() ([]int, error)
}

// Backend owns the substores and the cross-domain compound operations for one
// storage implementation. File keeps the documented saga semantics; postgres
// executes the same operations inside real transactions.
//
// Backend is NOT a filesystem facade: Dir() returns the engine root for file
// and the empty string for backends without a filesystem root. Callers that
// need a directory must check file capability explicitly instead of writing
// into a fake path — phase 06 introduces the explicit scratch capability.
type Backend interface {
	// Kind reports the driver; used for diagnostics and capability gating.
	Kind() BackendKind
	// Dir is the engine root for file backends, "" otherwise.
	Dir() string
	// Substores returns the domain substores; all fields must be non-nil for
	// the backend to serve the facade.
	Substores() Substores
	// Docs is the engine-relative document surface used by callers that
	// persist a domain document by name (project ledger, review listings,
	// rule snapshots). File mode maps it to Dir; postgres maps it to typed
	// rows. It is never a path a caller may hand to os.Open.
	Docs() *IO
	// Comments returns the comment lifecycle store for this backend. File mode
	// binds <story>/output/novel; postgres binds the scoped comment rows.
	Comments() *comments.Store

	// Compound operations — coordination that file performs with sequential
	// writes and postgres performs in one transaction.
	LoadProjectFormatVersion() (int, error)
	SaveProjectFormatVersion(version int) error
	Init() error
	FoundationFingerprint() (string, error)
	ExpandArc(volumeIdx, arcIdx int, expansion domain.ArcExpansion) error
	AppendVolume(vol domain.VolumeOutline) error
	ReviseOutline(fromChapter int, replacement []domain.OutlineEntry) (int, error)
	ClearHandledSteer() error
	InvalidateChapterAggregates(fromChapter int) error

	// Close releases backend resources; file is a no-op.
	Close() error
}
