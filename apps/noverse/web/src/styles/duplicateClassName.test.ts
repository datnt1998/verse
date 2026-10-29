// Chặn `className` khai hai lần trên cùng một thẻ JSX: một literal cạnh `{...stylex.props()}`.
// React chỉ giữ thuộc tính sau và cảnh báo "Duplicate className attribute" trong console.
// Cách đúng: gộp class bằng `` className={`legacy ${stylex.props(...).className}`} ``.
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const SRC = fileURLToPath(new URL('..', import.meta.url))
const EXEMPT = /\.test\.tsx?$/

/** Mọi thẻ mở JSX trong một tệp: từ `<Tên` tới `>` đầu tiên ngoài ngoặc nhọn và chuỗi. */
export function jsxTags(source: string): { tag: string; line: number }[] {
  const tags: { tag: string; line: number }[] = []
  let line = 1
  for (let i = 0; i < source.length; i++) {
    const ch = source[i]
    if (ch === '\n') line++
    if (ch !== '<' || !/[A-Za-z]/.test(source[i + 1] ?? '')) continue
    let depth = 0
    let quote: string | null = null
    const start = line
    let j = i + 1
    for (; j < source.length; j++) {
      const c = source[j]
      if (c === '\n') line++
      if (quote) {
        if (c === quote) quote = null
        else if (c === '\\') j++
        continue
      }
      if (c === '"' || c === "'" || c === '`') { quote = c; continue }
      if (c === '{') depth++
      else if (c === '}') depth--
      else if (c === '>' && depth === 0) break
    }
    tags.push({ tag: source.slice(i + 1, j), line: start })
    i = j
  }
  return tags
}

function offenders() {
  const found: string[] = []
  for (const entry of readdirSync(SRC, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.tsx')) continue
    const path = join(entry.parentPath, entry.name)
    const file = relative(SRC, path).split('\\').join('/')
    if (EXEMPT.test(file)) continue
    for (const { tag, line } of jsxTags(readFileSync(path, 'utf8'))) {
      if (!/\{\s*\.\.\.\s*(?:stylex\.props|stylex\.create|\.\.\.)/.test(tag)) continue
      if (!/\.\.\.\s*stylex\.props\(/.test(tag)) continue
      // `className="x"` hoặc `className={'x'}` = literal cạnh spread → đè lên nhau.
      const literal = tag.match(/\bclassName=(?:"[^"]*"|\{[^}]*['"][^}]*\})/)
      if (literal) found.push(`${file}:${line} ${literal[0]}`)
    }
  }
  return found
}

describe('JSX: không khai className hai lần', () => {
  it('không có className literal cạnh {...stylex.props()}', () => {
    expect(offenders(), 'Gộp bằng `className={`x ${stylex.props(...).className}`}`').toEqual([])
  })
})
