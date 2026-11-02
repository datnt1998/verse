// Dialog — nền surface-3, bo 18, shadow --dialog-shadow, backdrop --kit-scrim.
// Focus trap đơn giản (focus ô đầu, Esc đóng, Tab vòng quanh trong dialog).
// Chuyển động theo Kit.dc.html: scrim mờ dần 200 ms, hộp nảy vào 300 ms; đóng: hộp 180 ms, scrim 200 ms.
import { useEffect, useId, useRef, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit, radius, surface } from './tokens.stylex'
import { Icon } from './Icon'
import { pushEscapeLayer } from './Overlays'
import './kit.css'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const sx = stylex.create({
  overlay: { position: 'fixed', inset: 0, zIndex: 40 },
  scrim: {
    position: 'absolute', inset: 0, backgroundColor: kit['--kit-scrim'],
    animationName: { default: 'kit-fade', [NO_MOTION]: 'none' },
    animationDuration: '.2s', animationTimingFunction: 'ease-out', animationFillMode: 'both',
  },
  scrimStrong: { backgroundColor: kit['--kit-scrim-strong'] },
  center: {
    position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
    // Tai thỏ / home indicator: hộp không chui xuống dưới vùng an toàn (desktop = 16 px như cũ).
    paddingTop: 'max(16px, var(--m-safe-top))',
    paddingBottom: 'max(16px, var(--m-safe-bottom))',
    paddingLeft: 16, paddingRight: 16,
  },
  dlg: {
    animationName: { default: 'kit-dialog', [NO_MOTION]: 'none' },
    animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.25,1.08)', animationFillMode: 'both',
  },
  head: (padX: string) => ({
    display: 'flex', alignItems: 'center', gap: 10, paddingTop: 16, paddingBottom: 16,
    paddingInline: padX, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-2'],
  }),
  // Thân hộp: cùng lề ngang với đầu/chân. Trước đây thân
  // không có đệm nên câu mô tả dính mép; hộp cần tràn mép (danh sách) truyền
  // `padBody={false}` để thân không bọc đệm — giữ nguyên bố cục cũ.
  body: (padX: string) => ({
    paddingTop: 14, paddingBottom: 16, paddingInline: padX, minWidth: 0,
  }),
  title: { margin: 0, flex: 1, fontSize: 16, fontWeight: 600 },
  meta: { fontSize: 12, color: color['--color-fg-4'] },
  close: {
    width: 32, height: 32, borderRadius: 8, borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent',
    color: color['--color-fg-3'], display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  foot: (padX: string) => ({
    display: 'flex', justifyContent: 'flex-end', gap: 8, paddingTop: 12, paddingBottom: 16,
    paddingInline: padX, borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-2'],
  }),
  // Mặt hộp theo Kit/04-components: surface-3, viền line-4, bo dock, shadow dialog.
  // Caller cũ chỉ có class .dlg (Noverse tự vẽ) — kit vẽ sẵn để app khác không phải
  // khai lại; .dlg của Noverse (không layer) trùng giá trị nên không đổi hình.
  box: {
    display: 'flex', flexDirection: 'column',
    backgroundColor: color['--color-surface-3'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    borderRadius: radius['--radius-dock'],
    boxShadow: surface['--dialog-shadow'],
    // Dialogs.dc.html › .ov .dlg: max-height calc(100vh - 48px), overflow auto.
    // 100dvh + vùng an toàn để hộp cao (bảng phím tắt) cuộn bên trong thay vì tràn
    // khung nhìn/cắt chân; hộp ngắn giữ nguyên chiều cao thật.
    maxHeight: 'calc(100dvh - max(16px, var(--m-safe-top)) - max(16px, var(--m-safe-bottom)) - 16px)',
    overflow: 'auto',
  },
})
// Ngăn xếp dialog đang mở (thứ tự mount) — Esc chỉ đóng cái trên cùng.
const openDialogs: HTMLElement[] = []
// Phần nền bị inert khi có dialog mở (chỉ gỡ khi dialog cuối cùng đóng).
let inertRoot: HTMLElement | null = null
/** Thời gian lớp đóng còn hiển thị: scrim mờ đi 200 ms (hộp 180 ms). */
export const DIALOG_EXIT_MS = 200

/**
 * Dialog do caller unmount ngay khi đóng (Esc, nút Huỷ, bỏ ?dlg=…), nên lớp phủ React vừa gỡ
 * được gắn lại tạm thời chỉ để chạy chuyển động đóng: không nhận focus/chuột, không vào cây a11y.
 * Node còn trong DOM (StrictMode chạy lại effect) hoặc giảm chuyển động → không làm gì.
 */
export function playDialogExit(overlay: HTMLElement) {
  if (overlay.isConnected || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  overlay.classList.add('kit-dlg-closing')
  overlay.setAttribute('aria-hidden', 'true')
  overlay.setAttribute('inert', '')
  overlay.style.pointerEvents = 'none'
  // Bản đang đóng không còn là dialog: bỏ id/role/aria-modal để truy vấn "đang có modal" (Esc của dock…) không thấy nó.
  overlay.querySelectorAll('[id], [role], [aria-modal]').forEach((element) => {
    element.removeAttribute('id'); element.removeAttribute('role'); element.removeAttribute('aria-modal')
  })
  document.body.append(overlay)
  window.setTimeout(() => overlay.remove(), DIALOG_EXIT_MS)
}

export function Dialog({ title, label, icon, meta, onClose, footer, children, width = 560, alertDialog, style, padX = 18, padBody = true, scrim = 'default', scrimStyle: scrimOverride }: {
  title?: ReactNode; label?: string; icon?: ReactNode
  /** Lệnh mono cạnh tiêu đề, vd "/reopen" (Dialogs.dc.html) */
  meta?: ReactNode
  onClose?: () => void
  footer?: ReactNode; children: ReactNode
  width?: number | string; alertDialog?: boolean; style?: CSSProperties
  /** Lề ngang của đầu, thân và chân hộp (mặc định 18 px) */
  padX?: number
  /**
   * Đệm thân hộp theo `padX` (mặc định, khớp đầu/chân). `false` cho thân tràn
   * mép — danh sách dài, lưới thẻ, hoặc hộp đã tự đệm bên trong.
   */
  padBody?: boolean
  /** 'default' = --kit-scrim; 'strong' = --kit-scrim-strong (Onboard.dc.html › lib=browse). Animation không đổi. */
  scrim?: 'default' | 'strong'
  scrimStyle?: stylex.StyleXStyles
}) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const ref = useRef<HTMLDivElement>(null)
  const returnFocus = useRef<Element | null>(typeof document === 'undefined' ? null : document.activeElement)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const titleId = useId()

  useEffect(() => {
    const overlay = overlayRef.current
    return () => { if (overlay) playDialogExit(overlay) }
  }, [])

  useEffect(() => {
    const node = ref.current
    if (!node) return
    const focusables = () => Array.from(node.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
    )).filter((el) => !el.hasAttribute('disabled'))
    // Không cướp focus của input autoFocus (Dialog mở từ form)
    if (!node.contains(document.activeElement)) focusables()[0]?.focus()
    openDialogs.push(node)
    // Nền không tương tác/không vào cây a11y khi dialog mở (dialog portal ra body nên #root an toàn).
    if (openDialogs.length === 1) {
      const root = document.getElementById('root')
      if (root && !root.contains(node)) {
        inertRoot = root
        root.setAttribute('inert', '')
        root.setAttribute('aria-hidden', 'true')
      }
    }

    // Listener ở document: Esc vẫn đóng được khi focus rơi ra body (vd dialog con vừa đóng).
    // Esc do lớp trên cùng xử lý (Overlays.useEscapeLayer): chỉ dialog trên cùng đóng, menu/popover
    // đang mở bên trong đóng trước. Ở đây chỉ còn Tab (focus trap).
    const onKey = (event: KeyboardEvent) => {
      if (openDialogs[openDialogs.length - 1] !== node) return
      if (event.key !== 'Tab') return
      const items = focusables()
      if (items.length === 0) return
      const first = items[0]; const last = items[items.length - 1]
      const active = document.activeElement
      const outside = !node.contains(active)
      if (event.shiftKey && (active === first || outside)) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && (active === last || outside)) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', onKey, true)
    // Esc: đăng ký làm một lớp; chỉ lớp trên cùng nhận (menu/popover bên trong đăng ký sau → nằm trên).
    const popEscape = pushEscapeLayer(() => onCloseRef.current?.())
    return () => {
      popEscape()
      document.removeEventListener('keydown', onKey, true)
      const at = openDialogs.indexOf(node)
      if (at >= 0) openDialogs.splice(at, 1)
      if (openDialogs.length === 0 && inertRoot) {
        inertRoot.removeAttribute('inert')
        inertRoot.removeAttribute('aria-hidden')
        inertRoot = null
      }
      // StrictMode replays effects while the dialog is still mounted.
      if (node.isConnected && !node.closest('.kit-dlg-closing')) return
      const parent = openDialogs[openDialogs.length - 1]
      const trigger = returnFocus.current
      if (trigger instanceof HTMLElement && trigger.isConnected && !trigger.hasAttribute('disabled') && !trigger.closest('[hidden], [inert], [aria-hidden="true"]') && (!parent || parent.contains(trigger))) {
        trigger.focus({ preventScroll: true })
      } else if (parent && !parent.contains(document.activeElement)) {
        parent.querySelector<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')?.focus()
      }
    }
  }, [])

  const scrimStyle = stylex.props(sx.scrim, scrim === 'strong' && sx.scrimStrong, scrimOverride)
  const dlg = stylex.props(sx.dlg, sx.box)
  const head = stylex.props(sx.head(`${padX}px`))
  const body = stylex.props(sx.body(`${padX}px`))
  const foot = stylex.props(sx.foot(`${padX}px`))
  return createPortal(
    <div ref={overlayRef} {...stylex.props(sx.overlay)}>
      <div aria-hidden="true" className={`kit-dlg-scrim ${scrimStyle.className}`} onClick={onClose} style={scrimStyle.style} />
      <div {...stylex.props(sx.center)}>
        <div ref={ref} className={['dlg', 'kit-dlg', dlg.className].join(' ')} role={alertDialog ? 'alertdialog' : 'dialog'} aria-modal="true" aria-label={title === undefined ? label : undefined} aria-labelledby={title !== undefined ? titleId : undefined}
          style={{ ...dlg.style, width: `min(${typeof width === 'number' ? `${width}px` : width}, 100%)`, ...style }}>
          {title !== undefined && (
            <div className={head.className} style={head.style}>
              {icon}
              <h2 id={titleId} {...stylex.props(sx.title)}>{title}</h2>
              {meta !== undefined && (
                <span className={`mono ${stylex.props(sx.meta).className}`}>{meta}</span>
              )}
              {onClose && (
                <button type="button" className={`hov-ghost ${stylex.props(sx.close).className}`} onClick={onClose} aria-label={getKitLabels().dialog.close}>
                  <Icon name="close-circle-bulk" size={18} />
                </button>
              )}
            </div>
          )}
          {padBody ? <div className={`kit-dlg-body ${body.className}`} style={body.style}>{children}</div> : children}
          {footer !== undefined && (
            <div className={foot.className} style={foot.style}>
              {footer}
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}
