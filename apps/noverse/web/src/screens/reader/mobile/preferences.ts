import { getReaderFont, setReaderFont, setReaderSize } from '../../../api/storage'

export type ReaderPosition = { chapter: number; paragraph: number; offset: number; progress: number }
export type ReaderDisplay = { size: number; family: 'serif' | 'sans'; spacing: number }
const read = <T>(key: string, fallback: T): T => {
  try { const raw = localStorage.getItem(key); return raw ? JSON.parse(raw) as T : fallback } catch { return fallback }
}
const write = (key: string, value: unknown) => { try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* Đọc vẫn hoạt động khi bộ nhớ đầy. */ } }
const positionKey = (project: string, story: string) => `ainovel.mobileReader.position.${encodeURIComponent(project)}.${encodeURIComponent(story)}`
export const getPosition = (project: string, story: string): ReaderPosition | null => {
  const value = read<ReaderPosition | null>(positionKey(project, story), null)
  return value && Number.isInteger(value.chapter) && value.chapter > 0 && Number.isInteger(value.paragraph) && value.paragraph >= 0 && Number.isFinite(value.offset) && Number.isFinite(value.progress) ? value : null
}
export const savePosition = (project: string, story: string, value: ReaderPosition) => write(positionKey(project, story), value)
export const getDisplay = (): ReaderDisplay => {
  const spacing = read<number>('ainovel.mobileReader.spacing', 1.8)
  return { size: Math.min(23, Math.max(15, read<number>('ainovel.readerSize', 18))), family: getReaderFont() === 'sans' ? 'sans' : 'serif', spacing: [1.55, 1.8, 2.05].includes(spacing) ? spacing : 1.8 }
}
export const saveDisplay = (display: ReaderDisplay) => {
  setReaderSize(display.size)
  setReaderFont(display.family === 'serif' ? 'literata' : 'sans')
  write('ainovel.mobileReader.spacing', display.spacing)
}
