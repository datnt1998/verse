// Chọn trợ lý chủ của khung “Luồng viết trực tiếp” (Gói O · mục 6–9 của
// 21-dev-prompt.md): tên và màu theo trợ lý đang chạy, không cứng về Người viết.
import type { AgentSnapshot, Snapshot } from '../../api/types'
import { agentKindOf } from '../../ui/agentPalette'
import type { AgentKind } from '@noverse/ui'
import { agentLabel } from '../../lib/labels'
/** Agent ở trạng thái đang làm (`working` từ dispatch, `running` từ mock/dữ liệu cũ). */
const busy = (agent: AgentSnapshot) => {
  const state = agent.state.toLowerCase()
  return state === 'working' || state === 'running'
}

/**
 * Trợ lý chủ khung luồng viết:
 * 1. agent bận có `tool` khớp `liveTool` của luồng;
 * 2. agent bận còn có `tool` (luồng vừa dứt nhưng dispatch vẫn chạy);
 * 3. agent bận đầu tiên; 4. null.
 */
export function streamAgentOf(agents: AgentSnapshot[] | null | undefined, liveTool: string): AgentSnapshot | null {
  const active = (agents ?? []).filter(busy)
  return active.find((agent) => !!agent.tool && agent.tool === liveTool)
    ?? active.find((agent) => !!agent.tool)
    ?? active[0]
    ?? null
}

/** Giai đoạn không có agent nào bận nhưng vẫn là chủ khung: tiền đề/dàn ý thuộc Kiến trúc sư. */
export function phaseKindOf(phase: string | undefined): AgentKind | null {
  return phase === 'premise' || phase === 'outline' ? 'architect' : null
}

/** Màu khung luồng viết: agent đang chạy → giai đoạn lập kế hoạch → Người viết. */
export function streamKindOf(snapshot: Snapshot | null | undefined, agent: AgentSnapshot | null): AgentKind {
  if (agent) return agentKindOf(agent.name)
  return phaseKindOf(snapshot?.phase) ?? 'writer'
}
/** Tên hiển thị của agent; khi snapshot chưa kịp ghi trạng thái thì theo màu chủ khung. */
export function streamAgentName(agent: AgentSnapshot | null, kind: AgentKind): string {
  if (agent) return agentLabel(agent.name) || 'Trợ lý'
  return agentLabel(kind) || 'Trợ lý'
}
