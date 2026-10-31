// Đổi mật khẩu — một form hai ngữ cảnh:
//   · Bắt buộc (server đánh dấu mustChangePassword): ChangePasswordScreen đứng
//     trước shell, không nút bỏ qua — hash route không vượt được.
//   · Tự nguyện (màn Tài khoản): AccountScreen nhúng ChangePasswordForm.
// Server đổi xong là thu hồi MỌI phiên (cả app kia) và xoá cookie: thành công
// dọn phiên UI + báo "đăng nhập lại", tuyệt đối không refresh /api/me sau đó
// (sẽ 401 như thể đổi thất bại). Sai mật khẩu hiện tại (403) và lỗi kho
// (503/lỗi khác) hiện hai thông báo khác nhau.
import { useState, type FormEvent } from 'react'
import { ApiError, changePassword } from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { Button, FieldError, FieldLabel, Icon, PasswordField, toast } from '@noverse/ui'
import { AuthShell, HintLine } from './parts'

const MIN_PASSWORD = 8

/**
 * Form đổi mật khẩu của chính mình. Thành công → toast + endSession (máy chủ
 * đã thu hồi mọi phiên). `forced` chỉ đổi nhãn trường hiện tại + dòng dẫn.
 */
export function ChangePasswordForm({ forced = false }: { forced?: boolean }) {
  const { endSession } = useSession()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (busy || !current || next.length < MIN_PASSWORD || next !== confirm) return
    setBusy(true)
    setError('')
    try {
      await changePassword(current, next)
      // Phiên đã bị thu hồi phía máy chủ — dọn UI về Đăng nhập, không gọi lại me.
      toast.ok(t.auth.passwordForm.changed)
      endSession()
    } catch (e) {
      // 403 = sai mật khẩu hiện tại — thông báo riêng, không trôi chung với
      // 503 kho tài khoản hay mất kết nối.
      setError(e instanceof ApiError ? (e.status === 403 ? t.auth.passwordForm.wrongCurrent : e.message) : t.auth.passwordForm.failed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <FieldLabel htmlFor="pw-current">{forced ? t.auth.passwordForm.currentTemp : t.auth.passwordForm.current}</FieldLabel>
        <PasswordField id="pw-current" autoComplete="current-password"
          value={current} onChange={(event) => setCurrent(event.currentTarget.value)} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <FieldLabel htmlFor="pw-next">{t.auth.passwordForm.next}</FieldLabel>
        <PasswordField id="pw-next" autoComplete="new-password" placeholder={t.auth.passwordForm.hint}
          value={next} onChange={(event) => setNext(event.currentTarget.value)} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <FieldLabel htmlFor="pw-confirm">{t.auth.passwordForm.confirm}</FieldLabel>
        <PasswordField id="pw-confirm" autoComplete="new-password"
          value={confirm} onChange={(event) => setConfirm(event.currentTarget.value)} />
      </div>
      {confirm.length > 0 && next !== confirm && <FieldError id="pw-mismatch">{t.auth.passwordForm.mismatch}</FieldError>}
      {error && <FieldError id="pw-error">{error}</FieldError>}
      <Button type="submit" variant="primary" pending={busy}
        disabled={busy || !current || next.length < MIN_PASSWORD || next !== confirm}
        style={{ minHeight: 'var(--vv-s-44)', justifyContent: 'center', fontSize: 14, fontWeight: 600 }}>
        {busy ? t.auth.passwordForm.changing : t.auth.passwordForm.submit}
      </Button>
    </form>
  )
}

/** Cổng ép đổi mật khẩu tạm — render thay shell, không có đường tắt nào khác. */
export function ChangePasswordScreen() {
  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 'var(--vv-s-14)', textAlign: 'center' }}>
        <span aria-hidden="true" style={{
          width: 56, height: 56, borderRadius: '50%', display: 'grid', placeItems: 'center',
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', color: 'var(--color-fg-2)',
        }}>
          <Icon name="key" size={26} />
        </span>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.auth.passwordForm.forceTitle}</h1>
        <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 380 }}>{t.auth.passwordForm.forceBody}</p>
      </div>
      <ChangePasswordForm forced />
      <HintLine icon="shield-security">{t.auth.passwordForm.reauthNote}</HintLine>
    </AuthShell>
  )
}
