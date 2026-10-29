// Kiểm thử lớp brand:
//  1. NOVERSE (defineBrand(noverseCore)) tái tạo tokens.stylex.ts 1:1 — kể cả thang --scale-*.
//  2. Tương phản AA ≥4.5:1 cho vai trò chữ trên nền app, trên nhiều brand mẫu.
//  3. defineBrand đầy đủ: vars ⊇ mọi token mặc định + thang; roles đủ bảng.
//  4. Component không tham chiếu core/scale: cấm '--scale-' và import tokenDefaults/brand ngoài lớp brand.
//  5. Lỗi đầu vào gõ rõ: BrandInputError kèm tên trường.
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { brandToCss, defineBrand, tonalScale } from './brand'
import { colorSpans, contrastRatio, parseHex, srgbToOklch } from './brandColor'
import { noverseCore, NOVERSE } from './noverse'
import { FIXED_TOKENS, ROLE_LITERALS, TOKEN_DEFAULTS, TOKEN_GROUPS } from './tokenDefaults'

const BG = parseHex(TOKEN_DEFAULTS['--color-bg'])

describe('defineBrand: Noverse tái tạo token 1:1', () => {
  it('mọi token của tokens.stylex.ts khớp giá trị', () => {
    const diff = Object.entries(TOKEN_DEFAULTS).filter(([k, v]) => NOVERSE.vars[k as `--${string}`] !== v)
    expect(diff, 'NOVERSE lệch token mặc định').toEqual([])
  })
  it('không sinh thêm khoá lạ ngoài --scale-*', () => {
    const extra = Object.keys(NOVERSE.vars).filter((k) => !(k in TOKEN_DEFAULTS) && !k.startsWith('--scale-'))
    expect(extra).toEqual([])
  })
  it('thang scale trong tokens.stylex.ts khớp thang sinh từ core', () => {
    const scale = tonalScale(parseHex(noverseCore.primary))
    expect(NOVERSE.vars['--scale-primary-500']).toBe(scale[500])
    expect(NOVERSE.vars['--scale-primary-500']).toBe(TOKEN_DEFAULTS['--scale-primary-500'])
  })
})

