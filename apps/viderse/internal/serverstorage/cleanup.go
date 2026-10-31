package serverstorage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/storagegc"
	"github.com/datnt1998/verse/platform/blobstore"
)

type CleanupLegacy interface {
	ReclaimList(context.Context) ([]retention.ReclaimItem, error)
	Reclaim(context.Context, string) (retention.ReclaimItem, error)
	PreviewCleanup(context.Context) (*retention.Preview, error)
	RunCleanup(context.Context, string) (retention.RunResult, error)
}
type cleanupSnapshot struct {
	actor      string
	admin      bool
	store      string
	generation int64
	expires    time.Time
	objects    []blobstore.BlobInfo
	cache      []media.CacheCandidate
}
type Cleanup struct {
	legacy   CleanupLegacy
	manager  *Service
	mu       sync.Mutex
	previews map[string]cleanupSnapshot
}

func NewCleanup(legacy CleanupLegacy) *Cleanup {
	return &Cleanup{legacy: legacy, previews: make(map[string]cleanupSnapshot)}
}

// BindManager is composition-only, before requests or background cleanup start.
func (c *Cleanup) BindManager(manager *Service) { c.manager = manager }
func (c *Cleanup) remote(ctx context.Context) (bool, string, int64, error) {
	if c.manager == nil {
		return false, "", 0, nil
	}
	state, err := c.manager.opts.Repository.State(ctx)
	if err != nil {
		return false, "", 0, err
	}
	d, err := c.manager.opts.Repository.Descriptor(ctx, state.Active)
	if err != nil {
		return false, "", 0, err
	}
	return d.Config.Driver == "s3", state.Active, state.Generation, nil
}
func (c *Cleanup) collector() (storagegc.Service, error) {
	repo, ok := c.manager.opts.Repository.(storagegc.Repository)
	if !ok {
		return storagegc.Service{}, errors.New("thiếu bộ kiểm tra tham chiếu dọn kho")
	}
	return storagegc.Service{Repository: repo, Media: c.manager.Media(), Grace: 24 * time.Hour}, nil
}
func (c *Cleanup) PreviewCleanup(ctx context.Context) (*retention.Preview, error) {
	remote, store, generation, err := c.remote(ctx)
	if err != nil {
		return nil, err
	}
	if !remote {
		return c.legacy.PreviewCleanup(ctx)
	}
	viewer, ok := retention.Viewer(ctx)
	if !ok || viewer.Actor == "" {
		return nil, &Error{403, "Thiếu quyền dọn dữ liệu"}
	}
	gc, err := c.collector()
	if err != nil {
		return nil, err
	}
	objects, err := gc.Run(ctx, store, false)
	if err != nil {
		return nil, err
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	snapshot := cleanupSnapshot{actor: viewer.Actor, admin: viewer.Admin, store: store, generation: generation, expires: time.Now().Add(10 * time.Minute), objects: objects.Objects}
	out := &retention.Preview{ID: id, Items: []retention.PreviewItem{{Kind: "orphan_objects", Label: "Tệp trên kho của video đã xoá", Count: objects.Candidates, Bytes: objects.Bytes}}, Freed: objects.Bytes, Stopped: []string{}, Errors: []string{}}
	if viewer.Admin {
		snapshot.cache, err = c.manager.Media().Cache().PreviewTrim(ctx)
		if err != nil {
			return nil, err
		}
		var bytes int64
		for _, entry := range snapshot.cache {
			bytes += entry.Bytes()
		}
		out.Items = append(out.Items, retention.PreviewItem{Kind: "work_cache", Label: "Bộ đệm trên máy chủ", Count: len(snapshot.cache), Bytes: bytes})
		out.Freed += bytes
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, old := range c.previews {
		if now.After(old.expires) {
			delete(c.previews, key)
		}
	}
	if len(c.previews) >= 64 {
		return nil, &Error{429, "Có quá nhiều lượt xem trước; vui lòng thử lại sau"}
	}
	c.previews[id] = snapshot
	return out, nil
}
func (c *Cleanup) RunCleanup(ctx context.Context, id string) (retention.RunResult, error) {
	remote, store, generation, err := c.remote(ctx)
	if err != nil {
		return retention.RunResult{}, err
	}
	if !remote {
		return c.legacy.RunCleanup(ctx, id)
	}
	viewer, ok := retention.Viewer(ctx)
	if !ok || viewer.Actor == "" {
		return retention.RunResult{}, &Error{403, "Thiếu quyền dọn dữ liệu"}
	}
	c.mu.Lock()
	snapshot, exists := c.previews[id]
	if exists && snapshot.admin && !viewer.Admin {
		c.mu.Unlock()
		return retention.RunResult{}, &Error{403, "Chỉ quản trị viên được dọn bộ đệm trên máy chủ"}
	}
	if !exists || snapshot.actor != viewer.Actor || time.Now().After(snapshot.expires) || snapshot.store != store || snapshot.generation != generation {
		c.mu.Unlock()
		return retention.RunResult{}, conflict("Lượt xem trước đã thay đổi; hãy xem trước lại")
	}
	delete(c.previews, id)
	c.mu.Unlock()
	gc, err := c.collector()
	if err != nil {
		return retention.RunResult{}, err
	}
	result, err := gc.Apply(ctx, store, snapshot.objects)
	if err != nil {
		return retention.RunResult{}, err
	}
	out := retention.RunResult{Removed: result.Deleted, Freed: result.Bytes, Stopped: []string{}, Errors: []string{}, Kept: []string{}}
	if viewer.Admin {
		bytes, count, err := c.manager.Media().Cache().TrimPreview(ctx, snapshot.cache)
		if err != nil {
			return out, err
		}
		out.Removed += count
		out.Freed += bytes
	}
	c.manager.changed()
	return out, nil
}
func (c *Cleanup) ReclaimList(ctx context.Context) ([]retention.ReclaimItem, error) {
	remote, _, _, err := c.remote(ctx)
	if err != nil {
		return nil, err
	}
	if remote {
		return []retention.ReclaimItem{}, nil
	}
	return c.legacy.ReclaimList(ctx)
}
func (c *Cleanup) Reclaim(ctx context.Context, id string) (retention.ReclaimItem, error) {
	remote, _, _, err := c.remote(ctx)
	if err != nil {
		return retention.ReclaimItem{}, err
	}
	if remote {
		return retention.ReclaimItem{}, conflict("Phần dựng và xuất video chưa dùng kho máy chủ")
	}
	return c.legacy.Reclaim(ctx, id)
}

// Start owns the same lifetime as the manager; local cleanup remains V22,
// while S3 maintenance never invokes a filesystem graph scanner.
func (c *Cleanup) Start() {
	c.manager.wg.Add(1)
	go func() {
		defer c.manager.wg.Done()
		run := func() {
			remote, store, _, err := c.remote(c.manager.ctx)
			if err != nil {
				c.manager.logFailure("cleanup_mode", err)
				return
			}
			if !remote {
				if auto, ok := c.legacy.(interface{ RunAuto(context.Context) }); ok {
					auto.RunAuto(c.manager.ctx)
				}
				return
			}
			gc, err := c.collector()
			if err == nil {
				_, err = gc.Run(c.manager.ctx, store, true)
			}
			c.manager.logFailure("auto_gc", err)
			_, err = c.manager.Media().Cache().Trim(c.manager.ctx)
			c.manager.logFailure("auto_cache_trim", err)
		}
		run()
		ticker := time.NewTicker(retention.AutoEvery)
		defer ticker.Stop()
		for {
			select {
			case <-c.manager.ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
