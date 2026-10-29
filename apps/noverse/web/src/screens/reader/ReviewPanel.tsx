// ReviewPanel — thẩm duyệt mới nhất (verdict, hợp đồng, 7 tiêu chí, IssueCard) +
// các panel kế hoạch / dữ kiện / thay đổi / phục bút của chương.
import { useId, useState, type ReactNode } from 'react'
import type { ApiError } from '../../api/client'
import type { ChapterFacts, ChapterPlan, ForeshadowEntry, ReviewDecision, ReviewEntry, ReviewItem, RevisionEntry } from '../../api/types'
import { AgentOrb } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Card, TONE, type Tone } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import {
  OTHER_VALUE, contractStatusLabel, foreshadowActionLabel, hookTypeLabel, strandLabel, verdictLabel,
} from '../../lib/labels'
import { revisionKindLabel } from '../../lib/markdown'
import { packLabel } from '../settings/lib/packs'
import { t } from '../../i18n/vi'
import { EmptyState, Skeleton } from '@noverse/ui'
import { copy, formatDateTime } from './copy'
import type { ReviewView } from './useReaderData'
import { ReaderQualityLink } from '../quality/QualityLink'
import { ReviewIssue, reviewBlocking, reviewClock, reviewIssueCounts, reviewItemsOf, reviewLevel } from '../../ui/ReviewIssue'

const DIMENSION_ORDER = ['consistency', 'character', 'pacing', 'continuity', 'foreshadow', 'hook', 'aesthetic']

const verdictTone = (verdict: string): Tone =>
  verdict === 'accept' ? 'ok' : verdict === 'polish' ? 'warn' : verdict === 'rewrite' ? 'err' : 'neutral'


/** Chip trạng thái của một mục goal_checks — tông đúng nghĩa phán quyết. */
const GOAL_STATUS: Record<string, { tone: Tone; label: string }> = {
  pass: { tone: 'ok', label: 'đạt' },
  fail: { tone: 'err', label: 'trượt' },
  unknown: { tone: 'warn', label: 'chưa rõ' },
}
/** Nền/chữ của chip kết luận — đúng màu artboard (12% cho nền, không dùng tint 10%). */
const VERDICT_CHIP: Record<Tone, { background: string; color: string }> = {
  ok: { background: 'var(--kit-ok-12)', color: 'var(--color-ok)' },
  warn: { background: 'var(--kit-warn-12)', color: 'var(--color-warn)' },
  err: { background: 'var(--kit-errf-12)', color: 'var(--color-err)' },
  neutral: { background: 'var(--color-surface-5)', color: 'var(--color-fg-4)' },
  accent: { background: 'var(--tint-accent)', color: 'var(--color-accent-soft)' },
  info: { background: 'var(--tint-info)', color: 'var(--color-info)' },
}

function Pill({ children, color, background = 'var(--color-surface-0)' }: { children: ReactNode; color?: string; background?: string }) {
  return (
    <span style={{
      height: 24, padding: '0 8px', display: 'inline-flex', alignItems: 'center', gap: 4,
      borderRadius: 6, background, border: '1px solid var(--line-1)',
      color: color ?? 'var(--color-fg-3)', fontSize: 12, whiteSpace: 'nowrap',
    }}>{children}</span>
  )
}

function CollapsibleCard({ title, icon, count, children }: { title: string; icon: string; count?: ReactNode; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  return (
    <Card padding={14} radius={14} style={{ display: 'flex', flexDirection: 'column', gap: open ? 8 : 0 }}>
      <button type="button" aria-expanded={open} aria-controls={id} onClick={() => setOpen((value) => !value)}
        style={{
          display: 'flex', alignItems: 'center', gap: 8, padding: 0, border: 'none', background: 'transparent',
          color: 'var(--color-fg-3)', fontSize: 13, fontWeight: 500, textAlign: 'left', minHeight: 20,
        }}>
        <Icon name={icon} size={15} />
        <span style={{ flex: 1, minWidth: 0 }}>{title}</span>
        {count !== undefined && count !== null && <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{count}</span>}
        <Icon name="arrow-down-2" size={14} style={{ color: 'var(--color-fg-4)', transform: `rotate(${open ? '180deg' : '0deg'})` }} />
      </button>
      {open && <div id={id} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{children}</div>}
    </Card>
  )
}

function TextRow({ label, value }: { label: string; value?: string }) {
  if (!value) return null
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <span className="lbl">{label}</span>
      <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{value}</span>
    </div>
  )
}

