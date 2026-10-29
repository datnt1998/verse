// Render phía consumer trong happy-dom — chạy bởi smoke-consumer.mjs sau khi cài tarball.
import { Window } from 'happy-dom'

const window = new Window({ url: 'http://localhost/' })
for (const key of ['document', 'window', 'location', 'navigator', 'HTMLElement', 'Element', 'Node', 'localStorage', 'MutationObserver', 'requestAnimationFrame', 'cancelAnimationFrame', 'getComputedStyle', 'customElements']) {
  Object.defineProperty(globalThis, key, { value: window[key], configurable: true, writable: true })
}
globalThis.IS_REACT_ACT_ENVIRONMENT = false

const { createElement: h } = await import('react')
const { createRoot } = await import('react-dom/client')
const { flushSync } = await import('react-dom')
const ui = await import('@noverse/ui')

for (const name of ['Button', 'Dialog', 'Sheet', 'Icon', 'Chip', 'Tabs', 'Segmented', 'Spinner', 'setKitLabels']) {
  if (ui[name] == null) throw new Error(`export thiếu: ${name}`)
}

const root = createRoot(document.body.appendChild(window.document.createElement('div')))
flushSync(() => root.render(h(ui.Button, { variant: 'primary' }, 'Save now')))
if (!document.body.innerHTML.includes('Save now')) throw new Error('Button không render')

flushSync(() => root.render(h(ui.Sheet, { open: true, onClose() {}, title: 'Pick' }, h('p', null, 'sheet-body'))))
if (!document.body.innerHTML.includes('sheet-body')) throw new Error('Sheet không render nội dung')

flushSync(() => root.render(h(ui.Icon, { name: 'trash', size: 18, label: 'bin' })))
if (!document.body.innerHTML.includes('mask-image')) throw new Error('Icon không có mask-image')

// Brand tự định nghĩa: applyBrand gắn biến lên :root, component đọc lại đúng accent.
const brand = ui.defineBrand({ name: 'smoke', primary: '#7C5CFF' })
ui.applyBrand(brand)
const accent = brand.vars['--color-accent']
const applied = getComputedStyle(document.documentElement).getPropertyValue('--color-accent').trim()
if (!applied) throw new Error('applyBrand không gắn --color-accent lên :root')
if (applied.toLowerCase() !== accent.toLowerCase()) throw new Error(`--color-accent trên :root = ${applied}, brand = ${accent}`)
if (document.documentElement.getAttribute('data-brand') !== 'smoke') throw new Error('data-brand không được gắn')
if (!ui.brandToCss(brand).includes(`--color-accent: ${accent}`)) throw new Error('brandToCss thiếu accent')
// Nút mang accent qua var(): computed color phải là chính accent đã sinh.
flushSync(() => root.render(h(ui.Button, { variant: 'ghost', style: { color: 'var(--color-accent)' }, id: 'brand-probe' }, 'Brand')))
const probe = document.getElementById('brand-probe')
const computed = `#${getComputedStyle(probe).color.trim().replace(/^#/, '').toLowerCase()}`
const rgb = (hex) => `rgb(${parseInt(hex.slice(1, 3), 16)},${parseInt(hex.slice(3, 5), 16)},${parseInt(hex.slice(5, 7), 16)})`
const hexOf = (value) => {
  const m = /^rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(value)
  return m ? `#${[m[1], m[2], m[3]].map((n) => Number(n).toString(16).padStart(2, '0')).join('')}` : value
}
if (hexOf(computed) !== accent.toLowerCase()) throw new Error(`computed button color = ${computed}, accent = ${accent} (${rgb(accent)})`)

console.log('render ok')
