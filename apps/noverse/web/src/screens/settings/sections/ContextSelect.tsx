import { useEffect, useId, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { ConfigModelInfo } from '../../../api/types'
import { useModelContext } from '../../../data/modelContext'
import { t } from '../../../i18n/vi'
import { contextDraft, contextThousands, formatContextK, type ContextDraft } from '../../../lib/modelContext'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'
import { ModelContextChoices } from '../../../ui/ModelContextChoices'
import { usePopup } from '@noverse/ui'

const copy = t.settingsModelsW
const sx = stylex.create({
  root: { position: 'relative', minWidth: 0 },
  open: { zIndex: 40 },
  trigger: { display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, width: '100%', height: 36, paddingLeft: 11, paddingRight: 8, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], borderRadius: 9, backgroundColor: color['--color-surface-0'], color: color['--color-fg'], fontSize: 13, cursor: 'pointer' },
  focused: { borderColor: line['--line-focus'], boxShadow: '0 0 0 3px var(--tint-accent)' },
  fallback: { borderColor: color['--color-warn'] },
  number: { flexShrink: 0, fontWeight: 600, fontVariantNumeric: 'tabular-nums' },
  value: { flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: kit['--kit-fg-icon'], fontSize: 12 },
  arrow: { flexShrink: 0, color: color['--color-fg-4'] },
  pop: { position: 'absolute', zIndex: 40, right: 0, top: 'calc(100% + 8px)', width: 360, maxWidth: 'calc(100vw - 40px)', maxHeight: 'calc(100vh - 100px)', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 10, padding: 14, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], borderRadius: 14, backgroundColor: color['--color-surface-4'], color: color['--color-fg'], boxShadow: kit['--kit-shadow-pop'], fontSize: 13 },
  up: { top: 'auto', bottom: 'calc(100% + 8px)' },
  heading: { fontSize: 14, fontWeight: 600 },
  intro: { display: 'flex', flexDirection: 'column', gap: 2 },
  small: { fontSize: 12.5, lineHeight: 1.5, color: kit['--kit-fg-icon'], margin: 0 },
  warning: { display: 'flex', gap: 8, paddingBlock: 9, paddingInline: 10, borderRadius: 10, backgroundColor: kit['--kit-warn-7'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-warn-25'], color: color['--color-warn'] },
  button: { paddingBlock: 6, paddingInline: 10, borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 12, cursor: 'pointer' },
  footer: { display: 'flex', alignItems: 'center', gap: 8, paddingTop: 10, borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  applies: { flex: 1, fontSize: 11.5, lineHeight: 1.45, color: color['--color-fg-4'] },
  save: { color: color['--color-bg'], backgroundColor: color['--color-fg'], borderColor: color['--color-fg'] },
})

export function ContextSelect({ provider, model: initial, label, admin, onSave }: { provider: string; model: ConfigModelInfo; label: string; admin: boolean; onSave: (value: number | null) => Promise<void> }) {
  const resource = useModelContext(provider, initial.name)
  const model = resource.data ?? initial
  const { root, open, setOpen } = usePopup()
  const trigger = useRef<HTMLButtonElement>(null)
  const dialog = useRef<HTMLDivElement>(null)
  const id = useId()
  const [draft, setDraft] = useState<ContextDraft>(() => contextDraft(model))
  const [upward, setUpward] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const fallback = model.contextSource === 'default'
  const value = contextThousands(draft.value)
  const close = () => { if (!pending) { setOpen(false); trigger.current?.focus() } }
  useEffect(() => {
    if (!open) return
    const place = () => {
      const rect = trigger.current?.getBoundingClientRect()
      const height = dialog.current?.getBoundingClientRect().height || 420
      if (rect) setUpward(window.innerHeight - rect.bottom < height + 8 && rect.top > window.innerHeight - rect.bottom)
    }
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true) }
  }, [open, draft.mode, error])
  const text = `${fallback ? '⚠ ' : ''}${formatContextK(model.contextWindow)} · ${copy.source[model.contextSource]}`
  const save = async () => {
    if (!admin || pending || (draft.mode === 'custom' && value === null)) return
    setPending(true); setError('')
    try { await onSave(draft.mode === 'custom' ? value : null); setOpen(false); trigger.current?.focus() }
    catch (err) { setError(err instanceof Error ? err.message : String(err)) }
    finally { setPending(false) }
  }
  return <div ref={root} {...stylex.props(sx.root, open && sx.open)}>
    <button ref={trigger} type="button" data-context-trigger aria-label={`${label}: ${text}`} aria-haspopup="dialog" aria-controls={open ? id : undefined} aria-expanded={open} title={text} {...stylex.props(sx.trigger, fallback && sx.fallback, open && sx.focused)} onClick={() => {
      if (open) close()
      else { setDraft(contextDraft(model)); setError(''); setOpen(true) }
    }}>{fallback && <Icon name="warning-2" size={14} />}<span {...stylex.props(sx.number)}>{formatContextK(model.contextWindow)}</span><span {...stylex.props(sx.value)}>{` · ${copy.source[model.contextSource]}`}</span><Icon name="arrow-down-2" size={15} {...stylex.props(sx.arrow)} /></button>
    {open && <div ref={dialog} id={id} role="dialog" aria-label={copy.title(model.name)} data-placement={upward ? 'top' : 'bottom'} {...stylex.props(sx.pop, upward && sx.up)} onKeyDown={event => { if (event.key === 'Escape') close() }}>
      <div {...stylex.props(sx.intro)}><span {...stylex.props(sx.heading)}>{copy.title(model.name)}</span><p {...stylex.props(sx.small)}>{copy.description}</p></div>
      {fallback && <p {...stylex.props(sx.small, sx.warning)}><Icon name="warning-2" size={15} />{copy.warning(formatContextK(model.automaticWindow))}</p>}
      <ModelContextChoices model={model} draft={draft} onChange={setDraft} disabled={!admin || pending} variant="settings" />
      {error && <p role="alert" {...stylex.props(sx.small, sx.warning)}>{error}</p>}
      <div {...stylex.props(sx.footer)}><span {...stylex.props(sx.applies)}>{copy.applies}</span><button type="button" disabled={pending} {...stylex.props(sx.button)} onClick={close}>{copy.cancel}</button>{admin && <button type="button" disabled={pending || (draft.mode === 'custom' && value === null)} {...stylex.props(sx.button, sx.save)} onClick={() => void save()}>{pending ? copy.saving : copy.save}</button>}</div>
    </div>}
  </div>
}
