// Orb agent — gradient radial theo token --orb-*. live → ai-breathe; dim → mờ.
import type { CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { agent } from './tokens.stylex'

export type AgentKind = 'architect' | 'writer' | 'editor' | 'arbiter'

/** var(--orb-<agent>) — tham chiếu qua VarGroup để StyleX phát sinh biến vào :root. */
const AGENT_ORB: Record<AgentKind, string> = {
  architect: agent['--orb-architect'], arbiter: agent['--orb-arbiter'],
  writer: agent['--orb-writer'], editor: agent['--orb-editor'],
}

const sx = stylex.create({
  orb: (size: string, background: string, opacity: string) => ({
    width: size, height: size, borderRadius: '50%', flex: '0 0 auto',
    backgroundImage: background, boxShadow: 'var(--orb-shade)', opacity,
  }),
  live: { animationName: 'ai-breathe', animationDuration: '2.8s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite' },
})

export function AgentOrb({ agent, size, live = false, dim = false, label, style, className }: {
  agent: AgentKind; size: number; live?: boolean; dim?: boolean
  /** đứng một mình thì truyền nhãn (role=img) */
  label?: string
  style?: CSSProperties; className?: string
}) {
  const p = stylex.props(sx.orb(`${size}px`, AGENT_ORB[agent], dim ? '0.5' : '1'), live && sx.live)
  const merged = { ...p.style, ...style }
  if (label) return <span className={[p.className, className].filter(Boolean).join(' ')} role="img" aria-label={label} style={merged} />
  return <span className={[p.className, className].filter(Boolean).join(' ')} aria-hidden="true" style={merged} />
}
