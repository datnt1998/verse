import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Avatar, Button, FieldError, Icon, Sheet } from '@noverse/ui'
import { color } from '@noverse/ui/tokens.stylex'
import { mySessions } from '../../api/client'
import { accountStyles as layout } from './account.stylex'
import { useSession } from '../../api/session'
import { membersUrl } from '../../api/sso'
import { accountStyles, AppLinks, ExternalIcon } from '../../components/AppLinks'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { t } from '../../i18n/auth'
import { ChangePasswordForm } from '../auth/ChangePasswordScreen'
import { Devices } from './Devices'
import { navigate, paths } from '../../router'
import { box } from '../../styles/shared.stylex'
const MOBILE = '@media (max-width: 767px)'
const sx = stylex.create({
  card: { borderRadius: { default: 14, [MOBILE]: 16 }, padding: { default: 16, [MOBILE]: 14 }, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-12)' },
  user: { display: 'flex', flexDirection: 'row', alignItems: 'center', gap: { default: 14, [MOBILE]: 12 } },
  title: { margin: 0, fontSize: { default: 22, [MOBILE]: 16 }, fontWeight: 600, letterSpacing: '-.02em' },
  body: { margin: 'var(--vv-s-5) 0 0', fontSize: 13.5, color: color['--color-fg-3'], lineHeight: 1.6 },
  heading: { margin: 0, fontSize: 14, fontWeight: 600 },
})
export function AccountScreen() {
  const { user, apps, signOut } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [sheet, setSheet] = useState<'password' | 'devices' | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [sessionCount, setSessionCount] = useState<number | null>(null)
  useEffect(() => {
    if (!mobile || !user || sheet !== null) return
    let cancelled = false
    void mySessions().then(({ sessions }) => {
      if (!cancelled) setSessionCount(sessions.length)
    }).catch(() => {
      if (!cancelled) setError(t.account.devicesFailed)
    })
    return () => { cancelled = true }
  }, [mobile, user, sheet])
  if (!user) return null
  const name = user.name || user.username
  const memberHref = membersUrl(apps)
  const logout = async () => {
    if (busy) return
    setBusy(true); setError('')
    try { await signOut() } catch { setError(t.account.logoutFailed) } finally { setBusy(false) }
  }
  const card = stylex.props(sx.card, box.card)
  return <div {...stylex.props(layout.page)}>
    {mobile && <header {...stylex.props(layout.header)}><Button variant="quiet" aria-label={t.account.back} onClick={() => navigate(paths.workbench())} {...stylex.props(layout.back)} style={{ padding: 0, gap: 0, fontWeight: 400, color: 'var(--color-fg-2)' }}><Icon name="arrow-r" size={18} style={{ transform: 'rotate(180deg)' }} /></Button><h1 {...stylex.props(layout.mobileTitle)}>{t.account.title}</h1></header>}
    <main {...stylex.props(layout.main)}><div {...stylex.props(layout.column)}>
    {!mobile && <div><h1 {...stylex.props(sx.title)}>{t.account.title}</h1><p {...stylex.props(sx.body)}>{t.account.subtitle}</p></div>}
    <section {...stylex.props(sx.card, sx.user, box.card)} aria-label={t.account.title}>
      <Avatar name={name} size={mobile ? 48 : 52} style={{ fontSize: mobile ? 18 : 19, border: '1px solid var(--kit-wash-10)' }} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)' }}><b title={name} style={{ fontSize: 16, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name}</b><span {...stylex.props(accountStyles.tag)}>{t.role[user.role]}</span></span>
        <span style={{ fontSize: mobile ? 12.5 : 13, color: mobile ? 'var(--vv-fg-note)' : 'var(--color-fg-3)' }}><span>@{user.username}</span>{user.createdAt && <> · {t.account.memberSince.toLocaleLowerCase('vi')} {new Date(user.createdAt).toLocaleDateString('vi-VN')}</>}</span>
      </span>
    </section>
    {mobile && <span {...stylex.props(layout.mobileNote)}>{t.account.subtitleMobile}</span>}
    {(mobile ? apps.some((app) => app.id !== 'viderse') : apps.length > 0) && (mobile ? <>
      <span {...stylex.props(layout.mobileLabel)}>{t.account.apps}</span>
      <div {...stylex.props(layout.mobileCard)}><AppLinks apps={apps} /></div>
    </> : <section {...card} aria-label={t.account.yourApps}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }}><h2 {...stylex.props(sx.heading)}>{t.account.yourApps}</h2>
      <span {...stylex.props(accountStyles.fieldHelp)}>{user.role === 'admin' ? t.account.adminApps : apps.some((app) => app.id !== 'viderse') ? t.account.memberApps : t.account.noOtherApp}</span></div>
      <div style={{ margin: '0 calc(-1 * var(--vv-s-10))' }}><AppLinks apps={apps} /></div>
    </section>)}
    {user.role === 'admin' && memberHref && (mobile ?
      <a href={memberHref} target="_blank" rel="noopener noreferrer" {...stylex.props(layout.row, layout.mobileCard)}><Icon name="crown" size={19} style={{ color: 'var(--color-warn)' }} /><span {...stylex.props(layout.rowText)}>{t.account.manageMembers}<span {...stylex.props(layout.secondary)}>{t.account.membersHintMobile}</span></span><ExternalIcon /></a>
      : <section {...card} aria-label={t.account.members} style={{ flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--vv-s-14)' }}>
        <Icon name="crown" size={20} style={{ color: 'var(--color-warn)' }} />
        <span style={{ flex: '1 1 340px', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }}><h2 {...stylex.props(sx.heading)}>{t.account.members}</h2><span {...stylex.props(accountStyles.help)}>{t.account.membersBody}</span></span>
        <a href={memberHref} target="_blank" rel="noopener noreferrer" {...stylex.props(accountStyles.button)}>{t.account.manageMembers}<ExternalIcon /></a>
      </section>)}
    {mobile ? <>
      <span {...stylex.props(layout.mobileLabel)}>{t.account.security}</span>
      <section {...stylex.props(layout.mobileCard)}>
        <button type="button" onClick={() => setSheet('password')} {...stylex.props(layout.row, layout.rowDivider)}><Icon name="key" size={19} {...stylex.props(layout.mutedIcon)} /><span {...stylex.props(layout.rowText)}>{t.account.passwordTitle}</span><Icon name="arrow-r" size={15} style={{ color: 'var(--kit-fg-sub)' }} /></button>
        <button type="button" onClick={() => setSheet('devices')} {...stylex.props(layout.row)}><Icon name="monitor" size={19} {...stylex.props(layout.mutedIcon)} /><span {...stylex.props(layout.rowText)}>{t.account.devices}</span>{sessionCount !== null && <span {...stylex.props(layout.count)}>{sessionCount}</span>}<Icon name="arrow-r" size={15} style={{ color: 'var(--kit-fg-sub)' }} /></button>
      </section>
      <Sheet open={sheet !== null} onClose={() => setSheet(null)} label={sheet === 'password' ? t.account.passwordTitle : t.account.devices} title={sheet === 'devices' ? t.account.devices : undefined} size="md" height="auto" homeIndicator style={{ paddingBottom: 0, background: 'var(--color-surface-3)', fontSize: 14, lineHeight: 1.5 }}>
        {sheet === 'password' && <div {...stylex.props(layout.sheetHeader)}><span {...stylex.props(layout.sheetTitle)}>{t.account.passwordTitle}</span><span {...stylex.props(layout.sheetSubtitle)}>{t.auth.passwordForm.reauthNoteMobile}</span></div>}
        {sheet === 'devices' && <div {...stylex.props(layout.sheetHeader)}><span {...stylex.props(layout.sheetSubtitle)}>{t.account.deviceNote}</span></div>}
        <div {...stylex.props(layout.sheetBody)}>
          {sheet === 'password' ? <ChangePasswordForm /> : sheet === 'devices' ? <Devices /> : null}
        </div>
      </Sheet>
    </> : <>
      <section {...card} aria-label={t.account.passwordTitle}><div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }}><h2 {...stylex.props(sx.heading)}>{t.account.passwordTitle}</h2><span {...stylex.props(accountStyles.fieldHelp)}>{t.auth.passwordForm.reauthNote}</span></div><ChangePasswordForm horizontal /></section>
      <section {...card} aria-label={t.account.devices}><Devices /></section>
    </>}
    {error && <FieldError>{error}</FieldError>}
    <section className={mobile ? undefined : `card ${card.className}`} aria-label={t.account.logout} style={mobile ? { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' } : { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--vv-s-14)' }}>
      {!mobile && <span style={{ flex: '1 1 300px', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)' }}><h2 {...stylex.props(sx.heading)}>{t.account.logout}</h2><span {...stylex.props(accountStyles.help)}>{t.account.sharedLogoutNote}</span></span>}
      <Button variant="dsoft" pending={busy} disabled={busy} onClick={() => void logout()} icon={<Icon name="logout" size={mobile ? 17 : 15} />} {...(mobile ? stylex.props(layout.logout) : {})} style={mobile ? undefined : { minHeight: 32, borderColor: 'var(--vv-warn-bd)' }}>{t.account.logout}</Button>
      {mobile && <span {...stylex.props(layout.logoutNote)}>{t.account.sharedLogoutNote}</span>}
    </section>
    </div></main></div>
}
