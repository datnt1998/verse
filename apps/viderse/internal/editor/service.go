package editor

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// WorkspaceMedia is the durable publication seam shared by local and S3 stores.
type WorkspaceMedia interface {
	Media
	Unbind(context.Context, []string, func(context.Context) error) error
}

// Service owns the editor workspace. Mutable JSON lives behind revision pointers;
// media source and rendered bytes are immutable artifacts.
type Service struct {
	Store                  Store
	Media                  WorkspaceMedia
	Repository             Repository
	StagingDir             string
	Now                    func() time.Time
	Prober                 MediaProber
	Runner                 CommandRunner
	FFmpeg, FFprobe, YTDLP string
	Gate                   ExportGate
	Anchors                func(context.Context, string) (Object, error)
	MediaReferences        func(context.Context) ([]string, error)
	ExportOwner            func(context.Context, string) (string, error)
	ProjectDelete          func(context.Context, string) error
	OverlayAssets          fs.FS
}

type Error struct {
	Code, Message string
	Extra         Object
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error {
	if e.Code == "revision_conflict" {
		return ErrConflict
	}
	return nil
}
func apiError(code, message string, extra ...Object) error {
	e := &Error{Code: code, Message: message}
	if len(extra) > 0 {
		e.Extra = extra[0]
	}
	return e
}
func CheckRevision(actual, expected string) error {
	if expected == "" {
		return apiError("invalid_request", "If-Match revision is required")
	}
	if actual != expected {
		return apiError("revision_conflict", "Resource was modified elsewhere", Object{"revision": actual})
	}
	return nil
}
func ValidateID(id string) error {
	if len(id) > 64 || len(id) == 0 || !portableID.MatchString(id) || id[0] == '_' || id[0] == '-' {
		return apiError("invalid_request", "Invalid portable id")
	}
	return nil
}
func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func SanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune("<>:\"\\|?*", r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimRight(strings.TrimSpace(name), ". ")
	if name == "" || name == "." || name == ".." {
		return "source.bin"
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if r := []rune(stem); len(r)+len([]rune(ext)) > 200 {
		available := 200 - len([]rune(ext))
		if available < 0 {
			available = 0
			ext = string([]rune(ext)[:200])
		}
		stem = string(r[:available])
	}
	return stem + ext
}

var workspaceLocks sync.Map

func resourceLock(key string) *sync.Mutex {
	if strings.HasPrefix(key, "projects/") && strings.Count(key, "/") == 1 {
		return projectLock(strings.TrimPrefix(key, "projects/"))
	}
	lock, _ := workspaceLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func (s *Service) repository() Repository {
	if s.Repository != nil {
		return s.Repository
	}
	return s.Store.Repository
}
func (s *Service) resolvePath(ctx context.Context, path string) (string, error) {
	if resolver, ok := s.repository().(interface {
		ResolvePath(context.Context, string) (string, error)
	}); ok {
		return resolver.ResolvePath(ctx, path)
	}
	return path, nil
}
func (s *Service) acquire(ctx context.Context, path string) (*media.Handle, error) {
	resolved, err := s.resolvePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if s.Media != nil {
		return s.Media.Acquire(ctx, resolved)
	}
	return s.Store.Media.Acquire(ctx, resolved)
}
func (s *Service) projectStore() Store {
	st := s.Store
	if st.Media == nil {
		st.Media = s.Media
	}
	if st.Repository == nil {
		st.Repository = s.Repository
	}
	if st.StagingDir == "" {
		st.StagingDir = s.StagingDir
	}
	if st.Now == nil {
		st.Now = s.Now
	}
	return st
}
func (s *Service) readJSON(ctx context.Context, key, legacyPath string) (Object, string, error) {
	path := legacyPath
	if repo := s.repository(); repo != nil {
		r, err := repo.Get(ctx, key)
		if err == nil {
			path = r.Path
		} else if !isMissing(err) {
			return nil, "", err
		}
	}
	if path == "" {
		return nil, "", blobstore.ErrNotFound
	}
	h, err := s.acquire(ctx, path)
	if err != nil {
		return nil, "", err
	}
	defer h.Release()
	b, err := os.ReadFile(h.Path)
	if err != nil {
		return nil, "", err
	}
	var value Object
	if err = json.Unmarshal(b, &value); err != nil {
		return nil, Revision(b), apiError("invalid_request", "Stored JSON is unreadable")
	}
	if value == nil {
		return nil, Revision(b), apiError("invalid_request", "Stored JSON must be an object")
	}
	return value, Revision(b), nil
}
func isMissing(err error) bool {
	return errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}
func (s *Service) stageFile(path string, body io.Reader, maxBytes int64) (media.Artifact, string, int64, error) {
	if err := media.ValidatePath(path); err != nil {
		return media.Artifact{}, "", 0, err
	}
	dir := s.StagingDir
	if dir == "" {
		dir = s.Store.StagingDir
	}
	if dir == "" {
		return media.Artifact{}, "", 0, errors.New("editor staging directory is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return media.Artifact{}, "", 0, err
	}
	file, err := os.CreateTemp(dir, "editor-*")
	if err != nil {
		return media.Artifact{}, "", 0, err
	}
	hash := sha256.New()
	source := body
	if maxBytes > 0 {
		source = io.LimitReader(body, maxBytes+1)
	}
	n, err := io.Copy(io.MultiWriter(file, hash), source)
	if err == nil && maxBytes > 0 && n > maxBytes {
		err = apiError("payload_too_large", uploadTooLarge("Upload exceeds the size limit", maxBytes))
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(file.Name())
		return media.Artifact{}, "", 0, err
	}
	return media.Artifact{Path: path, File: file.Name()}, "sha256:" + hex.EncodeToString(hash.Sum(nil)), n, nil
}
func (s *Service) stageJSON(path string, value any) (media.Artifact, string, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return media.Artifact{}, "", err
	}
	b = append(b, '\n')
	artifact, revision, _, err := s.stageFile(path, bytes.NewReader(b), int64(len(b)))
	artifact.ContentType = "application/json"
	return artifact, revision, err
}
func (s *Service) writeJSON(ctx context.Context, key, legacyPath string, value Object, expected string) (string, error) {
	repo := s.repository()
	if repo == nil {
		return "", errors.New("editor repository is required")
	}
	if expected == "empty" {
		expected = ""
	}
	artifact, revision, err := s.stageJSON("editor/revisions/"+ID()+".json", value)
	if err != nil {
		return "", err
	}
	defer os.Remove(artifact.File)
	publish := s.Store.Media
	if s.Media != nil {
		publish = s.Media
	}
	if publish == nil {
		return "", errors.New("editor media service is required")
	}
	prior, priorErr := repo.Get(ctx, key)
	if priorErr != nil && !isMissing(priorErr) {
		return "", priorErr
	}
	err = publish.Publish(ctx, []media.Artifact{artifact}, func(tx context.Context) error {
		return repo.CAS(tx, key, expected, Resource{Key: key, Path: artifact.Path, Revision: revision, UpdatedAt: s.now(), PreviousPath: legacyPath})
	})
	if err == nil {
		discardReleasedLocal(ctx, publish, prior.Path, legacyPath)
	}
	return revision, err
}
func (s *Service) listPaths(ctx context.Context, prefix string) ([]string, error) {
	if s.repository() == nil {
		return nil, errors.New("editor repository is required")
	}
	return s.repository().Paths(ctx, prefix)
}
func (s *Service) removeResource(ctx context.Context, key, legacyPath string) error {
	repo := s.repository()
	if repo == nil {
		return errors.New("editor repository is required")
	}
	record, err := repo.Get(ctx, key)
	if err != nil && !isMissing(err) {
		return err
	}
	if s.Media == nil {
		return errors.New("editor media service is required")
	}
	err = s.Media.Publish(ctx, nil, func(tx context.Context) error { return repo.Delete(tx, key, record.Revision) })
	if err == nil {
		discardReleasedLocal(ctx, s.Media, record.Path, legacyPath)
	}
	return err
}

type ProjectSummary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	UpdatedAt float64 `json:"updatedAt"`
	CreatedAt float64 `json:"createdAt,omitempty"`
}
type ProjectResolution struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FPS    float64 `json:"fps"`
}
type CreateProjectInput struct {
	Name       string             `json:"name"`
	Template   string             `json:"template,omitempty"`
	Resolution *ProjectResolution `json:"resolution,omitempty"`
}

func (s *Service) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	paths, err := s.listPaths(ctx, "editor/projects/")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, path := range paths {
		parts := strings.Split(path, "/")
		if len(parts) == 4 && parts[3] == "project.json" {
			ids[parts[2]] = true
		}
	}
	rows, err := s.repository().List(ctx, "projects/")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if strings.Count(row.Key, "/") == 1 {
			ids[strings.TrimPrefix(row.Key, "projects/")] = true
		}
	}
	result := []ProjectSummary{}
	for id := range ids {
		p, _, e := s.projectStore().Read(ctx, id)
		if isMissing(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		name, ok := p["name"].(string)
		if !ok {
			continue
		}
		result = append(result, ProjectSummary{ID: id, Name: name, UpdatedAt: fillNum(Object(p), "updatedAt", 0), CreatedAt: fillNum(Object(p), "createdAt", 0)})
	}
	if provider, ok := s.projectStore().Templates.(interface {
		ListEditorProjects(context.Context) ([]ProjectSummary, error)
	}); ok {
		templates, e := provider.ListEditorProjects(ctx)
		if e != nil {
			return nil, e
		}
		for _, template := range templates {
			if !ids[template.ID] {
				result = append(result, template)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt == result[j].UpdatedAt {
			return result[i].ID < result[j].ID
		}
		return result[i].UpdatedAt > result[j].UpdatedAt
	})
	return result, nil
}
func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (Project, string, error) {
	resolution := ProjectResolution{1920, 1080, 30}
	if input.Resolution != nil {
		resolution = *input.Resolution
	} else {
		switch input.Template {
		case "", "youtube-1080p", "youtube-1080p30":
		case "shorts-1080x1920":
			resolution = ProjectResolution{1080, 1920, 30}
		default:
			return nil, "", apiError("invalid_request", "Preset project không hợp lệ")
		}
	}
	p, err := New(ID(), input.Name, resolution.Width, resolution.Height, resolution.FPS, s.now())
	if err != nil {
		return nil, "", apiError("invalid_request", err.Error())
	}
	revision, err := s.projectStore().Create(ctx, p)
	return p, revision, err
}
func (s *Service) LoadProject(ctx context.Context, id string) (Project, string, error) {
	p, revision, err := s.projectStore().Read(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if err = s.refreshProjectSources(ctx, p); err != nil {
		return nil, "", err
	}
	return p, revision, nil
}
func (s *Service) SaveProject(ctx context.Context, id string, p Project, expected string) (string, error) {
	if expected == "" {
		return "", apiError("invalid_request", "If-Match revision is required")
	}
	return s.projectStore().Write(ctx, id, p, expected)
}
func (s *Service) RenameProject(ctx context.Context, id, name, expected string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", apiError("invalid_request", "Project name is required")
	}
	p, _, err := s.projectStore().Read(ctx, id)
	if err != nil {
		return "", err
	}
	p["name"] = strings.TrimSpace(name)
	return s.SaveProject(ctx, id, p, expected)
}
func (s *Service) DuplicateProject(ctx context.Context, id, name string) (Project, string, error) {
	p, _, err := s.projectStore().Read(ctx, id)
	if err != nil {
		return nil, "", err
	}
	p = Clone(p)
	newID := ID()
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprint(p["name"]) + " (copy)"
	}
	p["id"] = newID
	p["name"] = strings.TrimSpace(name)
	p["createdAt"] = s.now().UnixMilli()
	p["updatedAt"] = s.now().UnixMilli()
	revision, err := s.projectStore().Create(ctx, p)
	if err != nil {
		return nil, "", err
	}
	keys := []string{"links", "sidecars/animation-presets", "sidecars/render-queue"}
	for _, key := range keys {
		legacy := "editor/projects/" + id + "/" + key + ".json"
		if strings.HasPrefix(key, "sidecars/") {
			legacy = "editor/projects/" + id + "/" + strings.TrimPrefix(key, "sidecars/") + ".json"
		}
		value, _, e := s.readJSON(ctx, "projects/"+id+"/"+key, legacy)
		if isMissing(e) {
			continue
		}
		if e != nil {
			return nil, "", e
		}
		if _, e = s.writeJSON(ctx, "projects/"+newID+"/"+key, "", value, ""); e != nil {
			return nil, "", e
		}
	}
	return p, revision, nil
}
func (s *Service) DeleteProject(ctx context.Context, id string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	var deleteTemplate func(context.Context, string) error
	if provider := s.projectStore().Templates; provider != nil {
		if _, _, err := provider.ReadEditorProject(ctx, id); err == nil {
			deleter, ok := provider.(interface {
				DeleteEditorProject(context.Context, string) error
			})
			if !ok {
				return errors.New("template deletion provider is required")
			}
			deleteTemplate = deleter.DeleteEditorProject
		} else if !isMissing(err) {
			return err
		}
	}
	lock := projectLock(id)
	lock.Lock()
	_, revision, err := s.projectStore().Read(ctx, id)
	if err != nil {
		lock.Unlock()
		return err
	}
	ids, err := s.ProjectMediaIDs(ctx, id)
	if err != nil {
		lock.Unlock()
		return err
	}
	head, headErr := s.repository().Get(ctx, "projects/"+id)
	if headErr != nil && !isMissing(headErr) {
		lock.Unlock()
		return headErr
	}
	rows, err := s.repository().List(ctx, "projects/"+id+"/")
	if err != nil {
		lock.Unlock()
		return err
	}
	err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
		if deleteTemplate != nil {
			if e := deleteTemplate(tx, id); e != nil {
				return e
			}
		} else if s.ProjectDelete != nil {
			if e := s.ProjectDelete(tx, id); e != nil {
				return e
			}
		}
		for _, row := range rows {
			if e := s.repository().Delete(tx, row.Key, row.Revision); e != nil {
				return e
			}
		}
		if deleteTemplate != nil {
			return nil
		}
		return s.repository().Delete(tx, "projects/"+id, revision)
	})
	lock.Unlock()
	if err != nil {
		return err
	}
	discardReleasedLocal(ctx, s.Media, head.Path)
	for _, row := range rows {
		discardReleasedLocal(ctx, s.Media, row.Path)
	}
	exports, err := s.exportIDs(ctx, id)
	if err != nil {
		return err
	}
	for _, exportID := range exports {
		if err = s.DeleteExport(ctx, id, exportID); err != nil {
			return err
		}
	}
	_, err = s.ReclaimMediaOf(ctx, ids)
	return err
}
func sidecar(name string) (string, error) {
	switch name {
	case "animation-presets", "render-queue":
		return name + ".json", nil
	default:
		return "", apiError("invalid_request", "Unknown sidecar: "+name)
	}
}
func (s *Service) LoadSidecar(ctx context.Context, id, name string) (Object, string, error) {
	if _, _, err := s.projectStore().Read(ctx, id); err != nil {
		return nil, "", err
	}
	file, err := sidecar(name)
	if err != nil {
		return nil, "", err
	}
	value, rev, err := s.readJSON(ctx, "projects/"+id+"/sidecars/"+name, "editor/projects/"+id+"/"+file)
	if isMissing(err) {
		return Object{}, "empty", nil
	}
	return value, rev, err
}
func (s *Service) SaveSidecar(ctx context.Context, id, name string, value Object, expected string) (string, error) {
	file, err := sidecar(name)
	if err != nil {
		return "", err
	}
	lock := projectLock(id)
	lock.Lock()
	defer lock.Unlock()
	_, actual, err := s.LoadSidecar(ctx, id, name)
	if err != nil {
		return "", err
	}
	if err = CheckRevision(actual, expected); err != nil {
		return "", err
	}
	if value == nil {
		return "", apiError("invalid_request", "sidecar must be an object")
	}
	return s.writeJSON(ctx, "projects/"+id+"/sidecars/"+name, "editor/projects/"+id+"/"+file, value, actual)
}
func (s *Service) LoadEffectPresets(ctx context.Context) (Object, string, error) {
	value, rev, err := s.readJSON(ctx, "workspace/effect-presets", "editor/app/effect-presets.json")
	if isMissing(err) {
		return Object{}, "empty", nil
	}
	return value, rev, err
}
func (s *Service) SaveEffectPresets(ctx context.Context, value Object, expected string) (string, error) {
	lock := resourceLock("workspace/effect-presets")
	lock.Lock()
	defer lock.Unlock()
	_, actual, err := s.LoadEffectPresets(ctx)
	if err != nil {
		return "", err
	}
	if err = CheckRevision(actual, expected); err != nil {
		return "", err
	}
	if value == nil {
		return "", apiError("invalid_request", "effect presets must be an object")
	}
	return s.writeJSON(ctx, "workspace/effect-presets", "editor/app/effect-presets.json", value, actual)
}
func (s *Service) WorkspaceID(ctx context.Context) (string, error) {
	lock := resourceLock("workspace/meta")
	lock.Lock()
	defer lock.Unlock()
	value, revision, err := s.readJSON(ctx, "workspace/meta", "editor/.freecut-workspace.json")
	if err == nil {
		if id, ok := value["workspaceId"].(string); ok && id != "" && value["schemaVersion"] == "2.0" {
			return id, nil
		}
	}
	var malformed *Error
	if err != nil && !isMissing(err) && (!errors.As(err, &malformed) || malformed.Code != "invalid_request") {
		return "", err
	}
	id := ID()
	_, err = s.writeJSON(ctx, "workspace/meta", "editor/.freecut-workspace.json", Object{"schemaVersion": "2.0", "workspaceId": id, "createdAt": s.now().UnixMilli()}, revision)
	if errors.Is(err, ErrConflict) {
		winner, _, readErr := s.readJSON(ctx, "workspace/meta", "editor/.freecut-workspace.json")
		if readErr != nil {
			return "", readErr
		}
		return fillString(winner, "workspaceId"), nil
	}
	return id, err
}
