// Đồng sáng tác (NewStory.dc.html › mode cocreate, 19-gap-closure.md G8):
// hội thoại + gợi ý ❶❷❸ + bản nháp yêu cầu + Ctrl+S commit.
// Ở màn Truyện mới là phiên cấp dự án: chưa tạo truyện cho tới /cocreate/commit;
// id nháp nhớ trong sessionStorage để tải lại trang vẫn tiếp tục.
import { useEffect, useRef, useState } from 'react'
import {
  cancelCoCreate, cancelProjectCoCreate, commitCoCreate, commitProjectCoCreate,
  getCoCreate, getProjectCoCreate, sendCoCreate, sendProjectCoCreate,
} from '../../api/client'
import type { CoCreateState } from '../../api/types'
import { useStoryEvents } from '../../api/useStoryEvents'
import { AgentOrb } from '@noverse/ui'
import { Button, Kbd } from '@noverse/ui'
import { Badge, TONE } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { useSseUnauthorized } from '../project/useProject'
import { forgetProjectDraft, readProjectDraft, rememberProjectDraft } from './cocreateDraft'
import { keys, useAction, useConnection, useResource, setData } from '../../data'
import { TextField } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { useStoryCheck } from './useStoryCheck'
import { StoryCheckBlock } from './StoryCheck'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

type Message = CoCreateState['messages'][number]

/** Nhận delta `cocreate` từ SSE (EventSource) — chỉ mount khi đã có truyện. */
function CoCreateStream({ project, story, onDelta }: { project: string; story: string; onDelta: (text: string) => void }) {
  const unauthorized = useSseUnauthorized()
  const { cocreateText } = useStoryEvents(project, story, unauthorized)
  useEffect(() => { onDelta(cocreateText) }, [cocreateText, onDelta])
  return null
}

const DOTS = (
  <span aria-hidden="true" style={{ display: 'inline-flex', gap: 3 }}>
    {[0, 1, 2].map((index) => (
      <span key={index} style={{
        width: 5, height: 5, borderRadius: '50%', background: 'var(--color-fg-3)',
        animation: `ai-twinkle 1.2s ease-in-out ${index * 0.15}s infinite`,
      }} />
    ))}
  </span>
)

