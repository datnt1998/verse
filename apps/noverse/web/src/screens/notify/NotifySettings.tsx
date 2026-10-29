import { useEffect, useState } from 'react'
import type { NotifyPrefs } from '../../api/types'
import { ConflictError, putNotifyPrefs, testPush, unsubscribePush } from '../../api/client'
import { invalidate, setData, useAction } from '../../data'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { PermissionPanel } from './PermissionPanel'
import { Switch } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { NOTIFY_PREFS_KEY, PUSH_DEVICES_KEY, useNotifyPrefs, usePushDevices } from './useNotify'
import { useNotifyToast } from './useNotifyToast'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const TYPES = [
  ['gate', 'Chương chờ bạn duyệt', 'Có nút Duyệt nhanh ngay trên thông báo', 'flag-bulk', '#F6CB8C'],
  ['error', 'Truyện dừng do lỗi', 'Luôn báo, kể cả giờ yên lặng', 'danger', '#FF9FA8'],
  ['job', 'Việc nền xong', 'Xuất EPUB/sách nói, nhập truyện, đồng bộ', 'headphone-bulk', '#C9A6F5'],
  ['done', 'Chương đã xong', 'Gộp theo lựa chọn bên dưới', 'book-1', '#A6EDC1'],
  ['security', 'Đăng nhập & thiết bị mới', 'Bắt buộc để giữ tài khoản an toàn', 'shield-security-bulk', '#9DB0F7'],
] as const