function ListRow({ label, items }: { label: string; items?: string[] }) {
  if (!items || items.length === 0) return null
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <span className="lbl">{label}</span>
      <ul style={{ margin: 0, paddingLeft: 16, display: 'flex', flexDirection: 'column', gap: 2, fontSize: 12, color: 'var(--color-fg-2)' }}>
        {items.map((item, index) => <li key={index}>{item}</li>)}
      </ul>
    </div>
  )
}

function EmptyLine({ children }: { children: ReactNode }) {
  return <p style={{ margin: 0, fontSize: 12, color: 'var(--color-fg-4)' }}>{children}</p>
}

function PlanFields({ plan }: { plan: ChapterPlan | null }) {
  if (!plan) return <EmptyLine>{copy.planEmpty}</EmptyLine>
  const contract = plan.contract
  return (
    <>
      <TextRow label={copy.fields.goal} value={plan.goal} />
      <TextRow label={copy.fields.conflict} value={plan.conflict} />
      <TextRow label={copy.fields.hook} value={plan.hook} />
      <TextRow label={copy.fields.emotionArc} value={plan.emotion_arc} />
      <TextRow label={copy.fields.openingState} value={plan.opening_state} />
      <TextRow label={copy.fields.notes} value={plan.notes} />
      <ListRow label={copy.fields.requiredBeats} items={contract?.required_beats} />
      <ListRow label={copy.fields.forbiddenMoves} items={contract?.forbidden_moves} />
      <ListRow label={copy.fields.continuity} items={contract?.continuity_checks} />
      <ListRow label={copy.fields.evaluation} items={contract?.evaluation_focus} />
      <ListRow label={copy.fields.payoff} items={contract?.payoff_points} />
      <TextRow label={copy.fields.hookGoal} value={contract?.hook_goal} />
      <TextRow label={copy.fields.emotionTarget} value={contract?.emotion_target} />
    </>
  )
}

function FactsFields({ facts }: { facts: ChapterFacts | null }) {
  if (!facts) return <EmptyLine>{copy.factsEmpty}</EmptyLine>
  return (
    <>
      <TextRow label={copy.fields.summary} value={facts.summary} />
      <ListRow label={copy.fields.keyEvents} items={facts.key_events} />
      {facts.characters.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className="lbl">{copy.fields.characters}</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
            {facts.characters.map((name) => <Pill key={name}>{name}</Pill>)}
          </div>
        </div>
      )}
      {facts.timeline_events.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span className="lbl">{copy.fields.timeline}</span>
          {facts.timeline_events.map((event, index) => (
            <div key={index} style={{ display: 'flex', gap: 6, fontSize: 12, color: 'var(--color-fg-2)' }}>
              <span className="mono" style={{ color: 'var(--color-fg-4)', flex: '0 0 auto' }}>{event.time}</span>
              <span style={{ minWidth: 0 }}>{event.event}</span>
            </div>
          ))}
        </div>
      )}
      {(facts.hook_type || facts.dominant_strand) && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
          {facts.hook_type && <Pill color="var(--color-warn)">{hookTypeLabel(facts.hook_type)}</Pill>}
          {facts.dominant_strand && <Pill>{strandLabel(facts.dominant_strand)}</Pill>}
        </div>
      )}
    </>
  )
}

function RevisionsList({ revisions }: { revisions: RevisionEntry[] | null }) {
  if (revisions === null) {
    return <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }} aria-busy="true">
      <Skeleton height={12} /><Skeleton height={12} width="80%" />
    </div>
  }
  if (revisions.length === 0) return <EmptyLine>{copy.revisionsEmpty}</EmptyLine>
  return (
    <>
      {revisions.map((revision, index) => (
        <div key={index} style={{
          display: 'flex', flexDirection: 'column', gap: 4,
          paddingTop: index === 0 ? 0 : 8, borderTop: index === 0 ? 'none' : '1px solid var(--line-1)',
        }}>
          <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
            {formatDateTime(revision.time)} · {revisionKindLabel(revision.kind)}
          </span>
          <p style={{
            margin: 0, padding: '4px 8px', borderRadius: 7, background: 'var(--tint-err)',
            fontSize: 12, color: 'var(--color-fg-3)', textDecoration: 'line-through', textDecorationColor: 'var(--color-err)',
          }}>{revision.old}</p>
          <p style={{ margin: 0, padding: '4px 8px', borderRadius: 7, background: 'var(--tint-ok)', fontSize: 12 }}>{revision.new}</p>
        </div>
      ))}
    </>
  )
}

