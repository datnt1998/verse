import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Khối nhận diện kênh (artboard Channel › tab=settings, identity=ready|generating|failed):
// board vẽ HAI card 403 px — card ảnh bìa (ảnh 16:9, nhãn 2560×1440, công tắc
// vùng an toàn, hàng nút vẽ lại/tải) và card thông tin (avatar, câu trạng thái,
// mô tả kênh, phong cách app tự rút, ô góp ý cho lần tạo lại). Không tự bấm vẽ:
// mỗi lần vẽ là một ảnh trả phí trong hạn mức ChatGPT.
import { useState } from 'react'
import { Icon, ProgressBar, Switch, TextArea } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/channels'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getActivity } from '../../api/activity'
import { brandingHref, generateBranding, type Channel, type IdentityPart } from '../../api/channels'
import { fmtTime } from './fmt'
import { channelTint } from '../../lib/channelTint'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { mch } from './mobile/mchannel.stylex'

type BriefField = {
  draft: string
  state: 'idle' | 'saving' | 'saved' | 'error'
  error: string | null
  dirty: boolean
  setDraft: (draft: string) => void
  commit: () => Promise<void>
  isSaved: () => boolean
}

export function IdentityBlock({ channel, brief }: { channel: Channel; brief: BriefField }) {
  const tt = t.channels.identity
  const phone = useMediaQuery('(max-width: 767px)')
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal), {
    enabled: channel.identity.state === 'generating',
  })
  // Artboard MChannel: khung an toàn đang bật sẵn để thấy vùng an toàn của ảnh bìa.
  const [safeArea, setSafeArea] = useState(true)
  const [direction, setDirection] = useState('')
  const [pending, setPending] = useState<IdentityPart | null>(null)
  const [error, setError] = useState<string | null>(null)

  const generating = channel.identity.state === 'generating'
  const failed = channel.identity.state === 'failed'
  // Lỗi bước chữ (Đạo diễn đặt tên/mô tả) khác lỗi bước ảnh; thông điệp phải theo `error.step`.
  const failedTextStep = failed && channel.identity.error?.step === 'text'
  const task = activity.data?.running.find((item) => item.kind === 'identity' && item.objectRef.id === channel.id)
  const percent = task?.progress ? Math.round((task.progress.done / Math.max(1, task.progress.total)) * 100) : 0
  const reference = channel.sources.find((source) => source.reference) ?? channel.sources[0]
  const briefLock = brief.state === 'saving'
    ? 'Đang lưu mô tả kênh. Chờ lưu xong trước khi tạo lại.'
    : brief.dirty
      ? 'Lưu mô tả kênh trước khi tạo lại nhận diện. Rời ô để lưu; nếu có lỗi, sửa rồi thử lại.'
      : undefined
  const regenerationLock = generating ? tt.generatingLock : briefLock
  const avatarPending = pending === 'avatar' || pending === 'all'
    || (generating && (!channel.identity.part || channel.identity.part === 'avatar' || channel.identity.part === 'all'))
  const avatar = (
    <span {...stylex.props(channelStyles.idAvatar)} aria-busy={avatarPending || undefined}>
      <ChannelAvatar
        name={channel.name || t.channels.page.noName}
        src={channel.avatarUrl ?? undefined}
        color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
        size={phone ? 48 : 56}
      />
      {avatarPending && (
        <span {...stylex.props(channelStyles.idAvatarPending)} role="status" aria-label="Đang tạo avatar mới; ảnh đang dùng được giữ tới khi xong." />
      )}
    </span>
  )

  const regen = async (part: IdentityPart) => {
    if (generating || pending) return
    // Never start a paid run with an unsaved/failed draft, including blur → click races.
    if (!brief.isSaved()) return
    setPending(part)
    setError(null)
    try {
      const result = await generateBranding(channel.id, {
        part,
        // Omit brief: the backend snapshots the persisted branding.brief, not this editor's draft.
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

  const brandFields = (
    <>
      <div {...stylex.props(channelStyles.idField)}>
        <span {...stylex.props(channelStyles.fieldRow)}>
          <label {...stylex.props(channelStyles.idHead)} htmlFor="vch-brief">{tt.briefTitle}</label>
          <span {...stylex.props(channelStyles.saveline)} aria-live="polite">
            {brief.state === 'saving' ? t.channels.settings.saving : brief.state === 'saved' ? t.channels.settings.saved : brief.state === 'error' ? 'Chưa lưu' : t.channels.settings.autosave}
          </span>
        </span>
        <TextArea
          id="vch-brief"
          xstyle={channelStyles.idBrief}
          value={brief.draft}
          placeholder={tt.briefEmpty}
          maxLength={2000}
          showCount={false}
          rows={4}
          error={brief.error ?? undefined}
          aria-describedby="vch-brief-hint vch-brief-count"
          onChange={(event) => brief.setDraft(event.target.value)}
          onBlur={() => void brief.commit()}
        />
        <span {...stylex.props(channelStyles.idBriefFoot)}>
          <span id="vch-brief-hint">{tt.briefHint}</span>
          <span id="vch-brief-count" {...stylex.props(channelStyles.idBriefCount)} className="num">{brief.draft.length} / 2.000</span>
        </span>
        {briefLock && (
          <span {...stylex.props(channelStyles.idStyleNote)} aria-live="polite">{briefLock}</span>
        )}
        {(brief.dirty || brief.error) && brief.state !== 'saving' && (
          <ActionButton variant="outline" size={phone ? 'lg' : 'sm'} xstyle={channelStyles.idBriefSave}
            onClick={() => void brief.commit()}>
            {brief.error ? 'Thử lưu lại mô tả' : 'Lưu mô tả'}
          </ActionButton>
        )}
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
        <div {...stylex.props(channelStyles.idDirRow, phone && mch.dirCol)}>
          <input
            id="vch-direction"
            {...stylex.props(channelStyles.idDirInput, phone && mch.idDirInput)}
            value={direction}
            placeholder={tt.directionPlaceholder}
            onChange={(event) => setDirection(event.target.value)}
            maxLength={1000}
          />
          <ActionButton variant="outline" size={phone ? 'lg' : 'sm'} block={phone}
            lockedReason={regenerationLock}
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
    </>
  )

  // Artboard MChannel › identity=failed: hộp cảnh báo nằm ngay dưới tab, TRƯỚC thẻ ảnh bìa.
  // Lỗi bước `text` dùng câu chữ của Đạo diễn, không mượn câu lỗi vẽ ảnh.
  const failedNotice = (mobile: boolean) => (
    <div {...stylex.props(mobile ? [mch.idFailed, mch.idFailedTop] : channelStyles.idFailed)} role="alert">
      <span {...stylex.props(mobile ? mch.idFailedText : channelStyles.idFailedText)}>
        <b>{failedTextStep ? tt.failedTitleText : tt.failedTitle}</b>{' '}
        {failedTextStep ? tt.failedBodyText : (channel.identity.error?.message ?? tt.failedBody)}
      </span>
      <span {...stylex.props(mobile ? mch.idFailedRow : channelStyles.idFailedRow)}>
        <ActionButton variant="outline" size={mobile ? 'lg' : 'sm'}
          onClick={() => void regen(failedTextStep ? 'name' : channel.identity.part ?? 'all')}
          pending={pending !== null}
          lockedReason={regenerationLock}
          icon={<Icon name="refresh" size={14} />}
        >
          {tt.retry}
        </ActionButton>
        {channel.identity.retryAt && <span {...stylex.props(mobile ? mch.idFailedWhen : channelStyles.idFailedWhen)}>{tt.retryAt(fmtTime(channel.identity.retryAt))}</span>}
      </span>
    </div>
  )

  return (
    <section {...stylex.props(channelStyles.idSection)} aria-label={t.channels.settings.identityLabel}>
      {phone && failed && failedNotice(true)}
      <div {...stylex.props(channelStyles.idBannerCard)}>
        <div {...stylex.props(channelStyles.banner)}>
          {channel.bannerUrl ? (
            <img {...stylex.props(channelStyles.bannerImage)} src={channel.bannerUrl} alt={t.channels.page.bannerAlt(channel.name || t.channels.page.noName)} />
          ) : (
            <div {...stylex.props(channelStyles.bannerEmpty)}>{tt.bannerCaption}</div>
          )}
          {safeArea && <span {...stylex.props(channelStyles.bannerSafe)} aria-hidden="true" />}
          {generating && (
            <div {...stylex.props(channelStyles.skelOverlay)} role="status">
              <Orb of="artist" size={26} live />
              <span {...stylex.props(channelStyles.skelText)}>{tt.generating}</span>
              <span {...stylex.props(channelStyles.skelSub)}>{tt.generatingSub}</span>
              {(task?.phase || percent > 0) && (
                <span {...stylex.props(channelStyles.progress)}>
                  <ProgressBar value={percent} tone="warn" height={6} />
                  <span {...stylex.props(channelStyles.skelSub)}>
                    {task?.phase ? `${task.phase} ${tt.progress(percent)}` : tt.progress(percent)}
                  </span>
                </span>
              )}
            </div>
          )}
          <span {...stylex.props(channelStyles.bannerChip)}>{tt.bannerCaption}</span>
          {!phone && (
            <div {...stylex.props(channelStyles.bannerSafeToggle)}>
              <Switch checked={safeArea} onChange={setSafeArea} label={tt.safeArea} size="sm" />
              {tt.safeArea}
            </div>
          )}
        </div>

        {phone && (
          <>
            <div {...stylex.props(mch.safeRow)}>
              <span {...stylex.props(mch.safeRowText)}>{tt.safeArea}</span>
              <Switch checked={safeArea} onChange={setSafeArea} label={tt.safeArea} />
            </div>
            <div {...stylex.props(mch.idAvatarRow)}>
              {avatar}
              <div {...stylex.props(mch.idAvatarText)}>
                <span {...stylex.props(mch.idName)}>{channel.name || t.channels.page.noName}</span>
                <span {...stylex.props(mch.idSub)}>{generating ? tt.generatingLock : tt.readySub}</span>
              </div>
            </div>
            <div {...stylex.props(mch.idGrid)}>
              <ActionButton variant="outline" size="lg" block xstyle={[mch.idAction, mch.idWide]}
                lockedReason={regenerationLock}
                onClick={() => void regen('banner')}
                pending={pending === 'banner'}
                icon={<Icon name="refresh" size={14} />}
              >
                {tt.regenBanner}
              </ActionButton>
              <ActionButton variant="outline" size="lg" block xstyle={mch.idAction}
                lockedReason={regenerationLock}
                onClick={() => void regen('avatar')}
                pending={pending === 'avatar'}
                icon={<Icon name="refresh" size={14} />}
              >
                {tt.regenAvatar}
              </ActionButton>
              <ActionButton variant="outline" size="lg" block xstyle={mch.idAction} tooltip={tt.regenNameTitle}
                lockedReason={regenerationLock}
                onClick={() => void regen('name')}
                pending={pending === 'name'}
                icon={<Icon name="edit" size={14} />}
              >
                {tt.regenName}
              </ActionButton>
              {(avatarDownload || bannnerDownload) && (
                <ActionButton variant="outline" size="lg" block xstyle={[mch.idAction, mch.idWide]} title={tt.downloadTitle} onClick={downloadAll} icon={<Icon name="download" size={14} />}>
                  {tt.download}
                </ActionButton>
              )}
            </div>
          </>
        )}

        {!phone && (
          <div {...stylex.props(channelStyles.bannerActions)}>
            <ActionButton variant="outline" size="sm"
              lockedReason={regenerationLock}
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
        )}
      </div>

      <div {...stylex.props(channelStyles.idInfoCard)}>
        {failed && !phone && failedNotice(false)}

        {!phone && (
          <div {...stylex.props(channelStyles.idrow)}>
            {avatar}
            <div {...stylex.props(channelStyles.idrowText)}>
              <span {...stylex.props(channelStyles.idname)}>{channel.name || t.channels.page.noName}</span>
              <span {...stylex.props(channelStyles.idsub, generating && channelStyles.idsubGen)}>
                {generating ? tt.generatingLock : tt.readySub}
              </span>
            </div>
            <span {...stylex.props(channelStyles.idActions)}>
              <ActionButton variant="outline" size="sm"
                lockedReason={regenerationLock}
                onClick={() => void regen('avatar')}
                pending={pending === 'avatar'}
                icon={<Icon name="refresh" size={14} />}
              >
                {tt.regenAvatar}
              </ActionButton>
              <ActionButton variant="outline" size="sm" tooltip={tt.regenNameTitle}
                lockedReason={regenerationLock}
                onClick={() => void regen('name')}
                pending={pending === 'name'}
                icon={<Icon name="edit" size={14} />}
              >
                {tt.regenName}
              </ActionButton>
            </span>
          </div>
        )}

        {brandFields}
      </div>
    </section>
  )
}
