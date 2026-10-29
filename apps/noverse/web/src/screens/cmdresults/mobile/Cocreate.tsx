import { useEffect, useRef, useState } from 'react'
import { commitCoCreate, getCoCreate, getForeshadow, sendCoCreate } from '../../../api/client'
import { useStoryEvents } from '../../../api/useStoryEvents'
import { invalidate, keys, setData, useAction, useResource } from '../../../data'
import { Icon } from '@noverse/ui'
import { ThinkingOrb } from '@noverse/ui'
import { Actions, Button, Failure, Loading, Orb, Tick, useCmdToast } from './shared'
import { chapterRange, impactKind, replyDelta } from './logic'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const noop = () => {}
const clock = (date: Date) => date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', hour12: false })
export function Cocreate({ project, story, onWrite }: { project: string; story: string; onWrite: () => void }) {
  const key = [...keys.story(project, story, 'snapshot'), 'cocreate']
  const resource = useResource(key, ({ signal }) => getCoCreate(project, story, signal), { staleTime: 0 })
  const live = useStoryEvents(project, story, noop)
  const state = resource.data
  const [text, setText] = useState('')
  const [pendingMessage, setPendingMessage] = useState('')
  const scroll = useRef<HTMLDivElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const streamStart = useRef('')
  const initialMessages = useRef<number | null>(null)
  const notify = useCmdToast()
  const foreshadow = useResource(keys.story(project, story, 'foreshadow'), ({ signal }) => getForeshadow(project, story, signal), { staleTime: 0 })
  const [lockedAt, setLockedAt] = useState('')
  const [openedAt] = useState(() => clock(new Date()))
  const send = useAction(async (message: string) => {
    streamStart.current = live.cocreateText
    setPendingMessage(message)
    try {
      await sendCoCreate(project, story, message)
      await invalidate(key)
      setText((draft) => draft === message ? '' : draft)
    } finally { setPendingMessage('') }
  }, { error: false })
  const commit = useAction(async () => {
    const result = await commitCoCreate(project, story, state?.draft)
    if (state) setData(key, { ...state, committed: result.committed === true, active: result.committed !== true, nextChapter: result.nextChapter ?? state.nextChapter })
    await invalidate(keys.story(project, story))
    if (result.committed) setLockedAt(clock(new Date()))
    notify(result.committed ? `Đã chốt hướng đi${result.nextChapter ? ` · đang viết chương ${result.nextChapter}` : ''}` : 'Đã gửi yêu cầu chốt · Kiến trúc sư đang cập nhật dàn ý', result.committed ? onWrite : null)
  }, { error: false })
  useEffect(() => { if (live.revision) void resource.refresh() }, [live.revision, resource.refresh])
  const streamText = replyDelta(live.cocreateText, streamStart.current)
  const streaming = send.busy && streamText
  const thinking = send.busy && !streaming
  useEffect(() => {
    if (!state) return
    if (initialMessages.current === null) { initialMessages.current = state.messages.length; return }
    if (state.messages.length !== initialMessages.current || pendingMessage || streaming || thinking) scroll.current?.scrollTo({ top: scroll.current.scrollHeight, behavior: 'smooth' })
  }, [state, pendingMessage, streaming, thinking])
  const proposal = state?.proposal
  const committed = state?.committed ?? false
  const next = state?.nextChapter ?? (live.snapshot ? live.snapshot.completedCount + 1 : null)
  const openForeshadows = foreshadow.data ? foreshadow.data.entries.filter((entry) => entry.status !== 'resolved').length : null
  const range = proposal ? chapterRange(proposal.chapters) : ''
  const reading = [live.snapshot ? `đọc ${live.snapshot.completedCount} chương` : '', openForeshadows === null ? '' : `${openForeshadows} phục bút đang mở`].filter(Boolean).join(', ')
  const submit = () => { const message = text.trim(); if (message && !send.busy) void send.run(message) }
  return <><div ref={scroll} className="mc-scroll mc-chat">
    <div className="mc-architect-status"><Orb size={22} live={send.busy} /><span>{committed ? `Đã chốt hướng đi${next ? ` · Người viết đang bắt đầu chương ${next}` : ''}` : send.busy ? 'Kiến trúc sư đang soạn trả lời…' : `Kiến trúc sư · ${reading || 'bàn hướng đi cùng bạn'}`}</span></div>
    <PanelRefreshBar resourceKeys={[resource.key]} /><Failure error={resource.error} onRetry={() => void resource.refresh()} />
    {!state && !resource.error && <Loading text="Đang mở cuộc bàn hướng đi…" />}
    {state && <><span className="mc-system">Bạn đã chạy /cocreate · hôm nay {openedAt}</span>
      {state.messages.length === 0 && !send.busy && <p className="mc-note">Bạn muốn viết tiếp theo hướng nào? Gửi ý tưởng để Kiến trúc sư đưa ra đề xuất.</p>}
      {state.messages.map((message, index) => message.role === 'user' ? <div className="mc-me mc-rise" key={index}>{message.content}</div> : <div className="mc-ai mc-rise" key={index}><Orb /><p>{message.content}</p></div>)}
      {pendingMessage && <div className="mc-me mc-rise">{pendingMessage}</div>}
      {thinking && <div className="mc-ai"><Orb live /><span className="mc-thinking" aria-label="Kiến trúc sư đang nghĩ"><i /><i /><i /></span></div>}
      {streaming && <div className="mc-ai"><Orb live /><p>{streamText}<ThinkingOrb size={22} label="Kiến trúc sư đang trả lời" /></p></div>}
      {proposal && <section className={`mc-proposal mc-rise ${committed ? 'mc-accepted' : ''}`} aria-label="Đề xuất hướng đi">
        <div className="mc-proposal-title"><Icon name="routing-2" size={18} /><span>ĐỀ XUẤT · {proposal.title.toLocaleUpperCase('vi')}</span>{proposal.revision > 1 && <small>Đã chỉnh v{proposal.revision}</small>}</div>
        <ol>{proposal.chapters.map((chapter) => <li key={chapter.chapter}><span className="mc-chapter-number">{chapter.chapter}</span><div><strong>{chapter.title}</strong><p>{chapter.summary}</p></div></li>)}</ol>
        <div className="mc-impacts">{proposal.impacts.map((impact, index) => { const kind = impactKind(impact); return <span key={index}><Icon name={kind.icon} size={13} style={{ color: kind.color }} />{impact}</span> })}</div>
        {committed ? <div className="mc-success"><Tick size={20} /><span>Đã chốt · dàn ý{range ? ` chương ${range}` : ''} đã cập nhật</span></div> : <div className="mc-two-buttons"><Button disabled={send.busy || commit.busy} onClick={() => { setText(`Chương ${proposal.chapters.at(-1)?.chapter ?? ''}: `); input.current?.focus() }}>Sửa thêm</Button><Button primary disabled={!state.ready || send.busy || commit.busy} onClick={() => void commit.run()}>{commit.busy ? 'Đang chốt…' : <><Icon name="tick-circle" size={17} />Chốt &amp; viết tiếp</>}</Button></div>}
      </section>}
      {lockedAt && committed && <span className="mc-system">Đã chốt · dàn ý{range ? ` chương ${range}` : ''} cập nhật lúc {lockedAt}</span>}
    </>}
    <Failure error={send.error || commit.error} />
  </div>{committed ? <Actions><Button primary onClick={onWrite}><Orb writer live size={16} />Xem Người viết{next ? ` làm chương ${next}` : ''}</Button></Actions> : <div className="mc-composer"><form className="kit-frame mc-composer-box" onSubmit={(event) => { event.preventDefault(); submit() }}><input ref={input} value={text} onChange={(event) => setText(event.target.value)} placeholder="Trả lời Kiến trúc sư…" aria-label="Trả lời Kiến trúc sư" enterKeyHint="send" /><button className="mc-send" type="submit" aria-label="Gửi" disabled={!text.trim() || send.busy || commit.busy}><Icon name="arrow-up" size={20} /></button></form></div>}</>
}
