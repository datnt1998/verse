package storageimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/persistence/postgres"
)

// writeItem inserts one normalized record and records its ledger row in the
// same transaction. It returns inserted=false when the target already held the
// record; the caller then compares the stored semantic digest (a mismatch is a
// conflict, never an overwrite).
func (im *importer) writeItem(ctx context.Context, pool *postgres.Pool, item Item, scope recordScope, semantic string) (bool, string, error) {
	if im.opts.DryRun {
		return true, "", nil
	}
	var inserted bool
	err := postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error {
		var err error
		inserted, err = im.insertRow(ctx, tx, item, scope)
		if err != nil {
			return err
		}
		return im.flush(ctx, tx)
	})
	if err != nil {
		return false, "", fmt.Errorf("%s/%s: %w", item.Kind, item.Key, err)
	}
	if inserted {
		return true, "", nil
	}
	existing, err := im.loadItem(ctx, pool.Pool(), item, scope)
	if err != nil {
		return false, "", fmt.Errorf("%s/%s: đọc lại bản ghi đã có: %w", item.Kind, item.Key, err)
	}
	got, err := ItemDigest(item.Kind, existing)
	if err != nil {
		return false, "", err
	}
	if got != semantic {
		return false, fmt.Sprintf("%s %q: bản ghi trong DB khác nguồn (bị sửa sau khi nhập) — từ chối ghi đè",
			item.Kind, item.Key), nil
	}
	return false, "", nil
}

