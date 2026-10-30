// Tài khoản nền tảng — artboard Settings section=accounts (máy tính) và
// MSettings screen=accounts (điện thoại). Wire: internal/web/platforms.go
//   platform {id,label,uses,kinds,targetHeight,login{type,saved,masked,
//   account,browserProfile,method,renewable,lastRefreshAt,refreshError},
//   state,pausedUntil,pauseReason,failureCount,lastError,usage{channels,
//   videos,rivals}}.
//   - Thẻ: ô chữ cái 36 + tên + câu “dùng cho” + chất lượng/số nguồn · link +
//     nhãn trạng thái; banner sự cố nằm ngay dưới đầu thẻ.
//   - YouTube: chọn hồ sơ trình duyệt (lưu ngay khi đổi, “Tắt” = xoá đăng nhập),
//     banner bot “Thử lại” → POST login/check.
//   - Twitch: chỉ đọc công khai, không đăng nhập.
//   - Bilibili: QR / cookie dán tay / hết hạn / chưa đăng nhập — xem BilibiliLogin.
//   - Cuối danh sách: thẻ nét đứt “Nền tảng khác” + ghi chú an toàn.
import { Badge, Button, Icon, Spinner } from '@noverse/ui'
import './accounts.css'
import { ApiError } from '../../../api/client'
import { listPlatforms, type Platform } from '../../../api/platforms'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { accountsCopy as t } from '../../../i18n/accounts'
import { paths } from '../../../router'
import { BilibiliLogin } from './BilibiliLogin'
import { YouTubeLogin, YouTubeLoginRow } from './YouTubeLogin'

