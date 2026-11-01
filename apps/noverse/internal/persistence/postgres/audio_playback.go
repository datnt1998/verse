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

// PlaybackProof keeps synthesis/cast interpretation in the audio domain while
// the repository owns locking, queue idempotency and atomic publication.
type PlaybackProof interface {
	Fingerprint(context.Context, gen.DBTX, ScopeRef, Script) ([]byte, error)
	LegacyCurrent(context.Context, gen.DBTX, Job) (bool, error)
}

func playbackRevision(ctx context.Context, db gen.DBTX, scope ScopeRef, chapter pgtype.UUID, proof PlaybackProof) (PlaybackRevision, error) {
	var out PlaybackRevision
	var config, renders []byte
	var script Script
	var fence int64
	err := db.QueryRow(ctx, `SELECT s.revision_no, s.source_revision, c.current_revision, s.source_sha256, s.payload,
	 jsonb_build_array(s.revision_no, s.source_revision, s.source_sha256, c.current_revision),
	 (SELECT jsonb_agg(jsonb_build_array(segment_id, synth_key, blob_id, duration_sec, sample_rate) ORDER BY segment_id)
	   FROM audio_renders r WHERE r.project_id=$1 AND r.chapter_id=$3 AND r.script_revision=s.revision_no AND selected),
	 latest.id, COALESCE(latest.fencing_token,0)
	 FROM chapters c JOIN audio_scripts s ON s.project_id=c.project_id AND s.chapter_id=c.id
	 LEFT JOIN LATERAL (SELECT id, fencing_token FROM jobs WHERE project_id=$1 AND story_id=$2 AND kind='tts'
	   AND input->>'chapterId'=($3::uuid)::text ORDER BY created_at DESC, id DESC LIMIT 1) latest ON true
	 WHERE c.project_id=$1 AND c.story_id=$2 AND c.id=$3
	 ORDER BY s.revision_no DESC LIMIT 1`, scope.ProjectID, scope.StoryID, chapter).Scan(
		&out.Revision, &out.SourceRevision, &out.CurrentRevision, &script.SourceSHA256, &script.Payload,
		&config, &renders, &out.LatestJob, &fence)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrJobNotFound
	}
	if err != nil {
		return out, sanitizeErr(err)
	}
	script.ChapterID, script.RevisionNo, script.SourceRevision = chapter, out.Revision, out.SourceRevision
	audible, err := proof.Fingerprint(ctx, db, scope, script)
	if err != nil {
		return out, err
	}
	config = append(config, audible...)
	h := sha256.Sum256(config)
	out.Ownership = hex.EncodeToString(h[:])
	h = sha256.Sum256(append(append(config, renders...), []byte(fmt.Sprintf("%s:%d", UUIDString(out.LatestJob), fence))...))
	out.Content = hex.EncodeToString(h[:])
	return out, nil
}

func (a *Audio) PlaybackRevision(ctx context.Context, scope ScopeRef, chapter pgtype.UUID, proof PlaybackProof) (PlaybackRevision, error) {
	return playbackRevision(ctx, a.pool, scope, chapter, proof)
}

// PlaybackRevisionTx is used inside the fenced ACK/publication transaction.
func (a *Audio) PlaybackRevisionTx(ctx context.Context, tx pgx.Tx, scope ScopeRef, chapter pgtype.UUID, proof PlaybackProof) (PlaybackRevision, error) {
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
	return playbackRevision(ctx, tx, scope, chapter, proof)
}

// EnqueuePlaybackTx shares the render completion transaction: there is no
// completion/dispatch crash window. The existing audio_export queue owns leases.
func (a *Audio) EnqueuePlaybackTx(ctx context.Context, tx pgx.Tx, job Job, proof PlaybackProof) error {
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
	rev, err := a.PlaybackRevisionTx(ctx, tx, scope, chapter, proof)
	if err != nil {
		return err
	}
	if rev.LatestJob != job.ID {
		return nil
	}
	current := in.Ownership != "" && in.Ownership == rev.Ownership
	if in.Ownership == "" {
		current, err = proof.LegacyCurrent(ctx, tx, job)
		if err != nil {
			return err
		}
	}
	if rev.Revision != in.ScriptRevision || rev.SourceRevision != rev.CurrentRevision || !current {
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
func (a *Audio) SelectPlaybackRender(ctx context.Context, scope ScopeRef, chapterID pgtype.UUID, scriptRevision int32, segmentID, synthKey string, volume, chapter int, proof PlaybackProof) error {
	return a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := a.PlaybackRevisionTx(ctx, tx, scope, chapterID, proof); err != nil {
			return err
		}
		if _, err := a.SelectRenderTx(ctx, tx, scope.ProjectID, chapterID, scriptRevision, segmentID, synthKey); err != nil {
			return err
		}
		revision, err := playbackRevision(ctx, tx, scope, chapterID, proof)
		if err != nil {
			return err
		}
		return enqueuePlaybackTx(ctx, tx, scope, "chapter-selection-"+revision.Content, volume, chapter, chapterID, revision.Content)
	})
}

// RecoverPlayback dispatches pre-upgrade completions as well as ACKs committed
// before a process restart. Each source job has exactly one durable dispatch.
func (a *Audio) RecoverPlayback(ctx context.Context, proof PlaybackProof) error {
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
			if err := a.EnqueuePlaybackTx(ctx, tx, jobFromRow(row), proof); err != nil {
				return fmt.Errorf("recover chapter playback: %w", err)
			}
		}
		return nil
	})
}
