// Tên hiển thị audio ngoài app (10-audio.md › A7): một bộ quy tắc cho cả Media Session
// lẫn tên tệp xuất. Mock phải khớp từng hàm với backend — không suy diễn thêm.

/** Người đọc trên màn khoá/thẻ ID3 — luôn cố định, kể cả chương chưa chọn giọng. */
export const NOVERSE_ARTIST = 'Noverse Audio'

/** Không lấy nội dung “Chưa chọn giọng” làm người đọc — thấy A7 bug 28/10. */
const FORBIDDEN = /[/\\:*?"<>|\u0000-\u001f]/g
export const SHORT_TITLE_LIMIT = 32
const FALLBACK = 'Truyện'

/** Làm sạch một phần của tên tệp: bỏ ký tự cấm, gộp khoảng trắng. */
export function audioNamePart(text: string): string {
  return text.replace(FORBIDDEN, ' ').split(/\s+/).filter(Boolean).join(' ').replace(/^[. ]+|[. ]+$/g, '')
}

const QUOTES = [['“', '”'], ['‘', '’'], ['«', '»'], ['"', '"'], ["'", "'"]] as const

/** Strip import-document labels and paired outer quotes, preserving casing. */
function storyTitle(text: string): string {
  let title = text.split(/\s+/).filter(Boolean).join(' ')
  while (true) {
    title = title.replace(/^rules\s+kịch\s+bản\s*[:\-–—]\s*/iu, '')
    const pair = QUOTES.find(([open, close]) => title.startsWith(open) && title.endsWith(close) && title.length >= open.length + close.length)
    if (!pair) return title
    title = title.slice(pair[0].length, -pair[1].length).trim()
  }
}

/** Saved shortTitle wins. Never cut a word or retain half a quoted phrase. */
export function shortStoryTitle(title: string | null | undefined, shortTitle?: string | null): string {
  const clean = audioNamePart(storyTitle(shortTitle ?? '')) || audioNamePart(storyTitle(title ?? ''))
  if ([...clean].length <= SHORT_TITLE_LIMIT) return clean || FALLBACK
  let cut = ''
  for (const word of clean.split(' ')) {
    const next = cut ? `${cut} ${word}` : word
    if ([...next].length > SHORT_TITLE_LIMIT) break
    cut = next
  }
  for (const [open, close] of QUOTES) {
    const opens = cut.split(open).length - 1
    const closes = cut.split(close).length - 1
    if (open === close ? opens % 2 : opens > closes) cut = cut.slice(0, cut.lastIndexOf(open)).trim()
  }
  return cut || FALLBACK
}

/** Tên bài trên màn khoá / thẻ TIT2 của một chương: “Chương N · tên” (+ " (bản thử)"). */
export const chapterTrackTitle = (chapter: number, title: string, trial = false) =>
  `Chương ${chapter}${title.trim() ? ` · ${title.trim()}` : ''}${trial ? ' (bản thử)' : ''}`

/** Tên bài của tệp cả tập trước khi phát (máy nghe nhạc): “Tên ngắn · Tập N”. */
export const volumeTrackTitle = (short: string, volume: number) => `${short} · Tập ${volume}`

/** Album trên màn khoá / thẻ TALB: “Tên truyện · Tập N”. */
export const audioAlbumTitle = (storyTitle: string, volume: number) => `${storyTitle} · Tập ${volume}`

const pad2 = (n: number) => String(n).padStart(2, '0')

/** “Tên ngắn · Tập 1 · Chương 04 · Tên chương.mp3” — bản thử thêm “ (bản thử)” trước đuôi. */
export function audioChapterFileName(short: string, volume: number, chapter: number, chapterTitle: string, ext: string, trial = false): string {
  const name = audioNamePart(chapterTitle)
  return `${audioNamePart(short) || FALLBACK} · Tập ${volume} · Chương ${pad2(chapter)}${name ? ` · ${name}` : ''}${trial ? ' (bản thử)' : ''}.${ext}`
}

/** `filename=` dự phòng không dấu trong Content-Disposition (§AF): NFD bỏ dấu, đ→d, ký tự lạ → _. */
export function asciiFileName(name: string): string {
  const flat = name.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D')
    .replace(/[·—–]/g, '-')
  const safe = flat.replace(/[^ -~]/g, '_').replace(/[/\\:*?"<>|]/g, '_').replace(/_+/g, '_').replace(/\s+-\s+/g, ' - ')
  return safe || 'file'
}

/** “Tên ngắn · Tập 1.mp3” — tệp cả tập (partial export nối thêm `-chN` phía sau tập). */
export const audioVolumeFileName = (short: string, volume: number, ext: string, suffix = '') =>
  `${audioNamePart(short)} · Tập ${volume}${suffix}.${ext}`

/** “Tên ngắn · Tập 1 · 3 chương.zip” — nhiều chương gộp một tệp. */
export const audioZipFileName = (short: string, volume: number, count: number, suffix = '') =>
  `${audioNamePart(short)} · Tập ${volume} · ${count} chương${suffix}.zip`
