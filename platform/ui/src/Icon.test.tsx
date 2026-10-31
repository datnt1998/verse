import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { consumers, PKG_SRC, sourceFiles } from '../test/consumers'
import { Icon } from './Icon'

describe('đường dẫn CSS mask của biểu tượng', () => {
  for (const name of ['trash', 'more', 'profile-delete', 'search-normal-1', 'arrow-down', 'refresh-2']) {
    it(`đặt URL ${name} trong dấu nháy để hỗ trợ SVG data URI`, () => {
      const html = renderToStaticMarkup(<Icon name={name} />)
      expect(html).toContain('-webkit-mask-image:url(&quot;')
      expect(html).toContain(';mask-image:url(&quot;')
      expect(html).not.toContain('1px dashed')
    })
  }
})

describe('tên biểu tượng không có file', () => {
  it('Icon dev vẽ ô đỏ gạch đứt kèm tên thiếu thay vì vòng tròn im lặng', () => {
    const html = renderToStaticMarkup(<Icon name="khong-co-bieu-tuong-nay" />)
    expect(html).toContain('Missing icon: khong-co-bieu-tuong-nay')
    expect(html).toContain('border:1px dashed var(--kit-dev-miss)')
  })
})

// Mã của kit và của mọi app dùng kit, khoá theo `kit/<đường dẫn>` hoặc `<app>/<đường dẫn trong src>`.
const roots = [{ label: 'kit', src: PKG_SRC }, ...consumers().map(({ app, src }) => ({ label: app, src }))]
const sources: Record<string, string> = {}
for (const { label, src } of roots) {
  for (const file of sourceFiles(src)) sources[`${label}/${file.slice(src.length + 1)}`] = readFileSync(file, 'utf8')
}
const files = new Set(readdirSync(join(PKG_SRC, 'icons')).filter((f) => f.endsWith('.svg')).map((f) => f.slice(0, -4)))
const kebab = /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/

/** Mọi tên biểu tượng viết thẳng trong mã: `<Icon name="x">`, `name={a ? 'x' : 'y'}`, `icon: 'x'`, `icon="x"`. */
function literalIconNames(source: string): string[] {
  const found: string[] = []
  const take = (value: string | undefined) => { if (value && kebab.test(value)) found.push(value) }
  for (const match of source.matchAll(/<Icon\b[^<]*?\bname=(?:"([^"]+)"|'([^']+)'|\{((?:[^{}]|\{[^{}]*\})*)\})/g)) {
    if (match[1] || match[2]) { take(match[1] ?? match[2]); continue }
    for (const literal of match[3].matchAll(/(['"])([^'"]+)\1/g)) {
      if (!/(===|!==|==|!=)\s*$/.test(match[3].slice(0, literal.index))) take(literal[2])
    }
  }
  for (const match of source.matchAll(/\b(?:icon|iconName)\s*(?::|=)\s*(['"])([^'"]+)\1/g)) take(match[2])
  return found
}

describe('mọi tên biểu tượng trong mã của kit và các app đều có file SVG', () => {
  it('quét được mã nguồn và danh sách file', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(100)
    expect(files.has('trash')).toBe(true)
  })
  it('không còn tên nào rơi về vòng tròn gạch đứt', () => {
    const missing: string[] = []
    for (const [path, source] of Object.entries(sources)) {
      // noverse/commands/model.ts dùng bí danh ngắn (timer, swap…) do SlashPopover/Detail đổi sang tên file, không phải tên file.
      // viderse/editor/vendor là mã shadcn chép nguyên (`icon: "size-9"` là biến thể cva), không dùng <Icon> của kit.
      if (/\.test\.tsx?$/.test(path) || /^[^/]+\/assets\//.test(path) || path === 'noverse/commands/model.ts' || path.startsWith('viderse/editor/vendor/')) continue
      for (const name of literalIconNames(source)) if (!files.has(name)) missing.push(`${path}: ${name}`)
    }
    expect(missing).toEqual([])
  })
})
