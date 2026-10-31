package storageimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/blobstore"
)

// querier is the read/write surface shared by *pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ── digest projection ────────────────────────────────────────────────────────

// digestPayload projects a payload onto the fields that are reproducible from
// the database. Two payloads with the same projection must hash identically
// whether they came from the source file or from a SELECT — that equality is
// what makes reconciliation meaningful.
func digestPayload(kind string, payload any) any {
	switch kind {
	case "user":
		u := payload.(AccountUser)
		return accountUserDigest{
			Username: u.Username, Name: u.Name, Role: u.Role,
			Disabled: u.Disabled || u.DeletedAt != nil, Deleted: u.DeletedAt != nil,
			MustChange: u.MustChangePassword, OnboardedAt: u.OnboardedAt,
			DisabledAt: u.DisabledAt, DisabledReason: u.DisabledReason,
			PasswordHash: u.PasswordHash, PasswordChangedAt: u.PasswordChangedAt,
			RecoveryHash: recoveryHash(u.Recovery), RecoveryMask: recoveryMask(u.Recovery),
			TempHash: tempHash(u.TempPassword), TempUsed: tempUsed(u.TempPassword),
		}
	case "audio_script":
		s := payload.(AudioScriptRow)
		return audioScriptDigest{Chapter: s.Chapter, Revision: s.Revision, Payload: s.Payload, Segments: s.SegmentCount}
	case "chapter":
		c := payload.(ChapterRow)
		versions := c.Versions
		if versions == nil {
			versions = []ChapterVersion{}
		}
		return chapterDigest{Number: c.Number, Title: c.Title, Current: c.Current, Versions: versions}
	case "job":
		j := payload.(JobRow)
		return jobDigest{Kind: j.Kind, ID: j.ID, State: j.State, IdemKey: j.IdemKey,
			Input: j.Input, Result: j.Result, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
	case "document":
		d := payload.(DocumentRow)
		return documentDigest{Kind: d.Kind, LogicalName: d.LogicalName, Text: d.Text}
	case "comment":
		c := payload.(CommentRow)
		proj, err := projectComment(c.Raw)
		if err != nil {
			return c
		}
		proj.Chapter = c.Chapter
		return proj
	case "review":
		r := payload.(ReviewRow)
		return reviewDigest{Chapter: r.Chapter, Round: r.Round, Payload: r.Payload}
	case "chapter_prose":
		p := payload.(ChapterProseRow)
		return chapterProseDigest{Number: p.Number, Text: p.Text}
	case "chapter_draft":
		d := payload.(ChapterDraftRow)
		return chapterDraftDigest{Number: d.Number, Text: d.Text}
	case "event":
		e := payload.(EventRow)
		return eventDigest{Stream: e.Stream, Seq: e.Seq, Payload: e.Payload}
	}
	return payload
}

type accountUserDigest struct {
	Username          string
	Name              string
	Role              string
	Disabled          bool
	Deleted           bool
	MustChange        bool
	OnboardedAt       *time.Time
	DisabledAt        *time.Time
	DisabledReason    string
	PasswordHash      string
	PasswordChangedAt *time.Time
	RecoveryHash      string
	RecoveryMask      string
	TempHash          string
	TempUsed          bool
}

type audioScriptDigest struct {
	Chapter  int
	Revision int
	Payload  json.RawMessage
	Segments int
}

func recoveryHash(r *AccountRecovery) string {
	if r == nil {
		return ""
	}
	return r.CodeHash
}

func recoveryMask(r *AccountRecovery) string {
	if r == nil {
		return ""
	}
	return r.Mask
}

func tempHash(t *AccountTempPassword) string {
	if t == nil {
		return ""
	}
	return t.CodeHash
}

func tempUsed(t *AccountTempPassword) bool { return t != nil && t.UsedAt != nil }

// ItemDigest is the canonical semantic digest for a decoded payload.
func ItemDigest(kind string, payload any) (string, error) {
	return SemanticDigest(digestPayload(kind, payload))
}

// ── row loaders ──────────────────────────────────────────────────────────────

// loadItem reads the normalized record back from the database in the same
// shape the decoder produced. It is used both by the idempotency check and by
// reconciliation, so the two can never drift apart.
func (im *importer) loadItem(ctx context.Context, q querier, item Item, scope recordScope) (any, error) {
	sourceID := im.scan.SourceID
	switch item.Kind {
	case "library":
		var payload []byte
		if err := q.QueryRow(ctx, `SELECT payload FROM library_settings WHERE source_id=$1`,
			uuidArg(sourceID)).Scan(&payload); err != nil {
			return nil, err
		}
		var cfg LibraryConfig
		if err := json.Unmarshal(payload, &cfg); err != nil {
			return nil, err
		}
		cfg.Dirs = cfg.Dirs.withDefaults()
		return cfg, nil
	case "user":
		id := im.ids.Users[item.Key]
		if id == "" {
			id = DeterministicUUID(sourceID, "user", strings.ToLower(item.Key))
		}
		return loadUser(ctx, q, id)
	case "audit":
		return loadAudit(ctx, q, item.TargetID)
	case "project":
		var row ProjectRow
		err := q.QueryRow(ctx, `
SELECT id, slug, name, description, notes, universe_name, settings, created_at, updated_at
FROM projects WHERE id=$1`, uuidArg(item.TargetID)).Scan(&row.ID, &row.Slug, &row.Name,
			&row.Description, &row.Notes, &row.UniverseName, &row.Settings, &row.CreatedAt, &row.UpdatedAt)
		return row, err
	case "document":
		var row DocumentRow
		want := item.Payload.(DocumentRow)
		err := q.QueryRow(ctx, `
SELECT id, project_id, kind, logical_name, COALESCE(text_content,'')
FROM project_documents WHERE project_id=$1 AND kind=$2 AND logical_name=$3`,
			uuidArg(scope.ProjectID), want.Kind, want.LogicalName).
			Scan(&row.ID, &row.ProjectID, &row.Kind, &row.LogicalName, &row.Text)
		return row, err
	case "story":
		var row StoryRow
		err := q.QueryRow(ctx, `
SELECT id, project_id, slug, title, short_title, synopsis, title_renamed, status, tags, published_at, created_at, updated_at
FROM stories WHERE project_id=$1 AND id=$2`, uuidArg(scope.ProjectID), uuidArg(item.TargetID)).
			Scan(&row.ID, &row.ProjectID, &row.Slug, &row.Title, &row.ShortTitle, &row.Synopsis,
				&row.TitleRenamed, &row.Status, &row.Tags, &row.PublishedAt, &row.CreatedAt, &row.UpdatedAt)
		return row, err
	case "chapter":
		return loadChapter(ctx, q, sourceID, scope, strconvInt(item.Key))
	case "chapter_prose":
		return loadChapterProse(ctx, q, scope, strconvInt(item.Key))
	case "chapter_draft":
		return loadChapterDraft(ctx, q, sourceID, scope, strconvInt(item.Key))
	case "outline":
		var row OutlineRow
		var payload []byte
		var text *string
		err := q.QueryRow(ctx, `
SELECT payload, text_content FROM outlines WHERE project_id=$1 AND story_id=$2 AND kind=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), item.Key).Scan(&payload, &text)
		if err != nil {
			return nil, err
		}
		row.Kind = item.Key
		row.Payload = payload
		if text != nil {
			row.Text = *text
		}
		return row, nil
	case "character":
		var row CharacterRow
		err := q.QueryRow(ctx, `
SELECT legacy_id, name, payload FROM characters WHERE project_id=$1 AND story_id=$2 AND legacy_id=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), item.Key).
			Scan(&row.LegacyID, &row.Name, &row.Payload)
		return row, err
	case "state":
		var row StateRow
		err := q.QueryRow(ctx, `
SELECT kind, payload FROM story_state WHERE project_id=$1 AND story_id=$2 AND kind=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), item.Key).Scan(&row.Kind, &row.Payload)
		return row, err
	case "artifact":
		want := item.Payload.(ArtifactRow)
		var row ArtifactRow
		var payload []byte
		var text *string
		err := q.QueryRow(ctx, `
SELECT COALESCE(payload::text,''), text_content FROM story_artifacts
WHERE project_id=$1 AND story_id=$2 AND kind=$3 AND artifact_key=$4`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), artifactKind(want), want.Key).
			Scan(&payload, &text)
		if err != nil {
			return nil, err
		}
		row.Kind = want.Kind
		row.Key = want.Key
		row.ContentType = want.ContentType
		if len(payload) > 0 {
			row.Payload = json.RawMessage(payload)
		}
		if text != nil {
			row.Text = *text
		}
		return row, nil
	case "comment":
		return loadComment(ctx, q, sourceID, scope, item.Key)
	case "review":
		var row ReviewRow
		var payload []byte
		var scopeKey string
		err := q.QueryRow(ctx, `
SELECT scope_key, round, payload FROM reviews WHERE project_id=$1 AND story_id=$2 AND id=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(item.TargetID)).
			Scan(&scopeKey, &row.Round, &payload)
		if err != nil {
			return nil, err
		}
		row.Payload = payload
		row.Scope = ""
		if n, ok := strings.CutPrefix(scopeKey, "chapter-"); ok {
			if v, err := strconv.Atoi(n); err == nil {
				row.Chapter = v
			}
		}
		return row, nil
	case "checkpoint":
		var row CheckpointRow
		var payload []byte
		err := q.QueryRow(ctx, `
SELECT seq, scope_key, step, artifact_ref, sha256, occurred_at, payload
FROM checkpoints WHERE project_id=$1 AND story_id=$2 AND seq=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), strconvInt(item.Key)).
			Scan(&row.Seq, &row.Scope, &row.Step, &row.Artifact, &row.Digest, &row.OccurredAt, &payload)
		row.Raw = payload
		return row, err
	case "event":
		stream, logKey, seq := splitEventRecordKey(item.Key)
		var row EventRow
		var payload []byte
		err := q.QueryRow(ctx, `
SELECT stream, log_key, seq, payload FROM story_events
WHERE project_id=$1 AND story_id=$2 AND stream=$3 AND log_key=$4 AND seq=$5`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), stream, logKey, seq).
			Scan(&row.Stream, &row.LogKey, &row.Seq, &payload)
		row.Payload = payload
		return row, err
	case "session":
		parts := strings.Split(item.Key, "/")
		if len(parts) != 3 {
			return nil, fmt.Errorf("session key lạ %q", item.Key)
		}
		seq, _ := strconv.ParseInt(parts[2], 10, 64)
		var row SessionRow
		var payload []byte
		err := q.QueryRow(ctx, `
SELECT agent, task_key, seq, payload FROM agent_sessions
WHERE project_id=$1 AND story_id=$2 AND agent=$3 AND task_key=$4 AND seq=$5`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), parts[0], parts[1], seq).
			Scan(&row.Agent, &row.TaskKey, &row.Seq, &payload)
		row.Payload = payload
		return row, err
	case "audio_settings":
		var row AudioSettingsRow
		var payload []byte
		err := q.QueryRow(ctx, `SELECT payload FROM audio_settings WHERE project_id=$1 AND story_id=$2`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID)).Scan(&payload)
		row.Payload = payload
		return row, err
	case "audio_cast":
		var row AudioCastRow
		var payload []byte
		err := q.QueryRow(ctx, `SELECT role_key, payload FROM audio_cast WHERE project_id=$1 AND story_id=$2 AND role_key=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), item.Key).Scan(&row.RoleKey, &payload)
		row.Payload = payload
		return row, err
	case "audio_script":
		return loadAudioScript(ctx, q, sourceID, scope, strconvInt(item.Key))
	case "job":
		var row JobRow
		var input, result []byte
		var finished *time.Time
		err := q.QueryRow(ctx, `
SELECT kind, id, state, idempotency_key, input, result, created_at, finished_at
FROM jobs WHERE project_id=$1 AND story_id=$2 AND id=$3`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(item.TargetID)).
			Scan(&row.Kind, &row.ID, &row.State, &row.IdemKey, &input, &result, &row.CreatedAt, &finished)
		row.Input = input
		row.Result = result
		row.FinishedAt = finished
		return row, err
	}
	return nil, fmt.Errorf("loại bản ghi %q chưa có loader", item.Kind)
}

