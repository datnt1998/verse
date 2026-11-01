// Trình đọc & duyệt (#/p/:p/s/:s/read/:chapter?) — Reader.dc.html.
// Cột chương 236 · Toolbar + Literata 66ch + composer · Thẩm duyệt/Xuất 360.
// Đọc trực tiếp qua API (không cần worker); khoá truyện hoặc vai trò reader → chỉ đọc.
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMediaQuery, useShellChrome } from '../../shell'
import { navigate, paths, useRoute } from '../../router'
import { approveWithComments, canManageProject, keepReviewItem, unkeepReviewItem } from '../../api/client'
import type { ParagraphComment, ReviewDecision, ReviewItem } from '../../api/types'
import { getReaderSize, setReaderSize } from '../../api/storage'
import { Tabs } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { ChapterPicker } from './ChapterPicker'
import { ReaderToolbar } from './ReaderToolbar'
import { ReaderBody } from './ReaderBody'
import { ReaderComposer } from './ReaderComposer'
import { ReviewPanel } from './ReviewPanel'
import { ExportPanel } from './ExportPanel'
import { copy } from './copy'
import { relevantForeshadow } from './logic'
import {
  useChapterContent, useChapterExtras, useChapters, useExports, useForeshadow, useStoryAccess,
} from './useReaderData'

