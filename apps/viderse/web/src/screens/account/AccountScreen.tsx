// Tài khoản — thông tin người dùng hiện tại + Đăng xuất (POST /api/logout).
import { useState } from 'react'
import { logout } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Avatar, Button, FieldError, Icon } from '@noverse/ui'

const fmtDate = (iso: string | undefined) => {
  if (!iso) return null
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? null : `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`
}

export function AccountScreen() {
  const { user, setUser } = useSession()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  if (!user) return null

  const name = user.name || user.username
  const since = fmtDate(user.createdAt)

  const doLogout = async () => {
    setBusy(true)
    setError('')
    try {
      await logout()
      setUser(null)
    } catch {
      setError(t.account.logoutFailed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ padding: 'var(--vv-s-24) var(--vv-s-20)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-20)', maxWidth: 480 }}>
      <h1 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>{t.account.title}</h1>
      <div className="card" style={{ borderRadius: 'var(--radius-tile)', padding: 'var(--vv-s-16)', display: 'flex', alignItems: 'center', gap: 'var(--vv-s-14)' }}>
        <Avatar name={name} size={44} />
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-3)', minWidth: 0 }}>
          <span style={{ fontSize: 15, fontWeight: 500 }}>{name}</span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-6)', fontSize: 13, color: 'var(--color-fg-3)' }}>
            <span className="mono">@{user.username}</span>
            <span aria-hidden="true" style={{ color: 'var(--color-fg-4)' }}>·</span>
            <Icon name={user.role === 'admin' ? 'crown' : 'profile-2user'} size={14} style={{ color: user.role === 'admin' ? 'var(--color-accent)' : 'var(--color-fg-4)' }} />
            {t.role[user.role]}
          </span>
          {since && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.account.memberSince}: <span className="num">{since}</span></span>}
        </div>
      </div>
      {error && <FieldError>{error}</FieldError>}
      <div>
        <Button variant="danger" pending={busy} onClick={() => void doLogout()} icon={<Icon name="logout" size={16} />}>
          {busy ? t.common.loggingOut : t.account.logout}
        </Button>
      </div>
    </div>
  )
}
