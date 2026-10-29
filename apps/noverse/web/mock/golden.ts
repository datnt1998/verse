import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

// Golden JSON do các test Go ghi (internal/entry/web, internal/lessons,
// internal/ledger — cờ -update). Mock phát lại đúng output của handler thật;
// không tính lại rule nào ở đây — chỉ áp trạng thái đổi do người dùng bấm.
const DIR = join(dirname(fileURLToPath(import.meta.url)), 'golden')
const cache = new Map<string, any>()

export function golden<T>(name: string): T {
  let v = cache.get(name)
  if (v === undefined) {
    v = JSON.parse(readFileSync(join(DIR, name), 'utf8'))
    cache.set(name, v)
  }
  return v as T
}
