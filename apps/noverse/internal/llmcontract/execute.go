package llmcontract

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/llmretry"
)

// FailureKind 区分不可由同一次结构化反馈修复的失败边界。
type FailureKind string

const (
	FailureRequest  FailureKind = "request"
	FailureProtocol FailureKind = "protocol"
	FailureLength   FailureKind = "length"
	FailureSafety   FailureKind = "safety"
	FailureContract FailureKind = "contract"
)

// Failure 保留失败类别和模型原始输出，供调用方决定日志、工件和 UI 表达。
type Failure struct {
	Kind     FailureKind
	Contract string
	Raw      string
	Err      error
}

func (e *Failure) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		return e.Contract
	}
	if e.Contract == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Contract, e.Err)
}

func (e *Failure) Unwrap() error { return e.Err }

// Correction 描述一次模型可修复的输出错误。Attempt 是刚失败的调用序号。
type Correction struct {
	Attempt int
	Layer   string
	Mode    Mode
	Raw     string
	Err     error
}

// Hooks 只负责可观测性，不改变执行语义。
type Hooks struct {
	Resolved     func(Resolution)
	RequestRetry func(llmretry.Event)
	Correction   func(Correction)
}

// Request 定义一次直接结构化返回。Contract 是结构的单一来源，Validate 只处理
// JSON Schema 无法表达的业务约束。
type Request[T any] struct {
	Contract     Contract
	SystemPrompt string
	Payload      string
	Options      []llmtypes.CallOption
	Validate     func(*T) error
	Agent        string
	Hooks        Hooks
	// RetryGate 可选：请求退避等待经共享闸门进行，Host 可提前放行（G4）。
	RetryGate *llmretry.Gate
	// MaxCorrections 是 schema/业务反馈重问的上限（不含 llmretry 的请求层重试）。
	// 0 或负值用 defaultMaxCorrections；耗尽后返回 FailureContract 并携带最后一份原始输出。
	MaxCorrections int
}

const promptCorrection = "Đầu ra ở trên không khớp JSON Schema. Hãy sửa theo lỗi và chỉ xuất một JSON object đầy đủ, không giải thích, không dùng Markdown fence."
const semanticCorrection = "JSON ở trên đúng cấu trúc nhưng giá trị các trường chưa qua kiểm tra nghiệp vụ. Hãy sửa theo lỗi và xuất lại JSON object đầy đủ."

const defaultMaxCorrections = 3

