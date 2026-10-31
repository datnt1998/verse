package editor

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// ExportGate returns nil policy for ordinary projects. Commit's file is a
// private local staged file; diagnostics are persisted even when it refuses.
type ExportGate interface {
	Begin(context.Context, string, string) (Object, error)
	Commit(context.Context, string, Object, string, Object) (Object, error)
}

type BeginCloneExportOptions struct {
	ProjectRevision string `json:"projectRevision,omitempty"`
}
type BeginExportOptions struct {
	FileName     string                   `json:"fileName"`
	MIME         string                   `json:"mime"`
	ExpectedSize int64                    `json:"expectedSize,omitempty"`
	RequestID    string                   `json:"requestId,omitempty"`
	Clone        *BeginCloneExportOptions `json:"clone,omitempty"`
}
type ExportRecord struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	MIME        string `json:"mime"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"createdAt"`
	CommittedAt *int64 `json:"committedAt"`
}

func exportObject(v any) Object {
	switch x := v.(type) {
	case map[string]any:
		return x
	case Project:
		return Object(x)
	}
	return nil
}
func exportArray(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []Object:
		a := make([]any, len(x))
		for i := range x {
			a[i] = x[i]
		}
		return a
	}
	return nil
}
func exportString(v any) string { s, _ := v.(string); return s }
func exportNumber(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		v, _ := n.Float64()
		return v
	}
	return math.NaN()
}
func exportInt(v any) int64 {
	n := exportNumber(v)
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return int64(n)
}
func exportCopy(v Object) (Object, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var out Object
	e = json.Unmarshal(b, &out)
	return out, e
}
func exportKey(pid, eid string) (string, error) {
	if err := ValidateID(pid); err != nil {
		return "", err
	}
	if err := ValidateID(eid); err != nil {
		return "", err
	}
	return "exports/" + pid + "/" + eid, nil
}
func exportLegacy(pid, eid string) string {
	return "editor/projects/" + pid + "/exports/" + eid + "/meta.json"
}
func exportPublic(m Object) ExportRecord {
	var at *int64
	if m["committedAt"] != nil {
		n := exportInt(m["committedAt"])
		at = &n
	}
	return ExportRecord{ID: exportString(m["id"]), FileName: exportString(m["fileName"]), MIME: exportString(m["mime"]), Size: exportInt(m["size"]), Status: exportString(m["status"]), CreatedAt: exportInt(m["createdAt"]), CommittedAt: at}
}
func (s *Service) exportMeta(ctx context.Context, pid, eid string) (Object, string, error) {
	key, e := exportKey(pid, eid)
	if e != nil {
		return nil, "", e
	}
	return s.readJSON(ctx, key, exportLegacy(pid, eid))
}
func (s *Service) saveExportMeta(ctx context.Context, pid, eid string, m Object, revision string) error {
	key, e := exportKey(pid, eid)
	if e != nil {
		return e
	}
	_, e = s.writeJSON(ctx, key, exportLegacy(pid, eid), m, revision)
	return e
}
func exportMissing(err error) bool {
	var e *Error
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, blobstore.ErrNotFound) || (errors.As(err, &e) && e.Code == "not_found")
}
func (s *Service) exportRoot() string {
	if s.StagingDir != "" {
		return s.StagingDir
	}
	return s.Store.StagingDir
}
func (s *Service) exportDir(pid, eid string) string {
	return filepath.Join(s.exportRoot(), "editor-export-sessions", pid, eid)
}
func (s *Service) exportStagedFile(pid, eid string) string {
	return filepath.Join(s.exportDir(pid, eid), "upload")
}
func exportLock(pid, eid string) func() {
	l := resourceLock("export-session/" + pid + "/" + eid)
	l.Lock()
	return l.Unlock
}

