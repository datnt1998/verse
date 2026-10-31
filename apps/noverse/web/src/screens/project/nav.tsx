// Gói AG — điều hướng màn Dự án (Outline.dc.html › nav none/project/story/search):
// thẻ truyện + popover đổi truyện (cột trái và crumb thanh trên), popover đổi dự án,
// menu dự án + danh sách truyện gọn (Tổng quan dự án), mục lục của truyện đang mở.
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { useLibrary } from '../../api/library'
import type { Outline, ProjectDetail, StorySummary } from '../../api/types'
import { getSnapshot, revealFolder } from '../../api/client'
import { keys, useAction, useResource } from '../../data'
import { navigate, paths, setDialog, useRoute } from '../../router'
import { styleCopy, t } from '../../i18n/vi'
import { projectLabel, storyLabel } from '../../shell/labels'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { ContextMenu } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import { StoryContextMenu } from './bits'
import { isRewriteFlow, storyStatusText } from './format'
import { useCanReveal } from './useProject'
import { tocVolumeOf, useToc, type TocVolume } from './toc'
import './project.css'

const ACCENT = 'var(--color-accent)'
const FG4 = 'var(--color-fg-4)'
const FADE = 'var(--kit-fg-faint)'
const ICON = 'var(--kit-fg-icon)'
const LINE = 'var(--line-1)'

/** Điều hướng phím mũi tên trong popover (spec AG: điều hướng được bằng phím). */
export function navListKeys(root: RefObject<HTMLElement | null>) {
  return (event: KeyboardEvent) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const buttons = Array.from(root.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') ?? [])
    if (!buttons.length) return
    event.preventDefault(); event.stopPropagation()
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
    buttons[next]?.focus()
  }
}

const popoverStyle: CSSProperties = {
  position: 'absolute', top: 'calc(100% + 6px)', left: 0, zIndex: 50,
  transformOrigin: 'top left',
}
const poptStyle: CSSProperties = {
  display: 'flex', alignItems: 'center', gap: 10, width: '100%', minHeight: 44, padding: '6px 10px',
  border: 'none', borderRadius: 9, background: 'transparent', color: 'var(--color-fg)', textAlign: 'left',
}
const divider = <div role="separator" style={{ height: 1, margin: '4px 2px', background: LINE }} />

const statusDot = (story: StorySummary) =>
  story.running || story.status === 'writing' ? 'var(--color-accent)' : 'var(--ring-paused)'

function StoryOption({ project, story, active, onPick }: { project: string; story: StorySummary; active: boolean; onPick: () => void }) {
  const sub = storyStatusText(story, false, story.status === 'idea' ? 0 : story.completedChapters + 1)
  return (
    <button type="button" role="option" aria-selected={active} className={active ? 'pj-popt on' : 'pj-popt'} style={poptStyle} onClick={onPick}>
      <span aria-hidden="true" style={{ width: 8, height: 8, borderRadius: '50%', background: statusDot(story), flex: '0 0 auto', margin: '0 4px' }} />
      <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
        <span style={{ fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }} title={storyLabel(story)}>{storyLabel(story)}</span>
        <span style={{ fontSize: 12, color: ICON }}>{sub}</span>
      </span>
      {active && <Icon name="tick-circle" size={15} style={{ color: ACCENT }} />}
    </button>
  )
}

