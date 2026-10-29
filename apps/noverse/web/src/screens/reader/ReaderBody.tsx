// ReaderBody — Literata 66ch, render markdown nhẹ + diff thay đổi gần nhất.
// Gói AI: mỗi đoạn văn (canonical, trước khi diff tách) nằm trong .pw để có gutter dot,
// gạch chân trích đã giải (anchor), viền đoạn gợi ý và chế độ gắn lại (03b fb).
import { useLayoutEffect, useMemo, useRef } from 'react'
import type { CSSProperties, RefObject } from 'react'
import type { ApiError } from '../../api/client'
import type { ChapterText, ParagraphComment, RevisionEntry } from '../../api/types'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { copy } from './copy'
import { applyRevisions, ChapterContent, parseChapterText, parseInline, type ChapterBlock, type DiffBlock } from '../../lib/markdown'
import { selectionInReader, type ParagraphSelection } from './comments'
import { paragraphMarks, type ParagraphMark } from './feedback'
import { SlowLoadingNotice } from '@noverse/ui'
import { ScrollTopButton } from '@noverse/ui'
import { t } from '../../i18n/vi'

/** Khối hiển thị: một slot đoạn canonical (có thể nhiều fragment quanh diff) hoặc khối khác. */
type BodyItem =
  | { type: 'para'; para: number; blocks: (ChapterBlock | DiffBlock)[] }
  | { type: 'block'; block: ChapterBlock | DiffBlock }

/** Nhóm khối đã chèn diff về đoạn canonical: fragment sau diff thuộc cùng đoạn trước đó;
 *  hai đoạn liền nhau (para rồi para) là hai đoạn khác nhau. */
export function bodyItems(blocks: (ChapterBlock | DiffBlock)[]): BodyItem[] {
  const items: BodyItem[] = []
  let para = -1
  let open: (ChapterBlock | DiffBlock)[] | null = null
  for (let index = 0; index < blocks.length; index += 1) {
    const block = blocks[index]
    const previous = index > 0 ? blocks[index - 1] : undefined
    if (block.kind === 'paragraph') {
      if (previous?.kind === 'diff' && open) open.push(block)
      else { para += 1; const slot: (ChapterBlock | DiffBlock)[] = [block]; items.push({ type: 'para', para, blocks: slot }); open = slot }
    } else if (block.kind === 'diff') {
      if (open) open.push(block)
      else items.push({ type: 'block', block })
    } else {
      items.push({ type: 'block', block })
      open = null
    }
  }
  return items
}

const tokenText = (block: ChapterBlock & { kind: 'paragraph' }) => parseInline(block.text).map((token) => token.text).join('')

/** Gạch chân trong đoạn theo mốc neo đã giải (§AI): tách vùng trích rồi render inline từng phần. */
function MarkedText({ text, marks, onOpen }: { text: string; marks: ParagraphMark[]; onOpen: (id: string) => void }) {
  if (marks.length === 0) return <>{parseInline(text).map((token, index) => <PlainToken key={index} t={token} />)}</>
  const parts: { text: string; mark?: ParagraphMark }[] = []
  let at = 0
  for (const mark of marks) {
    if (mark.at > at) parts.push({ text: text.slice(at, mark.at) })
    parts.push({ text: text.slice(mark.at, mark.at + mark.len), mark })
    at = mark.at + mark.len
  }
  if (at < text.length) parts.push({ text: text.slice(at) })
  return <>{parts.map((part, index) => part.mark ? (
    <mark key={index} className={`mk ${part.mark.tone}${part.mark.moved ? ' mv' : ''}${part.mark.open ? ' on' : ''}`}
      onClick={() => onOpen(part.mark!.comment.id)}>{parseInline(part.text).map((token, tokenIndex) => <PlainToken key={tokenIndex} t={token} />)}</mark>
  ) : parseInline(part.text).map((token, tokenIndex) => <PlainToken key={`${index}-${tokenIndex}`} t={token} />))}</>
}
const PlainToken = ({ t }: { t: { entity?: string; text: string } }) => (t.entity && t.entity !== 'break' ? <span className={`nl-${t.entity}`}>{t.text}</span> : <>{t.text}</>)

