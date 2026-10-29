import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { approveWithComments, getAddressTerms, getBrainUpTo, getSnapshot, sendInput } from '../../../api/client'
import { useAction, useConnection, useResource, useResourceValue, keys, events } from '../../../data'
import { BottomSheet, MobilePage, useMobileToast } from '../../../mobile/primitives'
import { useMobileRuntime, useMobileScroll } from '../../../mobile/runtime'
import { navigate, paths } from '../../../router'
import { Icon } from '@noverse/ui'
import { CharAvatar } from '../../../ui/CharAvatar'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { ChapterContent, parseChapterText, parseInline } from '../../../lib/markdown'
import { isGate } from '../../workspace/GateCard'
import { useChapterContent, useChapterExtras, useChapters, useStoryAccess } from '../useReaderData'
import { paragraphAnchor, selectionInReader, useParagraphComments, type ParagraphSelection } from '../comments'
import { attentionBadge, clipWords, commentBody, feedbackCounts, paragraphMarks, reanchoredOf, reanchorBanner, reanchorToast, type ReanchoredFrame, type ParagraphMark } from '../feedback'
import { FeedbackBody, RelinkBar, type FeedbackAction } from '../FeedbackGroups'
import { getDisplay, getPosition, saveDisplay, savePosition, type ReaderDisplay } from './preferences'
import type { ChapterInfo, ParagraphComment, ParagraphCommentPatch } from '../../../api/types'
import { enqueueComment, useCommentOutbox } from '../../../mobile/outbox'
import { ChapterActions } from './ChapterActions'
import { ChapterList } from './ChapterList'
import { CommentSheet } from './CommentSheet'
import { FeedbackSheet } from './FeedbackSheet'
import { chapterReadable, chapterReadingState, nextChapterCaption, readerSnapshot, swipeChapter } from './navigation'
import { setDraft, useAccountDraft } from '../../../mobile/accounts'
import { useSession } from '../../../api/session'
import { writingPercent } from '../../workspace/mobile/logic'
import { characterNames, personQuickView, splitByNames } from '../../brain/mobile/view'
import './reader.css'
import '../feedback.css'
import { ScrollTopButton } from '@noverse/ui'
import { t } from '../../../i18n/vi'
import { copy } from '../copy'
import { copyText } from '../../../lib/clipboard'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'
type Sheet = 'font' | 'chapters' | 'comment' | 'feedback' | null
const emptySelection: ParagraphSelection = { paragraph: null, quote: '' }
const serif = 'Literata, Georgia, serif'