// insertRow is the single writer switch. Every arm uses ON CONFLICT DO NOTHING
// (never DO UPDATE) so live edits are never clobbered by a re-import.
//
// This is the one documented exception to the "runtime domain SQL lives in
// queries/*.sql" rule (audit S7): the offline importer writes every domain in
// one pass, including append-only history rows and identity rows with source
// timestamps, and its arms are not request-path queries. It is not a second
// convention for repositories — do not copy these strings into a repository.
func (im *importer) insertRow(ctx context.Context, tx pgx.Tx, item Item, scope recordScope) (bool, error) {
	switch item.Kind {
	case "library":
		row := item.Payload.(LibraryConfig)
		return execInsert(ctx, tx, `
INSERT INTO library_settings (source_id, payload) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
			uuidArg(im.scan.SourceID), mustJSON(row))
	case "user":
		return im.insertUser(ctx, tx, item.Payload.(AccountUser))
	case "audit":
		return im.insertAudit(ctx, tx, item.Payload.(AccountAudit))
	case "project":
		row := item.Payload.(ProjectRow)
		return execInsert(ctx, tx, `
INSERT INTO projects (id, slug, name, description, notes, universe_name, settings, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
			uuidArg(row.ID), row.Slug, emptyDefault(row.Name, row.Slug), row.Description,
			row.Notes, row.UniverseName, nonNilJSON(row.Settings), tsOr(row.CreatedAt), tsOr(row.UpdatedAt))
	case "document":
		row := item.Payload.(DocumentRow)
		return execInsert(ctx, tx, `
INSERT INTO project_documents (project_id, id, kind, logical_name, text_content)
VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(item.TargetID), row.Kind, row.LogicalName, row.Text)
	case "story":
		row := item.Payload.(StoryRow)
		synopsis := im.synopsis[row.ProjectID+"/"+row.Slug]
		return execInsert(ctx, tx, `
INSERT INTO stories (project_id, id, slug, title, short_title, synopsis, title_renamed, status, tags, published_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT DO NOTHING`,
			uuidArg(row.ProjectID), uuidArg(row.ID), row.Slug, row.Title, row.ShortTitle,
			synopsis, row.TitleRenamed, row.Status, stringSlice(row.Tags), row.PublishedAt,
			tsOr(row.CreatedAt), tsOr(row.UpdatedAt))
	case "chapter":
		return im.insertChapter(ctx, tx, scope, item.Payload.(ChapterRow))
	case "chapter_prose":
		row := item.Payload.(ChapterProseRow)
		return im.applyChapterProse(ctx, tx, scope, row)
	case "chapter_draft":
		row := item.Payload.(ChapterDraftRow)
		return im.applyChapterDraft(ctx, tx, scope, row)
	case "outline":
		row := item.Payload.(OutlineRow)
		tag, err := tx.Exec(ctx, `
INSERT INTO outlines (project_id, story_id, kind, payload, text_content)
VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.Kind, nonNilJSON(row.Payload), nullableText(row.Text))
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	case "character":
		row := item.Payload.(CharacterRow)
		id := DeterministicUUID(im.scan.SourceID, "character", scope.ProjectID+"/"+scope.StoryID+"/"+row.LegacyID)
		return execInsert(ctx, tx, `
INSERT INTO characters (project_id, story_id, id, legacy_id, name, payload)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(id), row.LegacyID, row.Name, nonNilJSON(row.Payload))
	case "state":
		row := item.Payload.(StateRow)
		return execInsert(ctx, tx, `
INSERT INTO story_state (project_id, story_id, kind, payload, schema_version)
VALUES ($1,$2,$3,$4,1) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.Kind, nonNilJSON(row.Payload))
	case "artifact":
		row := item.Payload.(ArtifactRow)
		return execInsert(ctx, tx, `
INSERT INTO story_artifacts (project_id, story_id, kind, artifact_key, payload, text_content)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), artifactKind(row), row.Key,
			nullableJSON(row.Payload), nullableText(row.Text))
	case "comment":
		return im.insertComment(ctx, tx, scope, item.Payload.(CommentRow))
	case "review":
		row := item.Payload.(ReviewRow)
		id := item.TargetID
		scopeKey := fmt.Sprintf("chapter-%d", row.Chapter)
		if row.Chapter == 0 {
			scopeKey = "global"
		}
		return execInsert(ctx, tx, `
INSERT INTO reviews (project_id, story_id, id, scope_key, round, payload, reviewer_kind)
VALUES ($1,$2,$3,$4,$5,$6,'import') ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(id), scopeKey, row.Round, nonNilJSON(row.Payload))
	case "checkpoint":
		row := item.Payload.(CheckpointRow)
		scopeKey := canonicalText(row.Scope)
		return execInsert(ctx, tx, `
INSERT INTO checkpoints (project_id, story_id, seq, scope_key, step, artifact_ref, sha256, occurred_at, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.Seq, scopeKey, row.Step,
			row.Artifact, row.Digest, tsOr(row.OccurredAt), nonNilJSON(row.Raw))
	case "event":
		row := item.Payload.(EventRow)
		return execInsert(ctx, tx, `
INSERT INTO story_events (project_id, story_id, stream, log_key, seq, payload)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.Stream, row.LogKey,
			row.Seq, nonNilJSON(row.Payload))
	case "session":
		row := item.Payload.(SessionRow)
		return execInsert(ctx, tx, `
INSERT INTO agent_sessions (project_id, story_id, agent, task_key, seq, payload)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.Agent, row.TaskKey, row.Seq, nonNilJSON(row.Payload))
	case "audio_settings":
		row := item.Payload.(AudioSettingsRow)
		return execInsert(ctx, tx, `
INSERT INTO audio_settings (project_id, story_id, payload) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), nonNilJSON(row.Payload))
	case "audio_cast":
		row := item.Payload.(AudioCastRow)
		return execInsert(ctx, tx, `
INSERT INTO audio_cast (project_id, story_id, role_key, payload) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), row.RoleKey, nonNilJSON(row.Payload))
	case "audio_script":
		return im.insertAudioScript(ctx, tx, scope, item.Payload.(AudioScriptRow))
	case "job":
		row := item.Payload.(JobRow)
		return execInsert(ctx, tx, `
INSERT INTO jobs (project_id, story_id, id, kind, state, idempotency_key, input, result, progress_done, progress_total, created_at, finished_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,0,NULL,$9,$10) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(row.ID), row.Kind, row.State,
			row.IdemKey, nonNilJSON(row.Input), nullableJSON(row.Result), tsOr(row.CreatedAt), row.FinishedAt)
	}
	return false, fmt.Errorf("loại bản ghi %q chưa hỗ trợ", item.Kind)
}

func (im *importer) insertUser(ctx context.Context, tx pgx.Tx, u AccountUser) (bool, error) {
	id := im.ids.Users[u.Username]
	status := "active"
	if u.Disabled || u.DeletedAt != nil {
		status = "locked"
	}
	var disabledBy pgtype.UUID
	if u.DisabledBy != "" {
		disabledBy = uuidArg(im.ids.Users[u.DisabledBy])
	}
	inserted, err := execInsert(ctx, tx, `
INSERT INTO users (id, username, email, display_name, global_role, status, must_change_password,
    onboarded_at, disabled_at, disabled_by, disabled_reason, deleted_at, created_at, updated_at)
VALUES ($1,$2,NULL,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT DO NOTHING`,
		uuidArg(id), strings.ToLower(u.Username), u.Name, u.Role, status, u.MustChangePassword,
		u.OnboardedAt, u.DisabledAt, disabledBy, u.DisabledReason, u.DeletedAt,
		tsOr(u.CreatedAt), tsOr(u.UpdatedAt))
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO user_credentials (user_id, password_hash, password_changed_at) VALUES ($1,$2,$3)
ON CONFLICT (user_id) DO NOTHING`,
		uuidArg(id), nullableText(u.PasswordHash), u.PasswordChangedAt); err != nil {
		return false, err
	}
	if u.Recovery != nil {
		recoveryID := DeterministicUUID(im.scan.SourceID, "recovery", id)
		if _, err := tx.Exec(ctx, `
INSERT INTO recovery_codes (id, user_id, code_hash, mask, used_at, created_at) VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT DO NOTHING`,
			uuidArg(recoveryID), uuidArg(id), u.Recovery.CodeHash, nullableText(u.Recovery.Mask),
			u.Recovery.UsedAt, tsOr(u.Recovery.CreatedAt)); err != nil {
			return false, err
		}
	}
	if u.TempPassword != nil {
		temp := *u.TempPassword
		expires := temp.ExpiresAt
		grace := im.opts.Now.Add(im.opts.TemporaryPasswordTTL)
		if grace.After(expires) {
			expires = grace
		}
		created := temp.CreatedAt
		if created.IsZero() {
			created = im.opts.Now
		}
		tempID := DeterministicUUID(im.scan.SourceID, "temp_password", id)
		if _, err := tx.Exec(ctx, `
INSERT INTO temporary_passwords (id, user_id, password_hash, expires_at, used_at, created_at)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			uuidArg(tempID), uuidArg(id), temp.CodeHash, expires, temp.UsedAt, created); err != nil {
			return false, err
		}
	}
	return inserted, nil
}

