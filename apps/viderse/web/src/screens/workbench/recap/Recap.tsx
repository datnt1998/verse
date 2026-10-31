import * as stylex from '@stylexjs/stylex'
import { space as s, corners as r } from '../../../styles/viderse.stylex'
import type { RecapData } from '../../../api/workbench'
import { AssistantComposer } from '../../../components/assistant/AssistantComposer'
import { assistantCopy } from '../../../i18n/assistant'
import { RecapAutoLine } from '../../../components/suggestions/RecapAuto'
import { RecapSuggestions } from '../../../components/suggestions/SuggestionPanels'
import { useOnline } from '../../../data/status'

type Props = {
  data: RecapData | undefined
  loading: boolean
  error: unknown
  waitingCount: number
  readyCount: number
  channelNames: Record<string, string>
  onRetry: () => void
  onEnter: () => void
  onSelectJob: (id: string) => void
}

export function shouldAutoOpenRecap(from: string, to: string, hasNew: boolean, enabled: boolean): boolean {
  const start = new Date(from)
  const end = new Date(to)
  if (!hasNew || !enabled || !Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) return false
  const five = new Date(end)
  five.setHours(5, 0, 0, 0)
  return end.getTime() - start.getTime() >= 3 * 60 * 60 * 1000 || (end >= five && start < five)
}

export function recapLead(waiting: number, ready: number): string {
  return `Trong lúc bạn vắng: ${ready} video sẵn sàng đăng, ${waiting} video chờ bạn.`
}

const time = (value: string) => {
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : '—'
}

export function Recap({ data, loading, error, waitingCount, readyCount, channelNames, onRetry, onEnter, onSelectJob }: Props) {
  const online = useOnline()
  return <div {...stylex.props(styles.layout)}><div role="region" aria-label="Bản tin" {...stylex.props(styles.scroll)}><div {...stylex.props(styles.root)}>
    <header {...stylex.props(styles.header)}><div {...stylex.props(styles.heading)}><span {...stylex.props(styles.when)}>Bản tin · mọi kênh{data && ` · từ ${time(data.from)} tới ${time(data.to)}`}</span>{data && <p {...stylex.props(styles.lead)}>{recapLead(waitingCount, readyCount)}</p>}</div><button type="button" {...stylex.props(styles.quiet)} onClick={onEnter}>Vào việc ›</button></header>
    {loading ? <p role="status">Đang tải Bản tin…</p> : !data && error ? <p role="alert">Không tải được Bản tin. <button type="button" onClick={onRetry}>Thử lại</button></p> : data && <>
      <section aria-label="Cần bạn" {...stylex.props(styles.section)}><span {...stylex.props(styles.group)}>Cần bạn · {data.needsYou?.length || 0}</span><div {...stylex.props(styles.card)}>{data.needsYou?.length ? data.needsYou.map(item => <button key={item.jobId} type="button" {...stylex.props(styles.row)} onClick={() => onSelectJob(item.jobId)}><span {...stylex.props(styles.rowText)} title={item.label}>{item.label}</span><span>{item.state === 'review' ? 'Chờ duyệt' : 'Cần xử lý'} ›</span></button>) : <div {...stylex.props(styles.empty)}>Không có video cần bạn.</div>}</div>{data.needsYou?.some(item => item.state === 'review') && <a href="#/review" onClick={onEnter} {...stylex.props(styles.primary)}>Duyệt {data.needsYou.filter(item => item.state === 'review').length} video</a>}</section>
      {!!data.auto?.length && <section aria-label="Máy đã tự làm" {...stylex.props(styles.section)}><span {...stylex.props(styles.group)}>Máy đã tự làm</span>{data.auto.map((item, index) => <RecapAutoLine key={`${item.kind}-${index}`} item={item} offline={!online} />)}</section>}
      <RecapSuggestions />
      <section aria-label="Hôm nay" {...stylex.props(styles.section)}><span {...stylex.props(styles.group)}>Hôm nay</span><div {...stylex.props(styles.tiles)}><div {...stylex.props(styles.tile)}><b>{data.today.queued}</b><span>Chờ lượt</span></div><div {...stylex.props(styles.tile)}><b>{data.today.expectedDone ?? '—'}</b><span>Dự kiến xong</span></div><div {...stylex.props(styles.tile)}><b>{data.today.byTime ?? '—'}</b><span>Dự kiến lúc</span></div><div {...stylex.props(styles.tile)}><b>{data.today.imagesLeft == null ? '—' : `${data.today.imagesLeft}/${data.today.imagesTotal ?? '—'}`}</b><span>Ảnh còn lại</span></div></div></section>
      {!!data.channels?.length && <section aria-label="Theo kênh" {...stylex.props(styles.section)}><span {...stylex.props(styles.group)}>Theo kênh</span>{data.channels.filter(channel => channelNames[channel.channelId]).map(channel => <div key={channel.channelId} {...stylex.props(styles.line)}><span {...stylex.props(styles.rowText)}>{channelNames[channel.channelId]}</span><span>{channel.done}/{channel.cap} · {channel.waiting} chờ bạn · {channel.ready} sẵn sàng đăng</span></div>)}</section>}
    </>}
  </div></div>
  <div {...stylex.props(styles.composerFoot)}>
    <AssistantComposer scope="all" label={assistantCopy.composer.recap} placeholder={assistantCopy.composer.recapPlaceholder} hints={{ esc: false }} onSettled={onRetry} />
  </div></div>
}

const styles = stylex.create({
  layout: { flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'] },
  composerFoot: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  scroll: { flex: 1, minHeight: 0, overflowY: 'auto', margin: `0 calc(-1 * ${s['--vv-s-8']})`, padding: `${s['--vv-s-6']} ${s['--vv-s-8']} ${s['--vv-s-14']}` },
  root: { maxWidth: 800, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: s['--vv-s-18'] },
  header: { display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-12'] },
  heading: { flex: 1, display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  when: { fontSize: 12, color: 'var(--color-fg-4)' },
  lead: { margin: 0, fontSize: 20, fontWeight: 600, lineHeight: 1.4, color: 'var(--color-fg)' },
  section: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  group: { fontSize: 11, textTransform: 'uppercase', fontWeight: 600, color: 'var(--color-fg-4)' },
  card: { padding: s['--vv-s-4'], display: 'flex', flexDirection: 'column', gap: s['--vv-s-1'], border: '1px solid var(--line-1)', borderRadius: r['--vv-r-12'], backgroundColor: 'var(--color-surface-1)' },
  row: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], width: '100%', padding: `${s['--vv-s-7']} ${s['--vv-s-10']}`, border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg-2)', textAlign: 'left', cursor: 'pointer' },
  rowText: { flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  empty: { padding: `${s['--vv-s-7']} ${s['--vv-s-10']}`, fontSize: 13, color: 'var(--color-fg-4)' },
  line: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minHeight: 40, padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-1)', color: 'var(--color-fg-2)', fontSize: 13 },
  tag: { fontSize: 11, color: 'var(--color-fg-4)' },
  tiles: { display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: s['--vv-s-8'] },
  tile: { padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-10'], backgroundColor: 'var(--color-surface-1)', display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], fontSize: 12, color: 'var(--color-fg-3)' },
  quiet: { border: 0, backgroundColor: 'transparent', color: 'var(--color-fg-2)', padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, cursor: 'pointer', whiteSpace: 'nowrap' },
  primary: { alignSelf: 'flex-start', padding: `${s['--vv-s-7']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-8'], backgroundColor: 'var(--color-fg)', color: 'var(--color-surface-1)', textDecoration: 'none', fontSize: 13 },
})
