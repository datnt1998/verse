package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/datnt1998/noverse/internal/entry/startup"
	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/store"
)

// coCreateManager 持有 web 共创会话：冷启动（故事尚未开始，对话先于开写）与
// 阶段共创（已有故事 → PauseForCoCreate，产出方向经 ResumeFromCoCreate 注入）。
// 回复增量经 SSE `cocreate` 事件下发（{text}，增量文本）。
type coCreateManager struct {
	mu         sync.Mutex
	session    *startup.CoCreateSession
	stage      bool
	inflight   bool
	cancel     context.CancelFunc
	streamPrev string
	committed  bool
	loaded     bool
}

func newCoCreateManager() *coCreateManager { return &coCreateManager{} }

// state 渲染契约 §5 GET /cocreate 的响应体。
func (m *coCreateManager) state() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]any{
		"schemaVersion": 1,
		"active":        m.session != nil && !m.committed,
		"proposal":      (*host.CoCreateProposal)(nil),
		"committed":     m.committed,
		"nextChapter":   nil,
		"messages":      []host.CoCreateMessage{},
		"draft":         "",
		"ready":         false,
		"suggestions":   []string{},
	}
	if m.session == nil {
		return out
	}
	out["messages"] = displayMessages(m.session.History())
	out["ready"] = m.session.Ready()
	out["draft"] = m.session.DraftPrompt()
	out["proposal"] = m.session.Proposal()
	out["nextChapter"] = coCreateNextChapter(m.session.Proposal())
	if sg := m.session.Suggestions(); sg != nil {
		out["suggestions"] = sg
	}
	return out
}

// displayMessages 把会话历史投影为契约 messages：assistant 条目的 Content 会带
// <reply>/<draft>/<ready>/<suggestions> 原文（供模型自参考），展示时剥回纯回复。
// 移植 TUI extractReplyForDisplay。
func displayMessages(history []host.CoCreateMessage) []host.CoCreateMessage {
	out := make([]host.CoCreateMessage, 0, len(history))
	for _, m := range history {
		if m.Role == "assistant" {
			m.Content = extractCoCreateReply(m.Content)
		}
		out = append(out, m)
	}
	return out
}

func extractCoCreateReply(content string) string {
	rest := content
	if rIdx := strings.Index(content, "<reply>"); rIdx >= 0 {
		rest = content[rIdx+len("<reply>"):]
	}
	if cIdx := strings.Index(rest, "</reply>"); cIdx >= 0 {
		return strings.TrimSpace(rest[:cIdx])
	}
	cut := len(rest)
	for _, mark := range []string{"<draft>", "<ready>", "<suggestions>", "<proposal>"} {
		if idx := strings.Index(rest, mark); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	if cut == len(rest) && !strings.Contains(content, "<") {
		return content
	}
	return strings.TrimSpace(rest[:cut])
}

func (s *workerServer) getCoCreate(w http.ResponseWriter) {
	if !s.ensureCoCreateLoaded(w) {
		return
	}
	writeJSON(w, 200, s.co.state())
}

func (s *workerServer) ensureCoCreateLoaded(w http.ResponseWriter) bool {
	m := s.co
	m.mu.Lock()
	err := s.loadCoCreateLocked()
	m.mu.Unlock()
	if err != nil {
		jsonError(w, 500, err.Error())
		return false
	}
	return true
}

const coCreateSessionPath = ".ainovel/cocreate/session.json"

type savedCoCreateState struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Session       *startup.CoCreateSession `json:"session"`
	Stage         bool                     `json:"stage"`
	Committed     bool                     `json:"committed"`
}

// loadCoCreateLocked restores a session once, including the host's stage hold.
func (s *workerServer) loadCoCreateLocked() error {
	m := s.co
	if m.loaded || s.storyDir == "" {
		return nil
	}
	var saved savedCoCreateState
	err := store.NewIO(s.storyDir).ReadJSON(coCreateSessionPath, &saved)
	if os.IsNotExist(err) {
		m.loaded = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("Không thể đọc phiên đồng sáng tác: %w", err)
	}
	if saved.SchemaVersion != 1 {
		return fmt.Errorf("Phiên đồng sáng tác có phiên bản không được hỗ trợ: %d", saved.SchemaVersion)
	}
	if saved.Session != nil && saved.Stage && !saved.Committed && !s.h.PauseForCoCreate() {
		return fmt.Errorf("Không thể khôi phục phiên đồng sáng tác: truyện không còn cho phép tạm dừng")
	}
	m.session, m.stage, m.committed = saved.Session, saved.Stage, saved.Committed
	m.loaded = true
	return nil
}

