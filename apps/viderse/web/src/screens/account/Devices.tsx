import { useEffect, useState } from 'react'
import { Button, FieldError, Icon, Spinner } from '@noverse/ui'
import { mySessions, revokeSession, revokeAllMySessions } from '../../api/client'
import type { Session } from '../../api/types'
import { t } from '../../i18n/vi'
import * as stylex from '@stylexjs/stylex'
import { accountStyles } from '../../components/AppLinks'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { accountStyles as layout } from './account.stylex'
import { authStyles } from '../auth/auth.stylex'
import { ApiError } from '../../api/client'
import { Banner } from '@noverse/ui'
const sx = stylex.create({
  mobileList: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-hairline)', borderRadius: 'var(--vv-r-16)', backgroundColor: 'var(--color-surface-0)', overflow: 'hidden' },
  mobileAction: { flexGrow: 1, height: 48, justifyContent: 'center' },
})

function deviceActivity(session: Session): string {
  const days = Math.max(0, Math.ceil((new Date(session.expiresAt).getTime() - Date.now()) / 86400000))
  const minutes = Math.max(0, Math.floor((Date.now() - new Date(session.lastActiveAt).getTime()) / 60000))
  const activity = minutes < 1 ? t.account.justActive : minutes < 60 ? t.account.minutesAgo(minutes) : minutes < 1440 ? t.account.hoursAgo(Math.floor(minutes / 60)) : t.account.daysAgo(Math.floor(minutes / 1440))
  return session.current ? t.account.deviceCurrentActivity(days) : t.account.deviceActivity(activity, days)
}

export function Devices() {
  const mobile = useMediaQuery('(max-width: 767px)')
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmAll, setConfirmAll] = useState(false)
  const [done, setDone] = useState('')
  const [networkError, setNetworkError] = useState(false)
  const load = async () => {
    setError(''); setNetworkError(false)
    try { setSessions((await mySessions()).sessions) } catch (e) { if (e instanceof ApiError && e.status === 0) setNetworkError(true); else setError(t.account.devicesFailed) }
  }
  useEffect(() => { void load() }, [])
  const revoke = async (ids: string[], all = false) => {
    if (busy) return
    setBusy(true); setError(''); setNetworkError(false); setDone('')
    try {
      if (all) {
        await revokeAllMySessions()
        setSessions((previous) => previous?.filter((session) => session.current) ?? null)
        setConfirmAll(false)
        setDone(t.account.allDevicesDone)
      } else {
        for (const id of ids) {
          await revokeSession(id)
          setSessions((previous) => previous?.filter((session) => session.id !== id) ?? null)
          const session = sessions?.find((item) => item.id === id)
          if (session) setDone(t.account.deviceDone(`${session.device}${session.browser ? ` · ${session.browser}` : ''}`))
        }
      }
    } catch (e) { if (e instanceof ApiError && e.status === 0) setNetworkError(true); else setError(t.account.revokeFailed) } finally { setBusy(false) }
  }
  return <>
    {!mobile && <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'baseline', gap: 'var(--vv-s-10)' }}><h2 style={{ flex: 1, margin: 0, fontSize: 14, fontWeight: 600 }}>{t.account.devices}</h2><span {...stylex.props(accountStyles.fieldHelp)} style={{ color: 'var(--kit-fg-sub)' }}>{t.account.deviceNote}</span></div>}
    {networkError && <Banner tone="warning" title={t.account.networkFailed} titleStyle={authStyles.networkTitle} icon="danger" iconSize={16} role="alert" align="top" />}
    {error && <FieldError>{error}</FieldError>}
    {!sessions && (error || networkError ? <Button onClick={() => void load()}>{t.common.retry}</Button> : <Spinner size={16} />)}
    <div role="list" {...(mobile ? stylex.props(sx.mobileList) : {})}>{sessions?.map((session) => <div key={session.id} role="listitem" style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-12)', minHeight: mobile ? 64 : 48, padding: mobile ? '0 14px' : 0, borderTop: '1px solid var(--vv-hairline)' }}>
      <Icon name={/mobile|android|iphone|ipad/i.test(session.device) ? 'mobile' : 'monitor'} size={mobile ? 20 : 18} style={{ color: 'var(--color-fg-3)' }} />
      <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
        <span title={`${session.device}${session.browser ? ` · ${session.browser}` : ''}`} style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontSize: 13.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{session.device}{session.browser && ` · ${session.browser}`}{session.current && <span {...stylex.props(accountStyles.tag, accountStyles.neutralTag)}>{t.account.currentDevice}</span>}</span>
        <span style={{ fontSize: mobile ? 12.5 : 12, color: mobile && new Date(session.expiresAt).getTime() - Date.now() <= 3 * 86400000 ? 'var(--color-warn)' : 'var(--kit-fg-sub)' }}>{mobile ? deviceActivity(session) : t.account.lastSeen(new Date(session.lastActiveAt).toLocaleString('vi-VN'))}</span>
      </span>
      {!session.current && <Button size="sm" variant="quiet" disabled={busy} aria-label={`${t.account.logout} ${session.device}${session.browser ? ` · ${session.browser}` : ''}`} onClick={() => void revoke([session.id])} style={mobile ? { height: 44, flexShrink: 0, color: 'var(--vv-warn-ink)' } : undefined}>{t.account.logout}</Button>}
    </div>)}</div>
    {done ? <Banner tone="success" title={done} icon="tick-b" iconSize={16} /> : mobile && <span {...stylex.props(layout.mobileNote)}>{t.account.deviceSelfNote}</span>}
    <div style={{ display: 'flex', flexDirection: mobile ? 'column' : 'row', flexWrap: 'wrap', alignItems: mobile ? 'stretch' : 'center', gap: 'var(--vv-s-12)', paddingTop: 'var(--vv-s-12)', borderTop: '1px solid var(--vv-hairline)' }}>
      {confirmAll ? <>
        <span style={{ flex: mobile ? undefined : '1 1 300px', fontSize: 13 }}>{t.account.confirmOtherDevices(sessions?.filter((session) => !session.current).length ?? 0)}</span>
        <div style={{ display: 'flex', gap: 'var(--vv-s-10)' }}>
          <Button disabled={busy} onClick={() => setConfirmAll(false)} {...(mobile ? stylex.props(sx.mobileAction) : {})}>{t.common.cancel}</Button>
          <Button variant="dsoft" disabled={busy} pending={busy} onClick={() => void revoke([], true)} {...(mobile ? stylex.props(sx.mobileAction) : {})}>{busy ? t.account.loggingOut : t.account.confirmOtherDevicesButton(sessions?.filter((session) => !session.current).length ?? 0)}</Button>
        </div>
      </> : <>
        {!mobile && <span style={{ flex: '1 1 300px', fontSize: 12.5, color: 'var(--kit-fg-sub)' }}>{t.account.deviceLogoutNote}</span>}
        <Button variant={mobile ? 'dsoft' : 'ghost'} disabled={busy || !sessions?.some((session) => !session.current)} onClick={() => setConfirmAll(true)} {...(mobile ? stylex.props(sx.mobileAction) : {})}>{t.account.otherDevices}</Button>
        {mobile && <span style={{ textAlign: 'center', fontSize: 12.5, color: 'var(--kit-fg-sub)' }}>{t.account.deviceLogoutNote}</span>}
      </>}
    </div>
  </>
}
