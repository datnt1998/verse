// Package editor stores the schema-v15 project JSON consumed by the copied
// yt-factory editor engine. Unknown project/timeline fields are preserved:
// adding the full engine later does not migrate or rewrite these projects.
package editor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 15

var ErrConflict = errors.New("project revision conflict")
var portableID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Object = map[string]any
type Project map[string]any

func (p Project) Metadata() Object { m, _ := p["metadata"].(map[string]any); return m }
func (p Project) Timeline() Object { m, _ := p["timeline"].(map[string]any); return m }
func Parse(data []byte) (Project, error) {
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("project must be a JSON object")
	}
	return p, nil
}
func Clone(p Project) Project { b, _ := json.Marshal(p); copy, _ := Parse(b); return copy }
func Revision(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}
func Marshal(p Project) ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	return append(b, '\n'), err
}
func New(id, name string, width, height int, fps float64, now time.Time) (Project, error) {
	name = strings.TrimSpace(name)
	if !portableID.MatchString(id) || name == "" || width <= 0 || height <= 0 || fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
		return nil, fmt.Errorf("invalid project id/name/resolution")
	}
	return Project{"id": id, "name": name, "description": "", "createdAt": now.UnixMilli(), "updatedAt": now.UnixMilli(), "duration": 0, "schemaVersion": SchemaVersion, "metadata": Object{"width": width, "height": height, "fps": fps}}, nil
}

type Media interface {
	Acquire(context.Context, string) (*media.Handle, error)
	Publish(context.Context, []media.Artifact, func(context.Context) error) error
}
type Store struct {
	Media      Media
	Repository Repository
	Templates  TemplateProjects
	StagingDir string
	Now        func() time.Time
}

func Path(id string) (string, error) {
	if !portableID.MatchString(id) {
		return "", fmt.Errorf("invalid project id")
	}
	return "editor/projects/" + id + "/project.json", nil
}
func (s Store) Read(ctx context.Context, id string) (Project, string, error) {
	path, err := Path(id)
	if err != nil {
		return nil, "", err
	}
	if s.Templates != nil {
		project, revision, err := s.Templates.ReadEditorProject(ctx, id)
		if err == nil {
			return project, revision, nil
		}
		if !errors.Is(err, blobstore.ErrNotFound) {
			return nil, "", err
		}
	}
	return s.readWorkspace(ctx, id, path)
}

func (s Store) readWorkspace(ctx context.Context, id, path string) (Project, string, error) {
	if s.Repository != nil {
		resource, err := s.Repository.Get(ctx, "projects/"+id)
		if err == nil {
			path = resource.Path
		} else if !errors.Is(err, blobstore.ErrNotFound) {
			return nil, "", err
		}
	}
	if resolver, ok := s.Repository.(ResourcePathResolver); ok {
		resolved, err := resolver.ResolvePath(ctx, path)
		if err != nil {
			return nil, "", err
		}
		path = resolved
	}
	h, err := s.Media.Acquire(ctx, path)
	if err != nil {
		return nil, "", err
	}
	defer h.Release()
	b, err := os.ReadFile(h.Path)
	if err != nil {
		return nil, "", err
	}
	p, err := Parse(b)
	return p, Revision(b), err
}

// Stage preserves the canonical project shape and uses same-directory rename.
// Only Publish makes staged bytes visible, under the media publication lock.
func (s Store) Stage(p Project) (media.Artifact, string, error) {
	id, _ := p["id"].(string)
	path, err := Path(id)
	if err != nil {
		return media.Artifact{}, "", err
	}
	b, err := Marshal(p)
	if err != nil {
		return media.Artifact{}, "", err
	}
	file := filepath.Join(s.StagingDir, filepath.FromSlash(path))
	if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return media.Artifact{}, "", err
	}
	if err = os.WriteFile(file+".part", b, 0644); err != nil {
		return media.Artifact{}, "", err
	}
	if err = os.Rename(file+".part", file); err != nil {
		return media.Artifact{}, "", err
	}
	return media.Artifact{Path: path, File: file, ContentType: "application/json"}, Revision(b), nil
}

