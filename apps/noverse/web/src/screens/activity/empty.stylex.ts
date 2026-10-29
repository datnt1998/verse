// Nhật ký tiến độ — trạng thái lọc rỗng (Activity.dc.html › `empty`: một dòng chữ lệch giữa).
import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'

export const s = stylex.create({
  filteredEmpty: {
    margin: 0,
    paddingBlock: 32,
    paddingInline: 0,
    textAlign: 'center',
    color: color['--color-fg-4'],
    fontSize: 14,
  },
})
