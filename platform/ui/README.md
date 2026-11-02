# @noverse/ui

Noverse UI kit — design tokens, primitives, icons, motion presets and a11y
helpers extracted from the Noverse Studio web app. The base is dark; a project
gets its own identity by defining a **brand** (core colours → tonal scales →
semantic roles), never by switching to a light theme.

## Install

```sh
npm install @noverse/ui
# or from a packed tarball / workspace:
npm install file:../../platform/ui
```

Peer dependencies: `react >= 18`, `react-dom >= 18`, `@stylexjs/stylex >= 0.10`.
Runtime dependency: `qrcode-generator` (QrCode).

## CSS — pick one

The package ships compiled CSS; **no StyleX build plugin is required** in the
consumer:

```js
import '@noverse/ui/tokens.css'  // :root design tokens (535 CSS vars / 25 groups)
import '@noverse/ui/kit.css'    // component classes + keyframes (kit-*, m-*)
import '@noverse/ui/style.css'  // compiled StyleX output (stylex.create styles)
```

If your bundler already runs `@stylexjs` over dependencies, you can skip
`style.css` and let the plugin compile the imports inside `dist/index.js`
instead — but importing `style.css` is the supported no-plugin path.

## Tokens

Three views of the same source of truth (`src/tokens.stylex.ts`), checked by a
parity test (`tokenParity.test.ts`):

- `@noverse/ui/tokens.css` — `:root { --color-bg: … }` for plain CSS.
- `@noverse/ui/tokens.json` — `{ groups, tokens }` data export.
- `@noverse/ui/tokens.stylex` — the StyleX `defineVars` objects themselves:

```ts
import { color, line, kit } from '@noverse/ui/tokens.stylex'
// use inside stylex.create / inline style references in a StyleX app
```

All token names start with `--` so `color['--color-bg']` in StyleX and
`var(--color-bg)` in CSS resolve to the same value.

## Brand — nhận diện thương hiệu trong 5 phút

Nền vẫn tối. Cái đổi là **sắc thương hiệu**: một màu chính được triển khai
thành thang tông rồi thành vai trò ngữ nghĩa, nên mọi component đổi theo mà
không phải sửa từng chỗ.

Bốn lớp token, chỉ lớp cuối được component đọc:

| Lớp | Nội dung | Ai đọc |
| --- | --- | --- |
| core | `primary`, `accent`, `neutralTint`, `status.*` (đầu vào của bạn) | `defineBrand` |
| scale | `--scale-primary-50…950`, `--scale-accent-*`, `--scale-neutral-*`, `--scale-ok/warn/err/info-*` (OKLCH) | chỉ code tự xây variant |
| semantic | `--color-accent`, `--color-accent-fg`, `--color-sel`, `--color-link`, `--tint-*`, `--line-focus`, `--color-ok/warn/err/info`, … | component |
| component | `--kit-*`, `--brain-*`, `--cmd-*`, `--fb-*`, … | component |

Component **không** đọc core hay `--scale-*` (có test chặn: `brand.test.ts`).

### 1. Định nghĩa brand

```ts
import { defineBrand } from '@noverse/ui'

const acme = defineBrand({
  name: 'acme',
  primary: '#7C5CFF',            // bắt buộc
  // accent: '#FFB86B',          // mặc định = primary, tự sáng tới WCAG AA 4.5:1
  // neutralTint: '#2A2A3A',     // nhuộm sắc xám theo brand (bỏ trống = xám Noverse)
  status: { success: '#34D399', warning: '#FBBF24', danger: '#F87171', info: '#60A5FA' },
  // roles: { accentDeep: '#5B3BD6', link: '#A78BFA' },  // ghim tay một vai trò
  // overrides: { '--kit-switch-on': '#7C5CFF' },        // cứu chữa cuối cùng
})
```

`brand.vars` là map đầy đủ `{ '--token': value }` (đủ khoá của theme mặc định
+ `--scale-*`), `brand.roles` là các vai trò đã giải.

### 2. Gắn vào app

```ts
import { applyBrand } from '@noverse/ui'
applyBrand(acme)          // lên <html>
```

Hoặc lồng theo phạm vi (brand con thắng brand cha, var() cascade tự nhiên):

