import * as stylex from '@stylexjs/stylex'
import type { ConfigModelInfo } from '../api/types'
import { t } from '../i18n/vi'
import { CONTEXT_PRESETS, contextThousands, contextThreshold, formatContextK, type ContextDraft } from '../lib/modelContext'
import { color, line, tint, kit } from '@noverse/ui/tokens.stylex'
import { Radio, RadioCard } from '@noverse/ui'
import { Icon } from '@noverse/ui'

const copy = t.settingsModelsW
const sx = stylex.create({
  root: { display: 'flex', flexDirection: 'column', gap: 8, minWidth: 0 },
  choices: { display: 'flex', gap: 4, padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'] },
  stacked: { flexDirection: 'column', gap: 0, padding: 0, borderRadius: 16, backgroundColor: color['--color-card'], borderColor: line['--line-1'], overflow: 'hidden' },
  settingsChoices: { gap: 10, padding: 0, borderWidth: 0, backgroundColor: 'transparent', overflow: 'visible' },
  choice: { display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '1 1 0', minWidth: 0, gap: 6, height: 30, paddingBlock: 0, paddingInline: 8, borderWidth: 0, borderRadius: 7, color: color['--color-fg-3'], backgroundColor: 'transparent', fontSize: 12, whiteSpace: 'nowrap', cursor: 'pointer' },
  selected: { backgroundColor: color['--color-surface-7'], color: color['--color-fg'] },
  selectedChip: { borderColor: color['--color-accent'], backgroundColor: tint['--tint-accent'] },
  body: { display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8 },
  settingsBody: { alignItems: 'stretch', flexDirection: 'column' },
  mobileBody: { alignItems: 'stretch', flexDirection: 'column', gap: 10, marginTop: 10 },
  indent: { paddingLeft: 26 },
  inputLine: { display: 'flex', alignItems: 'center', gap: 6, color: color['--color-fg-3'], fontSize: 12.5 },
  mobileLine: { gap: 10, fontSize: 15 },
  input: { width: 76, height: 30, paddingInline: 8, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], borderRadius: 8, backgroundColor: color['--color-surface-0'], color: color['--color-fg'], textAlign: 'right', fontSize: 13, fontVariantNumeric: 'tabular-nums', outline: 'none' },
  settingsInput: { width: 96, height: 32, paddingInline: 10, borderRadius: 9 },
  mobileInput: { width: 120, height: 46, paddingInline: 14, borderRadius: 13, fontSize: 16 },
  presets: { display: 'flex', gap: 8, flexWrap: 'wrap' },
  scroll: { flexWrap: 'nowrap', overflowX: 'auto', scrollbarWidth: 'none', marginInline: -16, paddingInline: 16 },
  chip: { flexShrink: 0, height: 26, paddingInline: 9, borderRadius: 7, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 11.5, fontVariantNumeric: 'tabular-nums', cursor: 'pointer' },
  mobileChip: { height: 34, paddingInline: 12, borderRadius: 10, fontSize: 13.5 },
  small: { fontSize: 12, lineHeight: 1.5, color: kit['--kit-fg-icon'], margin: 0 },
  mobileNote: { marginTop: 10, marginInline: 14, fontSize: 12.5 },
  mobileRoot: { gap: 0 },
  settingsRoot: { gap: 10 },
  mobileSub: { fontSize: 13, color: kit['--kit-fg-icon'] },
  error: { color: color['--color-err'] },
  copy: { display: 'flex', flexDirection: 'column', gap: 1 },
})

