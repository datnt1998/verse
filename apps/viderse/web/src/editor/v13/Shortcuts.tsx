import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Dialog } from '@noverse/ui'
import { shortcutLabel, useRegisteredShortcuts, useShortcutAction } from '../ui/shortcuts'
import { sx } from './editor.stylex'

export function Shortcuts() {
  const [open, setOpen] = useState(false)
  const registered = useRegisteredShortcuts()
  useShortcutAction({ id: 'shortcuts', label: 'Xem phím tắt', scope: 'global', run: () => setOpen(true) })
  return <><button type="button" aria-label="Phím tắt" title="Phím tắt (?)" {...stylex.props(sx.button, sx.compact, sx.quiet, sx.iconButton)} onClick={() => setOpen(true)}>?</button>{open && <Dialog title="Phím tắt" width={560} onClose={() => setOpen(false)} footer={<button type="button" {...stylex.props(sx.button)} onClick={() => setOpen(false)}>Đóng</button>}><div {...stylex.props(sx.dialogBody)}><p {...stylex.props(sx.dialogCopy)}>Phím chữ chỉ chạy trong vùng editor. Ô nhập giữ nguyên phím gõ và hoàn tác.</p><dl {...stylex.props(sx.stack)}>{registered.filter((action) => action.bindings?.length).map((action) => <div key={`${action.scope}-${action.id}`} {...stylex.props(sx.row)}><dt {...stylex.props(sx.spread)}>{action.label}</dt><dd>{action.bindings?.map((binding) => <kbd key={binding}>{binding === 'shift+Slash' ? '?' : shortcutLabel(binding)}</kbd>)}</dd></div>)}</dl></div></Dialog>}</>
}
