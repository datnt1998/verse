// @vitest-environment node
// Kiểm tra luật thiết kế (08-dev-prompt V12): không hex cứng / bán kính cứng
// ngoài tệp token; không border-left/right (đường kẻ tách chỉ là line ngang
// hay viền đều bốn phía); mỗi route chỉ một ActionButton primary (kiểm ở
// dev-kit không tính vì đó là trưng bày).
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const SRC = new URL('..', import.meta.url).pathname

const files: string[] = []
const walk = (dir: string) => {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry)
    const stat = statSync(path)
    if (stat.isDirectory()) walk(path)
    else if (/\.(ts|tsx|css)$/.test(entry) && !entry.endsWith('.test.ts') && !entry.endsWith('.test.tsx')) files.push(path)
  }
}
walk(SRC)

// Tệp được phép chứa hex cứng: tệp token, bộ icon SVG (đồ hoạ), brand.
const TOKEN_FILES = /tokens\.css|viderse\.css|brand\.ts$|LensLogo\.tsx$|\/icons\//

describe('luật thiết kế trong src/', () => {
  it('không màu hex cứng ngoài token/đồ hoạ', () => {
    const offenders: string[] = []
    for (const file of files) {
      if (TOKEN_FILES.test(file)) continue
      const text = readFileSync(file, 'utf8')
      if (/#[0-9a-fA-F]{3,8}\b/.test(text)) offenders.push(file.replace(`${SRC}/`, ''))
    }
    expect(offenders).toEqual([])
  })
  it('không bán kính số cứng ngoài vòng tròn 50% (CSS lẫn JSX)', () => {
    const offenders: string[] = []
    for (const file of files) {
      if (!file.endsWith('.tsx') && !file.endsWith('.css')) continue
      const text = readFileSync(file, 'utf8')
      // CSS: border-radius: <N>px · JSX: borderRadius: <N> | '<N>px'
      for (const match of text.matchAll(/border-radius:\s*[0-9.]+px|borderRadius:\s*(?:[0-9.]+|['"][0-9.]+px['"])/g)) {
        offenders.push(`${file.replace(`${SRC}/`, '')}: ${match[0]}`)
      }
    }
    expect(offenders).toEqual([])
  })
  it('không khoảng cách số cứng (padding/margin/gap/vị trí) — cả CSS lẫn JSX', () => {
    const offenders: string[] = []
    // JSX dạng số: bỏ qua 0 (reset hợp lệ, không phải bước khoảng cách).
    const num = String.raw`(?:[1-9][0-9]*(?:\.[0-9]+)?|0\.[0-9]*[1-9][0-9]*)`
    const cssProp = String.raw`(?:gap|padding(?:-(?:top|right|bottom|left))?|margin(?:-(?:top|right|bottom|left))?)`
    const jsxProp = String.raw`(?:gap|padding(?:Left|Right|Top|Bottom)?|margin(?:Left|Right|Top|Bottom)?|inset)`
    for (const file of files) {
      if (!file.endsWith('.tsx') && !file.endsWith('.css')) continue
      // Tệp token là nơi định nghĩa bước khoảng cách — miễn.
      if (/viderse\.css$|tokens\.css$/.test(file)) continue
      const text = readFileSync(file, 'utf8')
      const patterns = file.endsWith('.css')
        ? [new RegExp(String.raw`(?<![-\w])${cssProp}:\s*[^;}]*[0-9.]+px`, 'g'),
           new RegExp(String.raw`(?<![-\w])(?:top|right|bottom|left):\s*-?[0-9.]+px`, 'g')]
        : [new RegExp(String.raw`\b${jsxProp}:\s*${num}\b`, 'g'),
           new RegExp(String.raw`\b${jsxProp}:\s*'[^']*[0-9.]+px[^']*'`, 'g')]
      for (const pattern of patterns) {
        for (const match of text.matchAll(pattern)) {
          offenders.push(`${file.replace(`${SRC}/`, '')}: ${match[0]}`)
        }
      }
    }
    expect(offenders).toEqual([])
  })
  it('không dùng border-left/right để tách dòng', () => {
    const offenders: string[] = []
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      if (/border-[Ll]eft\b|border-[Rr]ight\b|borderLeft:|borderRight:/.test(text)) offenders.push(file.replace(`${SRC}/`, ''))
    }
    expect(offenders).toEqual([])
  })
  it('mỗi màn hình nhiều nhất một nút primary', () => {
    // Đếm ActionButton variant="primary" theo file trong screens/ (trừ dev kit).
    const offenders: string[] = []
    const screens = join(SRC, 'screens')
    for (const file of files) {
      if (!file.startsWith(screens) || file.includes('/dev/')) continue
      const text = readFileSync(file, 'utf8')
      // Đếm JSX <ActionButton ... variant="primary" ...> — kit Button không tính.
      const count = (text.match(/<ActionButton[^>]*variant="primary"/gs) ?? []).length
      if (count > 1) offenders.push(`${file.replace(`${SRC}/`, '')}: ${count} nút primary`)
    }
    expect(offenders).toEqual([])
  })
  it('prefers-reduced-motion: mọi hoạ hình đứng yên (cung logo, orb, chấm đỏ)', () => {
    // V0 acceptance: reduced-motion thì vòng cung của logo đầu trang, AgentOrb
    // (kit ai-breathe) và chấm đỏ nhấp đều phải đứng yên. Hoạ hình sống trong
    // CSS nên guard đọc stylesheet — base.css phải có luật phủ toàn cục.
    const base = readFileSync(join(SRC, 'styles/base.css'), 'utf8')
    const block = base.match(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{[\s\S]*?\}\s*\}/)
    expect(block, 'base.css thiếu luật @media (prefers-reduced-motion: reduce) phủ toàn cục').toBeTruthy()
    expect(block![0]).toContain('animation')
    expect(block![0]).toContain('!important')
    // Các hoạ hình riêng của app phải được phủ bởi luật đó (không né bằng
    // animation inline không qua lớp này — kiểm nhanh không có `!important`
    // animation nào ngoài block reduced-motion).
    const anims = [...base.matchAll(/animation(?:-[\w]+)?:\s*[^;]*!/g)].filter((m) => !block![0].includes(m[0]))
    expect(anims).toEqual([])
  })
})

describe('nhịp khung Cài đặt theo artboard (trị số ghim chặt)', () => {
  const settingsCss = readFileSync(join(SRC, 'screens/settings/settings.css'), 'utf8')
  const toolsCss = readFileSync(join(SRC, 'screens/settings/tools/tools.css'), 'utf8')
  const componentsCss = readFileSync(join(SRC, 'styles/components.css'), 'utf8')

  it('nhãn nhóm sidebar 11.5px — artboard Settings .grp, không phải 11px', () => {
    const grp = settingsCss.match(/\.vv-set__grp\s*\{[^}]*\}/)
    expect(grp).toBeTruthy()
    expect(grp![0]).toContain('font-size: 11.5px')
  })

  it('header không co khi cột dưới thiếu chỗ (artboard top cao cố định)', () => {
    const header = componentsCss.match(/\.vv-header\s*\{[^}]*\}/)
    expect(header).toBeTruthy()
    expect(header![0]).toContain('flex-shrink: 0')
  })

  it('mục Công cụ › câu lỗi hàng cam (.vv-tools__hint.is-err)', () => {
    expect(toolsCss).toMatch(/\.vv-tools__hint\.is-err\s*\{[^}]*\}/)
  })

  it('MSettings › Chi tiết: tên công cụ (adv-k) không bị display:none ở mobile', () => {
    const mobile = toolsCss.match(/@media\s*\(max-width:\s*767px\)\s*\{[\s\S]*/)
    expect(mobile).toBeTruthy()
    expect(mobile![0]).not.toMatch(/\.vv-tools__adv-k\s*\{[^}]*display:\s*none[^}]*\}/)
  })

  it('Báo lỗi › lỗi chưa xử lý dùng token cam lỗi, không phải vàng warn', () => {
    const logsCss = readFileSync(join(SRC, 'screens/settings/logs/logs.css'), 'utf8')
    expect(logsCss).toMatch(/\.vv-logs__err-st\.is-err\s*\{[^}]*var\(--color-err\)[^}]*\}/)
    expect(logsCss).not.toMatch(/\.vv-logs__err-st\.is-warn/)
  })

  it('MSettings › Lưu trữ: lời dẫn sheet Đổi thư mục thụt vào lề 16', () => {
    // .vv-sto__hintLine là con trực tiếp của .m-sheet-body không padding — nếu
    // mất margin-inline 16 thì nó giãn x0/390 thay vì x16/358 như artboard.
    const storageCss = readFileSync(join(SRC, 'screens/settings/storage/storage.css'), 'utf8')
    const rule = storageCss.match(/\.vv-sto__hintLine\s*\{[^}]*\}/)
    expect(rule).toBeTruthy()
    expect(rule![0]).toContain('margin-inline: var(--vv-s-16)')
    expect(rule![0]).not.toContain('padding')
  })
})
