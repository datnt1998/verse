// OfflineStrip — dải cam ngay dưới đầu trang khi mất kết nối máy chủ
// (System artboard · state=offline): nội dung mờ 42%, thao tác ghi tạm khoá;
// hiện trong 5 giây kể từ lúc mất nối, tắt ngay khi nối lại.
// `status` có thể truyền vào để test; mặc định đọc trạng thái luồng SSE.
// Kiểu: styles/components.css .vv-offline*.
import { useEffect, useState } from 'react'
import { t } from '../i18n/vi'
import { useEventsStatus, events } from '../data/events'
import type { EventsStatus } from '@noverse/ui/data'

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
    <div role="alert" className="vv-offline">
      <span aria-hidden="true" className="vv-offline__dot" />
      <strong style={{ fontWeight: 600 }}>{t.offline.title}</strong>
      {failures > 0 && reconnectAt && (
        <span style={{ color: 'var(--color-fg-4)', fontSize: 12.5 }}>· {t.offline.retryingIn(nextIn)}</span>
      )}
      <span style={{ flex: 1 }} />
      <button type="button" aria-expanded={details} onClick={() => setDetails((v) => !v)} className="vv-offline__btn">
        {t.common.details}
      </button>
      <button type="button" onClick={retry ?? (() => events.retryNow())} className="vv-offline__btn">
        {t.common.retryNow}
      </button>
      {details && <div className="vv-offline__more">{t.offline.body}</div>}
    </div>
  )
}
