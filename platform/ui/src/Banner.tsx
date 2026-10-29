import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button } from './Button'
import { Icon } from './Icon'

export type BannerProps = { tone: 'info' | 'success' | 'warning' | 'error'; title: string; children?: ReactNode; action: { label: string; onClick: () => void } }

// Màu + biểu tượng theo Kit.dc.html › “Thông báo trong trang”: info-circle · tick-circle · warning-2 · danger.
// Giá trị là chuỗi var(--kit-*) để truyền cả vào style nội tuyến (Icon) và stylex (khung).
const tones = {
  info: { background: 'var(--kit-info-7)', border: 'var(--kit-info-20)', color: 'var(--kit-info-fg)', icon: 'info-circle' },
  success: { background: 'var(--kit-ok-6)', border: 'var(--kit-ok-20)', color: 'var(--color-ok)', icon: 'tick-circle' },
  warning: { background: 'var(--kit-warn-6)', border: 'var(--kit-warn-20)', color: 'var(--color-warn)', icon: 'warning-2' },
  error: { background: 'var(--kit-err-7)', border: 'var(--kit-errb-22)', color: 'var(--kit-err-bright)', icon: 'danger' },
} as const

const styles = stylex.create({
  root: {
    display: 'flex', gap: 12, alignItems: 'center', paddingTop: 10, paddingBottom: 10,
    paddingInline: 12, borderRadius: 12, borderWidth: 1, borderStyle: 'solid',
  },
  tone: (background: string, border: string) => ({ backgroundColor: background, borderColor: border }),
  body: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  title: { fontSize: 13, fontWeight: 500 },
  text: { fontSize: 12, color: 'var(--color-fg-3)' },
})

export function Banner({ tone, title, children, action }: BannerProps) {
  const look = tones[tone]
  const sx = stylex.props(styles.root, styles.tone(look.background, look.border))
  return <div role={tone === 'error' ? 'alert' : 'status'} className={sx.className} style={sx.style}><Icon name={look.icon} size={17} style={{ color: look.color }} /><span {...stylex.props(styles.body)}><span {...stylex.props(styles.title)}>{title}</span>{children && <span {...stylex.props(styles.text)}>{children}</span>}</span><Button size="sm" onClick={action.onClick}>{action.label}</Button></div>
}
