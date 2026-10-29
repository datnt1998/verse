// Package llmretry 是直接模型调用共用的请求层重试内核：仅重试模型适配器
// 明确标记为 retryable 的错误，遵守 Retry-After/指数退避，并经 ToolProgress
// 把进度送入既有工作台观察链。账户、鉴权、权限等终止错误会立即返回；
// retryable 错误持续重试，生命周期只由 context 控制。
package llmretry

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/datnt1998/verse/core/agentcore"
)

const maxRetryDelay = 60 * time.Second

// Gate 是共享的可唤醒退避闸门（G4）。等待者登记后阻塞到退避到点、被 Wake
// 提前放行或 context 结束；Host 据 Wake 的返回值判断"是否有请求在等待重试"。
type Gate struct {
	mu      sync.Mutex
	ch      chan struct{}
	waiting int
}

// NewGate 创建闸门。
func NewGate() *Gate { return &Gate{ch: make(chan struct{}, 1)} }

// Wait 等待 d 或被 Wake 提前放行；ctx 结束返回其错误。
func (g *Gate) Wait(ctx context.Context, d time.Duration) error {
	if g == nil {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	g.mu.Lock()
	g.waiting++
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.waiting--
		g.mu.Unlock()
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.ch:
		return nil
	case <-timer.C:
		return nil
	}
}

// Wake 提前放行一个等待者。没有等待者时返回 false 且不留下残留信号，
// 避免下一次退避被上一次的空点击提前打断。
func (g *Gate) Wake() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	waiting := g.waiting
	g.mu.Unlock()
	if waiting == 0 {
		return false
	}
	select {
	case g.ch <- struct{}{}:
	default:
	}
	return true
}

// Waiting 返回当前等待者数量。
func (g *Gate) Waiting() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.waiting
}

// Generator 是请求重试所需的最小模型接口。
type Generator interface {
	Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error)
}

// Event 描述一次即将发生的请求重试。
type Event struct {
	Attempt int
	Delay   time.Duration
	Err     error
}

// Config 配置重试的可观测信息，不改变重试语义。
type Config struct {
	Agent   string
	OnRetry func(Event)
	// Gate 可选：退避等待经共享闸门进行，Host 可提前放行（G4）。nil 时用独立计时器。
	Gate *Gate
}

// Generate 调用 model.Generate。retryable 错误退避后持续重试，直到成功或
// context 结束；非 retryable 错误立即返回。
func Generate(ctx context.Context, model Generator, cfg Config, messages []llmtypes.Message, opts ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	for retry := 1; ; retry++ {
		resp, err := model.Generate(ctx, messages, nil, opts...)
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !isRetryable(err) {
			return nil, err
		}

		delay := retryDelay(err, retry-1)
		event := Event{Attempt: retry, Delay: delay, Err: err}
		if cfg.OnRetry != nil {
			cfg.OnRetry(event)
		}
		meta, _ := json.Marshal(struct {
			DelayMS int64 `json:"retry_delay_ms"`
		}{DelayMS: delay.Milliseconds()})
		agentcore.ReportToolProgress(ctx, agentcore.ProgressPayload{
			Kind:    agentcore.ProgressRetry,
			Agent:   cfg.Agent,
			Attempt: retry,
			Message: err.Error(),
			Meta:    meta,
		})
		if cfg.Gate != nil {
			if err := cfg.Gate.Wait(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func isRetryable(err error) bool {
	var retryable agentcore.RetryableError
	return errors.As(err, &retryable) && retryable.Retryable()
}

func retryDelay(err error, attempt int) time.Duration {
	var hinter agentcore.RetryHinter
	if errors.As(err, &hinter) {
		if delay := hinter.RetryAfter(); delay > 0 {
			if delay > maxRetryDelay {
				return maxRetryDelay
			}
			return delay
		}
	}
	delay := time.Second
	for i := 0; i < attempt && delay < maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}
