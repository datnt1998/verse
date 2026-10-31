-- +goose Up
CREATE TABLE clone_jobs (
    id uuid PRIMARY KEY,
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE RESTRICT,
    source_key text NOT NULL,
    source_snapshot jsonb NOT NULL,
    template_id uuid REFERENCES templates(id) ON DELETE RESTRICT,
    template_snapshot jsonb NOT NULL DEFAULT '{}',
    model_refs jsonb NOT NULL DEFAULT '{}',
    no_images boolean NOT NULL DEFAULT false,
    generation integer NOT NULL DEFAULT 1 CHECK (generation > 0),
    superseded_at timestamptz,
    paused_at timestamptz,
    cancel_requested_at timestamptz,
    priority text NOT NULL DEFAULT 'pick' CHECK (priority IN ('pick','batch')),
    origin text NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual','bulk','auto')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','waiting','succeeded','failed','cancelled')),
    stage_group text NOT NULL DEFAULT 'queued' CHECK (stage_group IN ('queued','working','review','attention','paused','ready','posted')),
    active_step text,
    lease_owner text,
    lease_expires_at timestamptz,
    fence bigint NOT NULL DEFAULT 0,
    version bigint NOT NULL DEFAULT 1,
    posted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX clone_jobs_active_source_idx ON clone_jobs(source_key) WHERE superseded_at IS NULL;
CREATE INDEX clone_jobs_channel_stage_updated_idx ON clone_jobs(channel_id, stage_group, updated_at DESC);
CREATE INDEX clone_jobs_stage_updated_idx ON clone_jobs(stage_group, updated_at DESC) WHERE superseded_at IS NULL;
CREATE TABLE clone_steps (
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    step text NOT NULL CHECK (step IN ('download','backdrop','transcript','metadata','scenes','images','build','score','export')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','waiting','succeeded','failed','cancelled','stale')),
    attempt integer NOT NULL DEFAULT 0,
    progress_done bigint NOT NULL DEFAULT 0 CHECK (progress_done >= 0),
    progress_total bigint CHECK (progress_total >= 0),
    progress_phase text NOT NULL DEFAULT '',
    checkpoint jsonb,
    input_fingerprint text NOT NULL DEFAULT '',
    output_manifest jsonb,
    error jsonb,
    fence bigint NOT NULL DEFAULT 0,
    process_group integer,
    started_at timestamptz,
    finished_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(job_id, step)
);
CREATE TABLE clone_artifacts (
    id text NOT NULL,
    job_id uuid NOT NULL REFERENCES clone_jobs(id) ON DELETE CASCADE,
    step text NOT NULL,
    path text NOT NULL,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    bytes bigint NOT NULL CHECK (bytes >= 0),
    content_type text NOT NULL DEFAULT 'application/octet-stream',
    filename text NOT NULL DEFAULT '',
    PRIMARY KEY(job_id,id),
    UNIQUE(job_id,path)
);
CREATE TABLE clone_queue (
    id uuid PRIMARY KEY,
    channel_id uuid NOT NULL REFERENCES owner_channels(id) ON DELETE CASCADE,
    source_video_id text NOT NULL,
    source_key text NOT NULL,
    source_snapshot jsonb NOT NULL DEFAULT '{}',
    priority text NOT NULL DEFAULT 'batch' CHECK (priority IN ('pick','batch')),
    origin text NOT NULL DEFAULT 'bulk' CHECK (origin IN ('manual','bulk','auto')),
    position double precision NOT NULL,
    state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','started','skipped')),
    no_images boolean NOT NULL DEFAULT false,
    job_id uuid REFERENCES clone_jobs(id) ON DELETE SET NULL,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz
);
CREATE UNIQUE INDEX clone_queue_active_key_idx ON clone_queue(source_key) WHERE state='queued';
CREATE INDEX clone_queue_lane_idx ON clone_queue(state,priority,channel_id,position);
-- Paid image attempts continue to use image_receipts and its pre-spend journal.
-- +goose Down
DROP TABLE clone_queue;
DROP TABLE clone_artifacts;
DROP TABLE clone_steps;
DROP TABLE clone_jobs;
