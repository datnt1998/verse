#!/usr/bin/env node
// CLI: sinh CSS brand cho consumer không dùng StyleX.
//   npx noverse-ui brand --primary '#7C5CFF' [--accent …] [--neutral-tint …]
//                       [--success …] [--warning …] [--danger …] [--info …]
//                       [--selector '.brand-acme'] [--out brand.css]
// CSS ra stdout (mặc định) để pipe: npx noverse-ui brand --primary … > brand.css
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

const PKG = new URL('..', import.meta.url).pathname

const USAGE = `noverse-ui brand — sinh CSS brand

  npx noverse-ui brand --primary '#7C5CFF' [tuỳ chọn]

Tuỳ chọn:
  --primary <hex>        màu thương hiệu chính (bắt buộc)
  --accent <hex>         accent ngữ nghĩa (mặc định = primary, tự sáng tới WCAG AA)
  --neutral-tint <hex>   nhuộm sắc trung tính theo brand
  --success|--warning|--danger|--info <hex>   màu trạng thái
  --role <tên>=<hex>     ghi đè vai trò ngữ nghĩa (lặp lại được)
  --override <--token>=<giá trị>   ghi đè token cuối (lặp lại được)
  --selector <css>       selector nhận biến (mặc định ':root')
  --out <file>           ghi ra tệp thay vì stdout
  --help                 in trợ giúp
`

/** Đọc cặp --cờ giá-trị; cờ không có giá trị là lỗi. */
function parseArgs(argv) {
  const out = { roles: {}, overrides: {} }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    const next = () => {
      const v = argv[++i]
      if (v == null) throw new Error(`${a} thiếu giá trị`)
      return v
    }
    if (a === '--help' || a === '-h') out.help = true
    else if (a === '--primary') out.primary = next()
    else if (a === '--accent') out.accent = next()
    else if (a === '--neutral-tint') out.neutralTint = next()
    else if (a === '--success') out.success = next()
    else if (a === '--warning') out.warning = next()
    else if (a === '--danger') out.danger = next()
    else if (a === '--info') out.info = next()
    else if (a === '--selector') out.selector = next()
    else if (a === '--out') out.out = next()
    else if (a === '--role' || a === '--override') {
      const raw = next()
      const eq = raw.indexOf('=')
      if (eq < 1) throw new Error(`${a} cần dạng tên=giá-trị, nhận: ${raw}`)
      const [k, v] = [raw.slice(0, eq), raw.slice(eq + 1)]
      if (a === '--role') out.roles[k] = v
      else out.overrides[k] = v
    } else throw new Error(`tham số không nhận ra: ${a}`)
  }
  return out
}

async function loadBrandModule() {
  const dist = join(PKG, 'dist', 'brand.js')
  try {
    readFileSync(dist)
  } catch {
    throw new Error(`chưa có dist/brand.js — chạy "npm run build" trong @noverse/ui trước (${dist})`)
  }
  return import(pathToFileURL(dist).href)
}

async function main() {
  let opts
  try {
    opts = parseArgs(process.argv.slice(2))
  } catch (e) {
    console.error(`noverse-ui: ${e.message}\n\n${USAGE}`)
    process.exit(2)
  }
  if (opts.help) { process.stdout.write(USAGE); return }
  if (!opts.primary) {
    console.error(`noverse-ui: thiếu --primary\n\n${USAGE}`)
    process.exit(2)
  }
  const { defineBrand, brandToCss, BrandInputError } = await loadBrandModule()
  try {
    const brand = defineBrand({
      primary: opts.primary,
      accent: opts.accent,
      neutralTint: opts.neutralTint,
      status: { success: opts.success, warning: opts.warning, danger: opts.danger, info: opts.info },
      roles: opts.roles,
      overrides: opts.overrides,
    })
    const css = brandToCss(brand, { selector: opts.selector })
    if (opts.out) writeFileSync(opts.out, css)
    else process.stdout.write(css)
  } catch (e) {
    if (e instanceof BrandInputError) {
      console.error(`noverse-ui: ${e.message}`)
      process.exit(1)
    }
    throw e
  }
}

main()
