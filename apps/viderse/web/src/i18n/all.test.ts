// Canh việc tách chữ theo màn: entry chỉ được nạp chữ của khung app (./vi), còn
// bảng gộp đủ mọi màn (./all) là của test. Lỡ một tệp production import ./all là
// cả 13 mảng chữ quay lại entry — test này chặn đúng lỗi đó.
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { t as all } from './all'
import { t as shell } from './vi'

const srcRoot = join(process.cwd(), 'src')

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return entry.name === 'node_modules' ? [] : sourceFiles(path)
    return /\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name) ? [path] : []
  })
}

it('không tệp production nào nạp bảng chữ đầy đủ', () => {
  const offenders = sourceFiles(srcRoot)
    .filter((path) => /from '[^']*i18n\/all'/.test(readFileSync(path, 'utf8')))
    .map((path) => path.slice(srcRoot.length))
  expect(offenders).toEqual([])
})

it('chữ khung app chỉ giữ đúng khoá màn khác mà khung app đọc', () => {
  // Khung app đọc tiêu đề màn (tab, tiêu đề trang), cảnh báo thiếu công cụ và
  // tên kênh trống — chỉ ngần ấy khoá, không kéo cả khối của màn nào.
  expect(Object.keys(shell.channels)).toEqual(['page', 'create'])
  expect(Object.keys(shell.library)).toEqual(['titleShort', 'tabs'])
  expect(Object.keys(shell.prep)).toEqual(['titleShort', 'skippedWarn', 'skippedAction'])
  expect(Object.keys(shell.screens)).toEqual(['workbench', 'review', 'queue', 'explore', 'new', 'editor', 'settings'])
  const screenOnly = ['ai', 'accounts', 'settings', 'tools', 'storage', 'logs', 'backdrop', 'autonomy', 'learn', 'errors', 'devkit', 'auth', 'setup']
  expect(screenOnly.filter((key) => key in shell)).toEqual([])
})

it('bảng gộp của test có đủ khối của mọi màn', () => {
  const everyScreen = ['ai', 'accounts', 'settings', 'tools', 'storage', 'logs', 'prep', 'library', 'backdrop', 'channels', 'autonomy', 'learn', 'errors', 'devkit', 'auth', 'setup']
  expect(everyScreen.filter((key) => !(key in all))).toEqual([])
})
