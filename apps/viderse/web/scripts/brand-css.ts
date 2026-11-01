// Sinh src/styles/brand.css từ src/brand.ts — bảng token của brand Viderse.
// Chạy: npx vite-node scripts/brand-css.ts  (từ apps/viderse/web)
//
// Vì sao không gọi applyBrand() lúc chạy: defineBrand kéo theo tokenDefaults.ts
// (~45 kB bảng token) và toán màu của kit vào entry, trong khi kết quả là hằng
// số. Đưa vào CSS thì entry không còn phải parse/tính gì cho brand.
// Test src/styles/brand.test.ts canh tệp sinh ra khớp src/brand.ts.
import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { brandToCss } from '@noverse/ui'
import { brand } from '../src/brand'

const target = fileURLToPath(new URL('../src/styles/brand.css', import.meta.url))
const header = '/* SINH TỰ ĐỘNG từ src/brand.ts — npx vite-node scripts/brand-css.ts. KHÔNG sửa tay. */\n'
writeFileSync(target, header + brandToCss(brand))
console.log(`brand.css: ${Object.keys(brand.vars).length} token`)
