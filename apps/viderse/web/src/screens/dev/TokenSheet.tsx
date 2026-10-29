// Bảng swatch token đọc GIÁ TRỊ SỐNG từ CSS (:root sau khi applyBrand), in giá
// trị ngay cạnh ô màu để đối chiếu với artboard Kit › Màu / Bo góc / Chữ.
// Nhóm màu của brand (nhấn, trạng thái, trợ lý) đổi theo brand nên phải đọc
// lúc chạy — không chép lại hex vào tài liệu.
import { useEffect, useState } from 'react'

type Swatch = { label: string; token: string }
type Group = { title: string; note?: string; kind: 'color' | 'radius' | 'size' | 'font'; swatches: Swatch[] }

const GROUPS: Group[] = [
  {
    title: 'Nền & bề mặt',
    kind: 'color',
    swatches: [
      { label: 'Nền', token: '--color-bg' },
      { label: 'Bề mặt 1', token: '--color-surface-1' },
      { label: 'Bề mặt 2', token: '--color-surface-2' },
      { label: 'Card', token: '--color-card' },
      { label: 'Viền 1', token: '--line-1' },
      { label: 'Viền 2', token: '--line-2' },
    ],
  },
  {
    title: 'Chữ',
    kind: 'color',
    swatches: [
      { label: 'Chính', token: '--color-fg' },
      { label: 'Phụ 2', token: '--color-fg-2' },
      { label: 'Phụ 3', token: '--color-fg-3' },
      { label: 'Mờ nhất', token: '--color-fg-4' },
    ],
  },
  {
    title: 'Brand — nhấn, công tắc, logo',
    kind: 'color',
    swatches: [
      { label: 'Nhấn', token: '--color-accent' },
      { label: 'Công tắc bật (accentDeep)', token: '--kit-switch-on' },
      { label: 'Logo YouTube', token: '--vv-red' },
      { label: 'Nút + điện thoại', token: '--vv-red-button' },
    ],
  },
  {
    title: 'Trạng thái',
    kind: 'color',
    swatches: [
      { label: 'Sẵn sàng / Xong', token: '--color-ok' },
      { label: 'Chờ bạn / Chờ duyệt', token: '--color-warn' },
      { label: 'Lỗi / Xoá', token: '--color-err' },
      { label: 'Đang làm / Thông tin', token: '--color-info' },
      { label: 'Chờ lượt / Đã đăng', token: '--vv-st-neutral' },
    ],
  },
  {
    title: 'Trợ lý — orb & chữ',
    kind: 'color',
    swatches: [
      { label: 'Trinh sát · orb', token: '--orb-architect' },
      { label: 'Đạo diễn · orb', token: '--orb-writer' },
      { label: 'Hoạ sĩ · orb', token: '--orb-editor' },
      { label: 'Kiểm định · orb', token: '--orb-arbiter' },
      { label: 'Trinh sát · chữ', token: '--vv-scout-ink' },
      { label: 'Đạo diễn · chữ', token: '--vv-director-ink' },
      { label: 'Hoạ sĩ · chữ', token: '--vv-artist-ink' },
      { label: 'Kiểm định · chữ', token: '--vv-qa-ink' },
    ],
  },
  {
    title: 'Bo góc',
    note: 'kit › radius',
    kind: 'radius',
    swatches: [
      { label: 'Chip', token: '--radius-chip' },
      { label: 'Ô điều khiển', token: '--radius-control' },
      { label: 'Nút', token: '--radius-button' },
      { label: 'Hàng', token: '--radius-row' },
      { label: 'Khối nhỏ', token: '--radius-tile' },
      { label: 'Card', token: '--radius-card' },
      { label: 'Dock', token: '--radius-dock' },
    ],
  },
  {
    title: 'Khung & vùng chạm',
    note: 'app › viderse.css',
    kind: 'size',
    swatches: [
      { label: 'Thanh đầu', token: '--vv-h-header' },
      { label: 'Thanh điện thoại', token: '--vv-m-bar' },
      { label: 'Thanh dưới', token: '--vv-m-tabbar' },
      { label: 'Vùng chạm', token: '--vv-hit' },
    ],
  },
  {
    title: 'Phông chữ',
    kind: 'font',
    swatches: [
      { label: 'Giao diện', token: '--font-sans' },
      { label: 'Số & mã', token: '--font-mono' },
      { label: 'Đọc dài', token: '--font-read' },
    ],
  },
]

