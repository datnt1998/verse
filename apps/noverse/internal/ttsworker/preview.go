package ttsworker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Nghe thử giọng theo yêu cầu (10-audio.md §8.4 #6). Máy chủ không có bộ đọc,
// nên nó để một câu ngắn vào hàng đợi trong bộ nhớ; máy này hỏi lấy, tự đọc
// bằng bộ đọc cục bộ rồi trả WAV về. Không có daemon mới, không có đường vào
// máy Mac, không có nhà cung cấp ngoài.

// previewTextLimit chặn câu nghe thử ngay trên máy đọc: máy chủ đã chặn, nhưng
// đây là máy phải trả giá bằng CPU nếu ai đó gửi một chương thay vì một câu.
const previewTextLimit = 400

// previewTimeout: một câu nghe thử không được giữ máy đọc lâu hơn mức này.
// Nếu bộ đọc treo, chương đang xếp hàng vẫn phải chạy tiếp — nghe thử không
// được phép chặn việc của người dùng.
const previewTimeout = 60 * time.Second

// previewRequest là một lượt nghe thử máy chủ trao
// (GET /internal/worker/previews/next).
type previewRequest struct {
	ID      string  `json:"id"`
	VoiceID string  `json:"voiceId"`
	Text    string  `json:"text"`
	Tone    string  `json:"tone"`
	Quality string  `json:"quality"`
	Speed   float64 `json:"speed"`
	// Deadline là lúc người dùng thôi chờ; máy đọc không giữ lượt quá mốc này.
	Deadline string `json:"deadline"`
}

// servePreview lấy và đọc nhiều nhất MỘT lượt nghe thử đang chờ. true = có làm
// việc. Vòng Run gọi đúng một lần mỗi vòng rồi VẪN thử nhận chương: nghe thử
// chen trước cho người đang chờ tiếng, nhưng không bao giờ bỏ đói chương.
//
// Chỉ vòng Run gọi hàm này (đầu vòng và giữa hai đoạn của chương), nên bộ đọc
// không bao giờ bị dùng đồng thời: không có goroutine thứ hai, không cần thêm
// một cổng suy luận. Tim đập của job vẫn chạy trong lúc nghe thử.
//
// Lỗi mạng hoặc lượt đã hết hạn đều bỏ qua: nghe thử là thao tác tức thời,
// không có thử lại tự động và không được làm dừng worker.
func (w *Worker) servePreview(ctx context.Context) bool {
	if !w.cfg.Preview || !w.Ready() {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, previewTimeout)
	defer cancel()
	status, raw, err := w.do(ctx, http.MethodGet, "/internal/worker/previews/next", nil, nil)
	if err != nil || status != http.StatusOK {
		return false
	}
	var req previewRequest
	if json.Unmarshal(raw, &req) != nil || req.ID == "" || strings.TrimSpace(req.Text) == "" {
		return false
	}
	if len([]rune(req.Text)) > previewTextLimit {
		w.failPreview(ctx, req.ID, errors.New("câu nghe thử quá dài"))
		return true
	}
	// Máy chủ nói rõ lúc người dùng thôi chờ. Ghép mốc đó vào context để lượt
	// nghe thử không giữ bộ đọc sau khi chẳng còn ai nghe.
	if deadline, parseErr := time.Parse(time.RFC3339Nano, req.Deadline); parseErr == nil {
		deadlineCtx, stop := context.WithDeadline(ctx, deadline)
		defer stop()
		ctx = deadlineCtx
	}
	rendered, err := w.cfg.Engine.Synthesize(ctx, voice.SynthRequest{
		Text: req.Text, VoiceID: req.VoiceID, Tone: req.Tone,
		Quality: req.Quality, Speed: req.Speed,
	}.Normalized())
	if err != nil || len(rendered.WAV) == 0 {
		w.failPreview(ctx, req.ID, err)
		return true
	}
	sampleRate := rendered.SampleRate
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	headers := map[string]string{
		"Content-Type":  "audio/wav",
		"X-Sample-Rate": strconv.Itoa(sampleRate),
	}
	_, _, _ = w.do(ctx, http.MethodPost, "/internal/worker/previews/"+url.PathEscape(req.ID)+"/audio", rendered.WAV, headers)
	return true
}

// failPreview báo máy chủ là lượt này không đọc được, để người dùng nhận câu
// lỗi ngay thay vì chờ hết hạn.
func (w *Worker) failPreview(ctx context.Context, id string, cause error) {
	message := "không đọc được đoạn nghe thử"
	if cause != nil {
		message = cause.Error()
	}
	body, _ := json.Marshal(map[string]string{"message": message})
	_, _, _ = w.do(ctx, http.MethodPost, "/internal/worker/previews/"+url.PathEscape(id)+"/fail", body, nil)
}
