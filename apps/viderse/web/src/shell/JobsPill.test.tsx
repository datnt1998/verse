// @vitest-environment happy-dom
// Nút Việc nền trên điện thoại (Gói V32, 04-components › Đầu trang điện thoại ›
// AppBar): nút tròn 36px — đang chạy = vòng tiến độ + số, rảnh = chỉ icon
// activity, tạm dừng = vòng xám đầy + icon pause, mất kết nối = icon danger.
// Không còn chữ “Rảnh”/“2 · 12” trên điện thoại; câu đầy đủ nằm ở aria-label
// (06-copy › Thanh trên điện thoại và Thông báo).
import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { JobsPill } from './JobsPill'
import { t } from '../i18n/vi'
import type { Status } from '../api/status'

const queue = vi.hoisted(() => ({
  data: {
    pausedAt: new Date(2026, 0, 1, 0, 4).toISOString(),
    workers: { used: 2, total: 2 },
    running: [{ stepIndex: 4 }, { stepIndex: 4 }],
  },
}))
vi.mock('../data/useResource', () => ({ useResource: () => queue }))
const base: Status = { running: 2, queued: 12, paused: false, reviewCount: 0, attention: [] }
const dot = /class="vv-notice-dot/
/** Bước 4/8 của mọi việc đang chạy → 5/9 vòng; 106,8 là chu vi r 17. */
const partway = String(Number((106.8 * (1 - 5 / 9)).toFixed(1)))
const render = (status = base, options: { online?: boolean; unread?: boolean | number; onOpen?: () => void; expanded?: boolean } = {}) =>
  renderToStaticMarkup(<JobsPill status={status} online={options.online ?? true} unread={options.unread} mobileBar onOpen={options.onOpen} expanded={options.expanded} />)
const desktop = (status = base, online = true) => renderToStaticMarkup(<JobsPill status={status} online={online} />)

describe('Nút Việc nền trên điện thoại', () => {
  it('đang chạy: vòng tiến độ trung bình + số việc, không còn viên chữ', () => {
    const html = render()
    expect(html).toContain(`aria-label="${t.shell.jobsMobileRunning(2, 12)}"`)
    expect(html).toContain('aria-label="Việc nền: 2 đang chạy, 12 chờ lượt"')
    expect(html).toContain('data-jobs="running"')
    expect(html).toContain('vv-jobsbtn vv-jobsbtn--running')
    // Vòng: nền mờ + cung đỏ, offset theo tiến độ thật của các việc đang chạy.
    expect(html).toContain('vv-jobsbtn__ring')
    expect(html).toContain(`stroke-dashoffset="${partway}"`)
    expect(html).toContain('>2</span>')
    // Không còn viên chữ “2 · 12” hay “Rảnh”.
    expect(html).not.toContain(t.shell.jobsCompact(2, 12))
    expect(html).not.toContain('Rảnh')
  })
  it('máy rảnh: chỉ icon activity xám, không chữ', () => {
    const html = render({ ...base, running: 0, queued: 0 })
    expect(html).toContain('aria-label="Việc nền: máy rảnh"')
    expect(html).toContain('data-jobs="idle"')
    expect(html).not.toContain('vv-jobsbtn__ring')
    expect(html).toContain('activity')
    expect(html).not.toContain('Rảnh')
  })
  it('tạm dừng: vòng xám đầy + icon pause, câu aria có giờ dừng', () => {
    const html = render({ ...base, paused: true })
    expect(html).toContain('aria-label="Việc nền: đã tạm dừng lúc 00:04, 12 chờ lượt"')
    expect(html).toContain('data-jobs="paused"')
    expect(html).toContain('vv-jobsbtn vv-jobsbtn--paused')
    expect(html).toContain('stroke-dashoffset="0"')
    expect(html).toContain('pause')
    expect(html).not.toContain('>12</span>')
  })
  it('mất kết nối: icon danger cam và không chấm dù có thông báo chưa đọc', () => {
    const html = render(base, { online: false, unread: 3 })
    expect(html).toContain('aria-label="Việc nền: mất kết nối máy chủ, đang thử lại"')
    expect(html).toContain('data-jobs="offline"')
    expect(html).toContain('vv-jobsbtn vv-jobsbtn--offline')
    expect(html).toContain('danger')
    expect(html).not.toMatch(dot)
    expect(html).not.toContain('2 đang chạy')
  })
  it('chấm vàng: có số thì câu aria ghi số, không số thì “có thông báo cần bạn”', () => {
    const counted = render(base, { unread: 3 })
    expect(counted).toMatch(dot)
    expect(counted).toContain('aria-label="Việc nền: 2 đang chạy, 12 chờ lượt, 3 thông báo cần bạn"')
    const hinted = render(base, { unread: true })
    expect(hinted).toMatch(dot)
    expect(hinted).toContain('aria-label="Việc nền: 2 đang chạy, 12 chờ lượt, có thông báo cần bạn"')
    expect(render(base)).not.toMatch(dot)
    // Số 0 không phải “chưa đọc”.
    expect(render(base, { unread: 0 })).not.toMatch(dot)
  })
  it('bấm mở sheet Việc nền và giữ đúng trạng thái mở cho aria-expanded', () => {
    const html = render(base, { onOpen: () => {}, expanded: true })
    expect(html).toContain('aria-haspopup="dialog"')
    expect(html).toContain('aria-expanded="true"')
    const queuedOnly = render({ ...base, running: 0 })
    expect(queuedOnly).toContain('data-jobs="running"')
  })
})

describe('JobsPill máy tính giữ viên chữ 30px', () => {
  it('câu đầy đủ và số thật', () => {
    expect(desktop()).toContain('2 đang chạy · 12 chờ lượt')
    expect(desktop()).toContain('Máy đang chạy 2/2 luồng · 12 video chờ lượt')
    expect(desktop({ ...base, running: 0, queued: 0 })).toContain('>Máy rảnh</span>')
    expect(desktop({ ...base, paused: true })).toContain('Đã tạm dừng lúc 00:04 · 12 chờ lượt')
  })
  it('rảnh và mất kết nối không vẽ chấm nhấp', () => {
    expect(desktop({ ...base, paused: true })).not.toContain('vv-jobs__dot')
    expect(desktop(base, false)).not.toContain('vv-jobs__dot')
    expect(desktop(base, false)).toContain('Mất kết nối · đang thử lại')
  })
})