// Execute 统一完成协议选择、提示词准备、请求重试、停止原因分类、Schema/DTO
// 解码和业务反馈自愈。prompt 模式的格式/Schema 错误以及两种模式的业务错误会
// 反馈给模型重问，重问次数上限为 MaxCorrections（默认 defaultMaxCorrections），
// 耗尽后以 FailureContract 返回；原生契约违约会立即暴露。
func Execute[T any](ctx context.Context, model llmretry.Generator, req Request[T]) (T, error) {
	var zero T
	if model == nil {
		return zero, &Failure{Kind: FailureProtocol, Contract: req.Contract.Name, Err: errors.New("Mô hình chưa được cấu hình")}
	}

	schemaOptions, resolution := Plan(model, req.Contract)
	systemPrompt, err := PreparePrompt(req.SystemPrompt, req.Contract, resolution)
	if err != nil {
		return zero, &Failure{Kind: FailureContract, Contract: req.Contract.Name, Err: fmt.Errorf("chuẩn bị hợp đồng đầu ra: %w", err)}
	}
	if req.Hooks.Resolved != nil {
		req.Hooks.Resolved(resolution)
	}

	messages := []llmtypes.Message{
		llmtypes.SystemMsg(systemPrompt),
		llmtypes.UserMsg(req.Payload),
	}
	options := append(schemaOptions, req.Options...)
	native := resolution.Mode == ModeNativeJSONSchema

	maxCorrections := req.MaxCorrections
	if maxCorrections <= 0 {
		maxCorrections = defaultMaxCorrections
	}
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		resp, err := llmretry.Generate(ctx, model, llmretry.Config{
			Agent:   req.Agent,
			OnRetry: req.Hooks.RequestRetry,
			Gate:    req.RetryGate,
		}, messages, options...)
		if err != nil {
			if ctx.Err() != nil {
				return zero, ctx.Err()
			}
			return zero, &Failure{Kind: FailureRequest, Contract: req.Contract.Name, Err: err}
		}
		if resp == nil {
			return zero, &Failure{Kind: FailureProtocol, Contract: req.Contract.Name, Err: errors.New("Mô hình trả về phản hồi rỗng")}
		}

		raw := resp.Message.TextContent()
		switch resp.Message.StopReason {
		case llmtypes.StopReasonLength:
			return zero, &Failure{Kind: FailureLength, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Đầu ra của mô hình bị cắt vì độ dài (stop_reason=length)")}
		case llmtypes.StopReasonSafety:
			return zero, &Failure{Kind: FailureSafety, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Mô hình từ chối trả lời hoặc bị lọc nội dung (stop_reason=safety)")}
		case llmtypes.StopReasonError:
			return zero, &Failure{Kind: FailureProtocol, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Mô hình kết thúc với trạng thái lỗi (stop_reason=error)")}
		case llmtypes.StopReasonToolUse:
			return zero, &Failure{Kind: FailureProtocol, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Lệnh gọi có cấu trúc bất ngờ trả về lệnh gọi công cụ (stop_reason=tool_use)")}
		case llmtypes.StopReasonAborted:
			return zero, &Failure{Kind: FailureProtocol, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Lệnh gọi mô hình bị hủy (stop_reason=aborted)")}
		}

		// 先剥掉开头的 <think>/<thinking> 块：reasoning 模型偶尔把思考混入正文。
		// 只有开标签没有闭标签是协议错误，照常反馈给模型修正。
		stripped, thinkErr := stripLeadingThink(raw)
		body := strings.TrimSpace(stripped)
		if native {
			if body == "" && thinkErr == nil {
				return zero, &Failure{Kind: FailureContract, Contract: req.Contract.Name, Raw: raw, Err: errors.New("Schema gốc trả về nội dung rỗng")}
			}
		} else {
			body = ExtractJSONObject(stripped)
		}

		layer := "schema"
		var cause error
		if thinkErr != nil {
			layer, cause = "decode", thinkErr
		} else if body == "" {
			layer, cause = "decode", errors.New("Không tìm thấy JSON object trong đầu ra")
		} else if err := ValidateJSON(req.Contract.Schema, []byte(body)); err != nil {
			cause = err
		} else {
			var out T
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				// Schema 已通过而 DTO 无法解码，说明静态契约与 Go 类型不一致，
				// 继续要求模型重写无法修复代码缺陷。
				return zero, &Failure{Kind: FailureContract, Contract: req.Contract.Name, Raw: raw, Err: fmt.Errorf("schema không khớp DTO: %w", err)}
			}
			if req.Validate == nil {
				return out, nil
			}
			if err := req.Validate(&out); err == nil {
				return out, nil
			} else {
				layer, cause = "semantic", err
			}
		}

		if native && layer != "semantic" && thinkErr == nil {
			return zero, &Failure{Kind: FailureContract, Contract: req.Contract.Name, Raw: raw, Err: fmt.Errorf("vi phạm hợp đồng schema gốc: %w", cause)}
		}
		if attempt > maxCorrections {
			return zero, &Failure{Kind: FailureContract, Contract: req.Contract.Name, Raw: raw, Err: fmt.Errorf("đầu ra vẫn lỗi sau %d lần tự sửa: %w", maxCorrections, cause)}
		}
		correction := Correction{Attempt: attempt, Layer: layer, Mode: resolution.Mode, Raw: raw, Err: cause}
		if req.Hooks.Correction != nil {
			req.Hooks.Correction(correction)
		}
		hint := promptCorrection
		if layer == "semantic" {
			hint = semanticCorrection
		}
		messages = append(messages,
			llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock(raw)}},
			llmtypes.UserMsg(hint+"\nLỗi: "+cause.Error()),
		)
	}
}

// thinkBlockRE 只匹配开头的完整 <think>…</think> 或 <thinking>…</thinking> 块
// （可叠多层、忽略大小写、允许前导空白）；不剥正文中间的标签。
var thinkBlockRE = regexp.MustCompile(`(?is)\A\s*<think(?:ing)?(?:\s[^>]*)?>.*?</think(?:ing)?\s*>`)

// thinkOpenRE 识别剥完后仍以开标签起头但缺闭标签的残缺输出。
var thinkOpenRE = regexp.MustCompile(`(?i)\A\s*<think(?:ing)?[\s>]`)

// stripLeadingThink 剥掉 raw 开头的一串 think 块，返回剩余正文；只剩开标签没有
// 闭标签时返回错误，作为可由模型修正的协议错误走正常反馈循环。
func stripLeadingThink(raw string) (string, error) {
	s := raw
	for {
		loc := thinkBlockRE.FindStringIndex(s)
		if loc == nil || loc[0] != 0 {
			break
		}
		s = s[loc[1]:]
	}
	if thinkOpenRE.MatchString(s) {
		return raw, errors.New("Đầu ra mở bằng thẻ <think> không có thẻ đóng; hãy chỉ xuất JSON object đầy đủ, không kèm phần suy nghĩ")
	}
	return s, nil
}

// ExtractJSONObject 返回文本中的第一个平衡 JSON 对象，字符串中的花括号不计入层级。
func ExtractJSONObject(raw string) string {
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return ""
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(raw); i++ {
		switch c := raw[i]; {
		case inString && escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case !inString && c == '{':
			depth++
		case !inString && c == '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
