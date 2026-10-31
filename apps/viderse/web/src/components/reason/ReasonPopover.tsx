import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { Orb } from '../Orb'
import type { Assistant } from '../agents'

export type Reason = { agent: Assistant; text: string; facts: { label: string; value: string; source: string }[]; fix?: { label: string; href: string } }
const names: Record<Assistant, string> = { scout: 'Trinh sát', director: 'Đạo diễn', artist: 'Hoạ sĩ', qa: 'Kiểm định' }

export function ReasonPopover({ reason, label = 'Vì sao?' }: { reason: Reason; label?: string }) {
  return <div {...stylex.props(styles.root)} role="dialog" aria-label={label}>
    <div {...stylex.props(styles.head)}><Orb of={reason.agent} size={16} /><span>{names[reason.agent]} giải thích</span></div>
    <div {...stylex.props(styles.text)}>{reason.text}</div>
    <div {...stylex.props(styles.facts)}>{reason.facts.map((fact, index) => <div {...stylex.props(styles.fact)} key={`${fact.label}-${index}`}><span>{fact.label}</span><b {...stylex.props(styles.value)}>{fact.value}</b><span {...stylex.props(styles.source)}>{fact.source}</span></div>)}</div>
    {reason.fix && <a href={reason.fix.href}>{reason.fix.label}</a>}
  </div>
}
const styles = stylex.create({
  root: { padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)', borderRadius: 'var(--vv-r-14)', backgroundColor: color['--color-surface-4'], borderWidth: '1px', borderStyle: 'solid', borderColor: line['--line-4'], boxShadow: kit['--kit-shadow-pop'], fontSize: '13px', lineHeight: 1.5, color: 'var(--vv-fg-soft)' },
  head: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', fontSize: '12px', color: 'var(--vv-scout-ink)', fontWeight: 600 },
  text: { color: color['--color-fg'], fontSize: '13.5px' },
  facts: { display: 'flex', flexDirection: 'column', borderTopWidth: '1px', borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  fact: { display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 'var(--vv-s-2) var(--vv-s-10)', padding: 'var(--vv-s-7) 0', borderBottomWidth: '1px', borderBottomStyle: 'solid', borderBottomColor: line['--line-1'] },
  value: { fontWeight: 600, color: color['--color-fg'], fontVariantNumeric: 'tabular-nums', textAlign: 'right' },
  source: { gridColumn: '1 / -1', fontSize: '11.5px', color: 'var(--vv-fg-note)' },
})
