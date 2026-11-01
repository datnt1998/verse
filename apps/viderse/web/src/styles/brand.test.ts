// Canh tệp CSS sinh sẵn khớp brand thật: brand.css là nguồn duy nhất ghi token
// brand lúc chạy (main.tsx nạp nó thay cho applyBrand), nên lệch nghĩa là app
// chạy sai màu. Sửa src/brand.ts → chạy `npx vite-node scripts/brand-css.ts`.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { brandToCss } from '@noverse/ui'
import { brand } from '../brand'

// jsdom không cho `import.meta.url` là file URL, nên đọc theo gốc dự án (vitest
// chạy với cwd = apps/viderse/web).
const file = join(process.cwd(), 'src/styles/brand.css')

it('brand.css khớp bảng token của src/brand.ts', () => {
  // Dòng đầu là chú thích "SINH TỰ ĐỘNG" của scripts/brand-css.ts.
  const generated = readFileSync(file, 'utf8').split('\n').slice(1).join('\n')
  expect(generated).toBe(brandToCss(brand))
})

it('mọi token của brand đều có trong CSS — không token nào chỉ sống trong JS', () => {
  const css = readFileSync(file, 'utf8')
  const missing = Object.keys(brand.vars).filter((name) => !css.includes(`  ${name}: `))
  expect(missing).toEqual([])
})
