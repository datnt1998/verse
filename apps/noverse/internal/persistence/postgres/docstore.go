package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// DocStore renders the story's document surface (states, artifacts, streams,
// checkpoints, agent sessions, chapter draft/text) through the same key/value
// contract the file backend implements over files. The mapping lives here so
// no engine package knows a table name, and every path stays inside one
// project+story scope.
type DocStore struct {
	pool  *pgxpool.Pool
	fence FenceCheck
}

// NewDocStore builds the document surface on the shared pool.
func NewDocStore(p *Pool) *DocStore { return &DocStore{pool: p.Pool()} }

// SetFence wires the write-transaction fence (the story lease). Every write
// region starts by re-reading the lease, so a writer that lost it cannot commit
// even when the model returns late.
func (d *DocStore) SetFence(f FenceCheck) { d.fence = f }

// docClass names the table family a file path maps onto.
type docClass int

const (
	docState docClass = iota
	docOutline
	docArtifact
	docStream
	docCheckpoints
	docAgentSession
	docChapterDraft
	// docChapterRecord is a read-only derived view of the accepted-revision
	// table: the checkpoint digest readers open the record by its file-backend
	// path, but the record itself is only ever written through the revision
	// invariants (pgChapterRecords), never as a document.
	docChapterRecord
)

// docRef is one resolved path: which table, which kind/stream, which key.
type docRef struct {
	class docClass
	// name is the story_state kind, outlines kind, story_artifacts kind or
	// story_events stream.
	name string
	// key is the artifact_key, event log_key or agent session task key.
	key string
	// agent is set for agent_sessions rows.
	agent string
	// chapter is set for chapter-scoped paths.
	chapter int
	// text stores/reads plain text instead of a JSON payload.
	text bool
}

var (
	docSnapshotRe = regexp.MustCompile(`^meta/snapshots/v(\d+)a(\d+)\.json$`)
	docSummaryRe  = regexp.MustCompile(`^summaries/(.+)\.json$`)
	docDraftRe    = regexp.MustCompile(`^drafts/(\d+)\.draft\.md$`)
	docPlanRe     = regexp.MustCompile(`^drafts/(\d+)\.plan\.json$`)
	docChapterRe  = regexp.MustCompile(`^chapters/(\d+)\.md$`)
	docTaskLogRe  = regexp.MustCompile(`^meta/runtime/tasks/(.+)\.log$`)
	docSessionRe  = regexp.MustCompile(`^meta/sessions/agents/(.+)\.jsonl$`)
	// Accepted chapter records are typed rows; the path is only readable.
	docChapterRecordRe = regexp.MustCompile(`^meta/chapter_records/(\d+)\.json$`)
)

// stateKinds maps meta/*.json singletons onto story_state kinds.
var stateKinds = map[string]string{
	"meta/book.json":                        "book",
	"meta/progress.json":                    "progress",
	"meta/run.json":                         "run_meta",
	"meta/user_rules.json":                  "user_rules",
	"meta/user_rules_pin.json":              "user_rules_pin",
	"meta/user_rules_cache.json":            "user_rules_cache",
	"meta/user_rules_runtime.json":          "user_rules_runtime",
	"meta/lesson_rules.json":                "lesson_rules",
	"meta/ledger.json":                      "ledger",
	"meta/pending_commit.json":              "pending_commit",
	"meta/pending_revision.json":            "pending_revision",
	"meta/last_commit.json":                 "last_commit",
	"meta/last_review.json":                 "last_review",
	"meta/usage.json":                       "usage",
	"meta/simulation_profile.json":          "simulation",
	"meta/project_format.json":              "project_format",
	"meta/author_revision_style.json":       "author_revision_style",
	"meta/foundation_audit.json":            "foundation_audit",
	"meta/style_rules.json":                 "style_rules",
	"meta/cast_ledger.json":                 "cast",
	"meta/outline_feedback_resolution.json": "outline_feedback_resolution",
	"world_rules.json":                      "world_rules",
	"foreshadow_ledger.json":                "foreshadow",
	"relationship_state.json":               "relationships",
	"address_terms.json":                    "address_terms",
	"characters.json":                       "characters",
	// Legacy append arrays the file backend migrates from (appendLog).
	"timeline.json":           "timeline",
	"meta/state_changes.json": "state_changes",
}

// outlineDoc describes one outlines row: JSON payload, or plain text (premise).
type outlineDoc struct {
	kind string
	text bool
}

var outlineDocs = map[string]outlineDoc{
	"outline.json":              {"flat", false},
	"layered_outline.json":      {"layered", false},
	"meta/compass.json":         {"compass", false},
	"meta/story_retention.json": {"retention", false},
	"premise.md":                {"premise", true},
}

// streamDocs maps append logs onto story_events streams.
var streamDocs = map[string]string{
	"meta/decisions.jsonl":        "decisions",
	"meta/rule_violations.jsonl":  "rule_violations",
	"timeline.jsonl":              "timeline",
	"meta/state_changes.jsonl":    "state_changes",
	"relationship_log.jsonl":      "relationships",
	"meta/rewrites.jsonl":         "rewrite_history",
	"meta/outline_feedback.jsonl": "outline_feedback",
	"meta/runtime/queue.jsonl":    "runtime_queue",
}

// isMarkdownView reports the derived human-readable sidecars. They are stored
// so export/debugging see the same bytes as file mode; none is a data source.
func isMarkdownView(rel string) bool {
	switch rel {
	case "book.md", "characters.md", "world_rules.md", "timeline.md",
		"foreshadow_ledger.md", "relationship_state.md", "outline.md",
		"layered_outline.md", "nao/index.md":
		return true
	}
	return strings.HasPrefix(rel, "nao/nhan-vat/") && strings.HasSuffix(rel, ".md")
}

