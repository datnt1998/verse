import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { getLiveHistory } from '../../api/client'
import type { LiveHistoryOperation, LiveHistoryRound } from '../../api/types'
import { useStoryEvents } from '../../api/useStoryEvents'
import { runCardModel } from '../../lib/runCardModel'
import type { RunCardBlock } from '../../lib/runCardModel'
import { mergeRunEvents, mergeRunHistory } from '../../lib/runHistoryMerge'
import { BottomSheet, MobilePage } from '../../mobile/primitives'
import { Icon } from '@noverse/ui'
import { navigate, paths, useRoute } from '../../router'

const noop = () => {}
const roles: Record<string, string> = { writer: 'Người viết', architect: 'Kiến trúc sư', editor: 'Biên tập viên', arbiter: 'Trọng tài' }
const icons: Record<string, string> = { novel_context: 'note-text', read_chapter: 'book-1', plan_chapter: 'routing-2', draft_chapter: 'edit-2', edit_chapter: 'magicpen', check_consistency: 'shield-tick', commit_chapter: 'folder-open', audit_foundation: 'shield-tick', revise_outline: 'routing-2', resolve_outline_feedback: 'tick-circle', save_book: 'document-text', save_foundation: 'lamp-on', save_review: 'chart', save_arc_summary: 'note-text', save_volume_summary: 'book-1' }
const clock = (at: string) => { const d = new Date(at); return Number.isNaN(d.valueOf()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) }
const button = { border: 0, background: 'transparent', color: 'var(--color-fg)', cursor: 'pointer' } as const

