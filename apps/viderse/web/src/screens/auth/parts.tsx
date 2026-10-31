// Mảnh dùng chung của màn Đăng nhập / Thiết lập: khung trang giữa (AuthShell)
// và dòng thông tin nhỏ (HintLine) — tương đương screens/auth/parts.tsx của Noverse.
import type { CSSProperties, ReactNode } from 'react'
import { t } from '../../i18n/vi'
import { useSession } from '../../api/session'
import { Icon } from '@noverse/ui'
import { LensLogo } from '../../components/LensLogo'
import * as stylex from '@stylexjs/stylex'
import { authStyles as sx } from './auth.stylex'
import { Avatar } from '@noverse/ui'
import type { User } from '../../api/types'
import { useMediaQuery } from '../../lib/useMediaQuery'

/** Khung trang auth: brand trên cùng · nội dung căn giữa · chip máy chủ. Không có nav.
 *  Chip theo lần kiểm phiên gần nhất: chỉ báo "đang hoạt động" khi máy chủ đã trả lời. */
export function AuthShell({ children, footer, centered = false }: { children: ReactNode; footer?: 'login' | 'setup'; centered?: boolean }) {
  const { serverReachable, outage } = useSession()
  const mobile = useMediaQuery('(max-width: 767px)')
  const chip = serverReachable ? (mobile ? t.auth.serverOnlineMobile : t.auth.serverOnline) : outage === 'accounts' ? t.auth.accountsDownTitle : t.auth.serverOffline
  return (
    <div {...stylex.props(sx.shell)}>
      <header {...stylex.props(sx.header)}>
        {/* Lockup Viderse (12-brand): ống kính + chữ "Viderse" — không dùng BrandLockup của kit (orb Noverse). */}
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontWeight: 700, fontSize: 14, letterSpacing: '-0.02em' }}>
          <LensLogo size={24} />
          {t.brand.name}
        </span>
        <div role="status" title={chip} style={{
          display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', height: 30, padding: '0 var(--vv-s-12)', borderRadius: 'var(--vv-r-pill)',
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--color-fg-3)',
          whiteSpace: 'nowrap',
        }}>
          <span aria-hidden="true" style={{
            width: 7, height: 7, borderRadius: '50%', flex: '0 0 auto',
            background: serverReachable ? 'var(--color-ok)' : 'var(--color-err)',
            boxShadow: `0 0 0 3px ${serverReachable ? 'var(--tint-ok)' : 'var(--tint-err)'}`,
          }} />
          {chip}
        </div>
      </header>
      <main {...stylex.props(sx.main, centered && sx.centered)}>
        <div {...stylex.props(sx.content)}>{children}</div>
      </main>
      {footer && <footer {...stylex.props(sx.footer)}>
        {footer === 'login' ? (mobile ? <span>{t.auth.footerMobile}</span> : <><span>{t.auth.footerForgot}</span><span>{t.auth.footerNew}</span></>) : <span>{t.setup.codeHint}</span>}
      </footer>}
    </div>
  )
}

/** Dòng gợi ý nhỏ có icon ở footer màn auth. */
export function HintLine({ icon, children, style }: { icon: string; children: ReactNode; style?: CSSProperties }) {
  return (
    <span style={{ display: 'flex', alignItems: 'flex-start', gap: 'var(--vv-s-8)', color: 'var(--color-fg-3)', fontSize: 12, ...style }}>
      <Icon name={icon} size={14} style={{ color: 'var(--color-fg-4)', marginTop: 'var(--vv-s-1)', flex: '0 0 auto' }} />
      <span>{children}</span>
    </span>
  )
}

export function AuthUser({ user }: { user: User }) {
  return <div className={`card ${stylex.props(sx.row).className}`}>
    <Avatar name={user.name || user.username} size={38} />
    <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
      <span style={{ fontSize: 14, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis' }} title={user.name}>{user.name || user.username}</span>
      <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}><span className="mono">@{user.username}</span> · {t.role[user.role]}</span>
    </span>
  </div>
}

export const authLabelStyle: CSSProperties = { textTransform: 'none', fontWeight: 400, letterSpacing: 0, color: 'var(--color-fg-3)' }
