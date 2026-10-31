import * as stylex from '@stylexjs/stylex'
import { color, kit, radius } from '@noverse/ui/tokens.stylex'
import { metrics } from './editor.stylex'
import { v13 } from './editor-tokens.stylex'

export const gateStyles = stylex.create({
  head: { display: 'flex', alignItems: 'flex-start', gap: metrics['--ed-space-14'] },
  icon: { width: 42, height: 42, flexShrink: 0, borderRadius: radius['--radius-tile'], display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: kit['--kit-warn-12'], color: color['--color-warn'] },
  text: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: metrics['--ed-space-4'] },
  title: { margin: 0, fontSize: 16, fontWeight: 600, lineHeight: 1.4 },
  description: { margin: 0, fontSize: 13.5, color: kit['--kit-fg-body'], lineHeight: 1.55 },
  list: { margin: 0, paddingBlock: metrics['--ed-space-10'], paddingLeft: metrics['--ed-space-30'], paddingRight: metrics['--ed-space-12'], borderRadius: v13.choiceRadius, borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-hairline)', backgroundColor: v13.stage, display: 'flex', flexDirection: 'column', gap: metrics['--ed-space-5'], fontSize: 12.5, color: color['--color-fg-2'], lineHeight: 1.55 },
})
