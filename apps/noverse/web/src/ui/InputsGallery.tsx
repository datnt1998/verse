// Gallery tab “Ô nhập & nút” — Kit.dc.html › sec.inputs (số liệu, nhãn và thứ tự giữ nguyên artboard).
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { TextField, NumberStepper, Slider } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { Switch, Checkbox, RadioCard } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { toast } from '@noverse/ui'
import '@noverse/ui/kit.css'

const cardStyle: CSSProperties = { borderRadius: 16, padding: 18, display: 'flex', flexDirection: 'column' }
const Card = ({ title, gap, wide, children }: { title: string; gap: number; wide?: boolean; children: ReactNode }) =>
  <section className="card" style={{ ...cardStyle, gap, ...(wide ? { gridColumn: '1 / -1' } : {}) }}><span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span>{children}</section>
const Lbl = ({ children }: { children: ReactNode }) => <span className="lbl" style={{ fontSize: 10.5 }}>{children}</span>

const RADIOS = [['ask', 'Hỏi tôi', 'Mặc định — hiện hộp xác nhận'], ['replace', 'Thay bản cũ', 'Như --overwrite'], ['keep', 'Giữ cả hai', 'Thêm giờ vào tên tệp mới']] as const

export function InputsGallery() {
  const [f1, setF1] = useState('')
  const [f2, setF2] = useState('Trà Xanh')
  const [f3, setF3] = useState('Trà Xanh Cướp Kịch Bản')
  const [f4, setF4] = useState('Tra Xanh!')
  const [ta, setTa] = useState('')
  const [step, setStep] = useState(1)
  const [speed, setSpeed] = useState(1.05)
  const [checkA, setCheckA] = useState(true)
  const [checkB, setCheckB] = useState(false)
  const [radio, setRadio] = useState<string>('ask')
  const [swA, setSwA] = useState(true)
  const [swB, setSwB] = useState(false)
  const [swLoading, setSwLoading] = useState(false)
  const [seg, setSeg] = useState('quick')
  const timer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(timer.current), [])
  // Artboard: bật “Bộ đọc trên máy” mất ~0,9 s; trong lúc đó nút tròn là spinner, rãnh giữ màu cũ.
  const toggleReader = (on: boolean) => {
    if (!on) { setSwB(false); return }
    setSwLoading(true)
    timer.current = window.setTimeout(() => { setSwLoading(false); setSwB(true) }, 900)
  }
  return <div className="kit-gallery-grid" style={{ gap: 20, alignItems: 'start' }}>
    <Card title="Ô chữ" gap={12}>
      <TextField label="Mặc định" className="mono" placeholder="ten-thu-muc" value={f1} onChange={setF1}
        validate={(v) => /[^a-z0-9-]/.test(v) ? 'Chỉ dùng chữ thường không dấu, số và dấu gạch ngang' : undefined} hint="Gõ thử chữ có dấu để thấy lỗi" />
      <TextField label="Đang chọn" focused value={f2} onChange={setF2} />
      <TextField label="Có nút xoá" clearable value={f3} onChange={setF3} />
      <TextField label="Lỗi" value={f4} onChange={setF4} error="Tên thư mục không được có dấu cách hoặc ký tự đặc biệt" />
      <TextField label="Có tiền tố & hậu tố" defaultValue="tra-xanh"
        prefix={<><Icon name="folder-open" size={15} style={{ color: '#7E7E8A' }} />exports/</>} suffix=".epub" />
      <TextField label="Không sửa được" disabled value="Tài liệu › Truyện Noverse" hint="Đổi ở Tuỳ chỉnh › Nâng cao › Nơi lưu truyện" />
    </Card>

    <Card title="Vùng chữ · số · thanh trượt" gap={16}>
      <TextArea label="Góp ý cho Người viết" rows={4} maxLength={500} value={ta} onChange={(event) => setTa(event.target.value)}
        onSubmitValue={() => toast('Đã chọn thao tác', 'ok')} placeholder="Ví dụ: chương sau cho Tri Hạ xuất hiện lại, giữ giọng châm biếm nhẹ" />
      <NumberStepper label="Từ chương" min={1} max={24} value={step} onChange={setStep} hint="Từ 1 đến 24 (chương đã xong). Nút mờ khi chạm giới hạn." />
      <Slider label="Tốc độ đọc" min={.8} max={1.5} step={.05} value={speed} onChange={setSpeed}
        formatValue={(v) => `${v.toFixed(2).replace(/0$/, '')}×`} ticks={[{ value: .8, label: '0.8×' }, { value: 1, label: '1.0×' }, { value: 1.25, label: '1.25×' }, { value: 1.5, label: '1.5×' }]} />
    </Card>

    <Card title="Chọn & bật tắt" gap={16}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <Lbl>Hộp kiểm</Lbl>
        <Checkbox checked="mixed" onChange={() => undefined} label="Chọn tất cả chương (8/24)" />
        <Checkbox checked={checkA} onChange={setCheckA} label="Báo khi xong việc" />
        <Checkbox checked={checkB} onChange={setCheckB} label="Gửi chẩn đoán ẩn danh" />
        <Checkbox checked={false} disabled onChange={() => undefined} label="Tự xuất khi xong tập (cần Quản trị)" />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <Lbl>Khi tệp xuất đã có</Lbl>
        <div role="radiogroup" aria-label="Khi tệp xuất đã có" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {RADIOS.map(([id, label, note]) => <RadioCard key={id} checked={radio === id} onChange={() => setRadio(id)} label={label} description={note} />)}
        </div>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <Lbl>Công tắc</Lbl>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}><span style={{ flex: 1, fontSize: 13 }}>Duyệt từng chương</span><Switch label="Duyệt từng chương" checked={swA} onChange={setSwA} /></div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}><span style={{ flex: 1, fontSize: 13 }}>Bộ đọc trên máy</span><Switch label="Bộ đọc trên máy" checked={swB} pending={swLoading} onChange={toggleReader} /></div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}><span style={{ flex: 1, fontSize: 13, color: '#5E5E68' }}>Đồng bộ đám mây (sắp có)</span><Switch label="Đồng bộ đám mây (sắp có)" checked={false} disabled onChange={() => undefined} /></div>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <Lbl>Nhóm chọn</Lbl>
        <Segmented label="Nhóm chọn" value={seg} onChange={setSeg} options={[
          { id: 'quick', label: 'Nhanh', icon: <Icon name="flash" size={14} /> },
          { id: 'co', label: 'Đồng sáng tác', icon: <Icon name="messages-2" size={14} /> },
        ]} />
      </div>
    </Card>

    <Card title="Nút" gap={14} wide>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, alignItems: 'center' }}>
        <Button variant="primary" onClick={() => toast('Đã chọn thao tác', 'ok')}>Lưu thay đổi</Button>
        <Button>Huỷ</Button>
        <Button variant="dsoft" icon={<Icon name="logout" size={15} />}>Đăng xuất thiết bị</Button>
        <Button variant="danger" icon={<Icon name="trash" size={15} />}>Xoá vĩnh viễn</Button>
        <Button kbd="Esc">Tạm dừng</Button>
        <Button variant="primary" pending pendingLabel="Đang lưu…">Đang lưu…</Button>
        <Button variant="primary" disabled>Không bấm được</Button>
        <Button size="sm" icon={<Icon name="copy" size={13} />}>Nhỏ</Button>
        <Button variant="icon" aria-label="Cài đặt" tooltipKbd="⌘ ," icon={<Icon name="setting-2" />} />
      </div>
      <span style={{ fontSize: 12, color: '#7E7E8A' }}>Một màn hình chỉ có 1 nút chính (sáng). Đỏ đặc chỉ dùng cho xoá vĩnh viễn; đỏ nhạt cho việc nguy hiểm nhưng làm lại được (đăng xuất, bỏ thay đổi). Nút chỉ có icon luôn có tooltip và <span className="mono">aria-label</span>.</span>
    </Card>
  </div>
}
