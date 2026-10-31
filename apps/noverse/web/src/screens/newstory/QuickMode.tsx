// Bắt đầu nhanh (NewStory.dc.html): phần mở đầu và nội dung nằm trong shell chung
// để ModeSwitch không bị tháo/lắp khi đổi giữa Nhanh và Đồng sáng tác.
import { useId, useLayoutEffect, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, tint } from '@noverse/ui/tokens.stylex'
import { getConfigPresets } from '../../api/client'
import type { ConfigResponse } from '../../api/types'
import { useResource } from '../../data'
import { findPack, packLabel } from '../settings/lib/packs'
import { modelOptions } from '../settings/lib/config'
import { styleDisplayName } from '../settings/sections/style'
import { StyleQuickPicker, WriterQuickPicker } from '../settings/sections/quick-pickers'
import type { QuickStoryChoices } from './creation'
import { amWritingCopy, t } from '../../i18n/vi'
import { paths } from '../../router'
import { AgentOrb } from '@noverse/ui'
import { Button, Kbd } from '@noverse/ui'
import { TONE } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { Spinner, usePopup } from '@noverse/ui'
import type { StoryMode } from './ModeSwitch'
import { StoryCheckBlock } from './StoryCheck'
import type { StoryCheck } from './useStoryCheck'

const FLOAT = [
  { agent: 'architect', size: 34, delay: '0s' },
  { agent: 'writer', size: 42, delay: '1.5s' },
  { agent: 'editor', size: 34, delay: '3s' },
  { agent: 'arbiter', size: 28, delay: '4.5s' },
] as const

const FLOW = [
  { orb: 'arbiter', text: 'Trọng tài chọn truyện ngắn hay dài' },
  { orb: 'architect', text: 'Kiến trúc sư so với truyện trước, lập đại cương' },
  { orb: 'writer', text: 'Người viết viết từng chương' },
  { orb: 'editor', text: 'Biên tập viên thẩm duyệt cuối mỗi hồi' },
] as const

