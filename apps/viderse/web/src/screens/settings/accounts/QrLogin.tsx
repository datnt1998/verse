import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, saccQrBoxMarker, saccQrProgressMarker, saccQrTimerrowMarker, saccQrStepDoneMarker, saccQrStepNowMarker } from '../settings.stylex'
// QrLogin — đăng nhập Bilibili bằng mã QR (§24.1; artboard Settings qr=… và
// MSettings screen=accounts sheet mã QR).
// Wire: POST …/login/qr → {qrcodeKey, qrImage, loginUrl, expiresAt}; poll
// GET …/login/qr/:key → {status: waiting|scanned|expired|success|rejected,
// account?}. Poll 2 s CHỈ khi hộp đang mở; đóng/unmount → dừng ngay.
// Đếm ngược 1 s từ expiresAt (hết giờ phía client → expired, không chờ server).
//
// Trạng thái hiển thị (artboard Settings · QS):
//   loading  — skeleton, “Đang tạo mã…” xám
//   waiting  — mã rõ, “Đang chờ quét” xanh + spinner, thanh đếm
//   scanned  — mã mờ 14%, lớp đè tick, “Đã quét…” vàng, đếm giờ, bước 1–2 xong
//   expired  — mã mờ 10% + blur, lớp đè clock, “Mã đã hết hạn sau 3 phút.”
//   rejected — mã mờ + blur, lớp đè close cam, banner từ chối, bước 1–2 vẫn xong
//   network  — không có mã (ô trắng), lớp đè globe, banner “Chưa kết nối được”
//   busy     — như network nhưng lời “đang bận” (409/429)
//   success  — mã mờ, lớp đè tick xanh, “Đã đăng nhập: …”, bước 1–3 xong
// Lỗi tạo mã KHÔNG phải người dùng huỷ: 409/429 → busy, còn lại (mất mạng,
// 5xx) → network. Chỉ poll trả status=rejected mới là “Bilibili từ chối”.
import { useEffect, useState } from 'react'
import { Banner, Button, Dialog, Icon, Sheet, Spinner, toast, type IconName } from '@noverse/ui'
import { ApiError } from '../../../api/client'
import { createPlatformQr, getPlatformQr, type QrPoll, type QrSession } from '../../../api/platforms'
import { accountsCopy as t } from '../../../i18n/accounts'
import { fmtTime } from '../fmt'
import { sequentialPoll } from '../poll'

export type QrPhase = 'loading' | 'waiting' | 'scanned' | 'success' | 'expired' | 'rejected' | 'network' | 'busy'

const PHASE: Record<QrPoll['status'], QrPhase> = {
  waiting: 'waiting',
  scanned: 'scanned',
  success: 'success',
  expired: 'expired',
  rejected: 'rejected',
}

export const QR_SECONDS = 180

/** Giây còn lại tới expiresAt. */
const left = (expiresAt: string | undefined, now: number) =>
  expiresAt ? Math.max(0, Math.floor((new Date(expiresAt).getTime() - now) / 1000)) : 0

