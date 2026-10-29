// Mảnh dùng chung cho A1/A2 (09-auth-onboard.md §4): AuthShell, OrbHero, ô nhập 40px,
// nhãn thiết bị, định dạng ngày và mô tả route quay lại.
import { forwardRef, useMemo, type ReactNode } from 'react'
import { getLibrary } from '../../api/client'
import { useSession } from '../../api/session'
import { keys, useResource } from '../../data'
import { TextField, type TextFieldProps } from '@noverse/ui'
import { getLastRoute, takeNextRoute } from '../../api/storage'
import { AgentOrb, type AgentKind } from '@noverse/ui'
import { BrandLockup } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'

/** “Mac · Chrome” — nhãn thiết bị hiển thị (không lưu User-Agent thô). */
export function detectDevice(): string {
  if (typeof navigator === 'undefined') return 'Trình duyệt'
  const ua = navigator.userAgent
  const platform = /iPhone/.test(ua) ? 'iPhone'
    : /iPad/.test(ua) ? 'iPad'
    : /Android/.test(ua) ? 'Android'
    : /Macintosh|Mac OS X/.test(ua) ? 'Mac'
    : /Windows/.test(ua) ? 'Windows'
    : /Linux/.test(ua) ? 'Linux'
    : 'Máy tính'
  const browser = /Edg\//.test(ua) ? 'Edge'
    : /OPR\//.test(ua) ? 'Opera'
    : /Chrome\//.test(ua) ? 'Chrome'
    : /Firefox\//.test(ua) ? 'Firefox'
    : /Safari\//.test(ua) ? 'Safari'
    : 'Trình duyệt'
  return `${platform} · ${browser}`
}

export const fmtDate = (date: Date) =>
  `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}/${date.getFullYear()}`

export const fmtShort = (value: string | null | undefined) => {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}`
}

/** “vừa xong”, “1 giờ trước”, “3 ngày trước” từ mốc ISO. */
export function relativeTime(value: string | null | undefined): string {
  if (!value) return '—'
  const ms = Date.now() - new Date(value).getTime()
  if (Number.isNaN(ms)) return '—'
  if (ms < 60_000) return 'vừa xong'
  const minutes = Math.floor(ms / 60_000)
  if (minutes < 60) return `${minutes} phút trước`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} giờ trước`
  const days = Math.floor(hours / 24)
  return `${days} ngày trước`
}

export const daysUntil = (value: string | null | undefined) => {
  if (!value) return 0
  const ms = new Date(value).getTime() - Date.now()
  return Number.isNaN(ms) ? 0 : Math.max(0, Math.ceil(ms / 86_400_000))
}

/** Route sau khi đăng nhập: ưu tiên #/welcome khi chưa thiết lập lần đầu. */
export function authDestination(user: { username: string; onboardedAt?: string | null }): string {
  if (!user.onboardedAt) return '/welcome'
  const raw = takeNextRoute() ?? getLastRoute(user.username) ?? '#/'
  return raw.startsWith('#') ? raw.slice(1) : raw
}

