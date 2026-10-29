import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button } from './Button'
import { Icon } from './Icon'
import { AgentOrb, type AgentKind } from './AgentOrb'
import { agent as agentVars } from './tokens.stylex'

export type BannerProps = {
  /** `agent` = banner do một trợ lý nói (orb + màu trợ lý) — kèm `agent` prop. */
  tone: 'info' | 'success' | 'warning' | 'error' | 'agent'
  /** trợ lý đang nói khi `tone="agent"` (khoá của AgentOrb); mặc định `writer` */
  agent?: AgentKind
  title: string; children?: ReactNode; action: { label: string; onClick: () => void }
}

// Màu + biểu tượng theo Kit.dc.html › “Thông báo trong trang”: info-circle · tick-circle · warning-2 · danger.
// Giá trị là chuỗi var(--kit-*) để truyền cả vào style nội tuyến (Icon) và stylex (khung).
const tones = {
  info: { background: 'var(--kit-info-7)', border: 'var(--kit-info-20)', color: 'var(--kit-info-fg)', icon: 'info-circle' },
  success: { background: 'var(--kit-ok-6)', border: 'var(--kit-ok-20)', color: 'var(--color-ok)', icon: 'tick-circle' },
  warning: { background: 'var(--kit-warn-6)', border: 'var(--kit-warn-20)', color: 'var(--color-warn)', icon: 'warning-2' },
  error: { background: 'var(--kit-err-7)', border: 'var(--kit-errb-22)', color: 'var(--kit-err-bright)', icon: 'danger' },
} as const

/** tone="agent": nền/viền nhạt theo quầng của trợ lý (token --banner-agent-*), icon = orb của trợ lý đó. */
const AGENT_LOOK: Record<AgentKind, { background: string; border: string; color: string }> = {
  architect: { background: agentVars['--banner-agent-architect-bg'], border: agentVars['--banner-agent-architect-bd'], color: 'var(--color-architect)' },
  writer: { background: agentVars['--banner-agent-writer-bg'], border: agentVars['--banner-agent-writer-bd'], color: 'var(--color-writer)' },
  editor: { background: agentVars['--banner-agent-editor-bg'], border: agentVars['--banner-agent-editor-bd'], color: 'var(--color-editor)' },
  arbiter: { background: agentVars['--banner-agent-arbiter-bg'], border: agentVars['--banner-agent-arbiter-bd'], color: 'var(--color-arbiter)' },
}

const styles = stylex.create({
  root: {
    display: 'flex', gap: 12, alignItems: 'center', paddingTop: 10, paddingBottom: 10,
    paddingInline: 12, borderRadius: 12, borderWidth: 1, borderStyle: 'solid',
  },
  tone: (background: string, border: string) => ({ backgroundColor: background, borderColor: border }),
  body: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  title: { fontSize: 13, fontWeight: 500 },
  text: { fontSize: 12, color: 'var(--color-fg-3)' },
})

export function Banner({ tone, title, children, action, agent = 'writer' }: BannerProps) {
  const look = tone === 'agent' ? AGENT_LOOK[agent] : tones[tone]
  const sx = stylex.props(styles.root, styles.tone(look.background, look.border))
  return <div role={tone === 'error' ? 'alert' : 'status'} className={sx.className} style={sx.style}>{tone === 'agent' ? <AgentOrb agent={agent} size={17} /> : <Icon name={tones[tone].icon} size={17} style={{ color: look.color }} />}<span {...stylex.props(styles.body)}><span {...stylex.props(styles.title)}>{title}</span>{children && <span {...stylex.props(styles.text)}>{children}</span>}</span><Button size="sm" onClick={action.onClick}>{action.label}</Button></div>
}
