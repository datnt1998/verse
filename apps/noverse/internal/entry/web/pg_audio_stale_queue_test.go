//go:build !integration

package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
)

// Hosted queue freshness: a queued, paused or running render row was created
// from one accepted chapter revision. When the chapter changes, the next
// Generate must turn that row into preparation work for the new revision
// instead of reusing it — otherwise the worker synthesizes the old text, the
// ACK is refused and the job still completes with no audio. The same request
// repeated while the fresh work is active must reuse that row.

type voiceCall struct {
	release  chan struct{}
	answered bool
}

// queueVoiceProvider answers compose calls with a per-call segment text and
// holds each call until the test releases it, so the durable row can be
// inspected while composition is genuinely in flight.
type queueVoiceProvider struct {
	server  *httptest.Server
	mu      sync.Mutex
	calls   []*voiceCall
	texts   []string
	started chan int
}

func newQueueVoiceProvider(t *testing.T, texts ...string) *queueVoiceProvider {
	t.Helper()
	p := &queueVoiceProvider{texts: texts, started: make(chan int, 16)}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		n := len(p.calls)
		call := &voiceCall{release: make(chan struct{})}
		p.calls = append(p.calls, call)
		text := p.texts[len(p.texts)-1]
		if n < len(p.texts) {
			text = p.texts[n]
		}
		p.mu.Unlock()
		p.started <- n
		<-call.release
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, prepSegmentResponse(text))
	}))
	t.Cleanup(p.server.Close)
	t.Cleanup(p.answerAll)
	return p
}

func (p *queueVoiceProvider) waitCall(t *testing.T, want int) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case n := <-p.started:
			if n == want {
				return
			}
		case <-deadline:
			t.Fatalf("provider call %d never started", want)
		}
	}
}

func (p *queueVoiceProvider) answer(n int) {
	p.mu.Lock()
	call := p.calls[n]
	if !call.answered {
		call.answered = true
		close(call.release)
	}
	p.mu.Unlock()
}

func (p *queueVoiceProvider) answerAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, call := range p.calls {
		if !call.answered {
			call.answered = true
			close(call.release)
		}
	}
}

