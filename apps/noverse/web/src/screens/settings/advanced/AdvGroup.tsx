// Thẻ nhóm thu gọn + hàng của Tuỳ chỉnh › Nâng cao — Settings.dc.html (sec adv, advGroups).
import { useId, type ReactNode } from 'react'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'

export function AdvGroup({ icon, label, note, open, onToggle, children }: {
  /** tên file icon đã tra trong `.i-<alias>` của artboard */
  icon: string
  label: string
  note: string
  open: boolean
  onToggle: () => void
  children: ReactNode
}) {
  const drawer = useId()
  return (
    <section className="card" style={{ borderRadius: 16, overflow: 'hidden' }}>
      <button type="button" className="hov-row" onClick={onToggle} aria-expanded={open} aria-controls={drawer} style={{
        width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px', border: 'none',
        background: 'transparent', color: '#EDEDEF', textAlign: 'left',
      }}>
        <Icon name={icon} size={17} style={{ color: '#A0A0AB' }} />
        <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
          <span style={{ fontSize: 14, fontWeight: 500 }}>{label}</span>
          <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{note}</span>
        </span>
        <Icon name="arrow-down-2" size={15} style={{
          color: '#7E7E8A', transform: `rotate(${open ? 180 : 0}deg)`, transition: 'transform .25s',
        }} />
      </button>
      {open && (
        <div id={drawer} className="st-drawer" style={{ padding: '0 16px 14px 45px', display: 'flex', flexDirection: 'column', gap: 2 }}>
          {children}
        </div>
      )}
    </section>
  )
}

/** Hàng nhãn · giá trị mono · nút ghi 28px (artboard: drawer row). */
export function AdvRow({ label, value, valueTitle, children, below }: {
  label: string
  value: ReactNode
  valueTitle?: string
  /** nút hành động bên phải */
  children?: ReactNode
  /** nội dung mở rộng dưới hàng (hướng dẫn, thẻ mã khôi phục) */
  below?: ReactNode
}) {
  return (
    <>
      <div style={{
        display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '4px 12px', minHeight: 40,
        borderTop: '1px solid rgba(255,255,255,0.05)',
      }}>
        <span style={{ flex: '0 0 210px', fontSize: 13, color: '#C9C9D1' }}>{label}</span>
        <span className="mono" title={valueTitle} style={{
          flex: '1 1 160px', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          fontSize: 12, color: '#A0A0AB',
        }}>{value}</span>
        {children}
      </div>
      {below}
    </>
  )
}

/** Nút ghost của hàng: cao 28, bo 7, viền .1, chữ 12 — chờ thì thay nhãn bằng spinner, giữ bề rộng. */
export function AdvButton({ children, onClick, pending, disabled, title, expanded }: {
  children: ReactNode
  onClick: () => void
  pending?: boolean
  disabled?: boolean
  title?: string
  expanded?: boolean
}) {
  return (
    <button type="button" className="hov-ghost" onClick={onClick} disabled={disabled || pending}
      aria-busy={pending || undefined} aria-expanded={expanded} title={title} style={{
        height: 28, padding: '0 10px', borderRadius: 7, border: '1px solid rgba(255,255,255,0.1)',
        background: 'transparent', color: '#EDEDEF', fontSize: 12, whiteSpace: 'nowrap',
        display: 'inline-grid', alignItems: 'center', justifyItems: 'center',
        opacity: disabled && !pending ? 0.45 : 1,
      }}>
      <span style={{ gridArea: '1 / 1', visibility: pending ? 'hidden' : undefined }}>{children}</span>
      {pending && <span style={{ gridArea: '1 / 1' }}><Spinner size={13} color="currentColor" /></span>}
    </button>
  )
}