/** Giá trị của token trên :root, đọc lại khi mount (brand chạy trước render). */
function useTokenValues(tokens: string[]) {
  const [values, setValues] = useState<Record<string, string>>({})
  useEffect(() => {
    const style = getComputedStyle(document.documentElement)
    const next: Record<string, string> = {}
    for (const token of tokens) next[token] = style.getPropertyValue(token).trim()
    setValues(next)
  }, [tokens.join(',')])
  return values
}

const ALL = GROUPS.flatMap((group) => group.swatches.map((swatch) => swatch.token))

export function TokenSheet() {
  const values = useTokenValues(ALL)
  return (
    <div className="vv-dev__grid">
      {GROUPS.map((group) => (
        <div key={group.title} className="vv-dev__cell vv-dev__cell--wide">
          <span className="vv-dev__cell-head">
            {group.title}
            {group.note && <span style={{ textTransform: 'none', letterSpacing: 0, fontWeight: 400, marginLeft: 'var(--vv-s-8)' }}>· {group.note}</span>}
          </span>
          <div style={{ display: 'grid', gap: 'var(--vv-s-10)', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))' }}>
            {group.swatches.map((swatch) => (
              <div key={swatch.token} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)', minWidth: 0 }}>
                <SwatchPreview kind={group.kind} token={swatch.token} value={values[swatch.token]} />
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-2)' }}>{swatch.label}</span>
                <code className="mono" style={{ fontSize: 10.5, color: 'var(--color-fg-4)', overflowWrap: 'anywhere' }}>{swatch.token}</code>
                <code className="mono" style={{ fontSize: 11, color: 'var(--color-fg-3)', overflowWrap: 'anywhere' }}>{values[swatch.token] || '—'}</code>
              </div>
            ))}
          </div>
        </div>
      ))}
      <div className="vv-dev__cell vv-dev__cell--wide">
        <span className="vv-dev__cell-head">Thang chữ (giá trị đo từ mẫu đang hiển thị)</span>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)' }}>
          {[
            { size: 30, label: 'Tiêu đề lớn', weight: 700 },
            { size: 17, label: 'Tiêu đề trang', weight: 600 },
            { size: 14, label: 'Thân', weight: 400 },
            { size: 13, label: 'Nút & ô nhập', weight: 500 },
            { size: 12, label: 'Meta trạng thái', weight: 400 },
            { size: 11, label: 'Nhãn nhóm', weight: 600 },
          ].map((step) => (
            <span key={step.size} style={{ display: 'flex', alignItems: 'baseline', gap: 'var(--vv-s-10)' }}>
              <span style={{ fontSize: step.size, fontWeight: step.weight }}>Viderse — Chờ duyệt</span>
              <code className="mono" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{step.size}px / {step.weight} · {step.label}</code>
            </span>
          ))}
        </div>
      </div>
    </div>
  )
}

function SwatchPreview({ kind, token, value }: { kind: Group['kind']; token: string; value: string | undefined }) {
  if (kind === 'color') {
    return <span style={{ height: 44, borderRadius: 'var(--radius-control)', background: value ? `var(${token})` : 'var(--color-surface-2)', border: '1px solid var(--line-1)' }} />
  }
  if (kind === 'radius') {
    return <span style={{ height: 44, borderRadius: value ? `var(${token})` : undefined, background: 'var(--color-surface-3)', border: '1px solid var(--line-2)' }} />
  }
  if (kind === 'size') {
    return (
      <span style={{ height: 44, display: 'flex', alignItems: 'flex-end' }}>
        <span style={{ width: '100%', height: `min(44px, ${value || '0px'})`, borderRadius: 'var(--radius-chip)', background: 'var(--color-surface-3)', border: '1px solid var(--line-2)' }} />
      </span>
    )
  }
  return <span style={{ height: 44, display: 'flex', alignItems: 'center', fontFamily: `var(${token})`, fontSize: 16 }}>Viderse 123</span>
}