// resolveDoc maps a file-backend relative path onto its document row. Unknown
// paths are an error: silently dropping a write would lose a domain fact.
func resolveDoc(rel string) (docRef, error) {
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "./")
	if kind, ok := stateKinds[rel]; ok {
		return docRef{class: docState, name: kind}, nil
	}
	if doc, ok := outlineDocs[rel]; ok {
		return docRef{class: docOutline, name: doc.kind, text: doc.text}, nil
	}
	if stream, ok := streamDocs[rel]; ok {
		return docRef{class: docStream, name: stream}, nil
	}
	switch rel {
	case "meta/checkpoints.jsonl":
		return docRef{class: docCheckpoints, name: "checkpoints"}, nil
	case "meta/sessions/cocreate.jsonl":
		return docRef{class: docStream, name: "sessions", key: "cocreate"}, nil
	}
	if m := docSessionRe.FindStringSubmatch(rel); m != nil {
		agent, task := splitAgentTask(m[1])
		return docRef{class: docAgentSession, name: "agent_sessions", agent: agent, key: task}, nil
	}
	if m := docTaskLogRe.FindStringSubmatch(rel); m != nil {
		return docRef{class: docStream, name: "runtime_log", key: m[1]}, nil
	}
	if m := docDraftRe.FindStringSubmatch(rel); m != nil {
		chapter, _ := strconv.Atoi(m[1])
		return docRef{class: docChapterDraft, name: "chapter_draft", chapter: chapter, text: true}, nil
	}
	if m := docChapterRe.FindStringSubmatch(rel); m != nil {
		chapter, _ := strconv.Atoi(m[1])
		return docRef{class: docArtifact, name: "chapter_text", key: fmt.Sprintf("%02d", chapter), chapter: chapter, text: true}, nil
	}
	if m := docPlanRe.FindStringSubmatch(rel); m != nil {
		chapter, _ := strconv.Atoi(m[1])
		return docRef{class: docArtifact, name: "chapter_plan", key: fmt.Sprintf("%02d", chapter), chapter: chapter}, nil
	}
	if m := docSummaryRe.FindStringSubmatch(rel); m != nil {
		return docRef{class: docArtifact, name: "summary", key: m[1]}, nil
	}
	if m := docSnapshotRe.FindStringSubmatch(rel); m != nil {
		return docRef{class: docArtifact, name: "snapshot", key: fmt.Sprintf("v%sa%s", m[1], m[2])}, nil
	}
	if m := docChapterRecordRe.FindStringSubmatch(rel); m != nil {
		chapter, _ := strconv.Atoi(m[1])
		return docRef{class: docChapterRecord, chapter: chapter}, nil
	}
	if isMarkdownView(rel) {
		return docRef{class: docArtifact, name: "markdown_view", key: rel, text: true}, nil
	}
	return docRef{}, fmt.Errorf("postgres document backend has no mapping for %q: %w", rel, errUnmappedPath)
}

var errUnmappedPath = fmt.Errorf("unmapped document path")

// errChapterRecordReadOnly refuses a document write/delete of an accepted
// chapter record: revisions are append-only through the typed revision store,
// and a document write would bypass its invariants.
var errChapterRecordReadOnly = fmt.Errorf("chapter records are append-only through the revision store, not documents: %w", errUnmappedPath)

// DocClass names the row family a file-backend document path maps onto.
type DocClass string

const (
	DocClassState         DocClass = "state"
	DocClassOutline       DocClass = "outline"
	DocClassArtifact      DocClass = "artifact"
	DocClassStream        DocClass = "stream"
	DocClassCheckpoints   DocClass = "checkpoints"
	DocClassAgentSession  DocClass = "agent_session"
	DocClassChapterDraft  DocClass = "chapter_draft"
	DocClassChapterRecord DocClass = "chapter_record"
)

// DocumentRef is the canonical row identity of one file-backend relative path.
// It is the exported view of resolveDoc so offline tooling (import/export)
// writes and reads the exact keys the Store uses instead of inventing a
// second convention.
type DocumentRef struct {
	Class DocClass
	// Kind is the story_state/outlines kind, story_artifacts kind or
	// story_events stream name.
	Kind string
	// Key is the artifact_key, stream log_key or agent session task key.
	Key string
	// Agent is set for agent_sessions rows.
	Agent string
	// Chapter is set for chapter-scoped paths.
	Chapter int
	// Text reports that the path holds plain text (Markdown, premise, chapter
	// prose) rather than a JSON payload.
	Text bool
}

// ResolveDocument maps a story-relative file path onto the document row the
// Store reads and writes. Unmapped paths are an error, never a silent skip.
func ResolveDocument(rel string) (DocumentRef, error) {
	ref, err := resolveDoc(rel)
	if err != nil {
		return DocumentRef{}, err
	}
	out := DocumentRef{Kind: ref.name, Key: ref.key, Agent: ref.agent, Chapter: ref.chapter, Text: ref.text}
	switch ref.class {
	case docState:
		out.Class = DocClassState
	case docOutline:
		out.Class = DocClassOutline
	case docArtifact:
		out.Class = DocClassArtifact
	case docStream:
		out.Class = DocClassStream
	case docCheckpoints:
		out.Class = DocClassCheckpoints
	case docAgentSession:
		out.Class = DocClassAgentSession
	case docChapterDraft:
		out.Class = DocClassChapterDraft
		out.Key = fmt.Sprintf("%02d", ref.chapter)
	case docChapterRecord:
		out.Class = DocClassChapterRecord
		out.Key = fmt.Sprintf("%06d", ref.chapter)
	default:
		return DocumentRef{}, fmt.Errorf("lớp tài liệu %v: %w", ref.class, errUnmappedPath)
	}
	return out, nil
}

// stateDocPaths is the inverse of stateKinds: kind → the file path the Store
// reads. Two paths never map to one kind; the round-trip test
// TestDocumentPathIsInverseOfResolve pins that property.
var stateDocPaths = func() map[string]string {
	out := make(map[string]string, len(stateKinds))
	for rel, kind := range stateKinds {
		if prev, ok := out[kind]; ok && prev < rel {
			continue
		}
		out[kind] = rel
	}
	return out
}()

