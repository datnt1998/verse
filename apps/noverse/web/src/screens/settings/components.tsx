// Mảnh dùng chung cho màn Tuỳ chỉnh — bám Settings.dc.html: SettingRow, KeyTag, Segmented nhỏ,
// SourceBadge (nguồn kế thừa), ReadOnlyValue (viền nét đứt), InheritChain, SaveBar.
import {
  Children, cloneElement, isValidElement, useState, type CSSProperties, type ReactNode,
} from 'react'
import type { ConfigScope, RevealRequest } from '../../api/types'
import { ApiError, getRevealPath, revealFolder } from '../../api/client'
import { Button } from '@noverse/ui'
import { Card } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { SlowLoadingNotice } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { copyText } from '../../lib/clipboard'

export const SCOPE_LABEL: Record<ConfigScope, string> = {
  global: 'Toàn cục',
  project: 'Dự án',
  story: 'Truyện',
}

/** Chip khoá cấu hình (artboard `.key`). */
export function KeyTag({ children }: { children: ReactNode }) {
  return (
    <span className="mono" style={{
      fontSize: 11, color: 'var(--color-fg-4)', padding: '1px 6px', borderRadius: 5,
      background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', marginLeft: 6,
    }}>{children}</span>
  )
}

export function SectionBadge({ tone = 'neutral', children }: { tone?: 'neutral' | 'accent'; children: ReactNode }) {
  return (
    <span style={{
      height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', borderRadius: 6,
      background: tone === 'accent' ? 'var(--tint-accent)' : 'var(--line-1)',
      color: tone === 'accent' ? 'var(--color-accent-soft)' : 'var(--color-fg-3)',
      fontSize: 11, fontWeight: 500, whiteSpace: 'nowrap',
    }}>{children}</span>
  )
}

export function Section({ id, title, icon, keyName, badge, children }: {
  id: string; title: string; icon: string; keyName?: string
  badge?: ReactNode; children: ReactNode
}) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <h2 id={`${id}-title`} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 10, fontSize: 15, fontWeight: 600 }}>
        <Icon name={icon} style={{ color: 'var(--color-fg-3)' }} />
        {title}
        {keyName && <KeyTag>{keyName}</KeyTag>}
        {badge}
      </h2>
      {children}
    </section>
  )
}

/** Gắn `first` cho con trực tiếp đầu tiên để bỏ đường kẻ trên (artboard `.row:first-child`). */
export function Rows({ children }: { children: ReactNode }) {
  return (
    <>
      {Children.toArray(children).map((child, index) =>
        isValidElement<{ first?: boolean }>(child) ? cloneElement(child, { first: index === 0 }) : child,
      )}
    </>
  )
}

export function TinyButton({ children, onClick, tone = 'neutral', disabled, title, icon }: {
  children: ReactNode; onClick: () => void; tone?: 'neutral' | 'accent'
  disabled?: boolean; title?: string; icon?: string
}) {
  return (
    <button type="button" className="hov-ghost" onClick={onClick} disabled={disabled} title={title}
      style={{
        height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', gap: 5,
        borderRadius: 6, background: 'transparent', border: '1px solid var(--line-4)',
        color: tone === 'accent' ? 'var(--color-accent-soft)' : 'var(--color-fg-3)', fontSize: 11,
      }}>
      {icon && <Icon name={icon} size={12} />}
      {children}
    </button>
  )
}

export type FieldState = {
  path: string
  scope: ConfigScope
  source: ConfigScope
  /** khoá đã có trong tệp của cấp đang sửa */
  overridden: boolean
  canEdit: boolean
  onOverride: () => void
  onReset: () => void
}

export function FieldSourceLine({ scope, source, overridden, canEdit, onOverride, onReset }: FieldState) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6, marginTop: 4 }}>
      <span style={{
        height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 5,
        fontSize: 11,
        background: overridden ? 'var(--tint-accent)' : 'var(--line-1)',
        color: overridden ? 'var(--color-accent-soft)' : 'var(--color-fg-4)',
      }}>
        {overridden ? `Đã đặt ở ${SCOPE_LABEL[scope].toLowerCase()}` : `Kế thừa · ${SCOPE_LABEL[source].toLowerCase()}`}
      </span>
      {canEdit && !overridden && <TinyButton onClick={onOverride}>Ghi đè</TinyButton>}
      {canEdit && overridden && <TinyButton onClick={onReset}>Khôi phục</TinyButton>}
    </div>
  )
}

