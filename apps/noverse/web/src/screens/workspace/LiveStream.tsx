// LiveStream — “Luồng viết trực tiếp” (Main.dc.html): thanh tiến độ round, header
// orb WRITER + ✦✧ + tool đang chạy (Shimmer) + đồng hồ, thân là round ✻ bám đáy.
// Thẻ mỗi vòng ✻: “<Bước> · <Trợ lý>”, MỘT câu tóm tắt lời thường từ tham số thật
// (lib/roundSummary), phần chi tiết theo loại (diff/commit/kế hoạch/thẩm duyệt), kết quả
// khi xong. Tên công cụ + tham số gốc chỉ nằm trong “Chi tiết kỹ thuật” (ẩn mặc định).
import { useMemo, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { ReviewEntry, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { paths, useRoute } from '../../router'
import { translateHost } from '../../i18n/hostStrings'
import { activeToolDoing, activeToolLabel, agentLabel, fieldLabel, fieldValueLabel, foreshadowActionLabel, severityLabel, toolIcon, toolLabel, verdictLabel } from '../../lib/labels'
import { formatTimer } from '../../lib/format'
import { parseStream, displayValue, fieldChildren, fieldValue, looseText, plainFields, type StreamNode, type StreamRound } from '../../lib/streamRounds'
import { matchRoundEvents, nodesFromArgs, roundAgentOf, toolResultLine, toolSummary, type ToolEvent } from '../../lib/roundSummary'
import { wireTime } from './events'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Scroll } from '@noverse/ui'
import { EmptyState, Shimmer, Skeleton, Spinner } from '@noverse/ui'
import { ThinkingOrb } from '@noverse/ui'
import { gateReviewItems, gateReviewMeta, gateRoundNote, isCompleted } from './GateCard'
import { writingPercent } from './mobile/logic'
import { IDLE_SUBTITLE, IdleCard, idleModel, type IdleActions } from './IdleCard'
import { s } from './idleCard.stylex'
import { useMediaQuery } from '../../shell/AppShell'
import { useStreamReveal } from '../../lib/streamReveal'
import { streamAgentName, streamAgentOf, streamKindOf } from './streamAgent'
import { useElapsed } from './hooks'
import { StreamMarkdown } from '../../lib/StreamMarkdown'
import { ReviewIssue } from '../../ui/ReviewIssue'
import { CharAvatar } from '../../ui/CharAvatar'

const monoChip: React.CSSProperties = {
  padding: '3px 8px', borderRadius: 6, background: 'var(--color-surface-0)',
  border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--color-fg-4)',
}
const valueColor = (value: string | null) => (value === null || value === 'null' || value === '' ? 'var(--color-fg-4)' : 'var(--color-fg)')
const labelText: React.CSSProperties = { fontSize: 12, color: 'var(--color-fg-4)' }

/** Tint thẻ bước đang chạy theo trợ lý (Main.dc.html › round live): màu orb của agent
 * ở nền 5% viền 25% — xem --kit-acc/--kit-warn/--kit-ok/--kit-info trong tokens. */
const LIVE_TINT: Record<string, { bg: string; bd: string }> = {
  writer: { bg: 'var(--kit-acc-5)', bd: 'var(--kit-acc-25)' },
  editor: { bg: 'var(--kit-warn-5)', bd: 'var(--kit-warn-25)' },
  arbiter: { bg: 'var(--kit-ok-5)', bd: 'var(--kit-ok-25)' },
  architect: { bg: 'var(--kit-info-5)', bd: 'var(--kit-info-25)' },
}