const TONES = {
  b: { bg: 'var(--tint-info)', fg: 'var(--frame-info-fg)' },
  w: { bg: 'var(--kit-warn-12)', fg: 'var(--kit-warn-soft)' },
  r: { bg: 'var(--tint-err)', fg: 'var(--kit-err-soft)' },
  g: { bg: 'var(--tint-ok)', fg: 'var(--color-ok)' },
} as const
const chip = (tone?: 'b' | 'w' | 'r' | 'g') => ({ height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 6, fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap', background: tone ? TONES[tone].bg : 'var(--color-surface-0)', color: tone ? TONES[tone].fg : 'var(--color-fg)' } as const)
const lbl: React.CSSProperties = { fontSize: 12, color: 'var(--color-fg-4)' }
const initials = (name: string) => name.split(/\s+/).map(w => [...w][0]).slice(-2).join('').toLocaleUpperCase('vi')

function Block({ block, project, story, mobile }: { block: RunCardBlock; project: string; story: string; mobile: boolean }) {
  const blockWrap: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }
  switch (block.kind) {
    case 'kv': return <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{block.items.map(([k, v, tone], i) => <span key={i} title={v} style={{ padding: '3px 8px', borderRadius: 6, border: '1px solid var(--kit-wash-7)', fontSize: 12, color: 'var(--color-fg-4)', lineHeight: 1.45, overflowWrap: 'anywhere', background: tone ? TONES[tone].bg : 'var(--color-surface-0)' }}>{k} <span style={{ color: tone ? TONES[tone].fg : 'var(--color-fg)' }}>{v}</span></span>)}</div>
    case 'list': return <div style={blockWrap}><span style={lbl}>{block.label}</span><ul style={{ margin: 0, paddingLeft: 16, display: 'flex', flexDirection: 'column', gap: 3, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>{block.items.map((x, i) => <li key={i}>{x}</li>)}</ul></div>
    case 'text': return <div style={blockWrap}><span style={lbl}>{block.label}</span><p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg)', lineHeight: 1.6 }}>{block.text}</p></div>
    case 'quote': return <p style={{ margin: 0, paddingLeft: 12, borderLeft: '2px solid var(--kit-acc-40)', fontFamily: 'var(--font-read)', fontSize: 14, color: 'var(--kit-fg-note)', lineHeight: 1.65 }}>{block.text}</p>
    case 'diff': return <div style={{ display: 'flex', flexDirection: 'column', borderRadius: 10, overflow: 'hidden', border: '1px solid var(--kit-wash-7)', fontSize: 13, lineHeight: 1.6 }}>
      <div style={{ display: 'flex', gap: 10, padding: '8px 12px', background: 'var(--kit-errf-7)' }}><span aria-label="Cũ" style={{ color: 'var(--color-err)', flex: 'none' }}>−</span><span style={{ color: 'var(--color-fg-2)', textDecoration: 'line-through', textDecorationColor: 'var(--kit-errf-55)' }}>{block.old}</span></div>
      <div style={{ display: 'flex', gap: 10, padding: '8px 12px', background: 'var(--kit-ok-6)' }}><span aria-label="Mới" style={{ color: 'var(--color-ok)', flex: 'none' }}>+</span><span style={{ color: 'var(--color-fg)' }}>{block.next}</span></div>
    </div>
    case 'issues': return <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{block.items.map((i, x) => {
      const title = i.level === 'e' ? 'Bắt sửa' : i.level === 'w' ? 'Cảnh báo' : 'Lưu ý'
      return <div key={x} style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 12px', borderRadius: 10, background: 'var(--color-surface-0)', color: 'var(--color-fg)', border: '1px solid var(--kit-wash-7)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}><span style={{ ...chip(i.level === 'e' ? 'r' : i.level === 'w' ? 'w' : 'b'), background: i.level === 'e' ? 'var(--kit-errf-14)' : i.level === 'w' ? 'var(--kit-warn-14)' : 'var(--kit-up-badge-bg)', ...(i.level === 'i' ? { color: 'var(--kit-up-badge-fg)' } : {}) }}>{title}</span><span style={{ fontSize: 12, color: 'var(--kit-fg-meta)' }}>{i.ref}</span></span>
        <span style={{ fontSize: 13, color: 'var(--color-fg)', lineHeight: 1.5 }}>{i.text}</span>
        {i.fix && <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.5 }}><span style={{ color: 'var(--color-fg-2)' }}>Gợi ý:</span> {i.fix}</span>}
      </div> })}</div>
    case 'cast': return <div style={blockWrap}><span style={lbl}>Nhân vật</span><span style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{block.names.map((n, i) => <span key={i} title={n} style={{ height: 26, maxWidth: '100%', padding: '0 10px 0 4px', display: 'inline-flex', alignItems: 'center', gap: 6, borderRadius: 999, background: 'var(--color-surface-4)', color: 'var(--color-fg)', border: '1px solid var(--kit-wash-10)', fontSize: 12 }}><i aria-hidden="true" style={{ width: 18, height: 18, borderRadius: '50%', background: 'var(--kit-acc-16)', color: 'var(--color-accent-soft)', fontStyle: 'normal', fontSize: 8.5, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{initials(n)}</i><span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{n}</span></span>)}</span></div>
    case 'tags': return <div style={blockWrap}><span style={lbl}>{block.label}</span>{block.items.map(([a, t, tone], i) => <div key={i} style={{ display: 'flex', gap: 10, alignItems: 'flex-start', padding: '8px 12px', borderRadius: 10, background: 'var(--color-surface-0)', border: '1px solid var(--kit-wash-6)' }}><span style={{ ...chip(tone), flex: 'none' }}>{a}</span><span style={{ fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5, overflowWrap: 'anywhere' }}>{t}</span></div>)}</div>
    case 'scores': return <div style={{ display: 'grid', gridTemplateColumns: mobile ? 'repeat(2,minmax(0,1fr))' : 'repeat(4,minmax(0,1fr))', gap: 6 }}>{block.items.map(([k, v], i) => <span key={i} title={k} style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '7px 10px', borderRadius: 9, background: 'var(--color-surface-0)', border: '1px solid var(--kit-wash-6)', fontSize: 12, color: 'var(--kit-fg-meta)', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{k}<strong className="num" style={{ fontSize: 15, fontWeight: 600, color: 'var(--color-fg)' }}>{v}</strong></span>)}</div>
    case 'ol': return <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gridTemplateColumns: mobile ? '1fr' : 'repeat(2,minmax(0,1fr))', gap: '3px 18px', fontSize: 13, color: 'var(--color-fg-2)' }}>{block.items.map((t, i) => <li key={i} style={{ display: 'flex', gap: 8, minWidth: 0 }}><span className="num" style={{ color: 'var(--kit-fg-faint)', width: 14, flex: 'none' }}>{i + 1}</span><span title={t} style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t}</span></li>)}</ol>
    case 'note': return <p style={{ margin: 0, fontSize: 12.5, color: 'var(--kit-fg-meta)', lineHeight: 1.55 }}>{block.text}</p>
    case 'link': {
      if (!project || !story) return null
      let href: string
      if (block.href === 'premise') href = `${paths.story(project, story)}?tab=premise`
      else {
        if (!block.chapter) return null
        href = paths.reader(project, story, block.chapter)
      }
      return <a href={`#${href}`} style={{ display: 'inline-flex', fontSize: 12.5, lineHeight: 1.5, color: 'var(--color-accent)', textDecoration: 'none', alignSelf: 'flex-start' }}>{block.text}</a>
    }
  }
}

