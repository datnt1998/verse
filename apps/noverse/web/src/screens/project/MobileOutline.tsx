import { useState } from 'react'
import { Icon } from '@noverse/ui'
import { canWriteStory, getChapterPlan, getFrame, getOutline, getProjectLedger, getSnapshot, getStoryMeta, putFrame, putStoryMeta, sendInput } from '../../api/client'
import { useSession } from '../../api/session'
import type { StoryFile, StoryStatus } from '../../api/types'
import { invalidate, keys, setData, useResource } from '../../data'
import { foundationCopy as c } from '../../i18n/apFoundation'
import { BottomSheet, useMobileToast } from '../../mobile/primitives'
import { navigate, paths } from '../../router'
import { enqueueInput } from '../../mobile/outbox'
import { useMobileRuntime } from '../../mobile/runtime'
import { useCharacters } from './cast/data'
import { audienceLabels } from './retention/logic'
import { groupWorldRules } from './tabs/foundation'
import { useFoundation } from './tabs/useFoundation'
import { useForeshadow } from './tabs/ForeshadowTab'
import { chapterStatusLabel, isRewriteFlow } from './format'
import { CharAvatar } from '../../ui/CharAvatar'
import { roleTag } from './tabs/CastTab'
import { foreshadowActionLabel } from '../../lib/labels'

type Tab = 'info' | 'premise' | 'cast' | 'world' | 'outline' | 'fs'
type Field = 'title' | 'shortTitle' | 'status' | 'tags' | 'idea' | 'difference' | 'sequelSeed' | 'audience'
const tabs: [Tab, string][] = [['info', 'Thông tin'], ['premise', 'Tiền đề'], ['cast', 'Nhân vật'], ['world', 'Thế giới'], ['outline', 'Dàn ý'], ['fs', 'Cài cắm']]
const statuses: Record<StoryStatus, string> = { idea: 'Ý tưởng', writing: 'Đang viết', paused: 'Tạm ngưng', done: 'Hoàn thành', published: 'Đã đăng' }
const group = { margin: '0 16px', borderRadius: 16, background: 'var(--color-card)', border: '1px solid var(--line-1)', overflow: 'hidden' } as const
const row = { padding: '11px 14px', display: 'flex', flexDirection: 'column', gap: 3, borderBottom: '1px solid var(--line-1)', overflowWrap: 'anywhere' } as const
const label = { fontSize: 12.5, color: 'var(--color-fg-4)' } as const
const button = { border: 0, background: 'transparent', color: 'inherit', textAlign: 'left', font: 'inherit', width: '100%', cursor: 'pointer' } as const
const heading = { padding: '18px 30px 8px', fontSize: 12.5, color: 'var(--color-fg-4)' } as const
const action = { ...button, color: 'var(--color-accent)', padding: '0 14px', minHeight: 48, fontSize: 14.5 } as const