```tsx
import { ThemeProvider } from '@noverse/ui'
<ThemeProvider brand={acme}><App /></ThemeProvider>
```

`removeBrand(brand, el)` gỡ lại.

### 3. CSS cho consumer không dùng StyleX

```sh
npx noverse-ui brand --primary '#7C5CFF' --neutral-tint '#2A2A3A' > brand.css
npx noverse-ui brand --primary '#16A34A' --selector '.brand-forest' --out brand-forest.css
```

Đặt `brand.css` **sau** `tokens.css`/`style.css`. Trong code cũng có
`brandToCss(brand, { selector })`.

### 4. StyleX consumers

`brand.stylex` chia sẵn theo từng nhóm `defineVars`:

```ts
import * as stylex from '@stylexjs/stylex'
import { color, kit } from '@noverse/ui/tokens.stylex'
const theme = stylex.createTheme(color, brand.stylex.color)
// chỉ đổi một nhóm: stylex.createTheme(kit, brand.stylex.kit)
```

### Hai brand mẫu

```ts
// 1. Một dự án "biển": xanh dương, xám hơi xanh, trạng thái tươi
const ocean = defineBrand({ name: 'ocean', primary: '#2563EB', neutralTint: '#1D2433' })
applyBrand(ocean)

// 2. Một dự án "rừng": xanh lá, accent vàng, giữ nguyên trạng thái mặc định
const forest = defineBrand({
  name: 'forest',
  primary: '#16A34A',
  accent: '#FACC15',
  neutralTint: '#243024',
})
applyBrand(forest)
```

Thử nhanh trong app: `#/dev/kit?section=brand` — chọn màu, xem component và
CSS sinh ra ngay.

### Lỗi đầu vào

`defineBrand` kiểm tra trước mọi phép tính và ném `BrandInputError` (có
`field`): hex sai (`primary`, `status.success`, `roles.accent`…), vai trò
không tồn tại, khoá override không phải `--…`, thiếu `primary`.

### Brand mặc định của kit

`NOVERSE = defineBrand(noverseCore)` tái tạo **đúng** token hiện hữu
(`brand.test.ts` so từng khoá với `tokens.stylex.ts`); app không gọi
`applyBrand` nên không có pixel nào đổi.

## Theme

```ts
import { applyTheme, themes, themeNames } from '@noverse/ui'
applyTheme('dark') // only 'dark' exists; it is the default (removes data-theme)
```

New themes register in `themes` as `{ label, vars }` where `vars` is a full
`{ '--token': value }` map; `applyTheme(name)` sets `data-theme` on `<html>`
and the `[data-theme="name"]` CSS layer overrides the defaults.

## Labels (i18n)

Every string the kit renders comes from `getKitLabels()` — the package default
is **English**, with no hard-coded Vietnamese. Host apps set their locale once
at startup (shallow merge per group, read at render time — no provider):

```ts
import { setKitLabels } from '@noverse/ui'
setKitLabels({
  cancel: 'Huỷ',
  sheet: { grabHandle: 'Kéo xuống để đóng', defaultLabel: 'Tuỳ chọn' },
})
```

`KitLabels` covers: `cancel/retry/close`, `dialog`, `toast`, `controls`,
`inputs`, `selects`, `confirm` (type-to-confirm & hold-to-confirm copy),
`menus`, `overlays`, `slow`, `stale`, `conflict`, `scrollTop`, `brand`, `qr`,
`orbs`, `sheet`.

## Components

```ts
import {
  Icon, Button, Kbd, Avatar,
  Chip, Badge, Card, CardTitle, TONE,
  Tabs, Segmented,
  Switch, Checkbox, Radio, RadioCard, RadioDot, radioGroupKeys,
  Field, FieldLabel, FieldError, PasswordField, TextArea,
  Spinner, Shimmer, ProgressBar, Skeleton, EmptyState,
  Dialog, Sheet, ToastHost, toast, dismissToast,
  Scroll, ConfirmDialog, confirm,
  KitSelect, RichSelect, Combobox, MultiCombobox,
  TextField, NumberStepper, Slider,
  ActionMenu, ContextMenu, Tooltip, InfoPopover,
  Banner, StaleBanner, ConflictBar, SlowLoadingNotice,
  ScrollTopButton, useScrollTop,
  QrCode, QrZoom, QrZoomButton, QrCountdown,
  ProgressRing, ThinkingOrb, AgentOrb, BrandMark, BrandLockup,
} from '@noverse/ui'
```

