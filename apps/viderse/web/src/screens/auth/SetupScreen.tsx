// Màn thiết lập lần đầu (máy chủ chưa có tài khoản nào → setupNeeded):
//   bước 1: nhập mã in trong terminal `viderse serve` → POST /api/setup/verify → setupToken
//   bước 2: tên + tên đăng nhập + mật khẩu quản trị → POST /api/setup/admin → { user, recoveryCode }
//   bước 3: hiện mã khôi phục đúng một lần (server không lưu bản rõ, không lấy lại được)
// Mã thiết lập chỉ sống ở máy chủ; màn này không bao giờ nhận hay hiện lại mã.
import { useState, type FormEvent } from 'react'
import { ApiError, createSetupAdmin, verifySetup } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Button, FieldError, FieldLabel, Icon, PasswordField, TextField } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import { AuthShell, HintLine } from './parts'

const USERNAME_RE = /^[a-z0-9._-]{2,32}$/
const MIN_PASSWORD = 8

type Step = 'code' | 'account' | 'recovery'

/** Số lần thử còn lại trong payload 400 của /api/setup/verify (`attemptsLeft`). */
const attemptsLeft = (payload: unknown): number | null =>
  payload && typeof payload === 'object' && 'attemptsLeft' in payload && typeof payload.attemptsLeft === 'number'
    ? payload.attemptsLeft
    : null

function StepBar({ index }: { index: number }) {
  return (
    <ol aria-label={t.setup.stepsAria} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 'var(--vv-s-10)' }}>
      {t.setup.steps.map((label, i) => (
        <li key={label} aria-current={i === index ? 'step' : undefined} style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-6)', fontSize: 12, color: i === index ? 'var(--color-fg)' : 'var(--color-fg-4)', fontWeight: i === index ? 600 : 400 }}>
          <span aria-hidden="true" style={{
            width: 18, height: 18, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 11, background: i <= index ? 'var(--color-accent)' : 'var(--color-surface-2)',
            color: i <= index ? 'var(--color-bg)' : 'var(--color-fg-4)',
          }}>{i + 1}</span>
          {label}
        </li>
      ))}
    </ol>
  )
}

export function SetupScreen() {
  const { setUser, setSetupNeeded } = useSession()
  const [step, setStep] = useState<Step>('code')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [recovery, setRecovery] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

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
        setError('Không kiểm được mã thiết lập')
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
        setError('Không tạo được quản trị viên')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-14)', textAlign: 'center' }}>
        <LensLogo size={64} halo />
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.setup.title}</h1>
        <StepBar index={step === 'code' ? 0 : step === 'account' ? 1 : 2} />
      </div>

      {step === 'code' && (
        <form onSubmit={(event) => void verify(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-code">{t.setup.codeLabel}</FieldLabel>
            <TextField id="setup-code" autoComplete="off" autoCapitalize="characters" spellCheck={false}
              placeholder={t.setup.codePlaceholder} hint={t.setup.codeHint}
              value={code} onChange={setCode} style={{ fontFamily: 'var(--font-mono)', letterSpacing: '0.08em' }} />
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          <Button type="submit" variant="primary" pending={busy} disabled={busy || !code.trim()}
            style={{ height: 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {busy ? t.setup.verifying : t.common.continue}
          </Button>
        </form>
      )}

      {step === 'account' && (
        <form onSubmit={(event) => void create(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-name">{t.setup.nameLabel}</FieldLabel>
            <TextField id="setup-name" autoComplete="name" value={name} onChange={setName} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-user">{t.setup.usernameLabel}</FieldLabel>
            <TextField id="setup-user" autoComplete="username" autoCapitalize="none" spellCheck={false}
              placeholder={t.auth.usernamePlaceholder} hint={t.auth.usernameHint}
              value={username} onChange={(value) => setUsername(value.trim().toLowerCase())}
              style={{ fontFamily: 'var(--font-mono)' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
            <FieldLabel htmlFor="setup-pass">{t.setup.passwordLabel}</FieldLabel>
            <PasswordField id="setup-pass" autoComplete="new-password" placeholder={t.setup.passwordHint}
              value={password} onChange={(event) => setPassword(event.currentTarget.value)} />
          </div>
          {error && <FieldError id="setup-err">{error}</FieldError>}
          <Button type="submit" variant="primary" pending={busy}
            disabled={busy || !name.trim() || !USERNAME_RE.test(username) || password.length < MIN_PASSWORD}
            style={{ height: 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {busy ? t.setup.creating : t.setup.createAdmin}
          </Button>
        </form>
      )}

      {step === 'recovery' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
          <div className="card" role="alert" style={{
            borderRadius: 'var(--radius-tile)', padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)',
            borderColor: 'rgba(246,203,140,0.3)',
          }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontWeight: 600 }}>
              <Icon name="key" size={16} style={{ color: 'var(--color-warn)' }} />{t.setup.recoveryTitle}
            </span>
            {recovery !== null ? (
              <>
                <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{t.setup.recoveryWarning}</span>
                <div>
                  <span className="lbl">{t.setup.recoveryLabel}</span>
                  <p className="mono num" style={{ margin: 'var(--vv-s-6) 0 0', fontSize: 18, letterSpacing: '0.06em', color: 'var(--color-fg)' }}>{recovery}</p>
                </div>
              </>
            ) : (
              // Server đã tạo admin + đặt phiên nhưng chưa cấp được mã khôi phục:
              // vẫn vào app bình thường, cấp lại mã qua luồng khôi phục của Noverse.
              <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{t.setup.recoveryUnavailable}</span>
            )}
          </div>
          <Button type="button" variant="primary" onClick={() => setSetupNeeded(false)}
            style={{ height: 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
            {t.setup.enter}
          </Button>
        </div>
      )}

      {step !== 'recovery' && <HintLine icon="shield-security">{t.auth.sessionNote}</HintLine>}
    </AuthShell>
  )
}
