// @vitest-environment happy-dom
import { act, StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it } from 'vitest'
import { QR_MIN_CODE_PX, QR_MIN_MODULE_PX, QR_QUIET, QR_RADIUS, QR_RANGE, QR_ZOOM_PX, QrCode, QrZoom, qrMatrix, qrMetrics, qrPath } from './QrCode'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { document.body.replaceChildren() })

// Địa chỉ ghép thật: http://<ip>:8686/p/<32 hex> — dài 59 byte, mức sửa lỗi M ⇒ phiên bản 4 (33 ô).
const PAIR_URL = `http://192.168.1.15:8686/p/${'0123456789abcdef'.repeat(2)}`
const VERSIONS = [21, 25, 29, 33, 37, 41, 45]

describe('toán cỡ mã QR', () => {
  it('cỡ ô là số nguyên, lề đúng 4 ô, ≥ 7 px/ô, phần mã ≥ 200 px, mọi phiên bản', () => {
    for (const range of Object.values(QR_RANGE)) {
      for (const modules of VERSIONS) {
        const m = qrMetrics(modules, range)
        expect(Number.isInteger(m.k)).toBe(true)
        expect(m.total).toBe(modules + 2 * QR_QUIET)
        expect(m.size).toBe((modules + 8) * m.k)
        expect(m.k).toBeGreaterThanOrEqual(QR_MIN_MODULE_PX)
        expect(m.codePx).toBeGreaterThanOrEqual(QR_MIN_CODE_PX)
        expect(m.size).toBeGreaterThanOrEqual(range.min)
      }
    }
  })

  it('ví dụ trong kit: 25 ô → 297 px (k=9), 29 ô → 296 px (k=8), 33 ô → 287 px (k=7)', () => {
    expect(qrMetrics(25, QR_RANGE.dialog)).toMatchObject({ k: 9, size: 297 })
    expect(qrMetrics(29, QR_RANGE.dialog)).toMatchObject({ k: 8, size: 296 })
    expect(qrMetrics(33, QR_RANGE.dialog)).toMatchObject({ k: 7, size: 287 })
  })

  it('hộp thoại 264–300 px, thẻ thiết lập ≥ 232 px với mã ≤ 33 ô', () => {
    for (const modules of [21, 25, 29, 33]) {
      const dialog = qrMetrics(modules, QR_RANGE.dialog).size
      expect(dialog).toBeGreaterThanOrEqual(264)
      expect(dialog).toBeLessThanOrEqual(300)
      expect(qrMetrics(modules, QR_RANGE.card).size).toBeGreaterThanOrEqual(232)
    }
  })

  it('mã nhiều ô hơn giữ 7 px/ô thay vì thu nhỏ (không bao giờ < 200 px)', () => {
    expect(qrMetrics(45, QR_RANGE.dialog)).toMatchObject({ k: 7, size: 371 })
  })
})

describe('QrCode', () => {
  it('liên kết ghép ngắn ra mã ≤ 33 ô', () => {
    expect(qrMatrix(PAIR_URL).count).toBeLessThanOrEqual(33)
  })

  it('SVG đúng cỡ nguyên, lề 4 ô nằm trong SVG, không padding/opacity/filter trên ô trắng', () => {
    const matrix = qrMatrix(PAIR_URL)
    const { k, size, total } = qrMetrics(matrix.count, QR_RANGE.card)
    const html = renderToStaticMarkup(<QrCode value={PAIR_URL} variant="card" />)
    expect(html).toContain(`width="${size}" height="${size}" viewBox="0 0 ${total} ${total}"`)
    expect(html).toContain(`data-qr-k="${k}"`)
    expect(html).toContain('shape-rendering="crispEdges"')
    expect(html).toContain('fill="#FFFFFF"')
    expect(html).toContain(`border-radius:${QR_RADIUS.card}px`)
    expect(html).not.toMatch(/padding|opacity|filter/)
    // Bo góc ô trắng theo artboard: thẻ 16, hộp thoại 18, lớp phóng to 24.
    expect(QR_RADIUS).toEqual({ card: 16, dialog: 18, zoom: 24 })
    // Ô đen đầu tiên (góc định vị trái trên) cách mép đúng 4 ô.
    expect(qrPath(matrix).startsWith(`M${QR_QUIET} ${QR_QUIET}h`)).toBe(true)
  })

  it('hết hạn: lớp trắng 70% + “Mã đã hết hạn” + Tạo mã mới', () => {
    const html = renderToStaticMarkup(<QrCode value={PAIR_URL} expired renew={{ onClick: () => {} }} />)
    expect(html).toContain('rgba(255,255,255,0.7)')
    expect(html).toContain('Expired')
    expect(html).toContain('Renew')
  })
})

describe('QrZoom — lớp phủ phóng to', () => {
  it('hộp trắng 464 px, bo 24, mã nguyên px, câu dẫn đúng, Esc/click đóng', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const root = createRoot(host)
    let closes = 0
    await act(async () => { root.render(<StrictMode><QrZoom value={PAIR_URL} onClose={() => { closes += 1 }} /></StrictMode>) })
    const layer = document.body.querySelector('[role="dialog"]')
    const box = layer?.querySelector('[data-qr-size]')
    const svg = box?.querySelector('svg')
    expect(layer?.getAttribute('aria-label')).toBe('QR code')
    expect(layer?.textContent).toContain('Scan with your phone to continue.')
    expect(Number(box?.getAttribute('data-qr-size'))).toBe(QR_ZOOM_PX)
    const svgSize = Number(svg?.getAttribute('width'))
    const total = qrMatrix(PAIR_URL).count + 2 * QR_QUIET
    expect(svgSize % total).toBe(0) // cỡ ô nguyên px: k = 11 → 451 px, canh giữa hộp 464
    expect(svgSize).toBeLessThanOrEqual(QR_ZOOM_PX)
    const style = box?.getAttribute('style') ?? ''
    expect(style).toMatch(new RegExp(`border-radius:\\s*${QR_RADIUS.zoom}px`))
    expect(style).not.toMatch(/padding/)
    // Esc đóng
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    expect(closes).toBe(1)
    // bấm vào lớp phủ đóng (gắn lại QrZoom vì Esc đã gọi onClose)
    await act(async () => { root.render(<QrZoom value={PAIR_URL} onClose={() => { closes += 1 }} />) })
    await act(async () => { document.body.querySelector('[role="dialog"]')?.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    expect(closes).toBe(2)
    await act(async () => { root.unmount() })
    host.remove()
  })
})
