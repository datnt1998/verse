// LiveStream — luồng viết trực tiếp và các thẻ thao tác dùng chung với lịch sử.
import { useMemo } from 'react'
import type { ReviewEntry, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { paths, useRoute } from '../../router'
import { fieldValue, parseStream } from '../../lib/streamRounds'
import { formatTimer } from '../../lib/format'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState, Shimmer, Skeleton } from '@noverse/ui'
import { gateReviewItems, gateReviewMeta, gateRoundNote, isCompleted } from './GateCard'
import { writingPercent } from './mobile/logic'
import { IDLE_SUBTITLE, IdleCard, idleModel, type IdleActions } from './IdleCard'
import { streamAgentName, streamAgentOf, streamKindOf } from './streamAgent'
import { useElapsed } from './hooks'
import { ReviewIssue } from '../../ui/ReviewIssue'
import { runCardModel } from '../../lib/runCardModel'
import { RunHistory } from './RunHistory'
import { ProviderStopCard } from './ProviderStopCard'


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

export function LiveStream({ project, story, streamText, isRunning, snapshot, mobile = false, gate = false, review = null, reviewLoading = false, gateChapter = 0, onOpenReader, idleActions, onResume }: {
  project: string
  story: string
  streamText: string
  isRunning: boolean
  snapshot: Snapshot | null
  mobile?: boolean
  /** Main.dc.html › phase=gate: khung “Thẩm duyệt chương N” thay luồng viết */
  gate?: boolean
  review?: ReviewEntry | null
  /** đang tải thẩm duyệt của chương chờ duyệt */
  reviewLoading?: boolean
  gateChapter?: number
  onOpenReader?: () => void
  idleActions?: IdleActions
  /** Gói AU — thẻ “việc viết đã dừng” chuyển nhà cung cấp rồi viết tiếp. */
  onResume?: () => void
}) {
  const parsed = useMemo(() => parseStream(streamText, isRunning), [streamText, isRunning])
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
  const title = gate ? t.workspace.gateReviewTitle(review?.chapter || gateChapter) : t.workspace.streamTitle

  return (
    <section className="card" aria-label={title} style={{
      flex: mobile ? '1 1 0' : '6 1 0',
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
            <span role="status" style={{
              display: 'flex', alignItems: 'center', gap: 8, height: 26, padding: '0 10px 0 8px',
              borderRadius: 999, background: 'var(--kit-acc-7)', border: '1px solid var(--kit-acc-20)',
            }}>
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
              <Shimmer style={{ fontSize: 12 }}>{liveTool ? `Đang ${runCardModel(liveTool, JSON.stringify({ type: liveVariant }), '').name.toLocaleLowerCase('vi')}` : '…'}</Shimmer>
              <span className="num" role="timer" aria-label={`Đã chạy ${elapsedSec} giây`} style={{
                display: 'inline-block', minWidth: '5ch', textAlign: 'right', fontSize: 12, color: 'var(--color-fg-4)',
              }}>{formatTimer(elapsedSec * 1000)}</span>
            </span>
          )}
        </div>
        {!gate && snapshot && <ProviderStopCard snapshot={snapshot} isRunning={isRunning} mobile={mobile} onResume={onResume} />}
        {gate
          ? <GateBody review={review} chapter={gateChapter || (review?.chapter ?? 0)} loading={reviewLoading} onOpenReader={onOpenReader} quality={!mobile} />
          : <RunHistory project={project} story={story} mobile={mobile} embedded empty={idle && idleActions && snapshot ? <IdleCard model={idle} snapshot={snapshot} actions={idleActions} mobile={mobile} /> : undefined} />}
      </div>
    </section>
  )
}
