import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, type ButtonProps } from './Button'
import { Icon } from './Icon'
import { Spinner } from './feedback'
import type { IconName } from './iconNames'
import { AgentOrb, type AgentKind } from './AgentOrb'
import { agent as agentVars } from './tokens.stylex'

export type BannerProps = {
  /** `agent` = banner do một trợ lý nói (orb + màu trợ lý) — kèm `agent` prop. */
  tone: 'info' | 'success' | 'warning' | 'error' | 'agent'
  /** trợ lý đang nói khi `tone="agent"` (khoá của AgentOrb); mặc định `writer` */
  agent?: AgentKind
  title: string; children?: ReactNode
  /** Nút nhỏ bên phải phần chữ (Kit › “Thông báo trong trang”). */
  action?: { label: string; onClick: () => void; size?: ButtonProps['size'] }
  /** Hàng nút dưới phần chữ khi nhãn dài hoặc cần 2 hành động (Settings › Lưu trữ “Đã chuyển”). */
  actions?: { label: string; onClick: () => void; variant?: ButtonProps['variant']; icon?: ReactNode; size?: ButtonProps['size'] }[]
  /** Đè biểu tượng của tông (vd băng “Tạm chặn” dùng i-clock trên nền cam). */
  icon?: IconName
  iconSize?: number
  titleStyle?: stylex.StyleXStyles
  textStyle?: stylex.StyleXStyles
  iconStyle?: stylex.StyleXStyles
  /** Vai trò a11y của băng; mặc định status, riêng tone="error" là alert. */
  role?: 'status' | 'alert'
  /** `top` = icon căn dòng đầu thay vì giữa khối (băng sự cố trên điện thoại,
   *  artboard MSettings › Tài khoản nền tảng). */
  align?: 'center' | 'top'
  /** Đang chạy: thay icon bằng vòng xoay (băng “Trinh sát đang lấy video đầu tiên”). */
  busy?: boolean
}

// Màu + biểu tượng theo Kit.dc.html › “Thông báo trong trang”: info-circle · tick-circle · warning-2 · danger.
// Giá trị là chuỗi var(--kit-*) để truyền cả vào style nội tuyến (Icon) và stylex (khung).
const tones = {
  info: { background: 'var(--kit-info-7)', border: 'var(--kit-info-20)', color: 'var(--kit-info-fg)', icon: 'info-circle' },
  success: { background: 'var(--kit-ok-6)', border: 'var(--kit-ok-20)', color: 'var(--color-ok)', icon: 'tick-circle' },
  warning: { background: 'var(--kit-warn-6)', border: 'var(--kit-warn-20)', color: 'var(--color-warn)', icon: 'warning-2' },
  error: { background: 'var(--kit-err-7)', border: 'var(--kit-errb-22)', color: 'var(--kit-err-bright)', icon: 'danger' },
} as const

/** tone="agent": nền/viền nhạt theo quầng của trợ lý (token --banner-agent-*), icon = orb của trợ lý đó. */
const AGENT_LOOK: Record<AgentKind, { background: string; border: string; color: string }> = {
  architect: { background: agentVars['--banner-agent-architect-bg'], border: agentVars['--banner-agent-architect-bd'], color: 'var(--color-architect)' },
  writer: { background: agentVars['--banner-agent-writer-bg'], border: agentVars['--banner-agent-writer-bd'], color: 'var(--color-writer)' },
  editor: { background: agentVars['--banner-agent-editor-bg'], border: agentVars['--banner-agent-editor-bd'], color: 'var(--color-editor)' },
  arbiter: { background: agentVars['--banner-agent-arbiter-bg'], border: agentVars['--banner-agent-arbiter-bd'], color: 'var(--color-arbiter)' },
}

const styles = stylex.create({
  root: {
    display: 'flex', gap: 12, alignItems: 'center', paddingTop: 10, paddingBottom: 10,
    paddingInline: 12, borderRadius: 12, borderWidth: 1, borderStyle: 'solid',
  },
  top: { alignItems: 'flex-start' },
  tone: (background: string, border: string) => ({ backgroundColor: background, borderColor: border }),
  body: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  title: { fontSize: 13, fontWeight: 500 },
  actions: { display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' as const, alignItems: 'center' },
  text: { fontSize: 12, color: 'var(--color-fg-3)' },
})

export function Banner({ tone, title, children, action, actions, agent = 'writer', icon, iconSize = 17, titleStyle, textStyle, iconStyle, role, align = 'center', busy = false }: BannerProps) {
  const look = tone === 'agent' ? AGENT_LOOK[agent] : tones[tone]
  const glyph: IconName = icon ?? (tone === 'agent' ? 'info-circle' : tones[tone].icon)
  const ariaRole = role ?? (tone === 'error' ? 'alert' : 'status')
  const sx = stylex.props(styles.root, align === 'top' && styles.top, styles.tone(look.background, look.border))
  const row = actions && actions.length > 0
    ? <span {...stylex.props(styles.actions)}>{actions.map((a) => <Button key={a.label} size={a.size ?? 'sm'} variant={a.variant ?? 'ghost'} icon={a.icon} onClick={a.onClick}>{a.label}</Button>)}</span>
    : null
  return <div role={ariaRole} className={sx.className} style={sx.style}>{busy ? <Spinner size={iconSize} color={look.color} style={{ marginTop: align === 'top' ? 2 : undefined }} /> : tone === 'agent' ? <AgentOrb agent={agent} size={iconSize} /> : <Icon name={glyph} size={iconSize} xstyle={iconStyle} style={{ color: look.color, marginTop: align === 'top' ? 2 : undefined }} />}<span {...stylex.props(styles.body)}><span {...stylex.props(styles.title, titleStyle)}>{title}</span>{children && <span {...stylex.props(styles.text, textStyle)}>{children}</span>}{row}</span>{action && <Button size={action.size ?? 'sm'} onClick={action.onClick}>{action.label}</Button>}</div>
}
