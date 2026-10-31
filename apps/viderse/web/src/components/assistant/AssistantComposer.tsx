// Ô Nhắn trợ lý dùng ở Bàn dựng (một video), Bản tin (mọi kênh) và Kênh. Gửi một
// câu → thẻ Kế hoạch ngay trên ô nhập (chưa chạy gì); chỉ nút Làm mới chạy, và
// chỉ các bước còn tích. Esc/Huỷ đóng thẻ không chạy gì; Sửa câu trả câu vào ô.
import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import type { AssistantScope } from '../../api/assistant'
import { useOnline } from '../../data/status'
import { assistantCopy as copy } from '../../i18n/assistant'
import { Orb } from '../Orb'
import { assistantName, type Assistant } from '../agents'
import { AskCard, Bubble, ConfirmCard, ErrorCard, PlanCard, PlanningCard, UnknownCard } from './AssistantCards'
import { isSingleJobStep, readDraft, textRunes } from './planModel'
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
}

export function AssistantComposer({ scope, label, placeholder, recipient, hints, onSettled }: Props) {
  const online = useOnline()
  const { thread, send, choose, toggle, confirm, cancel, edit, retry, dismiss, busy } = useAssistantThread(scope, onSettled)
  const [draft, setDraft] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const inputId = useId()
  const phase = thread.phase
  const reading = readDraft(draft)
  const tooLong = !reading.ok && reading.reason === 'tooLong'
  const canSend = reading.ok && !busy && online
  const cardOpen = phase === 'plan'

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

  return <div {...stylex.props(styles.dock)} onKeyDown={keyDown}>
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
    <form aria-label={label} onSubmit={submit} {...stylex.props(styles.form)}>
      <Icon name="msg" size={18} xstyle={styles.formIcon} />
      <label htmlFor={inputId} className="sr-only">{copy.composer.workbench}</label>
      <input ref={input} id={inputId} type="text" autoComplete="off" placeholder={placeholder} value={draft} aria-invalid={tooLong || undefined}
        onChange={(event) => setDraft(event.target.value)} {...stylex.props(styles.input)} />
      {recipient && <span {...stylex.props(styles.consult)}><Orb of={recipient} size={12} />{copy.composer.consult(assistantName(recipient))}</span>}
      <button type="submit" aria-label={copy.composer.send} aria-disabled={!canSend || undefined} title={!online ? copy.states.offlineSend : undefined}
        onClick={(event) => { if (!canSend) event.preventDefault() }} {...stylex.props(styles.send, !canSend && styles.sendOff)}><Icon name="arrow-up" size={20} /></button>
    </form>
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
