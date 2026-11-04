// Kiểu dòng Đề xuất trong cột trái Bàn dựng V37 (artboard Main › nhóm Chờ bạn):
// orb + câu + "<Trợ lý> đề xuất · <bằng chứng>" + nút ×. Số đo đo từ board 1440×900.
import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'

export const suggestStyles = stylex.create({
  row: {
    position: 'relative', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minHeight: 44,
    paddingTop: s['--vv-s-7'], paddingBottom: s['--vv-s-7'], paddingLeft: s['--vv-s-8'], paddingRight: s['--vv-s-4'],
    borderRadius: r['--vv-r-8'],
  },
  orb: { flexShrink: 0, display: 'flex' },
  rowMain: {
    flex: 1, minWidth: 0, padding: 0, borderWidth: 0, backgroundColor: 'transparent', color: 'inherit',
    fontFamily: 'inherit', textAlign: 'left', cursor: 'pointer', display: 'flex', flexDirection: 'column', gap: 1,
    ':disabled': { cursor: 'default', opacity: .6 },
    ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 },
  },
  title: {
    fontSize: 13, color: color['--color-fg-2'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  evidence: {
    fontSize: 11.5, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  ink: (value: string) => ({ color: value }),
  notice: { fontSize: 11.5, color: 'var(--vv-warn-ink)', whiteSpace: 'normal' },
  skip: {
    flex: '0 0 auto', width: 24, height: 24, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', '@media (pointer: coarse)': { width: 44, height: 44 },
    borderWidth: 0, borderRadius: r['--vv-r-6'], backgroundColor: 'transparent', color: color['--color-fg-4'],
    cursor: 'pointer', ':hover': { color: color['--color-fg'], backgroundColor: color['--color-surface-4'] },
    ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 1 },
    ':disabled': { cursor: 'default', opacity: .5 },
  },
})
