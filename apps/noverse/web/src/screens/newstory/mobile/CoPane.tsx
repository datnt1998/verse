// M11 › Đồng sáng tác (MNewStory.dc.html, mode `co`). Cùng phiên cấp dự án và cùng
// `cocreateDraft` với bản máy tính: đóng app mở lại vẫn còn bản nháp; chỉ đổi bố cục —
// hội thoại chiếm cả màn, thẻ “Bản nháp yêu cầu” nằm ở đáy và mở ngăn chi tiết.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { AgentOrb, Icon, Sheet, Skeleton, Spinner } from '@noverse/ui'
import { cancelProjectCoCreate, commitProjectCoCreate, getProjectCoCreate, sendProjectCoCreate } from '../../../api/client'
import type { CoCreateState } from '../../../api/types'
import { keys, setData, useAction, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { forgetProjectDraft, readProjectDraft, rememberProjectDraft } from '../cocreateDraft'
import { StoryCheckBlock } from '../StoryCheck'
import { useStoryCheck } from '../useStoryCheck'
import { draftCard, draftStage, draftStartable } from './layout'
import { styles } from './styles'

type Message = CoCreateState['messages'][number]

/** Ba chấm “AI đang nghĩ” — cùng nhịp `ai-twinkle` của bản máy tính. */
function Dots() {
  return <span aria-hidden="true" {...stylex.props(styles.dots)}>
    {['0s', '0.15s', '0.3s'].map((delay) => <span key={delay} {...stylex.props(styles.dot(delay))} />)}
  </span>
}

export function CoPane({ project, writable, keyboard, onStarted, onCancel }: {
  project: string
  /** Còn ghi được (mạng + quyền) — tắt thì nút Bắt đầu mờ, không mời bấm. */
  writable: boolean
  keyboard: number
  onStarted: (story: string) => void
  onCancel: () => void
}) {
  const [draftId, setDraftId] = useState<string | null>(() => readProjectDraft(project))
  const [messages, setMessages] = useState<Message[]>([])
  const [draft, setDraft] = useState('')
  const [ready, setReady] = useState(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [text, setText] = useState('')
  const [sheet, setSheet] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const gate = useStoryCheck(project, draft, setDraft)
  const restoring = !!draftId && messages.length === 0

  const resourceKey = keys.project(project, 'meta', 'cocreate', draftId)
  const resource = useResource(resourceKey, () => getProjectCoCreate(project, draftId!), { enabled: !!draftId })
  useEffect(() => {
    if (!resource.data) return
    setMessages(resource.data.messages)
    setDraft(resource.data.draft)
    setReady(resource.data.ready)
    setSuggestions(resource.data.suggestions)
  }, [resource.data])

  const sendAction = useAction(async (value: string) => {
    const reply = await sendProjectCoCreate(project, value, draftId ?? undefined)
    const nextDraftId = 'draftId' in reply && typeof reply.draftId === 'string' ? reply.draftId : draftId
    if (nextDraftId) {
      setDraftId(nextDraftId)
      rememberProjectDraft(project, nextDraftId)
    }
    const nextMessages: Message[] = [...messages, { role: 'user', content: value }, { role: 'assistant', content: reply.reply }]
    setMessages(nextMessages)
    setDraft(reply.draft)
    setReady(reply.ready)
    setSuggestions(reply.suggestions)
    setText('')
    setData(keys.project(project, 'meta', 'cocreate', nextDraftId), { messages: nextMessages, draft: reply.draft, ready: reply.ready, suggestions: reply.suggestions })
    return reply
  })

  const send = (raw?: string) => {
    const value = (raw ?? text).trim()
    if (value && writable) void sendAction.run(value)
  }

  const commitAction = useAction(async () => {
    if (!draftId) return
    if (!await gate.check()) return
    try {
      const result = await commitProjectCoCreate(project, draftId, undefined, draft)
      forgetProjectDraft(project)
      setDraftId(null)
      onStarted(result.story.story)
      return result.story.story
    } catch (error) {
      if (!gate.acceptConflict(error)) throw error
    }
  }, { invalidate: [keys.project(project), keys.library()] })

  const cancelAction = useAction(async () => {
    if (draftId) await cancelProjectCoCreate(project, draftId)
    forgetProjectDraft(project)
    setDraftId(null)
    setMessages([])
    setDraft('')
    setReady(false)
    setSuggestions([])
    setText('')
    setSheet(false)
    onCancel()
    return true
  }, { success: 'Đã huỷ đồng sáng tác', invalidate: [resourceKey] })

  const busy = sendAction.busy || commitAction.busy || cancelAction.busy || gate.checking || gate.suggesting
  const failure = sendAction.error || commitAction.error || cancelAction.error
  const error = failure instanceof Error ? failure.message : null
  const stage = draftStage({ draft, ready, busy, blocked: gate.blocked })
  const card = draftCard(stage, draft)
  const canStart = draftStartable(stage, writable) && !busy
  const start = () => { if (canStart) void commitAction.run() }
  const startLabel = gate.blocked ? t.newStory.blockedBar : writable ? t.newStory.keepDraftingPhone : 'Mất kết nối — chờ kết nối lại để bắt đầu.'
  const pill = stage === 'blocked' ? styles.pillErr : stage === 'ready' ? styles.pillOk : styles.pillNeutral
  const bottomOffset = keyboard > 0 ? Math.round(keyboard) : 0

  return (
    <div {...stylex.props(styles.body)}>
      <div {...stylex.props(styles.chat, styles.chatPad(196))}>
        <div {...stylex.props(styles.msgs)}>
          {restoring && resource.initialLoading && <Skeleton height={160} radius={12} />}
          {messages.length === 0 && !busy && <div {...stylex.props(styles.coEmpty)}>
            <AgentOrb agent="architect" size={36} />
            <span {...stylex.props(styles.coEmptyTitle)}>{t.newStory.coEmptyTitle}</span>
            <p {...stylex.props(styles.coEmptyText)}>{t.newStory.coEmptyText}</p>
          </div>}
          {messages.map((message, index) => message.role === 'user'
            ? <div key={`${index}-user`} {...stylex.props(styles.bubble)}>{message.content}</div>
            : <div key={`${index}-assistant`} {...stylex.props(styles.aim)}>
              <AgentOrb agent="architect" size={22} style={{ marginTop: 2 }} />
              <p {...stylex.props(styles.aimText)}>{message.content}</p>
            </div>)}
          {busy && <div {...stylex.props(styles.waitRow)}>
            <AgentOrb agent="architect" size={22} live />
            <span {...stylex.props(styles.waitText)}>{t.newStory.coThinking}<Dots /></span>
          </div>}
        </div>
      </div>

      <div {...stylex.props(styles.coBottom, styles.barOffset(bottomOffset), keyboard > 0 && styles.coBottomTyping)}>
        {error && <span role="alert" {...stylex.props(styles.errorDetail, styles.errorRow)}>{error}</span>}

        <div {...stylex.props(styles.peek)}>
          <button type="button" onClick={() => setSheet(true)} aria-haspopup="dialog" {...stylex.props(styles.peekButton)}>
            <span {...stylex.props(styles.peekHead)}>
              <Icon name="document-text" size={16} />
              <span {...stylex.props(styles.peekTitle)}>{t.newStory.draftTitle}</span>
              <Icon name="arrow-right-2" size={13} {...stylex.props(styles.chevron)} />
            </span>
            <span {...stylex.props(styles.peekLine)}>
              <span {...stylex.props(styles.pill, pill)}>{card.label}</span>
              <span {...stylex.props(styles.peekText)}>{card.line || t.newStory.draftEmpty}</span>
            </span>
          </button>
          <button type="button" onClick={start} disabled={!canStart} aria-disabled={!canStart} aria-busy={commitAction.busy || undefined}
            {...stylex.props(styles.smallStart, gate.blocked && styles.smallStartLocked, !canStart && !gate.blocked && styles.smallStartOff)}>
            {commitAction.busy ? <Spinner size={14} /> : gate.blocked ? <Icon name="lock" size={14} /> : null}
            {t.newStory.startShort}
          </button>
        </div>

        {suggestions.length > 0 && <div {...stylex.props(styles.sugRow)}>
          {suggestions.map((suggestion, index) => <button key={suggestion} type="button"
            onClick={() => { setText(suggestion); inputRef.current?.focus() }} {...stylex.props(styles.sug)}>
            <i {...stylex.props(styles.sugNum)}>{index + 1}</i>
            {suggestion}
          </button>)}
        </div>}

        <form onSubmit={(event) => { event.preventDefault(); send() }} {...stylex.props(styles.composer)}>
          <Icon name="messages-2" size={18} {...stylex.props(styles.altIcon)} />
          <label htmlFor="m11-cochat" {...stylex.props(styles.srOnly)}>Trao đổi với AI</label>
          <input id="m11-cochat" ref={inputRef} type="text" value={text} maxLength={5000} enterKeyHint="send"
            onChange={(event) => setText(event.target.value)} placeholder={t.newStory.coPlaceholder} {...stylex.props(styles.composerInput)} />
          <button type="submit" aria-label="Gửi" disabled={busy || !writable || !text.trim()} {...stylex.props(styles.send, (!text.trim() || busy) && styles.sendOff)}>
            <Icon name="arrow-up" size={18} />
          </button>
        </form>
      </div>

      <Sheet open={sheet} onClose={() => setSheet(false)} label={t.newStory.draftTitle}>
        <div {...stylex.props(styles.sheetBody)}>
          <div {...stylex.props(styles.sheetHead)}>
            <span {...stylex.props(styles.sheetTitle)}>{t.newStory.draftTitle}</span>
            <button type="button" onClick={() => setSheet(false)} {...stylex.props(styles.sheetClose)}>{t.newStory.sheetClose}</button>
          </div>
          <span {...stylex.props(styles.peekLine)}>
            <span {...stylex.props(styles.pill, pill)}>{card.label}</span>
            <span {...stylex.props(styles.note)}>{t.newStory.draftHint}</span>
          </span>
          <div {...stylex.props(styles.draftText)}>{draft || t.newStory.draftEmpty}</div>
          <p {...stylex.props(styles.note)}>{t.newStory.draftNote}</p>
          <StoryCheckBlock project={project} gate={gate} compact />
          <button type="button" onClick={start} disabled={!canStart} aria-disabled={!canStart}
            {...stylex.props(styles.button, gate.blocked ? styles.locked : canStart ? styles.primary : styles.off)}>
            {commitAction.busy ? <Spinner size={15} /> : gate.blocked ? <Icon name="lock" size={15} /> : null}
            {t.newStory.draftStart}
          </button>
          <span {...stylex.props(styles.why)}>{startLabel}</span>
          <button type="button" onClick={() => void cancelAction.run()} disabled={busy || !writable} {...stylex.props(styles.dangerLink)}>
            {t.newStory.draftCancel}
          </button>
        </div>
      </Sheet>
    </div>
  )
}
