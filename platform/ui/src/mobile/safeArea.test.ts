// Hợp đồng vùng an toàn — R-SA1…R-SA9 (plan viderse-mobile-standard).
//
// Một nguồn duy nhất: `platform/ui/src/kit.css` được đọc `env(safe-area-inset-*)`.
// Mọi tệp CSS/TSX khác trong kit và trong hai app chỉ được đọc `var(--m-*)`.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'
import { PKG, PKG_SRC } from '../../test/consumers'
import { pickToastBottom, tabModeFor, PHONE_MAX_NOVERSE, PHONE_MAX_VIDERSE, phoneQuery } from './safeArea'

const REPO = dirname(dirname(PKG))
const KIT_CSS = join(PKG_SRC, 'kit.css')
const APP_ROOTS = [join(REPO, 'apps', 'noverse', 'web'), join(REPO, 'apps', 'viderse', 'web')]

function walk(root: string, out: string[] = []): string[] {
  for (const entry of readdirSync(root)) {
    if (entry === 'node_modules' || entry === 'dist' || entry.startsWith('.')) continue
    const path = join(root, entry)
    if (statSync(path).isDirectory()) walk(path, out)
    else if (/\.(css|ts|tsx|html)$/.test(entry)) out.push(path)
  }
  return out
}

const read = (path: string) => readFileSync(path, 'utf8')
/** Bỏ chú thích trước khi quét: tài liệu trong mã được phép nhắc tới env()/tên biến. */
const code = (path: string) => read(path).replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '')

describe('vùng an toàn: một nguồn khai báo', () => {
  const files = [...walk(PKG_SRC), ...APP_ROOTS.flatMap((root) => walk(root))]
  const envUsers = files.filter((path) => !path.endsWith('.test.ts') && !path.endsWith('.test.tsx') && code(path).includes('env(safe-area-inset'))

  it('chỉ kit.css đọc env(safe-area-inset-*), đúng hai lần (top + bottom)', () => {
    expect(envUsers.map((path) => relative(REPO, path))).toEqual(['platform/ui/src/kit.css'])
    expect(code(KIT_CSS).match(/env\(safe-area-inset-(top|bottom)/g)).toHaveLength(2)
  })

  it('khai báo đủ bộ biến, mỗi biến đúng một lần', () => {
    const css = code(KIT_CSS)
    for (const name of ['--m-safe-top', '--m-safe-bottom', '--m-edge', '--m-top', '--m-tabbar-h', '--m-tabbar-bottom', '--m-float-bottom', '--m-scroll-pad', '--kb', '--m-toast-bottom']) {
      const count = (css.match(new RegExp(`${name}\\s*:`, 'g')) ?? []).length
      // --m-top/--m-scroll-pad/--m-float-bottom/--m-toast-bottom có thêm biến thể
      // ở data-tab-mode="hide" (màn không thanh tab thì không giữ chỗ cho nó).
      const allowed = name === '--m-edge' ? 4 : ['--m-scroll-pad', '--m-float-bottom', '--m-toast-bottom'].includes(name) ? 2 : 1
      expect(count, `${name} khai báo ${count} lần`).toBe(allowed)
    }
    expect(css).toMatch(/--m-tabbar-h:\s*64px/)
    expect(css).toMatch(/--m-tabbar-bottom:\s*max\(var\(--m-tabbar-gap,\s*20px\),\s*var\(--m-safe-bottom\)\)/)
    expect(css).toMatch(/--m-float-bottom:\s*calc\(var\(--m-tabbar-bottom\) \+ var\(--m-tabbar-h\) \+ 8px\)/)
    expect(css).toMatch(/--m-scroll-pad:\s*calc\(var\(--m-tabbar-bottom\) \+ var\(--m-tabbar-h\) \+ 24px\)/)
  })

  it('mỗi app đặt nhịp thanh tab của mình, không tự khai báo lại vùng an toàn', () => {
    const noverse = read(join(REPO, 'apps', 'noverse', 'web', 'src', 'mobile', 'mobile.css'))
    const viderse = read(join(REPO, 'apps', 'viderse', 'web', 'src', 'styles', 'viderse.css'))
    expect(noverse).not.toContain('--m-safe-top:')
    expect(noverse).toMatch(/--m-tabbar-gap:\s*20px/)
    expect(noverse).toMatch(/--m-scroll-pad:\s*calc\(var\(--m-tabbar-bottom\) \+ 92px\)/)
    expect(viderse).not.toContain('--m-safe-top:')
    expect(viderse).toMatch(/html:root\s*\{[^}]*--m-tabbar-gap:\s*30px/)
  })

  it('cả hai index.html dùng viewport-fit=cover và chạy script data-standalone trước khi vẽ', () => {
    for (const app of ['noverse', 'viderse']) {
      const html = read(join(REPO, 'apps', app, 'web', 'index.html'))
      expect(html, app).toContain('viewport-fit=cover')
      expect(html, app).toContain('data-standalone')
    }
  })

  it('kit Toast không còn mốc 759px', () => {
    expect(read(join(PKG_SRC, 'Toast.tsx'))).not.toMatch(/759/)
    expect(read(join(PKG_SRC, 'Toast.tsx'))).toContain('phoneQuery(getPhoneMax())')
  })
})

describe('nhịp điện thoại', () => {
  it('mốc bề ngang là hằng số có tên', () => {
    expect(phoneQuery(PHONE_MAX_NOVERSE)).toBe('(max-width: 640px)')
    expect(phoneQuery(PHONE_MAX_VIDERSE)).toBe('(max-width: 767px)')
  })

  it('hide thắng mini; bàn phím mở luôn ẩn', () => {
    expect(tabModeFor({})).toBe('default')
    expect(tabModeFor({ down: true })).toBe('mini')
    expect(tabModeFor({ down: true, immersive: true })).toBe('hide')
    expect(tabModeFor({ down: true, hidden: true })).toBe('hide')
    expect(tabModeFor({ keyboard: 320 })).toBe('hide')
  })

  it('neo toast: ưu tiên cao thắng, cùng ưu tiên thì mục sau thắng', () => {
    expect(pickToastBottom([])).toBeNull()
    expect(pickToastBottom([{ px: 96, priority: 0 }])).toBe(96)
    expect(pickToastBottom([{ px: 96, priority: 0 }, { px: 148, priority: 1 }])).toBe(148)
    expect(pickToastBottom([{ px: 96, priority: 1 }, { px: 148, priority: 1 }])).toBe(148)
  })
})
