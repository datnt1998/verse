// Bảng màu từng trợ lý — 14-jobs-dock.md §1.5 (orb · gradient vòng & vạch · quầng).
// Dùng chung cho dock việc chạy nền và thẻ agent ở Workspace để hai nơi đồng bộ.
import type { AgentKind } from '@noverse/ui'

export type AgentTone = {
  /** tên hiển thị trong dòng phụ: “Kiến trúc sư · Bước 2/4 · …” */
  name: string
  c1: string
  c2: string
  c3: string
  /** "r,g,b" của quầng */
  aura: string
}

export const AGENT_PALETTE: Record<AgentKind, AgentTone> = {
  architect: { name: 'Kiến trúc sư', c1: '#B9D0F7', c2: '#7C93F3', c3: '#5A57E3', aura: '124,147,243' },
  writer: { name: 'Người viết', c1: '#C9A6F5', c2: '#9A63E3', c3: '#C9457F', aura: '154,99,227' },
  editor: { name: 'Biên tập viên', c1: '#F6CB8C', c2: '#EC9548', c3: '#E25E36', aura: '236,149,72' },
  arbiter: { name: 'Trọng tài', c1: '#A6EDC1', c2: '#5DC1B0', c3: '#3E73D6', aura: '93,193,176' },
}

export const ORB_SHADE = 'inset -3px -5px 10px rgba(0,0,0,.28), inset 2px 3px 6px rgba(255,255,255,.4)'

export const agentAura = (agent: AgentKind, alpha: number) => `rgba(${AGENT_PALETTE[agent].aura},${alpha})`

/** Tên engine → khoá màu: `architect_long/short` vẫn là Kiến trúc sư; tên lạ coi là Người viết. */
export const agentKindOf = (name: string): AgentKind => {
  const value = name.toLowerCase()
  if (value.includes('architect')) return 'architect'
  if (value.includes('editor')) return 'editor'
  if (value.includes('arbiter')) return 'arbiter'
  return 'writer'
}

/** Nền chip: radial rgba(quầng,.32) → trong suốt 70% trên #141416. */
export const agentChipBg = (agent: AgentKind) =>
  `radial-gradient(circle at 50% 55%, ${agentAura(agent, 0.32)} 0%, rgba(20,20,22,0) 70%), #141416`

/** Bóng orb: shade chuẩn + quầng `0 0 12px rgba(quầng,.55)`. */
export const agentOrbGlow = (agent: AgentKind, px = 12, alpha = 0.55) =>
  `${ORB_SHADE}, 0 0 ${px}px ${agentAura(agent, alpha)}`

/** Quầng ngoài nhịp thở quanh orb khi đang chạy. */
export const agentAuraBg = (agent: AgentKind) =>
  `radial-gradient(circle, ${agentAura(agent, 0.35)} 0%, ${agentAura(agent, 0)} 68%)`

/** Nền loang của thẻ agent ở Workspace (Jobs.dc.html › agents). */
export const agentWash = (agent: AgentKind, running: boolean) =>
  `radial-gradient(120% 140% at 0% 50%, ${agentAura(agent, running ? 0.16 : 0.06)}, transparent 60%)`

/** Vạch tiến độ 4 px theo agent. */
export const agentBar = (agent: AgentKind) => {
  const tone = AGENT_PALETTE[agent]
  return `linear-gradient(90deg,${tone.c1},${tone.c2},${tone.c3})`
}

