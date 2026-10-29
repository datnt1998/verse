// Ranh giới gói @noverse/ui:
//  1. Mã trong packages/ui KHÔNG được import vào app (mọi import tương đối phải giải trong gói).
//  2. App KHÔNG được với vào packages/ui/src hay src/assets/icons (đã chuyển): chỉ đi qua '@noverse/ui…'.
//  3. Không shim: web/src/ui không được re-export từ '@noverse/ui'.
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const PKG_SRC = fileURLToPath(new URL('.', import.meta.url))
const PKG = dirname(PKG_SRC)
const WEB = join(PKG, '..', '..')
const APP_SRC = join(WEB, 'src')

const SOURCE = /\.tsx?$/
const IMPORT = /(?:import|export)[\s\S]*?from\s+(['"])([^'"]+)\1|import\s*\(\s*(['"])([^'"]+)\3|import\s+(['"])([^'"]+)\5/g

function* files(root: string): Generator<string> {
  for (const entry of readdirSync(root, { recursive: true, withFileTypes: true })) {
    if (entry.isFile() && SOURCE.test(entry.name)) yield join(entry.parentPath, entry.name)
  }
}

function specifiers(file: string): string[] {
  const text = readFileSync(file, 'utf8')
  const out: string[] = []
  for (const m of text.matchAll(IMPORT)) out.push(m[2] ?? m[4] ?? m[6])
  return out
}

describe('ranh giới @noverse/ui', () => {
  it('gói không import vào app', () => {
    const bad: string[] = []
    for (const file of files(PKG_SRC)) {
      for (const spec of specifiers(file)) {
        if (spec.startsWith('.')) {
          const target = resolve(dirname(file), spec)
          if (!target.startsWith(PKG)) bad.push(`${file.slice(PKG.length + 1)} → ${spec}`)
        } else if (!spec.startsWith('@stylexjs/') && !spec.startsWith('node:') && !['react', 'react-dom', 'react-dom/client', 'react-dom/server', 'react/jsx-runtime', 'react/jsx-dev-runtime', 'qrcode-generator', 'vitest'].includes(spec)) {
          bad.push(`${file.slice(PKG.length + 1)} → ${spec}`)
        }
      }
    }
    expect(bad, 'gói đang import ngoài ranh giới').toEqual([])
  })

  it('app chỉ đến kit qua tên gói @noverse/ui', () => {
    const bad: string[] = []
    for (const file of files(APP_SRC)) {
      for (const spec of specifiers(file)) {
        if (spec.includes('packages/ui') || spec.startsWith('@noverse/ui/src')) bad.push(`${file.slice(APP_SRC.length + 1)} → ${spec}`)
        if (spec.endsWith('/assets/icons') || spec.includes('assets/icons')) bad.push(`${file.slice(APP_SRC.length + 1)} → ${spec}`)
        if (spec.endsWith('styles/tokens.stylex')) bad.push(`${file.slice(APP_SRC.length + 1)} → ${spec}`)
      }
    }
    expect(bad, 'app đang với vào nội bộ gói').toEqual([])
  })

  it('web/src/ui không re-export từ gói (không shim)', () => {
    const bad: string[] = []
    const uiDir = join(APP_SRC, 'ui')
    const REEXPORT = /export\s+(?:\*|\{[\s\S]*?\})\s+from\s+(['"])(@noverse\/ui[^'"]*)\1/g
    try {
      for (const file of files(uiDir)) {
        const text = readFileSync(file, 'utf8')
        for (const m of text.matchAll(REEXPORT)) bad.push(`${file.slice(APP_SRC.length + 1)} → export from ${m[2]}`)
        // re-export gián tiếp: import X từ gói rồi export { X }
        const imported = new Set<string>()
        for (const m of text.matchAll(/import\s+\{([\s\S]*?)\}\s+from\s+(['"])@noverse\/ui[^'"]*\2/g)) {
          for (const part of m[1].split(',')) imported.add(part.trim().split(/\s+as\s+/).pop()!.trim())
        }
        for (const m of text.matchAll(/export\s*\{([\s\S]*?)\}/g)) {
          for (const part of m[1].split(',')) {
            const name = part.trim().split(/\s+as\s+/)[0].trim().replace(/^type\s+/, '')
            if (name && imported.has(name)) bad.push(`${file.slice(APP_SRC.length + 1)} → export { ${name} } (shim)`)
          }
        }
      }
    } catch { /* thư mục đã xoá */ }
    expect(bad, 'lớp shim re-export trong web/src/ui').toEqual([])
  })
})
