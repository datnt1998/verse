// Ánh xạ trạng thái quét lịch sử → giao diện thẻ (câu, nút, thanh tiến độ).
// Nguồn bảng: contract history.state + bảng câu 06-copy › Khám phá.
import { describe, expect, it } from 'vitest'
import { scanView } from './scanView'
import type { ScanStatus } from '../../api/rivals'
import { vi as t } from '../../i18n/vi'

const tt = t.screens.explore.scanStates
const bt = t.screens.explore.scanButtons
const every6 = t.screens.explore.everyHours(6)

const status = (history: Partial<ScanStatus['history']>, extra: Partial<ScanStatus> = {}): ScanStatus => ({
  channelId: 'r1',
  historyCap: 2000,
  indexedVideos: 1284,
  rescanHours: 6,
  discovery: { checkedAt: null, caughtUp: false, running: false, nextAt: null },
  history: {
    state: 'idle', jobId: 'j1', scanned: 1284, added: 1284, pages: 0,
    reachedEnd: false, startedAt: null, updatedAt: null, nextAttemptAt: null,
    errorCode: null, error: null, recovery: null, ...history,
  },
  ...extra,
})
const discovering = { discovery: { checkedAt: null, caughtUp: false, running: true, nextAt: null } }

describe('thẻ Quét lịch sử', () => {
  it.each([
    ['running', tt.running, 'pause', bt.pause],
    ['paused', tt.paused, 'resume', bt.resume],
    ['error', tt.error, 'resume', bt.resume],
    ['idle', tt.idle, 'start', bt.start],
    ['cancelled', tt.cancelled, 'start', bt.rescan],
  ] as const)('%s → câu và nút theo bảng câu', (state, text, action, label) => {
    const v = scanView(status({ state }))
    expect(v.text).toBe(text)
    expect(v.button?.action).toBe(action)
    expect(v.button?.label).toBe(label)
  })

  it('running → thanh tiến độ theo số đã quét / giới hạn', () => {
    const v = scanView(status({ state: 'running' }))
    expect(v.live).toBe(true)
    expect(v.count).toBe('1.284 / 2.000')
    expect(v.pct).toBeCloseTo(0.642)
  })

  it('queued → câu chờ lượt, không có nút', () => {
    const v = scanView(status({ state: 'queued' }))
    expect(v.text).toBe(tt.queued)
    expect(v.live).toBe(true)
    expect(v.button).toBeNull()
  })

  it('backoff → limited kèm giờ tự thử lại (HH:MM), nút Tạm dừng', () => {
    const at = new Date(Date.now() + 30 * 60 * 1000)
    const v = scanView(status({ state: 'backoff', nextAttemptAt: at.toISOString() }))
    const time = at.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(v.text).toBe(`YouTube đang giới hạn tốc độ. Trinh sát tự thử lại lúc ${time}.`)
    expect(v.live).toBe(false)
    expect(v.button?.action).toBe('pause')
  })

  it('backoff thiếu nextAttemptAt → câu "sau ít phút"', () => {
    expect(scanView(status({ state: 'backoff', nextAttemptAt: null })).text).toBe(tt.limitedNoTime)
  })

  it('capped → số đã quét bằng giới hạn và chu kỳ từ rescanHours, nút Quét lại', () => {
    const v = scanView(status({ state: 'capped' }, { indexedVideos: 2000 }))
    expect(v.text).toBe(tt.capped('2.000', every6))
    expect(v.button?.action).toBe('start')
    expect(v.button?.label).toBe(bt.rescan)
    expect(scanView(status({ state: 'capped' }, { indexedVideos: 2000, rescanHours: 1 })).text)
      .toBe(tt.capped('2.000', 'mỗi giờ'))
  })

  it('complete → tổng video của kênh, không có nút', () => {
    const v = scanView(status({ state: 'complete', reachedEnd: true }))
    expect(v.text).toBe(tt.complete('1.284', every6))
    expect(v.button).toBeNull()
  })

  it.each(['idle', 'complete', 'cancelled'] as const)('%s + đang kiểm tra video mới → câu kiểm tra, không có nút', (state) => {
    const v = scanView(status({ state }, discovering))
    expect(v.text).toBe(tt.discovery)
    expect(v.live).toBe(true)
    expect(v.button).toBeNull()
  })

  it('đang kiểm tra video mới không lấn át running/paused/capped', () => {
    expect(scanView(status({ state: 'running' }, discovering)).text).toBe(tt.running)
    expect(scanView(status({ state: 'paused' }, discovering)).text).toBe(tt.paused)
    expect(scanView(status({ state: 'capped' }, { ...discovering, indexedVideos: 2000 })).text).toBe(tt.capped('2.000', every6))
  })

  it('giới hạn nhỏ hơn số đã quét → pct kẹp 1, số hiện theo giới hạn', () => {
    const v = scanView(status({ state: 'capped' }, { historyCap: 500, indexedVideos: 900 }))
    expect(v.pct).toBe(1)
    expect(v.now).toBe(500)
    expect(v.count).toBe('500 / 500')
    expect(v.text).toBe(tt.capped('500', every6))
  })
})
