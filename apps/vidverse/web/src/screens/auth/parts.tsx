// Mảnh dùng chung của màn Đăng nhập / Thiết lập: khung trang giữa (AuthShell)
// và dòng thông tin nhỏ (HintLine) — tương đương screens/auth/parts.tsx của Noverse.
import type { CSSProperties, ReactNode } from 'react'
import { t } from '../../i18n/vi'
import { useSession } from '../../api/session'
import { BrandLockup, Icon } from '@noverse/ui'

/** Khung trang auth: brand trên cùng · nội dung căn giữa · chip máy chủ. Không có nav.
 *  Chip theo lần kiểm phiên gần nhất: chỉ báo "đang hoạt động" khi máy chủ đã trả lời. */
export function AuthShell({ children }: { children: ReactNode }) {
  const { serverReachable } = useSession()
  const chip = serverReachable ? t.auth.serverOnline : t.auth.serverOffline
  return (
    <div style={{ minHeight: '100vh', height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)', color: 'var(--color-fg)' }}>
      <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '14px 20px' }}>
        <BrandLockup size={22} studio />
        <div role="status" title={chip} style={{
          display: 'flex', alignItems: 'center', gap: 8, height: 30, padding: '0 12px', borderRadius: 999,
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
      <main className="scroll" style={{ flex: 1, minHeight: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div style={{ width: 'min(400px, 100%)', display: 'flex', flexDirection: 'column', gap: 22 }}>{children}</div>
      </main>
    </div>
  )
}

/** Dòng gợi ý nhỏ có icon ở footer màn auth. */
export function HintLine({ icon, children, style }: { icon: string; children: ReactNode; style?: CSSProperties }) {
  return (
    <span style={{ display: 'flex', alignItems: 'flex-start', gap: 8, color: 'var(--color-fg-3)', fontSize: 12, ...style }}>
      <Icon name={icon} size={14} style={{ color: 'var(--color-fg-4)', marginTop: 1, flex: '0 0 auto' }} />
      <span>{children}</span>
    </span>
  )
}