export function ModelContextChoices({ model, draft, onChange, disabled, variant = 'panel', onCommit }: {
  model: ConfigModelInfo; draft: ContextDraft; onChange: (draft: ContextDraft) => void; disabled: boolean
  variant?: 'panel' | 'settings' | 'mobile'; onCommit?: (window: number | null) => void
}) {
  const custom = draft.mode === 'custom'
  const window = custom ? contextThousands(draft.value) : model.automaticWindow
  const change = (next: ContextDraft, commit = true) => {
    if (disabled) return
    onChange(next)
    const value = next.mode === 'auto' ? null : contextThousands(next.value)
    if (commit && (next.mode === 'auto' || value !== null)) onCommit?.(value)
  }
  const automatic = `${model.automaticSource === 'default' ? copy.fallback : copy.auto} · ${formatContextK(model.automaticWindow)}`
  const stacked = variant !== 'panel'
  return <div {...stylex.props(sx.root, variant === 'mobile' && sx.mobileRoot, variant === 'settings' && sx.settingsRoot)}>
    <div role="radiogroup" aria-label={copy.title(model.name)} {...stylex.props(sx.choices, stacked && sx.stacked, variant === 'settings' && sx.settingsChoices)}>
      {variant === 'settings' ? <>
        <RadioCard checked={!custom} disabled={disabled} onChange={() => change({ ...draft, mode: 'auto' })} style={{ padding: '8px 10px', borderRadius: 10, gap: 10, alignItems: 'flex-start' }}><span {...stylex.props(sx.copy)}><span>{automatic}</span><span {...stylex.props(sx.small)}>{model.automaticSource === 'default' ? copy.fallbackNote : copy.autoNote}</span></span></RadioCard>
        <RadioCard checked={custom} disabled={disabled} onChange={() => change({ ...draft, mode: 'custom' })} style={{ padding: '8px 10px', borderRadius: 10, gap: 10, alignItems: 'flex-start' }}><span {...stylex.props(sx.copy)}><span>{copy.custom}</span><span {...stylex.props(sx.small)}>{copy.customNote}</span></span></RadioCard>
      </> : variant === 'mobile' ? <>
        <Radio checked={!custom} disabled={disabled} mark="tick" markPosition="end" aria-label={automatic} label={automatic} leading={model.automaticSource === 'default' ? <Icon name="warning-2" size={18} style={{ color: 'var(--color-warn)' }} /> : undefined} description={<span {...stylex.props(sx.mobileSub)}>{model.automaticSource === 'default' ? copy.fallbackNote : copy.autoNote}</span>} onChange={() => change({ ...draft, mode: 'auto' })} style={{ minHeight: 54, padding: '8px 14px', margin: 0, fontSize: 15, lineHeight: 1.35 }} />
        <Radio checked={custom} disabled={disabled} mark="tick" markPosition="end" aria-label={copy.custom} label={copy.custom} description={<span {...stylex.props(sx.mobileSub)}>{copy.using(formatContextK(window ?? model.contextWindow))}</span>} onChange={() => change({ ...draft, mode: 'custom' })} style={{ minHeight: 54, padding: '8px 14px', margin: 0, borderTop: '1px solid var(--line-1)', fontSize: 15, lineHeight: 1.35 }} />
      </> : <>
        <button type="button" role="radio" aria-checked={!custom} disabled={disabled} {...stylex.props(sx.choice, !custom && sx.selected)} onClick={() => change({ ...draft, mode: 'auto' })}>{automatic}</button>
        <button type="button" role="radio" aria-checked={custom} disabled={disabled} {...stylex.props(sx.choice, custom && sx.selected)} onClick={() => change({ ...draft, mode: 'custom' })}>{copy.custom}</button>
      </>}
    </div>
    {custom && <div {...stylex.props(sx.body, variant === 'settings' && sx.settingsBody, variant === 'settings' && sx.indent, variant === 'mobile' && sx.mobileBody)}>
      <label {...stylex.props(sx.inputLine, variant === 'mobile' && sx.mobileLine)}><input type="number" inputMode="numeric" aria-label={copy.inputLabel} min={8} max={2000} value={draft.value} disabled={disabled} aria-invalid={window === null} {...stylex.props(sx.input, variant === 'settings' && sx.settingsInput, variant === 'mobile' && sx.mobileInput)} onChange={event => change({ mode: 'custom', value: event.target.value })} />{copy.thousand}</label>
      <div {...stylex.props(sx.presets, variant === 'mobile' && sx.scroll)}>{CONTEXT_PRESETS.map(n => <button key={n} type="button" disabled={disabled} {...stylex.props(sx.chip, variant === 'mobile' && sx.mobileChip, window === n && sx.selectedChip)} onClick={() => change({ mode: 'custom', value: String(n / 1000) })}>{formatContextK(n)}</button>)}</div>
      {window === null && <p role="alert" {...stylex.props(sx.small, sx.error)}>{copy.range}</p>}
    </div>}
    {window !== null && <p {...stylex.props(sx.small, variant === 'mobile' && sx.mobileNote)}>{variant === 'settings' ? copy.threshold(formatContextK(contextThreshold(model, window))) : copy.editorThreshold(formatContextK(contextThreshold(model, window)), variant === 'mobile')}</p>}
  </div>
}
