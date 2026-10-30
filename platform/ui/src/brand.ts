// Lớp brand của @noverse/ui: defineBrand({ primary, … }) → bộ token hoàn chỉnh.
// Lớp token (xem README §Brand): core brand colors → thang tông OKLCH (--scale-*)
// → vai trò ngữ nghĩa (--color-accent, --tint-*, --line-focus, …) → token component.
// Component chỉ đọc token ngữ nghĩa/component — không bao giờ đọc core hay --scale-*.
// File này không chứa literal màu: mặc định lấy từ tokenDefaults.ts (sinh từ tokens.stylex.ts).
import {
  BrandInputError, contrastRatio, darken, ensureAA, lighten, mapColors, mix,
  parseHex, readableOn, rotate, srgbToOklch, toHex, tonalScale, TONE_STOPS, withChroma, withTint,
  type Rgb,
} from './brandColor'
import { FIXED_TOKENS, ROLE_LITERALS, TOKEN_DEFAULTS, TOKEN_GROUPS } from './tokenDefaults'

export { BrandInputError } from './brandColor'
export type { Oklch, Rgb } from './brandColor'
export { tonalScale }

export type RoleName = keyof typeof ROLE_LITERALS | 'link'

export interface BrandCore {
  /** Tên hiển thị (data-brand, chú thích CSS). Mặc định = mã primary. */
  name?: string
  /** Màu thương hiệu chính — bắt buộc, '#RGB'/'#RRGGBB'. */
  primary: string
  /** Accent ngữ nghĩa; mặc định = primary (tự sáng hoá tới WCAG AA 4.5:1 trên nền). */
  accent?: string
  /** Hue sắc trung tính: nhuộm nền/chữ xám theo sắc brand. Bỏ trống = thang xám Noverse. */
  neutralTint?: string
  /** Màu trạng thái; bỏ trống từng cái → dùng sắc mặc định của kit. */
  status?: { success?: string; warning?: string; danger?: string; info?: string }
  /** Thang “nguy hiểm” của kit (nút dsoft/danger, Banner lỗi, ô lỗi) theo đúng
   *  brand. Bỏ trống → kit giữ thang đỏ Noverse (mọi token --kit-err* hiện tại).
   *  soft/border là màu gốc; alpha lấy theo độ mờ của từng token. */
  danger?: {
    soft: string
    border?: string
    ink?: string
    solid?: string
    solidInk?: string
    /** chữ của nút warn-ghost (nền trong, viền cam) — mặc định = `ink` */
    ghostInk?: string
  }
  /** Ghi đè một vai trò ngữ nghĩa (ví dụ roles: { accentDeep: '#…', link: '#…' }). */
  roles?: Partial<Record<RoleName, string>>
  /** Ghi đè trực tiếp token đầu ra (cứu chữa cuối cùng — ưu tiên roles hơn). */
  overrides?: Record<`--${string}`, string>
}

export interface Brand {
  name: string
  core: Required<Pick<BrandCore, 'primary'>> & Omit<BrandCore, 'primary'>
  /** Giá trị từng vai trò sau khi giải (pin → derive). */
  roles: Record<RoleName, string>
  /** Map phẳng token CSS → giá trị: đủ khoá của theme mặc định + thang --scale-*. */
  vars: Record<`--${string}`, string>
  /** vars chia theo nhóm defineVars — truyền thẳng vào stylex.createTheme(nhóm, …). */
  stylex: Record<string, Record<string, string>>
}

// --- Đầu vào ------------------------------------------------------------------

function reqColor(value: string | undefined, field: string): Rgb | undefined {
  if (value == null) return undefined
  return parseHex(value, field)
}

const KEY = /^--[\w-]+$/

