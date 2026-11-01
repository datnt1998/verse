// A1 › qr · Đăng nhập điện thoại bằng QR (Login.dc.html + LoginQR.dc.html).
// Mã QR vẽ ở trình duyệt từ liên kết ngắn /p/<mã> (ui/QrCode) · một liên kết duy nhất · hết hạn sau 5 phút · phiên mới cập nhật qua sự kiện.
import { useEffect, useRef, useState } from 'react'
import { listUsers, myPairing, mySessions, userPairing, userSessions } from '../../api/client'
import { useSession } from '../../api/session'
import type { Pairing, Session } from '../../api/types'
import { t } from '../../i18n/vi'
import { Dialog } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Skeleton, Spinner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { fmtDate } from '../auth/parts'
import { SessionList } from './shared'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Combobox } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { QrCode, QrZoom, QrZoomButton } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const TTL_MS = 5 * 60_000

const mmss = (ms: number) => {
  const total = Math.max(0, Math.ceil(ms / 1000))
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`
}

export function PairingDialog({ onClose, forUser }: { onClose: () => void; forUser?: string }) {
  const { user } = useSession()
  const me = user
  const isAdmin = me?.role === 'admin'
  const accountsResource = useResource(keys.users(), listUsers, { enabled: isAdmin })
  const accounts = accountsResource.data?.users ?? []
  const [target, setTarget] = useState(forUser ?? me?.username ?? '')
  const [pairing, setPairing] = useState<Pairing | null>(null)
  const sessionsResource = useResource([...keys.sessions(), target === me?.username ? 'me' : target], () => target === me?.username ? mySessions() : userSessions(target), { enabled: !!target })
  const sessions = sessionsResource.data?.sessions ?? []
  const connection = useConnection()
  const [baseline, setBaseline] = useState<Set<string>>(new Set())
  const [pairedSession, setPairedSession] = useState<Session | null>(null)
  const [qrZoom, setQrZoom] = useState(false)
  const generatedFor = useRef('')
  const [now, setNow] = useState(Date.now())
  const pairingAction = useAction(async () => {
    setBaseline(new Set(sessions.map((session) => session.id)))
    const created = target === me?.username ? await myPairing() : await userPairing(target)
    setPairing(created); setPairedSession(null); setNow(Date.now())
    return created
  })
  const loading = pairingAction.busy || sessionsResource.initialLoading
  const error = pairingAction.error instanceof Error ? pairingAction.error.message : ''
  const [announce, setAnnounce] = useState('')

  useEffect(() => {
    if (!sessionsResource.data || !target || generatedFor.current === target) return
    generatedFor.current = target
    setPairing(null)
    void pairingAction.run()
  }, [sessionsResource.data, target, pairingAction.run])

  useEffect(() => {
    if (!pairing) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [pairing])

  const expiresMs = pairing ? new Date(pairing.expiresAt).getTime() : 0
  const left = Math.max(0, expiresMs - now)
  const expired = pairing !== null && left <= 0

  // Đếm ngược chỉ đọc ở mốc 60s / 10s / hết hạn (aria-live polite).
  useEffect(() => {
    if (!pairing) return
    if (left <= 0) setAnnounce('Mã đã hết hạn')
    else if (left <= 10_000) setAnnounce(`Mã còn ${Math.ceil(left / 1000)} giây`)
    else if (left <= 60_000) setAnnounce('Mã còn 1 phút')
  }, [left, pairing])

  useEffect(() => {
    if (!pairing || expired) return
    const fresh = sessions.find((session) => !baseline.has(session.id) && !session.current)
    if (fresh) setPairedSession(fresh)
  }, [sessions, pairing, baseline, expired])

  const link = pairing?.links[0] ?? null
  const targetName = accounts.find((account) => account.username === target)?.name || target
  const ttlColor = expired ? 'var(--color-err)' : 'var(--color-ok)'

  const copyLink = () => {
    if (!link) return
    void navigator.clipboard?.writeText(link.url).then(() => toast.ok('Đã sao chép liên kết đăng nhập')).catch(() => toast.err('Trình duyệt chặn sao chép — hãy chọn liên kết bằng tay'))
  }

  return (
    <Dialog
      title={t.qr.title}
      icon={<Icon name="mobile-bulk" size={20} style={{ color: 'var(--color-accent)' }} />}
      onClose={onClose}
      width={880}
      style={{ maxHeight: 'calc(100dvh - 32px)' }}
    >
      <PanelRefreshBar resourceKeys={[keys.users(), [...keys.sessions(), target === me?.username ? 'me' : target]]} updatedAt={sessionsResource.updatedAt} />
      <StaleBanner error={sessionsResource.error || accountsResource.error || (!connection.writable ? new Error('Mất kết nối') : null)} onRetry={() => { void sessionsResource.refresh(); void accountsResource.refresh() }} />
      <div className="scroll" style={{ display: 'flex', flexWrap: 'wrap', gap: 24, padding: 20, overflowY: 'auto', minHeight: 0 }}>
        <div style={{ flex: '0 0 auto', width: 'fit-content', minWidth: 300, maxWidth: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
          {isAdmin && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <Combobox id="qr-user" label={t.qr.forAccount} value={target} onChange={setTarget} options={(accounts.length > 0 ? accounts : me ? [me] : []).filter((account) => !account.disabled).map((account) => ({ value: account.username, label: `${account.name || account.username}${account.username === me?.username ? ' (bạn)' : ''} · @${account.username}` }))} />
            </div>
          )}

          {link ? (
            <>
              <QrCode value={link.url} variant="dialog" expired={expired} renew={{ onClick: () => { void pairingAction.run() }, disabled: pairingAction.busy || !connection.writable, pending: pairingAction.pending, success: pairingAction.success }} />
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: 'var(--color-fg-3)' }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="timer-1" size={14} />{t.qr.singleUse}</span>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span className="mono num" style={{ color: ttlColor }}>{mmss(left)}</span>
                    {link && <QrZoomButton onClick={() => setQrZoom(true)} />}
                  </span>
                </div>
                <div aria-hidden="true" style={{ height: 3, borderRadius: 3, background: 'var(--color-surface-5)' }}>
                  <div style={{ width: `${Math.min(100, Math.max(0, (left / TTL_MS) * 100))}%`, height: '100%', borderRadius: 3, background: ttlColor }} />
                </div>
                <span className="sr-only" aria-live="polite">{announce}</span>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button variant="ghost" disabled={pairingAction.busy || !connection.writable} pending={pairingAction.pending} success={pairingAction.success} onClick={() => { void pairingAction.run() }} style={{ flex: 1, height: 34 }}><Icon name="refresh-2" size={15} />{t.qr.renew}</Button>
                <button type="button" className="hov-ghost" onClick={copyLink} style={{
                  flex: 1, height: 34, borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent',
                  color: 'var(--color-fg)', fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
                }}><Icon name="copy" size={15} />{t.qr.copy}</button>
              </div>
            </>
          ) : loading ? (
            <div style={{ minHeight: 384, borderRadius: 18, border: '1.5px dashed rgba(255,255,255,0.14)', padding: 22, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
              <Skeleton height={260} radius={18} />
            </div>
          ) : (
            <div style={{ minHeight: 384, borderRadius: 18, border: '1.5px dashed rgba(255,255,255,0.14)', padding: 22, display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 12 }}>
              <Icon name="link-2" size={28} style={{ color: 'var(--color-warn)' }} />
              <span style={{ fontSize: 15, fontWeight: 600 }}>{t.qr.linkFailed}</span>
              <Button variant="ghost" disabled={pairingAction.busy || !connection.writable} onClick={() => { void pairingAction.run() }} style={{ alignSelf: 'flex-start' }}>{t.qr.renew}</Button>
            </div>
          )}
          {error && (
            <span role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}>
              <Icon name="warning-2" size={14} />{error}
            </span>
          )}
        </div>

        <div style={{ flex: '1 1 360px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 16 }}>
          <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 14 }}>
            {[
              { title: 'Mở camera trên điện thoại', desc: 'Điện thoại chỉ cần Internet — không phải cùng mạng với máy tính.' },
              { title: 'Mở Camera và quét mã', desc: 'Trang Noverse mở ra trên trình duyệt điện thoại.' },
              { title: 'Bấm “Đăng nhập” trên điện thoại', desc: 'Phiên mới giữ 30 ngày, quản lý được ở “Thiết bị đang đăng nhập”.' },
            ].map((step, index) => (
              <li key={step.title} style={{ display: 'flex', gap: 12 }}>
                <span className="num" aria-hidden="true" style={{
                  width: 26, height: 26, flex: '0 0 auto', borderRadius: '50%', background: 'var(--color-surface-6)',
                  border: '1px solid var(--line-3)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 12, fontWeight: 600, color: 'var(--color-fg-2)',
                }}>{index + 1}</span>
                <span style={{ display: 'flex', flexDirection: 'column', gap: 2, paddingTop: 2 }}>
                  <span style={{ fontSize: 14, fontWeight: 500 }}>{step.title}</span>
                  <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{step.desc}</span>
                </span>
              </li>
            ))}
          </ol>

          {link && (
            <div role="status" className="card" style={{
              borderRadius: 14, padding: '12px 14px', display: 'flex', alignItems: 'center', gap: 12,
              borderColor: pairedSession ? 'rgba(166,237,193,0.35)' : 'var(--line-2)',
            }}>
              {pairedSession ? (
                <>
                  <Icon name="tick-circle-bulk" size={18} style={{ color: 'var(--color-ok)' }} />
                  <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <span style={{ fontSize: 13, fontWeight: 500 }}>{t.qr.paired(pairedSession.device)}</span>
                    <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>giữ đăng nhập đến {fmtDate(new Date(pairedSession.expiresAt))}</span>
                  </span>
                  <button type="button" className="hov-primary" onClick={onClose} style={{
                    height: 32, padding: '0 12px', borderRadius: 8, border: 'none',
                    background: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600,
                  }}>Xong</button>
                </>
              ) : (
                <>
                  <Spinner size={14} />
                  <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <span style={{ fontSize: 13, fontWeight: 500 }}>{t.qr.waiting}</span>
                    <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Thiết bị mới sẽ hiện ở danh sách bên dưới ngay khi đăng nhập</span>
                  </span>
                </>
              )}
            </div>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
              <span className="lbl">Thiết bị của {targetName}</span>
              <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{sessions.length}</span>
            </div>
            <SessionList sessions={sessions} loading={loading} error={error} freshId={pairedSession?.id ?? null} />
          </div>

          <div style={{
            display: 'flex', gap: 10, padding: '10px 12px', borderRadius: 12,
            background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.18)',
            fontSize: 12, color: 'var(--color-fg-2)',
          }}>
            <Icon name="warning-2" size={15} style={{ color: 'var(--color-warn)', marginTop: 1 }} />
            <span>Đừng chia sẻ mã này — ai quét được sẽ vào được tài khoản <span className="mono">@{target}</span> trong 5 phút.</span>
          </div>
        </div>
      </div>
      {qrZoom && link && <QrZoom value={link.url} onClose={() => setQrZoom(false)} />}
    </Dialog>
  )
}
