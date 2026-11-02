import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Cột phải trang Kênh (artboard Channel › aside 316 px): khối “Đang áp
// dụng cho kênh” chỉ đọc với nhãn “riêng” khi khác mặc định kênh mới, thanh
// “Hôm nay x/y video” đọc từ channel summary, rồi khối “Lịch đăng”.
// Hộp “Dặn trợ lý” chưa có dữ liệu API nên chưa dựng.
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/channels'
import type { Channel } from '../../api/channels'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { useResource } from '../../data/useResource'
import { appliedRows } from './applied'
import { ScheduleCard } from './ScheduleCard'

export function ChannelAside({ channel }: { channel: Channel }) {
  const tt = t.channels.applied
  const summaries = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const today = summaries.data?.find((summary) => summary.channelId === channel.id)?.today
  const cap = today?.cap ?? channel.pacePerDay
  const done = today?.done ?? 0
  const quotaPct = cap > 0 ? Math.min(100, Math.round((done / cap) * 100)) : 0
  return (
    <aside className={`vch-aside ${stylex.props(channelStyles.aside).className}`} aria-label={t.channels.page.aside}>
      <section className={`vch-card vch-applied ${stylex.props(channelStyles.card, channelStyles.applied).className}`} aria-label={tt.title}>
        <div className={`vch-applied__head ${stylex.props(channelStyles.appliedHead).className}`}>
          <span className={`vch-applied__title ${stylex.props(channelStyles.appliedTitle).className}`}>{tt.title}</span>
          <span className={`vch-applied__legend ${stylex.props(channelStyles.appliedLegend).className}`}>{tt.legend}</span>
        </div>
        {appliedRows(channel).map((row) => (
          <div className={`vch-applied__row ${stylex.props(channelStyles.appliedRow).className}`} key={row.key}>
            <span className={`vch-applied__text ${stylex.props(channelStyles.appliedText).className}`}>
              <span className={`vch-applied__label ${stylex.props(channelStyles.appliedLabel).className}`}>
                {row.label}
                {row.own && <span className={`vch-refbadge ${stylex.props(channelStyles.refbadge).className}`}>{tt.own}</span>}
              </span>
              <span className={`vch-applied__value ${stylex.props(channelStyles.appliedValue).className}`} title={row.value}>{row.value}</span>
            </span>
            <a className={`vch-applied__link ${stylex.props(channelStyles.appliedLink).className}`} href={row.href}>
              {row.linkLabel}
              {row.key !== 'ai' && <Icon name="arrow-r" size={12} />}
            </a>
          </div>
        ))}
        <div className={`vch-quota ${stylex.props(channelStyles.quota).className}`} aria-label={tt.today}>
          <div className={`vch-quota__row ${stylex.props(channelStyles.quotaRow).className}`}>
            <span>{tt.today}</span>
            <span className="num">{tt.todayValue(done, cap)}</span>
          </div>
          <div className={`vch-quota__bar ${stylex.props(channelStyles.quotaBar).className}`} role="progressbar"
            aria-valuemin={0} aria-valuemax={cap} aria-valuenow={done} aria-label={tt.todayValue(done, cap)}>
            <div className={stylex.props(channelStyles.quotaFill).className} style={{ width: `${quotaPct}%` }} />
          </div>
        </div>
      </section>
      <ScheduleCard channel={channel} variant="aside" />
    </aside>
  )
}
