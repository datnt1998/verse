// Package agent chạy writer agent của Viderse: nhận brief, chạy agentcore
// loop với model của role "writer", kết thúc bằng tool save_script và ghi một
// dòng agent_runs cho mỗi lần chạy (trạng thái, token, chi phí, thời điểm,
// lỗi).
//
// Database chính là hàng đợi: Submit chỉ ghi một dòng queued và báo hiệu cho
// dispatcher; dispatcher claim run cũ nhất bằng UPDATE … FOR UPDATE SKIP
// LOCKED nên run ghi trước khi serve (kể cả do tiến trình trước để lại) vẫn
// được chạy, và tắt máy giữa chừng không làm run kẹt ở queued. Hàng đợi giả
// định một dispatcher cho một database — serve là tiến trình duy nhất chạy
// writer; Start đánh failed mọi dòng còn 'running' (thuộc tiến trình đã chết).
package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	llmtypes "github.com/datnt1998/verse/core/llm"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/datnt1998/verse/core/agentcore"
)

// interruptedMessage là lỗi ghi cho run còn 'running' khi serve mới khởi động —
// tiến trình chủ của nó đã chết trước khi ghi kết quả.
const interruptedMessage = "run bị gián đoạn: tiến trình serve trước đã dừng"

// RunStore là phần DB runner cần: tạo/đọc/cập nhật agent_runs và lưu scripts.
// gen.Queries thoả interface này; test có thể thay bằng implementation khác.
type RunStore interface {
	CreateRun(ctx context.Context, arg gen.CreateRunParams) (gen.AgentRun, error)
	ClaimNextRun(ctx context.Context) (gen.AgentRun, error)
	FailInterruptedRuns(ctx context.Context, errText pgtype.Text) (int64, error)
	FinishRun(ctx context.Context, arg gen.FinishRunParams) (gen.AgentRun, error)
	CreateScript(ctx context.Context, arg gen.CreateScriptParams) (gen.Script, error)
}

// ModelFactory dựng ChatModel cho một role, trả kèm provider/model đã resolve.
// Gọi lúc enqueue (ghi vào agent_runs) và lúc bắt đầu chạy — resolve hai lần
// cùng một cấu hình nên kết quả nhất quán; chênh lệch do người dùng vừa đổi
// config thì bản mới nhất được dùng để chạy.
type ModelFactory func(role string) (llmtypes.ChatModel, string, string, error)

// Runner điều phối hàng đợi lần chạy trong tiến trình serve. Tuần tự, một
// goroutine: hàng đợi nằm trong agent_runs (xem doc của package), hủy theo ctx
// của serve.
type Runner struct {
	store    RunStore
	factory  ModelFactory
	prompt   string
	thinking func(role string) string

	dispatch chan struct{}
	once     sync.Once
	wg       sync.WaitGroup
}

// NewRunner tạo dispatcher. prompt là system prompt của writer (asset nhúng);
// thinking trả mức reasoning hiệu lực theo role (có thể nil).
func NewRunner(store RunStore, factory ModelFactory, prompt string, thinking func(role string) string) *Runner {
	return &Runner{
		store:    store,
		factory:  factory,
		prompt:   prompt,
		thinking: thinking,
		dispatch: make(chan struct{}, 1),
	}
}

// Start khởi động goroutine dispatch; ctx của serve quyết vòng đời runner.
// Trước khi vào vòng lặp, đánh failed mọi run còn 'running' của tiến trình
// trước — deployment giả định một dispatcher mỗi database.
func (r *Runner) Start(ctx context.Context) {
	r.once.Do(func() {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			if n, err := r.store.FailInterruptedRuns(ctx, pgtype.Text{String: interruptedMessage, Valid: true}); err != nil {
				slog.Error("không dọn run gián đoạn được", "module", "agent", "err", err)
			} else if n > 0 {
				slog.Info("đánh failed run bị gián đoạn từ tiến trình trước", "module", "agent", "count", n)
			}
			r.dispatchLoop(ctx)
		}()
	})
}

// Wait chờ dispatcher dừng hẳn (sau khi ctx huỷ).
func (r *Runner) Wait() { r.wg.Wait() }

// Submit ghi một dòng agent_runs queued (hàng đợi là DB) và báo hiệu cho
// dispatcher. RunID trả về để caller/API theo dõi qua polling.
func (r *Runner) Submit(ctx context.Context, brief script.Brief) (pgtype.UUID, error) {
	if err := brief.Validate(); err != nil {
		return pgtype.UUID{}, err
	}
	// provider/model ghi lúc enqueue từ cùng một resolve của run — không build
	// model ở đây (build chậm/lỗi credential là chuyện của run).
	_, provider, model, err := r.factory(bootstrap.WriterRole)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("resolve vai %s: %w", bootstrap.WriterRole, err)
	}
	briefJSON, err := json.Marshal(brief)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("mã hoá brief: %w", err)
	}
	id := NewUUID()
	if _, err := r.store.CreateRun(ctx, gen.CreateRunParams{
		ID:       id,
		Provider: provider,
		Model:    model,
		Brief:    briefJSON,
	}); err != nil {
		return pgtype.UUID{}, fmt.Errorf("tạo agent_runs: %w", err)
	}
	r.signal()
	return id, nil
}

