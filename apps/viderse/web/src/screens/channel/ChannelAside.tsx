// Cột phải trang Kênh (artboard Channel › aside 316 px): khối “Đang áp
// dụng cho kênh” chỉ đọc với nhãn “riêng” khi khác mặc định kênh mới và lối
// “Tuỳ chỉnh” mở đúng tab, rồi khối “Lịch đăng”.
// KHÔNG dựng (chưa có hàng chờ/mức tự chủ): thanh “Hôm nay x/y video” (cần
// clone job), dòng Mức tự chủ, hộp “Dặn trợ lý”.
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/vi'
import type { Channel } from '../../api/channels'
import { appliedRows } from './applied'
import { ScheduleCard } from './ScheduleCard'

export function ChannelAside({ channel }: { channel: Channel }) {
  const tt = t.channels.applied
  return (
    <aside className="vch-aside" aria-label={t.channels.page.aside}>
      <section className="vch-card vch-applied" aria-label={tt.title}>
        <div className="vch-applied__head">
          <span className="vch-applied__title">{tt.title}</span>
          <span className="vch-applied__legend">{tt.legend}</span>
        </div>
        {appliedRows(channel).map((row) => (
          <div className="vch-applied__row" key={row.key}>
            <span className="vch-applied__text">
              <span className="vch-applied__label">
                {row.label}
                {row.own && <span className="vch-refbadge">{tt.own}</span>}
              </span>
              <span className="vch-applied__value" title={row.value}>{row.value}</span>
            </span>
            <a className="vch-applied__link" href={row.href}>
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