function ForeshadowList({ entries, facts }: { entries: ForeshadowEntry[] | null; facts: ChapterFacts | null }) {
  if (entries === null) {
    return <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }} aria-busy="true">
      <Skeleton height={12} /><Skeleton height={12} width="70%" />
    </div>
  }
  if (entries.length === 0) return <EmptyLine>{copy.foreshadowEmpty}</EmptyLine>
  const actions = new Map((facts?.foreshadow_updates ?? []).map((update) => [update.id, update.action]))
  return (
    <>
      {entries.map((entry) => {
        const tone: Tone = entry.status === 'resolved' ? 'ok' : entry.status === 'advanced' ? 'warn' : entry.status === 'abandoned' ? 'neutral' : 'info'
        const action = actions.get(entry.id)
        return (
          <div key={entry.id} style={{
            display: 'flex', flexDirection: 'column', gap: 3,
            paddingTop: 6, borderTop: '1px solid var(--line-1)',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Pill background={TONE[tone].bg} color={TONE[tone].fg}>
                {copy.foreshadowStatus[entry.status] ?? entry.status}
              </Pill>
              {action && <Pill color="var(--color-warn)">{foreshadowActionLabel(action)}</Pill>}
              <span style={{ flex: 1 }} />
              {entry.due_by ? <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{copy.dueBy(entry.due_by)}</span> : null}
              <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{copy.plantedAt(entry.planted_at)}</span>
            </div>
            <span style={{ fontSize: 12, color: 'var(--color-fg-2)' }}>{entry.description}</span>
            {entry.abandon_reason && <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{entry.abandon_reason}</span>}
          </div>
        )
      })}
    </>
  )
}

function Criteria({ dimensions }: { dimensions: ReviewView['entry']['dimensions'] }) {
  const scores = new Map((dimensions ?? []).map((item) => [item.dimension, item]))
  const extra = (dimensions ?? []).filter((item) => !DIMENSION_ORDER.includes(item.dimension))
  const keys = [...DIMENSION_ORDER, ...extra.map((item) => item.dimension)]
  return (
    <>
      {keys.map((key) => {
        const item = scores.get(key)
        const value = item ? Math.round(item.score) : null
        const tone = value === null ? 'var(--color-surface-5)' : value >= 80 ? 'var(--color-ok)' : value >= 60 ? 'var(--color-warn)' : 'var(--color-err)'
        return (
          <div key={key} style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12 }} title={item?.comment}>
            <span style={{ flex: '0 0 96px', color: 'var(--color-fg-2)' }}>{copy.dimension[key] ?? OTHER_VALUE}</span>
            <div style={{ flex: 1, height: 4, borderRadius: 4, background: 'var(--color-surface-5)', overflow: 'hidden' }}>
              {value !== null && <div style={{ width: `${Math.min(100, Math.max(0, value))}%`, height: '100%', borderRadius: 4, background: tone }} />}
            </div>
            <span className="num mono" style={{ flex: '0 0 44px', textAlign: 'right', color: 'var(--color-fg-4)' }}>
              {value === null ? '—/100' : `${value}/100`}
            </span>
          </div>
        )
      })}
    </>
  )
}

