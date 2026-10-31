// Trợ lý nhận ra — khối ở Kênh › Video › Đã đăng (tối đa 3 InsightRow) và thẻ mời
// xem ở bộ lọc Tất cả. Chữ/khoảng cách theo artboard Channel.dc.html.
import { useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, toast } from '@noverse/ui'
import { ApiError } from '../../api/request'
import { applyInsight, getPerformance, type Insight, type Performance } from '../../api/performance'
import { keys } from '../../data/keys'
import { useResource, type Resource } from '../../data/useResource'
import { performanceCopy, viewsShort } from '../../i18n/performance'
import { Orb } from '../Orb'
import { barWidths, teaserEvidence } from './performance-logic'
import { performanceStyles as sx } from './performance.stylex'

const copy = performanceCopy.insights

/** Số liệu sau đăng của một kênh; chỉ gọi khi màn cần (hàng Đã đăng, khối, thẻ mời). */
export function usePerformance(channelId: string): Resource<Performance> {
  return useResource(keys.channelPerformance(channelId), ({ signal }) => getPerformance(channelId, signal), { keepPrevious: true })
}

const isOffline = (error: unknown) => error instanceof ApiError && error.status === 0

function Head({ children }: { children?: ReactNode }) {
  return (
    <div {...stylex.props(sx.head)}>
      <Orb of="scout" size={18} />
      <b {...stylex.props(sx.title)}>{copy.title}</b>
      {children}
    </div>
  )
}

function BarRow({ label, value, width, top }: { label: string; value: number; width: number; top: boolean }) {
  return (
    <div {...stylex.props(sx.bar)}>
      <span title={label} {...stylex.props(sx.barLabel)}>{label}</span>
      <span {...stylex.props(sx.track)} aria-hidden="true"><i {...stylex.props(sx.fill, top && sx.fillTop, sx.fillWidth(width))} /></span>
      <b {...stylex.props(sx.value)}>{viewsShort(value)}</b>
    </div>
  )
}

function InsightRow({ insight, channelId, onApplied }: { insight: Insight; channelId: string; onApplied: () => void }) {
  const [state, setState] = useState<'idle' | 'pending' | 'done'>(insight.applied ? 'done' : 'idle')
  const widths = barWidths(insight.groups.map((group) => group.median7d))
  const actionable = insight.apply?.op

  async function apply() {
    setState('pending')
    try {
      const result = await applyInsight(channelId, insight.id)
      toast.ok(result.kind === 'rule' ? copy.appliedRule : copy.appliedRotation)
      setState('done')
      onApplied()
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) { setState('done'); onApplied(); return }
      setState('idle')
      if (error instanceof ApiError && error.status === 404) { toast.err(copy.gone); onApplied(); return }
      toast.err(error instanceof Error ? error.message : performanceCopy.mark.failed)
    }
  }

  return (
    <div {...stylex.props(sx.row)}>
      <div title={insight.text} {...stylex.props(sx.text)}>{insight.text}</div>
      {state === 'done'
        ? <span role="status" {...stylex.props(sx.applied)}>{copy.applied}</span>
        : actionable && <Button size="sm" variant="ghost" pending={state === 'pending'} pendingLabel={copy.applying} onClick={() => void apply()}>{copy.apply[actionable]}</Button>}
      <div {...stylex.props(sx.bars)}>
        {insight.groups.map((group, index) => (
          <BarRow key={group.label} label={copy.bar(group.label, group.n, index === 0)} value={group.median7d} width={widths[index] ?? 0} top={index === 0} />
        ))}
      </div>
    </div>
  )
}

function Note({ title, sub, error, retry }: { title: string; sub?: string; error?: boolean; retry?: () => void }) {
  return (
    <div {...stylex.props(sx.note)} role={error ? 'alert' : 'status'}>
      <div {...stylex.props(sx.noteRow)}>
        <span {...stylex.props(error ? sx.noteErr : sx.noteTitle)}>{title}</span>
        {retry && <Button size="sm" variant="ghost" onClick={retry}>{copy.retry}</Button>}
      </div>
      {sub && <span {...stylex.props(sx.noteSub)}>{sub}</span>}
    </div>
  )
}

export function PostedInsights({ channelId }: { channelId: string }) {
  const perf = usePerformance(channelId)
  const data = perf.data
  const refresh = () => void perf.refresh()
  const insights = (data?.insights ?? []).slice(0, 3)
  const preliminary = insights.some((item) => item.preliminary)

  return (
    <section aria-label={copy.title} {...stylex.props(sx.card)}>
      <Head>
        {data && insights.length > 0 && preliminary && <span {...stylex.props(sx.tag)}>{copy.prelim(data.counted)}</span>}
        <span {...stylex.props(sx.grow)} />
        {insights.length > 0 && <span {...stylex.props(sx.help)}>{copy.help}</span>}
      </Head>
      {data === undefined && !perf.error && (
        <div {...stylex.props(sx.note)} role="status" aria-label={copy.loading}>
          <span {...stylex.props(sx.skel, sx.skelWidth(62))} />
          <span {...stylex.props(sx.skel, sx.skelWidth(38))} />
        </div>
      )}
      {data === undefined && perf.error != null && (isOffline(perf.error)
        ? <Note error title={copy.offline} retry={refresh} />
        : <Note error title={copy.loadFail} retry={refresh} />)}
      {data && insights.map((insight) => <InsightRow key={insight.id} insight={insight} channelId={channelId} onApplied={refresh} />)}
      {data && insights.length === 0 && (data.needMore > 0
        ? <Note title={copy.needMore(data.needMore)} sub={copy.needMoreSub} />
        : <Note title={copy.nothing} />)}
      {data !== undefined && perf.error != null && <Note error title={isOffline(perf.error) ? copy.offline : copy.loadFail} retry={refresh} />}
    </section>
  )
}

/** Thẻ mời xem ở bộ lọc Tất cả — chỉ hiện khi đã có nhận xét. */
export function InsightsTeaser({ channelId, onOpen }: { channelId: string; onOpen: () => void }) {
  const perf = usePerformance(channelId)
  const insights = (perf.data?.insights ?? []).slice(0, 3)
  if (!perf.data || insights.length === 0) return null
  const evidence = teaserEvidence(insights)
  return (
    <button type="button" onClick={onOpen} {...stylex.props(sx.teaser)}>
      <Orb of="scout" size={18} />
      <span {...stylex.props(sx.teaserBody)}>
        <span {...stylex.props(sx.teaserTitle)}>{copy.teaser(insights.length, perf.data.counted)}</span>
        <span title={evidence} {...stylex.props(sx.teaserEv)}>{evidence}</span>
      </span>
      <span {...stylex.props(sx.teaserGo)}>{copy.teaserOpen}</span>
    </button>
  )
}
