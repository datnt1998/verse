import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, FieldError, Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { AuthShell, HintLine } from './parts'
import { authStyles as sx } from './auth.stylex'

export function ServerDownScreen() {
  const { refresh, outage } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [remaining, setRemaining] = useState(12)
  const retry = async () => {
    if (busy) return
    setBusy(true); setError('')
    try { await refresh() } catch { setError(t.auth.serverDownTitle) } finally { setBusy(false); setRemaining(12) }
  }
  useEffect(() => {
    if (busy) return
    const timer = setTimeout(() => remaining > 1 ? setRemaining(remaining - 1) : void retry(), 1000)
    return () => clearTimeout(timer)
  }, [remaining, busy])
  const accounts = outage === 'accounts'
  return <AuthShell centered>
    <div {...stylex.props(sx.intro)}>
      <span {...stylex.props(sx.icon)}><Icon name="danger" size={26} /></span>
      <h1 {...stylex.props(sx.title)}>{accounts ? t.auth.accountsDownTitle : t.auth.serverDownTitle}</h1>
      <p {...stylex.props(sx.body)}>{accounts ? t.auth.accountsDownBody : t.auth.serverDownBody}</p>
    </div>
    <div {...stylex.props(sx.col)}>
      <Button variant="primary" onClick={() => void retry()} pending={busy} disabled={busy} {...stylex.props(sx.submit)}><Icon name="refresh" size={16} />{t.common.retry}</Button>
      <span className="num" style={{ textAlign: 'center', fontSize: 12.5, color: 'var(--color-fg-4)' }}>{t.auth.retryIn(remaining)}</span>
    </div>
    {error && <FieldError>{error}</FieldError>}
    <HintLine icon="info">{accounts ? t.auth.accountsDownHint : t.auth.serverDownHint}</HintLine>
  </AuthShell>
}
