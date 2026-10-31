import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, saccAcctMarker, saccEmptyTitleMarker, saccManualToggleMarker, saccManualLabelMarker } from '../settings.stylex'
// Bilibili trong thẻ Tài khoản nền tảng — bốn trạng thái của artboard:
//   · chưa đăng nhập (câu lợi ích + nút “Đăng nhập bằng mã QR”)
//   · mã QR (tên tài khoản · nhãn “Mã QR”, tự gia hạn mỗi ngày, “Gia hạn ngay”)
//   · cookie dán tay (nhãn xám, nhắc không tự gia hạn, “Đăng nhập bằng mã QR”)
//   · hết hạn (nhãn “Mã QR · hết hạn”, banner cam + “Quét mã QR” ngay trên banner)
// Bị giới hạn (state=paused) có banner “đang giới hạn yêu cầu từ mạng này”.
// “Xoá đăng nhập” luôn xác nhận cam trước DELETE. Cookie chỉ hiện dạng đã che.
import { useState } from 'react'
import { Badge, Banner, Button, Icon, TextArea, confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ApiError } from '../../../api/client'
import { deletePlatformLogin, putPlatformLogin, refreshPlatformLogin, type Platform } from '../../../api/platforms'
import { keys } from '../../../data/keys'
import { accountsCopy as t } from '../../../i18n/accounts'
import { fmtTime } from '../fmt'
import { QrLoginDialog } from './QrLogin'

/** “hôm nay 08:12” | “30/10 08:12”. */
function dayLabel(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const sameDay = d.toDateString() === new Date().toDateString()
  return sameDay
    ? `hôm nay ${fmtTime(iso)}`
    : `${d.toLocaleDateString('vi-VN', { day: '2-digit', month: '2-digit' })} ${fmtTime(iso)}`
}

const methodTag = (platform: Platform) => {
  if (platform.login.refreshError) return { label: t.biliMethodExpired, fg: 'var(--vv-warn-ink)' }
  if (platform.login.method === 'manual') return { label: t.biliMethodManual, fg: 'var(--color-fg-2)' }
  return { label: t.biliMethodQr, fg: 'var(--color-ok)' }
}

