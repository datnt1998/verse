package web

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Nghe thử giọng ở bản máy chủ (10-audio.md §8.4 #6): máy chủ không có bộ đọc,
// nên một lượt nghe thử là một yêu cầu ngắn đi qua đúng kênh pull của máy đọc.
// Máy Mac hỏi "có gì nghe thử không?", tự đọc bằng bộ đọc của nó rồi trả tệp WAV
// về; trình duyệt chờ trong một khoảng ngắn rồi nhận thẳng audio/wav.
//
// Hàng đợi nằm trong bộ nhớ tiến trình, không phải một bảng: nghe thử giọng là
// thao tác tức thời và KHÔNG lưu vào truyện (khác nghe thử đoạn — thao tác đó
// tạo và GIỮ đoạn). Mất một mục khi máy chủ khởi động lại chỉ tốn một lần bấm.
// Số mục, độ dài chữ, kích thước WAV và thời gian chờ đều bị chặn; không có thử
// lại tự động và không cache (`Cache-Control: no-store`, y như đường nghe thử
// cục bộ).
const (
	// previewQueueMax là số lượt nghe thử đang chờ/đang đọc tối đa. Máy đọc là
	// một cái Mac duy nhất; xếp thêm chỉ làm người sau chờ vô ích.
	previewQueueMax = 8
	// previewTextMaxRunes: một câu nghe thử, không phải cả chương.
	previewTextMaxRunes = 400
	// previewVoiceMaxRunes chặn id giọng do client gửi.
	previewVoiceMaxRunes = 120
	// previewToneMaxRunes chặn sắc thái do client gửi.
	previewToneMaxRunes = 40
	// previewWAVMax: một câu chỉ vài trăm KB; 8 MB là trần an toàn.
	previewWAVMax = 8 << 20
	// previewTicketTTL là hạn giữ một mục trong hàng đợi.
	previewTicketTTL = 2 * time.Minute
	// previewWaitTimeout: người dùng đang đứng chờ, không giữ request lâu hơn.
	// Đây cũng là hạn chót máy đọc được phép dùng cho lượt đó.
	previewWaitTimeout = 30 * time.Second
	previewRateMin     = 8000
	previewRateMax     = 96000
	previewRateDefault = 24000
)

var (
	errPreviewQueueFull = errors.New("đang có nhiều lượt nghe thử khác")
	errPreviewExpired   = errors.New("lượt nghe thử đã hết hạn")
	errPreviewTimeout   = errors.New("máy đọc chưa trả lời kịp")
	errPreviewFailed    = errors.New("máy đọc không đọc được đoạn này")
)

// previewTicket là một lượt nghe thử đang chờ máy đọc trả lời. Mọi trường thay
// đổi sau khi tạo (taken, workerID, terminal, kết quả) chỉ được ghi dưới q.mu;
// `done` đóng đúng một lần khi có kết quả cuối cùng.
type previewTicket struct {
	id        string
	req       voice.SynthRequest
	createdAt time.Time
	// deadline là lúc trình duyệt thôi chờ. Quá mốc này lượt chưa ai nhận thì bỏ
	// luôn — không đánh thức máy đọc cho một người đã rời đi.
	deadline time.Time

	taken    bool
	workerID string
	terminal bool

	done       chan struct{}
	wav        []byte
	sampleRate int
	failed     bool
}

// PreviewQueue là điểm hẹn giữa request nghe thử công khai và máy đọc pull.
// Một tiến trình chỉ có một hàng đợi (public listener và worker listener nằm
// chung process), nên dùng singleton như voice.Shared() thay vì luồn thêm một
// tham số qua cả hai listener.
type PreviewQueue struct {
	mu   sync.Mutex
	seq  int64
	next []*previewTicket
	byID map[string]*previewTicket
	now  func() time.Time
}

func newPreviewQueue() *PreviewQueue {
	return &PreviewQueue{byID: map[string]*previewTicket{}, now: time.Now}
}

// hostedPreviews là hàng đợi nghe thử của tiến trình máy chủ.
var hostedPreviews = newPreviewQueue()

// enqueue nhận một lượt nghe thử; từ chối khi hàng đợi đã đầy thay vì xếp
// thêm một người chờ không có hạn.
func (q *PreviewQueue) enqueue(req voice.SynthRequest) (*previewTicket, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now()
	q.pruneLocked(now)
	if len(q.byID) >= previewQueueMax {
		return nil, errPreviewQueueFull
	}
	q.seq++
	t := &previewTicket{
		id:        strconv.FormatInt(q.seq, 10) + "-" + rand.Text(),
		req:       req,
		createdAt: now,
		deadline:  now.Add(previewWaitTimeout),
		done:      make(chan struct{}),
	}
	q.next = append(q.next, t)
	q.byID[t.id] = t
	return t, nil
}

// take trao cho máy đọc `workerID` lượt nghe thử lâu nhất còn chờ. nil = không
// có gì. Chỉ máy đã nhận mới được ghi kết quả cho lượt đó.
func (q *PreviewQueue) take(workerID string) *previewTicket {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.now()
	q.pruneLocked(now)
	for _, t := range q.next {
		if t.taken || t.terminal || !now.Before(t.deadline) {
			continue
		}
		t.taken = true
		t.workerID = workerID
		return t
	}
	return nil
}

// complete gắn tệp WAV máy đọc `workerID` vừa đọc. false = lượt đã hết hạn, đã
// có kết quả cuối cùng, chưa được nhận, hoặc do máy khác nhận.
func (q *PreviewQueue) complete(workerID, id string, wav []byte, sampleRate int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.byID[id]
	if !ok || t.terminal || !t.taken || t.workerID != workerID || q.expiredLocked(t) || !q.now().Before(t.deadline) {
		return false
	}
	// Kết quả đầu tiên là kết quả cuối cùng; kết quả về sau bị từ chối thay vì
	// ghi đè lên thứ người dùng đã/đang nhận.
	t.terminal = true
	t.wav, t.sampleRate = wav, sampleRate
	close(t.done)
	return true
}

// fail đánh dấu máy đọc không đọc được, cũng chỉ một lần và đúng máy đã nhận.
// Lý do kỹ thuật ở lại phía máy chủ: người dùng chỉ nhận một câu chung
// (10-audio.md §8.5).
func (q *PreviewQueue) fail(workerID, id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.byID[id]
	if !ok || t.terminal || !t.taken || t.workerID != workerID || q.expiredLocked(t) || !q.now().Before(t.deadline) {
		return false
	}
	t.terminal = true
	t.failed = true
	close(t.done)
	return true
}

// wait chờ máy đọc trả lời một lượt nghe thử. Huỷ request (người dùng đóng
// trang) hoặc quá hạn thì bỏ mục luôn, để kết quả về muộn không bị phục vụ.
func (q *PreviewQueue) wait(ctx context.Context, id string) ([]byte, int, error) {
	q.mu.Lock()
	t, ok := q.byID[id]
	q.mu.Unlock()
	if !ok {
		return nil, 0, errPreviewExpired
	}
	timer := time.NewTimer(previewWaitTimeout)
	defer timer.Stop()
	select {
	case <-t.done:
		// Đọc kết quả dưới khoá: complete/fail ghi dưới khoá trước khi đóng
		// `done`, nên không có lần ghi nào chen vào sau khi đọc.
		q.mu.Lock()
		wav, sampleRate, failed := t.wav, t.sampleRate, t.failed
		q.dropLocked(id)
		q.mu.Unlock()
		if failed {
			return nil, 0, errPreviewFailed
		}
		return wav, sampleRate, nil
	case <-ctx.Done():
		q.drop(id)
		return nil, 0, ctx.Err()
	case <-timer.C:
		q.drop(id)
		return nil, 0, errPreviewTimeout
	}
}

// drop bỏ một mục khỏi hàng đợi (người dùng đã rời đi hoặc đã nhận kết quả).
func (q *PreviewQueue) drop(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dropLocked(id)
}

func (q *PreviewQueue) dropLocked(id string) {
	delete(q.byID, id)
	for i, t := range q.next {
		if t.id == id {
			q.next = append(q.next[:i], q.next[i+1:]...)
			return
		}
	}
}

// pruneLocked dọn mục quá hạn và mục chưa ai nhận mà người dùng đã thôi chờ.
func (q *PreviewQueue) pruneLocked(now time.Time) {
	for id, t := range q.byID {
		if q.expiredLocked(t) || (!t.taken && !now.Before(t.deadline)) {
			q.dropLocked(id)
		}
	}
}

func (q *PreviewQueue) expiredLocked(t *previewTicket) bool {
	return q.now().Sub(t.createdAt) > previewTicketTTL
}

// previewSampleRate đọc X-Sample-Rate máy đọc gửi kèm tệp WAV. Giá trị thiếu
// hoặc vô lý rơi về mức mặc định của bộ đọc, không chặn một tệp hợp lệ.
func previewSampleRate(raw string) int {
	rate, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || rate < previewRateMin || rate > previewRateMax {
		return previewRateDefault
	}
	return rate
}

// looksLikeWAV chỉ nhận đúng vỏ RIFF/WAVE: một tệp khác định dạng không được
// phục vụ như audio/wav (trình duyệt sẽ im lặng hoặc báo lỗi khó hiểu).
func looksLikeWAV(b []byte) bool {
	return len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WAVE"
}
