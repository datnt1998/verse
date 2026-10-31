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
import { Button, FieldError, Icon, toast } from '@noverse/ui'
import { AuthError, AuthLabel, AuthPassword, AuthShell, AuthUser, HintLine } from './parts'
import * as stylex from '@stylexjs/stylex'
import { authStyles as sx } from './auth.stylex'
import { accountStyles } from '../../components/AppLinks'
import { useMediaQuery } from '../../lib/useMediaQuery'

const MIN_PASSWORD = 8

/**
 * Form đổi mật khẩu của chính mình. Thành công → toast + endSession (máy chủ
 * đã thu hồi mọi phiên). `forced` chỉ đổi nhãn trường hiện tại + dòng dẫn.
 */
export function ChangePasswordForm({ forced = false, horizontal = false }: { forced?: boolean; horizontal?: boolean }) {
  const mobile = useMediaQuery('(max-width: 767px)')
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
      window.location.hash = '/login?pw=1'
      endSession({ reason: 'password_changed' })
    } catch (e) {
      // 403 = sai mật khẩu hiện tại — thông báo riêng, không trôi chung với
      // 503 kho tài khoản hay mất kết nối.
      setError(e instanceof ApiError ? (e.status === 403 ? (forced ? t.auth.passwordForm.wrongTemp : t.auth.passwordForm.wrongCurrent) : t.auth.passwordForm.failed) : t.auth.passwordForm.failed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} style={{ display: 'flex', flexDirection: 'column', gap: mobile ? (forced ? 20 : 12) : horizontal ? 12 : 14 }}>
      <div style={{ display: 'grid', gridTemplateColumns: horizontal ? 'repeat(3, minmax(0, 1fr))' : 'minmax(0, 1fr)', gap: mobile ? (forced ? 20 : 12) : horizontal ? 12 : 14 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <AuthLabel htmlFor="pw-current">{forced ? t.auth.passwordForm.currentTemp : t.auth.passwordForm.current}</AuthLabel>
        <AuthPassword id="pw-current" autoComplete="current-password"
          value={current} invalid={!!error} onChange={(event) => setCurrent(event.currentTarget.value)} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <AuthLabel htmlFor="pw-next">{t.auth.passwordForm.next}</AuthLabel>
        <AuthPassword id="pw-next" autoComplete="new-password"
          value={next} invalid={next.length > 0 && next.length < MIN_PASSWORD} onChange={(event) => setNext(event.currentTarget.value)} />
        <span {...stylex.props(accountStyles.fieldHelp)}>{t.auth.passwordForm.hint}</span>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
        <AuthLabel htmlFor="pw-confirm">{forced ? t.auth.passwordForm.confirm : t.auth.passwordForm.confirmShort}</AuthLabel>
        <AuthPassword id="pw-confirm" autoComplete="new-password"
          value={confirm} invalid={confirm.length > 0 && next !== confirm} onChange={(event) => setConfirm(event.currentTarget.value)} />
      </div>
      </div>
      {next.length > 0 && next.length < MIN_PASSWORD && <AuthError>{t.auth.passwordForm.short}</AuthError>}
      {confirm.length > 0 && next !== confirm && <AuthError id="pw-mismatch">{t.auth.passwordForm.mismatch}</AuthError>}
      {error && <AuthError id="pw-error">{error}</AuthError>}
      <Button type="submit" variant={horizontal ? 'ghost' : 'primary'} pending={busy}
        disabled={busy || !current || next.length < MIN_PASSWORD || next !== confirm}
        className={horizontal ? undefined : stylex.props(sx.submit).className}
        style={{ alignSelf: horizontal ? 'flex-start' : undefined, justifyContent: 'center', fontSize: horizontal ? 13 : mobile ? 15 : 14, fontWeight: horizontal ? 500 : 600 }}>
        {busy ? t.auth.passwordForm.changing : t.auth.passwordForm.submit}
      </Button>
    </form>
  )
}

/** Cổng ép đổi mật khẩu tạm — render thay shell, không có đường tắt nào khác. */
export function ChangePasswordScreen() {
  const { user, signOut } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const [error, setError] = useState('')
  const leave = async () => {
    try { await signOut() } catch { setError(t.account.logoutFailed) }
  }
  return (
    <AuthShell>
      <div {...stylex.props(sx.intro)}>
        <span aria-hidden="true" style={{
          width: 56, height: 56, borderRadius: '50%', display: 'grid', placeItems: 'center',
          background: 'var(--kit-warn-12)', color: 'var(--color-warn)',
        }}>
          <Icon name="key" size={26} />
        </span>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{t.auth.passwordForm.forceTitle}</h1>
        <p {...stylex.props(sx.body)}>{mobile ? t.auth.passwordForm.forceBodyMobile : t.auth.passwordForm.forceBody}</p>
      </div>
      {user && <AuthUser user={user} />}
      <div style={{ display: 'flex', flexDirection: 'column', gap: mobile ? 20 : 14 }}><ChangePasswordForm forced />
      <HintLine icon="info">{mobile ? t.auth.passwordForm.forceNoteMobile : t.auth.passwordForm.forceNote}</HintLine></div>
      {error && <FieldError>{error}</FieldError>}
      <Button variant="quiet" onClick={() => void leave()} style={{ alignSelf: 'center', height: 34 }}><Icon name="logout" size={15} />{t.account.logout}</Button>
    </AuthShell>
  )
}
