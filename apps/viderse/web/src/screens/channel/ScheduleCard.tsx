// Khối “Lịch đăng” ở cột phải trang Kênh (artboard Channel › aside) và ở tab
// Nguồn khi màn hẹp không có cột phải:
// toggle VN / New York, câu "N video gần nhất của k đối thủ · m khung lặp lại",
// từng khung gộp (thứ · giờ · số video · avatar đối thủ · tên), lối xem từng đối
// thủ trong Khám phá. Dữ liệu thật: posting-slots từng đối thủ liên kết, gộp
// bằng hàm thuần `mergePostingSchedules` (không nhân bản logic của Khám phá —
// dùng chung kiểu và hằng số ngày).
import { useState } from 'react'
import { Icon } from '@noverse/ui'
import { ChannelAvatar } from '../../components'
import { vi as t } from '../../i18n/vi'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getPostingSlots, type PostingZone } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { mergePostingSchedules, scheduleTotals, type ScheduleInput } from './schedule'

const ZONES: PostingZone[] = ['vn', 'ny']

export function ScheduleCard({ channel, variant = 'aside' }: {
  channel: Channel
  /** aside = cột phải; tab = bản trong tab Nguồn khi cột phải bị ẩn */
  variant?: 'aside' | 'tab'
}) {
  const tt = t.channels.schedule
  const [zone, setZone] = useState<PostingZone>('vn')
  const sources = channel.sources

  const data = useResource(
    keys.channelSchedule(channel.id, zone),
    async ({ signal }) => {
      const rows = await Promise.all(sources.map(async (source): Promise<ScheduleInput> => {
        try {
          return { rivalId: source.rivalId, rivalTitle: source.title, schedule: await getPostingSlots(source.rivalId, zone, signal) }
        } catch {
          return { rivalId: source.rivalId, rivalTitle: source.title, schedule: null }
        }
      }))
      return rows
    },
  )

  const inputs = data.data ?? []
  const slots = mergePostingSchedules(inputs)
  const totals = scheduleTotals(inputs)

  return (
    <section className={variant === 'aside' ? 'vch-card vch-schedule' : 'vch-card vch-schedule vch-schedule--tab'} aria-label={tt.title}>
      <div className="vch-cardhead" style={{ padding: 0 }}>
        <h2><Icon name="calendar" size={16} />{variant === 'aside' ? tt.titleShort : tt.title}</h2>
        <div role="radiogroup" aria-label={tt.zone} className="vch-zone">
          {ZONES.map((id) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={zone === id}
              className="vch-zone__btn"
              onClick={() => setZone(id)}
            >
              {tt.zones[id]}
            </button>
          ))}
        </div>
      </div>

      {sources.length === 0 ? (
        <p className="vch-hint">{tt.none}</p>
      ) : data.initialLoading && !data.data ? (
        <p className="vch-hint">{tt.loading}</p>
      ) : (
        <>
          <span className="vch-schedule__sum num">{tt.summary(totals.videos, totals.rivals, slots.length)}</span>
          {slots.length === 0 ? (
            <p className="vch-hint">{tt.empty}</p>
          ) : (
            <ul className="vch-slots" aria-label={tt.rowsLabel}>
              {slots.map((slot) => (
                <li key={slot.key} className="vch-slot">
                  <span className="vch-slot__when">
                    <span className="vch-slot__days">{slot.weekdays.map((day) => t.screens.explore.weekdays[day]).join(' · ')}</span>
                    <span className="vch-slot__time num">{slot.start}–{slot.end}</span>
                  </span>
                  <span className="vch-slot__count num">{tt.slotVideos(slot.count)}</span>
                  <span className="vch-slot__rivals">
                    <span className="vch-stack" aria-hidden="true">
                      {slot.rivals.slice(0, 3).map((rival) => (
                        <ChannelAvatar key={rival.id} name={rival.title} size={16} stacked style={{ fontSize: 7 }} />
                      ))}
                    </span>
                    <span className="vch-slot__names" title={slot.rivals.map((rival) => rival.title).join(' · ')}>
                      {slot.rivals.map((rival) => rival.title).join(' · ')}
                    </span>
                  </span>
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      {sources.length > 0 && (
        <a className="vch-schedule__more" href={`#${paths.explore()}`}>{tt.explore}</a>
      )}
    </section>
  )
}
