import { useEffect, useState, type CSSProperties } from 'react'
import type { Snapshot } from '../api/types'
import { approveWithComments, getReviews } from '../api/client'
import { keys, useAction, useConnection, useResource } from '../data'
import { BottomSheet, useMobileToast } from '../mobile/primitives'
import { paths } from '../router'
import { Icon } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { ReaderBody } from '../screens/reader/ReaderBody'
import { useChapterContent, useChapters, useStoryAccess } from '../screens/reader/useReaderData'
import { PARAGRAPH_COMMENT_CHIPS, useParagraphComments, type ParagraphSelection } from '../screens/reader/comments'
import { isGate } from '../screens/workspace/GateCard'
import { tabletChapter } from './logic'
import { PanelRefreshBar } from '../ui/PanelRefreshBar'

type Props = { project: string; story: string; snapshot: Snapshot | null; chapter: number | null; focused: boolean; onRestoreWrite: () => void }
export function TabletReader({ project, story, snapshot, chapter, focused, onRestoreWrite }: Props) {
  const chapters = useChapters(project, story)
  const [picked, setPicked] = useState<number | null>(null)
  const [committedChapter, setCommittedChapter] = useState<number | null>(null)
  const [sheet, setSheet] = useState<'chapters' | 'font' | 'comment' | null>(null)
  const [size, setSize] = useState(19)
  const [spacing, setSpacing] = useState(1.85)
  const [family, setFamily] = useState<'serif' | 'sans'>('serif')
  const [selection, setSelection] = useState<ParagraphSelection>({ paragraph: null, quote: '' })
  const [note, setNote] = useState('')
  const [chips, setChips] = useState<string[]>([])
  const toast = useMobileToast()
  const connection = useConnection()
  const access = useStoryAccess(project, story)
  useEffect(() => { setPicked(null); setSheet(null); setCommittedChapter(null) }, [project, story, chapter])
  const current = tabletChapter(chapters.chapters, picked ?? chapter)
  const pending = !!snapshot && (snapshot.waitingFor === 'gate' || isGate(snapshot))
  const gate = pending && current === tabletChapter(chapters.chapters, null)
  const info = chapters.chapters?.find((item) => item.chapter === current)
  const source = committedChapter !== current && info && (gate || !info.completed) ? 'draft' : 'committed'
  const content = useChapterContent(project, story, current, source)
  const reviews = useResource(keys.story(project, story, 'reviews', current, 'raw'), ({ signal }) => getReviews(project, story, current!, signal), { enabled: current !== null })
  const comments = useParagraphComments(project, story, current)
  const title = content.content?.title || info?.title || 'Đọc truyện'
  const words = content.content?.words ?? info?.words ?? 0
  const scores = (reviews.data?.chapter ?? reviews.data?.global)?.dimensions?.map((item) => item.score) ?? []
  const score = scores.length ? (scores.reduce((sum, value) => sum + value, 0) / scores.length / 10).toFixed(1).replace('.', ',') : null
  const writable = access.writable && connection.writable
  const approve = useAction(async () => { if (current === null) return; await approveWithComments(project, story, current); toast(`Đã duyệt chương ${current} · bắt đầu chương ${current + 1}`) }, { error: 'Không duyệt được chương', invalidate: [keys.story(project, story, 'snapshot'), keys.story(project, story, 'chapters'), keys.story(project, story, 'comments', current)] })
  const openComment = (value: ParagraphSelection = { paragraph: null, quote: '' }) => { setSelection(value); setNote(''); setChips([]); setSheet('comment') }
  const send = async () => {
    if (!writable || comments.add.busy || (!note.trim() && !chips.length)) return
    const result = await comments.add.run({ ...selection, text: note.trim(), chips })
    if (result) { setSheet(null); setSelection({ paragraph: null, quote: '' }); toast('Đã gửi góp ý cho Người viết') }
  }
  return <section className="tablet-reader" aria-label="Đọc">
    <header className="tablet-chapterbar">
      {focused && <button className="tablet-restore" aria-label="Hiện cột Viết" onClick={onRestoreWrite}><Icon name="sidebar-left" size={18} /></button>}
      <div className="tablet-chapter-title"><strong>{current ? `Chương ${current} · ` : ''}{title}</strong><small className={gate ? 'gate' : ''}>{gate ? 'Bản nháp chờ duyệt' : info?.completed ? 'Đã duyệt' : info?.hasDraft ? 'Đang viết' : 'Chọn chương để đọc'}{score && !gate ? ` · ${score} điểm` : ''}{words ? ` · ${words.toLocaleString('vi-VN')} chữ` : ''}</small></div>
      <button className="tablet-chip" aria-label="Chọn chương" onClick={() => setSheet('chapters')}><Icon name="note-text" size={15} /><span>Chương</span></button>
      <button className="tablet-chip tablet-aa" aria-label="Cỡ chữ" onClick={() => setSheet('font')}>Aa</button>
      <a className="tablet-chip" aria-label="Nghe chương này" href={`#${paths.audio(project, story, info?.volume, 'listen')}`}><Icon name="headphone" size={15} /></a>
    </header>
    <PanelRefreshBar resourceKeys={[keys.story(project, story)]} updatedAt={content.updatedAt} />
    <div className="tablet-reader-scroll" style={{ '--tablet-reader-spacing': spacing, '--tablet-reader-family': family === 'serif' ? 'Literata, Georgia, serif' : 'var(--font-sans)' } as CSSProperties}>
      {(chapters.error || (content.error && content.content)) && <div className="tablet-notice" role="alert">{(chapters.error || content.error)?.message}<button className="m-chip" onClick={() => { void chapters.reload(); void content.reload() }}>Thử lại</button></div>}
      {reviews.error != null && <div className="tablet-notice" role="alert">{reviews.error instanceof Error ? reviews.error.message : 'Không tải được điểm duyệt'}<button className="m-chip" onClick={() => { void reviews.refresh() }}>Thử lại</button></div>}
      {connection.state !== 'open' && <p className="tablet-notice" role="status">{connection.online ? 'Đang kết nối lại · nội dung có thể chưa mới nhất' : 'Ngoại tuyến · đang hiển thị bản đã lưu'}</p>}
      {current !== null ? <div className="tablet-prose"><h2>{title}</h2><ReaderBody chapter={current} content={content.content} loading={content.loading} error={content.error} source={source} size={size} showDiff={false} revisions={null} onRetry={() => { void content.reload() }} onViewFinal={() => setCommittedChapter(current)} onGoLatest={() => setPicked(null)} narrow onSelection={(value) => { if (access.writable && value.quote) openComment(value) }} /></div> : <div className="tablet-notice" role="status">{chapters.chapters === null ? 'Đang tải danh sách chương…' : 'Chưa có chương nào. Chương mới sẽ xuất hiện khi Người viết hoàn thành.'}</div>}
    </div>
    {gate && <div className="tablet-gate" role="region" aria-label={`Duyệt chương ${current}`}><div><strong>Chương {current} chờ bạn duyệt</strong><small>{score ? `Biên tập viên chấm ${score} · ` : ''}{words.toLocaleString('vi-VN')} chữ</small></div><div className="tablet-gate-actions"><button className="tablet-btn gho" disabled={!writable} onClick={() => openComment()}>Nhờ viết lại…</button><button className="tablet-btn pri" disabled={!writable || approve.busy || comments.add.busy} aria-busy={approve.pending} onClick={() => { void approve.run() }}><Icon name="tick-circle" size={17} />{approve.pending ? 'Đang duyệt…' : 'Duyệt & viết tiếp'}</button></div>{!writable && <small>{access.locked ? 'Truyện đang khoá' : !connection.writable ? 'Cần kết nối để duyệt chương' : 'Bạn chỉ có quyền đọc'}</small>}{approve.error != null && <p className="m-error" role="alert">{approve.error instanceof Error ? approve.error.message : 'Không duyệt được chương'}<button onClick={() => { void approve.run() }}>Thử lại</button></p>}</div>}
    <BottomSheet open={sheet === 'chapters'} onClose={() => setSheet(null)} title="Danh sách chương" height={620}><div className="m-group">{chapters.chapters?.map((item) => <button className="m-row" key={item.chapter} aria-current={current === item.chapter ? 'page' : undefined} disabled={!item.completed && !item.hasDraft && !item.words} onClick={() => { setPicked(item.chapter); setSheet(null) }}><span>{item.chapter}</span><span className="m-row-copy">{item.title}<small>{item.completed ? 'Đã duyệt' : item.hasDraft ? 'Bản nháp' : 'Chưa viết'}</small></span><span className="m-muted">{item.words.toLocaleString('vi-VN')} chữ</span></button>)}</div></BottomSheet>
    <BottomSheet open={sheet === 'font'} onClose={() => setSheet(null)} title="Hiển thị" height="auto"><div className="tablet-font"><label>Cỡ chữ · {size}<input aria-label="Cỡ chữ" type="range" min={15} max={23} value={size} onChange={(event) => setSize(Number(event.target.value))} /></label><div className="m-inline-actions"><button className={`m-chip${family === 'serif' ? ' on' : ''}`} onClick={() => setFamily('serif')}>Có chân</button><button className={`m-chip${family === 'sans' ? ' on' : ''}`} onClick={() => setFamily('sans')}>Không chân</button></div><label>Giãn dòng<KitSelect aria-label="Giãn dòng" value={String(spacing)} height={46} onChange={(value) => setSpacing(Number(value))} options={[{ value: '1.6', label: 'Vừa · 1,6' }, { value: '1.85', label: 'Thoáng · 1,85' }, { value: '2.1', label: 'Rộng · 2,1' }]} /></label></div></BottomSheet>
    <BottomSheet open={sheet === 'comment' && access.writable} onClose={() => setSheet(null)} title={selection.paragraph === null ? `Nhờ viết lại chương ${current}` : 'Góp ý đoạn này'} height="auto"><div className="tablet-font">{selection.quote && <blockquote>{selection.quote}</blockquote>}<div className="m-inline-actions">{PARAGRAPH_COMMENT_CHIPS.map((chip) => <button key={chip} className={`m-chip${chips.includes(chip) ? ' on' : ''}`} onClick={() => setChips((old) => old.includes(chip) ? old.filter((item) => item !== chip) : [...old, chip])}>{chip}</button>)}</div><label>Muốn Người viết sửa thế nào?<textarea className="m-field" rows={4} value={note} onChange={(event) => setNote(event.target.value)} /></label>{comments.add.error != null && <p className="m-error" role="alert">{comments.add.error instanceof Error ? comments.add.error.message : 'Không gửi được góp ý'}</p>}<button className="m-button" disabled={!writable || comments.add.busy || (!note.trim() && !chips.length)} aria-busy={comments.add.pending} onClick={() => { void send() }}>{comments.add.pending ? 'Đang gửi…' : 'Gửi cho Người viết'}</button></div></BottomSheet>
  </section>
}
