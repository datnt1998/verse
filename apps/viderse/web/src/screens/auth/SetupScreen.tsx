// Màn thiết lập lần đầu (máy chủ chưa có tài khoản nào → setupNeeded):
//   bước 1: nhập mã in trong terminal `viderse serve` → POST /api/setup/verify → setupToken
//   bước 2: tên + tên đăng nhập + mật khẩu quản trị → POST /api/setup/admin → { user, recoveryCode }
//   bước 3: hiện mã khôi phục đúng một lần (server không lưu bản rõ, không lấy lại được)
// Mã thiết lập chỉ sống ở máy chủ; màn này không bao giờ nhận hay hiện lại mã.
import { useState, type FormEvent } from 'react'
import { ApiError, createSetupAdmin, verifySetup } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Banner, Button, Checkbox, Field, FieldError, Icon } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import { AuthError, AuthLabel, AuthPassword, AuthShell } from './parts'
import { useMediaQuery } from '../../lib/useMediaQuery'
import * as stylex from '@stylexjs/stylex'
import { authStyles as sx } from './auth.stylex'

const USERNAME_RE = /^[a-z0-9._-]{2,32}$/
const MIN_PASSWORD = 8

type Step = 'code' | 'account' | 'recovery'

/** Số lần thử còn lại trong payload 400 của /api/setup/verify (`attemptsLeft`). */
const attemptsLeft = (payload: unknown): number | null =>
  payload && typeof payload === 'object' && 'attemptsLeft' in payload && typeof payload.attemptsLeft === 'number'
    ? payload.attemptsLeft
    : null

function StepBar({ index }: { index: number }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  if (mobile) return <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' }}>
    <div role="progressbar" aria-label={t.setup.progress(index + 1)} aria-valuemin={1} aria-valuemax={3} aria-valuenow={index + 1} style={{ display: 'flex', gap: 'var(--vv-s-6)', width: '100%' }}>
      {t.setup.steps.map((label, i) => <span key={label} style={{ flex: 1, height: 4, borderRadius: 'var(--vv-r-pill)', background: i <= index ? 'var(--color-fg)' : 'var(--color-surface-6)' }} />)}
    </div>
    <span className="num" style={{ alignSelf: 'flex-start', fontSize: 12.5, color: 'var(--vv-fg-note)' }}>{t.setup.progress(index + 1)} · {t.setup.steps[index]}</span>
  </div>
  return (
    <ol aria-label={t.setup.stepsAria} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 'var(--vv-s-8)' }}>
      {t.setup.steps.map((label, i) => (
        <li key={label} aria-current={i === index ? 'step' : undefined} style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontSize: 12.5, color: i === index ? 'var(--color-fg)' : 'var(--kit-fg-sub)' }}>
          <span aria-hidden="true" style={{
            width: 22, height: 22, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 11.5, fontWeight: 600, background: i < index ? 'var(--audio-tl-ok-14)' : i === index ? 'var(--color-fg)' : 'var(--color-surface-6)',
            color: i < index ? 'var(--color-ok)' : i === index ? 'var(--color-bg)' : 'var(--color-fg-3)',
          }}>{i < index ? '✓' : i + 1}</span>
          {label}
          {i < 2 && <span aria-hidden="true" style={{ width: 18, height: 1, background: 'var(--kit-wash-14)' }} />}
        </li>
      ))}
    </ol>
  )
}

