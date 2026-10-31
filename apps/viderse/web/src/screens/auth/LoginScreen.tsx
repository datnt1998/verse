import { useEffect, useState, type FormEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { ApiError } from '../../api/client'
import { useSession } from '../../api/session'
import type { SessionEnded } from '../../api/types'
import { t } from '../../i18n/vi'
import { Banner, Button, FieldError, FieldLabel, PasswordField, TextField } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import { authLabelStyle, AuthShell, HintLine } from './parts'
import { authStyles as sx } from './auth.stylex'

import { useMediaQuery } from '../../lib/useMediaQuery'
export function sessionExplanation(ended: SessionEnded) {
  switch (ended.reason) {
    case 'logout': return ended.app === 'noverse' ? t.auth.kicked.other : ended.app === 'viderse' ? t.auth.kicked.self : t.auth.kicked.unknown
    case 'password_changed': return t.auth.kicked.password
    case 'locked':
    case 'admin_changed': return t.auth.kicked.admin
    case 'expired': return t.auth.kicked.expired
    default: return t.auth.kicked.unknown
  }
}
export function LoginScreen() {
  const { signIn, ended } = useSession()
  const [username, setUsername] = useState('')
  const mobile = useMediaQuery('(max-width: 767px)')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [remaining, setRemaining] = useState(0)
  useEffect(() => {
    if (!remaining) return
    const timer = setTimeout(() => setRemaining((n) => Math.max(0, n - 1)), 1000)
    return () => clearTimeout(timer)
  }, [remaining])
  const time = `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')}`
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || remaining || !username.trim() || !password) return
    setBusy(true); setError('')
    try { await signIn(username.trim(), password) }
    catch (e) {
      if (e instanceof ApiError && e.status === 429) setRemaining(Math.max(1, Math.ceil(e.retryAfter ?? 30)))
      else setError(e instanceof ApiError && e.status === 401 ? t.auth.wrongLogin : t.auth.loginFailed)
    } finally { setBusy(false) }
  }
  const explanation = ended ? sessionExplanation(ended) : null
  const input = stylex.props(sx.input)
  const button = stylex.props(sx.submit)
  return <AuthShell footer="login">
    <div {...stylex.props(sx.intro)}>
      <LensLogo size={mobile ? 48 : 64} halo />
      <h1 {...stylex.props(sx.title)}>{t.auth.loginTitle}</h1>
      <p {...stylex.props(sx.body)}>{t.auth.loginSub}</p>
    </div>
    {explanation && <div data-session-ended><Banner title={explanation.title} tone={ended?.reason === 'logout' && ended.app === 'viderse' ? 'success' : 'info'} icon={ended?.reason === 'password_changed' ? 'key' : ended?.reason === 'expired' ? 'clock' : ended?.reason === 'admin_changed' || ended?.reason === 'locked' ? 'crown' : ended?.reason === 'logout' && ended.app === 'viderse' ? 'tick-b' : 'info'}>
      {explanation.body}
    </Banner></div>}
    <form onSubmit={(event) => void submit(event)} {...stylex.props(sx.form)}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <FieldLabel htmlFor="lg-user" size={12.5} style={authLabelStyle}>{t.auth.username}</FieldLabel>
        <TextField id="lg-user" type="text" autoComplete="username" autoCapitalize="none" spellCheck={false}
          placeholder={t.auth.usernamePlaceholder} aria-invalid={!!error || remaining > 0} value={username} onChange={setUsername}
          className={input.className} style={{ ...input.style, fontFamily: 'var(--font-mono)' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <FieldLabel htmlFor="lg-pass" size={12.5} style={authLabelStyle}>{t.auth.password}</FieldLabel>
        <PasswordField id="lg-pass" autoComplete="current-password" invalid={!!error || remaining > 0}
          value={password} onChange={(event) => setPassword(event.currentTarget.value)} {...input} />
      </div>
      {(error || remaining > 0) && <FieldError>{remaining ? t.auth.limited(time) : error}</FieldError>}
      <Button type="submit" variant="primary" pending={busy} disabled={busy || remaining > 0 || !username.trim() || !password} {...button}>
        {remaining ? t.auth.wait(time) : busy ? t.auth.loggingIn : t.auth.login}
      </Button>
      {mobile ? <p style={{ margin: 0, textAlign: 'center', fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>{t.auth.sessionNote}</p> : <HintLine icon="shield-tick">{t.auth.sessionNote}</HintLine>}
    </form>
  </AuthShell>
}
