// Package editor stores the schema-v15 project JSON consumed by the copied
// yt-factory editor engine. Unknown project/timeline fields are preserved:
// adding the full engine later does not migrate or rewrite these projects.
package editor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 15

var ErrConflict = errors.New("project revision conflict")
var portableID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
	if !portableID.MatchString(id) || name == "" || width <= 0 || height <= 0 || fps <= 0 {
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

// The serve process owns the exclusive pipeline lease. Project locks protect
// its editor mutations and worker fills from each other, as workspace.withLock
// does for the canonical server; media publication remains transactional.
var projectLocks sync.Map

func projectLock(id string) *sync.Mutex {
	value, _ := projectLocks.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}
func (s Store) Create(ctx context.Context, p Project) (string, error) {
	id, _ := p["id"].(string)
	lock := projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	if _, _, err := s.Read(ctx, id); err == nil {
		return "", ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, blobstore.ErrNotFound) {
		return "", err
	}
	artifact, rev, err := s.Stage(p)
	if err != nil {
		return "", err
	}
	return rev, s.Media.Publish(ctx, []media.Artifact{artifact}, nil)
}
func (s Store) Write(ctx context.Context, id string, p Project, expected string) (string, error) {
	lock := projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	if expected == "" {
		return "", ErrConflict
	}
	current, rev, err := s.Read(ctx, id)
	if err != nil {
		return "", err
	}
	if rev != expected {
		return "", ErrConflict
	}
	next := Clone(p)
	next["id"] = id
	next["createdAt"] = current["createdAt"]
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	next["updatedAt"] = now().UnixMilli()
	artifact, newRev, err := s.Stage(next)
	if err != nil {
		return "", err
	}
	return newRev, s.Media.Publish(ctx, []media.Artifact{artifact}, nil)
}
