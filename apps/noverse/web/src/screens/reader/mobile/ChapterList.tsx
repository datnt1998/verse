import { useEffect, useRef } from 'react'
import type { ChapterInfo } from '../../../api/types'
import { Icon } from '@noverse/ui'
import { chapterReadable, chapterReadingState, type ReaderSnapshot } from './navigation'

export function ChapterList({ chapters, current, snapshot, onPick, onMore }: {
  chapters: ChapterInfo[] | null; current: number | null; snapshot: ReaderSnapshot | undefined
  onPick: (chapter: number) => void; onMore: (chapter: ChapterInfo) => void
}) {
  const timer = useRef<NodeJS.Timeout | undefined>(undefined)
  const start = useRef<{ x: number; y: number } | null>(null)
  const held = useRef(false)
  const cancel = () => { clearTimeout(timer.current); start.current = null }
  useEffect(() => () => clearTimeout(timer.current), [])
  return <div className="m-group mr-chapters" ref={(node) => { if (node) requestAnimationFrame(() => node.querySelector<HTMLElement>('[aria-current=page]')?.scrollIntoView({ block: 'center' })) }}>{chapters?.map((item) => {
    const state = chapterReadingState(item, snapshot)
    const readable = chapterReadable(item)
    return <div key={item.chapter} className={`mr-chapter-row${readable ? '' : ' unavailable'}`}><button className="m-row" aria-current={item.chapter === current ? 'page' : undefined} disabled={!readable} onClick={() => { if (held.current) { held.current = false; return }; onPick(item.chapter) }} onContextMenu={(event) => { if (item.completed) { event.preventDefault(); onMore(item) } }} onPointerDown={(event) => {
      if (!item.completed) return
      cancel(); held.current = false; start.current = { x: event.clientX, y: event.clientY }
      timer.current = setTimeout(() => { held.current = true; onMore(item) }, 550)
    }} onPointerMove={(event) => { if (start.current && Math.hypot(event.clientX - start.current.x, event.clientY - start.current.y) > 10) cancel() }} onPointerUp={cancel} onPointerCancel={cancel}>
      <span className="mr-chapter-number">{item.chapter}</span><span className="mr-chapter-name">{!readable && state !== 'writing' ? 'Chưa viết' : item.title}</span>{state === 'writing' && <span className="m-orb live" aria-hidden="true" />}<span className={`mr-chapter-state${state === 'waiting' ? ' waiting' : state === 'writing' ? ' writing' : ''}`}>{state === 'waiting' ? 'Chờ duyệt' : state === 'writing' ? 'Đang viết' : state === 'approved' ? 'Đã duyệt' : ''}</span>
    </button>{item.completed && <button className="m-icon-button mr-chapter-more" aria-label={`Thao tác cho chương ${item.chapter}`} onClick={() => onMore(item)}><Icon name="more" size={18} /></button>}</div>
  })}</div>
}
