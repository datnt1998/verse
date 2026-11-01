// Khung hộp xác nhận Mức tự chủ (artboard Dialogs: .ov .dlg 520px, .dlg-ic, .li, .kbd).
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, surface } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

export const dialogStyles = stylex.create({
  scrim: {
    position: 'fixed', inset: 0, zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: space['--vv-s-20'],
    backgroundColor: kit['--kit-scrim'], backdropFilter: 'blur(3px)',
    animationName: { default: 'kit-fade', [NO_MOTION]: 'none' }, animationDuration: '.2s', animationTimingFunction: 'ease-out', animationFillMode: 'both',
  },
  scrimClosing: { animationName: { default: 'kit-fade-out', [NO_MOTION]: 'none' }, animationDuration: '.2s', animationTimingFunction: 'ease-in', animationFillMode: 'both' },
  dialog: {
    width: 'min(520px, calc(100vw - 32px))', display: 'flex', flexDirection: 'column', gap: space['--vv-s-16'], padding: space['--vv-s-20'],
    backgroundColor: color['--color-surface-3'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], borderRadius: 18,
    boxShadow: surface['--dialog-shadow'],
    animationName: { default: 'kit-dialog', [NO_MOTION]: 'none' }, animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.25,1.08)', animationFillMode: 'both',
  },
  dialogClosing: { animationName: { default: 'kit-dialog-out', [NO_MOTION]: 'none' }, animationDuration: '.18s', animationTimingFunction: 'cubic-bezier(.5,0,.75,0)', animationFillMode: 'both' },
  head: { display: 'flex', alignItems: 'flex-start', gap: space['--vv-s-12'] },
  chip: { width: 42, height: 42, flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: corners['--vv-r-12'], backgroundColor: 'rgba(166,237,193,.10)', color: color['--color-ok'] },
  headText: { flex: '1 1 0', minWidth: 0 },
  title: { margin: 0, fontSize: 17, fontWeight: 600, lineHeight: 1.35 },
  lead: { margin: '2px 0 0', fontSize: 13, lineHeight: 1.5, color: color['--color-fg-3'] },
  closeButton: { width: 32, height: 32, marginTop: -4, marginRight: -6, flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: corners['--vv-r-8'], borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', backgroundColor: { default: 'transparent', ':hover': 'var(--vv-hover)' }, color: 'var(--vv-fg-note)', cursor: 'pointer' },
  list: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-8'] },
  item: { display: 'flex', alignItems: 'flex-start', gap: space['--vv-s-10'], fontSize: 13, lineHeight: 1.5, color: color['--color-fg-2'] },
  itemIcon: { flex: '0 0 auto', marginTop: 2 },
  itemWarn: { color: color['--color-warn'] },
  itemNote: { color: 'var(--vv-fg-note)' },
  error: { fontSize: 12.5, color: color['--color-err'] },
  footer: { display: 'flex', alignItems: 'center', gap: space['--vv-s-8'], paddingTop: 2 },
  hint: { flex: '1 1 0', display: 'flex', alignItems: 'center', gap: space['--vv-s-6'], fontSize: 12, color: 'var(--vv-fg-note)' },
})
