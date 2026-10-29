// PlanPanel — “Kế hoạch truyện” (21-dev-prompt.md › Gói P · P1 + Main.dc.html › phase=plan):
// khung thay “Luồng viết trực tiếp” ở giai đoạn lập kế hoạch. Chấm + thanh tiến độ màu
// Kiến trúc sư, phụ đề “Kiến trúc sư · n/5 bước”, nhãn chạy theo bước.
// Năm bước bấm mở: Thông tin truyện · Tiền đề · Nhân vật (chip → Não truyện) ·
// Quy tắc thế giới · Dàn ý (lưới 2 cột, chương chưa lưu là thanh chờ). Bài giải ở chân
// danh sách là một dòng trong vùng cuộn, không phải thanh riêng.
import { useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { PlanChapter, PlanStep, PlanStepDetail, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Scroll } from '@noverse/ui'
import { Shimmer, Skeleton, Spinner } from '@noverse/ui'
import { formatDuration, formatTimer } from '../../lib/format'
import { agentLabel, activeToolLabel } from '../../lib/labels'
import { agentAura, agentBar } from '../../ui/agentPalette'
import { openBrainDrawer } from './brainDrawer'
import { useElapsed } from './hooks'
import { s } from './planPanel.stylex'
import { CharAvatar } from '../../ui/CharAvatar'

const STEP_LABEL: Record<string, string> = {
  book: 'Thông tin truyện',
  premise: 'Tiền đề',
  characters: 'Nhân vật',
  worldRules: 'Quy tắc thế giới',
  outline: 'Dàn ý',
}

/** Nhãn nhỏ bên phải nhãn chạy: agent đang lưu loại thiết lập nào. */
function runningLabel(snapshot: Snapshot): string {
  const agent = (snapshot.agents ?? []).find((a) => {
    const st = a.state.toLowerCase()
    return st === 'running' || st === 'working'
  })
  if (!agent) return ''
  const match = /save_foundation\[([a-z_]+)\]/.exec(agent.summary)
  const variant = agent.tool === 'save_foundation' ? match?.[1] ?? '' : ''
  return agent.tool ? activeToolLabel(agent.tool, variant) : ''
}

function StatusIcon({ status, paused }: { status: PlanStep['status']; paused: boolean }) {
  if (status === 'running' && !paused) return <Spinner size={14} color="var(--color-info)" />
  if (status === 'done') return <Icon name="tick-circle-bulk" size={15} label={t.workspace.done} style={{ color: 'var(--color-ok)' }} />
  if (status === 'paused' || paused) return <Icon name="pause" size={15} label={t.status.paused} style={{ color: 'var(--color-warn)' }} />
  return <Icon name="timer-1" size={14} style={{ color: 'var(--color-fg-4)' }} />
}

function BookBody({ detail }: { detail: Extract<PlanStepDetail, { kind: 'book' }> }) {
  const rows = ([['Tên truyện', detail.title], ['Giới thiệu', detail.synopsis]] as [string, string][]).filter((row) => row[1] !== '')
  const kvRow = stylex.props(s.kvRow)
  const kvValue = stylex.props(s.kvValue)
  return (
    <div {...kvRow}>
      {rows.map(([key, value]) => (
        <span key={key} {...stylex.props(s.kvChip)} title={value}>
          {key} <b {...kvValue}>{value}</b>
        </span>
      ))}
    </div>
  )
}

function CharactersBody({ detail }: { detail: Extract<PlanStepDetail, { kind: 'characters' }> }) {
  const chipRow = stylex.props(s.chipRow)
  const chip = stylex.props(s.charChip)
  const role = stylex.props(s.charRole)
  return (
    <div {...chipRow}>
      {detail.characters.map((c) => (
        <button key={c.name} type="button" title={`${c.name}${c.role ? ` (${c.role})` : ''} — ${t.plan.openBrain}`}
          onClick={() => openBrainDrawer(c.name)} {...chip} className={['hov-ghost', chip.className].filter(Boolean).join(' ')}>
          <CharAvatar name={c.name} kind={c.kind} size="sm" />
          {c.name}
          {c.role ? <span {...role}>{c.role.split('·')[0].trim().toLowerCase()}</span> : null}
        </button>
      ))}
    </div>
  )
}

function OutlineBody({ detail }: { detail: Extract<PlanStepDetail, { kind: 'outline' }> }) {
  const grid = stylex.props(s.outlineGrid)
  const cell = stylex.props(s.outlineCell)
  const num = stylex.props(s.outlineNum)
  const title = stylex.props(s.outlineTitle)
  const saved = new Map(detail.chapters.map((c) => [c.chapter, c]))
  const cells: PlanChapter[] = []
  for (let n = 1; n <= detail.total; n++) {
    cells.push(saved.get(n) ?? { chapter: n, title: '', hook: '' })
  }
  return (
    <ol {...grid}>
      {cells.map((c) => (
        <li key={c.chapter} {...cell}
          title={c.title ? `${c.title}${c.hook ? ` · ${c.hook}` : ''}` : t.plan.chapterWaiting(c.chapter)}>
          <span className="num" {...num}>{c.chapter}</span>
          {c.title
            ? <span {...title}>{c.title}{c.hook ? <span {...stylex.props(s.outlineHook)}> · {c.hook}</span> : null}</span>
            : <Skeleton height={9} width="72%" />}
        </li>
      ))}
    </ol>
  )
}

function StepBody({ step }: { step: PlanStep }) {
  const body = stylex.props(s.body)
  const detail = step.detail
  if (!detail) return null
  return (
    <div {...body}>
      {detail.kind === 'book' && <BookBody detail={detail} />}
      {detail.kind === 'premise' && <p {...stylex.props(s.premise)}>{detail.text}</p>}
      {detail.kind === 'characters' && <CharactersBody detail={detail} />}
      {detail.kind === 'worldRules' && (
        <ul {...stylex.props(s.ruleList)}>
          {detail.rules.map((rule, i) => (
            <li key={i}>{rule.rule}{rule.boundary ? ` — ${rule.boundary}` : ''}</li>
          ))}
        </ul>
      )}
      {detail.kind === 'outline' && <OutlineBody detail={detail} />}
    </div>
  )
}

export function PlanPanel({ snapshot, mobile = false, loading = false }: {
  snapshot: Snapshot | null
  mobile?: boolean
  loading?: boolean
}) {
  const plan = snapshot?.plan ?? null
  const [overrides, setOverrides] = useState<Record<string, boolean>>({})
  const paused = snapshot?.runtimeState === 'paused'
  const running = !!snapshot?.isRunning && !paused
  const elapsedSec = Math.floor(useElapsed(running, snapshot?.plan?.runningStep ?? 'plan'))
  const label = useMemo(() => (running && snapshot ? runningLabel(snapshot) : ''), [running, snapshot])
  const progress = plan && plan.total > 0 ? (plan.done / plan.total) * 100 : 0
  const agentName = useMemo(() => {
    const agent = (snapshot?.agents ?? []).find((a) => a.name.toLowerCase().includes('architect'))
      ?? (snapshot?.agents ?? [])[0]
    return agent ? agentLabel(agent.name) || 'Kiến trúc sư' : 'Kiến trúc sư'
  }, [snapshot])
  const meta = plan ? `${agentName} · ${plan.done}/${plan.total} ${t.plan.steps}` : ''

  return (
    <section className="card" aria-label={t.plan.title} style={{
      flex: mobile ? '1 1 0' : '7 1 0', minHeight: 0, display: 'flex', flexDirection: 'column',
      borderRadius: 14, overflow: 'hidden',
    }}>
      <div style={{ flex: '0 0 2px', height: 2, background: 'var(--color-surface-5)' }}>
        <div style={{ width: `${progress}%`, height: '100%', background: agentBar('architect'), transition: 'width .3s var(--ease-out)' }} />
      </div>
      <div style={{ flex: 1, minHeight: 0, padding: '8px 14px 0 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div style={{ flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px 10px' }}>
          <AgentOrb agent="architect" size={16} live={running} dim={!running} />
          <span style={{ fontSize: 13, fontWeight: 500 }}>{t.plan.title}</span>
          {meta !== '' && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{meta}</span>}
          <span style={{ flex: 1 }} />
          {running && (
            <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <svg width="22" height="12" viewBox="0 0 26 14" fill="none" aria-hidden="true">
                <defs>
                  <linearGradient id="plan-infinity" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0" stopColor="var(--color-architect)" />
                    <stop offset="0.5" stopColor="var(--color-writer)" />
                    <stop offset="1" stopColor="var(--color-user)" />
                  </linearGradient>
                </defs>
                <path d="M13 7C11 4.2 9 2.5 6.5 2.5a4.5 4.5 0 0 0 0 9C9 11.5 11 9.8 13 7s4-4.5 6.5-4.5a4.5 4.5 0 0 1 0 9C17 11.5 15 9.8 13 7z" stroke="var(--line-5)" strokeWidth="1.6" />
                <path pathLength="50" d="M13 7C11 4.2 9 2.5 6.5 2.5a4.5 4.5 0 0 0 0 9C9 11.5 11 9.8 13 7s4-4.5 6.5-4.5a4.5 4.5 0 0 1 0 9C17 11.5 15 9.8 13 7z"
                  stroke="url(#plan-infinity)" strokeWidth="1.8" strokeLinecap="round"
                  style={{ strokeDasharray: '14 36', animation: 'ai-infinity 1.8s linear infinite' }} />
              </svg>
              {label !== '' && <Shimmer style={{ fontSize: 12 }}>{label}</Shimmer>}
              <span className="num" role="timer" aria-label={`Đã chạy ${elapsedSec} giây`} style={{
                display: 'inline-block', minWidth: '5ch', textAlign: 'right', fontSize: 12, color: 'var(--color-fg-4)',
              }}>{formatTimer(elapsedSec * 1000)}</span>
            </span>
          )}
        </div>
        <Scroll stickToBottom style={{ padding: '0 4px 10px 0' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {loading && !plan && [0, 1, 2, 3, 4].map((row) => <Skeleton key={row} height={38} radius={12} />)}
            {plan?.steps.map((step) => {
              const active = step.status === 'running'
              const open = overrides[step.key] ?? active
              // Main.dc.html: bước Dàn ý đọc “Đang lưu · 5/8 chương” / “Tạm dừng · đã lưu 5/8 chương”.
              const summary = step.key === 'outline' && step.detail?.kind === 'outline'
                ? active && !paused
                  ? t.plan.outlineSaving(step.detail.saved, step.detail.total)
                  : paused ? t.plan.outlinePaused(step.detail.saved, step.detail.total) : step.summary
                : step.summary
              return (
                <div key={step.key} style={{
                  borderRadius: 12,
                  background: active ? agentAura('architect', 0.05) : 'var(--color-surface-0)',
                  border: `1px solid ${active ? 'var(--kit-info-30)' : 'var(--line-1)'}`,
                }}>
                  <button type="button" className={`hov-ghost ${stylex.props(s.step).className}`} aria-expanded={open}
                    onClick={() => setOverrides((previous) => ({ ...previous, [step.key]: !open }))}>
                    <StatusIcon status={step.status} paused={paused} />
                    <span {...stylex.props(s.stepName)}>{STEP_LABEL[step.key] ?? step.key}</span>
                    {summary !== '' && <span {...stylex.props(s.stepSummary)} title={summary}>{summary}</span>}
                    {step.durationMs > 0 && (
                      <span className={`num ${stylex.props(s.stepDuration).className}`}>{formatDuration(step.durationMs)}</span>
                    )}
                    <Icon name="arrow-down-2" size={14} style={{
                      color: 'var(--color-fg-4)', flexShrink: 0,
                      transform: `rotate(${open ? 180 : 0}deg)`,
                      transition: 'transform .15s var(--ease-out)',
                    }} />
                  </button>
                  {open && <StepBody step={step} />}
                </div>
              )
            })}
            {plan && <p {...stylex.props(s.footer)}>{t.plan.footer}</p>}
          </div>
        </Scroll>
      </div>
    </section>
  )
}
