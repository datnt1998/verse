// MobilePage — khung một màn hình điện thoại: thanh tiêu đề 52, MỘT vùng cuộn,
// chân trang (nếu có) nằm trên thanh tab. Số đo theo artboard Viderse MKit
// (`apps/viderse/docs/design/v1/artboards/MKit.dc.html`, khối "Thanh").
//
// Tên lớp `m-page/m-topbar/m-title/m-scroll/m-footer` là móc ổn định: Noverse còn
// CSS riêng cho chúng (mobile.css) nên màn Noverse giữ nguyên từng điểm ảnh.
import { useRef, type ReactNode, type RefObject } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius } from '../tokens.stylex'
import { Icon } from '../Icon'
import type { IconName } from '../iconNames'
import { getKitLabels } from '../labels'
import { useMobileRuntime, useMobileScroll } from './runtime'

const styles = stylex.create({
  page: {
    display: 'flex',
    flexDirection: 'column',
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minHeight: 0,
    overflow: 'hidden',
  },
  topbar: {
    flex: '0 0 auto',
    display: 'flex',
    alignItems: 'center',
    gap: 2,
    height: 52,
    minHeight: 52,
    paddingInline: 6,
  },
  iconButton: {
    flex: '0 0 auto',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 2,
    height: 44,
    minHeight: 44,
    paddingInline: 0,
    borderWidth: 0,
    borderRadius: radius['--radius-tile'],
    backgroundColor: 'transparent',
    color: color['--color-fg-2'],
    cursor: 'pointer',
    WebkitTapHighlightColor: 'transparent',
    ':active': { backgroundColor: kit['--kit-wash-6'] },
    ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: -2 },
  },
  iconOnly: { width: 44, minWidth: 44, paddingInline: 0 },
  backLabel: { fontSize: 15, color: color['--color-accent'] },
  titleWrap: {
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minWidth: 0,
    paddingInlineStart: 2,
    fontSize: 16,
    fontWeight: 600,
  },
  /* Chỉ xếp dọc khi có dòng phụ: Noverse không truyền dòng phụ nên `.m-title`
     của Noverse vẫn là khối như trước (CSS app đè lên lớp này). */
  titleStack: { display: 'flex', flexDirection: 'column', justifyContent: 'center', lineHeight: 1.3 },
  titleText: { minWidth: 0, overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' },
  subtitle: {
    minWidth: 0,
    overflow: 'hidden',
    whiteSpace: 'nowrap',
    textOverflow: 'ellipsis',
    fontSize: 12,
    fontWeight: 400,
    color: kit['--kit-fg-sub'],
  },
  scroll: {
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minHeight: 0,
    overflowY: 'auto',
    overflowX: 'hidden',
    overscrollBehaviorY: 'contain',
    scrollbarWidth: 'none',
    WebkitOverflowScrolling: 'touch',
  },
  /* Đệm cuối vùng cuộn: thanh tab còn chỗ thở (`--m-scroll-pad`); có chân trang
     thì chân trang đã giữ chỗ nên bỏ đệm. */
  scrollPad: { paddingBottom: 'var(--m-scroll-pad)' },
  scrollTight: { paddingBottom: 0 },
  footer: {
    flex: '0 0 auto',
    paddingInline: 16,
    paddingTop: 16,
    backgroundImage: 'linear-gradient(180deg, transparent, var(--color-bg) 26%)',
  },
  /* Chân trang đặt trên đường `--m-float-bottom` (thanh tab hiện), hoặc sát đáy an
     toàn khi thanh tab ẩn; bàn phím ảo mở thì nâng thêm `--kb`. */
  footFloat: { paddingBottom: 'var(--m-float-bottom)' },
  footHide: { paddingBottom: 'calc(max(26px, var(--m-safe-bottom)) + var(--kb, 0px))' },
})

