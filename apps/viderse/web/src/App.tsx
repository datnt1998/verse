// Viderse — ứng dụng quản trị một máy chủ, tài khoản trên kho chung.
//   Boot: setKitLabels + applyViderseBrand ở main.tsx → SessionProvider hỏi
//   /api/me; cổng theo trạng thái phiên: chưa kiểm → chờ · store trống → Setup
//   · không trả lời được → lỗi + Thử lại · chưa đăng nhập → Login
//   · chưa cấp quyền Viderse → màn bị chặn · ép đổi mật khẩu → màn đổi mật khẩu
//   · còn lại → shell + route. Bảng route → màn nằm ở `screens/RouteScreen.tsx`;
//   điện thoại dựng nội dung trong pane giữ-alive (`shell/MobileRouteStage.tsx`).
import { Suspense, useEffect, type ReactNode } from 'react'
import { Spinner, ToastHost } from '@noverse/ui'
import { SessionProvider, useSession } from './api/session'
import { AppShell } from './shell/AppShell'
import { startDataEvents } from './data/events'
import { navigate, routePath, useRoute, type Route } from './router'
import { ToastAnchor } from './shell/toastAnchor'
import { useMediaQuery } from './lib/useMediaQuery'
import { AccessDeniedScreen, ChangePasswordScreen, LoginScreen, RouteScreen, ServerDownScreen, SetupScreen } from './screens/RouteScreen'
import { MobileRouteStage } from './shell/MobileRouteStage'

/**
 * Nội dung theo route: điện thoại dựng trong pane giữ-alive của thanh tab
 * (đổi tab không mất màn/vị trí cuộn, không tải lại dữ liệu), máy tính dựng
 * đúng một màn như trước.
 */
function Stage({ route }: { route: Route }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  return mobile ? <MobileRouteStage route={route} /> : <RouteScreen route={route} />
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

  const boot = (
    <div style={{ minHeight: '100dvh', display: 'grid', placeItems: 'center' }} aria-busy="true">
      <Spinner size={18} color="var(--color-fg-3)" />
    </div>
  )
  if (!checked) return boot
  // Chờ đúng chunk của màn cổng đang mở; khung chờ giống lúc hỏi /api/me.
  const gate = (screen: ReactNode) => <Suspense fallback={boot}>{screen}</Suspense>
  if (!serverReachable) return gate(<ServerDownScreen />)
  if (setupNeeded) return gate(<SetupScreen />)
  if (accessDenied) return gate(<AccessDeniedScreen />)
  if (mustChangePassword) return gate(<ChangePasswordScreen />)
  if (!user) return gate(<LoginScreen />)
  if (parsed.route.name === 'editor') return <RouteScreen route={parsed.route} />
  return (
    <AppShell route={parsed.route} user={user}>
      <Stage route={parsed.route} />
    </AppShell>
  )
}

export default function App() {
  const { route } = useRoute()
  // Request-scoped credentials are independent of the human account session.
  if (route.name === 'render' || route.name === 'measure') return <RouteScreen route={route} />
  return (
    <SessionProvider>
      <Root />
      <ToastHost activity />
      <ToastAnchor />
    </SessionProvider>
  )
}