func (im *importer) insertAudit(ctx context.Context, tx pgx.Tx, a AccountAudit) (bool, error) {
	actorUser := pgtype.UUID{}
	actorKind := "import"
	var actorName string
	if len(a.Actor) > 0 && string(a.Actor) != "null" {
		var actor struct {
			Username string `json:"username"`
			Kind     string `json:"kind"`
		}
		if err := json.Unmarshal(a.Actor, &actor); err == nil {
			actorName = actor.Username
			if id, ok := im.ids.Users[actor.Username]; ok {
				actorUser = uuidArg(id)
				actorKind = "user"
			}
		}
	}
	details := map[string]any{"sourceType": a.Type, "raw": json.RawMessage(a.Actor), "entry": a}
	if actorName != "" {
		details["sourceActor"] = actorName
	}
	projectID := pgtype.UUID{}
	if a.Project != "" {
		projectID = uuidArg(im.ids.Projects[a.Project])
	}
	return execInsert(ctx, tx, `
INSERT INTO audit_logs (id, project_id, actor_user_id, actor_kind, action, target_kind, target_id, details, occurred_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`,
		uuidArg(DeterministicUUID(im.scan.SourceID, "audit", a.ID)), projectID, actorUser, actorKind,
		firstNonEmpty(a.Action, a.Type), firstNonEmpty(a.Type, "account"), a.Target,
		mustJSON(details), tsOr(a.At))
}

func (im *importer) insertChapter(ctx context.Context, tx pgx.Tx, scope recordScope, row ChapterRow) (bool, error) {
	chapterID := DeterministicUUID(im.scan.SourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(row.Number))
	inserted, err := execInsert(ctx, tx, `
INSERT INTO chapters (project_id, story_id, id, number, title, draft_text, current_revision)
VALUES ($1,$2,$3,$4,$5,'',$6) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(chapterID), row.Number,
		row.Title, nullableInt(row.Current))
	if err != nil {
		return false, err
	}
	for _, v := range row.Versions {
		author := "import"
		if _, err := tx.Exec(ctx, `
INSERT INTO chapter_versions (project_id, chapter_id, revision_no, content, sha256, origin, author_kind,
    legacy_build, accepted_at, facts, style_delta)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(chapterID), v.Revision, v.Content, v.SHA256,
			v.Origin, author, nullableText(v.Build), tsOr(nonZeroTime(v.AcceptedAt, im.opts.Now)),
			nonEmptyJSON(v.Facts), nonEmptyJSON(v.StyleDelta)); err != nil {
			return false, err
		}
	}
	return inserted, nil
}

