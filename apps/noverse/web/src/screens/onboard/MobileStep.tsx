// A3 › bước “Điện thoại & người cùng viết” (Onboard.dc.html step mobile, vai admin / writer).
// Thẻ QR (ui/QrCode) và thẻ mời người cùng viết; mọi dữ liệu + hành động nằm ở OnboardScreen (model).
import { useEffect, useState } from 'react'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { Skeleton, Spinner } from '@noverse/ui'
import { KitSelect, type Option } from '@noverse/ui'
import { Tooltip } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { QR_RANGE, QrCode, QrCountdown, QrZoom, QrZoomButton, qrMetrics } from '@noverse/ui'
import { AuthInput, fmtDate, linkKind } from '../auth/parts'
import { QrNoLan } from '../account/QrNoLan'
import { projectLabel, storyLabel } from '../../shell/labels'
import { USERNAME_RE, type OnboardModel } from './model'
import { copyText as writeClipboard } from '../../lib/clipboard'

const WHERE = [
  { key: 'home', label: 'Ở nhà', sub: 'cùng Wi-Fi', icon: 'wifi' },
  { key: 'out', label: 'Ra ngoài', sub: 'qua Tailscale', icon: 'global' },
] as const
type Where = (typeof WHERE)[number]['key']

const ROLES = [
  { key: 'collab', label: 'Cộng tác', sub: 'viết & góp ý' },
  { key: 'view', label: 'Chỉ xem', sub: 'đọc, nghe, tải bản xuất' },
] as const

const GUIDE = ['Mở Camera trên điện thoại, hướng vào mã.', 'Chạm đường dẫn hiện ra, bấm Đăng nhập. Phiên giữ 30 ngày.']

