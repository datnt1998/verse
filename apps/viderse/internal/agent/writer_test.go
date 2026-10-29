package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	llmtypes "github.com/datnt1998/verse/core/llm"

	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/jackc/pgx/v5/pgtype"
)

// memStore implements RunStore over maps — enough to prove the run lifecycle
// and script persistence without a database.
type memStore struct {
	mu      sync.Mutex
	runs    map[[16]byte]gen.AgentRun
	scripts []gen.Script
	finish  chan struct{}
}

func newMemStore() *memStore {
	return &memStore{runs: map[[16]byte]gen.AgentRun{}, finish: make(chan struct{}, 8)}
}

func (s *memStore) CreateRun(_ context.Context, arg gen.CreateRunParams) (gen.AgentRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := gen.AgentRun{ID: arg.ID, Status: "queued", Provider: arg.Provider, Model: arg.Model, Brief: arg.Brief}
	s.runs[arg.ID.Bytes] = row
	return row, nil
}

func (s *memStore) GetRun(_ context.Context, id pgtype.UUID) (gen.AgentRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.runs[id.Bytes]
	if !ok {
		return gen.AgentRun{}, fmt.Errorf("run không tồn tại")
	}
	return row, nil
}

func (s *memStore) MarkRunRunning(_ context.Context, id pgtype.UUID) (gen.AgentRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.runs[id.Bytes]
	if !ok {
		return gen.AgentRun{}, fmt.Errorf("run không tồn tại")
	}
	if row.Status != "queued" {
		return gen.AgentRun{}, fmt.Errorf("run không ở trạng thái queued")
	}
	row.Status = "running"
	s.runs[id.Bytes] = row
	return row, nil
}

func (s *memStore) FinishRun(_ context.Context, arg gen.FinishRunParams) (gen.AgentRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.runs[arg.ID.Bytes]
	if !ok {
		return gen.AgentRun{}, fmt.Errorf("run không tồn tại")
	}
	row.Status = arg.Status
	row.InputTokens = arg.InputTokens
	row.OutputTokens = arg.OutputTokens
	row.CostUsd = arg.CostUsd
	row.Error = arg.Error
	s.runs[arg.ID.Bytes] = row
	select {
	case s.finish <- struct{}{}:
	default:
	}
	return row, nil
}

func (s *memStore) CreateScript(_ context.Context, arg gen.CreateScriptParams) (gen.Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := gen.Script{
		ID: arg.ID, RunID: arg.RunID, Title: arg.Title, Sections: arg.Sections,
		FullText: arg.FullText, WordCount: arg.WordCount, EstDurationSec: arg.EstDurationSec,
	}
	s.scripts = append(s.scripts, row)
	return row, nil
}

// scriptedModel phát một stream duy nhất: assistant gọi save_script rồi dừng.
type scriptedModel struct {
	toolCall llmtypes.ToolCall
	usage    *llmtypes.Usage
	calls    int
}

func (m *scriptedModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return nil, fmt.Errorf("unexpected Generate call")
}

func (m *scriptedModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	m.calls++
	ch := make(chan llmtypes.StreamEvent, 8)
	go func() {
		defer close(ch)
		partial := llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("drafting")}}
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventToolCallStart, Message: partial}
		partial.Content = append(partial.Content, llmtypes.ToolCallBlock(m.toolCall))
		tc := m.toolCall
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventToolCallEnd, Message: partial, CompletedToolCall: &tc}
		ch <- llmtypes.StreamEvent{
			Type: llmtypes.StreamEventDone,
			Message: llmtypes.Message{
				Role:       llmtypes.RoleAssistant,
				Content:    partial.Content,
				StopReason: llmtypes.StopReasonToolUse,
				Usage:      m.usage,
			},
		}
	}()
	return ch, nil
}

func (m *scriptedModel) SupportsTools() bool  { return true }
func (m *scriptedModel) ProviderName() string { return "openai-codex" }
func (m *scriptedModel) ModelName() string    { return "gpt-6-luna" }

