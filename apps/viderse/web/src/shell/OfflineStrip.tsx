// OfflineStrip — dải cam ngay dưới đầu trang khi mất kết nối máy chủ
// (System artboard · state=offline): nội dung mờ 42%, thao tác ghi tạm khoá;
// hiện trong 5 giây kể từ lúc mất nối, tắt ngay khi nối lại.
// `status` có thể truyền vào để test; mặc định đọc trạng thái luồng SSE.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-offline*).
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { t } from '../i18n/vi'
import { useEventsStatus, events } from '../data/events'
import type { EventsStatus } from '@noverse/ui/data'

/** Dùng lại ở devkit (khung demo dải mất kết nối). */
export const offlineStyles = stylex.create({
  strip: {
    display: 'flex',
    alignItems: 'center',
    gap: 'var(--vv-s-10)',
    padding: 'var(--vv-s-8) var(--vv-s-18)',
    flexWrap: 'wrap',
    backgroundColor: 'var(--vv-err-11)',
    borderBottomWidth: 'var(--vv-s-1)',
    borderBottomStyle: 'solid',
    borderBottomColor: 'var(--vv-err-28)',
    fontSize: 13,
  },
  dot: {
    width: 8,
    height: 8,
    borderRadius: '50%',
    flex: '0 0 auto',
    backgroundColor: 'var(--color-err)',
    boxShadow: '0 0 0 3px var(--vv-err-18)',
  },
  btn: {
    fontSize: 12.5,
    color: { default: 'var(--color-fg-3)', ':hover': 'var(--color-fg)' },
    backgroundColor: 'transparent',
    borderWidth: 0,
    borderStyle: 'none',
    borderColor: 'transparent',
    padding: 0,
  },
  more: { flexBasis: '100%', fontSize: 12, color: 'var(--color-fg-4)' },
})

export function OfflineStrip({ status, retry }: {
  status?: Pick<EventsStatus, 'state' | 'reconnectAt' | 'failures'>
  retry?: () => void
}) {
  const live = useEventsStatus()
  const { reconnectAt, failures } = status ?? live
  const [details, setDetails] = useState(false)
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!reconnectAt) return
    const timer = setInterval(() => setNow(Date.now()), 500)
    return () => clearInterval(timer)
  }, [reconnectAt])
  const nextIn = reconnectAt ? Math.max(0, Math.ceil((reconnectAt - now) / 1000)) : 0

  return (
    <div role="alert" className={`vv-offline ${stylex.props(offlineStyles.strip).className}`}>
      <span aria-hidden="true" className={`vv-offline__dot ${stylex.props(offlineStyles.dot).className}`} />
      <strong style={{ fontWeight: 600 }}>{t.offline.title}</strong>
      {failures > 0 && reconnectAt && (
        <span style={{ color: 'var(--color-fg-4)', fontSize: 12.5 }}>· {t.offline.retryingIn(nextIn)}</span>
      )}
      <span style={{ flex: 1 }} />
      <button type="button" aria-expanded={details} onClick={() => setDetails((v) => !v)} {...stylex.props(offlineStyles.btn)}>
        {t.common.details}
      </button>
      <button type="button" onClick={retry ?? (() => events.retryNow())} {...stylex.props(offlineStyles.btn)}>
        {t.common.retryNow}
      </button>
      {details && <div {...stylex.props(offlineStyles.more)}>{t.offline.body}</div>}
    </div>
  )
}