/** Dòng cũ gạch đỏ / dòng mới nền xanh — dùng cho edit_chapter. */
export function DiffInline({ oldText, newText }: { oldText: string | null; newText: string | null }) {
  const line = (text: string, sign: string, tone: string, background: string, strike: boolean) => (
    <div key={`${sign}-${text.slice(0, 12)}`} style={{ display: 'flex', gap: 10, padding: '8px 12px', background }}>
      <span className="mono" aria-label={sign === '−' ? 'Cũ' : 'Mới'} style={{ color: tone }}>{sign}</span>
      <span style={{
        color: strike ? 'var(--color-fg-2)' : 'var(--color-fg)',
        textDecoration: strike ? 'line-through' : undefined,
        textDecorationColor: strike ? 'rgba(255,154,162,0.55)' : undefined,
        whiteSpace: 'pre-wrap',
      }}>{text}</span>
    </div>
  )
  const rows = (text: string | null, sign: string, tone: string, background: string, strike: boolean) =>
    displayValue(text).split('\n').map((part) => line(part, sign, tone, background, strike))
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', borderRadius: 10, overflow: 'hidden',
      border: '1px solid var(--line-2)', fontSize: 13, lineHeight: 1.65,
    }}>
      {rows(oldText, '−', 'var(--color-err)', 'var(--tint-err)', true)}
      {rows(newText, '+', 'var(--color-ok)', 'rgba(166,237,193,0.07)', false)}
    </div>
  )
}

/**
 * Một dòng tóm tắt lời thường cho vòng ✻ — thay cho lưới chip `nhãn giá trị` cũ (ảnh chụp
 * bug 2710: “Chương 2 · Chương 0 · Chương 0”). Câu do `toolSummary` dựng từ tham số thật;
 * chữ dài bị cắt bằng “…” và bản đầy đủ nằm trong `title`.
 */
function RoundSummaryLine({ text }: { text: string }) {
  return (
    <p title={text} style={{
      margin: 0, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5,
      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
    }}>{text}</p>
  )
}

/** Kết quả của lần gọi đã xong: “đã đọc 1.546 chữ” · “2 vấn đề”; lỗi thì câu lỗi màu đỏ. */
function RoundResultLine({ text, failed = false }: { text: string; failed?: boolean }) {
  return (
    <p style={{ margin: 0, fontSize: 12, color: failed ? 'var(--color-err)' : 'var(--color-fg-4)', lineHeight: 1.45 }}>
      <Icon name={failed ? 'close-circle-bulk' : 'tick-circle-bulk'} size={13} style={{ color: failed ? 'var(--color-err)' : 'var(--color-ok)', verticalAlign: '-2px', marginRight: 5 }} />
      {text}
    </p>
  )
}

function FieldGrid({ nodes }: { nodes: StreamNode[] }) {
  const contract = nodes.find((node) => !node.item && node.value === null && node.key === 'contract')
  const all = [...plainFields(nodes), ...(contract ? plainFields(contract.children) : [])]
    .flatMap((row) => {
      const label = fieldLabel(row.key)
      const value = fieldValueLabel(row.key, row.value)
      return label && value !== null ? [{ key: row.key, label, value }] : []
    })
  if (all.length === 0) return null
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {all.map((row) => (
        <div key={row.key} style={{ display: 'flex', gap: 10, alignItems: 'baseline' }}>
          <span style={{ ...labelText, flex: '0 0 108px' }}>{row.label}</span>
          <span style={{ flex: 1, minWidth: 0, fontSize: 13, color: 'var(--color-fg-2)' }}>{translateHost(displayValue(row.value))}</span>
        </div>
      ))}
    </div>
  )
}

const FORESHADOW_TONE: Record<string, { bg: string; fg: string }> = {
  plant: { bg: 'rgba(185,208,247,0.12)', fg: 'var(--color-info)' },
  advance: { bg: 'rgba(246,203,140,0.12)', fg: 'var(--color-warn)' },
  resolve: { bg: 'rgba(166,237,193,0.12)', fg: 'var(--color-ok)' },
}

