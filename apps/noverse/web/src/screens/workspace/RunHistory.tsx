import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { getLiveHistory } from '../../api/client'
import type { LiveHistoryOperation, LiveHistoryRound } from '../../api/types'
import { useStoryEvents } from '../../api/useStoryEvents'
import { runCardModel } from '../../lib/runCardModel'
import { mergeRunEvents, mergeRunHistory } from '../../lib/runHistoryMerge'
import { BottomSheet, MobilePage } from '../../mobile/primitives'
import { Icon } from '@noverse/ui'
import { navigate, paths, useRoute } from '../../router'
import { errorLabel } from '../../lib/labels'

const noop = () => {}
const roles: Record<string, string> = { writer: 'Người viết', architect: 'Kiến trúc sư', editor: 'Biên tập viên', arbiter: 'Trọng tài' }
const icons: Record<string, string> = { novel_context: 'note-text', read_chapter: 'book-1', plan_chapter: 'routing-2', draft_chapter: 'edit-2', edit_chapter: 'magicpen', check_consistency: 'shield-tick', commit_chapter: 'folder-open', audit_foundation: 'shield-tick', revise_outline: 'routing-2', resolve_outline_feedback: 'tick-circle', save_book: 'document-text', save_foundation: 'lamp-on', save_review: 'chart', save_arc_summary: 'note-text', save_volume_summary: 'book-1' }
const clock = (at: string) => { const d = new Date(at); return Number.isNaN(d.valueOf()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) }
const button = { border: 0, background: 'transparent', color: 'var(--color-fg)', cursor: 'pointer' } as const

export function RunOperationBody({ operation }: { operation: LiveHistoryOperation }) {
  const [techOpen, setTechOpen] = useState(false)
  const { route } = useRoute()
  let params: { chapter?: number; type?: string } = {}
  try { const parsed: unknown = JSON.parse(operation.params); if (parsed && typeof parsed === 'object') params = parsed as typeof params } catch { /* Dữ liệu cũ có thể bị cắt. */ }
  const card = runCardModel(operation.key, operation.params, operation.result)
  return <div style={{ padding: '8px 14px 14px', fontSize: 13, color: 'var(--color-fg-2)' }}>
    {operation.status === 'running' ? <p>Bắt đầu lúc {clock(operation.startedAt)}. Thẻ tự cập nhật khi xong; không cần tải lại.</p> : operation.status === 'error' ? <p role="alert" style={{ padding: '12px 14px', borderRadius: 10, background: 'var(--tint-err)', color: 'var(--color-err)', lineHeight: 1.5 }}>{errorLabel(operation.result) ? operation.result : operation.error}</p> : card.blocks.map((block, index) => <div key={index} style={{ marginBottom: 10, ...(block.label === 'Trước' || block.label === 'Sau' ? { padding: 10, borderRadius: 8, background: block.label === 'Trước' ? 'var(--tint-err)' : 'var(--tint-ok)' } : {}) }}><span style={{ fontWeight: 600 }}>{block.label === 'Trước' ? '−' : block.label === 'Sau' ? '+' : block.label}</span><div style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', textDecoration: block.label === 'Trước' ? 'line-through' : undefined }}>{block.value}</div></div>)}
    <details onToggle={event => setTechOpen(event.currentTarget.open)}><summary>Chi tiết kỹ thuật</summary>{techOpen && <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'monospace' }}>{operation.key}{'\n'}{operation.params}{'\n'}{operation.result}{operation.error && `\n${operation.error}`}</pre>}</details>
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
    {mobile && <BottomSheet open={!!current} onClose={() => setSelected(null)} height="auto" style={{ maxHeight: 'min(760px, 90vh)' }} title={current ? runCardModel(current.key, current.params, current.result).name : ''}>{current && <div style={{ overflowY: 'auto', paddingBottom: 8 }}><div style={{ padding: '0 18px', color: 'var(--color-fg-4)', fontSize: 12 }}>{[roles[current.role], `vòng ${current.round}`, clock(current.startedAt), current.status === 'error' ? 'lỗi' : current.status === 'running' ? 'Đang chạy' : `${(current.durationMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} giây`].filter(Boolean).join(' · ')}</div><p style={{ padding: '0 18px', color: current.status === 'error' ? 'var(--color-err)' : 'var(--color-fg-3)', fontSize: 13 }}>{current.status === 'error' ? 'không thực hiện được · ' : ''}{runCardModel(current.key, current.params, current.result).summary}</p><RunOperationBody operation={current} /></div>}</BottomSheet>}
  </div>
  return embedded ? content : mobile ? <MobilePage title="Luồng viết" onBack={() => navigate(paths.workspace(project, story))} manageScroll={false}>{content}</MobilePage> : <section className="card" style={{ flex: '6 1 0', minHeight: 0 }}>{content}</section>
}
