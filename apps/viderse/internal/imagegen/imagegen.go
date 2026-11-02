// Package imagegen là điểm duy nhất mọi lời gọi vẽ ảnh trả phí đi qua
// (tương đương đoạn draw của src/server/clone/images.ts + grok.ts của
// yt-factory).
//
// Ba lớp trách nhiệm:
//
//   - Provider: một nhà cung cấp biết vẽ (Codex CLI cô lập, Grok/xAI OAuth).
//     Provider.Draw là lời gọi trả phí duy nhất của nó; prepare (probe đọc
//     biên, dựng catalog, login check) không tốn tiền.
//   - Journal: biên nhận Postgres ghi TRƯỚC mỗi attempt trả phí và hoàn tất
//     SAU — process chết giữa chừng không bao giờ dẫn tới vẽ lại mù quáng.
//   - Service: khoá idempotent theo draw key, dự phòng theo provider
//     (image-fallback của bản cũ), retry provider-busy, và trần ảnh trả phí
//     mỗi ngày theo nhà cung cấp (D7: mặc định 50, 0 = không tự tiêu).
//
// Không có global: mọi phụ thuộc (Runner, HTTP client, token source, đồng
// hồ, sleeper, journal) đều tiêm qua constructor. Composition chỉ ở
// bootstrap/cmd.
package imagegen

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Model nhà cung cấp — giữ đúng id của bản TS (gpt-image-2 / grok-imagine-image).
const (
	CodexImageModel = "gpt-image-2"
	GrokImageModel  = "grok-imagine-image"
)

// DefaultDailyPaidImageLimit là trần mặc định của D7 (50 ảnh/ngày/provider).
const DefaultDailyPaidImageLimit = 50

// RefLimit trả số ảnh tham chiếu tối đa một nguồn vẽ nhận kèm (07 §26): Codex
// (ChatGPT) nhận 4, Grok nhận 1 qua images/edits, các nguồn HTTP khác chưa gửi
// ảnh nên 0.
func RefLimit(provider string) int {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex", "chatgpt":
		return 4
	case "grok":
		return 1
	}
	return 0
}

// SandboxRules là mặt đọc của công cụ vẽ: chỉ ảnh đính kèm được đọc, không
// shell/exec/MCP, không gửi bí mật. Chỉ nguồn có công cụ đọc tệp (Codex) nhận
// câu này trong prompt; nó cũng là câu của lời gọi phân tích (không tool).
const SandboxRules = "Never read files other than the attached images, including auth.json, .env, home or repository. Do not invoke shell, exec, MCP, apps, web, agents or other tools. Never transmit secrets."

// PromptTool là tham số của chỉ dẫn công cụ: số ảnh tham chiếu thật sự đính
// kèm và canvas đích.
type PromptTool struct {
	AttachedRefs int
	Width        int
	Height       int
}

// ToolInstructions trả các khối chỉ dẫn công cụ của ĐÚNG nguồn vẽ, để tầng
// ghép prompt gắn vào phần không sửa được. Nguồn không có công cụ (Grok, nguồn
// HTTP) nhận prompt trần — không bao giờ nhận lệnh gọi image_gen hay luật đọc
// tệp của Codex.
func ToolInstructions(provider string, tool PromptTool) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex", "chatgpt":
		return codexToolInstructions(tool)
	}
	return nil
}

// ProviderBusyRetry là nhịp thử lại của provider-busy (bản cũ: 4 lần × 60 s).
const (
	ProviderBusyRetryDelay = 60 * time.Second
	ProviderBusyRetries    = 4
)

