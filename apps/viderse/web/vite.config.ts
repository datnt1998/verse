// Viderse web — dev/test đọc thẳng source gói @noverse/ui (kit dùng chung
// platform/ui): StyleX biên dịch stylex.create/defineVars lúc build, gộp vào
// CSS chính. Mỗi màn hình lazy vào chunk riêng; scripts/size.mjs đo JS tải
// lần đầu từ manifest.
import { relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

import stylex from '@stylexjs/unplugin'

// Kit dùng chung nằm ở platform/ui (npm workspace ở gốc repo).
const uiPkg = (rel: string) => fileURLToPath(new URL(`../../../platform/ui/src/${rel}`, import.meta.url))

const editorPath = (rel: string) => fileURLToPath(new URL(`./src/editor/${rel}`, import.meta.url))
// Hợp đồng clone (schema wire) dùng chung cho shell và editor, nên nằm ngoài src/editor.
const appLibPath = (rel: string) => fileURLToPath(new URL(`./src/lib/${rel}`, import.meta.url))
// StyleX's own resolver can't see vite aliases — map the package to source.
// Vitest không có httpServer nên bỏ configureServer để vòng hẹn giờ HMR không kẹt.
function stylexPlugin(): Plugin {
  const plugin: Plugin = stylex.vite({
    useCSSLayers: true,
    aliases: {
      '@noverse/ui/tokens.stylex': [uiPkg('tokens.stylex.js')],
      '@noverse/ui/*': [uiPkg('*')],
    },
    unstable_moduleResolution: {
      type: 'commonJS',
      rootDir: fileURLToPath(new URL('.', import.meta.url)),
    },
  })
  if (process.env.VITEST) delete plugin.configureServer
  return plugin
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
  // Tailwind chỉ dùng preflight + @source (styles/base.css); StyleX nằm trên nó.
  plugins: [stylexPlugin(), react(), tailwindcss(), editorBudgetManifest()],
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
  },
})
