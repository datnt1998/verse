// Package serverstorage implements V25's app-owned storage control plane.
package serverstorage

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/secretbox"
)

type S3Config struct {
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint,omitempty"`
	AccountID       string `json:"accountId,omitempty"`
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	ForcePathStyle  bool   `json:"forcePathStyle,omitempty"`
	AllowInsecure   bool   `json:"allowInsecure,omitempty"`
	Delivery        string `json:"delivery"`
	PresignEndpoint string `json:"presignEndpoint,omitempty"`
}
type UsageGroup struct {
	Group string `json:"group"`
	Files *int64 `json:"files"`
	Bytes *int64 `json:"bytes"`
}
type OpStat struct {
	Op     string  `json:"op"`
	Count  int64   `json:"count"`
	Errors int64   `json:"errors"`
	Bytes  int64   `json:"bytes"`
	AvgMS  float64 `json:"avgMs"`
	MaxMS  float64 `json:"maxMs"`
}
type Previous struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Path  string `json:"path,omitempty"`
	Files int64  `json:"files"`
	Bytes int64  `json:"bytes"`
}
type Disk struct {
	Path      string `json:"path"`
	FreeBytes uint64 `json:"freeBytes"`
}
type Storage struct {
	Kind            string              `json:"kind"`
	Provider        string              `json:"provider,omitempty"`
	EndpointHost    string              `json:"endpointHost,omitempty"`
	PresignEndpoint string              `json:"presignEndpoint,omitempty"`
	Delivery        string              `json:"delivery"`
	Region          string              `json:"region,omitempty"`
	Bucket          string              `json:"bucket,omitempty"`
	ForcePathStyle  *bool               `json:"forcePathStyle,omitempty"`
	AllowInsecure   *bool               `json:"allowInsecure,omitempty"`
	AccessKeyLast4  string              `json:"accessKeyLast4,omitempty"`
	Sources         map[string]string   `json:"sources"`
	EncryptionKey   secretbox.Readiness `json:"encryptionKey"`
	Online          bool                `json:"online"`
	CheckedAt       time.Time           `json:"checkedAt"`
	LastError       string              `json:"lastError,omitempty"`
	Disk            *Disk               `json:"disk,omitempty"`
	Previous        *Previous           `json:"previous,omitempty"`
}
type WorkCache struct {
	UsedBytes  int64 `json:"usedBytes"`
	LimitBytes int64 `json:"limitBytes"`
}
type Snapshot struct {
	Role      string       `json:"role"`
	Storage   *Storage     `json:"storage,omitempty"`
	Usage     []UsageGroup `json:"usage"`
	WorkCache *WorkCache   `json:"workCache,omitempty"`
	Ops       []OpStat     `json:"ops,omitempty"`
	Migration *Migration   `json:"migration,omitempty"`
}
type Step struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	MS     *int64 `json:"ms,omitempty"`
}
type TestError struct {
	Code string `json:"code"`
	Step string `json:"step"`
}
type TestResult struct {
	TestID   string     `json:"testId"`
	Steps    []Step     `json:"steps"`
	ProbeURL string     `json:"probeUrl,omitempty"`
	Error    *TestError `json:"error,omitempty"`
}
type MigrationNeed struct {
	Files  int64        `json:"files"`
	Bytes  int64        `json:"bytes"`
	Groups []UsageGroup `json:"groups"`
}
type SaveResult struct {
	NeedsMigration *MigrationNeed `json:"needsMigration"`
}
type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}
type StoreLabel struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}
type MigrationGroup struct {
	Group string   `json:"group"`
	Files Progress `json:"files"`
	Bytes Progress `json:"bytes"`
	State string   `json:"state"`
}
type MigrationError struct {
	Group  string `json:"group"`
	JobID  string `json:"jobId,omitempty"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}
type Migration struct {
	TaskID     string           `json:"taskId"`
	State      string           `json:"state"`
	From       StoreLabel       `json:"from"`
	To         StoreLabel       `json:"to"`
	Files      Progress         `json:"files"`
	Bytes      Progress         `json:"bytes"`
	Checked    int64            `json:"checked"`
	RateBPS    float64          `json:"rateBps"`
	StartedAt  time.Time        `json:"startedAt"`
	FinishedAt *time.Time       `json:"finishedAt,omitempty"`
	Groups     []MigrationGroup `json:"groups"`
	Errors     []MigrationError `json:"errors"`
}
type MigrationRecord struct {
	Migration            Migration
	FromID, ToID         string
	Revision, JournalSeq int64
}
type MigrationItem struct {
	Ref                     media.Ref
	Group, State, ErrorCode string
}
type Health struct {
	StoreID   string
	Online    bool
	CheckedAt time.Time
	Since     *time.Time
	ErrorCode string
}

// Repository persists control-plane state. Cutover must try (never wait for)
// the database publication fence and recheck the complete live root graph.
type Repository interface {
	media.Repository
	Usage(context.Context) ([]UsageGroup, error)
	PreviousTotals(context.Context, string) (int64, int64, error)
	LatestMigration(context.Context) (*MigrationRecord, error)
	CreateMigration(context.Context, string) (MigrationRecord, error)
	SetMigrationState(context.Context, string, string, string) error
	ReconcileMigration(context.Context, string) ([]MigrationItem, error)
	VerifyMigrationItem(context.Context, string, media.Ref, string, string) error
	SaveMigrationProgress(context.Context, MigrationRecord) error
	TryCutover(context.Context, string) (bool, error)
	SaveHealth(context.Context, Health) error
	Health(context.Context) (*Health, error)
	SealedCredentials(context.Context) ([][]byte, error)
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string      { return e.Message }
func conflict(message string) error { return &Error{409, message} }
