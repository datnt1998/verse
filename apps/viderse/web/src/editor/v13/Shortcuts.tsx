import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Sheet, useEscapeLayer } from '@noverse/ui'
import { shortcutLabel, useRegisteredShortcuts, useShortcutAction } from '../ui/shortcuts'
import { sx } from './editor.stylex'
import { useMediaQuery } from '../../lib/useMediaQuery'

// Artboard Editor: “?” mở popover nổi ngay dưới nút, không phải hộp thoại giữa
// màn hình — các điều khiển bên dưới không dịch chỗ. Danh sách vẫn đọc từ
// registry phím tắt đang đăng ký, nên chỉ hiện phím có tác dụng thật.
export function Shortcuts() {
  const [open, setOpen] = useState(false)
  const mobile = useMediaQuery('(max-width: 767px)')
  const root = useRef<HTMLSpanElement | null>(null)
  const trigger = useRef<HTMLButtonElement | null>(null)
  const popover = useRef<HTMLDivElement | null>(null)
  const registered = useRegisteredShortcuts()
  // Cùng một phím được đăng ký ở nhiều vùng (preview + timeline): chỉ giữ một
  // dòng cho mỗi nhãn/phím để danh sách không lặp.
  const listed = registered
    .filter((action, index, all) => action.bindings?.length && all.findIndex((other) => other.label === action.label && other.bindings?.join(',') === action.bindings?.join(',')) === index)
  const close = () => setOpen(false)
  useShortcutAction({ id: 'shortcuts', label: 'Xem phím tắt', scope: 'global', run: () => setOpen(true) })
  useEscapeLayer(open && !mobile, () => { close(); trigger.current?.focus() })
  useEffect(() => {
    if (!open || mobile) return
    popover.current?.focus()
    const outside = (event: PointerEvent) => { if (event.target instanceof Node && !root.current?.contains(event.target)) setOpen(false) }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open, mobile])
  const contents = <>
    <p {...stylex.props(sx.shortcutNote)}>Phím chữ chỉ chạy trong vùng Biên tập. Ô nhập giữ nguyên phím gõ và hoàn tác.</p>
    <ul {...stylex.props(sx.shortcutList)}>
      {listed.map((action) => <li key={`${action.scope}-${action.id}`} {...stylex.props(sx.shortcutRow)}>
        <span {...stylex.props(sx.shortcutKeys)}>{action.bindings?.map((binding) => <kbd key={binding} {...stylex.props(sx.shortcutKey)}>{binding === 'shift+Slash' ? '?' : shortcutLabel(binding)}</kbd>)}</span>
        <span {...stylex.props(sx.shortcutText)}>{action.label}</span>
      </li>)}
    </ul>
  </>
  return <span ref={root} {...stylex.props(sx.shortcutWrap)}
    onBlur={event => { if (!mobile && (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget))) close() }}>
    <Button ref={trigger} size={mobile ? 'lg' : 'sm'} variant="icon" aria-label="Phím tắt" title="Phím tắt (?)" aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen((next) => !next)}>?</Button>
    {mobile ? <Sheet open={open} onClose={close} title="Phím tắt" size="md" height="auto">{contents}</Sheet>
      : open && <div ref={popover} role="dialog" aria-modal="false" aria-label="Phím tắt" tabIndex={-1}
        {...stylex.props(sx.shortcutPop)}>
        <span {...stylex.props(sx.shortcutTitle)}>Phím tắt</span>
        {contents}
      </div>}
  </span>
}
