import { useEffect, useRef, useState, type ReactNode } from 'react'
import { getOutline, getSnapshot } from '../api/client'
import { keys, useResource } from '../data'
import { helpTarget, rememberHelpTarget, subscribeHelpTarget } from '../commands/target'
import { navigate, paths, type Parsed } from '../router'
import { MobileFeedback, MobilePage } from '../mobile/primitives'
import { MobileRuntimeProvider } from '../mobile/runtime'
import { MobileWorkspace } from '../screens/workspace/mobile'
import MobileAudio from '../screens/audio/mobile'
import { MobileMore } from '../screens/more/mobile'
import { MobileActivity } from '../screens/activity/mobile'
import { HelpScreen } from '../screens/help'
import { useNotificationInbox } from '../screens/notify/useNotify'
import { NotifyPage } from '../screens/notify/NotifyPage'
import { useJobs } from '../shell/jobs/useJobs'
import { useDocumentTitle, useStoryName } from '../shell/useNames'
import { isGate } from '../screens/workspace/GateCard'
import { Icon } from '@noverse/ui'
import { BrandMark } from '@noverse/ui'
import { JobsPopover, JobsTrigger } from './TabletJobs'
import { storyMeta } from './logic'
import { TabletReader } from './TabletReader'
import '../mobile/mobile.css'
import './tablet.css'

export function TabletApp({ parsed }: { parsed: Parsed }) {
  return <MobileFeedback><MobileRuntimeProvider><TabletContent parsed={parsed} /></MobileRuntimeProvider></MobileFeedback>
}

