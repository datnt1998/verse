package editor

import (
	"context"
	"time"
)

// Resource points to immutable published bytes. Revision is the sha256 wire
// digest; Version is the database-generated, monotonically increasing counter.
// Version zero identifies an unedited phase-05 current-build artifact.
// PreviousPath is a CAS-only hint for adopting a legacy canonical binding before
// its first pointer exists. It is never persisted or returned by Get/List.
type Resource struct {
	Key          string
	Revision     string
	Path         string
	Version      int64
	UpdatedAt    time.Time
	PreviousPath string
}

// Repository mutations participate in the media publication transaction carried
// by ctx. Empty expected means create only; nonempty expected is a sha256 wire
// revision, not Version. Get returns blobstore.ErrNotFound for a missing/deleted
// resource, and may synthesize Version zero from the current phase-05 build.
// List enumerates pointers, whereas Paths discovers all published blob bindings.
// Prefixes are literal (not SQL LIKE patterns).
type Repository interface {
	Get(ctx context.Context, key string) (Resource, error)
	List(ctx context.Context, prefix string) ([]Resource, error)
	CAS(ctx context.Context, key, expected string, next Resource) error
	Delete(ctx context.Context, key, expected string) error
	Paths(ctx context.Context, prefix string) ([]string, error)
}

// ResourcePathResolver translates a phase-05 canonical editor path to its exact
// current build artifact. Deleted aliases must resolve as blobstore.ErrNotFound.
type ResourcePathResolver interface {
	ResolvePath(ctx context.Context, logical string) (string, error)
}
