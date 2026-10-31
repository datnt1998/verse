-- +goose Up
-- Existing rows are upgraded once, under a revision CAS by library.Service,
-- together with publication of the canonical bundled standard-template media.
-- SQL cannot run the Go layout builder. NULL is exclusively the pending state;
-- an authored/generated project is never replaced by the backfill.
ALTER TABLE templates ADD COLUMN project jsonb;
ALTER TABLE templates ADD CONSTRAINT templates_project_schema CHECK (
    project IS NULL OR (
        jsonb_typeof(project) = 'object'
        AND project->>'schemaVersion' = '15'
        AND project->>'id' = id::text
        AND jsonb_typeof(project->'metadata') = 'object'
        AND jsonb_typeof(project->'timeline') = 'object'
    )
);

-- +goose Down
ALTER TABLE templates DROP CONSTRAINT templates_project_schema;
ALTER TABLE templates DROP COLUMN project;
