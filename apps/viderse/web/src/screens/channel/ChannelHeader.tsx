import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Đầu trang Kênh (artboard Channel › .center đầu): avatar 64 (ảnh nhận diện
// hoặc chữ cái), chip ngôn ngữ, tên, dòng phụ (YouTube/ngày tạo/số đối thủ),
// "Mở YouTube" và "Video mới cho kênh" (mở màn Video mới đã chọn sẵn kênh này).
import { Icon } from '@noverse/ui'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import type { Channel } from '../../api/channels'
import { fmtDay } from './fmt'
import { channelTint } from '../../lib/channelTint'

export function ChannelHeader({ channel }: { channel: Channel }) {
  const tt = t.channels
  const name = channel.name || tt.page.noName
  const lang = tt.page.language[channel.language]
  const parts = [
    channel.youtubeUrl ? channel.youtubeUrl.replace(/^https?:\/\/(www\.)?/, '') : tt.page.noYouTube,
    tt.page.created(fmtDay(channel.createdAt)),
    tt.page.linkedRivals(channel.sources.length),
  ]
  return (
    <div className={`vch-head ${stylex.props(channelStyles.head).className}`}>
      <ChannelAvatar
        name={name}
        src={channel.avatarUrl ?? undefined}
        color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
        size={64}
        style={{ fontSize: 20, fontWeight: 700 }}
      />
      <div className={`vch-head__text ${stylex.props(channelStyles.headText).className}`}>
        <div className={`vch-head__chips ${stylex.props(channelStyles.headChips).className}`}>
          <span className={`vch-chip ${stylex.props(channelStyles.chip).className}`}>{lang}</span>
          {channel.identity.state === 'generating' && (
            <span className={`vch-chip vch-chip--warn ${stylex.props(channelStyles.chip, channelStyles.chipWarn).className}`}>{tt.identity.generatingSub}</span>
          )}
        </div>
        <h1 className={`vch-title ${stylex.props(channelStyles.title).className}`}>{name}</h1>
        <span className={`vch-sub ${stylex.props(channelStyles.sub).className}`} title={parts.join(' · ')}>{parts.join(' · ')}</span>
      </div>
      <div className={`vch-head__actions ${stylex.props(channelStyles.headActions).className}`}>
        {channel.youtubeUrl ? (
          <a
            className={`vch-chip ${stylex.props(channelStyles.chip).className}`}
            href={channel.youtubeUrl}
            target="_blank"
            rel="noreferrer"
            style={{ height: 'var(--vv-s-34)', padding: '0 var(--vv-s-12)' }}
          >
            <Icon name="link" size={15} />
            {tt.page.openYouTube}
          </a>
        ) : null}
        <ActionButton className={stylex.props(channelStyles.touchTarget).className}
          variant="primary"
          size="md"
          onClick={() => navigate(paths.new(channel.id))}
          icon={<Icon name="video-add" size={15} />}
        >
          {tt.page.newVideo}
        </ActionButton>
      </div>
    </div>
  )
}