export type SettingRowProps = {
  first?: boolean
  label?: ReactNode
  htmlFor?: string
  keyName?: ReactNode
  desc?: ReactNode
  /** icon đứng trước khối nhãn (artboard dùng cho hàng thư mục quy tắc) */
  icon?: ReactNode
  align?: 'center' | 'flex-start'
  controlStyle?: CSSProperties
  field?: FieldState
  children: ReactNode
}

export function SettingRow({
  first, label, htmlFor, keyName, desc, icon, align = 'center', controlStyle, field, children,
}: SettingRowProps) {
  return (
    <div style={{
      display: 'flex', flexWrap: 'wrap', alignItems: align, gap: '10px 20px',
      padding: '14px 18px', borderTop: first ? undefined : '1px solid var(--line-1)',
    }}>
      {icon}
      <div style={{ flex: '1 1 240px', minWidth: 0 }}>
        {(label || keyName) && (
          <div style={{ fontSize: 14, fontWeight: 500 }}>
            {htmlFor ? <label htmlFor={htmlFor}>{label}</label> : label}
            {keyName !== undefined && <KeyTag>{keyName}</KeyTag>}
          </div>
        )}
        {desc && <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{desc}</div>}
        {field && <FieldSourceLine {...field} />}
      </div>
      <div style={{
        display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8,
        minWidth: 0, justifyContent: 'flex-end', ...controlStyle,
      }}>
        {children}
      </div>
    </div>
  )
}

/** Giá trị chỉ đọc — viền nét đứt (artboard `.ro`). */
export function ReadOnlyValue({ children, mono = true, title, icon }: {
  children: ReactNode; mono?: boolean; title?: string; icon?: string
}) {
  return (
    <span title={title} className={mono ? 'mono' : undefined} style={{
      minHeight: 28, height: 28, padding: '0 10px', display: 'inline-flex', alignItems: 'center', gap: 6,
      borderRadius: 7, background: 'var(--color-surface-0)', border: '1px dashed var(--line-4)',
      fontSize: 12, color: 'var(--color-fg-3)', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    }}>
      {icon && <Icon name={icon} size={13} />}
      {children}
    </span>
  )
}

/** Sao chép đường dẫn/mã — dùng khi máy chủ không mở được thư mục (canReveal=false). */
export function CopyButton({ value, label = 'Sao chép', size = 30 }: { value: string; label?: string; size?: number }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await copyText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 1600)
    } catch {
      toast.err('Trình duyệt chặn sao chép — hãy chọn và sao chép thủ công.')
    }
  }
  return (
    <Button variant="icon" aria-label={label} title={label} onClick={() => void copy()} style={{ width: size, height: size }}>
      <Icon name={copied ? 'tick-circle' : 'copy'} size={15} />
    </Button>
  )
}

/**
 * G1 — “Mở thư mục” khi máy chủ chạy ngay trên máy này (`canReveal`), ngược lại là nút chép
 * đường dẫn. `compact` = chỉ icon (dùng ở tiêu đề panel, như Reader.dc.html).
 */
