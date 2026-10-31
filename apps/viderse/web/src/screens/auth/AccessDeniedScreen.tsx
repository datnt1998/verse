import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { BrandMark, Button, FieldError, Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { AuthShell, AuthUser, HintLine } from './parts'
import { authStyles as sx } from './auth.stylex'
import { accountStyles } from '../../components/AppLinks'
import { useMediaQuery } from '../../lib/useMediaQuery'

export function AccessDeniedScreen() {
  const { accessDenied, apps, refresh, signOut } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [busy, setBusy] = useState<'retry' | 'logout' | null>(null)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  if (!accessDenied) return null
  const noverse = apps.find((app) => app.id === 'noverse')
  const retry = async () => {
    if (busy) return
    setBusy('retry'); setError('')
    try { await refresh() } catch { setError(t.auth.serverDownTitle) } finally { setBusy(null) }
  }
  const leave = async () => {
    if (busy) return
    setBusy('logout'); setError('')
    try { await signOut() } catch { setError(t.account.logoutFailed) } finally { setBusy(null) }
  }
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(t.auth.grantRequest(accessDenied.username, accessDenied.name || accessDenied.username))
      setCopied(true)
    } catch { setError(t.auth.copyFailed) }
  }
  return <AuthShell fill>
    <div {...stylex.props(sx.intro)}>
      <span {...stylex.props(sx.icon)}><Icon name="lock" size={26} /></span>
      <h1 {...stylex.props(sx.title)}>{t.auth.accessDeniedTitle}</h1>
      <p {...stylex.props(sx.body)}>{t.auth.accessDenied}</p>
    </div>
    <AuthUser user={accessDenied} />
    {error && <FieldError>{error}</FieldError>}
    <div {...stylex.props(sx.col)}>
      {noverse && <a href={noverse.origin} target="_blank" rel="noopener noreferrer" {...stylex.props(accountStyles.button, accountStyles.primary, sx.submit, sx.otherApp)}>
        <BrandMark size={mobile ? 20 : 18} style={{ width: mobile ? 20 : 18 }} />{t.auth.openNoverse}{!mobile && <span style={{ opacity: .6, fontWeight: 400 }}>{new URL(noverse.origin).host}</span>}
      </a>}
      <Button variant={noverse ? 'ghost' : 'primary'} onClick={() => void retry()} disabled={busy !== null} pending={busy === 'retry'} {...stylex.props(sx.submit, sx.deniedRetry, noverse && sx.retry)}>
        <Icon name="refresh" size={mobile ? 17 : 16} />{t.common.retry}
      </Button>
      <Button variant="quiet" onClick={() => void copy()} {...stylex.props(sx.copy)}><Icon name="copy" size={mobile ? 16 : 15} />{copied ? t.auth.copied : t.auth.copyRequest}</Button>
    </div>
    <div {...stylex.props(sx.col, sx.divider, sx.deniedLogout)}>
      <Button variant="quiet" onClick={() => void leave()} disabled={busy !== null} pending={busy === 'logout'} style={{ color: 'var(--vv-warn-ink)', alignSelf: 'flex-start', height: mobile ? 44 : 36, fontSize: mobile ? 14 : 13, borderRadius: mobile ? 12 : 10, paddingInline: mobile ? 16 : 14 }}>
        <Icon name="logout" size={mobile ? 16 : 15} />{t.account.logout}
      </Button>
      <HintLine icon="info">{t.account.sharedLogoutNote}</HintLine>
    </div>
  </AuthShell>
}
