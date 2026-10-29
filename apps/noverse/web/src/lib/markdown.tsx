// Markdown nhẹ dùng chung: tiêu đề, đoạn, trích, danh sách, dấu ngắt cảnh + nhấn mạnh nội dòng.
// Không hiển thị cú pháp thô (##, **, `) ra người đọc. Trình đọc dùng cho văn bản chương
// (kèm diff thay đổi gần nhất); Inspector dùng cho tiền đề/tóm tắt trong snapshot.
import { Fragment, memo } from 'react'
import type { CSSProperties, ReactNode } from 'react'
import type { RevisionEntry } from '../api/types'

export type InlineToken = { text: string; bold?: boolean; italic?: boolean; code?: boolean }

export type ChapterBlock =
  | { kind: 'heading'; level: 1 | 2 | 3; text: string }
  | { kind: 'paragraph'; text: string }
  | { kind: 'quote'; text: string }
  | { kind: 'list'; items: string[] }
  | { kind: 'break' }

export type DiffBlock = { kind: 'diff'; old: string; new: string; meta: string }

const DIFF_NOT_IN_TEXT = 'Đoạn thay đổi không còn trong bản này.'

/** Nhãn tiếng Việt cho `RevisionEntry.kind` (engine ghi mã công cụ). */
export function revisionKindLabel(kind: string): string {
  switch (kind) {
    case 'edit_chapter':
    case 'edit':
      return 'Gọt giũa'
    case 'rewrite_chapter':
    case 'rewrite':
      return 'Viết lại'
    case 'sync':
      return 'Nhận bản sửa tay'
    default:
      return kind || '—'
  }
}

const HEADING = /^(#{1,6})\s+(.*)$/
const BREAK = /^\s*(?:---+|\*\s*\*\s*\*|___+)\s*$/
const QUOTE = /^\s*>\s?(.*)$/
const LIST = /^\s*[-*+]\s+(.*)$/

/** Tách văn bản chương thành khối; dòng trống ngăn khối. */
export function parseChapterText(raw: string): ChapterBlock[] {
  const text = raw.replace(/\r\n?/g, '\n').trim()
  if (!text) return []
  const blocks: ChapterBlock[] = []
  let paragraph: string[] = []
  let quote: string[] = []
  let list: string[] = []

  const flush = () => {
    if (paragraph.length) { blocks.push({ kind: 'paragraph', text: paragraph.join('\n') }); paragraph = [] }
    if (quote.length) { blocks.push({ kind: 'quote', text: quote.join('\n') }); quote = [] }
    if (list.length) { blocks.push({ kind: 'list', items: list }); list = [] }
  }

  for (const line of text.split('\n')) {
    if (!line.trim()) { flush(); continue }
    if (BREAK.test(line)) { flush(); blocks.push({ kind: 'break' }); continue }
    const heading = HEADING.exec(line)
    if (heading) {
      flush()
      blocks.push({ kind: 'heading', level: Math.min(3, heading[1].length) as 1 | 2 | 3, text: heading[2].trim() })
      continue
    }
    const quoted = QUOTE.exec(line)
    if (quoted) {
      if (paragraph.length || list.length) flush()
      quote.push(quoted[1])
      continue
    }
    const item = LIST.exec(line)
    if (item) {
      if (paragraph.length || quote.length) flush()
      list.push(item[1])
      continue
    }
    if (quote.length || list.length) flush()
    paragraph.push(line)
  }
  flush()
  return blocks
}

const INLINE = /(\*\*[^*]+\*\*|__[^_]+__|\*[^*\n]+\*|_[^_\n]+_|`[^`]+`)/g

/** Tách nhấn mạnh nội dòng: **đậm**, *nghiêng*, `mã`. */
export function parseInline(text: string): InlineToken[] {
  const tokens: InlineToken[] = []
  let last = 0
  for (const match of text.matchAll(INLINE)) {
    const index = match.index ?? 0
    if (index > last) tokens.push({ text: text.slice(last, index) })
    const token = match[0]
    if (token.startsWith('**') || token.startsWith('__')) tokens.push({ text: token.slice(2, -2), bold: true })
    else if (token.startsWith('`')) tokens.push({ text: token.slice(1, -1), code: true })
    else tokens.push({ text: token.slice(1, -1), italic: true })
    last = index + token.length
  }
  if (last < text.length) tokens.push({ text: text.slice(last) })
  return tokens
}

const Inline = memo(function Inline({ text, plain }: { text: string; plain?: (value: string) => ReactNode }) {
  return (
    <>
      {parseInline(text).map((token, index) => {
        if (token.code) return <code key={index} className="mono" style={{ fontSize: '0.9em', fontFamily: /[^\x00-\x7f]/.test(token.text) ? 'inherit' : undefined }}>{token.text}</code>
        if (token.bold) return <strong key={index} style={{ fontWeight: 600, color: 'var(--color-fg)' }}>{plain ? plain(token.text) : token.text}</strong>
        if (token.italic) return <em key={index}>{plain ? plain(token.text) : token.text}</em>
        return <span key={index}>{plain ? plain(token.text) : token.text}</span>
      })}
    </>
  )
})

