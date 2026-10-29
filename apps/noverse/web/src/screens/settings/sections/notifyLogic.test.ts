import { describe, expect, it } from 'vitest'
import { NOTIFY_KINDS, NOTIFY_ROWS, notifyStates, toggleNotify } from './notifyLogic'

describe('Báo động máy chủ', () => {
  it('artboard: 6 hàng, đúng nhãn; stop_guard không có hàng nhưng vẫn nằm trong danh sách sự kiện', () => {
    expect(NOTIFY_ROWS.map((row) => row.label)).toEqual([
      'Truyện chạy xong', 'Sắp hết ngân sách', 'Chương chờ duyệt', 'Trợ lý bị kẹt lặp lại', 'Bắt đầu kế hoạch tập mới', 'Tiến trình nền bị lỗi',
    ])
    expect(NOTIFY_ROWS.some((row) => (row.kind as string) === 'stop_guard')).toBe(false)
    expect(NOTIFY_KINDS).toContain('stop_guard')
  })

  it('notifyStates: không có events = bật hết; có danh sách = chỉ các sự kiện đó; enabled=false = tắt hết', () => {
    expect(Object.values(notifyStates(undefined)).every(Boolean)).toBe(true)
    expect(Object.values(notifyStates({ command: 'x' })).every(Boolean)).toBe(true)
    const some = notifyStates({ events: ['run_end', 'budget'] })
    expect(some.run_end).toBe(true)
    expect(some.budget).toBe(true)
    expect(some.deadlock).toBe(false)
    expect(Object.values(notifyStates({ enabled: false, events: ['run_end'] })).some(Boolean)).toBe(false)
  })

  it('toggleNotify: tắt một sự kiện ghi danh sách tường minh = mọi sự kiện trừ sự kiện đó', () => {
    const patch = toggleNotify(undefined, 'plan_start', false)
    expect(patch.enabled).toBeUndefined()
    expect(patch.events).toEqual(NOTIFY_KINDS.filter((kind) => kind !== 'plan_start'))
  })

  it('toggleNotify: bật lại sự kiện cuối cùng → xoá khoá events', () => {
    const all = NOTIFY_KINDS.filter((kind) => kind !== 'plan_start')
    expect(toggleNotify({ events: [...all] }, 'plan_start', true)).toEqual({ enabled: undefined, events: undefined })
  })

  it('toggleNotify: tắt sự kiện cuối cùng → enabled=false (danh sách rỗng bị hiểu là bật hết); bật lại từ đó chỉ bật sự kiện đó', () => {
    expect(toggleNotify({ events: ['run_end'] }, 'run_end', false)).toEqual({ enabled: false, events: undefined })
    expect(toggleNotify({ enabled: false }, 'budget', true)).toEqual({ enabled: undefined, events: ['budget'] })
  })
})
