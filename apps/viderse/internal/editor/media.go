package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/blobstore"
)

type CommandRunner = tools.Runner
type MediaProber interface {
	Probe(context.Context, string) (ProbeResult, error)
}
type MediaAttribution struct {
	Provider  string  `json:"provider"`
	Author    *string `json:"author,omitempty"`
	AuthorURL *string `json:"authorUrl,omitempty"`
	SourceURL *string `json:"sourceUrl,omitempty"`
	SourceID  *string `json:"sourceId,omitempty"`
	License   *string `json:"license,omitempty"`
}
type MediaBackground struct {
	Kind        string `json:"kind"`
	KeyColor    string `json:"keyColor,omitempty"`
	Version     int    `json:"version"`
	DetectedAt  int64  `json:"detectedAt"`
	ContentHash string `json:"contentHash"`
}
type MediaRecord struct {
	ID          string            `json:"id"`
	FileName    string            `json:"fileName"`
	MIME        string            `json:"mime"`
	Size        int64             `json:"size"`
	ContentHash string            `json:"contentHash"`
	CreatedAt   int64             `json:"createdAt"`
	Probe       ProbeResult       `json:"probe"`
	Tags        []string          `json:"tags,omitempty"`
	Attribution *MediaAttribution `json:"attribution,omitempty"`
	Background  *MediaBackground  `json:"background,omitempty"`
	Overlay     bool              `json:"overlay,omitempty"`
}
type storedMediaRecord struct {
	MediaRecord
	SourceFile  string `json:"sourceFile,omitempty"`
	UpdatedAt   int64  `json:"updatedAt,omitempty"`
	StorageType string `json:"storageType,omitempty"`
	LastUsedAt  int64  `json:"lastUsedAt,omitempty"`
}

// A present empty tags array is different from a record never tagged. Keep
// that distinction on the wire while hiding storage-only fields publicly.
type mediaRecordWire MediaRecord

func (m MediaRecord) MarshalJSON() ([]byte, error) {
	var tags *[]string
	if m.Tags != nil {
		tags = &m.Tags
	}
	return json.Marshal(struct {
		*mediaRecordWire
		Tags *[]string `json:"tags,omitempty"`
	}{mediaRecordWire: (*mediaRecordWire)(&m), Tags: tags})
}
func (m storedMediaRecord) MarshalJSON() ([]byte, error) {
	var tags *[]string
	if m.Tags != nil {
		tags = &m.Tags
	}
	return json.Marshal(struct {
		*mediaRecordWire
		Tags        *[]string `json:"tags,omitempty"`
		SourceFile  string    `json:"sourceFile,omitempty"`
		UpdatedAt   int64     `json:"updatedAt,omitempty"`
		StorageType string    `json:"storageType,omitempty"`
		LastUsedAt  int64     `json:"lastUsedAt,omitempty"`
	}{mediaRecordWire: (*mediaRecordWire)(&m.MediaRecord), Tags: tags, SourceFile: m.SourceFile, UpdatedAt: m.UpdatedAt, StorageType: m.StorageType, LastUsedAt: m.LastUsedAt})
}

type ImportMediaOptions struct {
	ProjectID, FileName, DeclaredMIME string
	Body                              io.Reader
	expectedContentHash               string
}
type ImportMediaFileOptions struct{ ProjectID, File, FileName, DeclaredMIME, ContentHash string }
type ImportFileOnceOptions struct{ ProjectID, File, FileName, DeclaredMIME, ContentHash, MIMEPrefix string }

