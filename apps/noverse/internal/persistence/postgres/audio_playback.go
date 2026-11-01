package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PlaybackRevision identifies every mutable input to chapter playback. Ownership
// excludes selected takes so a render job can attach its own outputs atomically.
type PlaybackRevision struct {
	Ownership       string
	Content         string
	Revision        int32
	SourceRevision  int32
	CurrentRevision int32
	LatestJob       pgtype.UUID
}

type playbackQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func playbackRevision(ctx context.Context, db playbackQuerier, scope ScopeRef, chapter pgtype.UUID) (PlaybackRevision, error) {
	var out PlaybackRevision
	var config, renders []byte
	err := db.QueryRow(ctx, `SELECT s.revision_no, s.source_revision, c.current_revision,
	 jsonb_build_array(to_jsonb(s), c.current_revision,
	   (SELECT payload->'settings' FROM audio_settings WHERE project_id=$1 AND story_id=$2),
	   (SELECT jsonb_agg(to_jsonb(a) ORDER BY role_key) FROM audio_cast a WHERE project_id=$1 AND story_id=$2),
	   (SELECT jsonb_agg(to_jsonb(v) ORDER BY legacy_id) FROM audio_voices v WHERE project_id=$1)),
	 (SELECT jsonb_agg(to_jsonb(r) ORDER BY segment_id) FROM audio_renders r
	   WHERE r.project_id=$1 AND r.chapter_id=$3 AND r.script_revision=s.revision_no AND selected),
	 (SELECT id FROM jobs WHERE project_id=$1 AND story_id=$2 AND kind='tts'
	   AND input->>'chapterId'=$3::text ORDER BY created_at DESC, id DESC LIMIT 1)
	 FROM chapters c JOIN audio_scripts s ON s.project_id=c.project_id AND s.chapter_id=c.id
	 WHERE c.project_id=$1 AND c.story_id=$2 AND c.id=$3
	 ORDER BY s.revision_no DESC LIMIT 1`, scope.ProjectID, scope.StoryID, chapter).Scan(
		&out.Revision, &out.SourceRevision, &out.CurrentRevision, &config, &renders, &out.LatestJob)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrJobNotFound
	}
	if err != nil {
		return out, sanitizeErr(err)
	}
	h := sha256.Sum256(config)
	out.Ownership = hex.EncodeToString(h[:])
	h = sha256.Sum256(append(append(config, renders...), []byte(UUIDString(out.LatestJob))...))
	out.Content = hex.EncodeToString(h[:])
	return out, nil
}

func (a *Audio) PlaybackRevision(ctx context.Context, scope ScopeRef, chapter pgtype.UUID) (PlaybackRevision, error) {
	return playbackRevision(ctx, a.pool, scope, chapter)
}

