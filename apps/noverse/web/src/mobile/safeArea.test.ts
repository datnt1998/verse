// Bug 2710 — vùng an toàn của khung điện thoại.
//
// Artboard (MWork.dc.html › `.mroot` + `.safe-top{height:40px}`) vẽ 40px thanh trạng thái giả;
// trên máy thật `env(safe-area-inset-*)` mới là số đúng. Lỗi người dùng thấy: appbar bị đẩy
// xuống ~100pt vì inset bị cộng hai lần (thanh trạng thái của iOS + 40px của artboard), và
// composer/chip lệnh đè lên thanh tab vì `bottom` viết cứng 84px trong khi thanh tab ngồi ở
// `max(20px, env(safe-area-inset-bottom))`.
//
// Bài kiểm tra đọc thẳng CSS: chỉ một nơi khai báo inset, `.m-app` cộng đúng một lần, và mọi
// thứ nổi phía trên thanh tab dùng chung `--m-tabbar-bottom`.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')

const mobileCss = read('./mobile.css')
const tabletCss = read('../tablet/tablet.css')
const phoneCss = read('../screens/auth/mobile/phone.css')
const cmdresultsCss = read('../screens/cmdresults/mobile/mobile.css')
const workspaceCss = read('../screens/workspace/mobile/workspace.css')
const readerCss = read('../screens/reader/mobile/reader.css')
const folderBrowser = read('../screens/dialogs/FolderBrowserDialog.tsx')
const indexHtml = read('../../index.html')

// Dải mờ trên cùng iOS 26 (Gói AJ): toàn bộ web đọc env() đúng một lần qua --m-safe-top.
const allShellSources = {
  'mobile.css': mobileCss,
  'tablet.css': tabletCss,
  'phone.css': phoneCss,
  'cmdresults/mobile.css': cmdresultsCss,
  'workspace/mobile.css': workspaceCss,
  'reader/mobile.css': readerCss,
  'FolderBrowserDialog.tsx': folderBrowser,
}

const ruleOf = (css: string, selector: string): string => {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return [...css.matchAll(new RegExp(`${escaped}\\{([^}]*)\\}`, 'g'))].map(m => m[1]).join(';')
}

describe('vùng an toàn điện thoại', () => {
  it('không tự khai báo lại vùng an toàn: một nguồn ở kit', () => {
    // Khai báo gốc chuyển sang platform/ui/src/kit.css (test hợp đồng nằm ở
    // platform/ui/src/mobile/safeArea.test.ts). Ở đây chỉ còn nhịp riêng của Noverse.
    const root = ruleOf(mobileCss, ':root')
    expect(root).toContain('--m-tabbar-gap:20px')
    expect(root).toContain('--m-scroll-pad:calc(var(--m-tabbar-bottom) + 92px)')
    expect(mobileCss).not.toMatch(/env\(safe-area-inset/)

    // Mọi tệp khung điện thoại đọc var(--m-*), không đọc lại env() — đọc lần nữa là cộng dồn
    // và né trật dải mờ Liquid Glass ~40pt của iOS 26 (lỗi 28/10).
    const forbidden = /env\(safe-area-inset/
    for (const [file, src] of Object.entries(allShellSources)) {
      expect(src, `${file} dùng thẳng env(safe-area-inset-*)`).not.toMatch(forbidden)
    }
    expect(mobileCss).not.toContain('--m-safe-top:')
  })

  it('khung app cộng inset đúng một lần, không sàn 40px của artboard', () => {
    // Quy tắc --m-edge/--m-top và dự phòng data-standalone nằm ở kit.css; Noverse vẫn phải
    // cộng đúng một lần qua var(--m-top) và giữ script pre-paint trong index.html.
    expect(indexHtml).toMatch(/navigator\.standalone/)
    expect(indexHtml).toMatch(/data-standalone|dataset\.standalone/)

    const app = ruleOf(mobileCss, '.m-app')
    expect(app).toContain('padding-top:var(--m-top)')
    expect(app).not.toMatch(/env\(/)
    expect(app).not.toMatch(/padding-top:calc\(/)
    expect(mobileCss).not.toContain('max(40px')
    // Cuối danh sách chừa đúng thanh tab + khoảng thở (112px khi inset 0), không viết cứng.
    expect(ruleOf(mobileCss, '.m-scroll')).toContain('padding-bottom:var(--m-scroll-pad)')
  })

  it('thanh tab ở đúng vùng an toàn dưới, cao 64', () => {
    const tabs = ruleOf(mobileCss, '.m-tabs')
    expect(tabs).toContain('bottom:var(--m-tabbar-bottom)')
    expect(tabs).toContain('height:var(--m-tabbar-h)')
  })

  it('composer và chân trang đọc cùng một gốc thanh tab, không viết cứng 84px', () => {
    const expected = 'bottom:calc(var(--m-tabbar-bottom) + var(--m-tabbar-h))'
    expect(ruleOf(workspaceCss, '.mw-composer-wrap')).toContain(expected)
    expect(ruleOf(readerCss, '.mr-root.mr-root .m-footer')).toContain(expected)
  })

  it('PWA vẽ tràn dưới thanh trạng thái nên env() chính là inset', () => {
    expect(indexHtml).toContain('content="width=device-width, initial-scale=1, viewport-fit=cover"')
    expect(indexHtml).toContain('name="apple-mobile-web-app-status-bar-style" content="black-translucent"')
  })
})