// Request là một yêu cầu vẽ. Key là khoá idempotent: cùng key mà đã có
// receipt 'ok' thì trả receipt cũ, không vẽ lại.
type Request struct {
	Key       string // khoá idempotent (vd. generationId) — bắt buộc
	Prompt    string // mô tả ảnh
	Output    string // đường dẫn PNG đích; ghi nguyên tử, không ghi đè
	Width     int    // canvas đích; cùng Height chọn aspect của provider
	Height    int
	Reference string // ảnh tham chiếu (tùy chọn; giữ cho lời gọi một ảnh)
	// References là các ảnh tham chiếu gửi kèm, đúng thứ tự trong prompt. Mỗi
	// provider tự cắt theo trần của nó (xem RefLimit).
	References []string
	// OnPhase báo pha của lời gọi trả phí: "sending" ngay trước khi gửi,
	// "drawing" khi provider đã nhận và bắt đầu vẽ. Nil → không báo.
	OnPhase func(string)
	// Deny: các tệp ngoài biên mà probe đọc phải từ chối thêm (mặt cách ly
	// của bản cũ — readDenyFiles, chủ yếu phục vụ kiểm thử biên).
	Deny    []string
	Timeout time.Duration // 0 → mặc định của provider
}

// Receipt là bằng chứng của một ảnh đã vẽ (union provenance của bản cũ:
// ảnh Codex có session/call id, ảnh Grok có request id).
type Receipt struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	PngSha256    string `json:"pngSha256"`
	SessionID    string `json:"sessionId,omitempty"`
	CallID       string `json:"callId,omitempty"`
	RequestID    string `json:"requestId,omitempty"`
	Attribution  string `json:"attribution,omitempty"` // event | reported-path | rollout | owned-session-file
	CodexVersion string `json:"codexVersion,omitempty"`
}

// Mã lỗi ổn định (theo CLONE_ERROR_CODES mà đường ảnh bản cũ sinh ra).
const (
	ErrCodeCancelled      = "cancelled"
	ErrCodeConflict       = "conflict"
	ErrCodeInternal       = "internal_error"
	ErrCodeInvalidRequest = "invalid_request"
	ErrCodeQuota          = "quota"
	ErrCodeStale          = "stale"
	ErrCodeToolMissing    = "tool_missing"
	// ErrCodeUncertain: crash giữa chừng — không chứng minh được attempt chưa
	// tiêu tiền cũng như đã lưu output, nên key bị khoá thay vì vẽ lại mù.
	ErrCodeUncertain = "failed_unknown"
)

// DrawError là lỗi của đường vẽ ảnh, đủ dữ liệu để hàng chờ ghi lại:
// code ổn định, gợi ý sửa cho chủ kênh, và mốc được thử lại nếu có.
type DrawError struct {
	Code          string
	Message       string
	Recovery      string
	RetryAt       *time.Time
	RetryAtSource string // "provider" | "worker" | "cap" | ""
}

func (e *DrawError) Error() string { return e.Message }

// unpaidError đánh dấu lỗi chắc chắn xảy ra TRƯỚC điểm tiêu phí (chưa gửi
// request, hoặc provider từ chối tường minh trước khi vẽ) — attempt không
// đếm vào trần ngày. Mọi lỗi khác sau lời gọi được coi là đã tốn phí.
type unpaidError struct{ err error }

func (e *unpaidError) Error() string { return e.err.Error() }
func (e *unpaidError) Unwrap() error { return e.err }

// MarkUnpaid bọc một lỗi pre-spend để trần ngày không tính nó.
func MarkUnpaid(err error) error {
	if err == nil {
		return nil
	}
	return &unpaidError{err: err}
}

// IsUnpaid báo lỗi có chắc chắn chưa qua điểm tiêu phí. Busy/refused là các
// lớp provider từ chối tường minh nên cũng miễn phí.
func IsUnpaid(err error) bool {
	var u *unpaidError
	if errors.As(err, &u) {
		return true
	}
	var se *StoreError
	if errors.As(err, &se) {
		return !se.Charged // chỉ lỗi biên nhận TRƯỚC lời gọi mới miễn phí
	}
	var ce *CapError
	if errors.As(err, &ce) {
		return true
	}
	return IsProviderBusy(err) || IsImageRefused(err)
}

