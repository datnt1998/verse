import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Cột trái trang Kênh (artboard Channel › aside "Kênh và thư viện"): "Mọi video
// của N kênh", danh sách kênh của tôi (avatar nhận diện hoặc chữ cái), "Tạo
// kênh", nhóm "Thư viện dùng chung" với số thật từ API, chân "Máy sẵn sàng".
// Nhịp "Hôm nay x/y" và chấm "Có video chờ" cần clone job — chưa có nên dòng
// phụ chỉ nói ngày tạo (dữ liệu thật) chứ không bịa số.
import { Icon } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { t } from '../../i18n/channels'
import { paths, type ChannelTab } from '../../router'
import type { Channel } from '../../api/channels'
import { fmtDay } from './fmt'
import { channelTint } from '../../lib/channelTint'

export type LibraryCounts = { templates?: number; prompts?: number; backdrops?: number; overlays?: number }

export function ChannelSideRail({ channels, currentId, tab, counts }: {
  channels: Channel[]
  currentId: string
  /** tab đang mở — bấm kênh khác giữ nguyên tab */
  tab: ChannelTab
  counts: LibraryCounts
}) {
  const tt = t.channels.page
  const library: { key: string; label: string; icon: 'layer' | 'magicpen' | 'video-play' | 'magic'; count?: number; href: string }[] = [
    { key: 'templates', label: tt.library.templates, icon: 'layer', count: counts.templates, href: paths.library('templates') },
    { key: 'prompts', label: tt.library.prompts, icon: 'magicpen', count: counts.prompts, href: paths.library('prompts') },
    { key: 'backdrops', label: tt.library.backdrops, icon: 'video-play', count: counts.backdrops, href: paths.library('backdrops') },
    { key: 'overlays', label: tt.library.overlays, icon: 'magic', count: counts.overlays, href: paths.library('overlays') },
  ]
  return (
    <aside className={`vch-side ${stylex.props(channelStyles.side).className}`} aria-label={tt.rail}>
      <a className={`vch-row ${stylex.props(channelStyles.row).className}`} href={`#${paths.workbench()}`} style={{ marginBottom: 'var(--vv-s-4)' }}>
        <Icon name="video-play" size={16} className={`vch-ic ${stylex.props(channelStyles.ic).className}`} />
        <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>{channels.length > 0 ? tt.allVideos(channels.length) : tt.allVideosEmpty}</span>
      </a>
      <div className={`vch-grp vch-grp--top ${stylex.props(channelStyles.grp, channelStyles.grpTop).className}`}><span>{tt.mine}</span><span className="num">{channels.length}</span></div>
      {channels.map((channel) => (
        <a
          key={channel.id}
          className={`vch-row ${stylex.props(channelStyles.row).className}`}
          href={`#${paths.channel(channel.id, tab)}`}
          aria-current={channel.id === currentId ? 'page' : undefined}
        >
          <ChannelAvatar
            name={channel.name || tt.noName}
            src={channel.avatarUrl ?? undefined}
            color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
            size={30}
            style={{ fontSize: 10.5, fontWeight: 700 }}
          />
          <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>
            <span className={`vch-row__name ${stylex.props(channelStyles.rowName).className}`}>{channel.name || tt.noName}</span>
            <span className={`vch-row__sub ${stylex.props(channelStyles.rowSub).className}`}>{tt.created(fmtDay(channel.createdAt))}</span>
          </span>
        </a>
      ))}
      <a className={`vch-row ${stylex.props(channelStyles.row).className}`} href={`#${paths.channelNew()}`}>
        <Icon name="add-square" size={16} />
        <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>{tt.create}</span>
      </a>
      <div className={`vch-grp ${stylex.props(channelStyles.grp).className}`}><span>{tt.shared}</span></div>
      {library.map((item) => (
        <a key={item.key} className={`vch-row vch-librow ${stylex.props(channelStyles.row, channelStyles.librow).className}`} href={`#${item.href}`}>
          <Icon name={item.icon} size={16} className={`vch-ic ${stylex.props(channelStyles.ic).className}`} />
          <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>{item.label}</span>
          {item.count !== undefined && <span className={`vch-row__count num ${stylex.props(channelStyles.rowCount).className}`}>{item.count}</span>}
        </a>
      ))}
      <div className={`vch-ready ${stylex.props(channelStyles.ready).className}`}>
        <Icon name="tick-b" size={15} />
        {tt.ready}
      </div>
    </aside>
  )
}
