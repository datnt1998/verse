// Chuyển động: chặn 3 loại lỗi đã từng làm animation “đứng hình” hoặc “giật ngược”:
//   1. tên keyframes gõ sai / bị đổi sau khi tách kit.css → animation không chạy;
//   2. vòng lặp `background-position` lệch một phần chu kỳ ảnh nền → mỗi vòng nhảy lùi;
//   3. `alternate` trên animation nền → ánh sáng chạy tới rồi chạy ngược.
// Chỉ đọc mã nguồn (không cần trình duyệt) nên chạy được trong vitest.
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const SRC = fileURLToPath(new URL('..', import.meta.url))
const PKG_SRC = fileURLToPath(new URL('../../packages/ui/src', import.meta.url))
const CSS = /\.css$/
const CODE = /\.(?:ts|tsx)$/
const TEST_FILE = /\.test\.tsx?$/

/** Từ không phải tên keyframes trong cú pháp `animation` viết tắt. */
const NOT_A_NAME: Record<string, true> = {
  none: true, normal: true, reverse: true, alternate: true, 'alternate-reverse': true,
  forwards: true, backwards: true, both: true, running: true, paused: true, infinite: true,
  linear: true, ease: true, 'ease-in': true, 'ease-out': true, 'ease-in-out': true,
  'step-start': true, 'step-end': true, initial: true, inherit: true, unset: true,
  revert: true, 'revert-layer': true, auto: true,
}

type Source = { file: string; text: string }

function walk(dir: string, match: RegExp): Source[] {
  const out: Source[] = []
  for (const entry of readdirSync(dir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !match.test(entry.name) || TEST_FILE.test(entry.name)) continue
    const path = join(entry.parentPath, entry.name)
    out.push({ file: relative(SRC, path).split('\\').join('/'), text: readFileSync(path, 'utf8') })
  }
  return out
}

