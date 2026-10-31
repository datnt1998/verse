// AccountMenu — menu avatar ở header (Login.dc.html › menu).
// Tên + vai trò + phiên còn lại (thanh tiến độ, cam khi ≤3 ngày) · QR · Thiết bị (số) ·
// Đổi mật khẩu · Quản lý tài khoản (admin, số) · Chuyển tài khoản · Đăng xuất.
// Bàn phím: ↑↓/Home/End di chuyển, Enter kích hoạt, Esc đóng và trả focus về avatar.
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { listUsers, mySessions } from '../api/client'
import { useSession } from '../api/session'
import { getRemembered, setNextRoute } from '../api/storage'
import { t } from '../i18n/vi'
import { navigate, paths, setDialog } from '../router'
import { rememberHelpTarget } from '../commands/target'
import '@noverse/ui/kit.css'
import type { Role } from '../api/types'
import { Avatar, Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { keys, useAction, useResource } from '../data'
import { WhatsNewDialog } from './WhatsNew'
import { hasNewVersion, useVersionInfo, versionText } from './version'
import { AppSwitcher } from './AppSwitcher'
import { ssoCopy } from '../i18n/sso'

const ROLE_STYLE: Record<Role, { bg: string; fg: string; icon: string }> = {
  admin: { bg: 'var(--tint-accent)', fg: 'var(--color-accent)', icon: 'crown' },
  member: { bg: 'var(--line-1)', fg: 'var(--color-fg-3)', icon: 'profile-2user' },
}

export function RoleBadge({ role, compact = false }: { role: Role; compact?: boolean }) {
  const c = ROLE_STYLE[role]
  // Ngoài menu tài khoản chung, giữ chữ “tài khoản” để không lẫn với trợ lý Người viết (Gói J).
  return (
    <span style={{
      height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', gap: 4,
      borderRadius: 6, background: compact && role === 'admin' ? 'var(--members-admin-bg)' : c.bg, color: compact && role === 'admin' ? 'var(--members-admin-fg)' : c.fg, fontSize: 11, fontWeight: 500, whiteSpace: 'nowrap',
    }}>
      <Icon name={c.icon} size={compact ? 11 : 12} />{compact ? t.role[role] : `tài khoản ${t.role[role]}`}
    </span>
  )
}

const itemStyle: CSSProperties = {
  display: 'flex', alignItems: 'center', gap: 12, padding: '9px 12px', border: 'none',
  borderRadius: 10, background: 'transparent', color: 'var(--color-fg)', fontSize: 13,
  textAlign: 'left', width: '100%',
}

const countStyle: CSSProperties = {
  height: 20, minWidth: 20, padding: '0 6px', display: 'flex', alignItems: 'center', justifyContent: 'center',
  borderRadius: 6, background: 'rgba(255,255,255,0.06)', color: 'var(--color-fg-3)', fontSize: 11,
}

export function AccountMenu() {
  const { user, session, signOut } = useSession()
  const [open, setOpen] = useState(false)
  const [whatsNew, setWhatsNew] = useState(false)
  const version = useVersionInfo()
  const newer = hasNewVersion(version)
  const sessions = useResource(keys.sessions(), mySessions, { enabled: open && !!user })
  const accounts = useResource(keys.users(), listUsers, { enabled: open && user?.role === 'admin' })
  const counts = sessions.data ? { devices: sessions.data.sessions.length, users: accounts.data?.users.length ?? 0 } : null
  const logoutAction = useAction(async () => {
    await signOut()
    setOpen(false)
    return true
  })
  const wrap = useRef<HTMLDivElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const onDoc = (event: MouseEvent) => { if (!wrap.current?.contains(event.target as Node)) setOpen(false) }
    const onKey = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDoc); document.removeEventListener('keydown', onKey) }
  }, [])


  useEffect(() => {
    if (!open) return
    const items = menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]')
    items?.[0]?.focus()
  }, [open])

  const onMenuKey = (event: KeyboardEvent<HTMLDivElement>) => {
    const items = Array.from(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    if (items.length === 0) return
    const index = items.indexOf(document.activeElement as HTMLElement)
    if (event.key === 'ArrowDown') {
      event.preventDefault(); items[(index + 1 + items.length) % items.length]?.focus()
    } else if (event.key === 'ArrowUp') {
      event.preventDefault(); items[(index - 1 + items.length) % items.length]?.focus()
    } else if (event.key === 'Home') {
      event.preventDefault(); items[0]?.focus()
    } else if (event.key === 'End') {
      event.preventDefault(); items[items.length - 1]?.focus()
    } else if (event.key === 'Escape') {
      event.preventDefault(); setOpen(false); trigger.current?.focus()
    }
  }

  if (!user) return null
  const name = user.name || user.username
  const others = getRemembered().filter((account) => account.username !== user.username)

  const expiresAt = session ? new Date(session.expiresAt) : null
  const daysLeft = expiresAt ? Math.max(0, Math.ceil((expiresAt.getTime() - Date.now()) / 86400000)) : 0
  const pct = Math.min(100, (daysLeft / 30) * 100)
  const low = daysLeft <= 3
  const fmt = (date: Date) => `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}/${date.getFullYear()}`

  const switchAccount = () => {
    setNextRoute('#/')
    void logoutAction.run().then((done) => { if (done) location.hash = '#/login' })
  }
  const openDialog = (dialog: 'qr' | 'devices') => {
    setOpen(false)
    setDialog(dialog)
  }

  return (
    <div ref={wrap} style={{ position: 'relative' }}>
      <button ref={trigger} type="button" aria-haspopup="menu" aria-expanded={open}
        aria-label={newer ? `${t.account.avatarLabel(name)} · có bản Noverse mới` : t.account.avatarLabel(name)}
        onClick={() => setOpen((value) => !value)}
        onKeyDown={(event) => { if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); setOpen(true) } }}
        style={{
          position: 'relative', flex: '0 0 auto', width: 32, height: 32, padding: 0, borderRadius: '50%', border: 'none', background: 'transparent',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: open ? '0 0 0 2px var(--color-bg), 0 0 0 4px var(--color-accent)' : 'none',
        }}>
        <Avatar name={name} size={30} />
        {newer && (
          <span aria-hidden="true" style={{
            position: 'absolute', top: -1, right: -1, width: 9, height: 9, borderRadius: '50%',
            background: 'var(--kit-info-fg)', boxShadow: '0 0 0 2px var(--color-bg)',
          }} />
        )}
      </button>
      {open && (
        <div ref={menu} className="kit-pop" role="menu" aria-label={t.account.menuTitle} onKeyDown={onMenuKey}
          onMouseOver={(event) => (event.target as HTMLElement).closest<HTMLElement>('[role="menuitem"]')?.focus()} style={{
          position: 'absolute', top: user.grants != null ? 'calc(100% + 9px)' : 40, right: user.grants != null ? -4 : 0, zIndex: 40, width: user.grants != null ? 330 : 336, padding: 6,
          display: 'flex', flexDirection: 'column',
        }}>
          <div style={{ padding: '12px 12px 14px', display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Avatar name={name} size={42} />
              <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 15, fontWeight: 600 }}>
                  {name}<RoleBadge role={user.role} compact={user.grants != null} />
                </span>
                <span className="mono" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>@{user.username}</span>
              </div>
            </div>
            {session && expiresAt && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--color-fg-3)' }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Icon name="shield-security" size={14} />{t.account.sessionOn(session.device)}
                  </span>
                  <span className="num">{t.account.daysLeft(daysLeft)}</span>
                </div>
                <div aria-hidden="true" style={{ height: 3, borderRadius: 3, background: 'var(--color-surface-5)' }}>
                  <div style={{ width: `${pct}%`, height: '100%', borderRadius: 3, background: low ? 'var(--color-warn)' : 'var(--color-ok)' }} />
                </div>
                <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                  {t.account.expiresAt(fmt(expiresAt))}
                </span>
              </div>
            )}
          </div>
          <div style={{ height: 1, background: 'var(--line-2)', margin: '0 6px 6px' }} />
          <button type="button" role="menuitem" className="hov-row" style={itemStyle} onClick={() => openDialog('qr')}>
            <Icon name="mobile" size={17} style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1 }}>{t.account.phoneQr}</span>
          </button>
          <button type="button" role="menuitem" className="hov-row" style={itemStyle} onClick={() => openDialog('devices')}>
            <Icon name="devices" size={17} style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1 }}>{t.account.devices}</span>
            {counts && <span className="num" style={countStyle}>{counts.devices}</span>}
          </button>
          <button type="button" role="menuitem" className="hov-row" style={itemStyle} onClick={() => openDialog('devices')}>
            <Icon name="password-check" size={17} style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1 }}>{t.account.changePassword}</span>
          </button>
          {user.role === 'admin' && (
            <button type="button" role="menuitem" className="hov-row" style={itemStyle} onClick={() => { setOpen(false); navigate(paths.settings('members')) }}>
              <Icon name="profile-2user" size={17} style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1 }}>{t.account.manageUsers}</span>
              {counts && counts.users > 0 && <span className="num" style={countStyle}>{counts.users}</span>}
            </button>
          )}
          {user.grants == null && <>
          <div role="separator" className="kit-separator" />
          <button type="button" role="menuitem" className="kit-option" onClick={() => { setOpen(false); navigate(paths.settings()) }}>
            <Icon name="setting-2" size={16} /><span style={{ flex: 1 }}>{t.nav.settings}</span><Kbd>⌘ ,</Kbd>
          </button>
          <button type="button" role="menuitem" className="kit-option" onClick={() => { setOpen(false); rememberHelpTarget(); navigate(paths.help()) }}>
            <Icon name="message-question" size={16} /><span style={{ flex: 1 }}>Hướng dẫn lệnh</span><Kbd>?</Kbd>
          </button>
          </>}
          {newer && (
            <button type="button" role="menuitem" className="kit-option" onClick={() => { setOpen(false); setWhatsNew(true) }}>
              <Icon name="refresh-2" size={16} /><span style={{ flex: 1 }}>Có bản mới · {versionText(version?.latestVersion)}</span>
              <span style={countStyle}>Xem</span>
            </button>
          )}
          <AppSwitcher />
          {others.length > 0 && (
            <>
              <div style={{ height: 1, background: 'var(--line-2)', margin: 6 }} />
              <div style={{ padding: '6px 12px 4px' }} className="lbl">{t.account.switchAccount}</div>
              {others.map((account) => (
                <button key={account.username} type="button" role="menuitem" className="hov-row" style={{ ...itemStyle, padding: '8px 12px' }}
                  onClick={switchAccount}>
                  <Avatar name={account.name || account.username} size={26} />
                  <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <span style={{ fontSize: 13 }}>{account.name || account.username}</span>
                    <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{user.grants != null ? ssoCopy.switchAccount(account.name || account.username) : <>{t.role[account.role]} · {t.account.needPassword}</>}</span>
                  </span>
                  <Icon name="arrow-right-2" size={15} style={{ color: 'var(--color-fg-4)' }} />
                </button>
              ))}
            </>
          )}
          <button type="button" role="menuitem" className="hov-row" style={{ ...itemStyle, padding: '8px 12px', color: 'var(--color-fg-2)' }} onClick={switchAccount}>
            <Icon name="user-add" size={17} style={{ color: 'var(--color-fg-3)', margin: '0 4px' }} />{t.auth.otherAccount}
          </button>
          <div style={{ height: 1, background: 'var(--line-2)', margin: 6 }} />
          <button type="button" role="menuitem" className="hov-row" style={{ ...itemStyle, ...(user.grants != null ? { padding: '8px 12px' } : {}), color: 'var(--color-err)' }}
            onClick={() => { void logoutAction.run() }}>
            <Icon name="logout" size={17} /><span style={{ flex: 1, display: 'flex', flexDirection: 'column', ...(user.grants != null ? { lineHeight: 1.35 } : { gap: 3 }) }}>{t.account.logout}{user.grants != null && <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{ssoCopy.logout}</span>}</span>
          </button>
        </div>
      )}
      {whatsNew && <WhatsNewDialog onClose={() => setWhatsNew(false)} />}
    </div>
  )
}

export { AccountMenu as UserMenu }
