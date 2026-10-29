import * as stylex from '@stylexjs/stylex'
import { color, kit, line, tint, surface } from '@noverse/ui/tokens.stylex'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { useAction } from '../../data'
import { AgentOrb } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import type { StoryCheck } from './useStoryCheck'

const styles = stylex.create({
  block: { width: '100%', borderRadius: 18, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 14, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], backgroundImage: surface['--card-bg'], boxShadow: surface['--card-shadow'] },
  blocked: { borderColor: 'color-mix(in srgb, var(--color-err) 35%, transparent)' },
  compact: { borderRadius: 12, padding: 12, gap: 8, backgroundImage: 'none', boxShadow: 'none', backgroundColor: 'color-mix(in srgb, var(--kit-white) 1.5%, transparent)' },
  clear: { borderColor: 'color-mix(in srgb, var(--color-ok) 25%, transparent)' },
  header: { display: 'flex', alignItems: 'flex-start', gap: 12 },
  compactHeader: { alignItems: 'center', gap: 8 },
  heading: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 },
  title: { fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  compactTitle: { fontSize: 13 },
  detail: { fontSize: 12.5, color: kit['--kit-fg-icon'] },
  pill: { flexShrink: 0, height: 24, padding: '0 9px', display: 'flex', alignItems: 'center', gap: 6, borderRadius: 999, backgroundColor: line['--line-1'], color: color['--color-fg-3'], fontSize: 12, fontWeight: 600 },
  compactPill: { height: 20, padding: '0 7px', fontSize: 11.5 },
  error: { color: kit['--kit-err-soft'], backgroundColor: kit['--kit-errf-12'] },
  warning: { color: kit['--kit-warn-soft'], backgroundColor: kit['--kit-warn-14'] },
  ok: { color: color['--color-ok'], backgroundColor: kit['--kit-ok-12'] },
  dot: { width: 6, height: 6, borderRadius: '50%', backgroundColor: 'currentColor' },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(5, minmax(0, 1fr))', gap: 6 },
  cell: { minWidth: 0, padding: '7px 9px', borderRadius: 10, backgroundColor: line['--line-1'], display: 'flex', flexDirection: 'column', gap: 1 },
  duplicateCell: { backgroundColor: 'color-mix(in srgb, var(--color-err) 8%, transparent)', color: kit['--kit-err-soft'], boxShadow: 'inset 0 0 0 1px var(--kit-errf-55)' },
  overusedCell: { backgroundColor: 'color-mix(in srgb, var(--color-warn) 8%, transparent)', color: kit['--kit-warn-soft'], boxShadow: 'inset 0 0 0 1px var(--kit-warn-35)' },
  changedCell: { backgroundColor: tint['--tint-accent'], color: kit['--kit-badge-run'], boxShadow: 'inset 0 0 0 1px var(--kit-acc-45)' },
  truncate: { whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  axis: { fontSize: 11, color: kit['--kit-fg-icon'] },
  value: { fontSize: 12.5, fontWeight: 500 },
  old: { fontSize: 11, color: kit['--kit-fg-icon'], textDecoration: 'line-through' },
  checks: { display: 'flex', flexDirection: 'column', gap: 6 },
  compactChecks: { gap: 8 },
  check: { display: 'flex', alignItems: 'flex-start', gap: 10, fontSize: 13, lineHeight: 1.5 },
  compactCheck: { gap: 8, fontSize: 12.5 },
  badge: { flexShrink: 0, width: 64, height: 20, marginTop: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 6, fontSize: 11.5, fontWeight: 600 },
  compactBadge: { width: 58, height: 19, borderRadius: 5, fontSize: 11 },
  checkText: { flex: 1, minWidth: 0, color: kit['--kit-fg-note'] },
  compactText: { color: color['--color-fg-2'] },
  actions: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8, paddingTop: 2 },
  note: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8, paddingTop: 2, fontSize: 12.5, color: color['--color-fg-3'] },
  noteText: { flex: 1, minWidth: 200 },
  link: { marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 4, fontSize: 12.5, color: color['--color-fg-3'] },
  compactLink: { marginLeft: 0, fontSize: 12, color: kit['--kit-fg-icon'] },
})