// outlineDocPaths is the inverse of outlineDocs.
var outlineDocPaths = func() map[string]string {
	out := make(map[string]string, len(outlineDocs))
	for rel, doc := range outlineDocs {
		out[doc.kind] = rel
	}
	return out
}()

// streamDocPaths is the inverse of streamDocs for keyless streams.
var streamDocPaths = func() map[string]string {
	out := make(map[string]string, len(streamDocs))
	for rel, stream := range streamDocs {
		out[stream] = rel
	}
	return out
}()

// DocumentPath is the inverse of ResolveDocument: it returns the file-backend
// path (story-relative) for a canonical document identity, and only for
// identities the forward parser could itself produce — a key that would map
// back to a different path is rejected rather than written to a wrong file.
// ok=false means the identity has no file layout (an import-only artifact or
// an unmappable soft key).
func DocumentPath(ref DocumentRef) (string, bool) {
	switch ref.Class {
	case DocClassState:
		rel, ok := stateDocPaths[ref.Kind]
		return rel, ok
	case DocClassOutline:
		rel, ok := outlineDocPaths[ref.Kind]
		return rel, ok
	case DocClassStream:
		if rel, ok := streamDocPaths[ref.Kind]; ok {
			return rel, ref.Key == ""
		}
		switch ref.Kind {
		case "sessions":
			if ref.Key == "cocreate" {
				return "meta/sessions/cocreate.jsonl", true
			}
		case "runtime_log":
			if docTaskLogRe.MatchString("meta/runtime/tasks/"+ref.Key+".log") && ref.Key != "" {
				return "meta/runtime/tasks/" + ref.Key + ".log", true
			}
		}
		return "", false
	case DocClassCheckpoints:
		return "meta/checkpoints.jsonl", true
	case DocClassAgentSession:
		// splitAgentTask takes the last hyphen, so a hyphen in the task would
		// come back as a different agent/task pair; a slash is fine.
		if ref.Agent == "" || ref.Key == "" || strings.Contains(ref.Key, "-") {
			return "", false
		}
		return "meta/sessions/agents/" + ref.Agent + "-" + ref.Key + ".jsonl", true
	case DocClassChapterDraft:
		if ref.Chapter <= 0 {
			return "", false
		}
		return fmt.Sprintf("drafts/%02d.draft.md", ref.Chapter), true
	case DocClassChapterRecord:
		if ref.Chapter <= 0 {
			return "", false
		}
		return fmt.Sprintf("meta/chapter_records/%06d.json", ref.Chapter), true
	case DocClassArtifact:
		switch ref.Kind {
		case "chapter_plan", "chapter_text":
			if !canonicalChapterKey(ref.Key) {
				return "", false
			}
			if ref.Kind == "chapter_plan" {
				return fmt.Sprintf("drafts/%s.plan.json", ref.Key), true
			}
			return fmt.Sprintf("chapters/%s.md", ref.Key), true
		case "summary":
			if !docSummaryRe.MatchString("summaries/" + ref.Key + ".json") {
				return "", false
			}
			return "summaries/" + ref.Key + ".json", true
		case "snapshot":
			if !docSnapshotRe.MatchString("meta/snapshots/" + ref.Key + ".json") {
				return "", false
			}
			return "meta/snapshots/" + ref.Key + ".json", true
		case "markdown_view":
			return ref.Key, ref.Key != "" && isMarkdownView(ref.Key)
		}
		return "", false
	}
	return "", false
}

// canonicalChapterKey reports that an artifact key is the canonical %02d form
// resolveDoc produces for chapter-numbered artifacts.
func canonicalChapterKey(key string) bool {
	n, err := strconv.Atoi(key)
	return err == nil && n > 0 && fmt.Sprintf("%02d", n) == key
}

// splitAgentTask separates "writer-ch01" into agent "writer" and task "ch01".
// Agent names may contain hyphens, so the last hyphen wins (the suffix is
// either "chNN" or a zero-padded run sequence).
func splitAgentTask(stem string) (agent, task string) {
	if i := strings.LastIndex(stem, "-"); i > 0 {
		return stem[:i], stem[i+1:]
	}
	return stem, stem
}

// ── ops: the store.ioOps implementation ─────────────────────────────────────

// docOps implements store.IOOps over DocStore. Every store.IO owns one
// instance and serializes access through its own mutex, so the ambient
// transaction below needs no further locking: a write region (IO.lockWrite) pins
// one transaction for the whole region, which is what makes one substore
// method atomic — exactly the file backend's per-file lock guarantee.
type docOps struct {
	ctx   context.Context
	ds    *DocStore
	scope ScopeRef

	tx    pgx.Tx
	txErr error
	// bound marks ops attached to a caller-owned transaction: beginWrite and
	// endWrite are no-ops because the owner commits.
	bound bool
}

func (d *DocStore) ops(ctx context.Context, scope ScopeRef) *docOps {
	return &docOps{ctx: ctx, ds: d, scope: scope}
}

// BoundOps returns ops pinned to an already-open transaction; the caller owns
// commit/rollback. Exposed so the backend can run compound operations in one
// transaction over a temp substore set.
func (d *DocStore) BoundOps(ctx context.Context, scope ScopeRef, tx pgx.Tx) *docOps {
	return &docOps{ctx: ctx, ds: d, scope: scope, tx: tx, bound: true}
}

// runTx executes fn on the ambient transaction, or in a fresh one when the
// caller is not inside a write region.
func (o *docOps) runTx(fn func(tx pgx.Tx) error) error {
	if o.txErr != nil {
		return o.txErr
	}
	if o.tx != nil {
		if err := fn(o.tx); err != nil {
			return err
		}
		if st := o.tx.Conn().PgConn().TxStatus(); st != 'T' {
			return fmt.Errorf("document transaction aborted (txstatus=%c)", st)
		}
		return nil
	}
	return RunInTx(o.ctx, o.ds.pool, fn)
}

