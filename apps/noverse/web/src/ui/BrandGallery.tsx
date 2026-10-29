// Gallery tab “Nhận diện thương hiệu” — sân chơi defineBrand: chọn màu chính → thấy ngay
// component đổi theo. Gắn brand trong phạm vi (applyBrand lên div bọc) để minh hoạ scope lồng nhau.
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import {
  applyBrand, defineBrand, removeBrand, brandToCss, Button, Chip, Icon, Tabs, Switch, TextField, toast,
  noverseCore, NOVERSE, type Brand,
} from '@noverse/ui'
import '@noverse/ui/kit.css'

// Preset = màu thương hiệu mẫu, lấy từ vai trò công khai của brand Noverse (không literal trong app).
const PRESETS = [
  [NOVERSE.roles.accent, 'Noverse'],
  [NOVERSE.roles.infoStrong, 'Ocean'],
  [NOVERSE.roles.ok, 'Forest'],
  [NOVERSE.roles.warn, 'Amber'],
  [NOVERSE.roles.err, 'Ember'],
  [NOVERSE.roles.user, 'Teal'],
] as const

const cardStyle: CSSProperties = { borderRadius: 16, padding: 18, display: 'flex', flexDirection: 'column' }
const Lbl = ({ children }: { children: ReactNode }) => <span className="lbl" style={{ fontSize: 10.5 }}>{children}</span>

function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return <label style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
    <Lbl>{label}</Lbl>
    <input type="color" aria-label={label} value={value} onChange={(e) => onChange(e.target.value)}
      style={{ width: 34, height: 26, border: '1px solid var(--line-4)', borderRadius: 8, background: 'transparent', padding: 2 }} />
    <span className="mono" style={{ fontSize: 11.5, color: 'var(--color-fg-3)' }}>{value.toUpperCase()}</span>
  </label>
}

export function BrandGallery() {
  const [primary, setPrimary] = useState<string>(NOVERSE.roles.accent)
  const [accent, setAccent] = useState('')
  const [tint, setTint] = useState('')
  const [showCss, setShowCss] = useState(false)
  const scope = useRef<HTMLDivElement | null>(null)

  const brand = useMemo<Brand | null>(() => {
    try {
      return defineBrand({
        name: 'preview',
        primary,
        accent: accent || undefined,
        neutralTint: tint || undefined,
      })
    } catch {
      return null
    }
  }, [primary, accent, tint])

  // Brand mặc định của kit: trả các biến về literal Noverse.
  const fallback = useMemo(() => defineBrand(noverseCore), [])

  useEffect(() => {
    const el = scope.current
    if (!el || !brand) return
    removeBrand(fallback, el)
    applyBrand(brand, el)
    return () => { if (brand) removeBrand(brand, el) }
  }, [brand, fallback])

  return <div className="kit-gallery-grid" style={{ gap: 20, alignItems: 'start' }}>
    <section className="card" style={{ ...cardStyle, gap: 14, gridColumn: '1 / -1' }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 14 }}>
        <span style={{ fontSize: 14, fontWeight: 600 }}>Nhận diện thương hiệu</span>
        <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>đổi màu chính → mọi component dưới đây đổi theo</span>
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, alignItems: 'center' }}>
        {PRESETS.map(([hex, name]) => (
          <button key={hex} type="button" title={name} aria-label={name} onClick={() => setPrimary(hex)}
            style={{
              width: 30, height: 30, borderRadius: 9, cursor: 'pointer', background: hex,
              border: primary.toLowerCase() === hex.toLowerCase() ? '2px solid var(--color-fg)' : '1px solid var(--line-4)',
            }} />
        ))}
        <span style={{ width: 1, height: 22, background: 'var(--line-3)' }} />
        <ColorField label="Chính" value={primary} onChange={setPrimary} />
        <ColorField label="Accent" value={accent || primary} onChange={setAccent} />
        <ColorField label="Sắc xám" value={tint || NOVERSE.roles.edgeWork} onChange={setTint} />
        <Button variant="ghost" size="sm" onClick={() => { setAccent(''); setTint('') }}>Đặt lại</Button>
        <Button variant="ghost" size="sm" onClick={() => setShowCss((v) => !v)}>{showCss ? 'Ẩn CSS' : 'Xem CSS'}</Button>
      </div>
      {!brand && <span style={{ fontSize: 12, color: 'var(--color-err)' }}>Màu không hợp lệ — nhập lại.</span>}
      {showCss && brand && <pre className="mono" style={{ margin: 0, maxHeight: 240, overflow: 'auto', fontSize: 11, color: 'var(--color-fg-3)', background: 'var(--color-surface-0)', border: '1px solid var(--line-2)', borderRadius: 10, padding: 12 }}>{brandToCss(brand, { selector: '.brand-preview' })}</pre>}
    </section>

    <div ref={scope} data-brand="preview" style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 20, gridColumn: '1 / -1' }}>
      <section className="card" style={{ ...cardStyle, gap: 14 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600 }}>Nút</span>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
          <Button variant="primary">Viết chương 1</Button>
          <Button variant="ghost">Huỷ</Button>
          <Button variant="dsoft">Dùng gợi ý</Button>
          <Button variant="danger">Xoá</Button>
          <Button variant="icon" aria-label="Tìm"><Icon name="search-normal-1" size={16} /></Button>
        </div>
      </section>

      <section className="card" style={{ ...cardStyle, gap: 14 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600 }}>Nhãn trạng thái</span>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          <Chip label="Mặc định" />
          <Chip label="Accent" tone="accent" />
          <Chip label="Đạt" tone="ok" />
          <Chip label="Cảnh báo" tone="warn" />
          <Chip label="Bắt sửa" tone="err" />
          <Chip label="Thông tin" tone="info" />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Icon name="lamp-on" size={16} />
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Icon theo accent</span>
        </div>
      </section>

      <section className="card" style={{ ...cardStyle, gap: 14 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600 }}>Ô nhập & bật tắt</span>
        <TextField value="Trà Xanh Cướp Kịch Bản" onChange={() => {}} aria-label="Tên truyện" />
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Switch checked onChange={() => {}} aria-label="Bật" />
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Bật bộ đọc</span>
        </div>
        <div style={{ display: 'flex', gap: 10 }}>
          <Button variant="ghost" size="sm" onClick={() => toast(`Chào từ ${primary.toUpperCase()}`, 'info')}>Thử toast</Button>
        </div>
      </section>

      <section className="card" style={{ ...cardStyle, gap: 14 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600 }}>Tab theo brand</span>
        <Tabs variant="bar" items={[{ id: 'a', label: 'Chương' }, { id: 'b', label: 'Nhân vật' }]} value="a" onChange={() => {}} label="Nhóm" />
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {[50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950].map((stop) => (
            <span key={stop} style={{ flex: '1 1 0', height: 22, borderRadius: 4, background: `var(--scale-primary-${stop})` }} title={`--scale-primary-${stop}`} />
          ))}
        </div>
        <Lbl>Thang tông primary 50 → 950</Lbl>
      </section>
    </div>
  </div>
}