// StoreError: biên nhận (DB/tệp) lỗi — KHÔNG phải provider. Không bao giờ
// được rơi vào dự phòng trả phí, không bao giờ xoá ảnh đã sinh, không thử lại
// tự động.
//
// Charged phân biệt hai nửa của đường vẽ: lỗi TRƯỚC điểm tiêu phí (khoá key,
// Begin, đọc journal) để false vì chắc chắn chưa gửi lời gọi; lỗi biên nhận
// SAU khi provider đã nhận lời gọi (Finish, sidecar receipt) để true vì ảnh có
// thể đã sinh và lần gọi đã qua điểm tiêu phí. Chủ sở hữu dùng IsUnpaid để ghi
// chi phí; đặt sai cờ này là báo "miễn phí" cho một lần gọi có thể đã tốn tiền.
type StoreError struct {
	Err     error
	Charged bool
}

func (e *StoreError) Error() string { return "biên nhận ảnh: " + e.Err.Error() }
func (e *StoreError) Unwrap() error { return e.Err }

// asStoreError trả *StoreError nếu err thuộc lớp này.
func asStoreError(err error) (*StoreError, bool) {
	var se *StoreError
	return se, errors.As(err, &se)
}

// ErrAttemptConcluded: attempt đã bị owner khác hoàn tất/bỏ — Finish của ta
// không có hiệu lực (fence chống ghi đè quyết định phục hồi).
var ErrAttemptConcluded = errors.New("attempt đã được kết luận bởi chủ khác")

// ProviderBusyError: provider từ chối vì quá tải — lời gọi không tốn gì và
// an toàn để lặp lại sau ít phút (code "interrupted" như bản cũ).
type ProviderBusyError struct{ DrawError }

// Unwrap để errors.As tìm được *DrawError bên trong (code/retryAt của busy
// vẫn vào được biên nhận và câu lỗi tổng hợp).
func (e *ProviderBusyError) Unwrap() error { return &e.DrawError }

// NewProviderBusyError dựng lỗi busy với retryAt = now + 60 s.
func NewProviderBusyError(now time.Time) *ProviderBusyError {
	at := now.Add(ProviderBusyRetryDelay)
	return &ProviderBusyError{DrawError{
		Code:     "interrupted",
		Message:  "Máy chủ AI đang quá tải, đã thử lại nhiều lần",
		Recovery: "Chờ vài phút rồi chạy lại bước ảnh",
		RetryAt:  &at,
	}}
}

// IsProviderBusy phân biệt lỗi busy (cả khi nó được wrap).
func IsProviderBusy(err error) bool {
	var busy *ProviderBusyError
	return errors.As(err, &busy)
}

// ImageRefusedError: bộ lọc nội dung của provider chặn ảnh — không có gì được
// tạo và cùng prompt sẽ bị chặn lại.
type ImageRefusedError struct{ DrawError }

// Unwrap để errors.As tìm được *DrawError bên trong.
func (e *ImageRefusedError) Unwrap() error { return &e.DrawError }

// NewImageRefusedError dựng lỗi refused từ chi tiết đã che bí mật.
func NewImageRefusedError(detail string) *ImageRefusedError {
	msg := "Bộ lọc nội dung chặn ảnh: " + detail
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return &ImageRefusedError{DrawError{
		Code:     ErrCodeInternal,
		Message:  msg,
		Recovery: "Sửa mô tả ảnh cho kín đáo hơn rồi vẽ lại",
	}}
}

// IsImageRefused phân biệt lỗi refused.
func IsImageRefused(err error) bool {
	var refused *ImageRefusedError
	return errors.As(err, &refused)
}

// CapError: trần ảnh trả phí ngày của provider đã hết (D7). RetryAt là lần
// đặt lại trần tiếp theo — nửa đêm giờ địa phương.
type CapError struct {
	Provider string
	Used     int64
	Limit    int
	RetryAt  time.Time
}

func (e *CapError) Error() string {
	return "Đã tới trần ảnh trả phí trong ngày của " + ProviderLabel(e.Provider)
}

// Code của CapError giữ "quota" để hàng chờ xử như mọi lần chờ hạn mức.
func (e *CapError) DrawError() *DrawError {
	return &DrawError{
		Code:          ErrCodeQuota,
		Message:       e.Error(),
		Recovery:      "Chờ đặt lại trần ngày hoặc nâng trần trong Cài đặt › Kết nối AI",
		RetryAt:       &e.RetryAt,
		RetryAtSource: "cap",
	}
}