// splitEventRecordKey reverses eventRecordKey: the seq is the last field and
// everything between the first and last field is the log key.
func splitEventRecordKey(key string) (stream, logKey string, seq int64) {
	first := strings.Index(key, "/")
	last := strings.LastIndex(key, "/")
	if first < 0 || last <= first {
		return key, "", 0
	}
	seq, _ = strconv.ParseInt(key[last+1:], 10, 64)
	return key[:first], key[first+1 : last], seq
}

func loadUser(ctx context.Context, q querier, id string) (AccountUser, error) {
	var u AccountUser
	var status, passwordHash, recoveryHash, recoveryMask, tempHash string
	var onboarded, disabledAt, deletedAt, passwordChanged, tempUsed *time.Time
	var passwordHashPtr, recoveryMaskPtr *string
	err := q.QueryRow(ctx, `
SELECT u.username, u.display_name, u.global_role, u.status, u.must_change_password,
       u.onboarded_at, u.disabled_at, u.disabled_reason, u.deleted_at, u.created_at, u.updated_at,
       COALESCE(c.password_hash,''), c.password_changed_at,
       COALESCE(r.code_hash,''), r.mask
FROM users u
LEFT JOIN user_credentials c ON c.user_id = u.id
LEFT JOIN recovery_codes r ON r.user_id = u.id AND r.revoked_at IS NULL
WHERE u.id=$1`, uuidArg(id)).Scan(&u.Username, &u.Name, &u.Role, &status, &u.MustChangePassword,
		&onboarded, &disabledAt, &u.DisabledReason, &deletedAt, &u.CreatedAt, &u.UpdatedAt,
		&passwordHash, &passwordChanged, &recoveryHash, &recoveryMaskPtr)
	if err != nil {
		return u, err
	}
	if recoveryMaskPtr != nil {
		recoveryMask = *recoveryMaskPtr
	}
	u.Disabled = status == "locked"
	u.OnboardedAt = onboarded
	u.DisabledAt = disabledAt
	u.DeletedAt = deletedAt
	u.PasswordHash = passwordHash
	u.PasswordChangedAt = passwordChanged
	if recoveryHash != "" {
		u.Recovery = &AccountRecovery{CodeHash: recoveryHash, Mask: recoveryMask}
	}
	err = q.QueryRow(ctx, `
SELECT password_hash, used_at FROM temporary_passwords WHERE user_id=$1
ORDER BY created_at DESC LIMIT 1`, uuidArg(id)).Scan(&tempHash, &tempUsed)
	if err == nil {
		u.TempPassword = &AccountTempPassword{CodeHash: tempHash, UsedAt: tempUsed}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return u, err
	}
	_ = passwordHashPtr
	return u, nil
}

