// Package web là HTTP gateway của viderse serve: SPA nhúng và các route
// /api nghiệp vụ (runs, scripts) gắn trên platform/webapp.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/agent"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/script"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:embed all:dist
var ui embed.FS

// StoreView là phần API phía runs/scripts mà gateway cần (gen.Queries thoả).
type StoreView interface {
	GetRun(ctx context.Context, id pgtype.UUID) (gen.AgentRun, error)
	ListRuns(ctx context.Context, limit int32) ([]gen.AgentRun, error)
	GetScript(ctx context.Context, id pgtype.UUID) (gen.Script, error)
	ListScripts(ctx context.Context, limit int32) ([]gen.Script, error)
}

// Submitter là cổng đưa brief vào hàng chạy (agent.Runner thoả).
type Submitter interface {
	Submit(ctx context.Context, brief script.Brief) (pgtype.UUID, error)
}

// Gateway phục vụ toàn bộ HTTP của app.
type Gateway struct {
	*webapp.Server
	queries StoreView
	runner  Submitter
}

// NewGateway dựng handler: accounts là kho identity file-backend; khi chưa có
// tài khoản nào, server tạo một setup state (chỉ giữ hash) — serve in mã
// plaintext một lần qua SetupCodeOnce.
func NewGateway(accounts *identity.Store, queries StoreView, runner Submitter) *Gateway {
	dist, err := fs.Sub(ui, "dist")
	if err != nil {
		// embed đảm bảo dist tồn tại khi build; lỗi ở đây là binary hỏng.
		panic(err)
	}
	g := &Gateway{
		Server: webapp.New(accounts, webapp.Options{
			AppName:    "Viderse",
			CookieName: "viderse_session",
			UI:         dist,
		}),
		queries: queries,
		runner:  runner,
	}
	g.Handle(http.MethodGet, "/api/runs", g.listRuns)
	g.Handle(http.MethodPost, "/api/runs", g.createRun)
	g.Handle(http.MethodGet, "/api/runs/", g.getRun)
	g.Handle(http.MethodGet, "/api/scripts", g.listScripts)
	g.Handle(http.MethodGet, "/api/scripts/", g.getScript)
	return g
}

// ── runs / scripts ──────────────────────────────────────────────────────────

func (g *Gateway) createRun(w http.ResponseWriter, r *http.Request, _ identity.User) {
	var brief script.Brief
	if !webapp.DecodeJSON(w, r, &brief) {
		return
	}
	id, err := g.runner.Submit(r.Context(), brief)
	if err != nil {
		webapp.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	run, err := g.queries.GetRun(r.Context(), id)
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	webapp.WriteJSON(w, http.StatusAccepted, map[string]any{"run": runWire(run)})
}

func (g *Gateway) listRuns(w http.ResponseWriter, r *http.Request, _ identity.User) {
	rows, err := g.queries.ListRuns(r.Context(), 100)
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, runWire(row))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (g *Gateway) getRun(w http.ResponseWriter, r *http.Request, _ identity.User) {
	id, err := agent.ParseUUID(strings.TrimPrefix(r.URL.Path, "/api/runs/"))
	if err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	row, err := g.queries.GetRun(r.Context(), id)
	if err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"run": runWire(row)})
}

func (g *Gateway) listScripts(w http.ResponseWriter, r *http.Request, _ identity.User) {
	rows, err := g.queries.ListScripts(r.Context(), 100)
	if err != nil {
		webapp.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, scriptSummaryWire(row))
	}
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"scripts": out})
}

func (g *Gateway) getScript(w http.ResponseWriter, r *http.Request, _ identity.User) {
	id, err := agent.ParseUUID(strings.TrimPrefix(r.URL.Path, "/api/scripts/"))
	if err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	row, err := g.queries.GetScript(r.Context(), id)
	if err != nil {
		webapp.Error(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	var sections []script.Section
	if err := json.Unmarshal(row.Sections, &sections); err != nil {
		webapp.Error(w, http.StatusInternalServerError, "sections hỏng")
		return
	}
	wire := scriptSummaryWire(row)
	wire["sections"] = sections
	webapp.WriteJSON(w, http.StatusOK, map[string]any{"script": wire})
}

func runWire(row gen.AgentRun) map[string]any {
	out := map[string]any{
		"id":            agent.UUIDString(row.ID),
		"status":        row.Status,
		"provider":      row.Provider,
		"model":         row.Model,
		"input_tokens":  row.InputTokens,
		"output_tokens": row.OutputTokens,
		"created_at":    row.CreatedAt.Time,
	}
	var brief script.Brief
	if err := json.Unmarshal(row.Brief, &brief); err == nil {
		out["brief"] = brief
	}
	if row.CostUsd.Valid {
		out["cost_usd"] = row.CostUsd.Float64
	}
	if row.Error.Valid {
		out["error"] = row.Error.String
	}
	if row.StartedAt.Valid {
		out["started_at"] = row.StartedAt.Time
	}
	if row.FinishedAt.Valid {
		out["finished_at"] = row.FinishedAt.Time
	}
	return out
}

func scriptSummaryWire(row gen.Script) map[string]any {
	return map[string]any{
		"id":               agent.UUIDString(row.ID),
		"run_id":           agent.UUIDString(row.RunID),
		"title":            row.Title,
		"full_text":        row.FullText,
		"word_count":       row.WordCount,
		"est_duration_sec": row.EstDurationSec,
		"created_at":       row.CreatedAt.Time,
	}
}