const sx = stylex.create({
  chipRoot: { position: 'relative', display: 'flex', minWidth: 0, maxWidth: '100%' },
  chip: {
    height: 30, padding: '0 8px', display: 'flex', alignItems: 'center', gap: 6,
    borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'],
    fontSize: 12, color: color['--color-fg-3'], backgroundColor: 'transparent',
    cursor: 'pointer', whiteSpace: 'nowrap', minWidth: 0, fontFamily: 'inherit',
    ':hover': { backgroundColor: kit['--kit-acc-6'] },
  },
  chipOpen: { borderColor: kit['--kit-acc-45'], backgroundColor: kit['--kit-acc-6'] },
  value: { color: color['--color-fg'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 160 },
  badge: {
    height: 18, padding: '0 6px', borderRadius: 5, fontSize: 11, display: 'inline-flex', alignItems: 'center',
    backgroundColor: tint['--tint-ok'], color: color['--color-ok'], flexShrink: 0,
  },
  arrow: { color: color['--color-fg-4'], flexShrink: 0 },
  popover: {
    position: 'absolute', zIndex: 60, left: 0, top: 'calc(100% + 6px)', width: 320,
    maxWidth: 'calc(100vw - 40px)', maxHeight: 400, overflowY: 'auto',
    borderRadius: 14, backgroundColor: color['--color-surface-4'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    boxShadow: kit['--kit-shadow-pop'],
    scrollbarWidth: 'thin',
  },
  writerPopover: { width: 300 },
  popoverOffset: (left: number) => ({ left }),
  footer: {
    display: 'block', marginTop: 4, padding: '9px 10px 5px', borderTopWidth: 1, borderTopStyle: 'solid',
    borderTopColor: line['--line-1'], fontSize: 12.5, color: color['--color-accent'], textDecoration: 'none',
  },
})

function ConfigChip({ icon, label, value, mono, badge, disabled, pickerLabel, children }: {
  icon: string; label?: string; value: string; mono?: boolean; badge?: string
  disabled: boolean; pickerLabel: string; children: (close: () => void) => ReactNode
}) {
  const popup = usePopup(disabled)
  const id = useId()
  const [left, setLeft] = useState(0)
  useLayoutEffect(() => {
    if (!popup.open) return
    const align = () => {
      const anchor = popup.root.current?.getBoundingClientRect()
      if (!anchor) return
      const width = Math.min(mono ? 300 : 320, window.innerWidth - 40)
      setLeft(Math.max(20 - anchor.left, Math.min(0, window.innerWidth - 20 - anchor.left - width)))
    }
    align()
    window.addEventListener('resize', align)
    return () => window.removeEventListener('resize', align)
  }, [popup.open, popup.root, mono])
  return (
    <div ref={popup.root} {...stylex.props(sx.chipRoot)}>
      <button type="button" {...stylex.props(sx.chip, popup.open && sx.chipOpen)}
        aria-label={`${label || 'Người viết'}: ${value}`} aria-haspopup="listbox" aria-expanded={popup.open}
        aria-controls={popup.open ? id : undefined} disabled={disabled} onClick={() => popup.setOpen(!popup.open)} title={value}>
        <Icon name={icon} size={15} />
        {label && <span>{label}</span>}
        <span {...stylex.props(sx.value)}><span className={mono ? 'mono' : undefined}>{value}</span></span>
        {badge && <span {...stylex.props(sx.badge)}>{badge}</span>}
        <Icon name="arrow-down-2" size={13} {...stylex.props(sx.arrow)} />
      </button>
      {popup.open && <div id={id} aria-label={pickerLabel}
        {...stylex.props(sx.popover, mono && sx.writerPopover, sx.popoverOffset(left))}>
        {children(() => popup.setOpen(false))}
      </div>}
    </div>
  )
}

function AltEntry({ icon, title, hint, onClick, busy, pending }: {
  icon: string; title: string; hint: string; onClick: () => void; busy: boolean; pending?: boolean
}) {
  return (
    <button type="button" className="hov-ghost" onClick={onClick} disabled={busy} aria-busy={pending || undefined}
      style={{
        display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12,
        border: '1px dashed rgba(255,255,255,0.12)', background: 'transparent', color: 'var(--color-fg-2)',
        textAlign: 'left', width: '100%',
      }}>
      {pending ? <Spinner size={18} /> : <Icon name={icon} size={18} style={{ color: 'var(--color-fg-4)' }} />}
      <span style={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <span style={{ fontSize: 13 }}>{title}</span>
        <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)', overflowWrap: 'anywhere' }}>{hint}</span>
      </span>
    </button>
  )
}

export function QuickMode({ prompt, onPrompt, onStart, choices, onChoices, busy, pending, success, alternativePending, activeAlternative, error, projectName, offline, config, idea, onClearIdea, onStartFile, onImport, onSimulate, project, gate }: {
  prompt: string
  onPrompt: (value: string) => void
  onStart: () => void
  choices: QuickStoryChoices
  onChoices: (choices: QuickStoryChoices) => void
  busy: boolean
  pending: boolean
  success: boolean
  alternativePending: boolean
  activeAlternative: 'import' | 'simulate' | null
  error: { message: string; detail: string } | null
  /** Tên hiển thị của dự án sẽ chứa truyện mới (project.json.name). */
  projectName: string
  offline: boolean
  config: ConfigResponse | null
  idea: { id: string; title: string } | null
  onClearIdea: () => void
  onStartFile: () => void
  onImport: () => void
  onSimulate: () => void
  project: string
  gate: StoryCheck
}) {
  const effective = config?.effective
  const projectStyle = effective?.style || 'default'
  const style = choices.style || projectStyle
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = findPack(presets.data?.packs, style)
  const projectModel = effective?.roles?.writer?.model || effective?.roles?.default?.model || effective?.model || '—'
  const model = choices.writer?.model || projectModel
  const disabledReason = offline
    ? 'Mất kết nối — chờ kết nối lại để bắt đầu.'
    : !prompt.trim()
      ? 'Nhập ý tưởng hoặc yêu cầu sáng tác để bắt đầu.'
      : gate.blocked
        ? t.newStoryCheck.blockedHint
        : null

  const submit = () => { if (!busy && !disabledReason) onStart() }

  return (
    <>

        {error && (
          <div role="alert" style={{
            width: '100%', display: 'flex', alignItems: 'flex-start', gap: 8, padding: '10px 14px',
            borderRadius: 12, background: TONE.err.bg, border: `1px solid ${TONE.err.bd}`,
            color: 'var(--color-err)', fontSize: 13,
          }}>
            <Icon name="warning-2" size={16} style={{ marginTop: 1 }} />
            <span style={{ flex: 1 }}>
              <b style={{ fontWeight: 500 }}>{error.message}</b>
              {error.detail && <span style={{ display: 'block', color: 'var(--color-fg-3)', fontSize: 12 }}>{error.detail}</span>}
            </span>
          </div>
        )}

        <form className="card kit-frame" aria-label="Yêu cầu sáng tác"
          onSubmit={(event) => { event.preventDefault(); submit() }}
          style={{
            width: '100%', borderRadius: 20, padding: '14px 14px 10px',
            display: 'flex', flexDirection: 'column', gap: 10, background: 'var(--color-surface-3)',
          }}>
          {idea && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-fg-3)', padding: '0 4px' }}>
              <Icon name="magic-star-bulk" size={14} />
              Từ ý tưởng:
              <b style={{ fontWeight: 500, color: 'var(--color-fg)' }}>{idea.title || idea.id}</b>
              <button type="button" className="hov-ghost" onClick={onClearIdea} aria-label="Bỏ liên kết ý tưởng"
                style={{
                  width: 20, height: 20, borderRadius: '50%', border: 'none', background: 'transparent',
                  color: 'var(--color-fg-4)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                <Icon name="close-circle-bulk" size={13} />
              </button>
            </div>
          )}
          <label htmlFor="ns-prompt" className="sr-only">Yêu cầu sáng tác truyện</label>
          <TextArea id="ns-prompt" rows={4} maxLength={5000} showCount={false} value={prompt}
            onChange={(event) => onPrompt(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey && !event.metaKey && !event.ctrlKey && !event.altKey) {
                event.preventDefault()
                submit()
              }
            }}
            placeholder={t.placeholder.newStory}
            style={{
              resize: 'none', width: '100%', background: 'transparent', border: 'none', outline: 'none',
              color: 'var(--color-fg)', fontSize: 15, lineHeight: 1.65, padding: '2px 4px',
            }} />
          <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
            <ConfigChip icon="brush-2" label="Văn phong" value={pack ? packLabel(pack) : styleDisplayName(presets.data ?? null, style)}
              badge={pack ? 'có chấm' : undefined} disabled={busy || !config} pickerLabel={amWritingCopy.styleTitle}>
              {close => <StyleQuickPicker variant="newstory" value={choices.style} projectStyle={projectStyle} presets={presets.data ?? null}
                onPick={value => { onChoices({ ...choices, style: value === projectStyle ? null : value }); close() }}
                footer={<a {...stylex.props(sx.footer)} href={`#${paths.project(project)}?section=pstyle`}>{amWritingCopy.projectFooter}</a>} />}
            </ConfigChip>
            <ConfigChip icon="cpu" value={model} mono disabled={busy || !config} pickerLabel={amWritingCopy.writerTitle}>
              {close => <WriterQuickPicker variant="newstory" value={choices.writer} projectModel={projectModel}
                models={modelOptions(effective ?? {})} presets={presets.data ?? null}
                onPick={value => {
                  const inherited = value?.model === projectModel && value?.provider === (effective?.roles?.writer?.provider || effective?.roles?.default?.provider || effective?.provider)
                  onChoices({ ...choices, writer: inherited ? null : value }); close()
                }}
                footer={<a {...stylex.props(sx.footer)} href={`#${paths.settings('connect')}`}>{amWritingCopy.modelFooter}</a>} />}
            </ConfigChip>
            <span title="Truyện mới nằm trong dự án này" style={{
              height: 30, padding: '0 10px 0 8px', display: 'flex', alignItems: 'center', gap: 6,
              borderRadius: 8, border: '1px dashed rgba(255,255,255,0.12)', fontSize: 12, color: 'var(--color-fg-3)',
              maxWidth: '100%',
            }}>
              <Icon name="folder-open" size={15} />
              Lưu vào
              <span style={{ color: 'var(--color-fg)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {projectName}
              </span>
            </span>
            <span style={{ flex: 1 }} />
            <span className="num mono" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{prompt.length}/5000</span>
            <Button variant="primary" type="submit" disabled={busy || !!disabledReason} pending={pending && !gate.checking} success={success}
              style={gate.blocked || gate.checking ? { background: gate.checking ? 'var(--color-surface-6)' : 'var(--kit-surface-toast)', color: gate.checking ? 'var(--color-fg-3)' : 'var(--kit-fg-arrow)', border: '1px solid var(--line-3)', opacity: 1 } : undefined}
              aria-describedby={disabledReason && !gate.blocked ? 'ns-start-reason' : undefined} pendingLabel="Đang bắt đầu…"
              tooltip={disabledReason ?? 'Bắt đầu sáng tác · Enter / ⌘S'} icon={gate.checking ? undefined : <Icon name={gate.blocked ? 'lock' : 'arrow-up'} size={gate.blocked ? 15 : 16} />}>
              {gate.checking ? t.newStoryCheck.checking : t.newStory.start} {!gate.blocked && !gate.checking && <Kbd>Enter</Kbd>}
            </Button>
          </div>
          {disabledReason && !gate.blocked && <p id="ns-start-reason" style={{ margin: '0 4px', fontSize: 12, color: 'var(--color-fg-3)' }}>{disabledReason}</p>}
          {pending && !gate.checking && <p role="status" style={{ margin: '0 4px', fontSize: 12, color: 'var(--color-fg-3)' }}>Đang tạo truyện và gửi yêu cầu cho AI…</p>}
        </form>

        <StoryCheckBlock project={project} gate={gate} onEdit={() => document.getElementById('ns-prompt')?.focus()} />
        {!gate.state && <>
        <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 12, color: 'var(--color-fg-4)', padding: '0 4px' }}>{t.newStory.tryIdea}</div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(220px, 100%), 1fr))', gap: 8 }}>
            {t.newStory.examples.map((example) => (
              <button key={example} type="button" className="hov-row" onClick={() => onPrompt(example)}
                style={{
                  minHeight: 72, padding: '12px 14px', borderRadius: 14, border: '1px solid var(--line-2)',
                  background: 'var(--color-surface-1)', color: 'var(--color-fg-2)',
                  fontSize: 13, lineHeight: 1.5, textAlign: 'left',
                }}>
                {example}
              </button>
            ))}
          </div>
        </div>

        <div style={{
          width: '100%', display: 'flex', flexWrap: 'wrap', justifyContent: 'center',
          alignItems: 'center', gap: '6px 10px', fontSize: 12, color: 'var(--color-fg-4)',
        }}>
          <span>{t.newStory.whatHappens}</span>
          {FLOW.map((step, index) => (
            <span key={step.text} style={{ display: 'contents' }}>
              <span style={{
                display: 'flex', alignItems: 'center', gap: 6, height: 26, padding: '0 9px 0 6px',
                borderRadius: 999, background: 'var(--color-surface-1)', border: '1px solid var(--line-1)',
                color: 'var(--color-fg-2)',
              }}>
                <AgentOrb agent={step.orb} size={12} />
                {step.text}
              </span>
              {index < FLOW.length - 1 && <Icon name="arrow-right-2" size={12} />}
            </span>
          ))}
        </div>

        <div style={{ width: '100%', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(220px, 100%), 1fr))', gap: 8 }}>
          <AltEntry icon="document-text" title={t.newStory.altStart} hint={t.newStory.altStartHint} onClick={onStartFile} busy={busy} />
          <AltEntry icon="folder-open" title={t.newStory.altImport} hint={t.newStory.altImportHint} onClick={onImport} busy={busy} pending={alternativePending && activeAlternative === 'import'} />
          <AltEntry icon="brush-2" title={t.newStory.altSimulate} hint={t.newStory.altSimulateHint} onClick={onSimulate} busy={busy} pending={alternativePending && activeAlternative === 'simulate'} />
        </div>
        </>}
    </>
  )
}