export function RunOperationBody({ operation, mobile = false }: { operation: LiveHistoryOperation; mobile?: boolean }) {
  const [techOpen, setTechOpen] = useState(false)
  const { route } = useRoute()
  const card = runCardModel(operation.key, operation.params, operation.result)
  const story = 'story' in route ? route.story : ''
  const project = 'project' in route ? route.project : ''
  return <div style={{ padding: mobile ? '4px 18px 8px' : '2px 14px 12px 40px', fontSize: 13, lineHeight: 1.5, color: 'var(--color-fg-2)', display: 'flex', flexDirection: 'column', gap: 10 }}>
    {operation.status === 'running' ? <p style={{ margin: 0, fontSize: 12.5, color: 'var(--kit-fg-meta)', lineHeight: 1.55 }}>Bắt đầu lúc {clock(operation.startedAt)}. Thẻ tự cập nhật khi xong; không cần tải lại.</p>
      : operation.status === 'error' ? <p role="alert" style={{ margin: 0, padding: '12px 14px', borderRadius: 10, background: 'var(--tint-err)', color: 'var(--color-err)', lineHeight: 1.5 }}>{operation.error}</p>
      : card.blocks.map((block, i) => <div key={i} data-run-block={block.kind}><Block block={block} project={project} story={story} mobile={mobile} /></div>)}
    <details onToggle={event => setTechOpen(event.currentTarget.open)} style={{ borderTop: '1px solid var(--kit-wash-6)', paddingTop: 8 }}>
      <summary style={{ display: 'flex', alignItems: 'center', gap: 6, height: 24, color: 'var(--color-fg-4)', fontSize: 12, cursor: 'pointer', listStyle: 'none' }}>Chi tiết kỹ thuật</summary>
      {techOpen && <pre style={{ marginTop: 6, padding: '8px 10px', borderRadius: 8, background: 'var(--color-bg)', border: '1px solid var(--kit-wash-6)', fontFamily: 'monospace', fontSize: 11.5, color: 'var(--color-fg-3)', lineHeight: 1.6, display: 'flex', flexDirection: 'column', gap: 2, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{`op: ${operation.key}\nargs: ${operation.params}\nresult: ${operation.result}${operation.error ? `\nerror: ${operation.error}` : ''}`}</pre>}
    </details>
  </div>
}

function Operation({ operation, mobile, onSelect }: { operation: LiveHistoryOperation; mobile: boolean; onSelect: (operation: LiveHistoryOperation) => void }) {
  const card = runCardModel(operation.key, operation.params, operation.result)
  const summary = operation.status === 'running' ? `đang ${card.name.toLocaleLowerCase('vi')}…` : operation.status === 'error' ? `không thực hiện được · ${card.summary}` : card.summary
  const content = <><Icon name={icons[operation.key] ?? 'layer'} size={16} style={{ color: 'var(--color-ok)', flex: 'none' }} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: mobile ? 'column' : 'row', alignItems: mobile ? 'stretch' : 'center', gap: mobile ? 0 : 12, textAlign: 'left' }}><strong style={{ fontSize: 13, whiteSpace: 'nowrap' }}>{card.name}</strong><span title={summary} style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12, color: operation.status === 'error' ? 'var(--color-err)' : 'var(--color-fg-4)' }}>{summary}</span></span><span style={{ fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap' }}>{operation.status === 'running' ? 'Đang chạy' : `${(operation.durationMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} giây`}</span><Icon name={operation.status === 'error' ? 'warning-2' : operation.status === 'running' ? 'clock' : 'tick-circle-bulk'} size={15} /></>
  const rowStyle = { ...button, width: '100%', height: mobile ? 58 : 40, padding: '0 12px', display: 'flex', alignItems: 'center', gap: 10 } as const
  if (mobile) return <button type="button" style={rowStyle} onClick={() => onSelect(operation)}>{content}</button>
  return <details style={{ position: operation.status === 'running' ? 'sticky' : 'relative', bottom: operation.status === 'running' ? 0 : undefined, zIndex: operation.status === 'running' ? 1 : undefined, border: `1px solid ${operation.status === 'error' ? 'var(--color-err)' : 'var(--line-1)'}`, borderRadius: 10, background: 'var(--color-surface-1)' }}><summary style={{ ...rowStyle, listStyle: 'none' }}>{content}</summary><RunOperationBody operation={operation} /></details>
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
  const content = <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0, height: '100%', position: 'relative' }}>
    {!mobile && <header style={{ display: 'flex', padding: '4px 16px', gap: 10 }}>{!embedded && <strong>Luồng viết</strong>}<span style={{ flex: 1 }} /><button type="button" style={button} onClick={() => scroll.current?.scrollTo({ top: 0, behavior: 'smooth' })}>Lịch sử ›</button></header>}
    <div style={{ display: 'flex', gap: 8, overflowX: 'auto', padding: '8px 16px' }}>{[['all','Tất cả'], ['architect',roles.architect], ['writer',roles.writer], ['editor',roles.editor]].map(([key, label]) => <button key={key} type="button" aria-pressed={role === key} onClick={() => setRole(key)} style={{ ...button, padding: '7px 12px', whiteSpace: 'nowrap', border: '1px solid var(--line-1)', borderRadius: 30, background: role === key ? 'var(--color-surface-5)' : 'var(--color-surface-1)' }}>{label}</button>)}</div>
    <div ref={scroll} onScroll={event => { const e = event.currentTarget; setAway(e.scrollHeight - e.scrollTop - e.clientHeight > 100) }} style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: mobile ? '8px 16px 24px' : '8px 7px 24px' }}>
      {cursor !== null && <button type="button" style={button} disabled={olderLoading} onClick={() => void older()}>{olderLoading ? 'Đang tải…' : 'Tải thêm vòng cũ hơn'}</button>}
      {error && <div role="alert">Không tải được lịch sử. <button type="button" style={button} onClick={() => void refresh()}>Thử lại</button></div>}
      {loading && <p>Đang tải lịch sử…</p>}
      {!loading && rounds.length === 0 && (empty ?? <p>Chưa có lịch sử luồng viết.</p>)}
      {rounds.filter(round => role === 'all' || round.role === role).map(round => <section key={round.round} style={{ marginTop: 18 }}><h3 style={{ fontSize: 12, fontWeight: 500, color: 'var(--color-fg-3)' }}><span className={`m-orb ${round.role}`} /> {['Vòng ' + round.round, roles[round.role], round.title, clock(round.startedAt)].filter(Boolean).join(' · ')}</h3><div style={{ display: 'flex', flexDirection: 'column', gap: mobile ? 0 : 6, ...(mobile ? { border: '1px solid var(--line-1)', borderRadius: 16, background: 'var(--color-surface-1)', overflow: 'hidden' } : {}) }}>{[...round.operations].sort((a, b) => Number(a.status === 'running') - Number(b.status === 'running')).map(operation => <Operation key={operation.id} operation={operation} mobile={mobile} onSelect={setSelected} />)}</div></section>)}
    </div>
    {mobile && running && away && <button type="button" onClick={() => scroll.current?.scrollTo({ top: scroll.current.scrollHeight, behavior: 'smooth' })} style={{ ...button, position: 'absolute', bottom: 34, left: '50%', transform: 'translateX(-50%)', width: 243, height: 40, borderRadius: 30, background: 'var(--color-surface-5)' }}>Đang chạy · về mới nhất</button>}
    {mobile && <BottomSheet open={!!current} onClose={() => setSelected(null)} height="auto" style={{ maxHeight: 'min(760px, 90vh)' }} title={current ? runCardModel(current.key, current.params, current.result).name : ''}>{current && <div style={{ overflowY: 'auto', paddingBottom: 8 }}><div style={{ padding: '0 18px', color: 'var(--color-fg-4)', fontSize: 12 }}>{[roles[current.role], `vòng ${current.round}`, clock(current.startedAt), current.status === 'error' ? 'lỗi' : current.status === 'running' ? 'Đang chạy' : `${(current.durationMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} giây`].filter(Boolean).join(' · ')}</div><p style={{ padding: '0 18px', color: current.status === 'error' ? 'var(--color-err)' : 'var(--color-fg-3)', fontSize: 13 }}>{current.status === 'error' ? 'không thực hiện được · ' : ''}{runCardModel(current.key, current.params, current.result).summary}</p><RunOperationBody operation={current} mobile /></div>}</BottomSheet>}
  </div>
  return embedded ? content : mobile ? <MobilePage title="Luồng viết" onBack={() => navigate(paths.workspace(project, story))} manageScroll={false}>{content}</MobilePage> : <section className="card" style={{ flex: '6 1 0', minHeight: 0 }}>{content}</section>
}
