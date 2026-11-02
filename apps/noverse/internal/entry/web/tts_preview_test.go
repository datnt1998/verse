package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// wavStub is a minimal RIFF/WAVE header used only to exercise the server's
// format check. It is not a playable WAV; transport behaviour is covered by
// throwaway/native smoke, not by this fixture.
var wavStub = []byte("RIFF\x24\x00\x00\x00WAVEfmt ")

// readyPreviewWorkerState publishes a ready worker with the preset the tests ask
// for, so the catalogue check accepts the id exactly as the picker would send it.
const readyPreviewWorkerState = `{"state":"ready","percent":100,"capabilities":{"preview":true},"voices":[{"id":"preset-f","name":"Nữ trẻ","gender":"female","source":"builtin"}]}`

// Chỉ preset có thật trên máy đọc mới đi tiếp; một id lạ không được đánh thức
// máy Mac, và thành viên đã đăng nhập vẫn dùng được đường nghe thử.
func TestHostedPreviewRejectsUnknownVoice(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	writer := tg.login("writer", "writer-pass")
	w := hostedWorker(t, jobs, "mac-preview-unknown")
	setWorkerState(t, jobs, w.ID, readyPreviewWorkerState)

	if rr := tg.do("GET", "/api/tts/voices/preset-not-there/sample", writer, ""); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown preset sample = %d (%s), want 404", rr.Code, rr.Body.String())
	}
	if rr := tg.do("POST", "/api/tts/preview", writer, `{"voiceId":"preset-not-there","text":"Xin chào"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown preset preview = %d (%s), want 400", rr.Code, rr.Body.String())
	}
}

// Một máy đọc đang chuẩn bị bộ đọc (chưa ready) không nhận được yêu cầu nào, nên
// nút nghe thử phải tắt và endpoint nói thẳng — không treo request.
func TestHostedPreviewNeedsReadyWorker(t *testing.T) {
	tg, jobs, _ := provisionHostedTTS(t)
	admin := tg.login("admin", "admin-pass")
	w := hostedWorker(t, jobs, "mac-preview-preparing")
	setWorkerState(t, jobs, w.ID, `{"state":"provisioning","step":"model","percent":10,"capabilities":{"preview":true}}`)

	rr := tg.do("POST", "/api/tts/preview", admin, `{"text":"Xin chào"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("preview while preparing = %d (%s), want 503", rr.Code, rr.Body.String())
	}
}

func TestPreviewQueueRoundTrip(t *testing.T) {
	q := newPreviewQueue()
	ticket, err := q.enqueue(voice.SynthRequest{Text: "Xin chào", VoiceID: "preset-f"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if taken := q.take("worker-a"); taken == nil || taken.id != ticket.id {
		t.Fatalf("take = %+v, want ticket %s", taken, ticket.id)
	}
	if second := q.take("worker-b"); second != nil {
		t.Fatalf("take handed the same preview twice: %+v", second)
	}
	if !q.complete("worker-a", ticket.id, wavStub, 22050) {
		t.Fatal("complete refused a live preview")
	}
	wav, rate, err := q.wait(context.Background(), ticket.id)
	if err != nil || !bytes.Equal(wav, wavStub) || rate != 22050 {
		t.Fatalf("wait = %q/%d/%v", wav, rate, err)
	}
	if q.complete("worker-a", ticket.id, wavStub, 22050) {
		t.Fatal("complete accepted a preview that was already consumed")
	}
}

func TestPreviewQueueBoundsAndCancellation(t *testing.T) {
	q := newPreviewQueue()
	for range previewQueueMax {
		if _, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}
	if _, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"}); !errors.Is(err, errPreviewQueueFull) {
		t.Fatalf("enqueue past the cap = %v, want errPreviewQueueFull", err)
	}
	claimed := q.take("worker-a")
	if claimed == nil {
		t.Fatal("take returned nothing while the queue was full")
	}
	// Người dùng đóng trang: request bị huỷ và kết quả về muộn không được phục vụ.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := q.wait(ctx, claimed.id); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait after cancel = %v, want context.Canceled", err)
	}
	if q.complete("worker-a", claimed.id, wavStub, 24000) {
		t.Fatal("complete accepted a preview nobody was waiting for")
	}

	failed := q.take("worker-b")
	if failed == nil {
		t.Fatal("take returned nothing for the second preview")
	}
	q.fail("worker-b", failed.id)
	if _, _, err := q.wait(context.Background(), failed.id); !errors.Is(err, errPreviewFailed) {
		t.Fatalf("wait after fail = %v, want errPreviewFailed", err)
	}
}

