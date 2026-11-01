// M11 › Bắt đầu nhanh (MNewStory.dc.html, tweak `state`). Cùng dữ liệu và cùng `disabledReason`
// với bản máy tính; chỉ khác bố cục: ô chữ 16 px, chip mở ngăn dưới, nút dính đáy nhảy lên
// khi bàn phím mở. Không gợi ý phím tắt, không nhắc Enter — điện thoại không có bàn phím vật lý.
import { useId } from 'react'
import * as stylex from '@stylexjs/stylex'
import { AgentOrb, Icon, Spinner } from '@noverse/ui'
import { getConfigPresets } from '../../../api/client'
import type { ConfigResponse } from '../../../api/types'
import { useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { findPack, packLabel } from '../../settings/lib/packs'
import { styleDisplayName } from '../../settings/sections/style'
import type { QuickStoryChoices } from '../creation'
import { startBlock, startBlockedReason } from '../creation'
import { FLOW } from '../QuickMode'
import { StoryCheckBlock } from '../StoryCheck'
import type { StoryCheck } from '../useStoryCheck'
import { quickChrome } from './layout'
import { styles } from './styles'

/** Chiều cao ô yêu cầu theo tweak `state` của artboard. */
const TEXTAREA_HEIGHT = { typing: 150, check: 232, idle: 124 } as const

export type QuickAlternative = 'startFile' | 'import' | 'simulate' | null

export function QuickPane({ project, gate, prompt, onPrompt, choices, config, idea, onClearIdea, offline, error, busy, starting, alternativePending, alternative, keyboard, onOpenStyle, onOpenWriter, onStart, onStartFile, onImport, onSimulate }: {
  project: string
  gate: StoryCheck
  prompt: string
  onPrompt: (value: string) => void
  choices: QuickStoryChoices
  config: ConfigResponse | null
  idea: { id: string; title: string } | null
  onClearIdea: () => void
  offline: boolean
  error: { message: string; detail: string } | null
  busy: boolean
  starting: boolean
  alternativePending: boolean
  alternative: QuickAlternative
  keyboard: number
  onOpenStyle: () => void
  onOpenWriter: () => void
  onStart: () => void
  onStartFile: () => void
  onImport: () => void
  onSimulate: () => void
}) {
  const promptId = useId()
  const effective = config?.effective
  const projectStyle = effective?.style || 'default'
  const style = choices.style || projectStyle
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = findPack(presets.data?.packs, style)
  const projectModel = effective?.roles?.writer?.model || effective?.roles?.default?.model || effective?.model || '—'
  const model = choices.writer?.model || projectModel

  const chrome = quickChrome({ keyboard, check: !!gate.state })
  const block = startBlock({ offline, prompt, blocked: gate.blocked })
  const reason = startBlockedReason(block, 'phone')
  const height = chrome.barOffset > 0 ? TEXTAREA_HEIGHT.typing : gate.state ? TEXTAREA_HEIGHT.check : TEXTAREA_HEIGHT.idle
  const canStart = !busy && !reason
  const label = error ? t.newStory.simulateRetry : starting ? t.newStory.starting : gate.checking ? t.newStory.comparingBar : t.newStory.start
  const why = starting ? t.newStory.startingDetail
    : alternativePending ? (alternative === 'import' ? t.newStory.creatingImport : t.newStory.simulateCreating)
      : reason
  const tone = starting || gate.checking || alternativePending ? styles.waiting
    : gate.blocked ? styles.locked
      : canStart ? styles.primary : styles.off

  return (
    <div {...stylex.props(styles.body)}>
      <div {...stylex.props(styles.scroll, styles.scrollPad(168))}>
        {chrome.heading && <div {...stylex.props(styles.heading)}>
          {chrome.orbs && <div aria-hidden="true" {...stylex.props(styles.orbRow)}>
            <AgentOrb agent="architect" size={26} />
            <AgentOrb agent="writer" size={32} style={{ marginLeft: -8 }} />
            <AgentOrb agent="editor" size={26} style={{ marginLeft: -8 }} />
            <AgentOrb agent="arbiter" size={22} style={{ marginLeft: -8 }} />
          </div>}
          <h1 {...stylex.props(styles.h1)}>{t.newStory.title}</h1>
          {chrome.subtitle && <p {...stylex.props(styles.subtitle)}>{t.newStory.subtitle}</p>}
        </div>}

        {error && <div role="alert" {...stylex.props(styles.error)}>
          <Icon name="warning-2" size={18} {...stylex.props(styles.errorIcon)} />
          <span {...stylex.props(styles.errorCopy)}>
            <span {...stylex.props(styles.errorTitle)}>{t.placeholder.initFailed}</span>
            <span {...stylex.props(styles.errorDetail)}>{error.detail || error.message}</span>
          </span>
        </div>}

        <form aria-label={t.newStory.promptForm} onSubmit={(event) => { event.preventDefault(); if (canStart) onStart() }}
          {...stylex.props(styles.card, chrome.barOffset > 0 && styles.cardTyping)}>
          {idea && <div {...stylex.props(styles.ideaRow)}>
            <Icon name="magic-star" size={15} {...stylex.props(styles.ideaIcon)} />
            <span>Từ ý tưởng:</span>
            <span {...stylex.props(styles.ideaTitle)}>{idea.title}</span>
            <button type="button" onClick={onClearIdea} aria-label="Bỏ liên kết ý tưởng" {...stylex.props(styles.ideaClear)}>
              <Icon name="close-circle-bulk" size={15} />
            </button>
          </div>}
          <label htmlFor={promptId} {...stylex.props(styles.srOnly)}>Yêu cầu sáng tác truyện</label>
          <textarea id={promptId} value={prompt} maxLength={5000} placeholder={t.newStory.promptPhone}
            onChange={(event) => onPrompt(event.target.value)}
            {...stylex.props(styles.textarea, styles.textareaHeight(height))} />
          <div {...stylex.props(styles.chips)}>
            <button type="button" onClick={onOpenStyle} disabled={busy || !config} aria-haspopup="dialog" {...stylex.props(styles.chip)}>
              <Icon name="brush-2" size={14} />
              <span>Văn phong</span>
              <span {...stylex.props(styles.chipValue)}>{pack ? packLabel(pack) : styleDisplayName(presets.data ?? null, style)}</span>
              {pack && <span {...stylex.props(styles.badge)}>có chấm</span>}
              <Icon name="arrow-down" size={12} {...stylex.props(styles.chevron)} />
            </button>
            <button type="button" onClick={onOpenWriter} disabled={busy || !config} aria-haspopup="dialog" {...stylex.props(styles.chip)}>
              <Icon name="cpu" size={14} />
              <span {...stylex.props(styles.chipValue, styles.chipMono)}>{model}</span>
              <Icon name="arrow-down" size={12} {...stylex.props(styles.chevron)} />
            </button>
            <span {...stylex.props(styles.count)}>{prompt.length}/5000</span>
          </div>
        </form>

        {gate.state && <div {...stylex.props(styles.checkWrap)}>
          <StoryCheckBlock project={project} gate={gate} columns={2} onEdit={() => document.getElementById(promptId)?.focus()} />
        </div>}

        {chrome.extras && <>
          <div {...stylex.props(styles.groupHead)}>{t.newStory.tryIdea}</div>
          <div {...stylex.props(styles.exampleList)}>
            {t.newStory.examples.map((example) => <button key={example} type="button" onClick={() => onPrompt(example)} {...stylex.props(styles.example)}>
              {example}
            </button>)}
          </div>

          <div {...stylex.props(styles.groupHead)}>{t.newStory.whatHappens}</div>
          <div {...stylex.props(styles.group)}>
            {FLOW.map((step, index) => <div key={step.text} {...stylex.props(styles.flowRow, index > 0 && styles.divider)}>
              <AgentOrb agent={step.orb} size={16} />
              <span>{step.text}</span>
            </div>)}
          </div>

          <div {...stylex.props(styles.groupHead)}>{t.newStory.altTitle}</div>
          <div {...stylex.props(styles.group)}>
            <AltRow icon="document-text" title={t.newStory.altStart} hint={t.newStory.altStartHint} onClick={onStartFile}
              busy={busy} pending={alternativePending && alternative === 'startFile'} dim={alternativePending && alternative !== 'startFile'} />
            <AltRow icon="folder-open" title={t.newStory.altImport} hint={t.newStory.altImportHint} onClick={onImport}
              busy={busy} pending={alternativePending && alternative === 'import'} dim={alternativePending && alternative !== 'import'} divider />
            <AltRow icon="brush-2" title={t.newStory.altSimulate} hint={t.newStory.altSimulateHint} onClick={onSimulate}
              busy={busy} pending={alternativePending && alternative === 'simulate'} dim={alternativePending && alternative !== 'simulate'} divider />
          </div>
        </>}
      </div>

      <div {...stylex.props(styles.bar, styles.barOffset(chrome.barOffset), chrome.barOffset > 0 && styles.barTyping)}>
        {why && <span role={starting || alternativePending ? 'status' : undefined} {...stylex.props(styles.why)}>{why}</span>}
        <button type="button" onClick={() => { if (canStart) onStart() }} disabled={!canStart} aria-disabled={!canStart}
          aria-busy={starting || gate.checking || undefined} {...stylex.props(styles.button, tone)}>
          {starting || gate.checking ? <Spinner size={15} /> : <Icon name={gate.blocked ? 'lock' : 'arrow-up'} size={gate.blocked ? 15 : 18} />}
          {label}
        </button>
      </div>
    </div>
  )
}

function AltRow({ icon, title, hint, onClick, busy, pending, dim, divider }: {
  icon: string
  title: string
  hint: string
  onClick: () => void
  busy: boolean
  pending: boolean
  dim: boolean
  divider?: boolean
}) {
  return <button type="button" onClick={onClick} disabled={busy} aria-busy={pending || undefined}
    {...stylex.props(styles.altRow, divider && styles.divider, dim && styles.dim)}>
    {pending ? <Spinner size={18} /> : <Icon name={icon} size={18} {...stylex.props(styles.altIcon)} />}
    <span {...stylex.props(styles.altCopy)}>
      <span>{title}</span>
      <span {...stylex.props(styles.altSub)}>{hint}</span>
    </span>
    <Icon name="arrow-right-2" size={14} {...stylex.props(styles.chevron)} />
  </button>
}