function CommitBody({ round }: { round: StreamRound }) {
  const castIntros = fieldChildren(round.nodes, 'cast_intros')
  const characters = fieldChildren(round.nodes, 'characters')
  const updates = fieldChildren(round.nodes, 'foreshadow_updates')
  const events = fieldChildren(round.nodes, 'key_events')
  const hook = fieldValueLabel('hook_type', fieldValue(round.nodes, 'hook_type'))
  const feedback = fieldValue(round.nodes, 'feedback')
  const chapter = fieldValueLabel('chapter', fieldValue(round.nodes, 'chapter'))
  const chips: { key: string; label: string; value: string; tone?: 'err' }[] = [
    ...(chapter ? [{ key: 'chapter', label: fieldLabel('chapter')!, value: chapter }] : []),
    { key: 'hook_type', label: fieldLabel('hook_type')!, value: displayValue(hook), tone: hook ? 'err' : undefined },
    { key: 'dominant_strand', label: fieldLabel('dominant_strand')!, value: displayValue(fieldValueLabel('dominant_strand', fieldValue(round.nodes, 'dominant_strand'))) },
    { key: 'feedback', label: fieldLabel('feedback')!, value: feedback === null ? 'chưa có' : displayValue(feedback) },
    {
      key: 'cast_intros',
      label: fieldLabel('cast_intros')!,
      value: castIntros.length === 0 ? '—' : castIntros.map((node) => displayValue(fieldValue(node.children, 'name') ?? node.value)).join(', '),
    },
  ]
  return (
    <>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
        {chips.map((chip) => (
          <span key={chip.key} style={{
            ...monoChip,
            ...(chip.tone === 'err' ? { background: 'rgba(255,154,162,0.08)', borderColor: 'rgba(255,154,162,0.25)' } : {}),
          }}>
            {chip.label} <span style={{ color: chip.tone === 'err' ? 'var(--color-err)' : valueColor(chip.value === 'chưa có' || chip.value === '—' ? null : chip.value) }}>{chip.value}</span>
          </span>
        ))}
      </div>
      {characters.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
          <span style={{ ...labelText, width: '100%' }}>{fieldLabel('characters')}</span>
          {characters.map((node, index) => {
            const name = displayValue(node.value ?? fieldValue(node.children, 'name'))
            return (
              <span key={`${name}-${index}`} style={{
                height: 26, padding: '0 10px 0 4px', display: 'flex', alignItems: 'center', gap: 6,
                borderRadius: 999, background: 'var(--color-surface-4)', border: '1px solid var(--line-3)', fontSize: 12,
              }}>
                <CharAvatar name={name} size="sm" />
                {name}
              </span>
            )
          })}
        </div>
      )}
      {updates.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={labelText}>{fieldLabel('foreshadow_updates')}</span>
          {updates.map((node, index) => {
            const rawAction = displayValue(fieldValue(node.children, 'action') ?? (node.key === 'action' ? node.value : null))
            const description = fieldValue(node.children, 'description')
            const tone = FORESHADOW_TONE[rawAction] ?? { bg: 'var(--color-surface-5)', fg: 'var(--color-fg-3)' }
            return (
              <div key={index} style={{
                display: 'flex', gap: 10, padding: '9px 12px', borderRadius: 10,
                background: 'var(--color-surface-0)', border: '1px solid var(--line-2)',
              }}>
                <span style={{
                  flex: '0 0 auto', height: 22, padding: '0 8px', display: 'flex', alignItems: 'center',
                  borderRadius: 6, background: tone.bg, color: tone.fg, fontSize: 11, fontWeight: 500,
                }}>{foreshadowActionLabel(rawAction)}</span>
                <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span style={{ fontSize: 13, color: description ? 'var(--color-fg-2)' : 'var(--color-fg-4)' }}>{displayValue(description)}</span>
                </div>
              </div>
            )
          })}
        </div>
      )}
      {events.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={labelText}>{fieldLabel('key_events')}</span>
          {events.map((node, index) => (
            <p key={index} style={{
              margin: 0, paddingLeft: 12, borderLeft: '2px solid rgba(201,166,245,0.4)',
              fontSize: 13, color: 'var(--color-fg)', whiteSpace: 'pre-wrap',
            }}>
              {displayValue(node.value)}
              {round.live && index === events.length - 1 && (
                <ThinkingOrb />
              )}
            </p>
          ))}
        </div>
      )}
    </>
  )
}

const VERDICT_TONE: Record<string, { bg: string; fg: string }> = {
  accept: { bg: 'var(--tint-ok)', fg: 'var(--color-ok)' },
  polish: { bg: 'var(--tint-warn)', fg: 'var(--color-warn)' },
  rewrite: { bg: 'var(--tint-err)', fg: 'var(--color-err)' },
}

