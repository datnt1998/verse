import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
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
import { t } from '../../i18n/channels'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getPostingSlots, type PostingZone } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { mergePostingSchedules, scheduleTotals, type ScheduleInput } from './schedule'
import { mch } from './mobile/mchannel.stylex'

const ZONES: PostingZone[] = ['vn', 'ny']

export function ScheduleCard({ channel, variant = 'aside' }: {
  channel: Channel
  /** aside = cột phải; tab = bản trong tab Nguồn khi cột phải bị ẩn */
  variant?: 'aside' | 'tab' | 'mobile'
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

  if (variant === 'mobile') {
    return (
      <section {...stylex.props(mch.card16)} aria-label={tt.title}>
        <div {...stylex.props(mch.scheduleHead)}>
          <Icon name="calendar" size={16} />
          <h2 {...stylex.props(mch.scheduleTitle)}>{tt.title}</h2>
          <button
            type="button"
            aria-label={tt.zone}
            onClick={() => setZone(zone === 'vn' ? 'ny' : 'vn')}
            {...stylex.props(mch.scheduleZone, mch.buttonReset)}
          >
            {tt.zones[zone]}
          </button>
        </div>
        {sources.length === 0 ? (
          <p {...stylex.props(mch.hint)}>{tt.none}</p>
        ) : data.initialLoading && !data.data ? (
          <p {...stylex.props(mch.hint)}>{tt.loading}</p>
        ) : (
          <>
            <span className={`num ${stylex.props(mch.scheduleSum).className}`}>{tt.summary(totals.videos, totals.rivals, slots.length)}</span>
            {slots.length === 0
              ? <p {...stylex.props(mch.hint)}>{tt.empty}</p>
              : (
                <ul aria-label={tt.rowsLabel} {...stylex.props(mch.listReset)}>
                  {slots.map((slot) => (
                    <li key={slot.key} {...stylex.props(mch.scheduleRow)}>
                      <span {...stylex.props(mch.scheduleDay)}>{slot.weekdays.map((day) => t.screens.explore.weekdays[day]).join(' · ')}</span>
                      <span className={`num ${stylex.props(mch.scheduleTime).className}`}>{slot.start}–{slot.end}</span>
                      <span {...stylex.props(mch.scheduleWho)} title={slot.rivals.map((rival) => rival.title).join(' · ')}>{slot.rivals.map((rival) => rival.title).join(' · ')}</span>
                      <span className={`num ${stylex.props(mch.scheduleCount).className}`}>{tt.slotVideos(slot.count)}</span>
                    </li>
                  ))}
                </ul>
              )}
          </>
        )}
        {sources.length > 0 && (
          <a href={`#${paths.explore()}`} {...stylex.props(mch.scheduleMore)}>{tt.explore} <Icon name="arrow-right-2" size={14} /></a>
        )}
      </section>
    )
  }

  return (
    <section className={`vch-card vch-schedule${variant === 'aside' ? '' : ' vch-schedule--tab'} ${stylex.props(channelStyles.card, variant !== 'aside' && channelStyles.scheduleTab).className}`} aria-label={tt.title}>
      <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
        <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="calendar" size={16} />{variant === 'aside' ? tt.titleShort : tt.title}</h2>
        <div role="radiogroup" aria-label={tt.zone} className={`vch-zone ${stylex.props(channelStyles.zone).className}`}>
          {ZONES.map((id) => (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={zone === id}
              className={`vch-zone__btn ${stylex.props(channelStyles.zoneBtn).className}`}
              onClick={() => setZone(id)}
            >
              {tt.zones[id]}
            </button>
          ))}
        </div>
      </div>

      {sources.length === 0 ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.none}</p>
      ) : data.initialLoading && !data.data ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.loading}</p>
      ) : (
        <>
          <span className={`vch-schedule__sum num ${stylex.props(channelStyles.scheduleSum).className}`}>{tt.summary(totals.videos, totals.rivals, slots.length)}</span>
          {slots.length === 0 ? (
            <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.empty}</p>
          ) : (
            <ul className={`vch-slots ${stylex.props(channelStyles.slots).className}`} aria-label={tt.rowsLabel}>
              {slots.map((slot) => (
                <li key={slot.key} className={`vch-slot ${stylex.props(channelStyles.slot).className}`}>
                  <span className={`vch-slot__when ${stylex.props(channelStyles.slotWhen).className}`}>
                    <span className={`vch-slot__days ${stylex.props(channelStyles.slotDays).className}`}>{slot.weekdays.map((day) => t.screens.explore.weekdays[day]).join(' · ')}</span>
                    <span className={`vch-slot__time num ${stylex.props(channelStyles.slotTime).className}`}>{slot.start}–{slot.end}</span>
                  </span>
                  <span className={`vch-slot__count num ${stylex.props(channelStyles.slotCount).className}`}>{tt.slotVideos(slot.count)}</span>
                  <span className={`vch-slot__rivals ${stylex.props(channelStyles.slotRivals).className}`}>
                    <span className={`vch-stack ${stylex.props(channelStyles.stack).className}`} aria-hidden="true">
                      {slot.rivals.slice(0, 3).map((rival) => (
                        <ChannelAvatar {...stylex.props(channelStyles.stackItem)} key={rival.id} name={rival.title} size={16} stacked style={{ fontSize: 7 }} />
                      ))}
                    </span>
                    <span className={`vch-slot__names ${stylex.props(channelStyles.slotNames).className}`} title={slot.rivals.map((rival) => rival.title).join(' · ')}>
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
        <a className={`vch-schedule__more ${stylex.props(channelStyles.scheduleMore).className}`} href={`#${paths.explore()}`}>{tt.explore}</a>
      )}
    </section>
  )
}