Notable behaviours:

- `Button` — `variant: primary|ghost|quiet|icon|icon-only|dsoft|danger|warnGhost`,
  `size: sm (30) | md (36) | compact (32, r8, 13px) | lg (44) | xl (48)`,
  `pending`/`success` states with spinner + tick, `kbd` hint, `forwardRef`.
  `aria-disabled` locks the button but keeps it focusable: disabled look,
  clicks (and form submits) do nothing — pair it with `aria-describedby` to
  the reason.
- `Banner` — `tone: info|success|warning|error|agent`; optional `icon`,
  `align: center|top`, `role: status|alert`, one trailing `action` or an
  `actions` row under the text.
- `Sheet` — bottom sheet (`.m-*` classes, drag-to-close, Esc layer). `size`:
  `lg` (default, Noverse sheet) or `md` (Viderse artboard sheet). Falls back to
  `labels.sheet.defaultLabel` for `aria-label` when neither `title` nor
  `label` is given.
  Full-height consumers set `height="100dvh"` and
  `--m-sheet-max-height: 100dvh` in `xstyle`; other sheets keep their size cap.
  Their header/content also reserves `var(--m-safe-top)` so the fullscreen
  frame does not put its title beneath the notch.
- `ConfirmDialog` / `confirm()` — typed-phrase and hold-to-confirm variants;
  `size` sets the footer button size (default `md`); uses `useAsyncAction`
  internally, all copy via `labels.confirm`.
- `Tabs` / `Segmented` — tablist or radiogroup over the same track;
  `Segmented` takes `size: sm (28) | md (30) | lg (36) | number`, `radius`,
  `columns`, `thumbStyle`, `tabs: { id, panel }` for tab semantics. Layout is
  `fit: 'equal'` (default — segments split the track, long labels ellipsize)
  or `'content'` (one-line groups size each segment to its label, padding 12
  per segment — override with `options[].style.paddingInline`).
  If the controlled value is custom or unmatched, no choice is marked selected;
  the first enabled choice remains keyboard-reachable and arrows move from focus.
- `Switch`, `Checkbox`, `Radio`, `RadioCard` — motion:controls v1 (no replay
  on re-render), roving arrow keys with `radioGroupKeys`.
  `Checkbox.size` is `16 | 18 | 22` (default `18`); the mobile 22 px box uses
  a 6 px radius. This changes the drawn mark only — size the row/button hit
  area separately.
- `ToastHost` + `toast()` — imperative toast API; mount one host per app.
- `ActionMenu` / `ContextMenu` — popover width 264 by default, override with
  `width` (272 for menus whose descriptions run long); rows show
  `label` + optional `description`/`kbd`/`meta`, `danger`, `disabled` with
  `tooltip`, nested `children` flyout. `lockedReason` locks a row while
  keeping it focusable: the reason replaces the description line and is the
  row's `aria-describedby`; selecting it does nothing and the menu stays open.
  Navigation items accept `href`, optional `target` and `rel`, and render as
  native links inside the same menu. Links participate in arrow/Home/End
  navigation alongside commands; disabled or locked destinations cannot navigate.
  Trigger alignment defaults to `end`; `align="start"` anchors a channel/account menu to the trigger's start while still clamping it inside the viewport. `separatorBefore` starts a logical item group without duplicating the danger separator.
- `Dialog`, `Tooltip`, `InfoPopover` share the escape-layer stack
  (`useEscapeLayer` / `pushEscapeLayer`); menus join it while open.
  `Dialog` restores the exact connected opener on close, including nested
  dialogs; rerendering its callback does not move focus or rebuild the stack.
- Mobile `ListRow` (`@noverse/ui`) — use the row's title/hint/trailing
  slots rather than a centred `Button` for full-width navigation. `disabled`
  blocks native activation; disabled links omit `href` and leave the tab order.
  `aria-label` and `aria-haspopup` describe row actions without replacing native
  link/button semantics.
