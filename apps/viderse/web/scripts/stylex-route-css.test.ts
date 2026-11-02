import assert from 'node:assert/strict'
import { test } from 'node:test'
import { runInNewContext } from 'node:vm'
import { Window } from 'happy-dom'
import type { Rollup } from 'vite'
import stylexBabel, { type Rule } from '@stylexjs/babel-plugin'
import postcss from 'postcss'
import { selectCriticalCSS, stylexRouteCSS } from './stylex-route-css'

const vars: Rule = ['xVars', { ltr: ':root,.xVars{--brand:red}' }, 0]
const desktopVars: Rule = ['xDesktopVars', { ltr: ':root,.xDesktopVars{--desktop:20px}' }, 0]
const phone: Rule = ['xPhone', { ltr: '.xPhone{color:var(--brand)}' }, 3000]
const desktop: Rule = ['xDesktop', { ltr: '.xDesktop{color:blue}' }, 3000]
const spacing: Rule = ['xSpacing', { ltr: '.xSpacing{margin:0}' }, 1000]
const phoneMedia: Rule = ['xPhoneMedia', { ltr: '@media(max-width:767px){.xPhoneMedia{display:flex}}' }, 3200]
const desktopMedia: Rule = ['xDesktopMedia', { ltr: '@media(min-width:768px){.xDesktopMedia{display:grid}}' }, 3200]
const hover: Rule = ['xHover', { ltr: '.xHover:hover{color:red!important;color:blue!important}' }, 3130]
const keyframe: Rule = ['xSpin', { ltr: '@keyframes xSpin{to{transform:rotate(360deg)}}' }, 0]
const all = [vars, desktopVars, phone, desktop, spacing, phoneMedia, desktopMedia, hover, keyframe]

function canonical(rules: Rule[]) {
  return stylexBabel.processStylexRules(rules, { useLayers: true })
}

function layerStatements(css: string) {
  return postcss.parse(css).nodes.filter((node) => node.type === 'atrule' && node.name === 'layer' && !node.nodes).map((node) => node.toString())
}