function ReviewBody({ round }: { round: StreamRound }) {
  const verdict = displayValue(fieldValue(round.nodes, 'verdict'))
  const tone = VERDICT_TONE[verdict] ?? { bg: 'var(--color-surface-5)', fg: 'var(--color-fg-3)' }
  const issues = fieldChildren(round.nodes, 'issues')
  const summary = fieldValue(round.nodes, 'summary')
  return (
    <>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
        <span style={{ height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', borderRadius: 6, background: tone.bg, color: tone.fg, fontSize: 12, fontWeight: 500 }}>{verdict === '—' ? verdict : verdictLabel(verdict)}</span>
        <span style={labelText}>{round.nodes.length > 0 ? t.workspace.issueCount(issues.length) : ''}</span>
      </div>
      {summary && <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-2)' }}>{translateHost(summary)}</p>}
      {issues.map((issue, index) => {
        const severity = displayValue(fieldValue(issue.children, 'severity'))
        const description = fieldValue(issue.children, 'description')
        const suggestion = fieldValue(issue.children, 'suggestion')
        return (
          <div key={index} style={{ padding: '8px 12px', borderRadius: 10, background: 'var(--color-surface-0)', border: '1px solid var(--line-2)', display: 'flex', flexDirection: 'column', gap: 3 }}>
            <span style={{ fontSize: 11, color: severity === 'critical' || severity === 'error' ? 'var(--color-err)' : 'var(--color-warn)' }}>{severity === '—' ? severity : severityLabel(severity)}</span>
            <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{translateHost(displayValue(description))}</span>
            {suggestion && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{translateHost(suggestion)}</span>}
          </div>
        )
      })}
    </>
  )
}

/**
 * Khối “Chi tiết kỹ thuật” — nơi duy nhất hiện tên công cụ và tham số gốc (invariant #7).
 * Người viết truyện không cần thấy: chỉ mở khi chế độ Nâng cao/dev bật (`?tech=1` trên bản
 * dev), nếu không thì ẩn hẳn. Muốn có công tắc thường trực thì cần một mục trong Tuỳ chỉnh.
 */
function TechnicalDetails({ round, dev }: { round: StreamRound; dev: boolean }) {
  if (!dev) return null
  const raw = [round.tool, round.kind === 'text' ? '' : round.body].filter((part) => part.trim() !== '').join('\n')
  if (raw === '') return null
  return (
    <details>
      <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--color-fg-4)' }}>{t.activity.technical}</summary>
      <pre className="mono" style={{ margin: '6px 0 0', fontSize: 12, color: 'var(--color-fg-3)', whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{raw}</pre>
    </details>
  )
}

/** Vòng kiểu `text`: văn bản nháp hiện nguyên; vòng khác chỉ hiện phần tóm tắt, tham số gốc nằm trong chi tiết. */
const textOf = (round: StreamRound): string =>
  round.tool === 'draft_chapter' || round.tool === '' ? round.body : fieldValue(round.nodes, 'summary') ?? ''


/** A successful read tool does not answer the worker's final question. */
const needsAttention = (round: StreamRound): boolean => {
  const text = (round.kind === 'text' ? textOf(round) : looseText(round.nodes)).trim()
  return text.startsWith('Cần Trọng tài:') || /\?[*_`]*$/.test(text)
}

/** Khối văn xuôi: khi là mục cuối và stream đang chạy, lộ từng cụm ký tự
 * (useStreamReveal — nhịp rAF, bắt kịp theo backlog, grapheme-safe); orb bám cuối chữ. */
function ProseStream({ text, live }: { text: string; live: boolean }) {
  const reduced = useMediaQuery('(prefers-reduced-motion: reduce)')
  const shown = useStreamReveal(text, live, reduced)
  const paragraphs = shown.split(/\n{2,}/)
  const head = paragraphs.slice(0, -1).join('\n\n')
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, minWidth: 0, overflowWrap: 'anywhere', fontFamily: 'var(--font-read)', fontSize: 14.5, lineHeight: 1.75, color: 'var(--color-fg-2)' }}>
      <StreamMarkdown text={head} />
      <StreamMarkdown text={paragraphs[paragraphs.length - 1]} />
      {live && <ThinkingOrb />}
    </div>
  )
}

/**
 * Thân thẻ một vòng ✻ (Main.dc.html › round): dòng tóm tắt lời thường trước, phần chi tiết
 * theo loại (diff/commit/kế hoạch/thẩm duyệt) sau, rồi dòng kết quả khi lần gọi đã xong.
 */
function RoundBody({ round, summary, result, failed, attention, dev }: { round: StreamRound; summary: string; result: string; failed: boolean; attention: boolean; dev: boolean }) {
  const body = ((): ReactNode => {
    switch (round.kind) {
      // `kv` (đọc chương/bối cảnh/soát) chỉ còn một dòng tóm tắt — không lưới chip tham số.
      case 'kv': return null
      case 'diff': return (
        <>
          <DiffInline oldText={fieldValue(round.nodes, 'old_string')} newText={fieldValue(round.nodes, 'new_string')} />
          <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
            {t.workspace.editedParagraph(Number(fieldValue(round.nodes, 'chapter')) || null)}
          </div>
        </>
      )
      case 'commit': return <CommitBody round={round} />
      case 'plan': return <FieldGrid nodes={round.nodes} />
      case 'review': return <ReviewBody round={round} />
      case 'text': {
        const text = textOf(round)
        return text === '' ? null : <StreamMarkdown text={text} />
      }
      default: return null
    }
  })()
  const loose = looseText(round.nodes)
  return (
    <div style={{ padding: '2px 14px 12px 38px', display: 'flex', flexDirection: 'column', gap: 10 }}>
      {round.thinking && (
        <p style={{ margin: 0, fontSize: 12.5, fontStyle: 'italic', color: 'var(--color-fg-4)', whiteSpace: 'pre-wrap' }}>{round.thinking}</p>
      )}
      {summary !== '' && <RoundSummaryLine text={summary} />}
      {body}
      {loose !== '' && round.kind !== 'text' && round.kind !== 'raw' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10, minWidth: 0, overflowWrap: 'anywhere', fontFamily: 'var(--font-read)', fontSize: 14, lineHeight: 1.7, color: 'var(--color-fg-2)' }}>
          <StreamMarkdown text={loose} />
        </div>
      )}
      {attention
        ? <p role="status" style={{ margin: 0, fontSize: 12, color: 'var(--color-warn)', lineHeight: 1.45 }}><Icon name="warning-2" size={13} style={{ verticalAlign: '-2px', marginRight: 5 }} />{t.stream.attention}</p>
        : result !== '' && <RoundResultLine text={result} failed={failed} />}
      <TechnicalDetails round={round} dev={dev} />
    </div>
  )
}

/** Dải xanh tóm tắt lần chấm trước + thẻ cảnh báo còn lại + lối mở Trình đọc (Main.dc.html › gateReview). */
export function GateBody({ review, chapter, loading, onOpenReader, quality = false }: { review: ReviewEntry | null; chapter: number; loading: boolean; onOpenReader?: () => void; quality?: boolean }) {
  const { route } = useRoute()
  const note = gateRoundNote(review)
  const { errors, warnings } = gateReviewItems(review)
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '2px 0' }}>
      {loading ? (
        <>
          <Skeleton height={44} radius={10} />
          <Skeleton height={58} radius={10} />
        </>
      ) : (
        <>
      {note !== '' && (
        <div style={{
          display: 'flex', gap: 10, alignItems: 'flex-start', padding: '9px 12px', borderRadius: 10,
          background: 'var(--kit-ok-6)', border: '1px solid var(--kit-ok-16)',
        }}>
          <Icon name="tick-circle-bulk" size={15} style={{ color: 'var(--color-ok)', marginTop: 2, flex: 'none' }} />
          <span style={{ fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5 }}>{note}</span>
        </div>
      )}
      {[...errors, ...warnings].map((item) => <ReviewIssue key={item.id} item={item} variant="gate" />)}
      {review === null && <EmptyState icon="shield-tick" title={t.workspace.gateNoReview} />}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12, color: 'var(--color-fg-4)', flexWrap: 'wrap' }}>
        <span style={{ flex: 1, minWidth: 0 }}>{t.workspace.gateFooter(chapter + 1)}</span>
        {onOpenReader && (
          <button type="button" onClick={onOpenReader} style={{
            border: 'none', background: 'transparent', padding: 0, cursor: 'pointer',
            fontSize: 12.5, color: 'var(--color-accent)', whiteSpace: 'nowrap',
          }}>{t.workspace.openInReader}</button>
        )}
        {quality && route.name === 'workspace' && route.project && (
          <a href={`#${paths.quality(route.project, route.story)}`} style={{ fontSize: 12.5, color: 'var(--color-fg-3)', whiteSpace: 'nowrap' }}>{t.quality.gateLink}</a>
        )}
      </div>
        </>
      )}
    </div>
  )
}

