// Ô Nhắn trợ lý dùng ở Bàn dựng (một video), Bản tin (mọi kênh) và Kênh. Gửi một
// câu → thẻ Kế hoạch ngay trên ô nhập (chưa chạy gì); chỉ nút Làm mới chạy, và
// chỉ các bước còn tích. Esc/Huỷ đóng thẻ không chạy gì; Sửa câu trả câu vào ô.
// Gõ `/` ở ô của một video mở danh sách lệnh (↑↓ chọn, Enter điền, Esc đóng).
import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { Icon } from '@noverse/ui'
import type { AssistantScope } from '../../api/assistant'
import { useOnline } from '../../data/status'
import { assistantCopy as copy } from '../../i18n/assistant'
import { Orb } from '../Orb'
import { assistantName, type Assistant } from '../agents'
import { AskCard, Bubble, ConfirmCard, ErrorCard, PlanCard, PlanningCard, UnknownCard } from './AssistantCards'
import { isSingleJobStep, readDraft, textRunes } from './planModel'
import { filterSlash, isComplete, slashQuery, type SlashCommand } from './slashCommands'
import { MAX_TEXT_RUNES } from '../../api/assistant'
import { ImageQuota } from './ImageQuota'
import { useAssistantThread } from './useAssistantThread'
import { assistantStyles as styles } from './assistant.stylex'

type Props = {
  scope: AssistantScope
  /** Nhãn trợ lý của biểu mẫu (aria-label) — artboard `cmpLabel`. */
  label: string
  placeholder: string
  /** Trợ lý nhận câu (chip “Bàn với Trinh sát” của ô ở Kênh) — chỉ nhãn người nhận, không phải nút. */
  recipient?: Assistant
  /** Hàng phím tắt dưới ô. `esc`: video đang chạy nên Esc tạm dừng; `panels`: màn có `[` `]` đóng/mở danh sách và bảng bên; `quota`: số ảnh hôm nay. */
  hints?: { esc?: boolean; panels?: boolean; quota?: boolean }
  /** Chạy xong/ bắt đầu chạy: làm mới dữ liệu màn đang mở. */
  onSettled?: () => void
  /** Lệnh `/` hợp lệ với video đang mở (`slashCommandsFor`). Trống hoặc bỏ qua: không có danh sách lệnh. */
  commands?: readonly SlashCommand[]
}