test('extracts graph-owned atoms while retaining global tokens and canonical layers', () => {
  const full = canonical(all)
  const critical = selectCriticalCSS(full, all, [vars, phone, phoneMedia])
  assert.match(critical, /\.xPhone\{/)
  assert.match(critical, /\.xPhoneMedia\{/)
  assert.match(critical, /--brand:red/)
  assert.match(critical, /--desktop:20px/)
  assert.doesNotMatch(critical, /\.xDesktop\{|\.xDesktopMedia\{|\.xSpacing\{/)
  assert.deepEqual(layerStatements(critical), layerStatements(full))
  const fullPhoneLayer = postcss.parse(full).nodes.find((node) => node.type === 'atrule' && node.name === 'layer' && node.nodes?.some((child) => child.type === 'rule' && child.selector === '.xPhone'))
  const criticalPhoneLayer = postcss.parse(critical).nodes.find((node) => node.type === 'atrule' && node.name === 'layer' && node.nodes?.some((child) => child.type === 'rule' && child.selector === '.xPhone'))
  assert.equal(fullPhoneLayer?.type === 'atrule' && fullPhoneLayer.params, criticalPhoneLayer?.type === 'atrule' && criticalPhoneLayer.params)
})

test('keeps canonical order, pseudo selectors, important fallback declarations and keyframe names', () => {
  const full = canonical(all)
  const critical = selectCriticalCSS(full, all, [phone, phoneMedia, hover])
  const selectors: string[] = []
  postcss.parse(critical).walkRules((rule) => { if (!rule.parent || rule.parent.type !== 'atrule' || !/keyframes$/.test(rule.parent.name)) selectors.push(rule.selector) })
  const expected: string[] = []
  postcss.parse(full).walkRules((rule) => { if (selectors.includes(rule.selector)) expected.push(rule.selector) })
  assert.deepEqual(selectors, expected)
  assert.match(critical, /\.xHover:hover\{color:red!important;color:blue!important\}/)
  assert.match(critical, /@keyframes xSpin/)
})

test('retains globals and unknown compiler selectors conservatively', () => {
  const full = '@layer priority1,priority2;html{--direction:1}@property --custom{syntax:"<number>";inherits:false;initial-value:0}@layer priority2{.xPhone{color:red}.unknown{display:block}}'
  const critical = selectCriticalCSS(full, all, [])
  assert.match(critical, /html\{--direction:1\}/)
  assert.match(critical, /@property --custom/)
  assert.match(critical, /\.unknown\{display:block\}/)
  assert.doesNotMatch(critical, /\.xPhone\{/)
})

test('retains atoms reused by a critical and a lazy module', () => {
  const shared: Rule = ['xShared', { ltr: '.xShared{padding:8px}' }, 1000]
  const full = canonical([shared, shared, desktop])
  assert.match(selectCriticalCSS(full, [shared, shared, desktop], [shared]), /\.xShared\{/)
})

test('keeps empty priority groups when no atoms are critical', () => {
  const full = canonical([spacing, phone])
  const critical = selectCriticalCSS(full, [spacing, phone], [])
  assert.deepEqual(layerStatements(critical), layerStatements(full))
  assert.doesNotMatch(critical, /\.xSpacing\{|\.xPhone\{/)
})

test('rejects malformed canonical CSS rather than shipping a partial sheet', () => {
  assert.throws(() => selectCriticalCSS('@layer priority1{.xPhone{', all, [phone]))
})

async function stylesheetRuntime(withLink = true) {
  const plugin = stylexRouteCSS({}, ['/phone.tsx'])
  if (typeof plugin.load !== 'function') throw new Error('Expected runtime load hook')
  const source = await plugin.load.call({} as Rollup.PluginContext, '\0virtual:viderse-stylex-runtime', {})
  if (typeof source !== 'string') throw new Error('Expected runtime source')
  // Relative fixture hrefs are deliberately nonfetchable; dispatch DOM events.
  const window = new Window({ url: 'about:blank' })
  if (withLink) window.document.head.innerHTML = '<style id="viderse-stylex-critical" data-full-href="/full.css">.critical{color:red}</style><link id="route-css" rel="stylesheet" href="/editor.css">'
  const context: Record<PropertyKey, unknown> = { document: window.document, Symbol, Promise }
  runInNewContext(source, context)
  const load = runInNewContext('globalThis[Symbol.for("viderse.stylex.load-full")]', context)
  if (typeof load !== 'function') throw new Error('Expected stylesheet gate')
  return { window, load: load as () => Promise<void> }
}

test('shares one full-sheet load and replaces critical CSS before the existing route CSS slot', async () => {
  const { window, load } = await stylesheetRuntime()
  const pending = load()
  assert.equal(load(), pending)
  const critical = window.document.getElementById('viderse-stylex-critical')!
  const full = critical.nextElementSibling!
  assert.equal(full.getAttribute('href'), '/full.css')
  assert.equal(full.nextElementSibling?.id, 'route-css')
  assert.ok(critical.isConnected)
  full.dispatchEvent(new window.Event('load'))
  await pending
  assert.equal(window.document.getElementById('viderse-stylex-critical'), null)
  assert.ok(full.isConnected)
  assert.equal(load(), pending)
})

test('rejects stylesheet failures without removing critical CSS or evaluating lazy UI', async () => {
  const { window, load } = await stylesheetRuntime()
  const pending = load()
  const rejected = assert.rejects(pending, /Unable to load canonical StyleX stylesheet/)
  const critical = window.document.getElementById('viderse-stylex-critical')!
  critical.nextElementSibling!.dispatchEvent(new window.Event('error'))
  await rejected
  assert.ok(critical.isConnected)
  assert.equal(critical.nextElementSibling?.id, 'route-css')
  const next = load()
  assert.notEqual(next, pending)
  const retryRejected = assert.rejects(next, /Unable to load canonical StyleX stylesheet/)
  critical.nextElementSibling!.dispatchEvent(new window.Event('error'))
  await retryRejected
})

test('rejects a missing stylesheet contract instead of silently rendering without CSS', async () => {
  const { load } = await stylesheetRuntime(false)
  await assert.rejects(load(), /Missing canonical StyleX stylesheet link/)
})
