import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { EmptyState, Icon, Skeleton, toast } from '@noverse/ui'
import { canWriteStory, getProjectLedger, getStoryMeta, putStoryMeta } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { Outline, Snapshot, StoryFile, StoryStatus, StorySummary } from '../../../api/types'
import { invalidate, keys, setData, useResource } from '../../../data'
import { foundationCopy as c } from '../../../i18n/apFoundation'
import { paths, useRoute } from '../../../router'
import type { TabId } from '../StoryView'
import { FrameCard } from '../retention/FrameCard'
import { AskButton } from './FoundationTabs'
import { foundationStyles as s } from './foundation.styles'
import type { FoundationResource } from './useFoundation'

type Editable = 'title' | 'shortTitle' | 'status' | 'tags' | 'idea'
const STATUS: Record<StoryStatus, string> = { idea: 'Ý tưởng', writing: 'Đang viết', paused: 'Tạm ngưng', done: 'Hoàn thành', published: 'Đã đăng' }
export function InfoTab({ project, summary, snapshot, outline, foundation, castCount, mainCount, onTab, onSteer }: {
  project: string; summary: StorySummary; snapshot: Snapshot | undefined; outline: Outline | null; foundation: FoundationResource;
  castCount: number; mainCount: number; onTab: (tab: TabId) => void; onSteer: (text: string) => void
}) {
  const story = summary.story
  const key = keys.story(project, story, 'meta')
  const meta = useResource(key, ({ signal }) => getStoryMeta(project, story, signal))
  const ledger = useResource(keys.project(project, 'stories', 'ledger'), ({ signal }) => getProjectLedger(project, signal))
  const { user } = useSession()
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const route = useRoute()
  const [editing, setEditing] = useState<Editable | null>(() => route.search.get('edit') === 'title' && canEdit ? 'title' : null)
  const [draft, setDraft] = useState<string | null>(null)
  const rename = route.search.get('tab') === 'info' && route.search.get('edit') === 'title'
  useEffect(() => { if (rename && canEdit) { setEditing('title'); setDraft(null) } }, [rename, canEdit])
  const [saving, setSaving] = useState(false)
  if (!meta.data) return meta.error ? <EmptyState icon="warning-2" title="Không tải được thông tin truyện" action={<button type="button" onClick={() => void meta.refresh()}>Thử lại</button>} /> : <Skeleton height={260} radius={16} />
  const data = meta.data
  const start = (field: Editable) => { setEditing(field); setDraft(field === 'tags' ? data.tags.join(', ') : data[field] ?? '') }
  const save = async () => {
    if (!editing || saving) return
    const value = draft ?? (editing === 'tags' ? data.tags.join(', ') : data[editing] ?? '')
    const next: StoryFile = { ...data, [editing]: editing === 'tags' ? value.split(',').map((tag) => tag.trim()).filter(Boolean) : value }
    setSaving(true)
    try {
      await putStoryMeta(project, story, next, data.version)
      setData(key, await getStoryMeta(project, story))
      setEditing(null); setDraft(null)
      void invalidate(keys.project(project)); void invalidate(keys.library())
      toast.ok('Đã lưu thông tin truyện')
    } catch (error) { toast.err(error instanceof Error ? error.message : 'Không lưu được thông tin truyện.') }
    finally { setSaving(false) }
  }
  const missing = foundation.required.filter((row) => row.missing).length
  const count = foundation.required.length
  const worldCount = foundation.world.data?.length
  const planned = outline?.volumes.reduce((sum, volume) => sum + volume.chapters.length, 0) ?? 0
  const strip: { tab: TabId; title: string; text: string; ok: boolean | null }[] = [
    { tab: 'premise', title: c.premise, text: !foundation.ready ? 'Đang tải…' : missing ? `${count - missing}/${count} mục · thiếu ${missing}` : c.complete(count), ok: foundation.ready ? missing === 0 : null },
    { tab: 'cast', title: 'Nhân vật', text: `${castCount} nhân vật · ${mainCount} chính`, ok: castCount > 0 },
    { tab: 'world', title: 'Thế giới', text: worldCount === undefined ? 'Đang tải…' : worldCount ? `${worldCount} quy tắc` : 'Chưa có quy tắc', ok: worldCount === undefined ? null : worldCount > 0 },
    { tab: 'outline', title: 'Dàn ý', text: `${planned}/${summary.totalChapters || planned} chương · ${outline?.volumes.length ?? 0} tập`, ok: planned > 0 },
  ]
  const rows: { key: Editable | 'synopsis'; label: string; ref?: string; value: string }[] = [
    { key: 'title', label: c.title, value: data.title }, { key: 'shortTitle', label: c.shortTitle, ref: c.shortRef, value: data.shortTitle ?? '' },
    { key: 'status', label: c.status, value: STATUS[data.status] }, { key: 'tags', label: c.tags, value: data.tags.join(', ') },
    { key: 'synopsis', label: c.synopsis, ref: c.synopsisRef, value: snapshot?.synopsis ?? '' }, { key: 'idea', label: c.idea, ref: c.ideaRef, value: data.idea },
  ]
  const axes = ledger.data?.stories.find((item) => item.id === story)?.values ?? (ledger.data?.candidate?.story === story ? ledger.data.candidate.values : [])
  const compass = [
    { label: c.direction, value: snapshot?.compassDirection ?? outline?.compassDirection ?? '' },
    { label: c.threads, value: '', items: snapshot?.openThreads ?? [], hint: c.threadsHint },
    { label: c.scale, value: snapshot?.compassScale ?? outline?.compassScale ?? '' },
  ]
  return <div {...stylex.props(s.root)}>
    <section className={`card ${stylex.props(s.strip).className}`} aria-label={c.foundation}>{strip.map((item) => <button key={item.tab} type="button" {...stylex.props(s.foundation)} onClick={() => onTab(item.tab)}><Icon name={item.ok === false ? 'warning-2' : 'tick-circle'} size={18} style={{ flexShrink: 0, color: item.ok === null ? 'var(--color-fg-4)' : item.ok ? 'var(--color-ok)' : 'var(--color-warn)' }} /><span {...stylex.props(s.foundationCopy)}><span {...stylex.props(s.foundationTitle)}>{item.title}</span><span title={item.text} {...stylex.props(s.foundationSub, item.ok === false && s.warn)}>{item.text}</span></span></button>)}</section>
    <section className={`card ${stylex.props(s.card).className}`} aria-labelledby="story-info-title"><div id="story-info-title" {...stylex.props(s.heading)}>{c.info}</div>
      {rows.map((row, index) => <div key={row.key} {...stylex.props(s.row, index === 0 && s.first)}>
        <span {...stylex.props(s.label)}>{row.label}{row.ref && <span {...stylex.props(s.small)}>{row.ref}</span>}</span>
        <div {...stylex.props(s.value)}>{editing === row.key ? row.key === 'status' ? <div role="radiogroup" aria-label={c.status} {...stylex.props(s.chipRow)}>{Object.entries(STATUS).map(([status, label]) => <button type="button" role="radio" key={status} aria-checked={(draft ?? data.status) === status} {...stylex.props(s.button, (draft ?? data.status) === status && s.badge)} onClick={() => setDraft(status)}>{label}</button>)}</div> : <><input autoFocus aria-label={row.label} {...stylex.props(s.input)} value={draft ?? (row.key === 'tags' ? data.tags.join(', ') : row.value)} maxLength={row.key === 'shortTitle' ? 32 : undefined} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void save(); if (event.key === 'Escape') setEditing(null) }} />{row.key === 'shortTitle' && <span {...stylex.props(s.small)}>{c.shortHint}</span>}</> : row.key === 'tags' ? <div {...stylex.props(s.chipRow)}>{data.tags.map((tag) => <span key={tag} {...stylex.props(s.chip)} title={tag}>{tag}</span>)}</div> : <span>{row.value || '—'}</span>}</div>
        <span>{row.key === 'synopsis' ? <AskButton small onClick={() => onSteer(c.editSynopsis)} /> : canEdit && (editing === row.key ? <button type="button" disabled={saving} {...stylex.props(s.button, s.save)} onClick={() => void save()}>{saving ? 'Đang lưu…' : 'Xong'}</button> : <button type="button" aria-label={`Sửa ${row.label}`} {...stylex.props(s.button)} onClick={() => start(row.key as Editable)}><Icon name="edit-2" size={13} />Sửa</button>)}</span>
      </div>)}
    </section>
    <FrameCard project={project} story={story} />
    <section className={`card ${stylex.props(s.card).className}`} aria-labelledby="story-compass-title"><div {...stylex.props(s.heading)}><span id="story-compass-title">{c.compass}</span><span {...stylex.props(s.spacer)} /><span {...stylex.props(s.small)}>{c.compassRef}</span></div>{compass.map((row, index) => <div key={row.label} {...stylex.props(s.row, index === 0 && s.first)}><span {...stylex.props(s.label)}>{row.label}</span><div {...stylex.props(s.value)}>{row.items ? <div {...stylex.props(s.chipRow)}>{row.items.length ? row.items.map((item) => <span key={item} {...stylex.props(s.chip)} title={item}>{item}</span>) : '—'}</div> : row.value || '—'}{row.hint && <span {...stylex.props(s.small)}>{row.hint}</span>}</div><AskButton small onClick={() => onSteer(c.editCompass)} /></div>)}</section>
    <section className={`card ${stylex.props(s.card).className}`} aria-labelledby="story-axes-title"><div {...stylex.props(s.heading)}><span id="story-axes-title">{c.axes}</span><span {...stylex.props(s.small)}>{c.axesHint}</span><span {...stylex.props(s.spacer)} /><a href={`#${paths.projectBrain(project)}`} {...stylex.props(s.link)}>{c.compare}</a></div><div {...stylex.props(s.axes)}>{ledger.data?.axes.map((axis, index) => <div key={axis.n} {...stylex.props(s.axis)}><span className={`num ${stylex.props(s.axisNumber).className}`}>{axis.n}</span><span {...stylex.props(s.axisLabel)}>{axis.name}</span><span>{axes[index] || '—'}</span></div>)}</div></section>
    <section className={`card ${stylex.props(s.builds).className}`} aria-label={c.builds}><span>{c.builds}</span>{data.builds?.length ? data.builds.map((build) => <span key={`${build.from}-${build.to}`} {...stylex.props(s.chip)}><span className="num">{build.version || 'Chưa rõ phiên bản'}</span>· chương {build.from}{build.to !== build.from ? `–${build.to}` : ''}</span>) : <span {...stylex.props(s.small)}>{c.noBuilds}</span>}<span {...stylex.props(s.spacer)} /><span {...stylex.props(s.small)}>{c.buildsHint}</span></section>
  </div>
}