/** Thân của mọi `@keyframes <tên>` (khớp ngoặc, không bắt nhầm block lồng). */
function keyframesOf(css: string): Map<string, string> {
  const out = new Map<string, string>()
  const head = /@keyframes\s+([\w-]+)\s*\{/g
  for (let match = head.exec(css); match; match = head.exec(css)) {
    let depth = 1
    let i = head.lastIndex
    while (i < css.length && depth > 0) {
      if (css[i] === '{') depth++
      else if (css[i] === '}') depth--
      i++
    }
    out.set(match[1], css.slice(head.lastIndex, i - 1))
  }
  return out
}

/** Tách tên animation trong một khai báo `animation`/`animation-name` (có thể nhiều lớp). */
function animationNames(value: string): string[] {
  const names: string[] = []
  for (const layer of value.split(',')) {
    for (const token of layer.trim().split(/\s+/)) {
      const bare = token.replace(/^['"`]|['"`]$/g, '')
      if (bare === '' || NOT_A_NAME[bare] === true) continue
      if (/^[\d.+-]/.test(bare)) continue // thời lượng, số vòng, số bước
      if (bare.includes('(') || bare.includes('var(')) continue // hàm thời gian / biến
      if (!/^[\w-]+$/.test(bare)) continue
      names.push(bare)
    }
  }
  return names
}

type Reference = { name: string; file: string; source: string }

/** Mọi tên keyframes được tham chiếu: CSS, style nội tuyến trong TSX, StyleX animationName. */
function references(sources: Source[]): Reference[] {
  const out: Reference[] = []
  for (const { file, text } of sources) {
    if (CSS.test(file)) {
      const decl = /animation(?:-name)?\s*:\s*([^;}]+)/g
      for (let m = decl.exec(text); m; m = decl.exec(text)) {
        for (const name of animationNames(m[1])) out.push({ name, file, source: m[0].slice(0, 80) })
      }
      continue
    }
    // `animation: 'ai-run 4.5s linear infinite'`, `animation: \`…\``, `animationName: 'ai-breathe'`
    const inline = /animation(?:Name)?\s*:\s*(['"`])([^'"`]+)\1/g
    for (let m = inline.exec(text); m; m = inline.exec(text)) {
      for (const name of animationNames(m[2])) out.push({ name, file, source: m[0].slice(0, 80) })
    }
  }
  return out
}

/** Khai báo của từng selector (bỏ qua at-rule; quy tắc trong @media vẫn được lấy). */
function declarationsOf(css: string): Map<string, string> {
  const out = new Map<string, string>()
  const rule = /([^{}]+)\{([^{}]*)\}/g
  for (let m = rule.exec(css); m; m = rule.exec(css)) {
    const selector = m[1].slice(m[1].lastIndexOf('}') + 1).trim()
    if (selector === '' || selector.startsWith('@')) continue
    for (const one of selector.split(',')) {
      const key = one.trim()
      if (key === '') continue
      out.set(key, `${out.get(key) ?? ''};${m[2]}`)
    }
  }
  return out
}

/** Cặp `from`/`to` của background-position trong một @keyframes (đơn vị %). */
function backgroundTravel(body: string): { from: number; to: number } | null {
  const positions = [...body.matchAll(/background-position\s*:\s*(-?[\d.]+)%\s+-?[\d.]+%?/g)].map((m) => Number(m[1]))
  if (positions.length < 2) {
    if (positions.length === 1) return { from: 0, to: positions[0] }
    return null
  }
  return { from: positions[0], to: positions[1] }
}

// Gói @noverse/ui giữ kit.css + tokens.stylex.ts — quét cùng app để keyframes/token còn đầy đủ.
const cssSources = [...walk(SRC, CSS), ...walk(PKG_SRC, CSS)]
const codeSources = [...walk(SRC, CODE), ...walk(PKG_SRC, CODE)]
const allSources = [...cssSources, ...codeSources]
// TSX cũng có thể nhúng `@keyframes` trong thẻ <style> (vd QuickMode) — tính cả những tệp đó.
const keyframes = new Map<string, string>()
for (const { text } of allSources) for (const [name, body] of keyframesOf(text.replace(/\/\*[\s\S]*?\*\//g, ''))) keyframes.set(name, body)

describe('chuyển động: keyframes được tham chiếu phải tồn tại', () => {
  it('mọi `animation`/`animationName` trong CSS và TSX đều có @keyframes cùng tên', () => {
    const missing = references(allSources)
      .filter(({ name }) => !keyframes.has(name))
      .map(({ name, file, source }) => `${file}: "${name}" (${source})`)
    expect([...new Set(missing)].sort(), 'Thiếu @keyframes — animation sẽ không chạy').toEqual([])
  })

  it('trạng thái đang chạy của Bàn viết dùng đúng bộ keyframes trong tokens.css/kit.css', () => {
    // Bảng chốt 03-tokens.md › Chuyển động + 14-jobs-dock.md: viền ô chat, orb thở, chữ chạy,
    // ∞, spinner, caret, quầng dock. Đổi tên keyframes phải sửa cả bảng này (và mọi chỗ dùng).
    const running = [
      'ai-run', 'ai-breathe', 'ai-shimmer', 'ai-infinity', 'ai-spin', 'ai-blink',
      'ai-breathe-dock', 'ai-aura', 'jd-flow', 'kit-skel',
    ]
    expect(running.filter((name) => !keyframes.has(name))).toEqual([])
  })
})

describe('chuyển động: góc chạy của viền ô chat', () => {
  // `var()` trong một design token được thay MỘT LẦN tại :root (nơi StyleX phát biến), nên
  // `conic-gradient(from var(--run-angle), …)` nằm trong token sẽ đứng hình dù animation chạy.
  // Góc phải khai báo trên chính phần tử có animation — đúng như RunRing đang làm.
  const tokenFile = codeSources.find(({ file }) => file === '../packages/ui/src/tokens.stylex.ts')

  it('không token nào ở :root tham chiếu --run-angle', () => {
    expect(tokenFile, 'không đọc được styles/tokens.stylex.ts').toBeTruthy()
    const offending = [...tokenFile!.text.matchAll(/'(--[\w-]+)'\s*:\s*'([^']*var\(--run-angle\)[^']*)'/g)]
      .map((m) => m[1])
    expect(offending, 'Gradient phụ thuộc --run-angle phải khai báo trên phần tử, không trong defineVars').toEqual([])
  })

  it('RunRing khai báo `conic-gradient(from var(--run-angle), …)` trên phần tử', () => {
    const composer = codeSources.find(({ file }) => file === 'screens/workspace/Composer.tsx')
    expect(composer, 'không đọc được screens/workspace/Composer.tsx').toBeTruthy()
    expect(composer!.text).toContain('conic-gradient(from var(--run-angle), var(--ring-arc))')
  })

  it('--run-angle được đăng ký kiểu <angle> để nội suy được', () => {
    const tokens = cssSources.find(({ file }) => file === 'styles/tokens.css')
    expect(tokens, 'không đọc được styles/tokens.css').toBeTruthy()
    expect(tokens!.text).toMatch(/@property\s+--run-angle\s*\{\s*syntax:\s*"<angle>"/)
    expect(tokens!.text).toMatch(/@keyframes\s+ai-run\s*\{\s*from\s*\{\s*--run-angle:\s*0deg\s*\}/)
  })
})

describe('chuyển động: vòng lặp nền phải liền mạch', () => {
  /** Ảnh nền rộng S% khung: mỗi 100% background-position đi được (S−100)% khung; chu kỳ ảnh = S% khung. */
  const loops = (): { selector: string; animation: string; travel: number; period: number; alternate: boolean }[] => {
    const found: { selector: string; animation: string; travel: number; period: number; alternate: boolean }[] = []
    for (const { text } of cssSources) {
      for (const [selector, body] of declarationsOf(text.replace(/\/\*[\s\S]*?\*\//g, ''))) {
        const size = /background-size\s*:\s*([\d.]+)%\s+100%/.exec(body)
        if (!size) continue
        const anim = /animation\s*:\s*([^;}]+)/.exec(body)
        if (!anim) continue
        const width = Number(size[1])
        for (const name of animationNames(anim[1])) {
          const keyframesBody = keyframes.get(name)
          if (keyframesBody === undefined) continue
          const travel = backgroundTravel(keyframesBody)
          if (!travel) continue
          found.push({
            selector,
            animation: name,
            travel: ((width - 100) / 100) * Math.abs(travel.from - travel.to),
            period: width,
            alternate: anim[1].includes('alternate'),
          })
        }
      }
    }
    return found
  }

  it('`background-position` đi đúng bội số chu kỳ ảnh nền (không nhảy lùi cuối vòng)', () => {
    const jumped = loops()
      .filter(({ travel, period }) => Math.abs(travel / period - Math.round(travel / period)) > 1e-6)
      .map(({ selector, animation, travel, period }) => `${selector}: ${animation} đi ${travel.toFixed(2)} chu kỳ ảnh/${period.toFixed(2)} khung`)
    expect(jumped, 'Lệch chu kỳ → ánh sáng nhảy ngược mỗi vòng; đặt background-size sao cho quãng đi = bội số chiều rộng ảnh').toEqual([])
  })

  it('không dùng `alternate` cho animation nền (ánh sáng chỉ đi một chiều)', () => {
    const reversed = loops().filter(({ alternate }) => alternate).map(({ selector, animation }) => `${selector}: ${animation}`)
    expect(reversed).toEqual([])
  })
})
