import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, saccYtLabelMarker } from '../settings.stylex'
// YouTube trong thẻ Tài khoản nền tảng: chọn hồ sơ trình duyệt để lấy cookie
// (một select duy nhất, lưu ngay khi đổi; “Tắt” = xoá đăng nhập có xác nhận cam)
// và banner bot “xác nhận bạn không phải bot” → POST login/check.
// Máy tính: nhãn + select + dòng trạng thái chấm màu (artboard Settings).
// Điện thoại: chỉ một hàng chữ “Cookie từ trình duyệt / Chrome · Profile 1 ·
// đổi trên máy tính” — không có điều khiển (artboard MSettings).
import { useEffect, useState } from 'react'
import { Banner, KitSelect, confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { checkPlatformLogin, deletePlatformLogin, listBrowserProfiles, putPlatformLogin, type Platform } from '../../../api/platforms'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { accountsCopy as t } from '../../../i18n/accounts'
import { fmtTime } from '../fmt'

/** “Chrome:Kênh chính” → {browser, profile}. */
function splitProfile(value: string | undefined): { browser: string; profile: string } {
  const idx = value?.indexOf(':') ?? -1
  if (!value || idx < 0) return { browser: '', profile: '' }
  return { browser: value.slice(0, idx), profile: value.slice(idx + 1) }
}

const profileLabel = (value: string | undefined): string => {
  const { browser, profile } = splitProfile(value)
  return browser ? `${browser} · ${profile}` : ''
}

/** Banner bot dùng chung: máy tính ngay dưới đầu thẻ, điện thoại cùng vị trí
 *  (artboard MSettings không có tweak bot, nhưng thẻ vẫn nhận banner trong thân). */
function BotBanner({ platform, align }: { platform: Platform; align: 'top' | 'center' }) {
  const [busy, setBusy] = useState(false)

  async function retry() {
    setBusy(true)
    try {
      await checkPlatformLogin(platform.id)
      toast.ok(t.ytChecked)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.ytSaving)
    } finally {
      await invalidate(keys.platforms())
      setBusy(false)
    }
  }

  if (platform.lastError?.code !== 'cookies_failed') return null
  return (
    <Banner
      tone="error"
      icon="danger"
      role="alert"
      align={align}
      title={t.ytBotTitle(fmtTime(platform.lastError.at))}
      action={{ label: busy ? t.ytRetrying : t.ytRetry, onClick: () => void retry() }}
    >
      {t.ytBotBody}
    </Banner>
  )
}

export function YouTubeLogin({ platform }: { platform: Platform }) {
  const profiles = useResource(
    keys.platformProfiles(),
    ({ signal }) => listBrowserProfiles(platform.id, { signal }),
  )
  const savedProfile = platform.login.saved ? profileLabel(platform.login.browserProfile) : ''
  const savedValue = platform.login.saved && platform.login.browserProfile ? platform.login.browserProfile : 'off'
  const [choice, setChoice] = useState(savedValue)
  const [busy, setBusy] = useState(false)

  useEffect(() => setChoice(savedValue), [savedValue])

  const options = [
    { value: 'off', label: t.ytOff },
    ...(profiles.data ?? []).map((p) => ({ value: `${p.browser}:${p.profile}`, label: `${p.browser} · ${p.name || p.profile}` })),
  ]

  async function pick(value: string) {
    if (value === savedValue) return
    if (value === 'off') {
      // Tắt cookie = xoá đăng nhập → xác nhận cam trước DELETE.
      confirm({
        tone: 'warn',
        title: t.ytClearTitle,
        body: t.ytClearBody,
        cancelLabel: t.clearConfirmCancel,
        confirmLabel: t.ytOff,
        onConfirm: async () => {
          await deletePlatformLogin(platform.id)
          await invalidate(keys.platforms())
          toast.ok(t.ytCleared)
        },
      }).catch((error: unknown) => {
        toast.err(error instanceof Error ? error.message : t.ytSaving)
      })
      return
    }
    setChoice(value)
    const next = splitProfile(value)
    setBusy(true)
    try {
      await putPlatformLogin(platform.id, { browser: next.browser, profile: next.profile })
      toast.ok(t.ytSaved)
    } catch (error) {
      setChoice(savedValue)
      toast.err(error instanceof Error ? error.message : t.ytSaving)
    } finally {
      await invalidate(keys.platforms())
      setBusy(false)
    }
  }

  const status = (
    <span className={`sacc-yt__state ${stylex.props(settingsStyles.saccYtState).className}`}>
      <span className={`sacc-dot ${stylex.props(settingsStyles.saccDot, (platform.login.saved) && settingsStyles.saccDotOn, (!(platform.login.saved)) && settingsStyles.saccDotOff).className}`} aria-hidden="true" />
      {platform.login.saved ? t.ytStatusOn : t.ytStatusOff}
    </span>
  )

  // Artboard: banner sự cố nằm NGAY DƯỚI đầu thẻ (ngoài vùng đăng nhập có vạch
  // ngăn), rồi tới hàng chọn hồ sơ, rồi dòng trạng thái chấm màu.
  return (
    <>
      <BotBanner platform={platform} align="center" />
      <div className={`sacc-login ${stylex.props(settingsStyles.saccLogin).className}`}>
        <div className={`sacc-yt ${stylex.props(settingsStyles.saccYt).className}`}>
          <span className={`sacc-yt__label ${stylex.props(saccYtLabelMarker, settingsStyles.saccYtLabel).className}`}>
            <b {...stylex.props(settingsStyles.SaccYtLabelB)}>{t.ytLoginLabel}</b>
            <span {...stylex.props(settingsStyles.SaccYtLabelSpan)}>{t.ytLoginSub}</span>
          </span>
          {profiles.data !== undefined && options.length === 1
            ? <span className={`sacc-yt__state ${stylex.props(settingsStyles.saccYtState).className}`}>{t.ytProfilesEmpty}</span>
            : (
              <KitSelect
                options={options}
                value={choice}
                onChange={(value) => void pick(value)}
                aria-label={t.ytLoginLabel}
                disabled={busy || profiles.data === undefined}
              />
            )}
        </div>
      </div>
      {status}
    </>
  )
}

/** Hàng gọn của điện thoại: chỉ đọc hồ sơ đã lưu, đổi trên máy tính. */
export function YouTubeLoginRow({ platform }: { platform: Platform }) {
  const saved = platform.login.saved ? profileLabel(platform.login.browserProfile) : ''
  return (
    <>
      <BotBanner platform={platform} align="top" />
      <div className={`sacc-login ${stylex.props(settingsStyles.saccLogin).className}`}>
        <div className={`sacc-mrow ${stylex.props(settingsStyles.saccMrow).className}`}>
          <span className={`sacc-mrow__text ${stylex.props(settingsStyles.saccMrowText).className}`}>
            <span className={`sacc-mrow__k ${stylex.props(settingsStyles.saccMrowK).className}`}>{t.mYtLoginK}</span>
            <span className={`sacc-mrow__v ${stylex.props(settingsStyles.saccMrowV).className}`}>{saved ? t.mYtLoginV(saved) : t.mYtLoginVOff}</span>
          </span>
        </div>
      </div>
    </>
  )
}
