// Bảng route → màn của Viderse. Tách khỏi App.tsx để khung điện thoại dựng
// được từng màn trong pane giữ-alive (`shell/MobileRouteStage.tsx`) mà không
// phải import App (vòng import App → stage → App).
// Mọi màn trừ Bàn dựng vào chunk riêng (React.lazy).
import { Suspense, lazy, type ReactNode } from 'react'
import { Spinner } from '@noverse/ui'
import { WorkbenchScreen as HomeScreen } from './workbench/WorkbenchScreen'
import { SystemLoading } from '../components'
import type { Route } from '../router'
import { AdminReadOnly } from '../components/AdminReadOnly'

// Cổng phiên (thiết lập, đăng nhập, đổi mật khẩu, bị chặn, mất máy chủ) chỉ mở
// một màn một lúc: để riêng chunk, người đã đăng nhập không tải mã của chúng.
const SetupScreen = lazy(() => import('./auth/SetupScreen').then((m) => ({ default: m.SetupScreen })))
const LoginScreen = lazy(() => import('./auth/LoginScreen').then((m) => ({ default: m.LoginScreen })))
const AccessDeniedScreen = lazy(() => import('./auth/AccessDeniedScreen').then((m) => ({ default: m.AccessDeniedScreen })))
const ChangePasswordScreen = lazy(() => import('./auth/ChangePasswordScreen').then((m) => ({ default: m.ChangePasswordScreen })))
const ServerDownScreen = lazy(() => import('./auth/ServerDownScreen').then((m) => ({ default: m.ServerDownScreen })))

const ReviewScreen = lazy(() => import('./review/ReviewScreen').then((m) => ({ default: m.ReviewScreen })))
const QueueScreen = lazy(() => import('./queue/QueueScreen').then((m) => ({ default: m.QueueScreen })))
const ChannelScreen = lazy(() => import('./channel/ChannelScreen').then((m) => ({ default: m.ChannelScreen })))
const ChannelNewScreen = lazy(() => import('./channel/ChannelNewScreen').then((m) => ({ default: m.ChannelNewScreen })))
const ExploreScreen = lazy(() => import('./explore/ExploreScreen').then((m) => ({ default: m.ExploreScreen })))
const NewScreen = lazy(() => import('./new/NewScreen').then((m) => ({ default: m.NewScreen })))
const LibraryScreen = lazy(() => import('./library/LibraryScreen').then((m) => ({ default: m.LibraryScreen })))
const TemplateScreen = lazy(() => import('./library/TemplateScreen').then((m) => ({ default: m.TemplateScreen })))
const EditorScreen = lazy(() => import('./editor/EditorScreen').then((m) => ({ default: m.EditorScreen })))
const RenderPage = lazy(() => import('../editor/pages/RenderPage').then((m) => ({ default: m.RenderPage })))
const MeasurePage = lazy(() => import('../editor/pages/MeasurePage').then((m) => ({ default: m.MeasurePage })))
const SettingsScreen = lazy(() => import('./settings/SettingsScreen').then((m) => ({ default: m.SettingsScreen })))
const MachineSetupScreen = lazy(() => import('./setup/SetupScreen'))
const AccountScreen = lazy(() => import('./account/AccountScreen').then((m) => ({ default: m.AccountScreen })))
const DevKitScreen = import.meta.env.DEV
  ? lazy(() => import('./dev/DevKitScreen').then((m) => ({ default: m.DevKitScreen })))
  : () => <HomeScreen route={{ name: 'workbench' }} />

/** Màn của một route, không kèm khung shell. */
export function RouteScreen({ route }: { route: Route }) {
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
      case 'setup': return <AdminReadOnly note="Chuẩn bị máy là việc của quản trị viên. Bạn đang xem ở chế độ chỉ đọc."><MachineSetupScreen /></AdminReadOnly>
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

// Các màn cổng phiên được App dựng trực tiếp (ngoài shell) — xuất để App
// không phải giữ bản sao của danh sách lazy.
export { SetupScreen, LoginScreen, AccessDeniedScreen, ChangePasswordScreen, ServerDownScreen }
