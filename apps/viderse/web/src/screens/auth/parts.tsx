// Mảnh dùng chung của màn Đăng nhập / Thiết lập: khung trang giữa (AuthShell)
// và dòng thông tin nhỏ (HintLine) — tương đương screens/auth/parts.tsx của Noverse.
import type { ComponentProps, CSSProperties, ReactNode } from 'react'
import { t } from '../../i18n/auth'
import { useSession } from '../../api/session'
import { FieldError, Icon, PasswordField } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import * as stylex from '@stylexjs/stylex'
import { authStyles as sx } from './auth.stylex'
import { Avatar } from '@noverse/ui'
import type { User } from '../../api/types'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { box } from '../../styles/shared.stylex'

/** Khung trang auth: brand trên cùng · nội dung căn giữa · chip máy chủ. Không có nav.
 *  Chip theo lần kiểm phiên gần nhất: chỉ báo "đang hoạt động" khi máy chủ đã trả lời. */
export function AuthShell({ children, footer, centered = false, fill = false, offline = false }: { children: ReactNode; footer?: 'login' | 'setup'; centered?: boolean; fill?: boolean; offline?: boolean }) {
  const { serverReachable, outage } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const reachable = serverReachable && !offline
  const chip = reachable ? (mobile ? t.auth.serverOnlineMobile : t.auth.serverOnline) : mobile ? t.auth.serverOfflineMobile : outage === 'accounts' && !offline ? t.auth.accountsDownChip : t.auth.serverOffline
  return (
    <div {...stylex.props(sx.shell)}>
      <header {...stylex.props(sx.header)}>
        {/* Lockup Viderse (12-brand): ống kính + chữ "Viderse" — không dùng BrandLockup của kit (orb Noverse). */}
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontWeight: 700, fontSize: mobile ? 15 : 14, letterSpacing: mobile ? 'normal' : '-0.02em' }}>
          <LensLogo size={mobile ? 22 : 24} />
          {t.brand.name}
        </span>
        <div role="status" title={chip} style={{
          display: 'flex', alignItems: 'center', gap: mobile ? 7 : 'var(--vv-s-8)', height: mobile ? 28 : 30, padding: mobile ? '0 10px' : '0 var(--vv-s-12)', borderRadius: 'var(--vv-r-pill)',
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--color-fg-3)',
          whiteSpace: 'nowrap',
        }}>
          <span aria-hidden="true" style={{
            width: 7, height: 7, borderRadius: '50%', flex: '0 0 auto',
            background: reachable ? 'var(--color-ok)' : 'var(--color-err)',
            boxShadow: `0 0 0 3px ${reachable ? 'var(--tint-ok)' : 'var(--vv-err-18)'}`,
          }} />
          {chip}
        </div>
      </header>
      <main {...stylex.props(sx.main, centered && sx.centered, fill && sx.mainFilled)}>
        <div {...stylex.props(sx.content, fill && sx.filled)}>{children}</div>
      </main>
      {footer && <footer {...stylex.props(sx.footer)}>
        {footer === 'login' ? (mobile ? <span>{t.auth.footerMobilePrompt}<br />{t.auth.footerMobileHelp}</span> : <><span>{t.auth.footerForgot}</span><span aria-hidden="true">·</span><span>{t.auth.footerNew}</span></>) : <><span>{t.setup.footerShared}</span><span aria-hidden="true">·</span><span>{t.setup.footerLogin}</span></>}
      </footer>}
    </div>
  )
}

/** Dòng gợi ý nhỏ có icon ở footer màn auth. */
export function HintLine({ icon, children, style }: { icon: string; children: ReactNode; style?: CSSProperties }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  return (
    <span {...stylex.props(sx.hint)} style={style}>
      <Icon name={icon} size={mobile ? 15 : 14} style={{ marginTop: 'var(--vv-s-2)', flex: '0 0 auto', color: 'var(--kit-fg-sub)' }} />
      <span>{children}</span>
    </span>
  )
}

export function AuthUser({ user }: { user: User }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  return <div {...stylex.props(sx.row, box.card)}>
    <Avatar name={user.name || user.username} size={mobile ? 40 : 38} style={{ fontSize: mobile ? 15 : 14, border: '1px solid var(--kit-wash-10)' }} />
    <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
      <span style={{ fontSize: mobile ? 15 : 14, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis' }} title={user.name}>{user.name || user.username}</span>
      <span style={{ fontSize: mobile ? 12.5 : 12, color: mobile ? 'var(--vv-fg-note)' : 'var(--color-fg-3)' }}><span>@{user.username}</span> · {t.role[user.role]}</span>
    </span>
  </div>
}

export function AuthLabel({ htmlFor, children }: { htmlFor: string; children: ReactNode }) {
  return <label htmlFor={htmlFor} {...stylex.props(sx.label)}>{children}</label>
}

export function AuthError({ children, id }: { children: ReactNode; id?: string }) {
  return <FieldError id={id} showIcon={false} style={{ fontSize: 11.5, lineHeight: 1.5, color: 'var(--vv-warn-ink)' }}>{children}</FieldError>
}

export function AuthPassword(props: ComponentProps<typeof PasswordField>) {
  const mobile = useMediaQuery('(max-width: 767px)')
  return <PasswordField {...props} {...stylex.props(sx.input, sx.password)}
    frameStyle={{ height: mobile ? 48 : 40, paddingLeft: mobile ? 14 : 12, borderRadius: 'var(--vv-r-10)', fontSize: 14 }}
    toggleIcon={<Icon name="eye" size={mobile ? 17 : 15} />}
    toggleStyle={{ position: 'static', flexShrink: 0, width: mobile ? 40 : 30, height: mobile ? 40 : 30, borderRadius: mobile ? 12 : 8, fontSize: mobile ? 14 : 12, fontWeight: mobile ? 400 : 500, lineHeight: mobile ? '21px' : '18.6px', gap: mobile ? undefined : 7, border: mobile ? 'none' : '1px solid transparent', padding: mobile ? '1px 6px' : 0, color: mobile ? 'var(--color-fg-2)' : 'var(--color-fg-3)' }} />
}
