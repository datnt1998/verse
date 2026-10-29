// Postgres integration test: provisions an isolated schema via pgtest with
// the viderse role pair, applies the embedded migrations, then proves the
// whole run lifecycle against real tables (app role does DML, migrate role
// owns DDL).
//
// Skips unless TEST_DATABASE_URL is set; REQUIRE_POSTGRES_TEST=1 fails instead
// of skipping so acceptance jobs can't pass on a skipped suite.
package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	llmtypes "github.com/datnt1998/verse/core/llm"

	"github.com/datnt1998/verse/apps/viderse/internal/agent"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Viderse's own role pair and lock keys (noverse uses 2710,11 — these must
// not collide when suites run in parallel on the shared container).
var roleSpec = pgtest.RoleSpec{
	MigrateRole:   "viderse_migrate",
	AppRole:       "viderse_app",
	LockKey1:      2790,
	LockKey2:      1,
	InitScriptRef: "deploy/postgres-init/01-roles.sh",
}

func provision(t *testing.T) *pgxpool.Pool {
	t.Helper()
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t, roleSpec)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res, err := pgmigrate.Up(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("first migrate up applied no migrations")
	}
	pool, err := pgxpool.New(ctx, appDSN)
	if err != nil {
		t.Fatalf("open app pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// streamModel emits a single save_script call then stops, mirroring a real
// tool-calling provider (streaming variant — the loop prefers GenerateStream).
type streamModel struct {
	toolCall llmtypes.ToolCall
	usage    *llmtypes.Usage
}

func (m *streamModel) Generate(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (*llmtypes.LLMResponse, error) {
	return nil, fmt.Errorf("unexpected Generate call")
}

func (m *streamModel) GenerateStream(context.Context, []llmtypes.Message, []llmtypes.ToolSpec, ...llmtypes.CallOption) (<-chan llmtypes.StreamEvent, error) {
	ch := make(chan llmtypes.StreamEvent, 8)
	go func() {
		defer close(ch)
		partial := llmtypes.Message{Role: llmtypes.RoleAssistant, Content: []llmtypes.ContentBlock{llmtypes.TextBlock("drafting")}}
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventToolCallStart, Message: partial}
		partial.Content = append(partial.Content, llmtypes.ToolCallBlock(m.toolCall))
		tc := m.toolCall
		ch <- llmtypes.StreamEvent{Type: llmtypes.StreamEventToolCallEnd, Message: partial, CompletedToolCall: &tc}
		ch <- llmtypes.StreamEvent{
			Type:    llmtypes.StreamEventDone,
			Message: llmtypes.Message{Role: llmtypes.RoleAssistant, Content: partial.Content, StopReason: llmtypes.StopReasonToolUse, Usage: m.usage},
		}
	}()
	return ch, nil
}

func (m *streamModel) SupportsTools() bool { return true }

func TestWriterRunOnPostgres(t *testing.T) {
	pool := provision(t)
	q := gen.New(pool)

	args, err := json.Marshal(map[string]any{
		"title": "Smoke script",
		"sections": []map[string]any{
			{"kind": "hook", "title": "Mở", "narration": "Câu mở đầu cho kịch bản thử nghiệm.", "rehook": false, "cta": false, "scenes": []any{}},
			{"kind": "ending_cta", "title": "Kết", "narration": "Lời kết kêu gọi hành động.", "rehook": false, "cta": true, "scenes": []any{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := &streamModel{
		toolCall: llmtypes.ToolCall{ID: "tc-1", Name: "save_script", Args: args},
		usage:    &llmtypes.Usage{Input: 240, Output: 160, TotalTokens: 400, Cost: &llmtypes.Cost{Total: 0.0088}},
	}
	runner := agent.NewRunner(q, func(string) (llmtypes.ChatModel, string, string, error) {
		return model, "openai-codex", "gpt-6-luna", nil
	}, "system", nil)

	ctx := context.Background()
	id, err := runner.Submit(ctx, script.Brief{Topic: "test", Audience: "testers", TargetWords: 300})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runner.Start(runCtx)

	deadline := time.Now().Add(15 * time.Second)
	var row gen.AgentRun
	for {
		row, err = q.GetRun(ctx, id)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if row.Status == "succeeded" || row.Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run không xong sau 15s, status=%q", row.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	runner.Wait()

	if row.Status != "succeeded" {
		t.Fatalf("status = %q, err = %q", row.Status, row.Error.String)
	}
	if row.Provider != "openai-codex" || row.Model != "gpt-6-luna" {
		t.Fatalf("provider/model = %s/%s", row.Provider, row.Model)
	}
	if row.InputTokens != 240 || row.OutputTokens != 160 {
		t.Fatalf("tokens = %d/%d, want > 0 (240/160)", row.InputTokens, row.OutputTokens)
	}
	if !row.CostUsd.Valid || row.CostUsd.Float64 != 0.0088 {
		t.Fatalf("cost_usd = %+v", row.CostUsd)
	}
	if !row.StartedAt.Valid || !row.FinishedAt.Valid {
		t.Fatal("started_at/finished_at phải được ghi")
	}

	scripts, err := q.ListScripts(ctx, 10)
	if err != nil {
		t.Fatalf("list scripts: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("scripts = %d, want 1", len(scripts))
	}
	var sections []script.Section
	if err := json.Unmarshal(scripts[0].Sections, &sections); err != nil {
		t.Fatalf("sections JSON: %v", err)
	}
	if want := script.CountWords(scripts[0].FullText); scripts[0].WordCount != int32(want) {
		t.Fatalf("word_count = %d, want %d", scripts[0].WordCount, want)
	}

	// App role đọc lại danh sách run — chứng minh grant DML đúng.
	runs, err := q.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != id {
		t.Fatalf("list runs = %+v", runs)
	}
}

// TestAppRoleCannotDDL chứng minh role tách quyền: viderse_app chỉ có DML —
// tạo bảng trên pool app phải bị từ chối.
func TestAppRoleCannotDDL(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE TABLE forbidden (id int)"); err == nil {
		t.Fatal("viderse_app không được tạo bảng")
	}
	// DML vẫn đi: insert một run trực tiếp.
	q := gen.New(pool)
	if _, err := q.CreateRun(ctx, gen.CreateRunParams{
		ID: agent.NewUUID(), Provider: "p", Model: "m", Brief: []byte(`{"topic":"t","target_seconds":60}`),
	}); err != nil {
		t.Fatalf("app role không ghi được agent_runs: %v", err)
	}
}

// TestClaimNextRunOrdersAndLocks: claim trả run queued cũ nhất trước, và hai
// claim cạnh tranh (hai tx giữ khóa cùng lúc) không bao giờ lấy cùng một dòng —
// FOR UPDATE SKIP LOCKED đúng ngữ nghĩa hàng đợi.
func TestClaimNextRunOrdersAndLocks(t *testing.T) {
	pool := provision(t)
	q := gen.New(pool)
	ctx := context.Background()

	var want []string
	for range 3 {
		row, err := q.CreateRun(ctx, gen.CreateRunParams{
			ID: agent.NewUUID(), Provider: "p", Model: "m", Brief: []byte(`{"topic":"t","target_seconds":60}`),
		})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}
		want = append(want, agent.UUIDString(row.ID))
		time.Sleep(2 * time.Millisecond) // đảm bảo created_at phân biệt
	}

	// Claim tuần tự: đúng thứ tự cũ nhất trước, dòng đã ở running với started_at.
	for i, expect := range want {
		row, err := q.ClaimNextRun(ctx)
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
		if got := agent.UUIDString(row.ID); got != expect {
			t.Fatalf("claim %d = %s, want oldest %s", i, got, expect)
		}
		if row.Status != "running" || !row.StartedAt.Valid {
			t.Fatalf("claimed run không running/started_at: %+v", row)
		}
	}
	if _, err := q.ClaimNextRun(ctx); err == nil {
		t.Fatal("claim khi hàng rỗng phải báo lỗi (ErrNoRows)")
	}

	// Claim cạnh tranh: tx1 giữ khóa dòng đầu trong một update chưa commit —
	// tx2 claim phải SKIP sang dòng kế, không lấy trùng.
	rowA, err := q.CreateRun(ctx, gen.CreateRunParams{
		ID: agent.NewUUID(), Provider: "p", Model: "m", Brief: []byte(`{"topic":"t","target_seconds":60}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	rowB, err := q.CreateRun(ctx, gen.CreateRunParams{
		ID: agent.NewUUID(), Provider: "p", Model: "m", Brief: []byte(`{"topic":"t","target_seconds":60}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	q1, q2 := gen.New(tx1), gen.New(tx2)

	claimed1, err := q1.ClaimNextRun(ctx)
	if err != nil {
		t.Fatalf("tx1 claim: %v", err)
	}
	if claimed1.ID != rowA.ID {
		t.Fatalf("tx1 claimed %s, want oldest %s", agent.UUIDString(claimed1.ID), agent.UUIDString(rowA.ID))
	}
	claimed2, err := q2.ClaimNextRun(ctx)
	if err != nil {
		t.Fatalf("tx2 claim (SKIP LOCKED): %v", err)
	}
	if claimed2.ID == claimed1.ID {
		t.Fatal("hai claim cạnh tranh lấy cùng một dòng")
	}
	if claimed2.ID != rowB.ID {
		t.Fatalf("tx2 claimed %s, want %s", agent.UUIDString(claimed2.ID), agent.UUIDString(rowB.ID))
	}
}
