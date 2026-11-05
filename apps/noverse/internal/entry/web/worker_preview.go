package web

import (
	"io"
	"net/http"
	"time"
)

// Nghe thử giọng qua máy đọc pull (10-audio.md §8.4 #6). Ba endpoint riêng,
// cùng credential worker + capability 'tts' như mọi đường khác của listener
// riêng; máy chủ không mở đường nào vào máy Mac.
//
//	GET  /internal/worker/previews/next        → 200 {id, voiceId, text, tone, quality, speed, deadline} | 204
//	POST /internal/worker/previews/{id}/audio  (raw WAV, X-Sample-Rate)
//	POST /internal/worker/previews/{id}/fail   {"message":"..."}
//
// Không có hàng đợi bền: một lượt nghe thử chỉ sống trong lúc người dùng đang
// chờ, và mất nó không mất dữ liệu truyện. Lượt chỉ được nhận bởi đúng máy đã
// nhận nó và chỉ một kết quả cuối cùng được ghi.

// handlePreviewNext trao một lượt nghe thử đang chờ cho máy đọc đang hỏi (GET
// /internal/worker/previews/next). 204 = không có gì để đọc. `deadline` là lúc
// người dùng thôi chờ: máy đọc phải đọc xong trước mốc đó.
func (a *WorkerAPI) handlePreviewNext(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	t := hostedPreviews.take(worker.ID)
	if t == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       t.id,
		"voiceId":  t.req.VoiceID,
		"text":     t.req.Text,
		"tone":     t.req.Tone,
		"quality":  t.req.Quality,
		"speed":    t.req.Speed,
		"deadline": t.deadline.UTC().Format(time.RFC3339Nano),
	})
}

// handlePreviewAudio nhận tệp WAV máy đọc vừa tạo (POST
// /internal/worker/previews/{id}/audio). Kích thước và định dạng đều bị chặn:
// thứ được phục vụ lại cho trình duyệt phải đúng là WAV, và chỉ máy đã nhận
// lượt này mới ghi được kết quả.
func (a *WorkerAPI) handlePreviewAudio(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	body, err := io.ReadAll(io.LimitReader(r.Body, previewWAVMax+1))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Không đọc được tệp nghe thử")
		return
	}
	if len(body) == 0 {
		jsonError(w, http.StatusBadRequest, "Tệp nghe thử rỗng")
		return
	}
	if len(body) > previewWAVMax {
		jsonError(w, http.StatusRequestEntityTooLarge, "Tệp nghe thử quá lớn")
		return
	}
	if !looksLikeWAV(body) {
		jsonError(w, http.StatusBadRequest, "Tệp nghe thử không phải WAV")
		return
	}
	rate := previewSampleRate(r.Header.Get("X-Sample-Rate"))
	if !hostedPreviews.complete(worker.ID, r.PathValue("id"), body, rate) {
		// Người dùng đã rời đi, lượt đã có kết quả, hoặc máy khác nhận lượt này.
		jsonError(w, http.StatusNotFound, "Lượt nghe thử đã hết hạn")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePreviewFail ghi nhận máy đọc không đọc được lượt này (POST
// /internal/worker/previews/{id}/fail). Lý do kỹ thuật không đi tiếp ra UI.
func (a *WorkerAPI) handlePreviewFail(w http.ResponseWriter, r *http.Request, worker WorkerIdentity) {
	var body struct {
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !hostedPreviews.fail(worker.ID, r.PathValue("id")) {
		jsonError(w, http.StatusNotFound, "Lượt nghe thử đã hết hạn")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
