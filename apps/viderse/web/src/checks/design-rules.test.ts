// @vitest-environment node
// Kiểm tra luật thiết kế (08-dev-prompt V12): không hex cứng / bán kính cứng
// ngoài tệp token; không border-left/right (đường kẻ tách chỉ là line ngang
// hay viền đều bốn phía); mỗi route chỉ một ActionButton primary (kiểm ở
// dev-kit không tính vì đó là trưng bày).
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const SRC = new URL('..', import.meta.url).pathname.replace(/\/$/, '')

// MIT donor internals are not Viderse chrome. New V13 surfaces stay fully checked.
// Exact legacy UI exceptions only; none of these NLE panels is mounted by V13.
const LEGACY_EDITOR_INTERNALS: Record<string, string> = {
  'editor/ui/controls/index.tsx': 'Legacy NLE controls; content-color swatch defaults, not V13 controls.',
  'editor/ui/inspector/canvas.tsx': 'Legacy canvas settings; V13 has a read-only fixed-frame inspector.',
  'editor/ui/inspector/text.tsx': 'Legacy rich-text inspector; V13 caption inspector is separate.',
  'editor/ui/inspector/effect-preview.ts': 'Synthetic effect sample image, not V13 surface colors.',
  'editor/ui/library/library-actions.ts': 'Timeline media constructors: literal colors describe rendered text, not UI.',
  'editor/ui/preview/use-preview-composition.ts': 'Composition pixel background fallback belongs to rendering, not app chrome.',
  'editor/ui/timeline/timeline.css': 'Retained upstream NLE engine stylesheet; V13 timeline uses StyleX track tokens.',
}
const vendoredEditor = (path: string) => {
  const relative = path.replace(`${SRC}/`, '')
  return relative.startsWith('editor/engine/') || relative.startsWith('editor/vendor/') || Object.hasOwn(LEGACY_EDITOR_INTERNALS, relative)
}

const files: string[] = []
const walk = (dir: string) => {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry)
    const stat = statSync(path)
    if (stat.isDirectory()) walk(path)
    else if (/\.(ts|tsx|css)$/.test(entry) && !entry.endsWith('.test.ts') && !entry.endsWith('.test.tsx') && !vendoredEditor(path)) files.push(path)
  }
}
walk(SRC)

