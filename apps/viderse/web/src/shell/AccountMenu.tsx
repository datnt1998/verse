import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Avatar, FieldError, Icon, useEscapeLayer } from '@noverse/ui'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { useSession } from '../api/session'
import { membersUrl } from '../api/sso'
import { accountStyles, AppLinks, ExternalIcon } from '../components/AppLinks'
import { t } from '../i18n/vi'
import { navigate, paths } from '../router'
import { useMediaQuery } from '../lib/useMediaQuery'
const sx = stylex.create({
  anchor: { position: 'relative' },
  menu: { position: 'absolute', zIndex: 40, top: 38, right: 0, width: 336, padding: 'var(--vv-s-6)', borderRadius: 'var(--vv-r-16)', backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], boxShadow: 'var(--vv-shadow-pop)', display: 'flex', flexDirection: 'column' },
  user: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-12)', padding: 'var(--vv-s-10) var(--vv-s-10) var(--vv-s-12)' },
  text: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 },
  name: { fontSize: 14.5, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  sub: { fontSize: 11.5, color: color['--color-fg-3'] },
  heading: { padding: 'var(--vv-s-6) var(--vv-s-10) var(--vv-s-4)', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: kit['--kit-fg-sub'] },
  divider: { height: 1, margin: 'var(--vv-s-6)', backgroundColor: line['--line-2'] },
  row: { minHeight: 40, padding: 'var(--vv-s-6) var(--vv-s-10)', display: 'flex', alignItems: 'center', gap: 'var(--vv-s-12)', borderRadius: 'var(--vv-r-10)', borderWidth: 0, backgroundColor: { default: 'transparent', ':hover': kit['--kit-wash-5'] }, color: color['--color-fg'], fontSize: 13.5, textAlign: 'left', cursor: 'pointer' },
  tall: { minHeight: 48 },
  actionText: { lineHeight: 1.3 },
  logout: { color: 'var(--vv-warn-ink)' },
  trigger: { color: color['--color-fg'], backgroundColor: color['--color-surface-6'], padding: 'var(--vv-s-1) var(--vv-s-6)', borderColor: line['--line-3'] },
  /* Vòng chữ cái đầu của tài khoản ở cuối AppHeader (trước ở .vv-avatar). */
  avatar: {
    width: 'var(--vv-hit)',
    height: 'var(--vv-hit)',
    borderRadius: '50%',
    flex: '0 0 auto',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 12,
    fontWeight: 600,
    borderWidth: 'var(--vv-s-1)',
    borderStyle: 'solid',
    borderColor: line['--line-1'],
    textDecoration: 'none',
  },
  triggerOpen: { borderColor: 'var(--vv-check-ring)', boxShadow: '0 0 0 3px var(--line-3)' },
  muted: { color: color['--color-fg-3'] },
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
    <button ref={trigger} type="button" className={`vv-avatar ${stylex.props(sx.trigger, open && sx.triggerOpen, sx.avatar).className}`} title={t.account.menuLabel} aria-label={t.account.menuLabel}
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
      <div {...stylex.props(sx.user)}><Avatar name={name} size={40} style={{ fontSize: 15, border: '1px solid var(--kit-wash-10)' }} /><span {...stylex.props(sx.text)}><span style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)' }}><b {...stylex.props(sx.name)} title={name}>{name}</b><span {...stylex.props(accountStyles.tag)}>{t.role[user.role]}</span></span><span className="mono" style={{ fontSize: 12, color: 'var(--vv-fg-note)' }}>@{user.username}</span></span></div>
      {apps.some((app) => app.id !== 'viderse') && <><div {...stylex.props(sx.divider)} /><span {...stylex.props(sx.heading)}>{t.account.apps}</span><AppLinks apps={apps} menu /></>}
      <div {...stylex.props(sx.divider)} />
      <button type="button" role="menuitem" {...stylex.props(sx.row)} onClick={() => { close(); navigate(paths.account()) }}><Icon name="user" size={17} {...stylex.props(sx.muted)} /><span style={{ flex: 1 }}>{t.account.title}</span><span {...stylex.props(sx.sub)}>{t.account.menuAccountHint}</span></button>
      {user.role === 'admin' && memberHref && <a role="menuitem" href={memberHref} target="_blank" rel="noopener noreferrer" {...stylex.props(sx.row, sx.tall)}><Icon name="crown" size={17} {...stylex.props(sx.muted)} /><span {...stylex.props(sx.text, sx.actionText)}>{t.account.manageMembers}<span {...stylex.props(sx.sub)}>{t.account.membersHint}</span></span><ExternalIcon /></a>}
      <button type="button" role="menuitem" disabled={busy} onClick={() => void logout()} {...stylex.props(sx.row, sx.tall, sx.logout)}><Icon name="logout" size={17} /><span {...stylex.props(sx.text, sx.actionText)}>{t.account.logout}<span {...stylex.props(sx.sub)}>{t.account.menuLogoutHint}</span></span></button>
      {error && <FieldError>{error}</FieldError>}
    </div>}
  </div>
}
