// ActionSheet / ConfirmSheet — tấm thao tác điện thoại.
//
// Hai cách trình bày, cùng một hành vi (khoá lời hứa, báo lỗi tại chỗ):
// · `list`    — tấm danh sách theo artboard Viderse MKit (khối "Sheet"): tiêu đề 18,
//               các hàng 56 có icon + mô tả phụ, hàng nguy hiểm màu chữ cam.
// · `grouped` — nhóm kiểu iOS mà Noverse đang dùng (MMore): nhóm nền surface, hàng 56,
//               nút Huỷ riêng. CSS của nhóm này nằm ở app (Noverse `.m-action-group`,
//               `.m-confirm-*`) để màn Noverse giữ nguyên từng điểm ảnh; kit giữ đúng
//               cấu trúc lớp và hành vi.
import { useId, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius } from '../tokens.stylex'
import { Icon } from '../Icon'
import type { IconName } from '../iconNames'
import { getKitLabels } from '../labels'
import { Sheet } from '../Sheet'
import { Button } from '../Button'

const styles = stylex.create({
  list: { display: 'flex', flexDirection: 'column', paddingInline: 8, paddingBottom: 14 },
  row: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    width: '100%',
    minHeight: 56,
    paddingTop: 6,
    paddingBottom: 6,
    paddingInline: 10,
    borderWidth: 0,
    borderRadius: radius['--radius-tile'],
    backgroundColor: 'transparent',
    color: color['--color-fg'],
    fontSize: 13.5,
    textAlign: 'start',
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
    ':active': { backgroundColor: kit['--kit-wash-6'] },
    ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: -2 },
  },
  danger: { color: kit['--kit-err-ghost-ink'] },
  rowText: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 1 },
  rowLabel: { fontSize: 13.5, lineHeight: 1.35 },
  rowHint: { fontSize: 11.5, lineHeight: 1.4, color: kit['--kit-fg-sub'] },
  /* Hàng bị khoá (chức năng chưa mở): vẫn đọc được lý do, không bấm được. */
  rowDisabled: { color: kit['--kit-fg-sub'], cursor: 'not-allowed', opacity: 0.55, ':active': { backgroundColor: 'transparent' } },
  error: { paddingInline: 18, paddingBottom: 8, fontSize: 11.5, color: kit['--kit-err-bright'] },
  errorInline: { fontSize: 11.5, color: kit['--kit-err-bright'] },
  confirm: { display: 'flex', flexDirection: 'column', gap: 10, paddingTop: 6, paddingInline: 16, paddingBottom: 16 },
  confirmTitle: { fontSize: 18, fontWeight: 600, lineHeight: 1.35, color: color['--color-fg'] },
  confirmText: { fontSize: 13.5, lineHeight: 1.5, color: kit['--kit-fg-sub'] },
  confirmButtons: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginTop: 4 },
})

/** Một hành động trong tấm thao tác. `onSelect` trả lời hứa: tấm khoá tới khi xong. */
export type MobileSheetAction = {
  key?: string
  label: string
  /** dòng phụ 11.5px dưới nhãn (chỉ cách trình bày `list`) */
  description?: string
  icon?: IconName
  destructive?: boolean
  /** Hàng khoá (chức năng chưa mở, hoặc đang bị ràng buộc); `description` giữ lý do. */
  disabled?: boolean
  onSelect: () => void | Promise<void>
}

const message = (error: unknown, fallback: string) => (error instanceof Error && error.message ? error.message : fallback)

export type ActionSheetProps = {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  actions: MobileSheetAction[]
  /** `list` (mặc định, artboard MKit) hoặc `grouped` (kiểu iOS của Noverse) */
  layout?: 'list' | 'grouped'
  cancelLabel?: string
}

