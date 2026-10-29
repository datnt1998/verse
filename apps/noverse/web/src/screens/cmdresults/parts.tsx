// Mảnh dùng chung của CmdResults.dc.html: chip lệnh vừa gửi, đầu thẻ (ô icon 40px + tiêu đề),
// nút chính/viền theo đúng kích thước artboard.
import type { CSSProperties, ReactNode } from 'react'
import { Icon } from '@noverse/ui'

/** Chip tím phía trên mọi thẻ kết quả: lệnh vừa gửi (`/export exports/tra-xanh.epub`). */
export function CmdChip({ command }: { command: string }) {
  return (
    <div className="cr-rise" style={{ display: 'flex', justifyContent: 'flex-end' }}>
      <span className="mono" style={{
        height: 28, padding: '0 12px', borderRadius: 999, background: 'rgba(201,166,245,0.1)',
        border: '1px solid rgba(201,166,245,0.25)', color: '#DCC8F8', fontSize: 12.5,
        display: 'flex', alignItems: 'center', gap: 8, whiteSpace: 'nowrap', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis',
      }}>
        <Icon name="command" size={13} />{command}
      </span>
    </div>
  )
}

/** Khung thẻ kết quả (card · rise · bo 16). */
export function ResultCard({ children, style, role, label }: {
  children: ReactNode; style?: CSSProperties; role?: 'alert' | 'status'; label?: string
}) {
  return (
    <div className="card cr-rise" role={role} aria-label={label} style={{
      borderRadius: 16, padding: 16, display: 'flex', flexDirection: 'column', gap: 12, ...style,
    }}>
      {children}
    </div>
  )
}

/** Ô icon 40×40 bo 11 + tiêu đề 14.5/600 + dòng phụ; `aside` nằm bên phải. */
export function CardHead({ tint, children, title, sub, subMono, aside, alignTop }: {
  tint: string; children: ReactNode; title: ReactNode; sub?: ReactNode; subMono?: boolean; aside?: ReactNode; alignTop?: boolean
}) {
  return (
    <div style={{ display: 'flex', gap: 12, alignItems: alignTop ? 'flex-start' : 'center' }}>
      <span style={{
        width: 40, height: 40, borderRadius: 11, background: tint, display: 'flex',
        alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
      }}>{children}</span>
      <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: alignTop ? 2 : 0 }}>
        <span style={{ fontSize: 14.5, fontWeight: 600 }}>{title}</span>
        {sub !== undefined && (
          <span className={subMono ? 'mono' : undefined} style={{
            fontSize: subMono ? 12 : 12.5, color: subMono ? '#8E8E99' : '#A0A0AB',
            ...(subMono ? { whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' } : {}),
          }}>{sub}</span>
        )}
      </div>
      {aside}
    </div>
  )
}

/** Dấu tick vẽ dần (stroke-dashoffset) — bìa EPUB & thẻ đã tiếp nhận. */
export function DrawTick({ size, color, stroke }: { size: number; color: string; stroke: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path className="cr-draw" d="M6 12.5l4 4 8-9" stroke={color} strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

type BtnProps = {
  children: ReactNode; icon?: string; iconColor?: string; iconSize?: number; onClick?: () => void; disabled?: boolean; pending?: boolean
  height?: number; style?: CSSProperties; label?: string
}

/** Nút trắng (primary) — 34px, chữ 12.5/600. */
export function PrimaryBtn({ children, icon, onClick, disabled, pending, height = 34, style, label }: BtnProps) {
  return (
    <button className="cr-primary" type="button" onClick={onClick} disabled={disabled || pending} aria-busy={pending} aria-label={label} style={{
      height, padding: '0 14px', borderRadius: 9, border: 'none', background: '#EDEDEF', color: '#0B0B0D',
      fontSize: 12.5, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', ...style,
    }}>
      {pending ? <span className="cr-spinner" style={{ width: 15, height: 15, borderTopColor: '#0B0B0D' }} /> : icon && <Icon name={icon} size={15} />}{children}
    </button>
  )
}

/** Nút viền mảnh (tbtn) — 34px mặc định; `bare` bỏ viền (nút Huỷ). */
export function OutlineBtn({ children, icon, iconColor, iconSize = 15, onClick, disabled, pending, height = 34, style, label, bare }: BtnProps & { bare?: boolean }) {
  return (
    <button className="cr-tbtn" type="button" onClick={onClick} disabled={disabled || pending} aria-busy={pending} aria-label={label} style={{
      height, padding: '0 12px', borderRadius: 9, border: bare ? 'none' : '1px solid rgba(255,255,255,0.1)',
      background: 'transparent', color: bare ? '#A0A0AB' : '#EDEDEF', fontSize: 12.5,
      display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', ...style,
    }}>
      {pending ? <span className="cr-spinner" style={{ width: iconSize, height: iconSize }} /> : icon && <Icon name={icon} size={iconSize} style={iconColor ? { color: iconColor } : undefined} />}{children}
    </button>
  )
}

/** Nhãn nhỏ in hoa (lbl 10.5px) dùng trong ô so sánh / ô đặc điểm. */
export function MiniLabel({ children, color }: { children: ReactNode; color?: string }) {
  return <span className="lbl" style={{ fontSize: 10.5, ...(color ? { color } : {}) }}>{children}</span>
}

export const errorText = (reason: unknown, fallback: string) =>
  reason instanceof Error && reason.message ? reason.message : fallback
