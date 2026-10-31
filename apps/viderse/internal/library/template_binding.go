package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// TemplateBinding is the canonical clone/template-snapshot.ts content binding.
// No file paths enter its identity; MediaFiles supplies the storage locations.
type TemplateBinding struct {
	ProjectID   string                 `json:"projectId"`
	Revision    string                 `json:"revision"`
	SHA256      string                 `json:"sha256"`
	LinksSHA256 string                 `json:"linksSha256"`
	Media       []TemplateMediaBinding `json:"media"`
}
type TemplateMediaBinding struct {
	MediaID     string `json:"mediaId"`
	ContentHash string `json:"contentHash"`
}
type TemplateMediaLinks struct {
	Version  string              `json:"version"`
	MediaIDs []TemplateMediaLink `json:"mediaIds"`
}
type TemplateMediaLink struct {
	ID      string `json:"id"`
	AddedAt int64  `json:"addedAt"`
}

func templateDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Service) bindTemplateMedia(ctx context.Context, tpl *domain.Template, project editor.Project, projectBytes []byte) (TemplateBinding, TemplateMediaLinks, map[string]string, error) {
	ids := map[string]bool{}
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				collect(child)
			}
		case map[string]any:
			for key, child := range v {
				if key == "mediaId" || key == "coverMediaId" {
					if id, ok := child.(string); ok && id != "" {
						ids[id] = true
					}
				} else {
					collect(child)
				}
			}
		}
	}
	collect(project["timeline"])
	if refs, ok := project["mediaItems"].([]any); ok {
		for _, ref := range refs {
			if object, ok := ref.(map[string]any); ok {
				if id, ok := object["id"].(string); ok && id != "" {
					ids[id] = true
				}
			}
		}
	}
	if tpl.Recipe.Scenes.CoverMediaID != "" {
		ids[tpl.Recipe.Scenes.CoverMediaID] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	assets, err := standardAssets()
	if err != nil {
		return TemplateBinding{}, TemplateMediaLinks{}, nil, err
	}
	known := make(map[string]standardAsset, len(assets))
	for _, asset := range assets {
		known[standardAssetID(asset.Name)] = asset
	}
	binding := TemplateBinding{ProjectID: tpl.ID, Revision: editor.Revision(projectBytes), SHA256: templateDigest(projectBytes), Media: make([]TemplateMediaBinding, 0, len(ordered))}
	links := TemplateMediaLinks{Version: "1.0", MediaIDs: make([]TemplateMediaLink, 0, len(ordered))}
	files := make(map[string]string, len(ordered))
	for _, id := range ordered {
		var file, hash string
		if asset, ok := known[id]; ok {
			file, hash = standardAssetPath(asset.Name), "sha256:"+asset.SHA256
		} else {
			file, hash, err = s.templateMediaRecord(ctx, id)
			if err != nil {
				return TemplateBinding{}, TemplateMediaLinks{}, nil, err
			}
		}
		if s.media != nil {
			handle, acquireErr := s.media.Acquire(ctx, file)
			if asset, builtin := known[id]; builtin && (errors.Is(acquireErr, blobstore.ErrNotFound) || errors.Is(acquireErr, os.ErrNotExist)) {
				// Rows created before media-aware CLI wiring can already own a
				// complete project but lack its bundled files. Repair only absent
				// immutable assets; never overwrite present-but-drifted bytes.
				if err := s.publishStandardAssetFiles(ctx, []standardAsset{asset}, nil); err != nil {
					return TemplateBinding{}, TemplateMediaLinks{}, nil, err
				}
				handle, acquireErr = s.media.Acquire(ctx, file)
			}
			if acquireErr != nil {
				return TemplateBinding{}, TemplateMediaLinks{}, nil, missingTemplateMedia(id, acquireErr)
			}
			actual, digestErr := digestTemplateFile(handle)
			handle.Release()
			if digestErr != nil {
				return TemplateBinding{}, TemplateMediaLinks{}, nil, digestErr
			}
			if actual != strings.TrimPrefix(hash, "sha256:") {
				return TemplateBinding{}, TemplateMediaLinks{}, nil, invalidInput("Media của template đã thay đổi", "Tải lại template rồi chạy lại video")
			}
		}
		binding.Media = append(binding.Media, TemplateMediaBinding{MediaID: id, ContentHash: hash})
		links.MediaIDs = append(links.MediaIDs, TemplateMediaLink{ID: id, AddedAt: tpl.CreatedAt.UnixMilli()})
		files[id] = file
	}
	bytes, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return TemplateBinding{}, TemplateMediaLinks{}, nil, err
	}
	binding.LinksSHA256 = templateDigest(append(bytes, '\n'))
	return binding, links, files, nil
}

func digestTemplateFile(handle *media.Handle) (string, error) {
	file, err := os.Open(handle.Path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := file.WriteTo(hash)
	closeErr := file.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func missingTemplateMedia(id string, cause error) error {
	return fmt.Errorf("%w: %s: %v", invalidInput("Media của template không còn trong thư viện", "Mở template và liên kết lại media trước khi chạy video"), id, cause)
}

func (s *Service) templateMediaRecord(ctx context.Context, id string) (string, string, error) {
	// Shared library effects may be referenced directly by authored projects;
	// their immutable content binding is already indexed by the library store.
	overlay, err := s.store.OverlayByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	if overlay != nil {
		if overlay.FilePath == "" || len(overlay.SHA256) != 64 {
			return "", "", missingTemplateMedia(id, fmt.Errorf("invalid overlay metadata"))
		}
		if _, err := hex.DecodeString(overlay.SHA256); err != nil {
			return "", "", missingTemplateMedia(id, err)
		}
		return overlay.FilePath, "sha256:" + overlay.SHA256, nil
	}
	// Uploaded editor media has the same canonical index and source file shape
	// as yt-factory. An opaque cover id is never guessed to be a filesystem path.
	if strings.ContainsAny(id, "/\\") || id == "." || id == ".." || id == "" || s.media == nil {
		return "", "", missingTemplateMedia(id, fmt.Errorf("missing editor media index"))
	}
	handle, err := s.media.Acquire(ctx, "editor/media/"+id+"/metadata.json")
	if err != nil {
		return "", "", missingTemplateMedia(id, err)
	}
	data, err := os.ReadFile(handle.Path)
	handle.Release()
	if err != nil {
		return "", "", err
	}
	var record struct {
		ContentHash string `json:"contentHash"`
		SourceFile  string `json:"sourceFile"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return "", "", missingTemplateMedia(id, err)
	}
	if !strings.HasPrefix(record.ContentHash, "sha256:") || len(record.ContentHash) != 71 || record.SourceFile == "" || strings.ContainsAny(record.SourceFile, "/\\") || record.SourceFile == "." || record.SourceFile == ".." {
		return "", "", missingTemplateMedia(id, fmt.Errorf("invalid editor media metadata"))
	}
	if _, err := hex.DecodeString(record.ContentHash[7:]); err != nil {
		return "", "", missingTemplateMedia(id, err)
	}
	return "editor/media/" + id + "/" + record.SourceFile, record.ContentHash, nil
}
