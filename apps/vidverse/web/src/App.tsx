// App — ghép Session + Router + AppShell.
// Cổng phiên: chưa kiểm xong /api/me → spinner; máy chủ chưa có tài khoản → Setup;
// chưa đăng nhập → Login; có phiên → shell với màn theo route.
import { useEffect } from 'react'
import { SessionProvider, useSession } from './api/session'
import { navigate, routePath, useRoute, type Parsed } from './router'
import { AppShell, useMediaQuery } from './shell/AppShell'
import { SetupScreen } from './screens/auth/SetupScreen'
import { LoginScreen } from './screens/auth/LoginScreen'
import { MobileShell } from './shell/MobileShell'
import { HomeScreen } from './screens/home/HomeScreen'
import { AccountScreen } from './screens/account/AccountScreen'
import { Spinner, ToastHost } from '@noverse/ui'

export function Screen({ parsed }: { parsed: Parsed }) {
  return parsed.route.name === 'account' ? <AccountScreen /> : <HomeScreen />
}

function Root() {
  const parsed = useRoute()
  const { checked, user, setupNeeded } = useSession()
  const mobile = useMediaQuery('(max-width: 759px)')

  // Hash không hợp lệ đã parse về home — chuẩn hoá luôn thanh địa chỉ (replace,
  // không đẩy entry lịch sử thừa). So sánh chuỗi thay vì chỉ kiểm `routePath`
  // để '?query' lạ trên route hợp lệ vẫn bị cắt.
  useEffect(() => {
    if (!checked || !user) return
    const target = `#${routePath(parsed.route)}`
    if (location.hash !== target) navigate(routePath(parsed.route), true)
  }, [checked, user, parsed.route])

  if (!checked) {
    return <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Spinner size={22} /></div>
  }
  if (setupNeeded) return <SetupScreen />
  if (!user) return <LoginScreen />
  if (mobile) {
    return (
      <MobileShell tabs={parsed.route}>
        <Screen parsed={parsed} />
      </MobileShell>
    )
  }
  return (
    <AppShell>
      <Screen parsed={parsed} />
    </AppShell>
  )
}

export default function App() {
  return (
    <SessionProvider>
      <Root />
      <ToastHost />
    </SessionProvider>
  )
}
