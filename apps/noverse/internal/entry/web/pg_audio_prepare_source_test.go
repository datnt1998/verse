//go:build !integration

package web

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// A chapter revision accepted while the hosted preparation model call is
// blocked must not publish the prose the provider saw: the same job stays a
// retryable preparation (needsScript kept, no script, no claimable render row),
// it is not retried opaquely, and the explicit Retry re-resolves the accepted
// source and completes on the same row. The provider is the only local
// substitution; the gateway runner, real ChatComposer, repositories and
// AcceptRecord revision writer all execute.
func TestPGHostedPreparationRejectsSourceChangedDuringModelCall(t *testing.T) {
	const (
		original = "Trời đã tối. Nàng nhìn ra cửa sổ."
		revised  = "Sáng hôm sau, nàng mở cửa bước ra khu vườn."
		voiced   = "Đoạn của nội dung mới."
	)
	ctx := context.Background()
	provider := newBlockingComposer(t, voiced)
	h := newPrepHarness(t, "prep-source", "novel-source", original, provider.server.URL)
	h.tg.g.StartAudioPreparation(h.ctx)

	postPrepQueue(t, h, map[string]any{"chapters": []int{1}, "quality": "standard", "lock": true})
	provider.waitStarted(t)

	rows := prepJobs(t, h)
	if len(rows) != 1 || rows[0].State != postgres.JobQueued || !prepJobInput(t, rows[0]).NeedsScript {
		t.Fatalf("rows = %+v, want one queued preparation row", rows)
	}
	jobID := postgres.UUIDString(rows[0].ID)

	// The author accepts a new revision while the provider still composes the
	// old prose. AcceptRecord is the real revision writer: it locks the chapter
	// row and moves current_revision.
	if _, err := h.nar.AcceptRecord(ctx, h.scope, 1, domain.ChapterOriginGenerated,
		revised, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept revised chapter: %v", err)
	}
	provider.answer()

	waitPrep(t, "retryable preparation failure", func() bool {
		rows := prepJobs(t, h)
		return len(rows) == 1 && rows[0].State == postgres.JobFailed
	})
	rows = prepJobs(t, h)
	if len(rows) != 1 || postgres.UUIDString(rows[0].ID) != jobID {
		t.Fatalf("rows after the stale publish = %+v, want the same job %s", rows, jobID)
	}
	if !prepJobInput(t, rows[0]).NeedsScript {
		t.Fatalf("failed preparation lost needsScript: %s", rows[0].Input)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider calls after the stale publish = %d, want 1 (no opaque retry)", got)
	}

	// Nothing was published: no locked script and no native claim.
	if script, err := h.svc.Script(ctx, 1, 1); err != nil || script != nil {
		t.Fatalf("stale preparation published a script: %+v %v", script, err)
	}
	workerHash := sha256.Sum256([]byte("prep-source-worker-token"))
	worker, err := h.jobs.RegisterWorker(ctx, "prep-source-worker", workerHash[:], []string{"tts"})
	if err != nil {
		t.Fatalf("register worker: %v", err)
	}
	if _, err := h.jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{}); !errors.Is(err, postgres.ErrNoClaimableJob) {
		t.Fatalf("stale preparation was claimable by the native worker: %v", err)
	}

	// The explicit Retry re-enters preparation, re-resolves the volume source
	// (the revised accepted record) and publishes on the same row.
	if err := h.svc.Retry(ctx, jobID, ""); err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitPrep(t, "retried locked script", func() bool {
		script, err := h.svc.Script(ctx, 1, 1)
		return err == nil && script != nil && script.Status == audio.StatusLocked &&
			script.SourceHash == audio.SourceHash(revised)
	})
	script, err := h.svc.Script(ctx, 1, 1)
	if err != nil || script == nil || len(script.Segments) != 1 {
		t.Fatalf("retried script = %+v %v", script, err)
	}
	if script.Segments[0].Text != voiced {
		t.Fatalf("retried script text = %q, want the provider's fresh segment", script.Segments[0].Text)
	}
	waitPrep(t, "retried render row", func() bool {
		rows := prepJobs(t, h)
		return len(rows) == 1 && rows[0].State == postgres.JobQueued && !prepJobInput(t, rows[0]).NeedsScript
	})
	rows = prepJobs(t, h)
	if len(rows) != 1 || postgres.UUIDString(rows[0].ID) != jobID {
		t.Fatalf("retry moved the work to another row: %+v", rows)
	}
	if in := prepJobInput(t, rows[0]); len(in.Segments) != 1 || in.Segments[0].Text != voiced {
		t.Fatalf("retried render input = %+v, want the fresh segment", in)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("provider calls = %d, want the stale call plus exactly one retry", got)
	}
}
