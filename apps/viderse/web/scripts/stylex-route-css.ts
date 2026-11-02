import { resolve } from 'node:path'
import { transformAsync } from '@babel/core'
import stylexBabel, { type Options, type Rule } from '@stylexjs/babel-plugin'
import browserslist from 'browserslist'
import { browserslistToTargets, transform as transformCSS } from 'lightningcss'
import postcss, { type Container } from 'postcss'
import type { Plugin, Rollup } from 'vite'

const runtimeId = 'virtual:viderse-stylex-runtime'
const runtimeKey = 'viderse.stylex.load-full'
const criticalLinkId = 'viderse-stylex-critical'

/** CSS is fetched once, before a noncritical dynamic module is evaluated. Keep
 * the old sheet until load succeeds; rejection reaches the existing lazy-route
 * error boundary rather than rendering an unstyled screen. Inserting beside the
 * original link preserves the position of unlayered tokens relative to app CSS. */
const runtime = `
const key = Symbol.for(${JSON.stringify(runtimeKey)});
let pending;
globalThis[key] = () => {
  if (pending) return pending;
  const critical = document.getElementById(${JSON.stringify(criticalLinkId)});
  if (!critical || !critical.dataset.fullHref) {
    return Promise.reject(new Error('Missing canonical StyleX stylesheet link'));
  }
  // Vite's browser baseline predates Promise.withResolvers().
  pending = new Promise((resolve, reject) => {
    const full = document.createElement('link');
    full.rel = 'stylesheet';
    full.href = critical.dataset.fullHref;
    full.onload = () => {
      full.onload = full.onerror = null;
      critical.remove();
      resolve();
    };
    full.onerror = () => {
      full.remove();
      pending = undefined;
      reject(new Error('Unable to load canonical StyleX stylesheet'));
    };
    critical.after(full);
  });
  return pending;
};
`

/** Public Babel metadata is the compiler/bundler boundary. Fail closed if its
 * documented Rule tuple changes, instead of silently shipping incomplete CSS. */
function readRules(metadata: unknown, id: string): Rule[] {
  if (metadata == null || typeof metadata !== 'object' || !('stylex' in metadata)) return []
  const rules: unknown = metadata.stylex
  if (!Array.isArray(rules)) throw new Error(`Invalid StyleX metadata in ${id}`)
  for (const rule of rules) {
    if (!Array.isArray(rule) || rule.length !== 3 || typeof rule[0] !== 'string'
      || !rule[1] || typeof rule[1] !== 'object' || typeof rule[1].ltr !== 'string'
      || (rule[1].rtl != null && typeof rule[1].rtl !== 'string')
      || typeof rule[2] !== 'number' || !Number.isFinite(rule[2])) {
      throw new Error(`Invalid StyleX Rule tuple in ${id}`)
    }
  }
  return rules as Rule[]
}

/** Select from ONE canonical compilation, never recompile a subset: the public
 * processor numbers layers according to the priority groups present in a call.
 * Keep their exact declarations/order, all keyframes, and every global rule.
 * This is build-graph ownership, not DOM/viewport/class usage pruning. */
export function selectCriticalCSS(canonical: string, allRules: readonly Rule[], criticalRules: readonly Rule[]): string {
  const known = new Set(allRules.map(([key]) => key))
  const needed = new Set(criticalRules.map(([key]) => key))
  const root = postcss.parse(canonical)
  const filter = (container: Container, inPriorityLayer = false) => {
    for (const node of [...(container.nodes ?? [])]) {
      if (node.type === 'rule' && inPriorityLayer) {
        const classes = [...node.selector.matchAll(/\.([a-zA-Z_][\w-]*)/g)].map((match) => match[1])
        const owned = classes.filter((name) => known.has(name))
        if (owned.length && !owned.some((name) => needed.has(name))) node.remove()
      } else if (node.type === 'atrule' && node.nodes && !/keyframes$/i.test(node.name)) {
        filter(node, inPriorityLayer || (node.name === 'layer' && /^priority\d+$/.test(node.params)))
        // Keep empty layer blocks/statements: they establish canonical priority.
        if (!node.nodes.length && node.name !== 'layer') node.remove()
      }
    }
  }
  filter(root)
  return root.toString()
}

function staticClosure(roots: readonly string[], getImports: (id: string) => readonly string[]): Set<string> {
  const seen = new Set<string>()
  const pending = [...roots]
  while (pending.length) {
    const id = pending.pop()!
    if (seen.has(id)) continue
    seen.add(id)
    pending.push(...getImports(id))
  }
  return seen
}

function htmlAttribute(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;')
}

/** Production-only replacement for the stock unplugin's global CSS appending.
 * Supported interfaces only: Babel withOptions/metadata/Rule/processStylexRules,
 * Rollup module graph + asset emission + renderDynamicImport, and HTML/manifest
 * output assets. No private StyleX store or Vite chunk metadata is patched.
 *
 * Critical = entry + phone workbench STATIC closures. Every other dynamic import
 * awaits the entire canonical sheet. We do not attempt unsafe per-route atom
 * reordering: full CSS replaces critical CSS in the same document slot.
 * Stock unplugin continues to own development/HMR. Build/watch state resets on
 * each buildStart; cached StyleX transforms are rerun to repopulate metadata. */