export function AssistantComposer({ scope, label, placeholder, recipient, hints, onSettled, commands }: Props) {
  const online = useOnline()
  const { thread, send, choose, toggle, confirm, cancel, edit, retry, dismiss, busy } = useAssistantThread(scope, onSettled)
  const [draft, setDraft] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const inputId = useId()
  const listId = useId()
  const [active, setActive] = useState(0)
  // Esc đóng danh sách lệnh cho đúng chữ đang gõ; gõ tiếp thì mở lại.
  const [closedFor, setClosedFor] = useState<string | null>(null)
  const phase = thread.phase
  const reading = readDraft(draft)
  const tooLong = !reading.ok && reading.reason === 'tooLong'
  const canSend = reading.ok && !busy && online
  const cardOpen = phase === 'plan'
  const query = commands?.length ? slashQuery(draft) : null
  const matches = query === null || !commands ? [] : filterSlash(commands, query)
  const listOpen = matches.length > 0 && closedFor !== draft
  const activeIndex = Math.min(active, matches.length - 1)
  const optionId = (index: number) => `${listId}-${index}`

  // Đổi phạm vi (sang video/kênh khác) thì bỏ thẻ đang mở: kế hoạch thuộc video cũ.
  const scopeKey = typeof scope === 'string' ? scope : 'jobId' in scope ? `j:${scope.jobId}` : `c:${scope.channelId}`
  const lastScope = useRef(scopeKey)
  useEffect(() => {
    if (lastScope.current === scopeKey) return
    lastScope.current = scopeKey
    dismiss()
  }, [scopeKey, dismiss])

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!canSend) return
    if (send(draft)) setDraft('')
  }

  const editText = () => {
    const text = edit()
    if (text === undefined) return
    setDraft(text)
    input.current?.focus()
  }

  const pick = (command: SlashCommand) => {
    setDraft(`/${command.id} `)
    setActive(0)
    input.current?.focus()
  }

  const inputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (!listOpen || event.nativeEvent.isComposing) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const step = event.key === 'ArrowDown' ? 1 : -1
      setActive((activeIndex + step + matches.length) % matches.length)
    } else if (event.key === 'Enter' && !isComplete(matches[activeIndex], draft)) {
      event.preventDefault()
      event.stopPropagation()
      pick(matches[activeIndex])
    } else if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      setClosedFor(draft)
    }
  }

  const keyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape' && phase !== 'idle' && phase !== 'running') {
      event.preventDefault()
      event.stopPropagation()
      cancel()
    } else if (event.key === 'Enter' && event.target === input.current && cardOpen && draft === '' && thread.phase === 'plan' && isSingleJobStep(thread.plan, scope) && online) {
      // “Làm Enter” của thẻ xác nhận: ô nhập trống thì Enter đồng ý.
      event.preventDefault()
      confirm()
    }
  }

  const single = thread.phase === 'plan' && isSingleJobStep(thread.plan, scope)
  const showBubble = phase === 'planning' || phase === 'ask' || phase === 'unknown' || phase === 'error' || single

  return <div className={`vv-composer-dock ${stylex.props(styles.dock).className}`} data-toast-anchor onKeyDown={keyDown}>
    {phase !== 'idle' && <div {...stylex.props(styles.thread)}>
      {showBubble && 'text' in thread && <Bubble text={thread.text} />}
      {thread.phase === 'planning' && <PlanningCard />}
      {thread.phase === 'plan' && (single
        ? <ConfirmCard step={thread.plan.steps[0]} imagesLeftToday={thread.plan.imagesLeftToday} online={online} onCancel={cancel} onRun={confirm} />
        : <PlanCard plan={thread.plan} text={thread.text} selected={thread.selected} mode="review" online={online} onToggle={toggle} onEdit={editText} onCancel={cancel} onRun={confirm} onClose={dismiss} />)}
      {thread.phase === 'ask' && <AskCard text={thread.text} ask={thread.ask} online={online} onChoose={choose} />}
      {thread.phase === 'unknown' && <UnknownCard unknown={thread.unknown} online={online} onChoose={choose} />}
      {thread.phase === 'error' && <ErrorCard kind={thread.kind} message={thread.message} onRetry={retry} onEdit={editText} onClose={dismiss} />}
      {(thread.phase === 'running' || thread.phase === 'done') && <PlanCard plan={thread.plan} text={thread.text} selected={thread.selected} mode={thread.phase}
        progress={thread.steps} lost={thread.phase === 'running' ? thread.lost : false} online={online} onToggle={toggle} onEdit={editText} onCancel={cancel} onRun={confirm} onClose={dismiss} />}
    </div>}
    <div {...stylex.props(styles.ringBox)}>
      {listOpen && <ul id={listId} role="listbox" aria-label={copy.slash.label} {...stylex.props(styles.slash)}>
        {matches.map((command, index) => <li key={command.id} id={optionId(index)} role="option" aria-selected={index === activeIndex}
          onMouseDown={(event) => event.preventDefault()} onMouseMove={() => { if (index !== activeIndex) setActive(index) }} onClick={() => pick(command)}
          {...stylex.props(styles.slashRow, index === activeIndex && styles.slashRowOn)}>
          <span {...stylex.props(styles.slashUsage)}>{command.usage}</span><span>{command.text}</span>
        </li>)}
      </ul>}
      <form aria-label={label} onSubmit={submit} {...stylex.props(styles.form)}>
        <Icon name="msg" size={18} xstyle={styles.formIcon} />
        <label htmlFor={inputId} {...stylex.props(text.srOnly)}>{label}</label>
        <input ref={input} id={inputId} type="text" autoComplete="off" placeholder={placeholder} value={draft} aria-invalid={tooLong || undefined}
          role={commands?.length ? 'combobox' : undefined} aria-expanded={commands?.length ? listOpen : undefined} aria-controls={listOpen ? listId : undefined}
          aria-autocomplete={commands?.length ? 'list' : undefined} aria-activedescendant={listOpen ? optionId(activeIndex) : undefined}
          onChange={(event) => { setDraft(event.target.value); setActive(0) }} onKeyDown={inputKeyDown} {...stylex.props(styles.input)} />
        {recipient && <span {...stylex.props(styles.consult)}><Orb of={recipient} size={12} />{copy.composer.consult(assistantName(recipient))}</span>}
        <button type="submit" aria-label={copy.composer.send} aria-disabled={!canSend || undefined} title={!online ? copy.states.offlineSend : undefined}
          onClick={(event) => { if (!canSend) event.preventDefault() }} {...stylex.props(styles.send, !canSend && styles.sendOff)}><Icon name="arrow-up" size={20} /></button>
      </form>
    </div>
    {(hints || tooLong || !online) && <div {...stylex.props(styles.hints)}>
      {hints && <span {...stylex.props(styles.hint)}><kbd {...stylex.props(styles.kbd)}>Enter</kbd>{copy.composer.hintEnter}</span>}
      {hints?.esc && <span {...stylex.props(styles.hint)}><kbd {...stylex.props(styles.kbd)}>Esc</kbd>{copy.composer.hintEsc}</span>}
      {hints?.panels && <span {...stylex.props(styles.hint)}><kbd {...stylex.props(styles.kbd)}>[ ]</kbd>{copy.composer.hintPanels}</span>}
      {tooLong && <span role="alert">{copy.states.tooLong} ({textRunes(draft)}/{MAX_TEXT_RUNES})</span>}
      {!online && <span role="status">{copy.states.offline}</span>}
      {hints?.quota && <><span {...stylex.props(styles.grow)} /><ImageQuota /></>}
    </div>}
  </div>
}
