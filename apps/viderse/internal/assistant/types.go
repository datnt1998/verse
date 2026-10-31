package assistant

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Scope là nơi người dùng đang đứng khi nhắn: một video, một kênh hoặc mọi kênh.
// JSON: {"jobId":…} | {"channelId":…} | "all".
type Scope struct {
	JobID     string
	ChannelID string
	All       bool
}

func (s *Scope) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == `"all"` {
		*s = Scope{All: true}
		return nil
	}
	var obj struct {
		JobID     string `json:"jobId"`
		ChannelID string `json:"channelId"`
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&obj); err != nil {
		return errors.New("scope phải là {jobId}, {channelId} hoặc \"all\"")
	}
	if (obj.JobID == "") == (obj.ChannelID == "") {
		return errors.New("scope cần đúng một trong jobId, channelId")
	}
	*s = Scope{JobID: obj.JobID, ChannelID: obj.ChannelID}
	return nil
}

func (s Scope) kind() string {
	switch {
	case s.All:
		return "all"
	case s.JobID != "":
		return "job"
	default:
		return "channel"
	}
}

// Cost là giá dự kiến của một bước (chỉ ảnh; không bịa tiền).
type Cost struct {
	Images int `json:"images"`
}

// Rerun là phần việc sẽ chạy lại khi làm bước này (dòng “Làm lại” của thẻ).
type Rerun struct {
	Agent string `json:"agent"`
	Label string `json:"label"`
}

// Step là một bước đã phân giải: params chỉ chứa id thật; title/sub/detail là
// chữ tiếng Việt dựng từ dữ liệu thật.
type Step struct {
	ID            string          `json:"id"`
	Intent        IntentID        `json:"intent"`
	Params        json.RawMessage `json:"params"`
	Title         string          `json:"title"`
	Sub           string          `json:"sub"`
	Agent         string          `json:"agent"`
	Cost          *Cost           `json:"cost,omitempty"`
	EnqueuesVideo bool            `json:"enqueuesVideo"`
	Detail        string          `json:"detail"`
	Reruns        []Rerun         `json:"reruns"`
}

// Total là dòng tổng của thẻ kế hoạch.
type Total struct {
	Images int    `json:"images"`
	Note   string `json:"note,omitempty"`
}

// Plan là kế hoạch 1–5 bước, CHƯA chạy gì.
type Plan struct {
	ID              string `json:"planId"`
	Steps           []Step `json:"steps"`
	Total           Total  `json:"total"`
	ImagesLeftToday *int   `json:"imagesLeftToday,omitempty"`
}

// Option là một lựa chọn của câu hỏi làm rõ: bấm là gửi lại `Text` như câu mới.
type Option struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// Ask là câu hỏi làm rõ khi thiếu thông tin.
type Ask struct {
	Question string   `json:"question"`
	Options  []Option `json:"options"`
}

// Suggestion là gợi ý khi câu chưa làm được: luôn là một câu hợp lệ cho phạm vi.
type Suggestion struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	Icon  string `json:"icon,omitempty"`
}

// Link là lối tắt tới nơi làm việc đó (vd. Kênh › Hồ sơ kênh).
type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// Unknown là “Chưa làm được việc này” + đúng 2 gợi ý.
type Unknown struct {
	Lead        string       `json:"lead"`
	Suggestions []Suggestion `json:"suggestions"`
	Link        *Link        `json:"link,omitempty"`
}

// Reply là một trong ba kết quả của plan: kế hoạch, câu hỏi, hoặc chưa hiểu.
type Reply struct {
	Plan    *Plan
	Ask     *Ask
	Unknown *Unknown
}

// Mã lỗi gửi cho client.
const (
	CodeTextInvalid      = "text_invalid"
	CodeNotFound         = "not_found"
	CodeModelUnavailable = "model_unavailable"
	CodeRateLimited      = "rate_limited"
	CodePlanExpired      = "plan_expired"
	CodeAssistantFailed  = "assistant_failed"
	CodeInvalid          = "invalid_request"
	CodeConflict         = "conflict"
)

// Error là lỗi của chính trợ lý (không phải của service bên dưới).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func fail(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// StepError là lỗi của một bước khi chạy: Message đã là câu tiếng Việt cho
// người dùng; Err là nguyên nhân gốc (để web chọn mã HTTP).
type StepError struct {
	Message string
	Err     error
}

func (e *StepError) Error() string { return e.Message }
func (e *StepError) Unwrap() error { return e.Err }

// userMessage biến lỗi service thành câu tiếng Việt. Lỗi lạ KHÔNG lộ chi tiết
// nội bộ; bên gọi đã ghi Diag.
func userMessage(err error) string {
	var ce *domain.CloneError
	if errors.As(err, &ce) && ce.Message != "" {
		if ce.Recovery != nil && *ce.Recovery != "" {
			return strings.TrimSpace(ce.Message) + ". " + *ce.Recovery
		}
		return ce.Message
	}
	var che *channels.Error
	if errors.As(err, &che) && che.Message != "" {
		return che.Message
	}
	var se *StepError
	if errors.As(err, &se) {
		return se.Message
	}
	return "Chưa làm được việc này. Thử lại sau ít phút."
}
