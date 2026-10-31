import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Avatar, Button, FieldError, Icon, Sheet } from '@noverse/ui'
import { color } from '@noverse/ui/tokens.stylex'
import { useSession } from '../../api/session'
import { membersUrl } from '../../api/sso'
import { accountStyles, AppLinks, ExternalIcon, MembersLink } from '../../components/AppLinks'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { t } from '../../i18n/vi'
import { ChangePasswordForm } from '../auth/ChangePasswordScreen'
import { Devices } from './Devices'
import { navigate, paths } from '../../router'
const MOBILE = '@media (max-width: 767px)'
const sx = stylex.create({
  page: { padding: { default: '28px 32px 48px', [MOBILE]: '4px 16px 20px' }, width: '100%', overflowY: 'auto' },
  content: { maxWidth: 680, marginInline: 'auto', display: 'flex', flexDirection: 'column', gap: { default: 16, [MOBILE]: 14 } },
  card: { borderRadius: { default: 14, [MOBILE]: 16 }, padding: { default: 16, [MOBILE]: 14 }, display: 'flex', flexDirection: 'column', gap: 12 },
  user: { display: 'flex', flexDirection: 'row', alignItems: 'center', gap: 14 },
  title: { margin: 0, fontSize: { default: 22, [MOBILE]: 16 }, fontWeight: 600, letterSpacing: '-.02em' },
  body: { margin: '5px 0 0', fontSize: 13.5, color: color['--color-fg-3'], lineHeight: 1.6 },
  heading: { margin: 0, fontSize: 14, fontWeight: 600 },
  mobileRow: { width: '100%', minHeight: 56, padding: '6px 0', display: 'flex', alignItems: 'center', gap: 12, borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], fontSize: 15, textAlign: 'left' },
})
export function AccountScreen() {
  const { user, apps, signOut } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [sheet, setSheet] = useState<'password' | 'devices' | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  if (!user) return null
  const name = user.name || user.username
  const memberHref = membersUrl(apps)
  const hasOther = apps.some((app) => app.id !== 'viderse')
  const logout = async () => {
    if (busy) return
    setBusy(true); setError('')
    try { await signOut() } catch { setError(t.account.logoutFailed) } finally { setBusy(false) }
  }
  const card = stylex.props(sx.card)
  return <div {...stylex.props(sx.page)}><div {...stylex.props(sx.content)}>
    <div style={mobile ? { display: 'flex', alignItems: 'center', minHeight: 44, gap: 8 } : undefined}>{mobile && <Button variant="quiet" aria-label={t.account.back} onClick={() => navigate(paths.workbench())}><Icon name="arrow-r" size={16} style={{ transform: 'rotate(180deg)' }} /></Button>}<h1 {...stylex.props(sx.title)}>{t.account.title}</h1>{!mobile && <p {...stylex.props(sx.body)}>{t.account.subtitle}</p>}</div>
    <section className={`card ${stylex.props(sx.card, sx.user).className}`} aria-label={t.account.title}>
      <Avatar name={name} size={mobile ? 48 : 52} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><b title={name} style={{ fontSize: 16, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name}</b><span {...stylex.props(accountStyles.tag)}>{t.role[user.role]}</span></span>
        <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}><span className="mono">@{user.username}</span>{user.createdAt && <> · {t.account.memberSince.toLocaleLowerCase('vi')} {new Date(user.createdAt).toLocaleDateString('vi-VN')}</>}</span>
      </span>
    </section>
    {mobile && <p {...stylex.props(sx.body)}>{t.account.subtitle}</p>}
    {hasOther && <section className={`card ${card.className}`} aria-label={t.account.yourApps}>
      <h2 {...stylex.props(sx.heading)}>{mobile ? t.account.apps : t.account.yourApps}</h2>
      {!mobile && <span {...stylex.props(accountStyles.help)}>{user.role === 'admin' ? t.account.adminApps : t.account.memberApps}</span>}
      <div style={{ margin: '0 -10px' }}><AppLinks apps={apps} /></div>
    </section>}
    {user.role === 'admin' && memberHref && <section className={`card ${card.className}`} aria-label={t.account.members} style={mobile ? undefined : { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 14 }}>
      {mobile ? <MembersLink href={memberHref} /> : <>
        <Icon name="crown" size={20} style={{ color: 'var(--color-warn)' }} />
        <span style={{ flex: '1 1 340px', display: 'flex', flexDirection: 'column', gap: 2 }}><h2 {...stylex.props(sx.heading)}>{t.account.members}</h2><span {...stylex.props(accountStyles.help)}>{t.account.membersBody}</span></span>
        <a href={memberHref} target="_blank" rel="noopener noreferrer" {...stylex.props(accountStyles.button)}>{t.account.manageMembers}<ExternalIcon /></a>
      </>}
    </section>}
    {mobile ? <>
      <span {...stylex.props(accountStyles.help)}>{t.account.security}</span>
      <section className={`card ${card.className}`}>
        <button type="button" onClick={() => setSheet('password')} {...stylex.props(sx.mobileRow)}><Icon name="key" size={19} /><span style={{ flex: 1 }}>{t.account.passwordTitle}</span><Icon name="arrow-r" size={15} /></button>
        <button type="button" onClick={() => setSheet('devices')} {...stylex.props(sx.mobileRow)}><Icon name="monitor" size={19} /><span style={{ flex: 1 }}>{t.account.devices}</span><Icon name="arrow-r" size={15} /></button>
      </section>
      <Sheet open={sheet !== null} onClose={() => setSheet(null)} title={sheet === 'password' ? t.account.passwordTitle : t.account.devices} size="md" height="auto" homeIndicator>
        <div style={{ padding: '0 16px 16px', display: 'flex', flexDirection: 'column', gap: 18 }}>
          {sheet === 'password' ? <><p {...stylex.props(sx.body)}>{t.auth.passwordForm.reauthNote}</p><ChangePasswordForm /></> : sheet === 'devices' ? <Devices /> : null}
        </div>
      </Sheet>
    </> : <>
      <section className={`card ${card.className}`} aria-label={t.account.passwordTitle}><h2 {...stylex.props(sx.heading)}>{t.account.passwordTitle}</h2><span {...stylex.props(accountStyles.help)}>{t.auth.passwordForm.reauthNote}</span><ChangePasswordForm horizontal /></section>
      <section className={`card ${card.className}`} aria-label={t.account.devices}><Devices /></section>
    </>}
    {error && <FieldError>{error}</FieldError>}
    <section className={mobile ? undefined : `card ${card.className}`} aria-label={t.account.logout} style={mobile ? { display: 'flex', flexDirection: 'column', gap: 10 } : { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 14 }}>
      {!mobile && <span style={{ flex: '1 1 300px', display: 'flex', flexDirection: 'column', gap: 2 }}><h2 {...stylex.props(sx.heading)}>{t.account.logout}</h2><span {...stylex.props(accountStyles.help)}>{t.account.sharedLogoutNote}</span></span>}
      <Button variant="dsoft" pending={busy} disabled={busy} onClick={() => void logout()} icon={<Icon name="logout" size={16} />} style={{ minHeight: mobile ? 48 : 32, alignSelf: mobile ? 'stretch' : 'flex-start' }}>{t.account.logout}</Button>
      {mobile && <p {...stylex.props(accountStyles.help)} style={{ textAlign: 'center' }}>{t.account.sharedLogoutNote}</p>}
    </section>
  </div></div>
}