// maintenance runs fn with the append-only trigger escape enabled in the same
// transaction. Only the document layer rewrites/deletes streams it owns (tail
// repair, reset), never a domain handler.
func (o *docOps) maintenance(fn func(tx pgx.Tx) error) error {
	return o.runTx(func(tx pgx.Tx) error {
		if _, err := tx.Exec(o.ctx, `SET LOCAL noverse.allow_immutable_write = 'on'`); err != nil {
			return sanitizeErr(err)
		}
		return fn(tx)
	})
}

// BeginWrite opens the region's transaction behind the fence. A failure is
// held for the region only: every op in it fails, and the next region retries.
func (o *docOps) BeginWrite() {
	if o.bound {
		return
	}
	o.txErr = nil
	tx, err := o.ds.pool.Begin(o.ctx)
	if err != nil {
		o.txErr = fmt.Errorf("begin document transaction: %w", sanitizeErr(err))
		return
	}
	if o.ds.fence != nil {
		if ferr := o.ds.fence(o.ctx, tx); ferr != nil {
			_ = tx.Rollback(o.ctx)
			o.txErr = ferr
			return
		}
	}
	o.tx = tx
}

// EndWrite commits the region's transaction and returns the commit failure to
// the region's caller, so a write never reports success it did not persist.
func (o *docOps) EndWrite() error {
	if o.bound {
		return nil
	}
	regionErr := o.txErr
	o.txErr = nil
	if o.tx == nil {
		return regionErr
	}
	tx := o.tx
	o.tx = nil
	if err := tx.Commit(o.ctx); err != nil {
		return fmt.Errorf("commit document transaction: %w", sanitizeErr(err))
	}
	return nil
}

func (o *docOps) Read(rel string) ([]byte, error) {
	ref, err := resolveDoc(rel)
	if err != nil {
		return nil, err
	}
	var out []byte
	err = o.runTx(func(tx pgx.Tx) error {
		var err error
		out, err = o.readRef(tx, ref)
		return err
	})
	return out, err
}

func (o *docOps) readRef(tx pgx.Tx, ref docRef) ([]byte, error) {
	q := gen.New(tx)
	switch ref.class {
	case docState:
		row, err := q.GetStoryState(o.ctx, gen.GetStoryStateParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name})
		if err != nil {
			return nil, o.notFound(err)
		}
		return row.Payload, nil
	case docOutline:
		row, err := q.GetOutline(o.ctx, gen.GetOutlineParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name})
		if err != nil {
			return nil, o.notFound(err)
		}
		if !ref.text {
			return row.Payload, nil
		}
		if !row.TextContent.Valid {
			return nil, o.notFound(pgx.ErrNoRows)
		}
		return []byte(row.TextContent.String), nil
	case docArtifact:
		row, err := q.GetStoryArtifactByKey(o.ctx, gen.GetStoryArtifactByKeyParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: ref.name, ArtifactKey: ref.key})
		if err != nil {
			return nil, o.notFound(err)
		}
		if ref.text {
			if !row.TextContent.Valid {
				return nil, o.notFound(pgx.ErrNoRows)
			}
			return []byte(row.TextContent.String), nil
		}
		if len(row.Payload) == 0 {
			return nil, o.notFound(pgx.ErrNoRows)
		}
		return row.Payload, nil
	case docStream:
		rows, err := o.listEvents(tx, ref.name, ref.key)
		if err != nil {
			return nil, err
		}
		return joinPayloads(rows), nilOrNotExist(joinPayloads(rows))
	case docCheckpoints:
		rows, err := q.ListCheckpoints(o.ctx, gen.ListCheckpointsParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID})
		if err != nil {
			return nil, sanitizeErr(err)
		}
		if len(rows) == 0 {
			return nil, o.notFound(pgx.ErrNoRows)
		}
		var buf bytes.Buffer
		for _, r := range rows {
			data, err := json.Marshal(checkpointFromRow(domain.Scope{}, r))
			if err != nil {
				return nil, err
			}
			buf.Write(data)
			buf.WriteByte('\n')
		}
		return buf.Bytes(), nil
	case docAgentSession:
		rows, err := q.ListAgentSessions(o.ctx, gen.ListAgentSessionsParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Agent: ref.agent, TaskKey: ref.key})
		if err != nil {
			return nil, sanitizeErr(err)
		}
		if len(rows) == 0 {
			return nil, o.notFound(pgx.ErrNoRows)
		}
		payloads := make([][]byte, 0, len(rows))
		for _, r := range rows {
			payloads = append(payloads, r.Payload)
		}
		return joinPayloads(payloads), nil
	case docChapterDraft:
		row, err := q.GetChapterByNumber(o.ctx, gen.GetChapterByNumberParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Number: int32(ref.chapter)})
		if err != nil {
			return nil, o.notFound(err)
		}
		if row.DraftText == "" {
			return nil, o.notFound(pgx.ErrNoRows)
		}
		return []byte(row.DraftText), nil
	case docChapterRecord:
		// The accepted record is the current revision row; render it exactly as
		// the file backend's record file so checkpoint digests see a fact change
		// (not a path change) when a revision is accepted.
		row, err := q.GetChapterByNumber(o.ctx, gen.GetChapterByNumberParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Number: int32(ref.chapter)})
		if err != nil {
			return nil, o.notFound(err)
		}
		version, err := q.LatestChapterVersion(o.ctx, gen.LatestChapterVersionParams{
			ProjectID: o.scope.ProjectID, ChapterID: row.ID})
		if err != nil {
			return nil, o.notFound(err)
		}
		record := chapterVersionRecord(ref.chapter, version)
		return json.Marshal(record)
	}
	return nil, errUnmappedPath
}

