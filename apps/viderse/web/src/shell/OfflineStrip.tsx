// OfflineStrip — dải cam ngay dưới đầu trang khi mất kết nối máy chủ
// (System artboard · state=offline): nội dung mờ 42%, thao tác ghi tạm khoá;
// hiện trong 5 giây kể từ lúc mất nối, tắt ngay khi nối lại.
// `status` có thể truyền vào để test; mặc định đọc trạng thái luồng SSE.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-offline*).
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon } from '@noverse/ui'
import { t } from '../i18n/vi'
import { useEventsStatus, events } from '../data/events'
import type { EventsStatus } from '@noverse/ui/data'

/** Dùng lại ở devkit (khung demo dải mất kết nối). */
export const offlineStyles = stylex.create({
  strip: {
    display: 'flex',
    alignItems: 'center',
    gap: 'var(--vv-s-10)',
    padding: 'var(--vv-s-10) var(--vv-s-12)',
    flexWrap: 'wrap',
    borderRadius: 'var(--vv-r-12)',
    borderWidth: 'var(--vv-s-1)',
    borderStyle: 'solid',
    borderColor: 'var(--vv-err-28)',
    backgroundColor: 'var(--vv-warn-bg)',
    fontSize: 13,
    color: 'var(--color-fg-3)',
  },
  icon: { color: 'var(--vv-danger)', flex: '0 0 auto' },
  banner: { margin: 'var(--vv-s-10) var(--vv-s-16) 0' },
  body: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  title: { fontSize: 13, fontWeight: 500, color: 'var(--color-fg)' },
  text: { fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.55 },
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
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!reconnectAt) return
    const timer = setInterval(() => setNow(Date.now()), 500)
    return () => clearInterval(timer)
  }, [reconnectAt])
  const nextIn = reconnectAt ? Math.max(0, Math.ceil((reconnectAt - now) / 1000)) : 0

  return (
    <div role="alert" className={`vv-offline ${stylex.props(offlineStyles.strip, offlineStyles.banner).className}`}>
      <Icon name="danger" size={17} xstyle={offlineStyles.icon} />
      <div {...stylex.props(offlineStyles.body)}>
        <strong {...stylex.props(offlineStyles.title)}>{t.offline.title}{failures > 0 && reconnectAt ? <> · {t.offline.retryingIn(nextIn)}</> : null}</strong>
        <span {...stylex.props(offlineStyles.text)}>{t.offline.body}</span>
      </div>
      <Button size="sm" variant="ghost" onClick={retry ?? (() => events.retryNow())}><Icon name="refresh" size={14} />{t.common.retryNow}</Button>
    </div>
  )
}