export function MobileReader({ project, story, chapter }: { project: string; story: string; chapter: number | null }) {
  const { user } = useSession()
  const toast = useMobileToast()
  const connection = useConnection()
  const { chapters, outline, error: chaptersError, reload: reloadChapters } = useChapters(project, story)
  const remembered = useMemo(() => getPosition(project, story), [project, story])
  const current = chapter ?? (chapters?.some((item) => item.chapter === remembered?.chapter) ? remembered!.chapter : chapters?.filter((item) => item.completed || item.words > 0).at(-1)?.chapter) ?? null
  const info = chapters?.find((item) => item.chapter === current)
  const source = info && !info.completed ? 'draft' : 'committed'
  const { content, loading, error, reload, updatedAt } = useChapterContent(project, story, current, source)
  const extras = useChapterExtras(project, story, current)
  const snapshot = useResourceValue(keys.story(project, story, 'snapshot'), () => getSnapshot(project, story), readerSnapshot)
  // M8 · xem nhanh nhân vật: chỉ dữ liệu tới chương đang đọc (máy chủ cắt bằng `upTo`).
  const brain = useResource(keys.story(project, story, 'brain', 'upTo', current ?? 0), ({ signal }) => getBrainUpTo(project, story, current ?? 1, signal), { enabled: current !== null })
  const address = useResource(keys.story(project, story, 'address'), ({ signal }) => getAddressTerms(project, story, signal), { enabled: current !== null })
  const [person, setPerson] = useState<string | null>(null)
  const comments = useParagraphComments(project, story, current)
  const { locked, writable } = useStoryAccess(project, story)
  const [display, setDisplay] = useState(getDisplay)
  const [sheet, setSheet] = useState<Sheet>(null)
  const [selected, setSelected] = useState<ParagraphSelection>(emptySelection)
  const [chips, setChips] = useState<string[]>([])
  const [rewriteChapter, setRewriteChapter] = useState<number | null>(null)
  const [note, setNote] = useAccountDraft(`reader:${project}/${story}/${rewriteChapter ?? current ?? 0}`)
  const [menuChapter, setMenuChapter] = useState<ChapterInfo | null>(null)
  const [queueError, setQueueError] = useState<unknown>(null)
  const [queueing, setQueueing] = useState(false)
  const progressRef = useRef<HTMLDivElement>(null)
  const updateProgress = useCallback((value: number) => {
    const node = progressRef.current
    if (!node) return
    node.setAttribute('aria-valuenow', String(Math.round(value * 100)))
    const bar = node.firstElementChild as HTMLElement | null
    if (bar) bar.style.transform = `scaleX(${value})`
  }, [])
  const bodyRef = useRef<HTMLDivElement>(null)
  const paragraphsRef = useRef<HTMLDivElement>(null)
  const scrollerRef = useRef<HTMLElement | null>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const previousChapter = useRef(current)
  useLayoutEffect(() => {
    scrollerRef.current?.scrollTo({ top: 0, behavior: 'instant' })
    updateProgress(0)
  }, [current, updateProgress])
  const rememberScrollRef = useRef<() => void>(() => {})
  const rememberScroll = useCallback(() => rememberScrollRef.current(), [])
  const { immersive, reveal } = useMobileScroll(scrollerRef, { immersive: true, onScroll: rememberScroll })
  const { keyboard } = useMobileRuntime()
  const queuedComments = useCommentOutbox(project, story, current ?? 0)
  const swipeStart = useRef<{ x: number; y: number } | null>(null)
  const longPress = useRef<NodeJS.Timeout | undefined>(undefined)
  const pressedAt = useRef<{ x: number; y: number } | null>(null)
  const suppressClick = useRef(false)
  const contextRef = useRef('')
  contextRef.current = JSON.stringify([project, story, current])
  const blocks = useMemo(() => parseChapterText(content?.text ?? ''), [content?.text])
  const paragraphTexts = useMemo(() => blocks.filter((block) => block.kind === 'paragraph').map((block) => parseInline(block.text).map((token) => token.text).join('')), [blocks])
  const names = useMemo(() => (brain.data && current !== null ? characterNames(brain.data, current) : []), [brain.data, current])
  const quick = person && brain.data && current !== null ? personQuickView(brain.data, address.data ?? null, person, current) : null
  // Tên nhân vật trong chính văn: gạch chấm + chạm mở sheet xem nhanh (MReader.dc.html › state=person).
  const renderPlain = useCallback((value: string) => splitByNames(value, names).map((segment, index) => segment.id
    ? <span key={index} className={`mr-ent${person === segment.id ? ' on' : ''}`} role="link" tabIndex={0} aria-label={`Xem nhanh ${segment.text}`}
        style={{ '--ent': segment.color } as CSSProperties}
        onClick={(event) => { event.stopPropagation(); setPerson(segment.id!) }}
        onPointerDown={(event) => event.stopPropagation()}
        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); setPerson(segment.id!) } }}>{segment.text}</span>
    : <span key={index}>{segment.text}</span>), [names, person])
  const title = content?.title || info?.title || (current ? `Chương ${current}` : 'Đọc truyện')
  const words = content?.words ?? info?.words ?? 0
  const pendingGate = !!snapshot && isGate(snapshot) && current === snapshot.completedCount
  const noteCount = comments.data?.comments.length ?? 0
  const review = useAction(() => approveWithComments(project, story, current!), {
    success: current === null ? 'Đã duyệt' : noteCount > 0 ? `Đã duyệt · Người viết sửa theo ${noteCount} góp ý rồi viết chương ${current + 1}` : `Đã duyệt chương ${current} · đang viết chương ${current + 1}`,
    invalidate: [keys.story(project, story, 'snapshot'), keys.story(project, story, 'chapters'), keys.story(project, story, 'comments', current)],
  })
  const rewrite = useAction((input: { chapter: number; text: string }) => sendInput(project, story, `Viết lại chương ${input.chapter}: ${input.text}`), {
    success: `Đã gửi · Người viết sẽ viết lại chương ${rewriteChapter ?? ''}`.trim(),
    invalidate: [keys.story(project, story)],
  })
  const closeSheet = useCallback(() => {
    setSheet(null)
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  }, [])
  const changeDisplay = (patch: Partial<ReaderDisplay>) => setDisplay((previous) => { const next = { ...previous, ...patch }; saveDisplay(next); return next })
  const openComment = (selection: ParagraphSelection) => {
    if (!writable) return
    setRewriteChapter(null); setQueueError(null); setSelected(selection); setChips([]); setNote(''); setSheet('comment')
  }
  const cancelPress = () => { clearTimeout(longPress.current); pressedAt.current = null }
  useEffect(() => () => clearTimeout(longPress.current), [])
  useEffect(() => { setSelected(emptySelection); setSheet(null); setMenuChapter(null); setRewriteChapter(null); setQueueError(null); setChips([]); setNote(''); setPerson(null); reveal() }, [project, story, current, reveal])
  useEffect(() => { if (!writable && sheet === 'comment') setSheet(null) }, [writable, sheet])
  useEffect(() => { setSelected(emptySelection); setSheet(null); setMenuChapter(null); setChips([]) }, [user?.username])

  // Le repère est un paragraphe + décalage, pas un pourcentage fragile après réglage de police.
  useEffect(() => {
    const body = bodyRef.current
    const scroller = body?.parentElement
    if (!body || !scroller || !content?.text || current === null) return
    const saved = getPosition(project, story)
    const nodes = [...body.querySelectorAll<HTMLElement>('[data-reader-paragraph]')]
    const topOf = (node: HTMLElement) => node.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop
    const target = previousChapter.current === current && saved?.chapter === current ? nodes[saved.paragraph] : undefined
    previousChapter.current = current
    scroller.scrollTop = target ? Math.max(0, topOf(target) + saved!.offset) : 0
    let saveTimer: ReturnType<typeof setTimeout> | undefined
    let position: Parameters<typeof savePosition>[2] | undefined
    const persist = () => { clearTimeout(saveTimer); if (position) savePosition(project, story, position) }
    const remember = () => {
      const y = scroller.scrollTop
      const max = scroller.scrollHeight - scroller.clientHeight
      const pct = max > 0 ? Math.min(1, Math.max(0, y / max)) : 1
      const scrollerTop = scroller.getBoundingClientRect().top
      const paragraphTop = (index: number) => nodes[index].getBoundingClientRect().top - scrollerTop + y
      let low = 0, high = nodes.length
      while (low < high) {
        const middle = (low + high) >>> 1
        if (paragraphTop(middle) <= y + 4) low = middle + 1; else high = middle
      }
      const index = Math.max(0, low - 1)
      position = { chapter: current, paragraph: index, offset: nodes[index] ? y - paragraphTop(index) : 0, progress: pct }
      updateProgress(pct)
      clearTimeout(saveTimer); saveTimer = setTimeout(persist, 150)
    }
    remember()
    rememberScrollRef.current = remember
    window.addEventListener('pagehide', persist)
    return () => { persist(); rememberScrollRef.current = () => {}; window.removeEventListener('pagehide', persist) }
  }, [project, story, current, content?.text, display.size, display.family, display.spacing, updateProgress])

  const pickChapter = (next: number) => { closeSheet(); setMenuChapter(null); navigate(paths.reader(project, story, next)) }
  const listenChapter = (next: ChapterInfo) => { closeSheet(); setMenuChapter(null); navigate(`${paths.audio(project, story, next.volume, 'listen')}?chapter=${next.chapter}&play=1`) }
  const requestRewrite = (number: number) => { setMenuChapter(null); setSelected(emptySelection); setChips([]); setNote(''); setRewriteChapter(number); setQueueError(null); setSheet('comment') }
  const sendComment = async () => {
    if (!writable || comments.add.busy || rewrite.busy || queueing || (!note.trim() && !chips.length) || current === null) return
    const input = { ...selected, chips: [...chips], text: note.trim() }
    const origin = contextRef.current
    if (rewriteChapter !== null) {
      if (!connection.writable) return
      const result = await rewrite.run({ chapter: rewriteChapter, text: [chips.join(', '), note.trim()].filter(Boolean).join(' — ') })
      if (result) { setNote(''); if (contextRef.current === origin) { setDraft('active', ''); closeSheet() } }
      return
    }
    if (!connection.writable) {
      setQueueing(true); setQueueError(null)
      try {
        await enqueueComment({ project, story, chapter: current, input })
        setNote('')
        if (contextRef.current === origin) { setDraft('active', ''); closeSheet(); setSelected(emptySelection) }
        toast('Chờ gửi · sẽ tự gửi khi có mạng')
      } catch (cause) { setQueueError(cause) }
      finally { setQueueing(false) }
      return
    }
    const request = comments.add.run(input)
    closeSheet(); setSelected(emptySelection)
    const result = await request
    if (!result && contextRef.current === origin) { setSelected(selected); setSheet('comment') }
    if (result) { setNote(''); if (contextRef.current === origin) setDraft('active', '') }
  }
  const copyParagraph = async () => {
    try { await copyText(selected.quote); toast('Đã chép đoạn này'); setSelected(emptySelection) }
    catch { toast('Không chép được. Hãy chọn văn bản rồi chép từ trình duyệt.') }
  }
  // §AI — vòng đời góp ý: sheet .fbsheet, vết gạch trong đoạn, chế độ gắn lại, toast/banner viết lại.
  const [openId, setOpenId] = useState<string | null>(null)
  const [relinkId, setRelinkId] = useState<string | null>(null)
  const [relinkHov, setRelinkHov] = useState<number | null>(null)
  const [patching, setPatching] = useState<string | null>(null)
  const [reanchorToastMsg, setReanchorToastMsg] = useState<string | null>(null)
  useEffect(() => {
    let timer: number | undefined
    const stop = events.subscribe('comments.reanchored', (data) => {
      if (current === null) return
      const event = reanchoredOf(data as ReanchoredFrame, project, story, current)
      if (!event) return
      setReanchorToastMsg(reanchorToast(event))
      window.clearTimeout(timer)
      timer = window.setTimeout(() => setReanchorToastMsg(null), 5200)
    })
    return () => { stop(); window.clearTimeout(timer) }
  }, [project, story, current])
  const fbPatch = async (id: string, body: ParagraphCommentPatch) => {
    setPatching(id)
    try { await comments.patch.run(id, body) } finally { setPatching(null) }
  }
  const onFeedbackAction = async (action: FeedbackAction) => {
    const id = action.comment.id
    if (action.kind === 'remove') { void comments.remove(id); if (openId === id) setOpenId(null); return }
    if (action.kind === 'resolve') { void fbPatch(id, { action: 'resolve' }); return }
    if (action.kind === 'reopen') { void fbPatch(id, { action: 'reopen' }); return }
    if (action.kind === 'resend') { void fbPatch(id, { action: 'resend_whole' }); return }
    if (action.kind === 'relink') { setRelinkId(id); closeSheet(); setSelected(emptySelection); window.getSelection()?.removeAllRanges(); return }
    await fbPatch(id, { action: 'relink', paragraph: action.paragraph, quote: action.quote })
    setOpenId(id)
  }
  const pickRelink = (index: number) => {
    if (!relinkId) return
    const id = relinkId
    const quote = selected.paragraph === index ? selected.quote : undefined
    setRelinkId(null); setRelinkHov(null)
    void fbPatch(id, { action: 'relink', paragraph: index, quote }).then((done) => {
      if (done === undefined) return
      setOpenId(id); setSheet('feedback'); toast('Đã gắn lại · Chờ gửi')
    })
  }
  const captureSelection = () => {
    if (!writable || !paragraphsRef.current) return
    const result = selectionInReader(paragraphsRef.current, window.getSelection())
    if (result.paragraph !== null && result.quote) setSelected(result)
    else if (result.quote) setSelected(emptySelection)
  }
  const visibleComments = writable ? [...(comments.data?.comments ?? []), ...queuedComments.map((entry): ParagraphComment => ({ ...entry.input, id: `queued-${entry.id}`, createdAt: entry.createdAt, author: user?.username ?? '', appliedAt: undefined, status: 'pending' }))] : []
  const fbCounts = feedbackCounts(visibleComments)
  const fbOutcome = fbCounts.fixed + fbCounts.unfixed + fbCounts.need
  const fbBadge = attentionBadge(fbCounts)
  const relinkTarget = visibleComments.find((comment) => comment.id === relinkId)
  const relinkCandidates = relinkTarget ? (relinkTarget.candidates ?? []).map((candidate) => candidate.paragraph) : []
  const openCommentFb = visibleComments.find((comment) => comment.id === openId)
  const candParagraphs = openCommentFb ? (openCommentFb.candidates ?? []).map((candidate) => candidate.paragraph) : []
  const scores = extras.review?.entry.dimensions?.map((item) => item.score)
  const editorScore = scores?.length ? (scores.reduce((sum, value) => sum + value, 0) / scores.length / 10).toFixed(1) : undefined
  const nextChapter = current === null ? undefined : chapters?.find((item) => item.chapter === current + 1)
  const nextState = nextChapter ? chapterReadingState(nextChapter, snapshot) : null
  const chaptersTitle = `Tập ${info?.volume ?? 1} · ${outline?.volumes.find((volume) => volume.index === (info?.volume ?? 1))?.title.replace(/^Tập \d+\s*[·—-]?\s*/, '') || snapshot?.bookTitle || 'Danh sách chương'}`
  const draftMissing = source === 'draft' && error?.status === 404
  const emptyDraft = current !== null && source === 'draft' && !loading && !chaptersError && (!error || draftMissing) && !content?.text.trim()
  let paragraph = -1

  return <div className={`mr-root${immersive ? ' mr-immersive' : ''}${pendingGate && writable ? ' mr-gate' : ''}`} onClickCapture={(event) => {
    if (immersive && event.target instanceof Element && event.target.closest('.mr-paragraph')) { reveal(); event.stopPropagation() }
  }}><MobilePage manageScroll={false} title={<button className="mr-title" onClick={() => setSheet('chapters')} aria-haspopup="dialog"><span><small>Tập {info?.volume ?? '…'} · chương {current ?? '—'}/{chapters?.length ?? '—'}</small><strong>{title}</strong></span><Icon name="arrow-down-2" size={15} /></button>} actions={<><button className="m-icon-button mr-aa" aria-label="Cỡ chữ và kiểu chữ" onClick={() => setSheet('font')}>Aa</button><button type="button" className="m-icon-button" onClick={() => navigate(`${paths.audio(project, story, info?.volume, 'listen')}${current ? `?chapter=${current}&play=1` : ''}`)} aria-label="Nghe chương này"><Icon name="headphone" size={21} /></button></>} footer={pendingGate && writable && selected.paragraph === null ? <div className="mr-review" role="group" aria-label="Duyệt chương"><button className="m-button secondary" disabled={review.busy} onClick={() => openComment(emptySelection)}><Icon name="edit-2" size={17} />Góp ý sửa</button><button className="m-button" disabled={review.busy || comments.add.busy || !connection.writable} aria-busy={review.pending} onClick={() => { void review.run() }}>{review.pending ? 'Đang duyệt…' : 'Duyệt & viết tiếp'}</button></div> : undefined}>
    <div className="mr-body" ref={(node) => { bodyRef.current = node; scrollerRef.current = node?.parentElement ?? null }}>{connection.state !== 'open' && <div className="mr-freshness" role="status">{!connection.online ? 'Ngoại tuyến · đang hiển thị bản đã lưu' : 'Đang kết nối lại · nội dung có thể chưa mới nhất'}{updatedAt ? ` · ${new Date(updatedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}` : ''}</div>}
    {(chaptersError || (error && !draftMissing)) && <div className="mr-retry" role="alert"><span>{(chaptersError || error)?.message}</span><button className="m-button secondary" onClick={() => { void (chaptersError ? reloadChapters() : reload()) }}>Thử lại</button></div>}
    {!content && (loading || chapters === null) && <div className="mr-loading" aria-label="Đang tải chương" aria-busy="true">{[90, 100, 88, 96, 72].map((width, i) => <Skeleton key={i} width={`${width}%`} height={18} />)}</div>}
    {emptyDraft && <EmptyState icon="edit-2" title={copy.emptyDraft} hint={copy.emptyDraftHint} />}
    {current !== null && content && !emptyDraft && <>
      <div className="mr-heading"><span className="mr-label">Chương {current}</span><h1 ref={headingRef} tabIndex={-1}>{title}</h1><div className="mr-meta"><span className={`mr-status${pendingGate ? ' gate' : ''}`}><Icon name={pendingGate ? 'timer-1' : 'tick-circle'} size={13} />{pendingGate ? 'Chờ bạn duyệt' : info?.hasDraft && !info.completed ? 'Đang viết' : 'Đã duyệt'}</span><span>{words.toLocaleString('vi-VN')} chữ · {Math.max(1, Math.ceil(words / 260))} phút đọc{editorScore !== undefined ? ` · Biên tập ${String(editorScore).replace('.', ',')}` : ''}</span></div></div>
      {locked && <div className="mr-notice">Truyện đang khoá · bạn vẫn có thể đọc chương đã lưu.</div>}
      {!content.text && <div className="mr-notice">Chương này chưa có nội dung. Hãy chọn một chương đã viết trong danh sách.</div>}
      {/* §AI — gắn lại (M2b fb:relink) + banner đang sửa / đã viết lại (fb:sent, fb:after). */}
      {relinkId && <RelinkBar mobile body={clipWords(commentBody(relinkTarget!) || 'góp ý', 38)} hasCandidates={relinkCandidates.length > 0} onCancel={() => { setRelinkId(null); setRelinkHov(null); setSheet('feedback') }} />}
      {fbCounts.sent > 0 && (
        <div className="fbban s" role="status"><i className="fb-spin" aria-hidden="true" /><span><b>{`Người viết đang sửa theo ${fbCounts.sent} góp ý.`}</b>{' Chương sẽ tự thay khi sửa xong.'}</span></div>
      )}
      {fbOutcome > 0 && !relinkId && (
        <div className="fbban a" role="status">
          <b>{reanchorBanner(current!, fbCounts)}</b>
          <div className="bpills">
            {fbCounts.fixed > 0 && <button type="button" className="bpill f" onClick={() => setSheet('feedback')}>{`Đã sửa ${fbCounts.fixed}`}</button>}
            {fbCounts.unfixed > 0 && <button type="button" className="bpill p" onClick={() => setSheet('feedback')}>{`Chưa sửa ${fbCounts.unfixed}`}</button>}
            {fbCounts.need > 0 && <button type="button" className="bpill w" onClick={() => setSheet('feedback')}>{`Cần gắn lại ${fbCounts.need}`}</button>}
          </div>
        </div>
      )}
      <div ref={paragraphsRef} className="mr-paragraphs" style={{ fontFamily: display.family === 'serif' ? serif : 'var(--font-sans)', fontSize: display.size, lineHeight: display.spacing } as CSSProperties} onMouseUp={captureSelection}
        onTouchStart={(event) => { const touch = event.touches[0]; swipeStart.current = touch ? { x: touch.clientX, y: touch.clientY } : null }}
        onTouchEnd={(event) => {
          const start = swipeStart.current; swipeStart.current = null
          const touch = event.changedTouches[0]
          if (start && touch && current !== null) {
            const next = swipeChapter(chapters ?? [], current, touch.clientX - start.x, touch.clientY - start.y)
            if (next !== null) { cancelPress(); suppressClick.current = true; pickChapter(next); return }
          }
          captureSelection()
        }}>
        {blocks.map((block, blockIndex) => {
          if (block.kind !== 'paragraph') return <div key={blockIndex} style={{ padding: '6px 10px' }}><ChapterContent blocks={[block]} unmatchedRevisions={[]} showDiff={false} plain={renderPlain} /></div>
          paragraph++
          const index = paragraph
          const active = selected.paragraph === index
          const blockText = parseInline(block.text).map((token) => token.text).join('')
          const { marks, whole } = paragraphMarks(visibleComments, blockText, index, openId)
          const isCand = candParagraphs.includes(index)
          const pick = () => {
            if (!writable) return
            if (suppressClick.current) { suppressClick.current = false; return }
            if (relinkId) { setRelinkHov(relinkHov === index ? null : index); return }
            if (!window.getSelection()?.isCollapsed) { captureSelection(); return }
            setSelected(active ? emptySelection : paragraphAnchor(paragraphTexts, paragraphTexts[index], index))
          }
          return <div key={blockIndex}><div className={`mr-paragraph mp${writable ? ' selectable' : ''}${active && !relinkId ? ' selected' : selected.paragraph !== null ? ' dim' : ''}${whole ? ' whole' : ''}${isCand ? ' cand' : ''}${relinkId ? ' pickable' : ''}${relinkHov === index ? ' hov' : ''}`} data-reader-paragraph={index} role={writable ? 'button' : undefined} tabIndex={writable ? 0 : undefined} aria-pressed={writable ? active : undefined} aria-label={writable ? `Chọn đoạn này` : undefined} onClick={pick} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); pick() } }} onContextMenu={(event) => { if (writable && !relinkId) { event.preventDefault(); openComment(paragraphAnchor(paragraphTexts, paragraphTexts[index], index)) } }} onPointerDown={(event) => { if (!writable || relinkId) return; cancelPress(); pressedAt.current = { x: event.clientX, y: event.clientY }; longPress.current = setTimeout(() => { suppressClick.current = true; openComment(paragraphAnchor(paragraphTexts, paragraphTexts[index], index)) }, 550) }} onPointerMove={(event) => { if (pressedAt.current && Math.hypot(event.clientX - pressedAt.current.x, event.clientY - pressedAt.current.y) > 10) cancelPress() }} onPointerUp={cancelPress} onPointerCancel={cancelPress}>
            <span className="mdots" aria-hidden="true">{marks.map((mark) => <span key={mark.comment.id} className={`gdot ${mark.moved ? 'mv' : mark.tone}`} />)}{whole && marks.length === 0 && <span className="gdot p" />}</span>
            {isCand && <span className="cbadge">{`Gợi ý ${candParagraphs.indexOf(index) + 1}`}</span>}
            {marks.length > 0 || whole ? <MobileMarked text={blockText} marks={marks} render={renderPlain} onOpen={(id) => { setOpenId(id); setSheet('feedback') }} /> : <ChapterContent blocks={[block]} unmatchedRevisions={[]} showDiff={false} plain={renderPlain} />}
            {relinkId && relinkHov === index && <button type="button" className="mattach" onClick={(event) => { event.stopPropagation(); pickRelink(index) }}><Icon name="link-2" size={15} />Gắn góp ý vào đoạn này</button>}
          </div>{active && writable && !relinkId && <div className="mr-selection"><button className="m-chip on" onClick={() => openComment(selected)}><Icon name="edit-2" size={14} />Góp ý đoạn này</button><button className="m-chip" onClick={() => { void copyParagraph() }}><Icon name="copy" size={14} />Chép</button></div>}</div>
        })}
      </div>
      <div className="mr-chapter-end">
        <div className="mr-end-label"><span />Hết chương {current}<span /></div>
        {nextChapter && <button className="mr-next-chapter" disabled={!chapterReadable(nextChapter)} onClick={() => pickChapter(nextChapter.chapter)}>
          {nextState === 'writing' ? <span className="m-orb live" aria-hidden="true" /> : <Icon name="book-1" size={26} />}
          <span className="mr-next-copy"><small>{nextChapterCaption(nextState!, nextChapter.words, writingPercent(snapshot?.flow))}</small><strong>{nextChapter.chapter} · {nextChapter.title}</strong></span>
          <Icon name="arrow-right-2" size={18} />
        </button>}
        <span className="mr-swipe-hint">Vuốt sang trái/phải để đổi chương</span>
      </div>
      {writable && <>{comments.status === 'error' && <div className="mr-retry" role="alert">Không tải được góp ý.<button className="m-button secondary" onClick={() => { void comments.refresh() }}>Thử lại</button></div>}{!pendingGate && <div className="mr-selection"><button className="m-chip" onClick={() => openComment(emptySelection)}><Icon name="edit-2" size={14} />Góp ý cả chương</button></div>}</>}
    </>}
    {chapters?.length === 0 && <div className="mr-notice">Chưa có chương nào. Chương mới sẽ xuất hiện tại đây khi Người viết hoàn thành.</div>}</div>
  </MobilePage><PanelRefreshBar resourceKeys={[keys.story(project, story)]} updatedAt={updatedAt} /><div ref={progressRef} className="mr-progress" role="progressbar" aria-label="Tiến độ đọc" aria-valuemin={0} aria-valuemax={100} aria-valuenow={0}><span style={{ transform: 'scaleX(0)', transformOrigin: 'left', width: '100%' }} /></div>
  <ScrollTopButton key={current} target={scrollerRef} focusTarget={headingRef} label={t.scrollTop.chapter} progress size="lg"
    hideWithBars={immersive} hidden={!!sheet || !!person || !!menuChapter || keyboard > 0 || selected.paragraph !== null}
    style={{ bottom: pendingGate && writable && selected.paragraph === null ? 166 : immersive ? 24 : 100 }} />
  {/* §AI — viên Góp ý nổi (M2b): vàng khi cần gắn lại, tím khi chờ gửi; toast viết lại. */}
  {writable && visibleComments.length > 0 && !sheet && !person && !menuChapter && !relinkId && (
    <button type="button" className={`fpill${fbBadge?.tone === 'w' ? ' w' : ''}`}
      onClick={() => setSheet('feedback')} aria-label="Mở góp ý">
      <span className="pdot" aria-hidden="true" style={{ background: fbBadge?.tone === 'w' ? 'var(--color-warn)' : fbBadge?.tone === 'p' ? 'var(--color-accent)' : fbCounts.sent > 0 ? 'var(--fb-s-fg)' : 'var(--color-fg-4)' }} />
      {fbBadge?.tone === 'w' ? `Góp ý · ${fbBadge.n} cần gắn lại` : fbBadge?.tone === 'p' ? `Góp ý · ${fbBadge.n} chờ gửi` : fbCounts.sent > 0 ? `Góp ý · ${fbCounts.sent} đã gửi` : 'Góp ý'}
      <Icon name="arrow-up" size={15} style={{ color: 'var(--color-fg-4)' }} />
    </button>
  )}
  {reanchorToastMsg && (
    <div className="mfb-toast" role="status"><Icon name="magic-star" size={15} /><span>{reanchorToastMsg}</span><button type="button" onClick={() => { setReanchorToastMsg(null); setSheet('feedback') }}>Xem</button></div>
  )}
  <FeedbackSheet open={sheet === 'feedback'} onClose={closeSheet} list={visibleComments} chapter={current ?? 0} gate={pendingGate}
    relinkId={relinkId} openId={openId} onToggle={(id) => setOpenId(openId === id ? null : id)} onAction={onFeedbackAction}
    onSend={() => { void review.run() }} sendBusy={review.busy} busyId={patching} />
  <CommentSheet open={sheet === 'comment' && writable} onClose={closeSheet} selected={selected} chips={chips} setChips={setChips} note={note} setNote={(text) => { setNote(text); setDraft('active', text) }} onSubmit={() => { void sendComment() }} busy={comments.add.busy || rewrite.busy || queueing} error={queueError || (rewriteChapter !== null ? rewrite.error : comments.add.error)} keyboard={keyboard} rewriteChapter={rewriteChapter} offline={!connection.writable} />
  <BottomSheet open={sheet === 'font'} onClose={closeSheet} label="Hiển thị" height="auto">
    <div className="mr-sheet-content mr-font">
      <h2 className="mr-sheet-title">Hiển thị</h2>
      <div className="mr-font-range"><span style={{ fontSize: 14, color: '#A0A0AB' }}>A</span><input type="range" min={15} max={23} step={1} value={display.size} style={{ '--p': `${Math.round((display.size - 15) / 8 * 100)}%` } as CSSProperties} onChange={(event) => changeDisplay({ size: Number(event.target.value) })} aria-label="Cỡ chữ" /><span style={{ fontSize: 22 }}>A</span></div>
      <Segmented label="Kiểu chữ" value={display.family} onChange={(family) => changeDisplay({ family: family as ReaderDisplay['family'] })} columns={2} size={44} radius={11} thumbStyle={{ boxShadow: 'none' }}
        style={{ display: 'grid', gap: 4, padding: 4, borderRadius: 14, background: 'var(--color-surface-0)', border: 'none' }}
        options={[{ id: 'serif', label: 'Có chân', style: { fontFamily: serif, fontSize: 15 } }, { id: 'sans', label: 'Không chân', style: { fontSize: 15 } }]} />
      <Segmented label="Giãn dòng" value={String(display.spacing)} onChange={(spacing) => changeDisplay({ spacing: Number(spacing) })} columns={3} size={44} radius={11} thumbStyle={{ boxShadow: 'none' }}
        style={{ display: 'grid', gap: 4, padding: 4, borderRadius: 14, background: 'var(--color-surface-0)', border: 'none' }}
        options={[[1.55, 'Gọn'], [1.8, 'Vừa'], [2.05, 'Thoáng']].map(([spacing, optionLabel]) => ({ id: String(spacing), label: optionLabel as string, style: { fontSize: 14 } }))} />
    </div>
  </BottomSheet>
  <BottomSheet open={sheet === 'chapters'} onClose={closeSheet} label={chaptersTitle} height={620}><div className="mr-chapters-sheet"><div className="mr-chapters-head"><h2 className="mr-sheet-title">{chaptersTitle}</h2><span>{chapters?.filter((item) => item.completed).length ?? 0}/{chapters?.length ?? 0} xong</span></div><div className="mr-chapters-scroll"><ChapterList chapters={chapters} current={current} snapshot={snapshot} onPick={pickChapter} onMore={setMenuChapter} /></div></div></BottomSheet>
  <BottomSheet open={person !== null} onClose={() => setPerson(null)} label={quick ? `${quick.name} tại chương ${current ?? ''}` : 'Xem nhanh nhân vật'} height="auto">
    {quick && <div className="mr-person">
      <div className="mr-person-head">
        <CharAvatar name={quick.name} ini={quick.ini} kind={quick.kind} hidden={quick.hidden} size={44} />
        <span className="mr-person-title"><span>{quick.name}</span><span>{quick.role} · có mặt từ chương {quick.fromChapter}</span></span>
      </div>
      <p className="mr-person-desc">{quick.desc}</p>
      {quick.rows.length > 0 && <div className="mr-person-rows">
        {quick.rows.map((row, index) => <div className="mr-person-row" key={index}>
          <span className="mr-person-key">{row.label}</span>
          <span className="mr-person-value">{row.value}{row.note && <span className="mr-person-note"> · {row.note}</span>}</span>
        </div>)}
      </div>}
      <span className="mr-person-info"><Icon name="info-circle" size={14} />Chỉ hiện những gì đã xảy ra tới chương đang đọc, không lộ chương sau.</span>
      <div className="mr-person-actions">
        <button className="mr-person-open" type="button" onClick={() => { setPerson(null); navigate(paths.brain(project, story, current)) }}>Mở Não truyện</button>
        <button className="m-button" type="button" onClick={() => setPerson(null)}>Đọc tiếp</button>
      </div>
    </div>}
  </BottomSheet>
  <ChapterActions project={project} story={story} chapter={menuChapter} writable={writable} onClose={() => setMenuChapter(null)} onRead={pickChapter} onListen={listenChapter} onRewrite={requestRewrite} />
  </div>
}

/** Vết gạch chân trong đoạn (§AI): tách vùng trích đã giải rồi render từng phần qua `render`
 *  (để giữ tên nhân vật gạch chấm); mốc mất trích → .mp bọc cả đoạn. */
function MobileMarked({ text, marks, render, onOpen }: {
  text: string; marks: ParagraphMark[]
  render: (value: string) => ReactNode
  onOpen: (id: string) => void
}) {
  const parts: { text: string; mark?: ParagraphMark }[] = []
  let at = 0
  for (const mark of marks) {
    if (mark.at > at) parts.push({ text: text.slice(at, mark.at) })
    parts.push({ text: text.slice(mark.at, mark.at + mark.len), mark })
    at = mark.at + mark.len
  }
  if (at < text.length) parts.push({ text: text.slice(at) })
  return <p data-prose="" style={{ margin: 0 }}>{parts.map((part, index) => part.mark ? (
    <mark key={index} className={`mk ${part.mark.tone}${part.mark.moved ? ' mv' : ''}${part.mark.open ? ' on' : ''}`}
      onClick={(event) => { event.stopPropagation(); onOpen(part.mark!.comment.id) }}>{render(part.text)}</mark>
  ) : <span key={index}>{render(part.text)}</span>)}</p>
}
