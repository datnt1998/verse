// Chặn hồi quy: mỗi tab chỉ được mở MỘT kết nối SSE dài (/api/events).
// HTTP/1.1 giới hạn 6 kết nối/nguồn; mở thêm luồng cho worker/audio làm request
// xếp hàng và màn hình trông như mất dữ liệu rồi có lại. Mọi sự kiện thực thể đi
// qua luồng chung dưới dạng khung `live` (xem web/src/data/events.ts).
import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'

const root = join(__dirname, '..')
const files: string[] = []
const walk = (dir: string) => {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) walk(path)
    else if (/\.(ts|tsx)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name)) files.push(path)
  }
}
walk(root)

/** Bỏ chú thích trước khi soi: tài liệu nhắc tên endpoint không phải là mở luồng. */
const code = (path: string) => readFileSync(path, 'utf8')
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .replace(/(^|[^:])\/\/.*$/gm, '$1')

describe('một luồng SSE cho mỗi tab', () => {
  it('chỉ client toàn cục được tạo luồng sự kiện; màn hình không tự mở EventSource', () => {
    const offenders = files
      .filter((path) => /new\s+EventSource\s*\(/.test(code(path)))
      .map((path) => relative(root, path))
    // data/events.ts giữ đường dự phòng cho môi trường không có fetch-stream;
    // singleton `events` thật ra dùng fetchSource nên production chỉ có một kết nối.
    expect(offenders).toEqual(['data/events.ts'])
  })

  it('không màn hình nào dùng lại URL SSE riêng của worker/audio', () => {
    const offenders = files
      .filter((path) => /events\/live|audio\/events/.test(code(path)))
      .map((path) => relative(root, path))
    expect(offenders).toEqual([])
  })
})
