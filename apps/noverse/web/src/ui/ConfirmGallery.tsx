import { useState } from 'react'
import { ConfirmDialog } from '@noverse/ui'
import { CONFIRM_SITUATIONS, confirmationPreset, type ConfirmSituation } from './confirmPresets'
import { Button, Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import '@noverse/ui/kit.css'

// Màu theo TONE trong Kit.dc.html: ô icon 34 px + thẻ nhỏ bên phải mỗi tình huống.
type Visual = { icon: string; chip: string; ic: string; tag: string; tagBg: string; tagFg: string }
const NEUTRAL: Visual = { icon: 'pause', chip: 'rgba(201,166,245,0.12)', ic: '#C9A6F5', tag: 'Thường', tagBg: 'rgba(255,255,255,0.06)', tagFg: '#A0A0AB' }
const visuals: Record<ConfirmSituation, Visual> = {
  pause: NEUTRAL,
  overwrite: { icon: 'warning-2', chip: 'rgba(246,203,140,0.12)', ic: '#F6CB8C', tag: 'Cảnh báo', tagBg: 'rgba(246,203,140,0.12)', tagFg: '#F6CB8C' },
  device: { icon: 'devices', chip: 'rgba(255,95,110,0.1)', ic: '#FF9FA8', tag: 'Làm lại được', tagBg: 'rgba(255,95,110,0.1)', tagFg: '#FFB3BA' },
  user: { icon: 'profile-delete', chip: 'rgba(255,95,110,0.14)', ic: '#FF7A84', tag: 'Không hoàn tác', tagBg: 'rgba(255,95,110,0.16)', tagFg: '#FF9FA8' },
  story: { icon: 'trash', chip: 'rgba(255,95,110,0.14)', ic: '#FF7A84', tag: 'Gõ để xác nhận', tagBg: 'rgba(255,95,110,0.16)', tagFg: '#FF9FA8' },
  allDevices: { icon: 'logout', chip: 'rgba(255,95,110,0.1)', ic: '#FF9FA8', tag: 'Giữ để xác nhận', tagBg: 'rgba(255,95,110,0.1)', tagFg: '#FFB3BA' },
  leave: { ...NEUTRAL, icon: 'edit-2' },
}
// Lỗi mô phỏng khi chọn “Gặp lỗi” (C.err của từng tình huống).
const ERRORS: Record<ConfirmSituation, string> = {
  pause: 'Không tạm dừng được: máy chủ không phản hồi. Thử lại sau vài giây.',
  overwrite: 'Không ghi được tệp: thư mục exports đang chỉ cho đọc.',
  device: 'Không đăng xuất được: mất kết nối máy chủ.',
  user: 'Không xoá được: đây là tài khoản quản trị cuối cùng.',
  story: 'Không xoá được: truyện đang mở ở thiết bị khác.',
  allDevices: '',
  leave: 'Không lưu được: bản đọc đã bị sửa ở thiết bị khác. Tải lại để xem.',
}
const SKELETON = ['80%', '64%', '72%', '50%', '86%', '60%', '76%', '44%']
const RULES = [
  'Tiêu đề là câu hỏi có tên đối tượng: “Xoá tài khoản ‘linh’?”',
  'Thân bài nói hậu quả, không hỏi lại “Bạn có chắc không?”',
  'Nút ghi đúng động từ: “Xoá tài khoản”, không dùng “OK” / “Có”',
  'Huỷ bên trái, hành động bên phải. Mở hộp thì focus vào Huỷ khi hành động nguy hiểm',
  'Esc = huỷ. Enter chỉ chạy hành động khi không nguy hiểm',
  'Chỉ hỏi khi mất dữ liệu hoặc khó làm lại. Việc dễ hoàn tác: làm luôn, kèm “Hoàn tác”',
]

export function ConfirmGallery({ situation, onSituationChange }: { situation: ConfirmSituation; onSituationChange: (value: ConfirmSituation) => void }) {
  const [shown, setShown] = useState(true)
  const [revision, setRevision] = useState(0)
  const [outcome, setOutcome] = useState<'ok' | 'err'>('ok')
  const [done, setDone] = useState<string | null>(null)
  const preset = confirmationPreset(situation, { user: 'kit-demo', count: 3, chapter: 3 })
  const doneText = typeof preset.successMessage === 'function' ? preset.successMessage('after') : preset.successMessage
  const reopen = () => { setShown(true); setDone(null); setRevision((value) => value + 1) }
  const simulateAction = async () => {
    await new Promise<void>((resolve) => window.setTimeout(resolve, 900))
    if (outcome === 'err') throw new Error(ERRORS[situation])
  }
  const dangerousRule = situation === 'user' || situation === 'story' || situation === 'allDevices'
  return <div style={{ display: 'grid', gridTemplateColumns: '340px minmax(0, 1fr)', gap: 24, alignItems: 'start' }}>
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <span className="lbl" style={{ padding: '0 4px 4px' }}>Hộp xác nhận · 7 tình huống thật</span>
      {CONFIRM_SITUATIONS.map((item) => {
        const selected = item.id === situation
        const v = visuals[item.id]
        return <button key={item.id} className="kit-scard" type="button" aria-pressed={selected} onClick={() => { onSituationChange(item.id); reopen() }}
          style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 12, border: `1px solid ${selected ? 'rgba(255,255,255,0.12)' : 'rgba(255,255,255,0.05)'}`, background: selected ? '#18181B' : 'transparent', color: '#EDEDEF', textAlign: 'left' }}>
          <span style={{ width: 34, height: 34, borderRadius: 10, background: v.chip, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}><Icon name={v.icon} size={17} style={{ color: v.ic }} /></span>
          <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
            <span style={{ fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.label}</span>
            <span style={{ fontSize: 11.5, color: '#7E7E8A', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.location}</span>
          </span>
          <span style={{ height: 20, padding: '0 7px', borderRadius: 6, fontSize: 10.5, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap', background: v.tagBg, color: v.tagFg, flex: '0 0 auto' }}>{v.tag}</span>
        </button>
      })}
      <div className="card" style={{ marginTop: 8, borderRadius: 14, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <span style={{ fontSize: 13, fontWeight: 600 }}>Quy tắc chung</span>
        <ul style={{ margin: 0, padding: '0 0 0 16px', display: 'flex', flexDirection: 'column', gap: 5, fontSize: 12.5, color: '#B4B4BE', listStyle: 'disc' }}>{RULES.map((rule) => <li key={rule}>{rule}</li>)}</ul>
      </div>
    </div>
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <span style={{ fontSize: 12, color: '#7E7E8A' }}>Khi bấm nút chính:</span>
        <div role="radiogroup" aria-label="Kết quả thử" style={{ display: 'flex', gap: 3, padding: 3, borderRadius: 9, background: '#131315', border: '1px solid rgba(255,255,255,0.06)' }}>
          {([['ok', 'Thành công'], ['err', 'Gặp lỗi']] as const).map(([id, label]) => <button key={id} type="button" role="radio" aria-checked={outcome === id} onClick={() => setOutcome(id)}
            style={{ height: 26, padding: '0 10px', border: 'none', borderRadius: 6, background: outcome === id ? '#2A2A2E' : 'transparent', color: outcome === id ? '#EDEDEF' : '#A0A0AB', fontSize: 12, whiteSpace: 'nowrap' }}>{label}</button>)}
        </div>
        <span style={{ flex: 1 }} />
        <Button size="sm" icon={<Icon name="refresh-2" size={14} />} onClick={reopen}>Mở lại hộp thoại</Button>
      </div>
      <div style={{ position: 'relative', height: 600, borderRadius: 18, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.07)', background: '#0D0D0F' }}>
        <div aria-hidden="true" style={{ position: 'absolute', inset: 0, padding: 22, display: 'grid', gridTemplateColumns: '200px 1fr', gap: 18, opacity: 0.7 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>{SKELETON.map((width, i) => <span key={i} style={{ height: 12, borderRadius: 6, background: '#19191C', width }} />)}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <span style={{ height: 22, width: '60%', borderRadius: 7, background: '#1C1C1F' }} />
            {SKELETON.map((width, i) => <span key={i} style={{ height: 12, borderRadius: 6, background: '#17171A', width }} />)}
            <span style={{ flex: 1 }} />
            <span style={{ height: 50, borderRadius: 16, background: '#141416', border: '1px solid rgba(255,255,255,0.05)' }} />
          </div>
        </div>
        {shown && <ConfirmDialog key={`${situation}-${revision}`} {...preset} inline successMessage={undefined}
          secondary={situation === 'overwrite' ? { label: 'Giữ cả hai', onConfirm: simulateAction } : situation === 'leave' ? { label: 'Bỏ thay đổi', danger: true, onConfirm: simulateAction } : undefined}
          onConfirm={simulateAction}
          onClose={(accepted) => { setShown(false); if (accepted) setDone(doneText ?? '') }} />}
        {done !== null && <div className="kit-toast-in" role="status" style={{ position: 'absolute', left: '50%', bottom: 24, marginLeft: 0, height: 40, padding: '0 8px 0 14px', borderRadius: 999, background: '#1C1C1F', border: '1px solid rgba(166,237,193,0.28)', display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, whiteSpace: 'nowrap', boxShadow: '0 16px 40px -12px rgba(0,0,0,0.8)' }}>
          <Icon name="tick-circle" size={16} style={{ color: '#A6EDC1' }} />{done}
          <Button size="sm" style={{ height: 28 }} onClick={reopen}>{situation === 'pause' ? 'Hoàn tác' : 'Mở lại'}</Button>
        </div>}
      </div>
      <div style={{ display: 'flex', gap: 18, flexWrap: 'wrap', fontSize: 12, color: '#7E7E8A' }}>
        <span><Kbd>Esc</Kbd> huỷ</span><span><Kbd>Tab</Kbd> chuyển nút</span>
        <span>{dangerousRule ? 'Enter không tự xoá — phải bấm nút hoặc gõ / giữ để xác nhận' : 'Enter = nút chính'}</span>
      </div>
    </div>
  </div>
}