func loadAudit(ctx context.Context, q querier, id string) (AccountAudit, error) {
	var a AccountAudit
	var action, targetKind, targetID string
	var details []byte
	var actor pgtype.UUID
	err := q.QueryRow(ctx, `
SELECT actor_user_id, action, target_kind, target_id, details, occurred_at
FROM audit_logs WHERE id=$1`, uuidArg(id)).
		Scan(&actor, &action, &targetKind, &targetID, &details, &a.At)
	if err != nil {
		return a, err
	}
	var raw struct {
		Raw         json.RawMessage `json:"raw"`
		SourceType  string          `json:"sourceType"`
		SourceActor string          `json:"sourceActor"`
		Entry       *AccountAudit   `json:"entry"`
	}
	_ = json.Unmarshal(details, &raw)
	if raw.Entry != nil {
		return *raw.Entry, nil
	}
	a.ID = id
	a.Action = action
	a.Type = targetKind
	a.Target = targetID
	a.Actor = raw.Raw
	return a, nil
}

func loadChapter(ctx context.Context, q querier, sourceID string, scope recordScope, number int) (ChapterRow, error) {
	chapterID := DeterministicUUID(sourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(number))
	row := ChapterRow{Number: number}
	var current *int
	err := q.QueryRow(ctx, `SELECT title, draft_text, current_revision FROM chapters WHERE project_id=$1 AND id=$2`,
		uuidArg(scope.ProjectID), uuidArg(chapterID)).Scan(&row.Title, &row.Draft, &current)
	if err != nil {
		return row, err
	}
	if current != nil {
		row.Current = *current
	}
	rows, err := q.Query(ctx, `
SELECT revision_no, content, sha256, origin, COALESCE(legacy_build,''), accepted_at,
       facts::text, style_delta::text
FROM chapter_versions WHERE project_id=$1 AND chapter_id=$2 ORDER BY revision_no`,
		uuidArg(scope.ProjectID), uuidArg(chapterID))
	if err != nil {
		return row, err
	}
	defer rows.Close()
	for rows.Next() {
		var v ChapterVersion
		var facts, style string
		if err := rows.Scan(&v.Revision, &v.Content, &v.SHA256, &v.Origin, &v.Build,
			&v.AcceptedAt, &facts, &style); err != nil {
			return row, err
		}
		v.Facts = json.RawMessage(facts)
		v.StyleDelta = json.RawMessage(style)
		row.Versions = append(row.Versions, v)
	}
	return row, rows.Err()
}

// loadChapterProse reads the workspace prose back from the chapter_text
// artifact — the document the Store's Drafts.LoadChapterText reads.
func loadChapterProse(ctx context.Context, q querier, scope recordScope, number int) (ChapterProseRow, error) {
	row := ChapterProseRow{Number: number}
	var text *string
	err := q.QueryRow(ctx, `
SELECT text_content FROM story_artifacts
WHERE project_id=$1 AND story_id=$2 AND kind='chapter_text' AND artifact_key=$3`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), fmt.Sprintf("%02d", number)).Scan(&text)
	if err != nil {
		return row, err
	}
	if text != nil {
		row.Text = *text
	}
	return row, nil
}

