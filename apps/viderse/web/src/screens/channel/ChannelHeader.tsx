// Đầu trang Kênh (artboard Channel › .center đầu): avatar 64 (ảnh nhận diện
// hoặc chữ cái), chip ngôn ngữ, tên, dòng phụ (YouTube/ngày tạo/số đối thủ),
// "Mở YouTube" và "Video mới cho kênh". Nút video mới là việc của hàng chờ —
// khoá kèm lý do hàng chờ, không bịa trạng thái chạy.
import { Icon } from '@noverse/ui'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/vi'
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
    <div className="vch-head">
      <ChannelAvatar
        name={name}
        src={channel.avatarUrl ?? undefined}
        color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
        size={64}
        style={{ fontSize: 20, fontWeight: 700 }}
      />
      <div className="vch-head__text">
        <div className="vch-head__chips">
          <span className="vch-chip">{lang}</span>
          {channel.identity.state === 'generating' && (
            <span className="vch-chip vch-chip--warn">{tt.identity.generatingSub}</span>
          )}
        </div>
        <h1 className="vch-title">{name}</h1>
        <span className="vch-sub" title={parts.join(' · ')}>{parts.join(' · ')}</span>
      </div>
      <div className="vch-head__actions">
        {channel.youtubeUrl ? (
          <a
            className="vch-chip"
            href={channel.youtubeUrl}
            target="_blank"
            rel="noreferrer"
            style={{ height: 'var(--vv-s-34)', padding: '0 var(--vv-s-12)' }}
          >
            <Icon name="link" size={15} />
            {tt.page.openYouTube}
          </a>
        ) : null}
        <ActionButton
          variant="primary"
          size="md"
          lockedReason={tt.queueLock}
          icon={<Icon name="video-add" size={15} />}
        >
          {tt.page.newVideo}
        </ActionButton>
      </div>
    </div>
  )
}