func joinPayloads(rows [][]byte) []byte {
	if len(rows) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, p := range rows {
		buf.Write(p)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func nilOrNotExist(data []byte) error {
	if len(data) == 0 {
		return &fs.PathError{Op: "open", Path: "document", Err: fs.ErrNotExist}
	}
	return nil
}

// listEvents pages a stream so an unbounded log never rides one query.
func (o *docOps) listEvents(tx pgx.Tx, stream, key string) ([][]byte, error) {
	q := gen.New(tx)
	const page = 500
	var out [][]byte
	var after int64
	for {
		rows, err := q.ListStoryEventsPage(o.ctx, gen.ListStoryEventsPageParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Stream: stream, LogKey: key, Seq: after, Limit: page})
		if err != nil {
			return nil, sanitizeErr(err)
		}
		if len(rows) == 0 {
			return out, nil
		}
		for _, r := range rows {
			out = append(out, r.Payload)
		}
		after = rows[len(rows)-1].Seq
		if len(rows) < page {
			return out, nil
		}
	}
}

func (o *docOps) Write(rel string, data []byte) error {
	ref, err := resolveDoc(rel)
	if err != nil {
		return err
	}
	return o.runTx(func(tx pgx.Tx) error { return o.writeRef(tx, ref, data) })
}

func (o *docOps) writeRef(tx pgx.Tx, ref docRef, data []byte) error {
	q := gen.New(tx)
	switch ref.class {
	case docState:
		if err := q.UpsertStoryState(o.ctx, gen.UpsertStoryStateParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name,
			Payload: data, SchemaVersion: 1}); err != nil {
			return sanitizeErr(err)
		}
		return o.mirrorCharactersDoc(tx, ref.name, data)
	case docOutline:
		text := pgtype.Text{}
		payload := data
		if ref.text {
			payload = []byte(`{}`)
			text = pgtype.Text{String: string(data), Valid: true}
		}
		if err := q.UpsertOutline(o.ctx, gen.UpsertOutlineParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: ref.name, Payload: payload, TextContent: text}); err != nil {
			return sanitizeErr(err)
		}
		if ref.name == "layered" && !ref.text {
			return o.mirrorVolumes(tx, data)
		}
		return nil
	case docArtifact:
		payload, text := data, pgtype.Text{}
		if ref.text {
			payload, text = []byte(`{}`), pgtype.Text{String: string(data), Valid: true}
		}
		return o.upsertArtifact(tx, ref, payload, text)
	case docStream:
		return o.replaceStream(tx, ref, data)
	case docCheckpoints:
		return o.replaceCheckpoints(tx, data)
	case docAgentSession:
		return o.replaceSession(tx, ref, data)
	case docChapterDraft:
		row, err := o.ensureChapter(tx, ref.chapter)
		if err != nil {
			return err
		}
		n, err := q.SaveChapterDraft(o.ctx, gen.SaveChapterDraftParams{
			ProjectID: o.scope.ProjectID, ID: row.ID, DraftText: string(data)})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("lưu nháp chương %d: %w", ref.chapter, errs.ErrStorageConflict)
		}
		return nil
	case docChapterRecord:
		return errChapterRecordReadOnly
	}
	return errUnmappedPath
}

func (o *docOps) upsertArtifact(tx pgx.Tx, ref docRef, payload []byte, text pgtype.Text) error {
	return sanitizeErr(gen.New(tx).UpsertStoryArtifactByKey(o.ctx, gen.UpsertStoryArtifactByKeyParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
		Kind: ref.name, ArtifactKey: ref.key,
		Payload: payload, TextContent: text, SchemaVersion: 1}))
}

func (o *docOps) Append(rel string, data []byte) error {
	ref, err := resolveDoc(rel)
	if err != nil {
		return err
	}
	lines := splitLines(data)
	return o.runTx(func(tx pgx.Tx) error {
		for _, line := range lines {
			if err := o.appendRef(tx, ref, line); err != nil {
				return err
			}
		}
		return nil
	})
}

func (o *docOps) appendRef(tx pgx.Tx, ref docRef, line []byte) error {
	q := gen.New(tx)
	switch ref.class {
	case docStream:
		_, err := q.AppendStoryEventLog(o.ctx, gen.AppendStoryEventLogParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Stream: ref.name, LogKey: ref.key, Payload: line})
		return sanitizeErr(err)
	case docCheckpoints:
		var cp domain.Checkpoint
		if err := json.Unmarshal(line, &cp); err != nil {
			return fmt.Errorf("parse checkpoint line: %w", err)
		}
		return o.insertCheckpoint(tx, cp)
	case docAgentSession:
		_, err := q.AppendAgentSession(o.ctx, gen.AppendAgentSessionParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Agent: ref.agent, TaskKey: ref.key, Payload: line})
		return sanitizeErr(err)
	case docArtifact:
		// Derived text projections (timeline.md, …) append in place; rows have
		// no append primitive, so the document is read and rewritten inside the
		// caller's transaction.
		var existing []byte
		row, err := q.GetStoryArtifactByKey(o.ctx, gen.GetStoryArtifactByKeyParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: ref.name, ArtifactKey: ref.key})
		if err == nil && row.TextContent.Valid {
			existing = []byte(row.TextContent.String)
		} else if err != nil && !noRows(err) {
			return sanitizeErr(err)
		}
		merged := append(existing, line...)
		return o.upsertArtifact(tx, ref, []byte(`{}`), pgtype.Text{String: string(merged), Valid: true})
	}
	return fmt.Errorf("append is not supported for %s: %w", ref.name, errUnmappedPath)
}

func (o *docOps) insertCheckpoint(tx pgx.Tx, cp domain.Checkpoint) error {
	payload, err := json.Marshal(map[string]any{
		"scope": cp.Scope, "step": cp.Step, "artifact": cp.Artifact, "digest": cp.Digest})
	if err != nil {
		return err
	}
	when := cp.OccurredAt
	if when.IsZero() {
		when = nowUTC()
	}
	return sanitizeErr(gen.New(tx).InsertCheckpointExact(o.ctx, gen.InsertCheckpointExactParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Seq: cp.Seq,
		ScopeKey: cp.Scope.String(), Step: cp.Step, ArtifactRef: cp.Artifact,
		Sha256: cp.Digest, OccurredAt: ts(when), Payload: payload}))
}

