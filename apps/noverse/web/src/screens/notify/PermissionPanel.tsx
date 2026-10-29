import { useState } from 'react'
import { getNotifyPrefs, getPushKey, putNotifyPrefs, subscribePush } from '../../api/client'
import { invalidate } from '../../data'
import { Icon } from '@noverse/ui'
import { currentDeviceEnv, needsHomeScreenInstall, pushUnavailable, vapidBytes } from './logic'
import { NOTIFY_PREFS_KEY, PUSH_DEVICES_KEY } from './useNotify'
import { useNotifyToast } from './useNotifyToast'

export async function enableDevicePush() {
  const unavailable = pushUnavailable()
  if (unavailable) throw new Error(unavailable)
  // Quyền được hỏi ngay trong cử chỉ chạm; không đợi request mạng làm mất user activation.
  const permission = Notification.permission === 'granted' ? 'granted' : await Notification.requestPermission()
  if (permission !== 'granted') return permission
  const registration = await navigator.serviceWorker.register('/sw.js')
  await navigator.serviceWorker.ready
  let subscription = await registration.pushManager.getSubscription()
  if (!subscription) {
    const { publicKey } = await getPushKey()
    subscription = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: vapidBytes(publicKey) })
  }
  const json = subscription.toJSON()
  if (!json.endpoint || !json.keys?.p256dh || !json.keys.auth) throw new Error('Trình duyệt không trả đủ khoá đăng ký thông báo. Vui lòng thử lại.')
  await subscribePush({ endpoint: json.endpoint, keys: { p256dh: json.keys.p256dh, auth: json.keys.auth }, device: /iPhone|iPad/.test(navigator.userAgent) ? 'iPhone / iPad' : /Android/.test(navigator.userAgent) ? 'Điện thoại Android' : /Mac/.test(navigator.userAgent) ? 'Mac · trình duyệt web' : 'Máy tính · trình duyệt web' })
  const prefs = await getNotifyPrefs()
  if (!prefs.enabled) await putNotifyPrefs({ ...prefs, enabled: true }, prefs.version)
  invalidate(NOTIFY_PREFS_KEY)
  invalidate(PUSH_DEVICES_KEY)
  return permission
}

export function LockIllustration() {
  return <div className="notify-illustration" aria-hidden="true"><div className="notify-phone" /><strong className="notify-clock">14:32</strong><div className="notify-peek"><div className="notify-ln"><span className="notify-app-icon"><Icon name="flag-bulk" size={18} /></span><span className="notify-ln-copy"><span>Chương 25 chờ bạn duyệt</span><span>Trà Xanh Cướp Kịch Bản · 8,4 điểm</span></span></div></div><div className="notify-peek-second"><div className="notify-ln"><span className="notify-app-icon"><Icon name="headphone-bulk" size={15} /></span><span>Đã xuất sách nói Tập 1</span></div></div></div>
}

const PROMISES = [
  ['flag-bulk', '#F6CB8C', 'Chương chờ bạn duyệt — duyệt ngay từ thông báo'],
  ['danger', '#FF9FA8', 'Truyện dừng vì lỗi, kèm cách xử lý'],
  ['headphone-bulk', '#C9A6F5', 'Xuất EPUB, sách nói, nhập truyện đã xong'],
] as const
const FIX_STEPS = ['Mở Cài đặt → Thông báo', 'Chọn Noverse trong danh sách', 'Bật “Cho phép thông báo”'] as const

/** `forceIosHint`: màn xem trước (`?view=prompt|system`) luôn hiện lưu ý như artboard; ngoài ra chỉ iPhone/iPad chưa Thêm vào Màn hình chính. */
export function PermissionPanel({ denied = false, onDone, onLater, onDenied, system = false, forceIosHint = false }: { denied?: boolean; onDone: () => void; onLater: () => void; onDenied: () => void; system?: boolean; forceIosHint?: boolean }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const notify = useNotifyToast()
  const needsInstall = needsHomeScreenInstall(currentDeviceEnv())
  const iosHint = !denied && (needsInstall || forceIosHint)
  // Trên iPhone chưa cài, lưu ý “Thêm vào Màn hình chính” đã nói đủ lý do; không chồng thêm cảnh báo kỹ thuật.
  const unavailable = needsInstall ? null : pushUnavailable()
  const disabled = busy || !!pushUnavailable()
  const enable = async (recheck = false) => {
    if (busy) return
    setBusy(true); setError('')
    try {
      if (recheck && (!('Notification' in window) || Notification.permission !== 'granted')) { notify('Vẫn bị chặn — kiểm tra lại các bước trên'); return }
      const permission = await enableDevicePush()
      if (permission === 'granted') onDone()
      else if (permission === 'denied') onDenied()
      else setError('Bạn chưa cấp quyền. Có thể bật lại khi sẵn sàng.')
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'Không bật được thông báo. Vui lòng thử lại.') }
    finally { setBusy(false) }
  }
  return <div className="notify-permission"><div className="notify-permission-body"><LockIllustration />
    {denied ? <>
      <h1 className="is-denied">Thông báo đang bị tắt</h1>
      <p>Trình duyệt đã chặn Noverse gửi thông báo. Noverse không thể tự bật lại — bạn mở trong Cài đặt của máy:</p>
      <ol className="notify-group notify-steps">{FIX_STEPS.map((text, index) => <li key={text} className="notify-setting-row"><span className="notify-step">{index + 1}</span><span>{text}</span></li>)}</ol>
      <small>Trong lúc đó, mọi thông báo vẫn nằm trong hộp thư (biểu tượng chuông ở màn Viết).</small>
    </> : <>
      <h1>Biết ngay khi truyện cần bạn</h1>
      <p>Noverse chỉ báo những lúc đáng báo. Bạn chọn loại nào được báo trong Cài đặt.</p>
      <div className="notify-group notify-promises">{PROMISES.map(([icon, color, text]) => <div className="notify-setting-row" key={icon}><Icon name={icon} size={20} style={{ color }} /><span>{text}</span></div>)}</div>
      {iosHint && <div className="notify-ios"><Icon name="info-circle" size={16} style={{ marginTop: 2 }} /><span>Trên iPhone, thông báo chỉ hoạt động khi Noverse đã được <b>Thêm vào Màn hình chính</b> (Chia sẻ → Thêm vào MH chính).</span></div>}
    </>}
    {unavailable && <div className="notify-warning" role="note">{unavailable}</div>}
    {error && <div className="notify-warning" role="alert">{error}</div>}
  </div>
  <div className="notify-permission-actions"><button className="notify-primary notify-tap" disabled={disabled} onClick={() => void enable(denied)}><Icon name={denied ? 'refresh-2' : 'notification'} size={18} />{busy ? 'Đang bật thông báo…' : denied ? 'Tôi đã bật, kiểm tra lại' : 'Bật thông báo'}</button><button className="notify-later notify-tap" disabled={busy} onClick={onLater}>{denied ? 'Mở hộp thư' : 'Để sau'}</button></div>
  {system && <div className="notify-system-overlay"><div className="notify-system-scrim" aria-hidden="true" /><div className="notify-system-dialog" role="alertdialog" aria-labelledby="notify-sa-title"><div><span id="notify-sa-title">“Noverse” muốn gửi thông báo cho bạn</span><span>Thông báo có thể gồm cảnh báo, âm thanh và biểu tượng.</span></div><button onClick={onDenied}>Không cho phép</button><button disabled={busy} onClick={() => void enable()}>Cho phép</button></div><span className="notify-system-note">Hộp thoại của hệ điều hành — ngoài tầm thiết kế</span></div>}
  </div>
}
