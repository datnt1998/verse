// Màn thiết lập lần đầu (máy chủ chưa có tài khoản nào → setupNeeded):
//   bước 1: nhập mã in trong terminal `viderse serve` → POST /api/setup/verify → setupToken
//   bước 2: tên + tên đăng nhập + mật khẩu quản trị → POST /api/setup/admin → { user, recoveryCode }
//   bước 3: hiện mã khôi phục đúng một lần (server không lưu bản rõ, không lấy lại được)
// Mã thiết lập chỉ sống ở máy chủ; màn này không bao giờ nhận hay hiện lại mã.
import { useState, type FormEvent } from 'react'
import { ApiError, createSetupAdmin, verifySetup } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Banner, Button, Checkbox, FieldError, FieldLabel, Icon, PasswordField, TextField } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import { authLabelStyle, AuthShell } from './parts'
import { useMediaQuery } from '../../lib/useMediaQuery'

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
  if (mobile) return <>
    <div role="progressbar" aria-label={t.setup.progress(index + 1)} aria-valuemin={1} aria-valuemax={3} aria-valuenow={index + 1} style={{ display: 'flex', gap: 6, width: '100%' }}>
      {t.setup.steps.map((label, i) => <span key={label} style={{ flex: 1, height: 4, borderRadius: 999, background: i <= index ? 'var(--color-fg)' : 'var(--color-surface-6)' }} />)}
    </div>
    <span className="num" style={{ alignSelf: 'flex-start', fontSize: 12.5, color: 'var(--vv-fg-note)' }}>{t.setup.progress(index + 1)} · {t.setup.steps[index]}</span>
  </>
  return (
    <ol aria-label={t.setup.stepsAria} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8 }}>
      {t.setup.steps.map((label, i) => (
        <li key={label} aria-current={i === index ? 'step' : undefined} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: i === index ? 'var(--color-fg)' : 'var(--color-fg-4)' }}>
          <span aria-hidden="true" style={{
            width: 22, height: 22, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 11.5, fontWeight: 600, background: i <= index ? 'var(--color-fg)' : 'var(--color-surface-6)',
            color: i <= index ? 'var(--color-bg)' : 'var(--color-fg-4)',
          }}>{i + 1}</span>
          {label}
          {i < 2 && <span aria-hidden="true" style={{ width: 12, height: 1, background: 'var(--line-4)' }} />}
        </li>
      ))}
    </ol>
  )
}

