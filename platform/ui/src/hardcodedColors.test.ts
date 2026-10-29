// Chặn mã màu viết cứng trong gói @noverse/ui: màu mới phải vào src/tokens.stylex.ts.
// Bảng chốt chỉ đi xuống (token hoá dần). Thêm màu cứng → đỏ; bớt được → đỏ cho tới khi
// chốt lại bằng: UPDATE_COLOR_BASELINE=1 npx vitest run packages/ui/src/hardcodedColors.test.ts
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const SRC = fileURLToPath(new URL('.', import.meta.url))
const BASELINE = fileURLToPath(new URL('./hardcodedColors.baseline.json', import.meta.url))
const COLOR = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(/g
const SOURCE = /\.(?:tsx?|css)$/
// tokenDefaults.ts là bản dữ liệu SINH RA của tokens.stylex.ts + bảng vai trò brand —
// cùng nguồn token nên miễn như tokens.stylex.(ts|js); không phải nơi viết màu tự do.
const EXEMPT = /\.test\.tsx?$|^tokens\.stylex\.(ts|js)$|^tokenDefaults\.ts$/

function countColors(): Record<string, number> {
  const counts: Record<string, number> = {}
  for (const entry of readdirSync(SRC, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !SOURCE.test(entry.name)) continue
    const path = join(entry.parentPath, entry.name)
    const file = relative(SRC, path).split('\\').join('/')
    if (EXEMPT.test(file)) continue
    const n = readFileSync(path, 'utf8').match(COLOR)?.length ?? 0
    if (n > 0) counts[file] = n
  }
  return Object.fromEntries(Object.entries(counts).sort(([a], [b]) => a.localeCompare(b)))
}

describe('design token: không thêm mã màu viết cứng (gói)', () => {
  it('số mã màu mỗi tệp khớp bảng chốt', () => {
    const actual = countColors()
    if (process.env.UPDATE_COLOR_BASELINE) writeFileSync(BASELINE, `${JSON.stringify(actual, null, 2)}\n`)
    const baseline: Record<string, number> = JSON.parse(readFileSync(BASELINE, 'utf8'))
    const added = Object.entries(actual).filter(([f, n]) => n > (baseline[f] ?? 0)).map(([f, n]) => `${f}: ${baseline[f] ?? 0} → ${n}`)
    const removed = Object.entries(baseline).filter(([f, n]) => (actual[f] ?? 0) < n).map(([f, n]) => `${f}: ${n} → ${actual[f] ?? 0}`)
    expect(added, 'Mã màu mới viết cứng — dùng biến trong tokens.stylex.ts').toEqual([])
    expect(removed, 'Đã bớt mã màu — chốt lại bảng: UPDATE_COLOR_BASELINE=1 npx vitest run packages/ui/src/hardcodedColors.test.ts').toEqual([])
  })
})