func (s *Service) BeginExport(ctx context.Context, pid string, opts BeginExportOptions) (string, error) {
	if err := ValidateID(pid); err != nil {
		return "", err
	}
	if strings.TrimSpace(opts.FileName) == "" || opts.ExpectedSize < 0 {
		return "", apiError("invalid_request", "fileName and a nonnegative expectedSize are required")
	}
	if opts.RequestID != "" {
		if err := ValidateID(opts.RequestID); err != nil {
			return "", err
		}
	}
	p, revision, err := s.projectStore().Read(ctx, pid)
	if err != nil {
		return "", err
	}
	root := s.exportRoot()
	if root == "" {
		return "", errors.New("editor staging directory is required")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	if opts.ExpectedSize > 0 {
		var st syscall.Statfs_t
		if err = syscall.Statfs(root, &st); err != nil {
			return "", err
		}
		free := uint64(st.Bavail) * uint64(st.Bsize)
		if uint64(opts.ExpectedSize) > free || free-uint64(opts.ExpectedSize) < 512*1024*1024 {
			return "", apiError("insufficient_storage", "Not enough free disk space for this export")
		}
	}
	rendered := ""
	if opts.Clone != nil {
		rendered = opts.Clone.ProjectRevision
	}
	var policy Object
	if s.Gate != nil {
		policy, err = s.Gate.Begin(ctx, pid, rendered)
		if err != nil {
			return "", err
		}
	} else {
		owner := ""
		if s.ExportOwner != nil {
			owner, err = s.ExportOwner(ctx, pid)
			if err != nil {
				return "", err
			}
		}
		metadata := p.Metadata()
		if owner != "" || exportString(metadata["cloneJobId"]) != "" || exportString(p["cloneJobId"]) != "" {
			return "", apiError("invalid_request", "Clone export gate is unavailable", Object{"cloneGate": "gate_unavailable"})
		}
	}
	if policy != nil {
		if rendered != revision {
			return "", apiError("conflict", "Project đã đổi từ bản render — lưu rồi xuất lại", Object{"cloneGate": "revision_mismatch", "revision": revision})
		}
		policy, err = exportCopy(policy)
		if err != nil {
			return "", err
		}
		policy["projectRevision"] = revision
	}
	eid := ID()
	dir := s.exportDir(pid, eid)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	mime := opts.MIME
	if mime == "" {
		mime = "application/octet-stream"
	}
	meta := Object{"id": eid, "fileName": opts.FileName, "mime": mime, "size": 0, "status": "staged", "createdAt": s.now().UnixMilli(), "committedAt": nil}
	if policy != nil {
		meta["clone"] = policy
	}
	if opts.RequestID != "" {
		meta["requestId"] = opts.RequestID
	}
	if err = s.saveExportMeta(ctx, pid, eid, meta, ""); err != nil {
		return "", errors.Join(err, os.RemoveAll(dir))
	}
	return eid, nil
}

// UploadExport never publishes the mutable session file to the media store.
// A failed append is truncated back to its acknowledged offset. The SHA-256
// state is persisted privately, so chunks do not re-read earlier gigabytes.
func (s *Service) UploadExport(ctx context.Context, pid, eid string, body io.Reader, offset *int64) error {
	if _, err := exportKey(pid, eid); err != nil {
		return err
	}
	unlock := exportLock(pid, eid)
	defer unlock()
	meta, revision, err := s.exportMeta(ctx, pid, eid)
	if err != nil {
		return err
	}
	if meta["status"] != "staged" {
		return apiError("conflict", "Export is already committed")
	}
	if body == nil {
		return apiError("invalid_request", "Upload body is required")
	}
	target := s.exportStagedFile(pid, eid)
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	current := int64(0)
	if st, e := os.Stat(target); e == nil {
		current = st.Size()
	} else if !os.IsNotExist(e) {
		return e
	}
	if offset != nil && (*offset < 0 || (*offset != 0 && (*offset != current || *offset != exportInt(meta["uploadedBytes"])))) {
		return apiError("conflict", fmt.Sprintf("Sai vị trí chunk: gửi %d, server đang giữ %d", *offset, current), Object{"recovery": "Tải lại từ chunk bị thiếu"})
	}
	if offset != nil && *offset > 0 {
		hash := sha256.New()
		state, e := base64.StdEncoding.DecodeString(exportString(meta["stagedHashState"]))
		if e != nil {
			return apiError("conflict", "Invalid staged checksum state; restart upload")
		}
		if e = hash.(encoding.BinaryUnmarshaler).UnmarshalBinary(state); e != nil {
			return apiError("conflict", "Missing staged checksum state; restart upload")
		}
		f, e := os.OpenFile(target, os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			return e
		}
		n, e := io.Copy(io.MultiWriter(f, hash), &exportContextReader{ctx: ctx, r: body})
		syncErr := f.Sync()
		closeErr := f.Close()
		rollback := func(cause error) error { return errors.Join(cause, os.Truncate(target, current)) }
		if e != nil || syncErr != nil || closeErr != nil {
			return rollback(errors.Join(e, syncErr, closeErr))
		}
		state, e = hash.(encoding.BinaryMarshaler).MarshalBinary()
		if e != nil {
			return rollback(e)
		}
		next, e := exportCopy(meta)
		if e != nil {
			return rollback(e)
		}
		next["uploadedBytes"] = current + n
		next["stagedSha256"] = hex.EncodeToString(hash.Sum(nil))
		next["stagedHashState"] = base64.StdEncoding.EncodeToString(state)
		if e = s.saveExportMeta(ctx, pid, eid, next, revision); e != nil {
			return rollback(e)
		}
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".upload-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	writer := io.MultiWriter(f, hash)
	total := int64(0)
	n, err := io.Copy(writer, &exportContextReader{ctx: ctx, r: body})
	total += n
	syncErr := f.Sync()
	closeErr := f.Close()
	if err != nil || syncErr != nil || closeErr != nil {
		return errors.Join(err, syncErr, closeErr)
	}
	backup := target + ".previous"
	hadOld := current > 0
	if hadOld {
		if err = os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err = os.Rename(f.Name(), target); err != nil {
		if hadOld {
			return errors.Join(err, os.Rename(backup, target))
		}
		return err
	}
	state, stateErr := hash.(encoding.BinaryMarshaler).MarshalBinary()
	next, err := exportCopy(meta)
	err = errors.Join(err, stateErr)
	if err == nil {
		next["uploadedBytes"] = total
		next["stagedSha256"] = hex.EncodeToString(hash.Sum(nil))
		next["stagedHashState"] = base64.StdEncoding.EncodeToString(state)
		err = s.saveExportMeta(ctx, pid, eid, next, revision)
	}
	if err != nil {
		removeErr := os.Remove(target)
		if hadOld {
			return errors.Join(err, removeErr, os.Rename(backup, target))
		}
		return errors.Join(err, removeErr)
	}
	if hadOld {
		return os.Remove(backup)
	}
	return nil
}

type exportContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *exportContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func exportFileHash(file string) (string, int64, error) {
	f, e := os.Open(file)
	if e != nil {
		return "", 0, e
	}
	h := sha256.New()
	n, e := io.Copy(h, f)
	e = errors.Join(e, f.Close())
	return hex.EncodeToString(h.Sum(nil)), n, e
}

func (s *Service) exportDiagnostics(ctx context.Context, pid, eid string, diagnostics Object) error {
	key, _ := exportKey(pid, eid)
	key += "/diagnostics"
	legacy := "editor/projects/" + pid + "/exports/" + eid + "/gate-diagnostics.json"
	_, rev, e := s.readJSON(ctx, key, legacy)
	if e != nil && !exportMissing(e) {
		return e
	}
	_, e = s.writeJSON(ctx, key, legacy, diagnostics, rev)
	return e
}

func (s *Service) CommitExport(ctx context.Context, pid, eid string, metadata Object) (ExportRecord, error) {
	if _, err := exportKey(pid, eid); err != nil {
		return ExportRecord{}, err
	}
	if err := validateExportCommitMetadata(metadata); err != nil {
		return ExportRecord{}, err
	}
	unlock := exportLock(pid, eid)
	defer unlock()
	meta, revision, err := s.exportMeta(ctx, pid, eid)
	if err != nil {
		return ExportRecord{}, err
	}
	if meta["status"] != "staged" {
		return ExportRecord{}, apiError("conflict", "Export is already committed")
	}
	file := s.exportStagedFile(pid, eid)
	sum, size, err := exportFileHash(file)
	if os.IsNotExist(err) || err == nil && size == 0 {
		return ExportRecord{}, apiError("invalid_request", "Export has no uploaded content")
	}
	if err != nil {
		return ExportRecord{}, err
	}
	session := exportObject(meta["clone"])
	if metadata["captionCoverage"] != nil && session == nil {
		return ExportRecord{}, apiError("invalid_request", "Caption coverage requires a bound clone export session")
	}
	var coverage Object
	if session != nil {
		current, currentRevision, e := s.projectStore().Read(ctx, pid)
		if e != nil {
			return ExportRecord{}, e
		}
		coverage, e = validateExportCaptionCoverage(metadata, session, current, currentRevision)
		if e != nil {
			return ExportRecord{}, e
		}
		reasons := []string{}
		if exportString(meta["stagedSha256"]) != "" && sum != meta["stagedSha256"] {
			reasons = append(reasons, "File staged đã đổi sau khi tải lên")
		}
		if currentRevision != session["projectRevision"] {
			reasons = append(reasons, "Project đã đổi sau khi bắt đầu xuất — xuất lại từ đầu")
		}
		if len(reasons) > 0 {
			e = apiError("invalid_request", "Bản xuất chưa đạt kiểm tra Độ hoàn thiện", Object{"cloneGate": "final_check_failed", "blockers": reasons})
			return ExportRecord{}, errors.Join(e, s.exportDiagnostics(ctx, pid, eid, Object{"rejectedAt": s.now().Format("2006-01-02T15:04:05.999Z07:00"), "reasons": reasons, "score": nil, "probed": nil}))
		}
		if s.Gate == nil {
			return ExportRecord{}, apiError("invalid_request", "Clone export gate is unavailable", Object{"cloneGate": "gate_unavailable"})
		}
		diagnostics, e := s.Gate.Commit(ctx, pid, session, file, metadata)
		if diagnostics != nil {
			if de := s.exportDiagnostics(ctx, pid, eid, diagnostics); de != nil {
				return ExportRecord{}, errors.Join(e, de)
			}
		}
		if e != nil {
			return ExportRecord{}, e
		}
	} else if exportString(meta["stagedSha256"]) != "" && sum != meta["stagedSha256"] {
		return ExportRecord{}, apiError("conflict", "Staged export checksum changed")
	}
	next, err := exportCopy(meta)
	if err != nil {
		return ExportRecord{}, err
	}
	next["size"] = size
	next["stagedSha256"] = sum
	next["status"] = "ready"
	next["committedAt"] = s.now().UnixMilli()
	if coverage != nil {
		exportObject(next["clone"])["captionCoverage"] = coverage
	}
	if err = s.publishExportFile(ctx, pid, eid, file, next, revision); err != nil {
		return ExportRecord{}, err
	}
	if err = os.RemoveAll(s.exportDir(pid, eid)); err != nil {
		return exportPublic(next), err
	}
	return exportPublic(next), nil
}

// A fresh immutable path and a metadata CAS make file replacement atomic for
// readers. The old path stays live until the new pointer is durable.
func (s *Service) publishExportFile(ctx context.Context, pid, eid, file string, next Object, revision string) error {
	key, err := exportKey(pid, eid)
	if err != nil {
		return err
	}
	repo := s.repository()
	if repo == nil {
		return errors.New("editor repository is required")
	}
	previous, err := repo.Get(ctx, key+"/file")
	if exportMissing(err) {
		// next already contains the replacement digest. Only persisted metadata
		// can supply the old digest when adopting a pre-binary-root export.
		meta, _, readErr := s.exportMeta(ctx, pid, eid)
		if readErr != nil {
			return readErr
		}
		old := exportString(meta["filePath"])
		if old == "" && meta["status"] == "ready" {
			old = "editor/projects/" + pid + "/exports/" + eid + "/" + SanitizeFileName(exportString(meta["fileName"]))
		}
		previous = Resource{}
		if old != "" {
			previous = Resource{Path: old, Revision: "sha256:" + exportString(meta["stagedSha256"])}
		}
	} else if err != nil {
		return err
	}
	logical := "editor/exports/" + pid + "/" + eid + "/files/" + ID() + "/" + SanitizeFileName(exportString(next["fileName"]))
	next["filePath"] = logical
	// The revision gate and publication cutover are serialized with project writes.
	lock := projectLock(pid)
	lock.Lock()
	defer lock.Unlock()
	if session := exportObject(next["clone"]); session != nil && next["status"] == "ready" {
		_, current, e := s.projectStore().Read(ctx, pid)
		if e != nil {
			return e
		} else if current != session["projectRevision"] && previous.Path == "" {
			return apiError("conflict", "Project changed while committing export")
		}
	}
	_, err = s.publishJSONFile(ctx, key, exportLegacy(pid, eid), next, revision,
		media.Artifact{Path: logical, File: file, ContentType: exportString(next["mime"]), Filename: exportString(next["fileName"])},
		key+"/file", "sha256:"+exportString(next["stagedSha256"]), previous)
	return err
}

func (s *Service) AcquireExport(ctx context.Context, pid, eid string) (*media.Handle, ExportRecord, error) {
	if _, err := exportKey(pid, eid); err != nil {
		return nil, ExportRecord{}, err
	}
	unlock := exportLock(pid, eid)
	defer unlock()
	meta, _, err := s.exportMeta(ctx, pid, eid)
	if err != nil {
		return nil, ExportRecord{}, err
	}
	record := exportPublic(meta)
	if record.Status != "ready" {
		return nil, record, apiError("not_found", "Export is not committed yet")
	}
	path := exportString(meta["filePath"])
	if path == "" {
		path = "editor/projects/" + pid + "/exports/" + eid + "/" + SanitizeFileName(record.FileName)
	}
	h, err := s.acquire(ctx, path)
	return h, record, err
}

func (s *Service) ListExports(ctx context.Context, pid string) ([]ExportRecord, error) {
	if err := ValidateID(pid); err != nil {
		return nil, err
	}
	if _, _, err := s.projectStore().Read(ctx, pid); err != nil {
		return nil, err
	}
	ids, err := s.exportIDs(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make([]ExportRecord, 0, len(ids))
	for _, id := range ids {
		m, _, e := s.exportMeta(ctx, pid, id)
		if e != nil {
			if exportMissing(e) {
				continue
			}
			return nil, e
		}
		out = append(out, exportPublic(m))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}
func (s *Service) exportIDs(ctx context.Context, pid string) ([]string, error) {
	seen := map[string]bool{}
	if repo := s.repository(); repo != nil {
		resources, e := repo.List(ctx, "exports/"+pid+"/")
		if e != nil {
			return nil, e
		}
		for _, r := range resources {
			id := strings.TrimPrefix(r.Key, "exports/"+pid+"/")
			if portableID.MatchString(id) {
				seen[id] = true
			}
		}
	}
	paths, e := s.listPaths(ctx, "editor/projects/"+pid+"/exports/")
	if e != nil {
		return nil, e
	}
	for _, p := range paths {
		suffix := strings.TrimPrefix(p, "editor/projects/"+pid+"/exports/")
		parts := strings.Split(suffix, "/")
		if len(parts) == 2 && parts[1] == "meta.json" && portableID.MatchString(parts[0]) {
			seen[parts[0]] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
func (s *Service) DeleteExport(ctx context.Context, pid, eid string) error {
	if _, err := exportKey(pid, eid); err != nil {
		return err
	}
	unlock := exportLock(pid, eid)
	defer unlock()
	return s.deleteExportLocked(ctx, pid, eid)
}
func (s *Service) deleteExportLocked(ctx context.Context, pid, eid string) error {
	meta, _, err := s.exportMeta(ctx, pid, eid)
	if err != nil {
		return err
	}
	key, _ := exportKey(pid, eid)
	paths, err := s.listPaths(ctx, "editor/exports/"+pid+"/"+eid+"/")
	if err != nil {
		return err
	}
	legacyPaths, e := s.listPaths(ctx, "editor/projects/"+pid+"/exports/"+eid+"/")
	if e != nil {
		return e
	}
	paths = append(paths, legacyPaths...)
	if p := exportString(meta["filePath"]); p != "" {
		paths = append(paths, p)
	}
	repo := s.repository()
	if repo == nil {
		return errors.New("editor repository is required")
	}
	rows, err := repo.List(ctx, key)
	if err != nil {
		return err
	}
	owned := make([]Resource, 0, len(rows))
	ownedPaths := make([]string, 0, len(rows))
	rooted := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.Key == key || strings.HasPrefix(row.Key, key+"/") {
			owned = append(owned, row)
			ownedPaths = append(ownedPaths, row.Path)
			rooted[row.Path] = true
		}
	}
	if err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
		for _, row := range owned {
			if e := repo.Delete(tx, row.Key, row.Revision); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		return err
	}
	cleanupCtx := context.WithoutCancel(ctx)
	discardReleasedLocal(cleanupCtx, s.Media, ownedPaths...)
	// Resource deletion releases its own bindings transactionally. Only legacy
	// paths without resource ownership may be unbound directly after commit.
	unowned := make([]string, 0, len(paths))
	for _, path := range paths {
		if !rooted[path] {
			unowned = append(unowned, path)
		}
	}
	return errors.Join(s.unbindPaths(cleanupCtx, unowned, nil), os.RemoveAll(s.exportDir(pid, eid)))
}
func (s *Service) DeleteStagedExportsForRequest(ctx context.Context, pid, requestID string) ([]string, error) {
	if err := ValidateID(pid); err != nil {
		return nil, err
	}
	if err := ValidateID(requestID); err != nil {
		return nil, err
	}
	ids, err := s.exportIDs(ctx, pid)
	if err != nil {
		return nil, err
	}
	deleted := []string{}
	for _, id := range ids {
		unlock := exportLock(pid, id)
		m, _, e := s.exportMeta(ctx, pid, id)
		if e == nil && m["status"] != "ready" && m["requestId"] == requestID {
			e = s.deleteExportLocked(ctx, pid, id)
			if e == nil {
				deleted = append(deleted, id)
			}
		}
		unlock()
		if e != nil && !exportMissing(e) {
			return deleted, e
		}
	}
	return deleted, nil
}
func (s *Service) ReadExportEvidence(ctx context.Context, pid, eid string) (Object, error) {
	key, err := exportKey(pid, eid)
	if err != nil {
		return nil, err
	}
	out := Object{"stagedSha256": nil, "gateDiagnostics": nil}
	m, _, err := s.exportMeta(ctx, pid, eid)
	if err != nil && !exportMissing(err) {
		return nil, err
	}
	if hash := exportString(m["stagedSha256"]); len(hash) == 64 {
		if _, e := hex.DecodeString(hash); e == nil && strings.ToLower(hash) == hash {
			out["stagedSha256"] = hash
		}
	}
	d, _, e := s.readJSON(ctx, key+"/diagnostics", "editor/projects/"+pid+"/exports/"+eid+"/gate-diagnostics.json")
	if e != nil && !exportMissing(e) {
		return nil, e
	}
	if e == nil {
		out["gateDiagnostics"] = d
	}
	return out, nil
}

func (s *Service) ReplaceCloneExportFile(ctx context.Context, pid, eid string, transform func(context.Context, string, string, Project, Object) error) (ExportRecord, error) {
	if _, err := exportKey(pid, eid); err != nil {
		return ExportRecord{}, err
	}
	if transform == nil {
		return ExportRecord{}, apiError("invalid_request", "Transform is required")
	}
	if err := ctx.Err(); err != nil {
		return ExportRecord{}, err
	}
	unlock := exportLock(pid, eid)
	defer unlock()
	meta, revision, err := s.exportMeta(ctx, pid, eid)
	if err != nil {
		return ExportRecord{}, err
	}
	if meta["status"] != "ready" {
		return ExportRecord{}, apiError("conflict", "Export is not committed yet")
	}
	lock := projectLock(pid)
	lock.Lock()
	p, current, e := s.projectStore().Read(ctx, pid)
	session := exportObject(meta["clone"])
	if e == nil && (session == nil || current != session["projectRevision"]) {
		e = apiError("conflict", "Project changed since the committed clone render")
	}
	var snapshot Project
	var coverage Object
	if e == nil {
		snapshot = Clone(p)
		if c := exportObject(session["captionCoverage"]); c != nil {
			coverage, e = exportCopy(c)
		}
	}
	lock.Unlock()
	if e != nil {
		return ExportRecord{}, e
	}
	input := exportString(meta["filePath"])
	if input == "" {
		input = "editor/projects/" + pid + "/exports/" + eid + "/" + SanitizeFileName(exportString(meta["fileName"]))
	}
	h, err := s.acquire(ctx, input)
	if err != nil {
		return ExportRecord{}, err
	}
	defer h.Release()
	root := s.exportRoot()
	if root == "" {
		return ExportRecord{}, errors.New("editor staging directory is required")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return ExportRecord{}, err
	}
	dir, err := os.MkdirTemp(root, "export-replacing-")
	if err != nil {
		return ExportRecord{}, err
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "replacement"+filepath.Ext(exportString(meta["fileName"])))
	if err = transform(ctx, h.Path, output, snapshot, coverage); err != nil {
		return ExportRecord{}, err
	}
	if err = ctx.Err(); err != nil {
		return ExportRecord{}, err
	}
	// The transform has finished reading the old bytes. Release its shared
	// blob lease before CAS attempts to retire that content under an exclusive
	// lease; retaining it through publication deadlocks against our own reader.
	h.Release()
	sum, size, err := exportFileHash(output)
	if err != nil {
		return ExportRecord{}, err
	}
	if size == 0 {
		return ExportRecord{}, apiError("invalid_request", "Replacement is empty")
	}
	next, err := exportCopy(meta)
	if err != nil {
		return ExportRecord{}, err
	}
	_, hasOriginalDigest := next["originalSha256"]
	_, hasOriginalSize := next["originalSize"]
	if !hasOriginalDigest && !hasOriginalSize {
		originalDigest, originalSize, ok := originalUploadLineage(meta)
		if !ok {
			originalDigest = exportString(meta["stagedSha256"])
			originalSize = exportInt(meta["size"])
		}
		next["originalSha256"] = originalDigest
		next["originalSize"] = originalSize
	}
	next["size"] = size
	next["stagedSha256"] = sum
	if err = s.publishExportFile(ctx, pid, eid, output, next, revision); err != nil {
		return ExportRecord{}, err
	}
	return exportPublic(next), nil
}
