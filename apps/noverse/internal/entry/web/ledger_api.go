package web

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

type ledgerCandidate struct {
	Story  string         `json:"story"`
	Title  string         `json:"title"`
	Brief  string         `json:"brief"`
	Values ledger.Values  `json:"values"`
	Checks []ledger.Check `json:"checks"`
	Cells  []ledger.Cell  `json:"cells"`
}

func (g *Gateway) ledgerStories(ctx context.Context, lib library.Repository, user identity.User, p, exclude string) ([]ledger.Story, *ledgerCandidate, error) {
	stories, err := viewableProjectStories(ctx, lib, user, p)
	if err != nil {
		return nil, nil, err
	}
	slices.Reverse(stories)
	rows := make([]ledger.Story, 0, len(stories))
	var candidate *ledgerCandidate
	for _, st := range stories {
		// The ledger is a store document (meta/ledger.json): file mode reads
		// the engine directory, hosted reads the scoped story_state row —
		// never a path derived from st.Dir.
		engine, err := g.openFactStore(st, &user)
		if err != nil {
			return nil, nil, err
		}
		rec, err := ledger.LoadFrom(engine.Docs())
		if err != nil {
			return nil, nil, err
		}
		row := ledger.Story{ID: st.ID, Title: g.storyDisplayTitle(ctx, lib, user, st)}
		if sf, err := lib.LoadStoryFile(ctx, actorOf(user), st); err == nil && sf != nil {
			row.CreatedAt = sf.CreatedAt
		}
		if rec != nil {
			row.Values = rec.Values
			row.Opening = rec.Values[8]
			outline, err := engine.Outline.LoadOutline()
			if err != nil {
				return nil, nil, err
			}
			if len(outline) == 0 && rec.Brief != "" && candidate == nil {
				candidate = &ledgerCandidate{st.ID, row.Title, rec.Brief, rec.Values, rec.Result.Checks, rec.Result.Cells}
			}
		}
		if st.ID != exclude {
			rows = append(rows, row)
		}
	}
	return rows, candidate, nil
}
func ledgerArchitectModel(dir string) (llmtypes.ChatModel, error) {
	cfg, err := bootstrap.LoadConfigAt(dir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return nil, errRulesNoModel
	}
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		return nil, err
	}
	return models.ForRole("architect"), nil
}
func (g *Gateway) analyzeLedger(ctx context.Context, lib library.Repository, user identity.User, p, brief, exclude string, suggest bool) (ledger.Proposal, ledger.Result, error) {
	prior, _, err := g.ledgerStories(ctx, lib, user, p, exclude)
	if err != nil {
		return ledger.Proposal{}, ledger.Result{}, err
	}
	ref, _ := lib.ResolveProject(ctx, actorOf(user), p)
	dir := ref.Dir
	model, err := ledgerArchitectModel(dir)
	if err != nil {
		return ledger.Proposal{}, ledger.Result{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	proposal, err := ledger.Analyze(callCtx, model, brief, prior, suggest)
	if err != nil {
		return ledger.Proposal{}, ledger.Result{}, err
	}
	return proposal, ledger.CheckValues(proposal.Values, prior), nil
}
func (g *Gateway) serveLedger(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p string, segs []string) {
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	if len(segs) == 0 && r.Method == http.MethodGet {
		stories, candidate, err := g.ledgerStories(r.Context(), lib, user, p, "")
		if err != nil {
			jsonError(w, 500, "Không đọc được sổ cái truyện")
			return
		}
		writeJSON(w, 200, projectLedgerResponse{Axes: ledger.Axes, Stories: stories, Candidate: candidate})
		return
	}
	if len(segs) != 1 || r.Method != http.MethodPost || (segs[0] != "check" && segs[0] != "suggest") {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	if !user.CanManageProject(p) {
		jsonError(w, 403, "Không có quyền")
		return
	}
	var req struct {
		Brief string `json:"brief"`
		Story string `json:"story"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Brief = strings.TrimSpace(req.Brief)
	if req.Brief == "" {
		jsonError(w, 400, "Hãy nhập yêu cầu truyện")
		return
	}
	exclude := req.Story
	if exclude != "" && !user.CanRead(exclude) {
		jsonError(w, 404, "Không tìm thấy truyện")
		return
	}
	if exclude == "" {
		_, candidate, err := g.ledgerStories(r.Context(), lib, user, p, "")
		if err != nil {
			jsonError(w, 500, "Không đọc được sổ cái truyện")
			return
		}
		if candidate != nil && candidate.Brief == req.Brief {
			exclude = candidate.Story
		}
	}
	proposal, result, err := g.analyzeLedger(r.Context(), lib, user, p, req.Brief, exclude, segs[0] == "suggest")
	if err != nil {
		ledgerHTTPError(w, err)
		return
	}
	if segs[0] == "suggest" {
		writeJSON(w, 200, proposal)
	} else {
		writeJSON(w, 200, result)
	}
}
func ledgerHTTPError(w http.ResponseWriter, err error) {
	if err == errRulesNoModel {
		writeRulesNoModel(w)
		return
	}
	jsonError(w, http.StatusBadGateway, "Không đọc được mười trục từ yêu cầu truyện")
}
func (g *Gateway) ledgerStartGate(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, p, brief, exclude string) bool {
	prior, _, err := g.ledgerStories(r.Context(), lib, user, p, exclude)
	if err != nil {
		jsonError(w, 500, "Không đọc được sổ cái truyện")
		return false
	}
	hasPrior := false
	for _, s := range prior {
		if ledger.Validate(s.Values) == nil {
			hasPrior = true
			break
		}
	}
	if !hasPrior {
		return true
	}
	_, result, err := g.analyzeLedger(r.Context(), lib, user, p, brief, exclude, false)
	if err != nil {
		ledgerHTTPError(w, err)
		return false
	}
	if result.Blocking() {
		writeJSON(w, 409, map[string]any{"error": "Truyện này giống một truyện bạn đã đăng", "checks": result.Checks, "axes": result.Axes, "values": result.Values, "cells": result.Cells})
		return false
	}
	return true
}

func (g *Gateway) checkInitialLedgerInput(w http.ResponseWriter, r *http.Request, lib library.Repository, user identity.User, st library.Story) bool {
	engine, err := g.openFactStore(st, &user)
	if err != nil {
		jsonError(w, 500, "Không mở được kho dữ kiện truyện")
		return false
	}
	state, err := engine.Progress.Load()
	if err != nil {
		jsonError(w, 500, "Không đọc được tiến độ truyện")
		return false
	}
	book, err := engine.Book.Load()
	if err != nil {
		jsonError(w, 500, "Không đọc được thông tin truyện")
		return false
	}
	if state != nil && state.Phase != "" || book != nil && book.Title != "" {
		return true
	}
	var req struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &req) {
		return false
	}
	if strings.TrimSpace(req.Text) == "" {
		jsonError(w, 400, "Nội dung không được để trống")
		return false
	}
	if !g.ledgerStartGate(w, r, lib, user, st.Project, req.Text, st.ID) {
		return false
	}
	body, _ := json.Marshal(req)
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return true
}
