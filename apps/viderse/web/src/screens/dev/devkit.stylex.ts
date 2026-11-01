// Khung trang devkit (#/dev/kit) — lưới ô chứng minh thành phần và khung điện
// thoại giả lập. Dùng chung cho DevKitScreen và TokenSheet (bảng token).
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-dev__*).
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'

export const devkit = stylex.create({
  page: {
    padding: 'var(--vv-s-18) var(--vv-s-22) var(--vv-s-60)',
    display: 'flex',
    flexDirection: 'column',
    gap: 'var(--vv-s-18)',
    maxWidth: 1360,
    margin: '0 auto',
  },
  head: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-14)', flexWrap: 'wrap' },
  title: { fontSize: 17, fontWeight: 700, margin: 0 },
  grid: { display: 'grid', gap: 'var(--vv-s-14)', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))' },
  cell: {
    display: 'flex',
    flexDirection: 'column',
    gap: 'var(--vv-s-10)',
    padding: 'var(--vv-s-14)',
    borderRadius: 'var(--vv-r-14)',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-1'],
    backgroundColor: color['--color-surface-1'],
  },
  cellWide: { gridColumn: '1 / -1' },
  cellHead: {
    fontSize: 11,
    fontWeight: 600,
    letterSpacing: '.06em',
    textTransform: 'uppercase',
    color: color['--color-fg-4'],
  },
  row: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-10)', flexWrap: 'wrap' },
  mobile: {
    display: 'flex',
    flexDirection: 'column',
    borderRadius: 'var(--vv-r-26)',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    backgroundColor: color['--color-bg'],
    overflow: 'hidden',
    boxShadow: 'var(--vv-shadow-device)',
    width: 390,
    minHeight: 760,
    margin: '0 auto',
    position: 'relative',
  },
  mobileBody: { flex: '1 1 auto', paddingBottom: 'var(--vv-s-80)', overflowY: 'auto' },
})
