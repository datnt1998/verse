import * as stylex from '@stylexjs/stylex'
import { color } from './tokens.stylex'
import { Button } from './Button'
import { getKitLabels } from './labels'

const sx = stylex.create({
  root: {
    paddingTop: 10, paddingBottom: 10, paddingInline: 14,
    borderWidth: 1, borderStyle: 'solid', borderColor: color['--color-warn'],
    borderRadius: 10, display: 'flex', alignItems: 'center', flexWrap: 'wrap',
    gap: 10, fontSize: 13, color: color['--color-warn'],
  },
  label: { flex: 1 },
})

export function ConflictBar({ onView, onOverwrite, pending = false }: { onView: () => unknown; onOverwrite: () => unknown; pending?: boolean }) {
  return <div role="alert" {...stylex.props(sx.root)}>
    <span {...stylex.props(sx.label)}>{getKitLabels().conflict.changedElsewhere}</span>
    <Button size="sm" disabled={pending} onClick={() => { void onView() }}>{getKitLabels().conflict.viewNew}</Button>
    <Button size="sm" pending={pending} onClick={() => { void onOverwrite() }}>{getKitLabels().conflict.overwrite}</Button>
  </div>
}
