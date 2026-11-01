// Kiểu khung đầu trang dùng chung — thay ba lớp cuối cùng của
// styles/components.css còn được hai nơi trở lên đọc: nhãn breadcrumb của
// AppHeader/ExploreCrumb, và menu chọn kênh của ChannelSwitcher mà thanh chọn
// nhiều ở Khám phá (BulkBar) dùng lại. Kiểu riêng của từng component nằm
// trong chính tệp component đó.
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'

export const shellChrome = stylex.create({
  /** “Cài đặt · máy này” / “Khám phá / <đối thủ>” ở vùng trái thanh đầu. */
  crumb: { fontSize: 13, color: color['--color-fg-3'], whiteSpace: 'nowrap' },

  /** Popover 312px dưới bộ đổi kênh (absolute theo .vv-switcher-root). */
  menu: {
    position: 'absolute',
    left: 0,
    top: 'var(--vv-s-38)',
    zIndex: 40,
    width: 312,
    borderRadius: 'var(--vv-r-14)',
    backgroundColor: color['--color-surface-2'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    boxShadow: 'var(--vv-shadow-pop)',
    padding: 'var(--vv-s-6)',
    display: 'flex',
    flexDirection: 'column',
    gap: 'var(--vv-s-2)',
  },

  /** Một dòng trong menu chọn kênh (menuitemradio ở bộ đổi kênh, menuitem ở BulkBar). */
  item: {
    display: 'flex',
    alignItems: 'center',
    gap: 'var(--vv-s-10)',
    padding: 'var(--vv-s-8) var(--vv-s-10)',
    borderRadius: radius['--radius-button'],
    borderWidth: 0,
    borderStyle: 'none',
    borderColor: 'transparent',
    backgroundColor: { default: 'transparent', ':hover': color['--color-surface-1'] },
    color: color['--color-fg'],
    textAlign: 'left',
    width: '100%',
    fontFamily: 'inherit',
    fontSize: 'inherit',
    fontWeight: 'inherit',
    lineHeight: 'inherit',
    fontStyle: 'inherit',
    fontVariant: 'inherit',
    fontStretch: 'inherit',
    cursor: 'pointer',
  },
})
