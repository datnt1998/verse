// Màn bị chặn khi tài khoản đăng nhập được (phiên thật trên kho chung) nhưng
// chưa được cấp quyền dùng Viderse — 403 app_access_denied. Giữ phiên + hiển
// thị người đang đăng nhập: KHÔNG phải màn đăng nhập, không vòng lặp.
//   · Thử lại: đọc lại /api/me — được cấp rồi thì vào app, phiên hết thì về login.
//   · Đăng xuất: revoke phiên trên máy chủ (thoát cả Viderse lẫn Noverse vì
//     cùng một phiên) — KHÔNG có nút "đổi tài khoản" giấu phiên đang sống.
// Render ở Root trước shell nên dữ liệu/SSE không mount khi đang bị chặn.
import { useState } from 'react'
import { Avatar, Button, FieldError, Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { AuthShell, HintLine } from './parts'

export function AccessDeniedScreen() {
  const { accessDenied, refresh, signOut, endSession } = useSession()
  const [busy, setBusy] = useState<'retry' | 'logout' | null>(null)
  const [error, setError] = useState('')
  if (!accessDenied) return null

  const retry = async () => {
    if (busy) return
    setBusy('retry')
    setError('')
    try {
      const outcome = await refresh()
      // 'user'/'password': cổng tự chuyển màn. 'denied': chưa được cấp → ở lại.
      // 'anon' = phiên đã hết → về đăng nhập; 'down' = máy chủ không trả lời →
      // cổng lỗi lo phần hiển thị.
      if (outcome === 'anon') endSession()
    } finally {
      setBusy(null)
    }
  }

  const leave = async () => {
    if (busy) return
    setBusy('logout')
    setError('')
    try {
      await signOut()
    } catch {
      // Revoke thất bại: giữ nguyên màn chặn, báo lỗi — không báo giả đã thoát.
      setError(t.account.logoutFailed)
    } finally {
      setBusy(null)
    }
  }

  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-14)', textAlign: 'center' }}>
        <span aria-hidden="true" style={{
          width: 56, height: 56, borderRadius: '50%', display: 'grid', placeItems: 'center',
          background: 'var(--tint-err)', color: 'var(--color-err)',
        }}>
          <Icon name="shield-security" size={28} />
        </span>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.auth.accessDeniedTitle}</h1>
        <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 380 }}>{t.auth.accessDenied}</p>
      </div>

      <div className="card" style={{
        borderRadius: 'var(--radius-tile)', padding: 'var(--vv-s-10) var(--vv-s-12)',
        display: 'flex', alignItems: 'center', gap: 'var(--vv-s-12)',
      }}>
        <Avatar name={accessDenied.name || accessDenied.username} size={38} />
        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: 14, fontWeight: 500 }}>{accessDenied.name || accessDenied.username}</span>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
            <span className="mono">@{accessDenied.username}</span> · {t.role[accessDenied.role]}
          </span>
        </span>
      </div>

      {error && <FieldError>{error}</FieldError>}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)' }}>
        <Button variant="primary" onClick={() => void retry()} disabled={busy !== null} pending={busy === 'retry'}
          style={{ minHeight: 'var(--vv-s-44)', justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
          <Icon name="refresh-2" size={17} />{t.common.retry}
        </Button>
        <Button variant="dsoft" onClick={() => void leave()} disabled={busy !== null} pending={busy === 'logout'}
          style={{ minHeight: 'var(--vv-s-44)', justifyContent: 'center' }}>
          <Icon name="logout" size={16} />{busy === 'logout' ? t.common.loggingOut : t.account.logout}
        </Button>
        <HintLine icon="profile-2user">{t.account.sharedLogoutNote}</HintLine>
      </div>
    </AuthShell>
  )
}