function TabletContent({ parsed }: { parsed: Parsed }) {
  const { route, search } = parsed
  useDocumentTitle(route)
  const [target, setTarget] = useState(helpTarget)
  useEffect(() => subscribeHelpTarget(() => setTarget(helpTarget())), [])
  const [jobsOpen, setJobsOpen] = useState(false)
  const jobsRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const jobs = useJobs()
  const inbox = useNotificationInbox()
  const current = 'story' in route ? { project: route.project, story: route.story } : target
  const project = current?.project ?? ('project' in route ? route.project : null)
  const story = current?.story ?? null
  const snapshot = useResource(keys.story(project ?? '', story ?? '', 'snapshot'), ({ signal }) => getSnapshot(project!, story!, signal), { enabled: !!project && !!story })
  // Tên hiển thị (story.json.title); chưa tải xong thì dùng tên trong snapshot, không hiện slug thô.
  const storyTitle = useStoryName(project, story, snapshot.data?.bookTitle) || 'Noverse'
  const waiting = snapshot.data?.waitingFor === 'gate' || isGate(snapshot.data ?? null)
  const outline = useResource(keys.story(project ?? '', story ?? '', 'outline'), ({ signal }) => getOutline(project!, story!, signal), { enabled: !!project && !!story })
  const chapter = snapshot.data ? snapshot.data.inProgressChapter || snapshot.data.currentChapter || snapshot.data.completedCount : 0
  const volume = outline.data?.volumes.find((item) => item.chapters.some((entry) => entry.chapter === chapter))?.index
  const tab = route.name === 'reader' ? 1 : route.name === 'audio' ? 2 : ['workspace', 'activity', 'story'].includes(route.name) ? 0 : 3
  useEffect(() => {
    if ('story' in route) { rememberHelpTarget(); setTarget({ project: route.project, story: route.story }) }
    setJobsOpen(false)
  }, [route])
  useEffect(() => {
    if (!jobsOpen) return
    const pointer = (event: PointerEvent) => { if (!jobsRef.current?.contains(event.target as Node) && !triggerRef.current?.contains(event.target as Node)) setJobsOpen(false) }
    const key = (event: KeyboardEvent) => { if (event.key === 'Escape') { event.preventDefault(); setJobsOpen(false); triggerRef.current?.focus() } }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', key)
    return () => { document.removeEventListener('pointerdown', pointer); document.removeEventListener('keydown', key) }
  }, [jobsOpen])
  const tabs = [
    { label: 'Viết', icon: 'category', href: current ? paths.workspace(current.project, current.story) : '/' },
    { label: 'Đọc', icon: 'book-1', href: current ? paths.reader(current.project, current.story) : '/' },
    { label: 'Nghe', icon: 'headphone', href: current ? paths.audio(current.project, current.story, undefined, 'listen') : '/' },
    { label: 'Thêm', icon: 'setting-2', href: paths.settings('mobile') },
  ]
  const writeRead = !!project && !!story && ['workspace', 'story', 'reader'].includes(route.name)
  const morePage = (['home', 'projects', 'model', 'devices', 'password', 'guide'] as const).find((page) => page === search.get('page'))
  let screen: ReactNode
  if (writeRead) screen = <div className={`tablet-panes${tab === 1 ? ' read-focus' : ''}`}>
    <aside className="tablet-left" aria-label="Viết" inert={tab === 1 || undefined}><div className="tablet-left-content"><MobileWorkspace project={project!} story={story!} tablet /></div></aside>
    <TabletReader project={project!} story={story!} snapshot={snapshot.data ?? null} chapter={route.name === 'reader' ? route.chapter : null} focused={tab === 1} onRestoreWrite={() => navigate(paths.workspace(project!, story!))} />
  </div>
  else if (route.name === 'audio') screen = <div className="tablet-centered"><MobileAudio project={route.project} story={route.story} volume={route.volume} mode={route.mode} /></div>
  else if (route.name === 'activity') screen = <div className="tablet-centered"><MobileActivity project={route.project} story={route.story} /></div>
  else if (route.name === 'help') screen = <div className="tablet-centered"><HelpScreen cmd={route.cmd} /></div>
  else if (route.name === 'notifications') screen = <div className="tablet-centered"><NotifyPage initialView={search.get('view') ?? undefined} onBack={() => navigate(current ? paths.workspace(current.project, current.story) : paths.settings('mobile'))} /></div>
  else if (route.name === 'project' || route.name === 'settings') screen = <div className="tablet-centered"><MobileMore key={`${route.name}/${morePage ?? ''}/${search.get('cmd') ?? ''}`} project={project} story={story} initialPage={route.name === 'project' ? 'projects' : morePage} initialCommand={search.get('cmd') ?? undefined} onStoryChange={(chosen) => { rememberHelpTarget(`#${paths.workspace(chosen.project, chosen.story)}`); setTarget(chosen) }} /></div>
  else screen = <div className="tablet-centered"><MobilePage title="Trên máy tính"><div className="m-desktop-note"><Icon name="monitor" size={44} /><h2>Tiếp tục trên máy tính</h2><p>Dàn ý chi tiết, nhà cung cấp AI, thư viện và tạo truyện cần không gian lớn hơn. Bạn vẫn có thể theo dõi, duyệt chương và nghe trên máy tính bảng.</p></div></MobilePage></div>
  const unread = inbox.data?.unread ?? 0
  return <div className="tablet-app">
    <div className="tablet-safe" />
    <header className="tablet-topbar">
      <button type="button" className="tablet-story" onClick={() => navigate(paths.settings('mobile'))}><BrandMark size={20} /><span><strong>{storyTitle}</strong><small>{snapshot.data ? storyMeta(volume, snapshot.data.completedCount, snapshot.data.totalChapters) : story ? 'Đang tải…' : 'Chưa mở truyện nào'}</small></span></button>
      <nav className="tablet-tabs" aria-label="Điều hướng chính"><span className="tablet-indicator" style={{ left: `calc(4px + ${tab} * 94px)` }} aria-hidden="true" />{tabs.map((item, index) => <button type="button" className={`tablet-tab${tab === index ? ' selected' : ''}`} key={item.label} onClick={() => navigate(item.href)} aria-current={tab === index ? 'page' : 'false'}><Icon name={`${item.icon}${tab === index ? '-bulk' : ''}`} size={19} />{item.label}{index === 1 && waiting && <i className="tablet-dot" aria-label="Có việc cần bạn" />}</button>)}</nav>
      <div className="tablet-top-actions"><JobsTrigger ref={triggerRef} active={jobs.active} open={jobsOpen} onClick={() => setJobsOpen((open) => !open)} /><button type="button" className="tablet-bell" onClick={() => navigate('/notifications')} aria-label={`Thông báo${unread ? `, ${unread} chưa đọc` : ''}`}><Icon name="notification" size={21} />{unread > 0 && <i />}</button></div>
    </header>
    <main className="tablet-main">{screen}</main>
    {jobsOpen && <JobsPopover ref={jobsRef} jobs={jobs.jobs} loaded={jobs.state.loaded} story={current} onOpen={(job) => { setJobsOpen(false); jobs.actions.openHref(job.href) }} onLog={() => setJobsOpen(false)} />}
  </div>
}