export function MobileOutline({ project, story, initialTab }: { project: string; story: string; initialTab: string | null }) {
  const tab: Tab = tabs.some(([id]) => id === initialTab) ? initialTab as Tab : 'info'
  const pick = (id: Tab) => navigate(`${paths.story(project, story)}?tab=${id}`)
  const foundation = useFoundation(project, story)
  const characters = useCharacters(project, story)
  const foreshadow = useForeshadow(project, story)
  const metaKey = keys.story(project, story, 'meta')
  const meta = useResource(metaKey, ({ signal }) => getStoryMeta(project, story, signal))
  const snapshot = useResource(keys.story(project, story, 'snapshot'), ({ signal }) => getSnapshot(project, story, signal))
  const outline = useResource(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal))
  const frameKey = keys.story(project, story, 'frame')
  const frame = useResource(frameKey, ({ signal }) => getFrame(project, story, signal))
  const ledger = useResource(keys.project(project, 'stories', 'ledger'), ({ signal }) => getProjectLedger(project, signal))
  const { user } = useSession()
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const notify = useMobileToast()
  const { offline } = useMobileRuntime()
  const [open, setOpen] = useState<number | null>(null)
  const plan = useResource(keys.story(project, story, 'plan', open), ({ signal }) => getChapterPlan(project, story, open!, signal), { enabled: open !== null })
  const [sheet, setSheet] = useState<'ask' | 'edit' | null>(null)
  const [field, setField] = useState<Field>('title')
  const [draft, setDraft] = useState('')
  const [audience, setAudience] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const ask = (text: string) => { if (!canEdit) return; setDraft(text); setError(''); setSheet('ask') }
  const edit = (name: Field, value: string) => { if (!canEdit) return; setField(name); setDraft(value); setAudience(frame.data?.audience ?? []); setError(''); setSheet('edit') }
  const save = async () => {
    if (busy) return
    setBusy(true); setError('')
    try {
      if (sheet === 'ask') { if (!draft.trim()) return; if (offline) { await enqueueInput({ project, story, text: draft.trim() }); notify('Góp ý sẽ gửi khi có mạng') } else { await sendInput(project, story, draft.trim()); notify('Đã gửi góp ý · Trọng tài sẽ xét') } void invalidate(keys.story(project, story, 'snapshot')) }
      else if (field === 'audience' || field === 'difference' || field === 'sequelSeed') {
        const result = await putFrame(project, story, field === 'audience' ? { audience } : { [field]: draft })
        setData(frameKey, result)
      } else if (meta.data) {
        const next: StoryFile = { ...meta.data, [field]: field === 'tags' ? draft.split(',').map(x => x.trim()).filter(Boolean) : draft }
        await putStoryMeta(project, story, next, meta.data.version)
        setData(metaKey, await getStoryMeta(project, story)); void invalidate(keys.project(project)); void invalidate(keys.library())
      }
      setSheet(null)
    } catch (e) { setError(e instanceof Error ? e.message : 'Không thực hiện được. Vui lòng thử lại.') } finally { setBusy(false) }
  }
  const missing = foundation.rows.filter(x => x.missing).length
  const chars = characters.data?.characters ?? []
  const chapters = outline.data?.volumes.flatMap(v => v.chapters) ?? []
  const rewrite = isRewriteFlow(chapters)
  const axes = ledger.data?.stories.find(x => x.id === story)?.values ?? (ledger.data?.candidate?.story === story ? ledger.data.candidate.values : [])
  const data = meta.data
  const title = data?.shortTitle || data?.title || 'Truyện'
  const info: { key: Field | 'synopsis'; label: string; value: string }[] = data ? [
    { key: 'title', label: 'Tên truyện', value: data.title }, { key: 'shortTitle', label: 'Tên ngắn', value: data.shortTitle ?? '' },
    { key: 'status', label: 'Trạng thái', value: statuses[data.status] }, { key: 'tags', label: 'Thẻ', value: data.tags.join(', ') },
    { key: 'synopsis', label: 'Tóm tắt · Kiến trúc sư viết', value: snapshot.data?.synopsis ?? '' }, { key: 'idea', label: 'Ý tưởng ban đầu', value: data.idea },
  ] : []
  const editTitle: Record<Field, string> = { title: 'Tên truyện', shortTitle: 'Tên ngắn', status: 'Trạng thái', tags: 'Thẻ', idea: 'Ý tưởng ban đầu', audience: 'Người nghe', difference: 'Điểm khác', sequelSeed: 'Hạt giống phần hai' }
  return <section className="m-page" style={{ minWidth: 0 }}>
    <header className="m-topbar"><button className="m-icon-button" type="button" aria-label="Quay lại Viết" onClick={() => navigate(paths.workspace(project, story))}><Icon name="arrow-left-2" size={22} /></button><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}><strong style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</strong><small style={label}>{snapshot.data?.statusLabel ? `${snapshot.data.statusLabel} · ` : ''}chương {snapshot.data?.inProgressChapter || snapshot.data?.currentChapter || snapshot.data?.completedCount || 0}/{snapshot.data?.totalChapters ?? '…'}</small></span><button className="m-icon-button" type="button" aria-label="Não truyện" onClick={() => navigate(paths.brain(project, story))}><Icon name="routing-2" size={21} /></button></header>
    <div role="tablist" aria-label="Phần của truyện" className="m-hscroll" style={{ padding: '0 16px 16px', flexShrink: 0, gap: 8 }}>{tabs.map(([id, text]) => { const count = id === 'cast' ? chars.length : id === 'world' ? foundation.world.data?.length : id === 'outline' ? chapters.length : id === 'fs' ? foreshadow.data?.entries.length : undefined; return <button key={id} role="tab" aria-selected={tab === id} type="button" className="m-chip" onClick={() => pick(id)} style={{ whiteSpace: 'nowrap', flexShrink: 0, ...(tab === id ? { background: 'var(--kit-acc-16)', borderColor: 'var(--kit-acc-45)', color: 'var(--color-fg)' } : {}) }}>{text}{!!count && <span style={{ marginLeft: 6, fontSize: 12, color: 'var(--color-fg-4)' }}>{count}</span>}</button> })}<button type="button" className="m-chip" onClick={() => navigate('/settings/mobile?page=rules')} style={{ whiteSpace: 'nowrap', flexShrink: 0 }}>Cách viết ›</button></div>
    <main className="m-scroll" style={{ overflowX: 'hidden', paddingBottom: 120 }}>
      {tab === 'info' && <>
        <div aria-label="Nền móng" style={{ display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 8, margin: '0 16px' }}>{([['premise', 'Tiền đề', foundation.ready ? missing ? `${foundation.rows.length - missing}/${foundation.rows.length} mục · thiếu ${missing}` : c.complete(foundation.rows.length) : 'Đang tải…', foundation.ready && missing === 0], ['cast', 'Nhân vật', `${chars.length} nhân vật`, chars.length > 0], ['world', 'Thế giới', foundation.world.data?.length ? `${foundation.world.data.length} quy tắc` : 'Chưa có quy tắc', !!foundation.world.data?.length], ['outline', 'Dàn ý', `${chapters.length} chương`, chapters.length > 0]] as [Tab, string, string, boolean][]).map(([id, name, sub, ok]) => <button key={id} type="button" onClick={() => pick(id)} style={{ ...button, minWidth: 0, minHeight: 58, borderRadius: 14, padding: '10px 12px', background: 'var(--color-card)', border: '1px solid var(--line-1)' }}><span style={{ display: 'flex', gap: 10, alignItems: 'center' }}><Icon name={ok ? 'tick-circle' : 'warning-2'} size={18} style={{ color: ok ? 'var(--color-ok)' : 'var(--color-warn)' }} /><span style={{ minWidth: 0, display: 'flex', flexDirection: 'column', fontSize: 14, lineHeight: 1.35 }}>{name}<small title={sub} style={{ ...label, fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{sub}</small></span></span></button>)}</div>
        <div style={heading}>Thông tin truyện</div><div style={group}>{info.map(item => <button type="button" key={item.key} style={{ ...button, ...row }} onClick={() => item.key === 'synopsis' ? ask(c.editSynopsis) : edit(item.key, item.key === 'status' ? data!.status : item.value)}><span style={label}>{item.label}</span><span>{item.value || '—'}</span></button>)}</div>
        <div style={heading}>Khung truyện</div><div style={group}>{frame.data && ([['audience', 'Người nghe', audienceLabels(frame.data)], ['difference', 'Điểm khác', frame.data.difference], ['sequelSeed', 'Hạt giống phần hai', frame.data.sequelSeed]] as [Field, string, string][]).map(([key, name, value]) => <button key={key} type="button" style={{ ...button, ...row }} onClick={() => edit(key, value)}><span style={label}>{name}</span><span>{value || '—'}</span></button>)}</div>
        <div style={heading}>Định hướng kết · Kiến trúc sư cập nhật sau mỗi cung</div><div style={group}>{[['Hướng kết', snapshot.data?.compassDirection || outline.data?.compassDirection], ['Tuyến còn mở · phải khép trước khi hoàn tất', snapshot.data?.openThreads?.join(' · ')], ['Quy mô dự kiến', snapshot.data?.compassScale || outline.data?.compassScale]].map(([name, value]) => <div key={name} style={row}><span style={label}>{name}</span><span>{value || '—'}</span></div>)}{canEdit && <button style={action} type="button" onClick={() => ask(c.editCompass)}>Nhờ Kiến trúc sư đổi định hướng</button>}</div>
        <div style={heading}>10 trục của truyện</div><div style={group}>{ledger.data?.axes.map((axis, i) => <div key={axis.n} style={{ ...row, flexDirection: 'row', justifyContent: 'space-between' }}><span style={label}>{axis.n} · {axis.name}</span><span>{axes[i] || '—'}</span></div>)}<button type="button" style={action} onClick={() => navigate(paths.projectBrain(project))}>So với các truyện khác ở Não dự án ›</button></div>
      </>}
      {tab === 'premise' && <><div style={{ margin: '0 16px 8px', ...label }}>{foundation.ready ? missing ? c.incomplete(missing, foundation.rows.length) : c.complete(foundation.rows.length) : 'Đang tải…'} · Kiến trúc sư viết</div><div style={group}>{foundation.rows.map((item, index, rows) => <div key={item.heading} style={{ ...row, gap: 4, padding: '12px 14px', borderBottom: index < rows.length - 1 ? '1px solid var(--line-1)' : 0 }}><span style={{ ...label, lineHeight: 1.4 }}>{item.heading}</span>{item.missing ? <span style={{ display: 'flex', gap: 8, fontSize: 14, lineHeight: 1.55, color: 'var(--color-warn)' }}><Icon name="warning-2" size={16} style={{ flex: 'none', marginTop: 3 }} />{c.missing}</span> : <span style={{ fontSize: 15, lineHeight: 1.6 }}>{item.text}</span>}{item.missing && canEdit && <button type="button" className="m-chip" style={{ alignSelf: 'flex-start', marginTop: 4 }} onClick={() => ask(c.supplementPremise(item.heading))}><Icon name="magic-star" size={15} />Nhờ bổ sung</button>}</div>)}</div>{canEdit && <div style={{ ...group, marginTop: 12 }}><button type="button" style={action} onClick={() => ask(c.editPremise)}><Icon name="magic-star" size={17} /> Nhờ Kiến trúc sư sửa tiền đề</button></div>}</>}
      {tab === 'cast' && <><div style={group}>{chars.map((person, index) => { const tag = roleTag(person.role); return <button key={person.name} type="button" style={{ ...button, display: 'flex', alignItems: 'center', gap: 12, minHeight: 64, padding: '10px 14px', borderBottom: index < chars.length - 1 ? '1px solid var(--line-1)' : 0 }} onClick={() => navigate(`${paths.brain(project, story)}?person=${encodeURIComponent(person.name)}`)}><CharAvatar name={person.name} ini={person.ini} kind={person.kind} size={38} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}><span style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}><strong title={person.name} style={{ fontSize: 15, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{person.name}</strong><span style={{ flexShrink: 0, height: 20, padding: '0 9px', borderRadius: 7, fontSize: 11, background: tag.background, color: tag.color }}>{tag.label}</span></span><span style={{ ...label, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{person.description}</span></span><Icon name="arrow-right-2" size={16} /></button> })}</div><p style={{ margin: '12px 30px 0', ...label }}>Chạm một nhân vật để xem quan hệ và xưng hô theo chương ở Não truyện.</p></>}
      {tab === 'world' && <>{groupWorldRules(foundation.world.data ?? []).map((g, index) => <div key={g.category}><div style={{ ...heading, display: 'flex', alignItems: 'center', gap: 8, padding: `${index ? 22 : 10}px 26px 8px` }}><Icon name={g.icon} size={14} /> {g.label} <span>{g.rules.length}</span></div><div style={group}>{g.rules.map((r, i) => <div key={i} style={{ ...row, gap: 4, padding: '12px 14px', borderBottom: i < g.rules.length - 1 ? '1px solid var(--line-1)' : 0 }}><span style={{ fontSize: 15, lineHeight: 1.55 }}>{r.rule}</span><span style={{ ...label, fontSize: 13, lineHeight: 1.5 }}>Giới hạn · {r.boundary || '—'}</span></div>)}</div></div>)}{foundation.world.data && !foundation.world.data.length && <div style={{ margin: '8px 16px 0', padding: '26px 20px', borderRadius: 18, border: '1px dashed var(--line-4)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, textAlign: 'center' }}><Icon name="global" size={26} style={{ color: 'var(--color-fg-4)' }} /><strong style={{ fontSize: 15.5 }}>{c.worldEmpty}</strong><span style={{ ...label, fontSize: 13.5, lineHeight: 1.55 }}>{c.worldEmptyHint}</span>{canEdit && <button type="button" className="m-button secondary" style={{ height: 44, padding: '0 18px', fontSize: 15, marginTop: 6 }} onClick={() => ask(c.worldRequest(snapshot.data?.completedCount ?? 0))}><Icon name="magic-star" size={17} />{c.askWorldCreate}</button>}</div>}{canEdit && !!foundation.world.data?.length && <div style={{ ...group, marginTop: 12 }}><button type="button" style={action} onClick={() => ask(c.editWorld)}><Icon name="magic-star" size={17} /> {c.askWorldEdit}</button></div>}</>}
      {tab === 'outline' && outline.data?.volumes.map(volume => <div key={volume.index}><div style={{ ...heading, paddingTop: 6 }}>Tập {volume.index} · {volume.chapters.length} chương</div><div style={group}>{volume.chapters.map((ch, index) => <div key={ch.chapter}><button type="button" aria-expanded={open === ch.chapter} style={{ ...button, display: 'flex', gap: 12, minHeight: 52, padding: '8px 14px', alignItems: 'center', borderBottom: index < volume.chapters.length - 1 && open !== ch.chapter ? '1px solid var(--line-1)' : 0 }} onClick={() => setOpen(open === ch.chapter ? null : ch.chapter)}><span style={{ ...label, width: 18, flex: 'none' }}>{ch.chapter}</span><span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 15 }}>{ch.title}</span><span style={{ ...label, whiteSpace: 'nowrap' }}>{chapterStatusLabel(ch.status, rewrite)}</span><Icon name="arrow-down-2" size={15} /></button>{open === ch.chapter && <div style={{ ...row, padding: '0 14px 13px 44px', gap: 8 }}><span style={{ ...label, lineHeight: 1.4 }}>Sự kiện chính</span><span style={{ fontSize: 14.5, lineHeight: 1.55 }}>{ch.coreEvent || '—'}</span>{plan.data?.hook && <span style={{ alignSelf: 'flex-start', height: 24, display: 'inline-flex', alignItems: 'center', padding: '0 9px', borderRadius: 7, background: 'var(--kit-info-12)', color: 'var(--color-info)', fontSize: 12 }}>Móc: {plan.data.hook}</span>}<a href={`#${paths.reader(project, story, ch.chapter)}`} style={{ color: 'var(--color-accent)', fontSize: 14 }}>Đọc chương {ch.chapter} ›</a></div>}</div>)}</div></div>)}
      {tab === 'fs' && <><p style={{ margin: '8px 30px 10px', ...label, fontSize: 13, lineHeight: 1.5 }}>Chi tiết đã gieo và cần nhắc lại hoặc giải đáp ở chương sau. Kiến trúc sư theo dõi và xếp lịch.</p><div style={group}>{foreshadow.data?.entries.map((entry, index, entries) => <div key={entry.id} style={{ ...row, gap: 4, padding: '12px 14px', borderBottom: index < entries.length - 1 ? '1px solid var(--line-1)' : 0 }}><span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><span style={{ height: 24, padding: '0 9px', borderRadius: 7, background: entry.status === 'advanced' ? 'var(--kit-warn-12)' : 'var(--kit-info-12)', color: entry.status === 'advanced' ? 'var(--color-warn)' : 'var(--color-info)', fontSize: 12, display: 'inline-flex', alignItems: 'center' }}>{foreshadowActionLabel(({ planted: 'plant', advanced: 'advance', resolved: 'resolve', abandoned: 'abandon' } as Record<string, string>)[entry.status] ?? entry.status)}</span><span style={label}>Chương {entry.planted_at}</span></span><span style={{ fontSize: 14.5, lineHeight: 1.55 }}>{entry.description || 'Chưa có mô tả'}</span></div>)}</div></>}
    </main>
    <BottomSheet open={sheet !== null} onClose={() => setSheet(null)} label={sheet === 'ask' ? 'Nhờ Kiến trúc sư' : editTitle[field]} height="auto">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: sheet === 'ask' ? '13.5px 16px 1.5px' : '12px 16px 1.5px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}><strong style={{ flex: 1, fontSize: 17 }}>{sheet === 'ask' ? 'Nhờ Kiến trúc sư' : editTitle[field]}</strong><button type="button" onClick={() => setSheet(null)} style={{ ...button, width: 'auto', color: 'var(--color-fg-3)', height: 40 }}>Huỷ</button></div>
        {sheet === 'ask' && <span style={label}>Gửi cho Kiến trúc sư · trả lời ở tab Viết</span>}
        {sheet === 'edit' && field === 'audience' ? frame.data?.audiences.map(a => <label key={a.n}><input type="checkbox" checked={audience.includes(a.n)} onChange={() => setAudience(old => old.includes(a.n) ? old.filter(n => n !== a.n) : [...old, a.n])} /> {a.label}</label>)
          : sheet === 'edit' && field === 'status' ? <select aria-label={editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)}>{Object.entries(statuses).map(([value, name]) => <option key={value} value={value}>{name}</option>)}</select>
          : sheet === 'edit' && (field === 'title' || field === 'shortTitle' || field === 'tags') ? <input aria-label={editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)} style={{ width: '100%', height: 46, padding: '0 14px', borderRadius: 13, background: 'var(--color-surface-0)', color: 'var(--color-fg)', border: '1px solid var(--line-4)', font: 'inherit' }} />
          : <textarea aria-label={sheet === 'ask' ? 'Lời nhờ Kiến trúc sư' : editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)} rows={sheet === 'ask' ? 4 : 2} style={{ width: '100%', height: sheet === 'ask' ? 118 : undefined, padding: 12, borderRadius: 13, background: 'var(--color-surface-0)', color: 'var(--color-fg)', border: '1px solid var(--line-4)', font: 'inherit', resize: 'none' }} />}
        {sheet === 'ask' && <span style={label}>Kiến trúc sư sửa nền móng rồi lưu; dàn ý chương chưa viết được chỉnh theo nếu cần.</span>}
        {sheet === 'edit' && field === 'title' && <span style={label}>Tên đầy đủ hiện ở thư viện, trình đọc và bản xuất. Tên ngắn đổi riêng ở mục Tên ngắn.</span>}
        {error && <span role="alert">{error}</span>}
        <button className="m-button" type="button" disabled={busy} onClick={() => void save()}>{sheet === 'ask' ? 'Gửi' : 'Lưu'}</button>
      </div>
    </BottomSheet>
  </section>
}
