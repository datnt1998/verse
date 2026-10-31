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
    <section className="vch-card" aria-label={t.channels.page.tabs.settings}>
      <div className="vch-identity">
        <div className="vch-banner">
          {channel.bannerUrl ? (
            <img src={channel.bannerUrl} alt={t.channels.page.bannerAlt(channel.name || t.channels.page.noName)} />
          ) : (
            <div className="vch-banner__empty">{tt.bannerCaption}</div>
          )}
          {safeArea && <span className="vch-banner__safe" aria-hidden="true" />}
        </div>
        <div className="vch-bannercap">
          <span>{tt.bannerCaption}</span>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-8)' }}>
            {tt.safeArea}
            <Switch checked={safeArea} onChange={setSafeArea} label={tt.safeArea} size="sm" tone="mono" />
          </span>
        </div>

        {generating && (
          <div className="vch-skel" role="status">
            <span style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-6)', alignItems: 'center' }}>
              <span>{tt.generating}</span>
              <span style={{ fontSize: 11.5 }}>{tt.generatingSub}</span>
              <span className="vch-progress" style={{ width: 220 }}>
                <ProgressBar value={percent} tone="warn" height={6} />
                <span style={{ fontSize: 11.5 }}>{task?.phase ?? tt.generatingSub} {tt.progress(percent)}</span>
              </span>
            </span>
          </div>
        )}

        <div className="vch-idrow">
          <ChannelAvatar
            name={channel.name || t.channels.page.noName}
            src={channel.avatarUrl ?? undefined}
            color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
            size={56}
            style={{ fontSize: 17, fontWeight: 700 }}
          />
          <div className="vch-idrow__text">
            <span className="vch-idname">{channel.name || t.channels.page.noName}</span>
            <span className={generating ? 'vch-idsub vch-idsub--gen' : 'vch-idsub'}>
              {generating ? tt.generatingLock : tt.readySub}
            </span>
          </div>
          <div className="vch-actions">
            {avatarDownload && (
              <a className="vch-chip" href={avatarDownload} download>
                <Icon name="download" size={14} />
                {tt.downloadAvatar}
              </a>
            )}
            {bannnerDownload && (
              <a className="vch-chip" href={bannnerDownload} download>
                <Icon name="download" size={14} />
                {tt.downloadBanner}
              </a>
            )}
          </div>
        </div>

        {failed && (
          <div className="vch-alert" role="alert">
            <span><b>{tt.failedTitle}</b> {channel.identity.error?.message ?? tt.failedBody}</span>
            <span style={{ fontSize: 12.5 }}>{tt.failedBodyName}</span>
            <span className="vch-alert__actions">
              <ActionButton
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

        <div className="vch-actions">
          <ActionButton
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('avatar')}
            pending={pending === 'avatar'}
            icon={<Icon name="refresh" size={14} />}
          >
            {tt.regenAvatar}
          </ActionButton>
          <ActionButton
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('name')}
            pending={pending === 'name'}
            icon={<Icon name="edit" size={14} />}
          >
            {tt.regenName}
          </ActionButton>
          <ActionButton
            size="sm"
            lockedReason={generating ? tt.generatingLock : undefined}
            onClick={() => void regen('banner')}
            pending={pending === 'banner'}
            icon={<Icon name="refresh" size={14} />}
          >
            {tt.regenBanner}
          </ActionButton>
          <ActionButton
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
        <p className="vch-field__hint">{tt.costHint}</p>
        <p className="vch-field__hint">{tt.regenAllHint}</p>
        {error && <p className="vch-field__error" role="alert">{error}</p>}

        <div className="vch-field">
          <span className="vch-field__row">
            <span className="vch-field__label">{tt.briefTitle}</span>
            <span className="vch-counter num">{channel.branding.brief.length} / 2.000</span>
          </span>
          <div className="vch-readonly">{channel.branding.brief || tt.briefEmpty}</div>
          <p className="vch-field__hint">{tt.briefHint}</p>
        </div>

        <div className="vch-field">
          <span className="vch-field__label">{tt.styleTitle} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.styleAuto}</span></span>
          <div className="vch-readonly">
            {channel.branding.look
              ? `${channel.branding.look} ${reference ? tt.styleFrom(reference.title) : ''}`
              : tt.styleEmpty}
          </div>
        </div>

        <div className="vch-field">
          <label className="vch-field__label" htmlFor="vch-direction">{tt.direction}</label>
          <input
            id="vch-direction"
            className="vch-taginput"
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
