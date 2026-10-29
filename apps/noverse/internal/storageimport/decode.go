package storageimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// nilBytes is the length of a JSON "null" literal.
const nilBytes = 4

// ProjectMeta mirrors <project>/project.json (library.ProjectFile).
type ProjectMeta struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Notes        string     `json:"notes"`
	CreatedAt    *time.Time `json:"createdAt"`
	UniverseName string     `json:"universeName,omitempty"`
}

// StoryMeta mirrors <story>/story.json (library.StoryFile).
type StoryMeta struct {
	Title        string     `json:"title"`
	ShortTitle   string     `json:"shortTitle,omitempty"`
	TitleRenamed bool       `json:"titleRenamed,omitempty"`
	Status       string     `json:"status"`
	Idea         string     `json:"idea"`
	Tags         []string   `json:"tags"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    *time.Time `json:"createdAt"`
}

// AccountFile is web-accounts.json (version 3/4). The importer only reads the
// fields it preserves; unknown fields are kept in Raw for the ledger digest.
type AccountFile struct {
	Version  int            `json:"version"`
	Users    []AccountUser  `json:"users"`
	Sessions []AccountToken `json:"sessions"`
	Pairings []AccountToken `json:"pairings"`
	Audit    []AccountAudit `json:"audit"`
}

// AccountUser is one account row with its encoded credentials.
type AccountUser struct {
	Username           string               `json:"username"`
	Name               string               `json:"name"`
	Role               string               `json:"role"`
	Projects           map[string]string    `json:"projects"`
	Disabled           bool                 `json:"disabled"`
	PasswordHash       string               `json:"passwordHash"`
	OnboardedAt        *time.Time           `json:"onboardedAt"`
	MustChangePassword bool                 `json:"mustChangePassword"`
	DisabledAt         *time.Time           `json:"disabledAt,omitempty"`
	DisabledBy         string               `json:"disabledBy,omitempty"`
	DisabledReason     string               `json:"disabledReason,omitempty"`
	PasswordChangedAt  *time.Time           `json:"passwordChangedAt,omitempty"`
	TempPassword       *AccountTempPassword `json:"tempPassword,omitempty"`
	DeletedAt          *time.Time           `json:"deletedAt,omitempty"`
	Recovery           *AccountRecovery     `json:"recovery,omitempty"`
	CreatedAt          time.Time            `json:"createdAt"`
	UpdatedAt          time.Time            `json:"updatedAt"`
}

// AccountTempPassword is the one-time temporary credential (file v4).
type AccountTempPassword struct {
	CodeHash  string     `json:"codeHash"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// AccountRecovery is the one-time recovery code (hash + mask only).
type AccountRecovery struct {
	CodeHash  string     `json:"codeHash"`
	Mask      string     `json:"mask,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
}

// AccountToken is a session/pairing. The importer counts them as revoked on
// cutover; it never copies a bearer secret into the database.
type AccountToken struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// AccountAudit mirrors webauth.AuditEntry.
type AccountAudit struct {
	ID      string          `json:"id"`
	At      time.Time       `json:"at"`
	Actor   json.RawMessage `json:"actor"`
	Type    string          `json:"type"`
	Action  string          `json:"action"`
	Target  string          `json:"target"`
	Project string          `json:"project,omitempty"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Reason  string          `json:"reason,omitempty"`
}

// ProjectRow is the projects table row derived from project.json.
type ProjectRow struct {
	ID           string          `json:"id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Notes        string          `json:"notes"`
	UniverseName string          `json:"universe"`
	Settings     json.RawMessage `json:"settings"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// DocumentRow is a project_documents row.
type DocumentRow struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"projectId"`
	Kind        string          `json:"kind"`
	LogicalName string          `json:"logicalName"`
	Text        string          `json:"text,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// StoryRow is the stories table row derived from story.json.
type StoryRow struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"projectId"`
	Slug         string     `json:"slug"`
	Title        string     `json:"title"`
	ShortTitle   string     `json:"shortTitle"`
	Synopsis     string     `json:"synopsis"`
	TitleRenamed bool       `json:"titleRenamed"`
	Status       string     `json:"status"`
	Tags         []string   `json:"tags"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ChapterRow is a chapters row plus every known version and the workspace
// prose that diverged from the accepted record (kept as draft, never as an
// accepted version).
type ChapterRow struct {
	Number   int              `json:"number"`
	Title    string           `json:"title"`
	Draft    string           `json:"draft"`
	Current  int              `json:"currentRevision"`
	Versions []ChapterVersion `json:"versions"`
}

// ChapterVersion is one immutable chapter version. Facts and StyleDelta keep
// the source bytes verbatim: they are the structured projection input, so a
// normalizing re-encode must never silently drop a field.
type ChapterVersion struct {
	Revision   int             `json:"revision"`
	Content    string          `json:"content"`
	SHA256     string          `json:"sha256"`
	Origin     string          `json:"origin"`
	Build      string          `json:"build"`
	AcceptedAt time.Time       `json:"acceptedAt"`
	Facts      json.RawMessage `json:"facts,omitempty"`
	StyleDelta json.RawMessage `json:"styleDelta,omitempty"`
}

// ChapterDraftRow is the workspace draft of one chapter (drafts/NN.draft.md).
type ChapterDraftRow struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// OutlineRow is an outlines row (flat/layered/premise/compass/retention).
type OutlineRow struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Text    string          `json:"text,omitempty"`
}

// StateRow is a story_state row.
type StateRow struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// ArtifactRow is a story_artifacts row (immutable source copy of artifacts
// that have no normalized table).
type ArtifactRow struct {
	Kind        string          `json:"kind"`
	Key         string          `json:"key"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Text        string          `json:"text,omitempty"`
	ContentType string          `json:"contentType,omitempty"`
}

// CommentRow is a comments row.
type CommentRow struct {
	Chapter int             `json:"chapter"`
	Raw     json.RawMessage `json:"raw"`
}

// ReviewRow is a reviews row.
type ReviewRow struct {
	Chapter int             `json:"chapter"`
	Round   int             `json:"round"`
	Scope   string          `json:"scope"`
	Payload json.RawMessage `json:"payload"`
}

// ReviewDecisionRow is a review_decisions row.
type ReviewDecisionRow struct {
	Chapter int             `json:"chapter"`
	Round   int             `json:"round"`
	ItemKey string          `json:"itemKey"`
	Payload json.RawMessage `json:"payload"`
}

// CheckpointRow is a checkpoints row decoded from checkpoints.jsonl.
type CheckpointRow struct {
	Seq        int64           `json:"seq"`
	Scope      json.RawMessage `json:"scope"`
	Step       string          `json:"step"`
	Artifact   string          `json:"artifact"`
	Digest     string          `json:"digest"`
	OccurredAt time.Time       `json:"occurredAt"`
	Raw        json.RawMessage `json:"raw"`
}

// EventRow is a story_events row decoded from an append-only JSONL stream.
// LogKey keeps independent logs of one stream apart (a runtime task log, the
// cocreate session transcript), matching the file backend's per-file logs.
type EventRow struct {
	Stream  string          `json:"stream"`
	LogKey  string          `json:"logKey,omitempty"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload"`
	RawSHA  string          `json:"rawSha"`
}

// SessionRow is an agent_sessions row decoded from a session JSONL file.
type SessionRow struct {
	Agent   string          `json:"agent"`
	TaskKey string          `json:"taskKey"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload"`
}

// AudioSettingsRow is the audio_settings payload.
type AudioSettingsRow struct {
	Payload json.RawMessage `json:"payload"`
}

// AudioCastRow is one audio_cast role row.
type AudioCastRow struct {
	RoleKey string          `json:"roleKey"`
	Payload json.RawMessage `json:"payload"`
}

// AudioScriptRow is an audio_scripts row (+ segments) from ban-doc.
type AudioScriptRow struct {
	Chapter      int             `json:"chapter"`
	Revision     int             `json:"revision"`
	SourceRev    int             `json:"sourceRevision"`
	SourceSHA    string          `json:"sourceSha"`
	Status       string          `json:"status"`
	Payload      json.RawMessage `json:"payload"`
	SegmentCount int             `json:"segments"`
}

// JobRow is a jobs row derived from audio queue.json / export-requests.json.
type JobRow struct {
	Kind       string          `json:"kind"`
	ID         string          `json:"id"`
	State      string          `json:"state"`
	IdemKey    string          `json:"idempotencyKey"`
	Input      json.RawMessage `json:"input"`
	Result     json.RawMessage `json:"result,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	FinishedAt *time.Time      `json:"finishedAt,omitempty"`
	// Recovered marks a running job that came back as queued.
	Recovered bool `json:"recovered"`
}

// BlobSpec is a binary file destined for BlobStore.
type BlobSpec struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	ProjectID   string `json:"projectId"`
	StoryID     string `json:"storyId,omitempty"`
	ObjectKey   string `json:"objectKey"`
	// Role is the source role (e.g. audio_chapter, export, asset).
	Role string `json:"role"`
}

// Item is one classified source record. A file can yield many items (one per
// chapter record, comment, stream line…); every item becomes an import_items
// row and at most one target row.
type Item struct {
	Domain   string
	Kind     string
	Key      string
	TargetID string
	Payload  any
	// Text, when set, is the normalized text payload for digest purposes
	// (markdown/premise). Payload still drives the write.
	Text        string
	Blob        *BlobSpec
	Disposition string
	Reason      string
}

// DecodeContext carries everything a decoder needs to resolve ids.
type DecodeContext struct {
	SourceID string
	Library  LibraryConfig
	Projects map[string]string // project slug -> project uuid
	Stories  map[string]string // "<project>/<story>" -> story uuid
	Users    map[string]string // username -> user uuid
}

// ProjectID resolves the project uuid for a story dir.
func (c *DecodeContext) ProjectID(slug string) string { return c.Projects[slug] }

// StoryID resolves the story uuid for a project/story pair.
func (c *DecodeContext) StoryID(project, story string) string {
	return c.Stories[project+"/"+story]
}

// DecodeFile turns one manifest entry into items. Unknown roles are returned
// as an error so the caller records the path as quarantined instead of
// pretending it was imported.
func DecodeFile(ctx *DecodeContext, ref StoryRef, role, rel string, data []byte, mod time.Time) ([]Item, error) {
	switch role {
	case "library_config":
		return decodeLibrary(ctx, data)
	case "accounts":
		return DecodeAccounts(ctx, data)
	case "project_meta":
		return decodeProject(ctx, ref, data)
	case "story_meta":
		return decodeStory(ctx, ref, data)
	case "project_config":
		return decodeProjectConfig(ctx, ref, data)
	case "project_rules":
		return decodeDocument(ctx, ref, "rules", projectRel(ref, rel), data)
	case "story_rules":
		return decodeDocument(ctx, ref, "rules", projectRel(ref, rel), data)
	case "pronunciation":
		return decodeDocument(ctx, ref, "config", ".ainovel/cach-doc.json", data)
	case "project_draft":
		return decodeDocument(ctx, ref, "idea", ideaID(rel), data)
	case "idea":
		return decodeDocument(ctx, ref, "idea", ideaID(rel), data)
	case "voice_index":
		return decodeDocument(ctx, ref, "asset", projectRel(ref, rel), data)
	case "export_meta":
		return decodeArtifact(ctx, ref, "export_manifest", rel, data)
	case "story_config":
		return decodeArtifact(ctx, ref, "config", rel, data)
	case "engine_book":
		return decodeState(ctx, ref, "book", data)
	case "engine_format":
		return decodeState(ctx, ref, "format", data)
	case "engine_progress":
		return decodeState(ctx, ref, "progress", data)
	case "engine_run":
		return decodeState(ctx, ref, "run_meta", data)
	case "engine_runtime":
		return decodeArtifact(ctx, ref, "runtime", rel, data)
	case "engine_pending":
		return decodePending(ctx, ref, rel, data)
	case "chapter_record":
		return decodeChapterRecord(ctx, ref, rel, data, mod)
	case "chapter_prose":
		return decodeChapterProse(ctx, ref, rel, data, mod)
	case "outline_flat":
		return decodeOutline(ctx, ref, "flat", rel, data)
	case "outline_layered":
		return decodeOutline(ctx, ref, "layered", rel, data)
	case "outline_premise":
		return decodeOutlineText(ctx, ref, "premise", rel, data)
	case "characters":
		return decodeCharacters(ctx, ref, data)
	case "world_rules":
		return decodeState(ctx, ref, "world_rules", data)
	case "foreshadow":
		return decodeState(ctx, ref, "foreshadow", data)
	case "address_terms":
		return decodeState(ctx, ref, "address_terms", data)
	case "relationship_state":
		return decodeState(ctx, ref, "relationships", data)
	case "engine_doc":
		return decodeEngineDoc(ctx, ref, rel, data)
	case "usage_doc":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "checkpoint_stream":
		return decodeCheckpoints(ctx, ref, data)
	case "decision_stream":
		return decodeJSONLStream(rel, "decisions", data)
	case "rewrite_stream":
		return decodeJSONLStream(rel, "rewrite_history", data)
	case "violation_stream":
		return decodeJSONLStream(rel, "rule_violations", data)
	case "world_change_stream":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "relationship_stream":
		return decodeJSONLStream(rel, "relationships", data)
	case "timeline_stream":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "session_stream":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "snapshot":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "draft":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "summary":
		return decodeStoryDocument(ref, engineRel(ref, rel), data)
	case "comments":
		return decodeComments(ctx, ref, rel, data)
	case "comment_events":
		return decodeCommentEvents(ctx, ref, data)
	case "review_latest":
		return decodeReview(ctx, ref, rel, data, false)
	case "review_round":
		return decodeReview(ctx, ref, rel, data, true)
	case "review_decisions":
		return decodeReviewDecisions(ctx, ref, data)
	case "audio_settings":
		return decodeAudioSettings(ref, data)
	case "audio_cast":
		return decodeAudioCast(ref, data)
	case "audio_script":
		return decodeAudioScript(ref, rel, data)
	case "audio_chapter_meta":
		return decodeArtifact(ctx, ref, "audio_chapter", rel, data)
	case "audio_queue":
		return decodeAudioQueue(ctx, ref, data)
	case "audio_export_requests":
		return decodeExportRequests(ctx, ref, data)
	case "import_artifact":
		return decodeArtifact(ctx, ref, "import_artifact", rel, data)
	case "audio_flags":
		return decodeArtifact(ctx, ref, "audio_flags", rel, data)
	case "story_file":
		return decodeArtifact(ctx, ref, "story_file", rel, data)
	case "story_document":
		return decodeArtifact(ctx, ref, "story_document", rel, data)
	case "audio_file":
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%s: file nhị phân không có quy tắc blob", rel)
		}
		return decodeArtifact(ctx, ref, "audio_file", rel, data)
	case "engine_file":
		return decodeArtifact(ctx, ref, "engine_file", rel, data)
	case "project_file":
		return decodeArtifact(ctx, ref, "project_file", rel, data)
	}
	return nil, fmt.Errorf("role %q chưa có decoder", role)
}

// ── accounts ────────────────────────────────────────────────────────────────

// DecodeAccounts decodes web-accounts.json (v1..v4). Older versions stored
// books/writer/reader maps; only users, credentials, memberships and audit are
// facts the database needs, and every one of them is preserved as-is.
func DecodeAccounts(ctx *DecodeContext, data []byte) ([]Item, error) {
	var file AccountFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("web-accounts.json hỏng: %w", err)
	}
	if len(file.Users) == 0 {
		return nil, fmt.Errorf("web-accounts.json không có tài khoản nào")
	}
	var items []Item
	seen := map[string]bool{}
	for _, u := range file.Users {
		if seen[strings.ToLower(u.Username)] {
			return nil, fmt.Errorf("username trùng không phân biệt hoa thường: %q", u.Username)
		}
		seen[strings.ToLower(u.Username)] = true
		if u.Role != "admin" && u.Role != "member" {
			return nil, fmt.Errorf("tài khoản %q có role lạ %q", u.Username, u.Role)
		}
		if u.PasswordHash != "" && !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
			return nil, fmt.Errorf("tài khoản %q có hash không phải Argon2id", u.Username)
		}
		id := ctx.Users[u.Username]
		items = append(items, Item{
			Domain: DomainAccounts, Kind: "user", Key: u.Username, TargetID: id, Payload: u,
		})
	}
	for _, a := range file.Audit {
		payload := a
		items = append(items, Item{
			Domain: DomainAccounts, Kind: "audit", Key: a.ID,
			TargetID: DeterministicUUID(ctx.SourceID, "audit", a.ID), Payload: payload,
		})
	}
	// Sessions and pairings are deliberately not imported; the summary reports
	// how many bearer credentials were invalidated at cutover.
	return items, nil
}

// ── library / project / story ────────────────────────────────────────────────

func decodeLibrary(ctx *DecodeContext, data []byte) ([]Item, error) {
	var cfg LibraryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("library.json hỏng: %w", err)
	}
	cfg.Dirs = cfg.Dirs.withDefaults()
	return []Item{{
		Domain: DomainLibrary, Kind: "library", Key: "library",
		TargetID: DeterministicUUID(ctx.SourceID, "library", "library"), Payload: cfg,
	}}, nil
}

func decodeProject(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	var meta ProjectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("project.json hỏng: %w", err)
	}
	slug := ref.ProjectSlug
	if slug == "" {
		slug = "_"
	}
	row := ProjectRow{
		ID: ctx.Projects[slug], Slug: slug, Name: meta.Name,
		Description: meta.Description, Notes: meta.Notes,
		UniverseName: meta.UniverseName,
		Settings:     json.RawMessage(`{}`),
		CreatedAt:    tsOr(TimeOrZero(meta.CreatedAt)),
		UpdatedAt:    tsOr(TimeOrZero(meta.CreatedAt)),
	}
	return []Item{{Domain: DomainProject, Kind: "project", Key: slug, TargetID: row.ID, Payload: row}}, nil
}

func decodeStory(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	var meta StoryMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("story.json hỏng: %w", err)
	}
	switch meta.Status {
	case "idea", "writing", "paused", "done", "published":
	case "":
		meta.Status = "idea"
	default:
		return nil, fmt.Errorf("story.json có status lạ %q", meta.Status)
	}
	id := ctx.Stories[ref.ProjectSlug+"/"+ref.Slug]
	row := StoryRow{
		ID: id, ProjectID: ctx.Projects[ref.ProjectSlug], Slug: ref.Slug,
		Title: meta.Title, ShortTitle: meta.ShortTitle, TitleRenamed: meta.TitleRenamed,
		Status: meta.Status, Tags: stringSlice(meta.Tags), PublishedAt: meta.PublishedAt,
		CreatedAt: tsOr(TimeOrZero(meta.CreatedAt)), UpdatedAt: tsOr(TimeOrZero(meta.CreatedAt)),
	}
	return []Item{{Domain: DomainStory, Kind: "story", Key: ref.Slug, TargetID: id, Payload: row}}, nil
}

func decodeProjectConfig(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	return decodeDocument(ctx, ref, "config", ".ainovel/config.json", data)
}

// decodeDocument stores a project document under its project-relative logical
// path. The reverse export writes that path under the project dir verbatim, so
// the source location survives the round trip.
func decodeDocument(ctx *DecodeContext, ref StoryRef, kind, logical string, data []byte) ([]Item, error) {
	return []Item{{
		Domain: DomainProject, Kind: "document", Key: kind + ":" + logical,
		Payload:  DocumentRow{Kind: kind, LogicalName: logical, Text: string(data)},
		TargetID: DeterministicUUID(ctx.SourceID, "document", ref.ProjectSlug+"/"+kind+"/"+logical),
	}}, nil
}

// ideaID is the library's idea identity: the file name without extension and
// without any directory (project_documents.kind='idea' rejects path ids).
func ideaID(rel string) string {
	return strings.TrimSuffix(path.Base(rel), ".md")
}

// projectRel returns a path relative to its project directory. The anonymous
// project "_" owns the library root, so its paths are already relative.
func projectRel(ref StoryRef, rel string) string {
	if ref.ProjectSlug == "" || ref.ProjectSlug == "_" {
		return rel
	}
	return strings.TrimPrefix(rel, ref.ProjectSlug+"/")
}

// engineRel returns a path relative to <story>/output/novel, the base the
// production document mapping (postgres.ResolveDocument) speaks. Paths outside
// the engine directory stay story-relative so no source directory name leaks
// into a stored key.
func engineRel(ref StoryRef, rel string) string {
	if ref.Dir != "" {
		if rest, ok := strings.CutPrefix(rel, ref.Dir+"/output/novel/"); ok {
			return rest
		}
		if rest, ok := strings.CutPrefix(rel, ref.Dir+"/"); ok {
			return rest
		}
	}
	return rel
}

// ── engine ───────────────────────────────────────────────────────────────────

func decodeState(_ *DecodeContext, ref StoryRef, kind string, data []byte) ([]Item, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s không phải JSON hợp lệ", kind)
	}
	return []Item{{
		Domain: DomainEngine, Kind: "state", Key: kind,
		Payload: StateRow{Kind: kind, Payload: json.RawMessage(data)},
	}}, nil
}

func decodePending(ctx *DecodeContext, ref StoryRef, rel string, data []byte) ([]Item, error) {
	name := strings.TrimSuffix(path.Base(rel), ".json")
	kind := "pending_commit"
	if strings.Contains(name, "revision") {
		kind = "pending_revision"
	}
	return decodeState(ctx, ref, kind, data)
}

func decodeEngineDoc(ctx *DecodeContext, ref StoryRef, rel string, data []byte) ([]Item, error) {
	name := path.Base(rel)
	switch name {
	case "compass.json":
		return decodeOutline(ctx, ref, "compass", rel, data)
	case "story_retention.json":
		return decodeOutline(ctx, ref, "retention", rel, data)
	case "user_rules.json":
		return decodeState(ctx, ref, "user_rules", data)
	case "foundation_audit.json":
		return decodeState(ctx, ref, "foundation_audit", data)
	case "cast_ledger.json":
		return decodeState(ctx, ref, "cast", data)
	case "style_rules.json":
		return decodeState(ctx, ref, "style_rules", data)
	case "signals.json":
		return decodeState(ctx, ref, "signals", data)
	case "usage.json":
		return decodeState(ctx, ref, "usage", data)
	}
	if items, err := decodeStoryDocument(ref, engineRel(ref, rel), data); err == nil {
		return items, nil
	}
	return decodeArtifact(ctx, ref, "engine_doc", rel, data)
}

func decodeOutline(_ *DecodeContext, ref StoryRef, kind, rel string, data []byte) ([]Item, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s không phải JSON hợp lệ", rel)
	}
	return []Item{{
		Domain: DomainEngine, Kind: "outline", Key: kind,
		Payload: OutlineRow{Kind: kind, Payload: json.RawMessage(data)},
	}}, nil
}

func decodeOutlineText(_ *DecodeContext, ref StoryRef, kind, rel string, data []byte) ([]Item, error) {
	return []Item{{
		Domain: DomainEngine, Kind: "outline", Key: kind,
		Payload: OutlineRow{Kind: kind, Text: string(data), Payload: json.RawMessage(`{}`)},
		Text:    string(data),
	}}, nil
}

// ChapterProseRow is workspace prose for a chapter (chapters/NN.md).
type ChapterProseRow struct {
	Number int       `json:"number"`
	Text   string    `json:"text"`
	Source string    `json:"source"`
	Mod    time.Time `json:"mod"`
}

// decodeChapterRecord reads meta/chapter_records/NNNNNN.json — the accepted
// chapter record and the only authority for accepted prose. Every field the
// file backend requires (digest, facts, style delta, acceptance time) is
// preserved, because revision projections consume them.
func decodeChapterRecord(_ *DecodeContext, ref StoryRef, rel string, data []byte, _ time.Time) ([]Item, error) {
	name := strings.TrimSuffix(path.Base(rel), ".json")
	number, err := strconv.Atoi(name)
	if err != nil {
		return nil, fmt.Errorf("%s: tên file không phải số chương", rel)
	}
	var rec struct {
		Version       int             `json:"version"`
		Chapter       int             `json:"chapter"`
		Revision      int             `json:"revision"`
		Build         string          `json:"build"`
		Origin        string          `json:"origin"`
		Content       string          `json:"content"`
		ContentSHA256 string          `json:"content_sha256"`
		Facts         json.RawMessage `json:"facts"`
		StyleDelta    json.RawMessage `json:"style_delta"`
		AcceptedAt    time.Time       `json:"accepted_at"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%s hỏng: %w", rel, err)
	}
	if rec.Chapter == 0 {
		rec.Chapter = number
	}
	if rec.Chapter != number {
		return nil, fmt.Errorf("%s: số chương trong record (%d) khác tên file", rel, rec.Chapter)
	}
	if rec.Version != 1 && rec.Version != domain.ChapterRecordVersion {
		return nil, fmt.Errorf("%s: phiên bản bản ghi %d không hỗ trợ", rel, rec.Version)
	}
	if rec.Revision <= 0 {
		return nil, fmt.Errorf("%s: revision không hợp lệ (%d)", rel, rec.Revision)
	}
	if rec.Content == "" {
		return nil, fmt.Errorf("%s: record không có nội dung", rel)
	}
	origin := normalizeOrigin(rec.Origin)
	if origin == "import" {
		return nil, fmt.Errorf("%s: origin %q không phải bản ghi file backend", rel, rec.Origin)
	}
	content := domain.NormalizeChapterContent(rec.Content)
	digest := domain.ChapterContentSHA256(rec.Content)
	if rec.ContentSHA256 != "" && rec.ContentSHA256 != digest {
		return nil, fmt.Errorf("%s: content_sha256 không khớp nội dung", rel)
	}
	if rec.AcceptedAt.IsZero() {
		return nil, fmt.Errorf("%s: thiếu accepted_at", rel)
	}
	title := firstHeading(content)
	if v := factsTitle(rec.Facts); v != "" {
		title = v
	}
	row := ChapterRow{
		Number: number, Title: title, Current: rec.Revision,
		Versions: []ChapterVersion{{
			Revision: rec.Revision, Content: content, SHA256: digest,
			Origin: origin, Build: rec.Build,
			AcceptedAt: rec.AcceptedAt.UTC().Truncate(time.Microsecond),
			Facts:      nonEmptyJSON(rec.Facts), StyleDelta: nonEmptyJSON(rec.StyleDelta),
		}},
	}
	return []Item{{
		Domain: DomainEngine, Kind: "chapter", Key: fmt.Sprintf("%06d", number),
		Payload: row, TargetID: strconv.Itoa(number),
	}}, nil
}

