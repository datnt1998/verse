import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { useConnection } from '../data/Provider'
import { useSession } from '../api/session'
import { t } from '../i18n/vi'
import { Spinner } from '@noverse/ui'

const sx = stylex.create({
  root: {
    position: 'fixed', top: 12, left: '50%', transform: 'translateX(-50%)', zIndex: 1100,
    height: 40, paddingInline: 14, borderRadius: 999,
    backgroundColor: kit['--kit-surface-toast'],
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-warn-35'],
    color: color['--color-warn'], fontSize: 13,
    display: 'flex', alignItems: 'center', gap: 10,
    boxShadow: kit['--kit-shadow-conn'],
  },
  retry: {
    height: 28, paddingInline: 10, borderRadius: 999,
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    color: color['--color-fg'], backgroundColor: 'transparent', fontSize: 12,
  },
})

export function ConnectionBanner() {
  const { user } = useSession()
  const connection = useConnection()
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    if (connection.reconnectAt === null) return
    const timer = setInterval(() => setNow(Date.now()), 250)
    return () => clearInterval(timer)
  }, [connection.reconnectAt])
  if (!user || (connection.online && (connection.state === 'open' || connection.state === 'stopped'))) return null
  const seconds = Math.max(0, Math.ceil(((connection.reconnectAt ?? now) - now) / 1000))
  return <div role="status" aria-live="polite" {...stylex.props(sx.root)}>
    <Spinner size={14} color="var(--color-warn)" />
    <span>{t.connection.lost}{connection.reconnectAt !== null ? ` · ${t.connection.retryIn(seconds)}` : ' · Đang nối lại dữ liệu trực tiếp…'}</span>
    <button type="button" onClick={connection.retry} {...stylex.props(sx.retry)}>{t.connection.retryNow}</button>
  </div>
}
