import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Khối nhận diện kênh (artboard Channel › tab=settings, identity=ready|generating|failed):
// ảnh bìa 16:9 kèm vùng an toàn, avatar, câu trạng thái, nút vẽ lại từng phần
// (khoá khi đang vẽ, lý do trong tooltip/aria-describedby), tải ảnh, mô tả kênh
// và phong cách app tự rút, ô góp ý cho lần tạo lại. Không tự bấm vẽ: mỗi lần
// vẽ là một ảnh trả phí trong hạn mức ChatGPT.
import { useState } from 'react'
import { Icon, ProgressBar, Switch } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
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

  return (
    <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={t.channels.page.tabs.settings}>
      <div className={`vch-identity ${stylex.props(channelStyles.identity).className}`}>
        <div className={`vch-banner ${stylex.props(channelStyles.banner).className}`}>
          {channel.bannerUrl ? (
            <img {...stylex.props(channelStyles.bannerImage)} src={channel.bannerUrl} alt={t.channels.page.bannerAlt(channel.name || t.channels.page.noName)} />
          ) : (
            <div className={`vch-banner__empty ${stylex.props(channelStyles.bannerEmpty).className}`}>{tt.bannerCaption}</div>
          )}
          {safeArea && <span className={`vch-banner__safe ${stylex.props(channelStyles.bannerSafe).className}`} aria-hidden="true" />}
        </div>
        <div className={`vch-bannercap ${stylex.props(channelStyles.bannercap).className}`}>
          <span>{tt.bannerCaption}</span>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-8)' }}>
            {tt.safeArea}
            <Switch className={stylex.props(channelStyles.touchTarget).className} checked={safeArea} onChange={setSafeArea} label={tt.safeArea} size="sm" tone="mono" />
          </span>
        </div>

        {generating && (
          <div className={`vch-skel ${stylex.props(channelStyles.skel).className}`} role="status">
            <span style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)', alignItems: 'center' }}>
              <span>{tt.generating}</span>
              <span style={{ fontSize: 11.5 }}>{tt.generatingSub}</span>
              <span className={`vch-progress ${stylex.props(channelStyles.progress).className}`} style={{ width: 220 }}>
                <ProgressBar value={percent} tone="warn" height={6} />
                <span style={{ fontSize: 11.5 }}>{task?.phase ?? tt.generatingSub} {tt.progress(percent)}</span>
              </span>
            </span>
          </div>
        )}

        <div className={`vch-idrow ${stylex.props(channelStyles.idrow).className}`}>
          <ChannelAvatar
            name={channel.name || t.channels.page.noName}
            src={channel.avatarUrl ?? undefined}
            color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
            size={56}
            style={{ fontSize: 17, fontWeight: 700 }}
          />
          <div className={`vch-idrow__text ${stylex.props(channelStyles.idrowText).className}`}>
            <span className={`vch-idname ${stylex.props(channelStyles.idname).className}`}>{channel.name || t.channels.page.noName}</span>
            <span className={`vch-idsub${generating ? ' vch-idsub--gen' : ''} ${stylex.props(channelStyles.idsub, generating && channelStyles.idsubGen).className}`}>
              {generating ? tt.generatingLock : tt.readySub}
            </span>
          </div>
          <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
            {avatarDownload && (
              <a className={`vch-chip ${stylex.props(channelStyles.chip).className}`} href={avatarDownload} download>
                <Icon name="download" size={14} />
                {tt.downloadAvatar}
              </a>
            )}
            {bannnerDownload && (
              <a className={`vch-chip ${stylex.props(channelStyles.chip).className}`} href={bannnerDownload} download>
                <Icon name="download" size={14} />
                {tt.downloadBanner}
              </a>
            )}
          </div>
        </div>

        {failed && (
          <div className={`vch-alert ${stylex.props(channelStyles.alert).className}`} role="alert">
            <span><b>{tt.failedTitle}</b> {channel.identity.error?.message ?? tt.failedBody}</span>
            <span style={{ fontSize: 12.5 }}>{tt.failedBodyName}</span>
            <span className={`vch-alert__actions ${stylex.props(channelStyles.alertActions).className}`}>
              <ActionButton className={stylex.props(channelStyles.touchTarget).className}
                size="sm"
                onClick={() => void regen(channel.identity.part ?? 'all')}
                pending={pending !== null}
                icon={<Icon name="refresh" size={14} />}
              >
                {tt.retry}
              </ActionButton>
              {channel.identity.retryAt && <span style={{ fontSize: 12.5 }}>{tt.retryAt(fmtTime(channel.identity.retryAt))}</span>}
            </span>
          </div>
        )}

        <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className}
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('avatar')}
            pending={pending === 'avatar'}
            icon={<Icon name="refresh" size={14} />}
          >
            {tt.regenAvatar}
          </ActionButton>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className}
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('name')}
            pending={pending === 'name'}
            icon={<Icon name="edit" size={14} />}
          >
            {tt.regenName}
          </ActionButton>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className}
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('banner')}
            pending={pending === 'banner'}
            icon={<Icon name="refresh" size={14} />}
          >
            {tt.regenBanner}
          </ActionButton>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className}
            variant="primary"
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('all')}
            pending={pending === 'all'}
            icon={<Icon name="magic" size={14} />}
          >
            {tt.regenAll}
          </ActionButton>
        </div>
        <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.costHint}</p>
        <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.regenAllHint}</p>
        {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert">{error}</p>}

        <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
          <span className={`vch-field__row ${stylex.props(channelStyles.fieldRow).className}`}>
            <span className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`}>{tt.briefTitle}</span>
            <span className={`vch-counter num ${stylex.props(channelStyles.counter).className}`}>{channel.branding.brief.length} / 2.000</span>
          </span>
          <div className={`vch-readonly ${stylex.props(channelStyles.readonly).className}`}>{channel.branding.brief || tt.briefEmpty}</div>
          <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.briefHint}</p>
        </div>

        <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
          <span className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`}>{tt.styleTitle} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.styleAuto}</span></span>
          <div className={`vch-readonly ${stylex.props(channelStyles.readonly).className}`}>
            {channel.branding.look
              ? `${channel.branding.look} ${reference ? tt.styleFrom(reference.title) : ''}`
              : tt.styleEmpty}
          </div>
        </div>

        <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
          <label className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`} htmlFor="vch-direction">{tt.direction}</label>
          <input
            id="vch-direction"
            className={`vch-taginput ${stylex.props(channelStyles.taginput).className}`}
            style={{ width: '100%', height: 'var(--vv-s-36)' }}
            value={direction}
            placeholder={tt.directionPlaceholder}
            onChange={(event) => setDirection(event.target.value)}
            maxLength={1000}
          />
        </div>
      </div>
    </section>
  )
}