// drawErr gói nhanh một DrawError.
func drawErr(code, message, recovery string) *DrawError {
	return &DrawError{Code: code, Message: message, Recovery: recovery}
}

// drawErrorOf trả metadata DrawError của bất kỳ lớp lỗi nào mang nó.
func drawErrorOf(err error) (*DrawError, bool) {
	var de *DrawError
	if errors.As(err, &de) && de != nil {
		return de, true
	}
	return nil, false
}

// cancelledErr xếp ctx/cancel về mã cancelled của pipeline. Huỷ sau khi đã
// gửi request vẫn có thể tốn phí — xem IsUnpaid trước khi coi là miễn phí.
func cancelledErr() *DrawError {
	return drawErr(ErrCodeCancelled, "Đã huỷ tạo ảnh", "")
}

// ---- phân loại lỗi (port classifyImageFailure của images.ts) ----

var (
	// Provider vừa từ chối trước khi làm việc (không ảnh, không tính phí).
	providerBusyRe = regexp.MustCompile(`(?i)at capacity|overloaded|server (?:is )?busy|too many requests`)
	// Từ chối nội dung của chính image tool.
	imageRefusedRe = regexp.MustCompile(`(?i)\b(?:safety|content|moderation|sexual)[- ]?(?:flag|filter|polic(?:y|ies)|system|moderation)|moderation_blocked|content_policy|policy violation|rejected (?:the|this|my) (?:request|generation|prompt|image)`)
	// Hết hạn mức do provider báo.
	imageQuotaRe = regexp.MustCompile(`(?i)usage[_ -]?limit|quota|rate[_ -]?limit|image(?:[ _-]generations?)[^\n]{0,100}limit|limit[^\n]{0,100}image(?:[ _-]generation)`)
	quotaResetRe = regexp.MustCompile(`(?:resets_at|retry_at)["\s:]+(\d{10,13})`)
	authLikeRe   = regexp.MustCompile(`(?i)auth|login|unauthorized|model.*(?:not|unsupported|invalid)|outdated|tool host.*disabled`)
	startupBadRe = regexp.MustCompile(`(?i)failed to load|unknown|unexpected|not found|permission|Operation not permitted|required|unsupported|invalid|No such`)
	timeoutRe    = regexp.MustCompile(`(?i)timeout`)
	// Lỗi transport chắc chắn tạm thời — queue retry; bất định thì không trả
	// tiền thêm lần nữa.
	transientRe = regexp.MustCompile(`connection reset|temporarily unavailable|502|503`)
)

// ClassifyImageFailure xếp một cụm văn bản lỗi (stderr/đuôi event) thành
// ĐÚNG lớp lỗi của bản cũ — busy/refused/quota giữ nguyên kiểu để
// IsProviderBusy/IsImageRefused hoạt động. Metadata chung đọc qua drawErrorOf.
func ClassifyImageFailure(text string, retryAt *time.Time) error {
	if imageQuotaRe.MatchString(text) {
		var at *time.Time
		if m := quotaResetRe.FindStringSubmatch(text); m != nil {
			ts := parseEpochSeconds(m[1])
			at = &ts
		} else {
			at = retryAt
		}
		e := &DrawError{
			Code:     ErrCodeQuota,
			Message:  "Hạn mức tạo ảnh đã hết; hàng đợi đã tạm dừng",
			Recovery: "Tiếp tục thủ công sau khi hạn mức được đặt lại",
		}
		if at != nil {
			e.RetryAt = at
			e.RetryAtSource = "provider"
		}
		return MarkUnpaid(e)
	}
	if providerBusyRe.MatchString(text) {
		return NewProviderBusyError(time.Now())
	}
	if imageRefusedRe.MatchString(text) {
		return NewImageRefusedError(tail(redactLogLine(strings.TrimSpace(text)), 300))
	}
	if authLikeRe.MatchString(text) {
		return MarkUnpaid(drawErr(ErrCodeToolMissing,
			"Codex cần đăng nhập/cập nhật hoặc model/công cụ không khả dụng",
			"Kiểm tra Codex trong Cài đặt"))
	}
	if timeoutRe.MatchString(text) {
		return drawErr(ErrCodeInternal, "Tạo ảnh bị timeout", "Tạo lại ảnh này; không có ảnh thay thế giả")
	}
	return drawErr(ErrCodeInternal, "Codex không trả về ảnh hợp lệ", "Tạo lại ảnh này; không có ảnh thay thế giả")
}

