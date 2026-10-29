// Bắt đầu nhanh (NewStory.dc.html): phần mở đầu và nội dung nằm trong shell chung
// để ModeSwitch không bị tháo/lắp khi đổi giữa Nhanh và Đồng sáng tác.
import type { CSSProperties } from 'react'
import { getConfigPresets } from '../../api/client'
import type { ConfigResponse } from '../../api/types'
import { useResource } from '../../data'
import { findPack, packLabel } from '../settings/lib/packs'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { AgentOrb } from '@noverse/ui'
import { Button, Kbd } from '@noverse/ui'
import { TONE } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
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

const chipStyle: CSSProperties = {
  height: 30, padding: '0 10px 0 8px', display: 'flex', alignItems: 'center', gap: 6,
  borderRadius: 8, border: '1px solid var(--line-3)', fontSize: 12, color: 'var(--color-fg-3)',
  background: 'transparent', textDecoration: 'none',
}

function ConfigChip({ icon, label, value, href, mono, badge }: {
  icon: string; label?: string; value: string; href: string; mono?: boolean
  /** D2 — gói thể loại đang chọn: badge “có chấm” cạnh tên gói (NewStory.dc.html). */
  badge?: string
}) {
  return (
    <a className={`hov-ghost${mono ? ' mono' : ''}`} href={`#${href}`} style={chipStyle} title={label}>
      <Icon name={icon} size={15} />
      {label && <span>{label}</span>}
      <span style={{ color: 'var(--color-fg)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 160 }}>{value}</span>
      {badge && (
        <span style={{
          height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'inline-flex', alignItems: 'center',
          background: 'var(--tint-ok)', color: 'var(--color-ok)', whiteSpace: 'nowrap', flex: '0 0 auto',
        }}>{badge}</span>
      )}
    </a>
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

export function QuickMode({ prompt, onPrompt, onStart, busy, pending, success, alternativePending, activeAlternative, error, projectName, offline, config, idea, onClearIdea, onStartFile, onImport, onSimulate, project, gate }: {
  prompt: string
  onPrompt: (value: string) => void
  onStart: () => void
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
  const style = effective?.style || '—'
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = findPack(presets.data?.packs, effective?.style ?? '')
  const model = effective?.roles?.default?.model || effective?.model || '—'
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
            <ConfigChip icon="brush-2" label="Văn phong" value={pack ? packLabel(pack) : style}
              badge={pack ? 'có chấm' : undefined} href={paths.settings('writing')} />
            <ConfigChip icon="cpu" value={model} href={paths.settings('roles')} mono />
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
