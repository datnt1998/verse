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
import { ssoCopy as c } from '../../i18n/sso'
import { ExternalAppIcon } from '../../shell/AppSwitcher'

export function AccessDeniedScreen() {
  const { user, deniedUser, apps, refresh, signOut, endSession } = useSession()
  const [busy, setBusy] = useState<'retry' | 'logout' | null>(null)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState('')
  const shown = deniedUser ?? user
  if (!shown) return null
  const shared = shown.grants != null
  const other = apps?.find((app) => app.id === 'viderse')

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
    try { await signOut() } catch (failure) { setError(failure instanceof Error ? failure.message : c.copyError) } finally { setBusy(null) }
  }
  const copy = async () => {
    try { await navigator.clipboard.writeText(c.request(shown.username)); setCopied(true); setError('') }
    catch { setError(c.copyError) }
  }
  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, textAlign: 'center' }}>
        {shared ? <span aria-hidden="true" style={{ width: 56, height: 56, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--kit-err-12)', color: 'var(--color-err)' }}><Icon name="lock" size={26} /></span> : <OrbHero variant="error" />}
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{shared ? c.deniedTitle : t.auth.accessDeniedTitle}</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 380 }}>{shared ? c.deniedHint : t.auth.accessDenied}</p>
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
        {shared && other && <a className="kit-btn kit-btn-primary" href={other.origin} target="_blank" rel="noopener noreferrer" style={{ height: 42, borderRadius: 11, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, background: 'var(--color-fg)', color: 'var(--color-bg)', textDecoration: 'none', fontWeight: 600 }}>{c.openViderse}<ExternalAppIcon /></a>}
        <Button variant={shared && other ? 'ghost' : 'primary'} onClick={() => void retry()} disabled={busy !== null} pending={busy === 'retry'} style={{ height: 42, borderRadius: 11, justifyContent: 'center' }}>
          <Icon name="refresh-2" size={17} />{t.auth.retry}
        </Button>
        {shared && <Button variant="quiet" onClick={() => void copy()}>{copied ? c.copied : c.copyRequest}</Button>}
        <Button variant={shared ? 'quiet' : 'dsoft'} onClick={() => void leave()} disabled={busy !== null} pending={busy === 'logout'} style={{ height: 40, borderRadius: 11, justifyContent: 'center', color: shared ? 'var(--kit-err-ghost-ink)' : undefined }}>
          <Icon name="logout" size={16} />{t.account.logout}
        </Button>
      </div>
      {shared && <p style={{ margin: 0, fontSize: 12, textAlign: 'center', color: 'var(--color-fg-4)' }}>{c.logout}</p>}
      {error && <p role="alert">{error}</p>}
    </AuthShell>
  )
}
