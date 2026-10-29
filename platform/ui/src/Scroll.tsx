// Scroll — overflow-y:auto thanh mảnh; stickToBottom dùng column-reverse (bám đáy).
import { forwardRef, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'

const sx = stylex.create({
  root: { flex: 1, minHeight: 0 },
  stick: { display: 'flex', flexDirection: 'column-reverse' },
})

export const Scroll = forwardRef<HTMLDivElement, {
  children: ReactNode; stickToBottom?: boolean
  style?: CSSProperties; className?: string; label?: string
}>(function Scroll({ children, stickToBottom, style, className, label }, ref) {
  const p = stylex.props(sx.root, stickToBottom && sx.stick)
  return (
    <div ref={ref} className={['scroll', p.className, className].filter(Boolean).join(' ')} aria-label={label}
      style={{ ...p.style, ...style }}>
      {children}
    </div>
  )
})