func (s *workerServer) saveCoCreateLocked() error {
	if s.storyDir == "" {
		return nil
	}
	m := s.co
	if err := store.NewIO(s.storyDir).WriteJSON(coCreateSessionPath, savedCoCreateState{
		SchemaVersion: 1, Session: m.session, Stage: m.stage, Committed: m.committed,
	}); err != nil {
		return fmt.Errorf("Không thể lưu phiên đồng sáng tác: %w", err)
	}
	return nil
}

func coCreateNextChapter(proposal *host.CoCreateProposal) any {
	if proposal == nil || len(proposal.Chapters) == 0 {
		return nil
	}
	return proposal.Chapters[0].Chapter
}

// postMessage 契约 POST /cocreate/message：追加用户消息并同步跑一轮 LLM 回合；
// 增量经 SSE，响应体为完整回合结果。
func (s *workerServer) postCoCreateMessage(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	text := strings.TrimSpace(v.Message)
	if text == "" {
		jsonError(w, 400, "Vui lòng nhập tin nhắn")
		return
	}

	m := s.co
	m.mu.Lock()
	if err := s.loadCoCreateLocked(); err != nil {
		m.mu.Unlock()
		jsonError(w, 500, err.Error())
		return
	}
	if m.inflight {
		m.mu.Unlock()
		jsonError(w, 409, "AI đang trả lời, vui lòng chờ")
		return
	}
	if m.session == nil || m.committed {
		snap := s.h.Snapshot()
		m.stage = !(snap.Phase == "" && snap.BookTitle == "")
		if m.stage && !s.h.PauseForCoCreate() {
			m.mu.Unlock()
			jsonError(w, 409, "Không thể vào đồng sáng tác: truyện đã hoàn thành hoặc đang trong đồng sáng tác")
			return
		}
		m.session = startup.NewCoCreateSession(text)
		m.committed = false
	} else {
		m.session.AppendUser(text)
	}
	if err := s.saveCoCreateLocked(); err != nil {
		m.mu.Unlock()
		jsonError(w, 500, err.Error())
		return
	}
	history := m.session.History()
	stage := m.stage
	ctx, cancel := context.WithCancel(r.Context())
	m.inflight, m.cancel, m.streamPrev = true, cancel, ""
	m.mu.Unlock()

	stream := s.h.CoCreateStream
	if stage {
		stream = s.h.StageCoCreateStream
	}
	reply, err := stream(ctx, history, func(kind, text string) {
		if kind != host.CoCreateProgressReply {
			return // thinking 不进契约 {text} 增量流
		}
		m.mu.Lock()
		prev := m.streamPrev
		if strings.HasPrefix(text, prev) && len(text) > len(prev) {
			delta := text[len(prev):]
			m.streamPrev = text
			m.mu.Unlock()
			s.b.emit(message{kind: "cocreate", data: map[string]string{"text": delta}})
			return
		}
		if prev == "" && text != "" {
			m.streamPrev = text
			m.mu.Unlock()
			s.b.emit(message{kind: "cocreate", data: map[string]string{"text": text}})
			return
		}
		m.streamPrev = text
		m.mu.Unlock()
	})

	m.mu.Lock()
	m.inflight, m.cancel = false, nil
	if err != nil {
		if m.stage && m.session != nil {
			// 保留会话让用户可换句重试；PauseForCoCreate 的暂停态继续持有。
		}
		m.mu.Unlock()
		if ctx.Err() != nil {
			jsonError(w, 409, "Đồng sáng tác đã bị huỷ")
			return
		}
		jsonError(w, 500, err.Error())
		return
	}
	if m.session != nil {
		m.session.ApplyReply(reply)
	}
	draft, ready := "", false
	var suggestions []string
	if m.session != nil {
		draft, ready = m.session.DraftPrompt(), m.session.Ready()
		suggestions = m.session.Suggestions()
	}
	var proposal *host.CoCreateProposal
	if m.session != nil {
		proposal = m.session.Proposal()
		if err := s.saveCoCreateLocked(); err != nil {
			m.mu.Unlock()
			jsonError(w, 500, err.Error())
			return
		}
	}
	m.mu.Unlock()
	if suggestions == nil {
		suggestions = []string{}
	}
	writeJSON(w, 200, map[string]any{
		"schemaVersion": 1, "proposal": proposal, "committed": false, "nextChapter": coCreateNextChapter(proposal),
		"reply": reply.Message, "draft": draft, "ready": ready, "suggestions": suggestions,
	})
}