const clock = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`

/** Phiên QR: tạo khi mở, đúng MỘT vòng poll 2 s chừng nào open; đóng/kết thúc
 *  → huỷ ngay (kể cả unmount). */
export function useQrSession(platformId: string, open: boolean) {
  const [session, setSession] = useState<QrSession | null>(null)
  const [phase, setPhase] = useState<QrPhase>('loading')
  const [account, setAccount] = useState<string | undefined>(undefined)
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    if (!open) return
    let stopPoll: (() => void) | undefined
    let dead = false
    setSession(null)
    setPhase('loading')
    setAccount(undefined)

    const start = async () => {
      let created: QrSession
      try {
        created = await createPlatformQr(platformId)
      } catch (error) {
        if (dead) return
        const status = error instanceof ApiError ? error.status : 0
        // Huỷ mã không phải lỗi mạng: 409/429 = đang có phiên khác; còn lại là
        // không gọi được Bilibili (mất mạng, 5xx) → banner “Chưa kết nối được”.
        setPhase(status === 409 || status === 429 ? 'busy' : 'network')
        return
      }
      if (dead) return
      setSession(created)
      setPhase('waiting')
      // Poll tuần tự (một request một lúc, abort khi đóng); hết expiresAt theo
      // đồng hồ máy → dừng hẳn, không chờ server xác nhận.
      let misses = 0
      stopPoll = sequentialPoll(2000, async (signal) => {
        if (Date.now() >= new Date(created.expiresAt).getTime()) {
          setPhase('expired')
          return false
        }
        let next: QrPoll
        try {
          next = await getPlatformQr(platformId, created.qrcodeKey, signal)
        } catch {
          if (signal.aborted) return false
          misses += 1
          if (misses >= 5) {
            // Mất kết nối kéo dài → đưa lỗi ra UI thay vì poll mãi.
            setPhase('network')
            return false
          }
          return true
        }
        misses = 0
        if (next.account) setAccount(next.account)
        setPhase(PHASE[next.status])
        return next.status === 'waiting' || next.status === 'scanned'
      })
    }
    void start()
    return () => { dead = true; stopPoll?.() }
  }, [open, platformId, nonce])

  return { session, phase, account, retry: () => setNonce((n) => n + 1) }
}

/** Màu lớp đè + icon của ô mã (artboard: quét=nền đen, xong=xanh đậm,
 *  từ chối=cam cháy, hết hạn/lỗi=xám). */
const OVERLAY: Partial<Record<QrPhase, { icon: string; text: string; ring: string }>> = {
  scanned: { icon: 'tick', text: t.qrOverlayScanned, ring: 'scanned' },
  expired: { icon: 'clock', text: t.qrOverlayExpired, ring: 'muted' },
  rejected: { icon: 'close', text: t.qrOverlayRejected, ring: 'rejected' },
  network: { icon: 'global', text: t.qrOverlayError, ring: 'muted' },
  busy: { icon: 'global', text: t.qrOverlayError, ring: 'muted' },
  success: { icon: 'tick', text: t.qrOverlaySuccess, ring: 'success' },
}

/** Chữ dòng trạng thái + màu (artboard: chờ xanh, đã quét vàng, xong xanh lá,
 *  còn lại xám). */
const STATUS_COLOR: Partial<Record<QrPhase, string>> = {
  loading: 'var(--color-fg-3)',
  waiting: 'var(--color-info)',
  scanned: 'var(--color-warn)',
  success: 'var(--color-ok)',
  expired: 'var(--color-fg-3)',
}

const RETRYABLE: Record<QrPhase, boolean> = {
  loading: false,
  waiting: false,
  scanned: false,
  expired: true,
  rejected: true,
  network: true,
  busy: true,
  success: false,
}

/** Ô mã 200/196 px + lớp đè theo trạng thái. Mã mờ (và blur 2 px) khi đã
 *  quét/hết hạn/từ chối/xong; lỗi tạo mã thì ô trắng không skeleton. */
function QrPlate({ session, phase, mobile }: { session: QrSession | null; phase: QrPhase; mobile?: boolean }) {
  const overlay = OVERLAY[phase]
  const dim = phase === 'scanned' || phase === 'rejected' || phase === 'success'
  const hideCode = phase === 'loading' || phase === 'network' || phase === 'busy'
  const imgStyle =
    phase === 'expired' ? { opacity: 0.1, filter: 'blur(2px)' }
    : phase === 'rejected' ? { opacity: 0.1, filter: 'blur(2px)' }
    : dim ? { opacity: 0.14 }
    : undefined
  const size = mobile ? 168 : 172
  return (
    <div className={`sacc-qr__box ${stylex.props(saccQrBoxMarker, settingsStyles.saccQrBox).className}`}>
      {phase === 'loading' && <div className={`sacc-qr__skel ${stylex.props(settingsStyles.saccQrSkel).className}`} aria-hidden="true" />}
      {!hideCode && session?.qrImage && (
        <img src={session.qrImage} alt="" width={size} height={size} style={imgStyle} {...stylex.props(settingsStyles.SaccQrBoxImg)} />
      )}
      {overlay && (
        <div className={`sacc-qr__overlay ${stylex.props(settingsStyles.saccQrOverlay).className}`}>
          <span className={`sacc-qr__ovicon ${stylex.props(settingsStyles.saccQrOvicon, ((overlay.ring) === "scanned") && settingsStyles.saccQrOviconScanned, ((overlay.ring) === "success") && settingsStyles.saccQrOviconSuccess, ((overlay.ring) === "rejected") && settingsStyles.saccQrOviconRejected, ((overlay.ring) === "muted") && settingsStyles.saccQrOviconMuted).className}`} aria-hidden="true">
            <Icon name={overlay.icon} size={24} />
          </span>
          <span className={`sacc-qr__ovtext ${stylex.props(settingsStyles.saccQrOvtext).className}`}>{overlay.text}</span>
        </div>
      )}
    </div>
  )
}

/** Ô trạng thái dưới ba bước: chờ quét · đã quét · hết hạn · đã đăng nhập. */
function QrStatus({ phase, account, extra, mobile }: { phase: QrPhase; account?: string; extra?: string; mobile?: boolean }) {
  if (phase === 'rejected' || phase === 'network' || phase === 'busy') return null
  const text =
    phase === 'loading' ? t.qrStatusLoading
    : phase === 'waiting' ? t.qrStatusWaiting
    : phase === 'scanned' ? (mobile ? t.qrStatusScannedMobile : t.qrStatusScanned)
    : phase === 'expired' ? t.qrStatusExpired
    : t.qrStatusSuccess(account ?? '…')
  const color = STATUS_COLOR[phase] ?? 'var(--color-fg-3)'
  const spinner = phase === 'loading' || phase === 'waiting'
  return (
    <div role="status" aria-live="polite" className={`sacc-qr__status ${stylex.props(settingsStyles.saccQrStatus).className}`}>
      <span className={`sacc-qr__statusline ${stylex.props(settingsStyles.saccQrStatusline).className}`} style={{ color }}>
        {spinner
          ? <Spinner size={13} color={color} />
          : <span className={`sacc-qr__stdot ${stylex.props(settingsStyles.saccQrStdot).className}`} style={{ background: color }} aria-hidden="true" />}
        {text}
      </span>
      {extra && <span className={`sacc-qr__extra ${stylex.props(settingsStyles.saccQrExtra).className}`}>{extra}</span>}
    </div>
  )
}

/** Ba bước; bước đã xong hiện tick trên nền xanh 12%, bước đang làm nền vàng. */
function QrSteps({ phase, mobile }: { phase: QrPhase; mobile?: boolean }) {
  const steps = mobile ? t.qrStepsMobile : t.qrSteps
  const done = phase === 'success' ? 3 : phase === 'scanned' || phase === 'rejected' ? 2 : 0
  return (
    <ol className={`sacc-qr__steps ${stylex.props(settingsStyles.saccQrSteps).className}`}>
      {steps.map((step, i) => (
        <li
          key={step}
          className={`sacc-qr__step ${stylex.props((((i < done) && (i === done && phase === 'scanned')) || ((i < done) && (!(i === done && phase === 'scanned')))) && saccQrStepDoneMarker, (((i < done) && (i === done && phase === 'scanned')) || ((!(i < done)) && (i === done && phase === 'scanned'))) && saccQrStepNowMarker, settingsStyles.saccQrStep, (((i < done) && (i === done && phase === 'scanned')) || ((i < done) && (!(i === done && phase === 'scanned')))) && settingsStyles.saccQrStepDone, (((i < done) && (i === done && phase === 'scanned')) || ((!(i < done)) && (i === done && phase === 'scanned'))) && settingsStyles.saccQrStepNow).className}`}
        >
          <span className={`sacc-qr__stepnum ${stylex.props(settingsStyles.saccQrStepnum).className}`} aria-hidden="true">
            {i < done ? <Icon name="tick" size={13} /> : i + 1}
          </span>
          <span>{step}</span>
        </li>
      ))}
    </ol>
  )
}

function TimerRow({ session, now }: { session: QrSession | null; now: number }) {
  const secs = left(session?.expiresAt, now)
  return (
    <>
      <span
        className={`sacc-qr__progress ${stylex.props(saccQrProgressMarker, settingsStyles.saccQrProgress).className}`}
        role="progressbar"
        aria-label={t.qrTimerLabel}
        aria-valuemin={0}
        aria-valuemax={QR_SECONDS}
        aria-valuenow={secs}
      >
        <span style={{ width: `${Math.min(100, (secs / QR_SECONDS) * 100)}%` }} {...stylex.props(settingsStyles.SaccQrProgressSpan)} />
      </span>
      <span className={`sacc-qr__timer-label ${stylex.props(settingsStyles.saccQrTimerLabel).className}`}>{t.qrTimer(clock(secs))}</span>
    </>
  )
}

/** Thân hộp mã — Dialog máy tính (kèm hàng nút + ghi chú) hoặc Sheet điện thoại. */
export function QrBody({ session, phase, account, mobile, pausedUntil, onClose, onManual, retry, onDone, onExpire }: {
  session: QrSession | null
  phase: QrPhase
  account?: string
  mobile?: boolean
  pausedUntil?: string
  onClose: () => void
  onManual: () => void
  retry: () => void
  onDone: () => void
  /** Máy đếm hết giờ: báo lên hộp để hàng nút cũng chuyển sang “Tạo mã mới”. */
  onExpire?: () => void
}) {
  const [now, setNow] = useState(() => Date.now())
  const counting = phase === 'waiting' || phase === 'scanned'

  useEffect(() => {
    if (!counting) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [counting])

  const secs = left(session?.expiresAt, now)
  const expiredByClock = counting && session?.expiresAt != null && secs === 0
  const eff: QrPhase = expiredByClock ? 'expired' : phase
  useEffect(() => { if (expiredByClock) onExpire?.() }, [expiredByClock, onExpire])
  const banner: { icon: IconName; title: string; body: string } | null =
    eff === 'rejected' ? { icon: 'danger', title: t.qrRejectedTitle, body: t.qrRejectedBody }
    : eff === 'network' ? { icon: 'global', title: t.qrErrorTitle, body: t.qrErrorBody }
    : eff === 'busy' ? { icon: 'global', title: t.qrBusyTitle, body: t.qrBusyBody }
    : null
  // Artboard: thẻ “trên chính máy này?” chỉ có ở trạng thái chờ quét.
  const showSame = mobile && session?.loginUrl != null && eff === 'waiting'
  const extra = eff === 'success'
    ? (pausedUntil
        ? (mobile ? t.qrSuccessExtraBlockedMobile(fmtTime(pausedUntil)) : t.qrSuccessExtraBlocked(fmtTime(pausedUntil)))
        : (mobile ? t.qrSuccessExtraMobile : t.qrSuccessExtra))
    : undefined

  const bannerEl = banner && (
    <Banner tone="error" role="alert" icon={banner.icon} align="top" title={banner.title}>{banner.body}</Banner>
  )

  if (mobile) {
    return (
      <div className={`sacc-qr sacc-qr--sheet ${stylex.props(settingsStyles.saccQr, settingsStyles.saccQrSheet).className}`}>
        <div className={`sacc-qr__left ${stylex.props(settingsStyles.saccQrLeft).className}`}>
          <QrPlate session={session} phase={eff} mobile />
          {counting && !expiredByClock && (
            <span className={`sacc-qr__timerrow ${stylex.props(saccQrTimerrowMarker, settingsStyles.saccQrTimerrow).className}`}>
              <TimerRow session={session} now={now} />
            </span>
          )}
        </div>
        <QrStatus phase={eff} account={account} extra={extra} mobile />
        {bannerEl}
        <QrSteps phase={eff} mobile />
        {showSame && (
          <div className={`sacc-qr__same ${stylex.props(settingsStyles.saccQrSame).className}`}>
            <Icon name="mobile" size={20} className="sacc-qr__same-ic" xstyle={[settingsStyles.saccQrSameIc]} />
            <span className={`sacc-qr__same-text ${stylex.props(settingsStyles.saccQrSameText).className}`}>
              <span className={`sacc-qr__same-title ${stylex.props(settingsStyles.saccQrSameTitle).className}`}>{t.qrSameTitle}</span>
              <span className={`sacc-qr__same-body ${stylex.props(settingsStyles.saccQrSameBody).className}`}>{t.qrSameBody}</span>
            </span>
            <Button size="lg" onClick={() => { window.location.href = session.loginUrl }}>{t.qrSameAct}</Button>
          </div>
        )}
      </div>
    )
  }

  return (
    <>
      <div className={`sacc-qr ${stylex.props(settingsStyles.saccQr).className}`}>
        <div className={`sacc-qr__left ${stylex.props(settingsStyles.saccQrLeft).className}`}>
          <QrPlate session={session} phase={eff} />
          {counting && !expiredByClock && (
            <span className={`sacc-qr__timerrow ${stylex.props(saccQrTimerrowMarker, settingsStyles.saccQrTimerrow).className}`}>
              <TimerRow session={session} now={now} />
            </span>
          )}
        </div>
        <div className={`sacc-qr__right ${stylex.props(settingsStyles.saccQrRight).className}`}>
          <QrSteps phase={eff} />
          <QrStatus phase={eff} account={account} extra={extra} />
        </div>
      </div>
      {bannerEl}
      <div className={`sacc-qr__actions ${stylex.props(settingsStyles.saccQrActions).className}`}>
        <Button variant="quiet" size="sm" onClick={onManual} style={{ padding: '0 var(--vv-s-4)' }}>
          {t.qrManualLink}
        </Button>
        <span className={`sacc-qr__actions-end ${stylex.props(settingsStyles.saccQrActionsEnd).className}`}>
          {eff !== 'success' && <Button onClick={onClose}>{t.qrClose}</Button>}
          {RETRYABLE[eff] && (
            <Button variant="primary" icon={<Icon name="refresh" size={15} />} onClick={retry}>{t.qrNew}</Button>
          )}
          {eff === 'success' && <Button variant="primary" onClick={onDone}>{t.qrDone}</Button>}
        </span>
      </div>
      <span className={`sacc-qr__hint ${stylex.props(settingsStyles.saccQrHint).className}`}><Icon name="shield-tick" size={14} className="sacc-ic-top" xstyle={[settingsStyles.saccIcTop]} />{t.qrHint}</span>
    </>
  )
}

/** Hộp đăng nhập QR — Dialog 620 px trên máy tính, Sheet trên điện thoại. */
export function QrLoginDialog({ platformId, pausedUntil, open, mobile, onClose, onManual }: {
  platformId: string
  pausedUntil?: string
  open: boolean
  mobile: boolean
  onClose: () => void
  onManual: () => void
}) {
  const { session, phase, account, retry } = useQrSession(platformId, open)
  const [clockExpired, setClockExpired] = useState(false)
  useEffect(() => { if (phase === 'waiting' || phase === 'scanned') setClockExpired(false) }, [phase])
  const eff: QrPhase = clockExpired ? 'expired' : phase
  const finish = () => {
    toast.ok(t.biliSavedAccount(account ?? '…'))
    onClose()
  }

  if (mobile) {
    return (
      <Sheet
        open={open}
        onClose={onClose}
        label={t.qrTitleMobile}
        height="auto"
        style={{
          maxHeight: '86%',
          background: 'var(--color-card)',
          borderTop: '1px solid var(--line-2)',
          borderRadius: 'var(--vv-r-22) var(--vv-r-22) 0 0',
        }}
      >
        <div className={`sacc-sheet__head ${stylex.props(settingsStyles.saccSheetHead).className}`}>
          <span className={`sacc-sheet__title ${stylex.props(settingsStyles.saccSheetTitle).className}`}>{t.qrTitleMobile}</span>
          <span className={`sacc-sheet__sub ${stylex.props(settingsStyles.saccSheetSub).className}`}>{t.qrSubMobile}</span>
        </div>
        <div className={`sacc-sheet__body ${stylex.props(settingsStyles.saccSheetBody).className}`}>
          <QrBody
            session={session} phase={phase} account={account} mobile pausedUntil={pausedUntil}
            onClose={onClose} onManual={onManual} retry={retry} onDone={finish} onExpire={() => setClockExpired(true)}
          />
        </div>
        <div className={`sacc-sheet__foot ${stylex.props(settingsStyles.saccSheetFoot).className}`}>
          {RETRYABLE[eff] && (
            <Button size="xl" style={{ width: '100%' }} variant="primary" onClick={retry}>{t.qrNew}</Button>
          )}
          {eff === 'success' && <Button size="xl" style={{ width: '100%' }} variant="primary" onClick={finish}>{t.qrDone}</Button>}
          <span className={`sacc-sheet__help ${stylex.props(settingsStyles.saccSheetHelp).className}`}>{t.qrManualFootMobile}</span>
        </div>
      </Sheet>
    )
  }

  if (!open) return null
  return (
    <Dialog onClose={onClose} label={t.qrTitle} width={620} padBody={false} scrimStyle={settingsStyles.qrScrim}>
      <div className={`sacc-qr-dialog ${stylex.props(settingsStyles.saccQrDialog).className}`}>
        <div className={`sacc-qr-head ${stylex.props(settingsStyles.saccQrHead).className}`}>
          <span className={`sacc-qr-head__icon ${stylex.props(settingsStyles.saccQrHeadIcon).className}`}><Icon name="scan-line" size={20} /></span>
          <span className={`sacc-qr-head__text ${stylex.props(settingsStyles.saccQrHeadText).className}`}>
            <h2 className={`sacc-qr-head__title ${stylex.props(settingsStyles.saccQrHeadTitle).className}`}>{t.qrTitle}</h2>
            <p className={`sacc-qr-head__sub ${stylex.props(settingsStyles.saccQrHeadSub).className}`}>{t.qrSub}</p>
          </span>
          <Button variant="icon" aria-label={t.qrClose} onClick={onClose}><Icon name="close" size={14} /></Button>
        </div>
        <QrBody
          session={session} phase={phase} account={account} pausedUntil={pausedUntil}
          onClose={onClose} onManual={onManual} retry={retry} onDone={finish}
        />
      </div>
    </Dialog>
  )
}
