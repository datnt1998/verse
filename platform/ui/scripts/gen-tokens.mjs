// Sinh tokens.css + tokens.json từ src/tokens.stylex.ts (npm run gen).
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { flatTokens, parseTokens } from './token-source.mjs'
import { FIXED_TOKENS, ROLE_LITERALS } from './brand-roles.mjs'

const root = fileURLToPath(new URL('..', import.meta.url))
const source = readFileSync(`${root}/src/tokens.stylex.ts`, 'utf8')
const groups = parseTokens(source)
const flat = flatTokens(source)
if (Object.keys(flat).length < 100) throw new Error(`Chỉ đọc được ${Object.keys(flat).length} token — parser lệch cú pháp nguồn`)

const css = `/* SINH TỰ ĐỘNG từ src/tokens.stylex.ts (npm run gen) — không sửa tay.
   Consumer không dùng StyleX import tệp này để có cùng biến :root như app. */
:root {
${Object.entries(flat).map(([k, v]) => `  ${k}: ${v};`).join('\n')}
}
`
writeFileSync(`${root}/tokens.css`, css)
writeFileSync(`${root}/tokens.json`, `${JSON.stringify({ $schema: 'noverse/ui tokens v1', groups: Object.keys(groups), tokens: flat }, null, 2)}\n`)
writeFileSync(`${root}/src/tokens.stylex.js`, `// SINH TỰ ĐỘNG: bản JS của tokens.stylex.ts cho trình phân tích Babel của StyleX (npm run gen). KHÔNG sửa tay.\n${source}`)
// tokenDefaults.ts: bản dữ liệu của tokens.stylex.ts + bảng vai trò brand cho defineBrand.
// File sinh — được miễn ratchet hardcodedColors như tokens.stylex.js.
writeFileSync(`${root}/src/tokenDefaults.ts`, `// SINH TỰ ĐỘNG từ tokens.stylex.ts + scripts/brand-roles.mjs (npm run gen). KHÔNG sửa tay.
export const TOKEN_GROUPS: Record<string, Record<string, string>> = ${JSON.stringify(groups, null, 2)}
export const TOKEN_DEFAULTS: Record<string, string> = ${JSON.stringify(flat, null, 2)}
export const ROLE_LITERALS = ${JSON.stringify(ROLE_LITERALS, null, 2)} as const
export const FIXED_TOKENS: ReadonlySet<string> = new Set(${JSON.stringify([...FIXED_TOKENS])})
`)
console.log(`gen-tokens: ${Object.keys(flat).length} biến / ${Object.keys(groups).length} nhóm`)
