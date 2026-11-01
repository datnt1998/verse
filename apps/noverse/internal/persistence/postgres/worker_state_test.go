package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

// provisionWorkers builds an empty migrated database: the worker state channel
// touches nothing else.
func provisionWorkers(t *testing.T) *postgres.Jobs {
	t.Helper()
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	return postgres.NewJobs(openPool(t, dsn))
}

// The reader publishes its state on the same row as the credential, and the
// freshest state is what the public API reads.
func TestWorkerStateRoundTrip(t *testing.T) {
	jobs := provisionWorkers(t)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-state")

	first := map[string]any{"state": "provisioning", "step": "deps", "percent": 42, "message": "Đang cài thư viện…"}
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := jobs.SetWorkerState(ctx, w.ID, raw); err != nil {
		t.Fatalf("set state: %v", err)
	}

	row, err := jobs.WorkerByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("worker by id: %v", err)
	}
	if row.StateUpdatedAt == nil || time.Since(*row.StateUpdatedAt) > time.Minute {
		t.Fatalf("state_updated_at = %v, want just now", row.StateUpdatedAt)
	}
	var got map[string]any
	if err := json.Unmarshal(row.State, &got); err != nil {
		t.Fatalf("stored state %q: %v", row.State, err)
	}
	if got["state"] != "provisioning" || got["step"] != "deps" || got["percent"].(float64) != 42 {
		t.Fatalf("stored state = %v", got)
	}

	// A later post replaces the state (and only the state).
	done := []byte(`{"state":"ready","percent":100,"model":"vieneu-1"}`)
	if err := jobs.SetWorkerState(ctx, w.ID, done); err != nil {
		t.Fatalf("set ready: %v", err)
	}
	row, err = jobs.WorkerByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("worker by id: %v", err)
	}
	if err := json.Unmarshal(row.State, &got); err != nil {
		t.Fatalf("stored state %q: %v", row.State, err)
	}
	if got["state"] != "ready" || got["model"] != "vieneu-1" {
		t.Fatalf("stored state = %v", got)
	}

	// An empty body degrades to {} instead of failing the worker's tick.
	if err := jobs.SetWorkerState(ctx, w.ID, nil); err != nil {
		t.Fatalf("set empty: %v", err)
	}
	row, _ = jobs.WorkerByID(ctx, w.ID)
	if string(row.State) != "{}" {
		t.Fatalf("empty state stored as %q", row.State)
	}
}

// A revoked credential cannot publish: the row is filtered by revoked_at.
func TestWorkerStateRejectedWhenRevoked(t *testing.T) {
	jobs := provisionWorkers(t)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-revoked")

	state := []byte(`{"state":"ready","percent":100}`)
	if err := jobs.SetWorkerState(ctx, w.ID, state); err != nil {
		t.Fatalf("set state: %v", err)
	}
	if err := jobs.RevokeWorker(ctx, w.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := jobs.SetWorkerState(ctx, w.ID, []byte(`{"state":"error"}`)); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("set state after revoke = %v, want ErrJobNotFound", err)
	}
	row, err := jobs.WorkerByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("worker by id: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(row.State, &got)
	if got["state"] != "ready" {
		t.Fatalf("revoked worker state changed to %v", got)
	}
}

// One operator click = one command: the seq is monotonic, so a worker polling
// the same row never runs the same command twice.
func TestWorkerCommandBump(t *testing.T) {
	jobs := provisionWorkers(t)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-command")

	row, err := jobs.WorkerByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("worker by id: %v", err)
	}
	if row.CommandSeq != 0 {
		t.Fatalf("fresh command_seq = %d, want 0", row.CommandSeq)
	}

	first, err := jobs.BumpWorkerCommand(ctx, w.ID, "install")
	if err != nil {
		t.Fatalf("bump: %v", err)
	}
	if first.CommandSeq != 1 {
		t.Fatalf("seq = %d, want 1", first.CommandSeq)
	}
	var cmd map[string]string
	if err := json.Unmarshal(first.Command, &cmd); err != nil {
		t.Fatalf("command %q: %v", first.Command, err)
	}
	if cmd["op"] != "install" {
		t.Fatalf("op = %q, want install", cmd["op"])
	}

	second, err := jobs.BumpWorkerCommand(ctx, w.ID, "reinstall")
	if err != nil {
		t.Fatalf("bump again: %v", err)
	}
	if second.CommandSeq != 2 {
		t.Fatalf("seq = %d, want 2", second.CommandSeq)
	}

	if err := jobs.RevokeWorker(ctx, w.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := jobs.BumpWorkerCommand(ctx, w.ID, "install"); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("bump after revoke = %v, want ErrJobNotFound", err)
	}
}

// ListWorkers must expose the state channel so the hosted API can pick the
// freshest machine without a second query.
func TestListWorkersCarriesState(t *testing.T) {
	jobs := provisionWorkers(t)
	ctx := context.Background()
	w := registerTestWorker(t, jobs, "mac-list")
	if err := jobs.SetWorkerState(ctx, w.ID, []byte(`{"state":"ready","percent":100}`)); err != nil {
		t.Fatalf("set state: %v", err)
	}
	workers, err := jobs.ListWorkers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("workers = %d, want 1", len(workers))
	}
	if workers[0].StateUpdatedAt == nil {
		t.Fatal("list lost state_updated_at")
	}
	if len(workers[0].State) == 0 {
		t.Fatal("list lost the state payload")
	}
}
