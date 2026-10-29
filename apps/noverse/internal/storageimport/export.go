package storageimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/noverse/internal/blobkey"
	"github.com/datnt1998/noverse/internal/blobstore"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
	"github.com/datnt1998/noverse/internal/store"
)

// ExportOptions configures the reverse export into a file-mode library.
type ExportOptions struct {
	SourceID       string
	Destination    string
	Pool           *postgres.Pool
	Blobs          blobstore.BlobStore
	RevokeSessions bool
	Now            time.Time
}

// ExportReport is the machine-readable outcome of a reverse export.
type ExportReport struct {
	Destination     string    `json:"destination"`
	FilesWritten    int       `json:"filesWritten"`
	BlobsWritten    int       `json:"blobsWritten"`
	BytesWritten    int64     `json:"bytesWritten"`
	Accounts        int       `json:"accounts"`
	Projects        int       `json:"projects"`
	Stories         int       `json:"stories"`
	Checkpoints     int       `json:"checkpoints"`
	Events          int       `json:"events"`
	ChapterVersions int       `json:"chapterVersions"`
	SessionsRevoked int       `json:"sessionsRevoked"`
	PairingsRevoked int       `json:"pairingsRevoked"`
	Skipped         []string  `json:"skipped"`
	FinishedAt      time.Time `json:"finishedAt"`
}

// PathAccountsExport is the account file inside an exported library. It is the
// same layout the file backend reads (webauth opens one web-accounts.json), so
// a rollback only has to point the CLI/serve at this tree.
const PathAccountsExport = ".ainovel/web-accounts.json"

// ExportFiles writes the database back out as a file-mode library: library
// config, accounts, projects, stories, chapter records + retained history +
// prose, checkpoints, event streams, outlines, state, comments, reviews, audio
// metadata and every ready blob. The database — not the import ledger — is the
// source of truth for which rows exist, so projects and blobs created after
// the cutover are exported too.
func ExportFiles(ctx context.Context, opts ExportOptions) (*ExportReport, error) {
	if opts.Pool == nil || opts.Destination == "" {
		return nil, errors.New("cần --destination và kết nối Postgres")
	}
	if err := ensureEmptyDir(opts.Destination); err != nil {
		return nil, err
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	exp := &exporter{
		opts: opts, pool: opts.Pool.Pool(),
		report:   &ExportReport{Destination: opts.Destination, Skipped: []string{}},
		written:  map[string][]byte{},
		project:  map[string]exportProject{},
		story:    map[string]exportStory{},
		dirLive:  map[string]bool{},
		rootLive: true,
		dirs:     Dirs{}.withDefaults(),
	}
	if err := exp.run(ctx); err != nil {
		return exp.report, err
	}
	if opts.RevokeSessions {
		if err := exp.revoke(ctx); err != nil {
			return exp.report, err
		}
	}
	exp.report.FinishedAt = time.Now().UTC()
	return exp.report, nil
}

func ensureEmptyDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s không phải thư mục", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s không rỗng — reverse-export chỉ ghi vào thư mục mới", dir)
	}
	return nil
}

type exporter struct {
	opts    ExportOptions
	pool    querier
	report  *ExportReport
	written map[string][]byte
	// project/story index live rows by id so blob and document paths resolve
	// without re-querying (and only live rows are exported).
	project map[string]exportProject
	story   map[string]exportStory
	// dirLive maps every imported project/story directory to its current
	// live/deleted state, so a blob kept under a soft-deleted story is not
	// written back (that would resurrect the deleted tree).
	dirLive  map[string]bool
	rootLive bool
	dirs     Dirs
}

func (e *exporter) run(ctx context.Context) error {
	library, err := e.loadLibrary(ctx)
	if err != nil {
		return err
	}
	e.dirs = library.Dirs
	if err := e.writeJSON(".ainovel/library.json", library); err != nil {
		return err
	}
	if err := e.exportAccounts(ctx); err != nil {
		return err
	}
	projects, err := e.loadProjects(ctx)
	if err != nil {
		return err
	}
	e.report.Projects = len(projects)
	if err := e.loadDirLiveness(ctx); err != nil {
		return err
	}
	for _, p := range projects {
		if err := e.exportProject(ctx, p, library.Dirs); err != nil {
			return fmt.Errorf("dự án %s: %w", p.Slug, err)
		}
	}
	if err := e.exportBlobs(ctx); err != nil {
		return err
	}
	return nil
}

type exportProject struct {
	ID, Slug, Name, Description, Notes, UniverseName string
	CreatedAt                                        time.Time
}

func (p exportProject) dir() string {
	if p.Slug == "_" {
		return ""
	}
	return p.Slug
}

func (e *exporter) loadLibrary(ctx context.Context) (LibraryConfig, error) {
	var raw []byte
	cfg := LibraryConfig{Dirs: Dirs{}.withDefaults()}
	err := e.pool.QueryRow(ctx, `SELECT payload FROM library_settings WHERE source_id=$1`, uuidArg(e.opts.SourceID)).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	cfg.Dirs = cfg.Dirs.withDefaults()
	return cfg, nil
}

