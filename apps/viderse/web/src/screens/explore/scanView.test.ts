// Ánh xạ trạng thái quét lịch sử → giao diện thẻ (văn bản, nút, thanh tiến độ).
// Nguồn bảng: contract history.state + các tweak state của artboard.
import { describe, expect, it } from 'vitest'
import { scanView } from './scanView'
import type { ScanStatus } from '../../api/rivals'
import { vi as t } from '../../i18n/vi'

const tt = t.screens.explore.scanStates

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

describe('thẻ Quét lịch sử', () => {
  it('running → đang chạy, nút Tạm dừng', () => {
    const v = scanView(status({ state: 'running' }))
    expect(v.text).toBe(tt.running)
    expect(v.live).toBe(true)
    expect(v.button?.action).toBe('pause')
    expect(v.count).toBe('1.284 / 2.000')
    expect(v.pct).toBeCloseTo(0.642)
  })

  it('queued → cũng đang chạy nhưng text chờ hàng', () => {
    const v = scanView(status({ state: 'queued' }))
    expect(v.text).toBe(tt.queued)
    expect(v.live).toBe(true)
    expect(v.button?.action).toBe('pause')
  })

  it('backoff → limited kèm giờ tự thử lại (HH:MM)', () => {
    const at = new Date(Date.now() + 30 * 60 * 1000)
    const v = scanView(status({ state: 'backoff', nextAttemptAt: at.toISOString() }))
    const time = at.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(v.text).toBe(`YouTube đang giới hạn tốc độ. Trinh sát tự thử lại lúc ${time}.`)
    expect(v.live).toBe(false)
    expect(v.button?.action).toBe('pause')
  })

  it('backoff thiếu nextAttemptAt → text không giờ', () => {
    const v = scanView(status({ state: 'backoff', nextAttemptAt: null }))
    expect(v.text).toBe(tt.limitedNoTime)
  })

  it('paused → đã tạm dừng, nút Tiếp tục', () => {
    const v = scanView(status({ state: 'paused' }))
    expect(v.text).toBe(tt.paused)
    expect(v.live).toBe(false)
    expect(v.button?.action).toBe('resume')
  })

  it('capped → done, nút Quét lại (start)', () => {
    const v = scanView(status({ state: 'capped' }))
    // Artboard: "Đã quét đủ giới hạn. Video mới vẫn được kiểm tra mỗi giờ." —
    // chu kỳ lấy từ rescanHours của API (6 giờ ở fixture này).
    expect(v.text).toBe(tt.capped(t.screens.explore.everyHours(6)))
    expect(v.button?.action).toBe('start')
    expect(v.button?.label).toBe(t.screens.explore.scanButtons.rescan)
  })

  it('capped với chu kỳ 1 giờ → đúng câu "mỗi giờ" của artboard', () => {
    const v = scanView(status({ state: 'capped' }, { rescanHours: 1 }))
    expect(v.text).toBe('Đã quét đủ giới hạn. Video mới vẫn được kiểm tra mỗi giờ.')
  })

  it('complete → done hết lịch sử, nút Quét lại', () => {
    const v = scanView(status({ state: 'complete', reachedEnd: true }))
    expect(v.text).toBe(tt.complete(t.screens.explore.everyHours(6)))
    expect(v.button?.action).toBe('start')
  })

  it('idle → chưa quét, nút Quét lịch sử', () => {
    const v = scanView(status({ state: 'idle', jobId: null }))
    expect(v.text).toBe(tt.idle)
    expect(v.button?.action).toBe('start')
    expect(v.button?.label).toBe(t.screens.explore.scanButtons.start)
  })

  it('cancelled → đã huỷ, nút Quét lại', () => {
    const v = scanView(status({ state: 'cancelled' }))
    expect(v.text).toBe(tt.cancelled)
    expect(v.button?.action).toBe('start')
  })

  it('error → dùng recovery của API, nút Tiếp tục', () => {
    const v = scanView(status({ state: 'error', recovery: 'Thử lại lúc 15:30.', errorCode: 'rate_limited' }))
    expect(v.text).toBe('Thử lại lúc 15:30.')
    expect(v.button?.action).toBe('resume')
  })

  it('error không recovery → text lỗi chung', () => {
    const v = scanView(status({ state: 'error', recovery: null, error: null }))
    expect(v.text).toBe(tt.error)
  })

  it('discovery.running lấn át text khi lịch sử đứng yên (idle)', () => {
    const v = scanView(status({ state: 'idle', jobId: null }, { discovery: { checkedAt: null, caughtUp: false, running: true, nextAt: null } }))
    expect(v.text).toBe(tt.discovery)
    expect(v.live).toBe(true)
  })

  it('discovery.running không lấn át text running/queued/paused', () => {
    const s = status({ state: 'running' }, { discovery: { checkedAt: null, caughtUp: false, running: true, nextAt: null } })
    expect(scanView(s).text).toBe(tt.running)
  })

  it('cap nhỏ hơn indexed → pct kẹp 1 và valuemax theo cap', () => {
    const v = scanView(status({ state: 'capped' }, { historyCap: 500, indexedVideos: 900 }))
    expect(v.pct).toBe(1)
    expect(v.now).toBe(500)
    // Số hiện không vượt giới hạn đang đặt (tránh "20.000 / 2.000" phi lý)
    expect(v.count).toBe('500 / 500')
  })
})
