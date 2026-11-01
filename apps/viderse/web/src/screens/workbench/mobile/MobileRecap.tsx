// Bản tin toàn màn hình của điện thoại (artboard MMain › screen=recap): trang
// điện thoại riêng (thanh tab ẩn), dữ liệu /api/recap thật + đề xuất thật, và
// ô nhắn trợ lý mở sheet Kế hoạch.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Sheet } from '@noverse/ui'
import type { RecapData } from '../../../api/workbench'
import type { ChannelSummary } from '../../../api/workbenchChannels'
import { SuggestionCard } from '../../../components/suggestions/SuggestionCard'
import { useSuggestions } from '../../../components/suggestions/useSuggestions'
import { RecapAutoLine } from '../../../components/suggestions/RecapAuto'
import { screenWorkbench } from '../../../i18n/screens'
import { navigate, paths } from '../../../router'
import { recapLead } from '../recap/recap-logic'
import { recapWindow } from './mworkbench-logic'
import { mw } from './mworkbench.stylex'

const c = screenWorkbench.mobile
const r = c.recap

export function MobileRecap({ data, loading, error, channels, onRetry, onClose, onSelectJob, onPlan }: {
  data: RecapData | undefined
  loading: boolean
  error: unknown
  channels: ChannelSummary[]
  onRetry: () => void
  onClose: () => void
  onSelectJob: (id: string) => void
  onPlan: (text: string) => void
}) {
  const suggestions = useSuggestions('all')
  const [draft, setDraft] = useState('')
  const ready = channels.reduce((sum, channel) => sum + channel.ready, 0)
  const waiting = channels.reduce((sum, channel) => sum + channel.waitingYou, 0)
  const review = data?.needsYou?.filter((item) => item.state === 'review').length ?? 0

  return <Sheet open onClose={onClose} label={c.recapTitle}
    size="md" height="100dvh" grab={false} homeIndicator xstyle={mw.recapSheet}>
  <div {...stylex.props(mw.recapRoot)}>
    <header {...stylex.props(mw.recapHeader)}>
      <h2 {...stylex.props(mw.recapHeading)}>{c.recapTitle}</h2>
      {data && <p {...stylex.props(mw.recapSubtitle)}>{recapWindow(data.from, data.to)}</p>}
    </header>
    <div {...stylex.props(mw.backRow)}>
      <Button size="sm" variant="quiet" onClick={onClose} aria-label={`Quay lại ${c.back}`}>‹ {c.back}</Button>
    </div>

    <p {...stylex.props(mw.recapLead)}>{recapLead(waiting, ready)}</p>

    {loading && !data && <p role="status" {...stylex.props(mw.notice)}>{r.loading}</p>}
    {error != null && !data && <div role="alert" {...stylex.props(mw.error)}>{r.error} <Button size="sm" variant="quiet" onClick={onRetry}>{c.retry}</Button></div>}

    {data && <>
      <section aria-label={r.needsYou(0)} {...stylex.props(mw.section)}>
        <div {...stylex.props(mw.sectionHead)}><h2 {...stylex.props(mw.sectionTitle)}>{r.needsYou(data.needsYou?.length ?? 0)}</h2></div>
        {data.needsYou?.length
          ? data.needsYou.map((item) => <button key={item.jobId} type="button" {...stylex.props(mw.recapRow)} onClick={() => onSelectJob(item.jobId)}>
            <span {...stylex.props(mw.recapRowText)} title={item.label}>{item.label}</span>
            <span {...stylex.props(mw.recapRowState)}>{item.state === 'review' ? 'Chờ duyệt' : 'Cần xử lý'} ›</span>
          </button>)
          : <p {...stylex.props(mw.notice)}>{r.needsEmpty}</p>}
        {review > 0 && <Button size="lg" variant="primary" onClick={() => { onClose(); navigate(paths.review()) }}>{r.reviewAll(review)}</Button>}
      </section>

      {Boolean(data.auto?.length) && <section aria-label={r.auto} {...stylex.props(mw.section)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{r.auto}</h2>
        {data.auto?.map((item, index) => <RecapAutoLine key={`${item.kind}-${index}`} item={item} />)}
      </section>}

      <section aria-label={c.suggestions} {...stylex.props(mw.section)}>
        <div {...stylex.props(mw.sectionHead)}>
          <h2 {...stylex.props(mw.sectionTitle)}>{c.suggestions}</h2>
          <span {...stylex.props(mw.sectionCount)}>{suggestions.list.data?.total ?? 0}</span>
        </div>
        {suggestions.list.initialLoading ? <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>
          : suggestions.list.error != null && !suggestions.list.data ? <div role="alert" {...stylex.props(mw.notice)}>{c.suggestionsError} <Button size="sm" variant="quiet" onClick={() => void suggestions.list.refresh()}>{c.retry}</Button></div>
          : !suggestions.list.data?.suggestions.length ? <p {...stylex.props(mw.notice)}>{c.suggestionsEmpty}</p>
          : <div {...stylex.props(mw.section)}>{suggestions.list.data.suggestions.map((suggestion) => <SuggestionCard key={suggestion.id} suggestion={suggestion} busy={suggestions.busy === suggestion.id} offline={!suggestions.online}
            notice={suggestions.notice?.id === suggestion.id ? suggestions.notice.text : undefined}
            onAccept={() => void suggestions.accept(suggestion)} onDismiss={() => void suggestions.dismiss(suggestion)} onNever={() => void suggestions.dismiss(suggestion, true)} />)}</div>}
      </section>

      <section aria-label={r.today} {...stylex.props(mw.section)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{r.today}</h2>
        <div {...stylex.props(mw.tiles)}>
          <div {...stylex.props(mw.tile)}><span {...stylex.props(mw.tileValue)}>{data.today.queued}</span><span {...stylex.props(mw.tileLabel)}>{r.queued}</span></div>
          <div {...stylex.props(mw.tile)}><span {...stylex.props(mw.tileValue)}>{data.today.expectedDone ?? '—'}</span><span {...stylex.props(mw.tileLabel)}>{r.expectedDone}</span></div>
          <div {...stylex.props(mw.tile)}><span {...stylex.props(mw.tileValue)}>{data.today.byTime ?? '—'}</span><span {...stylex.props(mw.tileLabel)}>{r.byTime}</span></div>
          <div {...stylex.props(mw.tile)}><span {...stylex.props(mw.tileValue)}>{data.today.imagesLeft == null ? '—' : `${data.today.imagesLeft}/${data.today.imagesTotal ?? '—'}`}</span><span {...stylex.props(mw.tileLabel)}>{r.imagesLeft}</span></div>
        </div>
      </section>

      {Boolean(data.channels?.length) && <section aria-label={r.channels} {...stylex.props(mw.section)}>
        <h2 {...stylex.props(mw.sectionTitle)}>{r.channels}</h2>
        {data.channels?.map((line) => {
          const name = channels.find((channel) => channel.channelId === line.channelId)?.name
          return name ? <div key={line.channelId} {...stylex.props(mw.recapRow)}>
            <span {...stylex.props(mw.recapRowText)}>{name}</span>
            <span {...stylex.props(mw.recapRowState)}>{r.channelLine(line.done, line.cap, line.waiting, line.ready)}</span>
          </div> : null
        })}
      </section>}
    </>}

    <form {...stylex.props(mw.composerSheet)} onSubmit={(event) => { event.preventDefault(); const text = draft.trim(); if (text) onPlan(text) }}>
      <input {...stylex.props(mw.composerInput)} aria-label={c.composerRecap} value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={c.composerRecap} />
      <Button type="submit" size="lg" variant="primary" disabled={!draft.trim()}>{c.send}</Button>
    </form>
  </div>
  </Sheet>
}