// loadChapterDraft reads the draft document back from chapters.draft_text.
func loadChapterDraft(ctx context.Context, q querier, sourceID string, scope recordScope, number int) (ChapterDraftRow, error) {
	chapterID := DeterministicUUID(sourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(number))
	row := ChapterDraftRow{Number: number}
	err := q.QueryRow(ctx, `SELECT draft_text FROM chapters WHERE project_id=$1 AND id=$2`,
		uuidArg(scope.ProjectID), uuidArg(chapterID)).Scan(&row.Text)
	return row, err
}

func loadComment(ctx context.Context, q querier, sourceID string, scope recordScope, id string) (CommentRow, error) {
	row := CommentRow{}
	var c struct {
		Paragraph  *int
		Quote      string
		Chips      []string
		Text       string
		Author     string
		CreatedAt  time.Time
		Status     string
		SentAt     *time.Time
		FixedAt    *time.Time
		ResolvedAt *time.Time
		FixedBy    string
		Unfixed    bool
		Moved      bool
		Relinked   bool
		Whole      bool
		NewQuote   string
		Candidates []byte
		Anchor     []byte
	}
	var chapterID string
	err := q.QueryRow(ctx, `
SELECT chapter_id, original_paragraph, original_quote, chips, body, legacy_author_name, created_at,
       status, sent_at, fixed_at, resolved_at, COALESCE(fixed_by,''), unfixed, moved, relinked, whole,
       new_quote, candidates::text, COALESCE(anchor::text,'{}')
FROM comments WHERE project_id=$1 AND id=$2`, uuidArg(scope.ProjectID), id).
		Scan(&chapterID, &c.Paragraph, &c.Quote, &c.Chips, &c.Text, &c.Author, &c.CreatedAt,
			&c.Status, &c.SentAt, &c.FixedAt, &c.ResolvedAt, &c.FixedBy, &c.Unfixed, &c.Moved,
			&c.Relinked, &c.Whole, &c.NewQuote, &c.Candidates, &c.Anchor)
	if err != nil {
		return row, err
	}
	chapterNumber := 0
	err = q.QueryRow(ctx, `
SELECT c.number FROM chapters c WHERE c.project_id=$1 AND c.id=$2`, uuidArg(scope.ProjectID), uuidArg(chapterID)).
		Scan(&chapterNumber)
	if err != nil {
		return row, err
	}
	raw := map[string]any{
		"paragraph": c.Paragraph, "quote": c.Quote, "chips": c.Chips, "text": c.Text,
		"author": c.Author, "createdAt": c.CreatedAt, "status": c.Status,
		"sentAt": c.SentAt, "fixedAt": c.FixedAt, "resolvedAt": c.ResolvedAt,
		"fixedBy": c.FixedBy, "unfixed": c.Unfixed, "moved": c.Moved, "relinked": c.Relinked,
		"whole": c.Whole, "newQuote": c.NewQuote,
	}
	if len(c.Candidates) > 0 {
		raw["candidates"] = json.RawMessage(c.Candidates)
	}
	row.Chapter = chapterNumber
	row.Raw = mustJSON(raw)
	return row, nil
}

func loadAudioScript(ctx context.Context, q querier, sourceID string, scope recordScope, chapter int) (AudioScriptRow, error) {
	chapterID := DeterministicUUID(sourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(chapter))
	var row AudioScriptRow
	var payload []byte
	err := q.QueryRow(ctx, `
SELECT revision_no, source_revision, source_sha256, status, payload, chapter_id
FROM audio_scripts WHERE project_id=$1 AND chapter_id=$2 ORDER BY revision_no DESC LIMIT 1`,
		uuidArg(scope.ProjectID), uuidArg(chapterID)).
		Scan(&row.Revision, &row.SourceRev, &row.SourceSHA, &row.Status, &payload, &chapterID)
	if err != nil {
		return row, err
	}
	row.Chapter = chapter
	row.Payload = payload
	if err := q.QueryRow(ctx, `SELECT count(*) FROM audio_segments WHERE project_id=$1 AND chapter_id=$2 AND script_revision=$3`,
		uuidArg(scope.ProjectID), uuidArg(chapterID), row.Revision).Scan(&row.SegmentCount); err != nil {
		return row, err
	}
	return row, nil
}

// ── reconciliation ───────────────────────────────────────────────────────────

// ReconcileOptions configures a read-back.
type ReconcileOptions struct {
	Manifest    *Manifest
	SourceID    string
	Pool        *postgres.Pool
	Blobs       blobstore.BlobStore
	VerifyBlobs bool
	// Keys is the active object-key scheme (STORAGE_ENV). Blob keys are
	// re-parsed with it, so a key outside the scheme is a mismatch even when
	// the bytes are intact.
	Keys blobkey.Keys
}

// DomainReport is one domain's count + digest evidence.
type DomainReport struct {
	Records    int            `json:"records"`
	Tables     map[string]int `json:"tables"`
	Mismatches []string       `json:"mismatches,omitempty"`
}

// Report is the machine-readable reconciliation output.
type Report struct {
	SourceID        string                   `json:"sourceId"`
	ManifestSHA256  string                   `json:"manifestSha256"`
	RunVerified     bool                     `json:"runVerified"`
	FilesChecked    int                      `json:"filesChecked"`
	ItemsChecked    int                      `json:"itemsChecked"`
	BlobsChecked    int                      `json:"blobsChecked"`
	BlobsVerified   int                      `json:"blobsVerified"`
	BlobKeysChecked int                      `json:"blobKeysChecked"`
	Mismatches      []string                 `json:"mismatches"`
	Missing         []string                 `json:"missing"`
	Domains         map[string]*DomainReport `json:"domains"`
	Counts          map[string]int           `json:"counts"`
	OwnerViolations []string                 `json:"ownerViolations,omitempty"`
	CheckedAt       time.Time                `json:"checkedAt"`
}

// OK reports whether reconciliation found nothing wrong.
func (r *Report) OK() bool {
	return len(r.Mismatches) == 0 && len(r.Missing) == 0 && len(r.OwnerViolations) == 0 && r.RunVerified
}