/** Popover “Trong ‘{dự án}’”: Tổng quan dự án · các truyện · Truyện mới trong dự án này. */
export function StoryPopover({ project, detail, active, storyView, close }: {
  project: string; detail: ProjectDetail | null; active: StorySummary | null; storyView: boolean; close: () => void
}) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) close() }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [close])
  useEscapeLayer(true, close)
  useEffect(() => { root.current?.querySelector<HTMLElement>('button')?.focus({ preventScroll: true }) }, [])
  const name = projectLabel(detail?.project, project)
  return (
    <div ref={root} className="kit-pop" role="dialog" aria-label={t.nav.changeStory} tabIndex={-1}
      onKeyDown={navListKeys(root)} style={{ ...popoverStyle, width: 320 }}>
      <div style={{ padding: '6px 8px 8px', fontSize: 12, color: ICON }}>{t.projnav.inProject(name)}</div>
      <button type="button" className={storyView ? 'pj-popt' : 'pj-popt on'} style={{ ...poptStyle, minHeight: 40 }}
        onClick={() => { close(); navigate(paths.project(project)) }}>
        <Icon name="category" size={16} style={{ color: ACCENT }} />
        <span style={{ flex: '1 1 auto', fontSize: 13.5 }}>{t.projnav.overview}</span>
        <span style={{ fontSize: 11.5, color: FADE }}>{t.projnav.overviewHint}</span>
      </button>
      {divider}
      <div role="listbox" aria-label={t.nav.newStory} style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {(detail?.stories ?? []).map((story) => (
          <StoryOption key={story.story} project={project} story={story} active={!!storyView && story.story === active?.story}
            onPick={() => { close(); navigate(paths.story(project, story.story)) }} />
        ))}
      </div>
      {divider}
      <a className="pj-popt" href={`#${paths.newstory(project)}`} style={{ ...poptStyle, minHeight: 36, color: 'var(--color-fg-2)', fontSize: 13, textDecoration: 'none' }}>
        <Icon name="add" size={15} />{t.projnav.newStoryIn}
      </a>
    </div>
  )
}

/**
 * Thẻ truyện trên cùng cột trái (bấm → popover đổi truyện — một cú bấm, không đóng tập nào).
 * Ở Tổng quan dự án thẻ hiện “Tổng quan dự án · {dự án} · N truyện”.
 */