// factsTitle reads the accepted chapter title from the record facts.
func factsTitle(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var facts struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &facts); err != nil {
		return ""
	}
	return strings.TrimSpace(facts.Title)
}

// nonEmptyJSON keeps the source bytes and turns an absent object into the
// column default so a round-tripped digest stays stable.
func nonEmptyJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), raw...)
}

// decodeChapterProse reads the user-visible workspace prose. It is written
// under the key the Store reads (the chapter_text artifact) and, when no
// accepted record exists, becomes the chapter's baseline revision.
func decodeChapterProse(_ *DecodeContext, ref StoryRef, rel string, data []byte, mod time.Time) ([]Item, error) {
	name := strings.TrimSuffix(path.Base(rel), ".md")
	number, err := strconv.Atoi(name)
	if err != nil {
		return nil, fmt.Errorf("%s: tên file không phải số chương", rel)
	}
	items := []Item{{
		Domain: DomainEngine, Kind: "chapter_prose", Key: fmt.Sprintf("%06d", number),
		Payload: ChapterProseRow{Number: number, Text: string(data), Source: rel, Mod: mod},
		Text:    string(data), TargetID: strconv.Itoa(number),
	}}
	text, err := decodeStoryDocument(ref, fmt.Sprintf("chapters/%02d.md", number), data)
	if err != nil {
		return nil, err
	}
	return append(items, text...), nil
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		if line != "" {
			break
		}
	}
	return ""
}

