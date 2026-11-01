import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Cột phải trang Kênh (artboard Channel › aside 316 px): khối “Đang áp
// dụng cho kênh” chỉ đọc với nhãn “riêng” khi khác mặc định kênh mới và lối
// “Tuỳ chỉnh” mở đúng tab, rồi khối “Lịch đăng”.
// KHÔNG dựng (chưa có hàng chờ/mức tự chủ): thanh “Hôm nay x/y video” (cần
// clone job), dòng Mức tự chủ, hộp “Dặn trợ lý”.
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/channels'
import type { Channel } from '../../api/channels'
import { appliedRows } from './applied'
import { ScheduleCard } from './ScheduleCard'

export function ChannelAside({ channel }: { channel: Channel }) {
  const tt = t.channels.applied
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
      </section>
      <ScheduleCard channel={channel} variant="aside" />
    </aside>
  )
}
