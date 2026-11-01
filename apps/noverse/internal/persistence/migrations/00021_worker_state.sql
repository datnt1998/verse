-- +goose Up
-- Kênh trạng thái worker → server: the Mac reader publishes what it is doing
-- (missing | provisioning <step> <percent> | starting | ready | error) and the
-- server queues operator commands back (install | reinstall) on the same row.
--
-- `offline` is never stored: it is derived from last_seen_at against the job
-- lease plus two heartbeats, so a sleeping Mac needs no writer.
--
-- command_seq is monotonic per credential: the worker keeps the last seq it ran
-- and never executes one twice. state/command are opaque JSON objects (the
-- shared voice.ProvisionState contract) — no server code depends on their shape
-- beyond reading the fields the API exposes.
ALTER TABLE workers
    ADD COLUMN state            jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN state_updated_at timestamptz,
    ADD COLUMN command_seq      bigint NOT NULL DEFAULT 0 CHECK (command_seq >= 0),
    ADD COLUMN command          jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD CONSTRAINT workers_state_object CHECK (jsonb_typeof(state) = 'object'),
    ADD CONSTRAINT workers_command_object CHECK (jsonb_typeof(command) = 'object');

-- Newest-first lookup of the freshest state across live credentials.
CREATE INDEX workers_state_fresh ON workers (state_updated_at DESC NULLS LAST)
    WHERE revoked_at IS NULL;

-- +goose Down
-- Throwaway databases only: the columns are additive and a worker may already
-- have published a state this binary's predecessor does not read.
ALTER TABLE workers
    DROP CONSTRAINT IF EXISTS workers_command_object,
    DROP CONSTRAINT IF EXISTS workers_state_object,
    DROP COLUMN IF EXISTS command,
    DROP COLUMN IF EXISTS command_seq,
    DROP COLUMN IF EXISTS state_updated_at,
    DROP COLUMN IF EXISTS state;