export function CoCreatePanel({ project, story: initialStory, onCommitted, onCancel, narrow }: {
  project: string
  /** truyện có sẵn (dialog từ màn Truyện); null = đồng sáng tác cấp dự án (G8) */
  story?: string | null
  onCommitted: (story?: string) => void
  /** có mặt → hiện nút huỷ (POST /cocreate/cancel, xoá nháp) */
  onCancel?: () => void
  narrow?: boolean
}) {
  const connection = useConnection()
  const storyId = initialStory ?? null
  // G8: id phiên nháp cấp dự án — chỉ có khi chưa tạo truyện.
  const [draftId, setDraftId] = useState<string | null>(() => initialStory ? null : readProjectDraft(project))
  const [messages, setMessages] = useState<Message[]>([])
  const [draft, setDraft] = useState('')
  const [ready, setReady] = useState(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [text, setText] = useState('')
  const restoring = !!(storyId || draftId) && messages.length === 0
  const [streamFull, setStreamFull] = useState('')
  const streamBase = useRef(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const gate = useStoryCheck(project, draft, setDraft)

  const resourceKey = storyId ? keys.story(project, storyId, 'meta', 'cocreate') : keys.project(project, 'meta', 'cocreate', draftId)
  const resource = useResource(resourceKey, ({ signal }) => storyId ? getCoCreate(project, storyId, signal) : getProjectCoCreate(project, draftId!), { enabled: !!storyId || !!draftId })
  useEffect(() => {
    if (!resource.data) return
    setMessages(resource.data.messages)
    setDraft(resource.data.draft)
    setReady(resource.data.ready)
    setSuggestions(resource.data.suggestions)
  }, [resource.data])

  const sendAction = useAction(async (value: string) => {
    streamBase.current = streamFull.length
    const reply = storyId ? await sendCoCreate(project, storyId, value) : await sendProjectCoCreate(project, value, draftId ?? undefined)
    const nextDraftId = 'draftId' in reply && typeof reply.draftId === 'string' ? reply.draftId : draftId
    if (nextDraftId) {
      setDraftId(nextDraftId)
      rememberProjectDraft(project, nextDraftId)
    }
    const nextMessages: Message[] = [...messages, { role: 'user', content: value }, { role: 'assistant', content: reply.reply }]
    setMessages(nextMessages); setDraft(reply.draft); setReady(reply.ready); setSuggestions(reply.suggestions); setText('')
    const nextKey = storyId ? resourceKey : keys.project(project, 'meta', 'cocreate', nextDraftId)
    setData(nextKey, { messages: nextMessages, draft: reply.draft, ready: reply.ready, suggestions: reply.suggestions })
    return reply
  }, { invalidate: [keys.story(project, storyId ?? '', 'meta')] })
  const send = (raw?: string) => { const value = (raw ?? text).trim(); if (value && connection.writable) void sendAction.run(value) }

  const commitAction = useAction(async () => {
    if (!await gate.check()) return
    try {
      if (storyId) {
        await commitCoCreate(project, storyId, draft)
        onCommitted(storyId)
      } else if (draftId) {
        const result = await commitProjectCoCreate(project, draftId, undefined, draft)
        forgetProjectDraft(project); setDraftId(null)
        onCommitted(result.story.story)
      }
      return true
    } catch (error) {
      if (!gate.acceptConflict(error)) throw error
    }
  }, { invalidate: [keys.project(project), keys.library()] })
  const cancelAction = useAction(async () => {
    if (storyId) await cancelCoCreate(project, storyId)
    else if (draftId) await cancelProjectCoCreate(project, draftId)
    forgetProjectDraft(project); setDraftId(null); setMessages([]); setDraft(''); setReady(false); setSuggestions([]); setText('')
    onCancel?.()
    return true
  }, { success: 'Đã huỷ đồng sáng tác', invalidate: [resourceKey] })
  const busy = sendAction.busy || commitAction.busy || cancelAction.busy || gate.checking || gate.suggesting
  const failure = sendAction.error || commitAction.error || cancelAction.error
  const error = failure instanceof Error ? failure.message : null
  useUnsavedGuard(!!text.trim() && !commitAction.busy)
  const start = () => { if (draft.trim() && !busy && !gate.blocked && connection.writable && (storyId || draftId)) void commitAction.run() }
  useSaveShortcut(start, !!draft.trim() && !busy && !gate.blocked && connection.writable)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 's' && (event.metaKey || event.ctrlKey)) return
      const target = event.target as HTMLElement | null
      if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return
      const index = Number.parseInt(event.key, 10)
      if (index >= 1 && index <= 3 && suggestions[index - 1]) {
        event.preventDefault()
        setText(suggestions[index - 1])
        inputRef.current?.focus()
      }
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  })

  const streaming = busy ? streamFull.slice(streamBase.current) : ''
  const tone = ready ? TONE.ok : TONE.neutral

  return (
    <div style={{ flex: 1, minHeight: 0, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0 }}>
      <PanelRefreshBar resourceKeys={[resourceKey]} updatedAt={resource.updatedAt} />
      <StaleBanner error={resource.error || (connection.state === 'retrying' ? 'Mất luồng cập nhật' : null)} onRetry={() => { connection.retry(); void resource.refresh() }} />
      <div style={{ flex: 1, minHeight: 0, minWidth: 0, display: 'flex', flexDirection: narrow ? 'column' : 'row', gap: 12 }}>
      <section className="card" aria-label="Hội thoại đồng sáng tác" style={{
        flex: narrow ? '1 1 auto' : '1 1 0', minWidth: 0, minHeight: narrow ? 420 : 0,
        borderRadius: 16, display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        <div className="scroll" style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column-reverse', padding: '16px 18px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {restoring && resource.initialLoading && <Skeleton height={160} radius={12} />}
            {messages.length === 0 && !busy && (
              <div style={{ display: 'flex', gap: 10, maxWidth: '86%' }}>
                <AgentOrb agent="architect" size={24} style={{ marginTop: 2 }} />
                <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-2)' }}>
                  Mô tả truyện bạn muốn viết. Tôi sẽ hỏi lại vài câu để làm rõ bối cảnh, tông truyện và kết cục mong muốn trước khi bắt đầu.
                </p>
              </div>
            )}
            {messages.map((message, index) => (
              message.role === 'user' ? (
                <div key={`${index}-user`} style={{
                  alignSelf: 'flex-end', maxWidth: '78%', padding: '10px 14px',
                  borderRadius: '16px 16px 4px 16px', background: 'var(--color-surface-6)', fontSize: 14,
                  whiteSpace: 'pre-line', overflowWrap: 'anywhere',
                }}>{message.content}</div>
              ) : (
                <div key={`${index}-assistant`} style={{ display: 'flex', gap: 10, maxWidth: '86%' }}>
                  <AgentOrb agent="architect" size={24} style={{ marginTop: 2 }} />
                  <p style={{ margin: 0, fontSize: 14, color: 'var(--color-fg-2)', whiteSpace: 'pre-line', overflowWrap: 'anywhere' }}>
                    {message.content}
                  </p>
                </div>
              )
            ))}
            {busy && (
              <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                <AgentOrb agent="architect" size={24} live />
                <span style={{ fontSize: 13, color: 'var(--color-fg-3)', display: 'flex', alignItems: 'center', gap: 8 }}>
                  {streaming
                    ? <span style={{ whiteSpace: 'pre-line', color: 'var(--color-fg-2)' }}>{streaming}</span>
                    : <>AI đang tổng hợp yêu cầu của bạn{DOTS}</>}
                </span>
              </div>
            )}
          </div>
        </div>

        <div style={{
          flex: '0 0 auto', padding: '10px 12px 12px', borderTop: '1px solid var(--line-1)',
          display: 'flex', flexDirection: 'column', gap: 8,
        }}>
          {error && (
            <span role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}>
              <Icon name="warning-2" size={14} />{error}
            </span>
          )}
          {suggestions.length > 0 && (
            <>
              <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
                Gợi ý từ AI — bấm hoặc nhấn 1/2/3 để chọn, có thể sửa trước khi gửi
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {suggestions.map((suggestion, index) => {
                  const selected = text === suggestion
                  return (
                    <button key={suggestion} type="button" className="hov-row" onClick={() => { setText(suggestion); inputRef.current?.focus() }}
                      style={{
                        display: 'flex', alignItems: 'center', gap: 8, height: 32, padding: '0 12px 0 6px',
                        borderRadius: 999, border: `1px solid ${selected ? 'var(--line-5)' : 'var(--line-3)'}`,
                        background: selected ? 'var(--color-surface-6)' : 'var(--color-surface-1)',
                        color: 'var(--color-fg-2)', fontSize: 13, maxWidth: '100%',
                      }}>
                      <span className="mono" style={{
                        width: 20, height: 20, borderRadius: '50%', display: 'flex', alignItems: 'center',
                        justifyContent: 'center', background: 'var(--color-surface-7)', fontSize: 11, flex: '0 0 auto',
                      }}>{index + 1}</span>
                      <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{suggestion}</span>
                    </button>
                  )
                })}
              </div>
            </>
          )}
          <form onSubmit={(event) => { event.preventDefault(); void send() }}
            style={{
              borderRadius: 16, padding: '6px 6px 6px 12px', display: 'flex', alignItems: 'center', gap: 10,
              background: 'var(--color-surface-5)', border: '1px solid var(--line-3)',
            }}>
            <Icon name="messages-2" size={18} style={{ color: 'var(--color-fg-4)' }} />
            <label htmlFor="cochat" className="sr-only">Trao đổi với AI</label>
            <div style={{ flex: 1, minWidth: 0 }}>
            <TextField id="cochat" ref={inputRef} type="text" value={text} onChange={setText} maxLength={5000}
              placeholder="Tiếp tục bổ sung yêu cầu, Enter để gửi cho AI"
              style={{
                flex: 1, minWidth: 0, height: 36, background: 'transparent', border: 'none',
                outline: 'none', color: 'var(--color-fg)', fontSize: 14,
              }} />
            </div>
            <Button variant="primary" type="submit" tooltip="Gửi · Enter" aria-label="Gửi" disabled={busy || !connection.writable || !text.trim()}
              pending={sendAction.pending} success={sendAction.success} icon={<Icon name="arrow-up" size={18} />} />
          </form>
        </div>
      </section>

      <aside className="card" aria-label="Bản nháp yêu cầu" style={{
        flex: narrow ? '1 1 auto' : '0 0 360px', maxWidth: narrow ? 'none' : 360,
        minHeight: 0, borderRadius: 16, display: 'flex', flexDirection: 'column', overflow: 'hidden',
      }}>
        <div style={{
          flex: '0 0 auto', padding: '14px 16px 10px', display: 'flex', alignItems: 'center', gap: 8,
          borderBottom: '1px solid var(--line-1)',
        }}>
          <Icon name="document-text" size={18} style={{ color: 'var(--color-fg-3)' }} />
          <h2 style={{ margin: 0, flex: 1, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-2)' }}>Bản nháp yêu cầu</h2>
          <Badge tone={ready ? 'ok' : 'neutral'} style={{ height: 22, background: tone.bg, color: tone.fg }}>
            <Icon name={ready ? 'tick-circle-bulk' : 'magicpen'} size={13} />
            {ready ? 'Sẵn sàng viết' : 'Đang làm rõ'}
          </Badge>
        </div>
        <div className="scroll" style={{ flex: 1, minHeight: 0, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13, color: 'var(--color-fg-2)' }}>
          <p style={{ margin: 0, color: 'var(--color-fg-4)' }}>
            AI cập nhật bản này sau mỗi lượt trao đổi. Khi bắt đầu, toàn bộ nội dung dưới đây trở thành yêu cầu sáng tác.
          </p>
          <div style={{
            padding: 12, borderRadius: 10, background: 'var(--color-surface-0)',
            border: '1px solid var(--line-1)', whiteSpace: 'pre-line', lineHeight: 1.7,
            color: draft ? 'var(--color-fg-2)' : 'var(--color-fg-4)',
          }}>
            {draft || 'Chưa có bản nháp — trao đổi thêm với AI để AI tổng hợp yêu cầu.'}
          </div>
          <StoryCheckBlock project={project} gate={gate} compact />
        </div>
        <div style={{
          flex: '0 0 auto', padding: '12px 16px 14px', borderTop: '1px solid var(--line-1)',
          display: 'flex', flexDirection: 'column', gap: 8,
        }}>
          <Button variant="primary" onClick={start} disabled={busy || gate.blocked || !connection.writable || !draft.trim() || (!storyId && !draftId)}
            style={{ height: 40, borderRadius: 10, fontSize: 14, fontWeight: 600, ...(gate.blocked ? { background: 'var(--kit-surface-toast)', color: 'var(--kit-fg-arrow)', border: '1px solid var(--line-3)', opacity: 1 } : {}) }}
            pending={commitAction.pending} pendingLabel={gate.checking ? t.newStoryCheck.checking : undefined} success={commitAction.success && !gate.blocked} tooltip="Bắt đầu sáng tác · ⌘S / Ctrl+S" icon={gate.blocked ? <Icon name="lock" size={15} /> : undefined}>
            Bắt đầu sáng tác {!gate.blocked && <Kbd>⌘ S</Kbd>}
          </Button>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)', textAlign: 'center' }}>
            {gate.blocked ? t.newStoryCheck.blockedHint : t.newStory.keepDrafting}
          </span>
          {onCancel && (
            <Button onClick={() => void cancelAction.run()} disabled={busy || !connection.writable} pending={cancelAction.pending}>Huỷ đồng sáng tác</Button>
          )}
        </div>
      </aside>

      </div>
      {storyId && <CoCreateStream project={project} story={storyId} onDelta={setStreamFull} />}
    </div>
  )
}
