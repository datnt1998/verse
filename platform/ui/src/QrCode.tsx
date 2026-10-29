// QrCode (17-kit.md › QrCode): mã QR đăng nhập điện thoại.
// Lề trắng đúng 4 ô nằm TRONG SVG (không padding CSS), cỡ ô là số nguyên px, mức sửa lỗi M,
// đen #000 trên trắng #FFF, không đảo màu ở giao diện tối. Nút phóng to / làm mới nằm NGOÀI mã.
import { useMemo, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import qrcode from 'qrcode-generator'
import { getKitLabels } from './labels'
import { Button } from './Button'
import { Icon } from './Icon'
import { useEscapeLayer } from './Overlays'
import './kit.css'

/** Lề trắng quanh mã, tính bằng ô (chuẩn QR: 4). */
export const QR_QUIET = 4
/** Cỡ ô tối thiểu (px): mã luôn đủ lớn để camera điện thoại đọc được ở khoảng cách bình thường. */
export const QR_MIN_MODULE_PX = 7
/** Phần mã (không tính lề) không bao giờ nhỏ hơn cỡ này (px). */
export const QR_MIN_CODE_PX = 200
/** Bo góc ô trắng theo artboard (px): thẻ Thiết lập 16 (Onboard), hộp thoại 18 (Login), lớp phóng to 24. */
export const QR_RADIUS: Record<QrVariant | 'zoom', number> = { card: 16, dialog: 18, zoom: 24 }
/** Cỡ lớp phủ phóng to (px). */
export const QR_ZOOM_PX = 464

export type QrVariant = 'card' | 'dialog'
/** Khoảng cỡ hiển thị mong muốn của từng nơi dùng (px, cả lề). */
export const QR_RANGE: Record<QrVariant, { min: number; max: number }> = {
  card: { min: 232, max: 300 },
  dialog: { min: 264, max: 300 },
}

export type QrMetrics = { modules: number; k: number; total: number; size: number; codePx: number }

/**
 * Cỡ vẽ: `size = (số ô + 2×lề) × k` với k nguyên.
 * k lớn nhất để size ≤ max, nhưng không dưới 7 px/ô, không để phần mã < 200 px và không để size < min.
 * Mã quá nhiều ô (phiên bản cao) thì giữ k tối thiểu và chấp nhận vượt max — không bao giờ làm ô nhỏ đi.
 */
export function qrMetrics(modules: number, range: { min: number; max: number }): QrMetrics {
  const total = modules + 2 * QR_QUIET
  const floor = Math.max(QR_MIN_MODULE_PX, Math.ceil(QR_MIN_CODE_PX / modules), Math.ceil(range.min / total))
  const k = Math.max(floor, Math.floor(range.max / total))
  return { modules, k, total, size: total * k, codePx: modules * k }
}

export type QrMatrix = { count: number; dark: (row: number, col: number) => boolean }

export function qrMatrix(text: string): QrMatrix {
  const code = qrcode(0, 'M')
  code.addData(text, 'Byte')
  code.make()
  return { count: code.getModuleCount(), dark: (row, col) => code.isDark(row, col) }
}

/** Path SVG theo ô (đơn vị = 1 ô, đã cộng lề); gộp các ô đen liền nhau trên một hàng. */
export function qrPath(matrix: QrMatrix): string {
  let d = ''
  for (let row = 0; row < matrix.count; row += 1) {
    let col = 0
    while (col < matrix.count) {
      if (!matrix.dark(row, col)) { col += 1; continue }
      const start = col
      while (col < matrix.count && matrix.dark(row, col)) col += 1
      d += `M${start + QR_QUIET} ${row + QR_QUIET}h${col - start}v1h-${col - start}z`
    }
  }
  return d
}

function QrSvg({ matrix, k }: { matrix: QrMatrix; k: number }) {
  const total = matrix.count + 2 * QR_QUIET
  const path = useMemo(() => qrPath(matrix), [matrix])
  const size = total * k
  return (
    <svg width={size} height={size} viewBox={`0 0 ${total} ${total}`} shapeRendering="crispEdges" aria-hidden="true" style={{ display: 'block', shapeRendering: 'crispEdges' }}>
      <rect width={total} height={total} fill="#FFFFFF" />
      <path d={path} fill="#000000" />
    </svg>
  )
}

export type QrRenew = { onClick: () => void; pending?: boolean; success?: boolean; disabled?: boolean }

export function QrCode({ value, variant = 'card', expired = false, renew }: {
  value: string
  variant?: QrVariant
  expired?: boolean
  /** Nút “Tạo mã mới” trong lớp “Mã đã hết hạn”. */
  renew?: QrRenew
}) {
  const matrix = useMemo(() => qrMatrix(value), [value])
  const { k, size } = qrMetrics(matrix.count, QR_RANGE[variant])
  return (
    <div role="img" aria-label={getKitLabels().qr.codeLabel} data-qr-size={size} data-qr-k={k} style={{
      position: 'relative', width: size, height: size, flex: '0 0 auto', borderRadius: QR_RADIUS[variant], background: '#FFFFFF', overflow: 'hidden',
      boxShadow: '0 0 0 1px rgba(255,255,255,0.12), 0 22px 50px -22px rgba(0,0,0,0.9)',
    }}>
      <div style={expired ? { opacity: 0.25, filter: 'blur(3px)' } : undefined}><QrSvg matrix={matrix} k={k} /></div>
      {expired && (
        <div role="status" style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, background: 'rgba(255,255,255,0.7)' }}>
          <span style={{ fontSize: 14, fontWeight: 600, color: '#0B0B0D' }}>{getKitLabels().qr.expired}</span>
          {renew && (
            <Button variant="primary" disabled={renew.disabled} pending={renew.pending} success={renew.success} onClick={renew.onClick} style={{ height: 34, padding: '0 14px', borderRadius: 9, background: '#0B0B0D', color: '#FFFFFF', fontSize: 13, fontWeight: 600 }}>
              <Icon name="refresh-2" size={15} />{getKitLabels().qr.renew}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

/** Nút phóng to đặt cạnh viên đếm ngược (ngoài mã). */
export function QrZoomButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" className="hov-ghost" onClick={onClick} aria-label={getKitLabels().qr.zoomLabel} title={getKitLabels().qr.zoom} style={{
      width: 28, height: 28, borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#C9C9D1',
      display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
    }}><Icon name="scan" size={15} /></button>
  )
}

/** Viên đếm ngược xanh `04:37` (đỏ khi hết hạn). */
export function QrCountdown({ text, expired = false }: { text: string; expired?: boolean }) {
  return (
    <span className="num" style={{
      height: 24, padding: '0 9px', borderRadius: 999, display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, flex: '0 0 auto',
      background: expired ? 'rgba(255,154,162,0.1)' : 'rgba(166,237,193,0.1)', color: expired ? '#FF9AA2' : '#A6EDC1',
    }}><Icon name="timer-1" size={13} />{text}</span>
  )
}

/** Lớp phủ toàn màn: mã 464 px; Esc hoặc bấm bất kỳ đâu để đóng. */
export function QrZoom({ value, onClose }: { value: string; onClose: () => void }) {
  const matrix = useMemo(() => qrMatrix(value), [value])
  const total = matrix.count + 2 * QR_QUIET
  // 464 px trên màn rộng; màn nhỏ (điện thoại) thu vừa khung nhưng vẫn theo ô nguyên px.
  const side = Math.max(total, Math.min(QR_ZOOM_PX, window.innerWidth - 32, window.innerHeight - 96))
  const k = Math.max(1, Math.floor(side / total))
  // Lớp phủ toàn màn là một lớp: Esc đóng nó trước mọi hộp thoại bên dưới (17-kit, chốt 25/10).
  useEscapeLayer(true, onClose)
  const layer: ReactNode = (
    <div role="dialog" aria-modal="true" aria-label={getKitLabels().qr.zoomTitle} onClick={onClose} style={{
      position: 'fixed', inset: 0, zIndex: 80, background: 'rgba(4,4,6,0.86)', display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center', gap: 18, padding: 16, animation: 'kit-fade .25s ease-out both',
    }}>
      <div role="img" aria-label={getKitLabels().qr.codeLabel} data-qr-size={side} data-qr-k={k} style={{
        width: side, height: side, borderRadius: QR_RADIUS.zoom, background: '#FFFFFF', overflow: 'hidden',
        display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 30px 80px -20px rgba(0,0,0,0.9)',
      }}><QrSvg matrix={matrix} k={k} /></div>
      <span style={{ fontSize: 14, color: '#C9C9D1', textAlign: 'center' }}>{getKitLabels().qr.zoomHint}</span>
    </div>
  )
  return createPortal(layer, document.body)
}
