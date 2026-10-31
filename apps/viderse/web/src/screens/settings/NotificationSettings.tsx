import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { invalidate } from '@noverse/ui/data'
import { getNotificationSettings, saveNotificationSettings, type NotificationSettings } from '../../api/activity'
import { useResource } from '../../data/useResource'
import { listChannels } from '../../api/channels'
import { keys } from '../../data/keys'
import { useOnline } from '../../data/status'
import { SuggestionSettingsRow } from '../../components/learn/SuggestionSettingsRow'

const s = stylex.create({
  card: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', borderRadius: 'var(--vv-r-16)', backgroundColor: 'var(--color-surface-2)', overflow: 'hidden' },
  embedded: { borderWidth: 0, borderRadius: 'initial', backgroundColor: 'transparent' },
  grid: { display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 72px 72px', gap: 'var(--vv-s-8)', alignItems: 'center', padding: 'var(--vv-s-10) var(--vv-s-16)', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)' },
  header: { fontSize: 11, fontWeight: 600, color: 'var(--color-fg-3)', textTransform: 'uppercase', letterSpacing: '.06em' },
  centered: { textAlign: 'center' },
  row: { display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-12)', padding: 'var(--vv-s-12) var(--vv-s-16)', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)' },
  text: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-2)', flex: 1, minWidth: 0, fontSize: 13 },
  desc: { color: 'var(--color-fg-3)', fontSize: 12, lineHeight: 1.5 },
  sw: { width: 34, height: 20, border: 0, padding: 'var(--vv-s-2)', borderRadius: 'var(--vv-r-pill)', backgroundColor: 'var(--color-surface-6)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' },
  on: { backgroundColor: 'var(--color-accent)', justifyContent: 'flex-end' },
  knob: { width: 16, height: 16, borderRadius: '50%', backgroundColor: 'var(--color-fg)' },
  time: { color: 'var(--color-fg)', backgroundColor: 'var(--color-surface-1)', border: '1px solid var(--line-2)', borderRadius: 'var(--vv-r-8)', padding: 'var(--vv-s-6)', marginRight: 'var(--vv-s-6)' },
  error: { color: 'var(--color-err)', padding: 'var(--vv-s-16)' },
})
const rows = [
  ['review_ready', 'Chờ duyệt', 'Hoạ sĩ vẽ xong, chờ bạn duyệt ảnh'],
  ['needs_attention', 'Cần xử lý', 'Video dừng và cần bạn làm một việc'],
  ['ready_to_post', 'Sẵn sàng', 'MP4 đã xuất, có thể tải gói đăng'],
  ['task_done', 'Việc nền khác xong', 'Nhận diện, vẽ thử, bản xem thử, xuất từ editor'],
] as const
function Switch({ label, checked, onChange }: { label: string; checked: boolean; onChange: (next: boolean) => void }) {
  return <button type="button" role="switch" aria-label={label} aria-checked={checked} {...stylex.props(s.sw, checked && s.on)} onClick={() => onChange(!checked)}><span {...stylex.props(s.knob)} /></button>
}
export default function NotificationSettings({ embedded = false }: { embedded?: boolean }) {
  const resource = useResource(['activity', 'settings'], ({ signal }) => getNotificationSettings(signal))
  const [error, setError] = useState('')
  const [permission, setPermission] = useState<NotificationPermission | 'unsupported'>(() => 'Notification' in window ? Notification.permission : 'unsupported')
  const online = useOnline()
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const channelNames = Object.fromEntries((channels.data?.channels ?? []).map((channel) => [channel.id, channel.name]))
  const settings = resource.data
  const save = async (next: NotificationSettings) => {
    try { await saveNotificationSettings(next); setError(''); invalidate(['activity', 'settings']) }
    catch (e) { setError(e instanceof Error ? e.message : 'Không lưu được cài đặt.') }
  }
  if (!settings) return <div role={resource.error ? 'alert' : 'status'}>{resource.error ? 'Không tải được cài đặt thông báo.' : 'Đang tải…'}{resource.error ? <button onClick={() => void resource.refresh()}>Thử lại</button> : null}</div>
  const toggleSystem = async (kind: keyof NotificationSettings['kinds'], checked: boolean) => {
    if (checked && 'Notification' in window && Notification.permission === 'default') setPermission(await Notification.requestPermission())
    await save({ ...settings, kinds: { ...settings.kinds, [kind]: { ...settings.kinds[kind], system: checked } } })
  }
  return <section {...stylex.props(s.card, embedded && s.embedded)} aria-label="Cài đặt thông báo"><div {...stylex.props(s.grid, s.header)}><span>Báo khi</span><span {...stylex.props(s.centered)}>Trong app</span><span {...stylex.props(s.centered)}>Hệ thống</span></div>{rows.map(([kind, label, desc]) => <div {...stylex.props(s.grid)} key={kind}><span {...stylex.props(s.text)}>{label}<span {...stylex.props(s.desc)}>{desc}</span></span><span {...stylex.props(s.centered)}><Switch label={`${label} trong app`} checked={settings.kinds[kind].inApp} onChange={(checked) => void save({ ...settings, kinds: { ...settings.kinds, [kind]: { ...settings.kinds[kind], inApp: checked } } })} /></span><span {...stylex.props(s.centered)}><Switch label={`${label} hệ thống`} checked={settings.kinds[kind].system} onChange={(checked) => void toggleSystem(kind, checked)} /></span></div>)}
  <div {...stylex.props(s.row)}><span {...stylex.props(s.text)}>Gộp thông báo trong 10 giây<span {...stylex.props(s.desc)}>Nhiều video xong gần nhau thành một thông báo, ví dụ “3 video chờ duyệt”.</span></span><Switch label="Gộp thông báo trong 10 giây" checked={settings.merge} onChange={(merge) => void save({ ...settings, merge })} /></div>
  <div {...stylex.props(s.row)}><span {...stylex.props(s.text)}>Không làm phiền<span {...stylex.props(s.desc)}>Không bật toast hay thông báo hệ thống; mọi thứ vẫn ghi vào lịch sử. Cần xử lý vẫn hiện trong app.</span><span><input {...stylex.props(s.time)} type="time" aria-label="Không làm phiền từ" value={settings.doNotDisturb.start} onChange={(e) => void save({ ...settings, doNotDisturb: { ...settings.doNotDisturb, start: e.target.value } })} />đến <input {...stylex.props(s.time)} type="time" aria-label="Không làm phiền đến" value={settings.doNotDisturb.end} onChange={(e) => void save({ ...settings, doNotDisturb: { ...settings.doNotDisturb, end: e.target.value } })} /></span></span><Switch label="Không làm phiền" checked={settings.doNotDisturb.enabled} onChange={(enabled) => void save({ ...settings, doNotDisturb: { ...settings.doNotDisturb, enabled } })} /></div>
  <div {...stylex.props(s.row)}><span {...stylex.props(s.text)}>Số trên biểu tượng app<span {...stylex.props(s.desc)}>Hiện số video Chờ bạn (chờ duyệt + cần xử lý) trên Dock.</span></span><Switch label="Số trên biểu tượng app" checked={settings.badge} onChange={(badge) => void save({ ...settings, badge })} /></div>
  <div {...stylex.props(s.row, s.header)}>Trợ lý chủ động</div><div {...stylex.props(s.row)}><span {...stylex.props(s.text)}>Bản tin khi bạn quay lại<span {...stylex.props(s.desc)}>Mở ở Bàn dựng khi bạn quay lại sau 3 giờ trở lên hoặc lần đầu trong ngày, và có điều mới. Không bao giờ hiện toast.</span></span><Switch label="Bản tin khi bạn quay lại" checked={settings.recapOnReturn} onChange={(recapOnReturn) => void save({ ...settings, recapOnReturn })} /></div>
  <SuggestionSettingsRow off={settings.suggestions.off} channelNames={channelNames} offline={!online} renderSwitch={(label, checked, onChange) => <Switch label={label} checked={checked} onChange={onChange} />} onChange={(off) => void save({ ...settings, suggestions: { off } })} />
  {permission === 'default' && <div {...stylex.props(s.row)}><button type="button" onClick={() => void Notification.requestPermission().then(setPermission)}>Cho phép thông báo hệ thống</button></div>}
  {permission === 'denied' && <div {...stylex.props(s.row, s.desc)}>Thông báo hệ thống đang bị trình duyệt chặn.</div>}
  <div {...stylex.props(s.row, s.desc)}>Tự lưu · áp dụng ngay</div>{error && <div role="alert" {...stylex.props(s.error)}>{error}</div>}</section>
}