export function QuickModeHeading({ checked = false }: { checked?: boolean }) {
  return <>
    <style>{'@keyframes ns-float{0%,100%{transform:translateY(0)}50%{transform:translateY(-5px)}}'}</style>
    {!checked && <div aria-hidden="true" style={{ display: 'flex', alignItems: 'center' }}>
      {FLOAT.map((orb, index) => (
        <AgentOrb key={orb.agent} agent={orb.agent} size={orb.size}
          style={{ marginLeft: index === 0 ? 0 : -8, animation: `ns-float 6s ease-in-out ${orb.delay} infinite` }} />
      ))}
    </div>}
    <div style={{ textAlign: 'center', display: 'flex', flexDirection: 'column', gap: 6 }}>
      <h1 style={{ margin: 0, fontSize: 30, fontWeight: 600, letterSpacing: '-0.025em' }}>{t.newStory.title}</h1>
      {!checked && <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-3)' }}>{t.newStory.subtitle}</p>}
    </div>
  </>
}

export function ShortcutHints({ mode }: { mode: StoryMode }) {
  const items: { kbd: string; text: string }[] = [
    { kbd: 'Tab', text: 'đổi chế độ' },
    { kbd: 'Enter', text: mode === 'quick' ? 'bắt đầu ngay' : 'gửi cho AI' },
    { kbd: '/', text: 'lệnh' },
  ]
  return (
    <div style={{
      flex: '0 0 auto', width: '100%', display: 'flex', justifyContent: 'center', gap: 14,
      padding: '8px 0 12px', fontSize: 12, color: 'var(--color-fg-4)',
    }}>
      {items.map((item) => (
        <span key={item.kbd} style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <Kbd>{item.kbd}</Kbd>{item.text}
        </span>
      ))}
    </div>
  )
}