const shortDate = (iso: string) => { const date = new Date(iso); return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}` }

function Toggle({ label, checked, locked, forced, onChange }: { label: string; checked: boolean; locked?: boolean; forced?: boolean; onChange: () => void }) {
  return <Switch checked={checked} onChange={onChange} label={label} size={[51, 31]}
    disabled={locked} aria-disabled={forced || undefined} onClick={(event) => { if (forced) event.preventDefault() }}
    style={{ width: 55, margin: '-6.5px -2px', opacity: 1 }}
    trackStyle={locked ? { background: 'color-mix(in srgb, var(--kit-switch-on) 45%, transparent)' } : undefined}
    thumbStyle={{ background: 'var(--kit-white)', boxShadow: 'var(--kit-shadow-mobile-thumb)' }} />
}

export function NotifySettings({ desktop = false, onInbox }: { desktop?: boolean; onInbox?: () => void }) {
  const prefs = useNotifyPrefs()
  const devices = usePushDevices()
  const notify = useNotifyToast()
  const [permission, setPermission] = useState<'prompt' | 'denied' | null>(null)
  const [endpoint, setEndpoint] = useState<string | null>(null)
  const save = useAction(async (next: NotifyPrefs) => {
    try {
      const saved = await putNotifyPrefs(next, next.version)
      setData(NOTIFY_PREFS_KEY, saved)
      return saved
    } catch (failure) {
      if (failure instanceof ConflictError) await prefs.refresh()
      throw failure
    }
  }, { success: desktop ? 'Đã lưu cài đặt thông báo' : undefined })
  const test = useAction(testPush)
  const remove = useAction(async (target: string) => {
    await unsubscribePush(target)
    if (target === endpoint && 'serviceWorker' in navigator) {
      const registration = await navigator.serviceWorker.getRegistration()
      const subscription = await registration?.pushManager.getSubscription()
      await subscription?.unsubscribe()
      setEndpoint(null)
    }
  }, { success: 'Đã ngừng gửi thông báo tới thiết bị', invalidate: [PUSH_DEVICES_KEY] })
  useEffect(() => {
    if ('serviceWorker' in navigator) void navigator.serviceWorker.getRegistration().then((registration) => registration?.pushManager?.getSubscription()).then((subscription) => setEndpoint(subscription?.endpoint ?? null)).catch(() => {})
  }, [devices.updatedAt])
  const data = prefs.data
  const update = (patch: Partial<NotifyPrefs>) => { if (data && !save.busy) void save.run({ ...data, ...patch }) }
  if (permission) return <PermissionPanel denied={permission === 'denied'} onDenied={() => setPermission('denied')} onLater={() => setPermission(null)} onDone={() => { setPermission(null); invalidate(NOTIFY_PREFS_KEY); invalidate(PUSH_DEVICES_KEY); notify('Đã bật thông báo trên máy này') }} />
  if (!data) return <div className="notify-initial">{prefs.error ? <><p role="alert">{prefs.error instanceof Error ? prefs.error.message : 'Không tải được cài đặt thông báo.'}</p><button onClick={() => void prefs.refresh()}>Thử lại</button></> : <><Skeleton height={70} /><Skeleton height={290} /><Skeleton height={80} /></>}</div>
  const busy = save.busy
  const quiet = !!data.quiet.from && !!data.quiet.to
  const deviceList = devices.data?.subscriptions ?? []
  const subscribedHere = deviceList.some((device) => device.endpoint === endpoint)
  return <div className={`notify-settings${desktop ? ' notify-desktop-settings' : ''}`}><PanelRefreshBar resourceKeys={[NOTIFY_PREFS_KEY, ['push-subscriptions']]} updatedAt={prefs.updatedAt} style={{ minHeight: 0 }} /><StaleBanner error={prefs.error} onRetry={prefs.refresh} />{!!save.error && <div className="notify-warning" role="alert">{save.error instanceof Error ? save.error.message : 'Không lưu được cài đặt thông báo.'}{save.error instanceof ConflictError && ' Cài đặt mới đã được tải. Chọn lại thay đổi của bạn.'}</div>}{desktop && <p className="notify-scope-note">Cài đặt riêng cho tài khoản của bạn, áp dụng trên mọi truyện và thiết bị. Thay đổi được lưu ngay.</p>}
    <div className="notify-group notify-group-first"><div className="notify-setting-row"><Icon name="notification" size={20} style={{ color: '#C9A6F5' }} /><span className="notify-setting-copy"><span>Thông báo đẩy</span><small style={{ color: data.enabled ? undefined : '#F6CB8C' }}>{data.enabled ? `Đang bật trên ${deviceList.length} thiết bị` : 'Đang tắt — chỉ hiện trong hộp thư'}</small></span><Toggle label="Thông báo đẩy" checked={data.enabled} onChange={() => { if (busy) return; if (data.enabled) update({ enabled: false }); else if (!subscribedHere || !('Notification' in window) || Notification.permission !== 'granted') setPermission('Notification' in window && Notification.permission === 'denied' ? 'denied' : 'prompt'); else update({ enabled: true }) }} /></div></div>
    <h3>BÁO KHI…</h3><div className="notify-group" style={{ opacity: data.enabled ? 1 : .45 }}>{TYPES.map(([key, label, note, icon, color]) => <div className="notify-setting-row notify-type-row" key={key}><span className="notify-type-icon"><Icon name={icon} size={16} style={{ color }} /></span><span className="notify-setting-copy"><span>{label}</span><small>{note}</small></span><Toggle label={label} checked={key === 'security' || key === 'error' || data.types[key]} locked={key === 'security'} forced={key === 'error'} onChange={() => update({ types: { ...data.types, [key]: !data.types[key] } })} /></div>)}</div>
    <h3>CHƯƠNG ĐÃ XONG</h3><div className="notify-digest-wrap"><Segmented label="Gộp thông báo chương xong" size="lg" radius={9} columns={3} value={data.digest} onChange={(digest) => update({ digest: digest as NotifyPrefs['digest'], types: { ...data.types, done: true } })} options={[{ id: 'each', label: 'Mỗi chương' }, { id: 'five', label: 'Mỗi 5 chương' }, { id: 'day', label: 'Cuối ngày' }].map((option) => ({ ...option, style: { fontSize: 13.5, padding: 0 } }))} style={{ border: 'none', gap: 3, borderRadius: 12 }} /><span className="notify-digest-note">{data.digest === 'each' ? 'Có thể tới 10 thông báo mỗi giờ khi viết liên tục.' : data.digest === 'five' ? 'Một thông báo tóm tắt cho mỗi 5 chương xong.' : 'Một bản tin lúc 21:00 tóm tắt cả ngày.'}</span></div>
    <h3>GIỜ YÊN LẶNG</h3><div className="notify-group"><div className="notify-setting-row"><span className="notify-setting-copy"><span>Không làm phiền</span><small className="notify-num">{quiet ? data.quiet.from : '22:00'} – {quiet ? data.quiet.to : '07:00'} · trừ lỗi làm dừng truyện</small></span><Toggle label="Giờ yên lặng" checked={quiet} onChange={() => update({ quiet: quiet ? { from: '', to: '' } : { from: '22:00', to: '07:00' } })} /></div>{desktop && quiet && <div className="notify-quiet-fields"><label>Từ <input type="time" aria-label="Bắt đầu giờ yên lặng" value={data.quiet.from} disabled={busy} onChange={(event) => { if (event.target.value) update({ quiet: { ...data.quiet, from: event.target.value } }) }} /></label><span>–</span><label>Đến <input type="time" aria-label="Kết thúc giờ yên lặng" value={data.quiet.to} disabled={busy} onChange={(event) => { if (event.target.value) update({ quiet: { ...data.quiet, to: event.target.value } }) }} /></label></div>}</div>
    <h3>THIẾT BỊ NHẬN THÔNG BÁO</h3><StaleBanner error={devices.error} onRetry={devices.refresh} /><div className="notify-group">{!devices.data && !devices.error && <div className="notify-setting-row"><Skeleton height={40} /></div>}{devices.data && !deviceList.length && <div className="notify-setting-row"><small>Chưa có thiết bị nhận thông báo. Bật trên máy này để bắt đầu.</small></div>}{deviceList.map((device) => <div className="notify-setting-row" key={device.endpoint}><Icon name={/iPhone|iPad|Android|điện thoại/i.test(device.device) ? 'mobile' : 'monitor'} size={20} style={{ color: '#A0A0AB' }} /><span className="notify-setting-copy"><span>{device.device}</span><small>Bật từ {shortDate(device.createdAt)}</small></span>{device.endpoint === endpoint && <span className="notify-this-device">Máy này</span>}{desktop && <button className="notify-device-remove" aria-label={`Ngừng gửi tới ${device.device}`} disabled={remove.busy} onClick={() => void remove.run(device.endpoint)}><Icon name="close-circle" size={18} /></button>}</div>)}<button className="notify-test notify-press" disabled={test.busy || !data.enabled || !deviceList.length} onClick={() => void test.run().then((sent) => { if (sent) notify('Đã gửi — kiểm tra màn hình khoá') })}>{test.pending ? 'Đang gửi thử…' : 'Gửi thử một thông báo'}</button>{desktop && !subscribedHere && <button className="notify-test notify-press" onClick={() => setPermission('Notification' in window && Notification.permission === 'denied' ? 'denied' : 'prompt')}>Bật thông báo trên máy này</button>}</div>{desktop && <a className="notify-inbox-link" href="#/notifications" onClick={onInbox}>Mở hộp thư thông báo <Icon name="arrow-right-2" size={14} /></a>}
  </div>
}