// Kết quả đầu tiên là kết quả cuối cùng: máy đọc trùng lặp, máy khác, hoặc một
// lần báo lỗi sau khi đã có WAV đều không được ghi đè lên thứ người dùng nhận.
func TestPreviewQueueTerminalResultIsFinal(t *testing.T) {
	q := newPreviewQueue()
	ticket, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Chưa ai nhận thì không ai được ghi kết quả.
	if q.complete("worker-a", ticket.id, wavStub, 24000) || q.fail("worker-a", ticket.id) {
		t.Fatal("a terminal result was accepted for an unclaimed preview")
	}
	claimed := q.take("worker-a")
	if claimed == nil || claimed.id != ticket.id {
		t.Fatalf("take = %+v, want ticket %s", claimed, ticket.id)
	}
	if q.complete("worker-b", ticket.id, wavStub, 24000) {
		t.Fatal("another worker wrote the result of a claimed preview")
	}
	if !q.complete("worker-a", ticket.id, wavStub, 24000) {
		t.Fatal("the claiming worker could not write its result")
	}
	if q.complete("worker-a", ticket.id, []byte("RIFFxxxxWAVE-late"), 8000) {
		t.Fatal("a duplicate completion overwrote the final result")
	}
	if q.fail("worker-a", ticket.id) {
		t.Fatal("a failure replaced an already delivered result")
	}
	wav, rate, err := q.wait(context.Background(), ticket.id)
	if err != nil || !bytes.Equal(wav, wavStub) || rate != 24000 {
		t.Fatalf("wait = %q/%d/%v, want the first result", wav, rate, err)
	}
}

func TestPreviewQueueSkipsExpiredDeadline(t *testing.T) {
	q := newPreviewQueue()
	now := time.Now()
	q.now = func() time.Time { return now }
	ticket, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Người dùng đã thôi chờ nhưng mục chưa quá TTL: không đánh thức máy đọc.
	now = now.Add(previewWaitTimeout + time.Second)
	if taken := q.take("worker-a"); taken != nil {
		t.Fatalf("a preview past its wait deadline was claimed: %+v", taken)
	}
	if _, _, err := q.wait(context.Background(), ticket.id); !errors.Is(err, errPreviewExpired) {
		t.Fatalf("wait on a dropped preview = %v, want errPreviewExpired", err)
	}
}

func TestPreviewQueueRejectsResultPastDeadline(t *testing.T) {
	q := newPreviewQueue()
	now := time.Now()
	q.now = func() time.Time { return now }
	ticket, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	claimed := q.take("worker-a")
	if claimed == nil || claimed.id != ticket.id {
		t.Fatalf("take = %+v, want ticket %s", claimed, ticket.id)
	}
	// Quá mốc người dùng thôi chờ thì kết quả về muộn không được thắng race với
	// wait; TTL 2 phút chỉ còn giữ mục để dọn, không mở lại quyền ghi.
	now = now.Add(previewWaitTimeout)
	if q.complete("worker-a", claimed.id, wavStub, 24000) {
		t.Fatal("complete accepted a result past the browser deadline")
	}
	if q.fail("worker-a", claimed.id) {
		t.Fatal("fail accepted a result past the browser deadline")
	}
}

func TestPreviewQueueExpiresOldTickets(t *testing.T) {
	q := newPreviewQueue()
	now := time.Now()
	q.now = func() time.Time { return now }
	ticket, err := q.enqueue(voice.SynthRequest{Text: "Xin chào"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	now = now.Add(previewTicketTTL + time.Second)
	if taken := q.take("worker-a"); taken != nil {
		t.Fatalf("expired preview was handed out: %+v", taken)
	}
	if _, _, err := q.wait(context.Background(), ticket.id); !errors.Is(err, errPreviewExpired) {
		t.Fatalf("wait on an expired preview = %v", err)
	}
}

func TestPreviewWAVGuards(t *testing.T) {
	if looksLikeWAV([]byte("not audio at all")) {
		t.Fatal("a non-RIFF payload passed the WAV check")
	}
	if !looksLikeWAV(wavStub) {
		t.Fatal("a RIFF/WAVE payload failed the WAV check")
	}
	if previewSampleRate("44100") != 44100 {
		t.Fatal("a valid sample rate was not kept")
	}
	if previewSampleRate("") != previewRateDefault || previewSampleRate("7") != previewRateDefault {
		t.Fatal("an invalid sample rate did not fall back to the default")
	}
}
