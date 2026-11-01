import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Form thêm đối thủ — POST /api/rivals { input }. Lỗi theo `code` hiện ngay
// dưới ô nhập, câu và nút theo 06-copy › Khám phá: exists → Mở đối thủ đó ·
// blocked → Mở Tài khoản nền tảng · unexpected_page và lỗi khác → Thử lại
// (gửi lại đúng ô đang nhập). Dùng ở hai nơi: ô gọn đầu cột đối thủ và form
// lớn ở trạng thái trống.
import { useState, type FormEvent } from 'react'
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { addRival, apiErrorInfo, type Rival } from '../../api/rivals'
import { keys } from '../../data/keys'
import { t } from '../../i18n/explore'
import { paths } from '../../router'
import { fill } from '../settings/fmt'
import { ActionButton } from '../../components'

const fmtRetry = (iso: string): string => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

/** Việc tiếp theo đi kèm câu lỗi: mở đối thủ đã có, mở Tài khoản nền tảng, hoặc gửi lại. */
export type AddErrorAction = 'open' | 'accounts' | 'retry'
export type AddError = { code: string; message: string; action?: AddErrorAction; rivalId?: string }

/** `code` của POST /api/rivals → câu lỗi dưới ô (copy screens.explore.addErrors). */
export function addRivalError(err: unknown): AddError {
  const tt = t.screens.explore.addErrors
  const info = apiErrorInfo(err)
  switch (info.code) {
    case 'invalid_input': return { code: info.code, message: tt.invalid_input }
    case 'not_found': return { code: info.code, message: tt.not_found }
    case 'exists': return { code: info.code, message: tt.exists, action: info.rival ? 'open' : undefined, rivalId: info.rival?.id }
    case 'rate_limited': {
      const at = info.retryAt ? fmtRetry(info.retryAt) : ''
      return { code: info.code, message: at ? fill(tt.rate_limited, { time: at }) : tt.rateLimitedNoTime }
    }
    case 'blocked': return { code: info.code, message: tt.blocked, action: 'accounts' }
    case 'unexpected_page': return { code: info.code, message: tt.unexpected_page, action: 'retry' }
    default: return { code: info.code ?? 'failed', message: tt.failed, action: 'retry' }
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
    <span className={`${compact ? 'vx-add-err' : 'vx-empty__err'} ${stylex.props(compact ? exploreStyles.addErr : exploreStyles.emptyErr).className}`} role="alert">
      {error.message}
      {error.action === 'open' && error.rivalId && (
        <a {...stylex.props(exploreStyles.errorLink)} href="#" onClick={(e) => { e.preventDefault(); error.rivalId && onOpen(error.rivalId) }}>{tt.addErrors.existsOpen}</a>
      )}
      {error.action === 'accounts' && <a {...stylex.props(exploreStyles.errorLink)} href={`#${paths.settings('accounts')}`}>{tt.addErrors.blockedOpen}</a>}
      {error.action === 'retry' && <button type="submit" className={`vx-errbtn ${stylex.props(exploreStyles.errbtn).className}`}>{tt.addErrors.retry}</button>}
    </span>
  )

  if (compact) {
    return (
      <form aria-label={tt.addRival} onSubmit={submit} className={`vx-add ${stylex.props(exploreStyles.add).className}`} style={{ flexDirection: 'column', alignItems: 'stretch', gap: 'var(--vv-s-6)' }}>
        <span className={`vx-addbox${error ? ' is-error' : ''} ${stylex.props(exploreStyles.addbox, error && exploreStyles.addboxError).className}`}>
          <Icon {...stylex.props(exploreStyles.addIcon)} name="add" size={15} />
          <input {...stylex.props(exploreStyles.addInput, !compact && exploreStyles.emptyInput)}
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
    <form aria-label={tt.addRival} onSubmit={submit} className={`vx-empty__form ${stylex.props(exploreStyles.emptyForm).className}`} style={{ flexDirection: 'column', gap: 'var(--vv-s-8)' }}>
      <span style={{ display: 'flex', gap: 'var(--vv-s-8)', width: '100%' }}>
        <span className={`vx-addbox${error ? ' is-error' : ''} ${stylex.props(exploreStyles.addbox, error && exploreStyles.addboxError).className}`} style={{ height: 'var(--vv-s-36)' }}>
          <Icon {...stylex.props(exploreStyles.addIcon)} name="add" size={15} />
          <input {...stylex.props(exploreStyles.addInput, !compact && exploreStyles.emptyInput)}
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
        <ActionButton className={stylex.props(exploreStyles.touchTarget).className} variant="primary" type="submit" disabled={pending || !value.trim()} pending={pending}
          style={{ flex: '0 0 auto' }}>
          {pending ? tt.addRivalPending : tt.addRival}
        </ActionButton>
      </span>
      {err}
    </form>
  )
}
