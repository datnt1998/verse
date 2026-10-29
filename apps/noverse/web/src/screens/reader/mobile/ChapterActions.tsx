import { useId, useRef, useState } from 'react'
import type { ChapterInfo, ExportFormat } from '../../../api/types'
import { createExport, downloadExportBook } from '../../../api/client'
import { exportFormatOf, friendlyExportName, inferExportVolume } from '../../../lib/exportName'
import { paths } from '../../../router'
import { useStoryAccess } from '../useReaderData'
import { keys, useAction, useConnection } from '../../../data'
import { BottomSheet, useMobileToast } from '../../../mobile/primitives'
import { Icon } from '@noverse/ui'
import { copyToClipboard } from './clipboard'
import { chapterMenuItems, type ChapterMenuAction } from './chapterMenu'

/** Bước thứ hai “Xuất chương N dưới dạng” (MReader `state=exportMenu`, `acFmt`). */
const EXPORT_FORMATS: { id: ExportFormat; label: string; icon: string }[] = [
  { id: 'txt', label: 'Văn bản .txt', icon: 'document-text' },
  { id: 'epub', label: 'Ebook .epub', icon: 'book-1' },
  { id: 'narration', label: 'Bản đọc TTS · cho giọng đọc AI', icon: 'headphone' },
]
const FORMAT_WORD: Record<string, string> = { txt: 'văn bản', epub: 'ebook', narration: 'bản đọc TTS' }

/** Menu ⋯ của một chương: nhóm tiêu đề + mục có biểu tượng, và nhóm Huỷ riêng, không thanh kéo (MReader `chapterMenu`). */
export function ChapterActions({ project, story, chapter, writable, onClose, onRead, onListen, onRewrite }: {
  project: string; story: string; chapter: ChapterInfo | null; writable: boolean
  onClose: () => void; onRead: (chapter: number) => void; onListen: (chapter: ChapterInfo) => void; onRewrite: (chapter: number) => void
}) {
  const connection = useConnection()
  const toast = useMobileToast()
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const [pickingFormat, setPickingFormat] = useState(false)
  const { storyTitle } = useStoryAccess(project, story)
  const titleId = useId()
  const lastChapter = useRef<ChapterInfo | null>(null)
  if (chapter) lastChapter.current = chapter
  const shown = chapter ?? lastChapter.current
  const exportChapter = useAction(async (number: number, format: ExportFormat) => {
    const result = await createExport(project, story, {
      format, from: number, to: number,
      ...(format === 'narration' ? { readNumbers: true } : {}),
    })
    const name = friendlyExportName(storyTitle, inferExportVolume(result.path) ?? shown?.volume ?? 1, exportFormatOf(result.path) || format)
    await downloadExportBook(project, story, result.path, name)
    return result
  }, { invalidate: [keys.story(project, story, 'exports')] })

  const exportAs = async (chosen: ChapterInfo, format: ExportFormat) => {
    if (!connection.writable) throw new Error('Có mạng trở lại để xuất chương.')
    if (!(await exportChapter.run(chosen.chapter, format))) throw new Error('Không xuất được chương. Hãy thử lại.')
    toast(`Đã xuất chương ${chosen.chapter} · ${FORMAT_WORD[format]}`)
  }
  const perform = async (chosen: ChapterInfo, id: ChapterMenuAction) => {
    if (id === 'read') return onRead(chosen.chapter)
    if (id === 'listen') return onListen(chosen)
    if (id === 'rewrite') return onRewrite(chosen.chapter)
    if (id === 'export') { setPickingFormat(true); return }
    // “Chép liên kết chương” — liên kết tới chương, không phải đường dẫn tệp (D3).
    const link = typeof location === 'undefined'
      ? `#${paths.reader(project, story, chosen.chapter)}`
      : new URL(`#${paths.reader(project, story, chosen.chapter)}`, location.href).toString()
    await copyToClipboard(link)
    toast(`Đã chép liên kết chương ${chosen.chapter}`)
  }
  const choose = async (id: ChapterMenuAction) => {
    if (!chapter || pending) return
    setPending(true); setFailure('')
    try { await perform(chapter, id); if (id !== 'export') onClose() }
    catch (cause) { setFailure(cause instanceof Error ? cause.message : 'Không thực hiện được. Vui lòng thử lại.') }
    finally { setPending(false) }
  }
  const chooseFormat = async (format: ExportFormat) => {
    if (!chapter || pending) return
    setPending(true); setFailure('')
    try { await exportAs(chapter, format); onClose() }
    catch (cause) { setFailure(cause instanceof Error ? cause.message : 'Không xuất được chương. Vui lòng thử lại.') }
    finally { setPending(false) }
  }
  const close = () => { if (pending) return; setFailure(''); setPickingFormat(false); onClose() }
  const back = () => { if (pending) return; setFailure(''); setPickingFormat(false) }
  const title = pickingFormat
    ? (shown ? `Xuất chương ${shown.chapter} dưới dạng` : 'Xuất chương dưới dạng')
    : shown ? `Chương ${shown.chapter} · ${shown.title}` : 'Thao tác chương'
  return <BottomSheet open={chapter !== null} onClose={close} height="auto" grab={false} label={title} style={{ background: 'transparent', border: 0, boxShadow: 'none', padding: '0 10px 30px' }}>
    <div className="mr-menu">
      <div className="mr-menu-group" role="menu" aria-labelledby={titleId}>
        <div className="mr-menu-head"><span id={titleId}>{title}</span></div>
        {pickingFormat
          ? EXPORT_FORMATS.map((format) => (
            <button key={format.id} role="menuitem" type="button" className="mr-menu-item" disabled={pending} onClick={() => { void chooseFormat(format.id) }}>
              <Icon name={format.icon} size={18} />{format.label}
            </button>
          ))
          : chapterMenuItems({ writable, exporting: exportChapter.pending }).map((item) => (
            <button key={item.id} role="menuitem" type="button" className={`mr-menu-item${item.accent ? ' accent' : ''}`} disabled={pending} onClick={() => { void choose(item.id) }}>
              <Icon name={item.icon} size={18} />{item.label}
            </button>
          ))}
        {failure && <p className="m-error mr-menu-error" role="alert">{failure}</p>}
      </div>
      <div className="mr-menu-group">
        <button type="button" className="mr-menu-cancel" disabled={pending} onClick={pickingFormat ? back : close}>
          {pickingFormat ? 'Quay lại' : 'Huỷ'}
        </button>
      </div>
    </div>
  </BottomSheet>
}