// Reconcile reads everything back: every ledger item is compared with the
// target row by semantic digest, every blob by re-downloading it, and every
// table is counted against the ledger.
func Reconcile(ctx context.Context, opts ReconcileOptions) (*Report, error) {
	if opts.Pool == nil || opts.Manifest == nil {
		return nil, errors.New("thiếu manifest hoặc kết nối")
	}
	report := &Report{
		SourceID: opts.SourceID, ManifestSHA256: opts.Manifest.SHA256,
		Domains: map[string]*DomainReport{}, Counts: map[string]int{},
		CheckedAt: time.Now().UTC(),
	}
	pool := opts.Pool.Pool()
	if err := verifyRun(ctx, pool, opts.SourceID, opts.Manifest.SHA256, report); err != nil {
		return nil, err
	}
	items, err := loadLedger(ctx, pool, opts.SourceID)
	if err != nil {
		return nil, err
	}
	byPath := map[string][]ledgerItem{}
	for _, item := range items {
		byPath[item.path] = append(byPath[item.path], item)
	}
	// 1. every manifest file has ledger rows with matching raw hashes.
	for _, f := range opts.Manifest.Files {
		report.FilesChecked++
		rows := byPath[f.Path]
		if len(rows) == 0 {
			report.Missing = append(report.Missing, "thiếu ledger cho "+f.Path)
			continue
		}
		found := false
		for _, row := range rows {
			if row.rawSHA == f.SHA256 {
				found = true
			}
		}
		if !found {
			report.Mismatches = append(report.Mismatches, "raw hash lệch: "+f.Path)
		}
	}
	// 2. every imported item reloads to the same semantic digest.
	im := &importer{scan: &ScanResult{SourceID: opts.SourceID}, ids: &DecodeContext{SourceID: opts.SourceID, Projects: map[string]string{}, Stories: map[string]string{}, Users: map[string]string{}}}
	scopes, err := loadScopes(ctx, pool, opts.SourceID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.disposition != DispImported {
			continue
		}
		report.ItemsChecked++
		domain := domainReport(report, item.domain)
		domain.Records++
		if item.kind == "" {
			// File-level accounting row: no target row to verify.
			continue
		}
		decoded, err := item.toItem()
		if err != nil {
			report.Mismatches = append(report.Mismatches, item.path+": "+err.Error())
			continue
		}
		scope := scopes[item.path]
		loaded, err := im.loadItem(ctx, pool, decoded, scope)
		if err != nil {
			report.Mismatches = append(report.Mismatches, fmt.Sprintf("%s/%s: %v", decoded.Kind, decoded.Key, err))
			domain.Mismatches = append(domain.Mismatches, decoded.Kind+"/"+decoded.Key)
			continue
		}
		digest, err := ItemDigest(decoded.Kind, loaded)
		if err != nil {
			return nil, err
		}
		if digest != item.semanticSHA {
			detail := ""
			if debugDiff() {
				want, _ := CanonicalJSON(digestPayload(decoded.Kind, decoded.Payload))
				got, _ := CanonicalJSON(digestPayload(decoded.Kind, loaded))
				detail = " [" + diffHint(string(want), string(got)) + "]"
			}
			report.Mismatches = append(report.Mismatches, fmt.Sprintf("%s/%s: digest lệch%s", decoded.Kind, decoded.Key, detail))
			domain.Mismatches = append(domain.Mismatches, decoded.Kind+"/"+decoded.Key)
		}
	}
	// 3. blob keys: every registry key must parse under the active scheme with
	// the same environment/project/digest the database records. A key is an
	// immutable identifier: if it drifts, restore and GC point at the wrong
	// object, so this check runs even without --verify-blobs.
	if err := verifyBlobKeys(ctx, pool, opts.Keys, report); err != nil {
		return nil, err
	}
	// 4. blobs: re-download and hash when asked (initial migration requires it).
	if opts.VerifyBlobs {
		for _, item := range items {
			if item.disposition != DispBlob {
				continue
			}
			// The ledger row carries no scope; reconstruct it from the same
			// path→scope map the item checks above use. Without this the check
			// compares every key against the zero scope and rejects all blobs.
			item.scope = scopes[item.path]
			report.BlobsChecked++
			if err := verifyBlob(ctx, pool, opts.Blobs, opts.Keys, item); err != nil {
				report.Mismatches = append(report.Mismatches, item.path+": "+err.Error())
				continue
			}
			report.BlobsVerified++
		}
	}
	// 5. table counts against the ledger, per target kind.
	counts, err := countTables(ctx, pool, opts.SourceID, scopes)
	if err != nil {
		return nil, err
	}
	for kind, n := range counts {
		report.Counts[kind] = n
	}
	for _, item := range items {
		if item.disposition != DispImported && item.disposition != DispBlob {
			continue
		}
		report.Counts["ledger:"+item.kind]++
	}
	for kind, expected := range report.Counts {
		if !strings.HasPrefix(kind, "ledger:") {
			continue
		}
		target := strings.TrimPrefix(kind, "ledger:")
		if target == "chapter" {
			// Chapters are verified by coverage below (a chapter can also be
			// created from legacy prose without a record file).
			continue
		}
		actual, ok := report.Counts[target]
		if !ok {
			continue
		}
		if actual != expected {
			report.Mismatches = append(report.Mismatches,
				fmt.Sprintf("số dòng %s: nguồn %d, DB %d", target, expected, actual))
		}
	}
	// 5. every stored chapter is covered by a source record or by imported
	// prose: a count match alone would hide a swapped chapter.
	covered, err := chapterCoverage(ctx, pool, items, scopes)
	if err != nil {
		return nil, err
	}
	report.Mismatches = append(report.Mismatches, covered...)
	// 6. exactly-one-owner invariant per project.
	owners, err := ownerViolations(ctx, pool, opts.SourceID)
	if err != nil {
		return nil, err
	}
	report.OwnerViolations = owners
	return report, nil
}

func debugDiff() bool { return os.Getenv("STORAGEIMPORT_DEBUG") == "1" }