func normalizeOrigin(origin string) string {
	if origin == "user" || origin == "import" {
		return origin
	}
	return "generated"
}

// decodeCharacters splits characters.json into one row per character with a
// stable legacy id, so relations/aliases survive the move.
func decodeCharacters(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	var list []map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("characters.json hỏng: %w", err)
	}
	items, err := decodeStoryDocument(ref, "characters.json", data)
	if err != nil {
		return nil, err
	}
	for i, entry := range list {
		legacy, _ := entry["id"].(string)
		if legacy == "" {
			name, _ := entry["name"].(string)
			if name == "" {
				legacy = fmt.Sprintf("import-%d", i)
			} else {
				legacy = name
			}
		}
		payload, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		items = append(items, Item{
			Domain: DomainEngine, Kind: "character", Key: legacy,
			Payload:  CharacterRow{LegacyID: legacy, Name: stringOf(entry["name"]), Payload: payload},
			TargetID: legacy,
		})
	}
	return items, nil
}

// CharacterRow is one characters row.
type CharacterRow struct {
	LegacyID string          `json:"legacyId"`
	Name     string          `json:"name"`
	Payload  json.RawMessage `json:"payload"`
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

// decodeStoryDocument maps one story document path onto the canonical row the
// production Store reads (postgres.ResolveDocument). It is the single mapping
// shared by import, reconciliation and export; a path the Store cannot read is
// an error, never a private import-only key.
func decodeStoryDocument(ref StoryRef, storyRel string, data []byte) ([]Item, error) {
	dr, err := postgres.ResolveDocument(storyRel)
	if err != nil {
		return nil, err
	}
	switch dr.Class {
	case postgres.DocClassArtifact:
		row := ArtifactRow{Kind: dr.Kind, Key: dr.Key, ContentType: ContentType(storyRel)}
		if dr.Text {
			row.Text = string(data)
		} else if json.Valid(data) {
			row.Payload = append(json.RawMessage(nil), data...)
		} else {
			row.Text = string(data)
		}
		return []Item{artifactItem(row)}, nil
	case postgres.DocClassState:
		if !json.Valid(data) {
			return nil, fmt.Errorf("%s không phải JSON hợp lệ", storyRel)
		}
		return []Item{{
			Domain: DomainEngine, Kind: "state", Key: dr.Kind,
			Payload: StateRow{Kind: dr.Kind, Payload: append(json.RawMessage(nil), data...)},
		}}, nil
	case postgres.DocClassOutline:
		if dr.Text {
			return decodeOutlineText(nil, ref, dr.Kind, storyRel, data)
		}
		return decodeOutline(nil, ref, dr.Kind, storyRel, data)
	case postgres.DocClassChapterDraft:
		return []Item{{
			Domain: DomainEngine, Kind: "chapter_draft", Key: strconv.Itoa(dr.Chapter),
			Payload:  ChapterDraftRow{Number: dr.Chapter, Text: string(data)},
			TargetID: strconv.Itoa(dr.Chapter),
		}}, nil
	case postgres.DocClassStream:
		return decodeStreamLines(dr.Kind, dr.Key, storyRel, data)
	case postgres.DocClassCheckpoints:
		return decodeCheckpointLines(storyRel, data)
	case postgres.DocClassAgentSession:
		return decodeAgentSessionLines(dr.Agent, dr.Key, storyRel, data)
	}
	return nil, fmt.Errorf("%s: %w", storyRel, errUnsupportedDocClass(dr.Class))
}

func errUnsupportedDocClass(class postgres.DocClass) error {
	return fmt.Errorf("lớp tài liệu %q không nhập được", class)
}

// artifactItem builds the one item shape for an artifact row. The record key
// is the canonical identity, so reruns and reconciliation address the same row.
func artifactItem(row ArtifactRow) Item {
	item := Item{
		Domain: DomainEngine, Kind: "artifact",
		Key:     row.Kind + "/" + row.Key,
		Payload: row, TargetID: row.Kind + "/" + row.Key,
	}
	if row.Text != "" {
		item.Text = row.Text
	}
	return item
}

// decodeStreamLines turns one JSONL document into story_events rows, each line
// at its line position. A truncated trailing line is tolerated and reported by
// the caller like every other stream.
func decodeStreamLines(stream, logKey, rel string, data []byte) ([]Item, error) {
	lines, tail := splitJSONL(data)
	items := make([]Item, 0, len(lines))
	for i, line := range lines {
		if !json.Valid(line) {
			return nil, fmt.Errorf("%s dòng %d không phải JSON", rel, i+1)
		}
		seq := int64(i + 1)
		items = append(items, Item{
			Domain: DomainStream, Kind: "event", Key: eventRecordKey(stream, logKey, seq),
			TargetID: eventRecordKey(stream, logKey, seq),
			Payload:  EventRow{Stream: stream, LogKey: logKey, Seq: seq, Payload: append(json.RawMessage(nil), line...), RawSHA: RawDigest(line)},
		})
	}
	if tail != nil {
		items = append(items, Item{
			Domain: DomainStream, Kind: "tolerated", Key: stream + ":tail",
			Disposition: DispTolerated,
			Reason:      "dòng JSONL cuối bị cắt trong " + rel,
		})
	}
	return items, nil
}

// eventRecordKey is the ledger identity of one stream line. The seq is the
// last slash-separated field, so a log key may itself contain slashes.
func eventRecordKey(stream, logKey string, seq int64) string {
	return fmt.Sprintf("%s/%s/%d", stream, logKey, seq)
}

// decodeAgentSessionLines turns one session transcript into agent_sessions
// rows under the agent/task pair the Store reads.
func decodeAgentSessionLines(agent, task, rel string, data []byte) ([]Item, error) {
	lines, tail := splitJSONL(data)
	items := make([]Item, 0, len(lines))
	for i, line := range lines {
		if !json.Valid(line) {
			return nil, fmt.Errorf("%s dòng %d không phải JSON", rel, i+1)
		}
		seq := int64(i + 1)
		key := agent + "/" + task + "/" + strconv.FormatInt(seq, 10)
		items = append(items, Item{
			Domain: DomainStream, Kind: "session", Key: key, TargetID: key,
			Payload: SessionRow{Agent: agent, TaskKey: task, Seq: seq, Payload: append(json.RawMessage(nil), line...)},
		})
	}
	if tail != nil {
		items = append(items, Item{
			Domain: DomainStream, Kind: "tolerated", Key: "session:" + path.Base(rel) + ":tail",
			Disposition: DispTolerated,
			Reason:      "dòng JSONL cuối bị cắt trong " + rel,
		})
	}
	return items, nil
}

// decodeArtifact preserves one file as a story artifact. JSON files keep their
// payload, everything else keeps its text; the raw hash in the ledger still
// covers the original bytes. Unmapped paths keep their source-relative key so
// the reverse export can write them back where they came from.
func decodeArtifact(ctx *DecodeContext, ref StoryRef, kind, rel string, data []byte) ([]Item, error) {
	if ref.Slug == "" {
		// Project-scoped file: project_documents is the only table with a
		// project (no story) scope, so it keeps the bytes verbatim.
		docKind := "config"
		switch kind {
		case "asset", "voice_index", "export_meta":
			docKind = "asset"
		}
		logical := projectRel(ref, rel)
		return []Item{{
			Domain: DomainProject, Kind: "document", Key: docKind + ":" + logical,
			Payload:  DocumentRow{Kind: docKind, LogicalName: logical, Text: string(data)},
			TargetID: DeterministicUUID(ctx.SourceID, "document", ref.ProjectSlug+"/"+docKind+"/"+logical),
		}}, nil
	}
	key := engineRel(ref, rel)
	row := ArtifactRow{Kind: kind, Key: key, ContentType: ContentType(rel)}
	if json.Valid(data) && strings.HasSuffix(rel, ".json") {
		row.Payload = json.RawMessage(data)
	} else {
		row.Text = string(data)
	}
	return []Item{artifactItem(row)}, nil
}

// decodeCheckpoints parses meta/checkpoints.jsonl. A trailing partial line is
// quarantined with a report entry instead of being guessed at.
func decodeCheckpoints(_ *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	return decodeCheckpointLines("meta/checkpoints.jsonl", data)
}

// decodeCheckpointLines parses one checkpoint log at its canonical location.
func decodeCheckpointLines(rel string, data []byte) ([]Item, error) {
	lines, tail := splitJSONL(data)
	items := make([]Item, 0, len(lines))
	for i, line := range lines {
		var raw struct {
			Seq        int64           `json:"seq"`
			Scope      json.RawMessage `json:"scope"`
			Step       string          `json:"step"`
			Artifact   string          `json:"artifact"`
			Digest     string          `json:"digest"`
			OccurredAt time.Time       `json:"occurred_at"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("%s dòng %d hỏng: %w", rel, i+1, err)
		}
		seq := raw.Seq
		if seq <= 0 {
			seq = int64(i + 1)
		}
		items = append(items, Item{
			Domain: DomainStream, Kind: "checkpoint", Key: strconv.FormatInt(seq, 10),
			TargetID: strconv.FormatInt(seq, 10),
			Payload: CheckpointRow{
				Seq: seq, Scope: raw.Scope, Step: raw.Step, Artifact: raw.Artifact,
				Digest: raw.Digest, OccurredAt: tsOr(raw.OccurredAt), Raw: append(json.RawMessage(nil), line...),
			},
		})
	}
	if tail != nil {
		items = append(items, Item{
			Domain: DomainStream, Kind: "tolerated", Key: "tail",
			Disposition: DispTolerated,
			Reason:      "dòng JSONL cuối bị cắt (partial write) — bỏ qua, đã ghi nhận",
		})
	}
	return items, nil
}

// decodeJSONLStream turns an append-only JSONL stream into story_events rows.
// Sequence numbers are line positions: the source streams carry no seq.
func decodeJSONLStream(rel, stream string, data []byte) ([]Item, error) {
	return decodeStreamLines(stream, "", rel, data)
}

// splitJSONL returns complete JSON lines and the trailing incomplete line (nil
// when the file ends with a newline or a complete JSON value).
func splitJSONL(data []byte) ([][]byte, []byte) {
	trimmed := bytes.TrimRight(data, "\n")
	if len(trimmed) == 0 {
		return nil, nil
	}
	rawLines := bytes.Split(trimmed, []byte("\n"))
	lines := make([][]byte, 0, len(rawLines))
	var tail []byte
	for i, line := range rawLines {
		line = bytes.TrimRight(line, "\r")
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if !json.Valid(line) {
			if i == len(rawLines)-1 {
				tail = append([]byte(nil), line...)
				break
			}
			// A corrupt middle line is not a partial tail: keep it as a
			// quarantined marker line so the count is honest.
			lines = append(lines, append([]byte(nil), line...))
			continue
		}
		lines = append(lines, append([]byte(nil), line...))
	}
	return lines, tail
}

// decodeComments reads <story>/.ainovel/gop-y/<n>.json (array of commentFile).
func decodeComments(_ *DecodeContext, ref StoryRef, rel string, data []byte) ([]Item, error) {
	name := strings.TrimSuffix(path.Base(rel), ".json")
	chapter, err := strconv.Atoi(name)
	if err != nil {
		return nil, fmt.Errorf("%s: tên file không phải số chương", rel)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s hỏng: %w", rel, err)
	}
	items := make([]Item, 0, len(raw))
	for _, entry := range raw {
		var meta struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(entry, &meta); err != nil {
			return nil, fmt.Errorf("%s: comment hỏng: %w", rel, err)
		}
		key := meta.ID
		if key == "" {
			key = RawDigest(entry)
		}
		items = append(items, Item{
			Domain: DomainComment, Kind: "comment", Key: key, TargetID: key,
			Payload: CommentRow{Chapter: chapter, Raw: entry},
		})
	}
	return items, nil
}

func decodeCommentEvents(_ *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("_events.json hỏng")
	}
	return []Item{{
		Domain: DomainComment, Kind: "state", Key: "comment_events",
		Payload: StateRow{Kind: "comment_events", Payload: json.RawMessage(data)},
	}}, nil
}

// decodeReview reads reviews/<n>.json (latest per chapter) or
// reviews/rounds/<n>-r<k>.json (one round).
func decodeReview(ctx *DecodeContext, ref StoryRef, rel string, data []byte, round bool) ([]Item, error) {
	base := strings.TrimSuffix(path.Base(rel), ".json")
	chapterPart := base
	roundNo := 1
	if round {
		n, r, ok := strings.Cut(base, "-r")
		if !ok {
			return nil, fmt.Errorf("%s: tên round không hợp lệ", rel)
		}
		chapterPart = n
		var err error
		roundNo, err = strconv.Atoi(r)
		if err != nil {
			return nil, fmt.Errorf("%s: số round không hợp lệ", rel)
		}
	}
	chapter, err := strconv.Atoi(chapterPart)
	if err != nil {
		return nil, fmt.Errorf("%s: tên file không phải số chương", rel)
	}
	var head struct {
		Scope string `json:"scope"`
		Round int    `json:"round"`
	}
	_ = json.Unmarshal(data, &head)
	if head.Round > 0 {
		roundNo = head.Round
	}
	scope := head.Scope
	if scope == "" {
		scope = "chapter"
	}
	key := fmt.Sprintf("chapter-%d-r%d", chapter, roundNo)
	if !round {
		key = fmt.Sprintf("chapter-%d-latest", chapter)
	}
	id := DeterministicUUID(ctx.SourceID, "review", ref.ProjectSlug+"/"+ref.Slug+"/"+key)
	return []Item{{
		Domain: DomainReview, Kind: "review", Key: key, TargetID: id,
		Payload: ReviewRow{Chapter: chapter, Round: roundNo, Scope: scope, Payload: json.RawMessage(data)},
	}}, nil
}

func decodeReviewDecisions(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	return decodeArtifact(ctx, ref, "review_decisions", "reviews/decisions.json", data)
}

// ── audio ────────────────────────────────────────────────────────────────────

func decodeAudioSettings(ref StoryRef, data []byte) ([]Item, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("audio/settings.json hỏng")
	}
	return []Item{{
		Domain: DomainAudio, Kind: "audio_settings", Key: "settings",
		Payload: AudioSettingsRow{Payload: json.RawMessage(data)},
	}}, nil
}

func decodeAudioCast(ref StoryRef, data []byte) ([]Item, error) {
	// Two shapes exist in the wild: {"speakers":[…],"single":{…}} (current) and
	// {"narrator":{…},"roles":{role:{…}}} (older). Both carry role→voice facts.
	var legacy struct {
		Narrator json.RawMessage            `json:"narrator"`
		Roles    map[string]json.RawMessage `json:"roles"`
	}
	if err := json.Unmarshal(data, &legacy); err == nil && (len(legacy.Roles) > 0 || len(legacy.Narrator) > nilBytes) {
		var items []Item
		keys := make([]string, 0, len(legacy.Roles))
		for k := range legacy.Roles {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(legacy.Narrator) > nilBytes {
			items = append(items, Item{
				Domain: DomainAudio, Kind: "audio_cast", Key: "narrator", TargetID: "narrator",
				Payload: AudioCastRow{RoleKey: "narrator", Payload: legacy.Narrator},
			})
		}
		for _, k := range keys {
			items = append(items, Item{
				Domain: DomainAudio, Kind: "audio_cast", Key: k, TargetID: k,
				Payload: AudioCastRow{RoleKey: k, Payload: legacy.Roles[k]},
			})
		}
		return items, nil
	}
	var payload struct {
		Single struct {
			VoiceID string `json:"voiceId"`
		} `json:"single"`
		Speakers []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Role    string `json:"role"`
			VoiceID string `json:"voiceId"`
		} `json:"speakers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("audio/vai.json hỏng: %w", err)
	}
	var items []Item
	for i, sp := range payload.Speakers {
		key := sp.ID
		if key == "" {
			key = fmt.Sprintf("speaker-%d", i)
		}
		single, err := json.Marshal(map[string]any{
			"id": sp.ID, "name": sp.Name, "role": sp.Role, "voiceId": sp.VoiceID,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, Item{
			Domain: DomainAudio, Kind: "audio_cast", Key: key, TargetID: key,
			Payload: AudioCastRow{RoleKey: key, Payload: single},
		})
	}
	if len(items) == 0 && payload.Single.VoiceID != "" {
		single, _ := json.Marshal(map[string]any{"voiceId": payload.Single.VoiceID})
		items = append(items, Item{
			Domain: DomainAudio, Kind: "audio_cast", Key: "narrator", TargetID: "narrator",
			Payload: AudioCastRow{RoleKey: "narrator", Payload: single},
		})
	}
	return items, nil
}

func decodeAudioScript(ref StoryRef, rel string, data []byte) ([]Item, error) {
	// audio/ban-doc/tap-NN/CC.json
	dir := path.Dir(rel)
	chapterName := strings.TrimSuffix(path.Base(rel), ".json")
	chapter, err := strconv.Atoi(chapterName)
	if err != nil {
		return nil, fmt.Errorf("%s: tên chương không hợp lệ", rel)
	}
	var payload struct {
		Chapter    int    `json:"chapter"`
		Volume     int    `json:"volume"`
		Status     string `json:"status"`
		SourceHash string `json:"sourceHash"`
		Revision   int    `json:"revision"`
		SourceRev  int    `json:"sourceRevision"`
		Segments   []any  `json:"segments"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("%s hỏng: %w", rel, err)
	}
	revision := payload.Revision
	if revision <= 0 {
		revision = 1
	}
	sourceRev := payload.SourceRev
	if sourceRev <= 0 {
		sourceRev = revision
	}
	_ = dir
	return []Item{{
		Domain: DomainAudio, Kind: "audio_script", Key: fmt.Sprintf("%06d", chapter),
		TargetID: strconv.Itoa(chapter),
		Payload: AudioScriptRow{
			Chapter: chapter, Revision: revision, SourceRev: sourceRev,
			SourceSHA: payload.SourceHash, Status: payload.Status,
			Payload: json.RawMessage(data), SegmentCount: len(payload.Segments),
		},
	}}, nil
}

func decodeAudioQueue(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	var payload struct {
		Jobs []struct {
			JobID        string    `json:"jobId"`
			Volume       int       `json:"volume"`
			Chapter      int       `json:"chapter"`
			State        string    `json:"state"`
			SegmentsDone int       `json:"segmentsDone"`
			Segments     int       `json:"segments"`
			Percent      float64   `json:"percent"`
			Speaker      string    `json:"speaker"`
			CreatedAt    time.Time `json:"createdAt"`
			UpdatedAt    time.Time `json:"updatedAt"`
			Error        string    `json:"error"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("audio/queue.json hỏng: %w", err)
	}
	items := make([]Item, 0, len(payload.Jobs))
	for _, job := range payload.Jobs {
		state, recovered := mapJobState(job.State)
		input, err := json.Marshal(map[string]any{
			"volume": job.Volume, "chapter": job.Chapter, "speaker": job.Speaker,
			"segments": job.Segments, "segmentsDone": job.SegmentsDone, "percent": job.Percent,
			"legacyState": job.State, "legacyJobId": job.JobID, "recovered": recovered,
		})
		if err != nil {
			return nil, err
		}
		id := DeterministicUUID(ctx.SourceID, "job", ref.ProjectSlug+"/"+ref.Slug+"/tts/"+job.JobID)
		row := JobRow{
			Kind: "tts", ID: id, State: state, IdemKey: job.JobID, Input: input,
			CreatedAt: job.CreatedAt, Recovered: recovered,
		}
		if state == "completed" || state == "failed" || state == "cancelled" {
			// The table requires finished_at for every terminal state.
			finished := job.UpdatedAt
			if finished.IsZero() {
				finished = job.CreatedAt
			}
			row.FinishedAt = &finished
		}
		if state == "failed" {
			errPayload, _ := json.Marshal(map[string]string{"message": job.Error})
			row.Result = errPayload
		}
		items = append(items, Item{
			Domain: DomainAudio, Kind: "job", Key: job.JobID, TargetID: id, Payload: row,
		})
	}
	return items, nil
}

// mapJobState normalizes a file-mode queue state onto the durable job states.
// A running job loses its worker lease at cutover, so it becomes recoverable
// queued work with the historical info kept in the input payload.
func mapJobState(state string) (string, bool) {
	switch state {
	case "done", "completed":
		return "completed", false
	case "failed", "error":
		return "failed", false
	case "cancelled", "canceled":
		return "cancelled", false
	case "running":
		return "queued", true
	case "paused":
		return "paused", false
	default:
		return "queued", false
	}
}

func decodeExportRequests(ctx *DecodeContext, ref StoryRef, data []byte) ([]Item, error) {
	var envelope struct {
		Requests []struct {
			ID        string     `json:"id"`
			Format    string     `json:"format"`
			State     string     `json:"state"`
			CreatedAt time.Time  `json:"createdAt"`
			UpdatedAt *time.Time `json:"updatedAt"`
			From      int        `json:"from"`
			To        int        `json:"to"`
			Volume    int        `json:"volume"`
			Error     string     `json:"error"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("audio/export-requests.json hỏng: %w", err)
	}
	payload := envelope.Requests
	items := make([]Item, 0, len(payload))
	for i, req := range payload {
		key := req.ID
		if key == "" {
			key = fmt.Sprintf("request-%d", i)
		}
		state, recovered := mapJobState(req.State)
		if state == "completed" || state == "failed" {
			// Without the produced object we must not claim success.
			state = "failed"
			recovered = true
		}
		input, err := json.Marshal(map[string]any{
			"format": req.Format, "from": req.From, "to": req.To, "volume": req.Volume,
			"legacyState": req.State, "recovered": recovered,
		})
		if err != nil {
			return nil, err
		}
		id := DeterministicUUID(ctx.SourceID, "job", ref.ProjectSlug+"/"+ref.Slug+"/audio_export/"+key)
		row := JobRow{
			Kind: "audio_export", ID: id, State: state, IdemKey: key, Input: input,
			CreatedAt: req.CreatedAt, Recovered: recovered,
		}
		if req.UpdatedAt != nil && (state == "failed" || state == "completed") {
			row.FinishedAt = req.UpdatedAt
		}
		items = append(items, Item{
			Domain: DomainAudio, Kind: "job", Key: key, TargetID: id, Payload: row,
		})
	}
	return items, nil
}