// loadProjects enumerates the live project rows. The import ledger is only a
// historical record; a project created through the hosted library after the
// cutover must still be part of a rollback.
func (e *exporter) loadProjects(ctx context.Context) ([]exportProject, error) {
	rows, err := e.pool.Query(ctx, `
SELECT id::text, slug, name, description, notes, universe_name, created_at
FROM projects WHERE deleted_at IS NULL ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []exportProject
	for rows.Next() {
		var p exportProject
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.Notes, &p.UniverseName, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	e.project = make(map[string]exportProject, len(out))
	for _, p := range out {
		e.project[p.ID] = p
	}
	return out, rows.Err()
}

// loadDirLiveness records whether each imported project/story directory still
// exists live. A soft-deleted owner means its files must not be written back.
func (e *exporter) loadDirLiveness(ctx context.Context) error {
	rows, err := e.pool.Query(ctx, `
SELECT i.relative_path, i.target_kind, st.deleted_at IS NULL, p.deleted_at IS NULL
FROM import_items i
LEFT JOIN stories st ON i.target_kind='story' AND st.id::text = i.target_id
LEFT JOIN projects p ON i.target_kind='project' AND p.id::text = i.target_id
WHERE i.source_id=$1 AND i.target_kind IN ('story','project')`, uuidArg(e.opts.SourceID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rel, kind string
		var storyLive, projectLive *bool
		if err := rows.Scan(&rel, &kind, &storyLive, &projectLive); err != nil {
			return err
		}
		switch kind {
		case "story":
			dir := strings.TrimSuffix(rel, "/story.json")
			e.dirLive[dir] = storyLive != nil && *storyLive
		case "project":
			if rel == "project.json" {
				e.rootLive = projectLive != nil && *projectLive
				continue
			}
			e.dirLive[strings.TrimSuffix(rel, "/project.json")] = projectLive != nil && *projectLive
		}
	}
	return rows.Err()
}

// ownerDeleted reports whether a source path belongs to a soft-deleted
// project or story (deleted content must stay deleted on rollback).
func (e *exporter) ownerDeleted(rel string) bool {
	best := -1
	live := e.rootLive
	for dir, dirLive := range e.dirLive {
		if dir == "" {
			continue
		}
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			if len(dir) > best {
				best = len(dir)
				live = dirLive
			}
		}
	}
	return !live
}

func (e *exporter) exportProject(ctx context.Context, p exportProject, dirs Dirs) error {
	projDir := p.dir()
	meta := map[string]any{
		"name": p.Name, "description": p.Description, "notes": p.Notes,
		"createdAt": p.CreatedAt.Format(time.RFC3339Nano),
	}
	if p.UniverseName != "" {
		meta["universeName"] = p.UniverseName
	}
	if err := e.writeJSON(join(projDir, "project.json"), meta); err != nil {
		return err
	}
	// project documents (rules/ideas/config/assets text). The logical name is
	// the project-relative source path, so it is written back verbatim.
	docRows, err := e.pool.Query(ctx, `
SELECT id::text, kind, logical_name, COALESCE(text_content,'')
FROM project_documents WHERE project_id=$1 ORDER BY logical_name`, uuidArg(p.ID))
	if err != nil {
		return err
	}
	type doc struct{ id, kind, name, text string }
	var docs []doc
	for docRows.Next() {
		var d doc
		if err := docRows.Scan(&d.id, &d.kind, &d.name, &d.text); err != nil {
			docRows.Close()
			return err
		}
		docs = append(docs, d)
	}
	docRows.Close()
	for _, d := range docs {
		rel := projectDocPath(d.kind, d.name, dirs)
		if rel == "" {
			e.report.Skipped = append(e.report.Skipped, "document "+d.kind+"/"+d.name)
			continue
		}
		if err := e.writeFile(join(projDir, rel), []byte(d.text)); err != nil {
			return err
		}
	}
	stories, err := e.loadStories(ctx, p.ID)
	if err != nil {
		return err
	}
	e.report.Stories += len(stories)
	for _, s := range stories {
		if err := e.exportStory(ctx, projDir, p.ID, s, dirs); err != nil {
			return fmt.Errorf("truyện %s: %w", s.Slug, err)
		}
	}
	return nil
}

// projectDocPath maps one project_documents row back to its file location.
// Rows imported after the canonical mapping carry a project-relative path;
// older bare names fall back to the same layout the file backend uses.
func projectDocPath(kind, logical string, dirs Dirs) string {
	switch {
	case kind == "idea":
		name := strings.TrimSuffix(logical, ".md")
		if strings.Contains(name, "/") {
			return name + ".md"
		}
		return path.Join(dirs.Ideas, name+".md")
	case strings.Contains(logical, "/"):
		return logical
	case kind == "config":
		return path.Join(".ainovel", logical)
	case kind == "rules":
		return path.Join(".ainovel", "rules", logical)
	case kind == "asset":
		return path.Join(dirs.Assets, logical)
	}
	return logical
}

type exportStory struct {
	ID, Slug, Title, ShortTitle, Status string
	Idea                                string
	TitleRenamed                        bool
	Tags                                []string
	PublishedAt                         *time.Time
	CreatedAt                           time.Time
}

func (e *exporter) loadStories(ctx context.Context, projectID string) ([]exportStory, error) {
	rows, err := e.pool.Query(ctx, `
SELECT s.id::text, s.slug, s.title, s.short_title, s.status, s.tags, s.published_at, s.created_at,
       s.title_renamed, COALESCE(d.text_content,'')
FROM stories s
LEFT JOIN project_documents d ON d.project_id = s.project_id AND d.id = s.idea_document_id
WHERE s.project_id=$1 AND s.deleted_at IS NULL ORDER BY s.slug`, uuidArg(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []exportStory
	for rows.Next() {
		var s exportStory
		if err := rows.Scan(&s.ID, &s.Slug, &s.Title, &s.ShortTitle, &s.Status, &s.Tags,
			&s.PublishedAt, &s.CreatedAt, &s.TitleRenamed, &s.Idea); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	for _, s := range out {
		e.story[s.ID] = s
	}
	return out, rows.Err()
}

func (e *exporter) exportStory(ctx context.Context, projDir, projectID string, s exportStory, dirs Dirs) error {
	storyDir := join(projDir, dirs.Stories, s.Slug)
	if e.project[projectID].Slug == "_" {
		// Legacy stories of the anonymous project live at the library root.
		storyDir = s.Slug
	}
	deleted, err := e.deletedChapters(ctx, projectID, s)
	if err != nil {
		return err
	}
	if err := e.writeJSON(join(storyDir, "story.json"), map[string]any{
		"title": s.Title, "shortTitle": s.ShortTitle, "titleRenamed": s.TitleRenamed,
		"status": s.Status, "tags": s.Tags, "idea": s.Idea,
		"publishedAt": s.PublishedAt, "createdAt": s.CreatedAt.Format(time.RFC3339Nano),
	}); err != nil {
		return err
	}
	engineDir := join(storyDir, "output", "novel")
	if err := e.exportState(ctx, engineDir, storyDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportOutlines(ctx, engineDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportCharacters(ctx, engineDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportChapters(ctx, engineDir, projectID, s, deleted); err != nil {
		return err
	}
	if err := e.exportCheckpoints(ctx, engineDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportStreams(ctx, engineDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportComments(ctx, storyDir, projectID, s, deleted); err != nil {
		return err
	}
	if err := e.exportReviews(ctx, engineDir, projectID, s); err != nil {
		return err
	}
	if err := e.exportArtifacts(ctx, engineDir, storyDir, projectID, s, deleted); err != nil {
		return err
	}
	return e.exportAudio(ctx, storyDir, projectID, s, dirs)
}

// engineSingletonPaths covers state kinds the importer produces that are not
// part of the production document mapping. Values are engine-relative except
// ".ainovel/..." which resolves under the story dir.
var engineSingletonPaths = map[string]string{
	"format":         "meta/format.json",
	"signals":        "meta/signals.json",
	"comment_events": ".ainovel/gop-y/_events.json",
}

// exportState writes every story_state row under the path the Store reads.
func (e *exporter) exportState(ctx context.Context, engineDir, storyDir, projectID string, s exportStory) error {
	rows, err := e.pool.Query(ctx, `SELECT kind, payload::text FROM story_state WHERE project_id=$1 AND story_id=$2 ORDER BY kind`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	type stateRow struct{ kind, payload string }
	var states []stateRow
	for rows.Next() {
		var st stateRow
		if err := rows.Scan(&st.kind, &st.payload); err != nil {
			rows.Close()
			return err
		}
		states = append(states, st)
	}
	rows.Close()
	for _, st := range states {
		rel, ok := postgres.DocumentPath(postgres.DocumentRef{Class: postgres.DocClassState, Kind: st.kind})
		if !ok {
			rel, ok = engineSingletonPaths[st.kind]
		}
		if !ok {
			return fmt.Errorf("state kind %q không có đường dẫn file — dữ liệu sẽ mất", st.kind)
		}
		target := engineDir
		if strings.HasPrefix(rel, ".ainovel/") {
			target = storyDir
		}
		if err := e.writeFile(join(target, rel), []byte(st.payload)); err != nil {
			return err
		}
	}
	return nil
}

func (e *exporter) exportOutlines(ctx context.Context, engineDir, projectID string, s exportStory) error {
	rows, err := e.pool.Query(ctx, `SELECT kind, COALESCE(payload::text,''), COALESCE(text_content,'') FROM outlines WHERE project_id=$1 AND story_id=$2`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, payload, text string
		if err := rows.Scan(&kind, &payload, &text); err != nil {
			return err
		}
		rel, ok := postgres.DocumentPath(postgres.DocumentRef{Class: postgres.DocClassOutline, Kind: kind})
		if !ok {
			return fmt.Errorf("outline kind %q không có đường dẫn file — dữ liệu sẽ mất", kind)
		}
		content := payload
		if text != "" {
			content = text
		}
		if err := e.writeFile(join(engineDir, rel), []byte(content)); err != nil {
			return err
		}
	}
	return rows.Err()
}

// exportCharacters prefers the story_state document (the byte-faithful source
// of characters.json); typed character rows are the fallback for imports that
// predate it.
func (e *exporter) exportCharacters(ctx context.Context, engineDir, projectID string, s exportStory) error {
	var payload []byte
	err := e.pool.QueryRow(ctx, `
SELECT payload FROM story_state WHERE project_id=$1 AND story_id=$2 AND kind='characters'`,
		uuidArg(projectID), uuidArg(s.ID)).Scan(&payload)
	switch {
	case err == nil:
		return e.writeFile(join(engineDir, "characters.json"), payload)
	case !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	rows, err := e.pool.Query(ctx, `SELECT payload FROM characters WHERE project_id=$1 AND story_id=$2 ORDER BY legacy_id`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	var chars []json.RawMessage
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		chars = append(chars, json.RawMessage(raw))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(chars) == 0 {
		return nil
	}
	return e.writeJSON(join(engineDir, "characters.json"), chars)
}

// deletedChapters lists the soft-deleted chapter numbers of a story so no
// chapter-scoped file is written back for them.
func (e *exporter) deletedChapters(ctx context.Context, projectID string, s exportStory) (map[int]bool, error) {
	rows, err := e.pool.Query(ctx, `
SELECT number FROM chapters WHERE project_id=$1 AND story_id=$2 AND deleted_at IS NOT NULL`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// exportRecord is domain.ChapterRecord with raw facts/style bytes: the file
// backend loads this exact shape, and the raw objects keep every source field.
type exportRecord struct {
	Version       int                  `json:"version"`
	Chapter       int                  `json:"chapter"`
	Revision      int                  `json:"revision"`
	Build         string               `json:"build,omitempty"`
	Origin        domain.ChapterOrigin `json:"origin"`
	Content       string               `json:"content"`
	ContentSHA256 string               `json:"content_sha256"`
	Facts         json.RawMessage      `json:"facts"`
	StyleDelta    json.RawMessage      `json:"style_delta"`
	AcceptedAt    time.Time            `json:"accepted_at"`
}

func (e *exporter) exportChapters(ctx context.Context, engineDir, projectID string, s exportStory, deleted map[int]bool) error {
	rows, err := e.pool.Query(ctx, `
SELECT id::text, number, title, draft_text, COALESCE(current_revision,0) FROM chapters
WHERE project_id=$1 AND story_id=$2 AND deleted_at IS NULL ORDER BY number`, uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	type chapter struct {
		id, title, draft string
		number, current  int
	}
	var chapters []chapter
	for rows.Next() {
		var c chapter
		if err := rows.Scan(&c.id, &c.number, &c.title, &c.draft, &c.current); err != nil {
			rows.Close()
			return err
		}
		chapters = append(chapters, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, c := range chapters {
		if c.draft != "" && !deleted[c.number] {
			if err := e.writeFile(join(engineDir, fmt.Sprintf("drafts/%02d.draft.md", c.number)), []byte(c.draft)); err != nil {
				return err
			}
		}
		versions, err := e.chapterVersions(ctx, projectID, c.id, c.number)
		if err != nil {
			return fmt.Errorf("chương %d: %w", c.number, err)
		}
		if len(versions) == 0 {
			continue
		}
		var current *exportRecord
		for i := range versions {
			if versions[i].Revision == c.current {
				current = &versions[i]
			}
		}
		if current == nil {
			current = &versions[len(versions)-1]
		}
		recordPath := fmt.Sprintf("meta/chapter_records/%06d.json", c.number)
		if err := e.writeJSON(join(engineDir, recordPath), current); err != nil {
			return err
		}
		// Retained history keeps every immutable revision, not only the
		// current one; the latest record stays the authority.
		for _, v := range versions {
			rel := fmt.Sprintf("meta/chapter_versions/%06d/%d.json", c.number, v.Revision)
			if err := e.writeJSON(join(engineDir, rel), v); err != nil {
				return err
			}
			e.report.ChapterVersions++
		}
		// The workspace prose is the chapter_text document when present (that
		// is what Drafts.LoadChapterText reads); the accepted record's content
		// is the fallback for a chapter whose prose file was never written.
		prose := current.Content
		var stored *string
		err = e.pool.QueryRow(ctx, `
SELECT text_content FROM story_artifacts
WHERE project_id=$1 AND story_id=$2 AND kind='chapter_text' AND artifact_key=$3`,
			uuidArg(projectID), uuidArg(s.ID), fmt.Sprintf("%02d", c.number)).Scan(&stored)
		if err == nil && stored != nil {
			prose = *stored
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := e.writeFile(join(engineDir, fmt.Sprintf("chapters/%02d.md", c.number)), []byte(prose)); err != nil {
			return err
		}
	}
	return nil
}

// chapterVersions reads every immutable revision of one chapter as a complete
// domain record. The digest is verified against the content: emitting a record
// the file backend would reject is a data-integrity failure, not a warning.
func (e *exporter) chapterVersions(ctx context.Context, projectID, chapterID string, number int) ([]exportRecord, error) {
	rows, err := e.pool.Query(ctx, `
SELECT revision_no, content, sha256, origin, COALESCE(legacy_build,''), accepted_at,
       facts::text, style_delta::text
FROM chapter_versions WHERE project_id=$1 AND chapter_id=$2 ORDER BY revision_no`,
		uuidArg(projectID), uuidArg(chapterID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []exportRecord
	for rows.Next() {
		rec := exportRecord{Version: domain.ChapterRecordVersion, Chapter: number}
		var origin, facts, style string
		if err := rows.Scan(&rec.Revision, &rec.Content, &rec.ContentSHA256, &origin, &rec.Build,
			&rec.AcceptedAt, &facts, &style); err != nil {
			return nil, err
		}
		if rec.ContentSHA256 != domain.ChapterContentSHA256(rec.Content) {
			return nil, fmt.Errorf("revision %d: sha256 trong DB không khớp nội dung", rec.Revision)
		}
		switch origin {
		case string(domain.ChapterOriginGenerated), string(domain.ChapterOriginUser):
			rec.Origin = domain.ChapterOrigin(origin)
		case "import":
			// The file backend only knows generated/user revisions; an
			// imported acceptance was authored outside the engine.
			rec.Origin = domain.ChapterOriginUser
		default:
			return nil, fmt.Errorf("revision %d: origin lạ %q", rec.Revision, origin)
		}
		rec.Facts = nonEmptyJSON(json.RawMessage(facts))
		rec.StyleDelta = nonEmptyJSON(json.RawMessage(style))
		rec.AcceptedAt = rec.AcceptedAt.UTC()
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (e *exporter) exportCheckpoints(ctx context.Context, engineDir, projectID string, s exportStory) error {
	rows, err := e.pool.Query(ctx, `
SELECT seq, step, artifact_ref, sha256, occurred_at, payload::text
FROM checkpoints WHERE project_id=$1 AND story_id=$2 ORDER BY seq`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	var lines [][]byte
	for rows.Next() {
		var seq int64
		var step, artifact, digest, payload string
		var occurred time.Time
		if err := rows.Scan(&seq, &step, &artifact, &digest, &occurred, &payload); err != nil {
			return err
		}
		cp := domain.Checkpoint{Seq: seq, Step: step, Artifact: artifact, Digest: digest, OccurredAt: occurred.UTC()}
		var doc struct {
			Scope domain.Scope `json:"scope"`
		}
		_ = json.Unmarshal([]byte(payload), &doc)
		cp.Scope = doc.Scope
		data, err := json.Marshal(cp)
		if err != nil {
			return err
		}
		lines = append(lines, data)
		e.report.Checkpoints++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	return e.writeFile(join(engineDir, "meta/checkpoints.jsonl"), joinLines(lines))
}

// exportStreams writes every story_events log back to the file the Store
// reads. A stream with no file layout is still preserved under meta/streams/
// and reported, never dropped.
func (e *exporter) exportStreams(ctx context.Context, engineDir, projectID string, s exportStory) error {
	rows, err := e.pool.Query(ctx, `
SELECT stream, log_key, payload::text FROM story_events
WHERE project_id=$1 AND story_id=$2 ORDER BY stream, log_key, seq`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	type group struct {
		stream, logKey string
		lines          [][]byte
	}
	var groups []*group
	var cur *group
	for rows.Next() {
		var stream, logKey, payload string
		if err := rows.Scan(&stream, &logKey, &payload); err != nil {
			rows.Close()
			return err
		}
		if cur == nil || cur.stream != stream || cur.logKey != logKey {
			cur = &group{stream: stream, logKey: logKey}
			groups = append(groups, cur)
		}
		cur.lines = append(cur.lines, []byte(payload))
		e.report.Events++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, g := range groups {
		rel, ok := postgres.DocumentPath(postgres.DocumentRef{Class: postgres.DocClassStream, Kind: g.stream, Key: g.logKey})
		if !ok {
			rel = path.Join("meta", "streams", streamFileName(g.stream, g.logKey))
			e.report.Skipped = append(e.report.Skipped,
				fmt.Sprintf("stream %s/%s không có đường dẫn file — giữ ở %s", g.stream, g.logKey, rel))
		}
		if err := e.writeFile(join(engineDir, rel), joinLines(g.lines)); err != nil {
			return err
		}
	}
	return nil
}

func streamFileName(stream, logKey string) string {
	if logKey == "" {
		return stream + ".jsonl"
	}
	return stream + "-" + strings.ReplaceAll(logKey, "/", "_") + ".jsonl"
}

func (e *exporter) exportComments(ctx context.Context, storyDir, projectID string, s exportStory, deleted map[int]bool) error {
	rows, err := e.pool.Query(ctx, `
SELECT c.chapter_id::text, c.id::text, c.original_paragraph, c.original_quote, c.chips, c.body,
       COALESCE(c.legacy_author_name,''), c.created_at, c.status, c.sent_at, c.fixed_at,
       c.resolved_at, COALESCE(c.fixed_by,''), c.unfixed, c.moved, c.relinked, c.whole,
       c.new_quote, c.candidates::text, COALESCE(c.anchor::text,'')
FROM comments c WHERE c.project_id=$1 AND c.story_id=$2 AND c.deleted_at IS NULL ORDER BY c.created_at`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	byChapter := map[int][]map[string]any{}
	numbers := map[string]int{}
	for rows.Next() {
		var chapterID, id, quote, body, author, status, fixedBy, newQuote, candidates, anchor string
		var chips []string
		var paragraph *int
		var created time.Time
		var sent, fixed, resolved *time.Time
		var unfixed, moved, relinked, whole bool
		if err := rows.Scan(&chapterID, &id, &paragraph, &quote, &chips, &body, &author, &created,
			&status, &sent, &fixed, &resolved, &fixedBy, &unfixed, &moved, &relinked, &whole,
			&newQuote, &candidates, &anchor); err != nil {
			return err
		}
		number, ok := numbers[chapterID]
		if !ok {
			if err := e.pool.QueryRow(ctx, `SELECT number FROM chapters WHERE project_id=$1 AND id=$2`,
				uuidArg(projectID), uuidArg(chapterID)).Scan(&number); err != nil {
				return err
			}
			numbers[chapterID] = number
		}
		if deleted[number] {
			continue
		}
		entry := map[string]any{
			"id": id, "paragraph": paragraph, "quote": quote, "chips": chips, "text": body,
			"author": author, "createdAt": created, "status": status, "sentAt": sent,
			"fixedAt": fixed, "resolvedAt": resolved, "fixedBy": fixedBy, "unfixed": unfixed,
			"moved": moved, "relinked": relinked, "whole": whole, "newQuote": newQuote,
		}
		if candidates != "" {
			entry["candidates"] = json.RawMessage(candidates)
		}
		if anchor != "" && anchor != "{}" {
			entry["anchor"] = json.RawMessage(anchor)
		}
		byChapter[number] = append(byChapter[number], entry)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for number, entries := range byChapter {
		if err := e.writeJSON(join(storyDir, ".ainovel", "gop-y", fmt.Sprintf("%d.json", number)), entries); err != nil {
			return err
		}
	}
	return nil
}

// exportReviews writes every review round back where the file backend reads
// it: the latest round of each scope is reviews/NN[-global].json and the older
// rounds are the archive files reviews/rounds/NN[-global]-rK.json.
func (e *exporter) exportReviews(ctx context.Context, engineDir, projectID string, s exportStory) error {
	rows, err := e.pool.Query(ctx, `
SELECT scope_key, round, payload::text FROM reviews
WHERE project_id=$1 AND story_id=$2 AND invalidated_at IS NULL ORDER BY scope_key, round`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	type round struct {
		number  int
		payload []byte
		entry   domain.ReviewEntry
	}
	groups := map[string][]round{}
	var order []string
	for rows.Next() {
		var scopeKey, payload string
		var number int
		if err := rows.Scan(&scopeKey, &number, &payload); err != nil {
			rows.Close()
			return err
		}
		var entry domain.ReviewEntry
		if err := json.Unmarshal([]byte(payload), &entry); err != nil {
			rows.Close()
			return fmt.Errorf("review %s round %d: %w", scopeKey, number, err)
		}
		if _, seen := groups[scopeKey]; !seen {
			order = append(order, scopeKey)
		}
		groups[scopeKey] = append(groups[scopeKey], round{number: number, payload: []byte(payload), entry: entry})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, scopeKey := range order {
		rounds := groups[scopeKey]
		latest := rounds[len(rounds)-1]
		name := reviewFileName(latest.entry)
		if name == "" {
			return fmt.Errorf("review %s không có đường dẫn file — dữ liệu sẽ mất", scopeKey)
		}
		if err := e.writeFile(join(engineDir, "reviews", name), latest.payload); err != nil {
			return err
		}
		base := strings.TrimSuffix(name, ".json")
		for i, r := range rounds[:len(rounds)-1] {
			rel := path.Join("reviews", "rounds", fmt.Sprintf("%s-r%d.json", base, i+1))
			if err := e.writeFile(join(engineDir, rel), r.payload); err != nil {
				return err
			}
		}
	}
	return nil
}

// reviewFileName is the name the file backend uses for one review scope:
// reviews/NN.json for chapter/arc scopes and reviews/NN-global.json for a
// global review, including an unanchored global review at chapter 0.
func reviewFileName(entry domain.ReviewEntry) string {
	return store.ReviewScopeOf(entry.Scope).FileName(entry.Chapter)
}

// exportArtifacts writes every artifact row at the path its canonical identity
// maps to; unmapped import-only artifacts keep the source-relative key they
// were stored under.
func (e *exporter) exportArtifacts(ctx context.Context, engineDir, storyDir, projectID string, s exportStory, deleted map[int]bool) error {
	rows, err := e.pool.Query(ctx, `
SELECT kind, artifact_key, COALESCE(payload::text,''), COALESCE(text_content,'')
FROM story_artifacts WHERE project_id=$1 AND story_id=$2 ORDER BY kind, artifact_key`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, key, payload, text string
		if err := rows.Scan(&kind, &key, &payload, &text); err != nil {
			return err
		}
		if (kind == "chapter_text" || kind == "chapter_plan") && deleted[chapterNumber(key)] {
			continue
		}
		content := payload
		if text != "" {
			content = text
		}
		rel, ok := postgres.DocumentPath(postgres.DocumentRef{Class: postgres.DocClassArtifact, Kind: kind, Key: key})
		if ok {
			if err := e.writeFile(join(engineDir, rel), []byte(content)); err != nil {
				return err
			}
			continue
		}
		if !safeRelativePath(key) {
			return fmt.Errorf("artifact %s/%q không phải đường dẫn tương đối an toàn — dữ liệu sẽ mất", kind, key)
		}
		target := engineDir
		if strings.HasPrefix(key, ".ainovel/") || strings.HasPrefix(key, "audio/") ||
			strings.HasPrefix(key, "xuat-ban/") || strings.HasPrefix(key, "exports/") {
			target = storyDir
		}
		if err := e.writeFile(join(target, key), []byte(content)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (e *exporter) exportAudio(ctx context.Context, storyDir, projectID string, s exportStory, dirs Dirs) error {
	var settings []byte
	err := e.pool.QueryRow(ctx, `SELECT payload::text FROM audio_settings WHERE project_id=$1 AND story_id=$2`,
		uuidArg(projectID), uuidArg(s.ID)).Scan(&settings)
	if err == nil {
		if err := e.writeFile(join(storyDir, dirs.Audio, "settings.json"), settings); err != nil {
			return err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	castRows, err := e.pool.Query(ctx, `SELECT payload::text FROM audio_cast WHERE project_id=$1 AND story_id=$2 ORDER BY role_key`,
		uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	var speakers []json.RawMessage
	for castRows.Next() {
		var payload string
		if err := castRows.Scan(&payload); err != nil {
			castRows.Close()
			return err
		}
		speakers = append(speakers, json.RawMessage(payload))
	}
	castRows.Close()
	if err := castRows.Err(); err != nil {
		return err
	}
	if len(speakers) > 0 {
		if err := e.writeJSON(join(storyDir, dirs.Audio, "vai.json"), map[string]any{"speakers": speakers}); err != nil {
			return err
		}
	}
	scriptRows, err := e.pool.Query(ctx, `
SELECT s.number, sc.payload::text FROM audio_scripts sc
JOIN chapters s ON s.project_id = sc.project_id AND s.id = sc.chapter_id
WHERE sc.project_id=$1 AND sc.story_id=$2 AND s.deleted_at IS NULL ORDER BY s.number`, uuidArg(projectID), uuidArg(s.ID))
	if err != nil {
		return err
	}
	defer scriptRows.Close()
	for scriptRows.Next() {
		var number int
		var payload string
		if err := scriptRows.Scan(&number, &payload); err != nil {
			return err
		}
		var head struct {
			Volume int `json:"volume"`
		}
		_ = json.Unmarshal([]byte(payload), &head)
		rel := path.Join(dirs.Audio, "ban-doc", fmt.Sprintf("tap-%02d", head.Volume), fmt.Sprintf("%02d.json", number))
		if err := e.writeFile(join(storyDir, rel), []byte(payload)); err != nil {
			return err
		}
	}
	return scriptRows.Err()
}

// exportAccounts writes the identity domain (users, credentials, memberships
// and audit) as the web-accounts.json the file backend already reads.
func (e *exporter) exportAccounts(ctx context.Context) error {
	file := AccountFile{Version: 4, Sessions: []AccountToken{}, Pairings: []AccountToken{}, Audit: []AccountAudit{}}
	rows, err := e.pool.Query(ctx, `
SELECT u.id::text, u.username::text, u.display_name, u.global_role, u.status,
       u.must_change_password, u.onboarded_at, u.disabled_at, u.disabled_by::text,
       u.disabled_reason, u.deleted_at, u.created_at, u.updated_at,
       COALESCE(c.password_hash,''), c.password_changed_at
FROM users u LEFT JOIN user_credentials c ON c.user_id = u.id
ORDER BY u.username`)
	if err != nil {
		return err
	}
	byID := map[string]string{}
	type pendingUser struct {
		user     AccountUser
		id       string
		disabled string
	}
	var pending []pendingUser
	for rows.Next() {
		var u AccountUser
		var id, role, status, disabledBy, passwordHash string
		var disabledByID *string
		var disabledReason string
		var mustChange bool
		var onboarded, disabledAt, deletedAt, passwordChanged *time.Time
		if err := rows.Scan(&id, &u.Username, &u.Name, &role, &status, &mustChange,
			&onboarded, &disabledAt, &disabledByID, &disabledReason, &deletedAt,
			&u.CreatedAt, &u.UpdatedAt, &passwordHash, &passwordChanged); err != nil {
			rows.Close()
			return err
		}
		u.Role = role
		u.Disabled = status == "locked"
		u.MustChangePassword = mustChange
		u.OnboardedAt = onboarded
		u.DisabledAt = disabledAt
		u.DisabledReason = disabledReason
		u.DeletedAt = deletedAt
		u.PasswordHash = passwordHash
		u.PasswordChangedAt = passwordChanged
		u.Projects = map[string]string{}
		byID[id] = u.Username
		if disabledByID != nil {
			disabledBy = *disabledByID
		}
		pending = append(pending, pendingUser{user: u, id: id, disabled: disabledBy})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	index := map[string]*pendingUser{}
	for i := range pending {
		index[pending[i].user.Username] = &pending[i]
	}
	for _, p := range pending {
		if p.disabled != "" {
			if username, ok := byID[p.disabled]; ok {
				index[p.user.Username].user.DisabledBy = username
			}
		}
	}
	members, err := e.pool.Query(ctx, `
SELECT m.user_id::text, p.slug, m.role FROM project_members m
JOIN projects p ON p.id = m.project_id
WHERE m.removed_at IS NULL AND p.deleted_at IS NULL
ORDER BY p.slug`)
	if err != nil {
		return err
	}
	for members.Next() {
		var userID, slug, role string
		if err := members.Scan(&userID, &slug, &role); err != nil {
			members.Close()
			return err
		}
		if p := index[byID[userID]]; p != nil {
			p.user.Projects[slug] = role
		}
	}
	members.Close()
	if err := members.Err(); err != nil {
		return err
	}
	recoveries, err := e.pool.Query(ctx, `
SELECT user_id::text, code_hash, COALESCE(mask,''), created_at, used_at
FROM recovery_codes WHERE revoked_at IS NULL ORDER BY created_at`)
	if err != nil {
		return err
	}
	for recoveries.Next() {
		var userID string
		var r AccountRecovery
		if err := recoveries.Scan(&userID, &r.CodeHash, &r.Mask, &r.CreatedAt, &r.UsedAt); err != nil {
			recoveries.Close()
			return err
		}
		if p := index[byID[userID]]; p != nil {
			rec := r
			p.user.Recovery = &rec
		}
	}
	recoveries.Close()
	if err := recoveries.Err(); err != nil {
		return err
	}
	temps, err := e.pool.Query(ctx, `
SELECT t.user_id::text, t.password_hash, t.expires_at, t.used_at, t.created_at,
       COALESCE(u.username::text,'')
FROM temporary_passwords t LEFT JOIN users u ON u.id = t.created_by
WHERE t.revoked_at IS NULL ORDER BY t.created_at`)
	if err != nil {
		return err
	}
	for temps.Next() {
		var userID string
		var t AccountTempPassword
		if err := temps.Scan(&userID, &t.CodeHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt, &t.CreatedBy); err != nil {
			temps.Close()
			return err
		}
		if p := index[byID[userID]]; p != nil {
			temp := t
			p.user.TempPassword = &temp
		}
	}
	temps.Close()
	if err := temps.Err(); err != nil {
		return err
	}
	audits, err := e.pool.Query(ctx, `
SELECT a.id::text, a.occurred_at, COALESCE(u.username::text,''), a.action, a.target_kind, a.target_id, a.details::text
FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_user_id
ORDER BY a.occurred_at, a.id`)
	if err != nil {
		return err
	}
	for audits.Next() {
		var entry AccountAudit
		var actor, details string
		if err := audits.Scan(&entry.ID, &entry.At, &actor, &entry.Action, &entry.Type, &entry.Target, &details); err != nil {
			audits.Close()
			return err
		}
		var raw struct {
			Entry *AccountAudit `json:"entry"`
		}
		if json.Unmarshal([]byte(details), &raw) == nil && raw.Entry != nil && raw.Entry.At.Equal(entry.At) {
			file.Audit = append(file.Audit, *raw.Entry)
			continue
		}
		if actor != "" {
			payload, _ := json.Marshal(map[string]string{"username": actor})
			entry.Actor = payload
		}
		file.Audit = append(file.Audit, entry)
	}
	audits.Close()
	if err := audits.Err(); err != nil {
		return err
	}
	for _, p := range pending {
		file.Users = append(file.Users, p.user)
	}
	if len(file.Users) == 0 {
		return nil
	}
	e.report.Accounts = len(file.Users)
	return e.writeJSON(PathAccountsExport, file)
}

// exportBlobs writes every ready blob back into the library. Imported objects
// keep the source path recorded in the ledger; objects created after the
// cutover get a deterministic path derived from their key. A blob that cannot
// be fetched fails the export — a rollback with a hole is not a rollback.
func (e *exporter) exportBlobs(ctx context.Context) error {
	if e.opts.Blobs == nil {
		return errors.New("thiếu BlobStore để xuất blob")
	}
	ledger, err := e.ledgerBlobPaths(ctx)
	if err != nil {
		return err
	}
	rows, err := e.pool.Query(ctx, `
SELECT id::text, COALESCE(project_id::text,''), object_key, sha256, content_type
FROM blobs WHERE state='ready' ORDER BY object_key`)
	if err != nil {
		return err
	}
	type blobRow struct{ path, key, sha, contentType string }
	var blobs []blobRow
	for rows.Next() {
		var id, projectID, key, sha, contentType string
		if err := rows.Scan(&id, &projectID, &key, &sha, &contentType); err != nil {
			rows.Close()
			return err
		}
		rel, action := e.blobPath(id, projectID, key, contentType, ledger)
		switch action {
		case blobSkip:
			continue
		case blobFallback:
			rel = path.Join(".ainovel", "blobs", sha+blobExt(key))
			e.report.Skipped = append(e.report.Skipped,
				fmt.Sprintf("blob %s ngoài thư viện — giữ ở %s", key, rel))
		}
		blobs = append(blobs, blobRow{path: rel, key: key, sha: sha, contentType: contentType})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, b := range blobs {
		rc, _, err := e.opts.Blobs.Open(ctx, b.key, blobstore.FullRange)
		if err != nil {
			return fmt.Errorf("blob %s (%s): không tải được: %w", b.path, b.key, err)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return fmt.Errorf("blob %s: đọc: %w", b.path, readErr)
		}
		if RawDigest(data) != b.sha {
			return fmt.Errorf("blob %s: hash tải khác DB", b.path)
		}
		if err := e.writeFile(b.path, data); err != nil {
			return err
		}
		e.report.BlobsWritten++
		e.report.BytesWritten += int64(len(data))
	}
	return nil
}

// ledgerBlobPaths maps blob row ids to the source-relative path the import
// ledger recorded for them.
func (e *exporter) ledgerBlobPaths(ctx context.Context) (map[string]string, error) {
	rows, err := e.pool.Query(ctx, `
SELECT DISTINCT ON (target_id) target_id, relative_path
FROM import_items WHERE source_id=$1 AND disposition='blob'
ORDER BY target_id, relative_path`, uuidArg(e.opts.SourceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, rel string
		if err := rows.Scan(&id, &rel); err != nil {
			return nil, err
		}
		out[id] = rel
	}
	return out, rows.Err()
}

// blobAction says how one blob row is written.
type blobAction int

const (
	blobAtPath blobAction = iota
	// blobSkip drops the object with the deleted owner instead of
	// resurrecting a soft-deleted project/story.
	blobSkip
	// blobFallback keeps the bytes outside the library (user avatars,
	// staging debris) so nothing is lost.
	blobFallback
)

// blobPath resolves the file location of one blob: the recorded source path
// for imported objects, a key-derived path inside the owning project/story for
// objects created later.
func (e *exporter) blobPath(id, projectID, objectKey, contentType string, ledger map[string]string) (string, blobAction) {
	if rel, ok := ledger[id]; ok && rel != "" {
		if e.ownerDeleted(rel) {
			e.report.Skipped = append(e.report.Skipped,
				"blob "+rel+" thuộc dự án/truyện đã xoá mềm — không xuất")
			return "", blobSkip
		}
		return rel, blobAtPath
	}
	if projectID == "" {
		return "", blobFallback
	}
	proj, ok := e.project[projectID]
	if !ok {
		return "", blobSkip
	}
	obj, err := blobkey.Parse(objectKey)
	if err != nil {
		return path.Join(proj.dir(), e.dirs.Assets, "blobs", blobName(objectKey)), blobAtPath
	}
	ext := obj.Ext
	if ext == "" {
		ext = extForContentType(contentType)
	}
	name := obj.SHA256 + ext
	if obj.StoryID != "" {
		story, ok := e.story[obj.StoryID]
		if !ok {
			return "", blobSkip
		}
		storyDir := join(proj.dir(), e.dirs.Stories, story.Slug)
		if proj.Slug == "_" {
			storyDir = story.Slug
		}
		switch obj.Kind {
		case blobkey.KindExport:
			return join(storyDir, e.dirs.Exports, firstNonEmpty(obj.Name, name)), blobAtPath
		default:
			return join(storyDir, e.dirs.Audio, "import", name), blobAtPath
		}
	}
	switch obj.Kind {
	case blobkey.KindExport:
		return join(proj.dir(), e.dirs.Exports, firstNonEmpty(obj.Name, name)), blobAtPath
	default:
		return join(proj.dir(), e.dirs.Assets, "blobs", name), blobAtPath
	}
}

func blobName(key string) string {
	name := path.Base(key)
	if name == "." || name == "/" || name == "" {
		return "blob"
	}
	return name
}

func blobExt(key string) string {
	return path.Ext(key)
}

func extForContentType(contentType string) string {
	for ext, ct := range map[string]string{
		".json": "application/json", ".md": "text/markdown; charset=utf-8",
		".txt": "text/plain; charset=utf-8", ".mp3": "audio/mpeg",
		".wav": "audio/wav", ".m4b": "audio/mp4", ".epub": "application/epub+zip",
		".png": "image/png", ".jpg": "image/jpeg", ".webp": "image/webp",
	} {
		if ct == contentType {
			return ext
		}
	}
	return ""
}

func (e *exporter) revoke(ctx context.Context) error {
	tag, err := e.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now(), updated_at=now(), version=version+1 WHERE revoked_at IS NULL`)
	if err != nil {
		return fmt.Errorf("thu hồi phiên: %w", err)
	}
	e.report.SessionsRevoked = int(tag.RowsAffected())
	tag, err = e.pool.Exec(ctx, `UPDATE device_pairings SET revoked_at=now(), updated_at=now(), version=version+1 WHERE revoked_at IS NULL`)
	if err != nil {
		return fmt.Errorf("thu hồi pairing: %w", err)
	}
	e.report.PairingsRevoked = int(tag.RowsAffected())
	return nil
}

