// Mảnh dựng chung của các mục Tuỳ chỉnh, đúng lớp của Settings.dc.html (.sec .srow .stx .ovr .sw .segc).
import { Switch } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import type { CSSProperties, ReactNode } from 'react'
import type { ConfigScope } from '../../api/types'
import { resetLabel, type Ovr } from './lib/labels'

export function SectionHead({ title, note }: { title: string; note?: string }) {
  return (
    <div>
      <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600 }}>{title}</h1>
      {note && <p style={{ margin: '4px 0 0', fontSize: 13.5, color: '#A0A0AB' }}>{note}</p>}
    </div>
  )
}

export function Page({ children, top = 18, maxWidth = 820 }: { children: ReactNode; top?: number; maxWidth?: number }) {
  return <div style={{ maxWidth, marginTop: top, display: 'flex', flexDirection: 'column', gap: 14 }}>{children}</div>
}

export function SecCard({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <section className="card st-sec" style={style}>{children}</section>
}

export function SRow({ label, note, children, center, style }: {
  label: ReactNode; note?: ReactNode; children?: ReactNode; center?: boolean; style?: CSSProperties
}) {
  return (
    <div className="st-srow" style={{ ...(center ? { alignItems: 'center' } : null), ...style }}>
      <div className="st-stx"><b>{label}</b>{note && <span>{note}</span>}</div>
      {children}
    </div>
  )
}

/** Nhãn kế thừa + nút *Theo dự án* đặt lại (xoá khoá ở cấp đang chọn). */
export function OvrBadge({ ovr, scope, onReset, disabled, width, ownTestId }: {
  ovr: Ovr; scope: ConfigScope; onReset?: () => void; disabled?: boolean; width?: number; ownTestId?: string
}) {
  return (
    <>
      <span className={`st-ovr ${ovr.cls}`} data-testid={ownTestId}
        style={width ? { width, justifyContent: 'center', flex: '0 0 auto' } : undefined}>{ovr.text}</span>
      {ovr.reset && onReset && (
        <button type="button" className="hov-ghost" onClick={onReset} disabled={disabled}
          style={{ height: 24, padding: '0 8px', border: 'none', borderRadius: 6, background: 'transparent', color: '#C9A6F5', fontSize: 12 }}>
          {resetLabel(scope)}
        </button>
      )}
    </>
  )
}

export function Sw({ on, label, onToggle, disabled }: { on: boolean; label: string; onToggle: () => void; disabled?: boolean }) {
  return <Switch checked={on} label={label} onChange={onToggle} disabled={disabled} size="lg" />
}

export function Segc<T extends string | boolean>({ value, options, label, onPick, disabled, stretch }: {
  value: T; options: { id: T; label: string }[]; label?: string; onPick: (id: T) => void; disabled?: boolean
  /** Giãn kín cột lưới: 4 mục chia đều; 5 mục (thêm “Rất cao”) giữ khổ tự nhiên vừa đúng cột. */
  stretch?: boolean
}) {
  // Bề rộng cột cố định (Settings mô hình): 4 mục giãn đều vẫn đủ chỗ cho “Tự động”; mục thứ năm
  // dùng bề rộng tự nhiên nên cả hai trường hợp đều đúng một khổ.
  const fill = stretch && options.length <= 4
  return (
    <Segmented label={label ?? ''} size="sm"
      value={String(value)} onChange={(id) => onPick(options.find((option) => String(option.id) === id)!.id)}
      options={options.map((option) => ({
        id: String(option.id), label: option.label, disabled,
        style: { flex: fill ? '1 1 0' : '0 0 auto', minWidth: fill ? 0 : undefined, padding: '0 11px' },
      }))}
      style={{ display: stretch ? 'flex' : 'inline-flex', gap: 2, border: '1px solid var(--line-2)', ...(stretch ? { width: '100%' } : null) }} />
  )
}
