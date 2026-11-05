import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'
import { space, typography } from '../../styles/viderse.stylex'

// Ô nhập link trong xác nhận đánh dấu đã đăng.
export const performanceStyles = stylex.create({
  dialogField: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'], marginTop: space['--vv-s-12'] },
  dialogLabel: { fontSize: typography['--vv-fs-meta'], color: color['--color-fg-3'] },
})