export type MobilePageProps = {
  /** tiêu đề thanh trên; chuỗi thì một dòng, nút/đoạn khác thì render nguyên */
  title: ReactNode
  /** dòng phụ 12px dưới tiêu đề (artboard MKit: `vv-navbar__title span`) */
  subtitle?: ReactNode
  /** nhãn nút quay lại; có nhãn thì nút rộng theo nhãn (kiểu Noverse) */
  backTitle?: string
  onBack?: () => void
  /** nút đóng thay cho nút quay lại */
  onClose?: () => void
  /** thay toàn bộ nút đầu (ví dụ logo) */
  leading?: ReactNode
  /** chèn sau tiêu đề, trước `actions` (Noverse: StoragePill) */
  headerExtra?: ReactNode
  /** nút bên phải (44×44 icon) */
  actions?: ReactNode
  /** dải dưới thanh trên (Noverse: StorageBanner) */
  banner?: ReactNode
  /** chân trang: dính trên thanh tab, nâng theo bàn phím ảo */
  footer?: ReactNode
  /** tắt theo dõi cuộn (màn tự quản lý) */
  manageScroll?: boolean
  scrollRef?: RefObject<HTMLElement | null>
  xstyle?: stylex.StyleXStyles
  footStyle?: stylex.StyleXStyles
  children: ReactNode
}

export function MobilePage({
  title, subtitle, backTitle, onBack, onClose, leading, headerExtra, actions, banner,
  footer, manageScroll = true, scrollRef, xstyle, footStyle, children,
}: MobilePageProps) {
  const ownRef = useRef<HTMLElement>(null)
  const ref = scrollRef ?? ownRef
  const { tabMode } = useMobileRuntime()
  useMobileScroll(ref, { disabled: !manageScroll })
  const t = getKitLabels()
  const footSx = stylex.props(styles.footer, tabMode === 'hide' ? styles.footHide : styles.footFloat, footStyle)
  const barSx = stylex.props(styles.topbar)
  const titleSx = stylex.props(styles.titleWrap, subtitle != null && styles.titleStack)
  const scrollSx = stylex.props(styles.scroll, footer != null ? styles.scrollTight : styles.scrollPad)
  const labelled = backTitle != null
  return (
    <section className={`m-page ${stylex.props(styles.page, xstyle).className ?? ''}`}>
      <header className={`m-topbar ${barSx.className ?? ''}`} style={barSx.style} role="banner">
        {leading ?? (onBack || onClose
          ? <button
              type="button"
              className={`m-back ${stylex.props(styles.iconButton, !labelled && styles.iconOnly).className ?? ''}`}
              aria-label={backTitle ?? t.mobile.back}
              onClick={onBack ?? onClose}
            >
              {labelled
                ? <><Icon name="arrow-left-2" size={22} /><span {...stylex.props(styles.backLabel)}>{backTitle}</span></>
                : <Icon name="arrow-r" size={20} style={{ transform: 'rotate(180deg)' }} />}
            </button>
          : null)}
        <div className={`m-title ${titleSx.className ?? ''}`} style={titleSx.style}>
          <span {...stylex.props(styles.titleText)}>{title}</span>
          {subtitle != null && <small {...stylex.props(styles.subtitle)}>{subtitle}</small>}
        </div>
        {headerExtra}
        {actions}
      </header>
      {banner}
      <main
        ref={ref as RefObject<HTMLElement>}
        className={`m-scroll ${scrollSx.className ?? ''}`}
        style={scrollSx.style}
      >
        {children}
      </main>
      {footer != null && (
        <div className={`m-footer ${footSx.className ?? ''}`} style={footSx.style}>{footer}</div>
      )}
    </section>
  )
}

/** Nút icon 44×44 của thanh trên (artboard `.vv-ib`) — dùng cho `actions`. */
export function MobilePageAction({ icon, label, onClick, disabled }: {
  icon: IconName
  label: string
  onClick?: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      className={stylex.props(styles.iconButton, styles.iconOnly).className}
      style={stylex.props(styles.iconButton, styles.iconOnly).style}
      aria-label={label}
      title={label}
      onClick={onClick}
      disabled={disabled}
    >
      <Icon name={icon} size={20} />
    </button>
  )
}
