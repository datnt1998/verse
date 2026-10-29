package context

import (
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"

	"time"
)

// ContextSummary is a compacted context summary message.
// It implements AgentMessage but is NOT a Message, so DefaultConvertToLLM
// will filter it out. Use ContextConvertToLLM to handle it.
type ContextSummary struct {
	Summary       string
	TokensBefore  int
	ReadFiles     []string
	ModifiedFiles []string
	Timestamp     time.Time
}

func (c ContextSummary) GetRole() llmtypes.Role  { return llmtypes.RoleUser }
func (c ContextSummary) GetTimestamp() time.Time { return c.Timestamp }
func (c ContextSummary) TextContent() string     { return c.Summary }
func (c ContextSummary) ThinkingContent() string { return "" }
func (c ContextSummary) HasToolCalls() bool      { return false }

// ContextConvertToLLM converts AgentMessages to LLM Messages,
// handling ContextSummary by wrapping it as a user message with XML tags.
// For all other message types, it delegates to DefaultConvertToLLM behavior.
func ContextConvertToLLM(msgs []llmtypes.AgentMessage) []llmtypes.Message {
	out := make([]llmtypes.Message, 0, len(msgs))
	for _, m := range msgs {
		switch v := m.(type) {
		case ContextSummary:
			out = append(out, llmtypes.Message{
				Role:    llmtypes.RoleUser,
				Content: []llmtypes.ContentBlock{llmtypes.TextBlock("<context-summary>\n" + v.Summary + "\n</context-summary>")},
				Metadata: map[string]any{
					"type":           "context_summary",
					"tokens_before":  v.TokensBefore,
					"read_files":     v.ReadFiles,
					"modified_files": v.ModifiedFiles,
				},
				Timestamp: v.Timestamp,
			})
		case llmtypes.Message:
			if v.StopReason == llmtypes.StopReasonError || v.StopReason == llmtypes.StopReasonAborted {
				continue
			}
			out = append(out, v)
		}
	}
	return out
}

// PinnedPrefixLength protects leading system instructions and the original
// user request. Summary checkpoints belong after this immutable prefix.
func PinnedPrefixLength(msgs []llmtypes.AgentMessage) int {
	n := 0
	for n < len(msgs) && msgs[n].GetRole() == llmtypes.RoleSystem {
		n++
	}
	if n < len(msgs) {
		if m, ok := msgs[n].(llmtypes.Message); ok && m.Role == llmtypes.RoleUser {
			n++
		}
	}
	return n
}
