// Gallery tab “Gợi ý & trạng thái” — Kit.dc.html › sec.overlays (+ khung thử Toast chung, chỉ có ở bản dev).
import type { CSSProperties, ReactNode } from 'react'
import { Banner } from '@noverse/ui'
import { Badge, Chip } from '@noverse/ui'
import { Button, Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { InfoPopover, Tooltip } from '@noverse/ui'
import { Skeleton, Spinner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { paths } from '../router'
import '@noverse/ui/kit.css'

const TIPS = [
  { icon: 'pause', label: 'Tạm dừng', kbd: 'Esc', side: 'top' }, { icon: 'command', label: 'Bảng lệnh', kbd: '⌘ K', side: 'top' },
  { icon: 'activity', label: 'Việc chạy nền', kbd: '⌥ J', side: 'bottom' }, { icon: 'copy', label: 'Chép đường dẫn', kbd: '', side: 'bottom' },
] as const
const BADGES = [['running', 'Đang chạy'], ['ok', 'Xong'], ['warn', 'Cần bạn'], ['err', 'Lỗi'], ['neutral', 'Nháp'], ['accent', 'Quản trị']] as const
const CHIPS = [['flag', 'Giai đoạn', 'Sáng tác'], ['layer', 'Tập', '1/3'], ['edit-2', 'Chế độ', 'Tự động']] as const
const BANNERS = [
  { tone: 'info', title: 'Có bản cập nhật cho bộ đọc trên máy', text: 'VieNeu-TTS 1.3 đọc tên riêng tốt hơn.', act: 'Cập nhật' },
  { tone: 'success', title: 'Đã kết nối ChatGPT', text: 'Các trợ lý dùng hạn mức gói Plus của bạn.', act: 'Chọn model' },
  { tone: 'warning', title: 'Gần hết hạn mức hôm nay', text: 'Còn khoảng 12% — Người viết có thể chậm lại.', act: 'Đổi model' },
  { tone: 'error', title: 'Mất kết nối máy chủ', text: 'Đang thử lại sau 5 giây. Việc đang chạy không bị mất.', act: 'Thử ngay' },
] as const

const cardStyle: CSSProperties = { borderRadius: 16, padding: 18, display: 'flex', flexDirection: 'column' }
function Card({ title, gap = 14, minHeight, style, children }: { title: string; gap?: number; minHeight?: number; style?: CSSProperties; children: ReactNode }) {
  return <section className="card" style={{ ...cardStyle, gap, minHeight, ...style }}><span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span>{children}</section>
}

export function OverlaysGallery() {
  const ack = () => toast('Đã chọn thao tác', 'ok')
  return <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 20 }}>
      <Card title="Tooltip" minHeight={260}>
        <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Rê chuột ~0,4 giây mới hiện, rời là tắt ngay. Một dòng, kèm phím tắt nếu có.</span>
        <div style={{ display: 'flex', gap: 10, padding: '34px 0 30px', justifyContent: 'center' }}>
          {TIPS.map((tip) => <Tooltip key={tip.label} content={tip.label} kbd={tip.kbd || undefined} side={tip.side} asChild>
            <button type="button" className="hov-ghost" aria-label={tip.label} style={{ width: 38, height: 38, borderRadius: 10, border: '1px solid rgba(255,255,255,0.08)', background: 'transparent', color: '#C9C9D1', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Icon name={tip.icon} size={17} /></button>
          </Tooltip>)}
        </div>
      </Card>

      <Card title="Popover giải thích" minHeight={260}>
        <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Bấm để mở (không phải rê chuột), có thể chứa liên kết. Đóng bằng Esc hoặc bấm ra ngoài.</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
          <Badge tone="warn">Cần rảnh</Badge>
          <InfoPopover label="Cần rảnh là gì?">
            <span style={{ fontSize: 13, fontWeight: 600 }}>“Cần rảnh” là gì?</span>
            <span style={{ fontSize: 12.5, color: '#B4B4BE' }}>Lệnh này chỉ chạy khi các trợ lý không viết. Chờ chương hiện tại xong, hoặc bấm Tạm dừng.</span>
            <a href={`#${paths.help()}`} style={{ fontSize: 12.5, color: '#C9A6F5', display: 'flex', alignItems: 'center', gap: 4 }}>Xem hướng dẫn lệnh<Icon name="arrow-right-2" size={13} /></a>
          </InfoPopover>
        </div>
      </Card>

      <Card title="Huy hiệu · chip · phím" gap={12}>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{BADGES.map(([tone, label]) => <Badge key={label} tone={tone} dot>{label}</Badge>)}</div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{CHIPS.map(([icon, key, value]) => <Chip key={key} size="md" icon={<Icon name={icon} size={13} />} label={`${key} `} value={value} />)}</div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, fontSize: 12, color: '#A0A0AB' }}>
          <span><Kbd>⌘</Kbd> <Kbd>K</Kbd></span><span><Kbd>⌥</Kbd> <Kbd>J</Kbd></span><span><Kbd>Esc</Kbd></span><span><Kbd>/</Kbd></span><span><Kbd>?</Kbd></span>
        </div>
      </Card>

      <Card title="Thông báo trong trang" gap={10} style={{ gridColumn: 'span 2' }}>
        {BANNERS.map((banner) => <Banner key={banner.tone} tone={banner.tone} title={banner.title} action={{ label: banner.act, onClick: ack }}>{banner.text}</Banner>)}
      </Card>

      <Card title="Đang tải & trống" gap={12}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}><Skeleton width="70%" height={14} /><Skeleton height={10} /><Skeleton width="85%" height={10} /></div>
        <div style={{ display: 'flex', gap: 14, alignItems: 'center' }}><Spinner size={12} /><Spinner /><Spinner size={20} style={{ borderWidth: 2.5 }} /><span className="kit-shim" style={{ fontSize: 13 }}>Đang phân tích…</span></div>
        <div style={{ marginTop: 4, padding: 18, borderRadius: 12, border: '1px dashed rgba(255,255,255,0.12)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, textAlign: 'center' }}>
          <span style={{ width: 40, height: 40, borderRadius: 12, background: 'rgba(201,166,245,0.1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Icon name="book-1" size={19} style={{ color: '#C9A6F5' }} /></span>
          <span style={{ fontSize: 13.5, fontWeight: 500 }}>Chưa có truyện nào</span><span style={{ fontSize: 12, color: '#8E8E99' }}>Bắt đầu từ một ý tưởng, hoặc nhập truyện có sẵn.</span>
          <div style={{ display: 'flex', gap: 6 }}><Button size="sm" variant="primary" onClick={ack}>Truyện mới</Button><Button size="sm" onClick={ack}>Nhập truyện</Button></div>
        </div>
      </Card>
    </div>

    <Card title="Toast (thành phần dùng chung · chỉ có ở bản dev)" gap={12}>
      <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Pill đáy giữa, tự đóng sau 6 giây (hoàn tác 5 giây). Rê chuột vào chồng toast thì dừng đếm ngược.</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
        <Button size="sm" onClick={() => toast('Đã lưu thay đổi')}>Trung tính</Button>
        <Button size="sm" onClick={() => toast('Đã tạm dừng Người viết', 'ok')}>Thành công</Button>
        <Button size="sm" onClick={() => toast('Chương 25 cần bạn xem lại', 'warn')}>Cần bạn</Button>
        <Button size="sm" onClick={() => toast('Không kết nối được máy chủ', 'err')}>Lỗi</Button>
        <Button size="sm" onClick={() => toast('Đã xuất 24 chương', 'info')}>Thông tin</Button>
        <Button size="sm" onClick={() => toast('Người viết đã sẵn sàng', 'accent')}>Nhấn</Button>
        <Button size="sm" onClick={() => toast('Đã tạm dừng Người viết', 'ok', { label: 'Hoàn tác', onClick: () => { toast('Đã chạy tiếp', 'ok') } }, { undo: true })}>Có hoàn tác (5 s)</Button>
        <Button size="sm" onClick={() => toast('Đã xuất tệp EPUB', 'ok', { label: 'Mở thư mục', onClick: () => undefined })}>Có hành động</Button>
      </div>
    </Card>
  </div>
}
