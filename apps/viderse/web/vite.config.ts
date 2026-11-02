// Dev/test use the shared kit's source and stock StyleX HMR. Production emits
// canonical critical/full StyleX sheets; noncritical routes await the full sheet.
// scripts/size.mjs reads the cold entry closure from Vite's manifest.
import { createRequire } from 'node:module'
import { relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

import stylex from '@stylexjs/unplugin'
import { mobileRouteChunks } from './scripts/mobile-route-chunks'
import { stylexRouteCSS } from './scripts/stylex-route-css'

// Kit dùng chung nằm ở platform/ui (npm workspace ở gốc repo).
const uiPkg = (rel: string) => fileURLToPath(new URL(`../../../platform/ui/src/${rel}`, import.meta.url))

const editorPath = (rel: string) => fileURLToPath(new URL(`./src/editor/${rel}`, import.meta.url))
// Hợp đồng clone (schema wire) dùng chung cho shell và editor, nên nằm ngoài src/editor.
const appLibPath = (rel: string) => fileURLToPath(new URL(`./src/lib/${rel}`, import.meta.url))
// StyleX's own resolver can't see vite aliases — map the package to source.
// Vitest không có httpServer nên bỏ configureServer để vòng hẹn giờ HMR không kẹt.
function stylexPlugins(): Plugin[] {
  const options = {
    aliases: {
      '@noverse/ui/tokens.stylex': [uiPkg('tokens.stylex.js')],
      '@noverse/ui/*': [uiPkg('*')],
    },
    unstable_moduleResolution: {
      type: 'commonJS' as const,
      rootDir: fileURLToPath(new URL('.', import.meta.url)),
    },
  }
  const plugin: Plugin = stylex.vite({ ...options, useCSSLayers: true })
  plugin.apply = 'serve'
  if (process.env.VITEST) delete plugin.configureServer
  return [plugin, stylexRouteCSS(options, [
    fileURLToPath(new URL('./src/screens/workbench/mobile/MobileWorkbench.tsx', import.meta.url)),
    createRequire(import.meta.url).resolve('react-dom/client'),
  ])]
}

/** Preserve module provenance alongside Vite's manifest for the cold-route
 * budget. Ids are relative to this app so the shipped bundle carries no
 * machine path and builds byte-identically on every machine. */
function editorBudgetManifest(): Plugin {
  const root = fileURLToPath(new URL('.', import.meta.url))
  return {
    name: 'viderse-editor-budget',
    apply: 'build',
    generateBundle(_options, bundle) {
      const chunks: Record<string, string[]> = {}
      for (const output of Object.values(bundle)) {
        if (output.type !== 'chunk') continue
        chunks[output.fileName] = Object.keys(output.modules)
          .filter((id) => id.includes('/src/editor/'))
          .map((id) => relative(root, id))
      }
      this.emitFile({ type: 'asset', fileName: '.vite/editor-budget.json', source: JSON.stringify(chunks) })
    },
  }
}

export default defineConfig({
  resolve: {
    alias: [
      { find: /^@noverse\/ui\/tokens\.stylex$/, replacement: uiPkg('tokens.stylex.ts') },
      { find: /^@noverse\/ui\/(.+\.css)$/, replacement: uiPkg('$1') },
      { find: /^@noverse\/ui\/data$/, replacement: uiPkg('data/index.ts') },
      { find: /^@noverse\/ui$/, replacement: uiPkg('index.ts') },
      { find: /^@editor\/(.+)$/, replacement: editorPath('engine/$1') },
      { find: /^@\/editor\/(.+)$/, replacement: editorPath('$1') },
      { find: /^@\/components\/ui\/(.+)$/, replacement: editorPath('vendor/components/ui/$1') },
      { find: /^@\/components\/(icon-tip|video-thumb|link-pending)$/, replacement: editorPath('vendor/components/$1') },
      { find: /^@\/components\/clone\/source-comparison$/, replacement: editorPath('vendor/components/clone/source-comparison.tsx') },
      { find: /^@\/components\/clone\/clone-entry$/, replacement: editorPath('bridge/ui/clone-status.ts') },
      { find: /^@\/app\/editor\/actions$/, replacement: editorPath('bridge/ui/project-actions.ts') },
      { find: /^@\/lib\/(clone-view|clone-quality)$/, replacement: appLibPath('clone/$1.ts') },
      { find: /^@\/lib\/(app-routes|navigation-return|format|video-sort)$/, replacement: editorPath('vendor/lib/$1.ts') },
    ],
    dedupe: ['react', 'react-dom'],
  },
  // Preflight stays global; NLE utilities are imported by the lazy editor.
  plugins: [...stylexPlugins(), react(), tailwindcss(), editorBudgetManifest(), mobileRouteChunks()],
  worker: { format: 'es' },
  server: {
    port: 5181,
    proxy: {
      '/api': { target: process.env.VITE_API_TARGET ?? 'http://127.0.0.1:8789', changeOrigin: true },
    },
  },
  build: {
    // entry/web phục vụ dist như app nhúng (xem internal/web).
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    manifest: true,
    // Inline only masks painted by the cold phone shell and workbench.
    // The eager icon catalogue must not pull every route's SVG into the entry.
    assetsInlineLimit: (path) => /\/platform\/ui\/src\/icons\/(?:activity|add|channel|radar|gallery|workbench-b|arrow-d|arrow-r|info|pause)\.svg$/.test(path) ? true : undefined,
  },
})