/** Nhãn nơi sẽ quay lại sau khi đăng nhập (sessionStorage ainovel.next). */
export function nextRouteLabel(): string | null {
  let raw: string | null = null
  try { raw = sessionStorage.getItem('ainovel.next') } catch { return null }
  if (!raw) return null
  const path = raw.replace(/^#/, '').split('?')[0]
  const seg = path.split('/').filter(Boolean)
  if (seg[0] === 'p' && seg[2] === 's' && seg[4] === 'work') return 'Workspace'
  if (seg[0] === 'p' && seg[2] === 's' && seg[4] === 'read') return 'Trình đọc'
  if (seg[0] === 'p' && seg[2] === 's' && seg[4] === 'activity') return 'Nhật ký tiến độ'
  if (seg[0] === 'p' && seg[1]) return 'Dự án'
  if (seg[0] === 'settings') return 'Tuỳ chỉnh'
  if (seg.length === 0) return 'Thư viện truyện'
  return null
}

/** 4 orb xếp cụm + quầng sáng (A1/A2/A3). */
export function OrbHero({ size = 'md', variant = 'default' }: { size?: 'md' | 'lg'; variant?: 'default' | 'error' | 'done' }) {
  const large = size === 'lg'
  const orbs: { agent: AgentKind; left: number; top: number; size: number; delay: number }[] = [
    { agent: 'architect', left: 0, top: 14, size: 34, delay: 0 },
    { agent: 'writer', left: 22, top: 0, size: 40, delay: -0.9 },
    { agent: 'editor', left: 50, top: 16, size: 32, delay: -1.8 },
    { agent: 'arbiter', left: 28, top: 34, size: 28, delay: -2.7 },
  ]
  return (
    <div aria-hidden="true" style={{ position: 'relative', width: 84, height: 64, flex: '0 0 auto' }}>
      <div style={{
        position: 'absolute', inset: -18, borderRadius: '50%',
        background: variant === 'error'
          ? 'radial-gradient(closest-side, var(--tint-err), transparent 75%)'
          : `radial-gradient(closest-side, ${variant === 'done' ? 'rgba(93,193,176,0.22)' : 'rgba(154,99,227,0.22)'}, rgba(90,87,227,0.10) 55%, transparent 75%)`,
        filter: 'blur(6px)',
      }} />
      {variant === 'error' ? (
        <span style={{
          position: 'absolute', left: 20, top: 10, width: 44, height: 44, borderRadius: '50%',
          background: 'var(--tint-err)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-err)',
        }}><Icon name="close-circle-bulk" size={26} /></span>
      ) : orbs.map((orb) => (
        <AgentOrb key={orb.agent} agent={orb.agent} size={large ? orb.size + 4 : orb.size} live
          style={{ position: 'absolute', left: orb.left, top: orb.top, animationDelay: `${orb.delay}s` }} />
      ))}
    </div>
  )
}

/** Header gọn của màn đăng nhập: brand + chip trạng thái máy chủ. Không có nav. */
export function AuthShell({ children }: { children: ReactNode }) {
  const { user } = useSession()
  const { data: library } = useResource(keys.library(), getLibrary, { enabled: !!user })
  const rootDisplay = library?.rootDisplay
  return (
    <div style={{ minHeight: '100vh', height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)', color: 'var(--color-fg)' }}>
      <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '14px 20px' }}>
        <BrandLockup size={22} studio />
        <div title="Noverse đang chạy" style={{
          display: 'flex', alignItems: 'center', gap: 8, height: 30, padding: '0 12px', borderRadius: 999,
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--color-fg-3)',
          maxWidth: '55vw', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>
          <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--color-ok)', boxShadow: '0 0 0 3px var(--tint-ok)', flex: '0 0 auto' }} />
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{rootDisplay ? `Lưu ở: ${rootDisplay}` : 'Máy chủ Noverse'}</span>
          <span style={{ color: 'var(--color-fg-5)' }}>·</span>
          <span>đang hoạt động</span>
        </div>
      </header>
      <main style={{ flex: 1, minHeight: 0, overflowY: 'auto', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div style={{ width: 'min(400px, 100%)', display: 'flex', flexDirection: 'column', gap: 22 }}>{children}</div>
      </main>
      <footer style={{ flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: '6px 18px', padding: '14px 20px 18px', fontSize: 12, color: 'var(--color-fg-4)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="mobile" size={14} />Điện thoại: quét mã QR từ máy tính đã đăng nhập</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="user-add" size={14} />Chưa có tài khoản? Nhờ quản trị viên tạo</span>
      </footer>
    </div>
  )
}

/** TextField dùng chung, giữ chiều cao và chữ 40px của artboard A1/A2/A3. */
export const AuthInput = forwardRef<HTMLInputElement, TextFieldProps & { invalid?: boolean; mono?: boolean }>(
  function AuthInput({ invalid, mono, style, ...rest }, ref) {
    return <TextField {...rest} ref={ref} aria-invalid={invalid || rest['aria-invalid']} className={mono ? 'mono' : undefined}
      style={{ height: 38, fontSize: 14, fontFamily: mono ? 'var(--font-mono)' : undefined, ...style }} />
  },
)

/** Ô chọn “Máy dùng chung” — tick khi chọn (Login.dc.html). */
export function CheckLine({ checked, onChange, children }: { checked: boolean; onChange: (v: boolean) => void; children: ReactNode }) {
  return (
    <Checkbox checked={checked} onChange={onChange} label={children} tone="mono" size={16}
      style={{ gap: 8, padding: 0, margin: 0, fontSize: 12, color: 'var(--color-fg-3)' }} />
  )
}

/** Dòng gợi ý nhỏ có icon (dùng ở footer màn auth). */
export function HintLine({ icon, children, tone }: { icon: string; children: ReactNode; tone?: string }) {
  return (
    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: tone ?? 'var(--color-fg-3)' }}>
      <Icon name={icon} size={15} style={{ color: 'var(--color-fg-4)' }} />{children}
    </span>
  )
}

/** Chip máy chủ trên màn điện thoại: đoán Wi-Fi nhà / Tailscale từ hostname. */
export function useServerKind(): { kind: 'wifi' | 'tailscale'; label: string } {
  return useMemo(() => {
    const host = typeof location === 'undefined' ? '' : location.hostname
    const tailscale = /^100\.(6[4-9]|[7-9]\d|1[01]\d|12[0-7])\./.test(host)
    return tailscale ? { kind: 'tailscale', label: 'Tailscale' } : { kind: 'wifi', label: 'Wi-Fi nhà' }
  }, [])
}

/** Loại địa chỉ từ link server trả: label Tailscale hoặc host 100.64.0.0/10 → Tailscale. */
export function linkKind(link: { label: string; url: string }): { kind: 'wifi' | 'tailscale'; kindLabel: string; sub: string } {
  let host = ''
  try { host = new URL(link.url).hostname } catch { host = link.label }
  const tailscale = /tailscale/i.test(link.label) || /^100\.(6[4-9]|[7-9]\d|1[01]\d|12[0-7])\./.test(host)
  return tailscale
    ? { kind: 'tailscale', kindLabel: 'Tailscale', sub: 'khi ra ngoài' }
    : { kind: 'wifi', kindLabel: 'Wi-Fi nhà', sub: 'khi ở nhà' }
}

/** SVG do server trả — chỉ cần cho nó lấp đầy khung trắng (đủ quiet zone). */
export function fillSvg(svg: string): string {
  return svg.includes('<svg ')
    ? svg.replace('<svg ', '<svg width="100%" height="100%" shape-rendering="crispEdges" ')
    : svg
}

/** Khung QR trắng 18px padding (Login.dc.html); dim khi mã hết hạn. */
export function QrBox({ svg, size = 300, radius = 18, dim = false, children }: {
  svg: string; size?: number; radius?: number; dim?: boolean; children?: ReactNode
}) {
  return (
    <div style={{
      position: 'relative', width: size, maxWidth: '100%', aspectRatio: '1 / 1', padding: 18,
      borderRadius: radius, background: 'white', boxSizing: 'border-box',
      boxShadow: '0 0 0 1px rgba(255,255,255,0.1), 0 20px 50px -20px rgba(0,0,0,0.8)',
    }}>
      <div aria-hidden="true" style={{ width: '100%', height: '100%', opacity: dim ? 0.25 : 1, filter: dim ? 'blur(3px)' : undefined }}
        dangerouslySetInnerHTML={{ __html: fillSvg(svg) }} />
      {children}
    </div>
  )
}

/** Điểm mạnh mật khẩu: 4 vạch + nhãn (09-auth-onboard.md §4). */
export function strengthOf(value: string): { score: number; color: string; label: string } {
  let score = 0
  if (value.length >= 8) {
    score = 1
    if (value.length >= 12) score++
    if (/[^a-zA-Z0-9]/.test(value)) score++
    if (/\d/.test(value) && /[a-zA-Z]/.test(value)) score++
  }
  const color = score >= 3 ? 'var(--color-ok)' : score === 2 ? 'var(--color-warn)' : 'var(--color-err)'
  const label = value.length === 0 ? 'Ít nhất 8 ký tự'
    : value.length < 8 ? `Cần ít nhất 8 ký tự (còn ${8 - value.length})`
    : ['', 'Yếu', 'Tạm được', 'Mạnh', 'Rất mạnh'][score]
  return { score, color, label }
}

export function StrengthMeter({ value, showLabel = true }: { value: string; showLabel?: boolean }) {
  const strength = strengthOf(value)
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <div aria-hidden="true" style={{ display: 'flex', gap: 4 }}>
        {[0, 1, 2, 3].map((index) => (
          <span key={index} style={{
            flex: 1, height: 3, borderRadius: 3,
            background: index < strength.score ? strength.color : 'var(--color-surface-6)',
          }} />
        ))}
      </div>
      {showLabel && (
        <span style={{ fontSize: 12, color: value.length > 0 && value.length < 8 ? 'var(--color-err)' : 'var(--color-fg-3)' }}>
          {strength.label}
        </span>
      )}
    </div>
  )
}