describe('defineBrand: đầy đủ', () => {
  const brand = defineBrand({ primary: '#3B82F6' })
  it('vars chứa mọi khoá token mặc định', () => {
    const missing = Object.keys(TOKEN_DEFAULTS).filter((k) => !(k in brand.vars))
    expect(missing).toEqual([])
  })
  it('đủ thang tông cho mọi màu core', () => {
    for (const fam of ['primary', 'accent', 'ok', 'warn', 'err', 'info']) {
      for (const stop of [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]) {
        expect(brand.vars[`--scale-${fam}-${stop}` as `--${string}`], `--scale-${fam}-${stop}`).toMatch(/^#[0-9A-F]{6}$/)
      }
    }
  })
  it('stylex groups khớp cấu trúc defineVars', () => {
    for (const group of Object.keys(TOKEN_GROUPS)) {
      for (const k of Object.keys(TOKEN_GROUPS[group])) {
        expect(brand.stylex[group]?.[k], `${group}/${k}`).toBe(brand.vars[k as `--${string}`])
      }
    }
  })
  it('mọi vai trò có giá trị', () => {
    for (const k of Object.keys(ROLE_LITERALS)) expect(brand.roles[k as keyof typeof brand.roles], k).toBeTruthy()
  })
  it('brandToCss phát selector + đủ biến', () => {
    const css = brandToCss(brand, { selector: '.acme' })
    expect(css).toContain('.acme {')
    expect(css).toContain('--color-accent:')
    expect(css).toContain('--scale-primary-500:')
  })
  it('nested: overrides thắng substitution', () => {
    const b = defineBrand({ primary: '#3B82F6', overrides: { '--color-fg': '#ABCDEF' } })
    expect(b.vars['--color-fg']).toBe('#ABCDEF')
  })
})

describe('defineBrand: tương phản WCAG AA (≥4.5:1 trên nền tối)', () => {
  const samples = [
    ['noverse', noverseCore],
    ['blue', { primary: '#3B82F6' }],
    ['green+dark', { primary: '#16A34A', neutralTint: '#223322' }],
    ['dim red', { primary: '#7F1D1D' }],
    ['custom roles', { primary: '#F59E0B', accent: '#FBBF24', status: { danger: '#DC2626' } }],
  ] as const
  const FG_PAIRS: [string, string][] = [
    ['--color-fg', '--color-bg'],
    ['--color-fg-2', '--color-bg'],
    ['--color-fg-3', '--color-bg'],
    ['--color-fg-4', '--color-bg'],
    ['--color-accent', '--color-bg'],
    ['--color-accent-soft', '--color-surface-3'],
    ['--color-ok', '--color-bg'],
    ['--color-warn', '--color-bg'],
    ['--color-err', '--color-bg'],
    ['--color-info', '--color-bg'],
    ['--kit-err-soft', '--color-surface-3'],
    ['--kit-info-fg', '--color-surface-3'],
    ['--kit-fg-body', '--color-surface-3'],
  ]
  for (const [name, core] of samples) {
    it(`brand ${name}: cặp chữ/nền đạt AA`, () => {
      const brand = defineBrand(core)
      const fails = FG_PAIRS.filter(([fg, bg]) => {
        const f = brand.vars[fg as `--${string}`]; const b = brand.vars[bg as `--${string}`]
        if (!f?.startsWith('#') || !b?.startsWith('#')) return false
        return contrastRatio(parseHex(f), parseHex(b)) < 4.5
      })
      expect(fails).toEqual([])
    })
  }
  it('accent-fg tương phản với accent (đen/trắng tự chọn)', () => {
    for (const [, core] of samples) {
      const brand = defineBrand(core)
      const ratio = contrastRatio(parseHex(brand.vars['--color-accent-fg']), parseHex(brand.vars['--color-accent']))
      expect(ratio).toBeGreaterThanOrEqual(4.5)
    }
  })
  it('primary tối được sáng hoá thành accent đạt AA', () => {
    const brand = defineBrand({ primary: '#1D4ED8' }) // xanh đậm: trên nền đen < 4.5
    expect(brand.vars['--color-accent']).not.toBe('#1D4ED8')
    expect(contrastRatio(parseHex(brand.vars['--color-accent']), BG)).toBeGreaterThanOrEqual(4.5)
  })
})

describe('defineBrand: lỗi đầu vào', () => {
  it('thiếu primary → BrandInputError field primary', () => {
    expect(() => defineBrand({} as never)).toThrowError(/primary/)
  })
  it('hex sai → BrandInputError kèm tên trường', () => {
    expect(() => defineBrand({ primary: 'xám-xanh' })).toThrowError(/primary.*xám-xanh|xám-xanh/)
    expect(() => defineBrand({ primary: '#FF0000', status: { success: '#GGHHII' } })).toThrowError(/status\.success/)
    expect(() => defineBrand({ primary: '#FF0000', roles: { nope: '#FF0000' } as never })).toThrowError(/roles\.nope/)
  })
  it('override khoá lạ → lỗi', () => {
    expect(() => defineBrand({ primary: '#FF0000', overrides: { accent: 'x' } as never })).toThrowError(/overrides\.accent/)
  })
})

describe('ranh giới lớp token: component không đụng core/scale', () => {
  const SRC = fileURLToPath(new URL('.', import.meta.url))
  const ALLOWED = new Set(['index.ts', 'brand.ts', 'brandColor.ts', 'noverse.ts', 'ThemeProvider.tsx', 'tokenDefaults.ts', 'tokens.stylex.ts', 'tokens.stylex.js'])
  const isBrandLayer = (f: string) => ALLOWED.has(f) || f.endsWith('.test.ts') || f.endsWith('.test.tsx')
  const files = readdirSync(SRC, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile() && /\.(tsx?|css)$/.test(e.name))
    .map((e) => relative(SRC, join(e.parentPath, e.name)).split('\\').join('/'))
  it('không tệp component nào nhắc --scale-* hay import lớp core', () => {
    const bad: string[] = []
    for (const f of files) {
      if (isBrandLayer(f)) continue
      const text = readFileSync(join(SRC, f), 'utf8')
      if (/--scale-/.test(text)) bad.push(`${f}: tham chiếu --scale-*`)
      if (/from\s+['"]\.\/tokenDefaults|from\s+['"]\.\/brand['"]|from\s+['"]\.\/brandColor|from\s+['"]\.\/noverse/.test(text)) {
        bad.push(`${f}: import lớp core/scale`)
      }
    }
    expect(bad).toEqual([])
  })
  it('mọi literal chromatic của token thuộc một vai trò hoặc FIXED', () => {
    const rgb2role = new Map<string, string>()
    for (const [role, lits] of Object.entries(ROLE_LITERALS)) {
      for (const lit of lits) {
        const rgb = lit.startsWith('#')
          ? parseHex(lit)
          : (() => { const [r, g, b] = lit.split(',').map(Number); return { r, g, b } })()
        rgb2role.set(`${rgb.r},${rgb.g},${rgb.b}`, role)
      }
    }
    const unmapped: string[] = []
    for (const [k, v] of Object.entries(TOKEN_DEFAULTS)) {
      if (FIXED_TOKENS.has(k) || k.startsWith('--scale-')) continue
      for (const s of colorSpans(v)) {
        const key = `${Math.round(s.rgb.r)},${Math.round(s.rgb.g)},${Math.round(s.rgb.b)}`
        if (!rgb2role.has(key) && srgbToOklch(s.rgb).c >= 0.03) unmapped.push(`${k}: ${s.text}`)
      }
    }
    expect(unmapped).toEqual([])
  })
})
