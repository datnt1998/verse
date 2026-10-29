// Dialogs.dc.html › state=recovery — “Bạn đang có một mã khôi phục chưa dùng”.
// Mở từ Tuỳ chỉnh › Truy cập & khôi phục › Tạo mã mới khi máy chủ trả 409 (đã có mã chưa dùng).
// Hai lối ra: “Giữ mã cũ” (đóng) và “Huỷ mã cũ, tạo mã mới” (nút cam → POST …/recovery/rotate).
import { useRef } from 'react'
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'

/** Ngày tạo mã theo kiểu “24/10” như artboard (giờ địa phương). */
function createdDay(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getDate()}/${date.getMonth() + 1}`
}

export function RecoveryConflictDialog({ createdAt, mask, pending = false, onKeep, onRotate }: {
  /** `createdAt` của mã đang có (GET /api/me/recovery) */
  createdAt: string
  /** mã che giữa do máy chủ trả, dạng `TRAX-••••-••••-41ZN`; thiếu thì che cả mã */
  mask?: string
  pending?: boolean
  onKeep: () => void
  onRotate: () => void
}) {
  const keep = useRef<HTMLButtonElement>(null)
  useEscapeLayer(true, onKeep)
  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 40 }}>
      <div aria-hidden="true" onClick={onKeep} style={{ position: 'absolute', inset: 0, background: 'var(--kit-scrim)' }} />
      <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div className="dlg" role="alertdialog" aria-modal="true" aria-labelledby="rc-conflict-t" aria-describedby="rc-conflict-d"
          style={{ width: 'min(480px, 100%)', padding: 22, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Icon name="key" size={26} style={{ color: 'var(--color-warn)' }} />
          <h2 id="rc-conflict-t" style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>{t.dialogs.recoveryConflict.title}</h2>
          <p id="rc-conflict-d" style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
            {t.dialogs.recoveryConflict.body(createdDay(createdAt))}
          </p>
          <div style={{
            display: 'flex', gap: 10, alignItems: 'center', padding: '10px 12px', borderRadius: 12,
            background: 'var(--color-bg)', border: '1px solid var(--line-1)', fontSize: 13,
          }}>
            <Icon name="tick-circle-bulk" size={15} style={{ color: 'var(--color-ok)', flex: '0 0 auto' }} />
            <span style={{ flex: 1 }}>{t.dialogs.recoveryConflict.current}</span>
            <span className="mono" style={{ color: 'var(--color-fg-3)', letterSpacing: '.08em' }}>
              {mask || t.dialogs.recoveryConflict.fullyMasked}
            </span>
          </div>
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button ref={keep} type="button" className="hov-ghost" onClick={onKeep}
              style={{ height: 36, padding: '0 14px', borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
              {t.dialogs.recoveryConflict.keep}
            </button>
            <button type="button" className="hov-primary" onClick={onRotate} disabled={pending} aria-busy={pending}
              style={{ height: 36, padding: '0 14px', borderRadius: 9, border: 'none', background: 'var(--color-warn)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600, opacity: pending ? 0.7 : 1 }}>
              {t.dialogs.recoveryConflict.rotate}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