export function StoryCard({ project, detail, active, storyView }: {
  project: string; detail: ProjectDetail; active: StorySummary | null; storyView: boolean
}) {
  const [open, setOpen] = useState(false)
  const wrap = useRef<HTMLDivElement>(null)
  const snapshot = useResource(keys.story(project, active?.story ?? '', 'snapshot'), ({ signal }) => getSnapshot(project, active!.story, signal), { enabled: storyView && !!active })
  const title = storyView && active ? storyLabel(active) : t.projnav.overview
  const sub = storyView && active
    ? `${storyStatusText(active, snapshot.data?.flow === 'rewriting', 0)} · ${active.volumes} tập · ${active.totalChapters} chương`
    : `${projectLabel(detail.project, project)} · ${detail.stories.length} truyện`
  return (
    <div ref={wrap} style={{ position: 'relative', flex: '0 0 auto' }}>
      <button type="button" className="pj-scard" aria-haspopup="listbox" aria-expanded={open} title={t.projnav.changeStory}
        onClick={() => setOpen(!open)}
        onKeyDown={(event) => { if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && !open) { event.preventDefault(); setOpen(true) } }}
        style={{
          width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', borderRadius: 12,
          border: '1px solid', borderColor: open ? 'var(--kit-acc-35)' : 'var(--line-3)', background: 'var(--color-surface-1)', color: 'var(--color-fg)', textAlign: 'left',
        }}>
        <span style={{
          width: 30, height: 30, borderRadius: 9, border: '1px solid var(--line-4)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
          background: storyView ? 'var(--nav-story-tile)' : 'var(--nav-project-tile)',
        }}>
          {storyView ? <AgentOrb agent="writer" size={10} live={!!active?.running} dim={!active?.running} /> : <Icon name="category" size={15} style={{ color: ACCENT }} />}
        </span>
        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
          <span style={{ fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{title}</span>
          <span style={{ fontSize: 11.5, color: ICON, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{sub}</span>
        </span>
        <Icon name="arrow-down-2" size={14} style={{ color: FG4 }} />
      </button>
      {open && <StoryPopover project={project} detail={detail} active={active} storyView={storyView} close={() => setOpen(false)} />}
    </div>
  )
}

/** Popover đổi dự án ở thanh trên (Tên dự án ▾) — chỗ duy nhất đổi dự án. */
export function ProjectPopover({ current, close }: { current: string | null; close: () => void }) {
  const { library } = useLibrary()
  const canReveal = useCanReveal()
  const reveal = useAction((p: string) => revealFolder({ target: 'project', project: p }), { success: 'Đã mở thư mục dự án' })
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) close() }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [close])
  useEscapeLayer(true, close)
  useEffect(() => { root.current?.querySelector<HTMLElement>('button')?.focus({ preventScroll: true }) }, [])
  const projects = library?.projects ?? []
  return (
    <div ref={root} className="kit-pop" role="dialog" aria-label={t.nav.changeProject} tabIndex={-1}
      onKeyDown={navListKeys(root)} style={{ ...popoverStyle, width: 340 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 8px 8px' }}>
        <span style={{ fontSize: 12, color: ICON }}>{t.projnav.projects(projects.length)}</span>
        <span style={{ fontSize: 11.5, color: FADE }}>{t.projnav.openOverview}</span>
      </div>
      <div role="listbox" aria-label={t.nav.project} style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {projects.map((p) => {
          const sel = p.id === current
          const tile = (
            <button type="button" role="option" aria-selected={sel} className={sel ? 'pj-popt on' : 'pj-popt'} style={poptStyle}
              onClick={() => { close(); navigate(paths.project(p.id)) }}>
              <span style={{
                width: 30, height: 30, borderRadius: 9, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
                background: sel ? 'var(--nav-project-tile)' : 'var(--color-card)', border: '1px solid var(--line-4)',
              }}>
                <Icon name="folder-open" size={15} style={{ color: sel ? ACCENT : ICON }} />
              </span>
              <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
                <span style={{ fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{projectLabel(p, p.id)}</span>
                <span style={{ fontSize: 12, color: ICON }}>
                  {p.storyCount} truyện{p.runningCount > 0 ? ` · ${p.runningCount} đang viết` : ''}
                </span>
              </span>
              {sel && <Icon name="tick-circle" size={15} style={{ color: ACCENT }} />}
            </button>
          )
          return (
            <ContextMenu key={p.id} style={{ gap: 0 }} items={[
              { id: 'open', label: 'Mở dự án', icon: <Icon name="folder-open" size={16} />, onSelect: () => { close(); navigate(paths.project(p.id)) } },
              { id: 'reveal', label: 'Mở thư mục', icon: <Icon name="folder-open" size={16} />, disabled: !canReveal || reveal.busy, tooltip: !canReveal ? 'Chỉ mở thư mục trên máy chủ' : undefined, onSelect: () => { void reveal.run(p.id) } },
              { id: 'rename', label: 'Đổi tên', icon: <Icon name="edit-2" size={16} />, disabled: !!p.legacy, onSelect: () => { close(); navigate(paths.project(p.id)); requestAnimationFrame(() => document.getElementById('pname')?.focus()) } },
            ]}>
              {tile}
            </ContextMenu>
          )
        })}
      </div>
      {divider}
      <button type="button" className="pj-popt" style={{ ...poptStyle, minHeight: 36, color: 'var(--color-fg-2)', fontSize: 13 }}
        onClick={() => { close(); setDialog('newProject') }}>
        <Icon name="add" size={15} />{t.projnav.newProject}
      </button>
    </div>
  )
}

function VolumeHeader({ volume, open, onToggle, onNext }: { volume: TocVolume; open: boolean; onToggle: () => void; onNext: () => void }) {
  const chevron = volume.locked ? 'lock' : 'arrow-down-2'
  return (
    <button type="button" className="pj-vhd" aria-expanded={volume.locked ? undefined : open} disabled={volume.locked}
      title={volume.next ? undefined : volume.label}
      onClick={volume.next ? onNext : onToggle}
      style={{ opacity: volume.locked ? 0.6 : 1 }}>
      <Icon name={chevron} size={12} style={{ color: ICON, transform: volume.locked || volume.next ? undefined : `rotate(${open ? 0 : -90}deg)`, transition: 'transform .2s' }} />
      <span style={{ flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{volume.label}</span>
      <span className="num" style={{ fontSize: 11.5, color: volume.live ? 'var(--color-ok)' : FG4 }}>{volume.meta}</span>
    </button>
  )
}

function ChapterRow({ n, title, live, done, doneLabel, untitled, current, onPick, flat, vol }: {
  n: number; title: string; live: boolean; done: boolean; doneLabel: string; untitled?: boolean
  current: boolean; onPick: () => void; flat?: boolean; vol?: string
}) {
  return (
    <button type="button" className="pj-crow2" aria-current={current || undefined} data-ch={n}
      title={`Chương ${n} · ${title}`} onClick={onPick}
      style={flat ? { paddingLeft: 8 } : undefined}>
      <span className="num" style={{ width: flat ? 28 : 24, color: FG4 }}>{n}</span>
      <span style={{
        flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        color: untitled ? FG4 : done || live ? 'var(--kit-fg-note)' : 'var(--color-fg-3)', fontWeight: live ? 600 : 400,
      }}>{title || '—'}</span>
      {done && <Icon name="tick-circle" size={13} label={doneLabel} style={{ color: 'var(--color-ok)' }} />}
      {live && <AgentOrb agent="writer" size={9} live label="Đang viết" style={{ marginRight: 2 }} />}
      {vol && <span style={{ flex: '0 0 auto', fontSize: 11, color: 'var(--kit-fg-faint)' }}>{vol}</span>}
    </button>
  )
}

/** Mục lục của truyện đang mở (cột trái, view story): tìm · tới chương đang viết · thu gọn · tập dính. */
export function StoryToc({ project, summary, outline }: { project: string; summary: StorySummary; outline: Outline | null }) {
  const story = summary.story
  const route = useRoute()
  const [q, setQ] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const snapshot = useResource(keys.story(project, story, 'snapshot'), ({ signal }) => getSnapshot(project, story, signal))
  const current = snapshot.data?.inProgressChapter ?? 0
  const { volumes, hits, openVolume, setOpenVolume, loadingHeaders, loadingVolume } = useToc(project, summary, storyLabel(summary), outline, current, q)
  const allChapters = volumes.flatMap((volume) => volume.chapters)
  const rewriteFlow = isRewriteFlow(allChapters)
  const searching = q.trim() !== ''
  const chParam = Number(route.search.get('ch') ?? '') || 0
  const anyOpen = volumes.some((v) => v.index === openVolume)
  const liveVolume = current > 0 ? tocVolumeOf(volumes, current) || volumes.find((v) => v.live)?.index || 0 : 0

  const pickChapter = (n: number) => navigate(`${paths.story(project, story)}?ch=${n}`)
  const goLive = () => {
    if (current <= 0) return
    setQ('')
    const volIndex = tocVolumeOf(outline?.volumes ?? [], current) || liveVolume || openVolume
    if (volIndex) setOpenVolume(volIndex)
    pickChapter(current)
    setTimeout(() => listRef.current?.querySelector(`[data-ch="${current}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' }), 80)
  }
  const collapse = () => setOpenVolume(anyOpen ? null : liveVolume || volumes.find((v) => !v.locked)?.index || null)

  return (
    <>
      <div style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 4, padding: '6px 2px 0 8px' }}>
        <span style={{ flex: '1 1 auto', fontSize: 11, fontWeight: 600, letterSpacing: '.08em', color: FADE }}>
          {t.projnav.toc} · <span className="num">{summary.totalChapters} ch.</span>
        </span>
        <button type="button" className="pj-ib" title={t.projnav.goLive} aria-label={t.projnav.goLive} disabled={current <= 0} onClick={goLive}>
          <Icon name="edit-2" size={15} />
        </button>
        <button type="button" className="pj-ib" title={anyOpen ? t.projnav.collapseAll : t.projnav.openLiveVolume}
          aria-label={anyOpen ? t.projnav.collapseAll : t.projnav.openLiveVolume} onClick={collapse}>
          <Icon name="layer" size={15} />
        </button>
      </div>
      <label className="pj-tocq" style={{ flex: '0 0 auto' }}>
        <Icon name="search-normal-1" size={14} style={{ color: FG4 }} />
        <input type="search" value={q} onChange={(event) => setQ(event.target.value)} placeholder={t.projnav.searchPlaceholder} aria-label={t.projnav.searchLabel}
          style={{ flex: '1 1 auto', minWidth: 0, border: 'none', outline: 'none', background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5 }} />
      </label>
      <div ref={listRef} className="scroll" style={{ flex: '1 1 auto', minHeight: 0, margin: '0 -4px', padding: '0 4px' }}>
        {searching ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 1, paddingTop: 4 }}>
            <span style={{ padding: '2px 8px 6px', fontSize: 11.5, color: FG4 }}>
              {hits.length ? t.projnav.found(hits.length) : t.projnav.noMatch(q.trim())}
            </span>
            {hits.map((hit) => (
              <ChapterRow key={hit.n} flat n={hit.n} title={hit.title} live={false} done={false} doneLabel="" vol={hit.volumeLabel}
                current={chParam === hit.n} onPick={() => pickChapter(hit.n)} />
            ))}
          </div>
        ) : loadingHeaders ? (
          <div style={{ padding: 8, display: 'flex', flexDirection: 'column', gap: 8 }}><Skeleton height={28} /><Skeleton height={28} /><Skeleton height={28} /></div>
        ) : volumes.map((volume) => {
          const open = volume.index === openVolume && !volume.locked && !volume.next
          return (
            <div key={volume.next ? 'next' : volume.index} style={{ paddingTop: 4 }}>
              <VolumeHeader volume={volume} open={open}
                onToggle={() => setOpenVolume(open ? null : volume.index)}
                onNext={() => setDialog('reopen')} />
              {open && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {loadingVolume && volume.chapters.length === 0
                    ? <div style={{ padding: '4px 8px' }}><Skeleton height={24} /><div style={{ marginTop: 4 }}><Skeleton height={24} /></div></div>
                    : volume.chapters.map((c) => (
                      <ChapterRow key={c.chapter} n={c.chapter} title={c.title || `Chương ${c.chapter}`}
                        untitled={!c.title.trim() && !rewriteFlow}
                        live={c.status === 'writing' || c.status === 'rewriting'}
                        done={c.completed} doneLabel={rewriteFlow ? 'Đã viết lại' : 'Hoàn thành'}
                        current={chParam === c.chapter} onPick={() => pickChapter(c.chapter)} />
                    ))}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </>
  )
}

/** Menu dự án + “TRUYỆN · n” danh sách gọn (view project). */
export function ProjectNav({ project, detail }: { project: string; detail: ProjectDetail }) {
  const ideaCount = detail.ideas.length
  const assetCount = detail.assets.filter((asset) => asset.kind === 'file').length
  const goSection = (id: string) => {
    setTimeout(() => document.getElementById(id)?.scrollIntoView({ block: 'start', behavior: 'smooth' }), 0)
  }
  const nav = [
    { id: 'overview', label: t.projnav.overviewShort, icon: 'category', n: '', go: () => goSection('overview') },
    { id: 'ideas', label: 'Ý tưởng', icon: 'lamp-on', n: String(ideaCount), go: () => goSection('ideas') },
    { id: 'assets', label: 'Tư liệu', icon: 'box', n: String(assetCount), go: () => goSection('assets') },
    { id: 'pstyle', label: styleCopy.tab, icon: 'brush-2', n: '', go: () => goSection('pstyle') },
  ]
  return (
    <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      {nav.map((item) => {
        const on = item.id === 'overview'
        return (
          <button key={item.id} className={`pj-sb${on ? ' on' : ''}`} type="button" onClick={item.go} aria-current={on ? 'page' : undefined}>
            <Icon name={item.icon} size={16} style={{ color: on ? ACCENT : ICON }} />
            <span style={{ flex: '1 1 auto' }}>{item.label}</span>
            <span className="num" style={{ fontSize: 12, color: FG4 }}>{item.n}</span>
          </button>
        )
      })}
      <a className="pj-sb" href={`#${paths.projectBrain(project)}`} title={t.projbrain.sidebar}>
        <Icon name="routing-2" size={16} style={{ color: 'var(--kit-fg-icon)' }} />
        <span style={{ flex: '1 1 auto' }}>{t.projbrain.title}</span>
      </a>
      <div className="pj-sh">{t.projnav.stories} · {detail.stories.length}</div>
      {detail.stories.map((story) => (
        <StoryContextMenu key={story.id} project={project} story={story}>
          <button className="pj-sb" type="button" title={storyLabel(story)} onClick={() => navigate(paths.story(project, story.story))}>
            <span aria-hidden="true" style={{ width: 8, height: 8, borderRadius: '50%', background: statusDot(story), flex: '0 0 auto', margin: '0 4px' }} />
            <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
              <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{storyLabel(story)}</span>
              <span style={{ fontSize: 11.5, color: FG4 }}>
                {storyStatusText(story, false, story.status === 'idea' ? 0 : story.completedChapters + 1)}
              </span>
            </span>
          </button>
        </StoryContextMenu>
      ))}
      <a className="pj-sb" href={`#${paths.newstory(project)}`} style={{ color: 'var(--color-fg-3)' }}>
        <Icon name="add" size={15} />{t.projnav.newStory}
      </a>
    </div>
  )
}