const mmss = (ms: number) => {
  const total = Math.max(0, Math.ceil(ms / 1000))
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`
}

/** Chép vào clipboard; trình duyệt chặn (http qua Wi-Fi nhà) thì báo để người dùng chọn tay. */
async function copyText(text: string): Promise<boolean> {
  try {
    if (!navigator.clipboard) throw new Error('no clipboard')
    await writeClipboard(text)
    return true
  } catch {
    toast.err('Trình duyệt chặn sao chép — hãy chọn bằng tay')
    return false
  }
}

/** Hiện “đã chép” 2 giây rồi trả về trạng thái thường. */
function useCopied(): [boolean, () => void] {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(timer)
  }, [copied])
  return [copied, () => setCopied(true)]
}


const ghostButton = { border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#C9C9D1', display: 'flex', alignItems: 'center', justifyContent: 'center' } as const

export function MobileStep({ model }: { model: OnboardModel }) {
  return (
    <>
      <div>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-.015em' }}>{model.isAdmin ? 'Điện thoại & người cùng viết' : 'Mở Noverse trên điện thoại'}</h1>
        <p style={{ margin: '6px 0 0', fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 560 }}>
          {model.isAdmin
            ? 'Theo dõi và can thiệp ngay trên điện thoại. Thêm tài khoản cho người cộng tác — mỗi người chỉ thấy truyện được giao.'
            : 'Quét mã để đăng nhập trên điện thoại, không cần gõ mật khẩu.'}
        </p>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: model.isAdmin ? 'repeat(auto-fit, minmax(min(100%, 330px), 1fr))' : 'minmax(0, 340px)', gap: 12, alignItems: 'start' }}>
        <PhoneCard model={model} />
        {model.isAdmin && <InviteCard model={model} />}
      </div>
      {model.isAdmin && (
        <div style={{ display: 'flex', gap: 10, padding: '10px 12px', borderRadius: 12, background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.18)', fontSize: 12.5, color: '#C9C9D1' }}>
          <Icon name="warning-2" size={15} style={{ color: '#F6CB8C', marginTop: 2 }} />
          <span>Gửi tên đăng nhập và mật khẩu tạm qua kênh riêng (tin nhắn, gặp trực tiếp). Lần đầu vào, họ được nhắc đặt mật khẩu mới.</span>
        </div>
      )}
    </>
  )
}

function PhoneCard({ model }: { model: OnboardModel }) {
  const { pairing, pairedSession } = model
  const [now, setNow] = useState(Date.now())
  const [where, setWhere] = useState<Where>('home')
  const [zoom, setZoom] = useState(false)
  const [copied, markCopied] = useCopied()
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  const links = pairing?.links ?? []
  const byKind: Partial<Record<Where, (typeof links)[number]>> = {}
  for (const entry of links) {
    const kind: Where = linkKind(entry).kind === 'tailscale' ? 'out' : 'home'
    byKind[kind] ??= entry
  }
  const available = WHERE.filter((entry) => byKind[entry.key])
  const current: Where | undefined = byKind[where] ? where : available[0]?.key
  const link = current ? byKind[current] : undefined
  const left = pairing ? Math.max(0, new Date(pairing.expiresAt).getTime() - now) : 0
  const expired = pairing !== null && left <= 0
  const renew = { onClick: model.renewPairing, disabled: model.pairingLoading, pending: model.pairingLoading }

  return (
    <section className="card" aria-label="Điện thoại của bạn" style={{ borderRadius: 18, padding: 18, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14 }}>
      <div style={{ alignSelf: 'stretch', display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon name="mobile" size={18} style={{ color: '#C9A6F5' }} />
        <span style={{ flex: '1 1 auto', fontSize: 14.5, fontWeight: 600 }}>Điện thoại của bạn</span>
        {link && <><QrCountdown text={mmss(left)} expired={expired} /><QrZoomButton onClick={() => setZoom(true)} /></>}
      </div>
      {link ? (
        <>
          <QrCode value={link.url} variant="card" expired={expired} renew={renew} />
          {available.length > 1 && (
            <Segmented label="Điện thoại đang ở đâu" size={40} radius={8}
              value={current} onChange={(next) => setWhere(next as Where)}
              style={{ alignSelf: 'stretch', gap: 2, borderRadius: 11, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)' }}
              options={WHERE.map((entry) => ({
                id: entry.key, label: <span style={{ fontSize: 12.5, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 5 }}><Icon name={entry.icon} size={13} />{entry.label}</span>,
                hint: entry.sub,
              }))} />
          )}
          <ol style={{ alignSelf: 'stretch', margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13, color: '#B8B8C2' }}>
            {GUIDE.map((text, index) => (
              <li key={text} style={{ display: 'flex', gap: 8 }}>
                <span className="num" style={{ width: 18, height: 18, borderRadius: '50%', background: 'rgba(255,255,255,0.07)', fontSize: 11, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto', marginTop: 1 }}>{index + 1}</span>
                {text}
              </li>
            ))}
          </ol>
          <div style={{ alignSelf: 'stretch', display: 'flex', alignItems: 'center', gap: 8, paddingTop: 10, borderTop: '1px solid rgba(255,255,255,0.06)' }}>
            {pairedSession ? (
              <span role="status" style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
                <Icon name="tick-circle-bulk" size={18} style={{ color: '#A6EDC1' }} />
                <span style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontSize: 13, fontWeight: 500 }}>Đã đăng nhập trên {pairedSession.device}</span>
                  <span style={{ fontSize: 12, color: '#7E7E8A' }}>giữ đăng nhập đến {fmtDate(new Date(pairedSession.expiresAt))}</span>
                </span>
              </span>
            ) : (
              <span role="status" style={{ flex: '1 1 auto', display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: '#A0A0AB' }}>
                {expired ? 'Mã đã hết hạn' : <><Spinner size={12} />Đang chờ quét…</>}
              </span>
            )}
            <button type="button" className="hov-ghost" onClick={() => { void copyText(link.url).then((ok) => { if (ok) markCopied() }) }} style={{ ...ghostButton, height: 28, padding: '0 10px', borderRadius: 8, fontSize: 12, gap: 5, whiteSpace: 'nowrap' }}>
              <Icon name={copied ? 'tick-circle' : 'link-2'} size={13} />{copied ? 'Đã chép' : 'Chép đường dẫn'}
            </button>
          </div>
        </>
      ) : model.pairingLoading ? (
        <Skeleton width={qrMetrics(33, QR_RANGE.card).size} height={qrMetrics(33, QR_RANGE.card).size} radius={16} />
      ) : (
        <QrNoLan canEnable={model.isAdmin} busy={model.lanBusy} error={model.lanError} onEnable={model.enableLan} onRetry={pairing ? undefined : model.renewPairing} />
      )}
      {zoom && link && <QrZoom value={link.url} onClose={() => setZoom(false)} />}
    </section>
  )
}

function projectOptions(model: OnboardModel): Option[] {
  return [...model.projects.map((project) => ({ value: project.id, label: projectLabel(project, project.id) })), { value: 'later', label: 'Chọn sau' }]
}

function InviteCard({ model }: { model: OnboardModel }) {
  const [pwCopied, markPwCopied] = useCopied()
  const [inviteCopied, markInviteCopied] = useCopied()
  const done = model.invited
  const user = model.inviteUsername
  const bad = !USERNAME_RE.test(user)
  const showBad = bad && user !== ''
  const origin = (() => { try { return new URL(model.pairing?.links[0]?.url ?? '').origin } catch { return location.origin } })()
  const inviteText = done
    ? `Mời bạn cùng viết trên Noverse.\nĐịa chỉ: ${origin}\nTên đăng nhập: ${done.username}\nMật khẩu tạm: ${done.password}\nLần đầu vào, bạn sẽ được nhắc đặt mật khẩu mới.`
    : ''
  return (
    <section className="card" aria-label="Mời người cùng viết" style={{ borderRadius: 18, padding: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14.5, fontWeight: 600 }}>
        <Icon name="user-add" size={18} style={{ color: '#C9A6F5' }} />Mời người cùng viết
      </span>
      <label style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span className="lbl">Tên đăng nhập</span>
        <AuthInput id="ob-inv-user" value={user} invalid={showBad} onChange={(value) => model.setInviteUsername(value.toLowerCase())} placeholder="vd: linh" autoComplete="off" />
        <span style={{ fontSize: 11.5, color: showBad ? '#FF9AA2' : '#7E7E8A' }}>
          {showBad ? 'Chỉ dùng chữ thường không dấu, số, “.”, “_” hoặc “-” (2–32 ký tự)' : 'Chữ thường không dấu, số, dấu chấm hoặc gạch'}
        </span>
      </label>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span className="lbl">Vai trò</span>
        <Segmented label="Vai trò" value={model.inviteRole} size={40} radius={8}
          onChange={(next) => model.setInviteRole(next as 'collab' | 'view')}
          options={ROLES.map((entry) => ({
            id: entry.key, label: <span style={{ fontSize: 12.5, fontWeight: 500 }}>{entry.label}</span>,
            hint: entry.sub, disabled: !!done,
          }))}
          style={{ gap: 2, background: 'var(--color-surface-0)', border: '1px solid var(--line-3)' }} />
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span className="lbl">Mật khẩu tạm</span>
        <div style={{ display: 'flex', gap: 6 }}>
          <span className="num" style={{ flex: '1 1 auto', minWidth: 0, height: 38, padding: '0 12px', borderRadius: 9, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.1)', display: 'flex', alignItems: 'center', fontSize: 15, fontWeight: 600, letterSpacing: '.02em', color: '#EDEDEF' }}>{model.invitePassword}</span>
          <Tooltip content="Chép mật khẩu tạm" asChild>
            <button type="button" className="hov-ghost" aria-label="Chép mật khẩu tạm" onClick={() => { void copyText(model.invitePassword).then((ok) => { if (ok) markPwCopied() }) }} style={{ ...ghostButton, width: 38, height: 38, borderRadius: 9 }}>
              <Icon name={pwCopied ? 'tick-circle' : 'copy'} size={16} />
            </button>
          </Tooltip>
          <Tooltip content="Tạo mật khẩu khác" asChild>
            <button type="button" className="hov-ghost" aria-label="Tạo mật khẩu khác" disabled={!!done} onClick={model.refreshPassword} style={{ ...ghostButton, width: 38, height: 38, borderRadius: 9 }}>
              <Icon name="refresh-2" size={16} />
            </button>
          </Tooltip>
        </div>
      </div>
      <KitSelect
        label="Dự án được vào"
        aria-label="Dự án được vào"
        disabled={!!done}
        value={Object.keys(model.inviteProjects)[0] ?? 'later'}
        onChange={(value) => model.setInviteProjects(value === 'later' ? {} : { [value]: model.inviteRole })}
        options={projectOptions(model)}
      />
      {model.inviteError && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{model.inviteError}</span>}
      {done ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: 12, borderRadius: 12, background: 'rgba(166,237,193,0.06)', border: '1px solid rgba(166,237,193,0.25)' }}>
          <span role="status" style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13.5, color: '#C6F3D6' }}>
            <Icon name="tick-circle" size={16} />Đã tạo tài khoản cho {done.username}
          </span>
          <button type="button" className="hov-ghost" onClick={() => { void copyText(inviteText).then((ok) => { if (ok) markInviteCopied() }) }} style={{ ...ghostButton, alignSelf: 'stretch', minHeight: 30, padding: '6px 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.12)', color: '#EDEDEF', fontSize: 12.5, gap: 6, textAlign: 'left' }}>
            <Icon name="copy" size={14} />{inviteCopied ? 'Đã chép' : 'Chép lời mời (địa chỉ + tên + mật khẩu tạm)'}
          </button>
        </div>
      ) : (
        <Button variant="primary" pending={model.invitePending} success={model.inviteSuccess} disabled={bad || model.busy} onClick={() => { void model.createInvite() }} style={{ height: 38, borderRadius: 10, fontSize: 13.5, fontWeight: 600, background: bad ? 'rgba(255,255,255,0.08)' : '#EDEDEF', color: bad ? '#7E7E8A' : '#0B0B0D' }}>
          <Icon name="add" size={15} />Tạo tài khoản
        </Button>
      )}
    </section>
  )
}