import { StaleBanner } from '@noverse/ui'
import { canChangeComment, useParagraphComments, type ParagraphSelection } from './comments'
import { useSession } from '../../api/session'
import { events, keys, useAction } from '../../data'
import { attentionBadge, clipWords, commentBody, feedbackCounts, reanchoredOf, reanchorBanner, reanchorToast, type ReanchoredFrame } from './feedback'
import { FeedbackPanel } from './FeedbackPanel'
import { RelinkBar, type FeedbackAction } from './FeedbackGroups'
import type { ParagraphCommentPatch } from '../../api/types'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
type ReaderTab = 'review' | 'feedback' | 'export'
export function ReaderScreen({ project, story, chapter }: { project: string; story: string; chapter: number | null }) {
  const narrow = useMediaQuery('(max-width: 1099px)')
  // <760px có thanh tab đáy; 760–1099 thanh nav nằm trong header nên không trừ lần hai.
  const mobile = useMediaQuery('(max-width: 759px)')
  const shellChrome = useShellChrome()
  const paneRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const { chapters, outline, error: chaptersError, reload: reloadChapters } = useChapters(project, story)
  const latestChapter = chapters?.filter((item) => item.completed).at(-1)?.chapter ?? chapters?.at(-1)?.chapter ?? null
  const current = chapter ?? latestChapter
  const currentInfo = chapters?.find((item) => item.chapter === current) ?? null
  // Câu 11: “Thêm ghi chú cho Người viết” mở ngăn Góp ý của đúng chương (`?note=1`),
  // con trỏ vào ô góp ý cả chương — không dựng ô mới. Dùng xong thì trả URL về bình thường.
  const { search } = useRoute()
  const focusNote = search.get('note') === '1'
  useEffect(() => {
    if (focusNote && current !== null) { setTab('feedback'); navigate(paths.reader(project, story, current), true) }
  }, [focusNote, current, project, story])

  // Đổi chương → về đầu khung cuộn (mobile cuộn cả trang trong pane, desktop là cột giữa).
  useEffect(() => { paneRef.current?.scrollTo({ top: 0 }) }, [current])

  const [sourceChoice, setSourceChoice] = useState<{ project: string; story: string; chapter: number | null; source: 'committed' | 'draft' } | null>(null)
  // Chương chưa nộp mở bản nháp; lựa chọn thủ công chỉ áp dụng cho chương đang xem.
  const source = sourceChoice?.project === project && sourceChoice.story === story && sourceChoice.chapter === current
    ? sourceChoice.source : currentInfo && !currentInfo.completed ? 'draft' : 'committed'
  const setSource = (next: 'committed' | 'draft') => setSourceChoice({ project, story, chapter: current, source: next })

  const [size, setSize] = useState(getReaderSize())
  const zoom = (delta: number) => {
    const next = Math.min(24, Math.max(14, size + delta))
    setSize(next)
    setReaderSize(next)
  }

  const [showDiff, setShowDiff] = useState(true)
  const [tab, setTab] = useState<ReaderTab>('review')

  // §AI — góp ý theo đoạn của chương đang xem: ngăn Góp ý, vết gạch chân, gắn lại, banner viết lại.
  const comments = useParagraphComments(project, story, current)
  // Thẻ góp ý của người khác: cộng tác viên chỉ đọc (nút sửa/xoá ẩn), chủ dự án
  // và quản trị viên vẫn điều khiển được — cùng ranh giới với máy chủ.
  const { user } = useSession()
  const managesComments = !!user && canManageProject(user, project)
  const commentList = comments.data?.comments ?? []
  const counts = feedbackCounts(commentList)
  const outcome = counts.fixed + counts.unfixed + counts.need
  const [openId, setOpenId] = useState<string | null>(null)
  const [relinkId, setRelinkId] = useState<string | null>(null)
  const [relinkHov, setRelinkHov] = useState<number | null>(null)
  const [peek, setPeek] = useState<number | null>(null)
  const [bannerHidden, setBannerHidden] = useState(false)
  const [patching, setPatching] = useState<string | null>(null)
  const relinkTarget = commentList.find((comment) => comment.id === relinkId)
  const relinkCandidates = relinkTarget ? (relinkTarget.candidates ?? []).map((candidate) => candidate.paragraph) : []
  const approve = useAction(() => approveWithComments(project, story, current!), {
    success: currentInfo && !currentInfo.completed ? 'Đã duyệt và gửi góp ý cho Người viết' : `Đã gửi · Người viết sẽ viết lại chương ${current}`,
    invalidate: [keys.story(project, story)],
  })
  // Chương đang xem vừa được viết lại theo góp ý → banner + toast “Xem” (03b fb:after).
  useEffect(() => events.subscribe('comments.reanchored', (data) => {
    if (current === null) return
    const event = reanchoredOf(data as ReanchoredFrame, project, story, current)
    if (!event) return
    setBannerHidden(false)
    toast(reanchorToast(event), 'info', { label: 'Xem', onClick: () => { setTab('feedback'); setRelinkId(null) } })
  }), [project, story, current])
  useEffect(() => { setOpenId(null); setRelinkId(null); setRelinkHov(null); setPeek(null); setBannerHidden(false) }, [current])
  const patch = async (id: string, body: ParagraphCommentPatch) => {
    setPatching(id)
    try { await comments.patch.run(id, body) } finally { setPatching(null) }
  }
  const onFeedbackAction = async (action: FeedbackAction) => {
    const id = action.comment.id
    if (action.kind === 'remove') { void comments.remove(id); if (openId === id) setOpenId(null); return }
    if (action.kind === 'resolve') { void patch(id, { action: 'resolve' }); return }
    if (action.kind === 'reopen') { void patch(id, { action: 'reopen' }); return }
    if (action.kind === 'resend') { void patch(id, { action: 'resend_whole' }); setOpenId(id); return }
    if (action.kind === 'relink') { setRelinkId(id); setTab('feedback'); window.getSelection()?.removeAllRanges(); return }
    // Gắn vào đây từ đoạn gợi ý (unsure): PATCH relink rồi mở lại thẻ.
    await patch(id, { action: 'relink', paragraph: action.paragraph, quote: action.quote })
    setOpenId(id)
  }
  const pickParagraph = (index: number) => {
    if (!relinkId) return
    const quote = selection.paragraph === index ? selection.quote : undefined
    const id = relinkId
    setRelinkId(null); setRelinkHov(null)
    void patch(id, { action: 'relink', paragraph: index, quote }).then(() => setOpenId(id))
  }
  const [composer, setComposer] = useState('')
  const [selection, setSelection] = useState<ParagraphSelection>({ paragraph: null, quote: '' })
  const composerRef = useRef<HTMLInputElement>(null)
  useEffect(() => { setComposer(''); setSelection({ paragraph: null, quote: '' }) }, [current])

  const { content, loading, error, reload, refreshing, updatedAt } = useChapterContent(project, story, current, source)
  const extras = useChapterExtras(project, story, current)
  const exportsState = useExports(project, story)
  const foreshadow = useForeshadow(project, story)
  const { locked, writable, storyTitle } = useStoryAccess(project, story)
  useUnsavedGuard(!!composer.trim())

  const relatedForeshadow = useMemo(
    () => (foreshadow.entries ? relevantForeshadow(foreshadow.entries, extras.facts, current ?? 0) : null),
    [foreshadow.entries, extras.facts, current],
  )

  const pickChapter = (next: number) => {
    setComposer('')
    navigate(paths.reader(project, story, next))
  }
  const goLatest = () => pickChapter(latestChapter ?? 1)
  const requestFix = (text: string) => {
    setComposer(text)
    composerRef.current?.focus()
  }
  // GAP-47 — “Giữ nguyên” một Cảnh báo: lưu lý do vào quyết định thẩm duyệt của mục.
  const keepWarning = async (item: ReviewItem, reason: string): Promise<ReviewDecision | null> => {
    try {
      const decision = await keepReviewItem(project, story, item.id, reason)
      toast(t.review.keepSaved)
      return decision
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.review.keepFailed)
      return null
    }
  }
  const unkeepWarning = async (item: ReviewItem): Promise<boolean> => {
    try {
      await unkeepReviewItem(project, story, item.id)
      toast(t.review.unkeepSaved)
      return true
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.review.unkeepFailed)
      return false
    }
  }

  const title = content?.title || currentInfo?.title || (current !== null ? `Chương ${current}` : '')
  const words = content?.words ?? currentInfo?.words ?? 0
  const canDiff = !!content?.text && (extras.revisions ?? []).some((revision) => !!revision.old && content.text.includes(revision.old))

  if (chaptersError?.status === 403) {
    return (
      <div style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="shield-security" title={copy.noPermission}
          hint={chaptersError.message === copy.noPermission ? undefined : chaptersError.message}
          action={<Button size="sm" onClick={reloadChapters}>{copy.retry}</Button>} />
      </div>
    )
  }

  const composerNode = current !== null ? (
    <ReaderComposer
      project={project} story={story} chapter={current} writable={writable} locked={locked}
      value={composer} onChange={setComposer} inputRef={composerRef} narrow={narrow}
    />
  ) : null

  return (
    <div style={{
      flex: 1, minWidth: 0,
      ...(narrow
        // <1100px: cột đúng viewport — picker/toolbar/thân/thẩm duyệt cuộn trong pane,
        // composer neo đáy ngoài pane nên không phủ lên thân chương (03-tokens: ô chat sticky).
        ? { height: `calc(100dvh - ${shellChrome.header + (mobile ? shellChrome.tabbar : 0)}px)`, display: 'flex', flexDirection: 'column', overflow: 'hidden' }
        : { minHeight: 0, display: 'flex' }),
    }}>
      <div ref={paneRef} className={narrow ? 'scroll' : undefined} style={narrow
        ? { flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }
        : { flex: 1, minWidth: 0, minHeight: 0, display: 'flex' }}>
        <ChapterPicker chapters={chapters} outline={outline} current={current}
          onPick={pickChapter} error={chaptersError?.message ?? null} onRetry={reloadChapters} narrow={narrow} />

        <main style={{
          flex: narrow ? '0 0 auto' : '1 1 520px', width: narrow ? '100%' : undefined, minWidth: 0,
          minHeight: narrow ? undefined : 0, overflow: narrow ? 'visible' : 'hidden',
          display: 'flex', flexDirection: 'column',
        }}>
          {current !== null ? (
            <>
              <PanelRefreshBar resourceKeys={[[ 'story', project, story, 'chapter', current, source ]]} updatedAt={updatedAt} />
              {!!content && <StaleBanner error={error} onRetry={reload} />}
              <ReaderToolbar
                chapter={current} title={title} words={words} headingRef={headingRef}
                source={source} onSource={setSource}
                diff={showDiff} onDiff={() => setShowDiff((value) => !value)} canDiff={canDiff}
                size={size} onSize={zoom}
              />
              {/* §AI — chế độ gắn lại (03b fb:relink): dải vàng + đoạn pickable trong bài. */}
              {relinkId && <RelinkBar body={clipWords(commentBody(relinkTarget!) || 'góp ý', 60)} hasCandidates={relinkCandidates.length > 0} onCancel={() => { setRelinkId(null); setRelinkHov(null) }} />}
              {/* §AI — banner “Người viết đang sửa” (Đã gửi) và “Đã viết lại” (03b fb:after). */}
              {counts.sent > 0 && (
                <div className="fbban s" role="status"><i className="fb-spin" aria-hidden="true" /><span><b>{`Người viết đang sửa chương ${current} theo ${counts.sent} góp ý.`}</b>{' Bạn vẫn đọc được bản hiện tại; chương sẽ tự thay khi sửa xong.'}</span></div>
              )}
              {outcome > 0 && !bannerHidden && !relinkId && (
                <div className="fbban a" role="status">
                  <Icon name="magic-star" size={17} />
                  <div className="fbban-body">
                    <span><b>{reanchorBanner(current, counts)}</b> — kiểm tra kết quả trong ngăn Góp ý.</span>
                    <div className="bpills">
                      {counts.fixed > 0 && <button type="button" className="bpill f" onClick={() => setTab('feedback')}>{`Đã sửa ${counts.fixed}`}</button>}
                      {counts.unfixed > 0 && <button type="button" className="bpill p" onClick={() => setTab('feedback')}>{`Chưa sửa ${counts.unfixed}`}</button>}
                      {counts.need > 0 && <button type="button" className="bpill w" onClick={() => setTab('feedback')}>{`Cần gắn lại ${counts.need}`}</button>}
                    </div>
                  </div>
                  <button type="button" className="fbban-x" aria-label="Đóng" onClick={() => setBannerHidden(true)}><Icon name="add" size={14} style={{ transform: 'rotate(45deg)' }} /></button>
                </div>
              )}
              <ReaderBody key={current} focusTarget={headingRef}
                chapter={current} content={content} loading={loading} error={error} source={source}
                size={size} showDiff={showDiff && canDiff} revisions={extras.revisions}
                onSelection={setSelection}
                onRetry={reload} onViewFinal={() => setSource('committed')} onGoLatest={goLatest} narrow={narrow}
                comments={commentList}
                openId={openId}
                onOpenComment={(id) => { setOpenId(id); setTab('feedback') }}
                relink={relinkId ? { candidates: relinkCandidates, peek, hov: relinkHov, onHover: setRelinkHov, onPick: pickParagraph } : null}
                attach={relinkId && selection.paragraph !== null && selection.quote ? { paragraph: selection.paragraph, quote: selection.quote, onPick: () => pickParagraph(selection.paragraph!) } : null}
              />
              {!narrow && composerNode}
            </>
          ) : chapters === null ? (
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 18, padding: 40 }}>
              {[92, 100, 86, 96, 74].map((width, index) => <Skeleton key={index} width={`${width}%`} height={18} />)}
            </div>
          ) : (
            <div className={narrow ? undefined : 'scroll'}
              style={{ flex: 1, minHeight: 0, padding: 28, overflowY: narrow ? 'visible' : undefined }}>
              <EmptyState icon="book-1" title={chaptersError ? copy.loadError : copy.noChapters}
                hint={chaptersError ? chaptersError.message : copy.noChaptersHint}
                action={chaptersError ? <Button size="sm" onClick={reloadChapters}>{copy.retry}</Button> : undefined} />
            </div>
          )}
        </main>

        <aside aria-label="Duyệt · góp ý · xuất" style={{
          flex: narrow ? '0 0 auto' : '0 0 360px', width: narrow ? '100%' : undefined, minWidth: 0,
          maxWidth: narrow ? 'none' : 360,
          minHeight: narrow ? undefined : 0, display: 'flex', flexDirection: 'column',
          borderLeft: narrow ? 'none' : '1px solid var(--line-1)',
          borderTop: narrow ? '1px solid var(--line-1)' : undefined,
        }}>
          <PanelRefreshBar resourceKeys={tab === 'feedback' ? [['story', project, story, 'comments']] : tab === 'review' ? [['story', project, story, 'reviews'], ['story', project, story, 'facts']] : [['story', project, story, 'exports']]} updatedAt={tab === 'review' ? extras.updatedAt : exportsState.updatedAt} />
          <Tabs
            label="Duyệt · góp ý · xuất"
            columns={3}
            value={tab}
            onChange={(id) => setTab(id as ReaderTab)}
            style={{ margin: '12px 12px 0' }}
            items={[
              { id: 'review', label: copy.reviewTab, icon: <Icon name="shield-tick-bulk" size={15} /> },
              { id: 'feedback', label: <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>Góp ý{attentionBadge(counts) && <span className={`tbadge ${attentionBadge(counts)!.tone}`}>{attentionBadge(counts)!.n}</span>}</span>, icon: <Icon name="messages-2" size={15} /> },
              { id: 'export', label: copy.exportTab, icon: <Icon name="export-curve" size={15} /> },
            ]}
          />
          <div role="tabpanel" aria-label={tab === 'review' ? copy.reviewTab : tab === 'feedback' ? 'Góp ý' : copy.exportTab}
            className={narrow ? undefined : 'scroll'}
            style={{
              flex: 1, minHeight: 0, padding: tab === 'feedback' ? 0 : 12, display: 'flex', flexDirection: 'column', gap: 10,
              // Dock việc chạy nền (52 px + 16 px lề) nổi ở góc dưới phải trên cột này: chừa chỗ để
              // dòng cuối của thẻ thẩm duyệt không bị đè (JobsDock đặt --dock-dot-space khi pill/chấm hiện).
              paddingBottom: tab === 'feedback' ? 0 : 'calc(12px + var(--dock-dot-space, 0px))',
              overflowY: narrow ? 'visible' : undefined,
            }}>
            {tab === 'feedback' && current !== null && (
              <FeedbackPanel key={current} comments={comments}
                selection={selection} setSelection={setSelection}
                relinkId={relinkId} openId={openId}
                onToggle={(id) => { const next = openId === id ? null : id; setOpenId(next); setPeek(null) }}
                onAction={onFeedbackAction}
                onPeekCandidate={setPeek}
                onSend={() => { void approve.run() }} sendBusy={approve.busy}
                chapter={current} gate={!!currentInfo && !currentInfo.completed}
                busyId={patching} canChange={(comment) => canChangeComment(comment.author, user, managesComments)} focus={focusNote}
                error={null} onRetry={() => { void comments.refresh() }}
              />
            )}
            {tab === 'review' && <StaleBanner error={extras.error} onRetry={extras.reload} />}
            {tab === 'export' ? (
              <ExportPanel
                project={project} story={story} storyTitle={storyTitle} chapters={chapters ?? []}
                data={exportsState.data} loading={exportsState.loading} error={exportsState.error}
                onReload={exportsState.reload} writable={writable}
              />
            ) : tab === 'review' ? (
              <ReviewPanel
                chapter={current ?? 0}
                review={extras.review} plan={extras.plan} facts={extras.facts} revisions={extras.revisions}
                foreshadow={relatedForeshadow} error={extras.error} onRetry={extras.reload}
                writable={writable} onRequestFix={requestFix} onKeep={keepWarning} onUnkeep={unkeepWarning}
              />
            ) : null}
          </div>
        </aside>
      </div>
      {narrow && composerNode}
    </div>
  )
}
