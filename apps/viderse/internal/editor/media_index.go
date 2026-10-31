package editor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
)

type MediaIndexEntry struct {
	ContentHash string `json:"contentHash"`
	MIME        string `json:"mime"`
	Overlay     bool   `json:"overlay,omitempty"`
}
type MediaIndex map[string]MediaIndexEntry

func mediaFileHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, e := io.Copy(h, r); e != nil {
		return "", e
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
func (s *Service) readMediaIndex(ctx context.Context) (MediaIndex, string, error) {
	o, rev, e := s.readJSON(ctx, "workspace/media-index", "editor/media-index.json")
	if e != nil {
		return nil, rev, e
	}
	if o["version"] != "1.0" {
		return nil, rev, apiError("invalid_request", "Invalid media index version")
	}
	b, e := json.Marshal(o["media"])
	if e != nil {
		return nil, rev, e
	}
	var rows MediaIndex
	if e = json.Unmarshal(b, &rows); e != nil || rows == nil {
		return nil, rev, apiError("invalid_request", "Invalid media index")
	}
	for id, row := range rows {
		if !portableID.MatchString(id) || row.ContentHash == "" || row.MIME == "" {
			return nil, rev, apiError("invalid_request", "Invalid media index row")
		}
	}
	return rows, rev, nil
}
func (s *Service) writeMediaIndex(ctx context.Context, rows MediaIndex, rev string) error {
	o, e := mediaObject(rows)
	if e != nil {
		return e
	}
	_, e = s.writeJSON(ctx, "workspace/media-index", "editor/media-index.json", Object{"version": "1.0", "media": o}, rev)
	return e
}
func (s *Service) rebuildMediaIndexLocked(ctx context.Context) (MediaIndex, error) {
	rows := MediaIndex{}
	ids, e := s.mediaIDs(ctx)
	if e != nil {
		return nil, e
	}
	for _, id := range ids {
		m, e := s.GetMediaMetadata(ctx, id)
		if e != nil {
			continue
		}
		rows[id] = MediaIndexEntry{ContentHash: m.ContentHash, MIME: m.MIME, Overlay: m.Overlay}
	}
	_, rev, e := s.readJSON(ctx, "workspace/media-index", "editor/media-index.json")
	if e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rev = ""
		if s.Repository != nil {
			r, err := s.Repository.Get(ctx, "workspace/media-index")
			if err == nil {
				rev = r.Revision
			} else if !mediaMissing(err) {
				return nil, err
			}
		}
	}
	return rows, s.writeMediaIndex(ctx, rows, rev)
}
func (s *Service) RebuildMediaIndex(ctx context.Context) (MediaIndex, error) {
	l := resourceLock("media-index")
	l.Lock()
	defer l.Unlock()
	return s.rebuildMediaIndexLocked(ctx)
}
func (s *Service) MediaIndex(ctx context.Context) (MediaIndex, error) {
	rows, _, e := s.readMediaIndex(ctx)
	if e == nil {
		return rows, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return s.RebuildMediaIndex(ctx)
}
func (s *Service) setMediaIndex(ctx context.Context, id string, m *MediaRecord) error {
	l := resourceLock("media-index")
	l.Lock()
	defer l.Unlock()
	rows, rev, e := s.readMediaIndex(ctx)
	if e != nil {
		_, e = s.rebuildMediaIndexLocked(ctx)
		return e
	}
	if m == nil {
		delete(rows, id)
	} else {
		rows[id] = MediaIndexEntry{ContentHash: m.ContentHash, MIME: m.MIME, Overlay: m.Overlay}
	}
	return s.writeMediaIndex(ctx, rows, rev)
}
func (s *Service) FindMediaByContentHash(ctx context.Context, hash, prefix string) (*MediaRecord, error) {
	rows, e := s.MediaIndex(ctx)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for id, row := range rows {
		if row.ContentHash == hash && strings.HasPrefix(row.MIME, prefix) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		m, e := s.GetMediaMetadata(ctx, id)
		if mediaMissing(e) {
			if e = s.setMediaIndex(ctx, id, nil); e != nil {
				return nil, e
			}
			continue
		}
		if e != nil {
			return nil, e
		}
		if m.ContentHash == hash && strings.HasPrefix(m.MIME, prefix) {
			return &m, nil
		}
	}
	return nil, nil
}
func (s *Service) MediaContentHashes(ctx context.Context) (map[string]bool, error) {
	rows, e := s.MediaIndex(ctx)
	if e != nil {
		return nil, e
	}
	out := map[string]bool{}
	for _, row := range rows {
		out[row.ContentHash] = true
	}
	return out, nil
}
func (s *Service) ExistingMediaIDs(ctx context.Context, ids []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !portableID.MatchString(id) {
			continue
		}
		seen[id] = true
		if _, e := s.GetMediaMetadata(ctx, id); mediaMissing(e) {
			continue
		} else if e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, nil
}