func (r *Runner) signal() {
	select {
	case r.dispatch <- struct{}{}:
	default:
	}
}

func (r *Runner) dispatchLoop(ctx context.Context) {
	// Ngay khi vào vòng: hút hết run đang chờ (kể cả của tiến trình trước).
	r.drain(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.dispatch:
			r.drain(ctx)
		}
	}
}

// drain claim và chạy tuần tự mọi run đang chờ cho đến khi hàng rỗng. Lỗi
// claim khác ErrNoRows: log rồi chờ tín hiệu kế — không busy loop.
func (r *Runner) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		row, err := r.store.ClaimNextRun(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		if err != nil {
			slog.Error("claim run lỗi", "module", "agent", "err", err)
			return
		}
		r.runOnce(ctx, row)
	}
}

// runOnce điều khiển vòng đời một run đã claim (đã ở running): agent loop →
// succeeded/failed với số đo usage.
func (r *Runner) runOnce(ctx context.Context, row gen.AgentRun) {
	id := row.ID
	var brief script.Brief
	if err := json.Unmarshal(row.Brief, &brief); err != nil {
		r.finish(ctx, id, "failed", llmtypes.Usage{}, fmt.Sprintf("brief không hợp lệ: %v", err))
		return
	}

	model, _, _, err := r.factory(bootstrap.WriterRole)
	if err != nil {
		r.finish(ctx, id, "failed", llmtypes.Usage{}, fmt.Sprintf("dựng model: %v", err))
		return
	}

	saved := &saveOutcome{}
	tool := &saveScriptTool{store: r.store, runID: id, brief: brief, out: saved}

	cfg := agentcore.LoopConfig{
		Model:      model,
		MaxTurns:   24,
		MaxRetries: 2,
		StopAfterToolResult: func(toolName string, result json.RawMessage) bool {
			// Chỉ dừng khi save_script THÀNH CÔNG; lỗi validation trả về cho
			// model để nó tự sửa và gọi lại.
			return toolName == saveScriptToolName && saved.done
		},
	}
	if r.thinking != nil {
		if lvl, perr := llmtypes.ParseThinkingLevel(r.thinking(bootstrap.WriterRole)); perr == nil {
			cfg.ThinkingLevel = lvl
		}
	}

	agentCtx := agentcore.AgentContext{
		SystemPrompt: r.prompt,
		Tools:        []agentcore.Tool{tool},
	}
	events := agentcore.AgentLoop(ctx, []llmtypes.AgentMessage{llmtypes.UserMsg(buildBriefMessage(brief))}, agentCtx, cfg)

	var summary *agentcore.RunSummary
	var runErr error
	for ev := range events {
		switch ev.Type {
		case agentcore.EventError:
			if ev.Err != nil {
				runErr = ev.Err
			}
		case agentcore.EventAgentEnd:
			summary = ev.Summary
			if ev.Err != nil {
				runErr = ev.Err
			}
		}
	}

	var usage llmtypes.Usage
	endReason := agentcore.EndReasonError
	if summary != nil {
		usage = summary.Usage
		endReason = summary.EndReason
	}

	switch {
	case saved.done:
		r.finish(ctx, id, "succeeded", usage, "")
	case ctx.Err() != nil:
		// serve đang tắt: ctx đã huỷ nên FinishRun phải dùng nền riêng.
		r.finish(context.Background(), id, "failed", usage, "máy chủ dừng giữa chừng (run bị huỷ)")
	case runErr != nil:
		r.finish(ctx, id, "failed", usage, runErr.Error())
	case endReason == agentcore.EndReasonMaxTurns:
		r.finish(ctx, id, "failed", usage, "hết lượt mà chưa gọi save_script")
	default:
		r.finish(ctx, id, "failed", usage, "agent dừng mà chưa gọi save_script")
	}
}

func (r *Runner) finish(ctx context.Context, id pgtype.UUID, status string, usage llmtypes.Usage, errText string) {
	arg := gen.FinishRunParams{
		ID:           id,
		Status:       status,
		InputTokens:  int32(usage.Input),
		OutputTokens: int32(usage.Output),
		Error:        pgtype.Text{String: errText, Valid: errText != ""},
	}
	if usage.Cost != nil {
		arg.CostUsd = pgtype.Float8{Float64: usage.Cost.Total, Valid: true}
	}
	if _, err := r.store.FinishRun(ctx, arg); err != nil {
		slog.Error("không ghi kết quả run", "module", "agent", "run", UUIDString(id), "err", err)
	}
}

// ── save_script tool ────────────────────────────────────────────────────────

const saveScriptToolName = "save_script"

// saveScriptArgs là payload model gửi cho save_script; field names khớp
// contract section/scene.
type saveScriptArgs struct {
	Title    string           `json:"title"`
	Sections []script.Section `json:"sections"`
}

type saveOutcome struct {
	done     bool
	scriptID string
}

