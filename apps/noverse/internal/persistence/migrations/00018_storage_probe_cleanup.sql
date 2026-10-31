-- +goose Up
-- App-managed probe credentials have a finite lifetime. Registry/history DELETE
-- privileges remain maintenance-only; this grant applies only to saved configs.
-- Foreign keys preserve every active, pending, blob, migration and probe reference.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname='noverse_app') THEN
        GRANT DELETE ON blob_stores TO noverse_app;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname='noverse_app') THEN
        REVOKE DELETE ON blob_stores FROM noverse_app;
    END IF;
END $$;
-- +goose StatementEnd
