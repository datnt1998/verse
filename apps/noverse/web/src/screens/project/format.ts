// Định dạng & suy diễn dùng chung cho Dự án · Truyện mới · Hộp thoại.
// Giá trị chuỗi theo 07-copy.md; số theo vi-VN (tabular-nums ở component).
import type { AutoExport, OutlineChapter, Snapshot, StoryStatus, StorySummary } from '../../api/types'
import { storyLabel } from '../../shell/labels'
import { statusText } from '../workspace/RunHeader'

export const formatWords = (words: number) => words.toLocaleString('vi-VN')

/** “77,8 KB” — dấu phẩy thập phân như artboard. */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1).replace('.', ',')} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} MB`
}

const pad = (n: number) => String(n).padStart(2, '0')

export const STORY_STATUS_LABEL: Record<StoryStatus, string> = {
  idea: 'Ý tưởng', writing: 'Đang viết', paused: 'Tạm dừng', done: 'Hoàn thành', published: 'Đã xuất bản',
}

export const storyTitle = (story: StorySummary) => storyLabel(story)

/** Số thứ tự tiền tố thư mục truyện: "01-tra-xanh" → "01". */
export const folderNumber = (storyId: string) => /^(\d+)-/.exec(storyId)?.[1] ?? null

/**
 * Nhãn một dòng cho hàng tập trong cây dự án: tiêu đề tập, hoặc “Tập N” khi thiếu tiêu đề
 * hoặc khi engine đặt tiêu đề tập trùng tên truyện (dữ liệu thật: tập 1 mang nguyên tên sách).
 */
export function volumeLabel(volume: { index: number; title?: string }, bookTitle: string | null): string {
  const title = (volume.title ?? '').trim()
  const book = (bookTitle ?? '').trim()
  if (!title || (book !== '' && title.toLowerCase() === book.toLowerCase())) return `Tập ${volume.index}`
  return title
}

/** Engine đang viết lại (chương chờ/đang viết lại) → artboard gọi chương theo “viết lại”. */
export const isRewriteFlow = (chapters: Pick<OutlineChapter, 'status'>[]): boolean =>
  chapters.some((chapter) => chapter.status === 'rewrite' || chapter.status === 'rewriting')

/** Nhãn trạng thái chương: Outline.dc.html dùng Đã / Đang / Chờ viết lại khi truyện đang được viết lại. */
export function chapterStatusLabel(status: OutlineChapter['status'], rewriteFlow: boolean): string {
  if (status === 'rewriting') return 'Đang viết lại'
  if (status === 'rewrite') return 'Chờ viết lại'
  if (status === 'writing') return 'Đang viết'
  if (status === 'done') return rewriteFlow ? 'Đã viết lại' : 'Hoàn thành'
  return rewriteFlow ? 'Chờ viết lại' : 'Chưa viết'
}

/**
 * Dòng trạng thái một dòng của truyện (thẻ truyện, popover, danh sách gọn — Outline.dc.html › nav=story):
 * “Đang viết lại · chương 4” khi engine chạy, “Ý tưởng · chưa bắt đầu viết”, còn lại nhãn trạng thái.
 * `currentChapter` = snapshot.inProgressChapter (0 → không kèm số).
 */
export function storyStatusText(story: StorySummary, rewriting: boolean, currentChapter: number): string {
  if (story.status === 'idea' || story.totalChapters === 0) return 'Ý tưởng · chưa bắt đầu viết'
  if (story.running) return `${rewriting ? 'Đang viết lại' : 'Đang viết'}${currentChapter > 0 ? ` · chương ${currentChapter}` : ''}`
  return STORY_STATUS_LABEL[story.status]
}

/** “Tự xuất văn bản và ebook mỗi khi xong một tập hoặc cả truyện.” — câu tóm tắt xuất tự động bằng lời. */
export function autoExportSentence(auto: AutoExport): string {
  const formats = auto.formats.map((format) => (format === 'epub' ? 'ebook' : format === 'narration' ? 'bản đọc TTS' : 'văn bản'))
  const when = [
    auto.onChapter ? 'sau mỗi chương' : '',
    auto.onVolume && auto.onBook ? 'mỗi khi xong một tập hoặc cả truyện' : auto.onVolume ? 'mỗi khi xong một tập' : auto.onBook ? 'mỗi khi xong cả truyện' : '',
  ].filter(Boolean)
  if (formats.length === 0 || when.length === 0) return 'Xuất tự động đang tắt.'
  // D3: bản đọc TTS không nằm trong tự xuất mặc định — nhắc rằng nó chỉ xuất khi người dùng cần.
  const tts = auto.formats.includes('narration') ? '' : ' Bản đọc TTS xuất khi bạn cần.'
  const list = formats.length > 2 ? `${formats.slice(0, -1).join(', ')} và ${formats[formats.length - 1]}` : formats.join(' và ')
  return `Tự xuất ${list} ${when.join(' và ')}.${tts}`
}

/** Số truyện kế tiếp (NN 2 chữ số) — 11-api-contract.md §3 › POST stories. */
export function nextStoryNumber(stories: StorySummary[]): string {
  const max = stories.reduce((acc, story) => {
    const n = Number.parseInt(folderNumber(story.story) ?? '', 10)
    return Number.isFinite(n) ? Math.max(acc, n) : acc
  }, 0)
  return pad(max + 1)
}

type HeaderSnapshot = Pick<Snapshot, 'statusLabel' | 'runtimeState' | 'inProgressChapter' | 'currentChapter' | 'completedCount' | 'totalChapters'>

/** Dòng phụ ở đầu màn Truyện trên điện thoại: “Đang chạy · chương 4/8” — mã engine luôn dịch, không in thô. */
export function storyHeaderLine(snapshot: HeaderSnapshot | undefined): string {
  const status = snapshot ? statusText(snapshot.statusLabel, snapshot.runtimeState) : ''
  const chapter = snapshot?.inProgressChapter || snapshot?.currentChapter || snapshot?.completedCount || 0
  const line = `chương ${chapter}/${snapshot?.totalChapters ?? '…'}`
  return status ? `${status} · ${line}` : line
}