export function SetupScreen() {
  const { setUser, setSetupNeeded, refresh } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [step, setStep] = useState<Step>('code')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [recovery, setRecovery] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [networkError, setNetworkError] = useState(false)
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [copied, setCopied] = useState(false)

  const verify = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || !code.trim()) return
    setBusy(true)
    setError('')
    setNetworkError(false)
    try {
      const result = await verifySetup(code.trim())
      setToken(result.setupToken)
      setStep('account')
    } catch (e) {
      if (e instanceof ApiError && e.status === 0) { setNetworkError(true); return }
      if (e instanceof ApiError) {
        const left = attemptsLeft(e.payload)
        // 409 = đã có quản trị (tab khác hoàn tất trước): quay về cổng → Đăng nhập.
        if (e.status === 409) { setSetupNeeded(false); return }
        setError(left !== null ? `${e.message} ${t.setup.attemptsLeft(left)}` : e.message)
      } else {
        setError(t.setup.verifyFailed)
      }
    } finally {
      setBusy(false)
    }
  }

  const create = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || !name.trim() || !USERNAME_RE.test(username) || password.length < MIN_PASSWORD) return
    setBusy(true)
    setError('')
    setNetworkError(false)
    try {
      const result = await createSetupAdmin({ setupToken: token, name: name.trim(), username, password })
      setRecovery(result.recoveryCode)
      setStep('recovery')
      // Giữ user để bước recovery xong là vào app luôn (server đã đặt cookie phiên).
      setUser(result.user)
    } catch (e) {
      if (e instanceof ApiError && e.status === 0) { setNetworkError(true); return }
      if (e instanceof ApiError) {
        if (e.status === 409) { setSetupNeeded(false); return }
        // 410 = token hết hạn/dùng rồi: quay lại nhập mã, mã mới (nếu còn) vẫn dùng được.
        if (e.status === 410) { setStep('code'); setToken('') }
        setError(e.message)
      } else {
        setError(t.setup.createFailed)
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell footer={mobile ? undefined : 'setup'} offline={networkError}>
      <div {...stylex.props(sx.intro)}>
        <LensLogo size={mobile ? 48 : 56} halo />
        <h1 {...stylex.props(sx.title)}>{t.setup.title}</h1>
        <p {...stylex.props(sx.body)}>{mobile ? t.setup.bodyMobile : t.setup.body}</p>
      </div>
      <StepBar index={step === 'code' ? 0 : step === 'account' ? 1 : 2} />

      {step === 'code' && (
        <form onSubmit={(event) => void verify(event)} style={{ display: 'flex', flexDirection: 'column', gap: mobile ? 20 : 14 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <AuthLabel htmlFor="setup-code">{t.setup.codeLabel}</AuthLabel>
            <Field id="setup-code" autoComplete="one-time-code" autoCapitalize="characters" spellCheck={false}
              value={code} onChange={(event) => setCode(event.currentTarget.value)} {...stylex.props(sx.input)}
              style={{ height: mobile ? 52 : 44, borderRadius: 'var(--vv-r-10)', textAlign: 'center', fontFamily: 'var(--font-mono)', fontSize: mobile ? 20 : 18, letterSpacing: '0.18em' }} />
            <span style={{ fontSize: 11.5, color: 'var(--vv-fg-note)', lineHeight: 1.5 }}>{mobile ? t.setup.codeHintMobile : t.setup.codeHint}</span>
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          {networkError && <Banner tone="error" title={t.account.networkFailed} titleStyle={sx.networkTitle} icon="danger" iconSize={16} role="alert" align="top" />}
          <Button type="submit" variant="primary" pending={busy} disabled={busy || !code.trim()}
            {...stylex.props(sx.submit)}>
            {busy ? t.setup.verifying : t.common.continue}
          </Button>
        </form>
      )}

      {step === 'account' && (
        <form onSubmit={(event) => void create(event)} style={{ display: 'flex', flexDirection: 'column', gap: mobile ? 20 : 14 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <AuthLabel htmlFor="setup-name">{t.setup.nameLabel}</AuthLabel>
            <Field id="setup-name" autoComplete="name" value={name} onChange={(event) => setName(event.currentTarget.value)} {...stylex.props(sx.input)} style={{ borderRadius: 'var(--vv-r-10)' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <AuthLabel htmlFor="setup-user">{t.setup.usernameLabel}</AuthLabel>
            <Field id="setup-user" autoComplete="username" autoCapitalize="none" spellCheck={false}
              value={username} onChange={(event) => setUsername(event.currentTarget.value.trim().toLowerCase())}
              {...stylex.props(sx.input)} style={{ fontFamily: 'var(--font-mono)', borderRadius: 'var(--vv-r-10)' }} />
            {!mobile && <span style={{ fontSize: 11.5, color: 'var(--vv-fg-note)', lineHeight: 1.5 }}>{t.auth.usernameHint}</span>}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <AuthLabel htmlFor="setup-pass">{t.setup.passwordLabel}</AuthLabel>
            <AuthPassword id="setup-pass" autoComplete="new-password"
              value={password} onChange={(event) => setPassword(event.currentTarget.value)} />
            <span style={{ fontSize: 11.5, color: 'var(--vv-fg-note)', lineHeight: 1.5 }}>{t.setup.passwordHint}</span>
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          {networkError && <Banner tone="error" title={t.account.networkFailed} titleStyle={sx.networkTitle} icon="danger" iconSize={16} role="alert" align="top" />}
          <Button type="submit" variant="primary" pending={busy}
            disabled={busy || !name.trim() || !USERNAME_RE.test(username) || password.length < MIN_PASSWORD}
            {...stylex.props(sx.submit)}>
            {busy ? t.setup.creating : t.setup.createAdmin}
          </Button>
        </form>
      )}

      {step === 'recovery' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: mobile ? 20 : 14 }}>
          <Banner tone="success" title={`${t.setup.recoveryTitle} ${name}.${mobile ? '' : ` ${t.setup.signedIn}`}`} icon="tick-b" iconSize={16} titleStyle={sx.recoveryTitle} />
          {recovery !== null ? <div style={{ display: mobile ? 'contents' : 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' }}>
            {!mobile && <span style={{ fontSize: 11.5, color: 'var(--kit-fg-meta)' }}>{t.setup.recoveryLabel}</span>}
            <div className="card" style={{ borderRadius: mobile ? 'var(--vv-r-14)' : 'var(--vv-r-12)', padding: 'var(--vv-s-14)', display: 'flex', flexDirection: mobile ? 'column' : 'row', alignItems: mobile ? 'stretch' : 'center', gap: mobile ? 10 : 12 }}>
              {mobile && <span style={{ fontSize: 11.5, color: 'var(--kit-fg-meta)' }}>{t.setup.recoveryLabel}</span>}
              <span className="mono num" style={{ flex: 1, minWidth: 0, overflowWrap: 'anywhere', userSelect: 'all', fontSize: mobile ? 18 : 17, letterSpacing: mobile ? '.08em' : '.1em', textAlign: mobile ? 'center' : undefined }}>{recovery}</span>
              <Button size={mobile ? 'md' : 'sm'} style={{ height: mobile ? 44 : 30, ...(mobile ? { fontSize: 14, borderRadius: 'var(--vv-r-12)', paddingInline: 16 } : {}) }} onClick={() => {
                void (async () => {
                  try { await navigator.clipboard.writeText(recovery); setCopied(true); setError('') }
                  catch { setError(mobile ? t.auth.copyRecoveryFailedMobile : t.auth.copyRecoveryFailedDesktop) }
                })()
              }}><Icon name="copy" size={mobile ? 16 : 14} />{copied ? t.auth.copied : mobile ? t.setup.copyRecoveryMobile : t.setup.copyRecovery}</Button>
              {mobile && error && <AuthError>{error}</AuthError>}
            </div>
            {!mobile && error && <AuthError>{error}</AuthError>}
            <span style={{ fontSize: mobile ? 13 : 12.5, color: 'var(--vv-note-ink)', lineHeight: 1.55 }}>{mobile ? t.setup.recoveryWarningMobile : t.setup.recoveryWarning}</span>
          </div> : <Banner tone="error" role="status" title={t.setup.recoveryUnavailableTitle} titleStyle={sx.missingRecoveryTitle} textStyle={sx.missingRecoveryText} icon="danger" iconSize={16} align="top">{t.setup.recoveryUnavailable}</Banner>}
          {recovery !== null && <Checkbox checked={saved} onChange={setSaved} label={t.setup.saved} size={mobile ? 18 : 16} style={{ minHeight: 0, height: mobile ? 44 : 'auto', margin: 0, padding: 0, gap: mobile ? 12 : 10, fontSize: mobile ? 15 : 13.5, color: 'var(--color-fg)' }} />}
          <Button type="button" variant="primary" pending={busy} disabled={busy || (recovery !== null && !saved)} onClick={async () => {
            setBusy(true)
            await refresh()
            setSetupNeeded(false)
            setBusy(false)
          }}
            {...stylex.props(sx.submit)}>
            {t.setup.enter}
          </Button>
        </div>
      )}

    </AuthShell>
  )
}
