import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { JobsPill } from './JobsPill'
import type { Status } from '../api/status'

const queue = vi.hoisted(() => ({ data: { pausedAt: new Date(2026, 0, 1, 0, 4).toISOString(), workers: { used: 2, total: 2 } } }))
vi.mock('../data/useResource', () => ({ useResource: () => queue }))
const base: Status = { running: 2, queued: 12, paused: false, reviewCount: 0, attention: [] }
const render = (status = base, online = true, compact = false) => renderToStaticMarkup(<JobsPill status={status} online={online} compact={compact} mobileBar={compact} />)

describe('JobsPill canonical four states', () => {
  it('shows real running counts and announces their meaning on mobile', () => {
    expect(render()).toContain('2 đang chạy · 12 chờ lượt')
    expect(render()).toContain('Máy đang chạy 2/2 luồng · 12 video chờ lượt')
    const mobile = render(base, true, true)
    expect(mobile).toContain('2 · 12')
    expect(mobile).toContain('aria-label="Việc nền: 2 đang chạy · 12 chờ lượt"')
    expect(mobile).toContain('data-jobs="running"')
  })
  it('uses idle only when neither a running nor a queued video exists', () => {
    const idle = { ...base, running: 0, queued: 0 }
    expect(render(idle)).toContain('>Máy rảnh</span>')
    expect(render(idle, true, true)).toContain('>Rảnh</span>')
    expect(render({ ...base, running: 0 })).toContain('data-jobs="running"')
  })
  it('uses the persisted pause time and a pause icon rather than a rec dot', () => {
    const paused = { ...base, paused: true }
    expect(render(paused)).toContain('Đã tạm dừng lúc 00:04 · 12 chờ lượt')
    expect(render(paused)).not.toContain('vv-jobs__dot')
    expect(render(paused, true, true)).toContain('>12</span>')
    expect(render(paused)).toContain('video đang chạy làm nốt bước hiện tại')
  })
  it('never presents stale counts as current while offline', () => {
    const desktop = render(base, false)
    expect(desktop).toContain('Mất kết nối · đang thử lại')
    expect(desktop).not.toContain('vv-jobs__dot')
    expect(desktop).not.toContain('2 đang chạy')
    expect(render(base, false, true)).toContain('>Mất kết nối</span>')
  })
})
