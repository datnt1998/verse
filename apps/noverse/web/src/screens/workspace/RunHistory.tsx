import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, frame, font, kit, line, tint } from '@noverse/ui/tokens.stylex'
import { getLiveHistory } from '../../api/client'
import type { LiveHistoryOperation, LiveHistoryRound } from '../../api/types'
import { useStoryEvents } from '../../api/useStoryEvents'
import { runCardModel } from '../../lib/runCardModel'
import type { RunCardBlock } from '../../lib/runCardModel'
import { mergeRunEvents, mergeRunHistory } from '../../lib/runHistoryMerge'
import { MobilePage } from '../../mobile/primitives'
import { Icon, Sheet as BottomSheet } from '@noverse/ui'
import { navigate, paths, useRoute } from '../../router'

const noop = () => {}
const roles: Record<string, string> = { writer: 'Người viết', architect: 'Kiến trúc sư', editor: 'Biên tập viên', arbiter: 'Trọng tài' }
const icons: Record<string, string> = { novel_context: 'note-text', read_chapter: 'book-1', plan_chapter: 'routing-2', draft_chapter: 'edit-2', edit_chapter: 'magicpen', check_consistency: 'shield-tick', commit_chapter: 'folder-open', audit_foundation: 'shield-tick', revise_outline: 'routing-2', resolve_outline_feedback: 'tick-circle', save_book: 'document-text', save_foundation: 'lamp-on', save_review: 'chart', save_arc_summary: 'note-text', save_volume_summary: 'book-1' }
const clock = (at: string) => { const d = new Date(at); return Number.isNaN(d.valueOf()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) }
const s = stylex.create({
  button: { borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], cursor: 'pointer' },
  column: { display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 },
  label: { fontSize: 12, color: color['--color-fg-4'] },
  chips: { display: 'flex', flexWrap: 'wrap', gap: 6 },
  kv: { padding: '3px 8px', borderRadius: 6, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], fontSize: 12, color: color['--color-fg-4'], lineHeight: 1.45, overflowWrap: 'anywhere', backgroundColor: color['--color-surface-0'] },
  value: { color: color['--color-fg'] },
  toneB: { backgroundColor: tint['--tint-info'] }, toneW: { backgroundColor: kit['--kit-warn-12'] }, toneR: { backgroundColor: tint['--tint-err'] }, toneG: { backgroundColor: tint['--tint-ok'] },
  fgB: { color: frame['--frame-info-fg'] }, fgW: { color: kit['--kit-warn-soft'] }, fgR: { color: kit['--kit-err-soft'] }, fgG: { color: color['--color-ok'] },
  list: { margin: 0, paddingLeft: 16, display: 'flex', flexDirection: 'column', gap: 3, fontSize: 13, color: color['--color-fg-2'], lineHeight: 1.55 },
  text: { margin: 0, fontSize: 13, color: color['--color-fg'], lineHeight: 1.6 },
  quote: { margin: 0, paddingLeft: 12, borderLeftWidth: 2, borderLeftStyle: 'solid', borderLeftColor: kit['--kit-acc-40'], fontFamily: font['--font-read'], fontSize: 14, color: kit['--kit-fg-note'], lineHeight: 1.65 },
  diff: { display: 'flex', flexDirection: 'column', borderRadius: 10, overflow: 'hidden', borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], fontSize: 13, lineHeight: 1.6 },
  diffRow: { display: 'flex', gap: 10, padding: '8px 12px' }, diffOld: { backgroundColor: kit['--kit-errf-7'] }, diffNew: { backgroundColor: kit['--kit-ok-6'] },
  minus: { color: color['--color-err'], flexShrink: 0 }, plus: { color: color['--color-ok'], flexShrink: 0 },
  oldText: { color: color['--color-fg-2'], textDecorationLine: 'line-through', textDecorationColor: kit['--kit-errf-55'] },
  issues: { display: 'flex', flexDirection: 'column', gap: 8 },
  issue: { display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 12px', borderRadius: 10, backgroundColor: color['--color-surface-0'], color: color['--color-fg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'] },
  issueHead: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  chip: { height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 6, fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap', backgroundColor: color['--color-surface-0'], color: color['--color-fg'] },
  issueE: { backgroundColor: kit['--kit-errf-14'] }, issueW: { backgroundColor: kit['--kit-warn-14'] }, issueI: { backgroundColor: kit['--kit-up-badge-bg'], color: kit['--kit-up-badge-fg'] },
  meta: { fontSize: 12, color: kit['--kit-fg-meta'] }, issueText: { fontSize: 13, color: color['--color-fg'], lineHeight: 1.5 },
  issueFix: { fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5 }, fg2: { color: color['--color-fg-2'] },
  cast: { height: 26, maxWidth: '100%', padding: '0 10px 0 4px', display: 'inline-flex', alignItems: 'center', gap: 6, borderRadius: 999, backgroundColor: color['--color-surface-4'], color: color['--color-fg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-10'], fontSize: 12 },
  avatar: { width: 18, height: 18, borderRadius: '50%', backgroundColor: kit['--kit-acc-16'], color: color['--color-accent-soft'], fontStyle: 'normal', fontSize: 8.5, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  truncate: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  tag: { display: 'flex', gap: 10, alignItems: 'flex-start', padding: '8px 12px', borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'] },
  noShrink: { flexShrink: 0 }, tagText: { fontSize: 13, color: color['--color-fg-2'], lineHeight: 1.5, overflowWrap: 'anywhere' },
  scores: { display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 6 }, scoresMobile: { gridTemplateColumns: 'repeat(2,minmax(0,1fr))' },
  score: { display: 'flex', flexDirection: 'column', gap: 2, padding: '7px 10px', borderRadius: 9, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'], fontSize: 12, color: kit['--kit-fg-meta'], minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' },
  scoreValue: { fontSize: 15, fontWeight: 600, color: color['--color-fg'], fontVariantNumeric: 'tabular-nums' },
  ordered: { listStyleType: 'none', margin: 0, padding: 0, display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: '3px 18px', fontSize: 13, color: color['--color-fg-2'] }, orderedMobile: { gridTemplateColumns: '1fr' },
  orderedItem: { display: 'flex', gap: 8, minWidth: 0 }, orderedNumber: { color: kit['--kit-fg-faint'], width: 14, flexShrink: 0, fontVariantNumeric: 'tabular-nums' },
  note: { margin: 0, fontSize: 12.5, color: kit['--kit-fg-meta'], lineHeight: 1.55 },
  link: { display: 'inline-flex', fontSize: 12.5, lineHeight: 1.5, color: color['--color-accent'], textDecorationLine: 'none', alignSelf: 'flex-start' },
  body: { padding: '2px 14px 12px 40px', fontSize: 13, lineHeight: 1.5, color: color['--color-fg-2'], display: 'flex', flexDirection: 'column', gap: 10 }, bodyMobile: { padding: '4px 18px 8px' },
  errorBody: { margin: 0, padding: '12px 14px', borderRadius: 10, backgroundColor: tint['--tint-err'], color: color['--color-err'], lineHeight: 1.5 },
  technical: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-6'], paddingTop: 8 },
  technicalHead: { display: 'flex', alignItems: 'center', gap: 6, height: 24, color: color['--color-fg-4'], fontSize: 12, cursor: 'pointer', listStyleType: 'none' },
  technicalCode: { marginTop: 6, padding: '8px 10px', borderRadius: 8, backgroundColor: color['--color-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'], fontFamily: 'monospace', fontSize: 11.5, color: color['--color-fg-3'], lineHeight: 1.6, display: 'flex', flexDirection: 'column', gap: 2, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' },
  opIcon: { color: color['--color-ok'], flexShrink: 0 }, opCopy: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0, display: 'flex', flexDirection: 'row', alignItems: 'center', gap: 12, textAlign: 'left' }, opCopyMobile: { flexDirection: 'column', alignItems: 'stretch', gap: 0 },
  opName: { fontSize: 13, whiteSpace: 'nowrap' }, opSummary: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12, color: color['--color-fg-4'] }, errorText: { color: color['--color-err'] }, opDuration: { fontSize: 12, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  opRow: { width: '100%', height: 40, padding: '0 12px', display: 'flex', alignItems: 'center', gap: 10 }, opRowMobile: { height: 58 }, noMarker: { listStyleType: 'none' },
  opCard: { position: 'relative', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: 10, backgroundColor: color['--color-surface-1'] }, opCardError: { borderColor: color['--color-err'] }, opCardRunning: { position: 'sticky', bottom: 0, zIndex: 1 },
  root: { display: 'flex', flexDirection: 'column', minHeight: 0, height: '100%', position: 'relative' },
  header: { display: 'flex', padding: '4px 16px', gap: 10 }, spacer: { flexGrow: 1, flexShrink: 1, flexBasis: 0 },
  filters: { display: 'flex', gap: 8, overflowX: 'auto', padding: '8px 16px' }, filter: { padding: '7px 12px', whiteSpace: 'nowrap', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: 30, backgroundColor: color['--color-surface-1'] }, filterSelected: { backgroundColor: color['--color-surface-5'] },
  scroll: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0, overflowY: 'auto', padding: '8px 7px 24px' }, scrollMobile: { padding: '8px 16px 24px' },
  round: { marginTop: 18 }, roundTitle: { fontSize: 12, fontWeight: 500, color: color['--color-fg-3'] },
  roundCards: { display: 'flex', flexDirection: 'column', gap: 6 }, roundCardsMobile: { gap: 0, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: 16, backgroundColor: color['--color-surface-1'], overflow: 'hidden' },
  floating: { position: 'absolute', bottom: 34, left: '50%', transform: 'translateX(-50%)', width: 243, height: 40, borderRadius: 30, backgroundColor: color['--color-surface-5'] },
  sheet: { maxHeight: 'min(760px, 90vh) !important' }, sheetContent: { overflowY: 'auto', paddingBottom: 8 }, sheetMeta: { padding: '0 18px', color: color['--color-fg-4'], fontSize: 12 }, sheetSummary: { padding: '0 18px', color: color['--color-fg-3'], fontSize: 13 },
  panel: { flexGrow: 6, flexShrink: 1, flexBasis: 0, minHeight: 0 },
})
const toneBg = { b: s.toneB, w: s.toneW, r: s.toneR, g: s.toneG }
const toneFg = { b: s.fgB, w: s.fgW, r: s.fgR, g: s.fgG }
const initials = (name: string) => name.split(/\s+/).map(w => [...w][0]).slice(-2).join('').toLocaleUpperCase('vi')

function Block({ block, project, story, mobile }: { block: RunCardBlock; project: string; story: string; mobile: boolean }) {
  switch (block.kind) {
    case 'kv': return <div {...stylex.props(s.chips)}>{block.items.map(([k, v, tone], i) => <span key={i} title={v} {...stylex.props(s.kv, tone && toneBg[tone])}>{k} <span {...stylex.props(s.value, tone && toneFg[tone])}>{v}</span></span>)}</div>
    case 'list': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span><ul {...stylex.props(s.list)}>{block.items.map((x, i) => <li key={i}>{x}</li>)}</ul></div>
    case 'text': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span><p {...stylex.props(s.text)}>{block.text}</p></div>
    case 'quote': return <p {...stylex.props(s.quote)}>{block.text}</p>
    case 'diff': return <div {...stylex.props(s.diff)}>
      <div {...stylex.props(s.diffRow, s.diffOld)}><span aria-label="Cũ" {...stylex.props(s.minus)}>−</span><span {...stylex.props(s.oldText)}>{block.old}</span></div>
      <div {...stylex.props(s.diffRow, s.diffNew)}><span aria-label="Mới" {...stylex.props(s.plus)}>+</span><span {...stylex.props(s.value)}>{block.next}</span></div>
    </div>
    case 'issues': return <div {...stylex.props(s.issues)}>{block.items.map((i, x) => {
      const title = i.level === 'e' ? 'Bắt sửa' : i.level === 'w' ? 'Cảnh báo' : 'Lưu ý'
      return <div key={x} {...stylex.props(s.issue)}>
        <span {...stylex.props(s.issueHead)}><span {...stylex.props(s.chip, i.level === 'e' ? s.fgR : i.level === 'w' ? s.fgW : s.fgB, i.level === 'e' ? s.issueE : i.level === 'w' ? s.issueW : s.issueI)}>{title}</span><span {...stylex.props(s.meta)}>{i.ref}</span></span>
        <span {...stylex.props(s.issueText)}>{i.text}</span>
        {i.fix && <span {...stylex.props(s.issueFix)}><span {...stylex.props(s.fg2)}>Gợi ý:</span> {i.fix}</span>}
      </div> })}</div>
    case 'cast': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>Nhân vật</span><span {...stylex.props(s.chips)}>{block.names.map((n, i) => <span key={i} title={n} {...stylex.props(s.cast)}><i aria-hidden="true" {...stylex.props(s.avatar)}>{initials(n)}</i><span {...stylex.props(s.truncate)}>{n}</span></span>)}</span></div>
    case 'tags': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span>{block.items.map(([a, t, tone], i) => <div key={i} {...stylex.props(s.tag)}><span {...stylex.props(s.chip, tone && toneBg[tone], tone && toneFg[tone], s.noShrink)}>{a}</span><span {...stylex.props(s.tagText)}>{t}</span></div>)}</div>
    case 'scores': return <div {...stylex.props(s.scores, mobile && s.scoresMobile)}>{block.items.map(([k, v], i) => <span key={i} title={k} {...stylex.props(s.score)}>{k}<strong {...stylex.props(s.scoreValue)}>{v}</strong></span>)}</div>
    case 'ol': return <ol {...stylex.props(s.ordered, mobile && s.orderedMobile)}>{block.items.map((t, i) => <li key={i} {...stylex.props(s.orderedItem)}><span {...stylex.props(s.orderedNumber)}>{i + 1}</span><span title={t} {...stylex.props(s.truncate)}>{t}</span></li>)}</ol>
    case 'note': return <p {...stylex.props(s.note)}>{block.text}</p>
    case 'link': {
      if (!project || !story) return null
      let href: string
      if (block.href === 'premise') href = `${paths.story(project, story)}?tab=premise`
      else {
        if (!block.chapter) return null
        href = paths.reader(project, story, block.chapter)
      }
      return <a href={`#${href}`} {...stylex.props(s.link)}>{block.text}</a>
    }
  }
}

export function RunOperationBody({ operation, mobile = false }: { operation: LiveHistoryOperation; mobile?: boolean }) {
  const [techOpen, setTechOpen] = useState(false)
  const { route } = useRoute()
  const card = runCardModel(operation.key, operation.params, operation.result)
  const story = 'story' in route ? route.story ?? '' : ''
  const project = 'project' in route ? route.project ?? '' : ''
  return <div {...stylex.props(s.body, mobile && s.bodyMobile)}>
    {operation.status === 'running' ? <p {...stylex.props(s.note)}>Bắt đầu lúc {clock(operation.startedAt)}. Thẻ tự cập nhật khi xong; không cần tải lại.</p>
      : operation.status === 'error' ? <p role="alert" {...stylex.props(s.errorBody)}>{operation.error}</p>
      : card.blocks.map((block, i) => <div key={i} data-run-block={block.kind}><Block block={block} project={project} story={story} mobile={mobile} /></div>)}
    <details onToggle={event => setTechOpen(event.currentTarget.open)} {...stylex.props(s.technical)}>
      <summary {...stylex.props(s.technicalHead)}>Chi tiết kỹ thuật</summary>
      {techOpen && <pre {...stylex.props(s.technicalCode)}>{`op: ${operation.key}\nargs: ${operation.params}\nresult: ${operation.result}${operation.error ? `\nerror: ${operation.error}` : ''}`}</pre>}
    </details>
  </div>
}

function Operation({ operation, mobile, onSelect }: { operation: LiveHistoryOperation; mobile: boolean; onSelect: (operation: LiveHistoryOperation) => void }) {
  const card = runCardModel(operation.key, operation.params, operation.result)
  const summary = operation.status === 'running' ? `đang ${card.name.toLocaleLowerCase('vi')}…` : operation.status === 'error' ? `không thực hiện được · ${card.summary}` : card.summary
  const content = <><Icon name={icons[operation.key] ?? 'layer'} size={16} xstyle={s.opIcon} /><span {...stylex.props(s.opCopy, mobile && s.opCopyMobile)}><strong {...stylex.props(s.opName)}>{card.name}</strong><span title={summary} {...stylex.props(s.opSummary, operation.status === 'error' && s.errorText)}>{summary}</span></span><span {...stylex.props(s.opDuration)}>{operation.status === 'running' ? 'Đang chạy' : `${(operation.durationMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} giây`}</span><Icon name={operation.status === 'error' ? 'warning-2' : operation.status === 'running' ? 'clock' : 'tick-circle-bulk'} size={15} /></>
  if (mobile) return <button type="button" {...stylex.props(s.button, s.opRow, s.opRowMobile)} onClick={() => onSelect(operation)}>{content}</button>
  return <details {...stylex.props(s.opCard, operation.status === 'running' && s.opCardRunning, operation.status === 'error' && s.opCardError)}><summary {...stylex.props(s.button, s.opRow, s.noMarker)}>{content}</summary><RunOperationBody operation={operation} /></details>
}

export function RunHistory({ project, story, mobile = false, embedded = false, empty }: { project: string; story: string; mobile?: boolean; embedded?: boolean; empty?: ReactNode }) {
  const [rounds, setRounds] = useState<LiveHistoryRound[]>([])
  const [cursor, setCursor] = useState<number | null>(null)
  const [loading, setLoading] = useState(true), [olderLoading, setOlderLoading] = useState(false), [error, setError] = useState(false)
  const [role, setRole] = useState('all'), [selected, setSelected] = useState<LiveHistoryOperation | null>(null), [away, setAway] = useState(false)
  const scroll = useRef<HTMLDivElement>(null)
  const loaded = useRef(false)
  const positioned = useRef(false)
  const live = useStoryEvents(project, story, noop)
  const refresh = useCallback(async (signal?: AbortSignal) => {
    try {
      const page = await getLiveHistory(project, story, { limit: 20 }, signal)
      if (signal?.aborted) return
      setRounds(old => mergeRunHistory(old, page.rounds))
      if (!loaded.current) { setCursor(page.nextBeforeRound); loaded.current = true }
      setError(false)
    } catch { if (!signal?.aborted) setError(true) } finally { if (!signal?.aborted) setLoading(false) }
  }, [project, story])
  useEffect(() => { const controller = new AbortController(); loaded.current = false; positioned.current = false; setRounds([]); setCursor(null); setLoading(true); void refresh(controller.signal); return () => controller.abort() }, [refresh])
  useEffect(() => { if (!loading && live.events.length) setRounds(old => mergeRunEvents(old, live.events)) }, [live.events, loading])
  useEffect(() => { if (!positioned.current && rounds.length && scroll.current) { scroll.current.scrollTop = scroll.current.scrollHeight; positioned.current = true } }, [rounds])
  const older = async () => { if (cursor === null || olderLoading) return; setOlderLoading(true); try { const page = await getLiveHistory(project, story, { beforeRound: cursor, limit: 20 }); setRounds(old => mergeRunHistory(old, page.rounds)); setCursor(page.nextBeforeRound); setError(false) } catch { setError(true) } finally { setOlderLoading(false) } }
  const current = selected && rounds.flatMap(r => r.operations).find(o => o.id === selected.id) || selected
  const running = rounds.some(r => r.operations.some(o => o.status === 'running'))
  const content = <div {...stylex.props(s.root)}>
    {!mobile && <header {...stylex.props(s.header)}>{!embedded && <strong>Luồng viết</strong>}<span {...stylex.props(s.spacer)} /><button type="button" {...stylex.props(s.button)} onClick={() => scroll.current?.scrollTo({ top: 0, behavior: 'smooth' })}>Lịch sử ›</button></header>}
    <div {...stylex.props(s.filters)}>{[['all','Tất cả'], ['architect',roles.architect], ['writer',roles.writer], ['editor',roles.editor]].map(([key, label]) => <button key={key} type="button" aria-pressed={role === key} onClick={() => setRole(key)} {...stylex.props(s.button, s.filter, role === key && s.filterSelected)}>{label}</button>)}</div>
    <div ref={scroll} onScroll={event => { const e = event.currentTarget; setAway(e.scrollHeight - e.scrollTop - e.clientHeight > 100) }} {...stylex.props(s.scroll, mobile && s.scrollMobile)}>
      {cursor !== null && <button type="button" {...stylex.props(s.button)} disabled={olderLoading} onClick={() => void older()}>{olderLoading ? 'Đang tải…' : 'Tải thêm vòng cũ hơn'}</button>}
      {error && <div role="alert">Không tải được lịch sử. <button type="button" {...stylex.props(s.button)} onClick={() => void refresh()}>Thử lại</button></div>}
      {loading && <p>Đang tải lịch sử…</p>}
      {!loading && rounds.length === 0 && (empty ?? <p>Chưa có lịch sử luồng viết.</p>)}
      {rounds.filter(round => role === 'all' || round.role === role).map(round => <section key={round.round} {...stylex.props(s.round)}><h3 {...stylex.props(s.roundTitle)}><span className={`m-orb ${round.role}`} /> {['Vòng ' + round.round, roles[round.role], round.title, clock(round.startedAt)].filter(Boolean).join(' · ')}</h3><div {...stylex.props(s.roundCards, mobile && s.roundCardsMobile)}>{[...round.operations].sort((a, b) => Number(a.status === 'running') - Number(b.status === 'running')).map(operation => <Operation key={operation.id} operation={operation} mobile={mobile} onSelect={setSelected} />)}</div></section>)}
    </div>
    {mobile && running && away && <button type="button" onClick={() => scroll.current?.scrollTo({ top: scroll.current.scrollHeight, behavior: 'smooth' })} {...stylex.props(s.button, s.floating)}>Đang chạy · về mới nhất</button>}
    {mobile && <BottomSheet open={!!current} onClose={() => setSelected(null)} height="auto" xstyle={s.sheet} title={current ? runCardModel(current.key, current.params, current.result).name : ''}>{current && <div {...stylex.props(s.sheetContent)}><div {...stylex.props(s.sheetMeta)}>{[roles[current.role], `vòng ${current.round}`, clock(current.startedAt), current.status === 'error' ? 'lỗi' : current.status === 'running' ? 'Đang chạy' : `${(current.durationMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} giây`].filter(Boolean).join(' · ')}</div><p {...stylex.props(s.sheetSummary, current.status === 'error' && s.errorText)}>{current.status === 'error' ? 'không thực hiện được · ' : ''}{runCardModel(current.key, current.params, current.result).summary}</p><RunOperationBody operation={current} mobile /></div>}</BottomSheet>}
  </div>
  return embedded ? content : mobile ? <MobilePage title="Luồng viết" onBack={() => navigate(paths.workspace(project, story))} manageScroll={false}>{content}</MobilePage> : <section className={`card ${stylex.props(s.panel).className}`}>{content}</section>
}