- Mobile viewport ownership and bottom-nav recovery:
  [shell invariant](../../apps/noverse/web/src/mobile/README.md). Keep the dock
  inside the clipped layout frame; do not restore a viewport-fixed variant.

## Icons

```tsx
<Icon name="trash" size={18} variant="bulk" label="Bin" />
```

188 SVGs ship in `src/icons/`; names are typed:

```ts
import { ICON_NAMES, type IconName, hasIcon } from '@noverse/ui'
```

`Icon` renders a `currentColor` mask-image span (`role="img"` when `label` is
given, `aria-hidden` otherwise). In the built package the SVGs are inlined as
data URIs; `src/icons/*.svg` are also included in the tarball for consumers
that want the raw assets.

## Hooks & motion

- `useEscapeLayer` / `pushEscapeLayer` — shared Escape-key layer stack.
- `useSaveShortcut` — ⌘S / Ctrl+S binding.
- `useAsyncAction` / `createAsyncAction` — pending/busy/success/error runner
  used by ConfirmDialog and host confirm flows.
- `useScrollTop` — scroll position for `ScrollTopButton`.
- `MOTION` — shared easing/duration presets (`motion.ts`).

## Shared data cache

`@noverse/ui/data` exports the cache and SSE primitives; apps own endpoint keys, data types and event-to-key mappings. Imperative `cache.fetch(key, fetcher, { staleTime })` shares in-flight work and reuses a fresh successful result; invalidation/error always wins over the TTL. Omit `staleTime` to explicitly fetch. This lets a prefetch complete while a retained pane is hidden without discarding it or fetching it again on return.

## Consuming from another project

Minimal Vite consumer:

```jsonc
// package.json
{ "dependencies": { "@noverse/ui": "file:../../platform/ui/noverse-ui-0.1.0.tgz" } }
```

```jsx
// main.jsx
import '@noverse/ui/tokens.css'
import '@noverse/ui/kit.css'
import '@noverse/ui/style.css'
import './brand.css'                       // npx noverse-ui brand --primary '#7C5CFF' > brand.css
import { Button, Icon, Sheet, setKitLabels, defineBrand, applyBrand } from '@noverse/ui'

setKitLabels({ sheet: { grabHandle: 'Drag down to close', defaultLabel: 'Options' } })
applyBrand(defineBrand({ primary: '#7C5CFF' }))   // hoặc bỏ brand.css nếu muốn apply bằng JS

<Button variant="primary" icon={<Icon name="add" size={15} />}>Save</Button>
<Sheet open onClose={() => {}} grab><p>content</p></Sheet>
```

Reproduce this end-to-end with `npm run test:smoke`
(`scripts/smoke-consumer.mjs` — packs the tarball, installs it into a scratch
app, renders Button/Sheet/Icon under happy-dom and asserts the token CSS).

## Development

- `npm run gen` — regenerate `tokens.css`, `tokens.json`, `iconNames.ts`,
  `src/tokens.stylex.js` and `src/tokenDefaults.ts` (the data copy of the
  tokens + the brand role table `scripts/brand-roles.mjs` that `defineBrand`
  builds on) from `src/tokens.stylex.ts` / `src/icons`.
- `npm run build` — `gen` + `vite build` (ESM lib: `index`, `tokens.stylex`,
  `brand`) + `tsc` declarations.
- `npm pack` — produces `noverse-ui-<v>.tgz` (build runs via `prepack`).
- `npm run test:smoke` — consumer smoke described above; it also defines a
  custom brand, applies it and asserts the rendered button's computed colour
  equals the generated `--color-accent`.
- `npm test` / `npm run typecheck` — the kit's own vitest and tsc. From the
  repo root: `npm test -w platform/ui`.
- Guard tests: `boundary.test.ts` (package never imports an app; every app
  under `apps/*/web` that depends on `@noverse/ui` reaches it only through the
  package name), `Icon.test.tsx` (every icon name used by the kit or those apps
  has an SVG), `tokenParity.test.ts` (stylex ↔ css ↔ json), `brand.test.ts`
  (Noverse 1:1, WCAG AA pairs across sample brands, `defineBrand`
  completeness, components never read core/`--scale-*`),
  `hardcodedColors.test.ts` (colour ratchet may only go down).