func validScriptArgs(t *testing.T) json.RawMessage {
	t.Helper()
	args, err := json.Marshal(map[string]any{
		"title": "Vì sao đế chế La Mã sụp đổ",
		"sections": []map[string]any{
			{"kind": "hook", "title": "Mở đầu", "narration": "Đế chế hùng mạnh nhất thời cổ đại sụp đổ không phải trong một ngày.", "rehook": false, "cta": false, "scenes": []any{}},
			{"kind": "part", "title": "Khủng hoảng bên trong", "narration": "", "rehook": true, "cta": false, "scenes": []map[string]any{
				{"narration": "Nội chiến liên miên làm suy yếu quân đội và ngân khố.", "visual_note": "bản đồ đế chế rạn vỡ"},
				{"narration": "Lạm phát bào mòn đồng tiền và niềm tin của người dân.", "visual_note": "đồng xu cổ"},
			}},
			{"kind": "ending_cta", "title": "Bài học", "narration": "Đăng ký kênh để xem phần tiếp theo về Đông La Mã.", "rehook": false, "cta": true, "scenes": []any{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return args
}

func testBrief() Brief {
	return Brief{Topic: "Vì sao đế chế La Mã sụp đổ", Audience: "người thích lịch sử", TargetSeconds: 90}
}

// TestWriterRunSucceedsAndPersistsScript chứng minh toàn vòng đời: submit →
// queued → running → save_script → agent_runs succeeded với token > 0 và một
// dòng scripts đã đo đúng lời đọc.
func TestWriterRunSucceedsAndPersistsScript(t *testing.T) {
	store := newMemStore()
	model := &scriptedModel{
		toolCall: llmtypes.ToolCall{ID: "tc-1", Name: saveScriptToolName, Args: validScriptArgs(t)},
		usage:    &llmtypes.Usage{Input: 120, Output: 80, TotalTokens: 200, Cost: &llmtypes.Cost{Total: 0.0042}},
	}
	r := NewRunner(store, func(string) (llmtypes.ChatModel, string, string, error) {
		return model, "openai-codex", "gpt-6-luna", nil
	}, "system prompt", nil)

	id, err := r.Submit(context.Background(), testBrief())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	select {
	case <-store.finish:
	case <-time.After(10 * time.Second):
		t.Fatal("run không kết thúc sau 10s")
	}
	cancel()
	r.Wait()

	row, err := store.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded (err=%q)", row.Status, row.Error.String)
	}
	if row.InputTokens != 120 || row.OutputTokens != 80 {
		t.Fatalf("tokens = %d/%d, want 120/80", row.InputTokens, row.OutputTokens)
	}
	if !row.CostUsd.Valid || row.CostUsd.Float64 != 0.0042 {
		t.Fatalf("cost_usd = %+v, want 0.0042", row.CostUsd)
	}
	if len(store.scripts) != 1 {
		t.Fatalf("scripts = %d, want 1", len(store.scripts))
	}
	sc := store.scripts[0]
	if sc.RunID != id {
		t.Fatalf("script run_id khác run")
	}
	var sections []script.Section
	if err := json.Unmarshal(sc.Sections, &sections); err != nil {
		t.Fatalf("sections JSON: %v", err)
	}
	// Lời đọc của phần chia cảnh là scenes narration, không phải narration tóm tắt.
	want := script.MeasureScript(sections, 150)
	if sc.WordCount != int32(want.WordCount) {
		t.Fatalf("word_count = %d, want %d", sc.WordCount, want.WordCount)
	}
	if sc.FullText != want.FullText {
		t.Fatalf("full_text khác measure")
	}
	if model.calls != 1 {
		t.Fatalf("model calls = %d, want 1", model.calls)
	}
}

// TestWriterRunFailsWithoutSaveScript: model trả lời xong mà không gọi
// save_script → run failed kèm lý do, không có dòng scripts.
func TestWriterRunFailsWithoutSaveScript(t *testing.T) {
	store := newMemStore()
	model := &scriptedModel{
		toolCall: llmtypes.ToolCall{ID: "tc-1", Name: "some_other_tool", Args: json.RawMessage(`{}`)},
		usage:    &llmtypes.Usage{Input: 10, Output: 5},
	}
	r := NewRunner(store, func(string) (llmtypes.ChatModel, string, string, error) {
		return model, "openai-codex", "gpt-6-luna", nil
	}, "system prompt", nil)
	id, err := r.Submit(context.Background(), testBrief())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	select {
	case <-store.finish:
	case <-time.After(10 * time.Second):
		t.Fatal("run không kết thúc sau 10s")
	}
	cancel()
	r.Wait()

	row, err := store.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed", row.Status)
	}
	if !row.Error.Valid || row.Error.String == "" {
		t.Fatal("error phải được ghi")
	}
	if len(store.scripts) != 0 {
		t.Fatalf("scripts = %d, want 0", len(store.scripts))
	}
}

// TestWriterRunFailsOnModelError: lỗi credential/provider đi thẳng vào
// agent_runs.error — đúng smoke "không đăng nhập → run failed sạch".
func TestWriterRunFailsOnModelError(t *testing.T) {
	store := newMemStore()
	calls := 0
	r2 := NewRunner(store, func(string) (llmtypes.ChatModel, string, string, error) {
		calls++
		if calls == 1 {
			// lần resolve lúc enqueue
			return nil, "openai-codex", "gpt-6-luna", nil
		}
		return nil, "openai-codex", "gpt-6-luna", fmt.Errorf("chưa đăng nhập openai-codex — chạy viderse login")
	}, "system prompt", nil)
	id, err := r2.Submit(context.Background(), testBrief())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r2.Start(ctx)
	select {
	case <-store.finish:
	case <-time.After(10 * time.Second):
		t.Fatal("run không kết thúc sau 10s")
	}
	cancel()
	r2.Wait()

	row, err := store.GetRun(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed", row.Status)
	}
	if got := row.Error.String; got != "dựng model: chưa đăng nhập openai-codex — chạy viderse login" {
		t.Fatalf("error = %q", got)
	}
}