// diffHint shows the first differing region of two canonical payloads; it
// never prints more than a short window so a report stays safe to paste.
func diffHint(want, got string) string {
	i := 0
	for i < len(want) && i < len(got) && want[i] == got[i] {
		i++
	}
	start := i - 60
	if start < 0 {
		start = 0
	}
	cut := func(s string) string {
		if i+60 < len(s) {
			return s[start : i+60]
		}
		return s[start:]
	}
	return "nguồn… " + cut(want) + " | db… " + cut(got)
}

func domainReport(report *Report, domain string) *DomainReport {
	if report.Domains[domain] == nil {
		report.Domains[domain] = &DomainReport{Tables: map[string]int{}}
	}
	return report.Domains[domain]
}

func verifyRun(ctx context.Context, q querier, sourceID, manifestSHA string, report *Report) error {
	var status string
	err := q.QueryRow(ctx, `SELECT status FROM import_runs WHERE id=$1`,
		uuidArg(RunIDFor(sourceID, manifestSHA))).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		report.Missing = append(report.Missing, "chưa có import_run cho manifest này")
		return nil
	}
	if err != nil {
		return err
	}
	report.RunVerified = status == "verified"
	if !report.RunVerified {
		report.Mismatches = append(report.Mismatches, "import_run chưa verified (trạng thái "+status+")")
	}
	return nil
}

type ledgerItem struct {
	path, key, domain, disposition, kind, targetID string
	rawSHA, semanticSHA                            string
	scope                                          recordScope
}

func (l ledgerItem) toItem() (Item, error) {
	return decodeFromLedger(l.kind, l.key, l.targetID, l.path)
}

// decodeFromLedger rebuilds the payload shape from the ledger target kind. The
// loader ignores the payload value but needs the type; the target key carries
// everything else.
func decodeFromLedger(kind, key, targetID, sourcePath string) (Item, error) {
	item := Item{Kind: kind, Key: key, TargetID: targetID}
	switch kind {
	case "library":
		item.Payload = LibraryConfig{}
	case "user":
		item.Payload = AccountUser{}
	case "audit":
		item.Payload = AccountAudit{}
	case "project":
		item.Payload = ProjectRow{}
	case "document":
		docKind, logical, ok := strings.Cut(key, ":")
		if !ok {
			docKind, logical = "config", key
		}
		item.Payload = DocumentRow{Kind: docKind, LogicalName: logical}
	case "story":
		item.Payload = StoryRow{}
	case "chapter":
		item.Payload = ChapterRow{}
	case "chapter_prose":
		item.Payload = ChapterProseRow{}
	case "chapter_draft":
		item.Payload = ChapterDraftRow{}
	case "outline":
		item.Payload = OutlineRow{}
	case "character":
		item.Payload = CharacterRow{}
	case "state":
		item.Payload = StateRow{}
	case "artifact":
		// The target id encodes the row as "<kind>/<artifact key>".
		role, artifactKeyPart := splitArtifactKey(targetID)
		item.Payload = ArtifactRow{Kind: role, Key: artifactKeyPart, ContentType: ContentType(sourcePath)}
	case "comment":
		item.Payload = CommentRow{}
	case "review":
		item.Payload = ReviewRow{}
	case "checkpoint":
		item.Payload = CheckpointRow{}
	case "event":
		item.Payload = EventRow{}
	case "session":
		item.Payload = SessionRow{}
	case "audio_settings":
		item.Payload = AudioSettingsRow{}
	case "audio_cast":
		item.Payload = AudioCastRow{}
	case "audio_script":
		item.Payload = AudioScriptRow{}
	case "job":
		item.Payload = JobRow{}
	case "blob":
		item.Payload = BlobSpec{}
	default:
		return item, fmt.Errorf("target kind lạ %q", kind)
	}
	return item, nil
}

func splitArtifactKey(key string) (string, string) {
	role, rest, ok := strings.Cut(key, "/")
	if !ok || role == "" {
		return "import_artifact", key
	}
	return role, rest
}

// digest projections that must ignore values the database derives itself.
type chapterDigest struct {
	Number   int
	Title    string
	Current  int
	Versions []ChapterVersion
}

type jobDigest struct {
	Kind       string
	ID         string
	State      string
	IdemKey    string
	Input      json.RawMessage
	Result     json.RawMessage
	CreatedAt  time.Time
	FinishedAt *time.Time
}

type reviewDigest struct {
	Chapter int
	Round   int
	Payload json.RawMessage
}

type documentDigest struct {
	Kind        string
	LogicalName string
	Text        string
}

// commentProjection is the typed subset of a comment file record that is
// reproducible from the comments table; the raw JSON may carry legacy fields
// that the database does not, and those must not make reconciliation fail.
type commentProjection struct {
	Chapter    int        `json:"chapter"`
	Paragraph  *int       `json:"paragraph"`
	Quote      string     `json:"quote"`
	Chips      []string   `json:"chips"`
	Text       string     `json:"text"`
	Author     string     `json:"author"`
	CreatedAt  *time.Time `json:"createdAt"`
	Status     string     `json:"status"`
	SentAt     *time.Time `json:"sentAt"`
	FixedAt    *time.Time `json:"fixedAt"`
	ResolvedAt *time.Time `json:"resolvedAt"`
	FixedBy    string     `json:"fixedBy"`
	Unfixed    bool       `json:"unfixed"`
	Moved      bool       `json:"moved"`
	Relinked   bool       `json:"relinked"`
	Whole      bool       `json:"whole"`
	NewQuote   string     `json:"newQuote"`
}