export function BilibiliLogin({ platform, mobile }: { platform: Platform; mobile: boolean }) {
  const [qrOpen, setQrOpen] = useState(false)
  const [manualOpen, setManualOpen] = useState(false)
  const [cookie, setCookie] = useState('')
  const [busy, setBusy] = useState(false)
  const [renewing, setRenewing] = useState(false)
  const [renewedAt, setRenewedAt] = useState<string>()

  const login = platform.login
  const saved = login.saved
  const isExpired = saved && login.refreshError != null
  const isManual = login.method === 'manual'
  const paused = platform.state === 'paused'
  const account = login.account ?? t.biliSubAccount
  const tag = methodTag(platform)

  const canRenew = saved && !isExpired && login.method === 'qr' && login.renewable
  const canQr = saved && !isExpired && (isManual || (login.method === 'qr' && !login.renewable))
  const showManual = !saved || isManual || isExpired
  const pausedAt = platform.pausedUntil ? fmtTime(platform.pausedUntil) : '…'

  async function saveCookie() {
    const value = cookie.trim()
    if (!value) { toast.warn(t.biliCookieEmpty); return }
    if (!value.includes('SESSDATA')) { toast.err(t.biliCookieInvalid); return }
    setBusy(true)
    try {
      await putPlatformLogin(platform.id, { cookie: value })
      setCookie('')
      setManualOpen(false)
      toast.ok(t.biliSaved)
    } catch (error) {
      const status = error instanceof ApiError ? error.status : 0
      toast.err(status === 401 ? t.biliCookieRejected : error instanceof Error ? error.message : t.biliSaving)
    } finally {
      await invalidate(keys.platforms())
      setBusy(false)
    }
  }

  async function renew() {
    setRenewing(true)
    try {
      await refreshPlatformLogin(platform.id)
      setRenewedAt(new Date().toISOString())
      toast.ok(t.biliRenewed)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.biliRenewing)
    } finally {
      await invalidate(keys.platforms())
      setRenewing(false)
    }
  }

  function clearLogin() {
    confirm({
      tone: 'warn',
      title: t.biliClearTitle,
      body: t.biliClearBody,
      cancelLabel: t.clearConfirmCancel,
      confirmLabel: t.biliClear,
      onConfirm: async () => {
        await deletePlatformLogin(platform.id)
        await invalidate(keys.platforms())
        toast.ok(t.biliCleared)
      },
    }).catch((error: unknown) => {
      toast.err(error instanceof Error ? error.message : t.biliRenewing)
    })
  }

  const subline = renewedAt
    ? t.biliSubRenewed(fmtTime(renewedAt))
    : isExpired
      ? t.biliSubExpired(fmtTime(login.lastRefreshAt))
      : isManual
        ? t.biliSubManual(login.masked ?? '')
        : [t.biliSubDaily(dayLabel(login.lastRefreshAt)), login.masked].filter(Boolean).join(' · ')

  // Artboard: banner bị chặn không có nút (chỉ clock + lời giải thích); banner
  // hết hạn của máy tính có nút “Quét mã QR”, điện thoại không (nút nằm ở hàng
  // tài khoản). Điện thoại căn icon dòng đầu.
  const sinceAt = platform.lastError?.at ? fmtTime(platform.lastError.at) : ''
  const blockedBanner = paused && (
    <Banner tone="error" role="status" icon="clock" title={mobile ? t.biliBlockedTitleMobile(platform.label) : t.biliBlockedTitle(platform.label)} align={mobile ? 'top' : 'center'}>
      {mobile
        ? t.biliBlockedBodyMobile(pausedAt)
        : platform.failureCount
          ? t.biliBlockedBody(platform.failureCount, pausedAt, sinceAt || undefined)
          : t.biliBlockedBodyMobile(pausedAt)}
    </Banner>
  )

  const expiredBanner = isExpired && (
    <Banner
      tone="error"
      icon="danger"
      role="alert"
      align={mobile ? 'top' : 'center'}
      title={t.biliExpiredTitle}
      action={mobile ? undefined : { label: t.biliRescan, onClick: () => setQrOpen(true) }}
    >
      {mobile ? t.biliExpiredBodyMobile(fmtTime(login.lastRefreshAt)) : t.biliExpiredBody(fmtTime(login.lastRefreshAt))}
    </Banner>
  )

  const dialog = (
    <QrLoginDialog
      platformId={platform.id}
      pausedUntil={paused ? platform.pausedUntil : undefined}
      open={qrOpen}
      mobile={mobile}
      onClose={() => { setQrOpen(false); void invalidate(keys.platforms()) }}
      onManual={() => { setQrOpen(false); setManualOpen(true) }}
    />
  )

  if (mobile) {
    return (
      <>
        {blockedBanner}
        {expiredBanner}
        <div className={`sacc-login ${stylex.props(settingsStyles.saccLogin).className}`}>
          <div className={`sacc-mrow ${stylex.props(settingsStyles.saccMrow).className}`}>
            <span className={`sacc-mrow__text ${stylex.props(settingsStyles.saccMrowText).className}`}>
              <span className={`sacc-mrow__k ${stylex.props(settingsStyles.saccMrowK).className}`}>
                {saved ? `${account} · ${isManual ? t.mBiliMethodManual : t.mBiliMethodQr}` : t.mBiliEmptyK}
              </span>
              <span className={`sacc-mrow__v ${stylex.props(settingsStyles.saccMrowV, (isExpired) && settingsStyles.saccMrowVWarn).className}`}>
                {!saved
                  ? t.mBiliEmptyV
                  : isExpired
                    ? t.mBiliExpiredV
                    : isManual
                      ? t.mBiliManualV
                      : t.mBiliDaily(fmtTime(login.lastRefreshAt))}
              </span>
            </span>
            {!saved && (
              <Button size="lg" variant="ghost" onClick={() => setQrOpen(true)}>{t.mBiliLogin}</Button>
            )}
            {canRenew && (
              <Button size="lg" variant="ghost" pending={renewing} pendingLabel={t.biliRenewing} onClick={() => void renew()}>
                {t.mBiliRenew}
              </Button>
            )}
            {canQr && (
              <Button size="lg" variant="ghost" onClick={() => setQrOpen(true)}>{t.mBiliScan}</Button>
            )}
            {isExpired && (
              <Button size="lg" variant="primary" onClick={() => setQrOpen(true)}>{t.mBiliRescan}</Button>
            )}
          </div>
        </div>
        {dialog}
      </>
    )
  }

  return (
    <>
      {blockedBanner}
      {expiredBanner}
      <div className={`sacc-login ${stylex.props(settingsStyles.saccLogin).className}`}>
        {saved
          ? (
            <div className={`sacc-acct ${stylex.props(saccAcctMarker, settingsStyles.saccAcct).className}`} data-dim={isExpired ? 'true' : undefined}>
              <span className={`sacc-acct__tile ${stylex.props(settingsStyles.saccAcctTile).className}`} aria-hidden="true">{account.slice(0, 1).toUpperCase()}</span>
              <span className={`sacc-acct__text ${stylex.props(settingsStyles.saccAcctText).className}`}>
                <span className={`sacc-acct__row ${stylex.props(settingsStyles.saccAcctRow).className}`}>
                  {account}
                  <Badge tone="neutral" fg={tag.fg} bg={isExpired ? 'var(--vv-warn-bg)' : undefined}
                    style={{ height: 20, fontSize: 11 }}>{tag.label}</Badge>
                </span>
                <span className={`sacc-acct__sub ${stylex.props(settingsStyles.saccAcctSub).className}`}>{subline}</span>
              </span>
              {canRenew && (
                <Button size="sm" icon={<Icon name="refresh" size={13} />} pending={renewing} pendingLabel={t.biliRenewing} onClick={() => void renew()}>
                  {t.biliRenew}
                </Button>
              )}
              {canQr && (
                <Button size="sm" icon={<Icon name="scan-line" size={14} />} onClick={() => setQrOpen(true)}>
                  {t.biliQr}
                </Button>
              )}
              <Button size="sm" variant="dsoft" onClick={clearLogin}>{t.biliClear}</Button>
            </div>
          )
          : (
            <div className={`sacc-empty ${stylex.props(settingsStyles.saccEmpty).className}`}>
              <span className={`sacc-empty__text ${stylex.props(settingsStyles.saccEmptyText).className}`}>
                <span className={`sacc-empty__title ${stylex.props(saccEmptyTitleMarker, settingsStyles.saccEmptyTitle).className}`}>
                  {t.biliEmptyTitle} <span {...stylex.props(settingsStyles.SaccEmptyTitleSpan)}>{t.biliEmptyOptional}</span>
                </span>
                <span className={`sacc-empty__body ${stylex.props(settingsStyles.saccEmptyBody).className}`}>{t.biliEmptyBody}</span>
              </span>
              <Button size="sm" variant="primary" icon={<Icon name="scan-line" size={14} />} onClick={() => setQrOpen(true)}>
                {t.biliQr}
              </Button>
            </div>
          )}

        {isManual && (
          <span className={`sacc-help ${stylex.props(settingsStyles.saccHelp).className}`}>
            <Icon name="info" size={14} className="sacc-ic-top" xstyle={[settingsStyles.saccIcTop]} />
            <span>{t.biliManualHelp}</span>
          </span>
        )}

        {showManual && (
          <div className={`sacc-manual ${stylex.props(settingsStyles.saccManual).className}`}>
            <Button
              variant="quiet"
              size="sm"
              className={`sacc-manual__toggle ${stylex.props(saccManualToggleMarker).className}`} xstyle={[settingsStyles.saccManualToggle]}
              aria-expanded={manualOpen}
              icon={<Icon name="arrow-d" size={14} className="sacc-manual__chev" xstyle={[settingsStyles.saccManualChev]} />}
              onClick={() => setManualOpen((open) => !open)}
            >
              {isManual ? t.biliManualToggleOther : t.biliManualToggle}
            </Button>
            {manualOpen && (
              <div className={`sacc-manual__panel ${stylex.props(settingsStyles.saccManualPanel).className}`}>
                <span className={`sacc-manual__label ${stylex.props(saccManualLabelMarker, settingsStyles.saccManualLabel).className}`}>
                  <b {...stylex.props(settingsStyles.SaccManualLabelB)}>{t.biliManualLabel}</b>
                  <span {...stylex.props(settingsStyles.SaccManualLabelSpan)}>{t.biliManualHint}</span>
                </span>
                <TextArea
                  rows={2}
                  showCount={false}
                  className="mono"
                  placeholder={t.biliManualPlaceholder}
                  aria-label={t.biliManualLabel}
                  value={cookie}
                  onChange={(e) => setCookie(e.target.value)}
                  style={{ fontSize: 12.5, lineHeight: 1.6, paddingBlock: 'var(--vv-s-8)', paddingInline: 'var(--vv-s-12)', resize: 'vertical' }}
                />
                <span className={`sacc-manual__foot ${stylex.props(settingsStyles.saccManualFoot).className}`}>
                  <Button size="sm" pending={busy} pendingLabel={t.biliSaving} disabled={!cookie.trim()} onClick={() => void saveCookie()}>
                    {t.biliSave}
                  </Button>
                </span>
              </div>
            )}
          </div>
        )}
      </div>
      {dialog}
    </>
  )
}
