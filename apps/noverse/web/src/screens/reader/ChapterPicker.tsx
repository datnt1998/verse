// ChapterPicker — cột 236px: nhóm chương theo tập/hồi, dấu đã nộp / bản nháp.
// Mobile (<1100px): gập thành một hàng tóm tắt (chương đang đọc + số chữ) để phần
// thân chương không bị đẩy xuống dưới danh sách dài; mở ra khi cần đổi chương.
import { useId, useState } from 'react'
import type { ChapterInfo, Outline } from '../../api/types'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { copy, formatNumber } from './copy'
import { ChapterRowBadge } from './FeedbackPanel'
import { groupChapters, totalWords } from './logic'

export function ChapterPicker({ chapters, outline, current, onPick, error, onRetry, narrow }: {
  chapters: ChapterInfo[] | null
  outline: Outline | null
  current: number | null
  onPick: (chapter: number) => void
  error: string | null
  onRetry: () => void
  narrow: boolean
}) {
  const [open, setOpen] = useState(false)
  const listId = useId()
  const groups = chapters ? groupChapters(chapters, outline) : []
  const expanded = !narrow || open
  const selected = chapters?.find((chapter) => chapter.chapter === current) ?? null
  const pick = (chapter: number) => {
    onPick(chapter)
    if (narrow) setOpen(false)
  }
  return (
    <aside className="scroll" aria-label="Chọn chương" style={{
      flex: narrow ? '0 0 auto' : '0 0 236px',
      width: narrow ? '100%' : undefined, minWidth: 0,
      maxWidth: narrow ? 'none' : 236,
      minHeight: narrow ? undefined : 0,
      padding: '14px 10px', display: 'flex', flexDirection: 'column', gap: 10,
      overflowY: narrow ? 'visible' : undefined,
      borderRight: narrow ? 'none' : '1px solid var(--line-1)',
      borderBottom: narrow ? '1px solid var(--line-1)' : undefined,
    }}>
      {narrow ? (
        <button type="button" className="hov-ghost" aria-expanded={open} aria-controls={listId}
          onClick={() => setOpen((value) => !value)}
          style={{
            width: '100%', minHeight: 44, padding: '6px 8px', display: 'flex', alignItems: 'center', gap: 8,
            borderRadius: 9, border: '1px solid var(--line-2)', background: 'var(--color-surface-1)',
            color: 'var(--color-fg)', textAlign: 'left',
          }}>
          <Icon name="book-1" size={16} style={{ color: 'var(--color-fg-3)' }} />
          <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
            <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
              {copy.chapters} {current ?? '—'}{chapters ? `/${chapters.length}` : ''}
              {chapters ? ` · ${copy.words(formatNumber(totalWords(chapters)))}` : ''}
            </span>
            <span style={{ fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {selected?.title || copy.chapters}
            </span>
          </span>
          <Icon name="arrow-down-2" size={16}
            style={{ color: 'var(--color-fg-4)', transform: open ? 'rotate(180deg)' : undefined, transition: 'transform .15s var(--ease-out)' }} />
        </button>
      ) : (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 8px', gap: 8 }}>
          <h2 className="h3" style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' }}>
            <Icon name="book-1" size={16} />{copy.chapters}
          </h2>
          <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
            {chapters ? copy.words(formatNumber(totalWords(chapters))) : '—'}
          </span>
        </div>
      )}

      <div id={listId} style={{ display: expanded ? 'flex' : 'none', flexDirection: 'column', gap: 10, minWidth: 0 }}>
        {chapters === null && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }} aria-busy="true">
            {Array.from({ length: 7 }, (_, index) => <Skeleton key={index} height={30} radius={9} />)}
          </div>
        )}

        {chapters !== null && error && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '0 8px' }}>
            <span style={{ fontSize: 12, color: 'var(--color-err)' }}>{error}</span>
            <Button size="sm" onClick={onRetry}>{copy.retry}</Button>
          </div>
        )}

        {chapters !== null && !error && chapters.length === 0 && (
          <span style={{ padding: '0 8px', fontSize: 12, color: 'var(--color-fg-4)' }}>{copy.noChapters}</span>
        )}

        {groups.map((group) => (
          <div key={group.volume} role="group" aria-label={copy.volume(group.volume, group.title)}>
            {groups.length > 1 && (
              <div className="lbl" style={{ padding: '2px 8px 4px' }}>{copy.volume(group.volume, group.title)}</div>
            )}
            {group.arcs.map((arc) => (
              <div key={arc.arc}>
                {group.arcs.length > 1 && (
                  <div style={{ padding: '6px 8px 2px', fontSize: 11, color: 'var(--color-fg-4)' }}>
                    {copy.arc(arc.arc, arc.title)}
                  </div>
                )}
                <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {arc.chapters.map((chapter) => {
                    const isCurrent = chapter.chapter === current
                    return (
                      <li key={chapter.chapter}>
                        <button type="button" className="hov-row" aria-current={isCurrent ? 'true' : undefined}
                          onClick={() => pick(chapter.chapter)} style={{
                            width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '8px 10px',
                            borderRadius: 9, border: `1px solid ${isCurrent ? 'var(--line-4)' : 'transparent'}`,
                            background: isCurrent ? 'var(--color-surface-4)' : 'transparent',
                            color: 'var(--color-fg)', textAlign: 'left',
                          }}>
                          <span className="num" style={{ flex: '0 0 16px', fontSize: 12, color: 'var(--color-fg-4)' }}>{chapter.chapter}</span>
                          <span style={{
                            flex: 1, minWidth: 0, fontSize: 13, fontWeight: isCurrent ? 600 : 400,
                            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                          }}>{chapter.title || `Chương ${chapter.chapter}`}</span>
                          <ChapterRowBadge comments={chapter.comments} />
                          <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                            {chapter.words > 0 ? formatNumber(chapter.words) : ''}
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ol>
              </div>
            ))}
          </div>
        ))}
      </div>
    </aside>
  )
}
