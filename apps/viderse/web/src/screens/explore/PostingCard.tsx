// Thẻ "Lịch đăng" ở cột phải (§12): radiogroup múi giờ VN / New York, danh
// sách mọi khung lặp lại (thứ · khoảng giờ · n/30 · độ đều · nhãn "nhiều nhất"),
// bấm khung → làm nổi ô trên bảng thứ × khối-2-giờ, dòng "n video lẻ ngoài các
// khung". Zone đổi → refetch posting-slots?zone=.
import { useState } from 'react'
import { Icon, Segmented } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import type { PostingSchedule, PostingZone } from '../../api/rivals'
import { gridMax, slotCells, slotRhythm } from './posting'

export function PostingCard({ schedule, zone, onZone }: {
  schedule: PostingSchedule | undefined
  zone: PostingZone
  onZone: (zone: PostingZone) => void
}) {
  const tt = t.screens.explore
  const [picked, setPicked] = useState<number | null>(null)

  const slots = schedule?.slots ?? []
  const grid = schedule?.grid ?? []
  const max = gridMax(grid)
  const pickedSlot = picked != null ? slots[picked] : undefined
  const pickedCells = pickedSlot ? slotCells(pickedSlot) : null
  const sample = schedule?.sample ?? 30
  const zoneLong = tt.postingZoneLong[zone]

  return (
    <div className="vx-cardbox" style={{ gap: 'var(--vv-s-10)' }}>
      <div className="vx-posthead">
        <span className="vx-h3"><Icon name="calendar" size={15} style={{ color: 'var(--color-fg-3)' }} />{tt.postingTitle}</span>
        <Segmented
          label={tt.postingZone}
          value={zone}
          size={24}
          radius={6}
          fit="content"
          onChange={(id) => onZone(id as PostingZone)}
          options={[
            { id: 'vn', label: 'VN', style: { paddingInline: 8, fontSize: 11.5 } },
            { id: 'ny', label: 'New York', style: { paddingInline: 8, fontSize: 11.5 } },
          ]}
          style={{ padding: 'var(--vv-s-2)', flex: '0 0 auto', gap: 'var(--vv-s-2)', borderRadius: 'var(--vv-r-8)', background: 'var(--color-surface-1)', borderColor: 'var(--line-1)' }}
          thumbStyle={{ background: 'var(--color-surface-6)' }}
        />
      </div>
      <span className="vx-postsum num">
        <span>{tt.postingSample(sample)}</span>
        <b>{tt.postingSlotCount(slots.length)}</b>
      </span>
      {schedule && slots.length === 0 && (
        <span className="vx-scantext">
          {schedule.pausedUntil
            ? tt.postingPaused(new Date(schedule.pausedUntil).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }))
            : schedule.pending > 0 ? tt.postingPending
              : schedule.total === 0 ? tt.postingEmpty : tt.postingNoSlots}
        </span>
      )}
      {slots.length > 0 && (
        <ul className="vx-slots" aria-label={tt.postingSlotsLabel}>
          {slots.map((slot, i) => {
            const on = picked === i
            return (
              <li key={i}>
                <button
                  type="button"
                  className="vx-slot"
                  aria-pressed={on}
                  onClick={() => setPicked(on ? null : i)}
                >
                  <span className="vx-slot__top">
                    <span className="vx-days">
                      {slot.weekdays.map((d) => <span className="vx-day" key={d}>{tt.weekdays[d]}</span>)}
                    </span>
                    <span className="vx-slot__time">{slot.start}–{slot.end}</span>
                    <span className="vx-grow" />
                    <span className="vx-slot__count">{slot.count}/{sample}</span>
                  </span>
                  <span className="vx-slot__bot">
                    <span className="vx-slot__bar" aria-hidden="true">
                      <span style={{ width: `${Math.min(100, Math.round((slot.count / Math.max(1, slots[0]?.count ?? slot.count)) * 100))}%` }} />
                    </span>
                    {slot.isTop && <span className="vx-topchip">{tt.postingTop}</span>}
                    <span className="vx-slot__rhythm">{slotRhythm(slot)}</span>
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
      {(schedule?.offSlot ?? 0) > 0 && <span className="vx-offslot">{tt.postingOff(schedule!.offSlot)}</span>}
      {max > 0 && (
        <div role="img" aria-label={tt.postingHeat(zoneLong, slots.length)} className="vx-heat">
          {tt.weekdays.map((day, di) => (
            <div className="vx-heatrow" key={day}>
              <span>{day}</span>
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
                    className="vx-heatcell"
                    style={{ background: bg, boxShadow: inSlot ? '0 0 0 1.5px var(--color-fg)' : undefined }}
                  />
                )
              })}
            </div>
          ))}
          <div className="vx-heatx"><span /><span>0h</span><span>6h</span><span>12h</span><span>18h</span></div>
        </div>
      )}
      {schedule && slots.length > 0 && schedule.pending > 0 && <span className="vx-scantext">{tt.postingPending}</span>}
    </div>
  )
}
