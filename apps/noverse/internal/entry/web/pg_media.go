package web

// Hosted media + text-export storage (remove-PG-501 wave):
//   - uploaded assets (bìa, nhạc) live as project_documents kind='asset' rows
//     whose blob_id points at {env}/p/{project}/assets/{sha}.{ext};
//   - rendered covers are story-scoped {…/s/{story}/cover/{sha}.png} cached
//     behind export_cache rows (kind 'cover') so an identical request never
//     re-renders;
//   - produced exports (audio pipeline files and txt/epub/narration exports)
//     are {…/s/{story}/export/{sha}/{name}} objects, registered one row per
//     logical path under cache_key "export-file/<rel>" (chapter audio under
//     "chapter-audio:vN:cM" stays the playback namespace).
//
// Authorization is never re-derived here: every entry point sits behind
// serveStories (read for GET, collab for writes) or requireProject +
// CanCollaborateProject, and the helpers only see resolved slugs.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// ── shared plumbing ──────────────────────────────────────────────────────────

// pgMediaOK reports whether the hosted media deps are wired (blob store +
// registry). A hosted library without pgAudio is a misconfiguration, not a
// file fallback.
func (g *Gateway) pgMediaOK() bool {
	return g.pgAudio != nil && g.pgAudio.Blobs != nil && g.pgAudio.Audio != nil && g.pgAudio.Nar != nil
}

// pgScope resolves the story scope for media/export work. The actor is nil on
// purpose: authorization already ran in serveStories/storyAccessAllowed with
// the real account, and resolve errors (deleted story mid-request) must map
// to the shared storage policy, not a second permission check.
func (g *Gateway) pgScope(ctx context.Context, st library.Story) (postgres.ScopeRef, error) {
	return g.pgAudio.Nar.Resolve(ctx, st.Project, st.Story, nil, false)
}

// serveBlobDelivery handles /api/blobs/stream/<key> and /api/blobs/sign/<key>
// through the shared BlobDelivery plumbing: an authenticated member of the
// blob's project may stream or sign it. Everything else is untouched 404.
func (g *Gateway) serveBlobDelivery(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	if g.pgAudio == nil || g.pgAudio.Blobs == nil || g.pgAudio.Audio == nil {
		return false
	}
	var key string
	var sign bool
	if strings.HasPrefix(r.URL.Path, "/api/blobs/stream/") {
		key = strings.TrimPrefix(r.URL.Path, "/api/blobs/stream/")
	} else if strings.HasPrefix(r.URL.Path, "/api/blobs/sign/") {
		key = strings.TrimPrefix(r.URL.Path, "/api/blobs/sign/")
		sign = true
	} else {
		return false
	}
	d := &BlobDelivery{
		Store: g.pgAudio.Blobs,
		Metadata: func(req *http.Request, key string) (blobstore.BlobInfo, error) {
			blob, err := g.pgAudio.Audio.BlobByObjectKey(req.Context(), key)
			if err != nil {
				return blobstore.BlobInfo{}, err
			}
			return blobstore.BlobInfo{Key: key, Size: blob.SizeBytes, SHA256: blob.SHA256, ContentType: blob.ContentType}, nil
		},
		Authorize: func(_ *http.Request, key string) error {
			if _, err := blobkey.Parse(key); err != nil {
				return ErrBlobDenied
			}
			slug, err := g.pgAudio.Audio.BlobProject(r.Context(), key)
			if err != nil {
				return errs.ErrStorageNotFound
			}
			if !user.CanReadProject(slug) {
				return ErrBlobDenied
			}
			return nil
		},
	}
	r.SetPathValue("key", key)
	if sign {
		d.ServeSign(w, r)
	} else {
		d.ServeStream(w, r)
	}
	return true
}

// pgUpload registers immutable bytes under a content-addressed key: store the
// object (ExpectedSHA256 makes a corrupt/partial write impossible), then the
// registry row. An identical re-upload (or a cache-expired rewrite) finds the
// ready row and reuses it — the object key already commits to the digest.

