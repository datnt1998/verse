// App — ghép Session + Library + Router + AppShell + các screen stub.
// 401 toàn cục → lưu ainovel.next → #/login (chỉ ?expired=1 khi trình duyệt này từng có phiên).
import { useCallback, useEffect, useRef } from 'react'
import { setUnauthorizedHandler } from './api/client'
import { LibraryProvider } from './api/library'
import { SessionProvider, useSession } from './api/session'
import { loginPath, sessionExpired, setNextRoute } from './api/storage'
import { navigate, paths, useRoute, type Parsed, isPublicRoute, needsWelcomeRedirect } from './router'
import { AppShell } from './shell/AppShell'
import { AuthScreen, PairScreen } from './screens/auth'
import { AccessDeniedScreen } from './screens/auth/AccessDenied'
import { SetupScreen } from './screens/auth/setup'
import { OnboardScreen } from './screens/onboard'
import { ProjectScreen, StoryScreen } from './screens/project'
import { BrainScreen } from './screens/brain'
import { QualityScreen } from './screens/quality'
import { ProjectBrainScreen } from './screens/projbrain'
import { WorkspaceScreen } from './screens/workspace'
import { ActivityScreen } from './screens/activity'
import { ReaderScreen } from './screens/reader'
import { AudioScreen } from './screens/audio'
import { NewStoryScreen } from './screens/newstory'
import { HelpScreen } from './screens/help'
import { SettingsScreen } from './screens/settings'
import { SysBrainScreen } from './screens/sysbrain'
import { AccountDialogs } from './screens/account'
import { AppDialogs } from './screens/dialogs'
import { EmptyState, Spinner } from '@noverse/ui'
import { KitGallery } from './ui/KitGallery'
import { MobileApp, useLayoutMode } from './mobile'
import { DataProvider } from './data'
import { ToastHost } from '@noverse/ui'
import { TabletApp } from './tablet'
import { MobileAuth } from './screens/auth/mobile'
import { accountTransition, sessionGate } from './mobile/session'
import { DeepLink } from './mobile/deepLinks'
import { CommandDeepLink } from './mobile/commandRoute'
import { NotifyPage } from './screens/notify/NotifyPage'

export function Screen({ parsed }: { parsed: Parsed }) {
  const { route } = parsed
  switch (route.name) {
    case 'project': return <ProjectScreen project={route.project} />
    case 'projbrain': return <ProjectBrainScreen key={route.project} project={route.project} />
    case 'story': return <StoryScreen project={route.project} story={route.story} />
    case 'brain': return <BrainScreen project={route.project} story={route.story} />
    case 'quality': return <QualityScreen key={`${route.project}/${route.story}`} project={route.project} story={route.story} />
    case 'workspace': return <WorkspaceScreen project={route.project} story={route.story} />
    case 'activity': return <ActivityScreen project={route.project} story={route.story} />
    case 'reader': return <ReaderScreen project={route.project} story={route.story} chapter={route.chapter} />
    case 'audio': return <AudioScreen project={route.project} story={route.story} volume={route.volume} mode={route.mode} />
    case 'newstory': return <NewStoryScreen project={route.project} />
    case 'settings': return <SettingsScreen section={route.section} search={parsed.search} />
    case 'sysbrain': return <SysBrainScreen />
    case 'help': return <HelpScreen cmd={route.cmd} />
    case 'notifications': return <NotifyPage initialView={parsed.search.get('view') ?? undefined} onBack={() => navigate(paths.settings('notify'))} />
    default: return <EmptyState title="Không có trang này" hint="Đường dẫn không tồn tại hoặc chỉ dành cho bản phát triển." action={<a href="#/">Về thư viện</a>} />
  }
}