/** Chèn diff của `revisions` vào đúng đoạn chứa `old`; trả về các revision không khớp. */
export function applyRevisions(blocks: ChapterBlock[], revisions: RevisionEntry[]): { blocks: (ChapterBlock | DiffBlock)[]; unmatched: RevisionEntry[] } {
  let result: (ChapterBlock | DiffBlock)[] = [...blocks]
  const unmatched: RevisionEntry[] = []
  for (const revision of revisions) {
    const needle = revision.old?.trim()
    if (!needle) { unmatched.push(revision); continue }
    let placed = false
    for (let index = 0; index < result.length; index += 1) {
      const block = result[index]
      if (block.kind !== 'paragraph' && block.kind !== 'quote') continue
      const at = block.text.indexOf(needle)
      if (at < 0) continue
      const before = block.text.slice(0, at).trim()
      const after = block.text.slice(at + needle.length).trim()
      const replacement: (ChapterBlock | DiffBlock)[] = []
      if (before) replacement.push({ kind: block.kind, text: before })
      replacement.push({
        kind: 'diff', old: needle, new: revision.new,
        meta: revisionKindLabel(revision.kind),
      })
      if (after) replacement.push({ kind: block.kind, text: after })
      result = [...result.slice(0, index), ...replacement, ...result.slice(index + 1)]
      placed = true
      break
    }
    if (!placed) unmatched.push(revision)
  }
  return { blocks: result, unmatched }
}

const headingStyle = (level: 1 | 2 | 3): CSSProperties => ({
  margin: level === 1 ? '0 0 0.7em' : '0.9em 0 0.5em',
  fontFamily: 'var(--font-sans)',
  fontSize: level === 1 ? '1.3em' : level === 2 ? '1.15em' : '1.02em',
  fontWeight: 600, letterSpacing: '-0.015em', lineHeight: 1.4, color: 'var(--color-fg)',
})

/** Render khối markdown; Trình đọc bọc trong khung Literata, Inspector dùng cho tiền đề.
 *  `plain` cho phép thay cách vẽ chữ thường trong một token — Trình đọc dùng để gạch chấm tên nhân vật. */
export function ChapterContent({ blocks, unmatchedRevisions, showDiff, plain }: {
  blocks: (ChapterBlock | DiffBlock)[]
  unmatchedRevisions: RevisionEntry[]
  showDiff: boolean
  plain?: (value: string) => ReactNode
}) {
  return (
    <>
      {blocks.map((block, index) => {
        if (block.kind === 'heading') {
          return block.level === 1
            ? <h1 key={index} style={headingStyle(1)}><Inline text={block.text} plain={plain} /></h1>
            : block.level === 2
              ? <h2 key={index} style={headingStyle(2)}><Inline text={block.text} plain={plain} /></h2>
              : <h3 key={index} style={headingStyle(3)}><Inline text={block.text} plain={plain} /></h3>
        }
        if (block.kind === 'break') {
          return <p key={index} aria-hidden="true" style={{ margin: '0.4em 0', textAlign: 'center', color: 'var(--color-fg-4)', letterSpacing: '0.6em' }}>· · ·</p>
        }
        if (block.kind === 'list') {
          return (
            <ul key={index} style={{ margin: 0, paddingLeft: '1.3em', listStyleType: 'disc', display: 'flex', flexDirection: 'column', gap: '0.25em' }}>
              {block.items.map((item, itemIndex) => <li key={itemIndex}><Inline text={item} plain={plain} /></li>)}
            </ul>
          )
        }
        if (block.kind === 'quote') {
          return (
            <blockquote key={index} style={{ margin: 0, paddingLeft: 12, borderLeft: '2px solid var(--line-4)', color: 'var(--color-fg-2)', fontStyle: 'italic' }}>
              <Inline text={block.text} plain={plain} />
            </blockquote>
          )
        }
        if (block.kind === 'diff') {
          // Reader.dc.html: ba dòng nằm rời trong cột chương (cách nhau 1em), dòng cũ nền err 7%, dòng mới nền ok 8%.
          return (
            <Fragment key={index}>
              <p style={{
                margin: 0, padding: '4px 10px', borderRadius: 8, background: 'var(--kit-errf-7)',
                color: 'var(--color-fg-3)', textDecoration: 'line-through', textDecorationColor: 'var(--kit-errf-60)',
              }}><Inline text={block.old} plain={plain} /></p>
              <p style={{ margin: 0, padding: '4px 10px', borderRadius: 8, background: 'var(--kit-ok-8)' }}>
                <Inline text={block.new} plain={plain} />
              </p>
              <p className="mono" style={{ margin: 0, fontSize: 12, color: 'var(--color-fg-4)' }}>{block.meta}</p>
            </Fragment>
          )
        }
        // `data-prose` đánh dấu đoạn văn thật — bộ đếm đoạn của góp ý (comments.ts)
        // chỉ đọc các p này, bỏ qua p của dấu ngắt cảnh và diff ở trên.
        return <p key={index} data-prose="" style={{ margin: 0 }}><Inline text={block.text} plain={plain} /></p>
      })}
      {showDiff && unmatchedRevisions.length > 0 && (
        <p style={{ margin: 0, fontSize: 12, color: 'var(--color-fg-4)', fontStyle: 'italic' }}>{DIFF_NOT_IN_TEXT}</p>
      )}
    </>
  )
}
