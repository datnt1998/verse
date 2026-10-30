// Form thêm đối thủ — POST /api/rivals { input }. Lỗi theo `code` hiện ngay
// dưới ô nhập (07-data-contract §5): invalid_input · not_found · exists (kèm
// link Mở đối thủ đó) · rate_limited (kèm giờ thử lại) · blocked ·
// unexpected_page. Dùng ở hai nơi: ô gọn đầu cột đối thủ và form lớn ở trạng
// thái trống.
import { useState, type FormEvent } from 'react'
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { addRival, apiErrorInfo, type Rival } from '../../api/rivals'
import { keys } from '../../data/keys'
import { vi as t } from '../../i18n/vi'
import { fill } from '../settings/fmt'
import { ActionButton } from '../../components'

const fmtRetry = (iso: string): string => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

export type AddError = { code: string; message: string; rivalId?: string }

/** `code` của POST /api/rivals → câu lỗi dưới ô (copy screens.explore.addErrors). */
export function addRivalError(err: unknown): AddError {
  const tt = t.screens.explore.addErrors
  const info = apiErrorInfo(err)
  const message = err instanceof Error ? err.message : tt.failed
  const capital = message.charAt(0).toLocaleUpperCase('vi') + message.slice(1)
  switch (info.code) {
    case 'invalid_input': return { code: info.code, message: tt.invalid_input }
    case 'not_found': return { code: info.code, message: tt.not_found }
    case 'exists': return { code: info.code, message: tt.exists, rivalId: info.rival?.id }
    case 'rate_limited': {
      const at = info.retryAt ? fmtRetry(info.retryAt) : ''
      return { code: info.code, message: at ? fill(tt.rate_limited, { time: at }) : tt.rate_limited.replace(' {time}', '') }
    }
    case 'blocked': return { code: info.code, message: info.recovery ?? tt.blocked }
    case 'unexpected_page': return { code: info.code, message: info.recovery ?? tt.unexpected_page }
    default: return { code: info.code ?? 'failed', message: capital || tt.failed }
  }
}

export function AddRivalForm({ compact = false, onAdded, onOpen }: {
  /** ô gọn trong cột đối thủ (không có nút submit riêng — Enter thêm) */
  compact?: boolean
  onAdded: (rival: Rival) => void
  /** exists → chọn đối thủ đã có sẵn */
  onOpen: (id: string) => void
}) {
  const tt = t.screens.explore
  const [value, setValue] = useState('')
  const [error, setError] = useState<AddError | null>(null)
  const [pending, setPending] = useState(false)

  const submit = (event: FormEvent) => {
    event.preventDefault()
    const input = value.trim()
    if (!input || pending) return
    setPending(true)
    setError(null)
    addRival(input)
      .then((rival) => {
        setValue('')
        void invalidate(keys.rivals())
        onAdded(rival)
      })
      .catch((err) => setError(addRivalError(err)))
      .finally(() => setPending(false))
  }

  const err = error && (
    <span className={compact ? 'vx-add-err' : 'vx-empty__err'} role="alert">
      {error.message}
      {error.rivalId && (
        <a href="#" onClick={(e) => { e.preventDefault(); error.rivalId && onOpen(error.rivalId) }}>{tt.addErrors.existsOpen}</a>
      )}
    </span>
  )

  if (compact) {
    return (
      <form aria-label={tt.addRival} onSubmit={submit} className="vx-add" style={{ flexDirection: 'column', alignItems: 'stretch', gap: 'var(--vv-s-6)' }}>
        <span className={`vx-addbox${error ? ' is-error' : ''}`}>
          <Icon name="add" size={15} />
          <input
            type="text"
            value={value}
            onChange={(e) => { setValue(e.target.value); setError(null) }}
            placeholder={tt.addRivalCompact}
            aria-label={tt.addRivalAria}
            aria-invalid={error ? true : undefined}
            disabled={pending}
          />
        </span>
        {err}
      </form>
    )
  }
  return (
    <form aria-label={tt.addRival} onSubmit={submit} className="vx-empty__form" style={{ flexDirection: 'column', gap: 'var(--vv-s-8)' }}>
      <span style={{ display: 'flex', gap: 'var(--vv-s-8)', width: '100%' }}>
        <span className={`vx-addbox${error ? ' is-error' : ''}`} style={{ height: 'var(--vv-s-36)' }}>
          <Icon name="add" size={15} />
          <input
            type="text"
            value={value}
            onChange={(e) => { setValue(e.target.value); setError(null) }}
            placeholder={tt.addRivalPlaceholder}
            aria-label={tt.addRivalAria}
            aria-invalid={error ? true : undefined}
            disabled={pending}
            autoFocus
          />
        </span>
        <ActionButton variant="primary" type="submit" disabled={pending || !value.trim()} pending={pending}
          style={{ flex: '0 0 auto' }}>
          {pending ? tt.addRivalPending : tt.addRival}
        </ActionButton>
      </span>
      {err}
    </form>
  )
}