export function StoryCheckBlock({ project, gate, compact = false, onEdit }: { project: string; gate: StoryCheck; compact?: boolean; onEdit?: () => void }) {
  const action = useAction(gate.suggest)
  if (!gate.state) return null
  const { result, changed = [], previous } = gate.state
  const copy = t.newStoryCheck
  const errors = result?.checks.filter((check) => check.level === 'error').length ?? 0
  const title = gate.checking ? copy.checkingTitle : gate.blocked ? copy.conflictTitle : previous ? copy.fixedTitle : copy.clearTitle
  const pill = gate.checking ? copy.comparing : gate.blocked ? copy.blocked(errors) : copy.allowed
  const tone = gate.checking ? undefined : gate.blocked ? styles.error : styles.ok
  const block = stylex.props(styles.block, compact && styles.compact, gate.blocked && styles.blocked, !!result && !gate.blocked && styles.clear)
  return <section {...block} aria-label={copy.label} aria-live="polite" aria-busy={gate.checking || gate.suggesting}>
    <div {...stylex.props(styles.header, compact && styles.compactHeader)}>
      <AgentOrb agent="architect" size={compact ? 16 : 26} style={{ marginTop: compact ? 0 : 1 }} />
      <div {...stylex.props(styles.heading)}>
        <span {...stylex.props(styles.title, compact && styles.compactTitle)}>{compact ? copy.compactTitle : title}</span>
        {!compact && <span {...stylex.props(styles.detail)}>{copy.detail}</span>}
      </div>
      <span {...stylex.props(styles.pill, compact && styles.compactPill, tone)}>{!compact && <span aria-hidden="true" {...stylex.props(styles.dot)} />}{pill}</span>
    </div>
    {gate.checking && (compact ? <span {...stylex.props(styles.detail)}>{copy.checkingDetail}</span> : <div {...stylex.props(styles.grid)}>{Array.from({ length: 10 }, (_, n) => <Skeleton key={n} height={46} radius={10} />)}</div>)}
    {result && <>
      {!compact && result.axes.length > 0 && <div {...stylex.props(styles.grid)}>{result.axes.map((axis, index) => {
        const value = result.values[index] ?? '—'
        const modified = changed.includes(axis.n) && !!previous
        const duplicate = result.checks.some((check) => check.level === 'error' && check.axes.includes(axis.n))
        const overused = result.checks.some((check) => check.level === 'warning' && check.axes.includes(axis.n))
        const label = copy.axis(axis.n, copy.axisNames[axis.n - 1] ?? axis.name)
        return <div key={axis.n} title={`${copy.axis(axis.n, axis.name)}: ${value}${modified ? ` (${copy.previous(previous!.result.values[index])})` : ''}`}
          {...stylex.props(styles.cell, overused && styles.overusedCell, modified && styles.changedCell, duplicate && styles.duplicateCell)}>
          <span {...stylex.props(styles.axis, styles.truncate)}>{label}</span>
          <span title={value} {...stylex.props(styles.value, styles.truncate)}>{value}</span>
          {modified && <span {...stylex.props(styles.old, styles.truncate)}>{previous!.result.values[index]}</span>}
        </div>
      })}</div>}
      <div {...stylex.props(styles.checks, compact && styles.compactChecks)}>{result.checks.map((check, index) => {
        // §H câu 2: bản gọn dùng câu ngắn của máy chủ; chỉ khi rỗng mới cắt “…” kèm `title`.
        const compactText = compact ? check.short || check.text : check.text
        return <div key={`${check.rule}-${index}`} {...stylex.props(styles.check, compact && styles.compactCheck)}>
          <span {...stylex.props(styles.badge, compact && styles.compactBadge, check.level === 'error' ? styles.error : check.level === 'warning' ? styles.warning : styles.ok)}>{check.level === 'error' ? copy.error : check.level === 'warning' ? copy.warning : copy.ok}</span>
          <span title={compact && !check.short ? check.text : undefined} {...stylex.props(styles.checkText, compact && styles.compactText, compact && !check.short && styles.truncate)}>{compactText}</span>
        </div>
      })}</div>
      {gate.blocked && <div {...stylex.props(styles.actions)}>
        <Button variant={compact ? 'ghost' : 'primary'} onClick={() => void action.run()} disabled={gate.suggesting} pending={gate.suggesting} pendingLabel={copy.suggesting}
          style={{ height: compact ? 32 : 34, borderRadius: compact ? 8 : 9, fontSize: compact ? 12.5 : 13, width: compact ? '100%' : undefined }} icon={<AgentOrb agent="architect" size={compact ? 14 : 16} />}>{copy.suggest}</Button>
        {!compact && <Button onClick={onEdit} style={{ height: 34, borderRadius: 9 }}>{copy.edit}</Button>}
        {!compact && <a href={paths.projectBrain(project)} {...stylex.props(styles.link)}>{copy.projectBrain}<Icon name="arrow-right-2" size={12} /></a>}
      </div>}
      {previous && <div {...stylex.props(styles.note)}>
        {!compact && <Icon name="tick-circle-bulk" size={15} style={{ color: 'var(--color-ok)' }} />}
        <span {...stylex.props(styles.noteText)} style={compact ? { minWidth: 0, fontSize: 12 } : undefined}>{compact ? copy.compactChanged(changed.length) : copy.changed(changed.length)}</span>
        <Button onClick={gate.undo} style={compact ? { height: 'auto', padding: 0, border: 'none', background: 'transparent', fontSize: 12, textDecoration: 'underline' } : { height: 30, padding: '0 10px', borderRadius: 8, fontSize: 12.5 }}>{copy.undo}</Button>
      </div>}
    </>}
    {compact && <a href={paths.projectBrain(project)} {...stylex.props(styles.link, styles.compactLink)}>{copy.compactBrain}</a>}
    {action.error instanceof Error && <span role="alert" {...stylex.props(styles.detail)}>{action.error.message}</span>}
  </section>
}
