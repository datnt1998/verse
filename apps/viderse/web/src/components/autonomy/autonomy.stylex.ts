// Mức tự chủ (artboard Channel › tab=sources: .vv-autonomy / .vv-arow / .vv-guard) —
// khoảng cách, bán kính, màu đều qua token (design-rules).
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

export const autonomyStyles = stylex.create({
  head: { display: 'flex', alignItems: 'flex-start', gap: space['--vv-s-12'] },
  orbs: { position: 'relative', width: space['--vv-s-22'], height: space['--vv-s-22'], marginTop: space['--vv-s-2'], flex: '0 0 auto' },
  orbsSm: { position: 'relative', width: space['--vv-s-16'], height: space['--vv-s-16'], marginTop: space['--vv-s-2'], flex: '0 0 auto' },
  dim: { opacity: 0.5 },
  orbQa: { position: 'absolute', left: 0, top: 0 },
  orbArtist: { position: 'absolute', right: 0, bottom: 0 },
  headText: { flex: '1 1 0', minWidth: 0, display: 'flex', flexDirection: 'column', gap: space['--vv-s-2'] },
  titleRow: { display: 'flex', alignItems: 'center', gap: space['--vv-s-8'] },
  title: { margin: 0, fontSize: 14, fontWeight: 600 },
  tag: { height: space['--vv-s-22'], paddingInline: space['--vv-s-8'], display: 'inline-flex', alignItems: 'center', borderRadius: corners['--vv-r-6'], backgroundColor: 'rgba(255,255,255,.06)', color: color['--color-fg-3'], fontSize: 11.5, whiteSpace: 'nowrap' },
  sub: { margin: 0, fontSize: 12.5, lineHeight: 1.5, color: color['--color-fg-3'] },
  seg: { alignSelf: 'flex-start', maxWidth: '100%' },
  dlgList: { margin: 0, marginTop: space['--vv-s-10'], padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: space['--vv-s-8'], fontSize: 13, color: color['--color-fg-2'] },
  dlgItem: { display: 'flex', alignItems: 'flex-start', gap: space['--vv-s-10'], lineHeight: 1.5 },
  dlgIcon: { flex: '0 0 auto', marginTop: space['--vv-s-2'] },
  dlgOk: { color: color['--color-ok'] },
  dlgWarn: { color: color['--color-warn'] },
  dlgNote: { color: 'var(--vv-fg-note)' },
  rows: { display: 'flex', flexDirection: 'column' },
  row: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'], paddingBlock: space['--vv-s-10'], borderTopWidth: space['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  rowLabel: { fontSize: 13.5, color: color['--color-fg'] },
  rowHint: { fontSize: 12, lineHeight: 1.45, color: 'var(--vv-fg-note)' },
  thr: { display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: space['--vv-s-10'] },
  help: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  thrStepper: { display: 'inline-flex', alignItems: 'stretch', width: 148, height: space['--vv-s-36'], borderRadius: corners['--vv-r-10'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: color['--color-surface-0'], overflow: 'hidden' },
  thrButton: { width: space['--vv-s-36'], flex: '0 0 auto', borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', backgroundColor: { default: 'transparent', ':hover:not(:disabled)': 'var(--vv-hover)' }, color: color['--color-fg-2'], cursor: { default: 'pointer', ':disabled': 'not-allowed' }, opacity: { default: null, ':disabled': 0.35 }, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' },
  thrValue: { flex: '1 1 0', minWidth: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', borderInlineWidth: space['--vv-s-1'], borderInlineStyle: 'solid', borderInlineColor: line['--line-3'], fontVariantNumeric: 'tabular-nums', fontSize: 13 },
  guard: { display: 'flex', alignItems: 'flex-start', gap: space['--vv-s-8'], padding: `${space['--vv-s-10']} ${space['--vv-s-12']}`, borderRadius: corners['--vv-r-10'], backgroundColor: 'rgba(255,255,255,.03)', fontSize: 12.5, lineHeight: 1.5, color: color['--color-fg-3'] },
  guardIcon: { flex: '0 0 auto', marginTop: space['--vv-s-2'] },
  status: { margin: 0, fontSize: 12.5, color: color['--color-fg-3'] },
  retry: { alignSelf: 'flex-start' },
  prioWrap: { paddingTop: space['--vv-s-8'], paddingInline: space['--vv-s-14'] },
  prio: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'] },
  prioLabel: { fontSize: 12.5, color: color['--color-fg-3'] },
})
