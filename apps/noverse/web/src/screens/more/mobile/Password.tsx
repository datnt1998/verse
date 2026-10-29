import { useId, useState } from 'react'
import { ApiError, changePassword, me } from '../../../api/client'
import { useSession } from '../../../api/session'
import { invalidate } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { MobilePage, useMobileToast } from '../../../mobile'
import { useTabMode } from '../../../mobile/runtime'
import { Icon } from '@noverse/ui'
import { passwordChecklist, passwordStrength } from './passwordLogic'
import './more.css'

export function MobilePassword({ first = false, onDone, onSkip }: { first?: boolean; onDone: () => void; onSkip?: () => void }) {
  const { user, setUser } = useSession()
  useTabMode('hide')
  const toast = useMobileToast()
  const id = useId()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  const [visible, setVisible] = useState<Record<string, boolean>>({})
  const [error, setError] = useState<unknown>(null)
  const [pending, setPending] = useState(false)
  const [changed, setChanged] = useState(false)
  const rules = passwordChecklist(current, next, repeat, first)
  const strength = passwordStrength(next)
  const currentError = error instanceof ApiError && error.status === 403
  const valid = !!current && rules.every((rule) => rule.ok) && navigator.onLine && !pending
  const submit = async () => {
    if (!valid) return
    setError(null); setPending(true)
    try {
      // Sau khi POST thành công không gửi lại mật khẩu nếu lần tải /me bị lỗi.
      if (!changed) { await changePassword(current, next); setChanged(true) }
      const result = await me()
      setUser(result.user, result.session)
      void invalidate(keys.sessions()); void invalidate(keys.users())
      setCurrent(''); setNext(''); setRepeat('')
      toast(first ? `Đã đặt mật khẩu · chào mừng ${result.user.name || result.user.username}` : 'Đã đổi mật khẩu · các thiết bị khác đã đăng xuất')
      onDone()
    } catch (failure) { setError(failure) } finally { setPending(false) }
  }
  const footer = <div className="mm-cta"><button className="mm-primary" type="submit" form={`${id}-form`} disabled={!valid} aria-busy={pending}>{pending ? 'Đang lưu…' : changed ? 'Tải lại tài khoản' : first ? 'Lưu & vào Noverse' : 'Đổi mật khẩu'}</button>{first && <p><button type="button" disabled={pending} onClick={onSkip}>Để sau</button> · sẽ nhắc lại mỗi lần mở cho tới khi đổi</p>}</div>
  return <MobilePage title={first ? '' : 'Đổi mật khẩu'} backTitle="Thêm" onBack={first ? undefined : onDone} footer={footer}>
    <form id={`${id}-form`} className="mm-password mm-root" onSubmit={(event) => { event.preventDefault(); void submit() }}>
      {first && <div className="mm-first"><span className="mm-key"><Icon name="key" size={26} /></span><h1>Chào {user?.name || user?.username}, đặt mật khẩu của bạn</h1><p>Quản trị viên đã tạo tài khoản cho bạn bằng mật khẩu tạm. Nên đặt mật khẩu riêng ngay — người biết mật khẩu tạm vẫn đăng nhập được cho tới khi bạn đổi.</p></div>}
      <div className="mm-password-fields">
        {([
          { key: 'current', label: first ? 'Mật khẩu tạm' : 'Mật khẩu hiện tại', value: current, change: setCurrent, auto: 'current-password' },
          { key: 'next', label: 'Mật khẩu mới', value: next, change: setNext, auto: 'new-password' },
          { key: 'repeat', label: 'Nhập lại mật khẩu mới', value: repeat, change: setRepeat, auto: 'new-password' },
        ]).map((field) => <div className="mm-field-block" key={field.key}>
          <label htmlFor={`${id}-${field.key}`}>{field.label}</label><div className="mm-pwf"><input id={`${id}-${field.key}`} className="kit-frame mm-field" type={visible[field.key] ? 'text' : 'password'} autoComplete={field.auto} value={field.value} disabled={pending || changed} aria-invalid={field.key === 'current' ? currentError : field.key === 'repeat' && !!repeat && repeat !== next} aria-describedby={field.key === 'current' && currentError ? `${id}-error` : undefined} onChange={(event) => { field.change(event.target.value); if (field.key === 'current') setError(null) }} /><button type="button" className="mm-eye" aria-label={`${visible[field.key] ? 'Ẩn' : 'Hiện'} ${field.label.toLowerCase()}`} onClick={() => setVisible((old) => ({ ...old, [field.key]: !old[field.key] }))}><Icon name={visible[field.key] ? 'eye-slash' : 'eye'} size={19} /></button></div>
          {field.key === 'current' && currentError && <span id={`${id}-error`} className="mm-field-error" role="alert">{error.message}</span>}
          {field.key === 'next' && <div className="mm-strength" aria-label={`Độ mạnh: ${strength.label}`}><div aria-hidden="true">{[0, 1, 2, 3].map((index) => <span key={index} style={{ background: index < strength.score ? strength.color : '#222225' }} />)}</div><span>{next.length >= 8 ? `Độ mạnh: ${strength.label}` : next.length ? `Còn thiếu ${8 - next.length} ký tự` : strength.label}</span></div>}
        </div>)}
        <ul className="mm-rules" aria-label="Yêu cầu mật khẩu" aria-live="polite">{rules.map((rule) => <li key={rule.label} data-met={rule.ok}><Icon name={rule.ok ? 'tick-circle' : 'minus'} size={15} />{rule.label}</li>)}</ul>
        <div className="mm-password-note"><Icon name="devices" size={17} /><span>Máy này vẫn đăng nhập. Các thiết bị khác của bạn sẽ phải đăng nhập lại.</span></div>
        {!navigator.onLine && <p className="mm-field-error" role="status">Cần kết nối mạng để đổi mật khẩu.</p>}
        {error != null && !currentError && <p className="mm-field-error" role="alert">{error instanceof Error ? error.message : 'Chưa đổi được mật khẩu. Vui lòng thử lại.'}{changed ? ' Mật khẩu đã lưu; thử lại để tải tài khoản.' : ''}</p>}
      </div>
    </form>
  </MobilePage>
}

export function MobileFirstPassword({ onDone, onSkip }: { onDone: () => void; onSkip: () => void }) {
  return <MobilePassword first onDone={onDone} onSkip={onSkip} />
}
