// IdleCard — thẻ ở giữa khung luồng khi không có trợ lý nào chạy
// (Main.dc.html › phase=wait · ready · done). Giá trị lấy nguyên từ artboard.
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, tint } from '@noverse/ui/tokens.stylex'

export const s = stylex.create({
  center: {
    flex: '1 1 auto', minHeight: 0,
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    paddingBlockStart: 6, paddingBlockEnd: 18,
  },
  card: {
    width: 'min(660px, 100%)',
    display: 'flex', gap: 14,
    paddingBlock: 16, paddingInline: 18,
    borderRadius: 14,
    backgroundColor: color['--color-card'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    boxSizing: 'border-box',
  },
  wait: { borderColor: kit['--kit-warn-30'] },
  ready: { borderColor: kit['--kit-acc-30'] },
  done: { borderColor: kit['--kit-ok-30'] },
  glyph: {
    flex: '0 0 auto', width: 34, height: 34, borderRadius: 10,
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  },
  glyphWait: { backgroundColor: tint['--tint-warn'], color: color['--color-warn'] },
  glyphReady: { backgroundColor: tint['--tint-accent'], color: color['--color-accent'] },
  glyphDone: { backgroundColor: tint['--tint-ok'], color: color['--color-ok'] },
  body: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 8 },
  title: { fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  text: { fontSize: 13, color: color['--color-fg-3'], lineHeight: 1.6, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' },
  actions: { display: 'flex', flexWrap: 'wrap', gap: 8, paddingBlockStart: 2 },
  hint: { fontSize: 12, color: color['--color-fg-4'] },
  // Nút trong thẻ: 32 px · bo 8 · 13 px (artboard), không phải Button kit 36/10.
  action: {
    height: 32, paddingBlock: 0, paddingInlineStart: 10, paddingInlineEnd: 12,
    maxWidth: '100%',
    display: 'inline-flex', alignItems: 'center', gap: 6,
    borderRadius: 8, borderWidth: 1, borderStyle: 'solid',
    fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap',
    transition: 'background .15s var(--ease-out)',
    cursor: 'pointer',
  },
  actionLabel: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' },
  primary: {
    backgroundColor: color['--color-fg'], color: color['--color-bg'],
    borderColor: color['--color-fg'], fontWeight: 600,
    ':hover': { backgroundColor: 'white' },
  },
  ghost: {
    backgroundColor: 'transparent', color: color['--color-fg'],
    borderColor: kit['--kit-wash-12'],
    ':hover': { backgroundColor: line['--line-2'] },
  },
})
