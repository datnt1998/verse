package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
)

// staleScript 造一个"đã đọc xong nhưng bản đọc đã sửa"的 chapter（kind=update）。
func (f *audioFixture) staleScript(t *testing.T, chapter int, text string) {
	t.Helper()
	script := &audio.Script{
		Chapter: chapter, Volume: 1, Status: audio.StatusLocked,
		SourceHash:   audio.SourceHash(f.texts[chapter]),
		RenderedFrom: &audio.RenderSource{ScriptHash: "bản-đọc-cũ", Quality: "standard"},
		Segments: []audio.Segment{
			{ID: "s0001", Speaker: audio.NarratorID, Text: text, Source: text, Rendered: &audio.SegmentAudio{Key: "cũ", DurationSec: 1}},
		},
	}
	now := time.Now()
	script.LockedAt = &now
	if err := audio.WriteScript(f.svc.Layout(), script); err != nil {
		t.Fatal(err)
	}
}

func (f *audioFixture) queueBody(t *testing.T, body string) map[string]any {
	t.Helper()
	w := f.do(http.MethodPost, "/audio/queue?volume=1", body)
	if w.Code != http.StatusOK {
		t.Fatalf("POST queue: %d %s", w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

// waitKind 等某章的任务出现并返回（任务可能已经跑完，Kind 仍保留）。
func (f *audioFixture) waitKind(t *testing.T, chapter int) audio.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range f.svc.Queue() {
			if j.Chapter == chapter && j.Kind != "" {
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("chapter %d never queued: %+v", chapter, f.svc.Queue())
	return audio.Job{}
}

// §AK: POST …/audio/queue nhận redo[]; response gộp cả hai danh sách; queue trả kind.
func TestAudioQueueRedoContract(t *testing.T) {
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.seedScript(t, 1, audio.StatusLocked)
	f.seedScript(t, 2, audio.StatusLocked)
	out := f.queueBody(t, `{"chapters":[2],"redo":[1],"lock":false}`)
	chapters, _ := out["chapters"].([]any)
	if len(chapters) != 2 || chapters[0].(float64) != 2 || chapters[1].(float64) != 1 {
		t.Fatalf("response chapters = %v, want [2 1]", out["chapters"])
	}
	if job := f.waitKind(t, 1); job.Kind != audio.JobKindRedo {
		t.Fatalf("redo chapter kind = %+v", job)
	}
	if job := f.waitKind(t, 2); job.Kind != audio.JobKindCreate {
		t.Fatalf("new chapter kind = %+v", job)
	}

	// Chồng nhau: 409, không xếp gì thêm.
	w := f.do(http.MethodPost, "/audio/queue?volume=1", `{"chapters":[1],"redo":[1],"lock":false}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("overlap: %d %s", w.Code, w.Body.String())
	}
}

// §AK: chương "đã sửa" → kind=update; SSE audio.render mang kind.
func TestAudioRenderEventCarriesKind(t *testing.T) {
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := f.svc.Hub().Subscribe(ctx)
	f.staleScript(t, 1, "Đoạn cũ.")

	if out := f.queueBody(t, `{"chapters":[1],"lock":false}`); out["chapters"] == nil {
		t.Fatalf("empty response: %v", out)
	}
	if job := f.waitKind(t, 1); job.Kind != audio.JobKindUpdate {
		t.Fatalf("stale script kind = %+v", job)
	}
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type != audio.EventRender {
				continue
			}
			if kind, _ := ev.Data["kind"].(string); kind != audio.JobKindUpdate {
				t.Fatalf("audio.render kind = %v (%v)", ev.Data["kind"], ev.Data)
			}
			return
		case <-deadline:
			t.Fatal("no audio.render event with kind")
		}
	}
}

// §AK: dock ghi "Đang cập nhật" khi chạy và "Cập nhật giọng chương N" khi lỗi.
func TestAudioJobRowsShowUpdate(t *testing.T) {
	f := newAudioFixture(t)
	defer f.tg.g.Close()
	f.staleScript(t, 1, "Đoạn cũ.")

	entry, ok := audioServices.Load(f.st.Dir)
	if !ok {
		t.Fatal("audio entry missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := f.svc.Hub().Subscribe(ctx)
	f.queueBody(t, `{"chapters":[1],"lock":false}`)

	rendering := false
	timeout := time.After(20 * time.Second)
	for !rendering {
		select {
		case ev := <-events:
			rendering = ev.Type == audio.EventRender && ev.Data["state"] == "rendering"
		case <-timeout:
			t.Fatal("job never started rendering")
		}
	}

	rows := audioRows(entry.(audioEntry), time.Now(), true)
	var found bool
	for _, row := range rows {
		if row.ID != "proj/01-tra-xanh/audio/1" {
			continue
		}
		found = true
		if row.Status != "running" {
			t.Fatalf("row status = %+v", row)
		}
		if !strings.Contains(row.Step, "Đang cập nhật") {
			t.Fatalf("running step misses the update label: %q", row.Step)
		}
	}
	if !found {
		t.Fatalf("no dock row for the volume: %+v", rows)
	}

	// Đợi việc đầu xong rồi làm bản đọc "đã sửa" lần nữa, để lần xếp sau vẫn
	// là cập nhật; lần này bộ đọc lỗi.
	waitDone := time.Now().Add(20 * time.Second)
	for time.Now().Before(waitDone) {
		busy := false
		for _, j := range f.svc.Queue() {
			if j.Chapter == 1 && (j.State == audio.JobRendering || j.State == audio.JobQueued) {
				busy = true
			}
		}
		if !busy {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	f.staleScript(t, 1, "Đoạn mới bị lỗi.")
	f.synth.mu.Lock()
	f.synth.fail = errors.New("bộ đọc không phản hồi")
	f.synth.mu.Unlock()
	f.queueBody(t, `{"chapters":[1],"lock":false}`)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, row := range audioRows(entry.(audioEntry), time.Now(), true) {
			if row.ID == "proj/01-tra-xanh/audio/1/1" && row.Status == "failed" {
				if row.Title != "Cập nhật giọng chương 1" {
					t.Fatalf("error row title = %q", row.Title)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no update error row")
}
