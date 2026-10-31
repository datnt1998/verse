package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// GuardSourceCleanup locks the job in the publication transaction and rejects
// a changed version, active worker lease, resumed or superseded job.
type CleanupStore interface {
	ListFinishedJobs(context.Context, int) ([]domain.CloneJob, error)
	ListSteps(context.Context, string) ([]domain.CloneStepRecord, error)
	ListArtifacts(context.Context, string) ([]domain.CloneArtifact, error)
	GuardSourceCleanup(context.Context, string, int64) error
	RemoveArtifactsByPath(context.Context, string, []string) error
}
type CleanupMedia interface {
	Acquire(context.Context, string) (*media.Handle, error)
	Unbind(context.Context, []string, func(context.Context) error) error
}
type SourceCleanupItem struct {
	JobID        string `json:"jobId"`
	Title        string `json:"title"`
	Bytes        int64  `json:"bytes"`
	MediaRemoved bool   `json:"mediaRemoved"`
}
type SourceCleanupReport struct {
	DryRun bool                `json:"dryRun"`
	Items  []SourceCleanupItem `json:"items"`
	// Logical source bytes unbound, not immediately reclaimed physical space:
	// other CAS owners and GC grace remain authoritative.
	FreedBytes int64    `json:"freedBytes"`
	Errors     []string `json:"errors"`
}

// SourceCleaner ports source-cleanup.ts to canonical schema15 projects. The
// download is one CAS binding, not a separate artifact and APFS library copy.
// Every playback media record must resolve independently before removing it.
type SourceCleaner struct {
	Store CleanupStore
	Media CleanupMedia
}

func (c *SourceCleaner) Reclaim(ctx context.Context, dryRun bool) (SourceCleanupReport, error) {
	r := SourceCleanupReport{DryRun: dryRun, Items: []SourceCleanupItem{}, Errors: []string{}}
	jobs, e := c.Store.ListFinishedJobs(ctx, 0)
	if e != nil {
		return r, e
	}
	for _, j := range jobs {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if j.Status != domain.CloneSucceeded || j.SupersededAt != nil {
			continue
		}
		item, e := c.reclaimOne(ctx, j, dryRun)
		if e != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: %v", j.ID, e))
			continue
		}
		if item != nil {
			r.Items = append(r.Items, *item)
			r.FreedBytes += item.Bytes
		}
	}
	return r, nil
}
func (c *SourceCleaner) reclaimOne(ctx context.Context, j domain.CloneJob, dry bool) (*SourceCleanupItem, error) {
	steps, e := c.Store.ListSteps(ctx, j.ID)
	if e != nil {
		return nil, e
	}
	var build *domain.CloneStepOutput
	exported := false
	for _, s := range steps {
		if s.Step == domain.CloneBuild && s.Status == domain.CloneSucceeded {
			build = s.Output
		}
		if s.Step == domain.CloneExport && s.Status == domain.CloneSucceeded {
			exported = true
		}
	}
	if build == nil || !exported || build.Data["sourceIndependent"] != true {
		return nil, nil
	}
	arts, e := c.Store.ListArtifacts(ctx, j.ID)
	if e != nil {
		return nil, e
	}
	var source *domain.CloneArtifact
	for i := range arts {
		if arts[i].Step == domain.CloneDownload && (arts[i].Path == "download/source.mp4" || strings.HasSuffix(arts[i].Path, "/download/source.mp4")) {
			source = &arts[i]
			break
		}
	}
	if source == nil {
		return nil, nil
	}
	safe, e := c.independent(ctx, build, arts, *source)
	if e != nil || !safe {
		return nil, e
	}
	h, e := c.Media.Acquire(ctx, source.Path)
	if e != nil {
		return nil, nil
	}
	stat, e := os.Stat(h.Path)
	h.Release()
	if e != nil {
		return nil, e
	}
	if !stat.Mode().IsRegular() {
		return nil, nil
	}
	title := j.Source.Title
	if title == "" {
		title = j.ID
	}
	item := &SourceCleanupItem{JobID: j.ID, Title: title, Bytes: stat.Size(), MediaRemoved: true}
	if !dry {
		paths := []string{source.Path}
		e = c.Media.Unbind(ctx, paths, func(tx context.Context) error {
			if e := c.Store.GuardSourceCleanup(tx, j.ID, j.Version); e != nil {
				return e
			}
			// The publication lock serializes editor saves. Recheck canonical playback
			// evidence in this transaction so an owner edit cannot race the deletion.
			safe, e := c.independent(tx, build, arts, *source)
			if e != nil {
				return e
			}
			if !safe {
				return fmt.Errorf("project đã đổi, giữ lại video nguồn")
			}
			return c.Store.RemoveArtifactsByPath(tx, j.ID, paths)
		})
		if e != nil {
			return nil, e
		}
	}
	return item, nil
}
func (c *SourceCleaner) readJSON(ctx context.Context, p string) (map[string]any, bool, error) {
	h, e := c.Media.Acquire(ctx, p)
	if e != nil {
		return nil, false, nil
	}
	raw, e := os.ReadFile(h.Path)
	h.Release()
	if e != nil {
		return nil, false, e
	}
	var v map[string]any
	if e = json.Unmarshal(raw, &v); e != nil {
		return nil, false, e
	}
	return v, true, nil
}
func (c *SourceCleaner) independent(ctx context.Context, build *domain.CloneStepOutput, arts []domain.CloneArtifact, source domain.CloneArtifact) (bool, error) {
	projectPath, _ := build.Data["projectPath"].(string)
	if projectPath == "" {
		return false, nil
	}
	project, ok, e := c.readJSON(ctx, resolveArtifactPath(arts, projectPath))
	if e != nil || !ok {
		return false, e
	}
	if project["schemaVersion"] != float64(15) || referencesSource(project, source.Path) {
		return false, nil
	}
	checkpoint, ok, e := c.readJSON(ctx, resolveArtifactPath(arts, "build/checkpoint.json"))
	if e != nil || !ok {
		return false, e
	}
	if checkpoint["version"] != float64(1) {
		return false, nil
	}
	audioSHA, _ := checkpoint["audioSha256"].(string)
	audioPath, _ := build.Data["audioPath"].(string)
	if audioPath == "" {
		return false, nil
	}
	if ok, e := c.playbackReady(ctx, resolveArtifactPath(arts, audioPath), audioSHA); e != nil || !ok {
		return false, e
	}
	ids := map[string]bool{}
	collectPlaybackIDs(project, ids)
	if len(ids) == 0 {
		return false, nil
	}
	for id := range ids {
		if id == "" || path.Base(id) != id || id == "." || id == ".." {
			return false, nil
		}
		metadataPath := "editor/media/" + id + "/metadata.json"
		meta, ok, e := c.readJSON(ctx, resolveArtifactPath(arts, metadataPath))
		if e != nil || !ok {
			return false, e
		}
		if referencesSource(meta, source.Path) {
			return false, nil
		}
		file, _ := meta["sourceFile"].(string)
		hash, _ := meta["contentHash"].(string)
		hash = strings.TrimPrefix(hash, "sha256:")
		if file == "" || path.Base(file) != file || file == "." || file == ".." || hash == source.SHA256 {
			return false, nil
		}
		if ok, e := c.playbackReady(ctx, resolveArtifactPath(arts, "editor/media/"+id+"/"+file), hash); e != nil || !ok {
			return false, e
		}
	}
	return true, nil
}

