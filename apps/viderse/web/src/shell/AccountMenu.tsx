import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Avatar, FieldError, Icon, useEscapeLayer } from '@noverse/ui'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { useSession } from '../api/session'
import { membersUrl } from '../api/sso'
import { AppLinks, ExternalIcon } from '../components/AppLinks'
import { t } from '../i18n/vi'
import { navigate, paths } from '../router'
import { useMediaQuery } from '../lib/useMediaQuery'
const sx = stylex.create({
  anchor: { position: 'relative' },
  menu: { position: 'absolute', zIndex: 40, top: 40, right: 0, width: 336, padding: 6, borderRadius: 16, backgroundColor: color['--color-surface-4'], border: '1px solid', borderColor: line['--line-3'], boxShadow: 'var(--vv-shadow-pop)', display: 'flex', flexDirection: 'column' },
  user: { display: 'flex', alignItems: 'center', gap: 12, padding: '10px 10px 12px' },
  text: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 },
  name: { fontSize: 14.5, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  sub: { fontSize: 11.5, color: color['--color-fg-3'] },
  heading: { padding: '6px 10px 4px', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: color['--color-fg-4'] },
  divider: { height: 1, margin: 6, backgroundColor: line['--line-2'] },
  row: { minHeight: 44, padding: '6px 10px', display: 'flex', alignItems: 'center', gap: 12, borderRadius: 10, borderWidth: 0, backgroundColor: { default: 'transparent', ':hover': kit['--kit-wash-5'] }, color: color['--color-fg'], fontSize: 13.5, textAlign: 'left', cursor: 'pointer' },
  logout: { color: color['--color-err'] },
})
export function AccountMenu({ fallbackName }: { fallbackName?: string }) {
  const { user, apps, signOut } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [open, setOpen] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const close = () => { setOpen(false); trigger.current?.focus() }
  useEscapeLayer(open, close)
  useEffect(() => {
    if (!open) return
    ref.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    const outside = (event: PointerEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  const logout = async () => {
    if (busy) return
    setBusy(true); setError('')
    try { await signOut(); close() } catch { setError(t.account.logoutFailed) } finally { setBusy(false) }
  }
  const memberHref = membersUrl(apps)
  const name = user?.name || user?.username || fallbackName || t.account.title
  return <div ref={ref} {...stylex.props(sx.anchor)}>
    <button ref={trigger} type="button" className="vv-avatar" title={t.account.menuLabel} aria-label={t.account.menuLabel}
      aria-haspopup={mobile ? undefined : 'menu'} aria-expanded={mobile ? undefined : open}
      onClick={() => mobile ? navigate(paths.account()) : setOpen(!open)}>{name.trim()[0]?.toLocaleUpperCase('vi')}</button>
    {open && user && <div role="menu" aria-label={t.account.title} {...stylex.props(sx.menu)} onKeyDown={(event) => {
      if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const items = [...event.currentTarget.querySelectorAll<HTMLElement>('[role="menuitem"]')]
      const current = items.indexOf(document.activeElement as HTMLElement)
      const index = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
      items[index]?.focus()
    }}>
      <div {...stylex.props(sx.user)}><Avatar name={name} size={40} /><span {...stylex.props(sx.text)}><b {...stylex.props(sx.name)} title={name}>{name}</b><span {...stylex.props(sx.sub)}>{t.role[user.role]} · @{user.username}</span></span></div>
      {apps.some((app) => app.id !== 'viderse') && <><div {...stylex.props(sx.divider)} /><span {...stylex.props(sx.heading)}>{t.account.apps}</span><AppLinks apps={apps} menu /></>}
      <div {...stylex.props(sx.divider)} />
      <button type="button" role="menuitem" {...stylex.props(sx.row)} onClick={() => { close(); navigate(paths.account()) }}><Icon name="user" size={17} /><span style={{ flex: 1 }}>{t.account.title}</span><span {...stylex.props(sx.sub)}>{t.account.menuAccountHint}</span></button>
      {user.role === 'admin' && memberHref && <a role="menuitem" href={memberHref} target="_blank" rel="noopener noreferrer" {...stylex.props(sx.row)}><Icon name="crown" size={17} /><span {...stylex.props(sx.text)}>{t.account.manageMembers}<span {...stylex.props(sx.sub)}>{t.account.membersHint}</span></span><ExternalIcon /></a>}
      <button type="button" role="menuitem" disabled={busy} onClick={() => void logout()} {...stylex.props(sx.row, sx.logout)}><Icon name="logout" size={17} /><span {...stylex.props(sx.text)}>{t.account.logout}<span {...stylex.props(sx.sub)}>{t.account.menuLogoutHint}</span></span></button>
      {error && <FieldError>{error}</FieldError>}
    </div>}
  </div>
}
