// Kiểu của khung pane điện thoại: mỗi pane là một vùng cuộn riêng nên vị trí
// cuộn của tab này không dùng chung với tab khác; màn đẩy nằm trên cùng.
import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'

export const stageStyles = stylex.create({
  stage: {
    position: 'relative',
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  panes: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0, display: 'flex', flexDirection: 'column' },
  // Đệm cuối vùng cuộn = tabbar-bottom + tabbar-h + khe (kit khai báo `--m-scroll-pad`),
  // đúng 118px của artboard M* ở inset 0. Vùng cuộn để `block`, không phải cột flex:
  // Chromium bỏ qua `padding-bottom` của hộp cuộn flex khi nội dung dài hơn khung,
  // nên hàng cuối vẫn chui xuống dưới thanh tab. Cột flex của màn nằm trong
  // `paneBody` (`min-height: 100%`) — màn vẫn nhận đủ chiều cao để tự chia chỗ.
  scroll: {
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minHeight: 0,
    display: 'block',
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    paddingBottom: 'var(--m-scroll-pad)',
  },
  /** Cột flex của màn trong vùng cuộn: đủ cao bằng khung, cao hơn khi nội dung dài. */
  paneBody: { display: 'flex', flexDirection: 'column', minHeight: '100%' },
  /** Màn không có thanh tab dưới (Video mới): không cần đệm. */
  noPad: { paddingBottom: 0 },
  push: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    zIndex: 1,
    display: 'block',
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    backgroundColor: color['--color-bg'],
    paddingBottom: 'var(--m-scroll-pad)',
  },
  /** Hàng chờ tự quản vùng cuộn của nó (artboard Queue) — không thêm đệm/không cuộn ngoài. */
  fullBleed: { overflowY: 'hidden', paddingBottom: 0 },
})
