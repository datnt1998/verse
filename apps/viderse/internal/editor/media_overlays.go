package editor

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
)

func (s *Service) listOverlayMedia(ctx context.Context) ([]MediaRecord, error) {
	rows, e := s.MediaIndex(ctx)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for id, row := range rows {
		if row.Overlay {
			ids = append(ids, id)
		}
	}
	records, e := s.mediaRecords(ctx, ids)
	if e != nil {
		return nil, e
	}
	out := []MediaRecord{}
	for _, m := range records {
		if m.Overlay {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *Service) ListOverlays(ctx context.Context) ([]MediaRecord, error) {
	if s.OverlayAssets != nil && (s.FFprobe != "" || s.Prober != nil) {
		if e := s.SeedBuiltinOverlays(ctx, s.OverlayAssets); e != nil {
			return nil, e
		}
	}
	records, e := s.listOverlayMedia(ctx)
	if e != nil {
		return nil, e
	}
	byHash := map[string]MediaRecord{}
	for _, m := range records {
		old, ok := byHash[m.ContentHash]
		if !ok || m.CreatedAt < old.CreatedAt || (m.CreatedAt == old.CreatedAt && m.ID < old.ID) {
			byHash[m.ContentHash] = m
		}
	}
	out := []MediaRecord{}
	for _, m := range byHash {
		out = append(out, m)
	}
	sortMediaNewest(out)
	return out, nil
}
func (s *Service) RequireOverlayMedia(ctx context.Context, id string) (MediaRecord, error) {
	m, e := s.GetMediaMetadata(ctx, id)
	if e != nil {
		return m, e
	}
	if !m.Overlay {
		return MediaRecord{}, apiError("not_found", "Media is not in the overlay library")
	}
	return m, nil
}
func (s *Service) RemoveFromOverlayLibrary(ctx context.Context, id string) ([]MediaRecord, error) {
	m, e := s.RequireOverlayMedia(ctx, id)
	if e != nil {
		return nil, e
	}
	copies, e := s.listOverlayMedia(ctx)
	if e != nil {
		return nil, e
	}
	for _, copy := range copies {
		if copy.ContentHash == m.ContentHash {
			if _, e = s.UpdateMediaMetadata(ctx, copy.ID, Object{"overlay": false}); e != nil {
				return nil, e
			}
		}
	}
	return s.ListOverlays(ctx)
}
func overlayMIME(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return ""
}
func (s *Service) AddFileToOverlayLibrary(ctx context.Context, file string) (MediaRecord, error) {
	mime := overlayMIME(file)
	if mime == "" {
		return MediaRecord{}, apiError("invalid_request", "Overlay must be one of .mp4, .m4v, .mov, .webm, .png, .jpg, .jpeg, .webp")
	}
	m, _, e := s.ImportFileOnce(ctx, ImportFileOnceOptions{File: file, FileName: path.Base(file), DeclaredMIME: mime, MIMEPrefix: strings.Split(mime, "/")[0] + "/"})
	if e != nil {
		return MediaRecord{}, e
	}
	overlay := true
	return s.DetectMediaBackground(ctx, m.ID, true, &overlay)
}

var builtinMediaHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Service) SeedBuiltinOverlays(ctx context.Context, assets fs.FS) error {
	l := resourceLock("overlay-library-seed")
	l.Lock()
	defer l.Unlock()
	raw, e := fs.ReadFile(assets, "overlays/manifest.json")
	if e != nil {
		return e
	}
	var manifest struct {
		Effects []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"effects"`
	}
	if e = json.Unmarshal(raw, &manifest); e != nil {
		return e
	}
	if manifest.Effects == nil {
		return apiError("invalid_request", "Invalid built-in overlay manifest")
	}
	hashes, e := s.MediaContentHashes(ctx)
	if e != nil {
		return e
	}
	for _, effect := range manifest.Effects {
		if effect.ID == "" || effect.Name == "" || !fs.ValidPath(effect.File) || !builtinMediaHash.MatchString(effect.SHA256) {
			return apiError("invalid_request", "Invalid built-in overlay manifest entry")
		}
		hash := "sha256:" + effect.SHA256
		if hashes[hash] {
			continue
		}
		f, e := assets.Open(effect.File)
		if e != nil {
			return e
		}
		a, actual, _, e := s.stageFile("editor/overlay-seed/"+effect.ID, f, mediaLimit("video"))
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			os.Remove(a.File)
			return closeErr
		}
		if actual != hash {
			os.Remove(a.File)
			return apiError("internal_error", "Tệp hiệu ứng có sẵn đã thay đổi: "+effect.File)
		}
		m, _, e := s.ImportFileOnce(ctx, ImportFileOnceOptions{File: a.File, ContentHash: hash, FileName: effect.Name, DeclaredMIME: "video/mp4", MIMEPrefix: "video/"})
		os.Remove(a.File)
		if e != nil {
			return e
		}
		overlay := true
		if _, e = s.SaveMediaBackground(ctx, m.ID, MediaBackgroundDetection{Kind: "black"}, m.ContentHash, &overlay); e != nil {
			return e
		}
		hashes[hash] = true
	}
	return nil
}