const fmtUntil = (iso: string): string => {
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

const USE_LABEL: Record<string, string> = {
  competitor: t.useCompetitor,
  backdrop: t.useBackdrop,
}

const KIND_LABEL: Record<string, string> = {
  clip: t.kindClip,
  highlight: t.kindHighlight,
  vod: t.kindVod,
  video: t.kindVideo,
}

/** Câu “dùng cho” — câu riêng của nền tảng, nền tảng mới dựng từ uses/kinds.
 *  Điện thoại có câu ngắn hơn cho YouTube (MSettings · plats[].uses). */
export function useLine(p: Platform, mobile = false): string {
  const own = (mobile ? t.platformUsesMobile[p.id] : undefined) ?? t.platformUses[p.id]
  if (own) return own
  const uses = p.uses.map((u) => USE_LABEL[u]).filter(Boolean)
  if (uses.length === 0) return ''
  const kinds = p.kinds.map((k) => KIND_LABEL[k]).filter(Boolean)
  return kinds.length > 0 ? `${uses[0]}: ${kinds.join(', ')}` : uses[0]
}

/** “Tải 1080p · 1 kênh, 2 video lẻ” | “Tải 360p + phụ đề · 5 đối thủ”. */
function metaLine(p: Platform): string {
  const lead = t.platformMetaLead[p.id] ?? (p.targetHeight ? t.metaQuality(p.targetHeight) : '')
  const counts: string[] = []
  if (p.usage.channels) counts.push(t.metaChannels(p.usage.channels))
  if (p.usage.rivals) counts.push(t.metaRivals(p.usage.rivals))
  if (p.usage.videos) counts.push(t.metaVideos(p.usage.videos))
  return [lead, counts.join(', ')].filter(Boolean).join(' · ')
}

/** Ô chữ cái của thẻ: bảng riêng, nền tảng mới rơi về hai ký tự đầu của nhãn. */
const initial = (id: string, label: string): string =>
  t.platformIni[id] ?? label.replace(/[^A-Za-zÀ-ỹ]/g, '').slice(0, 2).toUpperCase()

/** “Cần cookie” chỉ dành cho YouTube báo bot; trạng thái needs_login khác vẫn “Sẵn sàng”. */
const needsCookie = (p: Platform): boolean => p.lastError?.code === 'cookies_failed'

const expired = (p: Platform): boolean => p.login.saved && p.login.refreshError != null

/** Nhãn trạng thái góc phải thẻ: Sẵn sàng · Tạm chặn tới HH:MM · Cần đăng nhập lại · Cần cookie. */
function StatusTag({ platform, mobile }: { platform: Platform; mobile: boolean }) {
  if (platform.state === 'paused') {
    const at = platform.pausedUntil ? fmtUntil(platform.pausedUntil) : '…'
    return <Badge tone="err" fg="var(--color-err)" dot={!mobile}>{t.statePaused(at)}</Badge>
  }
  if (expired(platform)) {
    return <Badge tone="err" fg="var(--color-err)" dot={!mobile}>{mobile ? t.stateExpired : t.stateRelogin}</Badge>
  }
  if (needsCookie(platform)) {
    return <Badge tone="err" fg="var(--color-err)" dot={!mobile}>{t.stateNeedCookie}</Badge>
  }
  return <Badge tone="ok" dot={!mobile}>{t.stateReady}</Badge>
}

function TwitchLogin() {
  return (
    <div className="sacc-none">
      <Icon name="info" size={14} />
      <span>{t.twitchNoLogin}</span>
    </div>
  )
}

/** Điện thoại: hàng “Không cần đăng nhập / Chỉ đọc clip và video công khai”. */
function TwitchRow() {
  return (
    <div className="sacc-login">
      <div className="sacc-mrow">
        <span className="sacc-mrow__text">
          <span className="sacc-mrow__k">{t.twitchLoginK}</span>
          <span className="sacc-mrow__v">{t.twitchLoginV}</span>
        </span>
      </div>
    </div>
  )
}

export function PlatformCard({ platform, mobile }: { platform: Platform; mobile: boolean }) {
  const uses = useLine(platform, mobile)
  const meta = metaLine(platform)
  const link = mobile
    ? undefined
    : platform.id === 'youtube'
      ? { href: `#${paths.explore()}`, label: t.platformLink.youtube }
      : platform.id === 'twitch' || platform.id === 'bilibili'
        ? { href: `#${paths.library('backdrops')}`, label: t.platformLink[platform.id] }
        : undefined
  // Viền cam: đăng nhập hết hạn hoặc YouTube báo bot (01-ia-flows W13). Riêng
  // bị chặn thì artboard để viền trung tính — banner cam đã đủ tín hiệu.
  const broken = expired(platform) || needsCookie(platform)
  return (
    <section className={`card sacc-card${broken ? ' sacc-card--issue' : ''}`} aria-label={platform.label}>
      <div className="sacc-head">
        <span className="sacc-tile" aria-hidden="true">{initial(platform.id, platform.label)}</span>
        <div className="sacc-head__text">
          <h2 className="sacc-name">{platform.label}</h2>
          {uses && <span className="sacc-uses">{uses}</span>}
          {meta && (
            <span className="sacc-meta">
              {meta}{link && <> · <a href={link.href}>{link.label}</a></>}
            </span>
          )}
        </div>
        <StatusTag platform={platform} mobile={mobile} />
      </div>
      {platform.login.type === 'browser_cookie' && (mobile
        ? <YouTubeLoginRow platform={platform} />
        : <YouTubeLogin platform={platform} />)}
      {platform.login.type === 'none' && (mobile ? <TwitchRow /> : <TwitchLogin />)}
      {(platform.login.type === 'qr' || platform.login.type === 'cookie_string') && (
        <BilibiliLogin platform={platform} mobile={mobile} />
      )}
    </section>
  )
}

function MorePlatforms() {
  return (
    <div className="sacc-more">
      <Icon name="add-square" size={18} />
      <span className="sacc-more__text">
        <span className="sacc-more__title">{t.moreTitle}</span>
        <span className="sacc-more__body">{t.moreBody}</span>
      </span>
      <Button size="sm" onClick={() => { window.location.hash = paths.library('backdrops') }}>{t.moreLink}</Button>
    </div>
  )
}

function SecureNote() {
  return (
    <span className="sacc-secure">
      <Icon name="shield-tick" size={14} />
      <span>{t.secureNote}</span>
    </span>
  )
}

export function AccountsSection() {
  const mobile = useMediaQuery('(max-width: 767px)')
  const platforms = useResource(keys.platforms(), ({ signal }) => listPlatforms(signal))
  const offline = platforms.error instanceof ApiError && platforms.error.status === 0

  return (
    <div className="sacc-section">
      {mobile && <p className="sacc-mlead">{t.mLead}</p>}
      {offline && <div className="sacc-fail" role="status">{t.offline}</div>}
      {platforms.error != null && !offline && <div className="sacc-fail" role="alert">{t.loadFail}</div>}
      {platforms.data === undefined && !platforms.error && (
        <div className="sacc-note"><Spinner size={13} color="var(--color-fg-3)" /> {t.loading}</div>
      )}
      <div className="sacc-list">
        {(platforms.data ?? []).map((p) => <PlatformCard key={p.id} platform={p} mobile={mobile} />)}
        {!mobile && <MorePlatforms />}
        {!mobile && <SecureNote />}
      </div>
      {mobile && <p className="sacc-mfoot">{t.mFoot}</p>}
    </div>
  )
}

export default AccountsSection