func (g *Gateway) pgUpload(ctx context.Context, projectID pgtype.UUID, key, contentType string, r io.Reader, sha string, size int64) (postgres.Blob, error) {
	driver := blobstore.DriverName(g.pgAudio.Blobs)
	if driver == "" {
		return postgres.Blob{}, errors.New("BlobStore không khai báo driver")
	}
	input := postgres.PublishInput{ProjectID: projectID, Key: key, Driver: driver, ContentType: contentType, SHA256: sha, Size: size}
	return postgres.UploadAndPublishBlob(ctx, g.pgAudio.Audio, g.pgAudio.Blobs, input, r)
}

// pgDownload streams one blob object to a scratch file (hash verified against
// the registry row).
func (g *Gateway) pgDownload(ctx context.Context, blob postgres.Blob, dest string) error {
	rc, info, err := g.pgAudio.Blobs.Open(ctx, blob.ObjectKey, blobstore.FullRange)
	if err != nil {
		return err
	}
	defer rc.Close()
	if info.SHA256 != "" && !strings.EqualFold(info.SHA256, blob.SHA256) {
		return fmt.Errorf("blob %s hash không khớp đăng ký", blob.ObjectKey)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(f, io.TeeReader(rc, h)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, blob.SHA256) {
		return fmt.Errorf("blob %s sai nội dung", blob.ObjectKey)
	}
	return nil
}

// serveHostedBlob streams one registry blob with the single-range contract of
// serveAudioObject, adding the caller's Cache-Control and Content-Disposition
// name ("" = inline with the key's last segment).
func (g *Gateway) serveHostedBlob(w http.ResponseWriter, r *http.Request, blob postgres.Blob, cacheControl, name string) {
	kind := "inline"
	if name == "" {
		name = fileNameOf(blob.ObjectKey, r)
	} else if r.URL.Query().Get("inline") != "1" {
		kind = "attachment"
	}
	serveStoredBlob(w, r, g.pgAudio.Blobs, blobstore.BlobInfo{Key: blob.ObjectKey, Size: blob.SizeBytes, SHA256: blob.SHA256, ContentType: blob.ContentType}, cacheControl, kind, name, 0)
}

// storyNamesFor returns (title, shortTitle) for one story on whichever
// backend is in play: file mode reads story.json + book.json straight from
// disk (the old path), hosted goes through the repository for story.json and
// the reader for the engine's book.json.
func (g *Gateway) storyNamesFor(ctx context.Context, st library.Story, sr *host.StoryReader) (title, short string) {
	if st.Dir != "" {
		return storyFileNames(st)
	}
	if lib, err := g.lib(ctx, nil); err == nil {
		if sf, err := lib.LoadStoryFile(ctx, nil, st); err == nil && sf != nil {
			title, short = sf.Title, strings.TrimSpace(sf.ShortTitle)
		}
	}
	if sr != nil {
		if canonical := strings.TrimSpace(sr.BookTitle()); canonical != "" {
			title = canonical
		}
	}
	return audio.CleanStoryTitle(title), short
}

// ── uploaded assets (Tư liệu: kind=cover|music) ──────────────────────────────

// pgAssetPrefix maps the upload kind onto the file-mode assets subdir so the
// hosted logical name is byte-identical to the file-mode relative path.
func pgAssetPrefix(kind string) string {
	switch kind {
	case "cover":
		return "bia"
	case "music":
		return "nhac"
	}
	return ""
}

// uploadHostedAsset stores one Tư liệu upload: content-addressed object key
// {env}/p/{project}/assets/{sha}.{ext}, registry row, asset document carrying
// the same logical path the file backend would have written. Name clashes
// dedupe exactly like createUnique: <base>-2, <base>-3, …
func (g *Gateway) uploadHostedAsset(w http.ResponseWriter, r *http.Request, p string) bool {
	if g.pgAudio == nil {
		return false
	}
	if !g.pgMediaOK() {
		jsonError(w, http.StatusInternalServerError, "kho đối tượng chưa cấu hình")
		return true
	}
	projectID, err := g.pgAudio.Nar.ProjectID(r.Context(), p, nil)
	if err != nil {
		jsonMappedError(w, err)
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAssetUpload+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			jsonError(w, http.StatusRequestEntityTooLarge, "Tệp quá lớn (tối đa 64 MB).")
			return true
		}
		jsonError(w, http.StatusBadRequest, "Không đọc được tệp gửi lên.")
		return true
	}
	defer r.MultipartForm.RemoveAll()
	kind := r.FormValue("kind")
	exts, known := assetKinds[kind]
	if !known {
		jsonError(w, http.StatusBadRequest, "Loại tư liệu không hợp lệ.")
		return true
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Thiếu tệp gửi lên.")
		return true
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := false
	for _, e := range exts {
		if e == ext {
			allowed = true
			break
		}
	}
	if !allowed {
		jsonError(w, http.StatusBadRequest, "Định dạng chưa được hỗ trợ.")
		return true
	}
	// Hash while streaming into a scratch file: 64 MB uploads stay off the heap
	// and the object PUT re-reads from disk instead of buffering twice.
	tmp, err := os.CreateTemp("", "ainovel-asset-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(file, maxAssetUpload+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	if written > maxAssetUpload {
		jsonError(w, http.StatusRequestEntityTooLarge, "Tệp quá lớn (tối đa 64 MB).")
		return true
	}
	sha := hex.EncodeToString(h.Sum(nil))
	key, err := g.pgAudio.Keys.Asset(postgres.UUIDString(projectID), sha, strings.TrimPrefix(ext, "."))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	reader, err := os.Open(tmp.Name())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	defer reader.Close()
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
	}
	blob, err := g.pgUpload(r.Context(), projectID, key, ct, reader, sha, written)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return true
	}
	// Name allocation is one create-only claim per candidate: two concurrent
	// uploads of the same filename can never land on the same logical path.
	stem := pgAssetPrefix(kind) + "/" + assetBaseName(header.Filename)
	var logical string
	for suffix := 1; ; suffix++ {
		cand := fmt.Sprintf("%s-%d%s", stem, suffix, ext)
		if suffix == 1 {
			cand = stem + ext
		}
		payload := map[string]any{
			"name": filepath.Base(cand), "size": written,
			"contentType": ct, "sha256": sha, "kind": kind,
		}
		if _, err = g.pgAudio.Audio.CreateAssetDoc(r.Context(), projectID, cand, payload, blob.ID); errors.Is(err, postgres.ErrAssetDocExists) {
			continue
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return true
		}
		logical = cand
		break
	}
	g.projectChanged(p)
	now := time.Now().UTC()
	writeJSON(w, http.StatusCreated, library.Asset{Path: logical, Kind: "file", Size: written, UpdatedAt: &now})
	return true
}

