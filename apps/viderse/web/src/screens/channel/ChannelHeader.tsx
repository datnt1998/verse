import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Đầu trang Kênh (artboard Channel › .center đầu): avatar 64 (ảnh nhận diện
// hoặc chữ cái), chip ngôn ngữ, tên, dòng phụ (YouTube/ngày tạo/số đối thủ),
// "Mở YouTube" và "Video mới cho kênh" (mở màn Video mới đã chọn sẵn kênh này).
import { Icon } from '@noverse/ui'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import type { Channel } from '../../api/channels'
import { fmtDay } from './fmt'
import { channelTint } from '../../lib/channelTint'
import { readChannelAutonomy } from '../../api/autonomy'
import { autonomyStyles } from '../../components/autonomy/autonomy.stylex'

export function ChannelHeader({ channel }: { channel: Channel }) {
  const tt = t.channels
  const narrow = useMediaQuery('(max-width: 767px)')
  const name = channel.name || tt.page.noName
  const lang = tt.page.language[channel.language]
  const ytFull = channel.youtubeUrl?.replace(/^https?:\/\/(www\.)?/, '')
  // Artboard: dòng phụ ghi tay kênh (@midnighttales.stories), tooltip ghi cả youtube.com/@…
  const autonomy = readChannelAutonomy(channel)
  const createdToday = channel.createdAt
    && new Date(channel.createdAt).toDateString() === new Date().toDateString()
  const parts = [
    ytFull ? ytFull.replace(/^youtube\.com\//, '') : tt.page.noYouTube,
    createdToday ? tt.page.createdToday : tt.page.created(fmtDay(channel.createdAt)),
    narrow ? tt.page.linkedRivalsShort(channel.sources.length) : tt.page.linkedRivals(channel.sources.length),
  ]
  return (
    <div className={`vch-head ${stylex.props(channelStyles.head).className}`}>
      <ChannelAvatar
        name={name}
        src={channel.avatarUrl ?? undefined}
        color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
        size={narrow ? 60 : 64}
        style={{ fontSize: 20, fontWeight: 700 }}
      />
      <div className={`vch-head__text ${stylex.props(channelStyles.headText).className}`}>
        <div className={`vch-head__chips ${stylex.props(channelStyles.headChips).className}`}>
          {autonomy && (
            <span className={`vch-chip vch-chip--auto ${stylex.props(channelStyles.chip, autonomyStyles.headChip, autonomyStyles.headChipOk).className}`}>
              <Icon name="refresh" size={13} />
              <span>{t.autonomy.headChip}</span>
              <span>{autonomy.autonomy.preset === 'custom' ? t.autonomy.custom : t.autonomy.presets[autonomy.autonomy.preset]}</span>
            </span>
          )}
          <span className={`vch-chip ${stylex.props(channelStyles.chip, autonomyStyles.headChip).className}`}>{lang}</span>
          {channel.identity.state === 'generating' && (
            <span className={`vch-chip vch-chip--warn ${stylex.props(channelStyles.chip, channelStyles.chipWarn).className}`}>{tt.identity.generatingSub}</span>
          )}
        </div>
        <h1 className={`vch-title ${stylex.props(channelStyles.title).className}`}>{name}</h1>
        <span className={`vch-sub ${stylex.props(channelStyles.sub).className}`} title={[ytFull ?? tt.page.noYouTube, ...parts.slice(1)].join(' · ')}>{parts.join(' · ')}</span>
      </div>
      <div className={`vch-head__actions ${stylex.props(channelStyles.headActions).className}`}>
        {!narrow && (
          <ActionButton
            className={stylex.props(channelStyles.touchTarget).className}
            size="compact"
            xstyle={[channelStyles.headBtn]}
            onClick={() => channel.youtubeUrl ? window.open(channel.youtubeUrl, '_blank', 'noreferrer') : navigate(paths.channel(channel.id, 'settings'))}
            icon={<Icon name="link" size={15} />}
          >
            {channel.youtubeUrl ? tt.page.openYouTube : tt.page.linkYouTube}
          </ActionButton>
        )}
        {narrow && channel.youtubeUrl && (
          <ActionButton
            className={stylex.props(channelStyles.headBtnLinkMobile).className}
            size="compact"
            xstyle={[channelStyles.headBtnLinkMobile]}
            aria-label={tt.page.openYouTube}
            onClick={() => { if (channel.youtubeUrl) window.open(channel.youtubeUrl, '_blank', 'noreferrer') }}
            icon={<Icon name="link" size={18} />}
          />
        )}
        {narrow && !channel.youtubeUrl && (
          <ActionButton
            className={stylex.props(channelStyles.headBtnLinkMobile).className}
            size="compact"
            xstyle={[channelStyles.headBtnLinkMobile]}
            aria-label={tt.page.linkYouTube}
            onClick={() => navigate(paths.channel(channel.id, 'settings'))}
            icon={<Icon name="link" size={18} />}
          />
        )}
        <ActionButton className={stylex.props(channelStyles.touchTarget).className}
          variant="primary"
          size={narrow ? 'md' : 'compact'}
          xstyle={[channelStyles.headBtn, channelStyles.headBtnPrimary, narrow && channelStyles.headBtnPrimaryMobile]}
          onClick={() => navigate(paths.new(channel.id))}
          icon={<Icon name="video-add" size={15} />}
        >
          {tt.page.newVideo}
        </ActionButton>
      </div>
    </div>
  )
}