// The same project lock protects Store mutations and the Service wrappers.
// Database CAS additionally fences writers in other serve/CLI processes.
var projectLocks sync.Map

func projectLock(id string) *sync.Mutex {
	value, _ := projectLocks.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}
func (s Store) Create(ctx context.Context, p Project) (string, error) {
	id, _ := p["id"].(string)
	if _, err := Path(id); err != nil {
		return "", err
	}
	if s.Repository == nil {
		return "", errors.New("editor project repository is required")
	}
	lock := projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	if _, _, err := s.Read(ctx, id); err == nil {
		return "", ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, blobstore.ErrNotFound) {
		return "", err
	}
	return s.publishProject(ctx, id, p, "", nil)
}

func (s Store) Write(ctx context.Context, id string, p Project, expected string) (string, error) {
	return s.write(ctx, id, p, expected, nil)
}

func (s Store) write(ctx context.Context, id string, p Project, expected string, retain func(context.Context) ([]Resource, error)) (string, error) {
	path, err := Path(id)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", errors.New("project must be a JSON object")
	}
	lock := projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	if expected == "" {
		return "", ErrConflict
	}
	if s.Templates != nil {
		if _, _, err := s.Templates.ReadEditorProject(ctx, id); err == nil {
			return s.Templates.WriteEditorProject(ctx, id, p, expected)
		} else if !errors.Is(err, blobstore.ErrNotFound) {
			return "", err
		}
	}
	if s.Repository == nil {
		return "", errors.New("editor project repository is required")
	}
	current, revision, err := s.readWorkspace(ctx, id, path)
	if err != nil {
		return "", err
	}
	if revision != expected {
		return "", ErrConflict
	}
	// Only these top-level fields change. Preserve caller-owned nested data and
	// let Marshal reject non-JSON values rather than silently dropping them.
	next := make(Project, len(p)+3)
	for key, value := range p {
		next[key] = value
	}
	next["id"] = id
	next["createdAt"] = current["createdAt"]
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	updatedAt := now().UnixMilli()
	if previous, ok := current["updatedAt"].(float64); ok && previous >= float64(updatedAt) {
		if previous >= float64(math.MaxInt64) {
			return "", errors.New("stored project updatedAt is out of range")
		}
		updatedAt = int64(previous) + 1
	}
	next["updatedAt"] = updatedAt
	var resources []Resource
	if retain != nil {
		resources, err = retain(ctx)
		if err != nil {
			return "", err
		}
	}
	return s.publishProject(ctx, id, next, expected, resources)
}

// Stage deliberately retains the phase-05 build path. Interactive writes use a
// unique path instead: media publication never overwrites local/S3 objects.
func (s Store) publishProject(ctx context.Context, id string, p Project, expected string, resources []Resource) (string, error) {
	data, err := Marshal(p)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(s.StagingDir, 0755); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(s.StagingDir, "project-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	path := "editor/projects/" + id + "/revisions/" + rand.Text() + ".json"
	revision := Revision(data)
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	next := Resource{Key: "projects/" + id, Path: path, Revision: revision, UpdatedAt: now()}
	var prior Resource
	if s.Repository != nil {
		prior, err = s.Repository.Get(ctx, next.Key)
		if err != nil && !errors.Is(err, blobstore.ErrNotFound) {
			return "", err
		}
	}
	artifact := media.Artifact{Path: path, File: file.Name(), ContentType: "application/json"}
	if err = s.Media.Publish(ctx, []media.Artifact{artifact}, func(txctx context.Context) error {
		for _, resource := range resources {
			if err := s.Repository.CAS(txctx, resource.Key, resource.Revision, resource); err != nil {
				return err
			}
		}
		return s.Repository.CAS(txctx, next.Key, expected, next)
	}); err != nil {
		return "", err
	}
	if err == nil {
		// A released pointer-owned local file has no grace collector; remove it
		// after commit so startup reconciliation never sees unbound strays.
		if prior.Path != "" && prior.Path != path {
			discardReleasedLocal(ctx, s.Media, prior.Path)
		}
	}
	return revision, nil
}
