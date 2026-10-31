import * as stylex from '@stylexjs/stylex'
import { color, line, kit, tint, role } from '@noverse/ui/tokens.stylex'
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

const styles = stylex.create({
  page: { minWidth: 0 },
  headerCopy: { flex: 1, minWidth: 0, display: "flex", flexDirection: "column" },
  truncate: { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  label1: { fontSize: 12.5, color: color["--color-fg-4"] },
  tabs: { display: 'flex', overflowX: 'auto', scrollbarWidth: 'none', padding: '0 16px 16px', flexShrink: 0, gap: 8 },
  chip: { position: 'relative', height: 34, minHeight: 34, padding: '0 13px', borderRadius: 999, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-10'], backgroundColor: kit['--kit-surface-15'], color: kit['--kit-fg-note'], display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 6, whiteSpace: 'nowrap', fontSize: 13.5, flexShrink: 0, '::before': { content: '""', position: 'absolute', top: -5, right: 0, bottom: -5, left: 0 } },
  tabOn: { whiteSpace: "nowrap", flexShrink: 0, backgroundColor: kit["--kit-acc-16"], borderColor: kit["--kit-acc-45"], color: color["--color-fg"] },
  tabOff: { whiteSpace: "nowrap", flexShrink: 0 },
  tabCount: { marginLeft: 6, fontSize: 12, color: color["--color-fg-4"] },
  body: { flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden', scrollbarWidth: 'none', paddingBottom: 120 },
  grid: { display: "grid", gridTemplateColumns: "repeat(2,minmax(0,1fr))", gap: 8, margin: "0 16px" },
  gridItem: { borderWidth: 1, borderStyle: "solid", borderColor: line["--line-1"], backgroundColor: color["--color-card"], color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", minWidth: 0, minHeight: 58, borderRadius: 14, padding: "10px 12px" },
  gridContent: { display: "flex", gap: 10, alignItems: "center" },
  gridIconOn: { color: color["--color-ok"] },
  gridIconOff: { color: color["--color-warn"] },
  gridText: { minWidth: 0, display: "flex", flexDirection: "column", fontSize: 14, lineHeight: 1.35 },
  gridSub: { fontSize: 12, color: color["--color-fg-4"], overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  heading1: { padding: "18px 30px 8px", fontSize: 12.5, color: color["--color-fg-4"] },
  group1: { margin: "0 16px", borderRadius: 16, backgroundColor: color["--color-card"], borderWidth: 1, borderStyle: "solid", borderColor: line["--line-1"], overflow: "hidden" },
  infoRow: { borderWidth: 0, backgroundColor: "transparent", color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", padding: "11px 14px", display: "flex", flexDirection: "column", gap: 3, borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"], overflowWrap: "anywhere" },
  compassRow: { padding: "11px 14px", display: "flex", flexDirection: "column", gap: 3, borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"], overflowWrap: "anywhere" },
  compassAction: { borderWidth: 0, backgroundColor: "transparent", color: color["--color-accent"], textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", padding: "0 14px", minHeight: 48, fontSize: 14.5 },
  axisRow: { padding: "11px 14px", display: "flex", flexDirection: "row", gap: 3, borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"], overflowWrap: "anywhere", justifyContent: "space-between" },
  premiseHint: { margin: "0 16px 8px", fontSize: 12.5, color: color["--color-fg-4"] },
  premiseRowOn: { padding: "12px 14px", display: "flex", flexDirection: "column", gap: 4, borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"], overflowWrap: "anywhere" },
  premiseRowOff: { padding: "12px 14px", display: "flex", flexDirection: "column", gap: 4, borderBottomWidth: 0, overflowWrap: "anywhere" },
  premiseLabel: { fontSize: 12.5, color: color["--color-fg-4"], lineHeight: 1.4 },
  missingText: { display: "flex", gap: 8, fontSize: 14, lineHeight: 1.55, color: color["--color-warn"] },
  missingIcon: { flex: "none", marginTop: 3 },
  premiseText: { fontSize: 15, lineHeight: 1.6 },
  supplement: { alignSelf: "flex-start", marginTop: 4 },
  groupAction: { margin: "0 16px", borderRadius: 16, backgroundColor: color["--color-card"], borderWidth: 1, borderStyle: "solid", borderColor: line["--line-1"], overflow: "hidden", marginTop: 12 },
  castRowOn: { borderWidth: 0, backgroundColor: "transparent", color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", display: "flex", alignItems: "center", gap: 12, minHeight: 64, padding: "10px 14px", borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"] },
  castRowOff: { borderWidth: 0, backgroundColor: "transparent", color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", display: "flex", alignItems: "center", gap: 12, minHeight: 64, padding: "10px 14px", borderBottomWidth: 0 },
  castCopy: { flex: 1, minWidth: 0, display: "flex", flexDirection: "column", gap: 2 },
  castTitle: { display: "flex", alignItems: "center", gap: 6, minWidth: 0 },
  castName: { fontSize: 15, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  castTagPurple: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: kit["--kit-acc-12"], color: color["--color-accent-soft"] },
  castTagRed: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: tint["--tint-err"], color: color["--color-err"] },
  castTagRedSoft: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: role["--role-accomplice-bg"], color: role["--role-accomplice-fg"] },
  castTagAmber: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: tint["--tint-warn"], color: color["--color-warn"] },
  castTagBlue: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: tint["--tint-info"], color: color["--color-info"] },
  castTagNeutral: { flexShrink: 0, height: 20, padding: "0 9px", borderRadius: 7, fontSize: 11, backgroundColor: kit["--kit-wash-6"], color: color["--color-fg-3"] },
  castDescription: { fontSize: 12.5, color: color["--color-fg-4"], display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" },
  castHint: { margin: "12px 30px 0", fontSize: 12.5, color: color["--color-fg-4"] },
  worldHeadingOn: { padding: "22px 26px 8px", fontSize: 12.5, color: color["--color-fg-4"], display: "flex", alignItems: "center", gap: 8 },
  worldHeadingOff: { padding: "10px 26px 8px", fontSize: 12.5, color: color["--color-fg-4"], display: "flex", alignItems: "center", gap: 8 },
  worldRuleText: { fontSize: 15, lineHeight: 1.55 },
  worldBoundary: { fontSize: 13, color: color["--color-fg-4"], lineHeight: 1.5 },
  emptyWorld: { margin: "8px 16px 0", padding: "26px 20px", borderRadius: 18, borderWidth: 1, borderStyle: "dashed", borderColor: line["--line-4"], display: "flex", flexDirection: "column", alignItems: "center", gap: 8, textAlign: "center" },
  emptyIcon: { color: color["--color-fg-4"] },
  emptyTitle: { fontSize: 15.5 },
  emptyHint: { fontSize: 13.5, color: color["--color-fg-4"], lineHeight: 1.55 },
  emptyAction: { height: 44, padding: '0 18px', fontSize: 15, marginTop: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, borderWidth: 0, borderRadius: 14, backgroundColor: kit['--kit-wash-7'], color: color['--color-fg'], fontWeight: 600, whiteSpace: 'nowrap' },
  outlineHeading: { padding: "18px 30px 8px", fontSize: 12.5, color: color["--color-fg-4"], paddingTop: 6 },
  chapterRowOn: { borderWidth: 0, backgroundColor: "transparent", color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", display: "flex", gap: 12, minHeight: 52, padding: "8px 14px", alignItems: "center", borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"] },
  chapterRowOff: { borderWidth: 0, backgroundColor: "transparent", color: "inherit", textAlign: "left", font: "inherit", width: "100%", cursor: "pointer", display: "flex", gap: 12, minHeight: 52, padding: "8px 14px", alignItems: "center", borderBottomWidth: 0 },
  chapterNumber: { fontSize: 12.5, color: color["--color-fg-4"], width: 18, flex: "none" },
  chapterTitle: { flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 15 },
  chapterStatus: { fontSize: 12.5, color: color["--color-fg-4"], whiteSpace: "nowrap" },
  chapterDetail: { padding: "0 14px 13px 44px", display: "flex", flexDirection: "column", gap: 8, borderBottomWidth: 1, borderBottomStyle: "solid", borderBottomColor: line["--line-1"], overflowWrap: "anywhere" },
  chapterEvent: { fontSize: 14.5, lineHeight: 1.55 },
  hook: { alignSelf: "flex-start", height: 24, display: "inline-flex", alignItems: "center", padding: "0 9px", borderRadius: 7, backgroundColor: kit["--kit-info-12"], color: color["--color-info"], fontSize: 12 },
  readChapter: { color: color["--color-accent"], fontSize: 14 },
  fsHint: { margin: "8px 30px 10px", fontSize: 13, color: color["--color-fg-4"], lineHeight: 1.5 },
  fsMeta: { display: "flex", alignItems: "center", gap: 8 },
  fsTagOn: { height: 24, padding: "0 9px", borderRadius: 7, backgroundColor: kit["--kit-warn-12"], color: color["--color-warn"], fontSize: 12, display: "inline-flex", alignItems: "center" },
  fsTagOff: { height: 24, padding: "0 9px", borderRadius: 7, backgroundColor: kit["--kit-info-12"], color: color["--color-info"], fontSize: 12, display: "inline-flex", alignItems: "center" },
  sheetBodyOn: { display: "flex", flexDirection: "column", gap: 12, padding: "13.5px 16px 1.5px" },
  sheetBodyOff: { display: "flex", flexDirection: "column", gap: 12, padding: "12px 16px 1.5px" },
  sheetHeading: { display: "flex", alignItems: "center", gap: 10 },
  sheetTitle: { flex: 1, fontSize: 17 },
  sheetCancel: { borderWidth: 0, backgroundColor: "transparent", color: color["--color-fg-3"], textAlign: "left", font: "inherit", width: "auto", cursor: "pointer", height: 40 },
  sheetInput: { width: "100%", height: 46, padding: "0 14px", borderRadius: 13, backgroundColor: color["--color-surface-0"], color: color["--color-fg"], borderWidth: 1, borderStyle: "solid", borderColor: line["--line-4"], font: "inherit" },
  sheetTextOn: { width: "100%", height: 118, padding: 12, borderRadius: 13, backgroundColor: color["--color-surface-0"], color: color["--color-fg"], borderWidth: 1, borderStyle: "solid", borderColor: line["--line-4"], font: "inherit", resize: "none" },
  sheetTextOff: { width: "100%", padding: 12, borderRadius: 13, backgroundColor: color["--color-surface-0"], color: color["--color-fg"], borderWidth: 1, borderStyle: "solid", borderColor: line["--line-4"], font: "inherit", resize: "none" },
})

function roleStyle(tag: ReturnType<typeof roleTag>) {
  if (tag.background === roleTag('nữ chính').background) return styles.castTagPurple
  if (tag.background === roleTag('phản diện chính').background) return styles.castTagRed
  if (tag.background === roleTag('đồng phạm').background) return styles.castTagRedSoft
  if (tag.background === roleTag('đồng minh hài').background) return styles.castTagAmber
  if (tag.background === roleTag('nam chính phụ trợ').background) return styles.castTagBlue
  return styles.castTagNeutral
}

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
  return <section className={`m-page ${stylex.props(styles.page).className}`} >
    <header className="m-topbar"><button className="m-icon-button" type="button" aria-label="Quay lại Viết" onClick={() => navigate(paths.workspace(project, story))}><Icon name="arrow-left-2" size={22} /></button><span {...stylex.props(styles.headerCopy)}><strong {...stylex.props(styles.truncate)}>{title}</strong><small {...stylex.props(styles.label1)}>{snapshot.data?.statusLabel ? `${snapshot.data.statusLabel} · ` : ''}chương {snapshot.data?.inProgressChapter || snapshot.data?.currentChapter || snapshot.data?.completedCount || 0}/{snapshot.data?.totalChapters ?? '…'}</small></span><button className="m-icon-button" type="button" aria-label="Não truyện" onClick={() => navigate(paths.brain(project, story))}><Icon name="routing-2" size={21} /></button></header>
    <div role="tablist" aria-label="Phần của truyện" {...stylex.props(styles.tabs)}>{tabs.map(([id, text]) => { const count = id === 'cast' ? chars.length : id === 'world' ? foundation.world.data?.length : id === 'outline' ? chapters.length : id === 'fs' ? foreshadow.data?.entries.length : undefined; return <button key={id} role="tab" aria-selected={tab === id} type="button" {...stylex.props(styles.chip, tab === id ? styles.tabOn : styles.tabOff)} onClick={() => pick(id)}>{text}{!!count && <span {...stylex.props(styles.tabCount)}>{count}</span>}</button> })}<button type="button" {...stylex.props(styles.chip, styles.tabOff)} onClick={() => navigate('/settings/mobile?page=rules')}>Cách viết ›</button></div>
    <main {...stylex.props(styles.body)}>
      {tab === 'info' && <>
        <div aria-label="Nền móng" {...stylex.props(styles.grid)}>{([['premise', 'Tiền đề', foundation.ready ? missing ? `${foundation.rows.length - missing}/${foundation.rows.length} mục · thiếu ${missing}` : c.complete(foundation.rows.length) : 'Đang tải…', foundation.ready && missing === 0], ['cast', 'Nhân vật', `${chars.length} nhân vật`, chars.length > 0], ['world', 'Thế giới', foundation.world.data?.length ? `${foundation.world.data.length} quy tắc` : 'Chưa có quy tắc', !!foundation.world.data?.length], ['outline', 'Dàn ý', `${chapters.length} chương`, chapters.length > 0]] as [Tab, string, string, boolean][]).map(([id, name, sub, ok]) => <button key={id} type="button" onClick={() => pick(id)} {...stylex.props(styles.gridItem)}><span {...stylex.props(styles.gridContent)}><Icon name={ok ? 'tick-circle' : 'warning-2'} size={18} {...stylex.props(ok ? styles.gridIconOn : styles.gridIconOff)} /><span {...stylex.props(styles.gridText)}>{name}<small title={sub} {...stylex.props(styles.gridSub)}>{sub}</small></span></span></button>)}</div>
        <div {...stylex.props(styles.heading1)}>Thông tin truyện</div><div {...stylex.props(styles.group1)}>{info.map(item => <button type="button" key={item.key} {...stylex.props(styles.infoRow)} onClick={() => item.key === 'synopsis' ? ask(c.editSynopsis) : edit(item.key, item.key === 'status' ? data!.status : item.value)}><span {...stylex.props(styles.label1)}>{item.label}</span><span>{item.value || '—'}</span></button>)}</div>
        <div {...stylex.props(styles.heading1)}>Khung truyện</div><div {...stylex.props(styles.group1)}>{frame.data && ([['audience', 'Người nghe', audienceLabels(frame.data)], ['difference', 'Điểm khác', frame.data.difference], ['sequelSeed', 'Hạt giống phần hai', frame.data.sequelSeed]] as [Field, string, string][]).map(([key, name, value]) => <button key={key} type="button" {...stylex.props(styles.infoRow)} onClick={() => edit(key, value)}><span {...stylex.props(styles.label1)}>{name}</span><span>{value || '—'}</span></button>)}</div>
        <div {...stylex.props(styles.heading1)}>Định hướng kết · Kiến trúc sư cập nhật sau mỗi cung</div><div {...stylex.props(styles.group1)}>{[['Hướng kết', snapshot.data?.compassDirection || outline.data?.compassDirection], ['Tuyến còn mở · phải khép trước khi hoàn tất', snapshot.data?.openThreads?.join(' · ')], ['Quy mô dự kiến', snapshot.data?.compassScale || outline.data?.compassScale]].map(([name, value]) => <div key={name} {...stylex.props(styles.compassRow)}><span {...stylex.props(styles.label1)}>{name}</span><span>{value || '—'}</span></div>)}{canEdit && <button {...stylex.props(styles.compassAction)} type="button" onClick={() => ask(c.editCompass)}>Nhờ Kiến trúc sư đổi định hướng</button>}</div>
        <div {...stylex.props(styles.heading1)}>10 trục của truyện</div><div {...stylex.props(styles.group1)}>{ledger.data?.axes.map((axis, i) => <div key={axis.n} {...stylex.props(styles.axisRow)}><span {...stylex.props(styles.label1)}>{axis.n} · {axis.name}</span><span>{axes[i] || '—'}</span></div>)}<button type="button" {...stylex.props(styles.compassAction)} onClick={() => navigate(paths.projectBrain(project))}>So với các truyện khác ở Não dự án ›</button></div>
      </>}
      {tab === 'premise' && <><div {...stylex.props(styles.premiseHint)}>{foundation.ready ? missing ? c.incomplete(missing, foundation.rows.length) : c.complete(foundation.rows.length) : 'Đang tải…'} · Kiến trúc sư viết</div><div {...stylex.props(styles.group1)}>{foundation.rows.map((item, index, rows) => <div key={item.heading} {...stylex.props(index < rows.length - 1 ? styles.premiseRowOn : styles.premiseRowOff)}><span {...stylex.props(styles.premiseLabel)}>{item.heading}</span>{item.missing ? <span {...stylex.props(styles.missingText)}><Icon name="warning-2" size={16} {...stylex.props(styles.missingIcon)} />{c.missing}</span> : <span {...stylex.props(styles.premiseText)}>{item.text}</span>}{item.missing && canEdit && <button type="button" {...stylex.props(styles.chip, styles.supplement)} onClick={() => ask(c.supplementPremise(item.heading))}><Icon name="magic-star" size={15} />Nhờ bổ sung</button>}</div>)}</div>{canEdit && <div {...stylex.props(styles.groupAction)}><button type="button" {...stylex.props(styles.compassAction)} onClick={() => ask(c.editPremise)}><Icon name="magic-star" size={17} /> Nhờ Kiến trúc sư sửa tiền đề</button></div>}</>}
      {tab === 'cast' && <><div {...stylex.props(styles.group1)}>{chars.map((person, index) => { const tag = roleTag(person.role); return <button key={person.name} type="button" {...stylex.props(index < chars.length - 1 ? styles.castRowOn : styles.castRowOff)} onClick={() => navigate(`${paths.brain(project, story)}?person=${encodeURIComponent(person.name)}`)}><CharAvatar name={person.name} ini={person.ini} kind={person.kind} size={38} /><span {...stylex.props(styles.castCopy)}><span {...stylex.props(styles.castTitle)}><strong title={person.name} {...stylex.props(styles.castName)}>{person.name}</strong><span {...stylex.props(roleStyle(tag))}>{tag.label}</span></span><span {...stylex.props(styles.castDescription)}>{person.description}</span></span><Icon name="arrow-right-2" size={16} /></button> })}</div><p {...stylex.props(styles.castHint)}>Chạm một nhân vật để xem quan hệ và xưng hô theo chương ở Não truyện.</p></>}
      {tab === 'world' && <>{groupWorldRules(foundation.world.data ?? []).map((g, index) => <div key={g.category}><div {...stylex.props(index > 0 ? styles.worldHeadingOn : styles.worldHeadingOff)}><Icon name={g.icon} size={14} /> {g.label} <span>{g.rules.length}</span></div><div {...stylex.props(styles.group1)}>{g.rules.map((r, i) => <div key={i} {...stylex.props(i < g.rules.length - 1 ? styles.premiseRowOn : styles.premiseRowOff)}><span {...stylex.props(styles.worldRuleText)}>{r.rule}</span><span {...stylex.props(styles.worldBoundary)}>Giới hạn · {r.boundary || '—'}</span></div>)}</div></div>)}{foundation.world.data && !foundation.world.data.length && <div {...stylex.props(styles.emptyWorld)}><Icon name="global" size={26} {...stylex.props(styles.emptyIcon)} /><strong {...stylex.props(styles.emptyTitle)}>{c.worldEmpty}</strong><span {...stylex.props(styles.emptyHint)}>{c.worldEmptyHint}</span>{canEdit && <button type="button" {...stylex.props(styles.emptyAction)} onClick={() => ask(c.worldRequest(snapshot.data?.completedCount ?? 0))}><Icon name="magic-star" size={17} />{c.askWorldCreate}</button>}</div>}{canEdit && !!foundation.world.data?.length && <div {...stylex.props(styles.groupAction)}><button type="button" {...stylex.props(styles.compassAction)} onClick={() => ask(c.editWorld)}><Icon name="magic-star" size={17} /> {c.askWorldEdit}</button></div>}</>}
      {tab === 'outline' && outline.data?.volumes.map(volume => <div key={volume.index}><div {...stylex.props(styles.outlineHeading)}>Tập {volume.index} · {volume.chapters.length} chương</div><div {...stylex.props(styles.group1)}>{volume.chapters.map((ch, index) => <div key={ch.chapter}><button type="button" aria-expanded={open === ch.chapter} {...stylex.props(index < volume.chapters.length - 1 && open !== ch.chapter ? styles.chapterRowOn : styles.chapterRowOff)} onClick={() => setOpen(open === ch.chapter ? null : ch.chapter)}><span {...stylex.props(styles.chapterNumber)}>{ch.chapter}</span><span {...stylex.props(styles.chapterTitle)}>{ch.title}</span><span {...stylex.props(styles.chapterStatus)}>{chapterStatusLabel(ch.status, rewrite)}</span><Icon name="arrow-down-2" size={15} /></button>{open === ch.chapter && <div {...stylex.props(styles.chapterDetail)}><span {...stylex.props(styles.premiseLabel)}>Sự kiện chính</span><span {...stylex.props(styles.chapterEvent)}>{ch.coreEvent || '—'}</span>{plan.data?.hook && <span {...stylex.props(styles.hook)}>Móc: {plan.data.hook}</span>}<a href={`#${paths.reader(project, story, ch.chapter)}`} {...stylex.props(styles.readChapter)}>Đọc chương {ch.chapter} ›</a></div>}</div>)}</div></div>)}
      {tab === 'fs' && <><p {...stylex.props(styles.fsHint)}>Chi tiết đã gieo và cần nhắc lại hoặc giải đáp ở chương sau. Kiến trúc sư theo dõi và xếp lịch.</p><div {...stylex.props(styles.group1)}>{foreshadow.data?.entries.map((entry, index, entries) => <div key={entry.id} {...stylex.props(index < entries.length - 1 ? styles.premiseRowOn : styles.premiseRowOff)}><span {...stylex.props(styles.fsMeta)}><span {...stylex.props(entry.status === 'advanced' ? styles.fsTagOn : styles.fsTagOff)}>{foreshadowActionLabel(({ planted: 'plant', advanced: 'advance', resolved: 'resolve', abandoned: 'abandon' } as Record<string, string>)[entry.status] ?? entry.status)}</span><span {...stylex.props(styles.label1)}>Chương {entry.planted_at}</span></span><span {...stylex.props(styles.chapterEvent)}>{entry.description || 'Chưa có mô tả'}</span></div>)}</div></>}
    </main>
    <BottomSheet open={sheet !== null} onClose={() => setSheet(null)} label={sheet === 'ask' ? 'Nhờ Kiến trúc sư' : editTitle[field]} height="auto">
      <div {...stylex.props(sheet === 'ask' ? styles.sheetBodyOn : styles.sheetBodyOff)}>
        <div {...stylex.props(styles.sheetHeading)}><strong {...stylex.props(styles.sheetTitle)}>{sheet === 'ask' ? 'Nhờ Kiến trúc sư' : editTitle[field]}</strong><button type="button" onClick={() => setSheet(null)} {...stylex.props(styles.sheetCancel)}>Huỷ</button></div>
        {sheet === 'ask' && <span {...stylex.props(styles.label1)}>Gửi cho Kiến trúc sư · trả lời ở tab Viết</span>}
        {sheet === 'edit' && field === 'audience' ? frame.data?.audiences.map(a => <label key={a.n}><input type="checkbox" checked={audience.includes(a.n)} onChange={() => setAudience(old => old.includes(a.n) ? old.filter(n => n !== a.n) : [...old, a.n])} /> {a.label}</label>)
          : sheet === 'edit' && field === 'status' ? <select aria-label={editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)}>{Object.entries(statuses).map(([value, name]) => <option key={value} value={value}>{name}</option>)}</select>
          : sheet === 'edit' && (field === 'title' || field === 'shortTitle' || field === 'tags') ? <input aria-label={editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)} {...stylex.props(styles.sheetInput)} />
          : <textarea aria-label={sheet === 'ask' ? 'Lời nhờ Kiến trúc sư' : editTitle[field]} value={draft} onChange={e => setDraft(e.target.value)} rows={sheet === 'ask' ? 4 : 2} {...stylex.props(sheet === 'ask' ? styles.sheetTextOn : styles.sheetTextOff)} />}
        {sheet === 'ask' && <span {...stylex.props(styles.label1)}>Kiến trúc sư sửa nền móng rồi lưu; dàn ý chương chưa viết được chỉnh theo nếu cần.</span>}
        {sheet === 'edit' && field === 'title' && <span {...stylex.props(styles.label1)}>Tên đầy đủ hiện ở thư viện, trình đọc và bản xuất. Tên ngắn đổi riêng ở mục Tên ngắn.</span>}
        {error && <span role="alert">{error}</span>}
        <button className="m-button" type="button" disabled={busy} onClick={() => void save()}>{sheet === 'ask' ? 'Gửi' : 'Lưu'}</button>
      </div>
    </BottomSheet>
  </section>
}