export function ReaderBody({ chapter, content, loading, error, source, size, showDiff, revisions, onRetry, onViewFinal, onGoLatest, narrow, onSelection, focusTarget, comments, openId, onOpenComment, relink, attach }: {
  chapter: number
  content: ChapterText | null
  loading: boolean
  error: ApiError | null
  source: 'committed' | 'draft'
  size: number
  showDiff: boolean
  revisions: RevisionEntry[] | null
  onRetry: () => void
  onViewFinal: () => void
  onGoLatest: () => void
  narrow: boolean
  onSelection?: (selection: ParagraphSelection) => void
  focusTarget?: RefObject<HTMLHeadingElement | null>
  /** §AI — góp ý của chương đang xem: vết gạch chân/gutter dot theo mốc neo đã giải. */
  comments?: ParagraphComment[]
  /** Thẻ đang mở → mở viền đoạn + tô vết; cũng định đoạn gợi ý (unsure). */
  openId?: string | null
  /** Bấm vào vết gạch chân → mở thẻ trong ngăn Góp ý. */
  onOpenComment?: (id: string) => void
  /** Chế độ gắn lại (03b fb:relink): đoạn pickable (vàng), hov theo chuột, nhấn để gắn. */
  relink?: { candidates: number[]; peek: number | null; hov: number | null; onHover: (index: number | null) => void; onPick: (index: number) => void } | null
  /** Đã quét câu trong đoạn đang hov → nút “Gắn vào “câu…”” (§AI chuột + quét). */
  attach?: { paragraph: number; quote: string; onPick: () => void } | null
}) {
  const parsed = useMemo(() => (content?.text ? parseChapterText(content.text) : []), [content?.text])
  const diffed = useMemo(
    () => (showDiff && revisions && revisions.length > 0 ? applyRevisions(parsed, revisions) : { blocks: parsed, unmatched: [] as RevisionEntry[] }),
    [parsed, revisions, showDiff],
  )
  const missing = error?.status === 404
  // Bản nháp chưa có (backend trả 404) không phải “chương không tồn tại” — hiện trạng thái riêng.
  const draftMissing = missing && source === 'draft'
  const missingChapter = missing && !draftMissing
  const emptyDraft = draftMissing || (!loading && !error && content !== null && !content.text && source === 'draft')
  const emptyChapter = !loading && !error && content !== null && !content.text && source === 'committed'

  // Desktop: chính bài đọc là khung cuộn — đổi chương phải về đầu bài.
  const bodyRef = useRef<HTMLElement>(null)
  useLayoutEffect(() => { bodyRef.current?.scrollTo({ top: 0, behavior: 'instant' }) }, [chapter])

  const commentList = comments ?? []
  const items = useMemo(() => bodyItems(diffed.blocks), [diffed.blocks])
  const openComment = commentList.find((comment) => comment.id === openId)
  const candidates = openComment?.candidates?.map((candidate) => candidate.paragraph) ?? []
  const picking = !!relink
  const sent = commentList.some((comment) => (comment.status ?? (comment.appliedAt ? 'sent' : 'pending')) === 'sent')

  return (
    <div style={{ flex: 1, minHeight: 0, position: 'relative', display: 'flex' }}>
    <article ref={bodyRef} className="scroll" onMouseUp={() => { if (bodyRef.current) onSelection?.(selectionInReader(bodyRef.current, window.getSelection())) }} onKeyUp={() => { if (bodyRef.current) onSelection?.(selectionInReader(bodyRef.current, window.getSelection())) }} aria-label={`${copy.chapters} ${chapter}`} style={{
      flex: 1, minHeight: 0, overflowY: narrow ? 'visible' : undefined,
      padding: '28px 24px 32px', display: 'flex', justifyContent: 'center',
    }}>
      <div className={`reader-body${sent ? ' fbsent' : ''}`} style={{
        width: '100%', maxWidth: '66ch', '--reader-size': `${size}px`,
        fontSize: size, lineHeight: 1.85, color: 'var(--kit-fg-prose)',
        display: 'flex', flexDirection: 'column', gap: '1em',
      } as CSSProperties}>
        {loading && !content && (
          <div aria-busy="true" style={{ display: 'flex', flexDirection: 'column', gap: 14, width: '100%' }}>
            {[92, 100, 86, 96, 74].map((width, index) => (
              <Skeleton key={index} width={`${width}%`} height={size} radius={6} />
            ))}
            <SlowLoadingNotice />
          </div>
        )}

        {!loading && error && !content && !draftMissing && (
          <EmptyState
            icon={missingChapter ? 'book-1' : 'danger'}
            title={missingChapter ? copy.chapterMissing(chapter) : copy.loadError}
            hint={missingChapter ? undefined : error.message}
            action={missingChapter
              ? <Button size="sm" onClick={onGoLatest}>{copy.goLatest}</Button>
              : <Button size="sm" onClick={onRetry}>{copy.retry}</Button>}
          />
        )}

        {emptyDraft && (
          <EmptyState icon="edit-2" title={copy.emptyDraft} hint={copy.emptyDraftHint}
            action={<Button size="sm" onClick={onViewFinal}>{copy.viewFinal}</Button>} />
        )}

        {emptyChapter && (
          <EmptyState icon="book-1" title={copy.emptyChapter} hint={copy.emptyChapterHint} />
        )}

        {content?.text && items.map((item, index) => {
          if (item.type === 'block') return <ChapterContent key={index} blocks={[item.block]} unmatchedRevisions={[]} showDiff={showDiff} />
          const slotText = item.blocks.map((block) => block.kind === 'paragraph' ? tokenText(block) : block.kind === 'diff' ? block.new : '')
          const isCand = candidates.includes(item.para)
          const classes = ['pw', slotText.some((text) => paragraphMarks(commentList, text, item.para, openId ?? null).whole) ? 'whole' : '',
            isCand ? 'cand' : '', relink?.peek === item.para ? 'peek' : '', picking ? 'pickable' : '', relink?.hov === item.para ? 'hov' : ''].filter(Boolean).join(' ')
          const slotMarks = item.blocks.map((block) => block.kind === 'paragraph' ? paragraphMarks(commentList, tokenText(block), item.para, openId ?? null) : { marks: [] as ParagraphMark[], whole: false })
          const anyMark = slotMarks.some((part) => part.marks.length > 0 || part.whole)
          return (
            <div key={index} className={classes}
              onClick={picking ? () => relink!.onPick(item.para) : undefined}
              onMouseEnter={picking ? () => relink!.onHover(item.para) : undefined}
              onMouseLeave={picking ? () => relink!.onHover(null) : undefined}
              aria-label={isCand ? `Đoạn gợi ý ${candidates.indexOf(item.para) + 1}/${candidates.length}` : undefined}>
              {anyMark && (
                <div className="gut" aria-hidden="true">
                  {slotMarks.flatMap((part) => part.marks).map((mark) => <span key={mark.comment.id} className={`gdot ${mark.tone}${mark.moved ? ' mv' : ''}`} />)}
                  {slotMarks.some((part) => part.marks.some((mark) => mark.moved)) && <span className="gmv">đã dời</span>}
                </div>
              )}
              {isCand && <span className="cbadge">{`Gợi ý ${candidates.indexOf(item.para) + 1}/${candidates.length}`}</span>}
              {item.blocks.map((block, blockIndex) => {
                if (block.kind !== 'paragraph') {
                  return <ChapterContent key={blockIndex} blocks={[block]} unmatchedRevisions={[]} showDiff={showDiff} />
                }
                return (
                  <p key={blockIndex} data-prose={item.para} style={{ margin: 0 }}>
                    <MarkedText text={tokenText(block)} marks={slotMarks[blockIndex]?.marks ?? []} onOpen={(id) => onOpenComment?.(id)} />
                  </p>
                )
              })}
              {relink?.hov === item.para && (
                <button type="button" className="attach" onClick={(event) => { event.stopPropagation(); relink.onPick(item.para) }}>
                  <Icon name="link-2" size={14} />{attach?.paragraph === item.para && attach.quote ? `Gắn vào “${attach.quote.slice(0, 30)}${attach.quote.length > 30 ? '…' : ''}”` : 'Gắn vào đây'}
                </button>
              )}
            </div>
          )
        })}
        {content?.text && showDiff && diffed.unmatched.length > 0 && (
          <p style={{ margin: 0, fontSize: 12, color: 'var(--color-fg-4)', fontStyle: 'italic' }}>Đoạn thay đổi không còn trong bản này.</p>
        )}
      </div>
    </article>
      {!narrow && <ScrollTopButton target={bodyRef} focusTarget={focusTarget} label={t.scrollTop.chapter} progress size="md" />}
    </div>
  )
}