export function SetupScreen() {
  const { setUser, setSetupNeeded } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [step, setStep] = useState<Step>('code')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [recovery, setRecovery] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [saved, setSaved] = useState(false)
  const [copied, setCopied] = useState(false)

  const verify = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || !code.trim()) return
    setBusy(true)
    setError('')
    try {
      const result = await verifySetup(code.trim())
      setToken(result.setupToken)
      setStep('account')
    } catch (e) {
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
    try {
      const result = await createSetupAdmin({ setupToken: token, name: name.trim(), username, password })
      setRecovery(result.recoveryCode)
      setStep('recovery')
      // Giữ user để bước recovery xong là vào app luôn (server đã đặt cookie phiên).
      setUser(result.user)
    } catch (e) {
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
    <AuthShell footer={mobile ? undefined : 'setup'}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-14)', textAlign: 'center' }}>
        <LensLogo size={mobile ? 48 : 56} halo />
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.setup.title}</h1>
        <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-3)' }}>{t.setup.body}</p>
        <StepBar index={step === 'code' ? 0 : step === 'account' ? 1 : 2} />
      </div>

      {step === 'code' && (
        <form onSubmit={(event) => void verify(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-code" size={12.5} style={authLabelStyle}>{t.setup.codeLabel}</FieldLabel>
            <TextField id="setup-code" autoComplete="one-time-code" autoCapitalize="characters" spellCheck={false}
              placeholder={t.setup.codePlaceholder} hint={t.setup.codeHint}
              value={code} onChange={setCode} style={{ height: mobile ? 52 : 44, textAlign: 'center', fontFamily: 'var(--font-mono)', fontSize: mobile ? 20 : 18, letterSpacing: '0.18em' }} />
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          <Button type="submit" variant="primary" pending={busy} disabled={busy || !code.trim()}
            style={{ height: mobile ? 48 : 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {busy ? t.setup.verifying : t.common.continue}
          </Button>
        </form>
      )}

      {step === 'account' && (
        <form onSubmit={(event) => void create(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-name" size={12.5} style={authLabelStyle}>{t.setup.nameLabel}</FieldLabel>
            <TextField id="setup-name" autoComplete="name" value={name} onChange={setName} style={{ height: mobile ? 48 : 40, fontSize: mobile ? 16 : 13 }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-user" size={12.5} style={authLabelStyle}>{t.setup.usernameLabel}</FieldLabel>
            <TextField id="setup-user" autoComplete="username" autoCapitalize="none" spellCheck={false}
              placeholder={t.auth.usernamePlaceholder} hint={t.auth.usernameHint}
              value={username} onChange={(value) => setUsername(value.trim().toLowerCase())}
              style={{ fontFamily: 'var(--font-mono)', height: mobile ? 48 : 40, fontSize: mobile ? 16 : 13 }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-pass" size={12.5} style={authLabelStyle}>{t.setup.passwordLabel}</FieldLabel>
            <PasswordField id="setup-pass" autoComplete="new-password" placeholder={t.setup.passwordHint}
              value={password} onChange={(event) => setPassword(event.currentTarget.value)} style={{ height: mobile ? 48 : 40, fontSize: mobile ? 16 : 13 }} />
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          <Button type="submit" variant="primary" pending={busy}
            disabled={busy || !name.trim() || !USERNAME_RE.test(username) || password.length < MIN_PASSWORD}
            style={{ height: mobile ? 48 : 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {busy ? t.setup.creating : t.setup.createAdmin}
          </Button>
        </form>
      )}

      {step === 'recovery' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <Banner tone="success" title={`${t.setup.recoveryTitle} ${name}. ${t.setup.signedIn}`} icon="tick-b" />
          {recovery !== null ? <>
            {!mobile && <span className="lbl">{t.setup.recoveryLabel}</span>}
            <div className="card" style={{ borderRadius: mobile ? 14 : 12, padding: 14, display: 'flex', flexDirection: mobile ? 'column' : 'row', alignItems: mobile ? 'stretch' : 'center', gap: mobile ? 10 : 12 }}>
              {mobile && <span className="lbl">{t.setup.recoveryLabel}</span>}
              <span className="mono num" style={{ flex: 1, minWidth: 0, overflowWrap: 'anywhere', fontSize: mobile ? 18 : 17, letterSpacing: '.1em', textAlign: mobile ? 'center' : undefined }}>{recovery}</span>
              <Button size={mobile ? 'md' : 'sm'} onClick={() => {
                void navigator.clipboard.writeText(recovery).then(() => setCopied(true), () => setError(t.auth.copyFailed))
              }}><Icon name="copy" size={14} />{copied ? t.auth.copied : t.common.copy}</Button>
            </div>
            <span style={{ fontSize: 12.5, color: 'var(--color-warn)', lineHeight: 1.55 }}>{t.setup.recoveryWarning}</span>
          </> : <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{t.setup.recoveryUnavailable}</span>}
          {error && <FieldError>{error}</FieldError>}
          {recovery !== null && <Checkbox checked={saved} onChange={setSaved} label={t.setup.saved} />}
          <Button type="button" variant="primary" disabled={recovery !== null && !saved} onClick={() => setSetupNeeded(false)}
            style={{ height: mobile ? 48 : 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {t.setup.enter}
          </Button>
        </div>
      )}

    </AuthShell>
  )
}
