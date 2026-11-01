import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Khối nhận diện kênh (artboard Channel › tab=settings, identity=ready|generating|failed):
// board vẽ HAI card 403 px — card ảnh bìa (ảnh 16:9, nhãn 2560×1440, công tắc
// vùng an toàn, hàng nút vẽ lại/tải) và card thông tin (avatar, câu trạng thái,
// mô tả kênh, phong cách app tự rút, ô góp ý cho lần tạo lại). Không tự bấm vẽ:
// mỗi lần vẽ là một ảnh trả phí trong hạn mức ChatGPT.
import { useState } from 'react'
import { Icon, ProgressBar } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getActivity } from '../../api/activity'
import { brandingHref, generateBranding, type Channel, type IdentityPart } from '../../api/channels'
import { fmtTime } from './fmt'
import { channelTint } from '../../lib/channelTint'

export function IdentityBlock({ channel }: { channel: Channel }) {
  const tt = t.channels.identity
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal), {
    enabled: channel.identity.state === 'generating',
  })
  const [safeArea, setSafeArea] = useState(false)
  const [direction, setDirection] = useState('')
  const [pending, setPending] = useState<IdentityPart | null>(null)
  const [error, setError] = useState<string | null>(null)

  const generating = channel.identity.state === 'generating'
  const failed = channel.identity.state === 'failed'
  const task = activity.data?.running.find((item) => item.kind === 'identity' && item.objectRef.id === channel.id)
  const percent = task?.progress ? Math.round((task.progress.done / Math.max(1, task.progress.total)) * 100) : 0
  const reference = channel.sources.find((source) => source.reference) ?? channel.sources[0]

  const regen = async (part: IdentityPart) => {
    if (generating || pending) return
    setPending(part)
    setError(null)
    try {
      const result = await generateBranding(channel.id, {
        part,
        direction: direction.trim() ? direction.trim() : undefined,
      })
      setData(keys.channel(channel.id), result.channel)
      void invalidate(keys.channels())
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.regenerateFail)
    } finally {
      setPending(null)
    }
  }

  const bannnerDownload = brandingHref(channel.id, channel.bannerUrl, true)
  const avatarDownload = brandingHref(channel.id, channel.avatarUrl, true)
  // Board gộp hai ảnh vào một nút "Tải avatar + ảnh bìa kênh": tải lần lượt
  // ảnh đang có (không có ảnh nào thì không vẽ nút).
  const downloadAll = () => {
    for (const href of [avatarDownload, bannnerDownload]) {
      if (!href) continue
      const link = document.createElement('a')
      link.href = href
      link.download = ''
      link.click()
    }
  }

  return (
    <section {...stylex.props(channelStyles.idSection)} aria-label={tt.identityLabel}>
      <div {...stylex.props(channelStyles.idBannerCard)}>
        <div {...stylex.props(channelStyles.banner)}>
          {channel.bannerUrl ? (
            <img {...stylex.props(channelStyles.bannerImage)} src={channel.bannerUrl} alt={t.channels.page.bannerAlt(channel.name || t.channels.page.noName)} />
          ) : (
            <div {...stylex.props(channelStyles.bannerEmpty)}>{tt.bannerCaption}</div>
          )}
          {safeArea && <span {...stylex.props(channelStyles.bannerSafeDim)} aria-hidden="true" />}
          {safeArea && <span {...stylex.props(channelStyles.bannerSafe)} aria-hidden="true" />}
          <span {...stylex.props(channelStyles.bannerChip)}>{tt.bannerCaption}</span>
          <button {...stylex.props(channelStyles.bannerSafeToggle)}
            type="button"
            role="switch"
            aria-checked={safeArea}
            aria-label={tt.safeArea}
            onClick={() => setSafeArea(!safeArea)}
          >
            <span {...stylex.props(channelStyles.safeTrack, safeArea && channelStyles.safeTrackOn)} aria-hidden="true">
              <span {...stylex.props(channelStyles.safeKnob)} />
            </span>
            {tt.safeArea}
          </button>
          {generating && (
            <div {...stylex.props(channelStyles.skelOverlay)} role="status">
              <Orb of="artist" size={26} live />
              <span {...stylex.props(channelStyles.skelText)}>{tt.generating}</span>
              <span {...stylex.props(channelStyles.skelSub)}>{tt.generatingSub}</span>
              <span {...stylex.props(channelStyles.progress)}>
                <ProgressBar value={percent} tone="warn" height={6} />
                <span {...stylex.props(channelStyles.skelSub)}>{task?.phase ?? tt.generatingSub} {tt.progress(percent)}</span>
              </span>
            </div>
          )}
        </div>
        <div {...stylex.props(channelStyles.bannerActions)}>
          <ActionButton variant="outline" size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('banner')}
            pending={pending === 'banner'}
            icon={<Icon name="refresh" size={14} />}
          >
            {tt.regenBanner}
          </ActionButton>
          {(avatarDownload || bannnerDownload) && (
            <ActionButton variant="outline" size="sm" title={tt.downloadTitle} onClick={downloadAll} icon={<Icon name="download" size={14} />}>
              {tt.download}
            </ActionButton>
          )}
        </div>
      </div>

      <div {...stylex.props(channelStyles.idInfoCard)}>
        {failed && (
          <div {...stylex.props(channelStyles.idFailed)} role="alert">
            <span {...stylex.props(channelStyles.idFailedText)}>
              <b>{tt.failedTitle}</b> {channel.identity.error?.message ?? tt.failedBody}
            </span>
            <span {...stylex.props(channelStyles.idFailedRow)}>
              <ActionButton variant="outline" size="sm" xstyle={channelStyles.idFailedBtn}
                onClick={() => void regen(channel.identity.part ?? 'all')}
                pending={pending !== null}
                icon={<Icon name="refresh" size={14} />}
              >
                {tt.retry}
              </ActionButton>
              {channel.identity.retryAt && <span {...stylex.props(channelStyles.idFailedWhen)}>{tt.retryAt(fmtTime(channel.identity.retryAt))}</span>}
            </span>
          </div>
        )}

        <div {...stylex.props(channelStyles.idrow)}>
          <ChannelAvatar
            name={channel.name || t.channels.page.noName}
            src={channel.avatarUrl ?? undefined}
            color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
            size={56}
          />
          <div {...stylex.props(channelStyles.idrowText)}>
            <span {...stylex.props(channelStyles.idname)}>{channel.name || t.channels.page.noName}</span>
            <span {...stylex.props(channelStyles.idsub, generating && channelStyles.idsubGen)}>
              {generating ? tt.generatingLock : tt.readySub}
            </span>
          </div>
          <span {...stylex.props(channelStyles.idActions)}>
            <ActionButton variant="outline" size="sm"
              lockedReason={generating ? tt.generatingLock : undefined}
              onClick={() => void regen('avatar')}
              pending={pending === 'avatar'}
              icon={<Icon name="refresh" size={14} />}
            >
              {tt.regenAvatar}
            </ActionButton>
            <ActionButton variant="outline" size="sm" tooltip={tt.regenNameTitle}
              lockedReason={generating ? tt.generatingLock : undefined}
              onClick={() => void regen('name')}
              pending={pending === 'name'}
              icon={<Icon name="edit" size={14} />}
            >
              {tt.regenName}
            </ActionButton>
          </span>
        </div>

        <div {...stylex.props(channelStyles.idField)}>
          <span {...stylex.props(channelStyles.idHead)}>{tt.briefTitle}</span>
          <div {...stylex.props(channelStyles.idBrief)}>{channel.branding.brief || tt.briefEmpty}</div>
          <span {...stylex.props(channelStyles.idBriefFoot)}>
            <span>{tt.briefHint}</span>
            <span className="num">{channel.branding.brief.length} / 2.000</span>
          </span>
        </div>

        <div {...stylex.props(channelStyles.idStyleBox)}>
          <span {...stylex.props(channelStyles.idHead)}>
            <Icon name="magic" size={15} xstyle={channelStyles.idStyleIcon} />
            {tt.styleTitle} <span {...stylex.props(channelStyles.idStyleAuto)}>{tt.styleAuto}</span>
          </span>
          <p {...stylex.props(channelStyles.idStyleText)}>{channel.branding.look || tt.styleEmpty}</p>
          {reference && channel.branding.look && (
            <span {...stylex.props(channelStyles.idStyleNote)}>{tt.styleFrom(reference.title)}</span>
          )}
        </div>

        <div {...stylex.props(channelStyles.idField)}>
          <label {...stylex.props(channelStyles.fieldLabel)} htmlFor="vch-direction">{tt.direction}</label>
          <div {...stylex.props(channelStyles.idDirRow)}>
            <input
              id="vch-direction"
              {...stylex.props(channelStyles.idDirInput)}
              value={direction}
              placeholder={tt.directionPlaceholder}
              onChange={(event) => setDirection(event.target.value)}
              maxLength={1000}
            />
            <ActionButton variant="outline" size="sm" xstyle={channelStyles.idBtnTall}
              lockedReason={generating ? tt.generatingLock : undefined}
              onClick={() => void regen('all')}
              pending={pending === 'all'}
              icon={<Icon name="magic" size={14} />}
            >
              {tt.regenAll}
            </ActionButton>
          </div>
          <span {...stylex.props(channelStyles.idStyleNote)}>{tt.regenAllHint}</span>
        </div>

        {error && <p {...stylex.props(channelStyles.fieldError)} role="alert">{error}</p>}
      </div>
    </section>
  )
}
