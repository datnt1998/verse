// Màn đăng nhập — POST /api/login của Viderse (kho tài khoản chung): 2xx →
// vào app, 403 app_access_denied (đúng mật khẩu nhưng chưa cấp quyền Viderse)
// → màn bị chặn, 401/429 vẫn chỉ là message server trả về. Không bao giờ
// chuyển hướng sang màn đăng nhập của app khác.
import { useState, type FormEvent } from 'react'
import { ApiError } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Button, FieldError, FieldLabel, PasswordField, TextField } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import { AuthShell, HintLine } from './parts'

export function LoginScreen() {
  const { signIn } = useSession()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || !username.trim() || !password) return
    setBusy(true)
    setError('')
    try {
      await signIn(username.trim(), password)
      // Kết quả vào app hay màn bị chặn do session layer quyết theo response.
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Đăng nhập thất bại')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <div style={{ textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-14)' }}>
        <LensLogo size={64} halo />
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.auth.loginTitle}</h1>
        <p style={{ margin: 'var(--vv-s-6) 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>{t.auth.loginSub}</p>
      </div>
      <form onSubmit={(event) => void submit(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
          <FieldLabel htmlFor="lg-user">{t.auth.username}</FieldLabel>
          <TextField id="lg-user" type="text" autoComplete="username" autoCapitalize="none" spellCheck={false}
            placeholder={t.auth.usernamePlaceholder} hint={t.auth.usernameHint}
            value={username} onChange={setUsername} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
          <FieldLabel htmlFor="lg-pass">{t.auth.password}</FieldLabel>
          <PasswordField id="lg-pass" autoComplete="current-password"
            value={password} onChange={(event) => setPassword(event.currentTarget.value)} />
        </div>
        {error && <FieldError id="lg-err">{error}</FieldError>}
        <Button type="submit" variant="primary" pending={busy} disabled={busy || !username.trim() || !password}
          style={{ height: 42, justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
          {busy ? t.auth.loggingIn : t.auth.login}
        </Button>
        <HintLine icon="shield-security">{t.auth.sessionNote}</HintLine>
      </form>
    </AuthShell>
  )
}