func (o *docOps) replaceStream(tx pgx.Tx, ref docRef, data []byte) error {
	if err := o.maintenance(func(tx pgx.Tx) error {
		return sanitizeErr(gen.New(tx).DeleteStoryEventsLog(o.ctx, gen.DeleteStoryEventsLogParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Stream: ref.name, LogKey: ref.key}))
	}); err != nil {
		return err
	}
	for i, line := range splitLines(data) {
		if err := gen.New(tx).InsertStoryEventExact(o.ctx, gen.InsertStoryEventExactParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Stream: ref.name, LogKey: ref.key, Seq: int64(i + 1), Payload: line}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}

func (o *docOps) replaceCheckpoints(tx pgx.Tx, data []byte) error {
	if err := o.maintenance(func(tx pgx.Tx) error {
		return sanitizeErr(gen.New(tx).DeleteCheckpoints(o.ctx, gen.DeleteCheckpointsParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}))
	}); err != nil {
		return err
	}
	for _, line := range splitLines(data) {
		var cp domain.Checkpoint
		if err := json.Unmarshal(line, &cp); err != nil {
			return fmt.Errorf("parse checkpoint line: %w", err)
		}
		if err := o.insertCheckpoint(tx, cp); err != nil {
			return err
		}
	}
	return nil
}

func (o *docOps) replaceSession(tx pgx.Tx, ref docRef, data []byte) error {
	if err := o.maintenance(func(tx pgx.Tx) error {
		return sanitizeErr(gen.New(tx).DeleteAgentSession(o.ctx, gen.DeleteAgentSessionParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Agent: ref.agent, TaskKey: ref.key}))
	}); err != nil {
		return err
	}
	for i, line := range splitLines(data) {
		if err := gen.New(tx).InsertAgentSessionExact(o.ctx, gen.InsertAgentSessionExactParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Agent: ref.agent, TaskKey: ref.key, Seq: int64(i + 1), Payload: line}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}

func (o *docOps) Sync(rel string) error {
	// Rows are durable once committed; the append protocols only need a sync
	// for filesystem tails. Existence is still checked so a caller cannot sync
	// a log that was never written.
	ok, err := o.Exists(rel)
	if err != nil {
		return err
	}
	if !ok {
		return o.notFound(pgx.ErrNoRows)
	}
	return nil
}

func (o *docOps) Remove(rel string) error {
	ref, err := resolveDoc(rel)
	if err != nil {
		return err
	}
	return o.runTx(func(tx pgx.Tx) error { return o.removeRef(tx, ref) })
}

// removeStreamRows deletes append-only rows: the document layer owns stream
// resets, so it opens the documented maintenance escape for this statement.
func (o *docOps) removeStreamRows(fn func(tx pgx.Tx) error) error {
	return o.maintenance(fn)
}

func (o *docOps) removeRef(tx pgx.Tx, ref docRef) error {
	q := gen.New(tx)
	switch ref.class {
	case docState:
		if err := q.DeleteStoryState(o.ctx, gen.DeleteStoryStateParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name}); err != nil {
			return sanitizeErr(err)
		}
		if ref.name == "characters" {
			return sanitizeErr(q.DeleteCharactersForStory(o.ctx, gen.DeleteCharactersForStoryParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}))
		}
		return nil
	case docOutline:
		if err := q.DeleteOutline(o.ctx, gen.DeleteOutlineParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name}); err != nil {
			return sanitizeErr(err)
		}
		if ref.name == "layered" {
			return sanitizeErr(q.DeleteVolumesForStory(o.ctx, gen.DeleteVolumesForStoryParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}))
		}
		return nil
	case docArtifact:
		_, err := q.DeleteStoryArtifact(o.ctx, gen.DeleteStoryArtifactParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: ref.name, ArtifactKey: ref.key})
		return sanitizeErr(err)
	case docStream:
		return o.removeStreamRows(func(tx pgx.Tx) error {
			return sanitizeErr(gen.New(tx).DeleteStoryEventsLog(o.ctx, gen.DeleteStoryEventsLogParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Stream: ref.name, LogKey: ref.key}))
		})
	case docCheckpoints:
		return o.removeStreamRows(func(tx pgx.Tx) error {
			return sanitizeErr(gen.New(tx).DeleteCheckpoints(o.ctx, gen.DeleteCheckpointsParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}))
		})
	case docAgentSession:
		return o.removeStreamRows(func(tx pgx.Tx) error {
			return sanitizeErr(gen.New(tx).DeleteAgentSession(o.ctx, gen.DeleteAgentSessionParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Agent: ref.agent, TaskKey: ref.key}))
		})
	case docChapterDraft:
		row, err := q.GetChapterByNumber(o.ctx, gen.GetChapterByNumberParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Number: int32(ref.chapter)})
		if err != nil {
			if noRows(err) {
				return nil
			}
			return sanitizeErr(err)
		}
		n, err := q.SaveChapterDraft(o.ctx, gen.SaveChapterDraftParams{
			ProjectID: o.scope.ProjectID, ID: row.ID, DraftText: ""})
		if err != nil {
			return sanitizeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("xoá nháp chương %d: %w", ref.chapter, errs.ErrStorageConflict)
		}
		return nil
	case docChapterRecord:
		return errChapterRecordReadOnly
	}
	return errUnmappedPath
}

func (o *docOps) RemoveTree(rel string) error {
	rel = strings.TrimSuffix(strings.ReplaceAll(rel, "\\", "/"), "/")
	switch rel {
	case "meta/runtime/tasks":
		return o.maintenance(func(tx pgx.Tx) error {
			keys, err := o.eventKeys(tx, "runtime_log")
			if err != nil {
				return err
			}
			for _, key := range keys {
				if err := gen.New(tx).DeleteStoryEventsLog(o.ctx, gen.DeleteStoryEventsLogParams{
					ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
					Stream: "runtime_log", LogKey: key}); err != nil {
					return sanitizeErr(err)
				}
			}
			return nil
		})
	case "meta/sessions/agents":
		return o.maintenance(func(tx pgx.Tx) error {
			keys, err := gen.New(tx).ListAgentSessionKeys(o.ctx, gen.ListAgentSessionKeysParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID})
			if err != nil {
				return sanitizeErr(err)
			}
			for _, k := range keys {
				if err := gen.New(tx).DeleteAgentSession(o.ctx, gen.DeleteAgentSessionParams{
					ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
					Agent: k.Agent, TaskKey: k.TaskKey}); err != nil {
					return sanitizeErr(err)
				}
			}
			return nil
		})
	case "nao/nhan-vat":
		return o.runTx(func(tx pgx.Tx) error {
			return o.removeArtifactPrefix(tx, "markdown_view", rel+"/")
		})
	}
	return fmt.Errorf("remove tree %q: %w", rel, errUnmappedPath)
}

