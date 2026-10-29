// Mảnh dùng chung của hộp thoại tài khoản (A1 › qr/devices).
// SessionList dùng ở QR và Thiết bị; quản trị thành viên ở Tuỳ chỉnh (F5b).
import type { ReactNode } from 'react'
import type { Session } from '../../api/types'
import { Icon } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { fmtDate, fmtShort, relativeTime } from '../auth/parts'
import { ContextMenu } from '@noverse/ui'

export const sessionIcon = (device: string) =>
  /iPhone|iPad|Android|Mobile/i.test(device) ? 'mobile' : 'monitor'

/** “đến 23/11” hoặc “hết hạn sau 2 ngày” (cam khi ≤2 ngày). */
export function expiryLabel(expiresAt: string): { text: string; warn: boolean } {
  const ms = new Date(expiresAt).getTime() - Date.now()
  if (Number.isNaN(ms)) return { text: '—', warn: false }
  const days = Math.ceil(ms / 86_400_000)
  if (days <= 0) return { text: 'hết hạn hôm nay', warn: true }
  if (days <= 2) return { text: `hết hạn sau ${days} ngày`, warn: true }
  return { text: `đến ${fmtShort(expiresAt)}`, warn: false }
}

export function SessionList({ sessions, loading, error, variant = 'compact', freshId, onRevoke }: {
  sessions: Session[]
  loading?: boolean
  error?: string
  variant?: 'compact' | 'full'
  freshId?: string | null
  onRevoke?: (session: Session) => void
}) {
  if (loading && sessions.length === 0) {
    return <div aria-label="Đang tải thiết bị" style={{ display: 'grid', gap: 12, padding: 12 }}>{[0, 1, 2].map((id) => <Skeleton key={id} height={48} />)}</div>
  }
  if (error) {
    return (
      <div role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, padding: 12, fontSize: 12.5, color: 'var(--color-err)' }}>
        <Icon name="warning-2" size={14} />{error}
      </div>
    )
  }
  if (sessions.length === 0) {
    return <EmptyState icon="devices" title="Chưa có thiết bị nào đăng nhập" />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {sessions.map((session, index) => {
        const expiry = expiryLabel(session.expiresAt)
        const fresh = freshId === session.id
        if (variant === 'full') {
          return (
            <ContextMenu key={session.id} items={onRevoke && !session.current ? [{ id: 'revoke', label: 'Đăng xuất thiết bị', icon: <Icon name="logout" size={15} />, danger: true, onSelect: () => onRevoke(session) }] : []} style={{
              display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px',
              borderTop: index ? '1px solid var(--line-1)' : 'none',
            }}>
              <span aria-hidden="true" style={{
                width: 36, height: 36, borderRadius: 10, background: 'var(--color-surface-5)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-fg-2)', flex: '0 0 auto',
              }}><Icon name={sessionIcon(session.device)} size={18} /></span>
              <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                <span style={{ fontSize: 13.5, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}>
                  {session.device}
                  {session.current && <Tag tone="ok">Thiết bị này</Tag>}
                </span>
                <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
                  {relativeTime(session.lastSeen)} · đăng nhập {fmtShort(session.createdAt)}
                </span>
              </span>
              <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 2 }}>
                <span className="num" style={{ fontSize: 12, color: expiry.warn ? 'var(--color-warn)' : 'var(--color-fg-3)' }}>{expiry.text}</span>
                {onRevoke && !session.current && (
                  <button type="button" className="hov-ghost" onClick={() => onRevoke(session)}
                    aria-label={`Đăng xuất ${session.device}`}
                    style={{
                      height: 26, padding: '0 8px', borderRadius: 7, border: '1px solid var(--line-3)',
                      background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12,
                    }}>Đăng xuất</button>
                )}
              </span>
            </ContextMenu>
          )
        }
        return (
          <div key={session.id} style={{
            display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px', borderRadius: 10,
            background: fresh ? 'var(--tint-accent)' : 'transparent',
          }}>
            <Icon name={sessionIcon(session.device)} size={17} style={{ color: 'var(--color-fg-3)' }} />
            <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 13, display: 'flex', alignItems: 'center', gap: 6 }}>
                {session.device}
                {session.current && <Tag tone="ok">Thiết bị này</Tag>}
                {fresh && <Tag tone="accent">Vừa đăng nhập</Tag>}
              </span>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{relativeTime(session.lastSeen)}</span>
            </span>
            <span className="num" style={{ fontSize: 11.5, color: expiry.warn ? 'var(--color-warn)' : 'var(--color-fg-4)' }}>{expiry.text}</span>
          </div>
        )
      })}
    </div>
  )
}

export function Tag({ tone, children }: { tone: 'ok' | 'accent' | 'neutral'; children: ReactNode }) {
  const style = tone === 'ok'
    ? { background: 'var(--tint-ok)', color: 'var(--color-ok)' }
    : tone === 'accent'
      ? { background: 'var(--tint-accent)', color: 'var(--color-accent)' }
      : { background: 'rgba(255,255,255,0.06)', color: 'var(--color-fg-3)' }
  return (
    <span style={{
      height: 18, padding: '0 6px', display: 'inline-flex', alignItems: 'center', borderRadius: 5,
      fontSize: 10.5, fontWeight: 400, whiteSpace: 'nowrap', ...style,
    }}>{children}</span>
  )
}

/** Ngày tạo tài khoản kiểu “tạo 02/10/2026”. */
export const createdLabel = (value: string | null | undefined) => value ? fmtDate(new Date(value)) : '—'