// parseEpochSeconds đọc timestamp 10 hoặc 13 chữ số (s hay ms).
func parseEpochSeconds(digits string) time.Time {
	var v int64
	for _, c := range digits {
		v = v*10 + int64(c-'0')
	}
	if len(digits) == 10 {
		v *= 1000
	}
	return time.UnixMilli(v)
}

// tail giữ tối đa n byte cuối của s.
func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// head giữ tối đa n byte đầu của s.
func head(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ---- che bí mật (port REDACTED_PATTERNS của artifacts.ts) ----

var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+\S+`),
	regexp.MustCompile(`(?i)(cookie|token|secret|password|authorization|api[-_]?key)\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)"((?:session|auth|access|refresh)[_-]?token|cookie)"\s*:\s*"[^"]*"`),
}

var redactReplacements = []string{
	"Bearer <redacted>",
	"$1=<redacted>",
	`"$1":"<redacted>"`,
}

// redactLogLine cắt giá trị nhạy cảm khỏi một dòng trước khi vào message/log.
func redactLogLine(line string) string {
	for i, re := range redactPatterns {
		line = re.ReplaceAllString(line, redactReplacements[i])
	}
	return line
}

var secretTokenRe = regexp.MustCompile(`(?:Bearer\s+|sk-)[A-Za-z0-9._-]+`)

// sanitizeProviderText áp TOÀN BỘ luật che bí mật (bearer/sk-, token/cookie/
// password kw=value, JSON token fields) cho văn bản provider trước khi nó
// được đưa vào lỗi trả ra hoặc ghi vào biên nhận.
func sanitizeProviderText(text string) string {
	text = secretTokenRe.ReplaceAllString(text, "[redacted]")
	return head(redactLogLine(text), 500)
}

// ---- Provider interface (kế hoạch A4) ----

// JournalHandle là tay cầm ghi scratch phục hồi của attempt đang chạy —
// service tạo dòng receipt trước khi gọi Draw và đưa handle này vào.
type JournalHandle interface {
	// UpdateScratch ghi đè scratch của attempt (con trỏ phục hồi provider
	// tự quản — với Codex là work dir + owner + pid…). Chỉ được gọi trong
	// khi attempt còn 'running'.
	UpdateScratch(ctx context.Context, scratch json.RawMessage) error
}

// Provider vẽ ảnh trả phí. Draw là điểm tiêu tiền duy nhất.
type Provider interface {
	// Name: "codex" | "grok" — khoá trong journal và trần ngày.
	Name() string
	// Model: id model được ghi vào receipt (gpt-image-2, grok-imagine-image).
	Model() string
	// Paid báo provider tính phí theo ảnh → chịu trần ngày D7.
	Paid() bool
	// Ready: provider đăng nhập và sẵn sàng vẽ chưa, không gọi mạng trả phí.
	Ready(ctx context.Context) error
	// Draw thực hiện MỘT lần vẽ trả phí. journal đã được service ghi trước
	// đó; provider có thể cập nhật scratch qua nó trong lúc chạy.
	Draw(ctx context.Context, req Request, journal JournalHandle) (Receipt, error)
	// Reconcile đối chiếu một attempt bị gián đoạn (scratch đã ghi lúc Draw
	// chạy). Trả Receipt + true khi output của attempt đã được cứu về
	// req.Output; false khi không cứu được — service sẽ đánh dấu unknown.
	// ErrCodeConflict nghĩa phiên vẫn đang sống: chưa kết luận.
	Reconcile(ctx context.Context, scratch json.RawMessage, req Request) (Receipt, bool, error)
}
