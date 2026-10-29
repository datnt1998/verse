// Vòng tiến độ của một việc — orb agent hoặc icon loại việc ở giữa, chấm đầu cung xoay theo %.
import type { CSSProperties } from 'react'
import type { Job } from '../../api/types'
import { agentAuraBg, agentChipBg, agentOrbGlow } from '../../ui/agentPalette'
import { Icon } from '@noverse/ui'
import { DONE_COLOR, hasRingHead, iconColor, isIndeterminate, kindIcon, ringColor, ringHeadColor } from './logic'
import { ProgressRing } from '@noverse/ui'

export function TickMark({ size, color = DONE_COLOR, stroke = 2.4 }: { size: number; color?: string; stroke?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path className="jd-draw" d="M6 12.5l4 4 8-9" stroke={color} strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

/**
 * `variant="pill"`: 34 px, viền tối 2 px, orb 18 · icon 15 (pill & chấm).
 * `variant="row"`: 38 px, không viền, orb 22 có quầng thở · icon 16 · tick vẽ nét khi xong.
 */
export function JobRing({ job, variant, style }: { job: Job; variant: 'pill' | 'row' | 'mobile'; style?: CSSProperties }) {
  const row = variant !== 'pill'
  const size = variant === 'mobile' ? 42 : row ? 38 : 34
  const done = job.status === 'done'
  const agent = job.agent && !(row && done) ? job.agent : undefined
  const running = job.status === 'running'
  const chip = job.agent ? agentChipBg(job.agent) : done ? 'rgba(166,237,193,0.1)' : '#141416'
  const deg = Math.round((job.progress ?? 0) * 360)
  return (
    <span className={`jd-rg${row && job.status === 'paused' ? ' jd-dimmed' : ''}`}
      style={{ width: size, height: size, background: chip, ...(row ? { boxShadow: 'none' } : null), ...style }}>
      {row && agent && (
        <span className="jd-aura" aria-hidden="true"
          style={{ background: agentAuraBg(agent), animationPlayState: running ? 'running' : 'paused' }} />
      )}
      <ProgressRing size={size} percent={['waiting', 'failed', 'done'].includes(job.status) ? 100 : (job.progress ?? 0) * 100}
        color={ringColor(job)} indeterminate={isIndeterminate(job) && running} style={{ position: 'absolute', inset: 0 }} />
      {variant !== 'mobile' && hasRingHead(job) && (
        <i className="jd-head" style={{ transform: `rotate(${deg}deg)` }}>
          <b style={{ boxShadow: `0 0 6px 2px ${ringHeadColor(job)}` }} />
        </i>
      )}
      {agent ? (
        <span className={`jd-orb${running ? ' jd-live' : ''}`} aria-hidden="true"
          style={{ position: 'relative', width: row ? 22 : 18, height: row ? 22 : 18, background: `var(--orb-${agent})`, boxShadow: agentOrbGlow(agent) }} />
      ) : row && done ? (
        <TickMark size={18} />
      ) : (
        <Icon name={kindIcon(job)} size={variant === 'mobile' ? 18 : row ? 16 : 15} style={{ color: iconColor(job), position: 'relative' }} />
      )}
    </span>
  )
}

/** Vòng khi không còn việc hoạt động: vòng mint đầy + tick. */
export function DoneRing() {
  return (
    <span className="jd-rg" style={{ width: 34, height: 34 }}>
      <ProgressRing size={34} percent={100} color={DONE_COLOR} style={{ position: 'absolute', inset: 0 }} />
      <Icon name="tick-circle" size={15} style={{ color: DONE_COLOR }} />
    </span>
  )
}

