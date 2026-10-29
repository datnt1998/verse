// Sinh src/iconNames.ts từ src/icons/*.svg — union tên cho <Icon name>.
import { readdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const names = readdirSync(`${root}/src/icons`).filter((f) => f.endsWith('.svg')).map((f) => f.slice(0, -4)).sort()
if (names.length < 50) throw new Error(`Chỉ có ${names.length} icon — kiểm tra src/icons`)
const kebab = /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/
for (const n of names) if (!kebab.test(n)) throw new Error(`Tên icon không hợp lệ: ${n}`)

writeFileSync(`${root}/src/iconNames.ts`, `// SINH TỰ ĐỘNG từ src/icons (npm run gen) — không sửa tay.
export const ICON_NAMES = ${JSON.stringify(names)} as const
export type IconName = ${names.map((n) => `'${n}'`).join(' | ')}
`)
console.log(`gen-icons: ${names.length} tên`)
