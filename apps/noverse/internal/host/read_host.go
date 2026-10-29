package host

import (
	"github.com/datnt1998/noverse/internal/bootstrap"
	storepkg "github.com/datnt1998/noverse/internal/store"
)

// ModelReady distinguishes a live engine from a disk-backed reading host. A
// reading host deliberately has no ChatModel, runner, or engine to invoke.
func (h *Host) ModelReady() bool { return h.models != nil }

// newReadHost builds a reading host over the file backend at cfg.OutputDir.
func newReadHost(cfg bootstrap.Config, lease *bookLease) *Host {
	return newReadHostWithStore(cfg, storepkg.NewStore(cfg.OutputDir), lease)
}

// newReadHostWithStore builds a reading host over an already-scoped store.
// Hosted runs and tests pass their injected store here, so no directory is
// derived from cfg.OutputDir.
func newReadHostWithStore(cfg bootstrap.Config, st *storepkg.Store, lease StoryLease) *Host {
	usage := NewUsageTracker(nil, st)
	usage.LoadFromStore()
	h := &Host{cfg: cfg, store: st, bookLease: lease, usage: usage, readOnly: true, lifecycle: lifecycleIdle, events: make(chan Event, 100), streamCh: make(chan string, 4096), done: make(chan struct{}, 1)}
	h.observer = newObserver(st, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	return h
}
