import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'

export const exportStyles = stylex.create({
  scopes: { display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 10 },
  scope: { display: 'flex', gap: 12, alignItems: 'flex-start', padding: '12px 14px', borderRadius: 14, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-surface-1'], color: color['--color-fg'], textAlign: 'left', fontSize: 13.5 },
  selected: { borderColor: kit['--kit-acc-55'], backgroundColor: kit['--kit-acc-6'] },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 6 },
  mobileGrid: { gridTemplateColumns: 'repeat(5,minmax(0,1fr))' },
  chapter: { height: 48, padding: '0 10px', borderRadius: 10, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-surface-0'], color: color['--color-fg'], display: 'flex', alignItems: 'center', gap: 8, textAlign: 'left', fontSize: 13 },
  mobileChapter: { height: 44, borderRadius: 12, justifyContent: 'center', fontSize: 15, fontWeight: 600 },
  disabled: { opacity: .4 },
  sheet: { maxHeight: 'calc(100dvh - 100px)', overflowY: 'auto' },
})
