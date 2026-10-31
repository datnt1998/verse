// Form “Dùng mã khôi phục” trong khung Quên mật khẩu (Login.dc.html › recoverOpen), chỉ cho quản trị viên.
// Nằm trong <form> đăng nhập nên dùng nút type=button; Enter trong ô nhập chạy khôi phục, không đăng nhập.
import { useEffect, useState, type KeyboardEvent } from 'react'
import { accessDeniedUser, recoverAdmin } from '../../api/client'
import type { User } from '../../api/types'
import { useSession } from '../../api/session'
import { useAction } from '../../data'
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { AuthInput } from './parts'
import { recoveryFailure, recoveryReady, type RecoveryFailure } from './recovery'

export function RecoveryForm({ username, usernameOk, onRecovered }: {
  username: string
  /** Chưa có tên đăng nhập hợp lệ (màn “tài khoản khác”) thì chưa gửi được. */
  usernameOk: boolean
  onRecovered: (user: User) => void
}) {
  const { setDeniedUser } = useSession()
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [failure, setFailure] = useState<RecoveryFailure | null>(null)
  const [retryLeft, setRetryLeft] = useState(0)
  // Lỗi hiện tại chỗ (dòng lỗi + viền ô), không toast trùng (16 §3).
  const action = useAction(recoverAdmin, { error: false })

  useEffect(() => {
    if (retryLeft <= 0) return
    const timer = setTimeout(() => setRetryLeft((n) => Math.max(0, n - 1)), 1000)
    return () => clearTimeout(timer)
  }, [retryLeft])

  useEffect(() => {
    if (!action.error) return
    // Mã đúng + mật khẩu đặt xong nhưng chưa được cấp quyền app → màn chặn.
    const denied = accessDeniedUser(action.error)
    if (denied) { setDeniedUser(denied); return }
    const next = recoveryFailure(action.error)
    setFailure(next)
    if (next.kind === 'limited') setRetryLeft(next.retryAfter)
  }, [action.error])

  const limited = retryLeft > 0
  const ready = recoveryReady(code, password) && usernameOk && !limited
  const disabled = !ready || action.busy

  const recover = async () => {
    if (disabled) return
    setFailure(null)
    const result = await action.run(username, code, password)
    if (result) onRecovered(result.user)
  }
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Enter') return
    event.preventDefault()
    void recover()
  }
  const edit = (set: (value: string) => void) => (value: string) => {
    set(value)
    if (failure && failure.kind !== 'limited') setFailure(null)
  }

  return (
    <div onKeyDown={onKeyDown} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <label htmlFor="lg-rc" className="lbl">{t.auth.recoveryCode}</label>
      <AuthInput id="lg-rc" type="text" value={code} placeholder={t.auth.recoveryPlaceholder} autoComplete="one-time-code"
        invalid={failure?.kind === 'code'} aria-describedby={failure ? 'lg-rc-err' : undefined}
        style={{ height: 38, letterSpacing: '.06em', fontSize: 14 }} onChange={edit((value) => setCode(value.toUpperCase()))} />
      <label htmlFor="lg-rp" className="lbl">{t.auth.newPassword}</label>
      <AuthInput id="lg-rp" type="password" value={password} autoComplete="new-password"
        invalid={failure?.kind === 'password'} aria-describedby={failure ? 'lg-rc-err' : undefined}
        style={{ height: 38, fontSize: 14 }} onChange={edit(setPassword)} />
      {failure && (
        <span id="lg-rc-err" role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}>
          <Icon name="warning-2" size={14} />{failure.message}
        </span>
      )}
      <button type="button" className="hov-primary" aria-busy={action.pending} disabled={disabled} onClick={() => void recover()} style={{
        height: 36, borderRadius: 10, border: 'none', fontSize: 13, fontWeight: 600,
        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        background: ready || action.busy ? 'var(--color-fg)' : 'rgba(255,255,255,0.08)',
        color: ready || action.busy ? 'var(--color-bg)' : 'var(--color-fg-4)',
      }}>
        {action.pending && <Spinner size={14} style={{ borderColor: 'rgba(0,0,0,0.15)', borderTopColor: 'var(--color-bg)' }} />}
        {limited ? `Thử lại sau ${Math.floor(retryLeft / 60)}:${String(retryLeft % 60).padStart(2, '0')}` : t.auth.recoverSubmit}
      </button>
      <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.auth.recoverNote}</span>
    </div>
  )
}