// PlaybackRevisionTx is used inside the fenced ACK/publication transaction.
func (a *Audio) PlaybackRevisionTx(ctx context.Context, tx pgx.Tx, scope ScopeRef, chapter pgtype.UUID) (PlaybackRevision, error) {
	// Hold existing mutable inputs through attachment/publication. A concurrent
	// editor or cast/settings writer cannot pass the ownership check and then
	// change those rows before the transaction commits.
	for _, query := range []string{
		`SELECT id FROM chapters WHERE project_id=$1 AND story_id=$2 AND id=$3 FOR SHARE`,
		`SELECT revision_no FROM audio_scripts WHERE project_id=$1 AND story_id=$2 AND chapter_id=$3 FOR SHARE`,
	} {
		if _, err := tx.Exec(ctx, query, scope.ProjectID, scope.StoryID, chapter); err != nil {
			return PlaybackRevision{}, sanitizeErr(err)
		}
	}
	for _, query := range []string{
		`SELECT story_id FROM audio_settings WHERE project_id=$1 AND story_id=$2 FOR SHARE`,
		`SELECT role_key FROM audio_cast WHERE project_id=$1 AND story_id=$2 FOR SHARE`,
	} {
		if _, err := tx.Exec(ctx, query, scope.ProjectID, scope.StoryID); err != nil {
			return PlaybackRevision{}, sanitizeErr(err)
		}
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM audio_voices WHERE project_id=$1 FOR SHARE`, scope.ProjectID); err != nil {
		return PlaybackRevision{}, sanitizeErr(err)
	}
	return playbackRevision(ctx, tx, scope, chapter)
}

// EnqueuePlaybackTx shares the render completion transaction: there is no
// completion/dispatch crash window. The existing audio_export queue owns leases.
func (a *Audio) EnqueuePlaybackTx(ctx context.Context, tx pgx.Tx, job Job) error {
	var in struct {
		Volume         int    `json:"volume"`
		Chapter        int    `json:"chapter"`
		ChapterID      string `json:"chapterId"`
		ScriptRevision int32  `json:"scriptRevision"`
		Ownership      string `json:"ownership"`
	}
	if err := json.Unmarshal(job.Input, &in); err != nil {
		return err
	}
	// Old non-chapter protocol jobs have no playable chapter to assemble.
	if in.Volume <= 0 || in.Chapter <= 0 || in.ChapterID == "" {
		return nil
	}
	chapter, err := ParseUUID(in.ChapterID)
	if err != nil {
		return err
	}
	scope := ScopeRef{ProjectID: job.ProjectID, StoryID: job.StoryID}
	rev, err := playbackRevision(ctx, tx, scope, chapter)
	if err != nil {
		return err
	}
	if rev.LatestJob != job.ID {
		return nil
	}
	if rev.Revision != in.ScriptRevision || rev.SourceRevision != rev.CurrentRevision ||
		(in.Ownership != "" && in.Ownership != rev.Ownership) {
		rev.Content = "superseded"
	}
	return enqueuePlaybackTx(ctx, tx, scope, "chapter-playback-"+UUIDString(job.ID), in.Volume, in.Chapter, chapter, rev.Content)
}

func enqueuePlaybackTx(ctx context.Context, tx pgx.Tx, scope ScopeRef, key string, volume, chapter int, chapterID pgtype.UUID, revision string) error {
	input, err := json.Marshal(map[string]any{
		"playback": true, "volume": volume, "chapter": chapter,
		"chapterId": UUIDString(chapterID), "revision": revision,
	})
	if err != nil {
		return err
	}
	id, err := randomUUID()
	if err != nil {
		return err
	}
	_, err = gen.New(tx).InsertJob(ctx, gen.InsertJobParams{
		ProjectID: scope.ProjectID, StoryID: scope.StoryID, ID: id,
		Kind: string(JobAudioExport), IdempotencyKey: key, Input: input,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return sanitizeErr(err)
}

// SelectPlaybackRender changes the take and dispatches its chapter rebuild in
// the same transaction, so accepting a candidate cannot leave playback missing.
func (a *Audio) SelectPlaybackRender(ctx context.Context, scope ScopeRef, chapterID pgtype.UUID, scriptRevision int32, segmentID, synthKey string, volume, chapter int) error {
	return a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := a.PlaybackRevisionTx(ctx, tx, scope, chapterID); err != nil {
			return err
		}
		if _, err := a.SelectRenderTx(ctx, tx, scope.ProjectID, chapterID, scriptRevision, segmentID, synthKey); err != nil {
			return err
		}
		revision, err := playbackRevision(ctx, tx, scope, chapterID)
		if err != nil {
			return err
		}
		return enqueuePlaybackTx(ctx, tx, scope, "chapter-selection-"+revision.Content, volume, chapter, chapterID, revision.Content)
	})
}

// RecoverPlayback dispatches pre-upgrade completions as well as ACKs committed
// before a process restart. Each source job has exactly one durable dispatch.
func (a *Audio) RecoverPlayback(ctx context.Context) error {
	return a.tx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT j.project_id, j.id FROM jobs j WHERE j.kind='tts' AND j.state='completed'
		 AND j.input ? 'chapterId' AND j.input ? 'volume'
		 AND NOT EXISTS (SELECT 1 FROM jobs newer WHERE newer.project_id=j.project_id AND newer.story_id=j.story_id
		   AND newer.kind='tts' AND newer.input->>'chapterId'=j.input->>'chapterId'
		   AND (newer.created_at,newer.id)>(j.created_at,j.id))
		 AND NOT EXISTS (SELECT 1 FROM jobs e WHERE e.project_id=j.project_id AND e.story_id=j.story_id
		 AND e.kind='audio_export' AND e.idempotency_key='chapter-playback-'||j.id::text)
		 ORDER BY j.created_at LIMIT 32`)
		if err != nil {
			return sanitizeErr(err)
		}
		var keys []JobKey
		for rows.Next() {
			var key JobKey
			if err := rows.Scan(&key.ProjectID, &key.ID); err != nil {
				rows.Close()
				return err
			}
			keys = append(keys, key)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, key := range keys {
			row, err := gen.New(tx).GetJob(ctx, gen.GetJobParams{ProjectID: key.ProjectID, ID: key.ID})
			if err != nil {
				return sanitizeErr(err)
			}
			if err := a.EnqueuePlaybackTx(ctx, tx, jobFromRow(row)); err != nil {
				return fmt.Errorf("recover chapter playback: %w", err)
			}
		}
		return nil
	})
}