// Tệp được phép chứa hex cứng: tệp token, bộ icon SVG (đồ hoạ), brand (kể cả
// styles/brand.css sinh sẵn từ src/brand.ts — cùng một bảng token).
const TOKEN_FILES = /tokens\.css|tokens\.stylex\.ts$|viderse\.css|brand\.(ts|css)$|LensLogo\.tsx$|\/icons\//

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
  it('fg-5 chỉ cho dấu phân cách, không làm màu chữ (tokens.stylex: ≥4.5:1 bắt đầu từ fg-4)', () => {
    const offenders: string[] = []
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      for (const match of text.matchAll(/(?<![\w-])(\w+):\s*\{[^}]*?(?<![\w-])color:\s*(?:color\[['"]--color-fg-5['"]\]|['"]?var\(--color-fg-5\))/g)) {
        if (/^(sep|separator|slash|divider)$/i.test(match[1] ?? '')) continue
        offenders.push(`${file.replace(`${SRC}/`, '')}: ${match[1]}`)
      }
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
  it('className đứng trước {...stylex.props()} sẽ bị ghi đè — ghép vào chuỗi className', () => {
    const offenders: string[] = []
    for (const file of files) {
      if (!file.endsWith('.tsx')) continue
      const text = readFileSync(file, 'utf8')
      if (/className=(?:"[^"]*"|\{[^{}]*\})\s+\{\.\.\.stylex\.props\(/.test(text)) offenders.push(file.replace(`${SRC}/`, ''))
    }
    expect(offenders).toEqual([])
  })
  it('mọi var(--token) không có giá trị dự phòng đều được khai báo', () => {
    // Token chưa khai báo làm cả khai báo CSS bị bỏ qua mà không báo lỗi
    // (hover mất nền, min-height mất tác dụng). Token khai báo ở CSS của app,
    // CSS của kit, hoặc gán trong style JSX ('--x': …).
    const kitFiles: string[] = []
    const collect = (dir: string) => {
      for (const entry of readdirSync(dir)) {
        const path = join(dir, entry)
        if (statSync(path).isDirectory()) collect(path)
        else if (/\.(ts|tsx|css)$/.test(entry)) kitFiles.push(path)
      }
    }
    collect(join(SRC, '../../../../platform/ui/src'))
    const declared = new Set<string>()
    for (const file of [...files, ...kitFiles]) {
      const text = readFileSync(file, 'utf8')
      for (const m of text.matchAll(/(--[A-Za-z0-9_-]+)\s*:/g)) declared.add(m[1])
      for (const m of text.matchAll(/['"](--[A-Za-z0-9_-]+)['"]\s*[:,\]]/g)) declared.add(m[1])
    }
    const offenders: string[] = []
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      for (const m of text.matchAll(/var\((--[A-Za-z0-9_-]+)\s*\)/g)) {
        if (!declared.has(m[1])) offenders.push(`${file.replace(`${SRC}/`, '')}: ${m[1]}`)
      }
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
    // CSS nên guard đọc stylesheet toàn cục duy nhất — viderse.css phải có luật
    // phủ toàn cục.
    const base = readFileSync(join(SRC, 'styles/viderse.css'), 'utf8')
    const block = base.match(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{[\s\S]*?\}\s*\}/)
    expect(block, 'styles/viderse.css thiếu luật @media (prefers-reduced-motion: reduce) phủ toàn cục').toBeTruthy()
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
  const settingsSource = readFileSync(join(SRC, 'screens/settings/settings.stylex.ts'), 'utf8')
  const settingsMetrics = readFileSync(join(SRC, 'screens/settings/settingsVars.stylex.ts'), 'utf8')
  const headerSource = readFileSync(join(SRC, 'shell/AppHeader.tsx'), 'utf8')

  it('nhãn nhóm sidebar 11.5px — artboard Settings .grp, không phải 11px', () => {
    expect(settingsSource.match(/^  setGrp:.*$/m)?.[0]).toContain('fontSize: settingsVars.setGrpfontSize')
    expect(settingsMetrics).toContain('setGrpfontSize: "11.5px"')
  })

  it('header không co khi cột dưới thiếu chỗ (artboard top cao cố định)', () => {
    const header = headerSource.match(/^  header: \{[\s\S]*?\n  \},$/m)
    expect(header).toBeTruthy()
    expect(header![0]).toContain('flexShrink: 0')
  })

  it('mục Công cụ › câu lỗi hàng cam (.vv-tools__hint.is-err)', () => {
    expect(settingsSource.match(/^  toolsHintIsErr:.*$/m)?.[0]).toContain('color: "var(--vv-warn-ink)"')
  })

  it('MSettings › Chi tiết: tên công cụ (adv-k) không bị display:none ở mobile', () => {
    const rule = settingsSource.match(/^  toolsAdvK:.*$/m)?.[0]
    expect(rule).toBeTruthy()
    expect(rule).not.toMatch(/display:.*"none"/)
    expect(rule).toContain('color:')
  })

  it('Báo lỗi › lỗi chưa xử lý dùng token cam lỗi, không phải vàng warn', () => {
    expect(settingsSource.match(/^  logsErrStIsErr:.*$/m)?.[0]).toContain('color: color["--color-err"]')
    expect(settingsSource).not.toMatch(/^  logsErrStIsWarn:/m)
  })

  it('MSettings › Lưu trữ: lời dẫn sheet Đổi thư mục thụt vào lề 16', () => {
    // .vv-sto__hintLine là con trực tiếp của .m-sheet-body không padding — nếu
    // mất margin-inline 16 thì nó giãn x0/390 thay vì x16/358 như artboard.
    const rule = settingsSource.match(/^  stoHintLine:.*$/m)?.[0]
    expect(rule).toBeTruthy()
    expect(rule).toContain('marginInline: "var(--vv-s-16)"')
    expect(rule).not.toContain('padding')
  })
})


describe('mốc điện thoại', () => {
  it('chỉ một mốc: mọi media query bề ngang trong dải 600–800px là 767px (dưới 768px)', () => {
    // Khung điện thoại bắt đầu ở 768px (#16). Một mốc phụ như 700px làm 700–767px
    // nhận bố cục desktop ở chỗ này và điện thoại ở chỗ khác.
    const offenders: string[] = []
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      for (const match of text.matchAll(/(?:max|min)-width:\s*(\d+)px/g)) {
        const width = Number(match[1])
        if (width >= 600 && width <= 800 && width !== 767 && width !== 768) offenders.push(`${file.replace(`${SRC}/`, '')}: ${match[0]}`)
      }
    }
    expect([...new Set(offenders)]).toEqual([])
  })
})