function Root() {
  const parsed = useRoute()
  const layout = useLayoutMode()
  const { route } = parsed
  const publicRoute = isPublicRoute(route)
  const { checked, user, deniedUser, setUser, setupNeeded } = useSession()
  // Trình duyệt này đã có phiên thành công trong vòng đời app chưa (/api/me OK hoặc vừa login)
  const hadSession = useRef(false)
  useEffect(() => { if (user) hadSession.current = true }, [user])

  // 401 ở bất cứ request nào → lưu route hiện tại, về màn đăng nhập. Chỉ báo "hết hạn"
  // khi trình duyệt này từng có phiên; 401 lần đầu ở trình duyệt sạch = chưa đăng nhập.
  const onUnauthorized = useCallback(() => {
    if (accountTransition.isActive()) return accountTransition.wait()
    if (publicRoute) { setUser(null); return }
    // Lần kiểm phiên đầu tiên: 401 của /api/me chỉ nghĩa "chưa đăng nhập" — hiệu ứng bên dưới
    // chọn #/setup (máy chủ chưa có tài khoản) hay #/login, nên không điều hướng ở đây.
    if (!checked) return
    if (layout !== 'desktop' && user) return sessionGate.request()
    if (!location.hash.startsWith('#/login')) {
      setNextRoute(location.hash || '#/')
      setUser(null)
      navigate(loginPath(sessionExpired(hadSession.current)), true)
    }
  }, [setUser, publicRoute, layout, checked, user])
  useEffect(() => {
    setUnauthorizedHandler(onUnauthorized)
    return () => setUnauthorizedHandler(null)
  }, [onUnauthorized])

  // Chưa đăng nhập → chỉ cho phép login/pair/setup; máy chủ chưa có tài khoản nào → #/setup (A0) thay vì Đăng nhập.
  useEffect(() => {
    if (!checked || user) return
    if (route.name === 'setup' || route.name === 'pair' || route.name === 'missing' || route.name === 'kit') return
    if (!isPublicRoute(route)) setNextRoute(location.hash || '#/')
    const target = setupNeeded ? '/setup' : '/login'
    if (route.name === 'login' && target === '/login') return
    navigate(target, true)
  }, [checked, user, route.name, setupNeeded])

  // Phone is a control surface: setup stays on desktop; invited users can enter after password change.
  useEffect(() => {
    if (!checked || !user) return
    if (layout === 'phone') {
      if (route.name === 'welcome' && !user.mustChangePassword) navigate('/', true)
    } else if (needsWelcomeRedirect(route, user)) navigate('/welcome', true)
  }, [checked, user, route.name, layout])

  if (import.meta.env.DEV && route.name === 'kit') return <KitGallery />
  if (!checked) {
    return <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Spinner size={22} /></div>
  }
  // Phiên hợp lệ nhưng chưa được cấp quyền app: màn chặn giữ người đang đăng
  // nhập — không render route/screen nào khác (user vẫn null nên dữ liệu/SSE
  // của DataProvider không mount, library không tải).
  if (deniedUser) return <AccessDeniedScreen />

  if (route.name === 'setup') return <SetupScreen />
  if (route.name === 'login') return layout === 'phone' ? <MobileAuth scan={parsed.search.get('scan') === '1'} /> : <AuthScreen expired={parsed.search.get('expired') === '1'} />
  if (route.name === 'pair') return layout === 'phone' ? <MobileAuth code={route.code} /> : <PairScreen code={route.code} />
  if (layout === 'phone' && user?.mustChangePassword) return <MobileApp parsed={parsed} />
  if (route.name === 'welcome') return <OnboardScreen step={route.step} />
  if (!user) return layout === 'phone' ? <MobileAuth /> : <AuthScreen />
  if (route.name === 'mobile-link') return <div className="m-app"><DeepLink parsed={parsed} /></div>
  if (route.name === 'command' || parsed.search.has('result')) return <CommandDeepLink parsed={parsed} />
  if (layout === 'phone') return <MobileApp parsed={parsed} />
  if (layout === 'tablet') return <TabletApp parsed={parsed} />

  return (
    <AppShell>
      <Screen parsed={parsed} />
      {parsed.dialog && ['qr', 'devices'].includes(parsed.dialog) && (
        <AccountDialogs dialog={parsed.dialog as 'qr' | 'devices'} />
      )}
      {parsed.dialog && !['qr', 'devices', 'locked'].includes(parsed.dialog) && (
        <AppDialogs dialog={parsed.dialog} route={route} />
      )}
    </AppShell>
  )
}
function DesktopConnectionBanner() {
  return useLayoutMode() === 'desktop' ? <ConnectionBanner /> : null
}


export default function App() {
  return (
    <SessionProvider>
      <DataProvider>
        <LibraryProvider>
          <Root />
        </LibraryProvider>
        <ToastHost />
        <DesktopConnectionBanner />
      </DataProvider>
    </SessionProvider>
  )
}

import { ConnectionBanner } from './ui/ConnectionBanner'