func mediaMetadataPath(id string) string { return "editor/media/" + id + "/metadata.json" }
func mediaMissing(err error) bool {
	var e *Error
	return errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, os.ErrNotExist) || (errors.As(err, &e) && e.Code == "not_found")
}
func mediaObject(v any) (Object, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var o Object
	e = json.Unmarshal(b, &o)
	return o, e
}
func (s *Service) readMedia(ctx context.Context, id string) (storedMediaRecord, string, error) {
	var m storedMediaRecord
	if e := ValidateID(id); e != nil {
		return m, "", e
	}
	o, rev, e := s.readJSON(ctx, "media/"+id, mediaMetadataPath(id))
	if e != nil {
		return m, "", e
	}
	b, e := json.Marshal(o)
	if e == nil {
		e = json.Unmarshal(b, &m)
	}
	if e != nil || m.ContentHash == "" || m.MIME == "" || m.Size < 0 || m.ID != id {
		return m, "", apiError("invalid_request", "Invalid media metadata")
	}
	return m, rev, nil
}
func (s *Service) saveMedia(ctx context.Context, m storedMediaRecord, rev string) error {
	o, e := mediaObject(m)
	if e != nil {
		return e
	}
	if _, e = s.writeJSON(ctx, "media/"+m.ID, mediaMetadataPath(m.ID), o, rev); e != nil {
		return e
	}
	return s.setMediaIndex(ctx, m.ID, &m.MediaRecord)
}
func (s *Service) GetMediaMetadata(ctx context.Context, id string) (MediaRecord, error) {
	m, _, e := s.readMedia(ctx, id)
	return m.MediaRecord, e
}
func mediaLimit(kind string) int64 {
	name := "YTF_EDITOR_MAX_VIDEO_BYTES"
	fallback := float64(20 << 30)
	switch kind {
	case "image":
		name = "YTF_EDITOR_MAX_IMAGE_BYTES"
		fallback = 50 << 20
	case "audio":
		name = "YTF_EDITOR_MAX_AUDIO_BYTES"
		fallback = 2 << 30
	}
	if n, e := strconv.ParseFloat(os.Getenv(name), 64); e == nil && n > 0 && !math.IsInf(n, 0) && !math.IsNaN(n) && n < float64(math.MaxInt64) {
		return int64(n)
	}
	return int64(fallback)
}
func declaredMediaLimit(mime string) int64 {
	kind := "video"
	if strings.HasPrefix(mime, "image/") {
		kind = "image"
	} else if strings.HasPrefix(mime, "audio/") {
		kind = "audio"
	}
	return mediaLimit(kind)
}
func (s *Service) probeMedia(ctx context.Context, file string) (ProbeResult, error) {
	if s.Prober != nil {
		return s.Prober.Probe(ctx, file)
	}
	return ProbeMediaFile(ctx, file, s.Runner, s.FFprobe)
}
func (s *Service) ImportMedia(ctx context.Context, opts ImportMediaOptions) (MediaRecord, error) {
	if strings.TrimSpace(opts.FileName) == "" || opts.Body == nil {
		return MediaRecord{}, apiError("invalid_request", "fileName and body are required")
	}
	if opts.ProjectID != "" {
		if _, _, e := s.Store.Read(ctx, opts.ProjectID); e != nil {
			return MediaRecord{}, e
		}
	}
	return s.importMedia(ctx, ID(), opts, nil, "")
}
func (s *Service) importMedia(ctx context.Context, id string, opts ImportMediaOptions, current *storedMediaRecord, rev string) (MediaRecord, error) {
	a, hash, n, e := s.stageFile("editor/media/"+id+"/incoming", opts.Body, declaredMediaLimit(opts.DeclaredMIME))
	if e != nil {
		return MediaRecord{}, e
	}
	defer os.Remove(a.File)
	if opts.expectedContentHash != "" && hash != opts.expectedContentHash {
		return MediaRecord{}, apiError("invalid_request", "File content hash does not match")
	}
	probe, e := s.probeMedia(ctx, a.File)
	if e != nil {
		return MediaRecord{}, e
	}
	if n > mediaLimit(probe.Kind) {
		return MediaRecord{}, apiError("payload_too_large", probe.Kind+" upload exceeds size limit")
	}
	filename := strings.TrimSpace(opts.FileName)
	if filename == "" && current != nil {
		filename = current.FileName
	}
	source := "sources/" + strings.TrimPrefix(hash, "sha256:") + "/" + SanitizeFileName(filename)
	a.Path = "editor/media/" + id + "/" + source
	a.ContentType = probe.MIME
	a.Filename = filename
	m := storedMediaRecord{MediaRecord: MediaRecord{ID: id, FileName: filename, MIME: probe.MIME, Size: n, ContentHash: hash, CreatedAt: s.now().UnixMilli(), Probe: probe}, SourceFile: source, UpdatedAt: s.now().UnixMilli(), StorageType: "workspace"}
	if current != nil {
		m.CreatedAt = current.CreatedAt
		m.Tags = current.Tags
		m.Attribution = current.Attribution
		m.Overlay = current.Overlay
	}
	if e = s.Media.Publish(ctx, []media.Artifact{a}, nil); e != nil {
		return MediaRecord{}, e
	}
	if e = s.saveMedia(ctx, m, rev); e != nil {
		if current == nil {
			if cleanup := s.removeMediaResources(ctx, id); cleanup != nil {
				return MediaRecord{}, errors.Join(e, cleanup)
			}
		}
		return MediaRecord{}, e
	}
	if current == nil && opts.ProjectID != "" {
		if _, e = s.LinkMedia(ctx, opts.ProjectID, id); e != nil {
			cleanup := s.removeMediaResources(ctx, id)
			if cleanup != nil {
				return MediaRecord{}, errors.Join(e, cleanup)
			}
			return MediaRecord{}, e
		}
	}
	return m.MediaRecord, nil
}
func (s *Service) ImportMediaFile(ctx context.Context, opts ImportMediaFileOptions) (MediaRecord, error) {
	f, e := os.Open(opts.File)
	if e != nil {
		return MediaRecord{}, e
	}
	defer f.Close()
	return s.ImportMedia(ctx, ImportMediaOptions{ProjectID: opts.ProjectID, FileName: opts.FileName, DeclaredMIME: opts.DeclaredMIME, Body: f, expectedContentHash: opts.ContentHash})
}
func (s *Service) ImportFileOnce(ctx context.Context, opts ImportFileOnceOptions) (MediaRecord, bool, error) {
	if opts.ContentHash == "" {
		f, e := os.Open(opts.File)
		if e != nil {
			return MediaRecord{}, false, e
		}
		hash, e := mediaFileHash(f)
		closeErr := f.Close()
		if e != nil {
			return MediaRecord{}, false, e
		}
		if closeErr != nil {
			return MediaRecord{}, false, closeErr
		}
		opts.ContentHash = hash
	}
	if !strings.HasPrefix(opts.ContentHash, "sha256:") {
		opts.ContentHash = "sha256:" + opts.ContentHash
	}
	lock := resourceLock("media-import/" + opts.ContentHash)
	lock.Lock()
	defer lock.Unlock()
	found, e := s.FindMediaByContentHash(ctx, opts.ContentHash, opts.MIMEPrefix)
	if e != nil {
		return MediaRecord{}, false, e
	}
	if found != nil {
		if opts.ProjectID != "" {
			_, e = s.LinkMedia(ctx, opts.ProjectID, found.ID)
		} else {
			e = s.TouchMedia(ctx, found.ID)
		}
		return *found, true, e
	}
	m, e := s.ImportMediaFile(ctx, ImportMediaFileOptions{ProjectID: opts.ProjectID, File: opts.File, FileName: opts.FileName, DeclaredMIME: opts.DeclaredMIME, ContentHash: opts.ContentHash})
	return m, false, e
}
func (s *Service) ReplaceMedia(ctx context.Context, id string, opts ImportMediaOptions) (MediaRecord, error) {
	if e := ValidateID(id); e != nil {
		return MediaRecord{}, e
	}
	lock := resourceLock("media/" + id)
	lock.Lock()
	defer lock.Unlock()
	m, rev, e := s.readMedia(ctx, id)
	if e != nil {
		return MediaRecord{}, e
	}
	if opts.Body == nil {
		return MediaRecord{}, apiError("invalid_request", "body is required")
	}
	return s.importMedia(ctx, id, opts, &m, rev)
}
func ValidateMediaMetadataPatch(patch Object) (Object, error) {
	if patch == nil {
		return nil, apiError("invalid_request", "media metadata patch must be a JSON object")
	}
	out := Object{}
	if v, ok := patch["tags"]; ok {
		var tags []string
		switch v := v.(type) {
		case []string:
			tags = v
		case []any:
			tags = make([]string, len(v))
			for i, x := range v {
				tag, ok := x.(string)
				if !ok {
					return nil, apiError("invalid_request", "Each tag must be a string")
				}
				tags[i] = tag
			}
		default:
			return nil, apiError("invalid_request", "tags must be an array")
		}
		if len(tags) > 32 {
			return nil, apiError("invalid_request", "tags must have at most 32 strings")
		}
		normalized := make([]string, len(tags))
		for i, tag := range tags {
			tag = strings.TrimSpace(tag)
			n := 0
			for _, r := range tag {
				n++
				if r > 0xffff {
					n++
				}
			}
			if n < 1 || n > 64 {
				return nil, apiError("invalid_request", "Each tag must contain 1–64 characters")
			}
			normalized[i] = tag
		}
		out["tags"] = normalized
	}
	if v, ok := patch["attribution"]; ok {
		if v == nil {
			out["attribution"] = nil
		} else {
			o, ok := v.(map[string]any)
			if !ok {
				return nil, apiError("invalid_request", "attribution must be an object")
			}
			a := Object{}
			for _, k := range []string{"provider", "author", "authorUrl", "sourceUrl", "sourceId", "license"} {
				v, exists := o[k]
				if !exists && k != "provider" {
					continue
				}
				x, ok := v.(string)
				if !ok {
					return nil, apiError("invalid_request", "attribution."+k+" must be a string")
				}
				a[k] = x
			}
			out["attribution"] = a
		}
	}
	if v, ok := patch["overlay"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, apiError("invalid_request", "overlay must be a boolean")
		}
		out["overlay"] = b
	}
	return out, nil
}
func (s *Service) UpdateMediaMetadata(ctx context.Context, id string, patch Object) (MediaRecord, error) {
	p, e := ValidateMediaMetadataPatch(patch)
	if e != nil {
		return MediaRecord{}, e
	}
	if e = ValidateID(id); e != nil {
		return MediaRecord{}, e
	}
	lock := resourceLock("media/" + id)
	lock.Lock()
	defer lock.Unlock()
	m, rev, e := s.readMedia(ctx, id)
	if e != nil {
		return MediaRecord{}, e
	}
	if v, ok := p["tags"]; ok {
		m.Tags = v.([]string)
	}
	if v, ok := p["attribution"]; ok {
		m.Attribution = nil
		if v != nil {
			b, e := json.Marshal(v)
			if e != nil {
				return MediaRecord{}, e
			}
			if e = json.Unmarshal(b, &m.Attribution); e != nil {
				return MediaRecord{}, e
			}
		}
	}
	if v, ok := p["overlay"]; ok {
		m.Overlay = v.(bool)
	}
	m.UpdatedAt = s.now().UnixMilli()
	e = s.saveMedia(ctx, m, rev)
	return m.MediaRecord, e
}
func (s *Service) TouchMedia(ctx context.Context, id string) error {
	if e := ValidateID(id); e != nil {
		return e
	}
	l := resourceLock("media/" + id)
	l.Lock()
	defer l.Unlock()
	m, rev, e := s.readMedia(ctx, id)
	if e != nil {
		return e
	}
	m.LastUsedAt = s.now().UnixMilli()
	o, e := mediaObject(m)
	if e != nil {
		return e
	}
	_, e = s.writeJSON(ctx, "media/"+id, mediaMetadataPath(id), o, rev)
	return e
}
func (s *Service) mediaIDs(ctx context.Context) ([]string, error) {
	ps, e := s.listPaths(ctx, "editor/media/")
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	for _, p := range ps {
		parts := strings.Split(strings.TrimPrefix(p, "editor/media/"), "/")
		if len(parts) == 2 && parts[1] == "metadata.json" && portableID.MatchString(parts[0]) {
			set[parts[0]] = true
		}
	}
	if s.Repository != nil {
		rs, e := s.Repository.List(ctx, "media/")
		if e != nil {
			return nil, e
		}
		for _, r := range rs {
			if id := strings.TrimPrefix(r.Key, "media/"); portableID.MatchString(id) {
				set[id] = true
			}
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func (s *Service) mediaRecords(ctx context.Context, ids []string) ([]MediaRecord, error) {
	out := []MediaRecord{}
	for _, id := range ids {
		m, e := s.GetMediaMetadata(ctx, id)
		if mediaMissing(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, nil
}
func (s *Service) ListMedia(ctx context.Context, pid string) ([]MediaRecord, error) {
	if pid == "" {
		ids, e := s.mediaIDs(ctx)
		if e != nil {
			return nil, e
		}
		return s.mediaRecords(ctx, ids)
	}
	if _, _, e := s.Store.Read(ctx, pid); e != nil {
		return nil, e
	}
	links, _, e := s.readMediaLinks(ctx, pid, false)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for _, l := range links.MediaIDs {
		ids = append(ids, l.ID)
	}
	return s.mediaRecords(ctx, ids)
}

type MediaLink struct {
	ID      string `json:"id"`
	AddedAt int64  `json:"addedAt"`
}
type MediaLinks struct {
	Version  string      `json:"version"`
	MediaIDs []MediaLink `json:"mediaIds"`
}

func mediaLinksPath(pid string) string { return "editor/projects/" + pid + "/links.json" }
func (s *Service) readMediaLinks(ctx context.Context, pid string, strict bool) (MediaLinks, string, error) {
	empty := MediaLinks{Version: "1.0", MediaIDs: []MediaLink{}}
	if e := ValidateID(pid); e != nil {
		return empty, "", e
	}
	o, rev, e := s.readJSON(ctx, "projects/"+pid+"/links", mediaLinksPath(pid))
	if mediaMissing(e) {
		o, rev, e = s.readJSON(ctx, "projects/"+pid+"/links", "editor/projects/"+pid+"/media-links.json")
	}
	if mediaMissing(e) {
		return empty, "", nil
	}
	if e != nil {
		if !strict {
			return empty, "", nil
		}
		return empty, "", e
	}
	b, e := json.Marshal(o)
	var links MediaLinks
	if e == nil {
		e = json.Unmarshal(b, &links)
	}
	if e != nil || links.MediaIDs == nil {
		if !strict {
			return empty, rev, nil
		}
		return empty, rev, apiError("invalid_request", "Media links have an unexpected shape")
	}
	for _, entry := range links.MediaIDs {
		if !portableID.MatchString(entry.ID) {
			return empty, rev, apiError("invalid_request", "Media link id is invalid")
		}
	}
	return links, rev, nil
}
func (s *Service) writeMediaLinks(ctx context.Context, pid string, links MediaLinks, rev string) error {
	o, e := mediaObject(links)
	if e != nil {
		return e
	}
	_, e = s.writeJSON(ctx, "projects/"+pid+"/links", mediaLinksPath(pid), o, rev)
	return e
}

// Caller may already hold the project lock. This routine never acquires a media lock.
func (s *Service) ReconcileProjectMediaLinks(ctx context.Context, pid string, ids []string) error {
	if _, _, e := s.Store.Read(ctx, pid); e != nil {
		return e
	}
	requested := map[string]bool{}
	for _, id := range ids {
		if _, e := s.GetMediaMetadata(ctx, id); e != nil {
			return e
		}
		requested[id] = true
	}
	links, rev, e := s.readMediaLinks(ctx, pid, false)
	if e != nil {
		return e
	}
	next := MediaLinks{Version: links.Version, MediaIDs: []MediaLink{}}
	existing := map[string]bool{}
	for _, entry := range links.MediaIDs {
		if requested[entry.ID] {
			next.MediaIDs = append(next.MediaIDs, entry)
			existing[entry.ID] = true
		}
	}
	for _, id := range ids {
		if !existing[id] {
			next.MediaIDs = append(next.MediaIDs, MediaLink{ID: id, AddedAt: s.now().UnixMilli()})
			existing[id] = true
		}
	}
	a, _ := json.Marshal(links)
	b, _ := json.Marshal(next)
	if string(a) == string(b) {
		return nil
	}
	return s.writeMediaLinks(ctx, pid, next, rev)
}
func (s *Service) changeMediaLink(ctx context.Context, pid, mid string, linked bool) ([]MediaRecord, error) {
	if e := ValidateID(pid); e != nil {
		return nil, e
	}
	if _, e := s.GetMediaMetadata(ctx, mid); e != nil {
		return nil, e
	}
	l := projectLock(pid)
	l.Lock()
	defer l.Unlock()
	links, _, e := s.readMediaLinks(ctx, pid, false)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	found := false
	for _, entry := range links.MediaIDs {
		if entry.ID == mid {
			found = true
			if !linked {
				continue
			}
		}
		ids = append(ids, entry.ID)
	}
	if linked && !found {
		ids = append(ids, mid)
	}
	if e = s.ReconcileProjectMediaLinks(ctx, pid, ids); e != nil {
		return nil, e
	}
	return s.mediaRecords(ctx, ids)
}
func (s *Service) LinkMedia(ctx context.Context, pid, mid string) ([]MediaRecord, error) {
	return s.changeMediaLink(ctx, pid, mid, true)
}
func (s *Service) UnlinkMedia(ctx context.Context, pid, mid string) ([]MediaRecord, error) {
	return s.changeMediaLink(ctx, pid, mid, false)
}
func (s *Service) AcquireMedia(ctx context.Context, id string) (*media.Handle, error) {
	m, _, e := s.readMedia(ctx, id)
	if e != nil {
		return nil, e
	}
	prefix := "editor/media/" + id + "/"
	if m.SourceFile != "" {
		if path.IsAbs(m.SourceFile) || path.Clean(m.SourceFile) != m.SourceFile || strings.HasPrefix(m.SourceFile, "../") || strings.Contains(m.SourceFile, "\\") {
			return nil, apiError("invalid_request", "Invalid source file path")
		}
		return s.acquire(ctx, prefix+m.SourceFile)
	}
	ps, e := s.listPaths(ctx, prefix)
	if e != nil {
		return nil, e
	}
	for _, p := range ps {
		name := strings.TrimPrefix(p, prefix)
		if strings.Contains(name, "/") || strings.HasPrefix(name, ".") || name == "metadata.json" || name == "thumbnail.jpg" || name == "cache" {
			continue
		}
		return s.acquire(ctx, p)
	}
	return nil, apiError("not_found", fmt.Sprintf("Media %s has no source file", id))
}
func (s *Service) removeMediaResources(ctx context.Context, id string) error {
	ps, e := s.listPaths(ctx, "editor/media/"+id+"/")
	if e != nil {
		return e
	}
	resolved := make([]string, 0, len(ps))
	for _, p := range ps {
		actual, err := s.resolvePath(ctx, p)
		if err != nil {
			return err
		}
		resolved = append(resolved, actual)
	}
	if e = s.removeResource(ctx, "media/"+id, mediaMetadataPath(id)); e != nil {
		return e
	}
	if len(resolved) > 0 {
		if e = s.unbindPaths(ctx, resolved); e != nil {
			return e
		}
	}
	return s.setMediaIndex(ctx, id, nil)
}
func (s *Service) DeleteMedia(ctx context.Context, id string) error {
	if e := ValidateID(id); e != nil {
		return e
	}
	l := resourceLock("media/" + id)
	l.Lock()
	defer l.Unlock()
	if _, _, e := s.readMedia(ctx, id); e != nil {
		return e
	}
	pids, e := s.mediaProjectIDs(ctx)
	if e != nil {
		return e
	}
	var failures []error
	for _, pid := range pids {
		l := projectLock(pid)
		l.Lock()
		links, rev, err := s.readMediaLinks(ctx, pid, true)
		if err == nil {
			next := links
			next.MediaIDs = []MediaLink{}
			for _, entry := range links.MediaIDs {
				if entry.ID != id {
					next.MediaIDs = append(next.MediaIDs, entry)
				}
			}
			if len(next.MediaIDs) != len(links.MediaIDs) {
				err = s.writeMediaLinks(ctx, pid, next, rev)
			}
		}
		l.Unlock()
		if err != nil && !mediaMissing(err) {
			failures = append(failures, fmt.Errorf("project %s: %w", pid, err))
		}
	}
	if len(failures) > 0 {
		return apiError("internal_error", errors.Join(failures...).Error())
	}
	return s.removeMediaResources(ctx, id)
}
