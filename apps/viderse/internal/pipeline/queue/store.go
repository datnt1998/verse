package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the database session used for queue transactions and advisory locks.
// Pools, transactions and acquired connections all implement this interface.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Store is the persistence port consumed by queue planning and admission.
// With binds operations to the caller's transaction or dedicated connection.
type Store interface {
	With(DBTX) Store
	TryAdvisoryLock(context.Context, int64) (bool, error)
	UnlockAdvisory(context.Context, int64) error
	GetQueueItem(context.Context, string) (*domain.CloneQueueItem, error)
	InsertQueueItem(context.Context, domain.CloneQueueItem) error
	ListQueuedItems(context.Context) ([]domain.CloneQueueItem, error)
	ListLaneItems(context.Context, string) ([]domain.CloneQueueItem, error)
	RemoveQueueItem(context.Context, string, time.Time) (bool, error)
	RestoreQueueItem(context.Context, string) (bool, error)
	PromotePick(context.Context, string, float64) error
	UpdateQueuePosition(context.Context, string, float64) error
	LockOwnerChannel(context.Context, string) error
	PruneQueueHistory(context.Context, time.Time) error
	Owners(context.Context) ([]domain.Channel, error)
	Owner(context.Context, string) (*domain.Channel, error)
	Templates(context.Context, string) ([]domain.ChannelTemplate, error)
	TemplateRecipe(context.Context, string) (json.RawMessage, error)
	LaneLastStarts(context.Context) (map[string]LaneLastStart, error)
	PaceUsedToday(context.Context, time.Time) (map[string]int64, error)
	CountActiveJobs(context.Context) (int, error)
	ListActiveJobs(context.Context) ([]domain.CloneJob, error)
	ListJobSteps(context.Context, string) ([]domain.CloneStepRecord, error)
	ListRecentStepDurations(context.Context) ([]StepDuration, error)
	FindJobsBySourceKeys(context.Context, []string) ([]domain.CloneJob, error)
	FindQueueRowsBySourceKeys(context.Context, []string) ([]domain.CloneQueueItem, error)
	ListVideosByIDs(context.Context, []string) (map[string]RivalVideoView, error)
	ChannelInfoOf(context.Context, []string) (map[string]struct {
		Title       string
		MedianViews *int64
	}, error)
	ListSourcesWithChannels(context.Context, string) ([]ChannelSourceRow, error)
	SetSourceBaseline(context.Context, string, string, time.Time) error
	LastAutoQueuedChannel(context.Context, string) (string, error)
	ListEligibleVideos(context.Context, string, time.Time, string) ([]string, error)
	ListDoneTodayJobs(context.Context, time.Time) ([]DoneTodayJob, error)
	ListJobStageCounts(context.Context) (map[string]map[string]int64, error)
	ListQueueCountsByChannel(context.Context) (map[string]int64, error)
	ListDoneTodayCounts(context.Context, time.Time) (map[string]int64, error)
	CountChargedImagesToday(context.Context, time.Time) (map[string]int64, error)
}

// LaneLastStart identifies the last started turn in a channel's batch lane.
type LaneLastStart struct {
	LastStart    *time.Time
	LastSequence *time.Time // Creation time of the newest job in that turn.
}

// StepDuration is a successful step sample used to calculate queue ETAs.
type StepDuration struct {
	Step          string
	Elapsed       time.Duration
	SourceSeconds float64
	SceneCount    int
	FinishedAt    time.Time
}

// RivalVideoView supplies source metadata for planning and enqueueing.
type RivalVideoView struct {
	ID            string
	ChannelID     string
	Title         string
	DurationSec   *int32
	ViewCount     *int64
	PublishedAt   *time.Time
	HasCaptions   *bool
	Position      int64
	Origin        string
	FirstSeenAt   time.Time
	ChannelTitle  string
	ChannelMedian *int64
}

// ChannelSourceRow is a linked rival's auto-pick baseline and scan state.
type ChannelSourceRow struct {
	RivalID        string
	LinkedAt       time.Time
	AutoBaselineAt *time.Time
	RivalScannedAt *time.Time
}

// DoneTodayJob supplies the completed-job portion of the queue read model.
type DoneTodayJob struct {
	JobID      string
	ChannelID  string
	Title      string
	StageGroup string
	UpdatedAt  time.Time
}