func projectComment(raw json.RawMessage) (commentProjection, error) {
	var p commentProjection
	if len(raw) == 0 {
		return p, errors.New("comment rỗng")
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	if p.Chips == nil {
		p.Chips = []string{}
	}
	if p.CreatedAt != nil {
		trunc := p.CreatedAt.UTC().Truncate(time.Microsecond)
		p.CreatedAt = &trunc
	}
	return p, nil
}

type chapterProseDigest struct {
	Number int
	Text   string
}

type eventDigest struct {
	Stream  string
	LogKey  string
	Seq     int64
	Payload json.RawMessage
}

type chapterDraftDigest struct {
	Number int
	Text   string
}

func loadLedger(ctx context.Context, q querier, sourceID string) ([]ledgerItem, error) {
	rows, err := q.Query(ctx, `
SELECT relative_path, record_key, domain, disposition, target_kind, target_id, raw_sha256, semantic_sha256
FROM import_items WHERE source_id=$1 ORDER BY relative_path, record_key`, uuidArg(sourceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ledgerItem
	for rows.Next() {
		var l ledgerItem
		if err := rows.Scan(&l.path, &l.key, &l.domain, &l.disposition, &l.kind, &l.targetID, &l.rawSHA, &l.semanticSHA); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// loadScopes reconstructs project/story ids per source path from the ledger's
// target ids, so reconciliation does not depend on the snapshot being present.
func loadScopes(ctx context.Context, q querier, sourceID string) (map[string]recordScope, error) {
	scopes := map[string]recordScope{}
	// Story rows carry the project id; iterate stories of this source via the
	// story ledger items themselves.
	storyRows, err := q.Query(ctx, `
SELECT i.relative_path, st.id, st.project_id
FROM import_items i
JOIN stories st ON st.id::text = i.target_id
WHERE i.source_id = $1 AND i.target_kind = 'story'`, uuidArg(sourceID))
	if err != nil {
		return nil, err
	}
	defer storyRows.Close()
	type storyInfo struct{ dir, id, project string }
	var stories []storyInfo
	for storyRows.Next() {
		var s storyInfo
		if err := storyRows.Scan(&s.dir, &s.id, &s.project); err != nil {
			return nil, err
		}
		stories = append(stories, s)
	}
	if err := storyRows.Err(); err != nil {
		return nil, err
	}
	// story dir = "<project dir>/<stories dir>/<slug>"; every path below it
	// belongs to that story.
	paths, err := q.Query(ctx, `SELECT DISTINCT relative_path FROM import_items WHERE source_id=$1`, uuidArg(sourceID))
	if err != nil {
		return nil, err
	}
	defer paths.Close()
	var all []string
	for paths.Next() {
		var p string
		if err := paths.Scan(&p); err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	sort.Strings(all)
	for _, p := range all {
		best := ""
		var scope recordScope
		for _, s := range stories {
			dir := strings.TrimSuffix(s.dir, "/story.json")
			if strings.HasPrefix(p, dir+"/") && len(dir) > len(best) {
				best = dir
				scope = recordScope{ProjectID: s.project, StoryID: s.id}
			}
		}
		if best == "" {
			continue
		}
		scopes[p] = scope
	}
	// project-level paths: the project of the deepest matching story dir is
	// wrong, so resolve from project.json ledger rows instead.
	projRows, err := q.Query(ctx, `
SELECT i.relative_path, p.id FROM import_items i
JOIN projects p ON p.id::text = i.target_id
WHERE i.source_id=$1 AND i.target_kind='project'`, uuidArg(sourceID))
	if err != nil {
		return nil, err
	}
	defer projRows.Close()
	for projRows.Next() {
		var p, id string
		if err := projRows.Scan(&p, &id); err != nil {
			return nil, err
		}
		dir := ""
		if i := strings.LastIndex(p, "/"); i >= 0 {
			dir = p[:i]
		}
		for _, path := range all {
			if _, ok := scopes[path]; ok {
				continue
			}
			if dir == "" || strings.HasPrefix(path, dir+"/") {
				scopes[path] = recordScope{ProjectID: id}
			}
		}
	}
	return scopes, nil
}

// verifyBlobKeys re-parses registry keys and compares tenant/environment
// ownership. Published ready rows also have verified content digests. Deleting
// rows, including NULL-project orphan claims with sentinel metadata, are
// transient maintenance state and must never be reconciled as tenant content.
func verifyBlobKeys(ctx context.Context, q querier, keys blobkey.Keys, report *Report) error {
	rows, err := q.Query(ctx, `SELECT object_key, project_id::text, sha256, state FROM blobs WHERE state <> 'deleting'`)
	if err != nil {
		return fmt.Errorf("đọc khoá blob: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var objectKey, projectID, sha, state string
		if err := rows.Scan(&objectKey, &projectID, &sha, &state); err != nil {
			return err
		}
		report.BlobKeysChecked++
		obj, err := blobkey.Parse(objectKey)
		if err != nil {
			report.Mismatches = append(report.Mismatches, fmt.Sprintf("khoá %q: %v", objectKey, err))
			continue
		}
		switch {
		case obj.Env != keys.Env():
			report.Mismatches = append(report.Mismatches, fmt.Sprintf(
				"khoá %q thuộc môi trường %q, đang chạy %q", objectKey, obj.Env, keys.Env()))
		case obj.ProjectID != projectID:
			report.Mismatches = append(report.Mismatches, fmt.Sprintf(
				"khoá %q thuộc dự án %s nhưng row ghi %s", objectKey, obj.ProjectID, projectID))
		case state == "ready" && obj.SHA256 != sha:
			report.Mismatches = append(report.Mismatches, fmt.Sprintf(
				"khoá %q ghi sha %s nhưng row ghi %s", objectKey, obj.SHA256, sha))
		}
	}
	return rows.Err()
}

func verifyBlob(ctx context.Context, q querier, blobs blobstore.BlobStore, keys blobkey.Keys, item ledgerItem) error {
	if blobs == nil {
		return errors.New("thiếu BlobStore để xác minh")
	}
	var objectKey, sha string
	var size int64
	err := q.QueryRow(ctx, `SELECT object_key, sha256, size_bytes FROM blobs WHERE id=$1`,
		uuidArg(item.targetID)).Scan(&objectKey, &sha, &size)
	if err != nil {
		return fmt.Errorf("không thấy metadata blob: %w", err)
	}
	obj, err := blobkey.Parse(objectKey)
	if err != nil {
		return fmt.Errorf("khoá đối tượng không theo quy ước: %w", err)
	}
	if obj.Env != keys.Env() {
		return fmt.Errorf("khoá %q thuộc môi trường %q, đang chạy %q", objectKey, obj.Env, keys.Env())
	}
	if obj.Kind != blobkey.KindImport {
		return fmt.Errorf("khoá %q không phải blob nhập (kind %s)", objectKey, obj.Kind)
	}
	// Scope is checked against the authoritative blobs.project_id column by
	// verifyBlobKeys (path→scope mapping in the ledger is not authoritative:
	// the same source path can be attributed to a project- or story-level
	// scope depending on how the library was adopted).
	if obj.SHA256 != item.rawSHA {
		return fmt.Errorf("khoá %q ghi sha khác nguồn %s", objectKey, item.rawSHA)
	}
	rc, info, err := blobs.Open(ctx, objectKey, blobstore.FullRange)
	if err != nil {
		return fmt.Errorf("mở blob: %w", err)
	}
	defer func() { _ = rc.Close() }()
	digest, n, err := blobstore.DigestAll(rc)
	if err != nil {
		return fmt.Errorf("đọc blob: %w", err)
	}
	if digest != sha || n != size || sha != item.rawSHA {
		return fmt.Errorf("blob lệch: db=%s/%d tải=%s/%d nguồn=%s", sha, size, digest, n, item.rawSHA)
	}
	_ = info
	return nil
}

func countTables(ctx context.Context, q querier, sourceID string, scopes map[string]recordScope) (map[string]int, error) {
	projects := map[string]bool{}
	stories := map[string]bool{}
	for _, s := range scopes {
		if s.ProjectID != "" {
			projects[s.ProjectID] = true
		}
		if s.StoryID != "" {
			stories[s.StoryID] = true
		}
	}
	ids := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for id := range m {
			out = append(out, id)
		}
		sort.Strings(out)
		return out
	}
	projectIDs := ids(projects)
	storyIDs := ids(stories)
	counts := map[string]int{}
	scoped := []struct {
		name, sql string
		args      []any
	}{
		{"users", `SELECT count(*) FROM users`, nil},
		{"audit", `SELECT count(*) FROM audit_logs WHERE actor_kind='import' OR details ? 'sourceType'`, nil},
		{"project", `SELECT count(*) FROM projects WHERE id = ANY($1)`, []any{uuidArray(projectIDs)}},
		{"document", `SELECT count(*) FROM project_documents WHERE project_id = ANY($1)`, []any{uuidArray(projectIDs)}},
		{"story", `SELECT count(*) FROM stories WHERE id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"chapters_total", `SELECT count(*) FROM chapters WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"outline", `SELECT count(*) FROM outlines WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"character", `SELECT count(*) FROM characters WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"state", `SELECT count(*) FROM story_state WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"artifact", `SELECT count(*) FROM story_artifacts WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"comment", `SELECT count(*) FROM comments WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"review", `SELECT count(*) FROM reviews WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"checkpoint", `SELECT count(*) FROM checkpoints WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"event", `SELECT count(*) FROM story_events WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"session", `SELECT count(*) FROM agent_sessions WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"audio_settings", `SELECT count(*) FROM audio_settings WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"audio_cast", `SELECT count(*) FROM audio_cast WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"audio_script", `SELECT count(*) FROM audio_scripts WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"job", `SELECT count(*) FROM jobs WHERE story_id = ANY($1)`, []any{uuidArray(storyIDs)}},
		{"blob", `SELECT count(*) FROM blobs WHERE project_id = ANY($1)`, []any{uuidArray(projectIDs)}},
	}
	for _, s := range scoped {
		var n int
		if err := q.QueryRow(ctx, s.sql, s.args...).Scan(&n); err != nil {
			return nil, fmt.Errorf("đếm %s: %w", s.name, err)
		}
		counts[s.name] = n
	}
	_ = sourceID
	return counts, nil
}

func uuidArray(ids []string) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, uuidArg(id))
	}
	return out
}

// chapterCoverage proves that every chapter row of an imported story is backed
// by a ledger item (record or legacy prose) with the same number.
func chapterCoverage(ctx context.Context, q querier, items []ledgerItem, scopes map[string]recordScope) ([]string, error) {
	want := map[string]map[int]bool{}
	for _, item := range items {
		// A chapter identity can come from an accepted record, workspace
		// prose, or a draft-only chapter (drafts/NN.draft.md).
		if item.kind != "chapter" && item.kind != "chapter_prose" && item.kind != "chapter_draft" {
			continue
		}
		scope := scopes[item.path]
		if scope.StoryID == "" {
			continue
		}
		n, err := strconv.Atoi(item.key)
		if err != nil {
			n = strconvInt(item.targetID)
		}
		if want[scope.StoryID] == nil {
			want[scope.StoryID] = map[int]bool{}
		}
		want[scope.StoryID][n] = true
	}
	var out []string
	for storyID, numbers := range want {
		rows, err := q.Query(ctx, `SELECT number FROM chapters WHERE story_id=$1 ORDER BY number`, uuidArg(storyID))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var n int
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return nil, err
			}
			if !numbers[n] {
				out = append(out, fmt.Sprintf("chương %d của truyện %s không có bản ghi nguồn", n, storyID))
			}
		}
		rows.Close()
	}
	return out, nil
}

func ownerViolations(ctx context.Context, q querier, sourceID string) ([]string, error) {
	rows, err := q.Query(ctx, `
SELECT p.slug, count(m.user_id)
FROM projects p
LEFT JOIN project_members m ON m.project_id = p.id AND m.role='owner' AND m.removed_at IS NULL
WHERE p.id IN (SELECT target_id::uuid FROM import_items WHERE source_id=$1 AND target_kind='project')
GROUP BY p.slug
ORDER BY p.slug`, uuidArg(sourceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var slug string
		var n int
		if err := rows.Scan(&slug, &n); err != nil {
			return nil, err
		}
		if n != 1 {
			out = append(out, fmt.Sprintf("dự án %q có %d owner (cần đúng 1)", slug, n))
		}
	}
	return out, rows.Err()
}