export function ActionSheet({ open, onClose, title, description, actions, layout = 'list', cancelLabel }: ActionSheetProps) {
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const t = getKitLabels()
  const close = () => { if (!pending) onClose() }
  const run = async (action: MobileSheetAction) => {
    setPending(true)
    setFailure('')
    try {
      await action.onSelect()
      onClose()
    } catch (error) {
      setFailure(message(error, t.confirm.actionFailed))
    } finally {
      setPending(false)
    }
  }

  if (layout === 'grouped') {
    return (
      <Sheet open={open} onClose={close} height="auto" style={{ background: 'transparent', border: 0, boxShadow: 'none' }}>
        <div className="m-action-group">
          <h2>{title}</h2>
          {description && <p>{description}</p>}
          {failure && <p className="m-error" role="alert">{failure}</p>}
          {actions.map((action) => (
            <button
              key={action.key ?? action.label}
              type="button"
              disabled={pending || action.disabled}
              aria-disabled={action.disabled || undefined}
              className={action.destructive && !action.disabled ? 'm-danger-text' : undefined}
              onClick={() => { void run(action) }}
            >
              {action.label}
            </button>
          ))}
        </div>
        <button className="m-action-cancel" type="button" disabled={pending} onClick={onClose}>
          {pending ? t.mobile.pending : cancelLabel ?? t.cancel}
        </button>
      </Sheet>
    )
  }

  return (
    <Sheet open={open} onClose={close} size="md" title={title} subtitle={description}>
      <div {...stylex.props(styles.list)}>
        {actions.map((action) => {
          const off = pending || action.disabled
          return (
            <button
              key={action.key ?? action.label}
              type="button"
              disabled={off}
              aria-disabled={action.disabled || undefined}
              aria-busy={pending || undefined}
              className={stylex.props(styles.row, action.destructive && !action.disabled && styles.danger, action.disabled && styles.rowDisabled).className}
              style={stylex.props(styles.row, action.destructive && !action.disabled && styles.danger, action.disabled && styles.rowDisabled).style}
              onClick={() => { void run(action) }}
            >
              {action.icon && <Icon name={action.icon} size={20} />}
              <span {...stylex.props(styles.rowText)}>
                <span {...stylex.props(styles.rowLabel)}>{action.label}</span>
                {action.description && <span {...stylex.props(styles.rowHint)}>{action.description}</span>}
              </span>
            </button>
          )
        })}
        {failure && <p {...stylex.props(styles.error)} role="alert">{failure}</p>}
      </div>
    </Sheet>
  )
}

export type ConfirmSheetProps = {
  open: boolean
  onClose: () => void
  title: string
  description: string
  action: { label: string; destructive?: boolean; onSelect: () => void | Promise<void> }
  cancel?: string
  /** `list` (mặc định, artboard MKit) hoặc `grouped` (kiểu iOS của Noverse) */
  layout?: 'list' | 'grouped'
  /** nút chính màu cam (hành động xoá) thay cho nút sáng */
  tone?: 'default' | 'danger'
  children?: ReactNode
}

export function ConfirmSheet({ open, onClose, title, description, action, cancel, layout = 'list', tone = 'default', children }: ConfirmSheetProps) {
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const id = useId()
  const t = getKitLabels()
  const close = () => { if (!pending) onClose() }
  const run = async () => {
    setPending(true)
    setFailure('')
    try {
      await action.onSelect()
      onClose()
    } catch (error) {
      setFailure(message(error, t.confirm.actionFailed))
    } finally {
      setPending(false)
    }
  }
  const primary = tone === 'danger' ? 'danger' : 'primary'

  if (layout === 'grouped') {
    return (
      <Sheet
        open={open}
        onClose={close}
        height="auto"
        grab={false}
        role="alertdialog"
        label={title}
        style={{ background: 'transparent', border: 0, boxShadow: 'none', padding: '0 10px 30px', gap: 10 }}
      >
        <div className="m-confirm-group" aria-describedby={id}>
          <div className="m-confirm-head">
            <span>{title}</span>
            <span id={id}>{description}</span>
            {failure && <span className="m-error" role="alert">{failure}</span>}
          </div>
          <button
            type="button"
            className={`m-confirm-act${action.destructive ? ' m-confirm-danger' : ''}`}
            disabled={pending}
            aria-busy={pending}
            onClick={() => { void run() }}
          >
            {action.label}
          </button>
        </div>
        <div className="m-confirm-group">
          <button type="button" className="m-confirm-act m-confirm-cancel" disabled={pending} onClick={onClose}>{cancel ?? t.cancel}</button>
        </div>
      </Sheet>
    )
  }

  return (
    <Sheet open={open} onClose={close} size="md" grab role="alertdialog" label={title}>
      <div {...stylex.props(styles.confirm)}>
        <span {...stylex.props(styles.confirmTitle)}>{title}</span>
        <span {...stylex.props(styles.confirmText)} id={id}>{description}</span>
        {children}
        {failure && <span {...stylex.props(styles.errorInline)} role="alert">{failure}</span>}
        <div {...stylex.props(styles.confirmButtons)}>
          <Button variant="ghost" size="lg" onClick={onClose} disabled={pending}>{cancel ?? t.cancel}</Button>
          <Button variant={primary} size="lg" pending={pending} onClick={() => { void run() }}>{action.label}</Button>
        </div>
      </div>
    </Sheet>
  )
}
