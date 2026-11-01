import { memo, useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useMediaQuery } from '../shell/AppShell'
import { useDocumentTitle } from '../shell/useNames'
import { setLastStory } from '../api/storage'
import { phoneLandingPath } from './landing'
import { navigate, paths, type Parsed } from '../router'
import { helpTarget, rememberHelpTarget, subscribeHelpTarget } from '../commands/target'
import { ApiError, canWriteStory, getAudioCast, getAudioQueue, getAudioReady, getAudioSettings, getAudioVolume, getChapter, getChapters, getConfig, getLibrary, getOutline, getProject, getSnapshot, mySessions } from '../api/client'
import { cache, keys, useResourceValue } from '../data'
import { useSession } from '../api/session'
import type { Snapshot } from '../api/types'
import { MobileReader } from '../screens/reader/mobile/MobileReader'
import { MobileBrain } from '../screens/brain/mobile/MobileBrain'
import { MobileOutline } from '../screens/project/MobileOutline'
import { ProjectBrainScreen } from '../screens/projbrain'
import MobileAudio from '../screens/audio/mobile'
import { getPosition } from '../screens/reader/mobile/preferences'
import { MobileWorkspace } from '../screens/workspace/mobile'
import { RunHistory } from '../screens/workspace/RunHistory'
import { MobileMore, MobileFirstPassword } from '../screens/more/mobile'
import { MobileNewStory } from '../screens/newstory/mobile'
import { MobileActivity } from '../screens/activity/mobile'
import { NotifyPage } from '../screens/notify/NotifyPage'
import { MOBILE_QUERY, TABLET_QUERY, showReviewDot, toastBottomFor, type LayoutMode } from './logic'
import { MobileFeedback } from './primitives'
import { MobileRuntimeProvider, useMobileRuntime, useTabMode } from './runtime'
import { MobilePanes, MobileTabBar, keepPane, moreStart, paneKeyOf, phoneRedirect, type PaneSnapshot } from './tabs'
import { useToastBottom } from './toastBottom'
import { useStorage } from '../screens/settings/storage/resource'
import './mobile.css'
export { BottomSheet, ActionSheet, ConfirmSheet, MobilePage, PushPage, useMobileToast } from './primitives'
export { MobileRuntimeProvider, useMobileRuntime, useTabMode, useMobileScroll } from './runtime'
export { useToastBottom } from './toastBottom'
export { shouldCloseSheet, mobileCommands } from './logic'
export function useMobileLayout(): boolean { return useMediaQuery(MOBILE_QUERY) }
export function useLayoutMode(): LayoutMode {
  const phone = useMediaQuery(MOBILE_QUERY)
  const tablet = useMediaQuery(TABLET_QUERY)
  return phone ? 'phone' : tablet ? 'tablet' : 'desktop'
}

/** Chỉ lần mở app đầu tiên (đã tải trang) mới nhảy thẳng vào truyện gần nhất; quay lại `#/` sau đó là chủ ý. */
let landingDone = false

/**
 * Nội dung một pane. `memo` theo ảnh chụp tham số: pane đang ẩn giữ nguyên ảnh chụp nên không
 * render lại khi đổi tab — chỉ pane vừa mở (ảnh chụp mới) và pane vừa ẩn (một nhịp) là chạy lại.
 */
const PaneView = memo(function PaneView({ id, pane, onStoryChange }: {
  id: string
  pane: PaneSnapshot
  onStoryChange: (target: { project: string; story: string }) => void
}) {
  const view = pane.parsed
  const route = view.route
  if (id === 'work' && route.name === 'workspace' && view.search.get('view') === 'run') return <RunHistory project={route.project} story={route.story} mobile />
  if (id === 'work' && (route.name === 'workspace' || route.name === 'story')) return <MobileWorkspace key={`${route.project}/${route.story}`} project={route.project} story={route.story} />
  if (id === 'read' && route.name === 'reader') return <MobileReader project={route.project} story={route.story} chapter={route.chapter} />
  if (id === 'audio' && route.name === 'audio') return <MobileAudio project={route.project} story={route.story} volume={route.volume} mode={route.mode} />
  const start = moreStart(view)
  return <MobileMore project={pane.project} story={pane.story} initialPage={start.initialPage} initialCommand={start.initialCommand} onStoryChange={route.name === 'project' ? undefined : onStoryChange} />
})