func (o *docOps) removeArtifactPrefix(tx pgx.Tx, kind, prefix string) error {
	q := gen.New(tx)
	keys, err := q.ListStoryArtifactKeys(o.ctx, gen.ListStoryArtifactKeysParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
		Kind: kind, Column4: pgtype.Text{String: prefix, Valid: true}})
	if err != nil {
		return sanitizeErr(err)
	}
	for _, key := range keys {
		if _, err := q.DeleteStoryArtifact(o.ctx, gen.DeleteStoryArtifactParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: kind, ArtifactKey: key}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}

// truncateFile keeps the first size bytes of an append log by deleting the
// rows whose re-marshalled payload no longer fits — the row-level equivalent
// of truncating an uncommitted tail.
func (o *docOps) Truncate(rel string, size int64) error {
	ref, err := resolveDoc(rel)
	if err != nil {
		return err
	}
	return o.maintenance(func(tx pgx.Tx) error {
		switch ref.class {
		case docStream:
			rows, err := gen.New(tx).ListStoryEventsPage(o.ctx, gen.ListStoryEventsPageParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Stream: ref.name, LogKey: ref.key, Seq: 0, Limit: 100000})
			if err != nil {
				return sanitizeErr(err)
			}
			var total int64
			for _, r := range rows {
				if total+int64(len(r.Payload)+1) > size {
					return sanitizeErr(gen.New(tx).DeleteStoryEventsFromSeq(o.ctx, gen.DeleteStoryEventsFromSeqParams{
						ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
						Stream: ref.name, LogKey: ref.key, Seq: r.Seq}))
				}
				total += int64(len(r.Payload) + 1)
			}
			return nil
		case docCheckpoints:
			rows, err := gen.New(tx).ListCheckpoints(o.ctx, gen.ListCheckpointsParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID})
			if err != nil {
				return sanitizeErr(err)
			}
			var total int64
			for _, r := range rows {
				data, _ := json.Marshal(checkpointFromRow(domain.Scope{}, r))
				if total+int64(len(data)+1) > size {
					return sanitizeErr(gen.New(tx).DeleteCheckpointsFromSeq(o.ctx, gen.DeleteCheckpointsFromSeqParams{
						ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Seq: r.Seq}))
				}
				total += int64(len(data) + 1)
			}
			return nil
		case docAgentSession:
			rows, err := gen.New(tx).ListAgentSessions(o.ctx, gen.ListAgentSessionsParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Agent: ref.agent, TaskKey: ref.key})
			if err != nil {
				return sanitizeErr(err)
			}
			var total int64
			for _, r := range rows {
				if total+int64(len(r.Payload)+1) > size {
					return sanitizeErr(gen.New(tx).DeleteAgentSessionsFromSeq(o.ctx, gen.DeleteAgentSessionsFromSeqParams{
						ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
						Agent: ref.agent, TaskKey: ref.key, Seq: r.Seq}))
				}
				total += int64(len(r.Payload) + 1)
			}
			return nil
		}
		return fmt.Errorf("truncate is not supported for %s: %w", ref.name, errUnmappedPath)
	})
}

func (o *docOps) ListDir(rel string) ([]store.DirEntry, error) {
	rel = strings.TrimSuffix(strings.ReplaceAll(rel, "\\", "/"), "/")
	switch rel {
	case "reviews", "reviews/rounds":
		// Reviews are typed rows; the file layout has no document twin.
		return nil, nil
	case "summaries":
		return o.artifactDir("summary", "", ".json")
	case "nao/nhan-vat":
		return o.artifactDir("markdown_view", "nao/nhan-vat/", "")
	case "meta/sessions/agents":
		var out []store.DirEntry
		err := o.runTx(func(tx pgx.Tx) error {
			keys, err := gen.New(tx).ListAgentSessionKeys(o.ctx, gen.ListAgentSessionKeysParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID})
			if err != nil {
				return sanitizeErr(err)
			}
			for _, k := range keys {
				out = append(out, store.DirEntry{Name: k.Agent + "-" + k.TaskKey + ".jsonl"})
			}
			return nil
		})
		return out, err
	case "meta/runtime/tasks":
		var out []store.DirEntry
		err := o.runTx(func(tx pgx.Tx) error {
			keys, err := o.eventKeys(tx, "runtime_log")
			if err != nil {
				return err
			}
			for _, k := range keys {
				out = append(out, store.DirEntry{Name: k + ".log"})
			}
			return nil
		})
		return out, err
	}
	return nil, fmt.Errorf("list dir %q: %w", rel, errUnmappedPath)
}

func (o *docOps) artifactDir(kind, prefix, suffix string) ([]store.DirEntry, error) {
	var out []store.DirEntry
	err := o.runTx(func(tx pgx.Tx) error {
		keys, err := gen.New(tx).ListStoryArtifactKeys(o.ctx, gen.ListStoryArtifactKeysParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
			Kind: kind, Column4: pgtype.Text{String: prefix, Valid: true}})
		if err != nil {
			return sanitizeErr(err)
		}
		for _, key := range keys {
			name := strings.TrimPrefix(key, prefix)
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}
			out = append(out, store.DirEntry{Name: name + suffix})
		}
		return nil
	})
	return out, err
}

