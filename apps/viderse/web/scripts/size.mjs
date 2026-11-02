// Gate the real cold phone graph, including inline critical CSS. Assets moved
// into HTML still count; lazy editor code must stay out of both cold closures.
import { gzipSync } from 'node:zlib'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const limits = { entryJS: 150 * 1024, initialJS: 200 * 1024, initialCSS: 40 * 1024 }
const dist = new URL('../../internal/web/dist', import.meta.url).pathname
const manifest = JSON.parse(readFileSync(join(dist, '.vite/manifest.json'), 'utf8'))
const editorModules = JSON.parse(readFileSync(join(dist, '.vite/editor-budget.json'), 'utf8'))

const entry = Object.entries(manifest).find(([, meta]) => meta.isEntry)
if (!entry) {
  console.error('[size] không thấy entry trong manifest — build trước bằng npm run build')
  process.exit(1)
}

const phone = Object.entries(manifest).find(([, meta]) => meta.name === 'mobile-workbench')
if (!phone) throw new Error('[size] thiếu chunk Bàn dựng điện thoại')
const coldFiles = JSON.parse(readFileSync(join(dist, '.vite/cold-entries.json'), 'utf8'))
const bootDependencies = coldFiles.filter((file) => file !== phone[1].file).map((file) => {
  const entry = Object.entries(manifest).find(([, meta]) => meta.file === file)
  if (!entry) throw new Error(`[size] thiếu chunk khởi động trong manifest: ${file}`)
  return entry[0]
})
if (!bootDependencies.length) throw new Error('[size] thiếu trình dựng React DOM của lần mở đầu')
for (const [key, meta] of Object.entries(manifest)) {
  if ((meta.imports ?? []).includes(phone[0]) || (key !== entry[0] && (meta.dynamicImports ?? []).includes(phone[0]))) {
    throw new Error(`[size] route khác kéo theo màn điện thoại: ${key} → ${phone[0]}`)
  }
}
const family = Object.entries(manifest).find(([, meta]) => meta.name === 'phone-family')
if (family) {
  for (const [key, meta] of Object.entries(manifest)) {
    if ((meta.imports ?? []).includes(family[0]) && key !== phone[0] && !meta.src?.includes('screens/workbench/mobile/')) {
      throw new Error(`[size] route ngoài Bàn dựng kéo theo mã riêng điện thoại: ${key} → ${family[0]}`)
    }
  }
}
const visit = (key, chunks) => {
  if (chunks.has(key)) return
  chunks.add(key)
  for (const dep of manifest[key].imports ?? []) visit(dep, chunks)
}
const entryChunks = new Set()
visit(entry[0], entryChunks)
// Yielding renderer initialization still counts every boot byte.
for (const key of bootDependencies) visit(key, entryChunks)
const initialChunks = new Set(entryChunks)
visit(phone[0], initialChunks)
const compressedJS = new Map()
const cssFiles = new Set()
for (const key of initialChunks) {
  const meta = manifest[key]
  if (!Object.hasOwn(editorModules, meta.file)) throw new Error(`[size] thiếu nguồn gốc module cho ${meta.file} — build lại ứng dụng`)
  if (editorModules[meta.file].length) throw new Error(`[size] chunk đầu chứa mã editor: ${meta.file}\n${editorModules[meta.file].join('\n')}`)
  const gz = gzipSync(readFileSync(join(dist, meta.file)), { level: 9 }).length
  compressedJS.set(key, gz)
  for (const css of meta.css ?? []) cssFiles.add(css)
  console.log(`  ${meta.file}  ${(gz / 1024).toFixed(1)} KiB`)
}
const html = readFileSync(join(dist, 'index.html'), 'utf8')
const inlineCSS = html.match(/<style\b[^>]*\bid="viderse-stylex-critical"[^>]*>([\s\S]*?)<\/style>/i)?.[1]
if (!inlineCSS) throw new Error('[size] thiếu CSS critical inline trong HTML đã build')
let cssBytes = gzipSync(Buffer.from(inlineCSS), { level: 9 }).length
for (const file of cssFiles) cssBytes += gzipSync(readFileSync(join(dist, file)), { level: 9 }).length
const measured = {
  entryJS: [...entryChunks].reduce((sum, key) => sum + compressedJS.get(key), 0),
  initialJS: [...compressedJS.values()].reduce((sum, size) => sum + size, 0),
  initialCSS: cssBytes,
}
let failed = false
for (const [metric, bytes] of Object.entries(measured)) {
  console.log(`[size] ${metric}: ${(bytes / 1024).toFixed(1)} KiB gzip; ngưỡng ${limits[metric] / 1024} KiB`)
  if (bytes > limits[metric]) failed = true
}
if (failed) {
  console.error('[size] VƯỢT ngân sách mobile — giảm tải thật, không dời byte ra khỏi phép đo')
  process.exit(1)
}
console.log('[size] cold entry + Bàn dựng điện thoại không chứa mã editor; CSS inline được tính đầy đủ')
