// Hộp thoại nhỏ của màn Truyện mới: bắt đầu từ tệp (/start).
import { useState } from 'react'
import { Button, Dialog, FieldLabel, Icon, TextField, useSaveShortcut } from '@noverse/ui'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function StartFileDialog({ onClose, onSubmit, busy, pending }: {
  onClose: () => void
  onSubmit: (path: string) => void
  busy: boolean
  pending?: boolean
}) {
  const [path, setPath] = useState('')
  const submit = () => { if (path.trim() && !busy && !/[\u0000-\u001f]/.test(path)) onSubmit(path.trim()) }
  useUnsavedGuard(!!path.trim() && !busy)
  useSaveShortcut(submit, !!path.trim() && !busy)
  return (
    <Dialog padBody={false} title="Bắt đầu từ tệp" icon={<Icon name="document-text" size={18} style={{ color: 'var(--color-accent)' }} />}
      onClose={onClose} width={560}
      footer={<>
        <Button onClick={onClose}>Huỷ</Button>
        <Button variant="primary" onClick={submit} pending={pending} disabled={busy || path.trim().length === 0 || /[\u0000-\u001f]/.test(path)}>
          Tạo truyện & bắt đầu
        </Button>
      </>}>
      <form style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}
        onSubmit={(event) => { event.preventDefault(); submit() }}>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>
          Chọn tệp thiết lập hoặc đại cương đã có trong dự án để AI viết tiếp từ đó.
        </p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="start-file">Tên tệp</FieldLabel>
          <TextField id="start-file" autoFocus value={path} onChange={setPath}
            validate={(value) => /[\u0000-\u001f]/.test(value) ? 'Tên tệp không chứa ký tự điều khiển' : undefined} placeholder="thiet-lap.md" />
        </div>
      </form>
    </Dialog>
  )
}
