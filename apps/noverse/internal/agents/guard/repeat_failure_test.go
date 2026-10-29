package guard

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// countingExecutor đếm số lần tool thật sự được chạy.
type countingExecutor struct {
	runs int
	err  error
}

func (c *countingExecutor) next(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	c.runs++
	if c.err != nil {
		return nil, c.err
	}
	return json.RawMessage(`{"ok":true}`), nil
}

// TestRepeatFailureMiddlewareBlocksIdenticalRetry: lời gọi y nguyên lỗi
// repeatFailureLimit lần thì lần sau bị chặn TRƯỚC khi chạy tool.
//
// Trước khi có middleware: kiến trúc sư gọi novel_context {"chapter":0} bốn
// lượt với cùng một lỗi rồi thử 1/25/-1/9/100/2147483647 — mỗi lượt là một
// request LLM đầy đủ mà không có tiến triển (tui.log 2026-10-26 22:24–22:31).
func TestRepeatFailureMiddlewareBlocksIdenticalRetry(t *testing.T) {
	mw := NewRepeatFailureMiddleware()
	exec := &countingExecutor{err: errors.New("context payload exceeds budget")}
	call := llmtypes.ToolCall{ID: "call_1", Name: "novel_context", Args: json.RawMessage(`{"chapter":0}`)}

	for i := 0; i < repeatFailureLimit; i++ {
		if _, err := mw(context.Background(), call, exec.next); err == nil {
			t.Fatalf("lần %d: muốn lỗi từ tool", i+1)
		}
	}
	if exec.runs != repeatFailureLimit {
		t.Fatalf("tool chạy %d lần, want %d", exec.runs, repeatFailureLimit)
	}

	_, err := mw(context.Background(), call, exec.next)
	if err == nil || !strings.Contains(err.Error(), "sẽ không thành công") {
		t.Fatalf("lần thứ %d phải bị chặn kèm lý do, got %v", repeatFailureLimit+1, err)
	}
	if exec.runs != repeatFailureLimit {
		t.Fatalf("tool vẫn chạy (%d lần) dù đã bị chặn", exec.runs)
	}

	// Khoá là (tool, args): cách gọi khác vẫn phải chạy được.
	other := llmtypes.ToolCall{ID: "call_2", Name: "novel_context", Args: json.RawMessage(`{}`)}
	if _, err := mw(context.Background(), other, exec.next); err == nil {
		t.Fatal("muốn lỗi từ tool cho lời gọi khác tham số")
	}
	if exec.runs != repeatFailureLimit+1 {
		t.Fatalf("lời gọi khác tham số không được chạy, runs=%d", exec.runs)
	}
}

// TestRepeatFailureMiddlewareResetsOnSuccess: một lần thành công xoá bộ đếm,
// nên chuỗi lỗi sau đó lại được phép chạy đủ repeatFailureLimit lần.
func TestRepeatFailureMiddlewareResetsOnSuccess(t *testing.T) {
	mw := NewRepeatFailureMiddleware()
	call := llmtypes.ToolCall{ID: "call_1", Name: "read_chapter", Args: json.RawMessage(`{"chapter":8,"source":"final"}`)}

	failing := &countingExecutor{err: errors.New("boom")}
	if _, err := mw(context.Background(), call, failing.next); err == nil {
		t.Fatal("muốn lỗi từ tool")
	}
	ok := &countingExecutor{}
	if _, err := mw(context.Background(), call, ok.next); err != nil {
		t.Fatalf("lần chạy thành công: %v", err)
	}
	// Sau thành công, bộ đếm về 0 → được chạy lại đủ repeatFailureLimit lần.
	again := &countingExecutor{err: errors.New("boom")}
	for i := 0; i < repeatFailureLimit; i++ {
		if _, err := mw(context.Background(), call, again.next); err == nil {
			t.Fatalf("lần %d sau khi reset: muốn lỗi từ tool", i+1)
		}
	}
	if again.runs != repeatFailureLimit {
		t.Fatalf("bộ đếm không reset: runs=%d, want %d", again.runs, repeatFailureLimit)
	}
	if _, err := mw(context.Background(), call, again.next); err == nil {
		t.Fatal("lần thứ ba liên tiếp phải bị chặn")
	}
	if again.runs != repeatFailureLimit {
		t.Fatalf("tool vẫn chạy sau khi bị chặn: runs=%d", again.runs)
	}
}
