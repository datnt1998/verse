// Màn bị chặn khi tài khoản đăng nhập được nhưng chưa được cấp quyền vào app
// (403 app_access_denied). Giữ phiên + hiển thị người đang đăng nhập — KHÔNG
// phải màn đăng nhập, không vòng lặp: Thử lại đọc lại /api/me, Đăng xuất revoke
// phiên rồi mới về login (đổi tài khoản cũng đi đường này, không để phiên mồ côi).
// Render ở App trước mọi screen nên không mount dữ liệu.
import { useState } from 'react'
import { Avatar, Button, Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { navigate } from '../../router'
import { AuthShell, OrbHero, authDestination } from './parts'

export function AccessDeniedScreen() {
  const { user, deniedUser, refresh, signOut, endSession } = useSession()
  const [busy, setBusy] = useState<'retry' | 'logout' | null>(null)
  const shown = deniedUser ?? user
  if (!shown) return null

  const retry = async () => {
    if (busy) return
    setBusy('retry')
    try {
      const outcome = await refresh()
      // Được cấp quyền rồi → vào đúng route đã lưu trước khi bị chặn.
      if (outcome === 'user') navigate(authDestination(shown), true)
      // 'denied' = vẫn chưa được cấp → ở lại màn này; 'anon' = phiên hết hạn
      // → cổng 401 đưa về đăng nhập (đăng nhập lại vào đúng tài khoản).
      else if (outcome === 'anon') endSession()
    } finally { setBusy(null) }
  }
  const leave = async () => {
    if (busy) return
    setBusy('logout')
    try { await signOut() } finally { setBusy(null) }
  }


  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, textAlign: 'center' }}>
        <OrbHero variant="error" />
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.auth.accessDeniedTitle}</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 380 }}>{t.auth.accessDenied}</p>
        </div>
      </div>

      <div className="card" style={{ borderRadius: 14, padding: '10px 12px', display: 'flex', alignItems: 'center', gap: 12 }}>
        <Avatar name={shown.name || shown.username} size={38} />
        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: 14, fontWeight: 500 }}>{shown.name || shown.username}</span>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}><span className="mono">@{shown.username}</span> · {t.role[shown.role]}</span>
        </span>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <Button variant="primary" onClick={() => void retry()} disabled={busy !== null} pending={busy === 'retry'} style={{ height: 42, borderRadius: 11, justifyContent: 'center' }}>
          <Icon name="refresh-2" size={17} />{t.auth.retry}
        </Button>
        <Button variant="dsoft" onClick={() => void leave()} disabled={busy !== null} pending={busy === 'logout'} style={{ height: 40, borderRadius: 11, justifyContent: 'center' }}>
          <Icon name="logout" size={16} />{t.account.logout}
        </Button>
      </div>
    </AuthShell>
  )
}