// Traverse nested compositions as well as root timeline.items. mediaItems is
// the project library, not playback; an unused library entry is not a source
// dependency (the importer never binds download/source.mp4 as an editor ID).
func collectPlaybackIDs(v any, ids map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["mediaId"].(string); ok {
			ids[id] = true
		}
		for k, v := range x {
			if k != "mediaItems" {
				collectPlaybackIDs(v, ids)
			}
		}
	case []any:
		for _, v := range x {
			collectPlaybackIDs(v, ids)
		}
	}
}
func resolveArtifactPath(arts []domain.CloneArtifact, logical string) string {
	for _, a := range arts {
		if a.Path == logical || strings.HasSuffix(a.Path, "/"+logical) {
			return a.Path
		}
	}
	return logical
}
func referencesSource(v any, source string) bool {
	switch x := v.(type) {
	case string:
		return x == source || x == "download/source.mp4" || strings.Contains(x, "/download/source.mp4")
	case map[string]any:
		for _, v := range x {
			if referencesSource(v, source) {
				return true
			}
		}
	case []any:
		for _, v := range x {
			if referencesSource(v, source) {
				return true
			}
		}
	}
	return false
}
func (c *SourceCleaner) playbackReady(ctx context.Context, p, sha string) (bool, error) {
	if p == "" || len(sha) != 64 {
		return false, nil
	}
	h, e := c.Media.Acquire(ctx, p)
	if e != nil {
		return false, nil
	}
	defer h.Release()
	if h.Ref.SHA256 != "" {
		return h.Ref.SHA256 == sha, nil
	}
	f, e := os.Open(h.Path)
	if e != nil {
		return false, e
	}
	defer f.Close()
	sum := sha256.New()
	if _, e = io.Copy(sum, f); e != nil {
		return false, e
	}
	return hex.EncodeToString(sum.Sum(nil)) == sha, nil
}
