// Viderse — ứng dụng quản trị một máy chủ, tài khoản trên kho chung.
//   Boot: setKitLabels + applyViderseBrand ở main.tsx → SessionProvider hỏi
//   /api/me; cổng theo trạng thái phiên: chưa kiểm → chờ · store trống → Setup
//   · không trả lời được → lỗi + Thử lại · chưa đăng nhập → Login
//   · chưa cấp quyền Viderse → màn bị chặn · ép đổi mật khẩu → màn đổi mật khẩu
//   · còn lại → shell + route; mọi màn trừ Bàn dựng vào chunk riêng (React.lazy).
import { Suspense, lazy, useEffect, type ReactNode } from 'react'
import { Spinner, ToastHost } from '@noverse/ui'
import { SessionProvider, useSession } from './api/session'
import { AppShell } from './shell/AppShell'
import { SetupScreen } from './screens/auth/SetupScreen'
import { LoginScreen } from './screens/auth/LoginScreen'
import { AccessDeniedScreen } from './screens/auth/AccessDeniedScreen'
import { ChangePasswordScreen } from './screens/auth/ChangePasswordScreen'
import { ServerDownScreen } from './screens/auth/ServerDownScreen'
import { WorkbenchScreen as HomeScreen } from './screens/workbench/WorkbenchScreen'
import { SystemLoading } from './components'
import { startDataEvents } from './data/events'
import { navigate, routePath, useRoute, type Route } from './router'

const ReviewScreen = lazy(() => import('./screens/review/ReviewScreen').then((m) => ({ default: m.ReviewScreen })))
const QueueScreen = lazy(() => import('./screens/queue/QueueScreen').then((m) => ({ default: m.QueueScreen })))
const ChannelScreen = lazy(() => import('./screens/channel/ChannelScreen').then((m) => ({ default: m.ChannelScreen })))
const ChannelNewScreen = lazy(() => import('./screens/channel/ChannelNewScreen').then((m) => ({ default: m.ChannelNewScreen })))
const ExploreScreen = lazy(() => import('./screens/explore/ExploreScreen').then((m) => ({ default: m.ExploreScreen })))
const NewScreen = lazy(() => import('./screens/new/NewScreen').then((m) => ({ default: m.NewScreen })))
const LibraryScreen = lazy(() => import('./screens/library/LibraryScreen').then((m) => ({ default: m.LibraryScreen })))
const TemplateScreen = lazy(() => import('./screens/library/TemplateScreen').then((m) => ({ default: m.TemplateScreen })))
const EditorScreen = lazy(() => import('./screens/editor/EditorScreen').then((m) => ({ default: m.EditorScreen })))
const RenderPage = lazy(() => import('./editor/pages/RenderPage').then((m) => ({ default: m.RenderPage })))
const MeasurePage = lazy(() => import('./editor/pages/MeasurePage').then((m) => ({ default: m.MeasurePage })))
const SettingsScreen = lazy(() => import('./screens/settings/SettingsScreen').then((m) => ({ default: m.SettingsScreen })))
const MachineSetupScreen = lazy(() => import('./screens/setup/SetupScreen'))
const AccountScreen = lazy(() => import('./screens/account/AccountScreen').then((m) => ({ default: m.AccountScreen })))
const DevKitScreen = import.meta.env.DEV
  ? lazy(() => import('./screens/dev/DevKitScreen').then((m) => ({ default: m.DevKitScreen })))
  : () => <HomeScreen route={{ name: 'workbench' }} />

function Screen({ route }: { route: Route }) {
  const screen: ReactNode = (() => {
    switch (route.name) {
      case 'workbench': return <HomeScreen route={route} />
      case 'review': return <ReviewScreen />
      case 'queue': return <QueueScreen />
      case 'channel': return <ChannelScreen route={route} />
      case 'channel-new': return <ChannelNewScreen route={route} />
      case 'explore': return <ExploreScreen route={route} />
      case 'new': return <NewScreen route={route} />
      case 'library': return <LibraryScreen route={route} />
      case 'template': return <TemplateScreen route={route} />
      case 'editor': return <EditorScreen route={route} />
      case 'render': return <RenderPage requestId={route.requestId} token={route.token} />
      case 'measure': return <MeasurePage requestId={route.requestId} token={route.token} />
      case 'settings': return <SettingsScreen route={route} />
      case 'setup': return <MachineSetupScreen />
      case 'account': return <AccountScreen />
      case 'devkit': return <DevKitScreen route={route} />
    }
  })()
  // Skeleton 3 thẻ là của Bàn dựng (System artboard); màn khác chờ bằng
  // spinner giữa khung, không mượn nhãn “Đang tải Bàn dựng”.
  const fallback = route.name === 'workbench' ? <SystemLoading /> : (
    <div aria-busy="true" style={{ flex: '1 1 auto', minHeight: '50vh', display: 'grid', placeItems: 'center' }}>
      <Spinner size={18} color="var(--color-fg-3)" />
    </div>
  )
  return <Suspense fallback={fallback}>{screen}</Suspense>
}

function Root() {
  const parsed = useRoute()
  const { checked, user, setupNeeded, accessDenied, mustChangePassword, serverReachable } = useSession()

  // Chỉ mở shell và dữ liệu sau khi đã lưu mã khôi phục, có quyền vào app
  // và hoàn tất đổi mật khẩu tạm.
  const full = !!user && !setupNeeded && !accessDenied && !mustChangePassword

  // Hash không hợp lệ đã parse về Bàn dựng — chuẩn hoá luôn thanh địa chỉ
  // (replace, không đẩy entry lịch sử thừa). Không chạm hash khi đang ở một
  // cổng phiên (đổi mật khẩu bắt buộc không cho hash route vượt qua).
  useEffect(() => {
    if (!checked || !full) return
    const target = `#${routePath(parsed.route)}`
    if (location.hash !== target) navigate(routePath(parsed.route), true)
  }, [checked, full, parsed.route])

  // Gắn SSE + lớp dữ liệu chỉ khi phiên dùng đầy đủ — bị chặn quyền app hay
  // bị ép đổi mật khẩu đều không mount (server cũng 403 mọi route nghiệp vụ).
  useEffect(() => {
    if (!full) return
    return startDataEvents()
  }, [full])

  if (!checked) {
    return (
      <div style={{ minHeight: '100dvh', display: 'grid', placeItems: 'center' }} aria-busy="true">
        <Spinner size={18} color="var(--color-fg-3)" />
      </div>
    )
  }
  if (!serverReachable) return <ServerDownScreen />
  if (setupNeeded) return <SetupScreen />
  if (accessDenied) return <AccessDeniedScreen />
  if (mustChangePassword) return <ChangePasswordScreen />
  if (!user) return <LoginScreen />
  if (parsed.route.name === 'editor') return <Screen route={parsed.route} />
  return (
    <AppShell route={parsed.route} user={user}>
      <Screen route={parsed.route} />
    </AppShell>
  )
}

export default function App() {
  const { route } = useRoute()
  // Request-scoped credentials are independent of the human account session.
  if (route.name === 'render' || route.name === 'measure') return <Screen route={route} />
  return (
    <SessionProvider>
      <Root />
      <ToastHost activity />
    </SessionProvider>
  )
}
