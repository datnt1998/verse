// Kiểu dùng chung của app. Màu và khoảng cách đọc token của kit hoặc Viderse.
import * as stylex from '@stylexjs/stylex'
import { color, line, surface as kitSurface } from '@noverse/ui/tokens.stylex'

/** Số canh cột, nhãn nhóm và chữ chỉ dành cho trình đọc màn hình. */
export const text = stylex.create({
  num: { fontVariantNumeric: 'tabular-nums' },
  lbl: {
    fontSize: 11,
    fontWeight: 600,
    letterSpacing: '.08em',
    color: color['--color-fg-4'],
    textTransform: 'uppercase',
  },
  srOnly: {
    position: 'absolute',
    width: 1,
    height: 1,
    overflow: 'hidden',
    clip: 'rect(0 0 0 0)',
    whiteSpace: 'nowrap',
  },
})

/** Khối: thẻ “liquid dark”, dòng lỗi cam (một câu + một cách sửa), và ô giãn hàng flex. */
export const box = stylex.create({
  card: {
    backgroundImage: kitSurface['--card-bg'],
    borderWidth: 'var(--vv-s-1)',
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    boxShadow: kitSurface['--card-shadow'],
  },
  errline: {
    display: 'flex',
    alignItems: 'center',
    gap: 'var(--vv-s-10)',
    flexWrap: 'wrap',
    padding: 'var(--vv-s-10) var(--vv-s-12)',
    borderRadius: 'var(--radius-row)',
    borderWidth: 'var(--vv-s-1)',
    borderStyle: 'solid',
    borderColor: 'var(--vv-warn-bd)',
    backgroundColor: 'var(--vv-warn-bg)',
    fontSize: 12.5,
    color: 'var(--vv-warn-ink)',
    lineHeight: 1.55,
  },
  grow: { flex: 1, minWidth: 0 },
})
