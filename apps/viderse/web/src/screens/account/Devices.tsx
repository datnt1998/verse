import { useEffect, useState } from 'react'
import { Button, FieldError, Icon, Spinner } from '@noverse/ui'
import { mySessions, revokeSession, revokeAllMySessions } from '../../api/client'
import type { Session } from '../../api/types'
import { t } from '../../i18n/vi'
import * as stylex from '@stylexjs/stylex'
import { accountStyles } from '../../components/AppLinks'

export function Devices() {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const load = async () => {
    setError('')
    try { setSessions((await mySessions()).sessions) } catch { setError(t.account.devicesFailed) }
  }
  useEffect(() => { void load() }, [])
  const revoke = async (ids: string[], all = false) => {
    if (busy) return
    setBusy(true); setError('')
    try {
      if (all) {
        await revokeAllMySessions()
        setSessions((previous) => previous?.filter((session) => session.current) ?? null)
      } else {
        for (const id of ids) {
          await revokeSession(id)
          setSessions((previous) => previous?.filter((session) => session.id !== id) ?? null)
        }
      }
    } catch { setError(t.account.revokeFailed) } finally { setBusy(false) }
  }
  return <>
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'baseline', gap: 10 }}><h2 style={{ flex: 1, margin: 0, fontSize: 14, fontWeight: 600 }}>{t.account.devices}</h2><span {...stylex.props(accountStyles.fieldHelp)} style={{ color: 'var(--kit-fg-sub)' }}>{t.account.deviceNote}</span></div>
    {error && <FieldError>{error}</FieldError>}
    {!sessions && (error ? <Button onClick={() => void load()}>{t.common.retry}</Button> : <Spinner size={16} />)}
    <div role="list">{sessions?.map((session) => <div key={session.id} role="listitem" style={{ display: 'flex', alignItems: 'center', gap: 12, minHeight: 48, borderTop: '1px solid var(--vv-hairline)' }}>
      <Icon name={/mobile|android|iphone|ipad/i.test(session.device) ? 'mobile' : 'monitor'} size={18} style={{ color: 'var(--color-fg-3)' }} />
      <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
        <span title={session.device} style={{ display: 'flex', alignItems: 'center', gap: 6, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{session.device}{session.current && <span {...stylex.props(accountStyles.tag, accountStyles.neutralTag)}>{t.account.currentDevice}</span>}</span>
        <span style={{ fontSize: 12, color: 'var(--kit-fg-sub)' }}>{t.account.lastSeen(new Date(session.lastSeen).toLocaleString('vi-VN'))}</span>
      </span>
      {!session.current && <Button size="sm" variant="quiet" disabled={busy} onClick={() => void revoke([session.id])}>{t.account.logout}</Button>}
    </div>)}</div>
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 12, paddingTop: 12, borderTop: '1px solid var(--vv-hairline)' }}>
      <span style={{ flex: '1 1 300px', fontSize: 12.5, color: 'var(--kit-fg-sub)' }}>{t.account.deviceLogoutNote}</span>
      <Button size="sm" disabled={busy || !sessions?.some((session) => !session.current)} pending={busy} onClick={() => void revoke([], true)}>{t.account.otherDevices}</Button>
    </div>
  </>
}