export function stylexRouteCSS(options: Partial<Options>, phoneEntry: string): Plugin {
  const rulesByModule = new Map<string, Rule[]>()
  let base = '/'
  let manifestFile: string | false = false
  let entryModule = ''
  return {
    name: 'viderse-stylex-route-css',
    apply: 'build',
    enforce: 'pre',
    configResolved(config) {
      base = config.base
      manifestFile = typeof config.build.manifest === 'string' ? config.build.manifest : config.build.manifest ? '.vite/manifest.json' : false
      entryModule = resolve(config.root, 'src/main.tsx')
    },
    buildStart() { rulesByModule.clear() },
    shouldTransformCachedModule({ meta }) { return Array.isArray(meta.viderseStylex) || meta.viderseStylexRuntime === true },
    resolveId(id) { if (id === runtimeId) return { id: `\0${runtimeId}`, moduleSideEffects: 'no-treeshake' } },
    load(id) { if (id === `\0${runtimeId}`) return runtime },
    async transform(code, id) {
      if (id === entryModule) return { code: `import ${JSON.stringify(runtimeId)};\n${code}`, map: null, meta: { viderseStylexRuntime: true } }
      if (!/\.[cm]?[jt]sx?(?:\?|$)/.test(id) || !/(?:from\s*|import\s*\(|require\s*\()\s*['"](?:@stylexjs\/stylex|stylex)['"]/.test(code)) return null
      const filename = id.split('?')[0]
      const result = await transformAsync(code, {
        filename,
        babelrc: false,
        configFile: false,
        parserOpts: { plugins: ['jsx', /\.[cm]?tsx?$/.test(filename) ? 'typescript' : 'flow'] },
        plugins: [stylexBabel.withOptions({ ...options, dev: false, runtimeInjection: false, treeshakeCompensation: true })],
        caller: { name: '@stylexjs/unplugin', supportsStaticESM: true, supportsDynamicImport: true, supportsTopLevelAwait: !code.includes('require('), supportsExportNamespaceFrom: true },
      })
      if (result?.code == null) throw new Error(`StyleX did not compile ${id}`)
      const rules = readRules(result.metadata, id)
      rulesByModule.set(id, rules)
      return { code: result.code, map: result.map ?? null, meta: { viderseStylex: rules } }
    },
    renderDynamicImport({ targetModuleId }) {
      if (targetModuleId === phoneEntry) return null
      return { left: `globalThis[Symbol.for(${JSON.stringify(runtimeKey)})]().then(() => import(`, right: '))' }
    },
    generateBundle: {
      order: 'post',
      handler(_output, bundle) {
        const chunks = Object.values(bundle).filter((item): item is Rollup.OutputChunk => item.type === 'chunk')
        const phoneChunk = chunks.find((chunk) => (chunk.modules[phoneEntry]?.renderedLength ?? 0) > 0)
        if (!phoneChunk) this.error(`Missing emitted critical phone entry: ${phoneEntry}`)
        const chunksByFile = new Map(chunks.map((chunk) => [chunk.fileName, chunk]))
        const files = staticClosure(chunks.filter((chunk) => chunk.isEntry).map((chunk) => chunk.fileName).concat(phoneChunk.fileName),
          (file) => chunksByFile.get(file)?.imports ?? [])
        const criticalModules = new Set<string>()
        for (const file of files) {
          for (const [id, module] of Object.entries(chunksByFile.get(file)?.modules ?? {})) {
            if (module.renderedLength > 0) criticalModules.add(id)
          }
        }
        const all: Rule[] = []
        const critical: Rule[] = []
        // Canonical module ordering is independent of transform concurrency.
        for (const [id, rules] of [...rulesByModule].sort(([a], [b]) => a.localeCompare(b))) {
          all.push(...rules)
          if (criticalModules.has(id)) critical.push(...rules)
        }
        const canonical = stylexBabel.processStylexRules(all, { useLayers: true })
        const targets = browserslistToTargets(browserslist())
        const [fullCSS, criticalCompiledCSS] = [canonical, selectCriticalCSS(canonical, all, critical)]
          .map((source) => transformCSS({ filename: 'stylex.css', code: Buffer.from(source), targets, minify: true }).code)
        const fullFile = this.getFileName(this.emitFile({ type: 'asset', name: 'stylex-full.css', source: fullCSS }))
        // Inline critical CSS removes one render-blocking request, not its byte
        // budget. size.mjs measures this generated style's actual UTF-8 payload.
        const criticalCSS = criticalCompiledCSS.toString().replace(/<\/style/gi, (match) => `\\3c ${match.slice(1)}`)
        let htmlFound = false
        for (const asset of Object.values(bundle)) {
          if (asset.type !== 'asset' || !asset.fileName.endsWith('.html')) continue
          const html = typeof asset.source === 'string' ? asset.source : Buffer.from(asset.source).toString()
          if (!html.includes('</head>')) this.error(`Missing HTML head in ${asset.fileName}`)
          asset.source = html.replace('</head>', `<style id="${criticalLinkId}" data-full-href="${htmlAttribute(`${base}${fullFile}`)}">${criticalCSS}</style></head>`)
          htmlFound = true
        }
        if (!htmlFound) this.error('No HTML output for canonical StyleX stylesheet')
        if (manifestFile) {
          const asset = bundle[manifestFile]
          if (!asset || asset.type !== 'asset') this.error(`Missing Vite manifest: ${manifestFile}`)
          const manifest: Record<string, { isEntry?: boolean; css?: string[]; file: string }> = JSON.parse(typeof asset.source === 'string' ? asset.source : Buffer.from(asset.source).toString())
          manifest['_stylex-full.css'] = { file: fullFile }
          asset.source = JSON.stringify(manifest, null, 2)
        }
      },
    },
  }
}
