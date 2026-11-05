import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Đầu trang Kênh (artboard Channel/MChannel › head): avatar nhận diện, chip
// Tự lấy (xanh khi bật, xám khi tắt), chip Tự duyệt ảnh ≥ 85 khi bật, chip
// ngôn ngữ, tên kênh, dòng phụ (YouTube/ngày tạo/số đối thủ), "Mở YouTube"
// và "Video mới cho kênh" (mở màn Video mới đã chọn sẵn kênh này).
import { Button, Icon } from '@noverse/ui'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import type { Channel } from '../../api/channels'
import { fmtDay } from './fmt'
import { channelTint } from '../../lib/channelTint'
import { mch } from './mobile/mchannel.stylex'

export function ChannelHeader({ channel }: { channel: Channel }) {
  const tt = t.channels
  const narrow = useMediaQuery('(max-width: 767px)')
  const name = channel.name || tt.page.noName
  const lang = tt.page.language[channel.language]
  const ytFull = channel.youtubeUrl?.replace(/^https?:\/\/(www\.)?/, '')
  const createdToday = channel.createdAt
    && new Date(channel.createdAt).toDateString() === new Date().toDateString()
  const parts = [
    ytFull ? ytFull.replace(/^youtube\.com\//, '') : tt.page.noYouTube,
    createdToday ? tt.page.createdToday : tt.page.created(fmtDay(channel.createdAt)),
    narrow ? tt.page.linkedRivalsShort(channel.sources.length) : tt.page.linkedRivals(channel.sources.length),
  ]
  const fetchChip = narrow
    ? (channel.autoFetch ? mch.headChip : mch.headChipPlain)
    : [channelStyles.chip, channel.autoFetch && channelStyles.chipAuto]
  const approveChip = narrow ? mch.headChip : [channelStyles.chip, channelStyles.chipAuto]
  const langChip = narrow ? mch.headChipPlain : channelStyles.chip
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
          <span className={`vch-chip ${stylex.props(fetchChip).className}`}>
            {channel.autoFetch && <Icon name="refresh" size={13} />}
            {channel.autoFetch ? tt.page.fetchChip(channel.perDay) : tt.page.fetchChipOff}
          </span>
          {channel.autoApprove && (
            <span className={`vch-chip ${stylex.props(approveChip).className}`}>
              <Icon name="tick" size={13} />
              {tt.page.approveChip}
            </span>
          )}
          <span className={`vch-chip ${stylex.props(langChip).className}`}>{lang}</span>
        </div>
        <h1 className={`vch-title ${stylex.props(channelStyles.title).className}`}>{name}</h1>
        <span className={`vch-sub ${stylex.props(channelStyles.sub).className}`} title={[ytFull ?? tt.page.noYouTube, ...parts.slice(1)].join(' · ')}>{parts.join(' · ')}</span>
      </div>
      <div className={`vch-head__actions ${stylex.props(channelStyles.headActions).className}`}>
        {!narrow && (
          <ActionButton
            size="compact"
            onClick={() => channel.youtubeUrl ? window.open(channel.youtubeUrl, '_blank', 'noreferrer') : navigate(paths.channel(channel.id, 'settings'))}
            icon={<Icon name="link" size={15} />}
          >
            {channel.youtubeUrl ? tt.page.openYouTube : tt.page.linkYouTube}
          </ActionButton>
        )}
        <ActionButton
          xstyle={narrow ? channelStyles.headPrimaryFlex : undefined}
          variant="primary"
          size={narrow ? 'xl' : 'compact'}
          onClick={() => navigate(paths.new(channel.id))}
          icon={<Icon name="video-add" size={15} />}
        >
          {tt.page.newVideo}
        </ActionButton>
        {/* MChannel: nút icon không viền, đứng sau nút chính (artboard đầu trang). */}
        {narrow && (
          <Button
            variant="icon"
            size="xl"
            aria-label={channel.youtubeUrl ? tt.page.openYouTube : tt.page.linkYouTube}
            onClick={() => channel.youtubeUrl ? window.open(channel.youtubeUrl, '_blank', 'noreferrer') : navigate(paths.channel(channel.id, 'settings'))}
            icon={<Icon name="link" size={18} />}
          />
        )}
      </div>
    </div>
  )
}
