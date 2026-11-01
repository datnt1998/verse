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
  // Đệm cuối vùng cuộn = tabbar-bottom + tabbar-h + khe (kit khai báo `--m-scroll-pad`);
  // trước khi kit lên, giữ đệm cũ của khung để hàng cuối không nằm dưới thanh tab.
  scroll: {
    flexGrow: 1,
    flexShrink: 1,
    flexBasis: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    paddingBottom: 'var(--m-scroll-pad, var(--vv-tabbar-pad))',
  },
  /** Màn không có thanh tab dưới (Video mới): không cần đệm. */
  noPad: { paddingBottom: 0 },
  push: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    zIndex: 1,
    display: 'flex',
    flexDirection: 'column',
    overflowY: 'auto',
    overscrollBehavior: 'contain',
    backgroundColor: color['--color-bg'],
    paddingBottom: 'var(--m-scroll-pad, var(--vv-tabbar-pad))',
  },
  /** Hàng chờ tự quản vùng cuộn của nó (artboard Queue) — không thêm đệm/không cuộn ngoài. */
  fullBleed: { overflowY: 'hidden', paddingBottom: 0 },
})