export function ReviewPanel({ chapter, review, plan, facts, revisions, foreshadow, error, onRetry, writable, onRequestFix, onKeep, onUnkeep }: {
  chapter: number
  review: ReviewView | null | undefined
  plan: ChapterPlan | null
  facts: ChapterFacts | null
  revisions: RevisionEntry[] | null
  foreshadow: ForeshadowEntry[] | null
  error: ApiError | null
  onRetry: () => void
  writable: boolean
  onRequestFix: (text: string) => void
  /** GAP-47 — giữ nguyên một Cảnh báo kèm lý do; trả về quyết định đã lưu, null khi lỗi */
  onKeep?: (item: ReviewItem, reason: string) => Promise<ReviewDecision | null>
  /** GAP-47 — bỏ giữ */
  onUnkeep?: (item: ReviewItem) => Promise<boolean>
}) {
  const [filter, setFilter] = useState<'all' | 'error' | 'warning'>('all')
  /** GAP-47 — quyết định vừa đổi trong phiên; `null` = đã bỏ giữ (đè `items[].decision` cũ). */
  const [decisions, setDecisions] = useState<Record<string, ReviewDecision | null>>({})
  const [saving, setSaving] = useState('')

  if (review === undefined) {
    return (
      <>
        <Skeleton height={96} radius={14} />
        <Skeleton height={168} radius={14} />
      </>
    )
  }

  if (review === null) {
    return (
      <>
        <Card padding={14} radius={14}>
          <EmptyState icon="shield-tick" title={copy.noReview} hint={copy.noReviewHint}
            action={error ? <Button size="sm" onClick={onRetry}>{copy.retry}</Button> : undefined} />
        </Card>
        <CollapsibleCard title={copy.plan} icon="note-text"><PlanFields plan={plan} /></CollapsibleCard>
        <CollapsibleCard title={copy.facts} icon="document-text"><FactsFields facts={facts} /></CollapsibleCard>
        <CollapsibleCard title={copy.revisions} icon="refresh-2" count={revisions?.length ?? undefined}>
          <RevisionsList revisions={revisions} />
        </CollapsibleCard>
        <CollapsibleCard title={copy.foreshadow} icon="flag" count={foreshadow?.length ?? undefined}>
          <ForeshadowList entries={foreshadow} facts={facts} />
        </CollapsibleCard>
      </>
    )
  }

  const entry = review.entry
  const items = reviewItemsOf(entry)
  const counts = reviewIssueCounts(items)
  const blocking = entry.blocking ?? reviewBlocking(items)
  const tone = blocking ? 'warn' : verdictTone(entry.verdict)
  const shown = items.filter((item) => filter === 'all' || reviewLevel(item) === filter)
  const subtitle = [
    'Biên tập viên',
    entry.pack ? `theo gói ${packLabel(entry.pack)}` : '',
    // GAP-47 — giờ chấm từ `at`; chưa có thì bỏ vế “· 21:52”.
    reviewClock(entry.at),
  ].filter(Boolean).join(' · ')
  const contractTone: Tone = entry.contract_status === 'met' ? 'ok' : entry.contract_status === 'partial' ? 'warn' : entry.contract_status === 'missed' ? 'err' : 'neutral'
  const filters: { id: 'all' | 'error' | 'warning'; label: string; dot: string }[] = [
    { id: 'all', label: t.review.counts.all(counts.all), dot: 'var(--color-fg-4)' },
    { id: 'error', label: t.review.counts.errors(counts.errors), dot: 'var(--color-err)' },
    { id: 'warning', label: t.review.counts.warnings(counts.warnings), dot: 'var(--color-warn)' },
  ]

  const keep = async (item: ReviewItem, reason: string) => {
    if (!onKeep) return false
    setSaving(item.id)
    const saved = await onKeep(item, reason)
    setSaving('')
    if (saved) setDecisions((previous) => ({ ...previous, [item.id]: saved }))
    return !!saved
  }
  const unkeep = async (item: ReviewItem) => {
    if (!onUnkeep) return
    setSaving(item.id)
    const removed = await onUnkeep(item)
    setSaving('')
    if (removed) setDecisions((previous) => ({ ...previous, [item.id]: null }))
  }
  const decisionOf = (item: ReviewItem): ReviewDecision | undefined =>
    (item.id in decisions ? decisions[item.id] ?? undefined : item.decision)

  return (
    <>
      <Card padding={14} radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <AgentOrb agent="editor" size={26} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 13, fontWeight: 600 }}>{t.review.title(chapter || entry.chapter)}</div>
            <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{subtitle}</div>
          </div>
          {entry.recheck && <Pill color="var(--color-info)">Kiểm lại</Pill>}
          <span style={{
            height: 26, padding: '0 10px', display: 'flex', alignItems: 'center', borderRadius: 7,
            ...VERDICT_CHIP[tone], fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap',
          }}>{verdictLabel(entry.verdict)}</span>
        </div>
        <div role="group" aria-label={t.review.filterLabel} style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {filters.map((option) => {
            const on = filter === option.id
            return (
              <button key={option.id} type="button" aria-pressed={on} onClick={() => setFilter(option.id)} style={{
                height: 26, padding: '0 9px', display: 'flex', alignItems: 'center', gap: 6, borderRadius: 7,
                border: `1px solid ${on ? 'var(--kit-wash-14)' : 'var(--line-1)'}`,
                background: on ? 'var(--color-surface-6)' : 'var(--color-surface-0)',
                color: on ? 'var(--color-fg)' : 'var(--color-fg-3)', fontSize: 12,
              }}>
                <span aria-hidden="true" style={{ width: 6, height: 6, borderRadius: '50%', background: option.dot }} />
                {option.label}
              </button>
            )
          })}
        </div>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>
          {blocking ? t.review.verdictNote.blocking(counts.errors) : t.review.verdictNote.clean}
        </p>
        <ReaderQualityLink />
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', fontSize: 12 }}>
          <Pill color={contractTone === 'neutral' ? undefined : TONE[contractTone].fg}>
            {copy.contract} {entry.contract_status ? contractStatusLabel(entry.contract_status) : '—'}
          </Pill>
        </div>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>{entry.summary}</p>
        {entry.contract_misses && entry.contract_misses.length > 0 && (
          <ul style={{ margin: 0, paddingLeft: 16, display: 'flex', flexDirection: 'column', gap: 2, fontSize: 12, color: 'var(--color-warn)' }}>
            {entry.contract_misses.map((miss, index) => <li key={index}>{miss}</li>)}
          </ul>
        )}
        {entry.goal_checks && entry.goal_checks.length > 0 && (
          <ul style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 4 }}>
            {entry.goal_checks.map((goal) => {
              const chip = GOAL_STATUS[goal.status] ?? { tone: 'neutral' as Tone, label: goal.status }
              return (
                <li key={goal.id} style={{ display: 'flex', alignItems: 'baseline', gap: 6, flexWrap: 'wrap', fontSize: 12 }}>
                  <span style={{ color: 'var(--color-fg-4)', fontFamily: 'monospace', flexShrink: 0 }}>{goal.id}</span>
                  <Pill color={chip.tone === 'neutral' ? undefined : TONE[chip.tone].fg}>{chip.label}</Pill>
                  <span style={{ color: 'var(--color-fg-2)', flex: 1, minWidth: 0 }}>{goal.text}</span>
                  {goal.quote && (
                    <span style={{ color: 'var(--color-fg-4)', fontStyle: 'italic', flexBasis: '100%', paddingLeft: 42 }}>
                      «{goal.quote}»
                    </span>
                  )}
                </li>
              )
            })}
          </ul>
        )}
        {entry.contract_notes && <p style={{ margin: 0, fontSize: 12, color: 'var(--color-fg-4)' }}>{entry.contract_notes}</p>}
        {entry.recheck && (entry.recheck_results ?? []).map((result) => {
          const source = entry.previous?.[entry.previous.length - 1]?.items.find((item) => item.id === result.finding_id)
          const chip = result.status === 'fixed' ? { tone: 'ok' as Tone, label: 'Đã sửa' }
            : result.status === 'unfixed' ? { tone: 'err' as Tone, label: 'Chưa sửa' }
            : { tone: 'warn' as Tone, label: 'Chưa rõ' }
          return <div key={result.finding_id} style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 12 }}>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <Pill color={TONE[chip.tone].fg}>{chip.label}</Pill>
              <span>{source?.text ?? result.finding_id}</span>
            </div>
            {source?.quote && <blockquote style={{ margin: 0, color: 'var(--color-fg-4)' }}>«{source.quote}»</blockquote>}
            {result.note && <span>{result.note}</span>}
          </div>
        })}
      </Card>

      {shown.map((item) => (
        <ReviewIssue key={item.id} item={item} variant="full" decision={decisionOf(item)} saving={saving === item.id}
          onKeep={writable && onKeep ? (reason) => keep(item, reason) : undefined}
          onUnkeep={writable && onUnkeep ? () => unkeep(item) : undefined}
          onRequestFix={writable ? onRequestFix : undefined} />
      ))}

      {items.length === 0 && (
        <Card padding={14} radius={14}>
          <EmptyState icon="shield-tick" title={copy.noIssues} hint={t.review.noItems} />
        </Card>
      )}

      <p style={{ margin: 0, padding: '0 4px', fontSize: 12, color: 'var(--color-fg-4)', lineHeight: 1.55 }}>
        <b style={{ color: 'var(--kit-err-soft)', fontWeight: 500 }}>{t.review.error}</b>{t.review.legendError}
        <b style={{ color: 'var(--kit-warn-soft)', fontWeight: 500 }}>{t.review.warning}</b>{t.review.legendWarning}
      </p>

      <Card padding={14} radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <h3 className="h3" style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' }}>
          {copy.criteria}
        </h3>
        <Criteria dimensions={entry.dimensions} />
      </Card>

      <CollapsibleCard title={copy.plan} icon="note-text"><PlanFields plan={plan} /></CollapsibleCard>
      <CollapsibleCard title={copy.facts} icon="document-text"><FactsFields facts={facts} /></CollapsibleCard>
      <CollapsibleCard title={copy.revisions} icon="refresh-2" count={revisions?.length ?? undefined}>
        <RevisionsList revisions={revisions} />
      </CollapsibleCard>
      <CollapsibleCard title={copy.foreshadow} icon="flag" count={foreshadow?.length ?? undefined}>
        <ForeshadowList entries={foreshadow} facts={facts} />
      </CollapsibleCard>
    </>
  )
}
