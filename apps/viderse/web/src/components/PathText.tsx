import * as stylex from '@stylexjs/stylex'
import { font } from '@noverse/ui/tokens.stylex'
import { useEffect, useRef, useState, type CSSProperties } from 'react'

// 04-components › Đường dẫn thư mục (#78): in `displayPath` của API, một dòng,
// chữ UI, hết chỗ thì cắt GIỮA theo thư mục — giữ phần đầu (`~` hoặc
// `/Volumes/<tên ổ>`) và 2 thư mục cuối, vẫn dài thì 1 thư mục cuối, tên cuối
// quá dài mới cắt ký tự ở đầu tên. Không xuống dòng, không cắt cuối. Bề rộng
// đo bằng canvas theo font thật của dòng (không đếm ký tự cứng) và đo lại khi
// đổi bề rộng / font tải xong.

/** Đo bề rộng một chuỗi bằng đúng font của dòng (px). */
export type Measure = (text: string) => number

/** Phần đầu được giữ nguyên khi cắt: `~`, `/Volumes/<tên ổ>`, hay gốc đường dẫn. */
function headOf(parts: string[]): { text: string; count: number } {
  if (parts[0] === '~') return { text: '~', count: 1 }
  if (parts[0] === '' && parts[1] === 'Volumes' && parts[2]) {
    return { text: `/Volumes/${parts[2]}`, count: 3 }
  }
  return { text: parts.slice(0, 2).join('/'), count: 2 }
}

/**
 * fitPath cắt giữa đường dẫn theo thư mục cho vừa `maxWidth`.
 * `maxWidth` là bề rộng thật của dòng; 0 (chưa đo được) thì trả nguyên.
 */
export function fitPath(path: string, maxWidth: number, measure: Measure): string {
  if (!path || maxWidth <= 0) return path
  if (measure(path) <= maxWidth) return path
  const parts = path.split('/')
  const head = headOf(parts)
  const rest = parts.slice(head.count)
  // Giữ 2 rồi 1 thư mục cuối — không cắt cuối vì thư mục cuối là phần người
  // dùng cần thấy nhất.
  for (const take of [2, 1]) {
    if (rest.length <= take) continue
    const candidate = `${head.text}/…/${rest.slice(-take).join('/')}`
    if (measure(candidate) <= maxWidth) return candidate
  }
  // Tên thư mục cuối quá dài: cắt ký tự ở ĐẦU tên, giữ đuôi tên.
  const last = parts[parts.length - 1] ?? ''
  const prefix = `${head.text}/…/…`
  let keep = last.length
  while (keep > 1 && measure(prefix + last.slice(-keep)) > maxWidth) keep--
  return `${prefix}${last.slice(-keep)}`
}

/** Đo chữ bằng canvas theo font đã tính của phần tử; null khi môi trường không có canvas. */
function measureFor(el: HTMLElement): Measure | null {
  const ctx = document.createElement('canvas').getContext('2d')
  if (!ctx) return null
  const cs = getComputedStyle(el)
  if (!cs.fontSize) return null
  ctx.font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`
  return (text) => ctx.measureText(text).width
}

const sx = stylex.create({
  root: {
    display: 'block',
    minWidth: 0,
    maxWidth: '100%',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    fontFamily: font['--font-sans'],
  },
})

export type PathTextProps = {
  /** `displayPath` của API (thư mục nhà đã rút thành `~`). */
  path: string
  /** Tooltip: đường dẫn đầy đủ (mặc định = `path`). */
  title?: string
  className?: string
  xstyle?: stylex.StyleXStyles
  style?: CSSProperties
}

/**
 * Dòng đường dẫn thư mục dùng chung cho mọi chỗ hiện thư mục lưu dự án
 * (thẻ vị trí, hộp chọn thư mục, thẻ chuyển hỏng): một dòng, cắt giữa theo
 * thư mục theo bề rộng thật, `title` = đường dẫn đầy đủ.
 */
export function PathText({ path, title, className, xstyle, style }: PathTextProps) {
  const ref = useRef<HTMLSpanElement>(null)
  const [shown, setShown] = useState(path)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const measure = measureFor(el)
    if (!measure) {
      setShown(path)
      return
    }
    const apply = () => setShown(fitPath(path, el.clientWidth, measure))
    apply()
    const observer = new ResizeObserver(apply)
    observer.observe(el)
    // Font UI tải xong sau lần đo đầu thì bề rộng chữ đổi — đo lại.
    void document.fonts?.ready.then(apply)
    return () => observer.disconnect()
  }, [path])
  return (
    <span ref={ref} title={title ?? path} className={`${className ?? ''} ${stylex.props(sx.root, xstyle).className ?? ''}`.trim()} style={style}>
      {shown}
    </span>
  )
}