func (e *exporter) writeJSON(rel string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return e.writeFile(rel, append(data, '\n'))
}

// writeFile writes one path exactly once. A second write to the same path is
// an error: it would silently overwrite another domain's file (the class of
// bug that produced project/project/... paths).
func (e *exporter) writeFile(rel string, data []byte) error {
	if prev, ok := e.written[rel]; ok {
		if string(prev) == string(data) {
			return nil
		}
		return fmt.Errorf("đường dẫn xuất %s bị ghi hai lần với nội dung khác nhau", rel)
	}
	full := filepath.Join(e.opts.Destination, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return err
	}
	e.written[rel] = data
	e.report.FilesWritten++
	e.report.BytesWritten += int64(len(data))
	return nil
}

// safeRelativePath reports that a recorded source path can be joined under an
// export directory without escaping it. Extensionless names are legitimate
// story files, so the decision is purely about path safety.
func safeRelativePath(rel string) bool {
	if rel == "" || strings.Contains(rel, "\\") || path.IsAbs(rel) {
		return false
	}
	if path.Clean(rel) != rel {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// chapterNumber parses a chapter artifact key ("03") back to its number;
// 0 means the key is not chapter-scoped.
func chapterNumber(key string) int {
	n, err := strconv.Atoi(key)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func joinLines(lines [][]byte) []byte {
	var out []byte
	for _, line := range lines {
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

func join(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return path.Join(out...)
}