func (o *docOps) eventKeys(tx pgx.Tx, stream string) ([]string, error) {
	rows, err := gen.New(tx).ListStoryEventKeys(o.ctx, gen.ListStoryEventKeysParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Stream: stream})
	if err != nil {
		return nil, sanitizeErr(err)
	}
	return rows, nil
}

func (o *docOps) Exists(rel string) (bool, error) {
	ref, err := resolveDoc(rel)
	if err != nil {
		return false, err
	}
	var ok bool
	err = o.runTx(func(tx pgx.Tx) error {
		q := gen.New(tx)
		switch ref.class {
		case docState:
			_, err := q.GetStoryState(o.ctx, gen.GetStoryStateParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name})
			ok = err == nil
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
		case docOutline:
			_, err := q.GetOutline(o.ctx, gen.GetOutlineParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Kind: ref.name})
			ok = err == nil
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
		case docArtifact:
			_, err := q.GetStoryArtifactByKey(o.ctx, gen.GetStoryArtifactByKeyParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Kind: ref.name, ArtifactKey: ref.key})
			ok = err == nil
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
		case docStream:
			rows, err := q.ListStoryEventsPage(o.ctx, gen.ListStoryEventsPageParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Stream: ref.name, LogKey: ref.key, Seq: 0, Limit: 1})
			if err != nil {
				return sanitizeErr(err)
			}
			ok = len(rows) > 0
		case docCheckpoints:
			rows, err := q.ListCheckpoints(o.ctx, gen.ListCheckpointsParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID})
			if err != nil {
				return sanitizeErr(err)
			}
			ok = len(rows) > 0
		case docAgentSession:
			rows, err := q.ListAgentSessions(o.ctx, gen.ListAgentSessionsParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
				Agent: ref.agent, TaskKey: ref.key})
			if err != nil {
				return sanitizeErr(err)
			}
			ok = len(rows) > 0
		case docChapterDraft:
			row, err := q.GetChapterByNumber(o.ctx, gen.GetChapterByNumberParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Number: int32(ref.chapter)})
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
			ok = err == nil && row.DraftText != ""
		case docChapterRecord:
			row, err := q.GetChapterByNumber(o.ctx, gen.GetChapterByNumberParams{
				ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, Number: int32(ref.chapter)})
			if err != nil && !noRows(err) {
				return sanitizeErr(err)
			}
			if err != nil {
				break
			}
			_, verr := q.LatestChapterVersion(o.ctx, gen.LatestChapterVersionParams{
				ProjectID: o.scope.ProjectID, ChapterID: row.ID})
			if verr != nil && !noRows(verr) {
				return sanitizeErr(verr)
			}
			ok = verr == nil
		}
		return nil
	})
	return ok, err
}

func (o *docOps) MkdirAll(_ []string) error { return nil }

// ── helpers ─────────────────────────────────────────────────────────────────

func (o *docOps) notFound(err error) error {
	if noRows(err) {
		return &fs.PathError{Op: "open", Path: "document", Err: fs.ErrNotExist}
	}
	return sanitizeErr(err)
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		out = append(out, append([]byte(nil), line...))
	}
	return out
}

func (o *docOps) ensureChapter(tx pgx.Tx, chapter int) (gen.Chapter, error) {
	id, err := randomUUID()
	if err != nil {
		return gen.Chapter{}, err
	}
	row, err := gen.New(tx).EnsureChapter(o.ctx, gen.EnsureChapterParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID,
		ID: id, Number: int32(chapter)})
	if err != nil {
		if noRows(err) {
			return gen.Chapter{}, fmt.Errorf("chương %d đã bị xoá; không thể dùng lại số chương: %w", chapter, errs.ErrStorageConflict)
		}
		return gen.Chapter{}, sanitizeErr(err)
	}
	return gen.Chapter{
		ProjectID: row.ProjectID, StoryID: row.StoryID, ID: row.ID,
		VolumeID: row.VolumeID, Number: row.Number, Title: row.Title,
		DraftText: row.DraftText, CurrentRevision: row.CurrentRevision,
		DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt, Version: row.Version,
	}, nil
}

// mirrorCharactersDoc keeps the relational character rows (search, FK) in
// step with the characters document, which stays authoritative exactly as
// characters.json is in file mode.
func (o *docOps) mirrorCharactersDoc(tx pgx.Tx, kind string, data []byte) error {
	if kind != "characters" {
		return nil
	}
	var chars []domain.Character
	if err := json.Unmarshal(data, &chars); err != nil {
		return fmt.Errorf("parse characters document: %w", err)
	}
	q := gen.New(tx)
	if err := q.DeleteCharactersForStory(o.ctx, gen.DeleteCharactersForStoryParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}); err != nil {
		return sanitizeErr(err)
	}
	for _, c := range chars {
		id, err := randomUUID()
		if err != nil {
			return err
		}
		payload, err := json.Marshal(c)
		if err != nil {
			return err
		}
		legacy := strings.TrimSpace(c.ID)
		if legacy == "" {
			legacy = c.Name
		}
		if err := q.InsertCharacter(o.ctx, gen.InsertCharacterParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, ID: id,
			LegacyID: legacy, Name: c.Name, Payload: payload}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}

func (o *docOps) mirrorVolumes(tx pgx.Tx, data []byte) error {
	var volumes []domain.VolumeOutline
	if err := json.Unmarshal(data, &volumes); err != nil {
		return fmt.Errorf("parse layered outline document: %w", err)
	}
	q := gen.New(tx)
	if err := q.DeleteVolumesForStory(o.ctx, gen.DeleteVolumesForStoryParams{
		ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID}); err != nil {
		return sanitizeErr(err)
	}
	for _, v := range volumes {
		vid, err := randomUUID()
		if err != nil {
			return err
		}
		outlineJSON, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if err := q.UpsertVolume(o.ctx, gen.UpsertVolumeParams{
			ProjectID: o.scope.ProjectID, StoryID: o.scope.StoryID, ID: vid,
			Number: int32(v.Index), Title: v.Title, Outline: outlineJSON}); err != nil {
			return sanitizeErr(err)
		}
	}
	return nil
}