// postCoCreateCommit 契约 POST /cocreate/commit：冷启动 → 用草稿开写；阶段共创 → 注入并续写。
func (s *workerServer) postCoCreateCommit(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Draft string `json:"draft"`
	}
	if !decodeJSON(w, r, &v) {
		return
	}
	m := s.co
	m.mu.Lock()
	if err := s.loadCoCreateLocked(); err != nil {
		m.mu.Unlock()
		jsonError(w, 500, err.Error())
		return
	}
	if m.session == nil || m.committed {
		m.mu.Unlock()
		jsonError(w, 409, "Chưa có phiên đồng sáng tác nào đang mở")
		return
	}
	if m.inflight {
		m.mu.Unlock()
		jsonError(w, 409, "AI đang trả lời, vui lòng chờ")
		return
	}
	draft := strings.TrimSpace(v.Draft)
	if draft == "" {
		draft = m.session.DraftPrompt()
	}
	stage := m.stage
	m.inflight = true
	m.mu.Unlock()
	release := func() {
		m.mu.Lock()
		m.inflight = false
		m.mu.Unlock()
	}
	if draft == "" {
		release()
		jsonError(w, 400, "Chưa có bản nháp chỉ dẫn sáng tác, gửi thêm tin nhắn để AI tổng hợp")
		return
	}

	action := "start"
	if stage {
		action = "resume"
	}
	pending, err := s.dispatchAction("Đang chốt hướng đi · Kiến trúc sư đang xử lý", func() error {
		defer release()
		if stage {
			if err := s.h.ResumeFromCoCreate(draft); err != nil {
				return err
			}
		} else {
			if err := s.h.PrepareUserRules(draft); err != nil {
				return err
			}
			if err := s.h.StartPrepared(draft); err != nil {
				return err
			}
		}
		return s.markCoCreateCommitted()
	})
	if err == errActionBusy {
		release()
	}
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	m.mu.Lock()
	proposal := m.session.Proposal()
	committed := m.committed
	m.mu.Unlock()
	status := 200
	if pending {
		status = 202
	}
	writeJSON(w, status, map[string]any{
		"schemaVersion": 1, "ok": true, "action": action, "pending": pending,
		"proposal": proposal, "committed": committed, "nextChapter": coCreateNextChapter(proposal),
	})
}

func (s *workerServer) markCoCreateCommitted() error {
	m := s.co
	m.mu.Lock()
	defer m.mu.Unlock()
	m.committed = true
	return s.saveCoCreateLocked()
}

// postCoCreateCancel 契约 POST /cocreate/cancel：取消在途回合、退出阶段共创暂停态、清会话。
func (s *workerServer) postCoCreateCancel(w http.ResponseWriter, r *http.Request) {
	m := s.co
	m.mu.Lock()
	if m.inflight && m.cancel == nil {
		m.mu.Unlock()
		jsonError(w, 409, "Đang chốt hướng đi, vui lòng chờ")
		return
	}
	if err := s.loadCoCreateLocked(); err != nil {
		m.mu.Unlock()
		jsonError(w, 500, err.Error())
		return
	}
	if s.storyDir != "" {
		if err := os.Remove(filepath.Join(s.storyDir, coCreateSessionPath)); err != nil && !os.IsNotExist(err) {
			m.mu.Unlock()
			jsonError(w, 500, fmt.Sprintf("Không thể xoá phiên đồng sáng tác: %v", err))
			return
		}
	}
	if m.cancel != nil {
		m.cancel()
	}
	stage := m.stage && !m.committed
	m.mu.Unlock()
	if stage {
		s.h.CancelCoCreate()
	}
	m.clear()
	writeJSON(w, 200, map[string]any{"schemaVersion": 1, "ok": true})
}

func (m *coCreateManager) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session, m.stage, m.streamPrev, m.committed = nil, false, "", false
}
