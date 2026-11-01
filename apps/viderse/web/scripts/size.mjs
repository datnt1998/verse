// Đo JS tải lần đầu (entry + import tĩnh theo manifest) ở dạng gzip.
// Ngưỡng 150 kB theo ngân sách mobile đã nhận — quá ngưỡng thì thoát mã 1.
import { gzipSync } from 'node:zlib'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const LIMIT_KB = 150
const dist = new URL('../../internal/web/dist', import.meta.url).pathname
const manifest = JSON.parse(readFileSync(join(dist, '.vite/manifest.json'), 'utf8'))
const editorModules = JSON.parse(readFileSync(join(dist, '.vite/editor-budget.json'), 'utf8'))

const entry = Object.entries(manifest).find(([, meta]) => meta.isEntry)
if (!entry) {
  console.error('[size] không thấy entry trong manifest — build trước bằng npm run build')
  process.exit(1)
}

const chunks = new Set()
const visit = (key) => {
  if (chunks.has(key)) return
  chunks.add(key)
  for (const dep of manifest[key].imports ?? []) visit(dep)
}
visit(entry[0])

let bytes = 0
for (const key of chunks) {
  const file = manifest[key].file
  if (!Object.hasOwn(editorModules, file)) {
    throw new Error(`[size] thiếu nguồn gốc module cho ${file} — cần build lại ứng dụng`)
  }
  if (editorModules[file].length) {
    throw new Error(`[size] chunk đầu chứa mã editor: ${file}\n${editorModules[file].join('\n')}`)
  }
  const gz = gzipSync(readFileSync(join(dist, file)), { level: 9 }).length
  bytes += gz
  console.log(`  ${file}  ${(gz / 1024).toFixed(1)} kB`)
}
const kb = bytes / 1024
console.log(`[size] JS tải lần đầu: ${kb.toFixed(1)} kB gzip (${chunks.size} chunk, ngưỡng ${LIMIT_KB} kB)`)
if (kb > LIMIT_KB) {
  console.error(`[size] VƯỢT ngưỡng ${LIMIT_KB} kB — tách thêm màn hoặc giảm phụ thuộc`)
  process.exit(1)
}
console.log('[size] chunk đầu không chứa mã editor; editor/render/measure chỉ tải ở route lazy')