export function LiveStream({ streamText, isRunning, snapshot, events = [], mobile = false, loading = !snapshot, gate = false, review = null, reviewLoading = false, gateChapter = 0, onOpenReader, idleActions }: {
  streamText: string
  isRunning: boolean
  snapshot: Snapshot | null
  /** Dòng Nhật ký của các lần gọi công cụ — ghép với vòng ✻ để hiện kết quả (“đã đọc 1.546 chữ”). */
  events?: ToolEvent[]
  mobile?: boolean
  loading?: boolean
  /** Main.dc.html › phase=gate: khung “Thẩm duyệt chương N” thay luồng viết */
  gate?: boolean
  review?: ReviewEntry | null
  /** đang tải thẩm duyệt của chương chờ duyệt */
  reviewLoading?: boolean
  gateChapter?: number
  onOpenReader?: () => void
  /** Gói P · P4: nút của thẻ trạng thái rảnh (dựng ở WorkspaceScreen). */
  idleActions?: IdleActions
}) {
  const parsed = useMemo(() => parseStream(streamText, isRunning), [streamText, isRunning])
  const roundEvents = useMemo(() => matchRoundEvents(parsed.rounds, events), [parsed.rounds, events])
  // Nâng cao/dev: “Chi tiết kỹ thuật” mặc định ẩn (xem RoundSummaryLine); bản dev mở bằng ?tech=1.
  const [dev] = useState(() => import.meta.env.DEV && /[?&]tech=1(?:&|$)/.test(window.location.hash))
  const [overrides, setOverrides] = useState<Record<string, boolean>>({})
  // Gói O · mục 6–7: trợ lý đang chạy (không cứng writer) quyết định tên, orb và màu thẻ bước.
  const lastRound = parsed.rounds.at(-1)
  const streamAgent = streamAgentOf(snapshot?.agents, parsed.liveTool)
  const streamKind = streamKindOf(snapshot, streamAgent)
  const chapter = snapshot?.inProgressChapter || snapshot?.currentChapter || 0
  const liveTool = parsed.liveTool || (isRunning ? streamAgent?.tool ?? '' : '')
  const liveVariant = lastRound?.live && lastRound.tool === 'save_foundation' ? fieldValue(lastRound.nodes, 'type') ?? '' : ''
  const elapsedSec = Math.floor(useElapsed(isRunning, liveTool || 'stream'))
  const agentName = streamAgentName(streamAgent, streamKind)
  const progress = gate ? 100 : isRunning || parsed.items.length > 0 ? writingPercent(snapshot?.flow) : 0
  // Gói P · P4: khi không có trợ lý nào chạy, khung hiện thẻ ở giữa thay dòng trống.
  const idle = gate || !snapshot ? null : idleModel(snapshot)
  const meta = gate
    ? gateReviewMeta(review)
    : idle
      ? IDLE_SUBTITLE[idle.phase]
      : isRunning
        ? [agentName, (streamKind === 'writer' || streamKind === 'editor') && chapter > 0 ? t.workspace.chapter(chapter) : ''].filter(Boolean).join(' · ')
        : isCompleted(snapshot) ? '' : t.workspace.waiting
  const hasContent = parsed.items.length > 0
  const title = gate ? t.workspace.gateReviewTitle(review?.chapter || gateChapter) : t.workspace.streamTitle
  // Mục 9: chưa có văn đang viết (lập kế hoạch…) → khung thu về nội dung, không để khoảng trống.
  const compact = !gate && !idle && parsed.prose === '' && parsed.items.length === 0

  return (
    <section className="card" aria-label={title} style={{
      flex: compact ? '0 0 auto' : mobile ? '1 1 0' : idle ? '5 1 0' : '6 1 0',
      minHeight: 0, display: 'flex', flexDirection: 'column',
      borderRadius: 14, overflow: 'hidden',
    }}>
      <div style={{ flex: '0 0 2px', height: 2, background: 'var(--color-surface-5)' }}>
        {/* artboard: thẻ trạng thái rảnh và khung thẩm duyệt đã xong → vạch tiến độ 0% */}
        <div style={{ width: `${idle ? 0 : progress}%`, height: '100%', background: gate ? 'var(--color-ok)' : 'var(--grad-writer-progress)', transition: 'width .3s var(--ease-out)' }} />
      </div>
      <div style={{ flex: 1, minHeight: 0, padding: '8px 14px 0 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div style={{ flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '6px 10px' }}>
          {gate
            ? <AgentOrb agent="editor" size={16} />
            : <AgentOrb agent={streamKind} size={16} live={isRunning} dim={!isRunning} />}
          <span style={{ fontSize: 13, fontWeight: 500 }}>{title}</span>
          {meta !== '' && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{meta}</span>}
          <span style={{ flex: 1 }} />
          {isRunning && (
            <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <svg width="22" height="12" viewBox="0 0 26 14" fill="none" aria-hidden="true">
                <defs>
                  <linearGradient id="live-infinity" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0" stopColor="var(--color-architect)" />
                    <stop offset="0.5" stopColor="#C9457F" />
                    <stop offset="1" stopColor="var(--color-user)" />
                  </linearGradient>
                </defs>
                <path d="M13 7C11 4.2 9 2.5 6.5 2.5a4.5 4.5 0 0 0 0 9C9 11.5 11 9.8 13 7s4-4.5 6.5-4.5a4.5 4.5 0 0 1 0 9C17 11.5 15 9.8 13 7z" stroke="var(--line-5)" strokeWidth="1.6" />
                <path pathLength="50" d="M13 7C11 4.2 9 2.5 6.5 2.5a4.5 4.5 0 0 0 0 9C9 11.5 11 9.8 13 7s4-4.5 6.5-4.5a4.5 4.5 0 0 1 0 9C17 11.5 15 9.8 13 7z"
                  stroke="url(#live-infinity)" strokeWidth="1.8" strokeLinecap="round"
                  style={{ strokeDasharray: '14 36', animation: 'ai-infinity 1.8s linear infinite' }} />
              </svg>
              <Shimmer style={{ fontSize: 12 }}>{liveTool ? activeToolLabel(liveTool, liveVariant) : '…'}</Shimmer>
              <span className="num" role="timer" aria-label={`Đã chạy ${elapsedSec} giây`} style={{
                display: 'inline-block', minWidth: '5ch', textAlign: 'right', fontSize: 12, color: 'var(--color-fg-4)',
              }}>{formatTimer(elapsedSec * 1000)}</span>
            </span>
          )}
        </div>
        {idle && idleActions && snapshot
          ? (
            <div className={stylex.props(s.center).className}>
              <IdleCard model={idle} snapshot={snapshot} actions={idleActions} mobile={mobile} />
            </div>
          )
          : (
        <Scroll stickToBottom style={{ padding: '0 4px 10px 0' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {gate ? (
              <GateBody review={review} chapter={gateChapter || (review?.chapter ?? 0)} loading={reviewLoading} onOpenReader={onOpenReader} quality={!mobile} />
            ) : (
              <>
            {loading && !snapshot ? [0, 1, 2, 3].map((row) => <Skeleton key={row} height={15} width={`${94 - row * 9}%`} />)
              : !hasContent && (isRunning
                ? <p style={{ margin: 0, fontSize: 12.5, color: 'var(--color-fg-4)' }}>{`${agentName} ${activeToolDoing(liveTool, liveVariant)} · ${t.stream.stepsHint}`}</p>
                : <EmptyState icon="magic-star" title={t.empty.stream} />)}
            {parsed.items.map((item, index) => {
              if (item.kind === 'thinking') {
                return (
                  <p key={`thinking-${index}`} style={{ margin: 0, fontSize: 12.5, fontStyle: 'italic', color: 'var(--color-fg-4)', whiteSpace: 'pre-wrap' }}>
                    {item.text}
                  </p>
                )
              }
              if (item.kind === 'prose') {
                return <ProseStream key={`prose-${index}`} text={item.text} live={isRunning && index === parsed.items.length - 1} />
              }
              const round = item.round
              const open = overrides[round.id] ?? round.defaultOpen
              // Tiêu đề thẻ = “<Bước> · <Trợ lý>” (Task bug 2710): bước là nhãn lời thường,
              // trợ lý lấy từ dòng Nhật ký của đúng lần gọi (công cụ dùng chung) hoặc bảng công cụ riêng.
              const stepLabel = round.tool ? toolLabel(round.tool, round.tool === 'save_foundation' ? fieldValue(round.nodes, 'type') ?? '' : '') : round.labelVi
              const matched = roundEvents.get(round.id)
              const assistant = agentLabel(matched?.agent || roundAgentOf(round.tool))
              const summary = toolSummary(round.tool, round.nodes.length > 0 ? round.nodes : nodesFromArgs(matched?.args), snapshot?.totalChapters)
              const toolDone = matched != null && (matched.status === 'ok' || matched.status === 'error' || wireTime(matched.finishedAt) !== '')
              const failed = toolDone && (matched?.failed === true || matched?.level === 'error')
              const attention = needsAttention(round)
              const result = toolDone ? toolResultLine(matched) : ''
              const liveTint = LIVE_TINT[streamKind] ?? { bg: 'var(--kit-wash-5)', bd: 'var(--kit-wash-25)' }
              return (
                <div key={round.id} style={{
                  borderRadius: 12,
                  background: round.live ? liveTint.bg : 'var(--color-surface-0)',
                  border: `1px solid ${round.live ? liveTint.bd : 'var(--kit-wash-5)'}`,
                }}>
                  <button type="button" className="hov-ghost" aria-expanded={open}
                    onClick={() => setOverrides((previous) => ({ ...previous, [round.id]: !open }))}
                    style={{
                      width: '100%', display: 'flex', alignItems: 'center', gap: 10, minHeight: 38,
                      padding: '0 12px', border: 'none', borderRadius: 12, background: 'transparent',
                      color: 'var(--color-fg)', textAlign: 'left',
                    }}>
                    <Icon name={round.tool === 'check_consistency' ? 'shield-tick-bulk' : round.tool ? toolIcon(round.tool) : 'magic-star-bulk'} size={15} style={{ color: 'var(--color-round-label)' }} />
                    <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-fg)' }}>{stepLabel}</span>
                    {assistant !== '' && (
                      <span style={{ fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap' }}>{`· ${assistant}`}</span>
                    )}
                    <span aria-hidden="true" style={{ flex: 1, height: 1, background: 'var(--line-2)' }} />
                    {attention
                      ? <Icon name="warning-2" size={15} label={t.stream.attention} style={{ color: 'var(--color-warn)' }} />
                      : round.live && !toolDone
                      ? <Spinner />
                      : failed
                        ? <Icon name="close-circle-bulk" size={15} label={t.stream.failed} style={{ color: 'var(--color-err)' }} />
                        : <Icon name="tick-circle-bulk" size={15} label={t.workspace.done} style={{ color: 'var(--color-ok)' }} />}
                    <Icon name="arrow-down-2" size={14} style={{
                      color: 'var(--color-fg-4)',
                      transform: `rotate(${open ? 180 : 0}deg)`,
                      transition: 'transform .15s var(--ease-out)',
                    }} />
                  </button>
                  {open && <RoundBody round={round} summary={summary} result={result} failed={failed} attention={attention} dev={dev} />}
                </div>
              )
            })}
              </>
            )}
          </div>
        </Scroll>
        )}
      </div>
    </section>
  )
}
