import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Thẻ "Lịch đăng" ở cột phải (§12): radiogroup múi giờ VN / New York, danh
// sách mọi khung lặp lại (thứ · khoảng giờ · n/30 · độ đều · nhãn "nhiều nhất"),
// bấm khung → làm nổi ô trên bảng thứ × khối-2-giờ, dòng "n video lẻ ngoài các
// khung". Zone đổi → refetch posting-slots?zone=.
import { useState } from 'react'
import { Icon } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
import { t } from '../../i18n/explore'
import type { PostingSchedule, PostingZone } from '../../api/rivals'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { gridMax, slotCells, slotRhythm } from './posting'
import { TOUCH_COLUMN } from './media'

export function PostingCard({ schedule, zone, onZone }: {
  schedule: PostingSchedule | undefined
  zone: PostingZone
  onZone: (zone: PostingZone) => void
}) {
  const tt = t.screens.explore
  const [picked, setPicked] = useState<number | null>(null)
  const touch = useMediaQuery(TOUCH_COLUMN)

  const slots = schedule?.slots ?? []
  const grid = schedule?.grid ?? []
  const max = gridMax(grid)
  const pickedSlot = picked != null ? slots[picked] : undefined
  const pickedCells = pickedSlot ? slotCells(pickedSlot) : null
  const sample = schedule?.sample ?? 30
  const zoneLong = tt.postingZoneLong[zone]

  return (
    <div className={`vx-cardbox ${stylex.props(exploreStyles.cardbox).className}`} style={{ gap: 'var(--vv-s-10)' }}>
      <div className={`vx-posthead ${stylex.props(exploreStyles.posthead).className}`}>
        <span className={`vx-h3 ${stylex.props(exploreStyles.h3).className}`}><Icon name="calendar" size={15} />{tt.postingTitle}</span>
        <Segmented
          label={tt.postingZone}
          value={zone}
          size={touch ? 44 : 'sm'}
          fit="content"
          onChange={(id) => onZone(id as PostingZone)}
          options={[
            { id: 'vn', label: 'VN' },
            { id: 'ny', label: 'New York' },
          ]}
        />
      </div>
      <span className={`vx-postsum num ${stylex.props(exploreStyles.postsum).className}`}>
        <span>{tt.postingSample(sample)}</span>
        <b {...stylex.props(exploreStyles.postsumValue)}>{tt.postingSlotCount(slots.length)}</b>
      </span>
      {schedule && slots.length === 0 && (
        <span className={`vx-scantext ${stylex.props(exploreStyles.scantext).className}`}>
          {schedule.pausedUntil
            ? tt.postingPaused(new Date(schedule.pausedUntil).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }))
            : schedule.pending > 0 ? tt.postingPending
              : schedule.total === 0 ? tt.postingEmpty : tt.postingNoSlots(schedule.total)}
        </span>
      )}
      {slots.length > 0 && (
        <ul className={`vx-slots ${stylex.props(exploreStyles.slots).className}`} aria-label={tt.postingSlotsLabel}>
          {slots.map((slot, i) => {
            const on = picked === i
            return (
              <li key={i}>
                <button
                  type="button"
                  className={`vx-slot ${stylex.props(exploreStyles.slot).className}`}
                  aria-pressed={on}
                  onClick={() => setPicked(on ? null : i)}
                >
                  <span className={`vx-slot__top ${stylex.props(exploreStyles.slotTop).className}`}>
                    <span className={`vx-days ${stylex.props(exploreStyles.days).className}`}>
                      {slot.weekdays.map((d) => <span className={`vx-day ${stylex.props(exploreStyles.day).className}`} key={d}>{tt.weekdays[d]}</span>)}
                    </span>
                    <span className={`vx-slot__time ${stylex.props(exploreStyles.slotTime).className}`}>{slot.start}–{slot.end}</span>
                    <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
                    <span className={`vx-slot__count ${stylex.props(exploreStyles.slotCount).className}`}>{slot.count}/{sample}</span>
                  </span>
                  <span className={`vx-slot__bot ${stylex.props(exploreStyles.slotBot).className}`}>
                    <span className={`vx-slot__bar ${stylex.props(exploreStyles.slotBar).className}`} aria-hidden="true">
                      <span {...stylex.props(exploreStyles.slotBarFill)} style={{ width: `${Math.min(100, Math.round((slot.count / Math.max(1, slots[0]?.count ?? slot.count)) * 100))}%` }} />
                    </span>
                    {slot.isTop && <span className={`vx-topchip ${stylex.props(exploreStyles.topchip).className}`}>{tt.postingTop}</span>}
                    <span className={`vx-slot__rhythm ${stylex.props(exploreStyles.slotRhythm).className}`}>{slotRhythm(slot)}</span>
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
      {/* "lẻ ngoài các khung trên" chỉ có nghĩa khi có khung; không có khung thì câu trên đã nói. */}
      {slots.length > 0 && (schedule?.offSlot ?? 0) > 0 && <span className={`vx-offslot ${stylex.props(exploreStyles.offslot).className}`}>{tt.postingOff(schedule!.offSlot)}</span>}
      {max > 0 && (
        <div role="img" aria-label={tt.postingHeat(zoneLong, slots.length)} className={`vx-heat ${stylex.props(exploreStyles.heat).className}`}>
          {tt.weekdays.map((day, di) => (
            <div className={`vx-heatrow ${stylex.props(exploreStyles.heatrow).className}`} key={day}>
              <span {...stylex.props(exploreStyles.heatLabel)}>{day}</span>
              {(grid[di] ?? Array(12).fill(0)).map((n: number, bi: number) => {
                const inSlot = pickedCells?.has(`${di}:${bi}`) ?? false
                const alpha = n === 0 ? 0 : 0.2 + (n / max) * 0.75
                const dim = pickedCells && !inSlot
                const bg = n === 0
                  ? 'var(--vv-heat-empty)'
                  : `color-mix(in srgb, var(--color-fg) ${Math.round((dim ? Math.min(alpha, 0.18) : alpha) * 100)}%, transparent)`
                return (
                  <span
                    key={bi}
                    className={`vx-heatcell ${stylex.props(exploreStyles.heatcell).className}`}
                    style={{ background: bg, boxShadow: inSlot ? '0 0 0 1.5px var(--color-fg)' : undefined }}
                  />
                )
              })}
            </div>
          ))}
          <div className={`vx-heatx ${stylex.props(exploreStyles.heatx).className}`}><span /><span>0h</span><span>6h</span><span>12h</span><span>18h</span></div>
        </div>
      )}
      {schedule && slots.length > 0 && schedule.pending > 0 && <span className={`vx-scantext ${stylex.props(exploreStyles.scantext).className}`}>{tt.postingPending}</span>}
    </div>
  )
}