func (p *queueVoiceProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

type queueFrozen struct {
	Kind           string `json:"kind"`
	Chapter        int    `json:"chapter"`
	Volume         int    `json:"volume"`
	ChapterID      string `json:"chapterId"`
	ScriptRevision int32  `json:"scriptRevision"`
	SourceRevision int32  `json:"sourceRevision"`
	NeedsScript    bool   `json:"needsScript"`
	Quality        string `json:"quality"`
	Segments       []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"segments"`
}

func frozenQueueInput(t *testing.T, row postgres.Job) queueFrozen {
	t.Helper()
	var in queueFrozen
	if err := json.Unmarshal(row.Input, &in); err != nil {
		t.Fatalf("job input: %v", err)
	}
	return in
}

// chapterJobs returns every tts row of one chapter of the harness story in
// enqueue order (the durable queue as the service reads it).
func chapterJobs(t *testing.T, h *prepHarness, chapter int) []postgres.Job {
	t.Helper()
	rows, err := h.jobs.List(context.Background(), h.scope)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	out := make([]postgres.Job, 0, len(rows))
	for _, row := range rows {
		if row.Kind != postgres.JobTTS {
			continue
		}
		var in queueFrozen
		if err := json.Unmarshal(row.Input, &in); err != nil || in.Chapter != chapter {
			continue
		}
		out = append(out, row)
	}
	return out
}

func queueView(t *testing.T, h *prepHarness) audio.QueueView {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "http://x/audio/queue?volume=1", nil)
	w := httptest.NewRecorder()
	admin := identity.User{Username: "admin", Role: identity.RoleAdmin}
	if !h.tg.g.serveStoryAudio(w, r, admin, h.st, r.URL.Path) {
		t.Fatalf("GET audio queue not handled")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("queue view: %d %s", w.Code, w.Body.String())
	}
	var view audio.QueueView
	if err := json.NewDecoder(w.Body).Decode(&view); err != nil {
		t.Fatalf("decode queue view: %v", err)
	}
	return view
}

func testQueueBody() map[string]any {
	return map[string]any{"chapters": []int{1}, "quality": "standard", "voiceMode": "cast", "lock": true}
}

// acceptQueueRevision appends a new accepted revision of chapter 1, the same
// write the editor makes after the author saves a change.
func acceptQueueRevision(t *testing.T, h *prepHarness, text string) {
	t.Helper()
	if _, err := h.nar.AcceptRecord(h.ctx, h.scope, 1, domain.ChapterOriginGenerated, text,
		domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept revised chapter: %v", err)
	}
}

func TestPGHostedStaleActiveJobGetsFreshPreparation(t *testing.T) {
	oldText := "Chương một. Bản cũ."
	newText := "Chương một. Bản mới."
	for _, mode := range []string{"queued", "paused", "running"} {
		t.Run(mode, func(t *testing.T) {
			provider := newQueueVoiceProvider(t, "Đoạn cũ.", "Đoạn mới.")
			h := newPrepHarness(t, "stale-"+mode, "novel-"+mode, oldText, provider.server.URL)
			h.tg.g.StartAudioPreparation(h.ctx)

			// First Generate: compose once, leaving a render row for revision 1.
			postPrepQueue(t, h, testQueueBody())
			provider.waitCall(t, 0)
			provider.answer(0)
			waitPrep(t, "first render row", func() bool {
				rows := chapterJobs(t, h, 1)
				if len(rows) != 1 || rows[0].State != postgres.JobQueued {
					return false
				}
				in := frozenQueueInput(t, rows[0])
				return !in.NeedsScript && in.SourceRevision == 1 && len(in.Segments) == 1
			})
			first := chapterJobs(t, h, 1)[0]
			firstID := postgres.UUIDString(first.ID)
			var fence int64
			var lease postgres.WorkerLease
			switch mode {
			case "paused":
				paused, err := h.jobs.Pause(h.ctx, postgres.JobKey{ProjectID: h.scope.ProjectID, ID: first.ID})
				if err != nil {
					t.Fatalf("pause old render: %v", err)
				}
				fence = paused.FencingToken
			case "running":
				hash := sha256.Sum256([]byte("stale-queue-" + mode))
				worker, err := h.jobs.RegisterWorker(h.ctx, "stale-queue-"+mode, hash[:], []string{"tts"})
				if err != nil {
					t.Fatalf("register worker: %v", err)
				}
				claimed, err := h.jobs.Claim(h.ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
				if err != nil {
					t.Fatalf("claim old render: %v", err)
				}
				if postgres.UUIDString(claimed.ID) != firstID {
					t.Fatalf("claimed job %s, want the chapter row %s", postgres.UUIDString(claimed.ID), firstID)
				}
				fence = claimed.FencingToken
				lease = postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: first.ProjectID, ID: first.ID}, WorkerID: worker.ID, Fence: claimed.FencingToken}
			default:
				fence = first.FencingToken
			}

			// A second story in the same project must never be touched by the
			// chapter fence: its own queued row is a different scope.
			if mode == "queued" {
				if _, err := h.nar.CreateStory(h.ctx, "stale-"+mode, nil, postgres.StoryInfo{Slug: "other", Title: "Truyện khác", Status: "writing"}); err != nil {
					t.Fatalf("create other story: %v", err)
				}
				otherScope, err := h.nar.Resolve(h.ctx, "stale-"+mode, "other", nil, false)
				if err != nil {
					t.Fatalf("resolve other story: %v", err)
				}
				if _, err := h.nar.AcceptRecord(h.ctx, otherScope, 1, domain.ChapterOriginGenerated, "Truyện khác.",
					domain.ChapterFacts{}, domain.StyleDelta{},
					postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
					t.Fatalf("accept other chapter: %v", err)
				}
				otherRef, err := h.nar.ChapterRefByNumber(h.ctx, otherScope, 1)
				if err != nil {
					t.Fatalf("other chapter ref: %v", err)
				}
				total := int32(1)
				if _, err := h.jobs.Enqueue(h.ctx, otherScope, postgres.JobTTS, "other-story-job", map[string]any{
					"kind": "create", "chapter": 1, "volume": 1,
					"chapterId": postgres.UUIDString(otherRef.ID), "sourceRevision": 1,
					"quality":  "standard",
					"segments": []map[string]any{{"id": "s0001", "text": "Khác."}},
				}, &total, pgtype.UUID{}); err != nil {
					t.Fatalf("enqueue other story job: %v", err)
				}
			}

			// The accepted chapter revision changes while the old row is live.
			acceptQueueRevision(t, h, newText)

			postPrepQueue(t, h, testQueueBody())
			provider.waitCall(t, 1)

			rows := chapterJobs(t, h, 1)
			if len(rows) != 1 {
				t.Fatalf("active rows = %d (%+v), want exactly one", len(rows), rows)
			}
			row := rows[0]
			if postgres.UUIDString(row.ID) != firstID {
				t.Fatalf("row id = %s, want the stale row %s rewritten in place", postgres.UUIDString(row.ID), firstID)
			}
			in := frozenQueueInput(t, row)
			if !in.NeedsScript || in.SourceRevision != 2 || in.ScriptRevision != 1 {
				t.Fatalf("preparation input = %+v, want needsScript for source revision 2", in)
			}
			if row.State != postgres.JobQueued || row.WorkerID.Valid {
				t.Fatalf("preparation row = %+v, want queued with no native worker", row)
			}
			if row.FencingToken <= fence {
				t.Fatalf("fence = %d, want it bumped past the old %d", row.FencingToken, fence)
			}
			if mode == "running" {
				if _, err := h.jobs.Complete(h.ctx, lease, map[string]any{"segments": 1}, 1, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
					t.Fatalf("stale worker completed after supersede: %v", err)
				}
			}

			// The same request while composition is active must reuse the row.
			postPrepQueue(t, h, testQueueBody())
			rows = chapterJobs(t, h, 1)
			if len(rows) != 1 || postgres.UUIDString(rows[0].ID) != firstID {
				t.Fatalf("repeated request changed the row: %+v", rows)
			}
			if provider.callCount() != 2 {
				t.Fatalf("compose calls = %d, want 2 (no parallel preparation)", provider.callCount())
			}

			provider.answer(1)
			waitPrep(t, "fresh render row", func() bool {
				rows := chapterJobs(t, h, 1)
				if len(rows) != 1 || rows[0].State != postgres.JobQueued {
					return false
				}
				in := frozenQueueInput(t, rows[0])
				return !in.NeedsScript && in.SourceRevision == 2 &&
					len(in.Segments) == 1 && in.Segments[0].Text == "Đoạn mới."
			})
			final := chapterJobs(t, h, 1)[0]
			if postgres.UUIDString(final.ID) != firstID {
				t.Fatalf("fresh render row id = %s, want the same row %s", postgres.UUIDString(final.ID), firstID)
			}
			// The frozen job must be the one the ACK would accept: otherwise a
			// full chapter of synthesized audio would be dropped on completion.
			ref, err := h.nar.ChapterRefByNumber(h.ctx, h.scope, 1)
			if err != nil {
				t.Fatalf("chapter ref: %v", err)
			}
			revision, err := h.repo.PlaybackRevision(h.ctx, h.scope, ref.ID, audio.PlaybackProof{})
			if err != nil {
				t.Fatalf("playback revision: %v", err)
			}
			current, err := (audio.PlaybackProof{}).Current(h.ctx, h.pool.Pool(), final, revision)
			if err != nil || !current {
				t.Fatalf("fresh render job would drop its ACK: current=%v err=%v", current, err)
			}
			script, err := h.svc.Script(context.Background(), 1, 1)
			if err != nil {
				t.Fatalf("script: %v", err)
			}
			if script == nil || script.SourceHash != audio.SourceHash(newText) || script.Status != audio.StatusLocked {
				t.Fatalf("script = %+v, want the locked bản đọc of the new revision", script)
			}

			// A second identical Generate after preparation is idempotent too:
			// the fresh render row is reused, not duplicated.
			postPrepQueue(t, h, testQueueBody())
			rows = chapterJobs(t, h, 1)
			if len(rows) != 1 || postgres.UUIDString(rows[0].ID) != firstID || rows[0].State != postgres.JobQueued {
				t.Fatalf("fresh request duplicated work: %+v", rows)
			}

			if mode == "queued" {
				other, err := h.nar.Resolve(h.ctx, "stale-"+mode, "other", nil, false)
				if err != nil {
					t.Fatalf("resolve other story: %v", err)
				}
				otherRows, err := h.jobs.List(h.ctx, other)
				if err != nil {
					t.Fatalf("list other story jobs: %v", err)
				}
				if len(otherRows) != 1 || otherRows[0].State != postgres.JobQueued ||
					otherRows[0].FencingToken != 0 || otherRows[0].IdempotencyKey != "other-story-job" {
					t.Fatalf("other story job was touched: %+v", otherRows)
				}
			}
		})
	}
}

// queueWaiting and waitingMachine describe chapters lost to an offline reader
// machine. A needsScript row is the web process composing the bản đọc, so it
// must not be counted or labelled as native work; the same row becomes machine
// work once preparation swaps in the render input.
func TestPGHostedPreparationIsNotWaitingMachine(t *testing.T) {
	provider := newQueueVoiceProvider(t, "Đoạn đọc.")
	h := newPrepHarness(t, "waiting-proj", "novel-waiting", "Chương chờ soạn.", provider.server.URL)

	// No preparation runner: the row stays a queued needsScript preparation.
	postPrepQueue(t, h, testQueueBody())
	if n := h.tg.g.queuedVoiceChapters(context.Background()); n != 0 {
		t.Fatalf("queueWaiting = %d, want 0 while only web preparation is queued", n)
	}
	view := queueView(t, h)
	if len(view.Jobs) != 1 {
		t.Fatalf("timeline jobs = %d (%+v), want the preparation row", len(view.Jobs), view.Jobs)
	}
	prep := view.Jobs[0]
	if !prep.NeedsScript || prep.Step != "compose" {
		t.Fatalf("preparation row = %+v, want needsScript + compose", prep)
	}
	if prep.WaitingMachine {
		t.Fatalf("preparation row labelled as waiting for the reader machine: %+v", prep)
	}
	for _, chapter := range view.Chapters {
		if chapter.Chapter == 1 && chapter.WaitingMachine {
			t.Fatalf("composing chapter labelled as waiting for the reader machine: %+v", chapter)
		}
	}

	// A real render row is work the native machine owns again.
	total := int32(1)
	if _, err := h.jobs.Enqueue(context.Background(), h.scope, postgres.JobTTS, "native-work", map[string]any{
		"kind": "create", "chapter": 2, "volume": 1, "quality": "standard",
		"segments": []map[string]any{{"id": "s0001", "text": "Đoạn đọc."}},
	}, &total, pgtype.UUID{}); err != nil {
		t.Fatalf("enqueue render row: %v", err)
	}
	if n := h.tg.g.queuedVoiceChapters(context.Background()); n != 1 {
		t.Fatalf("queueWaiting = %d, want 1 for the claimable render row", n)
	}
	view = queueView(t, h)
	var waiting bool
	for _, job := range view.Jobs {
		if job.Chapter == 2 && job.State == audio.JobQueued && job.WaitingMachine {
			waiting = true
		}
		if job.NeedsScript && job.WaitingMachine {
			t.Fatalf("preparation row labelled as waiting for the reader machine: %+v", job)
		}
	}
	if !waiting {
		t.Fatalf("render row not labelled waiting for the reader machine: %+v", view.Jobs)
	}
}