export function RevealButton({ input, canReveal, copyValue, label, compact, size = 30, disabled }: {
  input: RevealRequest
  canReveal: boolean
  /** đường dẫn hiển thị, dùng khi GET /api/reveal không trả được đường dẫn thật */
  copyValue: string
  /** nhãn đầy đủ, dùng cho aria-label/title */
  label: string
  compact?: boolean
  size?: number
  disabled?: boolean
}) {
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!canReveal) {
    // Ngoài loopback: xin đường dẫn tuyệt đối từ máy chủ (GET không mở gì), rồi chép.
    const copyPath = async () => {
      let text = copyValue
      try {
        text = (await getRevealPath(input)).path || copyValue
      } catch { /* giữ đường dẫn đang hiện */ }
      try {
        await copyText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 1600)
      } catch {
        toast.err('Trình duyệt chặn sao chép — hãy chọn và sao chép thủ công.')
      }
    }
    return (
      <Button variant="icon" aria-label={`Sao chép đường dẫn ${label}`} title={`Sao chép đường dẫn ${label}`}
        onClick={() => void copyPath()} style={{ width: size, height: size }}>
        <Icon name={copied ? 'tick-circle' : 'copy'} size={15} />
      </Button>
    )
  }

  const reveal = async () => {
    setBusy(true)
    try {
      await revealFolder(input)
      toast.ok('Đã mở thư mục trên máy chủ')
    } catch (error) {
      toast.err(error instanceof ApiError ? error.message : 'Không mở được thư mục trên máy chủ.')
    } finally {
      setBusy(false)
    }
  }
  return (
    <Button variant={compact ? 'icon' : 'ghost'} aria-label={label} title={label}
      onClick={() => void reveal()} disabled={disabled || busy}
      style={compact ? { width: size, height: size } : { height: size, padding: '0 10px 0 8px', fontSize: 12 }}>
      {busy ? <Spinner size={14} /> : <Icon name="folder-open" size={compact ? 16 : 15} />}
      {!compact && 'Mở thư mục'}
    </Button>
  )
}

export type SegmentOption = { id: string; label: ReactNode; disabled?: boolean }

/** Segmented nhỏ đúng artboard: cao 28/30, bo 9/10, nền surface-0/1. */
export function InlineSegmented({ options, value, onChange, label, small = true, mono = false, disabled }: {
  options: SegmentOption[]; value: string; onChange: (id: string) => void
  label: string; small?: boolean; mono?: boolean; disabled?: boolean
}) {
  return (
    <Segmented label={label} size={small ? 'sm' : 'md'} value={value} onChange={onChange}
      options={options.map((option) => ({
        id: option.id, disabled: disabled || option.disabled,
        label: mono ? <span className="mono">{option.label}</span> : option.label,
        style: { flex: '0 0 auto', padding: '0 12px', fontSize: small ? 12 : 13 },
      }))}
      style={{
        gap: 2, borderRadius: small ? 9 : 10,
        background: small ? 'var(--color-surface-0)' : 'var(--color-surface-1)',
        border: `1px solid ${small ? 'var(--line-3)' : 'var(--line-1)'}`,
        minWidth: 0,
      }} />
  )
}

/** Chuỗi kế thừa: ~/.ainovel → dự án → truyện; mục cuối (cấp đang sửa) tô sáng. */
export function InheritChain({ label, note, steps }: {
  label: ReactNode
  note: ReactNode
  steps: { path: string; exists: boolean }[]
}) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <span style={{ fontSize: 13, fontWeight: 500 }}>{label}</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
        {steps.map((step, index) => (
          <span key={`${step.path}-${index}`} style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <code className="mono" title={step.exists ? 'Đã có' : 'Chưa có tệp'} style={{
              height: 26, padding: '0 9px', display: 'flex', alignItems: 'center', borderRadius: 7,
              background: 'var(--color-surface-0)',
              border: `1px ${step.exists ? 'solid' : 'dashed'} ${index === steps.length - 1 ? 'var(--line-5)' : 'var(--line-1)'}`,
              fontSize: 11, color: index === steps.length - 1 ? 'var(--color-fg)' : 'var(--color-fg-3)',
            }}>{step.path}</code>
            {index < steps.length - 1 && <Icon name="arrow-right-2" size={12} style={{ color: 'var(--color-fg-4)' }} />}
          </span>
        ))}
      </div>
      <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{note}</span>
    </div>
  )
}

