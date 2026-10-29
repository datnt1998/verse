package llmretry

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/datnt1998/noverse/internal/agent"
)

// DefaultMaxRetries 是模型包装器的开流前重试上限，与 agents.subagentMaxRetries
// 保持一致（7）。agentcore 自身的 MaxRetries 保留给"流中途"失败：包装器只接管
// 首个事件之前的失败，耗尽后把错误标记为不可重试，避免同一错误被两层各重试 N 次。
const DefaultMaxRetries = 7

// RetryingModel 在 agentcore.ChatModel 外层重试"开流前"的瞬时失败（G4）：
// Generate 直接报错，或 GenerateStream 返回错误 / 首个事件即错误（尚未转发任何
// 增量）。等待走共享 Gate，Host.RetryNow 可提前放行。流中途的错误原样透传，交给
// agentcore 自己的重试；上下文取消与上下文溢出原样透传。
type RetryingModel struct {
	Inner      llmtypes.ChatModel
	Gate       *Gate
	Agent      string // 进度行的归属（角色名），空则用 "model"
	MaxRetries int    // 不含首次尝试
}

// NewRetryingModel 构造包装器；gate 为 nil 时使用独立计时器。
func NewRetryingModel(inner llmtypes.ChatModel, gate *Gate, agent string, maxRetries int) *RetryingModel {
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	return &RetryingModel{Inner: inner, Gate: gate, Agent: agent, MaxRetries: maxRetries}
}

func (m *RetryingModel) Generate(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	for attempt := 0; ; attempt++ {
		resp, err := m.Inner.Generate(ctx, messages, tools, opts...)
		if err == nil {
			return resp, nil
		}
		if m.terminal(ctx, err) || !m.retryable(err) || attempt >= m.MaxRetries {
			return nil, m.finalError(err, attempt)
		}
		m.report(ctx, attempt+1, err)
		if waitErr := m.wait(ctx, err, attempt); waitErr != nil {
			return nil, waitErr
		}
	}
}

func (m *RetryingModel) GenerateStream(ctx context.Context, messages []llmtypes.Message, tools []llmtypes.ToolSpec, opts ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	for attempt := 0; ; attempt++ {
		ch, err := m.Inner.GenerateStream(ctx, messages, tools, opts...)
		if err == nil {
			first, ok := <-ch
			if !ok {
				// 空流：原样交回（agentcore 会按 PartialStreamError 处理）。
				out := make(chan llmtypes.StreamEvent)
				close(out)
				return out, nil
			}
			if first.Type == llmtypes.StreamEventError && first.Err != nil &&
				!m.terminal(ctx, first.Err) && m.retryable(first.Err) && attempt < m.MaxRetries {
				go func() {
					for range ch {
					}
				}() // 排空失败流，避免生产方阻塞
				m.report(ctx, attempt+1, first.Err)
				if waitErr := m.wait(ctx, first.Err, attempt); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return replay(first, ch), nil
		}
		if m.terminal(ctx, err) || !m.retryable(err) || attempt >= m.MaxRetries {
			return nil, m.finalError(err, attempt)
		}
		m.report(ctx, attempt+1, err)
		if waitErr := m.wait(ctx, err, attempt); waitErr != nil {
			return nil, waitErr
		}
	}
}

func (m *RetryingModel) SupportsTools() bool { return m.Inner.SupportsTools() }

// ProviderName 转发底层适配器的可选接口，保证 agentcore 的 per-provider API key
// 解析与模型名展示不因包装而丢失。
func (m *RetryingModel) ProviderName() string {
	if pn, ok := m.Inner.(llmtypes.ProviderNamer); ok {
		return pn.ProviderName()
	}
	return ""
}

func (m *RetryingModel) ModelName() string {
	if mn, ok := m.Inner.(llmtypes.ModelNamer); ok {
		return mn.ModelName()
	}
	return ""
}

// terminal 表示不应由包装器重试的错误：调用方取消/超时、上下文溢出（交给
// agentcore 的压缩恢复）。
func (m *RetryingModel) terminal(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var overflow *llmtypes.ContextOverflowError
	return errors.As(err, &overflow)
}

// retryable 镜像 agentcore 的判定：错误链声明 Retryable()，或流未完成
// （PartialStreamError）。未声明者一律不重试。
func (m *RetryingModel) retryable(err error) bool {
	var pse *agentcore.PartialStreamError
	if errors.As(err, &pse) {
		return true
	}
	var r interface{ Retryable() bool }
	return errors.As(err, &r) && r.Retryable()
}

// finalError 在包装器已重试过之后把可重试错误标记为不可重试，防止 agentcore
// 再按自己的 MaxRetries 重来一轮。
func (m *RetryingModel) finalError(err error, attempt int) error {
	if attempt > 0 && m.retryable(err) {
		return &exhaustedError{err: err}
	}
	return err
}

// exhaustedError 保留原始错误链（errors.Is/As 可用）但显式声明不可重试。
type exhaustedError struct{ err error }

func (e *exhaustedError) Error() string   { return e.err.Error() }
func (e *exhaustedError) Unwrap() error   { return e.err }
func (e *exhaustedError) Retryable() bool { return false }

// report 产生与 agentcore EventRetry 相同的活动行，并标记 canRetryNow=true。
func (m *RetryingModel) report(ctx context.Context, attempt int, err error) {
	agent := m.Agent
	if agent == "" {
		agent = "model"
	}
	meta, _ := json.Marshal(map[string]any{
		"retry_delay_ms": retryDelay(err, attempt-1).Milliseconds(),
		"can_retry_now":  true,
	})
	agentcore.ReportToolProgress(ctx, agentcore.ProgressPayload{
		Kind:       agentcore.ProgressRetry,
		Agent:      agent,
		Attempt:    attempt,
		MaxRetries: m.MaxRetries,
		Message:    err.Error(),
		Meta:       meta,
	})
}

// wait 经共享 Gate 等待退避；Gate 为空时退化为独立计时器。
func (m *RetryingModel) wait(ctx context.Context, err error, attempt int) error {
	delay := retryDelay(err, attempt)
	if m.Gate != nil {
		return m.Gate.Wait(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// replay 把已读出的首事件与剩余流合并，保持事件顺序不变。
func replay(first llmtypes.StreamEvent, rest <-chan llmtypes.StreamEvent) <-chan llmtypes.StreamEvent {
	out := make(chan llmtypes.StreamEvent)
	go func() {
		defer close(out)
		out <- first
		for ev := range rest {
			out <- ev
		}
	}()
	return out
}
