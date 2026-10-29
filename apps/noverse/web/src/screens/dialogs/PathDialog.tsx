// Hộp thoại nhập một đường dẫn — /start-file và /importsim (cùng hình thức, khác API).
import { useState } from 'react'
import { Button } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { FieldLabel } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function PathDialog({ title, label, placeholder, hint, submitLabel, onSubmit, onClose, busy, disabled }: {
  title: string
  label: string
  placeholder: string
  /** dòng mono nhắc cú pháp lệnh */
  hint: string
  submitLabel: string
  onSubmit: (path: string) => void
  onClose: () => void
  busy: boolean
  disabled?: boolean
}) {
  const [path, setPath] = useState('')
  const submit = () => { if (path.trim() && !disabled && !busy && !/[\u0000-\u001f]/.test(path)) onSubmit(path.trim()) }
  useUnsavedGuard(!!path.trim() && !disabled && !busy)
  useSaveShortcut(submit, !!path.trim() && !disabled && !busy)
  return (
    <Dialog title={title} width={560} onClose={onClose}
      icon={<Icon name="document-text" size={18} style={{ color: 'var(--color-accent)' }} />}
      footer={<>
        <Button onClick={onClose}>Huỷ</Button>
        <Button variant="primary" onClick={submit} pending={busy} disabled={disabled || busy || path.trim().length === 0 || /[\u0000-\u001f]/.test(path)}>
          {submitLabel}
        </Button>
      </>}>
      <form style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}
        onSubmit={(event) => { event.preventDefault(); submit() }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="path-dialog">{label}</FieldLabel>
          {/* Ô nhập là tham số lệnh (/start-file, /importsim): ví dụ đường dẫn theo artboard Commands là cú pháp lệnh, được phép. */}
          <div data-tech>
            <TextField id="path-dialog" className="mono" autoFocus value={path}
              onChange={setPath} validate={(value) => /[\u0000-\u001f]/.test(value) ? 'Đường dẫn không chứa ký tự điều khiển' : undefined} placeholder={placeholder} />
          </div>
        </div>
        <span className="mono" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{hint}</span>
      </form>
    </Dialog>
  )
}