/** Kiểm tra hình dạng input; ném BrandInputError kèm tên trường. */
function validate(input: BrandCore): void {
  if (!input || typeof input !== 'object') throw new BrandInputError('core', 'defineBrand cần một object')
  if (!input.primary) throw new BrandInputError('primary', 'bắt buộc — màu thương hiệu chính')
  reqColor(input.primary, 'primary')
  reqColor(input.accent, 'accent')
  reqColor(input.neutralTint, 'neutralTint')
  if (input.status != null) {
    for (const [k, v] of Object.entries(input.status)) reqColor(v as string, `status.${k}`)
  }
  if (input.danger != null) {
    reqColor(input.danger.soft, 'danger.soft')
    reqColor(input.danger.border, 'danger.border')
    reqColor(input.danger.ink, 'danger.ink')
    reqColor(input.danger.solid, 'danger.solid')
    reqColor(input.danger.solidInk, 'danger.solidInk')
    reqColor(input.danger.ghostInk, 'danger.ghostInk')
  }
  if (input.roles != null) {
    for (const [k, v] of Object.entries(input.roles)) {
      if (k !== 'link' && !(k in ROLE_LITERALS)) throw new BrandInputError(`roles.${k}`, 'vai trò không tồn tại')
      if (k === 'link' && /^var\(/.test(v)) continue
      reqColor(v, `roles.${k}`)
    }
  }
  if (input.overrides != null) {
    for (const k of Object.keys(input.overrides)) {
      if (!KEY.test(k)) throw new BrandInputError(`overrides.${k}`, 'khoá override phải là biến CSS (--…)')
    }
  }
}

// --- Giải vai trò --------------------------------------------------------------

/** Vai trò ngữ nghĩa đã giải + core primary (dùng làm gốc derive). */
export type Resolved = Record<RoleName, Rgb> & { primary: Rgb }

type Recipe = (r: Resolved, c: Const) => Rgb
interface Const { bg: Rgb; surface: Rgb; fg: Rgb; white: Rgb; black: Rgb; gone: Rgb }

/** Công thức derive cho brand mới — Noverse ghim bằng ROLE_LITERALS nên đường này chỉ chạy cho brand khác. */
const RECIPES: Record<RoleName, Recipe> = {
  accent: (r, c) => ensureAA(r.primary, c.bg),
  accentSoft: (r, c) => mix(r.accent, c.fg, 0.35),
  accentPale: (r, c) => mix(r.accent, c.white, 0.3),
  accentPale2: (r, c) => mix(r.accent, c.white, 0.55),
  accentPale3: (r, c) => mix(r.accent, c.white, 0.45),
  accentPale4: (r, c) => mix(r.accent, c.white, 0.5),
  accentBar: (r, c) => mix(r.accent, c.white, 0.45),
  accentDeep: (r) => darken(r.primary, 0.18),
  accentPink: (r) => rotate(darken(r.primary, 0.1), -55),
  indigo: (r) => darken(r.infoStrong, 0.12),
  glow: (r) => rotate(r.accentDeep, 18),
  ok: (r, c) => ensureAA(r.ok, c.bg),
  okText: (r, c) => mix(r.ok, c.white, 0.3),
  okText2: (r, c) => mix(r.ok, c.white, 0.4),
  okText3: (r, c) => mix(r.ok, c.white, 0.35),
  okMint: (r, c) => mix(r.ok, c.white, 0.25),
  warn: (r, c) => ensureAA(r.warn, c.bg),
  warnText: (r, c) => mix(r.warn, c.white, 0.25),
  warnBody: (r, c) => mix(r.warn, c.fg, 0.2),
  warnMark: (r, c) => mix(r.warn, c.white, 0.35),
  warnCream: (r, c) => mix(r.warn, c.white, 0.4),
  warnPickBg: (r, c) => mix(r.warn, c.black, 0.88),
  warnBadgeBg: (r, c) => mix(r.warn, c.black, 0.86),
  warnDark: (r, c) => mix(r.warn, c.black, 0.92),
  err: (r, c) => ensureAA(r.err, c.bg),
  errSoft: (r, c) => mix(r.err, c.white, 0.2),
  errText: (r, c) => mix(r.err, c.white, 0.35),
  errText2: (r, c) => mix(r.err, c.white, 0.4),
  errBright: (r) => mix(r.err, r.errHot, 0.5),
  errStrong: (r) => darken(r.err, 0.1),
  errHot: (r) => rotate(darken(r.err, 0.15), 8),
  danger: (r) => darken(withChroma(r.errHot, srgbToOklch(r.errHot).c * 1.15), 0.18),
  dangerHi: (r) => lighten(r.danger, 0.08),
  info: (r, c) => ensureAA(r.info, c.bg),
  infoSoft: (r, c) => mix(r.info, c.white, 0.15),
  infoBadge: (r, c) => mix(r.info, c.white, 0.12),
  infoDeep: (r) => darken(r.info, 0.1),
  infoStrong: (r) => darken(r.info, 0.25),
  user: (r) => rotate(darken(r.ok, 0.1), 10),
  userFg: (r, c) => mix(r.user, c.white, 0.45),
  userSoft: (r, c) => mix(r.user, c.white, 0.5),
  pink: (r) => rotate(mix(r.err, r.errHot, 0.4), -12),
  pinkFg: (r, c) => mix(r.pink, c.white, 0.2),
  edgeWork: (r, c) => mix(r.infoDeep, c.gone, 0.4),
  admin: (r) => darken(r.warn, 0.4),
  adminFg: (r, c) => mix(r.admin, c.white, 0.5),
  lockNote: (r, c) => mix(r.err, c.gone, 0.55),
  roundStar: (r) => darken(r.warn, 0.1),
  dotBack: (r) => mix(r.infoStrong, r.accent, 0.5),
  orbLight1: (r, c) => mix(r.info, c.white, 0.55),
  orbLight2: (r, c) => mix(r.warn, c.white, 0.55),
  orbLight3: (r, c) => mix(r.ok, c.white, 0.55),
  editDeep: (r) => darken(r.warn, 0.18),
  editFlame: (r) => rotate(darken(r.warn, 0.35), -18),
  arbBlue: (r) => mix(r.infoStrong, r.user, 0.35),
  navTile: (r, c) => mix(r.accentDeep, c.bg, 0.9),
  navA: (r, c) => mix(r.accentDeep, c.bg, 0.82),
  navB: (r, c) => mix(r.accentDeep, c.bg, 0.9),
  leadBg: (r, c) => mix(r.accentDeep, c.bg, 0.82),
  antaBg: (r, c) => mix(r.errHot, c.bg, 0.86),
  allyBg: (r, c) => mix(r.ok, c.bg, 0.85),
  dotFront: (r, c) => mix(r.accentPale2, c.white, 0.3),
  link: (r, c) => c.fg,
}

/** Sắp xếp topo thô: vai trò phụ thuộc vai trò khác qua r — chạy 3 vòng đủ cho chuỗi phụ thuộc. */
function resolveRoles(input: BrandCore, c: Const): Resolved {
  const out = {} as Resolved
  // Tầng 0 — core.
  out.primary = parseHex(input.primary, 'primary')
  const seeds: Partial<Record<RoleName, Rgb | undefined>> = {
    accent: reqColor(input.accent, 'accent'),
    ok: reqColor(input.status?.success, 'status.success'),
    warn: reqColor(input.status?.warning, 'status.warning'),
    err: reqColor(input.status?.danger, 'status.danger'),
    info: reqColor(input.status?.info, 'status.info'),
  }
  const provided = new Set<RoleName>()
  for (const k of Object.keys(ROLE_LITERALS) as (keyof typeof ROLE_LITERALS)[]) {
    const seed = seeds[k as RoleName]
    if (seed) provided.add(k)
    out[k] = seed ?? parseHex(ROLE_LITERALS[k][0])
  }
  // Vai trò có pin trong input.roles; derive phần còn lại (3 vòng cho chuỗi phụ thuộc).
  const pinned = input.roles ?? {}
  const order = Object.keys(ROLE_LITERALS) as (keyof typeof ROLE_LITERALS)[]
  for (let pass = 0; pass < 3; pass++) {
    for (const k of order) {
      const p = pinned[k]
      if (p != null) { out[k] = parseHex(p, `roles.${k}`); continue }
      if (!provided.has(k)) out[k] = RECIPES[k](out, c)
    }
  }
  // link là vai trò chuỗi (có thể là 'var(--x)') — xử lý riêng ở assignRoles.
  out.link = pinned.link != null && !/^var\(/.test(pinned.link) ? parseHex(pinned.link, 'roles.link') : c.fg
  // Bảo đảm AA cho vai trò chữ (mọi vai trò có pin Noverse đạt ≥4.5:1 trên nền).
  for (const k of order) {
    if (contrastRatio(parseHex(ROLE_LITERALS[k][0]), parseHex(TOKEN_DEFAULTS['--color-bg'])) >= 4.5) {
      out[k] = ensureAA(out[k], c.bg)
    }
  }
  return out
}

// --- Thang tông ----------------------------------------------------------------
const STOPS = TONE_STOPS

/** Thang trung tính: giữ lightness của từng nấc xám mặc định, nhuộm theo neutralTint. */
export function neutralScale(tint: Rgb): Record<(typeof STOPS)[number], string> {
  const grays = [
    TOKEN_DEFAULTS['--color-bg'], TOKEN_DEFAULTS['--color-surface-2'], TOKEN_DEFAULTS['--color-surface-5'],
    TOKEN_DEFAULTS['--color-surface-6'], TOKEN_DEFAULTS['--color-surface-7'], TOKEN_DEFAULTS['--color-fg-5'],
    TOKEN_DEFAULTS['--color-fg-4'], TOKEN_DEFAULTS['--kit-fg-icon'], TOKEN_DEFAULTS['--color-fg-3'],
    TOKEN_DEFAULTS['--color-fg-2'], TOKEN_DEFAULTS['--color-fg'],
  ]
  const out = {} as Record<(typeof STOPS)[number], string>
  STOPS.forEach((stop, i) => { out[stop] = toHex(withTint(parseHex(grays[i]), tint)) })
  return out
}

// --- defineBrand ---------------------------------------------------------------


/** Khoá token set sau khi map màu (giá trị tính từ vai trò, không thuộc literal gốc). */
const POST_TOKENS: Record<string, true> = { '--color-accent-fg': true, '--color-link': true }

/**
 * Thang “nguy hiểm” của kit theo brand: mọi --kit-err-N / --kit-errf-N lấy
 * alpha của token nhưng màu gốc là `soft`; --kit-errb-N dùng `border`; chữ và
 * nút đặc dùng `ink`/`solid`/`solidInk`. Chạy trước overrides nên overrides vẫn thắng.
 */
function applyDangerScale(vars: Record<`--${string}`, string>, d: NonNullable<BrandCore['danger']>): void {
  const soft = parseHex(d.soft, 'danger.soft')
  const border = parseHex(d.border ?? d.soft, 'danger.border')
  const ink = d.ink ?? d.soft
  const solid = d.solid ?? d.soft
  const solidRgb = parseHex(solid, 'danger.solid')
  const rgba = (c: Rgb, pct: number) => `rgba(${Math.round(c.r)},${Math.round(c.g)},${Math.round(c.b)},${pct / 100})`
  for (const key of Object.keys(vars) as `--${string}`[]) {
    const tint = /^--kit-err(?:f)?-(\d+)$/.exec(key)
    if (tint) { vars[key] = rgba(soft, Number(tint[1])); continue }
    const line = /^--kit-errb-(\d+)$/.exec(key)
    if (line) { vars[key] = rgba(border, Number(line[1])); continue }
  }
  const set = (key: `--${string}`, value: string) => { if (key in vars) vars[key] = value }
  set('--kit-err-strong', d.soft)
  set('--kit-err-soft', d.soft)
  set('--kit-err-bright', d.soft)
  set('--kit-err-text', ink)
  set('--kit-err-ghost-ink', d.ghostInk ?? ink)
  set('--kit-err-deep', solid)
  set('--kit-err-deep-hi', toHex(lighten(solidRgb, 0.08)))
  set('--kit-err-deep-fg', d.solidInk ?? '#FFFFFF')
  set('--kit-ring-err', `0 0 0 3px ${rgba(soft, 10)}`)
  set('--kit-ring-danger', `0 0 0 4px ${rgba(border, 18)}`)
  set('--tint-err', rgba(soft, 10))
}

export function defineBrand(input: BrandCore): Brand {
  validate(input)
  const c: Const = {
    bg: parseHex(TOKEN_DEFAULTS['--color-bg']),
    surface: parseHex(TOKEN_DEFAULTS['--color-surface-2']),
    fg: parseHex(TOKEN_DEFAULTS['--color-fg']),
    white: parseHex(TOKEN_DEFAULTS['--kit-white']),
    black: { r: 0, g: 0, b: 0 },
    gone: parseHex(TOKEN_DEFAULTS['--fb-gone-fg']),
  }
  const rolesRgb = resolveRoles(input, c)

  // Bảng tra literal → vai trò (theo kênh màu, vì literal có hai dạng viết).
  const lut = new Map<number, RoleName>()
  for (const role of Object.keys(ROLE_LITERALS) as (keyof typeof ROLE_LITERALS)[]) {
    for (const lit of ROLE_LITERALS[role]) {
      const rgb = lit.startsWith('#') ? parseHex(lit) : (() => { const [r, g, b] = lit.split(',').map(Number); return { r, g, b } })()
      lut.set(rgb.r * 65536 + rgb.g * 256 + rgb.b, role)
    }
  }
  const tint = reqColor(input.neutralTint, 'neutralTint')

  const vars: Record<`--${string}`, string> = {}
  for (const [key, value] of Object.entries(TOKEN_DEFAULTS)) {
    if (FIXED_TOKENS.has(key) || POST_TOKENS[key] === true) { vars[key as `--${string}`] = value; continue }
    vars[key as `--${string}`] = mapColors(value, (rgb) => {
      const role = lut.get(Math.round(rgb.r) * 65536 + Math.round(rgb.g) * 256 + Math.round(rgb.b))
      if (role) return rolesRgb[role]
      if (tint && srgbToOklch(rgb).c < 0.03) return withTint(rgb, tint)
      return null
    })
  }

  // Vai trò post: chữ trên accent (đen/trắng — cái nào tương phản hơn) và link.
  vars['--color-accent-fg'] = toHex(readableOn(rolesRgb.accent, c.black, c.white))
  vars['--color-link'] = input.roles?.link ?? 'var(--color-fg)'

  // Thang tông --scale-<nhóm>-<50..950>.
  const scales: Record<string, Rgb> = {
    primary: rolesRgb.primary, accent: rolesRgb.accent,
    ok: rolesRgb.ok, warn: rolesRgb.warn, err: rolesRgb.err, info: rolesRgb.info,
  }
  for (const [name, rgb] of Object.entries(scales)) {
    const scale = tonalScale(rgb)
    for (const stop of STOPS) vars[`--scale-${name}-${stop}`] = scale[stop]
  }
  if (tint) {
    const scale = neutralScale(tint)
    for (const stop of STOPS) vars[`--scale-neutral-${stop}`] = scale[stop]
  }

  // Thang nguy hiểm theo brand (bỏ trống → giữ nguyên thang đỏ của kit).
  if (input.danger) applyDangerScale(vars, input.danger)

  // Overrides thắng tất cả (đã validate khoá).
  for (const [k, v] of Object.entries(input.overrides ?? {})) vars[k as `--${string}`] = v

  // vars → nhóm theo defineVars (cho stylex.createTheme).
  const stylex: Record<string, Record<string, string>> = {}
  for (const [group, entries] of Object.entries(TOKEN_GROUPS)) {
    stylex[group] = {}
    for (const k of Object.keys(entries)) if (k in vars) stylex[group][k] = vars[k as `--${string}`]
  }
  stylex.scale = Object.fromEntries(Object.entries(vars).filter(([k]) => k.startsWith('--scale-')))

  const roles = {} as Record<RoleName, string>
  for (const k of Object.keys(ROLE_LITERALS) as (keyof typeof ROLE_LITERALS)[]) roles[k] = toHex(rolesRgb[k])
  roles.link = vars['--color-link']

  return { name: input.name ?? input.primary, core: { ...input }, roles, vars, stylex }
}

// --- Áp dụng -------------------------------------------------------------------

/** Gắn biến brand lên element (mặc định :root). Phạm vi lồng nhau: var() cascade tự nhiên. */
export function applyBrand(brand: Brand, el?: HTMLElement | null): void {
  const target = el ?? document.documentElement
  for (const [k, v] of Object.entries(brand.vars)) target.style.setProperty(k, v)
  target.setAttribute('data-brand', brand.name)
}

/** Gỡ toàn bộ biến brand đã apply khỏi element. */
export function removeBrand(brand: Brand, el?: HTMLElement | null): void {
  const target = el ?? document.documentElement
  for (const k of Object.keys(brand.vars)) target.style.removeProperty(k)
  target.removeAttribute('data-brand')
}

export interface BrandCssOptions {
  /** Selector nhận biến (mặc định ':root' — đổi thành '.brand-acme' để scope). */
  selector?: string
}

/** CSS thuần cho consumer không dùng StyleX: `<selector> { --token: value; … }`. */
export function brandToCss(brand: Brand, opts: BrandCssOptions = {}): string {
  const sel = opts.selector ?? ':root'
  const lines = Object.entries(brand.vars).map(([k, v]) => `  ${k}: ${v};`).join('\n')
  return `/* @noverse/ui brand "${brand.name}" — đặt sau tokens.css/style.css để ghi đè. */\n${sel} {\n${lines}\n}\n`
}
