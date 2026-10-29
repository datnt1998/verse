package agents

import (
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore"
)

// SubAgentEventLogger 返回 subagent.Config.OnEvent 的实现：把信号事件经
// Sessions 写进与消息同一本 session 文件。事件 → (kind, 短字段) 的映射在
// store.SessionEventFields；本层只负责挂上 OnEvent 签名。
func SubAgentEventLogger(s *store.Store) func(agentName, task string, ev agentcore.Event) {
	write := s.Sessions.SubAgentEventLogger()
	return func(agentName, task string, ev agentcore.Event) {
		kind, fields := store.SessionEventFields(ev)
		if kind == "" {
			return
		}
		write(agentName, task, kind, fields)
	}
}
