// Token parity: StyleX vars (tokens.stylex.ts) ↔ tokens.css ↔ tokens.json phải khớp tuyệt đối.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
// @ts-expect-error mjs script không có declaration — test chạy trong vite
import { flatTokens } from '../scripts/token-source.mjs'

const PKG = fileURLToPath(new URL('..', import.meta.url))
const tsx = readFileSync(`${PKG}/src/tokens.stylex.ts`, 'utf8')
const stylexTokens = flatTokens(tsx)

const cssTokens = Object.fromEntries(
  [...readFileSync(`${PKG}/tokens.css`, 'utf8').matchAll(/^\s+(--[\w-]+):\s*(.+?);\s*$/gm)].map((m) => [m[1], m[2]]),
)

/** Đọc tokens.json với kiểm tra hình dạng tối thiểu tại ranh giới. */
function tokensOf(raw: unknown): Record<string, string> {
  if (!raw || typeof raw !== 'object' || !('tokens' in raw) || !raw.tokens || typeof raw.tokens !== 'object') {
    throw new Error('tokens.json thiếu khoá "tokens"')
  }
  return Object.fromEntries(Object.entries(raw.tokens).map(([k, v]) => [k, String(v)]))
}
const jsonTokens = tokensOf(JSON.parse(readFileSync(`${PKG}/tokens.json`, 'utf8')))

describe('token parity: stylex ↔ css ↔ json', () => {
  it('stylex ↔ css cùng tập khoá và giá trị', () => {
    const missing = Object.keys(stylexTokens).filter((k) => !(k in cssTokens))
    const extra = Object.keys(cssTokens).filter((k) => !(k in stylexTokens))
    const changed = Object.keys(stylexTokens).filter((k) => k in cssTokens && cssTokens[k] !== stylexTokens[k])
    expect({ missing, extra, changed }).toEqual({ missing: [], extra: [], changed: [] })
  })
  it('css ↔ json cùng tập khoá và giá trị', () => {
    expect(jsonTokens).toEqual(cssTokens)
  })
})