// ── story cover (GET S/cover?size=&volume=) ─────────────────────────────────

// hostedCover serves the story cover on the hosted backend: uploaded project
// image (newest bia/* asset doc) or the deterministic generated cover, the
// rendered PNG cached as a story-scoped blob behind an export_cache row so
// identical inputs never render twice.
func (g *Gateway) hostedCover(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader, size, volume int) {
	if !g.pgMediaOK() {
		jsonError(w, http.StatusInternalServerError, "kho đối tượng chưa cấu hình")
		return
	}
	ctx := r.Context()
	scope, err := g.pgScope(ctx, st)
	if err != nil {
		jsonMappedError(w, err)
		return
	}
	title, short := g.storyNamesFor(ctx, st, sr)
	short = coverShortName(short, title, st)

	docs, err := g.pgAudio.Audio.ListAssetDocs(ctx, scope.ProjectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var uploaded *postgres.AssetDoc
	for i := range docs {
		d := &docs[i]
		if !strings.HasPrefix(d.LogicalName, "bia/") {
			continue
		}
		switch strings.ToLower(filepath.Ext(d.LogicalName)) {
		case ".jpg", ".jpeg", ".png", ".webp":
		default:
			continue
		}
		if uploaded == nil || d.UpdatedAt.After(uploaded.UpdatedAt) {
			uploaded = d
		}
	}

	// Same fingerprint contract as Layout.CoverPNG: size + volume + (uploaded
	// identity | generated name). The uploaded side hashes the blob identity,
	// which never changes for identical bytes.
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|", size, volume)
	fingerprint := ""
	if uploaded != nil {
		fmt.Fprintf(h, "up:%s:%d", postgres.UUIDString(uploaded.BlobID), uploaded.Version)
		fingerprint = "up:" + postgres.UUIDString(uploaded.BlobID)
	} else {
		fmt.Fprintf(h, "gen:%s", short)
		fingerprint = "gen:" + short
	}
	cacheKey := "cover/" + fmt.Sprintf("%x", h.Sum(nil))[:16]

	if entry, err := g.pgAudio.Cache.Get(ctx, scope, cacheKey, "cover"); err == nil {
		if blob, berr := g.pgAudio.Audio.BlobByID(ctx, scope.ProjectID, entry.BlobID); berr == nil {
			g.serveHostedBlob(w, r, blob, "private, max-age=86400, immutable", "")
			return
		}
	}

	// Cache miss: materialize the uploaded image (if any) into a scratch cover
	// dir and let the file-mode renderer do its deterministic job.
	root, err := os.MkdirTemp("", "ainovel-pg-cover-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(root)
	layout := audio.Layout{
		AssetsDir: filepath.Join(root, "assets"),
		AudioDir:  filepath.Join(root, "audio"),
	}
	if uploaded != nil {
		blob, err := g.pgAudio.Audio.BlobByID(ctx, scope.ProjectID, uploaded.BlobID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := g.pgDownload(ctx, blob, filepath.Join(layout.CoverDir(), filepath.Base(uploaded.LogicalName))); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	path, err := layout.CoverPNG(short, volume, size)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không dựng được bìa.")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sha := sha256.Sum256(data)
	shaHex := hex.EncodeToString(sha[:])
	key, err := g.pgAudio.Keys.Cover(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), shaHex, "png")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	blob, err := g.pgUpload(ctx, scope.ProjectID, key, "image/png", bytes.NewReader(data), shaHex, int64(len(data)))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	manifest, _ := json.Marshal(map[string]any{
		"size": size, "volume": volume, "fingerprint": fingerprint,
	})
	if _, err := g.pgAudio.Cache.Put(ctx, scope, postgres.ExportCacheEntry{
		CacheKey: cacheKey, Kind: "cover", BlobID: blob.ID, InputManifest: manifest,
	}, nil); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	g.serveHostedBlob(w, r, blob, "private, max-age=86400, immutable", "")
}

// ── text exports (txt / epub / narration) ────────────────────────────────────

// pgExportFilePrefix namespaces the per-file registry entries inside
// export_cache: every produced artifact is one row, keyed by the same
// relative path the file backend would have on disk.
const pgExportFilePrefix = "export-file/"

// pgExportFileManifest is the registry metadata for one produced artifact.
type pgExportFileManifest struct {
	Rel        string     `json:"rel"`  // logical path, "/" separated
	Name       string     `json:"name"` // base name for download/format detection
	Size       int64      `json:"size"`
	SHA256     string     `json:"sha256"`
	Key        string     `json:"key"` // object key (audit aid; blob_id is the FK)
	Volume     *int       `json:"volume"`
	Format     string     `json:"format"` // txt | epub | audio | other (exportFormatOf)
	ExportedAt *time.Time `json:"exportedAt"`
	Chapters   int        `json:"chapters"`
	Skipped    []int      `json:"skipped"`
}

// pgExportKindOf maps a produced file name onto an export_cache kind (the
// schema constraint's file-type values).
func pgExportKindOf(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".epub":
		return "epub"
	case ".srt":
		return "srt"
	case ".mp3":
		return "mp3"
	case ".m4b":
		return "m4b"
	case ".wav":
		return "wav"
	case ".zip":
		return "zip"
	}
	return "txt"
}

// pgLiveCacheEntries lists live (unexpired) export_cache rows of the story.
// List returns expired rows too — the maintenance GC owns their deletion —
// so readers filter here once.
func pgLiveCacheEntries(entries []postgres.ExportCacheEntry, now time.Time) []postgres.ExportCacheEntry {
	out := entries[:0]
	for _, e := range entries {
		if e.ExpiresAt != nil && e.ExpiresAt.Before(now) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// pgHostedExportFiles lists every live produced artifact of the story
// (text + audio rows share the export-file/ namespace; chapter audio joins
// as its own registry rows so the matrix sees split files too).
func (g *Gateway) pgHostedExportFiles(ctx context.Context, st library.Story, singleVolume bool) ([]ExportFileAPI, error) {
	scope, err := g.pgScope(ctx, st)
	if err != nil {
		return nil, err
	}
	entries, err := g.pgAudio.Cache.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	files := []ExportFileAPI{}
	for _, e := range pgLiveCacheEntries(entries, time.Now()) {
		if !strings.HasPrefix(e.CacheKey, pgExportFilePrefix) && !strings.HasPrefix(e.CacheKey, "chapter-audio:") {
			continue
		}
		var m pgExportFileManifest
		if err := json.Unmarshal(e.InputManifest, &m); err != nil || m.Name == "" {
			continue
		}
		f := ExportFileAPI{
			Path: m.Key, Name: m.Name, Format: m.Format,
			Size: m.Size, UpdatedAt: m.ExportedAt, Volume: m.Volume,
		}
		if f.Format == "" {
			f.Format = exportFormatOf(m.Name)
		}
		if f.Volume == nil {
			f.Volume = inferExportVolume(m.Rel)
		}
		if f.Volume == nil && singleVolume {
			one := 1
			f.Volume = &one
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

// pgHostedExportResolve looks up one produced artifact by the caller-visible
// identifier: the manifest object key (files[].path / job results) or the
// logical relative path (export-file/<rel>). Unknown identifiers are
// ErrStorageNotFound; a blob that is not an export artifact of this story is
// never served through this path.
func (g *Gateway) pgHostedExportResolve(ctx context.Context, st library.Story, ident string) (postgres.Blob, pgExportFileManifest, error) {
	scope, err := g.pgScope(ctx, st)
	if err != nil {
		return postgres.Blob{}, pgExportFileManifest{}, err
	}
	if obj, err := blobkey.Parse(ident); err == nil && obj.ProjectID != "" {
		blob, berr := g.pgAudio.Audio.BlobByKey(ctx, scope.ProjectID, ident)
		if berr != nil {
			return postgres.Blob{}, pgExportFileManifest{}, berr
		}
		entries, lerr := g.pgAudio.Cache.List(ctx, scope)
		if lerr != nil {
			return postgres.Blob{}, pgExportFileManifest{}, lerr
		}
		for _, e := range pgLiveCacheEntries(entries, time.Now()) {
			if !strings.HasPrefix(e.CacheKey, pgExportFilePrefix) && !strings.HasPrefix(e.CacheKey, "chapter-audio:") {
				continue
			}
			if e.BlobID != blob.ID {
				continue
			}
			var m pgExportFileManifest
			_ = json.Unmarshal(e.InputManifest, &m)
			return blob, m, nil
		}
		return postgres.Blob{}, pgExportFileManifest{}, errs.ErrStorageNotFound
	}
	if err := validateExportRel(ident); err != nil {
		return postgres.Blob{}, pgExportFileManifest{}, err
	}
	cacheKey := pgExportFilePrefix + filepath.ToSlash(ident)
	entry, err := g.pgAudio.Cache.Get(ctx, scope, cacheKey, pgExportKindOf(filepath.Base(ident)))
	if err != nil {
		return postgres.Blob{}, pgExportFileManifest{}, err
	}
	blob, err := g.pgAudio.Audio.BlobByID(ctx, scope.ProjectID, entry.BlobID)
	if err != nil {
		return postgres.Blob{}, pgExportFileManifest{}, err
	}
	var m pgExportFileManifest
	_ = json.Unmarshal(entry.InputManifest, &m)
	return blob, m, nil
}

// validateExportRel applies the same rules as safeExportPath to a logical
// export path (relative, no "..", stays inside the namespace).
func validateExportRel(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) {
		return errors.New("invalid path")
	}
	for _, seg := range strings.FieldsFunc(rel, func(c rune) bool { return c == '/' || c == '\\' }) {
		if seg == ".." {
			return errors.New("invalid path")
		}
	}
	return nil
}

// pgHostedExport runs one text export into scratch, publishes the artifact as
// an export blob, and registers the export-file/<rel> row. It mirrors
// runGatewayExport's job tracking and conflict handling; the only difference
// is that "the file" is a registry row, never a path.
func (g *Gateway) pgHostedExport(ctx context.Context, st library.Story, sr *host.StoryReader, opts exp.Options, rel string) (*exp.Result, string, error) {
	preview := opts
	preview.DryRun, preview.Progress = true, nil
	incoming, err := sr.Export(ctx, preview)
	if err != nil {
		return nil, "", publicExportError(err, "")
	}
	mode := opts.OnExists
	if mode == "" {
		mode = "fail"
	}
	scope, err := g.pgScope(ctx, st)
	if err != nil {
		return nil, "", err
	}
	kind := pgExportKindOf(filepath.Base(rel))
	cacheKey := pgExportFilePrefix + filepath.ToSlash(rel)
	entry, getErr := g.pgAudio.Cache.Get(ctx, scope, cacheKey, kind)
	exists := getErr == nil
	// stem/variant back the keep-mode insert-claim loop after the render.
	stem, relExt, variant := rel, filepath.Ext(rel), 0
	switch mode {
	case "fail":
		if exists {
			return nil, "", &exportConflictError{payload: pgExportConflict(rel, entry, incoming)}
		}
	case "keep":
		if exists {
			var err error
			stem, variant, err = g.pgKeepVariant(ctx, scope, rel)
			if err != nil {
				return nil, "", err
			}
			rel = stem + relExt
			cacheKey = pgExportFilePrefix + filepath.ToSlash(rel)
		}
	}
	hub := g.jobsHub()
	hub.mu.Lock()
	hub.seq++
	id := fmt.Sprintf("%s/export/%d", st.ID, uint64(time.Now().UnixNano())+hub.seq)
	hub.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	job := Job{ID: id, Kind: "export", Title: "Xuất truyện", Status: JobRunning, Href: storyRouteBase(st.Project, st.Story) + "?tab=exports", Project: st.Project, Story: st.Story, StartedAt: time.Now()}
	hub.mu.Lock()
	if hub.controls == nil {
		hub.controls = map[string]func(context.Context, string) error{}
	}
	hub.controls[id] = func(ctx context.Context, op string) error {
		switch op {
		case "cancel":
			cancel()
			return nil
		case "retry":
			current, ok := hub.get(id)
			if !ok {
				return errJobNotFound
			}
			if current.Status != JobFailed {
				return errJobUnsupported
			}
			_, _, err := g.pgHostedExport(ctx, st, sr, opts, rel)
			return err
		default:
			return errJobUnsupported
		}
	}
	hub.mu.Unlock()
	hub.apply(JobChange{Job: job})
	opts.Progress = func(done, total, chapter int, title string) {
		p := float64(done) / float64(total)
		job.Progress = &p
		job.Unit = &JobUnit{Done: done, Total: total, Label: "Chương"}
		job.Step = fmt.Sprintf("Chương %d · %s", chapter, title)
		hub.apply(JobChange{Job: job})
	}

	dir, err := os.MkdirTemp("", "ainovel-pg-export-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)
	opts.OutPath = filepath.Join(dir, filepath.Base(rel))
	// The scratch target never exists; OnExists decisions were already made
	// against the registry above.
	opts.OnExists = ""
	result, err := sr.Export(ctx, opts)
	now := time.Now()
	job.FinishedAt = &now
	if err != nil {
		job.Status = JobFailed
		job.Error = publicExportError(err, "").Error()
		hub.apply(JobChange{Job: job})
		return nil, id, publicExportError(err, "")
	}

	file, err := os.Open(result.Path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	shaHex, size, err := blobstore.DigestAll(file)
	if err != nil {
		return nil, "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	// fail and keep claim the logical name with a create-only insert so a
	// concurrent export can never replace the winner's row; keep bumps the
	// variant (re-keying the object) until a free name is claimed.
	var blob postgres.Blob
	var key, name string
	for {
		name = filepath.Base(rel)
		key, err = g.pgAudio.Keys.Export(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), shaHex, name)
		if err != nil {
			return nil, "", fmt.Errorf("khoá xuất %s: %w", name, err)
		}
		ct := mime.TypeByExtension(filepath.Ext(name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, "", err
		}
		blob, err = g.pgUpload(ctx, scope.ProjectID, key, ct, file, shaHex, size)
		if err != nil {
			return nil, "", err
		}
		at := time.Now().UTC()
		manifest, _ := json.Marshal(pgExportFileManifest{
			Rel: filepath.ToSlash(rel), Name: name, Size: size, SHA256: shaHex, Key: key,
			Volume: inferExportVolume(filepath.ToSlash(rel)), Format: exportFormatOf(name),
			ExportedAt: &at, Chapters: result.Chapters, Skipped: nonNilInts(result.Skipped),
		})
		cacheKey = pgExportFilePrefix + filepath.ToSlash(rel)
		entryRow := postgres.ExportCacheEntry{CacheKey: cacheKey, Kind: kind, BlobID: blob.ID, InputManifest: manifest}
		if mode == "replace" {
			if _, err = g.pgAudio.Cache.Put(ctx, scope, entryRow, nil); err == nil {
				break
			}
		} else {
			_, err = g.pgAudio.Cache.Create(ctx, scope, entryRow)
			if err == nil {
				break
			}
			if errors.Is(err, postgres.ErrExportCacheExists) {
				if mode == "keep" {
					variant++
					if stem == rel {
						// First keep collision after an originally free name: mint the
						// timestamped base exactly like pgKeepVariant does.
						stem = strings.TrimSuffix(rel, relExt) + "-" + time.Now().Format("20060102-150405")
					}
					rel = pgKeepName(stem, relExt, variant)
					continue
				}
				loser, _ := g.pgAudio.Cache.Get(ctx, scope, cacheKey, kind)
				err = &exportConflictError{payload: pgExportConflict(rel, loser, incoming)}
			}
		}
		finished := time.Now()
		job.FinishedAt = &finished
		job.Status = JobFailed
		job.Error = publicExportError(err, "").Error()
		hub.apply(JobChange{Job: job})
		return nil, id, err
	}

	job.Status = JobDoneS
	p := 1.0
	job.Progress = &p
	download := storyAPIBase(st.Project, st.Story) + "/exports/file?path=" + url.QueryEscape(key)
	job.Result = &JobResult{Title: "Đã xuất truyện", Sub: fmt.Sprintf("%d chương · %s", result.Chapters, humanBytes(result.Bytes)), Actions: []JobAction{{Label: "Tải xuống", Kind: "download", Href: download, Primary: true}}, Data: map[string]any{"path": key, "bytes": result.Bytes, "chapters": result.Chapters, "skipped": nonNilInts(result.Skipped)}}
	hub.apply(JobChange{Job: job})
	// Result.Path carries the object key so the response's `path` field
	// resolves through /exports/file the same way manifest files do.
	result.Path = key
	return result, id, nil
}

// pgKeepVariant mints the timestamped base stem for onExists="keep" and the
// variant index whose name was free at check time. The final claim still goes
// through the insert loop in pgHostedExport — this snapshot only picks the
// starting point, never the winner.
func (g *Gateway) pgKeepVariant(ctx context.Context, scope postgres.ScopeRef, rel string) (string, int, error) {
	ext := filepath.Ext(rel)
	base := strings.TrimSuffix(rel, ext) + "-" + time.Now().Format("20060102-150405")
	for n := 1; ; n++ {
		name := pgKeepName(base, ext, n)
		_, err := g.pgAudio.Cache.Get(ctx, scope, pgExportFilePrefix+filepath.ToSlash(name), pgExportKindOf(filepath.Base(name)))
		if errors.Is(err, postgres.ErrJobNotFound) {
			return base, n, nil
		}
		if err != nil {
			return "", 0, err
		}
	}
}

// pgKeepName renders variant n of a keep base stem.
func pgKeepName(base, ext string, n int) string {
	if n <= 1 {
		return base + ext
	}
	return fmt.Sprintf("%s-%d%s", base, n, ext)
}

// pgExportConflict builds the same conflict payload exportConflict returns on
// file mode, with registry values standing in for os.Stat facts.
func pgExportConflict(rel string, entry postgres.ExportCacheEntry, incoming *exp.Result) map[string]any {
	display := filepath.ToSlash(rel)
	existing := map[string]any{"path": display, "size": 0, "bytes": 0, "modTime": nil, "modifiedAt": nil, "chapters": nil}
	var m pgExportFileManifest
	if err := json.Unmarshal(entry.InputManifest, &m); err == nil {
		existing["size"], existing["bytes"] = m.Size, m.Size
		existing["modTime"], existing["modifiedAt"] = m.ExportedAt, m.ExportedAt
		if m.Chapters > 0 {
			existing["chapters"] = m.Chapters
		}
	}
	next := map[string]any{"path": display, "chapters": incoming.Chapters, "size": incoming.Bytes, "bytesEstimate": incoming.Bytes, "skipped": nonNilInts(incoming.Skipped)}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(rel)), ".")
	return map[string]any{"path": display, "format": format, "existing": existing, "incoming": next}
}