function MobileContent({ parsed }: { parsed: Parsed }) {
  const { route } = parsed
  useDocumentTitle(route)
  useStorage(false, true, true)
  const { user } = useSession()
  const runtime = useMobileRuntime()
  const [target, setTarget] = useState(helpTarget)
  const [passwordSkipped, setPasswordSkipped] = useState(false)
  useEffect(() => { setPasswordSkipped(false) }, [user?.username])
  useEffect(() => subscribeHelpTarget(() => setTarget(helpTarget())), [])
  useEffect(() => {
    if ('story' in route) { rememberHelpTarget(); setTarget({ project: route.project, story: route.story }); if (user) setLastStory(user.username, { project: route.project, story: route.story }) }
  }, [route, user])
  const username = user?.username
  const forcePassword = !!user?.mustChangePassword
  const atRoot = route.name === 'project' && !route.project
  useEffect(() => {
    if (landingDone || !username || forcePassword) return
    landingDone = true
    if (!atRoot) return
    void phoneLandingPath(username).then((path) => { if (path && location.hash.replace(/^#/, '') .split('?')[0] === '/') navigate(path, true) })
  }, [username, forcePassword, atRoot])
  const current = 'story' in route ? { project: route.project, story: route.story } : target
  const project = current?.project ?? ('project' in route ? route.project : null)
  const story = current?.story ?? null
  const canWrite = !!user && !!project && !!story && canWriteStory(user, `${project}/${story}`)
  const selectReviewDot = useCallback((snapshot: Snapshot | undefined) => showReviewDot(snapshot ?? null, user, `${project ?? ''}/${story ?? ''}`), [user, project, story])
  const needYou = useResourceValue(keys.story(project ?? '', story ?? '', 'snapshot'), ({ signal }) => getSnapshot(project ?? '', story ?? '', signal), selectReviewDot, { enabled: !!project && !!story })
  const firstPassword = Boolean(user?.mustChangePassword && !passwordSkipped)
  // M11 · Truyện mới là màn đẩy: không thanh tab, ‹ về nơi mở.
  const subpage = route.name === 'activity' || route.name === 'projbrain' || route.name === 'notifications' || route.name === 'newstory' || (route.name === 'settings' && route.section === 'notify')
  useTabMode(firstPassword || subpage ? 'hide' : undefined)
  useToastBottom(toastBottomFor(route.name === 'settings' && route.section === 'notify' ? 'notifications' : route.name))
  const tab = route.name === 'reader' ? 'read' : route.name === 'audio' ? 'audio' : ['workspace', 'activity', 'story'].includes(route.name) ? 'work' : 'more'
  // Tab đã mở được giữ mounted: quay lại là hiện lại đúng pane cũ (vị trí cuộn, state, dữ liệu SWR)
  // thay vì dựng lại màn hình từ đầu. Màn riêng (Hoạt động, Não, Thông báo…) phủ lên trên.
  const panes = useRef(new Map<string, PaneSnapshot>()).current
  const active = firstPassword ? null : paneKeyOf(parsed)
  if (active) keepPane(panes, active, parsed, project, story)
  // Route chỉ có ở bản máy tính → về trang Thêm ngay (replace) thay vì để màn “Trên máy tính”
  // hiện dưới thanh tab vừa bấm.
  useEffect(() => {
    const target = phoneRedirect(parsed)
    if (target) navigate(target, true)
  }, [parsed])
  // Dữ liệu cho tab sẽ mở (chapters + chương đang đọc của Đọc, tập/nguồn của Nghe,
  // snapshot/outline của Viết, thiết bị của Thêm) — nạp sau khung hình đầu để lần mở đầu tiên
  // của mỗi tab cũng vào thẳng dữ liệu thật, không qua skeleton.
  useEffect(() => {
    if (!project || !story) return
    const warm = () => {
      // Start the request inside prefetch so an active chapters request is deduplicated.
      cache.prefetch(keys.story(project, story, 'chapters'), async ({ signal }) => {
        const response = await getChapters(project, story, signal)
        const remembered = getPosition(project, story)?.chapter
        const current = response.chapters.some((item) => item.chapter === remembered) ? remembered! : response.chapters.filter((item) => item.completed || item.words > 0).at(-1)?.chapter
        if (current !== undefined) {
          const info = response.chapters.find((item) => item.chapter === current)
          const source = info?.hasDraft && !info.completed ? 'draft' : 'committed'
          cache.prefetch(keys.story(project, story, 'chapter', current, source), ({ signal }) => getChapter(project, story, current, source, signal))
        }
        return response
      })
      cache.prefetch(keys.story(project, story, 'snapshot'), ({ signal }) => getSnapshot(project, story, signal))
      cache.prefetch(keys.project(project, 'meta'), () => getProject(project))
      // Như useChapters: outline thiếu (404) nghĩa là truyện chưa có dàn ý → null, không phải lỗi.
      cache.prefetch(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal).catch((error) => { if (error instanceof ApiError && error.status === 404) return null; throw error }))
      cache.prefetch(keys.story(project, story, 'audio', 'volume', 1), ({ signal }) => getAudioVolume(project, story, 1, signal))
      cache.prefetch(keys.story(project, story, 'audio', 'cast'), ({ signal }) => getAudioCast(project, story, signal))
      cache.prefetch(keys.story(project, story, 'audio', 'settings'), ({ signal }) => getAudioSettings(project, story, signal))
      cache.prefetch(keys.story(project, story, 'audio', 'ready'), ({ signal }) => getAudioReady(project, story, signal))
      cache.prefetch(keys.story(project, story, 'audio', 'queue'), ({ signal }) => getAudioQueue(project, story, signal))
      cache.prefetch(keys.sessions(), () => mySessions())
      // Tab Thêm: thư viện (tên dự án/truyện ở đầu trang) + cấu hình truyện (phụ đề Quy tắc/Tuỳ chỉnh)
      // — thiếu chúng thì lần đầu mở Thêm hiện “Đang tải…” khoảng nửa giây.
      cache.prefetch(keys.library(), () => getLibrary())
      if (canWrite) cache.prefetch(keys.config('story', project, story), () => getConfig('story', project, story))
    }
    const idle = typeof window.requestIdleCallback === 'function'
    const handle: number = idle ? window.requestIdleCallback(warm, { timeout: 1500 }) : window.setTimeout(warm, 250)
    return () => { if (idle) window.cancelIdleCallback(handle); else window.clearTimeout(handle) }
  }, [project, story, canWrite])
  const chooseStory = useCallback((chosen: { project: string; story: string }) => { rememberHelpTarget(`#${paths.workspace(chosen.project, chosen.story)}`); setTarget(chosen) }, [])
  let screen: ReactNode = null
  if (firstPassword) screen = <MobileFirstPassword onDone={() => { setPasswordSkipped(true); navigate('/') }} onSkip={() => { setPasswordSkipped(true); navigate('/') }} />
  else if (route.name === 'activity') screen = <MobileActivity project={route.project} story={route.story} />
  // M8 · Não truyện: mở đúng chương đang đọc (`?ch=N`), không mở chương mới nhất; `?person=` mở sẵn tấm kéo lên.
  else if (route.name === 'brain') screen = <MobileBrain key={`${route.project}/${route.story}`} project={route.project} story={route.story} chapter={Number.parseInt(parsed.search.get('ch') ?? '', 10) || null} person={parsed.search.get('person')} />
  else if (route.name === 'story' && parsed.search.has('tab')) screen = <MobileOutline key={`${route.project}/${route.story}`} project={route.project} story={route.story} initialTab={parsed.search.get('tab')} />
  else if (route.name === 'projbrain') screen = <ProjectBrainScreen key={route.project} project={route.project} />
  else if (route.name === 'newstory') screen = <MobileNewStory key={route.project} project={route.project} />
  else if (route.name === 'notifications' || (route.name === 'settings' && route.section === 'notify')) screen = <NotifyPage initialView={parsed.search.get('view') ?? undefined} onBack={() => navigate(current ? paths.workspace(current.project, current.story) : '/')} />
  // Không còn nhánh “Trên máy tính”: mọi route đều thuộc một pane (paneKeyOf) hoặc là màn riêng
  // có `screen` ở trên, nên màn hình dự phòng chỉ còn là đường lộ ra khi thanh tab đã bấm.
  const tabs = [
    { key: 'work' as const, label: 'Viết', icon: 'category', href: current ? paths.workspace(current.project, current.story) : '/write', disabled: !story },
    { key: 'read' as const, label: 'Đọc', icon: 'book-1', href: current ? paths.reader(current.project, current.story) : '/', disabled: !story },
    { key: 'audio' as const, label: 'Nghe', icon: 'headphone', href: current ? paths.audio(current.project, current.story, undefined, 'listen') : '/', disabled: !story },
    { key: 'more' as const, label: 'Thêm', icon: 'setting-2', href: paths.settings('mobile') },
  ]
  const index = Math.max(0, tabs.findIndex((item) => item.key === tab))
  return <div className={`m-app${runtime.offline ? ' m-is-offline' : ''}`} data-tab-mode={runtime.tabMode} data-keyboard={runtime.keyboard > 0 ? 'open' : 'closed'}>
    <div className="m-content">
      <MobilePanes panes={[...panes].map(([id, pane]) => [id, <PaneView key={id} id={id} pane={pane} onStoryChange={chooseStory} />] as [string, ReactNode])} active={active} />
      {screen}
    </div>
    <MobileTabBar tabs={tabs} index={index} mode={runtime.tabMode} dot={needYou} />
  </div>
}
export function MobileApp({ parsed }: { parsed: Parsed }) {
  return <MobileFeedback><MobileRuntimeProvider><MobileContent parsed={parsed} /></MobileRuntimeProvider></MobileFeedback>
}