// applyChapterProse guarantees a chapter row for workspace prose. The prose
// itself lives in the chapter_text artifact (the document the Store reads); a
// story without an accepted record gets the prose as its baseline revision
// under the same content/hash invariant as accepted records, and nothing is
// invented when a record already exists.
func (im *importer) applyChapterProse(ctx context.Context, tx pgx.Tx, scope recordScope, row ChapterProseRow) (bool, error) {
	chapterID := DeterministicUUID(im.scan.SourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(row.Number))
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM chapters WHERE project_id=$1 AND id=$2`,
		uuidArg(scope.ProjectID), uuidArg(chapterID)).Scan(&exists); err == nil {
		return false, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	title := firstHeading(row.Text)
	if _, err := tx.Exec(ctx, `
INSERT INTO chapters (project_id, story_id, id, number, title, draft_text, current_revision)
VALUES ($1,$2,$3,$4,$5,'',1) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(chapterID), row.Number, title); err != nil {
		return false, err
	}
	content := domain.NormalizeChapterContent(row.Text)
	tag, err := tx.Exec(ctx, `
INSERT INTO chapter_versions (project_id, chapter_id, revision_no, content, sha256, origin, author_kind, accepted_at)
VALUES ($1,$2,1,$3,$4,'import','import',now()) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(chapterID), content, domain.ChapterContentSHA256(row.Text))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// applyChapterDraft stores the workspace draft document under chapters.draft_text,
// creating the chapter identity when only a draft exists (the same rule the
// document backend applies to a drafts/NN.draft.md write). It never overwrites
// a differing stored draft: the row is left alone and writeItem's comparison
// reports the conflict, like every other item.
func (im *importer) applyChapterDraft(ctx context.Context, tx pgx.Tx, scope recordScope, row ChapterDraftRow) (bool, error) {
	chapterID := DeterministicUUID(im.scan.SourceID, "chapter", scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(row.Number))
	if _, err := tx.Exec(ctx, `
INSERT INTO chapters (project_id, story_id, id, number, title, draft_text, current_revision)
VALUES ($1,$2,$3,$4,$5,$6,NULL) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(chapterID), row.Number,
		firstHeading(row.Text), row.Text); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `UPDATE chapters SET draft_text=$3, updated_at=now(), version=version+1
WHERE project_id=$1 AND id=$2 AND draft_text = ''`,
		uuidArg(scope.ProjectID), uuidArg(chapterID), row.Text)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (im *importer) insertComment(ctx context.Context, tx pgx.Tx, scope recordScope, row CommentRow) (bool, error) {
	var c struct {
		ID         string          `json:"id"`
		Paragraph  *int            `json:"paragraph"`
		Quote      string          `json:"quote"`
		Chips      []string        `json:"chips"`
		Text       string          `json:"text"`
		Author     string          `json:"author"`
		CreatedAt  time.Time       `json:"createdAt"`
		Status     string          `json:"status"`
		SentAt     *time.Time      `json:"sentAt"`
		FixedAt    *time.Time      `json:"fixedAt"`
		ResolvedAt *time.Time      `json:"resolvedAt"`
		FixedBy    string          `json:"fixedBy"`
		Unfixed    bool            `json:"unfixed"`
		Moved      bool            `json:"moved"`
		Relinked   bool            `json:"relinked"`
		Whole      bool            `json:"whole"`
		NewQuote   string          `json:"newQuote"`
		Candidates json.RawMessage `json:"candidates"`
		Anchor     json.RawMessage `json:"anchor"`
	}
	if err := json.Unmarshal(row.Raw, &c); err != nil {
		return false, err
	}
	id := c.ID
	if id == "" {
		id = RawDigest(row.Raw)
	}
	switch c.Status {
	case "pending", "sent", "fixed", "lost", "unsure", "done":
	case "":
		c.Status = "pending"
	default:
		return false, fmt.Errorf("comment %s có status lạ %q", id, c.Status)
	}
	chapterID := DeterministicUUID(im.scan.SourceID, "chapter",
		scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(row.Chapter))
	return execInsert(ctx, tx, `
INSERT INTO comments (project_id, story_id, id, chapter_id, legacy_author_name, body, original_paragraph,
    original_quote, chips, candidates, status, sent_at, fixed_at, resolved_at, unfixed, moved, relinked, whole,
    new_quote, anchor, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), id, uuidArg(chapterID),
		firstNonEmpty(c.Author, "import"), c.Text, c.Paragraph, c.Quote, stringSlice(c.Chips),
		jsonOrDefault(c.Candidates, "[]"), c.Status, c.SentAt, c.FixedAt, c.ResolvedAt,
		c.Unfixed, c.Moved, c.Relinked, c.Whole, c.NewQuote, nullableJSON(c.Anchor), tsOr(c.CreatedAt))
}

func (im *importer) insertAudioScript(ctx context.Context, tx pgx.Tx, scope recordScope, row AudioScriptRow) (bool, error) {
	chapterID := DeterministicUUID(im.scan.SourceID, "chapter",
		scope.ProjectID+"/"+scope.StoryID+"/"+strconv.Itoa(row.Chapter))
	var current int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(current_revision,0) FROM chapters WHERE project_id=$1 AND id=$2`,
		uuidArg(scope.ProjectID), uuidArg(chapterID)).Scan(&current); err != nil {
		return false, fmt.Errorf("script chương %d: %w", row.Chapter, err)
	}
	// The file layout keeps only the latest accepted record per chapter, so a
	// script's recorded source revision may no longer exist. Bind it to the
	// newest revision that is still present and not newer than the intent; the
	// script payload still carries the true source hash.
	sourceRev := row.SourceRev
	var existing int
	if err := tx.QueryRow(ctx, `
SELECT COALESCE(MAX(revision_no),0) FROM chapter_versions
WHERE project_id=$1 AND chapter_id=$2 AND revision_no <= $3`,
		uuidArg(scope.ProjectID), uuidArg(chapterID), sourceRev).Scan(&existing); err != nil {
		return false, err
	}
	if existing <= 0 {
		existing = current
	}
	if existing <= 0 {
		return false, fmt.Errorf("script chương %d không có revision nguồn hợp lệ", row.Chapter)
	}
	sourceRev = existing
	sourceSHA := row.SourceSHA
	if sourceSHA == "" {
		_ = tx.QueryRow(ctx, `SELECT sha256 FROM chapter_versions WHERE project_id=$1 AND chapter_id=$2 AND revision_no=$3`,
			uuidArg(scope.ProjectID), uuidArg(chapterID), sourceRev).Scan(&sourceSHA)
	}
	status := firstNonEmpty(row.Status, "draft")
	tag, err := tx.Exec(ctx, `
INSERT INTO audio_scripts (project_id, story_id, chapter_id, revision_no, source_revision, source_sha256, status, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
		uuidArg(scope.ProjectID), uuidArg(scope.StoryID), uuidArg(chapterID), row.Revision,
		sourceRev, sourceSHA, status, nonNilJSON(row.Payload))
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	var payload struct {
		Segments []struct {
			ID       string          `json:"id"`
			Speaker  string          `json:"speaker"`
			Text     string          `json:"text"`
			Source   string          `json:"source"`
			PauseMs  int             `json:"pauseMs"`
			Tone     string          `json:"tone"`
			Rendered json.RawMessage `json:"rendered"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return true, nil
	}
	for i, seg := range payload.Segments {
		if seg.ID == "" {
			seg.ID = fmt.Sprintf("s%04d", i+1)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO audio_segments (project_id, chapter_id, script_revision, segment_id, ordinal, speaker, text_content, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
			uuidArg(scope.ProjectID), uuidArg(chapterID), row.Revision, seg.ID, i,
			firstNonEmpty(seg.Speaker, "narrator"), firstNonEmpty(seg.Text, seg.Source),
			segmentPayload(seg.ID, seg.Speaker, seg.Text, seg.Source, seg.PauseMs, seg.Tone, seg.Rendered)); err != nil {
			return true, err
		}
	}
	return true, nil
}

func segmentPayload(id, speaker, text, source string, pause int, tone string, rendered json.RawMessage) []byte {
	payload := map[string]any{"id": id, "speaker": speaker, "text": text, "source": source,
		"pauseMs": pause, "tone": tone}
	if len(rendered) > 0 {
		payload["rendered"] = json.RawMessage(rendered)
	}
	return mustJSON(payload)
}

func execInsert(ctx context.Context, tx pgx.Tx, sql string, args ...any) (bool, error) {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// artifactKind narrows the canonical document kind onto the
// CHECK-constrained story_artifacts.kind column; artifacts without a domain
// kind are kept as import_artifact under their source key.
func artifactKind(row ArtifactRow) string {
	switch row.Kind {
	case "chapter_plan", "summary", "snapshot", "import_artifact", "style_sample",
		"custom_style", "custom_rules", "diagnostic", "chapter_text", "markdown_view":
		return row.Kind
	}
	return "import_artifact"
}

func canonicalText(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	data, err := CanonicalJSON(v)
	if err != nil {
		return string(raw)
	}
	return string(data)
}

func nonNilJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func jsonOrDefault(raw json.RawMessage, def string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return def
	}
	return string(raw)
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableInt(n int) any {
	if n <= 0 {
		return nil
	}
	return n
}

func stringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func tsOr(t time.Time) time.Time {
	if t.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return t.UTC().Truncate(time.Microsecond)
}

func nonZeroTime(t time.Time, fallback time.Time) time.Time {
	if t.IsZero() {
		return fallback
	}
	return t
}

func emptyDefault(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

var _ = path.Base
