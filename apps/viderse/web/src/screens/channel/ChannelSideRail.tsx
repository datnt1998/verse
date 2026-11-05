import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Cột trái trang Kênh (artboard Channel r44 › aside "Kênh của tôi"): chỉ
// danh sách kênh của tôi (avatar nhận diện hoặc chữ cái, dòng phụ nhịp hôm
// nay hoặc ngày tạo, chấm đỏ khi có video chờ bạn) + "Tạo kênh", chân "Máy
// sẵn sàng". Không còn mục Thư viện (r44: thư viện vào top nav riêng).
// Dòng phụ đọc nhịp hôm nay và số video chờ bạn từ /api/channels/summary
// (artboard: "Hôm nay 3/5 · 2 chờ bạn").
import { Icon } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { t } from '../../i18n/channels'
import { paths, type ChannelTab } from '../../router'
import type { Channel } from '../../api/channels'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { useResource } from '../../data/useResource'
import { fmtDay } from './fmt'
import { channelTint } from '../../lib/channelTint'

export function ChannelSideRail({ channels, currentId, tab }: {
  channels: Channel[]
  currentId: string
  /** tab đang mở — bấm kênh khác giữ nguyên tab */
  tab: ChannelTab
}) {
  const tt = t.channels.page
  // Artboard Channel › rail: mỗi kênh đọc “Hôm nay x/y · n chờ bạn” từ
  // /api/channels/summary; chưa có summary thì giữ ngày tạo.
  const summaries = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  return (
    <aside className={`vch-side ${stylex.props(channelStyles.side).className}`} aria-label={tt.rail}>
      <div className={`vch-grp vch-grp--top ${stylex.props(channelStyles.grp, channelStyles.grpTop).className}`}><span>{tt.mine}</span><span className="num">{channels.length}</span></div>
      {channels.map((channel) => {
        const summary = summaries.data?.find((item) => item.channelId === channel.id)
        const sub = summary
          ? summary.waitingYou > 0
            ? tt.railTodayWait(summary.today.done, summary.today.cap, summary.waitingYou)
            : tt.railToday(summary.today.done, summary.today.cap)
          : tt.created(fmtDay(channel.createdAt))
        return (
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
              <span className={`vch-row__sub ${stylex.props(channelStyles.rowSub).className}`} title={sub}>{sub}</span>
            </span>
            {summary && summary.waitingYou > 0 && (
              <span className={stylex.props(channelStyles.recdot, channelStyles.recdotWarn).className} role="img" aria-label={tt.railWaitingAria(summary.waitingYou)} />
            )}
          </a>
        )
      })}
      <a className={`vch-row ${stylex.props(channelStyles.row).className}`} href={`#${paths.channelNew()}`}>
        <Icon name="add-square" size={16} />
        <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>{tt.create}</span>
      </a>
      <div className={`vch-ready ${stylex.props(channelStyles.ready).className}`}>
        <Icon name="tick-b" size={15} />
        {tt.ready}
      </div>
    </aside>
  )
}