/** SaveBar neo đáy (artboard) + pill "Đã lưu cấu hình". */
export function SaveBar({ dirty, saving, disabled = false, paths, onSave, onDiscard, saved, note, sticky }: {
  dirty: boolean; saving: boolean; disabled?: boolean; paths: string[]
  onSave: () => void; onDiscard: () => void; saved: boolean; note?: ReactNode; sticky?: boolean
}) {
  if (!dirty && !saved) return null
  return (
    <div style={{
      padding: sticky ? '8px 12px calc(8px + env(safe-area-inset-bottom, 0px))' : '0 24px 88px',
      display: 'flex', justifyContent: 'center',
      ...(sticky ? { position: 'sticky', bottom: 0, zIndex: 5, background: 'var(--color-bg)' } : {}),
    }}>
      {dirty ? (
        <Card role="status" padding="8px 8px 8px 16px" radius={14} style={{
          width: '100%', maxWidth: 852, display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 10,
          background: 'var(--color-surface-4)', boxShadow: '0 20px 50px -10px rgba(0,0,0,0.8)',
        }}>
          <span style={{ flex: 1, minWidth: 180, fontSize: 13, color: 'var(--color-fg-2)' }}>
            Có thay đổi chưa lưu · sẽ ghi vào{' '}
            <span className="mono" style={{ fontSize: 12 }}>{paths.join(' · ')}</span>
            {note && <span style={{ display: 'block', fontSize: 12, color: 'var(--color-fg-4)' }}>{note}</span>}
          </span>
          <Button onClick={onDiscard} disabled={saving} style={{ height: 34, padding: '0 14px', borderRadius: 9 }}>Hủy</Button>
          <Button variant="primary" title={disabled && !saving ? 'Mất kết nối hoặc đang lưu · ⌘S / Ctrl+S' : 'Lưu cấu hình · ⌘S / Ctrl+S'} onClick={onSave} pending={saving} disabled={disabled || saving} style={{ height: 34, padding: '0 16px', borderRadius: 9 }}>
            Lưu cấu hình
          </Button>
        </Card>
      ) : (
        <div role="status" style={{
          height: 36, padding: '0 14px', borderRadius: 999, display: 'flex', alignItems: 'center', gap: 8,
          background: 'var(--color-surface-4)', border: '1px solid var(--line-3)', fontSize: 13,
        }}>
          <Icon name="tick-circle-bulk" size={16} style={{ color: 'var(--color-ok)' }} />
          Đã lưu cấu hình
        </div>
      )}
    </div>
  )
}

export function LoadingRows({ rows = 3 }: { rows?: number }) {
  return (
    <Card radius={16} padding="6px 18px">
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} style={{
          display: 'flex', alignItems: 'center', gap: 16, padding: '16px 0',
          borderTop: index === 0 ? undefined : '1px solid var(--line-1)',
        }}>
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 7 }}>
            <div style={{ height: 12, width: '38%', borderRadius: 6, background: 'var(--color-surface-4)' }} />
            <div style={{ height: 10, width: '62%', borderRadius: 6, background: 'var(--color-surface-2)' }} />
          </div>
          <div style={{ height: 30, width: 140, borderRadius: 8, background: 'var(--color-surface-3)' }} />
        </div>
      ))}
      <SlowLoadingNotice />
    </Card>
  )
}

export function ErrorCard({ message, onRetry, retryLabel = 'Thử lại' }: {
  message: ReactNode; onRetry?: () => void; retryLabel?: string
}) {
  return (
    <Card radius={16} padding="16px 18px" style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 12 }}>
      <Icon name="warning-2" size={18} style={{ color: 'var(--color-err)' }} />
      <span style={{ flex: 1, minWidth: 180, fontSize: 13, color: 'var(--color-fg-2)' }}>{message}</span>
      {onRetry && <Button onClick={onRetry} style={{ height: 30, borderRadius: 8 }}>{retryLabel}</Button>}
    </Card>
  )
}

export function ReadOnlyNotice({ children }: { children: ReactNode }) {
  return (
    <Card radius={12} padding="10px 14px" style={{
      display: 'flex', alignItems: 'center', gap: 10, fontSize: 12, color: 'var(--color-fg-3)',
      background: 'var(--color-surface-1)',
    }}>
      <Icon name="shield-tick" size={16} style={{ color: 'var(--color-fg-4)' }} />
      <span style={{ flex: 1, minWidth: 0 }}>{children}</span>
    </Card>
  )
}
