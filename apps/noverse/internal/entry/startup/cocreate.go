package startup

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

// CoCreateSession 承载共创模式的非 UI 状态。
type CoCreateSession struct {
	history        []host.CoCreateMessage
	draftPrompt    string
	ready          bool
	streamReply    string
	streamThinking string
	suggestions    []string
	proposal       *host.CoCreateProposal
}

type coCreateSessionData struct {
	SchemaVersion int                    `json:"schemaVersion"`
	History       []host.CoCreateMessage `json:"history"`
	Draft         string                 `json:"draft"`
	Ready         bool                   `json:"ready"`
	Suggestions   []string               `json:"suggestions"`
	Proposal      *host.CoCreateProposal `json:"proposal"`
}

func (s *CoCreateSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(coCreateSessionData{
		SchemaVersion: 1, History: s.history, Draft: s.draftPrompt,
		Ready: s.ready, Suggestions: s.suggestions, Proposal: s.proposal,
	})
}

func (s *CoCreateSession) UnmarshalJSON(data []byte) error {
	var saved coCreateSessionData
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	if saved.SchemaVersion != 1 {
		return fmt.Errorf("Phiên đồng sáng tác có phiên bản không được hỗ trợ: %d", saved.SchemaVersion)
	}
	*s = CoCreateSession{
		history: saved.History, draftPrompt: saved.Draft, ready: saved.Ready,
		suggestions: saved.Suggestions, proposal: saved.Proposal,
	}
	return nil
}

func NewCoCreateSession(initial string) *CoCreateSession {
	return &CoCreateSession{
		history: []host.CoCreateMessage{
			{Role: "user", Content: strings.TrimSpace(initial)},
		},
	}
}

func (s *CoCreateSession) History() []host.CoCreateMessage {
	if s == nil {
		return nil
	}
	return append([]host.CoCreateMessage(nil), s.history...)
}

func (s *CoCreateSession) ApplyReply(reply host.CoCreateReply) {
	if s == nil {
		return
	}
	s.streamReply = ""
	s.streamThinking = ""
	// Keep the full model XML, including draft and proposal, so the next turn
	// revises the real previous plan rather than rebuilding it from reply text.
	text := strings.TrimSpace(reply.Raw)
	if text == "" {
		text = strings.TrimSpace(reply.Message)
	}
	if text != "" {
		s.history = append(s.history, host.CoCreateMessage{Role: "assistant", Content: text})
	}
	// 仅当 Prompt 非空才覆盖 draft：parse 降级路径会返回 Prompt=""，此时
	// 必须保留上一轮 draft，否则用户已积累的"当前创作指令"会被截断的回复清空。
	if prompt := strings.TrimSpace(reply.Prompt); prompt != "" {
		s.draftPrompt = prompt
	}
	s.ready = reply.Ready
	// suggestions 直接覆盖（包括覆盖为空）：每轮的引导只对当下有意义。
	s.suggestions = append(s.suggestions[:0], reply.Suggestions...)
	if reply.Proposal != nil {
		proposal := *reply.Proposal
		proposal.Revision = 1
		if s.proposal != nil {
			proposal.Revision = s.proposal.Revision + 1
		}
		proposal.Chapters = append([]host.CoCreateProposalChapter(nil), proposal.Chapters...)
		proposal.Impacts = append([]string(nil), proposal.Impacts...)
		s.proposal = &proposal
	}
}

func (s *CoCreateSession) AppendUser(text string) {
	if s == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// 用户已经决定下一句要说什么，suggestions 立即作废，避免 AI 还没回复时
	// 旧建议挂在输入框上误导。
	s.suggestions = nil
	s.history = append(s.history, host.CoCreateMessage{Role: "user", Content: text})
}

// ApplyDelta 接收流式累积；kind="thinking" 写入推理流，"reply" 写入回复预览。
// 两路分别累积，UI 可分块染色显示，让用户在 thinking 阶段也看到 LLM 在工作。
func (s *CoCreateSession) ApplyDelta(kind, text string) {
	if s == nil {
		return
	}
	text = strings.TrimSpace(text)
	switch kind {
	case host.CoCreateProgressThinking:
		s.streamThinking = text
	case host.CoCreateProgressReply:
		s.streamReply = text
	}
}

func (s *CoCreateSession) StreamReply() string {
	if s == nil {
		return ""
	}
	return s.streamReply
}

func (s *CoCreateSession) StreamThinking() string {
	if s == nil {
		return ""
	}
	return s.streamThinking
}

func (s *CoCreateSession) DraftPrompt() string {
	if s == nil {
		return ""
	}
	return s.draftPrompt
}

func (s *CoCreateSession) Suggestions() []string {
	if s == nil {
		return nil
	}
	return s.suggestions
}

// Proposal returns the latest model proposal, retained when a reply omits it.
// The returned proposal is read-only.
func (s *CoCreateSession) Proposal() *host.CoCreateProposal {
	if s == nil {
		return nil
	}
	return s.proposal
}

func (s *CoCreateSession) Ready() bool {
	if s == nil {
		return false
	}
	return s.ready
}

func (s *CoCreateSession) CanStart() bool {
	return strings.TrimSpace(s.DraftPrompt()) != ""
}

func (s *CoCreateSession) InitialInput() string {
	if s == nil || len(s.history) == 0 {
		return ""
	}
	return strings.TrimSpace(s.history[0].Content)
}

func (s *CoCreateSession) BuildPrompt() (string, error) {
	if s == nil || !s.CanStart() {
		return "", fmt.Errorf("cần nhập yêu cầu cho bản nháp đồng sáng tác")
	}
	return s.DraftPrompt(), nil
}
