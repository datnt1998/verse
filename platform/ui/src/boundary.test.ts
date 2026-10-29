// Ranh giới gói @noverse/ui:
//  1. Mã trong platform/ui KHÔNG được import vào app (mọi import tương đối phải giải trong gói).
//  2. App KHÔNG được với vào platform/ui/src hay src/assets/icons (đã chuyển): chỉ đi qua '@noverse/ui…'.
//  3. Không shim: <app>/web/src/ui không được re-export từ '@noverse/ui'.
// Luật 2 và 3 áp cho mọi app dùng kit (test/consumers.ts).
import { readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { consumers, PKG, PKG_SRC, sourceFiles } from '../test/consumers'

const IMPORT = /(?:import|export)[\s\S]*?from\s+(['"])([^'"]+)\1|import\s*\(\s*(['"])([^'"]+)\3|import\s+(['"])([^'"]+)\5/g

function specifiers(file: string): string[] {
  const text = readFileSync(file, 'utf8')
  const out: string[] = []
  for (const m of text.matchAll(IMPORT)) out.push(m[2] ?? m[4] ?? m[6])
  return out
}

describe('ranh giới @noverse/ui', () => {
  it('có app dùng kit để kiểm', () => {
    expect(consumers().map((c) => c.app)).toContain('noverse')
  })

  it('gói không import vào app', () => {
    const bad: string[] = []
    for (const file of sourceFiles(PKG_SRC)) {
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
    for (const { app, src } of consumers()) {
      for (const file of sourceFiles(src)) {
        const at = `${app}/${file.slice(src.length + 1)}`
        for (const spec of specifiers(file)) {
          if (spec.includes('platform/ui') || spec.startsWith('@noverse/ui/src')) bad.push(`${at} → ${spec}`)
          if (spec.endsWith('/assets/icons') || spec.includes('assets/icons')) bad.push(`${at} → ${spec}`)
          if (spec.endsWith('styles/tokens.stylex')) bad.push(`${at} → ${spec}`)
        }
      }
    }
    expect(bad, 'app đang với vào nội bộ gói').toEqual([])
  })

  it('<app>/web/src/ui không re-export từ gói (không shim)', () => {
    const bad: string[] = []
    const REEXPORT = /export\s+(?:\*|\{[\s\S]*?\})\s+from\s+(['"])(@noverse\/ui[^'"]*)\1/g
    for (const { app, src } of consumers()) {
      for (const file of sourceFiles(join(src, 'ui'))) {
        const at = `${app}/${file.slice(src.length + 1)}`
        const text = readFileSync(file, 'utf8')
        for (const m of text.matchAll(REEXPORT)) bad.push(`${at} → export from ${m[2]}`)
        // re-export gián tiếp: import X từ gói rồi export { X }
        const imported = new Set<string>()
        for (const m of text.matchAll(/import\s+\{([\s\S]*?)\}\s+from\s+(['"])@noverse\/ui[^'"]*\2/g)) {
          for (const part of m[1].split(',')) imported.add(part.trim().split(/\s+as\s+/).pop()!.trim())
        }
        for (const m of text.matchAll(/export\s*\{([\s\S]*?)\}/g)) {
          for (const part of m[1].split(',')) {
            const name = part.trim().split(/\s+as\s+/)[0].trim().replace(/^type\s+/, '')
            if (name && imported.has(name)) bad.push(`${at} → export { ${name} } (shim)`)
          }
        }
      }
    }
    expect(bad, 'lớp shim re-export trong <app>/web/src/ui').toEqual([])
  })
})