// saveScriptTool kết thúc run: validate contract, đo, ghi scripts rồi trả
// script_id — StopAfterToolResult dừng loop ngay sau kết quả thành công.
type saveScriptTool struct {
	store RunStore
	runID pgtype.UUID
	brief script.Brief
	out   *saveOutcome
}

func (t *saveScriptTool) Name() string  { return saveScriptToolName }
func (t *saveScriptTool) Label() string { return "Lưu kịch bản" }

func (t *saveScriptTool) Description() string {
	return "Lưu kịch bản video hoàn chỉnh. Gọi đúng một lần khi kịch bản đã xong: " +
		"mỗi phần có kind (chữ thường, vd hook/part/ending_cta), title, narration là lời đọc; " +
		"nếu phần chia cảnh thì lời đọc nằm trong scenes[].narration và narration chỉ là tóm tắt. " +
		"rehook=true cho phần mở lại sự chú ý, cta=true cho phần kêu gọi hành động."
}

func (t *saveScriptTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Tiêu đề video gợi ý",
			},
			"sections": map[string]any{
				"type":        "array",
				"minItems":    1,
				"maxItems":    script.MaxSections,
				"description": "Các phần của kịch bản theo thứ tự đọc",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"kind":      map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$"},
						"title":     map[string]any{"type": "string"},
						"narration": map[string]any{"type": "string"},
						"rehook":    map[string]any{"type": "boolean"},
						"cta":       map[string]any{"type": "boolean"},
						"scenes": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"narration":   map[string]any{"type": "string"},
									"visual_note": map[string]any{"type": "string"},
								},
								"required": []string{"narration", "visual_note"},
							},
						},
					},
					"required": []string{"kind", "title", "narration", "rehook", "cta", "scenes"},
				},
			},
		},
		"required": []string{"title", "sections"},
	}
}

// StrictSchema báo adapter dùng strict tool calling khi provider hỗ trợ.
func (t *saveScriptTool) StrictSchema() bool { return true }

func (t *saveScriptTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var in saveScriptArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("tham số save_script không phải JSON hợp lệ: %w", err)
	}
	// JSON cho phép bỏ trống scenes; chuẩn hoá về mảng rỗng để khớp contract.
	for i := range in.Sections {
		if in.Sections[i].Scenes == nil {
			in.Sections[i].Scenes = []script.Scene{}
		}
	}
	if err := script.Validate(in.Sections); err != nil {
		return nil, err
	}
	measure := script.MeasureScript(in.Sections, float64(t.brief.WPM()))
	sectionsJSON, err := json.Marshal(in.Sections)
	if err != nil {
		return nil, fmt.Errorf("mã hoá sections: %w", err)
	}
	title := strings.TrimSpace(in.Title)
	row, err := t.store.CreateScript(ctx, gen.CreateScriptParams{
		ID:             NewUUID(),
		RunID:          t.runID,
		Title:          title,
		Sections:       sectionsJSON,
		FullText:       measure.FullText,
		WordCount:      int32(measure.WordCount),
		EstDurationSec: int32(measure.EstDurationSec + 0.5),
	})
	if err != nil {
		return nil, fmt.Errorf("ghi scripts: %w", err)
	}
	t.out.done = true
	t.out.scriptID = UUIDString(row.ID)
	return json.Marshal(map[string]any{
		"script_id":        t.out.scriptID,
		"word_count":       measure.WordCount,
		"est_duration_sec": int(measure.EstDurationSec + 0.5),
	})
}

// ── tiện ích ────────────────────────────────────────────────────────────────

// NewUUID tạo uuid v4 dưới dạng pgtype.UUID (id sinh phía app, không cần
// extension pgcrypto trên database).
func NewUUID() pgtype.UUID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return pgtype.UUID{Bytes: b, Valid: true}
}

// UUIDString render pgtype.UUID dạng chuỗi chuẩn cho API/log.
func UUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

// ParseUUID đọc uuid từ chuỗi API (hex với hoặc không gạch nối).
func ParseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(strings.TrimSpace(s)); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

// buildBriefMessage dựng user message từ brief: chủ đề, khán giả, độ dài mục
// tiêu và nhắc kết thúc bằng save_script.
func buildBriefMessage(brief script.Brief) string {
	var b strings.Builder
	b.WriteString("Viết kịch bản video từ brief sau.\n\n")
	fmt.Fprintf(&b, "Chủ đề: %s\n", brief.Topic)
	if strings.TrimSpace(brief.Audience) != "" {
		fmt.Fprintf(&b, "Khán giả: %s\n", brief.Audience)
	}
	fmt.Fprintf(&b, "Độ dài mục tiêu: %s\n", brief.TargetDescription())
	fmt.Fprintf(&b, "Tốc độ đọc giả định: %d từ/phút.\n", brief.WPM())
	b.WriteString("\nKhi kịch bản đã hoàn chỉnh, gọi tool save_script đúng một lần để lưu và kết thúc.")
	return b.String()
}

// OpenPool mở pgxpool từ DSN (dùng chung cho serve/test).
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("mở pool: %w", err)
	}
	return pool, nil
}
